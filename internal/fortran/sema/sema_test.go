package sema

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
	"github.com/tucats/govax/internal/fortran/source"
)

// update rewrites the golden files from what semantic analysis
// produces: go test ./internal/fortran/sema -update. Review the diff
// before committing it.
var update = flag.Bool("update", false, "rewrite the golden .sema files")

// analyzeLines runs the first four passes over lines as the fixed-format
// file spec, a main program without PROGRAM being named T$MAIN.
func analyzeLines(t *testing.T, spec string, lines ...string) (*Program, []diag.Message) {
	t.Helper()

	var messages diag.List

	program := source.Read(source.Form{}, &source.File{Spec: spec, Lines: lines}, nil, &messages)
	file := parse.Parse(lex.Lex(program.Statements, &messages), &messages)
	prog := Analyze(file, "T$MAIN", &messages)

	return prog, messages.Messages()
}

// TestGolden analyzes each program in testdata and compares its dump
// and messages with the .sema file beside it.
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
		prog, ms := analyzeLines(t, strings.ToUpper(filepath.Base(src)), lines...)

		var b strings.Builder

		b.WriteString(Dump(prog))

		for _, m := range ms {
			b.WriteString("\n" + m.String() + "\n")
		}

		golden := strings.TrimSuffix(src, ".for") + ".sema"

		if *update {
			if err := os.WriteFile(golden, []byte(b.String()), 0o644); err != nil {
				t.Fatal(err)
			}

			continue
		}

		want, err := os.ReadFile(golden)
		if err != nil {
			t.Fatalf("%v (run with -update to write it)", err)
		}

		if b.String() != string(want) {
			t.Errorf("%s:\n%s\nwant:\n%s", src, b.String(), want)
		}
	}
}

