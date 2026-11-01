package sema

import (
	"fmt"
	"strconv"

	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
)

// Structure declarations (sections 2.2.5 and 4.15). A structure
// declaration describes the form of a record, as a type statement
// describes a variable's, but makes no variable: RECORD statements do
// that (subtask 3). Its body declares fields, each a typed item, an
// array of them, or a substructure (another structure's form, by a
// RECORD statement or a structure declared inside it), and unions,
// whose maps are alternative layouts of one stretch of the record.
//
// A structure is laid out packed (section 2.2.5.1): each field's offset
// is the sum of the lengths of the fields before it, with no gaps for
// alignment, so the VAX may reach a longword at an odd address (which
// it can, more slowly). A union takes as many bytes as its longest
// map, every map starting at the union's offset. The manual's example
//
//	STRUCTURE /STR/
//	  INTEGER*4 TAG
//	  UNION
//	    MAP
//	      REAL*4 FLT
//	      CHARACTER*2 CHR
//	    END MAP
//	    MAP
//	      INTEGER*2 INT
//	    END MAP
//	  END UNION
//	  LOGICAL*1 LOG
//	END STRUCTURE
//
// puts TAG at 0, FLT at 4 and CHR at 8 (the first map is 6 bytes), INT
// at 4 too, and LOG at 10, after the union's 6 bytes: 11 bytes in all.
//
// Structures have names of their own, apart from the unit's variables,
// arrays, and common blocks (section 4.15.1), so they are kept in their
// own table, not the symbol table: a structure, a field, and a variable
// may all be named X.

// Structure is one structure declaration's form.
type Structure struct {
	// Name is the structure's name, or "" for a nested structure
	// declared without one, which only its fields have.
	Name string
	Pos  diag.Position

	// Fields are every field, in the order declared, a union's maps'
	// fields among them: a map has no name, so its fields are referenced
	// as the structure's own (R.FLT). %FILL fields are here too, named
	// "%FILL".
	Fields []*Field

	// Size is the structure's length in bytes, which every record of it
	// has.
	Size int

	// Initials are the initial values its field declarations give, in
	// the order they are declared. Two in different maps of a union may
	// give the same bytes; the later one is what a record holds (section
	// 4.15.3).
	Initials []*FieldInitial

	byName map[string]*Field

	// imageList is a record's initial values, image's, made at its
	// first call (imageMade).
	imageList []*FieldInitial
	imageMade bool

	// open is set while the structure's body is being read, when a
	// RECORD statement inside it can't name it: a structure can't
	// contain itself (section 4.15.2).
	open bool
}

// Field is one of a structure's fields.
type Field struct {
	// Name is the field's name, or "%FILL" for an unnamed field, which
	// only takes space.
	Name string
	Pos  diag.Position

	// Type is a typed field's type (an array field's elements'), and
	// Struct a substructure's structure; exactly one is set.
	Type   Type
	Struct *Structure

	// Dims are an array field's dimensions, all constant (section
	// 4.15.1: no adjustable or assumed-size fields); nil for a field that
	// isn't an array.
	Dims []Dim

	// Offset is the field's place in its structure, in bytes from the
	// structure's start.
	Offset int

	// Union is the innermost union the field is in, and Map which of its
	// maps (from 1); nil and 0 for a field outside every union.
	Union *Union
	Map   int
}

// Union is one union declaration in a structure: Offset is where its
// maps start, Size its longest map's length, and Maps how many it has.
type Union struct {
	Offset, Size, Maps int
	Pos                diag.Position
}

// FieldInitial is the initial value a field declaration gives one item
// of the structure: a scalar field, or one element of an array field. A
// record of the structure starts with these bytes at Offset in it.
type FieldInitial struct {
	Offset int
	Type   Type
	Bytes  []byte

	// Item is the item as FORTRAN writes it, TAG or ARR(2), and Source
	// the constant as the program writes it, for comments.
	Item, Source string
	Pos          diag.Position
}

// Field returns the structure's field named name, or nil. A %FILL field
// has no name to find it by.
func (s *Structure) Field(name string) *Field {
	return s.byName[name]
}

// Display is the structure's name as messages and maps show it after
// "structure": /DATE/, or for a nested structure without a name "at
// line 8".
func (s *Structure) Display() string {
	if s.Name == "" {
		return "at line " + strconv.Itoa(s.Pos.Line)
	}

	return "/" + s.Name + "/"
}

