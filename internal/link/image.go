package link

import (
	"encoding/binary"
	"fmt"
	"time"
)

// This file writes a VAX/VMS executable image: the image header block,
// then each image section's pages, then the fixup section. Only the first
// header record ($IHDDEF) is in the VMS source archive as SDL
// (reference/vms/ihddef.sdl); the other layouts come from the images real
// LINK V11-39 wrote for the Phase 27 fixtures (testdata/mar/vax/*.exe and
// govax/gv_*.exe) and from eVAX's imgdef.h, as docs/PHASE-30.md's "What
// real LINK writes" records.

// blockSize is a disk block's, and a VAX page's, size.
const blockSize = 512

// Image header offsets and sizes. The fixed header ($IHDDEF) is 0x30 bytes,
// followed by the activation ($IHADEF, 0x14 bytes), symbol table and debug
// ($IHSDEF, 0x1C bytes), and identification ($IHIDEF, 0x50 bytes) blocks.
// The image section descriptors start at IHD$W_SIZE.
const (
	ihdLength = 0x30
	ihaLength = 0x14
	ihsLength = 0x1C
	ihiLength = 0x50

	ihaOffset = ihdLength
	ihsOffset = ihaOffset + ihaLength
	ihiOffset = ihsOffset + ihsLength
	isdOffset = ihiOffset + ihiLength

	// IHD$W_MAJORID and IHD$W_MINORID are the ASCII strings "02" and "05".
	ihdMajorID = '0' | '2'<<8
	ihdMinorID = '0' | '5'<<8

	// ihdTypeExecutable is IHD$K_EXE.
	ihdTypeExecutable = 1

	// ihdLinkFlags is IHD$L_LNKFLAGS as real LINK wrote it in every
	// fixture image. ANALYZE/IMAGE names the bits: IHD$V_PICIMG,
	// IHD$V_DBGDMT, and IHD$V_IHSLONG. The top byte, 1, is
	// IHD$V_MATCHCTL, which ANALYZE doesn't show.
	ihdPICIMG    = 1 << 3
	ihdDBGDMT    = 1 << 5
	ihdIHSLONG   = 1 << 7
	ihdMatchCtl  = 1 << 24
	ihdLinkFlags = ihdPICIMG | ihdDBGDMT | ihdIHSLONG | ihdMatchCtl
)

// ISD flags (ISD$V_xxx, from eVAX's imgdef.h), and the section type in the
// flags' top byte.
const (
	isdGBL      = 1 << 0  // a global section
	isdCRF      = 1 << 1  // copy on reference
	isdDZRO     = 1 << 2  // demand zero
	isdWRT      = 1 << 3  // writable
	isdLASTCLU  = 1 << 7  // in the last cluster
	isdFIXUPVEC = 1 << 10 // the fixup section

	isdTypeUserStack = 253 << 24
	isdTypeSharedPIC = 3 << 24 // ISD$K_SHRPIC

	// isdMatchShift is where a global section ISD's match control is in
	// its flags (ISD$V_MATCHCTL).
	isdMatchShift = 4

	// isdGlobalFixedSize is a global section ISD's size before its
	// counted section name: a private section's, then the global
	// section ident.
	isdGlobalFixedSize = 20

	// isdPrivateSize and isdDemandZeroSize are an ISD's size: a private
	// section's has a VBN, and a demand-zero section's doesn't.
	isdPrivateSize    = 16
	isdDemandZeroSize = 12
)

// Fixup section layout (the IAF and what follows it). The fixed part is
// 0x40 bytes. Then come the G^ fixup lists (an empty one is a zero
// longword), the change-protection list, and the shareable image list,
// whose entries are 0x40 bytes, the first being the image itself.
const (
	iafFixedLength = 0x40
	shlEntryLength = 0x40
	icpEntryLength = 8

	// prtUREW is PRT$C_UREW, user read and executive write: the
	// protection the change-protection entry gives the fixup section.
	prtUREW = 0x0D
)

// defaultStackPages is the user stack real LINK allocates without a
// STACK= option.
const defaultStackPages = 20

// vmsEpoch is the VMS time base, 17-Nov-1858, in Unix seconds.
const vmsEpoch = -3506716800

