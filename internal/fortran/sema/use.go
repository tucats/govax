package sema

import (
	"slices"

	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
)

// Use association (the Fortran 90 standard's 11.3.2 and 12.1.2.2.1;
// docs/planning/PHASE-106 - Fortran 90 modules.md, finding 2). USE M makes
// M's entities names of the unit that says it, each with the attributes M gave
// it: a variable stays in M's program section, a constant keeps its value, a
// type is M's type. A USE comes first in a specification part (order.go), so
// the names are the unit's own before any other declaration is read: each is
// a Symbol in the unit's scope (not its host's), marked with where it came
// from (Use), so it hides a host's name of the same spelling, as 12.1.2.2.1
// says, and a later declaration of it finds it there and is refused
// (USEATTR, USECOMMON): a use-associated name may be respecified only in a
// NAMELIST group, and, in a module, by PUBLIC or PRIVATE.
//
// The Symbol is the unit's own copy of the module's entity, as the module's
// text gave it (modread.go): the same type, Structure, value, and place, so
// that the IR finds a cell for it in this unit as for any name of its own. A
// variable's place is its offset in the module's section, which the unit sees
// through a block of its own (Unit.UsedBlocks), as a COMMON block's units do;
// every unit that uses the module contributes the section, and LINK overlays
// the contributions, the module's own holding the initial values.

// UseOrigin is where a use-associated name came from: the module a USE named,
// the entity's name there, and the entity as the module's text gave it.
type UseOrigin struct {
	Module string
	Remote string
	Entity *Symbol
}

// uses reads the unit's USE statements, entering each module's entities as the
// unit's names. A module that can't be had (MODNOTFOUND and the rest) leaves
// the unit's declarations incomplete, so that the names it would have given
// aren't each reported as something else.
func (a *analyzer) uses(body []ast.Stmt) {
	for _, st := range body {
		s, ok := st.(*ast.UseStmt)
		if !ok {
			continue
		}

		if s.Only || len(s.Items) > 0 {
			a.notImplForm(s.Position(), "a USE statement's list")
			a.unit.incomplete = true

			continue
		}

		ms := a.moduleScope(s.Module, s.ModulePos)
		if ms == nil {
			a.unit.incomplete = true

			continue
		}

		a.useAll(ms, s.ModulePos)
	}
}

// useAll enters every entity of the module ms as a name of the unit, by its own
// name; pos is the USE's module name, where each name is said to appear.
func (a *analyzer) useAll(ms *moduleScope, pos diag.Position) {
	copies := map[*Symbol]*Symbol{}

	for _, r := range ms.unit.Symbols {
		if !useEntity(r) || r.Generic != nil {
			continue
		}

		if c := a.useSymbol(ms, r, r.Name, pos); c != nil {
			copies[r] = c
		}
	}

	// Generic names after the procedures, whose copies their specifics are.
	for _, r := range ms.unit.Symbols {
		if r.Generic != nil && !r.broken {
			a.useGeneric(ms, r, r.Name, copies, pos)
		}
	}

	a.useOperators(ms, copies, pos)

	for _, st := range ms.unit.Structures {
		if st.Name != "" {
			a.useStructure(ms, st, st.Name, pos)
		}
	}

	for _, n := range ms.unit.Namelists {
		a.useNamelist(ms, n, copies, pos)
	}
}

// useEntity reports whether the symbol r of a module read from its text is an
// entity a USE gives: a constant, a variable in a program section, a module
// procedure, an external procedure with an interface body, or a generic name.
func useEntity(r *Symbol) bool {
	switch {
	case r.broken, r.Class == UnitName:
		return false
	case r.Class == Constant, r.Common != nil, r.Internal != nil, r.Generic != nil:
		return true
	}

	return r.Interface != nil && r.Dummy == 0
}

