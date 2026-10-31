package sema

import (
	"errors"

	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
	"github.com/tucats/govax/internal/fortran/format"
	"github.com/tucats/govax/internal/fortran/lex"
)

// The I/O statements (chapter 7, appendix A.1), and the statements they
// use: FORMAT (chapter 8) and NAMELIST (section 4.10). Every form of
// formatted, list-directed, and namelist READ, WRITE, PRINT, TYPE, and
// ACCEPT is analyzed into an IO, and ENCODE and DECODE too; what is left
// for later phases is unformatted, direct access, and keyed I/O, REWRITE,
// and variable format expressions (NOTIMPL).

// The default units, which the statements without a unit and the units
// written * use. VMS's FORTRAN run-time library numbers them -1 to -4,
// -1 and -2 writing SYS$OUTPUT and -3 and -4 reading SYS$INPUT
// (testdata/for80, round 2); which statement uses which of each pair
// wasn't shown, and this is govax's choice.
const (
	PrintUnit  = -1 // PRINT, and WRITE (*,...)
	TypeUnit   = -2 // TYPE
	AcceptUnit = -3 // ACCEPT
	ReadUnit   = -4 // READ f, and READ (*,...)
)

// Form is how an I/O statement turns values into text and back.
type Form int

// The forms of I/O this phase compiles.
const (
	ListForm      Form = iota // list-directed: format * (section 7.2.1.2)
	FormattedForm             // under a format
	NamelistForm              // a NAMELIST group's (section 7.2.1.3)
)

// Format is a FORMAT statement: its label, its specification's text,
// and the specification read by internal/fortran/format.
type Format struct {
	Label  int
	Text   string
	Parsed *format.Format
	AST    *ast.FormatStmt
}

// Namelist is a NAMELIST group: its name and the variables and arrays it
// holds, in order, and a reference to each (a *Var or an *ArrayRef).
type Namelist struct {
	Name  string
	Items []*Symbol
	Refs  []Expr
	Pos   diag.Position
}

// IO is an I/O statement: READ, WRITE, PRINT, TYPE, ACCEPT, ENCODE, or
// DECODE (by its AST's kind).
type IO struct {
	stmtBase

	// Read is set for input: READ, ACCEPT, DECODE.
	Read bool

	// Unit is the unit, an INTEGER*4 value, or with Internal set an
	// internal file (a CHARACTER variable, array element, substring, or
	// array), or with Code set ENCODE's or DECODE's buffer (a place of
	// any type); nil for a default unit, Default.
	Unit     Expr
	Default  int
	Internal bool
	Code     bool

	// Count is ENCODE's or DECODE's count of characters, an INTEGER*4.
	Count Expr

	// Form is the statement's form. A formatted one's format is Format, a
	// FORMAT statement's; FormatVar, an INTEGER*4 variable a FORMAT's
	// label was ASSIGNed to; or RunFormat, a run-time format (a CHARACTER
	// value, or an array or array element of any type whose bytes are the
	// format). A namelist statement's group is Group.
	Form      Form
	Format    *Format
	FormatVar Expr
	RunFormat Expr
	Group     *Namelist

	// Err and End are ERR='s and END='s labels, 0 when not given; IOStat
	// is IOSTAT='s variable or element, an INTEGER, or nil.
	Err, End int
	IOStat   Expr

	// Items are the I/O list.
	Items []*Item
}

// Item is one item of an I/O list: a value (an input list's a place), a
// whole array (an *ArrayRef), or an implied-DO list.
type Item struct {
	Value Expr
	Loop  *ImpliedDo
}

// ImpliedDo is an implied-DO list in an I/O list (section 7.1.2.2): its
// items for each value of Var from Start to Limit by Step, which are
// converted to Var's type as a DO statement's are.
type ImpliedDo struct {
	Var                *Symbol
	Start, Limit, Step Expr
	Items              []*Item
	Pos                diag.Position
}

