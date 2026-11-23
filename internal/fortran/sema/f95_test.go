package sema

import (
	"strings"
	"testing"
)

// TestF95Pending checks that Fortran 95's forms, which the parser builds since
// Phase 112's subtask 2, are NOTIMPL by name until their subtasks, each once,
// with nothing else said: a FORALL construct's statements are passed over, and
// a unit with an ELEMENTAL header keeps its structure. PURE is compiled since
// subtask 6 (pure_test.go), ELEMENTAL since 7, and the FORALL statement since 8
// (forall_test.go).
func TestF95Pending(t *testing.T) {
	cases := []struct {
		name  string
		lines []string
		want  string
	}{
		{"PURE", []string{
			"module m", "contains",
			"  pure integer function sq(x)", "    integer, intent(in) :: x", "    sq = x * x", "  end function sq",
			"end module m"},
			""},
		{"ELEMENTAL", []string{
			"program p", "  print *, inc(1)", "contains",
			"  elemental integer function inc(k)", "    integer, intent(in) :: k", "    inc = k + 1", "  end function inc",
			"end program p"},
			""},
		{"FORALL statement", []string{
			"program p", "  implicit none", "  integer :: a(4), i", "  forall (i = 1:4) a(i) = i", "end program p"},
			""},
		{"FORALL construct", []string{
			"program p", "  implicit none", "  integer :: a(4,4)",
			"  outer: forall (i = 1:4)", "    forall (j = 1:4, j > i) a(i,j) = i + j",
			"    inner: forall (j = 1:4)", "      a(j,j) = k", "    end forall inner",
			"    where (a(i,:) == 0) a(i,:) = -1",
			"  end forall outer", "  a(1,1) = 0", "end program p"},
			"NOTIMPL: the FORALL construct is not implemented yet"},
		{"an unknown prefix", []string{
			"module m", "contains",
			"  impure subroutine s(a)", "    integer :: a", "    a = 1", "  end subroutine s",
			"  subroutine t", "  end subroutine t",
			"end module m"},
			"UNRECPREFIX: IMPURE is not a prefix FORTRAN knows; the rest is read as a SUBROUTINE statement"},
		{"END INTERFACE named", []string{
			"program p", "  interface g", "    subroutine s(a)", "      integer :: a", "    end subroutine s",
			"  end interface g", "  call g(1)", "end program p"},
			""},
	}

	for _, c := range cases {
		ms := analyzeFree(t, c.lines...)

		var got []string
		for _, m := range ms {
			got = append(got, m.Ident+": "+m.Text)
		}

		if strings.Join(got, "; ") != c.want {
			t.Errorf("%s:\n got %q\nwant %q", c.name, got, c.want)
		}
	}
}

