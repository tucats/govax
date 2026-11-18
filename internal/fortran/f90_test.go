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
