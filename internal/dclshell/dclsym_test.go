package dclshell

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

// hasStatus reports whether err is a VMS error with status code.
func hasStatus(err error, code uint32) bool {
	var v vmserrors.VMSError

	return errors.As(err, &v) && v.Equals(code)
}
