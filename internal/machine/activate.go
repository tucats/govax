package machine

import (
	"errors"
	"fmt"
	"strings"

	"github.com/tucats/govax/internal/vmsdef"
	"github.com/tucats/govax/internal/vmserrors"
	"github.com/tucats/govax/internal/vmsimage"
)

// Activating a program's shareable images (docs/PHASE-68 - link
// shareable images.md, subtask 11), as VMS's image activator does for an
// image that isn't installed (the book's 21.1.1.4), in both loaders:
//
//   - Each image goes where the image that names it says: a section the
//     link placed (a global section ISD with ISD$V_BASED: a based
//     shareable image, or any image an executable linked with BASE= put
//     in a gap) at its address, any other at P0's high-water mark
//     (imageBase).
//   - The global section ident and match control the image was linked
//     with are checked against the shareable image's own header
//     (identMatches); a mismatch is SS$_SHRIDMISMAT
//     (IdentMismatchError).
//   - An image govax's shims stand for that the link placed gets pages
//     at the placed address, holding a transfer vector into the shims
//     (placeEmulated), so the references the link resolved to those
//     addresses reach them.
//
// The image's own fixups (its G^ cells, the .ADDRESS list relative to
// itself, image 0) and its protection changes are imageFixup's and
// setImageProtection's, as for any image.

// isdBASED is ISD$V_BASED: a global section ISD whose section the link
// placed at the ISD's address.
const isdBASED = 1 << 9

// p1Start is P1's first address: what is at it or above isn't in P0.
const p1Start = 0x40000000

// ihdPICIMG is IHD$V_PICIMG: a shareable image that isn't based, which
// may go anywhere.
const ihdPICIMG = 1 << 3

// The match controls, ISD$K_MATxxx: the field of a global section ISD's
// flags (ISD$V_MATCHCTL) and of the top byte of a shareable image's link
// flags.
const (
	matchAlways = 0 // any ident
	matchEqual  = 1 // the same ident
	matchLEQ    = 2 // the same major ident, and a minor one at least as high
)

// ErrNoTransfer is RUN of an image with no transfer address to start it
// at, a shareable image's among them: DCL's NOTFR (testdata/link68's E7).
var ErrNoTransfer = errors.New("no transfer address")

// errBadHeader is an image file whose header isn't an image's: one that
// isn't an image at all (testdata/mp/probe5, a text file run), which
// DCL reports as the image activator's BADHDR.
var errBadHeader = errors.New("an error was discovered in the image header")

// noTransfer is RUN's failure for the image main, from the file fn, with
// no transfer address: ErrNoTransfer for an executable or shareable
// image, a failed activation for a file that is neither.
func noTransfer(main *ICB, fn string) error {
	if main.Type == vmsimage.TypeExecutable || main.Type == vmsimage.TypeShareable {
		return ErrNoTransfer
	}

	return vmserrors.Wrap(vmserrors.CLI_ACTIVATE, errBadHeader, fn)
}

// IdentMismatchError is a shareable image whose global section ident
// doesn't match the one the image naming it was linked with: DCL shows
// the image (ACTIMAGE), its file (IMGNAME), and SS$_SHRIDMISMAT
// (testdata/link68's R5).
type IdentMismatchError struct {
	Image, File string
}

func (e *IdentMismatchError) Error() string {
	return fmt.Sprintf("ident mismatch with shareable image %s (%s)", e.Image, e.File)
}

// Unwrap gives the system's status, SS$_SHRIDMISMAT.
func (e *IdentMismatchError) Unwrap() error {
	return vmserrors.New(vmsdef.Symbols["SS$_SHRIDMISMAT"])
}

// linkedSections are the global section ISDs icb maps the shareable
// image name through, NAME_001 first, in the order of its header.
func linkedSections(icb *ICB, name string) []*ISD {
	var out []*ISD

	prefix := strings.ToUpper(name) + "_"

	for _, isd := range icb.ISDList {
		if isd.Flags&isdGBL != 0 && strings.HasPrefix(strings.ToUpper(isd.Name), prefix) {
			out = append(out, isd)
		}
	}

	return out
}

