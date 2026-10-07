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

// Phase 46's subtask 2: mailboxes between processes. Each test runs two
// processes on a booted machine, each its own program in its own P0,
// sharing a permanent mailbox named PIPE, for one of the cases the
// mailbox driver documents (internal/corevms/mbxdriver.go).
//
// The order of events is made certain by priority: process 1 runs at
// base priority 10 and process 2 at 2, so process 2, even with the
// largest boost (6), runs only while process 1 waits, and process 1
// runs again as soon as its wait ends. Each program finishes by storing
// 1 at mbxData and hibernating for good, so the other can run.

// mbxData is where each program keeps its results, in its own P0;
// mbxData+4 has the other process's PID, stored by the test.
const mbxData = 0x3000

// mbxCommon is the code both programs share, after their own:
//
//   - fin: store 1 at mbxData (done), and hibernate for good;
//   - qio and qiow: $QIO or $QIOW on the mailbox (channel in R6) with
//     event flag R1, function R2, IOSB R3, and p1 and p2 R4 and R5; R0
//     is the service's status;
//   - wake: $WAKE the other process; hiber: $HIBER.
const mbxCommon = `
fin:	movl	#1, @#data
forever: calls	#0, @#sys$hiber
	brb	forever
qio:	clrq	-(sp)			; p5, p6
	clrq	-(sp)			; p3, p4
	pushl	r5			; p2
	pushl	r4			; p1
	clrq	-(sp)			; astadr, astprm
	pushl	r3			; iosb
	pushl	r2			; func
	pushl	r6			; chan
	pushl	r1			; efn
	calls	#12, @#sys$qio
	rsb
qiow:	clrq	-(sp)
	clrq	-(sp)
	pushl	r5
	pushl	r4
	clrq	-(sp)
	pushl	r3
	pushl	r2
	pushl	r6
	pushl	r1
	calls	#12, @#sys$qiow
	rsb
wake:	pushl	#0
	pushal	@#data+4
	calls	#2, @#sys$wake
	rsb
hiber:	calls	#0, @#sys$hiber
	rsb
chanw:	.word	0
name:	.ascid	"PIPE"
`

// mbxOne is process 1's start: create PIPE (maxmsg %[1]d, bufquo
// %[2]d), keep its channel in R6, and wake process 2.
const mbxOne = `
	pushaq	@#name			; lognam
	pushl	#0			; acmode
	pushl	#0			; promsk
	pushl	#%[2]d			; bufquo
	pushl	#%[1]d			; maxmsg
	pushal	@#chanw			; chan
	pushl	#1			; prmflg: permanent
	calls	#7, @#sys$crembx
	movl	r0, @#data+8
	movzwl	@#chanw, r6
	jsb	@#wake
`

// mbxTwo is process 2's start: wait to be woken (process 1 has made the
// mailbox then), and assign a channel to PIPE, in R6.
const mbxTwo = `
	jsb	@#hiber
	pushl	#0			; mbxnam
	pushl	#0			; acmode
	pushal	@#chanw			; chan
	pushaq	@#name			; devnam
	calls	#4, @#sys$assign
	movl	r0, @#data+8
	movzwl	@#chanw, r6
`

// mbxSymbols are the names the programs use, as MACRO assignments.
func mbxSymbols() string {
	s := fmt.Sprintf("data = ^X%X\n", mbxData)

	for _, name := range []string{
		"IO$_READVBLK", "IO$_WRITEVBLK", "IO$_WRITEOF", "IO$_SETMODE",
		"IO$M_NOW", "IO$M_READATTN", "IO$M_WRTATTN", "IO$M_MB_ROOM_NOTIFY",
	} {
		s += fmt.Sprintf("%s = ^X%X\n", name, vmsdef.Symbols[name])
	}

	return s
}

// mbxPair is a booted machine running two mailbox programs.
type mbxPair struct {
	t        *testing.T
	c        *console.Console
	one, two *corevms.Environment
}

