package console

// Call implements the console CALL command's core mechanism (see
// docs/PHASE-13.md): invokes the procedure at addr with the given arguments
// (pushed right-to-left, matching a real CALLS instruction -- see
// cpu.Engine.CallEntry), then either runs Engine.Step in a loop until it
// returns (cpu.CallEntry/ErrConsoleCallReturned) or halts (cpu.ErrHalted),
// or -- if step is true -- executes exactly the entered procedure's first
// instruction and returns, leaving the rest to be single-stepped with the
// console's own STEP command. This matches console_exec.c's own
// console_call: on /STEP (or its /BREAK|/DEBUG synonyms) it doesn't run the
// code at all, it delegates straight to console_step for one instruction --
// the run-to-completion path below is only taken otherwise.
//
// The run goes through runLoop, as GO's does, so breakpoints stop it: RUN
// (which calls its IMAGE$INIT driver through here) and CALL used to run
// to completion past every breakpoint (docs/PHASE-42.md, bug 1). The
// first instruction is checked too (skipFirstCheck false): it's the
// called routine's, not a place the console was already stopped at, so
// a breakpoint there fires at once, as one does for STEP/OVER's callee.
func (c *Console) Call(addr uint32, step bool, args ...uint32) error {
	if err := c.requireInit(); err != nil {
		return err
	}

	if err := c.Engine.CallEntry(addr, args...); err != nil {
		return err
	}

	c.Engine.BeginRun()

	if step {
		return c.stepInto()
	}

	return c.runLoop(false, func(pc uint32) func() { return c.traceStep(pc, false) })
}
