package anl

import (
	"io"
	"strings"
)

// Line is one line of a report, before it's laid out on pages.
type Line struct {
	// Text is the line, without a line end. Tabs are real tabs, as
	// ANALYZE writes them.
	Text string

	// Keep, when more than 1, asks that this line and the Keep-1 lines
	// after it start on the same page: a heading and what follows it.
	Keep int

	// Page starts a new page before this line (the summary does).
	Page bool
}

// report collects an analyzer's lines.
type report struct {
	lines []Line
}

// line adds a line.
func (r *report) line(text string) {
	r.lines = append(r.lines, Line{Text: text})
}

// blank adds an empty line.
func (r *report) blank() {
	r.line("")
}

// page adds a line that starts a new page.
func (r *report) page(text string) {
	r.lines = append(r.lines, Line{Text: text, Page: true})
}

// WriteText writes lines as plain text, one per line, with no page
// layout: what a report says, for tests and for a quick look.
func WriteText(w io.Writer, lines []Line) error {
	var b strings.Builder

	for _, l := range lines {
		b.WriteString(l.Text)
		b.WriteByte('\n')
	}

	_, err := io.WriteString(w, b.String())

	return err
}
