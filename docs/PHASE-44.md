# Phase 44 — Multiprocessing, part 2: the scheduler

**Status:** planned (2026-10-06); decisions taken 2026-10-06 (see
PHASE-43.md, Part A). Not started. Needs Phase 43.

The program this phase belongs to — its goal, architecture, rules for
every commit, decisions, and known bugs — is in
[PHASE-43.md](PHASE-43.md), Part A. Read that first.

## Goal

Make the engine run several processes: choose which one runs, switch to
another when the current one waits, when its quantum runs out, or when a
higher-priority one becomes computable, and idle when none can run. All of
it behind `vax.process.scheduler`; with the flag off, or with only one
process, govax behaves exactly as it does now.

Processes in this phase are still built by hand (Phase 43's builder, from
Go tests); `$CREPRC` is Phase 45. That keeps the scheduler testable on its
own: tiny VAX loops in two or three processes, and the services that
already wait (`$HIBER`, `$WAITFR`, mailbox reads).

## What earlier phases leave in place

- Phase 43: `corevms.System` with the process table and current process;
  per-process `Environment`s; hardware PCBs, `Engine.SaveContext`/
  `LoadContext` (the `SVPCTX`/`LDPCTX` code); `vm.AddressSpace`;
  per-process stacks and image state; the three `vax.process.*` settings,
  read but inert.
- The wait-by-retry model: a waiting service returns `ErrWait`, the
  engine re-executes the XFC (`internal/cpu/xfc.go`).
- `Engine.Step`'s per-instruction work: limits, attention, the clock
  (`tickQuantum` or `pollHostClock`), interrupt delivery, AST delivery
  (`cpu.ASTSource`), the fetch window, decode, execute.
- Timers are per-Environment queues, expired when a waiting service
  retries (`corevms/timers.go`'s `expireTimers`).
- Run loops: `Console.runPlain` (no debugger) and the debugger's
  `runLoop`, both ending on Step errors (`ReportStop`).

## Design

### `internal/sched`

A leaf package with no CPU or VMS dependency, so its rules are unit-tested
directly:

- **States**: current, computable, and the wait states govax uses — LEF,
  CEF, HIB, SUSP, MWAIT (with a resource: RWMBX, RWAST, ...) — and
  deleted. Codes and names as VMS's (`SCH$C_*` in `internal/vmsdef`), so
  `$GETJPI`'s `JPI$_STATE` and SHOW SYSTEM can use them.
- **Priorities**: base and current, 0–31; one FIFO queue of computable
  processes per priority.
- **Choosing**: the head of the highest non-empty queue.
- **Quantum**: each process has a remaining count; the hook decrements it;
  at zero, a normal process's current priority decays by one (not below
  base) and it goes to the back of its queue, if another process of the
  same or higher priority is computable (otherwise it keeps running with
  a fresh quantum, as VMS does).
- **Boosts**: when an event ends a normal process's wait, its current
  priority is raised by the event's boost (I/O completion, wakeup, event
  flag, resource available, terminal input/output), up to the
  normal-priority ceiling; real-time processes are never boosted. The
  boost values come from the manuals or Decision 6's book (*VAX/VMS
  Internals and Data Structures*, usable in full); otherwise
  chosen and logged as unconfirmed.
- **Preemption test**: when a process becomes computable with a higher
  current priority than the current process's, a reschedule is requested.

The package keeps opaque process handles; `corevms` owns the processes.

### The engine's hook

`internal/cpu` gets a `Scheduler` interface (installed with the system
services, as `ASTSource` is) and keeps the per-instruction cost to a
decrement and a compare when nothing is due. Step:

1. counts the instruction against the current process's quantum (and its
   CPU time, for `JPI$_CPUTIM`);
2. at quantum end, or when a reschedule has been requested (a wait, a
   preemption), and if preemption is allowed now (IPL < 3, not on the
   interrupt stack, and the mode allowed by `vax.process.preempt`), asks
   the scheduler for the next process;
3. if it's another process, switches: `SaveContext` into the current
   PCB, `LoadContext` from the next (PCBB, TB, fetch window), the Go-side
   current Environment, and continues with the next instruction — the
   new process's.

A switch is between instructions, after any interrupt delivery and before
AST delivery, so the new process's own pending ASTs are delivered at once.
The existing hooks (system services, ASTs, attention, exceptions) reach
the current process through `System.Current`; the console's
`cpu.SystemServices` methods delegate there instead of to a fixed
`Console.RTL`.

### Waits

