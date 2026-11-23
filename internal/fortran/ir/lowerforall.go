package ir

import (
	"strconv"

	"github.com/tucats/govax/internal/fortran/sema"
)

// The FORALL statement (Fortran 95's 7.5.4; sema/forall.go). Its indexes'
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

// forall lowers a FORALL statement.
func (l *lowerer) forall(s *sema.Forall) {
	n := l.newNest(0, nil)
	defer n.done()

	name := "FORALL (line " + strconv.Itoa(l.here().Line) + ")"
	pos := l.here()

	// The bounds and strides, first, in the order written.
	type indexBounds struct{ lower, upper, stride sema.Expr }

	bounds := make([]indexBounds, len(s.Indexes))

	for i, x := range s.Indexes {
		b := &bounds[i]
		b.lower = n.keep(x.Lower)
		b.upper = n.keep(x.Upper)
		b.stride = sema.IntConstAt(1, pos)

		if x.Stride != nil {
			b.stride = n.keep(x.Stride)
		}
	}

	// Each index's place, an index cell of the statement's.
	for _, x := range s.Indexes {
		l.bound[x.Sym] = &Mem{Cell: l.indexCell(), T: Long}
		n.kept = append(n.kept, x.Sym)
	}

	loopName := func(k int) string { return name + ", index " + s.Indexes[k].Sym.Name }

	if !s.Temporary {
		var loops func(k int)

		loops = func(k int) {
			if k < 0 {
				l.forallPass(func() {
					l.forallMasked(s.Mask, func() { l.assign(s.Body) })
				})

				return
			}

			b := bounds[k]
			l.countedLoop(loopName(k), s.Indexes[k].Sym, b.lower, b.upper, b.stride, false, func(*Block) {
				loops(k - 1)
			})
		}

		loops(len(s.Indexes) - 1)

		return
	}

	// The index set's shape: each index's number of values, never below 0.
	extents := make([]Expr, len(s.Indexes))
	counts := make([]sema.Expr, len(s.Indexes))
	sizes := make([]int, len(s.Indexes))
	known := true

	for k, b := range bounds {
		count := n.atLeastZero(tripCount(Long, l.expr(b.lower), l.expr(b.upper), l.expr(b.stride)))
		extents[k] = count
		counts[k] = n.keepIR(count)

		if c, ok := count.(*Const); ok {
			sizes[k] = int(c.Int())
		} else {
			known = false
		}
	}

	temp := func(t sema.Type) *sema.ArrayRef {
		if known {
			return l.arrayTemp(t, sizes)
		}

		return l.runtimeTemp(t, extents, nil)
	}

	target := s.Body.Target
	values := temp(target.Type())

	var masks *sema.ArrayRef
	if s.Mask != nil {
		masks = temp(s.Mask.Type())
	}

	// Each loop's count, and the element of a temporary the counts name.
	counters := make([]*sema.Symbol, len(s.Indexes))
	subs := make([]sema.Expr, len(s.Indexes))
	inBounds := make([]bool, len(s.Indexes))

	for k := range s.Indexes {
		counters[k] = sema.NewSymbol("$I", sema.IntegerType)
		l.bound[counters[k]] = &Mem{Cell: l.indexCell(), T: Long}
		n.kept = append(n.kept, counters[k])
		subs[k] = sema.NewVar(counters[k], pos)
		inBounds[k] = true
	}

	element := func(of *sema.ArrayRef) sema.Expr { return sema.NewElem(of.Sym, subs, inBounds, pos) }

	// nest lowers body for every tuple, the indexes computed from the counts.
	nest := func(body func()) {
		var loops func(k int)

		loops = func(k int) {
			if k < 0 {
				l.forallPass(body)

				return
			}

			one := sema.IntConstAt(1, pos)
			l.countedLoop(loopName(k), counters[k], one, counts[k], one, false, func(*Block) {
				b := bounds[k]
				index := sema.IntSum(b.lower, sema.IntProduct(sema.IntDifference(sema.NewVar(counters[k], pos), one), b.stride))
				l.emit(&Assign{origin: l.origin(), Dst: l.symbolMem(s.Indexes[k].Sym), Src: l.expr(index)})
				loops(k - 1)
			})
		}

		loops(len(s.Indexes) - 1)
	}

	// The masks and values first, for every tuple.
	nest(func() {
		if masks == nil {
			l.forallStore(element(values), s.Body.Value)

			return
		}

		m := element(masks)
		l.assignScalar(m, s.Mask)
		l.forallMasked(m, func() { l.forallStore(element(values), s.Body.Value) })
	})

	// Then the stores.
	nest(func() {
		var m sema.Expr
		if masks != nil {
			m = element(masks)
		}

		l.forallMasked(m, func() { l.forallStore(target, element(values)) })
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
