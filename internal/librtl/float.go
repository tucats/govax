package librtl

import (
	"errors"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/vaxfloat"
	"github.com/tucats/govax/internal/vmsdef"
)

// The floating routines (docs/PHASE-53, subtask 8): LIB$EMODx and
// LIB$POLYx, in each of the four floating formats (F, D, G, and H), make
// the VAX's EMOD and POLY instructions callable.
//
// Every argument is passed by reference. A floating operand is read from
// memory in its format's layout (4, 8, or 16 bytes) and unpacked by
// internal/vaxfloat, as the CPU reads one; the arithmetic is the CPU's
// own (cpu.ExtendedModulus, vaxfloat.POLYStep), so the routines and the
// instructions follow one set of rules. The statuses are returned, as the
// manual lists them, with one exception VMS's run showed (testdata/lib53):
// LIB$EMODx's reserved operand (a floating operand whose sign is set and
// exponent zero) is signaled as SS$_ROPRAND, the EMOD instruction's fault,
// where LIB$POLYx's is returned. Nothing is stored either way.

var (
	ssFltOvf = vmsdef.Symbols["SS$_FLTOVF"]
	ssFltUnd = vmsdef.Symbols["SS$_FLTUND"]
)

// loadFloat reads the format f operand at addr. A reserved operand is
// vaxfloat.ErrReserved; any other error is the memory's.
func loadFloat(env *corevms.Environment, f vaxfloat.Format, addr uint32) (vaxfloat.Value, error) {
	mem, c := env.Memory(), env.CPU()

	var bits vaxfloat.Bits

	if f.Size() == 4 {
		l, err := mem.LoadLongword(c, addr)
		if err != nil {
			return vaxfloat.Value{}, err
		}

		bits.Lo = uint64(l)
	} else {
		q, err := mem.LoadQuadword(c, addr)
		if err != nil {
			return vaxfloat.Value{}, err
		}

		bits.Lo = q

		// H_floating's second quadword: the low-order fraction bits.
		if f.Size() == 16 {
			if bits.Hi, err = mem.LoadQuadword(c, addr+8); err != nil {
				return vaxfloat.Value{}, err
			}
		}
	}

	return vaxfloat.Unpack(f, bits)
}

// LoadFloat is loadFloat for another RTL package's routines
// (internal/mthrtl).
func LoadFloat(env *corevms.Environment, f vaxfloat.Format, addr uint32) (vaxfloat.Value, error) {
	return loadFloat(env, f, addr)
}

// StoreFloat is storeFloat for another RTL package's routines.
func StoreFloat(env *corevms.Environment, f vaxfloat.Format, addr uint32, v vaxfloat.Value) error {
	return storeFloat(env, f, addr, v)
}

// storeFloat writes v, already rounded to format f, at addr. An error is
// the memory's, marked as a write's.
func storeFloat(env *corevms.Environment, f vaxfloat.Format, addr uint32, v vaxfloat.Value) error {
	mem, c := env.Memory(), env.CPU()

	// A value rounded to f packs without error.
	bits, _ := vaxfloat.Pack(f, v)

	var err error

	switch f.Size() {
	case 4:
		err = mem.StoreLongword(c, addr, uint32(bits.Lo))
	case 8:
		err = mem.StoreQuadword(c, addr, bits.Lo)
	default:
		if err = mem.StoreQuadword(c, addr, bits.Lo); err == nil {
			err = mem.StoreQuadword(c, addr+8, bits.Hi)
		}
	}

	if err != nil {
		return writing(err)
	}

	return nil
}

// floatFailure is the routine's result for an error from loadFloat or
// storeFloat: SS$_ROPRAND, returned, for a reserved operand, and
// memoryFault's signaled SS$_ACCVIO for the rest.
func floatFailure(env *corevms.Environment, err error) (uint32, error) {
	if errors.Is(err, vaxfloat.ErrReserved) {
		return ssRopRand, nil
	}

	return memoryFault(env, err)
}

// emodFailure is LIB$EMODx's floatFailure: a reserved operand is
// signaled, as the EMOD instruction's fault is.
func emodFailure(env *corevms.Environment, err error) (uint32, error) {
	if errors.Is(err, vaxfloat.ErrReserved) {
		return reservedOperand(env)
	}

	return memoryFault(env, err)
}

// libEmodf is LIB$EMODF (LIB-130).
//
//	LIB$EMODF floating-point-multiplier ,multiplier-extension ,floating-point-multiplicand ,integer-portion ,fractional-portion
func libEmodf(env *corevms.Environment, argv []uint32) (uint32, error) {
	return emod(env, argv, vaxfloat.F)
}

// libEmodd is LIB$EMODD (LIB-128), LIB$EMODF in D_floating.
func libEmodd(env *corevms.Environment, argv []uint32) (uint32, error) {
	return emod(env, argv, vaxfloat.D)
}

// libEmodg is LIB$EMODG (LIB-132), LIB$EMODF in G_floating, its
// multiplier extension a word.
func libEmodg(env *corevms.Environment, argv []uint32) (uint32, error) {
	return emod(env, argv, vaxfloat.G)
}

// libEmodh is LIB$EMODH (LIB-134), LIB$EMODF in H_floating, its
// multiplier extension a word.
func libEmodh(env *corevms.Environment, argv []uint32) (uint32, error) {
	return emod(env, argv, vaxfloat.H)
}

