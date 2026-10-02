package librtl

import "github.com/tucats/govax/internal/rtl"

// The condition-handling routines (moved from rtl, docs/PHASE-26.md
// subtask 33). The dispatch itself -- building the signal and mechanism
// arrays, searching the call frames, unwinding -- is rtl's; see
// internal/rtl/signal.go and unwind.go.

// libSignal is LIB$SIGNAL:
//
//	LIB$SIGNAL condition-value [,number-of-arguments] [,FAO-argument...]
//
// It signals the condition: its argument list becomes the signal array,
// and the condition goes through the same search a hardware exception does.
func libSignal(env *rtl.Environment, argv []uint32) (uint32, error) {
	return env.Signal(argv, false)
}

// libStop is LIB$STOP: LIB$SIGNAL with the severity forced to SEVERE,
// which can't be continued.
func libStop(env *rtl.Environment, argv []uint32) (uint32, error) {
	return env.Signal(argv, true)
}

// libEstablish is LIB$ESTABLISH:
//
//	LIB$ESTABLISH new-handler
//
// It makes new-handler (a procedure's entry mask address, passed by value)
// the condition handler of the calling procedure's frame, and returns the
// one it replaces (0 if none).
func libEstablish(env *rtl.Environment, argv []uint32) (uint32, error) {
	return env.SetCallerHandler("LIB$ESTABLISH", arg(argv, 0))
}

// libRevert is LIB$REVERT: it removes the calling procedure's condition
// handler, returning the one it removes (0 if none).
func libRevert(env *rtl.Environment, _ []uint32) (uint32, error) {
	return env.SetCallerHandler("LIB$REVERT", 0)
}

// libSigToRet is LIB$SIG_TO_RET:
//
//	LIB$SIG_TO_RET signal-args ,mechanism-args
//
// A condition handler that turns any condition into a return status: the
// procedure that established it returns the condition value to its caller.
// It returns SS$_NORMAL, SS$_NOSIGNAL outside a handler, or $UNWIND's
// error status.
func libSigToRet(env *rtl.Environment, argv []uint32) (uint32, error) {
	if len(argv) < 2 {
		return ssNoSignal, nil
	}

	return env.ConditionToReturn(argv[0], argv[1])
}

// condIDMask is STS$M_COND_ID, bits 3-27 of a condition value: the facility
// and message number, without the severity (bits 0-2) or the control bits
// (28-31), which a handler along the way may have changed.
const condIDMask = 0x0FFFFFF8

// libMatchCond is LIB$MATCH_COND:
//
//	LIB$MATCH_COND condition-value ,condition-value-1 [,condition-value-2...]
//
// Every argument is a condition value passed by reference. It compares the
// first with each of the others, by their STS$V_COND_ID fields only, and
// returns the (1-based) position of the first that matches, or 0 if none
// does. An argument that can't be read doesn't match.
func libMatchCond(env *rtl.Environment, argv []uint32) (uint32, error) {
	if len(argv) < 2 {
		return 0, nil
	}

	mem, cpu := env.Memory(), env.CPU()

	want, err := mem.LoadLongword(cpu, argv[0])
	if err != nil {
		return 0, nil
	}

	for i, adr := range argv[1:] {
		if v, err := mem.LoadLongword(cpu, adr); err == nil && v&condIDMask == want&condIDMask {
			return uint32(i + 1), nil
		}
	}

	return 0, nil
}
