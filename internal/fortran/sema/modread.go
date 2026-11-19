package sema

import (
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
	"github.com/tucats/govax/internal/fortran/lex"
	"github.com/tucats/govax/internal/fortran/parse"
	"github.com/tucats/govax/internal/fortran/source"
)

// Reading a module's text (finding 7; modwrite.go writes it). A USE of M finds
// M's text, from this compilation or from M's module file, and reads it with
// the compiler's own passes: the source pass (free form, Fortran 90), the
// lexer, the parser, and semantic analysis of its one unit, the module, in a
// mode that makes no code. Before that, the text's !GOVAX$ lines are read into
// moduleFacts, which semantic analysis consults where Fortran's declarations
// can't say enough: each variable's place (applyLayoutFacts), each type's
// layout (readTypeLayout), the module's floating format. A text that doesn't
// read cleanly, or whose facts disagree with what analysis computes, is
// MODFILEBAD, naming the line: a file from another version of govax, or edited.

// moduleFacts are a module text's !GOVAX$ lines, by what each describes.
type moduleFacts struct {
	format    string
	gFloating bool

	// blocks are the program sections' sizes and alignments, by section, and
	// at each variable's place, by name.
	blocks map[string]blockFact
	at     map[string]atFact

	// types are each structure's layout, by name.
	types map[string]typeFact

	// folded are the REAL and COMPLEX constants whose values were computed,
	// not written (FOLDED lines).
	folded map[string]bool
}

// blockFact is a PSECT or COMMON line: a section's size and alignment.
type blockFact struct {
	size, align, line int
}

// atFact is an AT line: a variable's offset and size.
type atFact struct {
	offset, size, line int
}

// typeFact is a TYPE line: a structure's size, boundary, and whether it is
// naturally aligned.
type typeFact struct {
	size, boundary, line int
	natural              bool
}

// readFacts reads a text's !GOVAX$ lines, or reports what is wrong with one
// and the line it is on.
func readFacts(lines []string) (*moduleFacts, string, int) {
	f := &moduleFacts{blocks: map[string]blockFact{}, at: map[string]atFact{}, types: map[string]typeFact{}, folded: map[string]bool{}}

	for i, line := range lines {
		rest, ok := strings.CutPrefix(line, "!GOVAX$ ")
		if !ok {
			continue
		}

		n := i + 1
		w := strings.Fields(rest)

		number := func(i int) int {
			if i >= len(w) {
				return -1
			}

			v, err := strconv.Atoi(w[i])
			if err != nil {
				return -1
			}

			return v
		}

		bad := false

		switch {
		case len(w) == 0:
			bad = true
		case w[0] == "MODULE" && len(w) == 3 && w[1] == "FILE":
			f.format = w[2]
		case w[0] == "FLOATING" && len(w) == 2:
			f.gFloating = w[1] == "G"
		case (w[0] == "PSECT" || w[0] == "COMMON") && len(w) == 6:
			f.blocks[w[1]] = blockFact{size: number(3), align: number(5), line: n}
			bad = number(3) < 0 || number(5) < 0
		case w[0] == "AT" && len(w) == 4:
			f.at[w[1]] = atFact{offset: number(2), size: number(3), line: n}
			bad = number(2) < 0 || number(3) < 0
		case w[0] == "FOLDED" && len(w) == 2:
			f.folded[w[1]] = true
		case w[0] == "TYPE" && len(w) == 7:
			f.types[w[1]] = typeFact{size: number(3), boundary: number(5), natural: w[6] == "NATURAL", line: n}
			bad = number(3) < 0 || number(5) < 0
		}

		if bad {
			return nil, "the line \"" + line + "\" isn't one govax writes", n
		}
	}

	if f.format != moduleFormat {
		return nil, "it isn't a module file of this version of govax", 1
	}

	return f, "", 0
}

// moduleScope finds and reads the module name for a USE at pos, once per
// compilation: from this compilation's own modules, or by the finder from a
// module file. It reports MODCYCLE for a module that uses itself, MODLATER for
// one later in the file, MODNOTFOUND for one with no file, and MODFILEBAD for a
// text it can't read, and then returns nil.
func (a *analyzer) moduleScope(name string, pos diag.Position) *moduleScope {
	m := a.prog.modules

	if ms := m.loaded[name]; ms != nil {
		if ms.failed {
			return nil
		}

		return ms
	}

	failed := func(ident string, args ...any) *moduleScope {
		a.report(ident, pos, args...)
		m.loaded[name] = &moduleScope{name: name, failed: true}

		return nil
	}

	if name == m.current {
		return failed("MODCYCLE", name)
	}

	var (
		lines []string
		spec  string
	)

	switch mt, at := m.compiled[name], m.later[name]; {
	case mt != nil:
		lines, spec = mt.Lines, name+".F90$MOD"

	case at.Line != 0:
		a.report("MODLATER", pos, name, at.Line)

		return nil

	case m.finder != nil:
		found, where, ok, looked := m.finder.Find(name)
		if !ok {
			return failed("MODNOTFOUND", name, lookedText(looked))
		}

		lines, spec = found, where

	default:
		return failed("MODNOTFOUND", name, "nowhere: this compilation searches for no module files")
	}

	ms, why, line := m.read(name, lines, spec)
	if ms == nil {
		return failed("MODFILEBAD", spec, line, why)
	}

	m.loaded[name] = ms

	return ms
}

