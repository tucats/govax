package ir

import (
	"math"

	"github.com/tucats/govax/internal/fortran/sema"
)

// /CHECK=BOUNDS (docs/planning/PHASE-85 - FORTRAN compiler usability.md, Decisions 1 to 5).
//
// A subscript or substring bound that needs checking is wrapped in an InRange where it
// is lowered, so that the rest of an element's address is what it always was and the
// expression's registers work as they do for any other operation. It stays inside the
// expression that uses it, rather than going ahead of the statement as a function's
// value does, because of programs like
//
//	IF (I .GE. 1 .AND. A(I) .GT. 0.0) ...
//
// which a branch's .AND. keeps from reaching A(I) when I is 0. The check must only run
// when the subscript would have.

// inRange is x checked to be from low to high: x itself when all three are constants
// and it is. A constant out of range keeps its check, to trap if the statement runs.
func inRange(x, low, high Expr) Expr {
	cx, okx := x.(*Const)
	cl, okl := low.(*Const)
	ch, okh := high.(*Const)

	if okx && okl && okh && cx.Int() >= cl.Int() && cx.Int() <= ch.Int() {
		return x
	}

	return &InRange{x, low, high}
}

// checkedSubscript is a subscript's value, split as split does into the part that isn't
// constant and a constant added to it, checked against dimension i of dims if the unit
// asks. Both bounds are moved by the constant, so that TABLE(I+1) checks I and keeps its
// address's form, TABLE+0[R0]. A constant in range needs nothing, and one out of range
// becomes a part that isn't constant, the check being what does it.
func (l *lowerer) checkedSubscript(sub Expr, s *sema.Symbol, dims []sema.Dim, i int) (Expr, int32) {
	v, k := split(sub)
	if !l.unit.Checks.Bounds {
		return v, k
	}

	low, high := int64(dims[i].Lower), int64(dims[i].Upper)
	open := sema.OpenEnded(s, dims, i)

	if v == nil {
		if int64(k) >= low && (open || int64(k) <= high) {
			return nil, k
		}

		return &InRange{LongConst(k), clamped(low), upperOf(open, high)}, 0
	}

	// A dimension with no upper bound stays that way, however the constant moves it.
	if !open {
		high -= int64(k)
	}

	return &InRange{v, clamped(low - int64(k)), upperOf(open, high)}, k
}

// upperOf is a check's upper bound: high, or the most a Long can be if there is none.
func upperOf(open bool, high int64) *Const {
	if open {
		return LongConst(math.MaxInt32)
	}

	return clamped(high)
}

// clamped is n as a Long constant, or the nearest one. A bound moved past what a Long
// can hold by a subscript's constant still says the same of every Long.
func clamped(n int64) *Const {
	return LongConst(int32(min(max(n, math.MinInt32), math.MaxInt32)))
}

// checkedSubstring is what a substring S(first:last) of a value length long computes
// its place and its length's end from under /CHECK=BOUNDS: the manual's
//
//	1 <= first <= last <= length
//
// is two INDEX instructions, last from 1 to length and first from 1 to last, each
// failing with the subscript range trap, since FOR has no message for a substring
// (Decision 3). hasLast says the program gave last. The cases that need less are
// common, and worth finding: S(I:) only has first to check; a constant first of k only
// needs last from k to length; and S(I:I), one character, only needs I from 1 to
// length, and keeps a constant length, as the optimizer's one-character pass wants.
func checkedSubstring(first, last, length Expr, hasLast bool) (place, end Expr) {
	one := LongConst(1)

	if !hasLast || ExprText(first) == ExprText(last) {
		return inRange(first, one, length), last
	}

	if c, ok := first.(*Const); ok && c.Int() >= 1 {
		return first, inRange(last, c, length)
	}

	return inRange(first, one, last), inRange(last, one, length)
}

// checkArrayBounds writes what an entry point does for the adjustable array s, whose
// descriptor c its bounds are now in: an upper bound below its lower is FOR$_ADJARRDIM
// at once, by FTNRTL (Decision 5, ours, the manuals not giving the rule). An assumed
// dimension has no upper bound to compare.
func (l *lowerer) checkArrayBounds(s *sema.Symbol, c *Cell) {
	for i, dim := range s.Dims {
		if sema.OpenEnded(s, s.Dims, i) || (dim.LowerExpr == nil && dim.UpperExpr == nil) {
			continue
		}

		bad, next := l.newBlock(), l.newBlock()
		l.branch(&Cmp{Lt, l.upperBound(s, c, i), l.lowerBound(s, c, i)}, bad, next)

		l.begin(bad)
		l.emit(&Call{origin: l.origin(), Routine: ftnAdjArrDim})
		l.terminate(&NoReturn{l.origin()})

		l.begin(next)
	}
}
