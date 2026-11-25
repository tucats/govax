package sumslp

// Reading an update file: its lines into a list of edits (the phase doc's
// A.4.2, A.5.3). Every edit keeps its command line as written, for the
// messages that name it, and its place in the files, for the merge's
// precedence.

// MaxRecord is the longest record SUMSLP reads, in an update file or the
// input: a longer one is RMS's RTB (A.5.5). The command reads the files and
// enforces it; Parse and Apply take any length.
const MaxRecord = 255

// Edit is one edit command and the lines that follow it.
type Edit struct {
	// File is the update file's index among Apply's updates (the first
	// listed lowest in precedence), and Seq the command's line in it (a
	// later line higher).
	File, Seq int

	// Command is the command's line as written ("" for the edit that holds
	// the lines before a file's first command).
	Command string

	// First is locator1, and Last locator2 for a replace or delete; Range
	// says there was a locator2. Both are 16-bit words, signed: a number is
	// read modulo 65,536 (A.5.3).
	First, Last int
	Range       bool

	// OutOfSequence is EDOUTSEQ: First isn't above the locator1 of the
	// file's command before it (or, for the file's first command, below 0).
	OutOfSequence bool

	// Text is the lines to insert.
	Text []Text
}

// Text is one line an edit inserts, or a message among them.
type Text struct {
	// Record is the line as it goes into the output, its "<" taken off.
	Record []byte

	// Plain says the audit trail was off (after a "\" line) when it was
	// read.
	Plain bool

	// Message, when set, makes the line a message: the listing gets the
	// message's lines in its place, and the output nothing. Numbered says
	// it still takes an inserted line's number, as a syntax error does
	// (Y21: the next line is .3).
	Message  *Message
	Numbered bool
}

// Message is one of SUMSLP's warnings as the listing shows it: its line,
// then a line for each edit it is about, the command as written and the
// update file's name.
type Message struct {
	ID    string // EDITSCLSH, EDOUTSEQ, PRMEOF, or SLPSYNERR
	About []About
}

// About is an edit a message names.
type About struct {
	Command, File string
}

// parsed is an update file read: its edits, and its audit trail.
type parsed struct {
	edits []*Edit

	// trail is the file's audit trail: the last one a command gave. Every
	// inserted line of the file shows it, even one read before the command
	// that gave it (U03; unconfirmed beyond that case).
	trail string
}

// parse reads update file u, the file-th of the run.
//
// A line is a command if it starts with "-", the end of the file if it
// starts with "/" (whatever follows, B39; " /" is text, B40), the audit
// trail off or on if it starts with "\" or "%" (the line's text dropped,
// B25, B26), and otherwise text, a leading "<" dropped (B21 to B24, B41).
// Text before the file's first command, and a command with a syntax error,
// belong to the edit before them, or to an insert before line 1 if there is
// none (O04, Y21). A file with no "/" is PRMEOF, its edits applied (B17); an
// empty file is no edits and no message (B19).
func parse(u Update, file int) *parsed {
	p := &parsed{}

	var (
		cur     *Edit // the edit text lines go to
		current int   // ".", the current line
		prev    *Edit // the last edit command, for EDOUTSEQ
		plain   bool
		ended   bool
	)

	// add gives a line to the edit before it, made (an insert before line
	// 1) if there is none yet.
	add := func(seq int, t Text) {
		if cur == nil {
			cur = &Edit{File: file, Seq: seq}
			p.edits = append(p.edits, cur)
		}

		cur.Text = append(cur.Text, t)
	}

	for seq, rec := range u.Records {
		if len(rec) == 0 {
			add(seq, Text{Record: rec, Plain: plain})

			continue
		}

		switch rec[0] {
		case '/':
			ended = true
		case '\\':
			plain = true
		case '%':
			plain = false
		case '-':
			c, ok := readCommand(rec[1:], current)
			if !ok {
				m := &Message{ID: "SLPSYNERR", About: []About{{Command: string(rec), File: u.Name}}}
				add(seq, Text{Message: m, Numbered: true})

				continue
			}

			if c.trail != nil {
				p.trail = *c.trail
			}

			e := &Edit{File: file, Seq: seq, Command: string(rec), First: c.first, Last: c.last, Range: c.ranged}

			if c.ranged {
				current = c.last
			} else {
				current = c.first
			}

			p.edits = append(p.edits, e)
			cur = e
		case '<':
			add(seq, Text{Record: rec[1:], Plain: plain})
		default:
			add(seq, Text{Record: rec, Plain: plain})
		}

		if ended {
			break
		}
	}

	if !ended && len(u.Records) > 0 {
		m := &Message{ID: "PRMEOF", About: []About{{File: u.Name}}}
		add(len(u.Records), Text{Message: m})
	}

	// An insert with nothing to insert is no edit: it moved the current
	// line, and may have given the trail (U03's "-,,/;U03 NEW/"), but it is
	// in no clash and out of no sequence.
	edits := p.edits[:0]

	for _, e := range p.edits {
		if e.Range || len(e.Text) > 0 {
			edits = append(edits, e)
		}
	}

	p.edits = edits

	for _, e := range p.edits {
		if e.Command == "" {
			continue
		}

		e.OutOfSequence = e.First < 0 || (prev != nil && e.First <= prev.First)
		prev = e
	}

	return p
}

