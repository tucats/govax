package sema

import (
	"strconv"
	"strings"
	"testing"
)

// seqProgram is structProgram with two more types: MIXED, a SEQUENCE type with
// an INTEGER*2 component (the other class of EQUIVALENCE's), and NAME, a
// character sequence type.
func seqProgram(lines ...string) []string {
	return structProgram(append([]string{
		"type mixed",
		"  sequence",
		"  integer*2 :: i",
		"  real :: r",
		"end type mixed",
		"type name",
		"  sequence",
		"  character(len=4) :: first, last",
		"end type name",
	}, lines...)...)
}

// TestDataDerived checks the Initials DATA makes of derived types
// (datatypes.go): a constructor for a whole object, an array's elements, a
// component, an array component's elements, a component of each element, a
// component's substring, and implied-DO lists of components.
func TestDataDerived(t *testing.T) {
	cases := []struct {
		name  string
		lines []string
		want  string
	}{
		{"a whole object", []string{"type(pair) :: b", "data b / pair(1, 2) /"},
			"B%FIRST+0=1 B%SECOND+4=2"},
		{"a nested object", []string{"type(label) :: l", "data l / label('ab', (/ 3, 4 /), point(1, 2)) /"},
			"L%TEXT+0='ab  ' L%MARKS(1)+4=3 L%MARKS(2)+8=4 L%AT%X+12=1.0 L%AT%Y+16=2.0"},
		{"an expression in a constructor", []string{"type(point) :: a", "data a / point(1.0 / 2, -3) /"},
			"A%X+0=0.5 A%Y+4=-3.0"},
		{"a type statement's slashes", []string{"type(pair) b / pair(3, 4) /"},
			"B%FIRST+0=3 B%SECOND+4=4"},
		{"an array of objects", []string{"type(pair) :: b(2)", "data b / 2*pair(5, 6) /"},
			"B(1)%FIRST+0=5 B(1)%SECOND+4=6 B(2)%FIRST+8=5 B(2)%SECOND+12=6"},
		{"components", []string{"type(label) :: l", "data l%text, l%marks(2), l%at%y / 'w', 7, 8 /"},
			"L%TEXT+0='w' L%MARKS(2)+8=7 L%AT%Y+16=8"},
		{"an array component", []string{"type(label) :: l", "data l%marks / 1, 2 /"},
			"L%MARKS(1)+4=1 L%MARKS(2)+8=2"},
		{"a component of each element", []string{"type(pair) :: b(3)", "data b%second / 3*9 /"},
			"B(1)%SECOND+4=9 B(2)%SECOND+12=9 B(3)%SECOND+20=9"},
		{"a component of an element", []string{"type(label) :: l(2)", "data l(2)%at / point(1, 2) /"},
			"L(2)%AT%X+32=1.0 L(2)%AT%Y+36=2.0"},
		{"a component's substring", []string{"type(label) :: l", "data l%text(2:3) / 'xy' /"},
			"L%TEXT(2:3)+1='xy'"},
		{"an implied-DO list", []string{"type(pair) :: b(2)", "integer :: i", "data (b(i)%first, b(i)%second, i = 1, 2) / 1, 2, 3, 4 /"},
			"B(1)%FIRST+0=1 B(1)%SECOND+4=2 B(2)%FIRST+8=3 B(2)%SECOND+12=4"},
		{"objects in an implied-DO list", []string{"type(pair) :: b(2)", "integer :: i", "data (b(i), i = 2, 1, -1) / pair(1, 2), pair(3, 4) /"},
			"B(1)%FIRST+0=3 B(1)%SECOND+4=4 B(2)%FIRST+8=1 B(2)%SECOND+12=2"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			prog, ms := analyzeFreeWith(t, Options{F90: true}, seqProgram(c.lines...)...)
			if len(ms) != 0 {
				t.Fatalf("%s", messageText(ms))
			}

			var items []string

			for _, in := range prog.Units[0].Initials {
				items = append(items, in.Item+"+"+strconv.Itoa(in.Offset-in.Sym.Offset)+"="+in.Source)
			}

			if got := strings.Join(items, " "); got != c.want {
				t.Errorf("got  %s\nwant %s", got, c.want)
			}
		})
	}
}

