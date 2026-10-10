package console

import (
	"testing"

	"github.com/tucats/govax/internal/vmserrors"
)

// exprSymbols is a symbol table with the symbols the User's Manual's
// expression examples use (sections 12.4 to 12.9).
func exprSymbols(t *testing.T) *dclSymbolTable {
	t.Helper()

	symbols := &dclSymbolTable{}

	for _, a := range []struct{ name, op, text string }{
		{"COUNT", "=", "3"},
		{"TEMP", "=", `"CAT"`},
		{"DOG2", "=", `"No tag, light brown, 30 lbs."`},
		{"LAST_NAME", "=", `"WHITFIELD"`},
		{"BALANCE", "=", "-15237"},
		{"STATUS", "=", "1"},
		{"STAT1", "=", `"TRUE"`},
		{"STAT2", "=", `"FALSE"`},
		{"A", "=", `""`},
		{"B", "=", "2"},
		{"BUG", "=", `"BUMBLEBEE"`},
		{"FIVE", "=", `"5"`},
		{"LONGN*AME", "=", "5"},
	} {
		if err := symbols.assign(a.name, a.op, a.text); err != nil {
			t.Fatalf("%s %s %s: %v", a.name, a.op, a.text, err)
		}
	}

	return symbols
}

// TestDCLExpression evaluates the User's Manual's examples (each from the
// section named), and the rules they illustrate.
func TestDCLExpression(t *testing.T) {
	symbols := exprSymbols(t)

	for _, tc := range []struct {
		text    string
		integer bool
		want    string
	}{
		// 12.4 and 12.5: integers and symbols.
		{"COUNT + 1", true, "4"},
		{"1 + 2 + 3", true, "6"},
		{`A + B`, true, "2"},

		// 12.6: strings, concatenation, and reduction.
		{`"Saturday, " + "Sunday"`, false, "Saturday, Sunday"},
		{`"THE" + TEMP`, false, "THECAT"},
		{`DOG2 - ", 30 lbs."`, false, "No tag, light brown"},
		{`"abcabc" - "b"`, false, "acabc"},
		{`"abc" - "z"`, false, "abc"},
		{`"Type ""YES"" or ""NO"""`, false, `Type "YES" or "NO"`},
		{`"MONTHLY REPORT FOR" + " DECEMBER 1999"`, false, "MONTHLY REPORT FOR DECEMBER 1999"},

		// 12.6.4: string comparisons.
		{`LAST_NAME .EQS. "Hill"`, true, "0"},
		{`LAST_NAME .GES. "HILL"`, true, "1"},
		{`LAST_NAME .GTS. "HILL"`, true, "1"},
		{`LAST_NAME .LES. "HILL"`, true, "0"},
		{`LAST_NAME .LTS. "HILL"`, true, "0"},
		{`LAST_NAME .NES. "HILL"`, true, "1"},
		{`"dogs" .GTS. "dog"`, true, "1"},
		{`10 .EQS. "10"`, true, "1"},

		// 12.7: numbers, radixes, arithmetic.
		{"13", true, "13"},
		{"-15237", true, "-15237"},
		{"%XD", true, "13"},
		{"-%X3B85", true, "-15237"},
		{"%o17 + %d10", true, "25"},
		{"142 * 14", true, "1988"},
		{"1988 / 14", true, "142"},
		{"179 - 15416", true, "-15237"},
		{"8 / 3", true, "2"},
		{"-8 / 3", true, "-2"},
		{"2147483647 + 1", true, "-2147483648"},
		{"4294967297", true, "1"},
		{"- - 3", true, "3"},
		{"1+-2", true, "-1"},

		// 12.7.4: numeric comparisons.
		{"BALANCE .EQ. -15237", true, "1"},
		{"BALANCE .GE. -15237", true, "1"},
		{"BALANCE .GT. -15237", true, "0"},
		{"BALANCE .LE. -15237", true, "1"},
		{"BALANCE .LT. -15237", true, "0"},
		{"BALANCE .NE. -15237", true, "0"},
		{"1 .eq. 1", true, "1"},
		{`"10" .LT. 9`, true, "0"},

		// 12.8: logical operations.
		{".NOT. STATUS", true, "-2"},
		{"STAT1 .AND. STAT2", true, "0"},
		{"STAT1 .OR. STAT2", true, "1"},
		{"3 .AND. 5", true, "1"},
		{"3 .OR. 4", true, "7"},

		// 12.8.4: lexical functions.
		{`F$LENGTH("BUMBLEBEE") + 1`, true, "10"},
		{"F$LENGTH(BUG)", true, "9"},
		{`F$LENGTH(BUG + "S")`, true, "10"},
		{`f$length ("abc")`, true, "3"},
		{`F$LEN("abc")`, true, "3"},
		{`F$INTEGER("-9" + 23)`, true, "14"},
		{"F$STRING(65)", false, "65"},
		{"F$STRING(-1)", false, "-1"},

		// 12.8.5: precedence and parentheses.
		{"4 * (6 + 2)", true, "32"},
		{"4 * 6 + 2", true, "26"},
		{".NOT. 1 .EQ. 0", true, "-1"},
		{"5 .GT. 3 .AND. 2 .LT. 1", true, "0"},
		{`"abc" .EQS. "ab" + "c"`, true, "1"},

		// 12.9: conversions.
		{`"123" + 0`, true, "123"},
		{`"12XY" + 0`, true, "0"},
		{`"Test" + 0`, true, "1"},
		{`"hello" + 0`, true, "0"},
		{`"yes" * 1`, true, "1"},
		{`"12" + "3"`, false, "123"},
		{`"12" * "3"`, true, "36"},
		{`FIVE + 1`, true, "6"},
		{`FIVE + "1"`, false, "51"},
		{`-"5"`, true, "-5"},

		// Symbols abbreviate in an expression too (12.2.2).
		{"LONGN + 1", true, "6"},

		// A comment ends the expression.
		{"1 ! the rest", true, "1"},
	} {
		v, err := evaluateDCLExpression(tc.text, symbols, nil)
		if err != nil {
			t.Errorf("%s: %v", tc.text, err)

			continue
		}

		if v.integer != tc.integer || v.String() != tc.want {
			t.Errorf("%s = %q (integer %v), want %q (integer %v)", tc.text, v.String(), v.integer, tc.want, tc.integer)
		}
	}
}

