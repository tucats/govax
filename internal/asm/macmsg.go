package asm

import (
	"errors"
	"strings"

	"github.com/tucats/govax/internal/vmsdef"
	"github.com/tucats/govax/internal/vmserrors"
)

// This file says how a listing shows the errors and warnings govax
// reports: as real MACRO reports the same problem, in its own words. A
// message line is "%MACRO-s-IDENT, text", where IDENT is MACRO's
// identifier for the message and text MACRO's text for it, as the probe's
// listing shows them (testdata/mar/list/vax/errors.lis):
//
//	%MACRO-W-DATATRUNC, Data truncation error
//	%MACRO-E-MULDEFLBL, Multiple definition of label   !
//
// The text is MACRO's message definition (vmsdef.Messages, facility
// MACRO) with its first letter capitalized. Most messages end with a "!"
// under the column where MACRO found the problem; the ones it finds only
// once a value is known (macroNoColumn) don't.
//
// govax's terminal messages are unchanged: they're govax's own (VAX-E-
// DATARANGE and so on), with the line they were on.

// macroIdents is MACRO's message for each of govax's assembler errors, as
// far as one fits. The probe settles the ones it raised; the others are
// the MACRO message whose definition describes the same problem. An error
// with none here is listed in govax's own words (macroMessageLine).
var macroIdents = map[uint32]string{
	vmserrors.VAX_ABSDATA:       "ABSDATA",
	vmserrors.VAX_ALIGNPSECT:    "ALIGNXCEED",
	vmserrors.VAX_ALIGNZERO:     "INVALIGN",
	vmserrors.VAX_BADCHARLIT:    "ILLASCARG",
	vmserrors.VAX_BADCOND:       "ILLIFCOND",
	vmserrors.VAX_BADDECIMAL:    "ILLEXPR",
	vmserrors.VAX_BADDIGIT:      "ILLEXPR",
	vmserrors.VAX_BADFLOAT:      "FLTPNTSYNX",
	vmserrors.VAX_BADFORMAL:     "ILLMACARGN",
	vmserrors.VAX_BADHEX:        "ILLEXPR",
	vmserrors.VAX_BADIMMLIT:     "OPRNDSYNX",
	vmserrors.VAX_BADMASK:       "ILLMASKBIT",
	vmserrors.VAX_BADMASKENTRY:  "ILLMASKBIT",
	vmserrors.VAX_BADMODE:       "ILLMODE",
	vmserrors.VAX_BADOPCODE:     "UNRECSTMT",
	vmserrors.VAX_BADOPERANDS:   "NOTENUFOPR",
	vmserrors.VAX_BADOPERATOR:   "BADLEXARG",
	vmserrors.VAX_BADPACKED:     "NOTDECSTRG",
	vmserrors.VAX_BADREG:        "REGOPSYNX",
	vmserrors.VAX_BADSHORTFLOAT: "FLTPNTSYNX",
	vmserrors.VAX_BADSHORTLIT:   "OPRNDSYNX",
	vmserrors.VAX_BADSTRING:     "ILLASCARG",
	vmserrors.VAX_BRANCHRANGE:   "BRDESTRANG",
	vmserrors.VAX_CHARTOOLONG:   "ASCTOOLONG",
	vmserrors.VAX_CONDDEPTH:     "IFLEVLXCED",
	vmserrors.VAX_DATARANGE:     "DATATRUNC",
	vmserrors.VAX_DIRSYNX:       "DIRSYNX",
	vmserrors.VAX_DIVZERO:       "DIVBYZERO",
	vmserrors.VAX_DIVZEROWARN:   "DIVBYZERO",
	vmserrors.VAX_DUPSYM:        "MULDEFLBL",
	vmserrors.VAX_ENDMNAME:      "ENDWRNGMAC",
	vmserrors.VAX_ENTRYMASK:     "ILLMASKBIT",
	vmserrors.VAX_FLOATHERE:     "FLTPNTSYNX",
	vmserrors.VAX_FLOATRANGE:    "FLTPNTSYNX",
	vmserrors.VAX_FWDBYTE:       "DATATRUNC",
	vmserrors.VAX_FWDOPERATOR:   "ILLEXPR",
	vmserrors.VAX_FWDWORD:       "DATATRUNC",
	vmserrors.VAX_GENERR:        "GENERR",
	vmserrors.VAX_GENWRN:        "GENWRN",
	vmserrors.VAX_ILLEXPR:       "ILLEXPR",
	vmserrors.VAX_INCOMPLETENUM: "ILLEXPR",
	vmserrors.VAX_INDEXBASE:     "ILLINDXREG",
	vmserrors.VAX_INDEXNEST:     "MAYNOTINDX",
	vmserrors.VAX_LIBRARY:       "MLBOPNERR",
	vmserrors.VAX_LIBREAD:       "MACLBFMTER",
	vmserrors.VAX_MACRONAME:     "ILLMACNAM",
	vmserrors.VAX_MODEACCESS:    "ILLMODE",
	vmserrors.VAX_NEEDHASH:      "OPRNDSYNX",
	vmserrors.VAX_NOCLOSE:       "UNTERMARG",
	vmserrors.VAX_NOCOND:        "NOTINANIF",
	vmserrors.VAX_NOENDC:        "UNTERMCOND",
	vmserrors.VAX_NOTINDEF:      "NOTINMACRO",
	vmserrors.VAX_NOTINMACRO:    "NOTINMACRO",
	vmserrors.VAX_NOTINREPEAT:   "NOTINMACRO",
	vmserrors.VAX_NOTMACRO:      "UNRECSTMT",
	vmserrors.VAX_OUTOFPHASE:    "SYMOUTPHAS",
	vmserrors.VAX_PCREGISTER:    "ILLREGHERE",
	vmserrors.VAX_PSECTATTR:     "NOTPSECOPT",
	vmserrors.VAX_PSECTCONFLICT: "PSECOPCNFL",
	vmserrors.VAX_RELEXPR:       "SYMNOTABS",
	vmserrors.VAX_SHORTRANGE:    "DATATRUNC",
	vmserrors.VAX_TOOMANYPSECTS: "TOOMNYPSEC",
	vmserrors.VAX_TOOMNYARGS:    "TOOMNYARGS",
	vmserrors.VAX_UNDEFMACRO:    "CANTLOCMAC",
	vmserrors.VAX_UNDEFSYM:      "UNDEFSYM",
	vmserrors.VAX_UNTERMCHAR:    "UNTERMARG",
}

