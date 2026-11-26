package sumslp

import (
	"cmp"
	"encoding/binary"
	"slices"
	"strconv"

	"github.com/tucats/govax/internal/vmsdef"
)

// Update is one update file: its name, as SUMSLP's messages show it (a
// file's full name, or "SYS$INPUT:.;"), and its records, each a line of the
// file.
type Update struct {
	Name    string
	Records [][]byte
}

// Line is one line of the listing.
type Line struct {
	// Field is the 16 columns before the line's number: the audit trail of
	// an inserted line, or "-n" on the first line written after n deleted
	// ones; "" for none.
	Field string

	// Number is the line's number: the input's, or ".n" for the nth line of
	// a run of inserted ones. "" makes the line a message line, written as
	// Text alone.
	Number string

	// Text is the record (or the message line).
	Text []byte
}

// Result is what an update makes: the output file's records and the
// listing's lines.
type Result struct {
	Output [][]byte
	Lines  []Line

	// Heads are each output record's facts for /HEADER, Output's in order.
	Heads []Head
}

// Head is what /HEADER puts in an output record's 12-byte fixed control
// (the probe's U11 to U14, read from the VAX's own files): the input line's
// number, 0 for an inserted line; an inserted line's number in its run, or
// for an input line the deleted lines its "-n" shows (0 for none); and the
// audit trail the listing shows, cut or zero-filled to 8 bytes.
type Head struct {
	Line, Count int
	Trail       string
}

// HeaderSize is the fixed control /HEADER gives each record.
const HeaderSize = 12

// Headed is the output's records with their /HEADER fixed control before
// each: two little-endian words, then the trail's 8 bytes.
func (r *Result) Headed() [][]byte {
	out := make([][]byte, len(r.Output))

	for i, rec := range r.Output {
		h := r.Heads[i]
		b := make([]byte, HeaderSize, HeaderSize+len(rec))
		binary.LittleEndian.PutUint16(b[0:], uint16(h.Line))
		binary.LittleEndian.PutUint16(b[2:], uint16(h.Count))
		copy(b[4:HeaderSize], h.Trail)
		out[i] = append(b, rec...)
	}

	return out
}

// Messages are the listing's message lines alone, which EDIT/SUM writes to
// SYS$OUTPUT when it makes no listing (the probe's terminal session).
func (r *Result) Messages() [][]byte {
	var out [][]byte

	for _, l := range r.Lines {
		if l.Number == "" {
			out = append(out, l.Text)
		}
	}

	return out
}

// Apply updates the input's records with the update files, the first listed
// lowest in precedence: every file's edits sorted into one list and applied
// in one pass over the input (doc.go has the rules).
func Apply(input [][]byte, updates []Update) *Result {
	a := &applier{input: input, names: make([]string, len(updates))}

	for i, u := range updates {
		p := parse(u, i)
		a.edits = append(a.edits, p.edits...)
		a.trails = append(a.trails, p.trail)
		a.names[i] = u.Name
	}

	a.sort()
	a.run()

	return &a.r
}

// applier is one run of Apply.
type applier struct {
	input  [][]byte
	edits  []*Edit
	trails []string // each update file's audit trail
	names  []string // each update file's name
	r      Result

	// The pass's place: next is the input's next record (its index), and
	// passed the number of the last line read, written or deleted; the
	// numbers go on when the input is read again (A.5.4).
	next, passed int
	reread       bool // the input has been read again
	ended        bool // the input's end ended the run

	// deleted is the lines deleted since an input line was last written,
	// and marked says a range has ended normally since, so the next input
	// line written shows "-n" (A.5.4: a range that runs out marks nothing).
	deleted int
	marked  bool

	// inserted is the number of the last inserted line of the current run
	// (".n"), which an input line written ends.
	inserted int

	// open is the range being replaced: its lines are deleted as it opens,
	// its text and the inserts held while it is open written as it closes.
	open *openRange

	// groups is the clash message to list before each edit that starts a
	// group of edits that meet.
	groups map[*Edit]*Message
}

// openRange is a replace or delete whose lines are gone and whose text
// hasn't been written.
type openRange struct {
	edit *Edit
	held []*Edit // inserts after lines it deleted (A.5.2)

	// dropped says a range starting inside it was dropped, which discards
	// every insert it holds, those sorted after the dropped range too
	// (A.5.2's KACB to KCAB, G14R).
	dropped bool
}

// sort puts the edits in the pass's order: by locator1, the higher
// precedence first among equals (a later file, then a later line), and
// finds the groups that clash.
func (a *applier) sort() {
	slices.SortStableFunc(a.edits, func(x, y *Edit) int {
		if c := cmp.Compare(x.First, y.First); c != 0 {
			return c
		}

		if c := cmp.Compare(y.File, x.File); c != 0 {
			return c
		}

		return cmp.Compare(y.Seq, x.Seq)
	})

	a.groups = map[*Edit]*Message{}

	for i := 0; i < len(a.edits); {
		first := a.edits[i]
		_, hi := extent(first)

		j := i + 1
		for ; j < len(a.edits); j++ {
			lo, h := extent(a.edits[j])
			if lo > hi {
				break
			}

			hi = max(hi, h)
		}

		if j-i > 1 {
			m := &Message{ID: "EDITSCLSH"}
			for _, e := range a.edits[i:j] {
				m.About = append(m.About, a.about(e))
			}

			a.groups[first] = m
		}

		i = j
	}
}

// extent is the input lines an edit is about, for the clashes: an insert's
// line (an insert after n meets edits of n), a range's lines.
func extent(e *Edit) (int, int) {
	if e.Range && e.First > 0 {
		return e.First, e.Last
	}

	p := insertAfter(e)

	return p, p
}

