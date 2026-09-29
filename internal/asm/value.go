package asm

import (
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/vmserrors"
)

// parseFloat reads a floating-point literal (digits, '.', a leading sign,
// and an exponent marker), matching asm_float()'s greedy character-class
// scan: it doesn't validate the shape of what it collects, just hands the
// substring to the native float parser the way asm_float hands its buffer
// to atof().
func (a *Assembler) parseFloat(c *cursor) (float64, error) {
	start := c.pos

	for {
		ch := c.peek()
		if isDigit(ch) || ch == '.' || ch == '-' || ch == '+' || ch == 'E' {
			c.next()

			continue
		}

		break
	}

	s := c.s[start:c.pos]
	if s == "" {
		return 0, vmserrors.New(vmserrors.VAX_BADFLOAT, s)
	}

	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, vmserrors.New(vmserrors.VAX_BADFLOAT, s)
	}

	return v, nil
}

// exprState carries the context that flows down through the expression
// grammar (exprTop/exprAtom) without needing package-level globals the
// way the C source threads it through vax.assembler fields.
type exprState struct {
	allowForward bool
}

// exprVal is an expression's value while it is being evaluated: a
// constant plus, when forward references are allowed, a coefficient for
// each symbol not yet defined. Addition, subtraction, negation, and
// multiplication by a constant keep that form, so exprValue can queue one
// fixup that completes the whole expression later (see fixup). Anything
// else applied to an undefined symbol (division, comparison, one
// undefined symbol times another) has no such form and is
// VAX-E-FWDOPERATOR.
type exprVal struct {
	v     uint32
	terms []pendingTerm
}

func constVal(v uint32) exprVal { return exprVal{v: v} }

func (x exprVal) forward() bool { return len(x.terms) > 0 }

// scaled returns x multiplied by k.
func (x exprVal) scaled(k uint32) exprVal {
	out := exprVal{v: x.v * k}
	for _, t := range x.terms {
		out.terms = addTerm(out.terms, t.key, t.coeff*k)
	}

	return out
}

// plus returns x + y.
func (x exprVal) plus(y exprVal) exprVal {
	out := exprVal{v: x.v + y.v, terms: append([]pendingTerm(nil), x.terms...)}
	for _, t := range y.terms {
		out.terms = addTerm(out.terms, t.key, t.coeff)
	}

	return out
}

// addTerm adds coeff×key to terms, merging with an existing term for the
// same symbol and dropping one whose coefficient cancels to zero (so
// "B-B" is just 0).
func addTerm(terms []pendingTerm, key string, coeff uint32) []pendingTerm {
	for i := range terms {
		if terms[i].key == key {
			terms[i].coeff += coeff
			if terms[i].coeff == 0 {
				terms = append(terms[:i], terms[i+1:]...)
			}

			return terms
		}
	}

	if coeff == 0 {
		return terms
	}

	return append(terms, pendingTerm{key: key, coeff: coeff})
}

// exprNoForward evaluates an expression with forward references disabled,
// matching a direct asm_expr() call outside of asm_value() (used by .BASE,
// .ALIGN, .SET's value, .SCB/.VECTOR's vector code, .IF, and friends — none
// of which route through asm_value's fixup machinery).
func (a *Assembler) exprNoForward(c *cursor) (uint32, error) {
	x, err := a.exprTop(c, &exprState{})

	return x.v, err
}

// exprValue evaluates an expression allowing forward references, matching
// asm_value(). If the expression uses a symbol not yet defined, a fixup of
// kind fx at location loc is queued for the whole expression (it becomes
// a.lastFixup), and the value returned is only its constant part, a
// placeholder. The reference tool could only complete a bare symbol, and
// rejected "SYM+8" or "B-A" with a forward reference as FWDOPERATOR.
func (a *Assembler) exprValue(c *cursor, loc uint32, fx fixupKind) (value uint32, wasForward bool, err error) {
	return a.exprValueOf(c, loc, fx, a.exprTop)
}

// exprValueOf is exprValue for the grammar level parse, so a caller can
// read a single term (a.exprAtom) rather than a whole expression.
func (a *Assembler) exprValueOf(c *cursor, loc uint32, fx fixupKind, parse func(*cursor, *exprState) (exprVal, error)) (value uint32, wasForward bool, err error) {
	x, err := parse(c, &exprState{allowForward: true})
	if err != nil {
		return 0, false, err
	}

	if !x.forward() {
		return x.v, false, nil
	}

	a.queueFixup(loc, fx, x.v, x.terms)

	return x.v, true, nil
}