// TestEmptyArrays holds Fortran 90's zero-sized explicit-shape arrays (Phase 112's
// subtask 3): under /F90 a dimension declared with its upper bound below its lower
// is kept as (1:0), in a variable, a component, COMMON, EQUIVALENCE, a PARAMETER,
// and an initial value, its array of no elements and no storage; without /F90 it
// is still VAX FORTRAN's BOUNDORDER.
func TestEmptyArrays(t *testing.T) {
	prog, messages := analyzeFreeWith(t, Options{},
		"program p",
		"  implicit none",
		"  integer :: i",
		"  type box",
		"    integer :: n",
		"    real :: v(5:4)",
		"    integer :: m",
		"  end type",
		"  integer, parameter :: none(0) = (/ (i, i = 1, 0) /)",
		"  real :: a(0), b(5:1) = 2.0, c(3, -1:-2)",
		"  integer :: x, eq(0), z0(0), y",
		"  common /blk/ x, z0, y",
		"  equivalence (eq, i)",
		"  type(box) :: bx",
		"  bx%n = size(none) + size(a) + size(b) + size(c) + size(eq)",
		"end program p")
	if len(messages) != 0 {
		t.Fatalf("messages: %s", messageText(messages))
	}

	u := prog.Units[0]

	for _, c := range []struct {
		name string
		dims []Dim
	}{
		{"A", []Dim{{Lower: 1, Upper: 0}}},
		{"B", []Dim{{Lower: 1, Upper: 0}}},
		{"C", []Dim{{Lower: 1, Upper: 3}, {Lower: 1, Upper: 0}}},
		{"NONE", []Dim{{Lower: 1, Upper: 0}}},
	} {
		s := u.Lookup(c.name)
		if s == nil {
			t.Fatalf("no %s", c.name)
		}

		if len(s.Dims) != len(c.dims) {
			t.Fatalf("%s: %d dimensions", c.name, len(s.Dims))
		}

		for i, d := range c.dims {
			if s.Dims[i].Lower != d.Lower || s.Dims[i].Upper != d.Upper {
				t.Errorf("%s: dimension %d is %d:%d, want %d:%d", c.name, i+1, s.Dims[i].Lower, s.Dims[i].Upper, d.Lower, d.Upper)
			}
		}

		if !s.Empty() || s.Elements() != 0 {
			t.Errorf("%s: Empty %v, Elements %d", c.name, s.Empty(), s.Elements())
		}
	}

	if s := u.Lookup("A"); s.Size() != 0 {
		t.Errorf("A's size is %d", s.Size())
	}

	if y, x := u.Lookup("Y"), u.Lookup("X"); y.Offset != x.Offset+4 {
		t.Errorf("Y is at %d, X at %d: Z0 took space", y.Offset, x.Offset)
	}

	if bx := u.Lookup("BX"); bx.Struct == nil || bx.Struct.Size != 8 {
		t.Errorf("BOX isn't 8 bytes: %+v", bx.Struct)
	}

	for _, s := range []string{"X", "BX"} {
		if u.Lookup(s).Empty() {
			t.Errorf("%s is Empty", s)
		}
	}

	// An element of an empty array is never in it.
	_, messages = analyzeFreeWith(t, Options{}, "program p", "  real :: a(0), x", "  equivalence (a(1), x)", "end program p")
	if len(messages) != 1 || messages[0].Ident != "EQUIVRANGE" {
		t.Errorf("A(1): %s, want EQUIVRANGE", messageText(messages))
	}

	// Without /F90, still BOUNDORDER, in a variable and a component.
	_, messages = analyzeOpts(t, Options{}, "      REAL A(0)", "      STRUCTURE /S/", "        REAL V(5:4)",
		"      END STRUCTURE", "      END")
	if got := messageText(messages); got != "BOUNDORDER: the upper bound 0 is less than the lower bound 1; "+
		"BOUNDORDER: the upper bound 4 is less than the lower bound 5" {
		t.Errorf("without /F90: %s", got)
	}
}

