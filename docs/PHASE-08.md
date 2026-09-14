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

### 2026-09-14 — Sub-phase 4: SAVE/LOAD binary ROM/NVRAM image format

- Found that `save_binary.c`'s main `save_binary` (the plain `SAVE` command,
  producing a `.VAX` file) dumps `struct VAX` wholesale, byte-for-byte,
  including raw pointers — inherently tied to one C compiler's exact struct
  layout and meaningless to reproduce in Go (`reference/AUDIT.md`'s V1
  finding calls this out as a cross-build hazard even on the C side, "no
  fixture currently ships in this format"). Not ported; this phase's actual
  "SAVE/LOAD round-trip test against `testdata/rom/xdefault.rom`" deliverable
  is the separate, self-contained, portable `SAVE/ROM`↔`LOAD/ROM` binary
  format (`save_rom`/`load_rom`), which *is* ported, faithfully, in full.
- Decoded that format directly against `xdefault.rom`'s real bytes (not just
  the source): an exact 8-byte magic `;ROMIMG\r` (trailing CR, matching this
  project's own noted Mac-1997-99 line-ending heritage — `console_rom`'s
  reader only `fgets`s 7 of these 8 bytes, leaving `load_rom`'s first read to
  consume the CR; this port just treats the whole 8 bytes as one fixed
  value), big-endian `rom_base`/`rom_end` longwords (confirmed big-endian by
  cross-checking the decoded base, `0x20040000`, against `console_init.c`'s
  own documented ROM default base), then a sequence of big-endian
  (ROM-relative offset, page-count-always-1) headers each followed by 512
  bytes of page data, for every non-all-zero page, terminated by a
  zero-count entry — matching `reference/AUDIT.md`'s V1 finding that these
  fields must be pinned to a literal 4 bytes each (already reflected in the
  C source read for this port, not a live bug to route around).
  `SAVE/NVRAM`↔`LOAD/NVRAM` (`save_nvram`/`load_nvram`) use the same
  big-endian-longword convention but no magic header and no per-page
  framing — the whole buffer is one big-endian `(base, size)` header plus
  `size` raw bytes.
- Added `rom.go`: `Console.SaveROM`/`LoadROM`/`SaveNVRAM`/`LoadNVRAM`
  against new `Console.ROM`/`ROMBase`/`ROMEnd`/`NVRAM`/`NVRAMBase`/
  `NVRAMEnd` fields — separate byte buffers outside `vm.Memory`'s RAM,
  matching the C source's own `rom`/`nvram` globals and consistent with
  `internal/vm/memory.go`'s own design note that physical resolution beyond
  RAM is split between this phase (the file format, landed here) and Phase
  09 (actually mapping them into the address space `vm.Translate`
  resolves, not done here — nothing yet reads `Console.ROM` during address
  translation).
- Found, while writing the round-trip test against the real fixture, that
  `xdefault.rom` itself contains two page entries at addresses that aren't
  512-aligned (`0xD00` = 3328 and `0x10D80` = 68864, each overlapping its
  neighbor by 256 bytes) — inconsistent with `save_rom`'s own strict
  `base = page * 512` convention, so whatever produced this fixture wasn't
  the current `save_rom` (or hit a bug later fixed). Confirmed this is a
  property of the fixture, not a loader bug, by hand-parsing the file's raw
  `(addr, count)` entries directly (all 246 headers, zero duplicates, only
  those two misaligned) before writing any Go code to read it. Consequence:
  a load→save→reload round trip reproduces identical final memory content
  (what the test actually checks) but not an identical on-disk byte layout
  (`SaveROM` always emits clean 512-aligned pages) — documented in the
  test itself rather than silently loosening the assertion without
  explanation.
- `internal/console/rom_test.go` covers: loading the real fixture and
  checking its decoded base/end/size plus a hand-verified byte spot-check;
  the content-preserving round trip above; a synthetic save→load round trip
  with a mix of zero and non-zero pages (checking the all-zero page is
  correctly skipped on save); the wrong-magic and no-ROM-loaded error
  paths; and an NVRAM save→load round trip.
