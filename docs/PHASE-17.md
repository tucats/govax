# Phase 17: `SET DEBUG` / `SHOW DEBUG`, `SET TRACE` / `SHOW TRACE`, and the `DBG_*`/instruction-trace tracing flags

## Goal

`docs/PHASE-16.md` catalogued `SHOW DEBUG` and `SET DEBUG`/`SET NODEBUG` as
blocked on cross-cutting state this port didn't have: `vax.h`'s `DBG_*` bitmask
(`vax.debug` in the C source), which gates roughly two dozen developer/tracing
toggles scattered across the C reference's CPU, VM, RTL, and console layers.
This phase adds that bitmask, the two console commands that read/write it, and
wires real behavior for the flags whose C-side consumer has a clean Go
analogue. Sub-phases 6-8 (added after the first five sub-phases landed, per a
follow-up user request) extend this to `vax.console.disasm`'s own `SET TRACE`/
`SHOW TRACE` pair and the instruction-tracing mechanism it and two `DBG_*`
flags (`REGISTERS`, `FULLDISASM`) are built on — closing the one gap the
original five sub-phases had deliberately deferred.

Unlike Phases 00-14 (one C source file family each) or Phase 16 (an audit of
one console command surface), this phase's C-source mapping is deliberately
scattered — `vax.h`'s `DBG_*` `#define`s are consumed by a couple dozen
scattered call sites in `console_set.c`/`console_show.c` (the command
surface), `interrupt.c`/`vax.c`/`emul_call.c`/`emul_misc.c`/`vm.c` (CPU/VM
tracing), `logical_names.c`/`devices.c`/`rms.c`/`service.c`/`p1_vector.c` (RTL
tracing), and `console_run.c`/`console_dispatch.c`/`console_step.c`/
`asm_symbols.c` (console-level tracing).

**Status: complete (all 8 sub-phases).**

## Design decisions

### Where the bitmask lives

The C source keeps `vax.debug` on the single global `struct VAX vax`, visible
to every subsystem. This port has no such global (`CLAUDE.md`: "State lives in
an instantiated struct passed explicitly"), but `internal/vax.CPU` is already
threaded as a parameter (or held as a field) into every layer that needs a
`DBG_*` check: `internal/cpu.Engine` holds one, `internal/vm.Memory`'s
translation methods take one as a parameter, and `internal/rtl.Environment`
holds one. So the bitmask (and an accompanying trace-output `io.Writer`) live
on `*vax.CPU` (`internal/vax/debug.go`) rather than introducing a new
cross-cutting type or threading a new parameter through every constructor —
this reaches every consumer with zero new plumbing.

`vax.New()` seeds the same default the C source's `alloc_vax` does
(`DBG_REGISTERS|DBG_USERHALT|DBG_LIBINIT`), and `internal/console.Console.Init`
(which always constructs a fresh `*vax.CPU`) points the new CPU's debug writer
at `Console.Out`. `Console.Zero` does *not* recreate the CPU (matching
`console_zero.c`, which never touches `vax.debug`), so debug flags and the
writer both survive a `ZERO` — only `INIT` resets them, matching the C
source's own asymmetry (`console_init.c`'s save/restore list is
`radix`/`disasm`/`verify`; `vax.debug` is conspicuously not on it).

### Command surface matches the C source's actual asymmetry, not `PHASE-16.md`'s guess

`PHASE-16.md`'s inventory (written before any of this was implemented) guessed
at a `SET DEBUG <flag>` / `SET NODEBUG <flag>` pair of verbs "mirroring" other
`SET`/`SET NO*` pairs elsewhere in the console. The actual C source
(`console_set.c:581-639`) is a single `SET DEBUG <name>[,<name>...]` verb
(alias `SET DBG`) where each comma-separated item is independently prefixed
`NO` to clear that one bit (`SETDBG`'s macro expansion) — there is no separate
top-level `NODEBUG` verb. This phase matches the real source, not the
inventory's guess.

`SHOW DEBUG` (`console_show.c:439-564`) also does not display every settable
name: `console_set.c`'s `SETDBG` table accepts 25 names, but `show_debug`
only `printbit`s 20 of them. `MEMORY`/`P1`/`P2`/`P3`/`P4` are settable and
gettable via `Console.DebugEnabled` but never appear in `SHOW DEBUG`'s output,
and `FUNCTIONS`/`UNIMP` have no command-surface presence at all in the C
source (defined bits, no `SETDBG` entry, no `printbit` call, and no consumer
anywhere else either) — this phase reproduces both asymmetries exactly rather
than "fixing" them into a uniform list.

### Tracing granularity: coarser than the C source's, on purpose

These flags are a developer/tracing aid for *this emulator's own
development*, not emulated VAX-architectural behavior — `vax.h`'s own comments
call them out that way ("Debug memory", "Display image info on RUN command?").
`CLAUDE.md`'s bug-fixing policy (ISA/hardware-fidelity questions get
documented and deferred; obvious logic bugs get fixed) is about the *emulated
machine's* behavior; it doesn't extend to requiring printf-for-printf fidelity
in a debug-trace aid that itself has no architectural meaning. Several C call
sites print one line per item-code or sub-case inside a single service call
(e.g. `$TRNLNM`'s item-list loop in `logical_names.c` prints a separate line
per item code requested). This port traces at the operation level (one line
per service/operation call, naming the operation and its key arguments)
rather than per internal sub-case — enough to make each flag genuinely useful
for spotting what a subsystem is doing, without a disproportionate amount of
call-site-by-call-site porting for marginal debugging value. Each sub-phase
below says exactly what it traces.

