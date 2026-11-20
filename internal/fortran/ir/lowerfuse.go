package ir

import (
	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/sema"
)

// Fused nests (docs/planning/PHASE-110 - Fortran 90 compiler usability.md, subtask 8;
// sema/fusion.go has the rule). Under /OPTIMIZE (Options.Fuse) adjacent array
// assignments that sema finds may share one nest of loops are lowered as one:
//
//	A = B + 1.0
//	C = A * 2.0
//
// is
//
//	DO $I1 = 1, N
//	  A($I1) = B($I1) + 1.0
//	  C($I1) = A($I1) * 2.0
//	END DO
//
// and a WHERE construct's assignments as one nest that tests the mask once at each
// element:
//
//	DO $I1 = 1, N
//	  IF (A($I1) > 0) THEN
//	    B($I1) = SQRT(A($I1))
//	  ELSE
//	    B($I1) = 0
//	  END IF
//	END DO
//
// Each part keeps its own statement, so that the code generator writes each one's source
// line before its own code, inside the loop. Not under /CHECK=BOUNDS, whose checks of each
// statement's shapes and subscripts come before that statement's loops: one statement's
// failure would then come before the other's elements were assigned.

// nestPart is one assignment of a fused nest.
type nestPart struct {
	target, value sema.Expr
	order         *sema.ArrayOrder
	from          ast.Stmt
}

// fusing reports whether lowering fuses nests in this routine.
func (l *lowerer) fusing() bool {
	return l.opts.Fuse && !l.unit.Checks.Bounds
}

// fusedGroup is the array assignments at the start of list that share one nest: the
// first and each after it that sema's Fusable lets join them, none labeled. Fewer than
// two is no group.
func (l *lowerer) fusedGroup(list []sema.Stmt) []*sema.Assign {
	if !l.fusing() {
		return nil
	}

	first, ok := list[0].(*sema.Assign)
	if !ok || first.Target.Shape() == nil {
		return nil
	}

	group := []*sema.Assign{first}

	for _, st := range list[1:] {
		a, ok := st.(*sema.Assign)
		if !ok || a.Target.Shape() == nil || a.Source().Base().Label != 0 || !sema.Fusable(group, a) {
			break
		}

		group = append(group, a)
	}

	return group
}

// fusedStmts lowers a group of array assignments as one statement, one nest.
func (l *lowerer) fusedStmts(group []*sema.Assign) {
	l.from = group[0].Source()

	if n := l.from.Base().Label; n != 0 {
		l.startLabel(n)
	}

	parts := make([]nestPart, len(group))
	for i, a := range group {
		parts[i] = nestPart{a.Target, a.Value, a.Order, a.Source()}
	}

	l.fusedNest(parts, nil, 0)

	l.flush()
	l.release()
}

// fusedWhere lowers a WHERE construct whose assignments sema's FusableWhere lets share
// one nest: the mask tested once at each element, the first thenParts parts where it
// holds and the rest where it doesn't.
func (l *lowerer) fusedWhere(s *sema.Where) {
	var parts []nestPart

	thenParts := 0

	for _, arm := range []*sema.Arm{s.Then, s.Else} {
		if arm == nil {
			continue
		}

		for _, st := range arm.Body {
			a := st.(*sema.Assign)
			parts = append(parts, nestPart{a.Target, a.Value, a.Order, a.Source()})
		}

		if arm == s.Then {
			thenParts = len(parts)
		}
	}

	l.from = parts[0].from
	l.fusedNest(parts, s.Mask, thenParts)
	l.flush()
}

// fusedNest lowers parts as one nest of loops of the first part's shape, which every part
// has. With a mask, the first thenParts parts are assigned where it holds and the rest
// where it doesn't.
func (l *lowerer) fusedNest(parts []nestPart, mask sema.Expr, thenParts int) {
	first := parts[0]
	n := l.newNest(len(first.target.Shape()), nil)
	rank := len(n.index)

	for _, p := range parts {
		for _, leaf := range append(sema.Leaves(p.target), sema.Leaves(p.value)...) {
			n.prepare(leaf)
		}
	}

	if mask != nil {
		for _, leaf := range sema.Leaves(mask) {
			n.prepare(leaf)
		}
	}

	n.extents = make([]sema.Expr, rank)
	for d := range rank {
		n.extents[d] = n.extent(d, first.target, first.value)
	}

	// Each part's scalars as its own order says, the values computed before the loops
	// in the parts' order.
	targets := make([]sema.Expr, len(parts))
	values := make([]sema.Expr, len(parts))

	for i, p := range parts {
		l.from = p.from
		n.order = p.order
		targets[i] = sema.Scalarize(p.target, n.element, n.hoist)
		values[i] = sema.Scalarize(p.value, n.element, n.hoist)
	}

	n.order = nil

	var m sema.Expr
	if mask != nil {
		m = sema.Scalarize(mask, n.element, n.hoist)
	}

	l.from = first.from

	assign := func(from, to int) {
		for i := from; i < to; i++ {
			l.from = parts[i].from
			l.assignScalar(targets[i], values[i])
		}
	}

	n.loops(rank-1, func() {
		if m == nil {
			assign(0, len(parts))

			return
		}

		l.from = first.from
		yes, no, next := l.newBlock(), l.newBlock(), l.newBlock()
		l.branch(l.cond(m), yes, no)

		l.begin(yes)
		assign(0, thenParts)
		l.jump(next)

		l.begin(no)
		assign(thenParts, len(parts))
		l.jump(next)

		l.begin(next)
	})

	n.done()
}
