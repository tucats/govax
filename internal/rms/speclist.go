package rms

import "strings"

// Lists of file specifications, as a DCL command takes them
// (DIRECTORY A.LIS,B.LIS). The DCL Dictionary's DIRECTORY separates the
// elements with commas or plus signs, and the OpenVMS User's Manual,
// 4.3.3 ("Using Temporary Defaults"), says how an element that isn't a
// whole specification is completed: the node, device, and directory of
// the element before it are its temporary defaults.
//
//	COPY BASE:[STATS]A.LIS,[TIME]B.LIS,C.LIS [RESULTS]
//
// copies BASE:[STATS]A.LIS, BASE:[TIME]B.LIS, and BASE:[TIME]C.LIS. Empty
// brackets ([]) name the current default directory instead. The file
// name and type are not carried.

// SplitSpecList splits text, a list of file specifications, at each
// comma or plus sign outside quotes, and trims the blanks around each
// element. A quoted element (a host file name) may contain either.
func SplitSpecList(text string) []string {
	var (
		elements []string
		quoted   bool
		start    int
	)

	for i := 0; i < len(text); i++ {
		switch ch := text[i]; {
		case ch == '"':
			quoted = !quoted
		case !quoted && (ch == ',' || ch == '+'):
			elements = append(elements, strings.TrimSpace(text[start:i]))
			start = i + 1
		}
	}

	return append(elements, strings.TrimSpace(text[start:]))
}

// TemporaryDefaults completes each element of a list of file
// specifications with its temporary defaults: the node, device, and
// directory the element before it had, written or carried, where it
// gives none of its own. An element that names a device but no directory
// carries no directory (govax's choice, unconfirmed: a device may be a
// logical name whose translation has a directory of its own). A quoted
// element, a host file name, is left alone and carries nothing to the
// next one.
func TemporaryDefaults(elements []string) []string {
	out := make([]string, len(elements))

	var node, device, directory string

	for i, element := range elements {
		if strings.HasPrefix(element, `"`) {
			out[i] = element
			node, device, directory = "", "", ""

			continue
		}

		n, d, dir, rest := splitSpecPrefix(element)

		switch {
		case n != "":
			node, device, directory = n, d, dir
		case d != "":
			device, directory = d, dir
		case dir != "":
			directory = dir
		}

		// [] is the current default directory: written out, it stays
		// empty brackets, which RMS's parse reads as the default, and it
		// carries as a directory that is the default's.
		out[i] = node + device + directory + rest
	}

	return out
}

// splitSpecPrefix splits a file specification into its node (with its
// "::"), device (with its ":"), and directory (in [] or <>), and the rest:
// name, type, and version.
func splitSpecPrefix(spec string) (node, device, directory, rest string) {
	rest = spec

	if i := strings.Index(rest, "::"); i >= 0 {
		node, rest = rest[:i+2], rest[i+2:]
	}

	if i := strings.IndexByte(rest, ':'); i >= 0 && !strings.ContainsAny(rest[:i], "[<") {
		device, rest = rest[:i+1], rest[i+1:]
	}

	if strings.HasPrefix(rest, "[") || strings.HasPrefix(rest, "<") {
		closer := "]"
		if rest[0] == '<' {
			closer = ">"
		}

		if i := strings.Index(rest, closer); i >= 0 {
			directory, rest = rest[:i+1], rest[i+1:]
		}
	}

	return node, device, directory, rest
}
