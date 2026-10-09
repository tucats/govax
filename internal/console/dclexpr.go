package console

import (
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/vmserrors"
)

// DCL expressions (docs/PHASE-50 - DCL command procedures.md, subtask 9),
// as the OpenVMS User's Manual (7.3) describes them in sections 12.5 to
// 12.9: what the right side of "=" and "==" is, and (later) what IF,
// WRITE, and a lexical function's arguments are.
//
// An expression's operands are integers, quoted strings, symbols, and
// lexical function calls; its value is an integer or a string. These are
// the operators, highest precedence first (12.8.5); operators of the same
// precedence work left to right, and parentheses group:
//
//	7  unary + and -
//	6  * and /
//	5  + and - (on two strings: concatenation and reduction)
//	4  the comparisons: .EQ. .NE. .LT. .LE. .GT. .GE. (numbers) and
//	   .EQS. .NES. .LTS. .LES. .GTS. .GES. (strings)
//	3  .NOT.
//	2  .AND.
//	1  .OR.
//
// Which type a result has follows 12.8.6's table. A string where an
// integer is wanted is converted by 12.9.1's rules (dclValue.Int), and an
// integer where a string is wanted, as in a string comparison, becomes
// its decimal digits (dclValue.String). Integers are longwords: arithmetic
// wraps around without an error, as 12.7 says it does.
//
// A symbol in an expression is replaced by its value, once (12.13.4):
// an undefined one is CLI_UNDSYM (12.13.5).
//
// Unconfirmed against VMS, and asked by the probe in testdata/dcl50:
// what division by zero does (govax: CLI_DIVZERO); how a string with
// blanks around a number, or a radix prefix, converts to an integer
// (govax: blanks are ignored, and "%X10" is 16); a quoted string with no
// closing quote (govax: CLI_UNTERMSTR); whether a name that starts with
// F$ is always a lexical function (govax: yes, so a symbol can't be
// named that way in an expression); and every message's segment line.

// dclValue is the value of a DCL expression or symbol: an integer or a
// string.
type dclValue struct {
	// integer says the value is the integer num; otherwise it's the
	// string str.
	integer bool
	num     int32
	str     string
}

// dclInteger returns the integer value n.
func dclInteger(n int32) dclValue { return dclValue{integer: true, num: n} }

// dclString returns the string value s.
func dclString(s string) dclValue { return dclValue{str: s} }

// dclBool returns DCL's value for a truth: 1 for true, 0 for false.
func dclBool(b bool) dclValue {
	if b {
		return dclInteger(1)
	}

	return dclInteger(0)
}

// Int returns v as an integer. A string is converted by the User's
// Manual's rules (12.9.1): one that holds a number is that number; one
// that starts with T, t, Y, or y is 1 (true); any other is 0, the empty
// string among them.
func (v dclValue) Int() int32 {
	if v.integer {
		return v.num
	}

	text := strings.TrimSpace(v.str)

	sign := int64(1)

	switch {
	case strings.HasPrefix(text, "-"):
		sign, text = -1, strings.TrimSpace(text[1:])
	case strings.HasPrefix(text, "+"):
		text = strings.TrimSpace(text[1:])
	}

	if n, ok := parseDCLNumber(text); ok {
		return int32(sign * int64(n))
	}

	if text != "" && strings.ContainsRune("TtYy", rune(text[0])) {
		return 1
	}

	return 0
}

// String returns v as a string: an integer's decimal digits, with a
// minus sign if it's negative (12.9.2).
func (v dclValue) String() string {
	if v.integer {
		return strconv.FormatInt(int64(v.num), 10)
	}

	return v.str
}

// parseDCLNumber reads text, all of it, as a DCL integer: decimal digits,
// or digits after %X (hexadecimal), %O (octal), or %D (decimal), in
// either case. A number too big for a longword keeps its low 32 bits, as
// DCL reports no error for one (12.7).
func parseDCLNumber(text string) (uint32, bool) {
	digits, base := text, uint64(10)

	if len(text) >= 2 && text[0] == '%' {
		switch text[1] {
		case 'X', 'x':
			base = 16
		case 'O', 'o':
			base = 8
		case 'D', 'd':
			base = 10
		default:
			return 0, false
		}

		digits = text[2:]
	}

	if digits == "" {
		return 0, false
	}

	var n uint64

	for i := 0; i < len(digits); i++ {
		d, ok := digitValue(digits[i])
		if !ok || uint64(d) >= base {
			return 0, false
		}

		n = (n*base + uint64(d)) & 0xFFFFFFFF
	}

	return uint32(n), true
}

