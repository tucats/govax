# Phase 43 — Multiprocessing, part 1: processes as objects

**Status:** done (2026-10-07); decisions taken 2026-10-06.

Phase 43 is the first of six phases (43–48) that let govax run several VMS
processes at once on one engine. This document has two parts:

- **Part A, the program.** The goal of all six phases, the architecture they
  share, the rules every commit follows, the milestone that ends the program,
  the decisions to make before starting, and the bugs found while planning.
  The later phase documents refer back to it rather than repeat it.
- **Part B, Phase 43 itself.** The groundwork: process state separated from
  system state, a process table, the hardware process context (the PCB and
  the LDPCTX/SVPCTX instructions), and one address space per process. At the
  end of Phase 43, govax can build a second process and switch the CPU into
  and out of it by hand, from a Go test, but nothing schedules it yet.

---

## Part A — The program (Phases 43–48)

## The program's goal

govax runs one VMS process today. Every image the console runs, every
system service, and every RMS file belongs to that one process. The goal is
**several processes running at once on the same engine**, as on a VMS
system:

- A process can create one or more **subprocesses**, by VMS's own
  mechanisms: `SYS$CREPRC`, and `LIB$SPAWN`.
- Each process has its **own P0 and P1 address space** (its program and its
  stacks), and they all share **S0**, the system space.
- Processes **communicate**: by mailboxes, common event flags, wakeups, and
  **shared memory** (global sections).
- A **scheduler** decides which process runs. It knows when a process can
  run (is *computable*) and when it is waiting, honors **priorities**, and
  switches processes **involuntarily**, at any instruction boundary, when a
  process's **quantum** runs out or a higher-priority process becomes
  computable.
- **RMS honors file sharing and locking**, as VMS's XQP and lock manager
  do, so two processes using one Files-11 (ODS-2) volume don't corrupt it,
  and two processes using one file get VMS's sharing rules.

### The milestone (when the program is done)

The program is complete when **a MACRO-32 program runs, spawns a
subprocess, and passes messages between the two processes through
mailboxes, and both processes read and write a file on an ODS-2 volume
without corrupting the volume.** Concretely (Phase 48 builds it):

- `testdata/mp/parent.mar` and `testdata/mp/child.mar`, assembled and
  linked by govax's own MACRO and LINK.
- The parent creates a mailbox, starts the child (once with `$CREPRC`, once
  with `LIB$SPAWN`), and they exchange a fixed series of messages in both
  directions.
- Both processes append records to **one shared file** (opened with write
  sharing) and each writes a file of its own, on a mounted ODS-2 container.
- The parent learns of the child's end from its **termination mailbox**
  message and checks the child's exit status.
- A Go acceptance test runs it several times with different process
  quanta (so the two processes interleave at different instructions), and
  checks the messages, the files' contents, and the volume's structure
  (ods2's volume analysis: no lost or doubly allocated blocks, headers and
  directories consistent).

## How VMS does it, briefly

For a reader new to VMS, the pieces this program reproduces:

- **A process** is a running program plus its context: registers, address
  space, open channels and files, event flags, pending ASTs, quotas,
  privileges. VMS describes each with a *software PCB* (process control
  block; identity, state, priority, queues) and a *hardware PCB* (the
  96-byte block of registers the CPU saves and loads).
- **The address space.** A VAX virtual address's top two bits pick a
  region. P0 (0x00000000–0x3FFFFFFF: program and heap) and P1
  (0x40000000–0x7FFFFFFF: stacks and per-process system data) are
  per-process: each process has its own P0 and P1 page tables, located by
  the P0BR/P0LR and P1BR/P1LR registers. S0 (0x80000000 up) is the system's
  and the same in every process. A context switch loads another process's
  base and length registers, so the same P0 address means different memory
  in different processes.
- **Process states.** A process is *current* (running on the CPU),
  *computable* (ready to run, waiting only for the CPU), or *waiting* — for
  a local event flag (LEF), a common event flag (CEF), a wakeup (HIB),
  resumption (SUSP), or a resource (MWAIT, such as RWMBX: room in a full
  mailbox). VMS's `SHOW SYSTEM` shows these names; govax already has their
  codes (`SCH$C_*` in `internal/vmsdef`).
- **Scheduling.** 32 priorities: 0–15 "normal", 16–31 "real-time". The
  highest-priority computable process runs. Among equals, round robin: a
  process that uses up its quantum goes to the back of its priority's
  queue. A normal process's current priority is boosted when an event it
  waited for happens (I/O completion, a wakeup), and decays by one at each
  quantum end, never below its base priority. Real-time processes aren't
  boosted, and aren't preempted by quantum end.
- **The process-structure instructions.** `SVPCTX` saves the current
  process's registers into its hardware PCB, and `LDPCTX` loads another's
  (including its P0/P1 base and length registers) and invalidates the
  per-process translation-buffer entries. The `PCBB` register holds the
  hardware PCB's physical address. VMS's scheduler runs as a software
  interrupt at IPL 3, so a process is never switched while the CPU runs at
  IPL 3 or above, or on the interrupt stack. (VAX Architecture Reference
  Manual, chapter 6, "Process Structure".)
- **Subprocesses.** `$CREPRC` creates a process that runs a given image
  with given SYS$INPUT/SYS$OUTPUT/SYS$ERROR, name, priority, quotas, and
  privileges, and optionally a *termination mailbox* that receives an
  accounting message when it ends. A subprocess belongs to its creator's
  *job*: they share the job's quotas and its job logical-name table, and
  deleting a process deletes its subprocesses. `LIB$SPAWN` creates a
  subprocess running the command language interpreter (DCL), gives it the
  parent's symbols and logical names, runs one command, and (by default)
  waits for it to finish.
- **Files.** RMS arbitrates access to each file by the FAB's access
  (`FAB$B_FAC`) and sharing (`FAB$B_SHR`) fields; an open that conflicts
  with another accessor fails with RMS$_FLK. Underneath, VMS's file system
  (the XQP) and RMS take locks through the lock manager, so that each
  file's header, end of file, and blocks, and each shared record, have one
  owner at a time.

## Where govax stands (what exists to build on)

The survey for this plan found:

- **One process, by construction.** `corevms.Environment`
  (`internal/corevms/environment.go`) is "one VAX process worth of RTL
  state", built by the console on INIT/VMINIT/ZERO. Its `Process` record
  (`process.go`) holds identity, quotas, privileges, event flags, the AST
  queue, and exit handlers; its doc comment says "there is no process
  table, no PCB/JIB split, and no scheduling state". Services that take
  another process's PID or name (`$WAKE`, `$FORCEX`, `$DELPRC`, `$SETPRI`,
  `$GETJPI`) find only the caller (`processTarget`, `getjpi.go`), and
  answer SS$_NONEXPR otherwise.
- **System state mixed in.** The same Environment also holds system-wide
  state: common event flag clusters, mailboxes, the operator (OPCOM)
  state, the boot time and node name. Device, logical-name, and mount
  tables are injected from the console.
- **Waiting by retrying.** A service that must wait (`$HIBER`, `$WAITFR`,
  `$QIOW`, a full-mailbox write) returns `corevms.ErrWait`; the console
  turns it into `cpu.ErrServiceWait`, and the engine leaves the PC on the
  XFC, so the service runs again at the next step (`internal/cpu/xfc.go`).
  Interrupts and ASTs are delivered in between. This is the hook a
  scheduler needs: "this process can't go on; run someone else."
- **ASTs in Go.** The AST queues are Go state; the engine asks the RTL at
  each instruction boundary whether one is due (`cpu.ASTSource`), so a
  context switch only has to change which process the engine asks about.
- **Mailboxes exist** (`mailbox.go`, `mbxdriver.go`, Phase 26 subtask 29):
  `$CREMBX`, `$DELMBX`, reads and writes that wait, IO$M_NOW, attention
  ASTs, resource wait. But "a mailbox connects the process with itself".
- **One address space.** VMINIT (`internal/console/vminit.go`) builds one
  set of P0 and P1 page tables, in S0, and the privileged stacks in S0
  (`docs/MODE-STACKS.md`). S0 is mapped one to one onto physical memory
  (SBR is 0, S0 page *n* is physical page *n*). P0 and P1 pages are
  demand-zero: `internal/vm` gives a page a physical frame on first touch.
  The only fixed P1 contents are the user stack (top at 0x7FE00000) and the
  system-service vector (the "P1 vector", 0x7FFEDE00–0x7FFEE8FF), which
  `kernel.asm`'s `.P1VECTOR` writes at boot.
