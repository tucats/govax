package sema

import (
	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/lex"
)

// VOLATILE (section 4.16, with the HP manual's 5.19) changes no storage: it says
// that a name's value can change, or be read, in ways the unit can't see, so
// that an optimizer must leave every reference in place. Only its list is checked
// here. AUTOMATIC and STATIC (the HP manual's 5.3) do say where a name goes.
// AUTOMATIC's goes in the stack, in an area each entry point reserves in the
// routine's call frame, reached through FP, so each call has its own and its value
// is undefined when the call starts. STATIC's goes where a variable goes anyway,
// so it only has its list checked.
//
// That doesn't make a routine reentrant: the compiler's own temporaries, DO
// counts, and descriptors stay in $LOCAL (Phase 82's survey). The frame area is
// groundwork for a later phase that moves them too.
//
// EQUIVALENCE ties names to one storage, so it spreads both attributes: an
// AUTOMATIC name's group is in the frame area together (ours: the manual says
// nothing, and a group can't be in two places), and a VOLATILE name's group is
// all volatile (the manual's P1 and P4).

// storageStatements reads the unit's AUTOMATIC and STATIC statements, once every
// declaration is known and each EQUIVALENCE group is whole.
func (a *analyzer) storageStatements() {
	for _, st := range a.storageStmts {
		for _, item := range st.Items {
			a.storageName(st, item)
		}
	}

	for _, s := range a.unit.Symbols {
		if s.Automatic && s.group != nil {
			a.spreadAutomatic(s)
		}
	}
}

// storageName checks one name in st's list, an AUTOMATIC or STATIC statement's.
// A name in one of these is a variable from here on, as a name in a SAVE
// statement is.
func (a *analyzer) storageName(st *ast.AttrStmt, item *ast.AttrItem) {
	s := a.lookup(item.Name, item.Pos)
	statement := st.Kind.String()
	automatic := st.Kind == lex.Automatic

	switch {
	case s.broken:

	case s.storageStmt != nil:
		a.report("DUPSTORAGE", item.Pos, s.Name, s.storageStmt.Kind.String(), s.storageStmt.Pos.Line)

	case s.Dummy != 0:
		a.report("STORAGEDUMMY", item.Pos, s.Name, statement)

	case s.Class == UnitName || s.Result || s.Class == Constant || s.Class == StmtFunction ||
		s.Class == External || s.Class == Intrinsic || s.External:
		a.badUse(s, item.Pos, "be named in "+aStatement(st))

	case automatic && s.Common != nil:
		a.report("AUTOCOMMON", item.Pos, s.Name, s.Common.String())

	case automatic && s.saved:
		a.report("AUTOSAVE", item.Pos, s.Name)

	default:
		s.storageStmt = st

		if automatic {
			s.Automatic, s.automaticPos = true, item.Pos
		}

		if s.Class == Unclassified {
			s.Class = Variable
		}
	}
}

// spreadAutomatic makes the rest of the AUTOMATIC name s's EQUIVALENCE group
// AUTOMATIC too, or reports the member that can't be. placeInCommon has given a
// group with a name in a common block every member's Common.
func (a *analyzer) spreadAutomatic(s *Symbol) {
	for _, m := range s.group.members {
		switch {
		case m == s || m.broken:
		case m.Common != nil:
			a.report("AUTOCOMMON", s.automaticPos, s.Name, m.Common.String())
		case m.saved:
			a.report("AUTOSAVE", s.automaticPos, m.Name)
		case m.storageStmt != nil && m.storageStmt.Kind == lex.Static:
			a.report("AUTOEQUIVALENT", s.automaticPos, s.Name, m.Name)
		default:
			m.Automatic = true
			m.automaticPos = s.automaticPos
		}
	}
}

// checkAutomaticInitials reports each AUTOMATIC name given an initial value, by
// DATA, a type statement, or its structure: the value is in the image, in static
// storage, which an AUTOMATIC name isn't. The HP manual has such a name static
// instead; the survey chose the error, since a name in an AUTOMATIC statement
// plainly isn't meant to be.
func (a *analyzer) checkAutomaticInitials() {
	seen := map[*Symbol]bool{}

	for _, in := range a.unit.Initials {
		if in.Sym.Automatic && !seen[in.Sym] {
			seen[in.Sym] = true
			a.report("AUTOINIT", in.Sym.automaticPos, in.Sym.Name)
		}
	}
}

// volatiles reads the unit's VOLATILE statements. A name that isn't declared
// anything else is a variable from here on, unless it names a NAMELIST group.
func (a *analyzer) volatiles() {
	for _, st := range a.volatileStmts {
		for _, item := range st.Items {
			if item.Common {
				a.volatileBlock(item)
			} else {
				a.volatileName(item)
			}
		}
	}
}

