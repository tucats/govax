package console_test

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
	"github.com/tucats/govax/internal/machine"
	"github.com/tucats/govax/internal/paging"
	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vm"
)

// Phase 66's subtask 3 (docs/PHASE-66 - mapped image activation.md):
// with paging on, RUN maps the image rather than copying it. Each
// private ISD is a section of the image file whose pages come in when
// they're touched, each demand-zero ISD demand-zero pages; the fixups
// fault in the pages they use; every section gets its protection, and
// the fixup page the change-protection list's UREW, whatever form its
// PTEs are in; the image file is held while the image runs; and the
// debugger's DEPOSIT into a read-only page keeps its change, the page
// backed by the page file from then on.

// The image's ISD flags the tests look at (ISD$V_GBL, ISD$V_DZRO,
// ISD$V_WRT, ISD$V_FIXUPVEC, and ISD_K_USRSTACK in the top byte).
const (
	mapISDGBL      = 1 << 0
	mapISDDZRO     = 1 << 2
	mapISDWRT      = 1 << 3
	mapISDFIXUPVEC = 1 << 10
	mapISDUsrStack = 253
)

// imageMapSource waits at checkpoint 1 (R6, released by the test's R10,
// as mpwSource's are) with only its first code page run; then writes
// each of the 40 pages of its demand-zero array, which pushes its other
// pages out of a small working set (checkpoint 2); then says hello
// through LIB$PUT_OUTPUT (a G^ fixup) and returns. R8 is the address of
// the code past the first page, 4 pages that are never run.
const imageMapSource = `	.title	imap
	.macro	stop,n,?l
	movl	#n,r6
l:	cmpl	r10,r6
	bneq	l
	.endm	stop
	.psect	data,noexe,wrt
msg:	.ascid	/mapped hello/
	.psect	zero,noexe,wrt,page
array:	.blkb	512*40
	.psect	code,exe,nowrt,page
	.entry	start,^m<r2,r3,r6,r8,r10>
	clrl	r10
	moval	unrun,r8
	stop	1
	moval	array,r2
	movl	#40,r3
10$:	movl	r3,(r2)
	addl2	#512,r2
	sobgtr	r3,10$
	stop	2
	pushab	msg
	calls	#1,g^lib$put_output
	movl	#1,r0
	ret
	.align	page
unrun:	.blkb	512*4
	.end	start
`

// mappedImageConsole boots a console with paging on and a working set
// of 30 pages, mounts a fresh volume as DUA0, and puts the
// imageMapSource image there, returning its file name and the console's
// output. pfc is PFCDEFAULT, the image's sections' read cluster: "1"
// reads only the page faulted on, so a page is valid only if it was
// touched; "" leaves it at its default.
func mappedImageConsole(t *testing.T, pfc string) (*console.Console, *console.Dispatcher, string, *bytes.Buffer) {
	t.Helper()

	setSetting(t, machine.PagingSetting, "true")
	setSetting(t, "vax.sysgen.pql_dwsdefault", "30")
	if pfc != "" {
		setSetting(t, machine.PFCDefaultSetting, pfc)
	}

	c, out := scheduledConsole(t, longQuantum, brbSelf)
	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t), nil)

	if c.Mem.Paging() == nil {
		t.Fatal("vax.init's VMINIT left paging off")
	}

	path := filepath.Join(t.TempDir(), "work.dsk")
	if err := c.Commands().InitializeContainer(path, 400, "WORK", 0, "RD54"); err != nil {
		t.Fatal(err)
	}

	if err := c.Commands().Mount("DUA0", path, true); err != nil {
		t.Fatal(err)
	}

	host := buildImage(t, c, "imap", imageMapSource)
	name := "DUA0:[000000]IMAP.EXE"

	if err := c.Commands().Copy(host, true, name, false, rms.CopyOptions{Binary: true, Quiet: true}); err != nil {
		t.Fatalf("Copy: %v", err)
	}

	return c, d, name, out
}

