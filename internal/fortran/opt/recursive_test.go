package opt

import (
	"strings"
	"testing"

	"github.com/tucats/govax/internal/fortran/ir"
)

// A recursive routine's cells under the optimizer (docs/PHASE-89 - reentrant
// FORTRAN.md, subtask 4): the optimizer's own cells go in the frame, the frame closes
// up when its cells become registers, and a call may change a static local.

// TestRecursiveCellsInFrame checks that every cell the passes make in a recursive
// routine, and every one lowering made, is in the frame, and that the frame is
// smaller once the allocator has taken temporaries out of it.
func TestRecursiveCellsInFrame(t *testing.T) {
	lines := []string{
		"RECURSIVE SUBROUTINE S(A, N)",
		"INTEGER N, A(N), I, K",
		"DO 10 I = 1, N",
		"A(I) = A(I) * (N + 1) + A(I) * (N + 1)",
		"10:CONTINUE",
		"IF (N .GT. 1) CALL S(A, N - 1)",
		"END",
	}

	before := routineOf(t, "S", lines...).FrameSize
	r := allocated(t, "S", lines...)

	for _, c := range r.Cells {
		if !c.Frame && c.Register == 0 && c.Common == nil {
			t.Errorf("cell %s is in $LOCAL", c.Name)
		}
	}

	if r.LocalSize != 0 {
		t.Errorf("$LOCAL is %d bytes", r.LocalSize)
	}

	if r.FrameSize >= before {
		t.Errorf("the frame is %d bytes, %d before the passes", r.FrameSize, before)
	}

	if err := ir.Check(r); err != nil {
		t.Error(err)
	}
}

// TestRecursiveStaticLocals checks that a recursive routine's SAVE'd local isn't
// known across a call, which may change it through another activation, while its
// unsaved one, in this activation's frame, is.
func TestRecursiveStaticLocals(t *testing.T) {
	text := allocatedMacro(t,
		"RECURSIVE SUBROUTINE S(K, N)",
		"INTEGER V, W",
		"SAVE V",
		"V = 7",
		"W = 8",
		"IF (K .EQ. 1) CALL S(2, N)",
		"N = V + W",
		"END")

	// W's 8 is still known after the call; V's 7 isn't.
	if !strings.Contains(text, "ADDL3\tS.V,#8,") {
		t.Errorf("V is not read after the call, or W is:\n%s", text)
	}
}

// TestFrameBaseKept checks that the register a variable format expression keeps the
// frame's base in goes to no cell, however many the routine could use, and that a
// routine with no frame, whose expressions need no base, has it for its cells.
func TestFrameBaseKept(t *testing.T) {
	body := []string{
		"INTEGER A, B, C, D, E, F, G, H, I",
		"A = 0",
		"B = 1",
		"C = 2",
		"D = 3",
		"E = 4",
		"F = 5",
		"G = 6",
		"H = 7",
		"DO 20 I = 1, N",
		"A = A + 1",
		"B = B + A",
		"C = C + B",
		"D = D + C",
		"E = E + D",
		"F = F + E",
		"G = G + F",
		"H = H + G",
		"20:CONTINUE",
		"WRITE (6, 10) A, B, C, D, E, F, G, H",
		"10:FORMAT (8I<N>)",
		"END",
	}

	highest := func(header string) int {
		top := 0

		for _, c := range allocated(t, "S", append([]string{header}, body...)...).Cells {
			top = max(top, c.Register)
		}

		return top
	}

	if got := highest("RECURSIVE SUBROUTINE S(N)"); got != 10 {
		t.Errorf("a recursive routine's highest register cell is R%d, want R10", got)
	}

	if got := highest("SUBROUTINE S(N)"); got != 11 {
		t.Errorf("a routine with no frame's highest register cell is R%d, want R11", got)
	}
}
