package sema

import (
	"slices"
	"strconv"

	"github.com/tucats/govax/internal/fortran/diag"
)

// What /WARNINGS's own keywords look for (diag.Warnings), each a look at a unit once
// its statements have been read.

// needsType reports whether the name must have a type of its own or from its first
// letter: a variable, an array, a function, a statement function, a dummy argument. A
// subroutine, a program unit's name, and an EXTERNAL or INTRINSIC name no reference
// makes a function need none.
func needsType(s *Symbol) bool {
	return !s.Typed && !s.broken && s.Class != Subroutine && s.Class != UnitName && s.Class != Intrinsic &&
		(!s.External || s.Class == Function)
}

// undeclared warns of each name that takes its type from its first letter, as
// /WARNINGS=DECLARATIONS asks (the note in section 4.8): the names IMPLICIT NONE would
// refuse (untyped). Under IMPLICIT NONE that says so itself, and no name has a first
// letter's type. An old-form PARAMETER's name has its value's type, not a first letter's.
// A text library's name isn't the program's to declare.
func (a *analyzer) undeclared() {
	if !a.warnings.Declarations || a.implicit.none {
		return
	}

	for _, s := range a.unit.Symbols {
		if needsType(s) && !s.oldForm && !s.library {
			a.report("UNDECLARED", s.Pos, s.Name, s.Type.String())
		}
	}
}

// reportUnused warns of each name only declared and each label nothing refers to, as
// /WARNINGS=UNUSED asks, in the order they come in the source. After an error it says
// nothing, since a statement with an error may have lost its mentions, and a name
// would then look unused when it isn't.
func (a *analyzer) reportUnused() {
	if !a.warnings.Unused || a.messages.Failed() {
		return
	}

	type finding struct {
		ident string
		pos   diag.Position
		name  string
	}

	var findings []finding

	for _, s := range a.unit.Symbols {
		if !a.neverUsed(s) {
			continue
		}

		ident := "UNUSEDNAME"
		if s.Dummy != 0 {
			ident = "UNUSEDDUMMY"
		}

		findings = append(findings, finding{ident, s.Pos, s.Name})
	}

	for number, label := range a.unit.Labels {
		if onlyDefined(a.unit.labelRefs[number]) {
			findings = append(findings, finding{"UNUSEDLABEL", label.Stmt.Base().Source.LabelPosition(), strconv.Itoa(number)})
		}
	}

	slices.SortFunc(findings, func(x, y finding) int {
		if x.pos.Serial != y.pos.Serial {
			return x.pos.Serial - y.pos.Serial
		}

		return x.pos.Column - y.pos.Column
	})

	for _, f := range findings {
		a.report(f.ident, f.pos, f.name)
	}
}

// neverUsed reports whether every mention of the name only says what it is, and it
// isn't one of the names that needs no more. All of these are ours, no manual saying:
//
//   - A name in a common block or an EQUIVALENCE list has its place in storage that
//     something else reaches.
//   - A name an INCLUDE file or a text library declares belongs to a file many units share,
//     each using some of it.
//   - An EXTERNAL name is how a program gets a BLOCK DATA unit linked (Symbol.Linked), so
//     it does its work unused, but a dummy argument said to be EXTERNAL doesn't.
//   - A name a statement function's dummy argument shares may be declared only to give
//     that dummy its type.
//   - A unit's own names, the function's value and its ENTRY names among them, aren't
//     for it to use.
func (a *analyzer) neverUsed(s *Symbol) bool {
	if s.Class == UnitName || s.Result || s.broken || s.library || s.included || s.typesDummy ||
		s.Common != nil || s.Equivalenced || (s.External && s.Dummy == 0) {
		return false
	}

	for _, e := range a.unit.Entries {
		if e.Sym == s {
			return false
		}
	}

	return onlyDefined(s.refs)
}

// onlyDefined reports whether every mention of a name or label is its definition.
func onlyDefined(refs []Reference) bool {
	for _, r := range settled(refs) {
		if r.Kind != RefDefined {
			return false
		}
	}

	return true
}
