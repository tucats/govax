package lnm

import (
	"errors"

	"github.com/tucats/govax/internal/vmserrors"
)

// FileDevName is the logical name whose tables file-specification
// translation searches: by default LNM$PROCESS, LNM$GROUP, LNM$SYSTEM.
const FileDevName = "LNM$FILE_DEV"

// FileSpec is one result of TranslateFileSpec.
type FileSpec struct {
	// Spec is the fully translated specification. A physical device name
	// keeps its leading underscore ("_TTA0:"), as in a VMS resultant
	// string; the caller strips it when it looks the device up.
	Spec string

	// Concealed is the outermost logical name in the translation whose
	// equivalence string has AttrConcealed, or "" if there is none.
	Concealed string

	// Display is the specification as it should be shown to the user:
	// the text at the point Concealed was about to be translated, so it
	// names the concealed logical rather than the physical device. It is
	// the same as Spec when nothing was concealed.
	Display string
}

// TranslateFileSpec performs the logical-name translation RMS and DCL
// apply to a file specification or device name (CLRM §2.2.3, User's
// Manual §11.5), and returns the resulting specifications in search
// order:
//
//   - A spec starting with "_" is a physical name and isn't translated.
//   - Otherwise only the leftmost component is a candidate: the text
//     before the first ":" (unless it is "::", a node name), or, when the
//     spec has no ":" at all, the whole spec. The candidate must consist
//     of letters, digits, "$", "_" and "-", so a spec such as
//     "[DRYSDALE]PUP" or "PUP.TXT" is left alone.
//   - The candidate is upper-cased and looked up through LNM$FILE_DEV at
//     mode (see Translate). Its equivalence string replaces "name:" (or
//     the whole spec), and the result is examined again from the top of
//     the search order, until nothing translates or the equivalence used
//     has AttrTerminal.
//   - A search list fans out: each equivalence string is translated in
//     turn, depth-first, so a nested list's elements appear in place of
//     the name that led to it (the User's Manual's NESTED example).
//
// The equivalence string and the rest of the spec are joined as text;
// applying defaults and merging fields is the caller's job.
//
// Errors: SS$_TOOMANYLNAM when one chain needs more than MaxDepth
// translations or reaches a name it has already translated (a circular
// definition), and those of ResolveTables when LNM$FILE_DEV no longer
// designates any table. A name that isn't defined is not an error.
func (db *Database) TranslateFileSpec(spec string, mode Mode) ([]FileSpec, error) {
	var out []FileSpec

	err := db.translateFileSpec(spec, mode, 0, nil, "", "", &out)
	if err != nil {
		return nil, err
	}

	return out, nil
}

// translateFileSpec translates spec, depth translations into a chain
// whose names so far are in chain, appending its results to out.
// concealed and display carry the chain's concealed name, if any.
func (db *Database) translateFileSpec(spec string, mode Mode, depth int, chain []string, concealed, display string, out *[]FileSpec) error {
	name, rest, ok := leftmostComponent(spec)
	if ok {
		e, err := db.Translate(FileDevName, name, mode, 0)
		if err == nil {
			return db.substitute(e, rest, spec, mode, depth, chain, concealed, display, out)
		}

		var ve vmserrors.VMSError
		if !errors.As(err, &ve) || ve.Status != vmserrors.SS_NOLOGNAM {
			return err
		}
	}

	if concealed == "" {
		display = spec
	}

	*out = append(*out, FileSpec{Spec: spec, Concealed: concealed, Display: display})

	return nil
}

// substitute replaces the leftmost component of spec, whose translation
// is e and whose remaining text is rest, by each of e's equivalence
// strings in turn, and translates each result further.
func (db *Database) substitute(e *Entry, rest, spec string, mode Mode, depth int, chain []string, concealed, display string, out *[]FileSpec) error {
	if depth >= MaxDepth {
		return status(vmserrors.SS_TOOMANYLNAM)
	}

	for _, n := range chain {
		if n == e.Name {
			db.tracef("translate file spec: %s is circular", e.Name)

			return status(vmserrors.SS_TOOMANYLNAM)
		}
	}

	chain = append(chain[:len(chain):len(chain)], e.Name)

	for _, eqv := range e.Equivalences {
		next := eqv.Value + rest

		c, d := concealed, display
		if c == "" && eqv.Attrs&AttrConcealed != 0 {
			c, d = e.Name, spec
		}

		if eqv.Attrs&AttrTerminal != 0 {
			if c == "" {
				d = next
			}

			*out = append(*out, FileSpec{Spec: next, Concealed: c, Display: d})

			continue
		}

		if err := db.translateFileSpec(next, mode, depth+1, chain, c, d, out); err != nil {
			return err
		}
	}

	return nil
}

// leftmostComponent returns the logical-name candidate in spec, upper-
// cased, and the text following it (after its ":", or "" when the
// candidate is the whole spec). ok is false when spec has no candidate.
func leftmostComponent(spec string) (name, rest string, ok bool) {
	if spec == "" || spec[0] == '_' {
		return "", "", false
	}

	end := len(spec)

	for i := 0; i < len(spec); i++ {
		if spec[i] == ':' {
			if i+1 < len(spec) && spec[i+1] == ':' {
				return "", "", false
			}

			end, rest = i, spec[i+1:]

			break
		}
	}

	name = spec[:end]
	if name == "" || len(name) > MaxNameLength {
		return "", "", false
	}

	b := []byte(name)
	for i, c := range b {
		if !isNameChar(c) {
			return "", "", false
		}

		b[i] = upper(c)
	}

	return string(b), rest, true
}

// isNameChar reports whether c may appear in a logical name used in a
// file specification (User's Manual §11.3.3).
func isNameChar(c byte) bool {
	return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' ||
		c == '$' || c == '_' || c == '-'
}
