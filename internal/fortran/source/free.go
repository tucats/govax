package source

import (
	"strconv"
	"strings"
)

// Free source form is Fortran 90's (the standard's section 3.3.1). Where fixed
// form gives each column a meaning, free form has none: a line is up to 132
// characters, and a statement may be anywhere on it.
//
//	program squares            ! a comment runs from ! to the line's end
//	  integer :: i
//	  do i = 1, 3; print *, i*i; end do      ! ; separates statements
//	  print *, "the sum of the squares is", &
//	           1 + 4 + 9                       ! & continues a statement
//	10 continue                ! a label is the statement's first token
//	end program squares
//
// The rules, each where this file applies it:
//
//   - A comment starts with a ! anywhere outside a character context (a
//     character constant, or a FORMAT's Hollerith text). A line holding
//     nothing else, or nothing at all, is a comment line.
//   - A ; outside a character context ends a statement, so a line may hold
//     several. Several ; in a row, blanks between them, are one; a ; at a
//     line's end (or before its comment) is ignored; one with no statement
//     before it on its line is an error (EMPTYSEMI).
//   - An & as a line's last character (before any comment) continues the
//     statement on the next line that isn't a comment line. If that line's
//     first character is an &, the statement goes on just after it, so a
//     name or a number may be split across lines (INTE& then &GER is INTEGER);
//     otherwise it goes on with the line's first character, its blanks and
//     all. In a character context the second & is required (MISSINGAMP), and
//     the first must end the line: a ! after it is part of the constant.
//   - A line holding only an & (and perhaps a comment) is an error (LONEAMP).
//   - A statement may have 39 continuation lines, or /CONTINUATIONS=n.
//   - A label is one to five digits starting a statement, a blank after it.
//   - DEC's directives are written !DEC$ (the HP Fortran manual's form for
//     free form): a line whose first characters, after any blanks, are !DEC$
//     and a blank. A directive is never continued.
//
// One more rule changes what this pass hands on: in free form, blanks are
// significant. A blank must separate a name, a keyword, or a constant from the
// next, and none may stand inside one (the standard's 3.3.1, except in a
// character context). The lexer judges that (Phase 99, subtask 4), from the
// statement's Text, which keeps every blank its lines had between the
// statement's pieces: a continuation line without a leading & brings its own
// leading blanks along, and one with it brings none, the two halves of a split
// token then meeting. Statement.Free says to read the blanks this way.
//
// A line ending with & whose next line starts in column 1 without one, as in
// "X = AB&" followed by "C", is a token split without the second & the
// standard requires. The text read is "X = ABC", as the standard's words
// ("it continues with the first character position of the next noncomment
// line") give it; nothing here can know that AB and C were meant as two
// tokens, and the lexer sees one name.

// DefaultFreeContinuations is how many continuation lines a free-form
// statement may have when /CONTINUATIONS isn't given: the standard's limit,
// 39 (3.3.1.4).
const DefaultFreeContinuations = 39

// freeLineLength is the most characters a free-form line may hold (3.3.1).
const freeLineLength = 132

// freeContinuationLimit is the number of continuation lines a free-form
// statement may have under the form.
func (f Form) freeContinuationLimit() int {
	if f.Continuations == 0 {
		return DefaultFreeContinuations
	}

	return f.continuationLimit()
}

// freeStatement is a free-form statement being put together, which may be
// continued from line to line.
type freeStatement struct {
	s    *Statement
	text strings.Builder

	// scanner follows the statement's character constants and Hollerith
	// constants, across its lines, so that a !, ;, or & inside one is text.
	scanner commentScanner

	// continued counts the statement's continuation lines.
	continued int

	// inContext says the last line ended with an & inside a character
	// context, so the next one must start with an &.
	inContext bool

	// ampersand is the line and column of the & that continued the
	// statement last, for a message if no line follows it.
	ampersand   *Line
	ampersandAt int
}

