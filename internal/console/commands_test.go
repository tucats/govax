package console

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vmserrors"
)

// newCommandDispatcher returns a Dispatcher on the console grammar and the
// console's output buffer, emptied.
func newCommandDispatcher(t *testing.T) (*Dispatcher, *Console, *bytes.Buffer) {
	t.Helper()

	c, buf := newTestConsole(t)
	d := NewDispatcher(c, loadEvaxGrammar(t), ParseHelp("$HELP\nTop-level help.\n"))

	buf.Reset()

	return d, c, buf
}

// TestCommands_print checks PRINT's and ECHO's lists of strings and
// expressions, now read by the grammar (docs/PHASE-37.md).
func TestCommands_print(t *testing.T) {
	d, _, buf := newCommandDispatcher(t)

	cases := []struct{ line, want string }{
		{`PRINT "Hello, ", 200`, "Hello, 00000200\n"},
		{`ECHO "a/b", 10 + 6`, "a/b00000016\n"},
		{`PRIN "Mixed Case"`, "Mixed Case\n"},
		{"PRINT", "\n"},
		{"PRINT (1, 2)", ""},
	}

	for _, c := range cases {
		buf.Reset()

		err := d.Dispatch(c.line)
		if c.want == "" {
			if err == nil {
				t.Errorf("%s: no error", c.line)
			}

			continue
		}

		if err != nil {
			t.Errorf("%s: %v", c.line, err)

			continue
		}

		if buf.String() != c.want {
			t.Errorf("%s printed %q, want %q", c.line, buf.String(), c.want)
		}
	}
}

func TestCommands_helpAliases(t *testing.T) {
	d, _, buf := newCommandDispatcher(t)

	for _, line := range []string{"HELP", "HEL"} {
		buf.Reset()

		if err := d.Dispatch(line); err != nil {
			t.Fatalf("%s: %v", line, err)
		}

		if !strings.Contains(buf.String(), "Top-level help.") {
			t.Errorf("%s printed %q", line, buf.String())
		}
	}
}

func TestCommands_time(t *testing.T) {
	d, c, buf := newCommandDispatcher(t)

	if err := d.Dispatch("TIME SET R3=7"); err != nil {
		t.Fatal(err)
	}

	if got := c.CPU.GPR(vax.R3); got != 7 {
		t.Errorf("R3 = %#x, want 7", got)
	}

	if !strings.Contains(buf.String(), "Elapsed time:") {
		t.Errorf("TIME printed %q", buf.String())
	}

	buf.Reset()

	if err := d.Dispatch("TIME"); err != nil {
		t.Fatal(err)
	}

	if !strings.HasPrefix(buf.String(), "Time is ") {
		t.Errorf("bare TIME printed %q", buf.String())
	}
}

func TestCommands_ifErrors(t *testing.T) {
	d, _, _ := newCommandDispatcher(t)

	if err := d.Dispatch("IF"); !errors.Is(err, vmserrors.New(vmserrors.CLI_MISSINGPARAMETER)) {
		t.Errorf("bare IF: %v, want CLI_MISSINGPARAMETER", err)
	}

	if err := d.Dispatch("IF 0 THEN BOGUS"); err != nil {
		t.Errorf("IF 0 THEN BOGUS: %v (a false condition doesn't run its command)", err)
	}

	if err := d.Dispatch("IF 1 THEN BOGUS"); err == nil {
		t.Error("IF 1 THEN BOGUS: no error")
	}
}

func TestCommands_notImplemented(t *testing.T) {
	d, _, _ := newCommandDispatcher(t)

	for _, line := range []string{"BOOT", "ROM"} {
		if err := d.Dispatch(line); !errors.Is(err, vmserrors.New(vmserrors.CLI_NEEDDEP)) {
			t.Errorf("%s: %v, want CLI_NEEDDEP", line, err)
		}
	}
}