### Flags with no wired behavior (matches the C reference, not a gap)

- **`DEBUG` (`DBG_DEBUG`)** — C's only consumer (`console.c:168`) invokes a
  native (platform) debugger breakpoint once, then self-clears the bit. No Go
  equivalent exists (there is no "native debugger" concept for a Go binary,
  and `runtime.Breakpoint()` would only do something useful under an attached
  debugger, unlike C's `invoke_debugger()`). The flag is fully settable/
  showable (`SET DEBUG DEBUG`, `SHOW DEBUG`) but never auto-clears and never
  triggers anything — a deliberate, documented no-op, same as it would be with
  a native debugger not attached.
- **`SYMBOLS` (`DBG_SYMBOLS`)** — C's only consumer (`asm_symbols.c`, several
  sites) is itself wrapped in `#ifdef DEBUG` — i.e. gated behind a *compile-time*
  debug build even in the C reference, not just the runtime flag. `internal/asm`
  has no `*vax.CPU`/debug-flag plumbing at all (it's a standalone
  text assembler with no notion of "the current machine"), and threading one
  through for a symbol-lookup trace print is a disproportionate amount of new
  plumbing for a compile-time-gated diagnostic. The flag is settable/showable;
  no trace is wired.
- **`MEMORY`, `FUNCTIONS`, `UNIMP`, `P1`-`P4`** — no consumer anywhere in the
  C reference either (confirmed by grep across the whole tree); these are
  reserved/vestigial bits. `MEMORY`/`P1`-`P4` are settable via `SET DEBUG`
  (matching `SETDBG`'s table) but not shown by `SHOW DEBUG` (matching
  `show_debug`'s omission); `FUNCTIONS`/`UNIMP` have no command-surface
  presence at all (matching the C source having neither a `SETDBG` entry nor a
  `printbit` call for them).
- **`KEYBOARD` (`DBG_KBD`)** — C's consumer (`vax.c`'s `poll_keyboard`) is
  inside a platform-specific (`macintosh`/`WIN`) polling loop with no Go
  equivalent (`internal/cpu.Engine.DeliverConsoleByte` is push-based, not
  polled). Wired anyway at the one structurally-closest point
  (`DeliverConsoleByte`) as a Go-appropriate analogue, not a byte-for-byte
  port — see Sub-phase 2.

## Sub-phase 1: `DebugFlags` bitmask + `SET`/`SHOW DEBUG` (`internal/vax`, `internal/console`)

- `internal/vax/debug.go`: `DebugFlags uint32` type, all 26 named constants at
  their C bit values (`vax.h:320-346`), and `DebugDefault` (the three bits
  `alloc_vax` seeds). `CPU` grows a `debug DebugFlags` field (seeded by `New`)
  and a `debugOut io.Writer` field, plus `Debug`/`SetDebug`/`DebugEnabled`/
  `SetDebugWriter`/`DebugWriter` methods (the last defaulting to `io.Discard`
  when unset, so every consumer can unconditionally `fmt.Fprintf` behind a
  `DebugEnabled` check with no nil-writer special-casing).
- `internal/console/init.go`: `Console.Init` points the freshly-constructed
  CPU's debug writer at `Console.Out`.
- `internal/console/set.go`: `Console.SetDebug(names []string) error` — parses
  each name (optionally `NO`-prefixed) against the 25-name `SETDBG` table,
  case-insensitively, matching the full keyword spelling `SHOW DEBUG` displays
  (not the C source's 4-character abbreviation matching — this port's other
  hand-parsed `SET` sub-verbs, e.g. `RADIX`/`BREAKPOINT`, already match full
  words, not abbreviations, so this follows existing convention). A bare
  `SET DEBUG` with no argument list sets the `DEBUG` (native-debugger) bit
  alone, matching `console_set.c`'s `isend(*p)` special case.
- `internal/console/dispatch.go`'s `cmdSet`: new `"DEBUG"`/`"DBG"` case
  delegating to `SetDebug`.
