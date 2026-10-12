# Phase 51 — DCL follow-ups

**Status:** in progress (planned and approved 2026-10-12). Needs Phase
50. Subtask 1, round 6 of the VMS probe, is done (2026-10-12): the
author ran it, and govax now follows VMS's answers. Subtask 2, SPAWN in
a subprocess, is done (2026-10-12), and so is subtask 3, Ctrl/Y in a
subprocess's procedures (2026-10-12).

Phase 50 made the console a DCL command interpreter and closed on
2026-10-12 with every subtask done. Following Phase 49's example, the
author chose (2026-10-12) to collect what it left for later here instead
of keeping it open: three features it deferred on purpose, the probe
questions its fifth round left open, and the known differences from VMS
that its probes found but no subtask fixed.

## Goal

- **A subprocess's DCL can SPAWN.** At present SPAWN is
  `%VAX-E-NOTSUBPROC` in a subprocess (`processOneCommands`), because
  the subprocess's interpreter couldn't wait for a subprocess of its
  own. Subtask 21's yields (`cliYield`, `corevms.Yield`, `WaitUntil`)
  now give it a way to wait in the scheduler.
- **Ctrl/Y in a subprocess's procedures.** On VMS, SPAWN (without
  /NOWAIT) attaches the terminal to the subprocess, so Ctrl/Y goes to
  the subprocess's DCL, with the same rules a procedure in process 1
  follows (ON CONTROL_Y, the Ctrl/Y level, CONTINUE). In govax, a
  subprocess's procedures never get Ctrl/Y (DEVIATIONS.md, "[Phase 50]
  The subprocess's DCL").
- **The other processes run while the prompt waits for a line.** On VMS
  the processes keep running while DCL waits for a terminal line. In
  govax, process 1 lends the CPU at command level 0 only until the
  others are idle or `promptLendLimit` (50 ms) has passed
  (`lendWhileIdle`). After that, nothing runs until a line is typed. To
  do better, cmd/govax must read the terminal while the machine runs:
  its line reader on a goroutine of its own, with the machine running
  the other processes until a line (or a control key) arrives.
- **What the probes left open**, settled by round 6 (below), and govax
  changed to match VMS's answers.
- **Known differences from VMS** that the probes found and no subtask
  fixed (below, "Known differences").

## What earlier phases leave in place

- `internal/console/subcli.go` (Phase 50, subtask 19): each
  subprocess's DCL runs on a goroutine that takes turns with the engine
  through its CLI stub's shim (`EXE$CLI_COMMAND`), with the console's
  per-process DCL state swapped in (`swapDCLState`). A `cliYield` waits,
  yields after a command, calls an image, or logs out.
- `internal/console/dcllend.go` and `internal/corevms/lend.go` (Phase
  50, subtask 21): process 1 lends the CPU after each DCL command
  (`commandBoundary`, `lendCPU`), and at command level 0 until the
  others are idle (`lendWhileIdle`). A subprocess's DCL yields after
  each command (`yieldCommand`), and its WAIT waits in the scheduler
  (`yieldTimer`).
- `internal/console/spawncmd.go` (Phase 48): the console's SPAWN calls
  LIB$SPAWN in process 1 from a small procedure page (`spawnPage`). With
  no /NOWAIT, process 1 waits in LIB$SPAWN while the machine runs.
- `internal/console/dclcontroly.go` (Phase 50, subtask 18): process 1's
  Ctrl/Y and Ctrl/C in procedures (`HandleAttention`, `takeControlY`,
  `controlYLevel`). cmd/govax's key handling (`attention.go`,
  `terminal_unix.go`) asks `ControlYEnabled`.
- `cmd/govax/main.go`: the prompt loop reads a line with the readline
  package, then dispatches it. Nothing runs while it reads.

## Subtasks

