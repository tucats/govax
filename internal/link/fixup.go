package link

import (
	"encoding/binary"
	"fmt"
	"sort"
)

// sharedRef is a shareable image the image refers to, and the offsets of
// the targets its references reach in it, each of which gets a cell in
// the fixup section. addresses are the image addresses of the .ADDRESS
// longwords that hold an offset in it, which the image activator adds its
// base to. names are the global symbols the offsets were reached through,
// by offset, and orders those symbols' places in the image's global symbol
// table, which order the cells (see orderCells).
type sharedRef struct {
	image     SharedImage
	offsets   []uint32
	addresses []uint32
	names     map[uint32]string
	orders    map[uint32]int
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
func (l *linker) referShared(name string, offset uint32, sym string, field uint32) {
	ref := l.sharedTarget(name, offset, sym)
	l.gRefs = append(l.gRefs, gRef{field: field, shared: ref, offset: offset})
}

// referAddress records a .ADDRESS longword at addr that holds offset in the
// shareable image name. Real LINK gives the target a cell too, as it does a
// general mode operand's (ADDR.EXE, testdata/link/vax).
func (l *linker) referAddress(name string, offset uint32, sym string, addr uint32) {
	ref := l.sharedTarget(name, offset, sym)
	ref.addresses = append(ref.addresses, addr)
	l.addressFixups++
}

// sharedTarget records a reference to offset in the shareable image name,
// reached through the global symbol sym ("" for none), giving it a cell,
// and returns the image's record.
func (l *linker) sharedTarget(name string, offset uint32, sym string) *sharedRef {
	ref := l.sharedRef(name)
	if ref == nil {
		ref = l.addShared(l.sharedImage(name))
	}

	if _, named := ref.names[offset]; !named && sym != "" {
		ref.names[offset] = sym

		if g := l.symbols[sym]; g != nil {
			ref.orders[offset] = g.order
		}
	}

	found := false
	for _, o := range ref.offsets {
		found = found || o == offset
	}

	if !found {
		ref.offsets = append(ref.offsets, offset)
	}

	return ref
}

// sharedRef returns the record of the shareable image name among those
// the image refers to, or nil.
func (l *linker) sharedRef(name string) *sharedRef {
	for _, r := range l.shared {
		if r.image.Name == name {
			return r
		}
	}

	return nil
}

// addShared adds a shareable image to the ones the image refers to.
func (l *linker) addShared(img SharedImage) *sharedRef {
	ref := &sharedRef{image: img, names: map[uint32]string{}, orders: map[uint32]int{}}
	l.shared = append(l.shared, ref)

	return ref
}

// orderCells puts a shareable image's targets in the order real LINK
// gives their cells. That depends on how the link found the image, and
// every fixture with two cells or more fits one of two rules:
//
//   - An image an options file named (byName): the symbols' names in
//     descending order. testdata/link68's MAIN1 gives SHR1's six cells as
//     SHR1_TABLE, SHOW, JSB, COUNT, BASE, ADD, though SHR1's own table has
//     them in another order, and so do MAIN1A, MAIN1N, MAIN2, MAIN3P,
//     MAIN4, E8, and SHR2.
//   - An image found in a library (IMAGELIB.OLB, or a user's shareable
//     image library): the symbols' places in the image's global symbol
//     table, in descending order. MAIN1L and MAIN1U, which found SHR1 in
//     MYSHR.OLB, give its cells as SHR1_COUNT, JSB, BASE, ADD, SHOW,
//     TABLE, the reverse of SHR1's table; and LIBRTL's table puts the
//     cells of FAILSIG (LIB$STOP, LIB$SIGNAL), PG66, PI, CELLS, CELLS2,
//     and DBGCMD in their real orders too, which Phase 29 found no rule
//     for.
//
// A library image whose table places govax doesn't know (govax's own
// tables, vmsdef's, without them) falls back on the names in ascending
// order, which CELLS and CELLS2 fit and FAILSIG doesn't. A target reached
// without a symbol (an offset from arithmetic) has no name, and keeps its
// place after the named ones.
func (r *sharedRef) orderCells(byName bool) {
	byOrder := !byName

	for _, o := range r.offsets {
		if r.names[o] != "" && r.orders[o] == 0 {
			byOrder = false
		}
	}

	sort.SliceStable(r.offsets, func(i, j int) bool {
		a, b := r.offsets[i], r.offsets[j]

		if (r.names[a] == "") != (r.names[b] == "") {
			return r.names[b] == ""
		}

		switch {
		case byName:
			return r.names[a] > r.names[b]
		case byOrder:
			return r.orders[a] > r.orders[b]
		default:
			return r.names[a] < r.names[b]
		}
	})
}

// namedInOptions says an options file named the shareable image name
// (NAME/SHAREABLE), rather than the link finding it in a library.
func (l *linker) namedInOptions(name string) bool {
	for _, c := range l.opts.Clusters {
		if c.Shareable && c.Image == name {
			return true
		}
	}

	return false
}

// fixupLayout is where each part of the fixup section goes, as offsets
// from its start.
type fixupLayout struct {
	gfix  uint32 // the G^ fixup lists
	icp   uint32 // the change-protection list
	shl   uint32 // the shareable image list
	addr  uint32 // the .ADDRESS fixup lists, 0 if there are none
	size  uint32
	cells map[*sharedRef][]uint32 // each target's cell, by its offset's position
}

// layoutFixups places the fixup section's parts: the fixed part, then a
// G^ fixup list for each shareable image (its count, its index in the
// shareable image list, and a cell for each target), ended by a zero
// count, then the change-protection list, then the shareable image list,
// whose first entry is the image itself, then, if any .ADDRESS longwords
// hold offsets in shareable images, a .ADDRESS fixup list for each such
// image (its count, its index, and each longword's address relative to the
// image's base), ended by a zero count.
func (l *linker) layoutFixups() fixupLayout {
	f := fixupLayout{gfix: iafFixedLength, cells: map[*sharedRef][]uint32{}}

	for _, r := range l.shared {
		r.orderCells(l.namedInOptions(r.image.Name))
	}

	// An image placed at link time has no cells: its references were
	// resolved then (EXE1's LIBRTL), so it has no list.
	p := f.gfix
	for _, r := range l.shared {
		if len(r.offsets) == 0 {
			continue
		}

		p += 8 // the count and the image's index

		for range r.offsets {
			f.cells[r] = append(f.cells[r], p)
			p += 4
		}
	}

	p += 4 // the zero count that ends the lists

	f.icp = p
	f.shl = f.icp + 4 + icpEntryLength*uint32(1+len(l.readOnlyFixups))
	f.size = f.shl + uint32(1+len(l.shared))*shlEntryLength

	if l.addressFixups > 0 || len(l.selfAddresses) > 0 {
		f.addr = f.size
		f.size += 4 // the zero count that ends the lists

		if len(l.selfAddresses) > 0 {
			f.size += 8 + 4*uint32(len(l.selfAddresses))
		}

		for _, r := range l.shared {
			if len(r.addresses) > 0 {
				f.size += 8 + 4*uint32(len(r.addresses))
			}
		}
	}

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

	le.PutUint16(b[0x08:], iafFixedLength)

	if f.icp > f.gfix+4 {
		le.PutUint32(b[0x0C:], f.gfix)
	}

	le.PutUint32(b[0x14:], f.icp)
	le.PutUint32(b[0x18:], f.shl)
	le.PutUint32(b[0x1C:], uint32(1+len(l.shared))) // the image itself, and each shareable image

	// Each shareable image's G^ fixup list: each cell holds the target's
	// offset in the image, to which the image activator adds its base.
	p := f.gfix
	for i, r := range l.shared {
		if len(r.offsets) == 0 {
			continue
		}

		le.PutUint32(b[p:], uint32(len(r.offsets)))
		le.PutUint32(b[p+4:], uint32(i+1))

		for j, o := range r.offsets {
			le.PutUint32(b[f.cells[r][j]:], o)
		}

		p += 8 + 4*uint32(len(r.offsets))
	}

	// The change-protection entry: the fixup section itself, which the
	// image activator makes user read, executive write once it has done
	// the fixups. Its address is relative to the image's base, its first
	// section's address (ANALYZE/IMAGE: "relative to %X'00000200'", and
	// EXE1's %X'00010000').
	//
	// Then each read-only section of a shareable image the activator
	// stores .ADDRESS fixups in, made user read once they're done (SHR6:
	// "address: %X'00000000', page count: 1, protection: PRT$C_UR", after
	// the fixup section's entry).
	le.PutUint32(b[f.icp:], uint32(1+len(l.readOnlyFixups)))
	le.PutUint32(b[f.icp+4:], fixupVA-l.imageBase)
	le.PutUint16(b[f.icp+8:], uint16(len(b)/blockSize))
	le.PutUint16(b[f.icp+10:], prtUREW)

	for i, s := range l.readOnlyFixups {
		entry := b[f.icp+4+uint32(i+1)*icpEntryLength:]
		le.PutUint32(entry, s.base-l.imageBase)
		le.PutUint16(entry[4:], uint16(s.pages))
		le.PutUint16(entry[6:], prtUR)
	}

	// The .ADDRESS fixup lists, each longword's address relative to the
	// image's base.
	if f.addr != 0 {
		le.PutUint32(b[0x10:], f.addr)

		p := f.addr

		// A shareable image's own .ADDRESS longwords come first, a list
		// for image 0, the image itself (shareable.go).
		lists := [][]uint32{l.selfAddresses}
		for _, r := range l.shared {
			lists = append(lists, r.addresses)
		}

		for i, addresses := range lists {
			if len(addresses) == 0 {
				continue
			}

			le.PutUint32(b[p:], uint32(len(addresses)))
			le.PutUint32(b[p+4:], uint32(i))
			p += 8

			for _, a := range addresses {
				le.PutUint32(b[p:], a-l.imageBase)
				p += 4
			}
		}
	}

	if l.opts.Shareable {
		le.PutUint16(b[iafFlags:], iafSHR)
	}

	// The shareable image list: the image itself, then each shareable
	// image's name.
	b[f.shl+0x10] = shlEntryLength

	for i, r := range l.shared {
		entry := b[f.shl+uint32(i+1)*shlEntryLength:]
		_ = putCounted(entry[0x18:shlEntryLength], r.image.Name)
	}

	return b
}

// globalSectionISD returns the ISD that maps a shareable image the
// image activator places: a global section (ISD$V_GBL) the size of the
// image's first section, with its ident and match control, named for that
// section. Its type and attributes are the first section's too: SHR3P's
// is ISD$K_SHRFXD, and SHR6's, which the activator fixes up, is CRF+WRT
// (testdata/link68's MAIN3P and MAIN6). An image whose sections aren't
// known is taken to start with a shared, position independent one, as
// LIBRTL does.
func globalSectionISD(img SharedImage) (isd, error) {
	flags := uint32(isdTypeSharedPIC)
	if len(img.Layout) > 0 {
		flags = img.Layout[0].Flags
	}

	d := globalISD(img, 1, img.Pages, flags)
	if len(d.name) > 43 {
		return isd{}, fmt.Errorf("shareable image name %q is too long", img.Name)
	}

	return d, nil
}

// globalISD returns a global section ISD for section n (from 1) of the
// shareable image img: pages pages, with flags (the section's type and
// attributes), img's ident and match control, and the name NAME_00n.
func globalISD(img SharedImage, n int, pages, flags uint32) isd {
	return isd{
		pages: pages,
		flags: isdGBL | uint32(img.Match)<<isdMatchShift | flags,
		ident: uint32(img.MajorID)<<24 | img.MinorID&0xFFFFFF,
		name:  fmt.Sprintf("%s_%03d", img.Name, n),
	}
}
