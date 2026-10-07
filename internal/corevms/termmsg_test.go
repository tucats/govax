package corevms

import (
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/tucats/govax/internal/vmsdef"
)

// The termination message (docs/PHASE-45.md, subtask 7): a process
// $CREPRC gave a termination mailbox writes the accounting message to it
// when it's deleted. The fixture's processes share one memory; the
// message reaching another address space is tested with a booted
// machine, in internal/console's termmsg_test.go.

// termFixture makes a parent with a mailbox (maxmsg and bufquo as
// $CREMBX takes them) and a subprocess of it whose termination mailbox
// it is, created at system time login. The clock then reads logout.
func termFixture(t *testing.T, maxmsg, bufquo uint32) (parent, child *Environment, a *arena, ch uint32) {
	t.Helper()

	const login, logout = 0x00A0_0000_1000_0000, 0x00A0_0000_2000_0000

	parent, _ = fixture()
	a = newArena(t, parent)

	r0, ch := crembx(t, parent, a, 0, maxmsg, bufquo, "")
	wantR0(t, r0, ssNormal)

	parent.Clock = func() uint64 { return login }
	child = newSubprocess(t, parent)
	parent.Clock = func() uint64 { return logout }

	child.Process.TerminationMailbox = mailboxOn(t, parent, ch).Unit

	return parent, child, a, ch
}

// field reads a $ACCDEF field of a termination message.
func field(msg []byte, name string, size int) uint64 {
	off := vmsdef.Symbols[name]

	switch size {
	case 2:
		return uint64(binary.LittleEndian.Uint16(msg[off:]))
	case 8:
		return binary.LittleEndian.Uint64(msg[off:])
	default:
		return uint64(binary.LittleEndian.Uint32(msg[off:]))
	}
}

// TestTerminationMessage_fields: the message waits in the mailbox until
// its creator reads it, as from the deleted process (the read's IOSB
// has its PID), 84 bytes, with each field the manual lists.
func TestTerminationMessage_fields(t *testing.T) {
	parent, child, a, ch := termFixture(t, 0, 0)

	child.Process.ExitStatus = 0x1000_0003
	child.cpuTime = 250 * cpuQuotaUnit
	pid := child.Process.PID

	parent.DeleteProcess(child)

	if n := mailboxOn(t, parent, ch).Messages(); n != 1 {
		t.Fatalf("%d messages in the mailbox, want 1", n)
	}

	iosb, buf := a.alloc(8), a.alloc(128)
	wantR0(t, mbxQIO(t, parent, 0, ch, fnReadVBlk, iosb, 0, buf, 128), ssNormal)

	if st, n, info := readIOSB(a, iosb); st != ssNormal || n != 84 || info != pid {
		t.Fatalf("read IOSB: %#x, %d bytes, from %08X; want SS$_NORMAL, 84, %08X", st, n, info, pid)
	}

	msg := readBytes(t, parent, buf, 84)

	for _, f := range []struct {
		name string
		size int
		want uint64
	}{
		{"ACC$W_MSGTYP", 2, 3}, // MSG$_DELPROC
		{"ACC$L_FINALSTS", 4, 0x1000_0003},
		{"ACC$L_PID", 4, uint64(pid)},
		{"ACC$L_JOBID", 4, 0},
		{"ACC$Q_TERMTIME", 8, 0x00A0_0000_2000_0000},
		{"ACC$L_CPUTIM", 4, 250},
		{"ACC$L_PAGEFLTS", 4, 0},
		{"ACC$L_PGFLPEAK", 4, 0},
		{"ACC$L_WSPEAK", 4, 0},
		{"ACC$L_BIOCNT", 4, 0},
		{"ACC$L_DIOCNT", 4, 0},
		{"ACC$L_VOLUMES", 4, 0},
		{"ACC$Q_LOGIN", 8, 0x00A0_0000_1000_0000},
		{"ACC$L_OWNER", 4, uint64(parent.Process.PID)},
	} {
		if got := field(msg, f.name, f.size); got != f.want {
			t.Errorf("%s = %#x, want %#x", f.name, got, f.want)
		}
	}

	if got := field(msg, "ACC$W_MSGTYP", 4) >> 16; got != 0 {
		t.Errorf("the unused word after ACC$W_MSGTYP is %#x", got)
	}

	if got, want := string(msg[24:32]), fmt.Sprintf("%-8s", parent.Process.Account); got != want {
		t.Errorf("ACC$T_ACCOUNT = %q, want %q", got, want)
	}

	if got, want := string(msg[32:44]), fmt.Sprintf("%-12s", parent.Process.Username); got != want {
		t.Errorf("ACC$T_USERNAME = %q, want %q", got, want)
	}
}

