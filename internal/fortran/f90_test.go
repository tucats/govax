package fortran_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/fortran"
)

// TestNoF90Explicit checks that /NOF90 given is the default for a fixed-form
// source: every corpus source compiles to the same MACRO file and messages
// (docs/planning/PHASE-99 - Fortran 90.md, Decision 2).
func TestNoF90Explicit(t *testing.T) {
	for _, path := range noF90Sources(t) {
		opts := noF90Options("X")
		opts.Optimize = true

		given := opts
		given.F90Given = true

		if noF90Digest(t, path, opts) != noF90Digest(t, path, given) {
			t.Errorf("%s: /NOF90 compiles otherwise than no /F90", noF90Key(path))
		}
	}
}

// TestF90Default checks /F90's default: the first file's form, unless given.
// Nothing reads it yet but the source pass's " rule, which the lexer doesn't
// share until subtask 4, so what there is to see is that both forms compile.
func TestF90Default(t *testing.T) {
	fixed := fortran.File{Spec: "A.FOR", Name: "A", Lines: []string{"      PROGRAM A", "      END"}}
	free := fortran.File{Spec: "A.F90", Name: "A", Lines: []string{"program a; i = 1", "end"}, Free: true}

	ids := func(r fortran.Result) []string {
		var out []string
		for _, m := range r.Messages {
			out = append(out, m.Ident)
		}

		return out
	}

	opts := noF90Options("A")

	if r := fortran.Compile(opts, fixed); r.Errors != 0 || r.MACRO == nil {
		t.Errorf("fixed form: %v", ids(r))
	}

	if r := fortran.Compile(opts, free); r.Errors != 0 || r.MACRO == nil {
		t.Errorf("free form: %v", ids(r))
	}

	opts.F90, opts.F90Given = true, true
	if r := fortran.Compile(opts, fixed); r.Errors != 0 || r.MACRO == nil {
		t.Errorf("fixed form under /F90: %v", ids(r))
	}
}

// TestF90LexicalItems compiles Fortran 90's lexical items through every pass
// (docs/planning/PHASE-99 - Fortran 90.md, subtask 4): the relational operators'
// new spellings compile as .EQ. and the rest, a quoted character constant, and
// Z'1F' and B'101' in DATA; and a kind parameter (Phase 100).
func TestF90LexicalItems(t *testing.T) {
	free := fortran.File{Spec: "A.F90", Name: "A", Free: true, Lines: []string{
		"program a",
		"  integer i, j",
		"  data i /z'1f'/, j /b'101'/",
		`  if (i == 31 .and. j /= 4) print *, "it's ", i, j`,
		"end",
	}}

	r := fortran.Compile(noF90Options("A"), free)
	if r.Errors != 0 || r.MACRO == nil {
		t.Fatalf("messages: %v", r.Messages)
	}

	text := strings.Join(r.MACRO, "\n")
	for _, want := range []string{"\t.LONG\t31\t", "\t.LONG\t5\t", "\tCMPL\tA.I,#31\n\tBEQL", "\tCMPL\tA.J,#4\n\tBNEQ", "\t.ASCII\t/it's /"} {
		if !strings.Contains(text, want) {
			t.Errorf("the MACRO file has no %q", want)
		}
	}

	kinded := fortran.File{Spec: "B.F90", Name: "B", Free: true, Lines: []string{"x = 1.5_8", "end"}}

	r = fortran.Compile(noF90Options("B"), kinded)
	if len(r.Messages) != 0 {
		t.Errorf("a kind parameter: %v", r.Messages)
	}
}

