package opt

import (
	"slices"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/fortran/codegen"
	"github.com/tucats/govax/internal/fortran/ir"
)

// allocated is a routine with every pass run on it, the register allocator among them.
func allocated(t *testing.T, name string, lines ...string) *ir.Routine {
	t.Helper()

	r := routineOf(t, name, lines...)
	run([]*ir.Routine{r}, Passes)

	return r
}

// allocatedMacro is routine S's MACRO, with every pass run on it.
func allocatedMacro(t *testing.T, lines ...string) string {
	t.Helper()

	r := allocated(t, "S", lines...)

	var w codegen.Writer

	codegen.Generate(&w, []*ir.Routine{r}, nil, codegen.Options{Improve: Improve})

	return strings.Join(w.Lines(), "\n")
}

// inRegisters are what a routine's registers hold, as the storage map says it: a loop's
// variable by the variable's name, a dummy's address by "A's address", a compiler's cell by
// its name.
func inRegisters(r *ir.Routine) []string {
	var list []string

	for _, c := range r.Cells {
		switch {
		case c.Register == 0:
		case strings.HasSuffix(c.Holds, ", during a loop"):
			list = append(list, strings.TrimSuffix(c.Holds, ", during a loop"))
		case c.Holds != "":
			list = append(list, c.Holds)
		default:
			list = append(list, c.Name)
		}
	}

	return list
}

// TestWhatGetsARegister checks each rule for a variable or a dummy in a loop.
func TestWhatGetsARegister(t *testing.T) {
	start := []string{
		"SUBROUTINE S(IB, IO, A, CH)",
		"INTEGER I, J, N, IB, IO, IC, A(10), K, W(2), EQ",
		"INTEGER*2 HALF(2)",
		"CHARACTER*(*) CH",
		"VOLATILE W",
		"EQUIVALENCE (EQ, HALF)",
		"COMMON /V/ IC, N",
	}

	cases := []struct {
		name      string
		lines     []string
		has, hasn []string
	}{
		{"a local across a call", []string{"DO 10 I = 1, N", "J = J + 1", "CALL NEXT", "10:CONTINUE"}, []string{"I", "J"}, nil},
		{"a local passed, across a call", []string{"CALL NEXT(J)", "DO 10 I = 1, N", "J = J + 1", "CALL NEXT", "10:CONTINUE"}, []string{"I"}, []string{"J"}},
		{"a local passed, no call in the loop", []string{"CALL NEXT(J)", "DO 10 I = 1, N", "J = J + 1", "10:CONTINUE"}, []string{"I", "J"}, nil},
		{"a local in the I/O list, no call in the loop", []string{"PRINT *, J", "DO 10 I = 1, N", "J = J + 1", "10:CONTINUE"}, []string{"J"}, nil},
		{"COMMON with no call", []string{"DO 10 I = 1, N", "IC = IC + 1", "10:CONTINUE"}, []string{"IC"}, nil},
		{"COMMON across a call", []string{"DO 10 I = 1, 5", "IC = IC + 1", "CALL NEXT", "10:CONTINUE"}, []string{"I"}, []string{"IC"}},
		{"VOLATILE", []string{"DO 10 I = 1, N", "J = W(1)", "10:CONTINUE"}, []string{"J"}, []string{"W"}},
		{"EQUIVALENCE", []string{"DO 10 I = 1, N", "EQ = EQ + 1", "10:CONTINUE"}, []string{"I"}, []string{"EQ"}},
		{"a dummy read on every trip, moved out of the loop", []string{"DO 10 I = 1, N", "J = J + IB", "10:CONTINUE"}, []string{"$H1"}, []string{"IB's address"}},
		{"a dummy read and written on every trip", []string{"DO 10 I = 1, N", "IO = IO + I", "10:CONTINUE"}, []string{"IO"}, []string{"IO's address"}},
		{"a dummy read under a test", []string{"DO 10 I = 1, N", "IF (I .GT. J) J = J + IB", "10:CONTINUE"}, []string{"IB's address"}, []string{"IB"}},
		{"a dummy written on every trip", []string{"DO 10 I = 1, N", "IO = I", "10:CONTINUE"}, []string{"IO"}, []string{"IO's address"}},
		{"a dummy written under a test", []string{"DO 10 I = 1, N", "IF (I .GT. J) IO = I", "10:CONTINUE"}, []string{"IO's address"}, []string{"IO"}},
		{"a dummy across a call", []string{"DO 10 I = 1, N", "J = J + IB", "CALL NEXT", "10:CONTINUE"}, []string{"IB's address"}, []string{"IB"}},
		{"an array's address", []string{"DO 10 I = 1, N", "J = J + A(I)", "10:CONTINUE"}, []string{"A's address"}, nil},
		{"a CHARACTER dummy's text", []string{"DO 10 I = 1, N", "IF (CH(I:I) .EQ. 'X') J = J + 1", "10:CONTINUE"}, []string{"an address in "}, nil},
		{"an assigned GO TO", []string{"ASSIGN 10 TO K", "DO 10 I = 1, N", "J = J + 1", "GO TO K", "10:CONTINUE"}, nil, []string{"I", "J"}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := allocated(t, "S", append(append(append([]string{}, start...), c.lines...), "END")...)
			got := inRegisters(r)

			for _, want := range c.has {
				if !slices.ContainsFunc(got, func(s string) bool { return s == want || (strings.HasSuffix(want, " ") && strings.HasPrefix(s, want)) }) {
					t.Errorf("%s isn't in a register: %v", want, got)
				}
			}

			for _, unwanted := range c.hasn {
				if slices.Contains(got, unwanted) {
					t.Errorf("%s is in a register: %v", unwanted, got)
				}
			}
		})
	}
}

