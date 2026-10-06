package librtl

import "github.com/tucats/govax/internal/corevms"

// libGetInput is LIB$GET_INPUT (RTL Library manual):
//
//	LIB$GET_INPUT get-string [,prompt-string] [,resultant-length]
//
// It reads a line from SYS$INPUT (the console terminal), first writing
// prompt-string, if it's given, as the read's prompt. The line, without
// its terminator, goes into get-string (see storeString), and its length,
// as a word, into resultant-length. A line ends as a terminal read's
// does: at RETURN, or at CTRL/Z typed after some text
// (corevms.Environment.ReadInputLine).
//
// It returns SS$_NORMAL; LIB$_INPSTRTRU when the line didn't fit a
// fixed-length get-string; RMS$_EOF at the end of the input (CTRL/Z on an
// empty line); LIB$_INVARG with no get-string; or SS$_ACCVIO.
//
// Unconfirmed against VMS: that at the end of the input get-string and
// resultant-length are left as they were; and SYS$INPUT is always the
// terminal (a logical name redirecting it to a file isn't followed).
func libGetInput(env *corevms.Environment, argv []uint32) (uint32, error) {
	result, prompt, lenAddr := arg(argv, 0), arg(argv, 1), arg(argv, 2)

	if result == 0 {
		return libInvArg, nil
	}

	p := ""

	if prompt != 0 {
		var err error

		if p, _, err = env.StringDescriptor(prompt, maxPromptLength); err != nil {
			return ssAccVio, nil
		}
	}

	line, ok := env.ReadInputLine(p, maxInputLength)
	if !ok {
		return rmsEOF, nil
	}

	stored, truncated, err := storeString(env, result, line)
	if err != nil {
		return ssAccVio, nil
	}

	if lenAddr != 0 {
		if err := env.Memory().StoreWord(env.CPU(), lenAddr, uint16(stored)); err != nil {
			return ssAccVio, nil
		}
	}

	if truncated {
		return libInpStrTru, nil
	}

	return ssNormal, nil
}