// newMbxPair boots a machine whose process 1 runs mbxOne and then body1,
// and whose process 2 runs mbxTwo and then body2 (each falling into fin
// at its end), with PIPE's maxmsg and bufquo.
func newMbxPair(t *testing.T, maxmsg, bufquo int, body1, body2 string) *mbxPair {
	t.Helper()

	one, _ := assembleAt(t, mbxSymbols()+fmt.Sprintf(mbxOne, maxmsg, bufquo)+body1+mbxCommon)
	two, _ := assembleAt(t, mbxSymbols()+mbxTwo+body2+mbxCommon)

	c, _ := scheduledConsole(t, longQuantum, one)
	p := &mbxPair{t: t, c: c, one: c.RTL}
	p.two = handBuiltProcess(t, c, two)

	setBasePriority(t, p.one, 10)
	setBasePriority(t, p.two, 2)

	setLongword(t, c, p.one, mbxData+4, p.two.Process.PID)
	setLongword(t, c, p.two, mbxData+4, p.one.Process.PID)

	return p
}

// run steps the machine until both programs are done, calling watch (if
// not nil) before each instruction.
func (p *mbxPair) run(watch func()) {
	p.t.Helper()

	for range 100000 {
		if p.at(p.one, 0) == 1 && p.at(p.two, 0) == 1 {
			for _, env := range []*corevms.Environment{p.one, p.two} {
				if st := p.at(env, 8); st != 1 {
					p.t.Fatalf("process %08X's $CREMBX or $ASSIGN returned %08X", env.Process.PID, st)
				}
			}

			return
		}

		if watch != nil {
			watch()
		}

		step(p.t, p.c, 1)
	}

	p.t.Fatalf("not done: process 1 %s at %d, process 2 %s at %d",
		stateOf(p.one), p.at(p.one, 0), stateOf(p.two), p.at(p.two, 0))
}

// at reads the longword at mbxData+off in env's P0.
func (p *mbxPair) at(env *corevms.Environment, off uint32) uint32 {
	p.t.Helper()

	return longwordAt(p.t, p.c, env, mbxData+off)
}

// iosb checks the IOSB at mbxData+off in env's P0.
func (p *mbxPair) iosb(env *corevms.Environment, off uint32, what string, status, count, info uint32) {
	p.t.Helper()

	first := p.at(env, off)
	if st, n, i := first&0xFFFF, first>>16, p.at(env, off+4); st != status || n != count || i != info {
		p.t.Errorf("%s IOSB: %04X, %d bytes, %08X; want %04X, %d, %08X", what, st, n, i, status, count, info)
	}
}

// text reads n bytes at mbxData+off in env's P0.
func (p *mbxPair) text(env *corevms.Environment, off uint32, n int) string {
	p.t.Helper()

	b := make([]byte, n)
	if err := p.c.Mem.LoadIn(p.c.CPU, env.Space.AddressSpace, mbxData+off, b); err != nil {
		p.t.Fatal(err)
	}

	return string(b)
}

var (
	ssNormal    = vmsdef.Symbols["SS$_NORMAL"]
	ssEndOfFile = vmsdef.Symbols["SS$_ENDOFFILE"]
	ssMbFull    = vmsdef.Symbols["SS$_MBFULL"]
	ssCancel    = vmsdef.Symbols["SS$_CANCEL"]
)