// TestDCLExpression_errors: each malformed expression's message (the
// probe in testdata/dcl50 checks them against VMS 7.3, TestProbe50Oracle).
func TestDCLExpression_errors(t *testing.T) {
	symbols := exprSymbols(t)

	for _, tc := range []struct {
		text string
		code uint32
	}{
		{"", vmserrors.CLI_EXPSYN},
		{"1 +", vmserrors.CLI_EXPSYN},
		{"(1 + 2", vmserrors.CLI_EXPSYN},
		{"1 + 2)", vmserrors.CLI_SYMDEL},
		{"1 2", vmserrors.CLI_EXPSYN},
		{"1 .NOT. 2", vmserrors.CLI_EXPSYN},
		{"NOSUCH + 1", vmserrors.CLI_UNDSYM},
		{"1 .FOO. 2", vmserrors.CLI_IVOPER},
		{"1 .E. 2", vmserrors.CLI_IVOPER},
		{"1.5", vmserrors.CLI_IVOPER},
		{"12AB", vmserrors.CLI_IVCHAR},
		{"%XG", vmserrors.CLI_IVCHAR},
		{"%Q1", vmserrors.CLI_IVCHAR},
		{"F$NOSUCH(1)", vmserrors.CLI_IVFNAM},
		{`F$L("abc")`, vmserrors.CLI_ABFNAM},
		{`F$TIME()`, vmserrors.CLI_LEXNOTIMPL},
		{"F$LENGTH", vmserrors.CLI_UNDSYM},
		{`F$LENGTH("a"`, vmserrors.CLI_SYMDEL},
		{"F$LENGTH()", vmserrors.CLI_ARGREQ},
		{`F$LENGTH(,)`, vmserrors.CLI_ARGREQ},
		{`F$LENGTH("a", "b")`, vmserrors.CLI_SYMDEL},
		{`F$LENGTH(NOSUCH)`, vmserrors.CLI_UNDSYM},
	} {
		if _, err := evaluateDCLExpression(tc.text, symbols, nil); !hasStatus(err, tc.code) {
			t.Errorf("%s: %v, want %v", tc.text, err, vmserrors.New(tc.code))
		}
	}
}

// TestDCLExpression_vms: what VMS 7.3 answered where the manual is
// silent (testdata/dcl50).
func TestDCLExpression_vms(t *testing.T) {
	symbols := exprSymbols(t)

	for _, tc := range []struct {
		text    string
		integer bool
		want    string
	}{
		{"7 / 0", true, "2147483647"},
		{`"abc`, false, "abc"},
		{`""quoted""`, false, "QUOTED"},
		{"1 .EQ 1", true, "1"},
		{"1 .EQ. .NOT. 0", true, "0"},
		{`" 12" + 0`, true, "12"},
		{`"%X10" + 0`, true, "16"},
	} {
		v, err := evaluateDCLExpression(tc.text, symbols, nil)
		if err != nil {
			t.Errorf("%s: %v", tc.text, err)

			continue
		}

		if v.integer != tc.integer || v.String() != tc.want {
			t.Errorf("%s = %q (integer %v), want %q (integer %v)", tc.text, v.String(), v.integer, tc.want, tc.integer)
		}
	}
}

// TestLexicalAbbreviation: a lexical function's name may be shortened to
// any prefix that names only one of VMS's lexical functions.
func TestLexicalAbbreviation(t *testing.T) {
	for _, tc := range []struct {
		name string
		code uint32
	}{
		{"F$LEN", 0},
		{"f$length", 0},
		{"F$STR", 0},
		{"F$IN", 0},
		{"F$I", vmserrors.CLI_ABFNAM},
		{"F$L", vmserrors.CLI_ABFNAM},
		{"F$S", vmserrors.CLI_ABFNAM},
		{"F$LENGTHY", vmserrors.CLI_IVFNAM},
		{"F$TRNLNM", 0},
		{"F$GETQUI", vmserrors.CLI_LEXNOTIMPL},
	} {
		_, err := findLexicalFunction(tc.name)
		if (tc.code == 0 && err != nil) || (tc.code != 0 && !hasStatus(err, tc.code)) {
			t.Errorf("%s: %v, want %v", tc.name, err, vmserrors.New(tc.code))
		}
	}
}

// TestDCLExpression_message: an assignment's message shows the part of
// the expression in question, uppercased as the command line is, on its
// own line between backslashes, as DCL does.
func TestDCLExpression_message(t *testing.T) {
	err := exprSymbols(t).assign("X", "=", "1 + nosuch")

	want := "DCL-W-UNDSYM, undefined symbol - check validity and spelling\n \\NOSUCH\\"
	if err == nil || err.Error() != want {
		t.Errorf("message = %v, want %q", err, want)
	}
}
