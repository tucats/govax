package rms

// FORTRAN carriage control at the terminal (Phase 72).
//
// A record of a file whose record attributes are FAB$M_FTN starts with a
// carriage control character, which RMS turns into what the terminal is
// sent before and after the rest of the record (the RMS Reference
// Manual, FAB$B_RAT; the Guide to OpenVMS File Applications):
//
//	' '  a line feed before, a carriage return after: a line
//	'0'  two line feeds before (a blank line first), a carriage return after
//	'1'  a form feed before, a carriage return after
//	'+'  nothing before (overprinting), a carriage return after
//	'$'  a line feed before, nothing after: a prompt, the cursor left
//	     at its end for the reply
//	NUL  nothing before or after
//
// and any other character as a blank. A FORTRAN program writes its
// terminal units this way: DUNGEON's prompt is "$>", and its score line a
// "$" record and a "+" record, which VMS shows as one line.
//
// govax's terminal is written a line at a time, each record its text and
// a newline. So here a record ends its line unless it is "$" or NUL, and
// an open line (one a "$" record left) is ended before a record that
// starts its own, which every control but "+" and NUL does; a "+"
// record continues it. A terminal read ends it too: the reply typed
// ends its line. A "+" record after a line that ended can't overprint
// it, so it is a line of its own. Which streams take FTN: an indirect
// accessor's whose $OPEN or $CREATE asked for it (FAB$V_PPF_RAT, as
// FORRTL's units at the terminal do); a terminal opened directly is
// written as before (unconfirmed: its record attributes aren't kept).
//
// The same translation serves the terminal driver's $QIO writes, whose
// carriage control (p4) is either a FORTRAN character or a prefix and a
// postfix byte coded as print-file control is (PrintTerminalText).

// ratFTN is FAB$M_FTN.
var ratFTN = byte(vmsConst("FAB$M_FTN"))

// ftnTerminalText is what the terminal is sent for a record with FORTRAN
// carriage control, given whether the line is open, and whether the line
// is open after it: its control character's print-file control
// (ftnPrintControl), as PrintTerminalText sends it.
func ftnTerminalText(record []byte, lineOpen bool) (text []byte, open bool) {
	control := byte(' ')
	if len(record) > 0 {
		control, record = record[0], record[1:]
	}

	prefix, postfix := FortranPrintControl(control)

	return PrintTerminalText(record, prefix, postfix, lineOpen)
}

// Control characters print-file control names.
const (
	asciiLineFeed       = 0x0A
	asciiVerticalTab    = 0x0B
	asciiFormFeed       = 0x0C
	asciiCarriageReturn = 0x0D
)

// PrintTerminalText is what the terminal is sent for data written with
// print-file carriage control, given whether the line is open, and
// whether the line is open after it. prefix says what comes before the
// data and postfix what comes after, each a byte coded as FAB$B_RAT
// PRN's fixed control and a terminal $QIO write's p4 code them (the RMS
// Reference Manual; the I/O User's Reference Manual):
//
//	0            nothing
//	1 to 127     that many line feeds
//	100x xxxx    the control character xxxxx (^X8D a carriage return)
//	110x xxxx    the C1 control character 1000 xxxx (^X80 to ^X9F)
//	anything else nothing
//
// VMS's terminal output is a line feed before a line and a carriage
// return after it, so its cursor rests at the start of the line just
// written; govax's is the line and then a newline, so the host's cursor
// rests at the start of the next line. The two agree when govax counts
// one of a prefix's line feeds as the newline already written, and
// writes a newline for a postfix's carriage return, which is the
// translation here:
//
//   - n line feeds before: n-1 newlines, after a newline that ends an
//     open line;
//   - a form feed or vertical tab before: the character, after a newline
//     that ends an open line;
//   - a carriage return after: a newline (the line is closed); n line
//     feeds after: n+1 newlines, the n-th line below the data's being
//     where VMS's next line feed would put its next line;
//   - nothing after, or another control character: the line is left
//     open, so the next line starts with a newline.
//
// Other control characters (a bell, a carriage return before, C1 ones)
// are sent as they are. Nothing before a closed line can't overprint the
// line before it, so the data starts a line of its own.
func PrintTerminalText(data []byte, prefix, postfix byte, lineOpen bool) (text []byte, open bool) {
	endOpenLine := func() {
		if lineOpen {
			text = append(text, '\n')
			lineOpen = false
		}
	}

	switch c, lineFeeds := printControlCharacter(prefix); {
	case lineFeeds > 0:
		endOpenLine()

		for range lineFeeds - 1 {
			text = append(text, '\n')
		}

	case c == asciiFormFeed || c == asciiVerticalTab:
		endOpenLine()

		text = append(text, c)

	case c != 0:
		text = append(text, c)
	}

	text = append(text, data...)

	switch c, lineFeeds := printControlCharacter(postfix); {
	case lineFeeds > 0:
		for range lineFeeds + 1 {
			text = append(text, '\n')
		}

		return text, false

	case c == asciiCarriageReturn:
		return append(text, '\n'), false

	case c != 0:
		text = append(text, c)
	}

	return text, true
}