// exprTop parses an expression: terms joined by binary operators, which
// in MACRO-32 all have the same priority and apply left to right, so
// 1+2*3 is 9 (<...> groups). The operators are + - * / and MACRO-32's
// arithmetic shift (@), logical AND (&), inclusive OR (!), and exclusive
// OR (\\). The reference tool (asm_expr/asm_expr_math/asm_expr2) gave *
// and / priority over + and -, had comparison operators (= <> < <= > >=)
// MACRO-32 doesn't, and lacked the other four. A symbol not yet defined
// can only be added, subtracted, or multiplied by a constant (see
// exprVal).
func (a *Assembler) exprTop(c *cursor, st *exprState) (exprVal, error) {
	x1, err := a.exprAtom(c, st)
	if err != nil {
		return exprVal{}, err
	}

	for {
		save := c.pos
		c.skipBlanks()

		ch := c.peek()
		if ch == 0 || !strings.ContainsRune("+-*/@&!\\", rune(ch)) {
			c.pos = save

			break
		}

		c.next()

		x2, err := a.exprAtom(c, st)
		if err != nil {
			return exprVal{}, err
		}

		switch {
		case ch == '+':
			x1 = x1.plus(x2)

		case ch == '-':
			x1 = x1.plus(x2.scaled(0xFFFFFFFF))

		case ch == '*' && !x2.forward():
			x1 = x1.scaled(x2.v)

		case ch == '*' && !x1.forward():
			x1 = x2.scaled(x1.v)

		case x1.forward() || x2.forward():
			return exprVal{}, vmserrors.New(vmserrors.VAX_FWDOPERATOR)

		case ch == '/' && x2.v == 0:
			return exprVal{}, vmserrors.New(vmserrors.VAX_DIVZERO)

		default:
			x1 = constVal(binaryOp(ch, x1.v, x2.v))
		}
	}

	return x1, nil
}

// binaryOp applies a binary operator to two known values. Division is
// unsigned, as the reference tool's was. A shift count is signed:
// positive shifts left, negative shifts right arithmetically.
func binaryOp(op byte, v1, v2 uint32) uint32 {
	switch op {
	case '/':
		return v1 / v2

	case '@':
		n := int32(v2)

		switch {
		case n >= 32:
			return 0
		case n >= 0:
			return v1 << uint(n)
		case n <= -32:
			return uint32(int32(v1) >> 31)
		default:
			return uint32(int32(v1) >> uint(-n))
		}

	case '&':
		return v1 & v2

	case '!':
		return v1 | v2
	}

	return v1 ^ v2
}

// exprAtom parses one expression atom: a parenthesized sub-expression, "."
// (the current location counter), a function call or symbol reference, a
// character literal, or a numeric constant. Matches asm_expr3(), plus a
// leading unary +/- that asm_expr3 doesn't support — the reference tool has
// no way to write a negative immediate constant at all (asm_hex/asm_dec
// have no unary-minus handling reachable from this position), which is a
// plain gap rather than an ISA-fidelity concern, and testdata/asm/forth.asm
// uses "#-1" — so this port adds it rather than leaving those fixtures
// unassemblable.
func (a *Assembler) exprAtom(c *cursor, st *exprState) (exprVal, error) {
	c.skipBlanks()

	switch c.peek() {
	case '-':
		c.next()
		x, err := a.exprAtom(c, st)

		return x.scaled(0xFFFFFFFF), err
	case '+':
		c.next()

		return a.exprAtom(c, st)
	case '.':
		if !isSymbolChar(c.peekAt(1)) {
			c.next()

			return constVal(a.deposit), nil
		}
	case '(':
		c.next()

		x, err := a.exprTop(c, st)
		if err != nil {
			return exprVal{}, err
		}

		c.skipBlanks()

		if c.peek() == ')' {
			c.next()
		}

		return x, nil

	case '<':
		return a.angleGroup(c, st)
	}

	if name, ok := scanLocalLabel(c); ok {
		return a.lookupSymbolValue(name, st)
	}

	if isUpperAlpha(c.peek()) || c.peek() == '_' || c.peek() == '$' {
		name := scanName(c)

		if v, matched, err := a.callFunction(name, c); matched {
			return constVal(v), err
		}

		return a.lookupSymbolValue(name, st)
	}

	return a.numericLiteral(c, st)
}

