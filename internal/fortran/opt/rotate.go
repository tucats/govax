package opt

import (
	"slices"

	"github.com/tucats/govax/internal/fortran/ir"
)

// Loop rotation: the test at the bottom.
//
//	   20 IF (K .GT. M) GO TO 30
//	      F(K) = .FALSE.
//	      K = K + P
//	      GO TO 20
//	   30 ...
//
// A DO WHILE or a GO TO loop like this one tests at its top, and every trip runs two
// branches, one not taken and one back:
//
//	L20:    CMPL    R6,#20000
//	        BGTR    L30
//	        CLRB    F-1[R6]
//	        ADDL2   R7,R6
//	        BRB     L20
//	L30:
//
// With the test moved below the body, there is one branch on each trip, and the first way in
// jumps to the test:
//
//	        BRB     L20
//	$1:     CLRB    F-1[R6]
//	        ADDL2   R7,R6
//	L20:    CMPL    R6,#20000
//	        BLEQ    $1
//	L30:
//
// No statement changes, and none runs at another time: the header runs when it did, the
// body when it did. Only where the blocks lie in the file changes, which the terminators
// don't depend on (they name both ways). So this is always correct, and needs no analysis of
// what the test reads. The price is the one jump on the way in, which a loop of even one trip
// has earned back.
//
// A loop gets it if its header can leave the loop, and a latch goes back to it by a jump
// (a latch that branches is a test at the bottom already). The header goes just after the
// latch: nothing fell through into what comes next, as the latch ended in a jump, and the
// header's own way out now usually falls through to it.
//
// A loop the routine starts with isn't moved, since the first block is the entry. It runs
// after the register allocator, whose write-backs may be the way out: a block of the
// allocator's on that edge comes along, so the way out still falls through.
//
// A source line stays in order, so the test's line comes before the body's code, and its first
// instruction says where it's from (Decision 12 of docs/planning/PHASE-84 - FORTRAN optimizer.md).

// Rotate is the pass.
func Rotate(r *ir.Routine) {
	if len(r.Blocks) == 0 {
		return
	}

	g := NewGraph(r)
	expressions := ir.ExpressionBlocks(r)

	for _, l := range g.Loops {
		if l.Header == r.Blocks[0] || expressions[l.Header] {
			continue
		}

		exit := headerExit(l)
		if exit == nil {
			continue
		}

		if latch := lastJumpingLatch(r, l); latch != nil {
			rotate(r, g, l.Header, latch, exit)
		}
	}
}

// headerExit is where loop l's header goes if it leaves the loop by a branch, the other way
// going on in the loop; nil otherwise.
func headerExit(l *Loop) *ir.Block {
	b, ok := l.Header.End.(*ir.Branch)
	if !ok {
		return nil
	}

	switch {
	case l.Contains(b.True) && !l.Contains(b.False):
		return b.False
	case l.Contains(b.False) && !l.Contains(b.True):
		return b.True
	}

	return nil
}

// lastJumpingLatch is loop l's last latch in layout that goes back to the header by a jump,
// nil if there's none, or if the header follows it already.
func lastJumpingLatch(r *ir.Routine, l *Loop) *ir.Block {
	var last *ir.Block

	at := -1

	for _, latch := range l.Latches {
		if _, jumps := latch.End.(*ir.Jump); !jumps {
			continue
		}

		if i := slices.Index(r.Blocks, latch); i > at {
			at, last = i, latch
		}
	}

	if last != nil && at+1 < len(r.Blocks) && r.Blocks[at+1] == l.Header {
		return nil
	}

	return last
}

// rotate lays out header just after latch, and if the way out is a block of the allocator's
// with no other way in, that just after it.
func rotate(r *ir.Routine, g *Graph, header, latch, exit *ir.Block) {
	moved := []*ir.Block{header}

	if exit.Note == leavingLoop && len(g.Preds(exit)) == 1 {
		moved = append(moved, exit)
	}

	rest := slices.DeleteFunc(slices.Clone(r.Blocks), func(b *ir.Block) bool { return slices.Contains(moved, b) })
	at := slices.Index(rest, latch)

	r.Blocks = slices.Insert(rest, at+1, moved...)
	header.Rotated = true
}
