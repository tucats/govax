package cpu

import "math/big"

// CVTPS, CVTSP, CVTPT, and CVTTP: conversions between packed decimal and
// numeric strings (docs/PHASE-35.md, subtask 12).
//
// A leading separate numeric string is a sign byte, ASCII "+", "-", or
// (as a source only) a space, followed by its length's worth of ASCII
// digits. A trailing numeric string is its length's worth of ASCII bytes
// whose last carries both the last digit and the sign: an "overpunched"
// character such as "{" (+0) or "J" (-1), which CVTPT and CVTTP translate
// through a 256-byte table the program supplies.
//
// A conversion to a numeric string sets N and Z from the source's value,
// as the manual says; one to a packed string sets them from the result.
// Either way V is set on decimal overflow (digits that don't fit; the
// low-order ones are stored), with the DV trap after, and C is cleared.

func init() {
	reg := func(fn byte, h Handler) {
		instructionTable.SetHandler(instructionTable.Lookup(Opcode{Function: fn}), h)
	}
	reg(0x08, emulCvtps) // CVTPS
	reg(0x09, emulCvtsp) // CVTSP
	reg(0x24, emulCvtpt) // CVTPT
	reg(0x26, emulCvttp) // CVTTP
}

// fitDigits returns the last length of digits, padded with leading zeros
// if there are fewer, and whether a nonzero digit was dropped (a decimal
// overflow).
func fitDigits(digits []byte, length int) ([]byte, bool) {
	if len(digits) <= length {
		out := make([]byte, length-len(digits), length)

		return append(out, digits...), false
	}

	drop := len(digits) - length

	return digits[drop:], !allZero(digits[:drop])
}

// asciiDigits returns digit nibbles as ASCII characters. (An invalid
// nibble, over 9, comes out as the character that many past "0", as on
// VMS: its EDITPC turned a B nibble into ";".)
func asciiDigits(digits []byte) []byte {
	out := make([]byte, len(digits))
	for i, n := range digits {
		out[i] = '0' + n
	}

	return out
}

// storeBytes writes bytes at addr.
func (e *Engine) storeBytes(addr uint32, bytes []byte) error {
	for i, b := range bytes {
		if err := e.mem.StoreByte(e.cpu, addr+uint32(i), b); err != nil {
			return err
		}
	}

	return nil
}

// loadBytes reads n bytes at addr.
func (e *Engine) loadBytes(addr uint32, n int) ([]byte, error) {
	out := make([]byte, n)

	for i := range out {
		b, err := e.mem.LoadByte(e.cpu, addr+uint32(i))
		if err != nil {
			return nil, err
		}

		out[i] = b
	}

	return out, nil
}

// setSourceCC sets the condition codes for a conversion to a numeric
// string: N and Z from the packed source's value, V as given, C clear.
func setSourceCC(e *Engine, v *big.Int, overflow bool) {
	psl := e.cpu.PSL()
	psl.SetN(v.Sign() < 0)
	psl.SetZ(v.Sign() == 0)
	psl.SetV(overflow)
	psl.SetC(false)
	e.cpu.SetPSL(psl)
}

// emulCvtps is CVTPS srclen, srcaddr, dstlen, dstaddr: packed to leading
// separate numeric. The sign byte is "+" or "-" by the source's value (so
// -0 gives "+"). R0 = 0, R1 = srcaddr, R2 = 0, R3 = dstaddr.
func emulCvtps(e *Engine, d *Decoded) error {
	ops, err := e.packedOperands(d, 0, 2)
	if err != nil {
		return err
	}

	src, dst := ops[0], ops[1]

	p, err := e.readPacked(src.addr, src.length)
	if err != nil {
		return err
	}

	v := p.value()
	digits, overflow := fitDigits(p.digits, dst.length)

	sign := byte('+')
	if v.Sign() < 0 {
		sign = '-'
	}

	if err := e.storeBytes(dst.addr, append([]byte{sign}, asciiDigits(digits)...)); err != nil {
		return err
	}

	e.setRegisters(0, src.addr, 0, dst.addr)
	setSourceCC(e, v, overflow)

	return e.decimalOverflowTrap(overflow)
}

