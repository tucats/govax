package librtl

import (
	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/vmsdef"
)

var (
	libInpStrTru = vmsdef.LibrarySymbols["LIB$_INPSTRTRU"]
	rmsEOF       = vmsdef.Symbols["RMS$_EOF"]
)

// maxPromptLength is the longest prompt LIB$GET_FOREIGN and LIB$GET_INPUT
// write.
const maxPromptLength = 255

// maxInputLength is the longest line LIB$GET_FOREIGN (when it prompts)
// and LIB$GET_INPUT read: a string descriptor's length is a word.
const maxInputLength = maxStringLength

// libGetForeign is LIB$GET_FOREIGN (RTL Library manual):
//
//	LIB$GET_FOREIGN resultant-string [,prompt-string] [,resultant-length] [,flags]
//
// It returns the text of the foreign command that ran the image, after
// the command's verb (Environment.CommandLine). If there's none, or bit 0
// of the longword at flags is set, and a prompt-string is given, it
// prompts for the text on SYS$INPUT instead, as LIB$GET_INPUT does; it
// then sets flags to 1, so that a program calling it in a loop prompts
// from the second call on. The text goes into resultant-string (see
// writeString), and its length, as a word, into resultant-length.
//
// It returns SS$_NORMAL; LIB$_INPSTRTRU when the text didn't fit a
// fixed-length resultant-string; RMS$_EOF at the end of the input when it
// prompted; LIB$_INVARG with no resultant-string; LIB$_INVSTRDES when a
// descriptor's class isn't a string's; LIB$_INSVIRMEM; or SS$_ACCVIO.
//
// Unconfirmed against VMS (docs/PHASE-34.md, 2026-10-04): that flags is
// set to 1 whether or not the routine prompted, and that a prompted line
// is returned as typed, not uppercased.
func libGetForeign(env *corevms.Environment, argv []uint32) (uint32, error) {
	mem, cpu := env.Memory(), env.CPU()
	result, prompt, lenAddr, flagsAddr := arg(argv, 0), arg(argv, 1), arg(argv, 2), arg(argv, 3)

	if result == 0 {
		return libInvArg, nil
	}

	force := false

	if flagsAddr != 0 {
		flags, err := mem.LoadLongword(cpu, flagsAddr)
		if err != nil {
			return ssAccVio, nil
		}

		force = flags&1 != 0
	}

	text := env.CommandLine
	status := ssNormal

	if (text == "" || force) && prompt != 0 {
		p, err := readPrompt(env, prompt)
		if err != nil {
			return stringStatus(err), nil
		}

		line, ok, err := env.ReadInputLine(p, maxInputLength)
		if err != nil {
			return 0, err // waiting for the line: called again
		}

		if !ok {
			status = rmsEOF
		}

		text = line
	}

	if flagsAddr != 0 {
		if err := mem.StoreLongword(cpu, flagsAddr, 1); err != nil {
			return ssAccVio, nil
		}
	}

	stored, truncated, err := writeString(env, result, text)
	if err != nil {
		return stringStatus(err), nil
	}

	if lenAddr != 0 {
		if err := mem.StoreWord(cpu, lenAddr, uint16(stored)); err != nil {
			return ssAccVio, nil
		}
	}

	if truncated && status == ssNormal {
		status = libInpStrTru
	}

	return status, nil
}

// readPrompt reads a prompt-string argument: at most maxPromptLength of
// its characters are written.
func readPrompt(env *corevms.Environment, addr uint32) (string, error) {
	p, err := readString(env, addr)
	if len(p) > maxPromptLength {
		p = p[:maxPromptLength]
	}

	return p, err
}
