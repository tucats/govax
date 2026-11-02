package opt

import (
	"slices"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/fortran/ir"
)

// movedValues are what the pass moved out of loops in a routine, as the dump writes
// them.
func movedValues(r *ir.Routine) []string {
	var list []string

	for _, b := range r.Blocks {
		for _, st := range b.Stmts {
			if a, ok := st.(*ir.Assign); ok && a.Moved != nil {
				list = append(list, ir.ExprText(a.Src))
			}
		}
	}

	return list
}

// inLoop puts the statements in a subroutine with values no earlier pass can know:
// J, M, and N are locals set from COMMON, Y and Z too, and the array X.
func inLoop(lines ...string) []string {
	all := []string{
		"SUBROUTINE S(IB, RB)",
		"INTEGER I, J, M, N, K(10), IB, IC, IV(3)",
		"REAL X(10), Y, Z, RB, RV(2)",
		"COMMON /V/ IV, RV, IC",
		"CALL INIT",
		"J = IV(1)",
		"M = IV(2)",
		"N = IV(3)",
		"Y = RV(1)",
		"Z = RV(2)",
	}

	return append(append(all, lines...), "END")
}

// TestInvariants checks what moves out of a loop and what stays.
func TestInvariants(t *testing.T) {
	cases := []struct {
		name  string
		lines []string
		want  []string
	}{
		{"an index's part", []string{"DO 10 I = 1, N", "K(J*M+I) = I", "10:CONTINUE"}, []string{"mul.L(J, M)"}},
		{"an index alone isn't worth it", []string{"DO 10 I = 1, N", "K(J*5) = I", "10:CONTINUE"}, nil},
		{"an index alone that saves a read", []string{"DO 10 I = 1, N", "K(J*M) = I", "10:CONTINUE"}, []string{"mul.L(J, M)"}},
		{"an index with two operations", []string{"DO 10 I = 1, N", "K(J*M+N) = I", "10:CONTINUE"}, []string{"add.L(mul.L(J, M), N)"}},
		{"used twice, moved once", []string{"DO 10 I = 1, N", "K(J*M+I) = K(J*M+I+1)", "10:CONTINUE"}, []string{"mul.L(J, M)"}},
		{"what the loop changes stays", []string{"DO 10 I = 1, N", "K(J*M+I) = I", "J = J + 1", "10:CONTINUE"}, nil},
		{"the DO variable stays", []string{"DO 10 I = 1, N", "K(I*M+1) = I", "10:CONTINUE"}, nil},
		{"integer arithmetic moves from under a test",
			[]string{"DO 10 I = 1, N", "IF (I .GT. M) K(J*M+I) = I", "10:CONTINUE"}, []string{"mul.L(J, M)"}},
		{"a division on the first trip", []string{"DO 10 I = 1, N", "X(I) = I + Y/Z", "10:CONTINUE"}, []string{"div.F(Y, Z)"}},
		{"a division under a test", []string{"DO 10 I = 1, N", "IF (Z .NE. 0.0) X(I) = I + Y/Z", "10:CONTINUE"}, nil},
		{"an integer division under a test", []string{"DO 10 I = 1, N", "IF (M .NE. 0) K(I) = I + J/M", "10:CONTINUE"}, nil},
		{"a floating operation under a test", []string{"DO 10 I = 1, N", "IF (I .GT. M) X(I) = I + Y*Z", "10:CONTINUE"}, nil},
		{"a division after an exit", []string{"DO 10 I = 1, N", "IF (Z .EQ. 0.0) GO TO 20", "X(I) = I + Y/Z", "10:CONTINUE", "20:CONTINUE"}, nil},
		{"a division after a call", []string{"DO 10 I = 1, N", "CALL CHECK", "X(I) = I + Y/Z", "10:CONTINUE"}, nil},
		{"a division after a call under a test",
			[]string{"DO 10 I = 1, N", "IF (Z .EQ. 0.0) CALL GIVEUP", "X(I) = I + Y/Z", "10:CONTINUE"}, nil},
		{"a division before a call", []string{"DO 10 I = 1, N", "X(I) = I + Y/Z", "CALL NEXT", "10:CONTINUE"}, []string{"div.F(Y, Z)"}},
		{"a division in the test's second operand",
			[]string{"DO 10 I = 1, N", "IF (Z .NE. 0.0 .AND. Y/Z .GT. 1.0) K(I) = I", "10:CONTINUE"}, nil},
		{"one sure use makes the others safe",
			[]string{"DO 10 I = 1, N", "X(I) = I + Y/Z", "IF (I .GT. M) K(I) = I + Y/Z", "10:CONTINUE"}, []string{"div.F(Y, Z)"}},
		{"a dummy read on the first trip", []string{"DO 10 I = 1, N", "K(I) = I + IB", "10:CONTINUE"}, []string{"@IB"}},
		{"a dummy read under a test", []string{"DO 10 I = 1, N", "IF (I .GT. M) K(I) = I + IB", "10:CONTINUE"}, nil},
		{"an element under a test", []string{"DO 10 I = 1, N", "IF (I .GT. M) X(I) = I + K(J+M)", "10:CONTINUE"}, []string{"add.L(J, M)"}},
		{"COMMON in a loop with a call", []string{"DO 10 I = 1, N", "K(IC*M+I) = I", "CALL BUMP", "10:CONTINUE"}, nil},
		{"a local in a loop with a call", []string{"DO 10 I = 1, N", "K(J*M+I) = I", "CALL NEXT", "10:CONTINUE"}, []string{"mul.L(J, M)"}},
		{"a local passed in the loop", []string{"DO 10 I = 1, N", "K(J*M+I) = I", "CALL NEXT(M)", "10:CONTINUE"}, nil},
		{"a local passed elsewhere", []string{"CALL NEXT(M)", "DO 10 I = 1, N", "K(J*M+I) = I", "CALL NEXT", "10:CONTINUE"}, nil},
		{"an array the loop stores in", []string{"DO 10 I = 1, N", "K(I) = I + K(J)", "10:CONTINUE"}, nil},
		{"a GO TO loop", []string{"I = 0", "10:I = I + 1", "K(J*M+I) = I", "IF (I .LT. N) GO TO 10"}, []string{"mul.L(J, M)"}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := improved(t, "S", inLoop(c.lines...)...)

			if got := movedValues(r); !slices.Equal(got, c.want) {
				t.Errorf("moved %q, want %q:\n%s", got, c.want, codeText(r))
			}
		})
	}
}