// linkedSection is the first global section ISD icb maps the shareable
// image name through, nil if none.
func linkedSection(icb *ICB, name string) *ISD {
	if s := linkedSections(icb, name); len(s) > 0 {
		return s[0]
	}

	return nil
}

// identMatches reports whether the shareable image image satisfies the
// global section ISD linked, through which another image was linked
// against it. The rule fits every run of testdata/link68's R5, R6, and
// R9: the match controls must agree, and then the linked-with one
// decides: ALWAYS takes any ident, EQUAL the same ident, and LEQUAL the
// same major ident and a minor one at least as high. (ALWAYS against an
// image with another control wasn't tried: unconfirmed.)
func identMatches(linked *ISD, image *ICB) bool {
	want := int(linked.Flags&vmsimage.ISDMatchMask) >> vmsimage.ISDMatchBit
	if want != image.Match {
		return false
	}

	ident := linked.SectionID

	switch want {
	case matchAlways:
		return true
	case matchEqual:
		return image.Ident == ident
	case matchLEQ:
		return image.Ident>>24 == ident>>24 && image.Ident&0xFFFFFF >= ident&0xFFFFFF
	}

	return false
}

// imageBase is where the image whose file holds data goes, loaded with
// flag: the main image and a shareable image that may go anywhere at
// P0's high-water mark, though a main image whose first section is at
// VPN 0 (a shareable image run, which fails as NOTFR) a page above it,
// so P0's first page stays its guard page; a shareable image the image naming it maps
// through a based section (linked, its first global section ISD, if it
// has one) so that its first section is at that section's address; a
// based shareable image at its own addresses (base 0). An image placed
// on pages another image holds is SS$_VA_IN_USE (what VMS says is
// unconfirmed).
func (p *ImageProcess) imageBase(data []byte, flag uint32, linked *ISD) (uint32, error) {
	hw := p.process().RegionSize[0]

	img, err := vmsimage.ReadImage(data)
	if err != nil {
		return hw, nil //nolint:nilerr // the loader reports a damaged header
	}

	first, ok := firstSection(img.ISDs)

	if flag == icbMain {
		if ok && hw == 0 && first.VPN == 0 {
			return vmsimage.BlockSize, nil
		}

		return hw, nil
	}

	var base uint32

	switch {
	case linked != nil && linked.Flags&isdBASED != 0 && ok:
		base = linked.VPN<<9 - first.VPN<<9
	case img.Type == vmsimage.TypeShareable && img.LinkFlags&ihdPICIMG == 0:
		base = 0
	default:
		return hw, nil
	}

	for _, d := range img.ISDs {
		if d.Flags&vmsimage.ISDFlagGBL != 0 || d.Type() == isdUsrStack {
			continue
		}

		if p.pagesInUse(base+d.VPN<<9, uint32(d.Pages)) {
			return 0, vmserrors.New(vmsdef.Symbols["SS$_VA_IN_USE"])
		}
	}

	return base, nil
}

// firstSection is the first of an image's sections that is its own: not
// the user stack, nor another image's global section.
func firstSection(isds []vmsimage.ISD) (vmsimage.ISD, bool) {
	for _, d := range isds {
		if d.Flags&vmsimage.ISDFlagGBL == 0 && d.Type() != isdUsrStack {
			return d, true
		}
	}

	return vmsimage.ISD{}, false
}

// pagesInUse reports whether any of pages pages from addr is a page of
// an image already loaded.
func (p *ImageProcess) pagesInUse(addr, pages uint32) bool {
	end := addr + pages<<9

	for _, icb := range p.ICBList {
		for _, isd := range icb.ISDList {
			if isd.Flags&isdGBL != 0 || isdType(isd.Flags) == isdUsrStack {
				continue
			}

			from := icb.Base + isd.VPN<<9
			if from < end && addr < from+uint32(isd.Pages)<<9 {
				return true
			}
		}
	}

	return false
}

