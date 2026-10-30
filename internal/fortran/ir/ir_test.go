package ir

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/fortran/diag"
	"github.com/tucats/govax/internal/fortran/lex"
	"github.com/tucats/govax/internal/fortran/parse"
	"github.com/tucats/govax/internal/fortran/sema"
	"github.com/tucats/govax/internal/fortran/source"
	"github.com/tucats/govax/internal/vaxfloat"
)

// update rewrites the golden files from what lowering produces: go test
// ./internal/fortran/ir -update. Review the diff before committing it.
var update = flag.Bool("update", false, "rewrite the golden .ir files")

// lowerLines runs every pass up to lowering over lines as the
// fixed-format file spec, a main program without PROGRAM being named
// T$MAIN. The program must have no messages, and each routine must pass
// Check.
func lowerLines(t *testing.T, spec string, lines ...string) []*Routine {
	t.Helper()

	var messages diag.List

	program := source.Read(source.Form{}, &source.File{Spec: spec, Lines: lines}, nil, &messages)
	file := parse.Parse(lex.Lex(program.Statements, &messages), &messages)
	prog := sema.Analyze(file, "T$MAIN", &messages)

	for _, m := range messages.Messages() {
		t.Errorf("%s: %s", spec, m)
	}

	if messages.Failed() {
		t.FailNow()
	}

	routines := Lower(prog)

	for _, r := range routines {
		if err := Check(r); err != nil {
			t.Errorf("%s: %v", spec, err)
		}
	}

	return routines
}

// TestGolden lowers each program in testdata and compares its dump with
// the .ir file beside it.
func TestGolden(t *testing.T) {
	sources, err := filepath.Glob(filepath.Join("testdata", "*.for"))
	if err != nil || len(sources) == 0 {
		t.Fatalf("no programs in testdata: %v", err)
	}

	for _, src := range sources {
		data, err := os.ReadFile(src)
		if err != nil {
			t.Fatal(err)
		}

		lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
		got := Dump(lowerLines(t, strings.ToUpper(filepath.Base(src)), lines...))
		golden := strings.TrimSuffix(src, ".for") + ".ir"

		if *update {
			if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
				t.Fatal(err)
			}

			continue
		}

		want, err := os.ReadFile(golden)
		if err != nil {
			t.Fatalf("%v (run with -update to write it)", err)
		}

		if got != string(want) {
			t.Errorf("%s:\n%s\nwant:\n%s", src, got, want)
		}
	}
}

// stmtLines lowers a main program of the given statements, after the
// declarations decls (separated by semicolons; a statement starting
// with a number is labeled with it), and returns its first block's statements and
// terminator as the dump writes them, one a line, without the line
// marks.
func stmtLines(t *testing.T, decls string, stmts ...string) string {
	t.Helper()

	var lines []string

	for _, d := range strings.Split(decls, ";") {
		lines = append(lines, "      "+d)
	}

	for _, s := range stmts {
		// A statement starting with a number has it as its label.
		label, text, _ := strings.Cut(s, " ")
		if strings.Trim(label, "0123456789") != "" {
			label, text = "", s
		}

		lines = append(lines, fmt.Sprintf("%5s %s", label, text))
	}

	lines = append(lines, "      END")

	r := lowerLines(t, "T.FOR", lines...)[0]

	var out []string

	for _, b := range r.Blocks {
		for _, s := range b.Stmts {
			out = append(out, StmtText(s))
		}

		out = append(out, TerminatorText(b.End))
	}

	return strings.Join(out, "\n")
}

// TestExpressions checks how assignments are lowered: conversions,
// folding, array elements' offsets and indexes.
func TestExpressions(t *testing.T) {
	const decls = "INTEGER I, J, A(10), B(0:4, 2:3)"

	tests := []struct{ stmt, want string }{
		{"I = J + 1", "assign I = add.L(J, 1)"},
		{"I = J - 1 + 1", "assign I = J"},
		{"I = 5 - J", "assign I = sub.L(5, J)"},
		{"I = -J", "assign I = neg.L(J)"},
		{"I = J * 1", "assign I = J"},
		{"I = J / 2", "assign I = div.L(J, 2)"},
		{"X = I", "assign X = cvt.LF(I)"},
		{"I = X", "assign I = cvt.FL(X)"},
		{"X = X * 2", "assign X = mul.F(X, 2.0)"},
		{"X = -X", "assign X = neg.F(X)"},
		{"I = A(J)", "assign I = A-4[J]"},
		{"I = A(J+1)", "assign I = A[J]"},
		{"I = A(3)", "assign I = A+8"},
		{"I = B(J, I)", "assign I = B-40[add.L(J, mul.L(I, 5))]"},
		{"I = B(4, 3)", "assign I = B+36"},
		{"I = B(0, 2)", "assign I = B"},
		{"A(J-1) = J", "assign A-8[J] = J"},
		{"L = I .LT. J", "assign L = lt.L(I, J)"},
		{"L = X .GE. I", "assign L = ge.F(X, cvt.LF(I))"},
		{"L = .NOT. L .AND. I .EQ. J", "assign L = and.L(not.L(L), eq.L(I, J))"},
		{"I = I .OR. 6", "assign I = or.L(I, 6)"},
		{"L = .TRUE.", "assign L = -1"},
		{"X = 1.0 / 3", "assign X = 0.33333334"},
	}

	for _, tt := range tests {
		got := stmtLines(t, decls+";LOGICAL L", tt.stmt)
		want := tt.want + "\nreturn 1"

		if got != want {
			t.Errorf("%s:\n got %q\nwant %q", tt.stmt, got, want)
		}
	}
}

