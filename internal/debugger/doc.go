// Package debugger is govax's machine debugger, modeled on the VMS
// debugger (docs/PHASE-42.md).
//
// The govax console has two jobs. It is a VMS command line (SET DEFAULT,
// MOUNT, MACRO, LINK, RUN, ...), and it is a debugger for the emulated
// VAX (EXAMINE, DEPOSIT, STEP, SET BREAK, SHOW REGISTERS, ...). Phase 42
// separates them. The console stays in internal/console; the debugger
// lives here, with its own DCL grammar (debug.dcl, in internal/bootdata),
// its own help file (debug.help), and its own "DBG> " prompt. Where it
// implements a VMS debugger command, the syntax and the output are the
// VMS debugger's.
//
// # Sessions
//
// A debugger session starts when the console's DEBUG command hands the
// console's machine to the debugger, or when a GO, CALL, or STEP stops, or
// RUN of an image linked /DEBUG (or RUN/DEBUG) begins. A run that ends by
// itself returns to the console without a session. From the start until
// EXIT or QUIT, every command line goes to the debugger's Dispatcher, not
// the console's, and EXIT returns to the console, leaving the machine as
// it is.
//
// # Commands
//
// The grammar is debug.dcl and the help is debug.help. The VMS debugger's
// commands (EXAMINE, DEPOSIT, EVALUATE, SYMBOLIZE, STEP, GO, CALL, SET and
// CANCEL BREAK/TRACE/WATCH, SHOW CALLS/IMAGE/MODULE/SYMBOL/SCOPE, SET
// MODE/RADIX/SOURCE, ...) follow it, and govax's own for the machine
// (SHOW REGISTERS, SET PTE, SET PSL, SHOW MEMORY, ...) are marked as its
// own in debug.help. A command's messages and layouts are the VMS
// debugger's, checked against the logs of VMS 7.3's debugger
// (testdata/dbg and testdata/dbgcmd) by the oracle tests, of which
// TestDebuggerSessionOracle replays every probe session and lists what
// still differs.
//
// # Where things are
//
// debugger.go is the session and its start; runcontrol.go the run loop;
// eventpoint.go, breakcmd.go, tracepoint.go, watch.go, instbreak.go, and
// faultbreak.go the eventpoints; step.go and stack.go STEP and the call
// frames; examine.go, data.go, and modes.go EXAMINE, DEPOSIT, EVALUATE, and
// the display modes and radix; machine.go the commands for the CPU and
// kernel state; program.go the commands about the program's symbols;
// source.go the source lines; image.go RUN under the debugger.
//
// # Dependencies
//
// This package imports internal/console, because the debugger works on
// the console's machine: its engine, memory, symbols, and loaded images.
// The console can't import this package back, so it knows the debugger
// only through the small console.Debugger interface, which *Debugger
// implements. cmd/govax installs the debugger with Install.
package debugger
