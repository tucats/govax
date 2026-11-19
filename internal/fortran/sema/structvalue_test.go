package sema

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
)

// structProgram is a program defining POINT (no SEQUENCE), PAIR (SEQUENCE), and
// LABEL (a CHARACTER, an array, and a POINT component), then lines.
func structProgram(lines ...string) []string {
	head := []string{
		"program p",
		"type point",
		"  real :: x, y",
		"end type point",
		"type pair",
		"  sequence",
		"  integer :: first, second",
		"end type pair",
		"type label",
		"  character(len=4) :: text",
		"  integer :: marks(2)",
		"  type(point) :: at",
		"end type label",
	}

	return append(append(head, lines...), "end program")
}

// TestStructConstants checks constructors of constants (structvalue.go): a
// PARAMETER's value with its components converted and its bytes, a component of
// a constant folded, and an initial value's Initials, one per scalar component.
func TestStructConstants(t *testing.T) {
	prog, ms := analyzeFreeWith(t, Options{F90: true}, structProgram(
		"type(point), parameter :: unit = point(1, 0.5)",
		"type(label), parameter :: tag = label('ab', 7, unit)",
		"real, parameter :: half = unit%y, seven = tag%marks(2) + tag%at%x",
		"type(label) :: l = label('wxyz', (/ 3, 4 /), point(2, 3))",
		"print *, half, seven",
		"l = tag",
	)...)
	if len(ms) != 0 {
		t.Fatalf("%s", messageText(ms))
	}

	u := prog.Units[0]
	s := u.scope.byName

	if got, want := DumpExpr(s["TAG"].Value), "LABEL{'ab  ', (/:I 7 7 /), POINT{1.0, 0.5}}"; got != want {
		t.Errorf("TAG is %s, want %s", got, want)
	}

	if got, want := fmt.Sprintf("% X", s["UNIT"].Value.Raw), "80 40 00 00 00 40 00 00"; got != want {
		t.Errorf("UNIT's bytes are %s, want %s", got, want)
	}

	if got := DumpExpr(s["HALF"].Value) + " " + DumpExpr(s["SEVEN"].Value); got != "0.5 8.0" {
		t.Errorf("HALF and SEVEN are %s", got)
	}

	var items []string

	for _, in := range u.Initials {
		items = append(items, in.Item+"+"+strconv.Itoa(in.Offset)+"="+in.Source)
	}

	if got, want := strings.Join(items, " "), "L%TEXT+0='wxyz' L%MARKS(1)+4=3 L%MARKS(2)+8=4 L%AT%X+12=2.0 L%AT%Y+16=3.0"; got != want {
		t.Errorf("L's initial values are %s,\nwant %s", got, want)
	}
}

// TestStructValues checks constructors that aren't constant: a StructValue of
// each component's value converted, built in its target unless a value reads
// the target (Assign.InPlace).
func TestStructValues(t *testing.T) {
	prog, ms := analyzeFreeWith(t, Options{F90: true}, structProgram(
		"type(point) :: a, b",
		"real :: r",
		"integer :: i",
		"a = point(r, i)",
		"a = point(a%y, a%x)",
		"b = point(1, a%x)",
		"a = b",
	)...)
	if len(ms) != 0 {
		t.Fatalf("%s", messageText(ms))
	}

	var got []string

	for _, st := range prog.Units[0].Body {
		if as, ok := st.(*Assign); ok {
			got = append(got, DumpExpr(as.Value)+" "+map[bool]string{true: "in place", false: "moved"}[as.InPlace])
		}
	}

	want := "POINT{R, (REAL I)} in place; POINT{A.Y:+4, A.X:+0} moved; POINT{1.0, A.X:+0} in place; B moved"
	if g := strings.Join(got, "; "); g != want {
		t.Errorf("got  %s\nwant %s", g, want)
	}
}

