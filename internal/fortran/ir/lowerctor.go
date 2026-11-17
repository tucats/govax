package ir

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/fortran/sema"
)

// Array constants and constructors (docs/planning/PHASE-102 - Fortran 90 array
// expressions.md, finding 10).
//
// An array constant, a constructor of constants or an array PARAMETER, is
// read-only data in $PDATA, an ArrayDatum of its elements (a CHARACTER one a
// text of them all), written only if a statement reads it: an element with a
// subscript not constant, or the whole array or a section in an array
// expression, which the scalarizer reads as an array of its own whose lower
// bounds are 1.
//
// A constructor whose values aren't constants is built when the statement
// runs, its items in order, each stored at the next position of its target: an
// array temporary the scalarizer then reads (prepare), or an assignment's
// target itself, when the constructor reads nothing of the target's (sema's
// ArrayOrder.Fill). So
//
//	A = (/ X, (I * Y, I = 1, N), B(2:3) /)
//
// is, with $I1 the running position,
//
//	$I1 = 0
//	$I1 = $I1 + 1;  A($I1) = X
//	DO I' = 1, N
//	  $I1 = $I1 + 1;  A($I1) = I' * Y
//	END DO
//	DO $I2 = 1, 2
//	  $I1 = $I1 + 1;  A($I1) = B($I2 + 1)
//	END DO
//
// I' being the implied DO's own variable (its scope is the implied DO), an
// index cell of the statement's. Under /CHECK=BOUNDS a position past the
// target's extent, or a constructor that ends short of it, doesn't conform
// (FTN$NOTCONFOR).

// constExtents are an array constant's or a constructor's extents, every one
// known.
func constExtents(e sema.Expr) []int {
	list := make([]int, len(e.Shape()))
	for i, x := range e.Shape() {
		list[i] = x.Size
	}

	return list
}

// arrayConstMem is the place of the array constant c's first element: a datum
// of its elements, one per routine for each contents.
func (l *lowerer) arrayConstMem(c *sema.ArrayConst) *Mem {
	if c.T.Kind == sema.Character {
		var b strings.Builder
		for _, e := range c.Elems {
			b.WriteString(e.Text)
		}

		return l.text(b.String())
	}

	t := l.typeOf(c.T)
	values := make([]*Const, len(c.Elems))

	var key strings.Builder

	key.WriteString("A " + t.String())

	for i, e := range c.Elems {
		values[i] = l.constOf(e)
		fmt.Fprintf(&key, " %x:%x:%x:%x", values[i].Bits, values[i].High, values[i].ImBits, values[i].ImHigh)
	}

	if len(values) == 0 {
		// An empty array is never read: any place will do.
		return l.constant(LongConst(0))
	}

	d := l.datum(key.String(), func() *Datum {
		l.konsts++

		return &Datum{Name: "$K" + strconv.Itoa(l.konsts), Kind: ArrayDatum, Values: values}
	})

	return &Mem{Datum: d, T: t}
}

// constantArrayMem is where the array constant s (an array PARAMETER) is, for
// an element or section of it whose subscripts aren't constants.
func (l *lowerer) constantArrayMem(s *sema.Symbol) *Mem {
	return l.arrayConstMem(s.ArrayValue)
}

// fillConstructor lowers the constructor c's items into target, an array of
// rank one, from its first element on.
func (l *lowerer) fillConstructor(target sema.Expr, c *sema.Constructor) {
	n := l.newNest(1, nil)

	for _, leaf := range sema.Leaves(target) {
		n.prepare(leaf)
	}

	// The checks need the target's extent, unless it and the constructor's
	// are known to be equal.
	checked := l.unit.Checks.Bounds && (!target.Shape()[0].Known || !c.S[0].Known)

	var extent Expr
	if checked {
		extent = l.expr(n.extent(0, target, target))
	}

	element := sema.Scalarize(target, n.element, n.hoist)
	position := l.symbolMem(n.index[0])

	l.emit(&Assign{origin: l.origin(), Dst: position, Src: LongConst(0)})

	put := func(v sema.Expr) {
		l.emit(&Assign{origin: l.origin(), Dst: position, Src: arith(Add, Long, &Load{position}, LongConst(1))})

		if checked {
			l.notConforming(&Cmp{Gt, &Load{position}, extent})
		}

		l.assignScalar(element, v)
	}

	l.fillItems(c.Items, put)

	if checked {
		l.notConforming(&Cmp{Ne, &Load{position}, extent})
	}

	n.done()
}

// fillItems lowers each of a constructor's items, giving put each value: a
// scalar, each element of an array in array element order, and an implied
// DO's items once for each value of its variable.
func (l *lowerer) fillItems(items []sema.CtorItem, put func(sema.Expr)) {
	for _, it := range items {
		switch {
		case it.Do != nil:
			do := it.Do

			place := &Mem{Cell: l.indexCell(), T: Long}
			if t := l.typeOf(do.Var.Type); t != Long {
				place = l.temp(t)
			}

			l.bound[do.Var] = place

			name := "constructor (line " + strconv.Itoa(l.from.Position().Line) + "), implied DO of " + do.Var.Name

			l.countedLoop(name, do.Var, do.Start, do.Limit, do.Step, false, func(*Block) {
				l.fillItems(do.Items, put)
			})

			delete(l.bound, do.Var)

		case len(it.Value.Shape()) == 0:
			put(it.Value)

		default:
			l.eachElement(it.Value, put)
		}
	}
}

// notConforming lowers a test of cond that calls FTN$NOTCONFOR, which doesn't
// return, if it holds.
func (l *lowerer) notConforming(cond Expr) {
	bad, next := l.newBlock(), l.newBlock()
	l.branch(cond, bad, next)

	l.begin(bad)
	l.emit(&Call{origin: l.origin(), Routine: ftnNotConfor})
	l.terminate(&NoReturn{l.origin()})

	l.begin(next)
}
