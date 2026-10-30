package parse

import (
	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/lex"
)

// FORTRAN's operators bind in this order, loosest first (sections 2.3.1
// to 2.3.4; Decision 6 of docs/PHASE-74 - FORTRAN plan.md):
//
//	.EQV. .NEQV. .XOR.     0
//	.OR.                   1
//	.AND.                  2
//	.NOT.                  3  (unary)
//	.EQ. .NE. .LT. ...     4
//	//                     5
//	+ -                    6  (binary, and the unary signs)
//	* /                    7
//	**                     8  (right to left)
//
// So A + B*C .GT. D .AND. .NOT. E is ((A + (B*C)) .GT. D) .AND. (.NOT. E).
const (
	precNot = 3
	precAdd = 6
	precMul = 7
	precPow = 8
)

// binaryOp is one binary operator: its node's Op and its precedence.
type binaryOp struct {
	op   ast.Op
	prec int
}

// binaryOps are the binary operators, by their tokens' text.
var binaryOps = map[string]binaryOp{
	".EQV.": {ast.Eqv, 0}, ".NEQV.": {ast.Neqv, 0}, ".XOR.": {ast.Xor, 0},
	".OR.":  {ast.Or, 1},
	".AND.": {ast.And, 2},
	".EQ.":  {ast.EQ, 4}, ".NE.": {ast.NE, 4}, ".LT.": {ast.LT, 4},
	".LE.": {ast.LE, 4}, ".GT.": {ast.GT, 4}, ".GE.": {ast.GE, 4},
	"//": {ast.Concat, 5},
	"+":  {ast.Add, precAdd}, "-": {ast.Sub, precAdd},
	"*": {ast.Mul, precMul}, "/": {ast.Div, precMul},
	"**": {ast.Pow, precPow},
}

// expr reads an expression.
func (p *parser) expr() ast.Expr {
	return p.binary(0)
}

// binary reads an expression whose operators all bind at least as
// tightly as minPrec: precedence climbing. It reads one operand, then,
// for as long as the next token is an operator of precedence minPrec or
// more, the operator and its right operand, which is itself an
// expression of operators binding more tightly than that operator (or as
// tightly, for **, which groups right to left: A**B**C is A**(B**C)).
//
// For example, with A + B*C - D: binary(0) reads A, sees + (6), and
// reads its right operand with binary(7), which reads B, sees * (7, not
// below 7), reads C, sees - (6, below 7), and returns B*C. binary(0)
// makes A + B*C, sees - (6), reads D with binary(7), and makes
// (A + B*C) - D: left to right, as FORTRAN wants.
func (p *parser) binary(minPrec int) ast.Expr {
	x := p.unary(minPrec)

	for {
		t := p.peek()
		if t == nil || t.Kind != lex.Operator {
			return x
		}

		op, ok := binaryOps[t.Text]
		if !ok || op.prec < minPrec {
			return x
		}

		p.next()

		next := op.prec + 1
		if op.op == ast.Pow {
			next = op.prec
		}

		x = &ast.Binary{ExprBase: ast.ExprBase{Pos: p.pos(*t)}, Op: op.op, X: x, Y: p.binary(next)}
	}
}

// unary reads an operand that may start with a sign or .NOT.
//
// A sign at the start of an expression applies to the whole term after
// it, so -A*B is -(A*B) and -A**2 is -(A**2), as in algebra. VAX FORTRAN
// also allows a sign right after another operator, and gives it the same
// reach (section 2.3.1): A**-B*C is A**(-(B*C)), and A*-B+C is A*(-B)
// + C. So wherever it is, a sign's operand is an expression of the
// multiplying operators and **.
//
// .NOT.'s operand is everything that binds more tightly than .NOT.:
// .NOT. A .EQ. B is .NOT. (A .EQ. B), and .NOT. A .AND. B is (.NOT. A)
// .AND. B.
func (p *parser) unary(minPrec int) ast.Expr {
	t := p.peek()

	switch {
	case t == nil:
		p.syntax("an expression")

	case t.Is("+"), t.Is("-"):
		p.next()

		op := ast.Add
		if t.Text == "-" {
			op = ast.Sub
		}

		return &ast.Unary{ExprBase: ast.ExprBase{Pos: p.pos(*t)}, Op: op, X: p.binary(precMul)}

	case t.Is(".NOT."):
		p.next()

		return &ast.Unary{ExprBase: ast.ExprBase{Pos: p.pos(*t)}, Op: ast.Not, X: p.binary(precNot + 1)}
	}

	return p.primary()
}

