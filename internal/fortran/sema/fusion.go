package sema

import (
	"fmt"
	"strconv"
)

// Fusion (docs/planning/PHASE-110 - Fortran 90 compiler usability.md, subtask 8).
// Two array assignments one after the other are each a nest of loops of their own:
//
//	A = B + 1.0
//	C = A * 2.0
//
// is two loops, A stored by the first and loaded again by the second. One loop doing
// both, an element of A and then the same element of C, saves a loop's overhead, and is
// right when nothing either statement reads or writes at one element is written by the
// other at another: the arrays they share are the same element at each index (A here)
// or apart, and each statement's shape is the same. Then no element is read before or
// after a write the separate loops would have put on the other side of it.
//
// What the test asks of each statement, to keep it short and sure: its loops run up with
// no temporary, fill, or copied vector subscript (ArrayOrder); it calls no function and
// builds no constructor, which the scalarizer computes before the loops; it has no vector
// subscript and no record component; and it isn't CHARACTER or a record, whose elements
// are moved by their bytes. Of the pairs, a later statement's arrays against an earlier
// one's target, and an earlier one's against the later one's target, must be the same
// element or apart (relate); and a scalar either computes, or a bound or subscript of its
// sections, mustn't read the other's target at all, since the scalarizer computes those
// before the loops or at each element. The shapes must be the same when compiling: each
// extent the same constant, or the same array's, or bounds of the same text, computed
// when the routine was entered (an adjustable or automatic array's).
//
// Lowering asks, under /OPTIMIZE, for each array assignment whether it joins the ones
// just before it (ir's fused nests); a WHERE construct's assignments are asked the same,
// with the mask among each one's arrays.

// Fusable reports whether the array assignment next may share one nest of loops with
// the assignments group, the statements just before it, which already share one.
func Fusable(group []*Assign, next *Assign) bool {
	if len(group) == 0 || !plainForFusion(next) {
		return false
	}

	for _, s := range group {
		if !plainForFusion(s) || !sameShape(s.Target.Shape(), next.Target.Shape()) {
			return false
		}

		if !apartOrSame(s, next) || !apartOrSame(next, s) {
			return false
		}
	}

	return true
}

// FusableWhere reports whether a WHERE construct's assignments may all share one nest
// of loops, testing the mask once at each element: each a plain assignment, each pair as
// Fusable asks, and the mask, which no assignment of the construct may change
// (MaskTemp is nil), apart from or the same element as every target.
func FusableWhere(w *Where) bool {
	if w.MaskTemp != nil || !plainMask(w.Mask) {
		return false
	}

	var all []*Assign

	for _, arm := range []*Arm{w.Then, w.Else} {
		if arm == nil {
			continue
		}

		for _, st := range arm.Body {
			a, ok := st.(*Assign)
			if !ok || a.Source().Base().Label != 0 {
				return false
			}

			if len(all) > 0 && !Fusable(all, a) || len(all) == 0 && !plainForFusion(a) {
				return false
			}

			if !sameShape(w.Mask.Shape(), a.Target.Shape()) || !sharesOnlySame(a.Target, w.Mask) {
				return false
			}

			all = append(all, a)
		}
	}

	return len(all) > 1
}

// plainForFusion reports whether an array assignment is of the kind fusion takes.
func plainForFusion(s *Assign) bool {
	o := s.Order
	if o == nil || o.Temporary || o.Fill || o.CopyVectors {
		return false
	}

	for _, down := range o.Down {
		if down {
			return false
		}
	}

	switch s.Target.Type().Kind {
	case Character, Record:
		return false
	}

	return plainArray(s.Target) && plainMask(s.Value)
}

// plainMask reports whether an array expression's leaves are all plain arrays, and its
// scalars call nothing.
func plainMask(e Expr) bool {
	plain := true

	Scalarize(e, func(leaf Expr) Expr {
		plain = plain && plainArray(leaf)

		return leaf
	}, func(x Expr) Expr {
		plain = plain && !callsFunction(x)

		return x
	})

	return plain
}

// plainArray reports whether a leaf is a whole array or a section with no vector
// subscript, or an array constant.
func plainArray(leaf Expr) bool {
	switch leaf := leaf.(type) {
	case *ArrayRef, *ArrayConst:
		return true
	case *Section:
		for _, sub := range leaf.Subs {
			if sub.Vector != nil || callsFunction(sub.Index) || callsFunction(sub.Lower) ||
				callsFunction(sub.Upper) || callsFunction(sub.Stride) {
				return false
			}
		}

		return true
	}

	return false
}

