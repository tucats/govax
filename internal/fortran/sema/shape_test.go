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
		{"an adjustable array", []string{"SUBROUTINE S(A, N)", "DIMENSION A(N)", "A = A * 2", "A(2:) = A(:N-1)"}, notYet},
		{"character arrays", []string{"CHARACTER*4 C(3), D(3)*8", "D = C // 'XY'", "C(:)(2:3) = 'AB'"}, notYet},
		{"a section of size zero", []string{"DIMENSION A(10)", "A(5:4) = 1"}, notYet},

		// Overlap: the same elements, apart, ordered, or through a temporary
		// (TestArrayOrder has which).
		{"the same elements", []string{"DIMENSION A(10)", "A(1:N) = 2 * A(1:N) + A(1:N)"}, notYet},
		{"different arrays", []string{"DIMENSION A(10), B(10)", "A = B * B"}, notYet},
		{"two dummies", []string{"SUBROUTINE S(A, B)", "DIMENSION A(10), B(10)", "A = B"}, notYet},
		{"a scalar computed first", []string{"DIMENSION A(10)", "A = A / A(1) ** 2"}, notYet},
		{"the target's element read", []string{"DIMENSION A(10)", "A = A(1)"}, notYet},
		{"a shifted section", []string{"DIMENSION A(10)", "A(2:10) = A(1:9)"}, notYet},
		{"a reversed array", []string{"DIMENSION A(10)", "A(10:1:-1) = A"}, notYet},
		{"EQUIVALENCE", []string{"DIMENSION A(10), B(10)", "EQUIVALENCE (A(2), B)", "A = B"}, notYet},
		{"one common block", []string{"DIMENSION A(10), B(10)", "COMMON /C/ A, B", "A = B"}, notYet},
		{"overlapping in common", []string{"DIMENSION A(10), B(10)", "COMMON /C/ A", "EQUIVALENCE (A(5), B)", "A = B"}, notYet},
		{"a temporary of unknown size", []string{"DIMENSION A(10)", "A(1:N) = A(N:1:-1)"}, ""},
		{"a CHARACTER function", []string{"CHARACTER*4 C(3), F", "C = F(1)"}, ""},
		{"a concatenation of unknown length", []string{"SUBROUTINE S(C, D)", "CHARACTER*(*) C(3), D", "C = C // D"}, ""},
		{"a constructor of a substring's values", []string{"SUBROUTINE S(C, D, N)", "CHARACTER*(*) C(3), D", "C = (/ D(1:N), C(2:3) /)"},
			"NOTIMPL: an array constructor whose first CHARACTER value of a length not constant isn't a variable's, an array's, or a concatenation of them is not implemented yet"},
		{"a constructor of a dummy's values", []string{"SUBROUTINE S(C, D)", "CHARACTER*(*) C(3), D", "C = (/ D, (C(I), I = 2, 3) /)"}, ""},

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
		{"a subscript", []string{"DIMENSION A(10), I(2)", "X = A(I(1:1))"}, "NOTSCALAR: an array is used where a scalar is required"},

		// Vector subscripts (subtask 6).
		{"a vector subscript read", []string{"DIMENSION A(10), B(3)", "INTEGER IV(3)", "B = A(IV)"}, ""},
		{"a vector subscript assigned", []string{"DIMENSION A(10), B(3)", "INTEGER IV(3)", "A(IV) = B + 1"}, ""},
		{"a vector expression", []string{"DIMENSION A(10), B(3)", "INTEGER IV(3)", "B = A(IV + 1) + A(2 * IV(3:1:-1))"}, ""},
		{"a vector with a triplet", []string{"DIMENSION A(4, 10), B(4, 3)", "INTEGER IV(3)", "B = A(:, IV)"}, ""},
		{"a vector's extent", []string{"DIMENSION A(10), B(5)", "INTEGER IV(3)", "B = A(IV)"}, "NOTCONFORM: the shapes (5) and (3) don't conform"},
		{"a vector of rank 2", []string{"DIMENSION A(10), B(4)", "INTEGER IV(2, 2)", "B = A(IV)"}, "VECTORRANK: a vector subscript must be an array of rank one, not rank 2"},
		{"a vector of the target", []string{"INTEGER IV(3)", "IV(IV) = 1"}, ""},
		{"a vector where a scalar must be", []string{"DIMENSION A(10)", "INTEGER IV(3)", "IF (A(IV) .GT. 0) STOP"}, "NOTSCALAR: an array is used where a scalar is required"},
		{"a condition", []string{"DIMENSION A(10)", "IF (A .GT. 0) STOP"}, "NOTSCALAR: an array is used where a scalar is required"},
		{"a DO bound", []string{"DIMENSION A(10)", "DO 10 I = 1, A(1:2)", "10 CONTINUE"}, "NOTSCALAR: an array is used where a scalar is required"},
		{"a bare name stays NEEDSUBSCRIPT", []string{"DIMENSION A(10)", "IF (A) STOP"}, "NEEDSUBSCRIPT: the array A can't be used without subscripts here"},
		{"an argument", []string{"DIMENSION A(10)", "CALL S(A(2:5))"}, "NOTIMPL: an array section or array expression as an actual argument is not implemented yet"},
		{"an expression argument", []string{"DIMENSION A(10)", "CALL S(A + 1)"}, "NOTIMPL: an array section or array expression as an actual argument is not implemented yet"},
		{"a whole array argument", []string{"DIMENSION A(10)", "CALL S(A)"}, ""},
		{"an intrinsic's", []string{"DIMENSION A(10)", "X = SQRT(A)"}, "NOTSCALAR: an array is used where a scalar is required"},
		{"an output item", []string{"DIMENSION A(10)", "PRINT *, A(1:3)"}, ""},
		{"an input item", []string{"DIMENSION A(10)", "READ *, A(1:3)"}, ""},

		// I/O lists (subtask 11).
		{"output expressions", []string{"DIMENSION A(10), B(2, 3)", "PRINT *, A + 1, SQRT(B(:, 2)), (/ 1, 2 /), A(10:1:-3) * B(1, 1)"}, ""},
		{"an output constructor", []string{"PRINT *, (/ (I * I, I = 1, N) /)"}, ""},
		{"sections in an implied DO", []string{"DIMENSION B(2, 3)", "READ *, (B(I, :), I = 1, 2)"}, ""},
		{"a section's substrings read", []string{"CHARACTER*4 C(3)", "READ *, C(2:3)(1:2)"}, ""},
		{"an extent read first", []string{"DIMENSION A(10)", "READ *, N, A(1:N)"}, ""},
		{"a vector read into", []string{"DIMENSION A(10), IV(3)", "READ *, A(IV)"}, ""},
		{"a many-one section read", []string{"DIMENSION A(10)", "READ *, A((/ 2, 3, 2 /))"},
			"MANYONE: the vector subscript of A names element 2 more than once, so the section can't be given values"},
		{"an expression read into", []string{"DIMENSION A(10)", "READ *, A + 1"},
			"BADINPUT: an expression can't be read into: an input list holds variables, array elements, substrings, arrays, and implied-DO lists"},
		{"a CHARACTER function's values", []string{"CHARACTER*4 C(3), F", "PRINT *, C // F(1)"}, ""},
		{"a section as an internal file", []string{"CHARACTER*4 C(6)", "WRITE (C(1:6:2), '(A)') 'X'"},
			"NOTIMPL: an array section as an internal file or buffer is not implemented yet"},
		{"a section as ENCODE's buffer", []string{"DIMENSION A(10)", "ENCODE (8, 100, A(1:9:2)) X", "100 FORMAT (F8.2)"},
			"NOTIMPL: an array section as an internal file or buffer is not implemented yet"},
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

