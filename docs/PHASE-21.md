# Phase 21: Translation buffer / sequential translation cache

## Goal

Port `vm.c`'s translation-buffer cache (`struct TB tb[128]`) and its
"sequential translation cache" (STC, the one-slot `cached_virtual_page`/
`cached_physical_page` fast path) into `internal/vm`, which — per Phase 02's
own design notes and Phase 16 sub-phase 1f's `SHOW TB` audit — had
deliberately not ported either, on the grounds that `Translate`'s direct,
uncached page-table walk gets the same *results* without them and the cache
was "a pure 1999-era performance hack with no effect on the result."

Requested by the user (2026-09-17) specifically to (a) let `SHOW TB` report
real hit/miss/flush statistics and a real cache dump the way the reference
tool's `console_show.c` case 155 does, rather than the "not applicable to
this port" stub Phase 16 left behind, and (b) model the real cache-
invalidation events a real VAX (and this port's own PTE-mutating operations)
require — most importantly, that *any* write to a PTE (the "PFN database")
must invalidate whatever cache state depends on it, whether that write comes
from `TBIS`/`TBIA`, `SET PTE`/`SET PAGE`, or `Translate`'s own demand-paging
path. This supersedes Phase 16 sub-phase 1f's `SHOW TB`/`CLEAR TB` stubs and
Phase 17 sub-phase 3's `TB` trace point (which, absent a real cache, aliased
`VM`'s trace instead).

## Scope / C source mapping

- `reference/eVAX/eVAX/Source/CPU/vm.c`:
  - `struct TB tb[128]` / `dump_tb()` / `tb_try`/`tb_hit`/`tb_flush`/
    `tb_pflush` — the 128-entry cache and its counters.
  - `cached_virtual_page`/`cached_physical_page`/`cached_mbit`/
    `cached_page_try`/`cached_page_hit` (`#if STC`, always compiled in) —
    the one-slot sequential cache, consulted *before* the 128-entry TB.
  - `vm()`'s own STC-check / TB-check / full-walk-and-populate structure
    (lines ~291-559), including the `VM_NOSIGNAL` (0x80) flag's effect on
    which cache the STC check can hit (see "The STC/probe interaction"
    below).
  - `invalidate_tb()` (full flush, `tb_flush` counter), `invalidate_page()`
    (single-slot flush, `TBIS`), `invalidate_tb_prot()` (protection-only
    flush, mode changes) — all three also flush the STC (`STC_FLUSH`).
  - `validate_page()` — demand-paging's own PTE write; its caller in `vm()`
    invalidates the current TB slot around the call regardless of outcome.
- `reference/eVAX/eVAX/Source/CPU/emul_procreg.c` — `TBIS`/`TBIA` MTPR cases
  (`invalidate_page(value)` / `invalidate_tb()`); `TBDR`'s role as the "TB
  caching disabled" register (`if (!vax.TBDR)` gates whether the 128-entry
  TB is *consulted*, but not whether it's *populated* — see below).
- `reference/eVAX/eVAX/Source/CPU/registers.c` — `read_psl_bits`/
  `write_psl_bits`, both `if (cur_mod changed) invalidate_tb_prot()`. Go's
  `vax.PSL` has no wide/narrow duality (Phase 01's own design decision), so
  this port hooks the equivalent condition — "did `CurMod` actually change
  value" — directly at the two real mode-change choke points instead (see
  Design decisions).
- `reference/eVAX/eVAX/Source/Console/console_show.c` case 155 (`SHOW TB`)
  and `reference/eVAX/eVAX/Source/Console/console_clear.c` case 107
  (`CLEAR TB`) — exact output format and counter-reset behavior to match.
- `reference/eVAX/eVAX/Source/Console/console_vminit.c` (`invalidate_tb()`
  + counter reset at VMINIT) and `console_set.c`'s `setpte` (`invalidate_page
  (addr)` after a `SET PTE`/`SET PAGE` write).

## Design decisions

- **Where it lives**: the TB/STC arrays and counters are fields on
  `internal/vm.Memory`, alongside the existing `pageMap`/`vmValid`/
  `*Count` fields — `Memory` is already this port's per-machine owner of
  translation-adjacent state, and (unlike the C source's global `tb[]`)
  this keeps it scoped to one machine instance, matching every other
  Phase 02 design choice.
- **Entry shape**: `tbEntry{valid bool; page uint32; paddr uint32; code
  Protection; protMode AccessType}`. `valid` replaces the C source's
  `page == -1` sentinel; `protMode` replaces `prot_valid`, storing either
  the last-verified `AccessType` (0/1) or a distinct `tbProtInvalid`
  sentinel (2) — matching `TB_INVALID`. A cache hit still requires *both*
  `entry.valid && entry.page == page && entry.protMode == access`, exactly
  matching `tb.page==page && tb.prot_valid==mode`'s double duty (a slot
  that's merely protection-invalidated already fails this compare without
  needing a separate check).
- **The STC/probe interaction**: the C source's STC hit-check compares the
  *raw* `mode` parameter (which may carry the `VM_NOSIGNAL` flag bit,
  0x80) against `cached_mbit`, which is only ever *written* using the
  already-stripped 0/1 value — so a `PROBEx`-driven translation (always
  passed with `VM_NOSIGNAL` set) can never hit the STC, only the 128-entry
  TB (whose own hit-check runs after the flag is stripped). This is a real,
  observable difference in cache statistics between real accesses and
  `PROBEx` probing, not a cosmetic detail, so it's replicated: `Translate`
  (real accesses) and a new `ProbeTranslate` (used only by `emulProbe` and
  `LookupPTE`'s own recursive PTE-address step, matching `tracevm`'s own
  `VM_NOSIGNAL` call) share one internal `translate(cpu, addr, access,
  signal bool)`, and only the STC hit-check is gated on `signal` — STC/TB
  *population* on a successful walk happens unconditionally either way,
  matching the C source's own unconditional population on every success
  path.
- **TBDR gates consultation, not population**: `vm()` only skips the
  128-entry TB's *hit-check* when `vax.TBDR != 0` (`tb_try`/`tb_hit` don't
  increment, no early return) — every successful full walk still writes
  the new mapping into `tb[tb_idx]` regardless of `TBDR`. Replicated as-is
  (not treated as a bug): `TBDR` is a real, if oddly-scoped, 11/750-era
  diagnostic register, and this matches its one documented behavior
  exactly.
- **Mode-change protection flush, without the wide/narrow duality**: rather
  than trying to replicate `read_psl_bits`/`write_psl_bits`'s call-site
  parity (meaningless once there's only one `PSL` representation), this
  port calls the new `Memory.InvalidateProtection()` at the two places
  `CurMod` actually changes value for a *real* mode transition:
  `internal/cpu/handlefault.go`'s `setModeStack` (fault/interrupt delivery,
  and Phase 13's `RUN`-command kernel-mode switch) and
  `internal/cpu/call.go`'s `emulRei` (`REI`'s PSL restore) — plus
  `internal/console/set.go`'s bare `SET PSL=value` (matching
  `console_set.c`'s own `read_psl_bits()` call right after that same
  assignment). `emulProbe`'s own temporary `SetPSL` swap is deliberately
  **not** hooked: the C source's `emul_misc.c` PROBE handler mutates
  `vax.pslw.cur_mod` directly without ever calling `write_psl_bits`,
  bypassing `invalidate_tb_prot` entirely — replicating that exclusion
  keeps `PROBEx` from spuriously flushing protection state on every call,
  matching the reference tool's actual (if accidental-looking) behavior.
  `SetPSLField`'s `CUR_MOD`/`MODE` case already funnels through
  `setModeStack`, so it needs no separate hook.
- **PFN-database-write invalidation** (the user's explicit ask): every
  path that writes a PTE invalidates the affected slot:
  - `TBIS` (`internal/cpu/procreg.go`'s `setPrivReg`) →
    `Memory.InvalidatePage(value)`.
  - `TBIA` → `Memory.InvalidateTB()` (full flush).
  - `StorePTE` (`SET PTE`/`SET PAGE`, `internal/vm/translate.go`) — after
    the write succeeds, `InvalidatePage(addr)` using the *original virtual
    address* passed in, matching `setpte`'s own `invalidate_page(addr)`
    (not the PTE's own address).
  - `Translate`'s own demand-paging path (`validatePage`) — the current
    slot is invalidated inline around the call, matching `vm()`'s own
    `tbp->page = -1; STC_FLUSH;` immediately around its `validate_page()`
    call, regardless of outcome.
- **`CLEAR TB` / VMINIT counter reset asymmetry**: `console_clear.c`'s
  `CLEAR TB` and `console_vminit.c`'s VMINIT both do `invalidate_tb();
  tb_try = tb_hit = tb_pflush = 0L;` — note `tb_flush` itself (the flush
  *counter*) and the STC's own `cached_page_try`/`cached_page_hit` are
  deliberately left alone. Replicated exactly rather than "fixed" to reset
  everything, since nothing marks this as a bug and a per-project judgment
  call didn't seem warranted for a display-only counter's reset scope.
- **`tb_pflush` is permanently zero**: declared and displayed by `SHOW TB`
  in the C source, but no call site anywhere in `reference/eVAX` ever
  increments it — dead instrumentation, not a hidden mechanism this port
  is missing. `Memory`'s own `pflushes` counter is kept (for `SHOW TB`
  output parity) but likewise never incremented; noted here rather than in
  `docs/DEVIATIONS.md` since it's inert, not a fidelity question.
- **`LDPCTX`/`SVPCTX` are not yet implemented** in this port (Phase 07's
  own open question — see `internal/cpu/procreg.go`'s doc comment). The C
  source's versions call `invalidate_tb()` before remapping `P0BR`/`P1BR`
  on a context switch; whoever implements them should call
  `Memory.InvalidateTB()` there too. Left as a pointer comment rather than
  a stub, matching this project's convention of not building dead code for
  unimplemented instructions.
- **`tracevm`/`LookupPTE` don't consult or populate either cache**: matches
  the C source exactly (`tracevm` is a standalone diagnostic walk with its
  own dead, unused `tb_idx` computation) — only its recursive PTE-address
  step uses `VM_NOSIGNAL` (`ProbeTranslate`), same as `Translate`'s.

## Deliverables

- `internal/vm/tb.go`: `tbEntry`, the STC fields, `Memory`'s new fields,
  `InvalidateTB`/`InvalidatePage`/`InvalidateProtection`/`ResetTBCounters`,
  and read-only accessors for `SHOW TB` (`TBStats`, `TBSnapshot`).
- `internal/vm/translate.go`: `Translate`/`ProbeTranslate` sharing one
  internal `translate(..., signal bool)`; STC/TB consult-then-populate
  logic; `LookupPTE`'s recursive step switched to `ProbeTranslate`;
  `StorePTE` invalidates on write; `validatePage`'s caller invalidates the
  current slot.
- `internal/cpu/procreg.go`: `TBIA`/`TBIS` wired to real invalidation
  instead of the Phase 07 no-op.
- `internal/cpu/handlefault.go`, `internal/cpu/call.go`,
  `internal/console/set.go`: protection-invalidation hooked at real
  mode-change sites.
- `internal/console/show.go`: real `ShowTB`, matching `console_show.c`
  case 155 and `dump_tb()`'s exact output.
- `internal/console/misc.go`: real `ClearTB`, matching `console_clear.c`
  case 107 (silent — the C source prints nothing).
- `internal/console/vminit.go`: `VMInit` flushes the TB and resets its
  try/hit/pflush counters, matching `console_vminit.c`.
- `main.go`: `-stats` output gains a TB/STC hit-miss line (user's own
  explicit request, mid-phase).
- Tests: `internal/vm/tb_test.go` (hit/miss counting, all three
  invalidation kinds, the STC/probe exclusion, `TBDR` consult-vs-populate),
  plus `internal/console` coverage for `SHOW TB`/`CLEAR TB` output.
- `docs/PLAN.md` phase table entry; `docs/PHASE-02.md`,
  `docs/PHASE-16.md`, `docs/PHASE-17.md` cross-references updated to point
  here instead of describing the cache as unported.

## Open questions

- None currently blocking; `LDPCTX`/`SVPCTX` and their `invalidate_tb()`
  call are Phase 07's own open item, not this phase's.

## Progress Log

### 2026-09-17 — Design and initial implementation

- Investigated `vm.c`'s TB/STC mechanism and every real invalidation call
  site across `emul_procreg.c`, `registers.c`, `console_set.c`,
  `console_vminit.c`, `console_clear.c`, and `interrupt.c`'s
  `set_mode_stack` to determine which Go call sites are the faithful
  equivalent given Phase 01's single-`PSL`-representation design (no
  `read_psl_bits`/`write_psl_bits` call-site parity to replicate directly).
  See "Design decisions" above for the resulting mapping, including the
  deliberate `PROBEx`/STC exclusion and the `TBDR`-gates-consultation-not-
  population behavior.

### 2026-09-17 — Implementation complete

- All "Deliverables" items landed as designed: `internal/vm/tb.go` (the
  128-entry TB + one-slot STC, their counters, and
  `InvalidateTB`/`InvalidatePage`/`InvalidateProtection`/
  `ResetTBCounters`/`TBStats`/`STCStats`/`TBSnapshot`); `translate.go`'s
  `Translate`/`ProbeTranslate` split over a shared `translate(..., signal
  bool)`; `StorePTE`/`LookupPTE`/`validatePage` updated per the design
  notes; `TBIA`/`TBIS` wired in `procreg.go`; protection-invalidation
  hooked in `handlefault.go`'s `setModeStack`, `call.go`'s `emulRei`, and
  `set.go`'s bare `SET PSL=value`; real `ShowTB`/`ClearTB`; `VMInit`'s own
  flush-and-reset step; `-stats` gained a "Translation Buffer" section
  (user's own mid-phase request, 2026-09-17).
- Found and fixed one real, pre-existing bug while adding the `-stats`
  extension: `main.go`'s package-level `stats *bool` was left `nil` until
  `main()`'s own `flag.Bool` call ran, so any test calling `run()` directly
  (bypassing `main()`) paniced on `*stats` at the `printStats` call site --
  unrelated to this phase's own scope, but surfaced by exercising the same
  code path. Fixed by defaulting `stats` to a real `*bool` (`new(bool)`)
  instead of `nil`, per this project's "obvious bug, just fix it" policy.
  `TestRun_startupBootsFromEmbeddedFilesAlone` (pre-existing, previously
  never actually run to completion with `-stats`'s call site in place) now
  passes.
- The `internal/vm/translate_test.go` fixture (`newTranslateFixture`, a
  two-level P0-page-table-found-via-S0 setup) turned out to double as a
  good demonstration of a real, non-obvious TB interaction: translating
  two different P0 pages whose PTEs live in the *same* underlying S0 page
  produces a genuine `DEBUG(TB)` cache hit on the second page's own
  recursive PTE-address lookup, before that page's own outer walk ever
  completes -- adjusted `TestTranslateDebugVMAndTBTrace` and a few new
  `internal/vm/tb_test.go` cases to assert on hit-count *deltas* around
  this recursive reuse rather than brittle absolute counts.
- `go build ./...`, `go vet ./...`, and `go test ./...` all clean, plus a
  manual `-stats`/`SHOW TB` smoke test against a real boot showing
  sensible, non-trivial hit ratios (~89% STC, ~61% TB on one boot-plus-one-
  command run).
- Also updated `CLAUDE.md` for an unrelated, user-directed repo change
  made mid-phase: `cmd/govax` was removed and the `govax` CLI promoted to
  `main.go` at the repo root; updated every stale `cmd/govax` code-comment
  reference found along the way (`engine.go`, `codes_vax.go`, `misc.go`,
  `asm.go`, `machine.go`, `execute_test.go`) — historical `docs/PHASE-*.md`
  progress-log prose mentioning the old path was left alone, matching this
  project's convention of not rewriting dated history.
