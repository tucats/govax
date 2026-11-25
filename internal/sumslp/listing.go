package sumslp

import (
	"fmt"
	"strings"
	"time"
)

// Heading is what a listing's page headings say: when the run was, and the
// input's creation date and time and full name.
type Heading struct {
	Run, Created time.Time
	Input        string
}

// The listing's layout (the probe's L01 to L03, P01 to P04).
const (
	// PageLines is the lines of the file (and messages) on a page, after
	// its two heading lines and a blank line.
	PageLines = 57

	// FieldWidth is the audit trail's (or deletion marker's) field, and
	// NumberWidth the line number's, right-aligned.
	FieldWidth  = 16
	NumberWidth = 7

	// MaxLine is the longest listing line: a longer one is cut (W255).
	MaxLine = 255

	// version is the utility's name in the first heading line.
	version = "VAX-11 SUMSLP V01.00"

	// nameWidth is the room for the input's name in the second heading
	// line: a longer one is cut, a shorter one blank-padded (P04).
	nameWidth = 45
)

// Listing lays lines out on pages, each started by its heading when its
// first line is written: a listing with no lines has no records (L02).
func Listing(lines []Line, h Heading) [][]byte {
	var out [][]byte

	page := 0

	for i, l := range lines {
		if i%PageLines == 0 {
			page++
			out = append(out, headingLines(h, page)...)
		}

		out = append(out, lineRecord(l))
	}

	return out
}

// headingLines are page page's two heading lines and the blank line after
// them:
//
//	\f, 60 blanks, the run's time, 4 blanks, the version, 16 blanks, "Page",
//	the number right-aligned in 5 (130 bytes)
//	60 blanks, the input's creation time, 4 blanks, its name in 45 (129)
func headingLines(h Heading, page int) [][]byte {
	blanks := strings.Repeat(" ", 60)

	name := h.Input
	if len(name) > nameWidth {
		name = name[:nameWidth]
	}

	first := fmt.Sprintf("\f%s%s    %s%sPage%5d", blanks, timeText(h.Run), version, strings.Repeat(" ", 16), page)
	second := fmt.Sprintf("%s%s    %-*s", blanks, timeText(h.Created), nameWidth, name)

	return [][]byte{[]byte(first), []byte(second), {}}
}

// lineRecord is a line as the listing writes it.
func lineRecord(l Line) []byte {
	var b []byte

	if l.Number == "" {
		b = append(b, l.Text...)
	} else {
		field := l.Field
		if len(field) > FieldWidth {
			field = field[:FieldWidth]
		}

		b = fmt.Appendf(b, "%-*s%*s ", FieldWidth, field, NumberWidth, l.Number)
		b = append(b, l.Text...)
	}

	if len(b) > MaxLine {
		b = b[:MaxLine]
	}

	return b
}

// timeText is a time as the headings show it, "25-NOV-2026 14:37:46": VMS's
// date and time to the second. A day before the 10th is shown with a blank
// before it, as $ASCTIM shows it (unconfirmed: the probe ran on the 25th).
func timeText(t time.Time) string {
	return fmt.Sprintf("%2d-%s-%04d %02d:%02d:%02d", t.Day(), strings.ToUpper(t.Month().String()[:3]),
		t.Year(), t.Hour(), t.Minute(), t.Second())
}