// TestErrors checks each of semantic analysis's messages: each case is a
// program (the statements of a main program, in fixed form, unless the
// case gives its own header) and the messages it gets, each as
// IDENT@line:column.
func TestErrors(t *testing.T) {
	cases := []struct {
		name  string
		lines []string
		want  string
	}{
		// Program units.
		{"two units, one name", []string{"SUBROUTINE S", "END", "SUBROUTINE S", "END"}, "DUPUNIT@3:7"},
		{"two main programs", []string{"PROGRAM A", "END", "PROGRAM B", "END"}, "TWOMAINS@3:7"},
		{"a dummy twice", []string{"SUBROUTINE S(A, B, A)", "END"}, "DUPDUMMY@1:26"},
		{"the routine's own name as a dummy", []string{"SUBROUTINE S(S)", "END"}, "BADUSE@1:20"},

		// Declarations.
		{"typed twice", []string{"INTEGER I", "REAL I", "END"}, "DUPDECL@2:12"},
		{"typed twice in one statement", []string{"INTEGER I, J, I", "END"}, "DUPDECL@1:21"},
		{"a typed function's type declared", []string{"REAL FUNCTION F(X)", "INTEGER F", "END"}, "DUPDECL@2:15"},
		{"dimensioned twice", []string{"DIMENSION A(5)", "INTEGER A(5)", "END"}, "DUPDIM@2:15"},
		{"declaration after an executable", []string{"I = 1", "INTEGER J", "END"}, "LATEDECL@2:7"},
		{"a length INTEGER doesn't have", []string{"INTEGER*3 I", "END"}, "BADLENGTH@1:17"},
		{"a length after the name", []string{"REAL X*5", "END"}, "BADLENGTH@1:12"},
		{"a length not compiled yet", []string{"INTEGER*2 I", "REAL X*8", "END"}, "NOTIMPL@1:17 NOTIMPL@2:12"},
		{"a bound naming a variable", []string{"DIMENSION A(N)", "END"}, "BADBOUND@1:19"},
		{"a REAL bound", []string{"DIMENSION A(2.5)", "END"}, "BADBOUND@1:19"},
		{"an adjustable array", []string{"SUBROUTINE S(A, N)", "DIMENSION A(N)", "END"}, "NOTIMPL@2:19"},
		{"bounds out of order", []string{"DIMENSION A(5:1)", "END"}, "BOUNDORDER@1:21"},
		{"assumed size, not a dummy", []string{"DIMENSION A(*)", "END"}, "ASSUMEDSIZE@1:17"},
		{"assumed size, not the last", []string{"SUBROUTINE S(A)", "DIMENSION A(*, 2)", "END"}, "ASSUMEDSIZE@2:17"},
		{"a constant bound folded", []string{"DIMENSION A(2*3-1:10/2)", "A(5) = 1", "END"}, ""},
		{"the program's name typed", []string{"PROGRAM P", "INTEGER P", "END"}, "BADUSE@2:15"},
		{"the result dimensioned", []string{"FUNCTION F(X)", "DIMENSION F(3)", "END"}, "BADUSE@2:17"},

		// Names.
		{"a variable called", []string{"X = 1", "CALL X", "END"}, "BADUSE@2:12"},
		{"an array called", []string{"DIMENSION A(3)", "CALL A(1)", "END"}, "BADUSE@2:12"},
		{"a subroutine as a variable", []string{"CALL S", "S = 1", "END"}, "BADUSE@2:7"},
		{"a subroutine as a function", []string{"CALL S", "X = S(1)", "END"}, "BADUSE@2:11"},
		{"a function called", []string{"X = F(1)", "CALL F(2)", "END"}, "BADUSE@2:12"},
		{"a function as a variable", []string{"X = F(1)", "Y = F", "END"}, "BADUSE@2:11"},
		{"the program's name as a variable", []string{"PROGRAM P", "P = 1", "END"}, "BADUSE@2:7"},
		{"a subroutine calling itself", []string{"SUBROUTINE S", "CALL S", "END"}, "RECURSIVE@2:12"},
		{"a function referring to itself", []string{"FUNCTION F(X)", "F = F(X - 1)", "END"}, "RECURSIVE@2:11"},
		{"a function calling itself", []string{"FUNCTION F(X)", "CALL F", "END"}, "RECURSIVE@2:12"},
		{"a variable with subscripts", []string{"X = 1", "Y = X(2)", "END"}, "NOTARRAY@2:11"},
		{"assignment to a non-array element", []string{"X = 1", "B(2) = 1", "END"}, "NOTARRAY@2:7"},
		{"a statement function", []string{"F(X) = X + 1", "Y = F(2.0)", "END"}, "NOTIMPL@1:7"},
		{"a whole array in an expression", []string{"DIMENSION A(3)", "X = A + 1", "END"}, "NEEDSUBSCRIPT@2:11"},
		{"a whole array assigned", []string{"DIMENSION A(3)", "A = 1", "END"}, "BADTARGET@2:7"},
		{"too many subscripts", []string{"DIMENSION A(3)", "X = A(1, 2)", "END"}, "SUBSCRIPTS@2:11"},
		{"too few subscripts", []string{"DIMENSION A(3, 3)", "X = A(1)", "END"}, "SUBSCRIPTS@2:11"},
		{"an intrinsic function", []string{"X = SQRT(2.0)", "END"}, "NOTIMPL@1:11"},
		{"an intrinsic as an array", []string{"DIMENSION SQRT(3)", "X = SQRT(2)", "END"}, ""},
		{"a system subroutine", []string{"CALL EXIT", "END"}, "NOTIMPL@1:12"},
		{"a system function", []string{"X = RAN(I)", "END"}, "NOTIMPL@1:11"},
		{"a dummy procedure called", []string{"SUBROUTINE S(P)", "CALL P", "END"}, "NOTIMPL@2:12"},
		{"a dummy function", []string{"SUBROUTINE S(P)", "X = P(1)", "END"}, "NOTIMPL@2:11"},

		// Expressions.
		{"**", []string{"X = 2.0 ** 3", "END"}, "NOTIMPL@1:15"},
		{"//", []string{"PRINT *, 'A' // 'B'", "END"}, "NOTIMPL@1:20"},
		{".EQV.", []string{"L = L .EQV. L", "END"}, "NOTIMPL@1:13"},
		{".AND. of REALs", []string{"L = X .AND. Y", "END"}, "BADOPERAND@1:13"},
		{".NOT. of a REAL", []string{"L = .NOT. X", "END"}, "BADOPERAND@1:11"},
		{"a character constant assigned", []string{"X = 'AB'", "END"}, "NOTIMPL@1:11"},
		{"a character constant added", []string{"X = 1 + 'AB'", "END"}, "NOTIMPL@1:15"},
		{"character constants compared", []string{"L = 'A' .EQ. 'B'", "END"}, "NOTIMPL@1:15"},
		{"a character constant passed", []string{"CALL S('AB')", "END"}, "NOTIMPL@1:14"},
		{"a REAL condition", []string{"IF (X) STOP", "END"}, "NOTLOGICAL@1:11"},
		{"an INTEGER too big", []string{"I = 2147483648", "END"}, "INTRANGE@1:11"},
		{"INTEGER's smallest", []string{"I = -2147483648", "END"}, ""},
		{"an overflow folded", []string{"I = 2147483647 + 1", "END"}, "INTRANGE@1:22"},
		{"a REAL too big", []string{"X = 1E39", "END"}, "REALRANGE@1:11"},
		{"a REAL too small", []string{"X = 1E-39", "END"}, "REALRANGE@1:11"},
		{"a REAL*8 constant", []string{"X = 1D0", "END"}, "NOTIMPL@1:11"},
		{"a REAL product too big", []string{"X = 1E38 * 10", "END"}, "REALRANGE@1:16"},
		{"a REAL quotient too small", []string{"X = 1E-38 / 1E10", "END"}, ""},
		{"divide by zero", []string{"I = 1 / 0", "END"}, "DIVZERO@1:13"},
		{"divide by zero, REAL", []string{"X = 1.0 / 0", "END"}, "DIVZERO@1:15"},
		{"a REAL too big for INTEGER", []string{"I = 3E9", "END"}, "INTRANGE@1:11"},

		// Statements.
		{"RETURN in a main program", []string{"RETURN", "END"}, "MAINRETURN@1:7"},
		{"an alternate return", []string{"SUBROUTINE S", "RETURN 1", "END"}, "NOTIMPL@2:14"},
		{"formatted PRINT", []string{"PRINT 10, X", "10 FORMAT (F8.2)", "END"}, "NOTIMPL@1:13 NOTIMPL@2:7"},
		{"a unit number", []string{"WRITE (6,*) X", "END"}, "NOTIMPL@1:14"},
		{"unformatted WRITE", []string{"WRITE (*) X", "END"}, "NOTIMPL@1:7"},
		{"a format expression", []string{"WRITE (*,F) X", "END"}, "NOTIMPL@1:16"},
		{"an assumed-size array written", []string{"SUBROUTINE S(A)", "DIMENSION A(*)", "PRINT *, A", "END"}, "NOSIZE@3:16"},

		// IF blocks.
		{"ELSE with no IF", []string{"ELSE", "END"}, "NOIF@1:7"},
		{"END IF with no IF", []string{"END IF", "END"}, "NOIF@1:7"},
		{"an IF with no END IF", []string{"IF (L) THEN", "X = 1", "END"}, "NOENDIF@1:7"},
		{"ELSE IF after ELSE", []string{"IF (L) THEN", "ELSE", "ELSE IF (L) THEN", "END IF", "END"}, "ELSEORDER@3:7"},
		{"two ELSEs", []string{"IF (L) THEN", "ELSE", "ELSE", "END IF", "END"}, "ELSEORDER@3:7"},
		{"a loop across ELSE", []string{"IF (L) THEN", "DO 10 I = 1, 2", "ELSE", "10 CONTINUE", "END IF", "END"}, "DONEST@3:7"},
		{"a block IF with an error", []string{"IF (X .GT.) THEN", "X = 1", "END IF", "END"}, "SYNTAX@1:17"},

		// DO loops.
		{"a loop whose label never comes", []string{"DO 10 I = 1, 2", "END"}, "NODO@1:7"},
		{"a loop whose label is behind it", []string{"10 CONTINUE", "DO 10 I = 1, 2", "END"}, "NODO@2:7"},
		{"a loop ending at GO TO", []string{"DO 10 I = 1, 2", "10 GO TO 20", "20 END"}, "DOTERM@2:7"},
		{"a loop ending at RETURN", []string{"SUBROUTINE S", "DO 10 I = 1, 2", "10 RETURN", "END"}, "DOTERM@3:7"},
		{"a loop ending at END IF", []string{"IF (L) THEN", "DO 10 I = 1, 2", "10 END IF", "END"}, "DOTERM@3:7"},
		{"a loop ending at END", []string{"DO 10 I = 1, 2", "10 END"}, "DOTERM@2:7"},
		{"a loop ending at a declaration", []string{"DO 10 I = 1, 2", "10 INTEGER J", "END"}, "LATEDECL@2:7 DOTERM@2:7"},
		{"a loop ending at a logical IF", []string{"DO 10 I = 1, 2", "10 IF (I .EQ. 1) X = 1", "END"}, ""},
		{"loops crossing", []string{"DO 10 I = 1, 2", "DO 20 J = 1, 2", "10 CONTINUE", "20 CONTINUE", "END"}, "DONEST@3:7"},
		{"a loop ending in an IF block", []string{"DO 10 I = 1, 2", "IF (L) THEN", "10 CONTINUE", "END IF", "END"}, "DONEST@3:7"},
		{"a LOGICAL DO variable", []string{"LOGICAL L", "DO 10 L = 1, 2", "10 CONTINUE", "END"}, "DOVAR@2:13"},
		{"an array as DO variable", []string{"DIMENSION A(3)", "DO 10 A = 1, 2", "10 CONTINUE", "END"}, "DOVAR@2:13"},
		{"a DO variable assigned", []string{"DO 10 I = 1, 2", "I = 5", "10 CONTINUE", "END"}, "DOVARSET@2:7"},
		{"a DO variable reused", []string{"DO 10 I = 1, 2", "DO 10 I = 1, 2", "10 CONTINUE", "END"}, "DOVARSET@2:13"},
		{"a zero step", []string{"DO 10 I = 1, 2, 0", "10 CONTINUE", "END"}, "DOSTEP0@1:23"},
		{"a zero REAL step", []string{"DO 10 X = 1, 2, 0.0", "10 CONTINUE", "END"}, "DOSTEP0@1:23"},
		{"a step folded to zero", []string{"DO 10 I = 1, 2, 1-1", "10 CONTINUE", "END"}, "DOSTEP0@1:24"},

		// Labels.
		{"a label twice", []string{"10 CONTINUE", "10 CONTINUE", "END"}, "DUPLABEL@2:1"},
		{"a jump to nowhere", []string{"GO TO 20", "END"}, "UNDEFLBL@1:13"},
		{"a jump to a declaration", []string{"10 INTEGER I", "GO TO 10", "END"}, "NOTEXEC@2:13"},
		{"a jump to a FORMAT", []string{"10 FORMAT (I5)", "GO TO 10", "END"}, "NOTIMPL@1:7 NOTEXEC@2:13"},
		{"a jump to ELSE", []string{"IF (L) THEN", "GO TO 10", "10 ELSE", "END IF", "END"}, "NOTEXEC@2:13"},
		{"a jump to a statement with an error", []string{"10 READ *, X", "GO TO 10", "END"}, "NOTIMPL@1:7"},
		{"a jump into an IF block", []string{"GO TO 10", "IF (L) THEN", "10 CONTINUE", "END IF", "END"}, "INTOBLOCK@1:13"},
		{"a jump between blocks", []string{"IF (L) THEN", "10 X = 1", "ELSE", "GO TO 10", "END IF", "END"}, "INTOBLOCK@4:13"},
		{"a jump to END IF from an earlier block", []string{"IF (L) THEN", "GO TO 10", "ELSE", "X = 1", "10 END IF", "END"}, "INTOBLOCK@2:13"},
		{"a jump to END IF from the last block", []string{"IF (L) THEN", "X = 2", "ELSE", "GO TO 10", "10 END IF", "END"}, ""},
		{"a jump out of a block", []string{"IF (L) THEN", "GO TO 10", "END IF", "10 CONTINUE", "END"}, ""},
		{"a jump into a loop", []string{"GO TO 10", "DO 20 I = 1, 2", "10 X = 1", "20 CONTINUE", "END"}, ""},
		{"a jump to a shared terminal from outside", []string{"DO 20 I = 1, 2", "GO TO 20", "DO 20 J = 1, 2", "20 CONTINUE", "END"}, "SHAREDTERM@2:13"},
		{"a jump to a shared terminal from inside", []string{"DO 20 I = 1, 2", "DO 20 J = 1, 2", "GO TO 20", "20 CONTINUE", "END"}, ""},

		// Recovery: a name whose declaration had an error gets no more
		// messages, and a unit with a declaration this phase can't read
		// gets only the messages about its structure.
		{"a bound with an error", []string{"DIMENSION A(N)", "A(1) = 2", "X = A(2) + A", "END"}, "BADBOUND@1:19"},
		{"a type not compiled yet", []string{"LOGICAL*1 B", "L = B .AND. .TRUE.", "CALL S(B)", "END"}, "NOTIMPL@1:17"},
		{"an array in COMMON", []string{"COMMON /C/ A(10)", "X = A(1)", "GO TO 99", "END"}, "NOTIMPL@1:7 UNDEFLBL@3:13"},
		{"a declaration with a syntax error", []string{"INTEGER A(", "X = A(1)", "END IF", "END"}, "SYNTAX@1:17 NOIF@3:7"},
		{"a FORMAT declares nothing", []string{"10 FORMAT (I5)", "X = 1", "Y = X(2)", "END"}, "NOTIMPL@1:7 NOTARRAY@3:11"},
		{"an incomplete unit's references", []string{"COMMON /C/ X", "I = F(1.0)", "END", "REAL FUNCTION F(X)", "F = X", "END"}, "NOTIMPL@1:7"},

		// What the parser parses and semantic analysis doesn't compile
		// yet (Phase 76): statements, reported where they are, and the
		// forms inside statements, reported where analysis reaches them.
		{"a READ statement", []string{"READ *, X", "END"}, "NOTIMPL@1:7"},
		{"a TYPE statement", []string{"TYPE *, X", "END"}, "NOTIMPL@1:7"},
		{"PAUSE", []string{"PAUSE", "END"}, "NOTIMPL@1:7"},
		{"an arithmetic IF", []string{"IF (X) 10, 10, 10", "10 END"}, "NOTIMPL@1:7"},
		{"a logical IF's READ", []string{"IF (X .GT. 0) READ *, Y", "END"}, "NOTIMPL@1:21"},
		{"a DO loop without a label", []string{"DO I = 1, 10", "END DO", "END"}, "NOTIMPL@1:7 NOTIMPL@2:7"},
		{"the computed GO TO", []string{"GO TO (10, 20), I", "10 CONTINUE", "20 END"}, "NOTIMPL@1:7"},
		{"COMMON", []string{"COMMON /B/ X", "END"}, "NOTIMPL@1:7"},
		{"an OPTIONS statement", []string{"OPTIONS /G_FLOATING", "PROGRAM P", "END"}, "NOTIMPL@1:7"},
		{"BLOCK DATA", []string{"BLOCK DATA B", "COMMON /C/ X", "END"}, "NOTIMPL@1:7"},
		{"a CHARACTER declaration", []string{"CHARACTER*10 S", "S = 'ABC'", "PRINT *, S", "END"}, "NOTIMPL@1:7"},
		{"a DOUBLE PRECISION function", []string{"DOUBLE PRECISION FUNCTION D(X)", "D = X", "END"}, "NOTIMPL@1:7"},
		{"two names of a type not compiled", []string{"COMPLEX A, B", "A = B", "END"}, "NOTIMPL@1:7"},
		{"a length after an untyped function", []string{"FUNCTION F*8(X)", "F = X", "END"}, "NOTIMPL@1:17"},
		{"initial values", []string{"INTEGER A(10)/10*0/", "A(1) = 2", "END"}, "NOTIMPL@1:20"},
		{"an alternate return's dummy", []string{"SUBROUTINE S(A, *)", "CALL T(*10)", "10 END"}, "NOTIMPL@1:23 NOTIMPL@2:14"},
		{"a Hollerith constant", []string{"X = 5HHELLO", "END"}, "NOTIMPL@1:11"},
		{"an octal constant", []string{"I = '17'O", "END"}, "NOTIMPL@1:11"},
		{"a COMPLEX constant", []string{"X = (1.0, 2.0)", "END"}, "NOTIMPL@1:11"},
		{"a substring assigned", []string{"C(1:3) = 'ABC'", "END"}, "NOTIMPL@1:7"},
		{"a substring compared", []string{"IF (C(I)(1:1) .EQ. 'X') STOP", "END"}, "NOTIMPL@1:11"},
		{"a record's field", []string{"X = R.F + 1", "END"}, "NOTIMPL@1:13"},
		{"%VAL", []string{"CALL S(%VAL(X))", "END"}, "NOTIMPL@1:14"},
		{"an implied-DO list", []string{"PRINT *, (I, I = 1, 3)", "END"}, "NOTIMPL@1:16"},
		{"WRITE with ERR=", []string{"WRITE (*, *, ERR=10) X", "10 END"}, "NOTIMPL@1:20"},
		{"direct access", []string{"WRITE (1'10) X", "END"}, "NOTIMPL@1:16"},
		{"a Hollerith bound", []string{"DIMENSION A(1H1)", "END"}, "NOTIMPL@1:19"},
		{"an octal integer", []string{"I = \"17 + 1", "DIMENSION A(\"10)", "A(8) = I", "END"}, "LATEDECL@2:7"},
		{"an octal integer too big", []string{"I = \"777777777777", "END"}, "INTRANGE@1:11"},
		{"NOTIMPL in an incomplete unit", []string{"COMMON /C/ X", "Y = 3HABC", "END"}, "NOTIMPL@1:7 NOTIMPL@2:11"},

		// References between units.
		{"a function's type", []string{"I = IFUN(1.0)", "END", "REAL FUNCTION IFUN(X)", "IFUN = X", "END"}, "FUNCTYPE@1:11"},
		{"a function's type agreeing", []string{"REAL IFUN", "X = IFUN(1.0)", "END", "REAL FUNCTION IFUN(X)", "IFUN = X", "END"}, ""},
		{"a function called", []string{"CALL F(1.0)", "END", "FUNCTION F(X)", "F = X", "END"}, "BADUSE@1:12"},
		{"a subroutine referenced", []string{"X = S(1.0)", "END", "SUBROUTINE S(X)", "END"}, "BADUSE@1:11"},
		{"a unit as a variable", []string{"S = 1", "END", "SUBROUTINE S", "END"}, "BADUSE@1:7"},
		{"the main program called", []string{"PROGRAM P", "END", "SUBROUTINE S", "CALL P", "END"}, "BADUSE@4:12"},
		{"too few arguments", []string{"CALL S(1)", "END", "SUBROUTINE S(A, B)", "END"}, "ARGCOUNT@1:12"},
		{"too many arguments", []string{"X = F(1.0, 2.0)", "END", "FUNCTION F(A)", "F = A", "END"}, "ARGCOUNT@1:11"},
		{"null arguments count", []string{"CALL S(1,)", "END", "SUBROUTINE S(A, B)", "END"}, ""},
	}

	for _, c := range cases {
		lines := make([]string, len(c.lines))
		for i, l := range c.lines {
			lines[i] = fixedLine(l)
		}

		_, ms := analyzeLines(t, "T.FOR", lines...)

		var got []string
		for _, m := range ms {
			got = append(got, fmt.Sprintf("%s@%d:%d", m.Ident, m.Pos.Line, m.Pos.Column))
		}

		if g := strings.Join(got, " "); g != c.want {
			t.Errorf("%s: got %q, want %q", c.name, g, c.want)

			for _, m := range ms {
				t.Log(m.String())
			}
		}
	}
}

