package dclshell

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/gopackages/app-cli/settings"
	"github.com/tucats/govax/internal/lnm"
	"github.com/tucats/govax/internal/vmsdef"
	"github.com/tucats/govax/internal/vmserrors"
)

// Tests of the lexical functions (dcllexical.go and the dcllex*.go files),
// from the User's Manual's chapter 15 examples where it has them.

// lexCase is one lexical function call and the value it should have:
// integer says whether that's an integer, or the error it should be
// instead: code, a govax status, or vms, a VMS condition value (a system
// service's status).
type lexCase struct {
	text    string
	integer bool
	want    string
	code    uint32
	vms     string
}

// checkLexical evaluates each case's expression on c's symbols, checking
// its value or its error.
func checkLexical(t *testing.T, c *Interpreter, cases []lexCase) {
	t.Helper()

	for _, tc := range cases {
		v, err := evaluateDCLExpression(upcaseOutsideQuotes(tc.text), &c.dclSymbols, c)

		switch {
		case tc.vms != "":
			if want := vmsdef.Symbols[tc.vms]; err == nil || ConditionValue(err) != want {
				t.Errorf("%s: %v (value %q), want %s", tc.text, err, v.String(), tc.vms)
			}
		case tc.code != 0:
			if !hasStatus(err, tc.code) {
				t.Errorf("%s: %v (value %q), want %v", tc.text, err, v.String(), vmserrors.New(tc.code))
			}
		case err != nil:
			t.Errorf("%s: %v", tc.text, err)
		case v.integer != tc.integer || v.String() != tc.want:
			t.Errorf("%s = %q (integer %v), want %q (integer %v)", tc.text, v.String(), v.integer, tc.want, tc.integer)
		}
	}
}

// lexConsole is a console with its process, and symbols the cases use.
func lexConsole(t *testing.T) *Interpreter {
	t.Helper()

	c, _ := newTestInterpreter(t)

	for _, a := range []struct{ name, op, text string }{
		{"PROT", "=", `"SYSTEM=RWED, OWNER=RWED, GROUP=RE, WORLD"`},
		{"FILE", "=", `"LOGIN.COM;3"`},
		{"UIC", "=", `"[WRITERS,SMITH]"`},
		{"NUM", "=", "12"},
		{"NUMSTR", "=", `"12"`},
		{"HEXSTR", "=", `"-%X1F"`},
		{"WORD", "=", `"twelve"`},
	} {
		if err := c.AssignSymbol(a.name, a.op, a.text); err != nil {
			t.Fatalf("%s %s %s: %v", a.name, a.op, a.text, err)
		}
	}

	return c
}

