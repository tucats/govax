package ir

import (
	"slices"
	"strconv"

	"github.com/tucats/govax/internal/fortran/sema"
)

// Array temporaries whose size is known only when the program runs
// (docs/planning/PHASE-104 - Fortran 90 dynamic arrays.md, findings 6 and 8).
// An array statement that needs a temporary (an assignment whose sides
// overlap, a WHERE's mask kept, a constructor, a vector subscript in the
// target that reads the target) gets a cell of a size known when compiling,
// arrayTemp, when it can; when it can't,
//
//	SUBROUTINE SHIFT(A, N)
//	REAL A(N)
//	A(N:1:-1) = A
//
// the temporary is kept as an automatic array is (lowerautomatic.go): a class A
// descriptor cell ($Rn) that says where its storage is, standing among the
// adjustable arrays' (adjustableOf), so that the scalarizer reads its bounds,
// extents, and elements as it reads any adjustable array's. Its lower bounds
// are 1, and the statement stores each extent, the size, the storage's
// address, and A0:
//
//	MOVL    $T1,$R1+20              ; M1 and U1, the extent
//	MOVL    $T1,$R1+28
//	MULL3   #4,$R1+20,$R1+12        ; ARSIZE
//	MOVL    SP,$T2                  ; where the statement's storage starts
//	CMPL    $R1+12,#65536
//	BGTR    1$
//	SUBL2   $R1+12,SP               ; up to 64 KB: the stack
//	BICL2   #3,SP
//	MOVL    SP,$R1+4
//	BRB     2$
//	1$: PUSHAB $R1                  ; past it: the heap
//	CALLS   #1,G^FTN$GET_TEMP
//	2$: ...                          ; A0, then the loops
//	...
//	MOVL    $T2,SP                  ; the statement's end: the stack given back
//
// The storage is the statement's: at its end (endTemps) a heap block goes back
// to FTN$DEALLOCATE and SP back to where it was before the first, so a
// statement in a loop doesn't grow the stack. A statement left by an unwind
// leaves a heap block allocated (the stack's comes back with the frame).

// tempStackLimit is the most bytes a temporary takes from the stack: a bigger
// one comes from the heap (the author's answer 1: a constant of the compiler).
const tempStackLimit = 65536

// ftnGetTemp is FTNRTL's routine that takes a temporary's storage from the
// heap (mar/ftn_alloc.mar).
const ftnGetTemp = "FTN$GET_TEMP"

// runtimeTemps is what the statement being lowered must give back at its end:
// its run-time temporaries' descriptors, where SP was before the first, and the
// statements that free the dynamic strings of values computed once before an
// array statement's loops (hoistText); and the values of array-valued
// functions of run-time shape it has called, by reference, with their
// descriptors' cells (lowervalue.go).
type runtimeTemps struct {
	cells      []*Cell
	syms       []*sema.Symbol
	mark       *Mem
	after      []Stmt
	values     map[*sema.FuncRef]*sema.ArrayRef
	valueCells []*Cell

	// funcs are the transformational functions' values the statement has
	// computed (lowertransform.go), whatever the temporaries they are in.
	funcs map[*sema.ArrayFunc]*sema.ArrayRef
}

