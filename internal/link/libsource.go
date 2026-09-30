package link

import (
	"encoding/binary"
	"fmt"

	"github.com/tucats/govax/internal/lbr"
	"github.com/tucats/govax/internal/obj"
)

// This file is docs/PHASE-30.md's symbol sources made from VMS files:
//
//   - a shareable image's own global symbol table (ReadShareableImage),
//     read from the image file, as real LINK reads SYS$SHARE:LIBRTL.EXE;
//   - a shareable image symbol table library (ImageLibrarySource), such as
//     IMAGELIB.OLB, which says which shareable image defines each symbol;
//   - an object library (ObjectLibrarySource), such as STARLET.OLB, whose
//     module that defines a symbol is added to the link.

// ReadShareableImage reads a shareable image's global symbol table (GST)
// from the image file's bytes, and returns it as a source, with the
// image's name. The source defines each of the image's universal symbols:
// a relocatable one as the image plus its offset there, an absolute one
// as its value. It describes the image as a global section ISD will: its
// shareable section's page count, and its global section ident and match
// control (GSMATCH), from the image header.
func ReadShareableImage(data []byte) (*TableSource, string, error) {
	h, err := readImageHeader(data)
	if err != nil {
		return nil, "", err
	}

	if h.gstVBN == 0 {
		return nil, "", fmt.Errorf("link: shareable image %s has no global symbol table", h.name)
	}

	records, err := readVariableRecords(data, h.gstVBN, h.gstRecords)
	if err != nil {
		return nil, "", fmt.Errorf("link: shareable image %s's global symbol table: %w", h.name, err)
	}

	m, err := obj.Decode(records)
	if err != nil {
		return nil, "", fmt.Errorf("link: shareable image %s's global symbol table: %w", h.name, err)
	}

	name := h.name
	if name == "" {
		name = m.Name()
	}

	src := &TableSource{
		Symbols: map[string]Definition{},
		Images: map[string]SharedImage{name: {
			Name:    name,
			Pages:   h.sharedPages,
			MajorID: uint8(h.ident >> 24),
			MinorID: h.ident & 0xFFFFFF,
			Match:   Match(h.matchCtl),
		}},
	}

	for _, s := range m.Symbols() {
		if !s.Defined() {
			continue
		}

		d := Definition{Value: s.Value}
		if s.Flags&obj.SymREL != 0 {
			d.Image = name
		}

		src.Symbols[s.Name] = d
	}

	img := src.Images[name]
	img.Symbols, img.Psects, img.Sections = len(src.Symbols), len(m.Psects()), h.sections
	src.Images[name] = img

	return src, name, nil
}

// imageHeader is what ReadShareableImage needs from an image header.
type imageHeader struct {
	name        string
	ident       uint32 // IHD$L_IDENT: the global section ident
	matchCtl    uint32 // IHD$V_MATCHCTL
	sharedPages uint32 // the first shareable section's pages
	sections    int    // its image sections, other than global ones and the stack
	gstVBN      uint32
	gstRecords  int
}

// Image types (IHD$B_IMGTYPE).
const ihdTypeShareable = 2 // IHD$K_LIM

