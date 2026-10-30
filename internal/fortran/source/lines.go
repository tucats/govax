package source

import "strings"

// fields is what one line's columns hold: the line's kind, and for a line
// with a statement on it, where its label field and statement field are.
// Offsets are indexes into the line's text, counted from 0; columns, as
// messages show them, count from 1.
type fields struct {
	kind LineKind

	// label is the label field's text, and labelAt the index it starts
	// at (1 on a debugging line, whose column 1 holds the D).
	label   string
	labelAt int

	// start and end bound the statement field: text[start:end].
	start, end int

	// tooLong is set when /EXTEND_SOURCE is in effect and the line has
	// text past column 132, which the manual makes a fatal error.
	tooLong bool
}

// The columns of a fixed-format line, counted from 1.
const (
	labelColumns       = 5 // the label field is columns 1 to 5
	continuationColumn = 6 // the continuation indicator
	statementColumn    = 7 // the statement field's first column
)

// split works out what text's columns hold under the form f.
func (f Form) split(text string) fields {
	if isBlank(text) {
		return fields{kind: Comment}
	}

	switch text[0] {
	case 'C', 'c':
		switch {
		case hasTag(text, "CDEC$"):
			// A compiler directive: never continued, never labeled, its
			// text from column 7 (chapter 10's rules).
			return f.statementField(fields{kind: Directive}, text, statementColumn-1)
		default:
			// A comment, or a CPAR$ parallel directive, which govax treats
			// as one (there is no /PARALLEL).
			return fields{kind: Comment}
		}

	case '*', '!':
		return fields{kind: Comment}

	case 'D', 'd':
		if !f.DLines {
			return fields{kind: Comment}
		}

		// Compiled: the D is read as a blank, and the rest of the label
		// field may hold a label.
		return f.lineFields(text, 1)
	}

	return f.lineFields(text, 0)
}

// lineFields splits a line that holds a statement (or part of one) into
// its fields; the label field starts at index labelAt.
func (f Form) lineFields(text string, labelAt int) fields {
	// A line whose first nonblank character is a ! is all comment (a !
	// in column 6 of a fixed-format line excepted: that's a continuation
	// indicator). The manual says only that ! starts a comment in column
	// 1 or in the statement field; reading the rest of a line this way,
	// rather than as an empty statement, keeps such a line from ending a
	// statement whose continuation lines follow. govax's choice.
	if i := strings.IndexFunc(text[labelAt:], func(r rune) bool { return !isBlankRune(r) }); i >= 0 {
		if i += labelAt; text[i] == '!' && (i != continuationColumn-1 || hasTabBefore(text, i)) {
			return fields{kind: Comment}
		}
	}

	// A tab-format line: a tab within the first six columns ends the
	// label field (section 1.3.2).
	if tab := strings.IndexByte(text[:min(len(text), continuationColumn)], '\t'); tab >= labelAt {
		fl := fields{kind: Initial, label: text[labelAt:tab], labelAt: labelAt}
		start := tab + 1

		// After the tab, a digit is the continuation indicator, standing
		// in column 6: 1 to 9 continue the statement before, and 0 (as
		// in column 6 of a fixed-format line) marks an initial line.
		if start < len(text) && text[start] >= '0' && text[start] <= '9' {
			if text[start] != '0' {
				fl.kind = Continuation
			}

			start++
		}

		return f.statementField(fl, text, start)
	}

	// A fixed-format line: each field in its columns.
	fl := fields{kind: Initial, labelAt: labelAt}
	fl.label = text[labelAt:min(len(text), labelColumns)]

	if len(text) >= continuationColumn {
		if c := text[continuationColumn-1]; c != ' ' && c != '0' {
			fl.kind = Continuation
		}
	}

	return f.statementField(fl, text, statementColumn-1)
}

// statementField fills in fl's statement field, which starts at index
// start of text and is in column 7 there. It ends at column 72 (or 132):
// a tab before it stood for the columns before column 7, so the field
// holds as many characters as the columns from 7 to the end.
func (f Form) statementField(fl fields, text string, start int) fields {
	fl.start = min(start, len(text))
	fl.end = min(len(text), start+f.statementEnd()-statementColumn+1)

	if f.ExtendSource && !isBlank(text[fl.end:]) {
		fl.tooLong = true
	}

	return fl
}

// hasTag reports whether text starts with tag, a compiler directive's
// five-character tag (any case), followed by a blank, a tab, or the end
// of the line, in column 6.
func hasTag(text, tag string) bool {
	if len(text) < len(tag) || !strings.EqualFold(text[:len(tag)], tag) {
		return false
	}

	return len(text) == len(tag) || text[len(tag)] == ' ' || text[len(tag)] == '\t'
}

// hasTabBefore reports whether text has a tab before index i.
func hasTabBefore(text string, i int) bool {
	return strings.IndexByte(text[:i], '\t') >= 0
}

// isBlank reports whether text holds only blanks: spaces, tabs, form
// feeds (a listing's page break), and a carriage return left by a host
// file's CR-LF line ends.
func isBlank(text string) bool {
	return strings.IndexFunc(text, func(r rune) bool { return !isBlankRune(r) }) < 0
}

// isBlankRune reports whether r counts as a blank in a source line.
func isBlankRune(r rune) bool {
	switch r {
	case ' ', '\t', '\f', '\r', '\v':
		return true
	}

	return false
}