// dclOperator is one of DCL's dotted operators, such as .EQS.
type dclOperator struct {
	// precedence is the operator's place in 12.8.5's table.
	precedence int

	// apply computes a binary operator's result from its operands.
	apply func(a, b dclValue) dclValue
}

// Precedences, from 12.8.5's table.
const (
	precOr      = 1
	precAnd     = 2
	precNot     = 3
	precCompare = 4
)

// dclOperators are the dotted operators, by name without the dots.
// .NOT. is unary, so its apply is unused; the expression parser handles
// it by name.
var dclOperators = map[string]dclOperator{
	"OR":  {precOr, func(a, b dclValue) dclValue { return dclInteger(a.Int() | b.Int()) }},
	"AND": {precAnd, func(a, b dclValue) dclValue { return dclInteger(a.Int() & b.Int()) }},
	"NOT": {precNot, nil},

	"EQ": {precCompare, func(a, b dclValue) dclValue { return dclBool(a.Int() == b.Int()) }},
	"NE": {precCompare, func(a, b dclValue) dclValue { return dclBool(a.Int() != b.Int()) }},
	"LT": {precCompare, func(a, b dclValue) dclValue { return dclBool(a.Int() < b.Int()) }},
	"LE": {precCompare, func(a, b dclValue) dclValue { return dclBool(a.Int() <= b.Int()) }},
	"GT": {precCompare, func(a, b dclValue) dclValue { return dclBool(a.Int() > b.Int()) }},
	"GE": {precCompare, func(a, b dclValue) dclValue { return dclBool(a.Int() >= b.Int()) }},

	"EQS": {precCompare, func(a, b dclValue) dclValue { return dclBool(a.String() == b.String()) }},
	"NES": {precCompare, func(a, b dclValue) dclValue { return dclBool(a.String() != b.String()) }},
	"LTS": {precCompare, func(a, b dclValue) dclValue { return dclBool(a.String() < b.String()) }},
	"LES": {precCompare, func(a, b dclValue) dclValue { return dclBool(a.String() <= b.String()) }},
	"GTS": {precCompare, func(a, b dclValue) dclValue { return dclBool(a.String() > b.String()) }},
	"GES": {precCompare, func(a, b dclValue) dclValue { return dclBool(a.String() >= b.String()) }},
}

// dclExpression is the state of evaluating one expression: its text, the
// place reached in it, and where its symbols and lexical functions come
// from.
type dclExpression struct {
	text string
	pos  int

	// symbols is the symbol table names are looked up in.
	symbols *dclSymbolTable

	// console is the console a lexical function asks about the system;
	// nil for a subprocess's CLI, whose lexical functions are those that
	// need no console.
	console *Console
}

// evaluateDCLExpression evaluates text, all of it, as a DCL expression,
// with symbols from symbols. A "!" outside quotes starts a comment, which
// ends the expression.
func evaluateDCLExpression(text string, symbols *dclSymbolTable, c *Console) (dclValue, error) {
	e := &dclExpression{text: text, symbols: symbols, console: c}

	e.skipBlanks()

	if e.atEnd() {
		return dclValue{}, vmserrors.New(vmserrors.CLI_EXPSYN, strings.TrimSpace(text))
	}

	v, err := e.binary(precOr)
	if err != nil {
		return dclValue{}, err
	}

	if e.skipBlanks(); !e.atEnd() {
		return dclValue{}, e.syntaxError()
	}

	return v, nil
}

// skipBlanks moves past blanks and tabs.
func (e *dclExpression) skipBlanks() {
	for e.pos < len(e.text) && (e.text[e.pos] == ' ' || e.text[e.pos] == '\t') {
		e.pos++
	}
}

// atEnd reports whether the expression's text is used up: its end, or a
// comment.
func (e *dclExpression) atEnd() bool {
	return e.pos >= len(e.text) || e.text[e.pos] == '!'
}

