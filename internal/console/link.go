package console

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/tucats/govax/internal/lbr"
	"github.com/tucats/govax/internal/link"
	"github.com/tucats/govax/internal/obj"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmserrors"
)

// This file implements docs/PHASE-30.md's LINK command:
//
//	LINK file[,file...][/HOST] [/EXECUTABLE[=image] | /NOEXECUTABLE] [/[NO]TRACEBACK] [/[NO]SYSLIB]
//	     [/MAP[=map] [/BRIEF] | /NOMAP]
//
// where each file is an object, or has its own qualifier:
//
//	file/LIBRARY[/INCLUDE=(module,...)]   a library to search (.OLB)
//	file/INCLUDE=(module,...)             modules of a library to add
//	file/SELECTIVE_SEARCH                 an object searched selectively
//	file/OPTIONS                          an options file (.OPT)
//
// An options file (link.ParseOptions) names more input files, which may
// also be shareable images (file/SHAREABLE, .EXE), and options: STACK=,
// IDENTIFICATION=, NAME=, and SYMBOL=.
//
// LINK reads each object module, links them with internal/link, and writes
// the executable image, and a link map if asked. Every file can be a host
// file or a file on a mounted ODS-2 volume, by the same rules as MACRO's
// source and object (rms.Session.Locate). A file named without a type gets
// its kind's, and a later file's bare name is found beside the one before
// it; a file an options file names, beside the options file. The image and
// map are named after the first file, with the types EXE and MAP, unless
// /EXECUTABLE or /MAP names them.
//
// Symbols the objects don't define come from the shareable images the
// options files name, then the libraries named, in order, then the system
// libraries and govax's own tables (linksource.go).

// LinkOptions is one LINK command.
type LinkOptions struct {
	// Objects are object file names, and Files input files with their
	// qualifiers, which follow them. Host is an explicit /HOST on all of
	// them.
	Objects []string
	Files   []link.InputFile
	Host    bool

	// Executable is the image file name from /EXECUTABLE=; "" means the
	// first file's name with the type EXE. NoExecutable is
	// /NOEXECUTABLE: link, and report errors, but write nothing.
	Executable   string
	NoExecutable bool

	// NoTraceback is /NOTRACEBACK: the image doesn't start through
	// SYS$IMGSTA.
	NoTraceback bool

	// NoSysLib is /NOSYSLIB: don't search IMAGELIB.OLB and STARLET.OLB
	// (linksource.go).
	NoSysLib bool

	// Map is /MAP: write a link map, to MapFile, or by default to a file
	// named after the first file with the type MAP. Brief is /BRIEF:
	// only the object modules and the image synopsis.
	Map     bool
	MapFile string
	Brief   bool
}

// linkInputs is what LINK's input files give the link.
type linkInputs struct {
	modules []link.Input
	// shared are the shareable images options files name, and libraries
	// the libraries named, each a symbol source.
	shared, libraries []link.SymbolSource
	// options gathers the options files' options; a later one's value
	// replaces an earlier one's.
	options link.OptionsFile
	// first is the first file, which names the image and map by default,
	// and prev the one before the file being read.
	first, prev rms.FileLocation
	seen        bool
}

