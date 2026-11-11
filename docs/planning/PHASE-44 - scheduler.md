# Phase 44 — Multiprocessing, part 2: the scheduler

**Status:** done (2026-10-07); decisions taken 2026-10-06 (see
PHASE-43.md, Part A).

The program this phase belongs to — its goal, architecture, rules for
every commit, decisions, and known bugs — is in
[PHASE-43 - processes](PHASE-43%20-%20processes.md), Part A. Read that first.

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
- **The console's view** (decided by the author in subtask 9,
  2026-10-07). A run that stops while another process holds the CPU
  (CTRL/C, a limit, HALT, a fault break) leaves the CPU in that process:
  the stop message names it ("... at PC = 00000406 in process
  00000302"), and EXAMINE and SHOW REGISTERS see its registers and
  memory. The next run (GO, STEP, CALL, RUN) and the debugger's EXIT
  give the CPU back to process 1 first, the other process keeping its
  place in the scheduler. A later debugger command, SET PROCESS, will
  let the user choose the process (see "Future work").
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

## Future work

- **The debugger's `SET PROCESS [pid]`** (asked for by the author,
  2026-10-07): a command that switches the CPU to the process named
  (process 1 with no pid, or `SET PROCESS/ALL`-style forms if VMS's
  debugger has them), so its registers and memory can be examined, as
  after a stop in that process. `System.SwitchCPU` (subtask 9) does the
  switch, keeping the scheduler in step. Still to decide: whether STEP
  and breakpoints then follow that process (Decision 11 now ties them to
  process 1, and `Console.RunningProcessOne` gates the eventpoints), a
  SHOW PROCESS showing which one the debugger is looking at, and the
  command's name and syntax against the VMS debugger's own (its SET
  PROCESS is for multiprocess programs; a probe could settle the form).
  Not yet scheduled to a subtask; a candidate for subtask 10, beside
  SHOW SYSTEM/SHOW PROCESS, or a later phase. *Done (2026-10-08, in the
  program's close-out; PHASE-48.md's progress log):* `SET PROCESS
  [/VISIBLE] [pid|name]` and `SHOW PROCESS` (`debugger/process.go`);
  STEP and breakpoints stay with process 1, and the next run gives the
  CPU back to it.

## Open questions

- ~~Whether `$SETPRI`'s and `$GETJPI`'s view of "current priority" should
  include boosts~~ Settled in subtask 6: `JPI$_PRI` is the scheduler's
  current priority, boosts included; `JPI$_PRIB` the base.
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
- 2026-10-07: **Subtask 5 done: idle and timers.**
  - Engine (`cpu/idle.go`): `IdleUntil(t, limit)` moves emulated time to
    `t` for a scheduler with nothing to run: in quantum-clock mode the
    clock jumps there (whole milliseconds, rounding up; refused while
    the interval clock runs or interrupts are queued, whose ticks can't
    be skipped), in hardware-clock mode it sleeps, in 10 ms slices, at
    most `limit`, stopping early for a control key.
    `InstructionsUntil(t)` is how many instructions run before the
    quantum clock reaches `t` (exact: the rest of this millisecond plus
    whole ones); in hardware-clock mode, a fixed 16,384.
  - corevms: timers are system-wide in effect. Every scheduling call
    first expires every process's due timers (`pollEvents`), and the
    budget the hook returns is the rest of the quantum but no more than
    the instructions until the next timer of any process
    (`System.budget`, `nextTimer`), so a timer ends its process's wait
    on time while another process runs.
  - The null process (`corevms/idle.go`): when nothing is computable,
    `idle` moves time to the next timer due, expires it, tests the
    waiters, and repeats until one can run (at most 64 timers per call,
    so timers that end no wait, such as a repeating `$SCHDWK` for a
    process that isn't hibernating, can't keep it from checking for
    CTRL/C; and at most 50 ms of sleep in hardware-clock mode). With no
    timer pending, every waiter retries its service (the old spin),
    between instructions where CTRL/C and the limits are checked, and
    `SET DEBUG PROCESS` notes it once (`IDLE: every process waits and no
    timer is due`); each jump is traced as `IDLE for n ms`.
  - Bug fixed: adding a process asked the scheduler for a reschedule
    but not the engine, whose budget could be a whole quantum, so a new
    process waited for the current one's quantum end or service call
    (the earlier tests all made service calls, or asked by hand).
    `addProcess` now calls `requestReschedule`.
  - Note for test writers: vax.init sets `SET QUANTUM 1`, one emulated
    millisecond per instruction, so instruction counts show up in
    measured times at that rate.
  - Tests: `cpu/idle_test.go` (`InstructionsUntil` exact, the quantum
    jump and its refusal, the hardware sleep, its limit, and CTRL/C);
    `console/schedwait_test.go`: two processes waiting on 2 s and 1 s
    timers wake in time order and on time after a few dozen
    instructions; a timer wait ends on time while the other process
    computes through a 10-million-instruction quantum (it fails without
    the timer budget); hardware-clock idling sleeps until the timers;
    and with every process hibernating and no timer, the processes
    spin and the trace says so once. Benchmarks unchanged.
