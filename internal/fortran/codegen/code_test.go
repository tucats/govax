package codegen

import "testing"

// TestOperandBounds holds operandBound to the bytes MACRO assembles each operand
// form to. A bound too small makes the branch passes shorten a branch that then
// can't reach (Phase 111, subtask 3a: fcalc's FCEVAL, /CHECK=ALL, had a byte
// branch 128 bytes from its label past a MOVC3 with @(R10)[R0]).
func TestOperandBounds(t *testing.T) {
	c := &Code{Displacements: map[string]int{"X.A": -556, "X.P": 4}}

	for _, test := range []struct {
		text  string
		bytes int
	}{
		{"R7", 1},
		{"(R10)", 1},
		{"(R10)+", 1},
		{"-(SP)", 1},
		{"@(R10)+", 1},
		// No mode is register deferred twice: @(R10) is @0(R10).
		{"@(R10)", 2},
		{"@(R10)[R0]", 3},
		{"#31", 1},
		{"#200", 5},
		{"X.P(AP)", 2},
		{"@X.P(AP)", 2},
		{"X.A-23(FP)[R6]", 4},
		{"84(R0)", 2},
		{"G^FTN$NOTASSOC", 5},
	} {
		if got := c.operandBound(test.text); got != test.bytes {
			t.Errorf("operandBound(%q) = %d, want %d", test.text, got, test.bytes)
		}
	}
}
