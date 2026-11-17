package sema

// Scopes (the Fortran 90 standard's 14.1.2, "scoping units"). A name means one
// thing within its scoping unit: a program unit, and in Fortran 90 an internal
// or module procedure, a derived type definition, an interface body, and, in
// part, a statement function, whose dummy arguments are its own names while
// every other name in its expression is the unit's.
//
// A scope is one such unit's table of names. It may have a host, the scope
// it lies inside: a name the scope doesn't have is looked for in the host,
// and its host's host, outward (host association, the standard's 12.1.2.2.1).
// A program unit's scope has no host until internal and module procedures
// come (docs/planning/PHASE-99 - Fortran 90.md, Decision 4, and Phase 103);
// a statement function's scope has the unit's as its host (stmtfunc.go).
//
// Only the names of variables, constants, and procedures are in scopes. A
// unit's COMMON blocks, namelist groups, and structures keep tables of their
// own: they are other classes of names, which aren't host-associated.
//
// There are two kinds of read. find looks a name up and enters nothing;
// analyzer.lookup is the read of a name the program uses, which enters it, with
// its implicit type, if no scope in the chain has it.

// scope is one scoping unit's names.
type scope struct {
	// byName is the scope's own names.
	byName map[string]*Symbol

	// list, when the scope keeps its names in order, is where they are kept:
	// a unit's scope's is the unit's Symbols. A scope with none (a statement
	// function's) enters a new name in its host.
	list *[]*Symbol

	// implicit is the scope's letters' types (implicit.go), or nil for a
	// scope that takes its host's.
	implicit *implicitRules

	// host is the scope this one lies inside, or nil.
	host *scope
}

// newUnitScope is a program unit's scope, keeping its names in order in list,
// with the implicit rules r and no host.
func newUnitScope(list *[]*Symbol, r *implicitRules) *scope {
	return &scope{byName: map[string]*Symbol{}, list: list, implicit: r}
}

// newInnerScope is a scope inside host that enters no names of its own beyond
// those given it with define, and takes its host's implicit rules: a
// statement function's.
func newInnerScope(host *scope) *scope {
	return &scope{byName: map[string]*Symbol{}, host: host}
}

// find returns the symbol named name in this scope or the nearest host that
// has it, or nil; it enters nothing.
func (s *scope) find(name string) *Symbol {
	for sc := s; sc != nil; sc = sc.host {
		if sym := sc.byName[name]; sym != nil {
			return sym
		}
	}

	return nil
}

// define makes sym this scope's own name, hiding any of its name in a host.
func (s *scope) define(sym *Symbol) {
	s.byName[sym.Name] = sym
}

// enter makes sym, a name new to the chain, a name of the nearest scope that
// keeps its names in order: this one, or the host a statement function's
// expression shares.
func (s *scope) enter(sym *Symbol) {
	sc := s
	for sc.list == nil && sc.host != nil {
		sc = sc.host
	}

	sc.byName[sym.Name] = sym

	if sc.list != nil {
		*sc.list = append(*sc.list, sym)
	}
}

// rules are the implicit rules that hold in this scope: its own, or its
// nearest host's.
func (s *scope) rules() *implicitRules {
	sc := s
	for sc.implicit == nil && sc.host != nil {
		sc = sc.host
	}

	return sc.implicit
}