// TestArrayOrder checks overlap's answers (docs/planning/PHASE-102 - Fortran
// 90 array expressions.md, finding 6; subtask 5): each case is a main
// program's declarations and one array assignment, and the order the dump
// shows after it: "" for every loop up, "down in n", or "through a temporary".
func TestArrayOrder(t *testing.T) {
	cases := []struct {
		name  string
		lines []string
		want  string
	}{
		{"apart", []string{"DIMENSION A(10), B(10)", "A = B + A"}, ""},
		{"the same elements", []string{"DIMENSION A(10)", "A(1:N) = A(1:N) * 2"}, ""},
		{"a whole array and its full section", []string{"DIMENSION A(10)", "A = A(1:10) + A(:)"}, ""},
		{"read ahead", []string{"DIMENSION A(10)", "A(1:9) = A(2:10)"}, ""},
		{"read behind", []string{"DIMENSION A(10)", "A(2:10) = A(1:9)"}, "down in 1"},
		{"read behind, by bounds not constant", []string{"DIMENSION A(10)", "A(N+1:M+1) = A(N:M)"}, "down in 1"},
		{"read behind, strides of 2", []string{"DIMENSION A(20)", "A(3:19:2) = A(1:17:2)"}, "down in 1"},
		{"never meeting, strides of 2", []string{"DIMENSION A(20)", "A(2:20:2) = A(1:19:2)"}, ""},
		{"a negative stride behind", []string{"DIMENSION A(10)", "A(9:1:-1) = A(10:2:-1)"}, "down in 1"},
		{"different columns", []string{"DIMENSION A(4, 4)", "A(:, 1) = A(:, 2)"}, ""},
		{"rows not known to differ", []string{"DIMENSION A(4, 4)", "A(I, :) = A(K, :)"}, ""},
		{"rows not known to differ, a row's tail", []string{"DIMENSION A(4, 4)", "A(I, K:4) = A(I, K:4) - F * A(K, K:4)"}, ""},
		{"rows not known to differ, shifted", []string{"DIMENSION A(4, 4)", "A(I, 2:4) = A(K, 1:3)"}, "down in 1"},
		{"a row and a column not known to differ", []string{"DIMENSION A(4, 4)", "A(I, :) = A(:, K)"}, "through a temporary"},
		{"a column shifted", []string{"DIMENSION A(4, 4)", "A(2:4, 2) = A(1:3, 2)"}, "down in 1"},
		{"the outer dimension decides", []string{"DIMENSION A(4, 4)", "A(1:3, 2:4) = A(2:4, 1:3)"}, "down in 2"},
		{"both directions", []string{"DIMENSION A(10)", "A(2:9) = A(1:8) + A(3:10)"}, "through a temporary"},
		{"a reversal", []string{"DIMENSION A(10)", "A(10:1:-1) = A"}, "through a temporary"},
		{"a row into a column", []string{"DIMENSION A(4, 4)", "A(:, 1) = A(1, :)"}, "through a temporary"},
		{"EQUIVALENCE", []string{"DIMENSION A(10), B(10)", "EQUIVALENCE (A(2), B)", "A = B"}, "through a temporary"},
		{"the target's element", []string{"DIMENSION A(10)", "A = A(1) + A"}, ""},
		{"a CHARACTER element of the target", []string{"CHARACTER*4 C(3)", "C = C(1)"}, "through a temporary"},
		{"the same vector subscript", []string{"DIMENSION A(10)", "INTEGER IV(3)", "A(IV) = A(IV) + 1"}, ""},
		{"a vector subscript and the whole target", []string{"DIMENSION A(10), B(3)", "INTEGER IV(3)", "A(1:3) = A(IV)"}, "through a temporary"},
		{"a vector reading the target", []string{"DIMENSION B(10)", "INTEGER IV(3), JV(10)", "IV = JV(IV)"}, "through a temporary"},
		{"a vector of another array", []string{"DIMENSION A(10), B(10)", "INTEGER IV(10)", "A = B(IV)"}, ""},
		{"substrings of the same elements", []string{"CHARACTER*4 C(3)", "C(:)(2:3) = C(:)(1:2)"}, ""},
	}

	for _, c := range cases {
		lines := make([]string, 0, len(c.lines)+1)
		for _, l := range c.lines {
			lines = append(lines, fixedLine(l))
		}

		lines = append(lines, fixedLine("END"))

		prog, ms := analyzeLines(t, "T.FOR", lines...)
		if len(ms) > 0 {
			t.Errorf("%s: %s: %s", c.name, ms[0].Ident, ms[0].Text)

			continue
		}

		var got string

		for _, st := range prog.Units[0].Body {
			if a, ok := st.(*Assign); ok && a.Order != nil {
				got = strings.Trim(strings.TrimSpace(a.Order.String()), "[]")
			}
		}

		if got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}
