package mthrtl

import (
	"math/big"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/vaxfloat"
)

// The routines with word and longword arguments or results. Every result comes
// back in R0, a word's in the low word, the rest of R0 being undefined; here it's
// the word's sign, as a CVTWL would make it.
//
// VMS's vector shows which of these can trap: its entry mask sets IV (integer
// overflow enable) on every routine whose footnote says integer overflow can
// occur, so such a routine traps whatever its caller's IV. (The mask sets it on
// MTH$SGN, MTH$IMOD, and MTH$JMOD too, which have no such footnote.) An overflow
// leaves a result's low-order bits in the destination, and so in R0 when a handler
// continues. MTH$IISIGN's mask leaves IV clear, and it has no footnote, so its
// overflow goes unnoticed, as an instruction's would.

// width is an integer type: a signed word, or a signed or unsigned longword.
type width struct {
	bits     uint
	unsigned bool
}

var (
	word         = width{bits: 16}
	long         = width{bits: 32}
	unsignedLong = width{bits: 32, unsigned: true}
)

// load reads an integer of this width at addr.
func (w width) load(env *corevms.Environment, addr uint32) (int64, error) {
	if w.bits == 16 {
		v, err := env.Memory().LoadWord(env.CPU(), addr)

		return int64(int16(v)), err
	}

	v, err := env.Memory().LoadLongword(env.CPU(), addr)
	if w.unsigned {
		return int64(v), err
	}

	return int64(int32(v)), err
}

// lowBits is i's low-order bits, as this width holds them and as a two's complement
// number, however large i is, in the form R0 returns them.
func (w width) lowBits(i *big.Int) uint32 {
	// big.Int's And treats a negative number as two's complement, with as
	// many sign bits as it takes.
	mask := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), w.bits), big.NewInt(1))
	low := uint32(new(big.Int).And(i, mask).Uint64())

	if w.bits == 16 {
		return uint32(int32(int16(low)))
	}

	return low
}

// holds reports whether this width can hold i.
func (w width) holds(i *big.Int) bool {
	if w.unsigned {
		return i.Sign() >= 0 && i.BitLen() <= int(w.bits)
	}

	limit := new(big.Int).Lsh(big.NewInt(1), w.bits-1)

	return i.Cmp(limit) < 0 && i.Cmp(new(big.Int).Neg(limit)) >= 0
}

// returnInteger is how a routine ends with the integer result i: R0 holds its
// low-order bits, and if it doesn't fit it's SS$_INTOVF, signaled with the same in
// R0 for a handler that continues.
func returnInteger(env *corevms.Environment, w width, i *big.Int) (uint32, error) {
	low := w.lowBits(i)

	if !w.holds(i) {
		return env.SignalFailure([]uint32{ssIntOvf}, low)
	}

	return low, nil
}

// integerArgs is the loader of integer arguments of width w, for a routine that
// treats them as numbers among its floating relatives.
func integerArgs(w width) loader {
	return func(env *corevms.Environment, addr uint32) (vaxfloat.Value, error) {
		v, err := w.load(env, addr)

		return vaxfloat.FromInt(v), err
	}
}

// integerFunction computes an integer routine's result from its arguments. The
// result may be out of the width's range, which is an integer overflow.
type integerFunction func(w width, args []int64) int64

// loadIntegers reads an integer routine's count arguments, by reference.
func loadIntegers(env *corevms.Environment, w width, argv []uint32, count int) ([]int64, error) {
	args := make([]int64, count)

	for i := range args {
		v, err := w.load(env, arg(argv, i))
		if err != nil {
			return nil, err
		}

		args[i] = v
	}

	return args, nil
}

// integerEntry makes a routine of count integer arguments and an integer result.
func integerEntry(w width, count int, fn integerFunction) corevms.ShimFunc {
	return func(env *corevms.Environment, argv []uint32) (uint32, error) {
		args, err := loadIntegers(env, w, argv, count)
		if err != nil {
			return fail(env, err)
		}

		return returnInteger(env, w, big.NewInt(fn(w, args)))
	}
}

