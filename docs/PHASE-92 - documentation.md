# Phase 92 — documentation

**Status:** in progress (planned 2026-11-11). Part A, the first
document, `docs/VAX-VMS-INTERNALS.md`: subtasks 1 (the plan, the
document's outline, and its sections 0 to 3), 2 (section 4, the
instruction set), and 3 (section 5, memory) are done.

## Goal

govax's documents so far are of two kinds: the phase docs, which record
how each piece was built and decided, a phase at a time, and two guides,
the user's manual (`MANUAL.md`) and the FORTRAN compiler's tour
(`FORTRAN-INTERNALS.md`). What's missing is the guide to the rest of the
machine: how govax is a VAX, and how it makes a program running on that
VAX believe it is running under VMS. Someone who wants to know that today
has to read ninety phase docs in order, and much of what they say has
since been changed by later ones.

This phase writes such guides, one document at a time, each standalone
(it may say that the phase docs and `PLAN.md` hold the history, but it
must not send its reader to them for anything it explains), each
tutorial, for a reader who may know little of the VAX or VMS. It is
meant to grow: each new document is a part of this doc with its own
subtasks.

## Part A — `docs/VAX-VMS-INTERNALS.md`

The author's request (2026-11-11): a comprehensive guide to how govax
implements a VAX, including the VMS hooks. The taxonomy of the packages
that implement the VAX and VMS, then a deeper look at the key structures
(the engine, the CPU, the memory, and so on), and these topics:

1. the instruction set and its fidelity;
2. memory layout, page management, virtual address translation, page
   faults;
3. VMS-style paging, and how it connects to the memory;
4. processes, with the interprocess mechanisms and console I/O as it
   relates to context switching;
5. the emulation of I/O;
6. how RMS and ODS-2 interact;
7. how govax switches between VAX emulation and the Go-implemented VMS
   services;
8. how system startup works.

Out of scope: the user-mode tools, DCL, FORTRAN, MACRO, LINK, LIBRARY,
BACKUP, and the other utilities. The document is about the experience of
a user-mode process running on govax's VAX and VMS: what's beneath it.
DCL appears only where it is part of that machinery (it's the command
interpreter process 1 runs, and what reads the startup procedure).

### Decisions

1. **One document, standalone.** It names packages, types, and files,
   which a reader can open; it never names a phase doc. Facts it needs
   from the phase docs, CLAUDE.md, `MODE-STACKS.md`, and `PERFORMANCE.md`
   are restated in it.
2. **Tutorial order.** It starts with a primer on the VAX and VMS
   (section 1), then govax's layers (section 2) and its central
   structures (section 3), before the eight topics. The topics are
   ordered from the bottom up (instructions, memory, paging, processes,
   I/O, files), then the crossing between VAX code and Go, which needs
   all of them, then startup, which puts them together.
3. **Code shown is real.** Go excerpts are cut down from the source,
   with a note where they are; VAX code is MACRO-32 that govax's
   assembler takes, or bytes the source writes (the shim stubs, the
   system service vector).
4. **What govax doesn't do is said.** Where govax departs from the VAX or
   from VMS (no S1 region, a microkernel instead of an executive, the
   services in Go, no permanent part of the working set, synchronous
   paging), the document says so beside the mechanism, so a reader
   doesn't mistake it for VMS's.
5. **Name.** The request said `docs/VAX-VMS-NTERNALS.md`; read as
   `docs/VAX-VMS-INTERNALS.md`.
6. **Bugs found are fixed** (the author, 2026-11-11). Writing a guide
   means reading the code closely; a bug or a departure from the VAX or
   VMS that turns up is fixed in the same subtask, with its tests, rather
   than logged in `DEVIATIONS.md` and deferred, and the guide describes
   the fixed behavior. (This phase's exception to CLAUDE.md's default of
   documenting and deferring ISA questions.) A fix too large for a
   subtask, or whose right behavior is unclear, is raised with the author
   first.

### The outline

0. About this document
1. A VAX and VMS in brief (registers, the PSL and access modes, the
   address space and pages, instructions and their operands, procedure
   calls; processes, images, system services, the P1 vector, ASTs,
   conditions, RMS, ODS-2, logical names)
2. The shape of govax (the layers, and every package of the machine and
   of VMS, in a table per layer; the seams between layers)
