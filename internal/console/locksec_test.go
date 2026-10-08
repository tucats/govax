package console_test

import (
	"fmt"
	"testing"

	"github.com/tucats/govax/internal/sched"
	"github.com/tucats/govax/internal/vmsdef"
)

// Phase 47's subtask 7: gblpair_test.go's counter in a global section,
// guarded by the lock manager instead of a spinlock. Each process, gblTurns
// times, takes an EX lock on COUNTERLOCK with $ENQW, reads the counter,
// adds 1 over a few instructions, stores it, and $DEQs the lock. With a
// 5-instruction quantum each is often switched out holding the lock, and
// the other's $ENQW then waits (LEF) until the $DEQ grants it.

// enqLoop is the counting loop for the section at base; a failing $ENQW
// or $DEQ stores its status at data+0x10 and stops.
const enqLoop = `
	movl	#%[2]d, r7
take:	clrl	-(sp)			; acmode
	clrq	-(sp)			; astprm, blkast
	clrq	-(sp)			; parid, astadr
	pushaq	@#lcknam		; resnam
	pushl	#0			; flags
	pushal	@#data+^X40		; lksb
	pushl	#5			; lkmode: EX
	pushl	#0			; efn
	calls	#10, @#sys$enqw
	blbc	r0, bad
	movzwl	@#data+^X40, r0
	blbc	r0, bad
	movl	@#^X%[1]X+4, r0
	movl	r0, r1
	incl	r1
	movl	r1, r0
	movl	r0, @#^X%[1]X+4
	clrq	-(sp)			; acmode, flags
	pushl	#0			; valblk
	pushl	@#data+^X44		; lkid
	calls	#4, @#sys$deq
	blbc	r0, bad
	sobgtr	r7, take
	brw	fin
bad:	movl	r0, @#data+^X10
	brw	fin
lcknam:	.ascid	"COUNTERLOCK"
`

// TestLockedSection_enqw: every increment lands; the processes did wait
// for each other's locks.
func TestLockedSection_enqw(t *testing.T) {
	flags := vmsdef.LibrarySymbols["SEC$M_GBL"] | vmsdef.LibrarySymbols["SEC$M_PAGFIL"] |
		vmsdef.LibrarySymbols["SEC$M_WRT"]

	one, _ := assembleAt(t, mbxSymbols()+`
	clrq	-(sp)			; prot, pfc
	clrl	-(sp)			; vbn
	pushl	#1			; pagcnt
	clrq	-(sp)			; relpag, chan
	pushl	#0			; ident
`+fmt.Sprintf(gblArgs, flags)+`
	calls	#12, @#sys$crmpsc
	movl	r0, @#data+8
	jsb	@#wake
`+fmt.Sprintf(enqLoop, gblOne, gblTurns)+fmt.Sprintf(`
range:	.long	^X%[1]X, ^X%[1]X
`, gblOne)+mbxCommon+gblCommon)

	two, _ := assembleAt(t, mbxSymbols()+`
	jsb	@#hiber
	clrq	-(sp)			; relpag, ident
`+fmt.Sprintf(gblArgs, vmsdef.LibrarySymbols["SEC$M_WRT"])+`
	calls	#7, @#sys$mgblsc
	movl	r0, @#data+8
`+fmt.Sprintf(enqLoop, gblTwo, gblTurns)+fmt.Sprintf(`
range:	.long	^X%[1]X, ^X%[1]X
`, gblTwo)+mbxCommon+gblCommon)

	c, _ := scheduledConsole(t, "5", one)
	p := &mbxPair{t: t, c: c, one: c.RTL}
	p.two = handBuiltProcess(t, c, two)

	setLongword(t, c, p.one, mbxData+4, p.two.Process.PID)
	setLongword(t, c, p.two, mbxData+4, p.one.Process.PID)
	setBasePriority(t, p.one, 4)
	setBasePriority(t, p.two, 4)

	waits := 0

	for range 2000000 {
		if p.at(p.one, 0) == 1 && p.at(p.two, 0) == 1 {
			break
		}

		if stateOf(p.one) == sched.StateLEF || stateOf(p.two) == sched.StateLEF {
			waits++
		}

		step(t, c, 1)
	}

	if p.at(p.one, 0) != 1 || p.at(p.two, 0) != 1 {
		t.Fatalf("not done: process 1 %s, process 2 %s", stateOf(p.one), stateOf(p.two))
	}

	if st := p.at(p.one, 0x10); st != 0 {
		t.Errorf("process 1's lock service failed: %08X", st)
	}

	if st := p.at(p.two, 0x10); st != 0 {
		t.Errorf("process 2's lock service failed: %08X", st)
	}

	if waits == 0 {
		t.Error("neither process ever waited for the lock: the test proved nothing")
	}

	want := uint32(2 * gblTurns)
	if a := longwordAt(t, c, p.one, gblOne+4); a != want {
		t.Errorf("counter %d; want %d", a, want)
	}

}
