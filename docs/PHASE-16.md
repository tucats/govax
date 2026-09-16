# Phase 16: Console command fit-and-finish (SHOW / CLEAR / SET gaps)

## Goal

Catalogue and then implement the console commands
`reference/eVAX/eVAX/Source/Console/console_show.c`, `console_clear.c`, and
`console_set.c` support that this Go port (`internal/console/show.go`,
`dispatch.go`'s `bindGrammar`, `set.go`) doesn't yet — fit-and-finish work
against the SHOW/CLEAR/SET command surface, plus the handful of other missing
console mechanisms found along the way. Started from a `SHOW`-only ask (`SHOW
PAGE`, `SHOW QUANTUM`, `SHOW TB`, `SHOW CALLS`, etc. were named as examples) and
broadened, per the user's own mid-task direction, to catalogue every other missing
console command found along the way (`CLEAR`, `SET`, and a few structural gaps
like the DCL `/entry=` redirect mechanism).

**Status: in development.** Started as an audit-only phase (2026-09-15: "document,
do not implement") — the inventory below was written before any of it was
implemented, and is kept as originally written (findings, "recommend"s, open
questions and all) rather than rewritten after the fact, so it still reads as a
plan. The Progress Log at the bottom is the authoritative record of what has
actually landed; treat an inventory item above it as done only once the Progress
Log says so, not because the prose reads as settled.

## Scope / C source mapping

- `reference/eVAX/eVAX/Source/Console/console_show.c` — `console_show_dcl`, the `SHOW`
  command dispatcher (a `switch` on the DCL syntax id), plus its helper routines
  (`show_regions`, `show_calls`, `showscb`, `show_instructions`, `show_modes`,
  `printbit`).
- `reference/eVAX/eVAX/Source/Console/console_clear.c` — `console_clear_dcl`, the
  `CLEAR` command dispatcher.
- `reference/eVAX/eVAX/Source/Console/console_set.c` — `console_set` (**not**
  DCL-grammar-driven — see "Other missing console mechanisms" below), a hand-parsed
  `CHAR4`/`read_verb`-based dispatcher for `SET`.
- `internal/bootdata/files/evax.dcl` (== `testdata/dcl/evax.dcl`, a copy of
  `reference/eVAX/evax.dcl`) — the shared DCL grammar; its `show_types`/`clear_types`
  keyword tables and the `show`/`clear` verb's `syntax` sub-blocks are the
  authoritative list of what a real `SHOW`/`CLEAR` command line can even parse to.
  `SET` has no grammar entry at all (see below).
- `internal/console/show.go`, `internal/console/set.go`, `internal/console/dispatch.go`
  (`bindGrammar`, `fixedCommands`) — the current Go implementation being audited.

## How this document is organized

Each subtask below gives: the command as a user would type it, the C source location,
current Go status, expected behavior, and any dependency that blocks real
implementation (most commonly Phase 14, not started — interrupt/timer/quantum
delivery). Items are grouped by readiness so a future pass can pick low-hanging fruit
first.

---

## Sub-phase 1: Missing `SHOW` commands

### 1a. Ready to port now — no blocking dependency

These have all the underlying Go state already (`Console`/`vm.Memory`/`vax.CPU`
fields exist); the gap is purely a missing `show.go` function + grammar binding.

- **`SHOW NVRAM`** (`show_nvram`, C: `console_show.c:288`) — print the loaded NVRAM
  file name and `Console.NVRAM`/`NVRAMBase`/`NVRAMEnd` (already fields on `Console`,
  populated by `rom.go`'s `LoadNVRAM`). Trivial port; "No NVRAM initialized" when
  `NVRAM == nil`.
- **`SHOW ROM`** (`show_rom`, C: `console_show.c:306`) — same shape as `SHOW NVRAM`
  but for `Console.ROM`/`ROMBase`/`ROMEnd`. Note: the C `show_rom` case handler is
  bound to DCL id `154`, which the grammar's `show_types` table maps from the
  `rom` keyword — straightforward.
- **`SHOW MODE`** (`show_mode`, C: `console_show.c:343`) — one line, `"Current MODE is
  %s"` using `PSL.CurMod()` (already exposed, used by `ShowPSL`) against the same
  `{"KERNEL","EXEC","SUPER","USER","INTERRUPT"}` name table.
- **`SHOW SHIM`** (`show_shim`, C: `console_show.c:348`, body is `shim_dump()`) — dump
  `internal/console/shim.go`'s `shimTable` (already exists, built from `kernel.asm`'s
  `.shim` pseudo-op table) and, for each entry, whether `internal/rtl.ShimTable` (the
  real numeric dispatch) has a live handler for its `code`. C's `shim_dump` lives in
  the RTL/shim source, not shown above — read it before implementing to match its
  exact column layout, but the underlying data (`shimTable`, `rtl.ShimTable`) is
  already present in this port, so no new state is needed.
- **`SHOW STRING`** (`show_string`/`string_pool`, C: `console_show.c:216`) — walks a
  linked list of string descriptors in *emulated VAX memory*, rooted at the VAX
  symbols `CONSOLE$STRINGPOOL`/`_BASE`/`_SIZE` (set up by a booted `kernel.asm`, not a
  host-side Go structure). Portable now with `Console.Symbols.Get` + `vm.Memory` reads
  — no blocking dependency — but only produces useful output after `VMINIT` +
  assembling/running `kernel.asm`, same as the C version. `internal/console/vminit.go`
  already has a comment flagging `CONSOLE$STRINGPOOL*` as deferred to "Phase 11's
  scope" (now complete) — re-check that comment when this is picked up.
- **`SHOW PAGE`** / **`SHOW PTE`** (`show_page`, C: `console_show.c:357`, body is
  `tracevm()`) — given an address (and optional `/READ` or `/WRITE`), walk the page
  table and print the resulting `PTE` (valid/protection/modified/owner/PFN). This
  port's `internal/vm.PTE` type (`internal/vm/pte.go`) already exposes every field
  `tracevm` would print (`Valid()`, `Protection()`, `Modified()`, `Owner()`,
  `Software()`, `PFN()`) — the gap is a console-facing walk-and-print function, not
  missing VM state. Needs `internal/vm` to expose (or this command to duplicate) the
  P0BR/P1BR/SBR-relative PTE lookup `Translate` already does internally.
- **`SHOW SCB`** (`show_scb`, C: `console_show.c:1003`, body `showscb()`) — dump the 64
  SCB vector-table entries from `SCBB` (already a `vax.PrivReg`), decoding each
  slot via a static vector-name table (transcribe C's `vector_name[]`, C:
  `console_show.c:1676-1740`) and, when `MKVALID`-equivalent state allows, resolving
  the vector address back to a symbol name (`Console.Symbols` linear scan, matching
  C's `find_label`). No blocking dependency — `Engine`/`Mem`/`CPU.PR(vax.SCBB)` are
  all already available.
- **`SHOW CALL_FRAMES`** / **`SHOW CALLS`** (`show_call_frames`, C:
  `console_show.c:409`, body `show_calls()`) — walk the `CALLS`/`CALLG` frame chain
  from `FP`, printing handler/mask/SPA/calltype/saved-AP/saved-FP/saved-PC/argument
  list per frame, optionally bounded by a count parameter. The frame layout this
  walks is the same one Phase 07's `CALLS`/`CALLG`/`RET` emulation
  (`internal/cpu/emul_call.go` or equivalent) already builds and consumes — reuse its
  field offsets/constants rather than re-deriving them. No blocking dependency.
- **`SHOW REGIONS`** (`show_regions` keyword, DCL id `162`, C:
  `console_show.c:1029`) — prints P0/P1/S0 image-activation region limits via
  `get_region_size`. **Do not confuse with** the C helper function also named
  `show_regions()` (`console_show.c:1324`, a different routine used internally by
  `SHOW MEMORY`, DCL id `138` — **done, see the 2026-09-15 SHOW MEMORY progress log
  entry below**; this note originally said "see 1b below", but 1b never actually
  covered it) — same name, two unrelated dumps, a
  real gotcha when cross-referencing the C source. This one is straightforward:
  Phase 13 already added `Console.RTL.RegionSize` (used by `image.go`'s loader), so
  the data this command needs already exists.
- **`SHOW SHARE_PREFIX`** (`show_share`, C: `console_show.c:1015`) — one line
  reporting `Console.SharePrefix` (already a field, set by `image.go`'s sharable-image
  search — see `docs/PHASE-15.md`'s follow-up notes on `findImage`), including the
  literal-ESC "recursive loading disabled" special case.
- **`SHOW IMAGES`** (`show_images`, C: `console_show.c:1008`, body
  `dump_icb_list()`) — walk `Console.ICBList` (already exists, Phase 13), one line per
  loaded `ICB` (name, base, flags), with `/FULL` adding per-image detail (ISD/SHR
  dependency list — `ICB.Deps`/`SHR` types already exist in `image.go`).
- **`SHOW SYMBOL <name>`** single-symbol form (`show_sym`, DCL id `149`, C:
  `console_show.c:1088`) — **fidelity note, not just a gap**: the grammar's `SHOW
  SYMBOL <name>` syntax (id `149`) takes a `symbol` parameter and is meant to print
  *just that one symbol* (value + attribute flags: perm/entry/label/local/string).
  `dispatch.go` currently binds `SHOW_SYM` unconditionally to `Console.ShowSymbols()`
  (dump *everything*), ignoring the `symbol` parameter entirely — so `SHOW SYMBOL FOO`
  today prints the whole table instead of just `FOO`. Worth a real single-symbol
  lookup path (`Console.Symbols` already supports point lookups), reusing
  `ShowSymbols`'s existing kind-label logic for the attribute string. **Update
  2026-09-16:** the single-symbol lookup path (`Console.ShowSymbol`) and the "entry"
  attribute both now exist — see `docs/PHASE-11.md`'s 2026-09-16 entry.
  "perm"/"label"/"local"/"string" remain unimplemented.
- **`SHOW SYMBOL/SYSTEM`** (id `151`) — `Console.Symbols` already distinguishes
  `SymbolSystem` from `SymbolUser` (`show.go`'s existing `ShowSymbols` kind logic) —
  just needs a filtered variant and a grammar binding for the `/SYSTEM` qualifier.
- **`SHOW COMMAND_ARGS`** (`show_command_args`, C: `console_show.c:1059`) — reports
  the `CONSOLE$ARG_FILE`/`CONSOLE$ARG_COUNT`/`CONSOLE$ARG_n` symbols set up when
  `govax` is invoked with a command script + arguments. Portable via
  `Console.Symbols` once it's confirmed `govax`'s startup path (`cmd/govax/main.go`)
  actually defines these symbols the way `kernel.asm`'s VMS-side callers expect —
  check that before implementing, since this is the one item in this group whose
  upstream data source hasn't been directly confirmed to exist yet.
- **`SHOW EXPAND`** (unnamed keyword `expand`, DCL id `188`, C:
  `console_show.c:1123`) — one line, "Command line expansion is enabled/disabled",
  gated on a `CONSOLE_EXPAND` flag bit. Trivial once/if `SET EXPAND`/`NOEXPAND` (see
  Sub-phase 3) adds the underlying state — small, self-contained pair.

### 1b. Blocked on Phase 14 (interval timer / interrupt delivery, not started)

- **`SHOW QUANTUM`** (`show_quantum`, C: `console_show.c:425`) — prints
  `vax.quantum.{initial,current}` and `vax.uiquantum.{initial,current}`. Phase 14's
  own scope doc (`docs/PHASE-14.md`) is where the "quantum" counter mechanism itself
  gets ported to Go; this port has no `Console`/`cpu.Engine` field for it yet. Blocked
  until Phase 14 lands.
- **`SHOW CLOCK`** (`show_clock`, C: `console_show.c:1128`) — partially blocked: the
  `ICR`/`NICR` privileged-register values it prints are already accessible
  (`privRegNames["ICR"]`/`["NICR"]`/`["TODR"]` exist in `set.go`), but the "clock is
  currently running" and quantum-initial/current lines need Phase 14's clock-running
  state. Could land in two pieces (register dump now, quantum/running status after
  Phase 14) if that's preferred over waiting for the whole thing.
- **`SHOW FAULT`** / **`SHOW EXCEPTIONS`** / **`SHOW FAULTS`** (`show_fault`, C:
  `console_show.c:941`) — two independent halves: (1) a fault/exception *event
  history* ring buffer (`show_faults()` in `interrupt.c`, sized by `SET
  FAULT/HISTORY` — see Sub-phase 3), which only needs Phase 07's already-working
  exception/`HandleFault` machinery (`internal/cpu/handlefault.go`) plus a new
  ring-buffer recorder — **not** blocked on Phase 14; and (2) the pending-interrupt
  queue dump (`vax.interrupt_pending`, `vax.iqueue`), which has no Go equivalent
  until Phase 14 builds the interrupt-admission/delivery mechanism. Recommend
  splitting: the fault-history half can be a Phase 16 deliverable on its own, the
  pending-interrupt half stays blocked.

### 1c. Blocked on other missing state this same phase should add (see Sub-phases 2-4)

- **`SHOW WATCHPOINTS`** (`show_watchpoints`, C: `storage.c:96`) — needs the
  watchpoint subsystem itself (no Go equivalent of C's `struct WATCHPOINT`/
  `add_watchpoint`/`delete_watchpoint`/`watchpoint_hit`) — see "Other missing console
  mechanisms" below. `SHOW WATCHPOINTS` is the trivial list-and-print half once that
  exists.
- **`SHOW STEP_MODE`** — **done, see `docs/PHASE-18.md`**, extracted into its own
  phase (2026-09-15, at the user's request) rather than staying here: the work
  touches the CPU engine (a call-like-instruction classifier, a one-shot internal
  breakpoint mechanism), not just this console command surface.
- **`SHOW DEBUG`** — **done, see `docs/PHASE-17.md`.** That phase also
  corrected two assumptions this entry made before any of it was
  implemented: `SET DEBUG`/`SET NODEBUG` turned out to be one verb with
  per-item `NO`-prefixing, not a verb pair, and `SHOW DEBUG` displays 20 of
  the 25 settable names, not all of them.
- **`SHOW ASSEMBLER_FLAGS`** (`show_assembler_flags`, C: `console_show.c:567`) —
  needs `SET ASSEMBLER`'s flag set (`ADDRESS_PROMPT`, `FORWARD_WARNINGS`,
  `BRANCH_DESTINATION`, `SYMBOLS`, `RESOLVE_TEMP`, `SCOPENAMES`). Check whether
  `internal/asm` already models any of these as compile-time behavior before assuming
  all six need new runtime state — worth a design pass rather than a blind port,
  since Go's assembler (Phase 11) may not need some of these the way the C REPL did.
- **`SHOW TRACE`** / **`SHOW DISASSEMBLY`** (`show_trace`, C:
  `console_show.c:993`) — needs `SET TRACE`/`NOTRACE` (execution-trace-disassembly
  toggle) and, secondarily, the already-existing `DBG_REGISTERS` bit from `SET DEBUG`
  above. No `Console` field for the trace toggle currently exists.
- **`SHOW SYMBOL/TEMPORARY`** and **`SHOW SYMBOL/UNRESOLVED`** (ids `152`/`153`) —
  `internal/console/symbols.go`'s `SymbolKind` only has `SymbolUser`/`SymbolSystem`;
  there's no "temporary" kind (assembler-local scratch symbols, `.`-prefixed in the
  C source) or forward-reference/unresolved tracking. Whether these map onto
  anything `internal/asm`'s Phase 11 symbol handling already tracks internally (vs.
  needing genuinely new state) is worth checking before scoping the work.

### 1d. Existing Go implementations that don't actually match the C command they're named after

These are already "implemented" in the sense that the command runs and prints
something, but a side-by-side check against `console_show.c` found the Go version
answers a **different question** than the C command does. Flagging clearly since
these read as done in `show.go`'s own doc comment but aren't fidelity-equivalent —
worth the user's attention before deciding whether to fix now or track in
`docs/DEVIATIONS.md`.

- **`SHOW BASE`** (`show_base`, C: `console_show.c:832`) — **the C command has
  nothing to do with region base registers.** Its entire body is
  `printf("Next storage address is %08X\n", vax.console.deposit)` — i.e. it reports
  the current `EXAMINE`/`DEPOSIT` cursor (what this port calls
  `Console.DepositAddr`), the same value `SHOW CPU` falls through into printing. Go's
  `ShowBase` (`show.go:98`) instead prints `P0BR`/`P0LR`/`P1BR`/`P1LR`/`SBR`/`SLR` —
  a plausible-sounding but unrelated report, likely written from the name "BASE"
  alone rather than the C source. This isn't a console/DCL quirk to replicate as-is
  (per `CLAUDE.md`'s bug-fixing policy, a clear logic mismatch, not an ISA fidelity
  question) — recommend fixing `ShowBase` to print `Console.DepositAddr` to match C,
  and finding a different command/qualifier (or just leaving unbound) for the
  region-base-register dump Go currently has, since nothing in `show_types` actually
  asks for that report under any keyword.
- **`SHOW KSP`/`ESP`/`SSP`/`ISP`/`USP`** and the bare **`SHOW STACK`** (ids
  `139`-`144`, C: `console_show.c:856-939`) — the C commands dump N longwords of
  **stack memory** starting at the selected mode's `SP` (default 1 word, `/ALL` or a
  count parameter for more; `SHOW KSP`/etc. additionally switch to that mode's stack
  before dumping, then switch back), formatted per the current radix, stopping at
  end-of-stack/end-of-memory. Go's `ShowStack` (`show.go:122`) instead prints only
  the **register value** of the selected stack pointer — again a plausible read of
  the command name, but not what the C command does, and it also means the bare
  `SHOW STACK` keyword (id `139`, current-mode stack, no register-switch) isn't
  bound at all — only the five register-name variants are. Recommend reworking
  `ShowStack` into the real memory-dump behavior (reusing whatever `EXAMINE`'s
  memory-formatting helper already does for per-radix output) and adding the missing
  bare `SHOW STACK` binding, rather than treating this as "done."

### 1e. `SHOW INSTRUCTIONS` qualifiers — partially portable

- **`SHOW INSTRUCTIONS`** (`show_instructions`, C: `console_show.c:612`) has no Go
  binding at all yet. The base form (list every opcode name/value from the
  instruction table) is straightforward — `internal/cpu`'s dispatch table already has
  everything needed (opcode, mnemonic, whether a real handler vs.
  `emul_unimplemented`-equivalent is installed). The qualifiers split further:
  - `/UNIMPLEMENTED`, `/ALL`, a bare opcode filter — mechanical, same table walk with
    a different predicate.
  - `/MODES` (`show_modes()`) — needs the addressing-mode legality table per
    instruction; check whether `internal/cpu`/`internal/asm` already has this
    (Phase 03/11 territory) before assuming it's missing state.
  - `/PROFILE` (`show_instructions(-2)`) — needs per-opcode execution counters. No
    such profiling hook exists in `internal/cpu` today (confirmed: no `Profile`/
    counter field found in `engine.go`/`decode.go`). This is new instrumentation, not
    a port of existing state — worth a design decision on whether it's in scope for
    this port at all (it's a developer/debugging aid, not emulated VAX behavior).

### 1f. Needs a design decision — no clean 1:1 Go analogue

- **`SHOW ERROR`** (`show_error`, C: `console_show.c:326`) — decodes a VAX status
  code to text via `vaxmsg()`, a static VMS-style message-code-to-string table, and
  also reports the last command's `$STATUS` symbol. **Design question resolved**:
  `internal/vmserrors` (added after this entry was written) now gives govax exactly
  the parallel VAX-style status-code space this entry asked for — a `VMSError` type
  encoding facility/message/severity into one `uint32`, a `Messages` table mapping
  each registered code to `vaxmsg()`-style display text, and per-facility catalogs
  (`codes_sys.go`/`codes_cli.go`/`codes_vax.go`/`codes_lib.go`/`codes_rms.go`) —
  and the ad-hoc Go `error` values this port's CLI/command-processing/assembler code
  used to construct via `fmt.Errorf`/`errors.New` have all been converted to it
  (govax-wide sweep, 2026-09-15; see that change's own notes for the facility/
  severity assignments). `SHOW ERROR` itself is still unbound — decoding *a given*
  status code to text is now straightforward (`vmserrors.Messages[code]` or
  constructing a `VMSError{Status: code}` and calling `.Error()`), but reporting the
  last command's own `$STATUS` needs a "last error" slot on `Console` that doesn't
  exist yet. Worth picking up as a follow-on, not part of this resolved question.
- **`SHOW MAP`** (`show_map`, C: `structure_mapping.c:23`, body `map_dump()`) —
  dumps `structure_mapping.c`'s own runtime FAB/RAB field-offset registry
  (`STROFF`/`map()`/`map_add()`). `internal/rtl/rms.go`'s own doc comment
  (`rms.go:16`) confirms this port deliberately did **not** replicate that
  declarative-mapping mechanism — RMS struct fields are read/written directly by
  hardcoded Go offsets instead. There is nothing for `SHOW MAP` to dump in this
  port's architecture; recommend leaving this command unbound (or a one-line "not
  applicable to this port" stub) rather than inventing a registry solely to satisfy
  this command.
- **`SHOW TB`** (`show_tb`, C: `console_show.c:373`, plus `dump_tb()` in `vm.c:176`)
  — reports translation-cache hit/miss/ratio counters and a full page-cache dump.
  `internal/vm.Memory`'s translation (`translate.go`) does a direct page-table walk
  on every access with no caching layer (confirmed: no TB/TLB-style cache or
  hit/try counters anywhere in `internal/vm`). Porting this command as specified
  would mean first adding a translation cache Go's VM implementation doesn't
  currently have and arguably doesn't need for correctness — a design question
  (add a cache purely to have something to report on, vs. report "not modeled,
  translation is uncached" the way the C source's own `#if 0`-disabled
  `SHOW MEMORY STAT` sibling gets skipped in "optimized build") rather than a
  straightforward port.
- **`XTEST`** (id `5001`, entry `exe$xtest`) — not itself a `SHOW` command, but
  listed as a `show_types` keyword; per `reference/CLAUDE.md`'s own description,
  `console_test.c`/`XTEST`/`TEST` are "a grab-bag of low-level, partly-undocumented
  console hooks," not a stable command surface. Low priority; blocked on the
  `/entry=` mechanism regardless (see Sub-phase 4).

---

## Sub-phase 2: Missing `CLEAR` commands

`dispatch.go` currently binds only `CLEAR_SYM_ALL`, `CLEAR_SYMBOLS`,
`CLEAR_BREAK_ALL`, `CLEAR_BREAKPOINT`. `console_clear.c`'s `console_clear_dcl` has 18
cases; the rest are missing:

- **`CLEAR SYMBOL/TEMPORARY`** (id `115`, C: `console_clear.c:203`) — blocked on the
  same missing "temporary symbol" `SymbolKind` noted in Sub-phase 1c.
- **`CLEAR STRINGS`** (id `105`, C: `console_clear.c:114`) — resets the
  `CONSOLE$STRINGPOOL` chain (Sub-phase 1a's `SHOW STRING` data) back to empty.
  Portable alongside `SHOW STRING`.
- **`CLEAR TB`** / **`CLEAR TRANSLATION_BUFFER`** (id `107`, C:
  `console_clear.c:133`) — resets translation-cache state; blocked on the same "no TB
  cache exists" gap as `SHOW TB` (Sub-phase 1f). If that design question resolves to
  "don't add a cache," this command becomes a no-op stub rather than a real port.
- **`CLEAR ERROR`** (id `113`, C: `console_clear.c:144`) — resets `$STATUS`/last-error
  state. `SHOW ERROR`'s status-code design question (Sub-phase 1f) is now resolved
  (`internal/vmserrors`); this command still needs the same not-yet-existing "last
  error" slot on `Console` that entry flags before there's any state here to clear.
- **`CLEAR PROFILES`** (id `104`, C: `console_clear.c:149`) — resets the
  per-opcode profiling counters `SHOW INSTRUCTIONS/PROFILE` would report (Sub-phase
  1e) — blocked on the same missing profiling instrumentation.
- **`CLEAR MEMORY`** (id `103`, C: `console_clear.c:179`) and **`CLEAR
  MEMORY/STATISTICS`** (id `114`, C: `console_clear.c:184`) — the plain form is a
  near-no-op in C (falls through with no real body beyond the statistics case);
  `/STATISTICS` resets memory-access counters gated behind the C build's
  `printmem`/`decc_dump_memory` (`MKVALID`-gated, "not supported in optimized build"
  per `console_show.c:656`'s own comment) — low priority, likely also a design
  question of whether this port tracks memory-access statistics at all.
- **`CLEAR INTERRUPT <id>`** (id `102`, C: `console_clear.c:214`) and **`CLEAR
  INTERRUPT/ALL`** (id `110`, C: `console_clear.c:242`) — operate on `vax.iqueue`,
  the pending-interrupt queue. Blocked on Phase 14, same as `SHOW FAULT`'s
  pending-interrupt half.
- **`CLEAR BREAKPOINT/FAULT <addr>`** (id `108`, C: `console_clear.c:274`) and
  **`CLEAR BREAKPOINT/FAULT/ALL`** (id `109`, C: `console_clear.c:303`) — blocked on
  the missing `BreakFault` breakpoint kind (`internal/console/execute.go`'s
  `BreakKind` doc comment already flags this: "Only `BreakAddress` is implemented by
  this port"). See Sub-phase 4's breakpoint-kinds entry.
- **`CLEAR BREAKPOINT/INSTRUCTION <opcode>`** (id `551`, C:
  `console_clear.c:89`) and **`CLEAR BREAKPOINT/INSTRUCTION/ALL`** (id `553`, C:
  `console_clear.c:65`) — **implemented, see `docs/PHASE-18.md`'s sub-phase 2**
  (`Console.RemoveInstructionBreakpoint`/`ClearAllInstructionBreakpoints`,
  `internal/console/instbreak.go`), not this sub-phase.

---

## Sub-phase 3: Missing `SET` commands

`internal/console/set.go` currently implements only `SET <name>=<value>` (register /
privileged register / PSL / plain symbol) and, via `dispatch.go`'s hand-parsed
`cmdSet`, `SET RADIX`, `SET BREAKPOINT`, and (added by `docs/PHASE-17.md`) `SET
DEBUG`. `set.go`'s own doc comment already lists the remaining gap in one line
("SET PSL <field>=value, SET MODE, SET STEP, SET MKVALID/NOMK, SET PTE, SET ASM
flags, SET [NO]EXPAND/SHARE, SET FAULT ... not implemented"); this sub-phase
expands that into concrete subtasks.

**Structural note:** unlike `SHOW`/`CLEAR`, `SET` is **not** driven by the DCL
grammar in the C source at all — `evax.dcl` has no `verb set` block. `console_set`
hand-parses each sub-command via `read_verb` + `CHAR4` 4-character prefix matching
(the same style `cmdSet`/`cmdRun`/`cmdCall` in `dispatch.go` already use for their own
small syntaxes). Any new `SET` subcommand should extend `cmdSet`'s existing
hand-parsed `switch`, not attempt to add a DCL grammar entry that doesn't exist in
the reference.

- **`SET PSL <field>=<value>`** (C: `console_set.c:254-359`) — per-field PSL setters
  (`CM`/`TP`/`FPD`/`IS`/`CUR_MOD`/`PRV_MOD`/`IPL`/`DV`/`FU`/`IV`/`T`/`N`/`Z`/`V`/`C`).
  `vax.PSL` (`internal/vax`) already has named bit-field accessors for every one of
  these (used by `ShowPSL`) — likely just needs setters alongside the existing
  getters and a `cmdSet` sub-parser. Low risk, no missing state.
- **`SET MODE <KERNEL|EXEC|SUPER|USER|INTERRUPT>`** (C: `console_set.c:393-421`) —
  calls `set_mode_stack`, the same primitive `SHOW KSP`/etc. use in C (Sub-phase
  1d) to switch the active stack pointer by privileged mode. Worth implementing
  alongside the `ShowStack` rework in 1d, since they share the underlying mode-switch
  logic.
- **`SET STEP <OVER|INTO|RETURN>`** — **done, see `docs/PHASE-18.md`**, extracted
  into its own phase alongside `SHOW STEP_MODE` (Sub-phase 1c) for the same reason.
- **`SET MKVALID`** / **`SET NOMK`** (C: `console_set.c:487-497`) — toggles the
  C source's `MKVALID` gate (guards `SHOW STRING`/`SHOW SHIM`/`SHOW IMAGES`/`SHOW
  SHARE_PREFIX`/`SHOW REGIONS`/`SHOW COMMAND_ARGS`, among others). This port has no
  single equivalent flag — worth checking whether an analogous "is a microkernel
  actually booted" signal already exists implicitly (e.g. `Console.ICBList != nil`,
  or `VMInitValid`) before adding a new bool that duplicates existing state.
- **`SET PTE`** / **`SET PAGE`** (C: `console_set.c:498-`, a large block through
  `~1300` with its own sub-switch for `/VALID`, `/PROTECTION`, `/MODIFY`, `/OWNER`,
  `/SOFTWARE`, `/PFN`) — direct PTE field editing, the write-side counterpart to
  Sub-phase 1a's `SHOW PAGE`. `internal/vm.PTE`'s setters (`SetValid`,
  `SetProtection`, `SetModified`, `SetOwner`, `SetSoftware`, `SetPFN`) already exist
  — this is mostly a console-parsing task once the read-side (`SHOW PAGE`) lands.
- **`SET ASSEMBLER <flag>`** / **`SET NOASSEMBLER <flag>`** (C:
  `console_set.c:504-579`) — the six `ASM_*` flags `SHOW ASSEMBLER_FLAGS`
  (Sub-phase 1c) would report; same "check `internal/asm` first" caveat applies.
- **`SET DEBUG`** — **done, see `docs/PHASE-17.md`** (the bitmask lives on
  `internal/vax.CPU`, and most individual flags now have real tracing
  behavior wired too, not just the setter).
- **`SET VM`** / **`SET MAPEN`** / **`SET NOVM`** / **`SET NOMAPEN`** (C:
  `console_set.c:641-661`) — toggles the `MAPEN` privileged register, which already
  exists (`privRegNames["MAPEN"]`) — this may already work today via the generic
  `SET MAPEN=<value>` form in `SetSymbol`; confirm before treating as missing (the
  gap, if any, is only the bare on/off keyword spelling C uses, not the underlying
  state).
- **`SET WATCH <addr>[/BYTE|/WORD|/LONG] [name]`** (C: `console_set.c:662-705`) — see
  Sub-phase 4's watchpoint entry; this is the write side.
- **`SET FAULT/HISTORY <n>`** (C: `console_set.c:706-715`, `set_fault_history`) —
  sizes the fault-event ring buffer `SHOW FAULT`'s history half (Sub-phase 1b) would
  read from — implement together.
- **`SET BREAK[POINT] [/FAULT|/TEMPORARY|/INSTRUCTION] <addr>`** (C:
  `console_set.c:716-845`) — the existing `cmdSet`/`AddBreakpoint` only covers the
  plain-address form. `/INSTRUCTION` is **implemented, see `docs/PHASE-18.md`'s
  sub-phase 2** (`cmdSet`'s `BREAKPOINT`/`BREAK` case, `Console.AddInstructionBreakpoint`).
  `/FAULT` remains blocked on the same missing breakpoint-kind support as its
  `SHOW`/`CLEAR` counterparts (Sub-phase 4); `/TEMPORARY` (a `BreakKind`-adjacent
  one-shot flag) is a smaller, standalone gap worth checking independently.
- **`SET QUANTUM <n>`** (C: `console_set.c:846-862`) and **`SET UIQUANTUM <n>`** (C:
  `console_set.c:863-879`) — blocked on Phase 14, same as `SHOW QUANTUM`.
- **`SET BASE <addr>`** (C: `console_set.c:880-887`) — sets `vax.console.deposit`,
  the cursor `SHOW BASE` should actually be reporting per Sub-phase 1d's finding.
  Likely already partially covered by however `EXAMINE`/`DEPOSIT` update
  `Console.DepositAddr` today — confirm whether a standalone `SET BASE` is a real gap
  or just a missing alias.
- **`SET VERBOSE`** / **`SET VERIFY`** / **`SET NOVERBOSE`** (C:
  `console_set.c:888-900`) — `Console.Verbose`/`Verify` fields already exist
  (`machine.go:41-42`) — likely just missing the `cmdSet` keyword wiring, not missing
  state. Cheap win.
- **`SET RADIX <HEX|DEC|8|10|16>`** (C: `console_set.c:901-927`) — Go's `SET RADIX`
  (`cmdSet`) currently only takes a bare numeric argument (`strconv.Atoi`); the C
  form also accepts the keywords `HEX`/`HEXA`/`DEC`/`DECI` as synonyms for `16`/`10`.
  Small parsing gap, not a missing-state one.
- **`SET TRACE`** / **`SET DISASSEMBLY`** / **`SET NOTRACE`** (C:
  `console_set.c:928-936`) — the write side of `SHOW TRACE` (Sub-phase 1c).

---

## Sub-phase 4: Other missing console mechanisms found during the audit

These surfaced while tracing dependencies above and don't belong to a single `SHOW`/
`CLEAR`/`SET` triad — cataloguing them here since the user asked for missing commands
generally, not only `SHOW`.

- **DCL `/entry=` redirect is entirely unimplemented.** `dispatch.go`'s `Dispatch`
  already detects this case explicitly (`r.EntryPoint != ""`) and returns "requires
  the RTL microkernel (not yet implemented, see docs/PHASE-08.md)" — but that comment
  predates Phase 10/13 (both now complete). Grepping the C reference tree confirms
  `exe$about`, `exe$forth_dcl`, and `exe$xtest` are not native C functions at all —
  they're VAX assembly entry points defined in `kernel.asm` itself (`.entry
  exe$about` at `kernel.asm:824`, `exe$forth_dcl` at `:806`, `exe$xtest` at `:740`,
  confirmed in both `testdata/asm/kernel.asm` and `internal/bootdata/files/kernel.asm`).
  So `/entry=<name>` really means "resolve `<name>` as a VAX symbol (from a booted
  microkernel) and `CALL` it" — the same primitive `internal/console/call.go`'s
  `Console.Call` already implements for the user-facing `CALL` command. Wiring
  `Grammar.Dispatch`'s `EntryPoint` case to look up the symbol and invoke
  `Console.Call` would unblock **`ABOUT`**, **`FORTH`**, and **`XTEST`** (the three
  DCL verbs/syntaxes still stubbed as entry-only) in one shot, without needing new
  RTL/CPU work — this is now purely a console-dispatch wiring gap, not a "Phase 10"
  dependency the way the old doc.go comment frames it. Worth re-reading `doc.go`'s
  header comment and updating it once this phase's findings are acted on.
- **No watchpoint subsystem at all** — tracked as a likely future sub-phase of
  `docs/PHASE-18.md` (the "flow of control" phase STEP/breakpoint work now lives
  under), not this one. C's `struct WATCHPOINT`/`add_watchpoint`/
  `delete_watchpoint`/`watchpoint_hit`/`show_watchpoints` (all in
  `reference/eVAX/eVAX/Source/CPU/storage.c:44-140`) have no Go equivalent anywhere
  (confirmed by grep). This backs `SET WATCH` (Sub-phase 3), `SHOW WATCHPOINTS`
  (Sub-phase 1c), and an implied (ungrammared, like `SET`) `CLEAR WATCH` the C source
  also supports via `delete_watchpoint` — the grammar's `clear_types` keyword list
  doesn't include `watchpoint` either, so this is presumably hand-parsed in
  `console_clear`'s own `CHAR4` dispatch the same way `SET WATCH` is (worth
  confirming against `console_clear.c` directly when this is picked up — it wasn't
  in the case list this audit already extracted, id `1-115`). A watchpoint actually
  *doing* something (breaking execution on a memory write matching a watched
  address/size) also needs a hook into `internal/vm.Memory`'s store path, not just
  the bookkeeping list — check whether `vm.Memory`'s write primitives have room for
  such a hook before scoping this as a pure console-layer add.
- **Instruction-level (opcode) breakpoint mechanism — implemented, see
  `docs/PHASE-18.md`'s sub-phase 2** (`Console.InstructionBreakpoints`,
  `internal/console/instbreak.go`). Rather than adding a `debugdata`-style
  field to `internal/cpu`'s instruction-table entry type as this entry
  originally anticipated, the flag lives entirely in `Console` (a
  `map[*cpu.Instruction]bool`), keeping the instruction table itself
  immutable shared data — see `docs/PHASE-18.md`'s design notes.
- **No fault-kind breakpoints** — also tracked as a likely future sub-phase of
  `docs/PHASE-18.md`. `SET BREAK/FAULT`, `SHOW BREAK/FAULT` (the
  `/FAULT`/`/ADDRESSES` qualifiers on `SHOW BREAKPOINTS` itself, C:
  `console_show.c:698-758`, distinguishing `BREAK_FAULT`-kind entries in its print
  loop), and `CLEAR BREAKPOINT/FAULT[/ALL]` (Sub-phase 2) all depend on
  `internal/console/execute.go`'s `BreakKind` gaining a `BreakFault` value — that
  file's own doc comment already flags this as deliberately deferred. A breakpoint
  that fires on a *fault code* rather than a *PC address* also needs a hook in
  whatever exception-dispatch path `internal/cpu/handlefault.go` runs, similar in
  shape to the watchpoint hook question above.
- **`BOOT`** and **`ROM`** (fixed commands, `dispatch.go:223-224`) are already
  explicitly stubbed (`cmdNotImplemented`) rather than silently missing — flagging
  here only for completeness of this audit's "other missing commands" sweep, not as
  a new finding; no further investigation done on their C-side behavior in this pass.
- **`IF <expr> [THEN] <command>`** (`console_if`, C:
  `console_include.c:174`, dispatch table slot 22 in `initialization.c`'s
  `console_dispatch_table`) — an entire fixed console verb missing from
  `dispatch.go`'s `fixedCommands`, found outside this audit's original SHOW/CLEAR/SET
  sweep (user noticed it while reading `vax.init`, which depends on it directly:
  `IF DEFINED("CONSOLE$ARG_FILE") THEN SET NOVERBOSE`). Before the fix below, that
  line in `vax.init` errored on every `govax` startup (harmlessly swallowed by
  `Console.Include`'s continue-on-error loop, but still wrong).
  **Fixed 2026-09-15**: added `cmdIf` (`dispatch.go`) — evaluates the leading
  expression, optionally consumes a `THEN` keyword (present or absent, matching
  `console_if`'s own check), and recursively dispatches the rest of the line only if
  the expression is nonzero. This needed one supporting piece: the console's own
  small expression evaluator (`expr.go`, a from-scratch stand-in for the real
  assembler's `asm_expr`, see that file's doc comment) had no function-call syntax at
  all, so `DEFINED("SYMBOL")` — the one function `vax.init` actually calls — was
  added there too (`Evaluator.parseDefined`), checking `Console.Symbols` rather than
  the assembler's separate symbol table (`internal/asm/functions.go`'s own
  `DEFINED()`, used by the assembler's unrelated `.IF` pseudo-op — see
  `docs/PHASE-11.md` — is a different, independent implementation over a different
  symbol table; the two were never shared in the C source either, since
  `console_if` and the assembler's `.IF` both call the same underlying `asm_expr`/
  `asm_function` against one global symbol table, a unification this port's
  deliberately-separated console/assembler symbol tables don't replicate).
  **Not fixed, tracked separately**: the conditioned `SET NOVERBOSE` in `vax.init`'s
  own IF line is still a no-op today — `SET VERBOSE`/`NOVERBOSE` remains
  unimplemented, see Sub-phase 3's existing entry for it.

---

## Cross-cutting open questions

- **Sub-phase 1d's two fidelity mismatches (`SHOW BASE`, `SHOW KSP`/`ESP`/`SSP`/
  `ISP`/`USP`/`STACK`) are existing, shipped behavior that doesn't match the C
  reference.** Per `CLAUDE.md`'s bug-fixing policy these read as "clear, obvious
  logic errors not tied to ISA semantics" (console command semantics, not VAX ISA
  fidelity) rather than the "document and defer" ISA-fidelity path — but since they
  were found during an audit-only pass rather than while actively porting the
  surrounding code, flagging them here for the user to decide: fix directly in a
  follow-up change, or track via `docs/DEVIATIONS.md` anyway given they're
  console-layer rather than CPU-layer.
- **The shared `evax.dcl` grammar has a copy-paste typo inherited from the C
  reference**: `show_types`' keyword list defines `p1l4` (`internal/bootdata/files/
  evax.dcl:268`) where every surrounding register name and `privRegNames`'s own
  Go-side map (`set.go:14`, which does have a correct `"P1LR"` entry) both expect
  `p1lr`. Because the grammar itself rejects the keyword `p1lr` before parsing ever
  reaches Go's `ShowRegisterOrPrivReg` fallback, **`SHOW P1LR` cannot work in this
  port today**, matching the same bug's effect on the original C `evax`. Unlike the
  reference C source, this project's `internal/bootdata/files/evax.dcl` is a local,
  editable copy (not the read-only `reference/eVAX/` tree) — worth a one-line typo
  fix (`p1l4` → `p1lr`) independent of the rest of this phase, since it's an obvious
  copy-paste mistake with no ISA-fidelity question attached.
- **Several items above ("Ready to port now") still touch state gated behind other
  not-yet-confirmed assumptions** — e.g. `SHOW COMMAND_ARGS`'s dependency on
  `cmd/govax/main.go` actually defining `CONSOLE$ARG_*` symbols. Each such caveat is
  called out inline above; re-verify before starting rather than assuming this
  document's snapshot is still accurate, especially if other phases land in the
  meantime.

## Progress Log

### 2026-09-15 — Sub-phase 1 implemented (missing SHOW commands)

Landed nearly everything in sub-phase 1a/1b/1d/1e (see per-item detail below);
sub-phase 1c and the `SHOW ERROR`/`XTEST` items in 1f remain genuinely blocked or
deferred, as this document's own inventory already said they'd be — this entry
records what actually shipped, not a re-scope of the plan above.

- **1a, shipped in full** except `SHOW COMMAND_ARGS`/`SHOW EXPAND` (still blocked —
  neither has an underlying data source in this port: no `CONSOLE$ARG_*` symbol
  population at `govax` startup, no `SET EXPAND` state): `SHOW NVRAM`/`SHOW ROM`
  (new `Console.ROMFile`/`NVRAMFile` fields, set by `LoadROM`/`LoadNVRAM`), `SHOW
  MODE`, `SHOW SHIM` (reports each stub's numeric dispatch code and whether
  `internal/rtl.Environment.HasShim` — a new non-invoking predicate — has a live
  handler for it, not a second resolved label the way C's `shim_dump` does; see
  `ShowShim`'s own doc comment for why), `SHOW STRING`, `SHOW PAGE`/`PTE` (new
  `internal/vm.Memory.LookupPTE` read-only PTE walk plus `Protection.Allows`/
  `String`, since a diagnostic display needs to report even a page a real access
  would refuse), `SHOW SCB`, `SHOW CALL_FRAMES`/`CALLS` (decoded against this
  port's own real-VAX-architecture mask-word bit layout — see
  `internal/cpu/call.go`'s `emulRet` — not console_show.c's own `union MASKREG`
  bit-field declaration, a different and irrelevant layout for what this port's
  frames actually contain), `SHOW REGIONS`, `SHOW SHARE_PREFIX`, `SHOW IMAGES`,
  `SHOW SYMBOL <name>` (now a real single-symbol lookup, `ShowSymbol` — previously
  `SHOW_SYM` ignored the `symbol` parameter and dumped everything) and `SHOW
  SYMBOL/SYSTEM`.
- **1b, shipped for real** (Phase 14 landed since this document's original
  audit, which is exactly what these were blocked on): `SHOW QUANTUM`, `SHOW
  CLOCK` (`clock_running` read directly off `ICCS<0>`, matching `vax.h`'s own
  "Copy of ICCS<0>" comment — no separate mirrored field needed), and `SHOW
  FAULT`'s pending-interrupt half (new `Engine.PendingInterrupts`, exposing
  `interrupt_pending`/`iqueue`, now real state since Phase 14). `SHOW FAULT`'s
  *other* half — `show_faults()`'s fault/event history ring buffer — is **not**
  ported: it needs new recorder instrumentation hooked into
  `internal/cpu/handlefault.go` with no existing state to build on (unlike the
  pending-interrupt half), deliberately left for a follow-up per this document's
  own recommendation to split the two.
- **1d, both fidelity mismatches fixed**: `ShowBase` now reports
  `Console.DepositAddr` ("Next storage address is..."), matching the C command;
  `ShowStack` now does a real memory dump from the mode's stack pointer (not just
  the register value), plus the previously-unbound bare `SHOW STACK` form — all
  without adding a live mode-switch primitive: `stackPointerFor` reads
  `GPR(SP)` when the requested mode is the one currently active, otherwise the
  matching privileged register (`KSP`/`ESP`/`SSP`/`USP`/`ISP`), which already
  holds that mode's saved pointer from the last time it was actually live (see
  `internal/cpu/handlefault.go`/`call.go`'s own mode-transition code for where
  that save happens) — same observable result as C's own save/switch/dump/restore
  sequence, no new mode-stack state required.
- **1e, the base form and its non-blocked qualifiers**: `SHOW INSTRUCTIONS`'
  four-per-line opcode/name grid (default and `/UNIMPLEMENTED`), `/ALL`, and the
  opcode filter now all work, backed by two new `internal/cpu.Table` methods
  (`All`, a deterministic single-then-extended enumeration; `Implemented`, the
  has-a-real-handler check). `/MODES` and `/PROFILE` remain unimplemented (return
  an error naming why) — this document's own assessment of them (no addressing-
  mode legality table exposed, no per-opcode execution counters, arguably
  instrumentation rather than emulated VAX behavior) didn't change.
- **1f, the two "recommend a stub" items**: `SHOW MAP` and `SHOW TB` now report
  "not applicable to this port" (RMS field access is hardcoded Go offsets, not a
  runtime registry; `Translate` does an uncached page-table walk) instead of
  either fabricating data or being silently unbound. `SHOW ERROR` remains
  unbound — its design question (adopt a VAX-style status-code space, or shrink
  scope to "print the last error string"?) is still open, per this document's own
  "flag for the user rather than guessing."
- **Found and fixed alongside this work** (a clear, obvious copy-paste mistake
  with no ISA-fidelity question attached, not a new finding — this document's own
  cross-cutting open questions section already named it): `evax.dcl`'s
  `show_types` keyword table had `p1l4` where every surrounding register name
  expects `p1lr`, which made `SHOW P1LR` unparseable; fixed in both copies
  (`testdata/dcl/evax.dcl` and `internal/bootdata/files/evax.dcl`).
- All new/changed behavior has direct test coverage (`internal/vm/translate_test.go`,
  `internal/cpu/instruction_test.go`/`interrupt_test.go`, `internal/rtl/rtl_test.go`,
  `internal/console/show_test.go` and small edits to existing `dispatch_test.go`/
  `set_test.go`); `go build ./...`, `go vet ./...`, `go test ./...` all clean.
- **Deliberately still not done, matching this document's own categorization**:
  sub-phase 1c in full (needs sub-phase 2-4 cross-cutting state: `SET STEP`/
  `DEBUG`/`ASSEMBLER` flags, a watchpoint subsystem, temporary/unresolved symbol
  kinds); `SHOW COMMAND_ARGS`/`SHOW EXPAND` (no data source yet); `SHOW
  INSTRUCTIONS`'s `/MODES`/`/PROFILE`; `SHOW ERROR`; the fault-history-ring-buffer
  half of `SHOW FAULT`. None of these were silently skipped — each was a named,
  reasoned exclusion in this document before this pass started.

### 2026-09-15 — Audit complete, phase created

- Catalogued every `SHOW` sub-display in `console_show.c` (30 `show_types` keywords
  plus the bare-register/priv-reg fallback) against `internal/console/show.go` and
  `dispatch.go`'s `bindGrammar`; found roughly a dozen ready to port with no
  dependency, several genuinely blocked (mostly on Phase 14), a few needing a design
  decision (no clean Go analogue), and two cases where an already-"implemented"
  command doesn't actually match the C command it's named after (`SHOW BASE`, the
  `SHOW *SP`/`SHOW STACK` family).
- Broadened scope mid-task per user direction to also catalogue `CLEAR` (18 C-side
  cases, 4 currently bound) and `SET` (entirely hand-parsed, not DCL-grammar-driven —
  a structural fact worth noting for future work) gaps, plus three cross-cutting
  mechanisms with no Go equivalent at all (watchpoints, instruction-level
  breakpoints, fault-kind breakpoints) and one dispatch-layer gap (`/entry=`
  redirect) that, now that Phase 10/13 are both complete, turns out to be a small
  wiring task rather than the large RTL dependency `doc.go`'s existing comment
  implies.
- No code changes in this phase — documentation/inventory only, per explicit user
  instruction.

### 2026-09-15 — `IF` console verb found and fixed

- While reading `vax.init` (unrelated to this audit's original SHOW/CLEAR/SET scope),
  the user noticed the `IF`/`THEN` console verb (`console_if`) had no Go
  implementation at all — a gap this document didn't originally catalogue since it's
  neither SHOW, CLEAR, nor SET. Catalogued above (Sub-phase 4) and fixed in the same
  pass, since it was small, self-contained, and (unlike this phase's SHOW/CLEAR/SET
  backlog) actually blocks `govax`'s own startup script from parsing cleanly: `cmdIf`
  added to `dispatch.go`'s `fixedCommands`, plus `DEFINED("SYMBOL")` function-call
  support added to the console's own expression evaluator (`expr.go`) to evaluate
  the one call `vax.init` actually makes. `SET NOVERBOSE` (the conditioned command in
  `vax.init`'s own IF line) remains unimplemented — left for Sub-phase 3, unchanged
  by this entry.

### 2026-09-15 — SHOW MEMORY ported to the real `show_regions()`

- `ShowMemory` (`show.go`) previously printed only physical memory size — a
  placeholder, not a port of anything in `console_show.c`. Replaced with a real port
  of `show_regions()` (`console_show.c:1324`, the function that actually backs SHOW
  MEMORY's plain case — see the corrected cross-reference note under Sub-phase 1's
  `SHOW REGIONS` entry above, which had pointed at the wrong section): physical
  memory size/address range, MAPEN-enabled state, "virtual memory configuration is
  unknown" before VMINIT, and — once VMINIT has run — a mapped/free physical-page
  count plus per-region (P0/P1/S0) size/PTE-count/physical-and-virtual-address
  reporting.
- Doing this properly required real physical-page accounting, which this port didn't
  have: the C source's "physical pages mapped" figure only means anything under real
  DYNVM demand-paging semantics (`vm.c`'s `page_map`/`validate_page`/
  `mapped_pages`), not the eager pre-mapping `VMInit` used as a stand-in since Phase
  08. Per user direction, picked that up as part of this same change rather than
  reporting a number that would always just equal every requested page — see Phase
  08's own 2026-09-15 follow-up progress-log entry and `docs/DEVIATIONS.md`'s new
  entry (the C source's own `mapsize` off-by-one, found and deliberately not
  replicated) for the full detail; `internal/vm.Memory` gained `AllocatePage`/
  `ReservePage`/`MappedPages`/`SetVMValid`, and `Translate` now demand-pages an
  invalid P0/P1 (or S0) PTE instead of always faulting TNV.
- `Console.Regions` (`vminit.go`) is new: the Go equivalent of `vax.h`'s `struct
  VMREGION region[3]`, populated by `VMInit` at the same points `console_vminit.c`
  computes each field, and read back by `ShowMemory`.
- Found and fixed in passing, unrelated to the above: `ShowPage`'s `PROT: %02X`
  format on `pte.Protection()` — a type with a `String()` method — triggered Go's
  "format a Stringer's `String()` result, not its numeric value, for `%x`/`%X`" rule,
  printing the hex of the ASCII text "ALL" (`414C4C`) instead of the small numeric
  protection code. A plain Go formatting-verb mistake, not a C-fidelity question, so
  fixed directly rather than logged to `DEVIATIONS.md`.
- Tests written against the old eager-pre-map assumption updated to match real
  demand paging (a P0 page is invalid until first touched) rather than left passing
  against behavior that no longer exists — see Phase 08's follow-up entry for the
  full list. New: `internal/console/set_test.go`'s `TestShowMemory_afterVMInit`.
- `go build ./...`, `go vet ./...`, `gofmt -l .` (no output), and `go test ./...`
  all clean.

### 2026-09-16 — Sub-phases 2/3/4: most of CLEAR/SET implemented, `/entry=` wired

Picked up the "grind out the rest" ask directly (per-user direction, expanded mid-task
to include implementing genuine gaps found along the way, not just the originally
catalogued CLEAR/SET items, and fixing any clear bugs hit in scope). Landed nearly
everything in sub-phases 2-4 that this document's own inventory called "ready" or
"a smaller, standalone gap" — the items it flagged as needing a design decision or new
core-package instrumentation are still open; see "Still not done" below and the
session's own final report to the user for the discussion points on each.

**Sub-phase 2 (CLEAR), shipped:**

- **`CLEAR STRINGS`** (`ClearString`, misc.go) — resets `CONSOLE$STRINGPOOL` to
  `_BASE` and zeroes the pool's backing bytes, the write side of the already-shipped
  `SHOW STRING`.
- **`CLEAR TB`** (`ClearTB`) — reports "not modeled by this port," matching `SHOW
  TB`'s own established stub pattern (no translation cache exists to reset).
- **`CLEAR MEMORY`** (`ClearMemory`) — found while implementing: the C source's own
  case 103 body is literally a call to `console_zero()`, not a memory-specific
  routine (its own module comment says as much: "CLEAR MEMORY is mapped to the ZERO
  command") — so this is just `Console.Zero`, not new logic.
- **`CLEAR MEMORY/STATISTICS`** (`ClearMemoryStatistics`) — stub, matching `SHOW
  TB`/`SHOW MAP`'s "not modeled" precedent (this port's `internal/vm.Memory` is a
  fixed byte slice, not a tracked allocator).
- **`CLEAR INTERRUPT <id>` / `CLEAR INTERRUPT/ALL`** (`ClearInterrupt`/
  `ClearAllInterrupts`) — now unblocked by Phase 14's interrupt-queue state (this
  document's own inventory hadn't caught up to that): new `cpu.Engine.ClearInterrupt`/
  `ClearAllInterrupts` methods remove matching (or all) `iqueue` entries, the latter
  also cancelling any immediately-pending interrupt, matching `console_clear.c`'s own
  two cases.
- **`CLEAR SYMBOL/TEMPORARY`** (`ClearSymbolTemporary`) — this needed real new state,
  not just wiring: `Symbol` gained a `Permanent` bool (set via `SET`'s own
  `/PERMANENT` qualifier — see below) distinguishing it from the existing
  `SymbolUser`/`SymbolSystem` `Kind`, and `SymbolTable.ClearTemporary` removes every
  non-permanent user symbol, leaving permanent and system symbols alone — matching
  `clear_temp_symbols`'s own "non-permanent user symbols" semantics exactly (this is
  a different axis than the `SymbolKind` question sub-phase 1c's inventory entry
  speculated about; no "temporary symbol *kind*" was needed, just a flag).
- **Still not done, as flagged**: `CLEAR ERROR` (needs a "last `$STATUS`" slot and a
  decision on how broadly `Dispatcher.Dispatch` should set it — see the session's own
  report), `CLEAR PROFILES` (needs the same per-opcode execution-counter
  instrumentation `SHOW INSTRUCTIONS/PROFILE` does, itself an open design question per
  this document's own sub-phase 1e), `CLEAR BREAKPOINT/FAULT[/ALL]` (needs the
  `BreakFault` `BreakKind` execute.go's own doc comment defers).

**Sub-phase 3 (SET), shipped:**

- **`SET PSL <field>=<value>[,...]`** (`SetPSLField`, dispatch.go's `cmdSetPSL`) —
  every field `vax.PSL` already had a setter for (`CM`/`TP`/`FPD`/`IS`/`DV`/`FU`/`IV`/
  `T`/`N`/`Z`/`V`/`C`/`IPL`/`PRV_MOD`), plus `CUR_MOD` (and a bare `MODE` alias)
  routed through the same `SetMode` primitive `SET MODE` itself uses. Deliberately
  does **not** replicate `console_set.c`'s own pending-AST check on a `CUR_MOD`
  change (`if (cur_mod >= ASTLVL) interrupt(0x88, 2, 0)`) — this port has no
  AST-delivery mechanism anywhere yet (confirmed: no `ASTLVL`/interrupt-code-0x88
  consumer exists in `internal/cpu`), so there is nothing for that check to feed into;
  noted in `SetPSLField`'s own doc comment rather than logged to `DEVIATIONS.md`,
  since it's an absent *feature*, not an ISA-fidelity mismatch in a feature that
  exists.
- **`SET MODE <KERNEL|EXEC|SUPER|USER|INTERRUPT>`** (`SetMode`) — reuses
  `cpu.Engine.SetModeStack` (already exported for Phase 13's `RUN`), the same
  primitive `set_mode_stack` is in the C source; same AST-check omission as above.
- **`SET PTE <addr>[ TO <addr2>] <field>=<value>[,...]`** / **`SET PAGE`**
  (`SetPTE`, `cmdSetPTE`) — the write-side counterpart to `SHOW PAGE`'s already-shipped
  `vm.Memory.LookupPTE`; needed one new `internal/vm` primitive, `Memory.StorePTE`
  (`translate.go`), since `LookupPTE`'s own returned `pteAddr` is deliberately a
  pre-recursion *virtual* address for P0/P1 (matching `tracevm`'s own display
  convention) and writing the entry back needs the real *physical* one — `StorePTE`
  repeats `LookupPTE`'s region/length-register walk but resolves all the way through
  `Translate` first. All six fields (`VALID`/`PROT`/`MODIFY`/`OWNER`/`SOFTWARE`/`PFN`,
  plus the C source's short spellings `V`/`M`/`OWN`/`S`) are supported, along with the
  `TO`-range multi-page form (`setpte_multiple`'s own 512-byte-aligned loop).
- **`SET VM` / `SET MAPEN` / `SET NOVM` / `SET NOMAPEN`** (`SetVM`) — confirmed
  during implementation that the generic `SET MAPEN=<value>` form already worked via
  `SetSymbol`/`privRegNames` (this document's own inventory note was right to flag
  that as the only real question); this adds the bare on/off keyword spellings,
  requiring kernel mode (`requireKernelMode`, matching the C source's own `EXC_PRIV`
  check — reported as this port's other kernel-mode-gated commands are, not as a
  synthesized real fault delivery, since no console command does that).
- **`SET BASE <addr>`** (`SetBase`) — sets `Console.DepositAddr` directly, confirming
  this document's own suspicion that it's not a real gap beyond a missing binding.
- **`SET VERBOSE` / `SET VERIFY` / `SET NOVERBOSE`** (`SetVerbose`/`SetVerify`/
  `SetNoVerbose`) — **found a real bug while wiring these**: `Console.Verbose`
  defaulted to Go's zero value (`false`), but `initialization.c` defaults
  `vax.console.flags` to `CONSOLE_EXPAND | CONSOLE_VERBOSE` (verbose **on**) at
  startup — fixed (`New` now sets `Verbose: true`). Also found `Print` (`PRINT`/
  `ECHO`) had never actually gated on `CONSOLE_VERBOSE` at all, despite its own doc
  comment claiming the port "always treats [verbose] as on" as a deliberate
  simplification — the real C `console_print` is a hard no-op whenever
  `CONSOLE_VERBOSE` is clear (it doesn't even print blank output, just silently
  consumes the line), so `Print` now checks `c.Verbose` for real. Both are "clear,
  obvious logic errors" under `CLAUDE.md`'s bug-fixing policy (a stale doc comment
  describing behavior that was never implemented, and a default that didn't match
  the C source with no ISA-fidelity question attached), fixed directly rather than
  logged to `DEVIATIONS.md`.
- **`SET QUANTUM <n>`** (`SetQuantum`) — now real, backed by Phase 14's own
  `Engine.SetQuantum`/`Engine.Quantum` (this document's sub-phase 1b already noted
  `SHOW QUANTUM` had become portable for the same reason; `SET` was the missing
  write side), including the C source's own resumed/suspended informational message
  when `n` crosses the zero boundary.
- **`SET UIQUANTUM <n>`** (`SetUIQuantum`) — stub matching `ShowQuantum`'s own
  "not modeled" report for the same reason (no cooperative host-UI-polling loop in
  this port); parses and accepts the value rather than erroring, since the real
  command's own syntax is otherwise legal.
- **`SET RADIX HEX|HEXA|DEC|DECI`** keyword forms (`parseRadixArg`, dispatch.go) —
  added alongside (not replacing) the pre-existing bare-numeric form, which is a more
  lenient superset the C source doesn't actually offer (no octal keyword exists in
  `console_set.c`'s own `SET RADIX` switch) but which this port had already shipped
  and there's no reason to regress.
- **`SET BREAKPOINT/TEMPORARY`** (`AddTemporaryBreakpoint`, execute.go) — turned out
  to be genuinely small, as this document's own inventory guessed:
  `Breakpoint.Temporary` already existed (STEP/OVER's/STEP/RETURN's own internal
  one-shot breakpoints already used it) — this just exposes a user-facing way to set
  one.
- **`SET`'s own `/PERMANENT`, `/ENTRY`, `/LABEL` qualifiers** on the general
  `NAME=value` symbol form (`cmdSet`'s new leading-qualifier scan,
  `Console.SetSymbolQualified`) — a genuine gap found while scoping `CLEAR
  SYMBOL/TEMPORARY` above (that command's only meaning depends on some symbols being
  marked permanent): `Symbol` gained `Permanent`/`IsLabel` fields (`IsEntry` already
  existed, from `.ENTRY`/`.SHIM` symbols); `SHOW SYMBOL`'s attribute string
  (`ShowSymbol`, show.go) now reports `permanent`/`label` too, closing out two of the
  four attributes sub-phase 1a's own `SHOW SYMBOL <name>` entry had flagged as
  remaining (`local`/`string` are a different axis — assembler-local temporary
  symbols and string-descriptor values — and stay out of scope).
- **Still not done, as flagged**: `SET MKVALID`/`SET NOMK` (the document's own
  "is there already an implicit signal?" question is still open — `VMInitValid`
  exists but isn't the same semantic, and none of the already-shipped `SHOW`
  commands this would gate actually check it), `SET ASSEMBLER`/`SET NOASSEMBLER`
  (confirmed by grep: `internal/asm` has no `ADDRESS_PROMPT`/`FORWARD_WARNINGS`/etc.
  state at all to set), `SET WATCH` (needs the watchpoint subsystem sub-phase 4
  already flagged as its own future phase), `SET FAULT/HISTORY` (needs a fault-event
  recorder hooked into `internal/cpu/handlefault.go` — a core-package instrumentation
  decision, not a console-layer wiring task), `SET BREAK/FAULT` (needs `BreakFault`,
  same as `CLEAR BREAKPOINT/FAULT` above).

**Sub-phase 4, shipped:**

- **The DCL `/entry=` redirect now actually works** (`Dispatcher.Dispatch`,
  dispatch.go): instead of unconditionally returning "requires the RTL microkernel,"
  a `/entry=` result now resolves the entry name (already uppercased by the grammar,
  e.g. `EXE$ABOUT`) as a VAX symbol via the same `Evaluator` every other address
  expression uses, and `CALL`s it with no arguments — exactly the primitive this
  document's own audit recommended. Unblocks **`ABOUT`**, **`FORTH`**, and
  **`XTEST`** (all three now reach real `kernel.asm`-defined `.ENTRY` routines once a
  microkernel is booted; without one, they now fail with the accurate "Undefined
  symbol `EXE$ABOUT`" rather than a misleading "not yet implemented").
- **Found and fixed in passing**: `SHOW VERSION`'s own grammar syntax
  (`evax.dcl`'s `syntax show_version/entry=exe$about`) *also* carries `/entry=` — it
  shares `ABOUT`'s real routine in the C source. Because `Dispatch`'s `EntryPoint`
  check ran unconditionally before ever consulting the bound-handler table, the
  `SHOW_VERSION` grammar bind (→ `Console.ShowVersion`, a Go-native placeholder
  banner explicitly documented in Phase 08's own progress log as "rather than
  leaving `ABOUT`/`SHOW VERSION` with no output at all") was **already unreachable
  dead code** before this change — `SHOW VERSION` always hit the same
  "not-yet-implemented" error `ABOUT` did, silently. Removed the placeholder
  (`ShowVersion` method, its dead grammar bind, and `doc.go`'s stale "stubbed
  pending Phase 10" comment) now that the real redirect makes it behave correctly.
  A latent bug found while implementing an unrelated item, not an ISA-fidelity
  question — fixed directly per `CLAUDE.md`'s bug-fixing policy.
- **Still not done, as flagged**: the watchpoint subsystem and fault-kind
  breakpoints — both already tracked by this document as likely future sub-phases
  of `docs/PHASE-18.md`, unchanged by this pass.

**Testing**: every new/changed method has direct unit coverage (`internal/vm/
translate_test.go`'s `TestStorePTE*`, `internal/cpu/interrupt_test.go`'s
`TestClearInterrupt`/`TestClearAllInterrupts`, and new cases in
`internal/console/set_test.go`, `misc_test.go`, `dispatch_test.go`, `show_test.go`),
including two end-to-end `Dispatch("ABOUT"/"SHOW VERSION")` tests against a really
-booted `kernel.asm` (bounded by `Engine.SetLimits`, since `LIB$PUT_OUTPUT`'s
character-at-a-time `TXCS` busy-wait never completes in this port — the same
documented limitation `regression_test.go`'s `TestRegression_rtlDependentAsmFixtures`
already tracks for other `TXCS`/`RXCS`-polling fixtures — so the test only checks
that the banner's first character reaches `Console.Out`, not the full string).
`go build ./...`, `go vet ./...`, `gofmt -l .` (clean on every file this pass
touched — a handful of pre-existing, unrelated files were already gofmt-dirty before
this pass and are left as found), and `go test ./...` all clean.
