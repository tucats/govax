package opt

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/tucats/govax/internal/fortran/diag"
	"github.com/tucats/govax/internal/fortran/ir"
	"github.com/tucats/govax/internal/fortran/lex"
	"github.com/tucats/govax/internal/fortran/parse"
	"github.com/tucats/govax/internal/fortran/sema"
	"github.com/tucats/govax/internal/fortran/source"
	"github.com/tucats/govax/internal/fortran/sysdef"
)

// definitions opens govax's definition modules, as the FORTRAN command does last.
type definitions struct{}

// OpenInclude refuses: a test program includes only modules.
func (definitions) OpenInclude(name string, _ *source.File) (*source.File, error) {
	return nil, errors.New("no file " + name)
}

// OpenModule reads a definition module.
func (definitions) OpenModule(_, module string, _ *source.File) (*source.File, error) {
	lines, ok := sysdef.Module(module)
	if !ok {
		return nil, errors.New("no module " + module)
	}

	return &source.File{Spec: module, Lines: lines}, nil
}

// lowered runs every pass up to lowering over a program's lines. It reports false
// for a program with messages.
func lowered(spec string, lines []string) ([]*ir.Routine, bool) {
	var messages diag.List

	program := source.Read(source.Form{}, &source.File{Spec: spec, Lines: lines}, definitions{}, &messages)
	tree := parse.Parse(lex.Lex(program.Statements, &messages), &messages)
	prog := sema.Analyze(tree, "T$MAIN", sema.Options{}, &messages)

	if messages.Failed() {
		return nil, false
	}

	return ir.Lower(prog), true
}

// routineOf lowers a program given as lines, each after six blanks unless it
// starts with a label's digits or a tab of its own, and returns the routine named.
func routineOf(t *testing.T, name string, lines ...string) *ir.Routine {
	t.Helper()

	full := make([]string, len(lines))

	for i, line := range lines {
		label, rest, found := strings.Cut(line, ":")
		if _, err := strconv.Atoi(label); found && err == nil {
			full[i] = strings.Repeat(" ", 5-len(label)) + label + " " + rest
		} else {
			full[i] = "      " + line
		}
	}

	routines, ok := lowered("TEST", full)
	if !ok {
		t.Fatalf("the program doesn't compile:\n%s", strings.Join(full, "\n"))
	}

	for _, r := range routines {
		if r.Name == name {
			return r
		}
	}

	t.Fatalf("no routine %s", name)

	return nil
}

// labeled is the block with FORTRAN label n.
func labeled(t *testing.T, r *ir.Routine, n int) *ir.Block {
	t.Helper()

	for _, b := range r.Blocks {
		if b.Label == n {
			return b
		}
	}

	t.Fatalf("no block with label %d", n)

	return nil
}

// labels are the FORTRAN labels among the blocks, in order: what a test can say
// about a set of blocks without knowing how lowering numbered them.
func labels(list []*ir.Block) []int {
	var found []int

	for _, b := range list {
		if b.Label != 0 {
			found = append(found, b.Label)
		}
	}

	slices.Sort(found)

	return found
}

// TestGoToLoop finds a loop that has no DO statement, and what isn't in it.
func TestGoToLoop(t *testing.T) {
	r := routineOf(t, "S",
		"SUBROUTINE S(A, N, X)",
		"INTEGER A(N)",
		"I = 1",
		"10:IF (A(I) .GE. X) GO TO 20",
		"I = I + 1",
		"GO TO 10",
		"20:X = I",
		"END")

	g := NewGraph(r)

	if len(g.Loops) != 1 || len(g.Irreducible) != 0 {
		t.Fatalf("%d loops, %d irreducible edges", len(g.Loops), len(g.Irreducible))
	}

	l := g.Loops[0]
	top, after := labeled(t, r, 10), labeled(t, r, 20)

	if l.Header != top || l.Blocks[0] != top || !l.Contains(top) || l.Contains(after) {
		t.Errorf("the loop is headed by %s with %d blocks", l.Header.Name, len(l.Blocks))
	}

	if len(l.Exits) != 1 || l.Exits[0].To != after {
		t.Errorf("the loop's exits are %v", l.Exits)
	}

	if !g.Dominates(r.Blocks[0], after) || !g.Dominates(top, after) || g.Dominates(after, top) {
		t.Error("the dominators are wrong")
	}

	if g.IDom(r.Blocks[0]) != nil || g.LoopOf(after) != nil || g.LoopOf(top) != l {
		t.Error("IDom or LoopOf is wrong")
	}
}

// TestNesting checks a DO in a DO: which is inside which, and each one's depth.
func TestNesting(t *testing.T) {
	r := routineOf(t, "S",
		"SUBROUTINE S(N, K)",
		"DO 20 I = 1, N",
		"DO 10 J = 1, N",
		"K = K + 1",
		"10:CONTINUE",
		"K = K + 2",
		"20:CONTINUE",
		"END")

	g := NewGraph(r)

	if len(g.Loops) != 2 {
		t.Fatalf("%d loops", len(g.Loops))
	}

	outer, inner := g.Loops[0], g.Loops[1]

	if inner.Parent != outer || outer.Parent != nil || !slices.Contains(outer.Children, inner) {
		t.Error("the loops aren't nested")
	}

	if outer.Depth != 1 || inner.Depth != 2 {
		t.Errorf("the depths are %d and %d", outer.Depth, inner.Depth)
	}

	if !outer.Contains(inner.Header) || inner.Contains(outer.Header) {
		t.Error("the bodies aren't nested")
	}

	if g.LoopOf(inner.Header) != inner || g.LoopOf(labeled(t, r, 20)) != outer {
		t.Error("LoopOf isn't the innermost")
	}
}

