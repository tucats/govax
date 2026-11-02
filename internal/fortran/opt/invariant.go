package opt

import (
	"slices"
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/ir"
)

// Loop-invariant code motion: computing once, before a loop, what every trip computes
// the same.
//
//	      DO 40 K = 1, N
//	         DO 30 I = 1, N
//	            C(I,J) = C(I,J) + A(I,K)*B(K,J)
//	   30    CONTINUE
//	   40 CONTINUE
//
// With 60 rows, C's and B's subscripts both start from J*60, which neither loop changes.
// A's starts from K*60, which the inner loop doesn't change, and B(K,J) as a whole is
// the same on every trip of the inner loop. So
//
//	      $H1 = J*60                 before DO 40
//	      DO 40 K = 1, N
//	         $H2 = K*60              before DO 30
//	         $H3 = B($H1+K)
//	         DO 30 I = 1, N
//	            C($H1+I) = C($H1+I) + A($H2+I)*$H3
//
// which is what this pass makes.
//
// # Which loops
//
// The natural loops of the control flow graph, not the DO statements, so a GO TO loop
// is as good as any (Decision 1 of docs/PHASE-84 - FORTRAN optimizer.md). A loop's
// header is its only way in, so a new block before the header, its preheader, runs once
// on every path into the loop, and only those. A DO loop's zero-trip test comes before
// its header, so its preheader runs only if the body will. Outer loops go first: a value
// goes as far out as it can, and what's left is for the loops inside. A loop whose
// header is reached by an ENTRY statement or from outside by an assigned GO TO can't
// have a preheader, as nothing can be put on those paths, and is left alone.
//
// # What doesn't change
//
// An expression is invariant if nothing in the loop writes what it reads: no store
// Conflict says can touch it, and no call that may change it (Storage.ChangedByCall).
// Nothing reading something VOLATILE moves (Decision 7).
//
// # What may move
//
// Integer arithmetic can always move, overflow not being enabled. Whatever can fault
// can move only if the program was going to run it anyway (Decision 8): a division, any
// floating operation or load (a reserved operand faults), and a load through an index or
// a dummy argument, whose address may be bad. In
//
//	      DO 10 I = 1, N
//	         IF (Z .EQ. 0.0) GO TO 90
//	         X(I) = Y/Z
//	   10 CONTINUE
//
// Y/Z before the loop would fault where the program never divides by zero. So a value
// that can fault needs one use that the first trip surely reaches (surely): in a block
// that every latch and every exit of the loop comes through, with no call anywhere before
// it, since a call may not come back (IF (Z .EQ. 0.0) CALL GIVEUP). That is stricter
// than Decision 8's "every trip", which an exit on the first trip gets past. With such a
// use, the program ran it on the first trip anyway, with the same operands, and the other
// uses are reached later or not at all. The second operand of a branch's .AND. or .OR.
// isn't sure, as ever. What can't move as a whole may have parts that can.
//
// # What it's worth
//
// The value goes in a temporary ($H1), which the register allocator (registers.go) most
// likely puts in a register, since it is read on every trip. Then each use of J*60 as an
// index is no instruction at all:
//
//	      MULL3   J,#60,R0                ... A-244[R6] ...
//	      ... A-244[R0] ...
//
// If registers run out and the temporary stays in memory, a use costs one MOVL and a
// read, no more than the MULL3 and its read, and the only loss is a store each time the
// loop starts. So anything that takes an instruction or a read moves (movable). An
// array's base address and an adjustable array's A0 are the allocator's own affair,
// since they gain nothing except in a register.

// LoopInvariants is the pass.
func LoopInvariants(r *ir.Routine) {
	m := newMover(r, NewStorage(r))
	expressions := expressionBlocks(r)
	done := map[*ir.Block]bool{}

	// Moving code changes the graph, so each loop gets a new one, outer loops first.
	for {
		g := NewGraph(r)

		var next *Loop

		for _, l := range g.Loops {
			if !done[l.Header] {
				next = l

				break
			}
		}

		if next == nil {
			return
		}

		done[next.Header] = true

		if !expressions[next.Header] && canPrecede(g, next) {
			m.loop(g, next)
		}
	}
}