// fixedLine is a test's line in fixed form: a leading label of digits
// goes in the label field, the rest in the statement field.
func fixedLine(l string) string {
	i := 0
	for i < len(l) && l[i] >= '0' && l[i] <= '9' {
		i++
	}

	if i > 0 && i < len(l) && l[i] == ' ' {
		return fmt.Sprintf("%-6s%s", l[:i], l[i+1:])
	}

	return "      " + l
}

// expressionUnit is the declarations TestExpressions's expressions are
// analyzed under.
var expressionUnit = []string{
	"      SUBROUTINE E(D, DA)",
	"      INTEGER I, J, IA(0:4), DA(*)",
	"      REAL X, Y, A(5)",
	"      LOGICAL L, M",
}

// expressionOf analyzes text as the one item of a PRINT statement in
// expressionUnit, returning the item's dump.
func expressionOf(t *testing.T, text string) (string, []diag.Message) {
	t.Helper()

	lines := append(append([]string{}, expressionUnit...), "      PRINT *, "+text, "      END")
	prog, ms := analyzeLines(t, "E.FOR", lines...)

	p, ok := prog.Units[0].Body[0].(*Print)
	if !ok || len(p.Items) != 1 {
		t.Fatalf("%s: no item: %s", text, Dump(prog))
	}

	return DumpExpr(p.Items[0]), ms
}