// startMappedImage runs the image name until it reaches checkpoint 1.
func startMappedImage(t *testing.T, c *console.Console, d *console.Dispatcher, name string) {
	t.Helper()

	c.Engine.SetLimits(1, 0)

	if err := d.Dispatch("RUN " + name); err != nil {
		t.Fatalf("RUN %s: %v", name, err)
	}

	c.Engine.SetLimits(0, 0)

	if !c.ImageActive() {
		t.Fatal("the image isn't running")
	}

	runUntil(t, c, 200_000, func() bool { return c.CPU.GPR(6) == 1 })
	step(t, c, 4)
}

// toCheckpoint2 releases the image from checkpoint 1 and runs it to
// checkpoint 2.
func toCheckpoint2(t *testing.T, c *console.Console) {
	t.Helper()

	c.CPU.SetGPR(10, 1)
	runUntil(t, c, 200_000, func() bool { return c.CPU.GPR(6) == 2 })
	step(t, c, 4)
}

// finishMappedImage releases the image from checkpoint 2 (from 1 too, if
// it's there) and runs it to its end.
func finishMappedImage(t *testing.T, c *console.Console) {
	t.Helper()

	if c.CPU.GPR(6) == 1 {
		toCheckpoint2(t, c)
	}

	c.CPU.SetGPR(10, 2)

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
}

// TestMappedImage: at its first stop the image's pages are mostly still
// mapped to its file, with their sections' protections; its writable
// and demand-zero pages, but not its read-only ones, are charged; the
// file is held (its volume can't be dismounted); and the image runs to
// its end, after which its sections, and the file, are gone.
func TestMappedImage(t *testing.T) {
	c, d, name, _ := mappedImageConsole(t, "1")
	env := c.ProcessOne()

	startMappedImage(t, c, d, name)

	running := env.PageFileRemaining()

	main := c.FindMainICB()
	if main == nil {
		t.Fatal("no main image")
	}

	sections, charged := 0, int64(0)
	sawDZRO, sawRO, sawFixup := false, false, false

	for _, isd := range main.ISDList {
		if isd.Flags>>24 == mapISDUsrStack || isd.Flags&mapISDGBL != 0 {
			continue
		}

		want := vm.ProtUR

		switch {
		case isd.Flags&mapISDFIXUPVEC != 0:
			want, sawFixup = vm.ProtUREW, true
		case isd.Flags&mapISDWRT != 0:
			want = vm.ProtUW
		default:
			sawRO = true
		}

		if isd.Flags&mapISDDZRO != 0 {
			sawDZRO = true
		} else {
			sections++
		}

		if isd.Flags&mapISDWRT != 0 {
			charged += int64(isd.Pages)
		}

		for n := range uint32(isd.Pages) {
			va := main.Base + (isd.VPN+n)<<9
			pte := pteIn(t, c, va)

			if got := pte.Protection(); got != want {
				t.Errorf("page %08X (ISD flags %08X): PROT %s, want %s", va, isd.Flags, got, want)
			}

			// A demand-zero page nothing has touched is still
			// demand-zero; the code nothing has run is still mapped to
			// the file.
			switch {
			case isd.Flags&mapISDDZRO != 0 && !pte.IsDemandZero():
				t.Errorf("demand-zero page %08X has PTE %08X, want demand-zero", va, uint32(pte))
			case va >= c.CPU.GPR(8) && isd.Flags&mapISDWRT == 0 && !isSectionPTE(pte):
				t.Errorf("code page %08X never run has PTE %08X, want its section form", va, uint32(pte))
			}
		}
	}

	if !sawDZRO || !sawRO || !sawFixup {
		t.Fatalf("the image's ISDs: demand-zero %v, read-only %v, fixup %v; want each", sawDZRO, sawRO, sawFixup)
	}

	if got := env.ProcessSections(); got != sections {
		t.Errorf("%d process sections, want the image's %d", got, sections)
	}

	if err := c.Mounts.Dismount("DUA0"); err == nil {
		t.Fatal("the image's volume was dismounted while the image ran")
	}

	toCheckpoint2(t, c)
	checkWorkingSets(t, c)

	finishMappedImage(t, c)

	if got := env.ProcessSections(); got != 0 {
		t.Errorf("after the image: %d process sections, want none", got)
	}

	// The image's exit refunds what it was charged, which is all its
	// writable and demand-zero pages were charged at activation: none of
	// them was charged later.
	if got := env.PageFileRemaining() - running; got != charged {
		t.Errorf("activation charged %d pages, want the writable and demand-zero sections' %d", got, charged)
	}

	if err := c.Mounts.Dismount("DUA0"); err != nil {
		t.Errorf("Dismount after the image: %v", err)
	}
}

