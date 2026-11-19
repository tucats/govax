package sema

import (
	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
	"github.com/tucats/govax/internal/fortran/lex"
)

// Fortran 90's attributes (the standard's 5.1 and 5.2; docs/planning/PHASE-100 -
// Fortran 90 declarations and kinds.md, finding 11). An attribute is a fact
// about a name that a type declaration's list can give it (REAL, SAVE,
// DIMENSION(10) :: X) as well as a statement of its own (SAVE X, DIMENSION
// X(10)). The standard's constraints on which attributes may go together
// are written in its prose; here they are one table, attrTable, a row per
// attribute, which every check reads:
//
//   - excludes: the attributes it may not join, on one name;
//   - fate: what this compiler does with it now. Most are compiled as their
//     old statements are; INTENT, OPTIONAL, and TARGET are recorded on the
//     Symbol and mean nothing yet (a dummy is passed as before; their phases,
//     103 and 107, give them meaning); ALLOCATABLE is allocate.go's;
//     POINTER is pointer.go's;
//     PUBLIC and PRIVATE belong in a module (access.go), and anywhere else
//     they are an error.
//
// A name's attributes come from wherever they were given, old statements
// included (attrsOf), so INTEGER, PARAMETER :: N = 1 after SAVE N is
// reported. Only a Fortran 90 form is checked against the table: two old
// statements keep VAX FORTRAN's checks, so no old program hears anything new.

// attribute is one attribute, a bit, so that a name's attributes are a set.
type attribute uint32

const (
	attrParameter attribute = 1 << iota
	attrDimension
	attrSave
	attrExternal
	attrIntrinsic
	attrVolatile
	attrAutomatic
	attrStatic
	attrIntent
	attrOptional
	attrTarget
	attrPointer
	attrAllocatable
	attrPublic
	attrPrivate
)

// attrFate is what the compiler does with an attribute in this phase.
type attrFate int

const (
	attrCompiled   attrFate = iota // compiled as its old statement is
	attrRecorded                   // kept on the Symbol, meaning nothing yet
	attrNotYet                     // NOTIMPL, waiting for its phase
	attrModuleOnly                 // an error outside a module
)

// attrRow is one attribute's row of the table.
type attrRow struct {
	name     string
	excludes attribute
	fate     attrFate
}

// attrTable is the standard's constraints of 5.1 as a table. A pair is
// excluded if either row names the other. DEC's VOLATILE, AUTOMATIC, and
// STATIC follow the HP manual: AUTOMATIC and STATIC exclude each other and
// SAVE, and none goes on a constant or a procedure.
var attrTable = map[attribute]attrRow{
	attrParameter: {"PARAMETER", attrSave | attrExternal | attrIntrinsic | attrIntent | attrOptional | attrTarget |
		attrPointer | attrAllocatable | attrVolatile | attrAutomatic | attrStatic, attrCompiled},
	attrDimension: {"DIMENSION", attrExternal | attrIntrinsic, attrCompiled},
	attrSave:      {"SAVE", attrExternal | attrIntrinsic | attrIntent | attrOptional | attrAutomatic, attrCompiled},
	attrExternal: {"EXTERNAL", attrIntrinsic | attrIntent | attrTarget | attrPointer | attrAllocatable |
		attrVolatile | attrAutomatic | attrStatic, attrCompiled},
	attrIntrinsic: {"INTRINSIC", attrIntent | attrOptional | attrTarget | attrPointer | attrAllocatable |
		attrVolatile | attrAutomatic | attrStatic, attrCompiled},
	attrVolatile:    {"VOLATILE", 0, attrCompiled},
	attrAutomatic:   {"AUTOMATIC", attrStatic, attrCompiled},
	attrStatic:      {"STATIC", 0, attrCompiled},
	attrIntent:      {"INTENT", attrPointer, attrRecorded},
	attrOptional:    {"OPTIONAL", 0, attrRecorded},
	attrTarget:      {"TARGET", attrPointer, attrRecorded},
	attrPointer:     {"POINTER", attrAllocatable, attrCompiled},
	attrAllocatable: {"ALLOCATABLE", 0, attrCompiled},
	attrPublic:      {"PUBLIC", attrPrivate, attrModuleOnly},
	attrPrivate:     {"PRIVATE", 0, attrModuleOnly},
}

