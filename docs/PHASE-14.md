# Phase 14: Interval timer & device-interrupt delivery

## Goal

Port the interrupt *admission and delivery* mechanism `vax.c`'s main loop and
`interrupt.c` implement — the piece nothing before this phase has touched — and use
it to give the interval timer (ICCS/NICR/ICR) and the console TTY (TXCS/TXDB/RXCS/
RXDB) real, working interrupts instead of the plain-register-store stand-in
`internal/cpu/procreg.go`'s `setPrivReg` currently falls back to.

**Status: not started. This document is a planning placeholder**, written during
Phase 12's own integration work once running `testdata/asm/kernel.asm` for real (for
the first time, via Phase 12's new `ASM`/`CALL` console commands) found that its
`EXE$$PUT_CONSOLE`/`LIB$GET_INPUT` routines spin forever waiting for an
`EXC$CONWRITE`/`EXC$CONREAD` interrupt this port has never delivered — see
`docs/PHASE-12.md`'s sub-phase 2 progress log and
`internal/console/asm_test.go`'s `TestAssemble_kernelThenHelloRunsBounded`. Recorded
now, per the user's own request while that finding was still being chased, so the
shape of the problem doesn't have to be rediscovered when this phase actually starts.

## Why this wasn't caught by Phases 03/07/09

Phase 07 ("privileged & misc instructions") and Phase 09 ("I/O & device support")
both predate any code path that actually *executes* a CHMK-driven RTL routine —
Phase 10's own RTL services are unit-tested by constructing an argument scenario
directly in `vm.Memory` and invoking the relevant registry entry (see
`docs/PHASE-13.md`'s notes on why Phase 10 didn't need a working image loader), never
by running real VAX code that polls a ready-bit in a loop. `internal/cpu/procreg.go`'s
`setPrivReg` already has an honest doc comment flagging ICCS/RXCS/TXCS/TXDB's
"console/clock device modeling" as deferred to Phase 09 — but Phase 09 itself only
built the device-*table*/logical-*name* abstraction (`internal/io`), not any
interrupt-generating behavior for a specific device, and nothing before Phase 12
ever drove a real microkernel far enough to notice the gap.

## Scope / C source mapping

- `reference/eVAX/eVAX/Source/CPU/vax.c`'s `execute_vax` main loop (~lines 110-310):
  the "quantum" mechanism — a counter decremented every instruction, and on
  reaching zero, (1) reloading itself, (2) advancing the interval-clock/ICCS state,
  (3) polling the keyboard on a coarser UI-quantum boundary, and (4) scanning
  `vax.iqueue` for a pending interrupt whose `age` has reached zero and whose IPL
  exceeds the current PSL IPL, delivering exactly one per quantum tick via
  `set_fault`. This is **instruction-count-driven, not wall-clock-driven** — no
  timers or threads anywhere in it.
- `reference/eVAX/eVAX/Source/CPU/interrupt.c`'s `interrupt()` (~line 491): the
  admission routine every device-interrupt call site (TXCS/TXDB/RXCS/RXDB writes,
  the ICCS timer tick) goes through — either sets `vax.interrupt_pending` directly
  (if nothing else is already pending/masked) or queues a `struct INTERRUPT`
  (code/IPL/age) onto `vax.iqueue` for the main loop to age out later.
- `reference/eVAX/eVAX/Source/CPU/emul_procreg.c`'s `set_priv_reg`, `case 24`
  (ICCS, ~line 236: RUN/XFR/SGL/IE/ERR bit semantics, `vax.clock`/`vax.NICR`/
  `vax.ICR`), `case 32`/`34` (RXCS/TXCS, ~line 283/297: the always-instantly-ready
  console device, IE-gated `interrupt()` calls at IPL 0x14/0x20 depending on
  reading — the two call sites disagree on the exact IPL constant, worth
  double-checking against the SRM rather than copying either blindly), and
  `case 33`/`35` (RXDB/TXDB: the actual byte transfer, also IE-gated interrupts).
  Already partially ported (the bit-level register semantics minus any interrupt);
  see `internal/cpu/procreg.go`'s `setPrivReg`/`emulMfpr`.
