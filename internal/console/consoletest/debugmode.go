package consoletest

import (
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/debugger"
)

// InstallDebugger gives c the real debugger, as cmd/govax does, and makes
// d the console's dispatcher. GO, CALL, and (once the microkernel is in
// place) ASM are the debugger's commands, so a console that runs vax.init
// or those commands needs one installed first.
func InstallDebugger(t testing.TB, c *console.Console, d *console.Dispatcher) *debugger.Debugger {
	t.Helper()

	c.Dispatcher = d

	return debugger.Install(c, DebugGrammar(t), ParseHelp(t, "debug.help"))
}

// DebugMode puts the command processor into debugger mode, as the DEBUG
// command does, so the lines that follow (GO, CALL, ASM, ...) go to the
// debugger's grammar. Dispatch "EXIT" to leave it.
func DebugMode(t testing.TB, d *console.Dispatcher) {
	t.Helper()

	if err := d.Dispatch("DEBUG"); err != nil {
		t.Fatalf("DEBUG: %v", err)
	}
}
