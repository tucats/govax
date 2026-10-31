package sema

import (
	"errors"
	"math/big"
	"strings"

	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
	"github.com/tucats/govax/internal/vaxfloat"
)

// Compile-time constant expressions (section 4.11): expressions whose
// operands are constants, the names of constants, a few intrinsic
// functions of constant arguments, and other constant expressions. Their
// values are computed here, in every type: INTEGER*2 and INTEGER*4,
// REAL*4, REAL*8, and REAL*16 (through vaxfloat, which rounds as the
// VAX's floating instructions do), COMPLEX*8 and COMPLEX*16, LOGICAL, and
// CHARACTER.
//
// The evaluator works on the syntax tree, not on sema's typed
// expressions, because it is used where names mustn't be entered in the
// symbol table as variables: a PARAMETER statement's value, an array's
// bounds, a CHARACTER length. An executable statement's expression is
// analyzed as usual (expr.go), and comes here only for the forms that
// analysis doesn't compile yet but a constant expression may hold (the
// ** operator, the intrinsic functions, a REAL*8 constant): if the form
// is constant, its value is a constant like any other (folded).
//
// A constant expression whose types the code generator doesn't handle
// yet still has its value: a PARAMETER statement may define a REAL*8 or
// COMPLEX constant, which only its uses in expressions find NOTIMPL.

// constKind is what an expression turned out to be, if not a constant.
type constKind int

const (
	constOK         constKind = iota
	constNamed                // the expression names something that isn't a constant
	constNotInteger           // it isn't an integer expression (constInt)
	constNot                  // it uses a form a constant expression may not
	constFailed               // an error, already reported
)

// constErr says why an expression has no constant value: its kind and,
// for constNamed and constNot, the reason, as NOTCONST ends its message
// ("X is not a constant"), and where it is.
type constErr struct {
	kind constKind
	why  string
	pos  diag.Position
}

// failed is an expression with an error, already reported.
var failed = &constErr{kind: constFailed}

// incompatible is a conversion between types that have none: a REAL
// value to a LOGICAL constant, an INTEGER to a CHARACTER. The caller
// reports it.
var incompatible = &constErr{kind: constNot, why: "its type can't be converted"}

// named and notConst are an expression that isn't constant, for why, at
// pos.
func named(why string, pos diag.Position) *constErr {
	return &constErr{kind: constNamed, why: why, pos: pos}
}

func notConst(why string, pos diag.Position) *constErr {
	return &constErr{kind: constNot, why: why, pos: pos}
}

// worse is whichever of two errors matters more: one already reported
// (so no second message is wanted), else the first.
func worse(x, y *constErr) *constErr {
	switch {
	case x == nil:
		return y
	case y != nil && y.kind == constFailed:
		return y
	}

	return x
}

// evaluator computes constant expressions for one unit. quiet makes it
// report nothing, to learn whether an expression is constant at all
// before it is evaluated for its messages; typeless allows octal and
// hexadecimal constants, which only declarations may use yet (their use
// in executable statements is Phase 82's). vars are the implied-DO
// variables in scope, in a DATA statement's list, with their values now.
type evaluator struct {
	a        *analyzer
	quiet    bool
	typeless bool
	vars     map[string]int32
}

// declEval is the evaluator for declarations.
func (a *analyzer) declEval() *evaluator {
	return &evaluator{a: a, typeless: true}
}

// report reports a message, unless the evaluator is quiet, and returns
// failed.
func (ev *evaluator) report(ident string, pos diag.Position, args ...any) *constErr {
	if !ev.quiet {
		ev.a.report(ident, pos, args...)
	}

	return failed
}

// notImpl and notImplForm report NOTIMPL as the analyzer's do.
func (ev *evaluator) notImpl(pos diag.Position, what string) *constErr {
	if !ev.quiet {
		ev.a.notImpl(pos, what)
	}

	return failed
}

func (ev *evaluator) notImplForm(pos diag.Position, what string) *constErr {
	if !ev.quiet {
		ev.a.notImplForm(pos, what)
	}

	return failed
}

// intRange and realRange report a value outside type t's range.
func (ev *evaluator) intRange(pos diag.Position, what string, t Type) *constErr {
	if !ev.quiet {
		ev.a.intRangeError(pos, what, t)
	}

	return failed
}

func (ev *evaluator) realRange(pos diag.Position, what string, t Type) *constErr {
	if !ev.quiet {
		ev.a.realRangeError(pos, what, t)
	}

	return failed
}

