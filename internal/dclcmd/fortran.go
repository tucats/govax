package dclcmd

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"time"

	"github.com/tucats/govax/internal/fortran"
	"github.com/tucats/govax/internal/fortran/sema"
	"github.com/tucats/govax/internal/fortran/source"
	"github.com/tucats/govax/internal/fortran/sysdef"
	"github.com/tucats/govax/internal/lbr"
	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmserrors"
)

// This file is the FORTRAN command (docs/PHASE-74 - FORTRAN plan.md,
// Decision 18; subtask 2):
//
//	FORTRAN source[+source...][+library/LIBRARY][,...][/HOST] [/MACRO[=file] | /NOMACRO]
//	        [/[NO]EXTEND_SOURCE] [/[NO]D_LINES] [/CONTINUATIONS=n] [/[NO]I4]
//	        [/[NO]G_FLOATING] [/ALIGN[=value] | /NOALIGN]
//
// It compiles each FORTRAN source file with internal/fortran and writes a
// MACRO-32 source file, which the MACRO command assembles. As MACRO does
// with its source and object, the source and the MACRO file can each be
// a host file or a file on a mounted volume (rms.Session.Locate's rules),
// a source named without a type is .FOR, and the MACRO file is by default
// the source's name with the type MAR, beside the source.
//
// The compiler prints its own messages. A compilation with errors writes
// no MACRO file (so a failed compilation never replaces a good one) and
// ends with CLI_FORERRORS; one with warnings writes the file and leaves
// $STATUS a warning. Several sources, separated by commas, are compiled
// one after another, each to its own MACRO file, and the command's
// status is the last failure's. Sources joined by + are one compilation, to
// one MACRO file, named by default for the first (which VAX FORTRAN's object
// file is named for isn't in the manuals to hand; govax's choice, unconfirmed);
// each name's missing parts come from the name before it in the list.
//
// A source's INCLUDE 'file' statements name files found as MACRO's
// .INCLUDE finds them: relative to the file that includes them (on the
// host, in its directory, for a bare name or a relative path; on a
// volume, on its device and in its directory), with the default type FOR
// (subtask 3). Whether VAX
// FORTRAN looks beside the including file or in the default directory
// isn't in the manuals to hand; for a source in the default directory
// the two agree. govax's choice, unconfirmed.
//
// INCLUDE '(module)' searches, in order and unconfirmed (the VAX FORTRAN User
// Manual isn't to hand; docs/PHASE-83 - FORTRAN and VMS.md, Decision 2): the
// libraries named with /LIBRARY in the source's +-list, the library or search
// list the logical name FORT$LIBRARY names, SYS$LIBRARY:FORSYSDEF.TLB if
// there is one, and last the definition modules built into govax
// (internal/fortran/sysdef). INCLUDE 'lib(module)' reads only lib, found as an
// INCLUDE file is, of default type TLB.

// FortranOptions is one FORTRAN command.
type FortranOptions struct {
	// Sources are the source files and text libraries, in the order given,
	// and SourceHost an explicit /HOST on them.
	Sources    []FortranSource
	SourceHost bool

	// Macro is the MACRO file's name from /MACRO=; "" means the default:
	// the source's name with the type MAR. With several sources it names
	// a directory, or a file each compilation replaces (a new version on
	// a volume). NoMacro is /NOMACRO: compile and report, but write
	// nothing.
	Macro   string
	NoMacro bool

	// ExtendSource and DLines are /EXTEND_SOURCE and /D_LINES
	// (fortran.Options).
	ExtendSource bool
	DLines       bool

	// NoI4 is /NOI4 (fortran.Options).
	NoI4 bool

	// GFloating is /G_FLOATING (fortran.Options).
	GFloating bool

	// Align is /ALIGN's values, if AlignGiven says it was given; /NOALIGN is
	// the default, so nothing need say it (sema.Alignment).
	Align      []string
	AlignGiven bool

	// Continuations is /CONTINUATIONS=n.
	Continuations FortranContinuations

	// CommandLine is the command as typed, for the MACRO file's header.
	CommandLine string
}

// FortranSource is one element of the FORTRAN command's list.
type FortranSource struct {
	// Name is the file's name, and Library says it has /LIBRARY: a text
	// library for this compilation's INCLUDE statements.
	Name    string
	Library bool

	// Joined says a + follows it, so the next element is in the same
	// compilation.
	Joined bool
}

// FortranSources makes a list of source files, each its own compilation.
func FortranSources(names ...string) []FortranSource {
	sources := make([]FortranSource, len(names))
	for i, name := range names {
		sources[i].Name = name
	}

	return sources
}

