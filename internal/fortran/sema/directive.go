package sema

import (
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/fortran/ast"
	"github.com/tucats/govax/internal/fortran/diag"
	"github.com/tucats/govax/internal/fortran/lex"
)

// The general compiler directives (section 10.3). None of them generates code
// or declares a name:
//
//	CDEC$ IDENT 'V1.2'              the object module's identification
//	CDEC$ PSECT /BLK/ NOWRT,ALIGN=9 a common block's program section
//	CDEC$ TITLE 'PAYROLL'           the listing's heading (Phase 85)
//	CDEC$ SUBTITLE 'TAXES'
//
// A directive may stand anywhere: before a unit, among its statements, inside
// a structure declaration. Each is read where it stands in the declarations
// walk, or by prologue before a unit. TITLE and SUBTITLE only have their
// strings checked: the syntax tree keeps them, in place, for the listing
// (the compiler's listingHeadings).

// maxDirectiveString is the longest string IDENT, TITLE, and SUBTITLE take
// (sections 10.3.1 and 10.3.3).
const maxDirectiveString = 31

// DefaultAlign is a common block's alignment with no CDEC$ PSECT ALIGN=, as a
// power of two: a longword (Table 10-1).
const DefaultAlign = 2

// PsectAttrs are what CDEC$ PSECT can change of a common block's program
// section (Table 10-1). Two blocks with one meaning have equal PsectAttrs.
type PsectAttrs struct {
	// Local is LCL, clear for GBL: a section of this name in another
	// cluster is another section.
	Local bool

	// NoWrite is NOWRT: the program can't change the block.
	NoWrite bool

	// NoShare is NOSHR: processes can't share the block's pages.
	NoShare bool

	// Align is the block's alignment, as a power of two, 0 (a byte) to 9 (a
	// page).
	Align int
}

// String is the attributes as a message says them, those that aren't the
// defaults: "NOWRT, ALIGN=3".
func (p PsectAttrs) String() string {
	var list []string

	if p.Local {
		list = append(list, "LCL")
	}

	if p.NoWrite {
		list = append(list, "NOWRT")
	}

	if p.NoShare {
		list = append(list, "NOSHR")
	}

	if p.Align != DefaultAlign {
		list = append(list, "ALIGN="+strconv.Itoa(p.Align))
	}

	if list == nil {
		return "the defaults"
	}

	return strings.Join(list, ", ")
}

// endOptions is CDEC$ END OPTIONS's name, to the lexer, for which blanks don't
// count.
const endOptions = "ENDOPTIONS"

// directive reads one directive of the file. The first IDENT is the module's,
// since a file is one module (Decision 9; VAX FORTRAN made a module per unit,
// each with its first). A PSECT directive is returned, to be applied when its
// unit's common blocks are all known, and an OPTIONS or END OPTIONS directive,
// for its unit (align.go); it is nil for the others.
func (p *Program) directive(s *ast.DirectiveStmt, messages *diag.List) *ast.DirectiveStmt {
	if s.Name == "PSECT" || s.Name == "OPTIONS" || s.Name == endOptions {
		return s
	}

	if s.HasText && !printableString(s.Text) {
		messages.Report("DIRSTRING", s.Position(), s.Name, maxDirectiveString)
	}

	if s.Name == "IDENT" && !p.HasIdent {
		p.Ident, p.HasIdent = s.Text, true
	}

	return nil
}

// printableString reports whether text can be a directive's string: at most 31
// printable ASCII characters.
func printableString(text string) bool {
	if len(text) > maxDirectiveString {
		return false
	}

	for i := range len(text) {
		if text[i] < ' ' || text[i] > '~' {
			return false
		}
	}

	return true
}

// directive reads a directive among the unit's statements.
func (a *analyzer) directive(s *ast.DirectiveStmt) {
	d := a.prog.directive(s, a.messages)

	switch {
	case d == nil:
	case d.Name == "PSECT":
		a.psects = append(a.psects, d)
	default:
		a.optionsDirective(d)
	}
}