- `internal/console/show.go`: `Console.ShowDebug()`, a `printbit`-equivalent
  helper (`%-20s    %s`, `NO`-prefixed name when the bit is clear), printing
  the same 20 names in the same order as `console_show.c:439-564`.
- `internal/console/dispatch.go`'s `bindGrammar`: `g.Bind("SHOW_DEBUG", ...)` —
  the DCL grammar keyword/syntax (`debug/syntax=show_debug`,
  `syntax show_debug/id=129`) already existed in `evax.dcl` from Phase 16's
  pass; only the binding was missing.
- New `vmserrors` code: `CLI_BADDEBUGFLAG` (`Invalid SET DEBUG flag !Q`),
  following the existing `CLI_BADSETSYNTAX`/`CLI_BADRADIXVAL` pattern.

## Sub-phase 2: `internal/cpu` tracing (`USERHALT`, `EXCEPTIONS`, `INTERRUPTS`, `CHM`, `KEYBOARD`)

- **`USERHALT`** (`control.go`'s `emulHalt`) — this was a *named, already-
  documented* gap (`emulHalt`'s own doc comment: "a Phase 08 console concern
  with no `vax.debug` equivalent yet"). Now that the equivalent exists,
  `emulHalt` checks `e.cpu.DebugEnabled(vax.DebugUserHalt)` and allows HALT
  from any mode when set, matching `emul_misc.c:215`'s `!(vax.debug &
  DBG_USERHALT) && vax.pslw.cur_mod > 0` check exactly (this is the one flag
  in this phase with an exact, not coarsened, port — it's a single boolean
  gate, not a trace).
- **`EXCEPTIONS`** (`engine.go`'s `raise`, `handlefault.go`'s `HandleFault`) —
  `raise` (the choke point every decode-time and Handler-returned `*Fault`
  passes through before `HandleFault` runs, the Go analogue of C's
  `set_fault`+`handle_fault` sequence) prints a `SET` line; `HandleFault`
  prints a `TAKE` line, matching `interrupt.c:188`/`interrupt.c:286`'s two
  distinct `DEBUG(EXCEPTION):` messages.
- **`INTERRUPTS`** (`interrupt.go`'s `Interrupt`, `scanInterruptQueue`,
  `deliverPendingInterrupt`) — matches `interrupt.c:536`/`interrupt.c:543`'s
  queue/set-immediately messages and `vax.c:240`/`vax.c:273`'s aging-scan/
  taken messages.
- **`CHM`** (`call.go`'s `emulRei`, *not* `changemode.go`'s `emulChmx` — the
  C source's trace is on the mode-switch's reversal on return, not the
  CHMx request that starts it; see the Progress Log entry below for how this
  was found) — one line on an actual mode change, matching
  `emul_call.c:322`'s `old_mode != vax.pslw.cur_mod` guard exactly.
- **`KEYBOARD`** — see "Flags with no wired behavior" above for why this is a
  Go-appropriate analogue rather than a port: `interrupt.go`'s
  `DeliverConsoleByte` traces the byte delivered and whether it overwrote
  still-unread data (the one condition C's `poll_keyboard` itself
  distinguishes with its own `kbd_debug` print).

## Sub-phase 3: `internal/vm` tracing (`VM`, `TB`)

- `translate.go`'s `Translate`/`LookupPTE` trace address-to-PTE resolution
  (`VM`) and mirror `vm.c`'s `DBG_TB` sites' intent (this port's `Translate`
  does an uncached page-table walk with no separate TB-hit/miss state of its
  own — `docs/PHASE-16.md`'s `SHOW TB` entry already noted this — so `TB`
  traces the same translation event as `VM` rather than a distinct cache-hit/
  miss event that doesn't exist in this port; documented inline rather than
  silently aliasing the two).

## Sub-phase 4: `internal/rtl` tracing (`LOGICALS`, `DEVICES`, `RMS`, `SERVICES`, `PROCESS`)

Per the "coarser granularity" design decision above, each of these traces one
line per service/operation call (name + key arguments), not per internal
item-code sub-case:

- **`LOGICALS`** — `internal/console/device.go`'s `Console.DefineLogical`
  (the `DEFINE/LOGICAL` console command, matching `set_logical`'s trace) and
  `internal/rtl/logicals.go`'s `serviceSysTrnlnm` (matching `get_logical`'s).
- **`DEVICES`** — `internal/rtl/devices.go`'s `serviceSysGetdviw` (all 5 of
  `devices.c`'s `DBG_DEVICES` sites are there, not in `sys_assign` as
  initially assumed before checking the source).
- **`RMS`** — `internal/rtl/rms.go`'s `serviceSysCreate`/`serviceSysConnect`/
  `serviceSysPut`.
- **`SERVICES`** — `internal/rtl/environment.go`'s `SystemService`, the single
  choke point every `SYS$` call goes through (matches `p1_vector.c:433`'s
  "Debug P1 system service calls?" exactly — this port's table-driven service
  dispatch gives it a single call site the C source's `switch`-based
  `call_service` didn't have).
- **`PROCESS`** — `internal/rtl/core.go`'s `serviceSysGetjpiw`, matching
  `service.c:152`.

## Sub-phase 5: `internal/console` tracing (`IMAGES`, `LIBINIT`, `DCL`)

- **`IMAGES`** — `run.go`'s `Run` (once the main image is loaded) and
  `buildImageInitDriver` (once per dependency whose `LIB$INITIALIZE` entry
  is being wired into the driver procedure) trace, matching
  `console_run.c:67`/`console_run.c:343`.
- **`LIBINIT`** — implementing this flag's trace surfaced a real fidelity
  bug in `RunOptions.RunInits`'s default: `console_run.c:208` seeds
  `run_inits` from `vax.debug & DBG_LIBINIT` (on by default) *before* an
  explicit `/INIT`/`/NOINIT` qualifier can override it, but this port's
  `parseRunQualifier` defaulted to Go's zero value (`false`) when neither
  qualifier was given — meaning a plain `RUN foo.exe` never ran
  `LIB$INITIALIZE` by default, unlike the C reference. Fixed by threading
  `Console.DefaultRunInits()` (reading `DebugLibinit`) into
  `parseRunQualifier` as its seed.
- **`DCL`** — `dispatch.go`'s `Dispatch`, at the point it falls through to
  the DCL grammar, traces the line being parsed. `console_dispatch.c:121`'s
  own consumer (`DCLsetdebug(2, 0)`) sets a *third-party DCL parser
  library's* own verbosity knob — this port's `internal/console/dcl` is its
  own grammar interpreter, not a wrapped external library with a separate
  knob, so this is the closest equivalent visibility rather than a literal
  port of that one call.

## Explicitly out of scope

- **`COMMAND` (`DBG_EXPAND`)** — the C source's consumer
  (`console_dispatch.c`'s own `'symbol'`-substitution preprocessor,
  `did_sub`/the surrounding scan) is a full VMS-style command-line symbol-
  substitution feature this port has never implemented at all (confirmed:
  no substitution mechanism anywhere in `internal/console`) — not an
  existing-but-untraced behavior the way every other flag in this phase
  was. Building that feature is well beyond "wire a debug trace" and
  belongs in its own phase (`docs/PHASE-16.md` already named `SHOW
  COMMAND_ARGS`/`SHOW EXPAND` as blocked on the same missing data source).
  The flag itself remains settable/showable (Sub-phase 1); no trace is
  wired, and `SHOW DEBUG`'s `COMMAND` row will simply never turn on until
  that feature exists.
`REGISTERS`/`FULLDISASM` were also deferred here in the original five-sub-phase
pass (needed a snapshot/diff mechanism `Console.Step` had no equivalent of at
all) — **no longer deferred**, see Sub-phases 6-8 below, added after a
follow-up user request to also port `SET TRACE`/`SHOW TRACE` and the
instruction-tracing mechanism those two flags (and `vax.console.disasm`)
are built on.

## Sub-phase 6: `SET TRACE`/`SET NOTRACE` and `SHOW TRACE` (`internal/console`)

### Scope / C source mapping

- `console_set.c:928-939` — `SET TRACE`/`SET DISASSEMBLY` (aliases) set
  `vax.console.disasm = 1`; `SET NOTRACE`/`SET NODISASSEMBLE` clear it.
  Unlike `SET DEBUG`, this is a single on/off flag, not a bitmask — no
  per-item list syntax.
- `console_show.c:992-999` — `SHOW TRACE` (id `147`, keywords `TRACE`/
  `DISASSEMBLY`, already bound in `evax.dcl` from Phase 16's audit pass,
  same situation `SHOW DEBUG` was in before Sub-phase 1 of this phase):
  reports `vax.console.disasm` as "enabled"/"disabled", and — only when
  enabled — a second line reporting `DBG_REGISTERS`' own state (`SHOW
  TRACE` is the one place in the C source that reads a `DBG_*` flag outside
  `SHOW DEBUG` itself).

### Design

- New `Console.Trace bool` field (`internal/console/machine.go`), matching
  `vax.console.disasm`. Like `Radix`/`Verbose`/`Verify`, this is a plain
  `Console` struct field `Init`/`Zero` never touch, so it's automatically
  preserved across a re-`INIT` — matching `console_init.c`'s own
  save/restore list (`radix`/`disasm`/`verify`), which `Console.Init`'s
  existing doc comment already described in spirit even though it named
  `Verbose` rather than a `disasm`-equivalent field (there wasn't one to
  name yet).
- `Console.SetTrace(bool)`, wired into `cmdSet`'s `"TRACE"`/`"DISASSEMBLY"`
  (set true) and `"NOTRACE"`/`"NODISASSEMBLE"` (set false) cases.
- `Console.ShowTrace()`, wired into `bindGrammar`'s `SHOW_TRACE`.

## Sub-phase 7: instruction-trace infrastructure

### Scope / C source mapping

`vax.c:437-454`'s own `if (disasm) { ... }` block inside `execute_vax`'s main
loop: before dispatching each instruction's `Handler`, print
`[<stack> <SP>] <disassembled instruction>` — `<stack>` is `ISP` on the
interrupt stack or else the current mode's stack-pointer name (`KSP`/`ESP`/
`SSP`/`USP`, `vax.c:113`'s own `mode_name[]` — not to be confused with
`console_show.c`'s differently-named `mode_names[]`, KERNEL/EXEC/SUPER/USER;
two arrays with an unfortunately similar name for two different things in
the C source itself). `disasm` is `vax.console.disasm` (`Console.Trace`) for
`EXEC`/`GO`/`CALL`/`RUN` (`console_exec.c:80,389`), always `1` for plain
`STEP` (`console_step.c:117`, "Always in trace mode" per its own comment)
regardless of `Console.Trace`, and `vax.console.disasm` again for `STEP/
OVER`/`STEP/RETURN` (`console_step.c:129`).

### Design

- This port has no single `execute_vax`-equivalent loop — `Console.Execute`
  (`EXEC`/`GO`), `Console.Step` (`STEP`), and `Console.Call` (`CALL`, and
  `RUN` via its synthesized driver procedure) each run their own loop
  around `cpu.Engine.Step`, matching `docs/PHASE-03.md`'s original design
  split (CPU-loop mechanics in `internal/cpu`, STEP/BREAK/CALL semantics
  layered on top in `internal/console`). So the trace hook is added at all
  three call sites rather than in one shared place — see `trace.go` (new
  file), whose one entry point (`Console.traceStep`) each of the three
  loops calls identically.
- The disassembly text itself reuses `internal/asm.Disassemble` (via
  `disasm.go`'s existing `memByteReader`, already built for the
  `DISASSEMBLE` command) rather than adding a second decoder — this is a
  purely textual, static disassembly of the bytes at the about-to-execute
  PC, matching what `decode_instruction`'s own disassembly-buffer
  construction (`decode_opcode.c:76-206`) does before the C source's
  `disasm_operand` calls fill in operand text; a best-effort call (a
  disassembly failure prints a placeholder rather than aborting execution,
  since this is a developer trace, not part of instruction dispatch).
- `Console.Step` always traces (`force = true`, matching `execute_vax(1)`);
  `Execute`/`Call` trace only when `Console.Trace` is set — matching the
  `vax.console.disasm`-gated call sites.
- **Found, not fixed, while implementing this**: `Console.Call`'s existing
  `step bool` parameter prints `"Stepped to %08X\n"` after *every*
  instruction for the *entire* call (looping until return/halt) when true —
  but the real C source's `CALL/STEP` (`console_exec.c:377-379`) does
  exactly **one** `console_step` call and returns control to the console
  prompt; it does not run the call to completion at all. This is a
  pre-existing divergence (from Phase 13, predating this phase), not
  something introduced here, and changing `Call`'s stepping semantics is a
  bigger, separately-scoped behavior change than "add instruction tracing" —
  flagged here for the user rather than silently changed or silently left
  unmentioned.

## Sub-phase 8: wire `REGISTERS` and `FULLDISASM` into the trace point

### Scope / C source mapping

- `REGISTERS` (`registers.c:61-87`, `save_regset`/`check_regset`): a
  snapshot of `R0`-`R11`/`AP`/`FP`/`PSL` taken before the instruction runs,
  diffed after — only the registers that actually changed are printed
  (`SP`/`PC` excluded: matching `check_regset`'s own `n<14` bound, since
  both change on every instruction and would be pure noise here).
- `FULLDISASM` (`console_disasm.c:183-207`, `format_operands`): for each of
  the just-decoded instruction's operands, print its access kind (read/
  write/modify/address/bitfield/branch/immediate) and either the register
  name+value (register-direct operands) or the computed VAX address
  (memory operands) — printed *before* the instruction executes in the C
  source.

### Design

- `REGISTERS`: `Console.traceStep` snapshots `R0`-`R11`/`AP`/`FP`/`PSL`
  before calling `cpu.Engine.Step`, and diffs after, matching
  `check_regset`'s own format (`registers.c:76-85`) exactly — this needs no
  new `internal/cpu` state at all, just register reads `Console` already
  has access to.
- `FULLDISASM`: needs the just-decoded instruction's *resolved* operand
  data (which register, which computed address) — the static disassembler
  `Sub-phase 7` uses for the trace line doesn't have this (it never
  resolves addresses against live register/memory state). `cpu.Engine`
  grows one new accessor, `LastDecoded() Decoded`, returning whatever the
  most recent `Step` call decoded (the same value already cached in
  `Engine`'s own reused-per-call field, `docs/PHASE-03.md`'s "avoid a
  fresh heap allocation every instruction" design already relies on this
  being a single reused field — `LastDecoded` just exposes it read-only).
  **Documented deviation from the C source**: because `Engine.Step` decodes
  and executes in one call with no gap in between, this dump necessarily
  reads register/memory state *after* the instruction ran, not before like
  `format_operands`. For every `AccessRead` operand this is unobservable
  (nothing changed it); for `AccessWrite`/`AccessModify` operands it shows
  the operand's *new* value rather than its old one. Restructuring
  `Engine.Step` into a separate decode-then-execute pair to fix this
  precisely would be a much larger change for a low-stakes, developer-only
  trace of the emulator's own operand resolution (not emulated-VAX
  behavior) — not undertaken here; noted rather than silently accepted.

## Progress Log

### 2026-09-15 — Sub-phases 6-8 complete: `SET`/`SHOW TRACE` and instruction tracing

**Sub-phase 6**: `Console.Trace` (`machine.go`), `Console.SetTrace`
(`set.go`, wired into `cmdSet`'s `TRACE`/`DISASSEMBLY`/`NOTRACE`/
`NODISASSEMBLE` cases), `Console.ShowTrace` (`show.go`, bound to the
already-existing `SHOW_TRACE` grammar entry). Confirmed `SET [NO]VERBOSE`
(`Console.Verbose`) is a genuinely separate, still-unimplemented C feature
(`CONSOLE_VERBOSE`, "prattle on about things as we do them" console
messages) rather than a misnamed stand-in for `disasm`/`Trace` — checked
before considering reusing the field, since it was otherwise dead (zero
consumers anywhere in `internal/console`).

**Sub-phase 7**: new `internal/console/trace.go`. `Console.traceStep(pc,
force)` prints `[<stack> <SP>] <addr>: <disassembly>` before an instruction
runs (reusing `internal/asm.Disassemble` via `disasm.go`'s existing
`memByteReader` — no second decoder), called from all three loops that
drive `cpu.Engine.Step` (`Execute`, `Step`, `Call`). `Step` always traces
(`force=true`); `Execute`/`Call` trace only when `Console.Trace` is set.
Found, documented, and deliberately not fixed: `Console.Call`'s own `step`
parameter already diverges from the real C `CALL/STEP` semantics (traces
every instruction to completion instead of single-stepping once and
returning to the prompt) — a pre-existing Phase 13 behavior, not something
this sub-phase introduced or was asked to fix.

**Sub-phase 8**: `traceStep`'s returned `finish` closure prints
`DebugRegisters`' changed-register dump (`snapshotTraceRegs`/
`printRegisterChanges`, matching `check_regset` exactly: `R0`-`R11` in
hex+decimal, `AP`/`FP` in hex only, `PSL` last if changed, `SP`/`PC`
excluded as pure per-instruction noise) and `DebugFullDisasm`'s operand
dump (`printOperandDump`, matching `format_operands`'s access-kind-plus-
value format). The latter needed one new `internal/cpu` accessor,
`Engine.LastDecoded()`, exposing the `Decoded` value `Step` already caches
and reuses internally — the one deliberate, documented fidelity gap in
this sub-phase: because `Step` decodes and executes in a single call with
no gap to hook between the two, the operand dump necessarily reflects
post-execution state, so a written operand shows its new value rather than
the C source's pre-execution one.

Tests: `internal/console/execute_test.go` (trace-line presence/absence for
`Execute`/`Step`/`Call`, the `DebugRegisters` dump with and without the
flag, the `DebugFullDisasm` operand dump — using a hand-built
`MOVL #0x12345678, R0` fixture exercising both a read and a write operand),
`internal/console/set_test.go`/`show_test.go`/`dispatch_test.go`
(`SetTrace`/`ShowTrace`/`SET TRACE`+`SHOW TRACE` round-trip),
`internal/cpu/engine_test.go` (`TestEngineLastDecoded`). `go build ./...`,
`go vet ./...`, `go test ./...` all clean.

**All eight sub-phases are now complete.**

### 2026-09-15 — Sub-phase 5 complete: `internal/console` tracing; phase complete

`IMAGES`: `Run` traces the resolved main image once loaded; `buildImageInitDriver`
traces each dependency's `LIB$INITIALIZE` entry as it's wired into the driver.

`LIBINIT`: found and fixed a real fidelity bug while implementing this
flag's trace (not just wiring one): `RunOptions.RunInits` defaulted to
Go's zero value (`false`) when `RUN` was given no `/INIT`/`/NOINIT`
qualifier, so `LIB$INITIALIZE` never ran by default — but the C reference
defaults `run_inits` from `vax.debug & DBG_LIBINIT`, which is *on* by
default (`DebugDefault`). `parseRunQualifier` now takes that default as a
parameter (`Console.DefaultRunInits()`), overridden by an explicit
qualifier exactly as before.

`DCL`: `Dispatch` traces the line handed to the DCL grammar parser — the
closest equivalent to `console_dispatch.c`'s own `DCLsetdebug` call, which
sets a third-party parser library's own knob this port's independent
grammar interpreter has no equivalent of.

`COMMAND` (`DBG_EXPAND`): scoped out — see "Explicitly out of scope" above;
its C-side consumer is an entire unimplemented command-line substitution
feature, not an existing behavior needing only a trace.

Tests: `internal/console/run_test.go` (`TestDefaultRunInits`,
`TestParseRunQualifier_defaultAndOverride`, `TestRun_debugImagesTrace`),
`dispatch_test.go` (`TestDispatch_debugDCLTrace`). `go build ./...`,
`go vet ./...`, `go test ./...` all clean.

**All five sub-phases are now complete. Every `DBG_*` flag from
`vax.h:320-346` is settable via `SET DEBUG`/`SET DBG`, all 20 C-side-shown
ones are reported by `SHOW DEBUG`, and every flag with a real, portable
C-side consumer has one wired here — see "Flags with no wired behavior"
and "Explicitly out of scope" above for the handful that don't (each with
its own documented reason, not a silent gap).**