// ElementSize is the length in bytes of one of the field's items: its
// type's, or its substructure's.
func (f *Field) ElementSize() int {
	if f.Struct != nil {
		return f.Struct.Size
	}

	return f.Type.Size
}

// Elements is how many items the field has: 1, or an array field's
// element count.
func (f *Field) Elements() int {
	n := 1
	for _, d := range f.Dims {
		n *= d.Extent()
	}

	return n
}

// Size is the field's length in bytes.
func (f *Field) Size() int {
	return f.ElementSize() * f.Elements()
}

// TypeText is the field's type as a map shows it, with an array field's
// bounds: LOGICAL*1, CHARACTER*20 (1:4), /DATE/, /TIME/ (1:2), and for
// a structure without a name (structure at line 8).
func (f *Field) TypeText() string {
	text := f.Type.String()

	switch {
	case f.Struct != nil && f.Struct.Name == "":
		text = "(structure " + f.Struct.Display() + ")"
	case f.Struct != nil:
		text = f.Struct.Display()
	}

	if f.Dims != nil {
		text += " (" + dims(f.Dims) + ")"
	}

	return text
}

// maxStructureBytes is the longest structure: a record's offsets and
// lengths must fit in a longword, as an array's do.
const maxStructureBytes = maxArrayBytes

// operatorNames are the names a field shouldn't have (section 2.2.5.2's
// note): a reference R.EQ.X reads as a comparison, since the operators
// and logical constants are written between periods too.
var operatorNames = map[string]bool{
	"EQ": true, "NE": true, "LT": true, "LE": true, "GT": true, "GE": true,
	"AND": true, "OR": true, "NOT": true, "EQV": true, "NEQV": true, "XOR": true,
	"TRUE": true, "FALSE": true,
}

// structureStatement reads a structure declaration at a unit's level: its
// name must be given, and it may have no field list, which only a nested
// structure has (section 4.15.1). It goes in the unit's table, which
// RECORD statements look in.
func (a *analyzer) structureStatement(s *ast.StructureStmt) {
	switch {
	case s.Name == "":
		a.report("STRUCTNAME", s.Position())
	case len(s.Fields) > 0:
		a.report("STRUCTFIELDS", s.Fields[0].Pos, "the outermost structure declaration can't have a field list")
	}

	a.declareStructure(s)
}

// declareStructure makes the structure s declares, enters it in the
// unit's table if it has a name, and reads its body.
func (a *analyzer) declareStructure(s *ast.StructureStmt) *Structure {
	st := &Structure{Name: s.Name, Pos: s.Position(), byName: map[string]*Field{}, open: true}

	if s.Name != "" {
		if old := a.unit.structs[s.Name]; old != nil {
			a.report("DUPSTRUCT", s.Position(), s.Name, old.Pos.Line)
		} else {
			a.unit.structs[s.Name] = st
		}
	}

	a.unit.Structures = append(a.unit.Structures, st)

	b := &structBuilder{a: a, s: st}
	b.body(s.Body)

	st.Size, st.open = b.offset, false

	if !b.declares {
		a.report("STRUCTEMPTY", s.Position())
	}

	return st
}

// structBuilder lays out one structure as its body is read: offset is the
// next field's place, and union and mapNumber the innermost union being
// read and which of its maps.
type structBuilder struct {
	a      *analyzer
	s      *Structure
	offset int

	union     *Union
	mapNumber int

	// tooLarge is set once the structure has grown past
	// maxStructureBytes and said so.
	tooLarge bool

	// declares is set once the body has a statement that declares
	// fields, which a structure must (section 4.15), even if its fields
	// had errors.
	declares bool
}

// body reads a structure's or a map's statements. The parser has already
// reported any statement that can't be there (FIELDSTMT, NOTINBLOCK), and
// left it in the body, so it is passed over.
func (b *structBuilder) body(list []ast.Stmt) {
	for _, s := range list {
		switch s.(type) {
		case *ast.TypeStmt, *ast.RecordStmt, *ast.StructureStmt, *ast.UnionStmt, *ast.BadStmt:
			b.declares = true
		}

		switch s := s.(type) {
		case *ast.TypeStmt:
			b.typeStatement(s)

		case *ast.RecordStmt:
			b.recordStatement(s)

		case *ast.StructureStmt:
			b.nested(s)

		case *ast.UnionStmt:
			b.unionStatement(s)

		case *ast.ParameterStmt:
			// Section 4.15: "the same effect as if [it] appeared outside
			// the block": a constant of the unit.
			for _, d := range s.Defs {
				b.a.parameter(s, d)
			}

		case *ast.DirectiveStmt:
			b.a.directive(s)
		}
	}
}