// TestLexicalStrings: F$EXTRACT, F$LOCATE, F$ELEMENT, F$CVSI, F$CVUI, and
// F$TYPE, with the User's Manual's examples (15.6, 15.7).
func TestLexicalStrings(t *testing.T) {
	checkLexical(t, lexConsole(t), []lexCase{
		// 15.6.1: a semicolon's offset, or the length when there's none.
		{text: `F$LOCATE(";", FILE)`, integer: true, want: "9"},
		{text: `F$LOCATE(";", "LOGIN.COM")`, integer: true, want: "9"},
		{text: `F$LOCATE("", "ABC")`, integer: true, want: "0"},

		// 15.6.2: the group of a UIC, and the fourth element of a
		// protection.
		{text: `F$EXTRACT(1, F$LOCATE(",", UIC) - 1, UIC)`, want: "WRITERS"},
		{text: `F$EXTRACT(4, 100, "abcdef")`, want: "ef"},
		{text: `F$EXTRACT(10, 2, "abc")`, want: ""},
		{text: `F$EXTRACT(-1, 2, "abc")`, want: ""},
		{text: `F$EXTRACT(1, -1, "abc")`, want: "bc"},
		{text: `F$ELEMENT(3, ",", PROT)`, want: " WORLD"},
		{text: `F$ELEMENT(0, "/", "A/B/C")`, want: "A"},
		{text: `F$ELEMENT(5, "/", "A/B/C")`, want: "/"},
		{text: `F$ELEMENT(1, "/", "A//C")`, want: ""},
		{text: `F$ELEMENT(0, "/", "")`, want: ""},
		{text: `F$ELEMENT(0, "//", "A")`, code: vmserrors.CLI_STRTOOLNG},
		{text: `F$ELEMENT(-1, "/", "A")`, code: vmserrors.CLI_INVRANGE},

		// 15.7: bit fields, signed and unsigned, from the string's
		// first bit.
		{text: `F$CVUI(0, 8, "A")`, integer: true, want: "65"},
		{text: `F$CVUI(0, 16, "AB")`, integer: true, want: "16961"},
		{text: `F$CVUI(4, 8, "AB")`, integer: true, want: "36"},
		{text: `F$CVSI(0, 8, "` + "\xff" + `")`, integer: true, want: "-1"},
		{text: `F$CVUI(0, 8, "` + "\xff" + `")`, integer: true, want: "255"},
		{text: `F$CVSI(0, 3, "` + "\x04" + `")`, integer: true, want: "-4"},
		{text: `F$CVUI(0, 9, "A")`, code: vmserrors.CLI_INVRANGE},
		{text: `F$CVUI(0, 33, "ABCDE")`, code: vmserrors.CLI_INVRANGE},

		// 15.7.1 and 15.7.3: a symbol's type, "" for none; a string
		// that is an integer is INTEGER.
		{text: "F$TYPE(NUM)", want: "INTEGER"},
		{text: "F$TYPE(NUMSTR)", want: "INTEGER"},
		{text: "F$TYPE(HEXSTR)", want: "INTEGER"},
		{text: "F$TYPE(WORD)", want: "STRING"},
		{text: "F$TYPE(NOSUCH)", want: ""},
		{text: "F$TYPE(nu)", want: ""},
		{text: `F$TYPE("NUM")`, code: vmserrors.CLI_ARGREQ},

		// 15.7.2's example: F$INTEGER of an expression's text.
		{text: `F$INTEGER("9 + 7")`, integer: true, want: "0"},
		{text: `F$INTEGER(9 + 7)`, integer: true, want: "16"},
	})
}

// TestLexicalEdit: F$EDIT's edits, which leave quoted text alone.
func TestLexicalEdit(t *testing.T) {
	checkLexical(t, lexConsole(t), []lexCase{
		{text: `F$EDIT("  a  b	c  ", "TRIM")`, want: "a  b\tc"},
		{text: `F$EDIT("  a  b	c  ", "COMPRESS")`, want: " a b c "},
		{text: `F$EDIT("  a  b	c  ", "COLLAPSE")`, want: "abc"},
		{text: `F$EDIT("  a  b	c  ", "TRIM,COMPRESS")`, want: "a b c"},
		{text: `F$EDIT("  a  b	c  ", "compress, trim")`, want: "a b c"},
		{text: `F$EDIT("abc", "UPCASE")`, want: "ABC"},
		{text: `F$EDIT("ABC", "LOWERCASE")`, want: "abc"},
		{text: `F$EDIT("copy a b ! the files", "UNCOMMENT")`, want: "copy a b "},
		{text: `F$EDIT("copy a b ! the files", "UNCOMMENT,TRIM")`, want: "copy a b"},
		{text: `F$EDIT("a ""x  y"" b", "COLLAPSE,UPCASE")`, want: `A"x  y"B`},
		{text: `F$EDIT("say ""!"" ! now", "UNCOMMENT")`, want: `say "!" `},
		{text: `F$EDIT(" ""a"" ", "TRIM")`, want: `"a"`},
		{text: `F$EDIT("abc", "")`, want: "abc"},
		{text: `F$EDIT("abc", "SHOUT")`, code: vmserrors.CLI_IVKEYW},
	})
}

