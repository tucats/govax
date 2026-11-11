package sema

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
)

// Records and references to their fields (sections 2.2.5.2 and 4.13). A
// RECORD statement makes variables and arrays whose form is a structure
// (structure.go): RECORD /DATE/ TODAY, HISTORY(10) makes TODAY, a record of
// DATE's 4 bytes, and HISTORY, an array of ten. A record is a Symbol like
// any variable's, its Type of the kind Record and its Struct the structure,
// placed in $LOCAL by layout as any variable is.
//
// A scalar field reference names one typed item of a record:
// TODAY.YEAR, HISTORY(I).MONTH, NEXT_APP.APP_TIME(2).HOUR. It is a FieldRef,
// which may stand wherever a variable or an array element may (section
// 2.2.5.2: "can be treated like a normal reference to a FORTRAN variable or
// array element"), and its place is the record's, plus the field's offset,
// plus for each subscripted part its element's offset. Lowering folds the
// constant parts of that into one offset (Decision 2 of docs/PHASE-82 -
// FORTRAN DEC extensions.md).
//
// A record or a substructure as a whole is an aggregate reference (section
// 2.2.6): TODAY, HISTORY(3), NEXT_APP.APP_DATE. Its Type is of the kind Record,
// with the length of its structure, which structureOf finds. FORTRAN does only
// three things with one: assign it to another of the same structure (section
// 3.4), pass it by reference (section 6.1.1), and move its bytes in an
// unformatted I/O list (section 7.1.2). It is never a value, so value refuses
// it (scalar) and those three ask for it themselves. An array field without
// subscripts, like an array's name, is an array name reference (FieldRef's
// WholeArray), for arguments and I/O lists.

// maxItemBytes is the longest item a descriptor of class S can describe, whose
// length is a word.
const maxItemBytes = 65535

// FieldStep is one field of a field reference, with its subscripts, each
// INTEGER, if the field is an array; nil if it isn't.
type FieldStep struct {
	Field *Field
	Subs  []Expr
}

// FieldRef is a scalar field reference (section 2.2.5.2): Sym is the record,
// or the array of records whose element Subs (each INTEGER) names, and Path
// the fields from the record's own down to the field named last, whose type is
// the reference's, or for a substructure of the kind Record. Every field but
// the last is a substructure.
type FieldRef struct {
	exprBase

	// NamePos is where the record's name is; Pos is the last field's.
	NamePos diag.Position

	Sym  *Symbol
	Subs []Expr
	Path []FieldStep
}

// Field is the field the reference names, its last.
func (f *FieldRef) Field() *Field {
	return f.Path[len(f.Path)-1].Field
}

// WholeArray reports whether the reference is an array field without
// subscripts: every element of it, an array name reference.
func (f *FieldRef) WholeArray() bool {
	last := f.Path[len(f.Path)-1]

	return last.Field.Dims != nil && last.Subs == nil
}

// Text is the reference as messages and dumps name it, its subscripts
// shown only as there being some: R.F, RA().ARR().X.
func (f *FieldRef) Text() string {
	var b strings.Builder

	b.WriteString(f.Sym.Name)

	if f.Subs != nil {
		b.WriteString("()")
	}

	for _, s := range f.Path {
		b.WriteString("." + s.Field.Name)

		if s.Subs != nil {
			b.WriteString("()")
		}
	}

	return b.String()
}

