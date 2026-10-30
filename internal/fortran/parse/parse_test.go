package parse

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
	"github.com/tucats/govax/internal/fortran/lex"
	"github.com/tucats/govax/internal/fortran/source"
)

// update rewrites the golden files from what the parser produces:
// go test ./internal/fortran/parse -update. Review the diff before
// committing it.
var update = flag.Bool("update", false, "rewrite the golden .ast files")

// parseLines runs the first three passes over lines as the fixed-format
// file spec, returning the tree and the messages.
func parseLines(t *testing.T, spec string, lines ...string) (*ast.File, []diag.Message) {
	t.Helper()

	var messages diag.List

	program := source.Read(source.Form{}, &source.File{Spec: spec, Lines: lines}, nil, &messages)
	file := Parse(lex.Lex(program.Statements, &messages), &messages)

	return file, messages.Messages()
}

// fixed puts text in a fixed-format line's statement field.
func fixed(text string) string {
	return "      " + text
}

// idents are the messages' identifiers.
func idents(ms []diag.Message) []string {
	var list []string
	for _, m := range ms {
		list = append(list, m.Ident)
	}

	return list
}

// TestExpressions checks the shape of expressions: precedence, grouping,
// the signs, and the leaves, each as the value of an assignment.
func TestExpressions(t *testing.T) {
	cases := map[string]string{
		// Precedence and grouping.
		"A + B*C":            "(+ A (* B C))",
		"A*B + C":            "(+ (* A B) C)",
		"A - B - C":          "(- (- A B) C)",
		"A / B / C":          "(/ (/ A B) C)",
		"A**B**C":            "(** A (** B C))",
		"A*B**C":             "(* A (** B C))",
		"(A + B)*C":          "(* (paren (+ A B)) C)",
		"A // B // C":        "(// (// A B) C)",
		"A + B .GT. C*D":     "(.GT. (+ A B) (* C D))",
		"A .LT. B .AND. C":   "(.AND. (.LT. A B) C)",
		"A .OR. B .AND. C":   "(.OR. A (.AND. B C))",
		"A .AND. B .OR. C":   "(.OR. (.AND. A B) C)",
		"A .EQV. B .OR. C":   "(.EQV. A (.OR. B C))",
		"A .XOR. B .NEQV. C": "(.NEQV. (.XOR. A B) C)",

		// The signs: a leading sign takes the whole term, and so does a
		// sign after another operator (section 2.3.1).
		"-A":         "(- A)",
		"-A*B":       "(- (* A B))",
		"-A**2":      "(- (** A 2))",
		"-A + B":     "(+ (- A) B)",
		"+A - B":     "(- (+ A) B)",
		"A**-B*C":    "(** A (- (* B C)))",
		"A*-B":       "(* A (- B))",
		"A*-B + C":   "(+ (* A (- B)) C)",
		"A + -B*C":   "(+ A (- (* B C)))",
		"A .LT. -B":  "(.LT. A (- B))",
		"-A .LT. -B": "(.LT. (- A) (- B))",

		// .NOT. takes what binds more tightly than it.
		".NOT. A":                 "(.NOT. A)",
		".NOT. A .AND. B":         "(.AND. (.NOT. A) B)",
		".NOT. A .EQ. B":          "(.NOT. (.EQ. A B))",
		"A .AND. .NOT. B":         "(.AND. A (.NOT. B))",
		".NOT. .NOT. A":           "(.NOT. (.NOT. A))",
		"I .GT. 0 .AND. J .LE. 5": "(.AND. (.GT. I 0) (.LE. J 5))",

		// Leaves.
		"1":                 "1",
		"1.5E3":             "1.5E3",
		"2.5D0":             "2.5D0",
		".TRUE.":            ".TRUE.",
		".FALSE.":           ".FALSE.",
		"'IT''S'":           "'IT''S'",
		"A(I)":              "A(I)",
		"A(I+1, J)":         "A((+ I 1), J)",
		"F()":               "F()",
		"F(G(X), (Y))":      "F(G(X), (paren Y))",
		"A(I)*B(I) - C(-1)": "(- (* A(I) B(I)) C((- 1)))",
		"1.EQ.2":            "(.EQ. 1 2)",
	}

	for text, want := range cases {
		file, ms := parseLines(t, "E.FOR", fixed("X = "+text), fixed("END"))
		if len(ms) != 0 {
			t.Errorf("%s: messages %v", text, ms)

			continue
		}

		s, ok := file.Units[0].Body[0].(*ast.AssignStmt)
		if !ok {
			t.Errorf("%s: got %T", text, file.Units[0].Body[0])

			continue
		}

		if got := ast.DumpExpr(s.Value); got != want {
			t.Errorf("X = %s:\n got %s\nwant %s", text, got, want)
		}
	}
}