// attrNamed is each attribute by its keyword, made from attrTable.
var attrNamed = func() map[string]attribute {
	m := map[string]attribute{}
	for a, row := range attrTable {
		m[row.name] = a
	}

	return m
}()

// attrOfStatement is the attribute each attribute statement gives.
var attrOfStatement = map[lex.Kind]attribute{
	lex.Save: attrSave, lex.External: attrExternal, lex.Intrinsic: attrIntrinsic, lex.Volatile: attrVolatile,
	lex.Automatic: attrAutomatic, lex.Static: attrStatic, lex.Intent: attrIntent, lex.Optional: attrOptional,
	lex.Target: attrTarget, lex.Dimension: attrDimension, lex.Parameter: attrParameter,
	lex.Allocatable: attrAllocatable, lex.Pointer: attrPointer,
}

// excluded reports whether attributes x and y may not go together.
func excluded(x, y attribute) bool {
	return attrTable[x].excludes&y != 0 || attrTable[y].excludes&x != 0
}

// attrsOf is the attributes s has so far: those recorded as statements gave
// them, and those its other facts say it has.
func attrsOf(s *Symbol) attribute {
	set := s.attrs

	if s.Class == Constant {
		set |= attrParameter
	}

	if s.Dims != nil {
		set |= attrDimension
	}

	if s.External {
		set |= attrExternal
	}

	if s.Class == Intrinsic {
		set |= attrIntrinsic
	}

	return set
}

// conflict reports, and returns true, if s can't be given x for an
// attribute it has already.
func (a *analyzer) conflict(s *Symbol, x attribute, pos diag.Position) bool {
	has := attrsOf(s)

	for y := range attrTable {
		if has&y != 0 && excluded(x, y) {
			a.report("ATTRCONFLICT", pos, s.Name, attrTable[y].name, attrTable[x].name)

			return true
		}
	}

	return false
}

// attrSpecs is a type declaration's attribute list read against the table:
// the attributes to give each entity, in the list's order, with each one's
// specification (DIMENSION's bounds, INTENT's spec). An attribute the list
// gives twice (ATTRTWICE), two that can't go together (ATTRPAIR), one not
// compiled yet (NOTIMPL), and PUBLIC or PRIVATE outside a module
// (NOTINMODULE) are reported here, once for the statement, and left out.
func (a *analyzer) attrSpecs(list []*ast.AttrSpec) []*ast.AttrSpec {
	var (
		kept []*ast.AttrSpec
		seen attribute
	)

	for _, spec := range list {
		x := attrNamed[spec.Name]

		switch row := attrTable[x]; {
		case seen&x != 0:
			a.report("ATTRTWICE", spec.Pos, row.name)

			continue

		case row.fate == attrNotYet:
			a.notImplForm(spec.Pos, "the "+row.name+" attribute")

		case row.fate == attrModuleOnly && a.unit.Kind == ast.ModuleUnit:
			// access.go's.

		case row.fate == attrModuleOnly:
			a.report("NOTINMODULE", spec.Pos, "the "+row.name+" attribute")
		}

		pair := false

		for _, k := range kept {
			if y := attrNamed[k.Name]; excluded(x, y) {
				a.report("ATTRPAIR", spec.Pos, attrTable[y].name, attrTable[x].name)

				pair = true

				break
			}
		}

		seen |= x

		moduleOnly := attrTable[x].fate == attrModuleOnly && a.unit.Kind != ast.ModuleUnit

		if !pair && attrTable[x].fate != attrNotYet && !moduleOnly {
			kept = append(kept, spec)
		}
	}

	return kept
}

// giveAttributes gives the entity d of a Fortran 90 type declaration s, whose
// symbol is sym, the attributes of its list (attrSpecs's), each as the
// attribute's old statement would, so each has its old checks: SAVE on a
// dummy argument is SAVEDUMMY. PARAMETER is the caller's, which needs the
// entity's value. A name that has an attribute excluding one of these
// (ATTRCONFLICT) doesn't get it.
func (a *analyzer) giveAttributes(s *ast.TypeStmt, sym *Symbol, d *ast.Declarator, specs []*ast.AttrSpec) {
	item := &ast.AttrItem{Name: d.Name, Pos: d.Pos}

	for _, spec := range specs {
		x := attrNamed[spec.Name]

		if x == attrParameter || a.conflict(sym, x, d.Pos) {
			continue
		}

		switch x {
		case attrDimension:
			if d.Dims == nil {
				a.dimension(sym, &ast.Declarator{Name: d.Name, Pos: d.Pos, Dims: spec.Dims})
			}

		case attrExternal:
			a.external(item)

		case attrIntrinsic:
			a.intrinsic(item)

		case attrSave, attrVolatile, attrAutomatic, attrStatic:
			a.attributeLater(s, x, item)

			sym.f90Saved = sym.f90Saved || x == attrSave

		case attrIntent, attrOptional, attrTarget, attrAllocatable, attrPointer:
			a.recordAttribute(sym, x, spec.Intent, d.Pos)

		case attrPublic, attrPrivate:
			a.giveAccess(sym.Name, x == attrPrivate, d.Pos)
		}

		sym.attrs |= x
	}
}

