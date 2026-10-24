package librtl

import (
	"errors"
	"math/big"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/vaxfloat"
)

// OTS$$CVT_D_T_R8, _F_, _G_, and _H_ (Phase 72, subtask 6): the JSB
// routines that VMS's FORRTL calls to turn a floating value into the
// decimal digits its F, E, D, and G edit descriptors print. No manual
// describes them; this is what testdata/ots72's study (ots72t.mar, the
// phase doc's Part A, "OTS$ in LIBRTL") saw VMS 7.1's do, watching
// FORRTL's FOR$CVT_x_Ty routines call them through the trace trap.
//
// The routine works in its caller's frame, at fixed offsets from the FP
// the caller passes in R1:
//
//	R0      the value's address
//	R1      the caller's FP
//	R2      the digit buffer's size in bytes
//	FP-9    byte: the edit descriptor's kind, 0 E (or D), 1 G, 2 F
//	FP-16   E: the significant digits to round to
//	FP-20   the digit buffer's address
//	FP-24   out: the sign, 1, -1, or 0 for zero
//	FP-28   out: the decimal exponent e, the value being 0.ddd... x 10^e
//	FP-32   out: the offset in the buffer of the first digit
//	FP-36   F: the digits after the point to round to
//
// The buffer gets a guard 0, then the value's exact decimal digits in
// ASCII, cut off where the buffer ends (a byte short of it). For E the
// digits are rounded, half up, at the significant digit FP-16 asks for,
// for F at the digit after the point FP-36 asks for; for G not at all
// (FORRTL rounds them itself). Rounding changes the digits before the
// rounding position and leaves the ones after it as they were, as VMS's
// routine does; a carry into the guard makes it the first digit, one
// place earlier, and the exponent one more. FORRTL reads no register
// back. A zero value is a buffer of zeros, the first digit at offset 0
// and the sign 0, the exponent left as it was.
//
// VMS's routine fills the buffer with fewer digits than govax's (whole
// groups of nine, an integer part's leading zeros among them: it works
// in longwords of 10^9), but its digits are the same exact ones, and
// FORRTL prints no digit past the ones it asked for, so the output is
// the same.
//
// Unconfirmed: a reserved operand (SS$_ROPRAND, signaled, as the 2.0
// Run-Time Library manual says FOR$CVT_x_Ty's are); a value whose digits
// before the rounding position don't fit the buffer (not rounded).

// The edit descriptors' kinds, at FP-9.
const (
	cvtKindE = 0
	cvtKindG = 1
	cvtKindF = 2
)

func otsCvtDTR8(env *corevms.Environment, regs *corevms.JSBRegisters) error {
	return editDigits(env, regs, vaxfloat.D)
}

func otsCvtFTR8(env *corevms.Environment, regs *corevms.JSBRegisters) error {
	return editDigits(env, regs, vaxfloat.F)
}

func otsCvtGTR8(env *corevms.Environment, regs *corevms.JSBRegisters) error {
	return editDigits(env, regs, vaxfloat.G)
}

func otsCvtHTR8(env *corevms.Environment, regs *corevms.JSBRegisters) error {
	return editDigits(env, regs, vaxfloat.H)
}

// editDigits is the routines' work, on a value of format f.
func editDigits(env *corevms.Environment, regs *corevms.JSBRegisters, f vaxfloat.Format) error {
	mem, cpu := env.Memory(), env.CPU()
	fp := regs[registers.R1]

	v, err := loadFloat(env, f, regs[registers.R0])
	if errors.Is(err, vaxfloat.ErrReserved) {
		_, err = env.SignalFailure([]uint32{ssRopRand}, ssRopRand)

		return err
	} else if err != nil {
		_, err = memoryFault(env, err)

		return err
	}

	words := map[uint32]uint32{20: 0, 16: 0, 36: 0}
	for off := range words {
		if words[off], err = mem.LoadLongword(cpu, fp-off); err != nil {
			_, err = memoryFault(env, err)

			return err
		}
	}

	kind, err := mem.LoadByte(cpu, fp-9)
	if err != nil {
		_, err = memoryFault(env, err)

		return err
	}

	size := max(int(int32(regs[registers.R2]))-1, 1)
	text, first, exp := digitBuffer(v.Rat(), size)

	switch kind {
	case cvtKindE:
		first, exp = roundDigits(text, first, exp, first+int(int32(words[16])))
	case cvtKindF:
		first, exp = roundDigits(text, first, exp, first+exp+int(int32(words[36])))
	}

	out := []struct {
		off   uint32
		value int
	}{{32, first}, {28, exp}, {24, v.Sign()}}

	if v.Sign() == 0 {
		out = []struct {
			off   uint32
			value int
		}{{32, 0}, {24, 0}}
	}

	for i, c := range text {
		if err := mem.StoreByte(cpu, words[20]+uint32(i), c); err != nil {
			_, err = memoryFault(env, writing(err))

			return err
		}
	}

	for _, o := range out {
		if err := mem.StoreLongword(cpu, fp-o.off, uint32(int32(o.value))); err != nil {
			_, err = memoryFault(env, writing(err))

			return err
		}
	}

	return nil
}

// digitBuffer is size bytes of ASCII digits: a guard 0, then v's exact
// decimal digits, its first one not 0, cut off (truncated) at the end,
// zeros after them when they end first; first is the offset of the first
// digit, and exp the decimal exponent (v is 0.ddd... x 10^exp). A zero v
// is all zeros, first 0 and exp 0.
func digitBuffer(v *big.Rat, size int) (text []byte, first, exp int) {
	text = make([]byte, size)
	for i := range text {
		text[i] = '0'
	}

	a := new(big.Rat).Abs(v)
	if a.Sign() == 0 || size < 2 {
		return text, 0, 0
	}

	exp = decimalExponent(a)
	n := size - 1

	scaled := new(big.Rat).Mul(a, pow10(n-exp))
	digits := new(big.Int).Quo(scaled.Num(), scaled.Denom()).String()

	copy(text[1:], leftPad(digits, n))

	return text, 1, exp
}

// roundDigits rounds text's digits half up at offset at: the digit there
// decides, the digits before it are incremented with their carry, and
// the digits from it on are left alone. A carry into the guard digit
// before first makes it the first digit and exp one more. It returns the
// first digit's offset and the exponent.
func roundDigits(text []byte, first, exp, at int) (int, int) {
	if at <= 0 || at >= len(text) || at < first || text[at] < '5' {
		return first, exp
	}

	for i := at - 1; i >= 0; i-- {
		if text[i] != '9' {
			text[i]++

			break
		}

		text[i] = '0'
	}

	if first > 0 && text[first-1] != '0' {
		first--
		exp++
	}

	return first, exp
}
