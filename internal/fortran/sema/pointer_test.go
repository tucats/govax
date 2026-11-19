package sema

import "testing"

// TestPointers checks scalar and array pointers' declarations, pointer assignment,
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

		{"an array pointer", with("integer, pointer :: a(:), b(:, :)", "integer, target :: w(3, 4)",
			"a => v", "a => v(2:5:2)", "a => w(2, :)", "b => w", "b => w(:, ::2)", "a => a(3:1:-1)", "b => b",
			"a = 0", "a(2) = 1", "m = sum(a) + size(b) + lbound(a, 1)", "p => a(1)", "nullify (a, b)",
			"l = associated(a) .and. associated(a, v) .and. associated(b, w(1:2, 2:3)) .and. associated(a, b(1, :))"), ""},
		{"an array pointer of CHARACTER", with("character(len=4), pointer :: ca(:)", "character(len=4), target :: cs(3)",
			"ca => cs(1:3:2)", "ca => cs(:)(1:4)", "ca(1) = 'X'"), ""},
		{"an array pointer to a component", prog("type t", "integer i, k(0:2)", "end type", "type(t), target :: r(3)",
			"integer, pointer :: a(:)", "a => r%i", "a => r(2)%k", "a => r(1)%k(1:)"), ""},
		{"a scalar's target for an array pointer", with("integer, pointer :: a(:)", "a => n"),
			"POINTERTYPE: the pointer A points at an array of INTEGER, so its target can't be INTEGER"},
		{"a target of another rank", with("integer, pointer :: b(:, :)", "b => v"),
			"POINTERTYPE: the pointer B points at an array of rank 2 of INTEGER, so its target can't be an array of INTEGER"},
		{"an element of a pointer's target", with("integer, pointer :: a(:)", "a => v(1)"),
			"POINTERTYPE: the pointer A points at an array of INTEGER, so its target can't be INTEGER"},
		{"a section without TARGET", with("integer, pointer :: a(:)", "integer w(5)", "a => w(2:3)"),
			"NOTTARGET: W(2:3) is neither a TARGET, a part of one, nor a pointer, so no pointer may point at it"},
		{"a vector subscript", with("integer, pointer :: a(:)", "a => v((/ 1, 3 /))"),
			"VECTORTARGET: the target V((/ 1, 3 /)) has a vector subscript, so no pointer may point at it"},
		{"an assumed-size array", []string{"subroutine s(x)", "integer, target :: x(*)", "integer, pointer :: a(:)",
			"a => x", "end subroutine"}, "NOSHAPE: the assumed-size array X has no shape, so it can't be used as a whole here"},

		{"bounds on an array pointer", prog("real, pointer :: a(3)"),
			"POINTERSHAPE: A is an array POINTER, so its bounds must all be colons, (:)"},
		{"POINTER and ALLOCATABLE", prog("real, pointer, allocatable :: a(:)"),
			"ATTRPAIR: the POINTER and ALLOCATABLE attributes can't go together"},
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

		{"ALLOCATE and DEALLOCATE of pointers", with("integer, pointer :: a(:), b(:, :)", "integer :: is",
			"allocate (p, c, a(m), b(0:m, 3), stat=is)", "deallocate (p, c, a, b)", "deallocate (q, stat=is)"), ""},
		{"ALLOCATE of a record pointer", prog("type t", "integer i", "end type", "type(t), pointer :: r",
			"allocate (r)", "r%i = 1", "deallocate (r)"), ""},
		{"bounds for a scalar pointer", with("allocate (p(3))"),
			"ALLOCSCALAR: P is a scalar pointer, so ALLOCATE gives it no bounds"},
		{"no bounds for an array pointer", with("integer, pointer :: a(:)", "allocate (a)"),
			"ALLOCBOUNDS: in ALLOCATE, A needs 1 bound, each a value or two separated by a colon"},
		{"a pointer's bound using itself", with("integer, pointer :: a(:)", "allocate (a(p))", "allocate (p, a(p))"),
			"ALLOCSELF: a bound in ALLOCATE can't use P, which the same statement allocates or deallocates"},
		{"ALLOCATED of a pointer", with("l = allocated(p)"), "NOTALLOCATABLE: P is not an allocatable array"},
		{"an array pointer component, subtask 5b's", prog("type t", "integer, pointer :: k(:)", "end type"),
			"NOTIMPL: an array POINTER component is not implemented yet"},
		{"bounds on a pointer component", prog("type t", "integer, pointer :: k(3)", "end type"),
			"POINTERSHAPE: K is an array POINTER, so its bounds must all be colons, (:)"},
		{"a component of its own type without POINTER", prog("type t", "type(t) :: next", "end type"),
			"TYPESELF: a component of the derived type T can't be of that type itself unless it is a POINTER"},
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

