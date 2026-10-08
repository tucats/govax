package console_test

import (
	"testing"

	"github.com/tucats/govax/internal/sched"
	"github.com/tucats/govax/internal/vmsdef"
)

// Phase 46's subtask 4: common event flags between processes, on a
// booted machine, with mbxpair_test.go's harness (process 1 at base
// priority 10, process 2 at 2; each program's results in its own P0 at
// mbxData). Both processes associate the temporary cluster SHARED with
// cluster 2 (flags 64-95), and the $ASCEFC status goes at mbxData+8.

// cefStart is each process's association with SHARED.
const cefStart = `
	pushl	#0			; perm
	pushl	#0			; prot
	pushaq	@#cefnam		; name
	pushl	#64			; efn: cluster 2
	calls	#4, @#sys$ascefc
	movl	r0, @#data+8
`

// cefCommon is the cluster's name, after mbxCommon.
const cefCommon = `
cefnam:	.ascid	"SHARED"
`

// TestCommonFlagPair_handshake: process 1 waits for flag 65 (CEF);
// process 2's $SETEF of it lets process 1 run before process 2 goes on
// past its $SETEF. Process 1 then hibernates until a timer's wakeup, so
// that process 2 waits for flag 66 (CEF), sets 66, and waits for flags
// 67 and 68 together ($WFLAND); process 2, woken by flag 66, sets 67
// (not enough), and then a $GETJPIW completing with flag 68 sets that,
// and process 1 goes on at once: not only $SETEF reports a common flag.
func TestCommonFlagPair_handshake(t *testing.T) {
	one, _ := assembleAt(t, mbxSymbols()+cefStart+`
	jsb	@#wake
	pushl	#65
	calls	#1, @#sys$waitfr
	movl	r0, @#data+12
`+mbxTimerWake+`
	jsb	@#hiber
	pushl	#66
	calls	#1, @#sys$setef
	pushl	#^X18			; flags 67 and 68
	pushl	#64
	calls	#2, @#sys$wfland
	movl	r0, @#data+16
	brw	fin
delta:	.quad	-100000000		; 10 s: the idle loop jumps there
`+mbxCommon+cefCommon)

	two, _ := assembleAt(t, mbxSymbols()+`
	jsb	@#hiber
`+cefStart+`
	pushl	#65
	calls	#1, @#sys$setef
	movl	r0, @#data+12
	pushl	#66
	calls	#1, @#sys$waitfr
	pushl	#67
	calls	#1, @#sys$setef
	movl	#1, @#data+16		; flag 67 is set
	clrq	-(sp)			; astadr, astprm
	clrl	-(sp)			; iosb
	pushal	@#nolist		; itmlst: empty
	clrq	-(sp)			; pidadr, prcnam: this process
	pushl	#68			; efn
	calls	#7, @#sys$getjpiw
	movl	r0, @#data+20
	brw	fin
nolist:	.long	0
`+mbxCommon+cefCommon)

	c, _ := scheduledConsole(t, longQuantum, one)
	p := &mbxPair{t: t, c: c, one: c.RTL}
	p.two = handBuiltProcess(t, c, two)

	setBasePriority(t, p.one, 10)
	setBasePriority(t, p.two, 2)

	setLongword(t, c, p.one, mbxData+4, p.two.Process.PID)
	setLongword(t, c, p.two, mbxData+4, p.one.Process.PID)

	sawOneCEF, sawTwoCEF, twoFirst, oneEarly, twoLate := false, false, false, false, false

	p.run(func() {
		if stateOf(p.one) == sched.StateCEF {
			sawOneCEF = true
		}

		if stateOf(p.two) == sched.StateCEF {
			sawTwoCEF = true
		}

		// Process 2 is past its first $SETEF, but process 1 hasn't run.
		if p.at(p.two, 12) != 0 && p.at(p.one, 12) == 0 {
			twoFirst = true
		}

		// Process 1's $WFLAND ended with only flag 67 set: process 2
		// marks data+16 between its two $SETEFs.
		if p.at(p.one, 16) != 0 && p.at(p.two, 16) == 0 {
			oneEarly = true
		}

		// Process 2 is past its $GETJPIW, but process 1 hasn't run.
		if p.at(p.two, 20) != 0 && p.at(p.one, 16) == 0 {
			twoLate = true
		}
	})

	if !sawOneCEF || !sawTwoCEF {
		t.Errorf("seen in CEF: process 1 %v, process 2 %v; want both", sawOneCEF, sawTwoCEF)
	}

	if twoFirst || twoLate {
		t.Errorf("process 2 went on past its $SETEF of 65 (%v) or $GETJPIW (%v) before process 1 ran", twoFirst, twoLate)
	}

	if oneEarly {
		t.Error("process 1's $WFLAND ended before both its flags were set")
	}

	if p.at(p.one, 12) != ssNormal || p.at(p.one, 16) != ssNormal {
		t.Errorf("process 1's waits: %08X, %08X; want SS$_NORMAL", p.at(p.one, 12), p.at(p.one, 16))
	}

	if st := p.at(p.two, 12); st != vmsdef.Symbols["SS$_WASCLR"] {
		t.Errorf("process 2's $SETEF 65: %08X, want SS$_WASCLR", st)
	}
}
