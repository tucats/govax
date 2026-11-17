package sema

import (
	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
	"github.com/tucats/govax/internal/fortran/lex"
)

// What semantic analysis can compile so far, and what it says about the
// rest. The parser builds a node for every statement and form of VAX
// FORTRAN (Phase 76), and it is here, where analysis reaches one it has
// no meaning for yet, that it is reported: "the READ statement is not
// implemented yet" (NOTIMPL). Each later phase moves statements and forms
// from the "not yet" side to the other.

// handled reports whether semantic analysis compiles statement s. A
// statement it doesn't is reported where the statement walk reaches it
// (unimplemented), and keeps only its part in the unit's structure.
func handled(s ast.Stmt) bool {
	switch s.(type) {
	case *ast.TypeStmt, *ast.DimensionStmt, *ast.CommonStmt, *ast.EquivalenceStmt, *ast.ImplicitStmt, *ast.ParameterStmt,
		*ast.DataStmt, *ast.AssignStmt, *ast.CallStmt, *ast.GotoStmt, *ast.ComputedGotoStmt,
		*ast.AssignLabelStmt, *ast.AssignedGotoStmt, *ast.ArithIfStmt,
		*ast.IfStmt, *ast.IfThenStmt, *ast.ElseIfStmt, *ast.ElseStmt, *ast.EndIfStmt,
		*ast.DoStmt, *ast.DoWhileStmt, *ast.EndDoStmt, *ast.ContinueStmt, *ast.StopStmt, *ast.ReturnStmt, *ast.EndStmt,
		*ast.EntryStmt, *ast.StructureStmt, *ast.RecordStmt, *ast.FormatStmt, *ast.NamelistStmt, *ast.PrintStmt, *ast.CodeStmt, *ast.DefineFileStmt, *ast.AttrStmt, *ast.DirectiveStmt, *ast.BadStmt:
		return true

	case *ast.TransferStmt:
		return true

	case *ast.AuxIOStmt:
		return true
	}

	return false
}

// statementWhat names the statement s as a NOTIMPL message does: "the
// DICTIONARY statement"; for a Fortran 90 form of an old statement, the
// form ("the :: form of a type declaration").
func statementWhat(s ast.Stmt) string {
	if p, ok := s.(*ast.PendingStmt); ok && p.What != "" {
		return p.What
	}

	return "the " + s.Base().Kind.String() + " statement"
}

// pendingPosition is where a statement's NOTIMPL points: at the form it
// names, for a PendingStmt that names one, or at the statement.
func pendingPosition(s ast.Stmt) diag.Position {
	if p, ok := s.(*ast.PendingStmt); ok && p.WhatPos.Line != 0 {
		return p.WhatPos
	}

	return s.Position()
}

// pendingHeader is a unit's header if semantic analysis can't compile the
// unit because of it: a module's (Phase 106), or a header in a Fortran 90
// form not compiled yet (a PendingStmt). Otherwise nil.
func pendingHeader(u *ast.Unit) ast.Stmt {
	switch h := u.Header.(type) {
	case *ast.ModuleStmt, *ast.PendingStmt:
		return h
	}

	return nil
}

// unimplemented reports a statement semantic analysis doesn't compile
// yet, and keeps its part in the unit's block structure.
//
// A Fortran 90 construct's statements are reported once, at the statement
// that opens it: a CASE or END SELECT inside a SELECT CASE already
// reported, or an ELSEWHERE or END WHERE inside a block WHERE, says
// nothing more (pendingConstructs).
//
// PUBLIC and PRIVATE statements belong in a module (the standard's R522's
// constraint), and a module's statements never reach here (pendingHeader):
// anywhere else they are an error, NOTINMODULE.
func (a *analyzer) unimplemented(s ast.Stmt) {
	kind := s.Base().Kind

	if kind == lex.Public || kind == lex.Private {
		// Reported as NOTIMPL is, whatever else the unit lacks: the
		// statement makes the unit's declarations incomplete itself.
		a.messages.Report("NOTINMODULE", s.Position(), "the "+kind.String()+" statement")
		a.brokenStatement(kind, s)

		return
	}

	if quiet, ok := pendingInner[kind]; ok && a.pendingOpen[quiet] > 0 {
		if pendingClosers[kind] {
			a.pendingOpen[quiet]--
		}

		a.brokenStatement(kind, s)

		return
	}

	if _, opens := pendingOpeners[kind]; opens {
		a.pendingOpen[kind]++
	}

	a.notImplForm(pendingPosition(s), statementWhat(s))
	a.brokenStatement(kind, s)
}

// pendingOpeners, pendingInner, and pendingClosers are Fortran 90's
// constructs not compiled yet, for unimplemented: the statement that opens
// each, the statements inside it named by the one that opens their construct,
// and of those the ones that close it.
var (
	pendingOpeners = map[lex.Kind]bool{lex.SelectCase: true, lex.WhereBlock: true}
	pendingInner   = map[lex.Kind]lex.Kind{
		lex.Case: lex.SelectCase, lex.EndSelect: lex.SelectCase,
		lex.ElseWhere: lex.WhereBlock, lex.EndWhere: lex.WhereBlock,
	}
	pendingClosers = map[lex.Kind]bool{lex.EndSelect: true, lex.EndWhere: true}
)

// constructName reports Fortran 90's construct name on a statement, which
// Phase 101 compiles (OUTER: DO, END DO OUTER): the unit's first, since the
// rest would say the same.
func (a *analyzer) constructName(s ast.Stmt) {
	if b := s.Base(); b.Construct != "" && !a.constructReported {
		a.notImplForm(b.ConstructPos, "a construct name")
		a.constructReported = true
	}
}

// formWhat names an expression's form that semantic analysis doesn't
// compile yet, as a NOTIMPL message does ("an implied-DO list"), or ""
// for a form it compiles.
func formWhat(e ast.Expr) string {
	switch e.(type) {
	case *ast.ComplexConst:
		return "a COMPLEX constant"
	case *ast.ImpliedDo:
		return "an implied-DO list"
	case *ast.KeyList:
		return "a KEY= list"
	}

	return ""
}