// typeStatement declares a typed field for each entity of a type
// statement, with its initial values. A length after a name is that
// field's, as in a type statement outside (CHARACTER*8 A, B*4).
func (b *structBuilder) typeStatement(s *ast.TypeStmt) {
	for _, d := range s.Entities {
		l := s.Type.Len
		pos := s.Type.Pos

		if d.HasLength() {
			l, pos = d.Len, d.Pos
		}

		t, ok := b.a.typeOf(s.Type, l, pos)
		if ok && t.Kind == Character && t.Size == 0 {
			b.a.report("FIELDLEN", d.Pos, d.Name)

			ok = false
		}

		f := &Field{Name: d.Name, Pos: d.Pos, Type: t}

		dimsOK := b.fieldDims(f, d)

		if !ok || !dimsOK {
			// Nothing more to say about it, nor a size to lay it out by.
			// The name is still the structure's, so it isn't reported
			// again at each use.
			f.Type = invalid
			f.Dims = nil
		}

		b.add(f)

		if d.Init != nil && ok && dimsOK {
			b.initialValues(f, d)
		}
	}
}

// recordStatement declares a substructure field for each entity of a
// RECORD statement in a structure: the named structure's form, which must
// already be declared and complete (sections 4.15.1, 4.15.2).
func (b *structBuilder) recordStatement(s *ast.RecordStmt) {
	for _, g := range s.Groups {
		st := b.a.unit.structs[g.Structure]

		switch {
		case st == nil:
			b.a.report("NOSTRUCT", g.Pos, g.Structure)
		case st.open:
			b.a.report("RECURSTRUCT", g.Pos, g.Structure)

			st = nil
		}

		for _, d := range g.Entities {
			f := &Field{Name: d.Name, Pos: d.Pos, Struct: st}
			if st == nil || !b.fieldDims(f, d) {
				f.Struct, f.Type, f.Dims = nil, invalid, nil
			}

			b.add(f)
		}
	}
}

// nested declares a structure inside this one and a field of it for each
// name in its field list, which it must have: its fields are how a record
// reaches it (section 4.15.2). Its own layout starts at 0, as every
// structure's does; its fields' place in this one is the field's offset.
func (b *structBuilder) nested(s *ast.StructureStmt) {
	if len(s.Fields) == 0 {
		b.a.report("STRUCTFIELDS", s.Position(), "a nested structure declaration needs a field list")
	}

	st := b.a.declareStructure(s)

	for _, d := range s.Fields {
		f := &Field{Name: d.Name, Pos: d.Pos, Struct: st}
		if !b.fieldDims(f, d) {
			f.Struct, f.Type, f.Dims = nil, invalid, nil
		}

		b.add(f)
	}
}

// unionStatement lays out a union: each map from the union's offset, the
// union as long as its longest map (section 4.15.3).
func (b *structBuilder) unionStatement(s *ast.UnionStmt) {
	u := &Union{Offset: b.offset, Pos: s.Position()}
	outer, outerMap := b.union, b.mapNumber

	for _, m := range s.Body {
		if d, ok := m.(*ast.DirectiveStmt); ok {
			b.a.directive(d)
		}

		m, ok := m.(*ast.MapStmt)
		if !ok {
			continue
		}

		u.Maps++
		b.union, b.mapNumber, b.offset = u, u.Maps, u.Offset

		b.body(m.Body)

		u.Size = max(u.Size, b.offset-u.Offset)
	}

	b.union, b.mapNumber = outer, outerMap
	b.offset = u.Offset + u.Size
}

// fieldDims gives f the dimensions its declarator d declares, which must
// be constant (section 4.15.1). It reports false after an error,
// reported.
func (b *structBuilder) fieldDims(f *Field, d *ast.Declarator) bool {
	if d.Dims == nil {
		return true
	}

	if len(d.Dims) > maxDimensions {
		b.a.report("TOOMANYDIMS", d.Pos, len(d.Dims))

		return false
	}

	f.Dims = make([]Dim, len(d.Dims))

	for i, bd := range d.Dims {
		dim := Dim{Lower: 1}

		if bd.Star {
			b.a.report("FIELDDIM", d.Pos, d.Name)

			return false
		}

		if bd.Lower != nil {
			v, ok := b.constantBound(bd.Lower, d)
			if !ok {
				return false
			}

			dim.Lower = v
		}

		v, ok := b.constantBound(bd.Upper, d)
		if !ok {
			return false
		}

		dim.Upper = v

		if dim.Upper < dim.Lower {
			b.a.report("BOUNDORDER", bd.Upper.Position(), dim.Upper, dim.Lower)

			return false
		}

		f.Dims[i] = dim
	}

	return true
}

