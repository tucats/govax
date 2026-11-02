package opt

import (
	"strings"
	"testing"

	"github.com/tucats/govax/internal/fortran/ir"
)

// expectMacro checks routine S's MACRO, every pass run on it, for what it must and mustn't
// have.
func expectMacro(t *testing.T, lines []string, has, hasnt []string) {
	t.Helper()

	text := allocatedMacro(t, lines...)

	for _, want := range has {
		if !strings.Contains(text, want) {
			t.Errorf("no %q in:\n%s", want, text)
		}
	}

	for _, unwanted := range hasnt {
		if strings.Contains(text, unwanted) {
			t.Errorf("%q in:\n%s", unwanted, text)
		}
	}
}

// TestDoLimits checks which loops close on their limits, and how.
func TestDoLimits(t *testing.T) {
	cases := []struct {
		name       string
		lines      []string
		has, hasnt []string
	}{
		{"a constant limit", []string{
			"SUBROUTINE S", "REAL A(5)", "COMMON A", "DO 10 I = 1, 5", "A(I) = 0.0", "10:CONTINUE", "END",
		}, []string{"AOBLEQ\t#5,R"}, []string{"SOBGTR", "$C1", "$L1"}},

		{"a limit known only when it runs", []string{
			"SUBROUTINE S(N)", "REAL A(5)", "COMMON A", "DO 10 I = 1, N", "A(I) = 0.0", "10:CONTINUE", "END",
		}, []string{"AOBLEQ\tR", "$L1: DO 10's limit", "BGTR"}, []string{"SOBGTR", "$C1"}},

		{"a step of 2", []string{
			"SUBROUTINE S(N)", "REAL A(5)", "COMMON A", "DO 10 I = 1, N, 2", "A(I) = 0.0", "10:CONTINUE", "END",
		}, []string{"ACBL\tR", ",#2,R"}, []string{"$C1"}},

		{"down to 1 is SOBGTR on the variable", []string{
			"SUBROUTINE S(N)", "REAL A(5)", "COMMON A", "DO 10 I = N, 1, -1", "A(I) = 0.0", "10:CONTINUE", "END",
		}, []string{"SOBGTR\tR"}, []string{"$C1", "$L1"}},

		{"down to 0 is SOBGEQ on the variable", []string{
			"SUBROUTINE S(N)", "REAL A(0:5)", "COMMON A", "DO 10 I = N, 0, -1", "A(I) = 0.0", "10:CONTINUE", "END",
		}, []string{"SOBGEQ\tR"}, []string{"$C1", "$L1"}},

		{"a limit a step can't pass keeps its count", []string{
			"SUBROUTINE S", "DO 10 I = 2147483640, 2147483647", "J = J + 1", "10:CONTINUE", "END",
		}, []string{"SOBGTR"}, []string{"AOBLEQ", "ACBL"}},

		{"a step that isn't constant keeps its count", []string{
			"SUBROUTINE S(N, K)", "DO 10 I = 1, N, K", "J = J + 1", "10:CONTINUE", "END",
		}, []string{"; DO 10's count"}, []string{"AOBLEQ", "ACBL"}},

		{"a REAL variable keeps its count", []string{
			"SUBROUTINE S(N)", "DO 10 X = 1.0, 3.0", "J = J + 1", "10:CONTINUE", "END",
		}, []string{"SOBGTR"}, nil},

		{"a short variable keeps its count", []string{
			"SUBROUTINE S(N)", "INTEGER*2 I", "DO 10 I = 1, 3", "J = J + 1", "10:CONTINUE", "END",
		}, []string{"SOBGTR"}, nil},

		{"a call that may change the variable keeps its count", []string{
			"SUBROUTINE S(N)", "DO 10 I = 1, 3", "CALL F(I)", "10:CONTINUE", "END",
		}, []string{"SOBGTR"}, []string{"AOBLEQ"}},

		{"a call that can't change it doesn't", []string{
			"SUBROUTINE S(N)", "DO 10 I = 1, 3", "CALL F", "10:CONTINUE", "END",
		}, []string{"AOBLEQ"}, []string{"SOBGTR"}},

		{"a variable in COMMON and a call keeps its count", []string{
			"SUBROUTINE S(N)", "COMMON I", "DO 10 I = 1, 3", "CALL F", "10:CONTINUE", "END",
		}, []string{"SOBGTR"}, []string{"AOBLEQ"}},

		{"another name for the variable keeps its count", []string{
			"SUBROUTINE S(N)", "EQUIVALENCE (I, K)", "DO 10 I = 1, 3", "K = K + 1", "10:CONTINUE", "END",
		}, []string{"SOBGTR"}, []string{"AOBLEQ"}},

		{"a variable sharing storage with a VOLATILE one keeps its count", []string{
			"SUBROUTINE S(N)", "VOLATILE K", "EQUIVALENCE (I, K)", "DO 10 I = 1, 3", "J = J + 1", "10:CONTINUE", "END",
		}, []string{"SOBGTR"}, []string{"AOBLEQ"}},

		{"a VOLATILE variable keeps its count", []string{
			"SUBROUTINE S(N)", "VOLATILE I", "DO 10 I = 1, 3", "J = J + 1", "10:CONTINUE", "END",
		}, []string{"SOBGTR"}, []string{"AOBLEQ"}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			expectMacro(t, c.lines, c.has, c.hasnt)
		})
	}
}

