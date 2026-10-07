package console_test

import (
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/vmsdef"
)

// Phase 45's subtask 7: a process $CREPRC created with a termination
// mailbox writes the accounting message to it when it's deleted, and
// its creator reads it.

// Where the parent program keeps things, in process 1's P0.
const (
	termIOSB   = dataAddr + 8    // the read's I/O status block
	termChan   = dataAddr + 0x10 // the mailbox's channel (a word)
	termUnit   = dataAddr + 0x14 // its unit number, from $GETDVIW
	termBuffer = dataAddr + 0x80 // the message
)

// termParent is process 1's program: create a temporary mailbox, find
// its unit number with $GETDVIW, create a subprocess running the image
// at base priority %d with that mailbox for its termination message
// (its PID at dataAddr+4), and $QIOW a read of the mailbox; then store
// the read's R0 at dataAddr and spin.
const termParent = `
	callg	mbx, @#sys$crembx
	movzwl	@#^X%[1]X, dvi+8
	callg	dvi, @#sys$getdviw
	movl	@#^X%[2]X, crearg+44
	callg	crearg, @#sys$creprc
	movzwl	@#^X%[1]X, qio+8
	callg	qio, @#sys$qiow
	movl	r0, @#^X600
done:	brb	done
mbx:	.long	2, 0, ^X%[1]X
dvi:	.long	4, 0, 0, 0, items
items:	.word	4, %[3]d
	.long	^X%[2]X, 0
	.long	0
crearg:	.long	11, ^X604, image, 0, 0, 0, 0, 0, 0, %[4]d, 0, 0
image:	.word	%[5]d, 0
	.long	imagetext
qio:	.long	12, 0, 0, %[6]d, ^X%[7]X, 0, 0, ^X%[8]X, 128, 0, 0, 0, 0
imagetext: .ascii "%[9]s"
`

// runTermParent puts termParent, for image and the child's base
// priority, in process 1, and runs the machine until its read is done.
// It returns the message, the read's IOSB, and the PID $CREPRC returned.
func runTermParent(t *testing.T, c *console.Console, image string, baspri uint32) (msg []byte, status, count, info, pid uint32) {
	t.Helper()

	one := c.RTL
	src := fmt.Sprintf(termParent, termChan, termUnit, vmsdef.Symbols["DVI$_UNIT"], baspri, len(image),
		vmsdef.Symbols["IO$_READVBLK"], termIOSB, termBuffer, image)

	code, _ := assembleAt(t, src)
	if codeAddr+len(code) > dataAddr {
		t.Fatalf("the parent's program (%d bytes) runs into its data", len(code))
	}

	if err := c.Mem.StoreIn(c.CPU, one.Space.AddressSpace, codeAddr, code); err != nil {
		t.Fatal(err)
	}

	runUntil(t, c, 200000, func() bool { return longwordAt(t, c, one, dataAddr) != 0 })

	if r0 := longwordAt(t, c, one, dataAddr); r0 != 1 {
		t.Fatalf("$QIOW returned %08X", r0)
	}

	first := longwordAt(t, c, one, termIOSB)

	msg = make([]byte, 84)
	if err := c.Mem.LoadIn(c.CPU, one.Space.AddressSpace, termBuffer, msg); err != nil {
		t.Fatal(err)
	}

	return msg, first & 0xFFFF, first >> 16, longwordAt(t, c, one, termIOSB+4), longwordAt(t, c, one, dataAddr+4)
}

// accField reads a longword (or, for a Q field, a quadword) $ACCDEF
// field of a termination message.
func accField(msg []byte, name string) uint64 {
	off := vmsdef.Symbols[name]
	if name[4] == 'Q' {
		return binary.LittleEndian.Uint64(msg[off:])
	}

	return uint64(binary.LittleEndian.Uint32(msg[off:]))
}

// TestTerminationMessage: process 1 reads its child's termination
// message. With the child below process 1's priority, process 1's read
// is waiting when the child is deleted, so the deletion, in the child's
// context, completes a read in process 1's address space; at process 1's
// priority, the child runs to its end first, and the message waits in
// the mailbox for the read. Either way the read gets the 84-byte
// message, from the child's PID, with its final status (its image
// returned 3) and its owner, process 1.
func TestTerminationMessage(t *testing.T) {
	for _, tt := range []struct {
		name   string
		baspri func(one uint32) uint32
	}{
		{"read waiting", func(one uint32) uint32 { return one - 2 }},
		{"message waiting", func(one uint32) uint32 { return one }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c, _ := scheduledConsole(t, longQuantum, brbSelf)
			exe := buildChildImage(t, c)
			one := c.RTL

			msg, status, count, info, pid := runTermParent(t, c, exe, tt.baspri(one.Process.BasePriority))

			if status != 1 || count != 84 || info != pid {
				t.Errorf("read IOSB: %08X, %d bytes, from %08X; want SS$_NORMAL, 84, from %08X", status, count, info, pid)
			}

			if _, found := one.FindProcess(pid); found {
				t.Error("the child is still in the process table")
			}

			for name, want := range map[string]uint64{
				"ACC$W_MSGTYP":   uint64(vmsdef.Symbols["MSG$_DELPROC"]),
				"ACC$L_FINALSTS": 3,
				"ACC$L_PID":      uint64(pid),
				"ACC$L_OWNER":    uint64(one.Process.PID),
			} {
				if got := accField(msg, name); got != want {
					t.Errorf("%s = %#x, want %#x", name, got, want)
				}
			}

			login, logout := accField(msg, "ACC$Q_LOGIN"), accField(msg, "ACC$Q_TERMTIME")
			if login < one.Process.LoginTime || logout < login || logout > one.Clock() {
				t.Errorf("logged in at %#x, out at %#x; want between process 1's login (%#x) and now (%#x)",
					login, logout, one.Process.LoginTime, one.Clock())
			}

			if got, want := string(msg[32:44]), fmt.Sprintf("%-12s", one.Process.Username); got != want {
				t.Errorf("ACC$T_USERNAME = %q, want %q", got, want)
			}
		})
	}
}

// TestTerminationMessage_startupFails: a child whose image can't be
// found ends at once, and its creator learns why only from the
// termination message: RMS$_FNF.
func TestTerminationMessage_startupFails(t *testing.T) {
	c, _ := scheduledConsole(t, longQuantum, brbSelf)
	one := c.RTL

	msg, status, _, info, pid := runTermParent(t, c, "NOSUCH.EXE", one.Process.BasePriority-2)

	if status != 1 || info != pid {
		t.Errorf("read IOSB: %08X, from %08X; want SS$_NORMAL, from %08X", status, info, pid)
	}

	if got, want := accField(msg, "ACC$L_FINALSTS"), uint64(vmsdef.Symbols["RMS$_FNF"]); got != want {
		t.Errorf("ACC$L_FINALSTS = %08X, want RMS$_FNF (%08X)", got, want)
	}
}
