package mthrtl

import (
	"errors"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/librtl"
	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/vaxfloat"
	"github.com/tucats/govax/internal/vmsdef"
)

// The library's conditions. A routine that can't produce a result (a
// square root of a negative number, an exponential too large for its
// format) signals one, through the same search as LIB$SIGNAL's, with the
// message's one FAO argument the "user PC": where the routine's caller
// goes on. All of them are SEVERE, so with no handler to take the condition
// the image ends.
//
// A handler may instead return SS$_CONTINUE, and the routine then returns
// to its caller with R0 and R1 from the mechanism array. Those start as the
// reserved operand, the VAX's "not a number": a floating value with its
// sign set and its exponent zero, which faults when an instruction reads
// it, so a program that continues can't use the result by accident. A
// handler that knows better stores a corrected value in the mechanism array.
// A result in memory or in R2:R3 can't be corrected that way.
//
// The argument list's shape, and the user PC, are the manual's; the VMS
// probe (subtask 10) is to confirm them.

// reservedR0 is the reserved operand's first longword, in any format: the
// sign bit set (bit 15 of the first word) and everything else clear. The
// rest of the value, in R1, is 0.
const reservedR0 = 0x8000

var ssRopRand = vmsdef.Symbols["SS$_ROPRAND"]

// signalCondition signals one of the library's conditions, from a CALL or
// a JSB entry, and returns what the routine returns: the shim starts the
// search, and the handlers' answer decides what the caller sees.
func signalCondition(env *corevms.Environment, condition uint32) (uint32, error) {
	pc, err := env.CallerPC()
	if err != nil {
		return 0, err
	}

	return signalReserved(env, []uint32{condition, 1, pc})
}

// signalReserved signals the condition argv describes, with the reserved
// operand as the R0 and R1 a handler that continues gives the caller.
// signal takes R1 from the CPU, so that's where it's set.
func signalReserved(env *corevms.Environment, argv []uint32) (uint32, error) {
	env.CPU().SetGPR(registers.R1, 0)

	return env.SignalFailure(argv, reservedR0)
}

// signalZero signals condition with the user PC, as signalCondition does, but
// with 0.0 as the R0 and R1 a handler that continues gives the caller: the
// manual's result for an underflow.
func signalZero(env *corevms.Environment, condition uint32) (uint32, error) {
	pc, err := env.CallerPC()
	if err != nil {
		return 0, err
	}

	env.CPU().SetGPR(registers.R1, 0)

	return env.SignalFailure([]uint32{condition, 1, pc}, 0)
}

// fail is what a routine does with the error from reading an argument or
// writing a result: a reserved operand is signaled as SS$_ROPRAND, the
// fault the hardware's own floating instructions take on one, and a memory
// error as SS$_ACCVIO. Any other error is returned for the XFC to report.
//
// VMS's PC in these signals would be inside MTHRTL, where the routine's
// instruction faulted; here it's the routine's return address (the phase
// doc's Decision 8). No program can depend on an address inside the
// library.
func fail(env *corevms.Environment, err error) (uint32, error) {
	if errors.Is(err, vaxfloat.ErrReserved) {
		return signalReserved(env, []uint32{ssRopRand})
	}

	return librtl.MemoryFault(env, err)
}
