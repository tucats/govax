package parse

import (
	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/lex"
)

// Fortran 90's derived types (the standard's 4.4.1; docs/planning/PHASE-105 -
// Fortran 90 derived types.md, subtask 2). A definition is a statement that
// holds others, as a STRUCTURE declaration is:
//
//	TYPE POINT          <- a TypeDefStmt
//	  SEQUENCE          <-   its Body: a SequenceStmt,
//	  REAL :: X, Y      <-   type statements, its components,
//	END TYPE POINT      <-   and an EndTypeStmt, last
//
// units.go gathers the statements between TYPE and END TYPE into the
// definition's Body, and semantic analysis decides what each may be. Objects
// of the type are declared by TYPE(POINT) [, attributes ::] entities, a type
// statement whose TypeSpec names the type (Derived), and its components are
// reached by P%X, a FieldRef with Percent set (expr.go's referenceAfter).

// parseTypeDef parses a derived type's definition's first statement, TYPE
// [[, access-spec] ::] name (R424): access-spec is PUBLIC or PRIVATE.
func parseTypeDef(p *parser) ast.Stmt {
	s := &ast.TypeDefStmt{StmtBase: p.base()}

	if p.accept(",") {
		t := p.peek()
		if t == nil || t.Kind != lex.Name || (t.Text != "PUBLIC" && t.Text != "PRIVATE") {
			p.syntax("PUBLIC or PRIVATE")
		}

		p.next()

		s.Access, s.AccessPos = t.Text, p.pos(*t)

		p.expect("::")
	} else {
		p.accept("::")
	}

	n := p.name("the type's name")
	s.Name, s.NamePos = n.Name, n.Pos
	p.end()

	return s
}

// parseTypeDecl parses a declaration of a derived type's objects, TYPE
// (type-name) [[, attr]... ::] entities (R501 with R502's TYPE(name)), into a
// TypeStmt whose TypeSpec names the type.
func parseTypeDecl(p *parser) ast.Stmt {
	s := &ast.TypeStmt{StmtBase: p.base(), Colons: p.doubleColon() != nil}
	s.Type = p.derivedSpec()

	p.typeEntities(s, false)

	return s
}

// derivedSpec reads TYPE's parenthesized type name, (name), which is next:
// the TypeSpec of a derived type.
func (p *parser) derivedSpec() ast.TypeSpec {
	pos := p.here()

	p.expect("(")
	n := p.name("a derived type's name")
	p.expect(")")

	return ast.TypeSpec{Derived: n.Name, DerivedPos: n.Pos, Pos: pos}
}

// parseEndType parses END TYPE [name] (R429).
func parseEndType(p *parser) ast.Stmt {
	s := &ast.EndTypeStmt{StmtBase: p.base()}

	if p.atKind(lex.Name) {
		n := p.next()
		s.Name, s.NamePos = n.Text, p.pos(n)
	}

	p.end()

	return s
}

// parseSequence parses SEQUENCE (R426), which has nothing after it.
func parseSequence(p *parser) ast.Stmt {
	s := &ast.SequenceStmt{StmtBase: p.base()}
	p.end()

	return s
}
