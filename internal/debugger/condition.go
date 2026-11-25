package debugger

import (
	"strings"
)

// A breakpoint's WHEN clause holds a condition such as
//
//	WHEN (.COUNT EQL 5 AND R1 GTR 2)
//
// This file evaluates the conditions the debugger's commands accept so
// far: comparisons (EQL, NEQ, LSS, LEQ, GTR, GEQ) between two operands,
// joined with AND, OR, and NOT, and grouped with parentheses. An operand
// is an expression's value, as EVALUATE's: a data label is its contents,
// so ".COUNT" is the contents of the address COUNT holds, as in VMS 7.1's
// (testdata/dbgcmd's WHEN (.WATCHL EQL 2) read address 2, and
// testdata/dbg113's DBG113T addresses 0, 1, and 2 as COUNT went from 0 to
// 2). A condition that can't be evaluated says why, and the eventpoint is
// taken (eventpoint.go's conditionHolds), as VMS's was.

// comparisons are the relational operators, as the debugger spells them.
var comparisons = []string{"EQL", "NEQ", "LSS", "LEQ", "GTR", "GEQ"}

// evalCondition evaluates a WHEN condition, parentheses included, and
// reports whether it is true.
func (d *Debugger) evalCondition(text string) (bool, error) {
	text = unparenthesize(text)

	// OR binds loosest, then AND, then NOT, then a comparison; so each is
	// split off in that order, at its first occurrence outside any
	// parentheses.
	if left, right, ok := splitWord(text, "OR"); ok {
		l, err := d.evalCondition(left)
		if err != nil {
			return false, err
		}

		r, err := d.evalCondition(right)

		return l || r, err
	}

	if left, right, ok := splitWord(text, "AND"); ok {
		l, err := d.evalCondition(left)
		if err != nil {
			return false, err
		}

		r, err := d.evalCondition(right)

		return l && r, err
	}

	if rest, ok := strings.CutPrefix(strings.ToUpper(text), "NOT "); ok {
		v, err := d.evalCondition(text[len(text)-len(rest):])

		return !v, err
	}

	for _, op := range comparisons {
		left, right, ok := splitWord(text, op)
		if !ok {
			continue
		}

		// %EXC_NAME EQL "ACCVIO" compares strings.
		if result, isString, err := d.stringComparison(op, left, right); isString || err != nil {
			return result, err
		}

		l, err := d.operand(left)
		if err != nil {
			return false, err
		}

		r, err := d.operand(right)
		if err != nil {
			return false, err
		}

		return compare(op, int32(l), int32(r)), nil
	}

	// A lone operand is true when it isn't zero.
	v, err := d.operand(text)

	return v != 0, err
}

// compare applies a relational operator to two signed longwords.
func compare(op string, l, r int32) bool {
	switch op {
	case "EQL":
		return l == r
	case "NEQ":
		return l != r
	case "LSS":
		return l < r
	case "LEQ":
		return l <= r
	case "GTR":
		return l > r
	}

	return l >= r // GEQ
}

// operand evaluates one side of a comparison, an expression's value. The
// evaluator takes registers (R1, SP), and ".expression" for the longword
// at an address, which is %DEBUG-E-NOACCESSR when the program can't read
// it. A FORTRAN variable is its place, as before.
func (d *Debugger) operand(text string) (uint32, error) {
	text = unparenthesize(text)

	if _, ok, _ := d.fortranPlaces(text); ok {
		return d.evalWhole(text)
	}

	return d.Machine.EvalWholeMode(d.inputRadix, text, true)
}

// splitWord splits text around the first occurrence of word (compared
// without regard to case) that stands alone, as a whole word outside any
// parentheses or quotes. ok is false when there is none.
func splitWord(text, word string) (left, right string, ok bool) {
	upper := strings.ToUpper(text)

	if i := findWord(upper, word, 0); i >= 0 {
		return text[:i], text[i+len(word):], true
	}

	return "", "", false
}

// findWord returns the index of the first occurrence of word in text at or
// after from that is a whole word (not part of a longer name) and is
// outside parentheses and quotes, or -1.
func findWord(text, word string, from int) int {
	depth := 0

	var quote byte

	for i := 0; i < len(text); i++ {
		ch := text[i]

		switch {
		case quote != 0:
			if ch == quote {
				quote = 0
			}

			continue

		case ch == '"' || ch == '\'':
			quote = ch

			continue

		case ch == '(':
			depth++

			continue

		case ch == ')':
			depth--

			continue
		}

		if depth != 0 || i < from || !strings.HasPrefix(text[i:], word) {
			continue
		}

		before := i == 0 || !isWordByte(text[i-1])
		after := i+len(word) == len(text) || !isWordByte(text[i+len(word)])

		if before && after {
			return i
		}
	}

	return -1
}