// TestSharedHeader checks that two jumps back to one label are one loop.
func TestSharedHeader(t *testing.T) {
	r := routineOf(t, "S",
		"SUBROUTINE S(I)",
		"10:I = I + 1",
		"IF (I .EQ. 5) GO TO 10",
		"I = I + 2",
		"IF (I .LT. 9) GO TO 10",
		"END")

	g := NewGraph(r)

	if len(g.Loops) != 1 || len(g.Loops[0].Latches) != 2 {
		t.Fatalf("%d loops", len(g.Loops))
	}

	// The loop's header is the routine's first block, an entry too.
	if !g.IsEntry(g.Loops[0].Header) {
		t.Error("the header isn't the entry")
	}
}

// TestExtendedRange checks section 5.3.1.4's form: a block far from the DO in the
// text, which leaves and comes back, is in the loop.
func TestExtendedRange(t *testing.T) {
	r := routineOf(t, "S",
		"SUBROUTINE S(A, B, N)",
		"REAL A(N), B(N)",
		"DO 10 I = 1, N",
		"IF (A(I) .LT. 0.0) GO TO 100",
		"50:B(I) = A(I)",
		"10:CONTINUE",
		"RETURN",
		"100:A(I) = 0.0",
		"GO TO 50",
		"END")

	g := NewGraph(r)

	if len(g.Loops) != 1 || len(g.Irreducible) != 0 {
		t.Fatalf("%d loops, %d irreducible edges", len(g.Loops), len(g.Irreducible))
	}

	if got := labels(g.Loops[0].Blocks); !slices.Equal(got, []int{10, 50, 100}) {
		t.Errorf("the loop holds the labels %v", got)
	}
}

// TestEntryIntoLoop checks that an ENTRY statement in a loop's body is a second way
// in, so the cycle isn't a loop any pass may move code out of.
func TestEntryIntoLoop(t *testing.T) {
	r := routineOf(t, "S",
		"SUBROUTINE S(I)",
		"10:I = I + 1",
		"ENTRY T(I)",
		"I = I + 2",
		"IF (I .LT. 9) GO TO 10",
		"END")

	g := NewGraph(r)

	if len(g.Entries) != 2 {
		t.Errorf("%d entries", len(g.Entries))
	}

	if len(g.Loops) != 0 || len(g.Irreducible) != 1 {
		t.Errorf("%d loops, %d irreducible edges", len(g.Loops), len(g.Irreducible))
	}

	entry := g.Entries[1]
	if g.IDom(entry) != nil || g.Dominates(r.Blocks[0], entry) {
		t.Error("the ENTRY's block is dominated, though a call comes straight to it")
	}
}

// TestAssignedGoTo checks that an assigned GO TO's edges go to every label the unit
// ASSIGNs, listed or not (section 5.6.3), and that a loop it closes is found.
func TestAssignedGoTo(t *testing.T) {
	r := routineOf(t, "S",
		"SUBROUTINE S(I)",
		"ASSIGN 30 TO K",
		"20:I = I + 1",
		"IF (I .GT. 9) RETURN",
		"GO TO K, (20)",
		"30:I = I + 2",
		"ASSIGN 20 TO K",
		"GO TO K",
		"END")

	g := NewGraph(r)

	thirty := labeled(t, r, 30)

	if len(g.Preds(thirty)) != 2 || !g.Reachable(thirty) {
		t.Errorf("label 30 has %d predecessors", len(g.Preds(thirty)))
	}

	// The second jump may go to label 30 as well, so label 30 is a loop of its own,
	// inside the one the jumps to label 20 close.
	if len(g.Loops) != 2 || g.Loops[0].Header != labeled(t, r, 20) || !g.Loops[0].Contains(thirty) {
		t.Fatalf("%d loops", len(g.Loops))
	}

	if g.Loops[1].Header != thirty || g.Loops[1].Parent != g.Loops[0] {
		t.Errorf("the second loop is at %s", g.Loops[1].Header.Name)
	}

	if len(g.Entries) != 1 {
		t.Errorf("%d entries", len(g.Entries))
	}
}

// TestAssignedNoJump checks that a label only an ASSIGN names, with no GO TO to use
// it, is kept as an entry: a pass removing it would leave the ASSIGN's symbol
// undefined.
func TestAssignedNoJump(t *testing.T) {
	r := routineOf(t, "S",
		"SUBROUTINE S(I)",
		"ASSIGN 30 TO K",
		"RETURN",
		"30:I = 3",
		"END")

	g := NewGraph(r)

	if !g.Reachable(labeled(t, r, 30)) || !g.IsEntry(labeled(t, r, 30)) {
		t.Error("label 30 isn't kept")
	}
}

