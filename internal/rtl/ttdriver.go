package rtl

import (
	"bufio"
	"io"

	"github.com/tucats/govax/internal/vm"
)

// The terminal driver (docs/PHASE-26.md subtask 17): the $QIO functions
// a terminal (device class DC$_TERM, such as TTA0:) supports. Every
// terminal is the console: reads come from the console's input stream
// (the same one DECC$GETS reads, input.go) and writes go to its output
// stream.
//
// # Reading
//
// A terminal read (IO$_READVBLK and its variants) collects typed
// characters into the caller's buffer until one of these ends it:
//
//   - A *terminator* is typed. By default that's any control character
//     (codes 0-31) except backspace, tab, line feed, vertical tab, and
//     form feed — so, in practice, RETURN (carriage return, 13) or
//     CTRL/Z (26). p4 can supply a different set. The terminator isn't
//     part of the data: the transfer count excludes it, and the IOSB
//     reports it separately.
//   - The buffer is full (p2 characters). Then there is no terminator.
//   - With IO$M_TIMED, the time limit passes (see below).
//
// The IOSB of a completed read holds:
//
//	word 0   condition value (SS$_NORMAL, SS$_TIMEOUT, SS$_ENDOFFILE)
//	word 1   the transfer count: how many characters are data, which is
//	         also the terminator's offset in the buffer
//	word 2   the terminator character (0 if none)
//	word 3   the terminator's size (1, or 0 if none)
//
// The host delivers input a line at a time, ending in a newline where
// the terminal would send RETURN. So a host newline is read as a
// carriage return (and a host "\r\n" pair as one carriage return), which
// terminates a read with the default terminators just as RETURN would.
// When the buffer has room, the terminator is also stored after the data.
//
// # Writing
//
// A terminal write (IO$_WRITEVBLK and its variants) sends p2 bytes from
// p1 to the terminal, with optional carriage control from p4 (see
// carriageControl). The IOSB's transfer count is the number of bytes.
//
// # Characteristics
//
// IO$_SENSEMODE returns the terminal's characteristics in the 8- or
// 12-byte buffer at p1, and IO$_SETMODE changes them:
//
//	byte 0    device class (DC$_TERM, 66)
//	byte 1    terminal type (TT$_VT100, ...)
//	word 2    page width
//	bytes 4-6 terminal characteristics (TT$M_ bits)
//	byte 7    page length
//	long 8    extended characteristics (TT2$M_ bits), 12-byte form only
//
// They're kept on the device record (iodev.Device): the type in DevType,
// the page width in DevBufSize, bytes 4-7 in DevDepend, and the extended
// characteristics in DevDepend2 — the fields DEFINE/DEVICE sets and SHOW
// DEVICE shows, which are where VMS keeps them too (UCB$B_DEVTYPE,
// UCB$W_DEVBUFSIZ, UCB$L_DEVDEPEND, UCB$L_DEVDEPND2).
//
// # What the host terminal can't do
//
// govax doesn't own the host's terminal: it reads whatever the host
// delivers. So some modifiers are accepted but can't change anything
// (docs/DEVIATIONS.md):
//
//   - IO$M_NOECHO and IO$M_TRMNOECHO: the host has already echoed the
//     line (or, with redirected input, nothing is echoed at all).
//   - IO$M_TIMED: a time limit of 0 seconds reads only characters
//     already buffered (the common "is anything typed ahead?" poll),
//     ending with SS$_TIMEOUT if they run out before a terminator. A
//     longer limit waits for input with no limit at all.
//   - IO$M_NOFILTR, IO$M_REFRESH, IO$M_ESCAPE, IO$M_DSABLMBX, and on
//     writes IO$M_CANCTRLO, IO$M_NOFORMAT, IO$M_BREAKTHRU, and
//     IO$M_ENABLMBX: no line editing, escape sequences, mailboxes,
//     CTRL/O, or output formatting exist to change.
//   - IO$_SETMODE with IO$M_CTRLCAST or IO$M_CTRLYAST enables a CTRL/C or
//     CTRL/Y AST (ctrlast.go). Other modifiers (IO$M_OUTBAND,
//     IO$M_HANGUP, ...) succeed without doing anything.
//
// The end of the host's input is reported as SS$_ENDOFFILE, with any
// characters read before it as data.

