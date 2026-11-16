package authorize

import (
	"fmt"
	"strings"

	"github.com/tucats/govax/internal/uaf"
	"github.com/tucats/govax/internal/vmsdef"
)

// This file is how AUTHORIZE writes its messages: its own (the UAF
// facility's, vmsdef.UAFMessages), the CLI facility's for a command it
// can't parse, and a system status under one of its own.

// Condition values a one-command run leaves in $STATUS.
const (
	ssNormal      = 1          // SS$_NORMAL
	stsInhibitMsg = 0x10000000 // STS$M_INHIB_MSG: the message has been shown
)

// severityLetters are the letters a message shows for a condition's
// severity, bits 0 to 2.
var severityLetters = []byte{'W', 'S', 'E', 'I', 'F', '?', '?', '?'}

// message writes AUTHORIZE's message ident, its "!AS"s replaced by args
// in order; a segment, if not "", follows the text between backslashes,
// as VMS 7.1 showed UICERR's "\[300]\".
func (u *Utility) message(ident, segment string, args ...string) {
	m, ok := vmsdef.UAFMessages[ident]
	if !ok {
		panic("authorize: no message " + ident)
	}

	text := m.Text
	for _, a := range args {
		text = strings.Replace(text, "!AS", a, 1)
	}

	if segment != "" {
		text += ` \` + segment + `\`
	}

	fmt.Fprintf(u.Out, "%%UAF-%c-%s, %s\n", m.Severity, ident, text)
}

// cause writes the message of condition value code under the message
// before it: "-SYSTEM-F-DUPIDENT, duplicate identifier".
func (u *Utility) cause(code uint32) {
	m, ok := vmsdef.LookupMessage(code)
	if !ok {
		fmt.Fprintf(u.Out, "-NONAME-%c-NOMSG, Message number %08X\n", severityLetters[code&7], code)

		return
	}

	fmt.Fprintf(u.Out, "-%s-%c-%s, %s\n", m.Facility, severityLetters[code&7], m.Ident, m.Text)
}

// The system and RMS statuses AUTHORIZE puts under its messages.
var (
	ssDUPIDENT = vmsdef.Symbols["SS$_DUPIDENT"]
	ssDUPLNAM  = vmsdef.Symbols["SS$_DUPLNAM"]
	rmsPRV     = vmsdef.Symbols["RMS$_PRV"]
)

// uafStatus is the condition value AUTHORIZE's message ident leaves, as
// VMS 7.1 showed it.
func uafStatus(ident string) uint32 { return vmsdef.UAFMessages[ident].Status }

// identifierValue is an identifier's value as AUTHORIZE's messages show
// it: a UIC identifier "[000300,000001]", a general one "%X80010000".
func identifierValue(v uaf.IdentifierValue) string {
	if v&uaf.GeneralBit != 0 {
		return v.String()
	}

	u := uaf.UIC(v)

	return fmt.Sprintf("[%06o,%06o]", u.Group(), u.Member())
}
