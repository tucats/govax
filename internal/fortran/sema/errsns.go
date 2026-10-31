package sema

import "github.com/tucats/govax/internal/fortran/ast"

// The system subroutine ERRSNS (the 1988 VAX FORTRAN Language Reference
// Manual, D.4.3) returns what FORTRAN's run-time library saved of the last
// error: CALL ERRSNS(fnum, rmssts, rmsstv, iunit, condval), each argument an
// INTEGER variable or array element, or null, and all of them INTEGER*2 or
// all INTEGER*4 (D.4). The library's routine gets each argument's address
// and nothing of its length, so the compiler chooses between two routines
// by the arguments' type: FTN$ERRSNS stores longwords, FTN$ERRSNS_W words
// (docs/PHASE-80 - FORTRAN formatted IO.md, Decision 13). A call with no
// argument that isn't null calls FTN$ERRSNS, which only clears the saved
// error.

// errsnsRoutines are the two routines, by the arguments' length.
var errsnsRoutines = map[int]string{2: "FTN$ERRSNS_W", 4: "FTN$ERRSNS"}

// errsns analyzes CALL ERRSNS(list): a call of FTNRTL's routine for the
// arguments' length, through a symbol of its own that no unit's table holds.
func (a *analyzer) errsns(s *ast.CallStmt, list []ast.Expr) Stmt {
	if len(list) > 5 {
		a.report("ERRSNSCOUNT", s.NamePos, len(list))

		return nil
	}

	args := a.args(list)
	size, ok := 0, true

	for _, arg := range args {
		if arg.Expr == nil {
			continue // a null argument: nothing stored there
		}

		t := arg.Expr.Type()

		switch arg.Expr.(type) {
		case *Var, *Elem:
		default:
			t = Type{}
		}

		if arg.Copy || arg.Mode != ByDefault || t.Kind != Integer || (t.Size != 2 && t.Size != 4) {
			a.report("ERRSNSARG", arg.Expr.Position())

			ok = false

			continue
		}

		if size != 0 && t.Size != size {
			a.report("ERRSNSLEN", arg.Expr.Position())

			ok = false
		}

		if size == 0 {
			size = t.Size
		}
	}

	if !ok {
		return nil
	}

	if size == 0 {
		size = 4
	}

	sub := &Symbol{Name: errsnsRoutines[size], Class: Subroutine}

	return &Call{stmtBase: stmtBase{s}, Sub: sub, Args: args}
}
