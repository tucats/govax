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
// returns its value in R0). A COMPLEX value is lowered whole only to be
// moved or passed: a load of the place holding it (lowercomplex.go).
func (l *lowerer) expr(e sema.Expr) Expr {
	if e.Type().Kind == sema.Complex {
		if c, ok := e.(*sema.Const); ok {
			return l.constOf(c)
		}

		return &Load{l.complexValue(e)}
	}

	switch e := e.(type) {
	case *sema.Const:
		return l.constOf(e)

	case *sema.Var, *sema.Elem:
		return &Load{l.mem(e)}

	case *sema.FuncRef:
		result := l.temp(l.typeOf(e.T))
		l.callFunction(e, result)

		return &Load{result}

	case *sema.IntrinsicRef:
		return l.intrinsic(e)

	case *sema.Loc:
		return l.loc(e)

	case *sema.Unary:
		op := Neg
		if e.Op == ast.Not {
			op = Not
		}

		return arith(op, l.typeOf(e.T), l.expr(e.X), nil)

	case *sema.Binary:
		if e.Op == ast.Pow {
			return l.power(e)
		}

		x := l.expr(e.X)
		y := l.expr(e.Y)
		t := l.typeOf(e.T)

		// .EQV. is the opposite of .NEQV., bit by bit: the VAX has XOR
		// but no instruction for its complement.
		if e.Op == ast.Eqv {
			return arith(Not, t, arith(Xor, t, x, y), nil)
		}

		return arith(operators[e.Op], t, x, y)

	case *sema.Compare:
		switch e.X.Type().Kind {
		case sema.Complex:
			return l.complexCompare(e)

		case sema.Character:
			// Computed before the statement, so that no register is held
			// across its CMPC (CharCmp).
			t := l.temp(Long)
			l.emit(&Assign{l.origin(), t, l.charCompare(e)})

			return &Load{t}
		}

		x := l.expr(e.X)
		y := l.expr(e.Y)

		return &Cmp{operators[e.Op], x, y}

	case *sema.Convert:
		// A COMPLEX value converted to anything else is its real part,
		// converted (Table 3-1).
		if e.X.Type().Kind == sema.Complex {
			re, _ := l.complexParts(e.X)

			return convertTo(l.typeOf(e.T), re)
		}

		// An INTEGER and a LOGICAL of one size are the same bits: converting
		// between them does nothing.
		x := l.expr(e.X)
		if t := l.typeOf(e.T); t != x.Type() {
			return arith(Cvt, t, x, nil)
		}

		return x
	}

	panic("ir: an expression semantic analysis left that lowering can't lower")
}

// cond lowers a Branch's condition. It is expr's lowering, but for a
// character comparison, which is kept in the condition rather than computed
// before it, as long as only .AND., .OR., and .NOT. are above it: the code
// generator branches on each of those as it goes, and holds no register
// across a comparison.
func (l *lowerer) cond(e sema.Expr) Expr {
	switch e := e.(type) {
	case *sema.Compare:
		if e.X.Type().Kind == sema.Character {
			// A comparison of a dynamic string is computed before the
			// branch, so that the string can be freed before it (after).
			n := len(l.after)
			c := l.charCompare(e)

			if len(l.after) == n {
				return c
			}

			t := l.temp(Long)
			l.emit(&Assign{l.origin(), t, c})

			return &Load{t}
		}

	case *sema.Unary:
		if e.Op == ast.Not && e.T.Kind == sema.Logical {
			return arith(Not, l.typeOf(e.T), l.cond(e.X), nil)
		}

	case *sema.Binary:
		if (e.Op == ast.And || e.Op == ast.Or) && e.T.Kind == sema.Logical {
			return arith(operators[e.Op], l.typeOf(e.T), l.cond(e.X), l.cond(e.Y))
		}
	}

	return l.expr(e)
}

// charCompare lowers a comparison of two CHARACTER values.
func (l *lowerer) charCompare(e *sema.Compare) *CharCmp {
	return &CharCmp{operators[e.Op], l.chars(e.X), l.chars(e.Y)}
}