// TestNull holds Fortran 95's NULL() (Phase 112's subtask 4, null.go): read only
// where a pointer's value goes, after =>, in a declaration or DATA, for a pointer
// component in a structure constructor, and as a pointer dummy's argument, MOLD
// agreeing with the pointer; refused anywhere else, NULLPLACE.
func TestNull(t *testing.T) {
	prog := func(lines ...string) []string {
		return append(append([]string{"program prog", "  implicit none",
			"  integer, pointer :: p, q", "  real, pointer :: r(:)", "  integer :: n",
			"  type node", "    integer :: v", "    type(node), pointer :: next", "  end type"},
			lines...), "end program prog")
	}

	// generic is a program calling TELL, a generic name for two subroutines of
	// a scalar pointer dummy, and ONE, a generic name for one.
	generic := func(lines ...string) []string {
		return append(append([]string{"module g", "  interface tell", "    module procedure tell_i, tell_r",
			"  end interface", "  interface one", "    module procedure tell_i", "  end interface", "contains",
			"  subroutine tell_i(p)", "    integer, pointer :: p", "  end subroutine",
			"  subroutine tell_r(p)", "    real, pointer :: p", "  end subroutine", "end module g",
			"program gp", "  use g", "  integer, pointer :: ip", "  real, pointer :: rp, ra(:)", "  integer :: n"},
			lines...), "end program gp")
	}

	cases := []struct {
		name  string
		lines []string
		want  string
	}{
		{"pointer assignment", prog("p => null()", "p => null(q)", "p => null(mold=q)", "r => null(r)"), ""},
		{"initial values", prog("integer, pointer :: a => null(), b(:) => null()", "real, pointer :: c => null(r(1))"),
			"NOTPOINTER: R(1) is not a pointer"},
		{"DATA", prog("integer, pointer :: d, e(:)", "data d, e /2*null()/"), ""},
		{"a constructor", prog("type(node) :: x", "x = node(1, null())"), ""},
		{"a MOLD of another type", prog("real, pointer :: s", "p => null(s)"),
			"POINTERTYPE: the pointer P points at INTEGER, so its target can't be REAL"},
		{"a MOLD of another rank", prog("r => null(q)"),
			"POINTERTYPE: the pointer R points at an array of REAL, so its target can't be INTEGER"},
		{"two arguments", prog("p => null(p, q)"), "INTRINARGS: the intrinsic function NULL takes 0 or 1 arguments, not 2"},
		{"a wrong keyword", prog("p => null(target=q)"),
			"KEYWORDNAME: the intrinsic function NULL has no argument TARGET; its arguments are MOLD"},
		{"an expression", prog("n = null()"), "NULLPLACE: a reference to NULL() is a disassociated pointer, so it can stand " +
			"only where a pointer's value goes: after =>, in DATA for a pointer, for a pointer component in a structure " +
			"constructor, or as a pointer dummy's argument"},
		{"DATA for no pointer", prog("data n /null()/"), "NULLPLACE: a reference to NULL() is a disassociated pointer, so it can stand " +
			"only where a pointer's value goes: after =>, in DATA for a pointer, for a pointer component in a structure " +
			"constructor, or as a pointer dummy's argument"},
		{"=> for no pointer", prog("integer :: m => null()"), "NOTPOINTER: M is not a pointer"},
		{"=> of a target", prog("integer, target :: t", "integer, pointer :: m => t"),
			"POINTERINIT: a pointer's initial value after => can only be NULL(), not T"},
		{"DATA of a constant", prog("data p /1/"), "POINTERATTR: the pointer P can't be given an initial value but NULL()"},
		{"a function of the file's own", append(prog("integer, external :: null", "n = null()"),
			"integer function null()", "null = 1", "end function"), ""},
		{"an EXTERNAL NULL", prog("external null", "integer null", "p => null()"),
			"POINTERFUNC: the target NULL() is a function reference, and only a function whose result is a pointer gives a target"},
		{"a pointer dummy", []string{"program p", "  call s(null())", "contains", "  subroutine s(k)",
			"    integer, pointer :: k", "  end subroutine", "end program"}, ""},
		{"a pointer dummy of a MOLD of another type", []string{"program p", "  real, pointer :: x", "  call s(null(x))",
			"contains", "  subroutine s(k)", "    integer, pointer :: k", "  end subroutine", "end program"},
			"POINTERTYPE: the pointer K points at INTEGER, so its target can't be REAL"},
		{"a dummy that isn't a pointer", []string{"program p", "  call s(null())", "contains", "  subroutine s(k)",
			"    integer :: k", "  end subroutine", "end program"}, "NULLPLACE: a reference to NULL() is a disassociated " +
			"pointer, so it can stand only where a pointer's value goes: after =>, in DATA for a pointer, for a pointer " +
			"component in a structure constructor, or as a pointer dummy's argument"},
		{"a generic name", generic("call tell(null(ip))", "call tell(null(rp))", "call tell(p=null(ip))", "call one(null())"), ""},
		{"a generic name, no MOLD", generic("call tell(null())"),
			"NULLMOLD: an argument NULL() of the generic name TELL fits both TELL_I and TELL_R, so it needs MOLD= to say which pointer it is"},
		{"a generic name, a MOLD that fits none", generic("call tell(null(ra))"),
			"GENERICNONE: no specific procedure of the generic name TELL takes a pointer to an array of REAL"},
		{"a generic name's pointer dummy given no pointer", generic("call tell(n)"),
			"POINTERARG: argument 1 of TELL_I, for the pointer dummy argument P, must be a pointer to INTEGER"},
		{"AUTOMATIC", []string{"subroutine s", "  automatic x", "  integer, pointer :: x => null()", "end subroutine"},
			"AUTOINIT: X has an initial value, so it can't be AUTOMATIC"},
	}

	for _, c := range cases {
		if got := messageText(analyzeFree(t, c.lines...)); got != c.want {
			t.Errorf("%s:\n got %q\nwant %q", c.name, got, c.want)
		}
	}

	// A recursive unit's pointer given NULL() is static, as any initialized
	// local is.
	prog2, ms := analyzeFreeWith(t, Options{}, "recursive subroutine s(k)", "  integer :: k",
		"  integer, pointer :: a => null(), b, c", "  data b /null()/", "end subroutine s")
	if len(ms) != 0 {
		t.Fatalf("messages: %s", messageText(ms))
	}

	for name, automatic := range map[string]bool{"A": false, "B": false, "C": true} {
		if s := prog2.Units[0].Lookup(name); s.Automatic != automatic {
			t.Errorf("%s: Automatic %v, want %v", name, s.Automatic, automatic)
		}
	}

	// Without /F90, NULL() is the intrinsic's where a pointer's value goes,
	// and anywhere else a function's of the program's.
	_, ms = analyzeLines(t, "T.FOR",
		"      INTEGER, POINTER :: P",
		"      INTEGER N",
		"      P => NULL()",
		"      N = NULL()",
		"      END")
	if len(ms) != 0 {
		t.Errorf("/NOF90: %s", messageText(ms))
	}
}

