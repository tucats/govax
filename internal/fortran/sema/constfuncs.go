package sema

import (
	"math/big"
	"strconv"

	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/vaxfloat"
)

// The intrinsic functions a constant expression may use (section 4.11):
// MIN, MAX, ABS, MOD, ICHAR, NINT, DIM, DPROD, CMPLX, CONJG, and IMAG in
// arithmetic expressions; IAND, IOR, NOT, IEOR, ISHFT, LGE, LGT, LLE, and
// LLT in logical ones; CHAR in character ones. They are the generic
// names, whose result's type follows their arguments'. Section 4.11 says
// IMAG where appendix D's table has AIMAG; both are taken here.

// constFunc is one of them: how many arguments it takes (max -1 for any
// number), as INTRINARGS says it, and its value.
type constFunc struct {
	min, max int
	takes    string
	eval     func(ev *evaluator, e *ast.Apply, args []*Const) (*Const, *constErr)
}

// constFuncs are the functions, by name.
var constFuncs = map[string]constFunc{
	"MIN":   {2, -1, "2 or more arguments", minMax(-1)},
	"MAX":   {2, -1, "2 or more arguments", minMax(1)},
	"ABS":   {1, 1, "1 argument", constAbs},
	"MOD":   {2, 2, "2 arguments", constMod},
	"DIM":   {2, 2, "2 arguments", constDim},
	"ICHAR": {1, 1, "1 argument", constIchar},
	"NINT":  {1, 1, "1 argument", constNint},
	"DPROD": {2, 2, "2 arguments", constDprod},
	"CMPLX": {1, 2, "1 or 2 arguments", constCmplx},
	"CONJG": {1, 1, "1 argument", constConjg},
	"IMAG":  {1, 1, "1 argument", constImag},
	"AIMAG": {1, 1, "1 argument", constImag},
	"IAND":  {2, 2, "2 arguments", bitwise(func(x, y int32) int32 { return x & y })},
	"IOR":   {2, 2, "2 arguments", bitwise(func(x, y int32) int32 { return x | y })},
	"IEOR":  {2, 2, "2 arguments", bitwise(func(x, y int32) int32 { return x ^ y })},
	"NOT":   {1, 1, "1 argument", constBitNot},
	"ISHFT": {2, 2, "2 arguments", constIshft},
	"LGE":   {2, 2, "2 arguments", lexical(func(o int) bool { return o >= 0 })},
	"LGT":   {2, 2, "2 arguments", lexical(func(o int) bool { return o > 0 })},
	"LLE":   {2, 2, "2 arguments", lexical(func(o int) bool { return o <= 0 })},
	"LLT":   {2, 2, "2 arguments", lexical(func(o int) bool { return o < 0 })},
	"CHAR":  {1, 1, "1 argument", constChar},
}

// apply is a function reference in a constant expression: one of
// constFuncs, of constant arguments. An array element, or any other
// function, isn't constant.
func (ev *evaluator) apply(e *ast.Apply) (*Const, *constErr) {
	if s := ev.a.unit.byName[e.Name]; s != nil && s.Class != Intrinsic && (s.Class != Unclassified || s.Dummy != 0) {
		if s.Class == Array {
			return nil, named("the array element "+e.Name+"(...) is not a constant", e.Pos)
		}

		return nil, named(e.Name+" is "+s.Class.String()+", not a constant", e.Pos)
	}

	fn, ok := constFuncs[e.Name]
	if !ok {
		if intrinsics[e.Name] {
			return nil, notConst("the intrinsic function "+e.Name+" can't be computed when compiling", e.Pos)
		}

		return nil, named(e.Name+" is a function, not a constant", e.Pos)
	}

	if n := len(e.Args); n < fn.min || fn.max >= 0 && n > fn.max {
		return nil, ev.report("INTRINARGS", e.Pos, e.Name, fn.takes, n)
	}

	args := make([]*Const, len(e.Args))

	var err *constErr

	for i, arg := range e.Args {
		if arg == nil {
			return nil, ev.report("INTRINARGS", e.Pos, e.Name, fn.takes, len(e.Args))
		}

		var argErr *constErr

		args[i], argErr = ev.eval(arg)
		err = worse(err, argErr)
	}

	if err != nil {
		return nil, err
	}

	return fn.eval(ev, e, args)
}

