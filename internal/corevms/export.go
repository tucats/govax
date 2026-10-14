package corevms

import (
	"fmt"

	"github.com/tucats/govax/internal/registers"
	"github.com/tucats/govax/internal/vm"
)

// What an RTL package needs from the Environment (docs/PHASE-34.md,
// subtask 10). The routines of a shareable image like LIBRTL.EXE live in a
// package of their own (internal/librtl); the process they run in -- its
// CPU and memory, its call frames and condition dispatcher, its heap --
// stays here, and these methods are that package's way in. Each is the
// smallest operation a documented routine is built from, not the routine
// itself.

// CPU is the processor the process runs on.
func (env *Environment) CPU() *registers.CPU { return env.cpu }

// Memory is the process's address space.
func (env *Environment) Memory() *vm.Memory { return env.mem }

// Shims is the table of XFC$SHIM routines: an RTL package registers its
// routines here (the console does it for each new System). Because
// Environment embeds *System, env.Shims() reaches it too.
func (sys *System) Shims() *ShimTable { return sys.shims }

// Signal signals the condition whose argument list (condition value,
// argument count, FAO arguments) is argv, from a running shim, as LIB$SIGNAL
// does -- or LIB$STOP, with stop, which forces SEVERE and can't be
// continued. See signal.go.
func (env *Environment) Signal(argv []uint32, stop bool) (uint32, error) {
	if stop {
		return env.signal(kindStop, argv, 0)
	}

	return env.signal(kindSignal, argv, 0)
}

// SignalFailure signals the condition argv describes, as Signal does, for
// a routine that fails by signaling rather than by returning a status: an
// access violation in its own code, say, which on VMS is the hardware's
// exception. If a handler continues the condition, the routine returns
// status (in R0) to its caller.
func (env *Environment) SignalFailure(argv []uint32, status uint32) (uint32, error) {
	return env.signal(kindSignal, argv, status)
}

// SetCallerHandler makes handler (0 for none) the condition handler of the
// frame of the procedure that called the running shim, returning the one
// it replaces. name is the routine's, for errors.
func (env *Environment) SetCallerHandler(name string, handler uint32) (uint32, error) {
	return env.setCallerHandler(name, handler)
}

// ConditionToReturn turns the condition being handled into a return
// status, as LIB$SIG_TO_RET does: it stores the condition value as the
// mechanism array's R0 and unwinds to the caller of the procedure that
// established the handler. sig and mech are the handler's signal and
// mechanism array addresses. It returns SS$_NORMAL, SS$_NOSIGNAL when no
// condition is being handled, or $UNWIND's error status. See unwind.go.
func (env *Environment) ConditionToReturn(sig, mech uint32) (uint32, error) {
	return env.conditionToReturn(sig, mech)
}

// AllocateVM allocates size bytes of the image's heap for LIBRTL,
// returning the block's address, or 0 when P0 has no room. The heap is
// the one DECC$MALLOC uses (memory.go).
func (env *Environment) AllocateVM(size uint32) (uint32, error) {
	return env.allocate(size), nil
}

// FreeVM frees the heap block that starts at addr, reporting false if no
// block starts there.
func (env *Environment) FreeVM(addr uint32) bool {
	return env.release(addr)
}

// GetVMPages allocates n contiguous pages from the image's page pool
// for LIB$GET_VM_PAGE, returning the first one's address. The pages are
// P0's, so user-mode read/write whatever mode asks (the 5.0 LIB$ manual),
// and demand-zero until first touched. The error is vmheap.ErrBadSize
// for n of 0, or vmheap.ErrNoMemory when P0 has no room (memory.go).
func (env *Environment) GetVMPages(n uint32) (uint32, error) {
	return env.pagePool().Allocate(n)
}

// FreeVMPages returns n pages starting at addr to the image's page pool
// for LIB$FREE_VM_PAGE. The error is vmheap.ErrBadSize for n of 0, or
// vmheap.ErrBadAddress for pages that aren't all allocated (or an addr
// not on a page boundary, or a zone's pages); nothing is freed then.
func (env *Environment) FreeVMPages(n, addr uint32) error {
	return env.pagePool().Free(n, addr)
}