func TestCommands_zero(t *testing.T) {
	d, c, _ := newCommandDispatcher(t)

	if err := c.Deposit("", 0x1000, SizeLongword, 0x55); err != nil {
		t.Fatal(err)
	}

	if err := d.Dispatch("ZERO"); err != nil {
		t.Fatal(err)
	}

	if got, err := c.Mem.LoadLongword(c.CPU, 0x1000); err != nil || got != 0 {
		t.Errorf("memory after ZERO = %#x, %v", got, err)
	}
}

// TestCommands_goIsNotConsole checks that GO and its spellings, CALL, and
// (once the microkernel is in place) ASM are the debugger's commands: the
// console's grammar has no GO or CALL (docs/PHASE-42.md's command split
// test checks the debugger has them).
func TestCommands_goIsNotConsole(t *testing.T) {
	g := loadEvaxGrammar(t)

	for _, line := range []string{"G", "GO 200", "EXEC", "EXECUTE .", "CALL A"} {
		if _, err := g.Parse(line); err == nil {
			t.Errorf("%s parsed as a console command", line)
		}
	}
}

// TestCommands_asmOnlyBeforeKernel checks that the console's ASM works
// until the microkernel is in place and then says it is unrecognized.
func TestCommands_asmOnlyBeforeKernel(t *testing.T) {
	c := newRunnableConsole(t)
	d := NewDispatcher(c, loadEvaxGrammar(t), nil)

	// A stand-in microkernel: all that matters is that it defines
	// EXE$INITIALIZE.
	kernel := filepath.Join(t.TempDir(), "kernel.asm")
	src := "\t.entry\texe$initialize, ^m<>\n\tret\n\t.end\n"

	if err := os.WriteFile(kernel, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := d.Dispatch(`ASM "` + asmFixturePath(t, "xor.asm") + `"`); err != nil {
		t.Fatalf("ASM before the microkernel: %v", err)
	}

	if err := d.Dispatch(`ASM "` + kernel + `"`); err != nil {
		t.Fatalf("ASM of the microkernel: %v", err)
	}

	err := d.Dispatch(`ASM "` + asmFixturePath(t, "xor.asm") + `"`)
	if !errors.Is(err, vmserrors.New(vmserrors.CLI_IVVERB)) {
		t.Errorf("ASM after the microkernel = %v, want IVVERB", err)
	}

	if err := d.Dispatch("ASM"); err == nil || c.InAssemblerMode() {
		t.Errorf("bare ASM after the microkernel = %v (assembler mode %v)", err, c.InAssemblerMode())
	}
}

// TestCommands_saveLoad checks SAVE's and LOAD's qualifiers, /NOERROR
// among them (which LoadROM used to recognize as its file name).
func TestCommands_saveLoad(t *testing.T) {
	d, c, _ := newCommandDispatcher(t)
	dir := t.TempDir()

	c.Engine.Memory().NVRAMBase = 0x20140400
	c.Engine.Memory().NVRAM = []byte{1, 2, 3, 4, 5, 6, 7, 8}

	ok := []string{
		`LOAD/ROM "` + romFixturePath(t) + `"`,
		`SAVE/ROM "` + filepath.Join(dir, "x.rom") + `"`,
		`LOAD "` + filepath.Join(dir, "x.rom") + `" /ROM`,
		`SAVE/NVRAM "` + filepath.Join(dir, "x.nvram") + `"`,
		`LOAD/NVRAM "` + filepath.Join(dir, "x.nvram") + `"`,
		"LOAD/ROM/NOERROR",
		`LOAD/NVRAM/NOERROR "` + filepath.Join(dir, "missing.nvram") + `"`,
	}

	for _, line := range ok {
		if err := d.Dispatch(line); err != nil {
			t.Errorf("%s: %v", line, err)
		}
	}

	bad := []struct {
		line   string
		status uint32
	}{
		{"SAVE FOO.VAX", vmserrors.CLI_NEEDROMNVRAM},
		{"LOAD/ROM", vmserrors.CLI_NEEDFILENAME},
		{"SAVE/ROM/NVRAM X", vmserrors.CLI_BADQUALIFIERCOMBO},
		{"SAVE/NOROM X", vmserrors.CLI_NONEGATE},
	}

	for _, c := range bad {
		if err := d.Dispatch(c.line); !errors.Is(err, vmserrors.New(c.status)) {
			t.Errorf("%s: %v, want %v", c.line, err, vmserrors.New(c.status))
		}
	}

	if err := d.Dispatch(`LOAD/ROM "` + filepath.Join(dir, "missing.rom") + `"`); err == nil {
		t.Error("LOAD/ROM of a missing file: no error")
	}
}

// TestCommands_at checks @'s spellings, that the console has no INCLUDE
// (docs/PHASE-50 - DCL command procedures.md), and RunCommandLine, which
// took over INCLUDE/COMMAND_LINE.
func TestCommands_at(t *testing.T) {
	d, c, _ := newCommandDispatcher(t)

	path := filepath.Join(t.TempDir(), "cmds.com")
	if err := os.WriteFile(path, []byte("$ SET R6=6\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, line := range []string{`@"` + path + `"`, `@ "` + path + `"`} {
		c.CPU.SetGPR(vax.R6, 0)

		if err := d.Dispatch(line); err != nil {
			t.Fatalf("%s: %v", line, err)
		}

		if got := c.CPU.GPR(vax.R6); got != 6 {
			t.Errorf("%s: R6 = %#x, want 6", line, got)
		}
	}

	for _, line := range []string{`INCLUDE "` + path + `"`, "INCLUDE/COMMAND_LINE"} {
		if err := d.Dispatch(line); !errors.Is(err, vmserrors.New(vmserrors.CLI_IVVERB)) {
			t.Errorf("%s: %v, want CLI_IVVERB", line, err)
		}
	}

	saved := CommandLineString
	defer func() { CommandLineString = saved }()

	CommandLineString = ""

	if c.RunCommandLine(d.Dispatch) || !c.Running() {
		t.Error("RunCommandLine with no command ran one, or ended the session")
	}

	CommandLineString = "SET R7=7"

	if !c.RunCommandLine(d.Dispatch) {
		t.Error("RunCommandLine didn't run the command")
	}

	if got := c.CPU.GPR(vax.R7); got != 7 {
		t.Errorf("R7 = %#x, want 7", got)
	}

	if c.Running() || c.CommandLineErr() != nil {
		t.Errorf("after the one-shot command: running %v, error %v; want ended, no error", c.Running(), c.CommandLineErr())
	}
}

// TestCommands_set checks SET's grammar: assignments, which win over a
// keyword they abbreviate, negated keywords, and its sub-forms' values.
func TestCommands_set(t *testing.T) {
	d, c, _ := newCommandDispatcher(t)

	steps := []string{
		"SET R=5",
		"SET X = 1 + 2 /PERMANENT",
		"SET/LBL Y=X",
		"SET NOVERBOSE",
		"SET UIQ 7",
		"SET DEBUG VM, NOUSERHALT",
		"SET RADIX = 10",
	}

	for _, line := range steps {
		if err := d.Dispatch(line); err != nil {
			t.Fatalf("%s: %v", line, err)
		}
	}

	for name, want := range map[string]uint32{"R": 5, "X": 3, "Y": 3, "RADIX": 0x10} {
		if v, ok := c.Symbols.Get(name); !ok || v != want {
			t.Errorf("symbol %s = %#x, %v; want %#x", name, v, ok, want)
		}
	}

	if c.Radix != 16 {
		t.Errorf("radix %d after SET RADIX = 10 (an assignment)", c.Radix)
	}

	if c.Trace || c.Verbose {
		t.Errorf("Trace %v, Verbose %v; want both off", c.Trace, c.Verbose)
	}

	for _, line := range []string{"SET NORADIX 10", "SET RADIX", "SET BOGUS", "SET V", "SET BREAK/BOGUS 100", "SET QUANTUM X"} {
		if err := d.Dispatch(line); err == nil {
			t.Errorf("%s: no error", line)
		}
	}
}
