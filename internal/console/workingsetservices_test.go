package console_test

import (
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
	"github.com/tucats/govax/internal/machine"
	"github.com/tucats/govax/internal/paging"
	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/vmsdef"
)

// Phase 63's subtask 6 (docs/PHASE-63 - working sets.md): with paging on,
// the working set services work on the real working set list. $LKWSET
// keeps pages in while others come and go, faulting a page in first if
// it isn't there, and fails with SS$_LKWSETFUL when too few of the list's
// entries would be left unlocked; $PURGWS takes every unlocked page of a
// range out, into transition; $LCKPAG's lock has no such limit; and
// image exit unlocks everything.

// wsServicesSource touches the 20 pages of buf and locks the first 5 in
// the working set (checkpoint 1, R9 the status); sweeps the 80 pages of
// buf2 (checkpoint 2); purges all of P0 and P1 (checkpoint 3, R9 the
// status); unlocks buf's 5 pages (R9 the status) and locks buf2's pages
// one at a time until a lock fails (R11 how many were locked, R5 the
// failing status), then locks that page in memory (R4 the status;
// checkpoint 4); and returns. At each checkpoint it waits in a loop until
// the test sets R10 to the checkpoint's number, R6. R8 is buf's address,
// R7 buf2's.
const wsServicesSource = `	.title	wsserv
	.macro	stop,n,?l
	movl	#n,r6
l:	cmpl	r10,r6
	bneq	l
	.endm	stop
	.psect	data,noexe,wrt,page
buf:	.blkb	512*20
buf2:	.blkb	512*80
range:	.blkl	2
retadr:	.blkl	2
all:	.long	0,^x7FFFFFFF
one:	.blkl	2
	.psect	code,exe,nowrt
	.entry	start,^m<r2,r3,r4,r5,r6,r7,r8,r9,r10,r11>
	clrl	r10
	moval	buf,r8
	moval	buf2,r7
	movl	r8,r2
	movl	#20,r3
	bsbw	touch
	movl	r8,range
	addl3	#512*4,r8,range+4
	$lkwset_s inadr=range,retadr=retadr
	movl	r0,r9
	stop	1
	movl	r7,r2
	movl	#80,r3
	bsbw	touch
	stop	2
	$purgws_s inadr=all
	movl	r0,r9
	stop	3
	$ulwset_s inadr=range
	movl	r0,r9
	clrl	r11
	movl	r7,r2
20$:	movl	r2,one
	movl	r2,one+4
	$lkwset_s inadr=one
	blbc	r0,30$
	incl	r11
	addl2	#512,r2
	brb	20$
30$:	movl	r0,r5
	$lckpag_s inadr=one
	movl	r0,r4
	stop	4
	movl	#1,r0
	ret
touch:	movl	r3,(r2)
	addl2	#512,r2
	sobgtr	r3,touch
	rsb
	.end	start
`

