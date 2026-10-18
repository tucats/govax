package console_test

import (
	"encoding/binary"
	"path/filepath"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
	"github.com/tucats/govax/internal/paging"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmsdef"
)

// Phase 65's subtask 2 (docs/PHASE-65 - sections and paging.md): a
// private copy-on-reference section's page becomes the process's own at
// its first fault, backed by the page file and effectively modified, so
// the modified page writer writes it to the page file and it comes back
// from there; its file is never written. Copy-on-reference pages, private
// and global, are charged against the paging file quota when they're
// mapped, and the mapping stops with SS$_EXQUOTA when the quota runs out.

// crfSource maps PAGES.DAT, opened read-only, as a writable
// copy-on-reference private section of 64 pages (checkpoint 1, R7 its
// first address); adds 1000 to the longword at the start of each page
// (checkpoint 2); and reads every page back, counting in R9 those that
// don't hold their block number plus 1000 (checkpoint 3). At each
// checkpoint it waits in a loop until the test sets R10 to the
// checkpoint's number, R6. R9 is a failing service's status before
// checkpoint 3.
const crfSource = `	.title	crfsec
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
range:	.blkl	2
	.psect	code,exe,nowrt
	.entry	start,^m<r2,r3,r4,r5,r6,r7,r8,r9,r10,r11>
	clrl	r10
	clrl	r7
	$open	fab=fab
	ok
	$crmpsc_s inadr=inadr,retadr=range,flags=#<sec$m_expreg!sec$m_wrt!sec$m_crf>,-
		chan=fab+fab$l_stv,pagcnt=#64
	ok
	movl	range,r7
	stop	1
	movl	range,r2
	movl	#64,r3
20$:	addl2	#1000,(r2)
	addl2	#512,r2
	sobgtr	r3,20$
	stop	2
	movl	range,r2
	clrl	r9
	movl	#1001,r4
30$:	cmpl	(r2),r4
	beql	31$
	incl	r9
31$:	incl	r4
	addl2	#512,r2
	cmpl	r4,#1065
	blss	30$
	stop	3
	movl	#1,r0
	ret
fail:	brb	fail
	.end	start
`

// makePagesFile mounts a fresh volume as DUA0 and writes PAGES.DAT on it,
// wsPagesBlocks blocks, each holding its block number in its first
// longword.
func makePagesFile(t *testing.T, c *console.Console) rms.FileLocation {
	t.Helper()

	path := filepath.Join(t.TempDir(), "work.dsk")
	if err := c.Commands().InitializeContainer(path, 400, "WORK", 0, "RD54"); err != nil {
		t.Fatal(err)
	}

	if err := c.Commands().Mount("DUA0", path, true); err != nil {
		t.Fatal(err)
	}

	pagesFile := rms.FileLocation{Name: "DUA0:[000000]PAGES.DAT"}

	blocks := make([][]byte, wsPagesBlocks)
	for i := range blocks {
		blocks[i] = make([]byte, 512)
		binary.LittleEndian.PutUint32(blocks[i], uint32(i+1))
	}

	if _, err := c.ContainerSession.CreateRecordFile(pagesFile, rms.ImageBlocks, blocks); err != nil {
		t.Fatal(err)
	}

	return pagesFile
}

// checkPagesFileUnchanged fails the test if any of PAGES.DAT's blocks no
// longer holds its block number: a copy-on-reference section never
// writes its file.
func checkPagesFileUnchanged(t *testing.T, c *console.Console, pagesFile rms.FileLocation, label string) {
	t.Helper()

	got, _, err := c.ContainerSession.ReadRecordFile(pagesFile, rms.ImageBlocks)
	if err != nil {
		t.Fatalf("%s: %v", label, err)
	}

	for i, b := range got {
		if v := binary.LittleEndian.Uint32(b); v != uint32(i+1) {
			t.Errorf("%s: PAGES.DAT block %d holds %d, want %d", label, i+1, v, i+1)
		}
	}
}

