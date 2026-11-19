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

	// origin is the entity's defining module and its name there, which may be
	// another module than the one the USE named, when that one passes on an
	// entity it uses (finding 8): one entity reached by two paths has one
	// origin, and is one name.
	origin entityOrigin
}

// entityOrigin is an entity's defining module and its name there.
type entityOrigin struct {
	Module, Name string
}

// symbolOrigin is the origin of the entity r of the module ms read from its
// text: its FROM line's, or the module's own.
func symbolOrigin(r *Symbol, ms *moduleScope) entityOrigin {
	if r.from != nil {
		return *r.from
	}

	return entityOrigin{Module: ms.name, Name: r.Name}
}

// useRequest is a unit's USE statements of one module, joined as 11.3.2 says:
// if any has no ONLY, every public entity, the lists one list of renames; if
// all have ONLY, only what the lists name.
type useRequest struct {
	module string
	pos    diag.Position
	all    bool
	items  []*ast.UseItem
}

// uses reads the unit's USE statements, entering each module's entities as the
// unit's names. A module that can't be had (MODNOTFOUND and the rest) leaves
// the unit's declarations incomplete, so that the names it would have given
// aren't each reported as something else.
func (a *analyzer) uses(body []ast.Stmt) {
	var requests []*useRequest

	byModule := map[string]*useRequest{}

	for _, st := range body {
		s, ok := st.(*ast.UseStmt)
		if !ok {
			continue
		}

		r := byModule[s.Module]
		if r == nil {
			r = &useRequest{module: s.Module, pos: s.ModulePos}
			byModule[s.Module] = r
			requests = append(requests, r)
		}

		r.all = r.all || !s.Only
		r.items = append(r.items, s.Items...)
	}

	for _, r := range requests {
		ms := a.moduleScope(r.module, r.pos)
		if ms == nil {
			a.unit.incomplete = true

			continue
		}

		a.useModule(ms, r)
	}
}

// localName is a name an entity is given here, and where.
type localName struct {
	name string
	pos  diag.Position
}

// useModule enters the module ms's public entities the request r asks for, by
// the names it gives them: each renamed entity by its local names, every
// other by its own if r takes every entity (ONLYPUBLIC for an item that names
// no public entity).
func (a *analyzer) useModule(ms *moduleScope, r *useRequest) {
	mu := ms.unit
	wanted := map[string][]localName{}
	ops := map[string]bool{}

	for _, it := range r.items {
		if it.Generic.Kind != ast.NoGeneric {
			key := specKey(it.Generic)

			if mu.operators[key] == nil || mu.access.privateOps[key] {
				a.report("ONLYPUBLIC", it.Pos, it.Generic.String(), ms.name)

				continue
			}

			ops[key] = true

			continue
		}

		if !ms.public(it.Remote) {
			a.report("ONLYPUBLIC", it.Pos, it.Remote, ms.name)

			continue
		}

		local := it.Local
		if local == "" {
			local = it.Remote
		}

		wanted[it.Remote] = append(wanted[it.Remote], localName{local, it.Pos})
	}

	localsOf := func(remote string) []localName {
		if w := wanted[remote]; len(w) > 0 {
			return w
		}

		if r.all {
			return []localName{{remote, r.pos}}
		}

		return nil
	}

	copies := map[*Symbol]*Symbol{}

	for _, e := range mu.Symbols {
		if !ms.publicSymbol(e) || e.Generic != nil {
			continue
		}

		for _, n := range localsOf(e.Name) {
			if c := a.useSymbol(ms, e, n.name, n.pos); c != nil && copies[e] == nil {
				copies[e] = c
			}
		}
	}

	// Generic names after the procedures, whose copies their specifics are.
	for _, e := range mu.Symbols {
		if ms.publicSymbol(e) && e.Generic != nil {
			for _, n := range localsOf(e.Name) {
				a.useGeneric(ms, e, n.name, copies, n.pos)
			}
		}
	}

	for _, key := range keysInOrder(mu.operators) {
		if !mu.access.privateOps[key] && (r.all || ops[key]) {
			a.useOperator(ms, key, mu.operators[key], copies, r.pos)
		}
	}

	for _, name := range keysInOrder(mu.structs) {
		if !mu.access.privateTypes[name] {
			for _, n := range localsOf(name) {
				a.useStructure(ms, mu.structs[name], n.name, n.pos)
			}
		}
	}

	for _, nl := range mu.Namelists {
		if !nl.private {
			for _, n := range localsOf(nl.Name) {
				a.useNamelist(ms, nl, n.name, copies, n.pos)
			}
		}
	}
}

