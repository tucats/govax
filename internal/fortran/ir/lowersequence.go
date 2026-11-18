package ir

import (
	"github.com/tucats/govax/internal/fortran/sema"
)

// Sections and array expressions given to dummies that aren't assumed-shape
// (sema's sequence.go; docs/planning/PHASE-104 - Fortran 90 dynamic arrays.md,
// finding 11). Such a dummy takes the address of elements one after another, so
// each actual is passed one of three ways:
//
//   - a section whose elements are known to be one after another (A(:, J)) as
//     its first element's address, no copy;
//   - any other section, a substring of one, or an array expression, through a
//     temporary of the statement's (valueTemp), copied in before the call and,
//     for a section the routine may change, back after it:
//
//     MOVAB   A+4,R6               ; the loops copying A(1:9:2) in
//     ...
//     PUSHAB  S.$W1                ; the temporary
//     CALLS   #1,G^TOTAL
//     ...                          ; the loops copying it back
//
//   - an assumed-shape array, or a section of one, by a test of its descriptor's
//     strides when the program runs (contiguityTest): the first must be the
//     element's size and each next the one before times its extent; if they
//     are, its first element's address with no copy, else a temporary as above,
//     and a flag saying which, so that only a copy is copied back:
//
//     CMPL    S.$B1+20,#4          ; S1 the element's size?
//     BNEQ    1$
//     ...
//
// A CHARACTER one is passed by a class A descriptor of whichever it is
// (ncaClassA, arrayDescriptor). The copy back is the call's: a function
// reference's is done when its call returns, before the rest of its statement
// (12.4.2's order). The same mechanism serves an assumed-shape array used as a
// run-time format, an internal file, or ENCODE's or DECODE's buffer
// (lowerio.go).

// sequenced is an array's elements one after another, as a routine or FTNRTL
// is given them: value is their first element's address, or the address of a
// class A descriptor of them; back, if not nil, copies a temporary's elements
// back to the array, and must be called once what it was given to is done.
type sequenced struct {
	value Expr
	back  func()
}

// sequenceArg is the actual argument a, marked Sequence by sema: the address of
// its elements one after another, or for a CHARACTER one a class A descriptor's,
// and what copies them back after the call (nil if nothing does).
func (l *lowerer) sequenceArg(a sema.Arg) (Arg, func()) {
	char := a.Expr.Type().Kind == sema.Character
	s := l.sequence(a.Expr, sema.Described(a), a.CopyBack, char)

	return Arg{Value: s.value}, s.back
}

// sequence is the array value e's elements one after another (sequenced):
// described says e is a whole array, a section without a vector subscript, or a
// substring of either, none in parentheses, which a copy goes back to if back;
// any other value is computed into a temporary, never copied back. desc asks
// for a class A descriptor's address rather than the first element's.
//
// The statements that run at a statement's end (after: the freeing of dynamic
// strings) are held aside while the copies are lowered, since a copy's loops
// end blocks, and a block's end does them: they belong to the whole
// statement, which may still use them.
func (l *lowerer) sequence(e sema.Expr, described, back, desc bool) sequenced {
	var s sequenced

	l.holdingAfter(func() { s = l.sequenceOf(e, described, back, desc) })

	if s.back != nil {
		copyBack := s.back
		s.back = func() { l.holdingAfter(copyBack) }
	}

	return s
}

// holdingAfter runs f with the statement's after list held aside, then puts it
// back before whatever f left there.
func (l *lowerer) holdingAfter(f func()) {
	held := l.after
	l.after = nil

	f()

	l.after = append(held, l.after...)
}

// sequenceOf is sequence's work.
func (l *lowerer) sequenceOf(e sema.Expr, described, back, desc bool) sequenced {
	if described {
		switch x := e.(type) {
		case *sema.ArrayRef:
			// A whole assumed-shape array (sema passes any other as itself),
			// with the shape an argument's reference to it has no need of
			// but the copies' loops do.
			e = sema.NewArrayRef(x.Sym, x.Pos)

			return l.testedSequence(e, l.wholeNCA(x), len(x.Sym.Dims), back, desc)

		case *sema.Section:
			switch {
			case x.Sym.AssumedShape():
				return l.testedSequence(e, l.sectionNCA(x, nil), len(x.S), back, desc)

			case sema.Contiguous(x):
				return sequenced{value: l.contiguousSection(x, desc)}
			}
		}
	}

	temp := l.valueTemp(e)
	s := sequenced{value: l.fromTemp(temp, desc)}

	if described && back {
		s.back = func() { l.arrayNest(e, temp, nil, nil) }
	}

	return s
}

