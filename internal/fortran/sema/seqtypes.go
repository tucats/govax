package sema

import (
	"github.com/tucats/govax/internal/fortran/diag"
)

// Derived types in COMMON and EQUIVALENCE (docs/planning/PHASE-105 - Fortran 90
// derived types.md, finding 12). Both put an object's storage where another
// name can reach it, so both need the object's layout to be the program's to
// know: a SEQUENCE type's components are in storage in the order written (the
// standard's 4.4.1), and any other type's are the compiler's to arrange. So
// only an object of a SEQUENCE type may be in a common block (5.5.2) or an
// EQUIVALENCE list (5.5.1), NOSEQUENCE otherwise; and never a component alone,
// EQUIVCOMPONENT.
//
// EQUIVALENCE has a second rule, of type classes (5.5.1): a numeric sequence
// type (every ultimate component a number or a logical value of a default kind)
// and a character sequence type (every one CHARACTER) share storage with
// objects of the default numeric and CHARACTER types and with each other, and
// any other sequence type only with objects of its own type. The standard keeps
// numbers and characters apart; VAX FORTRAN lets them share storage, and so
// does govax here (the author's answer 6), so the two classes are one. A list
// with no derived-type object keeps VAX FORTRAN's rules, untouched.

// sequenceClass is how EQUIVALENCE sees an object's type.
type sequenceClass int

const (
	// defaultClass is a number, a logical value, or CHARACTER of a kind the
	// standard calls default, or a sequence type made only of them.
	defaultClass sequenceClass = iota

	// otherClass is anything else: an intrinsic type of another kind
	// (INTEGER*2, REAL*16, COMPLEX*16), or a sequence type with such a
	// component.
	otherClass
)

// classOf is t's class: a record's by its components (componentsClass). A
// default kind is told by size, the standard's numeric storage unit being a
// longword: under /NOI4 an INTEGER is INTEGER*2, of the other class (ours).
func classOf(t Type) sequenceClass {
	if t.Kind == Record {
		return componentsClass(t.Struct)
	}

	switch {
	case t.Kind == Character:
		return defaultClass
	case (t.Kind == Integer || t.Kind == Logical) && t.Size == 4:
		return defaultClass
	case t.Kind == Real && (t.Size == 4 || t.Size == 8):
		// REAL and DOUBLE PRECISION.
		return defaultClass
	case t.Kind == Complex && t.Size == 8:
		return defaultClass
	}

	return otherClass
}

// componentsClass is a sequence type's class: default if every ultimate
// component's type is (a nested type's by its own components).
func componentsClass(st *Structure) sequenceClass {
	for _, f := range st.Fields {
		// A pointer is neither numeric nor character (4.4.1's sequence
		// types): a type with one is of the other class.
		if f.Pointer {
			return otherClass
		}

		t := f.Type
		if f.Struct != nil {
			t = recordType(f.Struct)
		}

		if classOf(t) != defaultClass {
			return otherClass
		}
	}

	return defaultClass
}

// notSequence reports s, a derived-type object being put in a common block or
// an EQUIVALENCE list (where), if its type has no SEQUENCE statement, and
// returns true then.
func (a *analyzer) notSequence(s *Symbol, pos diag.Position, where string) bool {
	if s.Struct == nil || !s.Struct.Derived || s.Struct.Sequence {
		return false
	}

	a.report("NOSEQUENCE", pos, s.Name, s.Struct.Name, where)

	return true
}

// checkCommonTypes reports each derived-type object in a common block whose type
// has no SEQUENCE statement. It waits for the declarations, since a name's
// TYPE(t) statement may follow its COMMON statement.
func (a *analyzer) checkCommonTypes() {
	for _, b := range a.unit.Commons {
		for _, s := range b.Members {
			if !s.broken && a.notSequence(s, s.commonPos, "in a common block") {
				s.broken = true
			}
		}
	}
}

// equivalenceClasses checks one EQUIVALENCE list's items by the standard's type
// classes, if any is a derived-type object: an object of a type of the other
// class with objects of its own type only, and objects of default-class types
// with each other only.
func (a *analyzer) equivalenceClasses(items []equivItem) {
	// derived is the list's first derived-type object, and other its first of
	// a type of the other class.
	var derived, other *equivItem

	for i := range items {
		it := &items[i]
		if it.sym.Struct == nil {
			continue
		}

		// An object with a pointer in it can't be in the list at all
		// (POINTERPART, checkPointers): that is the one message.
		if it.sym.Struct.HasPointers() {
			return
		}

		if derived == nil {
			derived = it
		}

		if other == nil && componentsClass(it.sym.Struct) == otherClass {
			other = it
		}
	}

	if derived == nil {
		return
	}

	for i := range items {
		it := &items[i]

		switch {
		case other != nil && it != other && !(it.sym.Struct != nil && sameType(it.sym.Struct, other.sym.Struct)):
			a.report("EQUIVTYPE", it.pos, it.sym.Name, it.sym.Type.String(), other.sym.Name,
				other.sym.Type.String(), "the latter's type has components that aren't all numbers, logical values, and characters of the default kinds, so it shares storage only with objects of its own type")

			return

		case other == nil && classOf(it.sym.Type) != defaultClass:
			a.report("EQUIVTYPE", it.pos, it.sym.Name, it.sym.Type.String(), derived.sym.Name,
				derived.sym.Type.String(), "a sequence type of numbers, logical values, and characters of the default kinds shares storage only with objects of those types")

			return
		}
	}
}
