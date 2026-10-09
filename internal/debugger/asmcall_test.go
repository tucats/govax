package debugger_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/console/consoletest"
	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vmserrors"
)

// debugModeConsole is a console with the real debugger installed and the
// command processor in debugger mode, where ASM, GO, and CALL are.
func debugModeConsole(t *testing.T) (*console.Console, *console.Dispatcher) {
	t.Helper()

	c, _ := consoletest.New(t)
	d := console.NewDispatcher(c, consoletest.ConsoleGrammar(t), nil)
	consoletest.InstallDebugger(t, c, d)
	consoletest.DebugMode(t, d)

	return c, d
}

// TestASMThenCall: assembling xor.asm merges its "test" label into the
// console's symbols, so a plain "CALL TEST" (no explicit address, no
// arguments) finds it by name and runs it to completion.
func TestASMThenCall(t *testing.T) {
	c, d := debugModeConsole(t)

	if err := d.Dispatch(`ASM "` + consoletest.RepoPath(t, "testdata", "asm", "xor.asm") + `"`); err != nil {
		t.Fatalf("ASM: %v", err)
	}

	if err := d.Dispatch("CALL TEST"); err != nil {
		t.Fatalf("CALL: %v", err)
	}

	const want = 0xC8600 ^ 0x10

	if got := c.CPU.GPR(vax.R4); got != want {
		t.Errorf("R4 = %#x, want %#x", got, want)
	}
}

// TestCallWithArgumentList checks CALL's "(arg1[,arg2...])" syntax against
// a small hand-assembled routine that doubles its one argument.
func TestCallWithArgumentList(t *testing.T) {
	c, d := debugModeConsole(t)

	src := "\t.entry\tdbltest, ^m<>\n\tmovl\t4(ap), r0\n\taddl2\tr0, r0\n\tret\n\t.end\n"
	path := filepath.Join(t.TempDir(), "dbl_test.asm")

	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if err := d.Dispatch(`ASM "` + path + `"`); err != nil {
		t.Fatalf("ASM: %v", err)
	}

	if err := d.Dispatch("CALL DBLTEST(^D21)"); err != nil {
		t.Fatalf("CALL: %v", err)
	}

	if got := c.CPU.GPR(vax.R0); got != 42 {
		t.Errorf("R0 = %d, want 42", got)
	}

	// The argument list may follow the routine after a blank.
	if err := d.Dispatch("CALL DBLTEST (^D4)"); err != nil {
		t.Fatalf("CALL, blank before the list: %v", err)
	}

	if got := c.CPU.GPR(vax.R0); got != 8 {
		t.Errorf("R0 = %d, want 8", got)
	}

	if err := d.Dispatch("CALL DBLTEST(^D21)"); err != nil {
		t.Fatalf("CALL: %v", err)
	}

	// Text after the argument list, or after an address with no list, is
	// an error, and nothing is called.
	for _, cmd := range []string{"CALL DBLTEST(^D5) JUNK", "CALL DBLTEST JUNK"} {
		err := d.Dispatch(cmd)
		if !errors.Is(err, vmserrors.New(vmserrors.CLI_EXTRAPARAMETER)) {
			t.Errorf("%s = %v, want EXTRAPARAMETER", cmd, err)
		}

		if got := c.CPU.GPR(vax.R0); got != 42 {
			t.Errorf("after %s, R0 = %d, want 42 (not called)", cmd, got)
		}
	}
}

// TestASMInteractive: a bare ASM in debugger mode enters the interactive
// assembler, whose lines go to the assembler until END.
func TestASMInteractive(t *testing.T) {
	c, d := debugModeConsole(t)

	for _, line := range []string{"ASM", "movl #^d42, r0", "halt", "end"} {
		if err := d.Dispatch(line); err != nil {
			t.Fatalf("%s: %v", line, err)
		}
	}

	if c.InAssemblerMode() {
		t.Error("END left the assembler on")
	}

	if err := d.Dispatch("GO 200"); err != nil {
		t.Fatalf("GO: %v", err)
	}

	if got := c.CPU.GPR(vax.R0); got != 42 {
		t.Errorf("R0 = %d, want 42", got)
	}
}
