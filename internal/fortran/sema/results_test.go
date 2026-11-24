package sema

import "testing"

// pointLines is a SEQUENCE type POINT's definition, which each unit of a test
// file repeats as a unit with no host must.
var pointLines = []string{"type point", "  sequence", "  real :: x, y", "end type point"}

// unit is a program unit: its first line, then lines (a type's definition
// among them), then its end.
func unit(first string, lines ...string) []string {
	return append(append([]string{first}, lines...), "end")
}

// file is units, one after another.
func file(units ...[]string) []string {
	var all []string
	for _, u := range units {
		all = append(all, u...)
	}

	return all
}

// TestDerivedResults checks functions of derived types (docs/planning/PHASE-105
// - Fortran 90 derived types.md, subtask 4): each way of giving the value its
// type, and the room the value takes, a longword or quadword when it comes back
// in registers.
func TestDerivedResults(t *testing.T) {
	cases := []struct {
		name  string
		lines []string
		room  int // the value's room in the function's storage
	}{
		{"in the header, the type defined in the body",
			unit("type(point) function f()", append(pointLines, "f%x = 1")...), 8},
		{"in a type statement",
			unit("function f()", append(pointLines, "type(point) :: f", "f = point(1, 2)")...), 8},
		{"by RESULT",
			unit("recursive function f() result(r)", append(pointLines, "type(point) :: r", "r%y = 2")...), 8},
		{"by IMPLICIT",
			unit("function f()", "implicit type(pair) (f)", "type pair", "sequence", "integer(2) :: a", "end type",
				"f%a = 1"), 4},
		{"three bytes, a longword",
			unit("function f()", "type tag", "character(len=3) :: c", "end type", "type(tag) :: f", "f%c = 'abc'"), 4},
		{"five bytes, a quadword",
			unit("function f()", "type tag", "sequence", "character(len=5) :: c", "end type", "type(tag) :: f",
				"f%c = 'abcde'"), 8},
		{"twelve bytes, a hidden argument's",
			unit("function f()", "type tag", "real :: a(3)", "end type", "type(tag) :: f", "f%a(1) = 1"), 0},
	}

	for _, c := range cases {
		prog, ms := analyzeFreeWith(t, Options{F90: true}, c.lines...)
		if len(ms) != 0 {
			t.Errorf("%s: %s", c.name, messageText(ms))

			continue
		}

		r := prog.Units[0].Result
		if r.Type.Kind != Record || r.Struct == nil || r.Type.Struct != r.Struct {
			t.Errorf("%s: the value is %s, not an object of a derived type", c.name, r.Type)

			continue
		}

		room := r.room()
		if r.HiddenValue() {
			room = 0
		}

		if room != c.room {
			t.Errorf("%s: the value takes %d bytes, want %d", c.name, room, c.room)
		}
	}
}

