package dclcmd

import (
	"errors"
	"path/filepath"
	"strings"
	"time"

	"github.com/tucats/govax/internal/fortran"
	"github.com/tucats/govax/internal/fortran/source"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmserrors"
)

// This file is the FORTRAN command (docs/PHASE-74 - FORTRAN plan.md,
// Decision 18; subtask 2):
//
//	FORTRAN source[,...][/HOST] [/MACRO[=file] | /NOMACRO]
//	        [/[NO]EXTEND_SOURCE] [/[NO]D_LINES] [/CONTINUATIONS=n]
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
// status is the last failure's.
//
// A source's INCLUDE 'file' statements name files found as MACRO's
// .INCLUDE finds them: relative to the file that includes them (on the
// host, in its directory, for a bare name or a relative path; on a
// volume, on its device and in its directory), with the default type FOR
// (subtask 3). Whether VAX
// FORTRAN looks beside the including file or in the default directory
// isn't in the manuals to hand; for a source in the default directory
// the two agree. govax's choice, unconfirmed.

// FortranOptions is one FORTRAN command.
type FortranOptions struct {
	// Sources are the source files, in the order given, and SourceHost
	// an explicit /HOST on them.
	Sources    []string
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

	// Continuations is /CONTINUATIONS=n.
	Continuations FortranContinuations

	// CommandLine is the command as typed, for the MACRO file's header.
	CommandLine string
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

// defaultFortranType is the file type FORTRAN gives a source named
// without one, as VMS's FORTRAN command does.
const defaultFortranType = "FOR"

// Fortran compiles each source file and, unless opts.NoMacro, writes its
// MACRO file.
func (c *Context) Fortran(opts FortranOptions) error {
	if _, err := opts.Continuations.limit(); err != nil {
		return err
	}

	if len(opts.Sources) == 1 {
		return c.compileFortran(opts.Sources[0], opts)
	}

	// With several sources, each failure is shown as it happens, so the
	// next file's messages follow it in order, and the command ends with
	// the last failure's status, not shown again.
	var failed uint32

	for _, source := range opts.Sources {
		if err := c.compileFortran(source, opts); err != nil {
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

// compileFortran compiles one source file.
func (c *Context) compileFortran(source string, opts FortranOptions) error {
	s := c.ContainerSession

	loc, err := s.Locate(source, opts.SourceHost)
	if err != nil {
		return FileFailure(err, source)
	}

	loc = loc.WithDefaultType(defaultFortranType)

	records, found, err := s.ReadRecordFile(loc, rms.TextRecords)
	if err != nil {
		return FileFailure(err, loc.Name)
	}

	lines := recordLines(records)

	continuations, _ := opts.Continuations.limit() // checked by Fortran

	result := fortran.Compile(fortran.Options{
		ExtendSource:  opts.ExtendSource,
		DLines:        opts.DLines,
		Continuations: continuations,
		Includes:      fortranIncludes{c},
		Version:       BuildVersion,
		Compiled:      time.Now(),
		CommandLine:   opts.CommandLine,
	}, fortran.File{
		Spec:   sourceSpec(found),
		Name:   sourceStem(found),
		Lines:  lines,
		Handle: found,
	})

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

// fortranIncludes reads the files a source's INCLUDE statements name
// (source.Opener).
type fortranIncludes struct {
	c *Context
}

// OpenInclude reads the file named name, which the file from includes:
// name is resolved relative to from, with the default type FOR.
func (fi fortranIncludes) OpenInclude(name string, from *source.File) (*source.File, error) {
	s := fi.c.ContainerSession

	related, _ := from.Handle.(rms.FileLocation)

	loc, err := s.LocateRelated(name, false, related)
	if err != nil {
		return nil, FileFailure(err, name)
	}

	// A relative host path (inc/limits.for) is relative to the including
	// file's directory too, as a bare name is.
	if loc.Host && related.Host && strings.ContainsAny(name, `/\`) && !filepath.IsAbs(name) {
		loc.Name = filepath.Join(filepath.Dir(related.Name), name)
	}

	loc = loc.WithDefaultType(defaultFortranType)

	records, found, err := s.ReadRecordFile(loc, rms.TextRecords)
	if err != nil {
		return nil, FileFailure(err, loc.Name)
	}

	return &source.File{Spec: sourceSpec(found), Lines: recordLines(records), Handle: found}, nil
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
