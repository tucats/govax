package sema

import (
	"slices"
	"strconv"

	"github.com/tucats/govax/internal/fortran/ast"
)

// The transformational functions whose values are computed into a temporary
// before an array statement's loops (the Fortran 90 standard's 13.13; docs/
// planning/PHASE-104 - Fortran 90 dynamic arrays.md, finding 13, subtask 8):
//
//	SPREAD(SOURCE, DIM, NCOPIES)        SOURCE copied NCOPIES times along a new
//	                                    dimension DIM
//	CSHIFT(ARRAY, SHIFT, DIM)           ARRAY's elements moved SHIFT places along
//	                                    DIM, those off one end coming in at the
//	                                    other
//	EOSHIFT(ARRAY, SHIFT, BOUNDARY, DIM) the same, BOUNDARY (or 0, .FALSE., or
//	                                    blanks) coming in instead
//	PACK(ARRAY, MASK, VECTOR)           ARRAY's elements where MASK holds, in
//	                                    array element order, then VECTOR's past
//	                                    them
//	UNPACK(VECTOR, MASK, FIELD)         VECTOR's elements in order where MASK
//	                                    holds, FIELD's elsewhere
//	RESHAPE(SOURCE, SHAPE, PAD, ORDER)  of values not constant (constarray.go
//	                                    folds the rest)
//
// and the reductions with a DIM= that isn't constant on an array of rank above
// 1, whose result's shape depends on DIM's value. Each is an ArrayFunc that
// isn't Element(): lowering computes the whole value into a temporary of its
// shape, known when compiling or only then (ir/lowertransform.go), and the
// statement's loops read that. A result extent not known when compiling is the
// function's own (Extent.Of is the ArrayFunc), read from its temporary.
//
// SHIFT may be an array of the rank below ARRAY's (each line along DIM shifted
// its own amount), as may BOUNDARY; MASK may be a scalar for PACK. DIM must be
// constant for SPREAD, CSHIFT, and EOSHIFT (ours: NOTIMPL otherwise), and
// RESHAPE's SHAPE must have a size known when compiling (the standard's
// constraint) and ORDER be constant (ours).

// transformational reports whether name is one of the functions this file
// analyzes.
func transformational(name string) bool {
	switch name {
	case "SPREAD", "CSHIFT", "EOSHIFT", "PACK", "UNPACK", "RESHAPE":
		return true
	}

	return false
}

// transform analyzes a reference to one of the transformational functions, its
// arguments in its dummies' order, nil for one left out.
func (a *analyzer) transform(e *ast.Apply, args []ast.Expr) Expr {
	arg := func(i int) ast.Expr {
		if i < len(args) {
			return args[i]
		}

		return nil
	}

	f := &ArrayFunc{exprBase: exprBase{Pos: e.Pos}, Name: e.Name}

	value := func(i int) Expr {
		if arg(i) == nil {
			return nil
		}

		return a.arrayValue(arg(i))
	}

	// array is argument i, which must be an array.
	array := func(i int, what string) Expr {
		x := value(i)
		if x == nil || x.Type().Kind == Invalid {
			return nil
		}

		if rankOf(x) == 0 {
			a.report("ARRAYARG", x.Position(), e.Name, what)

			return nil
		}

		return x
	}

	var ok bool

	switch e.Name {
	case "SPREAD":
		ok = a.spread(f, value(0), arg(1), value(2))
	case "CSHIFT", "EOSHIFT":
		ok = a.shift(f, array(0, "ARRAY"), arg, value)
	case "PACK":
		ok = a.pack(f, array(0, "ARRAY"), value(1), value(2))
	case "UNPACK":
		ok = a.unpack(f, array(0, "VECTOR"), value(1), value(2))
	case "RESHAPE":
		ok = a.reshapeValues(f, e, array(0, "SOURCE"), value(1), value(2), arg(3))
	}

	if !ok {
		return a.bad(e.Pos)
	}

	// A CHARACTER value of a length known only when the program runs takes it
	// from its argument, which lowering can find only as a name's.
	if f.T.Kind == Character && f.T.Size == 0 {
		switch f.Array.(type) {
		case *ArrayRef, *Section:
		default:
			a.notImplForm(e.Pos, e.Name+" of a CHARACTER expression whose length is known only when the program runs")

			return a.bad(e.Pos)
		}
	}

	// An extent known only when the program runs is the function's own,
	// read from its temporary. (f.S is the function's own slice: an argument's
	// shape, written over here, would point at the function.)
	for i := range f.S {
		if !f.S[i].Known {
			f.S[i] = Extent{Of: f, Dim: i}
		}
	}

	return f
}

