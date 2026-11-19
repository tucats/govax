package sema

import (
	"fmt"
	"strings"
	"testing"
)

// TestDerivedLayouts checks where a derived type's components go (derived.go's
// derivedLayout; the author's answer 1 in docs/planning/PHASE-105 - Fortran 90
// derived types.md): a type without SEQUENCE naturally aligned unless the
// setting says PACKED explicitly, a SEQUENCE type as a STRUCTURE is.
func TestDerivedLayouts(t *testing.T) {
	lines := []string{
		"program p",
		"type plain",
		"  logical(1) :: on",
		"  integer :: n",
		"end type",
		"type seq",
		"  sequence",
		"  logical(1) :: on",
		"  integer :: n",
		"end type",
		"type outer",
		"  type(plain) :: inner(2)",
		"  character(len=3) :: c",
		"  real(8), dimension(2) :: d",
		"end type",
		"type(outer) :: x",
		"x%inner(2)%n = 1",
		"end program",
	}

	natural, _ := (Alignment{}).WithAlign(nil)
	packed, _ := (Alignment{}).WithAlign([]string{"RECORDS=PACKED"})

	cases := []struct {
		name string
		al   Alignment
		want string
	}{
		{"default", Alignment{}, "PLAIN 8: ON+0 N+4; SEQ 5: ON+0 N+1; OUTER 40: INNER+0 C+16 D+24"},
		{"/ALIGN", natural, "PLAIN 8: ON+0 N+4; SEQ 8: ON+0 N+4; OUTER 40: INNER+0 C+16 D+24"},
		{"/ALIGN=RECORDS=PACKED", packed, "PLAIN 5: ON+0 N+1; SEQ 5: ON+0 N+1; OUTER 29: INNER+0 C+10 D+13"},
		{"/NOALIGN", Alignment{}.WithNoAlign(), "PLAIN 5: ON+0 N+1; SEQ 5: ON+0 N+1; OUTER 29: INNER+0 C+10 D+13"},
	}

	for _, c := range cases {
		prog, ms := analyzeFreeWith(t, Options{Align: c.al}, lines...)
		if len(ms) != 0 {
			t.Fatalf("%s: %s", c.name, messageText(ms))
		}

		var parts []string

		for _, st := range prog.Units[0].Structures {
			fields := make([]string, len(st.Fields))
			for i, f := range st.Fields {
				fields[i] = fmt.Sprintf("%s+%d", f.Name, f.Offset)
			}

			parts = append(parts, fmt.Sprintf("%s %d: %s", st.Name, st.Size, strings.Join(fields, " ")))
		}

		if got := strings.Join(parts, "; "); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

// TestDerivedObjects checks what a derived type's object is: a record of its
// structure, whose Type names it, so that two types of one size differ, and
// IMPLICIT TYPE(t) naming a type defined after it.
func TestDerivedObjects(t *testing.T) {
	prog, ms := analyzeFreeWith(t, Options{},
		"subroutine s(q)",
		"  implicit type(a) (q)",
		"  type a",
		"    real :: x",
		"  end type",
		"  type b",
		"    real :: y",
		"  end type",
		"  type(b) :: r",
		"  q%x = r%y",
		"end subroutine",
	)
	if len(ms) != 0 {
		t.Fatalf("%s", messageText(ms))
	}

	u := prog.Units[0]
	q, r := u.scope.byName["Q"], u.scope.byName["R"]

	switch {
	case q.Struct == nil || q.Struct.Name != "A" || q.Type != recordType(q.Struct):
		t.Errorf("Q is %v, %v", q.Type, q.Struct)
	case r.Struct == nil || r.Struct.Name != "B":
		t.Errorf("R is %v, %v", r.Type, r.Struct)
	case q.Type == r.Type:
		t.Errorf("types of A and B are equal: %v", q.Type)
	case q.Type.String() != "TYPE(A)" || q.TypeText() != "TYPE(A)":
		t.Errorf("Q's type is shown as %s, %s", q.Type, q.TypeText())
	}
}

// TestDerivedMessages checks what semantic analysis says of derived types'
// definitions and declarations: nothing for what is right, each message for a
// mistake, and NOTIMPL for what later subtasks compile.
func TestDerivedMessages(t *testing.T) {
	def := []string{"type point", "  real :: x, y", "end type point"}

	program := func(lines ...string) []string {
		return append(append([]string{"program p"}, lines...), "end program")
	}
	withPoint := func(lines ...string) []string {
		return program(append(append([]string{}, def...), lines...)...)
	}

	cases := []struct {
		name  string
		lines []string
		want  string
	}{
		{"right", withPoint("type(point) :: a, b(3)", "a%x = b(2)%y", "a.y = 1"), ""},
		{"a host's type", []string{"program p", "type point", "real x", "end type", "call s", "contains",
			"subroutine s", "type(point) q", "q%x = 1", "end subroutine", "end program"}, ""},
		{"intrinsic name", program("type real", "integer i", "end type"),
			"TYPENAME: a derived type can't be named REAL, an intrinsic type's name"},
		{"access", program("type, public :: t", "integer i", "end type"),
			"TYPEACCESS: PUBLIC is allowed in a derived type's definition only in a module"},
		{"private", program("type t", "private", "integer i", "end type"),
			"TYPEACCESS: PRIVATE is allowed in a derived type's definition only in a module"},
		{"twice", withPoint("type point", "integer i", "end type"),
			"DUPTYPE: a derived type or structure named POINT is already defined, at line 2"},
		{"a structure's name", program("structure /point/", "real x", "end structure", "type point", "real y", "end type"),
			"DUPTYPE: a derived type or structure named POINT is already defined, at line 2"},
		{"empty", program("type t", "end type"), "TYPEEMPTY: the derived type T has no components"},
		{"sequence twice", program("type t", "sequence", "sequence", "integer i", "end type"),
			"TYPETWICE: the derived type T already has a SEQUENCE statement, at line 3"},
		{"sequence late", program("type t", "integer i", "sequence", "end type"),
			"TYPEORDER: a SEQUENCE statement must come before the type's components"},
		{"sequence component", withPoint("type t", "sequence", "type(point) :: p", "end type"),
			"SEQCOMPONENT: T is a SEQUENCE type, so its component P must be too, and the type POINT isn't"},
		{"end name", program("type t", "integer i", "end type u"),
			"ENDTYPENAME: this END TYPE names U, but ends the definition of T"},
		{"attribute", program("type t", "integer, save :: i", "end type"),
			"TYPEATTR: a component can have only the DIMENSION or POINTER attribute, not SAVE"},
		{"statement", program("type t", "integer i", "dimension i(3)", "end type"),
			"TYPESTMT: a derived type's definition holds only component declarations, SEQUENCE, and PRIVATE, not a DIMENSION statement"},
		{"itself", program("type t", "type(t) :: next", "end type"),
			"TYPESELF: a component of the derived type T can't be of that type itself unless it is a POINTER"},
		{"undefined", program("type(point) :: a"),
			"NOTYPEDEF: there is no derived type named POINT defined before this statement"},
		{"defined later", program("type t", "type(point) :: p", "end type", "type point", "real x", "end type"),
			"NOTYPEDEF: there is no derived type named POINT defined before this statement"},
		{"a structure", program("structure /s/", "real x", "end structure", "type(s) :: a"),
			"TYPESTRUCT: S is a structure, not a derived type: a RECORD statement declares its records"},
		{"record of a type", withPoint("record /point/ a"),
			"RECORDTYPE: POINT is a derived type, not a structure: TYPE(POINT) declares its objects"},
		{"a variable's name", withPoint("point = 1.0"),
			"TYPECLASH: POINT names the derived type defined at line 2, so it can't also be a variable"},
		{"a length", withPoint("type(point) :: a*4"), "NOLENGTH: TYPE(POINT) can't be given a length"},
		{"no component", withPoint("type(point) :: a", "a%z = 1"),
			"NOFIELD: the derived type POINT has no component named Z"},
		{"an implicit type missing", program("implicit type(nothing) (q)", "q%x = 1"),
			"NOTYPEDEF: there is no derived type named NOTHING defined before this statement"},
		{"no END TYPE", program("type t", "integer i"),
			"NOENDTYPE: the definition of the derived type T has no END TYPE statement"},
		{"END TYPE alone", program("end type"),
			"NOTYPEEND: this END TYPE statement ends no derived type's definition"},
		{"default initialization", program("type t", "integer :: i = 0", "end type"),
			"NOTIMPL: a component's default initial value is not implemented yet"},
		{"a POINTER component", program("type t", "integer, pointer :: i", "end type"),
			"NOTIMPL: a POINTER component is not implemented yet"},
		{"a function", []string{"type(point) function f()", "type point", "real x", "end type", "f%x = 1", "end function"}, ""},
		{"a function's type", []string{"function f()", "type point", "real x", "end type", "type(point) f", "f%x = 1", "end function"}, ""},
	}

	for _, c := range cases {
		if got := messageText(analyzeFree(t, c.lines...)); got != c.want {
			t.Errorf("%s:\n got %q\nwant %q", c.name, got, c.want)
		}
	}
}
