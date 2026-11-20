package sema

import (
	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/vaxfloat"
)

// The numeric model's functions (Fortran 90's 13.7.1 and 13.13: EXPONENT,
// FRACTION, NEAREST, RRSPACING, SCALE, SET_EXPONENT, and SPACING). Each VAX
// format is the model exactly (vaxfloat's model.go): a nonzero value is 0.1f
// × 2**e, its hidden bit the model's first digit, and its exponent field e
// plus a bias (128 for F and D, 1024 for G, 16384 for H). So each function is
// a change to the value's fields:
//
//	EXPONENT(X)         the field less the bias; 0 for 0
//	FRACTION(X)         X with the field the bias's
//	SET_EXPONENT(X, I)  X with the field the bias plus I
//	SCALE(X, I)         X with I added to the field
//	RRSPACING(X)        X with the field the bias plus p, the sign clear
//	SPACING(X)          the field less p - 1, the fraction 0.1
//	NEAREST(X, S)       X's bits, sign apart, one more or one less
//
// EXPONENT and FRACTION are a few instructions and are written in line
// (ir's lowermodel.go). The others must check their result's range, and
// NEAREST carries through every word of the value: they are FTNRTL's,
// FTN$SCALE_F, FTN$SCALE_D, FTN$SCALE_G, FTN$SCALE_H, and so on, by X's
// format (docs/planning/PHASE-108 - Fortran 90 intrinsic procedures.md,
// finding 7 and the author's answer 1). Of constants each is folded
// (constfuncs.go's modelFunctions, over vaxfloat's).

// modelFunction is a reference to one of the model's functions, e.Name, of
// args, elemental of shape.
func (a *analyzer) modelFunction(e *ast.Apply, args []Expr, shape Shape) Expr {
	x := args[0]
	if x.Type().Kind != Real {
		a.report("BADARG", x.Position(), e.Name, "a REAL X", x.Type().String())

		return a.bad(e.Pos)
	}

	if len(args) == 2 {
		switch y := args[1]; e.Name {
		case "NEAREST":
			// S's sign is all the routine reads: bit 15, in every format,
			// so S goes as it is, of whatever kind.
			if y.Type().Kind != Real {
				a.report("BADARG", y.Position(), e.Name, "a REAL S", y.Type().String())

				return a.bad(e.Pos)
			}

		default:
			if y.Type().Kind != Integer {
				a.report("BADARG", y.Position(), e.Name, "an INTEGER I", y.Type().String())

				return a.bad(e.Pos)
			}

			args = []Expr{x, a.convert(y, IntegerType)}
		}
	}

	ref := &IntrinsicRef{exprBase{T: x.Type(), Pos: e.Pos, S: shape}, e.Name, CallOp, args, ""}

	switch e.Name {
	case "EXPONENT":
		ref.T, ref.Op = a.defaultInt, ExponentOp
	case "FRACTION":
		ref.Op = FractionOp
	default:
		ref.Routine = modelRoutine(e.Name, a.format(x.Type()))
	}

	return ref
}

// modelRoutine is FTNRTL's routine for the model's function name of a value
// of format f: FTN$SCALE_F, FTN$SET_EXPONENT_G, ...
func modelRoutine(name string, f vaxfloat.Format) string {
	return "FTN$" + name + "_" + f.String()
}