// callsFunction reports whether the scalar e references a function or statement
// function, which may do anything.
func callsFunction(e Expr) bool {
	found := false

	var walk func(Expr)

	walk = func(e Expr) {
		switch e := e.(type) {
		case nil:
		case *FuncRef, *StmtFuncRef, *ArrayFunc:
			found = true
		case *Elem:
			for _, s := range e.Subs {
				walk(s)
			}
		case *Unary:
			walk(e.X)
		case *Binary:
			walk(e.X)
			walk(e.Y)
		case *Compare:
			walk(e.X)
			walk(e.Y)
		case *Convert:
			walk(e.X)
		case *IntrinsicRef:
			for _, x := range e.Args {
				walk(x)
			}
		case *Const, *Var:
		default:
			found = true
		}
	}

	walk(e)

	return found
}

// apartOrSame reports whether nothing the assignment later reads or writes is changed
// at another element by earlier's target, nor anything earlier read: each array they
// share is the same element at each index, or apart, and no scalar of either (an operand,
// a bound, a subscript) reads the other's target.
func apartOrSame(earlier, later *Assign) bool {
	return sharesOnlySame(earlier.Target, later.Value) && sharesOnlySame(earlier.Target, later.Target) &&
		sharesOnlySame(later.Target, earlier.Value)
}

// sharesOnlySame reports whether every array of the array expression e that may share
// target's storage is target's same element at each index, or apart from it, and no
// scalar of e, nor any subscript or bound of its sections, reads target's storage.
func sharesOnlySame(target, e Expr) bool {
	ts := baseSymbol(target)
	ok := true

	Scalarize(e, func(leaf Expr) Expr {
		if section, isSection := leaf.(*Section); isSection {
			for _, sub := range section.Subs {
				for _, x := range []Expr{sub.Index, sub.Lower, sub.Upper, sub.Stride} {
					if x != nil && readsAny(x, ts) {
						ok = false
					}
				}
			}
		}

		if !mayShare(ts, baseSymbol(leaf)) {
			return leaf
		}

		if rel, _, _ := relate(target, leaf); rel != relApart && rel != relSame {
			ok = false
		}

		return leaf
	}, func(x Expr) Expr {
		if readsAny(x, ts) {
			ok = false
		}

		return x
	})

	return ok
}

// sameShape reports whether two shapes are surely the same when the statements run.
func sameShape(x, y Shape) bool {
	if len(x) != len(y) {
		return false
	}

	for d := range x {
		if !sameExtent(x[d], y[d]) {
			return false
		}
	}

	return true
}

// sameExtent reports whether two extents are surely equal: the same constant, or the
// same text by extentKey.
func sameExtent(x, y Extent) bool {
	if x.Known || y.Known {
		return x.Known && y.Known && x.Size == y.Size
	}

	kx, ky := extentKey(x), extentKey(y)

	return kx != "" && kx == ky
}

// extentKey is a text two extents not known when compiling share only if they are
// equal: a whole array's dimension, the array itself if its bounds may change while the
// routine runs (an allocatable array, a pointer, an assumed-shape dummy, a temporary),
// or else the text of its bounds, which the routine computed once, when it was entered;
// a section's triplet, the text of its bounds and stride, a bound left out the array's.
// "" for any other, which is never taken as equal.
func extentKey(e Extent) string {
	switch of := e.Of.(type) {
	case *ArrayRef:
		return dimKey(of.Sym, e.Dim)

	case *Section:
		ranging := -1

		for i, sub := range of.Subs {
			if !sub.Ranges() {
				continue
			}

			if ranging++; ranging != e.Dim {
				continue
			}

			if !sub.Triplet() {
				return ""
			}

			lower, upper := exprKeyOr(sub.Lower), exprKeyOr(sub.Upper)
			if sub.Lower == nil || sub.Upper == nil {
				own := dimKey(of.Sym, i)
				if own == "" {
					return ""
				}

				lower, upper = lower+"@"+own, upper+"@"+own
			}

			return "triplet:" + lower + ":" + upper + ":" + exprKeyOr(sub.Stride)
		}
	}

	return ""
}

// dimKey is extentKey's text for dimension d of the whole array s.
func dimKey(s *Symbol, d int) string {
	if d >= len(s.Dims) {
		return ""
	}

	dim := s.Dims[d]

	switch {
	case dim.Assumed:
		return ""
	case s.Allocatable || s.Pointer || dim.Deferred || dim.AssumedShape || dim.Runtime:
		return fmt.Sprintf("array:%p:%d", s, d)
	}

	return "bounds:" + boundKey(dim.Lower, dim.LowerValue) + ":" + boundKey(dim.Upper, dim.UpperValue)
}

// boundKey is a bound's text: its constant, or its expression's.
func boundKey(n int, e Expr) string {
	if e == nil {
		return strconv.Itoa(n)
	}

	return DumpExpr(e)
}

// exprKeyOr is e's text, or "-" for none.
func exprKeyOr(e Expr) string {
	if e == nil {
		return "-"
	}

	return DumpExpr(e)
}
