package cpu

import "github.com/tucats/govax/internal/vax"

// This is the Go port of emul_integer_math.c's two handlers (the shared
// ADD/SUB/MUL/DIV/BIS/BIC/ADWC/SBWC routine, and the separate emul_xor).
// Rather than the C source's single routine that decomposes opcode.function
// into size/operation/operand-count at runtime, each opcode here gets its
// own small handler registered individually -- the decode table already
// carries operand count/size/access per opcode, so there's nothing left to
// decompose from the raw function byte (see Phase 03's design notes on
// dispatch). Condition codes use the wide-arithmetic helpers in
// condcodes.go rather than emul_integer_math.c's per-size formulas; see
// docs/DEVIATIONS.md for why.

func init() {
	reg := func(fn byte, h Handler) {
		instructionTable.SetHandler(instructionTable.Lookup(Opcode{Function: fn}), h)
	}
	for _, fn := range []byte{0x80, 0x81, 0xA0, 0xA1, 0xC0, 0xC1} { // ADDx2/3
		reg(fn, emulAdd)
	}
	for _, fn := range []byte{0x82, 0x83, 0xA2, 0xA3, 0xC2, 0xC3} { // SUBx2/3
		reg(fn, emulSub)
	}
	for _, fn := range []byte{0x84, 0x85, 0xA4, 0xA5, 0xC4, 0xC5} { // MULx2/3
		reg(fn, emulMul)
	}
	for _, fn := range []byte{0x86, 0x87, 0xA6, 0xA7, 0xC6, 0xC7} { // DIVx2/3
		reg(fn, emulDiv)
	}
	for _, fn := range []byte{0x88, 0x89, 0xA8, 0xA9, 0xC8, 0xC9} { // BISx2/3
		reg(fn, emulBis)
	}
	for _, fn := range []byte{0x8A, 0x8B, 0xAA, 0xAB, 0xCA, 0xCB} { // BICx2/3
		reg(fn, emulBic)
	}
	for _, fn := range []byte{0x8C, 0x8D, 0xAC, 0xAD, 0xCC, 0xCD} { // XORx2/3
		reg(fn, emulXor)
	}
	reg(0xD8, emulAdwc)
	reg(0xD9, emulSbwc)
}

// loadPair loads this family's first two operands, along with the size to
// compute at -- operand 0's declared size, which matches operand 1's for
// every opcode in this family except BISB3 (see docs/DEVIATIONS.md: the
// reference instruction table declares its destination operand, not
// involved in this computation, as longword-sized by what looks like a
// transcription error).
func loadPair(e *Engine, d *Decoded) (a, b uint64, size int, err error) {
	size = d.Operands[0].Size
	if a, err = d.Operands[0].Load(e.cpu, e.mem); err != nil {
		return
	}
	b, err = d.Operands[1].Load(e.cpu, e.mem)
	return
}

// storeResult writes result to this family's destination operand -- always
// the last one, matching emul_integer_math.c's put_operand(opcode,
// opcode->count - 1, ...) for both the 2-operand (in place) and 3-operand
// (separate destination) forms.
func storeResult(e *Engine, d *Decoded, result uint64) error {
	dst := d.Instruction.OperandCount - 1
	return d.Operands[dst].Store(e.cpu, e.mem, result)
}

// emulAdd is ADD{B,W,L}{2,3}: sum <- op0 + op1 (commutative -- order doesn't
// matter for either the value or the manual's V/C formulas), written to the
// last operand.
func emulAdd(e *Engine, d *Decoded) error {
	a, b, size, err := loadPair(e, d)
	if err != nil {
		return err
	}
	result, v, c := addResult(a, b, size)
	setArithPSL(e.cpu, result, v, c, size)
	return storeResult(e, d, result)
}

// emulSub is SUB{B,W,L}{2,3}: dif <- op1 - op0. op0 is always the
// subtrahend and op1 the minuend, true for both the 2-operand ("dif <- dif
// - sub") and 3-operand ("dif <- min - sub") forms per the manual's format
// lines.
func emulSub(e *Engine, d *Decoded) error {
	subtrahend, minuend, size, err := loadPair(e, d)
	if err != nil {
		return err
	}
	result, v, c := subResult(minuend, subtrahend, size)
	setArithPSL(e.cpu, result, v, c, size)
	return storeResult(e, d, result)
}

