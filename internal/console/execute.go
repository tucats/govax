package console

import (
	"errors"

	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vmserrors"
)

// Execute runs the CPU starting at the current PC (or startAddr, if
// non-nil) until it halts, hits a breakpoint, or an unhandled fault stops
// it — the console's GO/EXECUTE command (console_exec.c's console_exec).
//
// The run itself belongs to the debugger (docs/PHASE-42.md, subtask 4):
// the breakpoints, the run loop, and the stop messages are in
// internal/debugger. With one installed, this starts a session and
// returns when the program ends or the debugger stops it (the DBG>
// prompt then follows). A bare Console with none, as many of this
// package's own tests use, runs the program to its end with no
// breakpoints.
func (c *Console) Execute(startAddr *uint32) error {
	if err := c.requireInit(); err != nil {
		return err
	}

	if c.Debugger != nil {
		return c.Debugger.Start(Activation{Kind: ActivateGo, Addr: startAddr})
	}

	if startAddr != nil {
		c.CPU.SetGPR(vax.PC, *startAddr)
	}

	c.Engine.BeginRun()

	return c.runPlain()
}

// Call invokes the procedure at addr with the given arguments (pushed
// right-to-left, as a real CALLS instruction does; see
// cpu.Engine.CallEntry) and runs it until it returns. RUN's IMAGE$INIT
// driver, the CHF's condition handlers, and the CALL command all come
// through here. With step true, only the routine's first instruction runs
// (CALL/STEP), which needs the debugger.
//
// As with Execute, a debugger, when installed, runs it (and so honors
// breakpoints, docs/PHASE-42.md bug 1); a bare Console runs it to its end.
func (c *Console) Call(addr uint32, step bool, args ...uint32) error {
	if err := c.requireInit(); err != nil {
		return err
	}

	if c.Debugger != nil {
		return c.Debugger.Start(Activation{Kind: ActivateCall, Addr: &addr, Step: step, Args: args})
	}

	if step {
		return vmserrors.New(vmserrors.DBG_NOTAVAILABLE)
	}

	if err := c.Engine.CallEntry(addr, args...); err != nil {
		return err
	}

	c.Engine.BeginRun()

	return c.runPlain()
}

// Step is the console's STEP command: the debugger does the stepping, and
// a STEP always leaves it stopped, so the session it starts stays open
// for the next command. mode is "", INTO, OVER, or RETURN ("" being the
// debugger's own default, SET STEP).
func (c *Console) Step(startAddr *uint32, mode string) error {
	if err := c.requireInit(); err != nil {
		return err
	}

	if c.Debugger == nil {
		return vmserrors.New(vmserrors.DBG_NOTAVAILABLE)
	}

	return c.Debugger.Start(Activation{Kind: ActivateStep, Addr: startAddr, StepMode: mode})
}

// runPlain runs the CPU until something ends the run, honoring SET TRACE
// but no breakpoints: what GO and CALL do on a console without a debugger.
func (c *Console) runPlain() error {
	for {
		finish := c.traceStep(c.CPU.GPR(vax.PC), false)

		if err := c.Engine.Step(); err != nil {
			return c.ReportStop(err)
		}

		finish()
	}
}

// ReportStop handles every "the run stopped for a benign, expected
// reason" outcome Engine.Step can produce -- a HALT instruction, a Ctrl-C
// interrupt (see cpu.Engine.Attention), and this project's own
// -instruction-limit/-time-limit guards (docs/PHASE-15.md's sub-phase 2) --
// by printing a matching console message and returning nil, so
// the debugger's run loops can just call this
// on any Step error. Any other error (an unhandled fault, a real Go error)
// is returned unchanged for the caller to propagate.
func (c *Console) ReportStop(err error) error {
	switch {
	// A RET popping a console-initiated CallEntry frame (Console.Call,
	// whether or not /STEP) is clean, expected completion, not an error --
	// matches emul_call.c's own CALL_active/FFFFDEAF handling, which just
	// restores the console's state and falls through with VAX_OK. No
	// message is printed here, matching Call's own pre-existing silent
	// return on this same condition.
	case errors.Is(err, cpu.ErrConsoleCallReturned):
		if c.imageActive {
			c.imageActive = false
			c.imageRundown()
		}

		return nil

	case errors.Is(err, cpu.ErrHalted):
		if c.Verbose {
			c.Printf("%%SYSTEM-S-HALT, cpu halted at PC = %08X\n", c.CPU.GPR(vax.PC))
		}

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
