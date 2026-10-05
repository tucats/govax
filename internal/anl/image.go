package anl

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// This file decodes a VAX/VMS image file for ANALYZE/IMAGE
// (docs/PHASE-40.md). An image starts with one or more 512-byte header
// blocks: the fixed header ($IHD), which gives the offsets of the
// activation ($IHA), symbol table and debug ($IHS), identification
// ($IHI), and patch ($IHP) blocks, then the image section descriptors
// (ISDs) that say how the rest of the file maps into memory. One of the
// sections is the image activator's fixup section ($IAF), which lists
// what must be patched when the shareable images the program uses are
// mapped.
//
// Every offset here was read off real LINK's images (the fixtures beside
// VMS's ANALYZE/IMAGE output) and agrees with internal/link/image.go,
// which writes them. Nothing is taken from VMS's own sources (the project
// is clean room). The decoder neither runs nor maps the image; the
// console's image loader (internal/console/image.go) does that through
// emulated memory.

// imageBlock is a disk block's size, and a VAX page's.
const imageBlock = 512

// Fixed header ($IHD) offsets.
const (
	ihdISDOffset     = 0x00 // word: the first ISD
	ihdActivOffset   = 0x02 // word: the activation block
	ihdSymDbgOffset  = 0x04 // word: the symbol table and debug block
	ihdImgIDOffset   = 0x06 // word: the identification block
	ihdPatchOffset   = 0x08 // word: the patch block, 0 for none
	ihdMajorIDOffset = 0x0C // two ASCII characters: "02"
	ihdMinorIDOffset = 0x0E // two ASCII characters: "05"
	ihdBlockCount    = 0x10 // byte: how many header blocks
	ihdImageType     = 0x11 // byte: IHD$K_EXE, IHD$K_LIM, ...
	ihdIOChannels    = 0x1C // word: I/O channels, 0 for the default
	ihdIOPages       = 0x1E // word: I/O pages, 0 for the default
	ihdLinkFlags     = 0x20 // longword: IHD$V_ flags
	ihdIdent         = 0x24 // longword: the image's ident
	ihdSysVersion    = 0x28 // longword: the system version linked against
	ihdIAFVA         = 0x2C // longword: the fixup section's address
	ihdFixedLength   = 0x30
)

// Image section descriptor ($ISD) layout and flags.
const (
	isdSizeOffset  = 0x00 // word: the ISD's size; 0 ends the list, 0xFFFF goes on in the next block
	isdPagesOffset = 0x02 // word: the section's pages
	isdVPNOffset   = 0x04 // longword: the VPN (low 23 bits) and page fault cluster (top byte)
	isdFlagsOffset = 0x08 // longword: ISD$V_ flags, the section type in the top byte
	isdVBNOffset   = 0x0C // longword: the section's first block in the file
	isdIdentOffset = 0x10 // longword: a global section's ident
	isdNameOffset  = 0x14 // counted string: a global section's name

	isdDemandZeroLength = 0x0C // a demand-zero section's ISD has no VBN
	isdPrivateLength    = 0x10
	isdGlobalLength     = 0x14 // a global section's, before its name

	isdContinue = 0xFFFF

	isdFlagGBL      = 1 << 0  // a global section
	isdFlagDZRO     = 1 << 2  // demand zero
	isdFlagFIXUPVEC = 1 << 10 // the image activator's fixup section

	isdVPNMask   = 0x7FFFFF
	isdP1VPN     = 1 << 21 // a VPN in P1 space
	isdMatchMask = 0x70    // ISD$V_MATCHCTL, bits 4-6
	isdMatchBit  = 4
)