// angleGroup parses a MACRO-32 "<expression>" group, the cursor at its
// '<'. The reference tool had only parentheses for grouping.
func (a *Assembler) angleGroup(c *cursor, st *exprState) (exprVal, error) {
	c.next()

	x, err := a.exprTop(c, st)
	if err != nil {
		return exprVal{}, err
	}

	c.skipBlanks()

	if c.next() != '>' {
		return exprVal{}, vmserrors.New(vmserrors.VAX_NOCLOSE, ">")
	}

	return x, nil
}

// scanLocalLabel reads a MACRO-32 local label reference ("1$", "20$", ...)
// if one starts at the cursor: decimal digits, then "$", then a character
// that can't continue a name. Otherwise it consumes nothing. Without this
// the digits would be read as a number and the "$" left behind.
func scanLocalLabel(c *cursor) (string, bool) {
	n := 0
	for isDigit(c.peekAt(n)) {
		n++
	}

	if n == 0 || c.peekAt(n) != '$' || isSymbolChar(c.peekAt(n+1)) {
		return "", false
	}

	name := c.s[c.pos : c.pos+n+1]
	c.skip(n + 1)

	return name, true
}

// scanName reads a symbol/register/mnemonic-style name: letters, digits,
// '_' and '$'. The line has already been uppercased outside quoted regions
// by the time any parser sees it (see Assembler.preprocessLine).
func scanName(c *cursor) string {
	start := c.pos

	for isSymbolChar(c.peek()) {
		c.pos++
	}

	return c.s[start:c.pos]
}

// lookupSymbolValue reads a symbol in an expression. A symbol not yet
// defined, where forward references are allowed, becomes a term of the
// expression's value (see exprVal) instead of an error.
func (a *Assembler) lookupSymbolValue(name string, st *exprState) (exprVal, error) {
	resolved, local := a.resolvedName(name)

	sym, found := a.symbols.find(resolved)
	if found && local {
		sym.flags |= SymLocal
	}

	switch {
	case found && (len(sym.forward) == 0 || !st.allowForward):
		return constVal(sym.value), nil

	case !st.allowForward:
		return exprVal{}, vmserrors.New(vmserrors.VAX_UNDEFSYM, name)
	}

	return exprVal{terms: []pendingTerm{{key: resolved, coeff: 1}}}, nil
}

// numericLiteral parses a numeric constant, matching asm_hex()/asm_dec()'s
// combined behavior: a radix prefix (^D decimal, ^X/0X hex, ^M register
// mask), else digits in the current radix (decimal, as in MACRO-32; see
// Assembler.radix), or a character literal / symbol reference if the
// value doesn't start with a digit.
func (a *Assembler) numericLiteral(c *cursor, st *exprState) (exprVal, error) {
	c.skipBlanks()

	if c.atEnd() {
		return exprVal{}, vmserrors.New(vmserrors.VAX_INCOMPLETENUM)
	}

	if c.peek() == '^' {
		if x, handled, err := a.unaryOperator(c, st); handled {
			return x, err
		}
	}

	if c.peek() == '^' && c.peekAt(1) == 'D' {
		c.skip(2)

		return a.decimalLiteral(c, st)
	}

	if c.peek() == '^' && c.peekAt(1) == 'M' {
		c.skip(2)

		return constResult(a.maskLiteral(c))
	}

	if (c.peek() == '^' && c.peekAt(1) == 'X') || (c.peek() == '0' && c.peekAt(1) == 'X') {
		c.skip(2)

		return constResult(a.hexDigits(c))
	}

	if c.peek() == '^' && c.peekAt(1) == 'F' {
		return exprVal{}, vmserrors.New(vmserrors.VAX_FLOATHERE)
	}

	if a.radix == 10 {
		return a.decimalLiteral(c, st)
	}

	if a.radix != 16 && isDigit(c.peek()) {
		return constResult(radixDigits(c, a.radix))
	}

	if isUpperAlpha(c.peek()) || c.peek() == '_' || c.peek() == '$' {
		name := scanName(c)

		return a.lookupSymbolValue(name, st)
	}

	if c.peek() == '\'' {
		return constResult(a.charLiteral(c))
	}

	return constResult(a.hexDigits(c))
}