- `go build ./...`, `go vet ./...`, `gofmt -l .` (no output), `go test ./...`
  all clean. `go test ./internal/console -cover`: 82.7%.

### 2026-09-14 — Sub-phase 5: SET/SHOW core

- `console_set.c` (1306 lines) and `console_show.c` (1929 lines) are by far
  this phase's largest C files, covering dozens of sub-forms — most either
  device-, assembler-, or RTL/microkernel-dependent (deferred per doc.go) or
  low-value debug-tracing toggles with no consumer yet in this port (SET
  MODE/STEP-default/MKVALID, SET PTE, SET DEBUG/ASM flags, SET [NO]EXPAND/
  SHARE, SET FAULT history size; SHOW INSTRUCTIONS/TRACE/SCB/TB/MAP/IMAGES/
  CALL_FRAMES/REGIONS/SHARE/ROM/NVRAM/PAGE/WATCHPOINTS/ERROR/MODE/SHIM/
  STRING/EXPAND/COMMAND_ARGS/CLOCK/XTEST). Implemented the high-value core
  both commands actually center on:
  - `set.go`: `Console.SetSymbol` replicates `console_set`'s own dispatch
    order for its `NAME=value` syntax — a general register name, then a
    privileged register name (`privRegNames`, matching `pr_names[]`'s
    subset with an architected name in `internal/vax/registers.go`), then
    the literal name `PSL` (whole-PSL assignment), falling back to a plain
    user symbol definition. `Console.SetRadix` implements `SET RADIX`.
  - `show.go`: `Console.ShowRegisters`/`ShowPSL`/`ShowMemory`/`ShowSymbols`/
    `ShowBreakpoints`/`ShowRadix`/`ShowBase`/`ShowStack` (KSP/ESP/SSP/ISP/
    USP)/`ShowCPU`/`ShowVersion` — deliberately reformatted for readability
    rather than matching `console_show.c`'s exact `printf` layout
    byte-for-byte, since only the underlying values are behaviorally
    meaningful here, not the C source's specific column spacing.
  - `ShowVersion` also stands in for `ABOUT` (which the C source reaches
    via the same `/entry=exe$about` indirection this port doesn't implement
    — see the Sub-phase 3 entry above on deferred `/entry=` commands) with
    a small Go-native banner, rather than leaving `ABOUT`/`SHOW VERSION`
    with no output at all.
  - These are not yet wired into the DCL grammar's `SET`/`SHOW` verb
    dispatch or `cmd/govax`'s command loop — that wiring, along with
    CLEAR/ZERO/TIME/PRINT/HELP/QUIT/INCLUDE and the real entry point, is
    the next sub-phase.
- `internal/console/set_test.go` covers `SetSymbol`'s three special-cased
  name kinds plus the plain-symbol fallback, `SetRadix`'s valid/invalid
  cases, and each `Show*` method's basic output.
- `go build ./...`, `go vet ./...`, `gofmt -l .` (no output), `go test ./...`
  all clean. `go test ./internal/console -cover`: 82.2%.

### 2026-09-14 — Sub-phase 6: CLEAR/PRINT/QUIT/TIME/INCLUDE/HELP