// canPrecede reports whether a block can be put on every path into loop l, and on no
// other. The first block of a routine is an entry, but one that a new first block can
// go before; another entry isn't, and nor is an assigned GO TO from outside, whose
// variable holds the header's address.
func canPrecede(g *Graph, l *Loop) bool {
	r := g.Routine

	for _, e := range r.Entries {
		if e.First == l.Header {
			return false
		}
	}

	if g.IsEntry(l.Header) && l.Header != r.Blocks[0] {
		return false
	}

	for _, p := range g.Preds(l.Header) {
		if _, jumps := p.End.(*ir.IndirectJump); jumps && !l.Contains(p) {
			return false
		}
	}

	return true
}

// mover is the pass's state for one routine.
type mover struct {
	c      *cse
	named  int
	blocks int
}

// newMover starts on routine r, going on from what an earlier run named.
func newMover(r *ir.Routine, s *Storage) *mover {
	m := &mover{c: &cse{r: r, s: s}, named: highestNamed(r, "$H")}

	for _, b := range r.Blocks {
		if strings.HasPrefix(b.Name, "P") {
			m.blocks++
		}
	}

	return m
}

// invariant is an expression found in a loop that doesn't change there.
type invariant struct {
	key  string
	e    ir.Expr
	from ast.Stmt

	// uses is how many times the loop computes it; instructions what those uses cost
	// as they are; and sure whether the first trip surely reaches one.
	uses, instructions int
	sure               bool
}

// loop moves what it can out of loop l.
func (m *mover) loop(g *Graph, l *Loop) {
	changed, call := m.changedIn(l)
	refused := map[string]bool{}

	for {
		found := m.scan(g, l, changed, call, refused)
		again := false

		for _, v := range found {
			if !v.movable() {
				refused[v.key] = true
				again = true
			}
		}

		if again {
			continue
		}

		if len(found) > 0 {
			m.hoist(g, l, found)
		}

		return
	}
}

// movable reports whether v can move out of the loop, and gains by it.
func (v *invariant) movable() bool {
	if unsafeToMove(v.e) && !v.sure {
		return false
	}

	return v.instructions+v.uses*costOf(v.e).reads >= 1
}

// changedIn is every place loop l's statements write, and whether any of them calls.
func (m *mover) changedIn(l *Loop) (writes []Place, call bool) {
	for _, b := range l.Blocks {
		e := m.c.s.BlockEffects(b)
		writes = append(writes, e.Writes...)
		call = call || e.Call
	}

	return writes, call
}

// scan is the largest invariant expressions in loop l, by what they compute: those
// refused don't count, but what's inside them may.
func (m *mover) scan(g *Graph, l *Loop, changed []Place, call bool, refused map[string]bool) []*invariant {
	var found []*invariant

	byKey := map[string]*invariant{}

	for _, b := range l.Blocks {
		sure := surely(g, l, b)

		var from ast.Stmt

		walk := walker{visit: func(e ir.Expr, _, maybe bool) bool {
			if !m.c.candidate(e) || !m.unchanged(e, changed, call) {
				return true
			}

			key := exprKey(e)
			if refused[key] {
				return true
			}

			v := byKey[key]
			if v == nil {
				v = &invariant{key: key, e: e, from: from}
				byKey[key] = v
				found = append(found, v)
			}

			v.uses++
			v.instructions += costOf(e).instructions
			v.sure = v.sure || (sure && !maybe)

			return false
		}}

		for _, st := range b.Stmts {
			from = st.Source()
			walk.stmt(st)

			if _, calls := st.(*ir.Call); calls {
				sure = false
			}
		}

		from = b.End.Source()
		walk.terminator(b.End)
	}

	return found
}

// unchanged reports whether nothing in the loop changes what e reads.
func (m *mover) unchanged(e ir.Expr, changed []Place, call bool) bool {
	for _, p := range m.c.readsOf(e) {
		if overlapsAny(p, changed) || (call && m.c.s.ChangedByCall(p)) {
			return false
		}
	}

	return true
}