// unaryOperator handles the MACRO-32 unary operators the reference tool
// lacked, the cursor at their '^': ^A/text/ (ASCII), ^B (binary), ^O
// (octal), ^C (one's complement), and a radix operator (^B, ^O, ^D, ^X)
// applied to a whole <expression>. It reports false for the rest (^D and
// ^X on a number, ^M, ^F), which numericLiteral handles itself.
func (a *Assembler) unaryOperator(c *cursor, st *exprState) (exprVal, bool, error) {
	op := c.peekAt(1)

	base := map[byte]int{'B': 2, 'O': 8, 'D': 10, 'X': 16}[op]

	switch {
	case op == 'A':
		c.skip(2)
		v, err := a.asciiOperator(c)

		return constVal(v), true, err

	case op == 'C':
		c.skip(2)

		x, err := a.exprAtom(c, st)
		if err == nil && x.forward() {
			err = vmserrors.New(vmserrors.VAX_FWDOPERATOR)
		}

		return constVal(^x.v), true, err

	case base != 0 && c.peekAt(2) == '<':
		c.skip(2)

		saved := a.radix
		a.radix = base
		x, err := a.angleGroup(c, st)
		a.radix = saved

		return x, true, err

	case base == 2 || base == 8:
		c.skip(2)
		v, err := radixDigits(c, base)

		return constVal(v), true, err
	}

	return exprVal{}, false, nil
}

// asciiOperator reads the text of ^A/text/ (the cursor after the "^A"):
// up to four characters between two of the same delimiter, packed like a
// character literal, the first character in the low-order byte.
func (a *Assembler) asciiOperator(c *cursor) (uint32, error) {
	q := c.next()
	if q == 0 || isBlank(q) || q == ';' {
		return 0, vmserrors.New(vmserrors.VAX_BADSTRING, string(q))
	}

	var value uint32

	for size := 0; c.peek() != q; size++ {
		if c.atEnd() {
			return 0, vmserrors.New(vmserrors.VAX_NOCLOSE, string(q))
		}

		if size > 3 {
			return 0, vmserrors.New(vmserrors.VAX_CHARTOOLONG)
		}

		value |= uint32(c.next()) << (8 * uint(size))
	}

	c.next()

	return value, nil
}

// radixDigits reads unsigned digits in base 2 or 8.
func radixDigits(c *cursor, base int) (uint32, error) {
	value := uint32(0)
	digits := 0

	for isDigit(c.peek()) && int(c.peek()-'0') < base {
		value = value*uint32(base) + uint32(c.next()-'0')
		digits++
	}

	if digits == 0 || isDigit(c.peek()) || isUpperAlpha(c.peek()) || c.peek() == '_' {
		return 0, vmserrors.New(vmserrors.VAX_BADDIGIT, base)
	}

	return value, nil
}

// decimalLiteral parses an optionally-signed decimal integer, or falls back
// to a radix prefix/symbol/character literal — matching asm_dec().
func (a *Assembler) decimalLiteral(c *cursor, st *exprState) (exprVal, error) {
	c.skipBlanks()

	if (c.peek() == '^' && c.peekAt(1) == 'X') || (c.peek() == '0' && c.peekAt(1) == 'X') {
		c.skip(2)

		return constResult(a.hexDigits(c))
	}

	if c.peek() == '^' && c.peekAt(1) == 'D' {
		c.skip(2)
	}

	if c.peek() == '^' && c.peekAt(1) == 'F' {
		return exprVal{}, vmserrors.New(vmserrors.VAX_FLOATHERE)
	}

	if isUpperAlpha(c.peek()) || c.peek() == '_' || c.peek() == '$' {
		name := scanName(c)

		return a.lookupSymbolValue(name, st)
	}

	if c.peek() == '\'' {
		return constResult(a.charLiteral(c))
	}

	sign := int32(1)
	haveSign := false
	value := int32(0)
	digits := 0

	for {
		ch := c.peek()

		switch {
		case ch == '+' && !haveSign:
			haveSign = true

			c.next()

		case ch == '-' && !haveSign:
			haveSign = true
			sign = -1

			c.next()

		case isDigit(ch):
			// Matches asm_dec()'s own double-duty use of its sign flag:
			// reading a digit also closes off the leading-sign position,
			// so a '+'/'-' seen after this is a binary operator for the
			// caller (exprTop) to handle, not part of this number.
			haveSign = true
			value = value*10 + int32(ch-'0')
			digits++

			c.next()

		default:
			if digits == 0 {
				return exprVal{}, vmserrors.New(vmserrors.VAX_BADDECIMAL)
			}

			// "0FF" is a bad decimal number, not 0 followed by FF.
			if isUpperAlpha(ch) || ch == '_' {
				return exprVal{}, vmserrors.New(vmserrors.VAX_BADDIGIT, 10)
			}

			return constVal(uint32(value * sign)), nil
		}
	}
}

