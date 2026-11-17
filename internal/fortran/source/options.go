package source

import (
	"regexp"
	"strings"
)

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
	"F90", "NOF90",
	"G_FLOATING", "NOG_FLOATING",
	"I4", "NOI4",
	"RECURSIVE", "NORECURSIVE",
}

// shortestForm is the fewest letters a qualifier added after VAX FORTRAN's may be
// cut to: /F90 came after /F77 (docs/planning/PHASE-99 - Fortran 90.md), and
// /F and /NOF still mean /F77 and /NOF77, as they always have.
var shortestForm = map[string]int{"F90": len("F9"), "NOF90": len("NOF9")}

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

		if strings.HasPrefix(candidate, name) && len(name) >= shortestForm[candidate] {
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
	case r.inUnit && r.nested:
		r.followNested(s)

	case isEnd(s):
		r.form = r.command
		r.inUnit = false

	case r.inUnit:
		r.nested, r.depth = opensProcedures(s), 1

	case isBlank(s.Text):

	default:
		qualifiers, isOptions := optionsQualifiers(s.Text)
		if !isOptions {
			r.inUnit = true
			r.nested, r.depth = opensProcedures(s), 1

			return
		}

		for _, q := range qualifiers {
			switch OptionsQualifier(q) {
			case "EXTEND_SOURCE":
				r.form.ExtendSource = true
			case "NOEXTEND_SOURCE":
				r.form.ExtendSource = false
			case "F90":
				r.form.F90 = true
			case "NOF90":
				r.form.F90 = false
			}
		}
	}
}

// followNested follows a unit with procedures inside it (Fortran 90's CONTAINS
// and interface blocks; docs/planning/PHASE-99 - Fortran 90.md, subtask 5),
// where an END may be a procedure's: each procedure's header opens one more
// level, each END closes one, and the unit ends with the END that closes its
// own.
func (r *reader) followNested(s *Statement) {
	switch {
	case isEnd(s):
		if r.depth--; r.depth == 0 {
			r.form = r.command
			r.inUnit, r.nested = false, false
		}

	case isProcedureHeader(s):
		r.depth++
	}
}

// isEnd reports whether s is an END statement: only END on its initial line,
// whatever continuation lines follow it (section 5.5), which covers a statement
// that's only END. "END = 1" and "ENDIF" aren't. Fortran 90's END PROGRAM,
// END SUBROUTINE, END FUNCTION, END MODULE, and END BLOCK DATA, each with or
// without a name, are too (the lexer's endStatement).
func isEnd(s *Statement) bool {
	initial := s.Initial()

	start, rest := squeezeKeyword(initial, "END")
	if start < 0 {
		return false
	}

	if isBlank(initial[rest:]) {
		return true
	}

	return namedEnd.MatchString(squeezedUpper(s.Text))
}

// namedEnd is Fortran 90's END statements that end a unit, squeezed:
// ENDPROGRAMP, ENDSUBROUTINE, ENDBLOCKDATA.
var namedEnd = regexp.MustCompile(`^END(PROGRAM|SUBROUTINE|FUNCTION|MODULE|BLOCKDATA)([A-Z][A-Z0-9_$]*)?$`)

// procedureHeader is a FUNCTION or SUBROUTINE statement, squeezed: RECURSIVE
// before or after the type, the type with a length or a kind selector, or
// TYPE (name).
var procedureHeader = regexp.MustCompile(`^(RECURSIVE)?((INTEGER|REAL|LOGICAL|COMPLEX|DOUBLEPRECISION|DOUBLECOMPLEX|CHARACTER|BYTE)` +
	`(\*[0-9]+|\*\([^)]*\)|\([^)]*\))?|TYPE\([A-Z0-9_$]+\))?(RECURSIVE)?(FUNCTION|SUBROUTINE)[A-Z]`)

// isProcedureHeader reports whether s is a FUNCTION or SUBROUTINE statement,
// in a unit that has procedures inside it. An assignment has an = outside
// parentheses (FUNCTIONX = 1); a header has none.
func isProcedureHeader(s *Statement) bool {
	text := squeezedUpper(s.Text)

	return procedureHeader.MatchString(text) && !equalsAtLevel0(text)
}

// opensProcedures reports whether s is a statement after which a unit's
// procedures have ENDs of their own: Fortran 90's CONTAINS, or INTERFACE (an
// interface block, holding interface bodies). INTERFACE = 1 is an assignment.
// The first statement of a unit counts too: a module or main program may hold
// procedures from there on.
func opensProcedures(s *Statement) bool {
	text := squeezedUpper(s.Text)

	switch {
	case text == "CONTAINS":
		return true
	case strings.HasPrefix(text, "INTERFACE"):
		return !equalsAtLevel0(text)
	}

	return false
}

// squeezedUpper is text without blanks and tabs, in capitals: enough to
// recognize the few statements unit following needs, none of which has a
// character constant.
func squeezedUpper(text string) string {
	return strings.Map(func(c rune) rune {
		if isBlankRune(c) {
			return -1
		}

		return c
	}, strings.ToUpper(text))
}

// equalsAtLevel0 reports whether text has an = outside parentheses.
func equalsAtLevel0(text string) bool {
	depth := 0

	for _, c := range text {
		switch c {
		case '(':
			depth++
		case ')':
			depth--
		case '=':
			if depth == 0 {
				return true
			}
		}
	}

	return false
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
