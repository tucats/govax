package mthrtl

import (
	"errors"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/librtl"
	"github.com/tucats/govax/internal/vaxfloat"
)

// Floating values, from where a routine's caller puts them and back. The
// library's calling conventions (the phase doc's survey):
//
//	MTH$ CALL, F or integer result         by reference    R0
//	MTH$ CALL, D or G, F complex result    by reference    R0:R1
//	MTH$ CALL, H, D or G complex result    by reference    through the first argument
//	MTH$ JSB                               in R0, R0:R1, or R0:R3, and the same for the result
//	OTS$ CALL                              by value        R0, R0:R1, or R0:R3
//
// A value in registers is in the same layout as in memory, the register
// numbers going up as the addresses do: R0 holds a D_floating value's
// first longword, the one with the sign and exponent, and R1 its second.
// So vaxfloat.Bits, a value's bytes as a little-endian integer, splits into
// registers by simple shifts.

// loadArg reads the format f argument at addr, for a routine that takes it
// by reference. An error, which fail signals, is vaxfloat.ErrReserved or
// the memory's.
func loadArg(env *corevms.Environment, f vaxfloat.Format, addr uint32) (vaxfloat.Value, error) {
	return librtl.LoadFloat(env, f, addr)
}

// storeResult writes v, already rounded to format f, through addr: the
// first argument of a routine with a result too long for R0:R1. An error
// is the memory's, marked as a write's, which fail signals.
func storeResult(env *corevms.Environment, f vaxfloat.Format, addr uint32, v vaxfloat.Value) error {
	return librtl.StoreFloat(env, f, addr, v)
}

// storeFailed writes what a routine whose result goes through addr leaves there
// when err, one of its function's errors, gives it no result: 0.0 for an
// underflow, and otherwise the reserved operand, whose longwords are ^X8000
// and then zeros. A reserved operand can't be stored as a Value, which is why
// this isn't storeResult.
func storeFailed(env *corevms.Environment, f vaxfloat.Format, addr uint32, err error) error {
	if errors.Is(err, errNoAngle) {
		return nil
	}

	first := uint32(reservedR0)
	if errors.Is(err, vaxfloat.ErrUnderflow) {
		first = 0
	}

	for i := range f.Size() / 4 {
		word := uint32(0)
		if i == 0 {
			word = first
		}

		if err := env.Memory().StoreLongword(env.CPU(), addr+uint32(4*i), word); err != nil {
			return librtl.Writing(err)
		}
	}

	return nil
}

// registerWords returns v, already rounded to format f, as the longwords
// it takes in registers: 1 for F, 2 for D and G, 4 for H.
func registerWords(f vaxfloat.Format, v vaxfloat.Value) []uint32 {
	// A value rounded to f packs without error.
	b, _ := vaxfloat.Pack(f, v)
	words := []uint32{uint32(b.Lo), uint32(b.Lo >> 32), uint32(b.Hi), uint32(b.Hi >> 32)}

	return words[:f.Size()/4]
}

// returnFloat puts v, already rounded to format f, in the registers of a
// CALL entry's result, R0, R0:R1, or R0:R3, and returns R0's part for the
// routine to return. A stub saves no registers (its entry mask is 0), so
// the RET that ends it leaves R1 to R3 as they're set here.
func returnFloat(env *corevms.Environment, f vaxfloat.Format, v vaxfloat.Value) uint32 {
	return returnWords(env, registerWords(f, v))
}

// loadRegisters reads a format f value from regs, starting at R(first).
// A reserved operand is vaxfloat.ErrReserved. It's how a JSB entry gets its
// argument (first is 0) and how an OTS$ routine reads its by-value arguments,
// from the argument list.
func loadRegisters(f vaxfloat.Format, regs []uint32, first int) (vaxfloat.Value, error) {
	var b vaxfloat.Bits

	for i := range f.Size() / 4 {
		w := uint64(regs[first+i])

		switch i {
		case 0:
			b.Lo |= w
		case 1:
			b.Lo |= w << 32
		case 2:
			b.Hi |= w
		case 3:
			b.Hi |= w << 32
		}
	}

	return vaxfloat.Unpack(f, b)
}

// storeRegisters puts v, already rounded to format f, in regs starting at
// R(first). MTH$HSINCOS_R7's second result starts at R4.
func storeRegisters(f vaxfloat.Format, regs *corevms.JSBRegisters, first int, v vaxfloat.Value) {
	copy(regs[first:], registerWords(f, v))
}
