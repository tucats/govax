package sema

import (
	"github.com/tucats/govax/internal/fortran/ast"
)

// Alternate returns (sections 5.9 and 6.1.1.5). A SUBROUTINE or ENTRY
// statement's dummy argument list has a * for each, and a CALL's list has *s
// or &s in the same places, s the label of a statement in the calling unit.
// RETURN i goes back to the i-th of those labels; any other i, and a plain
// RETURN, go back to the statement after the CALL.
//
// The labels mean nothing to the routine called, so the argument list that is
// built has no longword for them (the author's answer 2 in docs/PHASE-79):
// CALL CHECK(A,B,*10,*20,C) is CALLS #3, and C is CHECK's third longword. The
// routine leaves i in R0, and the caller chooses its label.

// altPlace notes that a dummy argument list has a * at its place i. Only a
// subroutine's lists may: in a function it is the message named by ident.
func (a *analyzer) altPlace(p *ast.Name, i int, places *[]int, ident string) {
	if a.unit.Kind == ast.FunctionUnit {
		a.report(ident, p.Pos)

		return
	}

	*places = append(*places, i)
}

// HasAltReturns reports whether any of the unit's dummy argument lists has a *.
func (u *Unit) HasAltReturns() bool {
	if len(u.AltPlaces) > 0 {
		return true
	}

	for _, e := range u.Entries {
		if len(e.AltPlaces) > 0 {
			return true
		}
	}

	return false
}

// altReturns divides a CALL's list into its actual arguments and its alternate
// return specifiers: their labels, and their places in the list. A list with
// no specifiers comes back as it is, nil if there was none.
func altReturns(list []ast.Expr) (args []ast.Expr, targets []ast.LabelRef, places []int) {
	for i, e := range list {
		if alt, ok := e.(*ast.AltReturn); ok {
			targets = append(targets, alt.Target)
			places = append(places, i)
		}
	}

	if targets == nil {
		return list, nil, nil
	}

	args = make([]ast.Expr, 0, len(list)-len(targets))

	for _, e := range list {
		if _, ok := e.(*ast.AltReturn); !ok {
			args = append(args, e)
		}
	}

	return args, targets, places
}

// returnStmt analyzes RETURN [i] (section 5.9). i is any arithmetic
// expression, converted to an integer as an assignment would, and only a
// subroutine may have one. How many alternate returns there are is the
// caller's to know, so it isn't checked here.
func (a *analyzer) returnStmt(s *ast.ReturnStmt) Stmt {
	r := &Return{stmtBase: stmtBase{s}}

	switch {
	case a.unit.Kind == ast.MainProgram:
		a.report("MAINRETURN", s.Pos)
	case s.Alt == nil:
	case a.unit.Kind == ast.FunctionUnit:
		a.report("FUNCRETURN", s.Alt.Position())
	default:
		i := a.arithmetic(a.value(s.Alt))
		if i.Type().Kind == Invalid {
			return nil
		}

		r.Alt = a.convert(i, IntegerType)
	}

	return r
}