// formatStatement reads a FORMAT statement (chapter 8), in the
// declarations walk so that a statement may use a FORMAT that comes after
// it: its specification checked by internal/fortran/format, and the
// format entered under its label.
func (a *analyzer) formatStatement(s *ast.FormatStmt) {
	if s.Label == 0 {
		a.messages.Report("FMTLABEL", s.Pos)

		return
	}

	f, err := format.Parse(s.Text, format.Options{})

	var e *format.Error
	if errors.As(err, &e) {
		a.messages.Report("FMTSYNTAX", s.Source.Position(s.Offset+e.Offset), e.Text)

		return
	}

	if len(f.VFEs) > 0 {
		a.notImplForm(s.Source.Position(s.Offset+f.VFEs[0].Offset-1), "a variable format expression")

		return
	}

	if a.unit.Formats == nil {
		a.unit.Formats = map[int]*Format{}
	}

	a.unit.Formats[s.Label] = &Format{Label: s.Label, Text: s.Text, Parsed: f, AST: s}
}

// namelistStatement reads a NAMELIST statement's groups (section 4.10).
// Each name is looked up now, and checked once the unit's statements
// have said what each is (checkNamelists).
func (a *analyzer) namelistStatement(s *ast.NamelistStmt) {
	for _, g := range s.Groups {
		if old := a.unit.namelists[g.Name]; old != nil {
			a.report("NMLTWICE", g.Pos, g.Name, old.Pos.Line)

			continue
		}

		n := &Namelist{Name: g.Name, Pos: g.Pos}

		for _, item := range g.Items {
			n.Items = append(n.Items, a.lookup(item.Name, item.Pos))
		}

		if a.unit.namelists == nil {
			a.unit.namelists = map[string]*Namelist{}
		}

		a.unit.namelists[g.Name] = n
		a.unit.Namelists = append(a.unit.Namelists, n)
	}
}

// checkNamelists checks each NAMELIST group's names, now that the
// statements have said what each is: only variables and arrays, and no
// dummy arguments (section 4.10). A name used nowhere else is a
// variable.
func (a *analyzer) checkNamelists() {
	for _, n := range a.unit.Namelists {
		for _, s := range n.Items {
			if s.broken {
				continue
			}

			if s.Class == Unclassified && !s.External && s.Dummy == 0 {
				s.Class = Variable
			}

			switch {
			case s.Dummy != 0:
				a.report("NMLDUMMY", n.Pos, s.Name)
			case s.External || (s.Class != Variable && s.Class != Array):
				a.report("NMLITEM", n.Pos, s.Name, a.describe(s))
			case s.Class == Array:
				n.Refs = append(n.Refs, &ArrayRef{exprBase{s.Type, n.Pos}, s})
			default:
				n.Refs = append(n.Refs, &Var{exprBase{s.Type, n.Pos}, s})
			}
		}
	}
}

// printStatement analyzes PRINT, TYPE, ACCEPT, and READ f (sections
// 7.2.1, 7.5, 7.6): a default unit, and a format, *, or a namelist group.
func (a *analyzer) printStatement(s *ast.PrintStmt) Stmt {
	io := &IO{stmtBase: stmtBase{s}}

	switch s.Kind {
	case lex.Print:
		io.Default = PrintUnit
	case lex.Type:
		io.Default = TypeUnit
	case lex.Accept:
		io.Read, io.Default = true, AcceptUnit
	default:
		io.Read, io.Default = true, ReadUnit
	}

	if !a.ioFormat(io, &s.Format) {
		return nil
	}

	return a.ioItems(io, s.Items)
}

