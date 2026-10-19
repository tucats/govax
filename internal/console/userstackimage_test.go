package console_test

import (
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/machine"
	"github.com/tucats/govax/internal/registers"
)

// deepStackSource stops at checkpoint 1 with the stack activation made,
// then writes a longword in each of the 40 pages below its stack pointer,
// one page at a time, and stops at checkpoint 2 with its stack pointer
// back in R2, then ends.
const deepStackSource = `	.title	deep
	.macro	stop,n,?l
	movl	#n,r6
l:	cmpl	r10,r6
	bneq	l
	.endm	stop
	.psect	code,exe,nowrt
	.entry	start,^m<r2,r3>
	clrl	r10
	stop	1
	movl	sp,r2
	movl	#40,r3
10$:	subl2	#512,sp
	movl	r3,(sp)
	sobgtr	r3,10$
	movl	r2,sp
	stop	2
	movl	#1,r0
	ret
	.end	start
`

// TestUserStackImage (Phase 67's subtask 3): with paging on, an image's
// activation creates its user stack, LINK's default 20 pages below
// UserStackTop, charged; the image's stack grows past them a page at a
// time, each page charged; and its rundown deletes the stack and gives
// every charge back.
func TestUserStackImage(t *testing.T) {
	setSetting(t, machine.PagingSetting, "true")

	c, _ := scheduledConsole(t, longQuantum, brbSelf)
	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t), nil)
	env := c.ProcessOne()
	exe := buildImage(t, c, "deep", deepStackSource)

	stackTop := uint32(corevms.UserStackTop)

	// The test console's P0 pages (its spin loop's and its counter's),
	// and the user stack pages it pushed onto, are charged; the image's
	// activation and rundown give them back too (as TestPG66Oracle's
	// harness).
	before := env.PageFileRemaining()

	for va := uint32(codeAddr); va < codeAddr+programExtent(brbSelf); va += 512 {
		if env.PageFileChargedAt(va) {
			before++
		}
	}

	for va := env.FreeP1VA() + 512; va < stackTop; va += 512 {
		if env.PageFileChargedAt(va) {
			before++
		}
	}

	c.Engine.SetLimits(1, 0)
	runImage(t, d, exe)
	c.Engine.SetLimits(0, 0)

	runUntil(t, c, 200_000, func() bool { return c.CPU.GPR(6) == 1 })
	step(t, c, 4)

	sp := c.CPU.GPR(registers.SP)
	if low := env.FreeP1VA() + 512; low != stackTop-20*512 {
		t.Errorf("at the first stop P1 starts at %#x, want 20 pages below UserStackTop, %#x", low, stackTop-20*512)
	}

	for va := stackTop - 20*512; va < stackTop; va += 512 {
		if !env.PageFileChargedAt(va) {
			t.Errorf("stack page %#x isn't charged", va)
		}
	}

	c.CPU.SetGPR(10, 1)
	runUntil(t, c, 200_000, func() bool { return c.CPU.GPR(6) == 2 })
	step(t, c, 4)

	if got := c.CPU.GPR(2); got != sp {
		t.Fatalf("R2 %#x, the stack pointer at the first stop %#x", got, sp)
	}

	want := (sp - 40*512) &^ 511
	if low := env.FreeP1VA() + 512; low != want {
		t.Errorf("after 40 pages of stack P1 starts at %#x, want %#x", low, want)
	}

	for va := want; va < stackTop; va += 512 {
		if !env.PageFileChargedAt(va) {
			t.Errorf("stack page %#x isn't charged", va)
		}
	}

	c.CPU.SetGPR(10, 2)

	for range 200_000 {
		if err := c.StepMachine(); err != nil {
			if err := c.ReportStop(err); err != nil {
				t.Fatalf("at PC %08X: %v", c.CPU.GPR(registers.PC), err)
			}

			break
		}
	}

	if c.ImageActive() {
		t.Fatal("the image didn't end")
	}

	if got := env.PageFileRemaining(); got != before {
		t.Errorf("after the image %d pages of quota left, %d before", got, before)
	}

	for va := want; va < stackTop; va += 512 {
		if env.PageFileChargedAt(va) {
			t.Errorf("stack page %#x still charged after the image", va)
		}
	}
}
