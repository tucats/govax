package parse

import (
	"strings"

	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
	"github.com/tucats/govax/internal/fortran/lex"
)

// FORTRAN's operators bind in this order, loosest first (sections 2.3.1
// to 2.3.4; Decision 6 of docs/planning/PHASE-74 - FORTRAN plan.md):
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
//
// Fortran 90's defined operators (R704, R723) bind most and least tightly of
// all: .INVERSE. A ** 2 is (.INVERSE. A)**2, and A + B .CROSS. C .EQV. D is
// (A + B) .CROSS. (C .EQV. D). A binary one is precDefined, below .EQV.; a
// unary one is read with its operand, a primary, before any other operator
// can take it (unary). They are operators only under /F90.
const (
	precDefined = -1
	precNot     = 3
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
	"==": {ast.EQ, 4}, "/=": {ast.NE, 4}, "<": {ast.LT, 4}, // Fortran 90's spellings
	"<=": {ast.LE, 4}, ">": {ast.GT, 4}, ">=": {ast.GE, 4}, // of the same operators
	"//": {ast.Concat, 5},
	"+":  {ast.Add, precAdd}, "-": {ast.Sub, precAdd},
	"*": {ast.Mul, precMul}, "/": {ast.Div, precMul},
	"**": {ast.Pow, precPow},
}

// expr reads an expression.
func (p *parser) expr() ast.Expr {
	return p.binary(precDefined)
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

		if name, n := p.definedBinary(); n > 0 {
			if minPrec > precDefined {
				return x
			}

			p.i += n

			x = &ast.DefinedOp{ExprBase: ast.ExprBase{Pos: p.pos(*t)}, Op: name, X: x, Y: p.binary(precDefined + 1)}

			continue
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

		x = &ast.Binary{ExprBase: ast.ExprBase{Pos: p.pos(*t)}, Op: op.op, X: x, Y: p.binary(next),
			Symbolic: op.op.Relational() && !strings.HasPrefix(t.Text, ".")}
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

	case p.f90() && t.DefinedOperator():
		// A unary defined operator's operand is a primary (R704).
		p.next()

		return &ast.DefinedOp{ExprBase: ast.ExprBase{Pos: p.pos(*t)}, Op: t.Text, X: p.primary()}

	case p.f90() && p.dottedAhead(0):
		// The same, left apart by the lexer after a ), as after an I/O
		// statement's control list: WRITE (*, F) .ROT. V. Where an operand
		// must start no record can come before the periods, so this is no
		// field reference.
		p.next()
		op := "." + p.next().Text + "."
		p.next()

		return &ast.DefinedOp{ExprBase: ast.ExprBase{Pos: p.pos(*t)}, Op: op, X: p.primary()}
	}

	return p.primary()
}

// f90 reports whether the statement is compiled under /F90, where Fortran
// 90's readings of DEC's forms apply.
func (p *parser) f90() bool {
	return p.st.Source != nil && p.st.Source.F90
}

// definedBinary reports whether a binary defined operator is next under /F90,
// returning it (.CROSS.) and how many tokens it is, or 0. The lexer makes one
// token of it where no record field can be (after a constant); after a name or
// a ), where A.B.C may be DEC's record fields, it leaves the period, the name,
// and the period apart, and the parser has read a field already if A is a
// record (fieldNext), so the three here are the operator.
func (p *parser) definedBinary() (string, int) {
	t := p.peek()

	switch {
	case !p.f90():
		return "", 0
	case t.DefinedOperator():
		return t.Text, 1
	case p.dottedAhead(0):
		return "." + p.peekAt(1).Text + ".", 3
	}

	return "", 0
}

// dottedAhead reports whether the tokens from the nth next are a period, a
// name of letters only, and a period: a defined operator the lexer left apart.
func (p *parser) dottedAhead(n int) bool {
	dot, name, end := p.peekAt(n), p.peekAt(n+1), p.peekAt(n+2)
	if dot == nil || !dot.Is(".") || name == nil || name.Kind != lex.Name || end == nil || !end.Is(".") {
		return false
	}

	for _, c := range name.Text {
		if c < 'A' || c > 'Z' {
			return false
		}
	}

	return true
}

