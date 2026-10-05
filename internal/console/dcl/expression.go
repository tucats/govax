package dcl

import (
	"strings"
)

// elementFunc reads one element of a list value (see readList): the
// element's text and what follows it. paren is set inside a qualifier's
// parenthesised list, where ')' also ends an element.
type elementFunc func(s string, paren bool) (token, rest string, err error)

// elementReader returns how a list of typ's values reads each element: an
// $expression element with readExpression, anything else as a plain DCL
// token.
func elementReader(typ ValueType) elementFunc {
	if typ == TypeExpression {
		return func(s string, _ bool) (string, string, error) {
			return readExpression(s, 0)
		}
	}

	return readListElement
}

// readTypedValue reads one parameter or qualifier value of type typ: an
// $expression with readExpression, anything else as readValueToken reads
// it. A nonzero sep (the parameter's /separator=) also ends the value.
func readTypedValue(s string, typ ValueType, sep byte) (token, rest string, err error) {
	if typ == TypeExpression {
		return readExpression(s, sep)
	}

	token, rest, err = readValueToken(s)
	if err != nil || sep == 0 {
		return token, rest, err
	}

	// A bare token runs to its first separator ("PC=200" is "PC" and
	// "=200"); a quoted one has already ended at its closing quote.
	if s = strings.TrimLeft(s, " \t"); !strings.HasPrefix(s, `"`) {
		if i := strings.IndexByte(token, sep); i >= 0 {
			return token[:i], s[i:], nil
		}
	}

	return token, rest, nil
}

// skipSeparator skips one sep character, and the blanks before it, from
// the front of s; s is returned as it is when sep is zero or doesn't come
// next.
func skipSeparator(s string, sep byte) string {
	if sep == 0 {
		return s
	}

	if t := strings.TrimLeft(s, " \t"); t != "" && t[0] == sep {
		return t[1:]
	}

	return s
}

// isAssignment reports whether s starts with a name followed by "=" (with
// or without blanks before the "="), the form an entry's /assignment=
// looks for.
func isAssignment(s string) bool {
	name, rest := readBareToken(s)
	if name == "" || strings.HasPrefix(name, `"`) {
		return false
	}

	return strings.HasPrefix(strings.TrimLeft(rest, " \t"), "=")
}

// expressionOperators are the console expression evaluator's binary
// operators (expr.go): an operator joins whatever is on either side of
// it, blanks included.
const expressionOperators = "+-*/=<>"

// readExpression reads one console expression from s (an $expression
// value) and returns its text, trailing blanks removed, and what follows.
// The expression's own syntax is the console evaluator's; this only finds
// where it ends:
//
//   - Inside parentheses or double quotes, everything belongs to it.
//   - A blank ends it unless an operator joins the two sides ("X + 4",
//     "X+ 4", "X +4"), or the expression so far ends in an operator.
//   - A '/' is division, except after a blank and before a letter, where
//     it starts a qualifier ("EXAMINE 100 /BYTE"): "X/Y" and "X / Y" are
//     divisions, "X /Y" is X and the qualifier /Y.
//   - A ',' outside parentheses ends it (a list's next element), as does
//     a ')' with no '(' to match (the end of a qualifier's list), and the
//     parameter's separator, sep, when it's nonzero.
//
// A quoted string keeps its quotes, so the evaluator sees it as a string
// literal. One with no closing quote runs to the end of s, as the
// evaluator's own string literal does.
func readExpression(s string, sep byte) (token, rest string, err error) {
	s = strings.TrimLeft(s, " \t")

	depth := 0
	needOperand := true // at the start, or after an operator or '('
	i := 0

scan:
	for i < len(s) {
		ch := s[i]

		switch {
		case ch == '"':
			end := strings.IndexByte(s[i+1:], '"')
			if end < 0 {
				i = len(s)

				break scan
			}

			i += end + 2
			needOperand = false

		case ch == '(':
			depth++
			needOperand = true
			i++

		case ch == ')':
			if depth == 0 {
				break scan
			}

			depth--
			needOperand = false
			i++

		case depth > 0:
			i++

		case ch == ',' || (sep != 0 && ch == sep):
			break scan

		case ch == ' ' || ch == '\t':
			j := i
			for j < len(s) && (s[j] == ' ' || s[j] == '\t') {
				j++
			}

			if j == len(s) || !joinsAcrossBlank(s[j:], needOperand, sep) {
				break scan
			}

			i = j

		case strings.IndexByte(expressionOperators, ch) >= 0:
			needOperand = true
			i++

		default:
			needOperand = false
			i++
		}
	}

	return strings.TrimRight(s[:i], " \t"), s[i:], nil
}

// joinsAcrossBlank reports whether the expression continues past a run of
// blanks, next being what follows them: when it's waiting for an operand,
// or next is an operator — but not a '/' starting a qualifier, or the
// separator.
func joinsAcrossBlank(next string, needOperand bool, sep byte) bool {
	ch := next[0]

	switch {
	case sep != 0 && ch == sep:
		return false

	case ch == ',':
		return false

	case ch == '/':
		return len(next) < 2 || !isLetter(next[1])

	case needOperand:
		return true

	default:
		return strings.IndexByte(expressionOperators, ch) >= 0
	}
}

func isLetter(ch byte) bool {
	return (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') || ch == '$' || ch == '_'
}
