package cpu

import "github.com/tucats/govax/internal/vax"

// AST delivery: the engine's half (docs/PHASE-26.md subtask 15).
//
// # What an AST is
//
// An AST ("asynchronous system trap") is VMS's software interrupt for
// a process. A program hands a system service the address of a
// procedure (the AST routine: $SETIMR's astadr, $DCLAST's, ...), and when
// the event happens — the timer expires, the I/O completes — VMS
// *interrupts* whatever the program is doing, calls that routine as if
// the program had executed CALLG at that point, and when the routine
// returns, resumes the program exactly where it was, registers and
// condition codes intact. The routine gets five arguments:
//
//	4(AP)   astprm, the parameter given with the request
//	8(AP)   R0 at the moment of interruption
//	12(AP)  R1
//	16(AP)  the interrupted PC
//	20(AP)  the interrupted PSL
//
// # Who does what
//
// On a real VAX, AST delivery belongs to the VMS executive, not the
// hardware. govax's executive is the Go RTL (internal/rtl), so the RTL
// owns the AST queues and decides *whether* an AST can be delivered now
// and *which* one. What it can't do is build a procedure-call frame: that
// is CALLG's job, and CALLG lives here, in the engine (buildCallFrame).
// So the work is split:
//
//  1. Before every instruction, Step asks the RTL (through ASTSource)
//     whether an AST is due.
//  2. If one is, the RTL has already pushed the five-argument list above
//     (plus its count, 5) on the current stack, and answers with the
//     routine's address, the argument list's address, and a return
//     address. If the AST belongs to a more privileged mode than the
//     CPU's (a kernel AST interrupting user code), the RTL has also
//     switched the CPU into that mode and onto its stack first, so the
//     list is on that stack and the routine runs in that mode.
//  3. deliverAST then does exactly what CALLG argList, routine would:
//     builds the call frame (saving the registers the routine's entry
//     mask names) and jumps to the routine, with the given return
//     address as the saved PC.
//  4. The return address is an XFC instruction in the P1 vector (the
//     SYS$CLRAST entry). When the AST routine executes RET, it "returns"
//     there, the XFC calls back into the RTL, and the RTL pops the
//     argument list, restoring R0, R1, PC, and PSL from it. The program
//     continues as if nothing had happened.
//
// The cpu package never learns the argument list's layout, and the RTL
// never touches a call frame: each side owns what it already owned.

// ASTCall describes one AST the RTL wants delivered: call Routine (the
// address of its entry mask) with the argument list at ArgList, as CALLG
// would, returning to ReturnPC.
type ASTCall struct {
	Routine  uint32
	ArgList  uint32
	ReturnPC uint32
}

// ASTSource is the optional half of SystemServices that delivers ASTs.
// SetSystemServices checks for it, so an implementation without ASTs
// (a test double) needn't provide it.
type ASTSource interface {
	// NextAST is called at every instruction boundary. It reports
	// ok=false when no AST can be delivered now (the usual case, and it
	// must be cheap). When one can, it has already pushed that AST's
	// argument list on the current stack and removed it from its queue.
	// An error stops the engine (Step returns it).
	NextAST() (call ASTCall, ok bool, err error)
}

// deliverAST gives an AST routine control, if the RTL has one due: the
// equivalent of the program executing CALLG call.ArgList, call.Routine
// at this instruction boundary, with call.ReturnPC standing in for the
// address after the CALLG. The next instruction Step decodes is then the
// routine's first.
//
// A fault building the frame — the routine address unreadable, say —
// is returned as a Fault for Step to raise, the way VMS reports an
// invalid AST routine: as an access violation when it's given control.
// So the fault's PC is the routine's address (instructionPC, which
// raise reports), and the AST counts as delivered: its argument list
// stays on the stack under the exception, as it would on VMS.
func (e *Engine) deliverAST() error {
	call, ok, err := e.astSource.NextAST()
	if err != nil || !ok {
		return err
	}

	e.instructionPC = call.Routine

	// CALLG's own steps (emulCall): remember SP as it is, align it to a
	// longword, and build the frame. calls=false: RET won't pop the
	// argument list, which the RTL's AST exit reads and removes itself.
	savedSP := e.cpu.GPR(vax.SP)
	e.cpu.SetGPR(vax.SP, savedSP&^3)

	return e.buildCallFrame(call.ArgList, call.Routine, savedSP, call.ReturnPC, e.cpu.GPR(vax.FP), false)
}
