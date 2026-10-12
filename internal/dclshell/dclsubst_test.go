package dclshell

import (
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
