package console_test

import (
	"encoding/binary"
	"path/filepath"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
	"github.com/tucats/govax/internal/machine"
	"github.com/tucats/govax/internal/paging"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vm"
)

// Phase 63's subtask 3 (docs/PHASE-63 - working sets.md): with paging on,
// each page a process faults in enters its working set; a full working
// set gives up its oldest page, whose PTE goes into transition and whose
// frame goes to the modified list (a demand-zero page) or the free list
// (a clean page of a file); a touch of a page in transition is a soft
// fault; and a frame reused from the free list sends its old page's PTE
// back to its section form, so the next touch reads the page again.

// wsLimit is the working set limit the tests give process 1, through
// PQL_DWSDEFAULT: small, so the programs' pages don't all fit.
const wsLimit = 30

// wsPagesBlocks is how many blocks PAGES.DAT has: block n's first
// longword is n.
const wsPagesBlocks = 64

// workingSetSource writes the page number into each of 80 pages of an
// array and reads them all back, twice; reads the first 34 back four
// times; maps PAGES.DAT as a private read-only section and reads each
// page's number, twice; and spins with R6 999, R7 the section's first
// address, and R8 the array's. R6 is the step that failed otherwise.
// Each sweep is longer than the working set, so with replacement
// strictly oldest first, every touch faults.
const workingSetSource = `	.title	wsets
	$fabdef
	$secdef
	.macro	ok,?l
	blbs	r0,l
	brw	spin
l:
	.endm	ok
	.psect	data,noexe,wrt,page
buf:	.blkb	512*80
fab:	$fab	fnm=<DUA0:[000000]PAGES.DAT>,fac=get,fop=ufo,shr=get
inadr:	.long	^x200,^x200
range:	.blkl	2
	.psect	code,exe,nowrt
	.entry	start,^m<>
	movl	#1,r6
	moval	buf,r2
	movl	#80,r3
	clrl	r4
10$:	incl	r4
	movl	r4,(r2)
	addl2	#512,r2
	sobgtr	r3,10$
	movl	#80,r5
	movl	#2,r6
	bsbw	check_buf
	movl	#3,r6
	bsbw	check_buf
	movl	#34,r5
	movl	#7,r6
15$:	bsbw	check_buf
	incl	r6
	cmpl	r6,#11
	blss	15$
	movl	#4,r6
	$open	fab=fab
	ok
	$crmpsc_s inadr=inadr,retadr=range,flags=#sec$m_expreg,-
		chan=fab+fab$l_stv,pagcnt=#64
	ok
	movl	#5,r6
	bsbw	check_map
	movl	#6,r6
	bsbw	check_map
	movl	#999,r6
	movl	range,r7
	moval	buf,r8
	brw	spin
check_buf:
	moval	buf,r2
	movl	r5,r3
	clrl	r4
20$:	incl	r4
	cmpl	r4,(r2)
	bneq	spin
	addl2	#512,r2
	sobgtr	r3,20$
	rsb
check_map:
	movl	range,r2
	movl	#64,r3
	clrl	r4
30$:	incl	r4
	cmpl	r4,(r2)
	bneq	spin
	addl2	#512,r2
	sobgtr	r3,30$
	rsb
spin:	brb	spin
	.end	start
`

// TestWorkingSetReplacement runs workingSetSource with a 30-page working
// set until the instruction limit stops it in its spin, then looks at
// what the sweeps left: the working set at its limit; the array's pages
// that left it in transition on the modified list, the section's on the
// free list; and, with TBSKIPWSL 0 (replacement strictly oldest first),
// every touch counted as a fault. It then reads the array's pages from
// Go, which come back off the modified list in the frames they left in
// (soft faults); reuses the free list's frames, which sends the
// section's pages back to their section form; and reads the section's
// pages, which are read from the file again (hard faults). Each page
// holds what it held.
//
// With TBSKIPWSL 8, the default, each scan passes over the oldest pages
// while they're still in the translation buffer (in a sweep, all of
// them: the TB has a slot for each of 32 pages in a row), which gives
// them a second chance; a sweep a little longer than the working set,
// as the 34-page ones are, then finds many of its pages still there.
func TestWorkingSetReplacement(t *testing.T) {
	strict := workingSetRun(t, "0")
	skipping := workingSetRun(t, "8")

	t.Logf("the program's faults: %d with TBSKIPWSL 0, %d with 8", strict, skipping)

	if skipping >= strict {
		t.Errorf("%d faults with TBSKIPWSL 8, %d with 0; want fewer", skipping, strict)
	}
}

