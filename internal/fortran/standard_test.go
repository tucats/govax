package fortran

import (
	"errors"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/fortran/diag"
	"github.com/tucats/govax/internal/fortran/sema"
	"github.com/tucats/govax/internal/fortran/source"
)

// extensionsOf is what a program compiled with the /STANDARD keywords flags, each
// finding its line and what it says. A compilation that fails is the test's failure.
func extensionsOf(t *testing.T, opts Options, keywords []string, lines ...string) []string {
	t.Helper()

	standard, err := sema.Conformance{}.With(keywords)
	if err != nil {
		t.Fatal(err)
	}

	opts.Standard = standard

	r := Compile(opts, File{Spec: "T.FOR", Name: "T", Lines: lines})

	if r.Errors != 0 {
		t.Fatalf("errors: %v", r.Messages)
	}

	var found []string

	for _, m := range r.Messages {
		if m.Ident == "EXTENSION" {
			found = append(found, strconv.Itoa(m.Pos.Line)+" "+strings.TrimPrefix(m.Text, "extension to FORTRAN-77: "))
		}
	}

	return found
}

// TestStandardSyntaxFlagged holds each kind of extension to one line of a program, so a
// case says what's flagged and nothing else on that line is.
func TestStandardSyntaxFlagged(t *testing.T) {
	cases := []struct {
		line string
		want string
	}{
		{"      BYTE B", "the BYTE statement"},
		{"      DOUBLE COMPLEX Z", "the DOUBLE COMPLEX statement"},
		{"      INTEGER*2 I", "a length after INTEGER"},
		{"      INTEGER I*2", "a length after the name I"},
		{"      INTEGER I/1/", "initial values in a type statement"},
		{"      IMPLICIT NONE", "IMPLICIT NONE"},
		{"      IMPLICIT REAL*8 (A-H)", "a length after REAL"},
		{"      VOLATILE I", "the VOLATILE statement"},
		{"      NAMELIST /N/ I", "the NAMELIST statement"},
		{"      LONGNAME = 1", "the name LONGNAME, longer than six characters"},
		{"      A$B = 1", "the name A$B, with a $ or _ in it"},
		{"      A_B = 1", "the name A_B, with a $ or _ in it"},
		{"      I = '17'O", "an octal or hexadecimal constant"},
		{"      I = '1F'X", "an octal or hexadecimal constant"},
		{"      I = \"17", "an octal constant with a quotation mark"},
		{"      I = 2HAB", "a Hollerith constant"},
		{"      X = 1.0Q0", "a REAL*16 constant"},
		{"      L = K .XOR. J", "the .XOR. operator"},
		{"      X = Y*-Z", "a sign straight after an operator"},
		{"      X = Y**-Z", "a sign straight after an operator"},
		{"      I = %LOC(J)", "the %LOC built-in function"},
		{"      CALL S(,1)", "a null argument"},
		{"      X = SIND(Y)", "the intrinsic function SIND"},
		{"      I = IAND(J, K)", "the intrinsic function IAND"},
		{"      DO I = 1, 2\n      END DO", "a DO statement without a label"},
		{"      DO WHILE (I .LT. 2)\n      END DO", "the DO WHILE statement"},
		{"      TYPE *, I", "the TYPE statement"},
		{"      ACCEPT *, I", "the ACCEPT statement"},
		{"      READ (1'5) I", "a record number after an apostrophe"},
		{"      OPEN (1, NAME='A')", "the NAME specifier"},
		{"      OPEN (1, READONLY)", "the READONLY specifier"},
		{"      OPEN (1, ACCESS='APPEND')", "ACCESS='APPEND'"},
		{"      CLOSE (1, STATUS='PRINT')", "STATUS='PRINT'"},
		{" 10   FORMAT (O4)", "the O edit descriptor"},
		{" 10   FORMAT (Z4)", "the Z edit descriptor"},
		{" 10   FORMAT (Q)", "the Q edit descriptor"},
		{" 10   FORMAT ('A',$)", "the $ edit descriptor"},
		{" 10   FORMAT (I)", "the I edit descriptor without a width"},
		{" 10   FORMAT (2(F))", "the F edit descriptor without a width"},
		{" 10   FORMAT (I<J>)", "a variable format expression"},
	}

	for _, c := range cases {
		got := extensionsOf(t, Options{}, []string{"SYNTAX"}, append(strings.Split(c.line, "\n"), "      END")...)
		want := []string{"1 " + c.want}

		if strings.Contains(c.line, "\n") && c.want == "a DO statement without a label" {
			want = []string{"1 a DO statement without a label", "2 the END DO statement"}
		}

		if c.want == "the DO WHILE statement" {
			want = []string{"1 the DO WHILE statement", "2 the END DO statement"}
		}

		if !slices.Equal(got, want) {
			t.Errorf("%q flags %q, want %q", c.line, got, want)
		}
	}
}