// Fixup section ($IAF) offsets.
const (
	iafGFixOffset    = 0x0C // longword: the G^ reference fixup lists
	iafDotAddrOffset = 0x10 // longword: the .ADDRESS reference fixup lists
	iafChgPrtOffset  = 0x14 // longword: the protection change list
	iafShlOffset     = 0x18 // longword: the shareable image list
	iafShrImgCount   = 0x1C // longword: entries in the shareable image list
	iafShlExtra      = 0x20 // longword: extra shareable image entries (unconfirmed)
	iafFlags         = 0x24 // longword: IAF$V_ flags (unconfirmed)
	iafFixedLength   = 0x40

	// shlEntryLength is a shareable image list entry's size, and
	// shlNameOffset where its counted name is.
	shlEntryLength = 0x40
	shlNameOffset  = 0x18
	shlNameLength  = 40
)

// Image is a decoded image file: its header and its fixup section.
type Image struct {
	// Header holds the header blocks, Blocks of them.
	Header []byte
	Blocks int

	// The fixed header.
	MajorID, MinorID string
	Type             byte
	IOChannels       uint16
	IOPages          uint16
	LinkFlags        uint32
	Ident            uint32
	SysVersion       uint32
	IAFVA            uint32

	// Offsets of the other header blocks, from the fixed header; 0 when
	// the image has no such block.
	ActivOffset, SymDbgOffset, ImgIDOffset, PatchOffset int

	// The activation block's transfer addresses.
	Transfers [3]uint32

	// The symbol table and debug block.
	DSTVBN, GSTVBN, DMTVBN uint32
	DSTBlocks, GSTRecords  uint16
	DMTBytes               uint32

	// The identification block.
	Name, FileID, LinkerID string
	LinkTime               uint64

	// The patch block, as it is in the header (nil without one).
	Patch []byte

	// ISDs are the image section descriptors, in order.
	ISDs []ISD

	// Fixups is the fixup section, nil when the image has none.
	Fixups *Fixups

	// FileBlocks is the file's size in blocks.
	FileBlocks int

	// Problems are what couldn't be decoded, in the order found. The
	// report shows each as an error, at the end of the part it's about.
	Problems []Problem

	// part is the part being decoded, which a problem is about.
	part Part
}

// Part is a part of an image a problem is about.
type Part int

// An image's parts, in the order the report shows them.
const (
	PartHeader   Part = iota // the fixed header and its blocks
	PartSections             // the image section descriptors
	PartFixups               // the fixup section
)

// Problem is something in an image that couldn't be decoded.
type Problem struct {
	Part Part
	Text string
}

// ISD is one image section descriptor.
type ISD struct {
	Size  int    // the descriptor's size in bytes
	Pages uint16 // the section's size in pages
	VPN   uint32 // its first virtual page
	PFC   byte   // its page fault cluster, 0 for the default
	Flags uint32 // ISD$V_ flags, the section type in the top byte
	VBN   uint32 // its first block in the file, 0 for demand zero

	// A global section's ident and name.
	GlobalIdent uint32
	GlobalName  string
}

// Address is the section's first virtual address.
func (d ISD) Address() uint32 { return d.VPN << 9 }

// Type is the section type, ISD$K_.
func (d ISD) Type() byte { return byte(d.Flags >> 24) }

// Match is a global section's match control, ISD$K_MAT....
func (d ISD) Match() int { return int(d.Flags&isdMatchMask) >> isdMatchBit }

// Fixups is an image's fixup section.
type Fixups struct {
	// VA is the section's address, and Base the address the protection
	// change and .ADDRESS lists are relative to: the image's first
	// section's.
	VA, Base uint32

	Flags      uint32
	ShareCount uint32
	Extra      uint32

	// Offsets of the lists from the section's start; 0 for none.
	GFixOffset, DotAddrOffset, ChgPrtOffset, ShlOffset uint32

	// Shared are the shareable image list's names, the image itself (an
	// empty name) first.
	Shared []string

	// GRefs and DotAddrRefs are the reference fixup lists, one per
	// shareable image that has references.
	GRefs, DotAddrRefs []RefList

	// Protections are the protection changes.
	Protections []Protection
}

