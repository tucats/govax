package sema

// The optimizer's aliasing rule for pointers (docs/planning/PHASE-107 - Fortran 90
// pointers.md, finding 11). A pointer's target must have its type: an INTEGER
// pointer points at an INTEGER, never at a REAL or at the bytes of a CHARACTER
// variable. So a store through a pointer can change only storage holding a value
// of its type, by itself or as a part of something larger: an INTEGER pointer may
// point at a NODE's VALUE, and a NODE pointer's store changes every component of
// the NODE it points at.
//
// That sorts types into classes for the question. An intrinsic type is one class
// by its kind and kind number (INTEGER*2 and INTEGER*4 are two). Every CHARACTER
// length is one class, since a CHARACTER pointer may point at a substring of a
// longer variable. A derived type is one class by sameType, so that an interface
// body's SEQUENCE copy of a type is the type it copies.

// PointersMayMeet reports whether a value of type t, reached through a pointer,
// and a value of type u, reached through a pointer or held by a TARGET, may share
// storage: t and u are of one class, or one is a record that ultimately holds a
// component of the other's class. A pointer component is passed over, since a
// record holds only an address there, not a value of its type.
func PointersMayMeet(t, u Type) bool {
	return mayHold(t, u, nil) || mayHold(u, t, nil)
}

// mayHold reports whether storage holding a value of type t may hold a value of
// type u as itself or as a part. seen holds the structures being walked, which
// a structure can't contain again except through a pointer, but the guard costs
// nothing.
func mayHold(t, u Type, seen map[*Structure]bool) bool {
	if sameClass(t, u) {
		return true
	}

	if t.Kind != Record || t.Struct == nil || seen[t.Struct] {
		return false
	}

	if seen == nil {
		seen = map[*Structure]bool{}
	}

	seen[t.Struct] = true

	for _, f := range t.Struct.Fields {
		if f.Pointer {
			continue
		}

		ft := f.Type
		if f.Struct != nil {
			ft = recordType(f.Struct)
		}

		if mayHold(ft, u, seen) {
			return true
		}
	}

	return false
}

// sameClass reports whether two types are one class for the rule: any two
// CHARACTER lengths, two records sameType says are one (or of a structure not
// known, conservatively), and otherwise equal types.
func sameClass(t, u Type) bool {
	switch {
	case t.Kind == Character || u.Kind == Character:
		return t.Kind == u.Kind
	case t.Kind == Record && u.Kind == Record:
		return t.Struct == nil || u.Struct == nil || sameType(t.Struct, u.Struct)
	}

	return t == u
}

// FieldTargetType is the type of what the pointer component f points at: its
// target's structure's record type, or its Type.
func FieldTargetType(f *Field) Type {
	if f.Struct != nil {
		return recordType(f.Struct)
	}

	return f.Type
}