// useProcedure makes c, a copy of the module's procedure r, a procedure here:
// with r's interface, called by r's global name, a module procedure's M.P or an
// external procedure's own name, which a rename doesn't change.
func useProcedure(c, r *Symbol) {
	c.Interface = explicitInterface(r)
	c.External = true

	switch {
	case r.LinkName != "":
		c.LinkName = r.LinkName
	case c.Name != r.Name:
		c.LinkName = r.Name
	}
}

// useCopy is the unit's copy of the module's procedure r, for a generic name's
// or an operator's specific: the copy a USE entered, or one made now that no
// name of the unit reaches, as for a specific known here only by the generic
// name.
func (a *analyzer) useCopy(ms *moduleScope, r *Symbol, copies map[*Symbol]*Symbol, pos diag.Position) *Symbol {
	if c := copies[r]; c != nil {
		return c
	}

	c := &Symbol{Name: r.Name, Class: r.Class, Type: r.Type, Typed: true, Offset: -1, Pos: pos,
		Use: &UseOrigin{Module: ms.name, Remote: r.Name, Entity: r}}
	useProcedure(c, r)
	copies[r] = c

	return c
}

// useGeneric enters the module's generic name r as local, its specifics the
// unit's copies of the module's procedures. A generic name the unit has from
// another module, or another USE, is one generic interface with this one
// (11.3.2): its specifics are joined, and checkGenerics then holds the joined
// list to 14.1.2.3 in this unit.
func (a *analyzer) useGeneric(ms *moduleScope, r *Symbol, local string, copies map[*Symbol]*Symbol, pos diag.Position) {
	specifics := make([]*Symbol, 0, len(r.Generic.Specifics))
	for _, p := range r.Generic.Specifics {
		specifics = append(specifics, a.useCopy(ms, p, copies, pos))
	}

	old := a.scope.byName[local]

	switch {
	case old == nil && r.Interface != nil:
		// The generic name is a specific's own too: its copy.
		old = a.useCopy(ms, r, copies, pos)
		a.scope.enter(old)
		a.note(&old.refs, pos, RefDefined)

		old.Generic = &Generic{Name: local}

	case old == nil:
		old = &Symbol{Name: local, Offset: -1, Pos: pos, Typed: true, Class: r.Class,
			Use: &UseOrigin{Module: ms.name, Remote: r.Name, Entity: r}, Generic: &Generic{Name: local}}
		a.scope.enter(old)
		a.note(&old.refs, pos, RefDefined)

	case old.Generic == nil:
		a.report("USELOCAL", pos, local, ms.name, old.Pos.Line)

		return
	}

	for _, p := range specifics {
		if !slices.Contains(old.Generic.Specifics, p) {
			old.Generic.Specifics = append(old.Generic.Specifics, p)
		}
	}
}

// useOperators joins the module's operators and assignment to the unit's, their
// specifics the unit's copies of the module's procedures.
func (a *analyzer) useOperators(ms *moduleScope, copies map[*Symbol]*Symbol, pos diag.Position) {
	for key, g := range ms.unit.operators {
		if a.unit.operators == nil {
			a.unit.operators = map[string]*Generic{}
		}

		mine := a.unit.operators[key]
		if mine == nil {
			mine = &Generic{Name: g.Name}
			a.unit.operators[key] = mine
		}

		for _, p := range g.Specifics {
			if c := a.useCopy(ms, p, copies, pos); !slices.Contains(mine.Specifics, c) {
				mine.Specifics = append(mine.Specifics, c)
			}
		}
	}
}