### 2026-09-15 — Sub-phase 4 complete: `internal/rtl` tracing

`LOGICALS`: `Console.DefineLogical` (`internal/console/device.go`, guarded
against a nil `c.CPU` since this command deliberately doesn't require
`INIT`) and `serviceSysTrnlnm` (`internal/rtl/logicals.go`), matching
`set_logical`/`get_logical`'s three outcomes (table not found/name not
found/value) rather than the item-list sub-case prints.

`DEVICES`: found while implementing that all 5 of `devices.c`'s
`DBG_DEVICES` call sites are actually in `sys_getdviw`, not `sys_assign` as
`PHASE-17.md`'s initial plan assumed — corrected before landing the code (in
both the plan section above and here). Traces the resolved device name once,
matching `devices.c:326`.

`RMS`: `serviceSysCreate`/`serviceSysConnect`/`serviceSysPut`
(`internal/rtl/rms.go`), one summary line each matching `rms_create`/
`rms_connect`/`rms_put`'s own entry traces (not their further per-branch
prints).

`SERVICES`: `Environment.SystemService` (`internal/rtl/environment.go`) —
the single choke point every `SYS$` call already goes through in this
port's table-driven dispatch, so one trace call covers what `p1_vector.c`
needed its own dedicated `call_service` switch case for.