- `testdata/asm/kernel.asm`'s own consumers, useful as an executable spec: `.scb
  exc$conwrite, exe$tx` / `.scb exc$conread, exe$rx` (the ISR entry points —
  `exe$tx`/`exe$rx` presumably set the polled memory flags
  `exe$tx_ready`/an analogous read-ready flag back to 1); `EXE$$PUT_CONSOLE`
  (CHMK 0)/`EXE$$GET_CONSOLE` (CHMK 2)'s own poll loops; `.set` `VAX$PR_ICCS`/
  `VAX$PR_NICR`-style symbols already seeded in `internal/asm/builtins.go` if this
  phase's own test fixtures need to program the timer directly.

## Key design questions (not yet decided)

- **Where does the quantum tick live in this port's own step loop?** `internal/cpu`'s
  `Engine.Step()` executes exactly one instruction with no notion of a "loop" at all
  — the loop lives in each *caller* (`Console.Call`/`Execute`, `console.callBounded`
  in tests, `cmd/govax`'s own REPL). A quantum-driven tick that's supposed to fire
  every N instructions regardless of which caller is stepping probably belongs
  inside `Engine` itself (a counter field, checked at the top or bottom of `Step`)
  rather than duplicated in every caller — needs a design pass before writing code,
  not an assumption.
- **Interrupt queue representation**: a plain slice of `{code, ipl, age}` mirroring
  `struct INTERRUPT`/`vax.iqueue` is probably sufficient — this project has
  generally preferred a plain, obvious Go structure over porting a C linked-list
  shape verbatim (see e.g. `internal/io`'s `LogicalNameTable` split, Phase 09).
- **IPL constants**: `emul_procreg.c`'s own two device-interrupt call sites use
  different literal IPL values for what both comment as "Console Term
  Trans"/similar (0x14 in one MTPR case's comment math, 0x20 in the actual
  `interrupt()` call argument in a couple of places, 0x10 implied elsewhere) —
  worth resolving against the VAX SRM's real IPL assignments (console terminal is
  architecturally IPL 20 = 0x14) rather than copying whichever literal happens to
  be closest, since this is exactly the kind of "the C source's own comment doesn't
  agree with itself" situation `docs/DEVIATIONS.md`'s policy exists for.
- **Idle detection** (explicitly *not* to be implemented yet, per the user
  (2026-09-15) — recorded here only so the idea isn't lost): once real interrupt
  delivery exists, a program that's legitimately waiting for one (like
  `EXE$$PUT_CONSOLE`'s own poll loop, or a real OS's idle loop) will busy-spin
  `Engine.Step()` until the quantum tick eventually delivers something — fine for a
  bounded test, wasteful for an interactive session. `simh` (a mature, widely-used
  VAX/PDP simulator) treats certain self-referencing branch patterns (a `JMP` to its
  own address is its specific example) not as a real infinite loop but as a cue that
  the simulated CPU can go idle until the next timer event, rather than burning host
  CPU re-executing the same no-op branch. A govax equivalent would need: (1) a way
  to recognize "this instruction stream provably cannot change state before the next
  interrupt" (simh's narrow self-branch check, or something broader), and (2) a
  defined "idle" outcome distinct from both "faulted" and "ran to completion" for
  `Console.Call`/`Execute` to report — neither exists yet, and both need real design
  thought (not a quick add) before this is attempted. Whether Go timers/goroutines
  ever belong here (e.g. to let idle time actually elapse in wall-clock terms for an
  interactive console session, as opposed to the deterministic instruction-count
  model everywhere else in this emulator) is part of the same open question — the
  step-count-driven model matches this project's existing architecture and test
  determinism far better than real concurrency would, so the bar for introducing
  goroutines/timers here should be high.

## Deliverables (draft — expect revision once this phase actually starts)

- An `Engine`-level quantum/instruction counter and a pending-interrupt admission
  queue, ported from `interrupt()`/`vax.c`'s own scan loop.
- Real ICCS/NICR/ICR interval-timer modeling: `RUN`/`XFR`/`SGL` bit semantics
  already partially there in spirit (plain register store); wire the actual
  clock-tick-on-quantum-boundary behavior and its IPL 0x18-ish (`EXC$INTERVAL`,
  already a builtin symbol at 0xC0) interrupt.
- Real TXCS/TXDB/RXCS/RXDB console-I/O interrupts (`EXC$CONWRITE`/`EXC$CONREAD`,
  0xFC/0xF8), closing the exact gap `TestAssemble_kernelThenHelloRunsBounded`
  documents — that test's own doc comment says to update its expectations once this
  lands.
- Whatever minimal idle/quiescent-detection strategy comes out of the open question
  above, if this phase's own scope ends up including it (may be split into its own
  follow-up phase instead, once the shape is clearer).

## Progress Log

_Not started. This document exists as a planning placeholder only — see the header
note above._
