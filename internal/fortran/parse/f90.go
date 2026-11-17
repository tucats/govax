package parse

import (
	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
	"github.com/tucats/govax/internal/fortran/lex"
)

// Fortran 90's statements (docs/planning/PHASE-99 - Fortran 90.md, subtask 5).
// The lexer recognizes every one; the parser makes most of them a
// PendingStmt, which semantic analysis reports NOTIMPL by name, so a program
// using WHERE hears "the WHERE statement is not implemented yet" where it
// is, not a syntax error. Each later phase gives its statements
// real parsers. What is parsed now:
//
//   - MODULE name, a unit's header, so a module is one unit;
//   - END PROGRAM name and the rest, checked against the unit they end
//     (checkEnd);
//   - construct names (StmtBase.Construct), and Phase 101's EXIT, CYCLE,
//     DO with no loop control, and the CASE construct (stmt.go's
//     parseLoopExit, parseDo, parseSelectCase, parseCase, parseEndSelect);
//   - Fortran 90's declarations (Phase 100): a type statement's :: form,
//     with its attributes (attrSpec) and = values (entityValue), the
//     attribute statements with ::, and INTENT, OPTIONAL, and TARGET, into
//     the old statements' nodes (decl.go, spec.go); a kind or length
//     selector (REAL(8) X) is still a PendingStmt naming the form;
//   - and the statements that hold others: CONTAINS's procedures, an
//     interface block's bodies, a derived type's components. Left in the
//     unit's list, they would be read as the unit's own declarations, so
//     the parser gathers them into the PendingStmt that holds them
//     (nesting).

// parsePending makes a statement a PendingStmt, reported NOTIMPL by its kind.
func parsePending(p *parser) ast.Stmt {
	return &ast.PendingStmt{StmtBase: p.base()}
}

// pending makes the statement a PendingStmt naming what, at t (the
// statement's start for nil): a Fortran 90 form of an old statement.
func (p *parser) pending(what string, t *lex.Token) ast.Stmt {
	s := &ast.PendingStmt{StmtBase: p.base(), What: what}
	if t != nil {
		s.WhatPos = p.pos(*t)
	}

	return s
}

// doubleColon is the statement's :: token at level 0, or nil: Fortran 90's
// form of a declaration (INTEGER, SAVE :: X, DIMENSION :: A(10)).
func (p *parser) doubleColon() *lex.Token {
	depth := 0

	for i := range p.tokens {
		switch t := &p.tokens[i]; {
		case t.Is("("), t.Is("(/"):
			depth++
		case t.Is(")"), t.Is("/)"):
			depth--
		case t.Is("::") && depth == 0:
			return t
		}
	}

	return nil
}

// typeSelector reads the Fortran 90 kind or length selector after a type
// keyword, if one is next, into spec (the standard's R502 and R506): for
// every type but CHARACTER, ([KIND=] expr); for CHARACTER, a length, a kind,
// or both: (len), (LEN=len), (len, [KIND=]kind), (LEN=len, KIND=kind),
// (KIND=kind, LEN=len), and (KIND=kind), each len an expression or *. A kind
// unnamed must follow a length unnamed. Whether the kind is one the type has
// is semantic analysis's question. VAX FORTRAN's type statements never have a
// selector: a length follows a *.
func (p *parser) typeSelector(spec *ast.TypeSpec) {
	if !p.accept("(") {
		return
	}

	spec.Selector = true

	if spec.Type != ast.Character {
		if p.atName("KIND") && p.peekAt(1) != nil && p.peekAt(1).Is("=") {
			p.next()
			p.next()
		}

		spec.KindPos = p.here()
		spec.Kind = p.expr()
		p.expect(")")

		return
	}

	positional := true // every item so far without its keyword

	for i := 0; ; i++ {
		word := ""

		if t, eq := p.peek(), p.peekAt(1); t != nil && t.Kind == lex.Name && eq != nil && eq.Is("=") &&
			(t.Text == "LEN" || t.Text == "KIND") {
			word = t.Text

			p.next()
			p.next()
		}

		switch {
		case word == "" && i == 0, word == "LEN" && !spec.HasLength():
			spec.Len = p.selectorLength()

		case word == "" && i == 1 && positional, word == "KIND" && spec.Kind == nil:
			spec.KindPos = p.here()
			spec.Kind = p.expr()

		default:
			p.syntax("LEN= or KIND=")
		}

		positional = positional && word == ""

		if i == 1 || !p.accept(",") {
			break
		}
	}

	p.expect(")")
}

