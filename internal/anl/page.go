package anl

import (
	"fmt"
	"io"
	"strings"
	"time"
)

// Titles of the reports, as each page header shows them.
const (
	TitleObject = "Analyze Object File"
	TitleImage  = "Analyze Image"
)

// version is the ANALYZE version each page header shows: VMS 7.3's.
const version = "ANALYZ V07-04"

// Page geometry, as real ANALYZE lays its output out.
const (
	// pageLines is how many lines of a report fit on a page below the
	// page header (the Spill lines aside).
	pageLines = 55

	// titleWidth is the column the date in a page header starts at.
	titleWidth = 45

	// commandWidth is the width the closing command line is padded to.
	commandWidth = 80
)

// Pager lays a report's lines out on pages: a header at the top of each
// (a form feed, the title, the date and time, the page number, the file
// analyzed, and ANALYZE's version), and the command line at the end.
type Pager struct {
	// Title is the report's title (TitleObject, TitleImage), and File
	// the full specification of the file analyzed.
	Title string
	File  string

	// Command is the command line that ran ANALYZE, shown at the end.
	Command string

	// Now is the clock each page header's time is read from, time.Now
	// when nil.
	Now func() time.Time

	w     io.Writer
	err   error
	page  int // the current page's number, 0 before the first
	count int // lines written on the current page
}

// NewPager returns a pager writing to w.
func NewPager(w io.Writer, title, file, command string) *Pager {
	return &Pager{Title: title, File: file, Command: command, w: w}
}

// Write lays out lines, starting pages as they need.
func (p *Pager) Write(lines []Line) error {
	for _, l := range lines {
		need := max(l.Keep, 1)

		switch {
		case p.page == 0 || l.Page:
			p.newPage()
		case l.Spill:
		case p.count+need > pageLines:
			p.newPage()
		}

		p.text(l.Text + "\n")
		p.count++
	}

	return p.err
}

// Close ends the report with its command line.
func (p *Pager) Close() error {
	p.text(fmt.Sprintf("%-*s\n", commandWidth, p.Command))

	return p.err
}

// newPage starts a page with its header.
func (p *Pager) newPage() {
	p.page++
	p.count = 0

	now := time.Now
	if p.Now != nil {
		now = p.Now
	}

	p.text(fmt.Sprintf("\f\n%-*s%s   Page %d\n%s\n%s\n\n", titleWidth, p.Title, vmsTime(now()), p.page, p.File, version))
}

func (p *Pager) text(s string) {
	if p.err == nil {
		_, p.err = io.WriteString(p.w, s)
	}
}

// vmsTime is a time as VMS shows it to the hundredth of a second, with
// the day blank-padded: " 2-OCT-2026 19:03:18.27".
func vmsTime(t time.Time) string {
	return fmt.Sprintf("%2d-%s-%04d %02d:%02d:%02d.%02d", t.Day(), strings.ToUpper(t.Format("Jan")), t.Year(),
		t.Hour(), t.Minute(), t.Second(), t.Nanosecond()/10_000_000)
}
