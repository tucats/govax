package ir

import (
	"github.com/tucats/govax/internal/fortran/sema"
)

// Exponentiation, X ** Y (section 2.3.1 of the language manual). The VAX has
// no instruction for it, so a power is a call of one of MTHRTL's OTS$
// routines, which take their arguments by value (the values themselves in
// the argument list, not their addresses) and return the result in R0 and
// up. Semantic analysis has given the base and the exponent the types the
// routine takes (sema's powerTypes), so the routine follows from them.
//
// A small constant exponent is written in line instead: X**2 is X*X. It is
// computed in the order the OTS$ routines and constant folding use
// (vaxfloat.PowerBySquaring), so that a value doesn't depend on whether it
// was folded, written in line, or called: the product of X's squares for
// each bit of the exponent, X**7 being (X * X**2) * X**4, where X**4 is
// (X**2)**2, each product rounded as the VAX's MUL instructions round. A
// negative exponent multiplies the squares' reciprocals, which takes more
// instructions than the call, so only exponents from 1 to maxInlinePower are
// in line. A COMPLEX power always calls: the routines round each product's
// parts once, which a COMPLEX*8 product written in line doesn't
// (lowercomplex.go).

// maxInlinePower is the largest constant exponent written in line.
const maxInlinePower = 8

// powerKey is a routine's base and exponent types: the parts' type for a
// COMPLEX base.
type powerKey struct{ base, exponent Type }

// The OTS$ powers, by their operands' types. G_floating has no form of
// OTS$POWRD or OTS$POWDR: analysis makes a REAL*4 beside a REAL*8 a REAL*8
// there.
var (
	realPowers = map[powerKey]string{
		{Word, Word}:     "OTS$POWII",
		{Long, Long}:     "OTS$POWJJ",
		{Float, Long}:    "OTS$POWRJ",
		{Double, Long}:   "OTS$POWDJ",
		{GFloat, Long}:   "OTS$POWGJ",
		{HFloat, Long}:   "OTS$POWHJ_R3",
		{Float, Float}:   "OTS$POWRR",
		{Float, Double}:  "OTS$POWRD",
		{Double, Float}:  "OTS$POWDR",
		{Double, Double}: "OTS$POWDD",
		{GFloat, GFloat}: "OTS$POWGG",
		{HFloat, HFloat}: "OTS$POWHH_R3",
	}

	complexPowers = map[powerKey]string{
		{Float, Long}:    "OTS$POWCJ",
		{Double, Long}:   "OTS$POWCDJ_R3",
		{GFloat, Long}:   "OTS$POWCGJ_R3",
		{Float, Float}:   "OTS$POWCC",
		{Double, Double}: "OTS$POWCDCD_R3",
		{GFloat, GFloat}: "OTS$POWCGCG_R3",
	}
)

// power lowers X ** Y of an INTEGER or REAL type.
func (l *lowerer) power(e *sema.Binary) Expr {
	t := l.typeOf(e.T)
	x := l.expr(e.X)

	if n, ok := inlineExponent(e.Y); ok {
		return l.inlinePower(t, x, n)
	}

	y := l.expr(e.Y)
	routine := realPowers[powerKey{x.Type(), y.Type()}]

	// An INTEGER*2 is passed as a longword, as every value is at least.
	if x.Type() == Word {
		x, y = arith(Cvt, Long, x, nil), arith(Cvt, Long, y, nil)
	}

	result := l.temp(t)
	l.emit(&Call{origin: l.origin(), Routine: routine, Args: []Arg{{Value: x}, {Value: y}}, Result: result})

	return &Load{result}
}

// inlineExponent is y's value, if it is an INTEGER constant a power is
// written in line for.
func inlineExponent(y sema.Expr) (int64, bool) {
	c, ok := y.(*sema.Const)
	if !ok || c.T.Kind != sema.Integer || c.Int < 1 || c.Int > maxInlinePower {
		return 0, false
	}

	return int64(c.Int), true
}

// inlinePower is x ** n in type t, for n from 1 up, as vaxfloat.PowerBySquaring
// computes it: for each bit of n from the lowest, the square so far multiplied
// into the result if the bit is set, then squared for the next bit, if there
// is one. A value used again is computed once (reusable).
func (l *lowerer) inlinePower(t Type, x Expr, n int64) Expr {
	var result Expr

	for {
		if n > 1 {
			x = l.reusable(x)
		}

		if n&1 != 0 {
			result = arith(Mul, t, result, x)
		}

		if n >>= 1; n == 0 {
			return result
		}

		x = arith(Mul, t, x, x)
	}
}

// complexPower lowers X ** Y of a COMPLEX type, whose parts have type p, into
// its result's parts: OTS$POWCJ and its kind for an INTEGER exponent, given as
// a longword, and OTS$POWCC and its kind for a COMPLEX one.
func (l *lowerer) complexPower(e *sema.Binary, p Type) (re, im Expr) {
	a, b := l.complexParts(e.X)

	if e.Y.Type().Kind == sema.Integer {
		n := l.expr(e.Y)
		routine := complexPowers[powerKey{p, Long}]
		result := l.temp(l.typeOf(e.T))
		l.emit(&Call{origin: l.origin(), Routine: routine, Args: []Arg{{Value: a}, {Value: b}, {Value: n}}, Result: result})

		r, i := parts(result, p)

		return &Load{r}, &Load{i}
	}

	c, d := l.complexParts(e.Y)

	return l.complexCall(complexPowers[powerKey{p, p}], e.T, a, b, c, d)
}
