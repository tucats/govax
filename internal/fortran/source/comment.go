package source

// commentScanner finds where an end-of-line comment starts in a
// statement's text. A ! starts one, except inside a character constant
// ('WHAT!') or a Hollerith constant (5HWHAT!), so finding it means
// following those constants through the text, and through continuation
// lines, since a constant may go on from one line to the next. A scanner
// lives for one statement and is fed its lines' statement fields in turn.
//
// FORTRAN has two kinds of constant to follow:
//
//   - A character constant is in apostrophes; an apostrophe inside it is
//     written twice ('IT''S'). Two constants written side by side would
//     read the same, so the scanner needn't tell them apart: each
//     apostrophe simply switches between inside and outside.
//   - A Hollerith constant (from FORTRAN's early days, still in VAX
//     FORTRAN's FORMAT and DATA statements) is a count, an H, and that
//     many characters: 5HHELLO. Anything may follow the H, blanks,
//     apostrophes, and ! included. Since blanks don't matter outside
//     constants, "5 HHELLO" is the same constant.
//
// Telling a Hollerith count from other digits followed by an H needs a
// little context: in REAL*8 HVAL, the 8 is a length, not a count, and in
// DO 10 H = 1, 2 the 10 is a label. A Hollerith constant can only be
// where a value can go, so the scanner counts digits as a Hollerith count
// only after a character a value can follow: ( , / = or . (as in
// .EQ.5HHELLO), or the * of a repeat count (DATA X/2*4HABCD/). Digits that
// follow a letter are part of a name, and the other places digits go
// (labels, lengths) are after a keyword.
//
// Phase 82 widened that: a value may also follow + and - (ICOUNT + 2HXY), and
// any * but the one in a type statement's length, which is why a * looks back
// at the statement so far (CountMayFollow). And a RADIX-50 constant, nRabc
// (appendix B.3), is followed the same way, as a count, an R, and that many
// characters.
//
// VAX FORTRAN uses the apostrophe once more: READ (1'10) X reads record
// 10 of unit 1. There the apostrophe follows the unit, a name or number,
// inside parentheses, where a character constant never follows a name, so
// the scanner treats an apostrophe after a letter or digit inside
// parentheses as that separator, not as the start of a constant.
type commentScanner struct {
	quoted    bool // inside a character constant
	hollerith int  // a Hollerith constant's characters still to come

	depth int // how many parentheses are open

	// The last character that wasn't a blank, outside constants, and
	// whether the digits ending at it could be a Hollerith count (count is
	// their value).
	last     byte
	counting bool
	count    int

	// start is the statement's first characters outside constants, in
	// capitals: enough to know a type statement's length (CountMayFollow).
	start []byte

	// lower is where the last cut found its field's first small letter outside
	// constants, or -1 if it found none, for /STANDARD=SOURCE_FORM.
	lower int
}

// maxHollerith caps a Hollerith count, so a long run of digits can't
// overflow: no statement is longer.
const maxHollerith = 1 << 20

// cut returns the length of field before its end-of-line comment: the
// index of the ! that starts one, or len(field) if there is none. The
// scanner remembers where it is in any constant for the next line.
func (s *commentScanner) cut(field string) int {
	s.lower = -1

	for i := 0; i < len(field); i++ {
		c := field[i]

		if c >= 'a' && c <= 'z' && s.lower < 0 && s.hollerith == 0 && !s.quoted {
			s.lower = i
		}

		switch {
		case s.hollerith > 0:
			s.hollerith--

			continue

		case s.quoted:
			if c == '\'' {
				s.quoted = false
				s.note(c)
			}

			continue

		case c == ' ' || c == '\t':
			continue

		case c == '!':
			return i

		case c == '\'':
			if s.depth > 0 && isNameChar(s.last) {
				s.note(c) // READ (1'10): a record number follows
			} else {
				s.quoted = true
			}

			continue

		case c >= '0' && c <= '9':
			switch {
			case s.counting:
				s.count = min(s.count*10+int(c-'0'), maxHollerith)
				s.last = c

				continue

			case s.countMayFollow():
				s.note(c)
				s.counting = true
				s.count = int(c - '0')

				continue
			}

		case (c == 'H' || c == 'h' || c == 'R' || c == 'r') && s.counting && s.count > 0:
			s.hollerith = s.count
			s.note(c)

			continue

		case c == '(':
			s.depth++

		case c == ')':
			s.depth = max(s.depth-1, 0)
		}

		s.note(c)
	}

	return len(field)
}

// note records c as the last character seen outside a constant, which
// ends any digits being counted.
func (s *commentScanner) note(c byte) {
	s.last = c
	s.counting = false

	if len(s.start) < TypeLengthStart {
		if c >= 'a' && c <= 'z' {
			c -= 'a' - 'A'
		}

		s.start = append(s.start, c)
	}
}

// countMayFollow reports whether a Hollerith count may start here.
func (s *commentScanner) countMayFollow() bool {
	return CountMayFollow(s.last, string(s.start))
}

// TypeLengthStart is how much of a statement CountMayFollow needs to see:
// "CHARACTER*", the longest of its words, and one more, so that a longer
// statement that starts the same way isn't taken for it.
const TypeLengthStart = 11

// CountMayFollow reports whether digits after last may be a Hollerith or
// RADIX-50 count: whether a value can go there. start is the statement so far,
// last included, in capitals, of which only the first TypeLengthStart characters
// matter.
//
// After a * they may, but for a type statement's length: in REAL*8 H the 8 is
// a length and H a name, where in X = N*2HAB the 2 is a count. The lexer uses
// this too, since the two must agree where constants are.
func CountMayFollow(last byte, start string) bool {
	switch last {
	case '(', ',', '/', '=', '.', '+', '-':
		return true
	case '*':
		switch start {
		case "REAL*", "INTEGER*", "LOGICAL*", "COMPLEX*", "CHARACTER*":
			return false
		}

		return true
	}

	return false
}

// isNameChar reports whether c can be part of a FORTRAN name or number:
// a letter, a digit, $, or _.
func isNameChar(c byte) bool {
	return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '$' || c == '_'
}
