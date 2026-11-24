package dclcmd

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmserrors"
)

// This file holds the file naming rules MACRO, LINK, and the other
// utilities share: an input's default file type, an output's default name
// (the input's, with the output's type, in the case of the input's type,
// in the default directory), and how a failure to find or read a file is
// reported.

// outputLocation is where an output file goes: the name given (MACRO's
// /OBJECT=, LINK's /EXECUTABLE=), or by default the input's name with the
// type typ, in the process's default directory. A name that is only a
// directory (an existing host directory, or a VMS specification with no
// name or type) gets the default name in it, and a VMS name with no type
// gets typ.
//
// The input is a related file only for the name and type, as RMS's output
// file parse (FAB$V_OFP) uses one: its device and directory never carry
// over, so MACRO [SRC]A and MACRO [SRC]A/OBJECT=B both write into the
// default directory, as VMS's utilities do, not into [SRC].
//
// A host input's default directory is the host's current directory, by
// the author's choice: a host file in, a host file out, as a Unix compiler
// writes src/a.c's object in the current directory, so that a one-shot
// build from host files works whether or not the session's default
// directory is on a volume. A bare output name given for a host input
// goes there too.
//
// An output that ends up on the host, by any of these rules, is one the
// process must be allowed to make there (vax.host.sysprv): a refusal is
// an *rms.DeniedError with Host and Output set.
func outputLocation(s *rms.Session, name string, input rms.FileLocation, typ string) (rms.FileLocation, error) {
	loc, err := placeOutput(s, name, input, typ)
	if err == nil && loc.Host && !s.MayUseHost() {
		return rms.FileLocation{}, &rms.DeniedError{Name: loc.Name, Host: true, Output: true}
	}

	return loc, err
}

// placeOutput is outputLocation's rules, without its question of whether
// the process may make a host file.
func placeOutput(s *rms.Session, name string, input rms.FileLocation, typ string) (rms.FileLocation, error) {
	if name == "" {
		return defaultOutputLocation(s, input, typ)
	}

	loc, err := s.Side(name, false)
	if err != nil {
		return loc, err
	}

	// A bare name (not a logical name, which stands for its translation)
	// given for a host input is in the host's current directory.
	if input.Host && rms.BareName(name) && loc.Name == name {
		loc = rms.FileLocation{Host: true, Name: inCurrentDirectory(name)}
	}

	if loc.Host {
		if info, err := os.Stat(loc.Name); err == nil && info.IsDir() {
			loc.Name = filepath.Join(loc.Name, outputFileName(input, typ))

			return loc, nil
		}

		return loc, nil
	}

	if rest, _ := rms.SplitVersion(loc.Name); rms.NameType(rest) == "" {
		loc.Name = rest + strings.ToUpper(outputFileName(input, typ))

		return loc, nil
	}

	// A VMS name without a type gets typ, as VMS gives /OBJECT=NAME the
	// type OBJ.
	return loc.WithDefaultType(typ), nil
}

// defaultOutputLocation is the input's name with the type typ, in the
// default directory: for a volume input the process's (on the default
// volume if there is one, otherwise the host's current directory), and
// for a host input the host's current directory.
func defaultOutputLocation(s *rms.Session, source rms.FileLocation, typ string) (rms.FileLocation, error) {
	name := outputFileName(source, typ)

	if source.Host || !s.DefaultOnVolume() {
		return rms.FileLocation{Host: true, Name: inCurrentDirectory(name)}, nil
	}

	return rms.FileLocation{Name: strings.ToUpper(name)}, nil
}

// inCurrentDirectory is name, a host file's, in the host's current
// directory as a whole path, so that what names the file (a map's "in
// file" line, a message) says where it is, as a VMS file specification
// would; name as it is if there's no current directory to be had.
func inCurrentDirectory(name string) string {
	if path, err := filepath.Abs(name); err == nil {
		return path
	}

	return name
}

