package lex

import (
	"reflect"
	"strconv"
	"testing"

	"github.com/tucats/govax/internal/fortran/diag"
	"github.com/tucats/govax/internal/fortran/source"
)

// lexLines reads lines as a fixed-format source file and lexes it,
// returning each statement's dump and the messages' identifiers.
func lexLines(t *testing.T, lines ...string) ([]*Statement, []string, []string) {
	t.Helper()

	var messages diag.List

	program := source.Read(source.Form{}, &source.File{Spec: "TEST.FOR", Lines: lines}, nil, &messages)
	if messages.Failed() {
		t.Fatalf("the source pass failed: %v", messages.Messages())
	}

	statements := Lex(program.Statements, &messages)

	dumps := make([]string, len(statements))
	for i, s := range statements {
		dumps[i] = Dump(s)
	}

	var idents []string
	for _, m := range messages.Messages() {
		idents = append(idents, m.Ident)
	}

	return statements, dumps, idents
}

// fixed puts text in a fixed-format line's statement field.
func fixed(text string) string {
	return "      " + text
}

// TestClassify checks single statements: FORTRAN's hard cases first,
// then each kind of statement and token. A statement is lexed as the
// second of its unit (after a CONTINUE), except where a case says it is
// first.
func TestClassify(t *testing.T) {
	cases := []struct {
		text  string
		want  string
		first bool // the unit's first statement
	}{
		// The DO test: a comma after the = at level 0.
		{text: "DO 10 I = 1, 5", want: "DO: 10 I = 1 , 5"},
		{text: "DO 10 I = 1.5", want: "assignment: DO10I = 1.5"},
		{text: "DO 10, I = 1, 5", want: "DO: 10 , I = 1 , 5"},
		{text: "DO 10 I = F(1,2)", want: "assignment: DO10I = F ( 1 , 2 )"},
		{text: "DO 10 I = 1, N, 2", want: "DO: 10 I = 1 , N , 2"},

		// A label before a name starting E, D, or Q and a digit.
		{text: "DO 10 E1 = 1, 5", want: "DO: 10 E1 = 1 , 5"},
		{text: "DO 20 D2 = 1.0, 2.0", want: "DO: 20 D2 = 1.0 , 2.0"},

		// The IF statements, and arrays named IF.
		{text: "IF (X) = 3", want: "assignment: IF ( X ) = 3"},
		{text: "IF (I(2)) = 3", want: "assignment: IF ( I ( 2 ) ) = 3"},
		{text: "IF (X) GO TO 10", want: "logical IF: ( X ) / GO TO: 10"},
		{text: "IF (J.GT.4 .OR. J.LT.1) GO TO 250", want: "logical IF: ( J .GT. 4 .OR. J .LT. 1 ) / GO TO: 250"},
		{text: "IF (E) THEN I = J", want: "logical IF: ( E ) / assignment: THENI = J"},
		{text: "IF (X) THEN = 1", want: "logical IF: ( X ) / assignment: THEN = 1"},
		{text: "IF (A .GT. B) THEN", want: "IF THEN: ( A .GT. B ) THEN"},
		{text: "IF (X) 10, 20, 30", want: "arithmetic IF: ( X ) 10 , 20 , 30"},
		{text: "IF (X) IF (Y) Z = 1", want: "logical IF: ( X ) / logical IF: ( Y ) / assignment: Z = 1"},
		{text: "IF (C .EQ. ')') X = 1", want: "logical IF: ( C .EQ. ')' ) / assignment: X = 1"},
		{text: "ELSE IF (A .GT. B/2.) THEN", want: "ELSE IF: ( A .GT. B / 2. ) THEN"},
		{text: "ELSE", want: "ELSE:"},
		{text: "END IF", want: "END IF:"},
		{text: "ELSE X = 1", want: "assignment: ELSEX = 1"},

		// Keywords are names elsewhere.
		{text: "CALL X = 1", want: "assignment: CALLX = 1"},
		{text: "CALL SUB(A, 'B', 3)", want: "CALL: SUB ( A , 'B' , 3 )"},
		{text: "CALL EXIT", want: "CALL: EXIT"},
		{text: "GO TO 7734", want: "GO TO: 7734"},
		{text: "G O T O 10", want: "GO TO: 10"},
		{text: "GO TO (10, 20), I", want: "GO TO: ( 10 , 20 ) , I"},
		{text: "END = 3", want: "assignment: END = 3"},

		// Typed FUNCTION statements, only at a unit's start.
		{text: "INTEGER FUNCTION F(X)", want: "FUNCTION (INTEGER): FUNCTION F ( X )", first: true},
		{text: "INTEGER FUNCTION F(10)", want: "INTEGER: FUNCTIONF ( 10 )"},
		{text: "REAL*8 FUNCTION G(Y)", want: "FUNCTION (REAL): * 8 FUNCTION G ( Y )", first: true},
		{text: "CHARACTER*(*) FUNCTION H(S)", want: "FUNCTION (CHARACTER): * ( * ) FUNCTION H ( S )", first: true},
		{text: "DOUBLE PRECISION FUNCTION D(X)", want: "FUNCTION (DOUBLE PRECISION): FUNCTION D ( X )", first: true},
		{text: "FUNCTION ROOT(A)", want: "FUNCTION: ROOT ( A )", first: true},
		{text: "LOGICAL FUNCTION", want: "LOGICAL: FUNCTION", first: true},

		// Declarations, and type lengths before names.
		{text: "INTEGER COUNT, MATRIX(4,4), SUM", want: "INTEGER: COUNT , MATRIX ( 4 , 4 ) , SUM"},
		{text: "REAL*8 D1, D2", want: "REAL: * 8 D1 , D2"},
		{text: "INTEGER*2 I, J, M12*4, E1", want: "INTEGER: * 2 I , J , M12 * 4 , E1"},
		{text: "REAL X(3)/3*1.5E2/", want: "REAL: X ( 3 ) / 3 * 1.5E2 /"},
		{text: "DOUBLE PRECISION D", want: "DOUBLE PRECISION: D"},
		{text: "DIMENSION A(-1:5), B(10)", want: "DIMENSION: A ( - 1 : 5 ) , B ( 10 )"},
		{text: "PARAMETER (N = 10)", want: "PARAMETER: ( N = 10 )"},
		{text: "PROGRAM SQUARES", want: "PROGRAM: SQUARES", first: true},
		{text: "SUBROUTINE CHECK(X,Y,*,*,C)", want: "SUBROUTINE: CHECK ( X , Y , * , * , C )", first: true},

		// Numbers and the dotted operators.
		{text: "L = I.EQ.1.AND.J.EQ.2", want: "assignment: L = I .EQ. 1 .AND. J .EQ. 2"},
		{text: "X = 1.E5 + .5 + 2.D0 + 3. + 1E3 + 1.5Q-2", want: "assignment: X = 1.E5 + .5 + 2.D0 + 3. + 1E3 + 1.5Q-2"},
		{text: "L = .TRUE..AND..NOT..FALSE.", want: "assignment: L = .TRUE. .AND. .NOT. .FALSE."},
		{text: "L = A .EQV. B .NEQV. C .XOR. D", want: "assignment: L = A .EQV. B .NEQV. C .XOR. D"},
		{text: "X = 1.EQ.2", want: "assignment: X = 1 .EQ. 2"},
		{text: "X = A**B*C - D/E", want: "assignment: X = A ** B * C - D / E"},
		{text: "C = A // 'B'", want: "assignment: C = A // 'B'"},
		{text: "X = A**-B*C", want: "assignment: X = A ** - B * C"},

		// Constants.
		{text: "PRINT *, 'IT''S', I", want: "PRINT: * , 'IT''S' , I"},
		{text: "PRINT *, 'A  b'", want: "PRINT: * , 'A  b'"},
		{text: "print *, x", want: "PRINT: * , X"},
		{text: "STOP 'END OF RUN'", want: "STOP: 'END OF RUN'"},
		{text: "STOP 98", want: "STOP: 98"},
		{text: "CALL S(5HHELLO, 4H A B)", want: `CALL: S ( 5H"HELLO" , 4H" A B" )`},
		{text: "DATA X/2*4HAB D/", want: `DATA: X / 2 * 4H"AB D" /`},
		{text: "I = '17'O + '1F'X + \"17", want: "assignment: I = '17'O + '1F'X + '17'O"},
		{text: "READ (1'10) X", want: "READ: ( 1 ' 10 ) X"},
		{text: "WRITE (*,*) 'X =', X", want: "WRITE: ( * , * ) 'X =' , X"},

		// Other keyword statements.
		{text: "DO WHILE (X .LT. 5)", want: "DO WHILE: WHILE ( X .LT. 5 )"},
		{text: "DO 10 WHILE (X)", want: "DO WHILE: 10 WHILE ( X )"},
		{text: "DO 10 I 1, 5", want: "DO: 10 I1 , 5"},
		{text: "END FILE 5", want: "ENDFILE: 5"},
		{text: "END DO", want: "END DO:"},
		{text: "CONTINUE", want: "CONTINUE:"},
		{text: "RETURN 1", want: "RETURN: 1"},
		{text: "BLOCK DATA INIT", want: "BLOCK DATA: INIT", first: true},
		{text: "FORMAT (1X, I5, 'X=', F8.2)", want: "FORMAT:"},
	}

	for _, c := range cases {
		lines := []string{fixed(c.text)}
		if !c.first {
			lines = append([]string{fixed("CONTINUE")}, lines...)
		}

		_, dumps, idents := lexLines(t, lines...)

		if len(idents) != 0 {
			t.Errorf("%s: messages %v", c.text, idents)
		}

		if got := dumps[len(dumps)-1]; got != c.want {
			t.Errorf("%s:\n got %s\nwant %s", c.text, got, c.want)
		}
	}
}