// RefList is a shareable image's references: Image is its index in the
// shareable image list. A G^ list's Values are the cells' contents (each
// target's offset in the shareable image); a .ADDRESS list's are the
// addresses of the longwords to fix, relative to the image base.
type RefList struct {
	Image  uint32
	Values []uint32
}

// Protection is one protection change: Pages pages from Address
// (relative to the image base) get protection Code (PRT$C_).
type Protection struct {
	Address uint32
	Pages   uint16
	Code    uint16
}

// errNotImage is returned for a file too short to hold an image header.
var errNotImage = errors.New("the file is too short to be an image")

// ReadImage decodes an image file's contents. It fails only when data
// can't be an image at all; what it can't decode past that point is
// collected in Image.Problems.
func ReadImage(data []byte) (*Image, error) {
	if len(data) < imageBlock {
		return nil, errNotImage
	}

	img := &Image{FileBlocks: (len(data) + imageBlock - 1) / imageBlock}
	le := binary.LittleEndian

	img.Blocks = int(data[ihdBlockCount])
	if img.Blocks == 0 {
		img.Blocks = 1
	}

	if img.Blocks*imageBlock > len(data) {
		img.problem("The header block count, %d, is more than the file holds.", img.Blocks)
		img.Blocks = len(data) / imageBlock
	}

	h := data[:img.Blocks*imageBlock]
	img.Header = h

	img.MajorID = string(h[ihdMajorIDOffset : ihdMajorIDOffset+2])
	img.MinorID = string(h[ihdMinorIDOffset : ihdMinorIDOffset+2])
	img.Type = h[ihdImageType]
	img.IOChannels = le.Uint16(h[ihdIOChannels:])
	img.IOPages = le.Uint16(h[ihdIOPages:])
	img.LinkFlags = le.Uint32(h[ihdLinkFlags:])
	img.Ident = le.Uint32(h[ihdIdent:])
	img.SysVersion = le.Uint32(h[ihdSysVersion:])
	img.IAFVA = le.Uint32(h[ihdIAFVA:])

	img.ActivOffset = int(le.Uint16(h[ihdActivOffset:]))
	img.SymDbgOffset = int(le.Uint16(h[ihdSymDbgOffset:]))
	img.ImgIDOffset = int(le.Uint16(h[ihdImgIDOffset:]))
	img.PatchOffset = int(le.Uint16(h[ihdPatchOffset:]))

	img.activation(h)
	img.symbolTables(h)
	img.identification(h)
	img.patch(h)

	img.part = PartSections
	img.sections(h, int(le.Uint16(h[ihdISDOffset:])))

	img.part = PartFixups
	img.fixups(data)

	return img, nil
}

// problem records something that couldn't be decoded.
func (img *Image) problem(format string, args ...any) {
	img.Problems = append(img.Problems, Problem{Part: img.part, Text: fmt.Sprintf(format, args...)})
}

// block returns the n bytes of the header block at offset, or nil (with
// a problem recorded) when they aren't all in the header.
func (img *Image) block(h []byte, what string, offset, n int) []byte {
	if offset < ihdFixedLength || offset+n > imageBlock {
		img.problem("The %s block's offset, %d, is outside the header.", what, offset)

		return nil
	}

	return h[offset : offset+n]
}

// Header block sizes.
const (
	ihaLength = 0x14
	ihsLength = 0x1C
	ihiLength = 0x50
)

func (img *Image) activation(h []byte) {
	if img.ActivOffset == 0 {
		return
	}

	b := img.block(h, "activation", img.ActivOffset, ihaLength)
	if b == nil {
		return
	}

	for i := range img.Transfers {
		img.Transfers[i] = binary.LittleEndian.Uint32(b[4*i:])
	}
}

func (img *Image) symbolTables(h []byte) {
	if img.SymDbgOffset == 0 {
		return
	}

	b := img.block(h, "symbol table", img.SymDbgOffset, ihsLength)
	if b == nil {
		return
	}

	le := binary.LittleEndian
	img.DSTVBN = le.Uint32(b[0:])
	img.GSTVBN = le.Uint32(b[4:])
	img.DSTBlocks = le.Uint16(b[8:])
	img.GSTRecords = le.Uint16(b[10:])
	img.DMTVBN = le.Uint32(b[12:])
	img.DMTBytes = le.Uint32(b[16:])
}

