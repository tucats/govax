package cpu

// This is the Go port of emul_cmp.c's integer paths (CMPB/W/L, BITB/W/L,
// TSTB/W/L). CMPF/TSTF are Phase 05's.
//
// The C source attaches a separate, size-specialized emul_cmpl to CMPL's
// table slot (init_emulators.c) purely as a performance optimization --
// same behavior as the generic emul_cmp's longword case, just without the
// runtime size dispatch. Not ported as a separate handler: this is the same
// category of 1999-era speed hack Phase 03 already declined to carry over
// (see its notes on the extended-opcode lookup), and emulCmp below is
// already generic over size with nothing further to specialize in Go.

func init() {
	reg := func(fn byte, h Handler) {
		instructionTable.SetHandler(instructionTable.Lookup(Opcode{Function: fn}), h)
	}
	reg(0x91, emulCmp) // CMPB
	reg(0xB1, emulCmp) // CMPW
	reg(0xD1, emulCmp) // CMPL

	reg(0x93, emulBit) // BITB
	reg(0xB3, emulBit) // BITW
	reg(0xD3, emulBit) // BITL

	reg(0x95, emulTst) // TSTB
	reg(0xB5, emulTst) // TSTW
	reg(0xD5, emulTst) // TSTL
}

// emulCmp is CMP{B,W,L}: src1 is compared with src2; neither operand is
// modified. N <- src1 LSS src2, Z <- src1 EQL src2, V <- 0, C <- src1 LSSU
// src2 (unsigned) -- exactly subResult's V/C shape (src1 - src2's borrow and
// overflow), reused here even though CMP never writes a result.
func emulCmp(e *Engine, d *Decoded) error {
	size := d.Operands[0].Size

	src1, err := d.Operands[0].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}

	src2, err := d.Operands[1].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}

	result, _, c := subResult(src1, src2, size)
	psl := e.cpu.PSL()
	psl.SetN(signBit(result, size))
	psl.SetZ(isZero(result, size))
	psl.SetV(false)
	psl.SetC(c)
	e.cpu.SetPSL(psl)

	return nil
}

// emulBit is BIT{B,W,L}: the logical AND of the mask and source operands is
// computed; neither operand is modified. N/Z from the AND result, V <- 0,
// C unaffected -- per the manual. emul_cmp.c's shared handler force-clears
// C for BIT (it only ever sets a nonzero cbit in the CMP case); see
// docs/DEVIATIONS.md.
func emulBit(e *Engine, d *Decoded) error {
	size := d.Operands[0].Size

	mask, err := d.Operands[0].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}

	src, err := d.Operands[1].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}

	result := maskToSize(int64(mask&src), size)
	setLogicalPSL(e.cpu, result, size)
	
	return nil
}

// emulTst is TST{B,W,L}: the condition codes are set from the source
// operand's value, which is otherwise unmodified. N/Z from the value, V and
// C both 0 -- per the manual (matching emul_cmp.c, which gets TST right).
func emulTst(e *Engine, d *Decoded) error {
	size := d.Operands[0].Size

	v, err := d.Operands[0].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}

	setArithPSL(e.cpu, v, false, false, size)

	return nil
}