// expectCount checks that a routine's code has the line just count times.
func expectCount(t *testing.T, r *ir.Routine, line string, count int) {
	t.Helper()

	if n := strings.Count("\n"+codeText(r)+"\n", "\n"+line+"\n"); n != count {
		t.Errorf("%q is there %d times, want %d, in:\n%s", line, n, count, codeText(r))
	}
}

// TestWriteBack checks where a variable's value is copied in and given back.
func TestWriteBack(t *testing.T) {
	r := allocated(t, "S",
		"SUBROUTINE S(N, R)",
		"INTEGER N, R, I, J, K",
		"K = N",
		"DO 10 I = 1, N",
		"J = J + K",
		"IF (J .GT. 100) GO TO 20",
		"10:CONTINUE",
		"20:R = J",
		"END")

	// J is written, and has two ways out; K isn't written.
	expectCount(t, r, "assign K = $R3", 0)
	expectCount(t, r, "assign J = "+holder(t, r, "J"), 2)
	expectCount(t, r, "assign "+holder(t, r, "J")+" = J", 1)
}

// holder is the name of the cell holding a variable in a loop.
func holder(t *testing.T, r *ir.Routine, name string) string {
	t.Helper()

	for _, c := range r.Cells {
		if c.Holds == name+", during a loop" {
			return c.Name
		}
	}

	t.Fatalf("no cell holds %s: %v", name, inRegisters(r))

	return ""
}

// TestCopiesKeepBits checks that a REAL's copies are integer moves, which a reserved
// operand can't stop.
func TestCopiesKeepBits(t *testing.T) {
	text := allocatedMacro(t,
		"SUBROUTINE S",
		"REAL X, Y(10)",
		"REAL*8 D",
		"COMMON /V/ X, Y, D",
		"DO 10 I = 1, 5",
		"Y(I) = X",
		"D = D*2.0D0",
		"10:CONTINUE",
		"END")

	for _, want := range []string{"MOVL\tS.X,R", "MOVQ\tS.D,R", "MOVQ\tR"} {
		if !strings.Contains(text, want) {
			t.Errorf("no %q in:\n%s", want, text)
		}
	}

	if strings.Contains(text, "MOVF\tS.X,R") {
		t.Errorf("a floating copy of X:\n%s", text)
	}
}

// TestSharing checks that cells not live together share a register, and those that are
// don't.
func TestSharing(t *testing.T) {
	r := allocated(t, "S",
		"SUBROUTINE S(N)",
		"INTEGER N, I, J, K, L",
		"DO 10 I = 1, N",
		"K = K + I",
		"10:CONTINUE",
		"DO 20 J = 1, N",
		"L = L + J",
		"20:CONTINUE",
		"END")

	registers := map[string]int{}

	for _, c := range r.Cells {
		if c.Register != 0 {
			registers[c.Holds] = c.Register
		}
	}

	if registers["I, during a loop"] == registers["K, during a loop"] {
		t.Errorf("two values of one loop share a register: %v", registers)
	}

	used := map[int]bool{}
	for _, reg := range registers {
		used[reg] = true
	}

	if len(used) >= len(registers) {
		t.Errorf("the second loop's values share nothing with the first's: %v", registers)
	}
}

// TestNoMoreThanSix checks that a loop with too many values uses every register, the most
// used value among them.
func TestNoMoreThanSix(t *testing.T) {
	lines := []string{"SUBROUTINE S", "INTEGER I, A, B, C, D, E, F, G, H", "DO 10 I = 1, 9"}

	for _, name := range []string{"A", "B", "C", "D", "E", "F", "G"} {
		lines = append(lines, name+" = "+name+" + 1")
	}

	lines = append(lines, "H = H + H + H + H + H + 1", "10:CONTINUE", "END")

	r := allocated(t, "S", lines...)

	seen := map[int]bool{}

	for _, c := range r.Cells {
		if c.Register != 0 {
			if c.Register < firstCellRegister || c.Register > lastCellRegister || seen[c.Register] {
				t.Errorf("%s is in R%d, used twice or not to give", c.Name, c.Register)
			}

			seen[c.Register] = true
		}
	}

	if len(seen) != 6 || !slices.Contains(inRegisters(r), "H") {
		t.Errorf("in registers: %v", inRegisters(r))
	}
}