// volatileName marks one name and the names EQUIVALENCE ties to it. A record's
// fields are volatile with it. A function's value, a procedure, and a namelist
// group can't be named (the HP manual's 5.19), nor a constant, which has no
// storage.
func (a *analyzer) volatileName(item *ast.AttrItem) {
	if a.unit.byName[item.Name] == nil && a.unit.namelists[item.Name] != nil {
		a.report("VOLATILEGROUP", item.Pos, item.Name)

		return
	}

	s := a.lookup(item.Name, item.Pos)

	switch {
	case s.broken:

	case s.Class == UnitName || s.Result || s.Class == Constant || s.Class == StmtFunction ||
		s.Class == External || s.Class == Intrinsic || s.External:
		a.badUse(s, item.Pos, "be named in a VOLATILE statement")

	default:
		if s.Class == Unclassified {
			s.Class = Variable
		}

		markVolatile(s)
	}
}

// markVolatile marks s, and the names of its EQUIVALENCE group.
func markVolatile(s *Symbol) {
	s.Volatile = true

	if s.group != nil {
		for _, m := range s.group.members {
			m.Volatile = true
		}
	}
}

// volatileBlock marks every name of a named common block the unit declares,
// those EQUIVALENCE puts in it among them, and the names tied to those.
func (a *analyzer) volatileBlock(item *ast.AttrItem) {
	if item.Name == "" {
		a.report("VOLATILEBLANK", item.Pos)

		return
	}

	for _, b := range a.unit.Commons {
		if b.Name != item.Name {
			continue
		}

		for _, s := range a.unit.Symbols {
			if s.Common == b {
				markVolatile(s)
			}
		}

		return
	}

	a.report("SAVEBLOCK", item.Pos, item.Name)
}

// layout gives each of the unit's variables and arrays not in a common
// block (common.go) its place in the unit's static storage, $LOCAL
// (Decision 10 of docs/PHASE-74 - FORTRAN plan.md): in order of first
// appearance, each on its type's natural boundary (Type.Align: a
// longword's on a multiple of 4, a LOGICAL*1 or a CHARACTER variable
// anywhere), and a group of names EQUIVALENCE ties together
// (equivalence.go) as a unit when its first name is reached. The size is
// rounded up to a longword, so that the compiler's own cells, which
// follow, are aligned.
// Storage is static because FORTRAN-77's routines aren't recursive
// (section 6.2), so a routine's variables are never live in two calls at
// once, and a variable keeps its value from one call to the next as VAX
// FORTRAN's do.
//
// A dummy argument has no storage: it is the caller's, reached through
// the argument list, as a function's value is when it goes through a hidden
// argument (Type.HiddenResult). Nor has a routine's name. A name only
// declared, never used, is a variable, given storage as VAX FORTRAN gives it.
//
// An AUTOMATIC name is placed the same way in the frame area instead, whose
// size is kept in Unit.FrameSize; each entry point's code reserves it.
func (a *analyzer) layout() {
	offset, frame := 0, 0

	// A function with ENTRY statements has one place for all its names'
	// values, unless they go through a hidden argument.
	shared := len(a.unit.Entries) > 0 && a.unit.Result != nil && !a.unit.Result.Type.HiddenResult()
	if shared {
		offset = a.placeResults()
	}

	for _, s := range a.unit.Symbols {
		if s.Class == Unclassified {
			s.Class, s.onlyDeclared = Variable, true
		}

		if s.Dummy != 0 || s.Common != nil || (s.Class != Variable && s.Class != Array) {
			continue
		}

		if s.Result && (shared || s.Type.HiddenResult()) {
			continue
		}

		place := &offset
		if s.Automatic {
			place = &frame
		}

		if s.group != nil {
			if !s.group.placed {
				*place = s.group.place(*place)
			}

			continue
		}

		align := s.Type.Align()

		// A record of a structure laid out by RECORDS=NATURAL goes on the
		// structure's boundary, so that its fields are aligned as addresses.
		// FP is only on a longword, so a frame can't do that past a longword
		// (ours, unconfirmed).
		if !s.Automatic && s.Struct != nil && s.Struct.Natural && s.Struct.Boundary > align {
			align = s.Struct.Boundary

			if power := alignPower(align); power > 2 {
				a.unit.LocalAlign = max(a.unit.LocalAlign, power)
			}
		}

		s.Offset = (*place + align - 1) / align * align
		*place = s.Offset + s.Size()
	}

	a.unit.LocalSize = (offset + 3) &^ 3
	a.unit.FrameSize = (frame + 3) &^ 3
}
