package sema

import "testing"

// TestPointers checks scalar pointers' declarations, pointer assignment,
// NULLIFY, and ASSOCIATED (pointer.go): nothing for a program that is right,
// and each message for a mistake, or NOTIMPL for what a later subtask of
// docs/planning/PHASE-107 - Fortran 90 pointers.md compiles.
func TestPointers(t *testing.T) {
	prog := func(lines ...string) []string {
		return append(append([]string{"program main"}, lines...), "end program")
	}

	// decl declares the program's usual names: pointers P and Q to INTEGER, a
	// TARGET N, a plain M, and a CHARACTER*4 pointer C with targets.
	decl := []string{"integer, pointer :: p, q", "integer, target :: n, v(5)", "integer :: m",
		"character(len=4), pointer :: c", "character(len=8), target :: s", "character(len=4), target :: s4"}
	with := func(lines ...string) []string { return prog(append(append([]string{}, decl...), lines...)...) }

	cases := []struct {
		name  string
		lines []string
		want  string
	}{
		{"the attribute", with("p => n", "p = 3", "m = p + 1"), ""},
		{"the statement", prog("integer x", "pointer x", "nullify (x)"), ""},
		{"the statement with ::", prog("pointer :: x", "real x", "nullify (x)"), ""},
		{"an element of a TARGET", with("p => v(3)"), ""},
		{"another pointer", with("p => n", "q => p"), ""},
		{"a substring", with("c => s(2:5)", "c => s4", "c = 'ABCD'"), ""},
		{"a substring of run-time bounds", with("c => s(m:m+3)"), ""},
		{"NULLIFY of several", with("nullify (p, q, c)"), ""},
		{"ASSOCIATED", with("if (associated(p)) m = 1", "l = associated(p, n)", "l = associated(p, q)"), ""},
		{"ASSOCIATED by keyword", with("l = associated(target=n, pointer=p)"), ""},
		{"a component of a TARGET", prog("type t", "integer i", "end type", "type(t), target :: r",
			"integer, pointer :: p", "p => r%i"), ""},
		{"a pointer to a record and its component", prog("type t", "integer i", "end type", "type(t), target :: r",
			"type(t), pointer :: pr", "integer, pointer :: p", "pr => r", "p => pr%i", "pr%i = 2"), ""},
		{"in COMMON", prog("integer, pointer :: p", "common /b/ p"), ""},
		{"a recursive unit's", []string{"recursive subroutine r(n)", "integer, target :: n", "integer, pointer :: p",
			"p => n", "end subroutine"}, ""},
		{"as an actual argument", with("p => n", "call sub(p)", "m = abs(p)"), ""},
		{"in I/O", with("p => n", "read *, p", "print *, p", "c => s4", "write (c, '(I4)') 1"), ""},

		{"bounds on an array pointer", prog("real, pointer :: a(3)"),
			"POINTERSHAPE: A is an array POINTER, so its bounds must all be colons, (:)"},
		{"POINTER and ALLOCATABLE", prog("real, pointer, allocatable :: a(:)"),
			"ATTRPAIR: the POINTER and ALLOCATABLE attributes can't go together; NOTIMPL: an array pointer is not implemented yet"},
		{"POINTER and TARGET", prog("real, target :: x", "pointer x"),
			"ATTRCONFLICT: X has the TARGET attribute, so it can't have the POINTER attribute too"},
		{"an initial value", prog("integer, target :: n", "integer, pointer :: p = 1"),
			"POINTERATTR: the pointer P can't be given an initial value"},
		{"DATA", prog("integer, pointer :: p", "data p /1/"), "POINTERATTR: the pointer P can't be given an initial value"},
		{"NAMELIST", prog("integer, pointer :: p", "namelist /g/ p"), "POINTERATTR: the pointer P can't be in a NAMELIST group"},
		{"EQUIVALENCE", prog("integer, pointer :: p", "integer k", "equivalence (p, k)"),
			"POINTERATTR: the pointer P can't be in an EQUIVALENCE list"},
		{"=> of what isn't a pointer", with("m => n"), "NOTPOINTER: M is not a pointer"},
		{"=> of an element", with("v(1) => n"), "NOTPOINTER: V(1) is not a pointer"},
		{"NULLIFY of what isn't a pointer", with("nullify (p, m)"), "NOTPOINTER: M is not a pointer"},
		{"ASSOCIATED of what isn't a pointer", with("l = associated(m)"), "NOTPOINTER: M is not a pointer"},
		{"a target without TARGET", with("p => m"),
			"NOTTARGET: M is neither a TARGET, a part of one, nor a pointer, so no pointer may point at it"},
		{"a constant target", with("p => 5"),
			"NOTTARGET: 5 is neither a TARGET, a part of one, nor a pointer, so no pointer may point at it"},
		{"an expression", with("p => n + 1"),
			"NOTTARGET: the expression is neither a TARGET, a part of one, nor a pointer, so no pointer may point at it"},
		{"a function's reference", with("p => abs(n)"),
			"POINTERFUNC: the target ABS(N) is a function reference, and only a function whose result is a pointer gives a target"},
		{"another type", prog("integer, pointer :: p", "real, target :: x", "p => x"),
			"POINTERTYPE: the pointer P points at INTEGER, so its target can't be REAL"},
		{"another kind", prog("integer, pointer :: p", "integer(2), target :: k", "p => k"),
			"POINTERTYPE: the pointer P points at INTEGER, so its target can't be INTEGER*2"},
		{"another length", with("c => s"),
			"POINTERTYPE: the pointer C points at CHARACTER*4, so its target can't be CHARACTER*8"},
		{"an array", with("p => v"), "POINTERTYPE: the pointer P points at INTEGER, so its target can't be an array of INTEGER"},
		{"ASSOCIATED of another type", prog("integer, pointer :: p", "real, target :: x", "l = associated(p, x)"),
			"POINTERTYPE: the pointer P points at INTEGER, so its target can't be REAL"},
		{"ASSOCIATED of three", with("l = associated(p, n, n)"),
			"INTRINARGS: the intrinsic function ASSOCIATED takes 1 or 2 arguments, not 3"},

		{"an array pointer, subtask 3's", prog("real, pointer :: a(:)"), "NOTIMPL: an array pointer is not implemented yet"},
		{"ALLOCATE of a pointer, subtask 4's", with("allocate (p)"),
			"NOTIMPL: ALLOCATE of a pointer is not implemented yet"},
		{"a pointer component, subtask 5's", with("nullify (r%next)"),
			"NOTIMPL: a pointer component is not implemented yet"},
		{"a pointer dummy, subtask 6's", []string{"subroutine s(p)", "integer, pointer :: p", "end subroutine"},
			"NOTIMPL: a pointer dummy argument is not implemented yet"},
		{"a pointer result, subtask 6's", []string{"function f()", "integer, pointer :: f", "end function"},
			"NOTIMPL: a function whose result is a pointer is not implemented yet"},
	}

	for _, c := range cases {
		if got := messageText(analyzeFree(t, c.lines...)); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}
