package source

import (
	"github.com/tucats/govax/internal/fortran/diag"
)

// The continuation limits: VAX FORTRAN's /CONTINUATIONS=n takes 0 to 99,
// and allows 19 when it isn't given (the numbers as the VAX FORTRAN User
// Manual is remembered to give them; that manual isn't to hand, so they
// are unconfirmed, docs/PHASE-74 - FORTRAN plan.md, subtask 3).
const (
	DefaultContinuations = 19
	MaxContinuations     = 99

	// NoContinuations is Form.Continuations for /CONTINUATIONS=0, since
	// the zero value of the field means the default.
	NoContinuations = -1
)

// MaxIncludeDepth is how deeply INCLUDE files may nest: a file the source
// includes is at depth 1, a file that one includes at depth 2, and so on.
// The manual sets no limit; this one is govax's, enough for any real
// program and small enough to stop a file that includes itself.
const MaxIncludeDepth = 10

// The statement field's last column: 72 normally, 132 with
// /EXTEND_SOURCE.
const (
	standardEnd = 72
	extendedEnd = 132
)

// Form is how the source is laid out: the FORTRAN command's qualifiers
// that bear on reading lines.
type Form struct {
	// ExtendSource is /EXTEND_SOURCE: the statement field runs to column
	// 132, and there is no sequence number field.
	ExtendSource bool

	// DLines is /D_LINES: debugging lines (a D in column 1) are compiled
	// rather than treated as comments.
	DLines bool

	// Continuations is /CONTINUATIONS=n, the most continuation lines one
	// statement may have. 0 means the default, DefaultContinuations, and
	// NoContinuations means none at all.
	Continuations int
}

// continuationLimit is the number of continuation lines a statement may
// have under the form.
func (f Form) continuationLimit() int {
	switch {
	case f.Continuations == 0:
		return DefaultContinuations
	case f.Continuations < 0:
		return 0
	}

	return f.Continuations
}

// statementEnd is the statement field's last column under the form.
func (f Form) statementEnd() int {
	if f.ExtendSource {
		return extendedEnd
	}

	return standardEnd
}

// File is one source file: the file compiled, or one an INCLUDE statement
// names.
type File struct {
	// Spec is the file's full specification, as messages show it.
	Spec string

	// Lines are its records, without line terminators.
	Lines []string

	// Handle is the Opener's own note of where the file is, so that it
	// can find a file this one includes relative to it. This package
	// never looks at it.
	Handle any
}

// Opener finds and reads the file an INCLUDE statement names. The FORTRAN
// command supplies one that reads host and volume files; tests supply
// files from memory.
type Opener interface {
	// OpenInclude reads the file named name (the INCLUDE statement's
	// string, without its /LIST or /NOLIST), which the file from
	// includes.
	OpenInclude(name string, from *File) (*File, error)
}

// LineKind says what a line is.
type LineKind int

// The kinds of line.
const (
	// Comment is a line the compiler ignores: a comment line, a blank
	// line, a debugging line without /D_LINES, a CPAR$ directive.
	Comment LineKind = iota

	// Initial is a statement's first line.
	Initial

	// Continuation continues the statement on the lines before it.
	Continuation

	// Directive is a CDEC$ compiler directive.
	Directive
)

// String names the kind, for tests and dumps.
func (k LineKind) String() string {
	return [...]string{"comment", "initial", "continuation", "directive"}[k]
}

// Line is one line of a source file, as read.
type Line struct {
	// File is the file the line is in, and Number its line number there,
	// counted from 1.
	File   *File
	Number int

	// Text is the line as it is in the file.
	Text string

	// Depth is how deeply the file is included: 0 for the file being
	// compiled, 1 for a file it includes, and so on.
	Depth int

	// Kind is what the line is.
	Kind LineKind
}

// Statement is one statement: its label, its text, and where it came
// from.
type Statement struct {
	// Label is the statement label, or 0 if it has none.
	Label int

	// Directive is true for a CDEC$ compiler directive; its Text is what
	// follows the CDEC$ tag.
	Directive bool

	// Text is the statement: the statement fields of its initial line
	// and its continuation lines, joined, with end-of-line comments
	// removed. Its blanks are as they were in the source (the lexer
	// removes those outside character constants).
	Text string

	// First and Last are the indexes, in Program.Lines, of the statement's
	// initial line and its last continuation line. The lines between them
	// are its continuation lines and any comment lines among them.
	First, Last int

	// Depth is the include depth of the file the statement is in.
	Depth int

	// pieces map Text back to the source: one for each line that gave
	// the statement text, in order.
	pieces []piece

	// labelLine and labelColumn are where the label is.
	labelLine   *Line
	labelColumn int
}

// piece is the part of a statement's text that came from one line.
type piece struct {
	offset int   // where in Statement.Text the line's text starts
	line   *Line // the line
	column int   // the column of that first character in the line, from 1
}

// Position is where the character at offset in the statement's text
// came from: its file, line, and column. An offset at or past the end of
// the text is the place just after the statement's last character, for
// messages about something missing at the end.
func (s *Statement) Position(offset int) diag.Position {
	if len(s.pieces) == 0 {
		return diag.Position{}
	}

	p := s.pieces[0]

	for _, next := range s.pieces[1:] {
		if next.offset > offset {
			break
		}

		p = next
	}

	return linePosition(p.line, p.column+offset-p.offset)
}

// Initial is the part of the statement's text that came from its initial
// line, before any continuation line's. The manual's section 5.5 needs
// it: an initial line holding only END is an END statement, whatever
// continuation lines follow it.
func (s *Statement) Initial() string {
	if len(s.pieces) < 2 {
		return s.Text
	}

	return s.Text[:s.pieces[1].offset]
}

// LabelPosition is where the statement's label is: the line and column of
// its first digit.
func (s *Statement) LabelPosition() diag.Position {
	if s.labelLine == nil {
		return s.Position(0)
	}

	return linePosition(s.labelLine, s.labelColumn)
}

// linePosition is a message's position for column of line.
func linePosition(line *Line, column int) diag.Position {
	return diag.Position{
		File:   line.File.Spec,
		Line:   line.Number,
		Column: column,
		Text:   line.Text,
	}
}

// Program is what Read found: every line and every statement.
type Program struct {
	// Lines are every line read, in the order read: the source's lines,
	// with each included file's lines after its INCLUDE statement's.
	Lines []*Line

	// Statements are the statements, in order, INCLUDE statements
	// replaced by the statements of the files they include.
	Statements []*Statement
}
