package parse

import (
	"flag"
	"fmt"
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

		// Headers.
		{"BLOCK DATA BLKDAT", "BLOCK DATA BLKDAT"},
		{"BLOCK DATA", "BLOCK DATA"},
		{"SUBROUTINE CHECK(X, Y, *, *, C)", "SUBROUTINE CHECK(X, Y, *, *, C)"},
		{"CHARACTER*(*) FUNCTION H(S)", "CHARACTER*(*) FUNCTION H(S)"},
		{"CHARACTER FUNCTION C*10(S)", "CHARACTER*10 FUNCTION C(S)"},
		{"DOUBLE PRECISION FUNCTION D(X)", "DOUBLE PRECISION FUNCTION D(X)"},
		{"COMPLEX*16 FUNCTION Z()", "COMPLEX*16 FUNCTION Z()"},
		{"FUNCTION F*8(X)", "*8 FUNCTION F(X)"},
		{"ENTRY E2(A, *)", "ENTRY E2(A, *)"},

		// Type declarations (section 4.4).
		{"INTEGER COUNT, MATRIX(4,4), SUM", "INTEGER COUNT, MATRIX(4, 4), SUM"},
		{"REAL*8 WX1, WX2, WX3*4, WX5, WX6*4", "REAL*8 WX1, WX2, WX3*4, WX5, WX6*4"},
		{"REAL*16 PI/3.14159Q0/, E/2.72Q0/, QARRAY(10)/5*0.0,5*1.0/",
			"REAL*16 PI /3.14159Q0/, E /2.72Q0/, QARRAY(10) /5*0.0, 5*1.0/"},
		{"BYTE B", "BYTE B"},
		{"DOUBLE COMPLEX Z(2)/2*(1.0D0, -1.0D0)/", "DOUBLE COMPLEX Z(2) /2*(complex 1.0D0 (- 1.0D0))/"},
		{"LOGICAL*1 L/.TRUE./, M/'10'X/, N/1HA/", "LOGICAL*1 L /.TRUE./, M /'10'X/, N /1HA/"},
		{"CHARACTER*32 NAMES(100),SOCSEC(100)*9,NAMETY*10/'ABCDEFGHIJ'/",
			"CHARACTER*32 NAMES(100), SOCSEC*9(100), NAMETY*10 /'ABCDEFGHIJ'/"},
		{"CHARACTER*(4+LENGTH) LAST, FIRST", "CHARACTER*((+ 4 LENGTH)) LAST, FIRST"},
		{"CHARACTER LETTER(26), BUBBLE*(*)", "CHARACTER LETTER(26), BUBBLE*(*)"},
		{"CHARACTER*8, A, B*4", "CHARACTER*8 A, B*4"},
		{"INTEGER I/-1/, J/N/, K/\"17/", "INTEGER I /(- 1)/, J /N/, K /\"17/"},
		{"DIMENSION BUD(12,24,10)", "DIMENSION BUD(12, 24, 10)"},
		{"DIMENSION A1(N1:N2), A2(N3:*)", "DIMENSION A1(N1:N2), A2(N3:*)"},
		{"VIRTUAL A(10)", "VIRTUAL A(10)"},

		// IMPLICIT and PARAMETER (sections 4.8, 4.11).
		{"IMPLICIT INTEGER (I,J,K,L,M,N)", "IMPLICIT INTEGER (I, J, K, L, M, N)"},
		{"IMPLICIT REAL (A-H, O-Z)", "IMPLICIT REAL (A-H, O-Z)"},
		{"IMPLICIT DOUBLE PRECISION (D)", "IMPLICIT DOUBLE PRECISION (D)"},
		{"IMPLICIT COMPLEX (S,Y), LOGICAL*1 (L,A-C)", "IMPLICIT COMPLEX (S, Y), LOGICAL*1 (L, A-C)"},
		{"IMPLICIT CHARACTER*32 (T-V)", "IMPLICIT CHARACTER*32 (T-V)"},
		{"IMPLICIT NONE", "IMPLICIT NONE"},
		{"PARAMETER (PI=3.1415927, DPI=3.141592653589793238D0)", "PARAMETER (PI = 3.1415927, DPI = 3.141592653589793238D0)"},
		{"PARAMETER (FLAG=.TRUE., LONGNAME='A STRING')", "PARAMETER (FLAG = .TRUE., LONGNAME = 'A STRING')"},

		// The other specification statements (chapter 4).
		{"COMMON HEAT,X /BLK1/KILO,Q", "COMMON // HEAT, X /BLK1/ KILO, Q"},
		{"COMMON /BLK1/LIMA,R / /ALFA,BET", "COMMON /BLK1/ LIMA, R // ALFA, BET"},
		{"COMMON /AREA1/R,S,T,U /AREA2/W,X,Y", "COMMON /AREA1/ R, S, T, U /AREA2/ W, X, Y"},
		{"COMMON X(M), Y(M), // Z", "COMMON // X(M), Y(M) // Z"},
		{"EQUIVALENCE (DVAR, IARR(1))", "EQUIVALENCE (DVAR, IARR(1))"},
		{"EQUIVALENCE (TABLE(2,2), TRIPLE(1,2,2)), (A, B)", "EQUIVALENCE (TABLE(2, 2), TRIPLE(1, 2, 2)), (A, B)"},
		{"EQUIVALENCE (NAME(10:13), ID(2:5))", "EQUIVALENCE (NAME(10:13), ID(2:5))"},
		{"EQUIVALENCE (FIELDS(1)(2:4), STAR(2)(3:5))", "EQUIVALENCE (FIELDS(1)(2:4), STAR(2)(3:5))"},
		{"DATA A,STARS /10*0,'****'/", "DATA A, STARS /10*0, '****'/"},
		{"DATA BELL,TAB,LF,FF /7,9,10,12/", "DATA BELL, TAB, LF, FF /7, 9, 10, 12/"},
		{"DATA (B(I), I=1,10,2) /5*1/", "DATA (do B(I); I = 1, 10, 2) /5*1/"},
		{"DATA R/1.0,2*2.0/, T/.FALSE./, U/0.214537D-7/", "DATA R /1.0, 2*2.0/ T /.FALSE./ U /0.214537D-7/"},
		{"DATA ((M(I,J), I=1,J), J=1,3) /6*0/", "DATA (do (do M(I, J); I = 1, J); J = 1, 3) /6*0/"},
		{"DATA X/4HABCD/ C(2:3)/'XY'/", "DATA X /4HABCD/ C(2:3) /'XY'/"},
		{"SAVE", "SAVE"},
		{"SAVE A, /BLK/, B", "SAVE A, /BLK/, B"},
		{"EXTERNAL CTN, SIN", "EXTERNAL CTN, SIN"},
		{"INTRINSIC SIN, COS", "INTRINSIC SIN, COS"},
		{"VOLATILE /BLK1/,D,E", "VOLATILE /BLK1/, D, E"},
		{"AUTOMATIC A, B", "AUTOMATIC A, B"},
		{"STATIC C", "STATIC C"},
		{"NAMELIST /INPUT/ NAME, GRADE, DATE /OUTPUT/ TOTAL, NAME", "NAMELIST /INPUT/ NAME, GRADE, DATE /OUTPUT/ TOTAL, NAME"},
		{"NAMELIST /A/ X, /B/ Y", "NAMELIST /A/ X /B/ Y"},
		{"RECORD /DATE/ TODAY,YESTERDAY", "RECORD /DATE/ TODAY, YESTERDAY"},
		{"RECORD /CHECK/ CURRENT_CHECK, CHECKBOOK(1000), /DATE/ D", "RECORD /CHECK/ CURRENT_CHECK, CHECKBOOK(1000), /DATE/ D"},
		{"DICTIONARY 'CDD$TOP.SALES.JONES.SALARY'", "DICTIONARY 'CDD$TOP.SALES.JONES.SALARY'"},

		// Compilation control.
		{"FORMAT (1X, I5, 'X=', F8.2)", "FORMAT (1X, I5, 'X=', F8.2)"},
		{"CDEC$ IDENT '1.0'", "CDEC$ IDENT '1.0'"},
		{"CDEC$ TITLE", "CDEC$ TITLE"},
		{"CDEC$ PSECT /COM1/ NOWRT, ALIGN=3", "CDEC$ PSECT /COM1/ NOWRT, ALIGN=3"},
		{"CDEC$ PSECT // GBL", "CDEC$ PSECT // GBL"},

		// Assignment and control (chapters 3 and 5).
		{"ASSIGN 10 TO NSTART", "ASSIGN 10 TO NSTART"},
		{"ASSIGN 99999 TO KSTOP", "ASSIGN 99999 TO KSTOP"},
		{"GO TO (12,24,36), INDEX", "GO TO (12, 24, 36) INDEX"},
		{"GO TO (320,330,340,350) ITEM", "GO TO (320, 330, 340, 350) ITEM"},
		{"GO TO NEXT,(10,20,30)", "GO TO NEXT (10, 20, 30)"},
		{"GO TO NEXT (10, 20)", "GO TO NEXT (10, 20)"},
		{"GO TO NEXT", "GO TO NEXT"},
		{"IF (THETA-CHI) 50,50,100", "IF (- THETA CHI) 50, 50, 100"},
		{"IF (X) READ *, Y", "IF X READ * Y"},
		{"DO I = 1, 10", "DO I = 1, 10"},
		{"DO WHILE (X .LT. 5)", "DO WHILE (.LT. X 5)"},
		{"DO 10, WHILE (I .GT. J)", "DO 10 WHILE (.GT. I J)"},
		{"END DO", "END DO"},
		{"PAUSE", "PAUSE"},
		{"PAUSE 'MOUNT TAPE'", "PAUSE 'MOUNT TAPE'"},
		{"CALL S(*10, &20, %VAL(N), %REF(A), %DESCR(C))", "CALL S(*10, *20, %VAL(N), %REF(A), %DESCR(C))"},
		{"X = %LOC(A)", "X = %LOC(A)"},
		{"X = 5HHELLO + '17'O + '1F'X + \"17", "X = (+ (+ (+ 5HHELLO '17'O) '1F'X) \"17)"},
		{"X = (1.0, 2.0)", "X = (complex 1.0 2.0)"},
		{"C(1:3) = 'ABC'", "C(1:3) = 'ABC'"},
		{"C(:I) = C(2:)", "C(:I) = C(2:)"},
		{"C(I)(1:3) = 'ABC'", "C(I)(1:3) = 'ABC'"},
		{"IF (C(I)(1:1) .EQ. 'X') GO TO 10", "IF (.EQ. C(I)(1:1) 'X') GO TO 10"},
		{"APP.APP_TIME(1).HOUR = 2", "APP.APP_TIME(1).HOUR = 2"},
		{"CHECKBOOK(5).AMOUNT = X", "CHECKBOOK(5).AMOUNT = X"},
		{"R.NAME(1:3) = 'ABC'", "R.NAME(1:3) = 'ABC'"},

		// Input and output (chapters 7 and 9, appendix A).
		{"READ *, X", "READ * X"},
		{"READ 100, A, B", "READ 100 A B"},
		{"READ (5, 100, ERR=50, END=60, IOSTAT=IOS) J, K", "READ (5, 100, ERR=50, END=60, IOSTAT=IOS) J K"},
		{"READ (UNIT=1, FMT='(I5)', REC=N) X", "READ (1, '(I5)', REC=N) X"},
		{"READ (1'10) X", "READ (1, REC=10) X"},
		{"READ (1'10, 20) X", "READ (1, 20, REC=10) X"},
		{"READ (5, NML=INPUT)", "READ (5, NML=INPUT)"},
		{"READ (5, INPUT)", "READ (5, INPUT)"},
		{"READ (3, KEYGE=K, KEYID=1) R", "READ (3, KEYGE=K, KEYID=1) R"},
		{"READ (BUF, *) A, (B(I), I=1,N)", "READ (BUF, *) A (do B(I); I = 1, N)"},
		{"READ INPUT", "READ INPUT"},
		{"WRITE (6, 10) J, K(3), 4, (L+4)/2, N", "WRITE (6, 10) J K(3) 4 (/ (paren (+ L 4)) 2) N"},
		{"WRITE (*, *, ERR=10) X", "WRITE (*, *, ERR=10) X"},
		{"WRITE (1'10) X", "WRITE (1, REC=10) X"},
		{"PRINT *, ((A(I,J), I=1,2), J=1,3)", "PRINT * (do (do A(I, J); I = 1, 2); J = 1, 3)"},
		{"REWRITE (1, 10) X", "REWRITE (1, 10) X"},
		{"ACCEPT *, X", "ACCEPT * X"},
		{"ACCEPT 10, X, Y", "ACCEPT 10 X Y"},
		{"TYPE *, 'X'", "TYPE * 'X'"},
		{"TYPE OUTPUT", "TYPE OUTPUT"},
		{"PRINT NL", "PRINT NL"},
		{"ENCODE (12,100,B) K(3), K(2), K(1)", "ENCODE (12, 100, B) K(3) K(2) K(1)"},
		{"DECODE (12,100,A,IOSTAT=I,ERR=9) K", "DECODE (12, 100, A, IOSTAT=I, ERR=9) K"},
		{"OPEN (UNIT=1, FILE='X.DAT', STATUS='OLD', READONLY, SHARED)", "OPEN (1, FILE='X.DAT', STATUS='OLD', READONLY, SHARED)"},
		{"OPEN (2, FILE=F, KEY=(1:10:CHARACTER, 11:14:INTEGER:DESCENDING))",
			"OPEN (2, FILE=F, KEY=(keys 1:10:CHARACTER, 11:14:INTEGER:DESCENDING))"},
		{"OPEN (FILE='X', USEROPEN=MYOPEN, TYPE='NEW')", "OPEN (FILE='X', USEROPEN=MYOPEN, TYPE='NEW')"},
		{"CLOSE (UNIT=1, STATUS='DELETE', ERR=10)", "CLOSE (1, STATUS='DELETE', ERR=10)"},
		{"INQUIRE (FILE='X', EXIST=EX, NUMBER=N)", "INQUIRE (FILE='X', EXIST=EX, NUMBER=N)"},
		{"INQUIRE (3, OPENED=OP)", "INQUIRE (3, OPENED=OP)"},
		{"REWIND 5", "REWIND (5)"},
		{"REWIND (UNIT=5, ERR=10)", "REWIND (5, ERR=10)"},
		{"BACKSPACE 3", "BACKSPACE (3)"},
		{"END FILE 2", "ENDFILE (2)"},
		{"ENDFILE (2, IOSTAT=I)", "ENDFILE (2, IOSTAT=I)"},
		{"UNLOCK 4", "UNLOCK (4)"},
		{"DELETE (1'N)", "DELETE (1, REC=N)"},
		{"DELETE (UNIT=1, REC=N, ERR=10)", "DELETE (1, REC=N, ERR=10)"},
		{"FIND (4'INDX)", "FIND (4, REC=INDX)"},
		{"FIND (UNIT=4, REC=1)", "FIND (4, REC=1)"},
		{"DEFINE FILE 3 (1000,48,U,NREC)", "DEFINE FILE 3(1000, 48, U, NREC)"},
		{"DEFINE FILE LUN(100,10,U,NR), 4(10,2,U,N4)", "DEFINE FILE LUN(100, 10, U, NR), 4(10, 2, U, N4)"},
	}

	for _, c := range cases {
		var lines []string

		text := fixed(c.text)
		if strings.HasPrefix(c.text, "CDEC$") {
			text = c.text
		}

		switch {
		case strings.Contains(c.text, "FUNCTION"), strings.HasPrefix(c.text, "SUBROUTINE"),
			strings.HasPrefix(c.text, "PROGRAM"), strings.HasPrefix(c.text, "BLOCK DATA"):
			lines = []string{text, fixed("END")}
		case strings.HasPrefix(c.text, "FORMAT"):
			lines = []string{fixed("CONTINUE"), "   10 " + c.text, fixed("END")}
		default:
			lines = []string{fixed("CONTINUE"), text, fixed("END")}
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
		{"IF (X .GT. 0", "SYNTAX", 19, ""},
		{"IF (X)", "SYNTAX", 13, "expected a statement after the condition, but found the end of the statement"},
		{"IF (X) DO 10 I = 1, 5", "BADIFSTMT", 14, "a logical IF can't execute a DO statement"},
		{"IF (X) IF (Y) Z = 1", "BADIFSTMT", 14, "a logical IF can't execute another logical IF"},
		{"IF (X) INTEGER I", "BADIFSTMT", 14, "a logical IF can't execute an INTEGER statement"},
		{"IF (X) END IF", "BADIFSTMT", 14, ""},
		{"IF (X .GT. 0)", "SYNTAX", 20, ""},
		{"ELSE IF (X)", "SYNTAX", 18, "expected 'THEN', but found the end of the statement"},
		{"STOP 123456", "SYNTAX", 12, "expected a character constant or 1 to 5 digits, but found the number 123456"},
		{"STOP X", "SYNTAX", 12, ""},
		{"CALL S(A", "SYNTAX", 15, ""},
		{"DIMENSION A", "SYNTAX", 18, "expected '(', but found the end of the statement"},
		{"PRINT * X", "SYNTAX", 15, "expected ',', but found the name X"},
		{"WRITE (FMT=*) X", "SYNTAX", 19, "expected a unit (UNIT=), but found ')'"},
		{"WRITE (*, *, *) X", "SYNTAX", 20, ""},
		{"WRITE (BAD=1) X", "BADSPECIFIER", 14, "BAD= is not a specifier of the WRITE statement"},
		{"WRITE (6, END=10) X", "BADSPECIFIER", 17, ""},
		{"READ (5, *, ERR=10, ERR=20) X", "DUPSPECIFIER", 27, "the ERR= specifier is given twice"},
		{"READ (5, UNIT=6) X", "DUPSPECIFIER", 16, ""},
		{"READ (5, *, ERR=X) Y", "SYNTAX", 23, "expected a statement label, but found the name X"},
		{"CLOSE (STATUS='KEEP')", "SYNTAX", 27, "expected a unit (UNIT=), but found ')'"},
		{"OPEN (1, READONLY=1)", "SYNTAX", 24, ""},
		{"ENCODE (8, 10, B, END=20) X", "BADSPECIFIER", 25, ""},
		{"DEFINE FILE 3(100, 10, F, N)", "SYNTAX", 30, "expected U, for an unformatted file, but found the name F"},
		{"ASSIGN 10 I", "SYNTAX", 17, "expected TO and an integer variable, but found the name I"},
		{"IMPLICIT REAL (Z-A)", "BADLETTERS", 22, "Z-A is not a range of letters in alphabetical order"},
		{"IMPLICIT REAL (AB)", "SYNTAX", 22, "expected a letter, but found the name AB"},
		{"IMPLICIT FOO (A)", "SYNTAX", 16, "expected a data type, but found the name FOO"},
		{"EXTERNAL F, *TAN", "NOF77", 19, "the form EXTERNAL *TAN is appendix A.6's, for /NOF77, which govax doesn't have"},
		{"INTEGER*(4) I", "SYNTAX", 15, "expected a length in bytes, but found '('"},
		{"CHARACTER A*4(10)", "EXTRATEXT", 20, ""},
		{"INTEGER A(10)/1, 2", "SYNTAX", 25, "expected '/', but found the end of the statement"},
		{"DATA A/1/, B", "SYNTAX", 19, "expected '/', but found the end of the statement"},
		{"COMMON /B, X", "SYNTAX", 16, "expected '/', but found ','"},
		{"SAVE /B/, C(1)", "EXTRATEXT", 18, ""},
		{"INTRINSIC /B/", "SYNTAX", 17, "expected a name, but found '/'"},
		{"PRINT *, %FOO(X)", "SYNTAX", 17, "expected LOC, VAL, REF, or DESCR, but found the name FOO"},
		{"GO TO (10, 20)", "SYNTAX", 21, "expected an expression, but found the end of the statement"},
		{"IF (X) 10, 20", "SYNTAX", 20, "expected ',', but found the end of the statement"},
		{"DO 10 WHILE X", "SYNTAX", 20, "expected '=', but found the end of the statement"},
		{"INTEGER %FILL", "FILLNAME", 15, ""},
		{"END MAP", "NOBLOCK", 7, "this END MAP statement ends no MAP declaration"},
		{"UNION", "NOTINBLOCK", 7, "a UNION declaration can only be in a STRUCTURE declaration"},
		{"OPTIONS G_FLOATING", "SYNTAX", 15, "expected '/', but found the name G_FLOATING"},
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

		if _, ok := u.Body[1].(*ast.BadStmt); !ok {
			t.Errorf("%s: kept as %T", c.text, u.Body[1])
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

	if strings.Join(idents(ms), " ") != "NOEND NOEND" {
		t.Fatalf("messages: %v", ms)
	}

	if ms[0].Text != "SUBROUTINE S has no END statement" || ms[0].Pos.Line != 4 || ms[0].Pos.Column != 13 {
		t.Errorf("first NOEND: %v", ms[0])
	}

	if ms[1].Text != "the main program LATE has no END statement" || ms[1].Pos.Line != 11 {
		t.Errorf("second NOEND: %v", ms[1])
	}

	if _, ok := file.Units[0].Body[1].(*ast.EndStmt); !ok || file.Units[0].Header != nil {
		t.Errorf("first unit: %s", ast.Dump(file))
	}
}

// TestNoStatements checks the things that are in no unit's body: a file
// of comments, a directive or OPTIONS between units (the next unit's
// prologue, or the file's trailing statements), and a label with no
// statement.
func TestNoStatements(t *testing.T) {
	file, ms := parseLines(t, "N.FOR", "C     nothing")
	if len(file.Units) != 0 || len(ms) != 0 {
		t.Errorf("a file of comments: %d units, %v", len(file.Units), ms)
	}

	file, ms = parseLines(t, "N.FOR", "CDEC$ IDENT 'X'", fixed("OPTIONS /G_FLOATING"), "   10", fixed("END"),
		"CDEC$ TITLE 'T'")
	if strings.Join(idents(ms), " ") != "EMPTYSTMT" || len(file.Units) != 1 || len(file.Units[0].Body) != 1 ||
		len(file.Units[0].Prologue) != 2 || len(file.Trailing) != 1 {
		t.Errorf("directives and an empty statement: %v %s", ms, ast.Dump(file))
	}

	if ms[0].Text != "statement label 10 is on no statement" || ms[0].Pos.Column != 4 {
		t.Errorf("EMPTYSTMT: %v", ms[0])
	}

	// OPTIONS before a header doesn't stop the lexer finding a typed
	// FUNCTION statement.
	file, ms = parseLines(t, "N.FOR", fixed("OPTIONS /CHECK=(BOUNDS, NOOVERFLOW) /NOI4"), fixed("REAL FUNCTION F(X)"),
		fixed("END"))
	if len(ms) != 0 || len(file.Units) != 1 || file.Units[0].Kind() != ast.FunctionUnit {
		t.Errorf("OPTIONS and a FUNCTION: %v %s", ms, ast.Dump(file))
	}

	if got := ast.DumpStmt(file.Units[0].Prologue[0]); got != "OPTIONS /CHECK=(BOUNDS, NOOVERFLOW) /NOI4" {
		t.Errorf("OPTIONS: %s", got)
	}
}

// TestBlocks checks structure declarations: their statements gathered
// into them, nesting, and the messages about their ends and contents.
func TestBlocks(t *testing.T) {
	file, ms := parseLines(t, "B.FOR",
		fixed("STRUCTURE /APPOINTMENT/"),
		fixed("  RECORD /DATE/ APP_DATE"),
		fixed("  STRUCTURE /TIME/ APP_TIME (2)"),
		fixed("    LOGICAL*1 HOUR, MINUTE"),
		fixed("  END STRUCTURE"),
		fixed("  CHARACTER*20 APP_MEMO (4)"),
		fixed("  UNION"),
		fixed("    MAP"),
		fixed("      INTEGER*2 WORD_0, %FILL"),
		fixed("    END MAP"),
		fixed("    MAP"),
		fixed("      INTEGER*4 LONG /0/"),
		fixed("    END MAP"),
		fixed("  END UNION"),
		fixed("END STRUCTURE"),
		fixed("X = 1"),
		fixed("END"),
	)

	want := `== main program (no PROGRAM statement)
1           STRUCTURE /APPOINTMENT/
2             RECORD /DATE/ APP_DATE
3             STRUCTURE /TIME/ APP_TIME(2)
4               LOGICAL*1 HOUR, MINUTE
5               END STRUCTURE
6             CHARACTER*20 APP_MEMO(4)
7             UNION
8               MAP
9                 INTEGER*2 WORD_0, %FILL
10                END MAP
11              MAP
12                INTEGER*4 LONG /0/
13                END MAP
14              END UNION
15            END STRUCTURE
16          X = 1
17          END
`

	if got := ast.Dump(file); len(ms) != 0 || got != want {
		t.Errorf("messages %v, dump:\n%s\nwant:\n%s", ms, got, want)
	}

	cases := []struct {
		lines []string
		want  string
	}{
		{[]string{"STRUCTURE /S/", "INTEGER I", "END"}, "NOBLOCKEND@1:7"},
		{[]string{"STRUCTURE /S/", "UNION", "MAP", "END STRUCTURE", "END"}, "NOBLOCKEND@3:7 NOBLOCKEND@2:7"},
		{[]string{"STRUCTURE /S/", "MAP", "END MAP", "END STRUCTURE", "END"}, "NOTINBLOCK@2:7"},
		{[]string{"STRUCTURE /S/", "UNION", "INTEGER I", "END UNION", "END STRUCTURE", "END"}, "NOTINBLOCK@3:7"},
		{[]string{"STRUCTURE /S/", "UNION", "UNION", "END UNION", "END UNION", "END STRUCTURE", "END"}, "NOTINBLOCK@3:7"},
		{[]string{"STRUCTURE /S/", "X = 1", "END STRUCTURE", "END"}, "FIELDSTMT@2:7"},
		{[]string{"STRUCTURE /S/", "END UNION", "END STRUCTURE", "END"}, "NOBLOCK@2:7"},
		{[]string{"STRUCTURE /S/", "PARAMETER (N = 2)", "INTEGER A(N)", "END STRUCTURE", "END"}, ""},
		{[]string{"MAP", "INTEGER I", "END MAP", "END"}, "NOTINBLOCK@1:7"},
		{[]string{"STRUCTURE /S/", "INTEGER I", "SUBROUTINE T", "END"}, "NOBLOCKEND@1:7 NOEND@2:16"},
	}

	for _, c := range cases {
		lines := make([]string, len(c.lines))
		for i, l := range c.lines {
			lines[i] = fixed(l)
		}

		_, ms := parseLines(t, "B.FOR", lines...)

		var got []string
		for _, m := range ms {
			got = append(got, fmt.Sprintf("%s@%d:%d", m.Ident, m.Pos.Line, m.Pos.Column))
		}

		if g := strings.Join(got, " "); g != c.want {
			t.Errorf("%q: got %q, want %q", c.lines, g, c.want)
		}
	}
}

// TestEveryKindParses checks that every kind of statement the lexer
// knows has a parser: no statement of VAX FORTRAN is left to a "not
// implemented" from the parser.
func TestEveryKindParses(t *testing.T) {
	for k := lex.Kind(0); k.String() != "unknown kind"; k++ {
		if k == lex.Unknown || k == lex.Empty {
			continue
		}

		if parsers[k] == nil {
			t.Errorf("no parser for %s", k)
		}
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
