package console

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"time"

	"github.com/tucats/govax/internal/asm"
	"github.com/tucats/govax/internal/bootdata"
	"github.com/tucats/govax/internal/lbr"
	"github.com/tucats/govax/internal/obj"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmserrors"
)

// This file implements docs/PHASE-27.md subtask 10's MACRO command, with
// docs/PHASE-28.md subtask 9's macro libraries:
//
//	MACRO source[/HOST] [/[NO]OBJECT[=object]] [/[NO]LIST[=listing]]
//	      [/LIBRARY=(library[,...])]
//
// It assembles a MACRO-32 source file with internal/asm's MACRO dialect
// and writes the object module. The source and object can each be a host
// file or a file on a mounted ODS-2 volume, decided by internal/rms's
// file-name rules (rms.Session.Locate): the source by its own name and
// /HOST, and an object name given with /OBJECT= by its name, a bare one
// following the source (same side, and on a volume the same device and
// directory). .INCLUDE names are resolved the same way, relative to the
// source.
//
// Macros the source doesn't define come from macro libraries, searched as
// VMS MACRO searches them: the libraries .LIBRARY names, the last named
// first; then /LIBRARY='s, also the last named first; then STARLET.MLB.
// .LIBRARY and /LIBRARY= names are resolved relative to the source, with
// the default type MLB. STARLET.MLB is SYS$LIBRARY:STARLET.MLB on a mounted
// volume, or STARLET.MLB in the host library directory (syslib.go), or
// else govax's own, from bootdata; it's read only when a macro is looked
// for.
//
// Every assembly error is reported, and then no object file is written:
// the object is built in memory and written only once assembly succeeds,
// so a failed assembly never leaves a partial object or replaces a good
// one.
//
// /LIST (docs/PHASE-29.md subtask 5) writes the listing internal/asm lays
// out, named and placed as the object is but with the type LIS. It's
// written whether or not the assembly succeeds, as real MACRO's is.

// BuildVersion is govax's version ("1.0-115"), set by cmd/govax. The
// object's language processor header names it, as real MACRO's names
// "VAX MACRO V5.4-3".
var BuildVersion string

// MacroOptions is one MACRO command.
type MacroOptions struct {
	// Source is the source file name, and SourceHost an explicit /HOST on
	// it.
	Source     string
	SourceHost bool

	// Object is the object file name from /OBJECT=; "" means the default
	// name: the source's, with the extension OBJ in the same case as the
	// source's extension. NoObject is /NOOBJECT: assemble and report
	// errors, but write nothing.
	Object   string
	NoObject bool

	// List is /LIST: write a listing, to ListFile, or by default to the
	// source's name with the type LIS, beside the source as the object
	// is (docs/PHASE-29.md subtask 5).
	List     bool
	ListFile string

	// Libraries are the macro libraries /LIBRARY= names, in the order
	// given.
	Libraries []string

	// CommandLine is the command as typed, for the object's SRC header,
	// where real MACRO records its command line.
	CommandLine string
}

// defaultSourceType is the file type MACRO gives a source named without
// one, as VMS MACRO does.
const defaultSourceType = "MAR"

