package vmsdef

// The symbols LINK finds in VMS's own files, as govax keeps them
// (docs/planning/PHASE-31.md). LINK reads a shareable image's symbols from the
// image file (SYS$SHARE:LIBRTL.EXE, say), and adds STARLET.OLB's modules
// to a link, when it can find those files. When it can't, it uses these:
// the same facts, captured once from the files by internal/vmsdef/gen, so
// that a program links as it would with the files there.
//
// SharedImages and ImageSymbols (images_generated.go) are shareable
// images' global symbol tables, captured with gen's -image: a program
// links to a routine's real offset without the image being there, though
// running the routine still needs the image itself, or a shim for it.
//
// LibrarySymbols (library_generated.go) are the absolute symbols of an
// object library's definition modules, captured with gen's -olb: modules
// such as STARLET's SYS$SSDEF, which defines every SS$_ code and adds
// nothing to an image but symbols. A module with code or data can't be
// captured this way; a program that needs one still needs the library.

// SharedImage is what a shareable image's header says about it: what an
// image linked against it records in the global section ISD that maps it.
type SharedImage struct {
	// Pages is the size of its shareable section.
	Pages uint32

	// MajorID and MinorID (24 bits) are its global section ident, and
	// Match the match control (ISD$K_MATxxx) the image activator checks it
	// with.
	MajorID uint8
	MinorID uint32
	Match   uint32

	// Symbols and Psects are how many symbols and psects its global symbol
	// table defines, and Sections how many image sections of its own it
	// has (global ones and its stack aside): a map reports these counts.
	Symbols, Psects, Sections int
}

// ImageSymbol is one universal symbol a shareable image defines.
type ImageSymbol struct {
	// Image names the shareable image, and Value is the symbol's offset
	// there. With no Image, Value is the symbol's absolute value, as a
	// LIB$_ condition value's is.
	Image string
	Value uint32
	// Order is the symbol's place in its image's global symbol table, from
	// 1, which orders the fixup cells of an image linked against it
	// (internal/link's orderCells); 0 if unknown.
	Order int
}

// SharedImageSection is one image section of a shareable image: its
// offset from the image's start, its pages, and its ISD flags (copy on
// reference, demand zero, writable, and the section type, ISD$K_, in the
// top byte).
type SharedImageSection struct {
	Offset, Pages, Flags uint32
}

// SharedImageSections are the image sections of shareable images in
// SharedImages, for a link that places an image itself (LINK's BASE=,
// docs/planning/PHASE-68 - link shareable images.md, subtask 4) and maps each
// section through a global section ISD. They come from VMS's own output,
// not the image files: testdata/link68/vax/exe1.ani, where VMS 7.1's LINK
// placed LIBRTL as LIBRTL_001 (264 pages, ISD$K_SHRPIC) and LIBRTL_002
// (15 demand-zero, writable pages, ISD$K_PRVPIC). Kept by hand, as gen
// doesn't capture them; an image not listed can't be placed at link time
// without its file.
var SharedImageSections = map[string][]SharedImageSection{
	"LIBRTL": {
		{Offset: 0, Pages: 264, Flags: 3 << 24},
		{Offset: 0x21000, Pages: 15, Flags: 4<<24 | 0x0C},
	},
	// KnownImages' two, from VMS 7.1's map of SEC93.EXE
	// (testdata/prot93/vax/round3/sec93.map): READ WRITE is ISD$V_WRT,
	// COPY ON REF ISD$V_CRF, FIXUP VECTORS ISD$V_FIXUPVEC, and a
	// protected section (P) ISD$V_PROTECT.
	"SECURESHR": {
		{Offset: 0, Pages: 2, Flags: 3 << 24},
		{Offset: 0x400, Pages: 18, Flags: 3<<24 | isdWRT | isdCRF},
		{Offset: 0x2800, Pages: 82, Flags: 3 << 24},
		{Offset: 0xCC00, Pages: 1, Flags: 2<<24 | isdWRT | isdCRF},
		{Offset: 0xCE00, Pages: 14, Flags: 4<<24 | isdWRT | isdCRF},
		{Offset: 0xEA00, Pages: 6, Flags: 2<<24 | isdWRT | isdFixupVec},
	},
	"SECURESHRP": {
		{Offset: 0, Pages: 1, Flags: 3<<24 | isdProtect},
		{Offset: 0x200, Pages: 1, Flags: 3<<24 | isdProtect},
		{Offset: 0x400, Pages: 1, Flags: 2<<24 | isdProtect | isdWRT | isdCRF},
		{Offset: 0x600, Pages: 13, Flags: 3<<24 | isdProtect | isdWRT | isdCRF},
		{Offset: 0x2000, Pages: 80, Flags: 3<<24 | isdProtect},
		{Offset: 0xC000, Pages: 1, Flags: 2<<24 | isdProtect | isdWRT | isdCRF},
		{Offset: 0xC200, Pages: 13, Flags: 4<<24 | isdProtect | isdWRT | isdCRF},
		{Offset: 0xDC00, Pages: 4, Flags: 2<<24 | isdProtect | isdWRT | isdFixupVec},
	},
}

// The ISD flags SharedImageSections uses.
const (
	isdCRF      = 1 << 1
	isdWRT      = 1 << 3
	isdFixupVec = 1 << 10
	isdProtect  = 1 << 18
)

// KnownImages are shareable images whose headers gen hasn't captured
// (govax has no copy of their files) but whose facts VMS's own output
// gives, so that an image linked against them by govax's LINK names them
// as VMS's LINK would and runs on VMS. Without an entry here, an image
// the shims stand for is recorded by name only, with no pages, which VMS
// 7.1's image activator refuses (SS$_BADISD; testdata/vms93's VMS93CG).
// SECURESHR and SECURESHRP are from VMS 7.1's map of SEC93.EXE
// (testdata/prot93/vax/round3/sec93.map) and its image (VMS93C.EXE,
// testdata/vms93): LINK of a program that calls SECURESHR lists
// SECURESHRP after it, which SECURESHR uses. Their symbols are the
// shims' (internal/securertl).
var KnownImages = map[string]SharedImage{
	"SECURESHR":  {Pages: 2, MajorID: 1, MinorID: 2, Match: 2, Sections: 6},
	"SECURESHRP": {Pages: 1, MajorID: 1, MinorID: 1, Match: 2, Sections: 8},
}

// ImageUses names, for a shareable image in SharedImages or KnownImages,
// the shareable images it uses itself, in its list's order; an image
// linked against it lists them too. gen doesn't capture them.
var ImageUses = map[string][]string{
	"SECURESHR": {"SECURESHRP"},
}
