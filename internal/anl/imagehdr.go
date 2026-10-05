package anl

import (
	"fmt"

	"github.com/tucats/govax/internal/vmsdef"
	"github.com/tucats/govax/internal/vmsimage"
)

// This file is ANALYZE/IMAGE's report of an image's header
// (docs/PHASE-40.md): the fixed header and its blocks, then each image
// section descriptor. The fixup section's report is in imagefix.go.

// ImageOptions are the ANALYZE/IMAGE choices that change a report.
type ImageOptions struct {
	// Header limits the report to the image header; Fixups to the
	// header and the fixup section. With neither, the report has both
	// (unconfirmed: what real ANALYZE leaves out for each qualifier isn't
	// shown by any fixture).
	Header bool
	Fixups bool
}

// ImageReport is the result of analyzing an image file.
type ImageReport struct {
	Lines  []Line
	Errors int
}

// AnalyzeImage describes an image, as ANALYZE/IMAGE does.
func AnalyzeImage(img *vmsimage.Image, opts ImageOptions) ImageReport {
	a := &imageAnalyzer{img: img, opts: opts}
	a.run()

	return ImageReport{Lines: a.lines, Errors: a.errors}
}

// imageAnalyzer is one ANALYZE/IMAGE in progress.
type imageAnalyzer struct {
	report

	img    *vmsimage.Image
	opts   ImageOptions
	errors int
}

// fail reports an error where it's found.
func (a *imageAnalyzer) fail(format string, args ...any) {
	a.errors++
	a.line("***  " + fmt.Sprintf(format, args...))
}

func (a *imageAnalyzer) run() {
	a.line("This is an OpenVMS VAX image file")
	a.blank()

	a.header()

	if !a.opts.Header || a.opts.Fixups {
		a.fixupSection()
	}

	a.blank()
	a.blank()

	if a.errors == 0 {
		a.line("The analysis uncovered NO errors.")
	} else {
		a.line(fmt.Sprintf("The analysis uncovered %d error%s.", a.errors, plural(a.errors)))
	}

	a.blank()
	a.blank()
}

// Image types (IHD$B_IMGTYPE), as ANALYZE names them. Only the
// executable's description is confirmed by a fixture.
var imageTypes = map[byte]string{
	1: "executable (IHD$K_EXE)",
	2: "shareable (IHD$K_LIM)",
}

// The named bits of the header's link flags, each ISD's flags, and the
// fixup section's flags, as ANALYZE lists them.
var (
	linkFlagBits = []flagBit{
		{0, "IHD$V_LNKDEBUG"}, {1, "IHD$V_LNKNOTFR"}, {2, "IHD$V_NOP0BUFS"},
		{3, "IHD$V_PICIMG"}, {4, "IHD$V_P0IMAGE"}, {5, "IHD$V_DBGDMT"},
		{6, "IHD$V_INISHR"}, {7, "IHD$V_IHSLONG"}, {8, "IHD$V_UPCALLS"},
	}

	isdFlagBits = []flagBit{
		{0, "ISD$V_GBL"}, {1, "ISD$V_CRF"}, {2, "ISD$V_DZRO"}, {3, "ISD$V_WRT"},
		{7, "ISD$V_LASTCLU"}, {8, "ISD$V_INITALCODE"}, {9, "ISD$V_BASED"},
		{10, "ISD$V_FIXUPVEC"}, {11, "ISD$V_RESIDENT"}, {17, "ISD$V_VECTOR"},
		{18, "ISD$V_PROTECT"},
	}

	iafFlagBits = []flagBit{{0, "IAF$V_SHR"}}
)

// Section types (an ISD's top flags byte) and global sections' match
// controls (ISD$V_MATCHCTL). NORMAL, SHRPIC, USRSTACK, and MATLEQ are
// seen in the fixtures.
var (
	sectionTypes = map[byte]string{
		0:   "ISD$K_NORMAL",
		1:   "ISD$K_SHRFXD",
		2:   "ISD$K_PRVFXD",
		3:   "ISD$K_SHRPIC",
		4:   "ISD$K_PRVPIC",
		253: "ISD$K_USRSTACK",
	}

	matchControls = map[int]string{
		0: "ISD$K_MATALL",
		1: "ISD$K_MATEQU",
		2: "ISD$K_MATLEQ",
		3: "ISD$K_MATNEV",
	}
)

