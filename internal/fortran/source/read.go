package source

import (
	"strconv"
	"strings"

	"github.com/tucats/govax/internal/fortran/diag"
)

// Read reads file, and every file it includes, into a Program, under
// the source form f. Problems with the source's layout are reported to
// messages; an INCLUDE file that can't be read, INCLUDE files nested too
// deeply, and text past column 132 under /EXTEND_SOURCE are fatal, and
// reading stops there. opener reads INCLUDE files; with none, an INCLUDE
// statement is a fatal error.
func Read(f Form, file *File, opener Opener, messages *diag.List) *Program {
	r := &reader{form: f, opener: opener, messages: messages, program: &Program{}}
	r.readFile(file, 0)

	return r.program
}

// reader is Read's state.
type reader struct {
	form     Form
	opener   Opener
	messages *diag.List
	program  *Program

	// stopped is set by a fatal error: nothing more is read.
	stopped bool
}

// readFile reads one file's lines, at include depth depth, adding them
// and their statements to the program.
func (r *reader) readFile(file *File, depth int) {
	n := len(file.Lines)
	lines := make([]*Line, n)
	splits := make([]fields, n)

	for i, text := range file.Lines {
		splits[i] = r.form.split(text)
		lines[i] = &Line{File: file, Number: i + 1, Text: text, Depth: depth, Kind: splits[i].kind}
	}

	for i := 0; i < n && !r.stopped; {
		first := len(r.program.Lines)

		switch lines[i].Kind {
		case Comment:
			r.program.Lines = append(r.program.Lines, lines[i])
			i++

		case Directive:
			r.program.Lines = append(r.program.Lines, lines[i])
			r.directive(lines[i], splits[i], first)
			i++

		case Continuation:
			// A continuation line with no statement to continue: at the
			// start of a file (an included file can't begin with one,
			// section 1.4.2) or after a directive.
			r.program.Lines = append(r.program.Lines, lines[i])
			r.messages.Report("NOINITIAL", linePosition(lines[i], 0))
			i++

		case Initial:
			// The statement runs to its last continuation line; comment
			// lines may come between its lines.
			last := i

			for j := i + 1; j < n && (lines[j].Kind == Comment || lines[j].Kind == Continuation); j++ {
				if lines[j].Kind == Continuation {
					last = j
				}
			}

			r.program.Lines = append(r.program.Lines, lines[i:last+1]...)
			r.statement(lines[i:last+1], splits[i:last+1], first, depth)
			i = last + 1
		}
	}
}

// statement puts together the statement on lines, whose first is its
// initial line and the rest its continuation lines and the comment lines
// among them; first is the initial line's index in the program's lines.
func (r *reader) statement(lines []*Line, splits []fields, first, depth int) {
	s := &Statement{First: first, Last: first + len(lines) - 1, Depth: depth}
	r.label(s, lines[0], splits[0])

	var (
		text      strings.Builder
		comments  commentScanner
		continued int
	)

	for k, line := range lines {
		fl := splits[k]

		switch fl.kind {
		case Comment:
			continue

		case Continuation:
			continued++

			if continued == r.continuationLimit()+1 {
				r.messages.Report("TOOMANYCONT", linePosition(line, 0), r.continuationLimit())
			}

			if label := strings.TrimSpace(fl.label); label != "" {
				r.messages.Report("CONTLABEL", linePosition(line, fl.labelAt+1+strings.Index(fl.label, label)))
			}
		}

		if r.tooLong(line, fl) {
			return
		}

		field := line.Text[fl.start:fl.end]
		field = field[:comments.cut(field)]

		s.pieces = append(s.pieces, piece{offset: text.Len(), line: line, column: fl.start + 1})
		text.WriteString(field)
	}

	s.Text = text.String()

	if name, ok := r.include(s); ok {
		r.includeFile(s, name, depth)

		return
	}

	r.program.Statements = append(r.program.Statements, s)
}

// directive adds a CDEC$ line as a directive statement.
func (r *reader) directive(line *Line, fl fields, index int) {
	if r.tooLong(line, fl) {
		return
	}

	var comments commentScanner

	field := line.Text[fl.start:fl.end]
	field = field[:comments.cut(field)]

	r.program.Statements = append(r.program.Statements, &Statement{
		Directive: true,
		Text:      field,
		First:     index,
		Last:      index,
		Depth:     line.Depth,
		pieces:    []piece{{line: line, column: fl.start + 1}},
	})
}

// continuationLimit is the form's limit on continuation lines.
func (r *reader) continuationLimit() int {
	return r.form.continuationLimit()
}

// tooLong reports, as a fatal error, a line with text past column 132
// under /EXTEND_SOURCE.
func (r *reader) tooLong(line *Line, fl fields) bool {
	if !fl.tooLong {
		return false
	}

	r.messages.Report("LINETOOLONG", linePosition(line, fl.end+1), extendedEnd)
	r.stopped = true

	return true
}

// label reads the label field of a statement's initial line into s:
// one to five digits, blanks ignored among them (section 1.3.3).
func (r *reader) label(s *Statement, line *Line, fl fields) {
	digits := strings.Map(func(c rune) rune {
		if c == ' ' {
			return -1
		}

		return c
	}, fl.label)

	if digits == "" {
		return
	}

	column := fl.labelAt + 1 + strings.IndexFunc(fl.label, func(c rune) bool { return c != ' ' })
	s.labelLine, s.labelColumn = line, column

	if len(digits) > labelColumns || strings.Trim(digits, "0123456789") != "" {
		r.messages.Report("BADLABEL", linePosition(line, column), strings.TrimSpace(fl.label))

		return
	}

	n, _ := strconv.Atoi(digits)
	if n == 0 {
		r.messages.Report("ZEROLABEL", linePosition(line, column))

		return
	}

	s.Label = n
}
