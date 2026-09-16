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

**Status: STEP/SET STEP/SHOW STEP_MODE complete. Instruction-level (opcode)
breakpoints complete** (sub-phase 2, 2026-09-15 — see "Instruction-level
(opcode) breakpoints" under Design decisions and the Progress Log). This
document is also, per the user's own suggestion when the phase was created,
the intended home for two more related but not-yet-implemented mechanisms
`docs/PHASE-16.md` sub-phase 4 already flagged as missing entirely —
fault-kind breakpoints and the watchpoint subsystem (`SET WATCH`, `SHOW
WATCHPOINTS`, `CLEAR WATCH`, none of which exist yet). Neither of those two
is implemented yet; they're named here as likely next sub-phases under this
same "flow of control" umbrella rather than left to spawn a fourth or fifth
STEP-adjacent phase document later.

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

### Sub-phase 2: instruction-level (opcode) breakpoints

- `reference/eVAX/eVAX/Headers/vaxinstr.h:36-42` — `struct INSTRUCTION`'s
  `debugdata` field and its one defined bit, `OP_DBG_BREAK`.
- `reference/eVAX/eVAX/Source/CPU/decode_opcode.c:203-226,282-289` —
  `decode_instruction` itself testing `OP_DBG_BREAK` and returning
  `VAX_BREAK`: once immediately for a zero-operand instruction, or after
  operand decode otherwise.
- `reference/eVAX/eVAX/Source/CPU/vax.c:366-376` — `execute_vax`'s handling
  of a `VAX_BREAK` from `decode_instruction`: guarded by `saved_pc !=
  initial_PC` (the same run-starting-point guard address breakpoints use),
  it resets `vax.PC` to the not-yet-executed instruction's address and
  prints `" Instruction break at  "` via `format_instruction`.
- `reference/eVAX/eVAX/Source/Console/console_set.c:716-798` — `SET
  BREAK[POINT]`'s qualifier dispatch (`/FAULT`, `/TEMPORARY`, `/INSTRUCTION`,
  or plain address) and the `BREAK_INSTRUCTION` case itself: resolves the
  given mnemonic through a synthetic `"OPC$_<name>"` symbol
  (`init_symbols.c:227-238` registers these for the first 256 single-byte
  opcodes only) and sets `OP_DBG_BREAK` directly on `instruction[n]` —
  never added to `vax.console.breakpoint_list` at all, unlike every other
  breakpoint kind.
- `reference/eVAX/eVAX/Source/Console/console_show.c:162-181` —
  `SHOW BREAK/INSTR` (`show_break_instr`, DCL id `412`): scans `instruction[]`
  for `OP_DBG_BREAK`, printing each and a count summary.
- `reference/eVAX/eVAX/Source/Console/console_clear.c:65-111` — `CLEAR
  BREAK/INSTR/ALL` (id `553`) and `CLEAR BREAK/INSTR <opcode>` (id `551`);
  see "A wrong index and a missing `break`" below for two bugs found here.
