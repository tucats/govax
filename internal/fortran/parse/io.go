package parse

import (
	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/lex"
)

// The I/O statements (chapters 7 and 9, appendix A). They share one
// grammar, the control list (section 7.1.1): a parenthesized list of
// specifiers, each keyword=value, where the first, the unit (or an
// internal file), may leave out its UNIT= and the second, the format or
// namelist, its FMT= or NML= if the first did. A direct access record
// may be given as u'r, right after a unit without its keyword. Which
// keywords a statement takes is the specifiers table's.

// specKind is what a specifier's keyword is followed by.
type specKind int

const (
	specValue specKind = iota // =, and an expression
	specLabel                 // =, and a statement label (ERR=, END=)
	specNone                  // nothing: OPEN's READONLY
	specKeys                  // =, and a parenthesized list of keys (OPEN's KEY=)
)

// The keywords the control lists take beside UNIT=, FMT=, NML=, and REC=,
// which every statement that has them takes out of its list, by the
// statement.
var (
	transferSpecs = map[string]specKind{"ERR": specLabel, "END": specLabel, "IOSTAT": specValue}
	keyedSpecs    = map[string]specKind{
		"KEY": specValue, "KEYEQ": specValue, "KEYGE": specValue, "KEYGT": specValue,
		"KEYLE": specValue, "KEYLT": specValue, "KEYNXT": specValue, "KEYNXTNE": specValue,
		"KEYID": specValue,
	}
	errorSpecs = map[string]specKind{"ERR": specLabel, "IOSTAT": specValue}
	openSpecs  = map[string]specKind{
		"ACCESS": specValue, "ASSOCIATEVARIABLE": specValue, "BLANK": specValue,
		"BLOCKSIZE": specValue, "BUFFERCOUNT": specValue, "CARRIAGECONTROL": specValue,
		"DEFAULTFILE": specValue, "DISP": specValue, "DISPOSE": specValue, "ERR": specLabel,
		"EXTENDSIZE": specValue, "FILE": specValue, "FORM": specValue, "INITIALSIZE": specValue,
		"IOSTAT": specValue, "KEY": specKeys, "MAXREC": specValue, "NAME": specValue,
		"NOSPANBLOCKS": specNone, "ORGANIZATION": specValue, "READONLY": specNone,
		"RECL": specValue, "RECORDSIZE": specValue, "RECORDTYPE": specValue, "SHARED": specNone,
		"STATUS": specValue, "TYPE": specValue, "USEROPEN": specValue,
	}
	closeSpecs   = map[string]specKind{"STATUS": specValue, "DISPOSE": specValue, "DISP": specValue, "ERR": specLabel, "IOSTAT": specValue}
	inquireSpecs = map[string]specKind{
		"FILE": specValue, "DEFAULTFILE": specValue, "ACCESS": specValue, "BLANK": specValue,
		"CARRIAGECONTROL": specValue, "DIRECT": specValue, "ERR": specLabel, "EXIST": specValue,
		"FORM": specValue, "FORMATTED": specValue, "IOSTAT": specValue, "KEYED": specValue,
		"NAME": specValue, "NAMED": specValue, "NEXTREC": specValue, "NUMBER": specValue,
		"OPENED": specValue, "ORGANIZATION": specValue, "RECL": specValue,
		"RECORDTYPE": specValue, "SEQUENTIAL": specValue, "UNFORMATTED": specValue,
	}
)

// controlList is what a control list may hold.
type controlList struct {
	specs  map[string]specKind // its keyword specifiers
	format bool                // FMT= and NML=, and a format by position
	rec    bool                // REC=, and u'r
	keyed  bool                // the keyed READ's KEY= forms

	// needsUnit is set where the list must give a unit: everywhere but
	// INQUIRE, which may ask by FILE=, and OPEN, whose missing UNIT= is
	// a run-time matter for FTNRTL (section 9.1.27).
	needsUnit bool
}

