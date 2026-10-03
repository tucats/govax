package corevms

import (
	"fmt"

	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vmsdef"
)

// $UNWIND and LIB$SIG_TO_RET (docs/PHASE-26.md subtask 34): a condition
// handler's third answer.
//
// # Unwinding
//
// A handler can resignal or continue (condition.go), or it can abandon
// the procedures the condition happened in altogether: *unwind the call
// stack*. It calls $UNWIND, which arranges that when the handler
// returns, the procedures between the condition and some caller further
// up are removed, as if each had executed RET, and execution resumes in
// that caller: by default the caller of the procedure whose frame
// established the handler, just after its CALL, as though the
// establisher had returned. R0 and R1 come from the mechanism array, so
// the handler decides what the establisher "returned". This is how a
// program turns a condition into an error status, and the only way on
// after LIB$STOP.
//
//	SYS$UNWIND [depadr] ,[newpc]
//
// depadr is the address of a longword saying how many frames to remove,
// counting from the frame the condition happened in (0: none, 1: that
// frame, returning to its caller, ...). newpc, if given, is where the
// caller resumes instead of its return address.
//
// Before a frame is removed, its handler, if it has one, is called once
// more, with the signal array's condition changed to SS$_UNWIND, so it
// can clean up (release a lock, close a file). Its answer doesn't
// matter. That includes the establisher's own handler, which is removed
// too: a handler has to check for SS$_UNWIND and just return.
//
// # How govax does it
//
// $UNWIND itself only records the request (conditionDispatch.unwind)
// and returns SS$_NORMAL; as on VMS, nothing happens until the handler
// returns. Then serviceSysSrchandler, seeing the request, calls each
// removed frame's handler with SS$_UNWIND, one CallRequest at a time,
// and finally makes the frames go away the way VMS does, by changing
// return addresses: the saved PC of every frame but the last is pointed
// at a RET instruction (the one after SYS$SRCHANDLER's XFC), the last
// one's at newpc if given, and execution continues at that RET with FP
// at the innermost frame. Each RET removes a frame exactly as the
// program's own would, restoring the registers its entry mask saved and
// popping a CALLS's arguments, and returns into the next RET, until the
// last returns into the surviving caller.
//
// For LIB$SIGNAL and LIB$STOP, the shim stub's own frame is removed as
// well, first; it isn't counted in the depth, as it isn't in the search.

// unwindRequest is a $UNWIND waiting for its handler to return, and then
// in progress.
type unwindRequest struct {
	// frames are the call frames to remove, innermost first.
	frames []uint32

	// newPC is where the surviving caller resumes, if hasNewPC.
	newPC    uint32
	hasNewPC bool

	// next is the next frame whose handler is to be called with
	// SS$_UNWIND, and started is true once the signal array has been
	// changed to say SS$_UNWIND.
	next    int
	started bool
}

// Status values the unwind services use.
var (
	ssUnwind    = vmsdef.Symbols["SS$_UNWIND"]
	ssUnwinding = vmsdef.Symbols["SS$_UNWINDING"]
	ssInsFrame  = vmsdef.Symbols["SS$_INSFRAME"]
)

// unwindRETAddr returns the address of the RET instruction the unwind's
// frames return through: .P1VECTOR puts one right after each entry's XFC,
// and SYS$SRCHANDLER has no entry mask, so its RET is 2 bytes in.
func unwindRETAddr() uint32 {
	return srchandlerAddr + 2
}

// opRET is the RET instruction's opcode.
const opRET = 0x04

// handlingDispatch returns the innermost dispatch whose handler is
// running (the one a $UNWIND call comes from), or nil.
func (env *Environment) handlingDispatch() *conditionDispatch {
	for i := len(env.Process.conditions) - 1; i >= 0; i-- {
		if d := env.Process.conditions[i]; d.calling {
			return d
		}
	}

	return nil
}

// serviceSysUnwind is SYS$UNWIND (see this file's opening comment). It
// returns SS$_NORMAL (for a depth of 0, without doing anything),
// SS$_NOSIGNAL if no condition handler is running, SS$_UNWINDING if the
// handler has already asked for an unwind, SS$_INSFRAME if there aren't
// that many frames (or the program's outermost procedure would be
// removed), and SS$_ACCVIO if depadr or a frame can't be read.
//
// Called by a vectored handler without depadr, it does nothing: such a
// handler has no establisher frame to unwind past.
//
// newpc is the resume address itself: the manual describes it as "a
// longword value containing the address at which execution is to
// resume".
func serviceSysUnwind(env *Environment, argv []uint32) (uint32, error) {
	d := env.handlingDispatch()
	if d == nil {
		return ssNoSignal, nil
	}

	var depth int32

	if depadr := optArg(argv, 0); depadr != 0 {
		v, err := env.mem.LoadLongword(env.cpu, depadr)
		if err != nil {
			return ssAccVio, nil
		}

		depth = int32(v)
	} else {
		mechDepth, err := env.mem.LoadLongword(env.cpu, d.mech+8)
		if err != nil {
			return ssAccVio, nil
		}

		depth = max(int32(mechDepth), -1) + 1
	}

	return env.requestUnwind(d, depth, optArg(argv, 1), len(argv) > 1 && argv[1] != 0)
}

