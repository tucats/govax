# Phase 44 — Multiprocessing, part 2: the scheduler

**Status:** in progress (started 2026-10-07); decisions taken 2026-10-06
(see PHASE-43.md, Part A). Subtasks 1–4 done.

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
  read but inert. Specifically, as Phase 43 left them (its progress log
  has the details):
  - Building a process by hand: `corevms.NewEnvironment` (adds it to the
    table), `System.BuildAddressSpace(pid, p0, p1)` (P1 at least
    `corevms.MinP1Pages` for the user stack), `System.BuildStacks`, and
    `corevms.InitialPCB` written with `cpu.WritePCB` at
    `Stacks.PCBB`; `console/handswitch_test.go` is the worked example.
    `System.RemoveProcess` frees the address space and pool pages.
  - Process 1 has no PCB page until `System.EnsurePCB` gives it one,
    which must happen after the microkernel is assembled (the pool's
    bottom pages are claimed by it); its PCB's memory-management
    longwords must be written before its first `SaveContext`, which
    doesn't write them.
  - `System.ProcessSettings` holds the three settings; `Current`/
    `SetCurrent` are the Go side's current process, which nothing yet
    keeps in step with PCBB.
  - The console's image state is per process (`imagesOf(env)`), but RUN
    only ever activates in process 1.
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
  at zero it gets a fresh quantum, and a normal process is rescheduled: it
  goes to the back of its queue, so a computable process of the same
  priority gets a turn; with none, it's chosen again (and keeps the CPU).
  A real-time process has no quantum end.
- **Decay**: each time a normal process above its base is *chosen*, its
  current priority drops by one first (the book, section 10.3.3, step 3),
  so a compute-bound process drifts back to its base at quantum ends.
- **Boosts**: when an event ends a normal process's wait, its current
  priority is raised by the event's boost (I/O completion, wakeup, event
  flag, resource available, terminal input/output), up to the
  normal-priority ceiling; real-time processes are never boosted. The
  boost values come from the manuals or Decision 6's book (*VAX/VMS
  Internals and Data Structures*, usable in full); otherwise
  chosen and logged as unconfirmed.
- **Preemption test**: when a process becomes computable with a current
  priority higher than *or equal to* the current process's, a reschedule
  is requested (the book, section 10.2.3, step 4); an equal one then runs
  first, the preempted process going behind it.

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
- ~~What VMS does at quantum end for a process at real-time priority~~
  Settled in subtask 1: nothing but a fresh quantum (the book, sections
  10.1.2.1 and 10.1.2.4); real-time processes run until they wait or a
  higher-or-equal priority process preempts them.
- ~~How much the scheduler's per-instruction work costs~~ Measured in
  subtask 2: nothing measurable with the flag off, at most about 1% with
  it on (docs/PERFORMANCE.md, "Check: the scheduler hook"); no need to
  fold it into `tickQuantum`.

## Progress log

- 2026-10-06: Planned with Phase 43.
- 2026-10-06: The author took every recommended decision in
  PHASE-43.md, Part A.
