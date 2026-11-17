package sema

import "testing"

// TestArrayFunctions checks Fortran 90's array functions (docs/planning/PHASE-102
// - Fortran 90 array expressions.md, subtask 9): each case is a main program's
// lines after the declarations, compiled /F90, and the dump of its last
// assignment's value with its shape, or its first message, "IDENT: text".
func TestArrayFunctions(t *testing.T) {
	decls := []string{
		"REAL A(3, 4), V(4), W(4), M(4, 2)",
		"INTEGER K(4), J(2)",
		"LOGICAL L(3, 4), LL",
		"COMPLEX Z(3)",
	}

	cases := []struct {
		name  string
		lines []string
		want  string
	}{
		{"SUM", []string{"X = SUM(A)"}, "SUM(A[]) a scalar"},
		{"SUM with MASK", []string{"X = SUM(A, MASK=A .GT. 0)"}, "SUM(A[], MASK=(.GT.:R A[] 0.0)) a scalar"},
		{"SUM with DIM", []string{"V = SUM(A, DIM=1)"}, "SUM(A[], DIM=1) (4)"},
		{"SUM along the second", []string{"X = SUM(SUM(A, 2))"}, "SUM(SUM(A[], DIM=2)) a scalar"},
		{"DIM of a vector", []string{"X = PRODUCT(V, 1)"}, "PRODUCT(V[]) a scalar"},
		{"MAXVAL", []string{"X = MAXVAL(V) - MINVAL(V)"}, ""},
		{"COUNT", []string{"N = COUNT(L)"}, "COUNT(L[]) a scalar"},
		{"ANY with DIM", []string{"J(1) = 0", "LL = ANY(ANY(L, DIM=1))"}, "ANY(ANY(L[], DIM=1)) a scalar"},
		{"DOT_PRODUCT", []string{"X = DOT_PRODUCT(V, W)"}, "SUM((*:R V[] W[])) a scalar"},
		{"DOT_PRODUCT of COMPLEX", []string{"ZZ = DOT_PRODUCT(Z, Z)"}, ""},
		{"MAXLOC", []string{"J = MAXLOC(A)"}, "MAXLOC(A[]) (2)"},
		{"MINLOC with MASK", []string{"K(1:1) = MINLOC(V, MASK=V .GT. 0)"}, "MINLOC(V[], MASK=(.GT.:R V[] 0.0)) (1)"},
		{"MATMUL", []string{"V(1:3) = MATMUL(A, V)"}, "MATMUL(A[], V[]) (3)"},
		{"MATMUL of matrices", []string{"M(1:3, :) = MATMUL(A, M)"}, "MATMUL(A[], M[]) (3, 2)"},
		{"TRANSPOSE", []string{"M = TRANSPOSE(A(1:2, :))"}, "TRANSPOSE(A[1:2:1, 1:4:1]) (4, 2)"},
		{"SIZE of a section", []string{"N = SIZE(A(1:I, :))"}, "SIZE(A[1:I:1, 1:4:1]) a scalar"},
		{"SHAPE of a section", []string{"J = SHAPE(A(1:I, :))"}, "SHAPE(A[1:I:1, 1:4:1]) (2)"},
		{"UBOUND with a DIM not constant", []string{"N = UBOUND(A(1:I, :), I)"}, "UBOUND(A[1:I:1, 1:4:1], DIM=I) a scalar"},
		{"SIZE folded", []string{"N = SIZE(A, 2)"}, "4 a scalar"},

		{"a DIM out of range", []string{"V = SUM(A, DIM=3)"}, "DIMRANGE: SUM's DIM, 3, isn't a dimension of an array of rank 2"},
		{"a DIM not constant", []string{"V = SUM(A, DIM=I)"}, "NOTIMPL: SUM with a DIM= not constant (Phase 104) is not implemented yet"},
		{"a scalar ARRAY", []string{"X = SUM(X)"}, "ARRAYARG: SUM's ARRAY must be an array"},
		{"MAXVAL of COMPLEX", []string{"X = MAXVAL(Z)"}, "BADARG: the intrinsic function MAXVAL's arguments must be INTEGER or REAL, and this one is COMPLEX"},
		{"a mask that doesn't conform", []string{"X = SUM(V, MASK=L)"}, "NOTCONFORM: the shapes (4) and (3, 4) don't conform"},
		{"MATMUL's shapes", []string{"V(1:3) = MATMUL(A, W(1:3))"}, "NOTCONFORM: the shapes (4) and (3) don't conform"},
		{"MATMUL of vectors", []string{"X = MATMUL(V, W)"}, "ARRAYRANK: MATMUL's MATRIX_B must be of rank 2 when MATRIX_A's is 1, not 1"},
		{"TRANSPOSE of a vector", []string{"V = TRANSPOSE(V)"}, "ARRAYRANK: TRANSPOSE's MATRIX must be of rank 2, not 1"},
		{"RESHAPE not constant", []string{"M = RESHAPE(V, (/ 4, 2 /))"}, "NOTIMPL: RESHAPE of values not constant (Phase 104) is not implemented yet"},
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

		if c.want == "" && len(ms) == 0 {
			continue
		}

		if got != c.want {
			t.Errorf("%s: got\n%s\nwant\n%s", c.name, got, c.want)
		}
	}
}
