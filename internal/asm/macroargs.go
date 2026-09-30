package asm

import (
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/vmserrors"
)

// This file handles a macro's arguments: the formal arguments a .MACRO
// directive lists, the actual arguments a call gives, how the one is
// matched to the other, and how the actual arguments are substituted into
// the macro's lines. The rules are the MACRO manual's chapter 4.
//
// # Arguments, for a reader new to VAX MACRO
//
// Given
//
//	        .MACRO  STORE   ARG1=12, ARG2, ARG3=1000
//
// a call can give its arguments by position, by name ("keyword"
// arguments), or both, and leave any out:
//
//	        STORE   3, 2, 1             ; ARG1=3, ARG2=2, ARG3=1
//	        STORE   , 5                 ; ARG1=12 (its default), ARG2=5,
//	                                    ; ARG3=1000
//	        STORE   ARG3=7, ARG1=X      ; ARG1=X, ARG2 blank, ARG3=7
//
// Arguments are separated by commas, blanks, or tabs. An argument that
// itself holds a separator is put in angle brackets, which are removed:
// <A B C> is the one argument "A B C". A "^" followed by any character
// uses that character as the brackets instead: ^/A <B> C/ is "A <B> C".
//
// Two more forms come from chapter 4 too. "\SYMBOL" passes the symbol's
// value, as decimal digits, instead of its name (§4.6): with COUNT = 2,
// "TESTDEF \COUNT" passes "2". And a formal argument written "?NAME" is
// a created local label (§4.7): when a call leaves it blank, the
// assembler makes up a local label that no other code uses (30000$,
// 30001$, ...), so a macro can have labels of its own without clashing
// with the labels around its calls.
//
// In the body, a formal argument's name is replaced wherever it appears
// as a whole name, even in strings and comments. An apostrophe next to
// the name glues it to the text beside it and disappears: with ARG1=MOV,
// "ARG1'L" becomes "MOVL", and "X'ARG1" becomes "XMOV".

// actual is one actual argument of a macro call.
type actual struct {
	// keyword is the formal argument's name for a keyword argument
	// ("NAME=value"), and "" for a positional one.
	keyword string
	// text is the argument, delimiters removed.
	text string
}

// isArgSeparator reports whether ch separates macro arguments.
func isArgSeparator(ch byte) bool { return ch == ',' || isBlank(ch) }

// parseFormals reads a .MACRO directive's formal argument list: names
// separated by commas or blanks, each optionally "?NAME" (a created local
// label) or "NAME=default".
func parseFormals(c *cursor) ([]formal, error) {
	var formals []formal

	for {
		c.skipBlanks()

		if c.peek() == ',' {
			c.next()

			continue
		}

		if c.atEnd() {
			return formals, nil
		}

		var f formal

		if c.peek() == '?' {
			c.next()

			f.created = true
		}

		f.name = scanName(c)
		if f.name == "" {
			return nil, vmserrors.New(vmserrors.VAX_BADFORMAL, c.rest())
		}

		c.skipBlanks()

		if c.peek() == '=' {
			c.next()
			c.skipBlanks()

			def, err := scanArgument(c)
			if err != nil {
				return nil, err
			}

			f.def = def
		}

		formals = append(formals, f)
	}
}

// firstCreatedLabel is the first created local label's number, 30000$
// (the manual, §4.7). User code shouldn't use 30000$ to 65535$.
const firstCreatedLabel = 30000

// parseActuals reads a macro call's actual arguments, for macro m. Each
// comma ends an argument, so ",A,," is three arguments: a null (empty)
// one, A, and another null one. Blanks separate arguments too, but
// don't make null ones.
func (a *Assembler) parseActuals(c *cursor, m *macroDef) ([]actual, error) {
	var actuals []actual

	for {
		c.skipBlanks()

		if c.atEnd() {
			return actuals, nil
		}

		if c.peek() == ',' {
			c.next()

			actuals = append(actuals, actual{})

			continue
		}

		var act actual

		// NAME=value is a keyword argument when NAME is one of the
		// macro's formal arguments. Otherwise it's a positional
		// argument that happens to hold "=" (the manual's RESERVE
		// example passes LOCATION=12 that way).
		save := c.pos
		
		if name := scanCasedName(c); name != "" && c.peek() == '=' && c.peekAt(1) != '=' && m.formalIndex(name) >= 0 {
			c.next()

			act.keyword = strings.ToUpper(name)
		} else {
			c.pos = save
		}

		text, err := a.scanActual(c)
		if err != nil {
			return nil, err
		}

		act.text = text
		actuals = append(actuals, act)

		c.skipBlanks()

		if c.peek() == ',' {
			c.next()
		}
	}
}

// scanActual reads one actual argument's text at c: "\expression" is the
// expression's value as decimal digits (§4.6), and anything else is read
// by scanArgument. The expression has to be absolute and use only
// symbols already defined, since its value is needed now.
func (a *Assembler) scanActual(c *cursor) (string, error) {
	if c.peek() != '\\' {
		return scanArgument(c)
	}

	c.next()

	text, err := scanArgument(c)
	if err != nil {
		return "", err
	}

	v, err := a.exprNoForward(newCursor(preprocessLine(text)))
	if err != nil {
		return "", err
	}

	return strconv.FormatInt(int64(int32(v)), 10), nil
}