// TestPointerComponents checks pointer components (docs/planning/PHASE-107 -
// Fortran 90 pointers.md, subtask 5a): a type pointing at itself, references
// through chains of pointers, the pointer statements on components,
// constructors' targets, and the refusals of an object with a pointer in it.
func TestPointerComponents(t *testing.T) {
	// with declares NODE, a type pointing at itself with a CHARACTER pointer
	// too, a pointer H, a TARGET N0, a plain R, and targets K and S4.
	with := func(lines ...string) []string {
		decl := []string{"program main", "type node", "integer v", "type(node), pointer :: next",
			"character(len=4), pointer :: tag", "end type", "type(node), pointer :: h", "type(node), target :: n0",
			"type(node) :: r, ra(3)", "integer, target :: k", "character(len=4), target :: s4", "integer m"}

		return append(append(decl, lines...), "end program")
	}

	cases := []struct {
		name  string
		lines []string
		want  string
	}{
		{"a list's components", with("allocate (h)", "h%next => n0", "h%next%next => h", "h%v = 1",
			"r%next%v = h%next%v + 2", "nullify (r%next, h%tag)",
			"l = associated(h%next) .and. associated(h%next, n0) .and. associated(h%next, r%next)",
			"allocate (r%next, r%next%next)", "deallocate (r%next%next)", "r = n0", "r%tag => s4",
			"r%tag(1:2) = 'AB'", "ra(2)%next => ra(1)%next%next", "m = ra(3)%next%v", "call sub(r%next, r%tag)"), ""},
		{"constructors", with("r = node(1, h, s4)", "r = node(m, n0, h%tag)", "h = node(2, h%next, s4)"), ""},
		{"a constructor's target without TARGET", with("r = node(1, r, s4)"),
			"NOTTARGET: R is neither a TARGET, a part of one, nor a pointer, so no pointer may point at it"},
		{"a constructor's target of another type", with("r = node(1, n0, k)"),
			"POINTERTYPE: the pointer NODE%TAG points at CHARACTER*4, so its target can't be INTEGER"},
		{"a constant of a type with pointers", with("type(node), parameter :: z = node(1, n0, s4)"),
			"NOTCONST: a PARAMETER constant's value must be a constant expression, and the component NEXT of NODE is a pointer, whose value is a target"},
		{"in an I/O list", with("print *, r"),
			"POINTERPART: R is of the type NODE, which has a pointer component, so it can't be an item of an I/O list"},
		{"a target in an I/O list", with("read *, h"),
			"POINTERPART: H is of the type NODE, which has a pointer component, so it can't be an item of an I/O list"},
		{"a pointer's component in an I/O list", with("print *, h%next%v, r%tag"), ""},
		{"nested in another type", with("type holder", "type(node) :: inner", "end type", "type(holder) :: hh",
			"print *, hh"),
			"POINTERPART: HH is of the type HOLDER, which has a pointer component, so it can't be an item of an I/O list"},
		{"in a NAMELIST group", with("namelist /g/ r"),
			"POINTERPART: R is of the type NODE, which has a pointer component, so it can't be in a NAMELIST group"},
		{"in DATA", with("data r%v /1/"),
			"POINTERPART: R is of the type NODE, which has a pointer component, so it can't be given an initial value"},
		{"in EQUIVALENCE", []string{"program main", "type link", "sequence", "integer v", "type(link), pointer :: next",
			"end type", "type(link) :: r", "integer k(2)", "equivalence (r, k)", "end program"},
			"POINTERPART: R is of the type LINK, which has a pointer component, so it can't be in an EQUIVALENCE list"},
		{"a pointer after a part with rank", with("m = ra(:)%next%v"),
			"POINTERRANKED: the pointer component NEXT can't follow a part of the reference that has a rank"},
		{"NULLIFY of a component that isn't a pointer", with("nullify (r%v)"), "NOTPOINTER: R%V is not a pointer"},
		{"=> of an array of pointers", with("ra%next => n0"),
			"POINTERRANKED: the pointer component NEXT can't follow a part of the reference that has a rank"},
		{"ALLOCATE of a component that isn't a pointer", with("allocate (r%v)"),
			"NOTALLOCATABLE: R%V is not an allocatable array or a pointer"},
	}

	for _, c := range cases {
		if got := messageText(analyzeFree(t, c.lines...)); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}