// fortranCompilations groups the command's list: each compilation's elements,
// from one a + doesn't follow to the next.
func fortranCompilations(list []FortranSource) [][]FortranSource {
	var groups [][]FortranSource

	for start, i := 0, 0; i < len(list); i++ {
		if !list[i].Joined || i == len(list)-1 {
			groups = append(groups, list[start:i+1])
			start = i + 1
		}
	}

	return groups
}

// FortranContinuations is /CONTINUATIONS=n: whether it was given, and
// n, the most continuation lines one statement may have.
type FortranContinuations struct {
	Given bool
	Lines int64
}

// limit is the compiler's Options.Continuations for the qualifier, or an
// error if n is out of range.
func (fc FortranContinuations) limit() (int, error) {
	switch {
	case !fc.Given:
		return 0, nil
	case fc.Lines < 0 || fc.Lines > source.MaxContinuations:
		return 0, vmserrors.New(vmserrors.CLI_FORCONTRANGE, fc.Lines, source.MaxContinuations)
	case fc.Lines == 0:
		return source.NoContinuations, nil
	}

	return int(fc.Lines), nil
}

// alignment is the compiler's Options.Align for /ALIGN, or an error if a value
// isn't one.
func (opts FortranOptions) alignment() (sema.Alignment, error) {
	if !opts.AlignGiven {
		return sema.Alignment{}, nil
	}

	al, err := sema.Alignment{}.WithAlign(opts.Align)
	if err != nil {
		return al, vmserrors.New(vmserrors.CLI_FORALIGN, err.Error())
	}

	return al, nil
}

// defaultFortranType is the file type FORTRAN gives a source named
// without one, as VMS's FORTRAN command does, and defaultTextLibraryType a
// text library's. fortLibraryName and forSysDefFile name the libraries
// searched by default.
const (
	defaultFortranType     = "FOR"
	defaultTextLibraryType = "TLB"
	fortLibraryName        = "FORT$LIBRARY"
	forSysDefFile          = "FORSYSDEF.TLB"
	builtinModuleSpec      = "FORSYSDEF.TLB (built into govax)"
)

// Fortran compiles each source file and, unless opts.NoMacro, writes its
// MACRO file.
func (c *Context) Fortran(opts FortranOptions) error {
	if _, err := opts.Continuations.limit(); err != nil {
		return err
	}

	if _, err := opts.alignment(); err != nil {
		return err
	}

	groups := fortranCompilations(opts.Sources)
	if len(groups) == 1 {
		return c.compileFortran(groups[0], opts)
	}

	// With several sources, each failure is shown as it happens, so the
	// next file's messages follow it in order, and the command ends with
	// the last failure's status, not shown again.
	var failed uint32

	for _, group := range groups {
		if err := c.compileFortran(group, opts); err != nil {
			c.Printf("%%%s\n", err)

			failed = vmserrors.CLI_FORERRORS

			var vmsErr vmserrors.VMSError
			if errors.As(err, &vmsErr) {
				failed = vmsErr.Status
			}
		}
	}

	if failed != 0 && c.SetStatus != nil {
		c.SetStatus(failed | stsInhibitMsg)
	}

	return nil
}

// compileFortran does one compilation: its source files, and the text
// libraries among them.
func (c *Context) compileFortran(group []FortranSource, opts FortranOptions) error {
	s := c.ContainerSession
	includes := &fortranIncludes{c: c}

	var (
		files []fortran.File
		prev  rms.FileLocation
	)

	for i, item := range group {
		var (
			loc rms.FileLocation
			err error
		)

		if i == 0 {
			loc, err = s.Locate(item.Name, opts.SourceHost)
		} else {
			loc, err = s.LocateRelated(item.Name, opts.SourceHost, prev)
		}

		if err != nil {
			return FileFailure(err, item.Name)
		}

		if item.Library {
			lib, found, err := includes.readLibrary(loc)
			if err != nil {
				return err
			}

			includes.named = append(includes.named, lib)
			prev = found

			continue
		}

		loc = loc.WithDefaultType(defaultFortranType)

		records, found, err := s.ReadRecordFile(loc, rms.TextRecords)
		if err != nil {
			return FileFailure(err, loc.Name)
		}

		prev = found

		files = append(files, fortran.File{Spec: sourceSpec(found), Name: sourceStem(found), Lines: recordLines(records), Handle: found})
	}

	if len(files) == 0 {
		return vmserrors.New(vmserrors.CLI_FORNOSOURCE, group[0].Name)
	}

	found, _ := files[0].Handle.(rms.FileLocation)

	continuations, _ := opts.Continuations.limit() // checked by Fortran
	align, _ := opts.alignment()                   // checked by Fortran

	result := fortran.CompileAll(fortran.Options{
		ExtendSource:  opts.ExtendSource,
		DLines:        opts.DLines,
		NoI4:          opts.NoI4,
		GFloating:     opts.GFloating,
		Align:         align,
		Continuations: continuations,
		Includes:      includes,
		Version:       BuildVersion,
		Compiled:      time.Now(),
		CommandLine:   opts.CommandLine,
	}, files)

	for _, m := range result.Messages {
		c.Printf("%s\n", m)
	}

	if result.Errors > 0 {
		return vmserrors.New(vmserrors.CLI_FORERRORS, result.Errors, found.Name)
	}

	if !opts.NoMacro {
		if err := c.writeMACRO(result.MACRO, opts.Macro, found); err != nil {
			return err
		}
	}

	if c.SetStatus != nil {
		if result.Warnings > 0 {
			c.SetStatus(vmserrors.CLI_FORWARNINGS | stsInhibitMsg)
		} else {
			c.SetStatus(ssNormal | stsInhibitMsg)
		}
	}

	return nil
}