`PROCESS`: `serviceSysGetjpiw` (`internal/rtl/core.go`) — this closed a
small pre-existing gap the function's own doc comment had flagged: `argv[2]`
(an optional process-name descriptor) was deliberately left unread because
"this port has no equivalent trace output to feed"; now that it does, it's
read and traced, and the comment updated to say so.

Tests: one trace-present test per wired call site, plus explicit
trace-absent tests for `serviceSysTrnlnm` and `Console.DefineLogical`
(the latter also covered for the pre-`INIT` nil-`CPU` case, since this
command doesn't require it) — `internal/rtl/logicals_test.go`,
`devices_test.go`, `rms_test.go`, `rtl_test.go`, `core_test.go`,
`internal/console/device_test.go`. `go build ./...`, `go vet ./...`,
`go test ./...` all clean.

### 2026-09-15 — Sub-phase 3 complete: `internal/vm` tracing

`Translate` (`translate.go`) traces `VM`/`TB` right after reading the PTE
(matching `vm.c`'s own placement — the C trace fires once the PTE is loaded
regardless of whether the protection/valid checks that follow it then fault,
so this port's trace point matches that, not "only on a fully successful
translation"). `TB` prints the identical VA/region/PTE-address/PTE/
protection/access/physical-address line under its own `DEBUG(TB):` label
(no separate cache state to report — see the design-decision note above).
`LookupPTE` (the `SHOW PAGE`-only diagnostic walk, a different C function —
`tracevm`, not `vm()`) is untouched, since neither of `vm.c`'s two `DBG_VM`
call sites is inside it.

Tests: `internal/vm/translate_test.go`'s `TestTranslateDebugVMAndTBTrace`/
`TestTranslateNoDebugTraceWhenFlagsClear`. `go build ./...`, `go vet ./...`,
`go test ./...` all clean.

### 2026-09-15 — Sub-phase 2 complete: `internal/cpu` tracing

`emulHalt` (`control.go`) now checks `DebugUserHalt` for real, closing the gap
its own doc comment had named since Phase 08 (and fixing a latent fidelity
mismatch this uncovered: the C default has `DBG_USERHALT` *on*, so HALT from
any mode is the C reference's actual default behavior, not the always-
kernel-only check the Go code had before this landed).

`EXCEPTIONS`: `engine.go`'s `raise` prints the `SET` line (`interrupt.c:188`'s
`%02X` code width, args list included), `handlefault.go`'s `HandleFault`
prints the `TAKE` line (`interrupt.c:286`'s `%04X` code width — genuinely a
different width from `SET`'s in the C source itself, not a transcription
slip).

`INTERRUPTS`: `interrupt.go`'s `Interrupt` (immediate-delivery and queue
cases), `scanInterruptQueue` (per-entry aging-scan and admission), matching
`interrupt.c:536/543` and `vax.c:240/273`. Note: the C source's aging-scan
trace also prints the interrupt's original `quantum` value; Go's
`queuedInterrupt` deliberately doesn't retain that field (see the type's own
doc comment), so the ported trace omits it rather than fabricating a value.

