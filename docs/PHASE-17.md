# Phase 17: `SET DEBUG` / `SHOW DEBUG` and the `DBG_*` tracing flags

## Goal

`docs/PHASE-16.md` catalogued `SHOW DEBUG` and `SET DEBUG`/`SET NODEBUG` as
blocked on cross-cutting state this port didn't have: `vax.h`'s `DBG_*` bitmask
(`vax.debug` in the C source), which gates roughly two dozen developer/tracing
toggles scattered across the C reference's CPU, VM, RTL, and console layers.
This phase adds that bitmask, the two console commands that read/write it, and
wires real behavior for the flags whose C-side consumer has a clean Go
analogue.

Unlike Phases 00-14 (one C source file family each) or Phase 16 (an audit of
one console command surface), this phase's C-source mapping is deliberately
scattered — `vax.h`'s `DBG_*` `#define`s are consumed by a couple dozen
scattered call sites in `console_set.c`/`console_show.c` (the command
surface), `interrupt.c`/`vax.c`/`emul_call.c`/`emul_misc.c`/`vm.c` (CPU/VM
tracing), `logical_names.c`/`devices.c`/`rms.c`/`service.c`/`p1_vector.c` (RTL
tracing), and `console_run.c`/`console_dispatch.c`/`console_step.c`/
`asm_symbols.c` (console-level tracing).

**Status: complete.**

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
- **`CHM`** (`changemode.go`'s `emulChmx`) — one line on an actual mode change,
  matching `emul_call.c:322`'s `old_mode != vax.pslw.cur_mod` guard exactly.
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
- **`DEVICES`** — `internal/rtl/devices.go`'s `serviceSysAssign`.
- **`RMS`** — `internal/rtl/rms.go`'s `serviceSysCreate`/`serviceSysConnect`/
  `serviceSysPut`.
- **`SERVICES`** — `internal/rtl/environment.go`'s `SystemService`, the single
  choke point every `SYS$` call goes through (matches `p1_vector.c:433`'s
  "Debug P1 system service calls?" exactly — this port's table-driven service
  dispatch gives it a single call site the C source's `switch`-based
  `call_service` didn't have).
- **`PROCESS`** — `internal/rtl/core.go`'s `serviceSysGetjpiw`, matching
  `service.c:152`.

## Sub-phase 5: `internal/console` tracing (`IMAGES`, `LIBINIT`, `DCL`, `COMMAND`)

- **`IMAGES`** — `run.go`'s image-load path traces the image name/transfer
  address, matching `console_run.c:67`/`console_run.c:343`.
- **`LIBINIT`** — `run.go`: whether `RUN` invokes the image's
  `LIB$INITIALIZE` default gated on `DebugLibinit` (currently always-on
  behavior in this port, matching the flag's default-on seed), matching
  `console_run.c:208`'s `run_inits = vax.debug & DBG_LIBINIT`.
- **`DCL`** — `dispatch.go`'s DCL-grammar dispatch path traces the parsed verb,
  matching `console_dispatch.c:121`.
- **`COMMAND`** — `dispatch.go`'s command-line substitution path traces the
  expanded line when a substitution actually happened, matching
  `console_dispatch.c:216`'s `(vax.debug & DBG_EXPAND) && did_sub` guard.

## Explicitly out of scope

- **`REGISTERS`/`FULLDISASM`** (register-change dump and full-operand
  disassembly at `STEP`) — the C source's version
  (`vax.c:449-506`) is built on a `save_regset`/`check_regset`
  snapshot-and-diff mechanism this port's `Console.Step`/`cmdStep` has no
  equivalent of at all (not just an unwired flag — the diffing mechanism
  itself doesn't exist). Building that mechanism is a self-contained feature
  in its own right, not a small addition to this phase's flag-wiring work;
  left for a future phase. The flags remain settable/showable now (Sub-phase
  1), so `SET DEBUG NOREGISTERS`/`SHOW DEBUG` etc. all work — only the actual
  register-dump/full-disassembly behavior is deferred.

## Progress Log

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