3. The central structures (`registers.CPU`, `vm.Memory`, `cpu.Engine`,
   `machine.Machine`, `corevms.System`, `corevms.Environment` and
   `Process`, the RMS session and mount table, the console; how they hold
   one another; one instruction's trip through them)
4. The instruction set
5. Memory: layout, translation, and faults
6. VMS paging
7. Processes
8. I/O
9. RMS and ODS-2
10. From VAX code to Go and back
11. Starting the system
12. A map of the source

### Subtasks

1. The plan (this part), the document's outline, and sections 0 to 3.
   (done 2026-11-11)
2. Section 4, the instruction set: the table and its generator, decode
   and operand specifiers, dispatch, condition codes, the floating and
   decimal types, faults and traps and the operand log, the trace trap,
   interrupts and the SCB, change mode and REI, the clocks, the
   instruction-fetch window; fidelity: the oracle tests
   (`testdata/insn35`), DEVIATIONS.md's policy, and what's known to
   differ. (done 2026-11-11)
3. Section 5, memory: physical memory, ROM and NVRAM, the regions,
   PTEs and protection, translation step by step, the TB and the
   sequential cache, length violations, VMINIT's layout of S0 (the old
   system service vector, the page tables, the mode stacks, the shim
   page, the SCB, the string pool, the S0 pool), the P1 vector, the
   pager hook and the invalid PTE forms, demand zero, file sections.
   (done 2026-11-11)
4. Section 6, VMS paging: the switch, the PFN database and its lists,
   working sets and replacement, soft and hard faults, the modified page
   writer and the reclaimer, the page file, PGFLQUOTA, global pages,
   read clustering, image activation by mapping, created address space
   and the table area, the user stack's growth; what govax's VMS lacks.
5. Section 7, processes: process 1 and the others, the process table
   and PIDs, address spaces and stacks, the hardware PCB and
   LDPCTX/SVPCTX, the scheduler (states, priorities, quantum, boosts),
   waits and event reporting, ASTs, event flags, mailboxes, global
   sections, the lock manager, subprocesses and detached processes, the
   CLI processes, the shared terminal, lending the CPU while DCL waits,
   Ctrl/C and Ctrl/Y.
6. Section 8, I/O: the device table, channels, `$QIO` and its drivers
   (terminal, mailbox, null, disk), I/O completion (IOSB, event flags,
   ASTs), the console's XFCs, process-permanent files, the host side.
7. Section 9, RMS and ODS-2: the `ods2` module, mounts and the mount
   table, the session, name processing and logical names, FAB/RAB/NAM/XAB
   in VAX memory, the file table and record streams, sequential,
   relative, and indexed files, sharing and record locks, block I/O and
   user file open, host files beside volume files.
8. Section 10, from VAX code to Go: XFC, the system service vector, the
   service table, argument lists, statuses, waits, calls back into VAX
   code, the shims and their stubs, JSB entries, the run-time libraries
   (LIBRTL, MTHRTL, LBRSHR, FTNRTL as VAX code), condition dispatch and
   the console handler, `$EXIT`, image rundown.
9. Section 11, startup: `cmd/govax`, the system volume, the grammars,
   STARTUP.COM, INIT and INITIALIZE/MEMORY, the microkernel and its
   initialization, devices, the page file, the user's startup, RUN and
   image activation (the IMAGE$INIT driver).
10. Section 12 and the close-out: the map, a read-through against the
    source, CLAUDE.md and PLAN.md, and this part's "Where things stand".

## Progress log

