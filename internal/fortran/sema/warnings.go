package sema

import (
	"slices"
	"strconv"

	"github.com/tucats/govax/internal/fortran/ast"
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
	if !a.warnings.Declarations || a.scope.rules().strict() {
		return
	}

	for _, s := range a.unit.Symbols {
		if needsType(s) && !s.oldForm && !s.library {
			a.report("UNDECLARED", s.Pos, s.Name, s.Type.String())
		}
	}
}

// hostHidden warns of each name an internal procedure declares as its own that its
// host has too, as /WARNINGS=DECLARATIONS asks (HOSTHIDDEN; docs/planning/PHASE-110 -
// Fortran 90 compiler usability.md, finding 5). The standard allows it (12.1.2.2.1),
// and it's often meant (a loop index of the procedure's own), but the host's name is
// then out of reach, which may not be. A dummy argument is a declaration too. Not
// reported: the procedure's own name and result, which its host has as the
// procedure's; a name its host has only as the host's own name; and a name an INCLUDE
// file or text library declares, which host and procedure may both include.
func (a *analyzer) hostHidden() {
	if !a.warnings.Declarations || a.unit.Host == nil || a.reading != nil || a.inBody || a.host == nil {
		return
	}

	for _, s := range a.unit.Symbols {
		if s.Class == UnitName || s.Result || s.broken || s.library || s.included {
			continue
		}

		// A generic interface of the procedure's own extends its host's of the
		// same name, which it hides no more than its host's specifics (14.1.2.3).
		h, sc := a.host.scope.findIn(s.Name)
		if h == nil || h == s || h.Class == UnitName || sc.unit == nil || s.Generic != nil && h.Generic != nil {
			continue
		}

		what := h.Name
		if from := h.UseText(); from != "" {
			what += " (" + from + ")"
		}

		a.report("HOSTHIDDEN", s.Pos, s.Name, sc.unit.Path(), what)
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

	for _, s := range a.unit.Symbols {
		if set, ok := a.neverRead(s); ok {
			findings = append(findings, finding{"UNREAD", a.mention(set), s.Name})
		}
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
//     something else reaches. A module's own block of its variables (moduleBlock) isn't
//     such a block: only the module's users reach it, and a private name no user can.
//   - A name an INCLUDE file or a text library declares belongs to a file many units share,
//     each using some of it.
//   - An EXTERNAL name is how a program gets a BLOCK DATA unit linked (Symbol.Linked), so
//     it does its work unused, but a dummy argument said to be EXTERNAL doesn't.
//   - A name a statement function's dummy argument shares may be declared only to give
//     that dummy its type.
//   - A unit's own names, the function's value and its ENTRY names among them, aren't
//     for it to use.
//   - A module's public names are for its users; its private ones are its own, and
//     reported. A name a USE of a whole module gave the unit with all the rest is
//     the module's, but one a USE's ONLY list or rename named is the unit's choice,
//     and reported if nothing uses it (Symbol.listed).
func (a *analyzer) neverUsed(s *Symbol) bool {
	switch {
	case a.unit.Kind == ast.ModuleUnit && !s.private:
		return false
	case s.Use != nil:
		return s.listed && onlyDefined(s.refs)
	}

	if s.Class == UnitName || s.Result || s.Internal != nil || s.broken || s.library || s.included || s.typesDummy ||
		(s.Common != nil && s.Common.Module == "") || s.Equivalenced || (s.External && s.Dummy == 0) {
		return false
	}

	for _, e := range a.unit.Entries {
		if e.Sym == s {
			return false
		}
	}

	return onlyDefined(s.refs)
}

// neverRead reports whether s is a variable given a value that nothing reads (UNREAD),
// and if so where it's first given one. It goes by the cross reference's mentions, as
// neverUsed does: in the order of the statements, not their flow, so a value read
// anywhere in the unit, or passed as an argument, counts. Left out, each because
// something other than the unit's own statements may read it (ours): a dummy argument
// or a function's value, which the caller reads; a name in COMMON (a module's
// variables among them), an EQUIVALENCE, or a NAMELIST group; a VOLATILE name; a
// pointer or a TARGET, read through an association; a name a USE gave; and a DO
// variable (loopVariable).
func (a *analyzer) neverRead(s *Symbol) (Reference, bool) {
	if (s.Class != Variable && s.Class != Array) || s.Dummy != 0 || s.Result || s.Common != nil || s.Equivalenced || s.Volatile ||
		s.Pointer || s.Target || s.Use != nil || s.loopVariable || s.broken || s.library || s.included ||
		a.inNamelist(s) {
		return Reference{}, false
	}

	var set *Reference

	refs := settled(s.refs)

	for i, r := range refs {
		switch r.Kind {
		case RefUsed:
			return Reference{}, false
		case RefAssigned:
			if a.unit.readsToo[r] {
				return Reference{}, false
			}

			if set == nil {
				set = &refs[i]
			}
		}
	}

	if set == nil {
		return Reference{}, false
	}

	return *set, true
}

// inNamelist reports whether s is an item of a NAMELIST group of the unit or a host.
func (a *analyzer) inNamelist(s *Symbol) bool {
	for u := a.unit; u != nil; u = u.Host {
		for _, n := range u.namelists {
			if slices.Contains(n.Items, s) {
				return true
			}
		}
	}

	return false
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
