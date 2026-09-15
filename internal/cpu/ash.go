package cpu

import "math/bits"

// This is the Go port of emul_ash.c: ROTL, ASHL, ASHQ. Both use the
// SETCONDITIONBITS(x, 0L) idiom in the C source, which incidentally always
// force-clears C and never computes V at all; see docs/DEVIATIONS.md (the
// finding logged in sub-phase 5/mov.go). Fixed here: C is left untouched
// (never written) for all three, and V uses the manual's real formulas --
// always 0 for ROTL, and shiftOverflow32/64 for a left-shifting ASHL/ASHQ.

func init() {
	reg := func(fn byte, h Handler) {
		instructionTable.SetHandler(instructionTable.Lookup(Opcode{Function: fn}), h)
	}
	reg(0x9C, emulRotl) // ROTL
	reg(0x78, emulAshl) // ASHL
	reg(0x79, emulAshq) // ASHQ
}

// emulRotl is ROTL: the source longword is rotated by the count operand's
// value (positive left, negative right -- exactly math/bits.RotateLeft32's
// own sign convention, so the C source's manual bit-by-bit loop doesn't
// need porting) and the destination is replaced by the result. N/Z from the
// result, V <- 0, C unaffected.
func emulRotl(e *Engine, d *Decoded) error {
	countRaw, err := d.Operands[0].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}

	value, err := d.Operands[1].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}

	result := uint64(bits.RotateLeft32(uint32(value), int(int8(countRaw))))
	setNZ(e.cpu, result, 4)
	psl := e.cpu.PSL()
	psl.SetV(false)
	e.cpu.SetPSL(psl)

	return d.Operands[2].Store(e.cpu, e.mem, result)
}

// emulAshl is ASHL: the source longword is arithmetically shifted by the
// count operand's value (positive left, negative right) and the
// destination is replaced by the result. N/Z from the result, V per
// shiftOverflow32 (left shifts only -- a right shift can't overflow a
// signed representation), C unaffected.
func emulAshl(e *Engine, d *Decoded) error {
	countRaw, err := d.Operands[0].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}

	source, err := d.Operands[1].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}

	count := int8(countRaw)
	s := int32(uint32(source))

	var (
		result int32
		v      bool
	)

	if count >= 0 {
		shift := uint(count)
		result = s << shift
		v = count > 0 && shiftOverflow32(s, shift)
	} else {
		result = s >> uint(-int(count))
	}

	r := uint64(uint32(result))
	setNZ(e.cpu, r, 4)
	psl := e.cpu.PSL()
	psl.SetV(v)
	e.cpu.SetPSL(psl)

	return d.Operands[2].Store(e.cpu, e.mem, r)
}

// emulAshq is ASHQ: ASHL's quadword counterpart.
func emulAshq(e *Engine, d *Decoded) error {
	countRaw, err := d.Operands[0].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}

	source, err := d.Operands[1].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}

	count := int8(countRaw)
	s := int64(source)

	var (
		result int64
		v      bool
	)

	if count >= 0 {
		shift := uint(count)
		result = s << shift
		v = count > 0 && shiftOverflow64(s, shift)
	} else {
		result = s >> uint(-int(count))
	}

	r := uint64(result)
	setNZ(e.cpu, r, 8)
	psl := e.cpu.PSL()
	psl.SetV(v)
	e.cpu.SetPSL(psl)
	
	return d.Operands[2].Store(e.cpu, e.mem, r)
}
