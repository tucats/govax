package sema

import "github.com/tucats/govax/internal/fortran/diag"

// Derived types in I/O (docs/planning/PHASE-105 - Fortran 90 derived types.md,
// findings 13 and 14). FTNRTL knows nothing of derived types: the compiler
// takes an object apart into its ultimate components, the components of an
// intrinsic type that are left once every component of a derived type has been
// taken apart in turn (the standard's 9.4.2), and gives FTNRTL those.
//
//   - A formatted, list-directed, or internal file statement's item of a
//     derived type is its ultimate components in order, each an item of its
//     own (Components; lowering calls it, ir/lowerio.go, an array of a type an
//     element at a time, each element's components together).
//   - An unformatted statement's item is the object's bytes, padding and all:
//     the standard calls the form processor-dependent, and this is a record's.
//   - A NAMELIST group's object of a derived type is an entity per ultimate
//     component, named P%X or P%C%Y, after a header entity for the object and
//     one for each component of a derived type (namelistObject), which tells
//     FTNRTL which entities P = 1.0, 2.0 fills, one after another.

// Components are the scalar derived-type object e's ultimate components in
// order, each a reference to it (a *FieldRef, an array component a whole array
// field, an array component of a derived type every element of it, a ranked
// reference) or a value (a constructor's or a constant's own). ok is false for
// an object that isn't a place, a constructor, or a constant (a function's
// value, MERGE's): lowering puts it in a place first and asks again.
func Components(e Expr) (parts []Expr, ok bool) {
	switch v := e.(type) {
	case *StructValue:
		for _, x := range v.Values {
			parts = append(parts, ultimate(x)...)
		}

		return parts, true

	case *Const:
		for _, x := range v.Parts {
			parts = append(parts, ultimate(x)...)
		}

		return parts, v.Parts != nil

	case *Var, *Elem:

	case *FieldRef:
		if v.S != nil || v.WholeArray() {
			return nil, false
		}

	default:
		return nil, false
	}

	for _, f := range e.Type().Struct.Fields {
		parts = append(parts, ultimate(fieldOf(e, f))...)
	}

	return parts, true
}

// Shaped is the I/O list's item e with its shape: a whole array (an array
// name, or an array field without subscripts), which an I/O list holds without
// one (it is passed whole, by descriptor), as the array of every element.
func Shaped(e Expr) Expr {
	switch r := e.(type) {
	case *ArrayRef:
		if r.S == nil {
			return NewArrayRef(r.Sym, r.Pos)
		}

	case *FieldRef:
		if r.WholeArray() {
			return r.wholeRanked()
		}
	}

	return e
}

// ultimate is the component x as items: its own components if it is a scalar
// of a derived type, else x itself (an array of a derived type stays whole, for
// lowering to take an element at a time).
func ultimate(x Expr) []Expr {
	if x.Type().Kind != Record || len(x.Shape()) > 0 {
		return []Expr{x}
	}

	parts, ok := Components(x)
	if !ok {
		return []Expr{x}
	}

	return parts
}

// fieldOf is component f of the scalar object base (a Var, an Elem, or a
// scalar FieldRef of a derived type): a whole array field if f is an array of
// an intrinsic type, every element of it (ranked) if an array of a derived type.
func fieldOf(base Expr, f *Field) Expr {
	t := f.Type
	if f.Struct != nil {
		t = recordType(f.Struct)
	}

	ref := &FieldRef{exprBase: exprBase{T: t, Pos: base.Position()}, NamePos: base.Position()}

	switch b := base.(type) {
	case *Var:
		ref.Sym = b.Sym
	case *Elem:
		ref.Sym, ref.Subs = b.Sym, b.Subs
	case *FieldRef:
		ref.Sym, ref.Subs = b.Sym, b.Subs
		ref.Path = append(ref.Path, b.Path...)
	}

	ref.Path = append(ref.Path, FieldStep{Field: f})

	if f.Struct != nil && f.Dims != nil {
		return ref.wholeRanked()
	}

	return ref
}

// NamelistEntity is one entity of a NAMELIST group's block as FTNRTL is
// given it: a name, and the variable, array, or component it names (Ref), or
// for a header (Ref nil) the number of entities after it that make up the
// object Name names, Parts, nested headers among them.
type NamelistEntity struct {
	Name  string
	Ref   Expr
	Parts int
}

// namelistObject adds the entities of a scalar object of a derived type to
// group n: a header named name, then for each component its own entity
// (name%component), or if the component is of a derived type its header and
// entities in turn. An array of a derived type among them is NOTIMPL: FTNRTL
// would need an element's subscripts inside a name (PTS(2)%X), which waits for
// Phase 109's NAMELIST work.
func (a *analyzer) namelistObject(n *Namelist, name string, base Expr, pos diag.Position) bool {
	at := len(n.Entities)
	n.Entities = append(n.Entities, NamelistEntity{Name: name})

	for _, f := range base.Type().Struct.Fields {
		part := name + "%" + f.Name

		switch {
		case f.Struct != nil && f.Dims != nil:
			a.notImplForm(pos, "an array of a derived type in a NAMELIST group")

			return false

		case f.Struct != nil:
			if !a.namelistObject(n, part, fieldOf(base, f), pos) {
				return false
			}

		default:
			n.Entities = append(n.Entities, NamelistEntity{Name: part, Ref: fieldOf(base, f)})
		}
	}

	n.Entities[at].Parts = len(n.Entities) - at - 1

	return true
}
