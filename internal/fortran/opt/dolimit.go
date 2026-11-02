package opt

import (
	"math"

	"github.com/tucats/govax/internal/fortran/ir"
)

// A DO loop's count dropped for its limit.
//
//	      DO 10 I = 1, N
//	         A(I) = 0.0
//	   10 CONTINUE
//
// The language defines this with an iteration count (section 5.3.1 of the manual), and
// lowering writes what it says, so the loop keeps two things in step: I, and how many
// trips are left.
//
//	      MOVL    N,R7                     ; the count, N - 1 + 1, before the loop
//	T:    CLRF    A-4[R6]
//	      INCL    R6
//	      SOBGTR  R7,T
//
// But the two say the same thing: I passes N on just the trip the count reaches zero. So I
// alone can end the loop, which is what AOBLEQ does, one instruction for the two:
//
//	      MOVL    N,R7                     ; before the loop
//	T:    CLRF    A-4[R6]
//	      AOBLEQ  R7,R6,T
//
// The limit still goes in a cell of its own, $L1, where the count was: section 5.3.1 says the
// program can change N in the loop without affecting how many trips it takes. A constant limit
// needs no cell, and no register.
//
// The pass only changes three statements and two branches: what's stored before the loop, the
// test for a loop of no trips, and the test at its end. Taking one from the count goes. From
// these the code generator writes a compare and a branch, and Branches (branches.go) makes
// AOBLEQ or ACBL of them, or a SOBGTR on I itself for DO 10 I = N, 1, -1, whose limit is
// what SOBGTR tests.
//
// # When it's safe
//
// The count's trips don't depend on I, and the limit's do, so:
//
//   - Nothing else in the loop may write I. The manual forbids it, but a compiler can't always
//     see it, and a loop that did would go on running a different number of trips. So any
//     other statement in the loop that may write I keeps the count, as does a call that may
//     change it (Decision 7). DO 10 I = 1, N with CALL F(I) in it keeps its count, then: a
//     call costs far more than the instruction this saves.
//   - Nothing VOLATILE, whose every read counts.
//   - The limit plus the step must be a number. I goes one step past the limit, and at the
//     largest integer that wraps round to a negative number, less than the limit again, and
//     the loop would never end. A constant limit that close keeps its count. A limit known only
//     when the program runs can't be looked at, and this pass goes ahead: section 5.3.1.1 has the
//     step "algebraically added" to the variable at the loop's end, and the sum in that case is
//     no value a variable can hold. So a program whose terminal value is the largest integer
//     goes on forever under /OPTIMIZE and ends under /NOOPTIMIZE. That is a choice of ours, as
//     with dummy arguments that alias (answer 1 of the phase doc).
//
// A REAL or short integer variable, or a step that isn't a constant whose sign is known, keeps
// its count: the VAX has no such instruction for those, and a step's sign says which way the
// test goes.
//
// This is the first pass, since it finds lowering's statements as they were written. Forward
// then knows what the limit is, if that can be known.

// DoLimits is the pass.
func DoLimits(r *ir.Routine) {
	counted := r.Counted
	r.Counted = nil

	if len(counted) == 0 {
		return
	}

	g := NewGraph(r)
	s := NewStorage(r)

	loops := map[*ir.Block]*Loop{}
	for _, l := range g.Loops {
		loops[l.Header] = l
	}

	for _, c := range counted {
		if l := loops[c.Body]; l != nil && canUseLimit(s, l, c) {
			useLimit(r, g, c)
		}
	}
}

// canUseLimit reports whether I alone can end loop l, by the rules above.
func canUseLimit(s *Storage, l *Loop, c *ir.CountedLoop) bool {
	v := c.Var
	if v.Index != nil || v.Ptr != nil || c.Step == 0 {
		return false
	}

	self := s.Place(v)
	if s.Volatile(self) {
		return false
	}

	if k, ok := c.Limit.(*ir.Const); ok {
		next := int64(k.Int()) + int64(c.Step)
		if next > math.MaxInt32 || next < math.MinInt32 {
			return false
		}
	} else if c.Test == nil {
		return false
	}

	for _, b := range l.Blocks {
		for _, st := range b.Stmts {
			if st == ir.Stmt(c.Advance) {
				continue
			}

			e := s.StmtEffects(st)
			if overlapsAny(self, e.Writes) || (e.Call && s.ChangedByCall(self)) {
				return false
			}
		}
	}

	return true
}

// useLimit rewrites what lowering wrote for one loop.
func useLimit(r *ir.Routine, g *Graph, c *ir.CountedLoop) {
	var limit ir.Expr

	if k, ok := c.Limit.(*ir.Const); ok {
		limit = k

		removeStmt(g, c.Set)
	} else {
		cell := c.Count.Cell
		cell.Kind = ir.LimitCell
		cell.Name = "$L" + cell.Name[len("$C"):]

		c.Set.Src = c.Limit
		limit = &ir.Load{Mem: c.Count}
	}

	removeStmt(g, c.Less)

	at := &ir.Load{Mem: c.Var}

	// Past the limit, by the step's sign.
	past, within := ir.Gt, ir.Le
	if c.Step < 0 {
		past, within = ir.Lt, ir.Ge
	}

	if c.Test != nil {
		c.Test.Cond = &ir.Cmp{Op: past, X: at, Y: limit}
	}

	c.Close.Cond = closing(c.Step, within, at, limit)
}

// closing is the test that goes round again. Stepping down to 1 or 0 is a test of the sign
// after the step, which SOBGTR and SOBGEQ are.
func closing(step int32, within ir.Operator, at, limit ir.Expr) ir.Expr {
	if k, ok := limit.(*ir.Const); ok && step == -1 && k.Int() == 1 {
		return &ir.Cmp{Op: ir.Gt, X: at, Y: ir.LongConst(0)}
	}

	return &ir.Cmp{Op: within, X: at, Y: limit}
}

// removeStmt takes statement st out of the block it's in.
func removeStmt(g *Graph, st ir.Stmt) {
	for _, b := range g.Routine.Blocks {
		for i, other := range b.Stmts {
			if other == st {
				b.Stmts = append(b.Stmts[:i:i], b.Stmts[i+1:]...)

				return
			}
		}
	}
}
