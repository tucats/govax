package debugger

import (
	"github.com/tucats/govax/internal/machine"
	"github.com/tucats/govax/internal/registers"
)

// The debugger beside a program that traces itself (docs/PHASE-73 - trace
// trap.md, subtasks 3 and 4).
//
// A program can ask for control after each of its instructions by setting
// PSL<T>, the trace bit: the CPU then takes a trace fault between each
// instruction and the next, and VMS signals it to the program's condition
// handlers as SS$_TBIT (internal/cpu/trace.go says how the T and TP bits
// work). The VMS debugger uses the same bit for its own work: a STEP sets
// T and takes the trace after the instruction; going on from a breakpoint
// it puts the instruction back, runs it with T, and takes that trace. It
// can't tell the program's traces from its own, so where they meet it
// takes them. govax's debugger steps from outside the machine, but does
// what VMS 7.1's debugger was seen to do (Decision 5; testdata/trace73's
// sessions):
//
//   - A breakpoint (or watchpoint, or a STEP/RETURN's RET) takes the trace
//     pending where it stops: the program's handler never sees it, and
//     EXAMINE PSL there shows T as the program had it and TP clear
//     (stopTakesTrace).
//   - Going on from such a stop, by GO or STEP, the first instruction is
//     the debugger's: after it the program's T and TP are cleared, so the
//     program's tracing ends (dropTracing). The end of a STEP clears them
//     too (stepEnded): after a STEP over a CALLS whose routine set T in
//     its frame, T is clear. A T set by DEPOSIT goes the same way.
//   - A breakpoint in the trace handler doesn't disturb the tracing: the
//     handler runs with T clear, and the program's PSL, in the signal
//     array, keeps it.
//   - SET BREAK/EXCEPTION stops at each SS$_TBIT before the program's
//     handlers see it, as at any condition; GO passes it on to them, and
//     the tracing goes on.
//   - Other stops, which VMS's debugger doesn't make with the trace bit
//     (an exception break, Ctrl-C, govax's SET BREAK/FAULT=28), leave the
//     trace with the program: a STEP from one lets the program's handler
//     take a trace pending there, silently, then runs its instruction
//     (deliverPendingTrace). SET BREAK/FAULT=28 stops with the trace still
//     pending (cpu.Engine's holdTrace).
//   - DEPOSIT to the PSL changes only its low word (depositPSL).
//
// Not done as VMS does (the session oracle lists them): a STEP from a
// break on SS$_TBIT goes "to routine" the handler on VMS and clears T in
// the signal array's PSL, so the tracing ends when the handler continues;
// govax steps the paused dispatcher, as for any condition.

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

// stopTakesTrace is the debugger stopping the program at a breakpoint,
// a watchpoint, or a STEP/RETURN's RET: it takes the trace pending there,
// and owns the next instruction the program runs (ownsNext).
func (d *Debugger) stopTakesTrace() {
	d.clearTraceBits(true)
	d.ownsNext = true
}

// stepEnded is the end of a STEP: VMS's debugger takes its trace and
// leaves T clear, and owns the next instruction.
func (d *Debugger) stepEnded() {
	d.clearTraceBits(false)
	d.ownsNext = true
}

// dropTracing is the end of the debugger's own instruction, the first
// after a stop it made: T and TP are cleared, the program's tracing with
// them.
func (d *Debugger) dropTracing() {
	d.clearTraceBits(false)
}

// clearTraceBits clears the PSL's TP, and T too unless keepT.
func (d *Debugger) clearTraceBits(keepT bool) {
	c := d.Machine
	psl := c.CPU.PSL()

	if !keepT {
		psl.SetT(false)
	}

	psl.SetTP(false)
	c.CPU.SetPSL(psl)
}

// deliverPendingTrace runs before each instruction a STEP executes. If a
// trace is pending (after a stop that left it with the program), it goes
// to the program: its handler runs, silently, in a run of its own that
// runLoop ends when the program is back. done is true when that run stopped somewhere
// else (a breakpoint in the handler, the program ending, an unhandled
// SS$_TBIT), which ends the step; outcome and err are then the run's.
func (d *Debugger) deliverPendingTrace() (outcome runOutcome, done bool, err error) {
	if !d.Machine.CPU.PSL().TP() {
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