// outputFileName is the input's name with the type typ and no directory or
// version: "A.OBJ" for DUA1:[SRC]A.MAR;3. A host input's type is given in
// the case of its own ("a.obj" for "src/a.mar"), so that a host directory
// of lowercase names stays one.
func outputFileName(source rms.FileLocation, typ string) string {
	if source.Host {
		base := filepath.Base(source.Name)
		ext := filepath.Ext(base)
		stem := strings.TrimSuffix(base, ext)
		ext = strings.TrimPrefix(ext, ".")

		return stem + "." + rms.MatchCase(ext, typ, base)
	}

	name, _ := rms.SplitVersion(source.Name)
	nameType := rms.NameType(name)

	if i := strings.LastIndexByte(nameType, '.'); i >= 0 {
		nameType = nameType[:i]
	}

	return nameType + "." + typ
}

// FileFailure turns an error finding, reading, or creating a file into
// the console's status for it, as TYPE and COPY report the same
// failures.
//
// A refusal by the process's protection is a *DeniedFailure, which the
// utility running turns into its own OPENIN or OPENOUT (openFailure).
func FileFailure(err error, spec string) error {
	if lnmErr := logicalNameFailure(err); lnmErr != nil {
		return lnmErr
	}

	if denied, ok := rms.IsDenied(err); ok {
		f := &DeniedFailure{Name: strings.ToUpper(spec), err: err}
		if denied != nil {
			f.Output = denied.Output

			if denied.Name != "" {
				f.Name = denied.Name
			}
		}

		return f
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

	// A volume with no room for what was being written is RMS$_FUL, not a
	// bad specification, with no STV, as VMS 7.1's utilities showed it
	// (testdata/err113's FULL).
	if rms.DeviceFull(err) {
		text := strings.TrimPrefix(ConditionLine(rmsFUL), "-")

		return vmserrors.ConditionError{Status: rmsFUL, Text: text}
	}

	return vmserrors.Wrap(vmserrors.CLI_BADFILESPEC, err, spec)
}

// expandedName is specText as RMS's $PARSE expands it ("DUA1:[A]B.DAT;"),
// which a utility's message names a file by when it couldn't find the
// file itself; the text uppercased if it can't be expanded.
func (c *Context) expandedName(specText string) string {
	name, err := c.ContainerSession.ExpandName(specText)
	if err != nil {
		return strings.ToUpper(specText)
	}

	return name
}

// hostDenied is err's *rms.DeniedError if err is a refusal of a host file
// (vax.host.sysprv: docs/planning/PHASE-97 - host file access.md), nil
// otherwise: for a command that shows that refusal in its own words.
func hostDenied(err error) *rms.DeniedError {
	if denied, ok := rms.IsDenied(err); ok && denied != nil && denied.Host {
		return denied
	}

	return nil
}

// DeniedFailure is a file a utility's process may not open, read, or make
// (docs/planning/PHASE-93 - file protection.md, subtask 9): Name is the
// file's whole name, with its version if it was found, and Output says it
// was one the utility was making. A utility reports it with its own
// facility's message (openFailure); one that doesn't shows RMS$_PRV.
type DeniedFailure struct {
	Name   string
	Output bool
	err    error
}

func (e *DeniedFailure) Error() string {
	return strings.TrimPrefix(ConditionLine(rmsPRV), "-")
}

func (e *DeniedFailure) Unwrap() error { return e.err }

// openFailure is err as the utility whose message prefix is prefix
// ("MACRO-E", "LINK-F") reports a file its process's protection refused,
// as VMS 7.1's did (the probe's A.4, "Statuses and messages"):
//
//	%MACRO-E-OPENIN, error opening DUA1:[A]B.MAR;1 as input
//	-RMS-E-PRV, insufficient privilege or file protection violation
//
// OPENOUT and "as output" for a file it was making. With bare, an input's
// name is shown without its version number, as LIBRARY's was
// ("ULIBNONE.OLB;"). Any other err is returned as it is.
func openFailure(err error, prefix string, bare bool) error {
	var denied *DeniedFailure
	if !errors.As(err, &denied) {
		return err
	}

	name, what, as := denied.Name, "OPENIN", "input"

	switch {
	case denied.Output:
		what, as = "OPENOUT", "output"
	case bare:
		if i := strings.LastIndexByte(name, ';'); i >= 0 {
			name = name[:i+1]
		}
	}

	return vmserrors.ConditionError{
		Status: rmsPRV,
		Text:   prefix + "-" + what + ", error opening " + name + " as " + as + "\n" + ConditionLine(rmsPRV),
	}
}