// constantDim is DIM= of a function whose result's shape depends on it,
// SPREAD's, CSHIFT's, or EOSHIFT's, from 1 to rank: its value, or 0 after a
// message.
func (a *analyzer) constantDim(f *ArrayFunc, x ast.Expr, rank int) int {
	d := a.convert(a.arithmetic(a.value(x)), IntegerType)
	if d.Type().Kind == Invalid {
		return 0
	}

	v, ok := constantInt(d)

	switch {
	case !ok:
		a.notImplForm(d.Position(), f.Name+" with a DIM= not constant")

		return 0

	case v < 1 || v > rank:
		a.report("DIMRANGE", d.Position(), f.Name, ast.DumpExpr(x), rank)

		return 0
	}

	return v
}

// lineArg checks that x, SHIFT or BOUNDARY, is a scalar or an
// array of rank one less than array's, conforming with array's shape without
// dimension dim, and returns it (of type t, converted, when t is given).
func (a *analyzer) lineArg(f *ArrayFunc, x Expr, what string, array Expr, dim int, t Type) (Expr, bool) {
	if x == nil || x.Type().Kind == Invalid {
		return nil, false
	}

	if rankOf(x) > 0 {
		var line Shape

		for i, s := range array.Shape() {
			if i != dim-1 {
				line = append(line, s)
			}
		}

		if rankOf(x) != len(line) {
			a.report("ARRAYRANK", x.Position(), f.Name, what, "0 or "+strconv.Itoa(len(line)), rankOf(x))

			return nil, false
		}

		if _, ok := a.conform(x.Position(), line, x.Shape()); !ok {
			return nil, false
		}
	}

	if t.Kind != Invalid {
		return a.convert(x, t), true
	}

	return x, true
}

// spread is SPREAD(SOURCE, DIM, NCOPIES) (13.13.101): rank one more than
// SOURCE's, which may be a scalar, NCOPIES (0 if less) along DIM.
func (a *analyzer) spread(f *ArrayFunc, source Expr, dim ast.Expr, ncopies Expr) bool {
	if source == nil || source.Type().Kind == Invalid || ncopies == nil || ncopies.Type().Kind == Invalid {
		return false
	}

	rank := rankOf(source)

	if f.Dim = a.constantDim(f, dim, rank+1); f.Dim == 0 {
		return false
	}

	if ncopies.Type().Kind != Integer || rankOf(ncopies) > 0 {
		a.report("BADARG", ncopies.Position(), f.Name, "an INTEGER scalar NCOPIES", ncopies.Type().String())

		return false
	}

	f.Array, f.Other, f.T = source, a.convert(ncopies, IntegerType), source.Type()

	copies := Extent{}
	if n, ok := constantInt(f.Other); ok {
		copies = Extent{Size: max(n, 0), Known: true}
	}

	shape := source.Shape()
	f.S = append(append(append(Shape{}, shape[:f.Dim-1]...), copies), shape[f.Dim-1:]...)

	return true
}

