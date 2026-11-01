package ir

import "github.com/tucats/govax/internal/fortran/sema"

// maxWordLength is the most bytes a word can say: a descriptor's length, or
// the length MOVC3 takes.
const maxWordLength = 65535

// fieldMem is where a field reference's field is (section 2.2.5.2):
// the record's place, plus the offset of each field on the way down in its
// structure, plus, for each part with subscripts (an element of an array of
// records, an element of an array field), that element's offset, computed as
// an array element's is (mem) with the element's length as its size. So
// HISTORY(I).YEAR, with RECORD /DATE/ HISTORY(10) and YEAR at +2 in DATE's 4
// bytes, is at HISTORY + 2 + (I-1)*4.
//
// The constant parts of that are the place's offset. What isn't constant is a
// count of bytes, which the VAX's indexed mode can take only as a count of the
// field's own size, which it multiplies by: HISTORY+2-4[I] as a word, since
// 4*(I-1) bytes is 2*(I-1) words. A structure is packed (structure.go), so a
// length needn't be a multiple of a field's size; a place where one isn't is
// reached through its address, computed with the bytes as the index
// (MOVAB HISTORY+1-5[R0],R1, then (R1)): Decision 2 of docs/PHASE-82 - FORTRAN
// DEC extensions.md. A CHARACTER field's place is a Byte place, whose index
// is a count of bytes already, as is a substructure's, which is bytes itself.
//
// A record that is a dummy argument is reached through its address. A dummy's
// place in the ordinary form says an offset as a count of its elements, which a
// field's offset needn't be, so the place here is a computed address too.
//
// The element of an adjustable array of records is at its A0, and the extents
// of its dimensions are in its descriptor (lowerbounds.go): its subscripts, each
// multiplied by the extents before it, are one count of records.
func (l *lowerer) fieldMem(e *sema.FieldRef) *Mem {
	base := l.symbolMem(e.Sym)
	if base.Param != nil {
		base = &Mem{Ptr: &ArgPointer{base.Param}, T: base.T, Volatile: base.Volatile}
	}

	offset := base.Offset

	// parts are the subscripts' variable parts, each with the bytes one of it
	// moves the place.
	type part struct {
		v      Expr
		stride int
	}

	var parts []part

	element := func(subs []sema.Expr, dims []sema.Dim, size int) {
		multiplier := size

		for i, sub := range subs {
			v, k := split(l.expr(sub))
			offset += (int(k) - dims[i].Lower) * multiplier

			if v != nil {
				parts = append(parts, part{v, multiplier})
			}

			multiplier *= dims[i].Extent()
		}
	}

	if c := l.adjustable[e.Sym]; c != nil && e.Subs != nil {
		base = &Mem{Ptr: &Load{&Mem{Cell: c, Offset: A0Offset, T: Long}}, T: Byte, Volatile: e.Sym.Volatile}
		v, k := split(l.adjustableIndex(e.Subs, e.Sym, c))
		offset += int(k) * e.Sym.Struct.Size

		if v != nil {
			parts = append(parts, part{v, e.Sym.Struct.Size})
		}
	} else {
		element(e.Subs, e.Sym.Dims, e.Sym.Struct.Size)
	}

	for _, s := range e.Path {
		offset += s.Field.Offset
		element(s.Subs, s.Field.Dims, s.Field.ElementSize())
	}

	t := l.typeOf(e.T)
	unit := t.Size()

	if e.T.Kind == sema.Character {
		unit = 1
	}

	scaled := true

	for _, p := range parts {
		scaled = scaled && p.stride%unit == 0
	}

	if !scaled {
		unit = 1
	}

	var index Expr

	for _, p := range parts {
		index = arith(Add, Long, index, arith(Mul, Long, p.v, LongConst(int32(p.stride/unit))))
	}

	m := *base
	m.Offset, m.Index = offset, index

	if scaled {
		m.T = t

		return &m
	}

	m.T = Byte

	return &Mem{Ptr: &Addr{&m}, T: t, Volatile: m.Volatile}
}

// fieldArrayDescriptor is a class A descriptor of a whole array field (section
// 2.2.6's array name reference): an output or input list's item, an internal
// file, or a CHARACTER array field as an argument. Its elements' place and
// bounds are the field's.
func (l *lowerer) fieldArrayDescriptor(e *sema.FieldRef) *Mem {
	f := e.Field()
	d := &Descriptor{DType: l.dtypeOf(e.T), Class: ClassA, Length: e.T.Size}

	for _, dim := range f.Dims {
		d.Dims = append(d.Dims, Bound{Lower: dim.Lower, Upper: dim.Upper})
	}

	return l.describe(d, l.fieldMem(e))
}

// moveRecord lowers an aggregate assignment (section 3.4): the value's bytes
// moved to the target's, a CharMove of the record's length, which being one for
// both is a MOVC3. A MOVC3's length is a word, so a longer record goes in pieces,
// the two addresses computed once first, since a piece may change what a
// subscript depends on. A structure's union may let its two records overlap,
// which one MOVC3 allows for and pieces don't; that takes a record longer than
// 65,535 bytes overlapping itself, and is left undefined (ours).
func (l *lowerer) moveRecord(s *sema.Assign) {
	size := s.Target.Type().Size
	dst, src := l.mem(s.Target), l.mem(s.Value)

	if size <= maxWordLength {
		n := LongConst(int32(size))
		l.emit(&CharMove{l.origin(), Text{dst, n}, Text{src, n}})

		return
	}

	address := func(m *Mem) Expr {
		t := l.temp(Long)
		l.emit(&Assign{l.origin(), t, &Addr{m}})

		return &Load{t}
	}

	to, from := address(dst), address(src)

	for done := 0; done < size; done += maxWordLength {
		n := LongConst(int32(min(size-done, maxWordLength)))
		l.emit(&CharMove{
			l.origin(),
			Text{&Mem{Ptr: to, Offset: done, T: Byte}, n},
			Text{&Mem{Ptr: from, Offset: done, T: Byte}, n},
		})
	}
}