// TestExpressionsKeepTheirRegisters checks that what a large H_floating expression
// uses isn't given away.
func TestExpressionsKeepTheirRegisters(t *testing.T) {
	r := routineOf(t, "S",
		"SUBROUTINE S",
		"REAL*16 A, B, C, D, E, F",
		"COMMON /V/ A, B, C, D, E, F",
		"DO 10 I = 1, 9",
		"F = (A+B)*(C+D)*E + (A-B)*(C-D)*E",
		"10:CONTINUE",
		"END")
	run([]*ir.Routine{r}, beforeRegisters())

	busy := codegen.ExpressionRegisters(r)

	Registers(r)

	for _, c := range r.Cells {
		for i := range c.T.Width() {
			if c.Register != 0 && busy[c.Register+i] {
				t.Errorf("%s has R%d, which an expression uses", c.Name, c.Register+i)
			}
		}
	}
}

// TestLiveOnEntryStaysInMemory checks that a value lasting from one call to the next isn't
// a register for the whole routine: a function's value on a path that doesn't set it. For
// a loop it can be, with its home kept up.
func TestLiveOnEntryStaysInMemory(t *testing.T) {
	r := allocated(t, "KEEP",
		"INTEGER FUNCTION KEEP(L)",
		"LOGICAL L",
		"DO 10 I = 1, 3",
		"IF (L) KEEP = KEEP + I",
		"10:CONTINUE",
		"END")

	for _, c := range r.Cells {
		if c.Sym != nil && c.Kind == ir.ResultCell && c.Register != 0 {
			t.Errorf("%s is in R%d", c.Name, c.Register)
		}
	}

	if !slices.Contains(inRegisters(r), "KEEP") {
		t.Errorf("in registers: %v", inRegisters(r))
	}
}

// TestFormatExpressionsStayInMemory checks that nothing a variable format expression reads
// is a register in a loop with its I/O statement, since FTNRTL's registers are there then.
func TestFormatExpressionsStayInMemory(t *testing.T) {
	r := allocated(t, "S",
		"SUBROUTINE S",
		"INTEGER I, NW",
		"DO 10 I = 1, 3",
		"NW = I + 2",
		"PRINT 20, I",
		"20:FORMAT (I<NW>)",
		"10:CONTINUE",
		"END")

	if got := inRegisters(r); slices.Contains(got, "NW") {
		t.Errorf("NW is in a register: %v", got)
	}
}

// TestRegisterCode checks the operands the code generator writes for what's in registers.
func TestRegisterCode(t *testing.T) {
	cases := []struct {
		name  string
		lines []string
		has   []string
		hasnt []string
	}{
		{"an index, used where it is", []string{
			"SUBROUTINE S", "REAL A(10)", "COMMON /V/ A, N", "DO 10 I = 1, N", "A(I) = 0.0", "10:CONTINUE", "END",
		}, []string{"CLRF\tS.A-4[R", "AOBLEQ\tR"}, []string{"MOVL\tS.I,R0"}},

		{"a dummy's element by a constant subscript", []string{
			"SUBROUTINE S(A, N)", "REAL A(10)", "DO 10 I = 1, N", "X = X + A(3)", "CALL NEXT", "10:CONTINUE", "END",
		}, []string{"8(R"}, []string{"@S.A(AP)["}},

		{"a dummy's element by a subscript", []string{
			"SUBROUTINE S(A, N)", "REAL A(N)", "DO 10 I = 1, N", "X = X + A(I)", "10:CONTINUE", "END",
		}, []string{"(R"}, nil},

		{"a base and an index", []string{
			"SUBROUTINE S(N, J)", "REAL A(5,5)", "COMMON /V/ A", "L = J", "DO 10 I = 1, N", "A(I,L) = 0.0", "10:CONTINUE", "END",
		}, []string{"MOVAL\tS.A", ")[R"}, nil},

		{"the entry mask", []string{
			"SUBROUTINE S(N)", "DO 10 I = 1, N", "J = J + I", "10:CONTINUE", "END",
		}, []string{"^M<R6"}, nil},

		{"nothing in a routine with no loop", []string{
			"SUBROUTINE S(N)", "J = N + 1", "K = J + N", "END",
		}, []string{"^M<>"}, nil},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			text := allocatedMacro(t, c.lines...)

			for _, want := range c.has {
				if !strings.Contains(text, want) {
					t.Errorf("no %q in:\n%s", want, text)
				}
			}

			for _, unwanted := range c.hasnt {
				if strings.Contains(text, unwanted) {
					t.Errorf("%q in:\n%s", unwanted, text)
				}
			}
		})
	}
}

// TestSharedResultsStayInMemory checks that a function's names, which share one place
// when it has ENTRY statements, never get a register by liveness: a store to G is a
// store to F, which the return reads (testdata/fortran/recursion.for found it).
func TestSharedResultsStayInMemory(t *testing.T) {
	r := allocated(t, "F",
		"LOGICAL FUNCTION F(N)",
		"LOGICAL G",
		"F = N .EQ. 0",
		"RETURN",
		"ENTRY G(N)",
		"G = N .NE. 0",
		"END")

	if got := inRegisters(r); len(got) != 0 {
		t.Errorf("in registers: %v", got)
	}
}