// TestF90StatementsNotImplemented compiles Fortran 90's statements through
// every pass (docs/planning/PHASE-99 - Fortran 90.md, subtask 5): each is
// NOTIMPL by name where it is, a construct's inner statements and a module's
// contents saying nothing more, and named ENDs compile.
func TestF90StatementsNotImplemented(t *testing.T) {
	lines := []string{
		"module shapes", // 1
		"  type point",
		"    real :: x, y",
		"  end type point",
		"contains", // 5
		"  real function area(r)",
		"    real r",
		"    area = 3.14 * r * r",
		"  end function area",
		"end module shapes", // 10
		"program squares",
		"  use shapes",
		"  integer :: i",
		"  real(8) x",
		"  type(point) p", // 15
		"  outer: do i = 1, 10",
		"    select case (i)",
		"    case (1)",
		"      cycle outer",
		"    case default", // 20
		"      if (i > 5) exit",
		"    end select",
		"  end do outer",
		"  where (a > 0) a = 1",
		"  where (a > 0)", // 25
		"    a = 2",
		"  elsewhere",
		"    a = 3",
		"  end where",
		"  p => q", // 30
		"  allocate (a(10))",
		"end program squares",
		"subroutine s(n)",
		"  print *, n",
		"end subroutine s", // 35
	}

	r := fortran.Compile(noF90Options("A"), fortran.File{Spec: "A.F90", Name: "A", Free: true, Lines: lines})

	var got []string
	for _, m := range r.Messages {
		got = append(got, fmt.Sprintf("%d:%d %s", m.Pos.Line, m.Pos.Column, strings.TrimSuffix(m.Text, " is not implemented yet")))
	}

	want := []string{
		"1:1 the MODULE statement",
		"12:3 the USE statement",
		"30:3 the pointer assignment statement",
	}

	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("got\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}

	// Named ENDs alone compile.
	named := fortran.File{Spec: "B.F90", Name: "B", Free: true, Lines: []string{
		"program b", "  call s(2)", "end program b", "subroutine s(n)", "  print *, n", "end subroutine s",
	}}

	if r := fortran.Compile(noF90Options("B"), named); r.Errors != 0 || r.MACRO == nil {
		t.Errorf("named ENDs: %v", r.Messages)
	}
}

// TestDerivedTypeMap checks the storage map's lines for derived types
// (docs/planning/PHASE-105 - Fortran 90 derived types.md, subtask 2): each type
// with its size and SEQUENCE, each component at its offset, a type without
// SEQUENCE naturally aligned, and an object's type as TYPE(name).
func TestDerivedTypeMap(t *testing.T) {
	lines := []string{
		"program p",
		"  type flag",
		"    logical(1) :: on",
		"    integer :: n",
		"  end type flag",
		"  type pair",
		"    sequence",
		"    logical(1) :: on",
		"    integer :: n",
		"  end type pair",
		"  type(flag) :: f",
		"  type(pair) :: g(2)",
		"  f%n = 1",
		"  g(2)%n = f%n",
		"end program",
	}

	opts := noF90Options("A")
	opts.F90 = true
	r := fortran.Compile(opts, fortran.File{Spec: "A.F90", Name: "A", Free: true, Lines: lines})

	if len(r.Messages) != 0 {
		t.Fatalf("messages: %v", r.Messages)
	}

	text := strings.Join(r.MACRO, "\n")

	for _, want := range []string{
		"Type FLAG, 8 bytes",
		"+0     ON    LOGICAL*1",
		"+4     N     INTEGER",
		"Type PAIR, 5 bytes, SEQUENCE",
		"+1     N     INTEGER",
		"F     TYPE(FLAG)",
		"G     TYPE(PAIR) (2)",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("no %q in the MACRO file:\n%s", want, text)
		}
	}
}

