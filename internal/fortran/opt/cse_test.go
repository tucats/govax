package opt

import (
	"strings"
	"testing"

	"github.com/tucats/govax/internal/fortran/codegen"
	"github.com/tucats/govax/internal/fortran/ir"
)

// inPlaceLines are a routine's statements marked in place.
func inPlaceLines(r *ir.Routine) []string {
	var lines []string

	for _, line := range strings.Split(codeText(r), "\n") {
		if strings.HasSuffix(line, " in place") {
			lines = append(lines, line)
		}
	}

	return lines
}

// temporaries counts the routine's stores to the pass's temporaries.
func temporaries(r *ir.Routine) int {
	return strings.Count(codeText(r), "assign $U")
}

// TestInPlace checks which assignments change their place where it is: those whose
// value is an operation on that very element, on the side a two-operand instruction
// can take it, and not a store to another element.
func TestInPlace(t *testing.T) {
	cases := []struct {
		statement string
		marked    bool
	}{
		{"A(I) = A(I) + X", true},
		{"A(I) = X + A(I)", true},
		{"A(I) = A(I) - X", true},
		{"A(I) = X - A(I)", false},
		{"A(I) = A(I)/X", true},
		{"A(I) = X/A(I)", false},
		{"A(I) = A(I)*X", true},
		{"A(I) = A(J) + X", false},
		{"A(I) = A(I+1) + X", false},
		{"A(I) = A(I) + A(I)", true},
		{"K(I) = K(I) + 1", true},
		{"K(I) = K(I) - 1", true},
		{"K(I) = K(I) .OR. 3", true},
		{"K(I) = IEOR(K(I), 3)", true},
		{"K(I) = K(I) .AND. 3", true},
		{"K(I) = 3 - K(I)", false},
	}

	for _, c := range cases {
		r := improved(t, "S",
			"SUBROUTINE S(X, I, J)",
			"REAL A(10), X",
			"INTEGER K(10)",
			"COMMON A, K",
			c.statement,
			"END")

		if got := len(inPlaceLines(r)) > 0; got != c.marked {
			t.Errorf("%s: in place %v, want %v:\n%s", c.statement, got, c.marked, codeText(r))
		}
	}
}

// TestInPlaceCode checks what the marked statement becomes, and that the place's
// subscript is worked out once: an element of a matrix, and of a dummy argument.
func TestInPlaceCode(t *testing.T) {
	for _, c := range []struct {
		lines []string
		want  string
	}{
		{[]string{"SUBROUTINE S(X, I, J)", "REAL C(5,5), X", "COMMON C", "C(I,J) = C(I,J) + X", "END"}, "ADDF2"},
		{[]string{"SUBROUTINE S(C, X, I, J)", "REAL C(5,5), X", "C(I,J) = C(I,J) - X", "END"}, "SUBF2"},
		{[]string{"SUBROUTINE S(K, I)", "INTEGER K(5)", "K(I) = K(I) + 1", "END"}, "INCL"},
		{[]string{"SUBROUTINE S(K, I)", "INTEGER K(5)", "K(I) = K(I) - 1", "END"}, "DECL"},
		{[]string{"SUBROUTINE S(K, I)", "INTEGER K(5)", "K(I) = K(I) .AND. 3", "END"}, "BICL2"},
	} {
		text := macroOf(t, c.lines...)
		if !strings.Contains(text, "\t"+c.want+"\t") {
			t.Errorf("no %s in:\n%s", c.want, text)
		}

		if n := strings.Count(text, "MULL3"); n > 1 {
			t.Errorf("%d subscripts in:\n%s", n, text)
		}
	}
}

// macroOf is a program's code as the generator writes it after the IR passes.
func macroOf(t *testing.T, lines ...string) string {
	t.Helper()

	r := improved(t, "S", lines...)

	var w codegen.Writer

	codegen.Generate(&w, []*ir.Routine{r}, nil, codegen.Options{})

	return strings.Join(w.Lines(), "\n")
}

// TestInPlaceVolatile checks that a VOLATILE element keeps its load and store.
func TestInPlaceVolatile(t *testing.T) {
	r := improved(t, "S",
		"SUBROUTINE S(X, I)",
		"REAL A(10), X",
		"VOLATILE A",
		"COMMON A",
		"A(I) = A(I) + X",
		"END")

	if lines := inPlaceLines(r); len(lines) != 0 {
		t.Errorf("marked: %v", lines)
	}
}

// TestMerged checks the example at the top of cse.go: a value used three times.
func TestMerged(t *testing.T) {
	r := improved(t, "S",
		"SUBROUTINE S(X, Y, Z, B, I, J)",
		"REAL A(5,5), X, Y, Z, B",
		"COMMON A",
		"X = A(I,J)*B + 1.0",
		"Y = A(I,J)*B - 1.0",
		"Z = A(I,J)*B",
		"END")

	if n := temporaries(r); n != 1 {
		t.Errorf("%d temporaries in:\n%s", n, codeText(r))
	}

	if n := strings.Count(codeText(r), "mul.F"); n != 1 {
		t.Errorf("%d multiplications in:\n%s", n, codeText(r))
	}
}

