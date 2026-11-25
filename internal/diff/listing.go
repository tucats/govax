package diff

import (
	"fmt"
	"strings"
)

// MinWidth is the narrowest listing: VMS 7.1 took /WIDTH=0 as 12
// (testdata/dif116's N37).
const MinWidth = 12

// Listing is the lines of a listing being written, at a width.
type Listing struct {
	Width int
	Lines []string
}

// NewListing starts a listing width columns wide (at least MinWidth).
func NewListing(width int) *Listing {
	return &Listing{Width: max(width, MinWidth)}
}

// Cut adds a line cut at the width, as DIFFERENCES cuts a record's line or
// a "File" line: what doesn't fit is left out.
func (l *Listing) Cut(line string) {
	if len(line) > l.Width {
		line = line[:l.Width]
	}

	l.Lines = append(l.Lines, line)
}

// Wrap adds a line broken to the width, as DIFFERENCES breaks its counts:
// a piece of one less than the width and a "-", until what is left fits.
func (l *Listing) Wrap(line string) {
	w := newWrapper(l.Width)
	w.write(line)
	l.Lines = append(l.Lines, w.finish()...)
}

// Text is the listing as one string, each line ending in a newline.
func (l *Listing) Text() string {
	if len(l.Lines) == 0 {
		return ""
	}

	return strings.Join(l.Lines, "\n") + "\n"
}

// Input is one file as the listing names it: its full name and its
// records.
type Input struct {
	Name    string
	Records [][]byte
}

// Style is how a form writes a section's records.
type Style struct {
	// After is how many matched records follow each side of a section
	// (/MERGED's or /PARALLEL's value; a negative count is none, as VMS's
	// /MERGED=-1 was).
	After int32

	// NoNumber leaves out the records' numbers (/NONUMBER).
	NoNumber bool

	// Closing ends a merged listing with a section of no records, as VMS
	// 7.1 ended every merged listing written after one of the records as
	// text (testdata/dif116's F21, F22, F31, F33; not R13's octal one after
	// the hexadecimal; List decides).
	Closing bool

	// Radix is the records' radix: as text, or dumped (radix.go).
	Radix Radix
}

// Merged lists res's sections in the merged form: for each, master's
// records, then revision's, each side followed by st.After records that
// match.
func Merged(l *Listing, master, revision Input, res Result, st Style) {
	after := st.After

	for k, s := range res.Sections {
		if res.Stopped && k == len(res.Sections)-1 {
			after = 0
		}

		l.Cut(strings.Repeat("*", 12))
		side(l, master, s.Master, after, st)
		l.Cut(strings.Repeat("*", 6))
		side(l, revision, s.Revision, after, st)
		l.Cut(strings.Repeat("*", 12))
	}

	if st.Closing {
		l.Cut(strings.Repeat("*", 12))
		l.Cut("File " + master.Name)
		l.Cut(strings.Repeat("*", 6))
		l.Cut("File " + revision.Name)
		l.Cut(strings.Repeat("*", 12))
	}
}

// side lists one file's part of a section, and the after records that
// follow it.
func side(l *Listing, in Input, span Span, after int32, st Style) {
	l.Cut("File " + in.Name)

	end := min(span.End+max(int(after), 0), len(in.Records))

	for r := span.Start; r < end; r++ {
		record(l, r+1, in.Records[r], st)
	}
}

// record lists a record in the merged and separated forms: in a radix, its
// dump (dumpRecord); as text, its line with its number (RecordLine) or,
// under /NONUMBER, its text alone (F15).
func record(l *Listing, number int, text []byte, st Style) {
	switch {
	case st.Radix != ASCII:
		dumpRecord(l, newDumpLayout(st.Radix, l.Width), number, text, false)
	case st.NoNumber:
		l.Cut(string(text))
	default:
		l.Cut(RecordLine(number, text))
	}
}

// RecordLine is a record's line: its number right-aligned in five columns
// (more, if it needs them), three blanks, and its text.
func RecordLine(number int, text []byte) string {
	return fmt.Sprintf("%5d   %s", number, text)
}

// Summary writes the counts that end every listing: after a blank line if
// any section was listed, then a blank line before the echo.
func Summary(l *Listing, res Result) {
	if len(res.Sections) > 0 {
		l.Lines = append(l.Lines, "")
	}

	l.Wrap(fmt.Sprintf("Number of difference sections found: %d", len(res.Sections)))
	l.Wrap(fmt.Sprintf("Number of difference records found: %d", res.Records))
	l.Lines = append(l.Lines, "")
}
