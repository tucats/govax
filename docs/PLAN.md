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
| 00 | [PHASE-00 - bootstrap](PHASE-00%20-%20bootstrap.md) | Project bootstrap & source import |
| 01 | [PHASE-01 - vrtual hardware](PHASE-01%20-%20vrtual%20hardware.md) | CPU hardware definition |
| 02 | [PHASE-02 - virtual memory](PHASE-02%20-%20virtual%20memory.md) | Virtual memory support |
| 03 | [PHASE-03 - instructoin decode](PHASE-03%20-%20instructoin%20decode.md) | Instruction decode engine |
| 04 | [PHASE-04 - core instructions](PHASE-04%20-%20core%20instructions.md) | Core instruction families (move/integer/branch) |
| 05 | [PHASE-05 - floating point](PHASE-05%20-%20floating%20point.md) | Floating point |
| 06 | [PHASE-06 - strings and bitfields](PHASE-06%20-%20strings%20and%20bitfields.md) | String, bitfield & queue instructions |
| 07 | [PHASE-07 - calls and prv instructions](PHASE-07%20-%20calls%20and%20prv%20instructions.md) | Procedure calls, privileged & misc instructions |
| 08 | [PHASE-08 - console](PHASE-08%20-%20console.md) | Console functionality |
| 09 | [PHASE-09 - devices](PHASE-09%20-%20devices.md) | I/O & device support |
| 10 | [PHASE-10 - RTL simulators](PHASE-10%20-%20RTL%20simulators.md) | RTL simulators |
| 11 | [PHASE-11 - ASM and DISASM](PHASE-11%20-%20ASM%20and%20DISASM.md) | Assembler / disassembler |
| 12 | [PHASE-12 - regression testing](PHASE-12%20-%20regression%20testing.md) | Integration & regression |
| 13 | [PHASE-13 - image activation](PHASE-13%20-%20image%20activation.md) | VMS image activation (RUN) |
| 14 | [PHASE-14 - timers and interrupts](PHASE-14%20-%20timers%20and%20interrupts.md) | Interval timer & device-interrupt delivery |
| 15 | [PHASE-15 - host system ux](PHASE-15%20-%20host%20system%20ux.md) | UX / ease-of-use support |
| 16 | [PHASE-16 - console commands](PHASE-16%20-%20console%20commands.md) | Console command fit-and-finish (SHOW/CLEAR/SET gaps) |
| 17 | [PHASE-17 - SET and SHOW](PHASE-17%20-%20SET%20and%20SHOW.md) | `SET`/`SHOW DEBUG`, `SET`/`SHOW TRACE`, instruction-trace infrastructure |
| 18 | [PHASE-18 - STEP](PHASE-18%20-%20STEP.md) | Flow of control: `STEP`/`SET STEP`/`SHOW STEP_MODE`, future breakpoints/watchpoints |
| 19 | [PHASE-19 - ASM REPL](PHASE-19%20-%20ASM%20REPL.md) | Interactive `ASM` REPL mode |
| 20 | [PHASE-20 - shims and exceptions](PHASE-20%20-%20shims%20and%20exceptions.md) | RTL shim resolution fix, and console-native exception reporting (CHF) |
| 21 | [PHASE-21 - TB and STC](PHASE-21%20-%20TB%20and%20STC.md) | Translation buffer / sequential translation cache |
| 22 | [PHASE-22 - ODS2](PHASE-22%20-%20ODS2.md) | RMS system services backed by `github.com/tucats/ods2` |
| 23 | [PHASE-23 - Filles-11 console cmds](PHASE-23%20-%20Filles-11%20console%20cmds.md) | Console commands to support using Files-11 containers |
| 24 | [PHASE-24 - RAB and FAB](PHASE-24%20-%20RAB%20and%20FAB.md) | `.RMSDEF`/`.FAB`/`.RAB` assembler pseudo-ops |
| 25 | [PHASE-25 - logical names](PHASE-25%20-%20logical%20names.md) | VMS-faithful logical names (tables, iterative translation, DEFINE/ASSIGN/SHOW LOGICAL, `$CRELNM`/`$TRNLNM`) |
| 26 | [PHASE-26 - more system services](PHASE-26%20-%20more%20system%20services.md) | Expanding system services (emulated process record; `$ADJSTK`, `$ADJWSL`, `$ALLOC`, `$ASCEFC`, ...) |
| 27 | [PHASE-27 - MACRO32 objects](PHASE-27%20-%20MACRO32%20objects.md) | MACRO-32 object modules: `MACRO` command producing VAX `.OBJ` files from `.MAR` source |
| 28 | [PHASE-28 - MACRO32 macros](PHASE-28%20-%20MACRO32%20macros.md) | The MACRO-32 macro facility (`.MACRO`, `.MCALL`, `STARLET.MLB`) and a librarian (`LIBRARY`) |
| 29 | [PHASE-29 - MACRO 32 listings](PHASE-29%20-%20MACRO%2032%20listings.md) | MACRO listings (`/LIST`, cross reference), traceback and debugger records, LINK's debug symbol table, and `LINK/DEBUG` (DMT and GST) — done. Objects, listings, and images match real MACRO's and LINK's, and VMS prints the same tracebacks |
| 30 | [PHASE-30 - LINK](PHASE-30%20-%20LINK.md) | A govax `LINK`: `.OBJ` modules to a runnable `.EXE` — done; its images match real LINK's and run on VMS |
| 31 | [PHASE-31 - clean room](PHASE-31%20-%20clean%20room.md) | Build without licensed VMS material: one symbol table, an augmenting gen, captured GSTs — done |
| 32 | [PHASE-32 - STARLET.MLB](PHASE-32%20-%20STARLET.MLB.md) | govax's own RMS macros ($FAB, $RAB, $NAM, XABs, services, $xxxDEF), written clean-room — done |
| 33 | [PHASE-33 - NAM and XAB](PHASE-33%20-%20NAM%20and%20XAB.md) | RMS name blocks and XABs at run time: $PARSE, $SEARCH, $DISPLAY, NAM/XABs on $OPEN/$CREATE/$CLOSE — done; matches VMS 7.3 on a runtime oracle |
| 34 | [PHASE-34 - CREATE DIR](PHASE-34%20-%20CREATE%20DIR.md) | `CREATE/DIRECTORY` (owner UIC, version limit, protection), with ods2 support; LIB$CREATE_DIR and `internal/librtl` — done; both match VMS 7.3 on oracle runs |
| 35 | [PHASE-35 - octaword instructions](PHASE-35%20-%20octaword%20instructions.md) | The rest of the instruction set: G/H floating, octaword moves, EMOD/POLY, packed decimal and EDITPC — done; matches VMS on a 567-case oracle |
| 36 | [PHASE-36 - FORTH](PHASE-36%20-%20FORTH.md) | The FORTH fixture (`testdata/mar/forth.mar`): RMS terminal I/O, LIB$PUT_OUTPUT/LIB$GET_FOREIGN, DCL symbols and foreign commands, MACRO listing/object fixes — done; object, listing, image, and map match VMS 7.3 |
| 37 | [PHASE-37 - DCL grammar](PHASE-37%20-%20DCL%20grammar.md) | The console's fixed commands (EXAMINE, DEPOSIT, SET, STEP, RUN, ...) move onto the DCL grammar, which gains `$expression`, separators, assignments, and nonegatable keywords — done |
| 38 | [PHASE-38 - ANALYZE OBJ](PHASE-38%20-%20ANALYZE%20OBJ.md) | `ANALYZE/OBJECT`, matching VMS 7.3's output line for line (records, GSD, TIR commands, dumps, errors, pages); the verb laid out for a later `ANALYZE/IMAGE` — done; matches VMS 7.3 byte for byte on 54 fixtures |
| 40 | [PHASE-40 - ANALYZE IMAGE](PHASE-40%20-%20ANALYZE%20IMAGE.md) | `ANALYZE/IMAGE`, matching VMS 7.3's output line for line (image header, ISDs, fixup section, errors, pages) — done; matches VMS 7.3 byte for byte on 29 fixtures |
| 41 | [PHASE-41 - symbolic disassembly](PHASE-41%20-%20symbolic%20disassembly.md) | Symbolic disassembly from a loaded image's DST, DMT, and GST, as the VMS debugger's `EXAMINE/INSTRUCTION` shows it; the disassembler moves to `internal/disasm`, symbols to `internal/symtab`, debug tables to `internal/dbgsym` (groundwork for a debugger mode) — done; `DISASSEMBLE` matches the VMS 7.3 debugger's 514 lines in 7 sessions, on VMS's images and govax's relinks |
| 42 | [PHASE-42 - debugger package](PHASE-42%20-%20debugger%20package.md) | The debugger as its own package (`internal/debugger`), grammar (`debug.dcl`), and `DBG>` prompt, entered by GO, CALL, and RUN/DEBUG; SET/SHOW split between console and debugger; VMS-compatible breakpoints, STEP, and EXAMINE; fixes to the existing run-control bugs — done; every probe session of `testdata/dbg` and `testdata/dbgcmd` (18) is replayed and compared with VMS 7.3's debugger log, with the remaining differences listed |
| 43 | [PHASE-43 - processes](PHASE-43%20-%20processes.md) | Multiprocessing, part 1 (and the plan for 43–48): system and process state split, a process table, hardware PCBs with LDPCTX/SVPCTX, per-process address spaces and stacks — done; a Go test builds a second process and switches the CPU into it and back |
| 44 | [PHASE-44 - scheduler](PHASE-44%20-%20scheduler.md) | Multiprocessing, part 2: the scheduler (`internal/sched`), quantum and priority preemption, wait states, idle, process-aware run loops, SHOW SYSTEM — done; processes built by hand share the CPU by VMS's rules, deterministically |
| 45 | [PHASE-45 - create and delete process](PHASE-45%20-%20create%20and%20delete%20process.md) | Multiprocessing, part 3: `$CREPRC`, process startup, rundown and deletion, termination mailboxes, jobs and job logical names, process-control services across processes, STOP, NL:, and the process and system service macros checked against real MACRO — done; a MACRO program creates a child, reads its termination message, and `$GETJPI`s it |
| 46 | [PHASE-46 - interprocess comm](PHASE-46%20-%20interprocess%20comm.md) | Multiprocessing, part 4: mailboxes, common event flags, and global sections between processes; RMS on mailboxes and NL:; a terminal that doesn't block other processes — done; a MACRO parent and child exchange messages through two mailboxes, with a common event flag handshake |
| 47 | [PHASE-47 - RMS and processes](PHASE-47%20-%20RMS%20and%20processes.md) | Multiprocessing, part 5: a lock manager and `$ENQ`/`$DEQ`, RMS file sharing (RMS$_FLK), a shared file control block in ods2, shared sequential files, record locks — done; three processes on a short quantum share a volume (appending, creating and erasing, extending) and it checks clean |
| 48 | [PHASE-48 - LIB_SPAWN](PHASE-48%20-%20LIB_SPAWN.md) | Multiprocessing, part 6: `LIB$SPAWN`, a subprocess CLI (and LOGINOUT through `$CREPRC`), the console's SPAWN, SYS$OUTPUT as a file, the scheduler on by default; the milestone (a MACRO parent and child, by `$CREPRC` and by `LIB$SPAWN`, passing mailbox messages and sharing files without corrupting the volume) passes under several quanta, and matches VMS's run of it — done; the close-out added the debugger's SET PROCESS, VMS's SHOW DEVICE layouts, and one more VMS run (`testdata/mp/final`, prepared) |
| 49 | [PHASE-49 - record updates and locks](PHASE-49%20-%20record%20updates%20and%20locks.md) | What the multiprocessing program left for later: RMS `$FIND`/`$UPDATE`/`$TRUNCATE`, read-write streams, RAB$V_TMO; `$GETLKI`, lock quotas, deadlock detection; file-backed sections; pending terminal reads; per-process I/O counts — planned |

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
system-service support (`SYS$CREATE`/`CONNECT`/`OPEN`/`CLOSE`/`GET`/`PUT`, and
later `SYS$RENAME`) backed
by genuine ODS-2 volume/file access via the sibling Go module
`github.com/tucats/ods2`, plus a new console `MOUNT` command attaching a
disk-image container to a device. Phase 10's existing `rms.c` port
(`internal/corevms/rms.go`) is a stopgap that just `fopen`s an arbitrary host path
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