// requestUnwind records an unwind of depth frames for d, which must not
// already have one, finding the frames to remove.
func (env *Environment) requestUnwind(d *conditionDispatch, depth int32, newPC uint32, hasNewPC bool) (uint32, error) {
	if d.unwind != nil {
		return ssUnwinding, nil
	}

	if depth <= 0 {
		return ssNormal, nil
	}

	// ten is an estimate...
	frames := make([]uint32, 0, 10)

	if d.kind != kindException {
		frames = append(frames, d.resumeFP) // the LIB$SIGNAL stub's frame
	}

	fp := d.startFP

	for range depth {
		if fp == 0 || fp == consoleCallSentinel {
			return ssInsFrame, nil
		}

		frames = append(frames, fp)

		next, err := env.mem.LoadLongword(env.cpu, fp+frameSavedFP)
		if err != nil {
			return ssAccVio, nil
		}

		fp = next
	}

	// fp is now the frame that receives control; it has to be a real
	// one, not past the program's outermost procedure.
	if fp == 0 || fp == consoleCallSentinel {
		return ssInsFrame, nil
	}

	d.unwind = &unwindRequest{frames: frames, newPC: newPC, hasNewPC: hasNewPC}

	return ssNormal, nil
}

// continueUnwind carries out d's unwind once the handler that asked for
// it has returned (see this file's opening comment): the next removed
// frame's handler is called with SS$_UNWIND, or, when they all have
// been, the frames are removed.
func (env *Environment) continueUnwind(d *conditionDispatch) (uint32, error) {
	c := env.cpu
	u := d.unwind

	for u.next < len(u.frames) {
		frame := u.frames[u.next]
		u.next++

		handler, err := env.mem.LoadLongword(c, frame+frameHandler)
		if err != nil {
			return 0, fmt.Errorf("rtl: can't read the call frame at %08X during an unwind: %w", frame, err)
		}

		if handler == 0 {
			continue
		}

		if !u.started {
			u.started = true

			if env.mem.StoreLongword(c, d.sig, 1) != nil || env.mem.StoreLongword(c, d.sig+4, ssUnwind) != nil {
				return 0, fmt.Errorf("rtl: can't write the signal array at %08X", d.sig)
			}
		}

		// The depth is the frame's position counting from the frame the
		// condition happened in, not counting a LIB$SIGNAL stub.
		depth := int32(u.next - 1)
		if d.kind != kindException {
			depth--
		}

		return env.callConditionHandler(d, handler, frame, depth)
	}

	return env.finishUnwind(d)
}

// finishUnwind removes u's frames by pointing their return addresses at
// a RET and executing it (see this file's opening comment), with R0 and
// R1 from the mechanism array. Dispatches whose stacks the unwind
// discards (a condition in a handler that unwinds past its own
// dispatch's frames) end too.
func (env *Environment) finishUnwind(d *conditionDispatch) (uint32, error) {
	c := env.cpu
	u := d.unwind
	last := len(u.frames) - 1

	ret := unwindRETAddr()

	if op, err := env.mem.LoadByte(c, ret); err != nil || op != opRET {
		return 0, fmt.Errorf("rtl: can't unwind: no RET after the SYS$SRCHANDLER stub at %08X", ret)
	}

	for i, frame := range u.frames {
		pc := ret

		if i == last {
			if !u.hasNewPC {
				break
			}

			pc = u.newPC
		}

		if err := env.mem.StoreLongword(c, frame+frameSavedPC, pc); err != nil {
			return 0, fmt.Errorf("rtl: can't change the return address in the call frame at %08X: %w", frame, err)
		}
	}

	r0, err1 := env.mem.LoadLongword(c, d.mech+12)
	r1, err2 := env.mem.LoadLongword(c, d.mech+16)

	if err1 != nil || err2 != nil {
		return 0, fmt.Errorf("rtl: can't read the mechanism array at %08X", d.mech)
	}

	// The frame that receives control is the last removed frame's
	// caller; any dispatch whose stack lies below it is gone.
	survivor, err := env.mem.LoadLongword(c, u.frames[last]+frameSavedFP)
	if err != nil {
		return 0, fmt.Errorf("rtl: can't read the call frame at %08X: %w", u.frames[last], err)
	}

	env.endDispatch()

	for n := len(env.Process.conditions); n > 0 && env.Process.conditions[n-1].sp < survivor; n-- {
		env.endDispatch()
	}

	c.SetGPR(vax.R1, r1)
	c.SetGPR(vax.FP, u.frames[0])
	c.SetGPR(vax.PC, ret)

	return r0, nil
}

// conditionToReturn is LIB$SIG_TO_RET's work (see ConditionToReturn):
// a condition handler (usually established directly, with LIB$ESTABLISH)
// that turns any condition into a return status: it stores the condition
// value as the mechanism array's R0 and unwinds to the caller of the
// procedure that established it, which therefore sees that procedure
// return the condition value. It returns SS$_NORMAL, or $UNWIND's error
// status.
func (env *Environment) conditionToReturn(sig, mech uint32) (uint32, error) {
	d := env.handlingDispatch()
	if d == nil {
		return ssNoSignal, nil
	}

	cond, err1 := env.mem.LoadLongword(env.cpu, sig+4)
	depth, err2 := env.mem.LoadLongword(env.cpu, mech+8)

	if err1 != nil || err2 != nil {
		return ssAccVio, nil
	}

	if cond&condIDMask == ssUnwind&condIDMask {
		return ssNormal, nil
	}

	if env.mem.StoreLongword(env.cpu, mech+12, cond) != nil {
		return ssAccVio, nil
	}

	return env.requestUnwind(d, max(int32(depth), -1)+1, 0, false)
}
