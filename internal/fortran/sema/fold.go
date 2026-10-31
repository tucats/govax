package sema

import (
	"math"
	"strconv"

	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
)

// Constant folding: an operation whose operands are all constants is done
// at compile time and replaced by its value. The values are the
// evaluator's (constant.go), one set of rules for an executable statement
// and a PARAMETER statement alike. REAL arithmetic goes through vaxfloat,
// which rounds as the VAX's floating instructions do (Decision 16 of
// docs/PHASE-74 - FORTRAN plan.md), so a folded expression has the value
// the program would have computed.

// INTEGER*4's range (section 2.2.1.1).
const (
	minInt = math.MinInt32
	maxInt = math.MaxInt32
)

// valueText is what a range message says is out of range when it isn't
// a constant as written.
const valueText = "the value of this expression"

// intRangeError reports what, of INTEGER type t, as out of t's range
// (INTRANGE).
func (a *analyzer) intRangeError(pos diag.Position, what string, t Type) {
	lo, hi := t.intRange()
	a.report("INTRANGE", pos, what, t.String(), lo, hi)
}

// realRangeError reports what, of REAL or COMPLEX type t, as out of the
// range of t's format (REALRANGE).
func (a *analyzer) realRangeError(pos diag.Position, what string, t Type) {
	a.report("REALRANGE", pos, what, t.part().String(), realRange(a.format(t)))
}

// constantType is the type of an integer constant whose value is v (never
// negative: a sign is an operator). It is the unit's default integer, unless
// v needs more than that has.
func (a *analyzer) constantType(v int64) Type {
	if v > math.MaxInt16 {
		return IntegerType
	}

	return a.defaultInt
}

// intValue is an integer constant's value: its digits in decimal, or in
// octal for appendix A.5's "17 (Radix 8), which is an INTEGER like any
// other.
func intValue(e *ast.IntConst) (int64, error) {
	radix := 10
	if e.Radix != 0 {
		radix = e.Radix
	}

	return strconv.ParseInt(e.Text, radix, 64)
}

// logicalConst is .TRUE. (-1) or .FALSE. (0).
func logicalConst(value bool, pos diag.Position) *Const {
	c := &Const{exprBase: exprBase{LogicalType, pos}}
	if value {
		c.Int = -1
	}

	return c
}

// fold replaces e, an operation, by its value if its operands are all
// constants; otherwise it returns e as it is.
func (a *analyzer) fold(e Expr) Expr {
	ev := &evaluator{a: a}

	var (
		c   *Const
		err *constErr
	)

	switch e := e.(type) {
	case *Convert:
		x, ok := e.X.(*Const)
		if !ok {
			return e
		}

		c, err = ev.assigned(x, e.T, e.Pos)

	case *Unary:
		x, ok := e.X.(*Const)
		if !ok {
			return e
		}

		if e.Op == ast.Not {
			c = &Const{exprBase: exprBase{e.T, e.Pos}, Int: ^x.Int}
		} else {
			c, err = ev.negate(x, e.Pos)
		}

	case *Binary:
		x, okx := e.X.(*Const)
		y, oky := e.Y.(*Const)

		switch {
		case !okx || !oky:
			return e
		case e.Op == ast.And, e.Op == ast.Or, e.Op == ast.Eqv, e.Op == ast.Neqv, e.Op == ast.Xor:
			c = bitwiseConst(e.Op, x, y, e.Pos)
		default:
			c, err = ev.operate(e.Op, x, y, e.Pos)
		}

	case *Compare:
		x, okx := e.X.(*Const)
		y, oky := e.Y.(*Const)

		if !okx || !oky {
			return e
		}

		return logicalConst(relation(e.Op, order(x, y)), e.Pos)

	default:
		return e
	}

	if err != nil {
		return a.bad(e.Position())
	}

	return c
}
