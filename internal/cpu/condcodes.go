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
