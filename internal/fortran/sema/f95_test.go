package sema

import (
	"strings"
	"testing"
)

// TestF95Pending checks that Fortran 95's forms, which the parser builds since
// Phase 112's subtask 2, are NOTIMPL by name until their subtasks, each once,
// with nothing else said: a FORALL construct's statements are passed over, and
// a unit with a PURE or ELEMENTAL header keeps its structure.
func TestF95Pending(t *testing.T) {
	cases := []struct {
		name  string
		lines []string
		want  string
	}{
		{"PURE", []string{
			"module m", "contains",
			"  pure integer function sq(x)", "    integer, intent(in) :: x", "    sq = x * x", "  end function sq",
			"end module m"},
			"NOTIMPL: a PURE procedure is not implemented yet"},
		{"ELEMENTAL", []string{
			"program p", "  print *, inc(1)", "contains",
			"  elemental integer function inc(k)", "    integer, intent(in) :: k", "    inc = k + 1", "  end function inc",
			"end program p"},
			"NOTIMPL: an ELEMENTAL procedure is not implemented yet"},
		{"FORALL statement", []string{
			"program p", "  implicit none", "  integer :: a(4)", "  forall (i = 1:4) a(i) = i", "end program p"},
			"NOTIMPL: the FORALL statement is not implemented yet"},
		{"FORALL construct", []string{
			"program p", "  implicit none", "  integer :: a(4,4)",
			"  outer: forall (i = 1:4)", "    forall (j = 1:4, j > i) a(i,j) = i + j",
			"    inner: forall (j = 1:4)", "      a(j,j) = k", "    end forall inner",
			"    where (a(i,:) == 0) a(i,:) = -1",
			"  end forall outer", "  a(1,1) = 0", "end program p"},
			"NOTIMPL: the FORALL construct is not implemented yet"},
		{"=> NULL()", []string{
			"program p", "  integer, pointer :: q => null()", "end program p"},
			"NOTIMPL: a pointer's initial value after => is not implemented yet"},
		{"an unknown prefix", []string{
			"module m", "contains",
			"  impure subroutine s(a)", "    integer :: a", "    a = 1", "  end subroutine s",
			"  subroutine t", "  end subroutine t",
			"end module m"},
			"UNRECPREFIX: IMPURE is not a prefix FORTRAN knows; the rest is read as a SUBROUTINE statement"},
		{"END INTERFACE named", []string{
			"program p", "  interface g", "    subroutine s(a)", "      integer :: a", "    end subroutine s",
			"  end interface g", "  call g(1)", "end program p"},
			""},
	}

	for _, c := range cases {
		ms := analyzeFree(t, c.lines...)

		var got []string
		for _, m := range ms {
			got = append(got, m.Ident+": "+m.Text)
		}

		if strings.Join(got, "; ") != c.want {
			t.Errorf("%s:\n got %q\nwant %q", c.name, got, c.want)
		}
	}
}

// TestEmptyArrays holds Fortran 90's zero-sized explicit-shape arrays (Phase 112's
// subtask 3): under /F90 a dimension declared with its upper bound below its lower
// is kept as (1:0), in a variable, a component, COMMON, EQUIVALENCE, a PARAMETER,
// and an initial value, its array of no elements and no storage; without /F90 it
// is still VAX FORTRAN's BOUNDORDER.
func TestEmptyArrays(t *testing.T) {
	prog, messages := analyzeFreeWith(t, Options{},
		"program p",
		"  implicit none",
		"  integer :: i",
		"  type box",
		"    integer :: n",
		"    real :: v(5:4)",
		"    integer :: m",
		"  end type",
		"  integer, parameter :: none(0) = (/ (i, i = 1, 0) /)",
		"  real :: a(0), b(5:1) = 2.0, c(3, -1:-2)",
		"  integer :: x, eq(0), z0(0), y",
		"  common /blk/ x, z0, y",
		"  equivalence (eq, i)",
		"  type(box) :: bx",
		"  bx%n = size(none) + size(a) + size(b) + size(c) + size(eq)",
		"end program p")
	if len(messages) != 0 {
		t.Fatalf("messages: %s", messageText(messages))
	}

	u := prog.Units[0]

	for _, c := range []struct {
		name string
		dims []Dim
	}{
		{"A", []Dim{{Lower: 1, Upper: 0}}},
		{"B", []Dim{{Lower: 1, Upper: 0}}},
		{"C", []Dim{{Lower: 1, Upper: 3}, {Lower: 1, Upper: 0}}},
		{"NONE", []Dim{{Lower: 1, Upper: 0}}},
	} {
		s := u.Lookup(c.name)
		if s == nil {
			t.Fatalf("no %s", c.name)
		}

		if len(s.Dims) != len(c.dims) {
			t.Fatalf("%s: %d dimensions", c.name, len(s.Dims))
		}

		for i, d := range c.dims {
			if s.Dims[i].Lower != d.Lower || s.Dims[i].Upper != d.Upper {
				t.Errorf("%s: dimension %d is %d:%d, want %d:%d", c.name, i+1, s.Dims[i].Lower, s.Dims[i].Upper, d.Lower, d.Upper)
			}
		}

		if !s.Empty() || s.Elements() != 0 {
			t.Errorf("%s: Empty %v, Elements %d", c.name, s.Empty(), s.Elements())
		}
	}

	if s := u.Lookup("A"); s.Size() != 0 {
		t.Errorf("A's size is %d", s.Size())
	}

	if y, x := u.Lookup("Y"), u.Lookup("X"); y.Offset != x.Offset+4 {
		t.Errorf("Y is at %d, X at %d: Z0 took space", y.Offset, x.Offset)
	}

	if bx := u.Lookup("BX"); bx.Struct == nil || bx.Struct.Size != 8 {
		t.Errorf("BOX isn't 8 bytes: %+v", bx.Struct)
	}

	for _, s := range []string{"X", "BX"} {
		if u.Lookup(s).Empty() {
			t.Errorf("%s is Empty", s)
		}
	}

	// An element of an empty array is never in it.
	_, messages = analyzeFreeWith(t, Options{}, "program p", "  real :: a(0), x", "  equivalence (a(1), x)", "end program p")
	if len(messages) != 1 || messages[0].Ident != "EQUIVRANGE" {
		t.Errorf("A(1): %s, want EQUIVRANGE", messageText(messages))
	}

	// Without /F90, still BOUNDORDER, in a variable and a component.
	_, messages = analyzeOpts(t, Options{}, "      REAL A(0)", "      STRUCTURE /S/", "        REAL V(5:4)",
		"      END STRUCTURE", "      END")
	if got := messageText(messages); got != "BOUNDORDER: the upper bound 0 is less than the lower bound 1; "+
		"BOUNDORDER: the upper bound 4 is less than the lower bound 5" {
		t.Errorf("without /F90: %s", got)
	}
}
