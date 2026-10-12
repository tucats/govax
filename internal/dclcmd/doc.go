// Package dclcmd is govax's utility commands, the ones VMS runs as
// images or as DCL commands that work on files, volumes, devices, and
// logical names: DIRECTORY, TYPE, COPY, DELETE, PURGE, RENAME, CREATE,
// SET/SHOW DEFAULT, MOUNT/DISMOUNT, INITIALIZE/CONTAINER, DEFINE/ASSIGN/
// DEASSIGN and SHOW LOGICAL/TRANSLATION, DEFINE/DEVICE and SHOW DEVICES,
// SHOW SYSTEM and SHOW PROCESS, and the development tools MACRO, LINK,
// LIBRARY, and ANALYZE (docs/PHASE-52 - console package split.md).
//
// Each command is a method of Context, which the console builds for each
// command from the DCL that runs it (internal/console's Console.cmd):
// that DCL's output, default directory, logical names, and process, the
// machine's devices and mounted volumes, and the few things a command
// asks of DCL itself (Context's function fields). The package knows
// nothing of the console, the DCL interpreter, or the machine; the
// console binds the commands into its grammar.
package dclcmd
