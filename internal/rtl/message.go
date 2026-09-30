package rtl

import (
	"fmt"
	"strings"

	"github.com/tucats/govax/internal/vax"
	"github.com/tucats/govax/internal/vmsdef"
)

// $GETMSG and $PUTMSG (docs/PHASE-26.md subtask 25): the text of a
// condition value.
//
// # Messages
//
// Every VMS condition value — the status a service returns in R0, the
// code an exception is signaled with — has a *message*: a line of text
// saying what it means. SS$_ACCVIO (12) is
//
//	%SYSTEM-F-ACCVIO, access violation, reason mask=04, virtual address=00000000, PC=00001234, PS=0000001B
//
// The line has four parts:
//
//   - the *facility* that defines the message (SYSTEM, RMS, CLI, ...),
//     from bits 16-27 of the condition value;
//   - a *severity* letter, from bits 0-2: W (warning), S (success), E
//     (error), I (informational), or F (fatal);
//   - the message's short *identifier* (ACCVIO);
//   - the *text*, which is a $FAO control string (fao.go). Its
//     directives (!XB, !XH, ...) are filled in with values describing
//     this particular occurrence — which address, which PC.
//
// VMS keeps the definitions in message files. govax has the CLI, LIB,
// MTH, OTS, RMS, and SYSTEM facilities of VMS 7.3's system message file,
// generated into vmsdef.Messages.
//
// $GETMSG returns a message's definition, unformatted (the directives
// left in). $PUTMSG takes a *message vector* — one or more condition
// values, each with the $FAO parameters its text needs — formats each
// message, and writes the lines to the terminal: this is how VMS reports
// an error. The first line starts with "%", the rest with "-".
//
// A program can choose which parts appear with four *message flags*: 1
// text, 2 identifier, 4 severity, 8 facility. All four (15) is the
// default, as it is for a process that hasn't changed it with SET
// MESSAGE.

// Message flags, the bits of $GETMSG's flags and $PUTMSG's message
// options.
const (
	msgText     = 1
	msgIdent    = 2
	msgSeverity = 4
	msgFacility = 8

	// defaultMessageFlags is all of them: the process default, used
	// when a caller passes 0.
	defaultMessageFlags = msgText | msgIdent | msgSeverity | msgFacility
)

// Status codes the message services return.
var ssMsgNotFnd = vmsdef.Symbols["SS$_MSGNOTFND"]

// severityLetters are the letters for a condition value's severity (bits
// 0-2). Values 5-7 are reserved; VMS shows them as "?".
var severityLetters = [8]string{"W", "S", "E", "I", "F", "?", "?", "?"}

// Facility numbers $PUTMSG treats specially (bits 16-27 of a condition
// value).
const (
	facilitySystem = 0
	facilityRMS    = 1
)

// messageFor returns the message definition for condition value code. A
// code the message file doesn't have gets the stand-in VMS 7.3 uses,
// with found false:
//
//	%FACILITY-S-NOMSG, Message number XXXXXXXX
//
// where FACILITY is the facility's name if it's one of the message
// file's, and NONAME otherwise.
func messageFor(code uint32) (m vmsdef.Message, found bool) {
	if m, ok := vmsdef.LookupMessage(code); ok {
		return m, true
	}

	facility, ok := vmsdef.MessageFacilities[code>>16&0xFFF]
	if !ok {
		facility = "NONAME"
	}

	return vmsdef.Message{Facility: facility, Ident: "NOMSG", Text: fmt.Sprintf("Message number %08X", code)}, false
}

// messageLine assembles one message line from its parts, as flags
// select them: "%FACILITY-S-IDENT, text", or any subset ("%SYSTEM-F-ACCVIO"
// with the text left out, just the text with the rest left out). prefix
// is the line's first character, "%" or "-". facility replaces the
// definition's facility name if it isn't empty.
func messageLine(code uint32, m vmsdef.Message, text string, flags uint32, prefix, facility string) string {
	if facility == "" {
		facility = m.Facility
	}

	var parts []string

	if flags&msgFacility != 0 {
		parts = append(parts, facility)
	}

	if flags&msgSeverity != 0 {
		parts = append(parts, severityLetters[code&7])
	}

	if flags&msgIdent != 0 {
		parts = append(parts, m.Ident)
	}

	head := ""
	if len(parts) > 0 {
		head = prefix + strings.Join(parts, "-")
	}

	switch {
	case flags&msgText == 0:
		return head
	case head == "":
		return text
	}

	return head + ", " + text
}

