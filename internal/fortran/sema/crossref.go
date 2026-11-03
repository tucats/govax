package sema

import (
	"slices"
	"strconv"

	"github.com/tucats/govax/internal/fortran/diag"
)

// The cross reference (docs/PHASE-85 - FORTRAN compiler usability.md, Decision 13):
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
	whatIntrinsic = "intrinsic"
	whatSystem    = "system routine"
)

// note adds a mention to list. A statement from a text library's module makes none.
func (a *analyzer) note(list *[]Reference, pos diag.Position, kind RefKind) {
	if !a.inLibrary {
		*list = append(*list, Reference{pos.Line, pos.Column, pos.Serial, kind})
	}
}

// declare is lookup for the name a specification statement says something about.
func (a *analyzer) declare(name string, pos diag.Position) *Symbol {
	s := a.lookup(name, pos)
	a.note(&s.refs, pos, RefDefined)

	return s
}

// noteAssigned notes that the statement gives the place e a value.
func (a *analyzer) noteAssigned(e Expr) {
	switch e := e.(type) {
	case *Var:
		a.note(&e.Sym.refs, e.Pos, RefAssigned)
	case *Elem:
		a.note(&e.Sym.refs, e.Pos, RefAssigned)
	case *ArrayRef:
		a.note(&e.Sym.refs, e.Pos, RefAssigned)
	case *FieldRef:
		a.note(&e.Sym.refs, e.NamePos, RefAssigned)
	case *Substring:
		a.noteAssigned(e.X)
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

	if s := a.unit.byName[name]; s != nil {
		a.note(&s.refs, pos, RefUsed)
	} else {
		a.noteOther(whatIntrinsic, name, pos, RefUsed)
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
		if refs := settled(s.refs); len(refs) > 0 && s.Class != UnitName {
			group, what := u.describeCross(s)
			rows = append(rows, row{group: group, name: CrossName{Name: s.Name, What: what, Refs: refs}})
		}
	}

	for key, list := range u.others {
		if refs := settled(list); len(refs) > 0 {
			group, name := crossOthers, key.name

			switch key.what {
			case whatIntrinsic, whatSystem:
				group = crossRoutines
			case whatCommon:
				name = (&CommonBlock{Name: name}).String()
			case whatStructure:
				name = "/" + name + "/"
			}

			rows = append(rows, row{group: group, name: CrossName{Name: name, What: key.what, Refs: refs}})
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

// describeCross is which group the name's row is in, and what it says the name is.
func (u *Unit) describeCross(s *Symbol) (int, string) {
	text := s.TypeText()

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