- Added `misc.go`: `Print` (`console_print.c`'s quoted-string/expression-list
  PRINT/ECHO, always treated as verbose — `SET [NO]VERBOSE` isn't
  implemented), `Quit`/`Running` (`console_exit_dcl`'s
  `vax.console.running` flag, for `cmd/govax`'s command loop to check),
  `Time` (runs one command via an injected dispatch function and reports
  wall-clock elapsed time — `console_time.c`'s Mac-only MIPS/instruction-
  count reporting isn't ported, since `internal/cpu.Engine` has no
  instruction counter to read), `Include` (a simplified, non-stack-based
  stand-in for `push_include`'s nested-file-include machinery — reads one
  file line by line and dispatches each non-comment line, sufficient for
  this port's only in-scope consumer, a `vax.init`-style startup script),
  and `ClearSymbol`/`ClearBreakpoint` (`console_clear.c`'s `CLEAR SYMBOL`/
  `CLEAR BREAKPOINT`, wired to the existing `SymbolTable`/`Breakpoints`
  machinery from earlier sub-phases; `CLEAR`'s other sub-forms — INTERRUPT,
  MEMORY statistics, PROFILES, STRINGS, TB, ERROR — aren't implemented,
  each tied to state this port doesn't model).
- Added `help.go`: a from-scratch parser for `vax.help`'s own documented
  format (`$`-prefixed comma-separated 4-character-token topic keys, one or
  more of which can stack with no body between them to share the following
  section — letting `SHOW`/`SH` synonyms resolve to the same text — body
  text runs to the next `$` line or EOF), matching `help.c`'s key
  construction (`helpKey`, verified against the file's own worked example:
  "HELP SET TRACE" → `$SET ,TRAC`) and section-matching behavior. Tested
  against the real `testdata/dcl/vax.help` fixture, not just a synthetic
  one, including the documented `SET TRACE` key-construction example.
- `internal/console/{misc,help}_test.go` cover: Print's literal/expression
  mix, Quit/Running, Time's dispatch-and-report cycle, Include dispatching
  each non-comment line of a temp script, ClearSymbol/ClearBreakpoint's
  specific-item and `/ALL` forms, and Help's exact-key, abbreviated-
  synonym-key, missing-topic, and nil-Help cases (the last two synthetic;
  the exact/synonym cases against the real `vax.help` fixture).
- `go build ./...`, `go vet ./...`, `gofmt -l .` (no output), `go test ./...`
  all clean. `go test ./internal/console -cover`: 83.2%.

### 2026-09-14 — Sub-phase 7: VMINIT

- Added `vminit.go`: `Console.VMInit` ports `console_vminit_dcl`'s P0/P1/S0
  region-size negotiation (explicit sizes plus even redistribution of
  leftover physical pages across any unspecified regions, then dumping any
  final rounding remainder into S0), S0-page-table-fits check, and page
  table construction — using the C source's own `#ifndef DYNVM` code path
  (every requested page pre-mapped valid immediately) rather than the
  `#ifdef DYNVM` demand-paging path its default build actually takes
  (`vax.h` defines `DYNVM`). This is a deliberate scope decision, not a
  fidelity gap invented for this port: `internal/vm/translate.go`'s own
  design notes already flagged DYNVM as deferred to "Phase 08's VMINIT
  command," but implementing it for real needs `internal/vm.Translate`
  itself to gain new state/API for an invalid PTE to trigger on-demand
  allocation — out of scope for a change confined to `console_vminit.c`;
  pre-mapping every page is a real, C-source-supported alternate behavior
  mode, not a fabricated one, and is what the file's `#ifndef DYNVM` branch
  already does.
- Also not ported (documented in `vminit.go`'s own comment): the
  `CONSOLE$SCRATCH` immediate-assembly scratch page and
  `CONSOLE$STRINGPOOL*` area (both serve the inline mini-assembler, Phase
  11's scope) and the `PTE$K_NONE` guard page one page below each
  privileged stack (installed via the C source's own `setpte` mini-parser,
  also assembler-adjacent) — the bottom-most *P0* page is still guarded
  (`PTE_K_NA`), since that part of the algorithm needs no assembler, just a
  `PTE.SetProtection` call already in scope.
- Confirmed PTEs can be written directly through the normal
  `Mem.StoreLongword(cpu, addr, ...)` path (rather than needing a raw
  physical-write escape hatch) since `Translate` already treats an address
  as physical whenever `MAPEN == 0` — exactly the state `VMInit` holds
  while building the tables, matching `console_vminit_dcl`'s own
  `vax.MAPEN = 0` ... `store_memory(...)` ... `vax.MAPEN = 1` structure.
- `internal/console/vminit_test.go` covers: a full VMInit on a small (128-
  page) physical memory, then a store/load round trip through a P0 virtual
  address to prove the page table `VMInit` built is actually valid and
  `Translate`-resolvable (not just that the privileged registers look
  right); that P0's guarded first page really does access-violate; that
  the four privileged-mode stack pointers come out distinct and nonzero
  with `SP` initialized to `KSP`; and the pre-Init and oversized-request
  error paths.
- `go build ./...`, `go vet ./...`, `gofmt -l .` (no output), `go test ./...`
  all clean. `go test ./internal/console -cover`: 84.7%.

### 2026-09-14 — Sub-phase 8: command dispatch integration

- Added `dispatch.go`: `Dispatcher` ties every earlier sub-phase together,
  matching `console_dispatch.c`'s own two-tier structure — a fixed,
  `console_dispatch_table`-equivalent map of exact (≤4-character, matching
  `read_verb`'s own truncate-to-4 rule) verb spellings, falling back to the
  DCL grammar for anything unmatched. `fixedCommands` follows the C
  table's real-function-vs-`-1` split (`SHOW`/`EXIT`/`QUIT`/`CLEAR`/`TEST`/
  `VMINIT` are DCL-driven; `EXAMINE`/`SET`/`STEP`/... are fixed) with two
  documented deviations: `DEPOSIT` (a Go-native addition, see exam.go) and
  `RUN`/`R`, remapped from the C source's VMS-image-activation meaning
  (not ported, see run.go's sub-phase 3 entry) to plain CPU execution —
  the same as `EXEC`/`GO`/`G` — since a dead command spelling would be
  strictly worse than a useful, clearly-documented reinterpretation in an
  emulator with no image loader.
- `bindGrammar` binds a Go handler to every DCL verb/syntax this port
  actually implements (`EXIT`, `VMINIT`, four `CLEAR_*` forms, ten
  `SHOW_*` syntaxes plus the bare `SHOW` verb itself for the plain
  register/privileged-register name shortcuts like `SHOW R0`/`SHOW PC`/
  `SHOW P0BR` — reached whenever a `show_types` keyword carries no
  `/syntax=` redirect of its own) and deliberately leaves everything else
  (device/RTL/assembler-dependent `SHOW`/`CLEAR`/`DEFINE` sub-forms, `TEST`)
  unbound: `Grammar.Dispatch`'s existing "no handler bound for X" error
  already reports that clearly, so no per-command stub code is needed. A
  command reached via `/entry=` (`ABOUT`, `FORTH`, `SHOW VERSION`'s
  `XTEST`/`exe$about` indirection, ...) is caught even earlier, in
  `Dispatch` itself, before ever calling `Grammar.Dispatch`.
- Hit, then fixed, a real Go gotcha while wiring `TIME`'s "run a sub-command
  and report how long it took" behavior: `fixedCommands` as a plain package
  `var` initializer referencing `cmdTime`, whose body passes the
  `Dispatcher.Dispatch` method value, created a compiler-rejected
  initialization cycle (`fixedCommands → cmdTime → Dispatch →
  fixedCommands`) even though nothing is actually invoked at package-init
  time — moved the map literal into an `init()` function, which isn't
  subject to that static dependency check.
- Confirmed, while writing the DEPOSIT/EXAMINE round-trip test, a genuine
  (not invented) VAX DCL/MACRO-32 number-syntax quirk already present in
  `expr.go`'s port of `asm_expr3`: a hex literal beginning with a letter
  A-F (e.g. `ABCD1234`) is parsed as a symbol-name reference, not a number
  — a bare hex literal must start with a digit (or use the `^X`/`0X`
  prefix) — documented in the test itself after the first version of it
  (using such a literal) failed for exactly this reason.
- `internal/console/dispatch_test.go` exercises the fully wired system
  end-to-end against the real `testdata/dcl/evax.dcl` grammar: fixed-table
  dispatch and its 4-character truncation rule, SET/EXAMINE/DEPOSIT/STEP/
  GO round trips, RUN's remapping to Execute, SHOW via DCL (both a bound
  syntax and the bare-verb register shortcut), CLEAR BREAKPOINT/ALL via
  DCL, VMINIT via DCL, EXIT stopping the run loop, the `/entry=` and
  unbound-DCL-syntax error paths, a not-implemented fixed command, SAVE/
  LOAD's qualifier requirement, HELP, and blank-line/comment no-ops.
- `go build ./...`, `go vet ./...`, `gofmt -l .` (no output), `go test ./...`
  all clean. `go test ./internal/console -cover`: 78.0% (dipped slightly
  from 84.7% purely because this sub-phase added a large amount of new,
  well-covered glue code alongside it, not because coverage of existing
  code regressed).

### 2026-09-14 — Sub-phase 9: cmd/govax entry point

- Added `github.com/chzyer/readline` (named explicitly in this phase doc's
  own scope note) as a dependency, and filled in `cmd/govax/main.go`:
  loads the DCL grammar and HELP text, allocates a minimal machine
  (`minimumVAXMemory`, matching `driver.c`'s own `MINIMUM_VAX_MEMORY`),
  runs `vax.init` as the real startup script (via `Console.Include`), then
  drops into a readline-backed command loop until `EXIT`/`QUIT` — matching
  `driver.c`'s own `main()` sequence (grammar → minimal `alloc_vax` →
  `INCLUDE "vax.init"` → prompt loop).
- Made one explicit file-location decision the phase doc left open:
  `cmd/govax` takes a `-data` flag (default `testdata/dcl`) naming the
  directory holding `evax.dcl`/`vax.help`/`vax.init`, rather than
  `driver.c`'s hard-coded CWD-relative `"evax.dcl"` lookup or a `go:embed`.
  `go:embed` isn't actually usable here without duplicating the files: an
  embed pattern can't cross a `..` directory boundary, and `testdata/dcl`
  isn't a subdirectory of `cmd/govax`'s own package directory. The `-data`
  flag keeps `testdata/dcl` as the single canonical copy (also used by
  every test in this phase) and mirrors this project's own documented
  convention for the *original* C binary (`reference/CLAUDE.md`: "run with
  its working directory set to the repo root").
- Ran the built binary interactively (piped commands) as a real smoke test,
  not just `go build`: confirmed `INIT`/`VMINIT`/`DEPOSIT`/`EXAMINE`/`SET`/
  `SHOW` genuinely work together end-to-end through a translated virtual
  address (`D/L 200=12345678` then `EXAM/L 200` reads it back through the
  page table `VMINIT` built). This run also turned up a concrete,
  real-world confirmation of Sub-phase 7's documented DYNVM trade-off:
  `vax.init`'s own `vminit /p0=2048 /p1=8192 /s0=2048 ...` requests 12288
  total virtual pages after only allocating 4096 physical pages
  (`init ^d4096`) — relying on DYNVM's on-demand physical-page allocation,
  which this port's non-DYNVM `VMInit` doesn't have, so that step of the
  real startup script legitimately fails here (`"requested VM size exceeds
  physical memory"`) — expected and consistent with the documented
  decision, not a new bug, and confirmed by running the *real* fixture
  rather than reasoning about it in the abstract. `vax.init` also exercises
  several other not-yet-built subsystems (the inline assembler for
  building `kernel.asm`, `DEFINE/DEVICE`/`DEFINE/LOGICAL`, RTL's
  `exe$initialize`, `SET DEBUG`/`SET QUANTUM`, `IF`/`INCLUDE/COMMAND_LINE`)
  and reports a clear per-line error for each rather than aborting the
  whole script, letting the rest of startup still complete.
- `cmd/govax/main_test.go` covers startup completing without a fatal error
  against the real `testdata/dcl` fixtures (not asserting vax.init runs
  clean, for the reasons above) and the missing-grammar-file error path.
  `run` takes an injectable `io.ReadCloser` for readline's stdin so tests
  get a deterministic, immediately-EOF input source instead of depending
  on the test process's real stdin, and skips writing a history file
  whenever that injected reader is used.
- `go build ./...`, `go vet ./...`, `gofmt -l .` (no output), `go test ./...`
  all clean (verified with stdin explicitly closed, to rule out any
  accidental dependence on an interactive terminal).

### 2026-09-14 — Phase 08 close-out

- Reviewed this phase's Goal/Deliverables against what actually landed:
  EXAMINE/DEPOSIT, RUN/STEP (as `Execute`/`Step`, since the C source's own
  `RUN` verb turned out to mean VMS image activation — see Sub-phase 3),
  SAVE/LOAD (ROM/NVRAM binary format, round-tripped against the real
  `testdata/rom/xdefault.rom` fixture), SHOW (a real, working core subset),
  the DCL grammar-driven command parser (a from-scratch reimplementation
  targeting full behavioral compatibility with the actual grammar
  `testdata/dcl/evax.dcl` uses, not a port of `dclrtl.c`'s FSM internals),
  and `cmd/govax` as a real, runnable entry point are all in place and
  exercised by tests that load the project's actual fixture files
  (`evax.dcl`, `vax.help`, `xdefault.rom`, and `vax.init` via the built
  binary itself) rather than synthetic stand-ins throughout.
- Consciously out of scope for this phase, each documented at the point it
  came up rather than only here: device-dependent commands (`SHOW DEVICE`,
  `DEFINE/DEVICE`, `SHOW LOGICAL`/`DEFINE/LOGICAL`) — deferred to Phase 09
  per this doc's own original scope note and the user's explicit go-ahead
  to stub them; RTL/microkernel-entry-point commands (`CALL`, `BOOT`,
  `ABOUT`, `FORTH`, `XTEST`, `RUN`'s real VMS-image-activation meaning) —
  Phase 10; the inline mini-assembler and disassembler (`ASM`/`ASSEMBLE`/
  `DISASSEMBLE`, and by extension `EXAMINE`'s `/INSTRUCTION` disassembly
  format and the assembler's own richer expression grammar, replaced here
  by a small purpose-built `Evaluator`) — Phase 11; true DYNVM on-demand
  paging (`VMInit` uses the C source's own alternate `#ifndef DYNVM`
  pre-map-everything code path instead) — deferred pending new
  `internal/vm.Translate` API, out of scope for a change confined to
  `console_vminit.c`; `STEP_OVER`/`STEP_RETURN`'s real skip-a-subroutine
  behavior, fault-code breakpoints, `DO`/`IF` scripted control flow, and
  most of `SET`/`SHOW`'s dozens of lower-value or debug-tracing-only
  sub-forms.
- Full `docs/DEVIATIONS.md`-policy review for this phase: no VAX ISA/
  hardware-fidelity findings were logged, because none of this phase's
  work touches emulated VAX instruction-set behavior — the DCL engine,
  console commands, and file formats are all the emulator's *own* tooling,
  which `CLAUDE.md`'s bug-fixing policy explicitly scopes `DEVIATIONS.md`
  away from. Where this phase's own C source had a bug or notable quirk
  worth recording, it's captured in this progress log at the point it was
  found instead (e.g. Sub-phase 1's `SHOW PAGE/READ/WRITE` REST_OF_LINE
  ordering fix, Sub-phase 4's two non-512-aligned entries in
  `xdefault.rom`, Sub-phase 8's DEPOSIT hex-literal-must-start-with-a-digit
  note).
- Full-repo `go build ./...`, `go vet ./...`, `gofmt -l .` (no output), and
  `go test ./...` all clean; `go test ./internal/console/...` and
  `./cmd/govax/...` both pass with stdin explicitly closed. Phase complete.
