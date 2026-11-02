package opt

import (
	"strings"
	"testing"

	"github.com/tucats/govax/internal/fortran/ir"
)

// improved lowers a program and runs the IR passes on the routine named.
func improved(t *testing.T, name string, lines ...string) *ir.Routine {
	t.Helper()

	r := routineOf(t, name, lines...)
	run([]*ir.Routine{r}, beforeRegisters())

	return r
}

// beforeRegisters are the passes before the register allocator, whose results a test can
// read by the program's own names.
func beforeRegisters() []Pass {
	return Passes[:len(Passes)-1]
}

// codeText is a routine's statements and terminators, a line each, as the dump
// writes them.
func codeText(r *ir.Routine) string {
	var lines []string

	for _, b := range r.Blocks {
		for _, st := range b.Stmts {
			lines = append(lines, ir.StmtText(st))
		}

		lines = append(lines, ir.TerminatorText(b.End))
	}

	return strings.Join(lines, "\n")
}

// expect checks that a routine's code has each line in has and none in hasnt.
func expect(t *testing.T, r *ir.Routine, has, hasnt []string) {
	t.Helper()

	lines := strings.Split(codeText(r), "\n")

	contains := func(want string) bool {
		for _, line := range lines {
			if line == want {
				return true
			}
		}

		return false
	}

	for _, want := range has {
		if !contains(want) {
			t.Errorf("no line %q in:\n%s", want, codeText(r))
		}
	}

	for _, unwanted := range hasnt {
		if contains(unwanted) {
			t.Errorf("a line %q in:\n%s", unwanted, codeText(r))
		}
	}
}

// TestConstantSubscript checks the example at the top of forward.go: the index folds
// into the offset, the comparison settles the branch, and the CALL goes with its block.
func TestConstantSubscript(t *testing.T) {
	r := improved(t, "S",
		"SUBROUTINE S(A)",
		"REAL B(3)",
		"K = 3",
		"B(K) = 1.0",
		"IF (K .GT. 5) THEN",
		"CALL NEVER",
		"END IF",
		"A = B(K)",
		"END")

	expect(t, r, []string{"assign K = 3", "assign B+8 = 1.0", "assign @A = 1.0"}, []string{"call NEVER()"})

	if strings.Contains(codeText(r), "branch") {
		t.Errorf("a branch is left:\n%s", codeText(r))
	}
}

// TestMerge checks what's known after two ways in: what they agree on.
func TestMerge(t *testing.T) {
	r := improved(t, "S",
		"SUBROUTINE S(L, IA, IB)",
		"LOGICAL L",
		"IF (L) THEN",
		"J = 1",
		"K = 2",
		"ELSE",
		"J = 1",
		"K = 3",
		"END IF",
		"IA = J",
		"IB = K",
		"END")

	expect(t, r, []string{"assign @IA = 1", "assign @IB = K"}, nil)
}

// TestLoopTop checks that a variable the loop changes isn't constant in it, though it
// is on the way in, and one it doesn't is.
func TestLoopTop(t *testing.T) {
	r := improved(t, "S",
		"SUBROUTINE S(N, IA)",
		"L = 1",
		"M = 7",
		"DO 10 I = 1, N",
		"IA = L + M",
		"L = L + 1",
		"10:CONTINUE",
		"END")

	expect(t, r, []string{"assign @IA = add.L(L, 7)", "assign L = add.L(L, 1)"}, nil)
}

// TestEntryKnowsNothing checks that what was stored before an ENTRY statement isn't
// known after it, since a call may start there.
func TestEntryKnowsNothing(t *testing.T) {
	r := improved(t, "S",
		"SUBROUTINE S(IA)",
		"K = 1",
		"ENTRY T(IA)",
		"IA = K",
		"END")

	expect(t, r, []string{"assign K = 1", "assign @IA = K"}, nil)
}

// TestOverlaps checks that a store to part of a place ends what's known of it, and
// that a value stored as one type isn't read as another.
func TestOverlaps(t *testing.T) {
	r := improved(t, "S",
		"SUBROUTINE S(IA, IB, IC)",
		"INTEGER*2 H(2)",
		"EQUIVALENCE (W, H)",
		"INTEGER W",
		"REAL R",
		"EQUIVALENCE (R, I)",
		"W = 65537",
		"H(2) = 5",
		"IA = W",
		"I = 16448",
		"IB = R",
		"H(1) = 3",
		"IC = H(1)",
		"END")

	expect(t, r, []string{"assign @IA = W", "assign @IB = cvt.FL(R)", "assign @IC = 3"}, nil)
}

