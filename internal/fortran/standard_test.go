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

// judgedExtensions is what a program compiled with the /STANDARD keywords flags, each
// finding its line and what it says after the standard's name: free-form text with /F90
// if free, else fixed-form under /NOF90. A compilation that fails is the test's failure.
func judgedExtensions(t *testing.T, keywords []string, free bool, lines ...string) []string {
	t.Helper()

	standard, err := sema.Conformance{}.With(keywords)
	if err != nil {
		t.Fatal(err)
	}

	opts := Options{Standard: standard}
	file := File{Spec: "T.FOR", Name: "T", Lines: lines}

	if free {
		opts.F90, opts.F90Given = true, true
		file = File{Spec: "T.F90", Name: "T", Lines: lines, Free: true}
	}

	r := Compile(opts, file)

	if r.Errors != 0 {
		t.Fatalf("errors: %v", r.Messages)
	}

	var found []string

	for _, m := range r.Messages {
		if m.Ident == "EXTENSION" {
			_, what, _ := strings.Cut(m.Text, ": ")
			found = append(found, strconv.Itoa(m.Pos.Line)+" "+what)
		}
	}

	return found
}

// TestStandardF90Flagged holds what Fortran 90 doesn't have, each case a free-form
// program's middle lines, whose findings are on line 2.
func TestStandardF90Flagged(t *testing.T) {
	cases := []struct {
		lines []string
		want  []string
	}{
		{[]string{"byte b"}, []string{"the BYTE statement"}},
		{[]string{"integer*2 i"}, []string{"a length after INTEGER"}},
		{[]string{"integer i/1/"}, []string{"initial values in a type statement"}},
		{[]string{"a$b = 1"}, []string{"the name A$B, with a $ in it"}},
		{[]string{"i = '1F'X"}, []string{"an octal or hexadecimal constant"}},
		{[]string{"i = Z'1F'"}, []string{"a binary, octal, or hexadecimal constant outside DATA"}},
		{[]string{"i = 2HAB"}, []string{"a Hollerith constant"}},
		{[]string{"x = 1.0Q0"}, []string{"a REAL*16 constant"}},
		{[]string{"l = k .xor. j"}, []string{"the .XOR. operator"}},
		{[]string{"x = y*-z"}, []string{"a sign straight after an operator"}},
		{[]string{"i = %loc(j)"}, []string{"the %LOC built-in function"}},
		{[]string{"x = sind(y)"}, []string{"the intrinsic function SIND"}},
		{[]string{"i = jnint(x)"}, []string{"the intrinsic function JNINT"}},
		{[]string{"type *, i"}, []string{"the TYPE statement"}},
		{[]string{"open (1, name='A')"}, []string{"the NAME specifier"}},
		{[]string{"open (1, file='A', access='APPEND')"}, []string{"ACCESS='APPEND'"}},
		{[]string{"read (1'5) i"}, []string{"a record number after an apostrophe"}},
		{[]string{"print 10, i", "10 format (q, i4, $)"}, []string{"the Q edit descriptor", "the $ edit descriptor"}},
		{[]string{"print 10, i", "10 format (i)"}, []string{"the I edit descriptor without a width"}},
		{[]string{"structure /s/", "integer i", "end structure"}, []string{"the STRUCTURE statement"}},
		{[]string{"type :: p", "real*8 x", "end type"}, []string{"a length after REAL"}},
	}

	for _, c := range cases {
		lines := append(append([]string{"program t"}, c.lines...), "end")

		var want []string
		for _, w := range c.want {
			want = append(want, "2 "+w)
		}

		if len(c.lines) > 1 && strings.HasPrefix(c.lines[1], "10") {
			want = nil

			for _, w := range c.want {
				want = append(want, "3 "+w)
			}
		}

		if strings.HasPrefix(c.lines[0], "type ::") {
			want = []string{"3 " + c.want[0]}
		}

		got := judgedExtensions(t, []string{"F90"}, true, lines...)
		if !slices.Equal(got, want) {
			t.Errorf("%q: flags %q, want %q", c.lines, got, want)
		}
	}
}