// TestMappedImageOutput: the mapped image's fixups worked: its G^ call
// of LIB$PUT_OUTPUT says hello.
func TestMappedImageOutput(t *testing.T) {
	c, d, name, out := mappedImageConsole(t, "1")

	startMappedImage(t, c, d, name)
	finishMappedImage(t, c)

	if !strings.Contains(out.String(), "mapped hello") {
		t.Errorf("the image wrote %q, want its hello", out.String())
	}
}

// TestMappedImageDeposit: the debugger's DEPOSIT into a code page never
// run (read-only, mapped to the image file) sets the page's modify bit;
// when the page leaves the working set its frame takes the page file as
// its backing store, not the file, so the change is kept (question 3).
func TestMappedImageDeposit(t *testing.T) {
	c, d, name, _ := mappedImageConsole(t, "1")
	startMappedImage(t, c, d, name)

	va := c.CPU.GPR(8) + 512
	if !isSectionPTE(pteIn(t, c, va)) {
		t.Fatalf("code page %08X isn't in its section form before the deposit", va)
	}

	if err := c.WriteBytes(va, []byte{0xA5}); err != nil {
		t.Fatalf("WriteBytes: %v", err)
	}

	if pte := pteIn(t, c, va); !pte.Valid() || !pte.Modified() || pte.Protection() != vm.ProtUR {
		t.Fatalf("after the deposit the page's PTE is %08X, want valid, modified, and UR", uint32(pte))
	}

	// The array's 40 pages push the page out of the 30-page working set.
	toCheckpoint2(t, c)

	pte := pteIn(t, c, va)

	pfn, ok := pte.Transition()
	if !ok {
		t.Fatalf("the page's PTE is %08X, want it in transition", uint32(pte))
	}

	e, err := c.Mem.Paging().Entry(paging.PFN(pfn))
	if err != nil {
		t.Fatal(err)
	}

	if e.State != paging.Modified || e.Backing.Kind != paging.PageFile {
		t.Errorf("the page's frame is %+v, want on the modified list, backed by the page file", e)
	}

	buf := make([]byte, 1)
	if err := c.Mem.LoadIn(c.CPU, c.ProcessOne().Space.AddressSpace, va, buf); err != nil || buf[0] != 0xA5 {
		t.Errorf("the page reads back %02X (%v), want the deposit's A5", buf[0], err)
	}

	finishMappedImage(t, c)
}

// TestMappedImageCluster: an image section's fault reads a cluster
// (PFCDEFAULT's, the ISD's cluster factor being 0): with the default,
// the first code page's fault brings in code pages never run.
func TestMappedImageCluster(t *testing.T) {
	c, d, name, _ := mappedImageConsole(t, "")
	startMappedImage(t, c, d, name)

	if pte := pteIn(t, c, c.CPU.GPR(8)); !pte.Valid() {
		t.Errorf("the code page after the first, never run, has PTE %08X, want it read in with the first", uint32(pte))
	}

	finishMappedImage(t, c)
}
