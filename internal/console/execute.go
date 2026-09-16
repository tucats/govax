package console

import (
	"errors"

	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/vax"
)

// BreakKind distinguishes the kinds of breakpoint vax.h's struct BREAKSTR
// supports that actually live in a breakpoint_list-like collection.
// BreakAddress is the only kind represented here: fault-kind breakpoints
// (a breakpoint that fires when a given exception code is about to be
// delivered, C's BREAK_FAULT) are implemented — see SET BREAKPOINT/FAULT,
// docs/PHASE-16.md — but deliberately live on cpu.Engine instead of growing
// this type, since they're checked synchronously inside Engine.Step's own
// fault-delivery path (see cpu/faultbreak.go's own doc comment for why that
// rules out Console.Breakpoints, which this file's runLoop only ever
// consults between Step calls). Instruction (opcode) breakpoints are a
// third BREAKSTR-adjacent kind the C source itself never stores in
// breakpoint_list at all (it flags the opcode directly via
// instruction[n].debugdata instead — see console_set.c's own
// BREAK_INSTRUCTION handling), so this port follows suit with an entirely
// separate mechanism (Console.InstructionBreakpoints, instbreak.go) rather
// than growing BreakKind to include it either.
type BreakKind int

const (
	BreakAddress BreakKind = iota
)

// Breakpoint is one entry in Console.Breakpoints. Temporary and Step back
// the internal one-shot breakpoints STEP/OVER and STEP/RETURN arm to resume
// the console at the right place (docs/PHASE-18.md) — the Go equivalent of
// vax.c's own set_break(pc, 0, BREAK_ADDRESS|BREAK_STEP|BREAK_TEMPORARY)
// calls, sharing this same list with user-set breakpoints exactly as the C
// source's single breakpoint_list does. A user breakpoint (AddBreakpoint)
// leaves both false.
type Breakpoint struct {
	Kind      BreakKind
	Addr      uint32
	Temporary bool // removed the moment it fires
	Step      bool // hit message reads "Stepped to" instead of "Break at"
}

// AddBreakpoint sets an address breakpoint, matching SET BREAKPOINT (see
// console_set.c). A duplicate address is a no-op.
func (c *Console) AddBreakpoint(addr uint32) {
	if c.breakpointAt(addr) != nil {
		return
	}

	c.Breakpoints = append(c.Breakpoints, &Breakpoint{Kind: BreakAddress, Addr: addr})
}

// AddTemporaryBreakpoint sets a one-shot address breakpoint, matching SET
// BREAKPOINT/TEMPORARY (console_set.c's BREAK_ADDRESS|BREAK_TEMPORARY
// case) — removed the moment it's hit (runLoop's own Breakpoint.Temporary
// handling, shared with STEP/OVER's and STEP/RETURN's internal one-shot
// breakpoints). A duplicate address is a no-op, matching AddBreakpoint.
func (c *Console) AddTemporaryBreakpoint(addr uint32) {
	if c.breakpointAt(addr) != nil {
		return
	}

	c.Breakpoints = append(c.Breakpoints, &Breakpoint{Kind: BreakAddress, Addr: addr, Temporary: true})
}

// RemoveBreakpoint clears one address breakpoint, matching CLEAR
// BREAKPOINT <address>.
func (c *Console) RemoveBreakpoint(addr uint32) {
	for i, bp := range c.Breakpoints {
		if bp.Kind == BreakAddress && bp.Addr == addr {
			c.Breakpoints = append(c.Breakpoints[:i], c.Breakpoints[i+1:]...)

			return
		}
	}
}

// ClearAllBreakpoints removes every breakpoint, matching CLEAR
// BREAKPOINT/ALL.
func (c *Console) ClearAllBreakpoints() {
	c.Breakpoints = nil
}

func (c *Console) breakpointAt(addr uint32) *Breakpoint {
	for _, bp := range c.Breakpoints {
		if bp.Kind == BreakAddress && bp.Addr == addr {
			return bp
		}
	}

	return nil
}

