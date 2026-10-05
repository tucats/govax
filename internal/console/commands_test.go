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

// TestCommands_stepGrammar checks STEP's spellings and qualifiers as the
// grammar reads them.
func TestCommands_stepGrammar(t *testing.T) {
	g := loadEvaxGrammar(t)

	cases := []struct {
		line, qual, addr string
	}{
		{"S", "", ""},
		{"ST/IN", "INTO", ""},
		{"STEP/INSTRUCTION 200", "INTO", "200"},
		{"STEP 200 + 4 /OVER", "OVER", "200 + 4"},
		{"STE/RET", "RETURN", ""},
	}

	for _, c := range cases {
		r, err := g.Parse(c.line)
		if err != nil {
			t.Errorf("%s: %v", c.line, err)

			continue
		}

		if r.Active != "STEP" || r.String("ADDRESS") != c.addr {
			t.Errorf("%s: %s, address %q", c.line, r.Active, r.String("ADDRESS"))
		}

		if c.qual != "" && !r.Present(c.qual) {
			t.Errorf("%s: %s not present", c.line, c.qual)
		}
	}

	for _, line := range []string{"STEP/OVER/RETURN", "STEP/NOOVER", "STEP/I"} {
		if _, err := g.Parse(line); err == nil {
			t.Errorf("%s: no error", line)
		}
	}

	for _, line := range []string{"G", "GO 200", "EXEC", "EXECUTE ."} {
		if r, err := g.Parse(line); err != nil || r.Active != "EXECUTE" {
			t.Errorf("%s: %v", line, err)
		}
	}
}

// TestCommands_depositExamine checks DEPOSIT's and EXAMINE's spellings,
// separators, and sizes. DEPOSIT spelled out used to reach no handler:
// the fixed table knew only DEP and D.
func TestCommands_depositExamine(t *testing.T) {
	d, c, buf := newCommandDispatcher(t)

	steps := []string{
		"DEPOSIT 2000 = 11223344",
		"DEPOSIT/BYTE 2000=55",
		"D/WORD 2002 6677",
		"DEP R0 = 2000 + 4",
		"D R1=1",
	}

	for _, line := range steps {
		if err := d.Dispatch(line); err != nil {
			t.Fatalf("%s: %v", line, err)
		}
	}

	if v, err := c.Mem.LoadLongword(c.CPU, 0x2000); err != nil || v != 0x66773355 {
		t.Errorf("longword at 2000 = %#x, %v; want 0x66773355", v, err)
	}

	if got := c.CPU.GPR(vax.R0); got != 0x2004 {
		t.Errorf("R0 = %#x, want 0x2004", got)
	}

	for _, line := range []string{"EXA R0", "EX/B 2000 2003", "DUMP 2000", "EXAMINE 2000 /WORD", "DIS 2000", "DISASSEMBLE 2000 2002"} {
		buf.Reset()

		if err := d.Dispatch(line); err != nil {
			t.Errorf("%s: %v", line, err)
		}

		if buf.Len() == 0 {
			t.Errorf("%s printed nothing", line)
		}
	}

	for _, line := range []string{"EXAMINE/BYTE/WORD 2000", "DEPOSIT 2000", "EXAMINE 2004 2000", "DEPOSIT/NOBYTE 2000 1"} {
		if err := d.Dispatch(line); err == nil {
			t.Errorf("%s: no error", line)
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