// TestTerminationMessage_waitingRead: a read the creator has waiting is
// completed by the deletion: the creator's IOSB, event flag, and AST,
// not the deleted process's.
func TestTerminationMessage_waitingRead(t *testing.T) {
	parent, child, a, ch := termFixture(t, 0, 0)
	pid := child.Process.PID
	iosb, buf := a.alloc(8), a.alloc(128)

	parent.Process.LocalEventFlags[0] &^= 1 << 7
	wantR0(t, mbxQIO(t, parent, 7, ch, fnReadVBlk, iosb, 0x4000, buf, 128), ssNormal)

	if parent.PendingIO() != 1 {
		t.Fatal("the read didn't wait")
	}

	child.Process.LocalEventFlags[0] &^= 1 << 7
	parent.DeleteProcess(child)

	if st, n, info := readIOSB(a, iosb); st != ssNormal || n != 84 || info != pid {
		t.Errorf("read IOSB: %#x, %d bytes, from %08X; want SS$_NORMAL, 84, %08X", st, n, info, pid)
	}

	if !flagSet(parent, 7) || parent.PendingIO() != 0 || parent.PendingASTs() != 1 {
		t.Errorf("the creator's flag set %v, %d pending, %d ASTs; want set, 0, 1",
			flagSet(parent, 7), parent.PendingIO(), parent.PendingASTs())
	}

	if flagSet(child, 7) {
		t.Error("the deleted process's event flag 7 was set")
	}

	if got := field(readBytes(t, parent, buf, 84), "ACC$L_PID", 4); got != uint64(pid) {
		t.Errorf("ACC$L_PID = %08X, want %08X", got, pid)
	}
}

// TestTerminationMessage_lost: a termination mailbox that doesn't exist,
// is too small for the message, or is full is treated as none: the
// message is lost, and the deletion goes on. So is the deleted process's
// own temporary mailbox, which went with its last channel.
func TestTerminationMessage_lost(t *testing.T) {
	for _, tt := range []struct {
		name           string
		maxmsg, bufquo uint32
		unit           uint32 // 0: the mailbox's
		fill           int    // bytes written to the mailbox first
	}{
		{name: "no such mailbox", unit: 999},
		{name: "too small", maxmsg: 83},
		{name: "full", maxmsg: 100, bufquo: 100, fill: 20},
	} {
		t.Run(tt.name, func(t *testing.T) {
			parent, child, a, ch := termFixture(t, tt.maxmsg, tt.bufquo)
			m := mailboxOn(t, parent, ch)

			if tt.unit != 0 {
				child.Process.TerminationMailbox = tt.unit
			}

			if tt.fill > 0 {
				wantR0(t, mbxQIO(t, parent, 0, ch, fnWriteNow, 0, 0, a.str(fmt.Sprintf("%*s", tt.fill, "")), uint32(tt.fill)), ssNormal)
			}

			before := m.Messages()
			parent.DeleteProcess(child)

			if !child.Deleted || m.Messages() != before {
				t.Errorf("deleted %v, %d messages; want deleted, %d", child.Deleted, m.Messages(), before)
			}
		})
	}

	t.Run("own temporary mailbox", func(t *testing.T) {
		parent, _ := fixture()
		child := newSubprocess(t, parent)
		a := newArena(t, child)

		r0, ch := crembx(t, child, a, 0, 0, 0, "")
		wantR0(t, r0, ssNormal)

		child.Process.TerminationMailbox = mailboxOn(t, child, ch).Unit
		parent.DeleteProcess(child)

		if !child.Deleted || len(parent.Mailboxes.All()) != 0 {
			t.Errorf("deleted %v, %d mailboxes; want deleted, none", child.Deleted, len(parent.Mailboxes.All()))
		}
	})
}
