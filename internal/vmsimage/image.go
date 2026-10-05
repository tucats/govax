package vmsimage

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// This file decodes a VAX/VMS image file, for ANALYZE/IMAGE
// (docs/PHASE-40.md) and the debug symbol reader (internal/dbgsym,
// docs/PHASE-41.md). An image starts with one or more 512-byte header
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

// BlockSize is a disk block's size, and a VAX page's.
const BlockSize = 512

// Fixed header ($IHD) offsets.
const (
	IHDISDOffset     = 0x00 // word: the first ISD
	IHDActivOffset   = 0x02 // word: the activation block
	IHDSymDbgOffset  = 0x04 // word: the symbol table and debug block
	IHDImgIDOffset   = 0x06 // word: the identification block
	IHDPatchOffset   = 0x08 // word: the patch block, 0 for none
	IHDMajorIDOffset = 0x0C // two ASCII characters: "02"
	IHDMinorIDOffset = 0x0E // two ASCII characters: "05"
	IHDBlockCount    = 0x10 // byte: how many header blocks
	IHDImageType     = 0x11 // byte: IHD$K_EXE, IHD$K_LIM, ...
	IHDIOChannels    = 0x1C // word: I/O channels, 0 for the default
	IHDIOPages       = 0x1E // word: I/O pages, 0 for the default
	IHDLinkFlags     = 0x20 // longword: IHD$V_ flags
	IHDIdent         = 0x24 // longword: the image's ident
	IHDSysVersion    = 0x28 // longword: the system version linked against
	IHDIAFVA         = 0x2C // longword: the fixup section's address
	IHDFixedLength   = 0x30
)

// Image section descriptor ($ISD) layout and flags.
const (
	ISDSizeOffset  = 0x00 // word: the ISD's size; 0 ends the list, 0xFFFF goes on in the next block
	ISDPagesOffset = 0x02 // word: the section's pages
	ISDVPNOffset   = 0x04 // longword: the VPN (low 23 bits) and page fault cluster (top byte)
	ISDFlagsOffset = 0x08 // longword: ISD$V_ flags, the section type in the top byte
	ISDVBNOffset   = 0x0C // longword: the section's first block in the file
	ISDIdentOffset = 0x10 // longword: a global section's ident
	ISDNameOffset  = 0x14 // counted string: a global section's name

	ISDDemandZeroLength = 0x0C // a demand-zero section's ISD has no VBN
	ISDPrivateLength    = 0x10
	ISDGlobalLength     = 0x14 // a global section's, before its name

	ISDContinue = 0xFFFF

	ISDFlagGBL      = 1 << 0  // a global section
	ISDFlagDZRO     = 1 << 2  // demand zero
	ISDFlagFIXUPVEC = 1 << 10 // the image activator's fixup section

	ISDVPNMask   = 0x7FFFFF
	ISDP1VPN     = 1 << 21 // a VPN in P1 space
	ISDMatchMask = 0x70    // ISD$V_MATCHCTL, bits 4-6
	ISDMatchBit  = 4
)

// Link flags (IHD$L_LNKFLAGS) the decoder and its users test.
const (
	// IHDFlagINISHR is IHD$V_INISHR: the image has a shareable image
	// initialization list.
	IHDFlagINISHR = 1 << 6
	// IHDFlagIHSLONG is IHD$V_IHSLONG: the symbol table block's 32-bit
	// sizes are the ones to use.
	IHDFlagIHSLONG = 1 << 7
)