// TestCommutes checks that A*B and B*A are one value, and A-B and B-A aren't.
func TestCommutes(t *testing.T) {
	merged := improved(t, "S",
		"SUBROUTINE S(X, Y, Z)",
		"REAL A, B, C, X, Y, Z",
		"COMMON A, B, C",
		"X = A*B + C",
		"Y = B*A - C",
		"Z = A*B*C",
		"END")

	if n := strings.Count(codeText(merged), "mul.F(A, B)") + strings.Count(codeText(merged), "mul.F(B, A)"); n != 1 {
		t.Errorf("%d multiplications of A and B in:\n%s", n, codeText(merged))
	}

	apart := improved(t, "S",
		"SUBROUTINE S(X, Y, Z)",
		"REAL A, B, C, X, Y, Z",
		"COMMON A, B, C",
		"X = (A-B) + C",
		"Y = (B-A) + C",
		"Z = (A-B)*(B-A)",
		"END")

	if !strings.Contains(codeText(apart), "sub.F(B, A)") || !strings.Contains(codeText(apart), "sub.F(A, B)") {
		t.Errorf("A-B and B-A merged:\n%s", codeText(apart))
	}
}

// threeUses is a value used three times, wide enough to be worth a temporary, with
// a statement put between its first use and its second.
func threeUses(between ...string) []string {
	lines := []string{
		"SUBROUTINE S(X, Y, Z, W)",
		"REAL A(5,5), B, X, Y, Z, W",
		"INTEGER I, J",
		"COMMON A, I, J",
		"X = A(I,J)*B + 1.0",
	}

	lines = append(lines, between...)

	return append(lines, "Y = A(I,J)*B - 1.0", "Z = A(I,J)*B + 2.0", "W = A(I,J)*B + 3.0", "END")
}

// TestWhatEndsAValue checks each thing that means the same expression isn't the same
// value any more: the first use isn't merged with the rest, and they still are with
// each other.
func TestWhatEndsAValue(t *testing.T) {
	for _, between := range []string{
		"I = I + 1",
		"A(1,1) = 0.0",
		"B = 2.0",
		"CALL OTHER",
	} {
		r := improved(t, "S", threeUses(between)...)

		if n := strings.Count(codeText(r), "mul.F"); n != 2 {
			t.Errorf("after %s, %d multiplications in:\n%s", between, n, codeText(r))
		}
	}
}

// TestLocalSurvivesCall checks that a call leaves alone a value that reads only locals
// nothing has been given the address of, and ends one if something has.
func TestLocalSurvivesCall(t *testing.T) {
	r := improved(t, "S",
		"SUBROUTINE S(X, Y, Z)",
		"REAL A, B, X, Y, Z",
		"X = A*B + 1.0",
		"CALL OTHER",
		"Y = A*B - 1.0",
		"Z = A*B + 2.0",
		"END")

	if n := strings.Count(codeText(r), "mul.F"); n != 1 {
		t.Errorf("%d multiplications in:\n%s", n, codeText(r))
	}

	r = improved(t, "S",
		"SUBROUTINE S(X, Y, Z)",
		"REAL A, B, X, Y, Z",
		"X = A*B + 1.0",
		"CALL OTHER(A)",
		"Y = A*B - 1.0",
		"Z = A*B + 2.0",
		"END")

	if n := strings.Count(codeText(r), "mul.F"); n < 2 {
		t.Errorf("merged across the call that may change A:\n%s", codeText(r))
	}
}

// TestEquivalenceEndsAValue checks that a store through another name ends a value.
func TestEquivalenceEndsAValue(t *testing.T) {
	r := improved(t, "S",
		"SUBROUTINE S(X, Y, Z)",
		"REAL A, B, X, Y, Z",
		"INTEGER N",
		"EQUIVALENCE (A, N)",
		"X = A*B + 1.0",
		"N = 0",
		"Y = A*B - 1.0",
		"Z = A*B + 2.0",
		"END")

	if !strings.Contains(codeText(r), "assign N = 0") {
		t.Fatalf("the store has gone:\n%s", codeText(r))
	}

	if n := strings.Count(codeText(r), "mul.F"); n < 2 {
		t.Errorf("merged across the store to N:\n%s", codeText(r))
	}
}

// TestVolatileNotMerged checks that what reads something VOLATILE is computed each
// time.
func TestVolatileNotMerged(t *testing.T) {
	r := improved(t, "S",
		"SUBROUTINE S(X, Y, Z)",
		"REAL A, B, X, Y, Z",
		"VOLATILE A",
		"X = A*B + 1.0",
		"Y = A*B - 1.0",
		"Z = A*B + 2.0",
		"END")

	if n := strings.Count(codeText(r), "mul.F"); n != 3 {
		t.Errorf("%d multiplications in:\n%s", n, codeText(r))
	}
}

