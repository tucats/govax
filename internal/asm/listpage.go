package asm

import (
	"fmt"
	"strings"
	"time"
)

// This file lays the recorded listing lines (listing.go) out as a MACRO
// listing's source pages, as real VAX MACRO lays them out
// (docs/PHASE-29.md, "What a real listing looks like"). A listing line is
// at most 132 columns (a long source line is kept whole, as real MACRO
// keeps it):
//
//	columns 0-35   the binary field: what the line stored, right-aligned,
//	               read right to left (lowest address rightmost)
//	column  36     a blank
//	columns 37-40  the location, in hex
//	columns 41-46  the line number
//	column  47     a blank
//	columns 48-    the source line as written, tabs kept
//
// Each page is 60 lines: a form feed and a two-line heading, a blank
// line, and 57 lines of the listing.

const (
	// listPageLines is the number of lines on a page, heading included,
	// and listHeadLines the heading's (its two lines and a blank one).
	listPageLines = 60
	listHeadLines = 3

	// binaryWidth is the binary field's width: columns 0 to 35, the last
	// of them the mark column of the field rightmost.
	binaryWidth = 36

	// opcodeSlot is the width an instruction's opcode takes at the right
	// of the binary field, with the blanks before it ("  D0 "). An
	// instruction's continuation lines leave it blank.
	opcodeSlot = 5

	// The heading's fields: the module name and the title (cut to fit),
	// the .IDENT string or other label (the second line's first field,
	// before the subtitle, which is cut as the title is), and the
	// assembler's name.
	headNameWidth      = 32
	headTitleWidth     = 40
	headLabelWidth     = 32
	headAssemblerWidth = 28
	headFileWidth      = 34
)

// ListingOptions are the facts a listing's headings show that the
// assembler doesn't know.
type ListingOptions struct {
	// Assembled is when the assembly ran.
	Assembled time.Time
	// Assembler names the assembler and its version, as the object's
	// language processor header does: "govax MACRO V1.2-34".
	Assembler string
	// Source is the source file's full file specification, and Revised
	// when the file was last revised.
	Source  string
	Revised time.Time
	// Command is the command line as typed, which the listing ends with:
	// "MACRO/LIST HELLO".
	Command string
	// Initialization and CommandProcessing are how long the caller took
	// to start up and to read the command, the performance indicators'
	// first two phases (see StartPhase).
	Initialization    PhaseTime
	CommandProcessing PhaseTime
}

// Listing returns the listing of the last Assemble, which SetListing
// asked for, one line per string. The first line of each page begins
// with a form feed, so joining the lines with newlines gives the listing
// file's text.
//
// The listing holds a table of contents, when the program has .SBTTL
// lines, then the program's source pages, and then the closing pages: the
// symbol table, the psect synopsis, and the statistics (listclose.go).
// Which lines the source pages show is the listing controls' choice
// (listctl.go).
func (a *Assembler) Listing(opts ListingOptions) []string {
	p := &listPager{opts: opts}
	p.name, p.title = a.Title()
	p.label = a.Ident()

	a.tableOfContents(p)

	for i, l := range a.listLines {
		// .PAGE starts a new page, unless this one is still empty.
		if l.page && !l.skipped && !l.collected && p.used > listHeadLines {
			p.breakPage()
		}

		// A .SBTTL's text heads the pages from here on, including the
		// one its own line starts (lctlnosh.lis's page 13).
		if l.hasSubtitle && !l.skipped && !l.collected {
			p.subtitle = l.subtitle
		}

		if listShown(l) {
			lines := a.listSourceLine(l)

			// A repeat block's .ENDR shows the bytes of its first
			// repetition's first line when the repetitions themselves
			// aren't listed (see repeatFirstLine).
			if first := a.repeatFirstLine(i); first != nil && !l.show.has(showExpansions|showBinary) {
				lines = a.listRepeatEnd(l, first)
			}

			for _, line := range lines {
				p.add(line)
			}

			// Real MACRO follows a .PRINT's line with an empty line (the
			// message itself goes to the terminal).
			for range l.messages {
				p.add("")
			}
		}
	}

	p.subtitle = ""
	a.closingPages(p, opts)

	return p.lines
}