// TestNestedLoops checks the example at the top of invariant.go: each value goes as far
// out as it can, a whole element comes out of the inner loop, and C's place is found once.
func TestNestedLoops(t *testing.T) {
	r := improved(t, "S",
		"SUBROUTINE S(N)",
		"INTEGER N, I, J, K",
		"PARAMETER (M = 60)",
		"REAL A(M,M), B(M,M), C(M,M)",
		"COMMON A, B, C",
		"DO 50 J = 1, M",
		"DO 40 K = 1, M",
		"DO 30 I = 1, M",
		"C(I,J) = C(I,J) + A(I,K)*B(K,J)",
		"30:CONTINUE",
		"40:CONTINUE",
		"50:CONTINUE",
		"END")

	got := movedValues(r)

	if len(got) != 3 || got[0] != "mul.L(J, 60)" || got[1] != "mul.L(K, 60)" || !strings.HasPrefix(got[2], "B-") {
		t.Fatalf("moved %q:\n%s", got, codeText(r))
	}

	// How many loops each is computed in: J*60 in J's, the rest in K's too.
	g := NewGraph(r)
	depths := []int{}

	for _, b := range r.Blocks {
		for _, st := range b.Stmts {
			if a, ok := st.(*ir.Assign); ok && a.Moved != nil {
				depths = append(depths, g.LoopOf(b).Depth)
			}
		}
	}

	if !slices.Equal(depths, []int{1, 2, 2}) {
		t.Errorf("computed at depths %v:\n%s", depths, codeText(r))
	}

	if len(inPlaceLines(r)) != 1 {
		t.Errorf("C's place isn't found once:\n%s", codeText(r))
	}
}

// TestValueInsideAnother checks that a value inside another found elsewhere is still
// read, in the other's definition as well: J*M, and J+J*M. Nothing is left that no one
// reads, which would make a second run of the passes different.
func TestValueInsideAnother(t *testing.T) {
	r := improved(t, "S", inLoop("DO 10 I = 1, N", "K(J*M+I) = I", "K(J+J*M) = I", "10:CONTINUE")...)

	want := []string{"mul.L(J, M)", "add.L(J, $H1)"}
	if got := movedValues(r); len(got) != 2 || got[0] != want[0] {
		t.Errorf("moved %q, want %q:\n%s", got, want, codeText(r))
	}

	if text := codeText(r); strings.Count(text, "mul.L(J, M)") != 1 {
		t.Errorf("J*M is computed twice:\n%s", text)
	}
}