// chars is where a CHARACTER value is, and its length: a constant's text, a
// variable, an array element, a substring of either, or a concatenation's
// temporary (concat). S(I:J) is S's place I-1 bytes on, the constant part of
// that in the offset and the rest added to the index, and J-I+1 characters
// long (S's length if J is left out).
func (l *lowerer) chars(e sema.Expr) Text {
	switch e := e.(type) {
	case *sema.Const:
		return Text{l.text(e.Text), LongConst(int32(len(e.Text)))}

	case *sema.Var:
		return Text{l.mem(e), l.charLength(e.Sym)}

	case *sema.Elem:
		return Text{l.mem(e), l.charLength(e.Sym)}

	case *sema.Concat:
		return l.concat(e)

	case *sema.FuncRef:
		// The value is as long as this unit declares the function, in a
		// temporary of that length; a dummy procedure declared CHARACTER*(*)
		// is as long as the unit that passed it says, which only its
		// descriptor knows, in a dynamic string.
		n := l.charLength(e.Fn)

		var result *Mem
		if e.T.Size > 0 {
			result = l.textTemp(e.T.Size)
		} else {
			result = l.dynamicText(n)
		}

		l.callFunction(e, result)

		return Text{result, n}

	case *sema.IntrinsicRef:
		// CHAR(I), the one intrinsic function with a CHARACTER value.
		return l.char(e)

	case *sema.Substring:
		t := l.chars(e.X)
		lower := l.expr(e.Lower)

		upper := t.Len
		if e.Upper != nil {
			upper = l.expr(e.Upper)
		}

		v, k := split(lower)
		m := *t.Mem
		m.Offset += int(k) - 1
		m.Index = arith(Add, Long, m.Index, v)

		return Text{&m, arith(Sub, Long, arith(Add, Long, upper, LongConst(1)), lower)}
	}

	panic("ir: a CHARACTER value semantic analysis left that lowering can't lower")
}

// operators are the IR's operators for FORTRAN's.
var operators = map[ast.Op]Operator{
	ast.Add: Add, ast.Sub: Sub, ast.Mul: Mul, ast.Div: Div,
	ast.And: And, ast.Or: Or, ast.Neqv: Xor, ast.Xor: Xor,
	ast.EQ: Eq, ast.NE: Ne, ast.LT: Lt, ast.LE: Le, ast.GT: Gt, ast.GE: Ge,
}

// constOf is a constant's IR form: an INTEGER's or a LOGICAL's bits in its
// own size (a LOGICAL's -1 or 0), a REAL's bits in its format, or a
// COMPLEX's two parts' bits.
func (l *lowerer) constOf(c *sema.Const) *Const {
	switch c.T.Kind {
	case sema.Real:
		return RealConst(l.typeOf(c.T), c.Real)
	case sema.Complex:
		p := l.partOf(c.T)

		return ComplexConst(RealConst(p, c.Real), RealConst(p, c.Imag))
	}

	return IntConst(l.typeOf(c.T), c.Int)
}