// readImageHeader reads a shareable image's header (block 1; image.go has
// the layout). Its ISDs run to a zero size word, or to a size of ^XFFFF,
// which continues them at the next header block.
func readImageHeader(data []byte) (imageHeader, error) {
	if len(data) < blockSize {
		return imageHeader{}, fmt.Errorf("link: an image file is at least a block")
	}

	le := binary.LittleEndian

	var h imageHeader

	isdOff := int(le.Uint16(data[0x00:]))
	symOff := int(le.Uint16(data[0x04:]))
	idOff := int(le.Uint16(data[0x06:]))
	hdrBlocks := max(int(data[0x10]), 1)

	if data[0x11] != ihdTypeShareable {
		return h, fmt.Errorf("link: not a shareable image (image type %d)", data[0x11])
	}

	if isdOff >= blockSize || symOff+ihsLength > blockSize || idOff+ihiLength > blockSize || hdrBlocks*blockSize > len(data) {
		return h, fmt.Errorf("link: the image header is damaged")
	}

	h.matchCtl = le.Uint32(data[0x20:]) >> 24 & 7
	h.ident = le.Uint32(data[0x24:])
	h.gstVBN = le.Uint32(data[symOff+4:])
	h.gstRecords = int(le.Uint16(data[symOff+10:]))
	h.name = counted(data[idOff : idOff+40])

	for blk, p := 0, isdOff; blk < hdrBlocks; {
		b := data[blk*blockSize : (blk+1)*blockSize]
		if p+2 > blockSize {
			return h, fmt.Errorf("link: the image header's section descriptors are damaged")
		}

		size := int(le.Uint16(b[p:]))

		switch {
		case size == 0:
			return h, nil
		case size == 0xFFFF:
			blk, p = blk+1, 0

			continue
		case size < isdDemandZeroSize || p+size > blockSize:
			return h, fmt.Errorf("link: the image header's section descriptors are damaged")
		}

		flags := le.Uint32(b[p+8:])
		if h.sharedPages == 0 && flags&0xFF000000 == isdTypeSharedPIC {
			h.sharedPages = uint32(le.Uint16(b[p+2:]))
		}

		if flags&isdGBL == 0 && flags&0xFF000000 != isdTypeUserStack {
			h.sections++
		}

		p += size
	}

	return h, nil
}

// readVariableRecords reads n records in ODS-2's variable-length layout (a
// length word, the record, and a pad byte to a word boundary) starting at
// block vbn.
func readVariableRecords(data []byte, vbn uint32, n int) ([][]byte, error) {
	if vbn == 0 || int64(vbn-1)*blockSize >= int64(len(data)) {
		return nil, fmt.Errorf("block %d is outside the file", vbn)
	}

	p := int(vbn-1) * blockSize
	records := make([][]byte, 0, n)

	for range n {
		if p+2 > len(data) {
			return nil, fmt.Errorf("the file ends after %d of %d records", len(records), n)
		}

		size := int(binary.LittleEndian.Uint16(data[p:]))
		p += 2

		if p+size > len(data) {
			return nil, fmt.Errorf("record %d runs past the end of the file", len(records)+1)
		}

		records = append(records, data[p:p+size])
		p += (size + 1) &^ 1
	}

	return records, nil
}

// counted reads a counted string from a fixed-length field.
func counted(b []byte) string {
	n := min(int(b[0]), len(b)-1)

	return string(b[1 : 1+n])
}

// ImageLibrarySource is a shareable image symbol table library, such as
// IMAGELIB.OLB, as a symbol source. The library's global symbol index
// says which shareable image defines each symbol, but its modules hold no
// offsets: real LINK reads those from the image itself, and so does this
// source, through Open.
type ImageLibrarySource struct {
	// File names the library, for messages.
	File    string
	Library *lbr.Library
	// Open returns a shareable image's own symbols, given its name: its
	// global symbol table, from the image file (ReadShareableImage), or
	// another source that knows its offsets.
	Open func(image string) (SymbolSource, error)

	opened map[string]SymbolSource
}

// Lookup implements SymbolSource.
func (s *ImageLibrarySource) Lookup(name string) (Definition, bool, error) {
	image, ok := s.imageOf(name)
	if !ok {
		return Definition{}, false, nil
	}

	src, err := s.open(image)
	if err != nil {
		return Definition{}, false, err
	}

	d, ok, err := src.Lookup(name)
	if err != nil {
		return Definition{}, false, err
	}

	if !ok {
		return Definition{}, false, fmt.Errorf("%s says shareable image %s defines %s, but its symbols don't include it", s.File, image, name)
	}

	return d, true, nil
}

// Image implements SymbolSource: what the image's own symbols say about it,
// once a symbol in it has been looked up.
func (s *ImageLibrarySource) Image(name string) (SharedImage, bool) {
	if src, ok := s.opened[name]; ok {
		return src.Image(name)
	}

	return SharedImage{}, false
}

