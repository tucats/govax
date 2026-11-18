package sema

import (
	"slices"

	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
)

// Defined operators and defined assignment (the Fortran 90 standard's 7.1.3,
// 7.5.1.3, 12.3.2.1.1, and 12.3.2.1.2; docs/planning/PHASE-103 - Fortran 90
// procedures and interfaces.md, findings 12 and 13). An interface block for an
// operator names the functions that give it a meaning:
//
//	      INTERFACE OPERATOR(.DIST.)
//	        REAL FUNCTION DIST(P, Q)
//	          REAL, INTENT(IN) :: P(2), Q(2)
//	        END FUNCTION
//	      END INTERFACE
//	      D = A .DIST. B          ! DIST(A, B)
//
// and one for ASSIGNMENT(=) the subroutines that give X = Y a meaning, CALL S(X,
// (Y)). Each is like a generic name (generic.go): the reference becomes a
// reference to the specific procedure whose dummies its operands fit exactly, by
// position, chosen when compiling.
//
// The operator is a defined one (.DIST., the parser's ast.DefinedOp, a unary one
// binding tightest and a binary one loosest), or an intrinsic one extended to
// operands it has no meaning for: + on LOGICAL values, // on a CHARACTER and an
// INTEGER. An extension may not change what Fortran already means (7.1.3,
// 14.1.2.3): + of two REALs is always the intrinsic's, and a block's function
// that takes what the intrinsic does is DEFOPINTR. What only DEC gives a meaning
// (.AND. of INTEGERs, bit by bit; .XOR.; a LOGICAL assigned an INTEGER) an
// interface may take over, the author's answer 5: an operation is the interface's
// when one fits, and DEC's otherwise.
//
// The functions take one or two operands, never OPTIONAL, each INTENT(IN), and an
// operand passes as any actual argument does. Assignment's subroutines take two,
// the first INTENT(OUT) or INTENT(INOUT), the second INTENT(IN), and the value
// goes as a copy, as if in parentheses. Until Phase 105's derived types, which
// these are for, the types are FORTRAN's own, with meanings Fortran leaves open.

// assignmentKey is the key of ASSIGNMENT(=) among a unit's operators.
const assignmentKey = "="

// declareOperator records the block's operator or assignment as standing for the
// procedures specifics, its bodies' symbols.
func (a *analyzer) declareOperator(spec ast.GenericSpec, specifics []*Symbol) {
	key := spec.Op
	if spec.Kind == ast.GenericAssign {
		key = assignmentKey
	}

	if a.unit.operators == nil {
		a.unit.operators = map[string]*Generic{}
	}

	g := a.unit.operators[key]
	if g == nil {
		g = &Generic{Name: spec.String()}
		a.unit.operators[key] = g
	}

	for _, s := range specifics {
		if a.checkOperatorProcedure(key, g.Name, s) {
			g.Specifics = append(g.Specifics, s)
		}
	}
}

// checkOperatorProcedure checks that the specific procedure s can give the
// operator key (named name in messages) a meaning, or assignment, reporting what
// it can't.
func (a *analyzer) checkOperatorProcedure(key, name string, s *Symbol) bool {
	u := s.Interface
	if u == nil || u.incomplete {
		return false
	}

	pos := u.AST.Position()
	shapes := dummyShapes(u)

	if key == assignmentKey {
		switch {
		case s.Class != Subroutine:
			a.report("DEFASSIGNSUB", pos, s.Name)
		case len(u.Params) != 2 || len(u.AltPlaces) > 0 || shapes[0].optional || shapes[1].optional ||
			shapes[0].proc || shapes[1].proc || (u.Params[0].Intent != "OUT" && u.Params[0].Intent != "INOUT") ||
			u.Params[1].Intent != "IN":
			a.report("DEFASSIGNDUMMY", pos, s.Name)
		case intrinsicAssignment(u.Params[0].Type, u.Params[1].Type):
			a.report("DEFASSIGNINTR", pos, s.Name, article(u.Params[0].Type.String()), article(u.Params[1].Type.String()))
		default:
			return true
		}

		return false
	}

	n := len(u.Params)

	switch {
	case s.Class != Function:
		a.report("DEFOPFUNC", pos, s.Name, name)

		return false

	case n < 1 || n > 2 || slices.ContainsFunc(u.Params, func(p *Symbol) bool {
		return p.Optional || p.DummyProcedure() || p.Intent != "IN"
	}):
		a.report("DEFOPDUMMY", pos, s.Name, name)

		return false
	}

	if op, intrinsic := intrinsicOperators[key]; intrinsic {
		unary := op == ast.Not || op == ast.Add || op == ast.Sub
		binary := op != ast.Not

		if n == 1 && !unary || n == 2 && !binary {
			a.report("DEFOPCOUNT", pos, s.Name, name, n)

			return false
		}

		types := make([]Type, n)
		for i, p := range u.Params {
			types[i] = p.Type
		}

		if intrinsicMeaning(op, types) {
			a.report("DEFOPINTR", pos, s.Name, key)

			return false
		}
	}

	return true
}