// selectorLength reads a CHARACTER selector's length: * or an expression.
func (p *parser) selectorLength() ast.Len {
	if p.at("*") && p.peekAt(1) != nil && (p.peekAt(1).Is(")") || p.peekAt(1).Is(",")) {
		p.next()

		return ast.Len{LenStar: true}
	}

	return ast.Len{LenExpr: p.expr()}
}

// attributes are the attribute keywords a Fortran 90 type declaration may
// list before its :: (the standard's R503, and DEC's VOLATILE, AUTOMATIC, and
// STATIC, as the HP manual allows them). Which may go together is semantic
// analysis's question (sema/attrtable.go).
var attributes = map[string]bool{
	"PARAMETER": true, "DIMENSION": true, "SAVE": true, "EXTERNAL": true, "INTRINSIC": true,
	"VOLATILE": true, "AUTOMATIC": true, "STATIC": true, "INTENT": true, "OPTIONAL": true,
	"TARGET": true, "POINTER": true, "ALLOCATABLE": true, "PUBLIC": true, "PRIVATE": true,
}

// attrSpec reads one attribute of a type declaration's list: a keyword,
// DIMENSION with its array specification, or INTENT with its spec.
func (p *parser) attrSpec() *ast.AttrSpec {
	n := p.name("an attribute")
	a := &ast.AttrSpec{Name: n.Name, Pos: n.Pos}

	switch {
	case !attributes[n.Name]:
		p.fail("BADATTR", n.Pos, n.Name)
	case n.Name == "DIMENSION":
		a.Dims = p.dimensions()
	case n.Name == "INTENT":
		a.Intent = p.intentSpec()
	}

	return a
}

// intentSpec reads INTENT's parenthesized spec, (IN), (OUT), or (INOUT),
// which free form may write IN OUT (the standard's R512), and returns IN,
// OUT, or INOUT.
func (p *parser) intentSpec() string {
	p.expect("(")

	t := p.peek()
	if t == nil || t.Kind != lex.Name || (t.Text != "IN" && t.Text != "OUT" && t.Text != "INOUT") {
		p.syntax("IN, OUT, or INOUT")
	}

	p.next()

	spec := t.Text
	if spec == "IN" && p.atName("OUT") {
		p.next()

		spec = "INOUT"
	}

	p.expect(")")

	return spec
}

// entityValue reads what may follow an entity of a Fortran 90 type
// declaration: = and its initialization expression (R504). DEC's initial
// values in slashes are VAX FORTRAN's form, without ::, and not allowed with
// it (SLASHINIT; the HP manual's rule). An array constructor as the value is
// Phase 102's.
func (p *parser) entityValue(d *ast.Declarator) ast.Stmt {
	switch {
	case p.at("/"):
		p.fail("SLASHINIT", p.here())

	case p.accept("="):
		if p.at("(/") {
			return p.pending("an array constructor", p.peek())
		}

		d.ValuePos = p.here()
		d.Value = p.expr()
	}

	return nil
}

// parseModule parses Fortran 90's MODULE name (the standard's R1105).
func parseModule(p *parser) ast.Stmt {
	s := &ast.ModuleStmt{StmtBase: p.base(), Name: p.name("the module's name").Name}
	p.end()

	return s
}

// unitWords are what END's word is shown as: BLOCKDATA as two words.
var unitWords = map[string]string{"BLOCKDATA": "BLOCK DATA"}

// parseEnd parses END (section 5.5), and Fortran 90's END PROGRAM [name],
// END SUBROUTINE [name], END FUNCTION [name], END BLOCK DATA [name], and END
// MODULE [name], whose word the lexer made a Keyword token.
func parseEnd(p *parser) ast.Stmt {
	s := &ast.EndStmt{StmtBase: p.base()}

	if t := p.peek(); t != nil && t.Kind == lex.Keyword {
		p.next()

		s.Unit = t.Text
		if w, ok := unitWords[t.Text]; ok {
			s.Unit = w
		}

		if p.atKind(lex.Name) {
			n := p.next()
			s.Name, s.NamePos = n.Text, p.pos(n)
		}
	}

	p.end()

	return s
}

