package console_test

import (
	"encoding/binary"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/paging"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmsdef"
)

// Phase 65's subtask 3 (docs/PHASE-65 - sections and paging.md), with
// paging on: a global file section's pages leave working sets. A process
// PTE leaving gets its global page table index back; the frame stays in
// memory while any PTE holds it valid (its share count), then waits on a
// list, tied to the global page, where a fault takes it back softly, the
// modified page writer writes it to the section's file, and a reuse
// breaks its tie in the global page table. $UPDSEC follows the global
// page table entry, and deleting the section writes back the pages left
// on the modified list.

// gblPagingSource maps PAGES.DAT, opened for writing, as the writable
// global section GBLPAGES, 64 pages, twice: at range A ($CRMPSC, R7 its
// first address) and range B ($MGBLSC, R8), so the one process has two
// PTEs on each page (checkpoint 1). It then runs $UPDSECW over range A
// (checkpoint 2, R9 its status, R7 and R8 the first and last pages of
// the first run it wrote) and exits. At each checkpoint it waits in
// a loop until the test sets R10 to the checkpoint's number, R6. R9 is a
// failing service's status before checkpoint 1.
const gblPagingSource = `	.title	gblpage
	$fabdef
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
fab:	$fab	fnm=<DUA0:[000000]PAGES.DAT>,fac=<get,put>,fop=ufo
inadr:	.long	^x200,^x200
arange:	.blkl	2
brange:	.blkl	2
urange:	.blkl	2
iosb:	.blkq	1
name:	.ascid	/GBLPAGES/
	.psect	code,exe,nowrt
	.entry	start,^m<r2,r3,r4,r5,r6,r7,r8,r9,r10,r11>
	clrl	r10
	clrl	r7
	clrl	r8
	$open	fab=fab
	ok
	$crmpsc_s inadr=inadr,retadr=arange,gsdnam=name,-
		flags=#<sec$m_gbl!sec$m_expreg!sec$m_wrt>,-
		chan=fab+fab$l_stv,pagcnt=#64
	ok
	$mgblsc_s inadr=inadr,retadr=brange,gsdnam=name,-
		flags=#<sec$m_expreg!sec$m_wrt>
	ok
	movl	arange,r7
	movl	brange,r8
	stop	1
	$updsecw_s inadr=arange,retadr=urange,iosb=iosb
	movl	r0,r9
	movl	urange,r7
	movl	urange+4,r8
	stop	2
	movl	#1,r0
	ret
fail:	brb	fail
	.end	start
`

// gblPagingRun is a run of gblPagingSource: the console, the program's
// two ranges, and the checkpoints.
type gblPagingRun struct {
	c          *console.Console
	env        *corevms.Environment
	pagesFile  rms.FileLocation
	rangeA     uint32
	rangeB     uint32
	checkpoint func(n uint32)
}

// startGlobalPaging builds and runs gblPagingSource to checkpoint 1.
func startGlobalPaging(t *testing.T) *gblPagingRun {
	t.Helper()

	c, out := scheduledConsole(t, longQuantum, brbSelf)
	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t), nil)

	t.Cleanup(func() {
		if t.Failed() {
			t.Log(out.String())
		}
	})

	r := &gblPagingRun{c: c, env: c.ProcessOne(), pagesFile: makePagesFile(t, c)}
	exe := buildImage(t, c, "gblpage", gblPagingSource)

	c.Engine.SetLimits(1, 0)
	runImage(t, d, exe)
	c.Engine.SetLimits(0, 0)

	if !c.ImageActive() {
		t.Fatalf("the image isn't running\n%s", out.String())
	}

	r.checkpoint = func(n uint32) {
		t.Helper()

		if n > 1 {
			c.CPU.SetGPR(10, n-1)
		}

		runUntil(t, c, 400_000, func() bool { return c.CPU.GPR(6) == n })
		step(t, c, 4)
	}

	r.checkpoint(1)

	r.rangeA, r.rangeB = c.CPU.GPR(7), c.CPU.GPR(8)
	if r.rangeA == 0 || r.rangeB == 0 {
		t.Fatalf("checkpoint 1: the section wasn't mapped twice (status %08X)", c.CPU.GPR(9))
	}

	return r
}

