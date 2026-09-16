package asm

import (
	"strconv"

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

// exprState carries the context that flows down through the four
// expression-grammar levels (exprTop/exprMath/exprTerm/exprAtom, matching
// asm_expr/asm_expr_math/asm_expr2/asm_expr3) without needing package-level
// globals the way the C source threads it through vax.assembler fields.
//
// usedOperator/wasForward together replicate asm_value()'s safety check:
// the assembler's forward-reference fixups can only patch in "this location
// equals a symbol's value", not an arbitrary expression, so an expression
// that both combines an operator *and* touches a forward-referenced symbol
// can't be resolved correctly later and is rejected up front instead of
// silently assembling the wrong bytes (see exprValue).
type exprState struct {
	allowForward bool
	loc          uint32
	fx           fixupKind
	usedOperator bool
	wasForward   bool
}

// exprNoForward evaluates an expression with forward references disabled,
// matching a direct asm_expr() call outside of asm_value() (used by .BASE,
// .ALIGN, .SET's value, .SCB/.VECTOR's vector code, .IF, and friends — none
// of which route through asm_value's fixup machinery).
func (a *Assembler) exprNoForward(c *cursor) (uint32, error) {
	st := &exprState{}
	return a.exprTop(c, st)
}

// exprValue evaluates an expression allowing one forward-referenced symbol,
// matching asm_value(): a bare forward reference queues a fixup of kind fx
// at location loc, but an expression that combines a forward reference with
// any operator is rejected (see exprState's doc comment) rather than
// producing a value that can never be corrected later.
func (a *Assembler) exprValue(c *cursor, loc uint32, fx fixupKind) (value uint32, wasForward bool, err error) {
	st := &exprState{allowForward: true, loc: loc, fx: fx}

	v, err := a.exprTop(c, st)
	if err != nil {
		return 0, false, err
	}
	if st.usedOperator && st.wasForward {
		return 0, false, vmserrors.New(vmserrors.VAX_FWDOPERATOR)
	}

	return v, st.wasForward, nil
}

func boolToU32(b bool) uint32 {
	if b {
		return 1
	}

	return 0
}

// exprTop parses conditional/comparison operators: = <> <= < >= >. Matches
// asm_expr().
func (a *Assembler) exprTop(c *cursor, st *exprState) (uint32, error) {
	v1, err := a.exprMath(c, st)
	if err != nil {
		return 0, err
	}

	for {
		save := c.pos
		c.skipBlanks()

		op := 0

		switch {
		case c.peek() == '=':
			op = 1

			c.next()

		case c.peek() == '<' && c.peekAt(1) == '>':
			op = 2

			c.skip(2)

		case c.peek() == '<' && c.peekAt(1) == '=':
			op = 3

			c.skip(2)

		case c.peek() == '<':
			op = 4

			c.next()

		case c.peek() == '>' && c.peekAt(1) == '=':
			op = 5

			c.skip(2)

		case c.peek() == '>':
			op = 6

			c.next()
		}

		if op == 0 {
			c.pos = save

			break
		}

		v2, err := a.exprMath(c, st)
		if err != nil {
			return 0, err
		}

		st.usedOperator = true

		switch op {
		case 1:
			v1 = boolToU32(v1 == v2)

		case 2:
			v1 = boolToU32(v1 != v2)

		case 3:
			v1 = boolToU32(int32(v1) <= int32(v2))

		case 4:
			v1 = boolToU32(int32(v1) < int32(v2))

		case 5:
			v1 = boolToU32(int32(v1) >= int32(v2))

		case 6:
			v1 = boolToU32(int32(v1) > int32(v2))
		}
	}

	return v1, nil
}

// exprMath parses +/-. Matches asm_expr_math().
func (a *Assembler) exprMath(c *cursor, st *exprState) (uint32, error) {
	v1, err := a.exprTerm(c, st)
	if err != nil {
		return 0, err
	}

	for {
		save := c.pos
		c.skipBlanks()

		ch := c.peek()
		if ch != '+' && ch != '-' {
			c.pos = save

			break
		}

		c.next()

		v2, err := a.exprTerm(c, st)
		if err != nil {
			return 0, err
		}

		st.usedOperator = true

		if ch == '+' {
			v1 += v2
		} else {
			v1 -= v2
		}
	}

	return v1, nil
}

// exprTerm parses */. Matches asm_expr2().
func (a *Assembler) exprTerm(c *cursor, st *exprState) (uint32, error) {
	v1, err := a.exprAtom(c, st)
	if err != nil {
		return 0, err
	}

	for {
		save := c.pos
		c.skipBlanks()

		ch := c.peek()
		if ch != '*' && ch != '/' {
			c.pos = save

			break
		}

		c.next()

		v2, err := a.exprAtom(c, st)
		if err != nil {
			return 0, err
		}

		st.usedOperator = true
		
		if ch == '*' {
			v1 *= v2
		} else {
			if v2 == 0 {
				return 0, vmserrors.New(vmserrors.VAX_DIVZERO)
			}

			v1 /= v2
		}
	}

	return v1, nil
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
func (a *Assembler) exprAtom(c *cursor, st *exprState) (uint32, error) {
	c.skipBlanks()

	switch c.peek() {
	case '-':
		c.next()
		v, err := a.exprAtom(c, st)

		return uint32(-int32(v)), err
	case '+':
		c.next()

		return a.exprAtom(c, st)
	case '.':
		if !isSymbolChar(c.peekAt(1)) {
			c.next()

			return a.deposit, nil
		}
	case '(':
		c.next()

		v, err := a.exprTop(c, st)
		if err != nil {
			return 0, err
		}

		c.skipBlanks()
		if c.peek() == ')' {
			c.next()
		}

		return v, nil
	}

	if isUpperAlpha(c.peek()) || c.peek() == '_' || c.peek() == '$' {
		name := scanName(c)

		if v, matched, err := a.callFunction(name, c); matched {
			return v, err
		}

		return a.lookupSymbolValue(name, st)
	}

	return a.numericLiteral(c, st)
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

func (a *Assembler) lookupSymbolValue(name string, st *exprState) (uint32, error) {
	v, wasForward, err := a.getSymbol(name, st.allowForward, st.loc, st.fx)
	if wasForward {
		st.wasForward = true
	}

	return v, err
}

// numericLiteral parses a numeric constant, matching asm_hex()/asm_dec()'s
// combined behavior: a radix prefix (^D decimal, ^X/0X hex, ^M register
// mask), else digits in the assembler's current radix (hex by default,
// matching the reference console's own default — see initialization.c), or
// a character literal / symbol reference if the value doesn't start with a
// digit.
func (a *Assembler) numericLiteral(c *cursor, st *exprState) (uint32, error) {
	c.skipBlanks()
	if c.atEnd() {
		return 0, vmserrors.New(vmserrors.VAX_INCOMPLETENUM)
	}

	if c.peek() == '^' && c.peekAt(1) == 'D' {
		c.skip(2)

		return a.decimalLiteral(c, st)
	}

	if c.peek() == '^' && c.peekAt(1) == 'M' {
		c.skip(2)

		return a.maskLiteral(c)
	}

	if (c.peek() == '^' && c.peekAt(1) == 'X') || (c.peek() == '0' && c.peekAt(1) == 'X') {
		c.skip(2)

		return a.hexDigits(c)
	}

	if c.peek() == '^' && c.peekAt(1) == 'F' {
		return 0, vmserrors.New(vmserrors.VAX_FLOATHERE)
	}

	if a.radix == 10 {
		return a.decimalLiteral(c, st)
	}

	if isUpperAlpha(c.peek()) || c.peek() == '_' || c.peek() == '$' {
		name := scanName(c)

		return a.lookupSymbolValue(name, st)
	}
	if c.peek() == '\'' {
		return a.charLiteral(c)
	}

	return a.hexDigits(c)
}

// decimalLiteral parses an optionally-signed decimal integer, or falls back
// to a radix prefix/symbol/character literal — matching asm_dec().
func (a *Assembler) decimalLiteral(c *cursor, st *exprState) (uint32, error) {
	c.skipBlanks()

	if (c.peek() == '^' && c.peekAt(1) == 'X') || (c.peek() == '0' && c.peekAt(1) == 'X') {
		c.skip(2)

		return a.hexDigits(c)
	}
	if c.peek() == '^' && c.peekAt(1) == 'D' {
		c.skip(2)
	}
	if c.peek() == '^' && c.peekAt(1) == 'F' {
		return 0, vmserrors.New(vmserrors.VAX_FLOATHERE)
	}
	if isUpperAlpha(c.peek()) || c.peek() == '_' || c.peek() == '$' {
		name := scanName(c)

		return a.lookupSymbolValue(name, st)
	}
	if c.peek() == '\'' {
		return a.charLiteral(c)
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
			// caller (exprMath) to handle, not part of this number.
			haveSign = true
			value = value*10 + int32(ch-'0')
			digits++

			c.next()

		default:
			if digits == 0 {
				return 0, vmserrors.New(vmserrors.VAX_BADDECIMAL)
			}

			return uint32(value * sign), nil
		}
	}
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
