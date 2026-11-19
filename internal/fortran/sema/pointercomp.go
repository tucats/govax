package sema

import (
	"github.com/tucats/govax/internal/fortran/ast"
)

// Array pointer components (docs/planning/PHASE-107 - Fortran 90 pointers.md,
// subtask 5b). A component declared
//
//	TYPE HOLDER
//	  INTEGER :: N
//	  INTEGER, POINTER :: V(:)
//	END TYPE
//
// is a pointer whose storage, in each HOLDER, is a class NCA descriptor, as a
// named array pointer's is (Field.Rank). Everything a program does with one,
// H%V => A(2:9), H%V(I), SUM(H%V), ALLOCATE (H%V(N)), H%V passed to an
// assumed-shape dummy, is what it does with a named array pointer, so the
// analyzer makes the reference to the component itself, H%V, a symbol of its
// own, an array pointer whose Component is that reference, and reads the rest
// of the reference as it would for a pointer of that name: H%V(I) an Elem of
// it, H%V(1:3) a Section, H%V a whole ArrayRef, and for an array pointer of
// records, T%KIDS(2)%KEY or T%KIDS%KEY, a FieldRef from it. Lowering gives such
// a symbol no cell of its own: its descriptor is found in the record, where
// the Component reference says, each time it is used (ir/lowerpointer.go's
// componentCell).
//
// The symbol for one reference, its subscripts written the same way, is made
// once in a scope, so that H%V = H%V + 1 reads and writes one array, which
// overlap.go can then order, rather than two pointers that may share. The
// scope is part of what makes it the same: in one scope a name is always the
// same symbol, where in a statement function's definition, an array
// constructor's implied DO, or an internal procedure, H or I may be another.

// componentKey is what makes two references to an array pointer component
// the same: the scope they are read in, and their text with subscripts.
type componentKey struct {
	scope *scope
	text  string
}

// componentPointer is the symbol that stands for the array pointer component
// ref names, its last field, with no part of ref having rank.
func (a *analyzer) componentPointer(ref *FieldRef) *Symbol {
	key := componentKey{a.scope, DumpExpr(ref)}

	if s := a.componentPointers[key]; s != nil {
		return s
	}

	s := NewComponentPointer(ref.Field())
	s.Name, s.Pos, s.TypePos, s.Volatile, s.Component = ref.Text(), ref.Pos, ref.Pos, ref.Sym.Volatile, ref

	if a.componentPointers == nil {
		a.componentPointers = map[componentKey]*Symbol{}
	}

	a.componentPointers[key] = s

	return s
}

// pointerPart is the reference to the array pointer component s stands for
// that step, the component's own step, ends with: the whole array, or with
// subscripts an element or a section of it.
func (a *analyzer) pointerPart(s *Symbol, step *ast.FieldRef) Expr {
	if step.Args == nil {
		return NewArrayRef(s, step.Pos)
	}

	return a.section(s, &ast.Apply{ExprBase: ast.ExprBase{Pos: step.Pos}, Name: s.Name, Args: step.Args})
}

// NewComponentPointer is an array pointer of the array pointer component f's
// type and rank, named for it, which stands for it where no reference names
// it: a structure constructor's (lowering's buildRecord).
func NewComponentPointer(f *Field) *Symbol {
	dims := make([]Dim, f.Rank)

	for i := range dims {
		dims[i].Deferred = true
	}

	return &Symbol{
		Name: f.Name, Pos: f.Pos, Class: Array, Type: componentType(f), Struct: f.Struct,
		Dims: dims, Pointer: true, Typed: true, TypePos: f.Pos,
	}
}
