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
//     FTNRTL which entities P = 1.0, 2.0 fills, one after another. An array of
//     a derived type is the same, each entity every element's (PTS%X), which
//     FTNRTL is given as a class NCA descriptor (namelistArray, namelistRanked;
//     docs/planning/PHASE-109 - Fortran 90 IO.md, subtask 6b).

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
//
// An entity under an array of a derived type (docs/planning/PHASE-109 -
// Fortran 90 IO.md, subtask 6b) has Ranks instead of a Ref: it is every element
// of a component across the arrays its designator goes through (PTS%X, every
// element's X), which FTNRTL is given as a class NCA descriptor. Sym is the
// group's object, Offset the component's place in bytes from the object's start
// with every subscript at its lower bound, Type its elements' type (a header's
// the record's), and Ranks its dimensions, the fastest first: the component's
// own, then each array's it is in, the innermost first. A header with Ranks
// has a shape FTNRTL reads its rank and bounds from.
type NamelistEntity struct {
	Name  string
	Ref   Expr
	Parts int

	Sym    *Symbol
	Offset int
	Type   Type
	Ranks  []NamelistDim
}

// NamelistDim is one dimension of a ranked namelist entity: its bounds, and the
// distance in bytes from one element to the next along it.
type NamelistDim struct {
	Lower, Upper, Stride int
}

// The limits FTNRTL's NAMELIST input keeps to (mar/ftn_namelist.mar's frame):
// how many dimensions an entity's elements may have in all, and how many objects
// of a derived type deep a component may be.
const (
	namelistMaxRank  = 15
	namelistMaxDepth = 8
)

// ranksOf is the dimensions dims of an array whose elements are size bytes
// apart, the fastest first, in front of outer.
func ranksOf(dims []Dim, size int, outer []NamelistDim) []NamelistDim {
	ranks := make([]NamelistDim, 0, len(dims)+len(outer))
	stride := size

	for _, d := range dims {
		ranks = append(ranks, NamelistDim{Lower: d.Lower, Upper: d.Upper, Stride: stride})
		stride *= d.Upper - d.Lower + 1
	}

	return append(ranks, outer...)
}

// namelistObject adds the entities of an object of a derived type to group n:
// a header named name, then for each component its own entity
// (name%component), or if the component is of a derived type its header and
// entities in turn. base is the object while no array of a derived type has
// been gone through (Ranks empty), and its entities are references as before;
// past one, each entity is described by sym, offset, and ranks (namelistRanked).
func (a *analyzer) namelistObject(n *Namelist, name string, base Expr, pos diag.Position) bool {
	return a.namelistRanked(n, name, base, base.(*Var).Sym, 0, base.Type(), nil, 1, pos)
}

// namelistRanked is namelistObject's walk: the object named name, of type t,
// is offset bytes into sym, and while it has no ranks it is base too; depth is
// how many objects deep it is.
func (a *analyzer) namelistRanked(n *Namelist, name string, base Expr, sym *Symbol, offset int, t Type,
	ranks []NamelistDim, depth int, pos diag.Position) bool {
	if depth > namelistMaxDepth || len(ranks) > namelistMaxRank {
		a.notImplForm(pos, "a NAMELIST object this deep in arrays and derived types")

		return false
	}

	at := len(n.Entities)
	header := NamelistEntity{Name: name}
	if len(ranks) > 0 {
		header = NamelistEntity{Name: name, Sym: sym, Offset: offset, Type: t, Ranks: ranks}
	}

	n.Entities = append(n.Entities, header)

	for _, f := range t.Struct.Fields {
		part := name + "%" + f.Name

		size := f.Type.Size
		ft := f.Type

		if f.Struct != nil {
			size, ft = f.Struct.Size, recordType(f.Struct)
		}

		own := ranksOf(f.Dims, size, ranks)

		switch {
		case f.Struct != nil && len(own) == 0:
			if !a.namelistRanked(n, part, fieldOf(base, f), sym, offset+f.Offset, ft, nil, depth+1, pos) {
				return false
			}

		case f.Struct != nil:
			if !a.namelistRanked(n, part, nil, sym, offset+f.Offset, ft, own, depth+1, pos) {
				return false
			}

		case len(ranks) == 0:
			n.Entities = append(n.Entities, NamelistEntity{Name: part, Ref: fieldOf(base, f)})

		default:
			if len(own) > namelistMaxRank {
				a.notImplForm(pos, "a NAMELIST object this deep in arrays and derived types")

				return false
			}

			n.Entities = append(n.Entities, NamelistEntity{Name: part, Sym: sym, Offset: offset + f.Offset, Type: ft, Ranks: own})
		}
	}

	n.Entities[at].Parts = len(n.Entities) - at - 1

	return true
}

// namelistArray adds the entities of s, an array of a derived type, to group n.
func (a *analyzer) namelistArray(n *Namelist, s *Symbol, pos diag.Position) bool {
	return a.namelistRanked(n, s.Name, nil, s, 0, s.Type, ranksOf(s.Dims, s.Type.Size, nil), 1, pos)
}
