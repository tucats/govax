package parse

import (
	"strconv"

	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
	"github.com/tucats/govax/internal/fortran/lex"
)

// The headers (PROGRAM, FUNCTION, SUBROUTINE, BLOCK DATA, ENTRY) and the
// declarations that give names types and shapes: the type statements,
// DIMENSION, IMPLICIT, and PARAMETER (chapter 4). spec.go has the other
// specification statements.

// parseProgram parses PROGRAM name (section 4.12).
func parseProgram(p *parser) ast.Stmt {
	s := &ast.ProgramStmt{StmtBase: p.base(), Name: p.name("the program's name").Name}
	p.end()

	return s
}

// parseFunction parses [type[*n]] FUNCTION name[*n] [([p[,p]...])]
// (section 6.2.2). The lexer has already found the type keyword, if any
// (the statement's FunctionType), and split off FUNCTION as a keyword
// token after the type's length. A length after the name of a function
// with no type keyword (FUNCTION F*8) gives a TypeSpec of no type, for
// semantic analysis to report. A CHARACTER function may have a length in
// only one of the two places (section 6.2.2.2).
func parseFunction(p *parser) ast.Stmt {
	s := &ast.FunctionStmt{StmtBase: p.base()}

	if k := p.st.FunctionType; k != lex.Unknown {
		s.Type = &ast.TypeSpec{Type: ast.TypeKeywords[k], Pos: s.Pos}

		if p.accept("*") {
			s.Type.Len = p.lengthSpec(s.Type.Type == ast.Character)
		}

		p.expect("FUNCTION")
	}

	s.Name = p.name("the function's name").Name

	if p.at("*") {
		if s.Type == nil {
			s.Type = &ast.TypeSpec{Pos: p.here()}
		}

		if s.Type.Type == ast.Character && s.Type.HasLength() {
			p.fail("TWOLENGTHS", p.here(), s.Name)
		}

		p.next()

		s.Type.Len = p.lengthSpec(s.Type.Type == ast.Character || s.Type.Type == 0)
	}

	s.Params = p.params()
	p.end()

	return s
}

// parseSubroutine parses SUBROUTINE name [([p[,p]...])] (section 6.2.3).
func parseSubroutine(p *parser) ast.Stmt {
	s := &ast.SubroutineStmt{StmtBase: p.base(), Name: p.name("the subroutine's name").Name}
	s.Params = p.params()
	p.end()

	return s
}

// parseBlockData parses BLOCK DATA [name] (section 4.1).
func parseBlockData(p *parser) ast.Stmt {
	s := &ast.BlockDataStmt{StmtBase: p.base()}

	if !p.atEnd() {
		s.Name = p.name("the BLOCK DATA subprogram's name").Name
	}

	p.end()

	return s
}

// parseEntry parses ENTRY name [([p[,p]...])] (section 6.2.4).
func parseEntry(p *parser) ast.Stmt {
	s := &ast.EntryStmt{StmtBase: p.base(), Name: p.name("the entry point's name").Name}
	s.Params = p.params()
	p.end()

	return s
}

// params reads a subprogram's dummy arguments: nothing, (), or a
// parenthesized list of names and *s, an alternate return's places
// (section 6.1.1.5), each a Name "*".
func (p *parser) params() []*ast.Name {
	if p.atEnd() {
		return nil
	}

	p.expect("(")

	list := []*ast.Name{}

	if p.accept(")") {
		return list
	}

	for {
		if t := p.peek(); t != nil && t.Is("*") {
			p.next()
			list = append(list, &ast.Name{ExprBase: ast.ExprBase{Pos: p.pos(*t)}, Name: "*"})
		} else {
			list = append(list, p.name("a dummy argument's name"))
		}

		if !p.accept(",") {
			break
		}
	}

	p.expect(")")

	return list
}

