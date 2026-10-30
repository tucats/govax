package codegen

import (
	"errors"
	"fmt"
	"flag"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/asm"
	"github.com/tucats/govax/internal/fortran/diag"
	"github.com/tucats/govax/internal/fortran/ir"
	"github.com/tucats/govax/internal/fortran/lex"
	"github.com/tucats/govax/internal/fortran/parse"
	"github.com/tucats/govax/internal/fortran/sema"
	"github.com/tucats/govax/internal/fortran/source"
)

// update rewrites the golden files from what the code generator writes:
// go test ./internal/fortran/codegen -update. Review the diff (and
// assemble what changed) before committing it.
var update = flag.Bool("update", false, "rewrite the golden .mar files")

// generate runs every pass over lines as the fixed-format file spec (a
// main program without PROGRAM named T$MAIN) and returns the MACRO lines
// Generate writes after a .TITLE, so that the result assembles. The
// program must have no messages.
func generate(t *testing.T, spec string, lines ...string) []string {
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

	var w Writer

	w.Statement("", ".TITLE", "TEST", "")
	Generate(&w, ir.Lower(prog), program.Lines)

	return w.Lines()
}

// assemble assembles a MACRO file with govax's MACRO, which must report
// nothing: no error, no warning, no message.
func assemble(t *testing.T, name string, lines []string) {
	t.Helper()

	a := asm.New(false)
	a.SetDialect(asm.DialectMACRO)

	if _, err := a.Assemble(strings.Join(lines, "\n") + "\n"); err != nil {
		var list *asm.Errors
		if errors.As(err, &list) {
			err = errors.Join(list.List...)
		}

		t.Errorf("%s doesn't assemble: %v", name, err)

		return
	}

	if w := a.Warnings(); len(w) > 0 {
		t.Errorf("%s: MACRO warns: %v", name, errors.Join(w...))
	}

	if m := a.Messages(); len(m) > 0 {
		t.Errorf("%s: MACRO says: %s", name, strings.Join(m, "\n"))
	}

	if _, err := a.Object(asm.ObjectOptions{}); err != nil {
		t.Errorf("%s: no object module: %v", name, err)
	}
}

// TestGolden generates each program in testdata, compares the MACRO
// with the .mar file beside it, and assembles it.
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
		out := generate(t, strings.ToUpper(filepath.Base(src)), lines...)
		assemble(t, src, out)

		got := strings.Join(out, "\n") + "\n"
		golden := strings.TrimSuffix(src, ".for") + ".mar"

		if *update {
			if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
				t.Fatal(err)
			}

			continue
		}

		want, err := os.ReadFile(golden)
		if err != nil {
			t.Fatalf("%v (run with -update to make it)", err)
		}

		if got != string(want) {
			t.Errorf("%s: the MACRO differs from %s:\n%s", src, golden, diffLines(string(want), got))
		}
	}
}

// diffLines shows the first lines where two texts differ.
func diffLines(want, got string) string {
	w, g := strings.Split(want, "\n"), strings.Split(got, "\n")

	for i := 0; i < max(len(w), len(g)); i++ {
		var a, b string
		if i < len(w) {
			a = w[i]
		}

		if i < len(g) {
			b = g[i]
		}

		if a != b {
			return "line " + strconv.Itoa(i+1) + ":\nwant: " + a + "\ngot:  " + b
		}
	}

	return "(no difference)"
}