// TestUnreachable checks that code nothing reaches is known not to run.
func TestUnreachable(t *testing.T) {
	r := routineOf(t, "S",
		"SUBROUTINE S(I)",
		"RETURN",
		"I = 1",
		"END")

	g := NewGraph(r)

	if len(g.Order) == len(r.Blocks) {
		t.Fatalf("all %d blocks are reachable", len(r.Blocks))
	}

	for _, b := range r.Blocks {
		if !slices.Contains(g.Order, b) {
			if g.Reachable(b) || g.Dominates(b, b) || g.IDom(b) != nil {
				t.Errorf("%s can't run, but seems to", b.Name)
			}
		}
	}
}

// TestVFEEntries checks that each variable format expression is a way in.
func TestVFEEntries(t *testing.T) {
	r := routineOf(t, "S",
		"SUBROUTINE S(I)",
		"K = I + 1",
		"PRINT 100, I",
		"100:FORMAT (I<K>, I<K+1>)",
		"END")

	g := NewGraph(r)

	if len(g.Entries) != 3 || len(r.VFEs) != 1 {
		t.Fatalf("%d entries", len(g.Entries))
	}

	s := NewStorage(r)

	k := cellNamed(t, r, "K")
	if !s.Taken(s.Whole(&ir.Mem{Cell: k, T: k.T})) {
		t.Error("what a format expression reads isn't taken")
	}
}

// TestThroughPointers checks the forms of a place reached through a computed address
// that aren't a dummy's: an AUTOMATIC variable from a format expression, a packed
// record's field, and a dynamic string's text.
func TestThroughPointers(t *testing.T) {
	r := routineOf(t, "S",
		"SUBROUTINE S",
		"INTEGER K",
		"AUTOMATIC K",
		"K = 3",
		"PRINT 100, K",
		"100:FORMAT (I<K>)",
		"END")

	s := NewStorage(r)
	k := cellNamed(t, r, "K")
	direct := s.Whole(&ir.Mem{Cell: k, T: k.T})

	if direct.Region != FrameRegion || !s.Taken(direct) {
		t.Errorf("K is at %+v, taken %v", direct, s.Taken(direct))
	}

	found := false

	for _, v := range r.VFEs {
		for _, p := range s.BlockEffects(v.Cases[0]).Reads {
			found = found || (p.Region == FrameRegion && Conflict(p, direct))
		}
	}

	if !found {
		t.Error("the format expression doesn't read K's place in the frame")
	}

	// A packed record's field, whose stride isn't a multiple of its size, is reached
	// by an address computed on the spot, which goes no further than the instruction.
	r = routineOf(t, "T$MAIN",
		"STRUCTURE /P/",
		"BYTE B",
		"INTEGER*4 L",
		"END STRUCTURE",
		"RECORD /P/ X(4), Y(4)",
		"X(I).L = 1",
		"Y(I).L = 2",
		"X(2).B = 3",
		"END")

	s = NewStorage(r)
	w := writes(s, r)

	if len(w) != 3 || !Conflict(w[0], w[2]) || Conflict(w[0], w[1]) {
		t.Errorf("the fields' places are %+v", w)
	}

	x := cellNamed(t, r, "X")
	if s.Taken(s.Whole(&ir.Mem{Cell: x, T: x.T})) {
		t.Error("reaching a field took the record's address")
	}

	r = routineOf(t, "S",
		"SUBROUTINE S(C)",
		"CHARACTER*(*) C",
		"CHARACTER*4 D",
		"D = C // 'X'",
		"END")

	s = NewStorage(r)
	found = false

	for _, b := range r.Blocks {
		for _, p := range s.BlockEffects(b).Writes {
			found = found || p.Region == DynamicRegion
		}
	}

	if !found {
		t.Error("nothing writes the dynamic string's text")
	}
}

// TestBlockData checks that a routine with no blocks is an empty graph.
func TestBlockData(t *testing.T) {
	r := routineOf(t, "D", "BLOCK DATA D", "COMMON /C/ I", "DATA I /1/", "END")

	g, s := NewGraph(r), NewStorage(r)
	if len(g.Order) != 0 || len(g.Loops) != 0 || len(s.EntryWrites()) != 0 {
		t.Error("a block data subprogram has something")
	}
}

// cellNamed is the routine's cell of that name.
func cellNamed(t *testing.T, r *ir.Routine, name string) *ir.Cell {
	t.Helper()

	for _, c := range r.Cells {
		if c.Name == name {
			return c
		}
	}

	t.Fatalf("no cell %s", name)

	return nil
}

// places are the places a program's assignments to and from named scalars write,
// found in its blocks: what's written by the statement that is the nth assignment.
func writes(s *Storage, r *ir.Routine) []Place {
	var list []Place

	for _, b := range r.Blocks {
		for _, st := range b.Stmts {
			list = append(list, s.StmtEffects(st).Writes...)
		}
	}

	return list
}

