package corevms

import (
	"fmt"

	"github.com/tucats/govax/internal/vax"
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
func (env *Environment) CPU() *vax.CPU { return env.cpu }

// Memory is the process's address space.
func (env *Environment) Memory() *vm.Memory { return env.mem }

// Shims is the table of XFC$SHIM routines: an RTL package registers its
// routines here (the console does it for each new System). Because
// Environment embeds *System, env.Shims() reaches it too.
func (sys *System) Shims() *ShimTable { return sys.shims }

// StringDescriptor reads the string a descriptor at addr describes. ok is
// false when it's longer than maxLen; err when memory can't be read.
func (env *Environment) StringDescriptor(addr uint32, maxLen int) (s string, ok bool, err error) {
	return strGet(env, addr, maxLen)
}

// Signal signals the condition whose argument list (condition value,
// argument count, FAO arguments) is argv, from a running shim, as LIB$SIGNAL
// does -- or LIB$STOP, with stop, which forces SEVERE and can't be
// continued. See signal.go.
func (env *Environment) Signal(argv []uint32, stop bool) (uint32, error) {
	if stop {
		return env.signal(kindStop, argv)
	}

	return env.signal(kindSignal, argv)
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

// AllocateVM allocates size bytes of the process's heap for LIBRTL, tagged
// with zone (0 for none), returning the block's address, or 0 when there's
// no room. The heap is the one DECC$MALLOC uses (memory.go).
func (env *Environment) AllocateVM(size, zone uint32) (uint32, error) {
	return shimDeccMalloc(env, []uint32{size, libvmLibrtl, zone})
}

// FreeVM frees the heap block at addr, reporting false if there's none.
func (env *Environment) FreeVM(addr uint32) bool {
	return env.freeBlock(addr)
}

// FreeVMZone frees every heap block tagged with zone.
func (env *Environment) FreeVMZone(zone uint32) {
	var toFree []uint32

	for _, p := range env.memAllocated {
		if p.zone == zone {
			toFree = append(toFree, p.addr)
		}
	}

	for _, addr := range toFree {
		env.freeBlock(addr)
	}
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
