package sema

import (
	"strconv"
	"strings"
	"testing"
)

// TestFusable checks Fusable (fusion.go): each case is a main program's array
// assignments, after the declarations, compiled /F90, and how lowering would group
// them, each group's statements' numbers joined by "+" ("1+2 3": the first two share a
// nest, the third has its own).
func TestFusable(t *testing.T) {
	decls := []string{
		"REAL A(10), B(10), C(10), D(5), M(3, 4), Q(3, 4)",
		"REAL, ALLOCATABLE :: X(:), Y(:)",
		"CHARACTER*4 S(10), U(10)",
		"INTEGER K(10), N",
		"COMMON /BLK/ P(10), R(10)",
	}

	cases := []struct {
		name  string
		lines []string
		want  string
	}{
		{"the same element", []string{"A = B + 1", "C = A * 2"}, "1+2"},
		{"three", []string{"A = B + 1", "C = A * 2", "B = C - A"}, "1+2+3"},
		{"apart", []string{"A = 1", "B = 2"}, "1+2"},
		{"two dimensions", []string{"M = Q + 1", "Q = M * M"}, "1+2"},
		{"sections of one extent", []string{"A(1:5) = 1", "B(6:10) = A(1:5)"}, "1+2"},
		{"shapes differ", []string{"A = 1", "D = 2"}, "1 2"},
		{"read ahead", []string{"A(1:9) = B(2:10)", "C(1:9) = A(2:10)"}, "1 2"},
		{"read behind", []string{"A(2:10) = B(1:9)", "C(2:10) = A(1:9)"}, "1 2"},
		{"written ahead of a read", []string{"C(1:9) = A(2:10)", "A(1:9) = 0"}, "1 2"},
		{"a scalar of the target", []string{"A = B", "C = A(10) + B"}, "1 2"},
		{"a bound of the target", []string{"K = 3", "A(1:K(1)) = 0"}, "1 2"},
		{"a function", []string{"A = 1", "C = F(1.0) + A"}, "1 2"},
		{"an elemental intrinsic", []string{"A = 1", "C = SQRT(A)"}, "1+2"},
		{"through a temporary", []string{"A = 1", "C = C(10:1:-1)"}, "1 2"},
		{"CHARACTER", []string{"S = 'X'", "U = S"}, "1 2"},
		{"common storage", []string{"P = 1", "R = P"}, "1+2"},
		{"the same allocatable", []string{"X = 1", "X = X + 1"}, "1+2"},
		{"two allocatables", []string{"X = 1", "Y = 2"}, "1 2"},
		{"a vector subscript", []string{"A = 1", "C = A(K)"}, "1 2"},
		{"a scalar between", []string{"A = 1", "N = 2", "C = A"}, "1 2 3"},
	}

	for _, c := range cases {
		lines := make([]string, 0, len(decls)+len(c.lines)+1)
		for _, l := range append(append([]string{}, decls...), c.lines...) {
			lines = append(lines, fixedLine(l))
		}

		lines = append(lines, fixedLine("END"))

		prog, ms := analyzeOpts(t, Options{F90: true}, lines...)
		if len(ms) > 0 {
			t.Errorf("%s: %s: %s", c.name, ms[0].Ident, ms[0].Text)

			continue
		}

		if got := fusionGroups(prog.Units[0].Body); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

// fusionGroups is TestFusable's summary of a body, as lowering groups it: each
// statement numbered from 1, those sharing a nest joined by "+".
func fusionGroups(body []Stmt) string {
	var groups []string

	var group []*Assign

	n := 0

	for _, st := range body {
		if _, end := st.(*End); end {
			break
		}

		n++

		a, ok := st.(*Assign)
		if ok && a.Target.Shape() != nil && len(group) > 0 && Fusable(group, a) {
			group = append(group, a)
			groups[len(groups)-1] += "+" + itoa(n)

			continue
		}

		group = nil
		if ok && a.Target.Shape() != nil {
			group = []*Assign{a}
		}

		groups = append(groups, itoa(n))
	}

	return strings.Join(groups, " ")
}

func itoa(n int) string { return strconv.Itoa(n) }

// TestFusableWhere checks FusableWhere: a construct's assignments share one nest when
// each is plain and none changes what the mask reads.
func TestFusableWhere(t *testing.T) {
	decls := []string{"REAL A(10), B(10), C(10)"}

	cases := []struct {
		name  string
		lines []string
		want  bool
	}{
		{"both arms", []string{"WHERE (A > 0)", "B = SQRT(A)", "ELSEWHERE", "B = 0", "END WHERE"}, true},
		{"two in an arm", []string{"WHERE (A > 0)", "B = A * A", "C = B + 1", "END WHERE"}, true},
		{"one assignment", []string{"WHERE (A > 0) B = 1"}, false},
		{"the mask changed", []string{"WHERE (A > 0)", "A = 0", "C = 1", "END WHERE"}, false},
		{"read ahead", []string{"WHERE (A(1:9) > 0)", "B(1:9) = 1", "C(1:9) = B(2:10)", "END WHERE"}, false},
	}

	for _, c := range cases {
		lines := make([]string, 0, len(decls)+len(c.lines)+1)
		for _, l := range append(append([]string{}, decls...), c.lines...) {
			lines = append(lines, fixedLine(l))
		}

		lines = append(lines, fixedLine("END"))

		prog, ms := analyzeOpts(t, Options{F90: true}, lines...)
		if len(ms) > 0 {
			t.Errorf("%s: %s: %s", c.name, ms[0].Ident, ms[0].Text)

			continue
		}

		var w *Where

		for _, st := range prog.Units[0].Body {
			if x, ok := st.(*Where); ok {
				w = x

				break
			}
		}

		if got := w != nil && FusableWhere(w); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}
