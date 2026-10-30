package parse

import (
	"strings"

	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
	"github.com/tucats/govax/internal/fortran/lex"
)

// The specification statements other than declarations (decl.go):
// COMMON, EQUIVALENCE, DATA, SAVE and its kin, NAMELIST, RECORD, the
// structure declarations' statements, and DICTIONARY (chapter 4,
// section 1.4.1); and compilation control: OPTIONS, the CDEC$
// directives, and FORMAT.

// slashName reads a name between slashes, /name/, or // or / / for a
// blank one where blank allows it: a common block's, a structure's, or a
// namelist group's name.
func (p *parser) slashName(what string, blank bool) (string, diag.Position) {
	pos := p.here()

	switch {
	case blank && p.accept("//"):
		return "", pos
	case !p.accept("/"):
		p.syntax("'/' and " + what)
	case blank && p.accept("/"):
		return "", pos
	}

	n := p.name(what)
	p.expect("/")

	return n.Name, n.Pos
}

// atSlash reports whether the next token is / or //, which starts a
// slashed name.
func (p *parser) atSlash() bool {
	return p.at("/") || p.at("//")
}

// parseCommon parses COMMON [/[cb]/]nlist[[,]/[cb]/nlist]... (section
// 4.2): the first block, if it has no slashes, is blank common, and so is
// one named // or / /.
func parseCommon(p *parser) ast.Stmt {
	s := &ast.CommonStmt{StmtBase: p.base()}

	for !p.atEnd() {
		b := &ast.CommonBlock{Pos: p.here()}

		if p.atSlash() || len(s.Blocks) > 0 {
			b.Name, b.Pos = p.slashName("a common block's name", true)
		}

		for {
			b.Entities = append(b.Entities, p.declarator(declPlain, false))

			// A comma may come before the next block's name.
			if !p.accept(",") || p.atSlash() {
				break
			}
		}

		s.Blocks = append(s.Blocks, b)
	}

	if len(s.Blocks) == 0 {
		p.syntax("a common block's entities")
	}

	return s
}

// parseEquivalence parses EQUIVALENCE (nlist)[,(nlist)]... (section 4.6),
// each nlist two or more names, array elements, or substrings.
func parseEquivalence(p *parser) ast.Stmt {
	s := &ast.EquivalenceStmt{StmtBase: p.base()}

	for {
		p.expect("(")

		var set []ast.Expr

		for {
			set = append(set, p.reference("a variable, an array element, or a substring"))

			if !p.accept(",") {
				break
			}
		}

		p.expect(")")

		s.Sets = append(s.Sets, set)

		if !p.accept(",") {
			break
		}
	}

	p.end()

	return s
}

// parseData parses DATA nlist/clist/[[,]nlist/clist/]... (section 4.3):
// each nlist names, array elements, substrings, and implied-DO lists.
func parseData(p *parser) ast.Stmt {
	s := &ast.DataStmt{StmtBase: p.base()}

	for {
		set := &ast.DataSet{}

		for {
			set.Names = append(set.Names, p.dataName())

			if !p.accept(",") {
				break
			}
		}

		set.Values = p.constantList()
		s.Sets = append(s.Sets, set)

		p.accept(",")

		if p.atEnd() {
			break
		}
	}

	return s
}

// dataName reads one item of a DATA statement's nlist: a reference, or
// an implied-DO list, (dlist, i = n1, n2[, n3]), whose dlist is items of
// the same kind.
func (p *parser) dataName() ast.Expr {
	if p.impliedDo() {
		return p.impliedDoList(p.dataName)
	}

	return p.reference("a variable, an array element, a substring, or an implied-DO list")
}

// parseAttr parses SAVE, EXTERNAL, INTRINSIC, VOLATILE, AUTOMATIC, and
// STATIC (sections 4.14, 4.7, 4.9, 4.16): a list of names, where SAVE
// and VOLATILE also take common blocks' names in slashes, and SAVE's list
// may be empty. EXTERNAL's *name form is appendix A.6's, under /NOF77
// only, which govax doesn't have (NOF77).
func parseAttr(p *parser) ast.Stmt {
	s := &ast.AttrStmt{StmtBase: p.base()}
	k := p.st.Kind
	commons := k == lex.Save || k == lex.Volatile

	if k == lex.Save && p.atEnd() {
		return s
	}

	for {
		switch t := p.peek(); {
		case commons && p.atSlash():
			name, pos := p.slashName("a common block's name", true)
			s.Items = append(s.Items, &ast.AttrItem{Name: name, Common: true, Pos: pos})

		case k == lex.External && t != nil && t.Is("*"):
			p.next()
			p.fail("NOF77", p.pos(*t), p.name("a subprogram's name").Name)

		default:
			n := p.name("a name")
			s.Items = append(s.Items, &ast.AttrItem{Name: n.Name, Pos: n.Pos})
		}

		if !p.accept(",") {
			break
		}
	}

	p.end()

	return s
}

// parseNamelist parses NAMELIST /group/ list[[,]/group/ list]...
// (section 4.10).
func parseNamelist(p *parser) ast.Stmt {
	s := &ast.NamelistStmt{StmtBase: p.base()}

	for {
		g := &ast.NamelistGroup{}
		g.Name, g.Pos = p.slashName("a group's name", false)

		for {
			g.Items = append(g.Items, p.name("a variable or array"))

			if !p.accept(",") || p.atSlash() {
				break
			}
		}

		s.Groups = append(s.Groups, g)

		if !p.atSlash() {
			break
		}
	}

	p.end()

	return s
}

