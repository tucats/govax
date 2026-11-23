package fortran

import (
	"slices"
	"strconv"
	"testing"

	"github.com/tucats/govax/internal/fortran/sema"
)

// findingsOf is what a free-form or fixed-form program compiled with the /STANDARD
// keywords says of the kinds idents, each finding its line and message. A compilation
// that fails is the test's failure.
func findingsOf(t *testing.T, keywords []string, free bool, idents []string, lines ...string) []string {
	t.Helper()

	opts := Options{}

	if keywords != nil {
		standard, err := sema.Conformance{}.With(keywords)
		if err != nil {
			t.Fatal(err)
		}

		opts.Standard = standard
	}

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
		if slices.Contains(idents, m.Ident) {
			found = append(found, strconv.Itoa(m.Pos.Line)+" "+m.Text)
		}
	}

	return found
}

// obsolescentProgram has each of Fortran 90's obsolescent features (its B.2) once: in
// Fortran 95 some are still obsolescent, and the rest deleted (its B.1).
var obsolescentProgram = []string{
	"program t", // 1
	"integer :: i, k, n",
	"real :: x",
	"logical :: l",
	"i = 1; n = 0; l = .true.", // 5
	"if (i) 10, 20, 20",
	"10 assign 30 to k",
	"go to k",
	"20 pause",
	"30 do x = 1.0, 2.0", // 10
	"end do",
	"do 40 i = 1, 2",
	"do 40 k = 1, 2",
	"40 n = n + 1",
	"go to 50", // 15
	"if (l) then",
	"n = 0",
	"50 end if",
	"assign 60 to k",
	"print k, n", // 20
	"60 format (1x, 2hn=, i4)",
	"call s(*70)",
	"70 continue",
	"contains",
	"subroutine s(*)", // 25
	"return 1",
	"end subroutine s",
	"end program t",
}

// obsolescentFound is what /STANDARD=F90 says of obsolescentProgram. An internal
// procedure is analyzed, and its messages given, before its host's statements are
// looked at, as for every message.
var obsolescentFound = []string{
	"25 obsolescent in Fortran 95: an alternate return",
	"26 obsolescent in Fortran 95: an alternate return",
	"6 obsolescent in Fortran 95: the arithmetic IF statement",
	"7 deleted in Fortran 95: the ASSIGN statement",
	"8 deleted in Fortran 95: the assigned GO TO statement",
	"9 deleted in Fortran 95: the PAUSE statement",
	"10 deleted in Fortran 95: a DO variable of type REAL",
	"14 obsolescent in Fortran 95: a terminal statement shared by DO loops",
	"14 obsolescent in Fortran 95: a DO loop ending at an assignment",
	"15 deleted in Fortran 95: a jump to END IF from outside its IF block",
	"19 deleted in Fortran 95: the ASSIGN statement",
	"20 deleted in Fortran 95: an assigned format specifier",
	"21 deleted in Fortran 95: the H edit descriptor",
	"22 obsolescent in Fortran 95: an alternate return",
}

// standardFindings are the messages /STANDARD gives.
var standardFindings = []string{"OBSOLESCENT", "DELETED", "EXTENSION"}

// TestStandardObsolescent holds each obsolescent and deleted feature to its message, the
// walk's and semantic analysis's together in each unit's source order.
func TestStandardObsolescent(t *testing.T) {
	got := findingsOf(t, []string{"F90"}, true, standardFindings, obsolescentProgram...)
	if !slices.Equal(got, obsolescentFound) {
		t.Errorf("flags\n%q\nwant\n%q", got, obsolescentFound)
	}
}

// TestStandardObsolescentQuiet is what isn't obsolescent: a jump to END IF from inside
// its block, a loop ending at CONTINUE or END DO, an INTEGER DO variable, a quoted
// edit descriptor, a FORMAT label as the format.
func TestStandardObsolescentQuiet(t *testing.T) {
	got := findingsOf(t, []string{"F90"}, true, standardFindings,
		"program t",
		"integer :: i, n",
		"logical :: l",
		"n = 0; l = .true.",
		"if (l) then",
		"if (n > 1) go to 10",
		"n = 1",
		"10 end if",
		"do 20 i = 1, 2",
		"n = n + i",
		"20 continue",
		"do 30 i = 1, 2",
		"n = n + i",
		"30 end do",
		"print 40, n",
		"40 format (1x, 'n=', i4)",
		"end program t")
	if got != nil {
		t.Errorf("flags %q", got)
	}
}