// Terminal function codes and modifiers, from $IODEF.
var (
	ioModCtrlCAST  = ioCode("IO$M_CTRLCAST")
	ioModCtrlYAST  = ioCode("IO$M_CTRLYAST")
	ioModTimed     = ioCode("IO$M_TIMED")
	ioModCvtLow    = ioCode("IO$M_CVTLOW")
	ioModPurge     = ioCode("IO$M_PURGE")
	ioModTypeahead = ioCode("IO$M_TYPEAHDCNT")
)

// terminalFunctions is the terminal driver's function table: the read
// variants (virtual, logical, physical block, and the "read all"
// pass-through forms, which differ only in ways the console can't
// express), the prompted read, the write variants, and sense/set mode
// with their older "characteristics" names.
var terminalFunctions = map[uint32]ioFunc{
	ioCode("IO$_READVBLK"):    ttRead,
	ioCode("IO$_READLBLK"):    ttRead,
	ioCode("IO$_READPBLK"):    ttRead,
	ioCode("IO$_TTYREADALL"):  ttRead,
	ioCode("IO$_TTYREADPALL"): ttReadPrompt,
	ioCode("IO$_READPROMPT"):  ttReadPrompt,
	ioCode("IO$_WRITEVBLK"):   ttWrite,
	ioCode("IO$_WRITELBLK"):   ttWrite,
	ioCode("IO$_WRITEPBLK"):   ttWrite,
	ioCode("IO$_SENSEMODE"):   ttSenseMode,
	ioCode("IO$_SENSECHAR"):   ttSenseMode,
	ioCode("IO$_SETMODE"):     ttSetMode,
	ioCode("IO$_SETCHAR"):     ttSetMode,
}

// Control characters the terminal driver treats specially.
const (
	ttBackspace      = 0x08
	ttTab            = 0x09
	ttLineFeed       = 0x0A
	ttVerticalTab    = 0x0B
	ttFormFeed       = 0x0C
	ttCarriageReturn = 0x0D
)

// defaultTerminators is the standard terminator set as a mask of control
// characters (bit n for character n): every one except backspace, tab,
// line feed, vertical tab, and form feed.
const defaultTerminators = ^uint32(1<<ttBackspace | 1<<ttTab | 1<<ttLineFeed | 1<<ttVerticalTab | 1<<ttFormFeed)

// terminatorSet is a read's terminators: a 256-bit mask, bit n for
// character n.
type terminatorSet [8]uint32

func (s *terminatorSet) has(b byte) bool { return s[b/32]&(1<<(b%32)) != 0 }

// terminators reads a read's terminator set from p4:
//
//   - 0: the default set (defaultTerminators).
//   - Otherwise p4 is the address of a quadword. When its first word (a
//     mask size) is 0, this is the "short form": the second longword is
//     the mask for control characters 0-31, and no other character
//     terminates. When it's nonzero, the second longword is the address
//     of a mask of that many bytes, bit n of the whole mask for
//     character n; characters past its end don't terminate.
//
// ok is false if the quadword or the mask can't be read.
func (env *Environment) terminators(p4 uint32) (set terminatorSet, ok bool) {
	if p4 == 0 {
		set[0] = defaultTerminators

		return set, true
	}

	size, err := env.mem.LoadWord(env.cpu, p4)
	if err != nil {
		return set, false
	}

	second, err := env.mem.LoadLongword(env.cpu, p4+4)
	if err != nil {
		return set, false
	}

	if size == 0 {
		set[0] = second

		return set, true
	}

	for i := uint32(0); i < uint32(min(size, 32)); i++ {
		b, err := env.mem.LoadByte(env.cpu, second+i)
		if err != nil {
			return set, false
		}

		set[i/4] |= uint32(b) << (8 * (i % 4))
	}

	return set, true
}