// tableOfContents starts the listing with its table of contents, page 0,
// if the program has any .SBTTL lines: each one's file number, line
// number, and text, uncut. A .SBTTL in a macro expansion is listed at the
// line of the program that called the macro.
func (a *Assembler) tableOfContents(p *listPager) {
	var entries []string

	line := 0

	for _, l := range a.listLines {
		if l.depth == 0 {
			line = l.line
		}

		if l.hasSubtitle && !l.skipped && !l.collected {
			entries = append(entries, fmt.Sprintf("    (1)%9d        %s", line, l.subtitle))
		}
	}

	if len(entries) == 0 {
		return
	}

	p.page = -1
	p.contents = true

	for _, e := range entries {
		p.add(e)
	}

	p.contents = false
	p.breakPage()
}

// listPager collects a listing's lines into pages, starting each with
// its heading. label is what the heading's second line begins with: the
// .IDENT string on the source pages, and on the closing pages the name of
// the part a page starts in.
//
// subtitle is the .SBTTL text in force, which the second line shows after
// the label, and contents says the page is the table of contents, whose
// second line is just "Table of contents".
type listPager struct {
	opts               ListingOptions
	name, title, label string
	subtitle           string
	contents           bool
	page               int
	started            bool // a page has been started
	used               int  // lines on the current page, heading included
	lines              []string
}

// add appends line to the listing, starting a new page first if this
// one is full (or none has been started).
func (p *listPager) add(line string) {
	if !p.started || p.used == listPageLines {
		p.newPage()
	}

	p.lines = append(p.lines, line)
	p.used++
}

// breakPage ends the current page: the next line starts a new one.
func (p *listPager) breakPage() {
	p.used = listPageLines
}

// newPage starts the next page with its heading. The first line names
// the module, its .TITLE text, when it was assembled, by what, and the
// page; the second gives its label (the .IDENT string, on a source
// page), the source file's revision date, its file specification, and
// its file number.
func (p *listPager) newPage() {
	p.page++
	p.started = true

	title := p.title
	if len(title) > headTitleWidth {
		title = title[:headTitleWidth]
	}

	first := fmt.Sprintf("\f%-*s%-*s %s  %-*sPage%4d",
		headNameWidth, p.name, headTitleWidth, title, vmsDateTime(p.opts.Assembled),
		headAssemblerWidth, p.opts.Assembler, p.page)

	file := fmt.Sprintf("%-*s", headFileWidth, p.opts.Source)
	if len(p.opts.Source) >= headFileWidth {
		file += " "
	}

	subtitle := p.subtitle
	if len(subtitle) > headTitleWidth {
		subtitle = subtitle[:headTitleWidth]
	}

	second := fmt.Sprintf("%-*s%-*s %s  %s(1)", headLabelWidth, p.label, headTitleWidth, subtitle,
		vmsDateTime(p.opts.Revised), file)
	if p.contents {
		second = "Table of contents"
	}

	p.lines = append(p.lines, first, second, "")
	p.used = listHeadLines
}

// vmsDateTime formats t as VMS does in a listing's heading:
// " 2-OCT-2026 19:02:58".
func vmsDateTime(t time.Time) string {
	return fmt.Sprintf("%2d-%s-%04d %02d:%02d:%02d",
		t.Day(), strings.ToUpper(t.Month().String()[:3]), t.Year(), t.Hour(), t.Minute(), t.Second())
}