- **LDPCTX and SVPCTX are not implemented** (Phase 35, Decision 1), and the
  TB has only full invalidation (`vm.Memory.InvalidateTB`).
- **Images load into P0** at the process's region high-water mark
  (`Environment.RegionSize`), shareable images included, by the console's
  image activator (`internal/console/image.go`, `run.go`), which keeps the
  image list (`Console.ICBList`) and image symbols (`Console.Symbols`) on
  the console.
- **The console owns the run.** RUN builds a small IMAGE$INIT driver
  procedure and calls it through `Engine.CallEntry`, whose frame has a
  sentinel return address; the run ends when Step returns
  `ErrConsoleCallReturned` (`internal/cpu/call.go`, `exit.go`). The run
  loops are `Console.runPlain` and the debugger's `runLoop`
  (`internal/debugger/runcontrol.go`).
- **The clock and quantum.** `Engine.Step` counts instructions into
  emulated milliseconds in quantum mode (`interrupt.go`'s `tickQuantum`;
  `vax.quantum` and `SET QUANTUM`), or polls the host clock
  (`vax.hardware.clock`). Timers (`$SETIMR`, `$SCHDWK`) are per-Environment
  queues expired when a waiting service retries.
- **RMS.** Each Environment has its own RMS file table
  (`rms.FileTable`); mounted volumes (`rms.MountTable`) are shared. Each
  open file is an ods2 `volume.File` with **its own copy** of the file
  header and extents (`ods2/volume/file.go`); the storage and index
  bitmaps are cached once per volume. RMS supports sequential files, and
  ignores `FAB$B_SHR`. There is no lock manager.

## Architecture

### One machine, many processes

```text
                 one cpu.Engine, one vax.CPU, one vm.Memory
   ┌────────────────────────────────────────────────────────────┐
   │ S0 (shared): microkernel, SCB, shim page, page tables,      │
   │   per-process stacks and hardware PCBs, global section and  │
   │   mailbox state lives in Go                                 │
   ├──────────────────────┬──────────────────────┬──────────────┤
   │ process 1 (console)  │ process 2            │ process n    │
   │  P0: image, heap     │  P0: image, heap     │  ...         │
   │  P1: user stack,     │  P1: user stack,     │              │
   │      P1 vector*      │      P1 vector*      │              │
   └──────────────────────┴──────────────────────┴──────────────┘
     * the P1 vector's physical pages are shared, mapped into each P1
```

- **System state** (one per machine) moves into a new `corevms.System`:
  the process table and scheduler, device table, mounted volumes,
  mailboxes, common event flag clusters, global sections, the lock manager,
  OPCOM, the system and group logical-name tables, the clock, boot time,
  and node name, and the service and shim registries.
- **Process state** stays in `corevms.Environment`, which becomes "one
  process's view of the system": its `Process` record, channels, RMS file
  table, default directory, heap, timers, AST queue, image list, and its
  process (and, through its job, job) logical-name tables. Each
  Environment points to the System.
