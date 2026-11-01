package sema

import (
	"sort"
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
// A record or a substructure as a whole (an aggregate reference: TODAY,
// NEXT_APP.APP_DATE) and an array field without subscripts are subtask 4's,
// as are records in common blocks and records as dummy arguments.

// aggregate is NOTIMPL's subject for a record or substructure as a whole.
const aggregate = "a record or substructure as a whole (an aggregate reference)"

// FieldStep is one field of a field reference, with its subscripts, each
// INTEGER, if the field is an array; nil if it isn't.
type FieldStep struct {
	Field *Field
	Subs  []Expr
}

// FieldRef is a scalar field reference (section 2.2.5.2): Sym is the record,
// or the array of records whose element Subs (each INTEGER) names, and Path
// the fields from the record's own down to the scalar field, whose type is
// the reference's. Every field but the last is a substructure.
type FieldRef struct {
	exprBase
	Sym  *Symbol
	Subs []Expr
	Path []FieldStep
}

// Field is the reference's scalar field, its last.
func (f *FieldRef) Field() *Field {
	return f.Path[len(f.Path)-1].Field
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
		}

		for _, d := range g.Entities {
			sym := a.lookup(d.Name, d.Pos)

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

			case sym.Dummy != 0:
				a.notImplForm(d.Pos, "a record as a dummy argument")

				sym.broken = true

			case sym.Common != nil:
				a.notImplForm(d.Pos, "a record in a common block")

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

// field analyzes a field reference. Only a scalar field reference is a
// value; an aggregate one, and where whole allows it (an actual argument,
// an I/O list item) an array field without subscripts, are subtask 4's.
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

		steps = append([]*ast.FieldRef{f}, steps...)
		x = f.X
	}

	ref := &FieldRef{exprBase: exprBase{Pos: e.Pos}}

	if !a.recordBase(ref, x, steps[0].Field) {
		return a.bad(e.Pos)
	}

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

	if prev.Struct != nil {
		a.notImplForm(e.Pos, aggregate)

		return a.bad(e.Pos)
	}

	ref.T = prev.Type

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
		subs, ok := a.subscripts(args)
		if !ok {
			return false
		}

		ref.Subs = subs
	}

	return true
}

// fieldSubscripts are a field's subscripts in a reference, one for each of
// its dimensions, or none for a field that isn't an array. An array field
// without them is an array name reference (section 2.2.6), which only an
// actual argument or an I/O list item may be (whole), and is subtask 4's.
func (a *analyzer) fieldSubscripts(f *Field, step *ast.FieldRef, whole bool) ([]Expr, bool) {
	switch {
	case f.Dims == nil && step.Args != nil:
		a.report("FIELDSUBS", step.Pos, f.Name, "no subscripts", len(step.Args))

		return nil, false

	case f.Dims == nil:
		return nil, true

	case step.Args == nil && whole && f.Struct == nil:
		a.notImplForm(step.Pos, "an array field as a whole")

		return nil, false

	case step.Args == nil:
		a.report("FIELDWHOLE", step.Pos, f.Name)

		return nil, false

	case len(step.Args) != len(f.Dims):
		a.report("FIELDSUBS", step.Pos, f.Name, count(len(f.Dims), "subscript"), len(step.Args))

		return nil, false
	}

	return a.subscripts(step.Args)
}

// subscripts analyzes an element's subscripts, each made INTEGER (a REAL one
// is truncated, section 2.2.3.2).
func (a *analyzer) subscripts(args []ast.Expr) ([]Expr, bool) {
	subs := make([]Expr, len(args))
	ok := true

	for i, arg := range args {
		subs[i] = a.convert(a.arithmetic(a.value(arg)), IntegerType)
		ok = ok && subs[i].Type().Kind != Invalid
	}

	return subs, ok
}

// recordInitials gives each record in $LOCAL its structure's initial values
// (section 4.15.1: they are "the initial values of every record" of it), every
// element of an array of records alike, as Initials of the record's Symbol,
// which placeInitials then places as it places DATA's.
func (a *analyzer) recordInitials() {
	for _, s := range a.unit.Symbols {
		if s.Struct == nil || s.broken || s.Dummy != 0 || s.Common != nil {
			continue
		}

		image := s.Struct.image()
		if len(image) == 0 {
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
	if s.imageMade {
		return s.imageList
	}

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

		inner := f.Struct.image()

		for n := range f.Elements() {
			item := f.Name
			if f.Dims != nil {
				item = f.Name + "(" + elementSubscripts(f.Dims, n) + ")"
			}

			for _, in := range inner {
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