// readFree reads a free-form file's lines, at include depth depth, adding them
// and their statements to the program. listed and library are each line's
// Listed and Library.
func (r *reader) readFree(file *File, depth int, listed, library bool) {
	// open is the statement an & continued onto the next line, or nil.
	var open *freeStatement

	for i, text := range file.Lines {
		if r.stopped {
			return
		}

		line := &Line{File: file, Number: i + 1, Text: text, Depth: depth, Listed: listed, Library: library}
		open = r.freeLine(line, open)
	}

	// A file can't end in the middle of a statement: an INCLUDE'd file's last
	// statement doesn't go on in the file that included it.
	if open != nil && !r.stopped {
		r.messages.Report("CONTNOLINE", linePosition(open.ampersand, open.ampersandAt))
		r.finishFree(open)
	}
}

// freeLine reads one line. open is the statement continued onto it, or nil;
// freeLine returns the statement continued onto the next line, or nil.
func (r *reader) freeLine(line *Line, open *freeStatement) *freeStatement {
	text := line.Text
	first := strings.IndexFunc(text, func(c rune) bool { return !isBlankRune(c) })

	// Comment lines, which may come between a statement's lines, and !DEC$
	// directives, which a continued statement can't have among its lines.
	switch {
	case first < 0 || text[first] == '!' && !hasTag(text[first:], "!DEC$"):
		line.Kind = Comment
		r.add(line)

		return open

	case text[first] == '!':
		if r.freeTooLong(line) {
			return nil
		}

		if open != nil {
			// A directive in the middle of a continued statement would have to
			// come before it or after it; it is neither, and is reported.
			line.Kind = Comment
			r.add(line)
			r.messages.Report("DIRINSTMT", linePosition(line, first+1))

			return open
		}

		line.Kind = Directive
		r.add(line)
		r.freeDirective(line, first)

		return nil
	}

	if r.freeTooLong(line) {
		return nil
	}

	line.Kind = Initial
	if open != nil {
		line.Kind = Continuation
	}

	r.add(line)

	// A line holding only an &: it neither continues a statement nor is
	// continued (3.3.1.3). Whatever was open stays open.
	// In a character context a ! after it is the constant's text, not a
	// comment.
	if text[first] == '&' {
		rest := text[first+1:]
		inContext := open != nil && open.inContext

		if isBlank(rest) || !inContext && isBlankOrComment(rest) {
			r.messages.Report("LONEAMP", linePosition(line, first+1))

			return open
		}
	}

	start := 0

	switch {
	case open != nil:
		open.continued++

		limit := r.form.freeContinuationLimit()
		if open.continued == limit+1 {
			r.messages.Report("TOOMANYCONT", linePosition(line, 0), limit)
		}

		switch {
		case text[first] == '&':
			start = first + 1
		case open.inContext:
			r.messages.Report("MISSINGAMP", linePosition(line, first+1))
		}

	case text[first] == '&':
		// An & starting a line that continues nothing.
		r.messages.Report("NOINITIAL", linePosition(line, first+1))
		start = first + 1
	}

	return r.freeStatements(line, start, open)
}

// isBlankOrComment reports whether text is blank, or blank up to a !.
func isBlankOrComment(text string) bool {
	rest := strings.TrimLeftFunc(text, isBlankRune)

	return rest == "" || rest[0] == '!'
}

// freeStatements reads the statements, or parts of statements, on line from
// index start. open is the statement continued onto the line, or nil; the
// statement continued onto the next line is returned, or nil.
func (r *reader) freeStatements(line *Line, start int, open *freeStatement) *freeStatement {
	text := line.Text
	pos := start

	// separated says a ; has ended a statement on this line, so more ;
	// after it are part of the same separator.
	separated := false

	for !r.stopped {
		if open == nil {
			// Between statements: blanks, then a statement, a ;, a comment, or
			// the line's end.
			pos += len(text[pos:]) - len(strings.TrimLeftFunc(text[pos:], isBlankRune))

			switch {
			case pos >= len(text) || text[pos] == '!':
				return nil

			case text[pos] == ';':
				if !separated {
					r.messages.Report("EMPTYSEMI", linePosition(line, pos+1))
				}

				separated = true
				pos++

				continue
			}

			open = r.startFree(line, &pos)
		}

		end, continued, separator := freeSegment(&open.scanner, text, pos)

		if end > pos || len(open.s.pieces) == 0 {
			open.s.pieces = append(open.s.pieces, piece{offset: open.text.Len(), line: line, column: pos + 1})
			open.text.WriteString(text[pos:end])
		}

		open.s.Last = len(r.program.Lines) - 1

		if continued {
			open.inContext = !open.scanner.outside()
			open.ampersand, open.ampersandAt = line, end+1

			return open
		}

		r.finishFree(open)
		open = nil

		if !separator {
			return nil
		}

		separated = true
		pos = end + 1
	}

	return nil
}