// shift is CSHIFT(ARRAY, SHIFT, DIM) or EOSHIFT(ARRAY, SHIFT, BOUNDARY, DIM)
// (13.13.25, 13.13.32): ARRAY's shape; SHIFT INTEGER, BOUNDARY of ARRAY's type,
// each a scalar or an array of the rank below ARRAY's; DIM 1 when left out.
func (a *analyzer) shift(f *ArrayFunc, array Expr, arg func(int) ast.Expr, value func(int) Expr) bool {
	if array == nil {
		return false
	}

	dimArg := 2
	if f.Name == "EOSHIFT" {
		dimArg = 3
	}

	f.Dim = 1
	if arg(dimArg) != nil {
		if f.Dim = a.constantDim(f, arg(dimArg), rankOf(array)); f.Dim == 0 {
			return false
		}
	}

	shift, ok := a.lineArg(f, value(1), "SHIFT", array, f.Dim, Type{})
	if !ok {
		return false
	}

	if shift.Type().Kind != Integer {
		a.report("BADARG", shift.Position(), f.Name, "an INTEGER SHIFT", shift.Type().String())

		return false
	}

	f.Array, f.Other, f.T, f.S = array, a.convert(shift, IntegerType), array.Type(), slices.Clone(array.Shape())

	if f.Name == "EOSHIFT" && arg(2) != nil {
		b, ok := a.lineArg(f, value(2), "BOUNDARY", array, f.Dim, Type{})
		if !ok {
			return false
		}

		if !sameKind(b.Type(), array.Type()) {
			a.report("BADARG", b.Position(), f.Name, "a BOUNDARY of ARRAY's type", b.Type().String())

			return false
		}

		f.Pad = a.convert(b, array.Type())
		if array.Type().Kind == Character {
			f.Pad = b
		}
	}

	if f.Name == "EOSHIFT" && f.Pad == nil {
		switch array.Type().Kind {
		case Integer, Real, Complex, Logical, Character:
		default:
			a.report("BADARG", array.Position(), f.Name, "of a type with a default BOUNDARY, or given one", array.Type().String())

			return false
		}
	}

	return true
}

// sameKind reports whether a value of type b may stand for one of type t: the
// same kind of type (a CHARACTER one of any length).
func sameKind(b, t Type) bool {
	return b.Kind == t.Kind
}

// pack is PACK(ARRAY, MASK, VECTOR) (13.13.77): of rank one, ARRAY's elements
// where MASK (conforming, or a scalar) holds, its extent VECTOR's if given, else
// the count of them.
func (a *analyzer) pack(f *ArrayFunc, array, mask, vector Expr) bool {
	if array == nil || mask == nil || mask.Type().Kind == Invalid {
		return false
	}

	if mask.Type().Kind != Logical {
		a.report("BADARG", mask.Position(), f.Name, "a LOGICAL MASK", mask.Type().String())

		return false
	}

	if rankOf(mask) > 0 {
		if _, ok := a.conforms(mask.Position(), array, mask); !ok {
			return false
		}
	}

	f.Array, f.Mask, f.T, f.S = array, mask, array.Type(), Shape{{}}

	if vector != nil {
		if vector.Type().Kind == Invalid {
			return false
		}

		if rankOf(vector) != 1 {
			a.report("ARRAYRANK", vector.Position(), f.Name, "VECTOR", "1", rankOf(vector))

			return false
		}

		if !sameKind(vector.Type(), array.Type()) {
			a.report("BADARG", vector.Position(), f.Name, "a VECTOR of ARRAY's type", vector.Type().String())

			return false
		}

		f.Other = a.convertLike(vector, array.Type())
		f.S = Shape{vector.Shape()[0]}
	}

	return true
}

// convertLike is x converted to t, but a CHARACTER value as it is (of any
// length: an element is moved to the result's length as an assignment would).
func (a *analyzer) convertLike(x Expr, t Type) Expr {
	if t.Kind == Character {
		return x
	}

	return a.convert(x, t)
}