// TestStandardObsolescentSettings is which settings flag the obsolescent features:
// Fortran 90's judgment with OBSOLESCENT on, by itself or with the rest, and nothing
// else; never FORTRAN-77's, and nothing without /STANDARD.
func TestStandardObsolescentSettings(t *testing.T) {
	idents := []string{"OBSOLESCENT", "DELETED"}
	lines := []string{"program t", "integer :: i", "i = 1", "if (i) 10, 10, 10", "10 pause", "end program t"}
	both := []string{
		"4 obsolescent in Fortran 95: the arithmetic IF statement",
		"5 deleted in Fortran 95: the PAUSE statement",
	}

	for _, c := range []struct {
		keywords []string
		want     []string
	}{
		{nil, nil},
		{[]string{"F90"}, both},
		{[]string{"SYNTAX"}, both},
		{[]string{"OBSOLESCENT"}, both},
		{[]string{"NOSYNTAX", "OBSOLESCENT"}, both},
		{[]string{"F90", "NOOBSOLESCENT"}, nil},
		{[]string{"F90", "NOSYNTAX"}, nil},
		{[]string{"F77"}, nil},
		{[]string{"F77", "OBSOLESCENT"}, nil},
	} {
		if got := findingsOf(t, c.keywords, true, idents, lines...); !slices.Equal(got, c.want) {
			t.Errorf("%v: flags %q, want %q", c.keywords, got, c.want)
		}
	}

	// NOSYNTAX with OBSOLESCENT looks only for the obsolescent features.
	got := findingsOf(t, []string{"NOSYNTAX", "OBSOLESCENT"}, true, []string{"EXTENSION"},
		"program t", "byte b", "b = 1", "end program t")
	if got != nil {
		t.Errorf("NOSYNTAX, OBSOLESCENT flags %q", got)
	}
}

// TestStandardConstantExpressions holds the intrinsic functions in constant expressions
// to each standard's rule: FORTRAN-77 has none there, Fortran 90 an elemental function
// only of INTEGER or CHARACTER arguments and value; an executable statement's folded
// expression is under no rule.
func TestStandardConstantExpressions(t *testing.T) {
	idents := []string{"EXTENSION"}

	got := findingsOf(t, []string{"F90"}, true, idents,
		"program t",
		"integer, parameter :: n = ceiling(2.5)",
		"real, parameter :: x = abs(-2.0)",
		"integer, parameter :: m = max(1, 2) + len_trim('ab ')",
		"character(len=3), parameter :: c = repeat('a', 3)",
		"logical, parameter :: l = lge('a', 'b')",
		"real :: y",
		"select case (m)",
		"case (min(1, 2))",
		"end select",
		"y = abs(-2.0) + n + x + len(c)",
		"end program t")
	want := []string{
		"2 extension to Fortran 90: a REAL argument to CEILING in a constant expression",
		"3 extension to Fortran 90: the intrinsic function ABS of type REAL in a constant expression",
		"6 extension to Fortran 90: the intrinsic function LGE of type LOGICAL in a constant expression",
	}

	if !slices.Equal(got, want) {
		t.Errorf("F90 flags\n%q\nwant\n%q", got, want)
	}

	got = findingsOf(t, []string{"F77"}, false, idents,
		"      PROGRAM T",
		"      PARAMETER (M = MAX(1, 2), X = 2.0)",
		"      Y = ABS(X) + M",
		"      END")
	want = []string{"2 extension to FORTRAN-77: the intrinsic function MAX in a constant expression"}

	if !slices.Equal(got, want) {
		t.Errorf("F77 flags %q, want %q", got, want)
	}
}
