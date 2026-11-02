package listing

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/tucats/govax/internal/fortran/diag"
	"github.com/tucats/govax/internal/fortran/source"
)

// The page's size and the heading's columns. A page is as long as a line printer's
// at six lines an inch, and as wide, as MACRO's listing is. A source line of
// /EXTEND_SOURCE with its number runs past the width: a listing never cuts one.
const (
	pageLines    = 66
	headingLines = 3  // the heading's two lines and a blank one
	titleColumn  = 41 // where the date and the file's name start
	dateWidth    = 20
	versionWidth = 30
	maxHeading   = 31 // the most a TITLE or SUBTITLE can show (section 10.3.3)
	tabStop      = 8
)

// Input is everything a listing shows.
type Input struct {
	// Lines are the source's lines, as the source pass read them.
	Lines []*source.Line

	// Messages are the compilation's messages, in the order reported.
	Messages []diag.Message

	// Units say where each program unit starts, in order. A compilation stopped
	// by the source pass has none.
	Units []Unit

	// Headings are the file's TITLE and SUBTITLE directives that give a string.
	Headings []Heading

	// Maps are the program units' storage maps. A compilation with errors has none.
	Maps []Map

	// Show is what /SHOW asks for.
	Show Show

	// Title heads a page no unit names, and File names the file a page shows
	// if no line has. Both come from the first source file.
	Title, File string

	// Version and Compiled are the compiler's version and the compilation's time.
	Version  string
	Compiled time.Time
}

// Unit is where a program unit starts: Line is the index in Input.Lines just
// past the unit before it, so a unit's opening comments are with it.
type Unit struct {
	Line int
	Name string
}

// Heading is a TITLE or SUBTITLE directive, on the line of index Line.
type Heading struct {
	Line     int
	Subtitle bool
	Text     string
}

// Map is a program unit's storage map, a line of text each.
type Map struct {
	Unit  string
	Lines []string
}

// Make lays out the listing, a string per line, each page's first starting with a
// form feed.
func Make(in Input) []string {
	p := &pager{in: in, unit: in.Title, file: in.File, needPage: true}

	p.source()
	p.maps()

	if p.page == 0 {
		p.newPage()
	}

	return p.out
}

// source writes the source's lines, each under the page its directives leave alone,
// and last the messages that belong to no line. A new unit starts a page, so that
// the unit's name can head it, as does each file of a compilation of several.
func (p *pager) source() {
	about := map[int][]diag.Message{}

	var unplaced []diag.Message

	lines := map[int]bool{}
	for _, line := range p.in.Lines {
		lines[line.Serial] = true
	}

	for _, m := range p.in.Messages {
		if lines[m.Pos.Serial] {
			about[m.Pos.Serial] = append(about[m.Pos.Serial], m)
		} else {
			unplaced = append(unplaced, m)
		}
	}

	shown := func(line *source.Line) bool {
		return line.Listed || p.in.Show.Include || len(about[line.Serial]) > 0
	}

	width := 1

	for _, line := range p.in.Lines {
		if shown(line) {
			width = max(width, len(strconv.Itoa(line.Number)))
		}
	}

	units := map[int]string{}
	for _, u := range p.in.Units {
		units[u.Line] = u.Name
	}

	headings := map[int][]Heading{}
	for _, h := range p.in.Headings {
		headings[h.Line] = append(headings[h.Line], h)
	}

	var compiled *source.File

	for i, line := range p.in.Lines {
		if name, ok := units[i]; ok {
			p.unit, p.needPage = name, true
		}

		if line.Depth == 0 && line.File != compiled {
			compiled, p.file = line.File, line.File.Spec
			p.needPage = true
		}

		if shown(line) {
			p.keep(lineWithMessages(line, width, about[line.Serial]))
		}

		for _, h := range headings[i] {
			p.pend(h)
		}
	}

	if len(unplaced) > 0 {
		p.blank(1)
	}

	for _, m := range unplaced {
		p.keep(strings.Split(m.String(), "\n"))
	}
}

// maps writes the storage maps. A page these start has the unit's name.
func (p *pager) maps() {
	if !p.in.Show.Map {
		return
	}

	for i, m := range p.in.Maps {
		p.unit = m.Unit

		if i == 0 {
			p.blank(2)
		} else {
			p.blank(1)
		}

		p.keep(m.Lines)
	}
}