// primary reads a constant, a reference (a name, an array element or
// function reference, a substring, a record's field), a parenthesized
// expression, a complex constant, one of the built-in functions of
// section 6.1.2, or a Fortran 90 array constructor.
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

	case t.Is("(/"):
		return p.arrayConstructor()

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
// real, logical, character, Hollerith, RADIX-50, octal, or hexadecimal constant;
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
		e = &ast.CharConst{ExprBase: base, Value: t.Text, Quoted: p.quoted(*t)}
	case lex.HollerithConst:
		e = &ast.HollerithConst{ExprBase: base, Value: t.Text}
	case lex.TypelessConst:
		e = &ast.TypelessConst{ExprBase: base, Digits: t.Text, Radix: t.Radix, Prefixed: t.Prefixed}
	case lex.Radix50Const:
		e = &ast.Radix50Const{ExprBase: base, Value: t.Text}
	default:
		return nil
	}

	p.next()

	if t.KindParam != "" {
		return &ast.KindConst{ExprBase: base, X: e, Kind: t.KindParam}
	}

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
// arguments, A(I, J), making an Apply, whose list may hold Fortran 90's
// subscript triplets, A(1:N:2), or a substring's bounds, C(1:3), which
// only semantic analysis can tell apart), a substring's bounds after
// that, NAMES(1, 3)(:7), and record fields, REC.FIELD, or a derived-type
// object's components, P%X (componentNext), each of which may have subscripts
// and a substring of its own.
//
// A field's parenthesized list is read as a name's is, subscripts that may
// hold triplets: whether R.A(1:3) is a section of an array field or a
// substring of a CHARACTER one is semantic analysis's to say, as for a name
// (docs/planning/PHASE-105 - Fortran 90 derived types.md, finding 6). A list
// after the field's own is a substring, R.NAMES(2)(1:3).
//
// Under /F90 a period after a reference may also start a defined operator, A
// .B. C, and the field reading is taken only where it must be meant
// (fieldNext): after a record, a name a RECORD statement declared, or a field
// or element of one, as the HP manual has it (a field of a record wins); after
// a derived type's object when the name is a component of a type in scope; or
// where no defined operator can be, a period and a name with no period after
// it, which semantic analysis can then report as the field of something that
// isn't a record.
func (p *parser) referenceAfter(t lex.Token) ast.Expr {
	base := ast.ExprBase{Pos: p.pos(t)}
	record := p.records.kind(t.Text)

	var x ast.Expr = &ast.Name{ExprBase: base, Name: t.Text}

	if p.at("(") {
		x = &ast.Apply{ExprBase: base, Name: t.Text, Args: p.subscripts()}
	}

	for {
		switch {
		case p.at("(") && p.substringAhead():
			return p.substring(x)

		case p.fieldNext(record) || p.componentNext():
			percent := p.next().Is("%")

			f := p.next()
			ref := &ast.FieldRef{ExprBase: ast.ExprBase{Pos: p.pos(f)}, X: x, Field: f.Text, Percent: percent}

			if p.at("(") {
				ref.Args = p.subscripts()
			}

			x = ref

		default:
			return x
		}
	}
}

// fieldNext reports whether a record field reference, .FIELD, is next after a
// reference: a period and a name, which under /F90 is a field only if the
// reference is a record's (record), or a derived-type object's and the name a
// component of a type in scope, or no defined operator could start there
// (dottedAhead).
func (p *parser) fieldNext(record recordKind) bool {
	if !p.at(".") || p.peekAt(1) == nil || p.peekAt(1).Kind != lex.Name {
		return false
	}

	switch {
	case !p.f90() || record == structRecord:
		return true
	case record == derivedObject && p.records.component(p.peekAt(1).Text):
		return true
	}

	return !p.dottedAhead(0)
}

// componentNext reports whether a derived-type object's component, %NAME, is
// next after a reference (the standard's R612): % and a name. No built-in
// function, %LOC and the rest, can follow a reference, so after one a % always
// selects a component, under /F90 or not.
func (p *parser) componentNext() bool {
	return p.at("%") && p.peekAt(1) != nil && p.peekAt(1).Kind == lex.Name
}

