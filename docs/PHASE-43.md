# Phase 43 — Multiprocessing, part 1: processes as objects

**Status:** planned (2026-10-06), awaiting review. Not started.

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
   the user's VMS 7.3 system), and from published books if Decision 6
   allows. Never from VMS source listings. A rule no source settles is
   chosen sensibly and logged as unconfirmed in the phase's doc.
8. **Phase docs.** Each phase extends its own progress log as it goes, and
   records discoveries (this is a large program; the subtask lists below
   will change).

## The phases

| Phase | Doc | Summary |
| --- | --- | --- |
| 43 | this doc, Part B | Processes as objects: system/process state split, process table, hardware PCB with LDPCTX/SVPCTX, per-process address spaces and stacks, cross-address-space access. One process still runs; a Go test switches into a second by hand |
| 44 | [PHASE-44.md](PHASE-44.md) | The scheduler: `internal/sched`, the engine's reschedule hook, quantum and priority preemption, wait states from waiting services, idle and timers, process-aware run loops and debugger, SHOW SYSTEM |
| 45 | [PHASE-45.md](PHASE-45.md) | Creating and deleting processes: `$CREPRC`, process startup and image activation in the new process, rundown and deletion, termination mailbox, jobs and logical-name tables, the other process-control services across processes, `$GETJPI` wildcards, NL: |
| 46 | [PHASE-46.md](PHASE-46.md) | Interprocess communication: mailboxes between processes, common event flags, global sections (shared memory), RMS on mailbox and null devices, terminal reads that don't block the machine |
| 47 | [PHASE-47.md](PHASE-47.md) | Files shared between processes: the lock manager and `$ENQ`/`$DEQ`, file access arbitration (RMS$_FLK), a shared file control block in ods2, shared sequential files, record locks, volume-integrity stress tests |
| 48 | [PHASE-48.md](PHASE-48.md) | `LIB$SPAWN` and the milestone: the subprocess CLI, the parent/child MACRO programs, the acceptance tests, the scheduler flag on by default, documentation |

Each phase leaves govax working and its new pieces tested. 44 needs 43;
45 needs 44; 46 and 47 need 45 and are independent of each other; 48
needs all of them.

## Decisions for the author

Each has a recommendation; the plan below assumes it.

1. **Where the scheduler lives.** *Recommended:* in Go (`internal/sched`
   plus the engine hook), switching contexts with the same code as
   `SVPCTX`/`LDPCTX`, as govax's system services are Go. The alternative,
   a VMS-style scheduler written in VAX code in `kernel.asm` driven by an
   IPL 3 software interrupt, is more faithful but would have to call back
   into Go for every service's wait anyway, and would be far slower to
   write and debug.
2. **The quantum's unit.** *Recommended:* instructions executed by the
   process (`vax.process.quantum`; a starting default of 20,000, tuned in
   Phase 44 so that switching costs little but processes visibly
   interleave), deterministic in both clock modes. The alternative is VMS's
   QUANTUM in 10 ms units of emulated time: deterministic in quantum-clock
   mode, but wall-clock driven (so not reproducible) with
   `vax.hardware.clock`, and sensitive to `SET QUANTUM` (which `vax.init`
   sets to 1).
3. **Which modes can be preempted.** *Recommended:* VMS's rule — any
   access mode, when IPL < 3 and not on the interrupt stack — with
   `vax.process.preempt=user` to limit it to user mode while debugging the
   scheduler, and `none` for cooperative switching (waits only).
4. **Other processes while the console waits at `VAX>`/`DBG>`.**
   *Recommended:* frozen. They run whenever the engine runs (a RUN, GO,
   CALL, or debugger GO/STEP), and a process left running when its parent's
   image ends waits for the next run. Running them in the background while
   the console reads commands would need the engine on its own goroutine,
   with locking around everything the console touches; possible later.
5. **The scheduler flag's default.** *Recommended:* `false` until Phase
   48's milestone passes, then `true`, keeping the key so it can be turned
   off. Alternatively it could stay `false` permanently (multiprocessing
   opt-in).
6. **Reference books.** May the clean room use DIGITAL's published book
   *VAX/VMS Internals and Data Structures* (in the manuals folder) for
   concepts such as the boost values, state transitions, and the shape of
   `$CREPRC`'s process startup? *Recommended:* yes, for descriptions only
   (it is a published DIGITAL book, like the manuals), never for its code
   excerpts; and the *VMS Internals I ... Listings* course book stays
   off-limits, as listings. If no, those rules are chosen and logged as
   unconfirmed.
