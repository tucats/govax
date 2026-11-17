package sema

import "testing"

// TestElementals checks the elemental intrinsic functions on arrays and MERGE
// (docs/planning/PHASE-102 - Fortran 90 array expressions.md, subtask 8): each
// case is a main program's lines, compiled /F90, and the dump of its last
// assignment's value with its shape, or its first message, "IDENT: text".
func TestElementals(t *testing.T) {
	decls := []string{
		"REAL A(10), B(10), C(10)",
		"INTEGER K(10)",
		"LOGICAL L(10)",
		"CHARACTER*2 S(10)",
	}

	cases := []struct {
		name  string
		lines []string
		want  string
	}{
		{"SQRT", []string{"B = SQRT(A)"}, "(call:REAL MTH$SQRT A[]) (10)"},
		{"MAX with a scalar", []string{"B = MAX(A, 0.0)"}, "(max:REAL AMAX1 A[] 0.0) (10)"},
		{"ABS of an expression", []string{"B = ABS(A - C)"}, "(abs:REAL ABS (-:R A[] C[])) (10)"},
		{"of a section", []string{"B(1:5) = SIN(A(2:6)) + COS(C(1:5))"}, "(+:R (call:REAL MTH$SIN A[2:6:1]) (call:REAL MTH$COS C[1:5:1])) (5)"},
		{"a conversion", []string{"B = REAL(K)"}, "(REAL K[]) (10)"},
		{"a bit test", []string{"L = BTEST(K, 1)"}, "(bittest:LOGICAL BJTEST K[] 1) (10)"},
		{"ICHAR", []string{"K = ICHAR(S)"}, "(ichar:INTEGER ICHAR S[]) (10)"},
		{"CHAR", []string{"S = CHAR(K)"}, "(char:CHARACTER*1 CHAR K[]) (10)"},
		{"INDEX", []string{"K = INDEX(S, 'B')"}, "(index:INTEGER INDEX S[] 'B') (10)"},
		{"LEN, an inquiry", []string{"N = LEN(S)"}, "2 a scalar"},
		{"SIZEOF of an array", []string{"N = SIZEOF(A)"}, "40 a scalar"},
		{"KIND of an array", []string{"N = KIND(A)"}, "4 a scalar"},
		{"MERGE", []string{"B = MERGE(A, C, A .GT. C)"}, "(merge:REAL MERGE A[] C[] (.GT.:R A[] C[])) (10)"},
		{"MERGE of a scalar", []string{"B = MERGE(A, 0.0, L)"}, "(merge:REAL MERGE A[] 0.0 L[]) (10)"},
		{"MERGE of scalars", []string{"X = MERGE(1.0, 2.0, N .GT. 0)"}, "(merge:REAL MERGE 1.0 2.0 (.GT.:I N 0)) a scalar"},
		{"MERGE of CHARACTER", []string{"S = MERGE(S, 'XX', L)"}, "(merge:CHARACTER*2 MERGE S[] 'XX' L[]) (10)"},
		{"arguments that don't conform", []string{"B(1:3) = MAX(A(1:3), C)"}, "NOTCONFORM: the shapes (3) and (10) don't conform"},
		{"MERGE of two types", []string{"B = MERGE(A, K, L)"}, "MERGEARGS: the TSOURCE and FSOURCE of MERGE must have one type and length, and they are REAL and INTEGER"},
		{"MERGE's mask", []string{"B = MERGE(A, C, K)"}, "BADARG: the intrinsic function MERGE's arguments must be a LOGICAL MASK, and this one is INTEGER"},
		{"an array where a scalar must be", []string{"X = SQRT(A)"}, "NOTSCALAR: an array is used where a scalar is required"},
	}

	for _, c := range cases {
		lines := make([]string, 0, len(decls)+len(c.lines)+1)
		for _, l := range append(append([]string{}, decls...), c.lines...) {
			lines = append(lines, fixedLine(l))
		}

		lines = append(lines, fixedLine("END"))

		prog, ms := analyzeOpts(t, Options{F90: true}, lines...)

		var got string

		if len(ms) > 0 {
			got = ms[0].Ident + ": " + ms[0].Text
		} else {
			for _, st := range prog.Units[0].Body {
				if a, ok := st.(*Assign); ok {
					got = DumpExpr(a.Value) + " " + a.Value.Shape().String()
				}
			}
		}

		if got != c.want {
			t.Errorf("%s: got\n%s\nwant\n%s", c.name, got, c.want)
		}
	}
}
