package debugger

import (
	"strings"

	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vmserrors"
)

// StepMode selects which of STEP's three behaviors a STEP (or SET STEP)
// command uses, matching vax.h's STEP_INSTRUCTION/STEP_OVER/STEP_RETURN
// constants — see docs/PHASE-18.md.
type StepMode int

const (
	// StepInto single-steps exactly one instruction, always tracing it
	// regardless of Console.Trace — STEP/INTO (also /IN, /INSTRUCTION),
	// and initialization.c's own startup default for Console.StepMode
	// (the zero value here matches that default).
	StepInto StepMode = iota
	// StepOver behaves like StepInto for an ordinary instruction, but for
	// a "call-like" one (see stepOverInstructions) lets the called
	// routine run to completion — silently, regardless of Console.Trace —
	// and stops once control returns to the instruction after it.
	StepOver
	// StepReturn runs (respecting Console.Trace throughout, unlike
	// StepOver's silent continuation) until the current procedure's
	// CALLS/CALLG frame returns to its caller.
	StepReturn
)

// String matches show_step's own ps assignment (console_show.c:765):
// "INTO" for anything other than OVER/RETURN.
func (m StepMode) String() string {
	switch m {
	case StepOver:
		return "OVER"
	case StepReturn:
		return "RETURN"
	default:
		return "INTO"
	}
}

// stepOverInstructions matches break_over_instruction's break_list
// (vax.c): the procedure-call/subroutine-call/change-mode instructions
// STEP/OVER treats as candidates to run to completion rather than step
// into, classified by Instruction.Name rather than raw opcode value.
var stepOverInstructions = map[string]bool{
	"BSBB": true, "JSB": true, "BSBW": true,
	"CHMK": true, "CHME": true, "CHMS": true, "CHMU": true,
	"CALLG": true, "CALLS": true,
}

// parseStepModeWord classifies a STEP-mode keyword — OVER, INTO/IN/
// INSTRUCTION, or RETURN, an optional leading '/' ignored — by an
// unambiguous prefix of its first three letters. console_set.c/
// console_step.c instead compare a literal, sometimes inconsistent list of
// CHAR4 4-character codes (e.g. bare "OVE" alone isn't one of the accepted
// spellings, only the full "OVER" or "/OVE" are); this is a console
// input-parsing convenience with no ISA-fidelity stakes; see
// docs/PHASE-18.md.
func parseStepModeWord(word string) (StepMode, bool) {
	w := strings.ToUpper(strings.TrimPrefix(strings.TrimSpace(word), "/"))

	switch {
	case w == "IN" || strings.HasPrefix(w, "INT") || strings.HasPrefix(w, "INS"):
		return StepInto, true
	case strings.HasPrefix(w, "OVE"):
		return StepOver, true
	case strings.HasPrefix(w, "RET"):
		return StepReturn, true
	default:
		return 0, false
	}
}

// SetStepMode implements SET STEP <OVER|INTO|IN|INSTRUCTION|RETURN>
// (console_set.c:450-486), matching console_step.c's own reading of
// Console.StepMode as STEP's default mode when no explicit qualifier is
// given.
func (d *Debugger) SetStepMode(word string) error {
	mode, ok := parseStepModeWord(word)
	if !ok {
		return vmserrors.New(vmserrors.CLI_BADQUALIFIER, word)
	}

	d.StepMode = mode

	return nil
}

// ShowStepMode implements SHOW STEP_MODE (show_step, console_show.c:765).
func (d *Debugger) ShowStepMode() error {
	d.Console.Printf("Default is STEP/%s\n", d.StepMode)

	return nil
}

// returnAddress reads the saved-PC slot of the call frame at the current
// FP, matching get_return (console_show.c) — the same frame layout
// internal/cpu/call.go's emulRet and show.go's ShowCallFrames already
// decode (handler @FP+0, mask @FP+4, saved AP @FP+8, saved FP @FP+12, saved
// PC @FP+16). Unlike get_return, this never arms a breakpoint before
// checking for an error — see stepReturn's own doc comment for why.
func (d *Debugger) returnAddress() (uint32, error) {
	fp := d.Console.CPU.GPR(vax.FP)
	ap := d.Console.CPU.GPR(vax.AP)

	if fp == 0 || ap == 0 {
		return 0, vmserrors.New(vmserrors.CLI_NOFRAMES)
	}

	return d.Console.Mem.LoadLongword(d.Console.CPU, fp+16)
}

// setStepBreakpoint installs a one-shot address breakpoint used internally
// by STEP/OVER and STEP/RETURN to resume the console once execution returns
// to addr — the Go equivalent of vax.c's own set_break(pc, 0,
// BREAK_ADDRESS|BREAK_STEP|BREAK_TEMPORARY) calls in its STEP_OVER/
// STEP_RETURN handling. It returns the breakpoint, for the STEP to remove
// once its run stops for any reason (endStep).
func (d *Debugger) setStepBreakpoint(addr uint32) *Breakpoint {
	bp := &Breakpoint{Kind: BreakAddress, Addr: addr, Temporary: true, Step: true}
	d.Breakpoints = append(d.Breakpoints, bp)

	return bp
}

