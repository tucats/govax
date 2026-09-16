package console

import (
	"errors"

	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/vax"
)

// BreakKind distinguishes the kinds of breakpoint console_run.c's
// struct BREAKSTR supports. Only BreakAddress is implemented by this port —
// see doc.go and this file's own comments for why BreakFault (a breakpoint
// that fires when a given exception code is about to be delivered) is
// deferred.
type BreakKind int

const (
	BreakAddress BreakKind = iota
)

// Breakpoint is one entry in Console.Breakpoints.
type Breakpoint struct {
	Kind BreakKind
	Addr uint32
}

// AddBreakpoint sets an address breakpoint, matching SET BREAKPOINT (see
// console_set.c). A duplicate address is a no-op.
func (c *Console) AddBreakpoint(addr uint32) {
	if c.breakpointAt(addr) != nil {
		return
	}

	c.Breakpoints = append(c.Breakpoints, &Breakpoint{Kind: BreakAddress, Addr: addr})
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

	first := true

	for {
		pc := c.CPU.GPR(vax.PC)
		if !first {
			if bp := c.breakpointAt(pc); bp != nil {
				c.Printf("Break at %08X\n", pc)

				return nil
			}
		}

		first = false

		finish := c.traceStep(pc, false)
		if err := c.Engine.Step(); err != nil {
			return c.reportStopReason(err)
		}
		finish()
	}
}

// Step implements STEP: executes exactly one instruction starting at the
// current PC (or startAddr, if non-nil), matching console_step.c's
// STEP_INSTRUCTION (the default and, per docs/PHASE-08.md's progress log,
// only step mode this port implements — STEP/OVER and STEP/RETURN are
// accepted as synonyms for it rather than skipping over a called
// subroutine, since that needs the same temporary-breakpoint machinery
// vax.c's STEP_OVER case uses, which has no other consumer to justify
// building yet).
func (c *Console) Step(startAddr *uint32) error {
	if err := c.requireInit(); err != nil {
		return err
	}

	if startAddr != nil {
		c.CPU.SetGPR(vax.PC, *startAddr)
	}

	c.Engine.BeginRun()

	// STEP always traces, regardless of Console.Trace, matching
	// console_step.c:117's own "Always in trace mode" (execute_vax(1)) --
	// see docs/PHASE-17.md sub-phase 7.
	finish := c.traceStep(c.CPU.GPR(vax.PC), true)
	if err := c.Engine.Step(); err != nil {
		return c.reportStopReason(err)
	}
	finish()

	c.Printf("Stepped to %08X\n", c.CPU.GPR(vax.PC))

	return nil
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
	switch {
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

	default:
		return err
	}
}
