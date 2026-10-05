package debugger_test

import (
	"bytes"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
	"github.com/tucats/govax/internal/debugger"
	"github.com/tucats/govax/internal/vax"
)

// Opcodes the run-control tests assemble by hand.
const (
	opHalt = 0x00
	opNop  = 0x01
)

// newTestConsole returns a small initialized console (64 KB, no page
// tables) with a debugger installed, as the run-control tests of
// internal/console used before the run loop moved here, and its output.
func newTestConsole(t *testing.T) (*console.Console, *bytes.Buffer) {
	t.Helper()

	var buf bytes.Buffer

	c := console.New(&buf)
	if err := c.Init(64 * 1024); err != nil {
		t.Fatalf("Init: %v", err)
	}

	debugger.Install(c, consoletest.DebugGrammar(t), nil)

	return c, &buf
}

// newRunnableConsole returns a console ready to run VMS images, with a
// debugger installed (consoletest.New's machine, and its output).
func newRunnableConsole(t testing.TB) *console.Console {
	t.Helper()

	c, _ := consoletest.New(t)

	debugger.Install(c, consoletest.DebugGrammar(t), nil)

	return c
}

// dbgOf returns c's debugger.
func dbgOf(c *console.Console) *debugger.Debugger {
	return c.Debugger.(*debugger.Debugger)
}

// loadProgram deposits bytes into memory at addr.
func loadProgram(t *testing.T, c *console.Console, addr uint32, bytes ...byte) {
	t.Helper()

	for i, b := range bytes {
		if err := c.Deposit("", addr+uint32(i), console.SizeByte, uint32(b)); err != nil {
			t.Fatalf("Deposit: %v", err)
		}
	}
}

// movR0Program loads MOVL #0x12345678,R0 and then a HALT at addr: enough
// to exercise the instruction trace and the register-change dump.
func movR0Program(t *testing.T, c *console.Console, addr uint32) {
	t.Helper()
	loadProgram(t, c, addr, 0xD0, 0x8F, 0x78, 0x56, 0x34, 0x12, 0x50, opHalt)
}

// kernelPath is the microkernel's source, for tests that run an image.
func kernelPath(t testing.TB) string { return consoletest.KernelPath(t) }

// dbgImagePath is one of Phase 41's debugger probe images.
func dbgImagePath(t testing.TB, name string) string { return consoletest.DebugImagePath(t, name) }

// noUserStep makes STEP step kernel-mode code, which these tests run in:
// STEP's default is to step only user-mode instructions (USERSTEP).
func noUserStep(c *console.Console) {
	c.CPU.SetDebug(c.CPU.Debug() &^ vax.DebugUserStep)
}

// newTestDispatcher returns a console dispatcher (the console grammar) and
// its console, with a debugger installed. Tests send console commands
// through DispatchConsole, since after a run stops the session is open and
// Dispatch would route the line to the debugger's own grammar, which
// doesn't have SET BREAK, DEPOSIT, ... until the later subtasks of
// docs/PHASE-42.md add them.
func newTestDispatcher(t *testing.T) (*console.Dispatcher, *console.Console) {
	t.Helper()

	c, _ := newTestConsole(t)

	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t), nil)
	c.Dispatcher = d

	return d, c
}
