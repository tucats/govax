package console

import (
	"errors"
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