// endStep removes a STEP's one-shot breakpoint when the STEP's run stops.
// When the run reached it, runLoop has already removed it and this does
// nothing; when the run stopped first somewhere else (a user breakpoint,
// a fault breakpoint, Ctrl-C, a HALT, an unhandled fault), the STEP is
// over, and its breakpoint would otherwise stay set and stop a later GO
// with a stray "Stepped to" (docs/PHASE-42.md, bug 2). The same goes for
// one that shares its address with a user breakpoint, which breakpointAt
// finds first, so runLoop never removes it (bug 3).
//
// VMS's debugger kept a STEP/RETURN pending across an exception break in
// the Phase 42 probe (testdata/dbgcmd/vax/step.dlg), and govax will copy
// that: docs/PHASE-42.md's subtask 7 makes STEP/RETURN wait for its
// frame's RET across other stops, and this then applies to STEP/OVER
// only.
func (d *Debugger) endStep(bp *Breakpoint) {
	d.removeBreakpointPtr(bp)
}

// stepRun implements STEP: starting at the current PC (or startAddr, if
// non-nil), single-steps one instruction (StepInto), runs a called routine
// to completion (StepOver), or runs until the current procedure returns
// (StepReturn) — matching console_step.c. mode is the qualifier the STEP
// command itself parsed (or, for a bare STEP with none, Debugger.StepMode).
func (d *Debugger) stepRun(startAddr *uint32, mode StepMode) (runOutcome, error) {
	c := d.Console

	if startAddr != nil {
		c.CPU.SetGPR(vax.PC, *startAddr)
	} else if err := d.requireProgram(); err != nil {
		return runEnded, err
	}

	c.Engine.BeginRun()

	switch mode {
	case StepOver:
		return d.stepOver()
	case StepReturn:
		return d.stepReturn()
	default:
		return d.stepInto()
	}
}

// stepInto executes exactly one instruction, always traced regardless of
// Console.Trace (console_step.c's own "Always in trace mode"), and always
// reports where it landed.
//
// The default is USER-mode stepping, so we just keep running the CPU loop
// if we get out of USER mode (for example, handling an interrupt). Use the
// SET DEBUG command to turn on USER mode stepping if you need to step into
// KERNEL mode, etc.
func (d *Debugger) stepInto() (runOutcome, error) {
	if outcome, done, err := d.stepOne(); done {
		return outcome, err
	}

	d.Console.Printf("Stepped to %s\n", d.Console.LocationText(d.Console.CPU.GPR(vax.PC)))

	return runStopped, nil
}

// stepOne executes the one instruction at the current PC, traced. It is
// the first half of both STEP/INTO and STEP/OVER. done is true when the
// run ended in that instruction (a HALT, a fault, ...), in which case
// outcome and err are the run's result and the caller returns them as they
// are.
func (d *Debugger) stepOne() (outcome runOutcome, done bool, err error) {
	c := d.Console
	finish := c.TraceStep(c.CPU.GPR(vax.PC), true)
	userStep := c.Engine.CPU().DebugEnabled(vax.DebugUserStep)

	for {
		if stepErr := c.Engine.Step(); stepErr != nil {
			outcome, err = d.reportStop(stepErr)

			return outcome, true, err
		}

		// If USER-mode only stepping is enabled and we are not in USER
		// mode, just keep running. This allows a STEP operation to trace
		// just USER mode code and ignores timer interrupts, etc.
		if userStep && c.Engine.CPU().PSL().CurMod() < 0b11 {
			continue
		}

		break
	}

	finish()

	// A condition nobody handled pauses the program at the instruction
	// that raised it.
	if d.unhandledBreak() {
		return runStopped, true, nil
	}

	return runStopped, false, nil
}

// stepOver executes the first instruction unconditionally traced (matching
// vax.c's own forced disasm=2 for STEP_OVER's first instruction, regardless
// of Console.Trace), then classifies it: an ordinary instruction stops
// there exactly like StepInto; a call-like one (stepOverInstructions) has
// this port let the call actually run — its Handler already computed the
// correct resume address as Engine.LastDecoded().NextPC, so unlike vax.c
// (which re-decodes the same instruction a second time to learn this,
// double-applying any autoincrement/autodecrement operand side effects —
// see docs/DEVIATIONS.md) this port never decodes the call twice — and then
// runs silently (regardless of Console.Trace, matching vax.c's own
// silencing of the stepped-over subroutine) until control returns there.
func (d *Debugger) stepOver() (runOutcome, error) {
	if outcome, done, err := d.stepOne(); done {
		return outcome, err
	}

	c := d.Console
	dec := c.Engine.LastDecoded()

	if !stepOverInstructions[dec.Instruction.Name] {
		c.Printf("Stepped to %s\n", c.LocationText(c.CPU.GPR(vax.PC)))

		return runStopped, nil
	}

	defer d.endStep(d.setStepBreakpoint(dec.NextPC))

	return d.runLoop(false, func(uint32) func() { return func() {} })
}

// stepReturn arms a one-shot breakpoint at the current procedure's return
// address before running anything, then runs exactly like GO (tracing
// per Console.Trace throughout, matching vax.c's own STEP_RETURN — unlike
// STEP_OVER, no local disasm silencing applies to it). Unlike get_return's
// own C caller (vax.c:536-541, which calls set_break with whatever *Addr
// happens to hold — 0 on failure — before checking the error), no
// breakpoint is armed at all if returnAddress fails: a clear, obvious logic
// slip (leaving a spurious breakpoint at address 0 behind on error), not an
// ISA-fidelity question, so fixed directly per CLAUDE.md's bug-fixing
// policy.
func (d *Debugger) stepReturn() (runOutcome, error) {
	addr, err := d.returnAddress()
	if err != nil {
		return runEnded, err
	}

	defer d.endStep(d.setStepBreakpoint(addr))

	c := d.Console

	return d.runLoop(true, func(pc uint32) func() { return c.TraceStep(pc, false) })
}
