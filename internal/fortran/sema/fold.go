package sema

import (
	"errors"
	"math"
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
	"github.com/tucats/govax/internal/vaxfloat"
)

// Constants, and constant folding: an operation whose operands are all
// constants is done here, at compile time, and replaced by its value.
// REAL arithmetic goes through vaxfloat, which rounds as the VAX's
// floating instructions do (Decision 16 of docs/PHASE-74 - FORTRAN
// plan.md), so a folded expression has the value the program would have
// computed.

// INTEGER*4's range (section 2.2.1.1).
const (
	minInt = math.MinInt32
	maxInt = math.MaxInt32
)

// intConst is the constant an integer literal writes.
func (a *analyzer) intConst(e *ast.IntConst) Expr {
	v, err := strconv.ParseInt(e.Text, 10, 64)
	if err != nil || v > maxInt {
		a.report("INTRANGE", e.Pos, e.Text)

		return a.bad(e.Pos)
	}

	return &Const{exprBase: exprBase{IntegerType, e.Pos}, Int: int32(v)}
}

// realConst is the constant a real literal writes, rounded to
// F_floating. A D or Q exponent makes a REAL*8 or REAL*16 constant,
// Phase 78's.
func (a *analyzer) realConst(e *ast.RealConst) Expr {
	switch {
	case strings.ContainsRune(e.Text, 'D'):
		a.notImpl(e.Pos, "a REAL*8 constant")

		return a.bad(e.Pos)

	case strings.ContainsRune(e.Text, 'Q'):
		a.notImpl(e.Pos, "a REAL*16 constant")

		return a.bad(e.Pos)
	}

	v, err := vaxfloat.Parse(e.Text)
	if err == nil {
		v, err = vaxfloat.Round(vaxfloat.F, v)
	}

	if err != nil {
		a.report("REALRANGE", e.Pos, e.Text)

		return a.bad(e.Pos)
	}

	return &Const{exprBase: exprBase{RealType, e.Pos}, Real: v}
}

// logicalConst is .TRUE. (-1) or .FALSE. (0).
func logicalConst(value bool, pos diag.Position) *Const {
	c := &Const{exprBase: exprBase{LogicalType, pos}}
	if value {
		c.Int = -1
	}

	return c
}

// intOp is x op y for INTEGER constants: the four arithmetic operators,
// .AND., and .OR. (bit by bit, section 2.3.4). An overflow or a division
// by zero is reported, and ok is false.
func (a *analyzer) intOp(op ast.Op, x, y int32, pos diag.Position) (int32, bool) {
	var v int64

	switch op {
	case ast.Add:
		v = int64(x) + int64(y)
	case ast.Sub:
		v = int64(x) - int64(y)
	case ast.Mul:
		v = int64(x) * int64(y)
	case ast.Div:
		if y == 0 {
			a.report("DIVZERO", pos)

			return 0, false
		}

		// Go's division truncates toward zero, as FORTRAN's does (1/4
		// is 0, -7/2 is -3).
		v = int64(x) / int64(y)
	case ast.And:
		return x & y, true
	case ast.Or:
		return x | y, true
	}

	r, kind := a.checkInt(v, pos)

	return r, kind == constOK
}

// realOp is x op y for REAL constants, rounded to F_floating. A result
// too small for F_floating is zero, as the VAX's is with floating
// underflow faults disabled (the CPU's default).
func (a *analyzer) realOp(op ast.Op, x, y vaxfloat.Value, pos diag.Position) (vaxfloat.Value, bool) {
	var (
		v   vaxfloat.Value
		err error
	)

	switch op {
	case ast.Add:
		v, err = vaxfloat.Add(vaxfloat.F, x, y)
	case ast.Sub:
		v, err = vaxfloat.Sub(vaxfloat.F, x, y)
	case ast.Mul:
		v, err = vaxfloat.Mul(vaxfloat.F, x, y)
	case ast.Div:
		v, err = vaxfloat.Div(vaxfloat.F, x, y)
	}

	switch {
	case errors.Is(err, vaxfloat.ErrDivideByZero):
		a.report("DIVZERO", pos)

		return v, false

	case errors.Is(err, vaxfloat.ErrOverflow):
		a.report("REALRANGE", pos, "the value of this expression")

		return v, false
	}

	return v, true
}

