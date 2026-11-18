package sema

import (
	"strings"
	"testing"
)

// TestArrayConstants checks array constants and constructors
// (docs/planning/PHASE-102 - Fortran 90 array expressions.md, subtask 7):
// each case is a main program's lines, compiled /F90, and what it gives: the
// dump of its last assignment's value, or of its last array constant's value
// if it has no assignment, or its first message, "IDENT: text".
func TestArrayConstants(t *testing.T) {
	cases := []struct {
		name  string
		lines []string
		want  string
	}{
		// Constructors of constants.
		{"a constructor", []string{"INTEGER :: K(3)", "K = (/ 1, 2, 3 /)"}, "(/:I 1 2 3 /)"},
		{"an implied DO", []string{"INTEGER :: K(5)", "K = (/ 0, (I * I, I = 1, 3), 9 /)"}, "(/:I 0 1 4 9 9 /)"},
		{"nested implied DOs", []string{"INTEGER :: K(4)", "K = (/ ((10 * I + J, J = 1, 2), I = 1, 2) /)"}, "(/:I 11 12 21 22 /)"},
		{"a step", []string{"INTEGER :: K(3)", "K = (/ (I, I = 5, 1, -2) /)"}, "(/:I 5 3 1 /)"},
		{"an empty implied DO", []string{"INTEGER :: K(2)", "K = (/ 1, (I, I = 1, 0), 2 /)"}, "(/:I 1 2 /)"},
		{"an array in a constructor", []string{"INTEGER, PARAMETER :: P(2) = (/ 4, 5 /)", "INTEGER :: K(4)", "K = (/ P, P(2:1:-1) /)"}, "(/:I 4 5 5 4 /)"},
		{"CHARACTER values", []string{"CHARACTER*2 :: C(2)", "C = (/ 'AB', 'CD' /)"}, "(/:C 'AB' 'CD' /)"},
		{"elementwise", []string{"INTEGER :: K(3)", "K = (/ 1, 2, 3 /) * 2 + (/ 10, 20, 30 /)"}, "(/:I 12 24 36 /)"},
		{"a comparison", []string{"LOGICAL :: L(3)", "L = (/ 1, 2, 3 /) > 1"}, "(/:L .FALSE. .TRUE. .TRUE. /)"},
		{"an elemental function", []string{"INTEGER :: K(3)", "K = ABS((/ -1, 2, -3 /))"}, "(/:I 1 2 3 /)"},
		{"MAX of an array and a scalar", []string{"INTEGER :: K(3)", "K = MAX((/ -1, 2, -3 /), 0)"}, "(/:I 0 2 0 /)"},

		// Array PARAMETERs.
		{"a PARAMETER", []string{"INTEGER, PARAMETER :: P(3) = (/ 2, 3, 5 /)"}, "(/:I 2 3 5 /)"},
		{"converted", []string{"REAL, PARAMETER :: P(2) = (/ 1, 2 /)"}, "(/:R 1.0 2.0 /)"},
		{"a scalar for every element", []string{"INTEGER, PARAMETER :: P(3) = 7"}, "(/:I 7 7 7 /)"},
		{"the old form", []string{"DIMENSION P(2)", "PARAMETER (P = (/ 1.5, 2.5 /))"}, "(/:R 1.5 2.5 /)"},
		{"CHARACTER*(*)", []string{"CHARACTER*(*), PARAMETER :: W(2) = (/ 'ONE', 'TWO' /)"}, "(/:C 'ONE' 'TWO' /)"},
		{"a padded length", []string{"CHARACTER*4, PARAMETER :: W(2) = (/ 'AB', 'CD' /)"}, "(/:C 'AB  ' 'CD  ' /)"},
		{"an element", []string{"INTEGER, PARAMETER :: P(3) = (/ 2, 3, 5 /)", "J = P(3)"}, "5"},
		{"a section", []string{"INTEGER, PARAMETER :: P(4) = (/ 2, 3, 5, 7 /)", "INTEGER :: K(2)", "K = P(2:4:2)"}, "(/:I 3 7 /)"},
		{"a vector of a constant", []string{"INTEGER, PARAMETER :: P(4) = (/ 2, 3, 5, 7 /)", "INTEGER :: K(2)", "K = P((/ 4, 1 /))"}, "(/:I 7 2 /)"},
		{"an element not constant", []string{"INTEGER, PARAMETER :: P(3) = (/ 2, 3, 5 /)", "J = P(I)"}, "P[I]"},
		{"a section not constant", []string{"INTEGER, PARAMETER :: P(3) = (/ 2, 3, 5 /)", "INTEGER :: K(2)", "K = P(I:I+1)"}, "P[I:(+:I I 1):1]"},
		{"a constant in its bounds", []string{"INTEGER, PARAMETER :: N = 3, P(N) = (/ (I, I = 1, N) /)"}, "(/:I 1 2 3 /)"},

		// RESHAPE and the inquiries.
		{"RESHAPE", []string{"INTEGER, PARAMETER :: M(2, 3) =", "     1 RESHAPE((/ 1, 2, 3, 4, 5, 6 /), (/ 2, 3 /))"}, "(/:I(2, 3) 1 2 3 4 5 6 /)"},
		{"RESHAPE with ORDER", []string{"INTEGER, PARAMETER :: M(2, 3) =", "     1 RESHAPE((/ 1, 2, 3, 4, 5, 6 /), (/ 2, 3 /),", "     2 ORDER=(/ 2, 1 /))"}, "(/:I(2, 3) 1 4 2 5 3 6 /)"},
		{"RESHAPE with PAD", []string{"INTEGER, PARAMETER :: M(2, 2) =", "     1 RESHAPE((/ 1 /), (/ 2, 2 /), PAD=(/ 0, 9 /))"}, "(/:I(2, 2) 1 0 9 0 /)"},
		{"SIZE", []string{"DIMENSION A(3, 0:4)", "INTEGER, PARAMETER :: N = SIZE(A), M = SIZE(A, 2)", "J = N + M"}, "20"},
		{"LBOUND and UBOUND", []string{"DIMENSION A(3, 0:4)", "J = LBOUND(A, 2) * 10 + UBOUND(A, DIM=2)"}, "4"},
		{"SHAPE", []string{"DIMENSION A(3, 0:4)", "INTEGER :: K(2)", "K = SHAPE(A)"}, "(/:I 3 5 /)"},
		{"LBOUND without DIM", []string{"DIMENSION A(3, 0:4)", "INTEGER :: K(2)", "K = LBOUND(A)"}, "(/:I 1 0 /)"},
		{"an array PARAMETER's bounds", []string{"INTEGER, PARAMETER :: P(-1:1) = 0", "J = UBOUND(P, 1)"}, "1"},

		// Constructors not constant.
		{"a constructor of values", []string{"INTEGER :: K(3)", "K = (/ I, J, 3 /)"}, "(/:I I J 3 /)"},
		{"an implied DO of values", []string{"REAL :: A(4)", "A = (/ (I * X, I = 1, 4) /)"}, "(/:R ((*:R (REAL I) X), I = 1, 4, 1) /)"},
		{"its own variable", []string{"REAL :: A(2)", "I = 7", "A = (/ (I, I = 1, 2) /)", "J = I"}, "I"},

		// Errors.
		{"two types", []string{"REAL :: A(2)", "A = (/ 1, 2.0 /)"}, "CONSTRUCTTYPE: an array constructor's values must have one type, and this one is REAL where the first is INTEGER"},
		{"two lengths", []string{"CHARACTER*2 :: C(2)", "C = (/ 'AB', 'C' /)"}, "CONSTRUCTLEN: an array constructor's CHARACTER values must have one length, and this one has 1 where the first has 2"},
		{"a PARAMETER of another shape", []string{"INTEGER, PARAMETER :: P(3) = (/ 1, 2 /)"}, "NOTCONFORM: the shapes (3) and (2) don't conform"},
		{"an initial value of another shape", []string{"INTEGER :: K(3) = (/ 1, 2 /)"}, "NOTCONFORM: the shapes (3) and (2) don't conform"},
		{"an initial array for a scalar", []string{"INTEGER :: K = (/ 1, 2 /)"}, "NOTSCALAR: an array is used where a scalar is required"},
		{"a scalar PARAMETER given an array", []string{"INTEGER, PARAMETER :: N = (/ 1, 2 /)"}, "NOTSCALAR: an array is used where a scalar is required"},
		{"a constant subscript out of range", []string{"INTEGER, PARAMETER :: P(3) = (/ 2, 3, 5 /)", "J = P(4)"}, "SUBSCRIPTRANGE: subscript 4 of P is past its upper bound, 3"},
		{"RESHAPE short", []string{"INTEGER, PARAMETER :: M(2, 2) = RESHAPE((/ 1, 2 /), (/ 2, 2 /))"}, "BADRESHAPE: this RESHAPE can't be computed: SOURCE has 2 elements, too few for the shape, and no PAD"},
		{"a DIM out of range", []string{"DIMENSION A(3)", "J = SIZE(A, 2)"}, "DIMRANGE: SIZE's DIM, 2, isn't a dimension of an array of rank 1"},
		{"a many-one section", []string{"DIMENSION A(5)", "A((/ 1, 2, 1 /)) = 0"}, "MANYONE: the vector subscript of A names element 1 more than once, so the section can't be given values"},
		{"a constructor of unknown size in an expression", []string{"REAL :: A(4)", "A(1:N) = (/ (X, I = 1, N) /) + A(1:N)"}, "NOTIMPL: an array constructor of a size not known when compiling, but as an array's whole value, (Phase 104) is not implemented yet"},
		{"an implied DO's REAL variable", []string{"REAL :: A(2)", "A = (/ (X, X = 1, 2) /)"}, "CTORDOVAR: an array constructor's implied-DO variable X must be INTEGER"},
	}

	for _, c := range cases {
		lines := make([]string, 0, len(c.lines)+1)
		for _, l := range c.lines {
			if strings.HasPrefix(l, "     ") {
				// A continuation line, as it is.
				lines = append(lines, l)

				continue
			}

			lines = append(lines, fixedLine(l))
		}

		lines = append(lines, fixedLine("END"))

		prog, ms := analyzeOpts(t, Options{F90: true}, lines...)

		var got string

		switch {
		case len(ms) > 0:
			got = ms[0].Ident + ": " + ms[0].Text
		default:
			for _, st := range prog.Units[0].Body {
				if a, ok := st.(*Assign); ok {
					got = DumpExpr(a.Value)
				}
			}

			if got == "" {
				for _, s := range prog.Units[0].Symbols {
					if s.ArrayValue != nil {
						got = DumpExpr(s.ArrayValue)
					}
				}
			}
		}

		if !strings.EqualFold(got, c.want) && got != c.want {
			t.Errorf("%s: got\n%s\nwant\n%s", c.name, got, c.want)
		}
	}
}
