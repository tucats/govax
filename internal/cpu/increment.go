package cpu

// This is the Go port of emul_increment.c, using the ADD/SUB result helpers
// (internal/cpu/condcodes.go) rather than the C source's per-size formulas
// -- the manual defines INCx/DECx as exactly equivalent to ADDx S^#1/SUBx
// S^#1, so sharing the formula keeps the two families from disagreeing on
// condition codes for the identical operation. See docs/DEVIATIONS.md.

func init() {
	reg := func(fn byte, h Handler) {
		instructionTable.SetHandler(instructionTable.Lookup(Opcode{Function: fn}), h)
	}
	reg(0x96, emulInc) // INCB
	reg(0x97, emulDec) // DECB
	reg(0xB6, emulInc) // INCW
	reg(0xB7, emulDec) // DECW
	reg(0xD6, emulInc) // INCL
	reg(0xD7, emulDec) // DECL
}

// emulInc is INC{B,W,L}: the operand is replaced by itself plus one.
func emulInc(e *Engine, d *Decoded) error {
	size := d.Operands[0].Size
	v, err := d.Operands[0].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}
	result, ov, c := addResult(v, 1, size)
	setArithPSL(e.cpu, result, ov, c, size)
	return d.Operands[0].Store(e.cpu, e.mem, result)
}

// emulDec is DEC{B,W,L}: the operand is replaced by itself minus one.
func emulDec(e *Engine, d *Decoded) error {
	size := d.Operands[0].Size
	v, err := d.Operands[0].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}
	result, ov, c := subResult(v, 1, size)
	setArithPSL(e.cpu, result, ov, c, size)
	return d.Operands[0].Store(e.cpu, e.mem, result)
}