// lengthSpec reads a length after its * (section 4.4): a number of bytes
// (REAL*8, M12*4), or, where character says the length is a CHARACTER
// one, also a parenthesized integer constant expression, *(4+LENGTH), or
// *(*), a passed length.
func (p *parser) lengthSpec(character bool) ast.Len {
	switch {
	case p.atKind(lex.IntegerConst):
		t := p.next()

		n, err := strconv.Atoi(t.Text)
		if err != nil {
			p.syntax("a length")
		}

		return ast.Len{Length: n}

	case character && p.accept("("):
		if p.at("*") && p.peekAt(1) != nil && p.peekAt(1).Is(")") {
			p.next()
			p.next()

			return ast.Len{LenStar: true}
		}

		e := p.expr()
		p.expect(")")

		return ast.Len{LenExpr: e}

	case character:
		p.syntax("a length, (an expression), or (*)")
	}

	p.syntax("a length in bytes")

	return ast.Len{}
}

// parseTypeStmt parses a type declaration (sections 4.4.1 and 4.4.2):
// type[*n] v[*n][/clist/] [,v[*n][/clist/]]..., each v a name or an array
// declarator. CHARACTER's length may be followed by a comma, CHARACTER*8,
// A; and its entities' lengths come after their bounds, A(10)*8, where a
// numeric entity's come before, IVEC*4(10).
func parseTypeStmt(p *parser) ast.Stmt {
	s := &ast.TypeStmt{StmtBase: p.base()}
	s.Type = ast.TypeSpec{Type: ast.TypeKeywords[p.st.Kind], Pos: s.Pos}
	character := s.Type.Type == ast.Character

	if p.accept("*") {
		s.Type.Len = p.lengthSpec(character)

		if character {
			p.accept(",")
		}
	}

	for {
		d := p.declarator(declTyped, character)

		if p.at("/") {
			d.InitPos = p.here()
			d.Init = p.constantList()
		}

		s.Entities = append(s.Entities, d)

		if !p.accept(",") {
			break
		}
	}

	p.end()

	return s
}

// parseDimension parses DIMENSION a(d) [,a(d)]... (section 4.5), and
// VIRTUAL, the same statement: every entity must be an array declarator.
func parseDimension(p *parser) ast.Stmt {
	s := &ast.DimensionStmt{StmtBase: p.base()}

	for {
		s.Entities = append(s.Entities, p.declarator(declArray, false))

		if !p.accept(",") {
			break
		}
	}

	p.end()

	return s
}

// declKind is what a declarator may have.
type declKind int

const (
	declPlain declKind = iota // a name, and bounds if it is an array (COMMON, RECORD)
	declTyped                 // a length too (a type statement)
	declArray                 // bounds it must have (DIMENSION)
)

// declarator reads one entity of a declaration: its name (%FILL in a
// structure's field declaration), its length after a * if kind is
// declTyped, and its bounds. A CHARACTER entity's length comes after its
// bounds, NAMES(100)*32; any other's before them, IVEC*4(10).
func (p *parser) declarator(kind declKind, character bool) *ast.Declarator {
	var d *ast.Declarator

	if t := p.peek(); kind == declTyped && t != nil && t.Is("%") && isName(p.peekAt(1), "FILL") {
		p.next()
		p.next()

		d = &ast.Declarator{Name: "%FILL", Pos: p.pos(*t)}
	} else {
		n := p.name("a name to declare")
		d = &ast.Declarator{Name: n.Name, Pos: n.Pos}
	}

	if kind == declTyped && !character && p.accept("*") {
		d.Len = p.lengthSpec(false)
	}

	switch {
	case p.at("("):
		d.Dims = p.dimensions()
	case kind == declArray:
		p.expect("(")
	}

	if kind == declTyped && character && p.accept("*") {
		d.Len = p.lengthSpec(true)
	}

	return d
}

// dimensions reads an array declarator's parenthesized bounds,
// ([dl:]du [,[dl:]du]...), where the last du may be * (section
// 2.2.3.1). Whether the bounds are constants, or a dummy's adjustable
// bounds, is semantic analysis's question.
func (p *parser) dimensions() []ast.Bound {
	p.expect("(")

	var list []ast.Bound

	for {
		b := p.upperBound()

		if b.Upper != nil && p.accept(":") {
			lower := b.Upper
			b = p.upperBound()
			b.Lower = lower
		}

		list = append(list, b)

		if !p.accept(",") {
			break
		}
	}

	p.expect(")")

	return list
}

// upperBound reads a dimension's upper bound, an expression or *.
func (p *parser) upperBound() ast.Bound {
	if p.accept("*") {
		return ast.Bound{Star: true}
	}

	return ast.Bound{Upper: p.expr()}
}

