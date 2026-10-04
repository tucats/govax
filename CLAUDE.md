# CLAUDE.md

This file provides guidance to Claude Code when working in this repository.

## Project overview

`govax` is a from-scratch Go rewrite (not a cross-compile) of `eVAX`, a C emulator for
a DEC VAX/VMS system. The goal is a careful evaluation of the C implementation followed
by rewriting it in idiomatic Go, with a comprehensive unit test suite the C version
never had.

- `docs/PLAN.md` — high-level plan, locked-in architecture decisions, and the phase
  index.
- `docs/PHASE-00.md` … `PHASE-35.md` — one doc per phase: goal, C-source file
  mapping, deliverables, open questions, and a dated progress log (all
  done through 35; Phase 29's debugger records, its subtask 12, are
  deferred). Read the relevant phase doc
  before starting work on that subsystem, and extend its progress log as you go.
- `docs/DEVIATIONS.md` — running log of suspected ISA/behavior fidelity issues found in
  the C source during porting (see "Bug-fixing policy" below).
- `docs/MODE-STACKS.md` — where VMINIT puts each access mode's stack, their sizes and
  page protections, and what was deliberately left unchanged.

## Reference material

- `reference/eVAX/` — a full, read-only copy of the C source tree (imported via
  `git archive` from the upstream `tucats/evax` repo). This is the primary behavioral
  reference during conversion — read the corresponding C file before/while porting a
  subsystem. Do not edit it; if upstream changes, re-import.
- `reference/AUDIT.md` — the C project's own closed 32-vs-64-bit portability audit
  (root-cause `LONGWORD` typedef bug and its downstream fixes). Explains *why* certain
  things in the C source look the way they do; the fixes it documents are already
  present in `reference/eVAX/`.
- `reference/CLAUDE.md` — the C project's own CLAUDE.md, useful background on its
  architecture and gotchas (e.g. CR-only line endings in some files).
- `testdata/{asm,exe,rom,dcl}/` — fixtures pulled from the C repo's root
  (`.asm` sources, real VMS `.exe` binaries, `xdefault.rom`, `evax.dcl`/
  `vax.init`), organized by type for use in Go tests. `vax.help` used to have
  a second copy here too; it was consolidated down to the single, live copy
  at `internal/bootdata/files/vax.help` (Phase 22) since, unlike `evax.dcl`,
  it had no ongoing reason to track a separate upstream-import lineage.
- `testdata/mar/forth.mar` — a FORTH interpreter in MACRO-32 (RMS I/O,
  five psects, macro-built dictionary), run by `internal/console`'s
  `TestForth` tests; not on the real-MACRO ladder yet.
- `testdata/mar/` — Phase 27's MACRO-32 fixtures, with real VAX MACRO's objects,
  listings, and analyses in `vax/` (see its README for the simh round trip).
  `testdata/disks/` holds local-only ODS-2 containers (gitignored).
- `~/Documents/Technical Doc/VMS/vax_instr_set.pdf` — the VAX architecture/
  instruction-set reference manual (a copy may also be at
  `reference/vax_instr_set.pdf`, local-only).
- **Clean-room barrier (Phase 32).** govax's own system macros
  (`internal/bootdata/files/starlet.mar`) are written from DIGITAL's manuals
  and from real MACRO's *output*, never from VMS's STARLET macro library's text.
  `.claude/hooks/cleanroom.sh`, a PreToolUse hook, refuses tool calls that
  name that library, `testdata/vmslib`, `reference/vms`, or unaudited
  real-MACRO listings. It matches on text, so a command or search that merely
  mentions the library's file name is refused too. Use the Edit/Write tools
  for docs that name it, and `git commit -F` for such commit messages. Don't
  work around the hook; its test cases are in `cleanroom_test.sh`.
  Since 2026-10-04 the project is fully clean room: the VMS source archive
  (`~/Documents/Technical Doc/VMS/vmssrc_archive/`, VMS's own source
  listings) is off-limits too. Earlier phase docs that cite it record
  history, not a method to follow. When neither a manual nor real VMS
  output settles a rule, make a reasonable choice and log it as
  unconfirmed in the phase doc.
- `reference/vms/` — local-only (gitignored, Phase 31): licensed VMS 7.3
  definition files (SDL sources, C headers, BLISS and message listings) that
  `internal/vmsdef`'s tables were first generated from. Nothing in the build
  or tests reads them; use them with `internal/vmsdef/gen` to add
  definitions. Never commit DIGITAL/HP/VSI-copyright material (see also
  `testdata/vmslib/` and `testdata/disks/`).