// freeSegment finds the end of the part of a statement that starts at index
// pos of text: a ! starting a comment, a ; separating statements, an & that
// continues the statement, or the line's end. It returns that index, and
// whether it is an & or a ;. scanner follows the constants on the way.
func freeSegment(scanner *commentScanner, text string, pos int) (end int, continued, separator bool) {
	for i := pos; i < len(text); i++ {
		c := text[i]
		outside := scanner.outside()

		switch {
		case outside && c == '!':
			return i, false, false

		case outside && c == ';':
			return i, false, true

		// An & continues the statement if nothing but blanks follows it, or
		// outside a character context, blanks and a comment. Anywhere else it
		// is an &, in a character constant or an error for the lexer.
		case c == '&' && (isBlank(text[i+1:]) || outside && isBlankOrComment(text[i+1:])):
			return i, true, false
		}

		scanner.feed(c, i)
	}

	return len(text), false, false
}

// startFree starts a statement at index *pos of line, reading its label if it
// has one, and moves *pos past the label and the blanks after it.
func (r *reader) startFree(line *Line, pos *int) *freeStatement {
	index := len(r.program.Lines) - 1
	open := &freeStatement{
		s: &Statement{
			First: index, Last: index, Depth: line.Depth, Library: line.Library,
			F90: r.form.F90, Free: true,
		},
		scanner: commentScanner{doubleQuotes: r.form.F90},
	}

	text := line.Text
	digits := *pos

	for digits < len(text) && text[digits] >= '0' && text[digits] <= '9' {
		digits++
	}

	// No statement starts with a digit, so digits first are a label (3.3.1.4),
	// and a blank must follow it (3.3.1): "10continue" is reported, and read
	// as if the blank were there.
	if digits > *pos {
		label := text[*pos:digits]
		open.s.labelLine, open.s.labelColumn = line, *pos+1

		switch n, _ := strconv.Atoi(label); {
		case len(label) > labelColumns:
			r.messages.Report("BADLABEL", linePosition(line, *pos+1), label)
		case n == 0:
			r.messages.Report("ZEROLABEL", linePosition(line, *pos+1))
		default:
			open.s.Label = n
		}

		if digits < len(text) && !isBlankRune(rune(text[digits])) && text[digits] != ';' && text[digits] != '!' {
			r.messages.Report("LABELBLANK", linePosition(line, digits+1))
		}

		*pos = digits
		*pos += len(text[*pos:]) - len(strings.TrimLeftFunc(text[*pos:], isBlankRune))
	}

	return open
}

// finishFree hands on a finished free-form statement.
func (r *reader) finishFree(open *freeStatement) {
	open.s.Text = open.text.String()
	r.complete(open.s)
}

// freeDirective adds a !DEC$ line as a directive statement; the tag starts at
// index first.
func (r *reader) freeDirective(line *Line, first int) {
	start := first + len("!DEC$")
	comments := commentScanner{doubleQuotes: r.form.F90}
	field := line.Text[start:]
	field = field[:comments.cut(field)]

	index := len(r.program.Lines) - 1

	r.program.Statements = append(r.program.Statements, &Statement{
		Directive: true,
		Text:      field,
		First:     index,
		Last:      index,
		Depth:     line.Depth,
		Library:   line.Library,
		F90:       r.form.F90,
		Free:      true,
		pieces:    []piece{{line: line, column: start + 1}},
	})
}

// freeTooLong reports, as a fatal error, a line with more than 132 characters
// other than blanks at its end. The line is added to the program either way.
func (r *reader) freeTooLong(line *Line) bool {
	if len(line.Text) <= freeLineLength || isBlank(line.Text[freeLineLength:]) {
		return false
	}

	line.Kind = Initial
	r.add(line)
	r.messages.Report("LINETOOLONG", linePosition(line, freeLineLength+1), freeLineLength)
	r.stopped = true

	return true
}