// checkOperators checks 14.1.2.3's rule for operators and assignment over the
// unit's blocks: two procedures for one operator with as many operands, or for
// assignment, must differ in type, kind, or rank at some position (OPAMBIG).
func (a *analyzer) checkOperators() {
	keys := make([]string, 0, len(a.unit.operators))
	for k := range a.unit.operators {
		keys = append(keys, k)
	}

	slices.Sort(keys)

	for _, k := range keys {
		g := a.unit.operators[k]

		for i, p := range g.Specifics {
			for _, q := range g.Specifics[i+1:] {
				if p != q && !differAtSomePosition(dummyShapes(p.Interface), dummyShapes(q.Interface)) {
					a.report("OPAMBIG", q.Interface.AST.Position(), p.Name, p.Pos.Line, q.Name, g.Name)
				}
			}
		}
	}
}

// differAtSomePosition reports whether two procedures for one operator, or for
// assignment, can be told apart: they take different numbers of operands, or
// their dummies at some position differ.
func differAtSomePosition(p, q []dummyShape) bool {
	if len(p) != len(q) {
		return true
	}

	for i := range p {
		if differ(p[i], q[i]) {
			return true
		}
	}

	return false
}

// intrinsicOperators are the operators Fortran (and VAX FORTRAN) gives meanings
// of its own, by their spellings.
var intrinsicOperators = func() map[string]ast.Op {
	m := map[string]ast.Op{}
	for op := ast.Add; op <= ast.Xor; op++ {
		m[op.String()] = op
	}

	return m
}()

// numeric reports whether a type is one of Fortran's numeric types.
func numeric(t Type) bool {
	return t.Kind == Integer || t.Kind == Real || t.Kind == Complex
}

// intrinsicMeaning reports whether the standard gives the operator op a meaning
// for operands of the types: numeric operands for the arithmetic operators,
// CHARACTER ones for // and the relational operators, numeric ones for those too
// (but COMPLEX only for .EQ. and .NE.), LOGICAL ones for the logical operators.
// .XOR. is DEC's alone, and so are its other meanings (INTEGER operands of the
// logical operators): an interface may take those over (the author's answer 5).
// An intrinsic operation is elemental, so ranks don't matter.
func intrinsicMeaning(op ast.Op, types []Type) bool {
	all := func(test func(Type) bool) bool { return !slices.ContainsFunc(types, func(t Type) bool { return !test(t) }) }
	logical := func(t Type) bool { return t.Kind == Logical }
	character := func(t Type) bool { return t.Kind == Character }

	switch {
	case op == ast.Concat:
		return all(character)
	case op == ast.EQ || op == ast.NE:
		return all(numeric) || all(character)
	case op.Relational():
		return all(func(t Type) bool { return t.Kind == Integer || t.Kind == Real }) || all(character)
	case op == ast.Not || op == ast.And || op == ast.Or || op == ast.Eqv || op == ast.Neqv:
		return all(logical)
	case op == ast.Xor:
		return false
	}

	return all(numeric)
}

// intrinsicAssignment reports whether the standard gives assignment of a value of
// type v to a place of type t a meaning: numeric to numeric, LOGICAL to LOGICAL,
// CHARACTER to CHARACTER (7.5.1.4). VAX FORTRAN's others, and records, an
// interface may take over.
func intrinsicAssignment(t, v Type) bool {
	return numeric(t) && numeric(v) || t.Kind == v.Kind && (t.Kind == Logical || t.Kind == Character)
}

