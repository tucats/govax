package sema

import (
	"strings"
	"testing"
)

// TestForallStatement holds the FORALL statement's analysis (Phase 112's
// subtask 8): each index a name of the statement's own, the bounds and mask
// checked, and the choice between doing each tuple's assignment in turn and
// computing every value first ("[in temporaries]"), which the dump shows.
func TestForallStatement(t *testing.T) {
	prog, messages := analyzeFreeWith(t, Options{},
		"module m",
		"contains",
		"  pure integer function sq(k)",
		"    integer, intent(in) :: k",
		"    sq = k * k",
		"  end function sq",
		"end module m",
		"program p",
		"  use m",
		"  implicit none",
		"  integer, parameter :: n = 4",
		"  real :: a(n,n), b(n,n)",
		"  integer :: v(8), i, j, lo",
		"  character(len=3) :: c(5)",
		"  forall (i = 1:n) a(i,i) = real(i)",
		"  forall (i = 1:n, j = 1:n, a(i,j) /= 0.0) b(i,j) = 1.0 / a(i,j)",
		"  forall (i = 2:8) v(i) = v(i-1)",
		"  forall (i = 8:1:-2) v(i) = sq(v(i))",
		"  forall (i = 1:n) a(i,:) = b(:,i)",
		"  forall (i = 1:lo, v(i) > 0) v(i) = v(i) + 1",
		"  forall (i = 1:4) c(i) = c(i+1)",
		"  forall (i = 1:4, v(i+1) > 0) v(i) = 0",
		"end program p")

	if len(messages) != 0 {
		t.Fatalf("messages: %s", messageText(messages))
	}

	dump := Dump(prog)

	for _, want := range []string{
		"FORALL (I = 1:4) A[I, I] = (REAL I)\n",
		"FORALL (I = 1:4, J = 1:4, (.NE.:R A[I, J] 0.0)) B[I, J] = (/:R 1.0 A[I, J])\n",
		"FORALL (I = 2:8) V[I] = V[(-:I I 1)]  [in temporaries]\n",
		"FORALL (I = 8:1:-2) V[I] = SQ(V[I])\n",
		"FORALL (I = 1:4) A[I, 1:4:1] = B[1:4:1, I]\n",
		"FORALL (I = 1:LO, (.GT.:I V[I] 0)) V[I] = (+:I V[I] 1)\n",
		"FORALL (I = 1:4) C[I] = C[(+:I I 1)]  [in temporaries]\n",
		"FORALL (I = 1:4, (.GT.:I V[(+:I I 1)] 0)) V[I] = 0  [in temporaries]\n",
	} {
		if !strings.Contains(dump, want) {
			t.Errorf("the dump has no %q:\n%s", want, dump)
		}
	}
}

// TestForallErrors holds the FORALL statement's messages.
func TestForallErrors(t *testing.T) {
	cases := []struct {
		name  string
		lines []string
		want  string
	}{
		{"an index not INTEGER", []string{"  forall (x = 1:4) v(1) = 0"},
			"FORALLINDEX: a FORALL index must be a scalar INTEGER variable's name, and X is a REAL"},
		{"an index an array", []string{"  forall (v = 1:4) w(1,1) = 0"},
			"FORALLINDEX: a FORALL index must be a scalar INTEGER variable's name, and V is an array"},
		{"an index a constant", []string{"  forall (n = 1:4) v(n) = 0"},
			"FORALLINDEX: a FORALL index must be a scalar INTEGER variable's name, and N is a named constant"},
		{"an index with no type", []string{"  forall (q = 1:4) v(q) = 0"},
			"NOTYPE: Q has no type: with IMPLICIT NONE, every name must be declared"},
		{"an index twice", []string{"  forall (k = 1:4, k = 1:2) v(k) = 0"},
			"FORALLTWICE: K is an index of this FORALL already"},
		{"a bound with an index", []string{"  forall (i = 1:4, j = 1:i) v(j) = 0"},
			"FORALLBOUND: a FORALL index's bounds and stride can't use I, an index of the same FORALL"},
		{"a REAL bound", []string{"  forall (i = 1:x) v(i) = 0"},
			"FORALLINT: a FORALL index's bounds and stride must be scalar INTEGER values, and this one is a REAL"},
		{"a zero stride", []string{"  forall (i = 1:4:0) v(i) = 0"},
			"FORALLSTRIDE: a FORALL index's stride can't be zero"},
		{"a REAL mask", []string{"  forall (i = 1:4, x) v(i) = 0"},
			"FORALLMASK: a FORALL's mask must be a scalar LOGICAL value, and this one is a REAL"},
		{"an index assigned", []string{"  forall (i = 1:4) i = 0"},
			"FORALLSET: I is an index of the FORALL, which only the FORALL gives values"},
		{"a function not PURE", []string{"  forall (i = 1:4) v(i) = f(i)"},
			"FORALLPURE: F isn't declared PURE, so it can't be referenced in a FORALL"},
		{"its own subscripts", []string{"  forall (i = 1:4) v(v(i)) = 0"},
			"NOTIMPL: a FORALL target whose subscripts read its own storage is not implemented yet"},
		{"an array target overlapped", []string{"  forall (i = 1:3) w(i,:) = w(i+1,:)"},
			"NOTIMPL: a FORALL whose array target its value overlaps is not implemented yet"},
		{"a pointer assignment", []string{"  forall (i = 1:4) pp(i)%p => t(i)"},
			"NOTIMPL: a pointer assignment in a FORALL statement is not implemented yet"},
	}

	for _, c := range cases {
		lines := []string{
			"program p", "  implicit none",
			"  type pt", "    integer, pointer :: p", "  end type pt",
			"  integer, parameter :: n = 4",
			"  integer :: v(8), w(4,4), i, j, k, f",
			"  integer, target :: t(4)",
			"  type(pt) :: pp(4)",
			"  real :: x",
		}
		lines = append(append(lines, c.lines...), "end program p")

		if got := messageText(analyzeFree(t, lines...)); got != c.want {
			t.Errorf("%s:\n got %q\nwant %q", c.name, got, c.want)
		}
	}
}

// TestForallIndexesHidden checks that a FORALL's indexes are its own: the
// unit's I is left alone, and a name the unit never declared, under implicit
// typing, is entered in no table.
func TestForallIndexesHidden(t *testing.T) {
	prog, messages := analyzeFreeWith(t, Options{},
		"program p",
		"  integer :: v(8)",
		"  forall (kk = 1:8) v(kk) = kk",
		"end program p")

	if len(messages) != 0 {
		t.Fatalf("messages: %s", messageText(messages))
	}

	for _, s := range prog.Units[0].Symbols {
		if s.Name == "KK" {
			t.Errorf("KK, the FORALL's index, is in the unit's table")
		}
	}
}

// TestForallNoF90 checks that the FORALL statement is VAX FORTRAN's too, in
// fixed form under /NOF90 (the author's answer 2).
func TestForallNoF90(t *testing.T) {
	prog, messages := analyzeOpts(t, Options{},
		"      PROGRAM P",
		"      INTEGER V(8)",
		"      FORALL (I = 2:8) V(I) = V(I-1)",
		"      END")

	if len(messages) != 0 {
		t.Fatalf("messages: %s", messageText(messages))
	}

	if want := "FORALL (I = 2:8) V[I] = V[(-:I I 1)]  [in temporaries]\n"; !strings.Contains(Dump(prog), want) {
		t.Errorf("the dump has no %q:\n%s", want, Dump(prog))
	}
}