// Macro assembles one MACRO-32 source file and, unless opts.NoObject,
// writes its object module; with opts.List, it writes the listing too.
func (c *Console) Macro(opts MacroOptions) error {
	// Everything before the assembler is called (finding and reading the
	// source, opening the libraries) is the listing's command processing
	// phase.
	commandStart := asm.StartPhase()
	s := c.ContainerSession

	loc, err := s.Locate(opts.Source, opts.SourceHost)
	if err != nil {
		return fileFailure(err, opts.Source)
	}

	loc = withDefaultType(loc, defaultSourceType)

	lines, found, err := s.ReadRecordFile(loc, rms.TextRecords)
	if err != nil {
		return fileFailure(err, loc.Name)
	}

	a := asm.New(false)
	a.SetDialect(asm.DialectMACRO)
	a.SetListing(opts.List)
	a.SetIncludeResolver(func(name string) (string, error) {
		incLoc, err := s.LocateRelated(name, false, found)
		if err != nil {
			return "", err
		}

		text, _, err := s.ReadRecordFile(incLoc, rms.TextRecords)

		return joinLines(text), err
	})
	a.SetLibraryResolver(func(name string) (asm.MacroLibrary, error) {
		return c.openMacroLibrary(name, found)
	})

	libs := make([]asm.MacroLibrary, 0, len(opts.Libraries)+1)

	for k := len(opts.Libraries) - 1; k >= 0; k-- {
		lib, err := c.openMacroLibrary(opts.Libraries[k], found)
		if err != nil {
			return err
		}

		libs = append(libs, lib)
	}

	a.SetMacroLibraries(append(libs, &starletMacros{c: c})...)

	commandProcessing := commandStart.Elapsed()

	_, asmErr := a.Assemble(joinLines(lines))

	for _, m := range a.Messages() {
		c.Printf("%s\n", m)
	}

	for _, w := range a.Warnings() {
		c.Printf("%%%s\n", vmserrors.Wrap(vmserrors.CLI_ASMWARNING, w, found.Name))
	}

	// The object is written only when the assembly succeeds; the listing
	// is written either way, as real MACRO writes it, after the object so
	// that it can count the object's records. A failure writing the
	// listing is reported only if nothing failed before it.
	var status error

	switch {
	case asmErr != nil:
		all := []error{asmErr}

		var list *asm.Errors
		if errors.As(asmErr, &list) {
			all = list.List
		}

		for _, e := range all {
			c.Printf("%%%s\n", vmserrors.Wrap(vmserrors.CLI_ASSEMBLING, e, found.Name))
		}

		status = vmserrors.New(vmserrors.CLI_ASMERRORS, len(all), found.Name)
	case !opts.NoObject:
		status = c.writeObject(a, opts, found)
	}

	if opts.List {
		if err := c.writeListing(a, opts, found, commandProcessing); err != nil && status == nil {
			status = err
		}
	}

	return status
}

// writeObject writes the object module of a's assembly of the source
// found.
func (c *Console) writeObject(a *asm.Assembler, opts MacroOptions, found rms.FileLocation) error {
	s := c.ContainerSession

	objLoc, err := outputLocation(s, opts.Object, found, "OBJ")
	if err != nil {
		return fileFailure(err, opts.Object)
	}

	module, err := a.Object(asm.ObjectOptions{
		Language: macroAssemblerName(),
		Source:   opts.CommandLine,
	})
	if err != nil {
		return vmserrors.Wrap(vmserrors.CLI_OBJWRITE, err, objLoc.Name)
	}

	records, err := obj.Encode(module)
	if err != nil {
		return vmserrors.Wrap(vmserrors.CLI_OBJWRITE, err, objLoc.Name)
	}

	if _, err := s.CreateRecordFile(objLoc, rms.VariableRecords, records); err != nil {
		return objectFailure(err, objLoc.Name)
	}

	return nil
}

// writeListing writes the listing of a's assembly of the source found, a
// text file with a record for each line. commandProcessing is how long
// the command took before the assembler was called.
func (c *Console) writeListing(a *asm.Assembler, opts MacroOptions, found rms.FileLocation, commandProcessing asm.PhaseTime) error {
	s := c.ContainerSession

	lisLoc, err := outputLocation(s, opts.ListFile, found, "LIS")
	if err != nil {
		return fileFailure(err, opts.ListFile)
	}

	// The heading shows the source's full file specification: a volume
	// file's, version included, or a host file's absolute path.
	source := found.Name
	if found.Host {
		if abs, err := filepath.Abs(source); err == nil {
			source = abs
		}
	}

	revised, err := s.RevisionDate(found)
	if err != nil {
		return fileFailure(err, found.Name)
	}

	lines := a.Listing(asm.ListingOptions{
		Assembled:         time.Now(),
		Assembler:         macroAssemblerName(),
		Source:            source,
		Revised:           revised,
		Command:           opts.CommandLine,
		CommandProcessing: commandProcessing,
	})

	records := make([][]byte, len(lines))
	for i, line := range lines {
		records[i] = []byte(line)
	}

	if _, err := s.CreateRecordFile(lisLoc, rms.TextRecords, records); err != nil {
		return objectFailureAs(vmserrors.CLI_LISWRITE, err, lisLoc.Name)
	}

	return nil
}