## Build & test

- `go build ./...`
- `go vet ./...`
- `go test ./...` (as tests are added per phase)

## Architecture

Module `github.com/tucats/govax`. State lives in an instantiated struct passed
explicitly / receiver-bound — no package-level singleton mirroring the C source's
global `struct VAX vax`. Current package layout (see `docs/PLAN.md` for rationale, and
expect adjustment as phases land):

- `internal/vax` — core machine state: registers, PSL, condition codes (Phase 01).
- `internal/vm` — virtual memory: address translation, load/store primitives (Phase 02).
- `internal/cpu` — instruction decode/execute engine and instruction-set emulation
  (Phases 03-07). Phase 35 finished the set: every instruction but LDPCTX and
  SVPCTX has a handler (`TestEveryInstructionImplemented`). Each operand has a
  `DataType` in the generated table, which decides how a short literal or
  floating operand is read. F, D, G, and H instructions run on
  `internal/vaxfloat` (`fpu.go` is the CPU's side), packed decimal on
  `decimal.go` (`decimalmath.go`, `decimalconvert.go`, `editpc.go`), and
  octawords through `Operand.LoadOctaword`/`StoreOctaword`. An arithmetic
  *trap* (integer or decimal overflow, DIVP by zero) is
  `Engine.arithmeticTrap`, whose saved PC is the next instruction's. The
  Phase 35 oracle (`testdata/insn35`, `TestInsn35Oracle` in
  `internal/console`) compares 567 instruction cases with VMS's run.
- `internal/vaxfloat` — the VAX floating formats (Phase 35): `Unpack`/`Pack` for
  F, D, G, and H, and exact `Value`s (`math/big`) whose arithmetic rounds once,
  half away from zero, to the destination format, reporting overflow,
  underflow, reserved operands, and divide by zero; plus EMOD and POLY's
  arithmetic, short literals, and decimal parsing for the assembler. A leaf
  package: no CPU dependency.
- `internal/console` — interactive monitor + DCL grammar interpreter (Phase 08).
- `internal/io` — device abstraction (Phase 09).
- `internal/vmsdef` — VMS's own definitions, shared by the assembler, RTL, RMS,
  and LINK: `Symbols` (every symbolic constant, one table; `symbols.go` says
  what each prefix holds), `Messages`, the FAB/RAB layouts, the P1 vector, and
  the symbols LINK would read from VMS files (`SharedImages`/`ImageSymbols`
  from LIBRTL.EXE, `LibrarySymbols` from STARLET.OLB's definition modules).
  The `*_generated.go` tables are committed data, and building doesn't
  regenerate them: `go run ./internal/vmsdef/gen` merges new definitions
  (`-h`/`-sdl`/`-bliss`/`-msg`/`-image`/`-olb FILE`) into them, add-only
  unless `-replace`; `-drop NAME` removes a stale name, `-n` is a dry run
  (Phase 31).
- `internal/lnm` — VMS logical-name database: directories, tables, access modes,
  search lists, `$TRNLNM`-style lookup and RMS file-spec translation (Phase 25).
  A leaf package shared by the console, `internal/rms`, and `internal/coreos`.
- `internal/coreos` — VMS RTL/system-service simulation (Phase 10).
- `internal/librtl` — LIBRTL.EXE's routines (Phase 34): the LIB$ and STR$
  shims a program reaches through `SHIM$LIBRTL_<offset>` stubs. `Routines`
  lists each with its transfer-vector offset (checked against
  `vmsdef.ImageSymbols`) and XFC$SHIM code; the console registers them into
  each RTL environment and builds the stubs from the same table. The process
  machinery they use (condition dispatch, the heap, memory) stays in
  `internal/coreos`, reached through `export.go`. Each further *RTL.EXE emulated
  gets a package like it. New routines are written from DIGITAL's manuals
  (clean room), and checked on VMS where a probe can: LIB$CREATE_DIR
  (`createdir.go`) against `testdata/credir/libcrd.mar`'s VMS 7.3 run
  (`TestLibCreateDirOracle`). LIB$PUT_OUTPUT is here too (`output.go`); kernel.asm's
  old interrupt-driven one is now its private `EXE$PUT_OUTPUT`.
- `internal/asm` — assembler/disassembler (Phase 11). Two dialects share one core
  (Phase 27): the console's `ASM` (absolute, into emulated memory, eVAX
  directives) and MACRO-32 (`SetDialect(DialectMACRO)`: psects, relocation
  trees, every error reported, and `Object()` for a `.OBJ` module). Phase 28's
  macro facility is in both: macro definitions and calls (`macros.go`,
  `macroargs.go`), repeat blocks (`repeat.go`), and a stack of sources
  (`source.go`); the MACRO dialect also searches macro libraries
  (`MacroLibrary`, `maclib.go`) handed in by the console's MACRO command.
  `overwrite.go` keeps a field stored twice (as `$FAB` does) as real MACRO
  writes it. Phase 29 adds MACRO listings: `SetListing` records a
  `listLine` per source line (`listing.go`), and `Listing` lays out the
  source pages (`listpage.go`), the listing controls (`listctl.go`),
  messages, the cross reference (`xref.go`, `SetCrossReference`), and the
  closing pages (`listclose.go`), as real MACRO lays them out. `Object()`
  writes traceback (TBT) records by default (`traceback`; `SetFunctions`
  is `/ENABLE=`/`/DISABLE=`, and the console maps `/DEBUG` onto it).
  Debugger (DBG) records aren't written (Phase 29's subtask 12, deferred).
