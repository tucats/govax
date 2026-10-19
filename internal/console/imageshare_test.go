package console_test

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
	"github.com/tucats/govax/internal/machine"
	"github.com/tucats/govax/internal/vm"
	"github.com/tucats/govax/internal/vmsimage"
)

// Phase 66's subtask 4 (docs/PHASE-66 - mapped image activation.md):
// with paging on, a shareable image the main image names is mapped as
// the main image is, privately, at P0's high-water mark after it.
//
// govax's LINK makes only executables, so the shareable image here is
// made by hand from one (shareableImage): a govax-linked image of one
// read-only data page, its sections moved down to start at VPN 0 and its
// header's type made IHD$K_LIM, saved as CRFSHR.EXE where the console's
// shareable image prefix finds it. The main image reaches it through a
// G^ reference to CRF$INSRTKEY, which govax's LINK resolves from its own
// table of CRFSHR's universal symbols (offset 0), so the fixup cell holds
// the stand-in image's base. A real shareable image, linked /SHAREABLE on
// VMS, would check what this one can't (an image with no user stack ISD,
// its own fixup section, a transfer vector); that needs a probe image or
// a LINK that makes them.

// shareMainSource puts the address CRF$INSRTKEY's fixup cell holds in R7
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
	moval	g^crf$insrtkey,r7
	stop	1
	movl	(r7),r9
	stop	2
	movl	#1,r0
	ret
	.end	start
`

// sharePattern fills the stand-in shareable image's one page.
const sharePattern = 0x5A5A1234

const shareDataSource = `	.title	crfshr
	.psect	data,noexe,nowrt,page
	.rept	128
	.long	^x5A5A1234
	.endr
	.end
`

// shareableImage turns the executable exe into a shareable image: each of
// its sections (the user stack's and global ones aside) one page lower,
// so the first is at VPN 0, its transfer addresses cleared (an
// executable's first is SYS$IMGSTA's, which the loader would take for
// the shareable image's initialization routine), and its image type
// IHD$K_LIM.
func shareableImage(t *testing.T, exe []byte) []byte {
	t.Helper()

	data := append([]byte(nil), exe...)
	h := data[:vmsimage.BlockSize]
	le := binary.LittleEndian

	h[vmsimage.IHDImageType] = 2

	activ := int(le.Uint16(h[vmsimage.IHDActivOffset:]))
	clear(h[activ : activ+3*4])

	for off := int(le.Uint16(h[vmsimage.IHDISDOffset:])); ; {
		size := int(le.Uint16(h[off+vmsimage.ISDSizeOffset:]))
		if size == 0 {
			break
		}

		if size == vmsimage.ISDContinue {
			t.Fatal("the image's ISDs go on in a second header block")
		}

		flags := le.Uint32(h[off+vmsimage.ISDFlagsOffset:])
		if flags&vmsimage.ISDFlagGBL == 0 && flags>>24 != mapISDUsrStack {
			vpn := le.Uint32(h[off+vmsimage.ISDVPNOffset:])
			if vpn&vmsimage.ISDVPNMask == 0 {
				t.Fatal("a section is already at VPN 0")
			}

			le.PutUint32(h[off+vmsimage.ISDVPNOffset:], vpn-1)
		}

		off += size
	}

	return data
}

// sharedImageConsole boots a console, paging on or off, with the
// stand-in CRFSHR where its shareable image prefix finds it, and returns
// it with the main image's host file.
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

	exe, err := os.ReadFile(buildImage(t, c, "crfshr", shareDataSource))
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "CRFSHR.exe"), shareableImage(t, exe), 0o644); err != nil {
		t.Fatal(err)
	}

	c.SharePrefix = dir + string(filepath.Separator)

	return c, d, buildImage(t, c, "ishr", shareMainSource)
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
		if icb.Name == "CRFSHR" {
			return icb
		}
	}

	t.Fatal("CRFSHR wasn't loaded")

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
		t.Errorf("the image read %08X from CRFSHR, want %08X", got, sharePattern)
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
		t.Errorf("CRFSHR's base %08X isn't past the main image's end %08X", shr.Base, mainICB.End)
	}

	if got := c.CPU.GPR(7); got != shr.Base {
		t.Errorf("CRF$INSRTKEY's cell holds %08X, want CRFSHR's base %08X", got, shr.Base)
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
		t.Errorf("CRFSHR's page has PTE %08X, want its section form, UR", uint32(pte))
	}

	toSharedCheckpoint2(t, c)

	if pte := pteIn(t, c, shr.Base); !pte.Valid() {
		t.Errorf("CRFSHR's page has PTE %08X after it was read, want it valid", uint32(pte))
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
		t.Errorf("CRF$INSRTKEY's cell holds %08X, want CRFSHR's base %08X", got, shr.Base)
	}

	toSharedCheckpoint2(t, c)
	finishMappedImage(t, c)
}