// eval is the value of the constant expression e.
func (ev *evaluator) eval(e ast.Expr) (*Const, *constErr) {
	switch e := e.(type) {
	case *ast.IntConst:
		v, err := intValue(e)
		if err != nil || v > maxInt {
			return nil, ev.intRange(e.Pos, ast.DumpExpr(e), IntegerType)
		}

		return &Const{exprBase: exprBase{ev.a.constantType(v), e.Pos}, Int: int32(v)}, nil

	case *ast.RealConst:
		return ev.real(e)

	case *ast.LogicalConst:
		return logicalConst(e.Value, e.Pos), nil

	case *ast.CharConst:
		return charConst(e.Value, e.Pos), nil

	case *ast.ComplexConst:
		return ev.complex(e)

	case *ast.TypelessConst:
		if !ev.typeless {
			return nil, ev.notImplForm(e.Pos, formWhat(e))
		}

		// "When the constant is used in any other context, an INTEGER*4
		// data type is assumed" (section 2.2.1.4).
		return ev.typelessAs(e, ev.a.defaultInt)

	case *ast.Name:
		if v, ok := ev.vars[e.Name]; ok {
			return &Const{exprBase: exprBase{IntegerType, e.Pos}, Int: v}, nil
		}

		s := ev.a.unit.byName[e.Name]

		switch {
		case s == nil || s.Class != Constant:
			return nil, named(e.Name+" is not a constant", e.Pos)
		case s.Value == nil:
			return nil, failed
		}

		return s.Value.at(e.Pos), nil

	case *ast.Paren:
		return ev.eval(e.X)

	case *ast.Unary:
		return ev.unary(e)

	case *ast.Binary:
		return ev.binary(e)

	case *ast.Apply:
		return ev.apply(e)
	}

	if what := formWhat(e); what != "" {
		return nil, ev.notImplForm(e.Position(), what)
	}

	return nil, notConst("this is not a constant", e.Position())
}

// charConst is a CHARACTER constant.
func charConst(text string, pos diag.Position) *Const {
	return &Const{exprBase: exprBase{Type{Character, len(text)}, pos}, Text: text}
}

// real is the constant a real literal writes (section 2.2.1.2): REAL*4,
// or REAL*8 with a D exponent, REAL*16 with a Q one, rounded to its
// format. It keeps its exact value too, for an operation of higher
// precision (Const.literal).
func (ev *evaluator) real(e *ast.RealConst) (*Const, *constErr) {
	t, text := RealType, e.Text

	switch {
	case strings.ContainsRune(text, 'D'):
		t, text = Type{Real, 8}, strings.Replace(text, "D", "E", 1)
	case strings.ContainsRune(text, 'Q'):
		t, text = Type{Real, 16}, strings.Replace(text, "Q", "E", 1)
	}

	exact, err := vaxfloat.Parse(text)
	if err != nil {
		return nil, ev.realRange(e.Pos, e.Text, t)
	}

	v, err := vaxfloat.Round(ev.a.format(t), exact)
	if err != nil {
		return nil, ev.realRange(e.Pos, e.Text, t)
	}

	return &Const{exprBase: exprBase{t, e.Pos}, Real: v, literal: true, exactRe: exact}, nil
}

// complex is a complex constant, (re, im) (section 2.2.1.3): each part
// an INTEGER or REAL constant, signed or not, or a constant's name
// (section 4.11), anything else BADCOMPLEX. It is COMPLEX*16 if a part
// is REAL*8 or REAL*16, else COMPLEX*8.
func (ev *evaluator) complex(e *ast.ComplexConst) (*Const, *constErr) {
	part := func(x ast.Expr) (*Const, *constErr) {
		inner := x
		if u, ok := x.(*ast.Unary); ok && (u.Op == ast.Add || u.Op == ast.Sub) {
			inner = u.X
		}

		switch inner.(type) {
		case *ast.IntConst, *ast.RealConst, *ast.Name:
		default:
			return nil, ev.report("BADCOMPLEX", x.Position())
		}

		c, err := ev.eval(x)

		switch {
		case err != nil && err.kind == constFailed:
			return nil, err
		case err != nil, c.T.Kind != Integer && c.T.Kind != Real:
			return nil, ev.report("BADCOMPLEX", x.Position())
		}

		return c, nil
	}

	re, err := part(e.Re)
	im, err2 := part(e.Im)

	if err = worse(err, err2); err != nil {
		return nil, err
	}

	t := Type{Complex, 8}
	if re.T.Kind == Real && re.T.Size > 4 || im.T.Kind == Real && im.T.Size > 4 {
		t = Type{Complex, 16}
	}

	pt := t.part()

	re, err = ev.convert(re, pt, e.Pos)
	im, err2 = ev.convert(im, pt, e.Pos)

	if err = worse(err, err2); err != nil {
		return nil, err
	}

	c := &Const{exprBase: exprBase{t, e.Pos}, Real: re.Real, Imag: im.Real, literal: true,
		exactRe: re.Real, exactIm: im.Real}

	if re.literal {
		c.exactRe = re.exactRe
	}

	if im.literal {
		c.exactIm = im.exactRe
	}

	return c, nil
}

