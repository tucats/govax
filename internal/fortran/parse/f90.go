package parse

import (
	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
	"github.com/tucats/govax/internal/fortran/lex"
)

// Fortran 90's statements (docs/planning/PHASE-99 - Fortran 90.md, subtask 5).
// The lexer recognizes every one. The parser made those not compiled yet a
// PendingStmt, which semantic analysis reports NOTIMPL by name, so a program
// using WHERE heard "the WHERE statement is not implemented yet" where it
// was, not a syntax error, and each later phase gave its statements real
// parsers; since Phase 107's pointer statements every one has its own, and a
// PendingStmt is left only for the semantic NOTIMPLs that still use one. What
// this file parses:
//
//   - MODULE name, a unit's header, so a module is one unit;
//   - END PROGRAM name and the rest, checked against the unit they end
//     (checkEnd);
//   - construct names (StmtBase.Construct), and Phase 101's EXIT, CYCLE,
//     DO with no loop control, and the CASE construct (stmt.go's
//     parseLoopExit, parseDo, parseSelectCase, parseCase, parseEndSelect);
//   - Fortran 90's declarations (Phase 100): a type statement's :: form,
//     with its attributes (attrSpec) and = values (entityValue), the
//     attribute statements with ::, and INTENT, OPTIONAL, TARGET, and
//     POINTER, into the old statements' nodes (decl.go, spec.go), an = value
//     an array constructor too (Phase 102);
//   - ALLOCATE and DEALLOCATE (Phase 104), pointer assignment and NULLIFY
//     (Phase 107);
//   - and the statements that hold others (units.go): CONTAINS's
//     internal procedures (ast.Unit.Contains), an interface block's bodies
//     (ast.InterfaceBlock), each a unit of its own (Phase 103), and a
//     derived type's components, gathered into its TYPE statement's
//     TypeDefStmt (derived.go; Phase 105).

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
// it (SLASHINIT; the HP manual's rule). The value may be an array constructor, an
// array's (Phase 102).
func (p *parser) entityValue(d *ast.Declarator) {
	switch {
	case p.at("/"):
		p.fail("SLASHINIT", p.here())

	case p.accept("="):
		d.ValuePos = p.here()
		d.Value = p.expr()
	}
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

// openUnit is what checkEnd checks a unit's END against: its kind, its
// name, and whether it is inside another.
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

	s.Then = statement(then, p.messages, p.records)

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

// parseAllocate parses Fortran 90's ALLOCATE (object[, object]...[,
// STAT=v]) and DEALLOCATE (name[, name]...[, STAT=v]) (the standard's R622
// and R631; docs/planning/PHASE-104 - Fortran 90 dynamic arrays.md). An
// ALLOCATE object is a name and its bounds, read as an array declarator's
// are, so A(N) and B(0:M, 3) are Declarators, or a name alone, a scalar
// pointer's (docs/planning/PHASE-107 - Fortran 90 pointers.md); whether the
// bounds are right for the object (no *, no :, as many as its rank) is
// semantic analysis's question. A DEALLOCATE object is a name alone. Either
// may be a component instead, HEAD%NEXT, read as a reference (subtask 5). STAT=
// may come only last.
func parseAllocate(p *parser) ast.Stmt {
	s := &ast.AllocateStmt{StmtBase: p.base()}
	allocate := p.st.Kind == lex.Allocate

	p.expect("(")

	for {
		if p.atName("STAT") && p.peekAt(1) != nil && p.peekAt(1).Is("=") {
			p.next()
			p.next()

			s.Stat = p.reference("a variable for STAT=")

			break
		}

		if p.componentObject() {
			start := p.peek()
			x := p.reference("a pointer component")

			f, ok := x.(*ast.FieldRef)
			if !ok {
				p.syntax("a pointer component")
			}

			s.Objects = append(s.Objects, &ast.AllocObject{
				Declarator: &ast.Declarator{Name: ast.DumpExpr(f), Pos: p.pos(*start)}, Component: f,
			})
		} else {
			n := p.name("an allocatable array's name")
			d := &ast.Declarator{Name: n.Name, Pos: n.Pos}

			// A scalar pointer has no bounds, and an array's missing bounds
			// are semantic analysis's to report (ALLOCBOUNDS).
			if allocate && p.at("(") {
				d.Dims = p.dimensions()
			}

			s.Objects = append(s.Objects, &ast.AllocObject{Declarator: d})
		}

		if !p.accept(",") {
			break
		}
	}

	p.expect(")")
	p.end()

	return s
}

// componentObject reports whether the next ALLOCATE or DEALLOCATE object is a
// component, HEAD%NEXT or LIST(I)%HEAD: a name, perhaps a parenthesized list,
// then a % (or a record's period) before the object's end.
func (p *parser) componentObject() bool {
	n := 0
	if t := p.peekAt(n); t == nil || t.Kind != lex.Name {
		return false
	}

	n++

	if t := p.peekAt(n); t != nil && t.Is("(") {
		depth := 0

		for ; p.peekAt(n) != nil; n++ {
			switch t := p.peekAt(n); {
			case t.Is("("):
				depth++
			case t.Is(")"):
				depth--
			}

			if depth == 0 {
				break
			}
		}

		n++
	}

	t := p.peekAt(n)

	return t != nil && (t.Is("%") || t.Is(".")) && p.peekAt(n+1) != nil && p.peekAt(n+1).Kind == lex.Name
}

// parsePointerAssign parses Fortran 90's pointer assignment, pointer =>
// target (the standard's R736; docs/planning/PHASE-107 - Fortran 90
// pointers.md, finding 4): the pointer a reference as the parser reads any,
// so that P, A%NEXT, and A(3) all parse, and the target an expression, so that
// semantic analysis, which says which may stand where, can say why P => 5 or
// P => T + 1 is no target.
func parsePointerAssign(p *parser) ast.Stmt {
	s := &ast.PointerAssignStmt{StmtBase: p.base()}
	s.Pointer = p.reference("a pointer")

	p.expect("=>")

	s.Target = p.expr()

	p.end()

	return s
}

// parseNullify parses Fortran 90's NULLIFY (object[, object]...) (the
// standard's R629), each object a pointer's name or a component.
func parseNullify(p *parser) ast.Stmt {
	s := &ast.NullifyStmt{StmtBase: p.base()}

	p.expect("(")

	for {
		s.Objects = append(s.Objects, p.reference("a pointer"))

		if !p.accept(",") {
			break
		}
	}

	p.expect(")")
	p.end()

	return s
}