// keysInOrder are a map's keys in order.
func keysInOrder[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}

	slices.Sort(keys)

	return keys
}

// public reports whether the module has a public entity named name: a name of
// its scope, a type, or a namelist group.
func (ms *moduleScope) public(name string) bool {
	mu := ms.unit

	if e := mu.scope.byName[name]; e != nil && ms.publicSymbol(e) {
		return true
	}

	if mu.structs[name] != nil && !mu.access.privateTypes[name] {
		return true
	}

	nl := mu.namelists[name]

	return nl != nil && !nl.private
}

// publicSymbol reports whether e, a symbol of the module, is a public entity
// a USE gives.
func (ms *moduleScope) publicSymbol(e *Symbol) bool {
	return useEntity(e) && !e.private && ms.unit.scope.byName[e.Name] == e
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
	c.moduleProc = isModuleProcedure(r)

	switch {
	case r.LinkName != "":
		c.LinkName = r.LinkName
	case c.Name != r.Name:
		c.LinkName = r.Name
	}
}

// isModuleProcedure reports whether s is a module procedure: one of this
// module's, or a use-associated one's copy.
func isModuleProcedure(s *Symbol) bool {
	return s.moduleProc || (s.Internal != nil && s.Internal.LinkName != "")
}

// useCopy is the unit's copy of the module's entity r, for a generic name's or
// an operator's specific, or a namelist group's item: the copy a USE entered,
// or one made now that no name of the unit reaches (Unit.hidden), as for a
// specific known here only by the generic name.
func (a *analyzer) useCopy(ms *moduleScope, r *Symbol, copies map[*Symbol]*Symbol, pos diag.Position) *Symbol {
	if c := copies[r]; c != nil {
		return c
	}

	c := a.symbolCopy(ms, r, r.Name, pos)
	copies[r] = c
	a.unit.hidden = append(a.unit.hidden, c)

	return c
}

// useGeneric enters the module's generic name r as local, its specifics the
// unit's copies of the module's procedures. A generic name the unit has from
// another module, or another USE, is one generic interface with this one
// (11.3.2): its specifics are joined, and checkGenerics then holds the joined
// list to 14.1.2.3 in this unit.
func (a *analyzer) useGeneric(ms *moduleScope, r *Symbol, local string, copies map[*Symbol]*Symbol, pos diag.Position) {
	old := a.scope.byName[local]

	switch {
	case old == nil && r.Interface != nil:
		// The generic name is a specific's own too: its copy.
		old = a.symbolCopy(ms, r, local, pos)
		a.scope.enter(old)
		a.note(&old.refs, pos, RefDefined)

		if copies[r] == nil {
			copies[r] = old
		}

		old.Generic = &Generic{Name: local}

	case old == nil:
		old = &Symbol{Name: local, Offset: -1, Pos: pos, Typed: true, Class: r.Class,
			Use: &UseOrigin{Module: ms.name, Remote: r.Name, Entity: r, origin: symbolOrigin(r, ms)}, Generic: &Generic{Name: local}}
		a.scope.enter(old)
		a.note(&old.refs, pos, RefDefined)

	case old.Generic == nil && old.Use != nil:
		a.ambiguous(old, ms)

		return

	case old.Generic == nil:
		a.report("USELOCAL", pos, local, ms.name, old.Pos.Line)

		return
	}

	for _, sp := range r.Generic.Specifics {
		if p := a.useCopy(ms, sp, copies, pos); !slices.Contains(old.Generic.Specifics, p) {
			old.Generic.Specifics = append(old.Generic.Specifics, p)
		}
	}
}

