package corevms

import (
	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/vmsdef"
)

// Services govax lacks (docs/PHASE-83 - FORTRAN and VMS.md, Decision 6). The
// P1 vector has an entry for every service of VMS's, so a program calling one
// govax doesn't have links and runs up to that call. The call signals as a
// missing LIB$ routine's does (librtl/missing.go): SS$_UNSUPPORTED, followed by
// GOVAX-E-NOTIMPL with the service's name.
//
//	%SYSTEM-E-UNSUPPORTED, unsupported operation or function
//	-GOVAX-E-NOTIMPL, routine SYS$GETCHN is not implemented in govax
//
// The program's handlers see it as any other condition. It is an error, not
// severe, so with no handler to take it the program goes on, the call
// returning SS$_UNSUPPORTED.

// What a class S descriptor of text has in its first longword, past the length
// (the VAX procedure calling standard).
const (
	dscClassS = 1
	dscDTypeT = 14
)

// missingService signals that the service name is missing. The call frame the
// signal is raised in is the vector entry's, and its RET is the continuation.
func (env *Environment) missingService(name string) (uint32, error) {
	desc, err := env.StackString(name)
	if err != nil {
		// No stack to put the name on, which is what the program's next
		// call would find too: the status alone says what happened.
		return ssUnsupported, nil
	}

	return env.SignalFailure([]uint32{ssUnsupported, vmsdef.GovaxNotImplemented, 1, desc}, ssUnsupported)
}

// StackString puts s, and a class S descriptor for it, on the stack below SP,
// moves SP below them, and returns the descriptor's address: a string to pass
// by descriptor in a signal's arguments, as a VAX routine would build one in
// its own frame. The space is the routine's call frame's, so its RET gives it
// back.
func (env *Environment) StackString(s string) (uint32, error) {
	mem, cpu := env.mem, env.cpu
	size := (uint32(len(s)) + 3) &^ 3
	desc := cpu.GPR(registers.SP) - size - 8
	text := desc + 8

	for i := range len(s) {
		if err := mem.StoreByte(cpu, text+uint32(i), s[i]); err != nil {
			return 0, err
		}
	}

	if err := mem.StoreLongword(cpu, desc, uint32(len(s))|dscClassS<<24|dscDTypeT<<16); err != nil {
		return 0, err
	}

	if err := mem.StoreLongword(cpu, desc+4, text); err != nil {
		return 0, err
	}

	cpu.SetGPR(registers.SP, desc)

	return desc, nil
}