// TestConflicts holds Conflict to each kind of overlap Decision 7 names: for each
// program, the writes of its statements, in order, conflict or don't as listed.
func TestConflicts(t *testing.T) {
	cases := []struct {
		name  string
		lines []string
		want  bool
	}{
		{"scalars", []string{"I = 1", "J = 2"}, false},
		{"one scalar", []string{"I = 1", "I = 2"}, true},
		{"EQUIVALENCE whole", []string{"EQUIVALENCE (I, J)", "I = 1", "J = 2"}, true},
		{"EQUIVALENCE half", []string{"INTEGER*2 H(2)", "EQUIVALENCE (I, H)", "I = 1", "H(2) = 2"}, true},
		{"EQUIVALENCE beside", []string{"INTEGER*2 H(4)", "EQUIVALENCE (I, H)", "I = 1", "H(3) = 2"}, false},
		{"elements, constant", []string{"DIMENSION A(4)", "A(1) = 1", "A(2) = 2"}, false},
		{"one element", []string{"DIMENSION A(4)", "A(2) = 1", "A(2) = 2"}, true},
		{"elements, variable", []string{"DIMENSION A(4)", "A(1) = 1", "A(K) = 2"}, true},
		{"two arrays, variable", []string{"DIMENSION A(4), B(4)", "A(K) = 1", "B(K) = 2"}, false},
		{"COMMON, same block", []string{"COMMON /C/ I, J", "I = 1", "J = 2"}, false},
		{"COMMON array", []string{"COMMON /C/ A(4), B(4)", "A(K) = 1", "A(2) = 2"}, true},
		{"COMMON, two arrays", []string{"COMMON /C/ A(4), B(4)", "A(K) = 1", "B(K) = 2"}, false},
		{"COMMON, two blocks", []string{"COMMON /C/ I /D/ J", "I = 1", "J = 2"}, false},
		{"COMMON and local", []string{"COMMON /C/ I", "I = 1", "J = 2"}, false},
		{"substrings", []string{"CHARACTER*8 S", "S(1:4) = 'A'", "S(5:8) = 'B'"}, false},
		{"substrings overlap", []string{"CHARACTER*8 S", "S(1:4) = 'A'", "S(4:8) = 'B'"}, true},
		{"substring, variable", []string{"CHARACTER*8 S", "S(1:4) = 'A'", "S(K:K) = 'B'"}, true},
		{"complex and its part", []string{"COMPLEX C", "REAL R(2)", "EQUIVALENCE (C, R)", "C = (1.0, 2.0)", "R(2) = 3.0"}, true},
		{"union's fields", []string{
			"STRUCTURE /U/", "UNION", "MAP", "INTEGER*4 L", "END MAP", "MAP", "INTEGER*2 H, K", "END MAP", "END UNION", "END STRUCTURE",
			"RECORD /U/ X", "X.L = 1", "X.K = 2",
		}, true},
		{"fields beside", []string{
			"STRUCTURE /U/", "INTEGER*4 L", "INTEGER*2 H, K", "END STRUCTURE",
			"RECORD /U/ X", "X.H = 1", "X.K = 2",
		}, false},
	}

	for _, c := range cases {
		r := routineOf(t, "T$MAIN", append(c.lines, "END")...)
		s := NewStorage(r)

		w := writes(s, r)
		if len(w) != 2 {
			t.Errorf("%s: %d writes", c.name, len(w))

			continue
		}

		if got := Conflict(w[0], w[1]); got != c.want {
			t.Errorf("%s: Conflict says %v of %+v and %+v", c.name, got, w[0], w[1])
		}

		if got := Conflict(w[1], w[0]); got != c.want {
			t.Errorf("%s: Conflict the other way round says %v", c.name, got)
		}
	}
}

// TestDummies checks the standard's rule on dummy arguments (the author's answer 1):
// each conflicts only with itself, and a CHARACTER dummy's and an adjustable array's
// places count as their dummies'.
func TestDummies(t *testing.T) {
	r := routineOf(t, "S",
		"SUBROUTINE S(A, B, N, C, D, W)",
		"DIMENSION A(N), B(4), W(N)",
		"CHARACTER*(*) C, D",
		"COMMON /K/ G",
		"A(N) = 1.0",   // 0
		"W(N) = 2.0",   // 1
		"B(N) = 3.0",   // 2
		"A(1) = 4.0",   // 3
		"B(1) = 5.0",   // 4
		"B(2) = 5.0",   // 5
		"C(1:1) = 'X'", // 6
		"D(1:1) = 'X'", // 7
		"C(2:2) = 'X'", // 8
		"G = 6.0",      // 9
		"END")

	s := NewStorage(r)
	w := writes(s, r)

	// Some statements write more than one place to get their work done; what
	// matters is the last write of each source statement.
	var last []Place

	for _, b := range r.Blocks {
		for _, st := range b.Stmts {
			if e := s.StmtEffects(st); len(e.Writes) > 0 {
				last = append(last, e.Writes[len(e.Writes)-1])
			}
		}
	}

	// Skip the writes the entry code's adjustable array bounds make.
	last = last[len(last)-10:]

	pairs := []struct {
		a, b int
		want bool
	}{
		{0, 1, false}, // two adjustable arrays
		{0, 2, false}, // adjustable and not
		{0, 3, true},  // one adjustable array
		{2, 4, true},  // an element and an element by subscript
		{4, 5, false}, // two constant elements of one dummy
		{6, 7, false}, // two CHARACTER dummies
		{6, 8, false}, // two characters of one
		{0, 9, false}, // a dummy and COMMON
		{6, 9, false},
	}

	for _, p := range pairs {
		if got := Conflict(last[p.a], last[p.b]); got != p.want {
			t.Errorf("statements %d and %d: Conflict says %v of %+v and %+v", p.a, p.b, got, last[p.a], last[p.b])
		}
	}

	if len(w) < 10 {
		t.Errorf("%d writes", len(w))
	}
}

