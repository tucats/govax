package librtl

import (
	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/vmsdef"
)

// The integer routines (docs/PHASE-53, subtask 7): LIB$ADDX, LIB$SUBX,
// LIB$EMUL, LIB$EDIV, and LIB$ADAWI make the VAX's extended-precision and
// interlocked integer instructions callable. The arithmetic is the CPU's
// own (cpu.AddWithCarry, cpu.SubtractWithBorrow, cpu.ExtendedMultiply,
// cpu.ExtendedDivide, cpu.AddWord), so the routines and the instructions
// follow one set of rules. Their statuses are returned, not signaled: an
// overflow is SS$_INTOVF (LIB$ADAWI's is LIB$_INTOVF), as the manual
// lists them.

var (
	ssIntOvf  = vmsdef.Symbols["SS$_INTOVF"]
	ssIntDiv  = vmsdef.Symbols["SS$_INTDIV"]
	libIntOvf = vmsdef.Symbols["LIB$_INTOVF"]
	libNormal = vmsdef.Symbols["LIB$_NORMAL"]
)

// defaultArrayLength is LIB$ADDX's and LIB$SUBX's array length when the
// call omits it: two longwords, a quadword.
const defaultArrayLength = 2

// libAddx is LIB$ADDX (LIB-7): resultant-array <- addend-array +
// augend-array, multiprecision signed integers of array-length longwords
// (2 if omitted), least significant first.
//
//	LIB$ADDX addend-array ,augend-array ,resultant-array [,array-length]
func libAddx(env *corevms.Environment, argv []uint32) (uint32, error) {
	return multiprecision(env, argv, cpu.AddWithCarry)
}

// libSubx is LIB$SUBX (LIB-399): difference-array <- minuend-array -
// subtrahend-array, as LIB$ADDX.
//
//	LIB$SUBX minuend-array ,subtrahend-array ,difference-array [,array-length]
func libSubx(env *corevms.Environment, argv []uint32) (uint32, error) {
	return multiprecision(env, argv, cpu.SubtractWithBorrow)
}

// multiprecision is LIB$ADDX and LIB$SUBX: op (ADWC's or SBWC's
// arithmetic) runs over the arrays a longword at a time from the least
// significant, each step's carry or borrow going into the next, as a VAX
// routine's ADWC or SBWC loop does. Reading and writing in step lets the
// result be either input array. The status is SS$_INTOVF when the most
// significant longword overflowed (the result is right but for its sign
// bit), else SS$_NORMAL.
//
// A negative length is LIB$_INVARG, nothing written (the manual lists it
// for LIB$SUBX; for LIB$ADDX too, unconfirmed). A length of 0 does
// nothing, SS$_NORMAL (unconfirmed).
func multiprecision(env *corevms.Environment, argv []uint32, op func(a, b uint32, carry bool) (uint32, bool, bool)) (uint32, error) {
	mem, c := env.Memory(), env.CPU()
	length := int32(defaultArrayLength)

	if lenAddr := arg(argv, 3); lenAddr != 0 {
		n, err := mem.LoadLongword(c, lenAddr)
		if err != nil {
			return memoryFault(env, err)
		}

		length = int32(n)
	}

	if length < 0 {
		return libInvArg, nil
	}

	a, b, r := arg(argv, 0), arg(argv, 1), arg(argv, 2)
	carry, overflow := false, false

	for i := range uint32(length) {
		x, err := mem.LoadLongword(c, a+4*i)
		if err != nil {
			return memoryFault(env, err)
		}

		y, err := mem.LoadLongword(c, b+4*i)
		if err != nil {
			return memoryFault(env, err)
		}

		var result uint32

		result, overflow, carry = op(x, y, carry)

		if err := mem.StoreLongword(c, r+4*i, result); err != nil {
			return memoryFault(env, writing(err))
		}
	}

	if overflow {
		return ssIntOvf, nil
	}

	return ssNormal, nil
}