// TestWorkingSetServices runs wsServicesSource with a 30-page working
// set (MINWSCNT 20, so 10 pages can be locked in it) and checks the
// working set at each checkpoint and after the image's exit.
func TestWorkingSetServices(t *testing.T) {
	setSetting(t, machine.PagingSetting, "true")
	setSetting(t, "vax.sysgen.pql_dwsdefault", "30")

	c, out := scheduledConsole(t, longQuantum, brbSelf)
	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t), nil)

	if c.Mem.Paging() == nil {
		t.Fatal("vax.init's VMINIT left paging off")
	}

	env := c.ProcessOne()
	p := env.Process

	if p.WSLimit != 30 || p.MinWSCount != 20 {
		t.Fatalf("WSLIMIT %d, MINWSCNT %d; want 30 and 20", p.WSLimit, p.MinWSCount)
	}

	exe := buildImage(t, c, "wsserv", wsServicesSource)

	c.Engine.SetLimits(1, 0)
	runImage(t, d, exe)
	c.Engine.SetLimits(0, 0)

	if !c.ImageActive() {
		t.Fatalf("the image isn't running\n%s", out.String())
	}

	checkpoint := func(n uint32) {
		t.Helper()

		runUntil(t, c, 200_000, func() bool { return c.CPU.GPR(6) == n })
		step(t, c, 4)
		checkWorkingSets(t, c)
	}

	release := func(n uint32) { c.CPU.SetGPR(10, n) }

	ws := env.WorkingSet()

	// lockedIn reports whether the n pages from va are all in the
	// working set with lock kind.
	lockedIn := func(va uint32, n int, kind paging.Locks) bool {
		for i := range n {
			pg, ok := ws.Lookup(va + uint32(i)*512)
			if !ok || pg.Locks&kind == 0 || !pteIn(t, c, va+uint32(i)*512).Valid() {
				return false
			}
		}

		return true
	}

	// inSet counts the n pages from va that are in the working set.
	inSet := func(va uint32, n int) int {
		count := 0

		for i := range n {
			if _, ok := ws.Lookup(va + uint32(i)*512); ok {
				count++
			}
		}

		return count
	}

	// Checkpoint 1: buf's first 5 pages locked.
	checkpoint(1)

	buf, buf2 := c.CPU.GPR(8), c.CPU.GPR(7)

	if got := c.CPU.GPR(9); got != vmsdef.Symbols["SS$_WASCLR"] {
		t.Errorf("checkpoint 1: $LKWSET = %#x, want SS$_WASCLR", got)
	}

	if !lockedIn(buf, 5, paging.LockedInWorkingSet) || ws.Counts().Locked != 5 {
		t.Errorf("checkpoint 1: %d pages locked, buf's 5 locked %v", ws.Counts().Locked, lockedIn(buf, 5, paging.LockedInWorkingSet))
	}

	// Checkpoint 2: 80 pages through the working set leave buf's locked
	// pages in it, and take the rest of buf out.
	release(1)
	checkpoint(2)

	if !lockedIn(buf, 5, paging.LockedInWorkingSet) {
		t.Error("checkpoint 2: buf's locked pages left the working set")
	}

	if n := len(pagesInTransition(t, c, buf+5*512, 15, paging.Modified)); n != 15 {
		t.Errorf("checkpoint 2: %d of buf's 15 unlocked pages in transition, want 15", n)
	}

	// Checkpoint 3: the purge takes every unlocked page out but those the
	// calling instruction used.
	release(2)
	checkpoint(3)

	if got := c.CPU.GPR(9); got != vmsdef.Symbols["SS$_NORMAL"] {
		t.Errorf("checkpoint 3: $PURGWS = %#x, want SS$_NORMAL", got)
	}

	if !lockedIn(buf, 5, paging.LockedInWorkingSet) {
		t.Error("checkpoint 3: the purge took buf's locked pages")
	}

	if n := inSet(buf2, 80); n != 0 {
		t.Errorf("checkpoint 3: %d of buf2's pages in the working set, want none", n)
	}

	t.Logf("after the purge: %d pages in the working set, 5 of them locked", ws.InUse())

	if ws.InUse() > 10 {
		t.Errorf("checkpoint 3: %d pages in the working set after the purge, want 10 at most", ws.InUse())
	}

	// Checkpoint 4: buf's pages unlocked (each was locked: SS$_WASSET);
	// 10 of buf2's purged pages faulted in and locked, the 11th
	// SS$_LKWSETFUL; and that page locked in memory, which has no limit.
	release(3)
	checkpoint(4)

	if got := c.CPU.GPR(9); got != vmsdef.Symbols["SS$_WASSET"] {
		t.Errorf("checkpoint 4: $ULWSET = %#x, want SS$_WASSET", got)
	}

	if got := c.CPU.GPR(11); got != 10 {
		t.Errorf("checkpoint 4: %d pages locked one at a time, want 10", got)
	}

	if got := c.CPU.GPR(5); got != vmsdef.Symbols["SS$_LKWSETFUL"] {
		t.Errorf("checkpoint 4: the 11th $LKWSET = %#x, want SS$_LKWSETFUL", got)
	}

	if got := c.CPU.GPR(4); got != vmsdef.Symbols["SS$_WASCLR"] {
		t.Errorf("checkpoint 4: $LCKPAG = %#x, want SS$_WASCLR", got)
	}

	if !lockedIn(buf2, 10, paging.LockedInWorkingSet) || !lockedIn(buf2+10*512, 1, paging.LockedInMemory) {
		t.Error("checkpoint 4: buf2's first 11 pages aren't locked as expected")
	}

	if got := ws.Counts().Locked; got != 11 {
		t.Errorf("checkpoint 4: %d pages locked, want 11", got)
	}

	// Image exit unlocks every page.
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

	if got := ws.Counts().Locked; got != 0 {
		t.Errorf("after image exit: %d pages locked, want none", got)
	}

	checkWorkingSets(t, c)
}