// scanArgument reads one argument's text at c (for a call's actual
// argument, or a formal argument's default value):
//
//   - <text>: the text inside the brackets, which may nest (<A <B> C> is
//     "A <B> C"). Brackets that don't enclose the whole argument, as in
//     <1+2>*3, are kept: that's an expression that uses them.
//   - ^Xtext X: for any character X, the text between the two X's.
//   - anything else: up to the next comma or blank not inside brackets.
func scanArgument(c *cursor) (string, error) {
	switch c.peek() {
	case '<':
		start := c.pos

		end, ok := matchBracket(c.s, c.pos)
		if !ok {
			return "", vmserrors.New(vmserrors.VAX_NOCLOSE, ">")
		}

		if end+1 >= len(c.s) || isArgSeparator(c.s[end+1]) {
			c.pos = end + 1

			return c.s[start+1 : end], nil
		}

	case '^':
		delim := c.peekAt(1)
		if delim == 0 || isBlank(delim) {
			break
		}

		text := c.s[c.pos+2:]

		end := strings.IndexByte(text, delim)
		if end < 0 {
			return "", vmserrors.New(vmserrors.VAX_NOCLOSE, string(delim))
		}

		c.skip(2 + end + 1)

		return text[:end], nil
	}

	start, depth := c.pos, 0

	for !c.atEnd() {
		ch := c.peek()

		if depth == 0 && isArgSeparator(ch) {
			break
		}

		switch ch {
		case '<':
			depth++
		case '>':
			if depth > 0 {
				depth--
			}
		}

		c.next()
	}

	return c.s[start:c.pos], nil
}

// matchBracket returns the index of the '>' that closes the '<' at s[i],
// counting nested brackets.
func matchBracket(s string, i int) (int, bool) {
	depth := 0

	for j := i; j < len(s); j++ {
		switch s[j] {
		case '<':
			depth++
		case '>':
			depth--
			if depth == 0 {
				return j, true
			}
		}
	}

	return 0, false
}

// bind matches a call's actual arguments to macro m's formal arguments
// and returns each formal argument's value, in the order the formal
// arguments are listed, and how many positional arguments the call gave.
//
// Positional arguments are matched in order. A keyword argument sets its
// own formal argument. When a formal argument gets both, the one later in
// the call wins. A formal argument left blank gets its default value, if
// the definition gives one, or, if it's a created local label, the next
// created label.
func (a *Assembler) bind(m *macroDef, actuals []actual) ([]string, int, error) {
	values := make([]string, len(m.formals))
	positional := 0

	for _, act := range actuals {
		if act.keyword != "" {
			values[m.formalIndex(act.keyword)] = act.text

			continue
		}

		if positional >= len(m.formals) {
			return nil, 0, vmserrors.New(vmserrors.VAX_TOOMNYARGS, m.name)
		}

		values[positional] = act.text
		positional++
	}

	for i, f := range m.formals {
		switch {
		case values[i] != "":
		case f.def != "":
			values[i] = f.def
		case f.created:
			values[i] = strconv.Itoa(a.createdLabel) + "$"
			a.createdLabel++
		}
	}

	return values, positional, nil
}

// isMacroNameChar reports whether ch can be part of a name when
// substituting arguments into a macro's line: a symbol character in
// either case (the line hasn't been uppercased yet), or ".".
func isMacroNameChar(ch byte) bool {
	return isSymbolChar(ch) || (ch >= 'a' && ch <= 'z') || ch == '.'
}

// substitute returns line, one raw line of macro m's body, with each
// formal argument's name replaced by its value from values. A name is
// only replaced whole: with a formal argument A, "A" and "X'A" are
// replaced but "AB" and "A.B" are not. Case doesn't matter.
//
// An apostrophe right before or after a replaced name is the
// concatenation operator, and is dropped. One that isn't next to a
// replaced name stays: in a macro that defines another macro, the inner
// macro's own "'NAME" has to survive the outer expansion.
func substitute(line string, m *macroDef, values []string) string {
	if len(m.formals) == 0 {
		return line
	}

	out := make([]byte, 0, len(line))

	for i := 0; i < len(line); {
		if !isMacroNameChar(line[i]) {
			out = append(out, line[i])
			i++

			continue
		}

		j := i
		for j < len(line) && isMacroNameChar(line[j]) {
			j++
		}

		k := m.formalIndex(strings.ToUpper(line[i:j]))
		if k < 0 {
			out = append(out, line[i:j]...)
			i = j

			continue
		}

		// Drop a concatenating apostrophe before the name (already
		// written) and after it (skipped).
		if n := len(out); n > 0 && out[n-1] == '\'' {
			out = out[:n-1]
		}

		out = append(out, values[k]...)

		if j < len(line) && line[j] == '\'' {
			j++
		}

		i = j
	}

	return string(out)
}
