# Project Plan

This document describes the *high-level* plan for the project, including project phases created
through pre-planning, and progress logging.

This is a conversion (from C to Go) of the VAX emulator project at [GitHub](https://github.com/tucats/evax) and
currently hosted on the local development system at /Users/tom/Documents/Projects/eVAX. This project is
not a cross-compilation, but a careful evaluation of the C version of the emulator followed by
rewriting it from scratch as Go code, using the benefits of the Go language to implement features
constructed in the reference system using C mechanisms like macros and other C language constructs.

This project will then take advantage of Go's superior ability to have integrated testing to
create a comprehensive suite of unit tests for each of the main components of the project:

- CPU hardware definition
- Virtual memory support
- VAX instruction set emulation
- Console functionality
- Integerated I/O capabilityies
- Runtime Library (RTL) simulators

## Locked-in decisions

Established during initial project planning (2026-09-14):

- **State model**: machine state lives in an instantiated struct (constructor-created,
  passed explicitly / receiver-bound), not a package-level singleton mirroring the C
  source's global `struct VAX vax`. Enables isolated unit tests and multiple machines
  per process.
- **Source import**: the full eVAX C source tree is copied into `reference/eVAX/` as a
  read-only reference for diffing during conversion (imported via `git archive` from
  the upstream `tucats/evax` repo, so only tracked source came across). Loose
  root-level fixtures from that repo are organized under `testdata/` (`asm/`, `exe/`,
  `rom/`, `dcl/`).
- **Phase order**: bottom-up, matching the subsystem list above — CPU hardware
  definition → virtual memory → instruction set → console → I/O → RTL — with the
  assembler last and an integration pass at the end. See the phase breakdown below.
- **Correctness reference**: the current C source has already been through a closed
  portability audit (`reference/eVAX/AUDIT.md`, no open findings) that fixed a
  root-cause 32-vs-64-bit `LONGWORD` typedef bug and its downstream consequences. It is
  treated as the primary behavioral reference for the port; the VAX ISA manual
  (`~/Documents/Technical Doc/VMS/vax_instr_set.pdf`) is consulted when something looks
  suspicious or underdocumented, rather than re-deriving every instruction from the
  spec.

## Phases

Each phase has its own document in `docs/PHASE-nn.md`, with scope, deliverables, open
questions, and a progress log extended as that phase is worked.

| Phase | Doc | Summary |
| --- | --- | --- |
| 00 | [PHASE-00.md](PHASE-00.md) | Project bootstrap & source import |
| 01 | [PHASE-01.md](PHASE-01.md) | CPU hardware definition |
| 02 | [PHASE-02.md](PHASE-02.md) | Virtual memory support |
| 03 | [PHASE-03.md](PHASE-03.md) | Instruction decode engine |
| 04 | [PHASE-04.md](PHASE-04.md) | Core instruction families (move/integer/branch) |
| 05 | [PHASE-05.md](PHASE-05.md) | Floating point |
| 06 | [PHASE-06.md](PHASE-06.md) | String, bitfield & queue instructions |
| 07 | [PHASE-07.md](PHASE-07.md) | Procedure calls, privileged & misc instructions |
| 08 | [PHASE-08.md](PHASE-08.md) | Console functionality |
| 09 | [PHASE-09.md](PHASE-09.md) | I/O & device support |
| 10 | [PHASE-10.md](PHASE-10.md) | RTL simulators |
| 11 | [PHASE-11.md](PHASE-11.md) | Assembler / disassembler |
| 12 | [PHASE-12.md](PHASE-12.md) | Integration & regression |
| 13 | [PHASE-13.md](PHASE-13.md) | VMS image activation (RUN) |
| 14 | [PHASE-14.md](PHASE-14.md) | Interval timer & device-interrupt delivery |
| 15 | [PHASE-15.md](PHASE-15.md) | UX / ease-of-use support |
| 16 | [PHASE-16.md](PHASE-16.md) | Console command fit-and-finish (SHOW/CLEAR/SET gaps) |
| 17 | [PHASE-17.md](PHASE-17.md) | `SET`/`SHOW DEBUG`, `SET`/`SHOW TRACE`, instruction-trace infrastructure |
| 18 | [PHASE-18.md](PHASE-18.md) | Flow of control: `STEP`/`SET STEP`/`SHOW STEP_MODE`, future breakpoints/watchpoints |
| 19 | [PHASE-19.md](PHASE-19.md) | Interactive `ASM` REPL mode |
| 20 | [PHASE-20.md](PHASE-20.md) | RTL shim resolution fix, and console-native exception reporting (CHF) |
| 21 | [PHASE-21.md](PHASE-21.md) | Translation buffer / sequential translation cache |
| 22 | [PHASE-22.md](PHASE-22.md) | RMS system services backed by `github.com/tucats/ods2` |
| 23 | [PHASE-23.md](PHASE-23.md) | Console commands to support using Files-11 containers |
| 24 | [PHASE-24.md](PHASE-24.md) | `.RMSDEF`/`.FAB`/`.RAB` assembler pseudo-ops |

Phase 13 was split out of Phase 10 once that phase's own investigation found that
`console_run.c`'s `RUN` command (real `.exe` image activation: ICB/ISD/IHD/IHI struct
mapping, sharable-image G^/.ADDRESS fixups, `LIB$INITIALIZE` calling) is a large,
separable concern on top of the RTL calling convention rather than a small wiring step —
see `docs/PHASE-10.md`'s own notes. Phase 10's RTL layer (SYS$/LIB$ services, RMS, CLI)
is fully unit-testable without a working image loader, so the split lets Phase 10 close
out on its own merits and Phase 13 land later, after Phase 11 exists if that turns out to
help (see PHASE-13.md's own notes on whether it truly needs the assembler).

Phase 14, added during Phase 12's own integration work once running `kernel.asm` for
real found that the interval timer/console-I/O interrupt delivery
`internal/cpu/procreg.go`'s `setPrivReg` always deferred to "Phase 09" was never
actually implemented anywhere, is complete: a deterministic, instruction-count-driven
quantum/interrupt-admission core (`internal/cpu/interrupt.go`), a real ICCS interval
timer, and real TXCS/TXDB/RXCS/RXDB console-I/O interrupts, closing the exact
`LIB$PUT_OUTPUT`-hangs-after-one-character gap PHASE-12.md's sub-phase 2 progress log
recorded — see PHASE-14.md's own progress log, including two real bugs (one in the C
reference, one in this port's own CHMK handler) found and fixed along the way.

Phase 15 is an open-ended, accumulating phase (not started) for `govax`-command UX/
ease-of-use improvements not tied to any single emulated subsystem — unlike every
other phase it isn't scoped to one C source area, and is expected to grow further
sub-phases as more gaps are found. Its first sub-phase (a `-path` search path for
locating unqualified file names like `vax.init`/`vax.help`/`evax.dcl`/`kernel.asm`/
`ssdef.asm`, falling back to an embedded copy) was requested by the user on
2026-09-15 — see PHASE-15.md.

Phase 16 started as an audit-only inventory, created 2026-09-15 at the user's
request: a catalogue of `SHOW`/`CLEAR`/`SET` console commands the C reference
implements that this port doesn't yet, plus a few cross-cutting console mechanisms
(watchpoints, instruction/fault-kind breakpoints, the DCL `/entry=` redirect) found
missing along the way. Most of the catalogued `SHOW`/`CLEAR` gaps have since been
implemented (see PHASE-16.md's own progress log); the watchpoint and
instruction/fault-kind-breakpoint mechanisms it found missing are tracked as future
sub-phases of PHASE-18.md instead of landing here.

Phase 17, complete, added the `vax.debug`/`DBG_*` bitmask `docs/PHASE-16.md` found
`SHOW DEBUG`/`SET DEBUG` blocked on, the two console commands that read/write it,
and (in a follow-up sub-phase set) `SET TRACE`/`SHOW TRACE` plus the instruction-
tracing infrastructure (`Engine.LastDecoded`, the `DebugRegisters`/`DebugFullDisasm`
trace-point hooks) later reused by Phase 18's own `STEP/OVER` — see PHASE-17.md.

Phase 18, its `STEP`/`SET STEP`/`SHOW STEP_MODE` sub-phase complete, was split out
of Phase 16's own `SHOW STEP_MODE`/`SET STEP` entries at the user's request,
2026-09-15: unlike Phase 16's other sub-phases, this one touches the CPU engine
itself (a call-like-instruction classifier for `STEP/OVER`, a one-shot internal
breakpoint mechanism shared with `EXEC`/`GO`'s own breakpoint handling), not just a
console command binding. It's also the intended home for the watchpoint and
instruction/fault-kind-breakpoint mechanisms Phase 16 found missing but didn't
implement — see PHASE-18.md.

Phase 19, requested by the user 2026-09-16, ports the reference tool's bare `ASM`
(no filename) interactive assembler-mode REPL — the one piece of `PHASE-11.md`'s own
scope its closeout explicitly left for follow-up. See PHASE-19.md.

Phase 20, complete, was triggered by a user report (2026-09-16) that `RUN`ning a real
`.exe` fixture failed with what looked like "a JSB to a HALT instruction". Root cause:
Phase 13's own `SHIM$` resolution mis-read `kernel.asm`'s `.shim` table — a code-0
entry doesn't mean "dead, no numeric dispatch", it means "resolve by looking up the
routine's own already-assembled label instead of synthesizing a stub", so every real
fixture depending on `DECC$SHR` crashed calling into the C runtime library's own real
startup routine, `decc$main`, via a fake stub whose leading placeholder bytes executed
as a HALT. Fixing that also exposed (rather than introduced) a second, genuinely
missing piece: `interrupt.c`'s Condition Handling Facility (`chf()`) and its
console-native `format_exception()` fallback — ported now, since a real fault (e.g.
`cli.exe`'s own still-unimplemented `SYS$`/`SHIM$` entry point) needs it to report
cleanly and halt rather than abort `RUN` outright. See PHASE-20.md, including a
one-character C-source bug (`&&` for `&`) found and fixed in `chf()` along the way.

Phase 21, requested by the user 2026-09-17, ports `vm.c`'s translation-buffer cache
(`struct TB tb[128]`) and its one-slot "sequential translation cache" into
`internal/vm` — reversing Phase 02's own decision, and Phase 16 sub-phase 1f's `SHOW TB`
audit, not to port it (a "pure performance hack with no result-visible effect"). The
request was specifically to let `SHOW TB`/`CLEAR TB` report real cache statistics the
way the reference tool does, and to model the cache-invalidation events a real VAX (and
this port's own PTE-mutating operations — `TBIS`/`TBIA`, `SET PTE`/`SET PAGE`, demand
paging) require. See PHASE-21.md, including how this port hooks the mode-change
protection-invalidation behavior (`invalidate_tb_prot`) given Phase 01's own decision not
to carry over the C source's wide/narrow `PSL` duality.

Phase 22, requested by the user 2026-09-22, is the first phase with no direct
`reference/eVAX` counterpart to port: it introduces real, functional VMS RMS
system-service support (`SYS$CREATE`/`CONNECT`/`OPEN`/`CLOSE`/`GET`/`PUT`) backed
by genuine ODS-2 volume/file access via the sibling Go module
`github.com/tucats/ods2`, plus a new console `MOUNT` command attaching a
disk-image container to a device. Phase 10's existing `rms.c` port
(`internal/rtl/rms.go`) is a stopgap that just `fopen`s an arbitrary host path
and calls it "RMS" — incomplete, not VMS-faithful, and removed outright rather
than kept as a fallback, per the user's explicit direction: the point of this
phase is a true VMS-like file system, faithful enough (since `ods2` implements
the real on-disk ODS-2 format) that a container should be interchangeable with
`simh` and other VAX simulators. `reference/eVAX` has no `MOUNT` command anywhere
either (only a dead `mountcount` field) — so this phase's correctness reference
is the real VMS RMS manual and `fab.h`/`rab.h`'s `$FABDEF`/`$RABDEF` layouts, plus
`ods2`'s own already-tested ODS-2 implementation, rather than the C source. See
PHASE-22.md, including its own design note on why
`internal/bootdata/files/evax.dcl` — not `testdata/dcl/evax.dcl` — is the grammar
file `MOUNT`/`DISMOUNT` get added to. `INITIALIZE` and broader `ods2`-CLI command
parity are deliberately deferred to a later phase. This first slice is complete:
a real, hand-written MACRO-32 program, assembled and executed for real, drives
`CREATE`->`CONNECT`->`PUT`->`CLOSE`->`OPEN`->`CONNECT`->`GET`->`CLOSE` against a
mounted device end to end (the first RMS test in the project to go through
genuinely fetched/executed `CALLS`/`XFC` dispatch rather than calling
`internal/rms` directly from Go — which surfaced and fixed a real, previously
unexercised address-arithmetic bug in `internal/cpu`'s `XFC$P1VECTOR` handler),
and an opt-in interop test confirms real read/write fidelity against a genuine
`simh`-produced VAX/VMS system disk. See PHASE-22.md's progress log for both.

Phase 23, requested by the user 2026-09-23, rounds out the operator-facing command
set Phase 22 started so a `govax` console session can do everything the separate
`ods2` module's own `cmd/ods2` interactive session can do, without needing that
second tool at all: `INITIALIZE/CONTAINER` (formatting a new, empty container),
`DIRECTORY`, `SET`/`SHOW DEFAULT` (the operator's current default device/
directory, resolving a partial file spec the way real VMS DCL does), `DELETE`,
`PURGE`, `COPY` (host-to-container, container-to-host, and container-to-
container, disambiguated per-argument by a new `/HOST` qualifier), and `TYPE`.
Like Phase 22, it has no `reference/eVAX` counterpart at all — its behavioral
reference is `github.com/tucats/ods2`'s own `cmd/ods2/internal/session` package,
read-only (that package's own Go `internal/` visibility rules out importing it
directly, so every command is a fresh `internal/rms` implementation against
`ods2`'s public API, matched functionally rather than literally ported). Also
unifies the pre-existing `INIT` (VAX-memory-allocation) and the new
`INITIALIZE/CONTAINER` under one DCL verb, `INITIALIZE`, selected by `/VAX`/
`/CONTAINER` qualifiers rather than shipping as two separately-named commands
that happened to collide under `dispatch.go`'s fixed-command first-4-characters
lookup; and adds parameter-scoped qualifiers to `internal/console/dcl`'s own
grammar engine (`Parameter.Qualifiers`), a real, additive engine capability none
of the twelve pre-existing verbs needed, purpose-built so `COPY`'s `/HOST` can
independently modify either its `SOURCE` or `DESTINATION` parameter. See
PHASE-23.md's own "Design decisions" section for the reasoning behind both, plus
its progress log for each command's own subtask. A scripted end-to-end
acceptance pass dispatches real command-line strings through one shared
`Console`/mounted-volume session end to end (`INITIALIZE/CONTAINER` ->
`MOUNT` -> `SET DEFAULT` -> file creation -> `DIRECTORY` -> `TYPE` -> `COPY` ->
`DELETE` -> `PURGE` -> `DISMOUNT`), and an opt-in interop check runs `TYPE`
against a real VAX/VMS system disk file. Documenting this phase's own new
commands in `internal/bootdata/files/vax.help` also surfaced and fixed a
pre-existing, unrelated bug in the console's `HELP` command itself: roughly
forty existing topics whose bare abbreviated form was under four characters
(`RUN`, `GO`, `SH`, `ST`, ...) silently returned "No help available", because
`ParseHelp` was discarding the file's own documented trailing-space key padding
before matching it against `helpKey`'s always-fully-padded query — both sides
now share one `normalizeHelpKey`/`normalizeHelpToken` implementation. See
PHASE-23.md's progress log for the full command-by-command breakdown.
