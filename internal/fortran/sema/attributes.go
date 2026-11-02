package sema

import (
	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/lex"
)

// SAVE, EXTERNAL, and INTRINSIC (sections 4.14, 4.7, 4.9): statements that
// say what a name is, or ask that it be kept.
//
// SAVE asks that a name keep its value between calls. Every variable here is
// static storage (Decision 10 of docs/PHASE-74 - FORTRAN plan.md), and "the
// definitions of data entities are retained automatically by VAX FORTRAN"
// (section 4.14's note), so SAVE changes nothing; only its list is checked.
//
// EXTERNAL says a name is a routine outside the unit, even if it is also an
// intrinsic function's name, and INTRINSIC that it is FORTRAN's own. Their
// uses as procedure arguments are arguments.go's. An EXTERNAL name's other use is
// BLOCK DATA's (section 4.1): naming a block data subprogram makes LINK find it
// in a library, since the object module refers to it even if no code calls it.

// attributeStatement reads an EXTERNAL or INTRINSIC statement in the
// declarations walk, since what they say changes how every later statement
// reads the name, and keeps a SAVE, VOLATILE, AUTOMATIC, or STATIC statement for
// when every declaration is known (saves, storage.go).
func (a *analyzer) attributeStatement(s *ast.AttrStmt) {
	switch s.Kind {
	case lex.Save:
		a.saveStmts = append(a.saveStmts, s)

	case lex.External:
		for _, item := range s.Items {
			a.external(item)
		}

	case lex.Intrinsic:
		for _, item := range s.Items {
			a.intrinsic(item)
		}

	case lex.Volatile:
		a.volatileStmts = append(a.volatileStmts, s)

	case lex.Automatic, lex.Static:
		a.storageStmts = append(a.storageStmts, s)

		for _, item := range s.Items {
			if item.Array.Dims != nil {
				a.dimension(a.declare(item.Name, item.Pos), item.Array)
			}
		}
	}
}

// external makes the name a routine outside the unit. A dummy argument may be
// one, a procedure the caller passes; the unit's own name may not, nor
// anything that has storage or a value.
func (a *analyzer) external(item *ast.AttrItem) {
	s := a.declare(item.Name, item.Pos)

	switch {
	case s.External || s.Class == Intrinsic:
		a.report("DUPATTR", item.Pos, s.Name, a.describe(s))
	case s.Class == UnitName || s.Result || s.Class == Constant || s.Class == StmtFunction || s.Class == Array ||
		s.Common != nil || s.Struct != nil:
		a.badUse(s, item.Pos, "be named in an EXTERNAL statement")

		s.broken = true

	default:
		s.External = true
		s.ownExternal = !a.inLibrary

		if s.Dummy == 0 {
			s.Class = External
		}
	}
}

// Linked reports whether the MACRO file should name an EXTERNAL name in a
// .EXTRN, which makes LINK find it. A program's own EXTERNAL statement always
// does, used or not, since that is how a BLOCK DATA unit gets linked. One from
// a text library's module does only if the unit uses the name: $SYSSRVNAM names
// every system service, and a unit calls few (docs/PHASE-83 - FORTRAN and
// VMS.md, Decision 5).
func (s *Symbol) Linked() bool {
	return s.External && s.Dummy == 0 && (s.ownExternal || s.passed || s.Class != External)
}

// UnusedFromLibrary reports whether the name is a variable a text library's
// module declares and the unit never uses, which the storage map leaves out.
func (s *Symbol) UnusedFromLibrary() bool {
	return s.library && s.onlyDeclared
}

// intrinsic makes the name FORTRAN's own function of that name, which it must
// have.
func (a *analyzer) intrinsic(item *ast.AttrItem) {
	s := a.declare(item.Name, item.Pos)

	switch {
	case !intrinsics[item.Name]:
		a.report("BADINTRINSIC", item.Pos, item.Name)

		s.broken = true

	case s.External || s.Class == Intrinsic:
		a.report("DUPATTR", item.Pos, s.Name, a.describe(s))

	case s.Dummy != 0 || s.Class == UnitName || s.Result || s.Class == Constant || s.Class == StmtFunction ||
		s.Class == Array || s.Common != nil || s.Struct != nil:
		a.badUse(s, item.Pos, "be named in an INTRINSIC statement")

		s.broken = true

	default:
		s.Class = Intrinsic
	}
}

// saves checks the unit's SAVE statements' lists (section 4.14): common blocks
// the unit declares, but not blank common; variables and arrays, but not names
// in a common block, procedures, or dummy arguments. A name in one is a
// variable from here on, so a use of it as a function is NOTARRAY.
func (a *analyzer) saves() {
	for _, st := range a.saveStmts {
		for _, item := range st.Items {
			if item.Common {
				a.saveBlock(item)

				continue
			}

			s := a.declare(item.Name, item.Pos)

			switch {
			case s.broken:

			case s.Dummy != 0:
				a.report("SAVEDUMMY", item.Pos, s.Name)

			case s.Common != nil:
				a.report("SAVECOMMON", item.Pos, s.Name, s.Common.String())

			case s.Class == UnitName || s.Result || s.Class == Constant || s.Class == StmtFunction ||
				s.Class == External || s.Class == Intrinsic || s.Struct != nil:
				// Section 4.13 excludes records, though their storage is
				// static anyway.
				a.badUse(s, item.Pos, "be named in a SAVE statement")

			default:
				s.saved = true

				if s.Class == Unclassified {
					s.Class = Variable
				}
			}
		}
	}
}

// saveBlock checks a common block in a SAVE statement's list. That the unit
// must declare it is the standard's rule, which the manual doesn't state.
func (a *analyzer) saveBlock(item *ast.AttrItem) {
	if item.Name == "" {
		a.report("SAVEBLANK", item.Pos)

		return
	}

	a.noteOther(whatCommon, item.Name, item.Pos, RefDefined)

	for _, b := range a.unit.Commons {
		if b.Name == item.Name {
			return
		}
	}

	a.report("SAVEBLOCK", item.Pos, item.Name)
}

// blockDataBody is the statements of a block data subprogram that may be in
// one (section 4.1), the others being reported (BLOCKSTMT) and left out, since
// a unit with no code has no meaning for them.
func (a *analyzer) blockDataBody(body []ast.Stmt) []ast.Stmt {
	kept := make([]ast.Stmt, 0, len(body))

	for _, s := range body {
		switch s.(type) {
		case *ast.CommonStmt, *ast.DataStmt, *ast.DimensionStmt, *ast.EquivalenceStmt, *ast.ImplicitStmt,
			*ast.ParameterStmt, *ast.RecordStmt, *ast.TypeStmt, *ast.StructureStmt, *ast.EndStmt, *ast.BadStmt:
			kept = append(kept, s)

			continue

		case *ast.AttrStmt:
			if s.Base().Kind == lex.Save {
				kept = append(kept, s)

				continue
			}
		}

		if s.Base().Kind == lex.Directive {
			kept = append(kept, s)

			continue
		}

		a.report("BLOCKSTMT", s.Position(), aStatement(s))
	}

	return kept
}