// Fixup section ($IAF) offsets.
const (
	IAFGFixOffset    = 0x0C // longword: the G^ reference fixup lists
	IAFDotAddrOffset = 0x10 // longword: the .ADDRESS reference fixup lists
	IAFChgPrtOffset  = 0x14 // longword: the protection change list
	IAFShlOffset     = 0x18 // longword: the shareable image list
	IAFShrImgCount   = 0x1C // longword: entries in the shareable image list
	IAFShlExtra      = 0x20 // longword: extra shareable image entries (unconfirmed)
	IAFFlags         = 0x24 // longword: IAF$V_ flags (unconfirmed)
	IAFFixedLength   = 0x40

	// SHLEntryLength is a shareable image list entry's size, and
	// SHLNameOffset where its counted name is.
	SHLEntryLength = 0x40
	SHLNameOffset  = 0x18
	SHLNameLength  = 40
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

	// The activation block's transfer addresses, and, with
	// IHD$V_INISHR, the address of the shareable image initialization
	// list.
	Transfers [3]uint32
	InitShare uint32

	// The symbol table and debug block.
	DSTVBN, GSTVBN, DMTVBN uint32
	DSTBlocks, GSTRecords  uint16
	DMTBytes               uint32

	// The symbol table block's 32-bit sizes (IHS$L_DSTBLKS and
	// IHS$L_GSTRECS), which an image with IHD$V_IHSLONG set says to use
	// in place of the 16-bit ones (docs/DEBUG-RECORDS.md, 2.2).
	DSTBlocksLong, GSTRecordsLong uint32

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
func (d ISD) Match() int { return int(d.Flags&ISDMatchMask) >> ISDMatchBit }

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
	if len(data) < BlockSize {
		return nil, errNotImage
	}

	img := &Image{FileBlocks: (len(data) + BlockSize - 1) / BlockSize}
	le := binary.LittleEndian

	img.Blocks = int(data[IHDBlockCount])
	if img.Blocks == 0 {
		img.Blocks = 1
	}

	if img.Blocks*BlockSize > len(data) {
		img.problem("The header block count, %d, is more than the file holds.", img.Blocks)
		img.Blocks = len(data) / BlockSize
	}

	h := data[:img.Blocks*BlockSize]
	img.Header = h

	img.MajorID = string(h[IHDMajorIDOffset : IHDMajorIDOffset+2])
	img.MinorID = string(h[IHDMinorIDOffset : IHDMinorIDOffset+2])
	img.Type = h[IHDImageType]
	img.IOChannels = le.Uint16(h[IHDIOChannels:])
	img.IOPages = le.Uint16(h[IHDIOPages:])
	img.LinkFlags = le.Uint32(h[IHDLinkFlags:])
	img.Ident = le.Uint32(h[IHDIdent:])
	img.SysVersion = le.Uint32(h[IHDSysVersion:])
	img.IAFVA = le.Uint32(h[IHDIAFVA:])

	img.ActivOffset = int(le.Uint16(h[IHDActivOffset:]))
	img.SymDbgOffset = int(le.Uint16(h[IHDSymDbgOffset:]))
	img.ImgIDOffset = int(le.Uint16(h[IHDImgIDOffset:]))
	img.PatchOffset = int(le.Uint16(h[IHDPatchOffset:]))

	img.activation(h)
	img.symbolTables(h)
	img.identification(h)
	img.patch(h)

	img.part = PartSections
	img.sections(h, int(le.Uint16(h[IHDISDOffset:])))

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
	if offset < IHDFixedLength || offset+n > BlockSize {
		img.problem("The %s block's offset, %d, is outside the header.", what, offset)

		return nil
	}

	return h[offset : offset+n]
}

// Header block sizes.
const (
	IHALength    = 0x14
	IHAInitShare = 0x10
	IHSLength    = 0x1C
	IHILength    = 0x50
)

func (img *Image) activation(h []byte) {
	if img.ActivOffset == 0 {
		return
	}

	b := img.block(h, "activation", img.ActivOffset, IHALength)
	if b == nil {
		return
	}

	for i := range img.Transfers {
		img.Transfers[i] = binary.LittleEndian.Uint32(b[4*i:])
	}

	// After the transfer addresses comes a zero longword that ends them,
	// then the initialization list's address (unconfirmed: no fixture
	// image sets IHD$V_INISHR).
	img.InitShare = binary.LittleEndian.Uint32(b[IHAInitShare:])
}

func (img *Image) symbolTables(h []byte) {
	if img.SymDbgOffset == 0 {
		return
	}

	b := img.block(h, "symbol table", img.SymDbgOffset, IHSLength)
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
	img.DSTBlocksLong = le.Uint32(b[20:])
	img.GSTRecordsLong = le.Uint32(b[24:])
}

// DSTBlockCount is the debug symbol table's size in blocks: the 32-bit
// count when the image has IHD$V_IHSLONG, the 16-bit one otherwise.
func (img *Image) DSTBlockCount() uint32 {
	if img.LinkFlags&IHDFlagIHSLONG != 0 {
		return img.DSTBlocksLong
	}

	return uint32(img.DSTBlocks)
}

