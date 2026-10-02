package asm

import "math/big"

// octa is a 128-bit integer value: the widest the VAX has (an octaword).
// Go has no 128-bit integer type, so it's two 64-bit halves; lo holds bits
// 0-63 and hi bits 64-127. Memory holds an octaword low-order byte first,
// so lo is stored before hi.
//
// The assembler's expression evaluator works in 32 bits, as MACRO-32's
// does, so most values start as a longword and are widened by
// signExtendOcta. Only a literal that is a single number (wideLiteral) can
// be wider: .QUAD and .OCTA, and the immediate operand of an instruction
// that reads a quadword or octaword (MOVQ #..., MOVO #...).
type octa struct {
	lo, hi uint64
}

// signExtendOcta widens a 32-bit value to 128 bits, copying its sign bit
// (bit 31) into every bit above it, so -1 stays -1.
func signExtendOcta(v uint32) octa {
	lo := uint64(int64(int32(v)))

	return octa{lo: lo, hi: uint64(int64(lo) >> 63)}
}

// fitsShortLiteral reports whether o can be a short literal: 0 through 63.
// A short literal packs the value into the six low bits of the operand
// specifier byte itself, so it saves the bytes of an immediate.
func (o octa) fitsShortLiteral() bool {
	return o.hi == 0 && o.lo < 64
}

// twoTo128 is 2^128, the modulus for wrapping a value into 128 bits.
var twoTo128 = new(big.Int).Lsh(big.NewInt(1), 128)

// wideLiteral reads an item that is exactly one numeric literal (an
// optional sign, then digits in the current radix or after a ^X, 0X, or ^D
// prefix), followed by a ',' or the end of the line, at up to 128 bits. A
// value too wide keeps its low 128 bits, and a negative one is stored in
// two's complement, as a VAX holds negative integers. If the item isn't a
// single number, it leaves c where it was and reports false, so the caller
// can evaluate it as an expression instead.
//
// math/big's Int is an integer of any size, so the digits can be
// accumulated without overflowing before the value is cut down to 128
// bits.
func (a *Assembler) wideLiteral(c *cursor) (octa, bool) {
	save := c.pos
	c.skipBlanks()

	neg := false
	if c.peek() == '-' || c.peek() == '+' {
		neg = c.next() == '-'
	}

	base := int64(a.radix)

	switch {
	case c.peek() == '^' && c.peekAt(1) == 'X', c.peek() == '0' && c.peekAt(1) == 'X':
		base = 16

		c.skip(2)

	case c.peek() == '^' && c.peekAt(1) == 'D':
		base = 10

		c.skip(2)
	}

	v := new(big.Int)
	bigBase := big.NewInt(base)
	digits := 0

	for {
		ch := c.peek()

		var d int64

		switch {
		case isDigit(ch):
			d = int64(ch - '0')
		case base == 16 && ch >= 'A' && ch <= 'F':
			d = int64(ch-'A') + 10
		default:
			d = base
		}

		if d >= base {
			break
		}

		v.Mul(v, bigBase)
		v.Add(v, big.NewInt(d))
		digits++

		c.next()
	}

	c.skipBlanks()

	if digits == 0 || (!c.atEnd() && c.peek() != ',') {
		c.pos = save

		return octa{}, false
	}

	if neg {
		v.Neg(v)
	}

	// Mod, unlike Rem, always gives a result from 0 to 2^128-1, which is
	// the two's-complement bit pattern of a negative value.
	v.Mod(v, twoTo128)

	lo := new(big.Int).And(v, new(big.Int).SetUint64(^uint64(0)))

	return octa{lo: lo.Uint64(), hi: new(big.Int).Rsh(v, 64).Uint64()}, true
}

// pseudoOcta assembles .OCTA: a comma-separated list of 128-bit values, 16
// bytes each, each read by wideItem, as .QUAD's are.
func (a *Assembler) pseudoOcta(c *cursor) error {
	first := true

	for {
		c.skipBlanks()

		if err := a.listSeparator(c, first); err != nil || c.atEnd() {
			return err
		}

		first = false

		v, err := a.wideItem(c)
		if err != nil {
			return err
		}

		if err := a.emitOcta(v); err != nil {
			return err
		}
	}
}

// emitOcta stores v's 16 bytes, as four longwords, low-order first.
func (a *Assembler) emitOcta(v octa) error {
	for _, l := range []uint32{uint32(v.lo), uint32(v.lo >> 32), uint32(v.hi), uint32(v.hi >> 32)} {
		if err := a.emitLongword(l); err != nil {
			return err
		}
	}

	return nil
}
