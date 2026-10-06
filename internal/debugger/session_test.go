package debugger_test

import (
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
	"github.com/tucats/govax/internal/cpu"
	"github.com/tucats/govax/internal/debugger"
	"github.com/tucats/govax/internal/vax"
)

// Tests of the session's lifetime under run control (docs/PHASE-42.md,
// Decision 2): a run that ends by itself leaves the console prompt; a run
// the debugger stops opens a session, whose commands are the debugger's.

// newRoutedSession is a console with both grammars and a debugger, whose
// lines go through the console dispatcher's routing, as the front end's do.
func newRoutedSession(t *testing.T) (*console.Console, *console.Dispatcher, *debugger.Debugger) {
	t.Helper()

	c, _ := newTestConsole(t)

	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t), nil)
	c.Dispatcher = d

	return c, d, dbgOf(c)
}

// TestGoThatHaltsReturnsToConsole: GO at the console that runs to a HALT
// leaves no session behind: the prompt stays VAX>.
func TestGoThatHaltsReturnsToConsole(t *testing.T) {
	c, d, db := newRoutedSession(t)
	loadProgram(t, c, 0x200, opNop, opHalt)

	if err := d.Dispatch("GO 200"); err != nil {
		t.Fatalf("GO: %v", err)
	}

	if db.Active() || c.InDebugger() {
		t.Error("a GO that halted left a debugger session open")
	}
}

// TestGoThatBreaksOpensSession: GO that hits a breakpoint stops at the DBG>
// prompt, where the debugger's GO continues it, and a HALT then leaves the
// session open (it opened before the run started).
func TestGoThatBreaksOpensSession(t *testing.T) {
	c, d, db := newRoutedSession(t)
	loadProgram(t, c, 0x200, opNop, opNop, opNop, opHalt)
	db.AddBreakpoint(0x202)

	if err := d.Dispatch("GO 200"); err != nil {
		t.Fatalf("GO: %v", err)
	}

	if !db.Active() || !c.InDebugger() {
		t.Fatal("a GO that hit a breakpoint left no debugger session")
	}

	if got := c.CPU.GPR(vax.PC); got != 0x202 {
		t.Fatalf("PC = %#x, want 0x202", got)
	}

	// At DBG>, the line is the debugger's: GO runs on to the HALT, and the
	// session stays open until EXIT.
	if err := d.Dispatch("GO"); err != nil {
		t.Fatalf("GO at DBG>: %v", err)
	}

	if !db.Active() {
		t.Error("a run that ended inside an open session closed it")
	}

	if err := d.Dispatch("EXIT"); err != nil {
		t.Fatalf("EXIT: %v", err)
	}

	if db.Active() {
		t.Error("EXIT left the session open")
	}
}

// TestDebuggerStepAtPrompt: STEP is a debugger command too. It steps one
// instruction from the current PC; a GO to an address starts there.
func TestDebuggerStepAtPrompt(t *testing.T) {
	c, d, db := newRoutedSession(t)
	noUserStep(c)
	loadProgram(t, c, 0x200, opNop, opNop, opHalt)

	if err := d.Dispatch("DEBUG"); err != nil {
		t.Fatalf("DEBUG: %v", err)
	}

	c.CPU.SetGPR(vax.PC, 0x200)

	if err := d.Dispatch("STEP"); err != nil {
		t.Fatalf("STEP: %v", err)
	}

	if got := c.CPU.GPR(vax.PC); got != 0x201 {
		t.Errorf("PC after STEP = %#x, want 0x201", got)
	}

	// At DBG> a STEP's parameter is a count, as VMS has it (the console's
	// STEP takes an address).
	if err := d.Dispatch("STEP 1"); err != nil {
		t.Fatalf("STEP 1: %v", err)
	}

	if got := c.CPU.GPR(vax.PC); got != 0x202 {
		t.Errorf("PC after STEP 1 = %#x, want 0x202", got)
	}

	if !db.Active() {
		t.Error("STEP closed the session")
	}
}