Phase 24, requested by the user 2026-09-23 directly following Phase 11's
`.P1VECTOR` work, adds `.RMSDEF`/`.FAB`/`.RAB` to `internal/asm`: this port's
own equivalent of real MACRO-32's `$FABDEF`/`$RABDEF`/`$RMSDEF`/`$FAB`/`$RAB`
library macros, letting a fixture build a FAB/RAB by keyword
(`.FAB FAC=FAB$M_PUT, ORG=FAB$C_SEQ, ...`) instead of hand-laying-out
`.BLKB`/`.BYTE`/`.WORD`/`.LONG` blocks at byte offsets only a `;` comment
documented. Like Phase 22/23, has no `reference/eVAX` counterpart (confirmed by
grepping `asm_pseudo.c`'s own pseudo-op table); its correctness reference is
VMS 7.3's `fabdef.h`, `rabdef.h`, and `rmsdef.h`, real SDL-generated headers
(kept outside the repository since Phase 31). Introduced `internal/vmsdef`, a shared data package (consolidated
mid-planning from the narrower `internal/p1vector` this phase started from,
once it was clear the asm/RTL-shared-static-VMS-data need — P1-vector
addresses, now FAB/RAB field layouts — was going to keep recurring as more of
the VMS system-service library gets ported) holding the complete FAB
(33-field)/RAB (26-field) offset tables and a `go:generate` generator
(mirroring `internal/cpu/gen`'s own precedent) producing a 393-entry constant
table from all three headers' flat `#define` lines; `internal/rms/fab.go`/
`rab.go`/`status.go` were migrated onto this same shared table instead of their
own previously-private, separately-verified literals. `testdata/asm/
rms_roundtrip.asm` (Phase 22's own end-to-end RMS fixture) was rewritten to use
all three new pseudo-ops, surfacing one genuine pre-existing assembler
limitation along the way (a forward-referenced symbol can't be combined with an
operator, `VAX_FWDOPERATOR`) rather than a new bug. See PHASE-24.md's own
"Design decisions" section and progress log for the full detail, including the
`.P1VECTOR`-idempotency lesson (`unique=false` from the start, not discovered
as a bug afterward) this phase's own `.RMSDEF` applied up front.

Phase 25, requested by the user 2026-09-27, replaces the eVAX-derived logical
names with a VAX/VMS 7.3-faithful facility, shared by the console, RMS, and the
RTL. The user asked for that goal, and for the existing govax/eVAX behavior to
be treated as suspect rather than preserved.
- A new leaf package, `internal/lnm`, models the VMS structure: the process and
  system directories; the process, group, and system tables (`LNM$PROCESS`,
  `LNM$GROUP`, `LNM$SYSTEM`) plus tables created at run time; `LNM$FILE_DEV` as
  a real search list of tables; per-access-mode entries; search lists; and the
  `CONCEALED`/`TERMINAL` attributes. It also does RMS file-spec translation:
  leftmost-component, iterative, restarting the search order at each level,
  with a 10-level limit and loop detection.
- The DCL grammar engine gained list-valued parameters and qualifiers and
  CDU-style `DISALLOW ANY2(...)`. The console has the VMS commands, with one
  shared handler for `DEFINE`/`ASSIGN` across `/PROCESS`/`/GROUP`/`/SYSTEM`/
  `/TABLE=`.
- RMS and every console file command translate specs, fan search lists out
  per the User's Manual, and take the default device from `SYS$DISK`. MOUNT
  defines `DISK$label`.
- `$TRNLNM` was rewritten to the System Services manual, and `$CRELNM`/
  `$DELLNM`/`$CRELNT` and the pre-V4 `$CRELOG`/`$DELLOG`/`$TRNLOG` were added,
  with an assembled acceptance fixture (`testdata/asm/lnm_roundtrip.asm`).

The generated `$LNMDEF`/`$SSDEF` constants come from real VMS 7.3 source files
(kept outside the repository since Phase 31). Each eVAX behavior the phase changed, and each remaining
gap (privileges, the job table, the process-permanent-file prefix), is in
`DEVIATIONS.md`. See PHASE-25.md for the design and the per-subtask log.

Phase 26, requested by the user 2026-09-27, starts a running record of system
services added beyond the eVAX set, one subtask per service; PHASE-26.md is
meant to keep growing as more are added, and its "Conventions for implementing
a service" section is the starting point for each new one. The first batch:
- `corevms.Process`, the emulated process record (PID, username SYSTEM, UIC
  [1,4], working-set quotas), replacing the Environment's loose PID/UIC.
- `$ADJSTK` (a less privileged mode's saved stack pointer) and `$ADJWSL`
  (working-set limit, recorded but not enforced).
- `$ALLOC`, with allocation state on `iodev.Device` and `$DEVDEF` generated
  from real VMS 7.3 SDL (the SDL parser learned `union` aggregates); `$ASSIGN`
  and SHOW DEVICE/FULL honor allocation, and image rundown releases user-mode
  allocations.
- `$ASCEFC`, with a common event flag cluster table; `$SETEF`/`$CLREF`/
  `$READEF` now follow the manual (`WASSET`/`WASCLR`, `ILLEFC`, `UNASEFC`).

eVAX implements none of these, so the VMS 5.0 System Services Reference Manual
is the reference. `testdata/asm/process_services.asm` exercises all four from
assembled code. Simplifications and the event-flag behavior change are in
`DEVIATIONS.md`.

A second batch added `$DALLOC`, `$DACEFC`/`$DLCEFC`, a real `$GETJPI`/`$GETJPIW`
(21 items over `corevms.Process`, with `$JPIDEF` generated from VMS 7.3 SDL), and
`$WAITFR`/`$WFLAND`/`$WFLOR`. An unsatisfied wait re-executes the service's `XFC`
(`cpu.ErrServiceWait`), so the interval timer's interrupt, govax's only
asynchronous source, can end it; `testdata/asm/wait_timer.asm` shows exactly
that.

A third batch added `$DASSGN` and `$SETIMR`/`$CANTIM`. The timers are the RTL's
own queue: they don't depend on the guest's interval-timer interrupt, ICCS, or
IPL, but run on a new `Engine.SystemTime` that shares the interval clock's time
base (one tick = one millisecond; deterministic in quantum mode), so they fire
in any program yet agree with the clock (`testdata/asm/timer_services.asm`).

A fourth batch added `$GETTIM` (system time is now local time, as on VMS),
`$ASCTIM`/`$BINTIM`, hibernation (`$HIBER`/`$WAKE`, and `$SCHDWK`/`$CANWAK` on
the timer queue), and **AST delivery**. The RTL, acting as the executive,
decides at each instruction boundary whether an AST can run (VMS's conditions:
IPL < 2, enabled, none active in the mode) and pushes its argument list. The
engine calls the routine with its `CALLG` frame builder, through an optional
`cpu.ASTSource` interface. The routine's `RET` returns through the `SYS$CLRAST`
vector entry, which restores the interrupted state. `$DCLAST`, a per-mode
`$SETAST`, and the `astadr` arguments of `$SETIMR` and `$GETJPI` use it;
waits are interrupted and resumed as on VMS (`testdata/asm/ast_delivery.asm`,
`timer_ast.asm`).

A fifth batch added terminal `$QIO`/`$QIOW` and `$CANCEL` (a per-device-class
driver registry, `$IODEF` generated from VMS 7.3 SDL), `$SYNCH`, exit handlers
(`$EXIT`, `$DCLEXH`, `$CANEXH`: a service may now ask the engine to call a
guest routine and return to its `XFC`), `$NUMTIM`, more `$GETJPI` items,
mode-switching AST delivery (a kernel AST interrupts user code), and
`$GETSYI`.

A sixth batch added `$FAO`/`$FAOL` (a directive registry), `$GETMSG`/`$PUTMSG`
(1,426 message texts generated from the VMS 7.3 system message file's
listing), `$CMKRNL`/`$CMEXEC`, CTRL/C and CTRL/Y ASTs (the engine offers an
attention key to the services before stopping), the full `$GETDVI`, mailboxes
(`$CREMBX`/`$DELMBX` and a driver whose reads wait for writes, which gave
`$QIO` pending requests, a waiting `$QIOW`, and a real `$CANCEL`), and
`$SETPRN`, `$SETPRI`, `$FORCEX`, `$DELPRC`.

A seventh batch added VMS condition handling (exceptions dispatched to the
program's condition handlers through `SYS$SRCHANDLER` in the RTL, with Phase
20's console search kept as a fallback; `$SETEXV`; `LIB$SIGNAL`, `LIB$STOP`,
and friends; `$UNWIND` by rewriting return addresses to a `RET`), the
virtual address space services (`$CRETVA`, `$DELTVA`, `$CNTREG`, `$SETPRT`,
page locking), resource wait mode and mailbox attention ASTs, privilege
masks with `$SETPRV` and the checks services had skipped, `$SNDOPR` and
`$BRKTHRU`, rights identifiers, and disk `$QIO` (the ACP's file functions),
wired into the ods2 module through `internal/rms`.

Phase 27, requested by the user 2026-09-29, adds a `MACRO` command that
assembles `.MAR` source into a VAX object module (`.OBJ`) that a real VMS
linker accepts. It shares `internal/asm` with the console's `ASM` command:
a dialect setting and a psect-based location model let one core do both
absolute (console) and relocatable (object) assembly. A new
`internal/obj` package reads, writes, dumps, and checks the VAX object
language, which is specified in the VMS 5.0 Linker manual's chapter 7 and
the VMS 7.3 `objfmt.sdl`. Objects from the user's real VAX are the
comparison fixtures. See PHASE-27.md.

Phase 27 is complete. `MACRO source[/HOST] [/[NO]OBJECT[=file]]` (and
`govax macro`, with repeatable `--mount`/`--mount-write`) reads and writes
host files or files on mounted ODS-2 volumes, by one set of file-name
rules in `internal/rms`. The MACRO dialect reports every error before
declining to write an object.

- **Matching real MACRO.** Twelve fixtures, assembled by real VAX MACRO
  V5.4-3 on VMS 7.3, match govax's objects record for record, apart from
  traceback records, which govax doesn't write yet.
- **Accepted by real VMS.** `ANALYZE/OBJECT` finds no errors in govax's
  objects, and VMS links and runs the complete programs.
- **Work along the way.** The phase also:
  - made `ods2`'s `INITIALIZE` build volumes VMS mounts, and fixed four
    `ods2` bugs VMS found in volumes `ods2` had written;
  - made COPY keep an object's records in both directions;
  - made govax dismount volumes at the end of a session, so cached
    bitmaps aren't lost.

What MACRO-32 still lacked was planned as three phases. Phase 30, a govax
`LINK`, was done first (2026-09-30): a govax-assembled program now links
and runs inside govax without a real VAX, and real VMS runs govax's
images. Phase 28, the macro facility, is done (below). Phase 29,
listings and traceback records, is done (2026-10-04), with debugger
records deferred. Known gaps
are in `DEVIATIONS.md` under Phases 27 and 28.

Phase 28 (2026-09-30) gave MACRO-32 its macro facility, so ordinary VMS
programs that call system macros assemble. It also added a librarian.

- **Macros.** `.MACRO`/`.ENDM` with every argument form, created local
  labels, `\symbol`, `.NARG`/`.NCHR`/`.NTYPE`, the string operators,
  repeat blocks (`.REPEAT`, `.IRP`, `.IRPC`), `.MEXIT`, `.MDELETE`, and
  `.ERROR`/`.WARN`/`.PRINT`. Every one of the real STARLET.MLB's 1529
  macros loads. The console's `ASM` gets the definition side too.
- **Macro libraries.** `.MCALL`, `.LIBRARY`, `MACRO /LIBRARY=(file,...)`,
  and MACRO's automatic search, in VMS MACRO's order. STARLET.MLB is
  `SYS$LIBRARY`'s on a mounted volume, or the host directory the
  `vax.library` setting names (shared with LINK), or govax's own small
  one, built from `internal/bootdata/files/starlet.mar`.
- **A librarian.** `internal/lbr` writes libraries as LIBRARIAN lays them
  out, and the `LIBRARY` command (and `govax library`) creates, changes,
  lists, and extracts from `.MLB` and `.OLB` libraries.
- **Matching real VMS.** Nine new fixtures, assembled on the user's VAX,
  match govax's objects record for record. govax builds the fixture
  libraries byte for byte as LIBRARIAN did, and its listings and
  extractions match LIBRARIAN's. VMS assembles and links with govax's
  libraries, and LIBRARIAN changes them. See PHASE-28.md.

## The multiprocessing program (Phases 43–48)

Phases 43 to 48 (2026-10-06 to 2026-10-08) made govax run several VMS
processes at once on one engine. PHASE-43.md's Part A is the program's
plan and its decisions; each phase's doc has its progress log.

- **Processes.** System state (`corevms.System`: devices, mounts,
  mailboxes, common event flags, global sections, the lock database, the
  process table, the S0 pool) is split from process state
  (`corevms.Environment`, one per process). Each process has its own P0
  and P1 page tables, privileged stacks, and a 96-byte hardware PCB in S0;
  the CPU switches between them with the same Go code as the LDPCTX and
  SVPCTX instructions. Process 1, the console's, keeps VMINIT's layout,
  so nothing an oracle sees moved.
- **Scheduling.** `internal/sched` holds VMS's rules (the Internals book,
  chapter 10): 32 priorities, a quantum counted in instructions, boosts
  and decay, preemption at any instruction boundary below IPL 3. A
  service that must wait puts its process in a VMS wait state (LEF, CEF,
  HIB, MWAIT, SUSP) and the next computable process runs; with none, the
  machine idles to the next timer. Runs are deterministic.
- **Creating and ending processes.** `$CREPRC` (subprocesses, detached
  processes, quotas, privileges, termination mailboxes), process startup
  and rundown, `$DELPRC`, and the process-control services across
  processes; jobs and job logical-name tables; STOP and SHOW SYSTEM.
- **Communication.** Mailboxes between processes (also as RMS record
  streams, with NL:), common event flags, global sections, and a shared
  terminal whose reads don't block the machine.
- **Files.** A lock manager (`internal/lck`) with `$ENQ`/`$DEQ`; RMS file
  sharing by FAC and SHR, one shared file control block per open file in
  ods2, shared sequential files, record locks.
- **LIB$SPAWN.** A subprocess running govax's small command interpreter
  in place of DCL (RUN, MCR, foreign commands, symbols, LOGOUT), with its
  parent's symbols and logical names, completion status, event flag, and
  AST; `$CREPRC` of LOGINOUT gets the same CLI; the console has SPAWN.
- **The milestone.** `testdata/mp/msparent.mar` starts `mschild.mar` by
  `$CREPRC` or `LIB$SPAWN`; they exchange messages through two mailboxes
  and append to one shared file and a file each on an ODS-2 volume. Run
  both ways under three quanta, the output, the files, and the volume's
  structure all check (`TestMilestone`, and through `govax run`).
- **On by default.** `vax.process.scheduler` is true since Phase 48; set
  to false, only the console's process runs and `$CREPRC`/`LIB$SPAWN`
  return SS$_UNSUPPORTED.

- **Checked against VMS.** Probes 1 to 4 and the milestone ran on VMS
  (7.1 and 7.3); govax matches probe 4's report and logs and the
  milestone's output and files (`TestProbe4`, `TestMilestone_vmsLog`).

What remains unconfirmed against VMS is listed in DEVIATIONS.md
("[Phases 43–48] Multiprocessing rules chosen without a manual or probe");
`testdata/mp/final` (probe 5 and round 7 of the macro probes) is the next
VMS run. What the program left for later is Phase 49.
