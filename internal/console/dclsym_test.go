package console

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/vmserrors"
)

// TestDCLText is DCL's treatment of a command's text: uppercased and
// compressed outside quotes, kept as written inside them, and ended by a
// "!" comment.
func TestDCLText(t *testing.T) {
	for _, tc := range []struct {
		text, foreign, value string
	}{
		{"  2 3 +   .  ", "2 3 + .", "2 3 + ."},
		{"dir/size\tsys$login", "DIR/SIZE SYS$LOGIN", "DIR/SIZE SYS$LOGIN"},
		{` ." Mixed  Case" cr`, `." Mixed  Case" CR`, `. Mixed  Case CR`},
		{`say "a ""quoted"" word"`, `SAY "a ""quoted"" word"`, `SAY a "quoted" word`},
		{`abc ! comment "x"`, "ABC", "ABC"},
		{`"! not a comment"`, `"! not a comment"`, "! not a comment"},
		{"", "", ""},
	} {
		if got := dclText(tc.text, true); got != tc.foreign {
			t.Errorf("dclText(%q, true) = %q, want %q", tc.text, got, tc.foreign)
		}

		if got := dclText(tc.text, false); got != tc.value {
			t.Errorf("dclText(%q, false) = %q, want %q", tc.text, got, tc.value)
		}
	}
}

// TestSplitAssignment recognizes each assignment operator, and leaves
// commands alone.
func TestSplitAssignment(t *testing.T) {
	for _, tc := range []struct {
		line, name, op, value string
		ok                    bool
	}{
		{"FORTH :== $FORTH", "FORTH", ":==", " $FORTH", true},
		{"fo*rth:=x", "fo*rth", ":=", "x", true},
		{`G == "s"`, "G", "==", ` "s"`, true},
		{"N = 5", "N", "=", " 5", true},
		{"SET PC=200", "", "", "", false},
		{"EXAMINE R0", "", "", "", false},
		{"@FILE", "", "", "", false},
		{"9X = 1", "", "", "", false},
	} {
		name, op, value, ok := splitAssignment(tc.line)
		if ok != tc.ok || name != tc.name || op != tc.op || value != tc.value {
			t.Errorf("splitAssignment(%q) = %q, %q, %q, %v; want %q, %q, %q, %v",
				tc.line, name, op, value, ok, tc.name, tc.op, tc.value, tc.ok)
		}
	}
}

// TestAssignSymbol defines symbols with each operator and abbreviation,
// and rejects what it can't evaluate.
func TestAssignSymbol(t *testing.T) {
	c := &Console{}

	for _, tc := range []struct{ name, op, text string }{
		{"DIR*ECTORY", ":==", " directory/size"},
		{"greet", "==", ` "Hello, ""World"""`},
		{"COUNT", "=", " 42"},
	} {
		if err := c.assignSymbol(tc.name, tc.op, tc.text); err != nil {
			t.Fatalf("assignSymbol(%q): %v", tc.name, err)
		}
	}

	for _, tc := range []struct{ word, value string }{
		{"DIR", "DIRECTORY/SIZE"},
		{"direc", "DIRECTORY/SIZE"},
		{"DIRECTORY", "DIRECTORY/SIZE"},
		{"GREET", `Hello, "World"`},
		{"COUNT", "42"},
	} {
		if v, ok := c.DCLSymbol(tc.word); !ok || v != tc.value {
			t.Errorf("DCLSymbol(%q) = %q, %v; want %q", tc.word, v, ok, tc.value)
		}
	}

	for _, word := range []string{"DI", "GREE", "DIRECTORYX"} {
		if v, ok := c.DCLSymbol(word); ok {
			t.Errorf("DCLSymbol(%q) = %q, want none", word, v)
		}
	}

	for _, tc := range []struct {
		name, op, text string
		code           uint32
	}{
		{"X", "=", " 12x", vmserrors.CLI_EXPSYN},
		{"X", "==", ` "open`, vmserrors.CLI_UNTERMSTR},
		{"X", "=", ` "a" b`, vmserrors.CLI_EXPSYN},
		{"A*B*C", ":=", "x", vmserrors.CLI_EXPSYN},
	} {
		err := c.assignSymbol(tc.name, tc.op, tc.text)
		if !hasStatus(err, tc.code) {
			t.Errorf("assignSymbol(%q %s %q) = %v, want code %#x", tc.name, tc.op, tc.text, err, tc.code)
		}
	}
}

