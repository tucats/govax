package console_test

import (
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/machine"
	"github.com/tucats/govax/internal/paging"
	"github.com/tucats/govax/internal/vmsdef"
)

// Phase 65's subtask 5 (docs/PHASE-65 - sections and paging.md): a
// global copy-on-reference section under the new global page states. A
// page's first fault through any PTE reads the file into a frame of the
// process's own, never the section's (the book's 15.3.3: the global page
// table entry keeps its section file form, and each PTE that faults gets
// a different frame), backed by the page file and effectively modified.
// From then on it's a private copy-on-reference page (subtask 2): it
// leaves the working set in the transition form, not with its global
// page table index back, it goes to the page file, and it comes back
// from there. The section loses each PTE's reference as it faults, so a
// temporary section goes once every page has been copied; its file is
// never written, and $UPDSEC finds nothing to write.

// gblCRFSource maps PAGES.DAT, opened read-only, as the writable global
// copy-on-reference section CRFPAGES, 64 pages, twice: at range A
// ($CRMPSC, R7 its first address) and range B ($MGBLSC, R8), so the one
// process has two PTEs on each page (checkpoint 1). It then runs
// $UPDSECW over range A (checkpoint 2, R9 its status) and exits. At each
// checkpoint it waits in a loop until the test sets R10 to the
// checkpoint's number, R6. R9 is a failing service's status before
// checkpoint 1.
const gblCRFSource = `	.title	gblcrf
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
fab:	$fab	fnm=<DUA0:[000000]PAGES.DAT>,fac=get,fop=ufo,shr=get
inadr:	.long	^x200,^x200
arange:	.blkl	2
brange:	.blkl	2
urange:	.blkl	2
iosb:	.blkq	1
name:	.ascid	/CRFPAGES/
	.psect	code,exe,nowrt
	.entry	start,^m<r2,r3,r4,r5,r6,r7,r8,r9,r10,r11>
	clrl	r10
	clrl	r7
	clrl	r8
	$open	fab=fab
	ok
	$crmpsc_s inadr=inadr,retadr=arange,gsdnam=name,-
		flags=#<sec$m_gbl!sec$m_expreg!sec$m_wrt!sec$m_crf>,-
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
	stop	2
	movl	#1,r0
	ret
fail:	brb	fail
	.end	start
`

// startGlobalCRF builds and runs gblCRFSource to checkpoint 1, as a
// gblPagingRun, whose touch helper it shares.
func startGlobalCRF(t *testing.T) *gblPagingRun {
	t.Helper()

	c, out := scheduledConsole(t, longQuantum, brbSelf)
	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t), nil)

	t.Cleanup(func() {
		if t.Failed() {
			t.Log(out.String())
		}
	})

	r := &gblPagingRun{c: c, env: c.ProcessOne(), pagesFile: makePagesFile(t, c)}
	exe := buildImage(t, c, "gblcrf", gblCRFSource)

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

// crfSection returns CRFPAGES, or nil once it's gone.
func crfSection(r *gblPagingRun) *corevms.GlobalSection {
	for _, s := range r.c.RTL.Sections.Sections() {
		if s.Name == "CRFPAGES" {
			return s
		}
	}

	return nil
}

// crfPages returns the address of each page of gblCRFSource's two
// ranges, range A's first.
func crfPages(r *gblPagingRun) []uint32 {
	vas := make([]uint32, 0, 2*wsPagesBlocks)

	for _, first := range []uint32{r.rangeA, r.rangeB} {
		for i := range wsPagesBlocks {
			vas = append(vas, first+uint32(i)*512)
		}
	}

	return vas
}