- 2026-10-07: **Subtask 1 done: `internal/sched`.** A leaf package
  (`doc.go`, `state.go`, `boost.go`, `scheduler.go`), its rules from
  *VAX/VMS Internals and Data Structures* (Decision 6), chapter 10,
  cited in the comments:
  - `State` is VMS's `SCH$C_*` numbering (Table 10-1; `TestStateCodes`
    checks it against `vmsdef.Symbols`), with `IsWait` and SHOW SYSTEM's
    names. `Resource` is an MWAIT's resource, numbered as Table 10-2's
    `RSN$_*` values (`RSN$_ASTWAIT` 1, `RSN$_MAILBOX` 2, ...), named
    `RWAST`, `RWMBX`, ... by `StateName`. **Unconfirmed:** the book is
    VMS 3.3's; that VMS 7.3 numbers the resources the same, and SHOW
    SYSTEM's `RW` names, await the subtask 10 probe. There is no
    "deleted" state: `Remove` forgets the process, as VMS does.
  - `Class` is the priority increment class (`PRI$_NULL`, `IOCOM`,
    `RESAVL` = `TIMER`, `TOCOM`, `TICOM`) and `Boost` its increment, 0,
    2, 3, 4, 6 (Table 10-3); process creation is the `TICOM` class. The
    boost rule is the book's three steps (section 10.2.4): base plus the
    increment, but not below the current priority, and the base if that's
    above 15 (so real-time processes, and normal ones at base 14 or 15,
    are never boosted).
  - `Scheduler`: `Add` (computable, with a creation boost class),
    `Remove`, `Wait` (state and resource), `Ready` (an event: boost,
    computable, and whether it preempts), `Tick` (one instruction:
    CPU count, quantum), `Reschedule` (the book's SCH$RESCHED then
    SCH$SCHED: current to the back of its queue; head of the highest
    queue chosen, demoted by one if a normal process above base),
    `SetBasePriority`, and `Info`/`Handles`/`Computable` for SHOW
    SYSTEM. Processes are opaque `Handle`s (govax will use PIDs).
  - Two places the plan's Design differed from the book, now corrected
    above: decay happens when a process is chosen, not at quantum end
    as such; and a newly computable process preempts at an *equal*
    priority too, not only a higher one.
  - A waiting process keeps the rest of its quantum (VMS does too); VMS's
    IOTA (a quantum charge per wait) is left out, as is everything about
    swapping and paging (COMO, LEFO, HIBO, SUSPO, PFW, FPG, COLPG have
    codes but no use).
  - `SetBasePriority` sets base and current priority to the new value;
    Table 10-3's "Set Priority" boost of 2, and how `$SETPRI` combines
    it, are left for subtask 6. Lowering the current process below
    (strictly) a computable one reschedules; raising a computable process
    preempts by the usual test.
  - Tests (`scheduler_test.go`, 97% coverage): round robin among equals,
    priority order, quantum end with no competitor or only a lower one
    keeps the CPU, real-time has no quantum end and no boost or decay,
    decay floor at base, the boost table and rule, the preemption test
    (higher, equal, lower, boosted, real-time), idling and waking, a
    waiting process keeping its quantum, removal, `SetBasePriority`,
    errors, and the start of the book's Figure 10-2 example.