// placeEmulated gives name, a shareable image govax's shims stand for,
// the pages parent's link placed it on, if it placed it (EXE1's LIBRTL,
// linked with BASE=: LIBRTL_001 and LIBRTL_002, BASED, with no G^
// cells, since the link resolved each reference to an address there).
// Each section is demand-zero pages, the first holding a transfer vector
// into the shims: at each routine's offset its entry mask and a JMP to
// its stub (a JSB routine's JMP alone), at each table's its contents.
// The image is listed as an image of its own, so G^ references resolve
// to the vector too, and its pages are protected as its sections ask.
func (p *ImageProcess) placeEmulated(parent *ICB, name string) error {
	var placed []*ISD

	for _, isd := range linkedSections(parent, name) {
		if isd.Flags&isdBASED != 0 {
			placed = append(placed, isd)
		}
	}

	if len(placed) == 0 || p.findLoadedICB(name) != nil {
		return nil
	}

	icb := p.newICB(name, icbSecondary)
	icb.Base = placed[0].VPN << 9
	icb.File = ""

	for _, s := range placed {
		isd := &ISD{
			Pages: s.Pages, VPN: s.VPN - placed[0].VPN,
			Flags: s.Flags &^ (isdGBL | isdBASED | vmsimage.ISDMatchMask) | isdDZRO, Name: "<NONE>",
		}

		if err := p.createPages(icb, isd); err != nil {
			return err
		}

		icb.ISDList = append(icb.ISDList, isd)
	}

	limit := uint32(placed[0].Pages) << 9

	for _, e := range ShimTable {
		if e.Library != strings.ToUpper(name) || e.Offset >= limit {
			continue
		}

		entry, err := p.vectorEntry(e.Library, e.Offset, e.Data, e.JSB)
		if err != nil {
			return err
		}

		if err := p.storeBytes(icb.Base+e.Offset, entry); err != nil {
			return err
		}
	}

	icb.Flags &^= icbIncomplete

	return nil
}

// createPages makes isd's pages, demand-zero, for icb: created pages
// with paging on, zeroed ones with it off.
func (p *ImageProcess) createPages(icb *ICB, isd *ISD) error {
	addr := icb.Base + isd.VPN<<9
	p.extend(icb, isd)

	if p.mapsImages() {
		return p.process().CreateImagePages(addr, uint32(isd.Pages))
	}

	return p.storeBytes(addr, make([]byte, uint32(isd.Pages)<<9))
}

// vectorEntry is what a placed image's transfer vector holds at offset
// for the shim of library there: data, a table's contents; for a
// routine reached by JSB, JMP @#stub; for one reached by CALLS, the
// routine's entry mask and JMP @#stub+2, eight bytes, as a real vector
// entry is an entry mask and a JMP.
func (p *ImageProcess) vectorEntry(library string, offset uint32, data []byte, jsb bool) ([]byte, error) {
	if data != nil {
		return data, nil
	}

	target, ok := p.m.Symbols.Get(fmt.Sprintf("SHIM$%s_%08X", library, offset))
	if !ok {
		return nil, vmserrors.New(vmserrors.LIB_UNRESOLVED, library)
	}

	// A routine in VAX code (shim code 0) is reached by CALLS if it's a
	// procedure, with an entry mask.
	if !jsb {
		_, entry := p.m.Symbols.EntryAt(target)
		jsb = !entry && !p.isStub(target)
	}

	if jsb {
		return jmpAbsolute(target), nil
	}

	mask, err := p.loadWord(target)
	if err != nil {
		return nil, err
	}

	return append([]byte{byte(mask), byte(mask >> 8)}, jmpAbsolute(target+2)...), nil
}

// isStub reports whether addr is a CALL stub's (ShimStub: an empty
// entry mask, then MOVL I^#code,R0).
func (p *ImageProcess) isStub(addr uint32) bool {
	b, err := p.loadBytes(addr, 4)

	return err == nil && b[0] == 0 && b[1] == 0 && b[2] == 0xD0 && b[3] == 0x8F
}

// jmpAbsolute is JMP @#target's bytes.
func jmpAbsolute(target uint32) []byte {
	return []byte{0x17, 0x9F, byte(target), byte(target >> 8), byte(target >> 16), byte(target >> 24)}
}
