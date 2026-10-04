package librtl

import "github.com/tucats/govax/internal/corevms"

// maxOutputRecord is the longest record LIB$PUT_OUTPUT writes: a
// descriptor's length is a word.
const maxOutputRecord = 65535

// libPutOutput is LIB$PUT_OUTPUT (RTL Library manual): it writes the
// string its one argument, a string descriptor, describes to SYS$OUTPUT
// as one record. On the terminal a record is a line, so the text is
// followed by a newline; an empty string writes an empty line. It returns
// SS$_NORMAL, or SS$_ACCVIO when the descriptor or its text can't be read.
//
// This replaces the microkernel's own LIB$PUT_OUTPUT (kernel.asm, now
// EXE$PUT_OUTPUT), which wrote a character at a time through the console
// transmit interrupt.
func libPutOutput(env *corevms.Environment, argv []uint32) (uint32, error) {
	s, _, err := env.StringDescriptor(arg(argv, 0), maxOutputRecord)
	if err != nil {
		return ssAccVio, nil
	}

	env.WriteOutput(s + "\n")

	return ssNormal, nil
}