// fold replaces e, an operation, by its value if its operands are all
// constants; otherwise it returns e as it is.
func (a *analyzer) fold(e Expr) Expr {
	switch e := e.(type) {
	case *Convert:
		if x, ok := e.X.(*Const); ok {
			return a.foldConvert(x, e.T, e.Pos)
		}

	case *Unary:
		if x, ok := e.X.(*Const); ok {
			return a.foldUnary(e, x)
		}

	case *Binary:
		x, okx := e.X.(*Const)
		y, oky := e.Y.(*Const)

		if okx && oky {
			return a.foldBinary(e, x, y)
		}

	case *Compare:
		x, okx := e.X.(*Const)
		y, oky := e.Y.(*Const)

		if okx && oky {
			return foldCompare(e, x, y)
		}
	}

	return e
}

// foldConvert is the constant x converted to type t.
func (a *analyzer) foldConvert(x *Const, t Type, pos diag.Position) Expr {
	c := &Const{exprBase: exprBase{t, pos}}

	switch {
	case t.Kind == Real && x.T.Kind == Real:
		c.Real = x.Real

	case t.Kind == Real:
		// CVTLF: exact up to 2**24, rounded beyond.
		c.Real, _ = vaxfloat.Round(vaxfloat.F, vaxfloat.FromInt(int64(x.Int)))

	case x.T.Kind == Real:
		// CVTFL truncates (Table 3-1).
		i := x.Real.Int(false)
		if !i.IsInt64() || i.Int64() < minInt || i.Int64() > maxInt {
			a.report("INTRANGE", pos, "the value of this expression")

			return a.bad(pos)
		}

		c.Int = int32(i.Int64())

	default:
		// INTEGER and LOGICAL are the same longword.
		c.Int = x.Int
	}

	return c
}

// foldUnary is -x or .NOT. x for the constant x.
func (a *analyzer) foldUnary(e *Unary, x *Const) Expr {
	c := &Const{exprBase: e.exprBase}

	switch {
	case e.Op == ast.Not:
		c.Int = ^x.Int

	case e.T.Kind == Real:
		c.Real = x.Real.Neg()

	default:
		v, kind := a.checkInt(-int64(x.Int), e.Pos)
		if kind != constOK {
			return a.bad(e.Pos)
		}

		c.Int = v
	}

	return c
}

// foldBinary is x op y for the constants x and y.
func (a *analyzer) foldBinary(e *Binary, x, y *Const) Expr {
	c := &Const{exprBase: e.exprBase}

	var ok bool

	if e.T.Kind == Real {
		c.Real, ok = a.realOp(e.Op, x.Real, y.Real, e.Pos)
	} else {
		c.Int, ok = a.intOp(e.Op, x.Int, y.Int, e.Pos)
	}

	if !ok {
		return a.bad(e.Pos)
	}

	return c
}

// foldCompare is the LOGICAL value of x op y for the constants x and y.
func foldCompare(e *Compare, x, y *Const) Expr {
	var order int

	if x.T.Kind == Real {
		order = x.Real.Cmp(y.Real)
	} else {
		switch {
		case x.Int < y.Int:
			order = -1
		case x.Int > y.Int:
			order = 1
		}
	}

	var v bool

	switch e.Op {
	case ast.EQ:
		v = order == 0
	case ast.NE:
		v = order != 0
	case ast.LT:
		v = order < 0
	case ast.LE:
		v = order <= 0
	case ast.GT:
		v = order > 0
	case ast.GE:
		v = order >= 0
	}

	return logicalConst(v, e.Pos)
}
