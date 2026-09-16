package cpu

import "github.com/tucats/govax/internal/vax"

// This is the Go port of emul_mov.c's emul_movf/emul_movd (MOVF/MNEGF and
// MOVD/MNEGD, despite the "emul_movf"/"emul_movd" function names not living
// in a separate emul_movf.c/emul_movd.c -- see docs/PHASE-05.md's scope
// mapping). The C source shares one function each for its MOV/MNEG pair,
// branching on opcode->function at runtime; this port instead gives each a
// separate handler, matching every other instruction family in this project
// (decode already carries which opcode is executing).

func init() {
	reg := func(fn byte, h Handler) {
		instructionTable.SetHandler(instructionTable.Lookup(Opcode{Function: fn}), h)
	}
	reg(0x50, emulMoveFloat)   // MOVF
	reg(0x70, emulMoveFloat)   // MOVD
	reg(0x52, emulNegateFloat) // MNEGF
	reg(0x72, emulNegateFloat) // MNEGD
}

// setFloatMovePSL sets N/Z from result and V <- 0 (a valid F/D-floating
// value, moved or sign-negated, can never overflow the format -- negation
// is just a sign-bit flip, always exactly representable, unlike two's-
// complement integer negation's most-negative-value edge case). C is left
// unaffected, matching the manual's MOVF/MNEGF (and D-floating
// counterparts) entries -- confirmed correct, not a bug, despite looking
// like the same "forgot to clear C" omission floatmath.go's ADD/SUB/MUL/DIV
// finding turned out to be: floating MNEG has no analogue of integer MNEG's
// real carry-out computation (see condcodes.go/docs/DEVIATIONS.md), so
// "unaffected" is the right definition here, and the C source's never
// touching vax.pslw.c happens to already match it.
func setFloatMovePSL(cpu *vax.CPU, result float64) {
	psl := cpu.PSL()
	psl.SetN(result < 0.0)
	psl.SetZ(result == 0.0)
	psl.SetV(false)
	cpu.SetPSL(psl)
}

// emulMoveFloat is MOVF/MOVD: dst <- src.
func emulMoveFloat(e *Engine, d *Decoded) error {
	value, err := loadFloat(e.cpu, e.mem, d.Operands[0])
	if err != nil {
		return err
	}
	
	setFloatMovePSL(e.cpu, value)

	return storeFloat(e.cpu, e.mem, d.Operands[1], value)
}

// emulNegateFloat is MNEGF/MNEGD: dst <- -src.
func emulNegateFloat(e *Engine, d *Decoded) error {
	value, err := loadFloat(e.cpu, e.mem, d.Operands[0])
	if err != nil {
		return err
	}

	result := -value
	setFloatMovePSL(e.cpu, result)

	return storeFloat(e.cpu, e.mem, d.Operands[1], result)
}