// useSymbol enters the module's entity r in the unit as local, unless the unit
// has the name: the same entity by another path is the name it has, and any
// other name of the unit's own is USELOCAL.
func (a *analyzer) useSymbol(ms *moduleScope, r *Symbol, local string, pos diag.Position) *Symbol {
	if old := a.scope.byName[local]; old != nil {
		if old.Use != nil && old.Use.Entity == r {
			return old
		}

		a.report("USELOCAL", pos, local, ms.name, old.Pos.Line)

		return nil
	}

	c := &Symbol{
		Name: local, Class: r.Class, Type: r.Type, Typed: true, TypePos: pos,
		Struct: r.Struct, Dims: r.Dims, DimPos: pos,
		Offset: -1, Pos: pos,
		Value: r.Value, ArrayValue: r.ArrayValue, recordValue: r.recordValue, recordValues: r.recordValues,
		Allocatable: r.Allocatable, Volatile: r.Volatile, Target: r.Target,
		Use: &UseOrigin{Module: ms.name, Remote: r.Name, Entity: r},
	}

	if r.Internal != nil || (r.Interface != nil && r.Dummy == 0) {
		useProcedure(c, r)
	}

	if r.Common != nil {
		c.Common = a.usedBlock(r.Common, pos)
		c.Offset = r.Offset
		c.Common.Members = append(c.Common.Members, c)
	}

	a.scope.enter(c)
	a.note(&c.refs, pos, RefDefined)

	return c
}

// usedBlock is the unit's view of the program section rb of a used module,
// made the first time (Unit.UsedBlocks).
func (a *analyzer) usedBlock(rb *CommonBlock, pos diag.Position) *CommonBlock {
	for _, b := range a.unit.UsedBlocks {
		if b.Psect == rb.Psect {
			return b
		}
	}

	b := &CommonBlock{
		Name: rb.Name, Module: rb.Module, Psect: rb.Psect, Size: rb.Size, LayoutAlign: rb.LayoutAlign,
		Attrs: rb.Attrs, NamePos: pos,
	}
	a.unit.UsedBlocks = append(a.unit.UsedBlocks, b)

	return b
}

// useStructure makes the module's type st the unit's, named local: it is the
// module's own Structure, so a type reached by two paths is one type (finding
// 3).
func (a *analyzer) useStructure(ms *moduleScope, st *Structure, local string, pos diag.Position) {
	if old := a.unit.structs[local]; old != nil {
		if old != st {
			a.report("USELOCAL", pos, local, ms.name, old.Pos.Line)
		}

		return
	}

	a.unit.structs[local] = st

	if a.unit.usedStructs == nil {
		a.unit.usedStructs = map[*Structure]string{}
	}

	a.unit.usedStructs[st] = ms.name
}

// useNamelist makes the module's NAMELIST group n the unit's, its items the
// unit's copies of the module's variables.
func (a *analyzer) useNamelist(ms *moduleScope, n *Namelist, copies map[*Symbol]*Symbol, pos diag.Position) {
	if old := a.unit.namelists[n.Name]; old != nil {
		a.report("USELOCAL", pos, n.Name, ms.name, old.Pos.Line)

		return
	}

	c := &Namelist{Name: n.Name, Pos: pos}

	for _, s := range n.Items {
		item := copies[s]
		if item == nil {
			return
		}

		c.Items = append(c.Items, item)
	}

	if a.unit.namelists == nil {
		a.unit.namelists = map[string]*Namelist{}
	}

	a.unit.namelists[n.Name] = c
	a.unit.Namelists = append(a.unit.Namelists, c)
}

// respecified reports a declaration of the use-associated name s, which only a
// NAMELIST group, and in a module PUBLIC and PRIVATE, may name (USEATTR, or
// USECOMMON for COMMON and EQUIVALENCE), and reports whether it was one.
func (a *analyzer) respecified(s *Symbol, pos diag.Position) bool {
	if s.Use == nil || a.respecify || a.scope.byName[s.Name] != s {
		return false
	}

	ident := "USEATTR"
	if a.storageAssoc {
		ident = "USECOMMON"
	}

	a.report(ident, pos, s.Name, s.Use.Module)

	// The declaration's own checks say nothing more of it.
	s.broken = true

	return true
}

// usedStructure reports a structure or type definition whose name a USE gave
// the unit (USELOCAL), and reports whether it was one.
func (a *analyzer) usedStructure(name string, pos diag.Position) bool {
	old := a.unit.structs[name]
	if old == nil {
		return false
	}

	module := a.unit.usedStructs[old]
	if module == "" {
		return false
	}

	a.report("USEATTR", pos, name, module)

	return true
}