// badArg reports an argument of a type or value the function doesn't
// take (BADARG): want is what it takes, and the argument is shown by its
// type.
func (ev *evaluator) badArg(e *ast.Apply, want string, arg *Const) *constErr {
	return ev.report("BADARG", arg.Pos, e.Name, want, arg.T.String())
}

// alike converts arguments of the kinds allowed to one type, the highest
// of theirs. Every argument must be of one kind, the first's (a generic
// function's arguments must agree, as VAX FORTRAN's do).
func (ev *evaluator) alike(e *ast.Apply, args []*Const, kinds ...Kind) ([]*Const, *constErr) {
	allowed := false

	for _, k := range kinds {
		allowed = allowed || args[0].T.Kind == k
	}

	if !allowed {
		want := ""

		for i, k := range kinds {
			switch {
			case i == 0:
			case i == len(kinds)-1:
				want += " or "
			default:
				want += ", "
			}

			want += Type{k, 4}.String()
		}

		return nil, ev.badArg(e, want, args[0])
	}

	t := args[0].T

	for _, arg := range args[1:] {
		if arg.T.Kind != t.Kind {
			return nil, ev.badArg(e, t.String()+", like the first", arg)
		}

		t = higher(t, arg.T)
	}

	out := make([]*Const, len(args))

	for i, arg := range args {
		c, err := ev.convert(arg, t, arg.Pos)
		if err != nil {
			return nil, err
		}

		out[i] = c
	}

	return out, nil
}

// minMax is MIN (sign -1) or MAX (1) of INTEGER or REAL arguments.
func minMax(sign int) func(*evaluator, *ast.Apply, []*Const) (*Const, *constErr) {
	return func(ev *evaluator, e *ast.Apply, args []*Const) (*Const, *constErr) {
		args, err := ev.alike(e, args, Integer, Real)
		if err != nil {
			return nil, err
		}

		best := args[0]

		for _, arg := range args[1:] {
			if order(arg, best)*sign > 0 {
				best = arg
			}
		}

		return best.at(e.Pos), nil
	}
}

// constAbs is ABS: an INTEGER's or REAL's magnitude, or a COMPLEX's
// modulus, the square root of the sum of its parts' squares, a REAL of
// the parts' type.
func constAbs(ev *evaluator, e *ast.Apply, args []*Const) (*Const, *constErr) {
	x := args[0]
	c := &Const{exprBase: exprBase{x.T, e.Pos}}

	switch x.T.Kind {
	case Integer:
		v := int64(x.Int)
		if v < 0 {
			v = -v
		}

		if _, hi := x.T.intRange(); v > hi {
			return nil, ev.intRange(e.Pos, valueText, x.T)
		}

		c.Int = int32(v)

	case Real:
		c.Real = x.Real.Abs()

	case Complex:
		t := x.T.part()
		sum := vaxfloat.AddExact(vaxfloat.MulExact(x.Real, x.Real), vaxfloat.MulExact(x.Imag, x.Imag))

		v, err := vaxfloat.Sqrt(ev.a.format(t), sum)
		if err != nil {
			return nil, ev.realRange(e.Pos, valueText, t)
		}

		c.T, c.Real = t, v

	default:
		return nil, ev.badArg(e, "INTEGER, REAL, or COMPLEX", x)
	}

	return c, nil
}

