package sema

import (
	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/lex"
)

// Statement order (section 1.1.2, Figure 1-1). After a unit's header its
// statements come in groups, each group's before the next's:
//
//	1  IMPLICIT NONE
//	2  IMPLICIT                          PARAMETER (in 2 or 3)
//	3  the other specification statements
//	4  statement function definitions    DATA (in 3, 4, or 5)
//	5  executable statements
//
// with END last; FORMAT, ENTRY, NAMELIST, and the general directives may
// go anywhere among them. DATA may come anywhere after IMPLICIT (VAX
// FORTRAN lets it sit among the specification statements), and PARAMETER
// anywhere from IMPLICIT to the last specification statement.
//
// So each statement has a group it puts the unit in (at) and the last
// group it may still come in (latest). The unit's group only rises;
// a statement that comes when the unit is past its latest group is out
// of order, reported against the first statement that took the unit past
// it: LATEDECL if that was an executable statement, ORDER otherwise.

// group is one of Figure 1-1's groups.
type group int

const (
	anywhere group = iota
	implicitNoneGroup
	implicitGroup
	specGroup
	stmtFuncGroup
	execGroup
)

// placement is where a statement may go: the group it puts the unit in,
// and the last group it may come in.
type placement struct {
	at, latest group
}

// placementOf is where the statement s may go; stmtFunc says whether
// it is a statement function's definition (which the declarations walk
// decides).
func placementOf(s ast.Stmt, stmtFunc bool) placement {
	switch s := s.(type) {
	case *ast.ImplicitStmt:
		if s.None {
			return placement{implicitNoneGroup, implicitNoneGroup}
		}

		return placement{implicitGroup, implicitGroup}

	case *ast.ParameterStmt:
		return placement{implicitGroup, specGroup}

	case *ast.DataStmt:
		return placement{specGroup, execGroup}

	case *ast.TypeStmt, *ast.DimensionStmt, *ast.CommonStmt, *ast.EquivalenceStmt,
		*ast.AttrStmt, *ast.RecordStmt, *ast.StructureStmt, *ast.DictionaryStmt:
		return placement{specGroup, specGroup}

	case *ast.AssignStmt:
		if stmtFunc {
			return placement{stmtFuncGroup, stmtFuncGroup}
		}

	case *ast.BadStmt, *ast.EndStmt:
		// A statement the parser rejected is reported already; END
		// ends the unit.
		return placement{}
	}

	if ast.Executable(s) && s.Base().Kind != lex.Unknown {
		return placement{execGroup, execGroup}
	}

	// FORMAT, ENTRY, NAMELIST, the directives, OPTIONS (which the parser
	// reports where it doesn't belong).
	return placement{}
}

// order checks that the statement s comes in its place (Figure 1-1),
// and moves the unit's group on. firsts are the first statement of each
// group the unit has reached.
func (a *analyzer) order(s ast.Stmt, stmtFunc bool) {
	p := placementOf(s, stmtFunc)
	if p.at == anywhere || a.rejected[s] {
		return
	}

	if a.group > p.latest {
		// The first statement that took the unit past this one's
		// place.
		var past ast.Stmt

		for g := p.latest + 1; g <= execGroup && past == nil; g++ {
			past = a.firsts[g]
		}

		if past == a.firsts[execGroup] {
			a.report("LATEDECL", s.Position(), past.Position().Line)
		} else {
			a.report("ORDER", s.Position(), orderName(s, stmtFunc), orderName(past, a.isStmtFunc(past)), past.Position().Line)
		}

		return
	}

	if p.at > a.group {
		a.group = p.at
	}

	if a.firsts[p.at] == nil {
		a.firsts[p.at] = s
	}
}

// orderName names the statement s for ORDER: "IMPLICIT statement",
// "IMPLICIT NONE statement", "statement function definition".
func orderName(s ast.Stmt, stmtFunc bool) string {
	if stmtFunc {
		return "statement function definition"
	}

	if st, ok := s.(*ast.ImplicitStmt); ok && st.None {
		return "IMPLICIT NONE statement"
	}

	return statementName(s) + " statement"
}