// transferStatement analyzes READ and WRITE with a control list
// (section 7.1.1).
func (a *analyzer) transferStatement(s *ast.TransferStmt, path []frame) Stmt {
	io := &IO{stmtBase: stmtBase{s}, Read: s.Kind == lex.Read, Default: PrintUnit}
	if io.Read {
		io.Default = ReadUnit
	}

	switch {
	case s.Rec != nil:
		a.notImplForm(s.Rec.Position(), "direct access I/O")

		return nil

	case s.Format == nil && s.Namelist == nil:
		a.notImplForm(s.Pos, "unformatted I/O")

		return nil
	}

	ok := a.ioUnit(io, &s.Unit)

	if s.Namelist != nil {
		io.Form, io.Group = NamelistForm, a.unit.namelists[s.Namelist.Name]

		if io.Group == nil {
			a.report("NOTNAMELIST", s.Namelist.Pos, s.Namelist.Name)

			ok = false
		}
	} else if !a.ioFormat(io, s.Format) {
		ok = false
	}

	if io.Internal && io.Form == NamelistForm {
		a.report("NMLINTERNAL", s.Unit.Pos)

		ok = false
	}

	if !a.ioSpecs(io, s.Specs, path) || !ok {
		return nil
	}

	return a.ioItems(io, s.Items)
}

// codeStatement analyzes ENCODE and DECODE (c, f, b[,IOSTAT=ios][,ERR=s])
// (appendix A.1): one record of c characters, b's, written or read under
// the format f.
func (a *analyzer) codeStatement(s *ast.CodeStmt, path []frame) Stmt {
	io := &IO{stmtBase: stmtBase{s}, Read: s.Kind == lex.Decode, Code: true}
	what := s.Kind.String()
	ok := true

	count := a.value(s.Count)

	switch t := count.Type(); {
	case t.Kind == Invalid:
		ok = false
	case t.Kind != Integer:
		a.report("BADCOUNT", s.Count.Position(), what)

		ok = false
	default:
		io.Count = a.convert(count, IntegerType)
	}

	if s.Format.Star || a.namelistNamed(&s.Format) != nil {
		a.report("CODEFORMAT", s.Format.Pos, what)

		ok = false
	} else if !a.ioFormat(io, &s.Format) {
		ok = false
	}

	switch b := a.place(s.Buffer, true).(type) {
	case nil:
		a.report("BADBUFFER", s.Buffer.Position(), what)

		ok = false
	case *ArrayRef:
		if b.Sym.AssumedSize() {
			a.report("NOSIZE", s.Buffer.Position(), b.Sym.Name)

			ok = false
		}

		io.Unit = b
	default:
		if b.Type().Kind == Invalid {
			ok = false
		}

		io.Unit = b
	}

	if !a.ioSpecs(io, s.Specs, path) || !ok {
		return nil
	}

	return a.ioItems(io, s.Items)
}

// ioUnit analyzes a control list's unit (section 7.1.1.1): * (the
// default unit), an INTEGER expression, or an internal file. A number
// outside the units FTNRTL has is the program's error when it runs
// (INVLOGUNI), as a unit may be any expression.
func (a *analyzer) ioUnit(io *IO, u *ast.UnitSpec) bool {
	if u.Star {
		return true
	}

	if t := a.place(u.Expr, true); t != nil && t.Type().Kind == Character {
		if r, ok := t.(*ArrayRef); ok && r.Sym.AssumedSize() {
			a.report("NOSIZE", u.Pos, r.Sym.Name)

			return false
		}

		io.Unit, io.Internal = t, true

		return true
	}

	v := a.value(u.Expr)

	switch t := v.Type(); {
	case t.Kind == Invalid:
		return false
	case t.Kind != Integer:
		a.report("BADUNIT", u.Pos, article(t.String())+" value")

		return false
	}

	io.Unit = a.convert(v, IntegerType)

	return true
}

// place is e if it is a place a value can be stored in: a variable, an
// array element, a substring, or, if whole is set, a whole array; nil if
// it isn't one. A name that is a constant or a procedure isn't, and gets
// no message here.
func (a *analyzer) place(e ast.Expr, whole bool) Expr {
	switch e := e.(type) {
	case *ast.Name:
		if s := a.unit.byName[e.Name]; s != nil && (s.Class == Constant || s.External || s.Class == Intrinsic ||
			s.Class == Function || s.Class == Subroutine || s.Class == StmtFunction || s.Class == UnitName) {
			return nil
		}

		switch t := a.name(e, whole).(type) {
		case *Var, *ArrayRef:
			return t
		case *Const:
			return nil
		default:
			// An error, reported.
			return t
		}

	case *ast.Apply:
		if s := a.unit.byName[e.Name]; s == nil || s.Class != Array {
			return nil
		}

		return a.target(e)

	case *ast.Substring:
		return a.target(e)
	}

	return nil
}

