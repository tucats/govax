package parse

import (
	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/lex"
)

// Fortran 95's statements (docs/planning/PHASE-112 - Fortran 95.md, subtask
// 2): FORALL's statement and construct. PURE and ELEMENTAL are flags of the
// FUNCTION and SUBROUTINE statements (decl.go), and the construct names on
// ELSEWHERE and END WHERE, and END INTERFACE's generic name, are read where
// those statements are (f90.go, units.go).

// parseForall parses the FORALL statement, FORALL (header) assignment (the HP
// Fortran manual's 4.2.5; Fortran 95's R755). The lexer has lexed the
// assignment as a statement of its own (the Then of the lex.Statement), as a
// WHERE statement's is; it may be an assignment or a pointer assignment
// (R756), nothing else (BADFORALLSTMT).
func parseForall(p *parser) ast.Stmt {
	s := &ast.ForallStmt{StmtBase: p.base(), Header: p.forallHeader()}
	p.end()

	then := p.st.Then
	if then == nil {
		p.syntax("an assignment after the FORALL header")
	}

	switch then.Kind {
	case lex.Assignment, lex.PointerAssignment:
	case lex.Unknown:
		panic(bailout{}) // the lexer reported it
	default:
		p.messages.Report("BADFORALLSTMT", then.Position(then.Offset), article(then.Kind.String())+" "+then.Kind.String()+" statement")
		panic(bailout{})
	}

	s.Then = statement(then, p.messages, p.records)

	if _, bad := s.Then.(*ast.BadStmt); bad {
		panic(bailout{})
	}

	return s
}

// parseForallBlock parses FORALL (header) alone, which starts a FORALL
// construct (R752).
func parseForallBlock(p *parser) ast.Stmt {
	s := &ast.ForallBlockStmt{StmtBase: p.base(), Header: p.forallHeader()}
	p.end()

	return s
}

// parseEndForall parses END FORALL [name] (R758).
func parseEndForall(p *parser) ast.Stmt {
	s := &ast.EndForallStmt{StmtBase: p.base()}
	p.constructAfter(s)
	p.end()

	return s
}

// forallHeader reads a FORALL's parenthesized header (R753): one or more
// index-name = lower : upper [: stride], then perhaps a mask, which must come
// last. An item is an index if it is a name followed by =, which no mask can
// start with (a mask's = would be ==).
func (p *parser) forallHeader() ast.ForallHeader {
	var h ast.ForallHeader

	p.expect("(")

	for {
		if t, eq := p.peek(), p.peekAt(1); t != nil && t.Kind == lex.Name && eq != nil && eq.Is("=") {
			name := p.next()
			p.next()

			x := &ast.ForallIndex{Name: name.Text, Pos: p.pos(name), Lower: p.expr()}
			p.expect(":")
			x.Upper = p.expr()

			if p.accept(":") {
				x.Stride = p.expr()
			}

			h.Indexes = append(h.Indexes, x)

			if p.accept(",") {
				continue
			}

			break
		}

		if len(h.Indexes) == 0 {
			p.syntax("a FORALL index, name = lower : upper")
		}

		h.Mask = p.expr()

		break
	}

	p.expect(")")

	return h
}
