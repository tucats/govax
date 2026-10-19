package machine

import (
	"encoding/binary"

	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/vmserrors"
	"github.com/tucats/govax/internal/vmsimage"
)

// Image activation by mapping (docs/PHASE-66 - mapped image
// activation.md, subtask 3).
//
// With paging on, an image is mapped rather than copied, as VMS's image
// activator maps it (the book's 21.1.1.3): the image file is held
// open as its sections' page source (OpenImage), the header is decoded
// from the file's contents in Go and never written to P0, and each ISD
// becomes a mapping (corevms's imagesec.go): a private section of the
// file for each process-private ISD, demand-zero pages for each
// demand-zero one. No page is read until it's touched; the fixups
// (imageFixup), which run next, fault in the pages they read and write,
// in the image's own process. setImageProtection then gives each
// section its protection, whatever form its PTEs are in.
//
// A shareable image the main image names (subtask 4) is mapped the same
// way, at P0's high-water mark after the main image, privately to the
// process, as the book's 21.1.1.4 maps a shareable image that isn't
// installed: a host file found by findImage, its sections starting at
// its own VPN 0. Installing one as shared global sections is future
// work.

// mapsImages reports whether images are mapped rather than copied: with
// paging on and memory mapping enabled, the main image and the shareable
// images it loads alike.
func (p *ImageProcess) mapsImages() bool {
	m := p.m

	return m.CPU != nil && m.Mem != nil && m.Mem.Paging() != nil &&
		m.CPU.PR(registers.MAPEN) != 0 && p.process() != nil
}

// imageMap is imageLoad for an image that's mapped: it opens fn, decodes
// its header, maps its sections at P0's high-water mark, and finishes as
// the copying loader does (finishLoad). The image file stays open while
// any of its sections is mapped.
func (p *ImageProcess) imageMap(fn string, flag uint32) (*ICB, error) {
	data, src, err := p.m.OpenImage(fn, flag == icbMain)
	if err != nil {
		return nil, err
	}

	env := p.process()

	// The sections hold the file now; the activator's own hold goes,
	// whatever happens.
	defer func() { _ = env.ReleaseSource(src) }()

	if len(data) < vmsimage.BlockSize {
		return nil, vmserrors.New(vmserrors.RMS_SHORTHEADER, fn)
	}

	img, err := vmsimage.ReadImage(data)
	if err != nil {
		return nil, err
	}

	icb := p.newICB(fn, flag)
	icb.Mapped = true

	// The transfer addresses: four longwords at the header's activation
	// offset (as the copying loader reads them from its copy in P0).
	header := img.Header
	at := uint32(binary.LittleEndian.Uint16(header[vmsimage.IHDActivOffset:]))

	for n := uint32(0); n < 4; n++ {
		var v uint32

		if off := at + n*4; off+4 <= uint32(len(header)) {
			v = binary.LittleEndian.Uint32(header[off:])
		}

		p.setTransfer(icb, n, v)
	}

	for _, d := range img.ISDs {
		name := d.GlobalName
		if name == "" {
			name = "<NONE>"
		}

		icb.ISDList = append(icb.ISDList, &ISD{
			Pages: d.Pages, VPN: d.VPN, PFC: d.PFC, Flags: d.Flags, VBN: d.VBN,
			SectionID: d.GlobalIdent, Name: name,
		})
	}

	for _, isd := range icb.ISDList {
		if isd.Flags&isdFIXUPVEC != 0 {
			icb.FixupISD = isd
		}

		if isdType(isd.Flags) == isdUsrStack || isd.Flags&isdGBL != 0 {
			continue
		}

		addr := icb.Base + isd.VPN<<9
		p.extend(icb, isd)

		if isd.Flags&isdDZRO != 0 {
			err = env.CreateImagePages(addr, uint32(isd.Pages))
		} else {
			err = env.MapImageSection(src, isd.VBN, uint32(isd.Pages), addr, isd.PFC, isd.Flags&isdWRT != 0)
		}

		if err != nil {
			return nil, err
		}
	}

	return icb, p.finishLoad(icb, data)
}