// libEmul is LIB$EMUL (LIB-136): product <- multiplier * multiplicand +
// addend, longwords multiplied into a quadword, the addend sign-extended.
//
//	LIB$EMUL longword-integer-multiplier ,longword-integer-multiplicand ,addend ,product
func libEmul(env *corevms.Environment, argv []uint32) (uint32, error) {
	mem, c := env.Memory(), env.CPU()

	var operands [3]int32

	for i := range operands {
		v, err := mem.LoadLongword(c, arg(argv, i))
		if err != nil {
			return memoryFault(env, err)
		}

		operands[i] = int32(v)
	}

	product := cpu.ExtendedMultiply(operands[0], operands[1], operands[2])

	if err := mem.StoreQuadword(c, arg(argv, 3), uint64(product)); err != nil {
		return memoryFault(env, writing(err))
	}

	return ssNormal, nil
}

// libEdiv is LIB$EDIV (LIB-126): the quadword dividend divided by the
// longword divisor, giving a longword quotient and remainder. A divisor
// of 0 is SS$_INTDIV and a quotient too big for a longword SS$_INTOVF;
// either way the quotient stored is the dividend's low longword and the
// remainder 0, as the instruction leaves them.
//
//	LIB$EDIV longword-integer-divisor ,quadword-integer-dividend ,longword-integer-quotient ,remainder
func libEdiv(env *corevms.Environment, argv []uint32) (uint32, error) {
	mem, c := env.Memory(), env.CPU()

	divisor, err := mem.LoadLongword(c, arg(argv, 0))
	if err != nil {
		return memoryFault(env, err)
	}

	dividend, err := mem.LoadQuadword(c, arg(argv, 1))
	if err != nil {
		return memoryFault(env, err)
	}

	quotient, remainder, ok := cpu.ExtendedDivide(int32(divisor), int64(dividend))

	if err := mem.StoreLongword(c, arg(argv, 2), uint32(quotient)); err != nil {
		return memoryFault(env, writing(err))
	}

	if err := mem.StoreLongword(c, arg(argv, 3), uint32(remainder)); err != nil {
		return memoryFault(env, writing(err))
	}

	switch {
	case ok:
		return ssNormal, nil
	case divisor == 0:
		return ssIntDiv, nil
	default:
		return ssIntOvf, nil
	}
}

// libAdawi is LIB$ADAWI (LIB-3): sum <- sum + add, words, as the ADAWI
// instruction adds them; the third argument gets the new sum's sign as a
// word (-1, 0, or 1; the manual calls it "result" and doesn't define its
// value further; VMS stores the same, testdata/lib53). The status is
// LIB$_NORMAL (VMS returns it, not SS$_NORMAL), or LIB$_INTOVF on
// overflow, the truncated sum stored.
//
// The sum must be word aligned, as the instruction's sum operand must
// be: an odd address is the instruction's reserved operand fault,
// signaled as SS$_ROPRAND (as VMS does). One CPU has no interlock to
// model.
//
//	LIB$ADAWI add ,sum ,result
func libAdawi(env *corevms.Environment, argv []uint32) (uint32, error) {
	mem, c := env.Memory(), env.CPU()
	sumAddr := arg(argv, 1)

	add, err := mem.LoadWord(c, arg(argv, 0))
	if err != nil {
		return memoryFault(env, err)
	}

	if sumAddr&1 != 0 {
		return reservedOperand(env)
	}

	sum, err := mem.LoadWord(c, sumAddr)
	if err != nil {
		return memoryFault(env, err)
	}

	result, overflow, _ := cpu.AddWord(add, sum)

	if err := mem.StoreWord(c, sumAddr, result); err != nil {
		return memoryFault(env, writing(err))
	}

	var sign uint16

	switch {
	case int16(result) < 0:
		sign = 0xFFFF // -1
	case result > 0:
		sign = 1
	}

	if err := mem.StoreWord(c, arg(argv, 2), sign); err != nil {
		return memoryFault(env, writing(err))
	}

	if overflow {
		return libIntOvf, nil
	}

	return libNormal, nil
}
