package link

import (
	"encoding/binary"
	"fmt"
)

// sharedRef is a shareable image the image refers to, and the offsets of
// the targets its general mode operands reach in it, each of which gets
// a cell in the fixup section.
type sharedRef struct {
	image   SharedImage
	offsets []uint32
}

// gRef is a general mode operand that reaches a shareable image: the
// address of its displacement, and the target's image and offset.
type gRef struct {
	field  uint32
	shared *sharedRef
	offset uint32
}

// referShared records a general mode operand at field (its displacement)
// that reaches offset in the shareable image name.
func (l *linker) referShared(name string, offset, field uint32) {
	var ref *sharedRef

	for _, r := range l.shared {
		if r.image.Name == name {
			ref = r

			break
		}
	}

	if ref == nil {
		ref = &sharedRef{image: l.sharedImage(name)}
		l.shared = append(l.shared, ref)
	}

	found := false
	for _, o := range ref.offsets {
		found = found || o == offset
	}

	if !found {
		ref.offsets = append(ref.offsets, offset)
	}

	l.gRefs = append(l.gRefs, gRef{field: field, shared: ref, offset: offset})
}

// fixupLayout is where each part of the fixup section goes, as offsets
// from its start.
type fixupLayout struct {
	gfix  uint32 // the G^ fixup lists
	icp   uint32 // the change-protection list
	shl   uint32 // the shareable image list
	size  uint32
	cells map[*sharedRef][]uint32 // each target's cell, by its offset's position
}

// layoutFixups places the fixup section's parts: the fixed part, then a
// G^ fixup list for each shareable image (its count, its index in the
// shareable image list, and a cell for each target), ended by a zero
// count, then the change-protection list, then the shareable image list,
// whose first entry is the image itself.
func (l *linker) layoutFixups() fixupLayout {
	f := fixupLayout{gfix: iafFixedLength, cells: map[*sharedRef][]uint32{}}

	p := f.gfix
	for _, r := range l.shared {
		p += 8 // the count and the image's index

		for range r.offsets {
			f.cells[r] = append(f.cells[r], p)
			p += 4
		}
	}

	p += 4 // the zero count that ends the lists

	f.icp = p
	f.shl = f.icp + 4 + icpEntryLength
	f.size = f.shl + uint32(1+len(l.shared))*shlEntryLength

	return f
}

// patchGRefs stores each general mode operand's displacement to its cell
// in the fixup section at fixupVA.
func (l *linker) patchGRefs(f fixupLayout, fixupVA uint32) error {
	for _, g := range l.gRefs {
		for i, o := range g.shared.offsets {
			if o != g.offset {
				continue
			}

			disp := make([]byte, 4)
			binary.LittleEndian.PutUint32(disp, fixupVA+f.cells[g.shared][i]-(g.field+4))

			if err := l.write(g.field, disp); err != nil {
				return err
			}
		}
	}

	return nil
}

// fixupSection returns the fixup section's contents, a whole number of
// pages, as real LINK writes them (docs/PHASE-30.md).
func (l *linker) fixupSection(f fixupLayout, fixupVA uint32) []byte {
	b := make([]byte, pageUp(f.size))
	le := binary.LittleEndian

	le.PutUint32(b[0x08:], iafFixedLength)

	if len(l.shared) > 0 {
		le.PutUint32(b[0x0C:], f.gfix)
	}

	le.PutUint32(b[0x14:], f.icp)
	le.PutUint32(b[0x18:], f.shl)
	le.PutUint32(b[0x1C:], uint32(1+len(l.shared))) // the image itself, and each shareable image

	// Each shareable image's G^ fixup list: each cell holds the target's
	// offset in the image, to which the image activator adds its base.
	p := f.gfix
	for i, r := range l.shared {
		le.PutUint32(b[p:], uint32(len(r.offsets)))
		le.PutUint32(b[p+4:], uint32(i+1))

		for j, o := range r.offsets {
			le.PutUint32(b[f.cells[r][j]:], o)
		}

		p += 8 + 4*uint32(len(r.offsets))
	}

	// The change-protection entry: the fixup section itself, which the
	// image activator makes user read, executive write once it has done
	// the fixups. Its address is relative to the image's base
	// (ANALYZE/IMAGE: "relative to %X'00000200'").
	le.PutUint32(b[f.icp:], 1)
	le.PutUint32(b[f.icp+4:], fixupVA-imageBase)
	le.PutUint16(b[f.icp+8:], uint16(len(b)/blockSize))
	le.PutUint16(b[f.icp+10:], prtUREW)

	// The shareable image list: the image itself, then each shareable
	// image's name.
	b[f.shl+0x10] = shlEntryLength

	for i, r := range l.shared {
		entry := b[f.shl+uint32(i+1)*shlEntryLength:]
		_ = putCounted(entry[0x18:shlEntryLength], r.image.Name)
	}

	return b
}

// globalSectionISD returns the ISD that maps a shareable image: a global
// section (ISD$V_GBL), position independent and shareable (ISD$K_SHRPIC),
// with the image's size, ident, and match control, named for the image's
// first section.
func globalSectionISD(img SharedImage) ([]byte, error) {
	name := img.Name + "_001"
	if len(name) > 43 {
		return nil, fmt.Errorf("shareable image name %q is too long", img.Name)
	}

	size := isdGlobalFixedSize + 1 + len(name)
	b := make([]byte, size)
	le := binary.LittleEndian

	le.PutUint16(b[0:], uint16(size))
	le.PutUint16(b[2:], uint16(img.Pages))
	le.PutUint32(b[8:], isdGBL|uint32(img.Match)<<isdMatchShift|isdTypeSharedPIC)
	le.PutUint32(b[16:], uint32(img.MajorID)<<24|img.MinorID&0xFFFFFF)
	b[20] = byte(len(name))
	copy(b[21:], name)

	return b, nil
}