// serviceSysGetmsg is SYS$GETMSG:
//
//	SYS$GETMSG msgid ,msglen ,bufadr ,[flags] ,[outadr]
//
// It stores the message for condition value msgid in the buffer bufadr
// describes, with the parts flags selects (0 or omitted: all of them),
// and its length at msglen (a word). The text isn't formatted: its $FAO
// directives are left for the caller to fill in. If outadr isn't 0, four
// bytes are stored there: 0, the number of $FAO parameters the text
// takes, 0 (the message's user value; no message govax has defines
// one), and 0.
//
// It returns SS$_BUFFEROVF if the line didn't fit (it's truncated),
// SS$_MSGNOTFND (a success) with a stand-in line for a code there's no
// message for (see messageFor), SS$_INSFARG for fewer than three
// arguments, and SS$_ACCVIO for an argument it can't access.
func serviceSysGetmsg(env *Environment, argv []uint32) (uint32, error) {
	if len(argv) < 3 {
		return ssInsfArg, nil
	}

	msgid, msglen, bufadr := argv[0], argv[1], argv[2]
	flags, outadr := optArg(argv, 3)&0xF, optArg(argv, 4)

	if flags == 0 {
		flags = defaultMessageFlags
	}

	m, found := messageFor(msgid)
	line := messageLine(msgid, m, m.Text, flags, "%", "")

	n, truncated, err := storeDescriptor(env, bufadr, line)
	if err != nil {
		return ssAccVio, nil
	}

	if err := env.mem.StoreWord(env.cpu, msglen, n); err != nil {
		return ssAccVio, nil
	}

	if outadr != 0 {
		if err := env.mem.StoreLongword(env.cpu, outadr, uint32(m.FAOCount&0xFF)<<8); err != nil {
			return ssAccVio, nil
		}
	}

	switch {
	case truncated:
		return ssBufferOvf, nil
	case !found:
		return ssMsgNotFnd, nil
	}

	return ssNormal, nil
}

// putmsgCall is the progress of one $PUTMSG with an action routine,
// kept while the routine runs (see serviceSysPutmsg). fp identifies the
// call: it's the frame pointer of the SYS$PUTMSG stub's frame, which is
// the same each time the stub's XFC runs.
type putmsgCall struct {
	fp uint32

	// lines are the formatted message lines, and next the one whose
	// action routine is running.
	lines []string
	next  int

	// sp is where $PUTMSG put the routine's argument list, and savedSP
	// the stack pointer before that, restored when the routine returns.
	sp, savedSP uint32
}