// Link links object modules into an executable image.
func (c *Console) Link(opts LinkOptions) error {
	s := c.ContainerSession

	files := make([]link.InputFile, 0, len(opts.Objects)+len(opts.Files))
	for _, name := range opts.Objects {
		files = append(files, link.InputFile{Name: name})
	}

	files = append(files, opts.Files...)

	if len(files) == 0 {
		return vmserrors.New(vmserrors.CLI_NEEDFILENAME, "LINK")
	}

	in := &linkInputs{}

	for _, f := range files {
		if err := c.linkInput(in, f, opts.Host, true); err != nil {
			return err
		}
	}

	if len(in.modules) == 0 {
		return vmserrors.Wrap(vmserrors.CLI_LINKING, fmt.Errorf("no object modules to link"), in.first.Name)
	}

	exe, err := outputLocation(s, opts.Executable, in.first, "EXE")
	if err != nil {
		return fileFailure(err, opts.Executable)
	}

	system, err := c.linkSources(!opts.NoSysLib)
	if err != nil {
		return vmserrors.Wrap(vmserrors.CLI_LINKING, err, exe.Name)
	}

	sources := append(append(in.shared, in.libraries...), system...)

	name := imageName(exe)
	if in.options.Name != "" {
		name = in.options.Name
	}

	img, err := link.Link(in.modules, link.Options{
		ImageName:  name,
		LinkerID:   linkerID(),
		Traceback:  !opts.NoTraceback,
		Sources:    sources,
		StackPages: in.options.Stack,
		Ident:      in.options.Ident,
		Symbols:    in.options.Symbols,
	})
	if err != nil {
		return vmserrors.Wrap(vmserrors.CLI_LINKING, err, exe.Name)
	}

	var imageFile string

	if !opts.NoExecutable {
		blocks := make([][]byte, 0, len(img.Bytes)/512)
		for i := 0; i < len(img.Bytes); i += 512 {
			blocks = append(blocks, img.Bytes[i:i+512])
		}

		created, err := s.CreateRecordFile(exe, rms.ImageBlocks, blocks)
		if err != nil {
			return vmserrors.Wrap(vmserrors.CLI_LINKING, err, exe.Name)
		}

		imageFile = created.Name
	}

	if !opts.Map {
		return nil
	}

	mapLoc, err := outputLocation(s, opts.MapFile, in.first, "MAP")
	if err != nil {
		return fileFailure(err, opts.MapFile)
	}

	// The map records its own name as it's about to be created: on a
	// volume, without the version it gets.
	lines := img.Map(link.MapOptions{
		ImageFile: imageFile,
		ImageText: opts.Executable,
		MapFile:   mapLoc.Name,
		Brief:     opts.Brief,
	})

	records := make([][]byte, len(lines))
	for i, line := range lines {
		records[i] = []byte(line)
	}

	if _, err := s.CreateRecordFile(mapLoc, rms.TextRecords, records); err != nil {
		return vmserrors.Wrap(vmserrors.CLI_LINKING, err, mapLoc.Name)
	}

	return nil
}

// imageName is the name an image's header records: its file's name,
// without directory, type, or version, in uppercase, as VMS names are.
func imageName(exe rms.FileLocation) string {
	name := exe.Name
	if exe.Host {
		name = filepath.Base(name)
	} else {
		name, _ = splitVersion(name)
		name = vmsNameType(name)
	}

	if i := strings.IndexByte(name, '.'); i >= 0 {
		name = name[:i]
	}

	return strings.ToUpper(name)
}

// linkerID names govax's LINK in an image header, as real LINK records
// its version ("V11-39"): at most 15 characters.
func linkerID() string {
	id := "govax"
	if BuildVersion != "" {
		id += " V" + BuildVersion
	}

	return id[:min(len(id), 15)]
}

