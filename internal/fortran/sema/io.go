package sema

import (
	"errors"
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
	"github.com/tucats/govax/internal/fortran/format"
	"github.com/tucats/govax/internal/fortran/lex"
)

// The I/O statements (chapter 7, appendix A.1), and the statements they
// use: FORMAT (chapter 8) and NAMELIST (section 4.10). Every form of
// formatted, list-directed, namelist, and unformatted sequential READ,
// WRITE, PRINT, TYPE, and ACCEPT is analyzed into an IO, and formatted and
// unformatted direct access READ and WRITE, ENCODE, and DECODE too, and
// REWRITE and the keyed READ (sections 7.4 and 7.2.3; FTNRTL answers the keyed
// READ FOR$_NOTIMP until indexed files are done, Decision 13 of docs/PHASE-81 -
// FORTRAN files.md).

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
	ListForm        Form = iota // list-directed: format * (section 7.2.1.2)
	FormattedForm               // under a format
	NamelistForm                // a NAMELIST group's (section 7.2.1.3)
	UnformattedForm             // no format: the values' bytes (section 7.2.1.4)
)

// Format is a FORMAT statement: its label, its specification's text,
// and the specification read by internal/fortran/format.
//
// VFEs are its variable format expressions (section 8.3.2), each an
// INTEGER*4 value, and Encoded is the text FTNRTL gets: Text, with each
// <expr> made <n>, its number in VFEs from 1.
type Format struct {
	Label   int
	Text    string
	Encoded string
	Parsed  *format.Format
	VFEs    []Expr
	AST     *ast.FormatStmt
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

	// Rewrite is set for REWRITE, which writes over a file's current record.
	Rewrite bool

	// Key is a keyed READ's key, an INTEGER or CHARACTER value or a BYTE array
	// holding text, nil for any other statement; KeyMatch is how a record's key
	// must compare with it, one of the Key constants; and KeyID is KEYID='s key
	// number, an INTEGER*4, or nil to go on with the unit's last.
	Key      Expr
	KeyMatch int
	KeyID    Expr

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

	// Rec is a direct access statement's record number (REC=r or u'r), an
	// INTEGER*4, or nil for sequential access.
	Rec Expr

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

	if a.unit.Formats == nil {
		a.unit.Formats = map[int]*Format{}
	}

	a.unit.Formats[s.Label] = &Format{Label: s.Label, Text: s.Text, Encoded: encodeVFEs(s.Text, f), Parsed: f, AST: s}
}

// encodeVFEs is a format's text with each variable format expression
// replaced by its number: FTNRTL has no use for the expression's text, and
// calls the routine the compiler writes with the number.
func encodeVFEs(text string, f *format.Format) string {
	var b strings.Builder

	at := 0

	for i, v := range f.VFEs {
		b.WriteString(text[at:v.Offset])
		b.WriteString(strconv.Itoa(i + 1))
		at = v.Offset + len(v.Text)
	}

	b.WriteString(text[at:])

	return b.String()
}

