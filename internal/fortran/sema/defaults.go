package sema

import (
	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
)

// Default initialization (Fortran 95; the HP manual's 3.3.1 and 3.3.2;
// docs/planning/PHASE-112 - Fortran 95.md, A.3's 4) gives a derived type's
// components values every object of the type starts with
// (Structure.DefaultInit). Where they go depends on where the object is:
//
//   - static storage that is set once, a main program's or a module's
//     variables, a subprogram's saved ones: the record's initial values, in the
//     image, as a STRUCTURE's are (record.go's recordInitials);
//   - a subprogram's other locals, its INTENT(OUT) dummies, and its value: set
//     at every entry point, before the first statement (Unit.Defaults; the
//     standard's 14.7.5), so a local is its defaults at each call, not what the
//     last call left, and default initialization doesn't imply SAVE (the
//     author's answer 4; gfortran agrees);
//   - what ALLOCATE makes: set after the allocation (Allocation.Default).
//
// The setting at run time is an assignment of a record constant, the type's
// DefaultBytes, which lowering moves as any aggregate assignment (a MOVC3 per
// record, a loop over an array's elements). A pointer component's bytes are 0
// in it, so it is disassociated too.

// Default is one object a unit's entry points set to its defaults: Assign is
// the assignment that does it, and Present, for an OPTIONAL dummy, the test
// that an argument was given for it.
type Default struct {
	Sym     *Symbol
	Assign  *Assign
	Present *Present
}

// defaultValue is the value of a record of the type st as its default
// initialization leaves it, a record constant.
func defaultValue(st *Structure, pos diag.Position) *Const {
	return &Const{exprBase: exprBase{T: recordType(st), Pos: pos}, Raw: st.DefaultBytes()}
}

// hasDefaults reports whether s is an object of a type with default
// initialization, or an array of one.
func hasDefaults(s *Symbol) bool {
	return s.Struct != nil && s.Type.Kind == Record && s.Struct.DefaultInit()
}

// defaults finds the objects each entry point of the unit sets to their
// defaults (Unit.Defaults), and marks them, so that recordInitials gives them
// no initial values. A dummy argument is set only if it is INTENT(OUT), and an
// assumed-size one can't be (DEFAULTINIT: nobody can count its elements). A
// function's value is set through its own name's place, which its entry points'
// names share.
func (a *analyzer) defaults() {
	u := a.unit

	if u.Kind != ast.FunctionUnit && u.Kind != ast.SubroutineUnit {
		return
	}

	for _, s := range u.Symbols {
		if s.broken || !hasDefaults(s) || s.Pointer || s.Allocatable || s.Use != nil || s.Common != nil ||
			s.Class != Variable && s.Class != Array && s.Class != Unclassified {
			continue
		}

		var present *Present

		switch {
		case s.Dummy != 0 && s.Intent != "OUT":
			continue

		case s.Dummy != 0 && s.AssumedSize():
			a.report("DEFAULTINIT", s.DimPos, s.Name, s.Struct.Name, "be an assumed-size INTENT(OUT) dummy argument")

			continue

		case s.Dummy != 0:
			if s.Optional {
				present = &Present{exprBase: exprBase{T: LogicalType, Pos: s.Pos}, Sym: s}
			}

		case s.Result:
			if s != u.Result {
				continue
			}

		case s.saved || a.saveAll || s.recordValue != nil || s.recordValues != nil:
			continue
		}

		s.entryDefault = true
		u.Defaults = append(u.Defaults, &Default{Sym: s, Assign: a.defaultAssign(s, s.Pos), Present: present})
	}
}

// defaultAssign is the assignment that sets s, a scalar or an array, to its
// type's defaults.
func (a *analyzer) defaultAssign(s *Symbol, pos diag.Position) *Assign {
	value := defaultValue(s.Struct, pos)
	header := a.unit.AST.Header

	if len(s.Dims) == 0 {
		return &Assign{stmtBase: stmtBase{header}, Target: NewVar(s, pos), Value: value}
	}

	return a.arrayDefault(NewArrayRef(s, pos), value)
}

// arrayDefault is target = value, an array's elements set to a scalar record
// constant, which reads nothing the target may share.
func (a *analyzer) arrayDefault(target Expr, value Expr) *Assign {
	order, _ := a.arrayOrder(value.Position(), target, value)

	return &Assign{stmtBase: stmtBase{a.unit.AST.Header}, Target: target, Value: value, Order: order}
}

// allocationDefault is the assignment that sets what ALLOCATE makes for obj
// to its type's defaults, or nil if its type has none.
func (a *analyzer) allocationDefault(obj *Allocation, pos diag.Position) *Assign {
	header := a.unit.AST.Header

	switch {
	case obj.Component != nil:
		t := obj.Component.Type()
		if t.Kind != Record || t.Struct == nil || !t.Struct.DefaultInit() {
			return nil
		}

		return &Assign{stmtBase: stmtBase{header}, Target: obj.Component, Value: defaultValue(t.Struct, pos)}

	case obj.Sym == nil || !hasDefaults(obj.Sym):
		return nil

	case obj.Sym.Allocatable || obj.Sym.ArrayPointer():
		return a.arrayDefault(NewArrayRef(obj.Sym, pos), defaultValue(obj.Sym.Struct, pos))
	}

	return &Assign{stmtBase: stmtBase{header}, Target: NewVar(obj.Sym, pos), Value: defaultValue(obj.Sym.Struct, pos)}
}

// checkDefaultCommons reports each object of a type with default
// initialization in a common block (Fortran 95's 5.5.2; DEFAULTINIT): a block
// has one image, which no object may set twice. A module's own variables are in
// a section of their own, which isn't a COMMON statement's.
func (a *analyzer) checkDefaultCommons() {
	for _, b := range a.unit.Commons {
		for _, s := range b.Members {
			if !s.broken && hasDefaults(s) {
				a.report("DEFAULTINIT", s.commonPos, s.Name, s.Struct.Name, "be in a common block")

				s.broken = true
			}
		}
	}
}
