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

// where lowers a WHERE construct or statement.
func (l *lowerer) where(s *sema.Where) {
	if s.Mask == nil {
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
