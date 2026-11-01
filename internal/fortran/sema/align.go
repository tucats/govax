package sema

import (
	"errors"
	"math/bits"
	"strings"

	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
)

// Alignment (/[NO]ALIGN and CDEC$ OPTIONS). VAX FORTRAN V5.0 packs its
// structures and common blocks: a field or a common name starts where the one
// before it ends, so a longword may sit at an odd address, which the VAX reaches
// more slowly. A later VAX FORTRAN could align them instead, by the FORTRAN
// command's /ALIGN and by groups of declarations in the source:
//
//	CDEC$ OPTIONS /ALIGN=RECORDS=NATURAL
//	      STRUCTURE /S/
//	        LOGICAL*1 FLAG        at 0
//	        INTEGER*4 COUNT       at 4, not 1
//	      END STRUCTURE           8 bytes, not 5
//	CDEC$ END OPTIONS
//
// A group's setting holds from its OPTIONS to its END OPTIONS, and changes only
// the classes it names, so groups nest. The rules are the HP Fortran manual's
// 14.14 (docs/PHASE-82 - FORTRAN DEC extensions.md, Decision 8); the boundaries,
// which that manual leaves to one we don't have, are ours (naturalBoundary).
//
// The setting where a declaration stands decides its layout: a structure's where
// its STRUCTURE statement is, a common name's where its COMMON statement is. So a
// structure declared packed stays packed wherever a RECORD statement names it.

// AlignRule is how a class of data is placed.
type AlignRule int

const (
	// Packed is on any byte, V5.0's rule and the default.
	Packed AlignRule = iota

	// Natural is on the item's natural boundary (naturalBoundary).
	Natural

	// Standard is on its natural boundary, but at most a longword. It applies
	// only to common blocks.
	Standard
)

// Alignment is the setting at one place in a source file. The zero value is the
// default, /NOALIGN.
type Alignment struct {
	// Records is the rule for structures' fields, and Commons for names in
	// common blocks.
	Records, Commons AlignRule

	// Warn is /WARN=ALIGNMENT: an informational message for each field or
	// common name not on its natural boundary. It is off unless asked for,
	// since under V5.0's packing nearly every structure would get one (ours).
	Warn bool
}

// matchKeyword is the keyword of list that word abbreviates, or "" if none or
// more than one: keywords may be cut short, as DCL's are (the manual's
// /ALIGN=RECO=NATU).
func matchKeyword(word string, list ...string) string {
	found := ""

	for _, k := range list {
		if word == k {
			return k
		}

		if word != "" && strings.HasPrefix(k, word) {
			if found != "" {
				return ""
			}

			found = k
		}
	}

	return found
}

// WithAlign is the setting with /ALIGN's values applied: each ALL, NONE, a rule
// alone (every class), or class=rule. No values is ALL. The classes a value
// doesn't name keep their rules.
func (al Alignment) WithAlign(values []string) (Alignment, error) {
	if len(values) == 0 {
		al.Records, al.Commons = Natural, Natural
	}

	for _, v := range values {
		var err error

		if al, err = al.withValue(v); err != nil {
			return al, err
		}
	}

	return al, nil
}

// WithNoAlign is the setting with /NOALIGN applied: PACKED for both classes.
func (al Alignment) WithNoAlign() Alignment {
	al.Records, al.Commons = Packed, Packed

	return al
}

// withValue applies one of /ALIGN's values.
func (al Alignment) withValue(value string) (Alignment, error) {
	class, word, hasClass := strings.Cut(value, "=")
	if !hasClass {
		word, class = class, ""
	}

	if !hasClass {
		switch matchKeyword(word, "ALL", "NONE", "NATURAL", "PACKED", "STANDARD") {
		case "ALL", "NATURAL":
			al.Records, al.Commons = Natural, Natural
		case "NONE", "PACKED":
			al.Records, al.Commons = Packed, Packed
		case "STANDARD":
			return al, errors.New("STANDARD needs a class: /ALIGN=COMMONS=STANDARD")
		default:
			return al, errors.New("/ALIGN takes ALL, NONE, a rule, or class=rule, not " + word)
		}

		return al, nil
	}

	var rule AlignRule

	switch matchKeyword(word, "PACKED", "NATURAL", "STANDARD") {
	case "PACKED":
		rule = Packed
	case "NATURAL":
		rule = Natural
	case "STANDARD":
		rule = Standard
	default:
		return al, errors.New("a rule is PACKED, NATURAL, or STANDARD, not " + word)
	}

	switch matchKeyword(class, "COMMONS", "RECORDS", "STRUCTURES") {
	case "COMMONS":
		al.Commons = rule
	case "RECORDS", "STRUCTURES":
		if rule == Standard {
			return al, errors.New("STANDARD applies only to COMMONS")
		}

		al.Records = rule
	default:
		return al, errors.New("a class is COMMONS, RECORDS, or STRUCTURES, not " + class)
	}

	return al, nil
}

