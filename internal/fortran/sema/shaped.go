package sema

import (
	"github.com/tucats/govax/internal/fortran/ast"
)

// Assumed-shape dummy arguments (the Fortran 90 standard's 5.1.2.4.2 and
// 12.4.1.1; docs/planning/PHASE-104 - Fortran 90 dynamic arrays.md, findings 9
// and 10).
//
//	      SUBROUTINE SCALE(A, F)
//	      REAL A(:, :), F
//	      A = A * F
//	      END
//	      ...
//	      CALL SCALE(X(1:10:2, :), 2.0)
//
// A's shape is its actual argument's, whatever that is: a whole array, or a
// section with any strides. Its lower bounds are its own, 1 here, or as declared
// (A(0:, :)), never the actual's. Since nothing in the call says how the
// elements are laid out but the actual argument itself, it is passed by a class
// NCA descriptor (the OpenVMS Calling Standard's 5.7), which has the strides in
// bytes; and since the caller must know to build one, a procedure with an
// assumed-shape dummy can be referenced only through an explicit interface
// (12.3.1.1): an internal procedure's, an interface body's, or a recursive
// procedure's own. A reference to one of the file's procedures without one is
// EXPLICITREQ.
//
// So which actual arguments are passed by descriptor is known only from the
// interface: argsTo analyzes each against its dummy, marking the ones an
// assumed-shape dummy takes (Arg.Shaped). Each must be an array of the dummy's
// rank (12.4.1.1), never an element (checkInterface's ARGRANK) nor an
// assumed-size array (NOSHAPE), and a CHARACTER one's length must be the
// dummy's (ARGLENGTH).
//
// Any other array value, an expression like A + 1.0, a section with a vector
// subscript, or (A), is computed into a temporary of the caller's statement, and
// that is described and passed; nothing is copied back, the dummy not being
// definable (12.4.1.1; INTENT(OUT) is NOTDEFINABLE).
//
// Inside the routine, an assumed-shape array is used as any other: whole, by
// element, by section. Given to a dummy that isn't assumed-shape, or used as a
// run-time format, an internal file, or ENCODE's or DECODE's buffer, where its
// elements must be one after another in storage, which they may not be, it goes
// through a temporary unless they are (sequence.go). An I/O list takes it an
// element at a time.

// argsTo analyzes the actual arguments, list, of a reference to the procedure s,
// in its dummies' order (actuals): an assumed-shape dummy's by shaped, every
// other's by sequenced (sequence.go).
func (a *analyzer) argsTo(s *Symbol, list []ast.Expr) []Arg {
	var params []*Symbol

	elemental := false

	if target := a.interfaceOf(s); target != nil && !target.incomplete && !s.broken {
		params, elemental = target.Params, target.Elemental
	}

	args := make([]Arg, len(list))

	for i, e := range list {
		var p *Symbol
		if i < len(params) && !params[i].broken {
			p = params[i]
		}

		switch {
		case p != nil && p.Pointer:
			args[i] = a.pointerArg(e, i, s, p)
		case p != nil && p.AssumedShape():
			args[i] = a.shaped(a.argValue(e))
		case elemental && p != nil && len(p.Dims) == 0:
			// An array goes to an elemental procedure's scalar dummy an
			// element at a time (elemental.go).
			args[i] = a.elementalArg(a.argValue(e))
		default:
			args[i] = a.sequenced(a.argValue(e), p)
		}

		a.notePassed(args[i])
	}

	return args
}

// shaped is arg, analyzed by argValue, as the actual argument of an
// assumed-shape dummy, which goes by descriptor (Arg.Shaped): a whole array, a
// section, a substring of either, or any other array value, an expression, a
// vector-subscripted section, or an array in parentheses, whose value the caller
// puts in a temporary (the IR's shapedArg), never copied back (12.4.1.1: the
// dummy isn't definable then). Anything else is left for checkInterface's ARGRANK
// and the rest.
func (a *analyzer) shaped(arg Arg) Arg {
	switch arg.Expr.(type) {
	case nil, *bad, *Procedure:
		// An OPTIONAL dummy left out, a failure, or a procedure.
		return arg
	}

	if arg.Hollerith || arg.Typeless || arg.Mode != ByDefault {
		return a.arrayChecked(arg)
	}

	v := arg.Expr
	arg.expression = false

	// An array field without subscripts, T%COORD, is every element of it,
	// as PATH%X is every element's X: described by stride, as a section is.
	if f, ok := v.(*FieldRef); ok && f.WholeArray() {
		v = f.wholeRanked()
		arg.Expr = v
	}

	if r, ok := v.(*ArrayRef); ok && r.Sym.AssumedSize() {
		a.report("NOSHAPE", r.Pos, r.Sym.Name)

		return Arg{Expr: a.bad(r.Pos)}
	}

	if argumentRank(v) == 0 {
		return arg
	}

	// A temporary's CHARACTER elements need a length, which a value whose
	// length is known only when the program runs gives only if it is made of
	// names (as a constructor's first value must, Constructor.LengthOf).
	if t := v.Type(); t.Kind == Character && t.Size == 0 && !Described(arg) && !lengthFromNames(v) {
		a.notImplForm(v.Position(), "a CHARACTER array expression of a length known only when the program runs, not made of names, as an assumed-shape dummy's actual argument")

		return Arg{Expr: a.bad(v.Position())}
	}

	arg.Shaped = true

	return arg
}

// Described reports whether arg, the actual argument of an assumed-shape dummy,
// is passed by a descriptor of its own elements where they are: a whole array, a
// section without a vector subscript, a component with a rank (PATH%X,
// T%COORD(1:2)) without one, or a substring of any, none in parentheses. Any
// other's value goes in a temporary first; so does a constant's component, which
// is never given to a procedure to change.
func Described(arg Arg) bool {
	if arg.Copy {
		return false
	}

	v := arg.Expr
	if sub, ok := v.(*Substring); ok {
		v = sub.X
	}

	switch x := v.(type) {
	case *ArrayRef:
		return true

	case *Section:
		return !hasVector(x.Subs)

	case *FieldRef:
		return x.S != nil && x.Sym.Class != Constant && !hasVector(x.Sections)
	}

	return false
}

// hasVector reports whether any of a section's subscripts is a vector.
func hasVector(subs []SectionSub) bool {
	for _, sub := range subs {
		if sub.Vector != nil {
			return true
		}
	}

	return false
}

// shapeText says what kind of array a dummy is, for INTERFACEDIFF.
func shapeText(s *Symbol) string {
	switch {
	case s.AssumedShape():
		return "an assumed-shape array"
	case s.AssumedSize():
		return "an assumed-size array"
	}

	return "an explicit-shape array"
}

// explicitDummy is the first dummy argument among params that makes a reference
// need an explicit interface (12.3.1.1), and what kind it is, for EXPLICITREQ:
// an OPTIONAL one, an assumed-shape one, a pointer, or a TARGET. nil if there
// is none.
func explicitDummy(params []*Symbol) (*Symbol, string) {
	for _, p := range params {
		switch {
		case p.Optional:
			return p, "OPTIONAL"
		case p.AssumedShape():
			return p, "assumed-shape"
		case p.Pointer:
			return p, "pointer"
		case p.Target:
			return p, "TARGET"
		}
	}

	return nil, ""
}
