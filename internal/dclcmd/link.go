package dclcmd

import (
	"errors"
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
//	LINK file[,file...][/HOST] [/EXECUTABLE[=image] | /NOEXECUTABLE | /SHAREABLE[=image]]
//	     [/[NO]TRACEBACK] [/[NO]DEBUG] [/[NO]SYSLIB] [/[NO]USERLIBRARY[=(table,...)]]
//	     [/MAP[=map] [/BRIEF] | /NOMAP]
//
// where each file is an object, or has its own qualifier:
//
//	file/LIBRARY[/INCLUDE=(module,...)]   a library to search (.OLB)
//	file/INCLUDE=(module,...)             modules of a library to add
//	file/SELECTIVE_SEARCH                 an object searched selectively
//	file/OPTIONS                          an options file (.OPT)
//
// An options file (link.OptionsParser) names more input files, which may
// also be shareable images (file/SHAREABLE, .EXE), and options. LINK acts
// on every VAX LINK option but UNSUPPORTED=, which it refuses.
// UNIVERSAL= and GSMATCH= are a shareable image's, ignored in an
// executable, as VMS's LINK ignored UNIVERSAL= (testdata/link68's E8).
// /SHAREABLE makes a shareable image (docs/PHASE-68, subtask 5), whose
// options files ignore ISD_MAX= (OPTIGNSHR).
// SYS$INPUT/OPTIONS reads the options from SYS$INPUT: in a command
// procedure, the data lines after the LINK command.
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
// options files name, then the libraries named, in order, then the default
// user libraries (LNK$LIBRARY, userlib.go), then the system libraries and
// govax's own tables (linksource.go). A shareable image library's images
// are looked for beside it first (openLibraryImage).

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

	// Shareable is /SHAREABLE: link a shareable image, named by
	// ShareableFile (/SHAREABLE='s value), or as Executable says.
	Shareable     bool
	ShareableFile string

	// SymbolTable is /SYMBOL_TABLE: write the image's global symbol
	// table as an object module file, to SymbolFile, or by default to a
	// file named after the first file with the type STB.
	SymbolTable bool
	SymbolFile  string

	// NoTraceback is /NOTRACEBACK: the image doesn't start through
	// SYS$IMGSTA.
	NoTraceback bool

	// Debug is /DEBUG: the objects' debugger records go into the image's
	// debug symbol table with their traceback records, and traceback is
	// on whatever NoTraceback says, as VMS LINK's /DEBUG does.
	// DebugModule is /DEBUG='s value, VMS's user-written debugger
	// module, which govax refuses.
	Debug       bool
	DebugModule string

	// NoSysLib is /NOSYSLIB: don't search IMAGELIB.OLB and STARLET.OLB
	// (linksource.go).
	NoSysLib bool

	// UserLibrary is /USERLIBRARY='s tables (PROCESS, GROUP, SYSTEM, ALL,
	// NONE; none given is ALL), whose LNK$LIBRARY names name default user
	// libraries (userlib.go). NoUserLibrary is /NOUSERLIBRARY.
	UserLibrary   []string
	NoUserLibrary bool

	// Map is /MAP: write a link map, to MapFile, or by default to a file
	// named after the first file with the type MAP. Brief is /BRIEF:
	// only the object modules and the image synopsis. Full is /FULL:
	// every section, and what libraries and shareable images contributed
	// (docs/PHASE-68, subtask 8); it conflicts with /BRIEF.
	Map     bool
	MapFile string
	Brief   bool
	Full    bool
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
	// shareable says the link makes a shareable image, whose options
	// files ignore some options.
	shareable bool
	// first is the first file, which names the image and map by default,
	// and prev the one before the file being read.
	first, prev rms.FileLocation
	seen        bool
}

