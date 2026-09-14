# Phase 08: Console functionality

## Goal

Build the interactive monitor front-end — EXAMINE, DEPOSIT, RUN, STEP, SAVE/LOAD,
SHOW, and friends — plus the DCL grammar-driven command parser, and stand up
`cmd/govax` as a real, runnable entry point for the first time.

## Scope / C source mapping

- `reference/eVAX/eVAX/Source/Console/driver.c` — `main()`; working-directory-relative
  loading of `evax.dcl`/`vax.init`/`vax.help` (now at `testdata/dcl/` in this repo —
  decide the Go equivalent's file-location strategy, e.g. embedding via `go:embed`
  vs. runtime path lookup).
- `reference/eVAX/eVAX/Source/Console/parse.c`, `console_dispatch.c` — command
  parsing/routing.
- `reference/eVAX/eVAX/Source/Console/console_*.c` — individual commands (EXAMINE,
  DEPOSIT, RUN, STEP, SAVE/LOAD, SHOW, CLEAR, ZERO, SET, VMINIT, TIME, TEST, QUIT,
  INCLUDE, EXEC).
- `reference/eVAX/eVAX/Source/Console/dclrtl.c` + `reference/eVAX/eVAX/Headers/
  dclrtl.h`, `dcldef.h` — the grammar-driven command-language parser itself, whose
  grammar is defined in `testdata/dcl/evax.dcl`. This engine is fairly
  self-contained/generic and worth porting close to as-is.
- `reference/eVAX/eVAX/Source/Console/console_load.c`, `save_binary.c` — binary
  memory-image/ROM/NVRAM file formats (`AUDIT.md` V1 pinned these to literal 4-byte
  fields — carry that fix forward).
- `reference/eVAX/eVAX/Source/Console/errors.c`, `help.c` — error reporting, HELP text
  (`testdata/dcl/vax.help`).
- When implementing the driver, adopt github.com/chzyer/readline package to provide
  prompted input, witt support for history/recall and in-line editing, etc.

## Deliverables

- `internal/console` package implementing the DCL interpreter and command set against
  `internal/vax`/`internal/vm`/`internal/cpu`.
- `cmd/govax/main.go` filled in as a real interactive entry point.
- Tests: DCL grammar parsing unit tests, command-level tests (EXAMINE/DEPOSIT
  round-trips, RUN/STEP against Phase 04-07 fixtures), and a SAVE/LOAD round-trip test
  against `testdata/rom/xdefault.rom`.

## Open questions / notes

- Devices referenced by SHOW DEVICE etc. depend on Phase 09 (I/O) — some console
  commands may need stubbing until that phase lands, or the two phases may end up
  interleaved in practice even though documented separately.

## Progress Log

_Not started._