// withQualifier applies one option of a CDEC$ OPTIONS directive.
func (al Alignment) withQualifier(q *ast.Qualifier) (Alignment, error) {
	switch matchKeyword(q.Name, "ALIGN", "NOALIGN", "WARN") {
	case "ALIGN":
		return al.WithAlign(q.Values)

	case "NOALIGN":
		if q.Values != nil {
			return al, errors.New("/NOALIGN takes no value")
		}

		return al.WithNoAlign(), nil

	case "WARN":
		if len(q.Values) != 1 {
			return al, errors.New("/WARN takes ALIGNMENT or NOALIGNMENT")
		}

		switch matchKeyword(q.Values[0], "ALIGNMENT", "NOALIGNMENT") {
		case "ALIGNMENT":
			al.Warn = true
		case "NOALIGNMENT":
			al.Warn = false
		default:
			return al, errors.New("/WARN takes ALIGNMENT or NOALIGNMENT, not " + q.Values[0])
		}

		return al, nil
	}

	return al, errors.New("an option is /[NO]ALIGN or /WARN, not /" + q.Name)
}

// maxAlignGroups is how deep groups may nest (the HP manual's 100 levels).
const maxAlignGroups = 100

// alignGroup is an open group: the setting its END OPTIONS goes back to, and
// where its OPTIONS is.
type alignGroup struct {
	saved Alignment
	pos   diag.Position
}

// optionsDirective reads a CDEC$ OPTIONS or END OPTIONS directive among the
// unit's declarations. A group must be before the executable part, but a late
// directive still counts, so that its END OPTIONS isn't reported as well. It
// can't begin or end inside a structure declaration, whose layout has one rule
// (ours; the manual doesn't say).
func (a *analyzer) optionsDirective(s *ast.DirectiveStmt) {
	if a.structDepth > 0 {
		a.messages.Report("OPTIONSSTRUCT", s.Position())

		return
	}

	if a.group >= execGroup {
		a.messages.Report("OPTIONSLATE", s.Position())
	}

	if s.Name == endOptions {
		if len(a.alignGroups) == 0 {
			a.messages.Report("NOOPTIONS", s.Position())

			return
		}

		last := len(a.alignGroups) - 1
		a.align, a.alignGroups = a.alignGroups[last].saved, a.alignGroups[:last]

		return
	}

	if len(a.alignGroups) == maxAlignGroups {
		a.messages.Report("OPTIONSDEPTH", s.Position(), maxAlignGroups)
	}

	a.alignGroups = append(a.alignGroups, alignGroup{a.align, s.Position()})

	for _, q := range s.Attrs {
		al, err := a.align.withQualifier(q)
		if err != nil {
			a.messages.Report("OPTIONSBAD", q.Pos, err.Error())

			continue
		}

		a.align = al
	}
}

// applyPrologueGroups reads the OPTIONS and END OPTIONS directives before the unit's
// first statement. The manual puts a group after a SUBROUTINE, FUNCTION, or BLOCK
// DATA statement; before a main program's first statement there is nothing for
// it to come after, and its list doesn't name PROGRAM (ours).
func (a *analyzer) applyPrologueGroups() {
	for _, s := range a.prologueGroups {
		switch a.unit.Kind {
		case ast.FunctionUnit, ast.SubroutineUnit, ast.BlockDataUnit:
			a.messages.Report("OPTIONSPLACE", s.Position(), a.unit.Kind.String())
		default:
			a.optionsDirective(s)
		}
	}
}