// macroAssemblerName is how the object's language processor header and
// the listing's heading name the assembler, where real MACRO's say "VAX
// MACRO V5.4-3".
func macroAssemblerName() string {
	return "govax MACRO V" + BuildVersion
}

// openMacroLibrary reads the macro library name, a /LIBRARY= or .LIBRARY
// file, found relative to the source file.
func (c *Console) openMacroLibrary(name string, source rms.FileLocation) (asm.MacroLibrary, error) {
	s := c.ContainerSession

	loc, err := s.LocateRelated(name, false, source)
	if err != nil {
		return nil, fileFailure(err, name)
	}

	loc = withDefaultType(loc, "MLB")

	data, found, err := s.ReadRawFile(loc)
	if err != nil {
		return nil, fileFailure(err, loc.Name)
	}

	lib, err := macroLibrary(data, found.Name)
	if err != nil {
		return nil, err
	}

	return asm.NamedMacroLibrary(lib, found.Name), nil
}

// macroLibrary reads data, the library file name, as a macro library.
func macroLibrary(data []byte, name string) (asm.MacroLibrary, error) {
	l, err := lbr.Open(data)
	if err == nil {
		var lib asm.MacroLibrary
		if lib, err = asm.NewMacroLibrary(l); err == nil {
			return lib, nil
		}
	}

	return nil, vmserrors.Wrap(vmserrors.CLI_LIBRARY, err, name)
}

// starletMacros is STARLET.MLB, the last library MACRO searches. It's
// found and read the first time a macro is looked for in it, so a program
// that needs no library macros never reads it.
type starletMacros struct {
	c   *Console
	lib asm.MacroLibrary
	err error
}

// Macro implements asm.MacroLibrary.
func (s *starletMacros) Macro(name string) ([]string, bool, error) {
	if s.lib == nil && s.err == nil {
		s.lib, s.err = s.c.openStarlet()
	}

	if s.err != nil {
		return nil, false, s.err
	}

	return s.lib.Macro(name)
}

// LibraryName implements asm.NamedLibrary, for the listing's macro
// library statistics, which name the library even when no macro came
// from it, as real MACRO's do. So a listing opens it if nothing has yet.
func (s *starletMacros) LibraryName() string {
	if s.lib == nil && s.err == nil {
		s.lib, s.err = s.c.openStarlet()
	}

	if n, ok := s.lib.(asm.NamedLibrary); ok {
		return n.LibraryName()
	}

	return "SYS$LIBRARY:STARLET.MLB"
}

// openStarlet reads STARLET.MLB: SYS$LIBRARY's, the host library
// directory's, or govax's own.
func (c *Console) openStarlet() (asm.MacroLibrary, error) {
	data, name, err := c.readLibraryFile("SYS$LIBRARY", "STARLET.MLB")
	if err != nil {
		return nil, objectFailureAs(vmserrors.CLI_LIBRARY, err, name)
	}

	if data == nil {
		name = "govax STARLET.MLB"

		if data, err = fs.ReadFile(bootdata.FS, bootdata.StarletLibrary); err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
	}

	lib, err := macroLibrary(data, name)
	if err != nil {
		return nil, err
	}

	return asm.NamedMacroLibrary(lib, name), nil
}

// joinLines turns records read from a text file back into source text.
func joinLines(lines [][]byte) string {
	var b strings.Builder

	for i, l := range lines {
		if i > 0 {
			b.WriteByte('\n')
		}

		b.Write(l)
	}

	return b.String()
}

// objectFailure turns an error creating the object file into the
// console's status for it: an unmounted device or a bad logical name as
// fileFailure reports them, and anything else as CLI_OBJWRITE.
func objectFailure(err error, spec string) error {
	return objectFailureAs(vmserrors.CLI_OBJWRITE, err, spec)
}

// objectFailureAs is objectFailure for any command's output file, with
// status for what fileFailure doesn't report.
func objectFailureAs(status uint32, err error, spec string) error {
	var notMounted *rms.NotMountedError
	if logicalNameFailure(err) != nil || errors.As(err, &notMounted) {
		return fileFailure(err, spec)
	}

	return vmserrors.Wrap(status, err, spec)
}