// TestDerivedArguments checks the arguments and values of derived types passed
// between a file's units: the same definition, or two SEQUENCE definitions
// agreeing, pass; two definitions of a non-SEQUENCE type's name, a record, or a
// number don't.
func TestDerivedArguments(t *testing.T) {
	plain := []string{"type point", "  real :: x, y", "end type point"}
	takes := unit("subroutine takes(p)", append(append([]string{}, pointLines...), "type(point) :: p")...)
	takesPlain := unit("subroutine takes(p)", append(append([]string{}, plain...), "type(point) :: p")...)
	half := unit("function half(p)", append(append([]string{}, pointLines...), "type(point) :: half, p", "half = p")...)
	halfPlain := unit("function half(p)", append(append([]string{}, plain...), "type(point) :: half, p", "half = p")...)

	cases := []struct {
		name  string
		lines []string
		want  string
	}{
		{"SEQUENCE types agreeing", file(
			unit("program main", append(append([]string{}, pointLines...),
				"type(point) :: a, half", "external half", "a = half(a)", "call takes(half(a))")...),
			half, takes), ""},
		{"the type of a host", file(
			unit("program main", append(append([]string{}, plain...), "type(point) :: a", "a = half(a)", "contains",
				"type(point) function half(p)", "type(point) :: p", "half = p", "end function")...)), ""},
		{"a dummy procedure", file(
			unit("subroutine s(f, a)", append(append([]string{}, pointLines...),
				"type(point) :: f, a, b", "external f", "b = f(a)")...)), ""},
		{"separate non-SEQUENCE definitions", file(
			unit("program main", append(append([]string{}, plain...), "type(point) :: a", "call takes(a)")...),
			takesPlain),
			"SEPARATEARG: argument 1 of TAKES is of the derived type POINT defined at line 2, but its dummy argument P " +
				"(line 8) is of the type defined at line 9: two definitions are one type only if both have SEQUENCE " +
				"and the same components"},
		{"a function's separate definitions", file(
			unit("program main", append(append([]string{}, plain...), "type(point) :: a, half", "external half",
				"a = half(a)")...),
			halfPlain),
			"SEPARATEFUNC: the function HALF (line 9) has a value of the derived type POINT defined at line 10, but " +
				"this unit gives it the type defined at line 2: two definitions are one type only if both have " +
				"SEQUENCE and the same components; SEPARATEARG: argument 1 of HALF is of the derived type POINT " +
				"defined at line 2, but its dummy argument P (line 9) is of the type defined at line 10: two " +
				"definitions are one type only if both have SEQUENCE and the same components"},
		{"a function used as another type", file(
			unit("program main", "real :: half", "print *, half(1.0)"), half),
			"FUNCTYPE: the function HALF is TYPE(POINT) (line 5), but is used here as REAL; TYPEARG: argument 1 of " +
				"HALF is a REAL, but its dummy argument P (line 5) is an object of the derived type POINT: an object " +
				"goes only to a dummy argument of its own type"},
		{"a number to an object", file(unit("program main", "call takes(1.0)"), takes),
			"TYPEARG: argument 1 of TAKES is a REAL, but its dummy argument P (line 4) is an object of the derived " +
				"type POINT: an object goes only to a dummy argument of its own type"},
		{"an object to a number", file(
			unit("program main", append(append([]string{}, pointLines...), "type(point) :: a", "call nums(a)")...),
			unit("subroutine nums(x)", "real :: x(2)")),
			"TYPEARG: argument 1 of NUMS is an object of the derived type POINT, but its dummy argument X (line 9) is " +
				"a REAL: an object goes only to a dummy argument of its own type"},
		{"a record to an object", file(
			unit("program main", "structure /point/", "real :: x, y", "end structure", "record /point/ r",
				"call takes(r)"),
			takes),
			"TYPEARG: argument 1 of TAKES is a record of the structure /POINT/, but its dummy argument P (line 8) is an " +
				"object of the derived type POINT: an object goes only to a dummy argument of its own type"},
		{"an entry point of another type", unit("function f()", append(append([]string{}, pointLines...),
			"type(point) :: f", "real :: g", "f%x = 1", "return", "entry g()", "g = 1")...),
			"ENTRYDERIVED: the entry point G, REAL, can't go with the function F, TYPE(POINT): a function of a " +
				"derived type has entry points only of the same type"},
		{"a statement function (Phase 113)", unit("program main", append(append([]string{}, pointLines...),
			"type(point) :: sf, a, b", "real :: v", "sf(v) = point(v, v)", "a = sf(1.0)")...),
			""},
		{"a statement function's argument of another type", unit("program main", append(append([]string{}, pointLines...),
			"type(point) :: sf, a, b", "sf(b) = b", "a = sf(1.0)")...),
			"STMTFUNCARG: argument 1 of the statement function SF is a REAL, but its dummy argument B is TYPE(POINT)"},
		{"arrays of a type and components with rank to assumed-shape dummies", unit("program main",
			append(append([]string{}, plain...), "type tagged", "  integer :: marks(3)", "end type tagged",
				"type(point) :: path(4)", "type(tagged) :: t, tags(2)", "call move(path)", "call move(path(1:3:2))",
				"call nums(path%x)", "call nums(path(2:3)%y)", "call ints(t%marks)", "call ints(tags%marks(2))",
				"call old(path%x, 4)", "call old(path(1:4:2)%y, 2)", "contains",
				"subroutine move(a)", "type(point) :: a(:)", "a%x = a%x + 1.0", "end subroutine",
				"subroutine nums(v)", "real :: v(:)", "end subroutine",
				"subroutine ints(v)", "integer :: v(:)", "end subroutine",
				"subroutine old(v, n)", "real :: v(n)", "end subroutine")...), ""},
		{"an object to an assumed-shape dummy", unit("program main",
			append(append([]string{}, plain...), "type(point) :: p", "call move(p)", "contains",
				"subroutine move(a)", "type(point) :: a(:)", "end subroutine")...),
			"ARGRANK: argument 1 of MOVE is a scalar, but its dummy argument A is an array of rank 1 (line 8)"},
		{"an array of a type to a scalar dummy", unit("program main",
			append(append([]string{}, plain...), "type(point) :: path(3)", "call one(path)", "contains",
				"subroutine one(a)", "type(point) :: a", "end subroutine")...),
			"ARGRANK: argument 1 of ONE is an array of rank 1, but its dummy argument A is a scalar (line 8)"},
		{"a header's type undefined", unit("type(nothing) function f()"),
			"NOTYPEDEF: there is no derived type named NOTHING defined before this statement"},
		{"a header's type given again", unit("type(point) function f()", append(append([]string{}, pointLines...),
			"type(point) :: f", "f%x = 1")...),
			"DUPDECL: F's type is already declared, at line 1"},
	}

	for _, c := range cases {
		_, ms := analyzeFreeWith(t, Options{F90: true}, c.lines...)
		if got := messageText(ms); got != c.want {
			t.Errorf("%s:\n got %q\nwant %q", c.name, got, c.want)
		}
	}
}
