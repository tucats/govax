# Phase 67 — the address space and page tables under paging

**Status:** planned (2026-10-15), **deferred to the end of the paging
program** (the author, 2026-10-15). Needs Phase 66. Part of the paging
program laid out in `PHASE-62 - VMS paging.md` (Decision 16 is this
phase's; Decision 10's switch to charging at creation happens here).

## Goal

With paging on, give govax VMS's model of a process's address space and
stop spending physical memory on page tables and S0 nobody uses:

- **Created space.** P0LR and P1LR describe the pages that exist, as on
  VMS. `$CRETVA`, `$EXPREG`, image activation, and `$CRMPSC` create
  pages, and `$DELTVA`, `$CNTREG`, and image reset delete them. A touch
  beyond P0's end, or below P1's low end, is a length violation (an
  ACCVIO with the length bit in its reason mask). The user stack grows by
  itself, one page at a time, when a user-mode access just below P1's low
  end faults (EXE$EXPANDSTK, 16.2.1.3). FREP0VA and FREP1VA are real.
- **PGFLQUOTA at creation.** Each demand-zero and copy-on-reference page
  is charged when it's created, so `$CRETVA` and `$EXPREG` fail with
  SS$_EXQUOTA exactly where VMS's do. Decision 10's first-touch rule
  goes away.
- **Page tables backed on demand.** Each process's tables are still made
  when the process is (VMINIT for process 1, `BuildAddressSpace` for the
  others), with a *virtual* size fixed then: VMINIT's `/P0` and `/P1` for
  process 1, and `vax.sysgen.virtualpagecnt` (VMS's VIRTUALPAGECNT, P0
  and P1 together) for the others. Only the S0 pages of a table that
  cover created pages get frames; growing a region backs more, and
  shrinking it gives them back. Page tables never fault (Decision 13): a
  table page is backed *before* the length register lets the hardware
  reach it.
- **The S0 pool backed on demand.** In paging mode VMINIT backs only the
  structures it lays out itself (and the microkernel's area, see below).
  The S0 pool's pages get frames when `AllocateS0` hands them out, and
  give them back when freed, instead of VMINIT's eager identity map of
  every S0 page (half of physical memory with the default settings).

## What earlier phases leave in place

- **`machine/vminit.go`:** the S0, P0, and P1 tables and their region
  records.
- **`corevms`:** `addrspace.go`'s `BuildAddressSpace` and
  `TeardownAddressSpace`, `s0pool.go`, `stacks.go` (which finds the
  PCB's physical address by translating, the right way),
  `vaspace.go`'s services, `RegionSize`, `expandP1`, and
  `userStackPages`.
- **`cpu/context.go`:** the PCB's copies of P0BR, P0LR, P1BR, and P1LR,
  and `SaveMemoryContext`, which writes the live registers into the
  outgoing PCB.
- **Phase 64:** the PGFLQUOTA charge, which moves to creation.

## Design notes

- **The length registers live in two places,** the CPU (current process)
  and each process's PCB. Changing a region updates the CPU when the
  process is current, and the PCB otherwise (`StorePTEIn` already reaches
  another process's tables).
- **The microkernel and other S0 writes.** The console's ASM assembles
  the microkernel into S0 after VMINIT's layout, before the S0 pool's
  base is set, and the console can DEPOSIT anywhere in S0. In paging
  mode an S0 page outside VMINIT's layout and outside the pool's
  allocations is demand-allocated: its first touch gets a *system frame*,
  never paged and never on a list. S0 still "never faults" from the
  program's point of view. The alternative is a fixed kernel reserve in
  VMINIT; the survey chooses.
- **Physical-address assumptions:** any code that computes an S0 page's
  physical address as its virtual address minus 0x80000000 breaks once
  pool pages aren't identity mapped. The survey finds them all (VMINIT's
  own layout stays identity mapped, so the interrupt stack, addressed
  physically, is safe).
- **The user stack's initial size:** VMS creates a small user stack and
  lets it grow. govax reserves `userStackPages` (1024) below the stack's
  top. With created space, the initial stack is created (and charged),
  and the rest of the reserve becomes room to grow.
- **Process 1's console conveniences:** ASM and DEPOSIT into process 1's
  P0 beyond its end expand P0, so interactive use keeps working.

## Open questions

- **ACCVIO's reason mask:** does `internal/vm` already set the length
  violation bit for a P0/P1 length fault, and does the exception reach
  the program with VMS's signal arguments? The probe compares.
- **VIRTUALPAGECNT for subprocesses:** one key for P0 and P1 together,
  as VMS's, means a split must be chosen per process. VMS grows each
  region's table within one process header, so govax would reserve
  VIRTUALPAGECNT entries for each region's virtual table (S0 virtual
  space is plentiful), backed only as used.

## Subtasks (provisional)

1. **Survey:** every reader and writer of P0LR/P1LR (CPU, PCB, VMINIT,
   corevms, the debugger's SHOW); physical-address assumptions in S0;
   tests that touch P0 or P1 beyond what they created; the microkernel's
   S0 area.
2. **Created space:** the services grow and shrink the regions, the
   length registers follow in the CPU and the PCB, and FREP0VA/FREP1VA
   are reported.
3. **Automatic user stack expansion** on a user-mode length violation
   just below P1's low end, and in the Go paths that push onto the user
   stack (AST delivery, exception dispatch).
4. **Page tables backed on demand** within the virtual size;
   `vax.sysgen.virtualpagecnt`.
5. **The S0 pool backed on demand**, and VMINIT backing only its own
   layout in paging mode; system frames for S0 pages first touched
   outside the pool.
6. **PGFLQUOTA at creation** (Decision 10's switch), with SS$_EXQUOTA.
7. **The console's P0 conveniences** for process 1.
8. **The VMS probe:** SS$_EXQUOTA at creation, an ACCVIO beyond P0's end
   (its reason mask and signal arguments), stack expansion by deep
   recursion, and FREP0VA/FREP1VA/VIRTPEAK after `$EXPREG` and
   `$DELTVA`.
9. **Close-out.**

## Progress log

- **2026-10-15:** Planned with Phase 62's review; deferred to the end of
  the program.
