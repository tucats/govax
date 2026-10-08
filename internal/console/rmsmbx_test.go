package console_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/sched"
	"github.com/tucats/govax/internal/vmsdef"
)

// Phase 46's subtask 6: RMS on mailboxes and NL:. A subprocess whose
// SYS$OUTPUT is a mailbox writes its LIB$PUT_OUTPUT lines there as
// messages, and its creator reads them with RMS.

// Where process 1's programs keep things, in its P0: a FAB and a RAB
// (built by the test), the statuses, and the records read.
const (
	rmsFAB     = mbxData + 0x400
	rmsRAB     = mbxData + 0x500
	rmsName    = mbxData + 0x600 // the file name the FAB names
	rmsStatus  = mbxData + 0x20  // each $GET's status and record size
	rmsRecords = mbxData + 0x100 // the records, 64 bytes apart
)

// putFABRAB builds, in env's P0, a FAB naming name with access fac and
// a RAB connected to it, its records going to rmsRecords.
func putFABRAB(t *testing.T, c *console.Console, env *corevms.Environment, name string, fac byte) {
	t.Helper()

	sym := vmsdef.Symbols
	as := env.Space.AddressSpace

	store := func(addr uint32, b ...byte) {
		if err := c.Mem.StoreIn(c.CPU, as, addr, b); err != nil {
			t.Fatal(err)
		}
	}

	long := func(addr, v uint32) { store(addr, byte(v), byte(v>>8), byte(v>>16), byte(v>>24)) }

	store(rmsFAB+sym["FAB$B_BID"], byte(sym["FAB$C_BID"]))
	store(rmsFAB+sym["FAB$B_BLN"], byte(sym["FAB$C_BLN"]))
	store(rmsFAB+sym["FAB$B_FAC"], fac)
	long(rmsFAB+sym["FAB$L_FNA"], rmsName)
	store(rmsFAB+sym["FAB$B_FNS"], byte(len(name)))
	store(rmsName, []byte(name)...)

	store(rmsRAB+sym["RAB$B_BID"], byte(sym["RAB$C_BID"]))
	store(rmsRAB+sym["RAB$B_BLN"], byte(sym["RAB$C_BLN"]))
	long(rmsRAB+sym["RAB$L_FAB"], rmsFAB)
	long(rmsRAB+sym["RAB$L_UBF"], rmsRecords)
	store(rmsRAB+sym["RAB$W_USZ"], 64, 0)
}

// rmsSymbols are the names the programs use, as assignments.
func rmsSymbols() string {
	s := fmt.Sprintf("data = ^X%X\nfab = ^X%X\nrab = ^X%X\n", mbxData, rmsFAB, rmsRAB)

	for _, name := range []string{"RAB$L_UBF", "RAB$W_RSZ", "RAB$L_RBF"} {
		s += fmt.Sprintf("%s = ^X%X\n", strings.NewReplacer("$", "_").Replace(name), vmsdef.Symbols[name])
	}

	return s
}

// outChild is the child's image: four 20-character lines through
// LIB$PUT_OUTPUT.
const outChild = `	.title	outchild
	.psect	code,exe,nowrt
	.entry	start,^m<>
	pushaq	line1
	calls	#1,g^lib$put_output
	pushaq	line2
	calls	#1,g^lib$put_output
	pushaq	line3
	calls	#1,g^lib$put_output
	pushaq	line4
	calls	#1,g^lib$put_output
	movl	#1,r0
	ret
	.psect	data,noexe,wrt
line1:	.ascid	/line 1 of the child./
line2:	.ascid	/line 2 of the child./
line3:	.ascid	/line 3 of the child./
line4:	.ascid	/line 4 of the child./
	.end	start
`

// outParent is process 1's program: create the temporary mailbox
// CHILDOUT (maxmsg 64, bufquo 48: room for two 20-byte lines), create a
// subprocess running the image %[1]s at base priority %[2]d with
// CHILDOUT as its output (its PID at data+4), sleep 10 s (the child
// fills the mailbox meanwhile), then $OPEN CHILDOUT and $GET four
// records, storing each $GET's status and RSZ at data+0x20 up, the
// records at data+0x100 up, and 1 at data.
const outParent = `
	callg	mbx, @#sys$crembx
	callg	crearg, @#sys$creprc
	movl	r0, @#data+12
	callg	wakeup, @#sys$schdwk
	calls	#0, @#sys$hiber
	pushal	@#fab
	calls	#1, @#sys$open
	movl	r0, @#data+8
	pushal	@#rab
	calls	#1, @#sys$connect
	movl	#4, r7
	movl	#^X%[3]X, r8
	movl	#^X%[4]X, r9
get:	movl	r8, @#rab+RAB_L_UBF
	pushal	@#rab
	calls	#1, @#sys$get
	movl	r0, (r9)+
	movzwl	@#rab+RAB_W_RSZ, (r9)+
	addl2	#64, r8
	sobgtr	r7, get
	movl	#1, @#data
done:	brb	done
mbx:	.long	7, 0, chan, 64, 48, 0, 0, mbxnam
chan:	.word	0
mbxnam:	.ascid	"CHILDOUT"
crearg:	.long	12, data+4, image, 0, mbxnam, 0, 0, 0, 0, %[2]d, 0, 0, 0
image:	.ascid	"%[1]s"
wakeup:	.long	4, 0, 0, delta, 0
delta:	.quad	-100000000
`

