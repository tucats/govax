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
	return ReadAll(f, []*File{file}, opener, messages)
}

// ReadAll is Read for files joined into one compilation (FORTRAN A+B): one
// Program, of their lines in turn.
func ReadAll(f Form, files []*File, opener Opener, messages *diag.List) *Program {
	r := &reader{command: f, form: f, opener: opener, messages: messages, program: &Program{}}

	// Each file is read in turn into the same Program, so a statement
	// list made of several files looks to the later passes like one file.
	// A fatal error in one stops the rest from being read.
	for _, file := range files {
		if !r.stopped {
			r.readFile(file, 0, true, false)
		}
	}

	return r.program
}

// reader is Read's state. Its methods (functions with a receiver, r *reader)
// share it rather than passing a long list of arguments to one another; the
// receiver is a pointer so that each method's changes are seen by the next.
type reader struct {
	// command is the form the command gave, and form the one in effect: the
	// command's, as the OPTIONS statements before the current unit change it.
	command Form
	form    Form

	// inUnit is set from a unit's first statement to its END (options.go).
	inUnit bool

	opener   Opener
	messages *diag.List
	program  *Program

	// stopped is set by a fatal error: nothing more is read.
	stopped bool

	// serial is the last Line.Serial given.
	serial int
}

// readFile reads one file's lines, at include depth depth, adding them
// and their statements to the program. listed and library are each line's
// Listed and Library.
func (r *reader) readFile(file *File, depth int, listed, library bool) {
	n := len(file.Lines)
	lines := make([]*Line, n)

	// A line's kind doesn't depend on where the statement field ends, so it can
	// be known ahead of the OPTIONS statements that change that. The fields
	// themselves are worked out as each line is reached.
	for i, text := range file.Lines {
		lines[i] = &Line{File: file, Number: i + 1, Text: text, Depth: depth, Kind: r.form.split(text).kind, Listed: listed, Library: library}
	}

	// Walk the lines, taking one line or one whole statement per turn. The
	// loop's post statement is empty because each case moves i on itself,
	// by one line or by as many as the statement took.
	for i := 0; i < n && !r.stopped; {
		// first is where this line will be in the program's list of every
		// line, which messages and the listing use to find it again.
		first := len(r.program.Lines)

		switch lines[i].Kind {
		case Comment:
			r.add(lines[i])
			i++

		case Directive:
			r.add(lines[i])
			r.directive(lines[i], first)
			i++

		case Continuation:
			// A continuation line with no statement to continue: at the
			// start of a file (an included file can't begin with one,
			// section 1.4.2) or after a directive.
			r.add(lines[i])
			r.messages.Report("NOINITIAL", linePosition(lines[i], 0))
			i++

		case Initial:
			// An initial line starts a statement (column 6 blank or 0).
			// The statement runs to its last continuation line; comment
			// lines may come between its lines. Look ahead for the last
			// continuation, so that comments after it aren't taken in:
			// they belong to whatever comes next.
			last := i

			for j := i + 1; j < n && (lines[j].Kind == Comment || lines[j].Kind == Continuation); j++ {
				if lines[j].Kind == Continuation {
					last = j
				}
			}

			r.add(lines[i : last+1]...)
			r.statement(lines[i:last+1], first, depth)
			i = last + 1
		}
	}
}

// add makes lines the program's next, and numbers them. A file's lines are numbered
// here, not when it's read, so that an included file's come between the lines around
// its INCLUDE: Serial is the source's order, the order a compilation sees its lines.
func (r *reader) add(lines ...*Line) {
	for _, line := range lines {
		r.serial++
		line.Serial = r.serial
	}

	r.program.Lines = append(r.program.Lines, lines...)
}

// statement puts together the statement on lines, whose first is its
// initial line and the rest its continuation lines and the comment lines
// among them; first is the initial line's index in the program's lines.
func (r *reader) statement(lines []*Line, first, depth int) {
	s := &Statement{First: first, Last: first + len(lines) - 1, Depth: depth, Library: lines[0].Library}

	// Only the initial line may have a label; a continuation's label field
	// should be blank (CONTLABEL below).
	r.label(s, lines[0], r.form.split(lines[0].Text))

	// The statement's text is every line's statement field joined end to
	// end, with no blank or newline between: FORTRAN's continuation simply
	// carries on the same text. Each piece records where in the joined
	// text a line's field begins, so that a column in the joined text can
	// later be turned back into a line and column for a message
	// (Statement.Position).
	var (
		text      strings.Builder
		comments  commentScanner
		continued int
	)

	for _, line := range lines {
		fl := r.form.split(line.Text)

		switch fl.kind {
		case Comment:
			continue

		case Continuation:
			continued++

			if continued == standardContinuations+1 {
				r.mark(line, 1, "more than 19 continuation lines")
			}

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

		// The statement field is the line from column 7 (or the end of a
		// tab-format label) to column 72 (or 132). markField notes, for
		// /STANDARD, anything in it FORTRAN-77 lacks; then an ! comment is
		// cut off its end. comments is shared from line to line because a
		// quoted string may run on from one line into its continuation, and
		// an ! inside it is text, not a comment.
		field := line.Text[fl.start:fl.end]
		r.markField(line, fl, field, &comments)
		field = field[:comments.cut(field)]

		s.pieces = append(s.pieces, piece{offset: text.Len(), line: line, column: fl.start + 1})
		text.WriteString(field)
	}

	s.Text = text.String()

	// An INCLUDE statement isn't a statement the later passes see: it is
	// replaced here by the statements of the file it names, read at one
	// include depth deeper.
	if spec, ok := r.include(s); ok {
		r.program.Includes = append(r.program.Includes, Mark{s.Position(0), "the INCLUDE statement", lines[0].Library})
		r.includeFile(s, spec, depth, lines[0])

		return
	}

	r.program.Statements = append(r.program.Statements, s)

	// Keep track of where program units start and end, since an OPTIONS
	// statement before a unit changes the source form for that unit alone.
	r.followUnit(s)
}

// directive adds a CDEC$ line as a directive statement.
func (r *reader) directive(line *Line, index int) {
	fl := r.form.split(line.Text)

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
		Library:   line.Library,
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
	// strings.Map calls the function for each character and builds a new
	// string from what it returns; returning -1 drops the character. So
	// this is the label field with its blanks taken out: " 1 0 " is "10".
	digits := strings.Map(func(c rune) rune {
		if c == ' ' {
			return -1
		}

		return c
	}, fl.label)

	if digits == "" {
		return
	}

	// The column of the label's first digit, for messages. Columns count
	// from 1, as a programmer counts them; Go's string indexes count from 0.
	column := fl.labelAt + 1 + strings.IndexFunc(fl.label, func(c rune) bool { return c != ' ' })
	s.labelLine, s.labelColumn = line, column

	// Trimming every digit from both ends leaves something only if there
	// was a character that isn't a digit.
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
