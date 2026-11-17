package lex

import "github.com/tucats/govax/internal/fortran/source"

// squeezed is a statement's text with the blanks and tabs taken out of it
// everywhere but inside constants, and letters outside constants made
// capitals: the first of the lexer's three steps (see doc.go).
//
// For each character kept it remembers where the character was in the
// statement's text (at), so that a token can say where it came from, and
// whether it is part of a constant (lit): a character or Hollerith
// constant's text, its apostrophes, and a Hollerith constant's H. The
// later steps skip lit characters when they look for the = or ( that
// decide a statement's kind, since '=' inside a constant is just text.
//
// gap says a blank (or tab) was taken out just before the character. In
// fixed form that means nothing; in free form blanks are significant
// (docs/planning/PHASE-99 - Fortran 90.md, Decision 3), and the tokenizer
// reads them from gap: one inside a token is an error, and one is needed
// between two words.
type squeezed struct {
	text []byte
	at   []int
	lit  []bool
	gap  []bool
}

// squeezeProblem is something wrong found while squeezing: an unclosed
// character constant or a Hollerith constant cut short by the statement's
// end.
type squeezeProblem struct {
	ident  string // NOQUOTE or HOLLEND
	offset int    // where in the statement's text the constant starts
	args   []any
}

// maxHollerith caps a Hollerith count, so that a long run of digits can't
// overflow: no statement is longer.
const maxHollerith = 1 << 20

// squeeze removes text's blanks outside constants. It follows the same
// rules as the source pass's search for a ! comment (source/comment.go),
// since both must agree where constants are:
//
//   - an apostrophe starts a character constant, and the next one ends it
//     (a doubled apostrophe inside one ends it and starts another at
//     once, which is as good: both are lit); under /F90 (f90) a quotation
//     mark does the same, and a constant ends only at the mark it started
//     with;
//   - but an apostrophe after a name or number inside parentheses is the
//     record separator of READ (1'10), not a constant, unless
//     source.ApostropheStartsConstant finds Fortran 90's K_'text' or Z'1F';
//   - digits where a value can go (source.CountMayFollow) followed by an H
//     are a Hollerith count, and that many characters after the H, blanks
//     and all, are the constant; followed by an R, a RADIX-50 constant's.
func squeeze(text string, f90 bool) (*squeezed, []squeezeProblem) {
	var (
		s        = &squeezed{}
		problems []squeezeProblem

		quoted    bool // inside a character constant
		delimiter byte // the ' or " that started it
		quoteAt   int  // where it started
		hollerith int  // a Hollerith constant's characters still to come
		hollAt    int  // where its count started
		hollCount int  // its count
		radix     bool // it is a RADIX-50 constant, not a Hollerith one

		depth int // parentheses open

		last     byte // the last character kept outside constants
		counting bool // the digits ending at last may be a Hollerith count
		countAt  int  // where they started
		count    int

		blank bool // a blank was taken out since the last character kept
	)

	// keep adds one character to the squeezed text, with where it came
	// from, whether it's inside a constant, and whether a blank came out
	// before it. The four slices grow together, so index n of each is about
	// the same character.
	keep := func(c byte, at int, lit bool) {
		s.text = append(s.text, c)
		s.at = append(s.at, at)
		s.lit = append(s.lit, lit)
		s.gap = append(s.gap, blank)
		blank = false
	}

	// note records the last character kept outside a constant, which the
	// rules for apostrophes and Hollerith counts look back at.
	note := func(c byte) {
		last = c
		counting = false
	}

	// One character per turn. The loop is a small state machine: the
	// variables above say whether we're inside a quoted constant, inside a
	// Hollerith constant's text, or reading digits that may be a count, and
	// each case either handles the character and continues the loop, or
	// falls out of the switch to the common code at the bottom, which keeps
	// it as an ordinary character in capitals.
	for i := 0; i < len(text); i++ {
		c := text[i]

		switch {
		case hollerith > 0:
			// Inside 5HHELLO's HELLO: kept as is, blanks and case and all.
			keep(c, i, true)
			hollerith--

			continue

		case quoted:
			// Inside 'text': kept as is, until the closing apostrophe.
			keep(c, i, true)

			if c == delimiter {
				quoted = false
				note(c)
			}

			continue

		case c == ' ' || c == '\t':
			// Outside constants, blanks are dropped: the point of squeezing.
			blank = true

			continue

		case c == '\'':
			if source.ApostropheStartsConstant(last, depth, text[i+1:]) {
				keep(c, i, true)
				quoted, delimiter, quoteAt = true, c, i
			} else {
				keep(c, i, false) // READ (1'10): a record number follows
				note(c)
			}

			continue

		case c == '"' && f90:
			// Under /F90 "text" is a character constant (Decision 2);
			// otherwise "17 is appendix A.5's octal constant, which the
			// tokenizer reads.
			keep(c, i, true)
			quoted, delimiter, quoteAt = true, c, i

			continue

		case c >= '0' && c <= '9':
			// Digits may be an ordinary number, or a Hollerith count if an
			// H follows. Whether a count can stand here depends on what came
			// before (source.CountMayFollow): in X = 5HABCDE it can, but in
			// DO 5 H = 1, 2 the 5 is a label and H a variable.
			switch {
			case counting:
				count = min(count*10+int(c-'0'), maxHollerith)
				keep(c, i, false)
				last = c

				continue

			case source.CountMayFollow(last, startOf(s.text)):
				keep(c, i, false)
				note(c)
				counting, countAt, count = true, i, int(c-'0')

				continue
			}

		case (c == 'H' || c == 'h' || c == 'R' || c == 'r') && counting && count > 0:
			// The H (or R) after a count: the next count characters are the
			// constant's text.
			c = upper(c)
			keep(c, i, true)
			hollerith, hollAt, hollCount, radix = count, countAt, count, c == 'R'
			note(c)

			continue

		case c == '(':
			depth++

		case c == ')':
			depth = max(depth-1, 0)
		}

		c = upper(c)
		keep(c, i, false)
		note(c)
	}

	switch {
	case quoted:
		problems = append(problems, squeezeProblem{ident: "NOQUOTE", offset: quoteAt, args: []any{quoteName(delimiter)}})
	case hollerith > 0:
		ident := "HOLLEND"
		if radix {
			ident = "RAD50END"
		}

		problems = append(problems, squeezeProblem{ident: ident, offset: hollAt,
			args: []any{hollCount, hollCount - hollerith}})
	}

	return s, problems
}

// quoteName names a character constant's delimiter, for NOQUOTE.
func quoteName(delimiter byte) string {
	if delimiter == '"' {
		return "quotation mark"
	}

	return "apostrophe"
}

// startOf is what source.CountMayFollow needs of the text kept so far.
func startOf(text []byte) string {
	return string(text[:min(len(text), source.TypeLengthStart)])
}

// isNameChar reports whether c can be part of a FORTRAN name or number:
// a letter, a digit, $, or _.
func isNameChar(c byte) bool {
	return isLetter(c) || isDigit(c) || c == '$' || c == '_'
}

func isLetter(c byte) bool {
	return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z'
}

func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}

// upper is c in capitals, if it is a lowercase letter.
func upper(c byte) byte {
	if c >= 'a' && c <= 'z' {
		return c - 'a' + 'A'
	}

	return c
}
