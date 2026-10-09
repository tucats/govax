package console

import (
	"math"
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
// VMS 7.3's run of testdata/dcl50 settled what the manual doesn't say:
// division by zero is 2147483647, with no message; a string converts to
// an integer with blanks around the number ignored, a sign, and a radix
// prefix ("%X10" is 16); a quoted string with no closing quote runs to
// the end of the line; a string token goes on through quotes and letters
// ("A"B"C" is one token); an operator's closing dot may be left out
// (".EQ 1"); .NOT. may follow another operator; a name starting with F$
// is a lexical function only when "(" follows it; and a ")" after a
// whole expression is CLI_SYMDEL, after the value is assigned.
// Unconfirmed: the quotient of a negative number divided by zero.

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
// ends the expression. A whole expression followed by a ")" or ","
// returns its value in a trailingDelimiter error (CLI_SYMDEL): VMS
// assigns the value and then reports the delimiter.
func evaluateDCLExpression(text string, symbols *dclSymbolTable, c *Console) (dclValue, error) {
	e := &dclExpression{text: text, symbols: symbols, console: c}

	e.skipBlanks()

	if e.atEnd() {
		return dclValue{}, vmserrors.New(vmserrors.CLI_EXPSYN)
	}

	v, err := e.binary(precOr)
	if err != nil {
		return dclValue{}, err
	}

	if e.skipBlanks(); !e.atEnd() {
		if ch := e.text[e.pos]; ch == ')' || ch == ',' {
			return dclValue{}, trailingDelimiter{value: v}
		}

		return dclValue{}, e.syntaxError()
	}

	return v, nil
}

// trailingDelimiter is the error for an expression followed by a stray
// delimiter: CLI_SYMDEL, with the expression's value, which an
// assignment still assigns.
type trailingDelimiter struct {
	value dclValue
}

func (e trailingDelimiter) Error() string { return vmserrors.New(vmserrors.CLI_SYMDEL).Error() }

func (e trailingDelimiter) Unwrap() error { return vmserrors.New(vmserrors.CLI_SYMDEL) }

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

// syntaxError is CLI_EXPSYN, which DCL shows without a segment line.
func (e *dclExpression) syntaxError() error {
	return vmserrors.New(vmserrors.CLI_EXPSYN)
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
// length. The closing dot may be left out (".EQ 1" is ".EQ. 1"). One DCL
// doesn't have is CLI_IVOPER, with the operator as its segment.
func (e *dclExpression) dottedName() (string, int, error) {
	end := e.pos + 1
	for end < len(e.text) && isLetter(e.text[end]) {
		end++
	}

	closed := end < len(e.text) && e.text[end] == '.'

	name := strings.ToUpper(e.text[e.pos+1 : end])
	if _, ok := dclOperators[name]; !ok || name == "" {
		segment := e.text[e.pos:end]
		switch {
		case closed:
			segment = e.text[e.pos : end+1]
		case name == "":
			segment = e.rest()
		}

		return "", 0, vmserrors.NewSegment(vmserrors.CLI_IVOPER, segment)
	}

	if closed {
		end++
	}

	return name, end - e.pos, nil
}

// isLetter reports whether ch is an ASCII letter.
func isLetter(ch byte) bool {
	return (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z')
}

// binary evaluates the operators of precedence min and higher, left to
// right, as 12.8.5 orders them.
func (e *dclExpression) binary(minPrec int) (dclValue, error) {
	// .NOT. is read as an operand (unary), so this level is the
	// comparisons'.
	if minPrec == precNot {
		return e.binary(precCompare)
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
			// VMS 7.3's answer for 7 / 0, with no message.
			return dclInteger(math.MaxInt32), nil
		}

		// Go's division truncates toward zero, as DCL's does ("8
		// divided by 3 equals 2", 12.7.1), and the one quotient too big
		// for a longword, the most negative one divided by -1, wraps.
		return dclInteger(left.Int() / divisor), nil
	}

	return op.apply(left, right), nil
}

// unary evaluates unary plus and minus, the operators of highest
// precedence, and .NOT., and then an operand. Each makes the value an
// integer. .NOT. comes between the comparisons and .AND. in precedence
// (12.8.5), so it takes a comparison as its operand: ".NOT. A .EQ. B" is
// ".NOT. (A .EQ. B)". It may follow another operator, as in
// "1 .EQ. .NOT. 0", which VMS evaluates as "1 .EQ. (.NOT. 0)".
func (e *dclExpression) unary() (dclValue, error) {
	e.skipBlanks()

	if e.atEnd() {
		return e.operand()
	}

	switch e.text[e.pos] {
	case '+', '-':
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

	case '.':
		name, length, err := e.dottedName()
		if err != nil {
			return dclValue{}, err
		}

		if name != "NOT" {
			return dclValue{}, e.syntaxError()
		}

		e.pos += length

		v, err := e.binary(precCompare)
		if err != nil {
			return dclValue{}, err
		}

		return dclInteger(^v.Int()), nil
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
		return dclString(e.stringToken()), nil

	case ch == '%' || (ch >= '0' && ch <= '9'):
		return e.number()

	case isDCLSymbolStart(ch):
		name := e.name()
		if len(name) > 2 && strings.EqualFold(name[:2], "F$") && e.parenNext() {
			return e.lexicalCall(name)
		}

		sym, ok := e.symbols.lookup(name)
		if !ok {
			return dclValue{}, vmserrors.NewSegment(vmserrors.CLI_UNDSYM, name)
		}

		return sym.dclValue(), nil
	}

	return dclValue{}, e.syntaxError()
}

// parenNext reports whether the next thing, after any blanks, is "(".
func (e *dclExpression) parenNext() bool {
	i := e.pos
	for i < len(e.text) && (e.text[i] == ' ' || e.text[i] == '\t') {
		i++
	}

	return i < len(e.text) && e.text[i] == '('
}

// stringToken reads the string token at the place reached, which starts
// with a quote. Quoted text keeps its case and blanks, with "" for a
// quote inside it, and a quote with no closing one runs to the end of
// the line. The token goes on through more quoted text and letters,
// digits, "$", and "_" (uppercased) until something else: VMS takes
// ""quoted"" as one token, the string QUOTED.
func (e *dclExpression) stringToken() string {
	var b strings.Builder

	for e.pos < len(e.text) {
		ch := e.text[e.pos]

		switch {
		case ch == '"':
			s, n, closed := quotedString(e.text[e.pos:])
			if !closed {
				// No closing quote: the rest of the line.
				s = strings.ReplaceAll(e.text[e.pos+1:], `""`, `"`)
			}

			b.WriteString(s)
			e.pos += n

		case isDCLNameChar(ch):
			b.WriteString(strings.ToUpper(e.name()))

		default:
			return b.String()
		}
	}

	return b.String()
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
		return dclValue{}, vmserrors.NewSegment(vmserrors.CLI_IVCHAR, text)
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