func (img *Image) identification(h []byte) {
	if img.ImgIDOffset == 0 {
		return
	}

	b := img.block(h, "identification", img.ImgIDOffset, ihiLength)
	if b == nil {
		return
	}

	img.Name = counted(b[0:40])
	img.FileID = counted(b[40:56])
	img.LinkTime = binary.LittleEndian.Uint64(b[56:])
	img.LinkerID = counted(b[64:80])
}

// ihpLength is the patch block's size (unconfirmed: no fixture image
// has one).
const ihpLength = 0x20

func (img *Image) patch(h []byte) {
	if img.PatchOffset == 0 {
		return
	}

	img.Patch = img.block(h, "patch", img.PatchOffset, ihpLength)
}

// sections reads the ISDs, starting at offset in the header blocks.
func (img *Image) sections(h []byte, offset int) {
	le := binary.LittleEndian
	p := offset

	if p < ihdFixedLength {
		img.problem("The image section descriptors' offset, %d, is inside the fixed header.", p)

		return
	}

	for {
		if p+2 > len(h) {
			img.problem("The image section descriptors don't end within the header.")

			return
		}

		size := int(le.Uint16(h[p:]))

		switch {
		case size == 0:
			return

		case size == isdContinue:
			// The list goes on at the start of the next header block.
			p = (p/imageBlock + 1) * imageBlock

			continue

		case size < isdDemandZeroLength || p+size > len(h):
			img.problem("Image section descriptor %d's size, %d, is invalid.", len(img.ISDs)+1, size)

			return
		}

		img.ISDs = append(img.ISDs, decodeISD(h[p:p+size]))
		p += size
	}
}

// decodeISD decodes one ISD, b being exactly its bytes.
func decodeISD(b []byte) ISD {
	le := binary.LittleEndian
	vpn := le.Uint32(b[isdVPNOffset:])

	d := ISD{
		Size:  len(b),
		Pages: le.Uint16(b[isdPagesOffset:]),
		VPN:   vpn & isdVPNMask,
		PFC:   byte(vpn >> 24),
		Flags: le.Uint32(b[isdFlagsOffset:]),
	}

	if len(b) >= isdPrivateLength {
		d.VBN = le.Uint32(b[isdVBNOffset:])
	}

	if d.Flags&isdFlagGBL != 0 && len(b) > isdGlobalLength {
		d.GlobalIdent = le.Uint32(b[isdIdentOffset:])
		d.GlobalName = counted(b[isdNameOffset:])
	}

	return d
}

// counted returns the counted string at the start of b, cut short if b
// doesn't hold it all.
func counted(b []byte) string {
	if len(b) == 0 {
		return ""
	}

	n := min(int(b[0]), len(b)-1)

	return string(b[1 : 1+n])
}