// TestDerivedResultCode checks how a function of a derived type returns its
// value (docs/planning/PHASE-105 - Fortran 90 derived types.md, finding 11): up
// to 8 bytes in R0, or R0 and R1, its value's cell loaded whole, a MOVL up to a
// longword and a MOVQ past it; stored by the caller straight into a target of a
// byte, word, longword, or quadword, and into a temporary otherwise; past 8
// bytes through a hidden first argument.
func TestDerivedResultCode(t *testing.T) {
	types := map[string][]string{
		"ONE":    {"integer(1) :: a"},
		"TWO":    {"integer(2) :: a"},
		"THREE":  {"character(len=3) :: a"},
		"EIGHT":  {"real :: a, b"},
		"TWELVE": {"real :: a, b, c"},
	}
	names := []string{"ONE", "TWO", "THREE", "EIGHT", "TWELVE"}

	define := func() []string {
		var lines []string
		for _, n := range names {
			lines = append(append(append(lines, "  type "+n, "    sequence"), "    "+types[n][0]), "  end type "+n)
		}

		return lines
	}

	lines := []string{"program p"}
	lines = append(lines, define()...)

	for _, n := range names {
		lines = append(lines, "  type("+n+") :: v"+n+", f"+n, "  external f"+n)
	}

	for _, n := range names {
		lines = append(lines, "  v"+n+" = f"+n+"()")
	}

	lines = append(lines, "end program")

	for _, n := range names {
		lines = append(lines, "type("+n+") function f"+n+"()")
		lines = append(lines, define()...)
		lines = append(lines, "  f"+n+"%a = 1", "end function")
	}

	// THREE's value is CHARACTER; give it text.
	for i, l := range lines {
		if l == "  fTHREE%a = 1" {
			lines[i] = "  fTHREE%a = 'abc'"
		}
	}

	opts := noF90Options("A")
	opts.F90 = true
	r := fortran.Compile(opts, fortran.File{Spec: "A.F90", Name: "A", Free: true, Lines: lines})

	if len(r.Messages) != 0 {
		t.Fatalf("messages: %v", r.Messages)
	}

	text := strings.Join(r.MACRO, "\n")

	for _, want := range []string{
		"CALLS\t#0,G^FONE\n\tMOVB\tR0,P.VONE",
		"CALLS\t#0,G^FTWO\n\tMOVW\tR0,P.VTWO",
		"CALLS\t#0,G^FTHREE\n\tMOVL\tR0,P.$T",
		"CALLS\t#0,G^FEIGHT\n\tMOVQ\tR0,P.VEIGHT",
		"CALLS\t#1,G^FTWELVE",
		"MOVL\tFONE.FONE,R0",
		"MOVL\tFTHREE.FTHREE,R0",
		"MOVQ\tFEIGHT.FEIGHT,R0",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("no %q in the MACRO file", want)
		}
	}

	if t.Failed() {
		t.Logf("the MACRO file:\n%s", text)
	}
}

// TestComponentArgumentCode holds what a component with a rank is as an actual
// argument (docs/planning/PHASE-105 - Fortran 90 derived types.md, subtask 5b):
// to an assumed-shape dummy, a class NCA descriptor of the components where they
// are, its POINTER the first one's address and its stride the type's size times
// the section's step, with no copy; to an explicit-shape dummy, a temporary the
// components are copied into and back out of.
func TestComponentArgumentCode(t *testing.T) {
	lines := []string{
		"program p",
		"  type point",
		"    real :: x, y",
		"  end type point",
		"  type(point) :: path(6)",
		"  call scale(path(2:6:2)%y)",
		"  call old(path%x, 6)",
		"contains",
		"  subroutine scale(v)",
		"    real :: v(:)",
		"    v = 2.0 * v",
		"  end subroutine scale",
		"  subroutine old(v, n)",
		"    integer :: n",
		"    real :: v(n)",
		"    v = 2.0 * v",
		"  end subroutine old",
		"end program",
	}

	opts := noF90Options("A")
	opts.F90 = true
	r := fortran.Compile(opts, fortran.File{Spec: "A.F90", Name: "A", Free: true, Lines: lines})

	if len(r.Messages) != 0 {
		t.Fatalf("messages: %v", r.Messages)
	}

	text := strings.Join(r.MACRO, "\n")

	for _, want := range []string{
		// PATH(2)%Y is 8 bytes past PATH, plus Y's 4; each next is 16 on.
		"MOVAL\tP.PATH+12,P.$D1+4\n\tMOVL\t#16,P.$D1+20\n\tMOVL\t#1,P.$D1+24\n\tMOVL\t#3,P.$D1+28",
		"PUSHAQ\tP.$D1\n\tCALLS\t#1,P.SCALE",
		// The components copied into a temporary, which is passed.
		"MOVF\tP.PATH-8[R0],P.$W1-4[R1]",
		"MOVAL\tP.$W1,R0\n\tPUSHL\tR0\n\tCALLS\t#2,P.OLD",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("no %q in the MACRO file", want)
		}
	}

	if !strings.Contains(text, "CALLS\t#2,P.OLD") || !strings.Contains(text[strings.Index(text, "CALLS\t#2,P.OLD"):],
		"MOVF\tP.$W1-4[R0],P.PATH-8[R1]") {
		t.Errorf("the temporary isn't copied back after the call")
	}

	if t.Failed() {
		t.Logf("the MACRO file:\n%s", text)
	}
}