- **Job state** (one per process tree): quotas shared by a process and its
  subprocesses (`PRCLM`, and the others as they're needed) and the job
  logical-name table.

### The hardware context: real PCBs, switched by Go

Each process has an architected **hardware PCB** — the 96-byte layout of
the VAX Architecture Reference Manual (KSP, ESP, SSP, USP, R0–R11, AP, FP,
PC, PSL, P0BR, P0LR with ASTLVL, P1BR, P1LR with PME) — in S0 memory, and
`PCBB` holds the current process's. **The scheduler is Go code**, as the
system services are: at a reschedule point, between two instructions, it
does exactly what an IPL 3 rescheduling interrupt running `SVPCTX` …
`LDPCTX` … `REI` would do, through the same Go functions that implement
those two instructions (which Phase 43 adds). So the PCBs in memory are
always right, a program (or the microkernel) could run its own `LDPCTX`,
and the console and debugger can show a process's saved registers.

Go-side, a switch also changes which Environment is "current": the one
the engine's system-service, AST, and attention hooks reach.

### The scheduler

A new leaf package, `internal/sched`, holds VMS's scheduling rules as
plain Go, testable without a CPU: the states, one queue per priority, the
choice of the next process, quantum accounting, boosts and decay, and the
preemption test. `internal/cpu` gets a hook (an interface, like
`cpu.ASTSource`) that `Engine.Step` calls at instruction boundaries:

- **Quantum end.** Each process's quantum is a count of instructions
  (Decision 2), so scheduling is deterministic in quantum-clock mode: the
  same run interleaves the same way every time, and tests can rely on it.
- **Waiting.** When a service returns `ErrServiceWait`, the process goes
  into a wait state (LEF, CEF, HIB, MWAIT, ...) with a Go predicate that
  says when it can go on; the scheduler runs someone else. When the
  process is dispatched again it re-executes the XFC — the existing retry
  model, unchanged. With one process, it is the only one computable, so it
  retries at once: exactly today's behavior.
- **Preemption.** Only when the CPU is at IPL < 3 and not on the interrupt
  stack, in any access mode (Decision 3). A service in Go always completes
  before the next boundary, so services are atomic with respect to
  scheduling, as VMS's are at IPL$_SYNCH.
- **Idle.** When no process is computable, the engine idles (the "null
  process"): it advances emulated time to the next timer due, so `$SETIMR`
  and `$SCHDWK` still end waits deterministically.

### Process 1 is the console's process

The process the console runs images in today becomes **process 1**, with
**exactly today's layout**: VMINIT's page tables, stacks, and PID
(0x00000301). A new process gets freshly allocated page tables, stacks,
and PCB. So with no subprocess created, every address, every listing, and
every oracle comparison stays as it is.

The console runs process 1's work in the foreground: RUN, GO, CALL, and
the debugger start and stop process 1. Other processes run whenever the
engine runs, and are frozen while the console waits at `VAX>` or `DBG>`
(Decision 4). A run ends when process 1's image ends (or HALT, Ctrl-C, a
breakpoint, a limit), not when another process's does: a subprocess's
image exit becomes that process's deletion.

### Touching another process's memory

Some work done while process A is current is on behalf of process B: a
mailbox write completing B's waiting read writes the message into B's
buffer and B's IOSB. VMS does this with buffered I/O and a kernel AST in
B's context. govax does it more simply: `internal/vm` gets an
**address-space** value (a process's P0BR/P0LR/P1BR/P1LR) and load/store
methods that translate through it rather than through the CPU's current
registers. P0 and P1 page tables are in S0, which every process maps, so
another process's PTEs are always reachable. Event flags, AST queues, and
wakeups are Go state and need nothing special.

### Files

Phase 47 adds a small lock manager (`internal/lck`, a leaf package:
resource names, the six VMS lock modes and their compatibility, waiting
requests that integrate with the scheduler, locks released at process
rundown), and builds on it: file access arbitration by `FAB$B_FAC`/
`FAB$B_SHR` (RMS$_FLK), one shared in-memory file control block per open
file (so two accessors see one header, one end of file, one extent map —
an ods2 change), shared sequential-file semantics, and the record locks
RMS takes for write-shared files. `$ENQ`/`$DEQ` are exposed as services on
the same manager (Decision 10).

## Rules for every commit

1. **Nothing breaks.** Each subtask is committed when it's complete and
   tested, and every commit passes `go build ./...`, `go vet ./...`, the
   linter, and `go test ./...`. Existing oracle tests (MACRO, LINK,
   ANALYZE, debugger, RMS, instruction set) keep passing unchanged.
2. **A feature flag** guards behavior that is new and not yet complete:
   `vax.process.scheduler` (bool, default **false** until Phase 48;
   Decision 5). With it false the engine never switches processes and
   `$CREPRC`/`LIB$SPAWN` fail as unsupported (SS$_UNSUPPORTED, or the
   status the manual names for a missing feature); with it true, they work.
   Tests turn it on explicitly. Other keys: `vax.process.quantum` (int,
   instructions per quantum), `vax.process.preempt` (`all`, `user`, or
   `none`; which modes may be preempted). Every key is added to
   `cmd/govax/main.go`'s `validConfigs` and `HELP CONFIG KEYS`.
3. **Layout preserved.** Process 1 keeps VMINIT's layout (above). Anything
   that would move an address a test or oracle sees is a decision for the
   user, not a side effect.
4. **Deterministic.** In quantum-clock mode a multi-process run depends on
   nothing but its inputs. Multi-process tests run in that mode.
5. **Commented for learners.** All new and changed Go code is commented
   for a reader who knows neither VAX nor VMS: what a PCB is, why the TB
   is flushed, what a CEF wait is, why a lock mode is compatible with
   another. (The project's comment style; see the existing `internal/vm`
   and `corevms` comments.)
6. **Bugs are in scope.** A bug found in existing code is fixed in the
   same phase, with a regression test, and logged in that phase's
   progress log (and `docs/DEVIATIONS.md` if it's an ISA or VMS fidelity
   issue).
7. **Clean room.** Behavior comes from DIGITAL's manuals (System Services
   Reference, RMS Reference, LIB$ Reference, the VAX Architecture
   Reference Manual, the User's Manual), from real VMS output (probes on
   the user's VMS 7.3 system), and from the book *VAX/VMS Internals and
   Data Structures*, which Decision 6 allows in full, code examples and
   listings included. Never from VMS source listings. A rule no source settles is
   chosen sensibly and logged as unconfirmed in the phase's doc.
8. **Phase docs.** Each phase extends its own progress log as it goes, and
   records discoveries (this is a large program; the subtask lists below
   will change).

## The phases

| Phase | Doc | Summary |
| --- | --- | --- |
| 43 | this doc, Part B | Processes as objects: system/process state split, process table, hardware PCB with LDPCTX/SVPCTX, per-process address spaces and stacks, cross-address-space access. One process still runs; a Go test switches into a second by hand |
| 44 | [PHASE-44 - scheduler](PHASE-44%20-%20scheduler.md) | The scheduler: `internal/sched`, the engine's reschedule hook, quantum and priority preemption, wait states from waiting services, idle and timers, process-aware run loops and debugger, SHOW SYSTEM |
| 45 | [PHASE-45 - create and delete process](PHASE-45%20-%20create%20and%20delete%20process.md) | Creating and deleting processes: `$CREPRC`, process startup and image activation in the new process, rundown and deletion, termination mailbox, jobs and logical-name tables, the other process-control services across processes, `$GETJPI` wildcards, NL: |
| 46 | [PHASE-46 - interprocess comm](PHASE-46%20-%20interprocess%20comm.md) | Interprocess communication: mailboxes between processes, common event flags, global sections (shared memory), RMS on mailbox and null devices, terminal reads that don't block the machine |
| 47 | [PHASE-47 - RMS and processes](PHASE-47%20-%20RMS%20and%20processes.md) | Files shared between processes: the lock manager and `$ENQ`/`$DEQ`, file access arbitration (RMS$_FLK), a shared file control block in ods2, shared sequential files, record locks, volume-integrity stress tests |
| 48 | [PHASE-48 - LIB_SPAWN](PHASE-48%20-%20LIB_SPAWN.md) | `LIB$SPAWN` and the milestone: the subprocess CLI, the parent/child MACRO programs, the acceptance tests, the scheduler flag on by default, documentation |

Each phase leaves govax working and its new pieces tested. 44 needs 43;
45 needs 44; 46 and 47 need 45 and are independent of each other; 48
needs all of them.

## Decisions

**Decided 2026-10-06:** the author took every recommendation below; each
"*Decided:*" is the option recommended at planning time. The author will
also run VMS 7.3 probes as the phases need them (Decision 7), and ods2 may
be changed in parallel with govax as needed (Decision 9). As the work
proceeds, a reason to change any of these is raised with the author and
recorded here.

1. **Where the scheduler lives.** *Decided:* in Go (`internal/sched`
   plus the engine hook), switching contexts with the same code as
   `SVPCTX`/`LDPCTX`, as govax's system services are Go. The alternative,
   a VMS-style scheduler written in VAX code in `kernel.asm` driven by an
   IPL 3 software interrupt, is more faithful but would have to call back
   into Go for every service's wait anyway, and would be far slower to
   write and debug.
2. **The quantum's unit.** *Decided:* instructions executed by the
   process (`vax.process.quantum`; a starting default of 20,000, tuned in
   Phase 44 so that switching costs little but processes visibly
   interleave), deterministic in both clock modes. The alternative is VMS's
   QUANTUM in 10 ms units of emulated time: deterministic in quantum-clock
   mode, but wall-clock driven (so not reproducible) with
   `vax.hardware.clock`, and sensitive to `SET QUANTUM` (which `vax.init`
   sets to 1).
3. **Which modes can be preempted.** *Decided:* VMS's rule — any
   access mode, when IPL < 3 and not on the interrupt stack — with
   `vax.process.preempt=user` to limit it to user mode while debugging the
   scheduler, and `none` for cooperative switching (waits only).
4. **Other processes while the console waits at `VAX>`/`DBG>`.**
   *Decided:* frozen. They run whenever the engine runs (a RUN, GO,
   CALL, or debugger GO/STEP), and a process left running when its parent's
   image ends waits for the next run. Running them in the background while
   the console reads commands would need the engine on its own goroutine,
   with locking around everything the console touches; possible later.
5. **The scheduler flag's default.** *Decided:* `false` until Phase
   48's milestone passes, then `true`, keeping the key so it can be turned
   off. Alternatively it could stay `false` permanently (multiprocessing
   opt-in).
6. **Reference books.** May the clean room use DIGITAL's published book
   *VAX/VMS Internals and Data Structures* (in the manuals folder,
   `VAX:VMS Internals and Datastructures.pdf`) for concepts such as the
   boost values, state transitions, and the shape of `$CREPRC`'s process
   startup? *Decided:* yes, and without restriction. At planning time the
   recommendation was descriptions only, never its code excerpts; the
   author revised that on 2026-10-06, after reviewing the book's rights
   notices and its purpose. The book was published as a text for learning
   how VMS works, is widely available as one, and is meant to be used the
   way a programmer writing this code by hand would use it. So it is
   reference material in every way, for this program (Phases 43–48) and
   later phases: its descriptions, tables, figures, data-structure
   layouts, and its example code and listings may be read, followed, and
   transcribed where they help. Cite the book (chapter or section) in a
   comment or the phase doc where something comes from it, as for the
   manuals. This applies to this one book only: the *VMS Internals I ...
   Listings* course book and the VMS source archive stay off-limits, as
   listings.
7. **VMS 7.3 probes.** Will the author run probe programs on the VMS 7.3
   system, as for earlier phases? *Decided:* yes, for the few things only real
   output settles: the termination message's contents, `$GETJPI` across
   processes, `SHOW SYSTEM`/`SHOW PROCESS` layouts, RMS's sharing statuses,
   and `LIB$SPAWN`'s behavior. Each phase names its probes; none blocks
   progress (an unprobed rule is logged as unconfirmed).
8. **`LIB$SPAWN`'s command language.** A spawned subprocess runs DCL, but
   govax's DCL is the console, in Go. *Decided:* the subprocess runs a
   small Go "subprocess CLI" that executes the commands that run images —
   `RUN`, foreign commands (DCL symbols), and `MCR` if wanted — with
   others rejected as an unsupported command; extended later if needed.
   The alternative, the whole console command set inside a subprocess, needs
   the console made process-aware throughout (its output, its defaults,
   its tables).
9. **Changing ods2.** The shared file control block (Phase 47) needs new
   ods2 API, tagged and pinned as in Phase 34. *Decided:* yes, and ods2
   may be changed in parallel with govax whenever a phase needs it.
10. **`$ENQ`/`$DEQ` as services.** *Decided:* yes; they're a thin
    layer on the lock manager RMS needs anyway, and the usual VMS way for
    processes sharing memory to synchronize.
11. **The debugger with several processes.** *Decided:* the debugger
    debugs process 1; breakpoints belong to process 1 (a breakpoint
    address in P0 means process 1's P0); stepping freezes the other
    processes; a subprocess's image never starts a debugger of its own
    (VMS would start one if the image was linked `/DEBUG`). A later phase
    could add debugging a subprocess: the author has asked for a debugger
    `SET PROCESS [pid]` to switch to another process (PHASE-44.md,
    "Future work").
12. **The milestone's two creation paths.** *Decided:* the milestone
    runs twice, child created by `$CREPRC` (possible from Phase 45/46) and
    by `LIB$SPAWN` (Phase 48).

## Known bugs and hazards found while planning

To fix in the phase named (each with a regression test):

1. **The IMAGE$INIT driver is in a shared page** (Phase 43). RUN writes
   its driver at `CONSOLE$SCRATCH+8`, one S0 page. A second image activated
   while the first's driver is still on the call stack (a subprocess's, or
   a nested RUN) overwrites code the first will return into (its
   `PUSHL R0`/`CALLS SYS$EXIT`/`RET` tail).
2. **`NewEnvironment` deletes every mailbox** (Phase 43):
   `removeStaleMailboxes` runs whenever an Environment is built, which
   will be once per process.
3. **Every process would have PID 0x301** (`nominalPID`), and
   `processTarget` can only find the caller (Phase 43/45).
4. **`LNM$TEMPORARY_MAILBOX` translates to `LNM$PROCESS`**, not
   `LNM$JOB` as on VMS (`internal/lnm/database.go`); a subprocess wouldn't
   see its parent's temporary mailbox names (Phase 45). Fixed in Phase
   45, subtask 3.
5. **Image symbols are console-global** (`SHARE$xxx_INITIALIZE`, `MAIN`
   in `Console.Symbols`); a second process's activation would overwrite
   process 1's (Phase 43).
6. **Terminal reads block the whole machine.** A process reading the
   terminal blocks the engine's goroutine in a host read, so no other
   process runs until the user types (Phase 46).
7. **The legacy CHF path nests a run loop** (`internal/console/chf.go`
   calls `Console.Call` to run a condition handler). A nested loop must not
   switch processes, or the outer loop's state is wrong (Phase 44).
8. **Two writers of one file corrupt it.** Each ods2 `volume.File` has
   its own copy of the header and extent map; two processes writing one
   file would each write back their own header, losing the other's blocks
   or end of file (Phase 47).
9. **Breakpoints are bare virtual addresses** (Phase 44, Decision 11).
10. **`MountTable`'s comment** says there will never be more than one
    process; several shared tables say the same (Phase 43 updates them as
    the state moves).

---

## Part B — Phase 43: processes as objects

## Phase 43's goal

Make processes real objects without changing what runs:

- system state and process state separated (`corevms.System` and
  per-process `Environment`s);
- a process table, with real, distinct PIDs;
- a hardware PCB per process, and the `LDPCTX`/`SVPCTX` instructions;
- per-process address spaces (P0/P1 page tables) and privileged stacks,
  allocated from S0, with the P1 vector shared;
- Go access to another process's memory;
- per-process image state on the console.

At the end, process 1 runs exactly as today, and a Go test can build
process 2, switch into it with `LDPCTX` semantics, run code there, and
switch back, showing that P0/P1 are separate and S0 is shared.

## State inventory

Subtask 1 completed and checked this table against the code (2026-10-07);
it's the map for subtask 2 and later phases. "Phase n" in the last column
says when a piece that isn't moved in Phase 43 moves.

### `corevms.Environment` (`internal/corevms/environment.go`)

| Field | Belongs to | Notes |
| --- | --- | --- |
| `mem`, `cpu` | machine | one `vm.Memory` and one `vax.CPU`, shared by every process |
| `shims`, `services` | system | registries, built once per System (subtask 2); `librtl.Register` adds to `shims` |
| `Devices` | system | injected by the console; owned by `Console`, survives INIT |
| `Mounts` | system | injected; owned by `Console`, survives INIT |
| `Mailboxes`, `EventFlagClusters`, `Operator` | system | built per INIT/VMINIT/ZERO |
| `Clock`, `BootTime`, `NodeName` | system | `Clock` rebound to `Engine.SystemTime` by `Console.newRTL` |
| `OnUnhandled`, `OnSignal` | system | debugger hooks; stay per Environment until the debugger is process-aware (Phase 44); they only ever fire for the current process |
| `Logicals` | split | one `lnm.Database` holds both directories: `ProcessDirectory` (process and job tables) and `SystemDirectory` (system and group tables). Stays a per-Environment pointer in Phase 43 (process 1 shares the console's); Phase 45 splits `lnm.Database` so a process gets its own process directory over the shared system one |
| `Session` | process | the default directory; process 1's is the console's `ContainerSession` (SET DEFAULT changes both, as today). A new process gets a copy of its creator's (Phase 45) |
| `files` (RMS IFIs and `$SEARCH` contexts) | process | |
| `channels`, `nextChannel` | process | a channel names a shared `iodev.Device` |
| `Process` | process | see below |
| `RegionSize` | process | P0/P1 high-water marks; [2] (S0) is unused by processes, and the S0 allocator (subtask 6) is the System's |
| `memAllocated`, `memFreed` | process | the LIB$GET_VM heap, in the process's P0 |
| `openFiles`, `nextFID` | process | host files of the CRTL shims |
| `consoleIn`/`consoleInBuf`, `consoleOut` | process | its SYS$INPUT/SYS$OUTPUT; the terminal under them is shared (Phase 46) |
| `CommandLine` | process | LIB$GET_FOREIGN's text |
| `timers` | process | `$SETIMR`/`$SCHDWK` requests; expired only by that Environment's `NextAST` (a hazard below) |
| `attentionASTs` | process | CTRL/C and CTRL/Y ASTs; the terminal's owner gets them (Phase 46) |
| `pendingIO`, `qiowWaits` | process | |
| `waitingPC` | process | trace bookkeeping for a waiting service |

### `corevms.Process` (`process.go`)

All per process: `PID`, `Username`, `Name`, `Account`, `Terminal`,
`CLIName`, `UIC`, the working-set fields, `ASTLimit`, `Priority`,
`BasePriority`, `AuthorizedPriority`, the four privilege masks,
`LocalEventFlags`, `CommonClusters` (associations; the clusters are the
System's), `ResourceWaitDisabled`, `WakePending`, `ast`, `exitHandlers`,
`ExitStatus`, `putmsg`, `cmode`, `conditions`, `exceptionVectors`, and the
page locks. `Username`, `Account`, and the quotas VMS keeps in the JIB
(`PRCLM` and friends, none modeled yet) move to a job record in Phase 45.

### `console.Console` (`internal/console/machine.go`)

| Field | Belongs to | Notes |
| --- | --- | --- |
| `Engine`, `CPU`, `Mem` | machine | |
| `RTL` | process | process 1's Environment, and the one the engine's hooks reach (`services.go`: `SystemService`, `Shim`, `NextAST`, `HandleAttention`, `DispatchException`). Becomes "the current process's" in Phase 44 |
| `Symbols` | system, with exceptions | console and microkernel symbols. The image activator also writes `MAIN` and `SHARE$xxx_INITIALIZE`/`_TRANSFER_n` here (`image.go`), which are process 1's (bug 5; subtask 10) |
| `ICBList`, `imageActive`, `runHost`, `runCommandLine` | process | process 1's image state (subtask 10) |
| `dclSymbols` | process | DCL symbols, a CLI's state: process 1's; `LIB$SPAWN` copies them (Phase 48) |
| `Regions`, `shimBase`, `shimsReady`, `s0Free` | system | `s0Free` is where the S0 pool starts (subtask 6) |
| `Devices`, `Logicals`, `Mounts`, `ContainerSession` | system / process 1 | see the Environment's |
| `Debugger`, `OnUnhandled`, `OnSignal`, `sourceDirs`, `sourceCache` | console | the debugger debugs process 1 (Decision 11) |
| `asmSession`, `assemblerMode`, `Radix`, `DepositAddr`, `Verbose`, `Verify`, `Trace`, `Dispatcher`, `In`, `Out`, `ScreenSize`, `Paths`, `HostLibrary`, `SharePrefix`, limits, `quit` | console | operator state |

### `internal/rms`

`rms.Context` is built per call from the Environment (`rmsContext`):
`Mem`, `CPU`, `Mounts` (system), `Files`, `Session`, `Console`,
`ConsoleIn` (process), `Logicals` (split, as above), `NodeName`
(system). `rms.Session` holds `Mounts` (system), `Logicals`, and
`Default` (process). `rms.FileTable` is per process. `MountTable` and
each mounted `ods2` volume are system state; an open file's
`volume.File` is per process today, and becomes shared in Phase 47.

### The CPU, the engine, and memory

| Where | Field | Belongs to |
| --- | --- | --- |
| `vax.CPU` | R0–R11, AP, FP, SP, PC, PSL, KSP/ESP/SSP/USP, P0BR/P0LR/P1BR/P1LR, ASTLVL | process (saved in its hardware PCB) |
| | ISP, SBR/SLR, SCBB, PCBB, SISR, MAPEN, clock, TODR, and console registers, debug flags | system (IPL is in the PSL, saved per process) |
| `cpu.Engine` | everything (the decode cache, interrupts, clock, limits, the fault history) | system; per-process CPU time is new (Phase 44). `decoded` is two scratch buffers for the instruction being run, not a cache, so a switch needn't touch it |
| `vm.Memory` | frames, the TB, the STC, the fetch window | system; P0/P1 TB entries, the STC, and the fetch window describe the current process and are flushed on a switch (subtask 5) |

### `kernel.asm` (the microkernel, in S0)

Nothing in it is laid out per process: it has no stack symbols and no
`CTL$` cells. Its data cells are system-wide, which matters for these:

- `vaxc$errno` (CRTL's `errno`), `exe$sig_buff`, and `exe$dclstring`
  are scratch cells that a routine running for one process could be
  preempted in the middle of using (Decision 3 allows preempting kernel
  mode at IPL 0). Phase 44 must either keep them per process or not
  preempt inside the microkernel's routines (`exe$base`..`exe$fend`).
- `exe$ast_list` is the legacy software-interrupt-2 AST queue, unused
  now that ASTs are Go state; system-wide.
- `exe$rxdata`/`exe$rxlen`/`exe$rxbuffer`/`exe$rxptr` buffer the console
  terminal's input: one terminal, so system-wide (Phase 46).
- The P1 vector (`.P1VECTOR`) is written once, by ASM, into process 1's
  demand-zero P1 pages 0x7FFEDE00–0x7FFEE8FF (6 pages). It holds only
  trampolines (entry mask, `XFC XFC$P1VECTOR`, `RET`) and two data cells
  nothing reads or writes (`SYS$GL_ASTRET`, `SYS$GL_COMMON`), so every
  process can map the same physical pages, read-only (subtask 8). No Go
  code has another fixed P1 address but the user stack top (`spP1`).

No package (`corevms`, `console`, `rms`, `librtl`, `lnm`, `cpu`, `vm`,
`io`, `debugger`) has a mutable package-level variable; their `var`s are
constant tables.

### VMINIT's layout and the S0 left over

With `vax.init`'s `VMINIT /P0=16384 /P1=8192 /S0=8192 /KSP=20` on
16384 pages of memory: the S0 table is 64 pages, P0's 128, P1's 64; then
the kernel stack (20), a guard and the executive stack (1+8), a guard and
the supervisor stack (1+8), the interrupt stack, CONSOLE$SCRATCH (1), the
shim page, the SCB (1), the string pool, and then the microkernel at
`s0Free`. A new process with VMINIT's sizes needs 192 page-table pages
and 38 stack pages: 230 (subtask 6 records the arithmetic).

## Cross-process hazards

Every place Go code reads or writes VAX memory, or process state, for a
process other than the one whose service call is running. Today each
uses the current Environment's `mem`/`cpu` (so the current P0/P1) or its
`Process`; each must use the owner's once there are several processes.

| Where | What it does | Fixed in |
| --- | --- | --- |
| `mbxdriver.go` `receive`, `send` | a write completing a waiting read stores the message into the *reader's* buffer, and completes the reader's or writer's request | Phase 45, subtask 7 (through the owner); Phase 46 tests every case |
| `qio.go` `completeIO` | stores the IOSB, sets the event flag, and queues the AST of a request that may be another process's (from `receive`/`send`, `cancelIO`) | Phase 45, subtask 7 (`ioRequest.owner`) |
| `mbxdriver.go` `deliverAttention`, `operator.go` `postMailboxMessage`/`operatorReplyTo` | queue attention ASTs and post messages for whichever process enabled them | `deliverAttention`: Phase 45, subtask 7 (`attentionRequest.owner`); OPCOM: Phase 46 |
| `timers.go` `expireTimers` | runs only from the owning Environment's `NextAST`, so a waiting process's timers never expire while another runs | Phase 44 (the scheduler expires every process's) |
| `ast.go` `NextAST` | pushes an AST frame on the current stack: correct only for the current process; the engine must ask the current one | Phase 44 |
| `ctrlast.go` `Attention` | CTRL/C and CTRL/Y go to `Console.RTL` | Phase 46 (the terminal's owner) |
| `hibernate.go` `$WAKE`/`$SCHDWK`, `process.go` `$FORCEX`/`$DELPRC`/`$SETPRI`, `getjpi.go` | act on the target's Go state only (`processTarget`) | Phase 45 |
| `condition.go`, `cmode.go`, `exit.go`, `unwind.go`, `message.go` | the caller's own stack and state: current process only | — |
| RMS (`internal/rms`) | synchronous, in the caller's context; only terminal reads wait | Phases 46–47 |
| console `image.go` (`imageLoad`), `run.go` (`buildImageInitDriver`) | write an image into P0 and the driver into `CONSOLE$SCRATCH` through the CPU's registers | subtask 10 |
| console EXAMINE/DEPOSIT, `expr.go`'s string pool, `shim.go` | the console's view: process 1's P0/P1, the shared S0 | Phase 44 (the debugger/console show process 1 even when another is current) |
| console `chf.go` | runs a condition handler in a nested run loop | Phase 44 (bug 7) |

## Subtasks

1. **Inventory.** Complete the table above from the code (every field of
   `Environment`, `Process`, `Console`, the console's `Session`, and
   `internal/rms`'s context), and list every place Go code reads or writes
   VAX memory on a process's behalf (the cross-process hazards for
   Phases 44–46). Record it here.
2. **`corevms.System`.** Move the system-wide fields into a new
   `System` (`internal/corevms/system.go`), built once per INIT/VMINIT/ZERO
   by the console; `NewEnvironment` takes the System and builds only
   process state. `removeStaleMailboxes` moves to System construction (bug
   2). The service and shim registries are built once per System. Every
   existing caller keeps working through accessors; no behavior change.
3. **The process table and PIDs.** `System` gets a process table: PID →
   Environment, process names (unique; `$SETPRN`'s SS$_DUPLNAM), and the
   current process. PIDs follow the shape of VMS's on a non-clustered
   system — a process index in the low bits and a sequence number above
   it — with process 1 keeping 0x00000301 (bug 3; the exact shape is
   unconfirmed unless a probe settles it). `processTarget` looks processes
   up in the table (still finding only process 1 until Phase 45 creates
   others).
4. **The hardware PCB, `SVPCTX`, and `LDPCTX`.** A Go description of the
   architected layout (`internal/cpu/context.go`, with offsets named), and
   the two instructions exactly as the Architecture Reference Manual
   specifies them: privileged (kernel mode), `LDPCTX` on the interrupt
   stack, loading the registers and memory-management registers, ASTLVL
   and PME, invalidating per-process TB entries, switching to the kernel
   stack and pushing the saved PSL and PC for `REI`; `SVPCTX` popping PC
   and PSL into the PCB and switching to the interrupt stack, raising IPL
   to at least 1. The manual's UNDEFINED cases take the preferred
   reserved-operand fault. `Engine.SaveContext`/`LoadContext` expose the
   same code to Go (Phase 44's switcher). `TestEveryInstructionImplemented`
   drops its two exceptions; tests follow the manual's operation text,
   including its RESCHED example run as a VAX program.
5. **Per-process TB invalidation.** `vm.Memory.InvalidateProcessTB`
   clears only P0 and P1 entries (and the STC and the instruction-fetch
   window), as `LDPCTX` requires; S0 entries survive a switch. Counted in
   the TB statistics; a test shows S0 entries kept and P0/P1 entries gone.
6. **An S0 page allocator.** The S0 pages past VMINIT's layout and the
   microkernel (`Console.s0Free`, advanced by the console's ASM) are a pool
   for per-process structures: page tables, stacks, hardware PCBs. A
   simple allocator (contiguous runs, freed on process deletion, in Go on
   the System) with a clear "S0 exhausted" error. Records what it hands
   out, for SHOW MEMORY later. How much S0 VMINIT leaves free decides how
   many processes fit; document the arithmetic (with `vax.init`'s sizes,
   about 230 pages per process).
7. **Address spaces.** `vm.AddressSpace` (P0BR, P0LR, P1BR, P1LR) and
   `Memory` methods that translate, load, and store through a given
   address space instead of the CPU's registers (demand-zero pages
   allocated as usual, protection checked as kernel mode). Tests: two
   address spaces mapping the same P0 address to different frames.
8. **Building and tearing down an address space.** Generalize VMINIT's
   P0/P1 page-table loops into a builder that lays out a new process's
   tables in pool pages (same sizes as VMINIT's by default; a setting if
   needed), with the bottom P0 page a no-access guard as today, and maps
   the P1 vector's physical pages into the new P1 (read-only to user mode;
   check what `.P1VECTOR`'s pages hold, including the `SYS$GL_*` data
   cells, and whether any must be per process). Teardown frees every
   demand-allocated frame, then the tables. Process 1 adopts VMINIT's
   tables as its address space.
9. **Per-process privileged stacks.** A new process gets kernel,
   executive, and supervisor stacks in pool pages with
   `docs/MODE-STACKS.md`'s layout and protections (and guard pages), and a
   user stack top at 0x7FE00000 in its own P1. Process 1 keeps VMINIT's.
   The interrupt stack stays shared. Update `MODE-STACKS.md`.
10. **Per-process image state.** Move `ICBList`, the image symbols, and
    the IMAGE$INIT driver to the process: each process's driver goes in a
    page of its own (in its P1, below the vector, or a pool page; chosen
    in the subtask) instead of `CONSOLE$SCRATCH` (bugs 1 and 5). The
    console's commands that show images (SHOW IMAGES, the debugger's
    symbol lookups) use process 1's. Image activation takes the target
    process as a parameter, writing through its address space (subtask 7)
    — Phase 45 activates images in new processes with it.
11. **Settings and help.** `vax.process.scheduler`, `vax.process.quantum`,
    `vax.process.preempt` (read but inert until Phase 44) in
    `validConfigs`, `HELP CONFIG KEYS`, and the console's settings
    plumbing.
12. **The hand-switch test.** A Go test (in `internal/console`, with the
    consoletest helpers): VMINIT and the microkernel as `vax.init` does;
    build process 2 (address space, stacks, PCB with a PC in its P0);
    write a few instructions into process 2's P0 through its address
    space; switch to it (`SaveContext`/`LoadContext`, as a scheduler
    would), run a few steps, switch back; check that process 1's P0 is
    untouched, both see the same S0 and P1 vector, and the PCBs in memory
    hold the expected registers. Also: the existing suite passes
    unchanged.
13. **Close-out.** Status, progress log, PLAN.md, CLAUDE.md package notes
    (`corevms.System`, `vm.AddressSpace`, `cpu/context.go`), and help.

## Open questions (Phase 43)

- Should the hardware PCB live next to each process's kernel stack (as
  VMS's process header does) or in its own small pool? Either; decided in
  subtask 4/9. *Answered (subtask 9):* a pool page of its own,
  kernel-only.
- Does anything in `kernel.asm` assume one process (fixed stack symbols,
  `CTL$`-style cells)? Subtask 1 checks. *Answered (subtask 1):* no
  per-process layout, but three scratch cells a preempted microkernel
  routine could leave half-used (Phase 44).
- Page-table sizes for subprocesses: VMINIT's (16384 P0 and 8192 P1 pages
  in `vax.init`) cost about 192 pool pages each. If that limits the
  process count, a subprocess could get smaller tables (VMS sizes them by
  quotas); decided in subtask 8. *Answered (subtasks 8 and 12):* the
  builder takes the sizes; Phase 45 chooses what `$CREPRC` passes. P1
  needs at least `corevms.MinP1Pages` (4097) for the user stack, so P1's
  table costs at least 33 pages; with VMINIT's sizes, about 34
  processes fit beside process 1.

## Progress log

- 2026-10-06: Planned (this document and Phases 44–48).
- 2026-10-06: The author took every recommended decision (Part A,
  "Decisions"), agreed to run VMS 7.3 probes as needed, and allowed ods2
  changes in parallel. The plan is under the author's review before
  implementation starts.
- 2026-10-07: Subtask 1 (inventory). Completed the state inventory from
  the code and added the cross-process hazards list and the kernel.asm
  and P1-vector findings (above). Answers to the open question about
  `kernel.asm`: it has no per-process layout, but `vaxc$errno`,
  `exe$sig_buff`, and `exe$dclstring` are shared scratch cells a
  preempted microkernel routine could leave half-used (for Phase 44).
  The P1 vector holds nothing per process. `Logicals` stays per
  Environment until Phase 45 splits `lnm.Database`.
- 2026-10-07: Subtask 2 (`corevms.System`). `internal/corevms/system.go`
  holds the machine (`mem`, `cpu`), the shim and service registries,
  `Devices`, `Mounts`, `Mailboxes`, `EventFlagClusters`, `Operator`,
  `Clock`, `BootTime`, and `NodeName`, built by `NewSystem`. Environment
  embeds `*System`, so every existing `env.Mailboxes`, `env.mem`, ...
  reads the System's field through Go's field promotion, with no other
  code changed; `env.System` names it. `NewEnvironment(sys, logicals,
  in, out)` builds only process state. The console's `newRTL` builds a
  System and then process 1's Environment; `librtl.Register` now takes
  `sys.Shims()`, once per System. Bug 2 fixed: `removeStaleMailboxes`
  runs in `NewSystem`, and `TestMailbox_newEnvironmentKeepsMailboxes`
  checks that a second Environment on one System keeps the first's
  mailbox. `OnUnhandled`/`OnSignal` stayed on the Environment (they fire
  for the current process; the debugger watches process 1). The
  `MountTable` and `Process` comments that said govax has one process
  are updated (bug 10, in part; the service comments that still describe
  one-process behavior change with that behavior, in Phases 45 and 46).
- 2026-10-07: Subtask 3 (process table and PIDs). `proctable.go`: the
  System's `processTable` holds each process's Environment by index
  (1–255; 0 is the null process's), a sequence number per slot, and the
  current process. A PID is the slot's sequence number above an 8-bit
  index; a new process takes the lowest free index and the slot's next
  sequence number, and `FindProcess` checks the whole PID against the
  slot's process, so a deleted process's PID finds nothing after its
  slot is reused. This is the scheme of *VAX/VMS Internals and Data
  Structures* (V3 edition), 20.1.3 "The PCB Vector" and 20.1.4
  "Fabrication of Process IDs", figure 20-4 (index in the low word,
  sequence in the high word there); the 8-bit index, the 13-bit sequence
  (an extended PID's 21-bit process field), and every slot starting at
  sequence 2 (so process 1 is 00000301, as before, and process 2 is
  00000302) are govax's choices, **unconfirmed**. A VMS 7.3 `SHOW SYSTEM`
  would show whether fresh processes' PIDs share one sequence part as
  these do. `NewEnvironment` now adds its process to the table and
  returns an error (SS$_NOSLOT when all 255 slots are taken); the first
  process is the current one. `RemoveProcess`, `Processes`,
  `FindProcessName`, `Current`, and `SetCurrent` complete the API.
  Process names are unique within a UIC group (System Services
  Reference: $SETPRN's and $CREPRC's SS$_DUPLNAM, $CANWAK's prcnam):
  `$SETPRN` now returns SS$_DUPLNAM for a name another process in the
  group has, and a new process whose default name is taken gets none.
  `processTarget` (bug 3) finds any process in the table, by PID, or by
  name in the caller's group, writing the found PID back, and returns
  the target; `callerTarget` wraps it for today's services, which still
  answer SS$_NONEXPR for any process but the caller until Phase 45 lets
  them act on another. Tests: `proctable_test.go` (PIDs, reuse,
  sequence wrap, a full table, names by group, processTarget, $SETPRN's
  SS$_DUPLNAM).
- 2026-10-07: Subtask 4 (the hardware PCB, `SVPCTX`, and `LDPCTX`).
  `internal/cpu/context.go`: the 96-byte PCB's offsets (`PCBKSP` ...
  `PCBP1LRPM`, `PCBSize`), a `PCB` struct with `ReadPCB`/`WritePCB`
  (physical addresses, through the new `vm.Memory.LoadPhysical`/
  `StorePhysical`), and the two instructions, following the VAX
  Architecture Reference Manual (EY-3459E, 1987), chapter 6, operation
  text line by line. `LDPCTX` checks every UNDEFINED case before changing
  anything (not on the interrupt stack; P0BR, or P1BR + 2**23, not an
  aligned S0 address; the MBZ bits of the P0LR/ASTLVL and P1LR/PME
  longwords; ASTLVL above 4) and takes the preferred reserved-operand
  fault. The one UNDEFINED case it can't check ahead, a kernel stack it
  can't push the PC and PSL on, restores the registers saved before the
  load and then faults (govax's choice). `SVPCTX` reads the PC and PSL off
  the stack before writing anything, so an access violation there
  restarts cleanly, and never writes the PCB's memory-management
  longwords (its note 1). `vax.PME` names privileged register 61 (the
  manual's name for what govax called PMR). `LDPCTX` empties the whole
  TB for now; subtask 5 narrows it to P0 and P1. `Engine.SaveContext`/
  `LoadContext` are the Go switcher's halves, sharing the instructions'
  code (`registerContext`, `readLoadablePCB`, `loadContext`):
  SaveContext has the effect of an IPL 3 interrupt plus `SVPCTX` (the PCB
  gets the PC of the next instruction and the current PSL, the live SP
  goes into its mode's slot, and the CPU is left in kernel mode on the
  interrupt stack at IPL max(3, IPL)); it refuses to run on the interrupt
  stack (`ErrContextOnInterruptStack`). LoadContext has the effect of
  `LDPCTX` plus `REI`, without the push and pop. The IPL 3 and the
  interrupt-style PSL (previous mode kernel, other bits clear) are govax's
  choices, **unconfirmed** by any manual (no probe can see them). The
  PCB's place in memory (beside the kernel stack, or a pool of its own;
  an open question) is left to subtask 9, which allocates it; PCBB is
  still 0 until then. `TestEveryInstructionImplemented` lost its two
  exceptions. Tests: `context_test.go` (layout, SVPCTX from either
  stack and its IPL rule, privilege, LDPCTX's loads and each UNDEFINED
  case, a bad kernel stack, SaveContext/LoadContext round trip) and
  `context_resched_test.go`, which assembles the manual's RESCHED example
  as a VAX program and runs two processes switching through it with the
  IPL 3 software interrupt.
- 2026-10-07: Subtask 5 (per-process TB invalidation).
  `vm.Memory.InvalidateProcessTB` empties the 64 TB slots that hold P0
  and P1 translations (the TB indexes slots by region, 32 each, so
  they're the first two blocks), and the STC and the instruction-fetch
  window, keeping S0's; `LDPCTX` (and `LoadContext`) use it in place of
  the whole-buffer flush. It's counted in SHOW TB's `PFlushes`, which had
  never been incremented (Phase 21 kept it, always zero, for the display);
  `HELP SHOW TB` (debug.help) now says what both flush counts mean.
  Tests: `TestInvalidateProcessTBKeepsSystemSlots` (P0 entries gone, S0
  entries kept and hit afterwards, the STC emptied, the counts), and
  `TestLdpctxLoadsContext` checks LDPCTX counts one process flush. From
  this subtask on, new comments describe behavior on its own terms
  rather than citing the C reference (the author's direction).
- 2026-10-07: Subtask 6 (the S0 page allocator). `corevms/s0pool.go`:
  an `S0Pool` hands out contiguous runs of S0 pages, first fit from the
  bottom, each recorded with its process's PID and a purpose
  (`Allocations`, for SHOW MEMORY later); `Free`, `FreeProcess`,
  `FreePages`, and an `S0ExhaustedError` that says how long the longest
  free run was. The System holds one (`SetS0Pool`/`S0Pool`), made by
  VMINIT for S0 from `s0Free` to S0's end; INIT and ZERO alone leave none.
  The microkernel, which ASM deposits at `s0Free`, takes its pages with
  `Claim` as `depositAsmImage` stores them, so the pool starts past it
  (and ASM can't later overwrite a process's pages: `Claim` over an
  allocation is an error). `System.AllocateS0` also clears the pages,
  through each page's physical address from the S0 page table, so a new
  page table starts all invalid. `RemoveProcess` frees whatever a process
  still holds. Measured after `vax.init` (`TestS0PoolAfterVaxInit`):
  VMINIT's layout is S0 pages 0-311, the microkernel 312-322, and the
  pool 323-8191, 7869 pages; at 233 pages per process (129 + 65
  page-table pages, 38 stack pages, a PCB page) that's 33 processes
  beside process 1. Tests: `corevms/s0pool_test.go` (first fit and
  holes, exhaustion, FreeProcess, Claim, clearing and freeing at
  removal on a mapped S0).
- 2026-10-07: VMINIT's page rounding fixed (the author's call, found in
  subtask 6). `roundUpPage` always advanced to the *next* page, even from
  an address already on a page boundary, so VMINIT left an unused page
  after each of its three page tables (each of which, with `vax.init`'s
  sizes, fills its last page exactly), and the region table's PTE page
  counts (SHOW MEMORY) were one too high for such tables. Now it rounds
  only when needed, and `pageTablePages` counts a table's pages. This
  moves every S0 address after the S0 page table down by a page or more
  (P0's table by one, P1's by two, the stacks, CONSOLE$SCRATCH, the shim
  page, the SCB, the string pool, and the microkernel by three); no test
  or oracle depended on them. After `vax.init` VMINIT's layout is now S0
  pages 0-308, the microkernel 309-319, and the pool 320-8191 (7872
  pages): at 231 pages per process (128 + 64 page-table pages, 38 stack
  pages, a PCB page), 34 processes. Tests: `TestVMInit_pageTablesPacked`
  and `TestPageRounding`.

- 2026-10-07: The length registers fixed (found writing subtask 7).
  `translate`, `LookupPTE`, and `StorePTE` checked `page > P0LR`,
  `page > SLR`, and `page <= P1LR`, where the Architecture Reference
  Manual (chapter 4) makes P0LR and SLR page counts and P1LR the lowest
  P1 page that exists: P0 and S0 admitted a page past their tables, and
  P1 refused its lowest page. A process's pool-allocated tables mustn't
  be read past their end, so this came first. VMINIT's register values
  were already the architectural ones, so P1's bottom page becomes
  reachable and nothing else visible moves; `$CNTREG`'s P1 end is now
  the page P1LR names. `TestTranslateLengthBoundaries`; logged in
  `DEVIATIONS.md`.
- 2026-10-07: Subtask 7 (address spaces). `internal/vm/space.go`:
  `vm.AddressSpace` (P0BR, P0LR, P1BR, P1LR; plain lengths, ASTLVL and
  PME unpacked away), `CurrentAddressSpace(cpu)`, and `Memory`'s
  `TranslateIn`, `LoadIn`, `StoreIn`, `LoadLongwordIn`, and
  `StoreLongwordIn`, which look a P0 or P1 address up in the given
  space's tables (an S0 address in the system table, as for every
  process). Protection is checked as kernel mode; demand-zero pages are
  allocated and the modify bit set as `Translate` does. They never read
  or fill the TB or the STC, which hold the current process's
  translations, so each call walks the tables. `cpu.PCB.AddressSpace`
  gives a PCB's. Tests: `space_test.go` (one P0 address in two spaces,
  shared S0, the TB untouched, page-crossing loads and stores,
  demand-zero and the modify bit, kernel-mode protection, the space's
  own lengths) and `TestPCBAddressSpace`.
- 2026-10-07: Subtask 8 (building and tearing down an address space).
  `corevms/addrspace.go`: `ProcessPTE` is the PTE a process page starts
  with (demand zero, user-owned, open to every mode; P0 page 0 a
  no-access, kernel-owned guard), and VMINIT's P0 and P1 loops now use it,
  so every process starts alike. `ProcessSpace` is a process's
  `vm.AddressSpace` with its table sizes and pool allocations;
  `Environment.Space` holds it. VMINIT makes process 1's with
  `AdoptAddressSpace` (its own tables, never given back).
  `System.BuildAddressSpace(pid, p0Pages, p1Pages)` allocates the two
  tables from the S0 pool, writes them through their S0 addresses, and
  maps the P1 vector: the console's ASM deposit of `.P1VECTOR` calls
  `System.ShareP1`, which records the vector's pages and their physical
  pages, and a new process maps those same pages read-only to every mode
  (ProtUR) and kernel-owned, so it can call services through the vector
  but can't change or delete it. The vector holds only trampolines and
  the two `SYS$GL_*` cells nothing uses (subtask 1), so nothing in it
  needs to be per process. `TeardownAddressSpace` frees every physical
  page the tables map except the shared ones, then the tables' pool
  pages; it refuses the CPU's current space and ignores process 1's.
  `RemoveProcess` calls it. The table sizes are the caller's (the open
  question): VMINIT's, 16384 and 8192 pages, cost 192 pool pages, and
  Phase 45 picks what `$CREPRC` passes. Tests: `addrspace_test.go`
  (process 1 adopts VMINIT's tables; a built space after `vax.init` has
  its own P0, a guard page 0, P1's top and bottom pages, process 1's
  vector page read-only; teardown gives back every frame and pool page;
  bad sizes and an exhausted pool leave nothing allocated;
  `RemoveProcess` tears down).
- 2026-10-07: Subtask 9 (per-process privileged stacks).
  `corevms/stacks.go`: `System.BuildStacks(pid, kernel, executive,
  supervisor)` takes one pool run for a new process's three stacks, laid
  out and protected as `docs/MODE-STACKS.md` describes process 1's
  (kernel URKW, a no-access guard, executive EW, a guard, supervisor SW;
  38 pages with `vax.init`'s sizes), and the user stack top is
  `corevms.UserStackTop` (0x7FE00000) in the process's own P1. The open
  question about the PCB's place is settled: a pool page of its own,
  kernel-only (KW), its physical address kept for PCBB; VMS's process
  header puts it beside the page tables, which a separate page doesn't
  need. `Environment.Stacks` holds a process's (`ProcessStacks`);
  VMINIT adopts its own as process 1's (`AdoptStacks`), with no PCB
  page until `EnsurePCB` gives it one (VMINIT's layout has no room, and
  the pool's bottom pages are the microkernel's, claimed as ASM deposits
  it, so process 1's PCB must be allocated after that; Phase 44's
  switcher does it). `InitialPCB` is a new process's first PCB (stacks,
  address space, ASTLVL 4, a given PC and PSL). `AllocateS0` now puts
  each page's protection back to S0's default (and empties its TB
  entry), so a freed stack's guard pages don't follow its pages to the
  next owner; `SetS0Protection` sets it. The interrupt stack stays
  shared. MODE-STACKS.md updated. Tests: `stacks_test.go` (layout,
  protections, pointers, the PCB page, reset on reuse; process 1's PCB;
  the initial PCB through memory).
- 2026-10-07: Subtask 10 (per-process image state). `console/images.go`:
  an `imageProcess` is one process's image state: its ICB list, the
  symbols activation defines (`MAIN`, `SHARE$name_INITIALIZE`,
  `SHARE$name_TRANSFER_n`), and its IMAGE$INIT driver page. The loader
  (`imageLoad`, `imageFixup`, `setImageProtection`, `activateImage`,
  `buildImageInitDriver`, `p1Stub`, and the header readers) are its
  methods, and read and write the process's memory through its address
  space (`vm.Memory`'s `...In` methods, and the new
  `LookupPTEIn`/`StorePTEIn` for section protection), so an image can be
  activated in a process that isn't current; its P0 high-water mark is
  that process's `RegionSize[0]`. Process 1's state is the console's
  (`Console.images()`), kept across INIT/VMINIT/ZERO as the ICB list
  always was; `Console.ICBList` is gone, and RUN, SHOW IMAGES, the
  debugger's lookups, and the console's other readers use process 1's.
  Another process's is made by `imagesOf(env)` and dropped when a new
  System is built (`newRTL`); Phase 45's process deletion should drop
  it too. The Console's old method names (`activateImage`, `imageLoad`,
  ...) remain as process 1's. Bug 5: process 1's activation symbols
  are still set in the console's symbol table (so `EXAMINE MAIN` works),
  another process's only in its own. Bug 1: process 1's driver stays at
  `CONSOLE$SCRATCH+8` (rule 3, the layout; a nested RUN in process 1
  reloads its P0 under the running image anyway), and every other
  process's goes in a pool page of its own, kernel-writable, charged to
  the process and freed with it. `runHost`, `runCommandLine`, and
  `imageActive` stay on the Console: they describe the console's
  foreground RUN of process 1. Tests: `images_test.go` (an image
  activated in a second, non-current process: its section in that P0,
  process 1's page untouched, separate ICB lists, high-water marks, and
  symbols, the driver in a pool page with its `$EXIT` call and
  `CONSOLE$SCRATCH` untouched, freed at removal; process 1's state kept
  at INIT, others' dropped) and `TestAddressSpacePTEs`.
- 2026-10-07: Subtask 11 (settings and help). `corevms/procsettings.go`:
  `ProcessSettings` (`Scheduler`, `Quantum`, `Preempt`), its defaults
  (`DefaultProcessSettings`: off, 20,000 instructions, `all`), and
  `PreemptMode` with `ParsePreemptMode` for `all`, `user`, and `none`.
  `System.ProcessSettings` holds them; the console reads
  `vax.process.scheduler`, `vax.process.quantum`, and
  `vax.process.preempt` into each System it builds
  (`console/procsettings.go`), keeping a default for a key that's unset
  or bad. Nothing acts on them until Phase 44. The keys are in
  `validConfigs`, and `auditConfig` also warns at startup when
  `vax.process.preempt` isn't one of its three values (the console would
  otherwise use `all` silently). `HELP CONFIG KEYS` has a Processes
  section. Tests: `TestParsePreemptMode`, `TestNewSystemProcessSettings`,
  `TestProcessSettings`.
- 2026-10-07: Subtask 12 (the hand-switch test).
  `console/handswitch_test.go` (`TestHandSwitchTwoProcesses`, an
  external test with the consoletest helpers) boots with `vax.init`,
  runs a few user-mode instructions in process 1 at P0 0x400, builds
  process 2 (`BuildAddressSpace`, `BuildStacks`, `InitialPCB` with its
  PC at the same 0x400), writes its program through its address space,
  and switches with `SaveContext`, PCBB, and `LoadContext`, then back.
  It checks: each process stores to P0 0x600 and pushes on its user
  stack at 0x7FDFFFFC, and each sees its own value at both; both read
  one S0 longword and the P1 vector's `SYS$EXIT` entry alike; process 1's
  PCB holds its PC, R0, user SP, PSL, and P0BR while process 2 runs, and
  process 2's its registers and address space after the switch back;
  process 1 then carries on. Found writing it: a P1 table must reach
  down to the user stack top for the stack to be there (4096 pages
  below P1's top; the vector is 145 pages down), so
  `corevms.MinP1Pages` (4097) names the smallest useful one. The whole
  suite passes unchanged.
- 2026-10-07: Subtask 13 (close-out). Status set to done; the open
  questions answered above; `PHASE-44.md`'s "What earlier phases leave in
  place" lists exactly what Phase 43 hands over (building a process by
  hand, process 1's PCB, the settings, the per-process image state);
  `PLAN.md`'s row, and `CLAUDE.md`'s package notes (`corevms.System` and
  the process table, `ProcessSpace`/`ProcessStacks`, `vm.AddressSpace`,
  `cpu/context.go`), updated. `CLAUDE.md` also named the RTL package
  `internal/coreos`, its old name; it's `internal/corevms`. Help: `HELP
  CONFIG KEYS` (subtask 11) is the only user-visible change; nothing else
  a user can do is new until Phase 44.