// TestCopyOnRefSectionPaging runs crfSource with a 30-page working set,
// the writer's thresholds at 10 and 2, and a host page file, and checks
// that the section's pages are charged at mapping, go to the page file
// once modified, and come back from it.
func TestCopyOnRefSectionPaging(t *testing.T) {
	pageFileSettings(t)

	c, out := scheduledConsole(t, longQuantum, brbSelf)
	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t), nil)
	env := c.ProcessOne()

	t.Cleanup(func() {
		if t.Failed() {
			t.Log(out.String())
		}
	})

	db := c.Mem.Paging()
	if db == nil {
		t.Fatal("vax.init's VMINIT left paging off")
	}

	pagesFile := makePagesFile(t, c)
	exe := buildImage(t, c, "crfsec", crfSource)

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

	// Checkpoint 1: every page of the section is charged, though none
	// has been touched, and each still holds its section index.
	checkpoint(1)

	section := c.CPU.GPR(7)
	if section == 0 {
		t.Fatalf("checkpoint 1: the section wasn't mapped (status %08X)", c.CPU.GPR(9))
	}

	for i := range wsPagesBlocks {
		va := section + uint32(i)*512
		if !env.PageFileChargedAt(va) {
			t.Errorf("checkpoint 1: section page %d isn't charged", i)
		}

		if pte := pteIn(t, c, va); !isSectionPTE(pte) {
			t.Errorf("checkpoint 1: section page %d's PTE is %08X, want its section form", i, uint32(pte))
		}
	}

	// Checkpoint 2: each page touched has a frame of its own backed by
	// the page file, never by PAGES.DAT. The pages out of the working set
	// were effectively modified: no more than MPW_HILIMIT wait on the
	// modified list, and the rest are on the free list, written to the
	// page file with their new values.
	release(1)
	checkpoint(2)

	pf := c.RTL.PageFile()
	buf := make([]byte, 512)

	var free, modified []int

	for i := range wsPagesBlocks {
		pte := pteIn(t, c, section+uint32(i)*512)

		pfn, inTransition := pte.Transition()
		if pte.Valid() {
			pfn = pte.PFN()
		} else if !inTransition {
			t.Errorf("checkpoint 2: section page %d's PTE is %08X, neither valid nor in transition", i, uint32(pte))

			continue
		}

		e, err := db.Entry(paging.PFN(pfn))
		if err != nil {
			t.Fatal(err)
		}

		if e.Backing.Kind != paging.PageFile {
			t.Errorf("checkpoint 2: section page %d's frame has backing %+v, want the page file", i, e.Backing)
		}

		if pte.Valid() && !pte.Modified() && !e.SavedModify {
			t.Errorf("checkpoint 2: section page %d is valid but neither modified nor effectively modified", i)
		}

		switch {
		case pte.Valid():
		case e.State == paging.Modified:
			modified = append(modified, i)
		case e.State == paging.Free:
			free = append(free, i)

			if e.Backing.Block == 0 {
				t.Errorf("checkpoint 2: section page %d's frame is on the free list with no block", i)

				continue
			}

			if err := pf.Read(e.Backing.Block, buf); err != nil {
				t.Fatal(err)
			}

			if v := binary.LittleEndian.Uint32(buf); v != uint32(i+1001) {
				t.Errorf("checkpoint 2: section page %d's block %d holds %d, want %d", i, e.Backing.Block, v, i+1001)
			}
		}
	}

	if len(modified) > mpwHiLimit || len(free)+len(modified) < wsPagesBlocks-wsLimit {
		t.Errorf("checkpoint 2: section pages on the free list %v, the modified list %v", free, modified)
	}

	checkPagesFileUnchanged(t, c, pagesFile, "checkpoint 2")

	// Each written page's frame is reused, and its PTE takes the page
	// file form with its block.
	stealFreeFrames(t, c)

	for _, i := range free {
		if vbn, ok := pteIn(t, c, section+uint32(i)*512).PageFileBlock(); !ok || vbn == 0 {
			t.Errorf("section page %d's PTE after its frame was reused isn't a page file block", i)
		}
	}

	checkPageFileHolders(t, c, "after the frames were reused")

	// Checkpoint 3: every page reads back with its new value, from the
	// page file where it was written, and PAGES.DAT is untouched.
	release(2)
	checkpoint(3)

	if got := c.CPU.GPR(9); got != 0 {
		t.Errorf("checkpoint 3: %d of the section's pages read back wrong", got)
	}

	checkPagesFileUnchanged(t, c, pagesFile, "checkpoint 3")

	// The pages read back from the page file and out of the working set
	// again are clean, on the free list with their blocks; reusing their
	// frames leaves them in the page file form for image rundown.
	stealFreeFrames(t, c)

	inPageFile := 0

	for i := range wsPagesBlocks {
		if _, ok := pteIn(t, c, section+uint32(i)*512).PageFileBlock(); ok {
			inPageFile++
		}
	}

	if inPageFile == 0 {
		t.Error("checkpoint 3: no section page is left in the page file form")
	}

	// Image rundown unmaps the section: its pages' charges and blocks
	// go back, whichever form their PTEs had.
	release(3)
	finishImage(t, c)

	for i := range wsPagesBlocks {
		if va := section + uint32(i)*512; env.PageFileChargedAt(va) {
			t.Errorf("after the image: section page %d still charged", i)
		}
	}

	checkPageFileHolders(t, c, "after the image")
	checkPagesFileUnchanged(t, c, pagesFile, "after the image")

	// The section went with its last page, the pages whose copies were
	// only in the page file among them.
	if n := env.ProcessSections(); n != 0 {
		t.Errorf("after the image: process 1 still has %d private sections", n)
	}
}

