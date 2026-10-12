package console

import (
	"strings"
	"testing"

	"github.com/tucats/govax/internal/vmserrors"
)

// TestDispatch_substitution runs the substitutions through the console's
// dispatcher: a procedure's parameter, an alias whose value has
// apostrophes, and an &SYMBOL whose value keeps its case.
func TestDispatch_substitution(t *testing.T) {
	d, _, buf := newCommandDispatcher(t)
	dir := t.TempDir()

	for _, line := range []string{
		`NAME = "Mixed"`,
		`MAC2 = "SHOW SYMBOL MAC2"`,
		`EXEC = "'MAC2'"`,
		`SUM = 1 + 'F$LENGTH(NAME)'`,
	} {
		if err := d.Dispatch(line); err != nil {
			t.Fatalf("%s: %v", line, err)
		}
	}

	show := func(line string) string {
		t.Helper()
		buf.Reset()

		if err := d.Dispatch(line); err != nil {
			t.Fatalf("%s: %v", line, err)
		}

		return buf.String()
	}

	if got := show("SHOW SYMBOL SUM"); !strings.HasPrefix(got, "  SUM = 6 ") {
		t.Errorf("SUM = %q, want 6", got)
	}

	// 14.2.3: a symbol as a procedure's parameter, uppercased unless
	// it's quoted.
	proc := writeProcedure(t, dir, "p.com", "$ SHOW SYMBOL P1")
	if got := show("@" + proc + " 'NAME'"); got != "  P1 = \"MIXED\"\n" {
		t.Errorf("@proc 'NAME': %q", got)
	}

	if got := show("@" + proc + ` "''NAME'"`); got != "  P1 = \"Mixed\"\n" {
		t.Errorf(`@proc "''NAME'": %q`, got)
	}

	// VMS 7.3 doesn't substitute an &SYMBOL in an @ command's parameters
	// or a string assignment (testdata/dcl50).
	if got := show("@" + proc + " &NAME"); got != "  P1 = \"&NAME\"\n" {
		t.Errorf("@proc &NAME: %q", got)
	}

	// In a string assignment, 'NAME' is uppercased with the text around
	// it.
	show("X := a'NAME'b")

	if got := show("SHOW SYMBOL X"); got != "  X = \"AMIXEDB\"\n" {
		t.Errorf("X := a'NAME'b: %q", got)
	}

	show("X := a &NAME b")

	if got := show("SHOW SYMBOL X"); got != "  X = \"A &NAME B\"\n" {
		t.Errorf("X := a &NAME b: %q", got)
	}

	// 12.13.4's EXEC example: an alias's value isn't scanned for
	// apostrophes, unless the alias itself is between them. VMS 7.3 gave
	// NOCOMD: the value doesn't start with a letter.
	if err := d.Dispatch("EXEC"); !hasStatus(err, vmserrors.CLI_NOCOMD) {
		t.Errorf("EXEC: %v, want CLI_NOCOMD", err)
	}

	if got := show("'EXEC'"); got != "  MAC2 = \"SHOW SYMBOL MAC2\"\n" {
		t.Errorf("'EXEC': %q", got)
	}

	// &SYMBOL keeps its value's case, and 'SYMBOL' doesn't (13.18.3's
	// example).
	if err := d.Dispatch(`X = "string"`); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct{ define, want string }{
		{"DEFINE Y 'X'", `"Y" = "STRING"`},
		{"DEFINE Y &X", `"Y" = "string"`},
	} {
		show(tc.define)

		if got := show("SHOW LOGICAL Y"); !strings.Contains(got, tc.want) {
			t.Errorf("%s: SHOW LOGICAL Y = %q, want %s", tc.define, got, tc.want)
		}
	}
}
