# Phase 73 — the trace trap

**Status:** in progress. Planned 2026-10-24; the author answered Part
B's questions the same day: the probe runs, and govax follows its
answers. Subtasks 1 (the CPU's trace rules), 2 (SS$_TBIT), and 3 (the
debugger beside a traced program) are done.

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

## Where things stand (2026-10-24)

- **Done**: subtask 1, the CPU's trace rules (`internal/cpu/trace.go`),
  and subtask 2, SS$_TBIT (`internal/corevms/condition.go`): a program
  can trace itself, and `ots72t.mar`'s self-test runs on govax as on
  VMS (the progress log). Subtask 3, the debugger beside a traced
  program (`internal/debugger/traced.go`): the program gets every
  trace, a STEP included, with Decision 5's other answer one function
  away (`programGetsTrace`).
- **In progress**: subtask 4, the probe. `testdata/trace73` is written
  and runs on govax (the progress log has govax's answers); it waits
  for the VAX run, after which govax follows its answers and gets an
  oracle test.

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
5. **`ots72t.mar` on govax**: with VMS's FORRTL, the study's
   FORRTL cases traced as on VMS. `ots72t-vax.txt` becomes an oracle
   for what the tracer records, but for what depends on VMS's own
   routines (addresses in VMS's images, the H digits).
6. **The close-out**: `docs/MANUAL.md` (the debugger and a traced
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
