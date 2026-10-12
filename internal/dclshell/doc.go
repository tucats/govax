// Package dclshell is govax's DCL, the command language interpreter the
// console runs in process 1 and in every process that runs a CLI
// (docs/PHASE-50 - DCL command procedures.md, docs/PHASE-52 - console
// package split.md).
//
// An Interpreter is every process's DCL. Each process has a Shell of its
// own (its process, logical names, default directory, output, symbols,
// command levels, $STATUS, open files, ...); the one whose commands run
// now is embedded in the Interpreter, so its fields read as the
// interpreter's. A subprocess's CLI (subcli.go) runs on a goroutine of
// its own that takes turns with the engine.
//
// DispatchConsole runs one command line as DCL does: an IF branch's
// lines passed over, labels, apostrophe substitution and verification,
// then a symbol as the verb, ampersands, "@file", and the console's
// grammar (Grammar), whose DCL verbs BindDCL binds. Command procedures
// (procedure.go), flow control, statuses and ON, SYS$INPUT and DECK,
// DCL's file I/O, STOP and Ctrl/Y, WAIT, lending the CPU between
// commands, SPAWN, and the lexical functions are here.
//
// The interpreter runs on a machine (internal/machine), whose Hooks
// NewInterpreter installs for what the machine asks of DCL. The console
// (internal/console) binds its own and the utility commands'
// (internal/dclcmd) verbs into the same grammar, and routes lines to the
// debugger or the interactive assembler when one is reading (SetRoute).
package dclshell
