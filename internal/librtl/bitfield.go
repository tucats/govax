package librtl

import (
	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/vmsdef"
)

// The bit field routines (docs/PHASE-53, subtask 6): LIB$EXTV, LIB$EXTZV,
// LIB$INSV, LIB$FFC, and LIB$FFS make the VAX's variable-length bit field
// instructions callable.
//
// A variable-length bit field is up to 32 bits anywhere in memory, named
// by three things: a base address (a byte), a position (a signed bit
// offset from bit 0 of that byte; negative reaches lower addresses), and
// a size in bits (0 to 32). Each routine takes the position by reference
// (a signed longword) and the size by reference (an unsigned byte). The
// base is the field's address itself: the manual's examples pass the
// variable holding the field (LIB-144, LIB-254), so the argument is that
// variable's address. LIB$FFx's entry words its base argument as "the
// address of an unsigned longword containing the base address"; govax
// reads it the same way as the others (unconfirmed).
//
// A size above 32 is the instruction's reserved operand fault, which the
// manual lists as signaled (SS$_ROPRAND). The field itself is read and
// written by the CPU's own functions (cpu.MemoryField, cpu.SetMemoryField,
// cpu.FirstBit), so the routines and the instructions follow one set of
// rules.

var (
	ssRopRand = vmsdef.Symbols["SS$_ROPRAND"]
	libNotFou = vmsdef.Symbols["LIB$_NOTFOU"]
)

// maxFieldSize is the largest bit field, a longword.
const maxFieldSize = 32

// reservedOperand signals SS$_ROPRAND as the hardware's reserved operand
// fault would be: the condition alone (the PC and PSL follow, as for any
// signal). If a handler continues it, the routine returns SS$_ROPRAND.
func reservedOperand(env *corevms.Environment) (uint32, error) {
	return env.SignalFailure([]uint32{ssRopRand}, ssRopRand)
}

// fieldArgs reads the position (a longword) and size (a byte) arguments
// at argv[i] and argv[i+1]. ok is false when the size is above 32; the
// caller then signals the reserved operand.
func fieldArgs(env *corevms.Environment, argv []uint32, i int) (position int32, size int, ok bool, err error) {
	mem, c := env.Memory(), env.CPU()

	p, err := mem.LoadLongword(c, arg(argv, i))
	if err != nil {
		return 0, 0, false, err
	}

	s, err := mem.LoadByte(c, arg(argv, i+1))
	if err != nil {
		return 0, 0, false, err
	}

	return int32(p), int(s), int(s) <= maxFieldSize, nil
}

// libExtv is LIB$EXTV (LIB-142): the field, sign-extended to a longword,
// is the routine's value.
//
//	LIB$EXTV position ,size ,base-address
func libExtv(env *corevms.Environment, argv []uint32) (uint32, error) {
	return extract(env, argv, true)
}

// libExtzv is LIB$EXTZV (LIB-145): LIB$EXTV, zero-extending the field.
//
//	LIB$EXTZV position ,size ,base-address
func libExtzv(env *corevms.Environment, argv []uint32) (uint32, error) {
	return extract(env, argv, false)
}

// extract is LIB$EXTV and LIB$EXTZV. A size of 0 is a field of no bits,
// whose value is 0.
func extract(env *corevms.Environment, argv []uint32, signed bool) (uint32, error) {
	position, size, ok, err := fieldArgs(env, argv, 0)
	if err != nil {
		return memoryFault(env, err)
	}

	if !ok {
		return reservedOperand(env)
	}

	v, err := cpu.MemoryField(env.CPU(), env.Memory(), position, size, arg(argv, 2), signed)
	if err != nil {
		return memoryFault(env, err)
	}

	return v, nil
}

// libInsv is LIB$INSV (LIB-253): bits 0 to size-1 of the source longword
// replace the field; a size of 0 stores nothing. The manual gives it no
// value; it returns SS$_NORMAL.
//
//	LIB$INSV longword-integer-source ,position ,size ,base-address
func libInsv(env *corevms.Environment, argv []uint32) (uint32, error) {
	mem, c := env.Memory(), env.CPU()

	src, err := mem.LoadLongword(c, arg(argv, 0))
	if err != nil {
		return memoryFault(env, err)
	}

	position, size, ok, err := fieldArgs(env, argv, 1)
	if err != nil {
		return memoryFault(env, err)
	}

	if !ok {
		return reservedOperand(env)
	}

	// The field is modified, so a failure to reach it is a write's (the
	// reason mask's write bit), whether its read or its write failed.
	if err := cpu.SetMemoryField(c, mem, position, size, arg(argv, 3), src); err != nil {
		return memoryFault(env, writing(err))
	}

	return ssNormal, nil
}

// libFfc is LIB$FFC (LIB-147): find the field's first clear bit.
//
//	LIB$FFC position ,size ,base ,find-position
func libFfc(env *corevms.Environment, argv []uint32) (uint32, error) {
	return findFirst(env, argv, false)
}

// libFfs is LIB$FFS (LIB-147): find the field's first set bit.
//
//	LIB$FFS position ,size ,base ,find-position
func libFfs(env *corevms.Environment, argv []uint32) (uint32, error) {
	return findFirst(env, argv, true)
}

// findFirst is LIB$FFC and LIB$FFS. find-position gets the found bit's
// position relative to the base (not to the field's start) and the
// status is SS$_NORMAL; with no such bit, or a size of 0, it gets
// position + size and the status is LIB$_NOTFOU.
func findFirst(env *corevms.Environment, argv []uint32, set bool) (uint32, error) {
	mem, c := env.Memory(), env.CPU()

	position, size, ok, err := fieldArgs(env, argv, 0)
	if err != nil {
		return memoryFault(env, err)
	}

	if !ok {
		return reservedOperand(env)
	}

	field, err := cpu.MemoryField(c, mem, position, size, arg(argv, 2), false)
	if err != nil {
		return memoryFault(env, err)
	}

	n, found := cpu.FirstBit(field, size, set)

	if err := mem.StoreLongword(c, arg(argv, 3), uint32(position)+uint32(n)); err != nil {
		return memoryFault(env, writing(err))
	}

	if !found {
		return libNotFou, nil
	}

	return ssNormal, nil
}
