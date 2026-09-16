package cpu

import "github.com/tucats/govax/internal/vax"

// This is the Go port of emul_float_math.c's arithmetic paths (dsize 2/3,
// func 0-3): ADD/SUB/MUL/DIV for both F_floating and D_floating, 2- and
// 3-operand forms. Unlike the C source's single opcode-decomposing routine,
// each operation gets its own handler, registered across every opcode
// variant -- the same "decode already carries what's needed" reasoning
// Phase 04's integermath.go used. loadFloat/storeFloat (fpu.go) already
// dispatch on operand size, so one handler per operation naturally covers
// both F_floating and D_floating.

func init() {
	reg := func(fn byte, h Handler) {
		instructionTable.SetHandler(instructionTable.Lookup(Opcode{Function: fn}), h)
	}

	for _, fn := range []byte{0x40, 0x41, 0x60, 0x61} { // ADDF2/3, ADDD2/3
		reg(fn, emulFAdd)
	}

	for _, fn := range []byte{0x42, 0x43, 0x62, 0x63} { // SUBF2/3, SUBD2/3
		reg(fn, emulFSub)
	}

	for _, fn := range []byte{0x44, 0x45, 0x64, 0x65} { // MULF2/3, MULD2/3
		reg(fn, emulFMul)
	}

	for _, fn := range []byte{0x46, 0x47, 0x66, 0x67} { // DIVF2/3, DIVD2/3
		reg(fn, emulFDiv)
	}
}

// loadFloatPair loads this family's first two operands as float64s (operand
// 0, then operand 1 -- emul_float_math.c's float1/float2).
func loadFloatPair(e *Engine, d *Decoded) (float1, float2 float64, err error) {
	if float1, err = loadFloat(e.cpu, e.mem, d.Operands[0]); err != nil {
		return
	}

	float2, err = loadFloat(e.cpu, e.mem, d.Operands[1])

	return
}

// storeFloatResult writes result to this family's destination operand --
// always the last one, matching storeResult's integer counterpart
// (integermath.go).
func storeFloatResult(e *Engine, d *Decoded, result float64) error {
	dst := d.Instruction.OperandCount - 1

	return storeFloat(e.cpu, e.mem, d.Operands[dst], result)
}

// setFloatPSL sets N/Z from result, and V/C <- 0 -- the manual's ADDF/SUBF/
// MULF/DIVF (and D-floating counterparts) condition-code entry. Genuine
// floating overflow is always a synchronous fault (storeFloatResult/
// fpuStore), so V is never anything but 0 on this function's surviving
// path.
//
// emul_float_math.c doesn't actually clear V/C at all on the normal path
// (leaving them stale from whatever the previous instruction set), and on
// F_floating overflow specifically sets V<-1 but then, unlike the D_floating
// path two lines below it, doesn't propagate the fpu_store fault -- it
// falls through to write a garbage/uninitialized destination and returns
// success. Both are clear-cut bugs (an omitted clear matching the same
// SETCONDITIONBITS-idiom pattern Phase 04 already found and fixed
// repeatedly elsewhere; an inconsistency between two nearly-identical
// branches of the same if/else), fixed directly rather than replicated --
// see docs/DEVIATIONS.md.
func setFloatPSL(cpu *vax.CPU, result float64) {
	psl := cpu.PSL()
	psl.SetN(result < 0.0)
	psl.SetZ(result == 0.0)
	psl.SetV(false)
	psl.SetC(false)
	cpu.SetPSL(psl)
}

// emulFAdd is ADDF{2,3}/ADDD{2,3}: sum <- op1 + op0 (commutative), written
// to the last operand.
func emulFAdd(e *Engine, d *Decoded) error {
	op0, op1, err := loadFloatPair(e, d)
	if err != nil {
		return err
	}

	result := op1 + op0
	setFloatPSL(e.cpu, result)

	return storeFloatResult(e, d, result)
}

// emulFSub is SUBF{2,3}/SUBD{2,3}: dif <- op1 - op0. op0 is always the
// subtrahend, matching integer SUB's "first operand is the modifier"
// convention for both the 2- and 3-operand forms.
func emulFSub(e *Engine, d *Decoded) error {
	subtrahend, minuend, err := loadFloatPair(e, d)
	if err != nil {
		return err
	}

	result := minuend - subtrahend
	setFloatPSL(e.cpu, result)

	return storeFloatResult(e, d, result)
}

// emulFMul is MULF{2,3}/MULD{2,3}: prod <- op1 * op0 (commutative), written
// to the last operand.
func emulFMul(e *Engine, d *Decoded) error {
	op0, op1, err := loadFloatPair(e, d)
	if err != nil {
		return err
	}

	result := op1 * op0
	setFloatPSL(e.cpu, result)

	return storeFloatResult(e, d, result)
}

// emulFDiv is DIVF{2,3}/DIVD{2,3}: quo <- op1 / op0. op0 is always the
// divisor, matching SUB/DIV's "first operand is the modifier" convention.
// Division by zero is a reserved-operand-shaped case on real VAX hardware
// (a floating divide by zero faults through the same overflow path store
// would, since the quotient's exponent is unbounded); Go's float64 division
// by zero instead produces +/-Inf or NaN, which then simply fails to
// convert to a finite VAX F/D-floating value -- fpuStore's own exponent
// check (Inf/NaN's IEEE exponent field is all-ones, decoding to a VAX
// exponent far past 127) already faults on it as an overflow, with no
// special-casing needed here.
func emulFDiv(e *Engine, d *Decoded) error {
	divisor, dividend, err := loadFloatPair(e, d)
	if err != nil {
		return err
	}

	result := dividend / divisor
	setFloatPSL(e.cpu, result)
	
	return storeFloatResult(e, d, result)
}
