package ir

import "github.com/tucats/govax/internal/fortran/sema"

// TRANSFER(SOURCE, MOLD, SIZE) (sema's transfer.go; docs/planning/PHASE-108 -
// Fortran 90 intrinsic procedures.md, finding 8): SOURCE's bytes, taken as
// MOLD's type. Its value is made in a temporary of the result's size, SOURCE's
// bytes moved in and the rest of it cleared, and then read as MOLD's type.
//
// A scalar SOURCE that is a number or a LOGICAL is stored in the temporary as
// itself (a REAL or COMPLEX variable as an integer of its size, storeSource),
// which is all of the move; and a scalar TRANSFER assigned to a REAL variable
// is moved as an integer too (rawMove):
//
//	X = TRANSFER(I, X)             ! INTEGER I, REAL X
//
//	MOVL    I,$T1                  ; SOURCE, as an INTEGER
//	MOVL    $T1,X                  ; MOLD's REAL, moved as a longword
//
// so bits that are a reserved operand in a floating format pass through
// without the fault MOVF would take on them.
//
// Any other SOURCE (a CHARACTER value, a record, an array) is bytes in memory:
// moved by MOVC3 and what is past them cleared (CLRL and the like), when both
// lengths are constants of a word's length, else by LIBRTL's OTS$MOVE5, whose
// lengths are longwords, with a fill of 0. An array SOURCE is moved from where
// its elements are if they are one after another, else from a temporary they
// are copied into first (lowersequence.go's sequence). An array result is a
// temporary of the statement's, like the other transformational functions'
// (lowertransform.go), of SIZE elements, or as many as SOURCE's bytes need;
// the statement's loops read its elements as their type, so a REAL one that is
// a reserved operand faults there, as any REAL value with those bits would.

// otsMove5 is LIBRTL's OTS$MOVE5 (OTS$ manual, OTS-51): MOVC5 with longword
// lengths, its arguments the source length and the fill byte by value.
const otsMove5 = "OTS$MOVE5"

// transferPlace is the scalar TRANSFER e's value, in a temporary of the
// statement's: a place of MOLD's type, or of Bytes for a CHARACTER value or a
// record.
func (l *lowerer) transferPlace(e *sema.IntrinsicRef) *Mem {
	source, size := e.Args[0], e.T.Size

	var place *Mem

	if st := source.Type(); len(source.Shape()) == 0 && st.Kind != sema.Character && st.Kind != sema.Record {
		// A number or a LOGICAL: stored as itself, the rest cleared.
		place = l.textTemp(max(st.Size, size))
		if size > st.Size {
			l.clearBytes(offsetBy(place, st.Size), size-st.Size)
		}

		l.storeSource(place, source)
	} else {
		place = l.textTemp(size)
		l.moveFilled(Text{place, LongConst(int32(size))}, l.sourceText(source))
	}

	v := *place
	v.T = l.typeOf(e.T)

	return &v
}

// transferText is the scalar TRANSFER e of a CHARACTER MOLD whose length is
// known only when the program runs, or is zero (Phase 113): a dynamic string as
// long as MOLD, e's second argument, SOURCE's bytes moved in and the rest
// cleared.
func (l *lowerer) transferText(e *sema.IntrinsicRef) Text {
	n := l.keepLong(l.chars(e.Args[1]).Len)
	place := Text{l.dynamicText(&Load{n}), &Load{n}}
	l.moveFilled(place, l.sourceText(e.Args[0]))

	return place
}