// typelessAs is an octal or hexadecimal constant (section 2.2.1.4) given
// type t: its bits, extended on the left with zeros or cut on the left to
// t's size (section 4.3's rule), as the VAX holds a t in memory.
func (ev *evaluator) typelessAs(e *ast.TypelessConst, t Type) (*Const, *constErr) {
	v, ok := new(big.Int).SetString(e.Digits, e.Radix)
	if !ok {
		return nil, failed
	}

	lo := new(big.Int).And(v, new(big.Int).SetUint64(^uint64(0))).Uint64()
	hi := new(big.Int).Rsh(v, 64).Uint64()
	c := &Const{exprBase: exprBase{t, e.Pos}}

	switch t.Kind {
	case Integer, Logical:
		c.Int = truncate(int32(uint32(lo)), t.Size)

	case Real:
		f := ev.a.format(t)
		if f == vaxfloat.F {
			lo &= 0xFFFFFFFF
		}

		r, err := vaxfloat.Unpack(f, vaxfloat.Bits{Lo: lo, Hi: hi})
		if err != nil {
			return nil, ev.report("BADTYPELESS", e.Pos, t.String())
		}

		c.Real = r

	case Complex:
		f, reBits, imBits := ev.a.format(t), vaxfloat.Bits{Lo: lo}, vaxfloat.Bits{Lo: hi}
		if f == vaxfloat.F {
			reBits, imBits = vaxfloat.Bits{Lo: lo & 0xFFFFFFFF}, vaxfloat.Bits{Lo: lo >> 32}
		}

		re, err := vaxfloat.Unpack(f, reBits)
		im, err2 := vaxfloat.Unpack(f, imBits)

		if err != nil || err2 != nil {
			return nil, ev.report("BADTYPELESS", e.Pos, t.String())
		}

		c.Real, c.Imag = re, im

	case Character:
		return nil, ev.notImplForm(e.Pos, "an octal or hexadecimal constant given a CHARACTER type")

	default:
		return nil, failed
	}

	return c, nil
}

// truncate is v cut to an INTEGER or LOGICAL of size bytes, sign
// extended: the value such a datum holds.
func truncate(v int32, size int) int32 {
	switch size {
	case 1:
		return int32(int8(v))
	case 2:
		return int32(int16(v))
	}

	return v
}

// unary is -X, +X, or .NOT. X.
func (ev *evaluator) unary(e *ast.Unary) (*Const, *constErr) {
	// -2147483648 is INTEGER's smallest value, though 2147483648 alone is
	// out of range: the sign and the digits are taken together.
	if c, ok := e.X.(*ast.IntConst); ok && e.Op == ast.Sub && c.Text == "2147483648" && c.Radix == 0 {
		return &Const{exprBase: exprBase{IntegerType, e.Pos}, Int: minInt}, nil
	}

	x, err := ev.eval(e.X)
	if err != nil {
		return nil, err
	}

	if e.Op == ast.Not {
		switch x.T.Kind {
		case Logical, Integer:
			c := &Const{exprBase: exprBase{x.T, e.Pos}, Int: ^x.Int}

			return c, nil
		case Character:
			return nil, ev.notImpl(x.Pos, hollerith)
		}

		return nil, ev.report("BADOPERAND", e.Pos, ".NOT.", x.T.String())
	}

	if x, err = ev.arithmetic(x); err != nil || e.Op == ast.Add {
		return x, err
	}

	return ev.negate(x, e.Pos)
}