// TestStatements checks each statement the parser takes, one at a time,
// as the second statement of a main program (or the first, for the
// headers).
func TestStatements(t *testing.T) {
	cases := []struct{ text, want string }{
		{"PROGRAM SQUARES", "PROGRAM SQUARES"},
		{"FUNCTION F(X)", "FUNCTION F(X)"},
		{"FUNCTION F()", "FUNCTION F()"},
		{"INTEGER FUNCTION ISQ(N)", "INTEGER FUNCTION ISQ(N)"},
		{"REAL*4 FUNCTION R(A, B)", "REAL*4 FUNCTION R(A, B)"},
		{"LOGICAL FUNCTION L*4(A)", "LOGICAL*4 FUNCTION L(A)"},
		{"SUBROUTINE S", "SUBROUTINE S"},
		{"SUBROUTINE S(A, B, C)", "SUBROUTINE S(A, B, C)"},

		{"INTEGER I, J, K", "INTEGER I, J, K"},
		{"INTEGER*2 I, M12*4, IVEC*4(10)", "INTEGER*2 I, M12*4, IVEC*4(10)"},
		{"REAL A(10), B(0:9), C(-5:5, 3)", "REAL A(10), B(0:9), C((- 5):5, 3)"},
		{"REAL D(N), E(*), F(2, *), G(0:*)", "REAL D(N), E(*), F(2, *), G(0:*)"},
		{"LOGICAL FLAG", "LOGICAL FLAG"},
		{"DIMENSION A(10), B(N+1)", "DIMENSION A(10), B((+ N 1))"},

		{"X = 1", "X = 1"},
		{"A(I) = A(I-1) + 1", "A(I) = (+ A((- I 1)) 1)"},
		{"IF (X) = 3", "IF(X) = 3"},
		{"DO 10 I = 1.5", "DO10I = 1.5"},
		{"F(X) = X*X", "F(X) = (* X X)"},

		{"CALL EXIT", "CALL EXIT"},
		{"CALL S()", "CALL S()"},
		{"CALL S(A, B(1), 3, X+1, (Y))", "CALL S(A, B(1), 3, (+ X 1), (paren Y))"},
		{"CALL S(A,,B)", "CALL S(A, , B)"},
		{"CALL S(,)", "CALL S(, )"},

		{"GO TO 10", "GO TO 10"},
		{"GOTO 00020", "GO TO 20"},
		{"IF (X .GT. 0) GO TO 10", "IF (.GT. X 0) GO TO 10"},
		{"IF (L) X = -X", "IF L X = (- X)"},
		{"IF (A .AND. B) CALL S(1)", "IF (.AND. A B) CALL S(1)"},
		{"IF (I .EQ. 0) PRINT *, 'ZERO'", "IF (.EQ. I 0) PRINT * 'ZERO'"},
		{"IF (A .GT. B) THEN", "IF (.GT. A B) THEN"},
		{"ELSE IF (A .LT. B/2.) THEN", "ELSE IF (.LT. A (/ B 2.)) THEN"},
		{"ELSE", "ELSE"},
		{"END IF", "END IF"},

		{"DO 10 I = 1, 5", "DO 10 I = 1, 5"},
		{"DO 10, I = N, 1, -1", "DO 10 I = N, 1, (- 1)"},
		{"DO 20 X = 0.0, 1.0, 0.1", "DO 20 X = 0.0, 1.0, 0.1"},
		{"DO 10 E1 = 1, 5", "DO 10 E1 = 1, 5"},
		{"CONTINUE", "CONTINUE"},
		{"STOP", "STOP"},
		{"STOP 'DONE'", "STOP 'DONE'"},
		{"STOP 12345", "STOP 12345"},
		{"RETURN", "RETURN"},
		{"RETURN 2", "RETURN 2"},

		{"PRINT *", "PRINT *"},
		{"PRINT *, I, I*I, 'X', .TRUE.", "PRINT * I (* I I) 'X' .TRUE."},
		{"PRINT 100, X", "PRINT 100 X"},
		{"PRINT '(I5)', N", "PRINT '(I5)' N"},
		{"PRINT *, (X+1)*2", "PRINT * (* (paren (+ X 1)) 2)"},
		{"WRITE (*,*) I", "WRITE (*, *) I"},
		{"WRITE (*,*)", "WRITE (*, *)"},
		{"WRITE (6, 100) A, B", "WRITE (6, 100) A B"},
		{"WRITE (UNIT=*, FMT=*) X", "WRITE (*, *) X"},
		{"WRITE (FMT=*, UNIT=6) X", "WRITE (6, *) X"},
		{"WRITE (LUN, *) X", "WRITE (LUN, *) X"},
		{"WRITE (6) X", "WRITE (6) X"},
	}

	for _, c := range cases {
		var lines []string

		switch {
		case strings.Contains(c.text, "FUNCTION"), strings.HasPrefix(c.text, "SUBROUTINE"),
			strings.HasPrefix(c.text, "PROGRAM"):
			lines = []string{fixed(c.text), fixed("END")}
		default:
			lines = []string{fixed("CONTINUE"), fixed(c.text), fixed("END")}
		}

		file, ms := parseLines(t, "S.FOR", lines...)
		if len(ms) != 0 {
			t.Errorf("%s: messages %v", c.text, ms)

			continue
		}

		u := file.Units[0]

		s := u.Header
		if len(lines) == 3 {
			s = u.Body[1]
		}

		if got := ast.DumpStmt(s); got != c.want {
			t.Errorf("%s:\n got %s\nwant %s", c.text, got, c.want)
		}
	}
}