// constantBound is a field's array bound, which must be a constant
// expression; it reports false after an error, reported.
func (b *structBuilder) constantBound(e ast.Expr, d *ast.Declarator) (int, bool) {
	ev := b.a.declEval()

	c, err := ev.eval(e)

	switch {
	case err != nil && err.kind == constFailed:
		return 0, false
	case err != nil:
		b.a.report("FIELDDIM", d.Pos, d.Name)

		return 0, false
	}

	return b.a.constantBound(ev, c, e.Position())
}

// add lays out the field f at the next offset and enters its name, which
// must be new at this level of the structure (section 4.15.1). A union's
// maps' fields are at their structure's level: R.FLT and R.INT name two
// maps' fields of one record.
func (b *structBuilder) add(f *Field) {
	f.Offset, f.Union, f.Map = b.offset, b.union, b.mapNumber

	if f.Name != "%FILL" {
		if old := b.s.byName[f.Name]; old != nil {
			b.a.report("DUPFIELD", f.Pos, f.Name, old.Pos.Line)
		} else {
			b.s.byName[f.Name] = f
		}

		if operatorNames[f.Name] {
			b.a.report("FIELDOP", f.Pos, f.Name)
		}
	}

	b.s.Fields = append(b.s.Fields, f)

	size := int64(f.ElementSize()) * int64(f.Elements())
	if int64(b.offset)+size > maxStructureBytes {
		if !b.tooLarge {
			b.a.report("STRUCTSIZE", f.Pos, b.s.Display())
		}

		b.tooLarge = true

		return
	}

	b.offset += int(size)
}

// initialValues reads the initial values a field declaration gives its
// field, one for a scalar and one for each element of an array, in storage
// order, as a type statement's are (section 4.15.1). %FILL can't have any.
func (b *structBuilder) initialValues(f *Field, d *ast.Declarator) {
	a := b.a

	if f.Name == "%FILL" {
		a.report("FILLINIT", d.InitPos)

		return
	}

	vals := &dataValues{a: a, list: d.Init}

	for n := range f.Elements() {
		v, ok := vals.next(d.Pos)
		if !ok {
			return
		}

		bytes, ok := a.initialBytes(v, f.Type)
		if !ok {
			return
		}

		item := f.Name
		if f.Dims != nil {
			item = f.Name + "(" + elementSubscripts(f.Dims, n) + ")"
		}

		b.s.Initials = append(b.s.Initials, &FieldInitial{
			Offset: f.Offset + n*f.Type.Size, Type: f.Type, Bytes: bytes, Item: item, Source: v.source, Pos: d.Pos,
		})
	}

	if pos, more := vals.more(); more {
		a.report("DATACOUNT", pos, "more values than names")
	}
}

// elementSubscripts are the subscripts of element n, in storage order, of
// an array of dimensions dims: "2,1".
func elementSubscripts(dims []Dim, n int) string {
	text := ""

	for i, d := range dims {
		if i > 0 {
			text += ","
		}

		text += strconv.Itoa(d.Lower + n%d.Extent())
		n /= d.Extent()
	}

	return text
}

// dumpStructure is a structure's lines in the dump: its name and size,
// then a line per field (its offset, and for a union's field which map it
// is in), then its initial values.
func dumpStructure(s *Structure) string {
	text := fmt.Sprintf("  structure %s is %d bytes\n", s.Display(), s.Size)

	for _, f := range s.Fields {
		where := ""
		if f.Union != nil {
			where = fmt.Sprintf("  map %d of the union at +%d", f.Map, f.Union.Offset)
		}

		text += fmt.Sprintf("    %-10s  %-18s  +%d%s\n", f.Name, f.TypeText(), f.Offset, where)
	}

	for _, in := range s.Initials {
		text += fmt.Sprintf("    initial +%d %s %s = % X\n", in.Offset, in.Item, in.Type, in.Bytes)
	}

	return text
}