// TestCopyOnRefSectionPagingOff runs crfSource with paging off: nothing
// is charged, the pages read back with their new values, and the file is
// untouched, as before Phase 65.
func TestCopyOnRefSectionPagingOff(t *testing.T) {
	c, out := scheduledConsole(t, longQuantum, brbSelf)
	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t), nil)
	env := c.ProcessOne()

	if c.Mem.Paging() != nil {
		t.Fatal("paging is on")
	}

	pagesFile := makePagesFile(t, c)
	exe := buildImage(t, c, "crfsec", crfSource)

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
	}

	checkpoint(1)

	section := c.CPU.GPR(7)
	if section == 0 {
		t.Fatalf("checkpoint 1: the section wasn't mapped (status %08X)", c.CPU.GPR(9))
	}

	if env.PageFileChargedAt(section) {
		t.Error("checkpoint 1: a section page is charged with paging off")
	}

	c.CPU.SetGPR(10, 1)
	checkpoint(2)
	c.CPU.SetGPR(10, 2)
	checkpoint(3)

	if got := c.CPU.GPR(9); got != 0 {
		t.Errorf("checkpoint 3: %d of the section's pages read back wrong", got)
	}

	c.CPU.SetGPR(10, 3)
	finishImage(t, c)
	checkPagesFileUnchanged(t, c, pagesFile, "after the image")
}

// crfQuotaSource maps copy-on-reference sections of PAGES.DAT against
// the quota the test leaves. At each checkpoint it waits in a loop until
// the test sets R10 to the checkpoint's number, R6; R9 is the last
// service's status, R7 the private range's first address and R8 the
// global one's.
//
//  1. PAGES.DAT is open on a channel.
//  2. $CRMPSC of a 64-page private section (the test leaves 20 pages).
//  3. $DELTVA of what it mapped.
//  4. $CRMPSC of a 64-page global copy-on-reference section, CRFSEC.
//  5. The first page of the global mapping read.
const crfQuotaSource = `	.title	crfquota
	$fabdef
	$secdef
	.macro	stop,n,?l
	movl	#n,r6
l:	cmpl	r10,r6
	bneq	l
	.endm	stop
	.psect	data,noexe,wrt,page
fab:	$fab	fnm=<DUA0:[000000]PAGES.DAT>,fac=get,fop=ufo,shr=get
inadr:	.long	^x200,^x200
range:	.blkl	2
grange:	.blkl	2
name:	.ascid	/CRFSEC/
	.psect	code,exe,nowrt
	.entry	start,^m<r2,r3,r4,r5,r6,r7,r8,r9,r10,r11>
	clrl	r10
	clrl	r7
	clrl	r8
	$open	fab=fab
	movl	r0,r9
	stop	1
	$crmpsc_s inadr=inadr,retadr=range,flags=#<sec$m_expreg!sec$m_wrt!sec$m_crf>,-
		chan=fab+fab$l_stv,pagcnt=#64
	movl	r0,r9
	movl	range,r7
	stop	2
	$deltva_s inadr=range
	movl	r0,r9
	stop	3
	$crmpsc_s inadr=inadr,retadr=grange,gsdnam=name,-
		flags=#<sec$m_gbl!sec$m_expreg!sec$m_wrt!sec$m_crf>,-
		chan=fab+fab$l_stv,pagcnt=#64
	movl	r0,r9
	movl	grange,r8
	stop	4
	tstl	(r8)
	stop	5
	movl	#1,r0
	ret
	.end	start
`

