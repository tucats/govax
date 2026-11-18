package sema

import (
	"strings"
	"testing"
)

// TestWhere checks WHERE (docs/planning/PHASE-102 - Fortran 90 array
// expressions.md, subtask 10): each case is a main program's lines after the
// declarations, compiled /F90, and a summary of its first Where, "mask" when
// the loops compute the mask or "temp" when it goes into a temporary, then each
// assignment's order ("" for up, as the dump shows it), or the first message,
// "IDENT: text".
func TestWhere(t *testing.T) {
	decls := []string{
		"REAL A(10), B(10), C(5), M(3, 4), Q(3, 4)",
		"INTEGER K(10)",
		"LOGICAL L(10)",
		"COMMON /BLK/ X(10)",
	}

	cases := []struct {
		name  string
		lines []string
		want  string
	}{
		{"one statement", []string{"WHERE (A > 0) A = SQRT(A)"}, "mask; "},
		{"another array", []string{"WHERE (A > 0) B = 1 / A"}, "mask; "},
		{"a construct", []string{"WHERE (A > 0)", "B = A", "ELSEWHERE", "B = 0", "END WHERE"}, "mask; ; "},
		{"ELSEWHERE only", []string{"WHERE (L)", "ELSEWHERE", "A = 0", "END WHERE"}, "mask; "},
		{"an INTEGER mask", []string{"WHERE (K) A = 0"}, "mask; "},
		{"a mask read later", []string{"WHERE (A > 0)", "A = -A", "B = 1", "END WHERE"}, "temp; ; "},
		{"its last write", []string{"WHERE (A > 0)", "B = 1", "A = -A", "END WHERE"}, "mask; ; "},
		{"the mask ordered", []string{"WHERE (A(1:9) > 0) A(2:10) = 1"}, "mask;   [down in 1]"},
		{"opposite ways", []string{"WHERE (A(2:10) > 0) A(1:9) = A(1:9) + A(1:9)"}, "mask; "},
		{"mask against value", []string{"WHERE (A(3:10) > 0) A(2:9) = A(1:8)"}, "temp;   [down in 1]"},
		{"through a temporary", []string{"WHERE (B > 0) A = A(10:1:-1)"}, "mask;   [through a temporary]"},
		{"temporary and mask", []string{"WHERE (A > 0) A = A(10:1:-1)"}, "temp;   [through a temporary]"},
		{"a scalar of the target", []string{"WHERE (A > A(1)) A = 0"}, "mask; "},
		{"common storage", []string{"WHERE (X > 0)", "X = 1", "B = 2", "END WHERE"}, "temp; ; "},
		{"a function twice", []string{"WHERE (A > F(1.0))", "B = 1", "C(1:5) = 2", "END WHERE"}, "NOTCONFORM: the shapes (10) and (5) don't conform"},
		{"a function twice, conforming", []string{"WHERE (A > F(1.0))", "B = 1", "K = 2", "END WHERE"}, "temp; ; "},
		{"a function once", []string{"WHERE (A > F(1.0)) B = 1"}, "mask; "},
		{"two dimensions", []string{"WHERE (M /= 0) Q = 1 / M"}, "mask; "},
		{"a constructor", []string{"WHERE (L(1:3)) C(1:3) = (/ 1.0, 2.0, 3.0 /)"}, "mask; "},
		{"in a logical IF", []string{"IF (A(1) > 0) WHERE (A > 0) B = 1"}, ""},

		{"a scalar mask", []string{"WHERE (A(1) > 0) B = 1"}, "WHEREMASK: a WHERE's mask must be a LOGICAL array, and this one is a scalar"},
		{"a REAL mask", []string{"WHERE (A) B = 1"}, "WHEREMASK: a WHERE's mask must be a LOGICAL array, and this one is a REAL array"},
		{"a scalar target", []string{"WHERE (L) Y = 1"}, "WHERETARGET: an assignment in a WHERE must be to an array of the mask's shape, not to a scalar"},
		{"shapes", []string{"WHERE (L) C = 1"}, "NOTCONFORM: the shapes (10) and (5) don't conform"},
		{"not an assignment", []string{"WHERE (L)", "CALL S", "END WHERE"}, "WHEREBODY: only assignments may be in a WHERE construct, not a CALL statement"},
		{"a DO inside", []string{"WHERE (L)", "DO I = 1, 2", "END DO", "END WHERE"}, "WHEREBODY: only assignments may be in a WHERE construct, not a DO statement"},
		{"nested", []string{"WHERE (L)", "WHERE (L) A = 1", "END WHERE"}, "F95ONLY: a WHERE statement inside a WHERE construct is Fortran 95's, not Fortran 90's, and isn't compiled yet (Phase 112)"},
		{"nested construct", []string{"WHERE (L)", "WHERE (L)", "A = 1", "END WHERE", "END WHERE"}, "F95ONLY: a WHERE construct inside a WHERE construct is Fortran 95's, not Fortran 90's, and isn't compiled yet (Phase 112)"},
		{"ELSEWHERE (mask)", []string{"WHERE (L)", "ELSEWHERE (A > 0)", "END WHERE"}, "F95ONLY: ELSEWHERE with a mask is Fortran 95's, not Fortran 90's, and isn't compiled yet (Phase 112)"},
		{"two ELSEWHEREs", []string{"WHERE (L)", "ELSEWHERE", "ELSEWHERE", "END WHERE"}, "ELSEWHEREORDER: this WHERE construct has an ELSEWHERE statement already, at line 6"},
		{"no END WHERE", []string{"WHERE (L)", "A = 1"}, "NOENDWHERE: this WHERE construct has no END WHERE statement"},
		{"END WHERE alone", []string{"END WHERE"}, "NOWHERE: this END WHERE statement is not in a WHERE construct"},
		{"an IF across", []string{"IF (A(1) > 0) THEN", "WHERE (L)", "END IF", "END WHERE"}, "WHEREBODY: only assignments may be in a WHERE construct, not an END IF statement"},
		{"a jump in", []string{"GO TO 10", "WHERE (L)", "10 A = 1", "END WHERE"}, "INTOBLOCK: this jump to label 10 goes into a WHERE construct from outside it"},
		{"a jump to END WHERE", []string{"GO TO 10", "WHERE (L)", "A = 1", "10 END WHERE"}, "INTOBLOCK: this jump to label 10 goes into a WHERE construct from outside it"},
		{"a jump to WHERE", []string{"GO TO 10", "10 WHERE (L)", "A = 1", "END WHERE"}, "mask; "},
		{"a temporary not known", []string{"N = 3", "WHERE (A(1:N) > 0)", "A(1:N) = -A(1:N)", "B(1:N) = 1", "END WHERE"},
			"temp; ; "},
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
			got = whereSummary(prog.Units[0].Body)
		}

		if c.want == "" && len(ms) == 0 {
			continue
		}

		if got != c.want {
			t.Errorf("%s: got\n%s\nwant\n%s", c.name, got, c.want)
		}
	}
}

// whereSummary is TestWhere's summary of the first Where in body.
func whereSummary(body []Stmt) string {
	for _, st := range body {
		w, ok := st.(*Where)
		if !ok {
			continue
		}

		parts := []string{"mask"}
		if w.MaskTemp != nil {
			parts[0] = "temp"
		}

		for _, arm := range w.arms() {
			for _, s := range arm.Body {
				parts = append(parts, s.(*Assign).Order.String())
			}
		}

		return strings.Join(parts, "; ")
	}

	return "no WHERE"
}