// namelistNamed is the NAMELIST group a format given by position names
// (READ (5, NL), PRINT NL), or nil.
func (a *analyzer) namelistNamed(f *ast.FormatSpec) *Namelist {
	if n, ok := f.Expr.(*ast.Name); ok {
		return a.unit.namelists[n.Name]
	}

	return nil
}

// ioFormat analyzes an I/O statement's format (section 7.1.1.2), in this
// order: *; a label, which must be a FORMAT statement's (checkLabels); a
// NAMELIST group's name; a CHARACTER value; an array or an array element,
// whose bytes hold a run-time format; an INTEGER*4 variable a FORMAT's
// label was ASSIGNed to. It reports whether the format is one.
func (a *analyzer) ioFormat(io *IO, f *ast.FormatSpec) bool {
	switch {
	case f.Star:
		io.Form = ListForm

		return true

	case f.Label != nil:
		io.Form = FormattedForm
		io.Format = a.unit.Formats[f.Label.Label]
		a.refs = append(a.refs, labelRef{label: f.Label.Label, pos: f.Label.Pos, format: true})

		return true
	}

	if n := a.namelistNamed(f); n != nil {
		io.Form, io.Group = NamelistForm, n

		return true
	}

	io.Form = FormattedForm

	var e Expr
	if n, ok := f.Expr.(*ast.Name); ok {
		e = a.name(n, true)
	} else {
		e = a.value(f.Expr)
	}

	switch t := e.Type(); {
	case t.Kind == Invalid:
		return false

	case t.Kind == Character:
		io.RunFormat = e

	case isArrayPlace(e):
		if r, ok := e.(*ArrayRef); ok && r.Sym.AssumedSize() {
			a.report("NOSIZE", f.Pos, r.Sym.Name)

			return false
		}

		io.RunFormat = e

	case t == IntegerType:
		if _, ok := e.(*Var); ok {
			io.FormatVar = e

			return true
		}

		fallthrough

	default:
		a.report("BADFORMAT", f.Pos, article(t.String())+" value")

		return false
	}

	return true
}

// isArrayPlace reports whether e is a whole array or an array element.
func isArrayPlace(e Expr) bool {
	switch e.(type) {
	case *ArrayRef, *Elem:
		return true
	}

	return false
}

// ioSpecs analyzes a control list's ERR=, END=, and IOSTAT= (sections
// 7.1.1.9 and 7.1.1.10): ERR and END are jumps from the statement, and
// IOSTAT names an INTEGER variable or array element. Keyed READ's
// specifiers are Phase 81's.
func (a *analyzer) ioSpecs(io *IO, specs []*ast.IOSpec, path []frame) bool {
	ok := true

	for _, s := range specs {
		switch s.Keyword {
		case "ERR", "END":
			a.jumps([]ast.LabelRef{*s.Label}, path)

			if s.Keyword == "ERR" {
				io.Err = s.Label.Label
			} else {
				io.End = s.Label.Label
			}

		case "IOSTAT":
			t := a.place(s.Value, false)

			switch {
			case t == nil:
				a.report("BADIOSTAT", s.Pos)

				ok = false
			case t.Type().Kind == Invalid:
				ok = false
			case t.Type().Kind != Integer:
				a.report("BADIOSTAT", s.Pos)

				ok = false
			default:
				if v, isVar := t.(*Var); isVar {
					a.checkLoopVariable(v.Sym, v.Pos)
				}

				io.IOStat = t
			}

		default:
			a.notImplForm(s.Pos, "the "+s.Keyword+"= specifier")

			ok = false
		}
	}

	return ok
}