// TestDummyDescriptor checks that a CHARACTER dummy's descriptor is read-only, and
// that reading it reads no dummy's text.
func TestDummyDescriptor(t *testing.T) {
	r := routineOf(t, "S",
		"SUBROUTINE S(C, N)",
		"CHARACTER*(*) C",
		"N = LEN(C)",
		"END")

	s := NewStorage(r)
	e := s.BlockEffects(r.Blocks[0])

	found := false

	for _, p := range e.Reads {
		if p.Region == DescriptorRegion {
			found = true

			if s.ChangedByCall(p) {
				t.Error("a call may change a dummy's descriptor")
			}
		}
	}

	if !found {
		t.Errorf("no read of the descriptor among %+v", e.Reads)
	}
}

// TestTaken holds the list of Decision 7 to each way an address leaves a unit, on
// one local named X; Y never leaves, and stays safe from calls throughout.
func TestTaken(t *testing.T) {
	cases := []struct {
		name  string
		lines []string
		want  bool
	}{
		{"not at all", []string{"X = 1", "CALL F"}, false},
		{"by reference", []string{"CALL F(X)"}, true},
		{"by descriptor", []string{"CHARACTER*4 X", "CALL F(X)"}, true},
		{"%REF", []string{"CALL F(%REF(X))"}, true},
		{"%DESCR", []string{"CALL F(%DESCR(X))"}, true},
		{"%VAL only", []string{"CALL F(%VAL(X))"}, false},
		{"a value in parentheses", []string{"CALL F((X))"}, false},
		{"%LOC", []string{"I = %LOC(X)"}, true},
		{"output", []string{"PRINT *, X"}, true},
		{"input", []string{"READ *, X"}, true},
		{"an element's", []string{"DIMENSION X(4)", "PRINT *, X(2)"}, true},
		{"a whole array's", []string{"DIMENSION X(4)", "PRINT *, X"}, true},
		{"NAMELIST", []string{"NAMELIST /N/ X", "WRITE (*, N)"}, true},
		{"NAMELIST, never used", []string{"NAMELIST /N/ X"}, false},
		{"format expression", []string{"INTEGER X", "PRINT 100, Y", "100:FORMAT (F<X>.2)"}, true},
		{"ASSOCIATEVARIABLE", []string{"INTEGER X", "OPEN (1, ACCESS='DIRECT', RECL=4, ASSOCIATEVARIABLE=X)"}, true},
		// FTNRTL returns the status, and the statement's own code stores it.
		{"IOSTAT", []string{"INTEGER X", "OPEN (1, IOSTAT=X)"}, false},
		{"internal file", []string{"CHARACTER*8 X", "WRITE (X, '(I3)') 1"}, true},
		{"an index only", []string{"INTEGER X, A(4)", "A(X) = 1", "CALL F"}, false},
		{"beside an element passed", []string{"DIMENSION A(4)", "EQUIVALENCE (X, A(3))", "CALL F(A(1))"}, true},
	}

	for _, c := range cases {
		// Y's assignment goes after the case's declarations.
		first := 0

		for first < len(c.lines) && isDeclaration(c.lines[first]) {
			first++
		}

		lines := slices.Concat(c.lines[:first], []string{"Y = 2"}, c.lines[first:], []string{"END"})

		r := routineOf(t, "T$MAIN", lines...)
		s := NewStorage(r)

		x, y := cellNamed(t, r, "X"), cellNamed(t, r, "Y")
		place := s.Whole(&ir.Mem{Cell: x, T: x.T})

		if got := s.Taken(place); got != c.want {
			t.Errorf("%s: Taken says %v of X", c.name, got)
		}

		if got := s.ChangedByCall(place); got != c.want {
			t.Errorf("%s: ChangedByCall says %v of X", c.name, got)
		}

		if s.Taken(s.Whole(&ir.Mem{Cell: y, T: y.T})) && !strings.Contains(c.name, "format") {
			t.Errorf("%s: Y is taken", c.name)
		}
	}
}

// isDeclaration reports whether a line of TestTaken's is a declaration.
func isDeclaration(line string) bool {
	for _, word := range []string{"DIMENSION", "CHARACTER", "INTEGER", "NAMELIST", "EQUIVALENCE"} {
		if strings.HasPrefix(line, word) {
			return true
		}
	}

	return false
}

