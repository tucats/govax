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

	for _, line := range []string{"HELP", "?", "HEL"} {
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

// TestCommands_goGrammar checks GO's spellings as the grammar reads them.
// STEP, EXAMINE, DEPOSIT, and DISASSEMBLE are the debugger's now
// (docs/PHASE-42.md), and its tests cover them.
func TestCommands_goGrammar(t *testing.T) {
	g := loadEvaxGrammar(t)

	for _, line := range []string{"G", "GO 200", "EXEC", "EXECUTE ."} {
		if r, err := g.Parse(line); err != nil || r.Active != "EXECUTE" {
			t.Errorf("%s: %v", line, err)
		}
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

// TestCommands_include checks INCLUDE's spellings and
// INCLUDE/COMMAND_LINE, now a qualifier rather than a file name.
func TestCommands_include(t *testing.T) {
	d, c, _ := newCommandDispatcher(t)

	path := filepath.Join(t.TempDir(), "cmds.com")
	if err := os.WriteFile(path, []byte("SET R6=6\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, line := range []string{`INCLUDE "` + path + `"`, `INC "` + path + `"`, `@"` + path + `"`, `@ "` + path + `"`} {
		c.CPU.SetGPR(vax.R6, 0)

		if err := d.Dispatch(line); err != nil {
			t.Fatalf("%s: %v", line, err)
		}

		if got := c.CPU.GPR(vax.R6); got != 6 {
			t.Errorf("%s: R6 = %#x, want 6", line, got)
		}
	}

	saved := CommandLineString
	defer func() { CommandLineString = saved }()

	CommandLineString = ""

	if err := d.Dispatch("INCLUDE/COMMAND_LINE"); err != nil {
		t.Errorf("INCLUDE/COMMAND_LINE with no command: %v", err)
	}

	CommandLineString = "SET R7=7"

	err := d.Dispatch("INCLUDE/COMMAND_LINE")
	if !errors.Is(err, vmserrors.New(vmserrors.VAX_QUIT)) {
		t.Errorf("INCLUDE/COMMAND_LINE: %v, want VAX_QUIT", err)
	}

	if got := c.CPU.GPR(vax.R7); got != 7 {
		t.Errorf("R7 = %#x, want 7", got)
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