// TestErrors checks the parser's messages, where they point, and that
// each statement in error is kept with its label.
func TestErrors(t *testing.T) {
	cases := []struct {
		text   string
		ident  string
		column int
		text2  string // the message's text, if checked
	}{
		{"X = ", "SYNTAX", 11, "expected an expression, but found the end of the statement"},
		{"X = (A + B", "SYNTAX", 17, "expected ')', but found the end of the statement"},
		{"X = A +* B", "SYNTAX", 14, "expected an expression, but found '*'"},
		{"X + 1 = 2", "SYNTAX", 9, "expected '=', but found '+'"},
		{"X = A, B", "EXTRATEXT", 12, "the statement should end before ','"},
		{"X = A(1", "SYNTAX", 14, ""},
		{"CONTINUE 5", "EXTRATEXT", 16, ""},
		{"DO 10 I 1, 5", "SYNTAX", 16, "expected '=', but found ','"},
		{"DO 0 I = 1, 5", "BADLABELREF", 10, "0 is not a statement label, which is 1 to 5 digits, not all zeros"},
		{"GO TO 123456", "BADLABELREF", 13, ""},
		{"GO TO X1", "NOTIMPL", 7, "the assigned GO TO statement is not implemented yet"},
		{"GO TO (10, 20), I", "NOTIMPL", 7, "the computed GO TO statement is not implemented yet"},
		{"IF (X .GT. 0", "SYNTAX", 19, ""},
		{"IF (X)", "SYNTAX", 13, "expected a statement after the condition, but found the end of the statement"},
		{"IF (X) DO 10 I = 1, 5", "BADIFSTMT", 14, "a logical IF can't execute a DO statement"},
		{"IF (X) IF (Y) Z = 1", "BADIFSTMT", 14, "a logical IF can't execute another logical IF"},
		{"IF (X) INTEGER I", "BADIFSTMT", 14, "a logical IF can't execute an INTEGER statement"},
		{"IF (X) END IF", "BADIFSTMT", 14, ""},
		{"IF (X) READ *, Y", "NOTIMPL", 14, "the READ statement is not implemented yet"},
		{"IF (X .GT. 0)", "SYNTAX", 20, ""},
		{"ELSE IF (X)", "SYNTAX", 18, "expected 'THEN', but found the end of the statement"},
		{"STOP 123456", "SYNTAX", 12, "expected a character constant or 1 to 5 digits, but found the number 123456"},
		{"STOP X", "SYNTAX", 12, ""},
		{"CALL S(A", "SYNTAX", 15, ""},
		{"CALL S(*10)", "NOTIMPL", 14, "an alternate return argument is not implemented yet"},
		{"CALL S(%VAL(X))", "NOTIMPL", 14, "%VAL is not implemented yet"},
		{"X = 5HHELLO", "NOTIMPL", 11, "a Hollerith constant is not implemented yet"},
		{"X = '17'O", "NOTIMPL", 11, ""},
		{"X = (1.0, 2.0)", "NOTIMPL", 11, "a COMPLEX constant is not implemented yet"},
		{"C(1:3) = 'ABC'", "NOTIMPL", 10, "a character substring is not implemented yet"},
		{"C(I)(1:3) = 'ABC'", "NOTIMPL", 11, ""},
		{"IF (C(I)(1:1) .EQ. 'X') GO TO 10", "NOTIMPL", 15, ""},
		{"INTEGER A(10)/10*0/", "NOTIMPL", 20, ""},
		{"DIMENSION A", "SYNTAX", 18, "expected '(', but found the end of the statement"},
		{"PRINT *, (A(I), I = 1, 10)", "NOTIMPL", 16, "an implied-DO list is not implemented yet"},
		{"PRINT * X", "SYNTAX", 15, "expected ',', but found the name X"},
		{"WRITE (*, *, ERR=10) X", "NOTIMPL", 20, "the ERR= specifier is not implemented yet"},
		{"WRITE (FMT=*) X", "SYNTAX", 19, "expected a unit (UNIT=), but found ')'"},
		{"WRITE (*, *, *) X", "SYNTAX", 20, ""},
		{"WRITE (BAD=1) X", "SYNTAX", 14, "expected UNIT=, FMT=, ERR=, IOSTAT=, REC=, or NML=, but found BAD="},
		{"WRITE (1'10) X", "NOTIMPL", 15, ""},
		{"DO I = 1, 10", "NOTIMPL", 7, "a DO loop without a statement label is not implemented yet"},
		{"READ *, X", "NOTIMPL", 7, "the READ statement is not implemented yet"},
		{"COMMON /B/ X", "NOTIMPL", 7, "the COMMON statement is not implemented yet"},
		{"CHARACTER*10 S", "NOTIMPL", 7, "the CHARACTER statement is not implemented yet"},
		{"IF (X) 10, 20, 30", "NOTIMPL", 7, "the arithmetic IF statement is not implemented yet"},
	}

	for _, c := range cases {
		lines := []string{fixed("CONTINUE"), "   99 " + c.text, fixed("END")}

		file, ms := parseLines(t, "ERR.FOR", lines...)
		if len(ms) != 1 || ms[0].Ident != c.ident || ms[0].Pos.Column != c.column || ms[0].Pos.Line != 2 ||
			(c.text2 != "" && ms[0].Text != c.text2) {
			t.Errorf("%s: got %v, want %s at column %d %q", c.text, ms, c.ident, c.column, c.text2)

			continue
		}

		// The statement is kept, with its label, and the unit goes on.
		u := file.Units[0]
		if len(u.Body) != 3 || u.Body[1].Base().Label != 99 {
			t.Errorf("%s: body %q", c.text, ast.Dump(file))

			continue
		}

		switch s := u.Body[1].(type) {
		case *ast.NotImplStmt:
			if c.ident != "NOTIMPL" {
				t.Errorf("%s: a NotImplStmt for %s", c.text, c.ident)
			}
		case *ast.BadStmt:
			if c.ident == "NOTIMPL" {
				t.Errorf("%s: a BadStmt for NOTIMPL", c.text)
			}
		default:
			t.Errorf("%s: kept as %T", c.text, s)
		}
	}
}

