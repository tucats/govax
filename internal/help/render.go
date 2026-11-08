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

	for _, l := range Lines(displays, width) {
		out = append(out, l.String())
	}

	return out
}

// LineFlags say what a Line holds, as LBR$GET_HELP tells its routine
// (HLP$M_NOHLPTXT, HLP$M_KEYNAMLIN, HLP$M_OTHERINFO). None set is a topic's
// text.
type LineFlags uint32

const (
	// NoText marks the lines of a display that found nothing: its
	// headings, the "Sorry" line, and what follows them.
	NoText LineFlags = 1
	// KeyName marks a heading, and the empty line before it.
	KeyName LineFlags = 2
	// OtherInfo marks the list of subtopics ("Additional information
	// available:"), from the empty line before its title to its end.
	OtherInfo LineFlags = 4
)

// Line is one line of HELP's output, apart: its text without the blanks
// HELP puts before it, how many those are, what kind of line it is, and its
// level. LBR$GET_HELP passes a routine the text, the flags, and the level
// (testdata/help90's LBR90, round 1b); Render puts the indentation back.
//
// A heading's level is its own (1 for the topic the module is); every other
// line's is the number of headings its display has, 0 for a display with
// none (a first word that named nothing).
type Line struct {
	Text   string
	Indent int
	Flags  LineFlags
	Level  int
}

// String is the line as HELP shows it.
func (l Line) String() string {
	if l.Text == "" {
		return ""
	}

	return strings.Repeat(" ", l.Indent) + l.Text
}

// Lines lays out HELP's displays at width columns, as Render does, as
// Lines.
func Lines(displays []Display, width int) []Line {
	var out []Line

	for _, d := range displays {
		out = append(out, d.lines(width)...)
	}

	return out
}

func (d Display) lines(width int) []Line {
	var out []Line

	// base is the flags of the display's own lines.
	var base LineFlags
	if d.Kind == ShowSorry {
		base = NoText
	}

	level := len(d.Headings)

	for i, h := range d.Headings {
		f := base | KeyName
		out = append(out, Line{Flags: f, Level: i + 1}, Line{Text: h, Indent: 2 * i, Flags: f, Level: i + 1})
	}

	listTitle := "Additional information available:"

	switch d.Kind {
	case ShowTopic, ShowSlash:
		out = append(out, Line{Level: level})
		for _, line := range d.Text {
			text, n := shift(line, d.Depth)
			out = append(out, Line{Text: text, Indent: n, Level: level})
		}
	case ShowSorry:
		out = append(out, Line{
			Text: "Sorry, no documentation on " + d.Sorry, Indent: 2 * max(level, 1), Flags: NoText, Level: level,
		})
	case ShowList:
		listTitle = "Information available:"
	}

	last := base

	if d.Listed {
		if d.Kind != ShowList {
			out = append(out, Line{Flags: base, Level: level})
		}

		f := base | OtherInfo
		at := 2 * d.ListDepth

		out = append(out, Line{Flags: f, Level: level}, Line{Text: listTitle, Indent: at, Flags: f, Level: level},
			Line{Flags: f, Level: level})

		for _, line := range listLines(d.List, d.ListDepth, width) {
			out = append(out, Line{Text: strings.TrimPrefix(line, indent(d.ListDepth)), Indent: at, Flags: f, Level: level})
		}

		if len(d.Libraries) > 0 {
			out = append(out, Line{Flags: f, Level: level},
				Line{Text: "Additional help libraries available (type @name for topics):", Indent: 2, Flags: f, Level: level},
				Line{Flags: f, Level: level},
				Line{Text: strings.Join(d.Libraries, "  "), Indent: 2, Flags: f, Level: level})
		}

		last = f
	}

	return append(out, Line{Flags: last, Level: level})
}

// indent is n steps of 2 blanks.
func indent(n int) string { return strings.Repeat("  ", n) }

// shift moves a line of text right by n steps of 2 blanks: it returns the
// text and how many blanks go before it. An empty line stays empty, and a
// line starting in column 1 or with a tab moves too. Blanks at a line's end
// are dropped, but not its last character: a line of one blank is shown as
// the shift and a blank (VMS's own HELPLIB.HLB has both kinds of line;
// LIBRARY/HELP now stores a line of blanks as an empty record).
func shift(line string, n int) (string, int) {
	if line == "" {
		return "", 0
	}

	end := len(line)
	for end > 1 && line[end-1] == ' ' {
		end--
	}

	return line[:end], 2 * n
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
