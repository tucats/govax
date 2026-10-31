package ir

import (
	"strconv"

	"github.com/tucats/govax/internal/fortran/sema"
)

// CHARACTER dummy arguments, character arguments, and concatenation
// (Decisions 2 and 8 of docs/PHASE-78 - FORTRAN expressions and types.md).
//
// A CHARACTER argument is passed by descriptor (Decision 11 of
// docs/PHASE-74 - FORTRAN plan.md): the argument list's longword holds the
// address of a class S descriptor, whose first word is the value's length
// and whose second longword (DSC$A_POINTER) its address. So a CHARACTER
// dummy's text is two steps from the argument list,
//
//	MOVL    SUB.A(AP),R0          the descriptor's address
//	MOVL    4(R0),R0              the text's
//
// and a passed-length dummy's (CHARACTER*(*)) length is the descriptor's
// first word, MOVZWL @SUB.A(AP),R1. A dummy declared with a length has that
// length, which section 6.1.1 says the actual argument's must be at least.

// The LIBRTL routines a concatenation of unknown length uses.
const (
	libGetString  = "LIB$SGET1_DD"
	libFreeString = "LIB$SFREE1_DD"
)

// charParam is the dummy argument s is, if s is a CHARACTER dummy argument.
func (l *lowerer) charParam(s *sema.Symbol) *Param {
	if s.Type.Kind != sema.Character {
		return nil
	}

	return l.params[s]
}

// descriptorField is a field of the descriptor a CHARACTER dummy argument's
// argument points at: the length (offset 0, a Word) or the address (at
// PointerOffset, a Long).
func descriptorField(p *Param, offset int, t Type) *Mem {
	return &Mem{Ptr: &ArgPointer{p}, Offset: offset, T: t}
}

// dummyText is where a CHARACTER dummy argument's text is: at the address
// its descriptor holds.
func dummyText(p *Param) *Mem {
	return &Mem{Ptr: &Load{descriptorField(p, PointerOffset, Long)}, T: Byte}
}

// charLength is the length of a CHARACTER variable or of each element of a
// CHARACTER array: its declared length, or for a passed-length dummy
// argument its descriptor's, an unsigned word made a longword.
func (l *lowerer) charLength(s *sema.Symbol) Expr {
	if s.Type.Size > 0 {
		return LongConst(int32(s.Type.Size))
	}

	return &Op{Zext, Long, &Load{descriptorField(l.params[s], 0, Word)}, nil}
}

// maxLength is the most characters a CHARACTER value can have, and whether
// that is known when compiling: a constant's, a variable's, an element's,
// a substring's when its bounds are constant or else its variable's, a
// concatenation's its parts' sum. A passed-length dummy's isn't.
func maxLength(e sema.Expr) (int, bool) {
	switch e := e.(type) {
	case *sema.Substring:
		if e.T.Size > 0 {
			return e.T.Size, true
		}

		return maxLength(e.X)

	case *sema.Concat:
		n := 0

		for _, p := range e.Parts {
			k, ok := maxLength(p)
			if !ok {
				return 0, false
			}

			n += k
		}

		return n, true
	}

	n := e.Type().Size

	return n, n > 0
}

// concat lowers a concatenation (section 2.3.2): each part moved into a
// temporary after the one before, its value the temporary and the parts'
// lengths' sum (Decision 8). The temporary is a cell as long as the
// concatenation can be, or, when that isn't known (a passed-length part), a
// dynamic string of the length it is.
func (l *lowerer) concat(e *sema.Concat) Text {
	parts := make([]Text, len(e.Parts))
	for i, p := range e.Parts {
		parts[i] = l.chars(p)
	}

	n, known := maxLength(e)

	return l.build(parts, n, known)
}

// build moves the CHARACTER values parts one after another into a
// temporary, and returns it: a static cell of n bytes when known, else a
// dynamic string. Each part's place in it is the lengths before it on from
// its start, the constant part of that in the offset and the rest in the
// index.
func (l *lowerer) build(parts []Text, n int, known bool) Text {
	var total Expr
	for _, p := range parts {
		total = arith(Add, Long, total, p.Len)
	}

	var place *Mem
	if known {
		place = l.textTemp(n)
	} else {
		place = l.dynamicText(total)
	}

	at := place

	for _, p := range parts {
		l.emit(&CharMove{l.origin(), Text{at, p.Len}, p})

		v, k := split(p.Len)
		next := *at
		next.Offset += int(k)
		next.Index = arith(Add, Long, next.Index, v)
		at = &next
	}

	return Text{place, total}
}

// copyText is a copy of the CHARACTER value t, whose most characters are
// e's: what a character argument in parentheses passes.
func (l *lowerer) copyText(e sema.Expr, t Text) Text {
	n, known := maxLength(e)

	return l.build([]Text{t}, n, known)
}