// listSourceLine returns l as the listing shows it: its line, then any
// continuation lines its binary field needs, each with the location of
// the bytes it shows and no line number.
//
// A line of a macro expansion or repeat block has no line number.
func (a *Assembler) listSourceLine(l *listLine) []string {
	tail := fmt.Sprintf("%6d %s", l.line, l.text)
	if l.depth > 0 {
		tail = strings.Repeat(" ", 7) + l.text
	}

	switch {
	// A .PSECT or .RESTORE_PSECT line (to a relocatable psect) shows the
	// psect's location where the binary field ends, over the location
	// column.
	case (l.op == ".PSECT" || l.op == ".RESTORE_PSECT") && l.endSect != nil && l.endSect.relocatable:
		return []string{fmt.Sprintf("%*s%08X%s", binaryWidth-3, "", l.endLoc, tail)}

	// A direct assignment shows the value it assigned (.MDELETE how many
	// macros it deleted, and an .IF the value it tested), and a .BLKx the
	// location it leaves, as a longword (with no mark, relocatable or
	// not).
	case (l.op == "=" || l.op == ".MDELETE" || l.op == ".IF") && l.hasValue:
		return []string{listColumns(fmt.Sprintf("%08X ", l.value), l.loc) + tail}

	case strings.HasPrefix(l.op, ".BLK"):
		return []string{listColumns(fmt.Sprintf("%08X ", l.endLoc), l.loc) + tail}

	case l.collected || l.skipped:
		return []string{listColumns("", l.loc) + tail}
	}

	rows := a.binaryRows(l)
	if len(rows) == 0 {
		return []string{listColumns("", l.loc) + tail}
	}

	out := []string{listColumns(rows[0].text, l.loc) + tail}

	for _, r := range rows[1:] {
		out = append(out, listColumns(r.text, r.offset)+strings.Repeat(" ", 7))
	}

	return out
}

// repeatFirstLine returns the line that the recorded line i shows the
// bytes of, or nil. That's the line i ended a repeat block (.ENDR), which
// MACRO then repeated: real MACRO's listing shows the bytes of the first
// line of the block's first repetition on the .ENDR line, as though that
// line were its own (usermac.lis: a .REPEAT 3 of .BYTE ^X11 shows 11).
// The repetition's lines are recorded right after the line that ended the
// block, one level down.
func (a *Assembler) repeatFirstLine(i int) *listLine {
	l := a.listLines[i]
	if !l.collected || i+1 == len(a.listLines) {
		return nil
	}

	next := a.listLines[i+1]
	if next.kind != sourceRepeat || next.depth != l.depth+1 {
		return nil
	}

	return next
}

// listRepeatEnd returns end, the line that ended a repeat block, as the
// listing shows it: with the bytes of first, the first line of the
// block's first repetition (see repeatFirstLine).
func (a *Assembler) listRepeatEnd(end, first *listLine) []string {
	shown := *first
	shown.text = end.text
	shown.line = end.line
	shown.depth = end.depth

	return a.listSourceLine(&shown)
}

// listColumns returns a listing line's binary field (right-aligned) and
// location columns, through column 40.
func listColumns(binary string, loc uint32) string {
	return fmt.Sprintf("%*s %04X", binaryWidth, binary, loc)
}

// binaryRow is one listing line's part of a binary field: its text,
// without the alignment, and the location of the lowest address it
// shows.
type binaryRow struct {
	text   string
	offset uint32
}

// binaryUnit is a piece of a binary field that's kept on one line: a
// data field, or an instruction's operand specifier. sep is what goes
// between it and the unit to its right (the one before it in memory)
// when they share a line.
type binaryUnit struct {
	text   string
	offset uint32
	sep    string
	// alone says the unit starts a line of its own, as .ASCIC's count,
	// stored back over the first byte, does.
	alone bool
}