// workingSetRun is TestWorkingSetReplacement with TBSKIPWSL tbskip, and
// returns the faults the program's run counted.
func workingSetRun(t *testing.T, tbskip string) uint32 {
	t.Helper()

	setSetting(t, machine.PagingSetting, "true")
	setSetting(t, "vax.sysgen.pql_dwsdefault", "30")
	setSetting(t, "vax.sysgen.tbskipwsl", tbskip)

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

	blocks := make([][]byte, wsPagesBlocks)
	for i := range blocks {
		blocks[i] = make([]byte, 512)
		binary.LittleEndian.PutUint32(blocks[i], uint32(i+1))
	}

	if _, err := c.ContainerSession.CreateRecordFile(rms.FileLocation{Name: "DUA0:[000000]PAGES.DAT"}, rms.ImageBlocks, blocks); err != nil {
		t.Fatal(err)
	}

	env := c.ProcessOne()
	exe := buildImage(t, c, "wsets", workingSetSource)
	faults := env.Process.PageFaults

	c.Engine.SetLimits(500_000, 0)
	runImage(t, d, exe)
	c.Engine.SetLimits(0, 0)

	if !c.ImageActive() || c.CPU.GPR(6) != 999 {
		t.Fatalf("the program stopped at step %d (active %v)\n%s", c.CPU.GPR(6), c.ImageActive(), out.String())
	}

	section, array := c.CPU.GPR(7), c.CPU.GPR(8)

	ws := env.WorkingSet()

	counts := ws.Counts()
	if counts.Size != wsLimit || counts.Entries != wsLimit || ws.InUse() > wsLimit {
		t.Errorf("working set counts %+v, want a size of %d", counts, wsLimit)
	}

	// Each of the program's sweeps passes over more pages than the
	// working set holds, so with replacement oldest first, each touch
	// faults.
	faults = env.Process.PageFaults - faults
	if want := uint32(80*3 + 34*4 + 64*2); tbskip == "0" && faults < want {
		t.Errorf("%d page faults, want at least %d", faults, want)
	}

	checkWorkingSets(t, c)

	arrayOut := pagesInTransition(t, c, array, 80, paging.Modified)
	sectionOut := pagesInTransition(t, c, section, wsPagesBlocks, paging.Free)

	if len(arrayOut) < 80-wsLimit || len(sectionOut) < wsPagesBlocks-wsLimit {
		t.Fatalf("%d of the array's pages and %d of the section's in transition", len(arrayOut), len(sectionOut))
	}

	// Read the array's pages, an instruction at a time as far as the
	// frames translated are concerned. Each page in transition comes back
	// in its own frame.
	frames := map[uint32]uint32{}
	for _, va := range arrayOut {
		frames[va], _ = pteIn(t, c, va).Transition()
	}

	for i := range uint32(80) {
		va := array + i*512

		c.Mem.BeginInstruction()

		if v, err := c.Mem.LoadLongword(c.CPU, va); err != nil || v != i+1 {
			t.Errorf("array page %d holds %d (%v), want %d", i, v, err, i+1)
		}

		if pfn, out := frames[va]; out {
			if pte := pteIn(t, c, va); !pte.Valid() || pte.PFN() != pfn {
				t.Errorf("array page %08X left in frame %d, came back with PTE %08X", va, pfn, uint32(pte))
			}
		}
	}

	checkWorkingSets(t, c)
	sectionOut = pagesInTransition(t, c, section, wsPagesBlocks, paging.Free)

	// Take every free frame (system frames, tied to nothing), so that
	// each section page's frame is reused and its tie broken; then give
	// them back.
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

	for _, va := range sectionOut {
		if pte := pteIn(t, c, va); !isSectionPTE(pte) {
			t.Errorf("section page %08X's PTE %08X after its frame was reused, want its section form", va, uint32(pte))
		}
	}

	if err := db.Check(); err != nil {
		t.Fatalf("after the frames were reused: %v", err)
	}

	for i := range uint32(wsPagesBlocks) {
		c.Mem.BeginInstruction()

		if v, err := c.Mem.LoadLongword(c.CPU, section+i*512); err != nil || v != i+1 {
			t.Errorf("section page %d holds %d (%v), want %d", i, v, err, i+1)
		}
	}

	checkWorkingSets(t, c)

	return faults
}