// codeOf generates a main program of the statements and returns its
// instructions: the lines after .ENTRY, without comments, source lines,
// or the return at the end, each as "OPCODE operands" (a label as
// "label:"). The program must assemble.
func codeOf(t *testing.T, stmts ...string) []string {
	t.Helper()

	lines := []string{"      PROGRAM T"}
	for _, s := range stmts {
		label := ""
		if s[0] >= '0' && s[0] <= '9' {
			label, s, _ = strings.Cut(s, " ")
		}

		lines = append(lines, fmt.Sprintf("%-5s %s", label, s))
	}

	out := generate(t, "T.FOR", append(lines, "      END")...)
	assemble(t, strings.Join(stmts, "; "), out)

	var code []string

	inCode := false

	for _, line := range out {
		if strings.HasPrefix(line, "\t.ENTRY") {
			inCode = true

			continue
		}

		if !inCode || line == "" || strings.HasPrefix(line, ";") {
			continue
		}

		if i := strings.Index(line, ";"); i >= 0 {
			line = line[:i]
		}

		fields := strings.Fields(line)
		code = append(code, strings.Join(fields, " "))
	}

	// The main program's return: MOVL #1,R0 and RET, then .END.
	if n := len(code); n >= 3 && code[n-3] == "MOVL #1,R0" && code[n-2] == "RET" {
		code = code[:n-3]
	}

	return code
}

