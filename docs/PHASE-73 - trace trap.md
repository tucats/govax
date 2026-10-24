# Phase 73 — the trace trap

**Status:** planned (2026-10-24). The author answered Part B's
questions the same day: the probe runs, and govax follows its answers.

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

- Planned. Nothing is implemented yet.

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
6. **Services stay shims; the probe measures the difference.** A
   program tracing through a service call gets a trace at each
   instruction of the P1 vector entry it runs (Question 6). Where the
   probe's count or PCs differ from govax's, the stub's traces are made
   to match VMS's if that's simple (the XFC standing for the CHMK);
   otherwise the difference is logged in `docs/DEVIATIONS.md`.
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
the PCs they report, may differ. The probe records VMS's, and Decision
6 says what govax does about a difference.

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
