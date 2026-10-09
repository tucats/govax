package debugger_test

import (
	"bytes"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/console"
	"github.com/tucats/govax/internal/vmserrors"
)

// The machine's symbol table (the one ASM and DEPOSIT use, and the
// assembler's predefined system symbols) is the debugger's: SHOW SYMBOL
// finds a name there when the program's debug symbols don't have it,
// SHOW SYMBOL/ALL and /SYSTEM list it, and CANCEL SYMBOL (or CLEAR
// SYMBOL) removes from it. The console's SHOW SYMBOL is DCL's.

// symbolSession is a console in debugger mode with no program loaded,
// and its output buffer.
func symbolSession(t *testing.T) (*console.Console, *bytes.Buffer) {
	t.Helper()

	c, _ := debugModeConsole(t)

	return c, c.Out.(*bytes.Buffer)
}

func TestShowSymbol(t *testing.T) {
	c, _ := symbolSession(t)

	c.Symbols.Set("MYSYM", 0x1234, console.SymbolUser)

	if out := say(t, c, "SHOW SYMBOL MYSYM"); !strings.Contains(out, "00001234") {
		t.Errorf("output = %q, want the symbol's value", out)
	}
}

func TestShowSymbol_permanentAndLabelAttributes(t *testing.T) {
	c, _ := symbolSession(t)

	c.Symbols.SetQualified("MYSYM", 0x1234, true, false, true)

	out := say(t, c, "SHOW SYMBOL MYSYM")
	if !strings.Contains(out, "permanent") || !strings.Contains(out, "label") {
		t.Errorf("output = %q, want it to report the permanent and label attributes", out)
	}
}

func TestShowSymbol_undefined(t *testing.T) {
	c, _ := symbolSession(t)

	if _, err := sayErr(c, "SHOW SYMBOL NOSUCH"); !errors.Is(err, vmserrors.New(vmserrors.DBG_NOSYMBOL, "NOSUCH")) {
		t.Errorf("err = %v, want DEBUG-E-NOSYMBOL", err)
	}
}

func TestShowSymbolsSystem(t *testing.T) {
	c, _ := symbolSession(t)

	c.Symbols.Set("USERSYM", 1, console.SymbolUser)
	c.Symbols.Set("SYS$SYM", 2, console.SymbolSystem)

	out := say(t, c, "SHOW SYMBOL/SYSTEM")
	if !strings.Contains(out, "SYS$SYM") {
		t.Errorf("output = %q, want the system symbol listed", out)
	}

	if strings.Contains(out, "USERSYM") {
		t.Errorf("output = %q, want the user symbol excluded", out)
	}
}

// showSymbolNames runs command and returns the symbol names its listing
// printed, in order.
func showSymbolNames(t *testing.T, c *console.Console, command string) []string {
	t.Helper()

	var names []string

	for _, line := range strings.Split(strings.TrimSpace(say(t, c, command)), "\n") {
		if f := strings.Fields(line); len(f) > 0 {
			names = append(names, f[0])
		}
	}

	return names
}

