package debugger

import (
	"errors"

	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/vax"
)

// This file is the debugger's run control: the list of address
// breakpoints, the loop that drives the CPU one instruction at a time while
// checking them, and the GO, CALL, and STEP entry points that use it
// (docs/PHASE-42.md, subtask 4). Until that subtask this lived in
// internal/console; the console now hands every run to the debugger
// (Console.Execute, Console.Call, and Console.Step forward here through the
// console.Debugger interface).
//
// A few VAX/VMS terms for readers new to them:
//
//   - A *breakpoint* is a place (here, an address) where the debugger
//     stops the program before the instruction there runs, so the user can
//     look around.
//   - To *step* is to run just one instruction (or one "line", or one
//     routine call) and stop again.
//   - A *call frame* is the block of stack memory a VAX CALLS or CALLG
//     instruction builds for a routine: its saved registers, its caller's
//     frame pointer (FP) and argument pointer (AP), and the return
//     address.

// BreakKind distinguishes the kinds of breakpoint vax.h's struct BREAKSTR
// supports that actually live in a breakpoint_list-like collection.
// BreakAddress is the only kind represented here: fault-kind breakpoints
// (a breakpoint that fires when a given exception code is about to be
// delivered, C's BREAK_FAULT) are implemented — see SET BREAKPOINT/FAULT,
// docs/PHASE-16.md — but deliberately live on cpu.Engine instead of growing
// this type, since they're checked synchronously inside Engine.Step's own
// fault-delivery path (see cpu/faultbreak.go's own doc comment for why that
// rules out Debugger.Breakpoints, which runLoop only ever consults between
// Step calls). Instruction (opcode) breakpoints are a third BREAKSTR-
// adjacent kind the C source itself never stores in breakpoint_list at all
// (it flags the opcode directly via instruction[n].debugdata instead — see
// console_set.c's own BREAK_INSTRUCTION handling), so this port follows
// suit with an entirely separate mechanism (Debugger.InstructionBreakpoints,
// instbreak.go) rather than growing BreakKind to include it either.
type BreakKind int

const (
	BreakAddress BreakKind = iota
)

// Breakpoint is one entry in Debugger.Breakpoints. Temporary and Step back
// the internal one-shot breakpoints STEP/OVER and STEP/RETURN arm to resume
// the debugger at the right place (docs/PHASE-18.md) — the Go equivalent of
// vax.c's own set_break(pc, 0, BREAK_ADDRESS|BREAK_STEP|BREAK_TEMPORARY)
// calls, sharing this same list with user-set breakpoints exactly as the C
// source's single breakpoint_list does. A user breakpoint (AddBreakpoint)
// leaves both false.
type Breakpoint struct {
	Kind      BreakKind
	Addr      uint32
	Temporary bool // removed the moment it fires
	Step      bool // hit message reads "Stepped to" instead of "Break at"

	// Quiet stops the run without a message. It is the debugger's own
	// breakpoint at an image's first instruction (startImage), where VMS
	// shows nothing more than the start-up messages.
	Quiet bool
}

// AddBreakpoint sets an address breakpoint, matching SET BREAKPOINT (see
// console_set.c). A duplicate address is a no-op.
func (d *Debugger) AddBreakpoint(addr uint32) {
	if d.breakpointAt(addr) != nil {
		return
	}

	d.Breakpoints = append(d.Breakpoints, &Breakpoint{Kind: BreakAddress, Addr: addr})
}

// AddTemporaryBreakpoint sets a one-shot address breakpoint, matching SET
// BREAKPOINT/TEMPORARY (console_set.c's BREAK_ADDRESS|BREAK_TEMPORARY
// case) — removed the moment it's hit (runLoop's own Breakpoint.Temporary
// handling, shared with STEP/OVER's and STEP/RETURN's internal one-shot
// breakpoints). A duplicate address is a no-op, matching AddBreakpoint.
func (d *Debugger) AddTemporaryBreakpoint(addr uint32) {
	if d.breakpointAt(addr) != nil {
		return
	}

	d.Breakpoints = append(d.Breakpoints, &Breakpoint{Kind: BreakAddress, Addr: addr, Temporary: true})
}

// RemoveBreakpoint clears one address breakpoint, matching CLEAR
// BREAKPOINT <address>.
func (d *Debugger) RemoveBreakpoint(addr uint32) {
	for i, bp := range d.Breakpoints {
		if bp.Kind == BreakAddress && bp.Addr == addr {
			d.Breakpoints = append(d.Breakpoints[:i], d.Breakpoints[i+1:]...)

			return
		}
	}
}

// ClearAllBreakpoints removes every breakpoint, matching CLEAR
// BREAKPOINT/ALL.
func (d *Debugger) ClearAllBreakpoints() {
	d.Breakpoints = nil
}

