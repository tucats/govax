package console

import (
	"fmt"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/vax"
)

// This file makes Console implement cpu.SystemServices (internal/cpu/
// services.go), the hook Engine's XFC opcode handler delegates to. SYS$/
// LIB$ dispatch, ASTs, and exceptions are the running process's, through
// the embedded corevms.EngineHooks; console I/O and command dispatch are
// handled here, since Console owns them.
//
// See services.go's own doc comment on Console.Dispatcher for why
// XFC$CONSOLE_CMD can report "unavailable" even on a fully initialized
// Console.

var _ cpu.SystemServices = (*Console)(nil)

// ConsoleWriteByte is XFC$CONSOLE_WRITE.
func (c *Console) ConsoleWriteByte(b byte) {
	if c.Out != nil {
		_, _ = c.Out.Write([]byte{b})
	}
}

// ConsoleWrite is XFC$CONSOLE_PUT: writes a whole string to Out, in one
// Write, so a line of microkernel output reaches the terminal at once
// instead of a character at a time. Does nothing if no output is configured.
func (c *Console) ConsoleWrite(p []byte) {
	if c.Out != nil {
		_, _ = c.Out.Write(p)
	}
}

// ConsoleReadByte is XFC$CONSOLE_READ. Reads one byte from In, or reports 0
// if no input source is configured or it's exhausted — matching a real
// getchar() at EOF returning a sentinel the C source doesn't itself check
// for either.
func (c *Console) ConsoleReadByte() byte {
	var buf [1]byte

	if c.In == nil {
		return 0
	}

	if _, err := c.In.Read(buf[:]); err != nil {
		return 0
	}

	return buf[0]
}

// ConsoleCommand is XFC$CONSOLE_CMD: dispatches cmd as a console command
// line. Returns 0 on success, 1 on any error (a Dispatch failure or no
// Dispatcher configured yet) — console_dispatch's own real VAX_xxx status
// codes have no exact Go equivalent here since Dispatch reports failure as
// a Go error, not a status code; callers needing more than "did it work"
// have Console's own richer error already available through normal Go
// channels wherever Dispatch is called directly.
func (c *Console) ConsoleCommand(cmdLine string) uint32 {
	if c.Dispatcher == nil {
		return 1
	}

	c.consoleCommands++
	defer func() { c.consoleCommands-- }()

	if err := c.Dispatcher.DispatchConsole(cmdLine); err != nil {
		return 1
	}

	return 0
}

// running is the process the engine's hooks reach: the System's current
// process, the one whose context the CPU holds (docs/PHASE-44.md).
// That's process 1 unless the scheduler has switched to another;
// console commands keep using process 1 (or their own subprocess's DCL
// its process), whichever runs. nil before the first INIT.
func (c *Machine) running() *corevms.Environment {
	return c.EngineHooks.Running()
}

// RunningProcessOne reports whether the CPU is running process 1, the
// console's (and the debugger's: docs/PHASE-43.md, Decision 11) process.
// The debugger's breakpoints, tracepoints, and watchpoints act only then
// (docs/PHASE-44.md, subtask 8): a P0 address names process 1's memory,
// and another process reaching the same address is somewhere else.
func (c *Machine) RunningProcessOne() bool {
	return c.running() == c.processOne
}

// ProcessNote is what a stop message adds when the CPU holds a process
// other than process 1, naming it (" in process 00000302"), and "" when
// it holds process 1 (docs/PHASE-44.md, subtask 9).
func (c *Machine) ProcessNote() string {
	if env := c.running(); env != c.processOne {
		return fmt.Sprintf(" in process %08X", env.Process.PID)
	}

	return ""
}

// ReturnToProcessOne gives the CPU back to process 1 if a run stopped in
// another process (docs/PHASE-44.md, subtask 9). After such a stop the
// CPU stays in that process, so EXAMINE and SHOW REGISTERS see it; the
// next run (GO, STEP, CALL, RUN) and the debugger's EXIT come back here,
// since the console's runs and the debugger's breakpoints are process
// 1's (docs/PHASE-43.md, Decision 11). The other process keeps its place
// in the scheduler. While scheduling is frozen (a nested run, inside
// another process's) it does nothing.
func (c *Machine) ReturnToProcessOne() error {
	if c.processOne == nil || c.Engine == nil || c.Engine.SchedulingFrozen() || c.running() == c.processOne {
		return nil
	}

	return c.processOne.SwitchCPU(c.Engine, c.processOne)
}

// ShowProcessContext moves the CPU to env's process, for the debugger's
// SET PROCESS: its registers and memory are then what EXAMINE and SHOW
// REGISTERS see, as after a run that stopped in it. The next run gives
// the CPU back to process 1 (ReturnToProcessOne). Not while a nested run
// has scheduling frozen.
func (c *Machine) ShowProcessContext(env *corevms.Environment) error {
	if c.processOne == nil || c.Engine == nil || env == c.running() {
		return nil
	}

	if c.Engine.SchedulingFrozen() {
		return fmt.Errorf("console: can't change process during a nested run")
	}

	return c.processOne.SwitchCPU(c.Engine, env)
}

// ProcessPC is env's PC: the CPU's if the CPU holds env's process, and
// otherwise the one saved in its hardware PCB when it last lost the CPU
// (0 if it has never had it).
func (c *Machine) ProcessPC(env *corevms.Environment) uint32 {
	if env == c.running() {
		return c.CPU.GPR(vax.PC)
	}

	if env.Stacks == nil || env.Stacks.PCBB == 0 {
		return 0
	}

	pcb, err := cpu.ReadPCB(c.Mem, env.Stacks.PCBB)
	if err != nil {
		return 0
	}

	return pcb.PC
}

// RequestQuit is XFC$QUIT_EMULATION's "stop the console entirely" half
// (see cpu.SystemServices' own doc comment): the same effect as Quit
// (misc.go, bound to the QUIT/EXIT console commands), reached here from a
// running VAX program instead of a typed command. Console is already
// initialized by the time any XFC executes, so this skips Quit's own
// requireInit guard rather than plumbing an error return XFC's caller
// (Engine.Step) has nowhere to put -- RequestQuit's signature is fixed by
// the SystemServices interface.
func (c *Console) RequestQuit() {
	c.quit = true
}
