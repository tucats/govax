package cpu

import (
	"errors"
	"fmt"
)

// ErrServiceWait is what SystemService returns when the service has put
// the process in a wait state that isn't satisfied yet ($WAITFR on a clear
// event flag, docs/PHASE-26.md). The XFC handler then leaves R0 alone and
// backs PC up to the XFC instruction, so the next Step calls the service
// again: the process waits in emulated time, and interrupts (the interval
// timer, the only asynchronous source today) are still delivered between
// attempts, as they would be to a waiting VMS process. Console attention
// and the instruction/time limits still stop it.
var ErrServiceWait = errors.New("cpu: system service waiting")

// SystemServices is the hook interface Engine's XFC handler (opcode 0xFC,
// internal/cpu/xfc.go) delegates to for every selector that needs state
// outside internal/cpu: console I/O, DCL parsing, and RTL SYS$/LIB$ dispatch.
// See docs/PHASE-07.md's XFC deferral and docs/PHASE-10.md's scope note.
//
// internal/console.Console implements this, delegating the RTL-specific
// methods (SystemService/Shim) to an embedded *corevms.Environment — kept as an
// interface here, rather than internal/cpu importing internal/console or
// internal/rtl directly, so internal/cpu stays independent of both (matching
// this project's layering: internal/cpu doesn't know about the console or
// RTL phases built on top of it).
type SystemServices interface {
	// ConsoleWriteByte/ConsoleReadByte implement XFC$CONSOLE_WRITE (R0's low
	// byte is the byte to write) and XFC$CONSOLE_READ (returns the byte read).
	ConsoleWriteByte(b byte)
	ConsoleReadByte() byte

	// ConsoleWrite implements XFC$CONSOLE_PUT: write a whole string (already
	// read out of emulated memory) to the console in one call.
	ConsoleWrite(p []byte)

	// ConsoleCommand implements XFC$CONSOLE_CMD: dispatch cmd as a console
	// command line, returning its status code.
	ConsoleCommand(cmd string) uint32

	// DCL* implement XFC$DCL's four subfunctions (selected by R0 == 1..4).
	// DCLGetString reports ok=false when it has nowhere to write the result
	// (matching emul_xfc.c's own "EXE$DCLSTRING buffer not set up" no-op
	// path) rather than an address of 0, which is itself a valid VAX address.
	DCLPresent(r1, r2 uint32) uint32
	DCLGetKeyword(r1, r2, r3 uint32) uint32
	DCLGetString(r1, r2 uint32) (addr uint32, ok bool)
	DCLGetInteger(r1, r2 uint32) uint32

	// SystemService implements XFC$P1VECTOR: dispatch a SYS$ system-service
	// call whose calling instruction is at pc (vax.PC - 4, matching
	// call_service's own addressing — the XFC handler passes the P1-vector
	// stub's own address, not the post-XFC PC). Returns the R0 status value
	// this call should leave. handled is false when pc doesn't correspond to
	// any known service, matching call_service's own "non-existent P1
	// vector" halt path — the caller (emulXfc) turns that into a fault
	// rather than halting the machine outright. ErrServiceWait means "not
	// done yet, call again" (see its own doc comment).
	SystemService(pc uint32) (r0 uint32, handled bool, err error)

	// Shim implements XFC$SHIM: dispatch a LIB$/CRTL shim call by numeric
	// code (already in R0 when the XFC executes). handled is false for an
	// unregistered code, matching shim()'s own "Unimplemented SHIM
	// invocation" path.
	Shim(code uint32) (r0 uint32, handled bool, err error)

	// RequestQuit implements the vax.console.running = 0 half of
	// XFC$QUIT_EMULATION (emul_xfc.c case 0x78) -- the half that reaches
	// past the CPU's own halt into the console's command loop, asking it
	// to stop entirely rather than just fall back to the "VAX>" prompt.
	// The halt itself is reported the ordinary way, via ErrHalted (see
	// xfc.go's emulXfc), matching console_exec.c treating VAX_USERHALT
	// (this opcode's halt reason) identically to a plain VAX_HALT.
	RequestQuit()
}

// ServiceCall is what SystemService returns when the service needs a
// guest procedure called on its behalf before it can go on: $EXIT calling
// an exit handler (docs/PHASE-26.md subtask 19). The XFC handler calls
// Routine as CALLG ArgList, Routine would, with the XFC itself as the
// return address. So when the procedure executes RET, the XFC runs
// again, calling the service again, which picks up where it left off
// (the RTL keeps track of how far it got). R0 is left alone.
type ServiceCall struct {
	Routine uint32 // the procedure's entry mask address
	ArgList uint32 // its argument list (a count longword, then arguments)
}

func (c *ServiceCall) Error() string {
	return fmt.Sprintf("cpu: system service calls %08X", c.Routine)
}

// ErrImageExit is what SystemService returns when the running image has
// finished exiting ($EXIT, after its exit handlers). The XFC handler sets
// R0 to the exit status the service returned, and ends the image by
// returning from the console's own call frame (see exitImage).
var ErrImageExit = errors.New("cpu: image exit")