// constructAfter reads the construct name a statement may end with (END DO
// OUTER, ELSE OUTER), if there is one, into s's base.
func (p *parser) constructAfter(s ast.Stmt) {
	if p.atKind(lex.Name) && p.peekAt(1) == nil {
		t := p.next()
		b := s.Base()
		b.Construct, b.ConstructPos = t.Text, p.pos(t)
	}
}

// unitEnds is the word END needs to name each kind of unit.
var unitEnds = map[ast.UnitKind]string{
	ast.MainProgram:    "PROGRAM",
	ast.FunctionUnit:   "FUNCTION",
	ast.SubroutineUnit: "SUBROUTINE",
	ast.BlockDataUnit:  "BLOCK DATA",
	ast.ModuleUnit:     "MODULE",
}

// openUnit is a unit, or a procedure inside one, whose END hasn't come yet:
// what checkEnd checks an END against.
type openUnit struct {
	kind   ast.UnitKind
	name   string
	header ast.Stmt // nil for a main program without a PROGRAM statement
	inner  bool     // inside another unit: after CONTAINS, or in an interface block
}

// unitOf is the open unit header starts.
func unitOf(header ast.Stmt, inner bool) openUnit {
	u := &ast.Unit{Header: header}

	return openUnit{kind: u.Kind(), name: u.Name(), header: header, inner: inner}
}

// what names the unit, for messages: "the SUBROUTINE S", "the main program".
func (o openUnit) what() string {
	switch {
	case o.kind == ast.MainProgram && o.name != "":
		return "the main program " + o.name
	case o.kind == ast.MainProgram:
		return "the main program"
	case o.name != "":
		return "the " + o.kind.String() + " " + o.name
	}

	return "the " + o.kind.String()
}

// checkEnd checks the END statement s against the unit it ends (the
// standard's R1103 and the rest): the word after END must be the unit's
// kind's, the name after it the unit's own, and a procedure inside another
// must say which it is (END SUBROUTINE). A unit whose header had an error
// isn't checked: its kind and name may be wrong.
func checkEnd(s ast.Stmt, o openUnit, messages *diag.List) {
	end, ok := s.(*ast.EndStmt)
	if !ok {
		return
	}

	switch o.header.(type) {
	case *ast.BadStmt, *ast.PendingStmt:
		return
	}

	word := unitEnds[o.kind]

	switch {
	case end.Unit != "" && end.Unit != word:
		messages.Report("ENDKIND", end.Pos, end.Unit, o.what())
	case end.Unit == "" && o.inner && (o.kind == ast.FunctionUnit || o.kind == ast.SubroutineUnit):
		messages.Report("ENDNOKIND", end.Pos, o.what(), word)
	case end.Name == "":
	case o.name == "" || o.header == nil:
		messages.Report("ENDNONAME", end.NamePos, end.Name, o.what())
	case end.Name != o.name:
		messages.Report("ENDNAME", end.NamePos, end.Name, o.what())
	}
}

// nesting gathers the statements a Fortran 90 statement holds into its
// PendingStmt's Inner, instead of the unit's body: a CONTAINS statement's
// procedures, up to its unit's END; an interface block's bodies, up to END
// INTERFACE; a derived type's components, up to END TYPE. One may hold
// another (a module procedure's own CONTAINS, an interface block in an
// interface body), so they are a stack, innermost last.
type nesting struct {
	open []*nest

	// units are the procedures open inside the nests (after CONTAINS, or
	// interface bodies), innermost last, for END's checks.
	units []openUnit
}

// nest is one open gathering statement.
type nest struct {
	stmt *ast.PendingStmt
	kind lex.Kind // Contains, Interface, or TypeDef

	// depth is how many of units belong to this nest: procedures opened
	// inside it whose END hasn't come.
	depth int
}

// active reports whether a nest is open.
func (n *nesting) active() bool {
	return len(n.open) > 0
}

// opens reports whether s starts a nest, and starts it: a CONTAINS, an
// interface block, a derived type's definition. A CONTAINS in a unit or
// procedure gathers up to that unit's END. Only a PendingStmt can.
func (n *nesting) opens(s ast.Stmt) bool {
	ps, ok := s.(*ast.PendingStmt)
	if !ok {
		return false
	}

	switch k := ps.Kind; k {
	case lex.Contains, lex.Interface, lex.TypeDef:
		n.open = append(n.open, &nest{stmt: ps, kind: k})

		return true
	}

	return false
}