- 2026-10-07: **Subtask 6 done: priority preemption.**
  - Most of it was already in place: a process made computable at a
    priority higher than or equal to the current one's requests a
    reschedule (`sched`), and the events that make one computable ask
    the engine to call the scheduler at the next boundary (`$WAKE` of
    another process, `$SETEF` of a common flag, adding a process; timers
    by the budget). Tests now show it end to end.
  - `$SETPRI` gives the scheduler the new base priority
    (`sched.SetBasePriority`: current priority = base; a computable
    process requeued, preempting if it now outranks the current one;
    the current one rescheduled if a computable process now outranks
    it) and asks for a reschedule. Table 10-3's "Set Priority" boost of
    2 isn't applied (**unconfirmed**: the book doesn't say which of
    `$SETPRI`'s targets it reaches). `$SETPRI` still acts only on the
    caller (Phase 45).
  - `JPI$_PRI` reports the scheduler's current priority, boosts
    included, as VMS's does (`currentPriority`).
  - Tests (`console/schedwait_test.go`): a hibernating process at
    priority 8 woken by process 1 (at 4) runs at the next boundary,
    before process 1's next instruction; one at 0, boosted only to 3,
    stays computable and never runs while process 1 computes; process 1
    lowering itself to 2 with `$SETPRI` gives a computable process at 3
    the CPU at once and doesn't get it back. `corevms`: `$SETPRI`
    reaches the scheduler, and `JPI$_PRI` shows a wakeup's boost.
- 2026-10-07: **Subtask 7 done: process-aware run loops.**
  - Every way an image ends reaches `Engine.Step` as
    `cpu.ErrConsoleCallReturned`: its outermost procedure's RET to the
    frame RUN or CALL built, or `$EXIT` (and an unhandled condition's
    exit) unwinding to it (`exitImage`). `Console.StepMachine` is `Step`
    for the run loops (`runPlain`, the debugger's `runLoop` and
    `stepOne`): when that comes from a process other than process 1, the
    process stops (`System.StopProcess`: image rundown, out of the
    scheduler for good, `Environment.Stopped`, a reschedule; it stays in
    the table with its memory until Phase 45 deletes such processes) and
    the run goes on. Only process 1's ends the console's RUN, CALL, or
    GO.
  - HALT stops the machine whoever executes it; one in another process
    is reported whatever the verbosity, naming it (`%SYSTEM-S-HALT, cpu
    halted at PC = ... in process 00000302`). What the console and
    debugger show after a stop in another process is subtask 9's.
  - Tests (`console/schedrun_test.go`): process 1's CALL counts to its
    end although process 2's image ends first, by RET and by `$EXIT`,
    with the console's run loop and the debugger's (each fails without
    `StepMachine`); process 2 is stopped and out of the scheduler. A
    kernel-mode HALT in process 2 stops a GO, naming the process. The
    Phase 42 debugger oracles are unchanged.
