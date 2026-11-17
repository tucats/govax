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
// Z'1F' and B'101' in DATA; a kind parameter is NOTIMPL until Phase 100.
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
	if len(r.Messages) != 1 || r.Messages[0].Ident != "NOTIMPL" || !strings.Contains(r.Messages[0].Text, "a kind parameter on a constant") {
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
		"14:7 a kind or length selector",
		"15:3 the TYPE (type-name) statement",
		"16:3 a construct name",
		"17:5 the SELECT CASE statement",
		"19:7 the CYCLE statement",
		"21:18 the EXIT statement",
		"24:3 the WHERE statement",
		"25:3 the block WHERE statement",
		"30:3 the pointer assignment statement",
		"31:3 the ALLOCATE statement",
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
