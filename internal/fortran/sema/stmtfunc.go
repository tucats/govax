package sema

import (
	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
)

// Statement functions (section 6.2.1): F(X, Y) = expression, a one-line
// function a unit defines for its own use. No routine is made for one. Each
// reference is expanded where it stands, when the unit is lowered (Decision 8
// of docs/PHASE-79), so analysis gives lowering the definition's typed
// expression once, and each reference its actual arguments.
//
// The dummy arguments belong to the definition alone: X is a new name there,
// taking only its type from the entity the unit calls X, which may be an array
// or a COMMON variable, and it has no storage. So while the definition's
// expression is analyzed each dummy argument's symbol stands in the unit's
// table in place of whatever has its name, and is taken out again after.

// StmtFuncDef is a statement function's definition: its symbol, its dummy
// arguments (symbols of their own, in none of the unit's tables), and its
// expression, converted to the function's type (a CHARACTER one keeps its own
// length, lowering cutting or padding it).
type StmtFuncDef struct {
	Sym     *Symbol
	Dummies []*Symbol
	Value   Expr
}

// defineStmtFunction analyzes the definition s of the statement function sym.
// A definition with an error leaves sym broken, so its references get no
// messages of their own.
func (a *analyzer) defineStmtFunction(s *ast.AssignStmt, sym *Symbol) {
	def := &StmtFuncDef{Sym: sym}
	saved := map[string]*Symbol{}
	ok := true

	for _, arg := range s.Target.(*ast.Apply).Args {
		n := arg.(*ast.Name)
		d := a.stmtFuncDummy(n)
		ok = ok && !d.broken

		if _, seen := saved[n.Name]; !seen {
			saved[n.Name] = a.unit.byName[n.Name]
		}

		def.Dummies = append(def.Dummies, d)
		a.unit.byName[n.Name] = d
	}

	a.defining = sym
	value := a.value(s.Value)
	a.defining = nil

	for name, was := range saved {
		if was == nil {
			delete(a.unit.byName, name)
		} else {
			a.unit.byName[name] = was
		}
	}

	if sym.untyped && !sym.Typed {
		a.report("NOTYPE", sym.Pos, sym.Name)

		ok = false
	} else if value.Type().Kind != Invalid {
		value = a.assignedValue(s.Pos, &Var{exprBase{sym.Type, s.Pos}, sym}, value)
	}

	if !ok || value == nil || value.Type().Kind == Invalid {
		sym.broken = true

		return
	}

	def.Value = value
	sym.StmtFunc = def
}

// stmtFuncDummy is a dummy argument's symbol, of the type the unit gives its
// name, whatever else the name is. Its length must be known here, since a
// statement function has no descriptor to take one from (FORTRAN-77's rule;
// ours, unconfirmed, for VAX FORTRAN: the manual doesn't say).
func (a *analyzer) stmtFuncDummy(n *ast.Name) *Symbol {
	d := &Symbol{Name: n.Name, Class: Variable, Offset: -1, Pos: n.Pos}

	var untyped bool

	if s := a.unit.byName[n.Name]; s != nil {
		d.Type, d.broken = s.Type, s.broken
		untyped = s.untyped && !s.Typed
	} else {
		d.Type, untyped = a.implicit.typeOf(n.Name)
	}

	switch {
	case d.broken:

	case untyped:
		a.report("NOTYPE", n.Pos, n.Name)

		d.broken = true

	case d.Type.Kind == Character && d.Type.Size == 0:
		a.report("STMTFUNCLEN", n.Pos, n.Name)

		d.broken = true
	}

	return d
}

// stmtFuncRef analyzes a reference to the statement function s. The actual
// arguments must agree with the dummies in number, order, and type (section
// 6.2.1; AVG(TEST3,TEST4,LAB2) with an INTEGER LAB2 is the manual's example of
// a reference that doesn't), but for a CHARACTER value's length, and a constant
// that fits a shorter type (actualFor).
func (a *analyzer) stmtFuncRef(s *Symbol, e *ast.Apply) Expr {
	switch {
	case a.defining == s:
		a.report("RECURSIVE", e.Pos, e.Name)

		return a.bad(e.Pos)

	case s.StmtFunc == nil:
		// A definition may use only those before it.
		a.report("STMTFUNCLATE", e.Pos, e.Name, s.defPos.Line)

		return a.bad(e.Pos)

	case len(e.Args) != len(s.StmtFunc.Dummies):
		a.report("ARGCOUNT", e.Pos, e.Name, count(len(s.StmtFunc.Dummies), "dummy argument"), s.defPos.Line, len(e.Args))

		return a.bad(e.Pos)
	}

	args := make([]Expr, len(e.Args))
	ok := true

	for i, arg := range e.Args {
		args[i] = a.actualFor(s, i, arg, e.Pos)
		ok = ok && args[i].Type().Kind != Invalid
	}

	if !ok {
		return a.bad(e.Pos)
	}

	return &StmtFuncRef{exprBase{s.Type, e.Pos}, s.StmtFunc, args}
}

// actualFor analyzes the actual argument for the statement function s's i-th
// dummy argument. An INTEGER constant that fits a shorter INTEGER dummy is one
// (section 2.2.1.1), and a LOGICAL constant is any LOGICAL.
func (a *analyzer) actualFor(s *Symbol, i int, arg ast.Expr, pos diag.Position) Expr {
	d := s.StmtFunc.Dummies[i]
	x := a.value(arg)
	t := x.Type()

	switch {
	case t.Kind == Invalid, t == d.Type, t.Kind == Character && d.Type.Kind == Character:
		return x
	}

	if c, ok := x.(*Const); ok && c.T.Kind == d.Type.Kind && (c.T.Kind == Logical || (c.T.Kind == Integer && fitsInteger(c.Int, d.Type.Size))) {
		return a.convert(x, d.Type)
	}

	a.report("STMTFUNCARG", x.Position(), i+1, s.Name, article(t.String()), d.Name, d.Type.String())

	return a.bad(pos)
}

// fitsInteger reports whether v is a value of an INTEGER of size bytes.
func fitsInteger(v int32, size int) bool {
	switch size {
	case 1:
		return v == int32(int8(v))
	case 2:
		return v == int32(int16(v))
	}

	return true
}
