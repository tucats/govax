package vmsdef

// Message is one VMS message's definition, from the system message file
// (VMS 7.3's, merged into Messages by internal/vmsdef/gen): what $GETMSG and
// $PUTMSG show for a condition value.
//
// A message is written "%FACILITY-S-IDENT, text": for SS$_ACCVIO that's
// "%SYSTEM-F-ACCVIO, access violation, reason mask=!XB, ...". The
// severity letter S isn't part of the definition: it comes from the
// condition value being shown.
type Message struct {
	// Facility is the name of the facility that defines the message
	// (SYSTEM, RMS, ...), and Ident its short identifier (ACCVIO).
	Facility string
	Ident    string

	// Text is the message text, a $FAO control string: "!XB" and the
	// like are replaced by values that describe one occurrence.
	Text string

	// FAOCount is how many $FAO parameters Text consumes.
	FAOCount int
}

// messageKeyMask selects the bits of a condition value that name a
// message: its facility (bits 16-27) and message number (bits 3-15). The
// severity (bits 0-2) and control bits (28-31) don't, so SS$_ACCVIO (12)
// and the same code with another severity find the same message.
const messageKeyMask = 0x0FFFFFF8

// LookupMessage returns the message for condition value code, if the
// system message file (or govax's own, govaxMessages) has one. The system facility's message 0 is
// SS$_NORMAL's, a success only: VMS 7.3 showed the other severities of
// it (0, 2, and 4) as having no message (testdata/dcl50's EXIT cases).
func LookupMessage(code uint32) (Message, bool) {
	if code&messageKeyMask == 0 && code&7 != 1 {
		return Message{}, false
	}

	m, ok := vaxMessages[code&messageKeyMask]
	if !ok {
		m, ok = Messages[code&messageKeyMask]
	}

	if !ok {
		m, ok = observedMessages[code&messageKeyMask]
	}

	if !ok {
		m, ok = forMessages[code&messageKeyMask]
	}

	if !ok {
		m, ok = strMessages[code&messageKeyMask]
	}

	if !ok {
		m, ok = backupMessages[code&messageKeyMask]
	}

	if !ok {
		m, ok = lbrMessages[code&messageKeyMask]
	}

	if !ok {
		m, ok = jbcMessages[code&messageKeyMask]
	}

	if !ok {
		m, ok = govaxMessages[code&messageKeyMask]
	}

	if !ok && code&facilitySpecific == 0 {
		m, ok = sharedMessage(code)
	}

	return m, ok
}

// facilitySpecific is STS$V_FAC_SP (bit 15 of a condition value): set,
// the message number is the facility's own; clear, it is one of the
// shared messages (SHR$), which any facility may use, shown with that
// facility's name: LIB$_NOWILD (%X15112A) is SHR$_NOWILD (%X1128) as
// %LIB-E-NOWILD.
const facilitySpecific = 0x8000

// sharedMessages are the shared (SHR$) messages, by message number (bits
// 3-14, the severity cleared), as VMS 7.1's $GETMSG showed them
// (testdata/lib58): the system message file govax's tables were made
// from doesn't have them.
//
// The rest are the shared messages govax's DCL utilities report, worded
// as VMS 7.1 showed them in the probes' logs (testdata/prot93,
// testdata/bat98, testdata/dcl50), so that $GETMSG, and a batch job's
// status in SHOW QUEUE and /NOTIFY, can show a utility's status.
var sharedMessages = map[uint32]Message{
	0x1098: {Ident: "OPENIN", Text: "error opening !AS as input", FAOCount: 1},
	0x10A0: {Ident: "OPENOUT", Text: "error opening !AS as output", FAOCount: 1},
	0x10B0: {Ident: "READERR", Text: "error reading !AS", FAOCount: 1},
	0x10F8: {Ident: "SYNTAX", Text: "error parsing '!AS'", FAOCount: 1},
	0x1110: {Ident: "BADVALUE", Text: "'!AS' is an invalid keyword value", FAOCount: 1},
	0x1128: {Ident: "NOWILD", Text: "no wildcard permitted"},
	0x11F0: {Ident: "FILNOTDEL", Text: "error deleting !AS", FAOCount: 1},
	0x1230: {Ident: "FILNOTPUR", Text: "error deleting !AS", FAOCount: 1},
	0x1238: {Ident: "SEARCHFAIL", Text: "error searching for !AS", FAOCount: 1},
	0x1248: {Ident: "PARSEFAIL", Text: "error parsing !AS", FAOCount: 1},
	0x1328: {Ident: "INVQUAVAL", Text: "value '!AS' invalid for /!AS qualifier", FAOCount: 2},
	0x1330: {Ident: "NOFILES", Text: "no files found"},
	0x1378: {Ident: "NOTDELETED", Text: "error deleting !AS", FAOCount: 1},
}

