package source

import "strings"

// An OPTIONS statement's /EXTEND_SOURCE says where the statement field ends on
// the lines of the unit after it, until that unit's END (section 1.4.3):
//
//	      OPTIONS /EXTEND_SOURCE
//	      PROGRAM P              <- lines read to column 132
//	      END
//	      SUBROUTINE S           <- lines read as the command said
//	      END
//
// Every other OPTIONS qualifier is read by semantic analysis, the fourth pass, but
// this one can't wait: where a line's statement ends has to be known to make the
// statements the later passes read. So this pass recognizes just enough of the
// language to follow it: an OPTIONS statement, and where a unit starts and ends.
// It agrees with the parser on those, parse.Parse's doc comment having the rules. The one
// difference is a unit a header interrupts, with no END: that's the parser's NOEND
// error, the compilation fails anyway, and here the form lasts to the next END.
//
// The OPTIONS statement's own lines, and any before it, are read under the form
// already in effect, since it can't be known until they've been read (ours; the
// manuals don't say).

// optionsNames are the OPTIONS statement's qualifiers (Appendix D's summary), each
// with its negative where it has one.
var optionsNames = []string{
	"CHECK", "NOCHECK",
	"EXTEND_SOURCE", "NOEXTEND_SOURCE",
	"F77", "NOF77",
	"G_FLOATING", "NOG_FLOATING",
	"I4", "NOI4",
	"RECURSIVE", "NORECURSIVE",
}

// OptionsQualifier is the whole name of an OPTIONS qualifier written name, which
// may be cut short so long as it stands for only one of them, as on the command line
// (section 1.4.3 says the two take the same abbreviations). A name that is none of
// them, or could be several, comes back as it is.
func OptionsQualifier(name string) string {
	whole, found := name, 0

	for _, candidate := range optionsNames {
		if candidate == name {
			return name
		}

		if strings.HasPrefix(candidate, name) {
			whole = candidate
			found++
		}
	}

	if found == 1 {
		return whole
	}

	return name
}

// followUnit keeps the reader's form up to date, given the next statement. A
// statement between units that holds nothing (just a label) or is an OPTIONS statement opens
// no unit; any other does. END ends the unit, and with it what OPTIONS said.
func (r *reader) followUnit(s *Statement) {
	switch {
	case isEnd(s):
		r.form = r.command
		r.inUnit = false

	case r.inUnit:

	case isBlank(s.Text):

	default:
		qualifiers, isOptions := optionsQualifiers(s.Text)
		if !isOptions {
			r.inUnit = true

			return
		}

		for _, q := range qualifiers {
			switch OptionsQualifier(q) {
			case "EXTEND_SOURCE":
				r.form.ExtendSource = true
			case "NOEXTEND_SOURCE":
				r.form.ExtendSource = false
			}
		}
	}
}

// isEnd reports whether s is an END statement: only END on its initial line,
// whatever continuation lines follow it (section 5.5), which covers a statement
// that's only END. "END = 1" and "ENDIF" aren't.
func isEnd(s *Statement) bool {
	initial := s.Initial()

	start, rest := squeezeKeyword(initial, "END")

	return start >= 0 && isBlank(initial[rest:])
}

// optionsQualifiers reports whether text is an OPTIONS statement and the names of
// its qualifiers, in capitals. An assignment to a variable named OPTIONS never has a
// slash next. A qualifier's values have no slash in them, and bad ones are the parser's
// to find.
func optionsQualifiers(text string) ([]string, bool) {
	start, rest := squeezeKeyword(text, "OPTIONS")
	if start < 0 || rest >= len(text) || text[rest] != '/' {
		return nil, false
	}

	squeezed := strings.Map(func(c rune) rune {
		if isBlankRune(c) {
			return -1
		}

		return c
	}, strings.ToUpper(text[rest+1:]))

	var names []string

	for _, part := range strings.Split(squeezed, "/") {
		name, _, _ := strings.Cut(part, "=")
		names = append(names, name)
	}

	return names, true
}