// macroWarnings are MACRO's messages of warning severity (W), from
// MACRO's message definitions; the rest are errors (E). The severity
// letter is only what the message line shows: whether the summary counts
// a message as an error or a warning is govax's own choice, made where
// it's reported (real MACRO counts DATATRUNC as an error, errors.lis's
// summary shows).
var macroWarnings = map[string]bool{
	"ABSDATA":    true,
	"DATATRUNC":  true,
	"DIVBYZERO":  true,
	"EXPOVR32":   true,
	"GENWRN":     true,
	"ILLSYMLEN":  true,
	"MISSINGEND": true,
}

// macroNoColumn are the messages real MACRO lists without a "!": the
// ones it finds in its second pass, once values are known.
var macroNoColumn = map[string]bool{
	"BRDESTRANG": true,
	"DATATRUNC":  true,
	"GENERR":     true,
	"GENWRN":     true,
	"SYMOUTPHAS": true,
}

// macroTexts is MACRO's message text for each of its identifiers.
var macroTexts = func() map[string]string {
	texts := map[string]string{}

	for _, m := range vmsdef.Messages {
		if m.Facility == "MACRO" {
			texts[m.Ident] = m.Text
		}
	}

	return texts
}()

// macroMessage is a note as real MACRO reports it: its message line,
// without the "!", and whether a "!" marks the column.
type macroMessage struct {
	text   string
	column bool
}

// macroMessageFor returns how real MACRO reports the note n of line l.
func (l *listLine) macroMessageFor(n listNote) macroMessage {
	e, ok := innermostError(n.err)
	if !ok {
		return macroMessage{text: "%" + n.err.Error()}
	}

	ident := l.macroIdent(e)
	text, known := macroTexts[ident]

	if ident == "" || !known {
		return macroMessage{text: "%" + e.Error()}
	}

	severity := "E"
	if macroWarnings[ident] {
		severity = "W"
	}

	switch ident {
	case "GENERR", "GENWRN":
		// The directive's own message follows MACRO's words, as it was
		// written ("Generated ERROR:  ERRORS: a message").
		if len(e.Arguments) > 0 {
			if s, ok := e.Arguments[0].(string); ok {
				text += s
			}
		}

	default:
		// A message that names what it's about ("undefined symbol
		// !AD") is cut before the name, which govax's error may not
		// carry in the same form.
		if k := strings.IndexByte(text, '!'); k >= 0 {
			text = strings.TrimRight(text[:k], " ")
		}
	}

	if text != "" {
		text = strings.ToUpper(text[:1]) + text[1:]
	}

	return macroMessage{
		text:   "%MACRO-" + severity + "-" + ident + ", " + text,
		column: !macroNoColumn[ident] && n.column >= 0,
	}
}

// macroIdent returns MACRO's identifier for e, an error of l's
// statement, or "" if it has none. A few of govax's errors are more than
// one of MACRO's, told apart by the statement: unexpected text after an
// instruction's operands is one operand too many, and a bad keyword is a
// bad listing option in a listing directive.
func (l *listLine) macroIdent(e vmserrors.VMSError) string {
	switch e.Status {
	case vmserrors.VAX_EXTRATEXT:
		if l.instruction {
			return "TOOMNYOPND"
		}

		return "DIRSYNX"

	case vmserrors.VAX_BADKEYWORD:
		switch l.op {
		case ".SHOW", ".NOSHOW", ".LIST", ".NLIST":
			return "NOTLGLISOP"
		case ".ENABLE", ".DISABLE", ".DSABL", ".ENABL":
			return "NOTENABOPT"
		}

		return "DIRSYNX"

	case vmserrors.VAX_PSECTSTACK:
		if len(e.Arguments) > 0 && e.Arguments[0] == "empty" {
			return "PSECBUFUND"
		}

		return "PSECBUFOVF"
	}

	return macroIdents[e.Status]
}

// innermostError returns the VMS error err reports: the error an
// instruction's operand raised, rather than the error that names the
// operand (VAX-E-OPERANDERR), and the error itself rather than the
// source stack's location.
func innermostError(err error) (vmserrors.VMSError, bool) {
	var e vmserrors.VMSError
	if !errors.As(err, &e) {
		return e, false
	}

	for e.Status == vmserrors.VAX_OPERANDERR {
		var inner vmserrors.VMSError
		if e.Cause == nil || !errors.As(e.Cause, &inner) {
			break
		}

		e = inner
	}

	return e, true
}