// sharedMessage returns the shared message code names, with the
// facility's name the code's (SYSTEM for facility 0). A number the shared
// messages don't have is the system message of that number: VMS 7.1's
// $GETMSG gave JBC$_NOOPER (%X42892) as %JBC-E-NOOPER, SS$_NOOPER's
// text, and every message number from 0 to 1,023 of the JBC facility as
// the system message with JBC's name, whatever the severity
// (testdata/bat98/vax/bat98b.log's JBC MESSAGES).
func sharedMessage(code uint32) (Message, bool) {
	fac := code >> 16 & 0xFFF

	name, ok := MessageFacilities[fac]
	if !ok {
		name, ok = otherFacilities[fac]
	}

	if !ok {
		return Message{}, false
	}

	m, ok := sharedMessages[code&0x7FF8]
	if !ok && fac != 0 {
		if m, ok = vaxMessages[code&0x7FF8]; !ok {
			m, ok = Messages[code&0x7FF8]
		}
	}

	if !ok {
		return Message{}, false
	}

	m.Facility = name

	return m, true
}

// otherFacilities are facilities' names the system message file govax's
// tables were made from doesn't give (MessageFacilities), for the
// messages they share.
// The DCL utilities' are the facility numbers govax's commands put in
// their statuses, from the probes' $STATUS values (COPY's unconfirmed).
var otherFacilities = map[uint32]string{
	JBCFacility:  "JBC",
	DIFFFacility: "DIFF",
	103:         "COPY",
	119:         "SET",
	120:         "SHOW",
	146:         "DIRECT",
	147:         "DELETE",
	149:         "TYPE",
	193:         "SUBMIT",
	251:         "QUEMAN",
}

