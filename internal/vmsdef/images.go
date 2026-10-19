package vmsdef

// The symbols LINK finds in VMS's own files, as govax keeps them
// (docs/PHASE-31.md). LINK reads a shareable image's symbols from the
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
// docs/PHASE-68 - link shareable images.md, subtask 4) and maps each
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
}
