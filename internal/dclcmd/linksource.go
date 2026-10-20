package dclcmd

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/lbr"
	"github.com/tucats/govax/internal/link"
	"github.com/tucats/govax/internal/rms"
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
// Each library is looked for through SYS$LIBRARY on a mounted volume
// first, then in the host library directory the vax.library setting
// names (readLibraryFile, syslib.go), and each shareable image as the
// image activator finds it (openSharedImage). A file that isn't there is
// skipped; a file that's there but can't be read is an error. /NOSYSLIB
// skips the libraries.
//
// The image a link writes is the same whichever source defined a routine,
// since it refers to the routine by its image and offset: govax's RUN
// resolves that against the real image if it's there, and against the
// shim registered for that offset otherwise.

// GovaxSymbols returns govax's own symbol source. The shareable images
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
func GovaxSymbols(shims []corevms.ShimLink) *link.TableSource {
	src := &link.TableSource{Symbols: map[string]link.Definition{}, Images: map[string]link.SharedImage{}}

	for name, i := range vmsdef.SharedImages {
		src.Images[name] = link.SharedImage{
			Name: name, Pages: i.Pages, MajorID: i.MajorID, MinorID: i.MinorID, Match: link.Match(i.Match),
			Symbols: i.Symbols, Psects: i.Psects, Sections: i.Sections,
		}

		for _, sec := range vmsdef.SharedImageSections[name] {
			img := src.Images[name]
			img.Layout = append(img.Layout, link.SharedSection{Offset: sec.Offset, Pages: sec.Pages, Flags: sec.Flags})
			src.Images[name] = img
		}
	}

	for name, s := range vmsdef.ImageSymbols {
		src.Symbols[name] = link.Definition{Image: s.Image, Value: s.Value, Order: s.Order}
	}

	for name, v := range vmsdef.LibrarySymbols {
		if _, ok := src.Symbols[name]; !ok {
			src.Symbols[name] = link.Definition{Value: v}
		}
	}

	for _, e := range vmsdef.P1VectorTable {
		src.Symbols[e.Name] = link.Definition{Value: e.Addr}
	}

	for _, e := range shims {
		if _, ok := src.Symbols[e.Name]; !ok {
			src.Symbols[e.Name] = link.Definition{Image: e.Library, Value: e.Offset}
		}
	}

	return src
}

// LinkSources returns LINK's symbol sources: the system libraries, unless
// sysLib is false, then govax's own tables.
func (c *Context) LinkSources(sysLib bool) ([]link.SymbolSource, error) {
	govax := GovaxSymbols(c.ShimLinks)

	if !sysLib {
		return []link.SymbolSource{govax}, nil
	}

	var sources []link.SymbolSource

	data, loc, err := c.readLibraryFile("SYS$LIBRARY", "IMAGELIB.OLB")
	if err != nil {
		return nil, err
	}

	if data != nil {
		lib, err := lbr.Open(data)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", loc.Name, err)
		}

		sources = append(sources, c.imageLibrary(lib, loc, govax))
	}

	data, loc, err = c.readLibraryFile("SYS$LIBRARY", "STARLET.OLB")
	if err != nil {
		return nil, err
	}

	if data != nil {
		lib, err := lbr.Open(data)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", loc.Name, err)
		}

		sources = append(sources, &link.ObjectLibrarySource{File: loc.Name, Library: lib, System: true})
	}

	return append(sources, govax), nil
}

// openSharedImage returns the symbols of the shareable image image: its
// global symbol table, read from the file the image activator's rules
// find (ImageLocator), or else govax's own tables for it. LINK reads an
// image's file even when govax's shims stand for it at RUN, since only
// the file gives its symbols as real LINK sees them.
func (c *Context) openSharedImage(image string, govax *link.TableSource) (link.SymbolSource, error) {
	found, err := c.ImageLocator().Find(image)

	var notFound *rms.ImageNotFoundError

	switch {
	case errors.As(err, &notFound):
		return &shimImage{image: image, file: image + ".EXE", govax: govax}, nil
	case err != nil:
		return nil, err
	}

	loc := found.Location

	var data []byte

	if loc.Host {
		data, err = os.ReadFile(loc.Name)
	} else {
		data, loc, err = c.ContainerSession.ReadRawFile(loc)
	}

	if err != nil {
		return nil, fmt.Errorf("%s: %w", loc.Name, err)
	}

	return sharedImageSource(data, loc.Name, image)
}