// TestTokenKinds checks the kinds and values of a statement's tokens,
// which Dump's short form doesn't show.
func TestTokenKinds(t *testing.T) {
	statements, _, _ := lexLines(t,
		fixed("IF (X .GT. 1.5) THEN"),
		fixed("L = .TRUE. .AND. I .EQ. 'IT''S' .OR. 3HA'B"),
		fixed("END IF"),
	)

	var got []string
	for _, s := range statements[:2] {
		for _, tok := range s.Tokens {
			got = append(got, tok.String())
		}
	}

	want := []string{
		"operator (", "name X", "operator .GT.", "real 1.5", "operator )", "keyword THEN",
		"name L", "operator =", "logical .TRUE.", "operator .AND.", "name I", "operator .EQ.",
		`character "IT'S"`, "operator .OR.", `hollerith "A'B"`,
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("tokens:\n got %q\nwant %q", got, want)
	}

	if s := statements[0]; s.Kind != IfThen || !s.Kind.Executable() {
		t.Errorf("IF THEN's kind is %v", s.Kind)
	}
}

// TestUnitStarts checks that a typed FUNCTION statement is recognized at
// the start of each program unit: the file's start and after each END.
func TestUnitStarts(t *testing.T) {
	_, dumps, idents := lexLines(t,
		fixed("REAL FUNCTION F(X)"),
		fixed("REAL FUNCTIONG(2)"),
		fixed("END"),
		fixed("INTEGER FUNCTION G(Y)"),
		fixed("END"),
		"C     a comment between units",
		fixed("LOGICAL FUNCTION H(Z)"),
		fixed("END"),
	)

	want := []string{
		"FUNCTION (REAL): FUNCTION F ( X )",
		"REAL: FUNCTIONG ( 2 )",
		"END:",
		"FUNCTION (INTEGER): FUNCTION G ( Y )",
		"END:",
		"FUNCTION (LOGICAL): FUNCTION H ( Z )",
		"END:",
	}

	if len(idents) != 0 || !reflect.DeepEqual(dumps, want) {
		t.Errorf("got %q %v\nwant %q", dumps, idents, want)
	}
}