7. **VMS 7.3 probes.** Will you run probe programs on your VMS 7.3 system,
   as for earlier phases? *Recommended:* yes, for the few things only real
   output settles: the termination message's contents, `$GETJPI` across
   processes, `SHOW SYSTEM`/`SHOW PROCESS` layouts, RMS's sharing statuses,
   and `LIB$SPAWN`'s behavior. Each phase names its probes; none blocks
   progress (an unprobed rule is logged as unconfirmed).
8. **`LIB$SPAWN`'s command language.** A spawned subprocess runs DCL, but
   govax's DCL is the console, in Go. *Recommended:* the subprocess runs a
   small Go "subprocess CLI" that executes the commands that run images —
   `RUN`, foreign commands (DCL symbols), and `MCR` if wanted — with
   others rejected as an unsupported command; extended later if needed.
   The alternative, the whole console command set inside a subprocess, needs
   the console made process-aware throughout (its output, its defaults,
   its tables).
9. **Changing ods2.** The shared file control block (Phase 47) needs new
   ods2 API, tagged and pinned as in Phase 34. *Recommended:* yes.
10. **`$ENQ`/`$DEQ` as services.** *Recommended:* yes; they're a thin
    layer on the lock manager RMS needs anyway, and the usual VMS way for
    processes sharing memory to synchronize.
11. **The debugger with several processes.** *Recommended:* the debugger
    debugs process 1; breakpoints belong to process 1 (a breakpoint
    address in P0 means process 1's P0); stepping freezes the other
    processes; a subprocess's image never starts a debugger of its own
    (VMS would start one if the image was linked `/DEBUG`). A later phase
    could add debugging a subprocess.
12. **The milestone's two creation paths.** *Recommended:* the milestone
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
   see its parent's temporary mailbox names (Phase 45).
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

## State inventory (first pass)

Subtask 1 completes and checks this table; it's the map for subtask 2.

| Where | Field | Belongs to |
| --- | --- | --- |
| `corevms.Environment` | `mem`, `cpu` | machine (shared) |
| | `shims`, `services` (registries) | system |
| | `Devices`, `Mounts`, `Mailboxes`, `EventFlagClusters`, `Operator` | system |
| | `Clock`, `BootTime`, `NodeName` | system |
| | `Logicals` | split: process directory/table per process, job table per job, system directory and system/group tables shared |
| | `OnUnhandled`, `OnSignal` (debugger hooks) | system, told which process |
| | `Process`, `channels`, `nextChannel`, `files` (RMS IFIs), `Session` (default directory) | process |
| | `RegionSize`, `memAllocated`/`memFreed` (heap), `openFiles`/`nextFID` | process |
| | `timers`, `attentionASTs`, `pendingIO`, `qiowWaits`, `waitingPC`, `CommandLine` | process |
| | `consoleIn`/`consoleOut` | process (its SYS$INPUT/SYS$OUTPUT; the terminal itself is shared) |
| `corevms.Process` | identity, quotas, privileges, event flags, AST state, exit handlers, condition and change-mode stacks, exception vectors, page locks | process; job quotas move to a job record |
| `console.Console` | `ICBList`, image symbols, the IMAGE$INIT driver, `imageActive`, `runHost` | process |
| | `Symbols` (system symbols), `shimBase`, `s0Free`, `Regions`, the debugger, run state | system / console |
| `vax.CPU` registers | R0–R11, AP, FP, SP, PC, PSL, KSP/ESP/SSP/USP, P0BR/P0LR/P1BR/P1LR, ASTLVL | process (saved in its hardware PCB) |
| | ISP, SBR/SLR, SCBB, PCBB, MAPEN, clock and console registers | system |
| `cpu.Engine` | everything | system; per-process CPU time accounting is new (Phase 44) |
| `vm.Memory` | everything | system; TB entries for P0/P1 are per process (flushed on switch) |

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
  subtask 4/9.
- Does anything in `kernel.asm` assume one process (fixed stack symbols,
  `CTL$`-style cells)? Subtask 1 checks.
- Page-table sizes for subprocesses: VMINIT's (16384 P0 and 8192 P1 pages
  in `vax.init`) cost about 192 pool pages each. If that limits the
  process count, a subprocess could get smaller tables (VMS sizes them by
  quotas); decided in subtask 8.

## Progress log

- 2026-10-06: Planned (this document and Phases 44–48). Awaiting the
  author's review and the decisions above.
