package console_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
	"github.com/tucats/govax/internal/machine"
	"github.com/tucats/govax/internal/vm"
)

// Phase 66's subtask 4 (docs/PHASE-66 - mapped image activation.md):
// with paging on, a shareable image the main image names is mapped as
// the main image is, privately, at P0's high-water mark after it.
//
// The shareable image, SHRDATA, is one page of read-only data whose
// first longword is its universal symbol SHR_DATA, made by govax's
// LINK/SHAREABLE (docs/PHASE-68 - link shareable images.md; until then
// this test made one by hand from an executable). The main image reaches
// it through a G^ reference to SHR_DATA, so the fixup cell holds the
// image's base. SHRDATA is a host file, found through the console's
// shareable image prefix.

// shareMainSource puts the address SHR_DATA's fixup cell holds in R7
// and waits at checkpoint 1 (R6, released by the test's R10); then reads
// the longword there into R9 and waits at checkpoint 2.
const shareMainSource = `	.title	ishr
	.macro	stop,n,?l
	movl	#n,r6
l:	cmpl	r10,r6
	bneq	l
	.endm	stop
	.psect	code,exe,nowrt,page
	.entry	start,^m<r6,r7,r9,r10>
	clrl	r10
	moval	g^shr_data,r7
	stop	1
	movl	(r7),r9
	stop	2
	movl	#1,r0
	ret
	.end	start
`

// sharePattern fills the shareable image's one page.
const sharePattern = 0x5A5A1234

const shareDataSource = `	.title	shrdata
	.psect	data,noexe,nowrt,page
shr_data::
	.rept	128
	.long	^x5A5A1234
	.endr
	.end
`

// linkHostImage assembles source as the host file dir/name.mar and links
// it into dir/name.exe, with the options file options, as a shareable
// image (LINK/SHAREABLE) or an executable, and returns the image's file.
func linkHostImage(t *testing.T, d *console.Dispatcher, dir, name, source, options string, shareable bool) string {
	t.Helper()

	path := func(ext string) string { return filepath.Join(dir, name+ext) }

	for file, text := range map[string]string{".mar": source, ".opt": options} {
		if err := os.WriteFile(path(file), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	for _, cmd := range []string{
		`MACRO "` + path(".mar") + `"/OBJECT="` + path(".obj") + `"`,
		`LINK/` + map[bool]string{false: "EXECUTABLE", true: "SHAREABLE"}[shareable] + `="` + path(".exe") + `" "` +
			path(".obj") + `","` + path(".opt") + `"/OPTIONS`,
	} {
		if err := d.Dispatch(cmd); err != nil {
			t.Fatalf("%s: %v", cmd, err)
		}
	}

	return path(".exe")
}

// sharedImageConsole boots a console, paging on or off, with SHRDATA
// where its shareable image prefix finds it, and returns it with the
// main image's host file, linked against SHRDATA.
func sharedImageConsole(t *testing.T, paging bool) (*console.Console, *console.Dispatcher, string) {
	t.Helper()

	if paging {
		setSetting(t, machine.PagingSetting, "true")
		setSetting(t, machine.PFCDefaultSetting, "1")
	}

	c, _ := scheduledConsole(t, longQuantum, brbSelf)
	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t), nil)

	if (c.Mem.Paging() != nil) != paging {
		t.Fatalf("paging is %v, want %v", c.Mem.Paging() != nil, paging)
	}

	dir := t.TempDir()
	shr := linkHostImage(t, d, dir, "shrdata", shareDataSource, "UNIVERSAL=SHR_DATA\n", true)
	main := linkHostImage(t, d, dir, "ishr", shareMainSource, `"`+shr+`"/SHAREABLE`+"\n", false)

	c.SharePrefix = dir + string(filepath.Separator)

	return c, d, main
}

// runSharedImage runs the main image main to checkpoint 1, and returns
// its shareable image's ICB.
func runSharedImage(t *testing.T, c *console.Console, d *console.Dispatcher, main string) *machine.ICB {
	t.Helper()

	c.Engine.SetLimits(1, 0)

	if err := d.Dispatch(`RUN "` + main + `"`); err != nil {
		t.Fatalf("RUN: %v", err)
	}

	c.Engine.SetLimits(0, 0)

	if !c.ImageActive() {
		t.Fatal("the image isn't running")
	}

	runUntil(t, c, 200_000, func() bool { return c.CPU.GPR(6) == 1 })
	step(t, c, 4)

	for _, icb := range c.ImageState(c.ProcessOne()).ICBList {
		if icb.Name == "SHRDATA" {
			return icb
		}
	}

	t.Fatal("SHRDATA wasn't loaded")

	return nil
}

// toSharedCheckpoint2 releases the image from checkpoint 1 and runs it to
// checkpoint 2, where R9 is what it read from the shareable image.
func toSharedCheckpoint2(t *testing.T, c *console.Console) {
	t.Helper()

	c.CPU.SetGPR(10, 1)
	runUntil(t, c, 200_000, func() bool { return c.CPU.GPR(6) == 2 })
	step(t, c, 4)

	if got := c.CPU.GPR(9); got != sharePattern {
		t.Errorf("the image read %08X from SHRDATA, want %08X", got, sharePattern)
	}
}

// TestMappedSharedImage: with paging on, the shareable image is a
// process section of its file after the main image, its page read-only
// and still in its section form until the main image reads it; then it
// reads the image's contents; and at the image's end its section is gone
// with the main image's.
func TestMappedSharedImage(t *testing.T) {
	c, d, main := sharedImageConsole(t, true)
	env := c.ProcessOne()

	shr := runSharedImage(t, c, d, main)

	mainICB := c.FindMainICB()
	if shr.Base <= mainICB.End {
		t.Errorf("SHRDATA's base %08X isn't past the main image's end %08X", shr.Base, mainICB.End)
	}

	if got := c.CPU.GPR(7); got != shr.Base {
		t.Errorf("SHR_DATA's cell holds %08X, want SHRDATA's base %08X", got, shr.Base)
	}

	sections := 0

	for _, icb := range []*machine.ICB{mainICB, shr} {
		for _, isd := range icb.ISDList {
			if isd.Flags>>24 != mapISDUsrStack && isd.Flags&(mapISDGBL|mapISDDZRO) == 0 {
				sections++
			}
		}
	}

	if got := env.ProcessSections(); got != sections {
		t.Errorf("%d process sections, want the two images' %d", got, sections)
	}

	pte := pteIn(t, c, shr.Base)
	if !isSectionPTE(pte) || pte.Protection() != vm.ProtUR {
		t.Errorf("SHRDATA's page has PTE %08X, want its section form, UR", uint32(pte))
	}

	toSharedCheckpoint2(t, c)

	if pte := pteIn(t, c, shr.Base); !pte.Valid() {
		t.Errorf("SHRDATA's page has PTE %08X after it was read, want it valid", uint32(pte))
	}

	finishMappedImage(t, c)

	if got := env.ProcessSections(); got != 0 {
		t.Errorf("after the image: %d process sections, want none", got)
	}
}

// TestCopiedSharedImage: with paging off, the same shareable image is
// copied, and the main image reads the same contents.
func TestCopiedSharedImage(t *testing.T) {
	c, d, main := sharedImageConsole(t, false)

	shr := runSharedImage(t, c, d, main)

	if got := c.CPU.GPR(7); got != shr.Base {
		t.Errorf("SHR_DATA's cell holds %08X, want SHRDATA's base %08X", got, shr.Base)
	}

	toSharedCheckpoint2(t, c)
	finishMappedImage(t, c)
}
