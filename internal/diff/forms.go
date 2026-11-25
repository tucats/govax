package diff

import (
	"fmt"
	"strconv"
	"strings"
)

// The listing forms other than the merged one, and the order a command's
// forms are written in, as VMS 7.1 wrote them (testdata/dif116's FORMS
// section, N64, and the generated pairs run /PARALLEL and /SEPARATED).

// Bar is one file's /CHANGE_BAR: the character that marks a changed
// record, and whether the records are numbered.
type Bar struct {
	Char     byte
	NoNumber bool
}

// Forms are the listings a command asked for.
type Forms struct {
	// Merged is the merged form, with its style.
	Merged      bool
	MergedStyle Style

	// Parallel is the parallel form, with its style.
	Parallel      bool
	ParallelStyle Style

	// SeparatedMaster and SeparatedRevision are /SEPARATED's two
	// listings; NoNumber is /NONUMBER for them.
	SeparatedMaster, SeparatedRevision bool
	NoNumber                           bool

	// MasterBar and RevisionBar are each file's /CHANGE_BAR, nil for
	// none.
	MasterBar, RevisionBar *Bar
}

// List writes the listings forms asks for, in VMS's order: the parallel
// form first, then the merged form, the separated ones (the master's, then
// the revision's), and the change bars (the master's, then the revision's),
// each after a blank line but the first; a merged listing that isn't the
// first ends with an empty section (Style.Closing). Files that don't differ
// get no listing at all (F45). A comparison /MAXIMUM_DIFFERENCES stopped
// gets only the first form (L02), the master's and revision's separated
// listings counting as one.
func List(l *Listing, master, revision Input, res Result, forms Forms) {
	if len(res.Sections) == 0 {
		return
	}

	first := true

	// start begins a listing, returning false if the comparison stopped and
	// a form was already written.
	start := func() bool {
		if !first {
			if res.Stopped {
				return false
			}

			l.Lines = append(l.Lines, "")
		}

		first = false

		return true
	}

	if forms.Parallel && start() {
		Parallel(l, master, revision, res, forms.ParallelStyle)
	}

	if forms.Merged && start() {
		st := forms.MergedStyle
		st.Closing = l.listed()
		Merged(l, master, revision, res, st)
	}

	if forms.SeparatedMaster || forms.SeparatedRevision {
		if start() {
			if forms.SeparatedMaster {
				Separated(l, master, res, masterSpan, forms.NoNumber)
			}

			if forms.SeparatedMaster && forms.SeparatedRevision {
				l.Lines = append(l.Lines, "")
			}

			if forms.SeparatedRevision {
				Separated(l, revision, res, revisionSpan, forms.NoNumber)
			}
		}
	}

	if forms.MasterBar != nil && start() {
		ChangeBars(l, master, Marks(res, len(master.Records), masterSpan), *forms.MasterBar)
	}

	if forms.RevisionBar != nil && start() {
		ChangeBars(l, revision, Marks(res, len(revision.Records), revisionSpan), *forms.RevisionBar)
	}
}

// listed says something was already written to the listing.
func (l *Listing) listed() bool { return len(l.Lines) > 0 }

// masterSpan and revisionSpan pick one file's side of a section.
func masterSpan(s Section) Span   { return s.Master }
func revisionSpan(s Section) Span { return s.Revision }

// Separated lists one file's records of every section under one header
// (/SEPARATED, F06 to F09):
//
//	************
//	File DUA1:[DIF116.IN]F01.A;1
//	    5   data line 5
//	   26   data line 26
//	   27   data line 27
//	************
func Separated(l *Listing, in Input, res Result, span func(Section) Span, noNumber bool) {
	l.Cut(strings.Repeat("*", 12))
	l.Cut("File " + in.Name)

	for _, s := range res.Sections {
		sp := span(s)

		for r := sp.Start; r < min(sp.End, len(in.Records)); r++ {
			l.Cut(recordLine(r+1, in.Records[r], noNumber))
		}
	}

	l.Cut(strings.Repeat("*", 12))
}

// Marks says which of a file's n records a change bar marks: a section's
// records on that file's side, or, where its side has none (an insertion in
// the other file), the record after the place, the one /MERGED=1 would show
// (F40's master record 16, F41's revision record 28).
func Marks(res Result, n int, span func(Section) Span) []bool {
	marks := make([]bool, n)

	for _, s := range res.Sections {
		sp := span(s)

		if sp.Len() == 0 {
			if sp.Start < n {
				marks[sp.Start] = true
			}

			continue
		}

		for r := sp.Start; r < min(sp.End, n); r++ {
			marks[r] = true
		}
	}

	return marks
}

