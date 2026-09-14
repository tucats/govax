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

### 2026-09-14 — Sub-phase 2: console core, EXAMINE/DEPOSIT, INIT/ZERO

- Added `internal/console`: `Console` (`machine.go`) wraps an
  `internal/cpu.Engine`/`internal/vax.CPU`/`internal/vm.Memory` trio that's
  `nil` until `Init` runs (the Go equivalent of the C source's `vax_init`
  flag, checked by `requireInit` the way every `console_*.c` handler checks
  `vax_init` itself), plus a simplified `SymbolTable` (`symbols.go` — a
  `map[string]*Symbol` replacing `struct SYMBOL`'s linked list and
  forward-reference-patching machinery, which has no purpose here since this
  port's expression evaluator never forward-references a symbol) and the
  console's own radix/deposit-address/verbose/verify settings.
- Added `expr.go`: a small, from-scratch recursive-descent expression
  evaluator (comparison → +/- → */÷ → atom, matching `asm_expr`/
  `asm_expr2`/`asm_expr3`'s three-tier precedence) standing in for the real
  assembler's `asm_expr`/`asm_hex`/`asm_dec` (`reference/eVAX/eVAX/Source/
  Assembler/asm_expr.c`, `asm_value.c` — Phase 11's scope, not this one's).
  Supports numeric literals in the console's default radix or with a
  `^D`/`^X`/`^O`/`^B` prefix override (matching `asm_hex`'s own prefix
  handling, confirmed against the actual prefixes `testdata/dcl/vax.init`
  uses, e.g. `init ^d4096`), symbol lookup, `.` for the current deposit
  address, parentheses, and the C source's `=`/`<>`/`<=`/`<`/`>=`/`>`
  comparison operators. Deliberately not ported: register names and
  indirect (`@`) register/PSL references (EXAMINE/DEPOSIT special-case a
  bare register name themselves before ever reaching the evaluator, matching
  `console_exam.c`'s own short-circuit, and no other in-scope command needs
  them), the `DEFINED()`-style function-call syntax, and quoted-string-to-
  string-pool literals (needs string-pool memory management with no other
  consumer yet).
- Added `init.go`: `Console.Init` (`alloc_vax`, minus ROM/NVRAM allocation —
  deferred to Phase 09, see `doc.go`) and `Console.Zero` (`console_zero.c`'s
  ZERO command, including its region/stack-register reset and symbol-table
  wipe), with `Init` also calling `Zero` at the end matching
  `console_init.c`'s own flow. `allocPhysMemory` replicates `alloc_vax`'s
  512-byte-rounding/8192-minimum sizing exactly, including its quirk where a
  too-small request gets clamped to 8192 and *then* bumped by a further 512
  bytes (since the clamped value no longer equals the raw request) — caught
  by a test that initially expected the more "obvious" plain-8192 result.
- Added `exam.go`: `Console.Examine` (`console_exam.c`'s EXAMINE, covering
  registers and BYTE/WORD/LONGWORD/ASCII/PTE memory display — the address-
  structure formats this port doesn't implement, F_FLOATING/D_FLOATING/
  DESCRIPTOR/COUNTED/ZERO-terminated, are noted in `doc.go`/`exam.go`'s
  comments as a deliberately deferred enhancement, not a fidelity gap, since
  the core byte/word/longword/ASCII/PTE path covers everyday register and
  memory debugging) and `Console.Deposit`. The C source has **no** standalone
  DEPOSIT command of its own — memory is normally modified through the
  inline mini-assembler's immediate mode, which is Phase 11's scope — so
  `Deposit` is a deliberate, Go-native addition implementing the "DEPOSIT"
  deliverable this phase's doc names explicitly, reusing Examine's exact
  register/address/size conventions rather than inventing a different one.
- `internal/console/{expr,exam}_test.go` cover: every evaluator feature
  above (radix prefixes, symbols, precedence, comparisons, division by
  zero, undefined-symbol and unparsed-remainder cases); Init's memory
  allocation/rounding and Zero's memory+symbol-table clearing; Examine/
  Deposit register and memory round trips at each size, including the
  ASCII and unknown-register cases, and the pre-Init error path.
- `go build ./...`, `go vet ./...`, `gofmt -l .` (no output), `go test ./...`
  all clean. `go test ./internal/console -cover`: 85.3%.

### 2026-09-14 — Sub-phase 3: RUN/STEP and breakpoints

- Found that `console_run.c`'s `console_run` (the `RUN` verb) is *not* "run
  the CPU" — it's VMS executable-image activation (`image_load`/
  `image_fixup`, an `.exe` file's Image Control Block/Image Section
  Descriptors, `LIB$INITIALIZE` calling and shared-image linking), needing
  the real assembler (`assemble_direct`, Phase 11) and RTL (`lib_initialize`,
  Phase 10) — none of which exist yet. The command this phase's own
  "RUN/STEP against Phase 04-07 fixtures" deliverable actually means is
  `console_exec.c`'s `console_exec`, bound to the dispatch table's separate
  `EXEC`/`GO`/`G` verb: "start the CPU executing at an address." Ported that
  one (as `Console.Execute`) and left the VMS image-loading `RUN` command
  for Phase 10/11 (see `doc.go`); `console_call` (the `CALL` verb, a
  CALLS-style frame builder for calling into a mask-prefixed procedure) is
  deferred alongside it — useful mainly for RTL entry points, per the same
  rationale as `ABOUT`/`FORTH`/`XTEST`'s `/entry=` commands.
- Added `run.go`: `Console.Execute` runs `Engine.Step` in a loop, checking
  an address-breakpoint list before each instruction (skipping the very
  address execution started from, matching `vax.c`'s `initial_PC` special
  case, so resuming from a breakpoint doesn't immediately re-trigger it) and
  stopping on `cpu.ErrHalted` or any other error — the breakpoint-on-top-of-
  `Step` layering `docs/PHASE-03.md` explicitly reserved for this phase.
  `Console.Step` runs exactly one instruction (`console_step.c`'s
  `STEP_INSTRUCTION`/default mode). `STEP_OVER`/`STEP_RETURN` are
  deliberately not implemented (both act as a synonym for a single
  instruction step instead of skipping over a called subroutine) — real
  step-over needs the same temporary-breakpoint-at-return-address machinery
  `vax.c`'s `STEP_OVER` case uses, and building it has no other consumer to
  justify the cost right now; noted here rather than silently behaving
  differently from its name.
- `Breakpoint`/`AddBreakpoint`/`RemoveBreakpoint`/`ClearAllBreakpoints`
  implement only address breakpoints (`struct BREAKSTR`'s `BREAK_ADDRESS`
  case) — fault-code breakpoints (`BREAK_FAULT`) aren't implemented, since
  `Engine.HandleFault` delivers a fault to the SCB internally before
  `Step` ever returns, giving the console no hook to intercept the raw
  fault code without further `internal/cpu` API surface; left as a gap to
  revisit if a real use for it shows up (e.g. once SET/SHOW BREAKPOINT's
  DCL grammar work is wired up in a later sub-phase).
- `internal/console/run_test.go` builds tiny NOP/HALT programs directly via
  `Deposit` (no assembler yet) and covers: running to HALT, stopping at a
  breakpoint, the breakpoint-at-start-doesn't-immediately-refire case,
  single-instruction STEP (including that a second STEP without a new start
  address continues from where the CPU is), breakpoint add/remove/clear
  (including a no-op duplicate add), and the pre-Init error path.
- `go build ./...`, `go vet ./...`, `gofmt -l .` (no output), `go test ./...`
  all clean.