// printControlCharacter decodes one print-file control byte
// (PrintTerminalText): the number of line feeds it stands for, or else
// the one character it stands for (0 for none). A line feed coded as a
// control character is one line feed.
func printControlCharacter(b byte) (c byte, lineFeeds int) {
	switch {
	case b&0x80 == 0:
		return 0, int(b)
	case b&0xE0 == 0x80:
		if c = b & 0x1F; c == asciiLineFeed {
			return 0, 1
		}

		return c, 0
	case b&0xE0 == 0xC0:
		return 0x80 | b&0x1F, 0
	}

	return 0, 0
}

// FortranPrintControl returns the print-file control (ftnPrintControl)
// that stands for a FORTRAN carriage control character: its prefix and
// postfix bytes.
func FortranPrintControl(control byte) (prefix, postfix byte) {
	pc := ftnPrintControl(control)

	return pc[0], pc[1]
}

// ftnPrintControl is the two bytes of print-file fixed control (a
// prefix, what comes before the record, and a postfix, what comes
// after: the RMS Reference Manual's FAB$B_RAT PRN) that stand for a
// FORTRAN carriage control character, for an FTN accessor's record
// written to a print file, as a command procedure's /OUTPUT file is
// (ppfaccess.go's indirectRecord). A prefix byte 0 to 127 is that many
// new lines and ^X8C a form feed (bit 7 set: bits 0 to 4 a control
// character); a postfix of ^X8D is a carriage return, 0 nothing:
//
//	' '  01 8D, a line, as DCL's own records are
//	'0'  02 8D, a blank line first
//	'1'  8C 8D, a new page
//	'+'  00 8D, overprinting
//	'$'  01 00, a prompt: no carriage return after it
//	NUL  00 00
//
// and any other character as a blank. Unconfirmed: what VMS's RMS
// writes for an FTN accessor's records to a print file.
func ftnPrintControl(control byte) []byte {
	switch control {
	case '0':
		return []byte{0x02, 0x8D}
	case '1':
		return []byte{0x8C, 0x8D}
	case '+':
		return []byte{0x00, 0x8D}
	case '$':
		return []byte{0x01, 0x00}
	case 0:
		return []byte{0x00, 0x00}
	}

	return []byte{0x01, 0x8D}
}

// terminalText is what SYS$PUT sends the terminal for record on a stream
// whose ISI is isi: the record and a newline, or for an FTN accessor
// (ftnTerminalText) its carriage control's, the process's open line
// (FileTable.lineOpen) kept up to date. An open line left by an FTN
// prompt is ended before any other record.
func (ctx *Context) terminalText(isi uint16, record []byte) []byte {
	open := ctx.Files != nil && ctx.Files.lineOpen

	if isIndirect(isi) && wordRAT(isi)&ratFTN != 0 {
		text, stillOpen := ftnTerminalText(record, open)
		ctx.Files.lineOpen = stillOpen

		return text
	}

	text := append(append([]byte{}, record...), '\n')
	if open {
		text = append([]byte{'\n'}, text...)
		ctx.Files.lineOpen = false
	}

	return text
}

// terminalRead notes that the terminal was read: the reply typed ended
// any open line.
func (ctx *Context) terminalRead() {
	if ctx.Files != nil {
		ctx.Files.lineOpen = false
	}
}

// LineOpen reports whether the terminal's last line, as this process
// wrote it, is open: written with no newline after it, so the next line
// must start with one.
func (t *FileTable) LineOpen() bool { return t.lineOpen }

// SetLineOpen records whether the terminal's last line is open, for
// output that reaches the terminal other than through RMS: the terminal
// driver's $QIO writes, and its reads, whose reply ends the line.
func (t *FileTable) SetLineOpen(open bool) { t.lineOpen = open }
