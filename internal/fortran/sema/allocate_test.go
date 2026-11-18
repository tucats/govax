package sema

import "testing"

// TestAllocatable checks allocatable arrays' declarations, ALLOCATE,
// DEALLOCATE, and ALLOCATED (allocate.go): nothing for a program that is
// right, and each message for a mistake.
func TestAllocatable(t *testing.T) {
	prog := func(lines ...string) []string {
		return append(append([]string{"program p"}, lines...), "end program")
	}

	cases := []struct {
		name  string
		lines []string
		want  string
	}{
		{"the attribute", prog("real, allocatable :: a(:), b(:, :)", "allocate (a(10), b(0:2, 3))"), ""},
		{"the statement", prog("real a(:)", "allocatable a", "allocate (a(3))"), ""},
		{"the statement with ::", prog("integer :: n", "allocatable :: a(:)", "allocate (a(n))"), ""},
		{"the statement and DIMENSION", prog("allocatable :: a", "dimension a(:, :)", "allocate (a(2, 2))"), ""},
		{"DIMENSION as an attribute", prog("real, allocatable, dimension(:) :: a", "allocate (a(-1:1))"), ""},
		{"DEALLOCATE", prog("real, allocatable :: a(:), b(:)", "deallocate (a, b)"), ""},
		{"ALLOCATED", prog("real, allocatable :: a(:)", "if (.not. allocated(a)) allocate (a(1))"), ""},
		{"ALLOCATED by keyword", prog("real, allocatable :: a(:)", "l = allocated(array=a)"), ""},
		{"elements and wholes", prog("real, allocatable :: a(:)", "allocate (a(4))", "a = 1", "a(2) = a(1) + sum(a)",
			"print *, a, size(a), lbound(a), a(2:3)"), ""},
		{"a recursive unit's", []string{"recursive subroutine r(n)", "real, allocatable :: w(:)", "allocate (w(n))",
			"end subroutine"}, ""},
		{"SAVE", prog("real, allocatable, save :: a(:)"), ""},

		{"a deferred shape not allocatable", prog("real a(:)"), "DEFERREDNOTALLOC: A has a deferred shape, (:), so it must be ALLOCATABLE"},
		{"colons and bounds", prog("real, allocatable :: a(:, 3)"), "DEFERREDMIX: the bounds of A must be all colons or none"},
		{"bounds on an allocatable", prog("real, allocatable :: a(3)"),
			"ALLOCSHAPE: A is ALLOCATABLE, so it must be an array whose bounds are all colons, (:)"},
		{"an allocatable scalar", prog("real, allocatable :: x"),
			"ALLOCSHAPE: X is ALLOCATABLE, so it must be an array whose bounds are all colons, (:)"},
		{"a dummy", []string{"subroutine s(a)", "real, allocatable :: a(:)", "end subroutine"},
			"ALLOCDUMMY: A can't be ALLOCATABLE: it is a dummy argument"},
		{"a function's value", []string{"function f()", "real, allocatable :: f", "end function"},
			"ALLOCDUMMY: F can't be ALLOCATABLE: it is a function's value"},
		{"in COMMON", prog("real, allocatable :: a(:)", "common /c/ a"),
			"ALLOCATTR: the allocatable array A can't be in a common block"},
		{"in EQUIVALENCE", prog("real, allocatable :: a(:)", "equivalence (a, x)"),
			"ALLOCATTR: the allocatable array A can't be in an EQUIVALENCE list"},
		{"in DATA", prog("real, allocatable :: a(:)", "data a /1.0/"),
			"ALLOCATTR: the allocatable array A can't be given an initial value"},
		{"in a NAMELIST group", prog("real, allocatable :: a(:)", "namelist /g/ a"),
			"ALLOCATTR: the allocatable array A can't be in a NAMELIST group"},
		{"with PARAMETER", prog("real, allocatable, parameter :: a(:) = 1"),
			"ATTRPAIR: the ALLOCATABLE and PARAMETER attributes can't go together; " +
				"ALLOCATTR: the allocatable array A can't be given an initial value"},
		{"twice", prog("real, allocatable :: a(:)", "allocatable a"),
			"ATTRAGAIN: A has the ALLOCATABLE attribute already"},
		{"an assumed shape", []string{"subroutine s(a)", "real a(:)", "end subroutine"}, ""},
		{"a lower bound", prog("real, allocatable :: a(0:)"),
			"ALLOCSHAPE: A is ALLOCATABLE, so it must be an array whose bounds are all colons, (:)"},

		{"ALLOCATE of an array not allocatable", prog("real a(3)", "allocate (a(3))"), "NOTALLOCATABLE: A is not an allocatable array"},
		{"DEALLOCATE of a variable", prog("deallocate (x)"), "NOTALLOCATABLE: X is not an allocatable array"},
		{"ALLOCATED of an array", prog("real a(3)", "l = allocated(a)"), "NOTALLOCATABLE: A is not an allocatable array"},
		{"too few bounds", prog("real, allocatable :: a(:, :)", "allocate (a(3))"),
			"ALLOCBOUNDS: in ALLOCATE, A needs 2 bounds, each a value or two separated by a colon"},
		{"a bound of *", prog("real, allocatable :: a(:)", "allocate (a(*))"),
			"ALLOCBOUNDS: in ALLOCATE, A needs 1 bound, each a value or two separated by a colon"},
		{"a bound of :", prog("real, allocatable :: a(:)", "allocate (a(:))"),
			"ALLOCBOUNDS: in ALLOCATE, A needs 1 bound, each a value or two separated by a colon"},
		{"a bound not a number", prog("real, allocatable :: a(:)", "character c", "allocate (a(c))"),
			"CHARNUMBER: a CHARACTER*1 value can't be used as a number or a logical value"},
		{"a bound that uses an object", prog("real, allocatable :: a(:), b(:)", "allocate (a(3), b(size(a)))"),
			"ALLOCSELF: a bound in ALLOCATE can't use A, which the same statement allocates or deallocates"},
		{"an object twice", prog("real, allocatable :: a(:)", "allocate (a(3), a(4))"),
			"ALLOCOBJECT: A is named twice in one ALLOCATE statement"},
		{"STAT=", prog("real, allocatable :: a(:)", "allocate (a(3), stat=i)", "deallocate (a, stat=i)"), ""},
		{"STAT= an element", prog("real, allocatable :: a(:)", "integer*2 k(2)", "allocate (a(3), stat=k(2))"), ""},
		{"STAT= not an integer", prog("real, allocatable :: a(:)", "allocate (a(3), stat=x)"),
			"BADSTAT: a STAT= specifier must name an INTEGER variable or array element"},
		{"STAT= a constant", prog("real, allocatable :: a(:)", "integer, parameter :: n = 1", "allocate (a(3), stat=n)"),
			"BADSTAT: a STAT= specifier must name an INTEGER variable or array element"},
		{"STAT= an object's element", prog("integer, allocatable :: a(:)", "deallocate (a, stat=a(1))"),
			"STATSELF: the STAT= specifier can't use A, which the same statement allocates or deallocates"},
		{"STAT= a DO variable", prog("real, allocatable :: a(:)", "do i = 1, 2", "allocate (a(3), stat=i)", "end do"),
			"DOVARSET: I is the control variable of the DO loop at line 3, which can't change it"},
		{"of records", prog("structure /s/", "integer i", "end structure", "record /s/ r(:)", "allocatable r"),
			"NOTIMPL: an allocatable array of records is not implemented yet"},
	}

	for _, c := range cases {
		if got := messageText(analyzeFree(t, c.lines...)); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

// TestFreedAtReturn checks which allocatable arrays a unit's returns
// deallocate (Symbol.FreedAtReturn): a subprogram's own, not saved by SAVE (by
// name or with no list) or STATIC; never a main program's, and never a host's
// that an internal procedure uses.
func TestFreedAtReturn(t *testing.T) {
	prog, messages := analyzeFreeWith(t, Options{},
		"program p",
		"real, allocatable :: m(:)",
		"contains",
		"subroutine inner",
		"real, allocatable :: i(:)",
		"allocate (m(1), i(1))",
		"end subroutine",
		"end program",
		"subroutine s",
		"real, allocatable :: a(:), b(:), c(:)",
		"save b",
		"static c",
		"end subroutine",
		"function f()",
		"real, allocatable :: a(:)",
		"save",
		"f = 1",
		"end function",
		"recursive subroutine r",
		"real, allocatable :: a(:)",
		"end subroutine")

	if len(messages) != 0 {
		t.Fatalf("messages: %s", messageText(messages))
	}

	want := map[string]map[string]bool{
		"P":     {"M": false},
		"INNER": {"I": true, "M": false},
		"S":     {"A": true, "B": false, "C": false},
		"F":     {"A": false},
		"R":     {"A": true},
	}

	for _, u := range prog.Units {
		for name, freed := range want[u.Name] {
			s := u.Lookup(name)
			if s == nil {
				t.Errorf("%s: no symbol %s", u.Name, name)

				continue
			}

			if s.FreedAtReturn != freed {
				t.Errorf("%s's %s: FreedAtReturn %v, want %v", u.Name, name, s.FreedAtReturn, freed)
			}
		}

		delete(want, u.Name)
	}

	for name := range want {
		t.Errorf("no unit %s", name)
	}
}