// TestExpressions checks expressions' types, the conversions mixed
// operations get (section 2.3.1.2), and folding.
func TestExpressions(t *testing.T) {
	cases := map[string]string{
		// Leaves.
		"I":       "I",
		"X":       "X",
		"L":       "L",
		"D":       "D",
		"A":       "A[]",
		"A(I)":    "A[I]",
		"IA(J)":   "IA[J]",
		"DA(3)":   "DA[3]",
		"A(X)":    "A[(INTEGER X)]",
		"A(L)":    "A[(INTEGER L)]",
		"F(I)":    "F(I)",
		"K(X)":    "K(X)",
		"F(A)":    "F(A[])",
		"F((I))":  "F((copy I))",
		"F(I+1)":  "F((+:I I 1))",
		"F(1)":    "F(1)",
		"'IT''S'": "'IT''S'",

		// Arithmetic: the result has the higher rank's type.
		"I + J":     "(+:I I J)",
		"I + X":     "(+:R (REAL I) X)",
		"X * I":     "(*:R X (REAL I))",
		"I / J * X": "(*:R (REAL (/:I I J)) X)",
		"-I":        "(-:I I)",
		"-X":        "(-:R X)",
		"+X":        "X",
		"L + 1":     "(+:I (INTEGER L) 1)",
		"L + X":     "(+:R (REAL (INTEGER L)) X)",
		"-L":        "(-:I (INTEGER L))",
		"(I + J)*2": "(*:I (+:I I J) 2)",

		// Comparisons: LOGICAL, the operands converted to one type.
		"I .LT. J":   "(.LT.:I I J)",
		"I .GE. X":   "(.GE.:R (REAL I) X)",
		"X+1 .EQ. Y": "(.EQ.:R (+:R X 1.0) Y)",
		"L .NE. M":   "(.NE.:I (INTEGER L) (INTEGER M))",

		// Logical operators: LOGICAL on LOGICALs, bitwise INTEGER
		// otherwise.
		"L .AND. M":       "(.AND.:L L M)",
		"L .OR. I .GT. J": "(.OR.:L L (.GT.:I I J))",
		".NOT. L":         "(.NOT.:L L)",
		"I .AND. J":       "(.AND.:I I J)",
		"I .OR. L":        "(.OR.:I I (INTEGER L))",
		".NOT. I":         "(.NOT.:I I)",
		"L .AND. .NOT. M": "(.AND.:L L (.NOT.:L M))",

		// Folding.
		"2 + 3":          "5",
		"7 / 2":          "3",
		"-7 / 2":         "-3",
		"2 * 3 + 4":      "10",
		"1 + 2.5":        "3.5",
		"1 / 4 + 1 / 4":  "0",
		"1.0 / 3":        "0.33333334",
		"-2147483648":    "-2147483648",
		"- (3)":          "-3",
		"2 .GT. 1":       ".TRUE.",
		"2.5 .LE. 1":     ".FALSE.",
		".NOT. .TRUE.":   ".FALSE.",
		".TRUE. .AND. 5": "5",
		"6 .AND. 3":      "2",
		"I + (2 - 2)":    "(+:I I 0)",
		// 2**24 + 1 is halfway between two F_floating values: the VAX
		// rounds it away from zero, where IEEE arithmetic would round it
		// to even (16777216).
		"16777217 + 0.0": "1.6777218E+07",
		"1E-38 / 1E10":   "0.0",
	}

	for text, want := range cases {
		got, ms := expressionOf(t, text)

		if len(ms) != 0 {
			t.Errorf("%s: %v", text, ms)

			continue
		}

		if got != want {
			t.Errorf("%s: got %s, want %s", text, got, want)
		}
	}
}

// TestConstants checks constants' values: REAL ones rounded to
// F_floating once, as the VAX rounds, and given as the longword the
// MACRO file will hold.
func TestConstants(t *testing.T) {
	cases := map[string]uint32{
		"1.0":       0x00004080,
		"1.5":       0x000040C0,
		"-2.5":      0x0000C120,
		"0.1":       0xCCCD3ECC,
		"1E3":       0x0000457A,
		"1.":        0x00004080,
		".5":        0x00004000,
		"3 * 0.5":   0x000040C0,
		"1 / 3.0":   0xAAAB3FAA,
		"16777217.": 0x00014C80, // halfway, rounded away from zero
		"0.0":       0,
	}

	for text, want := range cases {
		lines := append(append([]string{}, expressionUnit...), "      X = "+text, "      END")
		prog, ms := analyzeLines(t, "E.FOR", lines...)

		if len(ms) != 0 {
			t.Errorf("%s: %v", text, ms)

			continue
		}

		c, ok := prog.Units[0].Body[0].(*Assign).Value.(*Const)
		if !ok {
			t.Errorf("%s: not a constant", text)

			continue
		}

		if got := c.Bits(); got != want {
			t.Errorf("%s: ^X%08X, want ^X%08X", text, got, want)
		}
	}
}