// recordStatement makes each name a RECORD statement lists a record of its
// structure, which must be declared before it, with the dimensions an array
// declarator gives it (section 4.13). A record has no type of its own, so a
// name a type statement names can't be one.
func (a *analyzer) recordStatement(s *ast.RecordStmt) {
	for _, g := range s.Groups {
		st := a.unit.structs[g.Structure]
		if st == nil {
			a.report("NOSTRUCT", g.Pos, g.Structure)
		} else {
			a.noteOther(whatStructure, g.Structure, g.Pos, RefUsed)
		}

		for _, d := range g.Entities {
			sym := a.declare(d.Name, d.Pos)

			switch {
			case sym.broken:

			case st == nil:
				// Its uses would only repeat NOSTRUCT.
				sym.broken = true

			case sym.Struct != nil:
				a.report("DUPRECORD", d.Pos, d.Name, sym.TypePos.Line)

			case sym.Class == UnitName || sym.Result || sym.Class == Constant || sym.Class == StmtFunction ||
				sym.External || sym.Class == Intrinsic:
				a.badUse(sym, d.Pos, "be a record")

				sym.broken = true

			case sym.Typed:
				a.report("RECORDTYPED", d.Pos, d.Name, sym.TypePos.Line)

				sym.broken = true

			default:
				sym.Struct, sym.Type = st, Type{Record, st.Size}
				sym.Typed, sym.TypePos, sym.untyped = true, d.Pos, false

				if sym.Class == Unclassified {
					sym.Class = Variable
				}
			}

			if d.Dims != nil && !sym.broken {
				a.dimension(sym, d)
			}
		}
	}
}

// field analyzes a field reference: a scalar one, an aggregate one (which
// value refuses, scalar), or where whole allows it (an actual argument, an I/O
// list item) an array field without subscripts.
func (a *analyzer) field(e *ast.FieldRef, whole bool) Expr {
	// The syntax tree nests a reference from its last field in: R.A.B is
	// FieldRef{FieldRef{Name R, A}, B}. The steps, from the record's own
	// field out.
	var steps []*ast.FieldRef

	x := ast.Expr(e)

	for {
		f, ok := x.(*ast.FieldRef)
		if !ok {
			break
		}

		// Prepend: a new one-element slice with the older steps appended
		// after it ("steps..." spreads a slice into append's arguments).
		steps = append([]*ast.FieldRef{f}, steps...)
		x = f.X
	}

	ref := &FieldRef{exprBase: exprBase{Pos: e.Pos}, NamePos: x.Position()}

	if !a.recordBase(ref, x, steps[0].Field) {
		return a.bad(e.Pos)
	}

	// Walk down the path a field at a time. st is the structure the next
	// field must be in: the record's own first, then each substructure's.
	// A field that isn't a substructure has no structure (st nil), so a
	// field named after it is an error.
	st := ref.Sym.Struct

	var prev *Field

	for i, step := range steps {
		if st == nil {
			a.report("FIELDNOTRECORD", step.Pos, prev.Name, article(prev.Type.String()), step.Field)

			return a.bad(e.Pos)
		}

		f := st.Field(step.Field)

		switch {
		case f == nil:
			a.report("NOFIELD", step.Pos, st.Display(), step.Field)

			return a.bad(e.Pos)

		case f.Struct == nil && f.Type.Kind == Invalid:
			// Its declaration had an error, reported.
			return a.bad(e.Pos)
		}

		subs, ok := a.fieldSubscripts(f, step, whole && i == len(steps)-1)
		if !ok {
			return a.bad(e.Pos)
		}

		ref.Path = append(ref.Path, FieldStep{f, subs})
		st, prev = f.Struct, f
	}

	ref.T = prev.Type
	if prev.Struct != nil {
		ref.T = Type{Record, prev.Struct.Size}
	}

	return ref
}

// recordBase analyzes the record a field reference starts from, x: a
// record's name, or an element of an array of records. first is the field
// named after it, for the message if x isn't a record.
func (a *analyzer) recordBase(ref *FieldRef, x ast.Expr, first string) bool {
	var (
		name string
		pos  diag.Position
		args []ast.Expr
	)

	switch x := x.(type) {
	case *ast.Name:
		name, pos = x.Name, x.Pos
	case *ast.Apply:
		name, pos, args = x.Name, x.Pos, x.Args
	default:
		// The parser makes nothing else the start of a field reference.
		return false
	}

	s := a.lookup(name, pos)

	switch {
	case s.broken:
		return false

	case s.Struct == nil:
		what := a.describe(s)
		if s.Class == Unclassified {
			what = Variable.String()
		}

		a.report("NOTRECORD", pos, name, what, first)

		return false

	case args == nil && s.Class == Array:
		a.report("NEEDSUBSCRIPT", pos, name)

		return false

	case args != nil && s.Class != Array:
		a.report("NOTARRAY", pos, name)

		return false

	case len(args) != len(s.Dims):
		a.report("SUBSCRIPTS", pos, name, count(len(s.Dims), "subscript"), len(args))

		return false
	}

	ref.Sym = s

	if args != nil {
		subs, ok := a.subscripts(pos, args)
		if !ok {
			return false
		}

		a.checkConstantSubscripts(s, name, s.Dims, subs)

		ref.Subs = subs
	}

	return true
}