// ioItems analyzes the I/O list and finishes the statement.
func (a *analyzer) ioItems(io *IO, list []ast.Expr) Stmt {
	items, ok := a.ioList(list, io.Read)
	if !ok {
		return nil
	}

	io.Items = items

	return io
}

// ioList analyzes an I/O list (section 7.1.2): an output list's items
// are values of any kind, whole arrays, and implied-DO lists; an input
// list's are places, whole arrays, and implied-DO lists.
func (a *analyzer) ioList(list []ast.Expr, read bool) ([]*Item, bool) {
	items := make([]*Item, 0, len(list))
	ok := true

	for _, e := range list {
		if d, isDo := e.(*ast.ImpliedDo); isDo {
			loop, good := a.impliedDo(d, read)
			ok = ok && good
			items = append(items, &Item{Loop: loop})

			continue
		}

		var v Expr

		if read {
			v = a.inputItem(e)
		} else {
			v = a.outputItem(e)
		}

		if v == nil {
			ok = false

			continue
		}

		// An item with an error stays, as a statement's value does: the
		// statement is still shown in the dump.
		items = append(items, &Item{Value: v})
	}

	return items, ok
}

// outputItem analyzes an output list's item: a value, a whole array
// (section 2.2.3.5; not an assumed-size one, whose size isn't known), or
// a character constant.
func (a *analyzer) outputItem(e ast.Expr) Expr {
	if n, ok := e.(*ast.Name); ok {
		item := a.name(n, true)

		if r, ok := item.(*ArrayRef); ok && r.Sym.AssumedSize() {
			a.report("NOSIZE", n.Pos, n.Name)

			return nil
		}

		return item
	}

	return a.value(e)
}

// inputItem analyzes an input list's item, a place: a variable, an array
// element, a substring, or a whole array. Reading into a DO loop's
// variable inside the loop is DOVARSET, as assigning it is.
func (a *analyzer) inputItem(e ast.Expr) Expr {
	t := a.place(e, true)

	switch t := t.(type) {
	case nil:
		a.report("BADINPUT", e.Position(), inputWhat(e))

		return nil

	case *ArrayRef:
		if t.Sym.AssumedSize() {
			a.report("NOSIZE", t.Pos, t.Sym.Name)

			return nil
		}

	case *Var:
		a.checkLoopVariable(t.Sym, t.Pos)
	}

	return t
}

// inputWhat names an input item that can't be read into, for BADINPUT.
func inputWhat(e ast.Expr) string {
	switch e := e.(type) {
	case *ast.Name:
		return e.Name
	case *ast.Apply:
		return e.Name + "(...)"
	}

	return "an expression"
}

// impliedDo analyzes an implied-DO list (section 7.1.2.2): its variable is
// a DO loop's, INTEGER or REAL, and its parameters are converted to the
// variable's type.
func (a *analyzer) impliedDo(d *ast.ImpliedDo, read bool) (*ImpliedDo, bool) {
	loop := &ImpliedDo{Var: a.doVariable(d.Var), Pos: d.Pos}

	t := IntegerType
	if loop.Var != nil {
		t = loop.Var.Type
	}

	param := func(e ast.Expr) Expr {
		return a.convert(a.arithmetic(a.value(e)), t)
	}

	loop.Start = param(d.Start)
	loop.Limit = param(d.Limit)

	if d.Step != nil {
		loop.Step = param(d.Step)

		if c, ok := loop.Step.(*Const); ok && (c.Int == 0 && c.T.Kind != Real || c.T.Kind == Real && c.Real.IsZero()) {
			a.report("DOSTEP0", d.Step.Position())
		}
	} else {
		loop.Step = a.convert(&Const{exprBase: exprBase{IntegerType, d.Pos}, Int: 1}, t)
	}

	items, ok := a.ioList(d.Items, read)
	loop.Items = items

	ok = ok && loop.Var != nil
	for _, e := range []Expr{loop.Start, loop.Limit, loop.Step} {
		ok = ok && e.Type().Kind != Invalid
	}

	return loop, ok
}
