package source

import (
	"strings"

	"github.com/tucats/govax/internal/fortran/diag"
)

// What /STANDARD=SOURCE_FORM looks for belongs to the lines, not to the statements made
// from them, so this pass notes it as it goes (docs/planning/PHASE-85 - FORTRAN compiler
// usability.md, Decision 15). The notes cost little, and semantic analysis's
// ReportSourceExtensions turns them into messages only if they're asked for.

// Mark is one place where a source does what FORTRAN-77 doesn't have: What says what,
// for the message, and Library says the line came from a text library's module, which
// isn't the program's own to change.
type Mark struct {
	Pos     diag.Position
	What    string
	Library bool
}

// standardContinuations is the most continuation lines FORTRAN-77 allows.
const standardContinuations = 19

// standardColumns is how many characters FORTRAN-77 has in a statement field: columns
// 7 to 72.
const standardColumns = 66

// mark notes that line does What at a column, counted from 1.
func (r *reader) mark(line *Line, column int, what string) {
	r.program.Marks = append(r.program.Marks, Mark{linePosition(line, column), what, line.Library})
}

// markField notes what's out of the standard in a line's fields: its form, a statement
// field past column 72, small letters outside constants, and a comment at the line's
// end. It runs the comment scanner over a copy, since the caller's reading of the line
// is what moves the real one on.
func (r *reader) markField(line *Line, fl fields, field string, comments *commentScanner) {
	if tab := strings.IndexByte(line.Text[:min(len(line.Text), continuationColumn)], '\t'); tab >= 0 {
		r.mark(line, tab+1, "a tab in the label or continuation field")
	}

	if len(field) > standardColumns && !isBlank(field[standardColumns:]) {
		r.mark(line, fl.start+standardColumns+1, "a statement past column 72")
	}

	scanner := *comments
	scanner.start = append([]byte(nil), comments.start...)

	end := scanner.cut(field)

	if scanner.lower >= 0 {
		r.mark(line, fl.start+scanner.lower+1, "small letters")
	}

	if end < len(field) {
		r.mark(line, fl.start+end+1, "a comment after a statement")
	}
}

// LineMarks are the marks only a look at every line finds: a line with nothing to
// compile on it has no statement to note it, and a debugging line may not have one.
func (p *Program) LineMarks() []Mark {
	var marks []Mark

	for _, line := range p.Lines {
		text := strings.TrimLeft(line.Text, " \t")

		switch {
		case line.Text == "":
		case line.Text[0] == 'D' || line.Text[0] == 'd':
			marks = append(marks, Mark{linePosition(line, 1), "a debugging line", line.Library})
		case line.Kind == Comment && strings.HasPrefix(text, "!"):
			marks = append(marks, Mark{linePosition(line, len(line.Text)-len(text)+1), "a comment starting with !", line.Library})
		}
	}

	return marks
}