// TestMailboxPair_readThenWrite: process 1's $QIOW read waits on the
// empty mailbox; process 2's plain $QIOW write is handed straight to it,
// and both complete, each IOSB with the other process's PID. Process 1
// is seen waiting (LEF), and its read completing preempts process 2
// before process 2's $QIOW has returned.
func TestMailboxPair_readThenWrite(t *testing.T) {
	p := newMbxPair(t, 16, 64, `
	clrl	r1
	movl	#io$_readvblk, r2
	moval	@#data+16, r3
	moval	@#data+32, r4
	movl	#16, r5
	jsb	@#qiow
	movl	r0, @#data+12
`, `
	clrl	r1
	movl	#io$_writevblk, r2
	moval	@#data+16, r3
	moval	@#msg, r4
	movl	#5, r5
	jsb	@#qiow
	movl	r0, @#data+12
	brw	fin
msg:	.ascii	"hello"
`)

	sawLEF, twoFirst := false, false

	p.run(func() {
		if stateOf(p.one) == sched.StateLEF {
			sawLEF = true
		}

		if p.at(p.two, 12) != 0 && p.at(p.one, 12) == 0 {
			twoFirst = true
		}
	})

	if !sawLEF {
		t.Error("process 1 was never seen waiting for its read")
	}

	if twoFirst {
		t.Error("process 2's $QIOW returned before process 1 ran")
	}

	if p.at(p.one, 12) != ssNormal || p.at(p.two, 12) != ssNormal {
		t.Errorf("$QIOW statuses %08X, %08X; want SS$_NORMAL", p.at(p.one, 12), p.at(p.two, 12))
	}

	p.iosb(p.one, 16, "read", ssNormal, 5, p.two.Process.PID)
	p.iosb(p.two, 16, "write", ssNormal, 5, p.one.Process.PID)

	if got := p.text(p.one, 32, 5); got != "hello" {
		t.Errorf("read %q, want \"hello\"", got)
	}
}

// TestMailboxPair_writeThenRead: process 2's plain $QIO write is queued
// and stays pending until process 1 reads its message; process 2 waits
// for it ($WAITFR, LEF) meanwhile, and process 1 wakes by a timer. The
// read completes the write, whose
// IOSB has the reader's PID.
func TestMailboxPair_writeThenRead(t *testing.T) {
	p := newMbxPair(t, 16, 64, mbxTimerWake+`
	jsb	@#hiber			; until process 2 waits, and the timer
	clrl	r1
	movl	#io$_readvblk, r2
	moval	@#data+16, r3
	moval	@#data+48, r4
	movl	#16, r5
	jsb	@#qiow
	movl	r0, @#data+12
	brw	fin
delta:	.quad	-100000000
`, `
	movl	#1, r1
	movl	#io$_writevblk, r2
	moval	@#data+16, r3
	moval	@#msg, r4
	movl	#3, r5
	jsb	@#qio
	movl	r0, @#data+12
	pushl	#1
	calls	#1, @#sys$waitfr
	brw	fin
msg:	.ascii	"abc"
`)

	sawLEF := false

	p.run(func() {
		if p.at(p.one, 12) == 0 && stateOf(p.two) == sched.StateLEF {
			sawLEF = true
		}
	})

	if !sawLEF {
		t.Error("process 2 was never seen waiting for its write to be read")
	}

	p.iosb(p.one, 16, "read", ssNormal, 3, p.two.Process.PID)
	p.iosb(p.two, 16, "write", ssNormal, 3, p.one.Process.PID)

	if got := p.text(p.one, 48, 3); got != "abc" {
		t.Errorf("read %q, want \"abc\"", got)
	}
}

