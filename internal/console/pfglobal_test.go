package console_test

import (
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/paging"
	"github.com/tucats/govax/internal/vmsdef"
)

// Phase 65's subtask 4 (docs/PHASE-65 - sections and paging.md), with
// paging on: a page-file global section's pages are global demand-zero
// pages. Mapping gives them no frames; the first fault anywhere gives a
// page a frame of zeros, backed by the page file and effectively
// modified, and later faults share it. A page leaving the last working
// set waits on the modified list, the writer writes it to the page file,
// a reused frame leaves its block in the section, and the next fault
// reads it back. A permanent section keeps its contents while nothing
// maps it, and gives its blocks back when it's deleted. Neither the
// mapping nor the touches charge the paging file quota.

// pfGlobalSource creates the permanent page-file section PFGLOBAL, 64
// pages, writable, and maps it twice: at range A ($CRMPSC, R7 its first
// address) and range B ($MGBLSC, R8) (checkpoint 1). It then deletes both
// ranges with $DELTVA (checkpoint 2, R9 the status), maps the section
// again at range C ($MGBLSC, R7; checkpoint 3), and deletes it with
// $DGBLSC, then range C (checkpoint 4, R9 the $DGBLSC's status). At
// each checkpoint it waits in a loop until the test sets R10 to the
// checkpoint's number, R6. R9 is a failing service's status.
const pfGlobalSource = `	.title	pfglobal
	$secdef
	.macro	stop,n,?l
	movl	#n,r6
l:	cmpl	r10,r6
	bneq	l
	.endm	stop
	.macro	ok,?l
	movl	r0,r9
	blbs	r0,l
	brw	fail
l:
	.endm	ok
	.psect	data,noexe,wrt,page
inadr:	.long	^x200,^x200
arange:	.blkl	2
brange:	.blkl	2
crange:	.blkl	2
name:	.ascid	/PFGLOBAL/
	.psect	code,exe,nowrt
	.entry	start,^m<r2,r3,r4,r5,r6,r7,r8,r9,r10,r11>
	clrl	r10
	clrl	r7
	clrl	r8
	$crmpsc_s inadr=inadr,retadr=arange,gsdnam=name,-
		flags=#<sec$m_gbl!sec$m_pagfil!sec$m_perm!sec$m_expreg!sec$m_wrt>,-
		pagcnt=#64
	ok
	$mgblsc_s inadr=inadr,retadr=brange,gsdnam=name,-
		flags=#<sec$m_expreg!sec$m_wrt>
	ok
	movl	arange,r7
	movl	brange,r8
	stop	1
	$deltva_s inadr=arange
	ok
	$deltva_s inadr=brange
	ok
	stop	2
	$mgblsc_s inadr=inadr,retadr=crange,gsdnam=name,-
		flags=#<sec$m_expreg!sec$m_wrt>
	ok
	movl	crange,r7
	stop	3
	$dgblsc_s gsdnam=name
	ok
	$deltva_s inadr=crange
	ok
	stop	4
	movl	#1,r0
	ret
fail:	brb	fail
	.end	start
`

// pfGlobalPages is how many pages PFGLOBAL has.
const pfGlobalPages = 64

// pfGlobalStates counts PFGLOBAL's pages by where they are: valid in a
// working set, on the free list, on the modified list, out in a page
// file block, or out and never written (demand zero).
type pfGlobalStates struct {
	valid, free, modified, inPageFile, zero int
}