// imageLibrary is the shareable image library lib, read from loc, as a
// symbol source: the images it names are found by openLibraryImage.
func (c *Context) imageLibrary(lib *lbr.Library, loc rms.FileLocation, govax *link.TableSource) *link.ImageLibrarySource {
	return &link.ImageLibrarySource{
		File:    loc.Name,
		Library: lib,
		Open:    func(image string) (link.SymbolSource, error) { return c.openLibraryImage(image, loc, govax) },
	}
}

// openLibraryImage returns the symbols of the shareable image image, which
// the shareable image library at lib names. Real LINK looks for the image
// in the library's device and directory, then in SYS$LIBRARY (the Linker
// manual, 6.3.3.2); each is RMS's parse of the image's name with that
// default, so a logical name of the image's name leads to its file
// first, as testdata/link68's MAIN1L found SHR1. govax then looks where
// the image activator would (openSharedImage): SYS$SHARE, the host, and
// last its own tables.
func (c *Context) openLibraryImage(image string, lib rms.FileLocation, govax *link.TableSource) (link.SymbolSource, error) {
	s := c.ContainerSession

	if lib.Host {
		dir := filepath.Dir(lib.Name)

		for _, name := range []string{image + ".EXE", strings.ToLower(image) + ".exe"} {
			path := filepath.Join(dir, name)

			data, err := os.ReadFile(path)
			if err == nil {
				return sharedImageSource(data, path, image)
			}

			if !errors.Is(err, fs.ErrNotExist) {
				return nil, fmt.Errorf("%s: %w", path, err)
			}
		}
	} else if s != nil {
		for _, def := range []string{libraryDirectory(lib.Name) + ".EXE", "SYS$LIBRARY:.EXE"} {
			found, _, err := s.FindImageFile(image, def)
			if err != nil {
				continue
			}

			data, loc, err := s.ReadRawFile(found)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", found.Name, err)
			}

			return sharedImageSource(data, loc.Name, image)
		}
	}

	return c.openSharedImage(image, govax)
}

// libraryDirectory is a volume file's device and directory: what its
// name has before the file name ("DUA1:[000000]" of
// "DUA1:[000000]MYSHR.OLB;1").
func libraryDirectory(name string) string {
	return name[:strings.LastIndexAny(name, ":]>")+1]
}

// ImageLocator is how LINK finds a shareable image's file
// (rms.ImageLocator), as RUN does but for govax's shims: a logical name
// of the image's name, SYS$SHARE, then the host (SET SHARE_PREFIX's
// prefix, the host library directory, the current directory).
func (c *Context) ImageLocator() rms.ImageLocator {
	return rms.ImageLocator{Session: c.ContainerSession, Prefix: c.SharePrefix, Library: c.HostLibraryDir()}
}

// usedImages is the source of the shareable images that a shareable image
// the link uses uses itself (testdata/link68's MAIN2, linked against
// SHR2, which uses SHR1): each is read from its file (openSharedImage),
// which real LINK opens as the image activator would.
func (c *Context) usedImages() *link.ImageFileSource {
	govax := GovaxSymbols(c.ShimLinks)

	return &link.ImageFileSource{Open: func(image string) (link.SymbolSource, error) {
		return c.openSharedImage(image, govax)
	}}
}

// sharedImageSource reads the shareable image image's global symbol
// table from its file's bytes, read from file.
func sharedImageSource(data []byte, file, image string) (link.SymbolSource, error) {
	src, got, err := link.ReadShareableImage(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}

	if got != image {
		return nil, fmt.Errorf("%s is shareable image %s, not %s", file, got, image)
	}

	src.File = file

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
