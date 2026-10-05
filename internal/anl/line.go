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

	// Keep, when more than 1, is how many lines must be left on the page
	// for this line to be written there, rather than on a new page: a
	// heading asks for room for what follows it.
	Keep int

	// Spill writes the line on the current page even when the page is
	// full: the blank lines that close a record do.
	Spill bool

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

// keep adds a line that needs n lines left on the page.
func (r *report) keep(n int, text string) {
	r.lines = append(r.lines, Line{Text: text, Keep: n})
}

// spill adds a blank line that's written even on a full page.
func (r *report) spill() {
	r.lines = append(r.lines, Line{Spill: true})
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
