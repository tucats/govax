package sema

import (
	"slices"
	"strconv"

	"github.com/tucats/govax/internal/fortran/diag"
)

// The cross reference (docs/planning/PHASE-85 - FORTRAN compiler usability.md, Decision 13):
// where each of a unit's names and labels is defined, assigned, and used.
//
// Semantic analysis is the one pass that knows what each name is, so it notes each
// mention as it goes, rather than a second walk working that out again. Every name
// comes through lookup, which notes a use; a declaration's name goes through
// declare, and a place given a value through noteAssigned. A place is often looked at
// more than once (a constant expression is evaluated twice, an input item tried two
// ways), so a mention is its line and column, and CrossReference keeps one per place,
// the strongest kind said of it.
//
// A dummy argument of a statement function isn't listed: it is a name for the length
// of one line, in no table of the unit (stmtfunc.go), and a use of it is no use of
// the variable that shares its name. An implied-DO variable in a DATA statement is
// the same. Nor is a program unit's own name, or a record's field, which belongs to
// its structure. A name a text library's module declares appears only if the program
// uses it, with those uses: the program would have thousands of rows for one INCLUDE
// otherwise, each pointing into a file it never sees (ours).
//
// A use-associated name (docs/planning/PHASE-106 - Fortran 90 modules.md, finding
// 13) says where it came from after what it is, "INTEGER from CONSTS", and is
// listed by the same rule as a library's: if the unit mentions it, or a USE's ONLY
// list or rename names it (Symbol.Unmentioned). Each module a USE names has a row of
// its own, the USE statements' lines.

// RefKind is what a mention of a name does.
type RefKind int

// The kinds of mention, weakest first.
const (
	RefUsed     RefKind = iota // its value is used, or it is passed along
	RefAssigned                // the statement gives it a value
	RefDefined                 // a specification statement says what it is
)

// Reference is one mention of a name or label. Line is its number in its file, as a
// listing shows it, and Serial which reading of a line that is, since a file included
// twice has each line twice (source.Line.Serial).
type Reference struct {
	Line, Column, Serial int
	Kind                 RefKind
}

// CrossName is one row of a unit's cross reference. Label says it is a statement
// label, its name the label's number; What says what it is, as a listing shows it.
type CrossName struct {
	Name, What string
	Label      bool
	Refs       []Reference
}

// otherName is a name that has no Symbol, and What is what it is.
type otherName struct {
	what, name string
}

// The rows' groups, in the order a cross reference lists them: within each by name,
// and labels by number.
const (
	crossVariables = iota
	crossConstants
	crossRoutines
	crossOthers
	crossLabels
)

// What's said of the names with no Symbol.
const (
	whatCommon    = "common block"
	whatNamelist  = "namelist"
	whatStructure = "structure"
	whatType      = "derived type"
	whatModule    = "module"
	whatIntrinsic = "intrinsic"
	whatSystem    = "system routine"
	whatOperator  = "operator"
	whatAssign    = "assignment"
)

// operatorUse is where a USE gave a unit an operator or assignment: the module
// and the USE statement (use.go's useOperator).
type operatorUse struct {
	module string
	pos    diag.Position
}

// note adds a mention to list. A statement from a text library's module makes none.
func (a *analyzer) note(list *[]Reference, pos diag.Position, kind RefKind) {
	if a.inLibrary {
		return
	}

	*list = append(*list, Reference{pos.Line, pos.Column, pos.Serial, kind})

	if a.prog.lines == nil {
		a.prog.lines = map[int]diag.Position{}
	}

	if _, known := a.prog.lines[pos.Serial]; !known {
		pos.Column = 0
		a.prog.lines[pos.Serial] = pos
	}
}

// noteThrough notes a use of the specific procedure s made by a generic name, an
// operator, or defined assignment at pos (Symbol.through).
func (a *analyzer) noteThrough(s *Symbol, pos diag.Position) {
	if a.inLibrary {
		return
	}

	a.note(&s.refs, pos, RefUsed)
	s.through = append(s.through, s.refs[len(s.refs)-1])
}