// vaxMessages are messages VMS on a VAX words differently from the
// message file govax's tables were made from, whose text is in places the
// Alpha's (a 64-bit address shown with !XH, the PS rather than the PSL).
// LookupMessage finds them first. SS$_ACCVIO's is VMS 7.1's on a VAX
// (testdata/lib55, LIB$SYS_GETMSG), as every VAX log in testdata shows
// it. The others are the conditions of the hardware exceptions govax
// signals (corevms's exceptionCondition), whose PC and PSL a VAX shows
// as longwords after "PC=" and "PSL=": OPCDEC's, ROPRAND's, INTOVF's,
// and INTDIV's are as VMS 7.1 showed them (testdata/tb70), and TBIT's
// (testdata/trace73); the rest follow them, unconfirmed.
var vaxMessages = map[uint32]Message{
	0x00000008: {Facility: "SYSTEM", Ident: "ACCVIO", Text: "access violation, reason mask=!XB, virtual address=!XL, PC=!XL, PSL=!XL", FAOCount: 4},
	0x00000430: {Facility: "SYSTEM", Ident: "OPCCUS", Text: "opcode reserved to customer fault at PC=!XL, PSL=!XL", FAOCount: 2},
	0x00000438: {Facility: "SYSTEM", Ident: "OPCDEC", Text: "opcode reserved to Digital fault at PC=!XL, PSL=!XL", FAOCount: 2},
	0x00000448: {Facility: "SYSTEM", Ident: "RADRMOD", Text: "reserved addressing fault at PC=!XL, PSL=!XL", FAOCount: 2},
	0x00000450: {Facility: "SYSTEM", Ident: "ROPRAND", Text: "reserved operand fault at PC=!XL, PSL=!XL", FAOCount: 2},
	0x00000460: {Facility: "SYSTEM", Ident: "TBIT", Text: "T-bit pending trap at PC=!XL, PSL=!XL", FAOCount: 2},
	0x00000470: {Facility: "SYSTEM", Ident: "ARTRES", Text: "reserved arithmetic trap at PC=!XL, PSL=!XL", FAOCount: 2},
	0x00000478: {Facility: "SYSTEM", Ident: "INTOVF", Text: "arithmetic trap, integer overflow at PC=!XL, PSL=!XL", FAOCount: 2},
	0x00000480: {Facility: "SYSTEM", Ident: "INTDIV", Text: "arithmetic trap, integer divide by zero at PC=!XL, PSL=!XL", FAOCount: 2},
	0x00000488: {Facility: "SYSTEM", Ident: "FLTOVF", Text: "arithmetic trap, floating overflow at PC=!XL, PSL=!XL", FAOCount: 2},
	0x00000490: {Facility: "SYSTEM", Ident: "FLTDIV", Text: "arithmetic trap, floating/decimal divide by zero at PC=!XL, PSL=!XL", FAOCount: 2},
	0x00000498: {Facility: "SYSTEM", Ident: "FLTUND", Text: "arithmetic trap, floating underflow at PC=!XL, PSL=!XL", FAOCount: 2},
	0x000004A0: {Facility: "SYSTEM", Ident: "DECOVF", Text: "arithmetic trap, decimal overflow at PC=!XL, PSL=!XL", FAOCount: 2},
	0x000004A8: {Facility: "SYSTEM", Ident: "SUBRNG", Text: "arithmetic trap, subscript out of range at PC=!XL, PSL=!XL", FAOCount: 2},
	0x000004B0: {Facility: "SYSTEM", Ident: "FLTOVF_F", Text: "arithmetic fault, floating overflow at PC=!XL, PSL=!XL", FAOCount: 2},
	0x000004B8: {Facility: "SYSTEM", Ident: "FLTDIV_F", Text: "arithmetic fault, floating divide by zero at PC=!XL, PSL=!XL", FAOCount: 2},
	0x000004C0: {Facility: "SYSTEM", Ident: "FLTUND_F", Text: "arithmetic fault, floating underflow at PC=!XL, PSL=!XL", FAOCount: 2},
}

// observedMessages are messages the system message file govax's tables
// were made from doesn't have, as VMS 7.3 showed them (F$MESSAGE's
// text): CREATE's OPENOUT (testdata/dcl50's round 5). LookupMessage
// finds them after Messages.
var observedMessages = map[uint32]Message{
	0x009110A0: {Facility: "CREATE", Ident: "OPENOUT", Text: "error opening !AS as output", FAOCount: 1},
}

// GovaxFacility is the facility number of govax's own conditions, those
// that describe govax rather than VMS. Facility numbers with bit 11 set
// (STS$V_CUST_DEF, bit 27 of a condition value) are the ones DIGITAL left
// for software other than its own; govax takes the last of them, which a
// program's own messages are least likely to use.
const GovaxFacility = 0xFFF

// GovaxNotImplemented is GOVAX-E-NOTIMPL, the condition a routine of a
// shareable image govax stands in for, but doesn't implement, signals
// after SS$_UNSUPPORTED (docs/planning/PHASE-53, Decision 6). Its one $FAO
// parameter is the routine's name, by descriptor. Facility-specific (bit
// 15), message 1, severity E.
const GovaxNotImplemented = GovaxFacility<<16 | 1<<15 | 1<<3 | 2

// govaxMessages are govax's own messages, in GovaxFacility. LookupMessage
// finds them after the system's.
var govaxMessages = map[uint32]Message{
	GovaxNotImplemented & messageKeyMask: {Facility: "GOVAX", Ident: "NOTIMPL", Text: "routine !AS is not implemented in govax", FAOCount: 1},
}

// FacilityName is facility fac's name as a message shows it: the system
// message file's (MessageFacilities), or one of govax's utilities'
// (otherFacilities), so a status of DIFFERENCES' with no message shows as
// "%DIFF-I-NOMSG, Message number 006C8013", as VMS 7.1 showed it.
func FacilityName(fac uint32) (string, bool) {
	if name, ok := MessageFacilities[fac]; ok {
		return name, true
	}

	name, ok := otherFacilities[fac]

	return name, ok
}