// fieldSubscripts are a field's subscripts in a reference, one for each of
// its dimensions, or none for a field that isn't an array. An array field
// without them is an array name reference (section 2.2.6), which only an
// actual argument or an I/O list item may be (whole).
func (a *analyzer) fieldSubscripts(f *Field, step *ast.FieldRef, whole bool) ([]Expr, bool) {
	switch {
	case f.Dims == nil && step.Args != nil:
		a.report("FIELDSUBS", step.Pos, f.Name, "no subscripts", len(step.Args))

		return nil, false

	case f.Dims == nil:
		return nil, true

	case step.Args == nil && whole:
		return nil, true

	case step.Args == nil:
		a.report("FIELDWHOLE", step.Pos, f.Name)

		return nil, false

	case len(step.Args) != len(f.Dims):
		a.report("FIELDSUBS", step.Pos, f.Name, count(len(f.Dims), "subscript"), len(step.Args))

		return nil, false
	}

	subs, ok := a.subscripts(step.Pos, step.Args)
	if ok {
		a.checkConstantSubscripts(nil, f.Name, f.Dims, subs)
	}

	return subs, ok
}

// subscripts analyzes an element's subscripts, each made INTEGER (a REAL one
// is truncated, section 2.2.3.2).
func (a *analyzer) subscripts(pos diag.Position, args []ast.Expr) ([]Expr, bool) {
	if a.nullSubscript(pos, args) {
		return nil, false
	}

	subs := make([]Expr, len(args))
	ok := true

	for i, arg := range args {
		subs[i] = a.convert(a.arithmetic(a.value(arg)), IntegerType)
		ok = ok && subs[i].Type().Kind != Invalid
	}

	return subs, ok
}

// scalar is e where a value is wanted: an aggregate reference isn't one
// (section 2.2.5.2: "no other operations ... on aggregate fields"). The name of
// a record never gets here, since name refuses it itself.
func (a *analyzer) scalar(e Expr) Expr {
	if e.Type().Kind != Record {
		return e
	}

	a.report("AGGREGATE", e.Position(), aggregateName(e))

	return a.bad(e.Position())
}

// aggregateName names an aggregate reference for a message.
func aggregateName(e Expr) string {
	switch e := e.(type) {
	case *Var:
		return e.Sym.Name
	case *ArrayRef:
		return e.Sym.Name
	case *Elem:
		return e.Sym.Name + "()"
	case *FieldRef:
		return e.Text()
	}

	return "it"
}

// structureOf is the structure of an aggregate reference, or of a reference to
// an array of records or of substructures; nil for any other expression.
func structureOf(e Expr) *Structure {
	switch e := e.(type) {
	case *Var:
		return e.Sym.Struct
	case *ArrayRef:
		return e.Sym.Struct
	case *Elem:
		return e.Sym.Struct
	case *FieldRef:
		return e.Field().Struct
	}

	return nil
}

// aggregateValue analyzes the value of an assignment to an aggregate reference:
// another one if it can be, or whatever value makes of it, for assignedValue to
// refuse. An array of records as a whole isn't one.
func (a *analyzer) aggregateValue(e ast.Expr) Expr {
	switch x := e.(type) {
	case *ast.Name:
		v := a.name(x, true)
		if _, whole := v.(*ArrayRef); whole {
			a.report("NEEDSUBSCRIPT", x.Pos, x.Name)

			return a.bad(x.Pos)
		}

		return v

	case *ast.Apply:
		return a.apply(x)

	case *ast.FieldRef:
		return a.field(x, false)
	}

	return a.value(e)
}