// checkPFGlobal checks PFGLOBAL's pages against the PFN database and
// the PTEs of process 1's ranges given: each PTE is valid or holds the
// page's global page table index; a page with a frame has it tied to the
// global page, backed by the page file, active with a share for each
// valid PTE, or on a list with none (on the free list, a clean copy of
// its block; on the modified list, with no block); a page that's out has
// no valid PTE.
// The page file's blocks are the ones the pages hold (checkPageFileHolders).
func checkPFGlobal(t *testing.T, c *console.Console, label string, ranges ...uint32) pfGlobalStates {
	t.Helper()

	db := c.Mem.Paging()
	s := pfGlobalSection(t, c)

	checkWorkingSets(t, c)
	checkPageFileHolders(t, c, label)

	if err := db.Check(); err != nil {
		t.Errorf("%s: %v", label, err)
	}

	if s.Refs != len(ranges)*pfGlobalPages {
		t.Errorf("%s: the section has %d references, want %d", label, s.Refs, len(ranges)*pfGlobalPages)
	}

	var states pfGlobalStates

	blocks := s.PageFileBlocks()

	for i, frame := range s.Frames {
		valid := 0

		for _, first := range ranges {
			pte := pteIn(t, c, first+uint32(i)*512)

			switch _, global := pte.GlobalIndex(); {
			case pte.Valid():
				valid++

				if pte.PFN() != frame {
					t.Errorf("%s: page %d's PTE at %08X is valid on frame %d, the section's is %d", label, i, first, pte.PFN(), frame)
				}
			case !global:
				t.Errorf("%s: page %d's PTE at %08X is %08X, neither valid nor its global index", label, i, first, uint32(pte))
			}
		}

		if frame == 0 {
			if valid != 0 {
				t.Errorf("%s: page %d is out but %d PTEs are valid", label, i, valid)
			}

			if blocks[i] != 0 {
				states.inPageFile++
			} else {
				states.zero++
			}

			continue
		}

		if blocks[i] != 0 {
			t.Errorf("%s: page %d holds frame %d and page file block %d", label, i, frame, blocks[i])
		}

		e, err := db.Entry(paging.PFN(frame))
		if err != nil || !e.Tied || e.Type != paging.GlobalPage || e.Owner.VA != uint32(i)*512 || e.Backing.Kind != paging.PageFile {
			t.Errorf("%s: page %d's frame %d is %+v (%v), not tied to it", label, i, frame, e, err)

			continue
		}

		switch e.State {
		case paging.Active:
			states.valid++

			if e.ShareCount != valid || e.RefCount != 1 {
				t.Errorf("%s: page %d's frame has %d shares, %d references; %d PTEs are valid", label, i, e.ShareCount, e.RefCount, valid)
			}
		case paging.Free:
			states.free++

			if e.Backing.Block == 0 || e.SavedModify {
				t.Errorf("%s: page %d's frame is on the free list with %+v, not a clean copy of a block", label, i, e)
			}
		case paging.Modified:
			states.modified++

			// Modified since it was last written, so its old block (if it
			// had one) no longer holds it, and has gone back.
			if e.Backing.Block != 0 {
				t.Errorf("%s: page %d's frame waits on the modified list still holding block %d", label, i, e.Backing.Block)
			}
		}

		if e.State != paging.Active && valid != 0 {
			t.Errorf("%s: page %d's frame is on the %s list, but %d PTEs are valid", label, i, e.State, valid)
		}
	}

	return states
}

// pfGlobalSection returns PFGLOBAL.
func pfGlobalSection(t *testing.T, c *console.Console) *corevms.GlobalSection {
	t.Helper()

	for _, s := range c.RTL.Sections.Sections() {
		if s.Name == "PFGLOBAL" {
			return s
		}
	}

	t.Fatal("no section PFGLOBAL")

	return nil
}

// pfTouch reads page i of PFGLOBAL at range first, an instruction at a
// time, failing the test if it doesn't hold want; with add, it then
// stores the value plus add.
func pfTouch(t *testing.T, c *console.Console, first uint32, i int, want, add uint32) {
	t.Helper()

	va := first + uint32(i)*512

	c.Mem.BeginInstruction()

	v, err := c.Mem.LoadLongword(c.CPU, va)
	if err != nil || v != want {
		t.Errorf("section page %d at %08X holds %d (%v), want %d", i, va, v, err, want)
	}

	if add != 0 {
		if err := c.Mem.StoreLongword(c.CPU, va, v+add); err != nil {
			t.Fatal(err)
		}
	}

	c.Mem.BeginInstruction()
}