// constResult wraps a constant-only parser's result as an exprVal.
func constResult(v uint32, err error) (exprVal, error) {
	return constVal(v), err
}

// hexDigits parses unsigned hex digits with no sign or prefix handling —
// the tail end of asm_hex() reached once every special-case prefix has been
// ruled out.
func (a *Assembler) hexDigits(c *cursor) (uint32, error) {
	value := uint32(0)
	digits := 0

	for {
		ch := c.peek()

		switch {
		case ch >= '0' && ch <= '9':
			value = value<<4 + uint32(ch-'0')

		case ch >= 'A' && ch <= 'F':
			value = value<<4 + uint32(ch-'A') + 10

		default:
			if digits == 0 {
				return 0, vmserrors.New(vmserrors.VAX_BADHEX)
			}

			if isUpperAlpha(ch) || ch == '_' {
				return 0, vmserrors.New(vmserrors.VAX_BADDIGIT, 16)
			}

			return value, nil
		}

		digits++

		c.next()
	}
}

// charLiteral parses a 'x', 'xy', ... character-literal constant (up to 4
// characters, packed little-endian: the first character is the constant's
// low-order byte), with \n/\r/\t escapes — matching char_literal().
func (a *Assembler) charLiteral(c *cursor) (uint32, error) {
	var value uint32

	if c.peek() != '\'' {
		return 0, vmserrors.New(vmserrors.VAX_BADCHARLIT)
	}

	c.next()

	size := 0

	for c.peek() != '\'' {
		if c.atEnd() {
			return 0, vmserrors.New(vmserrors.VAX_UNTERMCHAR)
		}

		if size > 3 {
			return 0, vmserrors.New(vmserrors.VAX_CHARTOOLONG)
		}

		ch := c.next()
		if ch == '\\' {
			switch c.peek() {
			case 'n':
				ch = '\n'

			case 'r':
				ch = '\r'

			case 't':
				ch = '\t'

			default:
				ch = c.peek()
			}

			c.next()
		}

		value |= uint32(ch) << (8 * uint(size))
		size++
	}

	c.next() // closing quote

	return value, nil
}

// maskLiteral parses a "<R0,R1,...>" register-set mask (the argument to
// ^M, .MASK, and .ENTRY's second operand) into a 16-bit bitmask, matching
// asm_mask(). IV (integer overflow trap) is bit 15, DV (decimal overflow
// trap) is bit 14 — the two non-register bits an entry mask can carry.
func (a *Assembler) maskLiteral(c *cursor) (uint32, error) {
	c.skipBlanks()

	if c.peek() == '^' && c.peekAt(1) == 'M' {
		c.skip(2)
		c.skipBlanks()
	}

	if c.peek() != '<' {
		return 0, vmserrors.New(vmserrors.VAX_BADMASK)
	}

	c.next()

	var mask uint32

	for {
		c.skipBlanks()

		if c.peek() == ',' {
			c.next()

			continue
		}

		if c.peek() == '>' || c.atEnd() {
			break
		}

		var b []byte

		for !c.atEnd() && c.peek() != ',' && c.peek() != '>' {
			if ch := c.peek(); ch != ' ' {
				b = append(b, ch)
			}

			c.pos++
		}

		name := string(b)

		bit, ok := maskBit(name)
		if !ok {
			return 0, vmserrors.New(vmserrors.VAX_BADMASKENTRY, name)
		}

		mask |= 1 << bit
	}

	if c.peek() != '>' {
		return 0, vmserrors.New(vmserrors.VAX_BADMASK)
	}

	c.next()

	return mask, nil
}

var maskBits = map[string]uint{
	"R0": 0, "R1": 1, "R2": 2, "R3": 3,
	"R4": 4, "R5": 5, "R6": 6, "R7": 7,
	"R8": 8, "R9": 9, "R10": 10, "R11": 11,
	"IV": 15, "DV": 14,
}

func maskBit(name string) (uint, bool) {
	bit, ok := maskBits[name]

	return bit, ok
}
