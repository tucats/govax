package ir

import (
	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/sema"
	"github.com/tucats/govax/internal/vaxfloat"
)

// COMPLEX values (Decision 1 of docs/PHASE-78 - FORTRAN expressions and
// types.md). The VAX has no complex instructions, so lowering splits a
// complex expression into two trees of real operations, one for its real
// part and one for its imaginary part, and the code generator never sees
// complex arithmetic. A complex value is two floating values side by side in
// memory, the real part first (appendix C of the language manual): a
// COMPLEX*8's two F_floating longwords, a COMPLEX*16's two D_floating (or
// G_floating) quadwords. So a part is a place of its own, a Mem of the part's
// type, and the value as a whole is a Quad or an Octa, which is only ever
// moved (MOVQ, MOVO), passed, or returned.
//
// Sums, differences, and negation are done part by part. A COMPLEX*8 product
// is written in line, since MTHRTL has no OTS$MULC:
//
//	(a, b) * (c, d) = (ac - bd, ad + bc)
//
// each product and sum rounded as the VAX's instructions round, which is what
// folding a constant product does too (vaxfloat.ComplexMulSteps). A D or G
// product calls OTS$MULCD_R3 or OTS$MULCG_R3, and every quotient OTS$DIVC,
// OTS$DIVCD_R3, or OTS$DIVCG_R3, with the parts by value, and every power
// one of the OTS$ powers (lowerpower.go).

// The OTS$ routines for a product or quotient, by the parts' type.
var (
	complexProducts  = map[Type]string{Double: "OTS$MULCD_R3", GFloat: "OTS$MULCG_R3"}
	complexQuotients = map[Type]string{Float: "OTS$DIVC", Double: "OTS$DIVCD_R3", GFloat: "OTS$DIVCG_R3"}
)

// partOf is the machine type of a COMPLEX type's parts: F_floating for
// COMPLEX*8, D_floating or G_floating for COMPLEX*16.
func (l *lowerer) partOf(t sema.Type) Type {
	return l.typeOf(sema.Type{Kind: sema.Real, Size: t.Size / 2})
}

// parts are the places of the real and imaginary parts of the complex value
// at m. An element's index counts whole values, and a part's counts parts,
// twice as many: C(I) of a COMPLEX*8 array is C-8[I] as a Quad, and its real
// part C-8[2*I] as an F_floating value.
func parts(m *Mem, part Type) (re, im *Mem) {
	r := *m
	r.T = part

	if m.Index != nil {
		r.Index = arith(Mul, Long, m.Index, LongConst(2))
	}

	i := r
	i.Offset += part.Size()

	return &r, &i
}

// complexParts lowers a complex expression into its real part's tree and its
// imaginary part's.
func (l *lowerer) complexParts(e sema.Expr) (re, im Expr) {
	p := l.partOf(e.Type())

	switch e := e.(type) {
	case *sema.Const:
		return RealConst(p, e.Real), RealConst(p, e.Imag)

	case *sema.Var, *sema.Elem:
		r, i := parts(l.mem(e), p)

		return &Load{r}, &Load{i}

	case *sema.FuncRef:
		result := l.temp(l.typeOf(e.T))
		l.callFunction(e, result)

		r, i := parts(result, p)

		return &Load{r}, &Load{i}

	case *sema.IntrinsicRef:
		// CMPLX(X, Y), its parts converted already; CONJG(Z); or a call of
		// MTHRTL's (CSQRT, CDEXP).
		switch e.Op {
		case sema.CmplxOp:
			return l.expr(e.Args[0]), l.expr(e.Args[1])
		case sema.CallOp:
			r, i := parts(l.callIntrinsic(e), p)

			return &Load{r}, &Load{i}
		}

		re, im := l.complexParts(e.Args[0])

		return re, arith(Neg, p, im, nil)

	case *sema.Unary:
		// Negation: a COMPLEX has no .NOT.
		re, im := l.complexParts(e.X)

		return arith(Neg, p, re, nil), arith(Neg, p, im, nil)

	case *sema.Binary:
		return l.complexBinary(e, p)

	case *sema.Convert:
		// From another COMPLEX, each part converted; from an INTEGER or a
		// REAL, the real part, with an imaginary part of 0.0 (Table 3-1).
		if e.X.Type().Kind == sema.Complex {
			re, im := l.complexParts(e.X)

			return convertTo(p, re), convertTo(p, im)
		}

		return convertTo(p, l.expr(e.X)), RealConst(p, vaxfloat.Value{})
	}

	panic("ir: a COMPLEX expression semantic analysis left that lowering can't lower")
}

// complexBinary lowers X op Y for COMPLEX operands, whose parts have type p.
func (l *lowerer) complexBinary(e *sema.Binary, p Type) (re, im Expr) {
	if e.Op == ast.Pow {
		return l.complexPower(e, p)
	}

	a, b := l.complexParts(e.X)
	c, d := l.complexParts(e.Y)

	switch e.Op {
	case ast.Mul:
		if routine, ok := complexProducts[p]; ok {
			return l.complexCall(routine, e.T, a, b, c, d)
		}

		// Each part of each operand is used twice: one that must be
		// computed is computed once, into a temporary.
		a, b, c, d = l.reusable(a), l.reusable(b), l.reusable(c), l.reusable(d)

		return arith(Sub, p, arith(Mul, p, a, c), arith(Mul, p, b, d)),
			arith(Add, p, arith(Mul, p, a, d), arith(Mul, p, b, c))

	case ast.Div:
		return l.complexCall(complexQuotients[p], e.T, a, b, c, d)
	}

	op := operators[e.Op]

	return arith(op, p, a, c), arith(op, p, b, d)
}