// TestDispatch_dclSymbols assigns, uses, and deletes symbols through the
// dispatcher: an alias stands for its value, an alias of itself is
// refused, and DELETE/SYMBOL removes one.
func TestDispatch_dclSymbols(t *testing.T) {
	c := newBootableConsole(t)
	d := NewDispatcher(c, loadEvaxGrammar(t), nil)

	for _, line := range []string{"SD :== SET DEFAULT", "LOOP :== LOOP"} {
		if err := d.Dispatch(line); err != nil {
			t.Fatalf("%s: %v", line, err)
		}
	}

	if err := d.Dispatch("LOOP"); !hasStatus(err, vmserrors.CLI_SYMDEPTH) {
		t.Errorf("LOOP = %v, want CLI_SYMDEPTH", err)
	}

	if err := d.Dispatch("delete/symbol/global sd"); err != nil {
		t.Fatal(err)
	}

	if _, ok := c.DCLSymbol("SD"); ok {
		t.Error("SD is still defined")
	}

	if err := d.Dispatch("DELETE/SYMBOL SD"); !hasStatus(err, vmserrors.CLI_UNDEFSYM) {
		t.Errorf("deleting SD again = %v, want CLI_UNDEFSYM", err)
	}
}

// hasStatus reports whether err is a VMS error with status code.
func hasStatus(err error, code uint32) bool {
	var v vmserrors.VMSError

	return errors.As(err, &v) && v.Equals(code)
}

// dclSession is a console that runs commands through the console's
// dispatcher, and show runs one, returning its output.
func dclSession(t *testing.T) (*Dispatcher, func(string) string) {
	t.Helper()

	c := newBootableConsole(t)
	out := &bytes.Buffer{}
	c.Out = out
	d := NewDispatcher(c, loadEvaxGrammar(t), nil)

	return d, func(line string) string {
		t.Helper()
		out.Reset()

		if err := d.Dispatch(line); err != nil {
			t.Fatalf("%s: %v", line, err)
		}

		return out.String()
	}
}

// TestShowDCLSymbols shows symbols as DCL's SHOW SYMBOL does: "==" for a
// global symbol and "=" for a local one, the abbreviation's "*", a
// string's quotes doubled, and an integer in decimal, hex, and octal;
// those a wildcard matches, locals first, each table in name order; and
// one by any abbreviation it allows.
func TestShowDCLSymbols(t *testing.T) {
	d, show := dclSession(t)

	if got := show("SHOW SYMBOL/ALL"); got != "" {
		t.Errorf("with none: %q", got)
	}

	for _, line := range []string{`FO*RTH :== $FORTH`, `say := "a ""b"" c"`, "N == 42", "NEG = -1"} {
		show(line)
	}

	all := `  NEG = -1   Hex = FFFFFFFF  Octal = 37777777777
  SAY = "a ""b"" c"
  FO*RTH == "$FORTH"
  N == 42   Hex = 0000002A  Octal = 00000000052
`
	if got := show("SHOW SYMBOL *"); got != all {
		t.Errorf("all:\n%s\nwant:\n%s", got, all)
	}

	if got, want := show("show symbol fo"), "  FO*RTH == \"$FORTH\"\n"; got != want {
		t.Errorf("by abbreviation: %q, want %q", got, want)
	}

	if got := show("SHOW SYMBOLS N*"); !strings.HasPrefix(got, "  NEG = -1") || strings.Count(got, "\n") != 2 {
		t.Errorf("wildcard: %q", got)
	}

	if got, want := show("SHOW SYMBOL/ALL"), "  NEG = -1   Hex = FFFFFFFF  Octal = 37777777777\n  SAY = \"a \"\"b\"\" c\"\n"; got != want {
		t.Errorf("/ALL (the local table): %q, want %q", got, want)
	}

	if got, want := show("SHOW SYMBOL/GLOBAL/ALL"), "  FO*RTH == \"$FORTH\"\n  N == 42   Hex = 0000002A  Octal = 00000000052\n"; got != want {
		t.Errorf("/GLOBAL/ALL: %q, want %q", got, want)
	}

	for line, code := range map[string]uint32{
		"SHOW SYMBOL NOSUCH":        vmserrors.CLI_UNDEFSYM,
		"SHOW SYMBOL NOSUCH*":       vmserrors.CLI_UNDEFSYM,
		"SHOW SYMBOL/LOCAL N":       vmserrors.CLI_UNDEFSYM,
		"SHOW SYMBOL/GLOBAL SAY":    vmserrors.CLI_UNDEFSYM,
		"SHOW SYMBOL":               vmserrors.CLI_MISSINGPARAMETER,
		"SHOW SYMBOL/LOCAL/GLOBAL N": 0,
	} {
		err := d.Dispatch(line)
		if code == 0 && err == nil || code != 0 && !hasStatus(err, code) {
			t.Errorf("%s: %v, want an error (%#x)", line, err, code)
		}
	}
}

