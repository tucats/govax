package sema

import (
	"fmt"
	"strings"
	"testing"
)

// tagLines are a type TAG with a component of a derived type and an array
// component, after POINT's definition.
var tagLines = []string{
	"type point", "  real :: x, y", "end type point",
	"type tag", "  character(len=4) :: name", "  type(point) :: at", "  integer :: marks(3)", "end type tag",
}

// TestDerivedIO checks derived types in I/O lists and NAMELIST groups
// (docs/planning/PHASE-105 - Fortran 90 derived types.md, subtask 7): an object
// may be in any statement's list, a STRUCTURE's record still only in an
// unformatted one; a NAMELIST group takes a scalar object, not an array of a
// type (Phase 109's), nor a record.
func TestDerivedIO(t *testing.T) {
	with := func(lines ...string) []string {
		return unit("program main", append(append([]string{}, tagLines...), lines...)...)
	}

	cases := []struct {
		name  string
		lines []string
		want  string
	}{
		{"objects in every kind of list", with("type(tag) :: t, ts(2)", "character(len=40) :: line",
			"print *, t, ts", "write (*, '(a)') ts(1:2), t%at", "read *, t, ts(1)", "write (line, *) t", "read (line, *) ts",
			"write (1) t, ts", "read (1) t, ts(2)", "write (*, '(2f5.1)') point(1.0, 2.0)"), ""},
		{"a scalar object in a NAMELIST group", with("type(tag) :: t", "namelist /g/ t", "write (*, g)", "read (*, g)"), ""},
		{"an array of a type in a NAMELIST group", with("type(point) :: ps(2)", "namelist /g/ ps"),
			"NOTIMPL: an array of a derived type in a NAMELIST group is not implemented yet"},
		{"an array component of a type in a NAMELIST group", unit("program main",
			"type point", "  real :: x, y", "end type point", "type path", "  type(point) :: steps(2)", "end type path",
			"type(path) :: p", "namelist /g/ p"),
			"NOTIMPL: an array of a derived type in a NAMELIST group is not implemented yet"},
		{"a record in a formatted list", unit("program main", "structure /s/", "real :: x", "end structure",
			"record /s/ r", "print *, r"),
			"AGGREGATEIO: R is a record as a whole, which only an unformatted I/O list can hold"},
	}

	for _, c := range cases {
		_, ms := analyzeFreeWith(t, Options{F90: true}, c.lines...)
		if got := messageText(ms); got != c.want {
			t.Errorf("%s:\n got %q\nwant %q", c.name, got, c.want)
		}
	}
}

// TestNamelistEntities checks a group's entities for an object of a derived
// type: a header naming the object and counting the entities after it that
// are its own, nested headers among them, then one per ultimate component.
func TestNamelistEntities(t *testing.T) {
	p, ms := analyzeFreeWith(t, Options{F90: true}, unit("program main",
		append(append([]string{}, tagLines...), "type(tag) :: t", "integer :: n", "namelist /g/ n, t, n")...)...)
	if len(ms) != 0 {
		t.Fatalf("messages: %s", messageText(ms))
	}

	var got []string

	for _, e := range p.Units[0].Namelists[0].Entities {
		if e.Ref == nil {
			got = append(got, fmt.Sprintf("%s(%d)", e.Name, e.Parts))
		} else {
			got = append(got, e.Name)
		}
	}

	want := "N T(5) T%NAME T%AT(2) T%AT%X T%AT%Y T%MARKS N"
	if s := strings.Join(got, " "); s != want {
		t.Errorf("entities %s, want %s", s, want)
	}
}