// TestMailboxPair_nowAndEOF: with IO$M_NOW, a read of an empty mailbox
// ends at once with SS$_ENDOFFILE, and a write completes when it's
// queued (its IOSB with no reader's PID). Process 2 then queues a
// message and an end-of-file message, which process 1 reads, each from
// process 2's PID: the second as SS$_ENDOFFILE.
func TestMailboxPair_nowAndEOF(t *testing.T) {
	p := newMbxPair(t, 16, 64, `
	clrl	r1
	movl	#io$_readvblk!io$m_now, r2
	moval	@#data+16, r3
	moval	@#data+48, r4
	movl	#16, r5
	jsb	@#qiow			; empty
	jsb	@#hiber
	movl	#io$_readvblk!io$m_now, r2
	moval	@#data+24, r3
	jsb	@#qiow			; "now"
	movl	#io$_readvblk!io$m_now, r2
	moval	@#data+32, r3
	jsb	@#qiow			; end of file
`, `
	clrl	r1
	movl	#io$_writevblk!io$m_now, r2
	moval	@#data+16, r3
	moval	@#msg, r4
	movl	#3, r5
	jsb	@#qiow
	movl	#io$_writeof!io$m_now, r2
	moval	@#data+24, r3
	jsb	@#qiow
	jsb	@#wake
	brw	fin
msg:	.ascii	"now"
`)
	p.run(nil)

	two := p.two.Process.PID

	p.iosb(p.one, 16, "empty read", ssEndOfFile, 0, 0)
	p.iosb(p.one, 24, "read", ssNormal, 3, two)
	p.iosb(p.one, 32, "end-of-file read", ssEndOfFile, 0, two)
	p.iosb(p.two, 16, "write", ssNormal, 3, 0)
	p.iosb(p.two, 24, "end-of-file write", ssNormal, 0, 0)

	if got := p.text(p.one, 48, 3); got != "now" {
		t.Errorf("read %q, want \"now\"", got)
	}
}

// mbxFull is process 2's program for a full mailbox: (with resource
// wait mode disabled first, if %[1]s) write "abcd", filling the 4-byte
// mailbox, then write "efgh", both IO$M_NOW. Its second write's IOSB is
// at data+24. Process 1 is woken after the second write when that write
// completes at once; when it waits, process 1 wakes by a timer.
const mbxFull = `
	%[1]s
	clrl	r1
	movl	#io$_writevblk!io$m_now, r2
	moval	@#data+16, r3
	moval	@#msg1, r4
	movl	#4, r5
	jsb	@#qiow
	movl	#io$_writevblk!io$m_now, r2
	moval	@#data+24, r3
	moval	@#msg2, r4
	jsb	@#qiow
	movl	r0, @#data+12
	jsb	@#wake
	brw	fin
msg1:	.ascii	"abcd"
msg2:	.ascii	"efgh"
`

// mbxTimerWake schedules a wakeup for this process 10 s on.
const mbxTimerWake = `
	pushl	#0			; reptim
	pushaq	@#delta			; daytim
	pushl	#0			; prcnam
	pushl	#0			; pidadr
	calls	#4, @#sys$schdwk
`

// mbxDrain is process 1's program for a full mailbox: hibernate until a
// wakeup (process 2's, or a timer's 10 s on; at one emulated millisecond
// per instruction, a shorter one could end first), then read with IO$M_NOW
// (IOSB at data+16, data at data+48), and read again, waiting (data+24,
// data+52).
const mbxDrain = mbxTimerWake + `
	jsb	@#hiber
	clrl	r1
	movl	#io$_readvblk!io$m_now, r2
	moval	@#data+16, r3
	moval	@#data+48, r4
	movl	#4, r5
	jsb	@#qiow
	movl	#io$_readvblk%[1]s, r2
	moval	@#data+24, r3
	moval	@#data+52, r4
	jsb	@#qiow
	brw	fin
delta:	.quad	-100000000		; 10 s: the idle loop jumps there
`

