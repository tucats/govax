package debugger

import (
	"github.com/tucats/govax/internal/machine"
	"github.com/tucats/govax/internal/registers"
)

// The debugger beside a program that traces itself (docs/PHASE-73 - trace
// trap.md, subtask 3).
//
// A program can ask for control after each of its instructions by setting
// PSL<T>, the trace bit: the CPU then takes a trace fault between each
// instruction and the next, and VMS signals it to the program's condition
// handlers as SS$_TBIT (internal/cpu/trace.go says how the T and TP bits
// work). The VMS debugger uses the same bit for its own STEP; govax's
// debugger never does (it steps from outside the machine), so the
// program's traces are the program's alone.
//
// What that means for each command:
//
//   - EXAMINE PSL shows T and TP like the other bits, and DEPOSIT to the
//     PSL may set T, which starts tracing: the instruction after the
//     DEPOSIT runs, and the trace comes before the one after it.
//   - GO runs the program, which takes every trace in its handler.
//   - SET BREAK/EXCEPTION stops at each SS$_TBIT before the program's
//     handlers see it, as at any condition; GO passes it on to them.
//   - SET BREAK/FAULT=28 (govax's own, the trace fault's vector) stops
//     with the trace still pending (TP set), at the instruction the trace
//     comes before; going on delivers it (cpu.Engine's holdTrace).
//   - STEP: when the instruction a step is to run has a trace pending
//     (TP set: the instruction before it was traced), the trace fault
//     comes first. The step lets the program's handler take it, silently,
//     as STEP/OVER lets a called routine run, and then runs the
//     instruction (deliverPendingTrace). So a step still moves one
//     instruction, and the program sees every trace. A breakpoint in the
//     handler stops the step there.
//
// Whether VMS's debugger takes the trace of an instruction it steps (its
// own STEP's trace would come at the same moment), so that the program's
// handler misses it, is a question for the probe (subtask 4). Decision 5
// says govax follows VMS's answer; programGetsTrace is the one place that
// decides.

// traceReturn is a trace pending when a run begins: the program's
// handler has taken it when the program is back in the access mode it was
// in, off the interrupt stack, with no trace pending and its stack pointer
// where it was (or higher, had a handler unwound frames). The handler runs
// below that stack pointer, or in another mode.
type traceReturn struct {
	sp   uint32
	mode registers.AccessMode
}

// pendingTrace returns the trace pending at the instruction about to run,
// or nil if there is none (PSL<TP> is clear).
func (d *Debugger) pendingTrace() *traceReturn {
	c := d.Machine
	psl := c.CPU.PSL()

	if !psl.TP() {
		return nil
	}

	return &traceReturn{sp: c.CPU.GPR(registers.SP), mode: psl.CurMod()}
}

// back reports whether the program is back from the handler that took
// the trace.
func (t *traceReturn) back(c *machine.Machine) bool {
	psl := c.CPU.PSL()

	return !psl.TP() && !psl.IS() && psl.CurMod() == t.mode && c.CPU.GPR(registers.SP) >= t.sp
}

// programGetsTrace decides who takes a trace pending when a STEP begins:
// true gives it to the program (its handler runs, then the step's
// instruction), false lets the debugger take it as its own, so the
// program never sees it. govax gives the program every trace until the
// probe shows what VMS's debugger does (Decision 5).
func (d *Debugger) programGetsTrace() bool {
	return !d.takeTraces
}

// deliverPendingTrace runs before each instruction a STEP executes. If a
// trace is pending, it goes to the program (its handler runs, silently, in
// a run of its own that runLoop ends when the program is back) or is
// dropped (programGetsTrace). done is true when that run stopped somewhere
// else (a breakpoint in the handler, the program ending, an unhandled
// SS$_TBIT), which ends the step; outcome and err are then the run's.
func (d *Debugger) deliverPendingTrace() (outcome runOutcome, done bool, err error) {
	c := d.Machine
	psl := c.CPU.PSL()

	if !psl.TP() {
		return runStopped, false, nil
	}

	if !d.programGetsTrace() {
		psl.SetTP(false)
		c.CPU.SetPSL(psl)

		return runStopped, false, nil
	}

	d.stepTrace, d.traceDone = true, false

	outcome, err = d.runLoop(true, func(uint32) func() { return func() {} })

	delivered := d.traceDone
	d.stepTrace, d.traceDone = false, false

	if err != nil || outcome != runStopped || !delivered {
		return outcome, true, err
	}

	return runStopped, false, nil
}