// textTemp is a CHARACTER temporary of n bytes for the statement being
// lowered: one another statement used and gave back, or a new cell.
func (l *lowerer) textTemp(n int) *Mem {
	var c *Cell

	if free := l.freeText[n]; len(free) > 0 {
		c = free[len(free)-1]
		l.freeText[n] = free[:len(free)-1]
	} else {
		l.temps++
		c = l.newCell("$T"+strconv.Itoa(l.temps), TextCell, Byte, n)
	}

	l.busy = append(l.busy, c)

	return &Mem{Cell: c, T: Byte}
}

// dynamicText is the place of a dynamic string of length characters for
// the statement being lowered: a class D descriptor cell, which
// LIB$SGET1_DD gives the text, and which LIB$SFREE1_DD frees at the
// statement's end (after). The length is a word, as LIB$SGET1_DD reads
// it.
func (l *lowerer) dynamicText(length Expr) *Mem {
	c := l.descriptorCell(&Descriptor{DType: DTypeT, Class: ClassD})
	desc := &Mem{Cell: c, T: Long}

	n := l.temp(Word)
	l.emit(&Assign{l.origin(), n, arith(Cvt, Word, length, nil)})
	l.emit(&Call{origin: l.origin(), Routine: libGetString, Args: []Arg{{Ref: n}, {Ref: desc}}})
	l.after = append(l.after, &Call{origin: l.origin(), Routine: libFreeString, Args: []Arg{{Ref: desc}}})

	return &Mem{Ptr: &Load{&Mem{Cell: c, Offset: PointerOffset, T: Long}}, T: Byte}
}

// charArg is a CHARACTER actual argument: a descriptor of it, by reference.
// A passed-length dummy, or a whole array that is one, is passed on with
// the descriptor it came with; any other array is described as an array
// (class A), but an assumed-size one, whose size isn't known, by its first
// element; a parenthesized variable, element, or substring is copied first.
func (l *lowerer) charArg(a sema.Arg) Arg {
	d := &Descriptor{DType: DTypeT, Class: ClassS}

	switch e := a.Expr.(type) {
	case *sema.Const:
		return Arg{Ref: l.textDescriptor(e.Text)}

	case *sema.Var:
		if p := l.charParam(e.Sym); p != nil && e.Sym.Type.Size == 0 && !a.Copy {
			return Arg{Value: &ArgPointer{p}}
		}

	case *sema.ArrayRef:
		if p := l.charParam(e.Sym); p != nil && e.Sym.Type.Size == 0 {
			return Arg{Value: &ArgPointer{p}}
		}

		if e.Sym.AssumedSize() {
			return Arg{Ref: l.describeText(d, Text{l.symbolMem(e.Sym), l.charLength(e.Sym)})}
		}

		return Arg{Ref: l.arrayDescriptor(e.Sym)}
	}

	t := l.chars(a.Expr)
	if a.Copy {
		t = l.copyText(a.Expr, t)
	}

	return Arg{Ref: l.describeText(d, t)}
}

// arrayDescriptor is a class A descriptor of the whole array s, for an
// output list or an argument. A passed-length array's element length, size,
// and A0 are known only when the program runs, so the statement stores them,
// with the address, in a descriptor cell. An adjustable array has a descriptor
// cell of its own, which its entry points fill in (lowerbounds.go).
func (l *lowerer) arrayDescriptor(s *sema.Symbol) *Mem {
	if c := l.adjustable[s]; c != nil {
		return &Mem{Cell: c, T: Long}
	}

	d := &Descriptor{DType: l.dtypeOf(s.Type), Class: ClassA, Length: s.Type.Size}

	for _, dim := range s.Dims {
		d.Dims = append(d.Dims, Bound{Lower: dim.Lower, Upper: dim.Upper})
	}

	m := l.symbolMem(s)

	p := l.charParam(s)
	if p == nil || s.Type.Size > 0 {
		return l.describe(d, m)
	}

	c := l.descriptorCell(d)
	n := l.charLength(s)

	a0 := *m
	if first := firstElement(d); first != 0 {
		a0.Index = arith(Mul, Long, LongConst(int32(-first)), n)
	}

	l.emit(&Assign{l.origin(), &Mem{Cell: c, T: Word}, &Load{descriptorField(p, 0, Word)}})
	l.emit(&Assign{l.origin(), &Mem{Cell: c, Offset: PointerOffset, T: Long}, &Addr{m}})
	l.emit(&Assign{l.origin(), &Mem{Cell: c, Offset: ArsizeOffset, T: Long},
		arith(Mul, Long, n, LongConst(int32(d.Elements())))})
	l.emit(&Assign{l.origin(), &Mem{Cell: c, Offset: A0Offset, T: Long}, &Addr{&a0}})

	return &Mem{Cell: c, T: Long}
}

// firstElement is the number of elements an array's first element is past
// the one whose subscripts are all zero (where A0 points).
func firstElement(d *Descriptor) int {
	first, multiplier := 0, 1

	for _, b := range d.Dims {
		first += b.Lower * multiplier
		multiplier *= b.Upper - b.Lower + 1
	}

	return first
}