// TestSequenceStorage checks what COMMON, EQUIVALENCE, and DATA say of
// derived types (seqtypes.go, datatypes.go).
func TestSequenceStorage(t *testing.T) {
	cases := []struct {
		name  string
		lines []string
		want  string
	}{
		// COMMON.
		{"a SEQUENCE type in COMMON", []string{"type(pair) :: b", "type(name) :: n", "common /c/ b, n"}, ""},
		{"another type in COMMON", []string{"common /c/ a", "type(point) :: a"},
			"NOSEQUENCE: A is an object of the derived type POINT, which has no SEQUENCE statement, so it can't be in a common block"},

		// EQUIVALENCE.
		{"numbers with numbers", []string{"type(pair) :: b(3)", "integer :: k(6)", "real :: r", "equivalence (b(2), k(3)), (b, r)"}, ""},
		{"numbers with characters", []string{"type(pair) :: b", "type(name) :: n", "character*8 :: c", "equivalence (b, n, c)"}, ""},
		{"another type", []string{"type(point) :: a", "integer :: k(2)", "equivalence (a, k)"},
			"NOSEQUENCE: A is an object of the derived type POINT, which has no SEQUENCE statement, so it can't be in an EQUIVALENCE list"},
		{"a record", []string{"structure /s/", "integer i", "end structure", "record /s/ r", "integer :: k", "equivalence (r, k)"},
			"BADUSE: R is a record, so it can't be in an EQUIVALENCE list"},
		{"a component", []string{"type(pair) :: b", "integer :: k", "equivalence (b%first, k)"},
			"EQUIVCOMPONENT: B%FIRST is a component, which can't be in an EQUIVALENCE list: only its whole object can"},
		{"a component's substring", []string{"type(name) :: n", "character*2 :: c", "equivalence (n%first(1:2), c)"},
			"EQUIVCOMPONENT: N%FIRST(1:2) is a component, which can't be in an EQUIVALENCE list: only its whole object can"},
		{"a nondefault number", []string{"type(pair) :: b", "integer*2 :: k(4)", "equivalence (k, b)"},
			"EQUIVTYPE: K, of type INTEGER*2, can't share storage with B, of type TYPE(PAIR): a sequence type of numbers, logical values, and characters of the default kinds shares storage only with objects of those types"},
		{"the other class with its own type", []string{"type(mixed) :: m, m2(2)", "equivalence (m, m2(2))"}, ""},
		{"the other class with another", []string{"type(mixed) :: m", "real :: r(2)", "equivalence (r, m)"},
			"EQUIVTYPE: R, of type REAL, can't share storage with M, of type TYPE(MIXED): the latter's type has components that aren't all numbers, logical values, and characters of the default kinds, so it shares storage only with objects of its own type"},
		{"no derived type, VAX's rules", []string{"integer*2 :: k", "character*2 :: c", "equivalence (k, c)"}, ""},

		// DATA.
		{"a record in DATA", []string{"structure /s/", "integer i", "end structure", "record /s/ r", "data r /1/"},
			"BADUSE: R is a record, so it can't be in a DATA statement"},
		{"another type's value", []string{"type(point) :: a", "data a / pair(1, 2) /"},
			"DATATYPE: a TYPE(PAIR) constant can't initialize a TYPE(POINT) item"},
		{"a number for an object", []string{"type(point) :: a", "data a / 1.0 /"},
			"DATATYPE: a REAL constant can't initialize a TYPE(POINT) item"},
		{"text for an object", []string{"type(name) :: n", "data n / 'abcdefgh' /"},
			"DATATYPE: a CHARACTER constant can't initialize a TYPE(NAME) item"},
		{"an object for a number", []string{"real :: r", "data r / point(1, 2) /"},
			"DATATYPE: a TYPE(POINT) constant can't initialize a REAL item"},
		{"a value that isn't constant", []string{"type(point) :: a", "real :: r", "data a / point(r, 2) /"},
			"DATAVALUE: an initial value must be a constant, and R is not a constant"},
		{"not an object", []string{"real :: r", "data r%x / 1 /"},
			"DATAFORM: a DATA list can hold only variables, arrays, array elements, substrings, components, and implied-DO lists, and an implied-DO list no variable or array by itself"},
		{"no such component", []string{"type(point) :: a", "data a%z / 1 /"},
			"NOFIELD: the derived type POINT has no component named Z"},
		{"a component of a number", []string{"type(point) :: a", "data a%x%y / 1 /"},
			"FIELDNOTRECORD: the field A%X is a REAL, not a substructure, so it has no field Y"},
		{"two ranked parts", []string{"type(label) :: l(2)", "data l%marks / 4*1 /"},
			"MULTIRANK: the field MARKS is an array after an array, but only one part of a reference may be an array"},
		{"subscripts of a scalar component", []string{"type(point) :: a", "data a%x(1) / 1 /"},
			"NOTARRAY: X is not an array, so it can't have subscripts"},
		{"a component out of range", []string{"type(label) :: l", "data l%marks(3) / 1 /"},
			"DATARANGE: this element is outside the array MARKS"},
		{"a section of a component", []string{"type(label) :: l", "data l%marks(1:2) / 1, 2 /"},
			"NOTIMPL: an array section is not implemented yet"},
		{"a substring of a number", []string{"type(point) :: a", "data a%x(1:2) / 'ab' /"},
			"DATASUBSTR: A%X is REAL, so it can't have a substring"},
		{"too few values", []string{"type(pair) :: b(2)", "data b / pair(1, 2) /"},
			"DATACOUNT: this list has more names than values"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, ms := analyzeFreeWith(t, Options{F90: true}, seqProgram(c.lines...)...)
			if got := messageText(ms); got != c.want {
				t.Errorf("got  %s\nwant %s", got, c.want)
			}
		})
	}
}