// complexCall calls an OTS$ routine of two complex values, (a, b) and (c, d),
// each part by value, its complex result of type t coming back in R0 and up
// (R0:R1 for COMPLEX*8, R0:R3 for COMPLEX*16) into a temporary, whose parts
// it returns.
func (l *lowerer) complexCall(routine string, t sema.Type, a, b, c, d Expr) (re, im Expr) {
	result := l.temp(l.typeOf(t))
	args := []Arg{{Value: a}, {Value: b}, {Value: c}, {Value: d}}
	l.emit(&Call{origin: l.origin(), Routine: routine, Args: args, Result: result})

	r, i := parts(result, l.partOf(t))

	return &Load{r}, &Load{i}
}

// reusable is x, if using it twice costs nothing more than once (a constant,
// or a load), or else a load of a temporary x is computed into.
func (l *lowerer) reusable(x Expr) Expr {
	switch x.(type) {
	case *Const, *Load:
		return x
	}

	return &Load{l.value(x)}
}

// convertTo is x converted to the type t, or x itself if it has that type.
func convertTo(t Type, x Expr) Expr {
	if x.Type() == t {
		return x
	}

	return arith(Cvt, t, x, nil)
}

// complexValue is a place holding a complex expression's whole value: the
// variable or element itself, a constant's datum, or a temporary the value is
// computed into.
func (l *lowerer) complexValue(e sema.Expr) *Mem {
	switch e := e.(type) {
	case *sema.Var, *sema.Elem:
		return l.mem(e)

	case *sema.Const:
		return l.constant(l.constOf(e))

	case *sema.FuncRef:
		result := l.temp(l.typeOf(e.T))
		l.callFunction(e, result)

		return result

	case *sema.IntrinsicRef:
		if e.Op == sema.CallOp {
			return l.callIntrinsic(e)
		}
	}

	t := l.temp(l.typeOf(e.Type()))
	l.assignParts(t, e, true)

	return t
}

// assignComplex lowers dst = e, both COMPLEX of one type. A variable, an
// element, a constant, or the value of MTHRTL's routine is moved whole, with
// MOVQ or MOVO; anything else is computed part by part.
func (l *lowerer) assignComplex(dst *Mem, e sema.Expr) {
	if x, ok := e.(*sema.IntrinsicRef); ok && x.Op == sema.CallOp {
		l.emit(&Assign{l.origin(), dst, &Load{l.complexValue(e)}})

		return
	}

	switch e.(type) {
	case *sema.Var, *sema.Elem:
		l.emit(&Assign{l.origin(), dst, &Load{l.complexValue(e)}})

	case *sema.Const:
		l.emit(&Assign{l.origin(), dst, l.expr(e)})

	default:
		l.assignParts(dst, e, false)
	}
}

// assignParts stores e's real and imaginary parts at dst's. The real part is
// stored first, so if computing the imaginary part reads memory that might be
// the real part's place (C = C*D, or a dummy argument that is the same
// variable as another), the real part goes into a temporary until the
// imaginary part is stored. A fresh temporary can't be anything else, so it
// needs none.
func (l *lowerer) assignParts(dst *Mem, e sema.Expr, fresh bool) {
	re, im := l.complexParts(e)
	r, i := parts(dst, l.partOf(e.Type()))

	if !fresh && readsMemory(im) {
		re = &Load{l.value(re)}
		l.emit(&Assign{l.origin(), i, im})
		l.emit(&Assign{l.origin(), r, re})

		return
	}

	l.emit(&Assign{l.origin(), r, re})
	l.emit(&Assign{l.origin(), i, im})
}

// readsMemory reports whether computing e loads from a place a program's
// store could change: anything but a constant, a datum, or a temporary.
func readsMemory(e Expr) bool {
	switch e := e.(type) {
	case *Load:
		m := e.Mem

		return !(m.Datum != nil || m.Cell != nil && m.Cell.Kind == TempCell) || m.Index != nil && readsMemory(m.Index)

	case *Addr:
		return e.Mem.Index != nil && readsMemory(e.Mem.Index)

	case *Op:
		return readsMemory(e.X) || e.Y != nil && readsMemory(e.Y)

	case *Cmp:
		return readsMemory(e.X) || readsMemory(e.Y)

	case *Field:
		return readsMemory(e.X) || readsMemory(e.Pos) || readsMemory(e.Size)
	}

	return false
}

// complexCompare lowers X .EQ. Y or X .NE. Y for COMPLEX operands (section
// 2.3.3): equal if both parts are.
func (l *lowerer) complexCompare(e *sema.Compare) Expr {
	a, b := l.complexParts(e.X)
	c, d := l.complexParts(e.Y)

	if e.Op == ast.EQ {
		return arith(And, Long, &Cmp{Eq, a, c}, &Cmp{Eq, b, d})
	}

	return arith(Or, Long, &Cmp{Ne, a, c}, &Cmp{Ne, b, d})
}