// runtimeTemp is an array temporary of type t for the statement being lowered,
// with the extents given (Longs, none below 0), as a whole array; a CHARACTER
// one whose length isn't known when compiling (t.Size 0) has length, a Long,
// as its elements' length.
func (l *lowerer) runtimeTemp(t sema.Type, extents []Expr, length Expr) *sema.ArrayRef {
	sym := sema.NewRuntimeArray("$W", t, len(extents))

	d := &Descriptor{DType: l.dtypeOf(t), Class: ClassA, Length: t.Size}
	for range extents {
		d.Dims = append(d.Dims, Bound{1, 0, true})
	}

	key := descriptorKey(d)

	var c *Cell

	if free := l.freeRuntime[key]; len(free) > 0 {
		c = free[len(free)-1]
		l.freeRuntime[key] = free[:len(free)-1]
	} else {
		l.runtimes++
		c = l.newCell("$R"+strconv.Itoa(l.runtimes), DescriptorCell, Long, d.Size())
		c.Desc = d
	}

	// The cell's Sym says, to the optimizer (opt.AllocatedRegion) and the
	// storage map, that it describes a temporary's storage; it is the last
	// statement's to use the cell.
	c.Sym = sym
	l.busy = append(l.busy, c)
	l.fillDescriptor(c)

	if l.adjustable == nil {
		l.adjustable = map[*sema.Symbol]*Cell{}
	}

	l.adjustable[sym] = c

	field := func(offset int) *Mem {
		return &Mem{Cell: c, Offset: offset, T: Long}
	}

	store := func(offset int, x Expr) {
		l.emit(&Assign{origin: l.origin(), Dst: field(offset), Src: x})
	}

	count := Expr(LongConst(int32(t.Size)))

	if t.Kind == sema.Character && t.Size == 0 {
		// The length is the descriptor's first word, which every use reads
		// (runtimeLength).
		l.emit(&Assign{origin: l.origin(), Dst: &Mem{Cell: c, T: Word}, Src: arith(Cvt, Word, length, nil)})
		count = &Op{Zext, Long, &Load{&Mem{Cell: c, T: Word}}, nil}
	}

	for i, e := range extents {
		store(d.MultiplierOffset(i), e)
		store(d.LowerOffset(i)+Long.Size(), &Load{field(d.MultiplierOffset(i))})
		count = arith(Mul, Long, count, &Load{field(d.MultiplierOffset(i))})
	}

	store(ArsizeOffset, count)

	// The storage: SP marked before the statement's first, then the stack or
	// the heap by the size.
	temps := &l.runtime
	if temps.mark == nil {
		temps.mark = l.temp(Long)
		l.emit(&StackMark{origin: l.origin(), Dst: temps.mark})
	}

	temps.cells = append(temps.cells, c)
	temps.syms = append(temps.syms, sym)

	heap, stack, next := l.newBlock(), l.newBlock(), l.newBlock()
	l.branch(&Cmp{Gt, &Load{field(ArsizeOffset)}, LongConst(tempStackLimit)}, heap, stack)

	l.begin(stack)
	l.emit(&StackAllocate{origin: l.origin(), Size: &Load{field(ArsizeOffset)}, Dst: field(PointerOffset)})
	l.jump(next)

	l.begin(heap)
	l.emit(&Call{origin: l.origin(), Routine: ftnGetTemp, Args: []Arg{{Ref: field(0)}}})
	l.jump(next)

	l.begin(next)

	// A0: the first element's address, less the element number of all ones
	// counting from all zeros (computeAutomatic).
	base := &Mem{Ptr: &Load{field(PointerOffset)}, T: l.typeOf(t)}
	first := Expr(LongConst(1))

	for i := len(extents) - 2; i >= 0; i-- {
		first = arith(Add, Long, arith(Mul, Long, first, &Load{field(d.MultiplierOffset(i))}), LongConst(1))
	}

	store(A0Offset, &Addr{l.elementAt(base, sym, negated(first))})

	return sema.NewArrayRef(sym, l.here())
}

// endTemps gives back what the statement's run-time temporaries took, at its
// end: the dynamic strings of values computed once, each heap block, the
// functions' values, then the stack. It must be lowered where control passes once at the statement's end.
func (l *lowerer) endTemps() {
	temps := l.runtime
	l.runtime = runtimeTemps{}

	for _, s := range temps.after {
		l.emit(s)
	}

	for i := len(temps.cells) - 1; i >= 0; i-- {
		c := temps.cells[i]
		size := &Load{&Mem{Cell: c, Offset: ArsizeOffset, T: Long}}

		free, next := l.newBlock(), l.newBlock()
		l.branch(&Cmp{Gt, size, LongConst(tempStackLimit)}, free, next)

		l.begin(free)
		l.emit(&Call{origin: l.origin(), Routine: ftnDeallocate, Args: []Arg{{Ref: &Mem{Cell: c, T: Long}}, {Value: LongConst(0)}}})
		l.jump(next)

		l.begin(next)
	}

	for _, c := range temps.valueCells {
		l.freeValue(c)
	}

	if temps.mark != nil {
		l.emit(&StackRelease{origin: l.origin(), To: &Load{temps.mark}})
	}

	for _, sym := range temps.syms {
		delete(l.adjustable, sym)
	}

	for _, ref := range temps.values {
		delete(l.adjustable, ref.Sym)
	}
}

// pendingTemps reports whether the statement being lowered has something for
// endTemps to give back.
func (l *lowerer) pendingTemps() bool {
	return l.runtime.mark != nil || len(l.runtime.after) > 0 || len(l.runtime.valueCells) > 0
}

// stmtCond is cond for a statement's own branch (an IF's, a DO WHILE's): if the
// condition made run-time temporaries, its value is kept and they are given
// back before the branch, which is the statement's end on both its ways.
func (l *lowerer) stmtCond(e sema.Expr) Expr {
	c := l.cond(e)

	if !l.pendingTemps() {
		return c
	}

	t := l.temp(c.Type())
	l.emit(&Assign{origin: l.origin(), Dst: t, Src: c})
	l.endTemps()

	return &Load{t}
}

