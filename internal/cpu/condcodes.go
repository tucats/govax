package cpu

import "github.com/tucats/govax/internal/vax"

// signExtend sign-extends the low size bytes (1, 2, 4, or 8) of v -- a
// zero-extended value as returned by Operand.Load -- to a full int64.
func signExtend(v uint64, size int) int64 {
	switch size {
	case 1:
		return int64(int8(v))
	case 2:
		return int64(int16(v))
	case 4:
		return int64(int32(v))
	case 8:
		return int64(v)
	default:
		panic("cpu: unsupported operand size")
	}
}

// maskToSize truncates v to its low size bytes (1, 2, 4, or 8), returned
// zero-extended -- the value form Operand.Store expects.
func maskToSize(v int64, size int) uint64 {
	if size >= 8 {
		return uint64(v)
	}
	return uint64(v) & (1<<(uint(size)*8) - 1)
}

// signBit reports whether the low size bytes of v, interpreted as signed,
// are negative.
func signBit(v uint64, size int) bool {
	return signExtend(v, size) < 0
}

// isZero reports whether the low size bytes of v are all zero.
func isZero(v uint64, size int) bool {
	return maskToSize(int64(v), size) == 0
}

// minSigned returns the most negative representable value at the given size
// (1, 2, 4, or 8 bytes), e.g. -128 for size 1 -- the "largest negative
// integer" the VAX manual's MNEG/negation-overflow notes refer to.
func minSigned(size int) int64 {
	return -(int64(1) << (uint(size)*8 - 1))
}

// setNZ sets N and Z from value at the given size -- the "N <- value LSS 0;
// Z <- value EQL 0" pattern most instructions use for their result.  It does
// not touch V or C; callers set those per instruction.
func setNZ(cpu *vax.CPU, value uint64, size int) {
	psl := cpu.PSL()
	psl.SetN(signBit(value, size))
	psl.SetZ(isZero(value, size))
	cpu.SetPSL(psl)
}

// setArithPSL sets all four condition codes from an arithmetic result and
// its already-computed V/C -- the common "N/Z from the result, V and C as
// the instruction defines them" shape shared by INC/DEC/ADD/SUB/ADWC/SBWC.
func setArithPSL(cpu *vax.CPU, result uint64, v, c bool, size int) {
	psl := cpu.PSL()
	psl.SetN(signBit(result, size))
	psl.SetZ(isZero(result, size))
	psl.SetV(v)
	psl.SetC(c)
	cpu.SetPSL(psl)
}

// sizeMax returns the largest unsigned value representable at the given
// size (1, 2, 4, or 8 bytes).
func sizeMax(size int) uint64 {
	if size >= 8 {
		return ^uint64(0)
	}
	return 1<<(uint(size)*8) - 1
}

// The Add/Sub/Mul/Div result helpers below compute the VAX manual's exact
// N/Z/V/C definitions for these instruction families using Go's native
// 64-bit arithmetic to genuinely widen past the operand size -- the same
// technique the C source's longword path was reaching for (compute both a
// signed and unsigned version of the result, then compare) but couldn't
// achieve once LONGWORD is genuinely 32-bit rather than a wider native
// type, and which the C source's byte/word paths use a similarly narrow
// (and, for byte, differently and additionally wrong) approximation of
// instead. See docs/DEVIATIONS.md for the full analysis; every caller in
// this phase (ADD/SUB/INC/DEC/ADWC/SBWC/MUL/DIV) uses these rather than
// porting the C source's per-size formulas.

// addResult computes a+b at the given size, returning the masked result and
// V (signed overflow: same-sign operands, opposite-sign result) and C
// (unsigned carry out of the operand width) per the manual's ADD entry.
func addResult(a, b uint64, size int) (result uint64, v, c bool) {
	sum := signExtend(a, size) + signExtend(b, size)
	result = maskToSize(sum, size)
	v = signBit(a, size) == signBit(b, size) && signBit(result, size) != signBit(a, size)
	c = maskToSize(int64(a), size)+maskToSize(int64(b), size) > sizeMax(size)
	return result, v, c
}

// addCarryResult is addResult with an incoming carry added in as a third
// term (ADWC's "sum" in the manual is the three-way total), used for the
// same wide sum in both the result and the V/C computation.
func addCarryResult(a, b uint64, carryIn bool, size int) (result uint64, v, c bool) {
	sum := signExtend(a, size) + signExtend(b, size)
	usum := maskToSize(int64(a), size) + maskToSize(int64(b), size)
	if carryIn {
		sum++
		usum++
	}
	result = maskToSize(sum, size)
	v = signBit(a, size) == signBit(b, size) && signBit(result, size) != signBit(a, size)
	c = usum > sizeMax(size)
	return result, v, c
}

// subResult computes minuend-subtrahend at the given size, returning the
// masked result and V (signed overflow: operands differ in sign, result
// differs in sign from the minuend) and C (a borrow: unsigned minuend less
// than unsigned subtrahend) per the manual's SUB entry.
func subResult(minuend, subtrahend uint64, size int) (result uint64, v, c bool) {
	diff := signExtend(minuend, size) - signExtend(subtrahend, size)
	result = maskToSize(diff, size)
	v = signBit(minuend, size) != signBit(subtrahend, size) && signBit(result, size) != signBit(minuend, size)
	c = maskToSize(int64(minuend), size) < maskToSize(int64(subtrahend), size)
	return result, v, c
}

// subCarryResult is subResult with an incoming borrow subtracted in as a
// third term (SBWC), used for the same wide difference in both the result
// and the V/C computation.
func subCarryResult(minuend, subtrahend uint64, borrowIn bool, size int) (result uint64, v, c bool) {
	diff := signExtend(minuend, size) - signExtend(subtrahend, size)
	um := maskToSize(int64(minuend), size)
	need := maskToSize(int64(subtrahend), size)
	if borrowIn {
		diff--
		need++
	}
	result = maskToSize(diff, size)
	v = signBit(minuend, size) != signBit(subtrahend, size) && signBit(result, size) != signBit(minuend, size)
	c = um < need
	return result, v, c
}

// mulResult computes a*b at the given size, returning the masked result and
// V (the true product doesn't fit the destination size) per the manual's
// MUL entry; C is always 0 for MUL. Safe for sizes up to 4 bytes: two
// sign-extended 32-bit values' product always fits within int64's range.
func mulResult(a, b uint64, size int) (result uint64, v bool) {
	product := signExtend(a, size) * signExtend(b, size)
	result = maskToSize(product, size)
	return result, signExtend(result, size) != product
}

// divResult computes dividend/divisor (truncating toward zero, matching
// Go's integer division) at the given size, returning the masked result and
// V per the manual's DIV entry: integer overflow (MinInt/-1, which Go
// cannot even represent as a positive result) or division by zero (which
// would panic in Go the same way it's undefined behavior in C -- the reason
// this needs a guard at all). C is always 0 for DIV.
//
// When V is set for either reason, no division is actually performed -- the
// result is left as the unchanged dividend, the same "no defined result"
// convention this port's MNEG overflow case uses.
func divResult(dividend, divisor uint64, size int) (result uint64, v bool) {
	d, s := signExtend(dividend, size), signExtend(divisor, size)
	if s == 0 || (d == minSigned(size) && s == -1) {
		return maskToSize(d, size), true
	}
	return maskToSize(d/s, size), false
}
