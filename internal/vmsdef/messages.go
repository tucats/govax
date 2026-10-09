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
// system message file has one. The system facility's message 0 is
// SS$_NORMAL's, a success only: VMS 7.3 showed the other severities of
// it (0, 2, and 4) as having no message (testdata/dcl50's EXIT cases).
func LookupMessage(code uint32) (Message, bool) {
	if code&messageKeyMask == 0 && code&7 != 1 {
		return Message{}, false
	}

	m, ok := Messages[code&messageKeyMask]

	return m, ok
}
