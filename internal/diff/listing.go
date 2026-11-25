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

// Merged lists res's sections in the merged form: for each, master's
// records, then revision's, each side followed by after records that match
// (/MERGED; a negative count is none, as VMS's /MERGED=-1 was).
func Merged(l *Listing, master, revision Input, res Result, after int32) {
	for _, s := range res.Sections {
		l.Cut(strings.Repeat("*", 12))
		side(l, master, s.Master, after)
		l.Cut(strings.Repeat("*", 6))
		side(l, revision, s.Revision, after)
		l.Cut(strings.Repeat("*", 12))
	}
}

// side lists one file's part of a section, and the after records that
// follow it.
func side(l *Listing, in Input, span Span, after int32) {
	l.Cut("File " + in.Name)

	end := min(span.End+max(int(after), 0), len(in.Records))

	for r := span.Start; r < end; r++ {
		l.Cut(RecordLine(r+1, in.Records[r]))
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
