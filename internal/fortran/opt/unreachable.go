package opt

import (
	"slices"

	"github.com/tucats/govax/internal/fortran/ir"
)

// Unreachable: code nothing can get to.
//
//	      DEBUG = .FALSE.
//	      IF (DEBUG) THEN
//	         WRITE (6,*) 'HERE'
//	      END IF
//
// Once Forward has settled that branch, nothing goes to the WRITE's block. A block the
// graph can't reach from any entry never runs, so it goes, and everything it alone
// used goes with it. Instruction passes do the same for code after a jump, but only
// here can it take the temporaries and FTNRTL calls with it.
//
// The graph's entries are what keep a block that looks unreachable: an ENTRY
// statement's, a variable format expression's, or one whose label is ASSIGNed. Removing
// a block can remove the last ASSIGN of another's label, so it goes round until
// nothing else can go.

// Unreachable is the pass.
func Unreachable(r *ir.Routine) {
	for len(r.Blocks) > 0 {
		g := NewGraph(r)
		n := len(r.Blocks)

		r.Blocks = slices.DeleteFunc(r.Blocks, func(b *ir.Block) bool { return !g.Reachable(b) })

		if len(r.Blocks) == n {
			return
		}
	}
}
