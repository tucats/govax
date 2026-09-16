# Phase 18: Flow of control — `STEP`/`SET STEP`/`SHOW STEP_MODE`, and a future home for breakpoints/watchpoints

## Goal

`docs/PHASE-16.md`'s audit catalogued `SHOW STEP_MODE` (sub-phase 1c) and
`SET STEP` (sub-phase 3) as blocked on state this port didn't track at all:
`STEP`'s own `/OVER`/`/INTO`/`/RETURN` behavior, matching
`reference/eVAX/eVAX/Source/Console/console_step.c` and the
`vax.console.singlestep`/`STEP_OVER`/`STEP_RETURN` handling woven into
`vax.c`'s `execute_vax` fetch-decode-execute loop. Extracted into its own
phase (rather than staying a `PHASE-16.md` sub-phase) at the user's own
request, 2026-09-15: the work touches the CPU engine (a call-like-
instruction classifier, a one-shot internal breakpoint mechanism), not just
the console command surface `PHASE-16.md`'s other sub-phases are scoped to.

**Status: STEP/SET STEP/SHOW STEP_MODE complete.** This document is also,
per the user's own suggestion when the phase was created, the intended home
for two related but not-yet-implemented mechanisms `docs/PHASE-16.md`
sub-phase 4 already flagged as missing entirely — instruction-level
(opcode) breakpoints and fault-kind breakpoints — plus the watchpoint
subsystem that sub-phase also catalogued (`SET WATCH`, `SHOW WATCHPOINTS`,
`CLEAR WATCH`, none of which exist yet). None of those three are
implemented by this phase; they're named here as likely next sub-phases
under this same "flow of control" umbrella rather than left to spawn a
fourth or fifth STEP-adjacent phase document later.

## Scope / C source mapping

- `reference/eVAX/eVAX/Source/Console/console_step.c` — `console_step`, the
  `STEP` command itself: parses an optional `/OVER`, `/INTO` (also `/IN`,
  `/INSTRUCTION`), or `/RETURN` qualifier (defaulting to
  `vax.console.stepmode`), an optional starting address, then drives
  `execute_vax` with `vax.console.singlestep` set accordingly.
- `reference/eVAX/eVAX/Source/CPU/vax.c` — `execute_vax`'s own
  `vax.console.singlestep` handling (lines 386-396 and 522-556) and
  `break_over_instruction` (the `BSBB`/`JSB`/`BSBW`/`CHMK`/`CHME`/`CHMS`/
  `CHMU`/`CALLG`/`CALLS` "is this a call-like instruction" classifier
  STEP/OVER uses to decide whether to run a called routine to completion
  rather than step into it).
