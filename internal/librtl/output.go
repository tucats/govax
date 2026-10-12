package librtl

import "github.com/tucats/govax/internal/corevms"

// libPutOutput is LIB$PUT_OUTPUT (RTL Library manual): it writes the
// string its one argument, a string descriptor, describes to SYS$OUTPUT
// as one record. On the terminal a record is a line, so the text is
// followed by a newline; an empty string writes an empty line. When
// SYS$OUTPUT is a mailbox, the record is a message (docs/PHASE-46.md,
// subtask 6), and a full mailbox makes the caller wait for room. It
// returns the write's status (SS$_NORMAL on the terminal), LIB$_INVSTRDES
// when the descriptor's class isn't a string's, or SS$_ACCVIO when the
// descriptor or its text can't be read.
//
// This replaces the microkernel's own LIB$PUT_OUTPUT (kernel.asm, now
// EXE$PUT_OUTPUT), which wrote a character at a time through the console
// transmit interrupt.
func libPutOutput(env *corevms.Environment, argv []uint32) (uint32, error) {
	s, err := readString(env, arg(argv, 0))
	if err != nil {
		return stringStatus(err), nil
	}

	return env.PutOutput(s)
}
