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

Phase 16 is an audit-only inventory (not started; no code changes), created
2026-09-15 at the user's request: a catalogue of `SHOW`/`CLEAR`/`SET` console
commands the C reference implements that this port doesn't yet, plus a few
cross-cutting console mechanisms (watchpoints, instruction/fault-kind breakpoints,
the DCL `/entry=` redirect) found missing along the way — see PHASE-16.md.
