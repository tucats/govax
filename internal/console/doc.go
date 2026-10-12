// Package console is govax's front end (docs/PHASE-08.md, and
// docs/PHASE-52 - console package split.md for its present shape): a
// Console is a machine (internal/machine) and its DCL
// (internal/dclshell), and a Dispatcher routes each command line typed
// or read to whichever reads it now: the interactive assembler (ASM's
// mode), the debugger (internal/debugger, through machine.Debugger), or
// DCL (Interpreter.DispatchConsole), and at command level 0 lets the
// other processes run until they're idle.
//
// The console's grammar (internal/bootdata/files/console.dcl, read by
// internal/dcl) holds every command. DCL binds its own verbs
// (Interpreter.BindDCL); this package binds the machine's (INIT, VMINIT,
// ASM, SAVE/LOAD, SET and SHOW of the machine's state, ...) and the
// utility commands (internal/dclcmd: DIRECTORY, TYPE, COPY, MOUNT,
// DEFINE, MACRO, LINK, ...), each run on a dclcmd.Context built from
// the DCL running it (Console.cmd). It is also internal/debugger's
// debugger.Console, and runs govax's one-shot command line and its run
// limits (misc.go).
package console
