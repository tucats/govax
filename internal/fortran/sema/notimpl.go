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
	switch s := s.(type) {
	case *ast.TypeStmt, *ast.DimensionStmt, *ast.CommonStmt, *ast.EquivalenceStmt, *ast.ImplicitStmt, *ast.ParameterStmt,
		*ast.DataStmt, *ast.AssignStmt, *ast.CallStmt, *ast.GotoStmt, *ast.ComputedGotoStmt,
		*ast.AssignLabelStmt, *ast.AssignedGotoStmt, *ast.ArithIfStmt,
		*ast.IfStmt, *ast.IfThenStmt, *ast.ElseIfStmt, *ast.ElseStmt, *ast.EndIfStmt,
		*ast.DoStmt, *ast.DoWhileStmt, *ast.EndDoStmt, *ast.LoopExitStmt, *ast.SelectCaseStmt, *ast.CaseStmt, *ast.EndSelectStmt,
		*ast.WhereStmt, *ast.WhereBlockStmt, *ast.ElseWhereStmt, *ast.EndWhereStmt, *ast.ContinueStmt, *ast.StopStmt, *ast.ReturnStmt, *ast.EndStmt,
		*ast.AllocateStmt, *ast.PointerAssignStmt, *ast.NullifyStmt, *ast.EntryStmt, *ast.UseStmt, *ast.ContainsStmt, *ast.StructureStmt, *ast.TypeDefStmt, *ast.RecordStmt, *ast.FormatStmt, *ast.NamelistStmt, *ast.PrintStmt, *ast.CodeStmt, *ast.DefineFileStmt, *ast.AttrStmt, *ast.DirectiveStmt, *ast.BadStmt:
		return true

	case *ast.TransferStmt:
		return true

	case *ast.AuxIOStmt:
		return true

	case *ast.InterfaceBlock:
		return compiledBlock(s)
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

	if b, ok := s.(*ast.InterfaceBlock); ok {
		return interfaceWhat(b)
	}

	switch s.(type) {
	case *ast.ForallBlockStmt:
		return "the FORALL construct"
	case *ast.EndForallStmt:
		return "the END FORALL statement"
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
	case *ast.PendingStmt:
		return h
	}

	return nil
}

// unimplemented reports a statement semantic analysis doesn't compile
// yet, and keeps its part in the unit's block structure.
//
// A Fortran 90 construct's statements are reported once, at the statement
// that opens it: the statements inside it say nothing more (pendingOpeners).
//
// PUBLIC and PRIVATE statements belong in a module (the standard's R522's
// constraint), where access.go reads them: anywhere else they are an error,
// NOTINMODULE.
func (a *analyzer) unimplemented(s ast.Stmt) {
	kind := s.Base().Kind

	if (kind == lex.Public || kind == lex.Private) && a.unit.Kind == ast.ModuleUnit {
		// Read with the declarations (access.go).
		return
	}

	if kind == lex.Public || kind == lex.Private {
		// Reported as NOTIMPL is, whatever else the unit lacks: the
		// statement makes the unit's declarations incomplete itself.
		a.messages.Report("NOTINMODULE", s.Position(), "the "+kind.String()+" statement")
		a.brokenStatement(kind, s)

		return
	}

	if quiet, ok := pendingInner[kind]; ok && a.pendingOpen[quiet] > 0 {
		switch {
		case pendingClosers[kind]:
			a.pendingOpen[quiet]--
		case pendingOpeners[kind]:
			a.pendingOpen[kind]++
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

// pendingOpeners, pendingInner, and pendingClosers are the constructs not
// compiled yet, for unimplemented: the statement that opens each, the
// statements inside it named by the one that opens their construct (a nested
// opener among them, counted but not reported again), and of those the ones
// that close it. pendingBodies are the constructs whose every statement is
// passed over quietly (quietBody), since what they mean depends on the
// construct: a FORALL's assignments use its indexes (docs/planning/PHASE-112 -
// Fortran 95.md, subtask 2, until subtask 9 compiles the construct).
var (
	pendingOpeners = map[lex.Kind]bool{lex.ForallBlock: true}
	pendingInner   = map[lex.Kind]lex.Kind{lex.ForallBlock: lex.ForallBlock, lex.Forall: lex.ForallBlock, lex.EndForall: lex.ForallBlock}
	pendingClosers = map[lex.Kind]bool{lex.EndForall: true}
	pendingBodies  = map[lex.Kind]bool{lex.ForallBlock: true}
)

// quietBody reports whether s, a statement semantic analysis compiles, is
// inside a construct of pendingBodies, and if so keeps its part in the block
// structure without analyzing it.
func (a *analyzer) quietBody(s ast.Stmt) bool {
	for k := range pendingBodies {
		if a.pendingOpen[k] > 0 && handled(s) {
			a.brokenStatement(s.Base().Kind, s)

			return true
		}
	}

	return false
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
	case *ast.Triplet:
		return "an array section"
	case *ast.ArrayConstructor:
		return "an array constructor"
	case *ast.KeyList:
		return "a KEY= list"
	case *ast.KeywordArg:
		// An intrinsic function's reference takes its keyword arguments
		// (intrinsicSlots), and a reference through an explicit interface
		// (optional.go's actuals); one is left only where neither does, as in
		// a statement function's reference.
		return "a keyword argument here"
	case *ast.DefinedOp:
		return "a defined operator"
	}

	return ""
}