// Files implements FileCounter: the library, and the files of the images
// it has opened.
func (s *ImageLibrarySource) Files() int {
	n := 1

	for _, src := range s.opened {
		if fc, ok := src.(FileCounter); ok {
			n += fc.Files()
		}
	}

	return n
}

// imageOf returns the shareable image the library says defines name.
func (s *ImageLibrarySource) imageOf(name string) (string, bool) {
	if len(s.Library.Indexes) < 2 {
		return "", false
	}

	rfa, ok := s.Library.Indexes[1].Lookup(name)
	if !ok {
		return "", false
	}

	return s.Library.ModuleName(rfa)
}

// open returns image's symbols, opening them the first time.
func (s *ImageLibrarySource) open(image string) (SymbolSource, error) {
	if src, ok := s.opened[image]; ok {
		return src, nil
	}

	if s.Open == nil {
		return nil, fmt.Errorf("%s names shareable image %s, but there's no way to read it", s.File, image)
	}

	src, err := s.Open(image)
	if err != nil {
		return nil, fmt.Errorf("shareable image %s: %w", image, err)
	}

	if s.opened == nil {
		s.opened = map[string]SymbolSource{}
	}

	s.opened[image] = src

	return src, nil
}

// ObjectLibrarySource is an object library, such as STARLET.OLB, as a
// symbol source: the module that defines a symbol, which the link adds.
type ObjectLibrarySource struct {
	// File names the library, for messages and each module's Input.
	File    string
	Library *lbr.Library
	// System marks a system library, such as STARLET.OLB, whose modules a
	// default map leaves out.
	System bool

	modules map[lbr.RFA]*Input
}

// Lookup implements SymbolSource.
func (s *ObjectLibrarySource) Lookup(name string) (Definition, bool, error) {
	if len(s.Library.Indexes) < 2 {
		return Definition{}, false, nil
	}

	rfa, ok := s.Library.Indexes[1].Lookup(name)
	if !ok {
		return Definition{}, false, nil
	}

	in, err := s.module(rfa)
	if err != nil {
		return Definition{}, false, err
	}

	return Definition{Module: in}, true, nil
}

// Include returns the library's module name, for /INCLUDE=: the link adds
// all of it, whether or not anything refers to its symbols. A later lookup
// of one of its symbols returns the same Input.
func (s *ObjectLibrarySource) Include(name string) (*Input, error) {
	rfa, ok := s.Library.Lookup(name)
	if !ok {
		return nil, fmt.Errorf("%s has no module %s", s.File, name)
	}

	in, err := s.module(rfa)
	if err != nil {
		return nil, err
	}

	in.Selective = false

	return in, nil
}

// module reads the module at rfa, once.
func (s *ObjectLibrarySource) module(rfa lbr.RFA) (*Input, error) {
	if in, ok := s.modules[rfa]; ok {
		return in, nil
	}

	module, _ := s.Library.ModuleName(rfa)

	lm, err := s.Library.Module(rfa)
	if err != nil {
		return nil, fmt.Errorf("%s, module %s: %w", s.File, module, err)
	}

	m, err := obj.Decode(lm.Records)
	if err != nil {
		return nil, fmt.Errorf("%s, module %s: %w", s.File, module, err)
	}

	// The module's file is the library's, as real LINK's map and messages
	// name it.
	in := &Input{File: s.File, Module: m, Selective: lm.Header.SelectiveSearch(), System: s.System}

	if s.modules == nil {
		s.modules = map[lbr.RFA]*Input{}
	}

	s.modules[rfa] = in

	return in, nil
}

// Files implements FileCounter.
func (*ObjectLibrarySource) Files() int { return 1 }

// Image implements SymbolSource: an object library describes no
// shareable images.
func (*ObjectLibrarySource) Image(string) (SharedImage, bool) { return SharedImage{}, false }
