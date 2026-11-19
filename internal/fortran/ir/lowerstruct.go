package ir

import "github.com/tucats/govax/internal/fortran/sema"

// Structure constructors (sema/structvalue.go): POINT(X, 2.0), a value of a
// derived type. One whose values are all constant is a sema.Const whose bytes
// are a datum in $PDATA, moved or passed as any record is (mem). Any other is
// built a component at a time: a store of each scalar component's value at the
// component's offset, a MOVC3 or MOVC5 of a CHARACTER one, a constant array
// component's bytes moved from a datum, and a component of a derived type
// built in place if its value is a constructor too, else moved, and a pointer
// component given its target's address (pointer assignment). Where it is
// built is the assignment's target if none of the values reads it
// (Assign.InPlace), else a temporary of the type's size.

// buildRecord builds the constructor v's value at dst, a record's place.
func (l *lowerer) buildRecord(dst *Mem, v *sema.StructValue) {
	base := l.recordPlace(dst)

	for i, f := range v.T.Struct.Fields {
		place := *base
		place.Offset += f.Offset
		value := v.Values[i]

		switch {
		case f.Pointer && f.Rank > 0:
			// An array pointer component's descriptor is filled in where it
			// is in the record, as pointer assignment fills a named one's.
			p := sema.NewComponentPointer(f)
			l.associate(l.locatedCell(f.Name, l.ncaDescriptor(p), &place), p, value)

		case f.Pointer:
			// Pointer assignment of the component (7.5.1.5): its target's
			// address, and a CHARACTER one's length and type before it.
			if f.Struct == nil && charPointer(f.Type) {
				header := place
				header.T = Long
				l.emit(&Assign{origin: l.origin(), Dst: &header, Src: charPointerHeader(f.Type.Size)})
				place.Offset += PointerOffset
			}

			place.T = Long
			l.emit(&Assign{origin: l.origin(), Dst: &place, Src: l.targetAddress(value)})

		case v.Bytes[i] != nil:
			n := LongConst(int32(len(v.Bytes[i])))
			l.emit(&CharMove{l.origin(), Text{&place, n}, Text{l.text(string(v.Bytes[i])), n}})

		case f.Dims != nil:
			l.fillComponent(&place, f, value)

		case f.Struct != nil:
			if inner, ok := value.(*sema.StructValue); ok {
				l.buildRecord(&place, inner)

				continue
			}

			l.moveBytes(&place, l.mem(value), f.Struct.Size)

		case f.Type.Kind == sema.Character:
			l.emit(&CharMove{l.origin(), Text{&place, LongConst(int32(f.Type.Size))}, l.chars(value)})

		default:
			place.T = l.typeOf(f.Type)
			l.store(&place, value)
		}
	}
}

// fillComponent assigns value, an array of the array field f's shape or a
// scalar for every element, to the field at place, as an array assignment to an
// array of lowering's own whose storage is the field's: the scalarizer's loops.
// The value reads nothing of the record being built (sema's InPlace, or a
// temporary), so they need no order.
func (l *lowerer) fillComponent(place *Mem, f *sema.Field, value sema.Expr) {
	extents := make([]int, len(f.Dims))
	for i, d := range f.Dims {
		extents[i] = d.Extent()
	}

	t := value.Type()
	if f.Struct == nil {
		t = f.Type
	}

	at := l.temp(Long)
	l.emit(&Assign{origin: l.origin(), Dst: at, Src: &Addr{place}})

	sym := sema.NewArray("$P", t, extents)
	l.bound[sym] = &Mem{Ptr: &Load{at}, T: l.typeOf(t)}

	l.arrayNest(sema.NewArrayRef(sym, value.Position()), value, nil, nil)

	delete(l.bound, sym)
}

// recordPlace is m, a record's place, as a Byte place whose Offset counts
// bytes from the record's first and which has no index, so that a component's
// place is it with the component's offset added: a dummy's through its
// argument's longword, an element of an array whose subscripts aren't constant
// through its address, computed once into a temporary.
func (l *lowerer) recordPlace(m *Mem) *Mem {
	switch {
	case m.Index != nil || m.Param != nil && m.Offset != 0:
		t := l.temp(Long)
		l.emit(&Assign{origin: l.origin(), Dst: t, Src: &Addr{m}})

		return &Mem{Ptr: &Load{t}, T: Byte, Volatile: m.Volatile}

	case m.Param != nil:
		return &Mem{Ptr: &ArgPointer{m.Param}, T: Byte, Volatile: m.Volatile}
	}

	place := *m
	place.T = Byte

	return &place
}
