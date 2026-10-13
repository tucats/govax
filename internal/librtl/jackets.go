package librtl

import (
	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/vmsdef"
)

// The system service jackets (docs/PHASE-55, the survey's "Jackets"):
// LIB$SYS_ASCTIM, LIB$SYS_FAO, LIB$SYS_FAOL, LIB$SYS_GETMSG, and
// LIB$SYS_TRNLOG. A system service writes its string into a fixed
// buffer, which suits MACRO but not a language whose strings are
// dynamic or varying. Each jacket takes the service's own arguments, in
// the service's order, and returns the string through an output
// descriptor of any class.
//
// A jacket works as a VAX routine would: it calls the service with a
// scratch buffer in its own stack frame, then copies the result into the
// caller's string with writeString's class rules (padding a fixed
// string, reallocating a dynamic one, setting a varying one's length).
// A truncated copy is LIB$_STRTRU, which overrides the service's own
// success status (unconfirmed which status wins; the survey). A service
// failure is returned as it is, with nothing copied.

// Statuses the jackets return.
var ssBufferOvf = vmsdef.Symbols["SS$_BUFFEROVF"]

// Scratch buffer sizes. The FAO jackets' is the manual's "limited to 256
// bytes"; $GETMSG's messages are no longer, and a $TRNLOG equivalence
// string is at most 255 characters. $ASCTIM's longest text is 23.
const (
	faoScratch    = 256
	getmsgScratch = 256
	trnlogScratch = 255
	asctimScratch = 23
)

// scratch is a service's output buffer in the jacket's stack frame: a
// class S descriptor (desc) for size bytes, and a word (length) for the
// service's result length.
type scratch struct {
	desc, length, text uint32
}

// scratchBuffer reserves a scratch buffer of size bytes below SP and
// moves SP below it. As with stackString, the space is the routine's
// call frame's, so its RET gives it back.
func scratchBuffer(env *corevms.Environment, size uint32) (scratch, error) {
	mem, cpu := env.Memory(), env.CPU()
	base := cpu.GPR(registers.SP) - ((size+3)&^3 + 12)
	s := scratch{desc: base, length: base + 8, text: base + 12}

	if err := mem.StoreLongword(cpu, s.desc, size|dscClassS<<24|dscDTypeT<<16); err != nil {
		return s, err
	}

	if err := mem.StoreLongword(cpu, s.desc+4, s.text); err != nil {
		return s, err
	}

	if err := mem.StoreLongword(cpu, s.length, 0); err != nil {
		return s, err
	}

	cpu.SetGPR(registers.SP, base)

	return s, nil
}

// copyResult copies what a service left in s to the caller's output
// string dest, and its length to the word at resultLength (if not 0),
// returning LIB$_STRTRU if the copy was cut short and status (the
// service's) otherwise.
func copyResult(env *corevms.Environment, s scratch, dest, resultLength, status uint32) (uint32, error) {
	n, err := env.Memory().LoadWord(env.CPU(), s.length)
	if err != nil {
		return memoryFault(env, err)
	}

	text, err := readText(env, s.text, uint32(n))
	if err != nil {
		return memoryFault(env, err)
	}

	stored, truncated, err := writeString(env, dest, text)
	if err != nil {
		return stringFailure(env, err)
	}

	if resultLength != 0 {
		if err := env.Memory().StoreWord(env.CPU(), resultLength, uint16(stored)); err != nil {
			return memoryFault(env, writing(err))
		}
	}

	if truncated {
		return libStrTru, nil
	}

	return status, nil
}

// runJacket calls service with args, whose output buffer and length are
// a scratch buffer of size bytes (the slots outBuf and outLen of args,
// filled in here), then copies the result to dest and its length to
// resultLength (copyResult). A failing service's status is returned
// with nothing copied.
func runJacket(env *corevms.Environment, service string, args []uint32, outLen, outBuf int, size, dest, resultLength uint32) (uint32, error) {
	s, err := scratchBuffer(env, size)
	if err != nil {
		return memoryFault(env, writing(err))
	}

	args[outLen], args[outBuf] = s.length, s.desc

	status, err := env.CallService(service, args)
	if err != nil || status&1 == 0 {
		return status, err
	}

	return copyResult(env, s, dest, resultLength, status)
}

// optionalLongword reads the longword at addr, an argument passed by
// reference, or returns fallback when addr is 0 (omitted).
func optionalLongword(env *corevms.Environment, addr, fallback uint32) (uint32, error) {
	if addr == 0 {
		return fallback, nil
	}

	return env.Memory().LoadLongword(env.CPU(), addr)
}

// libSysAsctim is LIB$SYS_ASCTIM:
//
//	LIB$SYS_ASCTIM [resultant-length.wwu.r] ,time-string.wt.dx
//	               [,user-time.rq.r] [,flags.rlu.r]
//
// $ASCTIM's arguments, but flags by reference: user-time (an address; 0
// or omitted for the current time) is converted to text, the whole of it
// (23 characters for an absolute time, 16 for a delta, 11 for the time
// of day with flags bit 0), then copied to time-string. A fixed string
// of 12 gets the date, the manual's example. Returns SS$_NORMAL,
// LIB$_STRTRU, SS$_IVTIME (a delta of 10,000 days or more), or the
// string statuses.
func libSysAsctim(env *corevms.Environment, argv []uint32) (uint32, error) {
	flags, err := optionalLongword(env, arg(argv, 3), 0)
	if err != nil {
		return memoryFault(env, err)
	}

	args := []uint32{0, 0, arg(argv, 2), flags}

	return runJacket(env, "SYS$ASCTIM", args, 0, 1, asctimScratch, arg(argv, 1), arg(argv, 0))
}

