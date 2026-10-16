package console_test

import (
	"encoding/binary"
	"path/filepath"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
	"github.com/tucats/govax/internal/machine"
	"github.com/tucats/govax/internal/paging"
	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vm"
)

// Phase 63's subtask 7 (docs/PHASE-63 - working sets.md): the modified
// page writer writes a writable private section's pages from the
// modified list to their file, once the list is over MPW_HILIMIT, and
// moves their frames to the free list; pages it can't write (demand-zero
// pages, with no page file yet) stay on the list without stopping it;
// and $UPDSEC writes the pages still on the list, and those soft-faulted
// back whose modify bit is saved in their frames.

// The writer's thresholds the test sets: small, so a 64-page section
// passes them several times.
const (
	mpwHiLimit = 10
	mpwLoLimit = 2
)

// mpwSource writes each of 80 pages of an array (checkpoint 1); maps
// PAGES.DAT as a writable private section, adds 1000 to the longword at
// the start of each of its 64 pages, and reads pages 20 to 33 again
// (checkpoint 2, R7 the section's first address); calls $UPDSEC for the
// whole section (checkpoint 3, R9 its status); reads every page of the
// section back, counting in R9 those that don't hold their block number
// plus 1000 (checkpoint 4); and returns, which unmaps the section. At
// each checkpoint it waits in a loop until the test sets R10 to the
// checkpoint's number, R6. R8 is the array's address, and R9 a failing
// service's status before checkpoint 3.
const mpwSource = `	.title	wsmpw
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
buf:	.blkb	512*80
fab:	$fab	fnm=<DUA0:[000000]PAGES.DAT>,fac=<get,put>,fop=ufo,shr=get
inadr:	.long	^x200,^x200
range:	.blkl	2
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
	$open	fab=fab
	ok
	$crmpsc_s inadr=inadr,retadr=range,flags=#<sec$m_expreg!sec$m_wrt>,-
		chan=fab+fab$l_stv,pagcnt=#64
	ok
	movl	range,r2
	movl	#64,r3
20$:	addl2	#1000,(r2)
	addl2	#512,r2
	sobgtr	r3,20$
	movl	range,r2
	addl2	#512*20,r2
	movl	#14,r3
30$:	tstl	(r2)
	addl2	#512,r2
	sobgtr	r3,30$
	movl	range,r7
	stop	2
	$updsec_s inadr=range
	movl	r0,r9
	stop	3
	movl	range,r2
	clrl	r9
	movl	#1001,r4
40$:	cmpl	(r2),r4
	beql	41$
	incl	r9
41$:	incl	r4
	addl2	#512,r2
	cmpl	r4,#1065
	blss	40$
	stop	4
	movl	#1,r0
	ret
fail:	brb	fail
	.end	start
`