// TestStandardF90Quiet is a program in Fortran 90 throughout, with the things it adopted
// from DEC (NAMELIST, DO WHILE, END DO, IMPLICIT NONE, INCLUDE, ! comments, long names)
// and everything it adds, which must flag nothing.
func TestStandardF90Quiet(t *testing.T) {
	opts := Options{Includes: includes{"DEFS.F90": {"      INTEGER, PARAMETER :: DP = KIND(1.0D0)"}}}
	standard, _ := sema.Conformance{}.With([]string{"F90"})
	opts.Standard = standard
	opts.F90, opts.F90Given = true, true

	lines := []string{
		"module shapes",
		"  implicit none",
		"  include 'DEFS.F90'",
		"  type :: point",
		"    real(dp) :: x, y",
		"  end type point",
		"  interface operator (.plus.)",
		"    module procedure add_points",
		"  end interface",
		"  private :: hidden",
		"  integer :: hidden",
		"contains",
		"  function add_points(a, b) result(c)",
		"    type(point), intent(in) :: a, b",
		"    type(point) :: c",
		"    c = point(a%x + b%x, a%y + b%y)",
		"  end function add_points",
		"end module shapes",
		"recursive subroutine count_down(n)",
		"  integer, intent(in) :: n",
		"  if (n > 0) call count_down(n - 1)",
		"end subroutine count_down",
		"program long_name_program",
		"  use shapes",
		"  implicit none",
		"  real, allocatable :: values(:)",
		"  real, target :: t(4)",
		"  real, pointer :: p(:)",
		"  integer :: i, j, k(2) = (/ 1, 2 /), mask",
		"  real(dp) :: z = 0.0_dp",
		"  character(len=8) :: word = \"it's\"",
		"  type(point) :: q",
		"  logical :: l",
		"  namelist /nml/ i, j",
		"  data mask /Z'0F'/",
		"  allocate (values(4)); values = 1.0",
		"  p => t(1:4:2)",
		"  t(k) = 2.0",
		"  q = point(1.0, 2.0) .plus. point(3.0, 4.0)",
		"  l = i == j .or. i /= j .and. i < j .or. i <= j .or. i > j .or. i >= j",
		"  outer: do i = 1, 3",
		"    do while (j < 2)",
		"      j = j + 1",
		"      if (j == 2) exit outer",
		"    end do",
		"    cycle outer",
		"  end do outer",
		"  select case (i)",
		"  case (1:2)",
		"    j = 1",
		"  case default",
		"    j = 2",
		"  end select",
		"  where (values > 0.0) values = sqrt(values)",
		"  print '(B8, O4, Z4, EN12.3, ES12.3, 2/)', i, i, i, values(1), values(2)",
		"  write (*, '(A)', advance='NO') trim(word)",
		"  open (1, file='X', status='REPLACE', position='APPEND', action='WRITE', delim='QUOTE', pad='YES')",
		"  i = size(values) + lbound(values, dim=1) + selected_int_kind(4) ! a comment",
		"  deallocate (values)",
		"  nullify (p)",
		"end program long_name_program",
	}

	r := Compile(opts, File{Spec: "T.F90", Name: "T", Lines: lines, Free: true})
	if r.Errors != 0 {
		t.Fatalf("errors: %v", r.Messages)
	}

	for _, m := range r.Messages {
		if m.Ident == "EXTENSION" {
			t.Errorf("line %d: %s", m.Pos.Line, m.Text)
		}
	}
}

// TestStandardF77SeesF90 holds FORTRAN-77's reading of Fortran 90's forms: each is an
// extension to FORTRAN-77, though Fortran 90 has it.
func TestStandardF77SeesF90(t *testing.T) {
	cases := []struct {
		line string
		want string
	}{
		{"integer :: n", "a declaration with ::"},
		{"integer, save :: n", "the SAVE attribute"},
		{"real(8) y", "a kind or length selector"},
		{"integer :: n = 1", "an initial value after ="},
		{"x = 1.0_8", "a constant with a kind parameter"},
		{"l = i == j", "the == operator"},
		{"c = \"a\"", "a character constant in quotation marks"},
		{"v = (/ 1.0, 2.0 /)", "an array constructor"},
		{"i = int(x, kind=2)", "a keyword argument"},
		{"i = size(v)", "the intrinsic function SIZE"},
		{"i = len_trim(c)", "the intrinsic function LEN_TRIM"},
		{"allocate (a(2))", "the ALLOCATE statement"},
		{"do", "a DO statement without loop control"},
		{"where (v > 0) v = 1", "the WHERE statement"},
	}

	for _, c := range cases {
		lines := []string{"program t", "real :: x, v(2)", "real, allocatable :: a(:)", "logical :: l",
			"character :: c", c.line}
		if c.line == "do" {
			lines = append(lines, "exit", "end do")
		}

		lines = append(lines, "end")

		got := judgedExtensions(t, []string{"F77", "SYNTAX"}, true, lines...)
		if !slices.Contains(got, "6 "+c.want) {
			t.Errorf("%s: flags %q, want %q", c.line, got, "6 "+c.want)
		}
	}
}

