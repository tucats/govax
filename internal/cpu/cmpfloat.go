package cpu

// This is the Go port of emul_cmp.c's float paths (CMPF/TSTF) plus CMPD/TSTD
// (unimplemented in the C reference -- no dispatch entry, and no `case 3`
// (D_FLOAT) in emul_cmp.c's own `switch(dsize)` either; see docs/PHASE-05.md's
// design notes). Not folded into cmp.go's emulCmp/emulTst: those are
// integer-only (loadFloat/storeFloat's short-literal handling and the
// F/D-floating comparison semantics below don't apply to them), and the C
// source itself keeps float and integer compare as branches of one shared
// function rather than one generic-over-everything routine -- separate
// handlers here match the project's established "decode already knows which
// opcode is executing" convention just as clearly.

func init() {
	reg := func(fn byte, h Handler) {
		instructionTable.SetHandler(instructionTable.Lookup(Opcode{Function: fn}), h)
	}
	reg(0x51, emulCmpFloat) // CMPF
	reg(0x71, emulCmpFloat) // CMPD
	reg(0x53, emulTstFloat) // TSTF
	reg(0x73, emulTstFloat) // TSTD
}

// emulCmpFloat is CMPF/CMPD: src1 is compared with src2; neither operand is
// modified. N <- src1 LSS src2, Z <- src1 EQL src2, V <- 0, C <- 0 -- unlike
// integer CMP, there is no unsigned interpretation of a floating value to
// give C a real definition, and emul_cmp.c's own cbit stays 0 for its float
// path (only ever set in the dsize>3, integer branch), confirming this
// isn't a C-source omission the way the BIT finding was.
func emulCmpFloat(e *Engine, d *Decoded) error {
	src1, err := loadFloat(e.cpu, e.mem, d.Operands[0])
	if err != nil {
		return err
	}
	src2, err := loadFloat(e.cpu, e.mem, d.Operands[1])
	if err != nil {
		return err
	}
	psl := e.cpu.PSL()
	psl.SetN(src1 < src2)
	psl.SetZ(src1 == src2)
	psl.SetV(false)
	psl.SetC(false)
	e.cpu.SetPSL(psl)
	return nil
}

// emulTstFloat is TSTF/TSTD: the condition codes are set from the source
// operand's value, which is otherwise unmodified. N/Z from the value, V and
// C both 0 -- same shape as emulTst's integer case (cmp.go).
func emulTstFloat(e *Engine, d *Decoded) error {
	value, err := loadFloat(e.cpu, e.mem, d.Operands[0])
	if err != nil {
		return err
	}
	psl := e.cpu.PSL()
	psl.SetN(value < 0.0)
	psl.SetZ(value == 0.0)
	psl.SetV(false)
	psl.SetC(false)
	e.cpu.SetPSL(psl)
	return nil
}