// TestStandardSyntaxQuiet is a program in FORTRAN-77 throughout, with the things that look
// most like extensions but aren't, which must flag nothing.
func TestStandardSyntaxQuiet(t *testing.T) {
	got := extensionsOf(t, Options{}, []string{"ALL"},
		"      PROGRAM SIXLET",
		"      IMPLICIT DOUBLE PRECISION (D)",
		"      CHARACTER*8 NAME, LINE*80, S*(*)",
		"      PARAMETER (S = 'AB', MAX = 3)",
		"      INTEGER A(MAX), IOS",
		"      COMPLEX Z",
		"      LOGICAL L",
		"      EXTERNAL F",
		"      INTRINSIC SIN, DSQRT",
		"      COMMON /BLK/ A",
		"      SAVE",
		"      DATA NAME /'X'/",
		"      Z = (1.0, 2.0)",
		"      L = .NOT. (A(1) .EQ. 2) .AND. .TRUE. .EQV. .FALSE.",
		"      LINE = NAME(1:2) // 'IT''S'",
		"      X = -SQRT(2.0) * ANINT(1.5) + LGE('A', 'B')",
		"      Y = (X*(-Z2)) ** 2",
		"      DO 10, I = 1, MAX",
		"        IF (I .GT. 2) THEN",
		"          WRITE (6, 100, ERR=20, IOSTAT=IOS) I, (A(J), J = 1, 2)",
		"        ELSE IF (I .LT. 1) THEN",
		"          PRINT *, I",
		"        ELSE",
		"          READ (5, *, END=20) K",
		"        END IF",
		" 10   CONTINUE",
		"      OPEN (1, FILE='A', STATUS='OLD', ACCESS='DIRECT',",
		"     1      FORM='FORMATTED', RECL=2, BLANK='NULL')",
		"      READ (1, 100, REC=2) I",
		"      INQUIRE (1, OPENED=L, NEXTREC=I)",
		"      REWIND 1",
		"      BACKSPACE (1, ERR=20)",
		"      ENDFILE 1",
		"      CLOSE (1, STATUS='KEEP')",
		" 20   GO TO (30, 30), I",
		" 30   ASSIGN 40 TO I",
		"      GO TO I",
		" 40   STOP 'DONE'",
		"100   FORMAT (I4, 1X, F6.2, E12.4E2, G8.2, L1, A, A3, 2HAB, 'X', BN, BZ,",
		"     1        S, SP, SS, T3, TL1, TR1, 1P, /, :)",
		"      END",
	)

	if len(got) != 0 {
		t.Errorf("flagged %q", got)
	}
}