// Link links object modules into an executable image.
func (c *Context) Link(opts LinkOptions) error {
	s := c.ContainerSession

	files := make([]link.InputFile, 0, len(opts.Objects)+len(opts.Files))
	for _, name := range opts.Objects {
		files = append(files, link.InputFile{Name: name})
	}

	files = append(files, opts.Files...)

	if len(files) == 0 {
		return vmserrors.New(vmserrors.CLI_NEEDFILENAME, "LINK")
	}

	// The Linker manual: /FULL "is not compatible with the /BRIEF
	// qualifier". VMS's own message for the pair is unconfirmed; DCL's
	// for elements that conflict is CONFLICT.
	if opts.Full && opts.Brief {
		return vmserrors.New(vmserrors.CLI_CONFLICT)
	}

	if opts.Debug && opts.DebugModule != "" {
		return vmserrors.Wrap(vmserrors.CLI_LINKING, fmt.Errorf("a user-written debugger module (/DEBUG=%s) isn't supported", opts.DebugModule), "LINK")
	}

	if opts.Shareable && opts.ShareableFile != "" {
		if opts.Executable != "" {
			return vmserrors.Wrap(vmserrors.CLI_LINKING, fmt.Errorf("/EXECUTABLE and /SHAREABLE both name the image"), "LINK")
		}

		opts.Executable = opts.ShareableFile
	}

	in := &linkInputs{shareable: opts.Shareable}

	for _, f := range files {
		if err := c.linkInput(in, f, opts.Host, true); err != nil {
			return err
		}
	}

	if len(in.modules) == 0 {
		return vmserrors.Wrap(vmserrors.CLI_LINKING, fmt.Errorf("no object modules to link"), in.first.Name)
	}

	// Options govax's LINK reads but doesn't act on yet.
	if names := in.options.Unimplemented(); len(names) > 0 {
		return vmserrors.Wrap(vmserrors.CLI_LINKING, fmt.Errorf("the option %s isn't supported yet", strings.Join(names, ", ")), in.first.Name)
	}

	exe, err := outputLocation(s, opts.Executable, in.first, "EXE")
	if err != nil {
		return FileFailure(err, opts.Executable)
	}

	var user []link.SymbolSource

	if !opts.NoUserLibrary {
		tables, err := userLibraryTableNames(opts.UserLibrary)
		if err != nil {
			return err
		}

		if user, err = c.userLibraries(tables); err != nil {
			return err
		}
	}

	system, err := c.LinkSources(!opts.NoSysLib)
	if err != nil {
		return vmserrors.Wrap(vmserrors.CLI_LINKING, err, exe.Name)
	}

	// The default user libraries come after the command's own, and before
	// the system's. Last, the images the shareable images the link uses
	// use themselves, which no other source describes, read from their
	// files.
	sources := append(append(append(append(in.shared, in.libraries...), user...), system...), c.usedImages())

	name := imageName(exe)
	if in.options.Name != "" {
		name = in.options.Name
	}

	o := &in.options

	img, err := link.Link(in.modules, link.Options{
		ImageName:  name,
		LinkerID:   linkerID(),
		Traceback:  !opts.NoTraceback,
		Debug:      opts.Debug,
		Sources:    sources,
		StackPages: o.Stack,
		Ident:      o.Ident,
		Symbols:    o.Symbols,
		Clusters:   o.Clusters,
		Collects:   o.Collects,
		PsectAttrs: o.PsectAttrs,
		Base:       o.Base,
		DZROMin:    o.DZROMin,
		ISDMax:     o.ISDMax,
		IOSegment:  o.IOSegment,
		Shareable:  opts.Shareable,
		GSMatch:    o.GSMatch,
		Universals: o.Universals,
	})
	if err != nil {
		return linkFailure(err, exe.Name)
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

	// Real LINK's warnings, which don't stop the image: undefined symbols
	// and each reference to one, and a missing transfer address. Its
	// status says whether there were any (testdata/link68's log).
	status := uint32(ssNormal)

	note := func(m link.Message) {
		c.Printf("%s\n", m)

		if m.Severity == 'W' {
			status = linkWarnings
		}
	}

	for _, m := range img.Messages {
		note(m)
	}

	if imageFile != "" && !img.HasTransfer && !opts.Shareable {
		note(link.NoTransferMessage(imageFile))
	}

	if c.SetStatus != nil {
		c.SetStatus(status | stsInhibitMsg)
	}

	if opts.SymbolTable {
		if err := c.writeSymbolTable(img, opts.SymbolFile, in.first); err != nil {
			return err
		}
	}

	if !opts.Map {
		return nil
	}

	mapLoc, err := outputLocation(s, opts.MapFile, in.first, "MAP")
	if err != nil {
		return FileFailure(err, opts.MapFile)
	}

	// The map records its own name, on a volume with the version it
	// gets, as real LINK's maps do ("FULL in file DUA1:[000000]SHR1.MAP;1",
	// testdata/link68): the map is written, then written again at that
	// version with its name in it.
	mapRecords := func(name string) [][]byte {
		lines := img.Map(link.MapOptions{
			ImageFile: imageFile,
			ImageText: opts.Executable,
			MapFile:   name,
			Brief:     opts.Brief,
			Full:      opts.Full,
		})

		records := make([][]byte, len(lines))
		for i, line := range lines {
			records[i] = []byte(line)
		}

		return records
	}

	created, err := s.CreateRecordFile(mapLoc, rms.TextRecords, mapRecords(mapLoc.Name))
	if err != nil {
		return vmserrors.Wrap(vmserrors.CLI_LINKING, err, mapLoc.Name)
	}

	if !mapLoc.Host && created.Name != mapLoc.Name {
		if _, err := s.RewriteRecordFile(created, rms.TextRecords, mapRecords(created.Name)); err != nil {
			return vmserrors.Wrap(vmserrors.CLI_LINKING, err, created.Name)
		}
	}

	return nil
}

// writeSymbolTable writes the image's global symbol table (/SYMBOL_TABLE)
// as an object module file: file, or the first input file's name with the
// type STB. Another link can name it as an input object; real LINK's
// SHR1.STB is the table SHR1.EXE holds, record for record.
func (c *Context) writeSymbolTable(img *link.Image, file string, first rms.FileLocation) error {
	s := c.ContainerSession

	loc, err := outputLocation(s, file, first, "STB")
	if err != nil {
		return FileFailure(err, file)
	}

	records, err := img.SymbolTable()
	if err != nil {
		return vmserrors.Wrap(vmserrors.CLI_LINKING, err, loc.Name)
	}

	if _, err := s.CreateRecordFile(loc, rms.VariableRecords, records); err != nil {
		return vmserrors.Wrap(vmserrors.CLI_LINKING, err, loc.Name)
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
		name, _ = rms.SplitVersion(name)
		name = rms.NameType(name)
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
func (c *Context) linkInput(in *linkInputs, f link.InputFile, host, fromCommand bool) error {
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

	if typ == "OPT" && !host && c.InputStream != nil && c.InputStream(f.Name) {
		return c.linkOptions(in, inputOptionsName, c.inputLine, false, host)
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
		return FileFailure(err, f.Name)
	}

	loc = loc.WithDefaultType(typ)

	failed := func(err error, name string) error {
		return vmserrors.Wrap(vmserrors.CLI_LINKING, fmt.Errorf("%s: %w", name, err), name)
	}

	var found rms.FileLocation

	switch typ {
	case "OPT":
		var records [][]byte

		if records, found, err = s.ReadRecordFile(loc, rms.TextRecords); err != nil {
			return FileFailure(err, loc.Name)
		}

		next := func() (string, bool) {
			if len(records) == 0 {
				return "", false
			}

			line := string(records[0])
			records = records[1:]

			return line, true
		}

		in.noteFile(found)

		if err := c.linkOptions(in, found.Name, next, c.VerifyImage != nil && c.VerifyImage(), host); err != nil {
			return err
		}

		in.prev = found

		return nil

	case "EXE":
		var data []byte

		if data, found, err = s.ReadRawFile(loc); err != nil {
			return FileFailure(err, loc.Name)
		}

		src, image, err := link.ReadShareableImage(data)
		if err != nil {
			return failed(err, found.Name)
		}

		src.File = found.Name
		in.shared = append(in.shared, src)

		in.options.NameImage(image)

	case "OLB":
		if found, err = c.linkLibrary(in, loc, f); err != nil {
			return err
		}

	default:
		var records [][]byte

		if records, found, err = s.ReadRecordFile(loc, rms.VariableRecords); err != nil {
			return FileFailure(err, loc.Name)
		}

		m, err := obj.Decode(records)
		if err != nil {
			return failed(fmt.Errorf("not an object module: %w", err), found.Name)
		}

		if problems := obj.Check(m); len(problems) > 0 {
			return failed(fmt.Errorf("%s", problems[0]), found.Name)
		}

		in.modules = append(in.modules, link.Input{File: found.Name, Module: m, Selective: f.Selective, Cluster: f.Cluster})
	}

	in.noteFile(found)

	return nil
}

// linkLibrary reads a library: one to search (/LIBRARY), whose modules
// named by /INCLUDE are added to the link. A shareable image library's
// modules named by /INCLUDE are its images, which the link uses.
func (c *Context) linkLibrary(in *linkInputs, loc rms.FileLocation, f link.InputFile) (rms.FileLocation, error) {
	data, found, err := c.ContainerSession.ReadRawFile(loc)
	if err != nil {
		return found, FileFailure(err, loc.Name)
	}

	lib, err := lbr.Open(data)
	if err != nil {
		return found, vmserrors.Wrap(vmserrors.CLI_LINKING, fmt.Errorf("%s: %w", found.Name, err), found.Name)
	}

	switch lib.Type {
	case lbr.TypeShareable:
		govax := GovaxSymbols(c.ShimLinks)

		// /INCLUDE= takes the images named into the link as an options
		// file's NAME/SHAREABLE would (the Linker manual, 6.3.3.2).
		for _, name := range f.Include {
			image := strings.ToUpper(name)
			if _, ok := lib.Lookup(image); !ok {
				return found, vmserrors.Wrap(vmserrors.CLI_LINKING, fmt.Errorf("%s has no module %s", found.Name, image), found.Name)
			}

			src, err := c.openLibraryImage(image, found, govax)
			if err != nil {
				return found, vmserrors.Wrap(vmserrors.CLI_LINKING, err, found.Name)
			}

			in.shared = append(in.shared, src)
			in.options.Clusters = append(in.options.Clusters, link.Cluster{Shareable: true, Image: image})
		}

		if f.Library {
			in.libraries = append(in.libraries, c.imageLibrary(lib, found, govax))
		}

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

		module.Cluster = f.Cluster
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

// linkWarnings is LINK's status for an image made with warnings (USRTFR,
// NUDFSYMS, ...), less STS$M_INHIBIT_MSG: %X10648268 in testdata/link68's
// log.
const linkWarnings = 0x00648268

// inputOptionsName is SYS$INPUT as an options file, named as VMS's LINK
// names it in a message (testdata/link68's E4).
const inputOptionsName = "SYS$INPUT:.;"

// linkOptions reads an options file, named file, whose lines next
// returns, a line at a time, into in, then reads the input files it
// names. With echo, each line is shown as it's read, as VMS's LINK does
// with image verification on (testdata/link68's log); SYS$INPUT's lines
// DCL shows itself. LINK stops at the first line in error, so the data
// lines after it in a procedure are DCL's to skip.
func (c *Context) linkOptions(in *linkInputs, file string, next func() (string, bool), echo, host bool) error {
	p := link.OptionsParser{Shareable: in.shareable}

	// Notices about a line follow it (OPTIGNSHR).
	notices := func() {
		for _, m := range p.Notices() {
			c.Printf("%s\n", m)
		}
	}

	for {
		line, ok := next()
		if !ok {
			break
		}

		if echo {
			c.Printf("%s\n", line)
		}

		if err := p.Line(line); err != nil {
			return optionsFailure(err, file)
		}

		notices()
	}

	o, err := p.End()
	if err != nil {
		return optionsFailure(err, file)
	}

	notices()

	in.options.Merge(o)

	for _, g := range o.Files {
		if err := c.linkInput(in, g, host, false); err != nil {
			return err
		}
	}

	return nil
}

// linkFailure is the error for a link that made no image: a
// link.FatalError is real LINK's messages, with LINK's status (marked as
// shown).
func linkFailure(err error, image string) error {
	var fe *link.FatalError
	if errors.As(err, &fe) {
		return vmserrors.ConditionError{Status: fe.Status() | stsInhibitMsg, Text: strings.TrimPrefix(fe.Error(), "%")}
	}

	return vmserrors.Wrap(vmserrors.CLI_LINKING, err, image)
}

// optionsFailure is the error for an options file, named file, LINK
// couldn't read: an OptionError is VMS's message, with LINK's status
// (marked as shown, as VMS's LINK leaves it).
func optionsFailure(err error, file string) error {
	var oe *link.OptionError
	if errors.As(err, &oe) {
		return vmserrors.ConditionError{Status: oe.Status() | stsInhibitMsg, Text: oe.Message(file)}
	}

	return vmserrors.Wrap(vmserrors.CLI_LINKING, fmt.Errorf("%s: %w", file, err), file)
}

// inputLine reads SYS$INPUT's next line: in a command procedure, the data
// line after the command; at the terminal, a line typed. ok is false at
// its end of file.
func (c *Context) inputLine() (line string, ok bool) {
	if c.RTL == nil || (c.Running != nil && !c.Running()) {
		return "", false
	}

	return c.RTL.ReadLine(maxInputRecord)
}
