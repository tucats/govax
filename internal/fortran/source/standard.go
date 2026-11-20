package source

import (
	"strings"

	"github.com/tucats/govax/internal/fortran/diag"
)

// What /STANDARD=SOURCE_FORM looks for belongs to the lines, not to the statements made
// from them, so this pass notes it as it goes (docs/planning/PHASE-85 - FORTRAN compiler
// usability.md, Decision 15). The notes cost little, and semantic analysis's
// ReportSourceExtensions turns them into messages only if they're asked for.

// Standards is a set of the standards /STANDARD judges by, a bit each: which of them
// have something a source does (docs/planning/PHASE-110 - Fortran 90 compiler
// usability.md, finding 1). A mark or a finding is flagged when the standard judged by
// isn't in its set, so a zero set is flagged under either.
type Standards uint8

// The standards.
const (
	// InF77 is FORTRAN-77 (ANSI X3.9-1978), what the 1988 manual prints in black.
	InF77 Standards = 1 << iota

	// InF90 is Fortran 90 (ISO/IEC 1539:1991).
	InF90
)

// Has reports whether the set holds the standard judged by: Fortran 90's if f90, else
// FORTRAN-77's.
func (s Standards) Has(f90 bool) bool {
	if f90 {
		return s&InF90 != 0
	}

	return s&InF77 != 0
}

// Mark is one place where a source does what a standard doesn't have: What says what,
// for the message; In which standards have it; F90 whether /F90 was in force there,
// which says which standard the place is judged by unless /STANDARD names one; and
// Library says the line came from a text library's module, which isn't the program's
// own to change.
type Mark struct {
	Pos     diag.Position
	What    string
	In      Standards
	F90     bool
	Library bool
}

// standardContinuations is the most continuation lines FORTRAN-77 allows.
const standardContinuations = 19

// standardColumns is how many characters FORTRAN-77 has in a statement field: columns
// 7 to 72.
const standardColumns = 66

// mark notes that line does What at a column, counted from 1, which the standards in
// in have.
func (r *reader) mark(line *Line, column int, what string, in Standards) {
	r.program.Marks = append(r.program.Marks, Mark{
		Pos: linePosition(line, column), What: what, In: in, F90: r.form.F90, Library: line.Library,
	})
}

// markField notes what's out of the standard in a fixed-form line's fields: its form, a
// statement field past column 72, small letters outside constants, and a comment at the
// line's end. It runs the comment scanner over a copy, since the caller's reading of the
// line is what moves the real one on.
//
// Fortran 90 has no tab at all (3.1), but FORTRAN-77's reading flags only the tab format,
// a tab in the label or continuation field, as Phase 85 chose: a tab elsewhere is a blank
// to VAX FORTRAN and to the reader.
func (r *reader) markField(line *Line, fl fields, field string, comments *commentScanner) {
	if tab := strings.IndexByte(line.Text[:min(len(line.Text), continuationColumn)], '\t'); tab >= 0 {
		r.mark(line, tab+1, "a tab in the label or continuation field", 0)
	} else if tab := strings.IndexByte(line.Text, '\t'); tab >= 0 {
		r.mark(line, tab+1, "a tab", InF77)
	}

	if len(field) > standardColumns && !isBlank(field[standardColumns:]) {
		r.mark(line, fl.start+standardColumns+1, "a statement past column 72", 0)
	}

	scanner := *comments
	scanner.start = append([]byte(nil), comments.start...)

	end := scanner.cut(field)

	if scanner.lower >= 0 {
		r.mark(line, fl.start+scanner.lower+1, "small letters", InF90)
	}

	if end < len(field) {
		r.mark(line, fl.start+end+1, "a comment after a statement", InF90)
	}
}

// markFree notes what's out of Fortran 90's free form on a line read in it: a tab, and
// more than 132 characters on a comment line, whose statement lines the reader holds to
// it already. FORTRAN-77 has no free form at all, which LineMarks notes once per file, so
// these are flagged only under Fortran 90 (InF77 says FORTRAN-77's reading passes them).
func (r *reader) markFree(line *Line) {
	if tab := strings.IndexByte(line.Text, '\t'); tab >= 0 {
		r.mark(line, tab+1, "a tab", InF77)
	}

	if line.Kind == Comment && len(line.Text) > freeLineLength {
		r.mark(line, freeLineLength+1, "a line longer than 132 characters", InF77)
	}
}

// LineMarks are the marks only a look at every line finds: a line with nothing to
// compile on it has no statement to note it, and a debugging line may not have one. A
// free-form file is one mark, at its first line, for FORTRAN-77's reading.
func (p *Program) LineMarks() []Mark {
	var (
		marks []Mark
		free  = map[*File]bool{}
	)

	for _, line := range p.Lines {
		text := strings.TrimLeft(line.Text, " \t")
		at := func(column int, what string, in Standards) {
			marks = append(marks, Mark{Pos: linePosition(line, column), What: what, In: in, F90: line.F90, Library: line.Library})
		}

		switch {
		case line.File.Free:
			if !free[line.File] {
				free[line.File] = true

				at(1, "free source form", InF90)
			}

		case line.Text == "":
		case line.Text[0] == 'D' || line.Text[0] == 'd':
			at(1, "a debugging line", 0)
		case line.Kind == Comment && strings.HasPrefix(text, "!"):
			at(len(line.Text)-len(text)+1, "a comment starting with !", InF90)
		}
	}

	return marks
}