// ClearBreakpoint implements CLEAR BREAKPOINT: a specific address, or
// every breakpoint (CLEAR BREAKPOINT/ALL). The /FAULT and /INSTRUCTION
// sub-forms are separate commands (faultbreak.go, instbreak.go).
func (d *Debugger) ClearBreakpoint(addr uint32, all bool) error {
	if err := d.Console.RequireInit(); err != nil {
		return err
	}

	if all {
		d.ClearAllBreakpoints()

		return nil
	}

	d.RemoveBreakpoint(addr)

	return nil
}

func (d *Debugger) breakpointAt(addr uint32) *Breakpoint {
	for _, bp := range d.Breakpoints {
		if bp.Kind == BreakAddress && bp.Addr == addr {
			return bp
		}
	}

	return nil
}

// removeBreakpointPtr removes target by identity rather than by address, so
// runLoop can clear the exact one-shot breakpoint it just hit even if a
// permanent user breakpoint happens to share its address. In that case
// breakpointAt returns the permanent one first (the C source's first-match
// scan of a single breakpoint_list), and the STEP that set the one-shot
// breakpoint removes it when its run stops (step.go's endStep). Removing
// one that is already gone does nothing.
func (d *Debugger) removeBreakpointPtr(target *Breakpoint) {
	for i, bp := range d.Breakpoints {
		if bp == target {
			d.Breakpoints = append(d.Breakpoints[:i], d.Breakpoints[i+1:]...)

			return
		}
	}
}

// runLoop drives Engine.Step in a loop, stopping when a breakpoint is hit or
// when Engine.Step itself ends the run (halt, attention, fault, an
// instruction/time limit — see reportStop) — shared by GO, CALL, and
// Step's STEP/OVER and STEP/RETURN modes, both of which resume ordinary
// execution until a one-shot internal breakpoint is reached. This mirrors
// vax.c's execute_vax: every one of these cases shares the same
// breakpoint_list/set_break/clear_break machinery in the C source, not
// separate mechanisms. Instruction breakpoints (instructionBreakpointHit,
// instbreak.go) are checked right alongside address breakpoints here too,
// even though the C source stores them in an entirely separate mechanism
// (instruction[n].debugdata, not breakpoint_list).
//
// skipFirstCheck matches vax.c's own initial_PC tracking: a breakpoint
// sitting exactly on the address this loop starts from must not fire
// immediately — it has to be reached again after at least one instruction
// runs. GO and STEP/RETURN pass true (their first PC is genuinely the
// command's own starting point); STEP/OVER's continuation phase (called
// after its first, call-like instruction has already executed) and CALL
// pass false, since their first PC is a callee's entry point, not the
// original command's starting address, and a breakpoint sitting there
// must fire immediately. This port applies the same flag to instruction
// breakpoints too, deliberately not replicating a C-source quirk where
// whether an instruction breakpoint fires on a run's very first
// instruction incidentally depends on whether the unrelated address/fault
// breakpoint_list happens to be non-empty; a debugger-tool quirk with no
// ISA-fidelity stakes.
//
// trace is called with each instruction's PC immediately before it
// executes and must return a finish func to call once it has executed —
// the same protocol as Console.TraceStep, whose result callers typically
// pass straight through (STEP/OVER's silent continuation passes a func
// that never traces at all, regardless of Console.Trace, matching vax.c's
// own STEP_OVER-only silencing of the stepped-over subroutine's
// instructions).
//
// The result says how the run ended. outcome is runStopped when the
// debugger stopped it (a breakpoint, Ctrl-C, a fault breakpoint) and runEnded
// when the program finished (HALT, the CALLed routine's return) or hit one
// of govax's own limits; a real error is returned as err.
func (d *Debugger) runLoop(skipFirstCheck bool, trace func(pc uint32) func()) (runOutcome, error) {
	c := d.Console
	first := skipFirstCheck

	for {
		pc := c.CPU.GPR(vax.PC)
		if !first {
			if bp := d.breakpointAt(pc); bp != nil {
				if bp.Temporary {
					d.removeBreakpointPtr(bp)
				}

				switch {
				case bp.Quiet:
				case bp.Step:
					c.Printf("Stepped to %s\n", c.LocationText(pc))
				default:
					c.Printf("Break at %s\n", c.LocationText(pc))
				}

				return runStopped, nil
			}

			if d.instructionBreakpointHit() {
				c.Printf("Instruction break at %s\n", c.LocationText(pc))

				return runStopped, nil
			}
		}

		first = false
		finish := trace(pc)

		if err := c.Engine.Step(); err != nil {
			return d.reportStop(err)
		}

		finish()

		// A condition nobody handled pauses the program at the
		// instruction that raised it.
		if d.unhandledBreak() {
			return runStopped, nil
		}
	}
}

// runOutcome says how a run ended, which decides whether the debugger
// session carries on (docs/PHASE-42.md, Decision 2).
type runOutcome int

const (
	// runEnded: the program finished by itself (a HALT, or the routine a
	// CALL started returned), or a govax limit (-instruction-limit,
	// -time-limit) ended it. A session that began with this run ends too.
	runEnded runOutcome = iota

	// runStopped: the debugger stopped the program (a breakpoint, a step
	// completing, Ctrl-C, a fault breakpoint). The "DBG> " prompt appears.
	runStopped
)