// fixups finds and decodes the fixup section: the section whose ISD has
// ISD$V_FIXUPVEC, at the header's IAF address.
func (img *Image) fixups(data []byte) {
	if img.IAFVA == 0 {
		return
	}

	var sec *ISD

	for i := range img.ISDs {
		d := &img.ISDs[i]
		if img.IAFVA >= d.Address() && img.IAFVA < d.Address()+uint32(d.Pages)*imageBlock && d.Flags&isdFlagGBL == 0 {
			sec = d

			break
		}
	}

	if sec == nil {
		img.problem("No image section holds the fixup section at %%X'%08X'.", img.IAFVA)

		return
	}

	start := (int(sec.VBN)-1)*imageBlock + int(img.IAFVA-sec.Address())
	end := (int(sec.VBN) - 1 + int(sec.Pages)) * imageBlock

	if sec.VBN == 0 || start+iafFixedLength > len(data) {
		img.problem("The fixup section at %%X'%08X' is outside the file.", img.IAFVA)

		return
	}

	b := data[start:min(end, len(data))]
	le := binary.LittleEndian

	f := &Fixups{
		VA:            img.IAFVA,
		Base:          img.base(),
		GFixOffset:    le.Uint32(b[iafGFixOffset:]),
		DotAddrOffset: le.Uint32(b[iafDotAddrOffset:]),
		ChgPrtOffset:  le.Uint32(b[iafChgPrtOffset:]),
		ShlOffset:     le.Uint32(b[iafShlOffset:]),
		ShareCount:    le.Uint32(b[iafShrImgCount:]),
		Extra:         le.Uint32(b[iafShlExtra:]),
		Flags:         le.Uint32(b[iafFlags:]),
	}
	img.Fixups = f

	f.Shared = img.sharedImages(b, f)
	f.GRefs = img.refLists(b, f.GFixOffset, "G^")
	f.DotAddrRefs = img.refLists(b, f.DotAddrOffset, ".ADDRESS")
	f.Protections = img.protections(b, f.ChgPrtOffset)
}

// base is the address an image's relative fixup addresses count from:
// its first section's.
func (img *Image) base() uint32 {
	if len(img.ISDs) == 0 {
		return 0
	}

	return img.ISDs[0].Address()
}

// longAt returns the longword at offset in b, recording a problem (and
// returning ok false) when it's past b's end.
func (img *Image) longAt(b []byte, offset uint32, what string) (uint32, bool) {
	if uint64(offset)+4 > uint64(len(b)) {
		img.problem("The %s list runs past the end of the fixup section.", what)

		return 0, false
	}

	return binary.LittleEndian.Uint32(b[offset:]), true
}

func (img *Image) sharedImages(b []byte, f *Fixups) []string {
	if f.ShlOffset == 0 {
		return nil
	}

	names := make([]string, 0, min(f.ShareCount, uint32(len(b)/shlEntryLength)))

	for i := range f.ShareCount {
		p := uint64(f.ShlOffset) + uint64(i)*shlEntryLength
		if p+shlEntryLength > uint64(len(b)) {
			img.problem("The shareable image list runs past the end of the fixup section.")

			break
		}

		names = append(names, counted(b[p+shlNameOffset:p+shlNameOffset+shlNameLength]))
	}

	return names
}

// refLists reads the reference fixup lists at offset: for each
// shareable image, a count, its index, and count longwords, ended by a
// zero count.
func (img *Image) refLists(b []byte, offset uint32, what string) []RefList {
	if offset == 0 {
		return nil
	}

	var lists []RefList

	p := offset

	for {
		count, ok := img.longAt(b, p, what)
		if !ok || count == 0 {
			return lists
		}

		index, ok := img.longAt(b, p+4, what)
		if !ok {
			return lists
		}

		l := RefList{Image: index}
		p += 8

		for range count {
			v, ok := img.longAt(b, p, what)
			if !ok {
				return append(lists, l)
			}

			l.Values = append(l.Values, v)
			p += 4
		}

		lists = append(lists, l)
	}
}

// protections reads the protection change list at offset: a count, then
// that many entries of an address, a page count, and a protection.
func (img *Image) protections(b []byte, offset uint32) []Protection {
	if offset == 0 {
		return nil
	}

	count, ok := img.longAt(b, offset, "protection change")
	if !ok {
		return nil
	}

	out := make([]Protection, 0, min(count, uint32(len(b)/8)))
	le := binary.LittleEndian

	for i := range count {
		p := uint64(offset) + 4 + uint64(i)*8
		if p+8 > uint64(len(b)) {
			img.problem("The protection change list runs past the end of the fixup section.")

			break
		}

		out = append(out, Protection{Address: le.Uint32(b[p:]), Pages: le.Uint16(b[p+4:]), Code: le.Uint16(b[p+6:])})
	}

	return out
}
