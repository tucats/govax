package sema

import (
	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
	"github.com/tucats/govax/internal/fortran/lex"
)

// Fortran 90's construct names, and EXIT and CYCLE (the standard's 8.1 and
// 8.1.4.4; docs/planning/PHASE-101 - Fortran 90 control constructs.md).
//
// A DO loop, a block IF, or a SELECT CASE may be named: OUTER: DO. The name
// is the construct's (construct.name, set as the stack opens it, push), and
// the statements inside the construct that may repeat it (ELSE IF, ELSE, END
// IF, END DO, CASE, END SELECT) must name that construct and no other. One
// named at its start must be named at its end; one not named must not be
// (checkName). A named DO with a label must end at an END DO naming it, not
// at a labeled CONTINUE (endLoops).
//
// A construct name is a name of the unit like a variable's (the standard's
// 14.1.2, "local entities of class (1)"): no two constructs may share one
// (DUPCONSTRUCT), nor may a construct and a variable, a constant, a
// procedure, a statement function, a namelist group, or the unit itself
// (NAMECLASH, checked when the unit's statements are all read, since a
// variable may be first mentioned after the construct). A COMMON block may
// share a construct's name: its names are a class of their own.
//
// EXIT and CYCLE belong to a DO loop: the innermost one around them, or the
// one they name, which must be open around them and must be a loop
// (loopExit). The node they become, LoopExit, points at the loop's Do or
// DoWhile, which is all the IR needs to find where to go.

// whatConstruct is what the cross reference calls a construct name.
const whatConstruct = "construct"

// constructName is a construct name used in the unit: where it is given and
// on what line its construct starts, for DUPCONSTRUCT and NAMECLASH.
type constructName struct {
	name string
	pos  diag.Position
	line int
}

// opener is the statement that opened the construct: its DO, DO WHILE, IF
// THEN, or SELECT CASE.
func (c *construct) opener() ast.Stmt {
	switch {
	case c.do != nil:
		return c.do.AST
	case c.while != nil:
		return c.while.AST
	case c.selStart != nil:
		return c.selStart
	}

	return c.ifStart
}

// nameConstruct gives c its name, if its first statement has one, and
// records the name for the unit's checks: a name already given to another
// construct is DUPCONSTRUCT.
func (a *analyzer) nameConstruct(c *construct) {
	s := c.opener()
	if s == nil || s.Base().Construct == "" {
		return
	}

	b := s.Base()
	c.name = b.Construct
	a.noteOther(whatConstruct, b.Construct, b.ConstructPos, RefDefined)

	for _, n := range a.constructNames {
		if n.name == b.Construct {
			a.report("DUPCONSTRUCT", b.ConstructPos, b.Construct, n.line)

			return
		}
	}

	a.constructNames = append(a.constructNames, constructName{b.Construct, b.ConstructPos, s.Position().Line})
}

// checkName checks the construct name on s, a statement inside the
// construct c (ELSE IF, ELSE, END IF, END DO): any name it gives must be
// c's (WRONGNAME), and a statement that ends c (closing) must give c's name
// if c has one (MISSINGNAME).
func (a *analyzer) checkName(c *construct, s ast.Stmt, closing bool) {
	b := s.Base()

	switch {
	case b.Construct != "" && b.Construct != c.name:
		a.noteOther(whatConstruct, b.Construct, b.ConstructPos, RefUsed)

		has := "has no name"
		if c.name != "" {
			has = "is named " + c.name
		}

		a.report("WRONGNAME", b.ConstructPos, statementName(s), b.Construct, c.what(), c.line(), has)

	case b.Construct != "":
		a.noteOther(whatConstruct, b.Construct, b.ConstructPos, RefUsed)

	case closing && c.name != "":
		a.report("MISSINGNAME", s.Position(), c.what(), c.line(), c.name, statementName(s))
	}
}

// loopExit analyzes EXIT or CYCLE: it belongs to the loop it names, or
// without a name to the innermost loop around it. Outside every loop it is
// NOTINLOOP; a name no open construct has is NOLOOPNAME, and one an IF
// block or SELECT CASE has is NOTALOOP. Nothing is made of a statement
// with an error.
func (a *analyzer) loopExit(s *ast.LoopExitStmt) Stmt {
	verb := statementName(s)

	if s.Name != "" {
		a.noteOther(whatConstruct, s.Name, s.NamePos, RefUsed)
	}

	var c *construct

	for i := len(a.open) - 1; i >= 0 && c == nil; i-- {
		if o := a.open[i]; s.Name == "" && o.isDo() || s.Name != "" && o.name == s.Name {
			c = o
		}
	}

	switch {
	case c == nil && s.Name == "":
		a.report("NOTINLOOP", s.Pos, verb)

		return nil

	case c == nil:
		a.report("NOLOOPNAME", s.NamePos, verb, s.Name)

		return nil

	case !c.isDo():
		a.report("NOTALOOP", s.NamePos, verb, s.Name, c.what(), c.line())

		return nil
	}

	st := &LoopExit{stmtBase: stmtBase{s}, Cycle: s.Kind == lex.Cycle}

	if c.do != nil {
		st.Loop = c.do
	} else {
		st.Loop = c.while
	}

	return st
}

// checkConstructNames reports each construct name that also names something
// else of the unit's (NAMECLASH): a variable, a constant, a procedure, a
// statement function, the unit itself, or a namelist group.
func (a *analyzer) checkConstructNames() {
	for _, n := range a.constructNames {
		what := ""

		if s := a.scope.find(n.name); s != nil {
			what = classWhat(s)
		}

		for _, g := range a.unit.Namelists {
			if g.Name == n.name {
				what = "a NAMELIST group"
			}
		}

		if what != "" {
			a.report("NAMECLASH", n.pos, n.name, n.line, what)
		}
	}
}

// classWhat says what the symbol s is, for NAMECLASH: "a variable", "the
// program unit".
func classWhat(s *Symbol) string {
	switch s.Class {
	case UnitName:
		return "the program unit"
	case Constant:
		return "a named constant"
	case Function, Subroutine, External, Intrinsic:
		return "a procedure"
	case StmtFunction:
		return "a statement function"
	case Array:
		return "an array"
	}

	if s.Result {
		return "the function's result"
	}

	return "a variable"
}