// parseImplicit parses IMPLICIT typ(a[,a]...)[,typ(a[,a]...)]..., each a
// a letter or a range of them, c1-c2; or IMPLICIT NONE (section 4.8). A
// type's keyword is a name here, without its blanks: DOUBLEPRECISION.
func parseImplicit(p *parser) ast.Stmt {
	s := &ast.ImplicitStmt{StmtBase: p.base()}

	if p.atName("NONE") && p.peekAt(1) == nil {
		p.next()

		s.None = true

		return s
	}

	for {
		r := &ast.ImplicitRule{}

		t := p.peek()
		if t == nil || t.Kind != lex.Name || ast.TypeNames[t.Text] == 0 {
			p.syntax("a data type")
		}

		p.next()

		r.Type = ast.TypeSpec{Type: ast.TypeNames[t.Text], Pos: p.pos(*t)}

		if p.accept("*") {
			r.Type.Len = p.lengthSpec(r.Type.Type == ast.Character)
		}

		p.expect("(")

		for {
			first, pos := p.letter()
			last := first

			if p.accept("-") {
				last, _ = p.letter()

				if last < first {
					p.messages.Report("BADLETTERS", pos, first, last)
					panic(bailout{})
				}
			}

			r.Letters = append(r.Letters, ast.LetterRange{First: first, Last: last, Pos: pos})

			if !p.accept(",") {
				break
			}
		}

		p.expect(")")

		s.Rules = append(s.Rules, r)

		if !p.accept(",") {
			break
		}
	}

	p.end()

	return s
}

// letter reads a letter of an IMPLICIT statement: a name one letter long.
func (p *parser) letter() (byte, diag.Position) {
	t := p.peek()
	if t == nil || t.Kind != lex.Name || len(t.Text) != 1 {
		p.syntax("a letter")
	}

	p.next()

	return t.Text[0], p.pos(*t)
}

// parseParameter parses PARAMETER (p=c[,p=c]...) (section 4.11), and
// appendix A.4's older PARAMETER p=c[,p=c]..., which has no parentheses
// (the lexer has found it: see its oldParameter).
func parseParameter(p *parser) ast.Stmt {
	s := &ast.ParameterStmt{StmtBase: p.base(), Old: !p.at("(")}

	if !s.Old {
		p.next()
	}

	for {
		d := &ast.ParamDef{Name: p.name("a constant's name")}
		p.expect("=")
		d.Value = p.expr()

		s.Defs = append(s.Defs, d)

		if !p.accept(",") {
			break
		}
	}

	if !s.Old {
		p.expect(")")
	}

	p.end()

	return s
}

// constantList reads a list of constants in slashes, /c[,c]/, as a DATA
// statement and a type statement's initial values have them (section
// 4.3): each c a constant or r*c, r a repeat count.
func (p *parser) constantList() []*ast.DataValue {
	p.expect("/")

	var list []*ast.DataValue

	for {
		v := &ast.DataValue{Pos: p.here()}
		v.Value = p.constant()

		if p.accept("*") {
			v.Repeat, v.Value = v.Value, p.constant()
		}

		list = append(list, v)

		if !p.accept(",") {
			break
		}
	}

	p.expect("/")

	return list
}

// constant reads one constant of a constant list: a constant, possibly
// signed, a constant's name, or a complex constant, (c, c). It is not an
// expression: a / here is always the list's end.
func (p *parser) constant() ast.Expr {
	t := p.peek()
	if t == nil {
		p.syntax("a constant")
	}

	base := ast.ExprBase{Pos: p.pos(*t)}

	switch {
	case t.Is("+"), t.Is("-"):
		p.next()

		op := ast.Add
		if t.Text == "-" {
			op = ast.Sub
		}

		return &ast.Unary{ExprBase: base, Op: op, X: p.constant()}

	case t.Is("("):
		p.next()

		re := p.constant()
		p.expect(",")
		im := p.constant()
		p.expect(")")

		return &ast.ComplexConst{ExprBase: base, Re: re, Im: im}

	case t.Kind == lex.Name:
		p.next()

		return &ast.Name{ExprBase: base, Name: t.Text}
	}

	if c := p.literal(); c != nil {
		return c
	}

	p.syntax("a constant")

	return nil
}