// fortranIncludes reads the files and modules a compilation's INCLUDE
// statements name (source.Opener).
type fortranIncludes struct {
	c *Context

	// named are the libraries /LIBRARY named, in order.
	named []*includeLibrary

	// defaults are the libraries found by name, read at the first
	// INCLUDE '(module)' that gets past named; searched says they have been.
	defaults []*includeLibrary
	searched bool
}

// includeLibrary is a text library a compilation has read.
type includeLibrary struct {
	lib   *lbr.Library
	found rms.FileLocation
}

// readLibrary reads the text library at loc, of default type TLB.
func (fi *fortranIncludes) readLibrary(loc rms.FileLocation) (*includeLibrary, rms.FileLocation, error) {
	loc = loc.WithDefaultType(defaultTextLibraryType)

	data, found, err := fi.c.ContainerSession.ReadRawFile(loc)
	if err != nil {
		return nil, found, FileFailure(err, loc.Name)
	}

	lib, err := openTextLibrary(data, found)
	if err != nil {
		return nil, found, err
	}

	return lib, found, nil
}

// openTextLibrary makes a library's bytes an includeLibrary, if it is a
// text library.
func openTextLibrary(data []byte, found rms.FileLocation) (*includeLibrary, error) {
	lib, err := lbr.Open(data)
	if err != nil {
		return nil, vmserrors.Wrap(vmserrors.CLI_LIBRARY, err, found.Name)
	}

	if lib.Type != lbr.TypeText {
		err := fmt.Errorf("%s is a %s library, not a text library", found.Name, lib.Type)

		return nil, vmserrors.Wrap(vmserrors.CLI_LIBRARY, err, found.Name)
	}

	return &includeLibrary{lib: lib, found: found}, nil
}

// module returns the library's module of that name, as an included file, or
// nil if it has none. A name is tried as written, then in capitals, as
// LIBRARY writes a name given without quotes.
func (il *includeLibrary) module(name string) (*source.File, error) {
	rfa, ok := il.lib.Lookup(name)
	if !ok {
		name = strings.ToUpper(name)

		if rfa, ok = il.lib.Lookup(name); !ok {
			return nil, nil
		}
	}

	m, err := il.lib.Module(rfa)
	if err != nil {
		return nil, err
	}

	return &source.File{Spec: sourceSpec(il.found) + "(" + name + ")", Lines: recordLines(m.Records), Handle: il.found}, nil
}

// defaultLibraries finds the libraries searched after those /LIBRARY names:
// FORT$LIBRARY's, then SYS$LIBRARY:FORSYSDEF.TLB. A name that doesn't lead to
// a file adds nothing; one that leads to anything but a text library is an
// error, as a /LIBRARY's is.
func (fi *fortranIncludes) defaultLibraries() ([]*includeLibrary, error) {
	if fi.searched {
		return fi.defaults, nil
	}

	fi.searched = true

	s := fi.c.ContainerSession

	db := fi.c.Logicals
	if db == nil && s != nil {
		db = s.Logicals
	}

	if db != nil {
		for _, table := range []string{"LNM$PROCESS", "LNM$JOB", "LNM$GROUP", "LNM$SYSTEM"} {
			e, err := db.Translate(table, fortLibraryName, lnm.User, 0)
			if err != nil {
				continue
			}

			for _, equivalence := range e.Equivalences {
				loc, err := s.Locate(equivalence.Value, false)
				if err != nil {
					return nil, FileFailure(err, equivalence.Value)
				}

				lib, _, err := fi.readLibrary(loc)
				if err != nil {
					return nil, err
				}

				fi.defaults = append(fi.defaults, lib)
			}

			break
		}
	}

	data, found, err := fi.c.readLibraryFile("SYS$LIBRARY", forSysDefFile)
	if err != nil {
		return nil, FileFailure(err, forSysDefFile)
	}

	if data != nil {
		lib, err := openTextLibrary(data, found)
		if err != nil {
			return nil, err
		}

		fi.defaults = append(fi.defaults, lib)
	}

	return fi.defaults, nil
}