// take puts s, a statement inside the innermost nest, into it, closing nests
// as their ends come, and checking ENDs. It reports false for the statement
// that ends the outermost CONTAINS nest, the unit's own END, which the caller
// places in the unit's body as usual.
func (n *nesting) take(s ast.Stmt, ls *lex.Statement, messages *diag.List) bool {
	top := n.open[len(n.open)-1]

	switch {
	case isHeader(ls.Kind) && ls.Kind != lex.Module:
		// A procedure inside: after CONTAINS, or an interface body.
		n.units = append(n.units, unitOf(s, true))
		top.depth++

	case ls.Kind == lex.End && top.depth > 0:
		checkEnd(s, n.units[len(n.units)-1], messages)
		n.units = n.units[:len(n.units)-1]
		top.depth--

	case ls.Kind == lex.End:
		// The END of the unit the CONTAINS is in: the nest closes, and the
		// END belongs to what holds the CONTAINS. (An interface block or a
		// type definition an END reaches was never closed; its statement's
		// NOTIMPL will fail the compilation anyway.)
		n.open = n.open[:len(n.open)-1]

		if !n.active() {
			return false
		}

		return n.take(s, ls, messages)

	case ls.Kind == lex.EndInterface && top.kind == lex.Interface && top.depth == 0,
		ls.Kind == lex.EndType && top.kind == lex.TypeDef && top.depth == 0:
		top.stmt.Inner = append(top.stmt.Inner, s)
		n.open = n.open[:len(n.open)-1]

		return true
	}

	top.stmt.Inner = append(top.stmt.Inner, s)
	n.opens(s)

	return true
}

// closeAll closes every nest, at a unit's end or the file's: their ENDs
// never came, which the unit's NOEND, or the parse of what follows, reports.
func (n *nesting) closeAll() {
	n.open, n.units = nil, nil
}

// kindBeforeLetters reports whether the parenthesis next in an IMPLICIT
// statement, after a type keyword, is Fortran 90's kind selector rather
// than the letters: a parenthesis follows the one it closes (IMPLICIT REAL(8)
// (A-H), IMPLICIT REAL (A-H) being VAX FORTRAN's).
func (p *parser) kindBeforeLetters() bool {
	depth := 0

	for i := p.i; i < len(p.tokens); i++ {
		switch t := &p.tokens[i]; {
		case t.Is("("):
			depth++
		case t.Is(")"):
			if depth--; depth == 0 {
				next := p.peekAt(i + 1 - p.i)

				return next != nil && next.Is("(")
			}
		}
	}

	return false
}

// parseWhere parses the WHERE statement, WHERE (mask) assignment (Fortran
// 90's R739). The lexer has lexed the assignment as a statement of its own
// (the Then of the lex.Statement), as a logical IF's statement is, and
// nothing but an assignment may be there (BADWHERESTMT).
func parseWhere(p *parser) ast.Stmt {
	s := &ast.WhereStmt{StmtBase: p.base(), Mask: p.condition()}
	p.end()

	then := p.st.Then
	if then == nil {
		p.syntax("an assignment after the mask")
	}

	switch then.Kind {
	case lex.Assignment:
	case lex.Unknown:
		panic(bailout{}) // the lexer reported it
	default:
		p.messages.Report("BADWHERESTMT", then.Position(then.Offset), article(then.Kind.String())+" "+then.Kind.String()+" statement")
		panic(bailout{})
	}

	s.Then = statement(then, p.messages)

	if _, bad := s.Then.(*ast.BadStmt); bad {
		panic(bailout{})
	}

	return s
}

// parseWhereBlock parses WHERE (mask) alone, which starts a WHERE construct
// (R741).
func parseWhereBlock(p *parser) ast.Stmt {
	s := &ast.WhereBlockStmt{StmtBase: p.base(), Mask: p.condition()}
	p.end()

	return s
}

// parseElseWhere parses ELSEWHERE (R743), and Fortran 95's ELSEWHERE
// (mask), for semantic analysis to refuse.
func parseElseWhere(p *parser) ast.Stmt {
	s := &ast.ElseWhereStmt{StmtBase: p.base()}

	if p.at("(") {
		s.Mask = p.condition()
	}

	p.end()

	return s
}

// parseEndWhere parses END WHERE (R744).
func parseEndWhere(p *parser) ast.Stmt {
	s := &ast.EndWhereStmt{StmtBase: p.base()}
	p.end()

	return s
}
