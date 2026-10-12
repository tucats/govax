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

// This file holds the file naming rules MACRO and LINK share: an input's
// default file type, an output's default name (the input's, with the
// output's type, in the case of the input's type), and how a failure to
// find or read a file is reported.

// outputLocation is where an output file goes: the name given (MACRO's
// /OBJECT=, LINK's /EXECUTABLE=), or by default the input's name with the
// type typ, next to the input. A name that is only a directory (an
// existing host directory, or a VMS specification with no name or type)
// gets the default name in it, and a VMS name with no type gets typ.
func outputLocation(s *rms.Session, name string, input rms.FileLocation, typ string) (rms.FileLocation, error) {
	if name == "" {
		return defaultOutputLocation(s, input, typ)
	}

	loc, err := s.LocateRelated(name, false, input)
	if err != nil {
		return loc, err
	}

	if loc.Host {
		if info, err := os.Stat(loc.Name); err == nil && info.IsDir() {
			def, err := defaultOutputLocation(s, input, typ)
			loc.Name = filepath.Join(loc.Name, filepath.Base(def.Name))

			return loc, err
		}

		return loc, nil
	}

	if rest, _ := rms.SplitVersion(loc.Name); rms.NameType(rest) == "" {
		def, err := defaultOutputLocation(s, input, typ)
		loc.Name = rest + rms.NameType(def.Name)

		return loc, err
	}

	// A VMS name without a type gets typ, as VMS gives /OBJECT=NAME the
	// type OBJ.
	return loc.WithDefaultType(typ), nil
}

// defaultOutputLocation is the input's name with the type typ, next to
// the input, with the type in the case of the input's type.
func defaultOutputLocation(s *rms.Session, source rms.FileLocation, typ string) (rms.FileLocation, error) {
	if source.Host {
		base := filepath.Base(source.Name)
		ext := filepath.Ext(base)
		stem := strings.TrimSuffix(base, ext)
		ext = strings.TrimPrefix(ext, ".")

		return rms.FileLocation{Host: true, Name: filepath.Join(filepath.Dir(source.Name), stem+"."+rms.MatchCase(ext, typ, base))}, nil
	}

	// The version is dropped, so the output gets the next version of its
	// own name.
	name, _ := rms.SplitVersion(source.Name)
	nameType := rms.NameType(name)

	if i := strings.LastIndexByte(nameType, '.'); i >= 0 {
		nameType = nameType[:i]
	}

	return s.LocateRelated(nameType+"."+typ, false, source)
}

// FileFailure turns an error finding, reading, or creating a file into
// the console's status for it, as TYPE and COPY report the same
// failures.
func FileFailure(err error, spec string) error {
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