// OpenModule reads a text library's module (source.Opener).
func (fi *fortranIncludes) OpenModule(library, module string, from *source.File) (*source.File, error) {
	var search []*includeLibrary

	if library != "" {
		loc, err := fi.related(library, from)
		if err != nil {
			return nil, err
		}

		lib, _, err := fi.readLibrary(loc)
		if err != nil {
			return nil, err
		}

		search = []*includeLibrary{lib}
	} else {
		defaults, err := fi.defaultLibraries()
		if err != nil {
			return nil, err
		}

		search = append(append(search, fi.named...), defaults...)
	}

	for _, lib := range search {
		file, err := lib.module(module)
		if err != nil {
			return nil, err
		}

		if file != nil {
			return file, nil
		}
	}

	if library != "" {
		return nil, fmt.Errorf("the library has no module %s", module)
	}

	if lines, ok := sysdef.Module(module); ok {
		return &source.File{Spec: builtinModuleSpec + "(" + strings.ToUpper(module) + ")", Lines: lines, Handle: from.Handle}, nil
	}

	return nil, fmt.Errorf("no library has a module %s", module)
}

// OpenInclude reads the file named name, which the file from includes:
// name is resolved relative to from, with the default type FOR.
func (fi *fortranIncludes) OpenInclude(name string, from *source.File) (*source.File, error) {
	s := fi.c.ContainerSession

	loc, err := fi.related(name, from)
	if err != nil {
		return nil, err
	}

	loc = loc.WithDefaultType(defaultFortranType)

	records, found, err := s.ReadRecordFile(loc, rms.TextRecords)
	if err != nil {
		// A missing file is the commonest failure: its message alone,
		// without the reasons beneath it, reads best inside the
		// compiler's.
		var notFound *rms.NotFoundError
		if errors.As(err, &notFound) || errors.Is(err, fs.ErrNotExist) {
			return nil, vmserrors.New(vmserrors.SS_NOSUCHFILE, loc.Name)
		}

		return nil, FileFailure(err, loc.Name)
	}

	return &source.File{Spec: sourceSpec(found), Lines: recordLines(records), Handle: found}, nil
}

// related finds the file named name, relative to the file from.
func (fi *fortranIncludes) related(name string, from *source.File) (rms.FileLocation, error) {
	related, _ := from.Handle.(rms.FileLocation)

	loc, err := fi.c.ContainerSession.LocateRelated(name, false, related)
	if err != nil {
		return loc, FileFailure(err, name)
	}

	// A relative host path (inc/limits.for) is relative to the including
	// file's directory too, as a bare name is.
	if loc.Host && related.Host && strings.ContainsAny(name, `/\`) && !filepath.IsAbs(name) {
		loc.Name = filepath.Join(filepath.Dir(related.Name), name)
	}

	return loc, nil
}

// recordLines is a text file's records as strings.
func recordLines(records [][]byte) []string {
	lines := make([]string, len(records))
	for i, r := range records {
		lines[i] = string(r)
	}

	return lines
}

// writeMACRO writes the MACRO file lines for the source found, named
// name (or by default the source's name with the type MAR).
func (c *Context) writeMACRO(lines []string, name string, found rms.FileLocation) error {
	s := c.ContainerSession

	macLoc, err := outputLocation(s, name, found, "MAR")
	if err != nil {
		return FileFailure(err, name)
	}

	// A host source that is already a .MAR file would be its own default
	// MACRO file: refuse rather than overwrite the source. (On a volume
	// the MACRO file is a new version, and the source survives.)
	if macLoc.Host && found.Host && samePath(macLoc.Name, found.Name) {
		return vmserrors.New(vmserrors.CLI_FORSAMEFILE, macLoc.Name)
	}

	records := make([][]byte, len(lines))
	for i, line := range lines {
		records[i] = []byte(line)
	}

	if _, err := s.CreateRecordFile(macLoc, rms.TextRecords, records); err != nil {
		return FileFailure(err, macLoc.Name)
	}

	return nil
}

// sourceStem is the source file's name without its directory, type, or
// version: SQUARES for DUA0:[WORK]SQUARES.FOR;1 or /tmp/squares.for.
func sourceStem(found rms.FileLocation) string {
	if found.Host {
		base := filepath.Base(found.Name)

		return strings.TrimSuffix(base, filepath.Ext(base))
	}

	name, _ := rms.SplitVersion(found.Name)
	name = rms.NameType(name)

	if i := strings.LastIndexByte(name, '.'); i >= 0 {
		name = name[:i]
	}

	return name
}

// samePath reports whether two host paths name the same file, as far as
// their absolute, cleaned forms show (case matters, as it does to the
// host's file system on most systems).
func samePath(a, b string) bool {
	absA, errA := filepath.Abs(a)
	absB, errB := filepath.Abs(b)

	return errA == nil && errB == nil && absA == absB
}