// removeBreakpointPtr removes target by identity rather than by address, so
// runLoop can clear the exact one-shot breakpoint it just hit even if a
// permanent user breakpoint happens to share its address (in which case
// breakpointAt returns the permanent one first and this is never reached
// for the temporary one — matching the C source's own first-match linear
// scan of a single breakpoint_list).
func (c *Console) removeBreakpointPtr(target *Breakpoint) {
	for i, bp := range c.Breakpoints {
		if bp == target {
			c.Breakpoints = append(c.Breakpoints[:i], c.Breakpoints[i+1:]...)

			return
		}
	}
}

// runLoop drives Engine.Step in a loop, stopping when a breakpoint is hit or
// when Engine.Step itself ends the run (halt, attention, fault, an
// instruction/time limit — see reportStopReason) — shared by Execute and by
// Step's STEP/OVER and STEP/RETURN modes, both of which resume ordinary
// execution until a one-shot internal breakpoint is reached. This mirrors
// vax.c's execute_vax: every one of these cases shares the same
// breakpoint_list/set_break/clear_break machinery in the C source, not
// separate mechanisms. Instruction breakpoints (instructionBreakpointHit,
// instbreak.go) are checked right alongside address breakpoints here too,
// even though the C source stores them in an entirely separate mechanism
// (instruction[n].debugdata, not breakpoint_list) — see instbreak.go's own
// doc comments on why that storage split doesn't need to carry through to
// this loop's stop condition as well.
//
// skipFirstCheck matches vax.c's own initial_PC tracking: a breakpoint
// sitting exactly on the address this loop starts from must not fire
// immediately — it has to be reached again after at least one instruction
// runs. Execute and STEP/RETURN pass true (their first PC is genuinely the
// command's own starting point); STEP/OVER's continuation phase (called
// after its first, call-like instruction has already executed) passes
// false, since that phase's first PC is the callee's entry point, not the
// original STEP command's starting address, and a breakpoint sitting there
// must fire immediately. This port applies the same skipFirstCheck flag to
// instruction breakpoints too, deliberately not replicating a C-source
// quirk where whether an instruction breakpoint fires on a run's very first
// instruction incidentally depends on whether the unrelated address/fault
// breakpoint_list happens to be non-empty (vax.c's initial_PC guard for
// BREAK_ADDRESS/BREAK_FAULT and the guard on decode_instruction's own
// OP_DBG_BREAK check share one variable, but the former is set from inside
// a block gated on `if (vax.console.breakpoint_list)`); a debugger-tool
// quirk with no ISA-fidelity stakes, not logged to docs/DEVIATIONS.md.
//
// trace is called with each instruction's PC immediately before it
// executes and must return a finish func to call once it has executed —
// the same protocol as traceStep, whose result callers typically pass
// straight through (Execute uses c.traceStep(pc, false); STEP/OVER's silent
// continuation passes a func that never traces at all, regardless of
// Console.Trace, matching vax.c's own STEP_OVER-only silencing of the
// stepped-over subroutine's instructions).
func (c *Console) runLoop(skipFirstCheck bool, trace func(pc uint32) func()) error {
	first := skipFirstCheck

	for {
		pc := c.CPU.GPR(vax.PC)
		if !first {
			if bp := c.breakpointAt(pc); bp != nil {
				if bp.Temporary {
					c.removeBreakpointPtr(bp)
				}

				if bp.Step {
					c.Printf("Stepped to %08X\n", pc)
				} else {
					c.Printf("Break at %08X\n", pc)
				}

				return nil
			}

			if c.instructionBreakpointHit() {
				c.Printf("Instruction break at %08X\n", pc)

				return nil
			}
		}

		first = false
		finish := trace(pc)

		if err := c.Engine.Step(); err != nil {
			return c.reportStopReason(err)
		}

		finish()
	}
}