// binaryRows lays l's fields out as its binary field's lines. Each field
// is its value in hex (most significant digit first), then its mark
// column: ' for a field the linker finishes, or a blank. An
// instruction's operand specifiers are each kept together and set three
// columns apart (its mark column and two blanks), the opcode rightmost;
// data fields are set one column apart (the mark column). As many fit on
// a line as its 36 columns hold, lowest address first; the rest go on
// continuation lines, which for an instruction leave the opcode's
// columns blank.
func (a *Assembler) binaryRows(l *listLine) []binaryRow {
	fields := a.listFields(l)
	if len(fields) == 0 {
		return nil
	}

	units := binaryUnits(fields, l.instruction)

	width := binaryWidth
	pad := ""

	var (
		rows []binaryRow
		cur  *binaryRow
	)

	for _, u := range units {
		if cur != nil && !u.alone && len(cur.text)+len(u.sep)+len(u.text) <= width {
			cur.text = u.text + u.sep + cur.text

			continue
		}

		if cur != nil {
			cur.text += pad
			rows = append(rows, *cur)

			// An instruction's continuation lines keep its opcode's
			// columns empty.
			if l.instruction && pad == "" {
				width -= opcodeSlot
				pad = strings.Repeat(" ", opcodeSlot)
			}
		}

		cur = &binaryRow{text: u.text, offset: u.offset}
	}

	if len(rows) > 0 {
		cur.text += pad
	}

	return append(rows, *cur)
}

// binaryUnits turns fields, in address order, into the units of a binary
// field. An instruction's fields go by group (the opcode, then each
// operand specifier); a group too wide for a line is split into its
// fields.
func binaryUnits(fields []listBytes, instruction bool) []binaryUnit {
	var units []binaryUnit

	if !instruction {
		for _, f := range fields {
			for _, piece := range fieldPieces(f) {
				units = append(units, binaryUnit{text: piece.text, offset: piece.offset, alone: f.patch})
			}
		}

		return units
	}

	for i := 0; i < len(fields); {
		j := i + 1
		for j < len(fields) && fields[j].group == fields[i].group && !fields[j].patch {
			j++
		}

		group := fields[i:j]

		// The group's text, highest address first. A field joined to
		// the one after it (an index prefix) follows it with no blank.
		var text string

		for k := len(group) - 1; k >= 0; k-- {
			piece := fieldText(group[k])
			if group[k].join {
				text = strings.TrimSuffix(text, " ")
			}

			text += piece
		}

		if len(text) <= binaryWidth-opcodeSlot {
			units = append(units, binaryUnit{text: text, offset: group[0].offset, sep: "  ", alone: group[0].patch})
		} else {
			for k, f := range group {
				for n, piece := range fieldPieces(f) {
					sep := ""
					if k == 0 && n == 0 {
						sep = "  "
					}

					units = append(units, binaryUnit{text: piece.text, offset: piece.offset, sep: sep})
				}
			}
		}

		i = j
	}

	return units
}

// fieldPiece is part of a field as the binary field shows it.
type fieldPiece struct {
	text   string
	offset uint32
}

// fieldPieces returns f as the binary field shows it: a byte, word, or
// longword as one number; a quadword or octaword (an unmarked field of
// several longwords) as its longwords, each its own number; and a field of
// any other size as its bytes.
func fieldPieces(f listBytes) []fieldPiece {
	n := uint32(len(f.data))

	var size uint32

	switch {
	case n <= 2 || n == 4:
		return []fieldPiece{{fieldText(f), f.offset}}
	case n%4 == 0 && f.mark == markNone:
		size = 4
	default:
		size = 1
	}

	var out []fieldPiece

	for p := uint32(0); p < n; p += size {
		part := f
		part.offset = f.offset + p
		part.data = f.data[p : p+size]
		out = append(out, fieldPiece{fieldText(part), part.offset})
	}

	return out
}

// fieldText returns f's value in hex, most significant digit first, and
// its mark column: ' for a field the linker finishes, else a blank. A
// general mode operand's mode byte shows G for its high digit.
func fieldText(f listBytes) string {
	var b strings.Builder

	for k := len(f.data) - 1; k >= 0; k-- {
		fmt.Fprintf(&b, "%02X", f.data[k])
	}

	s := b.String()

	switch f.mark {
	case markReloc:
		return s + "'"
	case markGeneral:
		return "G" + s[1:] + " "
	}

	return s + " "
}
