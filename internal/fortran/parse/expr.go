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

// primary reads a constant, a name, an array element or function
// reference, or a parenthesized expression.
func (p *parser) primary() ast.Expr {
	t := p.peek()
	if t == nil {
		p.syntax("an expression")
	}

	base := ast.ExprBase{Pos: p.pos(*t)}

	switch t.Kind {
	case lex.IntegerConst:
		p.next()

		return &ast.IntConst{ExprBase: base, Text: t.Text}

	case lex.RealConst:
		p.next()

		return &ast.RealConst{ExprBase: base, Text: t.Text}

	case lex.LogicalConst:
		p.next()

		return &ast.LogicalConst{ExprBase: base, Value: t.Text == ".TRUE."}

	case lex.CharConst:
		p.next()

		return &ast.CharConst{ExprBase: base, Value: t.Text}

	case lex.HollerithConst:
		p.notImpl("a Hollerith constant", base.Pos)

	case lex.TypelessConst:
		p.notImpl("an octal or hexadecimal constant", base.Pos)

	case lex.Name:
		p.next()

		if p.at("(") {
			return p.apply(*t, base)
		}

		return &ast.Name{ExprBase: base, Name: t.Text}
	}

	switch {
	case t.Is("("):
		p.next()

		x := p.expr()
		if p.at(",") {
			p.notImpl("a COMPLEX constant", base.Pos)
		}

		p.expect(")")

		return &ast.Paren{ExprBase: base, X: x}

	case t.Is("%"):
		// %LOC, %VAL, %REF, %DESCR (section 6.1.2).
		if n := p.peekAt(1); n != nil && n.Kind == lex.Name {
			p.notImpl("%"+n.Text, base.Pos)
		}
	}

	p.syntax("an expression")

	return nil
}

// apply reads the parenthesized list after the name t, making an Apply.
// A second list after the first is a substring of a character array's
// element, A(I)(1:3) (Phase 82).
func (p *parser) apply(t lex.Token, base ast.ExprBase) *ast.Apply {
	a := &ast.Apply{ExprBase: base, Name: t.Text, Args: p.subscripts()}

	if p.at("(") {
		p.notImpl("a character substring", p.here())
	}

	return a
}

// subscripts reads the parenthesized list after a name in an
// expression: an array element's subscripts or a function's arguments.
// F() is a function reference with no arguments.
func (p *parser) subscripts() []ast.Expr {
	p.expect("(")

	list := []ast.Expr{}

	if p.accept(")") {
		return list
	}

	for {
		if p.at(":") {
			p.notImpl("a character substring", p.here())
		}

		list = append(list, p.expr())

		if p.at(":") {
			p.notImpl("a character substring", p.here())
		}

		if !p.accept(",") {
			break
		}
	}

	p.expect(")")

	return list
}
