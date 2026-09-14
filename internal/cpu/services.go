package cpu

// SystemServices is the hook interface Engine's XFC handler (opcode 0xFC,
// internal/cpu/xfc.go) delegates to for every selector that needs state
// outside internal/cpu: console I/O, DCL parsing, and RTL SYS$/LIB$ dispatch.
// See docs/PHASE-07.md's XFC deferral and docs/PHASE-10.md's scope note.
//
// internal/console.Console implements this, delegating the RTL-specific
// methods (SystemService/Shim) to an embedded *rtl.Environment — kept as an
// interface here, rather than internal/cpu importing internal/console or
// internal/rtl directly, so internal/cpu stays independent of both (matching
// this project's layering: internal/cpu doesn't know about the console or
// RTL phases built on top of it).
type SystemServices interface {
	// ConsoleWriteByte/ConsoleReadByte implement XFC$CONSOLE_WRITE (R0's low
	// byte is the byte to write) and XFC$CONSOLE_READ (returns the byte read).
	ConsoleWriteByte(b byte)
	ConsoleReadByte() byte

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
	// rather than halting the machine outright.
	SystemService(pc uint32) (r0 uint32, handled bool, err error)

	// Shim implements XFC$SHIM: dispatch a LIB$/CRTL shim call by numeric
	// code (already in R0 when the XFC executes). handled is false for an
	// unregistered code, matching shim()'s own "Unimplemented SHIM
	// invocation" path.
	Shim(code uint32) (r0 uint32, handled bool, err error)
}
