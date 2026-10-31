package sema

import (
	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
)

// ENTRY statements (section 6.2.4). An ENTRY statement gives a subprogram
// another name to be called by, with a dummy argument list of its own, and
// execution starts at the first executable statement after it. The statement
// does nothing when control reaches it from above.
//
// Every dummy argument of every entry point is one name in the unit, so a
// type statement or a DIMENSION statement that declares it may come before or
// after the ENTRY statement naming it. So entries reads the ENTRY statements
// before the unit's declarations, as header reads the header statement. A name
// in two lists is one symbol, whose place in the argument list differs from
// one entry point to another; lowering (ir's ENTRY code) makes that work.

// Entry is an entry point an ENTRY statement makes.
type Entry struct {
	// AST is the ENTRY statement.
	AST *ast.EntryStmt

	// Sym is the entry point's name: in a function a variable, the value
	// of the function called by that name; in a subroutine a name that
	// stands for nothing else.
	Sym *Symbol

	// Params are the entry point's dummy arguments, in its list's order.
	Params []*Symbol
}

// EntryPoint is an ENTRY statement among the unit's statements, where the
// entry point's code starts.
type EntryPoint struct {
	stmtBase
	Entry *Entry
}

// entries enters the names of the unit's ENTRY statements and their dummy
// arguments. The names all come first, since no entry point's name may be
// another's dummy argument, whichever statement is first.
func (a *analyzer) entries(body []ast.Stmt) {
	var list []*Entry

	for _, st := range body {
		s, ok := st.(*ast.EntryStmt)
		if !ok {
			continue
		}

		if a.unit.Kind != ast.FunctionUnit && a.unit.Kind != ast.SubroutineUnit {
			a.report("ENTRYUNIT", s.Pos)

			continue
		}

		if e := a.entryName(s); e != nil {
			list = append(list, e)
		}
	}

	for _, e := range list {
		a.entryParams(e)
	}

	a.unit.Entries = list
}

// entryName enters the name of the ENTRY statement s, and returns its entry
// point: nil if the name isn't free, which is reported (DUPUNIT, by
// unitNames, if it is the name of a unit or of an earlier entry point).
func (a *analyzer) entryName(s *ast.EntryStmt) *Entry {
	sym := a.lookup(s.Name, s.Pos)

	switch {
	case sym.Dummy != 0:
		a.badUse(sym, s.Pos, "be an entry point's name")

		return nil
	case sym.Class == UnitName || sym.Result:
		return nil
	}

	if a.unit.Kind == ast.FunctionUnit {
		sym.Class, sym.Result = Variable, true
	} else {
		sym.Class = UnitName
	}

	sym.laterEntry = s

	e := &Entry{AST: s, Sym: sym}
	a.entryOf[s] = e

	if fu := a.units[s.Name]; fu != nil && fu.pos == s.Pos {
		fu.unit, fu.entry = a.unit, e
	}

	return e
}

// entryParams enters an entry point's dummy arguments. A name no list before
// this one has can't be used in the statements before this one (laterEntry).
func (a *analyzer) entryParams(e *Entry) {
	seen := map[*Symbol]bool{}

	for _, p := range e.AST.Params {
		if p.Name == "*" {
			if a.unit.Kind == ast.FunctionUnit {
				a.report("ENTRYRETURN", p.Pos)
			} else {
				// As params says of a * in a header.
				a.notImplForm(p.Pos, "an alternate return")
				a.unit.incomplete = true
			}

			continue
		}

		s := a.lookup(p.Name, p.Pos)

		switch {
		case s.Class == UnitName || s.Result:
			a.badUse(s, p.Pos, "be a dummy argument")
		case seen[s]:
			a.report("DUPDUMMY", p.Pos, p.Name, "ENTRY statement")
		default:
			seen[s] = true
			e.Params = append(e.Params, s)

			if s.Dummy == 0 {
				a.addDummy(s)
				s.laterEntry = e.AST
			}
		}
	}
}

// entryPoint analyzes an ENTRY statement where the statement walk reaches it:
// it can't be in a block (section 6.2.4), and from here on its names can be
// used.
func (a *analyzer) entryPoint(s *ast.EntryStmt) {
	e := a.entryOf[s]

	if c := a.top(); c != nil {
		a.report("ENTRYBLOCK", s.Pos, c.what(), c.line())
	}

	if e == nil {
		return
	}

	e.Sym.laterEntry = nil

	for _, p := range e.Params {
		if p.laterEntry == s {
			p.laterEntry = nil
		}
	}

	a.add(&EntryPoint{stmtBase{s}, e})
}

// usedBeforeEntry reports a name used in a statement before the ENTRY
// statement that first names it, which section 6.2.4 forbids for entry names
// and for dummy arguments. Once for each name.
func (a *analyzer) usedBeforeEntry(s *Symbol, pos diag.Position) {
	if s.laterEntry == nil {
		return
	}

	a.report("ENTRYLATE", pos, s.Name, s.laterEntry.Pos.Line)
	s.laterEntry = nil
}

// checkEntryTypes checks a function's entry points' types against the
// function's (section 6.2.4.1): all in one of the groups that can share one
// place for the value, and if CHARACTER, all of one length.
func (a *analyzer) checkEntryTypes() {
	f := a.unit.Result
	if f == nil || f.broken {
		return
	}

	for _, e := range a.unit.Entries {
		s := e.Sym
		if s.broken || s.Type.Kind == Invalid || f.Type.Kind == Invalid {
			continue
		}

		same := valueGroup(s.Type) == valueGroup(f.Type)
		if s.Type.Kind == Character && s.Type != f.Type {
			same = false
		}

		if !same {
			a.report("ENTRYTYPE", e.AST.Pos, s.Name, s.Type.String(), f.Name, f.Type.String())
		}
	}
}

// valueGroup is the group of section 6.2.4.1 a type is in: 1 is what fits in
// R0 and R1, 2 what goes through a hidden argument by reference, and 3
// CHARACTER, through a hidden descriptor.
func valueGroup(t Type) int {
	switch {
	case t.Kind == Character:
		return 3
	case t.HiddenResult():
		return 2
	}

	return 1
}

// placeResults gives a function's names (its own and its entry points') their
// one place at the start of its storage, as long as the longest of them, and
// returns the first offset after it: defining one defines the others of its
// type (section 6.2.4.1), so they overlap as EQUIVALENCE would have them.
func (a *analyzer) placeResults() int {
	size := a.unit.Result.Size()
	a.unit.Result.Offset, a.unit.Result.Equivalenced = 0, true

	for _, e := range a.unit.Entries {
		size = max(size, e.Sym.Size())
		e.Sym.Offset, e.Sym.Equivalenced = 0, true
	}

	return size
}