// fromTemp is the temporary temp's first element's address, or with desc a
// class A descriptor's of it.
func (l *lowerer) fromTemp(temp *sema.ArrayRef, desc bool) Expr {
	if desc {
		return &Addr{l.arrayDescriptor(temp.Sym)}
	}

	return &Addr{l.symbolMem(temp.Sym)}
}

// fromNCA is the first element's address that the class NCA descriptor nca, of
// rank dims, holds, or with desc a class A descriptor's of the same elements,
// which must be one after another.
func (l *lowerer) fromNCA(nca *Mem, dims int, desc bool) Expr {
	if desc {
		return &Addr{l.ncaClassA(nca, dims)}
	}

	return &Load{&Mem{Cell: nca.Cell, Offset: PointerOffset, T: Long}}
}

// contiguousSection is the section e, whose elements are known to be one after
// another (sema.Contiguous), as its first element's address, or with desc a
// class A descriptor's of it, its lower bounds 1 and its extents the
// triplets'.
func (l *lowerer) contiguousSection(e *sema.Section, desc bool) Expr {
	m := l.describeSection(e, nil, func(el ncaElement, pointer Expr, dims []ncaDim) *Mem {
		if desc {
			return l.buildClassA(e.Sym, el, pointer, dims)
		}

		// Kept in a temporary: the address may be computed from the
		// section's own cells, which the next argument's may reuse.
		t := l.temp(Long)
		l.emit(&Assign{origin: l.origin(), Dst: t, Src: pointer})

		return t
	})

	if desc {
		return &Addr{m}
	}

	return &Load{m}
}

// buildClassA is a class A descriptor, in a descriptor cell of the statement's,
// of elements of the array s that are el, one after another from pointer, with
// the dimensions dims (each with lower bound 1): what an old CHARACTER dummy
// takes.
func (l *lowerer) buildClassA(s *sema.Symbol, el ncaElement, pointer Expr, dims []ncaDim) *Mem {
	d := &Descriptor{DType: l.dtypeOf(el.t), Class: ClassA, Length: el.t.Size}
	for range dims {
		d.Dims = append(d.Dims, Bound{1, 0, true})
	}

	c := l.descriptorCell(d)

	field := func(offset int) *Mem { return &Mem{Cell: c, Offset: offset, T: Long} }

	store := func(offset int, x Expr) {
		l.emit(&Assign{origin: l.origin(), Dst: field(offset), Src: x})
	}

	length := Expr(LongConst(int32(el.t.Size)))

	if el.t.Kind == sema.Character && el.t.Size == 0 {
		word := el.length
		if word == nil {
			word = l.lengthWord(s)
		} else {
			word = arith(Cvt, Word, word, nil)
		}

		l.emit(&Assign{origin: l.origin(), Dst: &Mem{Cell: c, T: Word}, Src: word})
		length = &Op{Zext, Long, &Load{&Mem{Cell: c, T: Word}}, nil}
	}

	store(PointerOffset, pointer)

	// A0: POINTER less the length times the element number of all ones,
	// 1 + M1 + M1*M2 + ...
	step, before := length, Expr(LongConst(0))

	for i, dm := range dims {
		store(d.MultiplierOffset(i), dm.extent)
		store(d.LowerOffset(i)+Long.Size(), &Load{field(d.MultiplierOffset(i))})

		before = arith(Add, Long, before, step)
		step = arith(Mul, Long, step, &Load{field(d.MultiplierOffset(i))})
	}

	store(ArsizeOffset, step)
	store(A0Offset, arith(Sub, Long, &Load{field(PointerOffset)}, before))

	return &Mem{Cell: c, T: Long}
}