// controls is each statement's control list.
var controls = map[lex.Kind]controlList{
	lex.Read:      {specs: transferSpecs, format: true, rec: true, keyed: true, needsUnit: true},
	lex.Write:     {specs: errorSpecs, format: true, rec: true, needsUnit: true},
	lex.Rewrite:   {specs: errorSpecs, format: true, needsUnit: true},
	lex.Open:      {specs: openSpecs},
	lex.Close:     {specs: closeSpecs, needsUnit: true},
	lex.Inquire:   {specs: inquireSpecs},
	lex.Rewind:    {specs: errorSpecs, needsUnit: true},
	lex.Backspace: {specs: errorSpecs, needsUnit: true},
	lex.EndFile:   {specs: errorSpecs, needsUnit: true},
	lex.Unlock:    {specs: errorSpecs, needsUnit: true},
	lex.Delete:    {specs: errorSpecs, rec: true, needsUnit: true},
	lex.Find:      {specs: errorSpecs, rec: true, needsUnit: true},
}

// control is a control list, read: the parts the statements take out of
// it, and the rest.
type control struct {
	unit     *ast.UnitSpec
	format   *ast.FormatSpec
	namelist *ast.Name
	rec      ast.Expr
	specs    []*ast.IOSpec
}

// controlList reads a statement's parenthesized control list.
func (p *parser) controlList() *control {
	cl := controls[p.st.Kind]
	c := &control{}
	seen := map[string]bool{}
	positional := 0 // specifiers given without a keyword, in order from the first

	p.expect("(")

	for i := 0; ; i++ {
		t := p.peek()

		if kind, ok := cl.specs[tokenText(t)]; ok && kind == specNone && t.Kind == lex.Name {
			// OPEN's READONLY, SHARED, and NOSPANBLOCKS take no value.
			p.next()
			p.once(seen, t)

			if p.at("=") {
				p.syntax("',' or ')'")
			}

			c.specs = append(c.specs, &ast.IOSpec{Keyword: t.Text, Pos: p.pos(*t)})
		} else if p.atKeyword() {
			p.next()
			p.next()
			p.once(seen, t)

			switch kw := t.Text; {
			case kw == "UNIT":
				u := p.unitSpec()
				c.unit = &u

			case kw == "FMT" && cl.format:
				f := p.format()
				c.format = &f

			case kw == "NML" && cl.format:
				c.namelist = p.name("a namelist group's name")

			case kw == "REC" && cl.rec:
				c.rec = p.expr()

			case cl.specs[kw] == specKeys:
				c.specs = append(c.specs, &ast.IOSpec{Keyword: kw, Value: p.keyList(), Pos: p.pos(*t)})

			default:
				kind, ok := cl.specs[kw]
				if !ok && cl.keyed {
					kind, ok = keyedSpecs[kw]
				}

				if !ok {
					p.fail("BADSPECIFIER", p.pos(*t), kw, p.st.Kind.String())
				}

				spec := &ast.IOSpec{Keyword: kw, Pos: p.pos(*t)}

				if kind == specLabel {
					l := p.labelRef()
					spec.Label = &l
				} else {
					spec.Value = p.expr()
				}

				c.specs = append(c.specs, spec)
			}
		} else {
			// By position: the unit only first, and the format (or
			// namelist) only second, after a unit given by position.
			switch {
			case i == 0:
				u := p.unitSpec()
				c.unit = &u
				seen["UNIT"] = true

				if cl.rec && p.accept("'") {
					c.rec = p.expr()
					seen["REC"] = true
				}

			case i == 1 && positional == 1 && cl.format:
				f := p.format()
				c.format = &f
				seen["FMT"] = true

			default:
				p.syntax("a specifier with its keyword, such as ERR=")
			}

			positional++
		}

		if !p.accept(",") {
			break
		}
	}

	if c.unit == nil && cl.needsUnit {
		p.syntax("a unit (UNIT=)")
	}

	p.expect(")")

	return c
}

