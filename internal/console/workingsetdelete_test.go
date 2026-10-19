package console_test

import (
	"encoding/binary"
	"io"
	"path/filepath"
	"slices"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/machine"
	"github.com/tucats/govax/internal/paging"
	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vm"
)

// Phase 63's subtask 5 (docs/PHASE-63 - working sets.md): a page deleted
// while it's in transition, out of the working set but still holding its
// frame on the free or modified list, gives its frame up as a valid page
// does, to the head of the free list, untied; and a writable private
// section's page written before it left the working set is still written
// back to its file when the section goes.

// deleteSource touches each of 80 pages of an array (checkpoint 1);
// recreates the first 40 with $CRETVA (checkpoint 2) and deletes the
// other 40 with $DELTVA (checkpoint 3), R9 each service's status; maps
// PAGES.DAT as a writable private section, adds 1000 to the longword at
// the start of each of its 64 pages, and reads the first 10 pages again
// (checkpoint 4, R7 the section's first address); and returns, which
// unmaps the section. At each checkpoint it waits in a loop until the
// test sets R10 to the checkpoint's number, R6. R8 is the array's
// address, and R9 a failing service's status before checkpoint 4.
const deleteSource = `	.title	wsdel
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
low:	.address buf,buf+<512*39>
high:	.address buf+<512*40>,buf+<512*79>
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
	clrq	-(sp)
	pushal	low
	calls	#3,g^sys$cretva
	movl	r0,r9
	stop	2
	clrq	-(sp)
	pushal	high
	calls	#3,g^sys$deltva
	movl	r0,r9
	stop	3
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
	movl	#10,r3
30$:	tstl	(r2)
	addl2	#512,r2
	sobgtr	r3,30$
	movl	range,r7
	stop	4
	movl	#1,r0
	ret
fail:	brb	fail
	.end	start
`