// libSysFao is LIB$SYS_FAO:
//
//	LIB$SYS_FAO character-string.rt.dx ,[resultant-length.wwu.r]
//	            ,resultant-string.wt.dx [,directive-argument...]
//
// $FAO's arguments and statuses (sysFAO).
func libSysFao(env *corevms.Environment, argv []uint32) (uint32, error) {
	return sysFAO(env, "SYS$FAO", argv)
}

// libSysFaol is LIB$SYS_FAOL:
//
//	LIB$SYS_FAOL character-string.rt.dx ,[resultant-length.wwu.r]
//	             ,resultant-string.wt.dx ,directive-argument-address.rlu.ra
//
// $FAOL's arguments and statuses (sysFAO): the fourth is the address of
// the longword array of directive arguments.
func libSysFaol(env *corevms.Environment, argv []uint32) (uint32, error) {
	return sysFAO(env, "SYS$FAOL", argv)
}

// sysFAO is LIB$SYS_FAO's and LIB$SYS_FAOL's work. "If called with other
// than a fixed-length string for output, the length of the resultant
// string is limited to 256 bytes and truncation will occur": so a fixed
// string (classes Z, S, and SD, whose DSC$W_LENGTH is the buffer's) is
// the service's buffer itself, written as $FAO writes one (unpadded,
// SS$_BUFFEROVF when the text doesn't fit), and any other class gets the
// text of a 256-byte scratch buffer, SS$_BUFFEROVF past it, copied by its
// class (LIB$_STRTRU if the copy is cut). Which classes count as fixed,
// and the missing padding, are unconfirmed (the survey). A failing $FAO
// (SS$_BADPARAM, SS$_ACCVIO) is returned with nothing copied.
func sysFAO(env *corevms.Environment, service string, argv []uint32) (uint32, error) {
	d, err := readDescriptor(env, arg(argv, 2))
	if err != nil {
		return stringFailure(env, err)
	}

	if d.class == dscClassZ || d.class == dscClassS || d.class == dscClassSD {
		return env.CallService(service, argv)
	}

	args := make([]uint32, max(len(argv), 3))
	copy(args, argv)

	return runJacket(env, service, args, 1, 2, faoScratch, arg(argv, 2), arg(argv, 1))
}

// libSysGetmsg is LIB$SYS_GETMSG:
//
//	LIB$SYS_GETMSG message-id.rlu.r ,[message-length.wwu.r]
//	               ,destination-string.wt.dx [,flags.rlu.r]
//	               [,unsigned-resultant-array.wbu.ra]
//
// $GETMSG's arguments, but message-id and flags by reference; flags
// omitted is 15, every part of the message (text, identifier, severity,
// facility). The array gets $GETMSG's four bytes (byte 1 the message's
// FAO argument count). Returns SS$_NORMAL, SS$_MSGNOTFND (a success, the
// stand-in line copied), SS$_BUFFEROVF, LIB$_STRTRU, or the string
// statuses.
func libSysGetmsg(env *corevms.Environment, argv []uint32) (uint32, error) {
	msgid, err := env.Memory().LoadLongword(env.CPU(), arg(argv, 0))
	if err != nil {
		return memoryFault(env, err)
	}

	flags, err := optionalLongword(env, arg(argv, 3), 15)
	if err != nil {
		return memoryFault(env, err)
	}

	args := []uint32{msgid, 0, 0, flags, arg(argv, 4)}

	return runJacket(env, "SYS$GETMSG", args, 1, 2, getmsgScratch, arg(argv, 2), arg(argv, 1))
}

// libSysTrnlog is LIB$SYS_TRNLOG:
//
//	LIB$SYS_TRNLOG logical-name.rt.dx ,[resultant-length.wwu.r]
//	               ,resultant-string.wt.dx [,table.wbu.r]
//	               [,access-mode.wbu.r] [,disable-mask.rlu.v]
//
// The 5.0 manual names it only in its table of jackets (Table 2-1), so
// its arguments are $TRNLOG's, in order, by section 2.1's rule for
// jackets, the disable mask by value as $TRNLOG takes it (unconfirmed;
// the survey). Returns $TRNLOG's statuses (SS$_NORMAL, SS$_NOTRAN with
// the name itself, ...), LIB$_STRTRU, or the string statuses.
func libSysTrnlog(env *corevms.Environment, argv []uint32) (uint32, error) {
	args := []uint32{arg(argv, 0), 0, 0, arg(argv, 3), arg(argv, 4), arg(argv, 5)}

	return runJacket(env, "SYS$TRNLOG", args, 1, 2, trnlogScratch, arg(argv, 2), arg(argv, 1))
}
