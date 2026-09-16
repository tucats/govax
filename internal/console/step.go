package console

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

// parseStepQualifier reads STEP's own optional leading qualifier —
// /OVER, /INTO, /IN, /INSTRUCTION, or /RETURN — matching console_step.c's
// narrower, slash-only acceptance (unlike SET STEP, which also takes a bare
// word; see cmdSet's own "STEP" case). An unrecognized (or absent) leading
// "/word" is left in place and def is returned, matching console_step's own
// "reset parse pointer, use default mode" fallback — the caller then tries
// to parse the same text as a starting address, exactly as the C source
// falls through to asm_hex on the same unconsumed text.
func parseStepQualifier(rest string, def StepMode) (StepMode, string) {
	trimmed := strings.TrimLeft(rest, " \t")
	if !strings.HasPrefix(trimmed, "/") {
		return def, rest
	}

	i := 1
	for i < len(trimmed) && trimmed[i] != ' ' && trimmed[i] != '\t' {
		i++
	}

	if mode, ok := parseStepModeWord(trimmed[1:i]); ok {
		return mode, trimmed[i:]
	}

	return def, rest
}

// SetStepMode implements SET STEP <OVER|INTO|IN|INSTRUCTION|RETURN>
// (console_set.c:450-486), matching console_step.c's own reading of
// Console.StepMode as STEP's default mode when no explicit qualifier is
// given.
func (c *Console) SetStepMode(word string) error {
	mode, ok := parseStepModeWord(word)
	if !ok {
		return vmserrors.New(vmserrors.CLI_BADQUALIFIER, word)
	}

	c.StepMode = mode

	return nil
}

// ShowStepMode implements SHOW STEP_MODE (show_step, console_show.c:765).
func (c *Console) ShowStepMode() error {
	c.Printf("Default is STEP/%s\n", c.StepMode)

	return nil
}

// returnAddress reads the saved-PC slot of the call frame at the current
// FP, matching get_return (console_show.c) — the same frame layout
// internal/cpu/call.go's emulRet and show.go's ShowCallFrames already
// decode (handler @FP+0, mask @FP+4, saved AP @FP+8, saved FP @FP+12, saved
// PC @FP+16). Unlike get_return, this never arms a breakpoint before
// checking for an error — see stepReturn's own doc comment for why.
func (c *Console) returnAddress() (uint32, error) {
	fp := c.CPU.GPR(vax.FP)
	ap := c.CPU.GPR(vax.AP)

	if fp == 0 || ap == 0 {
		return 0, vmserrors.New(vmserrors.CLI_NOFRAMES)
	}

	return c.Mem.LoadLongword(c.CPU, fp+16)
}

// setStepBreakpoint installs a one-shot address breakpoint used internally
// by STEP/OVER and STEP/RETURN to resume the console once execution returns
// to addr — the Go equivalent of vax.c's own set_break(pc, 0,
// BREAK_ADDRESS|BREAK_STEP|BREAK_TEMPORARY) calls in its STEP_OVER/
// STEP_RETURN handling.
func (c *Console) setStepBreakpoint(addr uint32) {
	c.Breakpoints = append(c.Breakpoints, &Breakpoint{Kind: BreakAddress, Addr: addr, Temporary: true, Step: true})
}

// Step implements STEP: starting at the current PC (or startAddr, if
// non-nil), single-steps one instruction (StepInto), runs a called routine
// to completion (StepOver), or runs until the current procedure returns
// (StepReturn) — matching console_step.c. mode is the qualifier the STEP
// command itself parsed (or, for a bare STEP with none, Console.StepMode --
// see cmdStep).
func (c *Console) Step(startAddr *uint32, mode StepMode) error {
	if err := c.requireInit(); err != nil {
		return err
	}

	if startAddr != nil {
		c.CPU.SetGPR(vax.PC, *startAddr)
	}

	c.Engine.BeginRun()

	switch mode {
	case StepOver:
		return c.stepOver()
	case StepReturn:
		return c.stepReturn()
	default:
		return c.stepInto()
	}
}

// stepInto executes exactly one instruction, always traced regardless of
// Console.Trace (console_step.c's own "Always in trace mode"), and always
// reports where it landed.
func (c *Console) stepInto() error {
	finish := c.traceStep(c.CPU.GPR(vax.PC), true)
	if err := c.Engine.Step(); err != nil {
		return c.reportStopReason(err)
	}
	finish()

	c.Printf("Stepped to %08X\n", c.CPU.GPR(vax.PC))

	return nil
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
func (c *Console) stepOver() error {
	finish := c.traceStep(c.CPU.GPR(vax.PC), true)
	if err := c.Engine.Step(); err != nil {
		return c.reportStopReason(err)
	}
	finish()

	dec := c.Engine.LastDecoded()
	if !stepOverInstructions[dec.Instruction.Name] {
		c.Printf("Stepped to %08X\n", c.CPU.GPR(vax.PC))

		return nil
	}

	c.setStepBreakpoint(dec.NextPC)

	return c.runLoop(false, func(uint32) func() { return func() {} })
}

// stepReturn arms a one-shot breakpoint at the current procedure's return
// address before running anything, then runs exactly like Execute (tracing
// per Console.Trace throughout, matching vax.c's own STEP_RETURN — unlike
// STEP_OVER, no local disasm silencing applies to it). Unlike get_return's
// own C caller (vax.c:536-541, which calls set_break with whatever *Addr
// happens to hold — 0 on failure — before checking the error), no
// breakpoint is armed at all if returnAddress fails: a clear, obvious logic
// slip (leaving a spurious breakpoint at address 0 behind on error), not an
// ISA-fidelity question, so fixed directly per CLAUDE.md's bug-fixing
// policy.
func (c *Console) stepReturn() error {
	addr, err := c.returnAddress()
	if err != nil {
		return err
	}

	c.setStepBreakpoint(addr)

	return c.runLoop(true, func(pc uint32) func() { return c.traceStep(pc, false) })
}
