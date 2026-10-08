package console

import (
	"errors"
	"fmt"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/vax"
)

// This file makes Console implement cpu.SystemServices (internal/cpu/
// services.go), the hook Engine's XFC opcode handler delegates to. SYS$/
// LIB$ dispatch is delegated straight to RTL (Phase 10); console I/O and
// command dispatch are handled directly since Console already owns them.
//
// See services.go's own doc comment on Console.Dispatcher for why
// XFC$CONSOLE_CMD can report "unavailable" even on a fully initialized
// Console, and DCLGetString/DCLPresent/DCLGetKeyword/DCLGetInteger's own
// comments for why the DCL-parse-callback subfunctions (XFC$DCL) are
// deliberately minimal stubs.

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

	if err := c.Dispatcher.DispatchConsole(cmdLine); err != nil {
		return 1
	}

	return 0
}

// DCLPresent/DCLGetKeyword/DCLGetString/DCLGetInteger implement XFC$DCL's
// four subfunctions, used by compiled VAX code (chiefly the microkernel's
// own kernel.asm bootstrap, assembled at vax_init time — see docs/PHASE-13.md) 
// to ask the console's DCL parser whether a qualifier was present on some 
// other, already-parsed command line and what value it carried.
//
// Phase 08's DCL engine (internal/console/dcl) parses one complete command
// line against the grammar as a single pass — it has no API for "re-open
// an already-parsed line and ask about one specific qualifier by name from
// outside," which is what these four callbacks need. Building one with no
// real caller yet to validate it against (nothing reaches XFC$DCL until
// Phase 13's kernel.asm bootstrap actually runs) would be guessing at an
// interface this port can't exercise; deliberately stubbed as "not
// present"/zero until a concrete need shows up.
func (c *Console) DCLPresent(r1, r2 uint32) uint32        { return 0 }
func (c *Console) DCLGetKeyword(r1, r2, r3 uint32) uint32 { return 0 }
func (c *Console) DCLGetInteger(r1, r2 uint32) uint32     { return 0 }

// DCLGetString reports ok=false unconditionally, for the same reason as its
// three siblings above — plus, even with real qualifier-lookup logic, this
// port has nowhere to write the result: the real callback stores into a
// fixed scratch buffer named by the "EXE$DCLSTRING" symbol, which only
// exists once Phase 13's kernel.asm bootstrap defines it.
func (c *Console) DCLGetString(r1, r2 uint32) (uint32, bool) { return 0, false }

// running is the process the engine's hooks below reach: the System's
// current process, the one whose context the CPU holds (docs/PHASE-44.md).
// That's process 1, c.RTL, unless the scheduler has switched to another;
// console commands keep using c.RTL, process 1, whichever runs. nil
// before the first INIT.
func (c *Console) running() *corevms.Environment {
	if c.RTL == nil {
		return nil
	}

	if cur := c.RTL.Current(); cur != nil {
		return cur
	}

	return c.RTL
}

// RunningProcessOne reports whether the CPU is running process 1, the
// console's (and the debugger's: docs/PHASE-43.md, Decision 11) process.
// The debugger's breakpoints, tracepoints, and watchpoints act only then
// (docs/PHASE-44.md, subtask 8): a P0 address names process 1's memory,
// and another process reaching the same address is somewhere else.
func (c *Console) RunningProcessOne() bool {
	return c.running() == c.RTL
}

