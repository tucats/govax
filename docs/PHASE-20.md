# Phase 20: RTL shim resolution fix, and console-native exception reporting (CHF)

## Goal

Fix a real bug in Phase 13's own `SHIM$` resolution (a code-0 `.shim` table
entry was being treated as "dead" and given a synthesized, fake dispatch
stub instead of being resolved to the real, hand-written `kernel.asm`
routine it names) that made every real `.exe` fixture with a `DECC$SHR`
dependency crash immediately on its very first call into the C runtime
library; and port `interrupt.c`'s Condition Handling Facility (`chf()`) and
its console-native exception-reporting fallback (`format_exception()`),
which `docs/PHASE-13.md`'s own `RUN` port never carried over -- so a fault
whose SCB vector is kernel.asm's `console$handler` sentinel (`0xFFFFFFFF`)
aborted the whole `RUN` command with a raw Go error instead of the C
source's own "look for a VMS condition handler, else report the exception
cleanly and halt" behavior.

Both were found investigating the same user report: running
`testdata/exe/simple.exe` failed with what looked like "a JSB to a HALT
instruction" partway through C-runtime startup.

## Scope / C source mapping

- `reference/eVAX/eVAX/Source/Assembler/asm_pseudo.c`, case 33 (`.SHIM`) --
  the real branch behavior a code-0 entry gets (`get_symbol` against the
  routine's own name), as opposed to a nonzero-code entry's synthesized
  `MOVL #code,R0` / `XFC #0x7D` / `RET` stub. `internal/console/shim.go`'s
  own doc comment is the canonical write-up of the bug and fix; see this
  phase's progress log below for how it was found.
- `reference/eVAX/kernel.asm`'s second `.shim` table (lines ~1546-1555) --
  every code-0 entry there names a real, already-assembled routine
  elsewhere in the same file (`decc$main`, `decc$exit`, `decc$strcat`,
  `decc$strcpy`, `decc$strlen`, `decc$calloc`, `cma$tis_errno_get_addr`,
  `lib$put_output`, and two data labels, `decc$$gl___ctypea`/
  `decc$$ga___ctypet`) -- confirmed present as real symbols once
  `kernel.asm` is assembled, not merely asserted.
- `reference/eVAX/eVAX/Source/CPU/interrupt.c` -- `chf()` (the call-frame
  walk looking for an installed VMS condition handler) and
  `format_exception()` (the console-native "print a `%VAX-E-CONHANDLER`
  diagnostic and halt" fallback when no handler wants the exception, or
  none exists), both only ever reached via `handle_fault()`'s own
  `vector == 0xFFFFFFFF` check -- kernel.asm's own `console$handler`
  convention (see its own comment around its two `.scb` tables, lines
  ~1562-1580: "this equates to -1, and causes the emulator to report on
  the exception using non-VAX native code, and halts the emulation").
- `reference/eVAX/eVAX/Source/Console/errors.c`'s `vaxexcept()` -- a
  longer-named exception table used only for this one message in the C
  source; not ported separately (see `internal/console/chf.go`'s own doc
  comment on reusing `exceptionName`'s existing short SCB-style names
  instead, matching this port's own established precedent elsewhere).

## The bug and how it was found

`docs/PHASE-13.md`'s own `internal/console/shim.go` (Phase 13, sub-phase 3)
read `kernel.asm`'s `.shim` table as "code 0 means dead, no numeric
dispatch" and synthesized the *same* `MOVL`/`XFC`/`RET` stub for every
entry regardless of its code -- including the leading 2-byte placeholder
that stub format reserves for a CALLS-style entry mask. That claim was
never actually checked against `asm_pseudo.c`'s own code-0 branch, which
does something else entirely: `get_symbol(name)` against the routine's own
label. `kernel.asm` itself says why this matters, right above its own
`decc$main:` label: "the MAIN entry initialization is via JSB not CALL" --
`decc$main` (and every other code-0 entry) is a plain label, not a
`.ENTRY` procedure, with no entry mask word of its own. A G^ fixup
resolving `SHIM$DECC$SHR_00000000` (i.e. `decc$main`) to this port's
synthesized fake stub and then reaching it via `JSB` (as the compiled C
runtime startup code actually does, matching kernel.asm's own comment) ran
that fake stub's leading placeholder bytes directly as an instruction:
`0x00 0x00` is `HALT`, and `HALT` outside kernel mode is a privileged-
instruction fault -- exactly the "JSB to a HALT instruction" symptom
reported. Confirmed directly: `RUN SIMPLE` failed with `PC` sitting exactly
on `SHIM$DECC$SHR_00000000`, one instruction into that synthesized stub.

## Deliverables

- `internal/console/shim.go`: `shimEntry` gained a `name` field (the
  routine's own label, e.g. `DECC$MAIN`); `ensureShims` now branches on
  `code == 0` exactly like `asm_pseudo.c`'s own `.SHIM` case 33 --
  resolving by symbol lookup instead of synthesizing a stub -- and (for a
  nonzero-code entry, matching the C source's own `set_symbol` call in that
  branch too) additionally defines the routine's own bare name as a second
  symbol pointing at its synthesized stub. Requires `kernel.asm` to already
  be assembled (true of every real call site -- `Console.Run` always runs
  after `vax.init`'s own boot sequence); a missing routine symbol is a
  clear internal error (`LIB$UNRESOLVED`), not a silent fallback.
- `internal/console/show.go`'s `ShowShim` (`SHOW SHIM`) updated to match:
  addresses are read back from each entry's own registered `SHIM$...`
  symbol rather than recomputed from the synthesized-stub page's base
  address (wrong for a code-0 entry, which lives wherever `kernel.asm`
  defined its real routine instead), and a code-0 entry is reported as
  "resolved to `<name>`" rather than "dead".
- `internal/cpu/handlefault.go`: `Engine.HandleFault`'s existing
  `0xFFFFFFFF`-vector case now returns a new `*ConsoleHandlerFault` (the
  faulting instruction's own PC, PSL, R0/R1, and the original `*Fault`)
  instead of the bare `ErrNoExceptionHandler` sentinel -- still matched by
  `errors.Is(err, ErrNoExceptionHandler)` via `Unwrap`, so every existing
  caller/test keeps working unchanged.
- `internal/console/chf.go` (new): `Console.handleConsoleFault` (the
  `format_exception` port), `Console.chf` (the call-frame walk),
  `Console.invokeHandler` (builds the VMS signal-argument/mechanism-
  argument vectors and calls a found handler via `Console.Call`, the same
  "nested call, run to completion" primitive Phase 13 built for
  `LIB$INITIALIZE`), and `Console.formatException` (the `%VAX-E-CONHANDLER`
  diagnostic + halt). Fixes one real C-source bug along the way: `chf()`'s
  own `if (rc && 0x00000001)` is a `&&` where a `&` was clearly meant (VMS's
  `SS$_CONTINUE` is bit 0 of R0) -- with the original, *any* nonzero R0
  (including `SS$_RESIGNAL` and friends) would short-circuit true and stop
  the search after the very first installed handler, silently breaking
  resignaling. A plain C typo against an unambiguous, well-documented VMS
  convention, not an ISA fidelity question -- fixed directly per this
  project's bug-fixing policy for clear-cut cases, not logged to
  `docs/DEVIATIONS.md`.
- `internal/console/execute.go`'s `reportStopReason` now recognizes
  `*cpu.ConsoleHandlerFault` (via `errors.As`, alongside the existing
  `*cpu.FaultBreak` check) and routes it to `handleConsoleFault` instead of
  propagating it as a command error -- so `RUN`, `CALL`, and `STEP` all
  report an otherwise-unhandled fault the same way the real console does:
  a clear diagnostic and a clean halt, not an aborted command.
- `internal/cpu/engine.go`: `Engine.Halt`/`Engine.ClearHalted`, small
  exported setters that `format_exception`/`chf` need and nothing
  previously exposed (matching the existing `Attention`/`AttentionRequested`
  pattern).

## Milestone check

Every fixture in `docs/PHASE-13.md`'s own named milestone list now runs to
a *real* completion, not just a "bounded, reported outcome" -- confirmed by
actually reading each program's own output, not just its terminating error:

- `simple.exe`, `getvm.exe`, `dbl.exe`: clean completion (`RET` through the
  console's own `SentinelReturn` frame).
- `put.exe`: prints its real message ("This is a test / that I am
  performing.") via `kernel.asm`'s own hand-written, interrupt-driven
  `LIB$PUT_OUTPUT` -- the first fixture to actually exercise that routine
  for real, rather than dying before ever reaching it.
- `putc.exe`: prints "Hello, world".
- `sieve.exe`: a genuine CPU-bound benchmark (sieve of Eratosthenes to
  100,000) that now actually runs its real code and completes, printing
  "The last prime is 99991" -- previously reported (Phase 13) as hitting an
  access violation, which was this same shim bug, not a real fault in the
  benchmark itself.
- `cli.exe`: still reaches a real, unimplemented `SYS$`/`SHIM$` entry point
  and faults (`PRIV`) -- but now via this phase's own console-native
  reporting path (`%VAX-E-CONHANDLER, PRIV, ...`) instead of aborting `RUN`
  with a raw error, matching this phase's own "report clearly, halt
  cleanly" bar.

## Progress Log

### 2026-09-16 — Both fixes, milestone re-check -- Phase complete

- Root-caused the user's "JSB to a HALT instruction" report to
  `internal/console/shim.go`'s code-0 `.shim` handling (see "The bug and
  how it was found" above); fixed by resolving code-0 entries against
  `kernel.asm`'s own already-assembled routine symbols instead of
  synthesizing a stub, matching `asm_pseudo.c`'s own `.SHIM` case 33
  exactly. `TestEnsureShims_definesSymbolsAndIsIdempotent` extended to
  assert `SHIM$DECC$SHR_00000000` resolves to the real `DECC$MAIN` address,
  not a synthesized-page address.
- Ported `chf()`/`format_exception()` (`internal/console/chf.go`) so a
  fault reaching kernel.asm's `console$handler` SCB sentinel is handled the
  way the reference tool actually handles it, not aborted -- this is what
  `cli.exe`'s still-real `PRIV` fault now demonstrates end-to-end.
  `chf_test.go` covers: no call-frame chain at all, a frame chain with no
  installed handler, a handler that continues (checking R0/R1/PC are
  correctly reloaded from the mechanism/signal arrays afterward, restoring
  the *original* fault state, not wherever the nested handler call itself
  left registers), and a handler that declines (checking R0/R1 are restored
  to their pre-call values before the search continues) --
  `internal/cpu/handlefault_test.go` separately checks
  `HandleFault`'s new `*ConsoleHandlerFault` carries the right PC/PSL/R0/R1.
- Every `internal/console` test that exercises `ensureShims`/`Console.Run`
  end-to-end now assembles `testdata/asm/kernel.asm` first (previously
  several didn't need to, since the old, wrong code-0 handling never
  actually depended on it) -- `dispatch_test.go`, `image_test.go`,
  `run_test.go`, `shim_test.go`, `show_test.go`. `TestRun_everyMilestoneFixture`
  additionally now boots a real-sized console (`newBootableConsole`,
  matching `vax.init`'s own `p0=2048/p1=8192/s0=2048/ksp=20` sizing rather
  than the lighter-weight `newRunnableConsole` every other image-load/fixup
  test shares) and runs `EXE$INITIALIZE` first (`runKernelInitialize`,
  matching `vax.init`'s own `go exe$initialize` boot step) -- both are
  genuine prerequisites for `put.exe`'s real, interrupt-driven
  `LIB$PUT_OUTPUT` to ever get a TXCS-ready interrupt delivered at all, not
  new requirements this phase introduced; they were simply never exercised
  before because no fixture ever reached that code path for real. Also
  bumped `sieve.exe`'s own step budget in that same test (60,000,000 vs.
  every other fixture's 2,000,000) once it became clear it's a genuine,
  correctly-terminating benchmark, not a hang.
- `go build ./...`, `go vet ./...`, and `go test ./...` all clean.
  `docs/PHASE-13.md`'s own scope note about code-0 `.shim` entries being
  "dead" corrected in place, pointing here.
