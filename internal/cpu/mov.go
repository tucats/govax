package cpu

// This is the Go port of emul_mov.c's integer paths. MOVF/MOVD/MNEGF/MNEGD
// are Phase 05's (floating F/D conversion); their opcode table slots stay on
// unimplementedHandler until then.

func init() {
	reg := func(fn byte, h Handler) {
		instructionTable.SetHandler(instructionTable.Lookup(Opcode{Function: fn}), h)
	}
	reg(0x9A, emulMove) // MOVZBL
	reg(0x9B, emulMove) // MOVZBW
	reg(0x3C, emulMove) // MOVZWL
	reg(0x90, emulMove) // MOVB
	reg(0x92, emulMcom) // MCOMB
	reg(0x8E, emulMneg) // MNEGB
	reg(0xB0, emulMove) // MOVW
	reg(0xB2, emulMcom) // MCOMW
	reg(0xAE, emulMneg) // MNEGW
	reg(0xD0, emulMove) // MOVL
	reg(0xD2, emulMcom) // MCOML
	reg(0xCE, emulMneg) // MNEGL
	reg(0x7D, emulMove) // MOVQ
}

// emulMove is shared by MOV{B,W,L,Q} and MOVZ{BW,BL,WL}: the destination
// operand is replaced by the source (zero-extended when the destination is
// wider than the source, since Operand.Load already zero-extends). N/Z come
// from the result at the destination's size, V <- 0, C is left unaffected --
// matching the manual's MOV and MOVZ entries exactly (both explicitly leave
// C alone; MOVZ's N is defined as a constant 0, which checking the sign bit
// of an always-zero-extended value reproduces without a special case).
//
// The C source's emul_movb/movw/movl/emul_movz* agree with this. emul_movq
// instead uses SETCONDITIONBITS(data, 0L), which incidentally always clears
// C and never sets V (the macro's C/V formulas are only meaningful for a
// real two-operand compare) -- fixed here to the same N/Z/V=0/C-unaffected
// shape as every other MOV size. See docs/DEVIATIONS.md.
//
// Operand.Store already writes a size-8 register destination as the Rn/Rn+1
// pair, so MOVQ needs no extra handling for a register destination the way
// the C source's emul_movq does.
func emulMove(e *Engine, d *Decoded) error {
	v, err := d.Operands[0].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}

	setNZ(e.cpu, v, d.Operands[1].Size)
	psl := e.cpu.PSL()
	psl.SetV(false)
	e.cpu.SetPSL(psl)

	return d.Operands[1].Store(e.cpu, e.mem, v)
}

// emulMcom is MCOM{B,W,L}: destination <- one's complement of source. N/Z
// from the result, V <- 0, C unaffected -- matching the manual (and the C
// source's shared emul_*_negated handlers, whose MCOM branch never touches
// C, only the MNEG branch does).
func emulMcom(e *Engine, d *Decoded) error {
	size := d.Operands[0].Size

	v, err := d.Operands[0].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}

	result := maskToSize(^signExtend(v, size), size)
	setNZ(e.cpu, result, size)
	psl := e.cpu.PSL()
	psl.SetV(false)
	e.cpu.SetPSL(psl)

	return d.Operands[1].Store(e.cpu, e.mem, result)
}

// emulMneg is MNEG{B,W,L}: destination <- -source. N/Z from the result;
// V <- 1 iff source is the size's largest negative integer (which has no
// positive counterpart -- the destination is then left as the unchanged
// source, per the manual's note), else V <- 0; C <- source NEQ 0 (per the
// manual's "C <- dst NEQ 0", which is equivalent since negation only maps
// zero to zero).
//
// The C source gets both of these wrong:
//   - It computes C as "source LSS 0" instead of "source NEQ 0" -- true only
//     when source is already negative, but the manual's C is 1 for *any*
//     nonzero source (0 minus a nonzero value always borrows, regardless of
//     its sign).
//   - emul_movb_negated has a stray second `vax.pslw.v = 0` right after
//     setting V for the overflow case, clobbering it; MNEGB can never
//     actually report overflow in the C source. emul_movw_negated/
//     emul_movl_negated don't have this extra line.
//
// Both are fixed here uniformly across B/W/L. See docs/DEVIATIONS.md.
func emulMneg(e *Engine, d *Decoded) error {
	size := d.Operands[0].Size

	v, err := d.Operands[0].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}

	source := signExtend(v, size)

	psl := e.cpu.PSL()
	psl.SetC(source != 0)

	var result int64

	if source == minSigned(size) {
		psl.SetV(true)

		result = source
	} else {
		psl.SetV(false)
		
		result = -source
	}
	
	masked := maskToSize(result, size)
	psl.SetN(signBit(masked, size))
	psl.SetZ(isZero(masked, size))
	e.cpu.SetPSL(psl)

	return d.Operands[1].Store(e.cpu, e.mem, masked)
}
