package parse

import (
	"strconv"

	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/lex"
)

// The header and specification statements: PROGRAM, FUNCTION,
// SUBROUTINE, the type declarations, and DIMENSION.

// baseTypes are the data types whose declarations the parser takes so
// far, by the lexer's kind of their keyword.
var baseTypes = map[lex.Kind]ast.BaseType{
	lex.Integer: ast.Integer,
	lex.Real:    ast.Real,
	lex.Logical: ast.Logical,
}

// parseProgram parses PROGRAM name (section 4.12).
func parseProgram(p *parser) ast.Stmt {
	s := &ast.ProgramStmt{StmtBase: p.base(), Name: p.name("the program's name").Name}
	p.end()

	return s
}

// parseFunction parses [type[*n]] FUNCTION name[*n] [([p[,p]...])]
// (section 6.2.2). The lexer has already found the type keyword, if any
// (the statement's FunctionType), and split off FUNCTION as a keyword
// token after the type's length.
func parseFunction(p *parser) ast.Stmt {
	s := &ast.FunctionStmt{StmtBase: p.base()}

	if k := p.st.FunctionType; k != lex.Unknown {
		t, ok := baseTypes[k]
		if !ok {
			p.notImpl(article(k.String())+" "+k.String()+" function", s.Pos)
		}

		s.Type = &ast.TypeSpec{Type: t, Pos: s.Pos}

		if p.accept("*") {
			s.Type.Length = p.length()
		}

		p.expect("FUNCTION")
	}

	s.Name = p.name("the function's name").Name

	if p.at("*") {
		if s.Type == nil {
			p.notImpl("a length after an untyped function's name", p.here())
		}

		p.next()

		s.Type.Length = p.length()
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

// params reads a subprogram's dummy arguments: nothing, (), or a
// parenthesized list of names. A * is an alternate return's dummy
// (Phase 79).
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
		if p.at("*") {
			p.notImpl("an alternate return", p.here())
		}

		list = append(list, p.name("a dummy argument's name"))

		if !p.accept(",") {
			break
		}
	}

	p.expect(")")

	return list
}

// length reads a type length's number, after its *: REAL*8, M12*4.
func (p *parser) length() int {
	if !p.atKind(lex.IntegerConst) {
		p.syntax("a length in bytes")
	}

	t := p.next()

	n, err := strconv.Atoi(t.Text)
	if err != nil {
		p.syntax("a length in bytes")
	}

	return n
}

// parseTypeStmt parses a numeric or logical type declaration, type[*n]
// v[*n] [,v[*n]]... (section 4.4.1), each v a name or an array
// declarator.
func parseTypeStmt(p *parser) ast.Stmt {
	s := &ast.TypeStmt{StmtBase: p.base()}
	s.Type = ast.TypeSpec{Type: baseTypes[p.st.Kind], Pos: s.Pos}

	if p.accept("*") {
		s.Type.Length = p.length()
	}

	s.Entities = p.declarators(false)
	p.end()

	return s
}

// parseDimension parses DIMENSION a(d) [,a(d)]... (section 4.5), where
// every entity must be an array declarator.
func parseDimension(p *parser) ast.Stmt {
	s := &ast.DimensionStmt{StmtBase: p.base(), Entities: p.declarators(true)}
	p.end()

	return s
}

// declarators reads a declaration's list of entities: names, each with
// an optional *n length (in a type statement; DIMENSION has none), then
// dimensions, which needDims makes required. A length goes between an
// array's name and its dimensions: IVEC*4(10).
func (p *parser) declarators(needDims bool) []*ast.Declarator {
	var list []*ast.Declarator

	for {
		n := p.name("a name to declare")
		d := &ast.Declarator{Name: n.Name, Pos: n.Pos}

		if !needDims && p.accept("*") {
			d.Length = p.length()
		}

		switch {
		case p.at("("):
			d.Dims = p.dimensions()
		case needDims:
			p.expect("(")
		}

		if p.at("/") {
			p.notImpl("a type statement's initial values", p.here())
		}

		list = append(list, d)

		if !p.accept(",") {
			return list
		}
	}
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