// serviceSysPutmsg is SYS$PUTMSG:
//
//	SYS$PUTMSG msgvec ,[actrtn] ,[facnam] ,[actprm]
//
// It formats each message in the message vector msgvec (see
// parseMessageVector) and writes the lines to the terminal, the first
// starting "%" and the rest "-". facnam (a descriptor) replaces the
// first message's facility name.
//
// With an action routine actrtn, each line is first passed to it,
// before it's written: the routine is called with a descriptor of the
// line and actprm, and the line is written only if the low bit of the
// R0 it returns is set. This uses the same mechanism as $EXIT's exit
// handlers: $PUTMSG puts the routine's argument list, the descriptor,
// and the text on the stack, and returns a *CallRequest. The engine calls
// the routine with $PUTMSG's own XFC as its return address, so when the
// routine returns, this service runs again; it recognizes the call in
// progress (putmsgCall, matched by frame pointer and stack pointer),
// writes the line if R0 says so, removes what it put on the stack, and
// goes on to the next line.
//
// It returns SS$_NORMAL, or SS$_ACCVIO if the vector can't be read or
// the stack can't be written.
func serviceSysPutmsg(env *Environment, argv []uint32) (uint32, error) {
	c := env.cpu
	actrtn := optArg(argv, 1)
	p := env.Process

	// A routine returning to an existing call?
	var call *putmsgCall

	if n := len(p.putmsg); n > 0 {
		top := p.putmsg[n-1]
		if top.fp == c.GPR(vax.FP) && top.sp == c.GPR(vax.SP) {
			call = top

			if c.GPR(vax.R0)&1 != 0 {
				env.writeConsole(call.lines[call.next] + "\n")
			}

			c.SetGPR(vax.SP, call.savedSP)
			
			call.next++
		}
	}

	if call == nil {
		lines, st := env.parseMessageVector(optArg(argv, 0), optArg(argv, 2))
		if st != 0 {
			return st, nil
		}

		if actrtn == 0 {
			for _, line := range lines {
				env.writeConsole(line + "\n")
			}

			return ssNormal, nil
		}

		call = &putmsgCall{fp: c.GPR(vax.FP), lines: lines}
		p.putmsg = append(p.putmsg, call)
	}

	if call.next >= len(call.lines) {
		p.putmsg = p.putmsg[:len(p.putmsg)-1]

		return ssNormal, nil
	}

	// Lay out, below SP: the argument list (a count, the descriptor's
	// address, and actprm unless it was omitted), the descriptor, and
	// the text, rounded up to a longword.
	line := call.lines[call.next]
	argc := uint32(2)

	if len(argv) < 4 {
		argc = 1
	}

	call.savedSP = c.GPR(vax.SP)
	sp := call.savedSP - (12 + 8 + (uint32(len(line))+3)&^3)
	desc, text := sp+12, sp+20

	stored := env.mem.StoreLongword(c, sp, argc) == nil &&
		env.mem.StoreLongword(c, sp+4, desc) == nil &&
		env.mem.StoreLongword(c, sp+8, optArg(argv, 3)) == nil &&
		env.mem.StoreLongword(c, desc, uint32(len(line))|dscTextStatic) == nil &&
		env.mem.StoreLongword(c, desc+4, text) == nil &&
		storeString(env, line, text, len(line)) == nil

	if !stored {
		p.putmsg = p.putmsg[:len(p.putmsg)-1]

		return ssAccVio, nil
	}

	c.SetGPR(vax.SP, sp)
	call.sp = sp

	return 0, &CallRequest{Routine: actrtn, ArgList: sp}
}

// dscTextStatic is a string descriptor's second word for a fixed-length
// ("static") text string: type DSC$K_DTYPE_T (14) and class DSC$K_CLASS_S
// (1), in the descriptor's first longword's high bytes.
const dscTextStatic = 14<<16 | 1<<24

// parseMessageVector reads the message vector at msgvec and returns its
// formatted lines (0), or SS$_ACCVIO if it can't be read.
//
// The vector's first longword holds the number of longwords after it
// (low word), and the default message flags (bits 16-19; 0 means all).
// Then come the messages, each a condition value followed by what its
// facility says:
//
//   - A SYSTEM message (facility 0): as many $FAO parameters as its text
//     takes (none for most; four for SS$_ACCVIO). So the longword after a
//     parameterless one is the next message.
//   - An RMS message (facility 1): one longword, the "status value" (STV)
//     that RMS returns alongside its status. If the text takes a
//     parameter, the STV is it; otherwise a nonzero STV is a SYSTEM
//     condition value explaining the RMS one, written as a message of its
//     own.
//   - Any other facility: a longword whose low word is the number of $FAO
//     parameters that follow, and whose high word, if not 0, is new
//     message flags for this and the remaining messages.
//
// If a message's text can't be formatted (a parameter missing from the
// vector, or a string parameter that can't be read), its text is written
// unformatted, as the manual says ("FAO parameters ... do not appear").
func (env *Environment) parseMessageVector(msgvec, facnam uint32) ([]string, uint32) {
	if msgvec == 0 {
		return nil, ssAccVio
	}

	head, err := env.mem.LoadLongword(env.cpu, msgvec)
	if err != nil {
		return nil, ssAccVio
	}

	count, flags := int(head&0xFFFF), head>>16&0xF
	if flags == 0 {
		flags = defaultMessageFlags
	}

	facility := ""

	if facnam != 0 {
		name, _, err := strGet(env, facnam, maxFAOOutput)
		if err != nil {
			return nil, ssAccVio
		}

		facility = name
	}

	vec := make([]uint32, count)

	for i := range vec {
		if vec[i], err = env.mem.LoadLongword(env.cpu, msgvec+uint32(4*(i+1))); err != nil {
			return nil, ssAccVio
		}
	}

	return env.formatMessageVector(vec, nil, flags, facility), 0
}

