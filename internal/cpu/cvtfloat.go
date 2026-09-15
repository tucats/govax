package cpu

import "math"

// This is the Go port of emul_float_math.c's conversion paths (func 4/5):
// float->integer (CVTFB/CVTFW/CVTFL/CVTRFL and D-floating counterparts) and
// integer->float (CVTBF/CVTWF/CVTLF and D-floating counterparts).
//
// Two confirmed fixes along the way (docs/DEVIATIONS.md): the shared C
// handler's float->integer case always reads its source as a single 4-byte
// F_FLOAT longword regardless of dsize, which would read only half of an
// 8-byte D_FLOAT source for CVTDB/CVTDW/CVTDL/CVTRDL (dead code in the C
// reference, since those opcodes are never dispatched there -- see
// docs/PHASE-05.md's design notes -- but a real bug had they been); and
// CVTRFL/CVTRDL (func==4, dsize2==3) fall through the C switch's default
// case and always fault EXC_PRIV instead of rounding. Both fixed directly:
// loadFloat already dispatches on the source operand's own declared size,
// and emulCvtRoundFloatToInt implements real round-to-nearest.

func init() {
	reg := func(fn byte, h Handler) {
		instructionTable.SetHandler(instructionTable.Lookup(Opcode{Function: fn}), h)
	}
	for _, fn := range []byte{0x48, 0x49, 0x4A, 0x68, 0x69, 0x6A} { // CVTFB/W/L, CVTDB/W/L
		reg(fn, emulCvtFloatToInt)
	}

	for _, fn := range []byte{0x4B, 0x6B} { // CVTRFL, CVTRDL
		reg(fn, emulCvtRoundFloatToInt)
	}
	
	for _, fn := range []byte{0x4C, 0x4D, 0x4E, 0x6C, 0x6D, 0x6E} { // CVTBF/W/L, CVTBD/W/L/CVTLD
		reg(fn, emulCvtIntToFloat)
	}
}

// intOverflowBounds returns the signed range representable at size bytes
// (1, 2, or 4), per the VAX ISA manual's §8.3 "Data Types" -- the named
// byteMin/Max/wordMin/Max/longMin/Max constants in fpu.go, already fixed
// per reference/eVAX/AUDIT.md's N2 finding.
func intOverflowBounds(size int) (minValue, maxValue float64) {
	switch size {
	case 1:
		return byteMin, byteMax

	case 2:
		return wordMin, wordMax

	case 4:
		return longMin, longMax

	default:
		panic("cpu: unsupported integer conversion size")
	}
}

// cvtFloatToInt is emulCvtFloatToInt/emulCvtRoundFloatToInt's shared body:
// load the float source, optionally round, range-check against the
// destination's integer bounds (faulting -- always synchronous, matching
// fpu_store's own overflow handling, never just a set-and-continue V bit --
// on overflow, matching the manual and, for the non-rounding forms, the C
// source's own behavior), and store the truncated result. N/Z come from the
// destination value, V/C are always false on the surviving path (matches
// the C source's SETCONDITIONBITS(d1, 0L) idiom for N/Z; V/C are always 0
// here since overflow diverts to a fault rather than ever setting V).
func cvtFloatToInt(e *Engine, d *Decoded, round bool) error {
	src := d.Operands[0]
	dst := d.Operands[1]

	value, err := loadFloat(e.cpu, e.mem, src)
	if err != nil {
		return err
	}
	// The bounds check compares the pre-truncation value (matching the C
	// source's already-N2-fixed style -- compare the raw float against
	// exact integer min/max literals, not the post-truncation result) --
	// but the *rounded* value for CVTRFL/CVTRDL, since rounding can itself
	// push an in-range value out of range (e.g. word max 32767.6 rounds to
	// 32768, which overflows even though 32767.6 alone wouldn't have).
	if round {
		value = math.Round(value) // VAX round-to-nearest: ties away from zero, matching math.Round.
	}

	min, max := intOverflowBounds(dst.Size)
	if value < min || value > max {
		return &Fault{Code: ExcArithmetic, Args: []uint32{trapIntOvf}}
	}

	result := maskToSize(int64(value), dst.Size)
	setArithPSL(e.cpu, result, false, false, dst.Size)
	return dst.Store(e.cpu, e.mem, result)
}

// emulCvtFloatToInt is CVTFB/CVTFW/CVTFL/CVTDB/CVTDW/CVTDL: truncate toward
// zero.
func emulCvtFloatToInt(e *Engine, d *Decoded) error {
	return cvtFloatToInt(e, d, false)
}

// emulCvtRoundFloatToInt is CVTRFL/CVTRDL: round to nearest, halfway cases
// away from zero -- unimplemented in the C reference (see this file's
// package comment); implemented fresh from the manual.
func emulCvtRoundFloatToInt(e *Engine, d *Decoded) error {
	return cvtFloatToInt(e, d, true)
}

// emulCvtIntToFloat is CVTBF/CVTWF/CVTLF/CVTBD/CVTWD/CVTLD: the source
// integer, sign-extended, converted exactly to float64 (always exact --
// every representable byte/word/long value fits well within float64's
// 53-bit exact-integer range) and stored as F_floating/D_floating per the
// destination's own declared size. N/Z from the result, V/C always false
// (matches the C source's SETCONDITIONBITS(d1, 0L) plus its explicit
// vax.pslw.v = 0 -- confirmed not a bug: an exact int->float conversion in
// this range can never overflow or need a carry).
func emulCvtIntToFloat(e *Engine, d *Decoded) error {
	src := d.Operands[0]
	dst := d.Operands[1]

	raw, err := src.Load(e.cpu, e.mem)
	if err != nil {
		return err
	}
	value := float64(signExtend(raw, src.Size))

	setFloatPSL(e.cpu, value)
	return storeFloat(e.cpu, e.mem, dst, value)
}
