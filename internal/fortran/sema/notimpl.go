package sema

import (
	"github.com/tucats/govax/internal/fortran/ast"
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
// DICTIONARY statement".
func statementWhat(s ast.Stmt) string {
	return "the " + s.Base().Kind.String() + " statement"
}

// unimplemented reports a statement semantic analysis doesn't compile
// yet, and keeps its part in the unit's block structure.
func (a *analyzer) unimplemented(s ast.Stmt) {
	a.notImplForm(s.Position(), statementWhat(s))
	a.brokenStatement(s.Base().Kind, s)
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
