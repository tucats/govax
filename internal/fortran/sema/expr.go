package sema

import (
	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
)

// Expressions: each syntax tree expression becomes a typed one. Names
// are looked up (an Apply becomes an array element or a function
// reference), each operation gets its type by section 2.3's rules, the
// operand of lower rank in a mixed operation gets an explicit Convert,
// and constant operations are folded.

// hollerith is NOTIMPL's subject for a character constant where a
// number is wanted: VAX FORTRAN takes it for a Hollerith constant there
// (section 2.2.1.6), which is Phase 82's.
const hollerith = "a character constant in a numeric context (a Hollerith constant)"

// bad is an expression whose error has been reported.
func (a *analyzer) bad(pos diag.Position) Expr {
	return &bad{exprBase{invalid, pos}}
}

// value analyzes an expression whose value is used. A whole array isn't
// a value (NEEDSUBSCRIPT); a character constant is, of type CHARACTER,
// for the caller to accept or not.
func (a *analyzer) value(e ast.Expr) Expr {
	switch e := e.(type) {
	case *ast.IntConst:
		return a.intConst(e)

	case *ast.RealConst:
		return a.realConst(e)

	case *ast.LogicalConst:
		return logicalConst(e.Value, e.Pos)

	case *ast.CharConst:
		return &Const{exprBase: exprBase{Type{Character, len(e.Value)}, e.Pos}, Text: e.Value}

	case *ast.Name:
		return a.name(e, false)

	case *ast.Apply:
		return a.apply(e)

	case *ast.Paren:
		// Parentheses group, and the tree's shape keeps the grouping;
		// they matter beyond that only around an actual argument (args).
		return a.value(e.X)

	case *ast.Unary:
		return a.unary(e)

	case *ast.Binary:
		return a.binary(e)
	}

	if c, ok := e.(*ast.ComplexConst); ok {
		v, _ := a.folded(c)

		return v
	}

	if what := formWhat(e); what != "" {
		a.notImplForm(e.Position(), what)
	}

	return a.bad(e.Position())
}

// folded is the value of e if it is a constant expression (constant.go),
// for a form this phase compiles only as one: **, the intrinsic
// functions, character operations, REAL*8 and COMPLEX constants. ok is
// false if e isn't constant, for the caller to report the form as it
// would. A constant of a type the code generator doesn't handle yet is
// NOTIMPL (Phase 78's); a CHARACTER one is a value wherever a character
// constant is.
func (a *analyzer) folded(e ast.Expr) (v Expr, ok bool) {
	// First quietly, to learn whether e is constant at all, then for
	// its messages.
	if _, err := (&evaluator{a: a, quiet: true}).eval(e); err != nil && err.kind != constFailed {
		return nil, false
	}

	c, err := (&evaluator{a: a}).eval(e)

	switch {
	case err != nil:
		return a.bad(e.Position()), true

	case !c.T.Compiled() && c.T.Kind != Character:
		a.notImpl(e.Position(), article(c.T.String()+" constant"))

		return a.bad(e.Position()), true
	}

	return c, true
}

// name analyzes a name standing alone: a variable, or, where wholeOK
// allows it (an actual argument, an I/O list item), a whole array.
func (a *analyzer) name(n *ast.Name, wholeOK bool) Expr {
	s := a.lookup(n.Name, n.Pos)

	if s.broken {
		return a.bad(n.Pos)
	}

	if s.Class == Constant {
		return a.constantUse(s, n)
	}

	if (s.Class == Unclassified || s.Class == Variable || s.Class == Array) && a.uncompiledUse(s, n.Pos) {
		return a.bad(n.Pos)
	}

	switch s.Class {
	case Unclassified:
		s.Class = Variable

		fallthrough

	case Variable:
		return &Var{exprBase{s.Type, n.Pos}, s}

	case Array:
		if wholeOK {
			return &ArrayRef{exprBase{s.Type, n.Pos}, s}
		}

		a.report("NEEDSUBSCRIPT", n.Pos, n.Name)

	default:
		a.badUse(s, n.Pos, "be used as a variable")
	}

	return a.bad(n.Pos)
}

