package cpu

// CMP and TST for the floating formats (originally the port of
// emul_cmp.c's float paths; CMPD and TSTD weren't in the C reference).

func init() {
	reg := func(fn byte, h Handler) {
		instructionTable.SetHandler(instructionTable.Lookup(Opcode{Function: fn}), h)
	}
	reg(0x51, emulCmpFloat) // CMPF
	reg(0x71, emulCmpFloat) // CMPD
	reg(0x53, emulTstFloat) // TSTF
	reg(0x73, emulTstFloat) // TSTD
}

// emulCmpFloat is CMPx: src1 is compared with src2, exactly. N <- src1 LSS
// src2, Z <- src1 EQL src2, V and C cleared: a floating value has no
// unsigned reading to give C a meaning.
func emulCmpFloat(e *Engine, d *Decoded) error {
	src1, err := e.loadFloat(d, 0)
	if err != nil {
		return err
	}

	src2, err := e.loadFloat(d, 1)
	if err != nil {
		return err
	}

	cmp := src1.Cmp(src2)

	psl := e.cpu.PSL()
	psl.SetN(cmp < 0)
	psl.SetZ(cmp == 0)
	psl.SetV(false)
	psl.SetC(false)
	e.cpu.SetPSL(psl)

	return nil
}

// emulTstFloat is TSTx: N and Z from the source, V and C cleared.
func emulTstFloat(e *Engine, d *Decoded) error {
	value, err := e.loadFloat(d, 0)
	if err != nil {
		return err
	}

	setFloatCC(e.cpu, value, false)

	return nil
}