// TestLimitIsOnce checks that a limit is read once, before the variable is set.
func TestLimitIsOnce(t *testing.T) {
	r := routineOf(t, "S", "SUBROUTINE S(I)", "DO 10 I = 1, I + 2", "J = J + 1", "10:CONTINUE", "END")

	run([]*ir.Routine{r}, Passes[:1])

	text := codeText(r)

	limit, variable := strings.Index(text, "$L1"), strings.Index(text, "I =")
	if limit < 0 || variable < 0 || limit > variable {
		t.Errorf("the limit isn't stored first:\n%s", text)
	}

	if len(r.Counted) != 0 {
		t.Errorf("%d loops still recorded", len(r.Counted))
	}
}

// TestRotate checks which loops have their tests moved, and what comes after them.
func TestRotate(t *testing.T) {
	cases := []struct {
		name       string
		lines      []string
		has, hasnt []string
	}{
		{"a GO TO loop", []string{
			"SUBROUTINE S(N)", "COMMON K", "K = 1", "J = 0",
			"20:IF (J .GT. N) GO TO 30", "J = J + K", "GO TO 20", "30:K = 2", "END",
		}, []string{"; from line ", "BLEQ", "BRB\tS.L20"}, []string{"BGTR\tS.L30"}},

		{"a DO WHILE", []string{
			"SUBROUTINE S(N)", "J = 0", "DO WHILE (J .LT. N)", "J = J + 1", "END DO", "K = J", "END",
		}, []string{"; from line ", "BLSS"}, nil},

		{"a DO loop has its test where it is", []string{
			"SUBROUTINE S(N)", "COMMON A(5)", "DO 10 I = 1, N", "A(I) = 0.0", "10:CONTINUE", "END",
		}, nil, []string{"; from line "}},

		{"a loop with no way out at its top", []string{
			"SUBROUTINE S(N)", "J = 0", "20:J = J + 1", "IF (J .GT. N) GO TO 30", "K = K + J",
			"IF (K .GT. 50) GO TO 30", "GO TO 20", "30:K = 2", "END",
		}, nil, nil},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			expectMacro(t, c.lines, c.has, c.hasnt)
		})
	}
}

// TestRotateLayout checks where a rotated loop's blocks lie: the test after the body, and
// the routine's first block first.
func TestRotateLayout(t *testing.T) {
	r := routineOf(t, "S", "SUBROUTINE S(N)", "J = 0", "20:IF (J .GT. N) GO TO 30", "J = J + 2", "GO TO 20", "30:K = J", "END")
	run([]*ir.Routine{r}, Passes)

	var test, body int

	for i, b := range r.Blocks {
		if b.Label == 20 {
			test = i

			if !b.Rotated {
				t.Errorf("the test isn't marked")
			}
		}

		if j, ok := b.End.(*ir.Jump); ok && j.To != nil && j.To.Label == 20 && i > 0 {
			body = i
		}
	}

	if body == 0 || test != body+1 {
		t.Errorf("the test is block %d, the body's end %d:\n%s", test, body, codeText(r))
	}

	// A loop the routine starts with can't move.
	r = routineOf(t, "S", "SUBROUTINE S(K)", "10:IF (K .GE. 3) RETURN", "K = K + 1", "GO TO 10", "END")
	Rotate(r)

	if r.Blocks[0].Label != 10 {
		t.Errorf("the first block is %s", r.Blocks[0].Name)
	}
}

