package rms

import (
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// The naming rules VMS commands share for an input file named without a
// type, and the pieces of a specification they look at.

// WithDefaultType gives an input file named without a file type the
// type typ, as VMS commands do (MACRO's .MAR, LINK's .OBJ). On the host, a file
// that exists under the name as given is used as it is, and the added
// extension is lowercase unless the name has an uppercase letter.
func (loc FileLocation) WithDefaultType(typ string) FileLocation {
	if loc.Host {
		if filepath.Ext(loc.Name) != "" {
			return loc
		}

		if _, err := os.Stat(loc.Name); err == nil {
			return loc
		}

		loc.Name += "." + MatchCase("", typ, filepath.Base(loc.Name))

		return loc
	}

	name, version := SplitVersion(loc.Name)
	if !strings.Contains(NameType(name), ".") {
		loc.Name = name + "." + typ + version
	}

	return loc
}

// MatchCase returns word in the case of model, letter by letter: each
// letter of word takes the case of model's letter at the same position,
// or of model's last letter past its end. With no model, word is
// lowercase if whole (the file name) has no uppercase letters, and
// uppercase otherwise.
func MatchCase(model, word, whole string) string {
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

// SplitVersion splits a VMS file specification's ";version" off,
// returning the rest and the version with its ";".
func SplitVersion(spec string) (rest, version string) {
	if i := strings.LastIndexByte(spec, ';'); i >= 0 {
		return spec[:i], spec[i:]
	}

	return spec, ""
}

// NameType is the name and type of a VMS file specification with no
// version: what follows its device and directory.
func NameType(spec string) string {
	if i := strings.LastIndexAny(spec, "]>:"); i >= 0 {
		return spec[i+1:]
	}

	return spec
}