// BlockSize returns the size of the heap block that starts at addr, as
// allocated (rounded up from what was asked for), and whether a block
// starts there. A dynamic string's storage is kept when it's big enough
// for a new value (LIB$SGET1_DD), so the RTL asks.
func (env *Environment) BlockSize(addr uint32) (uint32, bool) {
	if env.heapZone == nil {
		return 0, false
	}

	return env.heapZone.BlockSize(addr)
}

// WriteOutput writes s to the process's output stream (the console
// terminal, which SYS$OUTPUT names), as it is: a caller that wants a line
// ends s with a newline.
func (env *Environment) WriteOutput(s string) { env.writeConsole(s) }

// ReadInputLine writes prompt to the process's output stream, then reads
// a line of at most maxLen bytes from its input stream (SYS$INPUT, the
// console terminal), as a terminal read does (readTerminalLine), without
// the line's terminator. ok is false at the end of the input: CTRL/Z
// typed on an empty line, or the end of the host's input. With the
// scheduler on, err is ErrWait while the line hasn't been typed, or
// another process's read comes first: the caller is called again
// (terminal.go); the prompt is written only once.
func (env *Environment) ReadInputLine(prompt string, maxLen int) (line string, ok bool, err error) {
	if err := env.awaitTerminal(maxLen, prompt, lineEnd); err != nil {
		return "", false, err
	}

	line, ok = readTerminalLine(env, maxLen)
	env.terminalDone()
	env.countIO(false)

	return line, ok, nil
}

// AwaitTerminal and TerminalDone are awaitTerminal and terminalDone
// for a line read (lineEnd) by RMS's $GET of the terminal
// (rms.Context's hooks).
func (env *Environment) AwaitTerminal(maxLen int, prompt string) error {
	return env.awaitTerminal(maxLen, prompt, lineEnd)
}

func (env *Environment) TerminalDone() { env.terminalDone() }

// StatusText is the message line for a status value as the system shows
// it, such as "%SYSTEM-S-NORMAL, normal successful completion". A status
// with no message of its own is shown by its number, as $GETMSG shows it.
func (env *Environment) StatusText(status uint32) string {
	// STS$M_INHIB_MSG only says the message was already shown; it isn't
	// part of the message's identity.
	status &^= stsInhibitMsg

	lines := env.formatMessageVector([]uint32{status, 0}, nil, defaultMessageFlags, "")
	if len(lines) == 0 {
		return fmt.Sprintf("%%SYSTEM-?-NOMSG, message number %08X", status)
	}

	return lines[0]
}

// ParseTime is $BINTIM's conversion of text to a VMS time, the fields an
// absolute time leaves out taken from the process's clock. ok is false
// for text $BINTIM would reject (SS$_IVTIME). The debugger's
// DEPOSIT/DATE_TIME uses it.
func (env *Environment) ParseTime(text string) (uint64, bool) {
	return parseVMSTime(text, env.Clock())
}

// VectorTime is the VMS time that $NUMTIM's seven numbers (year, month,
// day, hour, minute, second, hundredths) stand for, ok false for a field
// out of range: LIB$CVT_VECTIM's conversion (see vectorTime).
func VectorTime(fields [7]uint16) (uint64, bool) { return vectorTime(fields) }

// CallService runs the system service called name ("SYS$FAO") with the
// argument list argv, as a program's CALLS would reach it, and returns
// its status: the RTL's jacket routines (LIB$SYS_FAO, ...) call the
// service this way, as VMS's call it. The argument count isn't checked
// first (argcount.go): a jacket passes a whole list. It panics for a
// service govax doesn't implement, a mistake in the caller.
func (env *Environment) CallService(name string, argv []uint32) (uint32, error) {
	fn, ok := env.services.Lookup(name)
	if !ok {
		panic("corevms: CallService of unimplemented " + name)
	}

	return fn(env, argv)
}

// ImageData returns the data an RTL package keeps for the running image
// under key, made by calling create the first time it's asked for after
// the image starts. On VMS a shareable image's own data (LIBRTL's event
// flag and LUN pools, its static timer block) is in the image's P0
// pages, so each image starts with it fresh; here ImageRundown forgets
// it. key should be a value of a type private to the package, so no two
// packages' keys collide.
func (env *Environment) ImageData(key any, create func() any) any {
	if v, ok := env.imageData[key]; ok {
		return v
	}

	if env.imageData == nil {
		env.imageData = map[any]any{}
	}

	v := create()
	env.imageData[key] = v

	return v
}
