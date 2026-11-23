package fortran

import (
	"slices"
	"strconv"
	"testing"

	"github.com/tucats/govax/internal/fortran/sema"
)

// /STANDARD and Fortran 95 (docs/planning/PHASE-112 - Fortran 95.md, subtask 13):
// what /STANDARD=F90 judges by is Fortran 90 with Fortran 95's additions (Decision
// 2), so they are flagged only when judging by FORTRAN-77; and the obsolescent and
// deleted features are Fortran 95's (the author's answer 1).

// fortran95Forms has each of Fortran 95's additions /STANDARD can see.
var fortran95Forms = []string{
	"module m", // 1
	"type t",
	"integer :: a = 1",
	"integer, pointer :: p => null()",
	"end type t", // 5
	"contains",
	"pure integer function f(x)",
	"integer, intent(in) :: x",
	"f = x + 1",
	"end function f", // 10
	"elemental real function g(x)",
	"real, intent(in) :: x",
	"g = x * x",
	"end function g",
	"end module m", // 15
	"program p",
	"use m",
	"integer :: i, j",
	"real :: a(4), b(4,4), c",
	"integer, pointer :: q => null()", // 20
	"forall (i = 1:4) a(i) = f(i)",
	"forall (i = 1:4, j = 1:4, i /= j)",
	"b(i, j) = 0",
	"end forall",
	"where (a > 1)", // 25
	"where (a > 2)",
	"a = g(a)",
	"end where",
	"elsewhere (a < 0)",
	"a = 1", // 30
	"end where",
	"q => null()",
	"call cpu_time(c)",
	"print 10, i, c",
	"10 format (i0, f0.2)", // 35
	"end program p",
}

// TestStandardFortran95Forms: Fortran 95's additions are Fortran 90's under /STANDARD=F90,
// and extensions to FORTRAN-77, each once where its form is.
func TestStandardFortran95Forms(t *testing.T) {
	if got := findingsOf(t, []string{"F90"}, true, standardFindings, fortran95Forms...); got != nil {
		t.Errorf("F90 flags %q", got)
	}

	got := findingsOf(t, []string{"F77"}, true, standardFindings, fortran95Forms...)

	for _, want := range []string{
		"4 extension to FORTRAN-77: the intrinsic function NULL",
		"7 extension to FORTRAN-77: a PURE subprogram",
		"11 extension to FORTRAN-77: an ELEMENTAL subprogram",
		"20 extension to FORTRAN-77: the intrinsic function NULL",
		"21 extension to FORTRAN-77: the FORALL statement",
		"22 extension to FORTRAN-77: the FORALL construct",
		"22 extension to FORTRAN-77: the /= operator",
		"29 extension to FORTRAN-77: the < operator",
		"32 extension to FORTRAN-77: the intrinsic function NULL",
		"33 extension to FORTRAN-77: the intrinsic subroutine CPU_TIME",
		"35 extension to FORTRAN-77: the I edit descriptor with a width of 0",
		"35 extension to FORTRAN-77: the F edit descriptor with a width of 0",
	} {
		if !slices.Contains(got, want) {
			t.Errorf("F77 doesn't flag %q; flags\n%q", want, got)
		}
	}
}

// TestStandardZeroWidths: of the zero widths, Fortran 95 has I0, B0, O0, Z0, and F0.d;
// the others are no standard's.
func TestStandardZeroWidths(t *testing.T) {
	got := findingsOf(t, []string{"F90"}, true, standardFindings,
		"program t",
		"print 10, 1, 2, 3, 4, 5.0, 6.0, 7.0, .true.",
		"10 format (i0, b0, o0, z0, f0.1, e0.2, g0.3, l0)",
		"end program t")
	want := []string{
		"3 extension to Fortran 90: the E edit descriptor with a width of 0",
		"3 extension to Fortran 90: the G edit descriptor with a width of 0",
		"3 extension to Fortran 90: the L edit descriptor with a width of 0",
	}

	if !slices.Equal(got, want) {
		t.Errorf("flags\n%q\nwant\n%q", got, want)
	}
}