// GSTRecordCount is the global symbol table's size in records, chosen as
// DSTBlockCount chooses.
func (img *Image) GSTRecordCount() uint32 {
	if img.LinkFlags&IHDFlagIHSLONG != 0 {
		return img.GSTRecordsLong
	}

	return uint32(img.GSTRecords)
}

// Blocks returns count blocks of data starting at virtual block vbn (the
// file's first block is VBN 1), or nil if the file doesn't hold them all.
func Blocks(data []byte, vbn, count uint32) []byte {
	if vbn == 0 {
		return nil
	}

	start := uint64(vbn-1) * BlockSize
	end := start + uint64(count)*BlockSize

	if end > uint64(len(data)) {
		return nil
	}

	return data[start:end]
}

func (img *Image) identification(h []byte) {
	if img.ImgIDOffset == 0 {
		return
	}

	b := img.block(h, "identification", img.ImgIDOffset, IHILength)
	if b == nil {
		return
	}

	img.Name = counted(b[0:40])
	img.FileID = counted(b[40:56])
	img.LinkTime = binary.LittleEndian.Uint64(b[56:])
	img.LinkerID = counted(b[64:80])
}

// IHPLength is the patch block's size (unconfirmed: no fixture image
// has one).
const IHPLength = 0x20

func (img *Image) patch(h []byte) {
	if img.PatchOffset == 0 {
		return
	}

	img.Patch = img.block(h, "patch", img.PatchOffset, IHPLength)
}

// sections reads the ISDs, starting at offset in the header blocks.
func (img *Image) sections(h []byte, offset int) {
	le := binary.LittleEndian
	p := offset

	if p < IHDFixedLength {
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

		case size == ISDContinue:
			// The list goes on at the start of the next header block.
			p = (p/BlockSize + 1) * BlockSize

			continue

		case size < ISDDemandZeroLength || p+size > len(h):
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
	vpn := le.Uint32(b[ISDVPNOffset:])

	d := ISD{
		Size:  len(b),
		Pages: le.Uint16(b[ISDPagesOffset:]),
		VPN:   vpn & ISDVPNMask,
		PFC:   byte(vpn >> 24),
		Flags: le.Uint32(b[ISDFlagsOffset:]),
	}

	if len(b) >= ISDPrivateLength {
		d.VBN = le.Uint32(b[ISDVBNOffset:])
	}

	if d.Flags&ISDFlagGBL != 0 && len(b) > ISDGlobalLength {
		d.GlobalIdent = le.Uint32(b[ISDIdentOffset:])
		d.GlobalName = counted(b[ISDNameOffset:])
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
		if img.IAFVA >= d.Address() && img.IAFVA < d.Address()+uint32(d.Pages)*BlockSize && d.Flags&ISDFlagGBL == 0 {
			sec = d

			break
		}
	}

	if sec == nil {
		img.problem("No image section holds the fixup section at %%X'%08X'.", img.IAFVA)

		return
	}

	start := (int(sec.VBN)-1)*BlockSize + int(img.IAFVA-sec.Address())
	end := (int(sec.VBN) - 1 + int(sec.Pages)) * BlockSize

	if sec.VBN == 0 || start+IAFFixedLength > len(data) {
		img.problem("The fixup section at %%X'%08X' is outside the file.", img.IAFVA)

		return
	}

	b := data[start:min(end, len(data))]
	le := binary.LittleEndian

	f := &Fixups{
		VA:            img.IAFVA,
		Base:          img.base(),
		GFixOffset:    le.Uint32(b[IAFGFixOffset:]),
		DotAddrOffset: le.Uint32(b[IAFDotAddrOffset:]),
		ChgPrtOffset:  le.Uint32(b[IAFChgPrtOffset:]),
		ShlOffset:     le.Uint32(b[IAFShlOffset:]),
		ShareCount:    le.Uint32(b[IAFShrImgCount:]),
		Extra:         le.Uint32(b[IAFShlExtra:]),
		Flags:         le.Uint32(b[IAFFlags:]),
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

	names := make([]string, 0, min(f.ShareCount, uint32(len(b)/SHLEntryLength)))

	for i := range f.ShareCount {
		p := uint64(f.ShlOffset) + uint64(i)*SHLEntryLength
		if p+SHLEntryLength > uint64(len(b)) {
			img.problem("The shareable image list runs past the end of the fixup section.")

			break
		}

		names = append(names, counted(b[p+SHLNameOffset:p+SHLNameOffset+SHLNameLength]))
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
