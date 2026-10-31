package ir

import (
	"slices"
	"strconv"

	"github.com/tucats/govax/internal/fortran/sema"
)

// Adjustable arrays (section 6.1.1.1 of the language manual). In
//
//	SUBROUTINE SUB(X, I, J)
//	DIMENSION X(-I/2:I/2, J)
//
// X's shape is whatever I and J hold when SUB is entered, and stays that way:
// "these values do not change during the execution of that subprogram even if
// the values of variables contained in the array declaration are changed". So
// the bounds are computed once, at each entry, and kept.
//
// They are kept in a class A descriptor of the array ($B1, a cell of $LOCAL),
// which is what a whole array needs to be passed or written anyway, and which
// holds all an element's address needs (the OpenVMS Calling Standard, chapter 7):
//
//	POINTER  the address of the first element
//	ARSIZE   the array's size in bytes
//	A0       the address the element whose subscripts are all 0 would have
//	Mi       dimension i's extent, its upper bound less its lower bound plus 1
//	Li, Ui   dimension i's bounds
//
// Since the first subscript varies fastest (section 2.2.3.3), the element
// X(s1, s2) is element number
//
//	(s1 - L1) + (s2 - L2)*M1  =  (s1 + s2*M1) - (L1 + L2*M1)
//
// from the first. The part on the right doesn't depend on the subscripts, and
// A0 has it built in, so the address of any element is A0 plus (s1 + s2*M1)
// times the element's size, which the VAX's indexed mode does the last step of:
//
//	MOVL   SUB.$B1+16,R1        A0
//	MOVL   SUB.J,R0             s2 (a subscript)
//	MULL2  SUB.$B1+20,R0        times M1
//	ADDL2  SUB.K,R0             plus s1
//	MOVF   (R1)[R0],R2          the element
//
// A dimension whose bounds are constant needs no storing, and its constants are
// used where they are. The last dimension's extent is never used for an element,
// which is why an assumed-size array's can be unknown: A(1:N, 1:*) is an
// adjustable array too, whose descriptor has no ARSIZE. (It is never used as a
// whole descriptor: sema refuses such an array in an I/O list, and it is passed
// by its first element.)
//
// Each entry point computes the bounds of the arrays its own list defines: those
// in its list whose bounds use only its other dummy arguments and variables in
// common blocks. Section 6.1.1.1 calls any other array undefined until an entry
// point that does define it, and here its cell keeps what it held.

// prepareBounds makes a descriptor cell for each adjustable array. A constant
// bound is in the cell from the start, so only the others are stored.
func (l *lowerer) prepareBounds() {
	n := 0

	for _, s := range l.unit.Symbols {
		if s.Class != sema.Array || !s.Adjustable() {
			continue
		}

		d := &Descriptor{DType: l.dtypeOf(s.Type), Class: ClassA, Length: s.Type.Size}

		for _, dim := range s.Dims {
			d.Dims = append(d.Dims, Bound{dim.Lower, dim.Upper, dim.Assumed || dim.LowerExpr != nil || dim.UpperExpr != nil})
		}

		n++
		c := l.newCell("$B"+strconv.Itoa(n), DescriptorCell, Long, d.Size())
		c.Desc, c.Dummy = d, s.Name

		if l.adjustable == nil {
			l.adjustable = map[*sema.Symbol]*Cell{}
		}

		l.adjustable[s] = c
		l.adjustableOrder = append(l.adjustableOrder, s)
	}
}

// defines reports whether an entry point whose list is params defines the
// adjustable array s: s itself and every dummy argument in its bounds are there.
func defines(s *sema.Symbol, params []*sema.Symbol) bool {
	if !slices.Contains(params, s) {
		return false
	}

	for _, p := range s.BoundDummies {
		if !slices.Contains(params, p) {
			return false
		}
	}

	return true
}

// computeBounds writes the code that fills in the descriptors of the adjustable
// arrays an entry point whose list is params defines.
func (l *lowerer) computeBounds(params []*sema.Symbol) {
	for _, s := range l.adjustableOrder {
		if defines(s, params) {
			l.computeArrayBounds(s, l.adjustable[s])
		}
	}
}

