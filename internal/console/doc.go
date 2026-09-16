// Package console implements the govax interactive monitor front end: the
// EXAMINE/DEPOSIT/RUN/STEP/SAVE/LOAD/SET/SHOW/CLEAR/... command set and the
// DCL grammar-driven verb/qualifier dispatch built on internal/console/dcl,
// against an internal/cpu.Engine and its internal/vax.CPU/internal/vm.Memory
// (see docs/PHASE-08.md).
//
// This mirrors reference/eVAX/eVAX/Source/Console/console_*.c/driver.c, with
// the scope adjustments recorded in docs/PHASE-08.md's progress log:
// device-dependent commands (SHOW DEVICE, DEFINE/DEVICE, SHOW LOGICAL,
// DEFINE/LOGICAL) were stubbed pending Phase 09 (I/O) and are now wired up
// in device.go against internal/io; the inline mini-assembler (ASM/DISASM,
// and EXAMINE's address-expression syntax) is replaced by a small
// standalone expression evaluator (expr.go) rather than waiting on Phase
// 11's real assembler. Every DCL /entry= command (ABOUT, FORTH, XTEST, SHOW
// VERSION) now works: Dispatch resolves the entry name as a VAX symbol
// (populated once kernel.asm's .ENTRY has been ASMed/booted) and CALLs it,
// matching the C source's own "these are real VAX routines, not native C
// functions" design — see docs/PHASE-16.md sub-phase 4. BOOT remains
// stubbed pending real device/RTL boot support (dispatch.go's
// cmdNotImplemented).
package console
