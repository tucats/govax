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
		// What isn't a substitution stays as it is: no closing
		// apostrophe, no name, a comment.
		{"X := 'NAME", "X := 'NAME"},
		{"X := ' NAME'", "X := ' NAME'"},
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

	if _, err := substituteApostrophes("X = 'SELF'", symbols, nil); !hasStatus(err, vmserrors.CLI_SYMDEPTH) {
		t.Errorf("a symbol whose value names itself: %v, want CLI_SYMDEPTH", err)
	}

	if _, err := substituteApostrophes("X := 'F$NOSUCH(1)'", symbols, nil); !hasStatus(err, vmserrors.CLI_IVFNAM) {
		t.Errorf("an unknown lexical function: %v, want CLI_IVFNAM", err)
	}
}

// TestSubstituteAmpersands: phase 2's substitution, once, after a
// delimiter, not in quotes, and with the value's case kept while the rest
// of the line is uppercased.
func TestSubstituteAmpersands(t *testing.T) {
	symbols := substSymbols(t)

	for _, tc := range []struct {
		line, want string
		upcased    bool
	}{
		{"TYPE &B", "TYPE MYFILE.DAT", true},
		{"define y &lc", "DEFINE Y string", true},
		{"type &P1 x", "TYPE  X", true},
		{"X = &A", "X = 'MAC'", true},
		{`A = "&B"`, `A = "&B"`, false},
		{"X := A&B", "X := A&B", false},
		{`type "a b" &b ! &b`, `TYPE "a b" MYFILE.DAT ! &B`, true},
	} {
		got, upcased := symbols.substituteAmpersands(tc.line)
		if got != tc.want || upcased != tc.upcased {
			t.Errorf("%s = %q, %v; want %q, %v", tc.line, got, upcased, tc.want, tc.upcased)
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

	// An &SYMBOL in a parameter keeps its case.
	if got := show("@" + proc + " &NAME"); got != "  P1 = \"Mixed\"\n" {
		t.Errorf("@proc &NAME: %q", got)
	}

	// In a string assignment: 'NAME' is uppercased with the text around
	// it, &NAME isn't (13.18.3).
	show("X := a'NAME'b")

	if got := show("SHOW SYMBOL X"); got != "  X = \"AMIXEDB\"\n" {
		t.Errorf("X := a'NAME'b: %q", got)
	}

	show("X := a &NAME b")

	if got := show("SHOW SYMBOL X"); got != "  X = \"A Mixed B\"\n" {
		t.Errorf("X := a &NAME b: %q", got)
	}

	// 12.13.4's EXEC example: an alias's value isn't scanned for
	// apostrophes, unless the alias itself is between them.
	if err := d.Dispatch("EXEC"); err == nil {
		t.Error("EXEC: no error, want the unrecognized 'MAC2'")
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
