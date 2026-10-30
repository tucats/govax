package parse

import (
	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/lex"
)

// The I/O statements: PRINT and WRITE so far (chapter 7).

// parsePrint parses PRINT f [, iolist] (section 7.6), f being *, a
// FORMAT statement's label, or a format expression.
func parsePrint(p *parser) ast.Stmt {
	s := &ast.PrintStmt{StmtBase: p.base(), Format: p.format()}

	if !p.atEnd() {
		p.expect(",")
		s.Items = p.ioList()
	}

	p.end()

	return s
}

// parseWrite parses WRITE (cilist) [iolist] (section 7.6). The control
// list's unit and format may be given by position, the unit first and
// the format second (WRITE (*,*)), or by keyword (WRITE (FMT=*,
// UNIT=*)). The other specifiers (ERR=, IOSTAT=, REC=, NML=) are later
// phases'.
func parseWrite(p *parser) ast.Stmt {
	s := &ast.WriteStmt{StmtBase: p.base()}

	p.expect("(")

	var (
		haveUnit   bool
		positional int // how many specifiers have been given by position
		items      int // how many specifiers have been given
	)

	for {
		t := p.peek()

		if t != nil && t.Kind == lex.Name && p.peekAt(1) != nil && p.peekAt(1).Is("=") {
			p.next()
			p.next()

			switch t.Text {
			case "UNIT":
				if haveUnit {
					p.syntax("a specifier other than UNIT=")
				}

				s.Unit, haveUnit = p.unitSpec(), true

			case "FMT":
				if s.Format != nil {
					p.syntax("a specifier other than FMT=")
				}

				f := p.format()
				s.Format = &f

			case "ERR", "IOSTAT", "REC", "NML":
				p.notImpl("the "+t.Text+"= specifier", p.pos(*t))

			default:
				p.messages.Report("SYNTAX", p.pos(*t), "UNIT=, FMT=, ERR=, IOSTAT=, REC=, or NML=", t.Text+"=")
				panic(bailout{})
			}
		} else {
			// By position: the unit only first, the format only second
			// after a unit given by position.
			switch {
			case items == 0:
				s.Unit, haveUnit = p.unitSpec(), true

				if p.at("'") {
					p.notImpl("direct access, WRITE (u'r)", p.here())
				}

			case items == 1 && positional == 1:
				f := p.format()
				s.Format = &f

			default:
				p.syntax("a specifier with its keyword, such as ERR=")
			}

			positional++
		}

		items++

		if !p.accept(",") {
			break
		}
	}

	if !haveUnit {
		p.syntax("a unit (UNIT=)")
	}

	p.expect(")")

	if !p.atEnd() {
		s.Items = p.ioList()
	}

	p.end()

	return s
}

// unitSpec reads an I/O unit: * or an expression.
func (p *parser) unitSpec() ast.UnitSpec {
	pos := p.here()

	if p.accept("*") {
		return ast.UnitSpec{Star: true, Pos: pos}
	}

	return ast.UnitSpec{Expr: p.expr(), Pos: pos}
}

// format reads an I/O statement's format: * for list-directed I/O, a
// FORMAT statement's label, or an expression that holds or names a
// format.
func (p *parser) format() ast.FormatSpec {
	pos := p.here()

	switch {
	case p.accept("*"):
		return ast.FormatSpec{Star: true, Pos: pos}

	case p.atKind(lex.IntegerConst):
		ref := p.labelRef()

		return ast.FormatSpec{Label: &ref, Pos: pos}
	}

	return ast.FormatSpec{Expr: p.expr(), Pos: pos}
}

// ioList reads an I/O list's items, separated by commas. An item in
// parentheses with an = in it is an implied-DO list, (A(I), I = 1, 10)
// (Phase 81).
func (p *parser) ioList() []ast.Expr {
	var list []ast.Expr

	for {
		if p.impliedDo() {
			p.notImpl("an implied-DO list", p.here())
		}

		list = append(list, p.expr())

		if !p.accept(",") {
			return list
		}
	}
}

// impliedDo reports whether the next token opens an implied-DO list: a
// parenthesis whose contents have an = outside any inner parenthesis.
// (A parenthesized expression, (X+1)*2, has none.)
func (p *parser) impliedDo() bool {
	if !p.at("(") {
		return false
	}

	depth := 0

	for _, t := range p.tokens[p.i:] {
		switch {
		case t.Is("("):
			depth++
		case t.Is(")"):
			if depth--; depth == 0 {
				return false
			}
		case t.Is("=") && depth == 1:
			return true
		}
	}

	return false
}
