package console

import (
	"errors"

	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vmserrors"
)

// Execute runs the CPU starting at the current PC (or startAddr, if
// non-nil) until it halts, hits a breakpoint, or an unhandled fault stops
// it — the console's GO/EXECUTE command.
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

	if err := c.ReturnToProcessOne(); err != nil {
		return err
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

	if err := c.ReturnToProcessOne(); err != nil {
		return err
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
		if err := c.BeginStep(); err != nil {
			return c.ReportStop(err)
		}

		finish := c.traceStep(c.CPU.GPR(vax.PC), false)

		if err := c.StepMachine(); err != nil {
			return c.ReportStop(err)
		}

		finish()
	}
}

// BeginStep lets the scheduler switch processes now, if it's due to at
// this boundary, so that what a run loop looks at before the next
// instruction (its PC, for a breakpoint or the trace) is the process
// that will run it (cpu.Engine.SwitchIfDue). Run loops call it before
// those checks, then StepMachine.
func (c *Console) BeginStep() error {
	return c.Engine.SwitchIfDue()
}

// StepMachine executes one instruction of whichever process the CPU is
// running (Engine.Step), for the console's and the debugger's run loops.
// With several processes (docs/PHASE-44.md, subtask 7), only process 1's
// image ending ends the run, as only process 1's image is the console's
// RUN, CALL, or GO: another process whose image ends (its main routine
// returning, or $EXIT, both of which reach Step as
// cpu.ErrConsoleCallReturned) is deleted (corevms.System.DeleteProcess,
// docs/PHASE-45.md), and the run goes on with the processes that are
// left. Every other
// error, a HALT in another process included (HALT stops the machine,
// whoever runs it), is returned as Step returned it.
func (c *Console) StepMachine() error {
	err := c.Engine.Step()
	if err == nil {
		return nil
	}

	if env := c.running(); env != c.RTL && errors.Is(err, cpu.ErrConsoleCallReturned) {
		// An image a subprocess's CLI ran has ended, and the CLI goes on
		// (subcli.go); otherwise the process's own run is over.
		if !c.cliImageEnded(env) {
			env.DeleteProcess(env)
		}

		return nil
	}

	return err
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
	// whether or not /STEP) is clean, expected completion, not an error. 
	// No message is printed here, matching Call's own pre-existing silent
	// return on this same condition.
	case errors.Is(err, cpu.ErrConsoleCallReturned):
		if c.imageActive {
			c.imageActive = false
			c.imageRundown()
		}

		return nil

	case errors.Is(err, cpu.ErrHalted):
		// A HALT in a process other than process 1 is unusual enough to
		// report whatever the verbosity, naming the process.
		if note := c.ProcessNote(); note != "" {
			c.Printf("%%SYSTEM-S-HALT, cpu halted at PC = %08X%s\n", c.CPU.GPR(vax.PC), note)
		} else if c.Verbose {
			c.Printf("%%SYSTEM-S-HALT, cpu halted at PC = %08X\n", c.CPU.GPR(vax.PC))
		}

		return nil

	case errors.Is(err, cpu.ErrAttention):
		c.Printf("%%VAX-I-ATTENTION, execution interrupted at PC = %08X%s\n", c.CPU.GPR(vax.PC), c.ProcessNote())

		return nil

	case errors.Is(err, cpu.ErrInstructionLimitExceeded):
		c.Printf("%%VAX-I-INSTRLIMIT, instruction limit reached at PC = %08X%s\n", c.CPU.GPR(vax.PC), c.ProcessNote())
		c.limitStop = err // a one-shot command fails on it (RunCommandLine)

		return nil

	case errors.Is(err, cpu.ErrTimeLimitExceeded):
		c.Printf("%%VAX-I-TIMELIMIT, time limit reached at PC = %08X%s\n", c.CPU.GPR(vax.PC), c.ProcessNote())
		c.limitStop = err // a one-shot command fails on it (RunCommandLine)

		return nil
	}

	// A fault whose SCB vector is kernel.asm's own "console$handler"
	// sentinel isn't a real Go-level error at all -- it's this port's cue to
	// run the VMS Condition Handling Facility search and, failing that,
	// report the exception natively and halt. Checked via errors.As, not 
	// folded into the errors.Is switch above, matching the FaultBreak 
	// check's own precedent just above it.
	var chf *cpu.ConsoleHandlerFault
	if errors.As(err, &chf) {
		return c.handleConsoleFault(chf)
	}

	return err
}
