package console_test

import (
	"io"
	"path/filepath"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/machine"
	"github.com/tucats/govax/internal/paging"
)

// Phase 64's subtask 4 (docs/PHASE-64 - page file.md): a page deleted
// gives its page file block back, wherever the page holds it: in its PTE
// (the page file form, its frame gone to another page), or as the
// backing store of the frame it still has, valid (read back from the
// page file, unmodified) or in transition on the free list (written,
// not yet reused). Image rundown is TestPageFileWriter's last check.

// pageFileSettings configures paging with a 30-page working set, the
// writer's thresholds at 10 and 2, clusters of up to 16 pages, and a
// 1000-block host page file in the test's directory.
func pageFileSettings(t *testing.T) {
	t.Helper()

	setSetting(t, machine.PagingSetting, "true")
	setSetting(t, "vax.sysgen.pql_dwsdefault", "30")
	setSetting(t, "vax.sysgen.mpw_hilimit", "10")
	setSetting(t, "vax.sysgen.mpw_lolimit", "2")
	setSetting(t, machine.WriteClusterSetting, "16")
	setSetting(t, "vax.sysgen.paging.file", `"`+filepath.Join(t.TempDir(), "pagefile.sys")+`"`)
	setSetting(t, "vax.sysgen.paging.size", "1000")
}

// deltvaSource fills each of the 80 pages of an array (checkpoint 1);
// reads the first 20 back (checkpoint 2); and deletes the whole array
// with $DELTVA, R9 its status (checkpoint 3). At each checkpoint it waits
// in a loop until the test sets R10 to the checkpoint's number, R6. R8 is
// the array's address.
const deltvaSource = `	.title	pfdel
	.macro	stop,n,?l
	movl	#n,r6
l:	cmpl	r10,r6
	bneq	l
	.endm	stop
	.psect	data,noexe,wrt,page
buf:	.blkb	512*80
inadr:	.address buf,buf+<512*80>-1
	.psect	code,exe,nowrt
	.entry	start,^m<r2,r3,r4,r5,r6,r7,r8,r9,r10,r11>
	clrl	r10
	moval	buf,r8
	moval	buf,r2
	movl	#80,r3
10$:	movl	r3,(r2)
	addl2	#512,r2
	sobgtr	r3,10$
	stop	1
	moval	buf,r2
	movl	#20,r3
20$:	tstl	(r2)
	addl2	#512,r2
	sobgtr	r3,20$
	stop	2
	$deltva_s inadr=inadr
	movl	r0,r9
	stop	3
	movl	#1,r0
	ret
	.end	start
`

// TestDeleteGivesPageFileBlocksBack runs deltvaSource: once the array's
// pages are written and their frames reused, some pages hold their
// blocks in page file PTEs and the 20 read back hold them as their valid
// frames' backing store; $DELTVA gives every block back.
func TestDeleteGivesPageFileBlocksBack(t *testing.T) {
	pageFileSettings(t)

	c, out := scheduledConsole(t, longQuantum, brbSelf)
	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t), nil)
	db := c.Mem.Paging()

	exe := buildImage(t, c, "pfdel", deltvaSource)

	c.Engine.SetLimits(1, 0)
	runImage(t, d, exe)
	c.Engine.SetLimits(0, 0)

	if !c.ImageActive() {
		t.Fatalf("the image isn't running\n%s", out.String())
	}

	checkpoint := func(n uint32) {
		t.Helper()

		runUntil(t, c, 400_000, func() bool { return c.CPU.GPR(6) == n })
		step(t, c, 4)
		checkWorkingSets(t, c)
	}

	release := func(n uint32) { c.CPU.SetGPR(10, n) }

	checkpoint(1)

	pf := c.RTL.PageFile()
	if pf == nil {
		t.Fatalf("checkpoint 1: no page file open\n%s", out.String())
	}

	array := c.CPU.GPR(8)

	stealFreeFrames(t, c)

	inPTEs := 0

	for i := range uint32(pfPages) {
		if vbn, ok := pteIn(t, c, array+i*512).PageFileBlock(); ok && vbn != 0 {
			inPTEs++
		}
	}

	release(1)
	checkpoint(2)

	inFrames := 0

	for i := range uint32(20) {
		pte := pteIn(t, c, array+i*512)
		if !pte.Valid() {
			continue
		}

		if e, err := db.Entry(paging.PFN(pte.PFN())); err == nil && e.Backing.Kind == paging.PageFile && e.Backing.Block != 0 {
			inFrames++
		}
	}

	if inPTEs == 0 || inFrames == 0 {
		t.Fatalf("blocks held in %d PTEs and %d valid frames, want some of each", inPTEs, inFrames)
	}

	if pf.Map.UsedBlocks() == 0 {
		t.Fatal("checkpoint 2: no page file blocks in use")
	}

	checkPageFileHolders(t, c, "checkpoint 2")

	release(2)
	checkpoint(3)

	if got := c.CPU.GPR(9); got != 1 {
		t.Fatalf("$DELTVA = %#x", got)
	}

	// The blocks still in use are other pages' (the image's code and
	// stack, the page after the array): none is an array page's, and none
	// is held by no page.
	checkPageFileHolders(t, c, "after $DELTVA")

	if err := pf.Map.Check(); err != nil {
		t.Fatal(err)
	}

	release(3)
}