// command is an edit command read.
type command struct {
	first, last int
	ranged      bool
	trail       *string
}

// readCommand reads an edit command's text after its "-":
//
//	locator1 , locator2 , /trail/ ;comment
//
// every part optional, blanks and tabs between them ignored (Y01, Y04, Y15,
// Y23), but a part's commas kept to reach a later one. A locator is digits
// (leading zeros allowed, Y02, N13) or ".", the current line; an empty
// locator1 is the current line too (B08's bare "-", Y22's "-,,/.../").
// current is "." before locator1; after it, "." is locator1 (B31's "-2,.").
// A sign, a letter, a third comma, a trail without its closing "/", two
// numbers with a blank between, or a locator2 below locator1 is an error
// (Y03, Y07 to Y11, Y17, Y19, Y20, O04, N12). A "-5," is an insert (B38),
// and "-5,5,;x" a replace with no trail (Y16).
func readCommand(b []byte, current int) (command, bool) {
	var c command

	i := 0

	skip := func() {
		for i < len(b) && (b[i] == ' ' || b[i] == '\t') {
			i++
		}
	}

	// done says the command ends here: at the line's end or a comment.
	done := func() bool {
		skip()

		return i == len(b) || b[i] == ';'
	}

	// locator reads a locator, def if it is empty.
	locator := func(def int) (int, bool) {
		skip()

		switch {
		case i < len(b) && b[i] == '.':
			i++

			return def, true
		case i < len(b) && b[i] >= '0' && b[i] <= '9':
			v := 0
			for i < len(b) && b[i] >= '0' && b[i] <= '9' {
				v = (v*10 + int(b[i]-'0')) & 0xFFFF
				i++
			}

			return int(int16(v)), true
		case i == len(b) || b[i] == ',' || b[i] == ';':
			return def, true
		}

		return 0, false
	}

	var ok bool

	if c.first, ok = locator(current); !ok {
		return c, false
	}

	if done() {
		return c, true
	}

	if b[i] != ',' {
		return c, false
	}

	i++
	skip()

	if i < len(b) && b[i] != ',' && b[i] != ';' {
		if c.last, ok = locator(c.first); !ok {
			return c, false
		}

		c.ranged = true

		if c.last < c.first {
			return c, false
		}
	}

	if done() {
		return c, true
	}

	if b[i] != ',' {
		return c, false
	}

	i++
	skip()

	if i < len(b) && b[i] == '/' {
		end := i + 1
		for end < len(b) && b[end] != '/' {
			end++
		}

		if end == len(b) {
			return c, false
		}

		trail := string(b[i+1 : end])
		c.trail = &trail
		i = end + 1
	}

	return c, done()
}
