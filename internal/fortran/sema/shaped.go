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
// element, by section. Only what needs its elements one after another in
// storage, which they may not be, waits for later: passing it to a dummy that
// isn't assumed-shape (sequence association, which needs a copy: subtask 6), and
// using it as a run-time format, an internal file, or ENCODE's or DECODE's
// buffer. An I/O list takes it an element at a time.

// argsTo analyzes the actual arguments, list, of a reference to the procedure s,
// in its dummies' order (actuals): an assumed-shape dummy's by shaped, every
// other's as args does.
func (a *analyzer) argsTo(s *Symbol, list []ast.Expr) []Arg {
	var params []*Symbol

	if target := a.interfaceOf(s); target != nil && !target.incomplete && !s.broken {
		params = target.Params
	}

	args := make([]Arg, len(list))

	for i, e := range list {
		if i < len(params) && params[i].AssumedShape() && !params[i].broken {
			args[i] = a.shaped(a.argValue(e))
		} else {
			args[i] = a.unshaped(a.arg(e))
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

	if f, ok := v.(*FieldRef); ok {
		if argumentRank(f) > 0 {
			a.notImplForm(f.Position(), "a record's array field as an assumed-shape dummy's actual argument (Phase 105)")

			return Arg{Expr: a.bad(f.Position())}
		}

		return arg
	}

	if r, ok := v.(*ArrayRef); ok {
		switch {
		case r.Sym.AssumedSize():
			a.report("NOSHAPE", r.Pos, r.Sym.Name)

			return Arg{Expr: a.bad(r.Pos)}

		case r.Sym.Struct != nil:
			a.notImplForm(r.Pos, "an array of records as a whole")

			return Arg{Expr: a.bad(r.Pos)}
		}
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
// section without a vector subscript, or a substring of either, none in
// parentheses. Any other's value goes in a temporary first.
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
		for _, sub := range x.Subs {
			if sub.Vector != nil {
				return false
			}
		}

		return true
	}

	return false
}

// unshaped is arg, the actual argument of a dummy that isn't assumed-shape, or a
// bad expression after NOTIMPL if it is a whole assumed-shape array: such a
// dummy takes its elements one after another (sequence association, 12.4.1.4),
// which needs a copy unless they are (subtask 6).
func (a *analyzer) unshaped(arg Arg) Arg {
	if r, ok := arg.Expr.(*ArrayRef); ok && r.Sym.AssumedShape() && !r.Sym.broken {
		a.notImplForm(r.Pos, "an assumed-shape array as the actual argument of a dummy that isn't assumed-shape (Phase 104, subtask 6)")
		arg.Expr = a.bad(r.Pos)
	}

	return arg
}

// noContiguous reports NOTIMPL, and returns true, if e is a whole assumed-shape
// array, which what is to be used as cannot have, its elements needing to be one
// after another in storage: a run-time format, an internal file, or ENCODE's or
// DECODE's buffer.
func (a *analyzer) noContiguous(e Expr, what string) bool {
	r, ok := e.(*ArrayRef)
	if !ok || !r.Sym.AssumedShape() || r.Sym.broken {
		return false
	}

	a.notImplForm(r.Pos, "an assumed-shape array as "+what+" (Phase 104, subtask 6)")

	return true
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
// an OPTIONAL one, or an assumed-shape one. nil if there is none.
func explicitDummy(params []*Symbol) (*Symbol, string) {
	for _, p := range params {
		switch {
		case p.Optional:
			return p, "OPTIONAL"
		case p.AssumedShape():
			return p, "assumed-shape"
		}
	}

	return nil, ""
}
