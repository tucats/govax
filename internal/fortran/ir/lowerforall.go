package ir

import (
	"strconv"

	"github.com/tucats/govax/internal/fortran/sema"
)

// The FORALL statement and construct (Fortran 95's 7.5.4; sema/forall.go). Its indexes'
// bounds and strides are computed first, once, in the order written. Then, if
// sema found that no tuple's assignment can change what another reads, the
// statement is a nest of counted loops, one per index, the first index's
// innermost (so that A(I,J) is walked in array element order), each tuple's
// mask tested and its assignment done in turn:
//
//	FORALL (I = 1:N, J = 1:N, A(I,J) /= 0.0) B(I,J) = 1.0 / A(I,J)
//
// is
//
//	DO J = 1, N
//	  DO I = 1, N
//	    IF (A(I,J) /= 0.0) B(I,J) = 1.0 / A(I,J)
//	  END DO
//	END DO
//
// with I and J the statement's own index cells, not the unit's variables. An
// index runs by its stride, a DO loop's count deciding how often.
//
// Otherwise (Forall.Temporary) the statement is two nests over the index set,
// each loop counting from 1 to its index's number of values, the index computed
// from the count. The first computes, for every tuple, the mask into a LOGICAL
// temporary and, where it holds, the value into a temporary of the target's
// type, both of the index set's shape; the second stores each value kept into
// its target. So FORALL (I = 2:8) V(I) = V(I-1) is
//
//	DO $I1 = 1, 7
//	  I = 2 + ($I1 - 1)
//	  $W1($I1) = V(I-1)
//	END DO
//	DO $I1 = 1, 7
//	  I = 2 + ($I1 - 1)
//	  V(I) = $W1($I1)
//	END DO
//
// The temporaries are cells when every bound is a constant, else run-time
// temporaries (lowertemps.go), given back at the statement's end. What one
// tuple's assignment needs only while it is done (a dynamic string, a run-time
// temporary of an array assignment's) is given back at the end of its pass.

// A FORALL construct (subtask 9) is its statements in turn, each lowered as
// a FORALL statement is, over the indexes of every FORALL around it (its
// levels, forallLevel), each level's loops inside the one around it and its
// mask tested after its own loops, so a nested FORALL's bounds and mask are
// computed only for the outer tuples whose masks hold:
//
//	FORALL (I = 1:N)
//	  FORALL (J = 1:N, J > I) A(I,J) = 10.0*I + J
//	  WHERE (A(I,:) == 0.0) A(I,:) = -1.0
//	END FORALL
//
// is
//
//	DO I = 1, N
//	  DO J = 1, N
//	    IF (J > I) A(I,J) = 10.0*I + J
//	  END DO
//	END DO
//	DO I = 1, N
//	  WHERE (A(I,:) == 0.0) A(I,:) = -1.0
//	END DO
//
// A level's bounds are kept as the FORALL is reached, unless a bound of it or
// of one around it uses an outer index (sema.ForallVaries), when its loops
// compute them. A level whose mask its statements may change (Forall.MaskTemp)
// has the mask computed as it is reached, for the whole index set through it,
// into a LOGICAL temporary its statements test; the loops through that level
// then count from 1, as the temporaries' do, the indexes computed from the
// counts.

// forallLevel is one FORALL among the levels a statement of a FORALL is done
// over: its bounds and strides as the loops use them, each index's number of
// values (counts, extents, sizes; made when first needed), the counters of loops
// that count from 1, and its mask's temporary.
type forallLevel struct {
	f                    *sema.Forall
	lower, upper, stride []sema.Expr
	varies               bool

	counts   []sema.Expr
	extents  []Expr
	sizes    []int
	known    bool
	counters []*sema.Symbol
	maskTemp *sema.ArrayRef
}

// forall lowers a FORALL statement or construct.
func (l *lowerer) forall(s *sema.Forall) {
	n := l.newNest(0, nil)
	defer n.done()

	l.forallLevels(n, nil, s)

	if s.EndForall != nil {
		l.from = s.EndForall

		if label := s.EndForall.Base().Label; label != 0 {
			l.startLabel(label)
		}
	}
}

