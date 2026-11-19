package sema

import (
	"strconv"

	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
)

// Derived types in DATA (docs/planning/PHASE-105 - Fortran 90 derived
// types.md, finding 12). The standard's DATA statement (5.2.9) takes two new
// things:
//
//	DATA ORIGIN / POINT(0.0, 0.0) /     an object given a structure
//	                                    constructor of constants: one value
//	                                    for the whole object, of its type
//	DATA ORIGIN%X, PTS(2)%Y / 1.0, 2.0 / a component designator: an item of
//	                                    its component's type, an array
//	                                    component every element
//
// Neither makes anything new in the image: each comes down to Initials of the
// object's Symbol, a component's place counted from the object's start, as a
// declaration's = value does (recordInitials). A STRUCTURE's record stays out
// of DATA, as the HP manual says (dataName); its fields' values are its
// structure's.

// dataSlot is one item a component designator in a DATA list names: its place
// in its object's storage, in bytes from the symbol's start, its type, and the
// item as Fortran 90 writes it, PTS(2)%Y.
type dataSlot struct {
	offset int
	t      Type
	text   string
}

// dataRecordItem gives an item of a derived type, the object at offset in s,
// the next value, which must be a constant of its type: each scalar component's
// value (an array component's per element) becomes an Initial at its place.
func (a *analyzer) dataRecordItem(s *Symbol, offset int, t Type, text string, pos diag.Position, vals *dataValues) bool {
	v, ok := vals.next(pos)
	if !ok {
		return false
	}

	if v.c == nil || v.c.T.Kind != Record || !sameType(v.c.T.Struct, t.Struct) {
		what := "a CHARACTER"

		switch {
		case v.c != nil:
			what = article(v.c.T.String())
		case v.typeless != nil, v.radix != nil:
			what = "a typeless"
		}

		a.report("DATATYPE", v.pos, what, article(t.String()))

		return false
	}

	for _, in := range a.valueInitials(v.c, v.pos) {
		a.unit.Initials = append(a.unit.Initials, &Initial{
			Sym: s, Offset: offset + in.Offset, Type: in.Type, Bytes: in.Bytes,
			Item: text + "%" + in.Item, Source: in.Source, Pos: pos,
		})
	}

	return true
}

// dataComponent takes the items a component designator names, PTS(2)%Y,
// ORIGIN%X, T%MARKS (every element), PTS%X (every element's component), giving
// each the next value.
func (a *analyzer) dataComponent(e *ast.FieldRef, vars map[string]int32, vals *dataValues) bool {
	s, slots, ok := a.dataComponentSlots(e, vars)
	if !ok {
		return false
	}

	for _, slot := range slots {
		if !a.dataItem(s, slot.offset, slot.t, slot.text, e.Pos, vals) {
			return false
		}
	}

	return true
}

// dataComponentSlots is the object a component designator starts from and the
// items it names, in array element order. Every subscript must be a constant
// expression, which may use the implied-DO lists' variables (vars). As in an
// expression, only one part may be an array without subscripts (MULTIRANK); a
// section is NOTIMPL, as it is for an array in DATA.
func (a *analyzer) dataComponentSlots(e *ast.FieldRef, vars map[string]int32) (*Symbol, []dataSlot, bool) {
	// The steps, from the object's own component out: the syntax tree nests
	// a reference from its last component in, as field's comment says.
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

	s, slots, ok := a.dataBase(x, steps[0].Field, vars)
	if !ok {
		return s, nil, false
	}

	ranked := len(slots) > 1

	for _, step := range steps {
		t := slots[0].t
		if t.Kind != Record {
			a.report("FIELDNOTRECORD", step.Pos, slots[0].text, article(t.String()), step.Field)

			return s, nil, false
		}

		f := t.Struct.Field(step.Field)

		switch {
		case f == nil:
			a.report("NOFIELD", step.Pos, t.Struct.What(), t.Struct.fieldWord(), step.Field)

			return s, nil, false

		case f.Struct == nil && f.Type.Kind == Invalid:
			return s, nil, false // its declaration's error, reported
		}

		ft := f.Type
		if f.Struct != nil {
			ft = recordType(f.Struct)
		}

		// The component's elements this step takes: one, by its subscripts,
		// or every one of an array without subscripts.
		numbers := []int{0}

		switch {
		case f.Dims == nil && step.Args != nil:
			a.report("NOTARRAY", step.Pos, f.Name)

			return s, nil, false

		case f.Dims != nil && step.Args == nil:
			if ranked {
				a.report("MULTIRANK", step.Pos, f.Name)

				return s, nil, false
			}

			ranked = true
			numbers = numbers[:0]

			for n := range f.Elements() {
				numbers = append(numbers, n)
			}

		case f.Dims != nil:
			n, ok := a.dataFieldElement(f, step, vars)
			if !ok {
				return s, nil, false
			}

			numbers[0] = n
		}

		var next []dataSlot

		for _, slot := range slots {
			for _, n := range numbers {
				text := slot.text + "%" + f.Name
				if f.Dims != nil {
					text += "(" + elementSubscripts(f.Dims, n) + ")"
				}

				next = append(next, dataSlot{slot.offset + f.Offset + n*f.ElementSize(), ft, text})
			}
		}

		slots = next
	}

	return s, slots, true
}

