package sema

import (
	"strconv"

	"github.com/tucats/govax/internal/fortran/diag"
)

// Array-valued functions (the Fortran 90 standard's 12.5.2.2; docs/planning/
// PHASE-104 - Fortran 90 dynamic arrays.md, finding 12). A function's value
// may be an array: its name (or its RESULT variable) given dimensions,
//
//	FUNCTION UNIT(V)
//	REAL V(3), UNIT(3)
//	UNIT = V / SQRT(SUM(V * V))
//	END
//
// so that inside it the value is an array like any other, assigned whole or
// an element at a time. A caller can't make sense of UNIT(X) unless it knows
// that, so a reference needs an explicit interface (12.3.1.1): an interface
// body, an internal function's host, or a recursive function's own.
//
// The value goes through a hidden first argument, as a CHARACTER or REAL*16
// value does (Type.HiddenResult; HiddenValue): the caller passes the address
// of a temporary of the value's shape (a class A descriptor of one for
// CHARACTER), which the function's value is, a dummy array in all but name.
// A reference is a FuncRef with that shape, which lowering computes into its
// temporary before the statement's loops, as it computes MATMUL's
// (ir/lowerarray.go's prepare).
//
// A value whose bounds aren't constant,
//
//	FUNCTION RAMP(N)
//	INTEGER N
//	REAL RAMP(N)
//
// can't have a temporary made by its caller, which can't know N's meaning (the
// bounds may use the function's COMMON, its host's variables, or SIZE of its
// dummies). Its symbol is an automatic object, its bounds computed at entry, and
// the function makes the storage itself, on the heap, and describes it in a
// class NCA descriptor the caller passes, which the caller gives back at the
// statement's end (finding 12; ir/lowervalue.go). A reference's extents are then
// the reference's own, Extent.Of, read from that descriptor once it is called.

// HiddenValue reports whether the function whose value s is returns it through
// a hidden first argument: a value too wide for R0 and R1, a CHARACTER value
// (Type.HiddenResult), or an array.
func (s *Symbol) HiddenValue() bool {
	return s.Type.HiddenResult() || s.Dims != nil
}

// HiddenValue reports whether f's value comes back through a hidden first
// argument, as Symbol.HiddenValue says of the function's own.
func (f *FuncRef) HiddenValue() bool {
	return f.T.HiddenResult() || len(f.S) > 0
}

// resultShape is the shape of a value of the function s, from its explicit
// interface: nil for a scalar, or for a function without one. An extent known
// only when the program runs has no Of yet: funcRef makes it the reference.
func (a *analyzer) resultShape(s *Symbol) Shape {
	u := a.interfaceOf(s)
	if u == nil || u.Result == nil || u.Result.Dims == nil || u.Result.broken {
		return nil
	}

	return valueShape(u.Result)
}

// valueShape is the shape of r, an array-valued function's value: each extent
// known when compiling if its bounds are constants.
func valueShape(r *Symbol) Shape {
	shape := make(Shape, len(r.Dims))

	for i, d := range r.Dims {
		shape[i] = Extent{Dim: i}
		if d.known() {
			shape[i] = Extent{Size: max(d.Extent(), 0), Known: true}
		}
	}

	return shape
}

// funcRef is a reference to the function s, its value of type t, with its
// arguments: of the shape its interface gives an array value.
func (a *analyzer) funcRef(s *Symbol, t Type, args []Arg, pos diag.Position) *FuncRef {
	f := &FuncRef{exprBase{T: t, Pos: pos}, s, args}
	f.S = a.resultShape(s)

	for i := range f.S {
		if !f.S[i].Known {
			f.S[i].Of = f
		}
	}

	return f
}

// checkArrayValue checks the unit's value if it is an array: its length can't
// be (*) (5.1.1.5's constraint: the caller couldn't make its temporary), and
// what isn't compiled yet is NOTIMPL. Bounds that aren't constant made it an
// automatic object (bound), which is how lowering knows the function makes its
// storage. A value that can't stand is
// broken, and taken out of the adjustable arrays, whose checks would be the
// wrong ones for it.
func (a *analyzer) checkArrayValue() {
	r := a.unit.Result
	if r == nil || r.Dims == nil || r.broken {
		return
	}

	switch {
	case r.Type.Kind == Character && r.Type.Size == 0 && r.LengthExpr == nil:
		a.report("STARARRAY", r.DimPos, r.Name)
	case r.Struct != nil:
		a.notImplForm(r.DimPos, "an array-valued function whose value is records")
	case len(a.unit.Entries) > 0:
		a.notImplForm(r.DimPos, "an ENTRY statement in an array-valued function")
	default:
		return
	}

	r.broken = true

	kept := a.adjustable[:0]
	for _, adj := range a.adjustable {
		if adj.sym != r {
			kept = append(kept, adj)
		}
	}

	a.adjustable = kept
}

// resultDifference is how the value of the function body declares differs
// from the value of its definition, there, in rank or extents, or "".
func resultDifference(here, there *Symbol) string {
	switch {
	case len(here.Dims) != len(there.Dims):
		return "its value is " + rankText(len(here.Dims)) + " here, " + rankText(len(there.Dims)) + " there"
	case here.broken || there.broken:
		return ""
	}

	for k := range here.Dims {
		dh, dt := here.Dims[k], there.Dims[k]
		if dh.known() && dt.known() && dh.Extent() != dt.Extent() {
			return "its value has " + strconv.Itoa(dh.Extent()) + " elements in dimension " + strconv.Itoa(k+1) +
				" here, " + strconv.Itoa(dt.Extent()) + " there"
		}
	}

	return ""
}
