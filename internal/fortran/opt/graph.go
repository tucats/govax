package opt

import (
	"slices"

	"github.com/tucats/govax/internal/fortran/ir"
)

// The control flow graph. A routine's blocks are its nodes, and an edge goes from
// a block to each place its terminator can go. Every other analysis starts here,
// never from the DO statements in the source, since a loop needn't be one:
//
//	   10 IF (A(I) .GE. X) GO TO 20
//	      I = I + 1
//	      GO TO 10
//	   20 ...
//
// is as much a loop as any DO, and a DO isn't always the simple shape it looks: the
// extended range of section 5.3.1.4 leaves the loop by GO TO and comes back (Decision 1
// of docs/PHASE-84 - FORTRAN optimizer.md).
//
// # Every way in
//
// A routine has more than one way in, and an analysis that forgot one would
// reason about code from paths that aren't all its paths:
//
//	      SUBROUTINE S(N)
//	      K = 0
//	      ENTRY T(N)
//	      K = K + N
//
// A call of T arrives at the second assignment with no first assignment run. So
// the graph's Entries are the first block, each ENTRY statement's block, and the
// first block of each variable format expression, which FTNRTL calls in the middle
// of an I/O statement. They all hang from one imaginary root, which precedes
// every entry.
//
// An assigned GO TO goes wherever the variable says, and section 5.6.3 says of its
// list that it "does not affect statement execution". So
//
//	      ASSIGN 30 TO K
//	      GO TO K, (20)
//
// goes to 30. The edges of an IndirectJump are the blocks its list names and every
// block whose label the unit ASSIGNs, the only other places the variable can hold,
// since an ASSIGN and its GO TO must be in one unit. A block whose label is ASSIGNed
// where no jump could reach it counts as one more entry: its address is in use, and
// its code must stay.
//
// # Dominators
//
// Block A dominates block B if every path from the root to B goes through A. It is
// what lets a pass say "this has always run by the time we get here": in
//
//	      X = 1
//	      IF (L) Y = 2
//	      Z = 3
//
// the block with X = 1 dominates the one with Z = 3, and the one with Y = 2 doesn't.
// Each block but the entries has an immediate dominator, the closest of those that
// dominate it, so they make a tree. IDom is that tree, found by the simple iteration
// of Cooper, Harvey, and Kennedy's "A Simple, Fast Dominance Algorithm": take the
// blocks in reverse postorder (a block before its successors, loops aside), and set
// each one's immediate dominator to the nearest common ancestor of what's known for
// its predecessors, until nothing changes.

// Edge is one way from a block to another.
type Edge struct {
	From, To *ir.Block
}

// Graph is a routine's control flow graph, with its dominators and loops. It is a
// picture of the routine when it was made: a pass that changes the blocks makes a
// new one.
type Graph struct {
	// Routine is the routine the graph is of.
	Routine *ir.Routine

	// Entries are the ways into the routine, the first block first.
	Entries []*ir.Block

	// Order is every block reachable from an entry, in reverse postorder: unless
	// an edge closes a cycle, a block comes before the blocks it goes to, which is
	// the order a forward analysis wants. A block not here can't run.
	Order []*ir.Block

	// Loops are the routine's natural loops, an outer loop before those inside it
	// (loops.go).
	Loops []*Loop

	// Irreducible are the edges that close a cycle that isn't a natural loop, one
	// with a second way in: an ENTRY statement in a loop's body, say. The blocks of
	// such a cycle are in no loop for it, and no pass may move code out of it.
	Irreducible []Edge

	succs, preds map[*ir.Block][]*ir.Block
	number       map[*ir.Block]int
	idom         map[*ir.Block]*ir.Block
	innermost    map[*ir.Block]*Loop
}

// NewGraph makes routine r's graph. A block data subprogram's has no blocks.
func NewGraph(r *ir.Routine) *Graph {
	g := &Graph{
		Routine: r,
		succs:   map[*ir.Block][]*ir.Block{},
		preds:   map[*ir.Block][]*ir.Block{},
		number:  map[*ir.Block]int{},
		idom:    map[*ir.Block]*ir.Block{},
	}

	if len(r.Blocks) == 0 {
		return g
	}

	assigned := g.assignedBlocks()

	for _, b := range r.Blocks {
		g.succs[b] = Successors(b.End)

		if _, ok := b.End.(*ir.IndirectJump); ok {
			g.succs[b] = union(g.succs[b], assigned)
		}
	}

	g.findEntries(assigned)
	retreating := g.walk()

	for _, b := range g.Order {
		for _, s := range g.succs[b] {
			g.preds[s] = append(g.preds[s], b)
		}
	}

	g.findDominators()
	g.findLoops(retreating)

	return g
}

// assignedBlocks are the blocks whose labels an ASSIGN in the routine takes, each
// once.
func (g *Graph) assignedBlocks() []*ir.Block {
	storage := &Storage{r: g.Routine}

	var list []*ir.Block

	for _, b := range g.Routine.Blocks {
		list = union(list, storage.BlockEffects(b).Labels)
	}

	return list
}

// union is list, then what's in more that isn't in it.
func union(list, more []*ir.Block) []*ir.Block {
	for _, b := range more {
		if !slices.Contains(list, b) {
			list = append(list, b)
		}
	}

	return list
}

