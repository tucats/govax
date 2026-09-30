package console

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode"

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

	loc = withDefaultType(loc)

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

	objLoc, err := objectLocation(s, opts.Object, found)
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

// withDefaultType gives a source named without a file type the type
// .MAR. On the host, a file that exists under the name as given is used
// as it is, and the added extension is lowercase unless the name has an
// uppercase letter.
func withDefaultType(loc rms.FileLocation) rms.FileLocation {
	if loc.Host {
		if filepath.Ext(loc.Name) != "" {
			return loc
		}

		if _, err := os.Stat(loc.Name); err == nil {
			return loc
		}

		loc.Name += "." + matchCase("", defaultSourceType, filepath.Base(loc.Name))

		return loc
	}

	name, version := splitVersion(loc.Name)
	if !strings.Contains(vmsNameType(name), ".") {
		loc.Name = name + "." + defaultSourceType + version
	}

	return loc
}

// objectLocation is where the object goes: the /OBJECT= name, or by
// default the source's name with the type OBJ, next to the source. An
// /OBJECT= that names only a directory (an existing host directory, or a
// VMS specification with no name or type) gets the default name in it.
func objectLocation(s *rms.Session, name string, source rms.FileLocation) (rms.FileLocation, error) {
	if name == "" {
		return defaultObjectLocation(s, source)
	}

	loc, err := s.LocateRelated(name, false, source)
	if err != nil {
		return loc, err
	}

	if loc.Host {
		if info, err := os.Stat(loc.Name); err == nil && info.IsDir() {
			def, err := defaultObjectLocation(s, source)
			loc.Name = filepath.Join(loc.Name, filepath.Base(def.Name))

			return loc, err
		}

		return loc, nil
	}

	if rest, _ := splitVersion(loc.Name); vmsNameType(rest) == "" {
		def, err := defaultObjectLocation(s, source)
		loc.Name = rest + vmsNameType(def.Name)

		return loc, err
	}

	return loc, nil
}

// defaultObjectLocation is the source's name with the type OBJ, next to
// the source.
func defaultObjectLocation(s *rms.Session, source rms.FileLocation) (rms.FileLocation, error) {

	if source.Host {
		base := filepath.Base(source.Name)
		ext := filepath.Ext(base)
		stem := strings.TrimSuffix(base, ext)
		ext = strings.TrimPrefix(ext, ".")

		return rms.FileLocation{Host: true, Name: filepath.Join(filepath.Dir(source.Name), stem+"."+matchCase(ext, "OBJ", base))}, nil
	}

	// The version is dropped, so the object gets the next version of its
	// own name.
	name, _ := splitVersion(source.Name)
	nameType := vmsNameType(name)

	if i := strings.LastIndexByte(nameType, '.'); i >= 0 {
		nameType = nameType[:i]
	}

	return s.LocateRelated(nameType+".OBJ", false, source)
}

// matchCase returns word in the case of model, letter by letter: each
// letter of word takes the case of model's letter at the same position,
// or of model's last letter past its end. With no model, word is
// lowercase if whole (the file name) has no uppercase letters, and
// uppercase otherwise.
func matchCase(model, word, whole string) string {
	if model == "" {
		if strings.ToLower(whole) == whole {
			return strings.ToLower(word)
		}

		return strings.ToUpper(word)
	}

	m := []rune(model)
	out := []rune(word)

	for i := range out {
		ref := m[min(i, len(m)-1)]
		if unicode.IsUpper(ref) {
			out[i] = unicode.ToUpper(out[i])
		} else {
			out[i] = unicode.ToLower(out[i])
		}
	}

	return string(out)
}

// splitVersion splits a VMS file specification's ";version" off,
// returning the rest and the version with its ";".
func splitVersion(spec string) (rest, version string) {
	if i := strings.LastIndexByte(spec, ';'); i >= 0 {
		return spec[:i], spec[i:]
	}

	return spec, ""
}

// vmsNameType is the name and type of a VMS file specification with no
// version: what follows its device and directory.
func vmsNameType(spec string) string {
	if i := strings.LastIndexAny(spec, "]>:"); i >= 0 {
		return spec[i+1:]
	}

	return spec
}

// fileFailure turns an error finding, reading, or creating a file into
// the console's status for it, as TYPE and COPY report the same
// failures.
func fileFailure(err error, spec string) error {
	if lnmErr := logicalNameFailure(err); lnmErr != nil {
		return lnmErr
	}

	var notMounted *rms.NotMountedError
	if errors.As(err, &notMounted) {
		return vmserrors.Wrap(vmserrors.SS_DEVNOTMOUNT, err, notMounted.Device)
	}

	var notFound *rms.NotFoundError
	if errors.As(err, &notFound) || errors.Is(err, fs.ErrNotExist) {
		return vmserrors.Wrap(vmserrors.SS_NOSUCHFILE, err, spec)
	}

	var ambiguous *rms.AmbiguousError
	if errors.As(err, &ambiguous) {
		return vmserrors.Wrap(vmserrors.CLI_AMBIGUOUS, err, "file specification", spec)
	}

	return vmserrors.Wrap(vmserrors.CLI_BADFILESPEC, err, spec)
}

// objectFailure turns an error creating the object file into the
// console's status for it: an unmounted device or a bad logical name as
// fileFailure reports them, and anything else as CLI_OBJWRITE.
func objectFailure(err error, spec string) error {
	var notMounted *rms.NotMountedError
	if logicalNameFailure(err) != nil || errors.As(err, &notMounted) {
		return fileFailure(err, spec)
	}

	return vmserrors.Wrap(vmserrors.CLI_OBJWRITE, err, spec)
}