// checkGlobalCopies checks process 1's PTEs at vas, pages of
// gblCRFSource's ranges that have been touched: each is valid, in
// transition, or in the page file form, never a global page table index,
// and a frame it holds is the process's own, tied to the PTE's page,
// backed by the page file and effectively modified unless it was read
// back from its block since (a clean page then). No two PTEs share a
// frame, and the working set holds no global page.
func checkGlobalCopies(t *testing.T, r *gblPagingRun, vas []uint32, label string) {
	t.Helper()

	c := r.c
	db := c.Mem.Paging()
	pid := r.env.Process.PID
	owners := map[uint32]uint32{}

	checkWorkingSets(t, c)

	if n := c.RTL.WorkingSet().Counts().Global; n != 0 {
		t.Errorf("%s: %d global pages in the working set, want none", label, n)
	}

	for _, va := range vas {
		pte := pteIn(t, c, va)

		if _, ok := pte.GlobalIndex(); ok {
			t.Errorf("%s: the PTE at %08X still holds its global page table index", label, va)

			continue
		}

		if _, ok := pte.PageFileBlock(); ok {
			continue
		}

		pfn, ok := pte.Transition()
		if pte.Valid() {
			pfn, ok = pte.PFN(), true
		}

		if !ok {
			t.Errorf("%s: the PTE at %08X is %08X, neither valid, in transition, nor in the page file", label, va, uint32(pte))

			continue
		}

		if other, shared := owners[pfn]; shared {
			t.Errorf("%s: the PTEs at %08X and %08X hold one frame, %d", label, other, va, pfn)
		}

		owners[pfn] = va

		e, err := db.Entry(paging.PFN(pfn))
		if err != nil || !e.Tied || e.Type != paging.ProcessPage || e.Owner != (paging.Owner{Handle: pid, VA: va}) ||
			e.Backing.Kind != paging.PageFile {
			t.Errorf("%s: the frame at %08X is %+v (%v), want process 1's page backed by the page file", label, va, e, err)

			continue
		}

		if e.Backing.Block == 0 && !e.SavedModify {
			t.Errorf("%s: the frame at %08X has no block and isn't effectively modified", label, va)
		}
	}
}

// TestGlobalCopyOnRefPaging runs gblCRFSource with a 30-page working set,
// the writer's thresholds at 10 and 2, and a host page file, and checks
// each PTE's copy of its page as it's made, leaves the working set, goes
// to the page file, and comes back.
func TestGlobalCopyOnRefPaging(t *testing.T) {
	pageFileSettings(t)

	// One page a fault, so that each copy is made by its own fault: read
	// clustering is clusterpaging_test.go's.
	setSetting(t, machine.PFCDefaultSetting, "1")

	r := startGlobalCRF(t)
	c, env, p := r.c, r.env, r.env.Process

	if c.Mem.Paging() == nil {
		t.Fatal("vax.init's VMINIT left paging off")
	}

	// Checkpoint 1: every PTE of both ranges holds its global page table
	// index and is charged (subtask 2); the section has a reference for
	// each, and no frames.
	s := crfSection(r)
	if s == nil || !s.CopyOnRef {
		t.Fatalf("checkpoint 1: CRFPAGES is %+v, want a copy-on-reference section", s)
	}

	if s.Refs != 2*wsPagesBlocks {
		t.Errorf("checkpoint 1: the section has %d references, want %d", s.Refs, 2*wsPagesBlocks)
	}

	for _, va := range crfPages(r) {
		if _, ok := pteIn(t, c, va).GlobalIndex(); !ok {
			t.Errorf("checkpoint 1: the PTE at %08X isn't a global page table index", va)
		}

		if !env.PageFileChargedAt(va) {
			t.Errorf("checkpoint 1: the page at %08X isn't charged", va)
		}
	}

	for i, frame := range s.Frames {
		if frame != 0 {
			t.Errorf("checkpoint 1: section page %d holds frame %d", i, frame)
		}
	}

	quota := env.PageFileRemaining()

	// Page 0 through both ranges: two hard faults, no global valid one,
	// each making a copy of its own from the file. Range A's write
	// doesn't reach range B's copy. The section loses both references,
	// and keeps no frame.
	before := p.Faults
	r.touch(t, r.rangeA, 0, 1, 1000)
	r.touch(t, r.rangeB, 0, 1, 2000)

	if got := p.Faults.Read - before.Read; got != 2 {
		t.Errorf("page 0 through both ranges: %d hard faults, want 2", got)
	}

	if got := p.Faults.GlobalValid - before.GlobalValid; got != 0 {
		t.Errorf("page 0 through both ranges: %d global valid faults, want none", got)
	}

	if a, b := pteIn(t, c, r.rangeA), pteIn(t, c, r.rangeB); !a.Valid() || !b.Valid() || a.PFN() == b.PFN() {
		t.Errorf("page 0's PTEs are %08X and %08X, want valid on two frames", uint32(a), uint32(b))
	}

	if s.Refs != 2*wsPagesBlocks-2 || s.Frames[0] != 0 {
		t.Errorf("page 0 copied: the section has %d references and frame %d, want %d and none",
			s.Refs, s.Frames[0], 2*wsPagesBlocks-2)
	}

	checkGlobalCopies(t, r, []uint32{r.rangeA, r.rangeB}, "page 0 copied")

	// Every other page modified through range A, then through range B,
	// four times the working set's limit: the copies leave the working
	// set in transition, the writer writes them to the page file, and the
	// fault charges nothing more.
	for i := 1; i < wsPagesBlocks; i++ {
		r.touch(t, r.rangeA, i, uint32(i+1), 1000)
	}

	for i := 1; i < wsPagesBlocks; i++ {
		r.touch(t, r.rangeB, i, uint32(i+1), 2000)
	}

	if got := env.PageFileRemaining(); got != quota {
		t.Errorf("after the copies: %d pages of quota left, want %d (charged at mapping)", got, quota)
	}

	checkGlobalCopies(t, r, crfPages(r), "after the copies")

	// The section's last reference went with the last page's copy, and
	// it was temporary: it's gone.
	if crfSection(r) != nil {
		t.Errorf("after the copies: CRFPAGES still exists with %d references", s.Refs)
	}

	checkPagesFileUnchanged(t, c, r.pagesFile, "after the copies")

	// The written copies' frames reused: their PTEs take the page file
	// form, and each page reads back with its own range's value.
	stealFreeFrames(t, c)
	checkGlobalCopies(t, r, crfPages(r), "after the frames were reused")
	checkPageFileHolders(t, c, "after the frames were reused")

	inPageFile := 0

	for _, va := range crfPages(r) {
		if vbn, ok := pteIn(t, c, va).PageFileBlock(); ok && vbn != 0 {
			inPageFile++
		}
	}

	if inPageFile == 0 {
		t.Error("after the frames were reused: no copy is in the page file form")
	}

	before = p.Faults

	for i := range wsPagesBlocks {
		r.touch(t, r.rangeA, i, uint32(i+1001), 0)
		r.touch(t, r.rangeB, i, uint32(i+2001), 0)
	}

	if got := p.Faults.Read - before.Read; got < uint64(inPageFile) {
		t.Errorf("reading back: %d hard faults, want at least the %d copies in the page file", got, inPageFile)
	}

	checkGlobalCopies(t, r, crfPages(r), "after reading back")

	// Checkpoint 2: $UPDSEC over range A has nothing to write, the copies
	// being the process's own.
	r.checkpoint(2)

	if got, want := c.CPU.GPR(9), vmsdef.Symbols["SS$_NOTMODIFIED"]; got != want {
		t.Errorf("$UPDSEC: status %08X, want SS$_NOTMODIFIED (%08X)", got, want)
	}

	// Image rundown gives back every copy's charge and block, and the
	// file was never written.
	c.CPU.SetGPR(10, 2)
	finishImage(t, c)

	for _, va := range crfPages(r) {
		if env.PageFileChargedAt(va) {
			t.Errorf("after the image: the page at %08X is still charged", va)
		}
	}

	checkPageFileHolders(t, c, "after the image")
	checkPagesFileUnchanged(t, c, r.pagesFile, "after the image")
	checkWorkingSets(t, c)
}