// tokenText is a token's text, or "" for none.
func tokenText(t *lex.Token) string {
	if t == nil {
		return ""
	}

	return t.Text
}

// once reports a specifier given twice (DUPSPECIFIER), noting t's.
func (p *parser) once(seen map[string]bool, t *lex.Token) {
	if seen[t.Text] {
		p.fail("DUPSPECIFIER", p.pos(*t), t.Text+"=")
	}

	seen[t.Text] = true
}

// keyList reads an OPEN statement's KEY=(kspec[,kspec]...), each kspec
// e1:e2[:dt[:dr]] (section 9.1.15).
func (p *parser) keyList() ast.Expr {
	l := &ast.KeyList{ExprBase: ast.ExprBase{Pos: p.here()}}

	p.expect("(")

	for {
		k := &ast.KeyField{Pos: p.here(), Start: p.expr()}
		p.expect(":")
		k.End = p.expr()

		if p.accept(":") {
			k.Type = p.name("INTEGER or CHARACTER").Name

			if p.accept(":") {
				k.Dir = p.name("ASCENDING or DESCENDING").Name
			}
		}

		l.Keys = append(l.Keys, k)

		if !p.accept(",") {
			break
		}
	}

	p.expect(")")

	return l
}

// parsePrint parses PRINT, TYPE, and ACCEPT, keyword f [, iolist]
// (sections 7.5, 7.6), f being *, a FORMAT statement's label, a format
// expression, or a namelist group's name.
func parsePrint(p *parser) ast.Stmt {
	s := &ast.PrintStmt{StmtBase: p.base(), Format: p.format()}

	if !p.atEnd() {
		p.expect(",")
		s.Items = p.ioList()
	}

	p.end()

	return s
}

// parseRead parses READ: with a control list, as WRITE is parsed
// (parseTransfer); without one, READ f [, iolist], as PRINT is (section
// 7.2.1).
func parseRead(p *parser) ast.Stmt {
	if p.at("(") {
		return parseTransfer(p)
	}

	return parsePrint(p)
}

// parseTransfer parses READ, WRITE, and REWRITE with a control list,
// keyword (cilist) [iolist] (chapter 7).
func parseTransfer(p *parser) ast.Stmt {
	s := &ast.TransferStmt{StmtBase: p.base()}
	c := p.controlList()

	s.Unit, s.Format, s.Namelist, s.Rec, s.Specs = *c.unit, c.format, c.namelist, c.rec, c.specs

	if !p.atEnd() {
		s.Items = p.ioList()
	}

	p.end()

	return s
}

// parseCode parses ENCODE and DECODE (c, f, b[,IOSTAT=ios][,ERR=s])
// [list] (appendix A.1).
func parseCode(p *parser) ast.Stmt {
	s := &ast.CodeStmt{StmtBase: p.base()}

	p.expect("(")
	s.Count = p.expr()
	p.expect(",")
	s.Format = p.format()
	p.expect(",")
	s.Buffer = p.expr()

	seen := map[string]bool{}

	for p.accept(",") {
		t := p.peek()
		if !p.atKeyword() {
			p.syntax("IOSTAT= or ERR=")
		}

		p.next()
		p.next()
		p.once(seen, t)

		spec := &ast.IOSpec{Keyword: t.Text, Pos: p.pos(*t)}

		switch t.Text {
		case "ERR":
			l := p.labelRef()
			spec.Label = &l
		case "IOSTAT":
			spec.Value = p.expr()
		default:
			p.fail("BADSPECIFIER", p.pos(*t), t.Text, p.st.Kind.String())
		}

		s.Specs = append(s.Specs, spec)
	}

	p.expect(")")

	if !p.atEnd() {
		s.Items = p.ioList()
	}

	p.end()

	return s
}