// TestStandardF90SourceForm holds what Fortran 90's section 3 forbids in how lines are
// written, and what it allows that FORTRAN-77 doesn't.
func TestStandardF90SourceForm(t *testing.T) {
	continued := []string{"i = 1 &"}
	for range 40 {
		continued = append(continued, "  + 1 &")
	}

	continued = append(continued, "  + 1", "end")

	cases := []struct {
		name  string
		free  bool
		lines []string
		want  []string
	}{
		{"small letters and !", false, []string{"      i = 1 ! one", "! two", "      end"}, nil},
		{"tab format", false, []string{"\tI = 1", "      END"}, []string{"1 a tab in the label or continuation field"}},
		{"tab in field", false, []string{"      I\t= 1", "      END"}, []string{"1 a tab"}},
		{"debugging line", false, []string{"D     I = 1", "      END"}, []string{"1 a debugging line"}},
		{"free form", true, []string{"i = 1 ! one", "! two", "end"}, nil},
		{"free tab", true, []string{"i =\t1", "end"}, []string{"1 a tab"}},
		{"free long comment", true, []string{"! " + strings.Repeat("x", 140), "end"}, []string{"1 a line longer than 132 characters"}},
		{"free continuations", true, continued, []string{"41 more than 39 continuation lines"}},
	}

	for _, c := range cases {
		keywords := []string{"F90", "SOURCE_FORM"}

		var got []string

		if c.name == "free continuations" {
			standard, _ := sema.Conformance{}.With(keywords)
			r := Compile(Options{Standard: standard, F90: true, F90Given: true, Continuations: 99},
				File{Spec: "T.F90", Name: "T", Lines: c.lines, Free: true})

			for _, m := range r.Messages {
				if m.Ident == "EXTENSION" {
					_, what, _ := strings.Cut(m.Text, ": ")
					got = append(got, strconv.Itoa(m.Pos.Line)+" "+what)
				}
			}
		} else {
			got = judgedExtensions(t, keywords, c.free, c.lines...)
		}

		if !slices.Equal(got, c.want) {
			t.Errorf("%s: flags %q, want %q", c.name, got, c.want)
		}
	}

	// FORTRAN-77's reading flags a free-form file once.
	got := judgedExtensions(t, []string{"F77", "SOURCE_FORM"}, true, "i = 1 ! one", "end")
	if want := []string{"1 free source form"}; !slices.Equal(got, want) {
		t.Errorf("free form under F77 flags %q, want %q", got, want)
	}
}

// TestStandardWhichStandard holds which standard a unit is judged by: the one named, or
// with none named its /F90 setting (the author's answer 2), and the message names it.
func TestStandardWhichStandard(t *testing.T) {
	compile := func(keywords []string, free bool, lines ...string) []string {
		standard, err := sema.Conformance{}.With(keywords)
		if err != nil {
			t.Fatal(err)
		}

		opts := Options{Standard: standard}
		file := File{Spec: "T.FOR", Name: "T", Lines: lines}

		if free {
			opts.F90, opts.F90Given = true, true
			file = File{Spec: "T.F90", Name: "T", Lines: lines, Free: true}
		}

		var texts []string

		for _, m := range Compile(opts, file).Messages {
			if m.Ident == "EXTENSION" {
				texts = append(texts, m.Text)
			}
		}

		return texts
	}

	free := []string{"implicit none", "end"}
	fixed := []string{"      IMPLICIT NONE", "      END"}
	f77 := []string{"extension to FORTRAN-77: IMPLICIT NONE"}

	for _, c := range []struct {
		keywords []string
		free     bool
		want     []string
	}{
		{nil, true, nil},
		{nil, false, f77},
		{[]string{"SYNTAX"}, true, nil},
		{[]string{"F77"}, true, []string{"extension to FORTRAN-77: free source form", f77[0]}},
		{[]string{"F90"}, false, nil},
		{[]string{"F77", "SYNTAX"}, true, f77},
		{[]string{"F90", "NOSYNTAX"}, false, nil},
	} {
		lines := fixed
		if c.free {
			lines = free
		}

		if got := compile(c.keywords, c.free, lines...); !slices.Equal(got, c.want) {
			t.Errorf("%v (free %v): flags %q, want %q", c.keywords, c.free, got, c.want)
		}
	}

	got := compile([]string{"F90"}, true, "byte b", "end")
	if want := []string{"extension to Fortran 90: the BYTE statement"}; !slices.Equal(got, want) {
		t.Errorf("F90 flags %q, want %q", got, want)
	}

	for _, word := range []string{"OBSOLESCENT", "NOOBS", "F7", "F9"} {
		if _, err := (sema.Conformance{}).With([]string{word}); err != nil {
			t.Errorf("%s: %v", word, err)
		}
	}

	if _, err := (sema.Conformance{}).With([]string{"F"}); err == nil {
		t.Error("F was taken as a keyword")
	}
}
