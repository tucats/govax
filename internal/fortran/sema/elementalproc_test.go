package sema

import (
	"strings"
	"testing"
)

// TestElementalProcedures holds ELEMENTAL procedures' rules (Phase 112's subtask 7;
// elemental.go): a definition's dummies, value, alternate returns, and
// prefixes; references with arrays, their conformance, ELEMOUT, and the
// explicit interface they need; a generic's elemental specific chosen only when
// no other fits; and elemental defined assignment in a WHERE, WHEREDEFINED for
// one that isn't.
func TestElementalProcedures(t *testing.T) {
	cases := []struct {
		name  string
		lines []string
		want  string
	}{
		{"references", []string{
			"module m", "contains",
			"  elemental real function clip(x, lo, hi)", "    real, intent(in) :: x, lo, hi",
			"    clip = max(lo, min(hi, x))", "  end function clip",
			"  elemental subroutine swap(a, b)", "    integer, intent(inout) :: a, b",
			"    integer :: t", "    t = a; a = b; b = t", "  end subroutine swap",
			"end module m",
			"program p", "  use m", "  real :: r(5), s(5)", "  integer :: i(3), j(3), k(2, 3)",
			"  s = clip(r, 0.0, 1.0) + clip(r(5:1:-1), -1.0, s)", "  s(1) = clip(r(1), 0.0, 1.0)",
			"  print *, sum(clip(r, 0.0, 1.0))", "  call swap(i, j)", "  call swap(i(1), j(2))", "  call swap(k(1, :), j)",
			"end program p"},
			""},
		{"a dummy in a bound (12.7.1, Phase 113)", []string{
			"module m", "contains",
			"  elemental real function e(a, n, c)", "    real, intent(in) :: a", "    integer, intent(in) :: n",
			"    character(len=*), intent(in) :: c",
			"    real :: w(n)", "    character(len=len(c)) :: t", "    integer :: b(bit_size(n))", "    character(len=n) :: u",
			"    e = a", "  end function e",
			"end module m"},
			"ELEMBOUND: N, a dummy argument of the ELEMENTAL function E, can be in an array's bounds only as an inquiry function's argument; " +
				"ELEMBOUND: N, a dummy argument of the ELEMENTAL function E, can be in a CHARACTER length only as an inquiry function's argument"},
		{"definitions", []string{
			"module m", "contains",
			"  elemental integer function f(a, p, g)", "    integer, intent(in) :: a(3)", "    integer, pointer :: p",
			"    integer, external :: g", "    f = 1", "  end function f",
			"  elemental function h(x) result(r)", "    integer, intent(in) :: x", "    integer :: r(2)", "    r = x",
			"  end function h",
			"  elemental subroutine s(a, *)", "    integer, intent(in) :: a", "  end subroutine s",
			"  recursive elemental subroutine t(a)", "    integer, intent(in) :: a", "  end subroutine t",
			"end module m"},
			"ELEMRECURSIVE: T can't be both ELEMENTAL and RECURSIVE; " +
				"ELEMDUMMY: A, a dummy argument of the ELEMENTAL function F, can't be an array; " +
				"ELEMDUMMY: P, a dummy argument of the ELEMENTAL function F, can't be a pointer; " +
				"ELEMDUMMY: G, a dummy argument of the ELEMENTAL function F, can't be a dummy procedure; " +
				"ELEMRESULT: the value of the ELEMENTAL function H can't be an array; " +
				"ELEMRETURN: the ELEMENTAL subroutine S can't have an alternate return"},
		{"shapes", []string{
			"module m", "contains",
			"  elemental real function add(x, y)", "    real, intent(in) :: x, y", "    add = x + y",
			"  end function add",
			"  elemental subroutine bump(a, b)", "    integer, intent(inout) :: a", "    integer, intent(in) :: b",
			"    a = a + b", "  end subroutine bump",
			"end module m",
			"program p", "  use m", "  real :: r(5), s(4)", "  integer :: n, v(3)",
			"  print *, add(r, s)", "  call bump(n, v)",
			"end program p"},
			"NOTCONFORM: the shapes (5) and (4) don't conform; " +
				"ELEMOUT: argument 1 of BUMP must be an array, since another is and its dummy argument A is INTENT(INOUT)"},
		{"an explicit interface", []string{
			"program p", "  real :: r(3)", "  r = half(r)", "end program p",
			"elemental real function half(x)", "  real, intent(in) :: x", "  half = x / 2.0", "end function half"},
			"EXPLICITELEMENTAL: HALF (line 5) is ELEMENTAL, so a reference to it needs an explicit interface"},
		{"generics", []string{
			"module m", "  interface twice", "    module procedure twicev, twicee", "  end interface", "contains",
			"  function twicev(v) result(r)", "    integer, intent(in) :: v(:)", "    integer :: r(size(v))", "    r = 2 * v",
			"  end function twicev",
			"  elemental integer function twicee(k)", "    integer, intent(in) :: k", "    twicee = 2 * k",
			"  end function twicee",
			"end module m",
			"program p", "  use m", "  integer :: a(3), b(2, 2)", "  real :: x(3)",
			"  a = twice(a)", "  b = twice(b)", "  a(1) = twice(4)", "  print *, twice(x)",
			"end program p"},
			"GENERICNONE: no specific procedure of the generic name TWICE takes a REAL array of rank 1"},
		{"defined assignment in a WHERE", []string{
			"module m", "  type t", "    integer :: v", "  end type",
			"  type u", "    integer :: w", "  end type",
			"  interface assignment(=)", "    module procedure asg, asgu", "  end interface",
			"contains",
			"  elemental subroutine asg(a, b)", "    type(t), intent(out) :: a", "    integer, intent(in) :: b",
			"    a%v = 2 * b", "  end subroutine asg",
			"  subroutine asgu(a, b)", "    type(u), intent(out) :: a(:)", "    integer, intent(in) :: b(:)",
			"    a%w = b", "  end subroutine asgu",
			"end module m",
			"program p", "  use m", "  type(t) :: x(4)", "  type(u) :: y(4)", "  integer :: n(4)",
			"  x = n", "  where (n > 2) x = n + 10", "  y = n", "  where (n > 2) y = n",
			"end program p"},
			"WHEREDEFINED: a defined assignment in a WHERE must be by an ELEMENTAL subroutine, and ASGU isn't one"},
	}

	for _, c := range cases {
		ms := analyzeFree(t, c.lines...)

		var got []string
		for _, m := range ms {
			got = append(got, m.Ident+": "+m.Text)
		}

		if g := strings.Join(got, "; "); g != c.want {
			t.Errorf("%s:\n got %s\nwant %s", c.name, g, c.want)
		}
	}
}
