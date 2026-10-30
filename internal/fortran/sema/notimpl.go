package sema

import (
	"github.com/tucats/govax/internal/fortran/ast"
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
		*ast.AssignStmt, *ast.CallStmt, *ast.GotoStmt,
		*ast.IfStmt, *ast.IfThenStmt, *ast.ElseIfStmt, *ast.ElseStmt, *ast.EndIfStmt,
		*ast.ContinueStmt, *ast.ReturnStmt, *ast.EndStmt, *ast.BadStmt:
		return true

	case *ast.DoStmt:
		// A loop without a label, which END DO ends, is Phase 79's.
		return s.Target.Label != 0

	case *ast.StopStmt:
		return s.Kind == lex.Stop

	case *ast.PrintStmt:
		return s.Kind == lex.Print

	case *ast.TransferStmt:
		return s.Kind == lex.Write
	}

	return false
}

// statementWhat names the statement s as a NOTIMPL message does: "the
// READ statement", "a CDEC$ directive".
func statementWhat(s ast.Stmt) string {
	switch k := s.Base().Kind; {
	case k == lex.Directive:
		return "a CDEC$ directive"
	case k == lex.Do:
		return "a DO loop without a statement label"
	default:
		return "the " + k.String() + " statement"
	}
}

// unimplemented reports a statement semantic analysis doesn't compile
// yet, and keeps its part in the unit's block structure.
func (a *analyzer) unimplemented(s ast.Stmt) {
	a.notImplForm(s.Position(), statementWhat(s))
	a.brokenStatement(s.Base().Kind, s)
}

// formWhat names an expression's form that semantic analysis doesn't
// compile yet, as a NOTIMPL message does ("a Hollerith constant"), or ""
// for a form it compiles.
func formWhat(e ast.Expr) string {
	switch e := e.(type) {
	case *ast.HollerithConst:
		return "a Hollerith constant"
	case *ast.TypelessConst:
		return "an octal or hexadecimal constant"
	case *ast.ComplexConst:
		return "a COMPLEX constant"
	case *ast.Substring:
		return "a character substring"
	case *ast.FieldRef:
		return "a record's field"
	case *ast.BuiltinArg:
		return "%" + e.Func
	case *ast.AltReturn:
		return "an alternate return argument"
	case *ast.ImpliedDo:
		return "an implied-DO list"
	case *ast.KeyList:
		return "a KEY= list"
	}

	return ""
}