// ttRead is IO$_READVBLK (and IO$_READLBLK, IO$_READPBLK,
// IO$_TTYREADALL):
//
//	p1  buffer address          p3  time limit, in seconds (IO$M_TIMED)
//	p2  buffer size             p4  terminator set (see terminators)
//
// Modifiers: IO$M_CVTLOW converts lower-case letters to upper case as
// they're stored; IO$M_PURGE discards typed-ahead input first;
// IO$M_TIMED is described in this file's opening comment. See the
// opening comment for how a read ends and what the IOSB holds.
func ttRead(env *Environment, req *ioRequest) (ioStatus, uint32) {
	return env.terminalRead(req, "")
}

// ttReadPrompt is IO$_READPROMPT (and IO$_TTYREADPALL): ttRead, after
// writing the p6-byte prompt at p5 to the terminal. The prompt is written
// as it is, without carriage control.
func ttReadPrompt(env *Environment, req *ioRequest) (ioStatus, uint32) {
	prompt, err := loadBytes(env, req.p[4], int(req.p[5]&0xFFFF))
	if err != nil {
		return ioStatus{}, ssAccVio
	}

	return env.terminalRead(req, prompt)
}

// terminalRead is the body of every read: see ttRead.
func (env *Environment) terminalRead(req *ioRequest, prompt string) (ioStatus, uint32) {
	buf, size := req.p[0], req.p[1]&0xFFFF

	if !env.accessible(buf, size, vm.AccessWrite) {
		return ioStatus{}, ssAccVio
	}

	set, ok := env.terminators(req.p[3])
	if !ok {
		return ioStatus{}, ssAccVio
	}

	r := env.consoleReader()

	if req.modified(ioModPurge) {
		_, _ = r.Discard(r.Buffered())
	}

	env.writeConsole(prompt)

	// A zero time limit means "only what's already typed ahead".
	pollOnly := req.modified(ioModTimed) && req.p[2] == 0

	var (
		count      uint32
		terminator byte
		status     = uint32(ssNormal)
	)

	for count < size {
		if pollOnly && r.Buffered() == 0 {
			status = ssTimeout

			break
		}

		b, err := readTerminalByte(r)
		if err != nil {
			status = ssEndOfFile

			break
		}

		if set.has(b) {
			terminator = b

			break
		}

		if req.modified(ioModCvtLow) && b >= 'a' && b <= 'z' {
			b -= 'a' - 'A'
		}

		// The buffer was checked above, so a failure here can't happen.
		_ = env.mem.StoreByte(env.cpu, buf+count, b)
		count++
	}

	var terminatorSize uint32

	if terminator != 0 {
		terminatorSize = 1

		if count < size {
			_ = env.mem.StoreByte(env.cpu, buf+count, terminator)
		}
	}

	return ioStatus{status: status, count: uint16(count), info: uint32(terminator) | terminatorSize<<16}, 0
}

// readTerminalByte reads one character as a terminal would deliver it:
// a host newline, or a "\r\n" pair, becomes a single carriage return.
func readTerminalByte(r *bufio.Reader) (byte, error) {
	b, err := r.ReadByte()
	if err != nil {
		return 0, err
	}

	switch b {
	case '\n':
		return ttCarriageReturn, nil

	case '\r':
		// Swallow the "\n" of a "\r\n" pair, but only if it has already
		// arrived: waiting for it would block a read that's complete.
		if r.Buffered() > 0 {
			if next, err := r.Peek(1); err == nil && next[0] == '\n' {
				_, _ = r.ReadByte()
			}
		}

		return ttCarriageReturn, nil
	}

	return b, nil
}

