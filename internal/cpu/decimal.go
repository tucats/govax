package cpu

import (
	"math/big"

	"github.com/tucats/govax/internal/vax"
)

// Packed decimal strings, and the CPU's helpers for the decimal string
// instructions (docs/PHASE-35.md, subtasks 10-13).
//
// A packed decimal string is given by a length, the number of decimal
// digits (0 to 31), and the address of its first byte. Each byte holds
// two 4-bit nibbles, the most significant digit first, and the last
// byte's low nibble is the sign: ^XA, ^XC, ^XE, or ^XF for plus, ^XB or
// ^XD for minus. The VAX writes ^XC and ^XD, the "preferred" signs. A
// string of length n takes n/2+1 bytes; when n is even, the first byte's
// high nibble is unused, and the VAX writes it as zero. So -123 is ^X12,
// ^X3D, and +1234 is ^X01, ^X23, ^X4C.
//
// The manual makes a digit nibble of ^XA to ^XF, or an unused high
// nibble that isn't zero, UNPREDICTABLE. govax does what VMS 7.1 on a
// VAX 8600 did with the Phase 35 probe (testdata/insn35): an instruction
// that copies digits copies such a nibble as it is (MOVP of ^X1A,^X3C
// moves it unchanged), and one that does arithmetic uses its value (CVTPL
// of "4C6" is 4*100 + 12*10 + 6 = 526).
//
// A length over 31 is a reserved operand fault, before anything changes.
//
// # A fault part way
//
// Like the character string instructions (MOVC3 and the rest), these
// don't model PSL<FPD>: an access violation or other memory fault part
// way through leaves the registers unchanged and the instruction is
// started again from the beginning once the handler returns. Each reads
// all its source strings first, works out the result, writes the
// destination, and only then sets the registers the manual lists, so a
// restart repeats the same work and gets the same result (unless the
// destination overlaps a source, which the manual makes UNPREDICTABLE
// for most of them anyway).

// Arithmetic exception type codes the decimal instructions raise
// (traps: the instruction has completed).
const (
	trapDivideByZero = 0x04 // floating or decimal divide by zero (DIVP)
	trapDecOvf       = 0x06 // decimal overflow, under PSL<DV>
)

// maxDecimalDigits is the longest packed decimal string, in digits.
const maxDecimalDigits = 31

// packedSize returns the number of bytes a packed decimal string of
// length digits takes.
func packedSize(length int) uint32 { return uint32(length/2 + 1) }

// decimal is a packed decimal string as read: its digit nibbles, most
// significant first (one per digit of its length; a nibble over 9 is kept
// as it is), and whether its sign nibble is a minus.
type decimal struct {
	digits []byte
	neg    bool
}

// value returns d's value: its nibbles as decimal digits (an invalid
// nibble counts its own value, 10 to 15), negated if the sign is minus.
func (d decimal) value() *big.Int {
	v := new(big.Int)
	ten := big.NewInt(10)

	for _, n := range d.digits {
		v.Mul(v, ten)
		v.Add(v, big.NewInt(int64(n)))
	}

	if d.neg {
		v.Neg(v)
	}

	return v
}

// allZero reports whether every one of digits is zero.
func allZero(digits []byte) bool {
	for _, n := range digits {
		if n != 0 {
			return false
		}
	}

	return true
}

// decimalLength loads decoded instruction d's operand i, a string length,
// and checks it: over 31 is a reserved operand.
func (e *Engine) decimalLength(d *Decoded, i int) (int, error) {
	raw, err := d.Operands[i].Load(e.cpu, e.mem)
	if err != nil {
		return 0, err
	}

	length := int(uint16(raw))
	if length > maxDecimalDigits {
		return 0, &Fault{Code: ExcReservedOp}
	}

	return length, nil
}