// apply analyzes Name(args): an array element if Name is an array, and
// otherwise a reference to an external function of Name's type. A name
// the unit hasn't classified that names one of FORTRAN's intrinsic
// functions is that function, Phase 78's.
func (a *analyzer) apply(e *ast.Apply) Expr {
	s := a.unit.byName[e.Name]

	if s == nil || (s.Class == Unclassified && s.Dummy == 0) {
		if _, ok := constFuncs[e.Name]; ok {
			if v, ok := a.folded(e); ok {
				return v
			}
		}

		if what := builtin(e.Name, true); what != "" {
			a.notImpl(e.Pos, what)

			return a.bad(e.Pos)
		}
	}

	s = a.lookup(e.Name, e.Pos)

	if s.broken {
		return a.bad(e.Pos)
	}

	switch s.Class {
	case StmtFunction:
		// A reference to a statement function, whose definition is
		// reported NOTIMPL (Phase 79).
		return a.bad(e.Pos)

	case Array:
		if a.uncompiledUse(s, e.Pos) {
			return a.bad(e.Pos)
		}

		return a.element(s, e)

	case Unclassified:
		if s.Dummy != 0 {
			// A dummy argument used as a function is a dummy procedure.
			a.notImpl(e.Pos, "a dummy procedure")

			return a.bad(e.Pos)
		}

		s.Class = Function

		fallthrough

	case Function:
		if a.uncompiledUse(s, e.Pos) {
			return a.bad(e.Pos)
		}

		args := a.args(e.Args)
		a.unit.calls = append(a.unit.calls, routineRef{s, len(args), e.Pos})

		return &FuncRef{exprBase{s.Type, e.Pos}, s, args}

	case Variable:
		if s.Result {
			a.report("RECURSIVE", e.Pos, e.Name)
		} else {
			a.report("NOTARRAY", e.Pos, e.Name)
		}

	case UnitName:
		if a.unit.Kind == ast.SubroutineUnit {
			a.report("RECURSIVE", e.Pos, e.Name)
		} else {
			a.badUse(s, e.Pos, "be used as a function")
		}

	default:
		a.badUse(s, e.Pos, "be used as a function")
	}

	return a.bad(e.Pos)
}

// uncompiledUse reports, once, a name used in an expression whose type
// this phase doesn't compile (NOTIMPL: "the REAL*8 type", Phase 78's),
// and makes the name broken, so its other uses are quiet. It reports
// whether it did. A name with no type at all (IMPLICIT NONE) has its own
// message, NOTYPE.
func (a *analyzer) uncompiledUse(s *Symbol, pos diag.Position) bool {
	if s.Type.Kind == Invalid || s.Type.Compiled() {
		return false
	}

	a.notImplForm(pos, "the "+s.Type.String()+" type")

	s.broken = true

	return true
}

// element analyzes an array element, s(subscripts): one subscript per
// dimension, each made INTEGER (a REAL one is truncated, section
// 2.2.3.2).
func (a *analyzer) element(s *Symbol, e *ast.Apply) Expr {
	if len(e.Args) != len(s.Dims) {
		a.report("SUBSCRIPTS", e.Pos, e.Name, count(len(s.Dims), "subscript"), len(e.Args))

		return a.bad(e.Pos)
	}

	subs := make([]Expr, len(e.Args))
	ok := true

	for i, arg := range e.Args {
		subs[i] = a.convert(a.arithmetic(a.value(arg)), IntegerType)
		ok = ok && subs[i].Type().Kind != Invalid
	}

	if !ok {
		return a.bad(e.Pos)
	}

	return &Elem{exprBase{s.Type, e.Pos}, s, subs}
}

// args analyzes a list of actual arguments (section 5.1, 6.1): a
// variable, an array element, or a whole array is passed as itself; a
// parenthesized one, or any other expression, as a copy. A character
// constant passed to a numeric dummy is a Hollerith constant (section
// 6.1.1.4), Phase 82's.
func (a *analyzer) args(list []ast.Expr) []Arg {
	args := make([]Arg, len(list))

	for i, e := range list {
		switch x := e.(type) {
		case nil:
			// A null argument: CALL S(A,,B).

		case *ast.Name:
			args[i].Expr = a.name(x, true)

		case *ast.CharConst:
			a.notImpl(x.Pos, "a character constant as an argument")

			args[i].Expr = a.bad(x.Pos)

		case *ast.Paren:
			args[i].Expr = a.value(x)

			switch args[i].Expr.(type) {
			case *Var, *Elem:
				args[i].Copy = true
			}

		default:
			args[i].Expr = a.value(x)
		}

		if c, ok := args[i].Expr.(*Const); ok && c.T.Kind == Character {
			a.notImpl(c.Pos, "a character constant as an argument")

			args[i].Expr = a.bad(c.Pos)
		}
	}

	return args
}

// arithmetic makes e an arithmetic operand: a LOGICAL value is used as
// an INTEGER (section 2.3.1.2: "Logical entities used in an arithmetic
// context are treated as integers"), and a character constant is a
// Hollerith constant, NOTIMPL.
func (a *analyzer) arithmetic(e Expr) Expr {
	switch e.Type().Kind {
	case Logical:
		return a.convert(e, IntegerType)
	case Character:
		a.notImpl(e.Position(), hollerith)

		return a.bad(e.Position())
	}

	return e
}

// convert is e converted to type t: e itself if it already has t's
// kind, else a Convert (folded, if e is a constant).
func (a *analyzer) convert(e Expr, t Type) Expr {
	if k := e.Type().Kind; k == t.Kind || k == Invalid {
		return e
	}

	return a.fold(&Convert{exprBase{t, e.Position()}, e})
}

