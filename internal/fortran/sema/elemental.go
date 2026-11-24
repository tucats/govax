package sema

import (
	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
)

// ELEMENTAL procedures (the Fortran 95 standard's 12.7; the HP Fortran
// manual's 8.5.1.3; docs/planning/PHASE-112 - Fortran 95.md, subtask 7). An
// elemental procedure is written for scalars and may be referenced with arrays:
//
//	ELEMENTAL REAL FUNCTION CLIP(X, LO, HI)
//	  REAL, INTENT(IN) :: X, LO, HI
//	  CLIP = MAX(LO, MIN(HI, X))
//	END FUNCTION CLIP
//	...
//	Y = CLIP(R, -1.0, 1.0)
//
// is CLIP called once for each element of R, the result an array of R's shape,
// as the elemental intrinsic functions are. So the definition is held to rules
// that make every dummy argument and the value a scalar the caller can supply
// an element for (12.7.1): each dummy a scalar, not a pointer, not a dummy
// procedure, no alternate return; the value a scalar, not a pointer; and the
// procedure PURE (it is, by its prefix: pure.go checks it), never RECURSIVE.
// ELEMDUMMY, ELEMRESULT, ELEMRETURN, and ELEMRECURSIVE say which rule a
// definition or an interface body breaks.
//
// A reference needs an explicit interface (12.3.1.1; EXPLICITELEMENTAL), since
// only that says the procedure is elemental. Through one, an array may go to
// any of its scalar dummies (argsTo leaves such an actual as it is: no sequence
// association, elementalArg), the arrays must conform, and the reference's
// shape is theirs (elementalShape):
//
//   - a function's reference is a FuncRef with that shape and Elemental set: the
//     scalarizer reaches into its arguments as it reaches into an elemental
//     intrinsic's (elements.go's Scalarize), so the call is made at each
//     element with each array argument's element and each scalar one as it is.
//     With no array among its arguments it is an ordinary reference;
//   - a subroutine's CALL is a Call with Shape, which lowering makes a call per
//     element in array element order, as Fortran 95 says (12.4.3; ir's
//     elementalCall, Fortran 90's MVBITS's). An actual given to an INTENT(OUT)
//     or INTENT(INOUT) dummy must then be an array itself (ELEMOUT): its
//     elements are what the calls define.
//
// A generic name's or an operator's specific that is elemental is chosen for
// arrays only when no specific fits them exactly (14.1.2.4.1's rule: the
// nonelemental first; generic.go's resolveGeneric, operators.go's
// resolveOperator). A defined assignment by an elemental subroutine is the
// same CALL with the target's shape (operators.go's definedAssignment), in an
// array assignment or a WHERE; its value is in parentheses (7.5.1.3), so it is
// computed whole before any element of the target is defined, which Call.Order
// keeps as an array assignment's order keeps it: a temporary first where the
// two overlap in a way the loops can't run.

// checkElemental checks an ELEMENTAL unit's dummy arguments and value against
// 12.7.1's rules, once the unit has been analyzed (an interface body's too).
func (a *analyzer) checkElemental() {
	u := a.unit
	if !u.Elemental {
		return
	}

	kind := "subroutine"
	if u.Kind == ast.FunctionUnit {
		kind = "function"
	}

	for _, s := range u.Dummies {
		if s.broken {
			continue
		}

		switch {
		case s.DummyProcedure():
			a.report("ELEMDUMMY", s.Pos, s.Name, kind, u.Name, "a dummy procedure")
		case s.Pointer:
			a.report("ELEMDUMMY", s.Pos, s.Name, kind, u.Name, "a pointer")
		case len(s.Dims) > 0:
			a.report("ELEMDUMMY", s.Pos, s.Name, kind, u.Name, "an array")
		}
	}

	if len(u.AltPlaces) > 0 {
		a.report("ELEMRETURN", u.AST.Position(), u.Name)
	}

	if r := u.Result; r != nil && !r.broken && u.Kind == ast.FunctionUnit {
		switch {
		case r.Pointer:
			a.report("ELEMRESULT", r.Pos, u.Name, "a pointer")
		case len(r.Dims) > 0:
			a.report("ELEMRESULT", r.Pos, u.Name, "an array")
		}
	}

	a.elementalBounds(kind)
}