// TestDeleteTransitionPages runs deleteSource with a 30-page working set
// and checks, at each checkpoint, that the pages deleted while in
// transition gave their frames up, and after the image's exit, that the
// section's pages reached its file, including those on the modified list
// and those soft-faulted back with clean PTEs.
func TestDeleteTransitionPages(t *testing.T) {
	setSetting(t, machine.PagingSetting, "true")
	setSetting(t, "vax.sysgen.pql_dwsdefault", "30")

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

	exe := buildImage(t, c, "wsdel", deleteSource)

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

	// Checkpoint 1: 80 pages through a 30-page working set leave at
	// least 50 in transition, on the modified list (demand-zero pages).
	checkpoint(1)

	array := c.CPU.GPR(8)

	frames := map[uint32]uint32{}
	for _, va := range pagesInTransition(t, c, array, 80, paging.Modified) {
		frames[va], _ = pteIn(t, c, va).Transition()
	}

	if len(frames) < 80-wsLimit {
		t.Fatalf("checkpoint 1: %d of the array's pages in transition, want at least %d", len(frames), 80-wsLimit)
	}

	// record notes the frames the pages from first, n of them, hold now,
	// valid or in transition, just before they're deleted: a page still
	// valid at checkpoint 1 may have left the working set since, or may
	// still be in it, and its frame is freed with the others.
	record := func(first, n uint32) {
		t.Helper()

		for _, va := range pagesInTransition(t, c, first, n, paging.Modified) {
			frames[va], _ = pteIn(t, c, va).Transition()
		}

		for i := range n {
			if pte := pteIn(t, c, first+i*512); pte.Valid() {
				frames[first+i*512] = pte.PFN()
			}
		}
	}

	// deleted checks that the pages from first, n of them, gave up the
	// frames they held in transition: each frame is free and tied to
	// nothing, and a new frame comes from among them (the free list's
	// head). Their PTEs must no longer be in transition.
	deleted := func(label string, first, n uint32) {
		t.Helper()

		var freed []uint32

		for i := range n {
			va := first + i*512

			if _, ok := pteIn(t, c, va).Transition(); ok {
				t.Errorf("%s: page %08X still in transition", label, va)
			}

			pfn, ok := frames[va]
			if !ok {
				continue
			}

			freed = append(freed, pfn)

			if e, _ := db.Entry(paging.PFN(pfn)); e.State != paging.Free || e.Tied {
				t.Errorf("%s: page %08X's frame %d is %+v, want free and untied", label, va, pfn, e)
			}
		}

		if len(freed) == 0 {
			t.Fatalf("%s: none of the pages was in transition", label)
		}

		pfn, ok := c.Mem.AllocateFrame(vm.FrameClaim{})
		if !ok || !slices.Contains(freed, pfn) {
			t.Errorf("%s: the next frame allocated is %d, want one of the deleted pages' %v", label, pfn, freed)
		}

		c.Mem.FreePage(pfn)
	}

	// Checkpoint 2: $CRETVA over the first 40 pages makes them new
	// demand-zero pages; those in transition give up their frames.
	release(1)
	checkpoint(2)

	if got := c.CPU.GPR(9); got != 1 {
		t.Fatalf("checkpoint 2: $CRETVA returned %08X", got)
	}

	deleted("$CRETVA", array, 40)

	// Checkpoint 3: $DELTVA of the other 40 does the same.
	record(array+40*512, 40)
	release(2)
	checkpoint(3)

	if got := c.CPU.GPR(9); got != 1 {
		t.Fatalf("checkpoint 3: $DELTVA returned %08X", got)
	}

	deleted("$DELTVA", array+40*512, 40)

	// Checkpoint 4: the section's 64 pages, each written, have passed
	// through the working set: most are on the modified list, and the
	// first 10 are back in it, clean in their PTEs.
	release(3)
	checkpoint(4)

	section := c.CPU.GPR(7)
	if section == 0 {
		t.Fatalf("checkpoint 4: the section wasn't mapped (status %08X)", c.CPU.GPR(9))
	}

	if n := len(pagesInTransition(t, c, section, wsPagesBlocks, paging.Modified)); n < wsPagesBlocks-wsLimit {
		t.Errorf("checkpoint 4: %d of the section's pages in transition, want at least %d", n, wsPagesBlocks-wsLimit)
	}

	if pte := pteIn(t, c, section); !pte.Valid() || pte.Modified() {
		t.Errorf("checkpoint 4: the section's first page has PTE %08X, want valid and unmodified", uint32(pte))
	}

	// The image's return unmaps the section, writing every modified page
	// back.
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

	got, _, err := c.ContainerSession.ReadRecordFile(pagesFile, rms.ImageBlocks)
	if err != nil {
		t.Fatal(err)
	}

	for i := range wsPagesBlocks {
		if i >= len(got) || len(got[i]) < 4 {
			t.Fatalf("PAGES.DAT has %d blocks, want %d", len(got), wsPagesBlocks)
		}

		if v := binary.LittleEndian.Uint32(got[i]); v != uint32(i+1001) {
			t.Errorf("PAGES.DAT block %d holds %d, want %d", i+1, v, i+1001)
		}
	}
}

// TestDeleteProcessTransitionPages touches 80 pages of a second process
// with a 30-page working set, which leaves at least 50 of them in
// transition, then removes the process: every frame it was given goes
// back, none stays tied to it.
func TestDeleteProcessTransitionPages(t *testing.T) {
	setSetting(t, machine.PagingSetting, "true")

	c, _ := scheduledConsole(t, longQuantum, brbSelf)
	sys := c.RTL.System

	db := c.Mem.Paging()
	if db == nil {
		t.Fatal("vax.init's VMINIT left paging off")
	}

	before := db.Counts()

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

	inTransition := 0

	for i := range uint32(80) {
		pte, err := c.Mem.LookupPTEIn(c.CPU, env.Space.AddressSpace, 0x200+i*512)
		if err != nil {
			t.Fatal(err)
		}

		if _, ok := pte.Transition(); ok {
			inTransition++
		}
	}

	if inTransition < 80-wsLimit {
		t.Fatalf("%d pages in transition, want at least %d", inTransition, 80-wsLimit)
	}

	checkWorkingSets(t, c)

	sys.RemoveProcess(env)

	checkWorkingSets(t, c)

	if after := db.Counts(); after != before {
		t.Errorf("frame counts %+v after the process went, want %+v", after, before)
	}
}