// How many lines each kind of heading needs left on a page to be written
// there (Line.Keep), reconstructed from the fixtures' page breaks
// (docs/PHASE-40.md, "Page layout").
const (
	keepImageItem  = 3
	keepImageFlags = 3
	// keepImageISD is an image section descriptor's heading: FORTH.ANI
	// (testdata/mar/dst/vax) pushes one off a page with 3 rows left, and
	// the fixtures write one with 6 left, so 4 to 6 fit; 4 keeps the
	// heading with the three lines before its flags (unconfirmed).
	keepImageISD = 4
)

// flagList adds a flags label, then a line for each named bit of flags
// with its value, indented by tabs.
func (a *imageAnalyzer) flagList(tabs, label string, bits []flagBit, flags uint32) {
	a.keep(keepImageFlags, tabs+label)

	for _, b := range bits {
		a.line(fmt.Sprintf("%s\t%-5s%-17s%d", tabs, fmt.Sprintf("(%d)", b.bit), b.name, flags>>b.bit&1))
	}
}

// part starts one of a section's parts: its title, then a blank line.
func (a *imageAnalyzer) part(title string) {
	a.keep(keepImageItem, "\t"+title)
	a.blank()
}

// orDefault is n, or "default" when it's 0.
func orDefault(n int) string {
	if n == 0 {
		return "default"
	}

	return fmt.Sprint(n)
}

func (a *imageAnalyzer) header() {
	img := a.img

	a.line("IMAGE HEADER")
	a.blank()

	a.part("Fixed Header Information")
	a.line(fmt.Sprintf("\t\timage format major id: %s, minor id: %s", img.MajorID, img.MinorID))
	a.line(fmt.Sprintf("\t\theader block count: %d", img.Blocks))

	typ, ok := imageTypes[img.Type]
	if !ok {
		typ = fmt.Sprintf("unknown (%d)", img.Type)
	}

	a.line("\t\timage type: " + typ)

	if img.MajorID != "02" || img.MinorID != "05" {
		a.fail("Image format %s.%s is not the VAX image format, 02.05.", img.MajorID, img.MinorID)
	}

	if !ok {
		a.fail("Image type %d is undefined.", img.Type)
	}

	a.line("\t\tI/O channel count: " + orDefault(int(img.IOChannels)))
	a.line("\t\tI/O page count: " + orDefault(int(img.IOPages)))
	a.flagList("\t\t", "linker flags:", linkFlagBits, img.LinkFlags)
	a.blank()

	a.part("Image Activation Information")
	a.line(fmt.Sprintf("\t\tfirst transfer address:  %%X'%08X'", img.Transfers[0]))
	a.line(fmt.Sprintf("\t\tsecond transfer address: %%X'%08X'", img.Transfers[1]))
	a.line(fmt.Sprintf("\t\tthird transfer address:  %%X'%08X'", img.Transfers[2]))

	if img.LinkFlags&vmsimage.IHDFlagINISHR != 0 {
		a.line(fmt.Sprintf("\t\tshareable image initialization list: %%X'%08X'", img.InitShare))
	}
	a.blank()

	a.part("Global Symbol Table & Debug Symbol Table Information")
	a.line(fmt.Sprintf("\t\tdebug symbol table VBN:  %d, block count: %d", img.DSTVBN, img.DSTBlocks))
	a.line(fmt.Sprintf("\t\tglobal symbol table VBN: %d, record count: %d", img.GSTVBN, img.GSTRecords))
	a.line(fmt.Sprintf("\t\tdebug module/psect table VBN: %d, byte count: %d", img.DMTVBN, img.DMTBytes))
	a.blank()

	a.part("Image Identification Information")
	a.line("\t\timage name: " + quote(img.Name))
	a.line("\t\timage file identification: " + quote(img.FileID))
	a.line("\t\tlink date/time: " + vmsTime(vmsdef.GoTime(img.LinkTime)))
	a.line("\t\tlinker identification: " + quote(img.LinkerID))
	a.blank()

	a.part("Patch Information")
	a.patchInfo()
	a.problems(vmsimage.PartHeader)
	a.blank()

	a.part("Image Section Descriptors (ISD)")

	for i, d := range img.ISDs {
		if i > 0 {
			a.blank()
		}

		a.section(i+1, d)
	}

	a.problems(vmsimage.PartSections)
}