// section returns GBLPAGES.
func (r *gblPagingRun) section(t *testing.T) *corevms.GlobalSection {
	t.Helper()

	for _, s := range r.c.RTL.Sections.Sections() {
		if s.Name == "GBLPAGES" {
			return s
		}
	}

	t.Fatal("no section GBLPAGES")

	return nil
}

// touch reads page i of the section at range first, an instruction at a
// time, failing the test if it doesn't hold want; with add, it then
// stores the value plus add.
func (r *gblPagingRun) touch(t *testing.T, first uint32, i int, want, add uint32) {
	t.Helper()

	c := r.c
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

// rewrite reads page i of the section at range first, failing the test
// if it doesn't hold want, and stores the same value back, setting the
// PTE's modify bit.
func (r *gblPagingRun) rewrite(t *testing.T, first uint32, i int, want uint32) {
	t.Helper()

	c := r.c
	va := first + uint32(i)*512

	r.touch(t, first, i, want, 0)

	if err := c.Mem.StoreLongword(c.CPU, va, want); err != nil {
		t.Fatal(err)
	}

	c.Mem.BeginInstruction()
}

// fileBlocks returns PAGES.DAT's blocks' first longwords.
func (r *gblPagingRun) fileBlocks(t *testing.T) []uint32 {
	t.Helper()

	got, _, err := r.c.ContainerSession.ReadRecordFile(r.pagesFile, rms.ImageBlocks)
	if err != nil {
		t.Fatal(err)
	}

	values := make([]uint32, len(got))
	for i, b := range got {
		values[i] = binary.LittleEndian.Uint32(b)
	}

	return values
}

// globalPageStates counts the section's pages by where they are: valid in
// a working set, on the free list, on the modified list, or out.
type globalPageStates struct {
	valid, free, modified, out int
}

// checkGlobalPages checks the section's pages against the PFN database
// and process 1's PTEs at both ranges: each PTE is valid or holds the
// page's global page table index, never a transition form; a page that's
// out has no valid PTE; a page with a frame has it tied to the global
// page, active with a share for each valid PTE, or on a list with none.
func (r *gblPagingRun) checkGlobalPages(t *testing.T, label string) globalPageStates {
	t.Helper()

	c := r.c
	db := c.Mem.Paging()
	s := r.section(t)

	checkWorkingSets(t, c)

	if s.Refs != 2*wsPagesBlocks {
		t.Errorf("%s: the section has %d references, want %d", label, s.Refs, 2*wsPagesBlocks)
	}

	var states globalPageStates

	for i, frame := range s.Frames {
		valid := 0

		for _, first := range []uint32{r.rangeA, r.rangeB} {
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
			states.out++

			if valid != 0 {
				t.Errorf("%s: page %d is out but %d PTEs are valid", label, i, valid)
			}

			continue
		}

		e, err := db.Entry(paging.PFN(frame))
		if err != nil || !e.Tied || e.Type != paging.GlobalPage || e.Owner.VA != uint32(i)*512 {
			t.Errorf("%s: page %d's frame %d is %+v (%v), not tied to it", label, i, frame, e, err)

			continue
		}

		switch e.State {
		case paging.Active:
			states.valid++

			if e.ShareCount != valid || e.RefCount != 1 {
				t.Errorf("%s: page %d's frame has %d shares, %d references; %d PTEs are valid", label, i, e.ShareCount, e.RefCount, valid)
			}
		case paging.Free, paging.Modified:
			if e.State == paging.Free {
				states.free++
			} else {
				states.modified++
			}

			if valid != 0 {
				t.Errorf("%s: page %d's frame is on the %s list, but %d PTEs are valid", label, i, e.State, valid)
			}
		}
	}

	return states
}

// TestGlobalSectionPaging runs gblPagingSource with a 30-page working set,
// the writer's thresholds at 10 and 2, and checks the section's pages as
// they leave the working set, come back, are written, and lose their
// frames.
func TestGlobalSectionPaging(t *testing.T) {
	pageFileSettings(t)

	r := startGlobalPaging(t)
	c, p := r.c, r.env.Process
	db := c.Mem.Paging()

	if db == nil {
		t.Fatal("vax.init's VMINIT left paging off")
	}

	if got := r.checkGlobalPages(t, "checkpoint 1"); got.out != wsPagesBlocks {
		t.Errorf("checkpoint 1: pages %+v, want all %d out", got, wsPagesBlocks)
	}

	// Page 0 through both ranges: a hard fault, then a global valid
	// fault, and the frame has two shares.
	before := p.Faults
	r.touch(t, r.rangeA, 0, 1, 0)
	r.touch(t, r.rangeB, 0, 1, 0)

	if got := p.Faults.Read - before.Read; got != 1 {
		t.Errorf("page 0 read through both ranges: %d hard faults, want 1", got)
	}

	if got := p.Faults.GlobalValid - before.GlobalValid; got != 1 {
		t.Errorf("page 0 read through both ranges: %d global valid faults, want 1", got)
	}

	frame := r.section(t).Frames[0]
	if e, _ := db.Entry(paging.PFN(frame)); e.ShareCount != 2 {
		t.Errorf("page 0's frame has %d shares, want 2", e.ShareCount)
	}

	// Each page modified through range A, twice the working set's limit:
	// the pages leave it, their frames to the modified list, and the
	// writer writes them to PAGES.DAT.
	counts := c.RTL.PageCounts

	for i := range wsPagesBlocks {
		r.touch(t, r.rangeA, i, uint32(i+1), 1000)
	}

	states := r.checkGlobalPages(t, "after the writes")

	// The section's pages went out in clusters, several to a write.
	written, ios := c.RTL.PageCounts.Written-counts.Written, c.RTL.PageCounts.WriteIOs-counts.WriteIOs
	if written < uint64(states.free) || 2*ios > written {
		t.Errorf("after the writes: the writer wrote %d pages in %d writes, with %d section pages written", written, ios, states.free)
	}
	if states.modified > mpwHiLimit || states.free == 0 || states.valid+states.free+states.modified != wsPagesBlocks {
		t.Errorf("after the writes: pages %+v", states)
	}

	if n := c.RTL.WorkingSet().Counts().Global; n > wsLimit {
		t.Errorf("after the writes: %d global pages in the working set, over its limit", n)
	}

	blocks := r.fileBlocks(t)
	s := r.section(t)

	for i, frame := range s.Frames {
		if e, _ := db.Entry(paging.PFN(frame)); frame != 0 && e.State == paging.Free && blocks[i] != uint32(i+1001) {
			t.Errorf("after the writes: page %d is clean on the free list, but its block holds %d", i, blocks[i])
		}
	}

	// Every page read through range B, from the last down (so that the
	// last pages written are read before replacement takes them out of
	// the working set): the pages out of it are soft faults off the
	// lists, those still valid through range A global valid faults.
	before = p.Faults

	for i := wsPagesBlocks - 1; i >= 0; i-- {
		r.touch(t, r.rangeB, i, uint32(i+1001), 0)
	}

	if got := p.Faults.FreeList + p.Faults.ModifiedList - before.FreeList - before.ModifiedList; got == 0 {
		t.Error("reading through range B: no soft fault off a list")
	}

	if got := p.Faults.GlobalValid - before.GlobalValid; got == 0 {
		t.Error("reading through range B: no global valid fault")
	}

	if got := p.Faults.Read - before.Read; got != 0 {
		t.Errorf("reading through range B: %d hard faults, want none (every page was in memory)", got)
	}

	r.checkGlobalPages(t, "after the reads")

	// The free list's frames reused: their pages are out, and no PTE
	// changed; reading through range A brings them back from the file.
	stealFreeFrames(t, c)

	states = r.checkGlobalPages(t, "after the frames were reused")
	if states.free != 0 || states.out == 0 {
		t.Errorf("after the frames were reused: pages %+v, want none on the free list, some out", states)
	}

	before = p.Faults

	for i := range wsPagesBlocks {
		r.touch(t, r.rangeA, i, uint32(i+1001), 0)
	}

	if got := p.Faults.Read - before.Read; got < uint64(states.out) {
		t.Errorf("reading through range A: %d hard faults, want at least the %d pages out", got, states.out)
	}

	r.checkGlobalPages(t, "after reading back")

	// Pages 0 to 5 rewritten through range B, then 41 pages read through
	// it, which take them out of the working set to the modified list
	// (MPW_HILIMIT raised, so that the writer leaves them there with the
	// image's own pages); then pages 6 to 8 rewritten through range B,
	// valid only there. Range A holds each of those pages' global page
	// table indexes.
	c.RTL.Sysgen.MPWHiLimit = 1000

	for i := range 6 {
		r.rewrite(t, r.rangeB, i, uint32(i+1001))
	}

	for i := 10; i <= 50; i++ {
		r.touch(t, r.rangeB, i, uint32(i+1001), 0)
	}

	for i := 6; i <= 8; i++ {
		r.rewrite(t, r.rangeB, i, uint32(i+1001))
	}

	r.checkGlobalPages(t, "before $UPDSEC")

	s = r.section(t)
	for i := range 9 {
		e, _ := db.Entry(paging.PFN(s.Frames[i]))
		if want := []paging.State{paging.Modified, paging.Active}[i/6]; e.State != want {
			t.Errorf("before $UPDSEC: page %d's frame is %s, want %s", i, e.State, want)
		}
	}

	// Checkpoint 2: $UPDSEC over range A follows each page's global page
	// table entry, writing the pages valid anywhere and those on the
	// modified list, leaving none there: its first run of pages written
	// is pages 0 to 8 (page 9 is on the free list, matching its block).
	// PAGES.DAT holds every page's new value.
	r.checkpoint(2)

	if got, want := c.CPU.GPR(9), vmsdef.Symbols["SS$_NORMAL"]; got != want {
		t.Errorf("$UPDSEC: status %08X, want SS$_NORMAL", got)
	}

	if first, last := c.CPU.GPR(7), c.CPU.GPR(8); first != r.rangeA || last != r.rangeA+9*512-1 {
		t.Errorf("$UPDSEC: first run written %08X to %08X, want %08X to %08X", first, last, r.rangeA, r.rangeA+9*512-1)
	}

	if states := r.checkGlobalPages(t, "after $UPDSEC"); states.modified != 0 {
		t.Errorf("after $UPDSEC: %d pages still on the modified list", states.modified)
	}

	for i, v := range r.fileBlocks(t) {
		if v != uint32(i+1001) {
			t.Errorf("after $UPDSEC: block %d holds %d, want %d", i+1, v, i+1001)
		}
	}

	// The first 40 pages modified again, through range B; image rundown
	// deletes the section, writing back what's valid and what waits on
	// the modified list.
	for i := range 40 {
		r.touch(t, r.rangeB, i, uint32(i+1001), 1)
	}

	r.checkGlobalPages(t, "after the second writes")

	c.CPU.SetGPR(10, 2)
	finishImage(t, c)

	for _, gs := range c.RTL.Sections.Sections() {
		if gs.Name == "GBLPAGES" {
			t.Error("after the image: the section still exists")
		}
	}

	checkWorkingSets(t, c)

	for n := range db.Frames() {
		if e, _ := db.Entry(paging.PFN(n)); e.Tied && e.Type == paging.GlobalPage && e.Backing.Kind == paging.GlobalSection {
			t.Errorf("after the image: frame %d is still tied to a file section's page: %+v", n, e)
		}
	}

	checkGlobalSectionFile(t, r, "after the image")
}

// checkGlobalSectionFile fails the test unless PAGES.DAT's first 40
// blocks hold their block numbers plus 1001, the rest plus 1000: what
// gblPagingSource's run writes.
func checkGlobalSectionFile(t *testing.T, r *gblPagingRun, label string) {
	t.Helper()

	for i, v := range r.fileBlocks(t) {
		want := uint32(i + 1001)
		if i < 40 {
			want++
		}

		if v != want {
			t.Errorf("%s: block %d holds %d, want %d", label, i+1, v, want)
		}
	}
}

// TestGlobalSectionPagingOff runs gblPagingSource's touches with paging
// off: every page stays in memory once read, and the file ends with the
// same contents.
func TestGlobalSectionPagingOff(t *testing.T) {
	r := startGlobalPaging(t)
	c := r.c

	if c.Mem.Paging() != nil {
		t.Fatal("paging is on")
	}

	for i := range wsPagesBlocks {
		r.touch(t, r.rangeA, i, uint32(i+1), 1000)
	}

	for i, frame := range r.section(t).Frames {
		if frame == 0 {
			t.Errorf("page %d has no frame", i)
		}
	}

	r.checkpoint(2)

	if got, want := c.CPU.GPR(9), vmsdef.Symbols["SS$_NORMAL"]; got != want {
		t.Errorf("$UPDSEC: status %08X, want SS$_NORMAL", got)
	}

	for i := range 40 {
		r.touch(t, r.rangeB, i, uint32(i+1001), 1)
	}

	c.CPU.SetGPR(10, 2)
	finishImage(t, c)
	checkGlobalSectionFile(t, r, "after the image")
}