When a service returns `ErrWait`, it also says why and until what: a wait
state and a predicate (a Go function: "is the wakeup pending", "is event
flag *n* set", "has this mailbox room"). The hook marks the process
waiting and reschedules. The scheduler re-tests waiting processes'
predicates when choosing; a process whose predicate is true, or that has a
deliverable AST, becomes computable (with its boost). When it runs, it
re-executes its XFC, exactly as now. Re-testing every waiting process at
each choice is simple and cannot miss a wakeup; with a handful of
processes it's cheap. (If it ever isn't, events can report to waiters
directly, as VMS's report-system-event does.)

A waiting process with a deliverable AST runs the AST and then waits
again (its XFC re-executes after the AST's REI), as on VMS.

### Idle and time

When nothing is computable the engine runs the "null process": it
doesn't execute VAX instructions, but advances emulated time — in
quantum-clock mode, to the next due timer of any process (deterministic);
in hardware-clock mode, by sleeping until then. Timers become
system-wide in effect: every tick (and every idle step) expires due timers
in all processes, not only the current one's. If nothing is computable and
no timer is pending, the machine idles until Ctrl-C, as a lone `$HIBER`
does now; a debug trace (`SET DEBUG PROCESS`) notes it.

With one process, a wait finds no one else computable, so the process
retries at once: today's behavior, whatever the flag.

### The console and debugger

- **Whose run ends a run.** `ErrConsoleCallReturned`, `ErrImageExit`, an
  unhandled condition's image exit, and HALT end the console's run only
  when process 1 causes them. In another process they end that process's
  image (in this phase the hand-built process just stops: state "deleted",
  never scheduled; Phase 45 adds real deletion). HALT in another process
  halts the machine (it's a machine-wide instruction) and is reported
  naming the process.
- **The console's view.** When a run stops, the console switches the
  CPU back to process 1 before its next command (EXAMINE, SHOW
  REGISTERS, RUN) unless the stop was in another process, in which case
  the debugger shows that process's state and switches back when it
  resumes or exits. Exactly how a stop in a subprocess is presented (a
  process name in the message, or SHOW PROCESS) is decided in subtask 9
  with Decision 11.
- **Stepping** (STEP, `SET STEP`) runs with scheduling frozen (no switches),
  so a step is one instruction of the debugged process; GO unfreezes it.
- **Breakpoints** carry the process they belong to (process 1, Decision
  11); a breakpoint address reached by another process is ignored.
- **Nested runs** (the legacy CHF's `Console.Call`, bug 7) run with
  scheduling frozen.
- **Limits** (`--instruction-limit`, `--time-limit`) count every
  process's instructions: they bound the machine, not one process.

### SHOW SYSTEM and SHOW PROCESS

Console commands showing the process table, in VMS's layouts (PID,
process name, state, priority, CPU time, ...), and one process's details.
The layouts come from the User's Manual and, if Decision 7 allows, a VMS
7.3 probe; otherwise chosen and logged as unconfirmed.

## Subtasks

1. **`internal/sched`.** States, queues, choosing, quantum, decay, boosts,
   preemption test; table-driven tests of each rule (round robin among
   equals, priority order, decay floor at base, no boost for real-time,
   quantum end with no competitor keeps the CPU).
2. **The engine hook.** The `Scheduler` interface, the per-instruction
   count, the reschedule request, the preemption test with
   `vax.process.preempt`, CPU-time accounting. A benchmark (the existing
   `bench_test.go` workloads) confirms single-process speed is unchanged,
   flag on and off; record the numbers in `docs/PERFORMANCE.md`.
3. **Switching.** The switch sequence above; the console's service hooks
   delegate to the current process; `Console.RTL` becomes an accessor for
   process 1 (or is replaced) so console commands keep their meaning.
4. **Wait states.** Each waiting service gives a state and predicate:
   `$HIBER` (HIB), `$WAITFR`/`$WFLOR`/`$WFLAND` (LEF, or CEF for a common
   cluster), `$QIOW` and the mailbox driver's waits (MWAIT/RWMBX, LEF on
   the request's event flag), `$SYNCH`, condition-dispatch waits. AST
   arrival makes a waiter computable. Tests: two hand-built processes
   ping-ponging with `$WAKE`/`$HIBER` (`processTarget` finds the other
   process in the table now), and with a common event flag cluster.
5. **Idle and timers.** The null process, system-wide timer expiry,
   quantum-mode idle that jumps to the next due timer, hardware-clock idle
   that sleeps. Tests: two processes each waiting on `$SETIMR` with
   different times wake in time order.
6. **Priority preemption.** A process made computable at a higher
   priority runs at the next boundary; `$SETPRI` on the current process
   reschedules when it lowers itself below a computable process. Tests.
7. **Process-aware run loops.** `runPlain` and the debugger's `runLoop`
   end only for process 1's outcomes; other processes' image ends and
   faults are handled as above. Tests: process 2 returning from its main
   routine doesn't end process 1's RUN.
8. **Frozen scheduling** for STEP and nested runs (bug 7); breakpoints
   qualified by process (bug 9). Tests, including the Phase 42 debugger
   oracles unchanged.
9. **The stopped-in-another-process experience.** Decide and implement
   how a stop (Ctrl-C, HALT, fault break) in process 2 is shown and what
   EXAMINE sees then; tests.
10. **SHOW SYSTEM and SHOW PROCESS** (console; and the debugger's own
    if its grammar should have one). Grammar, help, tests. Optional probe:
    `SHOW SYSTEM` and `SHOW PROCESS/ALL` on VMS 7.3 with a subprocess
    running.
11. **Determinism.** A test runs a three-process workload twice and
    compares instruction-by-instruction traces of which process ran;
    another runs it with three quanta and checks the outcomes agree.
12. **Close-out.** Status, progress log, PLAN.md, CLAUDE.md (`internal/sched`,
    the hook), HELP (SHOW SYSTEM, the settings), PERFORMANCE.md.

## Open questions

- Whether `$SETPRI`'s and `$GETJPI`'s view of "current priority" should
  include boosts (VMS's `JPI$_PRI` does). Probably yes.
- What VMS does at quantum end for a process at real-time priority (no
  quantum preemption among real-time processes, by the manuals' account);
  confirm in subtask 1.
- How much the scheduler's per-instruction work costs; if the decrement
  in Step is measurable, fold it into the existing `tickQuantum` counter.

## Progress log

- 2026-10-06: Planned with Phase 43.
- 2026-10-06: The author took every recommended decision in
  PHASE-43.md, Part A.