// parseRecord parses RECORD /structure/ list[,/structure/ list]...
// (section 4.13): each list names and array declarators.
func parseRecord(p *parser) ast.Stmt {
	s := &ast.RecordStmt{StmtBase: p.base()}

	for {
		g := &ast.RecordGroup{}
		g.Structure, g.Pos = p.slashName("a structure's name", false)

		for {
			g.Entities = append(g.Entities, p.declarator(declPlain, false))

			if !p.accept(",") || p.atSlash() {
				break
			}
		}

		s.Groups = append(s.Groups, g)

		if !p.atSlash() {
			break
		}
	}

	p.end()

	return s
}

// parseStructure parses STRUCTURE [/name/] [fields] (section 4.15.1): a
// nested structure's field namelist is names and array declarators. The
// statements of its body are gathered into it by Parse (blocks.go).
func parseStructure(p *parser) ast.Stmt {
	s := &ast.StructureStmt{StmtBase: p.base()}

	if p.atSlash() {
		s.Name, _ = p.slashName("a structure's name", false)
	}

	for !p.atEnd() {
		s.Fields = append(s.Fields, p.declarator(declPlain, false))

		if !p.accept(",") {
			break
		}
	}

	p.end()

	return s
}

// parseUnion parses UNION (section 4.15.3).
func parseUnion(p *parser) ast.Stmt {
	p.end()

	return &ast.UnionStmt{StmtBase: p.base()}
}

// parseMap parses MAP.
func parseMap(p *parser) ast.Stmt {
	p.end()

	return &ast.MapStmt{StmtBase: p.base()}
}

// parseEndBlock parses END STRUCTURE, END UNION, and END MAP.
func parseEndBlock(p *parser) ast.Stmt {
	p.end()

	return &ast.EndBlockStmt{StmtBase: p.base()}
}

// parseDictionary parses DICTIONARY 'cdd-path[/[NO]LIST]' (section
// 1.4.1).
func parseDictionary(p *parser) ast.Stmt {
	if !p.atKind(lex.CharConst) {
		p.syntax("a character constant naming a CDD record")
	}

	s := &ast.DictionaryStmt{StmtBase: p.base(), Path: p.next().Text}
	p.end()

	return s
}

// parseOptions parses OPTIONS /qualifier[/qualifier]... (section 1.4.3),
// each qualifier a name, or name=value, or name=(value[,value]...).
func parseOptions(p *parser) ast.Stmt {
	s := &ast.OptionsStmt{StmtBase: p.base()}

	for {
		p.expect("/")
		s.Qualifiers = append(s.Qualifiers, p.qualifier("a qualifier"))

		if p.atEnd() {
			break
		}
	}

	return s
}

// qualifier reads an OPTIONS qualifier or a PSECT attribute: a name,
// optionally = and a value or a parenthesized list of them. A value is a
// name or a number, kept as written.
func (p *parser) qualifier(what string) *ast.Qualifier {
	n := p.name(what)
	q := &ast.Qualifier{Name: n.Name, Pos: n.Pos}

	if !p.accept("=") {
		return q
	}

	value := func() string {
		if t := p.peek(); t != nil && (t.Kind == lex.Name || t.Kind == lex.IntegerConst) {
			return p.next().Text
		}

		p.syntax("a value")

		return ""
	}

	if !p.accept("(") {
		q.Values = []string{value()}

		return q
	}

	for {
		q.Values = append(q.Values, value())

		if !p.accept(",") {
			break
		}
	}

	p.expect(")")

	return q
}

// parseDirective parses a CDEC$ directive (section 10.3): IDENT 'string',
// TITLE ['string'], SUBTITLE ['string'], or PSECT /cb/ attr[,attr]...
// The lexer has tokenized the whole directive, its name first.
func parseDirective(p *parser) ast.Stmt {
	s := &ast.DirectiveStmt{StmtBase: p.base()}

	t := p.peek()
	if t == nil || t.Kind != lex.Name {
		p.syntax("a directive's name")
	}

	p.next()

	s.Name = t.Text

	switch s.Name {
	case "IDENT", "TITLE", "SUBTITLE":
		if p.atKind(lex.CharConst) || s.Name == "IDENT" {
			if !p.atKind(lex.CharConst) {
				p.syntax("a character constant")
			}

			s.Text, s.HasText = p.next().Text, true
		}

	case "PSECT":
		s.Common, _ = p.slashName("a common block's name", true)

		for {
			s.Attrs = append(s.Attrs, p.qualifier("an attribute"))

			if !p.accept(",") {
				break
			}
		}

	default:
		p.fail("BADDIRECTIVE", p.pos(*t), s.Name)
	}

	p.end()

	return s
}

// parseFormat parses FORMAT (spec) (chapter 8): its text is kept as
// written, from the lexer's Body on, for FTNRTL and Phase 80's check.
func parseFormat(p *parser) ast.Stmt {
	return &ast.FormatStmt{StmtBase: p.base(), Text: strings.TrimSpace(p.st.Source.Text[p.st.Body:])}
}

// parseInclude parses an INCLUDE statement, which never reaches the
// parser: the source pass reads the file it names in its place (section
// 1.4.2). It is here so that every kind of statement has a parser.
func parseInclude(p *parser) ast.Stmt {
	return &ast.BadStmt{StmtBase: p.base()}
}