- 2026-10-07: **Subtask 2 done: the engine hook.**
  - `internal/cpu/schedule.go`: the `Scheduler` interface, one method,
    `Schedule(e, ran, preemptible) (next, err)`: charge `ran`
    instructions to the current process, decide (and, from subtask 3,
    switch), and return how many instructions may run before the next
    call. `Engine.SetScheduler(s, modes)` installs it (nil removes it);
    it isn't picked up from the system services as `ASTSource` is, so
    with the flag off the engine has no hook at all and pays one nil
    test per instruction. `Step` counts down `schedLeft` after interrupt
    delivery and before AST delivery, calling the hook at zero.
    `RequestReschedule` makes the next boundary call it, keeping the
    count of instructions run right. `Preemptible` is the plan's test:
    IPL below 3, not on the interrupt stack, and the current mode in the
    `PreemptModes` the setting allows (`corevms.PreemptMode.Modes`).
  - `internal/corevms/schedule.go`: the `System` owns a
    `sched.Scheduler` (`System.Scheduler()`), kept in step with the
    process table: `addProcess` adds each process by PID at its base
    priority (class `PRI$_NULL`; `$CREPRC`'s creation boost is Phase
    45's), `RemoveProcess` removes it. `SetProcessSettings` replaces
    `ProcessSettings` and gives the scheduler its quantum
    (`sched.SetQuantum` now also cuts a longer quantum under way, since
    process 1 exists before the console applies the settings).
    `System.Schedule` is the hook: charge, and when a reschedule is
    due, choose, if preemption is allowed, or always when the current
    process has stopped running (waited); a quantum end that isn't
    preemptible yet is retried at the next boundary (budget 1). It
    returns the rest of the quantum otherwise. Choosing another process
    returns `ErrProcessSwitch` until subtask 3, and no computable
    process `ErrNoComputableProcess` until subtask 5's idle loop.
  - CPU time: `sched` counts each process's instructions (`Info.CPU`,
    `System.CPUInstructions`). Turning that into `JPI$_CPUTIM`'s 10 ms
    units, which `$GETJPI` doesn't return yet, is for `$GETJPI`'s
    cross-process work in Phase 45.
  - The console installs the System as the hook in `newRTL` (so on
    every INIT, VMINIT, and ZERO) when `vax.process.scheduler` is on.
  - Tests: `internal/cpu/schedule_test.go` (a fake scheduler: when it's
    called and with what counts, `RequestReschedule`, a budget below 1,
    removal, errors stopping `Step`, and the preemption test's table);
    `internal/corevms/schedule_test.go` (the scheduler follows the
    table, the settings, one process through quantum ends preemptible
    or not, the not-yet errors); `internal/console/schedule_test.go`
    (FORTH with the scheduler on and a 997-instruction quantum gives
    the same session as without, and its instructions are charged to
    process 1).
  - Benchmark: `BenchmarkSieveScheduled` beside `BenchmarkSieve`; no
    measurable cost off, at most about 1% on (docs/PERFORMANCE.md).
  - For subtask 4: an event that makes another process computable
    mid-budget (`sched.Ready` requesting a reschedule) has to reach
    `Engine.RequestReschedule`, or the switch waits for the current
    quantum's end. The System has no engine pointer yet; the waits'
    subtask gives it one (or a callback).
- 2026-10-07: **Subtask 3 done: switching.**
  - `System.Schedule` now switches when the scheduler chooses another
    process (`switchTo`, `corevms/schedule.go`): point PCBB at the
    outgoing process's PCB (process 1 gets its PCB page from
    `EnsurePCB` the first time it's switched out), write its
    memory-management longwords from the CPU's live P0BR/P0LR/P1BR/
    P1LR/ASTLVL/PME (`Engine.SaveMemoryContext`, new in
    `cpu/context.go`: SVPCTX doesn't save them, and process 1's grow as
    images expand P0, so they're written at every switch rather than
    whenever they change), `SaveContext`, PCBB to the incoming PCB,
    `LoadContext` (which empties the per-process TB entries and with
    them the fetch window), then `SetCurrent`. With `SET DEBUG PROCESS`
    each switch is traced (`DEBUG(PROCESS): SWITCH 00000301 ->
    00000302, PC=...`). `ErrProcessSwitch` is gone. A switch with no
    outgoing process (the current one deleted) is refused for now: the
    CPU wouldn't be on the interrupt stack where `LoadContext` starts;
    Phase 45's deletion settles it.
  - The console's engine hooks (`SystemService`, `Shim`, `NextAST`,
    `HandleAttention`, `DispatchException`) reach the System's current
    process (`Console.running()`), not `c.RTL`. `c.RTL` stays process
    1's Environment, so console commands keep their meaning; nothing
    else needed changing. (Ctrl-C/Ctrl-Y ASTs therefore go to whichever
    process is running; on VMS they belong to the terminal's owner.
    Revisit with subtask 9.)
  - Found while testing: a process made computable at the current
    process's priority preempts it at the hook's next call (the book's
    "higher or equal" rule, subtask 1), so a hand-built process 2 runs
    first; and the scheduler's figures (CPU, quantum left) lag the
    engine by the instructions since the hook's last call, which is
    charged at the next one. `Engine.RequestReschedule` forces the
    charge; SHOW SYSTEM (subtask 10) should do that before reading.
  - `sched.Resource`'s `ResourceNone` prints as `NONE` (it printed
    `MWAIT`).
  - Tests: `internal/console/schedswitch_test.go`, booting as cmd/govax
    does with the scheduler on: two counting processes at the same P0
    address take turns exactly a quantum each (process 1 first using
    the rest of the quantum it began at boot), each count right after
    every turn, each process charged its instructions, the switches
    traced, and process 1's PCB holding its address space; and
    `$SETPRN` called by process 2 names process 2, not process 1. The
    corevms test now checks a switch to a process with no PCB is
    refused. Benchmarks unchanged.