// TestCallChanges checks what a call may change: COMMON and a dummy always, nothing
// in read-only storage, a local only if taken, and an EQUIVALENCE's other name.
func TestCallChanges(t *testing.T) {
	r := routineOf(t, "S",
		"SUBROUTINE S(D)",
		"COMMON /C/ G",
		"DIMENSION V(2)",
		"EQUIVALENCE (V(2), Z)",
		"L = 1",
		"G = 1.0",
		"D = 1.0",
		"Z = 2.0",
		"CALL F(V)",
		"END")

	s := NewStorage(r)

	var seen []bool

	for _, p := range writes(s, r) {
		seen = append(seen, s.ChangedByCall(p))
	}

	if want := []bool{false, true, true, true}; !slices.Equal(seen, want) {
		t.Errorf("ChangedByCall says %v, want %v", seen, want)
	}

	if s.ChangedByCall(Place{Region: DataRegion}) {
		t.Error("a call may change read-only data")
	}
}

// TestVolatile checks VOLATILE as Decision 7 and section 4.16 have it: on a name, on
// a block, through EQUIVALENCE, and on what merely overlaps.
func TestVolatile(t *testing.T) {
	r := routineOf(t, "S",
		"SUBROUTINE S(D)",
		"COMMON /C/ G, H",
		"COMMON /E/ K",
		"VOLATILE /C/, V, D",
		"EQUIVALENCE (V, W)",
		"EQUIVALENCE (U, K)",
		"V = 1",
		"W = 2",
		"G = 3",
		"D = 4",
		"K = 5",
		"U = 6",
		"Q = 7",
		"END")

	s := NewStorage(r)

	var seen []bool

	for _, p := range writes(s, r) {
		seen = append(seen, s.Volatile(p))
	}

	if want := []bool{true, true, true, true, false, false, false}; !slices.Equal(seen, want) {
		t.Errorf("Volatile says %v, want %v", seen, want)
	}

	if !s.ChangedByCall(writes(s, r)[0]) {
		t.Error("a call may not change a VOLATILE local")
	}
}

// TestEffects holds a statement's effects to what it says, for each form of place.
func TestEffects(t *testing.T) {
	r := routineOf(t, "S",
		"SUBROUTINE S(N)",
		"DIMENSION A(4)",
		"CHARACTER*4 X, Y",
		"A(I) = B + N",
		"X = Y",
		"IF (X .EQ. Y) CALL F(C, %VAL(K))",
		"J = LOC(N)",
		"END")

	s := NewStorage(r)

	name := func(p Place) string {
		for _, c := range r.Cells {
			if p.Region == LocalRegion && p.Lo >= c.Offset && p.Lo < c.Offset+c.Size {
				return c.Name
			}
		}

		if p.Param != nil {
			return p.Param.Name
		}

		return "?"
	}

	names := func(list []Place) string {
		found := make([]string, len(list))
		for i, p := range list {
			found[i] = name(p)
		}

		slices.Sort(found)

		return strings.Join(found, " ")
	}

	var got []string

	for _, b := range r.Blocks {
		for _, st := range b.Stmts {
			e := s.StmtEffects(st)
			got = append(got, "R "+names(e.Reads)+" W "+names(e.Writes)+" A "+names(e.Addressed)+" C "+strconv.FormatBool(e.Call))
		}
	}

	want := []string{
		"R B I N W A A  C false",
		"R Y W X A  C false",
		"R K W  A C C true",
		"R  W  A N C true",
	}

	if len(got) < len(want) {
		t.Fatalf("the effects are %q", got)
	}

	for i, w := range want[:3] {
		if got[i] != w {
			t.Errorf("statement %d: %q, want %q", i, got[i], w)
		}
	}

	// The IF's comparison is the terminator's.
	first := s.TermEffects(r.Blocks[0].End)
	if names(first.Reads) != "X Y" || first.Call || len(first.Writes) != 0 {
		t.Errorf("the IF reads %q", names(first.Reads))
	}
}

// TestEntryWrites checks what an entry point writes, and that a dummy in a routine
// with ENTRY statements is reached by reading its address's cell.
func TestEntryWrites(t *testing.T) {
	r := routineOf(t, "F",
		"REAL FUNCTION F(A)",
		"F = A",
		"RETURN",
		"ENTRY G(A)",
		"G = A + 1",
		"END")

	s := NewStorage(r)

	if len(s.EntryWrites()) != 1 {
		t.Fatalf("the entry points write %+v", s.EntryWrites())
	}

	cell := s.EntryWrites()[0]

	reads := func() bool {
		for _, p := range s.BlockEffects(r.Blocks[0]).Reads {
			if Conflict(p, cell) {
				return true
			}
		}

		return false
	}

	if !reads() {
		t.Error("reading a dummy doesn't read its address's cell")
	}
}

// corpus are the programs a test of the analyses runs on: every routine the compiler
// can make from the repository's programs alone.
func corpus(t *testing.T) []*ir.Routine {
	t.Helper()

	var files []string

	for _, pattern := range []string{
		"../codegen/testdata/*.for",
		"../ir/testdata/*.for",
		"../../../testdata/fortran/*.for",
		"../../../testdata/fortran/modules/*/*.for",
		"../../../testdata/opt84/kernels/*.for",
	} {
		found, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatal(err)
		}

		files = append(files, found...)
	}

	var all []*ir.Routine

	skipped := 0

	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}

		routines, ok := lowered(path, strings.Split(strings.TrimSuffix(string(data), "\n"), "\n"))
		if !ok {
			skipped++

			continue
		}

		all = append(all, routines...)
	}

	t.Logf("%d files, %d skipped, %d routines", len(files), skipped, len(all))

	if len(all) < 400 {
		t.Errorf("only %d routines", len(all))
	}

	return all
}