// TestStandardFortran95Obsolescent holds the features Fortran 95 made obsolescent to their
// messages: the computed GO TO, a statement function, DATA among the executable
// statements, an assumed-length CHARACTER function, and CHARACTER*n (but not
// CHARACTER*(*), nor CHARACTER(LEN=n), nor a length after a name).
func TestStandardFortran95Obsolescent(t *testing.T) {
	got := findingsOf(t, []string{"F90"}, true, standardFindings,
		"program t", // 1
		"integer :: i, k, n",
		"character*8 :: c",
		"character(len=4) :: d, e*2, s",
		"real :: f, x", // 5
		"f(x) = x + 1.0",
		"i = 2",
		"go to (10, 20), i",
		"10 continue",
		"20 n = 1", // 10
		"data k /3/",
		"c = s('ab')",
		"d = 'abcd'",
		"end program t",
		"function s(a)", // 15
		"character*(*) :: s",
		"character*(*), intent(in) :: a",
		"s = a",
		"end function s")
	want := []string{
		"3 obsolescent in Fortran 95: the CHARACTER*n form of a length",
		"6 obsolescent in Fortran 95: a statement function",
		"8 obsolescent in Fortran 95: the computed GO TO statement",
		"11 obsolescent in Fortran 95: a DATA statement among the executable statements",
		"16 obsolescent in Fortran 95: an assumed-length CHARACTER function",
	}

	if !slices.Equal(got, want) {
		t.Errorf("flags\n%q\nwant\n%q", got, want)
	}

	// FORTRAN-77 has no obsolescent features.
	if got := findingsOf(t, []string{"F77"}, false, []string{"OBSOLESCENT", "DELETED"},
		"      PROGRAM T",
		"      CHARACTER*8 C",
		"      F(X) = X + 1.0",
		"      C = 'A'",
		"      GO TO (10), 1",
		"   10 CONTINUE",
		"      DATA K /3/",
		"      END"); got != nil {
		t.Errorf("F77 flags %q", got)
	}
}

// TestStandardFixedForm: fixed source form is obsolescent in Fortran 95, flagged once per
// fixed-form file at its first statement, an included file of its own, whatever /F90
// says when /STANDARD=F90 names the standard; never under FORTRAN-77's judgment, and not
// with NOOBSOLESCENT.
func TestStandardFixedForm(t *testing.T) {
	lines := []string{"C     A COMMENT FIRST", "      PROGRAM T", "      INTEGER I", "      I = 1", "      END"}
	want := []string{"2 obsolescent in Fortran 95: fixed source form"}

	for _, c := range []struct {
		keywords []string
		want     []string
	}{
		{[]string{"F90"}, want},
		{[]string{"F90", "NOSYNTAX", "OBSOLESCENT"}, want},
		{[]string{"F90", "NOOBSOLESCENT"}, nil},
		{[]string{"F77"}, nil},
		{[]string{"ALL"}, nil}, // a /NOF90 unit is judged by FORTRAN-77
	} {
		if got := findingsOf(t, c.keywords, false, []string{"OBSOLESCENT"}, lines...); !slices.Equal(got, c.want) {
			t.Errorf("%v: flags %q, want %q", c.keywords, got, c.want)
		}
	}

	// Under /F90 a fixed-form file is judged by Fortran 90 without F90 named.
	got := findingsOf(t, []string{"ALL"}, false, []string{"OBSOLESCENT"},
		"      OPTIONS /F90", "      PROGRAM T", "      END")
	if !slices.Equal(got, []string{"1 obsolescent in Fortran 95: fixed source form"}) {
		t.Errorf("OPTIONS /F90: flags %q", got)
	}

	// A free-form file including a fixed-form one: the included file, at its first
	// statement.
	opts := Options{F90: true, F90Given: true, Includes: includes{"DEFS.INC": {"C     DEFINITIONS", "      INTEGER N"}}}
	opts.Standard = standardOf(t, "F90")

	r := Compile(opts, File{Spec: "T.F90", Name: "T", Free: true,
		Lines: []string{"program t", "include 'DEFS.INC'", "n = 1", "end program t"}})

	var found []string

	for _, m := range r.Messages {
		if m.Ident == "OBSOLESCENT" {
			found = append(found, m.Pos.File+":"+strconv.Itoa(m.Pos.Line)+" "+m.Text)
		}
	}

	if !slices.Equal(found, []string{"[INC]DEFS.INC:2 obsolescent in Fortran 95: fixed source form"}) {
		t.Errorf("an included fixed-form file: %q", found)
	}
}

// TestStandardDeletedKept: /WARNINGS=NOGENERAL doesn't hold back the obsolescent and
// deleted features /STANDARD asked for.
func TestStandardDeletedKept(t *testing.T) {
	opts := Options{F90: true, F90Given: true, Standard: standardOf(t, "F90")}
	opts.Warnings.NoGeneral = true

	r := Compile(opts, File{Spec: "T.F90", Name: "T", Free: true,
		Lines: []string{"program t", "integer :: i", "i = 1", "if (i) 10, 10, 10", "10 pause", "end program t"}})

	var found []string

	for _, m := range r.Messages {
		found = append(found, m.Ident)
	}

	if !slices.Equal(found, []string{"OBSOLESCENT", "DELETED"}) {
		t.Errorf("under NOGENERAL: %q", found)
	}
}

// standardOf is the /STANDARD setting the keywords make.
func standardOf(t *testing.T, keywords ...string) sema.Conformance {
	t.Helper()

	s, err := sema.Conformance{}.With(keywords)
	if err != nil {
		t.Fatal(err)
	}

	return s
}
