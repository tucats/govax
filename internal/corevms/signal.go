package corevms

import (
	"fmt"

	"github.com/tucats/govax/internal/vax"
)

// Software conditions (docs/PHASE-26.md subtask 33): the machinery behind
// LIB$SIGNAL, LIB$STOP, LIB$ESTABLISH, and LIB$REVERT, whose routines are
// in internal/librtl (docs/PHASE-34.md, subtask 10) and reach this file
// through Signal and SetCallerHandler (export.go).
//
// # Signaling from software
//
// Hardware exceptions aren't the only conditions. A routine that finds
// an error can *signal* it, instead of (or as well as) returning an error
// status, by calling LIB$SIGNAL:
//
//	LIB$SIGNAL condition-value [,number-of-arguments] [,FAO-argument...]
//
// Its argument list becomes the signal array, with the PC and PSL of the
// call appended; then the condition goes through the same search a
// hardware exception does (condition.go): exception vectors, call-frame
// handlers, the catch-all. If a handler continues, LIB$SIGNAL returns to
// its caller, with R0 and R1 from the mechanism array. If none does and
// the condition isn't SEVERE, the catch-all prints its message and
// LIB$SIGNAL returns too; a SEVERE one ends the image.
//
// LIB$STOP is LIB$SIGNAL for errors the caller can't go on from: it
// forces the condition's severity to SEVERE, and it never returns. A
// handler that tries to continue gets "%LIB-F-ATTCONSTO, attempt to
// continue from stop" and the image exits; the only way on is $UNWIND.
//
// The search starts with LIB$SIGNAL's caller's frame, at depth 0: the
// LIB manual says "the call to LIB$SIGNAL or LIB$STOP is not included in
// the depth", so that a software condition looks just like a hardware
// one that happened in the caller.
//
// # How govax does it
//
// The routines are shims: a program's CALLS lands on a small stub
// (entry mask; MOVL #code, R0; XFC; RET) whose XFC runs the Go function
// registered for the code. LIB$SIGNAL's shim starts a dispatch the way
// DispatchException does, recording that continuing means "return from
// LIB$SIGNAL": resume at the stub's RET, which is where PC points while
// the shim runs, with the stub's FP and SP.
//
// LIB$ESTABLISH and LIB$REVERT set and clear the handler of the *calling*
// procedure's frame. The stub's own frame is LIB$ESTABLISH's; its saved
// FP (at 12(FP)) is the caller's frame.

// Offsets in a call frame (see buildCallFrame in internal/cpu): the
// condition handler, the saved PSW (the low word of the mask longword),
// the saved FP, and the saved PC (the return address).
const (
	frameHandler = 0
	frameMaskPSW = 4
	frameSavedFP = 12
	frameSavedPC = 16
)

// signal starts dispatching the condition described by argv, the
// argument list of a LIB$SIGNAL or LIB$STOP call, whose stub is running.
// It returns 0 for R0: the XFC handler stores that before execution goes
// on at SYS$SRCHANDLER, which leaves R0 to the handlers.
//
// The signal array is the argument list plus the PC the call returns to
// (the stub frame's saved PC) and the caller's PSL (the current PSL with
// the PSW the call saved in the frame). The mechanism array's R1 is the
// caller's, but its R0 is 0: the stub's MOVL #code, R0 has already
// replaced the caller's R0 by the time the shim runs.
func (env *Environment) signal(kind signalKind, argv []uint32) (uint32, error) {
	c := env.cpu
	name := "LIB$SIGNAL"

	if kind == kindStop {
		name = "LIB$STOP"
	}

	if len(argv) == 0 {
		return 0, fmt.Errorf("rtl: %s called with no condition value", name)
	}

	fp := c.GPR(vax.FP)

	callerFP, err1 := env.mem.LoadLongword(c, fp+frameSavedFP)
	returnPC, err2 := env.mem.LoadLongword(c, fp+frameSavedPC)
	maskPSW, err3 := env.mem.LoadLongword(c, fp+frameMaskPSW)

	if err1 != nil || err2 != nil || err3 != nil {
		return 0, fmt.Errorf("rtl: %s can't read its call frame at %08X", name, fp)
	}

	sig := append([]uint32(nil), argv...)
	if kind == kindStop {
		sig[0] = sig[0]&^7 | severitySevere
	}

	psl := c.PSL()&^0xFFFF | vax.PSL(maskPSW&0xFFFF)
	sig = append(sig, returnPC, uint32(psl))

	d := &conditionDispatch{
		kind:      kind,
		resumeSP:  c.GPR(vax.SP),
		resumeFP:  fp,
		resumePC:  c.GPR(vax.PC),
		resumePSL: c.PSL(),
	}

	if !env.startDispatch(d, sig, 0, c.GPR(vax.R1)) {
		return 0, fmt.Errorf("rtl: %s can't signal: no SYS$SRCHANDLER stub (the program needs .P1VECTOR), or the stack at %08X can't be written", name, c.GPR(vax.SP))
	}

	// Search from the caller's frame, not LIB$SIGNAL's own.
	d.startFP, d.fp = callerFP, callerFP

	if c.DebugEnabled(vax.DebugExceptions) {
		fmt.Fprintf(c.DebugWriter(), "DEBUG(EXCEPTION): %s, CONDITION=%08X  PC=%08X  SIGNAL=%08X\n", name, sig[0], returnPC, d.sig)
	}

	return 0, nil
}

// setCallerHandler stores handler in the frame of the procedure that
// called the running shim, returning the previous one.
func (env *Environment) setCallerHandler(name string, handler uint32) (uint32, error) {
	c := env.cpu

	callerFP, err := env.mem.LoadLongword(c, c.GPR(vax.FP)+frameSavedFP)
	if err != nil {
		return 0, fmt.Errorf("rtl: %s can't read its call frame: %w", name, err)
	}

	old, err := env.mem.LoadLongword(c, callerFP+frameHandler)
	if err != nil {
		return 0, fmt.Errorf("rtl: %s can't read its caller's frame at %08X: %w", name, callerFP, err)
	}

	if err := env.mem.StoreLongword(c, callerFP+frameHandler, handler); err != nil {
		return 0, fmt.Errorf("rtl: %s can't write its caller's frame at %08X: %w", name, callerFP, err)
	}

	return old, nil
}

// condIDMask is STS$M_COND_ID, bits 3-27 of a condition value: the
// facility and message number, without the severity (bits 0-2) or the
// control bits (28-31), which a handler along the way may have changed.
const condIDMask = 0x0FFFFFF8
