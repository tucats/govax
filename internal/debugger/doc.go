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
// A debugger session starts when the console's DEBUG command (or, in
// later subtasks, GO, CALL, or RUN of a debug image) hands the console's
// machine to the debugger. From then until EXIT or QUIT, every command
// line goes to the debugger's Dispatcher, not the console's. EXIT
// returns to the console; the machine is left as it is.
//
// # Dependencies
//
// This package imports internal/console, because the debugger works on
// the console's machine: its engine, memory, symbols, and loaded images.
// The console can't import this package back, so it knows the debugger
// only through the small console.Debugger interface, which *Debugger
// implements. cmd/govax installs the debugger with Install.
package debugger