// constMod is MOD(a, b), a - INT(a/b)*b: the remainder of a division
// truncated toward zero, with a's sign. A REAL remainder is computed
// exactly and rounded once.
func constMod(ev *evaluator, e *ast.Apply, args []*Const) (*Const, *constErr) {
	args, err := ev.alike(e, args, Integer, Real)
	if err != nil {
		return nil, err
	}

	x, y := args[0], args[1]
	c := &Const{exprBase: exprBase{x.T, e.Pos}}

	if x.T.Kind == Integer {
		if y.Int == 0 {
			return nil, ev.report("DIVZERO", e.Pos)
		}

		// Go's % has the dividend's sign, as MOD does.
		c.Int = int32(int64(x.Int) % int64(y.Int))

		return c, nil
	}

	if y.Real.IsZero() {
		return nil, ev.report("DIVZERO", e.Pos)
	}

	q := new(big.Rat).Quo(x.Real.Rat(), y.Real.Rat())
	whole := new(big.Int).Quo(q.Num(), q.Denom()) // truncated toward zero
	rem := new(big.Rat).Sub(x.Real.Rat(), new(big.Rat).Mul(new(big.Rat).SetInt(whole), y.Real.Rat()))

	v, rerr := round(ev.a.format(x.T), vaxfloat.FromRat(rem))
	if rerr != nil {
		return nil, ev.realRange(e.Pos, valueText, x.T)
	}

	c.Real = v

	return c, nil
}

// constDim is DIM(a, b): a - b if a is greater, else zero.
func constDim(ev *evaluator, e *ast.Apply, args []*Const) (*Const, *constErr) {
	args, err := ev.alike(e, args, Integer, Real)
	if err != nil {
		return nil, err
	}

	if order(args[0], args[1]) <= 0 {
		return &Const{exprBase: exprBase{args[0].T, e.Pos}}, nil
	}

	return ev.operate(ast.Sub, args[0], args[1], e.Pos)
}

// constIchar is ICHAR(c): the ASCII code of a CHARACTER*1, an INTEGER.
func constIchar(ev *evaluator, e *ast.Apply, args []*Const) (*Const, *constErr) {
	x := args[0]
	if x.T.Kind != Character || len(x.Text) != 1 {
		return nil, ev.badArg(e, "CHARACTER*1", x)
	}

	return &Const{exprBase: exprBase{IntegerType, e.Pos}, Int: int32(x.Text[0])}, nil
}

// constNint is NINT(r): a REAL rounded to the nearest INTEGER, halves
// away from zero.
func constNint(ev *evaluator, e *ast.Apply, args []*Const) (*Const, *constErr) {
	x := args[0]
	if x.T.Kind != Real {
		return nil, ev.badArg(e, "REAL", x)
	}

	i := x.Real.Int(true)
	if !i.IsInt64() || i.Int64() < minInt || i.Int64() > maxInt {
		return nil, ev.intRange(e.Pos, valueText, IntegerType)
	}

	return &Const{exprBase: exprBase{IntegerType, e.Pos}, Int: int32(i.Int64())}, nil
}

// constDprod is DPROD(x, y): the REAL*8 product of two REAL*4 values,
// which is exact.
func constDprod(ev *evaluator, e *ast.Apply, args []*Const) (*Const, *constErr) {
	for _, arg := range args {
		if arg.T != RealType {
			return nil, ev.badArg(e, "REAL*4", arg)
		}
	}

	t := Type{Real, 8}

	v, err := round(ev.a.format(t), vaxfloat.MulExact(args[0].Real, args[1].Real))
	if err != nil {
		return nil, ev.realRange(e.Pos, valueText, t)
	}

	return &Const{exprBase: exprBase{t, e.Pos}, Real: v}, nil
}

// constCmplx is CMPLX(x) or CMPLX(x, y): a COMPLEX*8 of one number (a
// COMPLEX's value, or a real part with an imaginary part of zero) or of
// two that aren't COMPLEX.
func constCmplx(ev *evaluator, e *ast.Apply, args []*Const) (*Const, *constErr) {
	t := Type{Complex, 8}

	for _, arg := range args {
		if !arg.T.numeric() || len(args) == 2 && arg.T.Kind == Complex {
			want := "INTEGER, REAL, or COMPLEX"
			if len(args) == 2 {
				want = "INTEGER or REAL"
			}

			return nil, ev.badArg(e, want, arg)
		}
	}

	if len(args) == 1 {
		c, err := ev.convert(args[0], t, e.Pos)
		if err != nil {
			return nil, err
		}

		return c.at(e.Pos), nil
	}

	re, err := ev.convert(args[0], RealType, e.Pos)
	if err != nil {
		return nil, err
	}

	im, err := ev.convert(args[1], RealType, e.Pos)
	if err != nil {
		return nil, err
	}

	return &Const{exprBase: exprBase{t, e.Pos}, Real: re.Real, Imag: im.Real}, nil
}