// noteOperator notes a use of the operator or assignment key that called the
// specific procedure spec: a row of this unit's, named as the interface block or
// USE that has it names it, from that host if it isn't this unit's.
func (a *analyzer) noteOperator(key string, spec *Symbol, pos diag.Position) {
	what := whatOperator
	if key == assignmentKey {
		what = whatAssign
	}

	for u := a.unit; u != nil; u = u.Host {
		if g := u.operators[key]; g != nil && slices.Contains(g.Specifics, spec) {
			// The host's row has the use too, as a name's has its contained
			// procedures' mentions.
			if u != a.unit {
				if u.others == nil {
					u.others = map[otherName][]Reference{}
				}

				refs := u.others[otherName{what, g.Name}]
				a.note(&refs, pos, RefUsed)
				u.others[otherName{what, g.Name}] = refs
				what += " from " + u.Path()
			}

			a.noteOther(what, g.Name, pos, RefUsed)

			return
		}
	}
}

// mention is the position of the mention r, for a message about it: its file and
// line, with the line's text to quote.
func (a *analyzer) mention(r Reference) diag.Position {
	pos := a.prog.lines[r.Serial]
	pos.Line, pos.Column, pos.Serial = r.Line, r.Column, r.Serial

	return pos
}

// declare is lookup for the name a specification statement says something about.
func (a *analyzer) declare(name string, pos diag.Position) *Symbol {
	s := a.local(name, pos)
	a.respecified(s, pos)
	a.note(&s.refs, pos, RefDefined)

	return s
}

// noteAssigned notes that the statement gives the place e a value.
func (a *analyzer) noteAssigned(e Expr) {
	a.notePlace(e, RefAssigned)
}

// noteRead notes that the statement reads the place e, for a place analyzed as a
// target (analyzer.target), which notes nothing itself: an internal file READ reads.
func (a *analyzer) noteRead(e Expr) {
	a.notePlace(e, RefUsed)
}

// notePlace notes a mention of the kind given of the place e.
func (a *analyzer) notePlace(e Expr, kind RefKind) {
	// An array pointer component's symbol is in no table: its record is
	// what the cross reference shows (pointercomp.go).
	if s := baseOf(e); s != nil && s.Component != nil {
		a.notePlace(s.Component, kind)

		return
	}

	switch e := e.(type) {
	case *Var:
		a.note(&e.Sym.refs, e.Pos, kind)
	case *Elem:
		a.note(&e.Sym.refs, e.Pos, kind)
	case *ArrayRef:
		a.note(&e.Sym.refs, e.Pos, kind)
	case *Section:
		a.note(&e.Sym.refs, e.Pos, kind)
	case *FieldRef:
		a.note(&e.Sym.refs, e.NamePos, kind)
	case *Substring:
		a.notePlace(e.X, kind)
	}
}

// noteOther notes a mention of a name that has no Symbol.
func (a *analyzer) noteOther(what, name string, pos diag.Position, kind RefKind) {
	if a.unit.others == nil {
		a.unit.others = map[otherName][]Reference{}
	}

	key := otherName{what, name}
	refs := a.unit.others[key]
	a.note(&refs, pos, kind)
	a.unit.others[key] = refs
}

// noteIntrinsic notes a reference to the intrinsic function name: a mention of its
// Symbol if the unit has declared it, or of the function.
func (a *analyzer) noteIntrinsic(name string, pos diag.Position) {
	a.extensions.intrinsic(name, pos)

	if s := a.scope.find(name); s != nil {
		a.note(&s.refs, pos, RefUsed)
	} else {
		a.noteOther(whatIntrinsic, name, pos, RefUsed)
	}
}

// noteSystem notes a reference to the system routine name: a mention of its Symbol
// if the unit has declared it (REAL RAN), so that the name counts as used, or of
// the routine.
func (a *analyzer) noteSystem(name string, pos diag.Position) {
	if s := a.scope.find(name); s != nil {
		a.note(&s.refs, pos, RefUsed)
	} else {
		a.noteOther(whatSystem, name, pos, RefUsed)
	}
}

