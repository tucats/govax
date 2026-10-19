# Phase 67 — the address space and page tables under paging

**Status:** in progress: subtask 1, the survey, done 2026-10-19 (Part A
below; its questions answered by the author the same day, each
recommendation accepted); subtask 2, created space, done 2026-10-19;
subtask 3, the user stack, done 2026-10-19; subtask 4, the table area
backed on demand, done 2026-10-19; subtask 5, the S0 pool backed on
demand, done 2026-10-19; subtask 6, PGFLQUOTA at creation only, done
2026-10-19; subtask 7, the console's P0 conveniences, done 2026-10-19;
subtask 8, VMINIT's sizes with paging on, done 2026-10-19; subtask 9 (the
suite with paging forced on, and the probe, `testdata/pg67`, checked
against VMS 7.1), done 2026-10-19.
Planned 2026-10-15,
deferred to the end of the paging program (the author, 2026-10-15).
Needs Phase 66. Part of the paging
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
  goes away. (Since Phase 64 the creating services and the image
  loader already charge at creation; only the first-touch charge, for
  pages nothing created, is left for this phase to remove.)
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
- **Phase 64:** the PGFLQUOTA charge, already at creation in the
  services that create pages; only the first-touch rule remains.
- **Phase 65 (2026-10-18):** `testdata/pg65` shows VMS's process page
  table pages in the working set: each P0 expansion's first touches
  fault a page table page in (a few more PAGEFLTS and PPGCNT than
  govax's, and a DIRIO of 2 on some, unexplained), and at a small
  working set they leave no entry free for a read cluster. Decision 13
  keeps govax's tables unpaged, so these stay listed differences
  (`expectedPG65Differences`) unless this phase revisits it; SYI$_
  PAGTBLPFC reports SYSGEN's 2.
- **Phase 66 (2026-10-19):** with paging on an image is mapped
  (`machine/imagemap.go`, `corevms/imagesec.go`): its ISDs' pages
  exist as section or demand-zero PTEs in P0, and `RegionSize[0]` and
  `icb.End` move by the ISDs' extent, which is where created space's P0
  length comes from at activation. The user stack ISD (type 253,
  `STACK=`) is still ignored in favor of the process's
  `userStackPages` reserve (finding 9). The forced runs found
  `TestPG64Oracle`'s detached process (a 16384-page P0's tables)
  taking the memory at 200 free frames, which page tables backed on
  demand would end; and VMS charged 62 pages at an image's first
  instruction where govax charges 24, part of it P1's image pieces
  (`testdata/pg66`, `expectedPG66Differences`).

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
  *Survey:* the length bit is set, and the signal array is VMS's, but
  the write bit never is and a protection violation's mask is wrong
  (finding 11, question 5; the author: fix it in both modes).
- **VIRTUALPAGECNT for subprocesses:** one key for P0 and P1 together,
  as VMS's, means a split must be chosen per process. VMS grows each
  region's table within one process header, so govax would reserve
  VIRTUALPAGECNT entries for each region's virtual table (S0 virtual
  space is plentiful), backed only as used. *Survey:* S0 virtual space
  isn't plentiful in govax (VMINIT's S0 is as big as the frames it maps,
  4 MB with vax.init's sizes), so the survey recommends VMS's single
  area instead (finding 5, question 1), which the author accepted.

# Part A — the survey (subtask 1, 2026-10-19)

## Sources

- ***VAX/VMS Internals and Data Structures*** (the OCR text, beside the
  PDF): 14.1.1 and figure 14-2 (the process page tables: one area of
  the process header, the P0 table growing up from its low end and the
  P1 table down from its high end, "room for expansion of either"
  between them; P1BR the address of the entry that would map
  40000000; P1LR the count of P1 entries that don't exist;
  PHD$L_FREP0VA, PHD$L_FREP1VA, and PHD$L_FREPTECNT, the entries left,
  VIRTUALPAGECNT less the two tables' sizes), 16.2 (creation and
  deletion: "the image reset routine deletes all of P0 space and a
  small part of P1 space"), 16.2.1 (pages created one at a time, so a
  service can partly succeed), 16.2.1.1 (the three limits:
  VIRTUALPAGECNT for P0 and P1 together, "the division of these pages
  between P0 space and P1 space is totally arbitrary and process
  specific"; the working set's room to lock the table pages,
  SS$_INSFWSL; PGFLQUOTA), 16.2.1.3 (automatic user stack expansion,
  below), 16.2.2 (what a deletion gives back), 26.1.1 (the tables'
  pages: (VIRTUALPAGECNT + 127) / 128).
- **VSI's system services pages** (`VSI-system-services-1.txt`):
  JPI$_FREP0VA ("the address of the first free page at the end of the
  program region"), JPI$_FREP1VA (the same at the end of the control
  region), JPI$_FREPTECNT (the pages available for expansion), and
  JPI$_VIRTPEAK (the peak virtual size).
- **VMS's own values:** `testdata/pg64/vax/pg64.log`, the VMS 7.1 VAX's
  SYSGEN: VIRTUALPAGECNT 65500 (default 12032, minimum 512). The
  debugger logs of `testdata/dbg` show that VAX's user stack near
  7FED5300 (under the debugger), a little below govax's UserStackTop.