// negate is -x, x being an INTEGER, REAL, or COMPLEX constant.
func (ev *evaluator) negate(x *Const, pos diag.Position) (*Const, *constErr) {
	c := &Const{exprBase: exprBase{x.T, pos}}

	switch x.T.Kind {
	case Integer:
		v := -int64(x.Int)
		if lo, hi := x.T.intRange(); v < lo || v > hi {
			return nil, ev.intRange(pos, valueText, x.T)
		}

		c.Int = int32(v)

	default:
		c.Real, c.Imag = x.Real.Neg(), x.Imag.Neg()
		c.literal, c.exactRe, c.exactIm = x.literal, x.exactRe.Neg(), x.exactIm.Neg()
	}

	return c, nil
}

// assigned is c converted to type t as an assignment or a mixed operation
// converts it. That is convert, but for a REAL or COMPLEX value given to a
// LOGICAL, which Table 3-1 treats as it does an INTEGER (truncating), where
// a PARAMETER statement's constant has no such conversion.
func (ev *evaluator) assigned(c *Const, t Type, pos diag.Position) (*Const, *constErr) {
	if t.Kind == Logical && (c.T.Kind == Real || c.T.Kind == Complex) {
		i, err := ev.convert(c, IntegerType, pos)
		if err != nil {
			return nil, err
		}

		c = i
	}

	return ev.convert(c, t, pos)
}

// arithmetic makes x an arithmetic operand, as the analyzer's arithmetic
// does: a LOGICAL is an INTEGER, a CHARACTER a Hollerith constant.
func (ev *evaluator) arithmetic(x *Const) (*Const, *constErr) {
	switch x.T.Kind {
	case Logical:
		return ev.convert(x, x.T.integer(), x.Pos)
	case Character:
		return nil, ev.notImpl(x.Pos, hollerith)
	}

	return x, nil
}

// pair is the values of a binary operator's operands. An octal or
// hexadecimal constant takes the other operand's type (section 2.2.1.4).
func (ev *evaluator) pair(ex, ey ast.Expr) (*Const, *Const, *constErr) {
	tx, okx := ex.(*ast.TypelessConst)
	ty, oky := ey.(*ast.TypelessConst)

	if ev.typeless && okx != oky {
		if okx {
			y, err := ev.eval(ey)
			if err != nil {
				return nil, nil, err
			}

			x, err := ev.typelessAs(tx, contextType(y.T))

			return x, y, err
		}

		x, err := ev.eval(ex)
		if err != nil {
			return nil, nil, err
		}

		y, err := ev.typelessAs(ty, contextType(x.T))

		return x, y, err
	}

	x, err := ev.eval(ex)
	y, err2 := ev.eval(ey)

	return x, y, worse(err, err2)
}

// contextType is the type an octal or hexadecimal constant takes beside
// an operand of type t: t's, or INTEGER*4 beside a CHARACTER.
func contextType(t Type) Type {
	if t.Kind == Character {
		return IntegerType
	}

	return t
}

// binary is X op Y.
func (ev *evaluator) binary(e *ast.Binary) (*Const, *constErr) {
	x, y, err := ev.pair(e.X, e.Y)
	if err != nil {
		return nil, err
	}

	switch {
	case e.Op.Relational():
		return ev.compare(e, x, y)

	case e.Op == ast.And || e.Op == ast.Or || e.Op == ast.Eqv || e.Op == ast.Neqv || e.Op == ast.Xor:
		return ev.logical(e, x, y)

	case e.Op == ast.Concat:
		for _, o := range []*Const{x, y} {
			if o.T.Kind != Character {
				return nil, ev.report("BADOPERAND", e.Pos, "//", o.T.String())
			}
		}

		return charConst(x.Text+y.Text, e.Pos), nil

	case e.Op == ast.Pow:
		return ev.power(e, x, y)
	}

	t := operationType(x.T, y.T)

	if x, err = ev.arithmetic(x); err != nil {
		return nil, err
	}

	if y, err = ev.arithmetic(y); err != nil {
		return nil, err
	}

	if x, err = ev.convert(x, t, x.Pos); err != nil {
		return nil, err
	}

	if y, err = ev.convert(y, t, y.Pos); err != nil {
		return nil, err
	}

	return ev.operate(e.Op, x, y, e.Pos)
}