// emulMul is MUL{B,W,L}{2,3}: prod <- op0 * op1 (commutative), written to
// the last operand. C is always 0 for MUL, per the manual.
func emulMul(e *Engine, d *Decoded) error {
	a, b, size, err := loadPair(e, d)
	if err != nil {
		return err
	}
	result, v := mulResult(a, b, size)
	setArithPSL(e.cpu, result, v, false, size)
	return storeResult(e, d, result)
}

// emulDiv is DIV{B,W,L}{2,3}: quo <- op1 / op0. op0 is always the divisor
// and op1 the dividend, the same "first operand is the modifier" convention
// as SUB. C is always 0 for DIV; V covers both integer overflow
// (MinInt/-1) and division by zero, per the manual -- unlike
// emul_integer_math.c, which divides unconditionally before ever checking
// for either (undefined behavior in C; a runtime panic in Go, which is why
// this needed a guard in the first place).
func emulDiv(e *Engine, d *Decoded) error {
	divisor, dividend, size, err := loadPair(e, d)
	if err != nil {
		return err
	}
	result, v := divResult(dividend, divisor, size)
	setArithPSL(e.cpu, result, v, false, size)
	return storeResult(e, d, result)
}

// setLogicalPSL is the BIS/BIC/XOR condition-code shape: N/Z from the
// result, V <- 0, C left unaffected -- per the manual's BIS/BIC/XOR entries.
// emul_integer_math.c gets this wrong for both BIS (force-clears C) and BIC
// (computes a meaningless arithmetic-carry value instead of leaving C
// alone); see docs/DEVIATIONS.md.
func setLogicalPSL(cpu *vax.CPU, result uint64, size int) {
	psl := cpu.PSL()
	psl.SetN(signBit(result, size))
	psl.SetZ(isZero(result, size))
	psl.SetV(false)
	cpu.SetPSL(psl)
}

// emulBis is BIS{B,W,L}{2,3}: dst <- op0 | op1 (commutative), written to the
// last operand.
func emulBis(e *Engine, d *Decoded) error {
	a, b, size, err := loadPair(e, d)
	if err != nil {
		return err
	}
	result := maskToSize(int64(a|b), size)
	setLogicalPSL(e.cpu, result, size)
	return storeResult(e, d, result)
}

// emulBic is BIC{B,W,L}{2,3}: dst <- op1 AND NOT op0 -- op0 is always the
// mask, the same "first operand is the modifier" convention as SUB/DIV --
// written to the last operand.
func emulBic(e *Engine, d *Decoded) error {
	mask, val, size, err := loadPair(e, d)
	if err != nil {
		return err
	}
	result := maskToSize(int64(val&^mask), size)
	setLogicalPSL(e.cpu, result, size)
	return storeResult(e, d, result)
}

// emulXor is XOR{B,W,L}{2,3}: dst <- op0 ^ op1 (commutative), written to the
// last operand. Port of emul_xor, a separate C handler from the rest of
// this family though its operand shape and condition codes are identical.
func emulXor(e *Engine, d *Decoded) error {
	a, b, size, err := loadPair(e, d)
	if err != nil {
		return err
	}
	result := maskToSize(int64(a^b), size)
	setLogicalPSL(e.cpu, result, size)
	return storeResult(e, d, result)
}

// emulAdwc is ADWC: sum <- sum + add + C. Word-sized, matching the C source
// and the generated instruction table rather than the manual's longword
// format line -- see docs/DEVIATIONS.md's ADWC/SBWC entry.
func emulAdwc(e *Engine, d *Decoded) error {
	addend, sum, size, err := loadPair(e, d)
	if err != nil {
		return err
	}
	carryIn := e.cpu.PSL().C()
	result, v, c := addCarryResult(addend, sum, carryIn, size)
	setArithPSL(e.cpu, result, v, c, size)
	return storeResult(e, d, result)
}

// emulSbwc is SBWC: dif <- dif - sub - C. Word-sized, see emulAdwc.
func emulSbwc(e *Engine, d *Decoded) error {
	subtrahend, minuend, size, err := loadPair(e, d)
	if err != nil {
		return err
	}
	borrowIn := e.cpu.PSL().C()
	result, v, c := subCarryResult(minuend, subtrahend, borrowIn, size)
	setArithPSL(e.cpu, result, v, c, size)
	return storeResult(e, d, result)
}
