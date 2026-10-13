package librtl

import (
	"errors"

	"github.com/tucats/govax/internal/corevms"
	"github.com/tucats/govax/internal/vmsdef"
)

// libGetInput is LIB$GET_INPUT (RTL Library manual):
//
//	LIB$GET_INPUT get-string [,prompt-string] [,resultant-length]
//
// It reads a line from SYS$INPUT (the console terminal), first writing
// prompt-string, if it's given, as the read's prompt. The line, without
// its terminator, goes into get-string (see writeString), and its length,
// as a word, into resultant-length. A line ends as a terminal read's
// does: at RETURN, or at CTRL/Z typed after some text
// (corevms.Environment.ReadInputLine).
//
// It returns SS$_NORMAL; LIB$_INPSTRTRU when the line didn't fit a
// fixed-length get-string; RMS$_EOF at the end of the input (CTRL/Z on an
// empty line); LIB$_INVARG with no get-string, or one whose descriptor's
// class isn't a string's or is 0 (the manual, LIB-213, says so of this
// routine, though other routines read class Z as S); LIB$_INVSTRDES for
// a prompt-string's; or LIB$_INSVIRMEM. An argument it can't read or
// write is an access violation, signaled (memoryFault).
//
// Unconfirmed against VMS: that at the end of the input get-string and
// resultant-length are left as they were; and SYS$INPUT is always the
// terminal (a logical name redirecting it to a file isn't followed).
func libGetInput(env *corevms.Environment, argv []uint32) (uint32, error) {
	return getLine(env, argv, env.ReadInputLine)
}

// libGetCommand is LIB$GET_COMMAND (RTL Library manual, LIB-199):
//
//	LIB$GET_COMMAND resultant-string [,prompt-string] [,resultant-length]
//
// It is LIB$GET_INPUT reading SYS$COMMAND rather than SYS$INPUT: "used
// when a program needs input from some source other than the current
// input stream". The two differ while DCL runs a command procedure,
// whose data lines are the image's SYS$INPUT and whose terminal is its
// SYS$COMMAND; the read goes to the terminal then
// (corevms.Environment.ReadCommandLine). Its arguments, results, and
// statuses are GET_INPUT's.
//
// Unconfirmed against VMS: a subprocess whose input is a file or a
// mailbox (SPAWN/INPUT, a LOGINOUT process) reads that input, its
// SYS$COMMAND in govax; and the prompt is written whatever SYS$COMMAND
// is (the manual: only to a terminal), as GET_INPUT's is.
//
// A process with no CLI (one $CREPRC made to run an image) has no
// SYS$COMMAND: the routine returns RMS$_FNF, with no prompt and nothing
// written (VMS V7.1, testdata/lib57).
func libGetCommand(env *corevms.Environment, argv []uint32) (uint32, error) {
	if env.CLI == nil {
		return vmsdef.Symbols["RMS$_FNF"], nil
	}

	return getLine(env, argv, env.ReadCommandLine)
}

// getLine is LIB$GET_INPUT's and LIB$GET_COMMAND's work: argv's
// arguments checked, the prompt read, a line read by read (from
// SYS$INPUT or SYS$COMMAND), and the line and its length written back.
func getLine(env *corevms.Environment, argv []uint32,
	read func(prompt string, maxLen int) (string, bool, error),
) (uint32, error) {
	result, prompt, lenAddr := arg(argv, 0), arg(argv, 1), arg(argv, 2)

	if result == 0 {
		return libInvArg, nil
	}

	switch d, err := readDescriptor(env, result); {
	case errors.Is(err, errInvStrDes), err == nil && d.class == dscClassZ:
		return libInvArg, nil
	case err != nil:
		return stringFailure(env, err)
	}

	p := ""

	if prompt != 0 {
		var err error

		if p, err = readPrompt(env, prompt); err != nil {
			return stringFailure(env, err)
		}
	}

	line, ok, err := read(p, maxInputLength)
	if err != nil {
		return 0, err // waiting for the line: called again
	}

	if !ok {
		return rmsEOF, nil
	}

	stored, truncated, err := writeString(env, result, line)
	if err != nil {
		return stringFailure(env, err)
	}

	if lenAddr != 0 {
		if err := env.Memory().StoreWord(env.CPU(), lenAddr, uint16(stored)); err != nil {
			return memoryFault(env, writing(err))
		}
	}

	if truncated {
		return libInpStrTru, nil
	}

	return ssNormal, nil
}