// TestVolatileNotMoved checks that nothing reading a VOLATILE variable moves.
func TestVolatileNotMoved(t *testing.T) {
	r := improved(t, "S",
		"SUBROUTINE S(N)",
		"INTEGER N, I, J, M, K(10)",
		"VOLATILE J",
		"COMMON K, M",
		"DO 10 I = 1, N",
		"K(J*M+I) = I",
		"10:CONTINUE",
		"END")

	if got := movedValues(r); len(got) != 0 {
		t.Errorf("moved %q:\n%s", got, codeText(r))
	}
}

// TestEquivalenceChanges checks that a store under another name keeps a value in the
// loop, and one to the other half of nothing it reads doesn't.
func TestEquivalenceChanges(t *testing.T) {
	r := improved(t, "S",
		"SUBROUTINE S(N)",
		"INTEGER N, I, IW, JW, M, K(10)",
		"INTEGER*2 HW(2)",
		"EQUIVALENCE (IW, HW)",
		"COMMON K, M",
		"IW = M",
		"JW = M",
		"DO 10 I = 1, N",
		"K(IW*JW+I) = I",
		"HW(2) = I",
		"10:CONTINUE",
		"END")

	if got := movedValues(r); len(got) != 0 {
		t.Errorf("moved %q:\n%s", got, codeText(r))
	}
}

// TestNoWayBefore checks the loops that can't have a preheader: an ENTRY statement at
// the top, and an assigned GO TO to the top from outside.
func TestNoWayBefore(t *testing.T) {
	cases := [][]string{
		{"SUBROUTINE S(J, M)", "INTEGER J, M, K(10), I", "COMMON K, I", "I = 0", "ENTRY T(J, M)", "10:I = I + 1", "K(J*M+I) = I", "IF (I .LT. 3) GO TO 10", "END"},
		{"SUBROUTINE S(J, M)", "INTEGER J, M, K(10), I, IA", "COMMON K, I", "ASSIGN 10 TO IA", "GO TO IA, (10)", "10:I = I + 1", "K(J*M+I) = I", "IF (I .LT. 3) GO TO 10", "END"},
	}

	for _, lines := range cases {
		r := improved(t, "S", lines...)

		if got := movedValues(r); len(got) != 0 {
			t.Errorf("moved %q:\n%s", got, codeText(r))
		}
	}
}

// TestLoopAtTheStart checks a loop whose top is the routine's first statement: a new
// first block goes before it, and the back edge still goes to the old one.
func TestLoopAtTheStart(t *testing.T) {
	r := improved(t, "S",
		"SUBROUTINE S(M, N)",
		"INTEGER M, N, K",
		"COMMON K",
		"10:K = K + M*N",
		"IF (K .LT. 100) GO TO 10",
		"END")

	if got := movedValues(r); len(got) != 1 {
		t.Fatalf("moved %q:\n%s", got, codeText(r))
	}

	if a, ok := r.Blocks[0].Stmts[0].(*ir.Assign); !ok || a.Moved == nil {
		t.Errorf("the first block doesn't compute it:\n%s", codeText(r))
	}
}

// TestMovedComment checks that moved code names the line it came from.
func TestMovedComment(t *testing.T) {
	text := macroOf(t, inLoop("DO 10 I = 1, N", "K(J*M+I) = I", "10:CONTINUE")...)

	if !strings.Contains(text, "; from line 12") {
		t.Errorf("no comment naming the line:\n%s", text)
	}
}

// TestDeferredIndex checks the example at the top of deferred.go.
func TestDeferredIndex(t *testing.T) {
	text := macroOf(t,
		"SUBROUTINE S(A, N, J, K)",
		"INTEGER N, J, K(10)",
		"REAL A(N), T",
		"T = A(J)",
		"A(J) = A(J+1)",
		"CALL SUB(A(J))",
		"END")

	for _, want := range []string{"MOVF\t@S.$B1+16[R0],S.T", "PUSHAL\t@S.$B1+16[R0]"} {
		if !strings.Contains(text, want) {
			t.Errorf("no %q in:\n%s", want, text)
		}
	}

	if strings.Contains(text, "4(R") {
		t.Errorf("A(J+1) has an offset still:\n%s", text)
	}
}