// forallLevels lowers the FORALL s, inside the levels outer: its bounds, its
// mask's temporary, then each of its statements in turn.
func (l *lowerer) forallLevels(n *arrayNest, outer []*forallLevel, s *sema.Forall) {
	foralls := make([]*sema.Forall, 0, len(outer)+1)
	for _, lv := range outer {
		foralls = append(foralls, lv.f)
	}

	lv := l.forallLevel(n, s, sema.ForallVaries(append(foralls, s)))
	levels := append(outer[:len(outer):len(outer)], lv)

	if s.MaskTemp {
		l.forallMaskTemp(n, levels)
	}

	from := l.from

	for i, st := range s.Body {
		if inner, nested := st.(*sema.Forall); nested {
			l.from = inner.AST
			l.forallLevels(n, levels, inner)
			l.from = from

			continue
		}

		if s.Construct() {
			l.from = st.Source()
		}

		if s.Temporaries[i] {
			l.forallThroughTemps(n, levels, st.(*sema.Assign))
		} else {
			l.forallNest(n, levels, innermostMaskTemp(levels), true, func() { l.forallStatement(st) })
		}

		l.from = from
	}
}

// forallLevel is the FORALL s as a level: its bounds and strides kept now,
// first, in the order written, unless varies says its loops compute them;
// then each index's place, an index cell of the statement's.
func (l *lowerer) forallLevel(n *arrayNest, s *sema.Forall, varies bool) *forallLevel {
	pos := l.here()
	lv := &forallLevel{f: s, varies: varies}

	for _, x := range s.Indexes {
		lower, upper, stride := x.Lower, x.Upper, sema.Expr(sema.IntConstAt(1, pos))

		if !varies {
			lower, upper = n.keep(lower), n.keep(upper)
		}

		if x.Stride != nil {
			stride = x.Stride
			if !varies {
				stride = n.keep(stride)
			}
		}

		lv.lower = append(lv.lower, lower)
		lv.upper = append(lv.upper, upper)
		lv.stride = append(lv.stride, stride)
	}

	for _, x := range s.Indexes {
		l.bound[x.Sym] = &Mem{Cell: l.indexCell(), T: Long}
		n.kept = append(n.kept, x.Sym)
	}

	return lv
}

// count makes the level's index set's shape, once: each index's number of
// values, never below 0. Only a level whose bounds are kept has one.
func (lv *forallLevel) count(n *arrayNest) {
	if lv.counts != nil {
		return
	}

	if lv.varies {
		panic("ir: a FORALL whose bounds vary has no shape")
	}

	l := n.l
	lv.known = true

	for k := range lv.f.Indexes {
		count := n.atLeastZero(tripCount(Long, l.expr(lv.lower[k]), l.expr(lv.upper[k]), l.expr(lv.stride[k])))
		lv.extents = append(lv.extents, count)
		lv.counts = append(lv.counts, n.keepIR(count))

		size := 0
		if c, ok := count.(*Const); ok {
			size = int(c.Int())
		} else {
			lv.known = false
		}

		lv.sizes = append(lv.sizes, size)
	}
}

// counter is the counter of index k's loop, when it counts from 1, made at its
// first use.
func (lv *forallLevel) counter(n *arrayNest, k int) *sema.Symbol {
	if lv.counters == nil {
		lv.counters = make([]*sema.Symbol, len(lv.f.Indexes))
	}

	if lv.counters[k] == nil {
		sym := sema.NewSymbol("$I", sema.IntegerType)
		n.l.bound[sym] = &Mem{Cell: n.l.indexCell(), T: Long}
		n.kept = append(n.kept, sym)
		lv.counters[k] = sym
	}

	return lv.counters[k]
}

// innermostMaskTemp is the innermost of levels whose mask is in a temporary,
// or -1.
func innermostMaskTemp(levels []*forallLevel) int {
	for j := len(levels) - 1; j >= 0; j-- {
		if levels[j].maskTemp != nil {
			return j
		}
	}

	return -1
}

// forallTemp is a temporary of type t over the index set of levels, every
// level counted: cells if every count is a constant, else a run-time temporary,
// given back at the statement's end.
func (l *lowerer) forallTemp(n *arrayNest, levels []*forallLevel, t sema.Type) *sema.ArrayRef {
	var (
		sizes   []int
		extents []Expr
	)

	known := true

	for _, lv := range levels {
		lv.count(n)
		sizes = append(sizes, lv.sizes...)
		extents = append(extents, lv.extents...)
		known = known && lv.known
	}

	if known {
		return l.arrayTemp(t, sizes)
	}

	return l.runtimeTemp(t, extents, nil)
}

