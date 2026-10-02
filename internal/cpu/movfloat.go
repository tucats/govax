package cpu

// MOV and MNEG for the floating formats (originally the port of
// emul_mov.c's emul_movf/emul_movd). A move unpacks and repacks its
// operand, so a reserved operand faults, and a "dirty zero" (exponent 0,
// sign clear, fraction bits set) is stored as a true zero.

func init() {
	reg := func(fn byte, h Handler) {
		instructionTable.SetHandler(instructionTable.Lookup(Opcode{Function: fn}), h)
	}
	reg(0x50, emulMoveFloat)   // MOVF
	reg(0x70, emulMoveFloat)   // MOVD
	reg(0x52, emulNegateFloat) // MNEGF
	reg(0x72, emulNegateFloat) // MNEGD

	regFD(0x50, emulMoveFloat)   // MOVG
	regFD(0x52, emulNegateFloat) // MNEGG
}

// regFD registers handler h for the two-byte opcode FD fn: the G and H
// floating instructions and the other FD-prefixed ones.
func regFD(fn byte, h Handler) {
	instructionTable.SetHandler(instructionTable.Lookup(Opcode{Extended: 0xFD, Function: fn}), h)
}

// emulMoveFloat is MOVx: dst <- src. N and Z from the value, V cleared, C
// unaffected (the manual's MOV).
func emulMoveFloat(e *Engine, d *Decoded) error {
	value, err := e.loadFloat(d, 0)
	if err != nil {
		return err
	}

	setFloatCC(e.cpu, value, true)

	return e.storeFloat(d, 1, value)
}

// emulNegateFloat is MNEGx: dst <- -src. N and Z from the result, V and C
// cleared (the manual's MNEG; govax left C unaffected before Phase 35,
// see docs/DEVIATIONS.md). Negating a VAX floating value can't overflow:
// only the sign bit changes, and zero stays zero.
func emulNegateFloat(e *Engine, d *Decoded) error {
	value, err := e.loadFloat(d, 0)
	if err != nil {
		return err
	}

	result := value.Neg()
	setFloatCC(e.cpu, result, false)

	return e.storeFloat(d, 1, result)
}