// ProcessNote is what a stop message adds when the CPU holds a process
// other than process 1, naming it (" in process 00000302"), and "" when
// it holds process 1 (docs/PHASE-44.md, subtask 9).
func (c *Console) ProcessNote() string {
	if env := c.running(); env != c.RTL {
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
func (c *Console) ReturnToProcessOne() error {
	if c.RTL == nil || c.Engine == nil || c.Engine.SchedulingFrozen() || c.running() == c.RTL {
		return nil
	}

	return c.RTL.SwitchCPU(c.Engine, c.RTL)
}

// ShowProcessContext moves the CPU to env's process, for the debugger's
// SET PROCESS: its registers and memory are then what EXAMINE and SHOW
// REGISTERS see, as after a run that stopped in it. The next run gives
// the CPU back to process 1 (ReturnToProcessOne). Not while a nested run
// has scheduling frozen.
func (c *Console) ShowProcessContext(env *corevms.Environment) error {
	if c.RTL == nil || c.Engine == nil || env == c.running() {
		return nil
	}

	if c.Engine.SchedulingFrozen() {
		return fmt.Errorf("console: can't change process during a nested run")
	}

	return c.RTL.SwitchCPU(c.Engine, env)
}

// ProcessPC is env's PC: the CPU's if the CPU holds env's process, and
// otherwise the one saved in its hardware PCB when it last lost the CPU
// (0 if it has never had it).
func (c *Console) ProcessPC(env *corevms.Environment) uint32 {
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

// SystemService delegates to the running process (Phase 10's SYS$
// dispatch).
func (c *Console) SystemService(pc uint32) (uint32, bool, error) {
	r0, handled, err := c.running().SystemService(pc)

	return r0, handled, translateHalt(err)
}

// NextAST delegates to the running process, making Console a
// cpu.ASTSource as well: the
// engine asks at every instruction boundary whether an AST is due, and
// the RTL, which owns the AST queues, answers (docs/PHASE-26.md subtask
// 15). This only converts between the two packages' types, keeping
// internal/rtl free of an internal/cpu import.
func (c *Console) NextAST() (cpu.ASTCall, bool, error) {
	routine, argList, returnPC, ok, err := c.running().NextAST()

	return cpu.ASTCall{Routine: routine, ArgList: argList, ReturnPC: returnPC}, ok, err
}

// HandleAttention delegates to the running process, making Console a
// cpu.AttentionHandler:
// when the user types CTRL/C (or CTRL/Y) while a program runs, the
// engine asks whether the program has an AST enabled for it before
// stopping the machine (docs/PHASE-26.md subtask 27).
func (c *Console) HandleAttention(key byte) bool {
	env := c.running()
	if env == nil {
		return false
	}

	return env.AttentionAny(env, key)
}

// DispatchException delegates to the running process, making Console a
// cpu.ExceptionDispatcher: an exception kernel.asm's SCB sends to
// console$handler is offered to the running program's condition
// handlers before the console reports it (docs/PHASE-26.md subtask 31).
func (c *Console) DispatchException(code uint32, params []uint32, pc uint32, psl vax.PSL) (bool, error) {
	env := c.running()
	if env == nil {
		return false, nil
	}

	return env.DispatchException(code, params, pc, psl)
}

// Shim delegates to the running process (Phase 10's LIB$/CRTL shim
// dispatch).
func (c *Console) Shim(code uint32) (uint32, bool, error) {
	r0, handled, err := c.running().Shim(code)

	return r0, handled, translateHalt(err)
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

// translateHalt turns corevms.ErrHalt (a SYS$ service or shim requesting the
// machine halt, e.g. an unrecognized SYS$CLI request) into cpu.ErrHalted,
// corevms.ErrWait (a service waiting for an event flag, docs/PHASE-26.md)
// into cpu.ErrServiceWait, and $EXIT's corevms.CallRequest (call an exit
// handler) and corevms.ErrExit (the image has exited) into cpu.ServiceCall
// and cpu.ErrImageExit — the signals Engine.Step actually recognizes,
// kept as a translation at the boundary rather than internal/rtl
// importing internal/cpu, so that package has no dependency on this one.
func translateHalt(err error) error {
	var call *corevms.CallRequest

	switch {
	case errors.Is(err, corevms.ErrHalt):
		return cpu.ErrHalted
	case errors.Is(err, corevms.ErrWait):
		return cpu.ErrServiceWait
	case errors.Is(err, corevms.ErrExit):
		return cpu.ErrImageExit
	case errors.As(err, &call):
		return &cpu.ServiceCall{Routine: call.Routine, ArgList: call.ArgList, Image: call.Image}
	}

	return err
}
