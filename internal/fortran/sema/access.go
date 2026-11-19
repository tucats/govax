package sema

import (
	"slices"

	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
)

// Accessibility (the Fortran 90 standard's 5.1.2.2, 5.2.3, and 4.4.1;
// docs/planning/PHASE-106 - Fortran 90 modules.md, subtask 4). A module's
// entities are public, reachable by a USE, or private, the module's own:
//
//	MODULE STACKS
//	  PRIVATE                       the default: private
//	  PUBLIC :: PUSH, POP, DEPTH    these public
//	  INTEGER :: DEPTH = 0
//	  REAL :: ITEMS(100)            private, by the default
//	  TYPE CELL
//	    PRIVATE                     a public type whose components are private
//	    REAL :: VALUE
//	  END TYPE CELL
//	CONTAINS
//	  ...
//
// The default is PUBLIC unless the module says PRIVATE alone (once at most,
// ACCESSDEFAULT); a name is given PUBLIC or PRIVATE once at most, by an access
// statement or an attribute (ACCESSTWICE), and only if it is a variable,
// procedure, type, constant, namelist group, or generic interface
// (ACCESSNAME). A use-associated name may be given either: that is how a
// module decides what it passes on (11.3.2).
//
// Access belongs to a name in a module, not to the entity: a module that uses
// another may make one of its public entities private, so a Symbol (each
// unit's copy of an entity) carries it, and a type, which is one Structure
// however it is reached, has its access by local name in moduleAccess.
//
// The statements are read in two steps. A module's procedures are known only
// after its declarations (analyzeContained), so accessStatements, with the
// declarations, records what each statement says and makes a name nothing
// declares an implicitly typed variable, as any first use would; then
// resolveAccess, once the procedures and their generic interfaces are known,
// gives every entity its access and checks the rules that tie access to types
// (accessChecks).

// moduleAccess is a module's accessibility.
type moduleAccess struct {
	// defaultPrivate is the default; defaultPos is where PUBLIC or PRIVATE
	// alone said it, if one did.
	defaultPrivate bool
	defaultPos     diag.Position

	// given is what an access statement or attribute said explicitly, by
	// name, or a generic specification's text for an operator
	// (OPERATOR(+)); items are the statements' items, checked by
	// resolveAccess.
	given map[string]accessMark
	items []*ast.AccessItem

	// privateTypes are the module's types' local names that are private, and
	// privateOps its operators' keys (operators.go) that are.
	privateTypes map[string]bool
	privateOps   map[string]bool
}

// accessMark is one explicit PUBLIC or PRIVATE.
type accessMark struct {
	private bool
	pos     diag.Position
}

// newModuleAccess is a module's accessibility before its statements: public.
func newModuleAccess() *moduleAccess {
	return &moduleAccess{given: map[string]accessMark{}, privateTypes: map[string]bool{}, privateOps: map[string]bool{}}
}

// specKey is a generic specification's key among a unit's operators.
func specKey(g ast.GenericSpec) string {
	if g.Kind == ast.GenericAssign {
		return assignmentKey
	}

	return g.Op
}

// operatorText is an operator's key as an access statement or USE writes it.
func operatorText(key string) string {
	if key == assignmentKey {
		return "ASSIGNMENT(=)"
	}

	return "OPERATOR(" + key + ")"
}

// giveAccess records PUBLIC or PRIVATE given the name (or operator text) key
// at pos, by a statement or an attribute: once only (ACCESSTWICE).
func (a *analyzer) giveAccess(key string, private bool, pos diag.Position) {
	acc := a.unit.access
	if acc == nil {
		return
	}

	if old, ok := acc.given[key]; ok {
		a.report("ACCESSTWICE", pos, key, old.pos.Line)

		return
	}

	acc.given[key] = accessMark{private: private, pos: pos}
}

