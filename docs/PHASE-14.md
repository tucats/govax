# Phase 14: Interval timer & device-interrupt delivery

## Goal

Port the interrupt *admission and delivery* mechanism `vax.c`'s main loop and
`interrupt.c` implement, and use it to give the interval timer (ICCS/NICR/ICR)
and the console TTY (TXCS/TXDB/RXCS/RXDB) real, working interrupts instead of
the plain-register-store stand-in `internal/cpu/procreg.go`'s `setPrivReg`
fell back to through Phase 12. Key deliverable: assembled code can use
`LIB$PUT_OUTPUT` to print a multibyte string to the console without hanging
or crashing — the exact gap `docs/PHASE-12.md`'s own progress log and
`internal/console/asm_test.go`'s `TestAssemble_kernelThenHelloRunsBounded`
found and left recorded for this phase.

## Why this wasn't caught by Phases 03/07/09

Phase 07 ("privileged & misc instructions") and Phase 09 ("I/O & device
support") both predate any code path that actually *executes* a CHMK-driven
RTL routine — Phase 10's own RTL services are unit-tested by constructing an
argument scenario directly in `vm.Memory` and invoking the relevant registry
entry, never by running real VAX code that polls a ready-bit in a loop.
`internal/cpu/procreg.go`'s `setPrivReg` already had an honest doc comment
flagging ICCS/RXCS/TXCS/TXDB's "console/clock device modeling" as deferred —
but nothing before Phase 12 ever drove a real microkernel far enough to
notice the gap, and Phase 12 itself only recorded the finding rather than
fixing it (see this doc's own original placeholder text, now superseded).

## Design: deterministic instruction-count quantum, not a wall-clock timer

Per explicit user direction (2026-09-15), this phase uses an
instruction-count-driven "quantum" model rather than reintroducing real
wall-clock timers/goroutines into the emulator. This matters because the C
reference's *own* driving mechanism for both the quantum counter and the
interval clock is a real `SIGALRM` handler (`console.c`'s `todr_timer`) —
`vax.c`'s own "decrement every instruction" comment describes aspirational
intent, but the actual per-instruction decrement it would take
(`vax.quantum.current--`) is commented out in favor of the signal handler's
real-time-driven one. `internal/cpu/interrupt.go`'s `Engine.tickQuantum`
(called once per `Engine.Step`) is this port's from-scratch replacement:
deterministic, reproducible, and consistent with this project's design
elsewhere (see `docs/PLAN.md`'s locked-in decisions). A useful side effect:
"SET QUANTUM 1" — a value calibrated for the C source's millisecond-scale
wall clock — does *not* translate literally to this model (it would mean
"re-evaluate every single instruction," firing the interval timer roughly
every 10 instructions instead of every ~10ms/thousands of instructions);
the default quantum (20, matching `initialization.c`'s own non-alarm-driven
default) is the sane choice for this model, and the phase's own tests use it.

## Deliverables

- **Quantum-driven interrupt admission/delivery core**
  (`internal/cpu/interrupt.go`): `Engine.Interrupt` (port of `interrupt()`:
  immediate delivery when unmasked and nothing pending, otherwise queued for
  quantum-boundary aging), `Engine.tickQuantum`/`scanInterruptQueue` (port of
  `execute_vax`'s own quantum block: interval-clock tick, one queued
  interrupt admitted per boundary), and `Engine.deliverPendingInterrupt`,
  wired into the top of `Engine.Step`. IPL (case 18) and SIRR (case 20) in
  `procreg.go` now correctly call `Interrupt` for their own pending-interrupt
  delivery, previously deferred for lack of this mechanism.
- **ICCS/NICR/ICR interval timer** (`procreg.go`'s ICCS case,
  `interrupt.go`'s `tickIntervalClock`): RUN/XFR/SGL/IE/ERR bit semantics,
  and a real quantum-tick-driven countdown that fires `EXC$INTERVAL` at
  IPL 22 and reloads from NICR — the actual driving algorithm ported from
  `emul_procreg.c`'s case 24 combined with `console.c`'s `todr_timer` (the
  *real* mechanism; `vax.c`'s own copy is dead code — see the design note
  above), substituting one quantum tick for one wall-clock tick.
- **Real TXCS/TXDB/RXCS/RXDB console-I/O device modeling**
  (`procreg.go`'s TXCS/TXDB/RXCS cases, `emulMfpr`'s RXDB read side effect):
  TXDB writes a real byte via `SystemServices.ConsoleWriteByte` and admits a
  genuine `EXC$CONWRITE` interrupt when TXCS's IE bit is set, closing the
  exact `EXE$$PUT_CONSOLE`/`exe$tx_ready` spin-forever gap
  `TestAssemble_kernelThenHelloRunsBounded` documented. RXDB reads pull a
  real byte from the same console input source `XFC$CONSOLE_READ` uses
  (fixing a confirmed-broken C-reference behavior — see below) and RXCS
  admits `EXC$CONREAD` on the IE-already-set/DON-pending case, matching
  `emul_procreg.c`'s own case 32. `Engine.DeliverConsoleByte` ports what
  `poll_keyboard`'s own (Mac/Windows-only) intended behavior does — deposit
  a byte, set DON, admit an interrupt if IE is set — as a real, callable
  primitive.
- Two real, confirmed C-reference/Go-port bugs found and fixed along the way
  (both logged in `docs/DEVIATIONS.md`, both clear-cut fixes rather than
  deferrals — see that doc for full detail):
  - `interrupt.c`'s own `handle_fault` saves the *stale* `instruction_PC`
    (the previous instruction's address, not the current one) as an
    interrupt's return PC — a genuine architectural bug in the C reference.
    This port's `deliverPendingInterrupt` uses the current PC instead.
  - This port's own `emulChmx` (CHMK/CHME/CHMS/CHMU, Phase 07) never updated
    `e.instructionPC` before faulting, so a CHMK handler's RET/REI resumed
    at the CHMK instruction's own address instead of the instruction after
    it — invisible for a single CHMK, an infinite re-trap loop for any
    caller issuing CHMK a second time after the first one's handler
    returns (exactly `kernel.asm`'s own `LIB$PUT_ONE` per-byte CHMK loop).
    This was the actual, final blocker on the phase's key deliverable —
    without it, only the first character of any string ever printed.
  - Also fixed, smaller: the C reference's own TXCS/TXDB call sites use the
    literal `0x20` (32 decimal) for what their own comments call "IPL 20"
    — inconsistent with RXCS's own call site (which correctly uses decimal
    20) and with the real VAX SRM's console-terminal IPL assignment (20).
    Corrected to decimal 20 directly rather than replicated, since the C
    source's own comment states the intended value.
- The IPL literal question the original placeholder text flagged as
  needing SRM research is resolved by the finding above: console-terminal
  I/O is architecturally IPL 20 (`0x14`) for both directions; the interval
  clock's own call site already correctly used decimal 22, unrelated to
  this inconsistency.

## Deliberately out of scope

- **Idle detection** (per the user, 2026-09-15): a program legitimately
  waiting for an interrupt (a real OS's idle loop, or a poll loop like
  `EXE$$PUT_CONSOLE`'s own) will busy-spin `Engine.Step` until the quantum
  tick delivers something — fine for a bounded test, wasteful for a live
  interactive session. Recognizing this and reporting a distinct "idle"
  outcome (`simh`-style) needs real design work with no current consumer to
  validate it against; left for a future phase if it becomes a real problem.
- **Real, OS-level non-blocking keyboard polling**: `Engine.DeliverConsoleByte`
  is the correct, working interrupt-admission primitive, but nothing in this
  phase wires a live terminal byte source into it — `vax.c`'s own
  `poll_keyboard` is a permanent no-op outside Mac/Windows builds (including
  this project's own `LINUX86` reference target), so there's no working
  upstream behavior to port either, and no current fixture exercises this
  path (`LIB$GET_INPUT`/`EXE$INPUT` already work correctly via a Go RTL shim
  that reads `Console.In` directly, bypassing RXCS/RXDB entirely — see
  `internal/rtl/input.go`). A real interactive front end wiring a live byte
  source to `DeliverConsoleByte` is future work.
- **Assembler/session-placement issue found during this phase's own testing,
  not fixed here**: assembling a second file in the same `asmSession` after
  a file that switches `.region` (`kernel.asm`'s own trailing `.region p0`)
  can place the new file's code overlapping the first file's own S0-region
  code, becoming a real protection violation once page protection is
  actually active (kernel.asm's own `.console SET PAGE ... PROT=...`
  pseudo-ops, which *do* work in this port). This is a distinct,
  `internal/asm`/`internal/console` session-bookkeeping concern (not
  interrupts/timers), found only because this phase's own testing was the
  first to combine a large, `vax.init`-realistic `VMInit` allocation with
  actually running kernel.asm's own initialization code between two
  `Assemble` calls — most existing tests use a small, non-representative
  `VMInit` where the bug happens not to manifest. `docs/PHASE-14.md`'s own
  end-to-end test works around it by not re-running the full `vax.init`
  boot sequence (enabling TXCS<IE> directly instead of via
  `GO EXE$INITIALIZE`). Worth a dedicated look in a future phase; not
  chased further here.

## Progress Log

### 2026-09-15 — Sub-phase 1: quantum-driven interrupt admission/delivery core

- `internal/cpu/interrupt.go`: `Engine.Interrupt`, `tickQuantum`,
  `scanInterruptQueue`, `deliverPendingInterrupt`, wired into `Engine.Step`.
  New `Exception` constants `ExcInterval`/`ExcConRead`/`ExcConWrite`/
  `ExcSoftware1` (`internal/cpu/exception.go`).
- `procreg.go`'s IPL (case 18) and SIRR (case 20) now call `Interrupt` for
  their own pending-interrupt admission, matching `set_priv_reg` exactly
  (previously a plain register store with the interrupt-delivery half
  silently skipped).
- Found and fixed (immediately, not deferred — a clear-cut correctness bug,
  not an ISA question): `interrupt.c`'s own `handle_fault` saves the stale
  `instruction_PC` as an interrupt's return address; see
  `docs/DEVIATIONS.md`.
- `internal/cpu/interrupt_test.go`, plus updated `procreg_test.go` SIRR
  tests (the existing `TestEmulMtprSirrQueuesSoftwareInterrupt` asserted the
  old, incomplete "always latch" behavior; split into
  `TestEmulMtprSirrLatchesWhenAtOrAboveCurrentIPL`/
  `TestEmulMtprSirrDeliversImmediatelyWhenAboveCurrentIPL`, plus a new
  `TestEmulMtprIPLAdmitsLatchedSoftwareInterruptOnceExposed`).
- `go build ./...`, `go vet ./...`, `go test ./...` all clean; committed.

### 2026-09-15 — Sub-phases 2-3: ICCS interval timer, TXCS/TXDB console output, and closing the key deliverable

- `procreg.go`'s ICCS/RXCS/TXCS/TXDB cases and `emulMfpr`'s RXDB read side
  effect implemented for real (see Deliverables above);
  `interrupt.go`'s `tickIntervalClock`/`DeliverConsoleByte` added.
  `internal/cpu/devices_test.go` covers all of it.
- Chasing the phase's own key deliverable end to end
  (`TestAssemble_kernelThenHelloRunsBounded`, updated to enable TXCS's IE
  bit and assert a clean completion with "Hello world" actually reaching
  the console output, not just "doesn't hit the step cap") surfaced three
  more findings along the way, in order:
  1. `newRunnableConsole`'s small `VMInit` allocation can't support
     `GO EXE$INITIALIZE` (its user-mode stack pointer, a fixed constant,
     lands outside the mapped P1 region unless `VMInit`'s own P1 size is
     large enough to reach it) — worked around by not running
     `EXE$INITIALIZE` in this test, enabling TXCS<IE> directly instead
     (see "Deliberately out of scope" above for the session-placement issue
     found alongside this).
  2. `kernel.asm`'s own boot-message print (`EXE$PRINTINITMSG`, gated by
     `exe$verbose`) hit the CHMK bug below independently, printing "K"
     (`"Kernel initialized..."`'s first letter) forever — not investigated
     further once isolated as the same root cause as finding 3, and worked
     around by silencing `exe$verbose` in the test rather than needed for
     the actual deliverable.
  3. The `emulChmx` return-address bug (see Deliverables above) — the real,
     final blocker. `internal/cpu/changemode_test.go`'s
     `TestEmulChmxReturnsPastTheChmxInstruction` regresses it directly.
  4. hello.asm's own delay loop ("`movl #1000000,r5`") is ~16.7M iterations
     under this assembler's default-hex-radix convention, not 1M under a
     literal decimal reading — not a bug, just an undersized step cap in
     the test itself once the real hang was fixed; corrected to 40M.
- `go build ./...`, `go vet ./...`, `go test ./...` all clean;
  `TestAssemble_kernelThenHelloRunsBounded` now completes in ~2s. Committed.
  Phase 14 complete: quantum/interrupt core, interval timer, and
  console-I/O device interrupts are all real and tested; the key
  deliverable (a multibyte `LIB$PUT_OUTPUT` string prints cleanly) is
  verified end to end.
