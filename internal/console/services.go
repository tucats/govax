package console

import (
	"github.com/tucats/govax/internal/cpu"
)

// This file makes Machine implement cpu.SystemServices (internal/cpu/
// services.go), the hook Engine's XFC opcode handler delegates to. SYS$/
// LIB$ dispatch, ASTs, and exceptions are the running process's, through
// the embedded corevms.EngineHooks; the console I/O and command selectors
// are here, reaching the terminal and the console's command line through
// the machine's Hooks.
//
// See services.go's own doc comment on Console.Dispatcher for why
// XFC$CONSOLE_CMD can report "unavailable" even on a fully initialized
// Console.

var (
	_ cpu.SystemServices      = (*Machine)(nil)
	_ cpu.ASTSource           = (*Machine)(nil)
	_ cpu.ExceptionDispatcher = (*Machine)(nil)
	_ cpu.AttentionHandler    = (*Machine)(nil)
)

// ConsoleWriteByte is XFC$CONSOLE_WRITE.
func (c *Machine) ConsoleWriteByte(b byte) {
	if out := c.Hooks.Output(); out != nil {
		_, _ = out.Write([]byte{b})
	}
}

// ConsoleWrite is XFC$CONSOLE_PUT: writes a whole string to the output, in one
// Write, so a line of microkernel output reaches the terminal at once
// instead of a character at a time. Does nothing if no output is configured.
func (c *Machine) ConsoleWrite(p []byte) {
	if out := c.Hooks.Output(); out != nil {
		_, _ = out.Write(p)
	}
}

// ConsoleReadByte is XFC$CONSOLE_READ. Reads one byte from the terminal's
// input (Hooks.Input), or reports 0
// if no input source is configured or it's exhausted — matching a real
// getchar() at EOF returning a sentinel the C source doesn't itself check
// for either.
func (c *Machine) ConsoleReadByte() byte {
	var buf [1]byte

	in := c.Hooks.Input()
	if in == nil {
		return 0
	}

	if _, err := in.Read(buf[:]); err != nil {
		return 0
	}

	return buf[0]
}

// ConsoleCommand is XFC$CONSOLE_CMD: has the console run cmd as a
// command line (Hooks.ConsoleCommand). Returns 0 on success, 1 on any
// error, or when there's no console to run it.
func (c *Machine) ConsoleCommand(cmdLine string) uint32 {
	if c.Hooks.ConsoleCommand == nil {
		return 1
	}

	return c.Hooks.ConsoleCommand(cmdLine)
}

// RequestQuit is XFC$QUIT_EMULATION's "stop the console entirely" half
// (see cpu.SystemServices' own doc comment): the same effect as QUIT,
// reached here from a running VAX program instead of a typed command
// (Hooks.RequestQuit).
func (c *Machine) RequestQuit() {
	if c.Hooks.RequestQuit != nil {
		c.Hooks.RequestQuit()
	}
}

// HandleAttention makes Machine a cpu.AttentionHandler: an attention key
// typed while the machine runs goes to Hooks.HandleAttention.
func (c *Machine) HandleAttention(key byte) bool {
	return c.Hooks.HandleAttention(key)
}