// emulCvtsp is CVTSP srclen, srcaddr, dstlen, dstaddr: leading separate
// numeric to packed. A sign byte other than "+", "-", or a space, or a
// digit byte other than "0"-"9", is a reserved operand, before anything
// changes. R0 = 0, R1 = srcaddr (the sign byte), R2 = 0, R3 = dstaddr.
func emulCvtsp(e *Engine, d *Decoded) error {
	ops, err := e.packedOperands(d, 0, 2)
	if err != nil {
		return err
	}

	src, dst := ops[0], ops[1]

	raw, err := e.loadBytes(src.addr, src.length+1)
	if err != nil {
		return err
	}

	var neg bool

	switch raw[0] {
	case '+', ' ':
	case '-':
		neg = true
	default:
		return &Fault{Code: ExcReservedOp}
	}

	digits := make([]byte, src.length)

	for i, ch := range raw[1:] {
		if ch < '0' || ch > '9' {
			return &Fault{Code: ExcReservedOp}
		}

		digits[i] = ch - '0'
	}

	return e.storeConverted(dst, digits, neg, src.addr)
}

// storeConverted stores digits (with sign neg) as the packed destination
// dst, as CVTSP and CVTTP do: overflow keeps the low-order digits, a zero
// value is plus, and N and Z come from the result. R0 = 0, R1 = srcaddr,
// R2 = 0, R3 = dst's address.
func (e *Engine) storeConverted(dst packedOperand, digits []byte, neg bool, srcaddr uint32) error {
	fitted, overflow := fitDigits(digits, dst.length)
	neg = neg && !allZero(digits)

	if err := e.writePacked(dst.addr, fitted, neg); err != nil {
		return err
	}

	e.setRegisters(0, srcaddr, 0, dst.addr)
	setDecimalCC(e.cpu, fitted, neg, overflow, false)

	return e.decimalOverflowTrap(overflow)
}

// emulCvtpt is CVTPT srclen, srcaddr, tbladdr, dstlen, dstaddr: packed to
// trailing numeric. The source's last byte (its last digit and sign, even
// for -0) indexes the table, and the byte there is the destination's last;
// the other destination bytes are the remaining digits in ASCII. R0 = 0,
// R1 = srcaddr, R2 = 0, R3 = dstaddr.
func emulCvtpt(e *Engine, d *Decoded) error {
	ops, err := e.packedOperands(d, 0, 3)
	if err != nil {
		return err
	}

	src, dst := ops[0], ops[1]
	table := d.Operands[2].Addr

	p, err := e.readPacked(src.addr, src.length)
	if err != nil {
		return err
	}

	last, err := e.mem.LoadByte(e.cpu, src.addr+packedSize(src.length)-1)
	if err != nil {
		return err
	}

	trailing, err := e.mem.LoadByte(e.cpu, table+uint32(last))
	if err != nil {
		return err
	}

	v := p.value()

	// The digits before the last one go in the destination's bytes before
	// its last; a zero-length destination has room for none, not even the
	// last digit.
	var (
		out      []byte
		overflow bool
	)

	if dst.length == 0 {
		overflow = v.Sign() != 0
	} else {
		var high []byte
		if len(p.digits) > 0 {
			high = p.digits[:len(p.digits)-1]
		}

		fitted, over := fitDigits(high, dst.length-1)
		out = append(asciiDigits(fitted), trailing)
		overflow = over
	}

	if err := e.storeBytes(dst.addr, out); err != nil {
		return err
	}

	e.setRegisters(0, src.addr, 0, dst.addr)
	setSourceCC(e, v, overflow)

	return e.decimalOverflowTrap(overflow)
}

// emulCvttp is CVTTP srclen, srcaddr, tbladdr, dstlen, dstaddr: trailing
// numeric to packed. The source's last byte, through the table, gives the
// last digit (high nibble) and the sign (low nibble); the other source
// bytes give their low 4 bits as digits. A source byte before the last
// that isn't "0"-"9", or a translation whose digit isn't 0-9 or whose
// sign isn't ^XA-^XF, is a reserved operand. R0 = 0, R1 = srcaddr, R2 =
// 0, R3 = dstaddr.
func emulCvttp(e *Engine, d *Decoded) error {
	ops, err := e.packedOperands(d, 0, 3)
	if err != nil {
		return err
	}

	src, dst := ops[0], ops[1]
	table := d.Operands[2].Addr

	raw, err := e.loadBytes(src.addr, src.length)
	if err != nil {
		return err
	}

	digits := make([]byte, 0, src.length)
	neg := false

	for i, ch := range raw {
		if i < len(raw)-1 {
			if ch < '0' || ch > '9' {
				return &Fault{Code: ExcReservedOp}
			}

			digits = append(digits, ch&0xF)

			continue
		}

		t, err := e.mem.LoadByte(e.cpu, table+uint32(ch))
		if err != nil {
			return err
		}

		digit, sign := t>>4, t&0xF
		if digit > 9 || sign < 0xA {
			return &Fault{Code: ExcReservedOp}
		}

		digits = append(digits, digit)
		neg = sign == 0xB || sign == 0xD
	}

	return e.storeConverted(dst, digits, neg, src.addr)
}