// readPacked reads the packed decimal string of length digits at addr.
func (e *Engine) readPacked(addr uint32, length int) (decimal, error) {
	size := packedSize(length)
	raw := make([]byte, size)

	for i := range raw {
		b, err := e.mem.LoadByte(e.cpu, addr+uint32(i))
		if err != nil {
			return decimal{}, err
		}

		raw[i] = b
	}

	// The nibbles in order, skipping an even length's unused one.
	nibbles := make([]byte, 0, 2*len(raw))
	for _, b := range raw {
		nibbles = append(nibbles, b>>4, b&0xF)
	}

	sign := nibbles[len(nibbles)-1]
	digits := nibbles[len(nibbles)-1-length : len(nibbles)-1]

	return decimal{digits: digits, neg: sign == 0xB || sign == 0xD}, nil
}

// writePacked writes a packed decimal string of len(digits) digits at
// addr, with the preferred sign (^XD if neg, else ^XC) and an even
// length's unused high nibble zero.
func (e *Engine) writePacked(addr uint32, digits []byte, neg bool) error {
	sign := byte(0xC)
	if neg {
		sign = 0xD
	}

	nibbles := make([]byte, 0, len(digits)+2)
	if len(digits)%2 == 0 {
		nibbles = append(nibbles, 0)
	}

	nibbles = append(nibbles, digits...)
	nibbles = append(nibbles, sign)

	for i := 0; i < len(nibbles); i += 2 {
		if err := e.mem.StoreByte(e.cpu, addr+uint32(i/2), nibbles[i]<<4|nibbles[i+1]); err != nil {
			return err
		}
	}

	return nil
}

// decimalDigits returns v as length decimal digits, most significant
// first: its low-order digits if it has more (a decimal overflow, also
// reported), or with leading zeros if fewer. neg is v's sign, which a
// result keeps even when its stored digits are all zero after an
// overflow; a zero v is never negative.
func decimalDigits(v *big.Int, length int) (digits []byte, neg, overflow bool) {
	text := new(big.Int).Abs(v).String()
	if text == "0" {
		text = ""
	}

	if len(text) > length {
		overflow = true
		text = text[len(text)-length:]
	}

	digits = make([]byte, length)
	offset := length - len(text)

	for i, ch := range text {
		digits[offset+i] = byte(ch - '0')
	}

	return digits, v.Sign() < 0, overflow
}

// setDecimalCC sets the condition codes from a decimal result: N if it's
// negative and not all zeros (-0 isn't less than zero), Z if its digits
// are all zero, V as given (decimal or integer overflow), and C cleared
// unless keepC.
func setDecimalCC(cpu *vax.CPU, digits []byte, neg, overflow, keepC bool) {
	zero := allZero(digits)

	psl := cpu.PSL()
	psl.SetN(neg && !zero)
	psl.SetZ(zero)
	psl.SetV(overflow)

	if !keepC {
		psl.SetC(false)
	}

	cpu.SetPSL(psl)
}

// decimalOverflowTrap returns the decimal overflow trap if overflow and
// PSL<DV> are both set, or nil.
func (e *Engine) decimalOverflowTrap(overflow bool) error {
	if overflow && e.cpu.PSL().DV() {
		return e.arithmeticTrap(trapDecOvf)
	}

	return nil
}

// setRegisters sets R0, R1, ... to values, in order.
func (e *Engine) setRegisters(values ...uint32) {
	for i, v := range values {
		e.cpu.SetGPR(vax.R0+vax.Reg(i), v)
	}
}

// ---------------------------------------------------------------------
// MOVP, CMPP3, CMPP4, CVTLP, and CVTPL.

func init() {
	reg := func(fn byte, h Handler) {
		instructionTable.SetHandler(instructionTable.Lookup(Opcode{Function: fn}), h)
	}
	reg(0x34, emulMovp)  // MOVP
	reg(0x35, emulCmpp3) // CMPP3
	reg(0x37, emulCmpp4) // CMPP4
	reg(0xF9, emulCvtlp) // CVTLP
	reg(0x36, emulCvtpl) // CVTPL
}