1. **Round 6 of the probe** (`testdata/dcl50`'s README, "Round 6" and
   "What VMS answered in round 6"; done 2026-10-12). Part 1, `@PROBE50A/OUTPUT=
   PROBE50A.LOG` (with `probe50b.com` and `probe50c.com`): WAIT's
   `$STATUS`; ELSE before a block's THEN; CALL of a plain label; GOSUB's
   and block IF's nesting limits; `/OUTPUT`'s default type, `/O=`, and
   blanks around `=`; OPEN/WRITE's fixed control bytes (DUMP/BLOCK);
   SHOW LOGICAL's indentation (three blanks on VMS, two in govax) and
   its numbered levels; delta times' hundredths; `1ABC`, `.X`, `(X)`.
   Part 2, at the terminal: RETURN, `RETURN 3`, SUBROUTINE,
   ENDSUBROUTINE, ELSE, and WAIT at command level 0. Then a
   `TestProbe50AOracle` that replays `vax/probe50a.log`, and govax
   changed where VMS differs.
2. **SPAWN in a subprocess** (done 2026-10-12). Take SPAWN out of `processOneCommands`. A
   subprocess's SPAWN calls LIB$SPAWN in its own process, as process 1's
   does. Without /NOWAIT, its interpreter waits (a `cliYield` wait) until
   the child's completion. The subprocess's own subprocesses go when it
   does (Phase 45's owner deletion). Also SPAWN's status when the
   failure is govax's own (DEVIATIONS.md: SPAWN doesn't repeat a status
   with no message in the message file).
3. **Ctrl/Y in a subprocess's procedures** (done 2026-10-12). The terminal's attention
   keys go to the process the terminal is attached to: process 1, or the
   subprocess a SPAWN is waiting for (and so on down a chain of SPAWNs).
   That process's DCL takes them by Phase 50's rules. A subprocess's WAIT
   is interrupted, as process 1's is. Optionally ATTACH, to move the
   terminal between processes of a job.
4. **The other processes while the prompt waits.** cmd/govax reads the
   terminal on a goroutine, and the console runs the machine (process 1
   in LEF, as `lendWhileIdle` does) until a line arrives or a key stops
   it, with no `promptLendLimit`. A subprocess's output during the wait
   must not garble the line being typed. With no other process to run,
   nothing changes. A one-shot command and input from a pipe stay
   synchronous.
5. **Known differences** (below), each fixed or confirmed as govax's
   choice.
6. **Docs and help**: HELP SPAWN, WAIT, and KEYS; CLAUDE.md; PLAN.md.

## Known differences

What Phase 50's probes showed VMS doing that govax still doesn't, and
smaller gaps the phase left:

- **@ and TYPE of a missing file.** VMS 7.3 (round 1, section D): `@NOSUCH`
  is `%DCL-E-OPENIN, error opening DUA1:[000000]NOSUCH.COM; as input`
  then `-RMS-E-FNF, file not found`, `$STATUS` %X10018292, an error that
  ends a calling procedure. `TYPE NOSUCH.TXT` is `%TYPE-W-SEARCHFAIL,
  error searching for DUA1:[000000]NOSUCH.TXT;` then `-RMS-E-FNF`,
  `$STATUS` %X10951238. govax gives `%SYSTEM-W-NOSUCHFILE` for both.
- **`/OUTPUT=.LOG` with no name**: VMS made a file named `.LOG`; govax
  refuses it.
- **Lexical functions not yet done**: F$CONTEXT, F$CSID, F$DEVICE,
  F$FILE_ATTRIBUTES, F$GETQUI, F$IDENTIFIER, F$LICENSE (CLI$_LEXNOTIMPL).
  F$FILE_ATTRIBUTES, F$DEVICE, and F$IDENTIFIER have something to report
  in govax; F$GETQUI and F$LICENSE have nothing behind them.
- **STOP/IMAGE and STOP/[NO]EXIT** are not done.
- **DIRECTORY's, DELETE's, and DUMP's layouts** differ from VMS's in the
  probe logs (their oracle tests skip them); govax has no DUMP.

## Open questions

- Subtask 4: how output from another process should appear while a line
  is being typed (VMS redraws the line after a broadcast; readline can
  refresh it).