// operate is x op y for the four arithmetic operators, x and y of one
// type.
func (ev *evaluator) operate(op ast.Op, x, y *Const, pos diag.Position) (*Const, *constErr) {
	t := x.T
	c := &Const{exprBase: exprBase{t, pos}}

	switch t.Kind {
	case Integer:
		a, b := int64(x.Int), int64(y.Int)

		var v int64

		switch op {
		case ast.Add:
			v = a + b
		case ast.Sub:
			v = a - b
		case ast.Mul:
			v = a * b
		case ast.Div:
			if b == 0 {
				return nil, ev.report("DIVZERO", pos)
			}

			// Go's division truncates toward zero, as FORTRAN's does.
			v = a / b
		}

		if lo, hi := t.intRange(); v < lo || v > hi {
			return nil, ev.intRange(pos, valueText, t)
		}

		c.Int = int32(v)

	case Real:
		v, err := realArith(op, ev.a.format(t), x.Real, y.Real)
		if err != nil {
			return nil, ev.floatError(err, pos, t)
		}

		c.Real = v

	case Complex:
		re, im, err := complexArith(op, ev.a.format(t), x.Real, x.Imag, y.Real, y.Imag)
		if err != nil {
			return nil, ev.floatError(err, pos, t)
		}

		c.Real, c.Imag = re, im
	}

	return c, nil
}

// floatError reports a floating operation's error: a division by zero,
// or an overflow.
func (ev *evaluator) floatError(err error, pos diag.Position, t Type) *constErr {
	if errors.Is(err, vaxfloat.ErrDivideByZero) {
		return ev.report("DIVZERO", pos)
	}

	return ev.realRange(pos, valueText, t)
}

// round is v rounded to format f. A value too small for f is zero, as the
// VAX's is with floating underflow faults disabled (the CPU's default).
func round(f vaxfloat.Format, v vaxfloat.Value) (vaxfloat.Value, error) {
	r, err := vaxfloat.Round(f, v)
	if errors.Is(err, vaxfloat.ErrUnderflow) {
		return vaxfloat.Value{}, nil
	}

	return r, err
}

// realArith is x op y in format f, rounded once, as the VAX's floating
// instructions do.
func realArith(op ast.Op, f vaxfloat.Format, x, y vaxfloat.Value) (vaxfloat.Value, error) {
	var (
		v   vaxfloat.Value
		err error
	)

	switch op {
	case ast.Add:
		v, err = vaxfloat.Add(f, x, y)
	case ast.Sub:
		v, err = vaxfloat.Sub(f, x, y)
	case ast.Mul:
		v, err = vaxfloat.Mul(f, x, y)
	case ast.Div:
		v, err = vaxfloat.Div(f, x, y)
	}

	if errors.Is(err, vaxfloat.ErrUnderflow) {
		return vaxfloat.Value{}, nil
	}

	return v, err
}

// complexArith is (a, b) op (c, d) in format f. A product or quotient is
// vaxfloat's, the same code the OTS$ routines use, so a constant has the
// value the same expression computes at run time; Phase 78's code for a
// product it writes in line must round as it does.
func complexArith(op ast.Op, f vaxfloat.Format, a, b, c, d vaxfloat.Value) (vaxfloat.Value, vaxfloat.Value, error) {
	switch op {
	case ast.Mul:
		return vaxfloat.ComplexMul(f, a, b, c, d)
	case ast.Div:
		return vaxfloat.ComplexDiv(f, a, b, c, d)
	}

	r, err := realArith(op, f, a, c)
	if err != nil {
		return r, r, err
	}

	i, err := realArith(op, f, b, d)

	return r, i, err
}