// TestMailboxPair_full: a write to a full mailbox. With resource wait
// mode on (VMS's default), process 2 waits in MWAIT (RWMBX) until
// process 1's read makes room, then its write goes to process 1's
// waiting read. With it off ($SETRWM), the write fails at once with
// SS$_MBFULL in its IOSB, and the message is lost.
func TestMailboxPair_full(t *testing.T) {
	t.Run("resource wait", func(t *testing.T) {
		p := newMbxPair(t, 4, 4, fmt.Sprintf(mbxDrain, ""), fmt.Sprintf(mbxFull, ""))

		sawRWMBX := false

		p.run(func() {
			info, _ := p.two.Scheduler().Info(sched.Handle(p.two.Process.PID))
			if info.State == sched.StateMWAIT && info.Resource == sched.ResourceMailbox {
				sawRWMBX = true
			}
		})

		if !sawRWMBX {
			t.Error("process 2 was never seen waiting for room (RWMBX)")
		}

		two := p.two.Process.PID

		p.iosb(p.one, 16, "first read", ssNormal, 4, two)
		p.iosb(p.one, 24, "second read", ssNormal, 4, two)
		p.iosb(p.two, 24, "second write", ssNormal, 4, p.one.Process.PID)

		if got := p.text(p.one, 48, 8); got != "abcdefgh" {
			t.Errorf("read %q, want \"abcdefgh\"", got)
		}
	})

	t.Run("no resource wait", func(t *testing.T) {
		p := newMbxPair(t, 4, 4, fmt.Sprintf(mbxDrain, "!io$m_now"), fmt.Sprintf(mbxFull, `
	pushl	#1
	calls	#1, @#sys$setrwm		; resource wait mode off`))
		p.run(nil)

		p.iosb(p.two, 24, "second write", ssMbFull, 0, 0)

		if p.at(p.two, 12) != ssNormal {
			t.Errorf("the second write's $QIOW returned %08X, want SS$_NORMAL", p.at(p.two, 12))
		}

		p.iosb(p.one, 16, "first read", ssNormal, 4, p.two.Process.PID)
		p.iosb(p.one, 24, "second read", ssEndOfFile, 0, 0)
	})
}

// TestMailboxPair_cancel: process 1's read, queued and pending, is
// cancelled ($CANCEL): its IOSB gets SS$_CANCEL, and process 2's later
// message goes to process 1's next read, not into the cancelled read's
// buffer.
func TestMailboxPair_cancel(t *testing.T) {
	p := newMbxPair(t, 16, 64, `
	movl	#1, r1
	movl	#io$_readvblk, r2
	moval	@#data+16, r3
	moval	@#data+48, r4
	movl	#16, r5
	jsb	@#qio
	pushl	r6
	calls	#1, @#sys$cancel
	movl	r0, @#data+12
	clrl	r1
	moval	@#data+24, r3
	moval	@#data+64, r4
	jsb	@#qiow
`, `
	clrl	r1
	movl	#io$_writevblk!io$m_now, r2
	moval	@#data+16, r3
	moval	@#msg, r4
	movl	#2, r5
	jsb	@#qiow
	brw	fin
msg:	.ascii	"ok"
`)
	p.run(nil)

	if p.at(p.one, 12) != ssNormal {
		t.Errorf("$CANCEL returned %08X", p.at(p.one, 12))
	}

	p.iosb(p.one, 16, "cancelled read", ssCancel, 0, 0)
	p.iosb(p.one, 24, "read", ssNormal, 2, p.two.Process.PID)

	if got := p.text(p.one, 48, 2); got != "\x00\x00" {
		t.Errorf("the cancelled read's buffer has %q", got)
	}

	if got := p.text(p.one, 64, 2); got != "ok" {
		t.Errorf("read %q, want \"ok\"", got)
	}
}

// mbxEnable enables an attention AST (function modifier %[1]s) on the
// mailbox, its routine astr, with parameter 7.
const mbxEnable = `
	clrl	r1
	movl	#io$_setmode!%[1]s, r2
	clrl	r3
	moval	@#astr, r4
	movl	#7, r5
	jsb	@#qiow
	movl	r0, @#data+12
`

// mbxAST is an attention AST routine: record that it ran (with its
// parameter, at data+32), do %[1]s, and wake this process.
const mbxAST = `
	.entry	astr, ^m<r1,r2,r3,r4,r5>
	movl	4(ap), @#data+32
	%[1]s
	pushl	#0
	pushl	#0
	calls	#2, @#sys$wake
	ret
`

