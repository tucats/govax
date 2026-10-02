// Package vaxfloat is the VAX's floating point: the four formats (F, D, G,
// and H_floating), their memory layouts, and arithmetic rounded as the VAX
// rounds it.
//
// It has no CPU dependency. The CPU loads an operand's bytes, hands them
// here as Bits, and gets back a Value: an exact number, independent of any
// format. Arithmetic on Values is exact too until a result is rounded to a
// destination format, once, half away from zero (the VAX Architecture
// Reference Manual's rounding; IEEE arithmetic, and so Go's float64, rounds
// ties to even instead). Pack then lays the rounded value out in the
// format's bits.
//
// Values are held as math/big Floats (docs/PHASE-35.md, Decision 2): they
// carry any precision, which H_floating's 113-bit fraction needs, and
// round correctly in the VAX's mode.
//
// # The formats
//
// Each format is a sign bit, a biased exponent, and a fraction with a
// hidden leading 1 bit. The value is 0.1fff...f (binary) times 2 to the
// power (exponent - bias): the fraction is always at least one half.
//
//	format  size  exponent bits  bias   fraction bits  precision
//	F       4     8              128    23             24
//	D       8     8              128    55             56
//	G       8     11             1024   52             53
//	H       16    15             16384  112            113
//
// An exponent of zero means zero if the sign is clear, whatever the
// fraction holds, and a reserved operand if the sign is set (any
// instruction that reads one takes a reserved-operand fault). There are no
// infinities, NaNs, denormals, or negative zero.
//
// In memory the bits are stored as 16-bit words, the word holding the sign
// and exponent first (at the lowest address), each word's low byte first.
// So read as a little-endian integer, as the CPU reads any operand, the
// words are in the opposite order from the value's significance: F_floating
// 1.5 is the longword ^X000040C0, not ^X40C00000.
package vaxfloat

import "fmt"

// Format is one of the VAX floating formats.
type Format int

// The formats.
const (
	F Format = iota // F_floating: 4 bytes
	D               // D_floating: 8 bytes, F's exponent range with a longer fraction
	G               // G_floating: 8 bytes, a wider exponent than D
	H               // H_floating: 16 bytes
)

// layout is a format's field widths.
type layout struct {
	size  int // bytes
	ebits int // exponent bits
	bias  int
	fbits int // stored fraction bits (the hidden bit isn't stored)
}

var layouts = [...]layout{
	F: {4, 8, 128, 23},
	D: {8, 8, 128, 55},
	G: {8, 11, 1024, 52},
	H: {16, 15, 16384, 112},
}

func (f Format) layout() layout { return layouts[f] }

// String returns the format's letter.
func (f Format) String() string {
	if f < F || f > H {
		return fmt.Sprintf("Format(%d)", int(f))
	}

	return string("FDGH"[f])
}

// Size returns the format's size in bytes: 4, 8, 8, or 16.
func (f Format) Size() int { return f.layout().size }

// Precision returns the number of significant bits in the format's
// values, counting the hidden bit: 24, 56, 53, or 113.
func (f Format) Precision() uint { return uint(f.layout().fbits + 1) }

// bits returns the total width of the format, in bits.
func (f Format) bits() int { return f.layout().size * 8 }

// minExp and maxExp are the range of the unbiased exponent e of a value
// 0.1fff * 2^e the format can hold.
func (f Format) minExp() int { return 1 - f.layout().bias }
func (f Format) maxExp() int { return (1 << f.layout().ebits) - 1 - f.layout().bias }

// Bits is a floating operand's bits as the CPU reads them: the operand's
// bytes as a little-endian integer, Lo the first 8 bytes and Hi the next 8
// (only H_floating uses Hi). F_floating uses Lo's low 32 bits.
type Bits struct {
	Lo, Hi uint64
}

// logical converts b, in format f's memory layout, to the value's bits in
// order of significance: sign, then exponent, then fraction, as a 128-bit
// number (hi, lo) whose low f.bits() bits are used.
func (f Format) logical(b Bits) (hi, lo uint64) {
	words := f.Size() / 2

	for i := 0; i < words; i++ {
		// Word i is at bit 16*i of the little-endian integer, and it's
		// the i-th most significant word of the value.
		w := word(b, i)
		shift := 16 * (words - 1 - i)
		whi, wlo := shl128(0, uint64(w), shift)
		hi, lo = or128(hi, lo, whi, wlo)
	}

	return hi, lo
}

// memory is logical's inverse.
func (f Format) memory(hi, lo uint64) Bits {
	words := f.Size() / 2

	var b Bits

	for i := 0; i < words; i++ {
		shift := 16 * (words - 1 - i)
		_, w := shr128(hi, lo, shift)
		setWord(&b, i, uint16(w))
	}

	return b
}

// word returns the 16-bit word at index i (counting from the lowest
// address) of b.
func word(b Bits, i int) uint16 {
	if i < 4 {
		return uint16(b.Lo >> (16 * i))
	}

	return uint16(b.Hi >> (16 * (i - 4)))
}

func setWord(b *Bits, i int, w uint16) {
	if i < 4 {
		b.Lo |= uint64(w) << (16 * i)
	} else {
		b.Hi |= uint64(w) << (16 * (i - 4))
	}
}

// 128-bit helpers, (hi, lo) pairs.

func shl128(hi, lo uint64, n int) (uint64, uint64) {
	switch {
	case n == 0:
		return hi, lo
	case n >= 128:
		return 0, 0
	case n >= 64:
		return lo << (n - 64), 0
	}

	return hi<<n | lo>>(64-n), lo << n
}

func shr128(hi, lo uint64, n int) (uint64, uint64) {
	switch {
	case n == 0:
		return hi, lo
	case n >= 128:
		return 0, 0
	case n >= 64:
		return 0, hi >> (n - 64)
	}

	return hi >> n, lo>>n | hi<<(64-n)
}

func or128(ahi, alo, bhi, blo uint64) (uint64, uint64) { return ahi | bhi, alo | blo }

// mask128 returns a mask of the low n bits.
func mask128(n int) (uint64, uint64) {
	switch {
	case n >= 128:
		return ^uint64(0), ^uint64(0)
	case n >= 64:
		return (uint64(1) << (n - 64)) - 1, ^uint64(0)
	}

	return 0, (uint64(1) << n) - 1
}