// unary analyzes -X, +X, and .NOT. X.
func (a *analyzer) unary(e *ast.Unary) Expr {
	// -2147483648 is INTEGER's smallest value, but 2147483648 alone is
	// out of range: the sign and the digits are taken together.
	if c, ok := e.X.(*ast.IntConst); ok && e.Op == ast.Sub && c.Text == "2147483648" {
		return &Const{exprBase: exprBase{IntegerType, e.Pos}, Int: minInt}
	}

	x := a.value(e.X)

	switch {
	case x.Type().Kind == Invalid:
		return x

	case e.Op == ast.Not:
		switch x.Type().Kind {
		case Logical, Integer:
			// On a LOGICAL, the logical negation; on an INTEGER, every
			// bit's (section 2.3.4).
			return a.fold(&Unary{exprBase{x.Type(), e.Pos}, ast.Not, x})
		case Character:
			a.notImpl(x.Position(), hollerith)
		default:
			a.report("BADOPERAND", e.Pos, ".NOT.", x.Type().String())
		}

		return a.bad(e.Pos)
	}

	x = a.arithmetic(x)

	if e.Op == ast.Add || x.Type().Kind == Invalid {
		return x
	}

	return a.fold(&Unary{exprBase{x.Type(), e.Pos}, ast.Sub, x})
}

// binary analyzes X op Y.
func (a *analyzer) binary(e *ast.Binary) Expr {
	switch e.Op {
	case ast.Pow, ast.Concat, ast.Eqv, ast.Neqv, ast.Xor:
		if v, ok := a.folded(e); ok {
			return v
		}
	}

	switch e.Op {
	case ast.Pow:
		// Exponentiation calls the OTS$ powers (Phases 77 and 78).
		a.notImpl(e.Pos, "the ** operator")

		return a.bad(e.Pos)

	case ast.Concat:
		a.notImpl(e.Pos, "character concatenation (//)")

		return a.bad(e.Pos)

	case ast.Eqv, ast.Neqv, ast.Xor:
		a.notImpl(e.Pos, "the "+e.Op.String()+" operator")

		return a.bad(e.Pos)
	}

	x, y := a.value(e.X), a.value(e.Y)
	if x.Type().Kind == Invalid || y.Type().Kind == Invalid {
		return a.bad(e.Pos)
	}

	switch {
	case e.Op.Relational():
		return a.compare(e, x, y)

	case e.Op == ast.And || e.Op == ast.Or:
		return a.logical(e, x, y)
	}

	x, y = a.arithmetic(x), a.arithmetic(y)
	if x.Type().Kind == Invalid || y.Type().Kind == Invalid {
		return a.bad(e.Pos)
	}

	t := higher(x.Type(), y.Type())

	return a.fold(&Binary{exprBase{t, e.Pos}, e.Op, a.convert(x, t), a.convert(y, t)})
}

// compare analyzes a relational expression (section 2.3.3): its operands
// are converted to the higher-ranked one's type, and its value is
// LOGICAL.
func (a *analyzer) compare(e *ast.Binary, x, y Expr) Expr {
	if x.Type().Kind == Character && y.Type().Kind == Character {
		cx, okx := x.(*Const)
		cy, oky := y.(*Const)

		if okx && oky {
			return logicalConst(relation(e.Op, compareText(cx.Text, cy.Text)), e.Pos)
		}

		a.notImpl(e.Pos, "comparing character values")

		return a.bad(e.Pos)
	}

	x, y = a.arithmetic(x), a.arithmetic(y)
	if x.Type().Kind == Invalid || y.Type().Kind == Invalid {
		return a.bad(e.Pos)
	}

	t := higher(x.Type(), y.Type())

	return a.fold(&Compare{exprBase{LogicalType, e.Pos}, e.Op, a.convert(x, t), a.convert(y, t)})
}

// logical analyzes .AND. and .OR. (section 2.3.4): on LOGICAL operands
// the result is LOGICAL; on INTEGER operands, or one of each, the
// operation is done bit by bit and the result is INTEGER. Nothing else
// may be an operand.
func (a *analyzer) logical(e *ast.Binary, x, y Expr) Expr {
	for _, o := range []Expr{x, y} {
		switch o.Type().Kind {
		case Character:
			a.notImpl(o.Position(), hollerith)

			return a.bad(e.Pos)

		case Real:
			a.report("BADOPERAND", e.Pos, e.Op.String(), o.Type().String())

			return a.bad(e.Pos)
		}
	}

	t := LogicalType
	if x.Type().Kind == Integer || y.Type().Kind == Integer {
		t = IntegerType
	}

	return a.fold(&Binary{exprBase{t, e.Pos}, e.Op, a.convert(x, t), a.convert(y, t)})
}

// condition analyzes an IF statement's condition, a logical expression
// (section 2.3.4), which may be INTEGER as well as LOGICAL: either way
// its truth is its bit 0, so an INTEGER condition is converted to
// LOGICAL, with no change to its bits.
func (a *analyzer) condition(e ast.Expr) Expr {
	c := a.value(e)

	switch c.Type().Kind {
	case Logical, Invalid:
		return c
	case Integer:
		return a.convert(c, LogicalType)
	case Character:
		a.notImpl(c.Position(), hollerith)
	default:
		a.report("NOTLOGICAL", e.Position(), c.Type().String())
	}

	return a.bad(e.Position())
}