// TestWhatACallChanges checks Decision 7 from the pass's side: COMMON and a local
// whose address was given out at any time are unknown after a call, and another local
// is still known.
func TestWhatACallChanges(t *testing.T) {
	r := improved(t, "S",
		"SUBROUTINE S(IA, IB, IC)",
		"COMMON /X/ I",
		"I = 1",
		"J = 2",
		"K = 3",
		"CALL T",
		"IA = I",
		"IB = J",
		"IC = K",
		"CALL U(J)",
		"END")

	expect(t, r, []string{"assign @IA = I", "assign @IB = J", "assign @IC = 3"}, nil)
}

// TestVolatileStays checks that a VOLATILE variable's stores and loads all stay.
func TestVolatileStays(t *testing.T) {
	r := improved(t, "S",
		"SUBROUTINE S(IA)",
		"VOLATILE K",
		"K = 1",
		"K = 1",
		"IA = K",
		"END")

	if n := strings.Count(codeText(r), "assign K{volatile} = 1"); n != 2 {
		t.Errorf("%d stores of K:\n%s", n, codeText(r))
	}

	expect(t, r, []string{"assign @IA = K{volatile}"}, nil)
}

// TestRepeatedStore checks that a store of what's there goes, and one of something
// else doesn't.
func TestRepeatedStore(t *testing.T) {
	r := improved(t, "S",
		"SUBROUTINE S(IA)",
		"K = 1",
		"K = 1",
		"K = 2",
		"IA = K",
		"END")

	if n := strings.Count(codeText(r), "assign K = 1"); n != 1 {
		t.Errorf("%d stores of 1 in K:\n%s", n, codeText(r))
	}

	expect(t, r, []string{"assign K = 2", "assign @IA = 2"}, nil)
}

// TestRawConstant checks that bits that may be a reserved operand are moved, never
// read as a number: the second load must still fault.
func TestRawConstant(t *testing.T) {
	r := improved(t, "S",
		"SUBROUTINE S(A)",
		"REAL R",
		"R = '00008000'X",
		"A = R",
		"END")

	expect(t, r, []string{"assign @A = R"}, nil)
}

// TestFaultStaysAtRunTime checks Decision 8: an operation that would fault isn't
// folded, and its store isn't removed for having no reader.
func TestFaultStaysAtRunTime(t *testing.T) {
	r := improved(t, "S",
		"SUBROUTINE S(IA)",
		"K = 0",
		"J = 5/K",
		"X = 0.0",
		"Y = 1.0/X",
		"IA = 1",
		"END")

	expect(t, r, []string{"assign J = div.L(5, 0)", "assign Y = div.F(1.0, 0.0)"}, nil)
}

// TestSettledTerminators checks each kind of terminator that can be settled.
func TestSettledTerminators(t *testing.T) {
	r := improved(t, "S",
		"SUBROUTINE S(IA)",
		"K = 2",
		"GO TO (10, 20, 30), K",
		"IA = 9",
		"10:IA = 10",
		"20:IA = 20",
		"X = -1.5",
		"IF (X) 40, 50, 50",
		"30:IA = 30",
		"40:IA = 40",
		"K = 9",
		"GO TO (50, 60), K",
		"IA = 41",
		"GO TO 70",
		"50:IA = 50",
		"60:IA = 60",
		"70:END")

	expect(t, r, []string{"assign @IA = 20", "assign @IA = 40", "assign @IA = 41"},
		[]string{"assign @IA = 9", "assign @IA = 10", "assign @IA = 30", "assign @IA = 50", "assign @IA = 60"})
}

// TestCharacterLengths checks that a length known is put in if it's a length, and not
// if it can't be one.
func TestCharacterLengths(t *testing.T) {
	r := improved(t, "S",
		"SUBROUTINE S(A)",
		"CHARACTER*8 T, A",
		"N = 2",
		"M = 4",
		"T(N:M) = A",
		"N = 5",
		"M = 3",
		"T(N:M) = A",
		"END")

	text := codeText(r)

	if !strings.Contains(text, "T+1") {
		t.Errorf("the known place isn't there:\n%s", text)
	}

	if strings.Contains(text, ",-") || strings.Contains(text, "-1") {
		t.Errorf("a negative length is there:\n%s", text)
	}
}

// TestFunctionValue checks the two forms of a function's value: a constant, forwarded,
// and an expression, sunk into the return. Neither leaves a store.
func TestFunctionValue(t *testing.T) {
	r := improved(t, "F",
		"FUNCTION F(X)",
		"F = 0.0",
		"END")

	expect(t, r, []string{"return 0.0"}, []string{"assign F = 0.0"})

	r = improved(t, "F",
		"FUNCTION F(X)",
		"F = X/2.0",
		"END")

	expect(t, r, []string{"return div.F(@X, 2.0)"}, []string{"assign F = div.F(@X, 2.0)"})
}

// TestKeptFunctionValue checks that a function with a path that doesn't set its value
// keeps the value of the call before, as its static storage always did.
func TestKeptFunctionValue(t *testing.T) {
	r := improved(t, "KEEP",
		"INTEGER FUNCTION KEEP(L)",
		"LOGICAL L",
		"IF (L) THEN",
		"KEEP = 7",
		"RETURN",
		"END IF",
		"END")

	expect(t, r, []string{"assign KEEP = 7"}, nil)
}

