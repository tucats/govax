package sema

import "testing"

// TestOperators checks defined operators and assignment (operators.go): what
// each operation and assignment becomes, and each message.
func TestOperators(t *testing.T) {
	dist := []string{"interface operator(.dist.)", "real function dist(p, q)", "real, intent(in) :: p(2), q(2)",
		"end function", "real function sdist(x, y)", "real, intent(in) :: x, y", "end function", "end interface"}
	plus := []string{"interface operator(+)", "logical function lor(p, q)", "logical, intent(in) :: p, q",
		"end function", "end interface"}
	assign := []string{"interface assignment(=)", "subroutine fromlog(n, l)", "character*(*), intent(out) :: n",
		"logical, intent(in) :: l", "end subroutine", "end interface"}
	program := func(blocks []string, lines ...string) []string {
		return append(append(append([]string{"program p"}, blocks...), lines...), "end program")
	}

	cases := []struct {
		name  string
		lines []string
		want  string
	}{
		{"a defined operator", program(dist, "real a(2), b(2)", "d = a .dist. b", "e = 1.0 .dist. 2.0"), ""},
		{"none fits", program(dist, "d = 1 .dist. 2"),
			"DEFOPNONE: no function for the operator .DIST. takes an INTEGER and an INTEGER"},
		{"no block", program(nil, "d = 1.0 .dist. 2.0"),
			"OPUNDEFINED: .DIST. is no operator here: no interface block gives it a meaning"},
		{"unary", []string{"program p", "interface operator(.neg.)", "real function neg(x)", "real, intent(in) :: x",
			"end function", "end interface", "y = .neg. 2.0", "end program"}, ""},
		{"an intrinsic operator extended", program(plus, "logical l", "l = l + .true.", "y = 1.0 + 2.0"), ""},
		{"an intrinsic meaning kept", program(plus, "integer i", "i = 1 + 2"), ""},
		{"no fit, so VAX FORTRAN's meaning", program(plus, "logical l", "y = l + 1.0"), ""},
		{"a meaning Fortran gives", []string{"program p", "interface operator(+)", "real function add(x, y)",
			"real, intent(in) :: x, y", "end function", "end interface", "end program"},
			"DEFOPINTR: ADD takes operands the operator + already has a meaning for"},
		{"DEC's meaning taken over", []string{"program p", "interface operator(.and.)", "integer function band(i, j)",
			"integer, intent(in) :: i, j", "end function", "end interface", "k = 1 .and. 3", "end program"}, ""},
		{"a subroutine for an operator", []string{"program p", "interface operator(.op.)", "subroutine s(x)",
			"real, intent(in) :: x", "end subroutine", "end interface", "end program"},
			"DEFOPFUNC: S is a subroutine, but a procedure for OPERATOR(.OP.) must be a function"},
		{"no INTENT(IN)", []string{"program p", "interface operator(.op.)", "real function f(x)", "real x",
			"end function", "end interface", "end program"},
			"DEFOPDUMMY: F, a function for OPERATOR(.OP.), must have one or two dummy arguments, each data, INTENT(IN), and not OPTIONAL"},
		{"unary .AND.", []string{"program p", "interface operator(.and.)", "real function f(x)", "real, intent(in) :: x",
			"end function", "end interface", "end program"},
			"DEFOPCOUNT: F, a function for OPERATOR(.AND.), takes 1 operands, which that operator never has"},
		{"two alike", []string{"program p", "interface operator(.op.)", "real function f(x)", "real, intent(in) :: x",
			"end function", "real function g(y)", "real, intent(in) :: y", "end function", "end interface", "end program"},
			"OPAMBIG: the procedures F (line 3) and G for OPERATOR(.OP.) can't be told apart by their operands"},
		{"<= is .LE.", []string{"program p", "interface operator(<=)", "logical function le(p, q)",
			"logical, intent(in) :: p, q", "end function", "end interface", "logical a", "a = a .le. a", "end program"}, ""},
		{"defined assignment", program(assign, "character*5 c", "c = .true."), ""},
		{"an assignment Fortran defines", program(assign, "character*5 c", "c = 'abc'"), ""},
		{"assignment's subroutine", []string{"program p", "interface assignment(=)", "subroutine s(x, y)",
			"logical, intent(in) :: x", "real, intent(in) :: y", "end subroutine", "end interface", "end program"},
			"DEFASSIGNDUMMY: S, a subroutine for ASSIGNMENT(=), must have two dummy arguments, not OPTIONAL, the first INTENT(OUT) or INTENT(INOUT) and the second INTENT(IN)"},
		{"redefining assignment", []string{"program p", "interface assignment(=)", "subroutine s(x, y)",
			"real, intent(out) :: x", "integer, intent(in) :: y", "end subroutine", "end interface", "end program"},
			"DEFASSIGNINTR: S defines assigning a REAL an INTEGER, which Fortran defines already"},
		{"the host's operator", program(dist, "call s", "contains", "subroutine s", "print *, 1.0 .dist. 2.0",
			"end subroutine"), ""},
	}

	for _, c := range cases {
		if got := messageText(analyzeFree(t, c.lines...)); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

// TestOperandArrays checks arrays as a defined operation's operands and a
// defined assignment's value (docs/planning/PHASE-107 - Fortran 90 pointers.md,
// subtask 8): each is passed as a reference's argument is, an array constant or
// expression to an assumed-shape dummy, and a record's value may be an array
// when an ASSIGNMENT(=) interface is there to take it.
func TestOperandArrays(t *testing.T) {
	// mod is a module of a type T with * extended to T and a vector, and
	// assignment from a vector (FILL) or from a REAL (PUT, scalar only when
	// scalar is set), and a program using it.
	mod := func(scalar bool, lines ...string) []string {
		fill := []string{"subroutine fill(a, x)", "type(t), intent(out) :: a", "real, intent(in) :: x(:)",
			"a%v = sum(x)", "end subroutine"}
		name := "fill"

		if scalar {
			fill = []string{"subroutine put(a, x)", "type(t), intent(out) :: a", "real, intent(in) :: x",
				"a%v = x", "end subroutine"}
			name = "put"
		}

		head := append([]string{"module m", "type t", "real :: v", "end type", "interface operator (*)",
			"module procedure tv", "end interface", "interface assignment (=)", "module procedure " + name,
			"end interface", "contains", "function tv(a, x) result(y)", "type(t), intent(in) :: a",
			"real, intent(in) :: x(:)", "real :: y(size(x))", "y = a%v * x", "end function"}, fill...)
		head = append(head, "end module", "program p", "use m", "type(t) :: a, b", "real :: x(3), y(3)")

		return append(append(head, lines...), "end program")
	}

	cases := []struct {
		name  string
		lines []string
		want  string
	}{
		{"operands", mod(false, "y = a * (/ 1.0, 2.0, 3.0 /)", "y = a * (x + 1.0)", "y = a * x(3:1:-1)", "y = a * x"), ""},
		{"assigned arrays", mod(false, "a = (/ 1.0, 2.0 /)", "a = x", "a = x * 2.0", "a = x(1:2)", "b = a"), ""},
		{"an array to a scalar's assignment", mod(true, "a = 2.0", "a = x + 1.0"),
			"NOTSCALAR: an array is used where a scalar is required"},
	}

	for _, c := range cases {
		if got := messageText(analyzeFree(t, c.lines...)); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}