// checkAlignGroups reports the groups the unit's END leaves open: a group is
// within its unit.
func (a *analyzer) checkAlignGroups() {
	for _, g := range a.alignGroups {
		a.messages.Report("OPTIONSOPEN", g.pos)
	}
}

// naturalBoundary is the boundary a value of type t is naturally on (Decision 8):
// its length, at most a quadword, so that a REAL*16 is on 8; a COMPLEX on an
// octaword, the author's choice (answer 2), past the manual's "up to 64-bit"; a
// CHARACTER on any byte. A record's is its structure's (Structure.Boundary), not
// its type's, so this is for what isn't one.
func naturalBoundary(t Type) int {
	switch t.Kind {
	case Character:
		return 1
	case Complex:
		return 16
	}

	return min(max(t.Size, 1), 8)
}

// boundary is the field's natural boundary, an array field's being its element's.
func (f *Field) boundary() int {
	if f.Struct != nil {
		return f.Struct.Boundary
	}

	return naturalBoundary(f.Type)
}

// boundary is the name's natural boundary, an array's being its element's.
func (s *Symbol) boundary() int {
	if s.Struct != nil {
		return s.Struct.Boundary
	}

	return naturalBoundary(s.Type)
}

// alignUp is offset moved up to a multiple of boundary.
func alignUp(offset, boundary int) int {
	return (offset + boundary - 1) / boundary * boundary
}

// alignPower is a boundary, a power of two, as its power: 16 is 4.
func alignPower(boundary int) int {
	return bits.TrailingZeros(uint(boundary))
}

// warnFields reports the structure's fields not on their natural boundaries, if
// the setting asks for it. A %FILL field holds no data to be slow to reach.
func (a *analyzer) warnFields(st *Structure) {
	if !a.align.Warn {
		return
	}

	for _, f := range st.Fields {
		if f.Name != "%FILL" && f.Offset%f.boundary() != 0 {
			a.report("ALIGNFIELD", f.Pos, f.Name, f.Offset, f.boundary())
		}
	}
}

// raiseCommonAlignments gives each block's program section the alignment its
// layout needs (Decision 8), the most any unit's does, since a file has one
// section for the block. checkPsects has already compared what the directives
// said; the layouts are compared by checkCommonLayouts.
func raiseCommonAlignments(prog *Program) {
	most := map[string]int{}

	for _, u := range prog.Units {
		for _, b := range u.Commons {
			most[b.Psect] = max(most[b.Psect], b.LayoutAlign)
		}
	}

	for _, u := range prog.Units {
		for _, b := range u.Commons {
			b.Attrs.Align = max(b.Attrs.Align, most[b.Psect])
		}
	}
}

// checkCommonLayouts warns of a block two of the file's units declare alike but
// lay out differently, which only different alignment settings can do. Units may
// give a block different names and types, so a block declared differently says
// nothing (ours).
func checkCommonLayouts(prog *Program, messages *diag.List) {
	first := map[string]*CommonBlock{}

	for _, u := range prog.Units {
		for _, b := range u.Commons {
			f := first[b.Psect]
			if f == nil {
				first[b.Psect] = b

				continue
			}

			if declaredAlike(f, b) && !placedAlike(f, b) {
				messages.Report("COMMONLAYOUT", b.NamePos, b.String(), f.NamePos.Line)
			}
		}
	}
}

// declaredAlike reports whether two units' views of a block list the same types
// and sizes in the same order, whatever their names.
func declaredAlike(x, y *CommonBlock) bool {
	if len(x.Members) != len(y.Members) {
		return false
	}

	for i, s := range x.Members {
		t := y.Members[i]

		if s.Type != t.Type || s.Elements() != t.Elements() || (s.Struct == nil) != (t.Struct == nil) {
			return false
		}

		if s.Struct != nil && s.Struct.form() != t.Struct.form() {
			return false
		}
	}

	return true
}

// placedAlike reports whether the same members of two blocks, declaredAlike, are
// at the same places.
func placedAlike(x, y *CommonBlock) bool {
	for i, s := range x.Members {
		if s.Offset != y.Members[i].Offset {
			return false
		}
	}

	return true
}
