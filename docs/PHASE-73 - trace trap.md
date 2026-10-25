# Phase 73 — the trace trap

**Status:** in progress. Planned 2026-10-24; the author answered Part
B's questions the same day: the probe runs, and govax follows its
answers. Subtasks 1 (the CPU's trace rules), 2 (SS$_TBIT), 3 (the
debugger beside a traced program), 4 (the probe, and govax changed
to its answers), 5 (DBG$INPUT and the debugger's log), and 6
(`ots72t.mar` with VMS's FORRTL) are done.

## Goal

govax takes the VAX trace fault as the architecture defines it, and VMS
reports it to a program as the condition SS$_TBIT. A program can then
trace itself as it can on VMS: set PSL<T> (BISPSW #^X10, or in a
signal array's PSL), take SS$_TBIT in a condition handler after each
instruction, and go on or stop by what the handler leaves in the
signal array's PC and PSL.

What started it: Phase 72's study of FORRTL's calls,
`testdata/ots72/ots72t.mar`, traces into FORRTL this way, and ran on
VMS only. On govax every case showed tracer state 0, because PSL<T> is
kept but never traps (Phase 72's side findings, "govax has no trace
trap").

1. **The CPU's trace rules**: T and TP, the trace fault before
   anything else at the start of an instruction, TP cleared by faults
   and kept by traps, CALLS/CALLG/RET/REI/CHMx/BISPSW as the manual
   gives them, and T and TP cleared in the PSL an exception starts.
2. **SS$_TBIT**: the trace fault signaled to the program's handlers,
   which aren't themselves traced; and continuing from it.
3. **The debugger beside a traced program**: what govax's debugger
   shows and does when the program traces itself, as VMS's debugger
   does where a probe can tell.

Out of scope: tracing in compatibility mode (govax has none); making
the debugger implement STEP with the T bit (it steps from outside the
machine, and keeps doing so: Decision 5).

## Where things stand (2026-10-25)

- **Done**: subtask 1, the CPU's trace rules (`internal/cpu/trace.go`);
  subtask 2, SS$_TBIT (`internal/corevms/condition.go`); subtask 3, the
  debugger beside a traced program (`internal/debugger/traced.go`); and
  subtask 4, the probe (`testdata/trace73`, two rounds on VMS 7.1), with
  govax changed to its answers and two oracles: `TestTrace73Oracle`
  (`internal/console`, VMS's images part by part) and the debugger's
  session oracle, which replays the ten sessions.
- **Done**: subtask 5, `DBG$INPUT`, `SET LOG`, and `SET OUTPUT`:
  `trace73.com` and `testdata/dbgcmd`'s `dbgcmd.com` run on govax as they
  did on VMS (`TestTrace73Procedure`, `TestDbgcmdProcedure`).
- **Done**: subtask 6, `ots72t.mar` on govax with VMS's FORRTL:
  `TestOTS72TOracle` holds every case to VMS's run when a local volume
  has FORRTL.EXE.
- **Next**: subtask 7, the close-out.

## Part A — the survey (2026-10-24)

### Sources

- *VAX Architecture Reference Manual* (EY-3459E-DP, 1987), chapter 5,
  "Trace Fault" (pp. 236–240): the rules of operation, Table 5.3, the
  trace handler conventions, and the between-instructions pseudocode
  under "Serialization of Notification of Multiple Events". Chapter 3's
  procedure call text: "the saved condition codes and the saved trace
  enable (PSW<T>) are cleared", and RET makes the frame's PSW<4> the
  new PSW<T>.
- *VAX/VMS Internals and Data Structures*, Table 4-2 (Trace Pending:
  SS$_TBIT, no extra parameters), and 16.4.4 (the debugger's
  watchpoints set the T bit to regain control after the instruction).
- *VMS 5.5 Debugger Manual* (AA-LA59D-TE), 3.7, "How the Debugger
  Controls Program Execution": the debugger is the primary exception
  handler, sees every exception first, and resignals those it didn't
  cause; it uses the T bit for STEP/INSTRUCTION, STEP/LINE, proceeding
  from a breakpoint, instruction-class breakpoints, and STEP/OVER's
  return; STEP/OVER and STEP/RETURN set a reserved bit in the saved
  PSW so RET faults.
- *VAX/VMS 2.0 Symbolic Debugger Reference*: "setting the trace trap
  condition code may cause a T-bit pending trap" (DEPOSIT to PSL).
- `vmsdef`: SS$_TBIT is ^X464, its message "T-bit pending trap at
  PC=!XH, PS=!XL".

### The architecture's rules

From the manual, in govax's terms:

1. At the start of an instruction, if TP is set, clear it and take the
   trace fault. The saved PC is the instruction about to run (the next
   one after the traced instruction). The trace fault takes precedence
   over every other exception, a reserved instruction and FPD
   included.
2. Then TP ← T, and the instruction runs.
3. If the instruction faults, or an interrupt is taken partway through
   it, TP is cleared before the PSL is saved (the instruction runs
   again, and is traced then).
4. If it aborts or takes an arithmetic trap, TP is unchanged in the
   saved PSL (so the trace comes after the trap handler's REI).
5. An interrupt taken after the instruction completes, before the next
   one's TP test, saves the PSL with TP unchanged.
6. CHMx clears T and TP in the new PSL; its saved PSL has T and TP set
   if T was set. A trace follows the service's REI.
7. REI: the next instruction is traced if T was set when the REI ran
   or the restored PSL's TP is set.
8. BISPSW setting T: tracing begins with the second instruction after
   it.
9. CALLS/CALLG save T clear, leaving PSL<T> as it was; RET restores T
   from the frame. So a RET ends tracing unless the trace handler set T
   in the frame. The manual's handler conventions: set T in the PSL the
   handler returns to (programs can clear it with RET, REI, BICPSW);
   leave TP alone while tracing goes on; clear both T and TP to stop.
10. Every exception and interrupt starts with T and TP clear in the new
    PSL, so handlers entered by an exception or CHMx aren't traced.

### govax today

- `registers.PSL` has `T()`, `TP()`, `SetT`, and the TP bit
  (`psl.go`); `cpu.ExcTracePending` is 0x28. Nothing reads them:
  `Engine.Step` (`engine.go`) never tests TP or loads it from T.
- `Engine.raise` resets PC to `instructionPC` for every exception. A
  trap sets `instructionPC` to the next PC first (`arithmeticTrap`,
  `emulChmx`; integer overflow and divide by zero through
  `arithmeticTrap`), but nothing says whether a `Fault` is a fault or
  a trap, which rule 3 and 4 need.
- `HandleFault` pushes the current PSL and runs the handler with the
  same PSL but its IPL: T and TP aren't cleared (rule 10). With the
  console-handler vector it offers the exception to
  `corevms.Environment.DispatchException`, which runs the handler
  search with the live PSL unchanged too.
- `corevms.exceptionCondition` has no condition for 0x28 (and
  `TestExceptionCondition` asserts there isn't one).
- `continueCondition` resumes an exception at the signal array's PC,
  with the signal array's PSL low byte (the condition codes, T, and the
  trap enables) over the PSL the exception had. So a handler that
  changes the PC or clears T is obeyed. TP (bit 30) comes from the
  saved PSL, not the signal array's.
- CALLS/CALLG save PSW<T> clear, and RET restores the whole PSW from
  the frame (`call.go`), as rule 9 says. BISPSW/BICPSW change T. REI
  loads the whole PSL.
- AST delivery (`ast.go`'s `deliverAST`) calls the AST routine as
  CALLG would, in the current PSL: an AST routine would run traced.
  The AST's exit restores PC and PSL from the argument list.
- System services are XFC shims run in Go from the P1 vector's stubs,
  not CHMK. A traced program sees a trace after each instruction of
  the stub (its CALL, the XFC, the RET) where VMS gives one after the
  CHMK's REI and one after the RET.
- The debugger (`internal/debugger`) steps from outside the machine:
  STEP is one `Machine.StepMachine`, breakpoints and instruction-class
  breakpoints are checked in Go, watchpoints compare values after each
  step. It never sets PSL<T>. SET BREAK/EXCEPTION sees conditions
  through the RTL's offer to the debugger. The console's SET
  BREAKPOINT/FAULT (`cpu/faultbreak.go`) stops on an exception code
  before delivery.

## Part B — decisions and questions

### Decisions (proposed 2026-10-24)

1. **TP in `Step`.** After interrupt and AST delivery, before the
   decode: if TP is set, clear it, set `instructionPC` to PC, and
   raise `ExcTracePending`; otherwise TP ← T. One test of two PSL
   bits per instruction. Interrupts and ASTs delivered earlier in the
   step keep TP in the PSL they save (rule 5).
2. **Faults and traps told apart.** A `Fault` gets a `Trap bool`,
   set by `arithmeticTrap` and `emulChmx`. `raise` clears TP before
   delivery of a fault (rule 3) and leaves it for a trap (rules 4, 6).
   A trace fault saves TP clear, having cleared it itself.
3. **Exceptions start untraced.** `HandleFault` clears T and TP in the
   new PSL, after saving the old one (rule 10). `DispatchException`
   clears them in the live PSL for the handler search; the signal
   array keeps the exception's PSL (TP clear for a trace fault), and
   `continueCondition`'s resume brings T back from it. A trace handler
   isn't traced, so it can't recurse.
4. **SS$_TBIT.** `exceptionCondition` maps 0x28 to SS$_TBIT with no
   detail longwords (Internals Table 4-2). Unhandled, the catch-all
   writes `%SYSTEM-F-TBIT` and its severity ends the image, as for any
   F condition (VMS's ending is a probe question).
5. **The debugger stays outside the machine, and follows VMS's
   debugger** (answer 2). It doesn't use the T bit for its own work.
   Where the probe shows VMS's debugger taking a trace the program
   meant for itself (the instruction a STEP runs, say), govax's
   debugger takes it too: the trace that step leaves pending is
   dropped, and the program's handler doesn't see it. Until the probe
   runs, subtask 3 builds the debugger's side so that choice is one
   place to change.
6. **Services stay shims, traced as govax runs them** (the author,
   2026-10-24). A program tracing through a service call gets a trace
   at each instruction govax's P1 vector entry really runs (Question
   6). govax doesn't add, drop, or move traces to imitate VMS's
   `CHMK`. The probe records VMS's traces, and where they differ the
   difference is described in `docs/DEVIATIONS.md` and the manual.
7. **AST routines aren't traced** (answer 3): `deliverAST` clears T
   and TP in the PSL the routine runs in, and the AST's exit restores
   the interrupted PSL, T and TP with it. The probe confirms it.
8. **LIB$SIGNAL and an unhandled SS$_TBIT follow the probe** (answers
   4 and 5): whether the handlers a traced LIB$SIGNAL calls are traced,
   the SS$_TBIT signal array's PSL, and how an unhandled SS$_TBIT ends
   the image. Decision 4's catch-all ending stands until then.

### Questions for the author (answered 2026-10-24)

The author's answers, in order: 1, yes, the author will run the probe;
2, follow VMS: if the probe shows the debugger taking the traces,
govax's does too (Decision 5); 3, ASTs aren't traced (Decision 7); 4,
follow the probe; 5, follow the probe where possible (Decision 8); 6,
the question was unclear. What it meant: on VMS a service call is
`CALLS` to the P1 vector entry, then a `CHMK` there that runs the
service in kernel mode untraced (the trace comes after its REI), then
a `RET`. A traced program sees a trace before each of those. govax's
entry runs an XFC to a Go shim instead, so the number of traces, and
the PCs they report, may differ. The probe records VMS's. The author's
answer, once the question was explained: report govax's traces as they
are, and document the difference, rather than fake traces to match
VMS (Decision 6).

1. A probe on the simh VAX (VMS 7.1), `testdata/trace73`, for
   subtask 4? It would answer questions 2 to 6.
2. The debugger and a traced program (Decision 5): when the debugger
   steps a program whose own T is set, does VMS's debugger take the
   trace and the program's handler miss it? After the step, does T
   stay set (the manual's convention 7)? What does SET
   BREAK/EXCEPTION show for SS$_TBIT, and does GO pass it on? If VMS
   loses the program's traces, should govax copy that, or keep giving
   the program every trace (Decision 5's default)?
3. AST routines: does an AST delivered to a traced program run
   traced? (Most likely not: delivery is an interrupt and an REI the
   executive builds.) If not, `deliverAST` clears T and TP for the
   routine, and the AST exit restores them from the saved PSL.
4. LIB$SIGNAL and LIB$STOP from traced code: LIB$SIGNAL is user-mode
   code on VMS, not an exception, so are the handlers it calls traced?
   And a trace fault during a handler search a LIB$SIGNAL began.
5. The SS$_TBIT signal array: its PSL's TP (0, by rule 3 and the
   fault's clearing it), and what an unhandled SS$_TBIT does (message,
   traceback, image exit).
6. Tracing around a system service: the traces VMS gives for a
   `CALLS #n,G^SYS$xxx` and a `$xxx_S` CHMK, to fill in Decision 6's
   difference.

## Part C — subtasks

1. **The CPU's trace rules** (Decisions 1 to 3, the CPU side): TP in
   `Step`, `Fault.Trap`, T and TP cleared in `HandleFault`'s new PSL.
   Tests in `internal/cpu` for each row of the manual's Table 5.3 that
   govax can reach: an ordinary instruction, a fault, an arithmetic
   trap, CHMx, REI with each combination of T and saved T/TP, BISPSW
   (the second instruction after it), CALLS/RET (RET ending tracing,
   a T set in the frame resuming it), and an interrupt between
   instructions.
2. **SS$_TBIT** (Decisions 3 and 4, the RTL side): the condition, the
   handler search with T and TP clear, continuing with the signal
   array's T. Tests in `internal/corevms`, and an end-to-end test in
   `internal/console` running `ots72t.mar`'s case 0 (the tracer's
   self-test, which should show tracer state 2, R2 `12345678` at the
   RSB, and `87654321` below R1).
3. **The debugger beside a traced program** (Decision 5): EXAMINE
   PSL showing T and TP, DEPOSIT to the PSL setting T, SET
   BREAK/EXCEPTION on SS$_TBIT, STEP through traced code (where the
   step lands when the trace fault comes first), and the console's SET
   BREAKPOINT/FAULT on the trace fault. Tests in `internal/debugger`.
4. **The probe** (Question 1): `testdata/trace73`, a program that
   traces itself through ordinary code, a CALLS/RET, a system service,
   an AST, a LIB$SIGNAL, and an unhandled SS$_TBIT, logging each trace's
   PC and PSL; and a debugger session over it (Question 2). Run on
   VMS 7.1, then govax changed to its answers, with an oracle test.
5. **`DBG$INPUT`, `SET LOG`, `SET OUTPUT`** (added 2026-10-25, the
   author's request): RUN/DEBUG reads its commands from the file
   `DBG$INPUT` names, if one does, until EXIT, QUIT, or its end (which is
   EXIT); `SET LOG file` and `SET OUTPUT [NO]LOG, [NO]VERIFY,
   [NO]TERMINAL` write a session log in VMS's form (the lines read from
   `DBG$INPUT` as they are, every output line after a `!`, an `@`
   procedure's lines echoed with a blank before them; the program's own
   output not in it). Then `trace73.com` and `testdata/dbgcmd`'s
   `dbgcmd.com` run on govax as they did on VMS, and RUN/DEBUG in a
   procedure no longer leaves its session (and image file) open.
6. **`ots72t.mar` on govax**: with VMS's FORRTL, the study's
   FORRTL cases traced as on VMS. `ots72t-vax.txt` becomes an oracle
   for what the tracer records, but for what depends on VMS's own
   routines (addresses in VMS's images, the H digits).
7. **The close-out**: `docs/MANUAL.md` (the debugger and a traced
   program; SS$_TBIT), `docs/DEVIATIONS.md` (Decision 6), Phase 72's
   side finding marked fixed, CLAUDE.md, PLAN.md.

## Progress log

### 2026-10-24 — Planned

The survey and proposals above, from the architecture manual, the
Internals book, the debugger manuals, and govax's CPU, condition
dispatch, and debugger. The author answered Part B's questions the
same day; Decisions 5 to 8 record the answers.

### 2026-10-24 — Subtask 1, the CPU's trace rules

- `internal/cpu/trace.go`: `beginTrace`, run by `Step` after interrupt
  and AST delivery and before the decode, takes the trace fault if TP
  is set (clearing it; the saved PC is the instruction about to run),
  and otherwise copies T into TP. The file's opening comment explains
  the two bits for a reader new to them.
- `Fault.Trap` (Decision 2): `arithmeticTrap` and `emulChmx` set it.
  `raise` clears TP before delivering a fault, and leaves it for a
  trap, so a trapping instruction's trace comes after the trap
  handler's REI.
- `HandleFault` clears T and TP in the PSL its handler starts with
  (the saved PSL keeps them), on the SCB-vector path. The
  console-handler path, `DispatchException`, is subtask 2's.
- REI sets TP if T was set when it began (rule 7), as well as when the
  restored PSL has TP.
- `deliverAST` clears T and TP for the AST routine (Decision 7); the
  AST's exit already restores the interrupted PSL whole.
- Tests (`trace_test.go`), from the manual's Table 5.3: every
  instruction traced with T, none without; BISPSW starting tracing an
  instruction late and BICPSW traced last; a fault's saved TP clear;
  an arithmetic trap's TP set and its trace after the handler; CHMK's
  handler untraced; CALLS saving T clear, RET ending tracing, and a T
  set in the frame resuming it after the instruction after the RET; REI
  in six combinations of T and the restored T and TP; an interrupt
  between instructions saving TP and taking the trace after its REI.

### 2026-10-24 — Subtask 2, SS$_TBIT

- kernel.asm's SCB sends the trace fault (EXC$TP, ^X28) to
  console$handler, so a running program's trace fault reaches
  `DispatchException`. It had no vector before: the fault would have
  gone to vector 0.
- `exceptionCondition` maps it to SS$_TBIT with no detail longwords
  (Decision 4); `TestExceptionCondition` no longer lists ^X28 among
  the exceptions that aren't signaled.
- `DispatchException` clears T and TP in the live PSL once the arrays
  are laid out, so the handler search and the handlers run untraced
  (Decision 3), as `HandleFault`'s SCB path already does. The signal
  array keeps the exception's PSL. `continueCondition` was already
  right: T comes from the signal array's low byte, TP from the
  exception's PSL (clear for a trace fault, kept for a trap).
- An unhandled SS$_TBIT is severe: the catch-all writes
  `%SYSTEM-F-TBIT, T-bit pending trap at PC=..., PS=...`, the
  traceback handler (in an image linked with traceback) its dump, and
  the image ends with SS$_TBIT, message inhibited. Unconfirmed until
  the probe (Decision 8).
- Left as it is until the probe (Decision 8): LIB$SIGNAL and LIB$STOP
  don't touch T, so the handlers a traced LIB$SIGNAL calls run with
  PSL<T> as the caller had it (CALLS saves T clear but leaves PSL<T>)
  and are traced, their traces nested dispatches.
- Side fix: SHOW's SCB vector names had the trace vector at ^X29
  (`machine/show.go`; and "Chnage Mode" for CHMS).
- Tests: `internal/corevms/trace_test.go` (the signal array and the
  untraced search; continuing with T kept, T cleared, and a new PC; a
  trap's TP kept through a dispatch; the unhandled catch-all's message
  and status), and `internal/console/trace73_test.go`:
  `TestTrace73SelfTest` runs `ots72t.mar` and checks its case 0
  against VMS's run (tracer state 2, the value at R0, R2 `12345678` at
  the RSB, `87654321` below R1; the SS$_TBIT signal array left in the
  frame, `00000003 00000464 000011F5 03C00000`, is VMS's too), and
  `TestTrace73Unhandled` a BISPSW with no handler (the first trace
  before the second instruction after it). The assembler's kernel.asm
  golden image has the new SCB longword.

### 2026-10-24 — Subtask 3, the debugger beside a traced program

What each command does when the program traces itself; the opening
comment of `internal/debugger/traced.go` lists them.

- EXAMINE PSL already showed T and TP. DEPOSIT to the PSL wasn't
  implemented (it evaluated `PSL` as an address): `depositPSL`
  (`data.go`) replaces the PSL with a longword, and with `/WORD` or
  `/BYTE` only its low word or byte, as the VMS debugger manuals'
  `DEPOSIT/WORD PSL = 0` and the 2.0 reference's `DEP/HEX/WORD PSL =
  expression` do; other sizes are OPTNOTALLOW. Setting T starts
  tracing. govax lets every bit change, as its SET PSL does;
  unconfirmed what VMS's debugger, a user-mode program, does when
  asked to change the privileged half.
- STEP: before this, a step whose instruction had a trace pending ran
  the trace fault's dispatch, landing in the dispatcher's stub
  (`stepped to 7FFEE118: XFC #^X7A`) and then the handler, so stepping
  traced code took several STEPs per instruction. Now `stepOne` first
  gives the pending trace to the program (`deliverPendingTrace`): the
  handler runs silently, as STEP/OVER runs a called routine, until the
  program is back (same mode, off the interrupt stack, TP clear, SP at
  or above where it was: `traceReturn.back`), and then the step runs
  its instruction. A step still moves one instruction, and the handler
  sees every trace, as under GO. A breakpoint in the handler, or a SET
  BREAK/EXCEPTION, stops the step there.
- Any run that starts at a breakpoint (GO, STEP/CALL, STEP/RETURN, and
  the run above) keeps its first-instruction exemption until the
  program is back from a trace pending there, so a breakpoint at the
  instruction a trace comes before doesn't stop the program a second
  time when the handler returns (`runLoop`).
- Decision 5: `programGetsTrace` is the one place that decides who
  takes a trace pending when a STEP begins; it says the program, until
  the probe. With the debugger taking it (`takeTraces`, set only by the
  tests), the trace is dropped (TP cleared) and the program's handler
  misses the traces of the instructions stepped.
- SET BREAK/EXCEPTION stopped at each SS$_TBIT, but its message showed
  the $FAO directives unfilled (`T-bit pending trap at PC=!XH,
  PS=!XL`), as it would have for any condition with parameters: the
  message is now formatted from the signal array as the catch-all
  formats it (`corevms`'s `signalMessage`, carried to the debugger in
  `UnhandledCondition.Message`). GO passes the condition on to the
  handler. The break says "at" the instruction the trace comes before;
  whether VMS says "at" or "preceding" is the probe's. At the break the
  registers are still the dispatcher's, as for every condition
  (the session oracle's `reasonDispatcher`).
- SET BREAK/FAULT=28 (the trace fault's vector) stopped before
  delivery, but the trace was lost: `beginTrace` had cleared TP, and
  GO went on without it. Now the stop leaves TP set (EXAMINE PSL shows
  the trace pending) and the next instruction boundary delivers it
  without stopping again (`cpu.Engine`'s `holdTrace`, `Fault.held`);
  clearing TP at the stop drops it.
- Tests: `internal/debugger/traced_test.go` (EXAMINE PSL, STEP through
  traced code, GO and STEP from a breakpoint before a pending trace, a
  breakpoint in the handler, SET BREAK/EXCEPTION, SET BREAK/FAULT=28,
  DEPOSIT to the PSL, and Decision 5's other answer), and
  `TestTraceFaultBreakpoint` in `internal/cpu/trace_test.go`.
- `debug.help`'s DEPOSIT and STEP, and the manual's DEPOSIT row.

### 2026-10-25 — Subtask 4, the probe written

`testdata/trace73` (its README says how to run it): `trace73.mar`, 12
parts picked by `TRACE73_PART`, each tracing itself under a handler,
`TRACER`, that records every condition it's called for (who, the
condition, the signal array's PC and PSL, the handler's own PSL), and
the main routine's `OUTER`, which gets the traces taken while `TRACER`
itself runs. Parts: A ordinary code; B arithmetic traps; C CALLS/RET, a
routine setting T in its frame, JSB/RSB; D REI with each T and TP; E
SYS$GETTIM by CALLS and CALLG, LIB$GET_EF, LIB$FREE_EF; F `$DCLAST`;
G LIB$SIGNAL; H the handler moving the PC; I the handler setting TP
and Z and clearing T in the signal PSL; J an unhandled SS$_TBIT (also
under TRACE73NT, `/NOTRACEBACK`, and TRACE73D run `/NODEBUG`); K an
access violation while tracing; N nothing traced, for the debugger.
Ten debugger sessions (`tr*.dbg`) ask Question 2: STEP and GO through
traced code, SET BREAK/EXCEPTION, a breakpoint in TRACER, stepping
traced calls and services, DEPOSIT to the PSL (the low word, then the
privileged half last), and an unhandled SS$_TBIT under the debugger.

govax's dry run (2026-10-25), by label, to compare with VMS's:

- A, C, D, H, K follow the architecture's rules: the first trace
  before the second instruction after the BISPSW, the BICPSW traced,
  RET ending the tracing unless the routine set T in its frame, each
  REI case as rule 7 says, the moved PC obeyed, the access violation's
  signal PSL with TP clear and the trace after the instruction it
  continued at.
- B: a trap's signal PSL has TP set (`43C00032`), and the trace at the
  same PC follows the handler's continue.
- E: three traces for each service call (`SYS$GETTIM+2`, `+4`, then the
  instruction after the CALLS), and three in each LIBRTL routine's stub
  (`+2`, `+9`, `+B`): Decision 6's difference, to be set beside VMS's.
- F: the AST routine runs untraced (its PSL `03C00000`) and no trace
  falls inside it (Decision 7).
- G: TRACER is called by govax's LIB$SIGNAL with T and TP set
  (`handler=43C00010`), so it runs traced and OUTER gets its 28 traces;
  the ENDOFFILE signal array's PSL is `43C00000` (Decision 8, the
  probe's).
- I: TP set in the signal PSL is ignored (no second trace before I2:
  `continueCondition` takes TP from the exception's PSL); Z is obeyed
  (the BEQL branches); clearing T stops the tracing.
- J: `%SYSTEM-F-TBIT, T-bit pending trap at PC=..., PS=03C00010`, the
  traceback (with lines under TRACE73D), the "improperly handled" dump
  without traceback; `$STATUS` `%X10000464`.

Side finding: govax's debugger doesn't read `DBG$INPUT`, so the
sessions do nothing on govax, and `RUN/DEBUG` in a command procedure
leaves the session's image file open after the procedure's next `$`
line (DISMOUNT then reports a user file open). The oracle will replay
the `.dbg` files directly, as `TestDebuggerSessionOracle` does.

### 2026-10-25 — Subtask 4, VMS's answers (round 1)

Round 1 ran on OpenVMS V7.1 (simh's VAX 8600); `testdata/trace73/vax`
has the log, sessions, maps, objects, and images (VMS's image matches
govax's byte for byte past the header, so the PCs name the same
labels). What it says, against govax's dry run above:

- **The same on both**: parts A, B, C, D, F, H, K, record for record
  (the PC, the signal PSL, the handler's PSL); J's traceback, the
  "improperly handled" dump's signal array, and `$STATUS`
  `%X10000464`. ASTs aren't traced (Decision 7 confirmed).
- **The message**: VMS 7.1 says `T-bit pending trap at PC=00003B1D,
  PSL=03C00010`; govax's text (the generated table's) says `PS=`. To
  go in `vmsdef`'s `vaxMessages`.
- **Part I**: VMS honors TP set in the signal array's PSL: a second
  trace before I2. govax takes TP from the exception's PSL. Taking it
  from the signal array instead also gives part B's answer (a trap's
  signal PSL has TP set) and part K's (a fault's has it clear).
- **Part E** (Decision 6's difference): a service call gives three
  traces on both, at the entry's `+2` and `+6` on VMS (its CHMK is four
  bytes) and `+2` and `+4` on govax. A LIBRTL routine gives about 15
  traces in VMS's code, with IV set (its entry mask), and three in
  govax's stub. To be documented, not imitated.
- **Part G**: LIB$SIGNAL from traced code ended the image before its
  report, with `%SYSTEM-W-ENDOFFILE` and `$STATUS %X10000870`. Round 2
  (`trace73b.mar`) reports from an exit handler, and tries the signal
  with LIB$SIGNAL's own code untraced (part L) and with no handler in
  the part's frame (part M).
- **The debugger** (Question 2; Decision 5 says follow VMS):
  - A breakpoint at an instruction a trace is pending before takes the
    trace: the break shows T 1 and TP 0, and the program's handler
    never sees it (TRSTEP, TRGO, TRSVC, TRCALL).
  - Going on from a breakpoint or a step, by STEP or GO, clears the
    program's T after the first instruction: after the first STEP, T
    is 0 and no trace reaches the program again (TRSTEP, TRCALL); GO
    from breakpoints in traced code gives the program no traces
    (TRGO). A T set by DEPOSIT goes the same way (TRDEP, TRDEPGO: no
    traces).
  - A breakpoint in the trace handler doesn't disturb the tracing: GO
    from it, and the program gets every trace (TRHAND).
  - SET BREAK/EXCEPTION stops at each SS$_TBIT with its message
    (`PSL=`), "break on exception at" the instruction the trace comes
    before, with the exception's registers and frames (SHOW CALLS shows
    the program's), `%EXC_NAME` `'TBIT'`; GO passes it to the handler,
    and tracing goes on. A STEP from it goes "to routine TRACER", and
    the signal array's PSL then has T clear, so the tracing stops after
    the handler continues (TREXC).
  - An unhandled SS$_TBIT: "break on unhandled exception at" the
    instruction; GO says `%DEBUG-I-CONFROMEXC, WARNING: continuing after
    severe error; integrity not guaranteed` and continues the program,
    which takes the next trace (TRUNH).
  - DEPOSIT to the PSL changes only its low word: `DEPOSIT PSL =
    03C00000` and `DEPOSIT PSL = 0` leave the modes as they were, and
    `DEPOSIT/BYTE PSL = 0F` sets the condition codes (TRPRIV).
  - Under the debugger, VMS's own debugger code in P0 is traced after a
    service's CHMK (TRSVC's records at `P0 000912xx`): VMS's internals,
    nothing govax can match.

### 2026-10-25 — Subtask 4, round 2 and govax changed to the answers

Round 2 (`trace73b.mar`, `vax/trace73b.log`) answers part G: LIB$SIGNAL
from traced code gives the handlers 54 traces of LIB$SIGNAL's code and
of the condition search's user-mode code (in S0), then no handler is
ever called for the condition: the image ends with its message
(`%SYSTEM-W-ENDOFFILE`, as `$PUTMSG` writes it) and the condition,
marked STS$M_INHIB_MSG, as its status (`10000870`), and the exit
handler runs. With no handler in the part's frame (M) the same; with
LIB$SIGNAL's own code untraced (L) the signal is handled as usual, its
signal PSL with T clear (govax's too). Decision 8's answer.

govax now follows round 1 and round 2:

- `vmsdef`'s `vaxMessages` has SS$_TBIT's VAX text, `PC=!XL, PSL=!XL`.
- `continueCondition` takes TP from the signal array's PSL with its low
  byte (part I; B and K come out the same either way).
- `corevms/signal.go`'s `tracedSignal`: LIB$SIGNAL (and LIB$STOP,
  unconfirmed) called with PSL<T> set writes the condition's message,
  clears T and TP, and ends the image by `$EXIT` with the condition as
  its status. govax's LIB$SIGNAL is a shim, so the traces before it are
  the stub's (`+2`, `+9`), where VMS's are its code's and the search's:
  a Decision 6 difference.
- The debugger (Decision 5, `traced.go`): a breakpoint, watchpoint, or
  STEP/RETURN's RET takes the trace pending where it stops
  (`stopTakesTrace`); going on from such a stop the first instruction
  is the debugger's and T and TP are cleared after it (`ownsNext`,
  `dropTracing`); a STEP's end clears them (`stepEnded`). Exception
  breaks, Ctrl-C, and govax's fault breakpoints leave the trace to the
  program, as subtask 3 had it. The Decision 5 switch
  (`programGetsTrace`, `takeTraces`) is gone. With SET STEP INSTRUCTION
  a breakpoint's "break at" line shows the instruction (TRSTEP,
  TRCALL), as an exception break's already did.
- GO (or STEP) from a break on an unhandled severe condition says
  `%DEBUG-I-CONFROMEXC, WARNING: continuing after severe error;
  integrity not guaranteed` and continues it (TRUNH): the catch-all and
  the traceback handler continue a severe condition the debugger held
  (`conditionDispatch.debuggerHeld`), but a LIB$STOP (unconfirmed). An
  access violation so continued runs its instruction again and faults
  again (`TestUnhandledExceptionBreak`, which had assumed the image
  ended: no VMS log had a GO there).
- DEPOSIT to the PSL changes only its low word (TRPRIV).
- Left as differences: STEP from a break on SS$_TBIT (VMS goes into the
  handler and clears T in the signal array's PSL; govax steps the
  dispatcher, as for every condition, the session oracle's
  `reasonStepException`); the traces in VMS's LIBRTL and debugger code
  (parts E and G, TRSVC); the service vector's offsets (`+6` on VMS,
  `+4` on govax).
- Tests: `internal/debugger/traced_test.go` rewritten to VMS's answers
  (a breakpoint takes the trace; GO from a breakpoint ends the tracing;
  a breakpoint in the handler doesn't; SET BREAK/EXCEPTION; the fault
  breakpoint; DEPOSIT's low word; GO from an unhandled SS$_TBIT);
  `TestSrchandler_traceContinue`'s TP case; `TestLibSignal_traced`.

Next in this subtask: `DBG$INPUT` and the debugger's `SET LOG` and `SET
OUTPUT`, which the probe's sessions use (the author asked for them, so
govax can run `trace73.com` as VMS did), then the oracle test.

### 2026-10-25 — Subtask 4, the oracles

- `TestTrace73Oracle` (`internal/console/trace73oracle_test.go`) runs
  VMS's own TRACE73, TRACE73NT, TRACE73D (`/NODEBUG`), and TRACE73B on
  govax, part by part, and holds each part's report and `$STATUS` to
  VMS's logs. Parts E and F (the service vector's offsets, LIBRTL's
  code) and round 2's G and M (LIB$SIGNAL's code) are compared without
  their records outside the program and the counts (`trace73Differences`);
  part J's "improperly handled" dump by `excmsgMatch`'s rules, R1 aside
  (what LIB$PUT_OUTPUT leaves, which varies on VMS too).
- The debugger's session oracle (`TestDebuggerSessionOracle`) replays
  the ten sessions on VMS's TRACE73D with `TRACE73_PART` defined
  (`runImageDefining`), the program's own lines left out
  (`trace73Output`). TRSTEP, TRGO, TRCALL, TRDEP, TRDEPGO, and TRPRIV
  match VMS's logs whole; TREXC, TRHAND, and TRUNH differ only where
  govax pauses in the dispatcher at an exception break or SHOW CALLS in
  a handler lacks the "above condition handler" lines (both known), and
  TRSVC in the traces VMS's own debugger code added.
- The oracle normalizes the test console's kernel mode to user mode in
  a PSL (it runs programs in kernel mode, VMS in user mode), which also
  took faillnk's ACCVIO message and exam's EXAMINE PSL off the list
  (their reason, `reasonUserMode`, is gone).
- Found along the way, changed to VMS's answers: EVALUATE shows a
  string in apostrophes (`'TBIT'`; govax had double quotes, which no
  log showed); a STEP into a routine by instruction names the place and
  its instruction (`stepped to TRACE73\CSUB\CS1: NOP`), by line the
  routine (`stepped to routine ...`, unconfirmed which of the unit and
  the label decides).

### 2026-10-25 — Subtask 5, DBG$INPUT, SET LOG, SET OUTPUT

From the *VMS 5.5 Debugger Manual*'s 8.1 and 8.3, and the two probes'
VMS logs:

- **DBG$INPUT** (`debugger/dbginput.go`): when RUN/DEBUG opens a session
  and the logical name names a file (not a terminal), the debugger reads
  its commands from it (DCL's `DebuggerInput` and `ReadDebuggerFile`,
  `dclshell/dbginput.go`) until one ends the session or the file ends,
  which is EXIT; RUN is over when the session is. The file is read once
  the run that opened the session is over (`Start` around `start`), so a
  GO among its commands is a run of its own, and the image's exit is
  reported (`%DEBUG-I-EXITSTATUS`).
- **EXIT and QUIT** (`exitSession`) end a program still running under
  the session (`EndAbandonedImage`; no exit handlers, unconfirmed), or
  let go of one that exited, whose memory the session kept for examining
  (corevms's KeepP0) and which held the image file open
  (`ResetICBList`). RUN's status is the image's, marked STS$M_INHIB_MSG
  (`%X10000001`), or `%X10028009` for an image still running, as VMS
  7.1's (TRUNH). This also ends the image file being left open by
  RUN/DEBUG in a procedure (subtask 4's side finding), and a session a
  procedure's `$` line ends the same way.
- **The log** (`debugger/dbglog.go`): SET LOG file (`.LOG` default,
  DEBUG.LOG when none is named; a quoted host name is taken as DCL's
  OPEN takes it), SET OUTPUT [NO]LOG, [NO]VERIFY, [NO]TERMINAL,
  [NO]SCREEN_LOG (accepted), SHOW OUTPUT (the manual's form), SHOW LOG
  (govax's words). The debugger's output is the machine's output while a
  session is in progress (`installOutput`); the program's goes to its
  terminal another way, so it isn't logged. Each command read from the
  debugger's input (DBG$INPUT, the terminal, a procedure's data lines) is
  logged as read; output lines after a "!". DCL makes the file
  (`CreateDebuggerLog`, a VAR file of records).
- **@** reads its file itself now (`runProcedure`; DCL's
  `RunDebuggerProcedure` and its "plain" levels are left without callers,
  to be taken out): a failing command is reported and the file goes on
  (the manual); with VERIFY each line is echoed after a blank, between
  `%DEBUG-I-VERIFYICF` lines, the echo held until the command's first
  output or its end and dropped if it fails first, as VMS's logs leave
  out a failing command's echo (`pendingEcho`); a GO, STEP, or CALL
  shows it as it starts. A DO clause's commands are echoed with VERIFY
  as VMS's are (`cutAfter`: " EXAMINE R0;", "  SHOW CALLS").
- **Found by the full runs**, changed:
  - The message of an unhandled condition the debugger takes is the
    debugger's to show (`unhandledBreak`), so it is in the log; the
    catch-all writes it only when no debugger takes it.
  - "break at", "stepped to", and the rest named a location's offset in
    the console's radix, which vax.init sets decimal (`PART_N+176` for
    VMS's `PART_N+0B0`): the machine's `LocationTextIn` and
    `InstructionTextIn` take the debugger's radix (`locationText`,
    `instructionAt`).
- **Results**: on govax, `trace73.com`'s TRSTEP, TRGO, TRCALL, TRDEP,
  TRDEPGO, and TRPRIV logs are VMS 7.1's line for line, and
  `dbgcmd.com`'s BRKCLS, WATCH, and TRACE VMS 7.3's; every other
  difference is one the session oracle lists. `TestTrace73Procedure` and
  `TestDbgcmdProcedure` (`internal/console/trace73session_test.go`) run
  both procedures on a fresh volume and hold those logs to VMS's, the
  RUN/DEBUG statuses to trace73's, and check that the volume dismounts.
- **The prompt's history** (the author's request, `cmd/govax/history.go`):
  the console, the debugger, and the assembler each recall their own
  commands, kept in `~/.govax/history.txt`, `debug_history.txt`, and
  `asm_history.txt`. readline's own switch (`SetHistoryPath`) starts
  the new history empty, so govax keeps each mode's lines and its file
  itself and refills readline's in-memory history when the mode changes
  (`modeHistories`, `TestModeHistories`).

### 2026-10-25 — Subtask 6, `ots72t.mar` with VMS's FORRTL

- `TestOTS72TOracle` (`internal/console/ots72toracle_test.go`): VMS 7.1's
  FORRTL.EXE is licensed, so the test looks for it on the local-only
  volumes (`testdata/disks/rq0-ra92.dsk`, the simh VAX's system disk,
  then `xfer.dsk`), mounts it read-only as DUA5, defines FORRTL to it,
  and MACROs, links (`FORRTL/SHAREABLE` in an options file), and runs
  `ots72t.mar` on a fresh volume, as `ots72.com` did on VMS; without one
  it skips. The study's 29 FORRTL cases are traced into FORRTL and
  caught at the JSB to govax's `OTS$$CVT_x_T_R8` and at its RSB, as on
  VMS, and `$STATUS` is SS$_NORMAL.
- What it holds to `vax/ots72t-vax.txt`: each case's text (what FORRTL
  wrote) and tracer state, the value at R0, and the records' fields
  both sides decide: FORRTL's arguments (R0, R2, the kind at FP-9, FP-16
  for E, FP-36 for F), the buffer's offset from FP and the frame's shape
  (AP and SP from FP, at the JSB and the RSB), and the routine's results
  (the sign; the exponent, or for a zero that it's left as it was; the
  guard digit; the digits, govax's starting with all of VMS's, which
  stop sooner). Not compared, being VMS's own: stack addresses, its
  images' addresses (FORRTL is placed elsewhere), the registers its
  routine leaves (VMS's changes R2; FORRTL reads none), FP-32 before
  the call, and what earlier calls left on the stack. Everything matches
  but the 40-digit H case (`ots72tDifferences`): VMS's H digits drift
  past H's precision, Phase 72's known difference.
- Found and fixed: LINK didn't translate a bare name in an options file
  as a logical name, applying the options file's device and directory
  instead (`FORRTL/SHAREABLE` with FORRTL defined: NOSUCHFILE).
  `rms.Session.LocateRelated` now takes a bare name that is a logical
  name as its translation (a single one as itself, so a default type
  goes on the translation), as RMS translates the primary name before
  applying related-file fields; `TestLocateRelated` has the case.
