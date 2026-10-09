package console

import (
	"strings"
	"testing"

	"github.com/tucats/govax/internal/vmserrors"
)

// substSymbols is a symbol table with the symbols the User's Manual's
// substitution examples use (sections 12.4, 12.12, and 12.13).
func substSymbols(t *testing.T) *dclSymbolTable {
	t.Helper()

	symbols := &dclSymbolTable{}

	for _, a := range []struct{ name, op, text string }{
		{"NAME", "=", `"MYFILE"`},
		{"TYPE", "=", `".DAT"`},
		{"FRED", "=", `"FRED"`},
		{"PN", "=", `"PRINT/NOTIFY"`},
		{"FILE1", "=", `"[BOLIVAR]TEST_CASE.TXT"`},
		{"NUM", "=", "1"},
		{"COUNT", "=", "1"},
		{"MAC", "=", `"5"`},
		{"A", "=", `"'MAC'"`},
		{"SELF", "=", `"'SELF'"`},
		{"LC", "=", `"string"`},
		{"B", "=", `"MYFILE.DAT"`},
		{"Q", "=", `"say ""hi"""`},
	} {
		if err := symbols.assign(a.name, a.op, a.text); err != nil {
			t.Fatalf("%s %s %s: %v", a.name, a.op, a.text, err)
		}
	}

	return symbols
}

// TestSubstituteApostrophes: phase 1, on the User's Manual's examples.
func TestSubstituteApostrophes(t *testing.T) {
	symbols := substSymbols(t)

	for _, tc := range []struct{ line, want string }{
		// 12.4.1: concatenation by apostrophes.
		{"PRINT 'NAME''TYPE'", "PRINT MYFILE.DAT"},
		// 12.4.2: an integer's digits.
		{"BARK := P'COUNT'", "BARK := P1"},
		// 12.12.2: inside quotes, two apostrophes before the name.
		{`MESSAGE = "Creating file ''FRED'.DAT"`, `MESSAGE = "Creating file FRED.DAT"`},
		{`X = "it's 'NAME'"`, `X = "it's 'NAME'"`},
		// 12.12.1: the PN, FILE1, and NUM example's first command.
		{`FILE = "'FILE''NUM''"`, `FILE = "'FILE1'"`},
		// 12.12.2: an undefined symbol is nothing, and left to right.
		{"TYPE 'P''COUNT'", "TYPE 1"},
		{"TYPE &P'COUNT'", "TYPE &P1"},
		// 12.13.4: iterative outside quotes, not inside.
		{"B = 'A'", "B = 5"},
		{`B = "''A'"`, `B = "'MAC'"`},
		// 12.13.5: an undefined symbol.
		{"FILE := MYFILE'FILE_TYPE'", "FILE := MYFILE"},
		// A lexical function between apostrophes.
		{`X := L'F$LENGTH("abc")'`, "X := L3"},
		{`X = "L''F$LENGTH(NAME)'"`, `X = "L6"`},
		// VMS 7.3: the closing apostrophe may be left out, and blanks
		// may follow the opening one (testdata/dcl50).
		{"X := 'NAME", "X := MYFILE"},
		{"X := A'NAME", "X := AMYFILE"},
		{"X := ' NAME'", "X := MYFILE"},
		// What isn't a substitution stays as it is: no name, a comment,
		// one apostrophe inside quotes.
		{"X := ' 1", "X := ' 1"},
		{"X := A ! it's 'NAME'", "X := A ! it's 'NAME'"},
		// The case of the value is kept (the grammar uppercases it
		// later, outside quotes).
		{"DEFINE Y 'LC'", "DEFINE Y string"},
	} {
		got, err := substituteApostrophes(tc.line, symbols, nil)
		if err != nil {
			t.Errorf("%s: %v", tc.line, err)

			continue
		}

		if got != tc.want {
			t.Errorf("%s = %q, want %q", tc.line, got, tc.want)
		}
	}

	if _, err := substituteApostrophes("X = 'SELF'", symbols, nil); !hasStatus(err, vmserrors.CLI_EXPSYN) {
		t.Errorf("a symbol whose value names itself: %v, want CLI_EXPSYN", err)
	}

	if _, err := substituteApostrophes("X := 'F$NOSUCH(1)'", symbols, nil); !hasStatus(err, vmserrors.CLI_IVFNAM) {
		t.Errorf("an unknown lexical function: %v, want CLI_IVFNAM", err)
	}
}

// TestSubstituteAmpersands: phase 2's substitution, once, after a
// delimiter, not in quotes; in a command, the value is one quoted token.
func TestSubstituteAmpersands(t *testing.T) {
	symbols := substSymbols(t)

	for _, tc := range []struct {
		line, want string
		quote      bool
	}{
		{"TYPE &B", `TYPE "MYFILE.DAT"`, true},
		{"DEFINE Y &LC", `DEFINE Y "string"`, true},
		{"TYPE &P1 X", `TYPE "" X`, true},
		{"X = &LC", "X = string", false},
		{"X = &A", "X = 'MAC'", false},
		{`A = "&B"`, `A = "&B"`, true},
		{"X := A&B", "X := A&B", true},
		{`TYPE "a b" &B ! &B`, `TYPE "a b" "MYFILE.DAT" ! &B`, true},
		{"X = &Q", `X = say "hi"`, false},
		{"DEFINE Y &Q", `DEFINE Y "say ""hi"""`, true},
	} {
		if got := symbols.substituteAmpersands(tc.line, tc.quote); got != tc.want {
			t.Errorf("%s = %q, want %q", tc.line, got, tc.want)
		}
	}
}

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