// TestShowDCLSymbols_simh is SHOW SYMBOL as a VMS 7.3 system (a simh VAX
// session) shows its global symbols, and a local string symbol.
func TestShowDCLSymbols_simh(t *testing.T) {
	_, show := dclSession(t)

	for _, line := range []string{
		`$RESTART == "FALSE"`,
		`$SEVERITY == "1"`,
		`$STATUS == "%X00030001"`,
		`REBOOT == "@SYS$SYSTEM:SHUTDOWN 0 SHUTDOWN YES NO LATER YES NONE"`,
		`SHUTDOWN == "@SYS$SYSTEM:SHUTDOWN 0 SHUTDOWN YES NO LATER NO NONE"`,
		`SHUTDOWN1 == "@SYS$SYSTEM:SHUTDOWN"`,
	} {
		show(line)
	}

	want := `  $RESTART == "FALSE"
  $SEVERITY == "1"
  $STATUS == "%X00030001"
  REBOOT == "@SYS$SYSTEM:SHUTDOWN 0 SHUTDOWN YES NO LATER YES NONE"
  SHUTDOWN == "@SYS$SYSTEM:SHUTDOWN 0 SHUTDOWN YES NO LATER NO NONE"
  SHUTDOWN1 == "@SYS$SYSTEM:SHUTDOWN"
`
	if got := show("show sym *"); got != want {
		t.Errorf("show sym *:\n%s\nwant:\n%s", got, want)
	}

	// ":=" assigns the rest of the line as a string, digits or not.
	show("age := 55")

	if got, want := show("show sym/local age"), "  AGE = \"55\"\n"; got != want {
		t.Errorf("show sym/local age: %q, want %q", got, want)
	}

	if got, want := show("show sym shutdown"), "  SHUTDOWN == \"@SYS$SYSTEM:SHUTDOWN 0 SHUTDOWN YES NO LATER NO NONE\"\n"; got != want {
		t.Errorf("show sym shutdown: %q, want %q", got, want)
	}
}

// TestDCLSymbols_localAndGlobal keeps a local and a global symbol of the
// same name apart: the local one is found first, SHOW SYMBOL/GLOBAL and
// /LOCAL find each, and DELETE/SYMBOL deletes the local one unless told
// /GLOBAL; /ALL empties a table.
func TestDCLSymbols_localAndGlobal(t *testing.T) {
	d, show := dclSession(t)

	show(`X = "local"`)
	show(`X == "global"`)
	show(`Y == 1`)

	if got, want := show("SHOW SYMBOL X"), "  X = \"local\"\n"; got != want {
		t.Errorf("SHOW SYMBOL X: %q, want %q", got, want)
	}

	if got, want := show("SHOW SYMBOL/GLOBAL X"), "  X == \"global\"\n"; got != want {
		t.Errorf("SHOW SYMBOL/GLOBAL X: %q, want %q", got, want)
	}

	show("DELETE/SYMBOL X")

	if got, want := show("SHOW SYMBOL X"), "  X == \"global\"\n"; got != want {
		t.Errorf("after DELETE/SYMBOL X: %q, want %q", got, want)
	}

	if err := d.Dispatch("DELETE/SYMBOL X"); !hasStatus(err, vmserrors.CLI_UNDEFSYM) {
		t.Errorf("DELETE/SYMBOL X with only a global X: %v, want CLI_UNDEFSYM", err)
	}

	show("DELETE/SYMBOL/GLOBAL/ALL")

	if err := d.Dispatch("SHOW SYMBOL *"); !hasStatus(err, vmserrors.CLI_UNDEFSYM) {
		t.Errorf("SHOW SYMBOL * after DELETE/SYMBOL/GLOBAL/ALL: %v, want CLI_UNDEFSYM", err)
	}

	for line, code := range map[string]uint32{
		"DELETE/SYMBOL":              vmserrors.CLI_MISSINGPARAMETER,
		"DELETE/SYMBOL/ALL Y":        vmserrors.CLI_EXPSYN,
		"DELETE/SYMBOL/LOCAL/GLOBAL Y": vmserrors.CLI_EXPSYN,
		"DELETE/SYMBOL/BOGUS Y":      vmserrors.CLI_EXPSYN,
	} {
		if err := d.Dispatch(line); !hasStatus(err, code) {
			t.Errorf("%s: %v, want %#x", line, err, code)
		}
	}
}
