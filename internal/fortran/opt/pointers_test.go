package opt

import (
	"slices"
	"testing"
)

// TestPointerConflicts holds Conflict to the aliasing rule for pointers
// (docs/planning/PHASE-107 - Fortran 90 pointers.md, finding 11): for each
// program, the last two writes, a store through a pointer and then another, conflict
// or don't as listed.
func TestPointerConflicts(t *testing.T) {
	declarations := []string{
		"OPTIONS /F90",
		"INTEGER, TARGET :: T, TT(4)",
		"INTEGER, POINTER :: P, Q",
		"REAL, POINTER :: RP",
		"REAL, TARGET :: R",
		"REAL X, Y",
		"INTEGER K, V(4)",
		"INTEGER, TARGET :: E",
		"REAL EX",
		"EQUIVALENCE (E, EX)",
		"TYPE PAIR",
		"INTEGER A, B",
		"END TYPE PAIR",
		"TYPE REALS",
		"REAL C, D",
		"END TYPE REALS",
		"TYPE(PAIR), TARGET :: PR",
		"TYPE(PAIR) NPR",
		"TYPE(REALS), TARGET :: RR",
		"TYPE(PAIR), POINTER :: PP",
		"INTEGER, ALLOCATABLE, TARGET :: AT(:)",
		"INTEGER, ALLOCATABLE :: AN(:)",
		"CHARACTER*8, TARGET :: S",
		"CHARACTER*8 NS",
		"CHARACTER*2, POINTER :: CP",
		"COMMON /C/ IC",
	}

	cases := []struct {
		name  string
		lines []string
		want  bool
	}{
		{"a local without TARGET", []string{"P => T", "P = 1", "K = 2"}, false},
		{"an array without TARGET", []string{"P => T", "P = 1", "V(K) = 2"}, false},
		{"a TARGET", []string{"P => T", "P = 1", "T = 2"}, true},
		{"a TARGET array", []string{"P => TT(2)", "P = 1", "TT(K) = 2"}, true},
		{"a TARGET of another type", []string{"P => T", "P = 1", "R = 2.0"}, false},
		{"EQUIVALENCE with a TARGET", []string{"P => E", "P = 1", "EX = 2.0"}, true},
		{"a record holding the type", []string{"P => PR%A", "P = 1", "PR%B = 2"}, true},
		{"a record without TARGET", []string{"P => PR%A", "P = 1", "NPR%A = 2"}, false},
		{"a record without the type", []string{"P => T", "P = 1", "RR%C = 2.0"}, false},
		{"two pointers of one type", []string{"P => T", "Q => T", "P = 1", "Q = 2"}, true},
		{"two pointers of two types", []string{"P => T", "RP => R", "P = 1", "RP = 2.0"}, false},
		{"a record's pointer and its component's", []string{"PP => PR", "P => PR%A", "P = 1", "PP%B = 2"}, true},
		{"a record's pointer and another type's", []string{"PP => PR", "RP => R", "RP = 1.0", "PP%B = 2"}, false},
		{"COMMON", []string{"P => T", "P = 1", "IC = 2"}, true},
		{"an allocatable TARGET", []string{"ALLOCATE (AT(4))", "P => AT(1)", "P = 1", "AT(K) = 2"}, true},
		{"an allocatable array without TARGET", []string{"ALLOCATE (AN(4))", "P => T", "P = 1", "AN(K) = 2"}, false},
		{"a substring of a TARGET", []string{"CP => S(3:4)", "CP = 'AB'", "S(1:1) = 'X'"}, true},
		{"a string without TARGET", []string{"CP => S(3:4)", "CP = 'AB'", "NS(1:1) = 'X'"}, false},
		{"a CHARACTER pointer and an INTEGER TARGET", []string{"CP => S(3:4)", "CP = 'AB'", "T = 2"}, false},
	}

	for _, c := range cases {
		lines := append(slices.Clone(declarations), c.lines...)
		r := routineOf(t, "T$MAIN", append(lines, "END")...)
		s := NewStorage(r)

		w := writes(s, r)
		a, b := w[len(w)-2], w[len(w)-1]

		if a.Region != PointerRegion {
			t.Errorf("%s: the store through a pointer is in %+v", c.name, a)
		}

		if got := Conflict(a, b); got != c.want {
			t.Errorf("%s: Conflict says %v of %+v and %+v", c.name, got, a, b)
		}

		if got := Conflict(b, a); got != c.want {
			t.Errorf("%s: Conflict the other way round says %v", c.name, got)
		}

		if !s.ChangedByCall(a) {
			t.Errorf("%s: no call may change a pointer's target", c.name)
		}
	}
}

// TestPointerModules checks a module's variables, judged by their own attributes
// (the author's answer 3), among them a TARGET the program doesn't name.
func TestPointerModules(t *testing.T) {
	module := []string{
		"OPTIONS /F90",
		"MODULE M",
		"INTEGER, TARGET :: MT",
		"INTEGER MN",
		"INTEGER, TARGET :: MS",
		"INTEGER, POINTER :: MP",
		"END MODULE M",
		"USE M, ONLY: MT, MN, MP",
	}

	for _, c := range []struct {
		name string
		line string
		want bool
	}{
		{"a TARGET", "MT = 2", true},
		{"a variable without TARGET", "MN = 2", false},
	} {
		lines := append(slices.Clone(module), "MP = 1", c.line, "END")
		r := routineOf(t, "T$MAIN", lines...)
		s := NewStorage(r)

		w := writes(s, r)
		if a, b := w[len(w)-2], w[len(w)-1]; Conflict(a, b) != c.want {
			t.Errorf("%s: Conflict says %v of %+v and %+v", c.name, !c.want, a, b)
		}
	}
}

// TestPointerDummies checks dummy arguments: a TARGET one may be a pointer's
// target, one without TARGET may not (the standard's 12.5.2.9), and a pointer
// dummy's target is a pointer's like any other.
func TestPointerDummies(t *testing.T) {
	r := routineOf(t, "S",
		"OPTIONS /F90",
		"SUBROUTINE S(D, N, P)",
		"INTEGER, TARGET :: D",
		"INTEGER N",
		"INTEGER, POINTER :: P",
		"P = 1",
		"D = 2",
		"N = 3",
		"END")

	s := NewStorage(r)
	w := writes(s, r)

	if len(w) != 3 || w[0].Region != PointerRegion {
		t.Fatalf("the writes are %+v", w)
	}

	if !Conflict(w[0], w[1]) {
		t.Error("a store through P can't change the TARGET dummy D")
	}

	if Conflict(w[0], w[2]) {
		t.Error("a store through P may change N, which has no TARGET")
	}
}

// TestPointerRegisters checks what the rule is for: a variable without TARGET stays
// in a register across a loop's stores through a pointer, and a TARGET doesn't.
func TestPointerRegisters(t *testing.T) {
	r := allocated(t, "T$MAIN",
		"OPTIONS /F90",
		"INTEGER, TARGET :: T, U",
		"INTEGER, POINTER :: P",
		"INTEGER K, I, N",
		"READ *, N, K",
		"P => T",
		"DO 10 I = 1, N",
		"P = P + K",
		"U = U + 1",
		"10:CONTINUE",
		"PRINT *, T, U",
		"END")

	held := inRegisters(r)

	if !slices.Contains(held, "K") {
		t.Errorf("K isn't in a register; the registers hold %v", held)
	}

	if slices.Contains(held, "U") {
		t.Errorf("U, a TARGET, is in a register across a store through P: %v", held)
	}
}