// substringAhead reports whether the parenthesized list that is next has
// a : outside any inner parenthesis: a substring's bounds, not
// subscripts.
func (p *parser) substringAhead() bool {
	depth := 0

	for _, t := range p.tokens[p.i:] {
		switch {
		case t.Is("("), t.Is("(/"):
			depth++
		case t.Is(")"), t.Is("/)"):
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
// F() is a function reference with no arguments. Something left out between
// commas is a null argument, kept as nil, as in a CALL (stmt.go): F(A,,B) has
// three arguments, the second null, and F(,) two. Whether the name is a
// function or an array isn't known here, so semantic analysis refuses a null
// where the name turns out to be one that can't have it. An alternate return
// specifier is only for a CALL, where this isn't used. An argument may be
// written with its dummy's name, KIND=2 (Fortran 90's keyword argument, an
// ast.KeywordArg), which semantic analysis takes only in an intrinsic
// function's reference so far. An item with a colon is a subscript triplet
// (triplet).
func (p *parser) subscripts() []ast.Expr {
	p.expect("(")

	list := []ast.Expr{}

	if p.accept(")") {
		return list
	}

	for {
		switch {
		case p.atColon():
			list = append(list, p.triplet(nil, p.here()))
		case p.at(",") || p.at(")"):
			list = append(list, nil)
		case p.atKeyword():
			t := p.next()
			p.next()

			list = append(list, &ast.KeywordArg{ExprBase: ast.ExprBase{Pos: p.pos(t)}, Name: t.Text, X: p.expr()})
		default:
			pos := p.here()

			e := p.expr()
			if p.atColon() {
				e = p.triplet(e, pos)
			}

			list = append(list, e)
		}

		if !p.accept(",") {
			break
		}
	}

	p.expect(")")

	return list
}

// triplet reads the rest of a subscript triplet, [l]:[u][:s] (the standard's
// R619), whose lower bound, already read, is lower (nil if it was left out);
// the colon is next. pos is where the triplet starts. The upper bound may be
// left out; a stride, after a second colon, may not. With both left out, A(::2)
// and A(2::2), the lexer has read the two colons as Fortran 90's ::, which here
// is just that.
func (p *parser) triplet(lower ast.Expr, pos diag.Position) ast.Expr {
	t := &ast.Triplet{ExprBase: ast.ExprBase{Pos: pos}, Lower: lower}

	if p.accept("::") {
		t.Stride = p.expr()

		return t
	}

	p.expect(":")

	if !p.at(",") && !p.at(")") && !p.at(":") {
		t.Upper = p.expr()
	}

	if p.accept(":") {
		t.Stride = p.expr()
	}

	return t
}

// atColon reports whether a triplet's first colon is next: a colon, or the
// two of :: (triplet).
func (p *parser) atColon() bool {
	return p.at(":") || p.at("::")
}

// arrayConstructor reads Fortran 90's (/ values /) (the standard's R431 to
// R435): each value an expression or an implied-DO list, whose items are
// values of the same kind, (I, I*I, I = 1, N). The parentheses' own tokens,
// (/ and /), are the lexer's.
func (p *parser) arrayConstructor() ast.Expr {
	c := &ast.ArrayConstructor{ExprBase: ast.ExprBase{Pos: p.here()}}

	p.expect("(/")

	for {
		c.Values = append(c.Values, p.constructorValue())

		if !p.accept(",") {
			break
		}
	}

	p.expect("/)")

	return c
}

// constructorValue reads one value of an array constructor: an implied-DO
// list or an expression.
func (p *parser) constructorValue() ast.Expr {
	if p.impliedDo() {
		return p.impliedDoList(p.constructorValue)
	}

	return p.expr()
}

// quoted reports whether the character constant t was written between quotation
// marks: the first mark at or after its start (past a kind parameter, K_"text") is one.
func (p *parser) quoted(t lex.Token) bool {
	if p.st.Source == nil || t.Offset < 0 || t.Offset >= len(p.st.Source.Text) {
		return false
	}

	text := p.st.Source.Text[t.Offset:]
	mark := strings.IndexAny(text, `'"`)

	return mark >= 0 && text[mark] == '"'
}