// ttWrite is IO$_WRITEVBLK (and IO$_WRITELBLK, IO$_WRITEPBLK):
//
//	p1  buffer address          p4  carriage control (see carriageControl)
//	p2  number of bytes
//
// The bytes are written to the console as they are, between the carriage
// control's prefix and postfix. The IOSB's transfer count is p2.
func ttWrite(env *Environment, req *ioRequest) (ioStatus, uint32) {
	size := req.p[1] & 0xFFFF

	if !env.accessible(req.p[0], size, vm.AccessRead) {
		return ioStatus{}, ssAccVio
	}

	data, err := loadBytes(env, req.p[0], int(size))
	if err != nil {
		return ioStatus{}, ssAccVio
	}

	prefix, postfix := carriageControl(req.p[3])
	env.writeConsole(prefix + data + postfix)

	return ioStatus{status: ssNormal, count: uint16(size)}, 0
}

// carriageControl turns a terminal write's p4 into the characters to
// write before and after the data. There are two forms, as in an RMS
// print-format record:
//
//   - Byte 0 nonzero: a FORTRAN carriage-control character. " " (the
//     usual one) starts a new line and returns the carriage afterwards;
//     "0" starts two new lines; "1" starts a new page; "+" overprints
//     (no new line); "$" starts a new line but leaves the carriage at
//     the end, for a prompt. Any other character acts like " ".
//   - Byte 0 zero: byte 2 is a prefix and byte 3 a postfix, each coded
//     as follows. 0 is nothing. 1-127 is that many new lines (line
//     feeds). With bit 7 set and bit 6 clear, bits 0-4 are a control
//     character (0-31) to write; with bits 7 and 6 set, bits 0-4 select
//     one of the C1 control characters (128-159). Other values are
//     nothing.
//
// p4 of 0 is no carriage control at all, the usual case for programs
// that put their own carriage returns and line feeds in the data.
func carriageControl(p4 uint32) (prefix, postfix string) {
	const newLine, carriageReturn, formFeed = "\n", "\r", "\f"

	if c := byte(p4); c != 0 {
		switch c {
		case '0':
			return newLine + newLine, carriageReturn
		case '1':
			return formFeed, carriageReturn
		case '+':
			return "", carriageReturn
		case '$':
			return newLine, ""
		default: // ' ', and anything else
			return newLine, carriageReturn
		}
	}

	return printControl(byte(p4 >> 16)), printControl(byte(p4 >> 24))
}

// printControl decodes one prefix or postfix byte of carriageControl's
// second form.
func printControl(b byte) string {
	switch {
	case b&0x80 == 0: // 0-127: that many new lines
		out := make([]byte, b)
		for i := range out {
			out[i] = '\n'
		}

		return string(out)

	case b&0xE0 == 0x80: // 100x xxxx: a C0 control character
		return string([]byte{b & 0x1F})

	case b&0xE0 == 0xC0: // 110x xxxx: a C1 control character
		return string([]byte{0x80 | b&0x1F})
	}

	return ""
}

// ttSenseMode is IO$_SENSEMODE (and IO$_SENSECHAR): the terminal's
// characteristics (this file's opening comment has the layout) go to the
// buffer at p1, 12 bytes if p2 is at least 12 and 8 otherwise. p1 of 0
// returns nothing but still completes.
//
// With IO$M_TYPEAHDCNT, p1 instead gets the type-ahead count: a word
// holding how many characters are waiting to be read, then the first of
// them (0 if none), in an 8-byte buffer.
func ttSenseMode(env *Environment, req *ioRequest) (ioStatus, uint32) {
	buf := req.p[0]
	d := req.channel.Device

	var data []byte

	if req.modified(ioModTypeahead) {
		r := env.consoleReader()
		n := min(r.Buffered(), 0xFFFF)

		first := byte(0)
		if n > 0 {
			if peek, err := r.Peek(1); err == nil {
				first = peek[0]
			}
		}

		data = []byte{byte(n), byte(n >> 8), first, 0, 0, 0, 0, 0}
	} else {
		width := d.DevBufSize
		data = []byte{
			byte(d.DevClass), byte(d.DevType), byte(width), byte(width >> 8),
			byte(d.DevDepend), byte(d.DevDepend >> 8), byte(d.DevDepend >> 16), byte(d.DevDepend >> 24),
		}

		if req.p[1] >= 12 {
			data = append(data, byte(d.DevDepend2), byte(d.DevDepend2>>8), byte(d.DevDepend2>>16), byte(d.DevDepend2>>24))
		}
	}

	if buf != 0 {
		if !env.accessible(buf, uint32(len(data)), vm.AccessWrite) {
			return ioStatus{}, ssAccVio
		}

		if err := env.mem.Store(env.cpu, buf, data); err != nil {
			return ioStatus{}, ssAccVio
		}
	}

	// The IOSB's second longword would report line speeds, fill counts,
	// and parity, none of which a console has.
	return ioStatus{status: ssNormal}, 0
}