// checkPageFileHolders checks that the page file's blocks in use are
// exactly those process 1's pages and the global sections' pages hold,
// each by one page: in a page file PTE, or as the backing store of the
// frame a valid or transition PTE names, tied to the page; for a
// page-file global section's page, in the section's block for a page
// that's out, or as its frame's backing store.
func checkPageFileHolders(t *testing.T, c *console.Console, label string) {
	t.Helper()

	db := c.Mem.Paging()
	pf := c.RTL.PageFile()
	space := c.ProcessOne().Space
	held := map[uint32]uint32{}

	hold := func(vbn, va uint32) {
		if other, ok := held[vbn]; ok {
			t.Errorf("%s: page file block %d held by the pages at %08X and %08X", label, vbn, other, va)
		}

		held[vbn] = va
	}

	look := func(va uint32) {
		pte, err := c.Mem.LookupPTEIn(c.CPU, space.AddressSpace, va)
		if err != nil {
			return
		}

		if vbn, ok := pte.PageFileBlock(); ok {
			if vbn != 0 {
				hold(vbn, va)
			}

			return
		}

		pfn, ok := pte.Transition()
		if pte.Valid() {
			pfn, ok = pte.PFN(), true
		}

		if !ok {
			return
		}

		e, err := db.Entry(paging.PFN(pfn))
		if err == nil && e.Tied && e.Owner == (paging.Owner{Handle: c.ProcessOne().Process.PID, VA: va}) &&
			e.Backing.Kind == paging.PageFile && e.Backing.Block != 0 {
			hold(e.Backing.Block, va)
		}
	}

	for i := range space.AddressSpace.P0LR {
		look(i * 512)
	}

	for i := space.AddressSpace.P1LR; i < 1<<21; i++ {
		look(0x40000000 + i*512)
	}

	for _, s := range append(c.RTL.Sections.Sections(), c.RTL.Sections.PendingSections()...) {
		for i, vbn := range s.PageFileBlocks() {
			if vbn != 0 {
				hold(vbn, uint32(i)*512)
			}

			if s.Frames[i] == 0 {
				continue
			}

			if e, err := db.Entry(paging.PFN(s.Frames[i])); err == nil && e.Tied && e.Type == paging.GlobalPage &&
				e.Backing.Kind == paging.PageFile && e.Backing.Block != 0 {
				hold(e.Backing.Block, uint32(i)*512)
			}
		}
	}

	// The page file opens at its first need: until then, no page holds a
	// block.
	if pf == nil {
		if len(held) != 0 {
			t.Errorf("%s: pages hold %d page file blocks, and there's no page file", label, len(held))
		}

		return
	}

	for vbn := range held {
		if !pf.Map.Allocated(vbn) {
			t.Errorf("%s: the page at %08X holds page file block %d, which is free", label, held[vbn], vbn)
		}
	}

	if used := pf.Map.UsedBlocks(); used != uint32(len(held)) {
		t.Errorf("%s: %d page file blocks in use, %d held by pages", label, used, len(held))
	}
}

// TestDeleteProcessGivesPageFileBlocksBack touches 80 pages of a second
// process with a 30-page working set and the page file open, so the
// writer writes the pages that leave it, which wait in transition on the
// free list with their blocks; removing the process gives every block
// back.
func TestDeleteProcessGivesPageFileBlocksBack(t *testing.T) {
	pageFileSettings(t)

	c, _ := scheduledConsole(t, longQuantum, brbSelf)
	sys := c.RTL.System

	pf := sys.OpenPageFile()
	if pf == nil {
		t.Fatal("the page file didn't open")
	}

	env, err := corevms.NewEnvironment(sys, c.Logicals, nil, io.Discard)
	if err != nil {
		t.Fatal(err)
	}

	if env.Space, err = sys.BuildAddressSpace(env.Process.PID, 128, 64); err != nil {
		t.Fatal(err)
	}

	env.Process.WSLimit = wsLimit

	// With paging on a P0 page exists only once something creates it.
	if err := env.EnsurePages(0x200, 80*512); err != nil {
		t.Fatal(err)
	}

	for i := range uint32(80) {
		c.Mem.BeginInstruction()

		if err := c.Mem.StoreLongwordIn(c.CPU, env.Space.AddressSpace, 0x200+i*512, i); err != nil {
			t.Fatal(err)
		}
	}

	c.Mem.BeginInstruction()
	checkWorkingSets(t, c)

	if used := pf.Map.UsedBlocks(); used == 0 {
		t.Fatal("the writer wrote none of the process's pages")
	}

	sys.RemoveProcess(env)
	checkWorkingSets(t, c)

	if used := pf.Map.UsedBlocks(); used != 0 {
		t.Errorf("after the process went: %d page file blocks still in use", used)
	}

	if err := pf.Map.Check(); err != nil {
		t.Fatal(err)
	}
}