// pteIn is the PTE of process 1's page at va.
func pteIn(t *testing.T, c *console.Console, va uint32) vm.PTE {
	t.Helper()

	pte, err := c.Mem.LookupPTEIn(c.CPU, c.ProcessOne().Space.AddressSpace, va)
	if err != nil {
		t.Fatalf("PTE of %08X: %v", va, err)
	}

	return pte
}

// isSectionPTE reports whether pte is the process section table form.
func isSectionPTE(pte vm.PTE) bool {
	_, ok := pte.SectionIndex()

	return ok
}

// pagesInTransition returns the addresses of the n pages of process 1
// from first whose PTEs are in transition, checking that each frame is
// on list want and still tied to its page.
func pagesInTransition(t *testing.T, c *console.Console, first, n uint32, want paging.State) []uint32 {
	t.Helper()

	db := c.Mem.Paging()
	pid := c.ProcessOne().Process.PID

	var out []uint32

	for i := range n {
		va := first + i*512

		pfn, ok := pteIn(t, c, va).Transition()
		if !ok {
			continue
		}

		out = append(out, va)

		e, err := db.Entry(paging.PFN(pfn))
		if err != nil || e.State != want || !e.Tied || e.Owner != (paging.Owner{Handle: pid, VA: va}) {
			t.Errorf("page %08X in transition in frame %d: %+v (%v), want on the %s list", va, pfn, e, err, want)
		}
	}

	return out
}

// checkWorkingSets checks that each process's working set and the PFN
// database agree: each page in a working set is valid, its frame active
// (a process page's tied to it); each active process frame is in its
// page's working set; and each tied frame on a list is still its page's,
// whose PTE is in transition in that frame.
func checkWorkingSets(t *testing.T, c *console.Console) {
	t.Helper()

	db := c.Mem.Paging()
	if err := db.Check(); err != nil {
		t.Fatalf("the PFN database: %v", err)
	}

	for _, env := range c.RTL.Processes() {
		ws := env.WorkingSet()
		if err := ws.Check(); err != nil {
			t.Fatalf("PID %08X's working set: %v", env.Process.PID, err)
		}

		for _, p := range ws.Pages() {
			pte, err := c.Mem.LookupPTEIn(c.CPU, env.Space.AddressSpace, p.VA)
			if err != nil || !pte.Valid() {
				t.Errorf("PID %08X's working set page %08X has PTE %08X (%v)", env.Process.PID, p.VA, uint32(pte), err)

				continue
			}

			e, _ := db.Entry(paging.PFN(pte.PFN()))
			if e.State != paging.Active || !p.Global && e.Owner != (paging.Owner{Handle: env.Process.PID, VA: p.VA}) {
				t.Errorf("PID %08X's working set page %08X: frame %d is %+v", env.Process.PID, p.VA, pte.PFN(), e)
			}
		}
	}

	for n := range db.Frames() {
		e, _ := db.Entry(paging.PFN(n))
		if !e.Tied || e.Type != paging.ProcessPage {
			continue
		}

		env, ok := c.RTL.FindProcess(e.Owner.Handle)
		if !ok {
			t.Errorf("frame %d: tied to PID %08X, which doesn't exist", n, e.Owner.Handle)

			continue
		}

		pte, err := c.Mem.LookupPTEIn(c.CPU, env.Space.AddressSpace, e.Owner.VA)
		if err != nil {
			t.Fatal(err)
		}

		if e.State == paging.Active {
			if _, ok := env.WorkingSet().Lookup(e.Owner.VA); !ok {
				t.Errorf("frame %d: PID %08X's page %08X isn't in its working set", n, e.Owner.Handle, e.Owner.VA)
			}

			continue
		}

		if pfn, ok := pte.Transition(); !ok || pfn != uint32(n) {
			t.Errorf("frame %d on the %s list: its page %08X has PTE %08X", n, e.State, e.Owner.VA, uint32(pte))
		}
	}
}