// problems reports what couldn't be decoded in part of the image.
func (a *imageAnalyzer) problems(part vmsimage.Part) {
	for _, p := range a.img.Problems {
		if p.Part == part {
			a.fail("%s", p.Text)
		}
	}
}

// patchInfo describes the patch block. No fixture image has been patched,
// and nothing but a patched image's analysis would show the block's
// fields, so a patched image's block is shown as a hex dump, in
// ANALYZE/OBJECT's style (unconfirmed).
func (a *imageAnalyzer) patchInfo() {
	if a.img.Patch == nil {
		a.line("\t\tThere are no patches at this time.")

		return
	}

	a.line(fmt.Sprintf("\t\tpatch block, %d bytes:", len(a.img.Patch)))
	a.hexDump("\t\t", a.img.Patch)
}

// spaceName is the region of the address space an address is in.
func spaceName(addr uint32) string {
	switch addr >> 30 {
	case 0:
		return "P0 space"
	case 1:
		return "P1 space"
	}

	return "system space"
}

// section describes one image section descriptor.
func (a *imageAnalyzer) section(n int, d vmsimage.ISD) {
	a.keep(keepImageISD, fmt.Sprintf("\t\t%d)  image section descriptor (%d bytes)", n, d.Size))
	a.line(fmt.Sprintf("\t\t\tpage count: %d", d.Pages))
	a.line(fmt.Sprintf("\t\t\tbase virtual address: %%X'%08X' (%s)", d.Address(), spaceName(d.Address())))
	a.line("\t\t\tpage fault cluster size: " + orDefault(int(d.PFC)))
	a.flagList("\t\t\t", "ISD flags:", isdFlagBits, d.Flags)

	typ, ok := sectionTypes[d.Type()]
	if !ok {
		typ = fmt.Sprintf("unknown (%d)", d.Type())
	}

	a.line("\t\t\tsection type: " + typ)

	if !ok {
		a.fail("Section type %d is undefined.", d.Type())
	}

	if d.Size >= vmsimage.ISDPrivateLength {
		a.line(fmt.Sprintf("\t\t\tbase VBN: %d", d.VBN))
		a.checkBlocks(d)
	}

	if d.Flags&vmsimage.ISDFlagGBL != 0 && d.Size > vmsimage.ISDGlobalLength {
		match, ok := matchControls[d.Match()]
		if !ok {
			match = fmt.Sprintf("unknown (%d)", d.Match())
		}

		a.line(fmt.Sprintf("\t\t\tglobal section major id: %%X'%02X', minor id: %%X'%06X'", d.GlobalIdent>>24, d.GlobalIdent&0xFFFFFF))
		a.line("\t\t\tmatch control: " + match)
		a.line("\t\t\tglobal section name: " + quote(d.GlobalName))

		if !ok {
			a.fail("Match control %d is undefined.", d.Match())
		}
	}
}

// checkBlocks checks that a private section's pages are in the file: a
// section that isn't demand zero or global is read from VBN on.
func (a *imageAnalyzer) checkBlocks(d vmsimage.ISD) {
	if d.Flags&(vmsimage.ISDFlagDZRO|vmsimage.ISDFlagGBL) != 0 || d.Pages == 0 {
		return
	}

	last := int(d.VBN) + int(d.Pages) - 1

	if d.VBN <= uint32(a.img.Blocks) || last > a.img.FileBlocks {
		a.fail("The section's blocks, %d to %d, are not in the file's %d blocks of image sections.", d.VBN, last, a.img.FileBlocks)
	}
}