// TestEndWithContinuation checks section 5.5's rule: an initial line
// holding only END is an END statement even when continuation lines
// follow it, while END on a line of its own text continued is not.
func TestEndWithContinuation(t *testing.T) {
	_, dumps, _ := lexLines(t,
		fixed("CONTINUE"),
		fixed("END"),
		"     1IF",
		fixed("E"),
		"     1ND IF",
	)

	want := []string{"CONTINUE:", "END:", "END IF:"}
	if !reflect.DeepEqual(dumps, want) {
		t.Errorf("got %q, want %q", dumps, want)
	}
}

// TestLabels checks that statements keep their labels, a logical IF's
// statement having none of its own.
func TestLabels(t *testing.T) {
	statements, dumps, _ := lexLines(t,
		"   10 CONTINUE",
		"   20 IF (X) GO TO 10",
		"30\tX = 1",
	)

	want := []string{"10 CONTINUE:", "20 logical IF: ( X ) / GO TO: 10", "30 assignment: X = 1"}
	if !reflect.DeepEqual(dumps, want) {
		t.Errorf("got %q, want %q", dumps, want)
	}

	if statements[1].Then.Label != 0 {
		t.Errorf("the logical IF's statement has label %d", statements[1].Then.Label)
	}
}

// TestPositions checks that tokens point back at their columns, across a
// continuation line, and that blanks squeezed out don't shift them.
func TestPositions(t *testing.T) {
	statements, _, _ := lexLines(t,
		"   10 X = A   +",
		"     1      'B C' + 2",
	)

	s := statements[0]

	var got []string
	for _, tok := range s.Tokens {
		p := s.Position(tok.Offset)
		got = append(got, tok.short()+"@"+strconv.Itoa(p.Line)+":"+strconv.Itoa(p.Column))
	}

	want := []string{"X@1:7", "=@1:9", "A@1:11", "+@1:15", "'B C'@2:13", "+@2:19", "2@2:21"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q\nwant %q", got, want)
	}

	if p := s.Position(s.Offset); p.Column != 7 {
		t.Errorf("the statement starts at column %d", p.Column)
	}
}

