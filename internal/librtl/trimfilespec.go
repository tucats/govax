package librtl

import (
	"strings"

	"github.com/tucats/govax/internal/corevms"
)

// LIB$TRIM_FILESPEC (docs/PHASE-58, subtask 7; the survey's
// "TRIM_FILESPEC (461)"), written from the 5.0 LIB$ manual (LIB-461):
//
//	LIB$TRIM_FILESPEC old-filespec ,new-filespec [,word-integer-width]
//	                  [,resultant-length]
//
// It fits a file specification into a field of a given width, as a
// listing's header does, by leaving parts out, in this order, until it
// fits: the node (with its access control), the device, the directory,
// the version, and the type. If the name alone is still too wide, it is
// cut off, and the status is LIB$_STRTRU. The manual's example: width 32
// makes "DISK$NAME:[DIRECTORY1.DIRECTORY2]FILENAME.EXTENSTION;1"
// "FILENAME.EXTENSTION;1", leaving the device and directory out.
//
// VMS splits the name with $FILESCAN, which govax doesn't have yet;
// splitFileSpec finds the same parts.

// maxTrimSpec is the longest old-filespec the manual allows.
const maxTrimSpec = 255

// libTrimFilespec is LIB$TRIM_FILESPEC. new-filespec is written by its
// class; the width is word-integer-width, else new-filespec's current
// length; resultant-length gets the characters written, padding and
// truncated characters not counted.
//
// Choices the manual leaves open, each **unconfirmed** (probe cases):
//
//   - Each part goes only while what's left is too wide: the first that
//     makes it fit is the last left out, and a specification that fits
//     already is unchanged.
//   - A width of 0 (given, or a dynamic new-filespec's length 0) leaves
//     nothing: the name cut to nothing, LIB$_STRTRU unless it was empty.
//
// Statuses: SS$_NORMAL; LIB$_STRTRU when the name is cut off or the
// result doesn't fit new-filespec; LIB$_INVARG for an old-filespec of
// more than 255 characters; LIB$_WRONUMARG for fewer than two arguments
// or more than four; LIB$_INVSTRDES and the rest of the string routines'.
// An argument it can't read or write is an access violation, signaled.
func libTrimFilespec(env *corevms.Environment, argv []uint32) (uint32, error) {
	if len(argv) < 2 || len(argv) > 4 {
		return libWroNumArg, nil
	}

	old, err := readString(env, argv[0])
	if err != nil {
		return stringFailure(env, err)
	}

	if len(old) > maxTrimSpec {
		return libInvArg, nil
	}

	var width uint32

	if a := arg(argv, 2); a != 0 {
		w, err := env.Memory().LoadWord(env.CPU(), a)
		if err != nil {
			return memoryFault(env, err)
		}

		width = uint32(w)
	} else {
		d, err := readDescriptor(env, argv[1])
		if err != nil {
			return stringFailure(env, err)
		}

		width = d.length
	}

	trimmed, cut := trimFileSpec(old, int(width))

	stored, truncated, err := writeString(env, argv[1], trimmed)
	if err != nil {
		return stringFailure(env, err)
	}

	if a := arg(argv, 3); a != 0 {
		if err := env.Memory().StoreWord(env.CPU(), a, uint16(stored)); err != nil {
			return memoryFault(env, writing(err))
		}
	}

	if cut || truncated {
		return libStrTru, nil
	}

	return ssNormal, nil
}

// trimFileSpec fits spec into width characters, leaving out its node,
// device, directory, version, and type in turn until it fits, then
// cutting the name. cut reports that the name was cut.
func trimFileSpec(spec string, width int) (trimmed string, cut bool) {
	if len(spec) <= width {
		return spec, false
	}

	f := splitFileSpec(spec)

	for _, part := range []*string{&f.node, &f.device, &f.directory, &f.version, &f.typ} {
		*part = ""

		if s := f.String(); len(s) <= width {
			return s, false
		}
	}

	return f.name[:width], true
}

// fileSpecParts is a file specification split into the parts $FILESCAN
// finds, each with its punctuation: "NODE::", "DUA1:", "[DIR]", "NAME",
// ".TYP", ";3".
type fileSpecParts struct {
	node, device, directory, name, typ, version string
}

// String is the specification the parts make.
func (f fileSpecParts) String() string {
	return f.node + f.device + f.directory + f.name + f.typ + f.version
}

// splitFileSpec splits spec into its parts. The node is everything up to
// the last "::" (each node with its access control, in quotes); the
// device runs to a ":"; the directory is one or more bracketed parts,
// "[...]" or "<...>" ("[A.][B]", a rooted directory); the name runs to a
// "." or ";", the type from that "." to a ";" or another "."; the version
// is the rest. A part that isn't there is "". Text it can't place (an
// unmatched bracket) is the name's, so nothing is lost.
func splitFileSpec(spec string) fileSpecParts {
	var f fileSpecParts

	rest := spec

	// The node: up to the last "::" outside quotes, before any
	// directory.
	quoted := false
	nodeEnd := 0

	for i := 0; i < len(rest); i++ {
		c := rest[i]

		switch {
		case c == '"':
			quoted = !quoted
		case quoted:
		case c == '[' || c == '<':
			i = len(rest)
		case c == ':' && i+1 < len(rest) && rest[i+1] == ':':
			nodeEnd = i + 2
			i++
		}
	}

	f.node, rest = rest[:nodeEnd], rest[nodeEnd:]

	// The device: up to a ":" before the directory, name, type, or
	// version begins.
	if i := strings.IndexAny(rest, ":[<.;"); i >= 0 && rest[i] == ':' {
		f.device, rest = rest[:i+1], rest[i+1:]
	}

	// The directory: bracketed parts, one after another.
	for rest != "" && (rest[0] == '[' || rest[0] == '<') {
		end := strings.IndexAny(rest, "]>")
		if end < 0 {
			break
		}

		f.directory += rest[:end+1]
		rest = rest[end+1:]
	}

	// The name, type, and version.
	i := strings.IndexAny(rest, ".;")
	if i < 0 {
		f.name = rest

		return f
	}

	f.name, rest = rest[:i], rest[i:]

	if rest[0] == '.' {
		j := strings.IndexAny(rest[1:], ".;")
		if j < 0 {
			f.typ = rest

			return f
		}

		f.typ, rest = rest[:j+1], rest[j+1:]
	}

	f.version = rest

	return f
}