// TestConditionalUse checks that an operand a condition may not reach doesn't get its
// value computed ahead of the condition, and that it can use one already computed.
func TestConditionalUse(t *testing.T) {
	r := improved(t, "S",
		"SUBROUTINE S(M, N, X)",
		"INTEGER M, N, K",
		"LOGICAL X",
		"COMMON K",
		"IF (N.NE.0 .AND. (M/N.GT.2 .OR. M/N.LT.-2 .OR. M/N.EQ.0)) K = 1",
		"END")

	if n := temporaries(r); n != 0 {
		t.Errorf("%d temporaries, before the test of N:\n%s", n, codeText(r))
	}

	r = improved(t, "S",
		"SUBROUTINE S(M, N, X)",
		"INTEGER M, N, K, L, I",
		"LOGICAL X",
		"COMMON K, L, I",
		"L = (M/N + I) * 2",
		"IF (N .NE. 0 .AND. M/N + I .GT. 2) K = 1",
		"END")

	if n := strings.Count(codeText(r), "div.L"); n != 1 {
		t.Errorf("%d divisions in:\n%s", n, codeText(r))
	}
}

// TestHolder checks that a variable holding the value is read rather than a temporary
// made, as long as it does hold it.
func TestHolder(t *testing.T) {
	r := improved(t, "S",
		"SUBROUTINE S(I, J)",
		"REAL A(100)",
		"INTEGER I, J, K",
		"COMMON A, K",
		"K = J*5 + I",
		"A(J*5 + I) = 0.0",
		"END")

	if temporaries(r) != 0 || strings.Count(codeText(r), "mul.L") != 1 {
		t.Errorf("not read from K:\n%s", codeText(r))
	}

	if !strings.Contains(codeText(r), "assign A-4[K] = 0.0") {
		t.Errorf("not read from K:\n%s", codeText(r))
	}

	// K changes, so it holds something else by then.
	r = improved(t, "S",
		"SUBROUTINE S(I, J)",
		"REAL A(100)",
		"INTEGER I, J, K",
		"COMMON A, K",
		"K = J*5 + I",
		"K = K + 1",
		"A(J*5 + I) = 0.0",
		"END")

	if strings.Contains(codeText(r), "A-4[K]") {
		t.Errorf("read from a K that changed:\n%s", codeText(r))
	}

	// K = K + I isn't a value in K: it reads the K it changes.
	r = improved(t, "S",
		"SUBROUTINE S(I)",
		"REAL A(100)",
		"INTEGER I, K",
		"COMMON A, K",
		"K = K + I",
		"A(K + I) = 0.0",
		"END")

	if !strings.Contains(codeText(r), "add.L") || strings.Count(codeText(r), "add.L") != 2 {
		t.Errorf("K + I taken from K:\n%s", codeText(r))
	}
}

// TestNotWorthATemporary checks the example in cse.go's comment: a subscript two
// arrays use, from a temporary in memory, takes as many instructions and more memory
// references. The same element twice is worth one, having its own read to save.
func TestNotWorthATemporary(t *testing.T) {
	r := improved(t, "S",
		"SUBROUTINE S(X, Y)",
		"REAL A(5,5), B(5,5), X, Y",
		"INTEGER I, J",
		"COMMON A, B, I, J",
		"X = A(I,J) + 1.0",
		"Y = B(I,J) - 1.0",
		"END")

	if n := temporaries(r); n != 0 {
		t.Errorf("%d temporaries in:\n%s", n, codeText(r))
	}

	r = improved(t, "S",
		"SUBROUTINE S(X, Y)",
		"REAL A(5,5), X, Y",
		"INTEGER I, J",
		"COMMON A, I, J",
		"X = A(I,J) + 1.0",
		"Y = A(I,J) - 1.0",
		"END")

	if n := temporaries(r); n != 1 {
		t.Errorf("%d temporaries in:\n%s", n, codeText(r))
	}
}

// TestFormatExpressionsAlone checks that a variable format expression's blocks get no
// temporaries, which they would share with the statement running them.
func TestFormatExpressionsAlone(t *testing.T) {
	r := improved(t, "S",
		"SUBROUTINE S(A, I, J)",
		"REAL A(5,5)",
		"WRITE (6,10) A(I,J)",
		"10: FORMAT (F<A(I,J)*A(I,J)+A(I,J)*A(I,J)+A(I,J)*A(I,J)>.2)",
		"END")

	if n := temporaries(r); n != 0 {
		t.Errorf("%d temporaries in:\n%s", n, codeText(r))
	}
}

// TestSpareTemporaries checks that a block uses again what the block before it used.
func TestSpareTemporaries(t *testing.T) {
	r := improved(t, "S",
		"SUBROUTINE S(X, Y, Z, L)",
		"REAL A, B, X, Y, Z",
		"LOGICAL L",
		"COMMON A, B",
		"X = A*B + 1.0",
		"Y = A*B - 1.0",
		"IF (L) THEN",
		"   X = A/B + 1.0",
		"   Y = A/B - 1.0",
		"END IF",
		"END")

	used := map[string]bool{}

	for _, line := range strings.Split(codeText(r), "\n") {
		if name, ok := strings.CutPrefix(line, "assign $U"); ok {
			used[strings.Fields(name)[0]] = true
		}
	}

	if len(used) != 1 {
		t.Errorf("temporaries %v in:\n%s", used, codeText(r))
	}
}