// formatMessageVector formats the messages in vec, a message vector
// without its leading count longword (see parseMessageVector for its
// layout), with message flags flags. facility, if not empty, replaces
// the first message's facility name.
//
// tail is what follows the vector in memory, for a caller that knows:
// the last message's $FAO parameters may run on into it. VMS's $PUTMSG
// hands $FAOL a pointer into the vector, so a text that wants more
// parameters than the vector has left simply reads on. The one caller
// that relies on this is the catch-all condition handler (condition.go):
// it formats a signal array without its PC and PSL, as VMS's does, and
// the system exception messages (SS$_ACCVIO, ...) pick the PC and PSL up
// from here. $PUTMSG passes nil, so a missing parameter leaves the text
// unformatted instead.
func (env *Environment) formatMessageVector(vec, tail []uint32, flags uint32, facility string) []string {
	var lines []string

	// add formats one message and appends its line.
	add := func(code uint32, params []uint32) {
		m, _ := messageFor(code)

		text, st := env.formatFAO(m.Text, func(i int) (uint32, bool) {
			if i < len(params) {
				return params[i], true
			}

			return 0, false
		})
		if st != 0 {
			text = m.Text
		}

		prefix := "-"
		if len(lines) == 0 {
			prefix = "%"
		}

		lines = append(lines, messageLine(code, m, text, flags, prefix, facility))
		facility = "" // facnam is only for the first message
	}

	// take returns the next n longwords of the vector (fewer at its end).
	i := 0
	take := func(n int) []uint32 {
		n = min(max(n, 0), len(vec)-i)
		out := vec[i : i+n]
		i += n

		return out
	}

	// params is take for a message's $FAO parameters, which, at the end
	// of the vector, run on into tail (see above).
	params := func(n int) []uint32 {
		out := take(n)
		if short := n - len(out); short > 0 && i == len(vec) {
			out = append(append([]uint32(nil), out...), tail[:min(short, len(tail))]...)
		}

		return out
	}

	for i < len(vec) {
		code := take(1)[0]
		m, _ := messageFor(code)

		switch code >> 16 & 0xFFF {
		case facilitySystem:
			add(code, params(m.FAOCount))

		case facilityRMS:
			stv := take(1)

			switch {
			case m.FAOCount > 0:
				add(code, stv)
			case len(stv) == 1 && stv[0] != 0:
				add(code, nil)
				add(stv[0], nil)
			default:
				add(code, nil)
			}

		default:
			n, options := 0, uint32(0)

			if w := take(1); len(w) == 1 {
				n, options = int(w[0]&0xFFFF), w[0]>>16&0xF
			}

			if options != 0 {
				flags = options
			}

			add(code, params(n))
		}
	}

	return lines
}

// cancelPutmsgCalls is image rundown's $PUTMSG step: a $PUTMSG whose
// action routine never returned (it called $EXIT, say) is forgotten.
func (env *Environment) cancelPutmsgCalls() {
	env.Process.putmsg = nil
}

func registerMessageServices(t *ServiceTable) {
	t.Register("SYS$GETMSG", serviceSysGetmsg)
	t.Register("SYS$PUTMSG", serviceSysPutmsg)
}