// TestStandardSourceForm holds what's out of the standard in how lines are written.
func TestStandardSourceForm(t *testing.T) {
	continued := []string{"      I = 1"}
	for range 20 {
		continued = append(continued, "     1 + 1")
	}

	cases := []struct {
		name  string
		opts  Options
		lines []string
		want  []string
	}{
		{"nothing", Options{}, []string{"C     a comment in small letters", "* another", "", "      I = 'it''s'", "      J = 2Hab", "      END"}, nil},
		{"tab", Options{}, []string{"\tI = 1", "      END"}, []string{"1 a tab in the label or continuation field"}},
		{"tab label", Options{}, []string{"10\tI = 1", "      END"}, []string{"1 a tab in the label or continuation field"}},
		{"tab in field", Options{}, []string{"      I\t= 1", "      END"}, nil},
		{"small", Options{}, []string{"      i = 1", "      END"}, []string{"1 small letters"}},
		{"small each line", Options{}, []string{"      i = 1 +", "     1  j", "      END"}, []string{"1 small letters", "2 small letters"}},
		{"bang", Options{}, []string{"      I = 1 ! one", "      END"}, []string{"1 a comment after a statement"}},
		{"bang in a constant", Options{}, []string{"      PRINT *, 'WOW!'", "      END"}, nil},
		{"bang line", Options{}, []string{"! one", "      ! two", "      END"}, []string{"1 a comment starting with !", "2 a comment starting with !"}},
		{"debugging line", Options{}, []string{"D     I = 1", "      END"}, []string{"1 a debugging line"}},
		{"debugging line compiled", Options{DLines: true}, []string{"D     I = 1", "      END"}, []string{"1 a debugging line"}},
		{"past 72", Options{ExtendSource: true}, []string{"      I = 1" + strings.Repeat(" ", 70) + "+ 2", "      END"}, []string{"1 a statement past column 72"}},
		{"not past 72", Options{}, []string{"      I = 1" + strings.Repeat(" ", 70) + "+ 2", "      END"}, nil},
		{"continuations", Options{Continuations: 20}, append(continued, "      END"), []string{"21 more than 19 continuation lines"}},
	}

	for _, c := range cases {
		got := extensionsOf(t, c.opts, []string{"SOURCE_FORM"}, c.lines...)
		if !slices.Equal(got, c.want) {
			t.Errorf("%s: flags %q, want %q", c.name, got, c.want)
		}
	}
}

// TestStandardKeywords holds the keywords apart, and what's in a text library out.
func TestStandardKeywords(t *testing.T) {
	lines := []string{"      byte b", "      END"}
	syntax := []string{"1 the BYTE statement"}
	form := []string{"1 small letters"}
	both := []string{"1 small letters", "1 the BYTE statement"}

	for _, c := range []struct {
		keywords []string
		want     []string
	}{
		{[]string{"NONE"}, nil},
		{nil, both},
		{[]string{"ALL"}, both},
		{[]string{"SYNTAX"}, syntax},
		{[]string{"SOURCE_FORM"}, form},
		{[]string{"SYN", "SOU"}, both},
		{[]string{"ALL", "NOSYNTAX"}, form},
		{[]string{"ALL", "NOSOURCE_FORM"}, syntax},
	} {
		got := extensionsOf(t, Options{}, c.keywords, lines...)
		if !slices.Equal(got, c.want) {
			t.Errorf("%v: flags %q, want %q", c.keywords, got, c.want)
		}
	}

	if _, err := (sema.Conformance{}).With([]string{"SEMANTIC"}); err == nil {
		t.Error("SEMANTIC was taken as a keyword")
	}
}

// moduleOpener is an Opener with one text library module, whatever its name.
type moduleOpener []string

func (m moduleOpener) OpenModule(_, module string, _ *source.File) (*source.File, error) {
	return &source.File{Spec: module, Lines: m}, nil
}

func (moduleOpener) OpenInclude(name string, _ *source.File) (*source.File, error) {
	return nil, errors.New("no file " + name)
}

// TestStandardInclude flags the INCLUDE statement and what's in the file, but of a text
// library's module, nothing. Findings are informational, so nothing about them fails a
// compilation, and /WARNINGS=NOGENERAL doesn't hold back what /STANDARD asked for.
func TestStandardInclude(t *testing.T) {
	opts := Options{Includes: includes{"DEFS.FOR": {"      BYTE B"}}}

	got := extensionsOf(t, opts, []string{"SYNTAX"}, "      INCLUDE 'DEFS.FOR'", "      END")
	if want := []string{"1 the INCLUDE statement", "1 the BYTE statement"}; !slices.Equal(got, want) {
		t.Errorf("flags %q, want %q", got, want)
	}

	got = extensionsOf(t, Options{Includes: moduleOpener{"      BYTE B"}}, []string{"SYNTAX"}, "      INCLUDE '(MODULE)'", "      END")
	if want := []string{"1 the INCLUDE statement"}; !slices.Equal(got, want) {
		t.Errorf("a module flags %q, want %q", got, want)
	}

	quiet, _ := diag.Warnings{}.With([]string{"NOGENERAL"})
	opts.Warnings = quiet

	got = extensionsOf(t, opts, []string{"SYNTAX"}, "      BYTE B", "      END")
	if want := []string{"1 the BYTE statement"}; !slices.Equal(got, want) {
		t.Errorf("under NOGENERAL flags %q, want %q", got, want)
	}
}