// TestDefaults holds default initialization (Phase 112, subtask 5): the
// components' forms and their errors, which objects each entry point sets, and
// which get initial values in the image instead.
func TestDefaults(t *testing.T) {
	prog := func(lines ...string) []string {
		return append(append([]string{"program prog", "  implicit none", "  integer :: n"}, lines...), "end program prog")
	}

	typed := func(lines ...string) []string {
		return prog(append([]string{"  type t", "    integer :: i = 1", "    real :: r", "  end type"}, lines...)...)
	}

	cases := []struct {
		name  string
		lines []string
		want  string
	}{
		{"every form", prog("type t", "integer :: i = 2, j", "real :: v(3) = 1.5, w(2) = (/ 1.0, 2.0 /)",
			"character(len=4) :: c = 'ab'", "integer, pointer :: p => null()", "type(t), pointer :: next => null()",
			"end type", "type u", "type(t) :: x", "type(t) :: y = t(1, 2, 1.0, 2.0, 'x', null(), null())",
			"end type"),
			"DATAVALUE: an initial value must be a constant, and the component P of T is a pointer, whose value is a target"},
		{"nested defaults", prog("type t", "integer :: i = 2", "end type", "type u", "type(t) :: x, y = t(3)", "end type",
			"type(u) :: a"), ""},
		{"slashes", prog("type t", "integer i /3/", "end type"),
			"COMPSLASH: a component's default value is given after =, or after => as NULL() for a pointer, not between slashes"},
		{"=> for no pointer", prog("type t", "integer :: i => null()", "end type"), "NOTPOINTER: I is not a pointer"},
		{"= for a pointer", prog("type t", "integer, pointer :: p = 3", "end type"),
			"POINTERATTR: the pointer P can't be given an initial value but NULL()"},
		{"=> of a target", prog("type t", "integer, pointer :: p => n", "end type"),
			"POINTERINIT: a pointer's initial value after => can only be NULL(), not N"},
		{"a value of another type", prog("type t", "character(len=2) :: c = 3", "end type"),
			"CONSTRVALUE: an INTEGER value can't be the value of the component C of T, which is a CHARACTER*2 value"},
		{"a variable", prog("type t", "integer :: i = n", "end type"),
			"DATAVALUE: an initial value must be a constant, and N is not a constant"},
		{"an array of another shape", prog("type t", "integer :: v(3) = (/ 1, 2 /)", "end type"),
			"NOTCONFORM: the shapes (3) and (2) don't conform"},
		{"COMMON", prog("type t", "sequence", "integer :: i = 1", "end type", "type(t) :: x", "common /c/ x"),
			"DEFAULTINIT: X is of the type T, whose components have default values, so it can't be in a common block"},
		{"DATA", typed("type(t) :: x", "data x%r /2.0/"),
			"DEFAULTINIT: X is of the type T, whose components have default values, so it can't be given an initial value by DATA"},
		{"an assumed-size INTENT(OUT) dummy", typed("contains", "subroutine s(x)", "type(t), intent(out) :: x(*)",
			"end subroutine"), "DEFAULTINIT: X is of the type T, whose components have default values, so it can't be an " +
			"assumed-size INTENT(OUT) dummy argument"},
		{"an initial value of its own", typed("type(t) :: x = t(3, 4.0)"), ""},
	}

	for _, c := range cases {
		if got := messageText(analyzeFree(t, c.lines...)); got != c.want {
			t.Errorf("%s:\n got %q\nwant %q", c.name, got, c.want)
		}
	}

	// The defaults' bytes: a component's own default replaces its type's.
	p, ms := analyzeFreeWith(t, Options{}, prog("type t", "integer(2) :: i = 2, j", "end type",
		"type u", "type(t) :: x, y = t(7, 8)", "character(len=2) :: c = 'a'", "end type", "type(u) :: a")...)
	if len(ms) != 0 {
		t.Fatalf("messages: %s", messageText(ms))
	}

	st := p.Units[0].Lookup("A").Struct
	if got, want := st.DefaultBytes(), []byte{2, 0, 0, 0, 7, 0, 8, 0, 'a', ' '}; string(got) != string(want) {
		t.Errorf("DefaultBytes % X, want % X", got, want)
	}

	// Which objects each entry point sets: an unsaved local, an INTENT(OUT)
	// dummy, an automatic array, the value; not a saved local, one given a value,
	// an INTENT(IN) dummy, an allocatable array, or a pointer.
	p, ms = analyzeFreeWith(t, Options{}, "module m", "  type t", "    integer :: i = 1", "  end type", "contains",
		"  function f(a, b, k) result(r)", "    type(t), intent(out) :: a", "    type(t), intent(in) :: b",
		"    integer, intent(in) :: k", "    type(t) :: r, loc, w(k), kept = t(5)", "    type(t), save :: s",
		"    type(t), allocatable :: al(:)", "    type(t), pointer :: pt", "    loc%i = s%i + kept%i", "    w(1) = loc",
		"    r = b", "  end function", "end module m",
		"program main", "  use m", "  type(t) :: x, y", "  x = f(y, y, 2)", "end program main")
	if len(ms) != 0 {
		t.Fatalf("messages: %s", messageText(ms))
	}

	var names []string

	for _, u := range p.Units {
		for _, d := range u.Defaults {
			names = append(names, u.Name+"."+d.Sym.Name)
		}
	}

	if got, want := strings.Join(names, " "), "F.R F.A F.LOC F.W"; got != want {
		t.Errorf("defaults %q, want %q", got, want)
	}

	// A main program's locals are static, their defaults initial values.
	main := p.Units[len(p.Units)-1]

	given := map[string]bool{}
	for _, in := range main.Initials {
		given[in.Sym.Name] = true
	}

	if !given["X"] || !given["Y"] {
		t.Errorf("main program's initial values: %v", given)
	}

	// A recursive unit's unsaved local is in the frame: default initialization
	// doesn't imply SAVE.
	p, ms = analyzeFreeWith(t, Options{}, "recursive subroutine s(k)", "  integer :: k", "  type t",
		"    integer :: i = 1", "  end type", "  type(t) :: a", "  type(t), save :: b", "  a%i = b%i + k", "end subroutine s")
	if len(ms) != 0 {
		t.Fatalf("messages: %s", messageText(ms))
	}

	for name, automatic := range map[string]bool{"A": true, "B": false} {
		if s := p.Units[0].Lookup(name); s.Automatic != automatic {
			t.Errorf("%s: Automatic %v, want %v", name, s.Automatic, automatic)
		}
	}

	// A host's object is the host's to set: its internal procedure's entry
	// leaves it alone.
	p, ms = analyzeFreeWith(t, Options{}, "subroutine h", "  type t", "    integer :: i = 1", "  end type",
		"  type(t) :: x", "  call inner", "contains", "  subroutine inner", "    x%i = x%i + 1", "  end subroutine",
		"end subroutine h")
	if len(ms) != 0 {
		t.Fatalf("host: %s", messageText(ms))
	}

	for _, u := range p.Units {
		if want := map[string]int{"H": 1, "INNER": 0}[u.Name]; len(u.Defaults) != want {
			t.Errorf("host: %s sets %d objects, want %d", u.Name, len(u.Defaults), want)
		}
	}

	// Under /NOF90 too (the author's answer 2).
	p, ms = analyzeLines(t, "T.FOR",
		"      SUBROUTINE S",
		"      TYPE :: T",
		"        INTEGER :: I = 1",
		"        INTEGER, POINTER :: P => NULL()",
		"      END TYPE",
		"      TYPE(T) X",
		"      X%I = 2",
		"      END")
	if len(ms) != 0 {
		t.Fatalf("/NOF90: %s", messageText(ms))
	}

	if d := p.Units[0].Defaults; len(d) != 1 || d[0].Sym.Name != "X" {
		t.Errorf("/NOF90: defaults %v", d)
	}
}