- 2026-10-07: **Subtask 8 done: frozen scheduling; process 1's
  eventpoints** (PHASE-43.md's bugs 7 and 9).
  - `Engine.FreezeScheduling()` (nesting; returns the unfreeze) holds
    back the scheduler's calls, so the CPU's process keeps it. The
    instruction count goes on while frozen, so the instructions are
    charged, and a reschedule asked for meanwhile happens, at the first
    call after the last unfreeze. The debugger's STEP freezes for the
    whole command (Decision 11: stepping freezes the other processes;
    GO lets them run again), and the CHF's nested condition-handler run
    (`chf.go`'s `Console.Call`) freezes around the call, fixing bug 7.
  - Breakpoints, tracepoints, instruction breakpoints, STEP/RETURN's
    wait, and watchpoints are process 1's (`Console.RunningProcessOne`):
    the debugger's run loop checks them only while process 1 runs, so
    another process at the same P0 address passes them (bug 9).
    Watchpoints are checked after process 1's instructions only: a
    change another process makes to a shared (S0) location is reported
    after process 1's next instruction. A GO from a breakpoint exempts
    process 1's first instruction from the check until process 1 runs
    it (formerly the loop's first instruction, which with a switch at
    once was another process's: process 1 then stopped at the same
    breakpoint again, forever).
  - `Engine.SwitchIfDue` (`Console.BeginStep`) gives the scheduler its
    turn before the run loops look at the next instruction, so the
    trace and the checks see the process that will run it. (For
    breakpoints alone it isn't needed: a process is switched out only
    after its next instruction was checked. A test confirms it either
    way.) Its fast path is one compare, inlined into the loops (with no
    scheduler the budget sits at a huge value); an A/B of
    `BenchmarkSieve` shows no difference beyond noise.
  - `sched.Scheduler.RequestReschedule`: a reschedule as if the quantum
    had ended (for tests, and later for the services).
  - Tests: `cpu` (freeze nesting, charging after unfreeze, the deferred
    reschedule; `SwitchIfDue` when due, not due, frozen, without a
    scheduler); `console/schedrun_test.go` with the debugger and both
    processes running the same counter at the same address: a
    breakpoint stops only process 1, one count per GO, at the BRB, at
    the INCL, and with a switch forced at each GO; breakpoints on both
    instructions stop at each of process 1's instructions in turn; STEP
    40 leaves process 2 where it was and GO lets it run. Each fix
    except `BeginStep` fails a test when undone. The Phase 42 debugger
    oracles are unchanged.
- 2026-10-07: **Subtask 9 done: a stop in another process.** The author
  chose to leave the CPU in the stopped process (the design section
  above has the rule).
  - Stop messages name a process other than process 1 (`Console.
    ProcessNote`): `%VAX-I-ATTENTION`, `-INSTRLIMIT`, `-TIMELIMIT`,
    `%SYSTEM-S-HALT` (always shown for another process), and the
    debugger's "Break on fault".
  - `Console.ReturnToProcessOne` gives the CPU back to process 1 (doing
    nothing while scheduling is frozen, so a nested run stays where it
    is): the console's `Execute` and `Call` and the debugger's
    outermost `Start` (GO, STEP, CALL, RUN under the debugger) call it
    first, and the debugger's EXIT/QUIT after ending the session.
  - It switches with `System.SwitchCPU`: the context switch, then
    `sched.Choose` (new): the process the CPU held goes back to its
    queue with its priority and the rest of its quantum, and process 1
    becomes current; if process 1 is waiting it stays waiting, with no
    current process and a reschedule requested, so the next run's first
    boundary decides who runs (the CPU holds process 1 meanwhile, for
    the console's commands).
  - The author asked for a debugger `SET PROCESS [pid]` for later
    (recorded under "Future work").
  - Tests (`console/schedrun_test.go`): short limited runs until one
    stops in process 2; the message names it, the CPU's P0 and the
    debugger's EXAMINE show process 2's count, `ReturnToProcessOne`
    restores process 1's registers with the scheduler in step, and both
    go on counting; under the debugger, the session opens in process 2
    and EXIT returns to process 1; with process 1 hibernating, the CPU
    goes back to it, it stays in HIB, and process 2 runs on. `sched`:
    `Choose`.
- 2026-10-07: **Subtask 10 done: SHOW SYSTEM and SHOW PROCESS** (console
  commands; `corevms/showsys.go` builds the reports, `console/showsys.go`
  finds the process and prints).
  - SHOW SYSTEM's layout is real VMS's, from the author's VMS 7.1 SIMH
    system's output (in place of the probe Decision 7 planned): title,
    the column headings verbatim, and each process as `%08X %-15s
    %-6s%4d%9d<CPU as a delta time>%10d%7d`. `TestSystemLayout` lays
    out the sample's own values and gets its lines exactly.
  - The title's words are the author's choice: `GOVAX <govax's version>`
    (console.BuildVersion) where VMS says `OpenVMS V7.1`, and the node is
    the host's short name (`os.Hostname` up to the first dot), so it
    names the system really being looked at. `$GETSYI`'s node name stays
    GOVAX. **Unconfirmed:** padding of a node name under six characters
    (to six), and the uptime's day count past 9.
  - The state column is the scheduler's (a resource wait by its name,
    RWMBX, ...), or CUR/COM without the scheduler; priority is the
    current one. I/O and page faults are 0 (govax doesn't count them);
    Pages is the valid pages in the process's P0 and P1. A stopped
    process isn't listed.
  - CPU time: each process is now charged the emulated time that passes
    while it holds the CPU (`accountTime`, at each scheduling call and
    before a console switch; idle time is nobody's), `System.CPUTime`
    (process 1 without the scheduler: the time since boot). `$GETJPI`'s
    `JPI$_CPUTIM` (10 ms units) is new, from it.
  - SHOW PROCESS [name] [/IDENTIFICATION=pid] (the console's process by
    default; a name in its UIC group; the pid in hex, winning over a
    name): the date, user, PID, node (the host's), name, terminal, UIC
    (numeric, `[1,4]`), base priority, default directory. The layout is
    from memory of VMS's (**unconfirmed**; a VMS sample would settle
    it). A missing process is `%SYSTEM-W-NONEXPR`, a bad PID
    `%SYSTEM-F-IVIDENT`: both added to `vmserrors` (values and texts from
    `vmsdef`; the System Messages manual at hand lacks them).
  - The debugger has no SHOW PROCESS of its own yet: it goes with the
    future SET PROCESS.
  - Grammar (`show_system`, `show_process`; a grammar `disallow` works
    only between qualifiers, so a name and /IDENTIFICATION aren't
    exclusive), HELP SHOW SYSTEM and SHOW PROCESS, and SHOW's index.
  - Tests: the layout against the sample; the report's states and the
    stopped process left out; CPU time and `JPI$_CPUTIM`; through the
    console's grammar, SHOW SYSTEM with a running and a hibernating
    process (title by pattern), SHOW PROCESS by default, name, and
    /IDENTIFICATION, and NONEXPR.
- 2026-10-07: **Subtask 11 done: determinism** (`console/schedtrace_test.go`).
  A three-process workload, booted with the scheduler on in quantum-
  clock mode (forced, whatever the user's settings): processes 1 and 2
  hand the CPU back and forth 25 times through `$WAKE`/`$HIBER`, with
  some busy work each round, while process 3 waits 20 times on a 5 ms
  `$SETIMR`. Run twice at quantum 7, the instruction-by-instruction
  record of which process ran is identical; run at quanta 3, 50, and
  1000, the processes interleave differently (3 and 1000 are checked to
  differ) but the counts agree (25, 25, 20).
- 2026-10-07: **Subtask 12: close-out.** Status set to done; PLAN.md's
  index, CLAUDE.md (`internal/sched`, the engine's hook and idle, the
  corevms scheduler pieces), HELP CONFIG KEYS's processes paragraph (the
  scheduler now shares the CPU; process creation comes in Phase 45), and
  PERFORMANCE.md updated. Left for later phases, as planned: `$CREPRC`
  and deletion (a switch away from a deleted current process,
  `$SCHDWK`/`$CANWAK`/`$SETPRI` on other processes, Ctrl-C ASTs to the
  terminal's owner: Phase 45), terminal reads that block the machine
  (Phase 46), and the debugger's SET PROCESS (Future work above).