// TestUnits checks how statements are divided into program units.
func TestUnits(t *testing.T) {
	file, ms := parseLines(t, "U.FOR",
		fixed("X = 1"), // an unnamed main program
		fixed("END"),
		fixed("SUBROUTINE S"),
		fixed("RETURN"), // no END: NOEND at the next header
		fixed("FUNCTION F(A)"),
		fixed("F = A"),
		fixed("E N D"),
		fixed("BLOCK DATA"),
		fixed("END"),
		fixed("PROGRAM LATE"),
		fixed("STOP"), // no END at the file's end
	)

	want := []string{
		"main program ", "SUBROUTINE S", "FUNCTION F", "BLOCK DATA ", "main program LATE",
	}

	var got []string
	for _, u := range file.Units {
		got = append(got, u.Kind().String()+" "+u.Name())
	}

	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("units %q, want %q", got, want)
	}

	if strings.Join(idents(ms), " ") != "NOEND NOTIMPL NOEND" {
		t.Fatalf("messages: %v", ms)
	}

	if ms[0].Text != "SUBROUTINE S has no END statement" || ms[0].Pos.Line != 4 || ms[0].Pos.Column != 13 {
		t.Errorf("first NOEND: %v", ms[0])
	}

	if ms[2].Text != "the main program LATE has no END statement" || ms[2].Pos.Line != 11 {
		t.Errorf("second NOEND: %v", ms[2])
	}

	if _, ok := file.Units[0].Body[1].(*ast.EndStmt); !ok || file.Units[0].Header != nil {
		t.Errorf("first unit: %s", ast.Dump(file))
	}
}

