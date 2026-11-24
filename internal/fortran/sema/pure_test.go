package sema

import (
	"strings"
	"testing"
)

// TestPure holds a PURE procedure's constraints (Phase 112's subtask 6; pure.go):
// each case one program, and every message it gives.
func TestPure(t *testing.T) {
	cases := []struct {
		name  string
		lines []string
		want  string
	}{
		{"a clean module", []string{
			"module m", "  implicit none", "contains",
			"  pure integer function sq(x)", "    integer, intent(in) :: x", "    sq = x * x", "  end function sq",
			"  pure subroutine both(a, b)", "    integer, intent(inout) :: a", "    integer, intent(out) :: b",
			"    integer :: t", "    t = sq(a); b = t; a = 0", "    call inner(b)",
			"  contains",
			"    pure subroutine inner(k)", "      integer, intent(inout) :: k", "      k = k + 1", "    end subroutine inner",
			"  end subroutine both",
			"end module m"},
			""},
		{"intents", []string{
			"module m", "contains",
			"  pure integer function f(x, y, p, g)", "    integer :: x", "    integer, intent(inout) :: y",
			"    integer, pointer :: p", "    external g", "    f = x + y + p", "  end function f",
			"  pure subroutine s(a, b)", "    integer :: a", "    integer, intent(in) :: b", "  end subroutine s",
			"end module m"},
			"PUREINTENT: X, a dummy argument of the PURE function F, must be declared INTENT(IN); " +
				"PUREINTENT: Y, a dummy argument of the PURE function F, must be declared INTENT(IN); " +
				"PUREINTENT: A, a dummy argument of the PURE subroutine S, must be declared with an INTENT"},
		{"locals", []string{
			"module m", "contains",
			"  pure subroutine s(a)", "    integer, intent(out) :: a",
			"    integer, save :: k", "    integer :: j = 1", "    integer :: i", "    integer, pointer :: q => null()",
			"    data i /2/", "    a = i + j + k", "  end subroutine s",
			"  pure subroutine t(a)", "    integer, intent(out) :: a", "    save", "    a = 1", "  end subroutine t",
			"end module m"},
			"NOTPURE: K, a local variable, can't be saved in a PURE procedure; " +
				"NOTPURE: J, a local variable, can't have an initial value in a PURE procedure; " +
				"NOTPURE: I, a local variable, can't have an initial value in a PURE procedure; " +
				"NOTPURE: Q, a local variable, can't have an initial value in a PURE procedure; " +
				"NOTPURE: a SAVE statement can't be used in a PURE procedure"},
		{"globals", []string{
			"module m", "  integer :: g", "  integer, target :: tg", "  integer, pointer :: gp", "  integer, allocatable :: ga(:)",
			"contains",
			"  pure subroutine s(a)", "    integer, intent(inout) :: a", "    integer, pointer :: lp",
			"    common /c/ cv", "    g = 1", "    cv = 2", "    a = g + cv",
			"    lp => tg", "    gp => lp", "    nullify (gp)", "    allocate (ga(3))",
			"    call t(g)", "    do g = 1, 2", "    end do",
			"  end subroutine s",
			"  pure subroutine t(k)", "    integer, intent(out) :: k", "    k = 0", "  end subroutine t",
			"end module m",
			"program p", "  use m", "  integer :: h",
			"contains",
			"  pure subroutine u()", "    g = 3", "    h = 4", "  end subroutine u",
			"end program p"},
			"NOTPURE: G, a variable of the module M, can't be given a value in a PURE procedure; " +
				"NOTPURE: CV, a variable in common block /C/, can't be given a value in a PURE procedure; " +
				"NOTPURE: TG, a variable of the module M, can't be a pointer's target in a PURE procedure; " +
				"NOTPURE: GP, a variable of the module M, can't be given a pointer association in a PURE procedure; " +
				"NOTPURE: GP, a variable of the module M, can't be nullified in a PURE procedure; " +
				"NOTPURE: GA, a variable of the module M, can't be allocated in a PURE procedure; " +
				"NOTPURE: G, a variable of the module M, can't be given a value in a PURE procedure; " +
				"NOTPURE: G, a variable of the module M, can't be passed to an INTENT(OUT) dummy argument in a PURE procedure; " +
				"NOTPURE: H, a variable of its host P, can't be given a value in a PURE procedure"},
		{"pointers and storage association (Phase 113)", []string{
			"module m",
			"  type node", "    integer :: v", "    integer, pointer :: p", "  end type",
			"  integer, target :: g", "  type(node) :: gn",
			"contains",
			"  pure subroutine s(x, y, k, t)", "    type(node), intent(in) :: x", "    type(node), intent(inout) :: y",
			"    integer, intent(in) :: k", "    integer, intent(in), target :: t", "    type(node) :: loc",
			"    integer, pointer :: lp", "    real :: c, l", "    common /blk/ c", "    equivalence (l, c)",
			"    x%p = 1", "    gn%p = 2", "    y%p = 3", "    loc = node(1, g)", "    loc = gn", "    loc = x",
			"    loc = y", "    y = node(k, gn%p)", "    lp => t", "    l = 1.0",
			"  end subroutine s",
			"  pure integer function f(q)", "    integer, pointer :: q", "    q = 5", "    f = q", "  end function f",
			"end module m"},
			"NOTPURE: X, a dummy argument with INTENT(IN), can't be given a value through a pointer component in a PURE procedure; " +
				"NOTPURE: GN, a variable of the module M, can't be given a value through a pointer component in a PURE procedure; " +
				"NOTPURE: G, a variable of the module M, can't be a pointer's target in a PURE procedure; " +
				"NOTPURE: GN, a variable of the module M, can't be assigned to a variable of a type with a pointer component in a PURE procedure; " +
				"NOTPURE: X, a dummy argument with INTENT(IN), can't be assigned to a variable of a type with a pointer component in a PURE procedure; " +
				"NOTPURE: T, a dummy argument with INTENT(IN), can't be a pointer's target in a PURE procedure; " +
				"NOTPURE: L, a variable in common block /BLK/, can't be given a value in a PURE procedure; " +
				"NOTPURE: Q, a dummy argument of a PURE function, can't be given a value through the pointer in a PURE procedure"},
		{"an internal procedure not declared PURE (Phase 113)", []string{
			"module m", "contains",
			"  pure subroutine s(a)", "    integer, intent(inout) :: a", "    a = 1",
			"  contains",
			"    subroutine inner()", "    end subroutine inner",
			"    pure subroutine fine()", "    end subroutine fine",
			"  end subroutine s",
			"end module m"},
			"NOTPURE: INNER, an internal procedure not declared PURE, can't be contained in a PURE procedure"},
		{"statements", []string{
			"module m", "contains",
			"  pure subroutine s(a, text)", "    integer, intent(inout) :: a", "    character(len=*), intent(out) :: text", "    logical :: l",
			"    write (text, '(I5)') a", "    read (text, *) a",
			"    print *, a", "    write (6, *) a", "    read (5, *) a",
			"    open (1, file='x')", "    close (1)", "    rewind 1", "    inquire (unit=1, exist=l)",
			"    if (a < 0) stop", "    pause", "  end subroutine s",
			"end module m"},
			"NOTPURE: a PRINT statement of a unit can't be used in a PURE procedure; " +
				"NOTPURE: a WRITE statement of a unit can't be used in a PURE procedure; " +
				"NOTPURE: a READ statement of a unit can't be used in a PURE procedure; " +
				"NOTPURE: an OPEN statement can't be used in a PURE procedure; " +
				"NOTPURE: a CLOSE statement can't be used in a PURE procedure; " +
				"NOTPURE: a REWIND statement can't be used in a PURE procedure; " +
				"NOTPURE: an INQUIRE statement can't be used in a PURE procedure; " +
				"NOTPURE: a STOP statement can't be used in a PURE procedure; " +
				"NOTPURE: a PAUSE statement can't be used in a PURE procedure"},
		{"references", []string{
			"module m", "contains",
			"  integer function imp(x)", "    integer, intent(in) :: x", "    imp = x", "  end function imp",
			"  pure integer function f(x, g)", "    integer, intent(in) :: x",
			"    interface", "      pure integer function g(y)", "        integer, intent(in) :: y", "      end function g", "    end interface",
			"    real :: r", "    integer :: v(3)",
			"    f = imp(x) + ext(x) + g(x) + abs(x) + int(sin(1.0))",
			"    call mvbits(x, 0, 1, v(1), 0)", "    call random_number(r)", "    call sub(v)", "    call date(v)",
			"    r = secnds(0.0)",
			"  end function f",
			"end module m"},
			"NOTPURE: the intrinsic subroutine RANDOM_NUMBER can't be called in a PURE procedure; " +
				"NOTPURE: the system subroutine DATE can't be called in a PURE procedure; " +
				"NOTPURE: the system function SECNDS can't be referenced in a PURE procedure; " +
				"NOTPURE: IMP, not declared PURE, can't be referenced in a PURE procedure; " +
				"NOTPURE: EXT, not declared PURE, can't be referenced in a PURE procedure; " +
				"NOTPURE: SUB, not declared PURE, can't be called in a PURE procedure"},
		{"a pure recursive function calling itself", []string{
			"module m", "contains",
			"  pure recursive integer function fact(n) result(r)", "    integer, intent(in) :: n",
			"    if (n <= 1) then", "      r = 1", "    else", "      r = n * fact(n - 1)", "    end if",
			"  end function fact",
			"end module m"},
			""},
		{"a PURE interface for an impure procedure", []string{
			"program p",
			"  interface", "    pure integer function f(x)", "      integer, intent(in) :: x", "    end function f", "  end interface",
			"  print *, f(1)",
			"end program p",
			"integer function f(x)", "  integer, intent(in) :: x", "  f = x", "end function f"},
			"INTERFACEDIFF: this interface body for F doesn't agree with its definition (line 9): it is PURE here, not there"},
		{"an impure procedure to a PURE dummy", []string{
			"module m", "contains",
			"  integer function imp(x)", "    integer, intent(in) :: x", "    imp = x", "  end function imp",
			"  pure integer function ok(x)", "    integer, intent(in) :: x", "    ok = x", "  end function ok",
			"  integer function apply(g, x)",
			"    interface", "      pure integer function g(y)", "        integer, intent(in) :: y", "      end function g", "    end interface",
			"    integer, intent(in) :: x", "    apply = g(x)", "  end function apply",
			"end module m",
			"program p", "  use m", "  intrinsic iabs", "  print *, apply(ok, 1), apply(imp, 2), apply(iabs, 3)", "end program p"},
			"PUREACTUAL: IMP isn't declared PURE, so it can't be passed to G, a PURE dummy procedure"},
		{"a specification function", []string{
			"module m", "contains",
			"  pure integer function width(n)", "    integer, intent(in) :: n", "    width = 2 * n", "  end function width",
			"end module m",
			"program p", "  use m", "  call show(3)",
			"contains",
			"  subroutine show(n)", "    integer, intent(in) :: n", "    character(len=width(n)) :: s", "    real :: v(width(n) + 1)",
			"    s = ' '", "    v = 0", "  end subroutine show",
			"end program p"},
			""},
		{"not a specification function", []string{
			"module m", "contains",
			"  integer function imp(n)", "    integer, intent(in) :: n", "    imp = n", "  end function imp",
			"end module m",
			"program p", "  use m", "  call show(3)",
			"contains",
			"  pure integer function inner(n)", "    integer, intent(in) :: n", "    inner = n", "  end function inner",
			"  subroutine show(n)", "    integer, intent(in) :: n", "    real :: v(imp(n)), w(inner(n))",
			"    v = 0", "    w = 0", "  end subroutine show",
			"end program p"},
			"BOUNDREF: IMP can't be used in an array's bounds: arrays and functions aren't allowed there; " +
				"BOUNDREF: INNER can't be used in an array's bounds: arrays and functions aren't allowed there"},
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
