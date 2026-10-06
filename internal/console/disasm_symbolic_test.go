package console

import (
	"strings"
	"testing"
)

// TestEvaluateDebugNames: the expression evaluator finds names and lines
// in a loaded image's debug symbol table, after the console's own.
func TestEvaluateDebugNames(t *testing.T) {
	c, _ := runStepped(t, dbgImagePath(t, "dbgdis.exe"))

	cases := []struct {
		expr string
		want uint32
	}{
		{"START", 0x400},
		{`DBGDIS\START\LOOP`, 0x4DD},
		{`dbgdis\start\loop`, 0x4DD},
		{"JSBRTN+3", 0x53E},
		{`DBGSUB\SUBDATA`, 0x268},
		{"GLIMIT", 3},                // a global constant
		{"%LINE 47", 0x419},          // the debugger's EVALUATE/ADDRESS %LINE 47
		{`DBGSUB\%LINE 14`, 0x546},   // ... and DBGSUB\%LINE 14
		{`DBGDIS\START\%LINE 42+3`, 0x405},
		{"(%LINE 47)-2", 0x417},
	}

	for _, tc := range cases {
		v, rest, err := c.Evaluator().Eval(tc.expr)
		if err != nil || strings.TrimSpace(rest) != "" || v != tc.want {
			t.Errorf("Eval(%q) = %X, %q, %v; want %X", tc.expr, v, rest, err, tc.want)
		}
	}

	for _, bad := range []string{`DBGDIS\NOSUCH`, `DBGSUB\SUBEND`, "%LINE 9999", "%LINE", `NOSUCH\%LINE 1`} {
		if _, _, err := c.Evaluator().Eval(bad); err == nil {
			t.Errorf("Eval(%q) succeeded; want an error", bad)
		}
	}
}
