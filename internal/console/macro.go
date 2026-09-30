package console

import (
	"errors"
	"strings"

	"github.com/tucats/govax/internal/asm"
	"github.com/tucats/govax/internal/obj"
	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmserrors"
)

// This file implements docs/PHASE-27.md subtask 10's MACRO command:
//
//	MACRO source[/HOST] [/[NO]OBJECT[=object]]
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
// Every assembly error is reported, and then no object file is written:
// the object is built in memory and written only once assembly succeeds,
// so a failed assembly never leaves a partial object or replaces a good
// one.

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

	// CommandLine is the command as typed, for the object's SRC header,
	// where real MACRO records its command line.
	CommandLine string
}

// defaultSourceType is the file type MACRO gives a source named without
// one, as VMS MACRO does.
const defaultSourceType = "MAR"

// Macro assembles one MACRO-32 source file and, unless opts.NoObject,
// writes its object module.
func (c *Console) Macro(opts MacroOptions) error {
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
	a.SetIncludeResolver(func(name string) (string, error) {
		incLoc, err := s.LocateRelated(name, false, found)
		if err != nil {
			return "", err
		}

		text, _, err := s.ReadRecordFile(incLoc, rms.TextRecords)

		return joinLines(text), err
	})

	_, asmErr := a.Assemble(joinLines(lines))

	for _, m := range a.Messages() {
		c.Printf("%s\n", m)
	}

	for _, w := range a.Warnings() {
		c.Printf("%%%s\n", vmserrors.Wrap(vmserrors.CLI_ASMWARNING, w, found.Name))
	}

	if asmErr != nil {
		all := []error{asmErr}

		var list *asm.Errors
		if errors.As(asmErr, &list) {
			all = list.List
		}

		for _, e := range all {
			c.Printf("%%%s\n", vmserrors.Wrap(vmserrors.CLI_ASSEMBLING, e, found.Name))
		}

		return vmserrors.New(vmserrors.CLI_ASMERRORS, len(all), found.Name)
	}

	if opts.NoObject {
		return nil
	}

	objLoc, err := outputLocation(s, opts.Object, found, "OBJ")
	if err != nil {
		return fileFailure(err, opts.Object)
	}

	module, err := a.Object(asm.ObjectOptions{
		Language: "govax MACRO V" + BuildVersion,
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