- 2026-10-07: **Subtask 4 done: wait states** (`corevms/waits.go`).
  - A waiting service now says what it waits for before returning
    `ErrWait`: `env.waitOn(state, resource, class, over)`, where `over`
    is a Go test of whether the wait has ended (`waitOnFlag` picks LEF
    or CEF by the flag's cluster). `SystemService` and `Shim` pass that
    to `enterWait`, which, when the scheduler is installed, puts the
    process in that wait (`sched.Wait`) and asks the engine to
    reschedule; without the scheduler nothing changes and the process
    spins on its XFC as before. A service that waits without saying why
    is in LEF with no test, computable again at the next choice (a
    yield): the debugger's wait in condition dispatch
    (`OnUnhandled`) is one, rightly, since it isn't a VMS wait.
  - The waits: `$HIBER` (HIB, until a wakeup is pending), `$WAITFR`/
    `$WFLOR`/`$WFLAND` and `$SYNCH`'s flag step (LEF, or CEF for flags
    64-127, until the flags hold; the cluster is looked up at each test),
    `$QIOW` with a pending request (LEF, until the driver completes
    it), and a write to a full mailbox, by `$QIO` or `$QIOW` (MWAIT/
    RWMBX, until a read is waiting or there's room for the message).
  - `System.Schedule` first tests every waiting process
    (`wakeWaiters`, skipped when `System.waiters` is 0): each one's due
    timers are expired, and one whose test is true, or that has an AST
    it could take where it waits (`astDeliverable`: NextAST's rules at
    the PSL saved in its PCB, from the new side-effect-free
    `Process.deliverableAST`), becomes computable with its wait's boost
    class. It then runs the AST, and its XFC, run again after the AST's
    REI, finishes or waits again, as on VMS. When nothing is computable,
    every waiter is made computable without a boost to retry
    (`retryWaiters`): today's spin, until subtask 5's idle loop.
  - Boost classes (**unconfirmed**, govax's choice, since VMS's come
    from whoever reports the event): HIB and RWMBX end with
    `PRI$_RESAVL` (3, the book's "Wake a Process"/"Resource
    Available"), event-flag and `$QIOW` waits with `PRI$_IOCOM` (2).
  - `$WAKE` reaches any process in the table (`processTarget`), setting
    its wakeup and, for another process, asking for a reschedule so a
    woken process of higher or equal priority preempts at once.
    `$SETEF` of a common flag asks for one too. `$SCHDWK`/`$CANWAK`
    still act only on the caller (Phase 45, with timers across
    processes).
  - The console installs the scheduler with `System.InstallScheduler`
    (which keeps the engine, for `RequestReschedule`), answering subtask
    2's note.
  - Tests: `internal/console/schedwait_test.go`, booted with the
    scheduler on and a quantum longer than the run, so only waits
    switch: two processes ping-pong with `$WAKE`/`$HIBER` and through a
    common event flag cluster (`$ASCEFC`, `$SETEF`, `$WAITFR`), taking
    turns evenly with the other seen in HIB or CEF; and a hibernating
    process wakes by its own `$SETIMR` AST while process 1 never waits.
    Each fails with the wait states disabled (checked by hand).
    `internal/corevms/waits_test.go`: each service's state and resource,
    its end (and the boost), no change without the scheduler, the
    unspecified wait, `retryWaiters`, and `$WAKE` of another process.
    Benchmarks unchanged.