// ChangeBars lists the whole of one file, each record marked by marks with
// the bar's character (/CHANGE_BAR on a file, F40 to F42; N64 under
// /NONUMBER):
//
//	************
//	File DUA1:[DIF116.IN]F01.B;1
//	    4   data line 4
//	    5 # data line 5, revised
//	************
func ChangeBars(l *Listing, in Input, marks []bool, bar Bar) {
	l.Cut(strings.Repeat("*", 12))
	l.Cut("File " + in.Name)

	for r, text := range in.Records {
		mark := byte(' ')
		if marks[r] {
			mark = bar.Char
		}

		if bar.NoNumber {
			l.Cut(fmt.Sprintf("%c %s", mark, text))
		} else {
			l.Cut(fmt.Sprintf("%5d %c %s", r+1, mark, text))
		}
	}

	l.Cut(strings.Repeat("*", 12))
}

// Parallel lists res's sections side by side (/PARALLEL, F10 to F16, F46,
// F47), every line one less than the width:
//
//	--------------------------------------------- (a rule)
//	File master            |  File revision
//	---------- 5 --------------------- 5 -------- (each side's first record)
//	data line 5            |  data line 5, revised
//	---------------------------------------------
//
// The line is two halves of (width-1)/2, the bar at the second's first
// column; each side's text, in the printable form (Pretty), is cut two
// columns short of the bar, and the right side's starts two after it. A
// section's rule has each side's first record number, " n ", from the middle
// of its half (none under /NONUMBER). Each section's rows pair the master's
// records with the revision's, a side that runs out blank, then st.After
// rows of the records that follow both.
func Parallel(l *Listing, master, revision Input, res Result, st Style) {
	w := l.Width - 1
	half := w / 2
	leftWidth := max(half-2, 0)
	rightWidth := max(w-half-3, 0)

	rule := strings.Repeat("-", w)

	row := func(a, b string) {
		l.Lines = append(l.Lines, pad(a, leftWidth)+"  |  "+pad(b, rightWidth))
	}

	text := func(in Input, r int) string {
		if r < 0 || r >= len(in.Records) {
			return ""
		}

		return Pretty(in.Records[r])
	}

	l.Lines = append(l.Lines, rule)
	row("File "+master.Name, "File "+revision.Name)

	after := max(int(st.After), 0)

	for k, s := range res.Sections {
		if res.Stopped && k == len(res.Sections)-1 {
			after = 0
		}

		sep := []byte(rule)
		if !st.NoNumber {
			put(sep, half/2, " "+strconv.Itoa(s.Master.Start+1)+" ")
			put(sep, half+1+half/2, " "+strconv.Itoa(s.Revision.Start+1)+" ")
		}

		l.Lines = append(l.Lines, string(sep))

		for t := range max(s.Master.Len(), s.Revision.Len()) {
			a, b := "", ""
			if t < s.Master.Len() {
				a = text(master, s.Master.Start+t)
			}

			if t < s.Revision.Len() {
				b = text(revision, s.Revision.Start+t)
			}

			row(a, b)
		}

		for t := range after {
			row(text(master, s.Master.End+t), text(revision, s.Revision.End+t))
		}
	}

	l.Lines = append(l.Lines, rule)
}

// pad is text cut or filled with blanks to width.
func pad(text string, width int) string {
	if len(text) >= width {
		return text[:width]
	}

	return text + strings.Repeat(" ", width-len(text))
}

// put writes text over line from column at, as much as fits.
func put(line []byte, at int, text string) {
	for i := 0; i < len(text) && at+i < len(line); i++ {
		line[at+i] = text[i]
	}
}

// Pretty is a record's text in DIFFERENCES' printable form (/PARALLEL's
// always, /IGNORE=PRETTY's; R11, R12): tabs expanded to every eighth
// column, LF, VT, FF, and CR as <LF>, <VT>, <FF>, and <CR>, and the other
// control characters, DEL, the C1 controls (%X80 to %X9F), and %XFF as a
// period; the rest of DEC's multinational set kept.
func Pretty(text []byte) string {
	var b strings.Builder

	for _, c := range text {
		switch {
		case c == '\t':
			b.WriteByte(' ')

			for b.Len()%8 != 0 {
				b.WriteByte(' ')
			}
		case c == '\n':
			b.WriteString("<LF>")
		case c == '\v':
			b.WriteString("<VT>")
		case c == '\f':
			b.WriteString("<FF>")
		case c == '\r':
			b.WriteString("<CR>")
		case c < 0x20 || c == 0x7F || (c >= 0x80 && c < 0xA0) || c == 0xFF:
			b.WriteByte('.')
		default:
			b.WriteByte(c)
		}
	}

	return b.String()
}