// forallElement is the element of the temporary of over levels' index set that
// the loops' counters name.
func (l *lowerer) forallElement(n *arrayNest, of *sema.ArrayRef, levels []*forallLevel) sema.Expr {
	pos := l.here()

	var (
		subs     []sema.Expr
		inBounds []bool
	)

	for _, lv := range levels {
		for k := range lv.f.Indexes {
			subs = append(subs, sema.NewVar(lv.counter(n, k), pos))
			inBounds = append(inBounds, true)
		}
	}

	return sema.NewElem(of.Sym, subs, inBounds, pos)
}

// forallNest lowers body for every tuple of the levels' indexes: a nest of
// counted loops, each level's inside the one around it, the first index's
// innermost within a level. The loops of the levels through counted count from
// 1, the index computed from the count; the others run by their index's own
// bounds and stride. With test, each level's mask is tested after its loops,
// the innermost temporary's element in place of the masks it holds.
func (l *lowerer) forallNest(n *arrayNest, levels []*forallLevel, counted int, test bool, body func()) {
	pos := l.here()
	one := sema.IntConstAt(1, pos)
	temp := innermostMaskTemp(levels)

	var level func(j int)

	level = func(j int) {
		if j == len(levels) {
			l.forallPass(body)

			return
		}

		lv := levels[j]
		name := "FORALL (line " + strconv.Itoa(lv.f.AST.Position().Line) + ")"

		var loops func(k int)

		loops = func(k int) {
			if k < 0 {
				var mask sema.Expr

				switch {
				case !test, j < temp:
				case j == temp:
					mask = l.forallElement(n, lv.maskTemp, levels[:j+1])
				default:
					mask = lv.f.Mask
				}

				l.forallMasked(mask, func() { level(j + 1) })

				return
			}

			x := lv.f.Indexes[k]
			loopName := name + ", index " + x.Sym.Name

			if j > counted {
				l.countedLoop(loopName, x.Sym, lv.lower[k], lv.upper[k], lv.stride[k], false, func(*Block) {
					loops(k - 1)
				})

				return
			}

			lv.count(n)
			c := lv.counter(n, k)

			l.countedLoop(loopName, c, one, lv.counts[k], one, false, func(*Block) {
				index := sema.IntSum(lv.lower[k], sema.IntProduct(sema.IntDifference(sema.NewVar(c, pos), one), lv.stride[k]))
				l.emit(&Assign{origin: l.origin(), Dst: l.symbolMem(x.Sym), Src: l.expr(index)})
				loops(k - 1)
			})
		}

		loops(len(lv.f.Indexes) - 1)
	}

	level(0)
}

// forallStatement lowers one tuple's statement of a FORALL done in turn.
func (l *lowerer) forallStatement(st sema.Stmt) {
	switch st := st.(type) {
	case *sema.Assign:
		l.assign(st)
	case *sema.Call:
		l.call(st)
	case *sema.PointerAssign:
		l.pointerAssign(st)
	case *sema.Nullify:
		l.nullify(st)
	case *sema.Where:
		l.where(st)
	default:
		panic("ir: a FORALL's statement of an unknown kind")
	}
}

// stepMasks are the masks a statement of the FORALL levels tests, outer first:
// the element of the innermost mask temporary, which holds the masks around
// it, and the masks inside it.
func (l *lowerer) stepMasks(n *arrayNest, levels []*forallLevel) []sema.Expr {
	var masks []sema.Expr

	temp := innermostMaskTemp(levels)
	if temp >= 0 {
		masks = append(masks, l.forallElement(n, levels[temp].maskTemp, levels[:temp+1]))
	}

	for _, lv := range levels[temp+1:] {
		if lv.f.Mask != nil {
			masks = append(masks, lv.f.Mask)
		}
	}

	return masks
}