// TestLexicalLogicalNames: F$TRNLNM's arguments and items (15.5), and
// F$LOGICAL.
func TestLexicalLogicalNames(t *testing.T) {
	c := lexConsole(t)

	eqv := []lnm.Equivalence{{Value: "DUA0:[JONES]EMPLOYEE_NAMES.DAT"}, {Value: "DISK1:", Attrs: lnm.AttrConcealed}}
	if _, err := c.Logicals.Define(lnm.ProcessTableName, "NAMES", lnm.Supervisor, 0, eqv); err != nil {
		t.Fatal(err)
	}

	checkLexical(t, c, []lexCase{
		{text: `F$TRNLNM("NAMES")`, want: "DUA0:[JONES]EMPLOYEE_NAMES.DAT"},
		{text: `F$TRNLNM("names")`, want: "DUA0:[JONES]EMPLOYEE_NAMES.DAT"},
		{text: `F$TRNLNM("names",,,,"CASE_SENSITIVE")`, want: ""},
		{text: `F$TRNLNM("NAMES",, 1)`, want: "DISK1:"},
		{text: `F$TRNLNM("NAMES",, 2)`, want: ""},
		{text: `F$TRNLNM("NAMES",,,,, "MAX_INDEX")`, integer: true, want: "1"},
		{text: `F$TRNLNM("NAMES",, 1,,, "CONCEALED")`, want: "TRUE"},
		{text: `F$TRNLNM("NAMES",,,,, "CONCEALED")`, want: "FALSE"},
		{text: `F$TRNLNM("NAMES",,,,, "LENGTH")`, integer: true, want: "30"},
		{text: `F$TRNLNM("NAMES",,,,, "ACCESS_MODE")`, want: "SUPERVISOR"},
		{text: `F$TRNLNM("NAMES",,,,, "TABLE_NAME")`, want: "LNM$PROCESS_TABLE"},
		{text: `F$TRNLNM("NAMES",,,,, "TABLE")`, want: "FALSE"},
		{text: `F$TRNLNM("NAMES", "LNM$SYSTEM")`, want: ""},
		{text: `F$TRNLNM("NAMES", "LNM$PROCESS")`, want: "DUA0:[JONES]EMPLOYEE_NAMES.DAT"},
		{text: `F$TRNLNM("NAMES",,, "EXECUTIVE")`, want: ""},
		{text: `F$TRNLNM("NOSUCH")`, want: ""},
		{text: `F$TRNLNM("NAMES", "NO$SUCH_TABLE")`, want: ""},
		{text: `F$TRNLNM("NAMES",,, "USERS")`, code: vmserrors.CLI_IVKEYW},
		{text: `F$TRNLNM("NAMES",,,,, "WHAT")`, code: vmserrors.CLI_IVKEYW},
		{text: `F$LOGICAL("NAMES")`, want: "DUA0:[JONES]EMPLOYEE_NAMES.DAT"},
		{text: `F$LOGICAL("NOSUCH")`, want: ""},
	})
}

// TestLexicalMessage: F$MESSAGE's text for a condition value, its
// directives left as they are.
func TestLexicalMessage(t *testing.T) {
	checkLexical(t, lexConsole(t), []lexCase{
		{text: "F$MESSAGE(1)", want: "%SYSTEM-S-NORMAL, normal successful completion"},
		{text: "F$MESSAGE(%X10000001)", want: "%SYSTEM-S-NORMAL, normal successful completion"},
		{text: "F$MESSAGE(%X38090)", want: "%CLI-W-IVVERB, unrecognized command verb - check validity and spelling"},
		{text: "F$MESSAGE(2)", want: "%NONAME-E-NOMSG, Message number 00000002"},
	})
}

// TestLexicalConfig: F$CONFIG, govax's own, reads the configuration: a
// key's value (whatever case it's given in), whether it exists, and an
// operation it doesn't know.
func TestLexicalConfig(t *testing.T) {
	const key = "vax.test.lexical.config"

	if settings.Exists(key) {
		t.Fatalf("%s is already set", key)
	}

	settings.Set(key, "Some Value")
	t.Cleanup(func() { _ = settings.Delete(key) })

	checkLexical(t, lexConsole(t), []lexCase{
		{text: `F$CONFIG("vax.test.lexical.config")`, want: "Some Value"},
		{text: `F$CONFIG("VAX.TEST.LEXICAL.CONFIG", "VALUE")`, want: "Some Value"},
		{text: `F$CONFIG("vax.test.lexical.config", "exists")`, integer: true, want: "1"},
		{text: `F$CONFIG("vax.test.lexical.nosuch", "EXISTS")`, integer: true, want: "0"},
		{text: `F$CONFIG("vax.test.lexical.nosuch")`, want: ""},
		{text: `F$CONF("vax.test.lexical.config")`, want: "Some Value"},
		{text: `F$CON("vax.test.lexical.config")`, code: vmserrors.CLI_ABFNAM},
		{text: `F$CONFIG("vax.test.lexical.config", "SET")`, code: vmserrors.CLI_IVKEYW},
		{text: `F$CONFIG()`, code: vmserrors.CLI_ARGREQ},
	})
}