// computeArrayBounds fills in the descriptor c of the adjustable array s.
func (l *lowerer) computeArrayBounds(s *sema.Symbol, c *Cell) {
	store := func(offset int, t Type, x Expr) {
		l.emit(&Assign{l.origin(), &Mem{Cell: c, Offset: offset, T: t}, x})
	}

	field := func(offset int) Expr {
		return &Load{&Mem{Cell: c, Offset: offset, T: Long}}
	}

	d := c.Desc

	// A passed-length array's elements are as long as its descriptor says.
	if s.Type.Size == 0 {
		store(0, Word, &Load{descriptorField(l.params[s], 0, Word)})
	}

	base := l.symbolMem(s)
	store(PointerOffset, Long, &Addr{base})

	size := Expr(LongConst(int32(s.Type.Size)))
	if s.Type.Kind == sema.Character {
		size = l.charLength(s)
	}

	for i, dim := range s.Dims {
		lower := Expr(LongConst(int32(dim.Lower)))
		upper := Expr(LongConst(int32(dim.Upper)))

		if dim.LowerValue != nil {
			store(d.LowerOffset(i), Long, l.expr(dim.LowerValue))
			lower = field(d.LowerOffset(i))
		}

		if dim.UpperValue != nil {
			store(d.LowerOffset(i)+Long.Size(), Long, l.expr(dim.UpperValue))
			upper = field(d.LowerOffset(i) + Long.Size())
		}

		if !dim.Assumed && d.Dims[i].Set {
			store(d.MultiplierOffset(i), Long, arith(Add, Long, arith(Sub, Long, upper, lower), LongConst(1)))
		}
	}

	// A0 is the first element's address, less the element number the lowest
	// subscripts would have counting from all zeros.
	first := l.lowerBound(s, c, len(s.Dims)-1)

	for i := len(s.Dims) - 2; i >= 0; i-- {
		first = arith(Add, Long, arith(Mul, Long, first, l.extent(s, c, i)), l.lowerBound(s, c, i))
	}

	store(A0Offset, Long, &Addr{l.elementAt(base, s, negated(first))})

	if !s.AssumedSize() {
		count := size

		for i := range s.Dims {
			count = arith(Mul, Long, count, l.extent(s, c, i))
		}

		store(ArsizeOffset, Long, count)
	}

	l.release()
}

// negated is -e, its constant part kept apart so that it can become an offset.
func negated(e Expr) Expr {
	v, k := split(e)
	if v == nil {
		return LongConst(-k)
	}

	return arith(Add, Long, arith(Neg, Long, v, nil), LongConst(-k))
}

// extent is dimension i's extent: a constant, or its descriptor's multiplier.
func (l *lowerer) extent(s *sema.Symbol, c *Cell, i int) Expr {
	if dim := s.Dims[i]; !c.Desc.Dims[i].Set {
		return LongConst(int32(dim.Extent()))
	}

	return &Load{&Mem{Cell: c, Offset: c.Desc.MultiplierOffset(i), T: Long}}
}

// lowerBound is dimension i's lower bound: a constant, or its descriptor's.
func (l *lowerer) lowerBound(s *sema.Symbol, c *Cell, i int) Expr {
	if dim := s.Dims[i]; dim.LowerExpr == nil {
		return LongConst(int32(dim.Lower))
	}

	return &Load{&Mem{Cell: c, Offset: c.Desc.LowerOffset(i), T: Long}}
}

// elementAt is the place index elements past base in the array s. The constant
// part of index becomes the place's offset, unless a passed-length array's
// element size isn't known; a CHARACTER place is a Byte place, whose index the
// VAX doesn't multiply by the element's length, so that's done here.
func (l *lowerer) elementAt(base *Mem, s *sema.Symbol, index Expr) *Mem {
	m := *base

	if c, ok := index.(*Const); ok && c.Int() == 0 {
		return &m
	}

	if s.Type.Size == 0 {
		m.Index = arith(Mul, Long, index, l.charLength(s))

		return &m
	}

	v, k := split(index)
	m.Offset += int(k) * s.Type.Size

	if v != nil && s.Type.Kind == sema.Character {
		v = arith(Mul, Long, v, l.charLength(s))
	}

	m.Index = v

	return &m
}

// adjustableElement is where the element e of an adjustable array is: A0 plus
// its subscripts' sum, each multiplied by the extents before it, written from
// the last so that it takes one multiplication a dimension.
func (l *lowerer) adjustableElement(e *sema.Elem, c *Cell) *Mem {
	subs := make([]Expr, len(e.Subs))
	for i, sub := range e.Subs {
		subs[i] = l.expr(sub)
	}

	index := subs[len(subs)-1]

	for i := len(subs) - 2; i >= 0; i-- {
		index = arith(Add, Long, arith(Mul, Long, index, l.extent(e.Sym, c, i)), subs[i])
	}

	a0 := &Mem{
		Ptr: &Load{&Mem{Cell: c, Offset: A0Offset, T: Long}},
		T:   l.typeOf(e.Sym.Type),
	}

	return l.elementAt(a0, e.Sym, index)
}

// arraySize is an adjustable array's size in bytes, for SIZEOF.
func (l *lowerer) arraySize(s *sema.Symbol) Expr {
	return &Load{&Mem{Cell: l.adjustable[s], Offset: ArsizeOffset, T: Long}}
}

// layOutEntryBounds lays out, after the routine's last statement, the block for
// each ENTRY statement whose entry point defines an adjustable array. It can't
// go where the statement is, since control from the statements above falls into
// the statements after an ENTRY statement but doesn't enter the routine (section
// 6.2.4). The entry point goes to this block, which goes on to the statements.
func (l *lowerer) layOutEntryBounds() {
	for _, e := range l.unit.Entries {
		if !slices.ContainsFunc(l.adjustableOrder, func(s *sema.Symbol) bool { return defines(s, e.Params) }) {
			continue
		}

		entry := l.entries[e]
		body := entry.First

		b := l.newBlock()
		b.Note = e.Sym.Name + "'s adjustable arrays' bounds"
		l.begin(b)
		l.computeBounds(e.Params)
		l.jump(body)
		entry.First = b
	}
}
