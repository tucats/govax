package debugger_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
)

// stoppedAt runs the image at path under the debugger, sets a breakpoint
// at where, and continues to it, returning the console with its output
// cleared (the stop message is another test's business).
func stoppedAt(t *testing.T, path, where string) *console.Console {
	t.Helper()

	c, _ := runImage(t, path, console.RunOptions{Debug: console.DebugOn})
	dispatch(t, c, "SET BREAK "+where)
	dispatch(t, c, "GO")

	c.Out.(*bytes.Buffer).Reset()

	return c
}

// dispatch sends one line to the debugger, failing the test on an error.
func dispatch(t *testing.T, c *console.Console, line string) {
	t.Helper()

	if err := c.Debugger.Dispatch(line); err != nil {
		t.Fatalf("%s: %v", line, err)
	}
}

// examineOutput returns what the debugger command line prints.
func examineOutput(t *testing.T, c *console.Console, line string) string {
	t.Helper()

	buf := c.Out.(*bytes.Buffer)
	buf.Reset()

	dispatch(t, c, line)

	return buf.String()
}

// TestExamineOperands: EXAMINE/OPERANDS explains each register and memory
// operand, as dbgdis.dlg shows it: an operand longer than ten columns takes
// its own line, and its description follows under it.
func TestExamineOperands(t *testing.T) {
	c := stoppedAt(t, dbgImagePath(t, "dbgdis.exe"), "%LINE 47")

	want := `DBGDIS\START\%LINE 47:  MOVL     L^DBGDIS\COUNT,R0
     L^DBGDIS\COUNT
                DBGDIS\COUNT (address 00000200) contains 00000000
     R0         R0 contains 00000003
`

	for _, command := range []string{"EXAMINE/OPERANDS .PC", "EXAMINE/OPERANDS=FULL .PC"} {
		if got := examineOutput(t, c, command); got != want {
			t.Errorf("%s:\ngot:\n%s\nwant:\n%s", command, got, want)
		}
	}

	// Without the qualifier, the instruction alone.
	if got, want := examineOutput(t, c, "EXAMINE/INSTRUCTION .PC"), strings.SplitAfter(want, "\n")[0]; got != want {
		t.Errorf("EXAMINE/INSTRUCTION .PC: got %q, want %q", got, want)
	}

	// SET MODE OPERANDS turns it on for each EXAMINE/INSTRUCTION, and
	// NOOPERANDS off again.
	dispatch(t, c, "SET MODE OPERANDS")

	if got := examineOutput(t, c, "EXAMINE/INSTRUCTION .PC"); got != want {
		t.Errorf("SET MODE OPERANDS:\ngot:\n%s\nwant:\n%s", got, want)
	}

	dispatch(t, c, "SET MODE NOOPERANDS")

	if got := examineOutput(t, c, "EXAMINE/INSTRUCTION .PC"); strings.Count(got, "\n") != 1 {
		t.Errorf("SET MODE NOOPERANDS: got %q, want the instruction alone", got)
	}
}