// psectDirectives applies the unit's PSECT directives, those before it first,
// to the blocks they name. They wait until now since a directive may come
// before the block's COMMON statement.
func (a *analyzer) psectDirectives() {
	given := map[psectChoice]string{}

	for _, s := range a.psects {
		a.noteOther(whatCommon, s.Common, s.CommonPos, RefDefined)

		var b *CommonBlock

		for _, c := range a.unit.Commons {
			if c.Name == s.Common {
				b = c
			}
		}

		if b == nil {
			// Nothing to change, but its attributes can still be wrong.
			b = &CommonBlock{Name: s.Common, Attrs: PsectAttrs{Align: DefaultAlign}}
			a.report("PSECTBLOCK", s.CommonPos, b.String())
		}

		if b.psectPos == (diag.Position{}) {
			b.psectPos = s.CommonPos
		}

		for _, q := range s.Attrs {
			a.psectAttribute(b, q, given)
		}
	}
}

// psectChoice is one thing a unit's directives choose for one block, so that
// choosing it two ways is noticed: "scope" is LCL or GBL, "write" [NO]WRT,
// "share" [NO]SHR, and "align" ALIGN=.
type psectChoice struct {
	block *CommonBlock
	what  string
}

// psectAttribute applies the attribute q to the block b. The manual forbids
// only LCL with GBL, but WRT with NOWRT, say, is no less a contradiction (ours).
func (a *analyzer) psectAttribute(b *CommonBlock, q *ast.Qualifier, given map[psectChoice]string) {
	text, what := q.Name, ""
	align := 0

	switch q.Name {
	case "LCL", "GBL":
		what = "scope"
	case "WRT", "NOWRT":
		what = "write"
	case "SHR", "NOSHR":
		what = "share"
	case "ALIGN":
		var err error

		what = "align"

		// "A constant", but a name's value isn't known before a unit, where
		// a directive may stand: a number as written (ours).
		if len(q.Values) == 1 {
			align, err = strconv.Atoi(q.Values[0])
		}

		if len(q.Values) != 1 || err != nil || align > 9 {
			a.messages.Report("PSECTALIGN", q.Pos)

			b.psectBad = true

			return
		}

		text = "ALIGN=" + strconv.Itoa(align)
	default:
		a.messages.Report("PSECTATTR", q.Pos, q.Name)

		b.psectBad = true

		return
	}

	if what != "align" && q.Values != nil {
		a.messages.Report("PSECTVALUE", q.Pos, q.Name)

		b.psectBad = true

		return
	}

	choice := psectChoice{b, what}

	if first, ok := given[choice]; ok && first != text {
		a.messages.Report("PSECTCLASH", q.Pos, first, text, b.String())

		b.psectBad = true

		return
	}

	given[choice] = text

	switch q.Name {
	case "LCL", "GBL":
		b.Attrs.Local = q.Name == "LCL"
	case "WRT", "NOWRT":
		b.Attrs.NoWrite = q.Name == "NOWRT"
	case "SHR", "NOSHR":
		b.Attrs.NoShare = q.Name == "NOSHR"
	default:
		b.Attrs.Align, b.alignGiven = align, true
	}
}

// checkPsects reports a block whose attributes differ between the file's units:
// "all other units that reference the common block must also change those
// attributes in the same way" (section 10.3.2). A file is one module with one
// section for the block, so nothing else would tell. It is each unit's result
// that counts, so GBL in one unit agrees with no directive in another (ours).
// A block whose directive had an error isn't compared.
func checkPsects(prog *Program, messages *diag.List) {
	first := map[string]*CommonBlock{}

	for _, u := range prog.Units {
		if badDirective(u.AST) {
			continue
		}

		for _, b := range u.Commons {
			if b.psectBad {
				continue
			}

			f := first[b.Psect]
			if f == nil {
				first[b.Psect] = b

				continue
			}

			if f.Attrs != b.Attrs {
				messages.Report("PSECTDIFFER", b.attrsPos(), b.String(), b.Attrs.String(), f.attrsPos().Line, f.Attrs.String())
			}
		}
	}
}

// attrsPos is where the unit gives the block its attributes: its first PSECT
// directive for it, or with none where the unit first names it.
func (b *CommonBlock) attrsPos() diag.Position {
	if b.psectPos != (diag.Position{}) {
		return b.psectPos
	}

	return b.NamePos
}

// badDirective reports whether the parser rejected one of a unit's directives,
// which might have been a PSECT.
func badDirective(au *ast.Unit) bool {
	for _, list := range [][]ast.Stmt{au.Prologue, au.Body} {
		for _, s := range list {
			if _, bad := s.(*ast.BadStmt); bad && s.Base().Kind == lex.Directive {
				return true
			}
		}
	}

	return false
}