// TestCopyOnRefSectionQuota runs crfQuotaSource with 20 pages of quota
// left, and checks that each mapping stops at the quota with SS$_EXQUOTA,
// having mapped (and charged) the pages before, and that a global
// copy-on-reference page's fault isn't charged again.
func TestCopyOnRefSectionQuota(t *testing.T) {
	pageFileSettings(t)

	c, out := scheduledConsole(t, longQuantum, brbSelf)
	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t), nil)
	env := c.ProcessOne()
	job := env.Process.Job

	t.Cleanup(func() {
		if t.Failed() {
			t.Log(out.String())
		}
	})

	makePagesFile(t, c)
	exe := buildImage(t, c, "crfquota", crfQuotaSource)

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
	}

	release := func(n uint32) { c.CPU.SetGPR(10, n) }

	status := func(label, name string) {
		t.Helper()

		if got, want := c.CPU.GPR(9), vmsdef.Symbols[name]; got != want {
			t.Errorf("%s: status %08X, want %s (%08X)", label, got, name, want)
		}
	}

	remaining := func(label string, want int64) {
		t.Helper()

		if got := env.PageFileRemaining(); got != want {
			t.Errorf("%s: %d pages of quota left, want %d", label, got, want)
		}
	}

	// mapped counts the pages from first up that are charged, stopping
	// at the first that isn't.
	charged := func(first uint32) int {
		n := 0
		for env.PageFileChargedAt(first + uint32(n)*512) {
			n++
		}

		return n
	}

	checkpoint(1)
	status("$OPEN", "RMS$_NORMAL")

	job.Pooled.PGFLQUOTA = job.PageFileCharged + 20

	// Checkpoint 2: the private section maps 20 pages, each charged.
	release(1)
	checkpoint(2)
	status("$CRMPSC of the private section", "SS$_EXQUOTA")
	remaining("$CRMPSC of the private section", 0)

	if n := charged(c.CPU.GPR(7)); n != 20 {
		t.Errorf("$CRMPSC of the private section: %d pages charged, want 20", n)
	}

	// Checkpoint 3: $DELTVA gives the 20 pages' charges back.
	release(2)
	checkpoint(3)
	status("$DELTVA", "SS$_NORMAL")
	remaining("$DELTVA", 20)

	// Checkpoint 4: the global section is created, and its mapping stops
	// at the quota as the private one's did.
	release(3)
	checkpoint(4)
	status("$CRMPSC of the global section", "SS$_EXQUOTA")
	remaining("$CRMPSC of the global section", 0)

	global := c.CPU.GPR(8)
	if n := charged(global); n != 20 {
		t.Errorf("$CRMPSC of the global section: %d pages charged, want 20", n)
	}

	// Checkpoint 5: the first page's fault makes the process's own copy,
	// backed by the page file, already charged.
	release(4)
	checkpoint(5)
	remaining("the global page's fault", 0)

	pte := pteIn(t, c, global)
	if !pte.Valid() {
		t.Fatalf("the global section's first page's PTE is %08X after its read", uint32(pte))
	}

	if e, err := c.Mem.Paging().Entry(paging.PFN(pte.PFN())); err != nil || e.Type != paging.ProcessPage ||
		e.Backing.Kind != paging.PageFile || !e.SavedModify {
		t.Errorf("the global copy's frame is %+v (%v), want an effectively modified process page backed by the page file", e, err)
	}

	release(5)
	finishImage(t, c)

	if job.PageFileCharged != env.PageFileCharged() {
		t.Errorf("after the image: the job has %d pages charged, process 1 %d", job.PageFileCharged, env.PageFileCharged())
	}
}