// rest is the text not yet read, for a message.
func (e *dclExpression) rest() string {
	return strings.TrimSpace(e.text[min(e.pos, len(e.text)):])
}

// syntaxError is CLI_EXPSYN at the place reached.
func (e *dclExpression) syntaxError() error {
	return vmserrors.New(vmserrors.CLI_EXPSYN, e.rest())
}

// peekOperator returns the binary operator at the place reached, if
// there is one, its precedence, and its length; ok is false at anything
// else. A dotted name that isn't an operator is CLI_IVOPER.
func (e *dclExpression) peekOperator() (op dclOperator, name string, length int, ok bool, err error) {
	e.skipBlanks()

	if e.atEnd() {
		return op, "", 0, false, nil
	}

	switch ch := e.text[e.pos]; ch {
	case '+', '-':
		return dclOperator{precedence: 5}, string(ch), 1, true, nil
	case '*', '/':
		return dclOperator{precedence: 6}, string(ch), 1, true, nil
	case '.':
		name, length, err := e.dottedName()
		if err != nil {
			return op, "", 0, false, err
		}

		op, ok := dclOperators[name]
		if !ok || name == "NOT" {
			return op, "", 0, false, nil
		}

		return op, name, length, true, nil
	}

	return op, "", 0, false, nil
}

// dottedName reads the dotted operator at the place reached, without
// moving past it: its name, uppercase and without the dots, and its
// length. One DCL doesn't have is CLI_IVOPER.
func (e *dclExpression) dottedName() (string, int, error) {
	end := e.pos + 1
	for end < len(e.text) && isLetter(e.text[end]) {
		end++
	}

	if end >= len(e.text) || e.text[end] != '.' || end == e.pos+1 {
		return "", 0, vmserrors.New(vmserrors.CLI_IVOPER, e.rest())
	}

	name := strings.ToUpper(e.text[e.pos+1 : end])
	if _, ok := dclOperators[name]; !ok {
		return "", 0, vmserrors.New(vmserrors.CLI_IVOPER, e.text[e.pos:end+1])
	}

	return name, end + 1 - e.pos, nil
}

