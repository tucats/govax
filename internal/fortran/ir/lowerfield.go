package ir

import "github.com/tucats/govax/internal/fortran/sema"

// fieldMem is where a scalar field reference's field is (section 2.2.5.2):
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
// is a count of bytes already.
func (l *lowerer) fieldMem(e *sema.FieldRef) *Mem {
	base := l.symbolMem(e.Sym)
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

	element(e.Subs, e.Sym.Dims, e.Sym.Struct.Size)

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

	return &Mem{Ptr: &Addr{&m}, T: t}
}
