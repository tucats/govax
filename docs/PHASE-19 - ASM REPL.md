# Phase 19: Interactive `ASM` REPL mode

## Goal

`docs/PHASE-11.md`'s own closeout explicitly left one thing unported: the reference
tool's bare `ASM` (no filename) command, which puts the console into a line-at-a-time
interactive assembler mode instead of assembling a whole file in one batch. This port's
`internal/console/dispatch.go` currently rejects it outright
(`vmserrors.CLI_NOASMREPL`, "Interactive ASM mode ... is not implemented; use ASM
<filename>"). This phase implements it.

## Scope / C source mapping

- `reference/eVAX/eVAX/Source/Console/console_include.c`'s `console_asm` — a bare `ASM`
  with no trailing text sets `vax.console.assembler_mode = 1` and returns; nothing else
  happens until the next line is read.
- `reference/eVAX/eVAX/Source/Console/console_dispatch.c`'s `console_dispatch` — while
  `vax.console.assembler_mode` is set, *every* subsequent line (from the interactive
  prompt, or from an include file/script) is hand off directly to `assemble()`, bypassing
  the normal verb table/DCL grammar entirely.
- `reference/eVAX/eVAX/Source/Assembler/asm.c`'s `assemble()` — the per-statement engine:
  a blank line or a leading `#` (GNU-style) comment is a no-op; a bare `END` (no dot, only
  reachable this way — not through `asm_pseudo`) hex-parses an optional entry address,
  sets `vax.PC` directly, warns if `ASM_WARNFORWARD` is set and unresolved forward
  references remain, and turns assembler mode back off; otherwise the line is a normal
  label/pseudo-op/opcode statement, deposited at `vax.console.deposit` (the same "current
  address" register `EXAMINE`/`DEPOSIT` share) and immediately visible to later
  statements via the live symbol table.
- `reference/eVAX/eVAX/Source/Assembler/asm_pseudo.c` case 11 (`.END`, dotted) — reachable
  both from a batch file and from interactive mode (since both funnel through the same
  per-statement `assemble()`/`asm_pseudo()`); sets the one-shot `ASM_ENTRY` flag instead of
  `vax.PC` directly, letting `console.c`'s post-command hook issue `CALL __ENTRY`.
- `reference/eVAX/eVAX/Source/Console/driver.c` — the interactive prompt switches from
  `"VAX> "` to `"ASM> "` (or, if `ASM_ADDRPROMPT` were set — it isn't, by default —
  `"[%08X]  ASM> "`) while `assembler_mode` is on.

## Design decisions

- **Reuse, don't reimplement, the statement engine.** `internal/asm.Assembler`'s existing
  `assembleLines`/`assembleStatement` (Phase 11) already assembles one preprocessed line
  at a time, depositing at `a.deposit` and advancing it — that's precisely the semantics
  interactive mode needs, just fed one line per console command instead of pre-split from
  a whole file. Two small additions make this reachable from outside the package:
  `Assembler.AssembleLine(line string) (done bool, err error)` (preprocess + assemble one
  statement, reporting whether `.END`/`END` just stopped assembly) and
  `Assembler.BeginInteractive()` (clears the `stop` flag a *previous* interactive session's
  own `END` may have left set, exactly mirroring `Assemble`'s own reset-on-entry for the
  same reason — a persistent session is reused across multiple `ASM` invocations,
  batch or interactive, in a row).
- **`vax.console.deposit` is one register, not two.** The reference tool's interactive
  assembler and `EXAMINE`/`DEPOSIT` literally share one field. This port already has the
  same concept (`Console.DepositAddr`, `internal/console/exam.go`) but Phase 11's batch
  `Assemble` never consulted it — a freshly created `asmSession` always started at the
  assembler package's own hardcoded default (0x200), coincidentally equal to
  `DepositAddr`'s own initial value, so the gap never showed. Fixed as part of this phase
  (a real fidelity gap, not REPL-specific, so it improves the batch form too):
  `ensureAsmSession` now seeds a freshly created session with `SetOrigin(c.DepositAddr)`,
  and every successful statement (batch or interactive) syncs `c.DepositAddr` back from
  the assembler's own current deposit pointer (`Assembler.Deposit()`, new). A `DEP 4000`
  then bare `ASM` now correctly starts assembling at `0x4000`, matching the reference
  tool; existing batch tests are unaffected since they never move `DepositAddr` off its
  post-`VMINIT` default (0x200) before assembling.
- **Errors don't exit the mode.** `assemble()` returns a statement's error without ever
  touching `assembler_mode` itself — a typo stays correctable, it doesn't kick you back
  to the `VAX>` prompt. `Console.AssembleInteractiveLine` matches this: an error is
  returned for display, but `assemblerMode` stays true and the caller keeps routing
  lines to it.
- **`END`'s auto-`CALL __ENTRY` is unified, not duplicated.** This port's own
  `pseudoEnd` (Phase 11) already treats a bare or dotted `END` identically (the leading
  `.` is optional in `assemblePseudo`'s own name match) and always uses the general
  expression evaluator rather than the reference tool's hex-only bare-`END` parse — a
  deliberate simplification already in place before this phase, not something worth
  un-unifying now. `cmdAssemble` (batch) already knows to issue `CALL __ENTRY` when
  `TakeEntry()` reports one; the interactive path reuses the exact same "did `.END` name
  an address" plumbing so both forms behave identically once assembly stops.
- **Forward-reference warning.** `ASM_WARNFORWARD` (on by default,
  `initialization.c`) makes the reference tool's bare `END` print "There are unresolved
  forward references" if any symbol still has pending fixups. `internal/asm.symbol.go`
  already tracks this (`hasUnresolvedSymbols`) but nothing ever surfaced it. Exported as
  `Assembler.HasUnresolvedSymbols()` and checked when interactive mode's `END` fires,
  printed the same way `.PRINT` output already reaches the console. `SET`/`SHOW` for the
  `ASM_WARNFORWARD` toggle itself isn't implemented (no fixture/user request exercises
  it) — noted here as a deliberate, narrow scope cut, same spirit as Phase 11's own list.
- **`.CONSOLE` stays scope-cut.** The reference tool's `.CONSOLE <cmd>` pseudo-op
  temporarily drops out of assembler mode, dispatches `<cmd>` through the *live* console
  command dispatcher, then restores assembler mode — meaningful for both batch and
  interactive assembly alike, not something specific to this phase's own REPL work.
  Phase 11 already scope-cut it to two hardcoded recognized forms (`SET RADIX`, `SET
  VERIFY`) specifically because the assembler package has no (and shouldn't gain one)
  dependency on `internal/console`. Left as-is; revisiting it would need a callback hook
  threaded through `Assembler`, which is more machinery than any known fixture or use
  case justifies today.
- **Prompt.** `cmd/govax/main.go`'s `readline.Config.Prompt` is fixed at construction
  time in the reference port; switched to a per-iteration `rl.SetPrompt` call based on
  `Console`'s new `InAssemblerMode()` query, matching `driver.c`'s own
  `"ASM> "`/`"VAX> "` switch (the `ASM_ADDRPROMPT` address-in-prompt variant is the
  scope cut noted above).

## Deliverables

- `internal/asm`: `Assembler.AssembleLine`, `Assembler.BeginInteractive`,
  `Assembler.Deposit`, `Assembler.HasUnresolvedSymbols`.
- `internal/console`: `Console.assemblerMode` state (reset by `VMInit`/`Zero`, matching
  `asmSession`'s own existing reset); `ensureAsmSession` helper factored out of
  `Assemble` (shared with the new interactive entry point, now also seeding
  `SetOrigin(c.DepositAddr)`); `Console.AssembleBegin`; `Console.AssembleInteractiveLine`;
  `Console.InAssemblerMode`. `Dispatcher.Dispatch` checks `assemblerMode` first, exactly
  where `console_dispatch.c` does, ahead of verb-table lookup. `cmdAssemble` now enters
  interactive mode instead of returning `CLI_NOASMREPL` (removed — no longer reachable,
  and it was the only reference to the old "not implemented" message).
- `cmd/govax/main.go`: dynamic `ASM> `/`VAX> ` prompt.
- Tests: `internal/asm` unit tests for the new incremental API; `internal/console`
  end-to-end tests driving `Dispatcher.Dispatch` through a bare `ASM` / statements / `END`
  sequence (mirroring `TestAssemble_xorDepositsAndIsCallable`'s style, but line-by-line),
  covering: labels/opcodes depositing and becoming callable, an error line not exiting
  the mode, `DepositAddr`/`asmSession` interaction with a preceding `DEP`, and a
  persistent session shared between a batch `ASM <file>` and a following interactive
  block (and vice versa).

## Open questions / notes

- None outstanding; `.CONSOLE`'s live-dispatch form and the `ASM_ADDRPROMPT`/
  `ASM_WARNFORWARD` `SET`/`SHOW` toggles are recorded above as deliberate scope cuts,
  not open questions.

## Progress Log

### 2026-09-16 — Interactive ASM REPL mode complete

- `internal/asm`: `Assembler.AssembleLine`/`BeginInteractive`/`Deposit`/
  `HasUnresolvedSymbols`, all thin wrappers around Phase 11's existing
  per-statement engine (`assembleStatement`, `a.stop`, `a.deposit`,
  `hasUnresolvedSymbols`) — no new assembly semantics, just new exported
  seams into machinery that already worked one line at a time internally.
- `internal/console`: `ensureAsmSession`/`depositAsmImage`/`mergeAsmSymbols`
  factored out of `Assemble` (asm.go) and shared with the new
  `AssembleBegin`/`AssembleInteractiveLine`; `ensureAsmSession` now also
  seeds a freshly created session from `Console.DepositAddr` rather than
  the assembler package's own hardcoded default (see this doc's design
  notes above — a real fidelity gap in the existing batch form, fixed here
  since both forms share the helper). `Dispatcher.Dispatch` checks
  `Console.assemblerMode` first, matching `console_dispatch.c`; `cmdAssemble`
  now calls `AssembleBegin` instead of returning `CLI_NOASMREPL` for a bare
  `ASM` (the error code/message removed — no longer reachable, and it was
  their only reference). `assemblerMode`/`asmSession` are both reset by
  `VMInit` (already did the latter) and now also by `Zero` (didn't reset
  either before — a small pre-existing gap, harmless today since nothing
  else read `asmSession` post-`Zero` without an intervening `VMInit`, but
  fixed alongside this work for the same reason `assemblerMode` needs it).
- `cmd/govax/main.go`: the readline prompt switches to `"ASM> "` while
  `Console.InAssemblerMode()`, matching `driver.c`'s own prompt swap (the
  `ASM_ADDRPROMPT` address-in-prompt variant stays a deliberate scope cut,
  off by default upstream too).
- **Found via this phase's own new `HasUnresolvedSymbols` check** (nothing
  ever called `hasUnresolvedSymbols` before this phase — it existed but was
  dead code): `testdata/asm/kernel.asm` (identical to
  `reference/eVAX/kernel.asm`) has a genuine typo bug at line 276-281 —
  `beql _no_inc` branches to a label spelled `_no_inc`, but the label two
  lines later reads `_noinc` (no underscore before "inc"), so the branch
  target is never defined and stays a permanently unresolved forward
  reference. Every real `govax` boot now prints `%There are unresolved
  forward references` the first time interactive `ASM` mode's `END` fires
  (confirmed in `TestRun_interactiveAsmRepl`, `cmd/govax/main_test.go`),
  purely because this is the first code that ever asked the question — the
  underlying condition predates this phase and is unrelated to REPL mode
  itself. Left unfixed pending the user's own call: this is a bug in an
  upstream-mirrored fixture/boot-script file, not a C-to-Go port fidelity
  question `CLAUDE.md`'s bug-fixing policy is written for, and
  `reference/eVAX/kernel.asm` is explicitly read-only reference material
  (`testdata/asm/kernel.asm` is a separate, editable copy, but fixing it
  unilaterally would silently diverge this port's own boot script from the
  upstream file it's supposed to mirror). Functionally low-impact — the
  branch is only taken when `MFPR TODR` reads exactly zero, and would jump
  to whatever the unresolved fixup resolves to (address 0) rather than
  falling through to `_noinc`'s own `MTPR #0FF, #VAX$PR_ICCS`/`REI` — but
  worth the user's attention since it's a real, previously invisible defect
  in the shipped boot path.
- Tests: `internal/asm/interactive_test.go` (six new unit tests: line-by-
  line assembly matches whole-file `Assemble` byte-for-byte, bare `END`
  stops assembly and records an entry address, a statement error doesn't
  set `done`, `BeginInteractive` resets a previous session's stop flag,
  `Deposit()` tracks the location counter, `HasUnresolvedSymbols` on a real
  forward vs. resolved backward reference); `internal/console/asm_repl_test.go`
  (four new end-to-end `Dispatcher.Dispatch` tests: entering/exiting mode
  and rejecting a would-be command as a bad statement without exiting,
  depositing+auto-`CALL __ENTRY` on `END <name>`, `DepositAddr` sync in both
  directions, a persistent session shared between a batch file and a
  following interactive block); `cmd/govax/main_test.go`'s
  `TestRun_interactiveAsmRepl` (full `readline`-loop end-to-end: bare `ASM`,
  a few typed statements, `END <entry>` auto-invoking the routine, then a
  plain `EXAMINE` back in ordinary command mode confirming the register
  effect). One pre-existing test, `TestDispatch_notImplementedFixedCommand`
  (`dispatch_test.go`), asserted a bare `ASM` still errored under the old
  `cmdNotImplemented`-adjacent behavior; repointed at `BOOT`, still
  genuinely unimplemented, with a comment noting where `ASM`'s own coverage
  moved to.
- `go build ./...`, `go vet ./...`, `go test ./...` all clean. `gofmt -l`
  on every file this phase touched: clean (a handful of unrelated,
  pre-existing files elsewhere in the tree — present before this phase
  started, confirmed via a clean `git status` at session start — still
  show drift from this toolchain's `gofmt`; left alone as out of scope).