// reportStop handles every way Engine.Step can end a run. A fault
// breakpoint (SET BREAKPOINT/FAULT) is the debugger's: it carries the fault
// code it hit, and, as in vax.c's BREAK_FAULT check, execution stops with
// PC exactly where the fault was raised (no vector taken, no stack frame
// pushed; see cpu/faultbreak.go). Everything else is the console's to
// word (Console.ReportStop): a HALT, an instruction or time limit, a
// routine returning to the console, or a VMS condition (an exception
// the CHF searched handlers for). Ctrl-C (cpu.ErrAttention) is the one of
// those that is also a stop for the debugger, since the user interrupted
// the program to look at it.
func (d *Debugger) reportStop(err error) (runOutcome, error) {
	var fb *cpu.FaultBreak
	if errors.As(err, &fb) {
		d.Console.Printf("Break on fault %02X %s at PC = %08X\n",
			uint8(fb.Code), d.Console.ExceptionName(fb.Code), d.Console.CPU.GPR(vax.PC))

		return runStopped, nil
	}

	// The image exiting under the debugger isn't the end of the session:
	// the user may still look at the program's data.
	exited := d.imageExit(err)

	if err := d.Console.ReportStop(err); err != nil {
		return runEnded, err
	}

	if exited {
		return runStopped, nil
	}

	if errors.Is(err, cpu.ErrAttention) {
		return runStopped, nil
	}

	return runEnded, nil
}

// goRun runs the CPU starting at the current PC (or startAddr, if non-nil)
// until it halts, hits a breakpoint, or an unhandled fault stops it — the
// Go equivalent of console_exec.c's GO/EXEC command (console_exec), with
// breakpoint checking layered on top of Engine.Step exactly as
// docs/PHASE-03.md's design notes call for. A breakpoint at the address goRun
// started from is not treated as an immediate stop (it must be reached again
// after at least one instruction runs), matching vax.c's own initial_PC
// tracking.
func (d *Debugger) goRun(startAddr *uint32) (runOutcome, error) {
	c := d.Console

	if startAddr != nil {
		c.CPU.SetGPR(vax.PC, *startAddr)
	} else if err := d.requireProgram(); err != nil {
		return runEnded, err
	}

	c.Engine.BeginRun()

	return d.runLoop(true, func(pc uint32) func() { return c.TraceStep(pc, false) })
}

// callRun implements the CALL command's core mechanism (see
// docs/PHASE-13.md): invokes the procedure at addr with the given arguments
// (pushed right-to-left, matching a real CALLS instruction — see
// cpu.Engine.CallEntry), then either runs Engine.Step in a loop until it
// returns (cpu.CallEntry/ErrConsoleCallReturned) or halts (cpu.ErrHalted),
// or — if step is true — executes exactly the entered procedure's first
// instruction and returns, leaving the rest to be single-stepped with STEP.
// This matches console_exec.c's own console_call: on /STEP (or its
// /BREAK|/DEBUG synonyms) it doesn't run the code at all, it delegates
// straight to console_step for one instruction.
//
// The run goes through runLoop, as GO's does, so breakpoints stop it
// (docs/PHASE-42.md, bug 1). The first instruction is checked too
// (skipFirstCheck false): it's the called routine's, not a place the
// debugger was already stopped at, so a breakpoint there fires at once, as
// one does for STEP/OVER's callee.
func (d *Debugger) callRun(addr uint32, step bool, args []uint32) (runOutcome, error) {
	c := d.Console

	if err := c.Engine.CallEntry(addr, args...); err != nil {
		return runEnded, err
	}

	c.Engine.BeginRun()

	if step {
		return d.stepInto()
	}

	return d.runLoop(false, func(pc uint32) func() { return c.TraceStep(pc, false) })
}

// ShowBreakpoints prints every active breakpoint, matching SHOW
// BREAKPOINTS.
func (d *Debugger) ShowBreakpoints() error {
	if err := d.Console.RequireInit(); err != nil {
		return err
	}

	faults := d.Console.Engine.FaultBreakpoints()

	if len(d.Breakpoints) == 0 && len(faults) == 0 {
		d.Console.Printf("No breakpoints set\n")

		return nil
	}

	for _, bp := range d.Breakpoints {
		tag := ""

		switch {
		case bp.Step:
			tag = " <step>"

		case bp.Temporary:
			tag = " <temporary>"
		}

		d.Console.Printf("Breakpoint at %08X%s\n", bp.Addr, tag)
	}

	// Fault-kind breakpoints are a separate list from Debugger.Breakpoints
	// (see execute.go's BreakKind doc comment on why), but console_show.c's
	// own SHOW BREAK prints both kinds together in one listing -- matched
	// here by simply printing this second group right after the first,
	// each entry marked 'F' the way that C source's own print loop does.
	for _, code := range faults {
		d.Console.Printf("F Breakpoint on fault %02X %s\n", uint8(code), d.Console.ExceptionName(code))
	}

	return nil
}