// TestStructMessages checks what semantic analysis says of constructors,
// assignments, and operators on derived types.
func TestStructMessages(t *testing.T) {
	cases := []struct {
		name  string
		lines []string
		want  string
	}{
		{"right", structProgram("type(point) :: a", "type(pair) :: b", "a = point(1, 2)", "b = pair(a%x, 3)"), ""},
		{"too few", structProgram("type(point) :: a", "a = point(1.0)"),
			"CONSTRCOUNT: the derived type POINT has 2 components, but its constructor gives 1 values"},
		{"too many", structProgram("type(point), parameter :: a = point(1.0, 2.0, 3.0)"),
			"CONSTRCOUNT: the derived type POINT has 2 components, but its constructor gives 3 values"},
		{"a keyword", structProgram("type(point) :: a", "a = point(x=1.0, y=2.0)"),
			"CONSTRKEYWORD: a structure constructor's values have no keywords, as those of the constructor of POINT here"},
		{"a CHARACTER value", structProgram("type(point) :: a", "character*4 c", "a = point(c, 2.0)"),
			"CONSTRVALUE: a CHARACTER*4 value can't be the value of the component X of POINT, which is a REAL value"},
		{"a number for a CHARACTER", structProgram("type(label) :: l", "type(point) :: a", "l = label(1, 2, a)"),
			"CONSTRVALUE: an INTEGER value can't be the value of the component TEXT of LABEL, which is a CHARACTER*4 value"},
		{"another type", structProgram("type(label) :: l", "type(pair) :: b", "l = label('a', 2, b)"),
			"CONSTRVALUE: an object of the derived type PAIR can't be the value of the component AT of LABEL, which is an object of the derived type POINT"},
		{"an array's shape", structProgram("type(label), parameter :: l = label('a', (/ 1, 2, 3 /), point(1, 2))"),
			"NOTCONFORM: the shapes (2) and (3) don't conform"},
		{"an array that isn't constant", structProgram("type(label) :: l", "integer :: m(2)", "l = label('a', m, point(1, 2))"),
			""},
		{"an array of another shape", structProgram("type(label) :: l", "integer :: m(3)", "l = label('a', m, point(1, 2))"),
			"NOTCONFORM: the shapes (2) and (3) don't conform"},
		{"an array for a scalar component", structProgram("type(point) :: a", "real :: v(2)", "a = point(v, 1)"),
			"NOTSCALAR: an array is used where a scalar is required"},
		{"assigned another type", structProgram("type(point) :: a", "type(pair) :: b", "a = b"),
			"RECORDASSIGN: an object of the derived type PAIR can't be assigned to A, which is an object of the derived type POINT: both must be of one structure or type"},
		{"a constant of another type", structProgram("type(pair), parameter :: b = point(1, 2)"),
			"PARAMTYPE: the constant B is TYPE(PAIR), and can't take the value of a TYPE(POINT) expression"},
		{"an initial value of another type", structProgram("type(pair) :: b = point(1, 2)"),
			"INITTYPE: a TYPE(POINT) value can't initialize a TYPE(PAIR) item, which an assignment couldn't store it in"},
		{"an initial value that isn't constant", structProgram("real :: r", "type(point) :: a = point(r, 1)"),
			"DATAVALUE: an initial value must be a constant, and R is not a constant"},
		{"equality", structProgram("type(point) :: a, b", "if (a == b) stop"),
			"NODERIVEDOP: the operator .EQ. has no meaning for TYPE(POINT) unless an interface block gives it one"},
		{"addition", structProgram("type(point) :: a", "a = a + point(1, 1)"),
			"NODERIVEDOP: the operator + has no meaning for TYPE(POINT) unless an interface block gives it one"},
		{"negation of a constant", structProgram("type(point), parameter :: o = point(0, 0)", "type(point) :: a", "a = -o"),
			"NODERIVEDOP: the operator - has no meaning for TYPE(POINT) unless an interface block gives it one"},
		{"a constant as a value", structProgram("type(point), parameter :: o = point(0, 0)", "real :: r", "r = o"),
			"AGGREGATE: O is a record as a whole, which only an assignment, an argument, or an unformatted I/O list can use"},
		{"no such component of a constant", structProgram("type(point), parameter :: o = point(0, 0)", "real :: r", "r = o%z"),
			"NOFIELD: the derived type POINT has no component named Z"},
		{"an array constant", structProgram("type(point), parameter :: o(2) = point(0, 0)"), ""},
		{"an array constant's component assigned", structProgram("type(point), parameter :: o(2) = point(0, 0)", "integer :: i",
			"o(i)%x = 1"), "BADUSE: O is a constant, so it can't be assigned a value"},
		{"a constant's component assigned", structProgram("type(point), parameter :: o = point(0, 0)", "o%x = 1"),
			"BADUSE: O is a constant, so it can't be assigned a value"},
		{"a constant's component read into", structProgram("type(point), parameter :: o = point(0, 0)", "read *, o%x"),
			"BADUSE: O is a constant, so it can't be assigned a value"},
		{"an array initial value of another type", structProgram("type(pair) :: b(2) = (/ point(1, 2), point(3, 4) /)"),
			"INITTYPE: a TYPE(POINT) value can't initialize a TYPE(PAIR) item, which an assignment couldn't store it in"},
		{"an array initial value's shape", structProgram("type(point) :: b(3) = (/ point(1, 2), point(3, 4) /)"),
			"NOTCONFORM: the shapes (3) and (2) don't conform"},
		{"an array of a type as a number", structProgram("type(point) :: a(2)", "real :: r(2)", "r = a"),
			"AGGREGATE: A is a record as a whole, which only an assignment, an argument, or an unformatted I/O list can use"},
		{"an array of a type with an operator", structProgram("type(point) :: a(2)", "a = a + a"),
			"NODERIVEDOP: the operator + has no meaning for TYPE(POINT) unless an interface block gives it one"},
		{"two types in a constructor", structProgram("type(point) :: a(2)", "type(pair) :: b", "a = (/ point(1, 2), b /)"),
			"CONSTRUCTTYPE: an array constructor's values must have one type, and this one is TYPE(PAIR) where the first is TYPE(POINT)"},
		{"MERGE of two types", structProgram("type(point) :: a(2)", "type(pair) :: b", "a = merge(a, b, .true.)"),
			"MERGEARGS: the TSOURCE and FSOURCE of MERGE must have one type and length, and they are TYPE(POINT) and TYPE(PAIR)"},
		{"EOSHIFT without a BOUNDARY", structProgram("type(point) :: a(2)", "a = eoshift(a, 1)"),
			"BADARG: the intrinsic function EOSHIFT's arguments must be of a type with a default BOUNDARY, or given one, and this one is TYPE(POINT)"},
	}

	for _, c := range cases {
		if got := messageText(analyzeFree(t, c.lines...)); got != c.want {
			t.Errorf("%s:\n got %q\nwant %q", c.name, got, c.want)
		}
	}
}