// transferArray computes the array TRANSFER f's value into a temporary of the
// statement's, of SIZE elements (never below 0), or as many as hold SOURCE's
// bytes.
func (l *lowerer) transferArray(f *sema.ArrayFunc) *sema.ArrayRef {
	var m, length Expr = LongConst(int32(f.T.Size)), nil

	if f.Pad != nil {
		// A CHARACTER MOLD whose elements' length is known only now (sema's
		// Pad): that length, from MOLD itself.
		length = &Load{l.keepLong(l.moldLength(f.Pad))}
		m = length
	}

	src := l.sourceText(f.Array)

	var n Expr

	if f.Other != nil {
		n = l.expr(f.Other)

		if c, ok := n.(*Const); ok {
			n = LongConst(max(c.Int(), 0))
		} else {
			k := l.keepLong(n)
			l.notNegative(k)
			n = &Load{k}
		}
	} else if length != nil {
		// As many as hold SOURCE's bytes; none if MOLD's elements have no
		// length (ours: the standard's rule can't be met).
		k := l.keepLong(LongConst(0))
		some := l.newBlock()
		next := l.newBlock()
		l.branch(&Cmp{Gt, length, LongConst(0)}, some, next)
		l.begin(some)
		l.emit(&Assign{origin: l.origin(), Dst: k,
			Src: arith(Div, Long, arith(Add, Long, src.Len, arith(Sub, Long, m, LongConst(1))), m)})
		l.jump(next)
		l.begin(next)

		n = &Load{k}
	} else {
		n = arith(Div, Long, arith(Add, Long, src.Len, arith(Sub, Long, m, LongConst(1))), m)
		if _, ok := n.(*Const); !ok {
			n = &Load{l.keepLong(n)}
		}
	}

	temp := l.extentsTempOf(f.T, []Expr{n}, length)

	base := *l.symbolMem(temp.Sym)
	base.T = Byte

	l.moveFilled(Text{&base, arith(Mul, Long, n, m)}, src)

	return temp
}

// moldLength is the length of the CHARACTER MOLD's elements: a scalar's
// length, or an array's (a name's, as valueLength finds it).
func (l *lowerer) moldLength(mold sema.Expr) Expr {
	if c, ok := mold.(*sema.Constructor); ok && c.LengthOf != nil {
		return l.moldLength(c.LengthOf)
	}

	if len(mold.Shape()) > 0 {
		return l.valueLength(mold)
	}

	return l.chars(mold).Len
}

// sourceText is TRANSFER's SOURCE as bytes in memory: where they are, and how
// many. A scalar number or LOGICAL is stored in a temporary first; a CHARACTER
// value is its text; a record its bytes; an array its elements one after
// another, all of them.
func (l *lowerer) sourceText(e sema.Expr) Text {
	t := e.Type()

	if len(e.Shape()) == 0 {
		switch t.Kind {
		case sema.Character:
			return l.chars(e)

		case sema.Record:
			m := *l.mem(e)
			m.T = Byte

			return Text{&m, LongConst(int32(t.Size))}
		}

		place := l.textTemp(t.Size)
		l.storeSource(place, e)

		return Text{place, LongConst(int32(t.Size))}
	}

	// The elements' size, and their number.
	var size Expr = LongConst(int32(t.Size))
	if t.Kind == sema.Character && t.Size == 0 {
		size = l.valueLength(e)
	}

	n := arith(Mul, Long, size, product(l.extentsOf(e.Shape())))
	if _, ok := n.(*Const); !ok {
		n = &Load{l.keepLong(n)}
	}

	// A whole array whose elements are one after another is where it is.
	if x, ok := e.(*sema.ArrayRef); ok && !x.Sym.Strided() {
		m := *l.symbolMem(x.Sym)
		m.T = Byte

		return Text{&m, n}
	}

	_, whole := e.(*sema.ArrayRef)
	_, section := e.(*sema.Section)
	at := l.keepLong(l.sequence(e, whole || section, false, false).value)

	return Text{&Mem{Ptr: &Load{at}, T: Byte}, n}
}

