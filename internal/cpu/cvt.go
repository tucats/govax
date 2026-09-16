package cpu

// This is the Go port of emul_integer_cvt.c's integer-to-integer
// conversions. CVTxF/CVTFx and friends (integer <-> floating) are Phase 05's.

func init() {
	reg := func(fn byte, h Handler) {
		instructionTable.SetHandler(instructionTable.Lookup(Opcode{Function: fn}), h)
	}
	reg(0x98, emulCvt) // CVTBL
	reg(0x99, emulCvt) // CVTBW
	reg(0x32, emulCvt) // CVTWL
	reg(0x33, emulCvt) // CVTWB
	reg(0xF6, emulCvt) // CVTLB
	reg(0xF7, emulCvt) // CVTLW
}

// emulCvt is CVT{B,W,L}{B,W,L} (every same-size-excluded pair): the
// destination is replaced by the source, sign-extended or truncated to the
// destination's size. N/Z come from the destination (post-truncation)
// value, V from convertResult's truncation check, C is always 0 -- matching
// the manual's CVT entry exactly.
//
// emul_integer_cvt.c computes N/Z from the *source* value (sign-extended,
// but not yet truncated to the destination size) instead -- these only
// disagree when the conversion overflows, but when it does, they can
// disagree outright: converting long 0x00000080 (128, positive) to byte
// truncates to 0x80 (-128, negative) -- the manual's dst-based N is true,
// but the C source's source-based N is false. Its byte-destination V check
// also uses the same wrong constants (255/-256 instead of 127/-128) already
// found and fixed in emul_integer_math.c/emul_increment.c; see
// docs/DEVIATIONS.md.
func emulCvt(e *Engine, d *Decoded) error {
	srcSize := d.Operands[0].Size
	dstSize := d.Operands[1].Size

	v, err := d.Operands[0].Load(e.cpu, e.mem)
	if err != nil {
		return err
	}

	result, ov := convertResult(v, srcSize, dstSize)
	setArithPSL(e.cpu, result, ov, false, dstSize)
	
	return d.Operands[1].Store(e.cpu, e.mem, result)
}