// TestRMSMailbox_childOutput: the child's lines arrive whole and in
// order. The child fills the mailbox while process 1 sleeps and waits
// for room (MWAIT, RWMBX); process 1's $GETs make room, and then wait
// (LEF) for the lines the child hasn't written yet.
func TestRMSMailbox_childOutput(t *testing.T) {
	c, _ := scheduledConsole(t, longQuantum, brbSelf)
	exe := buildImage(t, c, "outchild", outChild)
	one := c.RTL

	code, _ := assembleAt(t, rmsSymbols()+fmt.Sprintf(outParent, exe, one.Process.BasePriority-2, rmsRecords, rmsStatus))
	if codeAddr+len(code) > mbxData {
		t.Fatalf("the parent's program (%d bytes) runs into its data", len(code))
	}

	if err := c.Mem.StoreIn(c.CPU, one.Space.AddressSpace, codeAddr, code); err != nil {
		t.Fatal(err)
	}

	putFABRAB(t, c, one, "CHILDOUT", byte(vmsdef.Symbols["FAB$M_GET"]))

	sawChildMWAIT, sawParentLEF := false, false

	runUntil(t, c, 400000, func() bool {
		if pid := longwordAt(t, c, one, mbxData+4); pid != 0 {
			if child, ok := one.FindProcess(pid); ok && stateOf(child) == sched.StateMWAIT {
				sawChildMWAIT = true
			}
		}

		// Waiting in a $GET: the open is done.
		if longwordAt(t, c, one, mbxData+8) != 0 && stateOf(one) == sched.StateLEF {
			sawParentLEF = true
		}

		return longwordAt(t, c, one, mbxData) == 1
	})

	if st := longwordAt(t, c, one, mbxData+12); st != 1 {
		t.Fatalf("$CREPRC: %08X", st)
	}

	if st := longwordAt(t, c, one, mbxData+8); st != vmsdef.Symbols["RMS$_NORMAL"] {
		t.Fatalf("$OPEN: %08X, want RMS$_NORMAL", st)
	}

	for i := range uint32(4) {
		st, rsz := longwordAt(t, c, one, rmsStatus+i*8), longwordAt(t, c, one, rmsStatus+i*8+4)

		buf := make([]byte, rsz)
		if err := c.Mem.LoadIn(c.CPU, one.Space.AddressSpace, rmsRecords+i*64, buf); err != nil {
			t.Fatal(err)
		}

		if want := fmt.Sprintf("line %d of the child.", i+1); st != vmsdef.Symbols["RMS$_NORMAL"] || string(buf) != want {
			t.Errorf("$GET %d: %08X, %q; want RMS$_NORMAL, %q", i+1, st, buf, want)
		}
	}

	if !sawChildMWAIT || !sawParentLEF {
		t.Errorf("seen: the child waiting for room %v, process 1 waiting for a line %v; want both", sawChildMWAIT, sawParentLEF)
	}
}

// nlProgram is process 1's program for NL:: $CREATE, $CONNECT, $PUT,
// and $CLOSE, then $OPEN, $CONNECT, and $GET, each status stored in
// turn from data+0x20, then 1 at data.
const nlProgram = `
	movl	#^X%[1]X, r9
	pushal	@#fab
	calls	#1, @#sys$create
	movl	r0, (r9)+
	pushal	@#rab
	calls	#1, @#sys$connect
	movl	r0, (r9)+
	moval	text, @#rab+RAB_L_RBF
	movw	#5, @#rab+RAB_W_RSZ
	pushal	@#rab
	calls	#1, @#sys$put
	movl	r0, (r9)+
	pushal	@#fab
	calls	#1, @#sys$close
	movl	r0, (r9)+
	movb	#^X%[2]X, @#fab+%[3]d
	pushal	@#fab
	calls	#1, @#sys$open
	movl	r0, (r9)+
	pushal	@#rab
	calls	#1, @#sys$connect
	movl	r0, (r9)+
	pushal	@#rab
	calls	#1, @#sys$get
	movl	r0, (r9)+
	movl	#1, @#data
done:	brb	done
text:	.ascii	"hello"
`

// TestRMSMailbox_null: NL: takes any record and has none to give: the
// $PUT succeeds and the $GET is RMS$_EOF.
func TestRMSMailbox_null(t *testing.T) {
	sym := vmsdef.Symbols

	code, _ := assembleAt(t, rmsSymbols()+fmt.Sprintf(nlProgram, rmsStatus, sym["FAB$M_GET"], sym["FAB$B_FAC"]))

	c, _ := scheduledConsole(t, longQuantum, code)
	one := c.RTL

	putFABRAB(t, c, one, "NL:", byte(sym["FAB$M_PUT"]))

	runUntil(t, c, 100000, func() bool { return longwordAt(t, c, one, mbxData) == 1 })

	normal := sym["RMS$_NORMAL"]
	want := []uint32{normal, normal, normal, normal, normal, normal, sym["RMS$_EOF"]}

	for i, w := range want {
		if got := longwordAt(t, c, one, rmsStatus+uint32(i)*4); got != w {
			t.Errorf("status %d: %08X, want %08X", i+1, got, w)
		}
	}

	if dev := longwordAt(t, c, one, rmsFAB+sym["FAB$L_DEV"]); dev&sym["DEV$M_REC"] == 0 {
		t.Errorf("FAB$L_DEV %08X: not record oriented", dev)
	}
}