// symbolMem is where a variable or array is: its cell, or, for a dummy
// argument, the caller's variable its argument points at, or a CHARACTER
// dummy's text, which its descriptor points at.
func (l *lowerer) symbolMem(s *sema.Symbol) *Mem {
	t := l.typeOf(s.Type)

	if p := l.charParam(s); p != nil {
		return dummyText(p)
	}

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
		if c := l.adjustable[e.Sym]; c != nil {
			return l.adjustableElement(e, c)
		}

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

			multiplier *= dim.Extent()
		}

		// A CHARACTER element's place is a Byte place, whose index the VAX
		// doesn't multiply by the element's length: it is multiplied here.
		// A passed-length array's length is known only when the program
		// runs, so its first element's offset is part of the index too.
		if e.Sym.Type.Kind == sema.Character {
			if e.Sym.Type.Size == 0 && first != 0 {
				index = arith(Add, Long, index, LongConst(int32(first)))
				first = 0
			}

			if index != nil {
				index = arith(Mul, Long, index, l.charLength(e.Sym))
			}
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

// arith is the operation op on x and y (y nil for Neg, Not, and Cvt) in
// type t, folded when it can be:
//
//   - an operation on constants is done now, an integer one in its type's
//     bits as the VAX's instructions do it (wrapping around on overflow,
//     which the code doesn't detect), a floating one with package vaxfloat,
//     which rounds as the VAX does; an operation that would overflow, divide
//     by zero, or give a reserved operand is left for run time;
//   - a Long addition or subtraction collects its constants into one
//     (I-1+1 is I), and adding or multiplying by nothing goes, which
//     keeps DO loops' counts and array subscripts short;
//   - adding or subtracting a floating 0.0 goes too.
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

	// A floating value plus or minus 0.0 is itself, exactly: the VAX has no
	// negative zero. A real number converted to COMPLEX has an imaginary
	// part of 0.0, which (X, 0.0) + (Y, Z) would otherwise add.
	// A word made a longword without its sign, then a word again, is the
	// word: a passed length stored into a descriptor's length.
	if z, ok := x.(*Op); ok && op == Cvt && z.Op == Zext && z.X.Type() == t {
		return z.X
	}

	if t.Floating() && (op == Add || op == Sub) {
		switch {
		case oky && cy.IsZero():
			return x
		case okx && cx.IsZero() && op == Add:
			return y
		case okx && cx.IsZero():
			return &Op{Neg, t, y, nil}
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
// one, added (or subtracted) last, and a part that isn't constant
// subtracted from itself gone.
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
	case op == Sub && vx != nil && ExprText(vx) == ExprText(vy):
		// I - I is 0, whatever I is: an expression has no side effects
		// (lowering took the calls out), and two that read alike have the
		// same value. So S(I:I) is one character long.
		v = nil
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
		return IntConst(t, ^x.Int())

	case op == Neg && t.Integer():
		return IntConst(t, -x.Int())

	case op == Neg:
		return RealConst(t, x.Real().Neg())

	case op == Cvt && t.Floating():
		// CVTLF: exact up to 2**24, rounded beyond; CVTLD and the wider ones
		// always exact. CVTFD and the other widenings are exact, CVTDF and
		// the other narrowings round (Table 3-1). A narrowing that would
		// overflow (an H value past D's range) is left for the CPU.
		v := vaxfloat.FromInt(int64(x.Int()))
		if x.T.Floating() {
			v = x.Real()
		}

		r, err := vaxfloat.Round(t.Format(), v)
		if err != nil && !errors.Is(err, vaxfloat.ErrUnderflow) {
			return nil
		}

		return RealConst(t, r)

	case op == Cvt && x.T.Integer():
		// A shorter type's value is the longer one's low bits.
		return IntConst(t, x.Int())

	case op == Zext:
		// The bits as memory holds them, read as an unsigned number.
		return IntConst(t, int32(uint32(x.Bits)))

	case op == Low:
		// A shorter type's value is the longer one's low bits.
		return IntConst(t, x.Int())

	case op == Round:
		// CVTRFL rounds halves away from zero; a value out of a Long's range
		// is left for the CPU, which sets V.
		i := x.Real().Int(true)
		if !i.IsInt64() || i.Int64() < math.MinInt32 || i.Int64() > math.MaxInt32 {
			return nil
		}

		return LongConst(int32(i.Int64()))

	case op == Cvt:
		// CVTFL truncates; a value out of the type's range is left
		// for the CPU, which sets V.
		lo, hi := intRange(t)

		i := x.Real().Int(false)
		if !i.IsInt64() || i.Int64() < lo || i.Int64() > hi {
			return nil
		}

		return IntConst(t, int32(i.Int64()))
	}

	return nil
}

// intRange is the range of an integer type's values.
func intRange(t Type) (lo, hi int64) {
	switch t {
	case Byte:
		return math.MinInt8, math.MaxInt8
	case Word:
		return math.MinInt16, math.MaxInt16
	}

	return math.MinInt32, math.MaxInt32
}

// foldBinary is x op y for constants of type t, or nil if it isn't done
// now.
func foldBinary(op Operator, t Type, x, y *Const) *Const {
	if t.Integer() {
		a, b := x.Int(), y.Int()
		lo, _ := intRange(t)

		switch op {
		case Add:
			return IntConst(t, a+b)
		case Sub:
			return IntConst(t, a-b)
		case Mul:
			return IntConst(t, a*b)
		case Div:
			if b == 0 || (int64(a) == lo && b == -1) {
				return nil
			}

			return IntConst(t, a/b)
		case And:
			return IntConst(t, a&b)
		case Or:
			return IntConst(t, a|b)
		case Xor:
			return IntConst(t, a^b)
		case Ash:
			return LongConst(shiftArithmetic(a, b))
		}

		return nil
	}

	var (
		v   vaxfloat.Value
		err error
	)

	f := t.Format()

	switch op {
	case Add:
		v, err = vaxfloat.Add(f, x.Real(), y.Real())
	case Sub:
		v, err = vaxfloat.Sub(f, x.Real(), y.Real())
	case Mul:
		v, err = vaxfloat.Mul(f, x.Real(), y.Real())
	case Div:
		v, err = vaxfloat.Div(f, x.Real(), y.Real())
	default:
		return nil
	}

	if err != nil && !errors.Is(err, vaxfloat.ErrUnderflow) {
		return nil
	}

	return RealConst(t, v)
}

// shiftArithmetic is ASHL's value: x shifted left n places, or right -n
// places with its sign copied in from the left; n is the count's low byte, a
// signed number, so a shift of 32 or more leaves 0 to the left and the sign
// to the right.
func shiftArithmetic(x, n int32) int32 {
	n = int32(int8(n))

	switch {
	case n >= 32:
		return 0
	case n >= 0:
		return x << uint(n)
	case n <= -32:
		return x >> 31
	}

	return x >> uint(-n)
}

// field is the bit field of x (a Long) size bits from bit pos, as an unsigned
// Long (Field), folded when all three are constants.
func field(x, pos, size Expr) Expr {
	cx, okx := x.(*Const)
	cp, okp := pos.(*Const)
	cs, oks := size.(*Const)

	if okx && okp && oks && cp.Int() >= 0 && cs.Int() >= 0 && cp.Int()+cs.Int() <= 32 {
		bits := uint64(uint32(cx.Int())) >> uint(cp.Int())

		return LongConst(int32(uint32(bits & (1<<uint(cs.Int()) - 1))))
	}

	return &Field{x, pos, size}
}
