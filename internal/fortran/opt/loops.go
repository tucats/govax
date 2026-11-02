package opt

import "github.com/tucats/govax/internal/fortran/ir"

// Natural loops. An edge from N to H is a back edge if H dominates N: control has
// to pass H to get to N, and N goes back to H. The natural loop of that edge is H
// and every block that can reach N without going through H. Since H dominates N,
// each of those blocks is dominated by H too, so H is the loop's only way in. That
// one way in is what makes a loop safe to work on: code put just before H runs once
// before the loop, on every path into it.
//
// The definition doesn't care what the source said. In
//
//	      DO 10 I = 1, N
//	         IF (A(I) .LT. 0.0) GO TO 100
//	   50    B(I) = A(I)
//	   10 CONTINUE
//	      STOP
//	  100 A(I) = 0.0
//	      GO TO 50
//
// the block at label 100 is far from the DO in the text, but it can reach the
// loop's end without going through the top of the loop, so it is in the loop. This
// is the extended range of section 5.3.1.4 of the language manual. If some other path
// could also reach label 100 from outside the loop, the top wouldn't dominate it
// and it would be a second way in: the back edge wouldn't be one, and the graph
// lists the edge as Irreducible rather than pretend.
//
// Two back edges to one header are one loop, as when two GO TOs go back to the top
// of a GO TO loop. Any two loops with different headers are either nested or have
// no block in common.

// Loop is a natural loop.
type Loop struct {
	// Header is the loop's only way in. Every block of the loop is dominated by it.
	Header *ir.Block

	// Blocks are the loop's blocks in the graph's Order, so the header is first.
	Blocks []*ir.Block

	// Latches are the blocks with an edge back to the header.
	Latches []*ir.Block

	// Exits are the edges from a block in the loop to one outside.
	Exits []Edge

	// Parent is the smallest loop this one is inside; nil for an outermost loop.
	Parent *Loop

	// Children are the loops immediately inside this one.
	Children []*Loop

	// Depth is how many loops the loop is in, itself included: 1 for an outermost
	// loop.
	Depth int

	body map[*ir.Block]bool
}

// Contains reports whether b is in the loop, or in a loop inside it.
func (l *Loop) Contains(b *ir.Block) bool { return l.body[b] }

// LoopOf is the smallest loop b is in, or nil.
func (g *Graph) LoopOf(b *ir.Block) *Loop { return g.innermost[b] }

// findLoops sorts the retreating edges into back edges and the rest, and makes
// the loops of the back edges.
func (g *Graph) findLoops(retreating []Edge) {
	byHeader := map[*ir.Block]*Loop{}

	for _, e := range retreating {
		if !g.Dominates(e.To, e.From) {
			g.Irreducible = append(g.Irreducible, e)

			continue
		}

		l := byHeader[e.To]
		if l == nil {
			l = &Loop{Header: e.To, body: map[*ir.Block]bool{e.To: true}}
			byHeader[e.To] = l
		}

		l.Latches = append(l.Latches, e.From)
		g.addToLoop(l, e.From)
	}

	// In Order an outer loop's header comes before the headers of the loops inside
	// it, since it dominates them.
	for _, b := range g.Order {
		if l := byHeader[b]; l != nil {
			g.Loops = append(g.Loops, l)
		}
	}

	g.innermost = map[*ir.Block]*Loop{}

	for _, l := range g.Loops {
		// The loop's parent is the innermost of the loops before it holding its
		// header: as they nest, that's the smallest one holding all of it.
		if p := g.innermost[l.Header]; p != nil {
			l.Parent = p
			l.Depth = p.Depth
			p.Children = append(p.Children, l)
		}

		l.Depth++

		for _, b := range g.Order {
			if l.body[b] {
				l.Blocks = append(l.Blocks, b)
				g.innermost[b] = l

				for _, s := range g.succs[b] {
					if !l.body[s] {
						l.Exits = append(l.Exits, Edge{b, s})
					}
				}
			}
		}
	}
}

// addToLoop adds b and everything that reaches b to loop l, walking predecessors
// back until they stop at blocks already in it, the header first among them.
func (g *Graph) addToLoop(l *Loop, b *ir.Block) {
	if l.body[b] {
		return
	}

	l.body[b] = true

	for _, p := range g.preds[b] {
		g.addToLoop(l, p)
	}
}