// TestExamineOperandsUnnamed: an address no debug symbol names is shown
// bare, and an operand of ten columns or fewer shares its line with its
// description (dbgtrc.dlg, an image with traceback only).
func TestExamineOperandsUnnamed(t *testing.T) {
	c := stoppedAt(t, dbgImagePath(t, "dbgtrc.exe"), "DBGDIS\\START+19")

	want := `DBGDIS\START+19:        MOVL     L^00000200,R0
     L^00000200 00000200 contains 00000000
     R0         R0 contains 00000003
`

	if got := examineOutput(t, c, "EXAMINE/OPERANDS .PC"); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

// TestExamineRangesAndLists: a location may be a range (start:end) or a
// comma-separated list, each shown in turn.
func TestExamineRangesAndLists(t *testing.T) {
	c := stoppedAt(t, dbgImagePath(t, "dbgdis.exe"), "%LINE 47")

	two := examineOutput(t, c, `EXAMINE/INSTRUCTION %LINE 47:%LINE 48`)
	if n := strings.Count(two, "\n"); n < 2 {
		t.Errorf("a range of two lines gave %d lines: %q", n, two)
	}

	if !strings.HasPrefix(two, `DBGDIS\START\%LINE 47:`) {
		t.Errorf("the range doesn't start at line 47: %q", two)
	}

	list := examineOutput(t, c, `EXAMINE/INSTRUCTION %LINE 47,%LINE 49`)
	if !strings.Contains(list, `%LINE 47:`) || !strings.Contains(list, `%LINE 49:`) {
		t.Errorf("a list of two lines gave %q", list)
	}
}

// TestSetModeAndRadix: SET MODE NOSYMBOLIC shows an address as a number,
// and SET RADIX changes the radix a number is typed in and the one
// offsets in names are shown in (dbgdis.dlg's GLIMIT+589).
func TestSetModeAndRadix(t *testing.T) {
	c := stoppedAt(t, dbgImagePath(t, "dbgdis.exe"), "%LINE 47")

	dispatch(t, c, "SET MODE NOSYMBOLIC")

	got := examineOutput(t, c, "EXAMINE/INSTRUCTION .PC")
	if strings.Contains(got, "DBGDIS") {
		t.Errorf("NOSYMBOLIC still names the location: %q", got)
	}

	dispatch(t, c, "SET MODE SYMBOLIC")

	if got := examineOutput(t, c, "EXAMINE/INSTRUCTION .PC"); !strings.Contains(got, `DBGDIS\START\%LINE 47`) {
		t.Errorf("SYMBOLIC: %q", got)
	}

	// 20 in decimal is 14 in hexadecimal.
	dispatch(t, c, "SET RADIX DECIMAL")

	pc := examineOutput(t, c, "EXAMINE/INSTRUCTION %LINE 47+10")
	dispatch(t, c, "CANCEL RADIX")

	hex := examineOutput(t, c, "EXAMINE/INSTRUCTION %LINE 47+0A")

	if pc == "" || pc != hex {
		t.Errorf("%%LINE 47+10 in decimal (%q) differs from +A in hexadecimal (%q)", pc, hex)
	}

	if err := c.Debugger.Dispatch("SET RADIX NONSENSE"); err == nil {
		t.Error("SET RADIX NONSENSE succeeded")
	}

	if err := c.Debugger.Dispatch("SET MODE NONSENSE"); err == nil {
		t.Error("SET MODE NONSENSE succeeded")
	}

	// The data forms of EXAMINE come later.
	if err := c.Debugger.Dispatch("EXAMINE R0"); err == nil {
		t.Error("EXAMINE R0 succeeded before the data forms exist")
	}
}

// TestRegistersInExpressions: a register is its contents in an address
// expression (R1+4, SP-8), and so is "." or "@" before one (.PC), so the
// debugger's commands take them anywhere an address goes: SET BREAK .PC+1,
// and a WHEN condition's R1.
func TestRegistersInExpressions(t *testing.T) {
	c := stoppedAt(t, dbgImagePath(t, "dbgdis.exe"), "%LINE 47")

	pc := c.CPU.GPR(15)

	for _, expr := range []string{".PC", "PC", "%PC", "@PC", "pc"} {
		got, err := c.EvalWhole(expr)
		if err != nil || got != pc {
			t.Errorf("%s = %08X, %v; want %08X", expr, got, err, pc)
		}
	}

	if got, err := c.EvalWhole("R0+4"); err != nil || got != c.CPU.GPR(0)+4 {
		t.Errorf("R0+4 = %08X, %v", got, err)
	}

	// A "." before a register is the register's contents, the address
	// EXAMINE then shows; before a symbol it is the longword stored there.
	if got, err := c.EvalWhole(".DBGDIS\\COUNT"); err != nil || got != 0 {
		t.Errorf(".DBGDIS\\COUNT = %08X, %v; want its contents, 0", got, err)
	}

	// An unreadable address (page 0 is never mapped) is the debugger's
	// NOACCESSR.
	if _, err := c.EvalWhole(".0"); err == nil || !strings.Contains(err.Error(), "NOACCESSR") {
		t.Errorf("an unreadable address gave %v, want NOACCESSR", err)
	}
}