// TestShowSymbol_wildcards checks VMS "*" and "%" name filtering in each
// SHOW SYMBOL form, including the predefined system symbols.
func TestShowSymbol_wildcards(t *testing.T) {
	c, _ := symbolSession(t)

	c.Symbols.Set("MYSYM", 1, console.SymbolUser)
	c.Symbols.Set("MYSYS", 2, console.SymbolSystem)
	c.Symbols.Set("MXSYM", 3, console.SymbolUser)
	c.Symbols.Set("PTE$K_UR", 7, console.SymbolUser) // shadows the predefined one

	cases := []struct {
		cmd  string
		want []string
	}{
		{"SHOW SYMBOL MY*", []string{"MYSYM", "MYSYS"}},
		{"SHOW SYMBOL my*", []string{"MYSYM", "MYSYS"}},
		{"SHOW SYMBOLS M%SYM", []string{"MXSYM", "MYSYM"}},
		{"SHOW SYMBOL/ALL MY*", []string{"MYSYM", "MYSYS"}},
		{"SHOW SYMBOL/SYSTEM M*SY%", []string{"MYSYS"}},
		{"SHOW SYMBOL PTE$K_UR*", []string{"PTE$K_UR", "PTE$K_UREW", "PTE$K_URKW", "PTE$K_URSW"}},
		{"SHOW SYMBOL/SYSTEM PTE$K_UR*", []string{"PTE$K_UREW", "PTE$K_URKW", "PTE$K_URSW"}},
		{"SHOW SYMBOL VAX$PR_%%R", []string{"VAX$PR_ICR", "VAX$PR_PMR", "VAX$PR_SBR", "VAX$PR_SLR"}},
		{"SHOW SYMBOL OPC$_MOV%", []string{"OPC$_MOVB", "OPC$_MOVD", "OPC$_MOVF", "OPC$_MOVL", "OPC$_MOVP", "OPC$_MOVQ", "OPC$_MOVW"}},
	}

	for _, tc := range cases {
		if got := showSymbolNames(t, c, tc.cmd); strings.Join(got, " ") != strings.Join(tc.want, " ") {
			t.Errorf("%s = %v, want %v", tc.cmd, got, tc.want)
		}
	}

	if out := say(t, c, "SHOW SYMBOL PTE$K_UR"); !strings.Contains(out, "00000007") {
		t.Errorf("SHOW SYMBOL PTE$K_UR = %q; want the table's own 7", out)
	}

	if _, err := sayErr(c, "SHOW SYMBOL NOSUCH*"); !errors.Is(err, vmserrors.New(vmserrors.DBG_NOSYMBOL, "NOSUCH*")) {
		t.Errorf("SHOW SYMBOL NOSUCH*: err = %v, want DEBUG-E-NOSYMBOL", err)
	}

	for _, cmd := range []string{"SHOW SYMBOL/ALL NOSUCH%", "SHOW SYMBOL/SYSTEM MX*"} {
		if _, err := sayErr(c, cmd); !errors.Is(err, vmserrors.New(vmserrors.CLI_UNDEFSYM)) {
			t.Errorf("%s: err = %v, want CLI-E-UNDEFSYM", cmd, err)
		}
	}

	if _, err := sayErr(c, "SHOW SYMBOL/ALL/SYSTEM"); err == nil {
		t.Error("SHOW SYMBOL/ALL/SYSTEM: no error")
	}
}

// TestShowSymbol_predefined checks that a single predefined system symbol
// shows as one, and that the unfiltered listings include them.
func TestShowSymbol_predefined(t *testing.T) {
	c, _ := symbolSession(t)

	if out := say(t, c, "SHOW SYMBOL VAX$PR_SBR"); !strings.Contains(out, "0000000C") || !strings.Contains(out, "predefined") {
		t.Errorf("output = %q, want VAX$PR_SBR's value 0000000C, predefined", out)
	}

	for _, cmd := range []string{"SHOW SYMBOL/ALL", "SHOW SYMBOL/SYSTEM"} {
		if names := showSymbolNames(t, c, cmd); !slices.Contains(names, "PTE$K_KW") {
			t.Errorf("%s doesn't list PTE$K_KW", cmd)
		}
	}
}

// TestCancelSymbol removes symbols from the machine's table: one by name,
// the temporary ones, and every user symbol; CLEAR is CANCEL's synonym.
// System symbols stay.
func TestCancelSymbol(t *testing.T) {
	c, _ := symbolSession(t)

	c.Symbols.SetQualified("PERM", 1, true, false, false)
	c.Symbols.SetQualified("TEMP", 2, false, false, false)
	c.Symbols.Set("ONE", 3, console.SymbolUser)
	c.Symbols.Set("SYS$KEEP", 4, console.SymbolSystem)

	defined := func(name string) bool {
		_, ok := c.Symbols.Get(name)

		return ok
	}

	say(t, c, "CANCEL SYMBOL ONE")

	if defined("ONE") {
		t.Error("CANCEL SYMBOL ONE: ONE is still defined")
	}

	say(t, c, "CLEAR SYMBOL/TEMPORARY")

	if !defined("PERM") || defined("TEMP") {
		t.Errorf("CLEAR SYMBOL/TEMPORARY: PERM %v (want true), TEMP %v (want false)", defined("PERM"), defined("TEMP"))
	}

	say(t, c, "CANCEL SYMBOLS/ALL")

	if defined("PERM") || !defined("SYS$KEEP") {
		t.Errorf("CANCEL SYMBOLS/ALL: PERM %v (want false), SYS$KEEP %v (want true)", defined("PERM"), defined("SYS$KEEP"))
	}

	if _, err := sayErr(c, "CANCEL SYMBOL"); err == nil {
		t.Error("CANCEL SYMBOL with no name: no error")
	}
}
