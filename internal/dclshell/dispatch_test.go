package dclshell

import (
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
