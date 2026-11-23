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