// unpack is UNPACK(VECTOR, MASK, FIELD) (13.13.112): MASK's shape, VECTOR's
// elements in order where it holds, FIELD's (conforming, or a scalar) elsewhere.
func (a *analyzer) unpack(f *ArrayFunc, vector, mask, field Expr) bool {
	if vector == nil || mask == nil || field == nil || mask.Type().Kind == Invalid || field.Type().Kind == Invalid {
		return false
	}

	if rankOf(vector) != 1 {
		a.report("ARRAYRANK", vector.Position(), f.Name, "VECTOR", "1", rankOf(vector))

		return false
	}

	if mask.Type().Kind != Logical {
		a.report("BADARG", mask.Position(), f.Name, "a LOGICAL MASK", mask.Type().String())

		return false
	}

	if rankOf(mask) == 0 {
		a.report("ARRAYARG", mask.Position(), f.Name, "MASK")

		return false
	}

	if !sameKind(field.Type(), vector.Type()) {
		a.report("BADARG", field.Position(), f.Name, "a FIELD of VECTOR's type", field.Type().String())

		return false
	}

	if rankOf(field) > 0 {
		if _, ok := a.conforms(field.Position(), mask, field); !ok {
			return false
		}
	}

	f.Array, f.Mask, f.Other, f.T, f.S = vector, mask, a.convertLike(field, vector.Type()), vector.Type(), slices.Clone(mask.Shape())

	return true
}

// reshapeValues is RESHAPE(SOURCE, SHAPE, PAD, ORDER) of values that aren't all
// constants (13.13.84): SHAPE an INTEGER array of rank one and a size known when
// compiling, its values the result's extents (known if they are constants); PAD
// of SOURCE's type; ORDER a constant permutation (ours: NOTIMPL otherwise).
func (a *analyzer) reshapeValues(f *ArrayFunc, e *ast.Apply, source, shape, pad Expr, order ast.Expr) bool {
	if source == nil || shape == nil || shape.Type().Kind == Invalid {
		return false
	}

	if shape.Type().Kind != Integer || rankOf(shape) != 1 {
		a.report("ARRAYARG", shape.Position(), f.Name, "SHAPE, an INTEGER array of rank one,")

		return false
	}

	if !shape.Shape()[0].Known {
		a.report("BADRESHAPE", e.Pos, "SHAPE's size isn't known when compiling")

		return false
	}

	rank := shape.Shape()[0].Size
	if rank < 1 || rank > 7 {
		a.report("BADRESHAPE", e.Pos, "SHAPE's size, "+strconv.Itoa(rank)+", isn't a rank from 1 to 7")

		return false
	}

	f.Array, f.Other, f.T = source, a.convert(shape, IntegerType), source.Type()

	if pad != nil {
		if pad.Type().Kind == Invalid {
			return false
		}

		if !sameKind(pad.Type(), source.Type()) || rankOf(pad) == 0 {
			a.report("BADRESHAPE", e.Pos, "PAD must be an array of SOURCE's type")

			return false
		}

		f.Pad = a.convertLike(pad, source.Type())
	}

	// The extents: SHAPE's values, if they are constants.
	ev := a.declEval()
	ev.quiet = true

	f.S = make(Shape, rank)

	if ac, ok := shape.(*ArrayConst); ok {
		for i, c := range ac.Elems {
			if c.Int < 0 {
				a.report("BADRESHAPE", e.Pos, "SHAPE has an extent below zero")

				return false
			}

			f.S[i] = Extent{Size: int(c.Int), Known: true}
		}
	}

	f.Order = make([]int, rank)
	for i := range f.Order {
		f.Order[i] = i
	}

	if order == nil {
		return true
	}

	o, err := ev.intVector(e, order, "ORDER")
	if err != nil {
		a.notImplForm(order.Position(), "RESHAPE with an ORDER= not constant")

		return false
	}

	seen := make([]bool, rank)
	ok := len(o) == rank

	for i := 0; ok && i < len(o); i++ {
		ok = o[i] >= 1 && o[i] <= rank && !seen[o[i]-1]
		if ok {
			seen[o[i]-1] = true
			f.Order[i] = o[i] - 1
		}
	}

	if !ok {
		a.report("BADRESHAPE", e.Pos, "ORDER isn't a permutation of 1 to the shape's rank")

		return false
	}

	return true
}
