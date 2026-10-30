package lex

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
type squeezed struct {
	text []byte
	at   []int
	lit  []bool
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
//     once, which is as good: both are lit);
//   - but an apostrophe after a name or number inside parentheses is the
//     record separator of READ (1'10), not a constant;
//   - digits after one of ( , / = . (or after * with a digit before it, a
//     DATA repeat count) followed by an H are a Hollerith count, and that
//     many characters after the H, blanks and all, are the constant.
func squeeze(text string) (*squeezed, []squeezeProblem) {
	var (
		s        = &squeezed{}
		problems []squeezeProblem

		quoted    bool // inside a character constant
		quoteAt   int  // where it started
		hollerith int  // a Hollerith constant's characters still to come
		hollAt    int  // where its count started
		hollCount int  // its count

		depth int // parentheses open

		last, prev byte // the last two characters kept outside constants
		counting   bool // the digits ending at last may be a Hollerith count
		countAt    int  // where they started
		count      int
	)

	keep := func(c byte, at int, lit bool) {
		s.text = append(s.text, c)
		s.at = append(s.at, at)
		s.lit = append(s.lit, lit)
	}

	note := func(c byte) {
		prev, last = last, c
		counting = false
	}

	for i := 0; i < len(text); i++ {
		c := text[i]

		switch {
		case hollerith > 0:
			keep(c, i, true)
			hollerith--

			continue

		case quoted:
			keep(c, i, true)

			if c == '\'' {
				quoted = false
				note(c)
			}

			continue

		case c == ' ' || c == '\t':
			continue

		case c == '\'':
			if depth > 0 && isNameChar(last) {
				keep(c, i, false) // READ (1'10): a record number follows
				note(c)
			} else {
				keep(c, i, true)
				quoted, quoteAt = true, i
			}

			continue

		case c >= '0' && c <= '9':
			switch {
			case counting:
				count = min(count*10+int(c-'0'), maxHollerith)
				keep(c, i, false)
				last = c

				continue

			case countMayFollow(last, prev):
				keep(c, i, false)
				note(c)
				counting, countAt, count = true, i, int(c-'0')

				continue
			}

		case (c == 'H' || c == 'h') && counting && count > 0:
			keep('H', i, true)
			hollerith, hollAt, hollCount = count, countAt, count
			note('H')

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
		problems = append(problems, squeezeProblem{ident: "NOQUOTE", offset: quoteAt})
	case hollerith > 0:
		problems = append(problems, squeezeProblem{ident: "HOLLEND", offset: hollAt,
			args: []any{hollCount, hollCount - hollerith}})
	}

	return s, problems
}

// countMayFollow reports whether a Hollerith count may start after last
// (and prev before it): where a value can go.
func countMayFollow(last, prev byte) bool {
	switch last {
	case '(', ',', '/', '=', '.':
		return true
	case '*':
		return isDigit(prev)
	}

	return false
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