// formatExpressions analyzes a FORMAT statement's variable format
// expressions, in the statements walk, when the declarations have said what
// each name is. Any expression may be one, but a CHARACTER, LOGICAL, or
// array value isn't a number; a REAL or COMPLEX one is converted to an
// integer (section 8.3.2).
func (a *analyzer) formatExpressions(s *ast.FormatStmt) {
	f := a.unit.Formats[s.Label]
	if f == nil || f.AST != s {
		return
	}

	for _, x := range s.VFEs {
		v := a.value(x)

		switch v.Type().Kind {
		case Integer, Real, Complex, Invalid:
		default:
			a.report("BADVFE", x.Position(), article(v.Type().String()))
		}

		f.VFEs = append(f.VFEs, a.convert(v, IntegerType))
	}
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
		a.noteOther(whatNamelist, g.Name, g.Pos, RefDefined)

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
			case s.Automatic:
				a.report("NMLAUTOMATIC", n.Pos, s.Name)
			case s.Struct != nil:
				a.report("NMLITEM", n.Pos, s.Name, a.describe(s))
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

	if s.Kind == lex.Rewrite {
		io.Rewrite = true

		return a.byRecord(io, s, path, "a REWRITE")
	}

	specs, keyed, ok := a.keySpecs(io, s)
	if !ok {
		return nil
	}

	s.Specs = specs

	switch {
	case keyed && s.Rec != nil:
		a.report("KEYEDREC", s.Pos)

		return nil
	case keyed:
		return a.byRecord(io, s, path, "a keyed READ")
	case s.Rec != nil:
		return a.direct(io, s, path)

	case s.Format == nil && s.Namelist == nil:
		return a.unformatted(io, s, path)
	}

	ok = a.ioUnit(io, &s.Unit)

	if s.Namelist != nil {
		io.Form, io.Group = NamelistForm, a.unit.namelists[s.Namelist.Name]

		if io.Group != nil {
			a.noteOther(whatNamelist, s.Namelist.Name, s.Namelist.Pos, RefUsed)
		}

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

// unformatted analyzes an unformatted sequential READ or WRITE (sections
// 7.2.1.4 and 7.3.1.4): a control list with a unit and no format, whose
// statement reads or writes one record of its items' bytes. The unit is a
// number: an internal file and the default unit are formatted (section
// 7.1.1.1).
func (a *analyzer) unformatted(io *IO, s *ast.TransferStmt, path []frame) Stmt {
	io.Form = UnformattedForm
	ok := a.ioUnit(io, &s.Unit)

	if ok && (io.Internal || io.Unit == nil) {
		a.report("UNFUNIT", s.Unit.Pos)

		ok = false
	}

	if !a.ioSpecs(io, s.Specs, path) || !ok {
		return nil
	}

	return a.ioItems(io, s.Items)
}

// direct analyzes a direct access READ or WRITE (sections 7.2.2 and 7.3.2):
// a unit number, a record number, and a format or none, formatted or
// unformatted, never list-directed or namelist; a READ has no END=, since a
// record by number has no end of file to reach.
func (a *analyzer) direct(io *IO, s *ast.TransferStmt, path []frame) Stmt {
	ok := a.ioUnit(io, &s.Unit)

	if ok && (io.Internal || io.Unit == nil) {
		a.report("DIRECTUNIT", s.Unit.Pos)

		ok = false
	}

	if io.Rec = a.recordNumber(s.Rec); io.Rec == nil {
		ok = false
	}

	switch {
	case s.Namelist != nil:
		a.report("DIRECTFORM", s.Namelist.Pos, "namelist")

		ok = false
	case s.Format == nil:
		io.Form = UnformattedForm
	case s.Format.Star:
		a.report("DIRECTFORM", s.Format.Pos, "list-directed")

		ok = false
	case a.namelistNamed(s.Format) != nil:
		a.report("DIRECTFORM", s.Format.Pos, "namelist")

		ok = false
	case !a.ioFormat(io, s.Format):
		ok = false
	}

	for _, spec := range s.Specs {
		if spec.Keyword == "END" {
			a.report("DIRECTEND", spec.Pos)

			ok = false
		}
	}

	if !a.ioSpecs(io, s.Specs, path) || !ok {
		return nil
	}

	return a.ioItems(io, s.Items)
}

// How a keyed READ's key compares (section 7.1.1.7), passed to FTNRTL as it
// stands. A file's direction isn't known until the program runs, so these say
// which specifier was written: FTNRTL makes KeyNext and KeyNextDifferent mean
// what the key's direction says, and refuses the four one-directional forms
// against the other direction (FOR$_INVMATKEY).
const (
	KeyEqual         = iota // KEY=, KEYEQ=
	KeyGreaterEqual         // KEYGE=, ascending keys only
	KeyGreater              // KEYGT=, ascending keys only
	KeyLessEqual            // KEYLE=, descending keys only
	KeyLess                 // KEYLT=, descending keys only
	KeyNext                 // KEYNXT=, equal or next in the key's direction
	KeyNextDifferent        // KEYNXTNE=, next in the key's direction
)

// keyMatches are the keyed READ's specifiers and the comparison each asks for.
var keyMatches = map[string]int{
	"KEY": KeyEqual, "KEYEQ": KeyEqual,
	"KEYGE": KeyGreaterEqual, "KEYGT": KeyGreater,
	"KEYLE": KeyLessEqual, "KEYLT": KeyLess,
	"KEYNXT": KeyNext, "KEYNXTNE": KeyNextDifferent,
}

// keySpecs takes a READ's key specifiers out of its control list (sections
// 7.1.1.7 and 7.1.1.8), returning the rest, and whether the READ is keyed.
func (a *analyzer) keySpecs(io *IO, s *ast.TransferStmt) (rest []*ast.IOSpec, keyed, ok bool) {
	ok = true

	var keyID *ast.IOSpec

	for _, spec := range s.Specs {
		match, isKey := keyMatches[spec.Keyword]

		switch {
		case isKey:
			keyed = true

			if io.Key != nil {
				a.report("KEYTWICE", spec.Pos, spec.Keyword, "a key")

				ok = false

				continue
			}

			if v := a.keyValue(spec); v != nil {
				io.Key, io.KeyMatch = v, match
			} else {
				ok = false
			}

		case spec.Keyword == "KEYID":
			keyed = true

			keyID = spec
			v := a.value(spec.Value)

			switch t := v.Type(); t.Kind {
			case Invalid:
				ok = false
			case Integer:
				io.KeyID = a.convert(v, IntegerType)
			default:
				a.report("KEYEDVALUE", spec.Pos, spec.Keyword, "an INTEGER value", article(t.String())+" value")

				ok = false
			}

		default:
			rest = append(rest, spec)
		}
	}

	// KEYID= only says which key the key's value is for (section 7.1.1.8).
	if keyID != nil && io.Key == nil && ok {
		a.report("KEYIDALONE", keyID.Pos)

		ok = false
	}

	return rest, keyed, ok
}

// keyValue analyzes a key's value (section 7.1.1.7): an INTEGER or CHARACTER
// expression, or the name of a BYTE array holding text; nil, the error
// reported, if it is none.
func (a *analyzer) keyValue(spec *ast.IOSpec) Expr {
	if n, isName := spec.Value.(*ast.Name); isName {
		if s := a.unit.byName[n.Name]; s != nil && len(s.Dims) > 0 && s.Type.Kind == Logical && s.Type.Size == 1 {
			if s.AssumedSize() {
				a.report("NOSIZE", spec.Pos, s.Name)

				return nil
			}

			return a.name(n, true)
		}
	}

	v := a.value(spec.Value)

	switch t := v.Type(); t.Kind {
	case Invalid:
		return nil
	case Integer, Character:
		return v
	default:
		a.report("KEYEDVALUE", spec.Pos, spec.Keyword, "an INTEGER or CHARACTER value", article(t.String())+" value")

		return nil
	}
}

// byRecord analyzes a REWRITE (section 7.4) or a keyed READ (section 7.2.3),
// what, each a statement on one record of a file, the unit's current record
// or the one the READ's key chooses: a unit number, and a format or none.
func (a *analyzer) byRecord(io *IO, s *ast.TransferStmt, path []frame, what string) Stmt {
	ok := a.ioUnit(io, &s.Unit)

	if ok && (io.Internal || io.Unit == nil) {
		a.report("RECORDUNIT", s.Unit.Pos, what)

		ok = false
	}

	switch {
	case s.Namelist != nil:
		a.report("RECORDFORM", s.Namelist.Pos, what, "namelist")

		ok = false
	case s.Format == nil:
		io.Form = UnformattedForm
	case s.Format.Star:
		a.report("RECORDFORM", s.Format.Pos, what, "list-directed")

		ok = false
	case a.namelistNamed(s.Format) != nil:
		a.report("RECORDFORM", s.Format.Pos, what, "namelist")

		ok = false
	case !a.ioFormat(io, s.Format):
		ok = false
	}

	// A keyed READ has no END=: a key has no end of file to reach (section
	// 7.2.3). The parser already refuses it on a REWRITE.
	for _, spec := range s.Specs {
		if spec.Keyword == "END" {
			a.report("KEYEDEND", spec.Pos)

			ok = false
		}
	}

	if !a.ioSpecs(io, s.Specs, path) || !ok {
		return nil
	}

	return a.ioItems(io, s.Items)
}

// recordNumber analyzes a direct access record number (section 7.1.1.4): a
// numeric expression, converted to INTEGER*4; nil if it is none.
func (a *analyzer) recordNumber(e ast.Expr) Expr {
	v := a.value(e)

	switch t := v.Type(); t.Kind {
	case Invalid:
		return nil
	case Integer, Real:
		return a.convert(v, IntegerType)
	default:
		a.report("BADREC", e.Position(), article(t.String())+" value")

		return nil
	}
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

	// Appendix A.1's buffer is a variable, an array, an element, or a substring:
	// not an aggregate reference.
	buffer := a.place(s.Buffer, true)
	if buffer != nil && buffer.Type().Kind == Record {
		buffer = nil
	}

	if !io.Read {
		a.noteAssigned(buffer)
	}

	switch b := buffer.(type) {
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

		if !io.Read {
			a.noteAssigned(t)
		}

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

	case *ast.FieldRef:
		return a.field(e, whole)
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
		a.noteOther(whatNamelist, n.Name, f.Expr.Position(), RefUsed)

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

	case isArrayPlace(e) && t.Kind != Record:
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

				a.noteAssigned(t)

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

	a.checkRecordItems(io, items)

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
// (section 2.2.3.5; not an assumed-size one, whose size isn't known), an
// aggregate reference, or a character constant.
func (a *analyzer) outputItem(e ast.Expr) Expr {
	if n, ok := e.(*ast.Name); ok {
		item := a.name(n, true)

		if r, ok := item.(*ArrayRef); ok && r.Sym.AssumedSize() {
			a.report("NOSIZE", n.Pos, n.Name)

			return nil
		}

		return item
	}

	if f, ok := e.(*ast.FieldRef); ok {
		return a.field(f, true)
	}

	if ap, ok := e.(*ast.Apply); ok {
		return a.apply(ap)
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

	a.noteAssigned(t)

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