// TestGlobalCopyOnRefPagingOff runs gblCRFSource's touches with paging
// off: each range still gets copies of its own, the section goes with its
// last reference, and the file is untouched.
func TestGlobalCopyOnRefPagingOff(t *testing.T) {
	r := startGlobalCRF(t)
	c := r.c

	if c.Mem.Paging() != nil {
		t.Fatal("paging is on")
	}

	for i := range wsPagesBlocks {
		r.touch(t, r.rangeA, i, uint32(i+1), 1000)
		r.touch(t, r.rangeB, i, uint32(i+1), 2000)
	}

	for i := range wsPagesBlocks {
		a, b := pteIn(t, c, r.rangeA+uint32(i)*512), pteIn(t, c, r.rangeB+uint32(i)*512)
		if !a.Valid() || !b.Valid() || a.PFN() == b.PFN() {
			t.Errorf("page %d's PTEs are %08X and %08X, want valid on two frames", i, uint32(a), uint32(b))
		}
	}

	if crfSection(r) != nil {
		t.Error("after the copies: CRFPAGES still exists")
	}

	for i := range wsPagesBlocks {
		r.touch(t, r.rangeA, i, uint32(i+1001), 0)
		r.touch(t, r.rangeB, i, uint32(i+2001), 0)
	}

	r.checkpoint(2)

	if got, want := c.CPU.GPR(9), vmsdef.Symbols["SS$_NOTMODIFIED"]; got != want {
		t.Errorf("$UPDSEC: status %08X, want SS$_NOTMODIFIED (%08X)", got, want)
	}

	c.CPU.SetGPR(10, 2)
	finishImage(t, c)
	checkPagesFileUnchanged(t, c, r.pagesFile, "after the image")
}