// forallMaskInto lowers m, a LOGICAL scalar, given whether every one of masks
// holds, each tested only where those before it hold: a single mask assigned
// as it is.
func (l *lowerer) forallMaskInto(m sema.Expr, masks []sema.Expr) {
	if len(masks) == 1 {
		l.assignScalar(m, masks[0])

		return
	}

	pos := l.here()
	l.assignScalar(m, sema.LogicalConstAt(false, pos))

	var test func(i int)

	test = func(i int) {
		if i == len(masks) {
			l.assignScalar(m, sema.LogicalConstAt(true, pos))

			return
		}

		l.forallMasked(masks[i], func() { test(i + 1) })
	}

	test(0)
}

// forallMaskTemp computes the mask of the last of levels, with the masks of
// those around it, for their whole index set into a LOGICAL temporary.
func (l *lowerer) forallMaskTemp(n *arrayNest, levels []*forallLevel) {
	lv := levels[len(levels)-1]
	masks := l.stepMasks(n, levels)

	t := sema.LogicalType
	if len(masks) == 1 {
		t = masks[0].Type()
	}

	temp := l.forallTemp(n, levels, t)

	l.forallNest(n, levels, len(levels)-1, false, func() { l.forallMaskInto(l.forallElement(n, temp, levels), masks) })

	lv.maskTemp = temp
}

// forallThroughTemps lowers the assignment s, a statement of the FORALL levels
// whose values go through temporaries (Forall.Temporaries): its masks and
// values computed for every tuple first, then stored.
func (l *lowerer) forallThroughTemps(n *arrayNest, levels []*forallLevel, s *sema.Assign) {
	for _, lv := range levels {
		lv.count(n)
	}

	masks := l.stepMasks(n, levels)
	target := s.Target
	values := l.forallTemp(n, levels, target.Type())

	var maskTemp *sema.ArrayRef
	if len(masks) > 0 {
		t := sema.LogicalType
		if len(masks) == 1 {
			t = masks[0].Type()
		}

		maskTemp = l.forallTemp(n, levels, t)
	}

	all := len(levels) - 1

	// The masks and values first, for every tuple.
	l.forallNest(n, levels, all, false, func() {
		if maskTemp == nil {
			l.forallStore(l.forallElement(n, values, levels), s.Value)

			return
		}

		m := l.forallElement(n, maskTemp, levels)
		l.forallMaskInto(m, masks)
		l.forallMasked(m, func() { l.forallStore(l.forallElement(n, values, levels), s.Value) })
	})

	// Then the stores.
	l.forallNest(n, levels, all, false, func() {
		var m sema.Expr
		if maskTemp != nil {
			m = l.forallElement(n, maskTemp, levels)
		}

		l.forallMasked(m, func() { l.forallStore(target, l.forallElement(n, values, levels)) })
	})
}

// forallPass lowers one tuple's part of a FORALL with what it alone needs given
// back at its end: the dynamic strings its values used (lowerer.after) and its
// run-time temporaries, which the statement's own (the FORALL's temporaries,
// made before the loops) are kept apart from.
func (l *lowerer) forallPass(body func()) {
	runtime, after := l.runtime, l.after
	l.runtime, l.after = runtimeTemps{}, nil

	body()
	l.flush()

	if l.pendingTemps() && l.cur != nil {
		l.endTemps()
	}

	l.runtime, l.after = runtime, after
}

// forallMasked lowers body where the scalar LOGICAL mask holds, or always for a
// nil mask. A temporary the mask made is given back before the test, so that
// both ways leave it given back.
func (l *lowerer) forallMasked(mask sema.Expr, body func()) {
	if mask == nil {
		body()

		return
	}

	yes, next := l.newBlock(), l.newBlock()
	l.branch(l.stmtCond(mask), yes, next)
	l.begin(yes)

	runtime, after := l.runtime, l.after
	l.runtime, l.after = runtimeTemps{}, nil

	body()
	l.flush()

	if l.pendingTemps() && l.cur != nil {
		l.endTemps()
	}

	l.runtime, l.after = runtime, after

	l.jump(next)
	l.begin(next)
}

// forallStore lowers target = value for one tuple of a FORALL whose values go
// through temporaries, target a scalar of value's type (the value converted
// already to the target's, a CHARACTER one of any length).
func (l *lowerer) forallStore(target, value sema.Expr) {
	if target.Type().Kind == sema.Record {
		if v, ok := value.(*sema.StructValue); ok {
			l.buildRecord(l.mem(target), v)

			return
		}

		l.moveRecord(&sema.Assign{Target: target, Value: value})

		return
	}

	l.assignScalar(target, value)
}