// TestSelection checks the instructions chosen for statements of the
// subset.
func TestSelection(t *testing.T) {
	cases := []struct {
		decl, stmt string
		want       []string
	}{
		// Moves, constants, and the two-operand and INC/DEC forms.
		{"", "I = J", []string{"MOVL T.J,T.I"}},
		{"", "I = 0", []string{"CLRL T.I"}},
		{"", "X = 0", []string{"CLRF T.X"}},
		{"", "I = -70", []string{"MOVL #-70,T.I"}},
		{"", "X = 1.0", []string{"MOVL #^X00004080,T.X"}},
		{"", "I = I + 1", []string{"INCL T.I"}},
		{"", "I = 1 + I", []string{"INCL T.I"}},
		{"", "I = I - 1", []string{"DECL T.I"}},
		{"", "I = I + J", []string{"ADDL2 T.J,T.I"}},
		{"", "I = J + I", []string{"ADDL2 T.J,T.I"}},
		{"", "I = I - J", []string{"SUBL2 T.J,T.I"}},
		{"", "I = J - I", []string{"SUBL3 T.I,T.J,T.I"}},
		{"", "I = 5 - J", []string{"SUBL3 T.J,#5,T.I"}},
		{"", "I = I / J", []string{"DIVL2 T.J,T.I"}},
		{"", "I = J / K", []string{"DIVL3 T.K,T.J,T.I"}},
		{"", "X = X * Y", []string{"MULF2 T.Y,T.X"}},
		{"", "X = Y + 2.5", []string{"ADDF3 T.Y,T.$F1,T.X"}},
		{"", "I = -J", []string{"MNEGL T.J,T.I"}},
		// A tree: interior values in registers, the last operation
		// straight into the target.
		{"", "I = J*K + L", []string{"MULL3 T.J,T.K,R0", "ADDL3 R0,T.L,T.I"}},
		{"", "I = (J+K) * (L+M)", []string{"ADDL3 T.J,T.K,R0", "ADDL3 T.L,T.M,R1", "MULL3 R0,R1,T.I"}},
		// The harder operand first (Sethi and Ullman).
		{"", "I = J - (K+L)*(M+N)", []string{"ADDL3 T.K,T.L,R0", "ADDL3 T.M,T.N,R1", "MULL2 R1,R0", "SUBL3 R0,T.J,T.I"}},
		// Conversions.
		{"", "X = I", []string{"CVTLF T.I,T.X"}},
		{"", "I = X", []string{"CVTFL T.X,T.I"}},
		{"", "X = I * Y", []string{"CVTLF T.I,R0", "MULF3 R0,T.Y,T.X"}},
		// LOGICAL operations: .AND. is BIC with the complement.
		{"LOGICAL A, B, C", "A = B .AND. C", []string{"MCOML T.C,R0", "BICL3 R0,T.B,T.A"}},
		{"LOGICAL A, B, C", "A = B .AND. .NOT. C", []string{"BICL3 T.C,T.B,T.A"}},
		{"", "I = J .AND. 6", []string{"BICL3 #-7,T.J,T.I"}},
		{"LOGICAL A, B, C", "A = B .OR. C", []string{"BISL3 T.B,T.C,T.A"}},
		{"LOGICAL A, B", "A = .NOT. B", []string{"MCOML T.B,T.A"}},
		{"LOGICAL A", "A = .TRUE.", []string{"MOVL #-1,T.A"}},
		{"LOGICAL A", "A = I .LT. J", []string{"CLRL R0", "CMPL T.I,T.J", "BGEQ T.$1", "MNEGL #1,R0", "T.$1:", "MOVL R0,T.A"}},
		// Array elements: indexed mode, the constant part an offset.
		{"INTEGER V(10)", "V(I) = 0", []string{"MOVL T.I,R0", "CLRL T.V-4[R0]"}},
		{"INTEGER V(10)", "V(I+1) = V(3)", []string{"MOVL T.I,R0", "MOVL T.V+8,T.V[R0]"}},
		{"INTEGER V(0:9)", "I = V(I-1) + 1", []string{"MOVL T.I,R0", "ADDL3 T.V-4[R0],#1,T.I"}},
		{"INTEGER M(2,3)", "M(I,J) = 1", []string{"MULL3 T.J,#2,R0", "ADDL2 T.I,R0", "MOVL #1,T.M-12[R0]"}},
		// Branches: the opposite condition around a BRW; a test of
		// what was just written is left out.
		{"", "IF (I .GT. J) I = 0", []string{"CMPL T.I,T.J", "BGTR T.$2", "BRW T.$1", "T.$2:", "CLRL T.I", "T.$1:"}},
		{"", "IF (I .EQ. 0) GO TO 10", []string{"TSTL T.I", "BNEQ T.$1", "BRW T.L10", "T.$1:", "T.L10:"}},
		{"", "IF (0 .LT. I) GO TO 10", []string{"TSTL T.I", "BLEQ T.$1", "BRW T.L10", "T.$1:", "T.L10:"}},
		{"LOGICAL A", "IF (A) I = 0", []string{"BLBS T.A,T.$2", "BRW T.$1", "T.$2:", "CLRL T.I", "T.$1:"}},
		{"", "IF (X - Y .EQ. 0.0) I = 0", []string{"SUBF3 T.Y,T.X,R0", "BEQL T.$2", "BRW T.$1", "T.$2:", "CLRL T.I", "T.$1:"}},
		// Short-circuit: either operand false makes .AND. false; a true
		// one makes .OR. true, past the test of the second.
		{"", "IF (I .GT. 0 .AND. J .GT. 0) I = 0", []string{
			"TSTL T.I", "BGTR T.$2", "BRW T.$1", "T.$2:",
			"TSTL T.J", "BGTR T.$3", "BRW T.$1", "T.$3:", "CLRL T.I", "T.$1:"}},
		{"", "IF (I .GT. 0 .OR. J .GT. 0) I = 0", []string{
			"TSTL T.I", "BLEQ T.$3", "BRW T.$2", "T.$3:",
			"TSTL T.J", "BGTR T.$4", "BRW T.$1", "T.$4:", "T.$2:", "CLRL T.I", "T.$1:"}},
		// Calls: arguments pushed last to first, a function's value
		// from R0.
		{"", "CALL S(I, 3, J+1, , (K))", []string{
			"ADDL3 T.J,#1,T.$T1", "MOVL T.K,T.$T2",
			"PUSHAL T.$T2", "PUSHL #0", "PUSHAL T.$T1", "PUSHAL T.$K1", "PUSHAL T.I", "CALLS #5,G^S"}},
		{"INTEGER V(10)", "V(I) = KF(J)", []string{"PUSHAL T.J", "CALLS #1,G^KF", "MOVL T.I,R1", "MOVL R0,T.V-4[R1]"}},
	}

	for _, tc := range cases {
		var stmts []string
		if tc.decl != "" {
			stmts = append(stmts, tc.decl)
		}

		stmts = append(stmts, tc.stmt)
		if strings.Contains(tc.stmt, "GO TO 10") {
			stmts = append(stmts, "10 CONTINUE")
		}

		got := codeOf(t, stmts...)
		if strings.Join(got, "\n") != strings.Join(tc.want, "\n") {
			t.Errorf("%s:\n%s\nwant:\n%s", tc.stmt, strings.Join(got, "\n"), strings.Join(tc.want, "\n"))
		}
	}
}