// reaches is the set of blocks an entry reaches without passing through avoid.
func reaches(g *Graph, avoid *ir.Block) map[*ir.Block]bool {
	seen := map[*ir.Block]bool{}

	var visit func(*ir.Block)

	visit = func(b *ir.Block) {
		if b == avoid || seen[b] {
			return
		}

		seen[b] = true

		for _, s := range g.Succs(b) {
			visit(s)
		}
	}

	for _, e := range g.Entries {
		visit(e)
	}

	return seen
}

// TestGraphsOnCorpus holds the graph, dominators, and loops of every routine to their
// definitions, computed the slow way, independent of the algorithms: a dominates b
// if and only if nothing reaches b once a is removed.
func TestGraphsOnCorpus(t *testing.T) {
	blocks, loops, irreducible, deepest := 0, 0, 0, 0

	for _, r := range corpus(t) {
		g := NewGraph(r)
		blocks += len(g.Order)
		loops += len(g.Loops)
		irreducible += len(g.Irreducible)

		if len(r.Blocks) > 0 && g.Entries[0] != r.Blocks[0] {
			t.Errorf("%s: the first entry isn't the first block", r.Name)
		}

		// Every block of the routine that's not in Order is one no entry reaches.
		all := reaches(g, nil)
		for _, b := range r.Blocks {
			if all[b] != g.Reachable(b) {
				t.Errorf("%s: %s: Reachable says %v", r.Name, b.Name, g.Reachable(b))
			}
		}

		for _, a := range g.Order {
			without := reaches(g, a)

			for _, b := range g.Order {
				if want := a == b || !without[b]; g.Dominates(a, b) != want {
					t.Errorf("%s: Dominates(%s, %s) is %v", r.Name, a.Name, b.Name, !want)
				}
			}
		}

		for _, b := range g.Order {
			checkEdges(t, g, b)
		}

		for _, l := range g.Loops {
			deepest = max(deepest, l.Depth)
			checkLoop(t, g, l)
		}

		for _, e := range g.Irreducible {
			if g.Dominates(e.To, e.From) {
				t.Errorf("%s: the edge %s to %s is a back edge", r.Name, e.From.Name, e.To.Name)
			}
		}
	}

	t.Logf("%d blocks, %d loops, depth up to %d, %d irreducible edges", blocks, loops, deepest, irreducible)
}

// checkEdges checks that one block's edges are as its terminator says, each with
// its other end's list, and that its place in Order is before its successors unless
// the edge closes a cycle.
func checkEdges(t *testing.T, g *Graph, b *ir.Block) {
	t.Helper()

	for _, want := range Successors(b.End) {
		if !slices.Contains(g.Succs(b), want) {
			t.Errorf("%s: %s doesn't go to %s", g.Routine.Name, b.Name, want.Name)
		}
	}

	for _, s := range g.Succs(b) {
		if !slices.Contains(g.Preds(s), b) {
			t.Errorf("%s: %s isn't a predecessor of %s", g.Routine.Name, b.Name, s.Name)
		}

		if g.number[s] <= g.number[b] && !closesCycle(g, b, s) {
			t.Errorf("%s: %s comes after %s, with no cycle", g.Routine.Name, b.Name, s.Name)
		}
	}
}

// closesCycle reports whether from can be reached from to.
func closesCycle(g *Graph, from, to *ir.Block) bool {
	seen := map[*ir.Block]bool{}

	var visit func(*ir.Block) bool

	visit = func(b *ir.Block) bool {
		if b == from {
			return true
		}

		if seen[b] {
			return false
		}

		seen[b] = true

		return slices.ContainsFunc(g.Succs(b), visit)
	}

	return visit(to)
}

// checkLoop holds one loop to the definition of a natural loop.
func checkLoop(t *testing.T, g *Graph, l *Loop) {
	t.Helper()

	name := g.Routine.Name + ": loop at " + l.Header.Name

	if l.Blocks[0] != l.Header {
		t.Errorf("%s: its first block is %s", name, l.Blocks[0].Name)
	}

	for _, b := range l.Blocks {
		if !g.Dominates(l.Header, b) {
			t.Errorf("%s: %s isn't dominated by the header", name, b.Name)
		}

		// A second way in would have to be through a predecessor outside, or an
		// entry; the header is the only block allowed either.
		if b != l.Header {
			if g.IsEntry(b) {
				t.Errorf("%s: %s is an entry", name, b.Name)
			}

			for _, p := range g.Preds(b) {
				if !l.Contains(p) {
					t.Errorf("%s: %s comes in from %s", name, b.Name, p.Name)
				}
			}
		}

		for _, s := range g.Succs(b) {
			exit := slices.Contains(l.Exits, Edge{b, s})
			if exit == l.Contains(s) {
				t.Errorf("%s: the edge %s to %s is wrongly an exit or not", name, b.Name, s.Name)
			}
		}
	}

	if len(l.Latches) == 0 {
		t.Errorf("%s: no latches", name)
	}

	for _, n := range l.Latches {
		if !l.Contains(n) || !slices.Contains(g.Succs(n), l.Header) {
			t.Errorf("%s: %s isn't a latch", name, n.Name)
		}
	}

	if l.Parent != nil {
		if l.Depth != l.Parent.Depth+1 {
			t.Errorf("%s: depth %d in a parent at %d", name, l.Depth, l.Parent.Depth)
		}

		for _, b := range l.Blocks {
			if !l.Parent.Contains(b) {
				t.Errorf("%s: %s isn't in the parent", name, b.Name)
			}
		}
	}

	// Loops either nest or are apart.
	for _, other := range g.Loops {
		if other == l {
			continue
		}

		shared, inside := 0, 0

		for _, b := range l.Blocks {
			if other.Contains(b) {
				shared++
			}
		}

		for _, b := range other.Blocks {
			if l.Contains(b) {
				inside++
			}
		}

		if shared != 0 && shared != len(l.Blocks) && inside != len(other.Blocks) {
			t.Errorf("%s: it overlaps the loop at %s", name, other.Header.Name)
		}
	}
}