// elementalBounds reports each dummy argument an ELEMENTAL procedure's bounds
// and CHARACTER lengths name other than as an inquiry function's argument
// (ELEMBOUND; 12.7.1: a dummy may appear in a specification expression only as
// the argument of BIT_SIZE, KIND, LEN, or a numeric inquiry function, since a
// reference with arrays calls the procedure for each element in turn, and a
// local's size mustn't change with them). So W(N) is refused, and
// CHARACTER(LEN=LEN(C)) :: T taken (Phase 113).
func (a *analyzer) elementalBounds(kind string) {
	for _, adj := range a.adjustable {
		for _, n := range adj.names {
			s := a.scope.find(n.Name)
			if s == nil || s.Dummy == 0 || s.broken || adj.inquired[n] {
				continue
			}

			a.report("ELEMBOUND", n.Pos, s.Name, kind, a.unit.Name, adj.where())
		}
	}
}

// elementalPrefixes reports ELEMENTAL given with RECURSIVE (12.5.2.2's
// constraint): only the prefix itself, not /RECURSIVE or OPTIONS's, which
// makes every unit recursive and so can't change what is legal.
func (a *analyzer) elementalPrefixes(pos diag.Position, name string, elemental, recursive bool) {
	if elemental && recursive {
		a.report("ELEMRECURSIVE", pos, name)
	}
}

// elementalTarget is the explicit interface of the procedure s if it is
// elemental, the one a reference in this unit goes through; nil otherwise.
func (a *analyzer) elementalTarget(s *Symbol) *Unit {
	if u := a.interfaceOf(s); u != nil && u.Elemental && !u.incomplete && !s.broken {
		return u
	}

	return nil
}

// elementalArg is arg, analyzed by argValue, as the actual argument of a scalar
// dummy of an elemental procedure: an array section or array expression goes as
// itself, each element in turn, never by sequence association; a whole array,
// which argValue passes as itself without a shape, given its shape (NOSHAPE for
// an assumed-size one, whose elements can't be counted).
func (a *analyzer) elementalArg(arg Arg) Arg {
	arg.expression = false

	if r, ok := arg.Expr.(*ArrayRef); ok && r.S == nil {
		if r.Sym.AssumedSize() {
			a.report("NOSHAPE", r.Pos, r.Sym.Name)

			return Arg{Expr: a.bad(r.Pos)}
		}

		c := *r
		c.S = arrayShape(&c)
		arg.Expr = &c
	}

	return arg
}

// elementalShape is the shape of a reference at pos to the elemental procedure
// target with the arguments args (in its dummies' order): the conforming
// arrays' among them, nil if none is an array. ok is false after NOTCONFORM or
// ELEMOUT.
func (a *analyzer) elementalShape(target *Unit, name string, args []Arg, pos diag.Position) (Shape, bool) {
	var list []Expr

	for _, arg := range args {
		if arg.Expr != nil {
			if _, proc := arg.Expr.(*Procedure); !proc {
				list = append(list, arg.Expr)
			}
		}
	}

	shape, ok := a.conforms(pos, list...)
	if !ok || shape == nil {
		return nil, ok
	}

	// Every element the calls define is an element of the actual: a scalar
	// given to an INTENT(OUT) or INTENT(INOUT) dummy would be defined by every
	// call.
	for i, arg := range args {
		if i >= len(target.Params) || arg.Expr == nil {
			continue
		}

		if p := target.Params[i]; (p.Intent == "OUT" || p.Intent == "INOUT") && rankOf(arg.Expr) == 0 {
			a.report("ELEMOUT", arg.Expr.Position(), i+1, name, p.Name, p.Intent)

			return nil, false
		}
	}

	return shape, true
}

// elementalCallShape gives c, a CALL of an elemental subroutine, its shape, nil if
// none of its arguments is an array; false after an error.
func (a *analyzer) elementalCallShape(c *Call, pos diag.Position) bool {
	target := a.elementalTarget(c.Sub)
	if target == nil {
		return true
	}

	shape, ok := a.elementalShape(target, c.Sub.Name, c.Args, pos)
	c.Shape = shape

	return ok
}