// ttSetMode is IO$_SETMODE (and IO$_SETCHAR): sets the terminal's
// characteristics from the buffer at p1, laid out as IO$_SENSEMODE
// returns them (12 bytes if p2 is at least 12, else 8). The device class
// in byte 0 can't be changed and is ignored. p1 of 0 changes nothing.
//
// With a modifier, p1 means something else. IO$M_CTRLCAST and
// IO$M_CTRLYAST enable a CTRL/C or CTRL/Y AST (ttAttentionAST); other
// modifiers (IO$M_OUTBAND, IO$M_HANGUP, ...) succeed without doing
// anything: see this file's opening comment.
func ttSetMode(env *Environment, req *ioRequest) (ioStatus, uint32) {
	switch {
	case req.modified(ioModCtrlCAST):
		return env.ttAttentionAST(req, AttentionCtrlC)
	case req.modified(ioModCtrlYAST):
		return env.ttAttentionAST(req, AttentionCtrlY)
	}

	buf := req.p[0]
	if req.modifiers != 0 || buf == 0 {
		return ioStatus{status: ssNormal}, 0
	}

	size := uint32(8)
	if req.p[1] >= 12 {
		size = 12
	}

	if !env.accessible(buf, size, vm.AccessRead) {
		return ioStatus{}, ssAccVio
	}

	data := make([]byte, size)
	if err := env.mem.Load(env.cpu, buf, data); err != nil {
		return ioStatus{}, ssAccVio
	}

	long := func(i int) uint32 {
		return uint32(data[i]) | uint32(data[i+1])<<8 | uint32(data[i+2])<<16 | uint32(data[i+3])<<24
	}

	d := req.channel.Device
	d.DevType = uint32(data[1])
	d.DevBufSize = uint32(data[2]) | uint32(data[3])<<8
	d.DevDepend = long(4)

	if size == 12 {
		d.DevDepend2 = long(8)
	}

	return ioStatus{status: ssNormal}, 0
}

// ttAttentionAST is IO$_SETMODE with IO$M_CTRLCAST or IO$M_CTRLYAST:
//
//	p1  the AST routine's address (0 cancels the channel's request)
//	p2  the AST parameter
//	p3  the access mode to deliver it in (maximized with the caller's)
//
// It enables a one-shot AST for the next CTRL/C or CTRL/Y typed (key),
// replacing any this channel enabled before (ctrlast.go).
func (env *Environment) ttAttentionAST(req *ioRequest, key byte) (ioStatus, uint32) {
	mode := max(req.p[2]&3, uint32(env.cpu.PSL().CurMod()))
	env.armAttentionAST(key, req.channel.Number, req.p[0], req.p[1], mode)

	return ioStatus{status: ssNormal}, 0
}

// writeConsole writes s to the console output stream, if there is one.
// A write error (a closed host stream) has nowhere to be reported: a VMS
// terminal write can't fail that way.
func (env *Environment) writeConsole(s string) {
	if env.consoleOut == nil || s == "" {
		return
	}

	_, _ = io.WriteString(env.consoleOut, s)
}