- **govax:** `vm/translate.go` and `space.go` (the length checks),
  `cpu/context.go` (the PCB's copies), `cpu/dispatch.go` and
  `corevms/condition.go` (ACCVIO's parameters), `machine/vminit.go`,
  `machine/asm.go`, `machine/imagemap.go`, `machine/show.go`,
  `corevms/addrspace.go`, `s0pool.go`, `stacks.go`, `vaspace.go`,
  `core.go` (`$EXPREG`), `memory.go` (the heap's P0), `gblsec.go`
  (`mapRange`, `expandP1`), `filesec.go` (`mapPage`, `PageIn`,
  `processForSpace`), `frames.go`, `pgflquota.go`, `ast.go`,
  `librtl/fault.go`; every test that turns paging on.

## VMS's address space (14.1.1, 16.2)

- **One area for both tables.** A process header has room for
  VIRTUALPAGECNT entries; P0's table takes them from the bottom (P0BR at
  the area's start), P1's from the top (P1BR the area's end less
  2^21 entries' 8 MB), and either region grows into the room between.
  P0LR is P0's page count; P1LR the number of P1 pages that *don't*
  exist; FREPTECNT what's left. Neither base register moves as the
  regions grow.
- **Created space:** a page beyond P0LR or below P1LR doesn't exist, and
  touching it is a length violation. `$CRETVA`, `$EXPREG`, and the
  image activator create pages; `$DELTVA`, `$CNTREG`, and image reset
  delete them; the services work a page at a time, so the limits
  (VIRTUALPAGECNT, the working set's room for table pages, PGFLQUOTA)
  can stop one partway, the pages before kept.
- **Automatic stack expansion (16.2.1.3):** EXE$EXPANDSTK is called by
  AST delivery and `$ADJSTK` (for user-mode stack space) and by the
  access violation handler for a user-mode access violation. It checks
  that the violation was a *length* violation (not protection) in P1,
  and creates pages from P1's low end down to the address: "typically
  one page at a time", with no limit but VIRTUALPAGECNT (SS$_VASFULL)
  and the other creation limits, so a stray reference anywhere below the
  stack creates pages rather than failing. A failed expansion is an
  ACCVIO reported to the process (an AST delivery's a stack fault).
- **Rundown** deletes all of P0 and "a small part of P1": the user
  stack, the image I/O segment, and expansions below them (21.3.1,
  Phase 66's survey).

## The length registers in govax today

| Where | What it does with P0LR/P1LR |
| --- | --- |
| `vm/translate.go` | `translate` (the CPU's), `LookupPTE`, `StorePTE`: an ACCVIO past the CPU's lengths; the TB is consulted *before* the length check |
| `vm/space.go` | `TranslateIn`/`LoadIn`/`StoreIn`/`LookupPTEIn`/`StorePTEIn`: the same check against a `vm.AddressSpace` copy |
| `cpu/context.go` | LDPCTX loads them from the PCB (checking the MBZ bits; 22-bit fields), SVPCTX doesn't save them, `SaveMemoryContext` writes the live ones into the outgoing PCB |
| `cpu/procreg.go`, `machine/set.go` | MTPR/MFPR, and the console's SET of a register (kernel.asm never touches them) |
| `machine/vminit.go` | process 1's: P0LR = /P0, P1LR = 2^21 − /P1, tables written physically before MAPEN |
| `machine/init_vax.go` | INIT zeroes them |
| `machine/show.go` | SHOW MEMORY's region lines (with `Machine.Regions`' sizes and the tables' physical ranges), SHOW REGISTERS |
| `corevms/addrspace.go` | `AdoptAddressSpace` copies the CPU's into process 1's `ProcessSpace`; `BuildAddressSpace` sets a new space's; `mapShared` and teardown read them |
| `corevms/stacks.go` | `InitialPCB` copies a space's into a new process's PCB |
| `corevms/vaspace.go` | `$CNTREG` of P1 reads the CPU's P1LR; `DeleteP0` walks `space.P0LR` |
| `corevms/gblsec.go` | `expandP1` bounds P1 growth by the **CPU's** P1LR (wrong for a process that isn't current, harmless today) |
| `corevms/schedule.go` | the Go switcher: `SaveMemoryContext`, `SaveContext`, `LoadContext` |

`Environment.RegionSize` is the services' own record beside them: [0]
P0's high-water mark (moved by `$CRETVA`, `$EXPREG`, `$CNTREG`, the
heap's `growP0`, the image activators, `mapPage`), [1] P1's low end
(`p1Low`, `expandP1`, `deleteImageP1`), [2] S0's, unused. SHOW REGIONS
shows it. The length registers never move: every table is full size and
every page in it exists from the start.

## How an address space is known

`processForSpace` (filesec.go) finds the process a fault or a frame's
owner belongs to by comparing a `vm.AddressSpace` — all four registers —
with each `Environment.Space.AddressSpace`; `TeardownAddressSpace`
refuses the current space the same way, and `PageFault.Space` is the
CPU's registers at the fault. That works only while the lengths never
change. With created space every change must reach the CPU (when the
process is current), the `ProcessSpace` copy, and the PCB (when it
isn't, or at the next switch) together, or a fault's process is lost.
The bases never move, and P0BR (or the table area's address) is unique
to a process: finding 2.

## The page tables and S0 today

- **VMINIT** maps S0 one-to-one onto frames 0 up and reserves every
  one (`ReservePage`). With vax.init's sizes (16384 frames, `/P0=16384
  /P1=8192 /S0=8192`) its layout is S0 pages 0 to 308: S0's own table
  (64 pages, at physical 0, SBR 0), process 1's P0 and P1 tables (128
  and 64), the kernel, executive, and supervisor stacks with their
  guard pages (38), the interrupt stack (addressed physically), the
  console scratch page, the shim page, the SCB, and the string pool.
  So 256 of its 309 pages are page tables, 192 of them process 1's.
  The microkernel follows (11 pages), then the S0 pool, pages 320 to
  8191, every one with a frame behind it: half of physical memory.
- **Other processes:** `BuildAddressSpace` takes two runs from the pool,
  `tablePages(P0Pages)` and `tablePages(P1Pages)` (the creator's sizes,
  `creprc.go`: VMINIT's for every process, 192 pages), and writes every
  entry (`ProcessPTE`: demand-zero, UW, user's; P0 page 0 NA). With 38
  stack pages and a PCB, 231 pool pages a process, 34 beside process 1.
  `TestPG64Oracle`'s detached process is one of these.
- **The pager already backs S0 on demand**, mechanically: an invalid S0
  PTE whose protection allows the access goes to `System.PageIn` like
  any other, `pagingProcess` gives it no working set, and
  `resolveFault` makes it demand-zero with an untied claim
  (`faultClaim`/`processClaim`: "an S0 page ... is a system page, tied
  to nothing"), which no list, working set, or reclaimer ever takes. No
  S0 PTE is invalid today, so nothing exercises it.
- **S0's size is physical memory's:** `size[2]` gets every frame the
  other sizes leave, and the S0 table is contiguous at physical 0. An S0
  bigger than memory would need S0 backed on demand everywhere; that
  isn't this phase's (finding 5 keeps the tables inside today's S0).

## Physical-address assumptions in S0

Every place that computes an S0 page's frame from its address lies in
VMINIT's own layout, which stays identity-mapped:

- VMINIT itself, which writes the tables and protections through
  physical addresses before MAPEN (`setS0Protection`, the string
  pool's loop: SBR 0, so S0 page N's PTE is at N*4);
- `machine/asm.go`'s `0x80000000 + SCBB` for `.SCB`/`.VECTOR`, and
  the engine's SCB reads (`HandleFault`, MAPEN off), on VMINIT's SCB
  page;
- the interrupt stack, addressed physically.

Everything else goes through a PTE: `AllocateS0` clears each page
through its S0 PTE's PFN (and fails on an invalid one: "S0 page ... is
not mapped"), `allocatePCB` finds the PCB's frame by translating
(`Stacks.PCBB`), `ReadPCB`/`WritePCB`/`SaveMemoryContext` use that
physical address, section and page file I/O move frames by PFN, and
the fetch window is emptied by every PTE write. `XFC$VMR`/`XFC$VMW`
return physical addresses, but nothing uses them. Two displays assume
the identity map: SHOW MEMORY's P0 and P1 "Physical addresses"
(`Machine.Regions`' `pStart`/`pEnd`, the tables' frames) and its
"There are N physical pages mapped" line counts S0's reservations.

## The microkernel's S0 area

vax.init's `ASM` puts the microkernel at `m.s0Free`, the pool's first
page, and `depositAsmImage` writes it (`storeBytes`) and *then* moves
the pool's base past it (`S0Pool.Claim`, refused if a process already
holds a page there). It writes the SCB page (VMINIT's) and the P1
vector (process 1's P1, then `ShareP1`: system frames every process
maps read-only). Nothing else writes S0 outside the layout and the
pool's allocations: no test deposits at a fixed S0 address, and DEPOSIT
or ASM into the pool after the microkernel would land in pages a
process may be given. With the pool backed on demand, its unallocated
pages need a rule (finding 6, question 4).

## The user stack and P1's layout

- **P1 from the top:** the P1 vector (7FFEDE00 up, a few pages, shared
  system frames), then nothing govax uses down to `UserStackTop`
  (7FE00000; 3950 pages that are demand-zero UW today and would be
  charged at a first touch), then the user stack, a reserve of
  `userStackPages` (1024) below it, then P1 expansions (`$EXPREG` of
  region 1, `SEC$M_EXPREG`) from `UserStackTop − 1024 pages` down, as
  far as the table reaches (`MinP1Pages` is 4097).
- **The stack is the process's, not the image's:** the user stack ISD
  (type 253, LINK's `STACK=`) is ignored at activation (Phase 66,
  finding 9), the stack's pages survive from image to image, and
  `deleteImageP1` deletes only the expansions below the reserve.
  Process 1 runs user-mode code from the console (the debugger's CALL,
  GO) with no image at all.
- **Pushes onto the user stack** come from instructions (CALLS, PUSHL,
  ..., and the exception frame of a user-mode CHMU/exception), and from
  Go: AST delivery (`ast.go`'s `enterASTMode`, which stores the frame
  in the target mode), condition dispatch (`condition.go`), callbacks
  (`callback.go`, `pageroutines.go`), the LIB$ routines that build
  frames (`librtl`'s jackets, `filewalk.go`, `deletefile.go`,
  `timers.go`, ...), `$EXIT`'s and the message routines' frames, and
  RUN's IMAGE$INIT driver. All of them store through `vm.Memory` in the
  current mode, so one check in `vm`, where a P1 length violation is
  found, sees every one (finding 9).

## ACCVIO's reason mask

The architecture's access-violation parameters are the virtual address
and a reason mask: bit 0 a length violation, bit 1 a fault on the
process PTE's own reference (the P0/P1 table's S0 page), bit 2 a write
(modify intent). govax's CPU (`cpu/dispatch.go`'s `wrapMemError`) gives
1 for a length violation and **2 for a protection violation**, and never
the write bit; a fault while reading a P0/P1 PTE is reported with the
PTE's S0 address rather than the original address, without bit 1.
`corevms`' `exceptionCondition` builds VMS's signal array correctly
(the mask first, then the address). The LIB$ routines' own
`memoryFault` already uses the architecture's bits, and VMS confirmed
them (`testdata/lib53`: "0 for a read of page 0, 4 for a write"), so a
program's ACCVIO from an instruction and from a LIB$ routine disagree
today. With created space a length violation is common (beyond P0's
end, below the stack), so its mask is what the probe's handlers see.
Logged in docs/DEVIATIONS.md (question 5).

## PGFLQUOTA's first-touch rule

`chargeFirstTouch` (pgflquota.go), called from `resolveFault`'s
demand-zero and copy-on-reference paths (filesec.go, three calls),
charges a page no service created. Today that's the user stack's pages,
whatever ASM or DEPOSIT writes into process 1's P0, the unused P1
above the stack if touched, and the pages tests write from Go. With
created space none of these can exist uncreated, so the rule and its
switch (`Environment.unchargedTouch`, `WithoutPageFileCharge`, used
only by the copying loader, which with paging on no longer runs since
Phase 66) go. The user stack is then charged where it's created.

## Tests that touch P0 or P1 beyond what they created

- **Paging off:** nothing changes (created space is paging mode's), so
  the debugger's and console's tests that ASM or DEPOSIT into P0 at
  will (`loadProgram`, `movR0Program`, and the like) are untouched.
- **Paging on**, 22 console test files and `cmd/govax/stats_test.go`,
  `machine/show_test.go`, `machine/sysgen_test.go`. Most run MACRO
  images, whose pages are created by activation, and stacks, which
  expansion will create. Those that write pages of a bare
  `BuildAddressSpace` from Go would get length violations:
  `workingsetdelete_test.go` (80 P0 pages of a fresh space),
  `pagefiledelete_test.go` (the same), and `pgflquota_test.go`'s
  subprocess test, which *is* the first-touch rule (20 pages charged by
  touching them). They must create their pages first.
- **Counts that will move:** the user stack charged at activation
  (VMS charged 62 pages at an image's first instruction where govax
  charges 24, `expectedPG66Differences`; part of that is the stack),
  frame counts (SHOW MEMORY's, `--stats`', `pagingframes_test.go`)
  once the pool and the tables stop holding frames, and
  `TestPG64Oracle`'s detached process at small memories.
- **The forced runs** (Phase 66's experiment: the whole suite with
  paging on) would reach every debugger and console test that loads
  code into process 1's P0: subtask 7's conveniences are what keep them
  working, and the run is subtask 9's check.

## Findings that change the plan

1. **One record of each region's length.** With paging on, the length
   registers are the region's end, and `RegionSize[0]`/`[1]` become
   their byte forms (P0's end rounded to a page, P1's low end), not a
   separate mark. One function sets a region's length: the
   `ProcessSpace`'s copy, the CPU's register when the process is
   current, its PCB's longwords otherwise (`WritePCB`'s fields, as
   `SaveMemoryContext` writes them). Pages are deleted (each `StorePTE`
   empties its TB slot) *before* a length shrinks, since the TB is
   consulted before the length check. `expandP1` and `$CNTREG` take P1LR
   from the process's space, not the CPU.
2. **An address space is known by its base**, P0BR (the table area's
   address), not by all four registers: `processForSpace`,
   `TeardownAddressSpace`'s check, and anything comparing
   `vm.AddressSpace` values (subtask 2).
3. **Creation extends the region first.** `createPage`, `mapPage`
   (sections and image sections), `CreateImagePages`, `$EXPREG`, the
   heap's `growP0`, and `SEC$M_EXPREG` reach a page through
   `LookupPTE`/`StorePTE`, which fail past the length (SS$_VASFULL
   today). Each extends the region to cover the page (backing the table
   page, finding 5) before writing its PTE; SS$_VASFULL is the virtual
   table's end. Pages a `$CRETVA` beyond the end skips over stay
   nonexistent (PTE 0, no access) inside the new length: unconfirmed,
   a probe case.
4. **Deletion contracts at the ends only.** `$DELTVA` of the region's
   last pages, `$CNTREG`, and image reset lower the length (and give
   back table pages wholly past it); a `$DELTVA` in the middle leaves
   nonexistent pages inside it, as VMS does (unconfirmed for VMS's
   lowering of P0LR by `$DELTVA` at the end: a probe case).
   `DeleteP0` leaves P0LR 0, `deleteImageP1` P1LR at the process's
   permanent P1.
5. **The table area** (question 1): VMS's layout, one run of
   VIRTUALPAGECNT entries per process with P0's table at its bottom and
   P1's at its top, rather than two full-size tables. S0 is small in
   govax: at VMS's 65500 entries one area is 512 pool pages, two
   would be 1024, and the pool has 7872. A table page gets a system
   frame (cleared, so its entries are PTE 0) when a region's length
   first reaches it, before the length register lets the hardware reach
   it (Decision 13), and gives it back when the length leaves it.
   Process 1's area is VMINIT's, `/P0 + /P1` entries. FREPTECNT is the
   room between the two tables.
6. **The S0 pool** (question 4): with paging on, VMINIT backs and
   reserves only its own layout but process 1's table pages; the pool's
   pages start invalid and no-access. `AllocateS0` makes each page it
   hands out a valid system page with a cleared frame and URKW;
   `S0Pool.Free` and `FreeProcess` give the frames back and make the
   pages no-access again. The microkernel's deposit claims its pages
   (`Claim`, which then backs them) *before* writing them, and so does
   any later ASM into S0; a DEPOSIT into an unallocated pool page is an
   ACCVIO. That's neither of the design notes' choices (frames at first
   touch, or a fixed reserve): it keeps the pool knowing every page in
   use, and the console's S0 writes keep working.
7. **SHOW MEMORY** shows the frames that back each region's table,
   not a physical range, and the frame counts move (the pool and the
   tables no longer hold half of memory).
8. **The user stack** (question 2): created at activation from the
   type-253 ISD's page count (LINK's `STACK=`, 20 by default per the
   book's 21.1.1.3), charged, and deleted at rundown with the image's
   other P1 pages; P1 expansions begin directly below the stack's low
   end, as on VMS, with no reserve. The 3950 pages between
   `UserStackTop` and the P1 vector are inside P1's length but
   nonexistent (PTE 0); P1's permanent part is the vector's pages. A
   process with no image (process 1 at the console) has no stack pages
   until it pushes, and expansion makes them.
9. **Stack expansion in one place** (question 3): `vm` reports a P1
   length violation in user mode to a hook (as faults go to the
   `Pager`), and corevms creates the pages from P1's low end down to
   the address (charged; SS$_VASFULL or SS$_EXQUOTA leave the ACCVIO).
   It covers the instructions and every Go path above, since they store
   in the target mode; `$ADJSTK` of the user stack calls it directly.
   Accesses through `TranslateIn` (kernel mode, another process) never
   expand.
10. **FREP0VA, FREP1VA, FREPTECNT, VIRTPEAK** are new `$GETJPI` items
    (and LIB$GETJPI, F$GETJPI by their tables): VIRTPEAK is the most P0
    and P1 pages the process has had at once. Whether FREP0VA and
    FREP1VA are addresses (VSI) or the figure's PTE addresses is the
    probe's question.
11. **ACCVIO's reason mask** (question 5): the CPU's mask becomes the
    architecture's (length 1, PTE reference 2, write 4; 0 for a read
    protection violation), with the original address for a fault on the
    PTE's reference.
12. **The first-touch charge goes** (subtask 6): `chargeFirstTouch`'s
    calls, `unchargedTouch`, `WithoutPageFileCharge`; the three Go-store
    tests create their pages first, `pgflquota_test.go`'s first-touch
    test becomes a creation test.
13. **`vax.sysgen.virtualpagecnt`** (subtask 4): the area's size for
    each new process, in `validConfigs` and HELP CONFIG KEYS. Its
    default is VMINIT's `/P0 + /P1` (24576 with vax.init's), so every
    process keeps today's virtual size and the pool fits as many
    processes as now; VMS's 65500 needs a larger `/S0`. SYI$_
    VIRTUALPAGECNT reports it.

## Questions for the author

Answered by the author on 2026-10-19: each recommendation accepted. The
findings and subtasks follow the answers.

1. **The table area** (finding 5): one run of VIRTUALPAGECNT entries per
   process, P0 up from the bottom and P1 down from the top (VMS's figure
   14-2), with `vax.sysgen.virtualpagecnt` defaulting to VMINIT's
   `/P0 + /P1`? *Recommended: yes.* The alternative is the plan's, two
   tables of VIRTUALPAGECNT entries each, which doubles the S0 each
   process reserves and has no FREPTECNT. **Answer:** yes, one area,
   the key defaulting to `/P0 + /P1`.
2. **The user stack** (finding 8): created at activation from its ISD
   and deleted at rundown, P1 expansions directly below it, no reserve,
   and the unused P1 above `UserStackTop` nonexistent? *Recommended:
   yes, with paging on only* (with it off, today's reserve and
   persistent stack stay). It changes where `$EXPREG` of P1 puts pages
   with paging on (programs see the address in retadr; the probes print
   differences). `UserStackTop` itself stays. **Answer:** yes, with
   paging on only.
3. **Stack expansion** (finding 9): one hook in `vm` for a user-mode P1
   length violation, rather than a check in each Go path that pushes?
   *Recommended: yes.* It expands for a user-mode access anywhere below
   P1's low end, as the book says VMS does. **Answer:** yes, one hook.
4. **The S0 pool's unallocated pages** (finding 6): no access until
   allocated, the microkernel's and ASM's S0 writes claiming their
   pages first, and a console DEPOSIT into an unallocated pool page an
   access violation? *Recommended: yes*, over frames at first touch
   (which the pager could already give, but the pool wouldn't know of
   them) or a fixed kernel reserve. **Answer:** yes.
5. **ACCVIO's reason mask** (finding 11): the architecture's bits from
   the CPU in *both* paging modes? It's ISA fidelity, not paging, and
   clear-cut (VMS's own masks are in `testdata/lib53`), so the
   bug-fixing policy allows fixing it in scope. *Recommended: yes, in
   subtask 2*; a test that checks today's 2 for a protection violation
   changes with it. **Answer:** yes, in both modes, in subtask 2.

## The probe's cases (subtask 9)

`testdata/pg67`, as pg64's and pg66's: a program in parts, a procedure,
counts printed as differences, addresses as offsets from FREP0VA and
FREP1VA where they vary.

- **`$GETJPI`'s items** FREP0VA, FREP1VA, FREPTECNT, VIRTPEAK at the
  first instruction, after `$EXPREG` of P0 and of P1, after `$DELTVA` of
  P0's last pages and of a middle page, after `$CNTREG`, and after a
  `$CRETVA` beyond P0's end (then PROBER of a skipped page, and a read of
  it under a handler: its reason mask).
- **Length violations:** a read and a write one page past P0's end and
  far past it, under a handler printing the signal array (the reason
  mask, the address); PROBER/PROBEW of those pages; a read below P1's
  low end in user mode (expanded?) and in executive mode (`$CMEXEC`:
  not expanded?).
- **Stack expansion:** recursion that uses 100 pages of stack, with
  FREP1VA and PAGFILCNT before and after (one page at a time, charged);
  then `$EXPREG` of P1 and where its pages land against the stack.
- **SS$_EXQUOTA at creation** in a detached process with a small
  PGFLQUOTA: `$EXPREG` and `$CRETVA` stopping partway (retadr), and
  stack expansion when the quota is gone (the ACCVIO).
- **SS$_VASFULL** in a detached process with a small VIRTUALPAGECNT
  (if `$CREPRC` can't set it, SYSGEN on the test VAX): `$EXPREG` at the
  limit, and deep recursion at the limit.
- **Rundown:** FREP0VA, FREP1VA, and PAGFILCNT from DCL (F$GETJPI)
  before and after RUN, and the user stack's size at the first
  instruction (SP against FREP1VA) with the default `STACK=` and with
  `STACK=50`.

## Subtasks (provisional; adjusted by the survey)

1. **Survey** (done 2026-10-19): Part A.
2. **Created space** (findings 1 to 4, 11): with paging on, one record
   of each region's length kept in the CPU, the `ProcessSpace`, and the
   PCB; address spaces known by their base; the services, the heap,
   the image activator, and sections extending and contracting the
   regions; the tables still full size and backed (subtask 4 changes
   that); ACCVIO's reason mask; FREP0VA, FREP1VA, FREPTECNT, VIRTPEAK.
3. **The user stack** (findings 8, 9): created from its ISD at
   activation and deleted at rundown, P1 expansions below it, and
   automatic expansion through the `vm` hook and `$ADJSTK`.
4. **The table area backed on demand** (findings 5, 13): one area per
   process, table pages backed as the lengths reach them;
   `vax.sysgen.virtualpagecnt`.
5. **The S0 pool backed on demand** (findings 6, 7): VMINIT backing only
   its layout in paging mode, the pool's pages backed by `AllocateS0`
   and given back by `Free`, the microkernel's claim before its
   deposit; SHOW MEMORY.
6. **PGFLQUOTA at creation only** (finding 12): the first-touch rule
   removed, with the tests that relied on it.
7. **The console's P0 conveniences** for process 1: ASM and DEPOSIT
   beyond P0's end extend it.
8. **VMINIT's sizes with paging on** (added 2026-10-19 at the author's
   request, after subtask 6): with created space, the table area, and the
   pool all backed on demand, /S0, /P0, and /P1 cost virtual space, not
   frames, and the defaults should say so. The author's aim is practical
   rather than VMS's: leave as many physical frames as possible free for
   programs (and, later, shareable images). With paging on:
   - **S0 apart from physical memory.** VMINIT maps only its own layout
     (the system page table, process 1's table area, the stacks, the
     scratch, shim, and SCB pages), identity mapped as now so the
     interrupt stack stays physically addressable; every pool page starts
     with the all-zero PTE, no access, rather than mapped and then given
     back (`UnbackS0Pool`'s work done at the start). S0's size then has
     no physical limit but the system page table's own frames (one per
     128 S0 pages), which must fit below the layout. S0TOOSMALL stays,
     against the layout's size.
   - **The size rules.** A region of 0 sharing what physical memory is
     left, and leftover pages folded into S0, are physical rules; they
     apply with paging off only. With paging on, 0 takes the
     paging-mode default.
   - **The defaults.** S0 sized for processes, not for memory (the
     author, 2026-10-19: the future has many detached processes, not
     only process 1 and its subprocesses, so the pool must hold many
     processes' tables and stacks at once; a full pool is the limit on
     how many can exist, and should be one users rarely meet): each
     process's pool cost is its table area (VIRTUALPAGECNT / 128 pages)
     plus about 40 pages of stacks and PCB, so 8192 fits about 33
     processes at VIRTUALPAGECNT 24576, 16384 about twice that (128
     system page table frames). P1 about 5120: the 4096 pages from
     UserStackTop to the P1 vector (a nonexistent hole, but inside P1's
     length) plus the stack and room for P1 expansions; P0 the rest of
     the area, whose total is VIRTUALPAGECNT's default. Where the
     defaults live is the subtask's choice: `vax.init` picking paging
     mode's values when `vax.sysgen.paging.enable` is set and the
     `vax.memory.*` keys aren't, or VMINIT's own defaults for regions
     given as 0. With paging off nothing changes.
   - **Showing it:** SHOW MEMORY's frame counts and `--stats`' figures
     after vax.init, and a test that S0 may be larger than physical
     memory with paging on (and is refused with it off).
9. **The full suite with paging on** (Phase 66's forced runs) and **the
   VMS probe** (`testdata/pg67`, the cases above), and govax changed to
   its answers.
10. **Close-out.**

## Progress log

- **2026-10-15:** Planned with Phase 62's review; deferred to the end of
  the program.
- **2026-10-19:** Subtask 1, the survey (Part A): VMS's address space
  from the Internals book (one table area per process header, P0 up and
  P1 down, FREPTECNT the room between; created space; EXE$EXPANDSTK on a
  user-mode P1 length violation; VIRTUALPAGECNT 65500 on the probe
  VAX); every reader and writer of P0LR and P1LR, and
  `Environment.RegionSize` beside them; address spaces matched by all
  four registers (`processForSpace`), which created space breaks;
  VMINIT's layout (256 of its 309 S0 pages page tables) and the pool's
  eager frames; the pager already able to back an S0 page on demand;
  the physical-address assumptions, all inside VMINIT's own layout;
  the microkernel written into the pool before `Claim`; P1's layout and
  the Go paths that push onto the user stack; ACCVIO's reason mask
  (the CPU's 2 for a protection violation and no write bit, where the
  LIB$ routines and VMS agree on the architecture's bits;
  docs/DEVIATIONS.md); the first-touch charge's remaining uses; the
  paging-on tests that write pages they never created. Thirteen
  findings, five questions for the author, the probe's cases, and the
  subtasks adjusted (the user stack its own subtask, the forced suite
  run with the probe).
- **2026-10-19:** The author answered the survey's five questions,
  accepting each recommendation: one table area per process with
  `vax.sysgen.virtualpagecnt` defaulting to `/P0 + /P1`; the user stack
  created at activation and deleted at rundown, P1 expansions below it,
  no reserve, with paging on only; stack expansion through one `vm`
  hook; the S0 pool's unallocated pages no-access, claimed before the
  console's writes; ACCVIO's reason mask fixed in both paging modes in
  subtask 2.
- **2026-10-19:** Subtask 2, created space (findings 1 to 4, 10, 11).
  - **ACCVIO's reason mask** (both paging modes): `vm.TranslationFault`'s
    `Mask` is the architecture's for every kind (`vm.MaskLength`,
    `MaskPTERef`, `MaskWrite`); a fault on a P0/P1 PTE's own reference
    is reported at the program's address with `MaskPTERef`
    (`pteReferenceFault`, in `translate` and `TranslateIn`); and
    `cpu.wrapMemError` passes the mask through, for a translation-not-valid
    fault too. `TestReasonMasks`; `TestConditions_assembledProgram`'s
    MASK is 0 now (a read protection violation); docs/DEVIATIONS.md's
    entry resolved.
  - **Created space** (`corevms/regions.go`), with paging on: VMINIT and
    `BuildAddressSpace` start a space with no P0 pages and P1's
    permanent part (`InitialLengths`, `InitialPTE`: every other entry
    zero), which until subtask 3 is everything from the bottom of the
    user stack's reserve (`userStackBottom`) up. One function sets a
    region's length (`setRegionLength`: the `ProcessSpace`, the CPU when
    the process is current, its PCB through the new
    `cpu.WritePCBLengths` when it isn't, `RegionSize`'s byte form, and
    the peak). Creators reach a page through `lookupPTE`/`storePTE`
    (`replacePTE`), which treat an entry past the length but within the
    table as the zero PTE and cover it (`coverPage`) before writing:
    `$CRETVA`, `$EXPREG` (P0's pages now created as `$CRETVA` creates
    them, owned by the caller's mode, never page 0; P1's by
    `expandP1Pages`), the heap's `growP0`, `mapPage` (sections),
    `mapImagePage` (image sections, for a process that may not be
    current). `expandP1` only finds room in created space; P1's floor is
    the process's table (`p1Floor`), not the CPU's P1LR. Deletion
    contracts: `deletePages` ($DELTVA, $CNTREG) ends with
    `contractRegions`, which passes back over every all-zero entry at a
    region's end; `DeleteP0` zeroes P0 and sets P0LR 0; `deleteImageP1`
    zeroes the image's P1 pages and puts P1LR back at the reserve.
    `vm.AddressSpace.Same` (P0BR) is how a space is known
    (`processForSpace`, `TeardownAddressSpace`, the TB-skip check), and
    teardown walks the P1 table from its own first entry.
    `Environment.EnsurePages` creates the pages Go code is about to write
    (tests' programs and scratch words; subtask 7's console writes can
    use it), doing nothing with paging off.
  - **$GETJPI** (and LIB$GETJPI, F$GETJPI): `JPI$_FREP0VA` (P0's end),
    `FREP1VA` (the page below P1's low end, the book's figure 14-2: P1's
    table maps FREP1VA+200 up), `FREPTECNT` (what the two tables can
    still describe), `VIRTPEAK` (`Process.VirtualPeak`, noted at each
    change; with paging off only the present size is known).
  - **Tests:** `TestCreatedSpace` (process 1's lengths through
    $CRETVA, $DELTVA at the end, $EXPREG of both regions, $CNTREG,
    DeleteP0; the items; the masks past P0's end, below P1's low end,
    and on a skipped page), `TestCreatedSpaceOtherProcess` (a process
    not current: its space and PCB change, the CPU's registers don't),
    `TestCreatedSpacePagingOff`. The paging-on fixtures that wrote pages
    of a bare space from Go (`scheduledConsole`, `handBuiltProcess`,
    `storeLong`'s scratch words, the delete and quota tests,
    `TestShowPage_forms`, `TestMemoryExhaustedNotice`) create them
    first with `EnsurePages`; `TestPageFileQuotaSubprocess`'s 20 pages
    are now charged at creation, the same count. `TestPG64Oracle`'s
    detached run: `$DELTVA` now gives P1's room back (before, Q5's 100
    pages never came back), so the fill takes P1 to within 72 pages of
    its table's end and Q11's request for 210 in P1 meets SS$_VASFULL
    where VMS's meets the quota (`pg64P1End`, listed until subtask 4's
    single table area).
  - **Unconfirmed, for the probe:** FREP1VA as an address (VSI) rather
    than a PTE's; `$DELTVA` contracting a region whose end it deletes
    (`$CNTREG` uses `$DELTVA`'s deletion routine, the book's 16.2.2.3),
    and contracting past entries a `$CRETVA` beyond the end skipped;
    `$EXPREG` of P0's pages owned by the caller's mode; a P1
    `SEC$M_EXPREG` mapping, made lowest page first, lengthening P1 to
    its lowest page at once (a failure partway leaves nonexistent pages
    inside P1).
- **2026-10-19:** Subtask 3, the user stack (findings 8, 9), with paging
  on only (`corevms/userstack.go`).
  - **P1's layout:** a new space's P1 length register is UserStackTop's
    page (`userStackTopPage`), not the bottom of the old 1024-page
    reserve; the entries from UserStackTop up to the P1 vector's lowest
    page (`p1VectorPage`, from `vmsdef.P1VectorTable`) are the all-zero
    PTE, inside P1's length but nonexistent, and the vector's pages up
    are P1's permanent part (`InitialPTE`). P1 never contracts above
    UserStackTop.
  - **Created at activation:** a mapped main image's user stack ISD
    (type 253, LINK's `STACK=`) gives the stack's size, 20 pages without
    one (`defaultUserStackPages`); `CreateUserStack` creates those pages
    below UserStackTop that don't exist, top down, each as `$CRETVA`
    creates one for user mode, charged (`machine/imagemap.go`). P1
    expansions now begin directly below the stack.
  - **Automatic expansion:** `vm.Memory.SetStackExpander` (installed by
    the System, `expandUserStack`) is called by `translate` for a real
    (non-PROBE) user-mode access below P1LR; `expandStack` creates the
    pages from P1's low end down to the address, one at a time, and the
    access goes on; a failure (the quota, the table's end) is the length
    violation, the pages created kept. Instructions and Go code in user
    mode (AST delivery, condition dispatch, LIB$ frames, `CallEntry`)
    all come through it; executive mode, `TranslateIn`, PROBE, and the
    console's kernel-mode reads never expand. `$ADJSTK` of the user
    stack expands to the new pointer first, returning the creation's
    status (SS$_EXQUOTA, SS$_VASFULL) with nothing changed.
  - **Rundown:** `deleteImageP1` deletes from P1's low end up to
    UserStackTop, but for the page the user stack pointer is in and
    those above it (`imageP1Top`, `userStackPointer`: the CPU's SP or
    USP when the process is current, its PCB's otherwise), which hold
    what called the image: a subprocess's CLI stub's frames. At the
    console process 1's user SP is back at UserStackTop when an image
    ends, so its whole stack goes, as on VMS.
  - **Tests:** `TestStackExpansion` (a push and a read below P1's low
    end expand it, charged; PROBE and executive mode don't; the hole
    above UserStackTop has no access, the vector reads),
    `TestStackExpansionQuota`, `TestAdjstkExpands`, `TestStackRundown`
    (the live part kept), `TestUserStackPagingOff`, and
    `TestUserStackImage` (a MACRO image: 20 pages at its first
    instruction, 40 more by expansion, every charge back at its exit).
    Changed: `TestCreatedSpace*` (P1 starts at UserStackTop; `$EXPREG`
    of P1 lands directly below it), `TestMappedImage` (activation charges
    the stack's 20 pages too: 43 at PG66's first instruction, VMS's 62,
    was 24), `TestPagingFrames` and `TestPagingFileSections` (no frame
    is left tied to a process page after an image now),
    `TestDeleteTransitionPages` (records the first range's still-valid
    frames too, which the free list's head may give back first),
    `TestPG64Oracle` (part S gives back a page more than it was charged:
    DCL's SPAWN before it calls LIB$SPAWN in process 1's user mode,
    expanding its stack, which the image's rundown deletes),
    `TestPG66Oracle`'s harness (the console's stack pages, given back by
    R1's rundown).
  - **Unconfirmed, for the probe:** pages created before an expansion
    fails being kept (the book's 16.2.1: services create a page at a
    time); a stray user-mode reference far below the stack creating
    every page down to it, quota and table permitting (the book says
    so); the user stack's size at the first instruction against
    FREP1VA (the probe's `STACK=50` case); the pages between the
    stack's top and the P1 vector being no-access.
- **2026-10-19:** Subtask 4, the table area backed on demand (findings
  5, 13).
  - **One table area** (`corevms.ProcessSpace`): `Entries` (how many
    pages P0 and P1 may have together) and `Table` (the pool allocation,
    0 for process 1) replace `P0Pages`/`P1Pages`/`P0Table`/`P1Table`. P0's
    table starts at the area's bottom (P0BR) and P1's ends at its top
    (P1BR + 2^21 entries is the area's end), so either region grows into
    the room the other leaves: `P0Capacity` is `Entries` less P1's pages,
    `P1Floor` the lowest P1 page with P0 as it is, and FREPTECNT the room
    between, `Entries` less both regions' pages. With paging off the
    formulas give each region's fixed size, as before.
    `BuildAddressSpace(pid, p0, p1)` makes one pool allocation of
    `tablePages(p0) + tablePages(p1)` pages ("page tables") in both
    modes; VMINIT's layout is unchanged with paging off, and with it on
    only P1's table moves, to the end of its last page (the rest of S0
    stays where it was).
  - **Backed on demand** (`corevms/tablearea.go`), with paging on: an
    area page has a frame (an untied system frame, URKW) only while it
    holds the entry of a page inside a region's length
    (`tablePageNeeded`); the others have the all-zero S0 PTE, so a stray
    reference is an access violation, never a fault that would back it
    behind the area's back. `setRegionLength` backs the pages a growing
    length reaches before the registers change (Decision 13: page tables
    never fault) and gives back those a shrinking one leaves after, only
    over the entries the change passes (`changedTablePages`); it reports
    false if no frame is free, which `coverPage` turns into SS$_VASFULL
    (govax's choice; VMS's tables are in the process header and the limit
    would be the working set's room, SS$_INSFWSL; unconfirmed). A new
    space backs only P1's permanent part's 32 pages (`fitTableArea`),
    VMINIT's frames for process 1's other table pages go to the free list
    (`TrimTableArea`), and teardown gives every frame back
    (`releaseTableArea`), leaving the pool's pages without; `AllocateS0`
    now backs a page that has none (the start of subtask 5's pool).
    `DeleteP0` at image exit gives back every P0 table page.
  - **VIRTUALPAGECNT:** `vax.sysgen.virtualpagecnt` (a count key, in
    `validConfigs` and HELP CONFIG KEYS; `Sysgen.VirtualPageCount`),
    VMINIT's `/P0` plus `/P1` when unset (set by VMINIT), and
    SYI$_VIRTUALPAGECNT. With paging on `$CREPRC`'s process gets an area
    of that many entries, split so P1 starts as process 1's does, from
    UserStackTop up (`newSpaceSizes`); with paging off the creator's
    region sizes, as before. P1's 4096 pages from UserStackTop up (most
    of them the nonexistent hole below the P1 vector) count against it,
    so a VIRTUALPAGECNT under about 4100 leaves P0 no room
    (unconfirmed against VMS's FREPTECNT: the probe's question).
  - **Tests:** `TestTableAreaOnDemand` (process 1: 33 of 80 table pages
    backed at the start; a P0 page across a table page boundary backs one
    and its deletion gives it back, as the System frame count shows; P0
    grows past `/P0` to the area's end, every table page backed and
    FREPTECNT 0, where `$CRETVA` and `$EXPREG` of P1 meet SS$_VASFULL; P1
    grows past `/P1` into P0's room; `DeleteP0` leaves only P1's 32),
    `TestTableAreaNewProcess` (32 of 65 at creation, P0 to the area's
    end, teardown frees all 65, `AllocateS0` backs them again, cleared),
    `TestVirtualPageCount` (the key and its default, a `$CREPRC`'d
    process's 72-page area), `TestVMInit_tableArea` (VMINIT's layout in
    both modes, the trimmed frames, the default). Changed:
    `TestDeleteProcessTransitionPages` (its table area's two pool pages'
    frames now go to the free list), `TestBuildAddressSpace`,
    `TestProcessOneAdoptsVMInitTables`, `TestCreatedSpace*`'s FREPTECNT,
    `TestVaxInitMemoryConfig` (sizes from `Machine.VMInitSizes`),
    `TestCreprc_subprocess`. `TestPG64Oracle`: the detached runs' P1 no
    longer ends before the quota (subtask 2's `pg64P1End` and the "Q10
    fill: region 1 stopped" line are gone, with `withoutP1End`); its
    Q11 lines are VMS's.
  - **Left for subtask 5:** SHOW MEMORY's P0 and P1 "Physical addresses"
    still show VMINIT's ranges, which with paging on no longer all have
    frames (finding 7).
- **2026-10-19:** Subtask 5, the S0 pool backed on demand (findings 6,
  7), with paging on.
  - **The pool's pages** (`corevms/s0pool.go`): VMINIT still maps all
    of S0 to frames as it lays S0 out (its S0 loop runs before the
    layout's end is known), then gives back every pool page's frame
    (`UnbackS0Pool`, after `TrimTableArea`), so a pool page has the
    all-zero PTE, no access for any mode, until it's used. `AllocateS0`
    backs each page it hands out with a cleared system frame, URKW
    (subtask 4's `backS0Page`); `FreeS0` and `FreeProcessS0`, which
    every caller now uses in place of `S0Pool.Free`/`FreeProcess` (the
    stacks, the PCB, the table area, process deletion), give the frames
    back first. With paging off nothing changes: the pool's pages keep
    VMINIT's frames for good.
  - **The microkernel:** `depositAsmImage` claims its S0 pages
    (`ClaimS0`: `S0Pool.Claim`, then a frame for each page claimed)
    *before* writing them, where it used to write first and claim after;
    so does any later ASM into S0 (the claim is still refused over pages
    already allocated). A console write (DEPOSIT, EXAMINE) to a pool page
    not in use is an access violation (the author's answer to question
    4).
  - **Frames:** after vax.init with paging on, 167 frames are in use
    (166 system pages: 160 mapped S0 pages, VMINIT's layout less process
    1's unused table pages, the microkernel, and the shim stubs; and the
    P1 vector's frames), where the pool alone used to hold 7873 (half of
    memory); `TestDeleteProcessTransitionPages` is back to its original
    check (every frame back after the process goes, its pool pages' too).
  - **SHOW MEMORY** (`machine/show.go`'s `showPagedRegions`), with paging
    on, for the process the CPU is in: P0's and P1's size is the region's
    length now, "Table frames" the table area's pages holding its entries
    (`System.TableFrames`), and its virtual addresses the pages it has
    ("none" for an empty region); S0's "Mapped pages" are those with
    frames (`System.MappedS0Pages`). The "Physical addresses" lines, which
    no longer meant anything with paging on, remain with it off. HELP
    SHOW MEMORY says so.
  - **Tests:** `TestS0PoolOnDemand` (after vax.init: frames behind the
    microkernel and the pool's allocations only, an unallocated page
    unreadable, `AllocateS0` cleared frames counted as system frames,
    `FreeS0` and `FreeProcessS0` giving them back, the database
    consistent), `TestS0PoolPagingOff`, `TestShowMemory_pagedRegions`.
- **2026-10-19:** Subtask 6, PGFLQUOTA at creation only (finding 12).
  - **The first-touch charge is gone:** `chargeFirstTouch` and its three
    calls in `filesec.go` (a demand-zero fault, a private and a global
    copy-on-reference copy), `Environment.unchargedTouch`, and
    `WithoutPageFileCharge`. With created space every page whose contents
    only memory holds is created, and charged, by something: `$CRETVA`,
    `$EXPREG`, the heap's `growP0`, a copy-on-reference section's mapping,
    a mapped image's writable and demand-zero sections, and the user
    stack's creation and expansion; a fault never charges. The copying
    loader's `loadSection` no longer suspends a charge that can't happen
    (it runs only when images aren't mapped, when nothing is charged).
    `pgflquota.go`'s account of what charges a page says so; the one
    charge still forced past the quota is the system zones' control page.
  - **Tests:** `TestPageFileQuotaSubprocess` is now a creation test: the
    subprocess's 20 pages are charged by `EnsurePages`, touching them
    charges nothing more, and a store to a page nothing created fails;
    `TestPageFileQuotaServices`'s checkpoint 1 expects the user stack
    charged by its creation at activation. The rest of the suite needed
    no change: subtasks 2 and 3 had already made the paging-on tests
    create what they write.
- **2026-10-19:** Subtask 7, the console's P0 conveniences, with paging
  on.
  - **One helper** (`machine/exam.go`'s `ensureConsolePages`): before the
    console writes process 1's memory it creates the P0 pages the write
    reaches that don't exist, through subtask 2's `EnsurePages` (each a
    user-mode demand-zero page as `$CRETVA` makes one, charged, P0's
    length growing to it; pages a write passes over stay nonexistent).
    Its callers are the console's DEPOSIT (`Machine.Deposit`), the
    debugger's DEPOSIT and DEPOSIT/INSTRUCTION (`Machine.WriteBytes`),
    and ASM, the console's and the debugger's, file and interactive
    (`depositAsmImage`'s P0 part). It acts only while the CPU is in
    process 1 and only on P0: a write into another process after SET
    PROCESS, below P1's low end, or into an unallocated S0 pool page
    gets the access check a program would (the S0 rule is subtask 5's).
    With paging off it does nothing.
  - **Tests:** `TestConsolePagesPagingOn` (DEPOSIT past P0's end, a
    longword across two new pages, `WriteBytes`, ASM at 0x200 into a
    page DEPOSIT passed over, the charge, the skipped pages' protection
    violation, a write below P1's low end refused),
    `TestConsolePagesPagingOff`.
  - **Checked by a forced run** (not committed; paging forced on by an
    environment variable): every console and debugger test that loads
    code with DEPOSIT passes. The debugger tests that RUN an image fail
    with SS$_VASFULL at activation: their fixture's VMINIT gives P1 200
    pages, which can't hold the 4096 nonexistent pages above
    `UserStackTop` (subtask 3). That's VMINIT's sizes with paging on,
    for subtasks 8 and 9.
- **2026-10-19:** Subtask 8 added at the author's request: VMINIT's sizes
  with paging on (S0 apart from physical memory, the physical size rules
  paging-off only, defaults that leave frames free for programs); the
  full suite and probe are now subtask 9, the close-out 10.
- **2026-10-19:** Subtask 8, VMINIT's sizes with paging on.
  - **S0 apart from physical memory** (`machine/vminit.go`): with paging
    on VMINIT maps only its own layout, each page to the frame of its
    own number as before; every other S0 page keeps the all-zero PTE
    memory was cleared to, so the pool starts unbacked and
    `UnbackS0Pool` is gone (its work is now never done in the first
    place). `layoutPages` counts the layout (the system page table,
    process 1's table area, the stacks and guard pages, the scratch,
    shim, and SCB pages, the string pool) before anything is written, so
    the checks come before memory is wiped: with paging on, an S0 past
    the architecture's 1 GB or a layout that doesn't fit in physical
    memory is VMTOOLARGE, and a layout that leaves S0 fewer than 32
    pages is S0TOOSMALL; with paging off the old S0TOOSMALL rule stays,
    and an S0 larger than physical memory, which was mapped to frames
    that don't exist, is now VMTOOLARGE. `TestVMInit_layoutPages` holds
    the count to the layout VMINIT lays out, in both modes.
  - **The size rules:** `physicalSizes` (paging off) is the old sharing
    of physical memory among regions given 0 and the leftover folded
    into S0; `pagingSizes` (paging on) gives each region given 0 its
    paging default: P1 5120 (the 4096 nonexistent pages from
    `UserStackTop` to the P1 vector, and 1024 for the stack and P1
    expansions), P0 19456 (with P1 a 24576-entry table area, which stays
    VIRTUALPAGECNT's default), S0 16384 (about 69 processes' table
    areas, stacks, and PCBs at once; 128 frames of system page table).
  - **Where the defaults live:** VMINIT's, for regions given 0. vax.init
    gives VMINIT 0 for each of P0, P1, and S0 the configuration doesn't
    size when `vax.sysgen.paging.enable` is on, asking with a new
    F$CONFIG operation, `BOOLEAN` (1 if the value is one govax reads as
    true, `settings.GetBool`'s rule, which DCL couldn't easily repeat).
    So the numbers are in one place, and VMINIT/PAGING typed at the
    console gets the same ones. With paging off vax.init's sizes are
    unchanged (16384, 8192, 8192).
  - **What it frees:** after vax.init with paging on, 231 frames are in
    use (16153 free of 16384), where subtask 5 left 167 with an
    8192-page S0: the larger S0's system page table is the difference.
    Before Phase 67, the pool and the tables alone held over half of
    memory.
  - **Help:** VMINIT's /P0, /P1, /S0 (the rules in each mode, the
    paging defaults, the errors), CONFIG KEYS' vax.memory.* (the paging
    defaults), and LEXICAL's F$CONFIG (BOOLEAN).
  - **Tests:** `TestVMInit_pagingDefaults`, `TestVMInit_s0BeyondMemory`
    (an S0 eight times memory with paging on: the layout mapped, the
    rest zero PTEs, the pool to S0's end, only the layout's frames
    taken; refused with paging off), `TestVMInit_pagingLimits`,
    `TestVMInit_layoutPages`; `TestVaxInitPagingSizes` and
    `TestVaxInitNoPagingSizes` (vax.init's sizes, SHOW MEMORY, a
    configured region kept); `TestRun_statsPaging` checks `--stats`'
    free and active frames; `TestLexicalConfig` has BOOLEAN. Tests that
    ran VMINIT with paging on, S0 given 0, and a 128-page memory give S0
    96 pages now (the default's 128-page system page table doesn't fit
    in it); `TestVMInit_tableArea` gives S0 the rest of memory, so that
    both modes' S0, and so their layouts, are the same.
  - **Noticed, for the author:** with paging on each process's table
    area backs about 33 frames for P1 from the start, 32 of them holding
    entries of the nonexistent pages between `UserStackTop` and the P1
    vector, which are inside P1's length (subtask 3's layout, VMS's).
    That's most of a new process's table frames. And the forced runs'
    image failures (subtask 7's log: SS$_VASFULL with a 200-page /P1)
    remain for subtask 9: a /P1 under 4097 can't hold P1's permanent
    part with paging on, which VMINIT doesn't refuse.
- **2026-10-19:** Subtask 9, part 1: the whole suite with paging forced
  on, and the probe written (`testdata/pg67`).
  - **The forced runs** (an experiment, not committed, as Phase 66's:
    environment variables that turn VMINIT's paging on whatever it's
    asked, with the paging-off sizes and /P1 raised to the paging
    default, run the PFN database's and the working set's `Check` after
    every fault, and take free frames after VMINIT until 400 or 200 are
    left). No check failed in any run. With paging forced on, every
    console, debugger, and oracle test that runs an image passes; the
    remaining failures are paging-off tests and fixtures (below).
  - **Found and fixed:**
    - **A /P1 too small for paging** (subtask 8's note): VMINIT with
      paging on now refuses a /P1 under 4116 pages, the 4096 from
      UserStackTop up and LINK's default 20-page stack
      (`corevms.MinPagingP1Pages`, `CLI_P1TOOSMALL`; HELP VMINIT). Before,
      every image's activation failed with SS$_VASFULL. The paging-on
      tests that gave /P1 20 or 64 give it the minimum.
    - **A creation the table area can't hold creates nothing**
      (`regions.go`'s `spaceHolds`): `$CRETVA` of a range that would
      take the regions past the area's entries is SS$_VASFULL with
      retadr -1, and so is stack expansion to an address that far below
      P1 (a length violation, nothing created). `TestLib59Oracle` failed
      with paging on: its cases pass 0x55555555 as junk arguments, and
      each user-mode reference to one, a P1 address far below the
      stack, created pages down towards it until the area was full,
      leaving P0 no room, so the C heap's later VM_MALLOC returned 0.
      VMS 7.1 ran the same cases and its heap went on working, so its
      expansion (a `$CRETVA`, the book's 16.2.1.3) can't have spent the
      room: the whole range is checked first (inferred; the probe's A12
      and A13 ask). The paging file quota is still met page by page,
      VMS's own answer (testdata/pg64's Q11). `$EXPREG` already checked
      its room first. `TestCreationNeedsRoom`.
    - **An exception with no user stack to report it on ends the
      image** (`condition.go`'s `noStackToReport`, `lastChanceExit`; the
      engine's `HandleFault` ends the image on `cpu.ErrImageExit` from
      the dispatcher): found by the probe's dry run, where a detached
      process's recursion ran its quota out. The condition dispatch
      couldn't push its arrays, the console's fallback couldn't either,
      and the error stopped the whole run, process 1's image with it.
      Now, as the book's 16.2.1.3 says, the handler search is passed
      over: the condition's message is written as the catch-all writes
      it, and the image ends with the condition (STS$M_INHIB_MSG) as its
      status, its exit handlers not run (there's no stack for them).
      Only in created space, in user mode.
  - **Fixtures:** `scheduledConsole` and `handBuiltProcess` now create
    P0 from `codeAddr` to `fixtureP0End` (^X4000, `programExtent`),
    which every test program's code and data fit in (the counter at
    `dataAddr` was on a page they didn't create, so with paging on a
    hand-built process died at its first store); `newSecondProcess`
    gives P1 the paging minimum. With them the scheduler, mailbox, RMS,
    shared-file, terminal, and stress tests pass with paging forced on.
    `TestUserStackImage` and `TestPG66Oracle` count the fixture's pages
    among the harness's charges.
  - **What else failed, and why** (all expected): the paging-off tests
    and VMINIT's layout tests (sizes, physical frames, S0TOOSMALL with
    the bigger /P1); tests whose fixtures write P0 pages from Go that
    nothing created (`TestSetprt*`, the lock tests, `TestClearTB`,
    `TestShowPage*`, `TestShowTB*`, `TestSetPTE*`,
    `TestHandSwitchTwoProcesses`, `TestGlobalSectionPair_spinlock`'s data
    at ^X6000, a hand-built process's user stack pushed onto from Go in
    `TestSchedulerRun_otherImageEnds`); the `$CRMPSC` tests' paging-off
    expectations (a page-file section's page valid at mapping);
    `TestRun_fileSections`' step 32, which expects a P1 `$EXPREG` below
    a P1 section the program deleted, where created space gives the
    addresses back (the probe's A4 asks VMS); and, as Phase 64 found, the
    LOGOUT reports' peak and `TestLkwsetFull`. At 400 free frames, the
    tests of the frame counts, PG64's writer timing, `TestCreprc_limits`
    (its filler takes a frame a pool page), and PG64's detached process
    (MEMEXHAUST); at 200, the oracles whose images' working sets (512
    pages by default) don't fit: govax has no free page wait or swapper
    (Decision: paging is synchronous), which Phase 66's forced runs
    avoided with small working sets.
  - **The probe** (`testdata/pg67`, README.md): one program linked twice
    (PG67S.EXE with `STACK=50`), in parts: A (the items at the first
    instruction; `$EXPREG`, `$DELTVA` of each region's end and of a
    middle page, `$CNTREG`, `$CRETVA` past P0's end; references past
    P0's end and below P1's low end from user and executive mode, with
    their masks and PROBE; `$CRETVA` of more than FREPTECNT pages, and a
    read further below than that), S (recursion 100 pages deep, then a
    P1 `$EXPREG`), T (the stack's size at the first instruction, both
    images), C (part Q in a detached process with a small quota and a
    termination mailbox: the quota nearly filled, then recursion past
    it). VIRTUALPAGECNT can't be set for one process, so SS$_VASFULL at
    a small one isn't asked. `TestPG67DryRun` runs it on govax with
    govax's own links; the README lists govax's answers.
  - **Noticed, not changed:** the catch-all (and now the last chance
    exit) writes a condition's message to the console for every process,
    where VMS writes it to the process's SYS$OUTPUT and SYS$ERROR, so a
    detached process's message isn't in its output file.
  - **Next:** the VAX run (the README's steps), then govax changed to
    its answers and `TestPG67Oracle` in place of the dry run.
- **2026-10-19:** Subtask 9, part 2: the probe's VMS run
  (`testdata/pg67/vax`; README.md's "The VAX run"), OpenVMS V7.1 on
  simh's VAX 8600. `TestPG67Oracle` (in place of the dry run) runs VMS's
  own two images on govax as PG67.COM did and matches VMS's lines,
  VIRTPEAK masked, but for `expectedPG67Differences`.
  - **Confirmed** (the progress log's unconfirmed items): FREP0VA and
    FREP1VA as addresses, FREP1VA the page below P1's low end;
    `$DELTVA` of a region's end shortens it, P1's too, and a later P1
    `$EXPREG` reuses the addresses (so `TestRun_fileSections`' step 32,
    a paging-off test, holds only with paging off); `$CRETVA` beyond
    P0's end lengthens P0 over the pages it skips, which are protection
    violations (mask 0); the masks past P0's end; PROBE never expands;
    user-mode references below P1's low end create every page down to
    them, charged, kept after the code returns, and executive-mode ones
    don't; the stack's size at the first instruction is the image's
    STACK= (30 pages more for STACK=50); a stray reference further below
    than FREPTECNT creates nothing; an exception with no stack to report
    it on ends the image with the condition, no handler run (part Q).
  - **Changed:** only P1 creation checks the room for the whole range
    first (`spaceHolds`): VMS's `$CRETVA` of FREPTECNT+100 P1 pages made
    none, but in P0 made its pages one at a time until the quota stopped
    it. `$CNTREG`'s retadr is in `$EXPREG`'s order, the reverse of the
    deletion's (`TestCntreg`). The last chance exit writes no message.
    F$GETJPI writes FREP0VA and FREP1VA as it writes JPI$_OWNER (eight
    hexadecimal digits, "" for 0), though the DCL Dictionary calls them
    integers.
  - **Differences that stay:** the first instruction's lines (the
    activation's stack use, FREPTECNT, the job's charges), A12's P0 count
    (the quota left), and A13's address (FREPTECNT's distance).
  - **Not asked:** SS$_VASFULL at a small VIRTUALPAGECNT (a SYSGEN
    parameter needing a reboot); the item forms of FREP0VA and FREP1VA
    in LIB$GETJPI, left numbers.