// useOperator joins the module's operator or assignment key to the unit's,
// its specifics the unit's copies of the module's procedures.
func (a *analyzer) useOperator(ms *moduleScope, key string, g *Generic, copies map[*Symbol]*Symbol, pos diag.Position) {
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

// useSymbol enters the module's entity r in the unit as local, unless the unit
// has the name: the same entity by another path is the name it has, another
// module's entity makes the name ambiguous, and any other name of the unit's
// own is USELOCAL.
func (a *analyzer) useSymbol(ms *moduleScope, r *Symbol, local string, pos diag.Position) *Symbol {
	if old := a.scope.byName[local]; old != nil {
		switch {
		case old.Use != nil && old.Use.origin == symbolOrigin(r, ms) && old.Generic == nil:
			return old
		case old.Use != nil:
			a.ambiguous(old, ms)
		default:
			a.report("USELOCAL", pos, local, ms.name, old.Pos.Line)
		}

		return nil
	}

	c := a.symbolCopy(ms, r, local, pos)
	a.scope.enter(c)
	a.note(&c.refs, pos, RefDefined)

	return c
}

// ambiguous marks the use-associated name old, which the module ms gives for
// another entity too: it may be there, but not referenced (11.3.2, USEAMBIG).
func (a *analyzer) ambiguous(old *Symbol, ms *moduleScope) {
	if old.ambiguous == "" {
		old.ambiguous = ms.name
	}
}

// symbolCopy is the unit's copy of the module's entity r, named local: the
// same type, structure, value, and place.
func (a *analyzer) symbolCopy(ms *moduleScope, r *Symbol, local string, pos diag.Position) *Symbol {
	c := &Symbol{
		Name: local, Class: r.Class, Type: r.Type, Typed: true, TypePos: pos,
		Struct: r.Struct, Dims: r.Dims, DimPos: pos,
		Offset: -1, Pos: pos,
		Value: r.Value, ArrayValue: r.ArrayValue, recordValue: r.recordValue, recordValues: r.recordValues,
		Allocatable: r.Allocatable, Volatile: r.Volatile, Target: r.Target,
		Use: &UseOrigin{Module: ms.name, Remote: r.Name, Entity: r, origin: symbolOrigin(r, ms)},
	}

	if r.Internal != nil || (r.Interface != nil && r.Dummy == 0) {
		useProcedure(c, r)
	}

	if r.Common != nil {
		c.Common = a.usedBlock(r.Common, pos)
		c.Offset = r.Offset
		c.Common.Members = append(c.Common.Members, c)
	}

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
// 3). Another module's type of the name makes the name ambiguous: an error
// only where it is used (ambiguousType).
func (a *analyzer) useStructure(ms *moduleScope, st *Structure, local string, pos diag.Position) {
	if old := a.unit.structs[local]; old != nil {
		switch module := a.unit.usedStructs[old]; {
		case old == st:
		case module != "":
			if a.unit.ambiguousTypes == nil {
				a.unit.ambiguousTypes = map[string][2]string{}
			}

			if _, ok := a.unit.ambiguousTypes[local]; !ok {
				a.unit.ambiguousTypes[local] = [2]string{module, ms.name}
			}
		default:
			a.report("USELOCAL", pos, local, ms.name, old.Pos.Line)
		}

		return
	}

	a.unit.structs[local] = st
	a.unit.usedTypes = append(a.unit.usedTypes, usedType{local, st})

	if a.unit.usedStructs == nil {
		a.unit.usedStructs = map[*Structure]string{}
	}

	if a.unit.usedStructs[st] == "" {
		a.unit.usedStructs[st] = ms.name
	}
}

// usedType is a type a USE gave the unit, and its local name.
type usedType struct {
	local string
	st    *Structure
}

// ambiguousType reports a use of a type name two USEs gave for two types
// (USEAMBIG), and reports whether it was one.
func (a *analyzer) ambiguousType(name string, pos diag.Position) bool {
	two, ok := a.ambiguousTypeOf(name)
	if ok {
		a.report("USEAMBIG", pos, name, two[0], two[1])
	}

	return ok
}

// ambiguousTypeOf is the two modules that gave the type name the unit reaches
// two types, if they did.
func (a *analyzer) ambiguousTypeOf(name string) ([2]string, bool) {
	for u := a.unit; u != nil; u = u.Host {
		if u.structs[name] != nil {
			two, ok := u.ambiguousTypes[name]

			return two, ok
		}
	}

	return [2]string{}, false
}

// useNamelist makes the module's NAMELIST group n the unit's, named local, its
// items the unit's copies of the module's variables. The same group by
// another path is the one the unit has; another module's group of the name
// makes it ambiguous.
func (a *analyzer) useNamelist(ms *moduleScope, n *Namelist, local string, copies map[*Symbol]*Symbol, pos diag.Position) {
	if old := a.unit.namelists[local]; old != nil {
		switch {
		case old.origin == n.origin:
		case old.origin.Module != "":
			if old.ambiguous == "" {
				old.ambiguous = ms.name
			}
		default:
			a.report("USELOCAL", pos, local, ms.name, old.Pos.Line)
		}

		return
	}

	c := &Namelist{Name: local, Pos: pos, origin: n.origin}

	for _, s := range n.Items {
		c.Items = append(c.Items, a.useCopy(ms, s, copies, pos))
	}

	if a.unit.namelists == nil {
		a.unit.namelists = map[string]*Namelist{}
	}

	a.unit.namelists[local] = c
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