// TestLexicalHostFiles: F$SEARCH through host files, its streams, and
// F$PARSE of a name with no device to check.
func TestLexicalHostFiles(t *testing.T) {
	c := lexConsole(t)
	dir := t.TempDir()

	for _, name := range []string{"b.txt", "A.TXT", "c.dat"} {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	if err := c.AssignSymbol("D", "=", `"`+dir+`/"`); err != nil {
		t.Fatal(err)
	}

	a, b := filepath.Join(dir, "A.TXT"), filepath.Join(dir, "b.txt")

	checkLexical(t, c, []lexCase{
		{text: `F$SEARCH(D + "*.TXT")`, want: a},
		{text: `F$SEARCH(D + "*.DAT", 1)`, want: filepath.Join(dir, "c.dat")},
		{text: `F$SEARCH(D + "*.TXT")`, want: b},
		{text: `F$SEARCH(D + "*.DAT", 1)`, want: ""},
		{text: `F$SEARCH(D + "*.TXT")`, want: ""},
		{text: `F$SEARCH(D + "*.TXT")`, want: a},
		{text: `F$SEARCH(D + "%.DAT")`, want: filepath.Join(dir, "c.dat")},
		{text: `F$SEARCH(D + "NOSUCH.*")`, want: ""},
		{text: `F$PARSE("X.Y",,,, "SYNTAX_ONLY")`, want: "[000000]X.Y;"},
		{text: `F$PARSE("X.Y",,,"NAME")`, want: "X"},
		{text: `F$PARSE("X.Y",,,"FILE")`, code: vmserrors.CLI_IVKEYW},
		{text: `F$PARSE("X.Y",,,,"QUICK")`, code: vmserrors.CLI_IVKEYW},
	})
}

// TestLexicalWithoutConsole: a subprocess's command interpreter, which
// has no console, has the functions that don't ask about the system.
func TestLexicalWithoutConsole(t *testing.T) {
	symbols := &dclSymbolTable{}

	if v, err := evaluateDCLExpression(`F$EDIT(" A ", "TRIM")`, symbols, nil); err != nil || v.String() != "A" {
		t.Errorf("F$EDIT: %q, %v", v.String(), err)
	}

	if _, err := evaluateDCLExpression("F$TIME()", symbols, nil); !hasStatus(err, vmserrors.CLI_LEXNOTIMPL) {
		t.Errorf("F$TIME: %v, want LEXNOTIMPL", err)
	}

	if _, err := evaluateDCLExpression(`F$TRN("X")`, symbols, nil); err == nil || !strings.Contains(err.Error(), "F$TRNLNM") {
		t.Errorf("F$TRN: %v, want LEXNOTIMPL naming F$TRNLNM", err)
	}
}

// TestLexicalTable: every function the table has is one of VMS's (or
// govax's F$CONFIG), takes a sensible number of arguments, and names a
// symbol argument it has.
func TestLexicalTable(t *testing.T) {
	for name, f := range lexicalFunctions {
		if lexicalName(name) != name {
			t.Errorf("%s isn't one of VMS's lexical functions", name)
		}

		if f.minArgs > f.maxArgs || f.symbolArg > f.maxArgs || f.call == nil {
			t.Errorf("%s: arguments %d to %d, symbol argument %d", name, f.minArgs, f.maxArgs, f.symbolArg)
		}
	}
}

// Tests of the lexical functions (dcllexical.go and the dcllex*.go files),
// from the User's Manual's chapter 15 examples where it has them.