// TestNoStatements checks the things that are in no unit: a file of
// comments, a directive, and a label with no statement.
func TestNoStatements(t *testing.T) {
	file, ms := parseLines(t, "N.FOR", "C     nothing")
	if len(file.Units) != 0 || len(ms) != 0 {
		t.Errorf("a file of comments: %d units, %v", len(file.Units), ms)
	}

	file, ms = parseLines(t, "N.FOR", "CDEC$ IDENT 'X'", "   10", fixed("END"))
	if strings.Join(idents(ms), " ") != "NOTIMPL EMPTYSTMT" || len(file.Units) != 1 || len(file.Units[0].Body) != 1 {
		t.Errorf("a directive and an empty statement: %v %s", ms, ast.Dump(file))
	}

	if ms[1].Text != "statement label 10 is on no statement" || ms[1].Pos.Column != 4 {
		t.Errorf("EMPTYSTMT: %v", ms[1])
	}
}

// TestPositions checks that nodes point at their tokens: a statement at
// its keyword, a binary operation at its operator, and the rest at their
// first tokens, across a continuation line.
func TestPositions(t *testing.T) {
	file, _ := parseLines(t, "P.FOR",
		"   10 X = A +",
		"     1     B(I)",
		fixed("END"),
	)

	s := file.Units[0].Body[0].(*ast.AssignStmt)
	plus := s.Value.(*ast.Binary)
	b := plus.Y.(*ast.Apply)

	got := []diag.Position{s.Pos, s.Target.Position(), plus.Pos, plus.X.Position(), b.Pos, b.Args[0].Position()}
	want := [][2]int{{1, 7}, {1, 7}, {1, 13}, {1, 11}, {2, 12}, {2, 14}}

	for i, p := range got {
		if p.Line != want[i][0] || p.Column != want[i][1] {
			t.Errorf("node %d at %d:%d, want %d:%d", i, p.Line, p.Column, want[i][0], want[i][1])
		}
	}

	if s.Label != 10 || s.Source.Label != 10 {
		t.Errorf("label %d", s.Label)
	}
}

// TestGolden parses each program in testdata and compares its dump and
// messages with the program's .ast file.
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
		file, ms := parseLines(t, strings.ToUpper(filepath.Base(src)), lines...)

		var b strings.Builder

		b.WriteString(ast.Dump(file))

		for _, m := range ms {
			b.WriteString("\n" + m.String() + "\n")
		}

		golden := strings.TrimSuffix(src, ".for") + ".ast"

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
