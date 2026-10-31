package sema

import (
	"math"

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
	case *ast.IntConst, *ast.RealConst:
		v, _ := a.folded(e)

		return v

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

	case *ast.Substring:
		return a.substring(e)
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
// for a form this phase compiles only as one (**, the intrinsic
// functions, character operations), and for a numeric or COMPLEX
// constant, whose value it is. ok is false if e isn't constant, for the
// caller to report the form as it would.
func (a *analyzer) folded(e ast.Expr) (v Expr, ok bool) {
	// First quietly, to learn whether e is constant at all, then for
	// its messages.
	if _, err := (&evaluator{a: a, quiet: true}).eval(e); err != nil && err.kind != constFailed {
		return nil, false
	}

	c, err := (&evaluator{a: a}).eval(e)
	if err != nil {
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

	if s.External || s.Class == Intrinsic {
		// A procedure's name where a value would be: as an argument, the
		// procedure itself (Phase 79); anywhere else, a mistake.
		if wholeOK {
			a.notImpl(n.Pos, "a procedure as an argument")
		} else {
			a.badUse(s, n.Pos, "be used as a variable")
		}

		return a.bad(n.Pos)
	}

	if (s.Class == Unclassified || s.Class == Variable || s.Class == Array) && a.characterDummy(s, n.Pos) {
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

	if s == nil || (s.Class == Unclassified && s.Dummy == 0) || s.Class == Intrinsic {
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
		if a.characterDummy(s, e.Pos) {
			return a.bad(e.Pos)
		}

		return a.element(s, e)

	case Unclassified, External:
		if s.Dummy != 0 {
			// A dummy argument used as a function is a dummy procedure.
			a.notImpl(e.Pos, "a dummy procedure")

			return a.bad(e.Pos)
		}

		s.Class = Function

		fallthrough

	case Function:
		if t := s.Type; t.Kind != Invalid && !t.returned() {
			a.notImplForm(e.Pos, article(t.String()+" function"))

			s.broken = true

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

// characterDummy reports, once, a CHARACTER dummy argument used in an
// expression (NOTIMPL): its value comes by descriptor (Decision 11 of
// docs/PHASE-74 - FORTRAN plan.md), which is a later subtask of Phase 78's.
// It makes the name broken, so its other uses are quiet, and reports
// whether it did.
func (a *analyzer) characterDummy(s *Symbol, pos diag.Position) bool {
	if s.Type.Kind != Character || s.Dummy == 0 {
		return false
	}

	a.notImplForm(pos, "a CHARACTER dummy argument")

	s.broken = true

	return true
}

// substring analyzes X(Lower:Upper) (section 2.2.4): X a CHARACTER variable
// or array element, each bound made INTEGER (a REAL one is truncated, as a
// subscript is). Lower left out is 1. A constant bound must be within X, 1 <=
// Lower <= Upper <= X's length; one that isn't constant is checked only by
// /CHECK=BOUNDS (Phase 85).
func (a *analyzer) substring(e *ast.Substring) Expr {
	var (
		x    Expr
		name string
	)

	switch n := e.X.(type) {
	case *ast.Name:
		if s := a.unit.byName[n.Name]; s != nil && s.Class == Constant {
			if !s.broken {
				a.badUse(s, n.Pos, "have a substring")
			}

			return a.bad(e.Pos)
		}

		x, name = a.name(n, false), n.Name

	case *ast.Apply:
		x, name = a.apply(n), n.Name

	default:
		a.notImplForm(e.X.Position(), formWhat(e.X))

		return a.bad(e.Pos)
	}

	t := x.Type()

	switch x.(type) {
	case *Var, *Elem:
	default:
		if t.Kind != Invalid {
			// A function's value (CHARACTER functions are Phase 79's) or a
			// constant expression.
			a.report("SUBSTR", e.Pos, name, "not a variable or an array element")
		}

		return a.bad(e.Pos)
	}

	if t.Kind != Character {
		a.report("SUBSTR", e.Pos, name, t.String())

		return a.bad(e.Pos)
	}

	lower := Expr(&Const{exprBase: exprBase{IntegerType, e.Pos}, Int: 1})
	if e.Lower != nil {
		lower = a.convert(a.arithmetic(a.value(e.Lower)), IntegerType)
	}

	var upper Expr
	if e.Upper != nil {
		upper = a.convert(a.arithmetic(a.value(e.Upper)), IntegerType)
	}

	if lower.Type().Kind == Invalid || (upper != nil && upper.Type().Kind == Invalid) {
		return a.bad(e.Pos)
	}

	first, firstKnown := constantInt(lower)
	last, lastKnown := t.Size, true

	if upper != nil {
		last, lastKnown = constantInt(upper)
	}

	switch {
	case firstKnown && lastKnown:
		if first < 1 || first > last || last > t.Size {
			a.report("SUBRANGE", e.Pos, first, last, name, t.Size)

			return a.bad(e.Pos)
		}

		return &Substring{exprBase{Type{Character, last - first + 1}, e.Pos}, x, lower, upper}

	case firstKnown && (first < 1 || first > t.Size):
		a.report("SUBBOUND", e.Pos, first, name, t.Size)

		return a.bad(e.Pos)

	case lastKnown && (last < 1 || last > t.Size):
		a.report("SUBBOUND", e.Pos, last, name, t.Size)

		return a.bad(e.Pos)
	}

	return &Substring{exprBase{Type{Character, 0}, e.Pos}, x, lower, upper}
}

// constantInt is e's value if it is an INTEGER constant.
func constantInt(e Expr) (int, bool) {
	if c, ok := e.(*Const); ok && c.T.Kind == Integer {
		return int(c.Int), true
	}

	return 0, false
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
		} else if x := args[i].Expr; x != nil && x.Type().Kind == Character {
			// A character argument is passed by descriptor (Decision 11),
			// a later subtask's.
			a.notImpl(x.Position(), "a character value as an argument")

			args[i].Expr = a.bad(x.Position())
		}
	}

	return args
}

// arithmetic makes e an arithmetic operand: a LOGICAL value is used as
// an INTEGER (section 2.3.1.2: "Logical entities used in an arithmetic
// context are treated as integers"), a character constant is a
// Hollerith constant, NOTIMPL, and any other CHARACTER value is an error
// (CHARNUMBER): FORTRAN converts no character value to a number.
func (a *analyzer) arithmetic(e Expr) Expr {
	switch e.Type().Kind {
	case Logical:
		return a.convert(e, e.Type().integer())
	case Character:
		a.notCharacter(e)

		return a.bad(e.Position())
	}

	return e
}

// notCharacter reports a CHARACTER value where a number or a logical value
// must be: a character constant is a Hollerith constant there (NOTIMPL, Phase
// 82's), and any other is CHARNUMBER.
func (a *analyzer) notCharacter(e Expr) {
	if _, ok := e.(*Const); ok {
		a.notImpl(e.Position(), hollerith)

		return
	}

	a.report("CHARNUMBER", e.Position(), article(e.Type().String()))
}

// convert is e converted to type t: e itself if it already has that type,
// else a Convert (folded, if e is a constant).
func (a *analyzer) convert(e Expr, t Type) Expr {
	if e.Type() == t || e.Type().Kind == Invalid {
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
			a.notCharacter(x)
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
	case ast.Pow, ast.Concat:
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
	}

	x, y := a.value(e.X), a.value(e.Y)
	if x.Type().Kind == Invalid || y.Type().Kind == Invalid {
		return a.bad(e.Pos)
	}

	switch {
	case e.Op.Relational():
		return a.compare(e, x, y)

	case e.Op == ast.And || e.Op == ast.Or || e.Op == ast.Eqv || e.Op == ast.Neqv || e.Op == ast.Xor:
		return a.logical(e, x, y)
	}

	t := a.mixedType(x, y)

	x, y = a.arithmetic(x), a.arithmetic(y)
	if x.Type().Kind == Invalid || y.Type().Kind == Invalid {
		return a.bad(e.Pos)
	}

	return a.fold(&Binary{exprBase{t, e.Pos}, e.Op, a.convert(x, t), a.convert(y, t)})
}

// mixedType is the type an arithmetic operation or comparison on x and y is
// done in: operationType, but for an INTEGER constant from -32768 to 32767,
// which "is treated as an INTEGER*2" (section 2.2.1.1; 2.3.1.2 says so of a
// PARAMETER constant too): beside an INTEGER*2 or a shorter logical that isn't
// constant it is one, so I2*1000 is done in a word, and wraps if the product
// doesn't fit one. Two constants aren't affected, so 30000+30000 is 60000; and
// neither is a constant alone, which is a longword as an argument or an output
// item. The author's choice (Phase 78's survey, answer 2), unconfirmed.
func (a *analyzer) mixedType(x, y Expr) Type {
	t := operationType(x.Type(), y.Type())

	cx, okx := x.(*Const)
	cy, oky := y.(*Const)

	switch {
	case okx == oky:
		return t
	case oky:
		cx, y = cy, x
	}

	if t == IntegerType && cx.T == IntegerType && cx.Int >= math.MinInt16 && cx.Int <= math.MaxInt16 && y.Type().integer().Size == 2 {
		return Type{Integer, 2}
	}

	return t
}

// compare analyzes a relational expression (section 2.3.3): its operands
// are converted to the higher-ranked one's type, and its value is
// LOGICAL. COMPLEX operands take only .EQ. and .NE. Two CHARACTER values
// are compared as they are, whatever their lengths, the shorter as if
// padded with blanks; a CHARACTER value can't be compared with any other
// (COMPARETYPE), but for a character constant, which is a Hollerith
// constant beside a number (NOTIMPL).
func (a *analyzer) compare(e *ast.Binary, x, y Expr) Expr {
	switch cx, cy := x.Type().Kind == Character, y.Type().Kind == Character; {
	case cx && cy:
		kx, okx := x.(*Const)
		ky, oky := y.(*Const)

		if okx && oky {
			return logicalConst(relation(e.Op, compareText(kx.Text, ky.Text)), e.Pos)
		}

		return &Compare{exprBase{LogicalType, e.Pos}, e.Op, x, y}

	case cx || cy:
		c, o := x, y
		if cy {
			c, o = y, x
		}

		if _, ok := c.(*Const); ok {
			a.notImpl(c.Position(), hollerith)
		} else {
			a.report("COMPARETYPE", e.Pos, article(c.Type().String()), article(o.Type().String()))
		}

		return a.bad(e.Pos)
	}

	t := a.mixedType(x, y)

	x, y = a.arithmetic(x), a.arithmetic(y)
	if x.Type().Kind == Invalid || y.Type().Kind == Invalid {
		return a.bad(e.Pos)
	}

	// COMPLEX values are only equal or not: they have no order.
	if t.Kind == Complex && e.Op != ast.EQ && e.Op != ast.NE {
		a.report("BADOPERAND", e.Pos, e.Op.String(), t.String())

		return a.bad(e.Pos)
	}

	return a.fold(&Compare{exprBase{LogicalType, e.Pos}, e.Op, a.convert(x, t), a.convert(y, t)})
}

// logical analyzes .AND., .OR., .EQV., .NEQV., and .XOR. (section 2.3.4):
// on LOGICAL operands the result is LOGICAL; on INTEGER operands, or one of
// each, the operation is done bit by bit and the result is INTEGER (of the
// types' higher rank, and a constant beside a word is a word: mixedType).
// Nothing else may be an operand.
func (a *analyzer) logical(e *ast.Binary, x, y Expr) Expr {
	for _, o := range []Expr{x, y} {
		switch o.Type().Kind {
		case Character:
			a.notCharacter(o)

			return a.bad(e.Pos)

		case Real, Complex:
			a.report("BADOPERAND", e.Pos, e.Op.String(), o.Type().String())

			return a.bad(e.Pos)
		}
	}

	t := logicalType(x.Type(), y.Type())
	if t.Kind == Integer {
		t = a.mixedType(x, y)
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
		return a.convert(c, Type{Logical, c.Type().Size})
	case Character:
		if _, ok := c.(*Const); ok {
			a.notImpl(c.Position(), hollerith)

			break
		}

		fallthrough
	default:
		a.report("NOTLOGICAL", e.Position(), c.Type().String())
	}

	return a.bad(e.Position())
}
