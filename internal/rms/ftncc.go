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

// ratFTN is FAB$M_FTN.
var ratFTN = byte(vmsConst("FAB$M_FTN"))

// ftnTerminalText is what the terminal is sent for a record with FORTRAN
// carriage control, given whether the line is open, and whether the line
// is open after it.
func ftnTerminalText(record []byte, lineOpen bool) (text []byte, open bool) {
	control := byte(' ')
	if len(record) > 0 {
		control, record = record[0], record[1:]
	}

	var before string

	switch control {
	case '+', '$', 0:
	case '0':
		before = "\n"
	case '1':
		before = "\f"
	default:
		control = ' '
	}

	if lineOpen && control != '+' && control != 0 {
		before = "\n" + before
	}

	text = append([]byte(before), record...)

	switch control {
	case '$', 0:
		return text, true
	}

	return append(text, '\n'), false
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
