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

### 2026-09-14 — Sub-phase 1: DCL grammar engine

- Added `internal/console/dcl`, a from-scratch reimplementation of
  `dclrtl.c`/`dclrtl.h`'s grammar-driven command-language parser, rather than
  a literal port of the C source's FSM byte-code interpreter
  (`FSMdefine`/`FSMrun`) and its self-hosting `$$$DCL$$$` grammar bootstrap.
  This is a deliberate architecture decision, not a fidelity deviation (see
  `internal/console/dcl/doc.go`'s design-notes comment for the full
  rationale): the DCL engine is the console's own parsing tool, not emulated
  VAX ISA behavior, so `docs/DEVIATIONS.md`'s bug-fixing policy doesn't
  apply, and this project's stated goal is a careful-but-not-cross-compiled
  rewrite (`docs/PLAN.md`). The reimplementation targets full behavioral
  compatibility with the actual grammar dialect `testdata/dcl/evax.dcl`
  uses (confirmed by grepping the file for every directive/switch it
  contains before implementing): `grammar`/`verb`/`syntax`/`type`/`keyword`/
  `qualifier`/`parameter`/`disallow`/`end` statements with `-`
  line-continuation and `!` comments; `$any`/`$name`/`$string`/`$integer`/
  `$rest_of_line` value types; `/id`/`/type`/`/prompt`/`/default`/`/alias`/
  `/entry`/`/syntax`/`/nonegatable` switches; unambiguous-abbreviation
  verb/qualifier/keyword matching with "NO"-prefix negation (ported from
  `DCLkeysearch`/`DCLqualsearch`'s matching rule); a keyword's or
  qualifier's `/syntax=` redirecting the parse into a named alternate
  syntax entry (ported from `DCLkeysearch`'s verb<->syntax switch, which
  `DCLdispatch` then walks to find whichever entry ended up "active");
  `$rest_of_line`'s greedy consume-to-end-of-line behavior once it's the
  next due parameter (matching `DCLparse`'s `fsm_allow_rest` gate — with one
  correction found while testing against `SHOW PAGE/READ/WRITE 200`: a `/`
  must always be tried as a qualifier first, even when a REST_OF_LINE
  parameter is next due, since qualifiers may legally precede it on the
  line; only once a plain (non-`/`) positional token is next does the
  REST_OF_LINE parameter's greedy consumption kick in — the C source's own
  FSM achieves the same net effect via its state-transition table, which
  this port doesn't reproduce literally); a parameter's required-ness being
  exactly `/prompt=`'s presence (`DCLprompt`'s `DCL_REQ` side effect, not a
  separate `/required` switch — confirmed no grammar statement in
  `evax.dcl` uses one); qualifier defaults and `DISALLOW` combination
  checks (`DCLcheck_requirements`).
- Deliberately not implemented: `DCLparse`'s interactive terminal
  re-prompting for a still-missing required parameter/qualifier (`Parse`
  reports it as an ordinary error instead, leaving any prompting UI to the
  console layer that calls it); `/local_to` qualifier-to-parameter binding
  and per-keyword `/nonegatable` (neither used anywhere in `evax.dcl`,
  confirmed by grep before starting); `$filename`/`$datetime` value types
  (same — unused).
- `internal/console/dcl/{grammar,match,define,validate,parse,dispatch}.go`
  plus tests. Tests load the real `testdata/dcl/evax.dcl` (not a synthetic
  fixture) and cover: grammar structure (verb/alias/parameter-type
  resolution), command parsing (abbreviated verbs/keywords, `/syntax=`
  redirection via both a parameter's keyword value and a qualifier,
  `$rest_of_line`, qualifier defaults, quoted-string case preservation,
  negation, the DISALLOW regression above, missing-required-parameter and
  ambiguous/unknown-verb errors), and `Bind`/`Dispatch`.
- `go build ./...`, `go vet ./...`, `gofmt -l .` (no output), `go test ./...`
  all clean. `go test ./internal/console/dcl -cover`: 79.8% statement
  coverage (the uncovered remainder is almost entirely defensive error
  paths for malformed grammar-definition text, which `evax.dcl` itself
  never exercises).