// noteLabel notes a mention of a statement label.
func (a *analyzer) noteLabel(label int, pos diag.Position, kind RefKind) {
	if a.unit.labelRefs == nil {
		a.unit.labelRefs = map[int][]Reference{}
	}

	refs := a.unit.labelRefs[label]
	a.note(&refs, pos, kind)
	a.unit.labelRefs[label] = refs
}

// CrossReference is the unit's cross reference. It is meaningful after errors too,
// as far as the names the analysis reached.
func (u *Unit) CrossReference() []CrossName {
	type row struct {
		group, number int
		name          CrossName
	}

	var rows []row

	for _, s := range u.Symbols {
		refs := settled(shown(s))
		if len(refs) == 0 || s.Class == UnitName || s.Use != nil && !s.listed && onlyDefined(refs) {
			continue
		}

		group, what := u.describeCross(s)
		if from := s.UseText(); from != "" {
			what += " " + from
		}

		rows = append(rows, row{group: group, name: CrossName{Name: s.Name, What: what, Refs: refs}})
	}

	for _, r := range u.hostNames() {
		rows = append(rows, row{group: r.group, name: r.name})
	}

	for key, list := range u.others {
		if refs := settled(list); len(refs) > 0 {
			group, name, what := crossOthers, key.name, key.what

			switch key.what {
			case whatIntrinsic, whatSystem:
				group = crossRoutines
			case whatOperator, whatAssign:
				group = crossRoutines

				if from, ok := u.operatorsFrom[name]; ok {
					refs = settled(append(slices.Clone(list), Reference{from.pos.Line, from.pos.Column, from.pos.Serial, RefDefined}))
					what += " from " + from.module
				}
			case whatCommon:
				name = (&CommonBlock{Name: name}).String()
			case whatStructure:
				name = "/" + name + "/"
			}

			rows = append(rows, row{group: group, name: CrossName{Name: name, What: what, Refs: refs}})
		}
	}

	for label, list := range u.labelRefs {
		if refs := settled(list); len(refs) > 0 {
			rows = append(rows, row{crossLabels, label, CrossName{Name: strconv.Itoa(label), What: "label", Label: true, Refs: refs}})
		}
	}

	slices.SortFunc(rows, func(x, y row) int {
		switch {
		case x.group != y.group:
			return x.group - y.group
		case x.number != y.number:
			return x.number - y.number
		case x.name.Name < y.name.Name:
			return -1
		case x.name.Name > y.name.Name:
			return 1
		}

		return 0
	})

	names := make([]CrossName, len(rows))
	for i, r := range rows {
		names[i] = r.name
	}

	return names
}

// crossRow is a cross reference row and its group.
type crossRow struct {
	group int
	name  CrossName
}

// hostNames are the rows for the names of the unit's hosts that its own
// statements mention: each a host's name, which the unit doesn't hide, with
// the mentions on the unit's own lines. A name a host has by USE says where it
// came from as the host's row does; any other says which host it is.
func (u *Unit) hostNames() []crossRow {
	var rows []crossRow

	seen := map[string]bool{}
	for _, s := range u.Symbols {
		seen[s.Name] = true
	}

	for h := u.Host; h != nil; h = h.Host {
		for _, s := range h.Symbols {
			if seen[s.Name] || s.Class == UnitName {
				continue
			}

			var mine []Reference

			for _, r := range shown(s) {
				if u.ownsLine(r.Serial) {
					mine = append(mine, r)
				}
			}

			if len(mine) == 0 {
				continue
			}

			seen[s.Name] = true

			group, what := h.describeCross(s)
			if from := s.UseText(); from != "" {
				what += " " + from
			} else {
				what += " from " + h.Path()
			}

			rows = append(rows, crossRow{group, CrossName{Name: s.Name, What: what, Refs: settled(mine)}})
		}
	}

	return rows
}

