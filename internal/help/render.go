package help

import "strings"

// cell is the width of a column of a list of topics. A key takes as many
// cells as it needs with one blank after it: a key of up to 10 characters
// one cell, of 11 to 21 two, and so on (testdata/help90's WIDE).
const cell = 11

// Render lays out HELP's displays at width columns (80 at a terminal, 132
// for a file), one string per line, as VMS 7.1's HELP lays them out
// (testdata/help90). Each display starts with an empty line:
//
//	(empty)
//	ALPHA                                       the headings, each after an
//	(empty)                                     empty line and indented 2
//	  Parameters                                more than the one before
//	(empty)
//	       Text of ALPHA Parameters.            the text, shifted 2 per level
//	(empty)
//	(empty)
//	    Additional information available:       if the topic has subtopics
//	(empty)
//	    filespec   count                        in cells of 11 columns
//	(empty)
//
// A "Sorry" display has no empty line between the last heading and its
// message, and one that found nothing at level 1 has no headings, and so
// no empty line before it.
func Render(displays []Display, width int) []string {
	var out []string

	for _, d := range displays {
		out = append(out, d.render(width)...)
	}

	return out
}

func (d Display) render(width int) []string {
	var out []string

	for i, h := range d.Headings {
		out = append(out, "", indent(i)+h)
	}

	listTitle := "Additional information available:"

	switch d.Kind {
	case ShowTopic, ShowSlash:
		out = append(out, "")
		for _, line := range d.Text {
			out = append(out, shift(line, d.Depth))
		}
	case ShowSorry:
		out = append(out, indent(max(len(d.Headings), 1))+"Sorry, no documentation on "+d.Sorry)
	case ShowList:
		listTitle = "Information available:"
	}

	if d.Listed {
		if d.Kind != ShowList {
			out = append(out, "")
		}

		out = append(out, "", indent(d.ListDepth)+listTitle, "")
		out = append(out, listLines(d.List, d.ListDepth, width)...)
	}

	return append(out, "")
}

// indent is n steps of 2 blanks.
func indent(n int) string { return strings.Repeat("  ", n) }

// shift moves a line of text right by n steps of 2 blanks. An empty line
// stays empty, and a line starting in column 1 or with a tab moves too.
// Blanks at a line's end are dropped, but not its last character: a line
// of one blank is shown as the shift and a blank (VMS's own HELPLIB.HLB
// has both kinds of line; LIBRARY/HELP now stores a line of blanks as an
// empty record).
func shift(line string, n int) string {
	if line == "" {
		return ""
	}

	end := len(line)
	for end > 1 && line[end-1] == ' ' {
		end--
	}

	return indent(n) + line[:end]
}

// listLines lays out a list of topics in cells, depth steps of 2 blanks
// in. A run of qualifier lines' keys starts a line of its own, and so does
// the first numbered key after one (testdata/help90's ALPHA lists
// "Parameters Qualifiers", then Qualifiers' "/BRIEF /COUNT /NOCONFIRM",
// then "/DIRECT Description"). A key goes on a line if its text ends by
// column width-3, even if the rest of its cell doesn't: at 132 columns a
// key ending at column 129 stays and one ending at 130 starts a new line
// (testdata/help90's WIDE, and VMS's ANALYZE /AUDIT /SELECT in round 1's
// part G). That the margin is the same at other widths is unconfirmed.
func listLines(list []Entry, depth, width int) []string {
	var (
		out  []string
		line strings.Builder
	)

	start := 2 * depth
	col := start

	flush := func() {
		if line.Len() > 0 {
			out = append(out, strings.TrimRight(line.String(), " "))
			line.Reset()
		}

		col = start
	}

	for i, e := range list {
		if i > 0 && e.Slash != list[i-1].Slash {
			flush()
		}

		if col > start && col+len(e.Key) > width-3 {
			flush()
		}

		if col == start {
			line.WriteString(indent(depth))
		}

		cells := (len(e.Key) + 1 + cell - 1) / cell
		line.WriteString(e.Key)
		line.WriteString(strings.Repeat(" ", cells*cell-len(e.Key)))
		col += cells * cell
	}

	flush()

	return out
}