// TestMessages checks the lexer's own messages and where they point.
func TestMessages(t *testing.T) {
	cases := []struct {
		text   string
		ident  string
		column int
	}{
		{text: "PRINT *, 'ABC", ident: "NOQUOTE", column: 16},
		{text: "X = 5HABC", ident: "HOLLEND", column: 11},
		{text: "X = A # B", ident: "BADCHAR", column: 13},
		{text: "FOO BAR", ident: "UNRECSTMT", column: 7},
		{text: "IF (X) FOO", ident: "UNRECSTMT", column: 14},
	}

	for _, c := range cases {
		var messages diag.List

		program := source.Read(source.Form{}, &source.File{Spec: "T.FOR", Lines: []string{fixed(c.text)}}, nil, &messages)
		Lex(program.Statements, &messages)

		ms := messages.Messages()
		if len(ms) != 1 || ms[0].Ident != c.ident || ms[0].Pos.Column != c.column {
			t.Errorf("%s: got %v, want %s at column %d", c.text, ms, c.ident, c.column)
		}
	}
}

// TestDirective checks that a CDEC$ directive is its own kind.
func TestDirective(t *testing.T) {
	_, dumps, idents := lexLines(t, "CDEC$ IDENT '1.0'", fixed("END"))

	want := []string{"CDEC$ directive:", "END:"}
	if len(idents) != 0 || !reflect.DeepEqual(dumps, want) {
		t.Errorf("got %q %v, want %q", dumps, idents, want)
	}
}

// TestKindTable checks that every kind has a name, and that no keyword
// statement's keyword is shadowed by a shorter one tried first.
func TestKindTable(t *testing.T) {
	for k := Kind(0); k < numKinds; k++ {
		if kinds[k].name == "" {
			t.Errorf("kind %d has no name", k)
		}
	}

	for i, k := range byKeyword {
		for _, earlier := range byKeyword[:i] {
			if len(kinds[earlier].keyword) < len(kinds[k].keyword) {
				t.Errorf("%s is tried before the longer %s", earlier, k)
			}
		}
	}
}

// TestFormatBody checks that a FORMAT statement, which isn't tokenized,
// says where its specification starts.
func TestFormatBody(t *testing.T) {
	statements, _, _ := lexLines(t, "  100 FORMAT (1X, F8.2)")

	s := statements[0]
	if s.Kind != Format || len(s.Tokens) != 0 {
		t.Fatalf("got %s", Dump(s))
	}

	if got := s.Source.Text[s.Body:]; got != "(1X, F8.2)" {
		t.Errorf("the specification is %q", got)
	}
}