// Execute runs the CPU starting at the current PC (or startAddr, if
// non-nil) until it halts, hits a breakpoint, or an unhandled fault stops
// it — the Go equivalent of console_exec.c's GO/EXEC command
// (console_exec), with breakpoint checking layered on top of Engine.Step
// exactly as docs/PHASE-03.md's design notes call for. A breakpoint at the
// address Execute started from is not treated as an immediate stop (it must
// be reached again after at least one instruction runs), matching vax.c's
// own initial_PC tracking.
func (c *Console) Execute(startAddr *uint32) error {
	if err := c.requireInit(); err != nil {
		return err
	}

	if startAddr != nil {
		c.CPU.SetGPR(vax.PC, *startAddr)
	}

	c.Engine.BeginRun()

	return c.runLoop(true, func(pc uint32) func() { return c.traceStep(pc, false) })
}

// reportStopReason handles every "the run stopped for a benign, expected
// reason" outcome Engine.Step can produce -- a HALT instruction, a Ctrl-C
// interrupt (see cpu.Engine.Attention), and this project's own
// -instruction-limit/-time-limit guards (docs/PHASE-15.md's sub-phase 2) --
// by printing a matching console message and returning nil, so
// Execute/Call/Step's own loops can just `return c.reportStopReason(err)`
// on any Step error. Any other error (an unhandled fault, a real Go error)
// is returned unchanged for the caller to propagate.
func (c *Console) reportStopReason(err error) error {
	// A fault-kind breakpoint (SET BREAKPOINT/FAULT) carries the fault code
	// it hit, so it's checked via errors.As rather than folded into the
	// errors.Is switch below — matching vax.c's own BREAK_FAULT check,
	// execution stops with PC exactly where the fault was raised (no
	// vector taken, no stack frame pushed; see cpu/faultbreak.go).
	var fb *cpu.FaultBreak
	if errors.As(err, &fb) {
		c.Printf("Break on fault %02X %s at PC = %08X\n", uint8(fb.Code), exceptionName(fb.Code), c.CPU.GPR(vax.PC))

		return nil
	}

	switch {
	// A RET popping a console-initiated CallEntry frame (Console.Call,
	// whether or not /STEP) is clean, expected completion, not an error --
	// matches emul_call.c's own CALL_active/FFFFDEAF handling, which just
	// restores the console's state and falls through with VAX_OK. No
	// message is printed here, matching Call's own pre-existing silent
	// return on this same condition.
	case errors.Is(err, cpu.ErrConsoleCallReturned):
		return nil

	case errors.Is(err, cpu.ErrHalted):
		c.Printf("HALT instruction executed at PC = %08X\n", c.CPU.GPR(vax.PC))

		return nil

	case errors.Is(err, cpu.ErrAttention):
		c.Printf("%%VAX-I-ATTENTION, execution interrupted at PC = %08X\n", c.CPU.GPR(vax.PC))

		return nil

	case errors.Is(err, cpu.ErrInstructionLimitExceeded):
		c.Printf("%%VAX-I-INSTRLIMIT, instruction limit reached at PC = %08X\n", c.CPU.GPR(vax.PC))

		return nil

	case errors.Is(err, cpu.ErrTimeLimitExceeded):
		c.Printf("%%VAX-I-TIMELIMIT, time limit reached at PC = %08X\n", c.CPU.GPR(vax.PC))

		return nil
	}

	// A fault whose SCB vector is kernel.asm's own "console$handler"
	// sentinel isn't a real Go-level error at all -- it's this port's cue to
	// run the VMS Condition Handling Facility search and, failing that,
	// report the exception natively and halt, matching interrupt.c's own
	// handle_fault/format_exception split (see docs/PHASE-20.md). Checked
	// via errors.As, not folded into the errors.Is switch above, matching
	// the FaultBreak check's own precedent just above it.
	var chf *cpu.ConsoleHandlerFault
	if errors.As(err, &chf) {
		return c.handleConsoleFault(chf)
	}

	return err
}
