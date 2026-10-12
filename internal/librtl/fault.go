package librtl

import (
	"errors"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/vm"
)

// Access violations (docs/PHASE-53, Decision 5). On VMS a LIB$ routine is
// VAX code: when it reads or writes an argument it can't reach, its own
// instruction faults, and the hardware's access violation is *signaled*,
// not returned. The program's condition handlers see SS$_ACCVIO, and with
// none to take it, the catch-all handler shows
// "%SYSTEM-F-ACCVIO, access violation, reason mask=..., virtual
// address=..." and ends the image (the condition is SEVERE).
//
// A routine written in Go does the same through memoryFault: given the
// error from its own load or store, it signals SS$_ACCVIO with the
// signal array the hardware would have built. If a handler continues
// the condition anyway, the routine returns SS$_ACCVIO.

// The reason mask of an access violation (the VAX Architecture Reference
// Manual): which kind of failure it was.
const (
	accvioLength = 1 << 0 // the address was beyond its region's length
	accvioWrite  = 1 << 2 // the access was a write (or a modify)
)

// writeFault marks a memory error as a write's, for the reason mask: the
// memory errors themselves don't say which kind of access failed.
type writeFault struct{ err error }

func (w writeFault) Error() string { return w.err.Error() }
func (w writeFault) Unwrap() error { return w.err }

// writing returns err marked as a write's (nil stays nil), so that
// memoryFault sets the reason mask's write bit:
//
//	if err := mem.StoreLongword(cpu, addr, v); err != nil {
//		return memoryFault(env, writing(err))
//	}
func writing(err error) error {
	if err == nil {
		return nil
	}

	return writeFault{err}
}

// memoryFault is what a routine does with the error from one of its own
// loads or stores of an argument: a memory error (a translation fault)
// is signaled as SS$_ACCVIO at the address it names. Any other error is
// returned as it is, for the XFC to report.
func memoryFault(env *corevms.Environment, err error) (uint32, error) {
	var (
		reason uint32
		va     uint32
		tf     *vm.TranslationFault
		pe     *vm.PhysicalAddressError
		wf     writeFault
	)

	switch {
	case errors.As(err, &tf):
		va = tf.Addr

		if tf.Kind == vm.AccessViolation {
			reason |= accvioLength
		}

	case errors.As(err, &pe):
		va = pe.Addr

	default:
		return 0, err
	}

	if errors.As(err, &wf) {
		reason |= accvioWrite
	}

	return accessViolation(env, va, reason)
}

// accessViolation signals SS$_ACCVIO for an access at va, with the
// reason mask reason. The signal array is the hardware's: the condition,
// the reason mask, and the address (the PC and PSL follow, as for any
// signal).
func accessViolation(env *corevms.Environment, va, reason uint32) (uint32, error) {
	return env.SignalFailure([]uint32{ssAccVio, reason, va}, ssAccVio)
}

// stringFailure is what a routine does with a descriptor helper's error
// (readString, writeString): an invalid descriptor or no memory for a
// dynamic string is a status it returns (LIB$_INVSTRDES,
// LIB$_INSVIRMEM); a memory error is an access violation (memoryFault).
func stringFailure(env *corevms.Environment, err error) (uint32, error) {
	switch {
	case errors.Is(err, errInvStrDes):
		return libInvStrDes, nil
	case errors.Is(err, errInsVirMem):
		return libInsVirMem, nil
	default:
		return memoryFault(env, err)
	}
}

// stackString puts s, and a class S descriptor for it, on the stack below
// SP, moves SP below them, and returns the descriptor's address: a
// string to pass by descriptor in a signal's arguments, as a VAX routine
// would build one in its own frame. The space is the routine's call
// frame's, so its RET gives it back.
func stackString(env *corevms.Environment, s string) (uint32, error) {
	mem, cpu := env.Memory(), env.CPU()
	size := (uint32(len(s)) + 3) &^ 3
	desc := cpu.GPR(registers.SP) - size - 8
	text := desc + 8

	for i := range len(s) {
		if err := mem.StoreByte(cpu, text+uint32(i), s[i]); err != nil {
			return 0, err
		}
	}

	if err := mem.StoreLongword(cpu, desc, uint32(len(s))|dscClassS<<24|dscDTypeT<<16); err != nil {
		return 0, err
	}

	if err := mem.StoreLongword(cpu, desc+4, text); err != nil {
		return 0, err
	}

	cpu.SetGPR(registers.SP, desc)

	return desc, nil
}