- Subtask 3: whether `ATTACH` is worth having without a way to detach
  (VMS's Ctrl/Y then ATTACH, or SPAWN back). Left out (2026-10-12):
  attaching to a parent would have to end its LIB$SPAWN's wait without
  the subprocess's end, and nothing yet asks for it.

## Progress log

- 2026-10-12: Planned, as Phase 50 closed. Round 6 of the probe
  (`testdata/dcl50/probe50a.com`, `probe50b.com`, `probe50c.com`; the
  README's "Round 6") is written and run under govax first. The README
  records govax's answers. The author is to run it on the simh VAX.
- 2026-10-12: The author reviewed and approved the plan.
- 2026-10-12: Subtask 1, round 6 of the probe. The author ran it
  (`testdata/dcl50/vax/probe50a.log`, and `probe50a-term.log` for the
  lines typed at the terminal), and govax now does what VMS 7.3 did:
  `TestProbe50AOracle` (new file `internal/console/probe50a_test.go`)
  matches every section, apart from DELETE's, DIRECTORY's, and DUMP's
  layouts, SHOW LOGICAL/PROCESS's list of the VAX's own names, and the
  job table's name. `TestProbe50ATerminal` matches every line of the
  terminal part. Changes (DEVIATIONS.md's "[Phase 51] Round 6 of the
  probe"):
  - An ELSE before a block's THEN is passed over, and the THEN after it
    is INVIFNEST and closes the block. A false IF's lines before its
    THEN aren't shown with verification.
  - A 16th block IF is accepted, and its THEN is refused.
  - GOSUBs nest 15 deep.
  - CALL of a plain label runs from the label at a new level, then
    fails with INVCALL (`callPlainLabel`).
  - RETURN at the terminal is silent and marks `$STATUS` as shown, or
    sets it to RETURN's status.
  - SHOW LOGICAL indents a name it looks up three blanks, numbers an
    iterated level `1  `, puts a tab before a search list's further
    values, and leaves no blank line after an empty table's header.
    Round 4's oracle no longer allows for the indent.
  - F$CVTIME writes a delta's fields as they were typed
    (`deltaAsTyped`).
  - A digit starts a verb (IVVERB, not NOCOMD).
  - WAIT's IVDTIME segment is `0  ` and the time.

  Confirmed: WAIT's status, `/OUTPUT`'s forms, and OPEN/WRITE's 01 8D.
  Unexplained: VMS dropped the blank in one verified comment, and the
  oracle allows for it. HELP: IF, GOSUB/RETURN, CALL.
- 2026-10-12: The author confirmed WAIT's segment on VMS: `WAIT BOB` is
  IVDTIME with ` \0  BOB\`, a known quirk of WAIT's message.
- 2026-10-12: Subtask 2, SPAWN in a subprocess. SPAWN is no longer in
  `processOneCommands`. In a subprocess, the console's `Spawn` calls
  `spawnInSubprocess`, which makes the new process with corevms's
  `Environment.Spawn` (LIB$SPAWN's own Go) in the subprocess. Unless
  /NOWAIT, the subprocess's DCL then waits with LIB$SPAWN's wait
  (`AwaitSpawn`: LEF until the new process is deleted), handed to its
  shim as a `cliYield` wait, so the machine runs the others meanwhile.
  It takes no instructions of the subprocess's own, so no procedure page
  is needed as in process 1 (where SPAWN calls LIB$SPAWN through the
  machine, waiting inside the call). The messages (SPAWNED, ATTACHED,
  RETURNED) and SPAWN's status are process 1's SPAWN's (`spawnNotice`,
  `spawnReturned`, now shared). A process's subprocesses end with it
  (Phase 45's owner deletion).

  Fixed on the way: a SPAWN that LIB$SPAWN refused (SS$_DUPLNAM for a
  name in use, say) left `$STATUS` SS$_ABORT, a plain Go error's. Its
  status is now LIB$SPAWN's, in process 1 too.

  SPAWN's not repeating a status of govax's own (one with no message in
  the message file) stays as it was: the arguments its message needs
  aren't there to show it again.

  Tests: `TestSpawnDCL_nested` (SPAWN SPAWN, with the inner process's
  owner the subprocess and the innermost status coming back; a spawned
  procedure that SPAWNs a procedure and then SPAWN/NOWAIT a command that
  runs beside it; a refused SPAWN in a subprocess). HELP SPAWN says a
  subprocess can SPAWN.
- 2026-10-12: F$CONFIG(key [,operation]), a lexical function of govax's
  own (not VMS's; `internal/dclshell/dcllexconfig.go`): reads govax's
  configuration. VALUE (the default) is the item's string value, "" if
  it isn't set; EXISTS is 1 or 0. The key is lowercased first, since
  every configuration key is lowercase and DCL users write uppercase.
  The operation is a keyword table (`configOperations`), so another is
  one more entry; one it doesn't have is CLI_IVKEYW. F$CONFIG is in
  `lexicalNames`, so it abbreviates as the others do: F$CONF is it, and
  F$CON, which was F$CONTEXT (not implemented), is now ambiguous. It
  needs no console, so a subprocess's DCL has it too. Test:
  `TestLexicalConfig`; HELP LEXICAL.
- 2026-10-12: vax.init's memory sizes come from the configuration when
  it sets them: `vax.memory.size` (INITIALIZE/VAX's pages) and
  `vax.memory.p0`, `.p1`, `.s0`, `.ksp`, `.esp`, `.ssp`, `.isp`
  (VMINIT's qualifiers), each read with F$CONFIG and F$INTEGER (so
  decimal or `%X`), else vax.init's own value as before: 16384 pages,
  P0 16384, P1 8192, S0 8192, KSP 20, and ESP 8, SSP 8, ISP 4, which
  were VMINIT's defaults and are now written out. The values go into
  the commands by symbol substitution, in decimal (INITIALIZE/VAX's
  parameter is an expression in the console's radix, hence `^d`;
  VMINIT's qualifiers are always decimal). The keys are in
  `validConfigs` and HELP CONFIG KEYS. HELP VMINIT was out of date (it
  showed `region = pages` parameters): it now gives the qualifiers,
  /STRINGPOOL, and that physical memory left over goes to S0. Test:
  `TestVaxInitMemoryConfig` (internal/console/memconfig_test.go).
- 2026-10-12: Subtask 3, Ctrl/Y in a subprocess's procedures. The keys
  go to the process the terminal is attached to: corevms's
  `Environment.Attached` follows the `spawnWait` chain from process 1
  (a SPAWN's, or a program's LIB$SPAWN's, without /NOWAIT), and
  `HandleAttention` hands a key for a subprocess to its DCL
  (`subprocessAttention`), however deep its command level:
  - An image the subprocess runs is interrupted as VMS's DCL interrupts
    one, with an AST: a user-mode AST (`corevms.QueueCLIAST`) to a
    second routine in the CLI stub, which calls the CLI's shim. The shim
    hands the interpreter the CPU with `resumeControlY`, in the
    middle of the command that ran the image (`handBack`), and the
    interpreter acts there by process 1's rules (`imageInterrupted`):
    the Ctrl/Y level read with the image paused, CONTINUE returning
    from the AST (`yieldContinue`); otherwise the shim ends the image
    at once (`yieldEndImage`: ErrExit from the AST, unwinding to the
    image's outermost frame; no exit handlers), and the procedure acts
    after RUN, as for process 1. The engine's shim XFC now ends an
    image as the service XFC does (`ErrImageExit`). An AST the image
    ended before it took becomes a Ctrl/Y after the command.
  - During one of DCL's own commands the key is noted
    (`subprocessCLI.interrupted`): WAIT ends, and `takeControlY` acts
    after the command.
  - Outside a procedure, the key ends the subprocess's image (status
    SS$_CONTROLY), and its DCL goes on; it doesn't end govax.
  - `dclTakesControlY` adds that the terminal is attached to the
    DCL's process, so process 1, waiting in SPAWN, never takes a key.
  - A subprocess's terminal reads (the Ctrl/Y level, INQUIRE, READ
    SYS$COMMAND) read through its process's terminal, as its commands
    are, waiting in the scheduler; during a procedure line they read
    SYS$COMMAND (`corevms.ReadCommandLine`), not the procedure's data
    lines that RUN's SYS$INPUT is redirected to. The Ctrl/Y level
    prompts with the subprocess's prompt.

  Choices without VMS (DEVIATIONS.md, "[Phase 51] Ctrl/Y in a
  subprocess"): the AST's mode, level 0's behavior, /NOWAIT
  subprocesses not interrupted, no ATTACH. Tests
  (`internal/console/subcontroly_test.go`): STOP, ON CONTROL_Y, and
  CONTINUE at a spawned procedure's Ctrl/Y level (CONTINUE after a
  command run in the procedure's context, with the image paused),
  WAIT interrupted, level 0, and a chain of two SPAWNs, where the key
  goes to the inner one. Tried by hand at a pseudo-terminal through
  cmd/govax's key handling, too. HELP KEYS.