// insertAfter is the line an insert goes after: its locator1, but 1 for a
// negative one (N01, Y05), and 0 for a range at 0 or below, which deletes
// nothing (Y24, N10).
func insertAfter(e *Edit) int {
	switch {
	case e.Range && e.First <= 0:
		return 0
	case e.First < 0:
		return 1
	}

	return e.First
}

// about is an edit as a message names it.
func (a *applier) about(e *Edit) About {
	return About{Command: e.Command, File: a.names[e.File]}
}

// run is the pass.
func (a *applier) run() {
	for _, e := range a.edits {
		if e.Range && e.First > 0 {
			a.replace(e)
		} else {
			a.insert(e)
		}
	}

	if a.open != nil {
		a.close(true)
	}

	if !a.ended {
		a.advance(int(^uint(0) >> 1))
	}
}

// insert applies an insert: its text after its line, at the current place if
// the pass is past it (O05, O08), held if a range open deleted that line.
func (a *applier) insert(e *Edit) {
	p := insertAfter(e)

	if a.open != nil {
		if p <= a.open.edit.Last {
			a.sequence(e)
			a.open.held = append(a.open.held, e)

			return
		}

		a.close(true)
	}

	a.clash(e, p-1)

	if !a.advance(p) {
		return // after a line past the input's end: lost (Y12, E08)
	}

	a.sequence(e)
	a.write(e)
}

// replace applies a replace or delete: its lines deleted at once, its text
// written when the range closes. A range whose first line the pass has
// already read is dropped, and so are the inserts the range open holds
// (O02, G14, G14R, A.5.2).
func (a *applier) replace(e *Edit) {
	if a.open != nil {
		if e.First <= a.open.edit.Last {
			a.sequence(e)
			a.open.dropped = true

			return
		}

		a.close(true)
	}

	if e.First <= a.passed {
		a.sequence(e)

		return
	}

	a.clash(e, e.First-1)

	if !a.advance(e.First - 1) {
		return // past the input's end (unconfirmed: no case had one)
	}

	a.sequence(e)
	a.open = &openRange{edit: e}

	for a.passed < e.Last {
		if a.next == len(a.input) {
			a.runOut()

			return
		}

		a.next++
		a.passed++
		a.deleted++
	}
}

// runOut is the input's end while a range is open: the range closes there,
// with no "-n" for its lines, and the input is read again from its start,
// numbered on, once (A.5.4).
func (a *applier) runOut() {
	a.close(false)

	if a.reread {
		a.ended = true

		return
	}

	a.reread = true
	a.next = 0
}

// close writes the open range's text, then the inserts it held. normal says
// it reached its last line, so the next input line shows its "-n".
func (a *applier) close(normal bool) {
	r := a.open
	a.open = nil

	a.write(r.edit)

	if !r.dropped {
		for _, e := range r.held {
			a.write(e)
		}
	}

	if normal {
		a.marked = true
	}
}

// clash lists the clash message of the group e starts, when the pass reaches
// the line before its first: after line before.
func (a *applier) clash(e *Edit, before int) {
	m := a.groups[e]
	if m == nil {
		return
	}

	a.advance(before)
	a.message(m)
}

// sequence lists EDOUTSEQ for e if it is out of sequence, just before it is
// applied (O01, O03, O06 to O10).
func (a *applier) sequence(e *Edit) {
	if e.OutOfSequence {
		a.message(&Message{ID: "EDOUTSEQ", About: []About{a.about(e)}})
	}
}

// advance writes input lines until line n is passed, and says whether it
// got there: the input's end, with no range open, ends the run (E11, E15).
func (a *applier) advance(n int) bool {
	for a.passed < n {
		if a.ended || a.next == len(a.input) {
			a.ended = true

			return false
		}

		rec := a.input[a.next]
		a.next++
		a.passed++

		l := Line{Number: strconv.Itoa(a.passed), Text: rec}
		h := Head{Line: a.passed}

		if a.marked {
			l.Field = "-" + strconv.Itoa(a.deleted)
			h.Count = a.deleted
			a.deleted, a.marked = 0, false
		}

		a.r.Lines = append(a.r.Lines, l)
		a.r.Output = append(a.r.Output, rec)
		a.r.Heads = append(a.r.Heads, h)
		a.inserted = 0
	}

	return true
}

// write writes an edit's text: each line numbered in the run of inserted
// lines, with its file's audit trail unless it was off; a message in its
// place listed (a syntax error's taking its number, Y21).
func (a *applier) write(e *Edit) {
	for _, t := range e.Text {
		if t.Message != nil {
			if t.Numbered {
				a.inserted++
			}

			a.message(t.Message)

			continue
		}

		a.inserted++

		l := Line{Number: "." + strconv.Itoa(a.inserted), Text: t.Record}
		if !t.Plain {
			l.Field = a.trails[e.File]
		}

		a.r.Lines = append(a.r.Lines, l)
		a.r.Output = append(a.r.Output, t.Record)
		a.r.Heads = append(a.r.Heads, Head{Count: a.inserted, Trail: l.Field})
	}
}

// message lists a message: its line, then a line per edit it names, the
// command as written, four blanks, and the update file's name.
func (a *applier) message(m *Message) {
	a.r.Lines = append(a.r.Lines, Line{Text: []byte(MessageLine(m.ID))})

	for _, ab := range m.About {
		a.r.Lines = append(a.r.Lines, Line{Text: []byte(ab.Command + "    " + ab.File)})
	}
}

// MessageLine is a warning's first line: "%SUM-W-EDITSCLSH, edits clash".
func MessageLine(id string) string {
	return "%SUM-W-" + id + ", " + vmsdef.SUMMessages[id]
}
