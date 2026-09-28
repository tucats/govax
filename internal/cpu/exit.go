package cpu

import "github.com/tucats/govax/internal/vax"

// The engine's half of image exit (docs/PHASE-26.md subtask 19).
//
// When a VMS program calls $EXIT, VMS first calls each *exit handler*
// the program declared ($DCLEXH), then ends the image, returning to the
// command interpreter; $EXIT never returns to its caller. In govax the
// RTL keeps the handler lists and decides what happens next. It asks the
// engine for the two things only the engine can do:
//
//   - Call a handler: SystemService returns a *ServiceCall, and
//     callForService builds the call frame, exactly as CALLG would, with
//     the XFC as the return address. The handler's RET lands back on
//     $EXIT's XFC, which calls the service again for the next handler.
//   - End the image: SystemService returns ErrImageExit, and exitImage
//     unwinds the stack to the call frame the console built to start
//     the program (Engine.CallEntry), then returns from it, as if the
//     program's main routine had returned. The console sees
//     ErrConsoleCallReturned, the usual end of a RUN, and runs image
//     rundown.

// maxUnwindFrames bounds exitImage's walk up the frame chain, so a
// corrupted chain that loops back on itself stops the machine instead
// of hanging it.
const maxUnwindFrames = 100_000

// callForService calls call.Routine with the argument list call.ArgList,
// as the instruction CALLG call.ArgList, call.Routine would if it were
// the XFC now executing: the saved return address is the XFC itself
// (instructionPC), so the routine's RET executes the XFC again.
//
// A fault building the frame (an unreadable entry mask) is returned for
// Step to raise at the XFC.
func (e *Engine) callForService(call *ServiceCall) error {
	// CALLG's own steps (emulCall): remember SP, align it to a longword,
	// and build the frame. calls=false: RET won't pop the argument list,
	// which the caller owns.
	savedSP := e.cpu.GPR(vax.SP)
	e.cpu.SetGPR(vax.SP, savedSP&^3)

	return e.buildCallFrame(call.ArgList, call.Routine, savedSP, e.instructionPC, e.cpu.GPR(vax.FP), false)
}

// exitImage ends the running program: it follows the chain of call
// frames from FP (each frame's saved FP, at FP+12, is its caller's
// frame) to the one Engine.CallEntry built — the frame whose saved PC and
// FP are both SentinelReturn — and executes RET from it. That discards
// every frame in between, restores the registers the console's call
// saved, and reports ErrConsoleCallReturned, just as if the program's
// outermost procedure had returned. R0, the exit status, is untouched.
//
// Frame layout, from FP up: condition handler (+0), entry mask and saved
// PSW (+4), saved AP (+8), saved FP (+12), saved PC (+16).
//
// If there's no such frame — the program was started with START rather
// than by a console CALL or RUN, or the chain is unreadable — there's
// nowhere to return to, so the machine halts (ErrHalted), leaving R0 as
// the exit status.
func (e *Engine) exitImage() error {
	fp := e.cpu.GPR(vax.FP)

	for n := 0; n < maxUnwindFrames && fp != 0; n++ {
		savedFP, err := e.mem.LoadLongword(e.cpu, fp+12)
		if err != nil {
			break
		}

		savedPC, err := e.mem.LoadLongword(e.cpu, fp+16)
		if err != nil {
			break
		}

		if savedFP == SentinelReturn && savedPC == SentinelReturn {
			e.cpu.SetGPR(vax.FP, fp)

			return emulRet(e, nil)
		}

		fp = savedFP
	}

	return ErrHalted
}