// operatorFor is the procedures for the operator key in this unit or a host of
// it, the unit's own first; nil if there are none.
func (a *analyzer) operatorFor(key string) []*Generic {
	var found []*Generic

	for u := a.unit; u != nil; u = u.Host {
		if g := u.operators[key]; g != nil && len(g.Specifics) > 0 {
			found = append(found, g)
		}
	}

	return found
}

// resolveOperator is the procedure for the operator key whose dummies the
// operands fit exactly, by position (generic.go's fits), or nil.
func (a *analyzer) resolveOperator(key string, operands []Arg) *Symbol {
	slots := make([]int, len(operands))
	for i := range slots {
		slots[i] = i
	}

	for _, g := range a.operatorFor(key) {
		for _, s := range g.Specifics {
			if u := s.Interface; len(u.Params) == len(operands) && fits(slots, operands, u) {
				return s
			}
		}
	}

	return nil
}

// operandArgs are an operation's operands as a procedure's actual arguments,
// each passed as itself, as any actual argument is.
func operandArgs(operands []Expr) []Arg {
	args := make([]Arg, len(operands))
	for i, x := range operands {
		args[i] = Arg{Expr: x}
	}

	return args
}

// operation is the reference to the function s for an operation at pos.
func (a *analyzer) operation(s *Symbol, args []Arg, pos diag.Position) Expr {
	a.note(&s.refs, pos, RefUsed)
	a.unit.calls = append(a.unit.calls, routineRef{sym: s, args: args, pos: pos})

	return &FuncRef{exprBase{T: s.Type, Pos: pos}, s, args}
}

// extendedOp is the operation op on operands (analyzed) as an interface block
// extends the intrinsic operator, if one does for operands Fortran gives no
// meaning; ok is false for the intrinsic operation (or its error) to follow.
func (a *analyzer) extendedOp(op ast.Op, pos diag.Position, operands ...Expr) (Expr, bool) {
	if len(a.operatorFor(op.String())) == 0 {
		return nil, false
	}

	types := make([]Type, len(operands))

	for i, x := range operands {
		if types[i] = x.Type(); types[i].Kind == Invalid {
			return nil, false
		}
	}

	if intrinsicMeaning(op, types) {
		return nil, false
	}

	args := operandArgs(operands)

	s := a.resolveOperator(op.String(), args)
	if s == nil {
		return nil, false
	}

	return a.operation(s, args, pos), true
}

// definedOp analyzes a defined operation, X .OP. Y or .OP. X: a reference to the
// function of the operator's interface its operands fit.
func (a *analyzer) definedOp(e *ast.DefinedOp) Expr {
	operands := []Expr{a.arrayValue(e.X)}
	list := []ast.Expr{e.X}

	if e.Y != nil {
		operands = append(operands, a.arrayValue(e.Y))
		list = append(list, e.Y)
	}

	for _, x := range operands {
		if x.Type().Kind == Invalid {
			return a.bad(e.Pos)
		}
	}

	if len(a.operatorFor(e.Op)) == 0 {
		a.report("OPUNDEFINED", e.Pos, e.Op)

		return a.bad(e.Pos)
	}

	args := operandArgs(operands)

	s := a.resolveOperator(e.Op, args)
	if s == nil {
		a.report("DEFOPNONE", e.Pos, e.Op, describeActuals(list, args))

		return a.bad(e.Pos)
	}

	return a.operation(s, args, e.Pos)
}

// definedAssignment is the assignment target = value as an interface block for
// ASSIGNMENT(=) defines it, CALL S(target, (value)), when Fortran gives it no
// meaning and a subroutine of the block fits; nil otherwise.
func (a *analyzer) definedAssignment(s *ast.AssignStmt, target, value Expr) Stmt {
	if len(a.operatorFor(assignmentKey)) == 0 || intrinsicAssignment(target.Type(), value.Type()) {
		return nil
	}

	args := []Arg{{Expr: target}, {Expr: value}}

	switch value.(type) {
	case *Var, *Elem, *Substring, *FieldRef:
		args[1].Copy = true
	}

	sub := a.resolveOperator(assignmentKey, args)
	if sub == nil {
		return nil
	}

	a.note(&sub.refs, s.Pos, RefUsed)
	a.unit.calls = append(a.unit.calls, routineRef{sym: sub, args: args, pos: s.Pos})

	return &Call{stmtBase: stmtBase{s}, Sub: sub, Args: args}
}
