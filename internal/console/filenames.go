package console

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/tucats/govax/internal/rms"
	"github.com/tucats/govax/internal/vmserrors"
)

// This file holds the file naming rules MACRO and LINK share: an input's
// default file type, an output's default name (the input's, with the
// output's type, in the case of the input's type), and how a failure to
// find or read a file is reported.

// withDefaultType gives an input file named without a file type the type
// typ, as VMS commands do (MACRO's .MAR, LINK's .OBJ). On the host, a file
// that exists under the name as given is used as it is, and the added
// extension is lowercase unless the name has an uppercase letter.
func withDefaultType(loc rms.FileLocation, typ string) rms.FileLocation {
	if loc.Host {
		if filepath.Ext(loc.Name) != "" {
			return loc
		}

		if _, err := os.Stat(loc.Name); err == nil {
			return loc
		}

		loc.Name += "." + matchCase("", typ, filepath.Base(loc.Name))

		return loc
	}

	name, version := splitVersion(loc.Name)
	if !strings.Contains(vmsNameType(name), ".") {
		loc.Name = name + "." + typ + version
	}

	return loc
}

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

	if rest, _ := splitVersion(loc.Name); vmsNameType(rest) == "" {
		def, err := defaultOutputLocation(s, input, typ)
		loc.Name = rest + vmsNameType(def.Name)

		return loc, err
	}

	// A VMS name without a type gets typ, as VMS gives /OBJECT=NAME the
	// type OBJ.
	return withDefaultType(loc, typ), nil
}

// defaultOutputLocation is the input's name with the type typ, next to
// the input, with the type in the case of the input's type.
func defaultOutputLocation(s *rms.Session, source rms.FileLocation, typ string) (rms.FileLocation, error) {
	if source.Host {
		base := filepath.Base(source.Name)
		ext := filepath.Ext(base)
		stem := strings.TrimSuffix(base, ext)
		ext = strings.TrimPrefix(ext, ".")

		return rms.FileLocation{Host: true, Name: filepath.Join(filepath.Dir(source.Name), stem+"."+matchCase(ext, typ, base))}, nil
	}

	// The version is dropped, so the output gets the next version of its
	// own name.
	name, _ := splitVersion(source.Name)
	nameType := vmsNameType(name)

	if i := strings.LastIndexByte(nameType, '.'); i >= 0 {
		nameType = nameType[:i]
	}

	return s.LocateRelated(nameType+"."+typ, false, source)
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