// lineWithMessages is a source line as the listing shows it, then for each message
// about it a pointer to its column and its first line. The second line of a
// message on the terminal says where it is, which here doesn't need saying.
func lineWithMessages(line *source.Line, width int, messages []diag.Message) []string {
	depth := ""
	if line.Depth > 0 {
		depth = strconv.Itoa(line.Depth)
	}

	prefix := fmt.Sprintf("%3s %*d  ", depth, width, line.Number)
	text := expand(line.Text)
	out := []string{strings.TrimRight(prefix+text, " ")}

	for _, m := range messages {
		if m.Pos.Column > 0 {
			out = append(out, strings.Repeat(" ", len(prefix)+columnWidth(line.Text, m.Pos.Column))+"^")
		}

		out = append(out, m.Headline())
	}

	return out
}

// expand is a source line with its tabs made blanks, so its columns on a page are
// those a terminal shows, whatever comes before it, and any other control
// character made a blank, so a form feed in a source starts no page.
func expand(text string) string {
	var b strings.Builder

	for i := range len(text) {
		switch c := text[i]; {
		case c == '\t':
			b.WriteString(strings.Repeat(" ", tabStop-b.Len()%tabStop))
		case c < ' ' || c == 0x7F:
			b.WriteByte(' ')
		default:
			b.WriteByte(c)
		}
	}

	return b.String()
}

// columnWidth is how far along a page a column of a source line is: the width of
// what comes before it, the column counted from 1 in the line's characters, as a
// message's is. A column past the line's end is that far past on the page.
func columnWidth(text string, column int) int {
	before := column - 1
	if before <= len(text) {
		return len(expand(text[:before]))
	}

	return before + len(expand(text)) - len(text)
}

// pager is Make's state: the lines so far, and what the page's heading says.
type pager struct {
	in  Input
	out []string

	// page is the page's number, and used how many of its lines are written.
	// needPage says the next line starts a page, whatever room this one has.
	page, used int
	needPage   bool

	// unit is the unit being listed, whose name heads a page with no TITLE, and
	// file the file a page names.
	unit, file string

	// title and subtitle are the directives' strings, which titled says the first
	// has given. pending holds those read since the page began: they head the next.
	title, subtitle string
	titled          bool
	pending         []Heading
}

// pend notes a directive on this page.
func (p *pager) pend(h Heading) {
	p.pending = append(p.pending, h)
}

// newPage starts a page: the directives on the last one take effect, the last of
// each kind winning, then the heading.
func (p *pager) newPage() {
	for _, h := range p.pending {
		if h.Subtitle {
			p.subtitle = h.Text
		} else {
			p.title, p.titled = h.Text, true
		}
	}

	p.pending = nil
	p.page++
	p.needPage = false

	title := p.unit
	if p.titled {
		title = p.title
	}

	p.out = append(p.out,
		fmt.Sprintf("\f%-*s%-*s %-*sPage %d", titleColumn, cut(title), dateWidth, dateTime(p.in.Compiled),
			versionWidth, "govax FORTRAN V"+p.in.Version, p.page),
		fmt.Sprintf("%-*s%s", titleColumn, cut(p.subtitle), p.file),
		"")
	p.used = headingLines
}

// cut is a heading's string as long as its field lets it be.
func cut(text string) string {
	if len(text) > maxHeading {
		return text[:maxHeading]
	}

	return text
}

// dateTime is t as the heading shows it: 2-NOV-2026 14:05:11.
func dateTime(t time.Time) string {
	return strings.ToUpper(t.Format("2-Jan-2006 15:04:05"))
}

// keep writes lines on one page, on a new one if they don't fit on this one but
// would on a page of their own.
func (p *pager) keep(lines []string) {
	if p.page == 0 || p.needPage || (p.used+len(lines) > pageLines && len(lines) <= pageLines-headingLines) {
		p.newPage()
	}

	for _, line := range lines {
		if p.used >= pageLines {
			p.newPage()
		}

		p.out = append(p.out, line)
		p.used++
	}
}

// blank writes n blank lines, unless the next line starts a page anyway.
func (p *pager) blank(n int) {
	if p.page == 0 || p.needPage || p.used+n >= pageLines {
		return
	}

	for range n {
		p.out = append(p.out, "")
	}

	p.used += n
}