- `testdata/dcl/evax.dcl` / `internal/bootdata/files/evax.dcl` — the DCL
  grammar already carried `clear_break_instr`/`clear_break_instr_all`
  (ids `551`/`553`) and `show_break_instr` (id `412`) unbound (per
  `docs/PHASE-16.md`'s audit); `SET` itself is never DCL-grammared in the C
  source (no `verb set` in `evax.dcl`) or in this port (`cmdSet`'s own small
  hand-rolled parser, `dispatch.go`), so `SET BREAK/INSTRUCTION` needed a
  parser change instead of a grammar binding.
- `internal/cpu/decode.go` (new `fetchOpcode` helper, shared by
  `decodeInstruction` and the new `Engine.PeekInstruction`), `engine.go`
  (`Engine.PeekInstruction`), `internal/console/instbreak.go` (new:
  `Console.InstructionBreakpoints` and its `Add`/`Remove`/`ClearAll`/`Show`/
  `instructionBreakpointHit` methods), `machine.go`
  (`Console.InstructionBreakpoints`), `execute.go` (`runLoop`'s new check,
  `BreakKind`'s doc comment), `misc.go` (`ClearBreakpoint`'s doc comment),
  `dispatch.go` (`cmdSet`'s qualifier parsing, three new grammar bindings),
  `internal/vmserrors/codes_cli.go` (`CLI_NEEDBREAKOPCODE`).

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

### Instruction-level (opcode) breakpoints

Sub-phase 2: `SET BREAK[POINT]/INSTRUCTION <mnemonic>`, `SHOW
BREAKPOINTS/INSTRUCTIONS`, and `CLEAR BREAKPOINT/INSTRUCTION [<mnemonic>|
/ALL]` — the instruction-breakpoint mechanism `docs/PHASE-16.md` sub-phase 4
catalogued as entirely missing and this document's own "Not yet
implemented" section (until now) named as a likely next sub-phase. Flags a
given opcode so execution stops just before *any* instance of it runs,
anywhere — not an address breakpoint on one specific occurrence.

**A wholly separate mechanism from `Console.Breakpoints`, matching the C
source.** Every other breakpoint kind this port supports (user address
breakpoints, and the internal one-shot temporary/step breakpoints
STEP/OVER and STEP/RETURN arm — see "One shared breakpoint list" above)
lives in `vax.console.breakpoint_list` in the C source, and correspondingly
in `Console.Breakpoints` here. Instruction breakpoints never do: `SET
BREAK/INSTRUCTION` sets `OP_DBG_BREAK` directly on the matched
`instruction[]` slot and `goto exit`s past `set_break`'s breakpoint_list
insertion entirely (`console_set.c:782-798`). This port follows suit with
`Console.InstructionBreakpoints map[*cpu.Instruction]bool`
(`instbreak.go`) — a completely separate collection from `Breakpoints`,
keyed by `*cpu.Instruction` pointer identity (every occurrence of that
opcode anywhere in memory qualifies, not one address) — rather than
stretching `BreakKind`/`Breakpoint` to cover a mechanism the C source itself
never folds in there either.

**Peeking the opcode instead of replicating the C source's decode-then-
maybe-discard shape.** `decode_opcode.c` finds out whether the *current*
instruction is flagged only *after* fully decoding it — operand decode
(with any autoincrement/autodecrement side effects) included — then
`execute_vax` discards the decode and never calls the instruction's
handler if `OP_DBG_BREAK` was set. Reproducing that shape in this port
would mean giving `Engine.Step` some way to decode without committing to
execution, a real split to its decode-execute-atomically design
(`docs/PHASE-03.md`) for a debugger-only feature. Instead, `runLoop`
(`execute.go`) checks `instructionBreakpointHit` — backed by
`Engine.PeekInstruction` (`engine.go`), a new side-effect-free lookup that
reads only the opcode byte(s) at the current PC (via a `fetchOpcode` helper
factored out of `decodeInstruction`, `decode.go`) and resolves them through
`Table.Lookup`, doing no operand decode at all — *before* calling
`Engine.Step`. A flagged instruction is therefore identified, and stopped
on, without ever touching its operands, matching the net user-visible
outcome (the instruction doesn't run) without the C source's decode-twice
mechanics. Checked in `runLoop` right alongside the existing address-
breakpoint check, under the same `skipFirstCheck`-gated `if !first` block —
see the next paragraph for why that's a deliberate improvement over the C
source's own guard, not just a convenient reuse.

**`skipFirstCheck` uniformly, not the C source's incidental cross-wiring.**
`decode_opcode.c`'s `OP_DBG_BREAK` check and `execute_vax`'s own address-
breakpoint check are guarded by the same `initial_PC` variable, but from two
different code paths with different preconditions: the address-breakpoint
block only resets `initial_PC` to `0xFFFFFFFF` (permanently disarming the
"is this the run's starting PC" guard for the rest of the run) when it
runs at all, which only happens `if (vax.console.breakpoint_list)` — i.e.,
only if at least one *address or fault* breakpoint currently exists,
completely unrelated to instruction breakpoints. So in the C source,
whether an instruction breakpoint sitting at a run's very first instruction
fires immediately depends on whether some unrelated address breakpoint
happens to exist elsewhere — a clear, obvious incidental bug (two checks
sharing one variable across mismatched guard conditions), not an
ISA-fidelity question, so not replicated (and not logged to
`docs/DEVIATIONS.md`) per `CLAUDE.md`'s bug-fixing policy. This port's
`runLoop` instead applies its single `skipFirstCheck`/`first` flag
uniformly to both breakpoint kinds: neither ever fires on the exact
instruction a run starts from, full stop, regardless of what other
breakpoints exist. `stepInto`/`stepOver`'s first instruction (executed via
a direct `Engine.Step` call, bypassing `runLoop` entirely — see "One shared
breakpoint list" above) needed no matching change: the only instruction
either ever runs outside `runLoop` is exactly the run's starting one, which
`skipFirstCheck` would have suppressed anyway.

**Resolving mnemonics via `Table.ByName`, not a synthetic `OPC$_` symbol.**
`console_set.c`/`console_clear.c` resolve a `SET`/`CLEAR
BREAK/INSTRUCTION`'s opcode argument by building a fake `"OPC$_<name>"`
string and looking it up in the symbol table, where `init_symbols.c:227-238`
registered one `OPC$_<name>` symbol per opcode — but only for the first 256
single-byte opcodes (`for (n = 0; n < 256; n++)`), never the two-byte
extended ones. This port instead resolves mnemonics directly via
`cpu.Table.ByName` (`lookupInstruction`, `instbreak.go`), which already
indexes every opcode this port defines, extended included. This is a
console-command-scope convenience with no ISA-fidelity stakes (which
mnemonics a debugger command accepts, not how any instruction executes),
so extending coverage to extended opcodes is a deliberate improvement
rather than something to flag in `docs/DEVIATIONS.md` — see
`lookupInstruction`'s own doc comment.

**Two more C-source bugs found and not replicated, per `CLAUDE.md`'s
bug-fixing policy.** `console_clear.c`'s `CLEAR BREAK/INSTR <opcode>` case
(`case 551`, lines 89-111): (1) after finding the matching `instruction[]`
slot at index `count` (by comparing `instruction[count].opcode` against the
looked-up value `n`), it clears the flag at `instruction[n]` instead of
`instruction[count]` — `n` still holds the *raw opcode value*, not the
table index the search just found, so for anything other than a low
single-byte opcode this clears the wrong slot (or an out-of-bounds one);
and (2) the `case` has no closing `break`, so it falls through into `case
105` (`CLEAR STRINGS`) immediately afterward on every invocation. Both are
clear, obvious logic slips (a copy-paste index mix-up, a missing `break`),
not ISA-fidelity questions, so this port's `RemoveInstructionBreakpoint`
just does the plainly-intended thing once: resolve the mnemonic, clear the
flag on the `*cpu.Instruction` actually found, print one confirmation, and
return — see its own doc comment.

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

**Sub-phase 2 — instruction-level (opcode) breakpoints:**

- `internal/cpu/decode.go`: `fetchOpcode` (opcode-byte-only fetch, factored
  out of `decodeInstruction`, which now calls it too — no behavior change).
- `internal/cpu/engine.go`: `Engine.PeekInstruction` — identifies the next
  instruction at the current PC with no operand decode, no PC advance, no
  side effects.
- `internal/console/instbreak.go` (new): `Console.InstructionBreakpoints`;
  `AddInstructionBreakpoint`/`RemoveInstructionBreakpoint`/
  `ClearAllInstructionBreakpoints`/`ShowInstructionBreakpoints`
  (`SET`/`CLEAR`/`SHOW`); `instructionBreakpointHit` (the `runLoop` hook);
  `lookupInstruction`, `opcodeString`, `instructionBreakpointsInOrder`,
  `pluralS` helpers.
- `internal/console/machine.go`: `Console.InstructionBreakpoints` field.
- `internal/console/execute.go`: `runLoop` now checks
  `instructionBreakpointHit` alongside the existing address-breakpoint
  check, under the same `skipFirstCheck`-gated guard (updated doc comments
  on `runLoop` and `BreakKind`).
- `internal/console/dispatch.go`: `cmdSet`'s `BREAKPOINT`/`BREAK` case now
  splits a `/qualifier` off the verb and recognizes `/INS…` (`/INSTRUCTION`,
  matching `parseStepModeWord`'s own unambiguous-prefix convention) for
  `SET BREAK/INSTRUCTION <mnemonic>`, reporting `CLI_BADQUALIFIER` for
  `/FAULT`/`/TEMPORARY` (recognized but not implemented) instead of falling
  through to `CLI_BADSETSYNTAX`; three new grammar bindings —
  `CLEAR_BREAK_INSTR`, `CLEAR_BREAK_INSTR_ALL`, `SHOW_BREAK_INSTR` — for DCL
  syntax entries `testdata/dcl/evax.dcl`/`internal/bootdata/files/evax.dcl`
  already carried unbound.
- `internal/console/misc.go`: `ClearBreakpoint`'s doc comment updated (its
  `/INSTRUCTION` sub-form is now implemented, but as an entirely separate
  command path, not routed through this function).
- `internal/vmserrors/codes_cli.go`: `CLI_NEEDBREAKOPCODE` ("SET
  BREAK/INSTRUCTION requires an opcode mnemonic"); `CLI_BADOPCODE` (already
  existed) reused for an unrecognized mnemonic passed to `SET`/`CLEAR
  BREAK/INSTRUCTION`.
- Tests: `internal/console/instbreak_test.go` — `Add`/`Remove`/`ClearAll`
  round trip (including the duplicate-add and remove-when-unset no-ops),
  an unrecognized-mnemonic error from both `Add` and `Remove`,
  `ShowInstructionBreakpoints`'s empty and non-empty output,
  `Execute` stopping right before a flagged `HALT` runs (confirming the
  instruction never actually executes, not just that execution stopped),
  the `skipFirstCheck` interaction (a run starting exactly on a flagged
  opcode must not stop immediately, but the same opcode occurring again
  later still must), and an end-to-end `Dispatch` round trip
  (`SET BREAK/INSTRUCTION`, `SHOW BREAKPOINTS/INSTRUCTIONS`, `EXEC`
  actually stopping, `CLEAR BREAKPOINT/INSTRUCTION`, `CLEAR
  BREAKPOINT/INSTRUCTION/ALL`), plus the two new `cmdSet` error paths
  (missing opcode, unimplemented `/FAULT` qualifier). `go build ./...`,
  `go vet ./...`, and `go test ./...` all clean.

## Not yet implemented (future sub-phases under this same document)

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

### 2026-09-15 — Instruction-level (opcode) breakpoints implemented (sub-phase 2)

Picked up as the next sub-phase this document already named as likely
follow-on work, at the user's request. Implemented `SET
BREAK[POINT]/INSTRUCTION <mnemonic>`, `SHOW BREAKPOINTS/INSTRUCTIONS`, and
`CLEAR BREAKPOINT/INSTRUCTION [<mnemonic>|/ALL]` in full — see "What's
implemented" above for the shipped surface and "Design decisions" for the
fidelity choices made along the way: a wholly separate mechanism from
`Console.Breakpoints` (matching the C source's own `instruction[].debugdata`
storage, never part of `breakpoint_list`), a side-effect-free
`Engine.PeekInstruction` peek instead of replicating the C source's decode-
then-maybe-discard shape, applying `runLoop`'s existing `skipFirstCheck`
flag uniformly rather than replicating an incidental C-source bug where an
instruction breakpoint's own initial-PC guard is accidentally gated on
whether unrelated address/fault breakpoints happen to exist, resolving
mnemonics via `cpu.Table.ByName` rather than the C source's `OPC$_`
symbol-table hack (extending coverage to two-byte extended opcodes as a
free side effect), and two C-source bugs in `CLEAR BREAK/INSTR <opcode>`
(a wrong-variable index, a missing `break` causing fallthrough into `CLEAR
STRINGS`) fixed rather than replicated, per `CLAUDE.md`'s bug-fixing
policy. `docs/PHASE-16.md` sub-phase 4's instruction-breakpoint entries
should be treated as superseded by this document going forward, matching
how sub-phases 1c/3 already point here for `STEP`/`SET STEP`.

Fault-kind breakpoints and the watchpoint subsystem remain the two
not-yet-implemented mechanisms this document is the intended home for; see
"Not yet implemented" above.