// vmsTime returns t as a VMS quadword time: 100-nanosecond ticks since
// 17-Nov-1858, in local wall-clock time, as VMS keeps it.
func vmsTime(t time.Time) uint64 {
	local := time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), time.UTC)

	return uint64(local.Unix()-vmsEpoch)*10_000_000 + uint64(local.Nanosecond()/100)
}

// isd is one image section descriptor to write.
type isd struct {
	pages uint32
	vpn   uint32
	flags uint32
	vbn   uint32 // 0 for a demand-zero section
}

func (d isd) encode() []byte {
	size := isdPrivateSize
	if d.flags&isdDZRO != 0 {
		size = isdDemandZeroSize
	}

	b := make([]byte, size)
	binary.LittleEndian.PutUint16(b[0:], uint16(size))
	binary.LittleEndian.PutUint16(b[2:], uint16(d.pages))
	binary.LittleEndian.PutUint32(b[4:], d.vpn)
	binary.LittleEndian.PutUint32(b[8:], d.flags)

	if size == isdPrivateSize {
		binary.LittleEndian.PutUint32(b[12:], d.vbn)
	}

	return b
}

// header returns the image header block. The global section ISDs, which
// map shareable images, follow the others.
func (l *linker) header(isds []isd, global [][]byte, fixupVA uint32) ([]byte, error) {
	b := make([]byte, blockSize)

	le := binary.LittleEndian
	le.PutUint16(b[0x00:], isdOffset)
	le.PutUint16(b[0x02:], ihaOffset)
	le.PutUint16(b[0x04:], ihsOffset)
	le.PutUint16(b[0x06:], ihiOffset)
	le.PutUint16(b[0x0C:], ihdMajorID)
	le.PutUint16(b[0x0E:], ihdMinorID)
	b[0x10] = 1 // IHD$B_HDRBLKCNT
	b[0x11] = ihdTypeExecutable

	for i := 0x14; i < 0x1C; i++ {
		b[i] = 0xFF // IHD$Q_PRIVREQS: all
	}

	linkTime := vmsTime(l.opts.Time)

	le.PutUint32(b[0x20:], ihdLinkFlags)
	le.PutUint32(b[0x24:], uint32(linkTime>>16)) // IHD$L_IDENT
	le.PutUint32(b[0x2C:], fixupVA)              // IHD$L_IAFVA

	// IHA: the transfer addresses. With traceback, the image activator
	// calls SYS$IMGSTA first, which sets up traceback and then calls the
	// program.
	transfers := make([]uint32, 0, 2)
	if l.opts.Traceback {
		transfers = append(transfers, sysImgsta)
	}

	if l.transferSet {
		transfers = append(transfers, l.transfer)
	}

	for i, t := range transfers {
		le.PutUint32(b[ihaOffset+4*i:], t)
	}

	// IHS is all zero: no debug symbol table or global symbol table.

	// IHI: the image name, image ID, link time, and linker ID, each
	// name a counted string in a fixed field.
	if err := putCounted(b[ihiOffset:ihiOffset+40], l.opts.ImageName); err != nil {
		return nil, fmt.Errorf("image name: %w", err)
	}

	if err := putCounted(b[ihiOffset+40:ihiOffset+56], l.imageID); err != nil {
		return nil, fmt.Errorf("image identification: %w", err)
	}

	le.PutUint64(b[ihiOffset+56:], linkTime)

	if err := putCounted(b[ihiOffset+64:ihiOffset+80], l.opts.LinkerID); err != nil {
		return nil, fmt.Errorf("linker identification: %w", err)
	}

	// The ISDs, a zero word to end them, and 0xFF to the end of the
	// block.
	p := isdOffset

	encoded := make([][]byte, 0, len(isds)+len(global))
	for _, d := range isds {
		encoded = append(encoded, d.encode())
	}

	encoded = append(encoded, global...)

	for _, e := range encoded {
		if p+len(e)+2 > blockSize {
			return nil, fmt.Errorf("%d image sections don't fit in one header block", len(encoded))
		}

		p += copy(b[p:], e)
	}

	p += 2

	for ; p < blockSize; p++ {
		b[p] = 0xFF
	}

	return b, nil
}

// putCounted stores s as a counted string in field, which must hold it.
func putCounted(field []byte, s string) error {
	if len(s) > len(field)-1 {
		return fmt.Errorf("%q is longer than %d characters", s, len(field)-1)
	}

	field[0] = byte(len(s))
	copy(field[1:], s)

	return nil
}