// constConjg is CONJG(z): z with its imaginary part negated.
func constConjg(ev *evaluator, e *ast.Apply, args []*Const) (*Const, *constErr) {
	x := args[0]
	if x.T.Kind != Complex {
		return nil, ev.badArg(e, "COMPLEX", x)
	}

	return &Const{exprBase: exprBase{x.T, e.Pos}, Real: x.Real, Imag: x.Imag.Neg()}, nil
}

// constImag is IMAG(z), AIMAG(z): z's imaginary part, a REAL of the
// parts' type.
func constImag(ev *evaluator, e *ast.Apply, args []*Const) (*Const, *constErr) {
	x := args[0]
	if x.T.Kind != Complex {
		return nil, ev.badArg(e, "COMPLEX", x)
	}

	return &Const{exprBase: exprBase{x.T.part(), e.Pos}, Real: x.Imag}, nil
}

// bitwise is IAND, IOR, or IEOR of two INTEGERs, op bit by bit.
func bitwise(op func(x, y int32) int32) func(*evaluator, *ast.Apply, []*Const) (*Const, *constErr) {
	return func(ev *evaluator, e *ast.Apply, args []*Const) (*Const, *constErr) {
		args, err := ev.alike(e, args, Integer)
		if err != nil {
			return nil, err
		}

		return &Const{exprBase: exprBase{args[0].T, e.Pos}, Int: op(args[0].Int, args[1].Int)}, nil
	}
}

// constBitNot is NOT(i): every bit of an INTEGER complemented.
func constBitNot(ev *evaluator, e *ast.Apply, args []*Const) (*Const, *constErr) {
	x := args[0]
	if x.T.Kind != Integer {
		return nil, ev.badArg(e, "INTEGER", x)
	}

	return &Const{exprBase: exprBase{x.T, e.Pos}, Int: ^x.Int}, nil
}

// constIshft is ISHFT(i, n): i's bits shifted left n places, or right -n
// places, zeros shifted in at either end (a logical shift); a shift of
// the whole width or more leaves zero.
func constIshft(ev *evaluator, e *ast.Apply, args []*Const) (*Const, *constErr) {
	x, n := args[0], args[1]

	for _, arg := range args {
		if arg.T.Kind != Integer {
			return nil, ev.badArg(e, "INTEGER", arg)
		}
	}

	width := int32(x.T.Size * 8)
	mask := uint32(1)<<width - 1

	if width == 32 {
		mask = ^uint32(0)
	}

	bits := uint32(x.Int) & mask

	switch shift := n.Int; {
	case shift >= width || shift <= -width:
		bits = 0
	case shift >= 0:
		bits <<= uint(shift)
	default:
		bits >>= uint(-shift)
	}

	return &Const{exprBase: exprBase{x.T, e.Pos}, Int: truncate(int32(bits&mask), x.T.Size)}, nil
}

// lexical is LGE, LGT, LLE, or LLT: two CHARACTER values compared in
// ASCII, the shorter padded with blanks.
func lexical(holds func(order int) bool) func(*evaluator, *ast.Apply, []*Const) (*Const, *constErr) {
	return func(ev *evaluator, e *ast.Apply, args []*Const) (*Const, *constErr) {
		for _, arg := range args {
			if arg.T.Kind != Character {
				return nil, ev.badArg(e, "CHARACTER", arg)
			}
		}

		return logicalConst(holds(compareText(args[0].Text, args[1].Text)), e.Pos), nil
	}
}

// constChar is CHAR(i): the CHARACTER*1 whose ASCII code is i, 0 to 255.
func constChar(ev *evaluator, e *ast.Apply, args []*Const) (*Const, *constErr) {
	x := args[0]

	switch {
	case x.T.Kind != Integer:
		return nil, ev.badArg(e, "INTEGER", x)
	case x.Int < 0 || x.Int > 255:
		return nil, ev.report("BADARG", x.Pos, e.Name, "INTEGER values from 0 to 255", strconv.Itoa(int(x.Int)))
	}

	return charConst(string([]byte{byte(x.Int)}), e.Pos), nil
}