- `internal/obj` — the VAX object language (Phase 27): reads, writes, dumps, and
  checks `.OBJ` object modules, keeping every record so a real VAX object
  round-trips byte for byte; `Builder` packs a module's psects, symbols, and TIR
  commands into records. Codes and layouts come from `vmsdef.Symbols`
  (the VAX object language names of VMS 7.3's `objfmt.sdl`). Host files hold records in ODS-2's
  on-disk variable-length layout (`ReadRecords`/`WriteRecords`).
  `dst.go` (Phase 29) decodes and encodes the debug symbol table (DST)
  records TBT and DBG records carry, and builds real MACRO's four
  traceback records; their layouts come from real objects (clean room),
  and `obj.Dump` shows them. `Builder.Traceback` adds TBT records.
- `internal/rms` — RMS (`SYS$CREATE`/`CONNECT`/`OPEN`/`CLOSE`/`GET`/`PUT`/`RENAME`,
  and Phase 33's `PARSE`/`SEARCH`/`DISPLAY` with NAM blocks and XABs) file
  I/O backed by the sibling Go module `github.com/tucats/ods2`'s real ODS-2
  volume/file implementation, plus the `MOUNT`/`DISMOUNT`-facing `MountTable`
  (Phase 22). Also decides whether a typed file name means a host file or a
  volume file (`Session.Locate`, `location.go`) and reads/creates record files
  on either side (`ReadRecordFile`/`CreateRecordFile`, and `RewriteRecordFile`,
  which keeps a volume file's version; `recordfile.go`; Phase
  27). The sole place in this project allowed to import `ods2`; owns its
  own IFI (open-file) table separately from `internal/coreos`'s state, since it
  tracks real `ods2` handles Phase 10's RTL layer never needed. `go.mod`
  pins a tagged `ods2` release, so a plain clone builds; a local, gitignored
  `go.work` (`use .` / `use ../ods2`) overrides it with the sibling checkout
  for co-development (see `docs/PHASE-22.md`'s "Dependency: `go.work`, not a
  `replace` directive"). Because the workspace hides a stale pin, when govax
  starts using new `ods2` API, tag and push `ods2`, then
  `GOWORK=off go get github.com/tucats/ods2@vX.Y.Z` and check
  `GOWORK=off go build ./...` before pushing govax. Name processing
  (`name.go`: default names, related files, logical names, search lists)
  is shared by every service, and `$SEARCH` (`search.go`) keeps VMS's two
  kinds of wildcard context; Phase 33's runtime oracle (`testdata/mar/rms3`,
  `TestRMS3Oracle`) checks them against VMS 7.3 byte for byte.
  `Session.CreateDirectory` (`createdir.go`, Phase 34) is the console's
  `CREATE/DIRECTORY` (`internal/console/create.go`): ods2 lays the directory
  files out (`volume.CreateDirectory`, `filespec.CreateDirectoryPath`), and
  `TestCreateDirectoryOracle` (`testdata/credir`) checks headers and
  messages against VMS 7.3's run. A RAB connected to the terminal reads lines from the
  console's input, prompting with RAB$L_PBF when RAB$V_PMT is set
  (`terminal.go`).
- `internal/link` — the VAX linker (Phase 30): builds a VMS executable image from
  `internal/obj` modules, laid out as real LINK lays images out (byte for byte on
  the fixtures). The console's `LINK` command (`internal/console/link.go`) drives it.
  Undefined symbols come from symbol sources (`source.go`, `libsource.go`):
  IMAGELIB.OLB plus shareable images' GSTs, STARLET.OLB, then govax's own tables
  (`internal/console/linksource.go`). With traceback (the default), pass 2
  also runs each module's TBT records into the image's debug symbol table,
  which follows the image's other blocks and the IHS block points at
  (Phase 29); DBG records are skipped. Fixup cells for a shareable image
  are in symbol-name order (`orderCells`).
- `internal/lbr` — the librarian file format (Phase 30): reads `.OLB`/`.MLB`/etc.
  libraries, their B-tree indexes and module records, including DCX data-reduced
  libraries (`dcx.go`) such as STARLET.OLB. Writes them too (Phase 28): `Builder`
  (`Create`/`Edit`, `Insert`/`Replace`/`Delete`, `Bytes`) lays a library out as
  VMS's librarian does, and `MacroModules`/`ObjectModules` (`input.go`) apply
  LIBRARIAN's rules for turning macro source and object files into modules;
  `List` is LIBRARY/LIST's listing, in LIBRARIAN's own formats. The console's
  `LIBRARY` command (`internal/console/library.go`) drives it. Imports only
  `internal/obj` and `internal/vmsdef`.
- `cmd/govax` — `main.go` (CLI entry point) plus `grammar.go` (the `tucats/gopackages`
  `app-cli/cli` option/subcommand grammar — `stats`/`path`/`instruction-limit`/
  `time-limit` options, repeatable `mount`/`mount-write DEVICE=container`,
  `console`/`asm`/`run`/`macro`/`link`/`library` subcommands). A one-shot subcommand that fails
  makes govax exit nonzero, and volumes still mounted are dismounted (flushed)
  when a session ends. Briefly moved to the repo
  root (2026-09-17); moved back into `cmd/govax` as the more standard layout
  (`go build ./...`/`go run ./cmd/govax`).
- `tucats/gopackages` also brings config-settings support (`app-cli/settings`), read at
  `Engine` construction (`internal/cpu/engine.go`'s `NewEngine`). Settings implemented
  so far: `vax.hardware.clock` (bool) — when true, `Engine.Step` drives the interval
  clock/TODR off `time.Now().UnixMilli()` instead of the deterministic instruction-quantum
  mechanism (see `tickQuantum`/`tickIntervalClock` in `internal/cpu/interrupt.go`); when
  false/unset, the old quantum-driven path is used. `vax.quantum` (int) — default
  quantum-tick interval instead of the hard-coded `defaultQuantum` (20); only takes
  effect if `> 0`. `vax.library` (string) — the host directory LINK and MACRO look
  in for IMAGELIB.OLB, STARLET.OLB, shareable images, and STARLET.MLB when
  `SYS$LIBRARY`/`SYS$SHARE` don't lead to them (read by the console, not `NewEngine`;
  `internal/console/syslib.go`); the older `vax.link.library` is still read when
  it isn't set. With no STARLET.MLB found, MACRO uses govax's own from bootdata:
  `files/starlet.mar` (system service and RMS macros, hand-written clean-room)
  plus `files/starletdef.mar` (the `$xxxDEF` macros, generated by
  `internal/bootdata/mkdefs` from `testdata/mar/rms/defined.txt`), built into
  `starlet.mlb` by `go generate ./internal/bootdata` (Phase 32). Each RMS macro
  is checked byte for byte against real MACRO's objects for the oracle probes
  in `testdata/mar/rms` (`internal/asm/oracle_test.go`).

## Bug-fixing policy while porting

The C source's fidelity to the VAX ISA/hardware definition is good but not perfect.
When you hit a bug or suspicious behavior while converting a piece of C to Go:

- **Clear, obvious logic errors not tied to ISA semantics** (off-by-one, copy-paste
  mistakes, dead code, a condition that plainly contradicts its own comment) — just fix
  them in the Go code as you go. No need to ask or log these.
- **Suspected ISA/hardware-definition fidelity issues** — cases where the emulated
  behavior doesn't match the VAX spec, or doesn't match what the C code's own comments
  claim it does — default to **documenting and deferring**: record the finding in
  `docs/DEVIATIONS.md`, and have the Go port replicate the C source's current
  (possibly imperfect) behavior for now, to be revisited in a future debugging round
  (see Phase 12). Only fix immediately if the correct fix is clear-cut and fits
  naturally within the current change's scope.
- **When it's not obvious which of the above applies**, ask the user rather than
  deciding unilaterally — this is a judgment call by design.

This mirrors how `reference/eVAX/AUDIT.md` was produced on the C side: findings get
catalogued with enough detail to act on later, not silently patched over or ignored.