// shapeTemp is an array temporary of type t for a value of the shape given:
// a cell of the extents known when compiling (arrayTemp) if every one is
// (sizes, sema's tempShape: -1 for one that isn't), else a run-time
// temporary, each extent not known computed now from the array the shape
// says it is (extentsOf). length is a CHARACTER element's length when t's
// isn't known (t.Size 0).
func (l *lowerer) shapeTemp(t sema.Type, sizes []int, shape sema.Shape, length Expr) *sema.ArrayRef {
	if !slices.Contains(sizes, -1) && (t.Kind != sema.Character || t.Size > 0) {
		return l.arrayTemp(t, sizes)
	}

	return l.runtimeTemp(t, l.extentsOf(shape), length)
}

// extentsOf is each extent of the shape: a constant where it is known, else
// computed now, never below 0, from the array (a leaf) it is an extent of, as
// an array statement's loops would compute it.
func (l *lowerer) extentsOf(shape sema.Shape) []Expr {
	n := l.newNest(len(shape), nil)
	defer n.done()

	list := make([]Expr, len(shape))

	for d, e := range shape {
		if e.Known {
			list[d] = LongConst(int32(e.Size))

			continue
		}

		// leafExtent's value is a constant or a temporary's, which stays the
		// statement's after the nest is done.
		n.prepare(e.Of)
		list[d] = n.leafExtent(e.Of, n.loopOf(e.Of, e.Dim))
	}

	return list
}

// hoistText is the CHARACTER scalar e computed now, once, before an array
// statement's loops (a function's value, or a concatenation whose length is
// known only when the program runs), as a symbol of the statement's bound to
// where its text is. A dynamic string's freeing waits for the statement's end
// (endTemps), not the first branch of its loops; a length known only now is
// kept in a cell the symbol's length is read from.
func (n *arrayNest) hoistText(e sema.Expr) sema.Expr {
	l := n.l
	k := len(l.after)
	t := l.chars(e)

	l.runtime.after = append(l.runtime.after, l.after[k:]...)
	l.after = l.after[:k]

	sym := sema.NewSymbol("$V", e.Type())
	l.bound[sym] = t.Mem
	n.kept = append(n.kept, sym)

	if e.Type().Size == 0 {
		m := l.temp(Long)
		l.emit(&Assign{origin: l.origin(), Dst: m, Src: t.Len})

		// A length's value only marks it as one known when the program runs
		// (charLength); the cell holds it.
		sym.LengthValue = sema.IntConstAt(0, e.Position())

		if l.lengths == nil {
			l.lengths = map[*sema.Symbol]*Cell{}
		}

		l.lengths[sym] = m.Cell
	}

	return sema.NewVar(sym, e.Position())
}

// copyVectors is the target section of an array assignment with each of its
// vector subscripts copied into a temporary first, since the vectors read the
// target's own array (IV(IV) = 1): the target's subscripts must all be known
// before any element is defined (Fortran 90's 7.5.1.5).
func (l *lowerer) copyVectors(target sema.Expr) sema.Expr {
	sub, _ := target.(*sema.Substring)

	x := target
	if sub != nil {
		x = sub.X
	}

	sec := x.(*sema.Section)
	cp := *sec
	cp.Subs = slices.Clone(sec.Subs)
	cp.S = slices.Clone(sec.S)

	k := 0

	for i, s := range sec.Subs {
		if !s.Ranges() {
			continue
		}

		if v := s.Vector; v != nil {
			temp := l.shapeTemp(v.Type(), sizesOf(v.Shape()), v.Shape(), nil)
			l.arrayNest(temp, v, nil, nil)
			cp.Subs[i].Vector = temp

			if !cp.S[k].Known {
				cp.S[k] = sema.Extent{Of: temp, Dim: 0}
			}
		} else if cp.S[k].Of == sec {
			cp.S[k].Of = &cp
		}

		k++
	}

	if sub == nil {
		return &cp
	}

	s := *sub
	s.X, s.S = &cp, cp.S

	return &s
}

// sizesOf is the shape's extents, -1 for one known only when the program
// runs (sema's tempShape).
func sizesOf(shape sema.Shape) []int {
	list := make([]int, len(shape))

	for i, e := range shape {
		list[i] = e.Size
		if !e.Known {
			list[i] = -1
		}
	}

	return list
}

// knownShape is an assignment's shape as its loops take it: in each
// dimension the target's extent if that is known when compiling, else the
// value's if that is, else the target's (arrayNest.extent).
func (l *lowerer) knownShape(target, value sema.Shape) sema.Shape {
	shape := slices.Clone(target)

	for d := range shape {
		if !shape[d].Known && d < len(value) && value[d].Known {
			shape[d] = value[d]
		}
	}

	return shape
}