// attributeLater keeps SAVE, VOLATILE, AUTOMATIC, or STATIC for a name, given
// by s's list, with the unit's statements of that kind, which are read when
// every declaration is known: as a statement of that kind naming the one
// name, at s.
func (a *analyzer) attributeLater(s *ast.TypeStmt, x attribute, item *ast.AttrItem) {
	kinds := map[attribute]lex.Kind{attrSave: lex.Save, attrVolatile: lex.Volatile, attrAutomatic: lex.Automatic, attrStatic: lex.Static}

	st := &ast.AttrStmt{StmtBase: s.StmtBase, Items: []*ast.AttrItem{item}}
	st.Kind = kinds[x]

	switch x {
	case attrSave:
		a.saveStmts = append(a.saveStmts, st)
	case attrVolatile:
		a.volatileStmts = append(a.volatileStmts, st)
	default:
		a.storageStmts = append(a.storageStmts, st)
	}
}

// recordAttribute records INTENT (with its spec), OPTIONAL, TARGET,
// ALLOCATABLE, or POINTER for a name: kept on the Symbol, for their phases
// (103, 104, 107). INTENT and OPTIONAL
// are a dummy argument's alone (ATTRDUMMY); none may be given twice.
func (a *analyzer) recordAttribute(s *Symbol, x attribute, intent string, pos diag.Position) {
	name := attrTable[x].name

	switch {
	case s.attrs&x != 0:
		a.report("ATTRAGAIN", pos, s.Name, name)

		return

	case (x == attrIntent || x == attrOptional) && s.Dummy == 0:
		a.report("ATTRDUMMY", pos, s.Name, name)

		return
	}

	switch x {
	case attrIntent:
		s.Intent = intent
	case attrOptional:
		s.Optional = true
	case attrTarget:
		s.Target = true

		if s.Class == Unclassified {
			s.Class = Variable
		}

	case attrAllocatable:
		s.Allocatable = true

	case attrPointer:
		s.Pointer = true

		if s.Class == Unclassified && s.Dims == nil {
			s.Class = Variable
		}
	}
}

// attributeItems reads INTENT, OPTIONAL, TARGET, ALLOCATABLE, and POINTER
// statements whole, and
// for the others checks a Fortran 90 statement's names (one with ::, SAVE
// :: X) against the table, and records each name's attribute. It returns the
// items the statement's old reading is to go on with: none for the three, and
// for the others all but those a conflict was reported for. An old
// statement's names are only recorded, and only names already known: VOLATILE
// may name a NAMELIST group, which is no symbol.
func (a *analyzer) attributeItems(s *ast.AttrStmt) []*ast.AttrItem {
	x := attrOfStatement[s.Kind]
	recorded := attrTable[x].fate == attrRecorded || x == attrAllocatable || x == attrPointer
	checked := s.Colons || recorded

	var kept []*ast.AttrItem

	for _, item := range s.Items {
		if item.Common {
			kept = append(kept, item)

			continue
		}

		var sym *Symbol

		switch {
		case checked:
			sym = a.declare(item.Name, item.Pos)
		default:
			sym = a.scope.find(item.Name)
		}

		switch {
		case sym == nil:
			kept = append(kept, item)

		case checked && a.conflict(sym, x, item.Pos):

		case recorded:
			a.recordAttribute(sym, x, s.Intent, item.Pos)

			if item.Array != nil && item.Array.Dims != nil {
				a.dimension(sym, item.Array)
			}

			sym.attrs |= x

		default:
			sym.attrs |= x
			sym.f90Saved = sym.f90Saved || s.Colons && x == attrSave
			kept = append(kept, item)
		}
	}

	if recorded {
		return nil
	}

	return kept
}