// storeSource stores the scalar e, a number or a LOGICAL, at place, a Byte
// place as long as it: as itself, but a REAL or COMPLEX variable, element, or
// field moved as an integer of its size, since its bits may be a reserved
// operand (another TRANSFER's) that a floating move would fault on.
func (l *lowerer) storeSource(place *Mem, e sema.Expr) {
	t := e.Type()

	switch e.(type) {
	case *sema.Var, *sema.Elem, *sema.FieldRef:
		if t.Kind == sema.Real || t.Kind == sema.Complex {
			src := *l.mem(e)
			src.T = Byte

			if t.Size == 32 {
				l.moveBytes(place, &src, t.Size)

				return
			}

			d := *place
			src.T, d.T = rawType(t.Size), rawType(t.Size)
			l.emit(&Assign{origin: l.origin(), Dst: &d, Src: &Load{&src}})

			return
		}
	}

	v := *place
	v.T = l.typeOf(t)
	l.store(&v, e)
}

// rawType is the integer type a value of size bytes, 4, 8, or 16, is moved as.
func rawType(size int) Type {
	switch size {
	case 4:
		return Long
	case 8:
		return Quad
	}

	return Octa
}

// moveFilled moves the bytes src to dst, and clears what of dst is past them:
// MOVC3 if src is at least as long, or MOVC3 and the rest cleared, when both
// lengths are constants of a word's length; otherwise OTS$MOVE5, with a fill
// of 0.
func (l *lowerer) moveFilled(dst, src Text) {
	dl, dOK := dst.Len.(*Const)
	sl, sOK := src.Len.(*Const)

	if dOK && sOK && dl.Int() <= maxWordLength && sl.Int() <= maxWordLength {
		d, s := int(dl.Int()), int(sl.Int())

		switch {
		case d == 0:
		case s >= d:
			l.emit(&CharMove{l.origin(), dst, Text{src.Mem, dst.Len}})
		default:
			if s > 0 {
				l.emit(&CharMove{l.origin(), Text{dst.Mem, src.Len}, src})
			}

			l.clearBytes(offsetBy(dst.Mem, s), d-s)
		}

		return
	}

	l.emit(&Call{origin: l.origin(), Routine: otsMove5, Args: []Arg{
		{Value: src.Len}, {Ref: src.Mem}, {Value: LongConst(0)}, {Value: dst.Len}, {Ref: dst.Mem},
	}})
}

// clearBytes clears n bytes at m, a Byte place: by CLRO, CLRQ, CLRL, CLRW, and
// CLRB for a few, by OTS$MOVE5 of nothing for more.
func (l *lowerer) clearBytes(m *Mem, n int) {
	const most = 64

	if n > most || m.Index != nil {
		l.emit(&Call{origin: l.origin(), Routine: otsMove5, Args: []Arg{
			{Value: LongConst(0)}, {Ref: m}, {Value: LongConst(0)}, {Value: LongConst(int32(n))}, {Ref: m},
		}})

		return
	}

	for done := 0; done < n; {
		var t Type

		switch left := n - done; {
		case left >= 16:
			t = Octa
		case left >= 8:
			t = Quad
		case left >= 4:
			t = Long
		case left >= 2:
			t = Word
		default:
			t = Byte
		}

		p := offsetBy(m, done)
		p.T = t
		l.emit(&Assign{origin: l.origin(), Dst: p, Src: &Const{T: t}})

		done += t.Size()
	}
}

// offsetBy is the place n bytes past m.
func offsetBy(m *Mem, n int) *Mem {
	p := *m
	p.Offset += n

	return &p
}

// rawMove stores the scalar TRANSFER e at dst, a place of e's floating type,
// as an integer of its size, so that bits that are a reserved operand in that
// format don't fault (MOVL, MOVQ, MOVO where MOVF, MOVD, MOVH would).
func (l *lowerer) rawMove(dst *Mem, e *sema.IntrinsicRef) {
	src := *l.transferPlace(e)
	d := *dst

	src.T, d.T = rawType(dst.T.Size()), rawType(dst.T.Size())

	l.emit(&Assign{origin: l.origin(), Dst: &d, Src: &Load{&src}})
}