// tempType is the type of the temporary an array assignment's value goes
// through, and for a CHARACTER one whose length isn't known when compiling
// that length: the value's type if its length is known, else the target's,
// the value being made the target's length on its way in, whatever it is
// computed in; and if neither is known the target's elements' length, as the
// program runs.
func (l *lowerer) tempType(target, value sema.Expr) (sema.Type, Expr) {
	t := value.Type()
	if t.Kind != sema.Character || t.Size > 0 {
		return t, nil
	}

	if t = target.Type(); t.Size > 0 {
		return t, nil
	}

	return t, l.elementLength(target)
}

// elementLength is the length of each element of the CHARACTER array target
// (an array, a section, or a substring of one), as the program runs.
func (l *lowerer) elementLength(target sema.Expr) Expr {
	sub, ok := target.(*sema.Substring)
	if !ok {
		return l.charLength(arraySymbol(target))
	}

	upper := l.charLength(arraySymbol(sub.X))
	if sub.Upper != nil {
		upper = l.expr(sub.Upper)
	}

	m := l.temp(Long)
	l.emit(&Assign{origin: l.origin(), Dst: m, Src: arith(Add, Long, arith(Sub, Long, upper, l.expr(sub.Lower)), LongConst(1))})
	l.notNegative(m)

	return &Load{m}
}

// arraySymbol is the array a whole array or a section is of.
func arraySymbol(e sema.Expr) *sema.Symbol {
	switch e := e.(type) {
	case *sema.ArrayRef:
		return e.Sym
	case *sema.Section:
		return e.Sym
	}

	panic("ir: no array for a CHARACTER temporary's length")
}

// countItems is how many elements the constructor's items make, as the
// program runs: each scalar one, each array its size, and an implied DO its
// items' count for each value of its variable, which is run through for it
// (the values themselves aren't computed).
func (l *lowerer) countItems(items []sema.CtorItem) Expr {
	count := l.temp(Long)
	l.emit(&Assign{origin: l.origin(), Dst: count, Src: LongConst(0)})

	add := func(e Expr) {
		l.emit(&Assign{origin: l.origin(), Dst: count, Src: arith(Add, Long, &Load{count}, e)})
	}

	var walk func([]sema.CtorItem)

	walk = func(items []sema.CtorItem) {
		for _, it := range items {
			switch {
			case it.Do != nil:
				do := it.Do

				place := &Mem{Cell: l.indexCell(), T: Long}
				if t := l.typeOf(do.Var.Type); t != Long {
					place = l.temp(t)
				}

				l.bound[do.Var] = place

				name := "constructor (line " + strconv.Itoa(l.here().Line) + "), count of implied DO of " + do.Var.Name

				l.countedLoop(name, do.Var, do.Start, do.Limit, do.Step, false, func(*Block) {
					walk(do.Items)
				})

				delete(l.bound, do.Var)

			case len(it.Value.Shape()) == 0:
				add(LongConst(1))

			default:
				var size Expr = LongConst(1)
				for _, e := range l.extentsOf(it.Value.Shape()) {
					size = arith(Mul, Long, size, e)
				}

				add(size)
			}
		}
	}

	walk(items)

	return &Load{count}
}

// ctorLength is the length of a CHARACTER constructor's elements when it isn't
// known when compiling: its first value's (sema's Constructor.LengthOf), nil
// for any other.
func (l *lowerer) ctorLength(c *sema.Constructor) Expr {
	if c.T.Kind != sema.Character || c.T.Size > 0 {
		return nil
	}

	return l.valueLength(c.LengthOf)
}

// valueLength is the length of the CHARACTER value e, or of each of its
// elements, as the program runs: a variable's or an array's, or a
// concatenation's parts' sum (the forms sema's Constructor.LengthOf may be).
func (l *lowerer) valueLength(e sema.Expr) Expr {
	switch e := e.(type) {
	case *sema.Var:
		return l.charLength(e.Sym)
	case *sema.Elem:
		return l.charLength(e.Sym)
	case *sema.ArrayRef:
		return l.charLength(e.Sym)
	case *sema.Section:
		return l.charLength(e.Sym)
	case *sema.Concat:
		var sum Expr

		for _, p := range e.Parts {
			sum = arith(Add, Long, sum, l.valueLength(p))
		}

		return sum
	}

	if n := e.Type().Size; n > 0 {
		return LongConst(int32(n))
	}

	panic("ir: a CHARACTER length lowering can't find")
}

// stmtExpr is expr for the value a statement's own branch goes by (an
// arithmetic IF's, a computed GO TO's, an INTEGER CASE construct's), as
// stmtCond is for a condition.
func (l *lowerer) stmtExpr(e sema.Expr) Expr {
	v := l.expr(e)

	if !l.pendingTemps() {
		return v
	}

	t := l.temp(v.Type())
	l.emit(&Assign{origin: l.origin(), Dst: t, Src: v})
	l.endTemps()

	return &Load{t}
}
