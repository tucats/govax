package console_test

import (
	"fmt"
	"testing"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/vmsdef"
)

// Phase 46's subtask 5: a global section shared by two processes, on a
// booted machine, with mbxpair_test.go's harness. Process 1 creates the
// section COUNTER at gblOne; process 2 maps it at gblTwo, another
// address in its own P0. Each adds 1 to the counter in the section's
// second longword gblTurns times, the read, add, and store guarded by a
// spinlock in bit 0 of its first longword (BBSSI to take it, BBCCI to
// give it back). The two run at the same priority with a 5-instruction
// quantum, so each is often switched out in the middle of its
// read-add-store; without the lock, increments would be lost.

// Where each process maps the section, and how many times each adds 1.
const (
	gblOne   = 0x6000
	gblTwo   = 0x7000
	gblTurns = 300
)

// gblLoop is the counting loop, for the section at base: with the lock
// held, it reads the counter, adds 1 over a few instructions (a wider
// window for a switch), and stores it.
const gblLoop = `
	movl	#%[2]d, r7
take:	bbssi	#0, @#^X%[1]X, take	; spin until the lock is ours
	movl	@#^X%[1]X+4, r0
	movl	r0, r1
	incl	r1
	movl	r1, r0
	movl	r0, @#^X%[1]X+4
	bbcci	#0, @#^X%[1]X, free
free:	sobgtr	r7, take
	brw	fin
`

// gblArgs pushes $CRMPSC's or $MGBLSC's first arguments, for a mapping
// of the page range at range.
const gblArgs = `
	pushaq	@#secnam		; gsdnam
	pushl	#^X%X			; flags
	pushl	#0			; acmode
	pushl	#0			; retadr
	pushaq	@#range			; inadr
`

// gblCommon is what both programs have after mbxCommon.
const gblCommon = `
secnam:	.ascid	"COUNTER"
`

// TestGlobalSectionPair_spinlock: both processes' increments all land
// in the one counter, seen at both addresses; process 2's deletion takes
// its mappings away, leaving process 1's.
func TestGlobalSectionPair_spinlock(t *testing.T) {
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
`+fmt.Sprintf(gblLoop, gblOne, gblTurns)+fmt.Sprintf(`
range:	.long	^X%[1]X, ^X%[1]X
`, gblOne)+mbxCommon+gblCommon)

	two, _ := assembleAt(t, mbxSymbols()+`
	jsb	@#hiber
	clrq	-(sp)			; relpag, ident
`+fmt.Sprintf(gblArgs, vmsdef.LibrarySymbols["SEC$M_WRT"])+`
	calls	#7, @#sys$mgblsc
	movl	r0, @#data+8
`+fmt.Sprintf(gblLoop, gblTwo, gblTurns)+fmt.Sprintf(`
range:	.long	^X%[1]X, ^X%[1]X
`, gblTwo)+mbxCommon+gblCommon)

	c, _ := scheduledConsole(t, "5", one)
	p := &mbxPair{t: t, c: c, one: c.RTL}
	p.two = handBuiltProcess(t, c, two)

	setLongword(t, c, p.one, mbxData+4, p.two.Process.PID)
	setLongword(t, c, p.two, mbxData+4, p.one.Process.PID)

	// Both at the same priority, so they share the CPU a quantum each.
	setBasePriority(t, p.one, 4)
	setBasePriority(t, p.two, 4)

	switches, last := 0, uint32(0)

	for range 200000 {
		if p.at(p.one, 0) == 1 && p.at(p.two, 0) == 1 {
			break
		}

		// Count the times the CPU passes from one process to the
		// other while the lock is held.
		if cur := c.RTL.Current().Process.PID; cur != last {
			if last != 0 && longwordAt(t, c, p.one, gblOne)&1 != 0 {
				switches++
			}

			last = cur
		}

		step(t, c, 1)
	}

	if st := p.at(p.one, 8); st != vmsdef.Symbols["SS$_CREATED"] {
		t.Fatalf("$CRMPSC: %08X, want SS$_CREATED", st)
	}

	if st := p.at(p.two, 8); st != vmsdef.Symbols["SS$_NORMAL"] {
		t.Fatalf("$MGBLSC: %08X, want SS$_NORMAL", st)
	}

	if p.at(p.one, 0) != 1 || p.at(p.two, 0) != 1 {
		t.Fatalf("not done: process 1 %s, process 2 %s", stateOf(p.one), stateOf(p.two))
	}

	if switches == 0 {
		t.Error("no process switch while the lock was held: the test proved nothing")
	}

	want := uint32(2 * gblTurns)
	if a, b := longwordAt(t, c, p.one, gblOne+4), longwordAt(t, c, p.two, gblTwo+4); a != want || b != want {
		t.Errorf("counter %d (process 1), %d (process 2); want %d", a, b, want)
	}

	var sec *corevms.GlobalSection

	for _, s := range c.RTL.Sections.Sections() {
		if s.Name == "COUNTER" {
			sec = s
		}
	}

	if sec == nil || sec.Refs != 2 {
		t.Fatalf("COUNTER: %+v, want 2 references", sec)
	}

	// Process 2's deletion (with the CPU in process 1) takes its
	// mapping away.
	if err := c.RTL.System.SwitchCPU(c.Engine, p.one); err != nil {
		t.Fatal(err)
	}

	c.RTL.System.DeleteProcess(p.two)

	if sec.Refs != 1 || len(sec.Frames) != 1 {
		t.Errorf("after process 2's deletion: %+v, want 1 reference", sec)
	}

	if v := longwordAt(t, c, p.one, gblOne+4); v != want {
		t.Errorf("process 1 reads %d after process 2's deletion", v)
	}
}
