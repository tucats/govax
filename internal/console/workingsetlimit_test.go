package console_test

import (
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
	"github.com/tucats/govax/internal/machine"
	"github.com/tucats/govax/internal/paging"
	"github.com/tucats/govax/internal/registers"
)

// Phase 63's subtask 4 (docs/PHASE-63 - working sets.md): with paging on,
// $ADJWSL changes the size of the working set list itself, a smaller
// limit taking the oldest pages over it out of the set; a larger limit
// is room the next faults use at once; and image exit puts the limit back
// at the process's default (DFWSCNT), taking out the pages over it.

// wsDefault is the default working set the test gives process 1, through
// PQL_DWSDEFAULT.
const wsDefault = 60

// adjwslSource touches each of 80 pages of an array, then stops at
// checkpoint 1; shrinks its working set by 30 pages with $ADJWSL
// (checkpoint 2, R9 the new limit); grows it by 100 and touches the
// array again (checkpoint 3, R9 the new limit); touches it once more
// (checkpoint 4); and returns. At each checkpoint it waits in a loop until
// the test sets R10 to the checkpoint's number, R6. R8 is the array's
// address.
const adjwslSource = `	.title	adjwsl
	.macro	stop,n,?l
	movl	#n,r6
l:	cmpl	r10,r6
	bneq	l
	.endm	stop
	.psect	data,noexe,wrt,page
buf:	.blkb	512*80
limit:	.long	0
	.psect	code,exe,nowrt
	.entry	start,^m<r2,r3,r4,r5,r6,r7,r8,r9,r10,r11>
	clrl	r10
	moval	buf,r8
	bsbw	touch
	stop	1
	pushal	limit
	mnegl	#30,-(sp)
	calls	#2,g^sys$adjwsl
	movl	limit,r9
	stop	2
	pushal	limit
	pushl	#100
	calls	#2,g^sys$adjwsl
	movl	limit,r9
	bsbw	touch
	stop	3
	bsbw	touch
	stop	4
	movl	#1,r0
	ret
touch:	moval	buf,r2
	movl	#80,r3
10$:	movl	r3,(r2)
	addl2	#512,r2
	sobgtr	r3,10$
	rsb
	.end	start
`

// TestAdjwslWorkingSet runs adjwslSource with a 60-page working set and
// checks the working set at each checkpoint and after the image's exit.
func TestAdjwslWorkingSet(t *testing.T) {
	setSetting(t, machine.PagingSetting, "true")
	setSetting(t, "vax.sysgen.pql_dwsdefault", "60")

	c, out := scheduledConsole(t, longQuantum, brbSelf)
	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t), nil)

	if c.Mem.Paging() == nil {
		t.Fatal("vax.init's VMINIT left paging off")
	}

	env := c.ProcessOne()
	p := env.Process

	if p.WSDefault != wsDefault || p.WSLimit != wsDefault {
		t.Fatalf("DFWSCNT %d, WSLIMIT %d; want %d", p.WSDefault, p.WSLimit, wsDefault)
	}

	exe := buildImage(t, c, "adjwsl", adjwslSource)

	// Stop RUN at once, inside the image, and drive it from here.
	c.Engine.SetLimits(1, 0)
	runImage(t, d, exe)
	c.Engine.SetLimits(0, 0)

	if !c.ImageActive() {
		t.Fatalf("the image isn't running\n%s", out.String())
	}

	// checkpoint runs the program until it waits at checkpoint n, and
	// a few instructions more, so that a working set over its limit has
	// been trimmed at an instruction boundary.
	checkpoint := func(n uint32) {
		t.Helper()

		runUntil(t, c, 200_000, func() bool { return c.CPU.GPR(6) == n })
		step(t, c, 4)
		checkWorkingSets(t, c)
	}

	release := func(n uint32) { c.CPU.SetGPR(10, n) }

	ws := env.WorkingSet()

	// Checkpoint 1: 80 pages through a 60-page working set.
	checkpoint(1)

	array := c.CPU.GPR(8)

	if ws.Size() != wsDefault || ws.InUse() > wsDefault {
		t.Errorf("checkpoint 1: size %d, %d in use; want %d at most", ws.Size(), ws.InUse(), wsDefault)
	}

	if n := len(pagesInTransition(t, c, array, 80, paging.Modified)); n < 80-wsDefault {
		t.Errorf("checkpoint 1: %d of the array's pages in transition, want at least %d", n, 80-wsDefault)
	}

	// Checkpoint 2: $ADJWSL -30 shrinks the list to 30, taking out the
	// pages over it.
	release(1)
	checkpoint(2)

	if got := c.CPU.GPR(9); got != wsDefault-30 || p.WSLimit != got || ws.Size() != int(got) {
		t.Errorf("checkpoint 2: wsetlm %d, WSLIMIT %d, list size %d; want %d", got, p.WSLimit, ws.Size(), wsDefault-30)
	}

	if ws.InUse() > wsDefault-30 || ws.Over() {
		t.Errorf("checkpoint 2: %d pages in use (over %v), want %d at most", ws.InUse(), ws.Over(), wsDefault-30)
	}

	if n := len(pagesInTransition(t, c, array, 80, paging.Modified)); n < 80-(wsDefault-30) {
		t.Errorf("checkpoint 2: %d of the array's pages in transition, want at least %d", n, 80-(wsDefault-30))
	}

	// Checkpoint 3: $ADJWSL +100 grows the list to 130, and the array's
	// next sweep fits in it whole.
	release(2)
	checkpoint(3)

	if got := c.CPU.GPR(9); got != wsDefault+70 || p.WSLimit != got || ws.Size() != int(got) {
		t.Errorf("checkpoint 3: wsetlm %d, WSLIMIT %d, list size %d; want %d", got, p.WSLimit, ws.Size(), wsDefault+70)
	}

	if n := len(pagesInTransition(t, c, array, 80, paging.Modified)); n != 0 {
		t.Errorf("checkpoint 3: %d of the array's pages in transition, want none", n)
	}

	// Checkpoint 4: a sweep of pages all in the working set faults on none.
	faults := p.PageFaults

	release(3)
	checkpoint(4)

	if p.PageFaults != faults {
		t.Errorf("checkpoint 4: the second sweep counted %d faults, want 0", p.PageFaults-faults)
	}

	// Image exit puts the limit back at DFWSCNT. The image's return ends
	// the run as the console's run loop sees it (ReportStop), which runs
	// image rundown.
	release(4)

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

	if p.WSLimit != wsDefault || ws.Size() != wsDefault || ws.InUse() > wsDefault {
		t.Errorf("after image exit: WSLIMIT %d, list size %d, %d in use; want %d", p.WSLimit, ws.Size(), ws.InUse(), wsDefault)
	}

	checkWorkingSets(t, c)
}