// TestPageFileGlobalSection runs pfGlobalSource with a 30-page working
// set, the writer's thresholds at 10 and 2, and a host page file.
func TestPageFileGlobalSection(t *testing.T) {
	pageFileSettings(t)

	c, out := scheduledConsole(t, longQuantum, brbSelf)
	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t), nil)

	t.Cleanup(func() {
		if t.Failed() {
			t.Log(out.String())
		}
	})

	env := c.ProcessOne()
	p := env.Process
	db := c.Mem.Paging()

	if db == nil {
		t.Fatal("vax.init's VMINIT left paging off")
	}

	exe := buildImage(t, c, "pfglobal", pfGlobalSource)

	c.Engine.SetLimits(1, 0)
	runImage(t, d, exe)
	c.Engine.SetLimits(0, 0)

	if !c.ImageActive() {
		t.Fatal("the image isn't running")
	}

	checkpoint := func(n uint32) {
		t.Helper()

		if n > 1 {
			c.CPU.SetGPR(10, n-1)
		}

		runUntil(t, c, 400_000, func() bool { return c.CPU.GPR(6) == n })
		step(t, c, 4)
	}

	charged := env.PageFileCharged()

	// Checkpoint 1: mapped twice, no page has a frame, and nothing is
	// charged to the paging file quota.
	checkpoint(1)

	rangeA, rangeB := c.CPU.GPR(7), c.CPU.GPR(8)
	if rangeA == 0 || rangeB == 0 {
		t.Fatalf("checkpoint 1: the section wasn't mapped twice (status %08X)", c.CPU.GPR(9))
	}

	if got := checkPFGlobal(t, c, "checkpoint 1", rangeA, rangeB); got.zero != pfGlobalPages {
		t.Errorf("checkpoint 1: pages %+v, want all %d demand zero", got, pfGlobalPages)
	}

	if got := env.PageFileCharged(); got != charged {
		t.Errorf("checkpoint 1: %d pages charged, %d before the section", got, charged)
	}

	// Page 0 through both ranges: a demand-zero fault, then a global
	// valid fault; the frame has two shares and is effectively modified.
	before := p.Faults
	pfTouch(t, c, rangeA, 0, 0, 0)
	pfTouch(t, c, rangeB, 0, 0, 0)

	if got := p.Faults.DemandZero - before.DemandZero; got != 1 {
		t.Errorf("page 0 through both ranges: %d demand-zero faults, want 1", got)
	}

	if got := p.Faults.GlobalValid - before.GlobalValid; got != 1 {
		t.Errorf("page 0 through both ranges: %d global valid faults, want 1", got)
	}

	if e, _ := db.Entry(paging.PFN(pfGlobalSection(t, c).Frames[0])); e.ShareCount != 2 || !e.SavedModify {
		t.Errorf("page 0's frame: %+v, want two shares and its saved modify bit", e)
	}

	// Each page written through range A, twice the working set's limit:
	// the pages leave it to the modified list, and the writer writes
	// them to the page file in clusters.
	counts := c.RTL.PageCounts

	for i := range pfGlobalPages {
		pfTouch(t, c, rangeA, i, 0, uint32(i+1))
	}

	states := checkPFGlobal(t, c, "after the writes", rangeA, rangeB)
	if states.modified > mpwHiLimit || states.free == 0 || states.zero != 0 {
		t.Errorf("after the writes: pages %+v", states)
	}

	if written, ios := c.RTL.PageCounts.Written-counts.Written, c.RTL.PageCounts.WriteIOs-counts.WriteIOs; written < uint64(states.free) || 2*ios > written {
		t.Errorf("after the writes: the writer wrote %d pages in %d writes, with %d section pages written", written, ios, states.free)
	}

	if got := env.PageFileCharged(); got > charged+wsLimit {
		t.Errorf("after the writes: %d pages charged, %d before the section", got, charged)
	}

	// The free list's frames reused: their pages are out, their blocks in
	// the section; reading through range B brings each back from the page
	// file with a hard fault, or off the modified list, or shares a page
	// still valid through range A.
	stealFreeFrames(t, c)

	states = checkPFGlobal(t, c, "after the frames were reused", rangeA, rangeB)
	if states.free != 0 || states.inPageFile == 0 {
		t.Errorf("after the frames were reused: pages %+v, want none on the free list, some in the page file", states)
	}

	before = p.Faults
	reads := c.RTL.PageCounts.PageFileReads

	for i := pfGlobalPages - 1; i >= 0; i-- {
		pfTouch(t, c, rangeB, i, uint32(i+1), 0)
	}

	if got := p.Faults.Read - before.Read; got < uint64(states.inPageFile) {
		t.Errorf("reading through range B: %d hard faults, want at least the %d pages in the page file", got, states.inPageFile)
	}

	if got := c.RTL.PageCounts.PageFileReads - reads; got < uint64(states.inPageFile) {
		t.Errorf("reading through range B: %d page file reads, want at least %d", got, states.inPageFile)
	}

	if got := p.Faults.DemandZero - before.DemandZero; got != 0 {
		t.Errorf("reading through range B: %d demand-zero faults, want none", got)
	}

	checkPFGlobal(t, c, "after reading back", rangeA, rangeB)

	// The first 20 pages written again, through range B, then the rest
	// read through range A, so that replacement takes the 20 out of the
	// working set: a page read back from the page file gives up its block
	// when it leaves modified.
	for i := range 20 {
		pfTouch(t, c, rangeB, i, uint32(i+1), 1000)
	}

	for i := pfGlobalPages - 1; i >= 20; i-- {
		pfTouch(t, c, rangeA, i, uint32(i+1), 0)
	}

	if states := checkPFGlobal(t, c, "after the second writes", rangeA, rangeB); states.modified == 0 {
		t.Errorf("after the second writes: pages %+v, want some on the modified list", states)
	}

	// Checkpoint 2: both ranges deleted. The permanent section lives on,
	// unmapped, its pages in transition or in the page file; with the
	// free list's frames reused, more of them are in the page file.
	checkpoint(2)

	if st := c.CPU.GPR(9); st != vmsdef.Symbols["SS$_NORMAL"] {
		t.Fatalf("checkpoint 2: $DELTVA = %08X", st)
	}

	states = checkPFGlobal(t, c, "checkpoint 2")
	if states.valid != 0 || states.zero != 0 {
		t.Errorf("checkpoint 2: pages %+v, want none valid or demand zero", states)
	}

	stealFreeFrames(t, c)
	checkPFGlobal(t, c, "checkpoint 2, frames reused")

	// Checkpoint 3: mapped again at range C, every page holds what was
	// written.
	checkpoint(3)

	rangeC := c.CPU.GPR(7)

	for i := range pfGlobalPages {
		want := uint32(i + 1)
		if i < 20 {
			want += 1000
		}

		pfTouch(t, c, rangeC, i, want, 0)
	}

	checkPFGlobal(t, c, "checkpoint 3", rangeC)

	// The free list's frames reused, so that the section, when it goes,
	// holds blocks both for pages out and as frames' backing stores.
	stealFreeFrames(t, c)

	if states := checkPFGlobal(t, c, "checkpoint 3, frames reused", rangeC); states.inPageFile == 0 || states.valid == 0 {
		t.Errorf("checkpoint 3, frames reused: pages %+v, want some in the page file and some valid", states)
	}

	if got := env.PageFileCharged(); got > charged+wsLimit {
		t.Errorf("checkpoint 3: %d pages charged, %d before the section", got, charged)
	}

	// Checkpoint 4: $DGBLSC, then range C deleted: the section is gone by
	// name, but waits to be released, holding its frames and page file
	// blocks, as VMS 7.1's did (testdata/pg65's R4).
	checkpoint(4)

	if st := c.CPU.GPR(9); st != vmsdef.Symbols["SS$_NORMAL"] {
		t.Fatalf("checkpoint 4: status %08X", st)
	}

	for _, s := range c.RTL.Sections.Sections() {
		if s.Name == "PFGLOBAL" {
			t.Error("checkpoint 4: the section still exists")
		}
	}

	if p := c.RTL.Sections.PendingSections(); len(p) != 1 || p[0].Name != "PFGLOBAL" {
		t.Errorf("checkpoint 4: pending %v, want PFGLOBAL", p)
	}

	checkWorkingSets(t, c)
	checkPageFileHolders(t, c, "checkpoint 4")

	// Image rundown releases it, with every frame it held and every page
	// file block.
	c.CPU.SetGPR(10, 4)
	finishImage(t, c)

	if p := c.RTL.Sections.PendingSections(); len(p) != 0 {
		t.Errorf("after rundown: pending %v", p)
	}

	for n := range db.Frames() {
		if e, _ := db.Entry(paging.PFN(n)); e.Tied && e.Type == paging.GlobalPage {
			t.Errorf("after rundown: frame %d is still tied to a global page: %+v", n, e)
		}
	}

	checkPageFileHolders(t, c, "after rundown")
}
