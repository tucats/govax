package dclshell

import (
	"bytes"
	"testing"
)

// TestDispatch_if exercises DCL's one-line IF expression THEN command
// (dclif.go's ifCommand): the command runs only when the expression is
// true, which is when its value is odd (the User's Manual, 14.16.4).
func TestDispatch_if(t *testing.T) {
	d, c := newTestDispatcher(t)

	for _, tc := range []struct {
		line, want string
	}{
		{`IF 0 THEN R == 1`, "0"},
		{`IF 2 THEN R == 1`, "0"},
		{`IF "NO" THEN R == 1`, "0"},
		{`IF 1 THEN R == 1`, "1"},
		{`IF 1 .EQ. 1 THEN $ R == 2`, "2"},
		{`IF "yes" THEN R == 3`, "3"},
		{`IF "27" THEN R == 4`, "4"},
		{`IF "28" THEN R == 5`, "4"},
	} {
		if _, ok := c.dclSymbols.lookup("R"); !ok {
			if err := d.Dispatch("R == 0"); err != nil {
				t.Fatal(err)
			}
		}

		if err := d.Dispatch(tc.line); err != nil {
			t.Fatalf("%s: %v", tc.line, err)
		}

		if got, _ := c.dclSymbols.lookup("R"); got.value != tc.want {
			t.Errorf("%s: R = %q, want %q", tc.line, got.value, tc.want)
		}
	}
}

// TestDispatch_comment: a "!" outside quotes starts a comment, which the
// grammar never sees (the DCL Dictionary), on a command line as on an
// assignment; a "!" in quotes is text. (Found by Phase 57's probe: a
// chained "SHOW SYMBOL X ! ..." was EXTRAPARAMETER.)
func TestDispatch_comment(t *testing.T) {
	d, c := newTestDispatcher(t)
	out := &bytes.Buffer{}
	c.Out = out

	for _, tc := range []struct {
		line, want string
	}{
		{`X = "one" ! a comment`, ""},
		{`SHOW SYMBOL X ! a comment`, `  X = "one"` + "\n"},
		{`SHOW SYMBOL X!no blank`, `  X = "one"` + "\n"},
		{`WRITE SYS$OUTPUT "a!b" ! then a comment`, "a!b\n"},
		{`   ! only a comment`, ""},
	} {
		out.Reset()

		if err := d.Dispatch(tc.line); err != nil {
			t.Fatalf("%s: %v", tc.line, err)
		}

		if out.String() != tc.want {
			t.Errorf("%s: output %q, want %q", tc.line, out.String(), tc.want)
		}
	}
}
