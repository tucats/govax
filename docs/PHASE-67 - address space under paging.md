# Phase 67 — the address space and page tables under paging

**Status:** in progress: subtask 1, the survey, done 2026-10-19 (Part A
below; its questions answered by the author the same day, each
recommendation accepted). Planned 2026-10-15,
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
  working, and the run is subtask 8's check.

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

## The probe's cases (subtask 8)

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
8. **The full suite with paging on** (Phase 66's forced runs) and **the
   VMS probe** (`testdata/pg67`, the cases above), and govax changed to
   its answers.
9. **Close-out.**

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