// checkRecordItems checks an I/O list's records (section 2.2.5.2): only an
// unformatted statement may hold one, and only as long as a descriptor can
// describe it.
func (a *analyzer) checkRecordItems(io *IO, items []*Item) {
	for _, it := range items {
		if it.Loop != nil {
			a.checkRecordItems(io, it.Loop.Items)

			continue
		}

		switch t := it.Value.Type(); {
		case t.Kind != Record:

		case io.Form != UnformattedForm:
			a.report("AGGREGATEIO", it.Value.Position(), aggregateName(it.Value))

		case t.Size > maxItemBytes:
			a.report("RECORDIOSIZE", it.Value.Position(), aggregateName(it.Value), t.Size, maxItemBytes)
		}
	}
}

// form is the structure's form as a dummy argument's must match an actual
// argument's (section 2.2.5.2: "the same number and types of fields in the same
// order. The ordering of map fields within a union declaration is irrelevant"):
// each field's place, type, and extents, in order of place, whatever it is named
// and whichever map it is in. Two structures with one form need not be one
// declaration.
func (s *Structure) form() string {
	parts := make([]string, 0, len(s.Fields))

	for _, f := range s.Fields {
		var b strings.Builder

		fmt.Fprintf(&b, "%010d:", f.Offset)

		if f.Struct != nil {
			b.WriteString("{" + f.Struct.form() + "}")
		} else {
			b.WriteString(f.Type.String())
		}

		for _, d := range f.Dims {
			fmt.Fprintf(&b, "*%d", d.Extent())
		}

		parts = append(parts, b.String())
	}

	sort.Strings(parts)

	return strconv.Itoa(s.Size) + "=" + strings.Join(parts, ",")
}

// recordArguments checks a reference's arguments against the dummy arguments of
// the file's routine it calls, for what section 6.1.1 says of records: a dummy
// declared as a record takes only an aggregate reference with a matching form,
// and a record goes to no other dummy (ours: the manual calls the argument's
// and the dummy's forms a match, and a number has none). %VAL and %DESCR choose
// how an argument goes, and a null argument has nothing to compare.
func recordArguments(r routineRef, fu *fileUnit, messages *diag.List) {
	for i, arg := range r.args {
		p := fu.params()[i]

		if arg.Expr == nil || p == nil || p.broken || arg.Expr.Type().Kind == Invalid ||
			arg.Mode == ByValue || arg.Mode == ByDescriptor {
			continue
		}

		given, wanted := structureOf(arg.Expr), p.Struct

		switch {
		case given == nil && wanted == nil:

		case given != nil && wanted != nil && given.form() == wanted.form():

		default:
			messages.Report("RECORDARG", arg.Expr.Position(), i+1, r.sym.Name, argumentKind(given, arg.Expr.Type()),
				p.Name, fu.pos.Line, argumentKind(wanted, p.Type))
		}
	}
}

// argumentKind says what an argument or a dummy argument is for RECORDARG.
func argumentKind(s *Structure, t Type) string {
	if s != nil {
		return "a record of the structure " + s.Display()
	}

	return article(t.String())
}

// recordInitials gives each record in $LOCAL or a named common block its
// structure's initial values (section 4.15.1: they are "the initial values of
// every record" of it), every element of an array of records alike, as Initials
// of the record's Symbol, which placeInitials then places as it places DATA's. A
// dummy argument's storage is its caller's, and blank common takes no initial
// values, as DATA's rule has it (DATABLANK; section 4.15.1 doesn't say).
func (a *analyzer) recordInitials() {
	for _, s := range a.unit.Symbols {
		if s.Struct == nil || s.broken || s.Dummy != 0 {
			continue
		}

		image := s.Struct.image()
		if len(image) == 0 {
			continue
		}

		if s.Common != nil && s.Common.Name == "" {
			a.report("DATABLANK", s.TypePos, s.Name)

			continue
		}

		for n := range s.Elements() {
			item := s.Name
			if s.Dims != nil {
				item = s.elementText(n)
			}

			for _, in := range image {
				a.unit.Initials = append(a.unit.Initials, &Initial{
					Sym: s, Offset: n*s.Struct.Size + in.Offset, Type: in.Type, Bytes: in.Bytes,
					Item: item + "." + in.Item, Source: in.Source, Pos: in.Pos,
				})
			}
		}
	}
}

