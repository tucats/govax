package ir

import (
	"errors"
	"math"

	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/sema"
	"github.com/tucats/govax/internal/vaxfloat"
)

// expr lowers a value. A function reference is taken out of the
// expression: its call goes before the statement, with the value in a
// temporary, which the expression loads. So the code generator never
// holds a value in a register across a CALLS, which may change R0 to
// R5 (a routine saves only the registers it uses of R2 to R11, and
// returns its value in R0).
func (l *lowerer) expr(e sema.Expr) Expr {
	switch e := e.(type) {
	case *sema.Const:
		return constOf(e)

	case *sema.Var, *sema.Elem:
		return &Load{l.mem(e)}

	case *sema.FuncRef:
		result := l.temp(typeOf(e.T))
		l.callFunction(e, result)

		return &Load{result}

	case *sema.Unary:
		op := Neg
		if e.Op == ast.Not {
			op = Not
		}

		return arith(op, typeOf(e.T), l.expr(e.X), nil)

	case *sema.Binary:
		x := l.expr(e.X)
		y := l.expr(e.Y)

		return arith(operators[e.Op], typeOf(e.T), x, y)

	case *sema.Compare:
		x := l.expr(e.X)
		y := l.expr(e.Y)

		return &Cmp{operators[e.Op], x, y}

	case *sema.Convert:
		// INTEGER and LOGICAL are one longword: converting between them
		// does nothing.
		x := l.expr(e.X)
		if t := typeOf(e.T); t != x.Type() {
			return arith(Cvt, t, x, nil)
		}

		return x
	}

	panic("ir: an expression semantic analysis left that lowering can't lower")
}

// operators are the IR's operators for FORTRAN's.
var operators = map[ast.Op]Operator{
	ast.Add: Add, ast.Sub: Sub, ast.Mul: Mul, ast.Div: Div,
	ast.And: And, ast.Or: Or,
	ast.EQ: Eq, ast.NE: Ne, ast.LT: Lt, ast.LE: Le, ast.GT: Gt, ast.GE: Ge,
}

// constOf is a constant's IR form: a LOGICAL's longword (-1 or 0), an
// INTEGER's, or a REAL's F_floating bits.
func constOf(c *sema.Const) *Const {
	if c.T.Kind == sema.Real {
		return FloatConst(c.Real)
	}

	return LongConst(c.Int)
}

// symbolMem is where a variable or array is: its cell, or, for a dummy
// argument, the caller's variable its argument points at.
func (l *lowerer) symbolMem(s *sema.Symbol) *Mem {
	t := typeOf(s.Type)

	if p := l.params[s]; p != nil {
		return &Mem{Param: p, T: t}
	}

	return &Mem{Cell: l.cells[s], T: t}
}

// mem is where a variable or an array element is. An element's address
// is the array's, plus the element's offset from the first element,
// which for A(s1, s2) with A(l1:u1, l2:u2) is
//
//	((s1 - l1) + (s2 - l2)*n1) * size,   n1 = u1 - l1 + 1
//
// since FORTRAN stores arrays in column-major order (section 2.2.3.3:
// the first subscript varies fastest). The parts of that which are
// constant (the bounds, and any constant in a subscript) become the
// Mem's offset, and the rest its index, which the VAX's indexed mode
// multiplies by the element's size: TABLE(I+1), with TABLE(1:10), is
// TABLE+0[I], and V(I-1) with V(0:9) is V-4[I].
func (l *lowerer) mem(e sema.Expr) *Mem {
	switch e := e.(type) {
	case *sema.Var:
		return l.symbolMem(e.Sym)

	case *sema.Elem:
		m := l.symbolMem(e.Sym)

		var index Expr

		first, multiplier := 0, 1

		for i, sub := range e.Subs {
			dim := e.Sym.Dims[i]
			v, k := split(l.expr(sub))
			first += (int(k) - dim.Lower) * multiplier

			if v != nil {
				index = arith(Add, Long, index, arith(Mul, Long, v, LongConst(int32(multiplier))))
			}

			multiplier *= dim.Upper - dim.Lower + 1
		}

		m.Offset = first * e.Sym.Type.Size
		m.Index = index

		return m
	}

	panic("ir: an assignment's target that is neither a variable nor an array element")
}

// value is a place holding x's value, for a statement that needs one: a
// temporary already holding it (a function's value), or a new one, x
// stored there.
func (l *lowerer) value(x Expr) *Mem {
	if ld, ok := x.(*Load); ok && ld.Mem.Cell != nil && ld.Mem.Cell.Kind == TempCell {
		return ld.Mem
	}

	t := l.temp(x.Type())
	l.emit(&Assign{l.origin(), t, x})

	return t
}

// args lowers a call's actual arguments (sections 5.1, 6.1.2.1): each
// by reference. A variable, an array element, or a whole array is
// passed itself, so the routine can change it; a constant is passed in
// a datum, read-only, so a routine that assigns to its dummy gets an
// access violation, not a changed constant; any other expression, and a
// variable or element in parentheses, is computed into a temporary and
// that is passed. A null argument is the longword 0.
func (l *lowerer) args(list []sema.Arg) []Arg {
	args := make([]Arg, len(list))

	for i, a := range list {
		switch e := a.Expr.(type) {
		case nil:
			args[i].Value = LongConst(0)

		case *sema.Var, *sema.Elem:
			if a.Copy {
				args[i].Ref = l.value(l.expr(e))
			} else {
				args[i].Ref = l.mem(e)
			}

		case *sema.ArrayRef:
			args[i].Ref = l.symbolMem(e.Sym)

		case *sema.Const:
			args[i].Ref = l.constant(constOf(e))

		default:
			args[i].Ref = l.value(l.expr(e))
		}
	}

	return args
}