// linkInput reads one input file into in. fromCommand says the command
// line named it, rather than an options file.
func (c *Console) linkInput(in *linkInputs, f link.InputFile, host, fromCommand bool) error {
	s := c.ContainerSession

	typ := "OBJ"

	switch {
	case f.Options && !fromCommand:
		return vmserrors.Wrap(vmserrors.CLI_LINKING, fmt.Errorf("an options file can't name another (%s)", f.Name), f.Name)
	case f.Shareable && fromCommand:
		return vmserrors.Wrap(vmserrors.CLI_LINKING, fmt.Errorf("%s/SHAREABLE: shareable images are named in an options file", f.Name), f.Name)
	case f.Options:
		typ = "OPT"
	case f.Shareable:
		typ = "EXE"
	case f.Library || len(f.Include) > 0:
		typ = "OLB"
	}

	var (
		loc rms.FileLocation
		err error
	)

	if in.seen {
		loc, err = s.LocateRelated(f.Name, host, in.prev)
	} else {
		loc, err = s.Locate(f.Name, host)
	}

	if err != nil {
		return fileFailure(err, f.Name)
	}

	loc = withDefaultType(loc, typ)

	failed := func(err error, name string) error {
		return vmserrors.Wrap(vmserrors.CLI_LINKING, fmt.Errorf("%s: %w", name, err), name)
	}

	var found rms.FileLocation

	switch typ {
	case "OPT":
		var records [][]byte

		if records, found, err = s.ReadRecordFile(loc, rms.TextRecords); err != nil {
			return fileFailure(err, loc.Name)
		}

		lines := make([]string, len(records))
		for i, r := range records {
			lines[i] = string(r)
		}

		o, err := link.ParseOptions(lines)
		if err != nil {
			return failed(err, found.Name)
		}

		in.noteFile(found)
		in.mergeOptions(o)

		for _, g := range o.Files {
			if err := c.linkInput(in, g, host, false); err != nil {
				return err
			}
		}

		in.prev = found

		return nil

	case "EXE":
		var data []byte

		if data, found, err = s.ReadRawFile(loc); err != nil {
			return fileFailure(err, loc.Name)
		}

		src, _, err := link.ReadShareableImage(data)
		if err != nil {
			return failed(err, found.Name)
		}

		src.File = found.Name
		in.shared = append(in.shared, src)

	case "OLB":
		if found, err = c.linkLibrary(in, loc, f); err != nil {
			return err
		}

	default:
		var records [][]byte

		if records, found, err = s.ReadRecordFile(loc, rms.VariableRecords); err != nil {
			return fileFailure(err, loc.Name)
		}

		m, err := obj.Decode(records)
		if err != nil {
			return failed(fmt.Errorf("not an object module: %w", err), found.Name)
		}

		if problems := obj.Check(m); len(problems) > 0 {
			return failed(fmt.Errorf("%s", problems[0]), found.Name)
		}

		in.modules = append(in.modules, link.Input{File: found.Name, Module: m, Selective: f.Selective})
	}

	in.noteFile(found)

	return nil
}

// linkLibrary reads a library: one to search (/LIBRARY), whose modules
// named by /INCLUDE are added to the link. A shareable image library, such
// as IMAGELIB.OLB, can only be searched.
func (c *Console) linkLibrary(in *linkInputs, loc rms.FileLocation, f link.InputFile) (rms.FileLocation, error) {
	data, found, err := c.ContainerSession.ReadRawFile(loc)
	if err != nil {
		return found, fileFailure(err, loc.Name)
	}

	lib, err := lbr.Open(data)
	if err != nil {
		return found, vmserrors.Wrap(vmserrors.CLI_LINKING, fmt.Errorf("%s: %w", found.Name, err), found.Name)
	}

	switch lib.Type {
	case lbr.TypeShareable:
		if len(f.Include) > 0 {
			return found, vmserrors.Wrap(vmserrors.CLI_LINKING, fmt.Errorf("%s is a shareable image library, whose modules can't be included", found.Name), found.Name)
		}

		govax := govaxSymbols()
		in.libraries = append(in.libraries, &link.ImageLibrarySource{
			File:    found.Name,
			Library: lib,
			Open:    func(image string) (link.SymbolSource, error) { return c.openSharedImage(image, govax) },
		})

		return found, nil

	case lbr.TypeObject:
	default:
		return found, vmserrors.Wrap(vmserrors.CLI_LINKING, fmt.Errorf("%s is a %s library, not an object library", found.Name, lib.Type), found.Name)
	}

	src := &link.ObjectLibrarySource{File: found.Name, Library: lib}

	for _, name := range f.Include {
		module, err := src.Include(name)
		if err != nil {
			return found, vmserrors.Wrap(vmserrors.CLI_LINKING, err, found.Name)
		}

		in.modules = append(in.modules, *module)
	}

	if f.Library {
		in.libraries = append(in.libraries, src)
	}

	return found, nil
}

// noteFile records a file LINK read: the first names the image and map,
// and each is where the next bare name is looked for.
func (in *linkInputs) noteFile(found rms.FileLocation) {
	if !in.seen {
		in.first, in.seen = found, true
	}

	in.prev = found
}

// mergeOptions takes an options file's options: a later file's value
// replaces an earlier one's, and symbols add up.
func (in *linkInputs) mergeOptions(o *link.OptionsFile) {
	if o.Stack != 0 {
		in.options.Stack = o.Stack
	}

	if o.Ident != "" {
		in.options.Ident = o.Ident
	}

	if o.Name != "" {
		in.options.Name = o.Name
	}

	in.options.Symbols = append(in.options.Symbols, o.Symbols...)
}