// dataBase is the object a component designator in a DATA list starts from,
// x, and its items: a derived-type object, an element of an array of them, or
// a whole array of them (every element). first is the component named after
// it, for the message if x isn't one.
func (a *analyzer) dataBase(x ast.Expr, first string, vars map[string]int32) (*Symbol, []dataSlot, bool) {
	switch x := x.(type) {
	case *ast.Name:
		s, ok := a.dataName(x.Name, x.Pos)
		if !ok || !a.dataRecord(s, x.Pos, first) {
			return s, nil, false
		}

		if s.Class != Array {
			return s, []dataSlot{{0, s.Type, s.Name}}, true
		}

		if s.Size() == 0 {
			a.report("DATASIZE", x.Pos, s.Name)

			return s, nil, false
		}

		slots := make([]dataSlot, 0, s.Elements())

		for n := range s.Elements() {
			slots = append(slots, dataSlot{n * s.Type.Size, s.Type, s.elementText(n)})
		}

		return s, slots, true

	case *ast.Apply:
		s, number, ok := a.dataElement(x, vars)
		if !ok || !a.dataRecord(s, x.Pos, first) {
			return s, nil, false
		}

		return s, []dataSlot{{number * s.Type.Size, s.Type, s.elementText(number)}}, true
	}

	a.report("DATAFORM", x.Position())

	return nil, nil, false
}

// dataRecord reports whether s, a name in a DATA list with a component after
// it, is a derived-type object, NOTRECORD if not. dataName has refused a
// STRUCTURE's record.
func (a *analyzer) dataRecord(s *Symbol, pos diag.Position, first string) bool {
	if s.Struct != nil {
		return true
	}

	what := a.describe(s)
	if s.Class == Unclassified {
		what = Variable.String()
	}

	a.report("NOTRECORD", pos, s.Name, what, first)

	return false
}

// dataFieldElement is the element number, from 0, of the array component f
// that step's constant subscripts name.
func (a *analyzer) dataFieldElement(f *Field, step *ast.FieldRef, vars map[string]int32) (int, bool) {
	if hasTriplet(step.Args) {
		a.notImplForm(step.Pos, "an array section")

		return 0, false
	}

	if len(step.Args) != len(f.Dims) {
		a.report("SUBSCRIPTS", step.Pos, f.Name, count(len(f.Dims), "subscript"), len(step.Args))

		return 0, false
	}

	if a.nullSubscript(step.Pos, step.Args) {
		return 0, false
	}

	subs := make([]int, len(step.Args))

	for i, arg := range step.Args {
		v, ok := a.dataInt(arg, vars)
		if !ok {
			return 0, false
		}

		subs[i] = int(v)
	}

	// The field's dimensions are a symbol's for ElementNumber's arithmetic.
	number, ok := (&Symbol{Dims: f.Dims}).ElementNumber(subs)
	if !ok {
		a.report("DATARANGE", step.Pos, f.Name)

		return 0, false
	}

	return number, true
}

// dataComponentSubstring is a substring of a component designator in a DATA
// list, ORIGIN%NAME(2:4): the one scalar CHARACTER item it names, for
// dataSubstring to cut.
func (a *analyzer) dataComponentSubstring(e *ast.FieldRef, vars map[string]int32) (*Symbol, dataSlot, bool) {
	s, slots, ok := a.dataComponentSlots(e, vars)
	if !ok {
		return s, dataSlot{}, false
	}

	if len(slots) != 1 {
		a.report("NEEDSUBSCRIPT", e.Pos, slots[0].text)

		return s, dataSlot{}, false
	}

	return s, slots[0], true
}

// derivedField reports whether e, a field reference, starts from a derived-type
// object. DATA and EQUIVALENCE refuse any other, a STRUCTURE's record's field
// among them, with their old messages, DATAFORM and EQUIVFORM, as before
// derived types.
func (a *analyzer) derivedField(e *ast.FieldRef) bool {
	x := ast.Expr(e)

	for {
		f, ok := x.(*ast.FieldRef)
		if !ok {
			break
		}

		x = f.X
	}

	var name string

	switch x := x.(type) {
	case *ast.Name:
		name = x.Name
	case *ast.Apply:
		name = x.Name
	default:
		return false
	}

	s := a.scope.find(name)

	return s != nil && s.Struct != nil && s.Struct.Derived
}

// substringText is item(first:last), a substring item's text.
func substringText(item string, first, last int) string {
	return item + "(" + strconv.Itoa(first) + ":" + strconv.Itoa(last) + ")"
}