// accessStatements reads a module's PUBLIC and PRIVATE statements: a default
// (ACCESSDEFAULT for a second), or a list, each item recorded for
// resolveAccess. A name that nothing declares and that isn't one of the
// module's procedures or generic interfaces is an implicitly typed variable,
// entered now so that the module's section holds it.
func (a *analyzer) accessStatements(body []ast.Stmt) {
	acc := a.unit.access
	later := a.laterNames()

	for _, st := range body {
		s, ok := st.(*ast.AccessStmt)
		if !ok {
			continue
		}

		private := s.Access == "PRIVATE"

		if len(s.Items) == 0 {
			if acc.defaultPos.Line != 0 {
				a.report("ACCESSDEFAULT", s.Position(), acc.defaultPos.Line)

				continue
			}

			acc.defaultPrivate, acc.defaultPos = private, s.Position()

			continue
		}

		for _, it := range s.Items {
			a.giveAccess(it.Text(), private, it.Pos)
			acc.items = append(acc.items, it)

			if it.Generic.Kind != ast.NoGeneric || later[it.Name] || a.declared(it.Name) {
				continue
			}

			a.lookup(it.Name, it.Pos)
		}
	}
}

// laterNames are the names a module's statements give only after its
// declarations are read: its procedures' and its generic interfaces'.
func (a *analyzer) laterNames() map[string]bool {
	names := map[string]bool{}

	for _, cu := range a.unit.AST.Contains {
		names[cu.Name()] = true
	}

	for _, b := range a.interfaces {
		if b.Generic.Kind == ast.GenericName {
			names[b.Generic.Name] = true
		}
	}

	return names
}

// declared reports whether the unit has the name: a name of its own scope, a
// type, or a namelist group.
func (a *analyzer) declared(name string) bool {
	return a.scope.byName[name] != nil || a.unit.structs[name] != nil || a.unit.namelists[name] != nil
}

// resolveAccess gives each of a module's entities its access, now that its
// procedures and generic interfaces are known: what was given it explicitly,
// else the default. Each item of an access statement must name an entity
// access can be given (ACCESSNAME).
func (a *analyzer) resolveAccess() {
	u := a.unit
	acc := u.access

	if acc == nil {
		return
	}

	for _, it := range acc.items {
		if it.Generic.Kind != ast.NoGeneric {
			if u.operators[specKey(it.Generic)] == nil {
				a.report("ACCESSNAME", it.Pos, it.Text(), "has no interface block in this module")
			}

			continue
		}

		if u.structs[it.Name] != nil || u.namelists[it.Name] != nil {
			continue
		}

		switch s := a.scope.byName[it.Name]; {
		case s == nil:
			a.report("ACCESSNAME", it.Pos, it.Name, "is nothing this module declares")
		case s.broken:
		case !accessEntity(s):
			a.report("ACCESSNAME", it.Pos, it.Name, "is "+a.describe(s))
		}
	}

	privateOf := func(key string) bool {
		if m, ok := acc.given[key]; ok {
			return m.private
		}

		return acc.defaultPrivate
	}

	for _, s := range u.Symbols {
		if a.scope.byName[s.Name] == s {
			s.private = privateOf(s.Name)
		}
	}

	for name := range u.structs {
		acc.privateTypes[name] = privateOf(name)
	}

	for _, n := range u.Namelists {
		n.private = privateOf(n.Name)
	}

	for key := range u.operators {
		acc.privateOps[key] = privateOf(operatorText(key))
	}

	// A module's text was checked when its module compiled; what it gives
	// that its module had no name for is private there, which these rules
	// would misread.
	if a.reading == nil {
		a.accessChecks()
	}
}

// accessEntity reports whether s is a name PUBLIC or PRIVATE may be given
// (R522's constraint): a variable, a constant, a procedure, or a generic name.
func accessEntity(s *Symbol) bool {
	switch {
	case s.Class == UnitName, s.Class == Intrinsic, s.Class == StmtFunction, s.Dummy != 0:
		return false
	}

	return true
}

// typePrivate reports whether the type st has the PRIVATE attribute in this
// module: the module has a name for it, and every name it has is private. A
// type the module can't name at all (one a USE's ONLY list left out) has no
// attribute here, as the standard's constraints read.
func (a *analyzer) typePrivate(st *Structure) bool {
	acc := a.unit.access
	named := false

	for name, t := range a.unit.structs {
		if t == st {
			if !acc.privateTypes[name] {
				return false
			}

			named = true
		}
	}

	return named
}

// symbolStruct is the derived type or structure of s's value, or nil.
func symbolStruct(s *Symbol) *Structure {
	if s == nil {
		return nil
	}

	if s.Struct != nil {
		return s.Struct
	}

	return s.Type.Struct
}