// power is x ** y, which is a constant expression only for an INTEGER
// exponent (section 4.11). An INTEGER base gives an INTEGER of the higher
// type; a REAL or COMPLEX base its own type, by vaxfloat.PowerBySquaring,
// which is the OTS$ routines' own order: the product of the base's squares,
// each rounded to its format, or for a negative exponent of their reciprocals.
// An INTEGER power's rules are mthrtl's too. Zero to a power of zero or less
// has no value (UNDEFPOW).
func (ev *evaluator) power(e *ast.Binary, x, y *Const) (*Const, *constErr) {
	x, err := ev.arithmetic(x)
	if err != nil {
		return nil, err
	}

	if y, err = ev.arithmetic(y); err != nil {
		return nil, err
	}

	if y.T.Kind != Integer {
		return nil, notConst("its ** operator has "+article(y.T.String())+" exponent, where only an INTEGER one is computed when compiling", y.Pos)
	}

	n := int64(y.Int)

	if x.T.Kind == Integer {
		t := higher(x.T, y.T)
		b := int64(x.Int)
		c := &Const{exprBase: exprBase{t, e.Pos}}

		var v int64

		switch {
		case b == 0 && n <= 0:
			return nil, ev.report("UNDEFPOW", e.Pos, n)
		case b == 1, n == 0:
			v = 1
		case b == -1:
			v = 1 - 2*(n&1)
		case b == 0, n < 0:
			// 1 / b**|n| truncates to 0 for any other b.
			v = 0
		default:
			// |b| is at least 2, so the range is passed within 32 steps.
			v = 1
			lo, hi := t.intRange()

			for range n {
				v *= b
				if v < lo || v > hi {
					return nil, ev.intRange(e.Pos, valueText, t)
				}
			}
		}

		c.Int = int32(v)

		return c, nil
	}

	t := x.T
	f := ev.a.format(t)
	zero := x.Real.IsZero() && x.Imag.IsZero()

	if zero && n <= 0 {
		return nil, ev.report("UNDEFPOW", e.Pos, n)
	}

	arith := func(op ast.Op) func(p, q *Const) (*Const, error) {
		return func(p, q *Const) (*Const, error) {
			r := &Const{exprBase: exprBase{t, e.Pos}}

			var err error

			if t.Kind == Real {
				r.Real, err = realArith(op, f, p.Real, q.Real)
			} else {
				r.Real, r.Imag, err = complexArith(op, f, p.Real, p.Imag, q.Real, q.Imag)
			}

			return r, err
		}
	}

	one := &Const{exprBase: exprBase{t, e.Pos}, Real: vaxfloat.FromInt(1)}

	result, perr := vaxfloat.PowerBySquaring(x, one, n, arith(ast.Mul), arith(ast.Div))
	if perr != nil {
		return nil, ev.floatError(perr, e.Pos, t)
	}

	return result, nil
}

// compare is X op Y for a relational operator (section 2.3.3): numbers
// converted to the higher type, COMPLEX values only equal or not, and
// CHARACTER values compared in ASCII, the shorter padded with blanks.
func (ev *evaluator) compare(e *ast.Binary, x, y *Const) (*Const, *constErr) {
	if x.T.Kind == Character || y.T.Kind == Character {
		if x.T.Kind != y.T.Kind {
			return nil, ev.notImpl(e.Pos, hollerith)
		}

		return logicalConst(relation(e.Op, compareText(x.Text, y.Text)), e.Pos), nil
	}

	t := operationType(x.T, y.T)

	x, err := ev.arithmetic(x)
	if err != nil {
		return nil, err
	}

	if y, err = ev.arithmetic(y); err != nil {
		return nil, err
	}

	if t.Kind == Complex && e.Op != ast.EQ && e.Op != ast.NE {
		return nil, ev.report("BADOPERAND", e.Pos, e.Op.String(), t.String())
	}

	if x, err = ev.convert(x, t, x.Pos); err != nil {
		return nil, err
	}

	if y, err = ev.convert(y, t, y.Pos); err != nil {
		return nil, err
	}

	return logicalConst(relation(e.Op, order(x, y)), e.Pos), nil
}

// order is -1, 0, or 1 as x is less than, equal to, or greater than y,
// both of one numeric type; COMPLEX values are only equal (0) or not (1).
func order(x, y *Const) int {
	switch x.T.Kind {
	case Real:
		return x.Real.Cmp(y.Real)
	case Complex:
		if x.Real.Cmp(y.Real) == 0 && x.Imag.Cmp(y.Imag) == 0 {
			return 0
		}

		return 1
	}

	switch {
	case x.Int < y.Int:
		return -1
	case x.Int > y.Int:
		return 1
	}

	return 0
}

// compareText compares two character values as FORTRAN does: in ASCII
// (the VAX's collating sequence), the shorter as if padded with blanks.
func compareText(x, y string) int {
	n := max(len(x), len(y))
	x += strings.Repeat(" ", n-len(x))
	y += strings.Repeat(" ", n-len(y))

	return strings.Compare(x, y)
}

// relation is whether op holds between values whose order is order.
func relation(op ast.Op, order int) bool {
	switch op {
	case ast.EQ:
		return order == 0
	case ast.NE:
		return order != 0
	case ast.LT:
		return order < 0
	case ast.LE:
		return order <= 0
	case ast.GT:
		return order > 0
	}

	return order >= 0
}