// surely reports whether the first trip of loop l surely runs block b, up to its first
// call: every way round the loop and out of it comes through b, and nothing that runs
// before b calls anything. A block b doesn't dominate is one that runs before b or not
// at all on that trip, since it can reach a latch and only through b.
func surely(g *Graph, l *Loop, b *ir.Block) bool {
	for _, latch := range l.Latches {
		if !g.Dominates(b, latch) {
			return false
		}
	}

	for _, e := range l.Exits {
		if !g.Dominates(b, e.From) {
			return false
		}
	}

	for _, other := range l.Blocks {
		if g.Dominates(b, other) {
			continue
		}

		for _, st := range other.Stmts {
			if _, calls := st.(*ir.Call); calls {
				return false
			}
		}
	}

	return true
}

// unsafeToMove reports whether computing e could fault where the program wouldn't have:
// mayFault's list (deadcode.go), which is enough to keep a store, and a load from a place
// that may not be there, which an earlier test may have been keeping off it. Anything
// else, it says yes to.
func unsafeToMove(e ir.Expr) bool {
	switch e := e.(type) {
	case *ir.Const:
		return false

	case *ir.Load:
		m := e.Mem

		return e.Type().Floating() || m.Index != nil || m.Ptr != nil || m.Param != nil

	case *ir.Op:
		if e.Op == ir.Div || e.T.Floating() || e.X.Type().Floating() {
			return true
		}

		return unsafeToMove(e.X) || (e.Y != nil && unsafeToMove(e.Y))
	}

	return true
}

// hoist makes a preheader for loop l that computes each value, and has the loop read
// its temporary instead. A value may be inside another found elsewhere (K*3, and
// K+K*3), so the longest go first, while they can still be recognized, and each shorter
// one is computed ahead of those and replaced in them too.
func (m *mover) hoist(g *Graph, l *Loop, found []*invariant) {
	r := g.Routine
	pre := m.preheader(g, l)

	slices.SortStableFunc(found, func(a, b *invariant) int { return len(b.key) - len(a.key) })

	for _, v := range found {
		m.named++

		t := newTemporary(r, "$H"+strconv.Itoa(m.named), v.e.Type())
		place := &ir.Mem{Cell: t, T: t.T}

		rep := &replacer{key: v.key, with: &ir.Load{Mem: place}}
		for _, b := range l.Blocks {
			rep.rewrite(b, 0, len(b.Stmts))
		}

		rep.rewrite(pre, 0, len(pre.Stmts))

		def := ir.NewAssign(pre.End.Source(), place, v.e)
		def.Moved = v.from

		pre.Stmts = append([]ir.Stmt{def}, pre.Stmts...)
	}
}

// preheader puts a new block just before loop l's header, and sends every way into the
// loop through it.
func (m *mover) preheader(g *Graph, l *Loop) *ir.Block {
	r := g.Routine

	var outside []*ir.Block

	for _, p := range g.Preds(l.Header) {
		if !l.Contains(p) {
			outside = append(outside, p)
		}
	}

	m.blocks++
	pre := &ir.Block{Name: "P" + strconv.Itoa(m.blocks), Note: "before the loop"}

	// Its source is what comes before the loop, whose lines are written already, so
	// that the file's lines stay in order. A loop that starts the routine has none.
	var before ast.Stmt
	if len(outside) > 0 {
		before = outside[0].End.Source()
	}

	pre.End = ir.NewJump(before, l.Header)

	for _, p := range outside {
		redirect(p.End, l.Header, pre)
	}

	for i, b := range r.Blocks {
		if b == l.Header {
			r.Blocks = append(r.Blocks[:i:i], append([]*ir.Block{pre}, r.Blocks[i:]...)...)

			break
		}
	}

	return pre
}

// redirect makes terminator t go to to wherever it went to from.
func redirect(t ir.Terminator, from, to *ir.Block) {
	swap := func(b **ir.Block) {
		if *b == from {
			*b = to
		}
	}

	switch t := t.(type) {
	case *ir.Jump:
		swap(&t.To)
	case *ir.Branch:
		swap(&t.True)
		swap(&t.False)
	case *ir.Switch:
		for i := range t.Targets {
			swap(&t.Targets[i])
		}

		swap(&t.Else)
	case *ir.Sign:
		swap(&t.Neg)
		swap(&t.Zero)
		swap(&t.Pos)
	}
}
