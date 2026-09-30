package console

import (
	"fmt"

	"github.com/tucats/govax/internal/lbr"
	"github.com/tucats/govax/internal/link"
	"github.com/tucats/govax/internal/vmsdef"
)

// This file is where LINK's symbols come from (docs/PHASE-30.md, "Where
// external symbols come from"). The sources are searched in real LINK's
// order:
//
//  1. IMAGELIB.OLB, which says which shareable image defines a routine.
//     Its offset comes from the image's own global symbol table
//     (SYS$SHARE:LIBRTL.EXE, say), or, with no such file, from govax's
//     own tables.
//  2. STARLET.OLB, whose module that defines a symbol is added to the
//     link: the system services' and status codes' definitions, say.
//  3. govax's own tables, which need no VMS files: the shareable images'
//     global symbol tables captured in vmsdef.SharedImages and
//     vmsdef.ImageSymbols, and STARLET.OLB's definition modules' symbols
//     captured in vmsdef.LibrarySymbols (docs/PHASE-31.md); the routines
//     the shims stand for (shimTable) in images not captured; and the
//     system services at their addresses in the P1 vector.
//
// Each VMS file is looked for through its logical name (SYS$LIBRARY or
// SYS$SHARE) on a mounted volume first, then in the host library
// directory the vax.library setting names (readLibraryFile, syslib.go). A
// file that isn't there is skipped; a file that's there but can't be read
// is an error. /NOSYSLIB skips the libraries.
//
// The image a link writes is the same whichever source defined a routine,
// since it refers to the routine by its image and offset: govax's RUN
// resolves that against the real image if it's there, and against the
// shim registered for that offset otherwise.

// govaxSymbols returns govax's own symbol source. The shareable images
// captured in vmsdef.SharedImages are described as their headers describe
// them. An image the shims stand for that isn't captured (DECC$SHR, say)
// isn't known, so an image records it by name only, with a match control
// that accepts any ident; govax's RUN doesn't check them. A shim for a
// captured image adds nothing, since the image's own symbols include the
// routine; TestShimOffsetsMatchCapturedImages checks that the two agree.
//
// A shareable image's symbol comes before a library's of the same name,
// as IMAGELIB.OLB is searched before STARLET.OLB. The P1 vector's
// addresses come last and take precedence: they are where govax's RUN
// puts the system services, and they match STARLET's SYS$P1_VECTOR for
// every service it has (TestLibrarySymbolsMatchP1Vector).
func govaxSymbols() *link.TableSource {
	src := &link.TableSource{Symbols: map[string]link.Definition{}, Images: map[string]link.SharedImage{}}

	for name, i := range vmsdef.SharedImages {
		src.Images[name] = link.SharedImage{
			Name: name, Pages: i.Pages, MajorID: i.MajorID, MinorID: i.MinorID, Match: link.Match(i.Match),
			Symbols: i.Symbols, Psects: i.Psects, Sections: i.Sections,
		}
	}

	for name, s := range vmsdef.ImageSymbols {
		src.Symbols[name] = link.Definition{Image: s.Image, Value: s.Value}
	}

	for name, v := range vmsdef.LibrarySymbols {
		if _, ok := src.Symbols[name]; !ok {
			src.Symbols[name] = link.Definition{Value: v}
		}
	}

	for _, e := range vmsdef.P1VectorTable {
		src.Symbols[e.Name] = link.Definition{Value: e.Addr}
	}

	for _, e := range shimTable {
		if _, ok := src.Symbols[e.name]; !ok {
			src.Symbols[e.name] = link.Definition{Image: e.library, Value: e.offset}
		}
	}

	return src
}

// linkSources returns LINK's symbol sources: the system libraries, unless
// sysLib is false, then govax's own tables.
func (c *Console) linkSources(sysLib bool) ([]link.SymbolSource, error) {
	govax := govaxSymbols()

	if !sysLib {
		return []link.SymbolSource{govax}, nil
	}

	var sources []link.SymbolSource

	data, name, err := c.readLibraryFile("SYS$LIBRARY", "IMAGELIB.OLB")
	if err != nil {
		return nil, err
	}

	if data != nil {
		lib, err := lbr.Open(data)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}

		sources = append(sources, &link.ImageLibrarySource{
			File:    name,
			Library: lib,
			Open:    func(image string) (link.SymbolSource, error) { return c.openSharedImage(image, govax) },
		})
	}

	data, name, err = c.readLibraryFile("SYS$LIBRARY", "STARLET.OLB")
	if err != nil {
		return nil, err
	}

	if data != nil {
		lib, err := lbr.Open(data)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}

		sources = append(sources, &link.ObjectLibrarySource{File: name, Library: lib, System: true})
	}

	return append(sources, govax), nil
}

// openSharedImage returns the symbols of the shareable image IMAGELIB
// names: its global symbol table, from SYS$SHARE:<image>.EXE or the host
// library directory, or else govax's own tables for it.
func (c *Console) openSharedImage(image string, govax *link.TableSource) (link.SymbolSource, error) {
	file := image + ".EXE"

	data, name, err := c.readLibraryFile("SYS$SHARE", file)
	if err != nil {
		return nil, err
	}

	if data == nil {
		return &shimImage{image: image, file: file, govax: govax}, nil
	}

	src, got, err := link.ReadShareableImage(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}

	if got != image {
		return nil, fmt.Errorf("%s is shareable image %s, not %s", name, got, image)
	}

	src.File = name

	return src, nil
}

// shimImage is a shareable image whose file LINK can't find: govax's own
// tables give the offsets of its routines, captured from the image or
// those the shims stand for, and any other routine in it is an error.
type shimImage struct {
	image, file string
	govax       *link.TableSource
}

// Lookup implements link.SymbolSource.
func (s *shimImage) Lookup(name string) (link.Definition, bool, error) {
	if d, ok, _ := s.govax.Lookup(name); ok && d.Image == s.image {
		return d, true, nil
	}

	return link.Definition{}, false, fmt.Errorf("it's in shareable image %s, but there's no %s to read, and govax has no record of it", s.image, s.file)
}

// Image implements link.SymbolSource.
func (s *shimImage) Image(name string) (link.SharedImage, bool) { return s.govax.Image(name) }
