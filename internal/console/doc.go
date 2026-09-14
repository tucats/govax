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
// in device.go against internal/io; RTL-entry-point commands (CALL, BOOT,
// ABOUT, FORTH, XTEST — anything reached via a DCL /entry=) are stubbed
// pending Phase 10 (RTL), and the inline mini-assembler (ASM/DISASM, and
// EXAMINE's address-expression syntax) is replaced by a small standalone
// expression evaluator (expr.go) rather than waiting on Phase 11's real
// assembler.
package console