// callFunction calls the function f refers to, its value going to
// result.
func (l *lowerer) callFunction(f *sema.FuncRef, result *Mem) {
	args := l.args(f.Args)
	l.emit(&Call{origin: l.origin(), Routine: f.Fn.Name, Args: args, Result: result})
}

// arith is the operation op on x and y (y nil for Neg, Not, and Cvt) in
// type t, folded when it can be:
//
//   - an operation on constants is done now, a Long one in 32 bits as
//     the VAX's instructions do it (wrapping around on overflow, which
//     the code doesn't detect), a Float one with package vaxfloat, which
//     rounds as the VAX does; an operation that would overflow, divide
//     by zero, or give a reserved operand is left for run time;
//   - a Long addition or subtraction collects its constants into one
//     (I-1+1 is I), and adding or multiplying by nothing goes, which
//     keeps DO loops' counts and array subscripts short.
//
// A nil x (in an addition being built up) is nothing: arith(Add, t,
// nil, y) is y.
func arith(op Operator, t Type, x, y Expr) Expr {
	if x == nil {
		return y
	}

	cx, okx := x.(*Const)
	cy, oky := y.(*Const)

	switch {
	case y == nil && okx:
		if c := foldUnary(op, t, cx); c != nil {
			return c
		}

	case okx && oky:
		if c := foldBinary(op, t, cx, cy); c != nil {
			return c
		}
	}

	if t == Long {
		switch op {
		case Add, Sub:
			return linear(op, x, y)
		case Mul:
			switch {
			case okx && cx.Int() == 1:
				return y
			case oky && cy.Int() == 1:
				return x
			}
		}
	}

	return &Op{op, t, x, y}
}

// split divides a Long expression into a part that isn't constant (nil
// if there is none) and a constant added to it: I+1 is I and 1, I-1 is
// I and -1, 5 is nil and 5.
func split(e Expr) (Expr, int32) {
	switch e := e.(type) {
	case *Const:
		return nil, e.Int()

	case *Op:
		if e.T != Long || (e.Op != Add && e.Op != Sub) {
			break
		}

		if c, ok := e.Y.(*Const); ok {
			v, k := split(e.X)
			if e.Op == Sub {
				return v, k - c.Int()
			}

			return v, k + c.Int()
		}

		if c, ok := e.X.(*Const); ok && e.Op == Add {
			v, k := split(e.Y)

			return v, k + c.Int()
		}
	}

	return e, 0
}

// linear is x+y or x-y for Longs, with their constants collected into
// one, added (or subtracted) last.
func linear(op Operator, x, y Expr) Expr {
	vx, kx := split(x)
	vy, ky := split(y)

	var v Expr

	k := kx + ky
	if op == Sub {
		k = kx - ky
	}

	switch {
	case vy == nil:
		v = vx
	case vx == nil && op == Sub && k != 0:
		// 5 - I stays a subtraction (SUBL3 I,#5,...).
		return &Op{Sub, Long, LongConst(k), vy}
	case vx == nil && op == Sub:
		v = &Op{Neg, Long, vy, nil}
	case vx == nil:
		v = vy
	default:
		v = &Op{op, Long, vx, vy}
	}

	switch {
	case v == nil:
		return LongConst(k)
	case k == 0:
		return v
	case k < 0 && k != math.MinInt32:
		return &Op{Sub, Long, v, LongConst(-k)}
	}

	return &Op{Add, Long, v, LongConst(k)}
}

// foldUnary is op on the constant x in type t, or nil if it isn't done
// now.
func foldUnary(op Operator, t Type, x *Const) *Const {
	switch {
	case op == Not:
		return LongConst(^x.Int())

	case op == Neg && t == Long:
		return LongConst(-x.Int())

	case op == Neg:
		return FloatConst(x.Real().Neg())

	case op == Cvt && t == Float:
		// CVTLF: exact up to 2**24, rounded beyond.
		v, _ := vaxfloat.Round(vaxfloat.F, vaxfloat.FromInt(int64(x.Int())))

		return FloatConst(v)

	case op == Cvt:
		// CVTFL truncates; a value out of a longword's range is left
		// for the CPU, which sets V.
		i := x.Real().Int(false)
		if !i.IsInt64() || i.Int64() < math.MinInt32 || i.Int64() > math.MaxInt32 {
			return nil
		}

		return LongConst(int32(i.Int64()))
	}

	return nil
}

// foldBinary is x op y for constants of type t, or nil if it isn't done
// now.
func foldBinary(op Operator, t Type, x, y *Const) *Const {
	if t == Long {
		a, b := x.Int(), y.Int()

		switch op {
		case Add:
			return LongConst(a + b)
		case Sub:
			return LongConst(a - b)
		case Mul:
			return LongConst(a * b)
		case Div:
			if b == 0 || (a == math.MinInt32 && b == -1) {
				return nil
			}

			return LongConst(a / b)
		case And:
			return LongConst(a & b)
		case Or:
			return LongConst(a | b)
		}

		return nil
	}

	var (
		v   vaxfloat.Value
		err error
	)

	switch op {
	case Add:
		v, err = vaxfloat.Add(vaxfloat.F, x.Real(), y.Real())
	case Sub:
		v, err = vaxfloat.Sub(vaxfloat.F, x.Real(), y.Real())
	case Mul:
		v, err = vaxfloat.Mul(vaxfloat.F, x.Real(), y.Real())
	case Div:
		v, err = vaxfloat.Div(vaxfloat.F, x.Real(), y.Real())
	default:
		return nil
	}

	if err != nil && !errors.Is(err, vaxfloat.ErrUnderflow) {
		return nil
	}

	return FloatConst(v)
}
