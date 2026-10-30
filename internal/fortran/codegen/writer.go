package codegen

import (
	"fmt"
	"strings"
)

// MaxLine is the longest source line MACRO-32 accepts: "a source line can
// be up to 132 characters long" (the VAX MACRO and Instruction Set
// Reference Manual). A tab counts as one character, however far it moves
// the text.
const MaxLine = 132

// The columns of a MACRO statement, as MACRO programmers lay them out
// and as MACRO's listings show them: the opcode at the first tab stop,
// the operands at the second, a comment at column 40. Columns are counted
// from 0, and a tab stop is every 8 columns.
const (
	opcodeColumn   = 8
	operandColumn  = 16
	commentColumn  = 40
	tabWidth       = 8
	sourcePrefixed = 8 // the width of a source line comment's prefix
)

// Writer builds a MACRO-32 source file a line at a time. Its methods
// never fail: a line too long for MACRO is laid out again to fit (a
// comment moved to its own line, a long comment split), so the file
// always assembles as far as its layout goes.
type Writer struct {
	lines []string
}

// Lines returns the file's lines, without line terminators: the records
// of the .MAR file.
func (w *Writer) Lines() []string {
	return append([]string(nil), w.lines...)
}

// Blank writes an empty line, to separate parts of the file.
func (w *Writer) Blank() {
	w.lines = append(w.lines, "")
}

// Comment writes a line that is only a comment: "; text", or ";" alone
// for empty text. Text longer than one line allows is continued on more
// comment lines, split at a blank where it can be.
func (w *Writer) Comment(text string) {
	text = strings.TrimRight(printable(text), " \t")
	if text == "" {
		w.lines = append(w.lines, ";")

		return
	}

	for _, piece := range splitText(text, MaxLine-2) {
		w.lines = append(w.lines, "; "+piece)
	}
}

// Statement writes one MACRO statement: an optional label (written with
// its colon), the opcode or directive, its operands, and an optional
// comment, each in its column. A label too long to leave the opcode its
// column goes on a line of its own, as MACRO programmers write them. If
// the statement and its comment won't fit in one line, the comment goes
// on a line of its own before the statement.
func (w *Writer) Statement(label, opcode, operands, comment string) {
	var b strings.Builder

	if label != "" {
		if len(label)+1 >= opcodeColumn {
			w.Label(label)
		} else {
			b.WriteString(label)
			b.WriteByte(':')
		}
	}

	padTo(&b, opcodeColumn)
	b.WriteString(opcode)

	if operands != "" {
		padTo(&b, operandColumn)
		b.WriteString(operands)
	}

	line := b.String()

	if comment != "" {
		var c strings.Builder
		c.WriteString(line)
		padTo(&c, commentColumn)
		c.WriteString("; ")
		c.WriteString(printable(comment))

		if c.Len() <= MaxLine {
			line = c.String()
		} else {
			w.Comment(comment)
		}
	}

	w.lines = append(w.lines, line)
}

// Label writes a label alone on its line.
func (w *Writer) Label(label string) {
	w.lines = append(w.lines, label+":")
}

// SourceLine writes one line of the FORTRAN source as a comment, with its
// line number:
//
//	;   12        DO 10 I = 1, N
//
// The prefix (";", the number in five columns, two blanks) is exactly 8
// characters, one tab stop, so a source line written with tabs (FORTRAN's
// tab-format lines) keeps its layout in the comment. A line too long for
// MACRO is continued on further comment lines whose prefix shows a "-"
// where the number was.
func (w *Writer) SourceLine(number int, text string) {
	text = strings.TrimRight(printable(text), " \t")

	prefix := fmt.Sprintf(";%5d  ", number)
	if len(prefix) != sourcePrefixed {
		// A line number past 99999 doesn't fit the five columns. One
		// blank keeps the prefix at 8 characters up to 999999; past that
		// the tabs' layout shifts, but the line is still all there.
		prefix = fmt.Sprintf(";%d ", number)
	}

	room := MaxLine - len(prefix)

	for len(text) > room {
		w.lines = append(w.lines, prefix+text[:room])
		text = text[room:]
		prefix = ";    -  "
		room = MaxLine - len(prefix)
	}

	w.lines = append(w.lines, strings.TrimRight(prefix+text, " "))
}

// padTo pads b with tabs to column (or with one blank, if b already
// reaches it), counting tabs as moving to the next multiple of 8.
func padTo(b *strings.Builder, column int) {
	width := columnWidth(b.String())

	if width >= column {
		b.WriteByte(' ')

		return
	}

	for width < column {
		b.WriteByte('\t')
		width = (width/tabWidth + 1) * tabWidth
	}
}

// columnWidth is how many columns text takes when shown, tabs moving to
// the next multiple of 8.
func columnWidth(text string) int {
	width := 0

	for i := 0; i < len(text); i++ {
		if text[i] == '\t' {
			width = (width/tabWidth + 1) * tabWidth
		} else {
			width++
		}
	}

	return width
}

// printable replaces the characters a MACRO source line shouldn't hold
// (control characters other than tab, and DEL) with "?", so a stray form
// feed or escape in a FORTRAN source can't upset MACRO or a terminal
// showing the .MAR file.
func printable(text string) string {
	clean := true

	for i := 0; i < len(text); i++ {
		if c := text[i]; (c < ' ' && c != '\t') || c == 0x7F {
			clean = false

			break
		}
	}

	if clean {
		return text
	}

	b := []byte(text)

	for i, c := range b {
		if (c < ' ' && c != '\t') || c == 0x7F {
			b[i] = '?'
		}
	}

	return string(b)
}

// splitText splits text into pieces of at most width characters, each
// break at the last blank within the width if there is one, else at the
// width.
func splitText(text string, width int) []string {
	var pieces []string

	for len(text) > width {
		cut := strings.LastIndexByte(text[:width+1], ' ')
		if cut <= 0 {
			cut = width
		}

		pieces = append(pieces, strings.TrimRight(text[:cut], " "))
		text = strings.TrimLeft(text[cut:], " ")
	}

	return append(pieces, text)
}