- **2026-11-11** Subtask 1: read the core packages (`registers`, `vm`,
  `cpu`, `machine`, `corevms`'s `System`, `Environment`, and dispatch,
  the system service vector, the shim stubs, `kernel.asm`, STARTUP.COM,
  and `cmd/govax`'s start) and wrote this plan and the document's
  sections 0 to 3, with placeholders for 4 to 12.
- **2026-11-11** Subtask 2: section 4, the instruction set: the table and
  its generator, operand specifiers and the `Operand` they decode to (with
  a worked ADDL3), handlers and condition codes, CALLS's frame, the
  floating, decimal, and string families, what happens between
  instructions (time, interrupts, the scheduler, ASTs, the trace trap),
  faults and traps with the operand log and `HandleFault`'s SCB sequence
  and `console$handler`, change mode and REI, and fidelity (the oracles
  and what's known to differ). Reading `emulRei` for it found that REI
  doesn't check the PSL it restores; logged in `DEVIATIONS.md` (deferred).
  The clocks' and interrupts' detail stays here; the instruction-fetch
  window and the TB are section 5's.
- **2026-11-11** Subtask 2, continued: the author made fixing such
  findings part of this phase (Decision 6), so REI's was fixed.
  `reiAllows` (`internal/cpu/call.go`) applies the eight checks of the
  1987 *VAX Architecture Reference Manual*'s REI operation (the 1982
  edition lists six; its "IS with a nonzero mode" follows from two of the
  later ones), a refused PSL a reserved operand fault with nothing changed;
  CM set faults, as on a processor without compatibility mode.
  `Engine.LoadContext` checks the PCB's PSL the same way. The microkernel's
  boot REI pushed `^X03000000` (current user, previous kernel), which the
  check refuses, so `kernel.asm` now pushes `^X03C00000`; a system volume
  whose KERNEL.ASM the user changed (kept, `%MOUNT-I-SYSKEPT`) needs the
  same change. Two `cpu` tests' hand-built frames had the same PSL and were
  corrected. New tests: `TestReiChecksPoppedPSL`, `TestLoadContextChecksPSL`.
  The entry moved to `DEVIATIONS-RESOLVED.md`, and section 4.6 describes
  the checks.
- **2026-11-11** Subtask 3: section 5, memory: physical memory, the ROM
  and NVRAM, the PTE and the protection codes (a table, and the formula
  behind it), the translation walk in order with a worked address from
  process 1, the faults and their masks, `vm`'s doors for each kind of
  reader, the TB, the STC, and the fetch window, INITIALIZE/MEMORY's S0
  layout as a table of addresses (measured on govax after STARTUP.COM),
  the S0 pool, process space and P1's map, the P1 vector, the invalid PTE
  forms and the pager hook, and a list of what differs from VMS. Section 4's
  sentences on interrupts' stack and the SCB vector's stack bits were
  corrected to match. Reading for it found, and this subtask fixed
  (Decision 6):
  - **The process PTE's reference was checked in the current mode.** The
    VAX reads a P0 or P1 page table entry through the system page table as
    a kernel-mode read. `vm`'s `translatePTE` now makes it so (kernel
    mode, the TB but not the STC, which holds the current mode's grant);
    `LookupPTE` and `StorePTE` reach the entry the same way. govax's
    page-table pages are URKW, so no access had failed.
    `TestPTEReferenceIsKernelRead`.
  - **Process 1's P1 vector was UW, owned by user mode**, so a user-mode
    program could change the vector every process maps; other processes
    mapped it UR. All now map it as VMS does, URKW and owned by kernel mode
    (*VAX/VMS Internals and Data Structures*, Table 26-4b; `corevms`'s
    `vectorPTE`, set in process 1 by `ShareP1`). `TestBuildAddressSpace`
    checks both processes and a user-mode write.
  - **Taking the interrupt stack turned MAPEN off, and leaving it for any
    other stack turned MAPEN on** (an eVAX choice kept since Phase 03,
    `DEVIATIONS-RESOLVED.md`); ISP was a physical address, which with
    mapping on named a P0 page. `setModeStack` no longer touches MAPEN, and
    INITIALIZE/MEMORY gives ISP its S0 virtual address. No microkernel
    vector names the interrupt stack, so only tests, the console's SET, and
    the debugger's ISP mode had reached the old path. With it: an exception
    served on the interrupt stack raises IPL to 31 (`Fault.Interrupt`
    tells an interrupt, which keeps its IPL), and REI to a PSL with IS set
    takes SP from ISP, not KSP. `TestHandleFaultInterruptStackMapped`,
    `TestReiStaysOnInterruptStack`; `TestVaxInitMemoryConfig`'s arithmetic.
  - **SHOW MEMORY's region lines, paging off**, gave P0 and P1 "physical
    addresses" from a counter VMInit advanced per PTE written (P0's ran
    past the end of memory) and called the page tables' size "PFN
    database". They now give "Table pages", and for P0 and P1 the page
    table's own physical addresses, "Page table (phys.)"; MANUAL.md's
    example follows.
  Comments brought up to date: `s0pool.go`'s figures (the layout is now
  pages 0-314, the string pool's default having grown to 8 pages; the pool
  325-8191), `.P1VECTOR`'s note on protection, MODE-STACKS.md's interrupt
  stack and stale paths. Not this subtask's: four `internal/console` oracle
  tests (`TestLBR90WOracle`, `TestVMS88Procedure`, `TestIdx88ReportsOracle`,
  `TestIdx88CAnalyzeOracle`) fail on and after the 10th of a month, VMS's
  logs having been made on a day of one digit and the masks keeping the
  date's width; they fail the same at subtask 2's commit.
