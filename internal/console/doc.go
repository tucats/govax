// Package console implements the govax interactive monitor front end: the
// VMS-style command line (RUN, MACRO, LINK, MOUNT, DIRECTORY, DEFINE, SET
// DEFAULT, SAVE/LOAD, ...) and the DCL grammar-driven verb/qualifier dispatch
// built on internal/console/dcl, against an internal/cpu.Engine and its
// internal/vax.CPU/internal/vm.Memory (see docs/PHASE-08.md).
//
// The machine debugger is not here. Phase 42 moved EXAMINE, DEPOSIT, STEP,
// breakpoints, and the SHOW/SET/CLEAR keywords for the machine's state to
// internal/debugger, which has its own grammar (debug.dcl), help file, and
// "DBG> " prompt, and reaches the machine through this package's exported
// methods (export.go). The console starts a debugger session when GO, CALL,
// or RUN stops, and with its DEBUG command; it knows the debugger only
// through the Debugger interface (debugger.go), and works with none
// installed (the commands that start one then say it isn't available).
//
// Changed as recorded in docs/PHASE-08.md's progress log:
// device-dependent commands (SHOW DEVICE, DEFINE/DEVICE) were stubbed
// pending Phase 09 (I/O) and are now wired up in device.go against
// internal/io (the logical-name commands are Phase 25's logical.go, over
// internal/lnm); the inline mini-assembler (ASM/DISASM,
// and EXAMINE's address-expression syntax) is replaced by a small
// standalone expression evaluator (expr.go) rather than waiting on Phase
// 11's real assembler. Every DCL /entry= command (ABOUT, XTEST, SHOW
// VERSION) now works: Dispatch resolves the entry name as a VAX symbol
// (populated once kernel.asm's .ENTRY has been ASMed/booted) and CALLs it,
// matching the C source's own "these are real VAX routines, not native C
// functions" design — see docs/PHASE-16.md sub-phase 4. BOOT remains
// stubbed pending real device/RTL boot support (dispatch.go's
// cmdNotImplemented).
package console