// lookedText is the places a search looked, for MODNOTFOUND.
func lookedText(looked []string) string {
	if len(looked) == 0 {
		return "nowhere"
	}

	return strings.Join(looked, ", ")
}

// read reads module name's text, from spec, into the scope a USE reaches; or
// returns nil, why not, and the line.
func (m *modules) read(name string, lines []string, spec string) (*moduleScope, string, int) {
	facts, why, line := readFacts(lines)
	if facts == nil {
		return nil, why, line
	}

	var msgs diag.List

	src := &source.File{Spec: spec, Lines: lines, Free: true}
	prog := source.Read(source.Form{F90: true, Continuations: len(lines) + 1}, src, nil, &msgs)
	tree := parse.Parse(lex.Lex(prog.Statements, &msgs), &msgs)

	if msgs.Failed() {
		why, line := firstError(msgs)

		return nil, why, line
	}

	if len(tree.Units) != 1 || tree.Units[0].Kind() != ast.ModuleUnit || tree.Units[0].Name() != name {
		return nil, "it doesn't hold the module " + name + " alone", 1
	}

	au := tree.Units[0]

	opts := m.opts
	opts.GFloating = facts.gFloating

	defaultInt, defaultLogical := IntegerType, LogicalType

	a := &analyzer{
		messages:       &msgs,
		prog:           &Program{modules: m},
		units:          unitNames(tree, name, &msgs),
		defaultInt:     defaultInt,
		defaultLogical: defaultLogical,
		gFloating:      facts.gFloating,
		f90:            true,
		warnings:       opts.Warnings,
		checks:         opts.Checks,
		reading:        facts,
	}
	a.makeMaps()
	a.analyzeUnit(au, name)

	if msgs.Failed() {
		why, line := firstError(msgs)

		return nil, why, line
	}

	ms := &moduleScope{name: name, unit: a.unit, gFloating: facts.gFloating}

	for _, s := range a.unit.Symbols {
		if s.Value != nil && facts.folded[s.Name] {
			s.Value.literal = false
		}
	}

	for _, b := range a.unit.Commons {
		if b.Module != "" {
			ms.block = b
		}
	}

	return ms, "", 0
}

// firstError is the first error of a reading, its text and line.
func firstError(msgs diag.List) (string, int) {
	for _, msg := range msgs.Messages() {
		if msg.Severity >= diag.Error {
			return msg.Text, msg.Pos.Line
		}
	}

	return "", 0
}

// applyLayoutFacts gives each section and variable of a module read from its
// text the size and place its facts say, after layoutCommon has placed them by
// the reader's own rules; a variable with no AT line, or one whose size isn't
// the line's, means the text and its declarations disagree (MODFACT).
func (a *analyzer) applyLayoutFacts() {
	f := a.reading

	for _, b := range a.unit.Commons {
		bf, ok := f.blocks[b.Psect]
		if !ok {
			a.report("MODFACT", b.NamePos, "no PSECT or COMMON line for "+b.Psect)

			continue
		}

		b.Size, b.LayoutAlign = bf.size, bf.align
	}

	for _, s := range a.unit.Symbols {
		if s.Common == nil {
			continue
		}

		at, ok := f.at[s.Name]

		switch {
		case !ok:
			a.report("MODFACT", s.Pos, "no AT line for "+s.Name)
		case at.size != s.Size():
			a.report("MODFACT", s.Pos, s.Name+" has "+strconv.Itoa(s.Size())+" bytes, not the AT line's "+strconv.Itoa(at.size))
		case at.offset+at.size > s.Common.Size:
			a.report("MODFACT", s.Pos, s.Name+" ends past its section's end")
		default:
			s.Offset = at.offset
		}
	}
}

// readNatural is whether the structure named name of a module's text is
// naturally aligned, by its TYPE line, or natural if the text isn't being read.
func (a *analyzer) readNatural(name string, natural bool) bool {
	if a.reading == nil {
		return natural
	}

	if tf, ok := a.reading.types[name]; ok {
		return tf.natural
	}

	return natural
}

// readTypeLayout checks a structure of a module's text against its TYPE line
// (MODFACT): its size and boundary must be the line's.
func (a *analyzer) readTypeLayout(st *Structure) {
	if a.reading == nil {
		return
	}

	tf, ok := a.reading.types[st.Name]

	switch {
	case !ok:
		a.report("MODFACT", st.Pos, "no TYPE line for "+st.Name)
	case tf.size != st.Size || tf.boundary != st.Boundary:
		a.report("MODFACT", st.Pos, "the layout of "+st.Name+" isn't its TYPE line's")
	}
}