// ownsLine reports whether the line read with the serial number given is one of
// the unit's own statements' lines: from its header to its END, but not an
// internal procedure's. It answers only for a unit with a host, whose lines are
// the ones its host's names may be mentioned on.
func (u *Unit) ownsLine(serial int) bool {
	if !u.spans(serial) {
		return false
	}

	for _, c := range u.Contains {
		if c.spans(serial) {
			return false
		}
	}

	return true
}

// spans reports whether the line with the serial number given is between the
// unit's header and its END, inclusive.
func (u *Unit) spans(serial int) bool {
	if u.AST == nil || u.AST.Header == nil || len(u.AST.Body) == 0 {
		return false
	}

	return serial >= u.AST.Header.Base().Pos.Serial && serial <= u.AST.Body[len(u.AST.Body)-1].Base().Pos.Serial
}

// shown is the mentions of s its own row shows: all but those a generic name,
// an operator, or defined assignment made (Symbol.through).
func shown(s *Symbol) []Reference {
	if len(s.through) == 0 {
		return s.refs
	}

	var refs []Reference

	for _, r := range s.refs {
		if !slices.Contains(s.through, r) {
			refs = append(refs, r)
		}
	}

	return refs
}

// describeCross is which group the name's row is in, and what it says the name
// is, with its attributes after it.
func (u *Unit) describeCross(s *Symbol) (int, string) {
	group, text := u.describeKind(s)

	for _, a := range s.Attributes() {
		text += ", " + a
	}

	return group, text
}

// describeKind is which group the name's row is in, and what it says the name is.
func (u *Unit) describeKind(s *Symbol) (int, string) {
	text := s.TypeText()

	// A generic name, which may be one of its specific procedures' too.
	if s.Generic != nil {
		if slices.Contains(s.Generic.Specifics, s) && s.Class == Function {
			return crossRoutines, text + " function, generic"
		} else if slices.Contains(s.Generic.Specifics, s) {
			return crossRoutines, "subroutine, generic"
		}

		return crossRoutines, "generic"
	}

	if s.Result {
		return crossRoutines, text + " function"
	}

	switch s.Class {
	case Array:
		return crossVariables, text + " array"
	case Constant:
		return crossConstants, "constant"
	case Function:
		return crossRoutines, text + " function"
	case Subroutine:
		return crossRoutines, "subroutine"
	case StmtFunction:
		return crossRoutines, text + " statement function"
	case External:
		return crossRoutines, "external"
	case Intrinsic:
		return crossRoutines, whatIntrinsic
	}

	return crossVariables, text
}

// settled is the mentions in source order, one for each place, the strongest kind
// said of it, and then one for each line and kind: X = X * X is an assignment and a
// use, whichever way it is written.
func settled(refs []Reference) []Reference {
	sorted := slices.Clone(refs)

	slices.SortStableFunc(sorted, func(x, y Reference) int {
		switch {
		case x.Serial != y.Serial:
			return x.Serial - y.Serial
		case x.Column != y.Column:
			return x.Column - y.Column
		}

		return int(y.Kind - x.Kind)
	})

	out := sorted[:0]

	for i, r := range sorted {
		if i > 0 && sorted[i-1].Serial == r.Serial && sorted[i-1].Column == r.Column {
			continue
		}

		repeated := false

		for j := len(out) - 1; j >= 0 && out[j].Serial == r.Serial; j-- {
			repeated = repeated || out[j].Kind == r.Kind
		}

		if !repeated {
			out = append(out, r)
		}
	}

	return out
}

// noteLabels notes the unit's references to labels, once they have all been made,
// and a DO statement's own. A label's definition is noted as it is defined.
func (a *analyzer) noteLabels() {
	for _, r := range a.refs {
		a.noteLabel(r.label, r.pos, RefUsed)
	}
}

// baseOf is the symbol a Var, an Elem, an ArrayRef, or a Section refers to, or
// nil for any other expression.
func baseOf(e Expr) *Symbol {
	switch e := e.(type) {
	case *Var:
		return e.Sym
	case *Elem:
		return e.Sym
	case *ArrayRef:
		return e.Sym
	case *Section:
		return e.Sym
	}

	return nil
}
