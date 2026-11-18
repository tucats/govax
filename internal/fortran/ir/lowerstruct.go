package ir

import "github.com/tucats/govax/internal/fortran/sema"

// Structure constructors (sema/structvalue.go): POINT(X, 2.0), a value of a
// derived type. One whose values are all constant is a sema.Const whose bytes
// are a datum in $PDATA, moved or passed as any record is (mem). Any other is
// built a component at a time: a store of each scalar component's value at the
// component's offset, a MOVC3 or MOVC5 of a CHARACTER one, a constant array
// component's bytes moved from a datum, and a component of a derived type
// built in place if its value is a constructor too, else moved. Where it is
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
		case v.Bytes[i] != nil:
			n := LongConst(int32(len(v.Bytes[i])))
			l.emit(&CharMove{l.origin(), Text{&place, n}, Text{l.text(string(v.Bytes[i])), n}})

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