// TestSpill checks that an expression needing more registers than the
// budget has a part computed into a spill cell first, and that the
// routine saves the registers from R2 up it uses.
func TestSpill(t *testing.T) {
	out := generate(t, "T.FOR",
		"      PROGRAM T",
		"      K = ((I+1)*(J+2) + (I+3)*(J+4)) * ((I+5)*(J+6) + (I+7)*(J+8))",
		"     1  - ((I-1)*(J-2) + (I-3)*(J-4)) * ((I-5)*(J-6) + (I-7)*(J-8))",
		"      END")
	assemble(t, "spill", out)

	text := strings.Join(out, "\n")

	for _, want := range []string{
		"T.$X1:\t.BLKL\t1\t\t\t; a part of an expression, set aside",
		"\t.ENTRY\tT,^M<R2,R3>",
		",T.$X1\n",
		"\tSUBL3\tR0,T.$X1,T.K",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("no %q in:\n%s", want, text)
		}
	}
}

// TestLongNames checks the numbered form of a symbol past MACRO's 31
// characters, and the storage map's list of them.
func TestLongNames(t *testing.T) {
	out := generate(t, "T.FOR",
		"      SUBROUTINE A_SUBROUTINE_WITH_A_LONG_NAME(ARGUMENT_TWO)",
		"      INTEGER ARGUMENT_TWO, A_VARIABLE",
		"      A_VARIABLE = ARGUMENT_TWO",
		"      END")
	assemble(t, "long names", out)

	text := strings.Join(out, "\n")

	for _, want := range []string{
		"A_SUBROUTINE_WITH_A_LONG_NA.$N1 = 4\t; ARGUMENT_TWO is argument 1",
		";   A_SUBROUTINE_WITH_A_LONG_NA.$N2 is A_SUBROUTINE_WITH_A_LONG_NAME.A_VARIABLE",
		"\tMOVL\t@A_SUBROUTINE_WITH_A_LONG_NA.$N1(AP),A_SUBROUTINE_WITH_A_LONG_NA.$N2",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("no %q in:\n%s", want, text)
		}
	}

	for _, line := range out {
		if len(line) > MaxLine {
			t.Errorf("a line of %d characters: %q", len(line), line)
		}
	}
}

// TestTexts checks character constants' .ASCII operands: a delimiter
// the text doesn't hold, a character that doesn't print as its code,
// long texts on several lines; and that MACRO assembles each into the
// text.
func TestTexts(t *testing.T) {
	cases := map[string][]string{
		"DONE":       {"/DONE/"},
		"A/B":        {`"A/B"`},
		"IT'S; OK":   {"/IT'S; OK/"},
		"TAB\tHERE":  {"/TAB/<9>/HERE/"},
		"/\"|!#%&*+": {"?/\"|!#%&*+?"},
		strings.Repeat("X", 100): {"/" + strings.Repeat("X", 64) + "/", "/" + strings.Repeat("X", 36) + "/"},
	}

	for text, want := range cases {
		if got := asciiPieces(text); strings.Join(got, " ") != strings.Join(want, " ") {
			t.Errorf("asciiPieces(%q) = %q, want %q", text, got, want)
		}
	}

	out := generate(t, "T.FOR", "      PRINT *, 'A/B', 'IT''S; OK'", "      STOP 'TAB\tHERE'", "      END")
	assemble(t, "texts", out)

	text := strings.Join(out, "\n")
	for _, want := range []string{"\t.ASCII\t\"A/B\"", "\t.ASCII\t/IT'S; OK/", "\t.ASCII\t/TAB/<9>/HERE/"} {
		if !strings.Contains(text, want) {
			t.Errorf("no %q in:\n%s", want, text)
		}
	}
}