// logical is X op Y for .AND., .OR., .EQV., .NEQV., and .XOR. (section
// 2.3.4): on LOGICAL operands a LOGICAL; with an INTEGER operand, bit by
// bit, an INTEGER.
func (ev *evaluator) logical(e *ast.Binary, x, y *Const) (*Const, *constErr) {
	for _, o := range []*Const{x, y} {
		switch o.T.Kind {
		case Character:
			return nil, ev.notImpl(o.Pos, hollerith)
		case Real, Complex:
			return nil, ev.report("BADOPERAND", e.Pos, e.Op.String(), o.T.String())
		}
	}

	t := logicalType(x.T, y.T)

	x, err := ev.convert(x, t, x.Pos)
	if err != nil {
		return nil, err
	}

	if y, err = ev.convert(y, t, y.Pos); err != nil {
		return nil, err
	}

	return bitwiseConst(e.Op, x, y, e.Pos), nil
}

// bitwiseConst is x op y for the logical operators, bit by bit, x and y
// being of one type.
func bitwiseConst(op ast.Op, x, y *Const, pos diag.Position) *Const {
	c := &Const{exprBase: exprBase{x.T, pos}}

	switch op {
	case ast.And:
		c.Int = x.Int & y.Int
	case ast.Or:
		c.Int = x.Int | y.Int
	case ast.Eqv:
		c.Int = ^(x.Int ^ y.Int)
	default:
		// .NEQV. and .XOR. are one operator (section 2.3.4).
		c.Int = x.Int ^ y.Int
	}

	return c
}

// convert is c converted to type t, by assignment's rules (Table 3-1):
// REAL to INTEGER truncates, a number to COMPLEX gets an imaginary part
// of zero and COMPLEX to anything else loses it, INTEGER and LOGICAL are
// the same bits, and a CHARACTER value is padded with blanks or cut on
// the right (CHARACTER*(*) takes the value's own length). A REAL or
// COMPLEX literal is rounded from its exact value. A conversion between
// types that have none is incompatible, for the caller to report.
func (ev *evaluator) convert(c *Const, t Type, pos diag.Position) (*Const, *constErr) {
	if c.T == t {
		return c, nil
	}

	r := &Const{exprBase: exprBase{t, pos}}

	switch t.Kind {
	case Integer:
		var v int64

		switch c.T.Kind {
		case Integer, Logical:
			v = int64(c.Int)
		case Real, Complex:
			i := c.Real.Int(false)
			if !i.IsInt64() {
				return nil, ev.intRange(pos, valueText, t)
			}

			v = i.Int64()
		case Character:
			return nil, ev.notImpl(pos, hollerith)
		default:
			return nil, failed
		}

		if lo, hi := t.intRange(); v < lo || v > hi {
			return nil, ev.intRange(pos, valueText, t)
		}

		r.Int = int32(v)

	case Logical:
		switch c.T.Kind {
		case Integer, Logical:
			r.Int = truncate(c.Int, t.Size)
		case Invalid:
			return nil, failed
		default:
			return nil, incompatible
		}

	case Real, Complex:
		var re, im vaxfloat.Value

		switch c.T.Kind {
		case Integer, Logical:
			re = vaxfloat.FromInt(int64(c.Int))
		case Real, Complex:
			re, im = c.Real, c.Imag
			if c.literal {
				re, im = c.exactRe, c.exactIm
			}
		case Character:
			return nil, ev.notImpl(pos, hollerith)
		default:
			return nil, failed
		}

		if t.Kind == Real {
			im = vaxfloat.Value{}
		}

		var err, err2 error

		f := ev.a.format(t)
		r.Real, err = round(f, re)
		r.Imag, err2 = round(f, im)

		if err != nil || err2 != nil {
			return nil, ev.realRange(pos, valueText, t)
		}

		if c.literal {
			r.literal, r.exactRe, r.exactIm = true, re, im
		}

	case Character:
		switch c.T.Kind {
		case Character:
		case Invalid:
			return nil, failed
		default:
			return nil, incompatible
		}

		text := c.Text

		switch n := t.Size; {
		case n == 0:
			r.T = c.T
		case len(text) < n:
			text += strings.Repeat(" ", n-len(text))
		default:
			text = text[:n]
		}

		r.Text = text

	default:
		return nil, failed
	}

	return r, nil
}