// isLetter reports whether ch is an ASCII letter.
func isLetter(ch byte) bool {
	return (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z')
}

// binary evaluates the operators of precedence min and higher, left to
// right, as 12.8.5 orders them.
func (e *dclExpression) binary(minPrec int) (dclValue, error) {
	if minPrec == precNot {
		return e.not()
	}

	// Unary plus and minus (precedence 7) are above every binary
	// operator.
	if minPrec >= 7 {
		return e.unary()
	}

	left, err := e.binary(minPrec + 1)
	if err != nil {
		return dclValue{}, err
	}

	for {
		op, name, length, ok, err := e.peekOperator()
		if err != nil {
			return dclValue{}, err
		}

		if !ok || op.precedence != minPrec {
			return left, nil
		}

		e.pos += length

		right, err := e.binary(minPrec + 1)
		if err != nil {
			return dclValue{}, err
		}

		if left, err = applyOperator(op, name, left, right); err != nil {
			return dclValue{}, err
		}
	}
}

// applyOperator applies the binary operator name (op) to left and right.
// "+" and "-" on two strings are concatenation and reduction (12.6.3);
// with an integer on either side they're arithmetic, as "*" and "/"
// always are.
func applyOperator(op dclOperator, name string, left, right dclValue) (dclValue, error) {
	bothStrings := !left.integer && !right.integer

	switch name {
	case "+":
		if bothStrings {
			return dclString(left.str + right.str), nil
		}

		return dclInteger(left.Int() + right.Int()), nil

	case "-":
		if bothStrings {
			return dclString(strings.Replace(left.str, right.str, "", 1)), nil
		}

		return dclInteger(left.Int() - right.Int()), nil

	case "*":
		return dclInteger(left.Int() * right.Int()), nil

	case "/":
		divisor := right.Int()
		if divisor == 0 {
			return dclValue{}, vmserrors.New(vmserrors.CLI_DIVZERO)
		}

		// Go's division truncates toward zero, as DCL's does ("8
		// divided by 3 equals 2", 12.7.1), and the one quotient too big
		// for a longword, the most negative one divided by -1, wraps.
		return dclInteger(left.Int() / divisor), nil
	}

	return op.apply(left, right), nil
}

// not evaluates .NOT., which comes between the comparisons and .AND. in
// precedence (12.8.5): ".NOT. A .EQ. B" is ".NOT. (A .EQ. B)".
func (e *dclExpression) not() (dclValue, error) {
	e.skipBlanks()

	if !e.atEnd() && e.text[e.pos] == '.' {
		name, length, err := e.dottedName()
		if err != nil {
			return dclValue{}, err
		}

		if name == "NOT" {
			e.pos += length

			v, err := e.not()
			if err != nil {
				return dclValue{}, err
			}

			return dclInteger(^v.Int()), nil
		}
	}

	return e.binary(precCompare)
}

// unary evaluates unary plus and minus, the operators of highest
// precedence, and then an operand. Either makes the value an integer.
func (e *dclExpression) unary() (dclValue, error) {
	e.skipBlanks()

	if !e.atEnd() && (e.text[e.pos] == '+' || e.text[e.pos] == '-') {
		negate := e.text[e.pos] == '-'
		e.pos++

		v, err := e.unary()
		if err != nil {
			return dclValue{}, err
		}

		if negate {
			return dclInteger(-v.Int()), nil
		}

		return dclInteger(v.Int()), nil
	}

	return e.operand()
}

// operand evaluates one operand: a parenthesized expression, a quoted
// string, a number, a lexical function call, or a symbol.
func (e *dclExpression) operand() (dclValue, error) {
	e.skipBlanks()

	if e.atEnd() {
		return dclValue{}, e.syntaxError()
	}

	switch ch := e.text[e.pos]; {
	case ch == '(':
		e.pos++

		v, err := e.binary(precOr)
		if err != nil {
			return dclValue{}, err
		}

		if e.skipBlanks(); e.pos >= len(e.text) || e.text[e.pos] != ')' {
			return dclValue{}, e.syntaxError()
		}

		e.pos++

		return v, nil

	case ch == '"':
		s, n, ok := quotedString(e.text[e.pos:])
		if !ok {
			return dclValue{}, vmserrors.New(vmserrors.CLI_UNTERMSTR)
		}

		e.pos += n

		return dclString(s), nil

	case ch == '%' || (ch >= '0' && ch <= '9'):
		return e.number()

	case isDCLSymbolStart(ch):
		name := e.name()
		if len(name) > 2 && strings.EqualFold(name[:2], "F$") {
			return e.lexicalCall(name)
		}

		sym, ok := e.symbols.lookup(name)
		if !ok {
			return dclValue{}, vmserrors.New(vmserrors.CLI_UNDSYM, strings.ToUpper(name))
		}

		return sym.dclValue(), nil
	}

	return dclValue{}, e.syntaxError()
}

// name reads the symbol or function name at the place reached.
func (e *dclExpression) name() string {
	start := e.pos
	for e.pos < len(e.text) && isDCLNameChar(e.text[e.pos]) {
		e.pos++
	}

	return e.text[start:e.pos]
}

// isDCLNameChar reports whether ch can be in a symbol's name as an
// expression uses it: a letter, digit, "$", or "_".
func isDCLNameChar(ch byte) bool {
	return isDCLSymbolStart(ch) || (ch >= '0' && ch <= '9')
}

// number reads an integer: digits, or a radix prefix and digits. The
// number goes on as far as letters and digits do, so "12AB" is one
// number, with a digit decimal doesn't have (CLI_IVCHAR).
func (e *dclExpression) number() (dclValue, error) {
	start := e.pos
	if e.text[e.pos] == '%' {
		e.pos++
	}

	for e.pos < len(e.text) && isDCLNameChar(e.text[e.pos]) {
		e.pos++
	}

	text := e.text[start:e.pos]

	n, ok := parseDCLNumber(text)
	if !ok {
		return dclValue{}, vmserrors.New(vmserrors.CLI_IVCHAR, strings.ToUpper(text))
	}

	return dclInteger(int32(n)), nil
}

// dclValue returns the symbol's value as an expression sees it: an
// integer for a symbol given one by "=" or "==", a string otherwise.
func (sym dclSymbol) dclValue() dclValue {
	if !sym.integer {
		return dclString(sym.value)
	}

	n, _ := strconv.ParseInt(sym.value, 10, 32)

	return dclInteger(int32(n))
}