// TestTakenFunctionValue checks that a function's value given to a routine stays.
func TestTakenFunctionValue(t *testing.T) {
	r := improved(t, "F",
		"FUNCTION F(X)",
		"F = 1.0",
		"CALL T(F)",
		"F = 2.0",
		"END")

	expect(t, r, []string{"assign F = 1.0", "assign F = 2.0"}, nil)
}

// TestDeadValueThatFaults checks that a store whose value may fault stays, and one
// whose value can't doesn't, in a function where both are overwritten.
func TestDeadValueThatFaults(t *testing.T) {
	r := improved(t, "F",
		"INTEGER FUNCTION F(K, J)",
		"F = 5/K",
		"F = J + 1",
		"F = 8",
		"END")

	expect(t, r, []string{"assign F = div.L(5, @K)", "return 8"}, []string{"assign F = add.L(@J, 1)", "assign F = 8"})
}

// TestVariablesKeepStores checks Decision 6: a variable's store stays with no reader
// in the routine, since the next call may read it.
func TestVariablesKeepStores(t *testing.T) {
	r := improved(t, "S",
		"SUBROUTINE S",
		"K = 1",
		"K = 2",
		"END")

	expect(t, r, []string{"assign K = 2"}, nil)
}

// TestDeadCount checks that a count is removed from a loop that can't run, and its
// cell with it, and the cells after it close up.
func TestDeadCount(t *testing.T) {
	r := improved(t, "S",
		"SUBROUTINE S(IA)",
		"N = 0",
		"DO 10 I = 5, N",
		"IA = 1",
		"10:CONTINUE",
		"DO 20 I = 1, 3",
		"IA = 2",
		"20:CONTINUE",
		"END")

	for _, c := range r.Cells {
		if c.Name == "$C1" {
			t.Errorf("the first loop's count is still there")
		}

		if c.Name == "$C2" && c.Offset != cellNamed(t, r, "I").Offset+4 {
			t.Errorf("the second loop's count is at %d", c.Offset)
		}
	}

	if want := cellNamed(t, r, "$C2").Offset + 4; r.LocalSize != want {
		t.Errorf("$LOCAL is %d bytes, want %d", r.LocalSize, want)
	}

	if err := ir.Check(r); err != nil {
		t.Error(err)
	}
}

// TestAssignedBlockStays checks that a label only an ASSIGN names keeps its block, and
// that one whose ASSIGN can't run doesn't.
func TestAssignedBlockStays(t *testing.T) {
	r := improved(t, "S",
		"SUBROUTINE S(IA)",
		"ASSIGN 20 TO K",
		"GO TO K",
		"IA = 1",
		"20:IA = 2",
		"END")

	expect(t, r, []string{"assign @IA = 2"}, []string{"assign @IA = 1"})

	r = improved(t, "S",
		"SUBROUTINE S(IA)",
		"L = 0",
		"IF (L .NE. 0) THEN",
		"ASSIGN 20 TO K",
		"END IF",
		"RETURN",
		"20:IA = 2",
		"END")

	expect(t, r, nil, []string{"assign @IA = 2"})
}

// TestPassesOnCorpus runs the passes on every routine to hand, each checked after
// each pass, and checks what they promise of a routine: every block can run, a second
// run changes nothing, and no VOLATILE store has gone.
func TestPassesOnCorpus(t *testing.T) {
	routines := corpus(t)

	before := 0

	for _, r := range routines {
		before += len(r.Blocks)
	}

	passes := beforeRegisters()
	run(routines, passes)

	after := 0

	for _, r := range routines {
		after += len(r.Blocks)

		g := NewGraph(r)

		for _, b := range r.Blocks {
			if !g.Reachable(b) {
				t.Errorf("%s: %s can't run", r.Name, b.Name)
			}
		}
	}

	first := ir.Dump(routines)

	run(routines, passes)

	if second := ir.Dump(routines); second != first {
		t.Errorf("a second run changed something:\n%s", firstDifference(first, second))
	}

	// The allocator is last, and a second run of it does nothing.
	allocate := Passes[len(Passes)-1:]
	run(routines, allocate)

	first = ir.Dump(routines)

	run(routines, allocate)

	if second := ir.Dump(routines); second != first {
		t.Errorf("a second run of the allocator changed something:\n%s", firstDifference(first, second))
	}

	t.Logf("%d routines, %d blocks to %d", len(routines), before, after)
}

// firstDifference is the first line of each text where they differ.
func firstDifference(a, b string) string {
	x, y := strings.Split(a, "\n"), strings.Split(b, "\n")

	for i := range min(len(x), len(y)) {
		if x[i] != y[i] {
			return x[i] + "\n" + y[i]
		}
	}

	return "one is longer"
}