// emod is LIB$EMODx in format f: the multiplier, extended by the
// extension's bits (a byte's 8 for F and D, a word's high 11 for G and
// high 15 for H), times the multiplicand, split into an integer (a
// longword) and a fraction (format f) of the same sign.
//
// The statuses are the manual's: SS$_INTOVF when the integer part doesn't
// fit a longword (its low-order 32 bits are stored, and the fraction);
// SS$_FLTUND when the fraction underflows (both stored as zero). A
// reserved operand is signaled as SS$_ROPRAND, neither stored: the manual
// lists it as returned, but VMS signals it (testdata/lib53). The manual says a
// floating overflow is SS$_INTOVF too; the fraction, less than 1 in
// magnitude, can't overflow, so only the integer part can.
func emod(env *corevms.Environment, argv []uint32, f vaxfloat.Format) (uint32, error) {
	mem, c := env.Memory(), env.CPU()

	mulr, err := loadFloat(env, f, arg(argv, 0))
	if err != nil {
		return emodFailure(env, err)
	}

	var ext uint16

	if f == vaxfloat.F || f == vaxfloat.D {
		b, err := mem.LoadByte(c, arg(argv, 1))
		if err != nil {
			return memoryFault(env, err)
		}

		ext = uint16(b)
	} else {
		if ext, err = mem.LoadWord(c, arg(argv, 1)); err != nil {
			return memoryFault(env, err)
		}
	}

	muld, err := loadFloat(env, f, arg(argv, 2))
	if err != nil {
		return emodFailure(env, err)
	}

	integer, fraction, overflow, err := cpu.ExtendedModulus(f, mulr, ext, muld)

	status := ssNormal

	switch {
	case errors.Is(err, vaxfloat.ErrUnderflow):
		// Both results are zero.
		status = ssFltUnd
	case err != nil:
		return emodFailure(env, err)
	case overflow:
		status = ssIntOvf
	}

	if err := mem.StoreLongword(c, arg(argv, 3), integer); err != nil {
		return memoryFault(env, writing(err))
	}

	if err := storeFloat(env, f, arg(argv, 4), fraction); err != nil {
		return memoryFault(env, err)
	}

	return status, nil
}

// libPolyf is LIB$POLYF (LIB-302).
//
//	LIB$POLYF polynomial-argument ,degree ,coefficient ,floating-point-result
func libPolyf(env *corevms.Environment, argv []uint32) (uint32, error) {
	return poly(env, argv, vaxfloat.F)
}

// libPolyd is LIB$POLYD (LIB-300), LIB$POLYF in D_floating.
func libPolyd(env *corevms.Environment, argv []uint32) (uint32, error) {
	return poly(env, argv, vaxfloat.D)
}

// libPolyg is LIB$POLYG (LIB-305), LIB$POLYF in G_floating.
func libPolyg(env *corevms.Environment, argv []uint32) (uint32, error) {
	return poly(env, argv, vaxfloat.G)
}

// libPolyh is LIB$POLYH (LIB-307), LIB$POLYF in H_floating.
func libPolyh(env *corevms.Environment, argv []uint32) (uint32, error) {
	return poly(env, argv, vaxfloat.H)
}

// poly is LIB$POLYx in format f: the polynomial whose degree+1
// coefficients are in the array (the highest-order term's first, at the
// lowest address) evaluated at the argument by Horner's method, each
// step POLY's (vaxfloat.POLYStep: multiply and add, each truncated, then
// rounded).
//
// The degree is an unsigned word, and unlike the POLY instruction's (at
// most 31) any degree is evaluated: VMS's LIB$POLYF of degree 32 is
// SS$_NORMAL (testdata/lib53). A reserved argument or coefficient is
// SS$_ROPRAND, returned. A step that overflows is SS$_FLTOVF, the result
// not stored; a step that underflows makes the partial result zero and
// the evaluation goes on, as the instruction does with PSL<FU> clear
// (both as VMS does).
func poly(env *corevms.Environment, argv []uint32, f vaxfloat.Format) (uint32, error) {
	mem, c := env.Memory(), env.CPU()
	size := uint32(f.Size())

	x, err := loadFloat(env, f, arg(argv, 0))
	if err != nil {
		return floatFailure(env, err)
	}

	degree, err := mem.LoadWord(c, arg(argv, 1))
	if err != nil {
		return memoryFault(env, err)
	}

	table := arg(argv, 2)

	result, err := loadFloat(env, f, table)
	if err != nil {
		return floatFailure(env, err)
	}

	for i := range uint32(degree) {
		coefficient, err := loadFloat(env, f, table+size*(i+1))
		if err != nil {
			return floatFailure(env, err)
		}

		result, err = vaxfloat.POLYStep(f, x, result, coefficient)

		switch {
		case errors.Is(err, vaxfloat.ErrOverflow):
			return ssFltOvf, nil
		case err != nil && !errors.Is(err, vaxfloat.ErrUnderflow):
			return floatFailure(env, err)
		}
	}

	if err := storeFloat(env, f, arg(argv, 3), result); err != nil {
		return memoryFault(env, err)
	}

	return ssNormal, nil
}
