package fdl

import (
	"strconv"
	"strings"
)

// Message is one of the FDL facility's messages, as Parse reports them and
// the utilities show them:
//
//	%FDL-W-UNQUAKW, unrecognized qualifier keyword in statement 5
//	 \1\
type Message struct {
	// Ident is the message's name, UNQUAKW; Severity its letter, W.
	Ident    string
	Severity byte

	// Statement is the statement it's about, numbered from 1 through the
	// file; 0 for a message about none (NULLPRI).
	Statement int

	// Segment is the text it's about, shown on a line of its own between
	// backslashes, when HasSegment.
	Segment    string
	HasSegment bool
}

// messageTexts are the FDL facility's texts, as VMS 7.1 showed them in
// round 3 of the probe (testdata/idx88's IDX88C.LOG). A message about a
// statement gets " in statement n" after its text, and one with a segment
// a blank after that; SYNTAX's had three.
var messageTexts = map[string]string{
	"ABKW":     "ambiguous keyword",
	"INVDATIM": "invalid date/time",
	"MULSEC":   "multiple secondary definition",
	"NOQUAL":   "no qualifier",
	"NULLPRI":  "null primary block",
	"OUTORDER": "key or area primary defined out of order",
	"SYNTAX":   "syntax error",
	"UNPRIKW":  "unrecognized primary keyword",
	"UNQUAKW":  "unrecognized qualifier keyword",
	"VALPRI":   "value required on primary",

	// The utilities' own (internal/dclcmd/createfdl.go): the end of a
	// parse, and CREATE/FDL's.
	"WARNING":  "parsed with warnings",
	"FDLERROR": "error parsing FDL file",
	"CREATE":   "error creating !AS",
	"OPENFDL":  "error opening !AS",
	"CREATED":  "!AS created",
}

// Lines returns m as its lines, without the leading "%" or "-":
// "FDL-W-ABKW, ambiguous keyword in statement 3 " and " \CONT\".
func (m Message) Lines() []string {
	text := "FDL-" + string(m.Severity) + "-" + m.Ident + ", " + messageTexts[m.Ident]

	if m.Statement > 0 {
		text += " in statement " + strconv.Itoa(m.Statement)
	}

	if !m.HasSegment {
		return []string{text}
	}

	blanks := " "
	if m.Ident == "SYNTAX" {
		blanks = "   "
	}

	return []string{text + blanks, " \\" + m.Segment + "\\"}
}

// Text returns the FDL facility's message called ident with its one
// argument, arg, in place of "!AS": "FDL-I-CREATED, DUA1:[R3]C23.DAT;1
// created" (without the leading "%").
func Text(ident string, severity byte, arg string) string {
	return "FDL-" + string(severity) + "-" + ident + ", " + strings.Replace(messageTexts[ident], "!AS", arg, 1)
}

// Failed reports whether msgs holds an error, which means no file.
func Failed(msgs []Message) bool {
	for _, m := range msgs {
		if m.Severity == 'E' || m.Severity == 'F' {
			return true
		}
	}

	return false
}