// parseAuxIO parses OPEN, CLOSE, INQUIRE, REWIND, BACKSPACE, ENDFILE,
// UNLOCK, DELETE, and FIND (chapter 9, appendix A.3). REWIND, BACKSPACE,
// ENDFILE, and UNLOCK may give their unit alone, REWIND 5.
func parseAuxIO(p *parser) ast.Stmt {
	s := &ast.AuxIOStmt{StmtBase: p.base()}

	switch p.st.Kind {
	case lex.Rewind, lex.Backspace, lex.EndFile, lex.Unlock:
		if !p.at("(") {
			u := p.unitSpec()
			s.Unit = &u
			p.end()

			return s
		}
	}

	c := p.controlList()

	s.Unit, s.Rec, s.Specs = c.unit, c.rec, c.specs
	p.end()

	return s
}

// parseDefineFile parses DEFINE FILE u(m,n,U,v)[,u(m,n,U,v)]... (appendix
// A.2): u is an integer constant or variable, read as one token, since
// LUN(100, 10, U, NREC) would otherwise be an array element.
func parseDefineFile(p *parser) ast.Stmt {
	s := &ast.DefineFileStmt{StmtBase: p.base()}

	for {
		t := p.peek()
		if t == nil || (t.Kind != lex.IntegerConst && t.Kind != lex.Name) {
			p.syntax("a unit number")
		}

		p.next()

		f := &ast.FileDef{Pos: p.pos(*t)}

		if t.Kind == lex.Name {
			f.Unit = &ast.Name{ExprBase: ast.ExprBase{Pos: f.Pos}, Name: t.Text}
		} else {
			f.Unit = &ast.IntConst{ExprBase: ast.ExprBase{Pos: f.Pos}, Text: t.Text}
		}

		p.expect("(")
		f.Records = p.expr()
		p.expect(",")
		f.Length = p.expr()
		p.expect(",")

		if !p.atName("U") {
			p.syntax("U, for an unformatted file")
		}

		p.next()
		p.expect(",")
		f.Var = p.name("the associated variable")
		p.expect(")")

		s.Files = append(s.Files, f)

		if !p.accept(",") {
			break
		}
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
// format (or a namelist group's name).
func (p *parser) format() ast.FormatSpec {
	pos := p.here()

	switch {
	case p.accept("*"):
		return ast.FormatSpec{Star: true, Pos: pos}

	case p.atKind(lex.IntegerConst) && p.peek().Radix == 0:
		ref := p.labelRef()

		return ast.FormatSpec{Label: &ref, Pos: pos}
	}

	return ast.FormatSpec{Expr: p.expr(), Pos: pos}
}

// ioList reads an I/O list's items, separated by commas: expressions (an
// output list's may be any), whole arrays, and implied-DO lists.
func (p *parser) ioList() []ast.Expr {
	var list []ast.Expr

	for {
		list = append(list, p.ioItem())

		if !p.accept(",") {
			return list
		}
	}
}

// ioItem reads one I/O list item: an implied-DO list, (list, i = e1, e2
// [, e3]) (section 7.1.2.2), or an expression.
func (p *parser) ioItem() ast.Expr {
	if p.impliedDo() {
		return p.impliedDoList(p.ioItem)
	}

	return p.expr()
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

// impliedDoList reads an implied-DO list, (items, v = e1, e2 [, e3]),
// each item read by item: the items run to the name followed by an =,
// the DO variable.
func (p *parser) impliedDoList(item func() ast.Expr) ast.Expr {
	d := &ast.ImpliedDo{ExprBase: ast.ExprBase{Pos: p.here()}}

	p.expect("(")

	for !p.atKeyword() {
		d.Items = append(d.Items, item())
		p.expect(",")
	}

	d.Var = p.name("the implied-DO variable")
	p.expect("=")
	d.Start = p.expr()
	p.expect(",")
	d.Limit = p.expr()

	if p.accept(",") {
		d.Step = p.expr()
	}

	p.expect(")")

	return d
}