// emulMovp is MOVP len, srcaddr, dstaddr: the destination string is
// replaced by the source string, its digit nibbles copied as they are
// (even invalid ones), with the preferred sign; -0 becomes +0. N and Z
// from the result, V cleared, C unaffected. R0 = 0, R1 = srcaddr, R2 = 0,
// R3 = dstaddr.
func emulMovp(e *Engine, d *Decoded) error {
	length, err := e.decimalLength(d, 0)
	if err != nil {
		return err
	}

	src, dst := d.Operands[1].Addr, d.Operands[2].Addr

	p, err := e.readPacked(src, length)
	if err != nil {
		return err
	}

	neg := p.neg && !allZero(p.digits)

	if err := e.writePacked(dst, p.digits, neg); err != nil {
		return err
	}

	e.setRegisters(0, src, 0, dst)
	setDecimalCC(e.cpu, p.digits, neg, false, true)

	return nil
}

// comparePacked sets the condition codes for src1 compared with src2, by
// value: N if src1 < src2, Z if they're equal (so -0 equals +0), V and C
// cleared. R0 = 0, R1 = src1addr, R2 = 0, R3 = src2addr.
func (e *Engine) comparePacked(len1 int, addr1 uint32, len2 int, addr2 uint32) error {
	p1, err := e.readPacked(addr1, len1)
	if err != nil {
		return err
	}

	p2, err := e.readPacked(addr2, len2)
	if err != nil {
		return err
	}

	cmp := p1.value().Cmp(p2.value())

	e.setRegisters(0, addr1, 0, addr2)

	psl := e.cpu.PSL()
	psl.SetN(cmp < 0)
	psl.SetZ(cmp == 0)
	psl.SetV(false)
	psl.SetC(false)
	e.cpu.SetPSL(psl)

	return nil
}

// emulCmpp3 is CMPP3 len, src1addr, src2addr: both strings have length
// len.
func emulCmpp3(e *Engine, d *Decoded) error {
	length, err := e.decimalLength(d, 0)
	if err != nil {
		return err
	}

	return e.comparePacked(length, d.Operands[1].Addr, length, d.Operands[2].Addr)
}

// emulCmpp4 is CMPP4 src1len, src1addr, src2len, src2addr.
func emulCmpp4(e *Engine, d *Decoded) error {
	len1, err := e.decimalLength(d, 0)
	if err != nil {
		return err
	}

	len2, err := e.decimalLength(d, 2)
	if err != nil {
		return err
	}

	return e.comparePacked(len1, d.Operands[1].Addr, len2, d.Operands[3].Addr)
}

// emulCvtlp is CVTLP src, dstlen, dstaddr: the longword as a packed
// decimal string. On decimal overflow the destination gets the low-order
// digits (with the true sign), V is set, and under PSL<DV> a decimal
// overflow trap follows. N and Z from the result, C cleared. R0 = R1 =
// R2 = 0, R3 = dstaddr.
func emulCvtlp(e *Engine, d *Decoded) error {
	raw, err := d.Operands[0].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}

	length, err := e.decimalLength(d, 1)
	if err != nil {
		return err
	}

	dst := d.Operands[2].Addr
	digits, neg, overflow := decimalDigits(big.NewInt(signExtend(raw, 4)), length)

	if err := e.writePacked(dst, digits, neg); err != nil {
		return err
	}

	e.setRegisters(0, 0, 0, dst)
	setDecimalCC(e.cpu, digits, neg, overflow, false)

	return e.decimalOverflowTrap(overflow)
}

// emulCvtpl is CVTPL srclen, srcaddr, dst: the string's value as a
// longword. On integer overflow the destination gets the low-order 32
// bits, V is set, and under PSL<IV> an integer overflow trap follows. N
// and Z from the destination, C cleared. The registers (R0 = 0, R1 =
// srcaddr, R2 = R3 = 0) are set before the destination is stored, so the
// destination may be one of them.
func emulCvtpl(e *Engine, d *Decoded) error {
	length, err := e.decimalLength(d, 0)
	if err != nil {
		return err
	}

	src := d.Operands[1].Addr

	p, err := e.readPacked(src, length)
	if err != nil {
		return err
	}

	result, overflow := lowOrderBits(p.value(), 4)

	e.setRegisters(0, src, 0, 0)

	if err := d.Operands[2].Store(e.cpu, e.mem, result); err != nil {
		return err
	}

	setArithPSL(e.cpu, result, overflow, false, 4)

	if overflow && e.cpu.PSL().IV() {
		return e.arithmeticTrap(trapIntOvf)
	}

	return nil
}