// wholeEntry makes a routine with one argument in format in and an integer result
// of width w, whole saying how the value becomes a whole number: truncated or
// nearest, or signOf for MTH$SGN.
func wholeEntry(in vaxfloat.Format, w width, whole func(vaxfloat.Value) *big.Int) corevms.ShimFunc {
	return func(env *corevms.Environment, argv []uint32) (uint32, error) {
		x, err := loadArg(env, in, arg(argv, 0))
		if err != nil {
			return fail(env, err)
		}

		return returnInteger(env, w, whole(x))
	}
}

// signOf is MTH$SGN: -1, 0, or 1 by the value's sign. The manual's one entry
// takes F_floating or D_floating, whose first longwords have the sign and exponent
// in the same places, so reading it as F_floating answers for both. Its page says
// the result is "by reference", but there's no argument to write it through, so
// it's R0.
func signOf(x vaxfloat.Value) *big.Int { return big.NewInt(int64(x.Sign())) }

func integerAbsolute(_ width, args []int64) int64 {
	if args[0] < 0 {
		return -args[0]
	}

	return args[0]
}

func integerDifference(_ width, args []int64) int64 {
	if args[0] <= args[1] {
		return 0
	}

	return args[0] - args[1]
}

// integerSign is MTH$IISIGN and MTH$JISIGN: x's size with y's sign, y's 0 counting
// as positive. The largest negative number has no positive, and stays as it is.
func integerSign(w width, args []int64) int64 {
	x := integerAbsolute(w, args)
	if args[1] < 0 {
		x = -x
	}

	return int64(int32(w.lowBits(big.NewInt(x))))
}

// integerRemainderEntry makes MTH$IMOD and MTH$JMOD, whose result has the
// dividend's sign, as Go's % does. Only divide by zero is in their footnote, an
// integer exception every VAX takes, whose handler continuing gets the dividend
// in R0. A remainder of -1 from the largest negative number is 0: no footnote says
// otherwise, though VMS's mask sets IV. Both are unconfirmed (the probe, subtask
// 10).
func integerRemainderEntry(w width) corevms.ShimFunc {
	return func(env *corevms.Environment, argv []uint32) (uint32, error) {
		args, err := loadIntegers(env, w, argv, 2)
		if err != nil {
			return fail(env, err)
		}

		if args[1] == 0 {
			return env.SignalFailure([]uint32{ssIntDiv}, w.lowBits(big.NewInt(args[0])))
		}

		return w.lowBits(big.NewInt(args[0] % args[1])), nil
	}
}

// The bit functions. On numbers that have been sign-extended (load does it), AND,
// OR, exclusive OR, and complement give the same bits as on the word or longword,
// sign-extended too, so none of them is ever out of range.

func integerAnd(_ width, args []int64) int64         { return args[0] & args[1] }
func integerOr(_ width, args []int64) int64          { return args[0] | args[1] }
func integerExclusiveOr(_ width, args []int64) int64 { return args[0] ^ args[1] }
func integerNot(_ width, args []int64) int64         { return ^args[0] }

// integerShift is MTH$IISHFT and MTH$JISHFT, the FORTRAN manual's ISHFT: a positive
// count shifts left and a negative one right, both logical, zeros coming in. What's
// shifted out of the width is gone, so that's no overflow. A count of the width or
// more leaves nothing: lowBits drops what went left, and a Go shift by 64 or more is
// 0, not a surprise as it is in C.
func integerShift(w width, args []int64) int64 {
	bits := uint64(args[0]) & (1<<w.bits - 1)

	if count := args[1]; count >= 0 {
		bits <<= uint(count)
	} else {
		bits >>= uint(-count)
	}

	return int64(int32(w.lowBits(new(big.Int).SetUint64(bits))))
}