// primary reads a constant, a reference (a name, an array element or
// function reference, a substring, a record's field), a parenthesized
// expression, a complex constant, or one of the built-in functions of
// section 6.1.2.
func (p *parser) primary() ast.Expr {
	t := p.peek()
	if t == nil {
		p.syntax("an expression")
	}

	if c := p.literal(); c != nil {
		return c
	}

	base := ast.ExprBase{Pos: p.pos(*t)}

	switch {
	case t.Kind == lex.Name:
		p.next()

		return p.referenceAfter(*t)

	case t.Is("("):
		// (X) or a complex constant, (1.0, 2.0). The parts of a complex
		// constant are read as expressions, for semantic analysis to say
		// what is wrong with (X, Y): it should be CMPLX(X, Y).
		p.next()

		x := p.expr()
		if p.accept(",") {
			im := p.expr()
			p.expect(")")

			return &ast.ComplexConst{ExprBase: base, Re: x, Im: im}
		}

		p.expect(")")

		return &ast.Paren{ExprBase: base, X: x}

	case t.Is("%"):
		// %LOC, %VAL, %REF, %DESCR (section 6.1.2).
		if n := p.peekAt(1); n != nil && n.Kind == lex.Name && builtinArgs[n.Text] {
			p.next()
			p.next()
			p.expect("(")

			x := p.expr()
			p.expect(")")

			return &ast.BuiltinArg{ExprBase: base, Func: n.Text, X: x}
		}

		p.next()
		p.syntax("LOC, VAL, REF, or DESCR")
	}

	p.syntax("an expression")

	return nil
}

// builtinArgs are the names after % of section 6.1.2's built-in
// functions.
var builtinArgs = map[string]bool{"LOC": true, "VAL": true, "REF": true, "DESCR": true}

// literal reads a literal constant, if the next token is one: an integer,
// real, logical, character, Hollerith, octal, or hexadecimal constant;
// otherwise it returns nil and reads nothing. A sign isn't part of it.
func (p *parser) literal() ast.Expr {
	t := p.peek()
	if t == nil {
		return nil
	}

	base := ast.ExprBase{Pos: p.pos(*t)}

	var e ast.Expr

	switch t.Kind {
	case lex.IntegerConst:
		e = &ast.IntConst{ExprBase: base, Text: t.Text, Radix: t.Radix}
	case lex.RealConst:
		e = &ast.RealConst{ExprBase: base, Text: t.Text}
	case lex.LogicalConst:
		e = &ast.LogicalConst{ExprBase: base, Value: t.Text == ".TRUE."}
	case lex.CharConst:
		e = &ast.CharConst{ExprBase: base, Value: t.Text}
	case lex.HollerithConst:
		e = &ast.HollerithConst{ExprBase: base, Value: t.Text}
	case lex.TypelessConst:
		e = &ast.TypelessConst{ExprBase: base, Digits: t.Text, Radix: t.Radix}
	default:
		return nil
	}

	p.next()

	return e
}

// reference reads a reference to storage: a name, then what may follow
// it (referenceAfter). what says what was expected, for the message if
// there is no name.
func (p *parser) reference(what string) ast.Expr {
	if !p.atKind(lex.Name) {
		p.syntax(what)
	}

	return p.referenceAfter(p.next())
}

// referenceAfter reads what follows the name t in a reference: a
// parenthesized list (an array element's subscripts or a function's
// arguments, A(I, J), making an Apply; or a substring's bounds, C(1:3)),
// a substring's bounds after an element, NAMES(1, 3)(:7), and record
// fields, REC.FIELD, each of which may have subscripts and a substring
// of its own.
func (p *parser) referenceAfter(t lex.Token) ast.Expr {
	base := ast.ExprBase{Pos: p.pos(t)}

	var x ast.Expr = &ast.Name{ExprBase: base, Name: t.Text}

	if p.at("(") {
		if p.substringAhead() {
			return p.substring(x)
		}

		x = &ast.Apply{ExprBase: base, Name: t.Text, Args: p.subscripts()}
	}

	for {
		switch {
		case p.at("(") && p.substringAhead():
			return p.substring(x)

		case p.at(".") && p.peekAt(1) != nil && p.peekAt(1).Kind == lex.Name:
			p.next()

			f := p.next()
			ref := &ast.FieldRef{ExprBase: ast.ExprBase{Pos: p.pos(f)}, X: x, Field: f.Text}

			if p.at("(") && !p.substringAhead() {
				ref.Args = p.subscripts()
			}

			x = ref

		default:
			return x
		}
	}
}

// substringAhead reports whether the parenthesized list that is next has
// a : outside any inner parenthesis: a substring's bounds, not
// subscripts.
func (p *parser) substringAhead() bool {
	depth := 0

	for _, t := range p.tokens[p.i:] {
		switch {
		case t.Is("("):
			depth++
		case t.Is(")"):
			if depth--; depth == 0 {
				return false
			}
		case t.Is(":") && depth == 1:
			return true
		}
	}

	return false
}

// substring reads a substring's bounds after x, ([e1]:[e2]) (section
// 2.2.4).
func (p *parser) substring(x ast.Expr) ast.Expr {
	s := &ast.Substring{ExprBase: ast.ExprBase{Pos: x.Position()}, X: x}

	p.expect("(")

	if !p.at(":") {
		s.Lower = p.expr()
	}

	p.expect(":")

	if !p.at(")") {
		s.Upper = p.expr()
	}

	p.expect(")")

	return s
}

// subscripts reads the parenthesized list after a name in an
// expression: an array element's subscripts or a function's arguments.
// F() is a function reference with no arguments. An argument may be an
// alternate return specifier only in a CALL (stmt.go), where this isn't
// used, and a null argument only there too.
func (p *parser) subscripts() []ast.Expr {
	p.expect("(")

	list := []ast.Expr{}

	if p.accept(")") {
		return list
	}

	for {
		list = append(list, p.expr())

		if !p.accept(",") {
			break
		}
	}

	p.expect(")")

	return list
}