// TestStepped checks which expressions become a stepped index.
func TestStepped(t *testing.T) {
	cases := []struct {
		name       string
		lines      []string
		has, hasnt []string
	}{
		{"the diagonal", []string{
			"SUBROUTINE S(N)", "COMMON A(60,60)", "DO 10 I = 1, N", "X = X + A(I,I)", "10:CONTINUE", "END",
		}, []string{"ADDL2\t#61,R", "stepped with I"}, nil},

		{"along a row", []string{
			"SUBROUTINE S(N, L)", "COMMON A(60,60)", "K = L", "DO 10 J = 1, N", "X = X + A(K,J)", "10:CONTINUE", "END",
		}, []string{"ADDL2\t#60,R"}, nil},

		{"down a column needs none", []string{
			"SUBROUTINE S(N)", "COMMON A(60)", "DO 10 I = 1, N", "X = X + A(I)", "10:CONTINUE", "END",
		}, nil, []string{"stepped with", "MULL"}},

		{"stepping down", []string{
			"SUBROUTINE S(N)", "COMMON A(60,60)", "DO 10 J = N, 1, -1", "X = X + A(2,J)", "10:CONTINUE", "END",
		}, []string{"SUBL2\t#60,R", "stepped with J"}, nil},

		{"an index going the other way", []string{
			"SUBROUTINE S(N)", "COMMON A(60,60)", "M = N", "DO 10 J = 1, M", "X = X + A(2,M+1-J)", "10:CONTINUE", "END",
		}, []string{"SUBL2\t#60,R", "stepped with J"}, nil},

		{"both sides of a difference", []string{
			"SUBROUTINE S(N)", "COMMON A(60)", "DO 10 I = 1, N", "J = J + (I*3 - I)", "A(1) = J", "10:CONTINUE", "END",
		}, []string{"ADDL2\t#2,R", "stepped with I"}, nil},

		{"another name for the variable isn't followed", []string{
			"SUBROUTINE S(N)", "COMMON A(60,60)", "EQUIVALENCE (J, K)", "J = 1", "20:J = J + 1", "X = X + A(2,K)", "K = K + 1",
			"IF (J .LT. N) GO TO 20", "END",
		}, nil, []string{"stepped with"}},

		{"a variable written otherwise isn't followed", []string{
			"SUBROUTINE S(N)", "COMMON A(60,60)", "J = 1", "20:X = X + A(2,J)", "J = J*2", "IF (J .LT. N) GO TO 20", "END",
		}, nil, []string{"stepped with"}},

		{"a variable a call may change isn't followed", []string{
			"SUBROUTINE S(N)", "COMMON A(60,60)", "DO 10 J = 1, N", "X = X + A(2,J)", "CALL F(J)", "10:CONTINUE", "END",
		}, nil, []string{"stepped with"}},

		{"a variable no call can change is, past a call", []string{
			"SUBROUTINE S(N)", "COMMON A(60,60)", "DO 10 J = 1, N", "X = X + A(2,J)", "CALL F", "10:CONTINUE", "END",
		}, []string{"stepped with J"}, nil},

		{"what isn't safe to read early stays in the loop", []string{
			"SUBROUTINE S(N, K)", "COMMON A(60,60)", "DO 10 J = 1, N", "IF (N .GT. 3) X = X + A(K,J)", "10:CONTINUE", "END",
		}, []string{"stepped with J", "ADDL3\t(R"}, []string{"MULL3\t@S.K", "ADDL3\t@S.K"}},

		{"an adjustable array steps by its stride", []string{
			"SUBROUTINE S(A, M, N)", "REAL A(M,N)", "DO 10 J = 1, N", "X = X + A(2,J)", "10:CONTINUE", "END",
		}, []string{"stepped with J"}, nil},

		{"a CHARACTER array", []string{
			"SUBROUTINE S(N)", "CHARACTER*8 C(10)", "COMMON C", "CHARACTER L", "COMMON L(10)", "DO 10 I = 1, N", "L(I) = C(I)(2:2)", "10:CONTINUE", "END",
		}, []string{"ADDL2\t#8,R"}, nil},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			expectMacro(t, c.lines, c.has, c.hasnt)
		})
	}
}

// TestSteppedBeforeTheStep checks that the addition comes before the variable's step, so
// that AOBLEQ can still be made of the step and its test.
func TestSteppedBeforeTheStep(t *testing.T) {
	text := allocatedMacro(t, "SUBROUTINE S(N)", "COMMON A(60,60)", "DO 10 I = 1, N", "X = X + A(I,I)", "10:CONTINUE", "END")

	add, closing := strings.Index(text, "ADDL2\t#61"), strings.Index(text, "AOBLEQ")
	if add < 0 || closing < 0 || add > closing {
		t.Errorf("the addition isn't before the loop's close:\n%s", text)
	}
}

// TestSteppedSameTwice checks that compiling twice gives the same code: the candidates are
// found in a map, and their order mustn't come from it.
func TestSteppedSameTwice(t *testing.T) {
	lines := []string{
		"SUBROUTINE S(N)", "COMMON A(60,60), B(60,60), C(60,60), D(60,60)", "DO 10 I = 1, N",
		"X = X + A(I,I) + B(I,I) + C(I,I) + D(I,I)", "Y = Y + A(2,I) + B(2,I) + C(2,I) + D(2,I)", "10:CONTINUE", "END",
	}

	first := allocatedMacro(t, lines...)

	for range 20 {
		if again := allocatedMacro(t, lines...); again != first {
			t.Fatalf("two compilations differ:\n%s", firstDifference(first, again))
		}
	}
}