// findEntries lists the routine's entries: the first block, the ENTRY statements',
// the variable format expressions', and any assigned block those don't reach.
func (g *Graph) findEntries(assigned []*ir.Block) {
	r := g.Routine
	g.Entries = union(g.Entries, r.Blocks[:1])

	for _, e := range r.Entries {
		if e.First != nil {
			g.Entries = union(g.Entries, []*ir.Block{e.First})
		}
	}

	for _, v := range r.VFEs {
		g.Entries = union(g.Entries, v.Cases)
	}

	for _, b := range assigned {
		if !g.reachable()[b] {
			g.Entries = append(g.Entries, b)
		}
	}
}

// reachable is the set of blocks the entries so far reach.
func (g *Graph) reachable() map[*ir.Block]bool {
	reached := map[*ir.Block]bool{}

	var visit func(*ir.Block)

	visit = func(b *ir.Block) {
		if reached[b] {
			return
		}

		reached[b] = true

		for _, s := range g.succs[b] {
			visit(s)
		}
	}

	for _, b := range g.Entries {
		visit(b)
	}

	return reached
}

// walk visits the graph depth first from the entries, setting Order and each
// block's number, and returns the retreating edges, each of which goes back to a
// block still being visited: every cycle has at least one.
func (g *Graph) walk() []Edge {
	var (
		post       []*ir.Block
		retreating []Edge
	)

	const (
		visiting = 1
		finished = 2
	)

	state := map[*ir.Block]int{}

	var visit func(*ir.Block)

	visit = func(b *ir.Block) {
		state[b] = visiting

		for _, s := range g.succs[b] {
			switch state[s] {
			case 0:
				visit(s)
			case visiting:
				retreating = append(retreating, Edge{b, s})
			}
		}

		state[b] = finished

		post = append(post, b)
	}

	for _, b := range g.Entries {
		if state[b] == 0 {
			visit(b)
		}
	}

	g.Order = make([]*ir.Block, len(post))

	for i, b := range post {
		place := len(post) - 1 - i
		g.Order[place] = b
		g.number[b] = place
	}

	return retreating
}

// findDominators sets each block's immediate dominator (see the top of this file).
// A block is known by its place in Order plus 1, 0 being the imaginary root, so
// that walking up the tree always goes to a smaller number.
func (g *Graph) findDominators() {
	const root = 0

	entry := map[*ir.Block]bool{}
	for _, b := range g.Entries {
		entry[b] = true
	}

	// doms[n] is what's known of block n's immediate dominator, -1 for nothing yet.
	doms := make([]int, len(g.Order)+1)
	for i := range doms {
		doms[i] = -1
	}

	doms[root] = root

	common := func(a, b int) int {
		for a != b {
			for a > b {
				a = doms[a]
			}

			for b > a {
				b = doms[b]
			}
		}

		return a
	}

	for changed := true; changed; {
		changed = false

		for i, b := range g.Order {
			n, nearest := i+1, -1

			if entry[b] {
				nearest = root
			}

			for _, p := range g.preds[b] {
				m := g.number[p] + 1

				switch {
				case doms[m] == -1:
					// Not reached yet on this round: it says nothing so far.
				case nearest == -1:
					nearest = m
				default:
					nearest = common(nearest, m)
				}
			}

			if nearest != doms[n] {
				doms[n] = nearest
				changed = true
			}
		}
	}

	for i, b := range g.Order {
		if d := doms[i+1]; d != root {
			g.idom[b] = g.Order[d-1]
		}
	}
}

// Successors are the blocks a terminator names, each once, in the order it names
// them. An assigned GO TO can also go elsewhere, which only the Graph knows.
func Successors(t ir.Terminator) []*ir.Block {
	var list []*ir.Block

	switch t := t.(type) {
	case *ir.Jump:
		list = []*ir.Block{t.To}
	case *ir.Branch:
		list = []*ir.Block{t.True, t.False}
	case *ir.Switch:
		list = append(append(list, t.Targets...), t.Else)
	case *ir.Sign:
		list = []*ir.Block{t.Neg, t.Zero, t.Pos}
	case *ir.IndirectJump:
		list = t.Targets
	}

	return union(nil, list)
}

// Succs are the blocks b can go to.
func (g *Graph) Succs(b *ir.Block) []*ir.Block { return g.succs[b] }

// Preds are the reachable blocks that can go to b. An entry may have some too, and
// is also reached by a call.
func (g *Graph) Preds(b *ir.Block) []*ir.Block { return g.preds[b] }

// Reachable reports whether b can run.
func (g *Graph) Reachable(b *ir.Block) bool {
	_, ok := g.number[b]

	return ok
}

// IsEntry reports whether b is one of the ways into the routine.
func (g *Graph) IsEntry(b *ir.Block) bool {
	return slices.Contains(g.Entries, b)
}

// IDom is b's immediate dominator, or nil for a block only the imaginary root
// dominates, as an entry is, or one that can't run.
func (g *Graph) IDom(b *ir.Block) *ir.Block { return g.idom[b] }

// Dominates reports whether every path from an entry to b goes through a. A block
// dominates itself; a block that can't run neither dominates nor is dominated.
func (g *Graph) Dominates(a, b *ir.Block) bool {
	if !g.Reachable(a) || !g.Reachable(b) {
		return false
	}

	for ; b != nil; b = g.idom[b] {
		if a == b {
			return true
		}
	}

	return false
}
