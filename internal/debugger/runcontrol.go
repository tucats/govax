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

// BreakKind says what makes a Breakpoint stop the program. An address
// breakpoint stops *at a place*: before the instruction at one address
// runs. The others stop at a *kind of instruction*, wherever it is (every
// call, every branch, the first instruction of every source line, and so
// on): the VMS debugger's SET BREAK/CALL, /BRANCH, /LINE, /INSTRUCTION,
// and /RETURN. All of them live in one list, Debugger.Breakpoints, so
// SHOW BREAK and CANCEL BREAK see them together.
//
// Two kinds of stop have their own mechanisms outside the list, for
// reasons that predate it: fault breakpoints (SET BREAK/FAULT, a stop when
// a given exception code is about to be delivered) live on cpu.Engine,
// because Engine.Step checks them inside its own fault delivery
// (cpu/faultbreak.go); and the older opcode flags the console's SET
// BREAKPOINT/INSTRUCTION sets are Debugger.InstructionBreakpoints
// (instbreak.go).
type BreakKind int

const (
	// BreakAddress stops before the instruction at Addr.
	BreakAddress BreakKind = iota

	// BreakCall stops before any instruction that transfers control to a
	// routine or back from one: BSBB, BSBW, CALLG, CALLS, JSB, RET, RSB.
	BreakCall

	// BreakBranch stops before any branch, jump, loop, or case
	// instruction.
	BreakBranch

	// BreakLine stops before the first instruction of each source line.
	BreakLine

	// BreakInstruction stops before any instruction in Ops.
	BreakInstruction

	// BreakAnyInstruction stops before every instruction.
	BreakAnyInstruction

	// BreakReturn stops before a RET inside the routine at Start.
	BreakReturn

	// BreakException stops when a condition is signaled, before the
	// program's handlers hear of it.
	BreakException
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
	Step      bool // hit message reads "Stepped to" instead of "break at"

	// Quiet stops the run without a message. It is the debugger's own
	// breakpoint at an image's first instruction (startImage), where VMS
	// shows nothing more than the start-up messages.
	Quiet bool

	// Routine is true for a breakpoint SET BREAK made at a routine's name:
	// Addr is just past the routine's entry mask, and the breakpoint is
	// shown "at routine NAME" rather than at an address. Name is that
	// routine's path name (DBGCMD\FACT), also used by BreakReturn.
	Routine bool
	Name    string

	// Ops are the instructions a BreakInstruction stops before, and
	// OpNames the mnemonics as typed, which SHOW BREAK lists.
	Ops     []*cpu.Instruction
	OpNames []string

	// Start and Size are the extent of the routine a BreakReturn watches:
	// it stops at a RET whose address is in that range.
	Start, Size uint32

	// After is SET BREAK/AFTER:n: the breakpoint is passed n-1 times
	// before it first stops, and stops every time after. Zero means no
	// count. hits is how many times it has been reached so far.
	After int
	hits  int

	// fired is set when this breakpoint was the one that stopped a run.
	// A STEP's own breakpoints use it to tell their stop from another's.
	fired bool

	// When is the text of a WHEN (condition) clause, parentheses and all,
	// as SHOW BREAK shows it; the breakpoint stops only when the
	// condition is true. Do is a DO (commands) clause, run each time the
	// breakpoint stops. Both are empty when the clause wasn't given.
	When string
	Do   string
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

// breakpointAt returns the address breakpoint at addr, if there is one.
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

	// Source lines are shown once per instruction while the loop runs
	// (showSource). A loop inside another (a condition handler's) leaves
	// the outer's setting as it found it.
	outer := d.shown.active
	d.shown.active = true

	defer func() { d.shown.active = outer }()

	// A watchpoint reports what changes while the program runs, not what
	// the user changed meanwhile (a DEPOSIT), so its baseline is the
	// memory as it is now.
	d.snapshotWatches()

	for {
		// A process switch due at this boundary happens first, so the
		// checks below see the process that will run the instruction.
		if err := c.BeginStep(); err != nil {
			return d.reportStop(err)
		}

		pc := c.CPU.GPR(vax.PC)
		d.shown.ok = false

		// The eventpoints are process 1's (Console.RunningProcessOne):
		// another process at the same address doesn't reach them.
		mine := c.RunningProcessOne()

		if !first && mine {
			d.traceHit(pc)

			if d.breakpointHit(pc) {
				return runStopped, nil
			}

			if d.instructionBreakpointHit() {
				c.Printf("Instruction break at %s\n", c.LocationText(pc))

				return runStopped, nil
			}
		}

		// A STEP/RETURN waiting for this RET fires before it runs, even on
		// the run's first instruction.
		if mine && d.returnDue() {
			return runStopped, nil
		}

		// The first-instruction exemption (resuming at a breakpoint) lasts
		// until process 1 runs an instruction: if another process runs
		// first, process 1 comes back to the same breakpoint unchecked.
		if mine {
			first = false
		}

		finish := trace(pc)

		if err := c.StepMachine(); err != nil {
			return d.reportStop(err)
		}

		finish()

		// A watched location the instruction changed stops the program
		// after it; so does a condition nobody handled, at the
		// instruction that raised it. Watchpoints are checked after
		// process 1's instructions only (a change another process makes
		// to a shared S0 location is seen after process 1's next one).
		if (c.RunningProcessOne() && d.watchHit(pc)) || d.signalBreak() || d.unhandledBreak() {
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
		d.Console.Printf("Break on fault %02X %s at PC = %08X%s\n",
			uint8(fb.Code), d.Console.ExceptionName(fb.Code), d.Console.CPU.GPR(vax.PC), d.Console.ProcessNote())

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
		d.interrupted = true

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
		// One instruction, into the routine, as STEP/INSTRUCTION/INTO would.
		outcome, _, err := d.stepUnit(stepRequest{mode: StepInto, byInstruction: true, count: 1})

		return outcome, err
	}

	return d.runLoop(false, func(pc uint32) func() { return c.TraceStep(pc, false) })
}