// testedSequence is sequence for e, an assumed-shape array or a section of one,
// described by nca, of rank dims: its elements passed where they are if its
// strides say they are one after another, else through a temporary, a flag
// saying which so that only a copy is copied back (the author's answer 4).
func (l *lowerer) testedSequence(e sema.Expr, nca *Mem, dims int, back, desc bool) sequenced {
	copied, place := l.temp(Long), l.temp(Long)

	// A run-time temporary is made in only one of the two ways, but the
	// statement's end gives the stack back on both, so SP is marked before.
	if l.runtime.mark == nil {
		l.runtime.mark = l.temp(Long)
		l.emit(&StackMark{origin: l.origin(), Dst: l.runtime.mark})
	}

	contiguous, copying, join := l.newBlock(), l.newBlock(), l.newBlock()
	l.contiguityTest(nca, dims, contiguous, copying)

	l.begin(copying)
	temp := l.valueTemp(e)
	l.emit(&Assign{origin: l.origin(), Dst: copied, Src: LongConst(1)})
	l.emit(&Assign{origin: l.origin(), Dst: place, Src: l.fromTemp(temp, desc)})
	l.jump(join)

	l.begin(contiguous)

	// A run-time temporary not made has nothing for the statement's end to
	// give back: its size 0 says so (endTemps).
	if c := l.adjustableOf(temp.Sym); c != nil {
		l.emit(&Assign{origin: l.origin(), Dst: &Mem{Cell: c, Offset: ArsizeOffset, T: Long}, Src: LongConst(0)})
	}

	l.emit(&Assign{origin: l.origin(), Dst: copied, Src: LongConst(0)})
	l.emit(&Assign{origin: l.origin(), Dst: place, Src: l.fromNCA(nca, dims, desc)})
	l.jump(join)

	l.begin(join)

	s := sequenced{value: &Load{place}}

	if back {
		s.back = func() {
			copyBack, next := l.newBlock(), l.newBlock()
			l.branch(&Cmp{Op: Ne, X: &Load{copied}, Y: LongConst(0)}, copyBack, next)

			l.begin(copyBack)
			l.arrayNest(e, temp, nil, nil)
			l.jump(next)

			l.begin(next)
		}
	}

	return s
}

// contiguityTest goes to yes if the class NCA descriptor nca, of rank dims,
// describes elements one after another in array element order (its first
// stride the element's size, each next the one before times the extent before),
// else to no.
func (l *lowerer) contiguityTest(nca *Mem, dims int, yes, no *Block) {
	d := nca.Cell.Desc

	field := func(offset int) Expr {
		return &Load{&Mem{Cell: nca.Cell, Offset: offset, T: Long}}
	}

	expected := Expr(&Op{Zext, Long, &Load{&Mem{Cell: nca.Cell, T: Word}}, nil})

	for i := range dims {
		next := yes
		if i < dims-1 {
			next = l.newBlock()
		}

		l.branch(&Cmp{Op: Eq, X: field(d.StrideOffset(i)), Y: expected}, next, no)

		if i < dims-1 {
			l.begin(next)

			upper := d.LowerOffset(i) + Long.Size()
			extent := arith(Add, Long, arith(Sub, Long, field(upper), field(d.LowerOffset(i))), LongConst(1))
			expected = arith(Mul, Long, field(d.StrideOffset(i)), extent)
		}
	}
}

// ncaClassA is a class A descriptor, in a descriptor cell of the statement's, of
// the elements the class NCA descriptor nca, of rank dims, describes, which must
// be one after another: the same length, POINTER, and size, lower bounds of 1,
// and each extent its multiplier and upper bound, A0 from those. What an old
// CHARACTER dummy takes.
func (l *lowerer) ncaClassA(nca *Mem, dims int) *Mem {
	src := nca.Cell
	sd := src.Desc

	d := &Descriptor{DType: sd.DType, Class: ClassA, Length: sd.Length}
	for range dims {
		d.Dims = append(d.Dims, Bound{1, 0, true})
	}

	c := l.descriptorCell(d)

	from := func(offset int) Expr { return &Load{&Mem{Cell: src, Offset: offset, T: Long}} }
	field := func(offset int) *Mem { return &Mem{Cell: c, Offset: offset, T: Long} }

	store := func(offset int, x Expr) {
		l.emit(&Assign{origin: l.origin(), Dst: field(offset), Src: x})
	}

	l.emit(&Assign{origin: l.origin(), Dst: &Mem{Cell: c, T: Word}, Src: &Load{&Mem{Cell: src, T: Word}}})
	store(PointerOffset, from(PointerOffset))
	store(ArsizeOffset, from(ArsizeOffset))

	// A0 is where the element of all zero subscripts would be: POINTER less
	// the length times 1 + M1 + M1*M2 + ..., the element number of all ones.
	step := Expr(&Op{Zext, Long, &Load{&Mem{Cell: c, T: Word}}, nil})
	before := Expr(LongConst(0))

	for i := range dims {
		upper := sd.LowerOffset(i) + Long.Size()
		store(d.MultiplierOffset(i), arith(Add, Long, arith(Sub, Long, from(upper), from(sd.LowerOffset(i))), LongConst(1)))
		store(d.LowerOffset(i)+Long.Size(), &Load{field(d.MultiplierOffset(i))})

		before = arith(Add, Long, before, step)
		step = arith(Mul, Long, step, &Load{field(d.MultiplierOffset(i))})
	}

	store(A0Offset, arith(Sub, Long, &Load{field(PointerOffset)}, before))

	return &Mem{Cell: c, T: Long}
}