// TestModifiedPageWriter runs mpwSource with a 30-page working set
// and the writer's thresholds at 10 and 2, and checks at each checkpoint
// where the section's pages are and what its file holds.
func TestModifiedPageWriter(t *testing.T) {
	setSetting(t, machine.PagingSetting, "true")
	setSetting(t, "vax.sysgen.pql_dwsdefault", "30")
	setSetting(t, "vax.sysgen.mpw_hilimit", "10")
	setSetting(t, "vax.sysgen.mpw_lolimit", "2")

	c, out := scheduledConsole(t, longQuantum, brbSelf)
	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t), nil)

	db := c.Mem.Paging()
	if db == nil {
		t.Fatal("vax.init's VMINIT left paging off")
	}

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

	exe := buildImage(t, c, "wsmpw", mpwSource)

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

	// fileHolds checks which of PAGES.DAT's blocks hold their new value
	// (their number plus 1000), returning a block's number for each, and
	// failing the test for a block holding anything but its old or new
	// value.
	fileHolds := func(label string) map[int]bool {
		t.Helper()

		got, _, err := c.ContainerSession.ReadRecordFile(pagesFile, rms.ImageBlocks)
		if err != nil {
			t.Fatalf("%s: %v", label, err)
		}

		if len(got) != wsPagesBlocks {
			t.Fatalf("%s: PAGES.DAT has %d blocks, want %d", label, len(got), wsPagesBlocks)
		}

		updated := map[int]bool{}

		for i, b := range got {
			switch v := binary.LittleEndian.Uint32(b); v {
			case uint32(i + 1001):
				updated[i] = true
			case uint32(i + 1):
			default:
				t.Errorf("%s: PAGES.DAT block %d holds %d", label, i+1, v)
			}
		}

		return updated
	}

	// sectionPages sorts the section's pages in transition by the list
	// their frames are on, and finds the valid pages whose PTEs are
	// clean but whose frames' saved modify bits are set.
	type where struct{ free, modified, saved []int }

	sectionPages := func(section uint32) where {
		t.Helper()

		var w where

		for i := range wsPagesBlocks {
			pte := pteIn(t, c, section+uint32(i)*512)

			pfn, inTransition := pte.Transition()
			if pte.Valid() {
				pfn = pte.PFN()
			} else if !inTransition {
				continue
			}

			e, err := db.Entry(paging.PFN(pfn))
			if err != nil {
				t.Fatal(err)
			}

			switch {
			case pte.Valid() && !pte.Modified() && e.SavedModify:
				w.saved = append(w.saved, i)
			case pte.Valid():
			case e.State == paging.Free:
				w.free = append(w.free, i)
			case e.State == paging.Modified:
				w.modified = append(w.modified, i)
			}
		}

		return w
	}

	// Checkpoint 1: the array's demand-zero pages wait on the modified
	// list, more of them than MPW_HILIMIT: the writer can't write them,
	// and leaves them.
	checkpoint(1)

	stuck := db.ModifiedCount()
	if stuck < 80-wsLimit {
		t.Fatalf("checkpoint 1: %d frames on the modified list, want at least %d", stuck, 80-wsLimit)
	}

	// Checkpoint 2: of the section's 64 pages, the 30 last touched are in
	// the working set; of the others, the writer has written all but at
	// most MPW_HILIMIT, whose frames are on the free list, and the file
	// holds those pages' new values. Pages 20 to 33, read again, came
	// back by soft faults; any that came off the modified list have
	// clean PTEs and their modify bits saved.
	release(1)
	checkpoint(2)

	section := c.CPU.GPR(7)
	if section == 0 {
		t.Fatalf("checkpoint 2: the section wasn't mapped (status %08X)", c.CPU.GPR(9))
	}

	w := sectionPages(section)
	if len(w.modified) > mpwHiLimit || len(w.free) < wsPagesBlocks-wsLimit-mpwHiLimit {
		t.Errorf("checkpoint 2: section pages on the free list %v, the modified list %v", w.free, w.modified)
	}

	if len(w.saved) == 0 {
		t.Error("checkpoint 2: no section page is back with its modify bit saved")
	}

	if n := db.ModifiedCount(); n < stuck {
		t.Errorf("checkpoint 2: %d frames on the modified list, fewer than the array's %d", n, stuck)
	}

	updated := fileHolds("checkpoint 2")
	for _, i := range w.free {
		if !updated[i] {
			t.Errorf("checkpoint 2: page %d's frame is on the free list, but its block wasn't written", i)
		}
	}

	// Checkpoint 3: $UPDSEC writes every page the writer hadn't: those
	// in the working set, those on the modified list, and those back
	// with their modify bits saved. No page of the section is left
	// modified.
	release(2)
	checkpoint(3)

	if got := c.CPU.GPR(9); got != 1 {
		t.Fatalf("checkpoint 3: $UPDSEC returned %08X", got)
	}

	w = sectionPages(section)
	if len(w.modified) != 0 || len(w.saved) != 0 {
		t.Errorf("checkpoint 3: section pages still on the modified list %v, with modify bits saved %v", w.modified, w.saved)
	}

	if updated := fileHolds("checkpoint 3"); len(updated) != wsPagesBlocks {
		t.Errorf("checkpoint 3: %d of PAGES.DAT's blocks updated, want %d", len(updated), wsPagesBlocks)
	}

	// Take every free frame, so each written page's frame is reused and
	// its PTE goes back to its section form; the program then reads every
	// page back from the file.
	var taken []uint32

	for db.FreeCount() > 0 {
		pfn, ok := c.Mem.AllocateFrame(vm.FrameClaim{})
		if !ok {
			t.Fatal("AllocateFrame failed with frames free")
		}

		taken = append(taken, pfn)
	}

	for _, pfn := range taken {
		c.Mem.FreePage(pfn)
	}

	for _, i := range w.free {
		if pte := pteIn(t, c, section+uint32(i)*512); !isSectionPTE(pte) {
			t.Errorf("section page %d's PTE %08X after its frame was reused, want its section form", i, uint32(pte))
		}
	}

	// Checkpoint 4: every page reads back with its new value.
	release(3)
	checkpoint(4)

	if got := c.CPU.GPR(9); got != 0 {
		t.Errorf("checkpoint 4: %d of the section's pages read back wrong", got)
	}

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

	checkWorkingSets(t, c)

	if updated := fileHolds("after the image"); len(updated) != wsPagesBlocks {
		t.Errorf("after the image: %d of PAGES.DAT's blocks updated, want %d", len(updated), wsPagesBlocks)
	}
}