// TestCallStepStopsAfterFirstInstruction: CALL/STEP runs only the routine's
// first instruction and opens a session; a plain CALL that returns doesn't.
func TestCallStepStopsAfterFirstInstruction(t *testing.T) {
	c, d, db := newRoutedSession(t)
	noUserStep(c)
	loadCallProgram(t, c)

	if err := d.Dispatch("CALL 200"); err != nil {
		t.Fatalf("CALL: %v", err)
	}

	if db.Active() {
		t.Error("a CALL that returned left a session open")
	}

	if err := d.Dispatch("CALL/STEP 200"); err != nil {
		t.Fatalf("CALL/STEP: %v", err)
	}

	if !db.Active() {
		t.Fatal("CALL/STEP left no session")
	}

	// The entry mask isn't an instruction: the first instruction is the
	// CALLS at 202, and it ran, so PC is inside the callee at 0x300 + 2.
	if got := c.CPU.GPR(vax.PC); got != 0x302 {
		t.Errorf("PC after CALL/STEP = %#x, want 0x302", got)
	}
}

// TestDebuggerCallAtPrompt: CALL at DBG> takes the routine and its
// argument list, as the console's does.
func TestDebuggerCallAtPrompt(t *testing.T) {
	c, d, db := newRoutedSession(t)
	loadCallProgram(t, c)

	if err := d.Dispatch("DEBUG"); err != nil {
		t.Fatalf("DEBUG: %v", err)
	}

	if err := d.Dispatch("CALL 200 (1,2)"); err != nil {
		t.Fatalf("CALL at DBG>: %v", err)
	}

	if !db.Active() {
		t.Error("a CALL that returned closed an open session")
	}
}

// interruptWhile runs dispatch while another goroutine types Ctrl-C
// (Engine.Attention) over and over, the way the host's keyboard does, until
// dispatch returns. Typing it once could land before the run's BeginRun,
// which clears a Ctrl-C typed while nothing was running.
func interruptWhile(t *testing.T, c *console.Console, dispatch func() error) {
	t.Helper()

	done := make(chan struct{})
	stopped := make(chan struct{})

	go func() {
		defer close(stopped)

		for {
			select {
			case <-done:
				return
			default:
				c.Engine.Attention()
			}
		}
	}()

	err := dispatch()

	close(done)
	<-stopped

	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
}

// loadSpinner loads a program that never ends by itself.
func loadSpinner(t *testing.T, c *console.Console) {
	t.Helper()

	loadProgram(t, c, 0x200,
		opNop,
		0x11, 0xFD, // BRB back to the NOP
	)
	c.Engine.SetLimits(100_000_000, 0)
}

// TestCtrlCReturnsToConsole: Ctrl-C interrupting a program the console
// started (GO) returns to the console's prompt, as CTRL/C returns to DCL's:
// it opens no debugger session.
func TestCtrlCReturnsToConsole(t *testing.T) {
	c, d, db := newRoutedSession(t)
	loadSpinner(t, c)

	interruptWhile(t, c, func() error { return d.Dispatch("GO 200") })

	if c.Engine.StoppedBy() != cpu.AttentionCtrlC {
		t.Fatalf("StoppedBy = %#x, want CTRL/C", c.Engine.StoppedBy())
	}

	if db.Active() {
		t.Error("Ctrl-C of a console GO opened a debugger session")
	}
}

// TestCtrlCReturnsToDebugger: Ctrl-C interrupting a program the debugger
// started (GO at DBG>) leaves it at DBG>.
func TestCtrlCReturnsToDebugger(t *testing.T) {
	c, d, db := newRoutedSession(t)
	loadSpinner(t, c)

	if err := d.Dispatch("DEBUG"); err != nil {
		t.Fatalf("DEBUG: %v", err)
	}

	c.CPU.SetGPR(vax.PC, 0x200)

	interruptWhile(t, c, func() error { return d.Dispatch("GO") })

	if c.Engine.StoppedBy() != cpu.AttentionCtrlC {
		t.Fatalf("StoppedBy = %#x, want CTRL/C", c.Engine.StoppedBy())
	}

	if !db.Active() {
		t.Error("Ctrl-C of a GO at DBG> ended the debugger session")
	}
}

// TestSetBreakNeedsNoSession: the debugger's SET BREAK works before any
// session is open (a breakpoint set ahead of a RUN), and the console, whose
// grammar has no SET BREAK, refuses it with a syntax error.
func TestSetBreakNeedsNoSession(t *testing.T) {
	c, d, db := newRoutedSession(t)

	if err := c.Debugger.Dispatch("SET BREAK 300"); err != nil {
		t.Fatalf("SET BREAK: %v", err)
	}

	if len(db.Breakpoints) != 1 || db.Breakpoints[0].Addr != 0x300 {
		t.Errorf("Breakpoints = %+v, want one at 300", db.Breakpoints)
	}

	if err := d.DispatchConsole("SET BREAK 300"); err == nil {
		t.Error("the console accepted SET BREAK")
	}
}