// TestStorageOnCorpus checks what must hold of every place in every routine: every
// argument passed by reference is one a call may change or read-only, every region
// is one lowering is known to make, and an unknown place is rare enough to log.
func TestStorageOnCorpus(t *testing.T) {
	places, unknown, taken := 0, 0, 0

	for _, r := range corpus(t) {
		s := NewStorage(r)
		taken += len(s.taken)

		for _, b := range r.Blocks {
			e := s.BlockEffects(b)

			for _, list := range [][]Place{e.Reads, e.Writes, e.Addressed} {
				for _, p := range list {
					places++

					if p.Region == UnknownRegion {
						unknown++
					}

					if p.Hi < p.Lo {
						t.Errorf("%s: %s: a place running backwards: %+v", r.Name, b.Name, p)
					}

					// Nothing here stores in read-only storage.
					if !Conflict(p, p) && p.Lo != p.Hi && p.Region != DataRegion && p.Region != DescriptorRegion {
						t.Errorf("%s: %s: %+v doesn't conflict with itself", r.Name, b.Name, p)
					}
				}
			}

			for _, p := range e.Writes {
				if p.Region == DataRegion || p.Region == DescriptorRegion {
					t.Errorf("%s: %s: a write of %+v", r.Name, b.Name, p)
				}
			}

			for _, p := range e.Addressed {
				if p.Region != DataRegion && p.Region != DescriptorRegion && !s.ChangedByCall(p) {
					t.Errorf("%s: %s: %+v was let out, but no call may change it", r.Name, b.Name, p)
				}
			}
		}

		for _, d := range r.Data {
			if d.Desc == nil || d.Desc.Target == nil || d.Desc.Target.Cell == nil {
				continue
			}

			if !s.ChangedByCall(s.Whole(d.Desc.Target)) {
				t.Errorf("%s: %s describes %s, which no call may change", r.Name, d.Name, d.Desc.Target.Cell.Name)
			}
		}
	}

	t.Logf("%d places, %d unknown, %d ranges taken", places, unknown, taken)

	if unknown*1000 > places {
		t.Errorf("%d of %d places unknown", unknown, places)
	}
}

// TestAstsProgram settles the question Phase 84's subtask 2 left, on the program that
// asked it: SHARED's static variable, changed by a second run of SHARED from an AST
// during $DCLAST, is in an I/O list, so a call may change it, and that's when it does.
func TestAstsProgram(t *testing.T) {
	data, err := os.ReadFile("../../../testdata/fortran/asts.for")
	if err != nil {
		t.Fatal(err)
	}

	routines, ok := lowered("asts.for", strings.Split(strings.TrimSuffix(string(data), "\n"), "\n"))
	if !ok {
		t.Fatal("asts.for doesn't compile")
	}

	for _, r := range routines {
		if r.Name != "SHARED" {
			continue
		}

		s := NewStorage(r)

		for _, name := range []string{"STAT", "AUTO"} {
			c := cellNamed(t, r, name)
			if !s.ChangedByCall(s.Whole(&ir.Mem{Cell: c, T: c.T})) {
				t.Errorf("no call may change %s", name)
			}
		}

		return
	}

	t.Error("no routine SHARED")
}

// TestOptastsProgram is the same for optasts.for's REENT, whose static variable no
// call is given: VOLATILE is all that says it can change.
func TestOptastsProgram(t *testing.T) {
	data, err := os.ReadFile("../../../testdata/fortran/optasts.for")
	if err != nil {
		t.Fatal(err)
	}

	routines, ok := lowered("optasts.for", strings.Split(strings.TrimSuffix(string(data), "\n"), "\n"))
	if !ok {
		t.Fatal("optasts.for doesn't compile")
	}

	for _, r := range routines {
		if r.Name != "REENT" {
			continue
		}

		s := NewStorage(r)
		c := cellNamed(t, r, "STAT")
		p := s.Whole(&ir.Mem{Cell: c, T: c.T})

		if s.Taken(p) || !s.Volatile(p) || !s.ChangedByCall(p) {
			t.Errorf("STAT: taken %v, volatile %v, changed by a call %v", s.Taken(p), s.Volatile(p), s.ChangedByCall(p))
		}

		return
	}

	t.Error("no routine REENT")
}
