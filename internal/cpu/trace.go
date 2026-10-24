package cpu

// The trace fault (VAX Architecture Reference Manual, chapter 5, "Trace
// Fault", and the between-instructions sequence under "Serialization of
// Notification of Multiple Events"; docs/PHASE-73 - trace trap.md).
//
// # What tracing is
//
// A program (or a debugger) that wants control after every instruction
// sets PSL<T>, the trace enable bit: with BISPSW #^X10, in the PSL an REI
// will restore, or in a call frame's saved PSW that a RET will restore.
// From then on, an exception, the trace fault, happens between each
// instruction and the next. Its saved PC is the next instruction's, so the
// handler sees where the program has got to, and its REI goes on from
// there. VMS signals it to the program's condition handlers as SS$_TBIT.
//
// # Why two bits
//
// "Exactly one trace per instruction" is harder than it sounds: an
// instruction may fault (and run again later), take an arithmetic trap
// (whose handler runs before the trace), or be followed by an interrupt.
// So the trace isn't generated from T directly. At the start of each
// instruction the processor copies T into a second bit, PSL<TP>, trace
// pending, and it's TP that makes the fault, at the start of the *next*
// instruction:
//
//  1. If TP is set, clear it and take the trace fault (before anything
//     else: a reserved instruction, FPD, ...).
//  2. Otherwise TP ← T, and the instruction runs.
//  3. If the instruction faults, TP is cleared before the PSL is saved:
//     the instruction runs again after the fault's handler, and is
//     traced then (Engine.raise).
//  4. If it takes a trap (an arithmetic trap, CHMx), TP is left as it is
//     in the saved PSL, so the trace comes when the trap's handler REIs.
//  5. An interrupt taken between instructions saves the PSL with TP as
//     it is, for the same reason.
//
// Every exception and interrupt starts its handler with T and TP clear
// (Engine.HandleFault), so handlers aren't traced. REI makes TP set if T
// was set when the REI ran or TP is set in the PSL it restores
// (emulRei). CALLS and CALLG save T clear in the frame and leave PSL<T>
// alone; RET restores T from the frame (call.go), so a traced routine's
// RET ends tracing unless the trace handler set T in the frame. Setting
// T with BISPSW makes the first trace come after the instruction after
// the BISPSW, which steps 1 and 2 give by themselves.

// beginTrace is steps 1 and 2 above, run by Step just before an
// instruction is decoded: it returns the trace fault if one is pending,
// clearing TP; otherwise it copies T into TP and returns nil. The fault's
// saved PC is the instruction about to run (Step has set instructionPC).
func (e *Engine) beginTrace() *Fault {
	psl := e.cpu.PSL()

	if psl.TP() {
		psl.SetTP(false)
		e.cpu.SetPSL(psl)

		return &Fault{Code: ExcTracePending}
	}

	if psl.T() {
		psl.SetTP(true)
		e.cpu.SetPSL(psl)
	}

	return nil
}

// clearTracePending clears PSL<TP>, as a fault does before its PSL is
// saved (step 3 above).
func (e *Engine) clearTracePending() {
	psl := e.cpu.PSL()
	if psl.TP() {
		psl.SetTP(false)
		e.cpu.SetPSL(psl)
	}
}