// TestFolding checks arith's folding rules directly.
func TestFolding(t *testing.T) {
	v := &Load{&Mem{Cell: &Cell{Name: "V", T: Long}, T: Long}}
	half := FloatConst(vaxfloat.FromFloat64(0.5))
	big := FloatConst(vaxfloat.FromFloat64(3e9))

	tests := []struct {
		e    Expr
		want string
	}{
		{arith(Add, Long, LongConst(2147483647), LongConst(1)), "-2147483648"},
		{arith(Div, Long, LongConst(7), LongConst(0)), "div.L(7, 0)"},
		{arith(Div, Long, LongConst(-7), LongConst(2)), "-3"},
		{arith(Sub, Long, arith(Add, Long, v, LongConst(3)), LongConst(5)), "sub.L(V, 2)"},
		{arith(Add, Long, LongConst(3), arith(Sub, Long, v, LongConst(3))), "V"},
		{arith(Sub, Long, LongConst(0), v), "neg.L(V)"},
		{arith(Add, Long, nil, v), "V"},
		{arith(Mul, Long, LongConst(1), v), "V"},
		{arith(Not, Long, LongConst(0), nil), "-1"},
		{arith(Cvt, Long, FloatConst(vaxfloat.FromFloat64(-2.75)), nil), "-2"},
		{arith(Cvt, Long, big, nil), "cvt.FL(3E+09)"},
		{arith(Cvt, Float, LongConst(16777217), nil), "1.6777218E+07"}, // a tie, rounded away from zero
		{arith(Div, Float, half, FloatConst(vaxfloat.FromFloat64(0))), "div.F(0.5, 0.0)"},
		{arith(Add, Float, half, half), "1.0"},
	}

	for _, tt := range tests {
		if got := ExprText(tt.e); got != tt.want {
			t.Errorf("got %s, want %s", got, tt.want)
		}
	}
}

// TestTripCounts checks DO loops' iteration counts.
func TestTripCounts(t *testing.T) {
	tests := []struct{ do, count string }{
		{"DO 10 I = 1, 5", "assign $C1 = 5"},
		{"DO 10 I = 1, N", "assign $C1 = N"},
		{"DO 10 I = N, 1, -1", "assign $C1 = N"},
		{"DO 10 I = 0, N", "assign $C1 = add.L(N, 1)"},
		{"DO 10 I = M, N", "assign $C1 = add.L(sub.L(N, M), 1)"},
		{"DO 10 I = 1, N, 2", "assign $C1 = div.L(add.L(N, 1), 2)"},
		{"DO 10 I = 1, 50, 2", "assign $C1 = 25"},
		{"DO 10 I = 5, 1", "assign $C1 = -3"},
		{"DO 10 X = 0.5, 2.5, 0.5", "assign $C1 = 5"},
		{"DO 10 X = 1.0, Y", "assign $C1 = cvt.FL(div.F(add.F(sub.F(Y, 1.0), 1.0), 1.0))"},
	}

	for _, tt := range tests {
		got := stmtLines(t, "INTEGER M, N", tt.do, "10 CONTINUE")
		if first, _, _ := strings.Cut(got, "\n"); first != tt.count {
			t.Errorf("%s: got %q, want %q", tt.do, first, tt.count)
		}
	}
}

// TestLabelsTakeBlocks checks that a label starts a block, sharing one
// just begun, and that a jump forward makes the block it goes to.
func TestLabelsTakeBlocks(t *testing.T) {
	r := lowerLines(t, "T.FOR",
		"      I = 1",
		"      GO TO 20",
		"   10 I = 2",
		"   20 I = 3",
		"      IF (I .GT. 0) GO TO 10",
		"      END")[0]

	var heads []string

	for _, b := range r.Blocks {
		heads = append(heads, b.Name+":"+TerminatorText(b.End))
	}

	got := strings.Join(heads, " ")
	want := "B1:jump B3 B2:jump B3 B3:branch gt.L(I, 0) B2 B4 B4:return 1"

	if got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}

	if r.Blocks[1].Label != 10 || r.Blocks[2].Label != 20 {
		t.Errorf("labels %d and %d, want 10 and 20", r.Blocks[1].Label, r.Blocks[2].Label)
	}
}

// TestTemporaries checks that temporaries are shared between statements
// and not within one.
func TestTemporaries(t *testing.T) {
	r := lowerLines(t, "T.FOR",
		"      INTEGER F",
		"      I = F(1) + F(2)",
		"      J = F(3) + F(4) + F(5)",
		"      X = F(6) + 1.0",
		"      END")[0]

	var temps []string

	for _, c := range r.Cells {
		if c.Kind == TempCell {
			temps = append(temps, c.Name+" "+c.T.String())
		}
	}

	if got, want := strings.Join(temps, ", "), "$T1 L, $T2 L, $T3 L"; got != want {
		t.Errorf("temporaries %s, want %s", got, want)
	}
}

// TestCheck checks that Check finds a block without a terminator and a
// jump to a block not laid out.
func TestCheck(t *testing.T) {
	r := &Routine{Name: "R", Blocks: []*Block{{Name: "B1"}}}
	if err := Check(r); err == nil || !strings.Contains(err.Error(), "no terminator") {
		t.Errorf("no terminator: %v", err)
	}

	r.Blocks[0].End = &Jump{To: &Block{Name: "B9"}}
	if err := Check(r); err == nil || !strings.Contains(err.Error(), "not laid out") {
		t.Errorf("stray jump: %v", err)
	}
}