- `reference/eVAX/eVAX/Source/Console/console_show.c:1563` — `get_return`,
  the single-frame FP-chain read STEP/RETURN uses to find the current
  procedure's return address (the same frame layout `show_calls`/`SHOW
  CALL_FRAMES` already decodes in this port — see `internal/cpu/call.go`'s
  `emulRet` and `internal/console/show.go`'s `ShowCallFrames`).
- `console_set.c:450-486` (`SET STEP`) and `console_show.c:765` (`show_step`,
  `SHOW STEP_MODE`) — the persistent-default read/write pair, matching this
  port's existing `SET TRACE`/`SHOW TRACE` (`docs/PHASE-17.md`) in shape.
- `internal/console/step.go` (new), `execute.go` (breakpoint
  infrastructure, shared with `Execute`/`Call`), `dispatch.go` (`cmdStep`,
  `cmdSet`'s new `STEP` case, `SHOW_STEP` grammar binding), `machine.go`
  (`Console.StepMode`), `set.go` (doc comment only).

## Design decisions

### STEP/OVER never double-decodes the call instruction

`vax.c`'s own STEP_OVER handling decodes the current instruction once to
classify it (`opcode.index`), then — for *either* outcome (call-like or
not) — resets `vax.PC` and decodes the **same instruction a second time**
(`vax.c:393-395`) purely to force its own local `disasm` flag on for
display. Since VAX operand-specifier evaluation itself has side effects
(autoincrement/autodecrement modes mutate the addressed register at decode
time, not at execution time — see `internal/cpu/operand.go`), decoding the
same instruction twice double-applies any such side effect for a
STEP/OVER'd instruction that happens to use one — a real, if obscure, C-side
fidelity bug (not logged to `docs/DEVIATIONS.md`: it's a debugger-only
double-decode, not a hardware/ISA execution-semantics question, and it's
avoidable in this port at essentially no cost — see below — rather than
something to replicate).

This port's `cpu.Engine.Step` decodes exactly once no matter what (it has
no console/debugger awareness at all — see `docs/PHASE-03.md`), and already
exposes the just-executed instruction's `Instruction` and `NextPC` via
`Engine.LastDecoded()` (added by `docs/PHASE-17.md` sub-phase 8 for a
different purpose — the `DebugFullDisasm` operand dump). `step.go`'s
`stepOver` reuses exactly that: run the instruction once via the ordinary
`Engine.Step`, then classify `LastDecoded().Instruction.Name` against
`stepOverInstructions` and — if it matches — arm a one-shot breakpoint at
`LastDecoded().NextPC`, the address the real decode already computed. No
second decode, no doubled side effect, and no per-opcode return-address
logic (`NextPC` is "the address after this instruction's operands" for
every classified opcode uniformly, whether it's a `CALLS`-style frame-based
call, a `JSB`/`BSBB`/`BSBW` plain-stack-push call, or a `CHMK`-style
change-of-mode — the temporary breakpoint fires whenever *anything* returns
control there, regardless of which return mechanism (`RET`, `RSB`, `REI`)
the callee actually uses).

### One shared breakpoint list, matching the C source

`vax.c`'s `set_break`/`clear_break` and its single `breakpoint_list` back
*every* kind of stop this file's own loop recognizes: user breakpoints
(`SET BREAKPOINT`), and the one-shot `BREAK_TEMPORARY|BREAK_STEP` entries
STEP_OVER/STEP_RETURN arm internally. This port matches that instead of
inventing a separate mechanism: `Breakpoint` (`execute.go`) gained
`Temporary`/`Step` bool fields, and `Console.Breakpoints` holds both kinds.
`Execute`'s own breakpoint-checking loop was factored out into `runLoop`
(`execute.go`), shared by `Execute` and by `Step`'s `StepOver`/`StepReturn`
continuations — removing a fired temporary breakpoint and choosing "Stepped
to" vs. "Break at" for its hit message both happen in that one shared place.

One deliberate fidelity improvement over the C source: `get_return`'s own
caller (`vax.c:536-541`) calls `set_break` with whatever its output
parameter holds — `0` on failure — *before* checking whether `get_return`
itself failed, leaving a spurious breakpoint at address 0 behind on a
`STEP/RETURN` issued with no call frame established. This port's
`stepReturn` checks the error first and never arms a breakpoint at all in
that case — a clear, obvious logic slip per `CLAUDE.md`'s bug-fixing policy
(not an ISA-fidelity question), fixed directly rather than logged.

### Tracing: three different rules, not one

Read literally, `vax.c`'s local `disasm` variable inside `execute_vax`
produces three distinct tracing behaviors depending on `singlestep` mode,
not one uniform rule:

- **STEP/INTO**: always traced (`console_step.c`'s own "Always in trace
  mode"), regardless of `Console.Trace` — unchanged from this port's
  pre-existing behavior (`docs/PHASE-08.md`).
- **STEP/OVER**: `vax.c:393-395` forces `disasm=2` unconditionally the
  moment `singlestep == STEP_OVER` is seen, so the *first* instruction is
  always traced too, regardless of `Console.Trace` — but if it turns out to
  be call-like, the local `disasm` is left at `0` for the rest of that
  `execute_vax` call (the "if (disasm == 2) disasm = 0" line), silencing
  every instruction inside the stepped-over routine even if `Console.Trace`
  is on. `stepOver` matches this with two different `trace` funcs passed to
  `runLoop`: `c.traceStep(pc, true)` for the first instruction, then a
  func that never traces at all for the continuation.
- **STEP/RETURN**: no special-casing in `vax.c` touches `disasm` for this
  mode at all — the whole run (from the first instruction through the
  return) obeys `Console.Trace` exactly like `EXEC`/`GO` would. `stepReturn`
  matches this by reusing `Execute`'s own tracer (`c.traceStep(pc, false)`)
  for `runLoop`.

### `skipFirstCheck` is about the command's starting PC, not the loop call

`vax.c`'s `initial_PC` guard exists so a breakpoint sitting exactly on the
address a run/step *starts* from doesn't fire immediately — it must be
reached again after at least one instruction runs. That guard applies once
per `execute_vax` call, keyed to the address execution started from when
that call began — not "the first PC any inner loop happens to see." This
matters for `STEP/OVER`'s continuation phase specifically: after the
call-like instruction has already run (via a direct `Engine.Step`, not
`runLoop`), the continuation's own first PC is the *callee's entry point* —
a new address with no special "starting point" status — so a permanent
breakpoint sitting there must still fire immediately. `runLoop` takes
`skipFirstCheck` as an explicit parameter for exactly this reason: `true`
for `Execute` and `stepReturn` (their first PC really is the command's own
starting point), `false` for `stepOver`'s continuation call. See
`TestStep_respectsBreakpointHitDuringStepOver`.

### `SET STEP`/`STEP`'s qualifier matching is approximate, deliberately

`console_set.c`'s `SET STEP` and `console_step.c`'s `STEP` each compare the
parsed verb against a literal, not entirely consistent list of 4-character
`CHAR4` codes (e.g. the bare word `OVE` alone isn't accepted — only the
full `OVER` or the slash-prefixed `/OVE` are, while `IN` alone *is* accepted
for `INTO`). `parseStepModeWord` (`step.go`) instead matches an unambiguous
prefix of `OVE`/`INT`/`INS`/`IN`/`RET`, uniformly for both commands (`STEP`
additionally restricts to a leading `/`, matching `console_step.c`'s own
narrower acceptance — see `parseStepQualifier`). This is a console
input-parsing convenience with no ISA-fidelity stakes, not a case for
`docs/DEVIATIONS.md`.

## What's implemented

- `StepMode` (`step.go`): `StepInto` (zero value, matching
  `initialization.c`'s own startup default), `StepOver`, `StepReturn`.
- `Console.StepMode`: the persistent default (`SET STEP`/`SHOW STEP_MODE`).
- `Console.Step(startAddr *uint32, mode StepMode) error`: the `STEP`
  command itself, now taking an explicit mode instead of always behaving as
  `STEP/INTO` — see `cmdStep` (`dispatch.go`) for how a bare `STEP`
  resolves `mode` from `Console.StepMode` before calling this.
- `Console.SetStepMode`/`Console.ShowStepMode`, wired to `cmdSet`'s new
  `STEP` case and the grammar's `SHOW_STEP` binding (`show_step`, DCL id
  `134`) respectively.
- `execute.go`: `Breakpoint.Temporary`/`Step`, `removeBreakpointPtr`,
  `runLoop` (factored out of `Execute`, now also used by `Step`).
- `step.go`: `stepOverInstructions`, `returnAddress` (`get_return`'s
  equivalent), `setStepBreakpoint`, `stepInto`/`stepOver`/`stepReturn`.
- Tests: `internal/console/step_test.go` (STEP/OVER over a real `CALLS`
  frame, STEP/OVER of an ordinary instruction, STEP/OVER's silence inside
  the called routine, STEP/RETURN from inside a called routine, STEP/RETURN
  with no frame established, a permanent breakpoint still firing during
  STEP/OVER's continuation, `SET STEP`/`SHOW STEP_MODE`, and an end-to-end
  `Dispatch` round trip including the default-mode-vs-explicit-qualifier
  interaction). `go build ./...`, `go vet ./...`, and `go test ./...` all
  clean.

## Not yet implemented (future sub-phases under this same document)

- **Instruction-level (opcode) breakpoints** — `SET BREAK/INSTRUCTION`,
  `SHOW BREAK/INSTRUCTION`, `CLEAR BREAK/INSTRUCTION[/ALL]`
  (`docs/PHASE-16.md` sub-phase 4): needs a per-opcode "break on this
  instruction" flag with no analogue on this port's `internal/cpu`
  instruction table today.
- **Fault-kind breakpoints** — `SET BREAK/FAULT`, the `/FAULT`/`/ADDRESSES`
  qualifiers on `SHOW BREAKPOINTS`, `CLEAR BREAKPOINT/FAULT[/ALL]`
  (`docs/PHASE-16.md` sub-phase 4): needs `BreakKind` to gain a `BreakFault`
  value and a hook into `internal/cpu/handlefault.go`'s exception dispatch.
- **Watchpoints** — `SET WATCH`, `SHOW WATCHPOINTS`, `CLEAR WATCH`
  (`docs/PHASE-16.md` sub-phase 4, `storage.c:44-140` in the C reference):
  no Go equivalent of `struct WATCHPOINT`/`add_watchpoint`/
  `delete_watchpoint`/`watchpoint_hit` exists yet; a watchpoint that
  actually breaks execution on a matching memory write also needs a hook in
  `internal/vm.Memory`'s store path, not just the bookkeeping list.

## Progress Log

### 2026-09-15 — `STEP`/`SET STEP`/`SHOW STEP_MODE` implemented

Extracted from `docs/PHASE-16.md` sub-phases 1c/3 at the user's request
(considerable cross-cutting scope: CPU-engine classification + a new
internal breakpoint mechanism, not just a console command binding) and
implemented in full — see "What's implemented" above for the shipped
surface and "Design decisions" for the fidelity choices made along the way
(no double-decode for STEP/OVER, the three different tracing rules, the
`skipFirstCheck` semantics, and the one `get_return`-ordering bug fixed
rather than replicated). `docs/PHASE-16.md` sub-phase 1c's `SHOW STEP_MODE`
entry and sub-phase 3's `SET STEP` entry now point here instead of carrying
their own stale "not implemented" text.