// accessChecks checks what ties access to types: a public entity can't be of
// a private type (ACCESSTYPE); a module procedure with an argument or value of
// one must be private and no public generic interface's (ACCESSPROC); a public
// namelist group can't hold a private variable (ACCESSNML); and a public type
// with a component of a private type must make its components private
// (ACCESSCOMPONENT).
//
// The findings are reported in the order of the lines they are on, which the
// checks, a kind at a time, don't visit in.
func (a *analyzer) accessChecks() {
	u := a.unit

	var findings []accessFinding

	report := func(ident string, pos diag.Position, args ...any) {
		findings = append(findings, accessFinding{ident, pos, args})
	}

	defer func() {
		slices.SortStableFunc(findings, func(x, y accessFinding) int {
			if x.pos.Line != y.pos.Line {
				return x.pos.Line - y.pos.Line
			}

			return x.pos.Column - y.pos.Column
		})

		for _, f := range findings {
			a.report(f.ident, f.pos, f.args...)
		}
	}()

	for _, st := range u.Structures {
		if !st.Derived || st.Module != u.Name || st.PrivateComponents || a.typePrivate(st) {
			continue
		}

		for _, f := range st.Fields {
			if f.Struct != nil && a.typePrivate(f.Struct) {
				report("ACCESSCOMPONENT", f.Pos, st.Name, f.Name, f.Struct.Name)
			}
		}
	}

	for _, s := range u.Symbols {
		if a.scope.byName[s.Name] != s || s.broken {
			continue
		}

		if s.Internal != nil {
			a.procedureAccess(s, report)

			continue
		}

		if s.private || s.Generic != nil || explicitInterface(s) != nil {
			continue
		}

		if st := symbolStruct(s); st != nil && st.Name != "" && a.typePrivate(st) {
			report("ACCESSTYPE", s.Pos, s.Name, st.Name)
		}
	}

	for _, n := range u.Namelists {
		if n.private {
			continue
		}

		for _, it := range n.Items {
			if it.private {
				report("ACCESSNML", n.Pos, n.Name, it.Name)

				break
			}
		}
	}
}

// accessFinding is a message accessChecks reports.
type accessFinding struct {
	ident string
	pos   diag.Position
	args  []any
}

// procedureAccess checks a module procedure with an argument or value of a
// private type (ACCESSPROC).
func (a *analyzer) procedureAccess(s *Symbol, report func(string, diag.Position, ...any)) {
	pu := s.Internal

	what, st := "", (*Structure)(nil)

	for _, p := range pu.Params {
		if t := symbolStruct(p); t != nil && t.Name != "" && a.typePrivate(t) {
			what, st = "the argument "+p.Name, t

			break
		}
	}

	if st == nil && pu.Result != nil {
		if t := symbolStruct(pu.Result); t != nil && t.Name != "" && a.typePrivate(t) {
			what, st = "its value", t
		}
	}

	if st != nil && (!s.private || a.publicGeneric(s)) {
		report("ACCESSPROC", s.Pos, s.Name, what, st.Name)
	}
}

// publicGeneric reports whether the procedure s is a specific of a public
// generic name or operator of the module.
func (a *analyzer) publicGeneric(s *Symbol) bool {
	for _, g := range a.unit.Symbols {
		if g.Generic != nil && !g.private && a.scope.byName[g.Name] == g && slices.Contains(g.Generic.Specifics, s) {
			return true
		}
	}

	for key, g := range a.unit.operators {
		if !a.unit.access.privateOps[key] && slices.Contains(g.Specifics, s) {
			return true
		}
	}

	return false
}

// insideModule reports whether this unit is the module name or lies in it: a
// module procedure, or one of its internal procedures.
func (a *analyzer) insideModule(name string) bool {
	for u := a.unit; u != nil; u = u.Host {
		if u.Kind == ast.ModuleUnit && u.Name == name {
			return true
		}
	}

	return false
}

// privateComponents reports a component or constructor of the type st outside
// its module, when its components are private (COMPPRIVATE), and reports
// whether it did.
func (a *analyzer) privateComponents(st *Structure, pos diag.Position) bool {
	if st == nil || !st.PrivateComponents || a.insideModule(st.Module) {
		return false
	}

	a.report("COMPPRIVATE", pos, st.Name, st.Module)

	return true
}
