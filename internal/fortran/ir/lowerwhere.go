package ir

import "github.com/tucats/govax/internal/fortran/sema"

// WHERE (docs/planning/PHASE-102 - Fortran 90 array expressions.md, finding
// 8; sema/where.go): each assignment of the construct is an array assignment
// whose loops assign an element only where the mask holds, ELSEWHERE's only
// where it doesn't. So
//
//	WHERE (A > 0.0) A = SQRT(A)
//
// is one nest,
//
//	DO $I1 = 1, 10
//	  IF (A($I1) > 0.0) A($I1) = SQRT(A($I1))
//	END DO
//
// Where sema found that an assignment of the construct may change what the
// mask reads before a later one reads it (Where.MaskTemp), the mask is
// computed first, once, into a LOGICAL array temporary of its shape, and each
// nest tests the temporary's element instead. The temporaries the construct
// uses are given back only at its end, so the mask's lasts through every
// assignment.
//
// A Fortran 95 construct (sema's Where.General: a WHERE nested in a block, or
// a masked ELSEWHERE) is lowered by generalWhere: each mask computed into its
// temporary when its statement is reached, only where its control mask holds,
// and each statement's loops testing the chain of temporaries that is its
// control mask, outer first. So
//
//	WHERE (A > 0)
//	   WHERE (B > 1 / A) B = 0
//	ELSEWHERE (A < 0)
//	   B = 1
//	END WHERE
//
// is, with $W1 to $W3 LOGICAL temporaries of the mask's shape,
//
//	$W1 = A > 0
//	WHERE ($W1) $W2 = B > 1 / A
//	WHERE ($W1) WHERE ($W2) B = 0
//	WHERE (.NOT. $W1) $W3 = A < 0
//	WHERE (.NOT. $W1) WHERE ($W3) B = 1
//
// each line one nest, the 1 / A computed only where A > 0.

// where lowers a WHERE construct or statement.
func (l *lowerer) where(s *sema.Where) {
	if s.Mask == nil {
		return
	}

	if s.General() {
		l.generalWhere(s, nil)
		l.endWhere(s)

		return
	}

	// Under /OPTIMIZE, one nest for the whole construct where sema allows it
	// (lowerfuse.go).
	if l.fusing() && sema.FusableWhere(s) {
		l.fusedWhere(s)
		l.endWhere(s)

		return
	}

	mask := s.Mask

	if s.MaskTemp != nil {
		temp := l.shapeTemp(mask.Type(), s.MaskTemp, mask.Shape(), nil)
		l.arrayNest(temp, mask, nil, nil)
		l.flush()

		mask = temp
	}

	for _, arm := range []*sema.Arm{s.Then, s.Else} {
		if arm == nil {
			continue
		}

		for _, st := range arm.Body {
			l.from = st.Source()

			if n := l.from.Base().Label; n != 0 {
				// No jump reaches it (sema's INTOBLOCK), but an ASSIGN may
				// name it.
				l.startLabel(n)
			}

			m := &whereMask{expr: mask, negate: arm == s.Else}

			switch st := st.(type) {
			case *sema.Assign:
				l.maskedAssign(st, m)
			case *sema.Call:
				// An elemental defined assignment (sema's elemental.go).
				l.maskedCall(st, m)
			}

			l.flush()
		}
	}

	l.endWhere(s)
}

// generalWhere lowers the Fortran 95 construct s under the control mask
// control (nil for none: s isn't nested).
func (l *lowerer) generalWhere(s *sema.Where, control *whereMask) {
	mask := l.maskTemp(s.Mask, s.MaskTemp, control)
	l.whereArm(s.Then, &whereMask{expr: mask, outer: control})

	// Each later block's control mask: every mask before it false.
	pending := &whereMask{expr: mask, negate: true, outer: control}

	for _, m := range s.Masked {
		l.from = m.AST

		mask := l.maskTemp(m.Mask, m.MaskTemp, pending)
		l.whereArm(m.Arm, &whereMask{expr: mask, outer: pending})

		pending = &whereMask{expr: mask, negate: true, outer: pending}
	}

	if s.Else != nil {
		l.whereArm(s.Else, pending)
	}
}

// maskTemp computes the mask m into a LOGICAL temporary of the extents
// sizes where control holds, and is the temporary. Its other elements are
// left as they were: nothing reads them, every test of it coming after its
// control's.
func (l *lowerer) maskTemp(m sema.Expr, sizes []int, control *whereMask) sema.Expr {
	temp := l.shapeTemp(m.Type(), sizes, m.Shape(), nil)
	l.arrayNest(temp, m, nil, control)
	l.flush()

	return temp
}

// whereArm lowers the statements of a block of a Fortran 95 construct under
// its control mask: assignments, elemental defined assignments, and nested
// WHEREs.
func (l *lowerer) whereArm(arm *sema.Arm, control *whereMask) {
	for _, st := range arm.Body {
		l.from = st.Source()

		if n := l.from.Base().Label; n != 0 {
			// No jump reaches it (sema's INTOBLOCK), but an ASSIGN may name
			// it.
			l.startLabel(n)
		}

		switch st := st.(type) {
		case *sema.Assign:
			l.maskedAssign(st, control)
		case *sema.Call:
			l.maskedCall(st, control)
		case *sema.Where:
			l.generalWhere(st, control)
			l.endWhere(st)
		}

		l.flush()
	}
}

// endWhere ends a WHERE construct at its END WHERE, whose label a jump may name.
func (l *lowerer) endWhere(s *sema.Where) {
	if s.EndWhere != nil {
		l.from = s.EndWhere

		if n := s.EndWhere.Base().Label; n != 0 {
			l.startLabel(n)
		}
	}
}

// maskedBy lowers body where the LOGICAL scalar m holds, or with negate where
// it doesn't.
func (l *lowerer) maskedBy(m sema.Expr, negate bool, body func()) {
	yes, next := l.newBlock(), l.newBlock()

	if negate {
		l.branch(l.cond(m), next, yes)
	} else {
		l.branch(l.cond(m), yes, next)
	}

	l.begin(yes)
	body()
	l.jump(next)
	l.begin(next)
}
