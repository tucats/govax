package sema

import (
	"strings"
	"testing"
)

// TestShapes checks Fortran 90's shapes and the contexts arrays may stand in
// (docs/planning/PHASE-102 - Fortran 90 array expressions.md, subtask 3): each
// case is a main program's statements and its messages, "IDENT: text" each,
// one per line; "" for none. Since the scalarizer (subtask 4) a case that
// conforms says nothing, unless its sides may overlap, which is subtask 5's.
func TestShapes(t *testing.T) {
	const notYet = ""

	const overlap = "NOTIMPL: an array assignment whose value may read its target's changed elements is not implemented yet"

	cases := []struct {
		name  string
		lines []string
		want  string
	}{
		// Whole arrays and sections that conform.
		{"whole arrays", []string{"DIMENSION A(10), B(10)", "A = B + 2.0 * A"}, notYet},
		{"a scalar broadcast", []string{"DIMENSION A(10)", "A = 0"}, notYet},
		{"a relation into LOGICAL", []string{"DIMENSION A(3), B(3)", "LOGICAL L(3)", "L = A .GT. B"}, notYet},
		{"sections of one shape", []string{"DIMENSION A(10), B(20)", "A(2:10) = B(1:17:2)"}, notYet},
		{"a column and a row", []string{"DIMENSION A(4, 6), B(6)", "B(1:4) = A(:, 3) + A(2, 1:4)"}, notYet},
		{"a negative stride", []string{"DIMENSION A(10), B(10)", "A(10:1:-1) = B"}, notYet},
		{"a bound not constant", []string{"DIMENSION A(10), B(10)", "A(1:N) = B(1:N)"}, notYet},
		{"an extent not known with one known", []string{"DIMENSION A(10), B(10)", "A(1:N) = B"}, notYet},
		{"an adjustable array", []string{"SUBROUTINE S(A, N)", "DIMENSION A(N)", "A = A * 2", "A(2:) = A(:N-1)"}, overlap},
		{"character arrays", []string{"CHARACTER*4 C(3), D(3)*8", "D = C // 'XY'", "C(:)(2:3) = 'AB'"}, notYet},
		{"a section of size zero", []string{"DIMENSION A(10)", "A(5:4) = 1"}, notYet},

		// Overlap: the same elements, apart, or not known (subtask 5's).
		{"the same elements", []string{"DIMENSION A(10)", "A(1:N) = 2 * A(1:N) + A(1:N)"}, notYet},
		{"different arrays", []string{"DIMENSION A(10), B(10)", "A = B * B"}, notYet},
		{"two dummies", []string{"SUBROUTINE S(A, B)", "DIMENSION A(10), B(10)", "A = B"}, notYet},
		{"a scalar computed first", []string{"DIMENSION A(10)", "A = A / A(1) ** 2"}, notYet},
		{"the target's element read", []string{"DIMENSION A(10)", "A = A(1)"}, overlap},
		{"a shifted section", []string{"DIMENSION A(10)", "A(2:10) = A(1:9)"}, overlap},
		{"a reversed array", []string{"DIMENSION A(10)", "A(10:1:-1) = A"}, overlap},
		{"EQUIVALENCE", []string{"DIMENSION A(10), B(10)", "EQUIVALENCE (A(2), B)", "A = B"}, overlap},
		{"one common block", []string{"DIMENSION A(10), B(10)", "COMMON /C/ A, B", "A = B"}, notYet},
		{"overlapping in common", []string{"DIMENSION A(10), B(10)", "COMMON /C/ A", "EQUIVALENCE (A(5), B)", "A = B"}, overlap},
		{"a CHARACTER function", []string{"CHARACTER*4 C(3), F", "C = F(1)"}, "NOTIMPL: a CHARACTER function's value in an array expression is not implemented yet"},
		{"a concatenation of unknown length", []string{"SUBROUTINE S(C, D)", "CHARACTER*(*) C(3), D", "C = C // D"}, "NOTIMPL: a concatenation of a length not constant in an array expression is not implemented yet"},

		// What doesn't conform.
		{"extents", []string{"DIMENSION A(10), B(5)", "A = B"}, "NOTCONFORM: the shapes (10) and (5) don't conform"},
		{"in an operation", []string{"DIMENSION A(10), B(5), C(10)", "C = A + B"}, "NOTCONFORM: the shapes (10) and (5) don't conform"},
		{"ranks", []string{"DIMENSION A(2, 5), B(10)", "B = A"}, "NOTCONFORM: the shapes (10) and (2, 5) don't conform"},
		{"a section's extent", []string{"DIMENSION A(10)", "A(1:5) = A(1:10:2) + A(2:4)"}, "NOTCONFORM: the shapes (5) and (3) don't conform"},
		{"an unknown extent's rank", []string{"DIMENSION A(10), B(4, 4)", "A(1:N) = B"}, "NOTCONFORM: the shapes (:) and (4, 4) don't conform"},
		{"a scalar target", []string{"DIMENSION A(10)", "X = A(1:2)"}, "NOTSCALAR: an array is used where a scalar is required"},

		// Sections' own errors.
		{"a stride of zero", []string{"DIMENSION A(10)", "A(1:10:0) = 1"}, "ZEROSTRIDE: a subscript triplet's stride can't be zero"},
		{"an assumed-size section's end", []string{"SUBROUTINE S(A)", "DIMENSION A(*)", "A(2:) = 0"}, "SECTIONEND: a section of the assumed-size array A needs an upper bound in its last dimension"},
		{"an assumed-size section", []string{"SUBROUTINE S(A)", "DIMENSION A(*)", "A(2:5) = 0"}, notYet},
		{"an assumed-size whole array", []string{"SUBROUTINE S(A)", "DIMENSION A(*)", "A = 0"}, "NOSHAPE: the assumed-size array A has no shape, so it can't be used as a whole here"},
		{"the subscripts used", []string{"DIMENSION A(10)", "A(3:11:7) = 1"}, notYet},
		{"a subscript used past the end", []string{"DIMENSION A(10)", "A(3:12:3) = 1"}, "SUBSCRIPTRANGE: subscript 12 of A is past its upper bound, 10"},
		{"a subscript used before the start", []string{"DIMENSION A(10)", "A(0:4) = 1"}, "SUBSCRIPTRANGE: subscript 0 of A is past its lower bound, 1"},
		{"too few subscripts", []string{"DIMENSION A(10, 10)", "A(:) = 1"}, "SUBSCRIPTS: the array A takes 2 subscripts, not 1"},

		// Where a scalar must be, and what isn't compiled yet.
		{"a subscript", []string{"DIMENSION A(10), I(2)", "X = A(I(1:1))"}, "NOTIMPL: a vector subscript is not implemented yet"},
		{"a condition", []string{"DIMENSION A(10)", "IF (A .GT. 0) STOP"}, "NOTSCALAR: an array is used where a scalar is required"},
		{"a DO bound", []string{"DIMENSION A(10)", "DO 10 I = 1, A(1:2)", "10 CONTINUE"}, "NOTSCALAR: an array is used where a scalar is required"},
		{"a bare name stays NEEDSUBSCRIPT", []string{"DIMENSION A(10)", "IF (A) STOP"}, "NEEDSUBSCRIPT: the array A can't be used without subscripts here"},
		{"an argument", []string{"DIMENSION A(10)", "CALL S(A(2:5))"}, "NOTIMPL: an array section or array expression as an actual argument is not implemented yet"},
		{"an expression argument", []string{"DIMENSION A(10)", "CALL S(A + 1)"}, "NOTIMPL: an array section or array expression as an actual argument is not implemented yet"},
		{"a whole array argument", []string{"DIMENSION A(10)", "CALL S(A)"}, ""},
		{"an intrinsic's", []string{"DIMENSION A(10)", "X = SQRT(A)"}, "NOTIMPL: an array argument to the intrinsic function SQRT is not implemented yet"},
		{"an output item", []string{"DIMENSION A(10)", "PRINT *, A(1:3)"}, "NOTIMPL: an array section or array expression in an I/O list is not implemented yet"},
		{"an input item", []string{"DIMENSION A(10)", "READ *, A(1:3)"}, "NOTIMPL: an array section in an I/O statement is not implemented yet"},
		{"a whole array item", []string{"DIMENSION A(10)", "PRINT *, A"}, ""},
		{"a statement function", []string{"DIMENSION A(10)", "F(X) = A(1:2) + X", "Y = F(1.0)"}, "NOTSCALAR: an array is used where a scalar is required"},
	}

	for _, c := range cases {
		lines := make([]string, 0, len(c.lines)+1)
		for _, l := range c.lines {
			lines = append(lines, fixedLine(l))
		}

		lines = append(lines, fixedLine("END"))

		_, ms := analyzeLines(t, "T.FOR", lines...)

		got := make([]string, len(ms))
		for i, m := range ms {
			got[i] = m.Ident + ": " + m.Text
		}

		if g := strings.Join(got, "\n"); g != c.want {
			t.Errorf("%s: got\n%s\nwant\n%s", c.name, g, c.want)
		}
	}
}
