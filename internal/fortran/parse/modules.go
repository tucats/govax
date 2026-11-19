package parse

import (
	"github.com/tucats/govax/internal/fortran/ast"
)

// Fortran 90's module statements (docs/planning/PHASE-106 - Fortran 90
// modules.md): USE, and PUBLIC and PRIVATE as statements. MODULE itself is
// f90.go's parseModule, and MODULE PROCEDURE units.go's.
//
//	USE CONSTS                          every public entity of CONSTS
//	USE CONSTS, TWOPI => PI2            the same, PI2 known here as TWOPI
//	USE CONSTS, ONLY: PI, OPERATOR(+)   only those
//	USE CONSTS, ONLY:                   nothing, but the module is used
//	PRIVATE                             the module's default
//	PUBLIC :: SWAP, OPERATOR(.CROSS.)   these names public
//
// What each item means, and whether the module has it, is semantic analysis's
// question; the parser only reads the lists.

// parseUse parses USE module [, rename-list] and USE module, ONLY: [only-list]
// (the standard's R1107 to R1109). A rename list follows a comma; ONLY follows
// one too, then a colon.
func parseUse(p *parser) ast.Stmt {
	s := &ast.UseStmt{StmtBase: p.base()}
	s.ModulePos = p.here()
	s.Module = p.name("the module's name").Name

	if p.atEnd() {
		return s
	}

	p.expect(",")

	if p.atName("ONLY") && p.peekAt(1) != nil && p.peekAt(1).Is(":") {
		p.next()
		p.next()

		s.Only = true

		if p.atEnd() {
			return s
		}
	}

	for {
		s.Items = append(s.Items, p.useItem(s.Only))

		if !p.accept(",") {
			break
		}
	}

	p.end()

	return s
}

// useItem reads one item of a USE statement's list: local => remote, or in an
// ONLY list (only says it is one) a name alone or a generic specification.
func (p *parser) useItem(only bool) *ast.UseItem {
	it := &ast.UseItem{Pos: p.here()}

	if only {
		if g, ok := p.genericSpecItem(); ok {
			it.Generic = g

			return it
		}
	}

	n := p.name("a name")

	if !p.accept("=>") {
		if !only {
			p.syntax("=> and the module's name for it")
		}

		it.Remote = n.Name

		return it
	}

	it.Local = n.Name
	it.Remote = p.name("the module's name for it").Name

	return it
}

// genericSpecItem reads OPERATOR(op) or ASSIGNMENT(=) if one is next, and
// reports whether it was: an item USE's ONLY list and access statements share
// (R1207's generic-spec, but for a generic name, which is a name).
func (p *parser) genericSpecItem() (ast.GenericSpec, bool) {
	g := ast.GenericSpec{Pos: p.here()}

	switch next := p.peekAt(1); {
	case next == nil:
		return g, false

	case p.atName("OPERATOR") && (next.Is("(") || next.Is("(/")):
		p.next()

		g.Kind = ast.GenericOp
		g.Op = p.genericOperator()

		return g, true

	case p.atName("ASSIGNMENT") && next.Is("("):
		p.next()
		p.next()
		p.expect("=")
		p.expect(")")

		g.Kind = ast.GenericAssign

		return g, true
	}

	return g, false
}

// parseAccess parses PUBLIC and PRIVATE (R521, R522): alone, the module's
// default, or with an optional :: and a list of names and generic
// specifications. Inside a derived type's definition PRIVATE stands alone
// (R424), which is this statement with no list.
func parseAccess(p *parser) ast.Stmt {
	s := &ast.AccessStmt{StmtBase: p.base(), Access: p.st.Kind.String()}

	p.accept("::")

	if p.atEnd() {
		return s
	}

	for {
		it := &ast.AccessItem{Pos: p.here()}

		if g, ok := p.genericSpecItem(); ok {
			it.Generic = g
		} else {
			it.Name = p.name("a name").Name
		}

		s.Items = append(s.Items, it)

		if !p.accept(",") {
			break
		}
	}

	p.end()

	return s
}