`CHM`: found during this sub-phase that `console_set.c`'s trace is actually
inside `emul_rei` (REI, the mode-switch's *reversal*), not `emul_chmx`
(CHMx only *requests* the switch via the fault mechanism) — `docs/PHASE-17.md`
sub-phase 2's plan initially assumed `changemode.go`; corrected to
`call.go`'s `emulRei` before implementing, comparing `PSL.CurMod()` before
and after the restore.

`KEYBOARD`: `interrupt.go`'s `DeliverConsoleByte` traces the "already
pending, ignoring" case. Implementing the trace surfaced a real behavior gap
in the byte-delivery primitive itself (it unconditionally overwrote RXDB
even when the previous byte hadn't been read, unlike `poll_keyboard`'s own
guard) — fixed alongside the trace, since printing "ignoring" while actually
overwriting would have been a lie about what the code does, not a pre-
existing, separately-scoped issue.

Tests: `internal/cpu/control_test.go` (`TestEmulHaltAllowedOutsideKernelModeWithUserHaltDebugFlag`,
and the existing kernel-mode-fault test updated to explicitly clear
`DebugUserHalt` first), `engine_test.go`, `handlefault_test.go`,
`interrupt_test.go`, `call_test.go` (one trace-present + one trace-absent
test per wired flag). `go build ./...`, `go vet ./...`, `go test ./...` all
clean.

### 2026-09-15 — Sub-phase 1 complete: `DebugFlags` bitmask + `SET`/`SHOW DEBUG`

Added `internal/vax/debug.go` (`DebugFlags`, all 26 named constants,
`DebugDefault`) and the `debug`/`debugOut` fields + accessors on `CPU`;
`vax.New`/`Reset` both seed `DebugDefault`. `Console.Init` points the new
CPU's debug writer at `Console.Out`. `Console.SetDebug` (`set.go`) implements
`SET DEBUG`/`SET DBG`'s real syntax (one verb, per-item `NO`-prefixing, bare
form sets the native-debugger bit), wired into `cmdSet`
(`dispatch.go`). `Console.ShowDebug` (`show.go`) reproduces `printbit`'s
format and the C source's exact 20-of-25-name display list. `SHOW_DEBUG`
bound in `bindGrammar` (the DCL grammar entry already existed from Phase 16).
New `vmserrors.CLI_BADDEBUGFLAG`. Tests:
`internal/console/set_test.go` (`TestSetDebug_*`), `show_test.go`
(`TestShowDebug`), `dispatch_test.go` (`TestDispatch_setDebugAndShowDebug`;
the old `TestDispatch_unboundShowSubformErrors` now points at `SHOW
ASSEMBLER_FLAGS`, still genuinely unbound, instead of the now-implemented
`SHOW DEBUG`). `go build ./...`, `go vet ./...`, `go test ./...` all clean.

### 2026-09-15 — Phase created, scope and design decisions recorded

Catalogued every `DBG_*` flag's C-side consumer (or lack of one) across
`console_set.c`/`console_show.c` (command surface), `interrupt.c`/`vax.c`/
`emul_call.c`/`emul_misc.c`/`vm.c` (CPU/VM), `logical_names.c`/`devices.c`/
`rms.c`/`service.c`/`p1_vector.c` (RTL), and `console_run.c`/
`console_dispatch.c`/`console_step.c`/`asm_symbols.c` (console tracing).
Corrected two assumptions `docs/PHASE-16.md`'s inventory made before any of
this was implemented: `SET DEBUG`/`SET NODEBUG` is actually one verb with
per-item `NO`-prefixing, not a verb pair; `SHOW DEBUG` displays 20 of the 25
settable names, not all of them. Decided where the bitmask lives
(`*vax.CPU`, reachable everywhere with zero new constructor plumbing) and the
tracing-granularity policy (coarser than the C source's, since these flags are
a development aid, not emulated-VAX behavior). No code changes yet.