// mbxReadNow reads a message with IO$M_NOW (IOSB at data+24, data at
// data+48).
const mbxReadNow = `
	clrl	r1
	movl	#io$_readvblk!io$m_now, r2
	moval	@#data+24, r3
	moval	@#data+48, r4
	movl	#16, r5
	jsb	@#qiow
`

// mbxWriteNow writes "att" with IO$M_NOW (IOSB at data+24).
const mbxWriteNow = `
	clrl	r1
	movl	#io$_writevblk!io$m_now, r2
	moval	@#data+24, r3
	moval	@#att, r4
	movl	#3, r5
	jsb	@#qiow
`

// TestMailboxPair_attention: each attention AST is delivered to the
// process that enabled it, and ends its wait (it hibernates for it):
//
//   - read attention: process 1 enables it and hibernates; process 2's
//     write, which no read waits for, has process 1's AST read it;
//   - write attention: process 2 enables it; process 1's read, waiting
//     on the empty mailbox, has process 2's AST write the message;
//   - room notification: process 2 fills the mailbox and enables it;
//     process 1's read makes room, and process 2's AST writes another
//     message, which process 1's waiting read gets.
func TestMailboxPair_attention(t *testing.T) {
	att := "\natt:\t.ascii\t\"att\"\n"
	readQIOW := strings.Replace(mbxReadNow, "!io$m_now", "", 1)

	tests := []struct {
		name       string
		one, two   string
		enabler    func(p *mbxPair) *corevms.Environment
		readerText uint32 // where process 1's message is
	}{
		{
			name: "read attention",
			one: fmt.Sprintf(mbxEnable, "io$m_readattn") + `
	jsb	@#hiber
	brw	fin
` + fmt.Sprintf(mbxAST, mbxReadNow),
			two:     mbxWriteNow + "\tbrw\tfin\n" + att,
			enabler: func(p *mbxPair) *corevms.Environment { return p.one },
		},
		{
			name: "write attention",
			one:  "\tjsb\t@#hiber\n" + readQIOW,
			two: fmt.Sprintf(mbxEnable, "io$m_wrtattn") + `
	jsb	@#wake
	jsb	@#hiber
	brw	fin
` + fmt.Sprintf(mbxAST, mbxWriteNow) + att,
			enabler: func(p *mbxPair) *corevms.Environment { return p.two },
		},
		{
			name: "room notification",
			one: "\tjsb\t@#hiber\n" + strings.Replace(mbxReadNow, "data+48", "data+64", 1) +
				strings.Replace(readQIOW, "data+24", "data+16", 1),
			two: strings.Replace(mbxWriteNow, "#3", "#4", 1) + fmt.Sprintf(mbxEnable, "io$m_mb_room_notify") + `
	jsb	@#wake
	jsb	@#hiber
	brw	fin
` + fmt.Sprintf(mbxAST, strings.Replace(mbxWriteNow, "data+24", "data+16", 1)) + att,
			enabler: func(p *mbxPair) *corevms.Environment { return p.two },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bufquo := 64
			if tt.name == "room notification" {
				bufquo = 4
			}

			p := newMbxPair(t, 16, bufquo, tt.one, tt.two)
			p.run(nil)

			enabler := tt.enabler(p)
			other := p.one
			if enabler == p.one {
				other = p.two
			}

			if p.at(enabler, 12) != ssNormal {
				t.Errorf("IO$_SETMODE returned %08X", p.at(enabler, 12))
			}

			if p.at(enabler, 32) != 7 {
				t.Errorf("the enabling process's AST ran with %d, want 7 (0: never ran)", p.at(enabler, 32))
			}

			if p.at(other, 32) != 0 {
				t.Error("an AST ran in the other process")
			}

			if got := p.text(p.one, 48, 3); got != "att" {
				t.Errorf("process 1 read %q, want \"att\"", got)
			}
		})
	}
}