// image is the initial values a record of the structure starts with, in order
// of place, no two sharing a byte: the structure's own field declarations', and
// each substructure field's structure's at the field's place. Two maps of a
// union may give one byte two values; the later declaration's is the record's
// (section 4.15.3), and what is left of the earlier value keeps its bytes, as
// CHARACTER pieces of them.
func (s *Structure) image() []*FieldInitial {
	// The image is worked out once per structure and kept (memoized):
	// every record of the structure, and every structure that nests it,
	// asks for the same list.
	if s.imageMade {
		return s.imageList
	}

	// A type declared inside a function is visible only there; entry pairs
	// a value with where it was declared, for sorting.
	type entry struct {
		in    *FieldInitial
		order diag.Position
	}

	var list []entry

	for _, in := range s.Initials {
		list = append(list, entry{in, in.Pos})
	}

	for _, f := range s.Fields {
		if f.Struct == nil {
			continue
		}

		// A substructure field's values are its structure's, a recursive
		// call, repeated for each element if the field is an array, each
		// copy moved to its place in this structure. "range n" over an
		// integer (Go 1.22 on) counts from 0 to n-1.
		inner := f.Struct.image()

		for n := range f.Elements() {
			item := f.Name
			if f.Dims != nil {
				item = f.Name + "(" + elementSubscripts(f.Dims, n) + ")"
			}

			for _, in := range inner {
				// *in copies the struct it points to, so changing c's
				// Offset doesn't change the inner structure's own list.
				c := *in
				c.Offset = f.Offset + n*f.Struct.Size + in.Offset
				c.Item = item + "." + in.Item
				list = append(list, entry{&c, f.Pos})
			}
		}
	}

	// In the order their fields are declared.
	sort.SliceStable(list, func(i, j int) bool {
		x, y := list[i].order, list[j].order
		if x.Line != y.Line {
			return x.Line < y.Line
		}

		return x.Column < y.Column
	})

	// Lay the values down in declaration order, each taking its bytes
	// from any earlier value it overlaps (in a UNION's maps, two fields may
	// share bytes): the last one declared wins, byte by byte.
	var kept []*FieldInitial

	for _, e := range list {
		kept = append(overlaid(kept, e.in), e.in)
	}


	sort.SliceStable(kept, func(i, j int) bool { return kept[i].Offset < kept[j].Offset })

	s.imageList, s.imageMade = kept, true

	return kept
}

// overlaid is the values of list less the bytes the value in gives: a value
// in shares none of is kept whole, and one it does is kept as the pieces of it
// before and after in's bytes.
func overlaid(list []*FieldInitial, in *FieldInitial) []*FieldInitial {
	start, end := in.Offset, in.Offset+len(in.Bytes)

	var out []*FieldInitial

	for _, k := range list {
		kStart, kEnd := k.Offset, k.Offset+len(k.Bytes)

		if kEnd <= start || kStart >= end {
			out = append(out, k)

			continue
		}

		if kStart < start {
			out = append(out, piece(k, kStart, start))
		}

		if kEnd > end {
			out = append(out, piece(k, end, kEnd))
		}
	}

	return out
}

// piece is the part of the value k from offset from to offset to, as
// CHARACTER bytes: a number cut short is no number.
func piece(k *FieldInitial, from, to int) *FieldInitial {
	b := k.Bytes[from-k.Offset : to-k.Offset]

	return &FieldInitial{
		Offset: from, Type: Type{Character, len(b)}, Bytes: b,
		Item: k.Item + " (in part)", Source: k.Source, Pos: k.Pos,
	}
}
