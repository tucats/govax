# Phase 62 — VMS paging: the plan, the decisions, and the groundwork

**Status:** planned (2026-10-15). Part A (the survey) and Part B (the
decisions) are written for the author's review; nothing is implemented
yet. Part C splits the work into Phases 62 to 67. Needs Phases 43–49
(processes, the scheduler, sections) and 52.

Today govax translates virtual addresses faithfully, but a page that
isn't valid has only one fate: the first touch gives it a physical page 
(a zeroed one, or one read from a section's file), and from then on it
stays in memory until the page is deleted. Nothing ever takes a page
away from a process. There is no working set, no list of pages waiting
to be reused, and no page file. Physical memory is spent once and only
comes back through `$DELTVA`, a section going away, or process deletion.

This program gives govax VMS's paging: per-process working sets with
VMS's limits, a free page list and a modified page list whose pages keep
their contents and can be faulted back, a page file
(`PAGEFILE.SYS`) as backing store, a modified page writer, and fault
accounting per process. It also moves the section services and image
activation onto the pager, so that, as on VMS, an image's pages come
from its file when they're first touched.

Fidelity matters in **how page faults are handled**: which faults
happen, where the page comes from, what's counted, what the services
report. The page file's layout and the pager's data structures don't
have to be VMS's.

## Sources

- ***VAX/VMS Internals and Data Structures*** (usable in full under
  `PHASE-43.md`'s Decision 6; OCR text
  `~/Documents/Technical Doc/VMS/vms-internals-data-structures.txt`):
  chapter 14 (memory management data structures: 14.1.1's invalid PTE
  forms, figure 14-3; 14.1.2's working set list; 14.2's PFN database;
  14.3's global page table), chapter 15 (paging dynamics: 15.2's private
  page transitions, figures 15-3 to 15-5; 15.3 global pages; 15.4
  working set replacement; 15.5 paging I/O, clustering, the modified page
  writer and page file allocation; 15.6 paging and scheduling), chapter
  16 (the memory management services: 16.2 creation and deletion, the
  page file quota, automatic stack expansion; table 16-1, the working
  set limits), chapter 20 (process creation, the quota table 20-3), and
  chapter 21 (image activation and image reset).
- **VMS 5.0 System Services Reference Manual** and VSI's public system
  services text (`VSI-system-services-1.txt`, `-2.txt`): `$ADJWSL`,
  `$PURGWS`, `$LKWSET`, `$LCKPAG`, `$CRETVA`, `$EXPREG`, `$DELTVA`,
  `$CRMPSC`, `$UPDSEC`, `$CREPRC`'s quota list (PQL$_WSDEFAULT,
  WSQUOTA, WSEXTENT, PGFLQUOTA, each with its PQL_Dxxx default and
  PQL_Mxxx minimum), and `$GETJPI`'s and `$GETSYI`'s memory items.
- **DCL Dictionary** (VMS 7.3-1): SHOW MEMORY, SHOW WORKING_SET,
  SET WORKING_SET, SHOW PROCESS/CONTINUOUS's fields.
- **VMS SYSAdmin I and II** (in the same folder; not yet read): SYSGEN
  CREATE of a page file and the parameters' defaults.
- **Real VMS output**, from probes on the simh VAX as earlier phases did:
  the subtasks list them.

# Part A — the survey

## What govax does today

**Translation and faults** (`internal/vm`). `translate.go` walks the
page tables; an invalid PTE whose protection allows the access goes to
`pager.go`'s `pageIn`, *inside* the translation, synchronously. With no
`Pager` installed the page is demand-zero (`validatePage`); with one,
the `Pager`'s `PageIn` returns the new valid PTE. No real
translation-not-valid exception reaches VAX code unless the pager fails.
`space.go`'s `TranslateIn`/`LoadIn`/`StoreIn` reach a process that isn't
current and fault the same way. Writing a PTE (`StorePTE`,
`StorePTEIn`) empties that TB slot and the STC, and so the
instruction-fetch window, which holds a physical address.

**Physical memory** (`vm/memory.go`). `Memory.pageMap` is one bool per
frame. `AllocatePage` hands out the lowest free frame; `FreePage` clears
the frame's contents and marks it free (so a frame handed out is always
zero). There is no list order, no record of who owns a frame, and no
notion of a frame that's free but still holds a page someone might want
back.

**VMINIT** (`machine/vminit.go`). S0 is mapped one-to-one onto physical
memory from frame 0 and every S0 frame is reserved at once; physical
pages left over after the regions' sizes are folded into S0. With
`vax.init`'s defaults (16384 frames, `/P0=16384 /P1=8192 /S0=8192`)
S0 takes 8192 frames, and about 8000 frames remain for every process's
P0 and P1 pages and the global sections. Process 1's P0 and P1 tables
are built at full size, every entry `corevms.ProcessPTE`: demand-zero,
UW, user-owned (P0 page 0 no-access). So in govax **every page of P0 and
P1 exists from the start**; `$CRETVA`/`$EXPREG` only rewrite PTEs and
move `RegionSize`.

**Other processes' address spaces** (`corevms/addrspace.go`,
`s0pool.go`). `BuildAddressSpace` takes full-size P0 and P1 tables from
the S0 pool (with the defaults, 192 pages of page table per process, all
of them already backed by frames) and fills them the same way. The P1
vector's pages are mapped onto process 1's frames (`mapShared`).

**The pager** (`corevms/filesec.go`'s `System.PageIn`, Phase 49). By the
invalid PTE's form: a global page table index (`pageInGlobal`: the
section's frame, read from the file at first touch and then held by the
section for good; a private copy for copy-on-reference), a process
section table index (read the block into a new private frame; the valid
PTE no longer says which section, `processSectionAt` finds it by
address), or demand-zero. `releasePTE` is the one place a page leaving a
PTE is accounted for (a modified private section page written back, a
global page's reference dropped, any other frame freed); `releaseFrame`
asks the global sections first. `writePage`/`readPage` move one page to
or from a file. The file comment already describes this as "the seam a
working set manager and a page file would use".

**Global sections** (`gblsec.go`). A page-file section
(`SEC$M_PAGFIL`) allocates and zeroes all its frames when it's created,
and a mapping process's PTEs are valid from the start. A file section's
pages come in through the global page table (Go:
`GlobalSections.pageTable`) and stay. `Frames`, `Dirty`, `Refs` are
per section.

**Fault accounting.** `countFault` adds each resolved P0/P1 fault to
`Process.PageFaults` (JPI$_PAGEFLTS, LIB$STAT_TIMER's and SHOW_TIMER's
count). S0 faults are nobody's.

**Quotas** (`corevms/quotas.go`, `process.go`). `quotaRules` holds the
PQL defaults and minimums, from the VMS 7.1 system's SYSGEN SHOW/PQL
(`testdata/mp/probe2/vax`): WSDEFAULT 294, WSQUOTA 588, WSEXTENT 16400,
PGFLQUOTA 16400 (pooled, in the job's `Job.Pooled`). `$CREPRC` copies
them into each new process (`applyQuotas`, which adds VMS's 4 pages).
Process 1 has nominal values (`nominalWSDefault` 512, `nominalWSQuota`
1024, `nominalWSExtent` 16400, `nominalMinWSCount` 20). All are recorded
and reported, **none enforced**. `$ADJWSL` moves `WSLimit` within
[MINWSCNT, WSEXTENT]. `$LKWSET`, `$ULWSET`, `$LCKPAG`, and `$ULKPAG` record
locks in a map and do nothing else (`pageprot.go`). `$PURGWS` isn't
implemented. `$GETJPI` has DFWSCNT, WSQUOTA, WSEXTENT, WSAUTH,
WSAUTHEXT, WSSIZE (= WSLimit), PGFLQUOTA, PAGEFLTS; it lacks PPGCNT,
GPGCNT, WSPEAK, VIRTPEAK, PAGFILCNT, FREP0VA, FREP1VA, FREPTECNT.
`$GETSYI` has no memory items.

**The scheduler** (`internal/sched`) already defines PFW, FPG, and COLPG
("unused: no paging").

**Image activation** (`machine/image.go`). `ReadImage` reads the whole
image file into a Go slice: from a mounted volume (`ReadRawFile`) or
from the host. `imageLoad` copies the header blocks to P0 at
`RegionSize[0]+0x200`, then every page of every process-private ISD
with `storeBytes` (each one a demand-zero fault followed by a copy);
DZRO ISDs get explicit zeros. Fixups (`imageFixup`) write into the
pages in place. Afterwards `setImageProtection` sets each section's
protection, **only on pages that are valid**. Global ISDs are skipped;
shareable images come from the host, loaded privately into each
process. **Image rundown doesn't delete P0**: `ResetICBList` sets the
old image's pages back to UW so the next RUN can load over them, and the
pages (and their frames) stay valid indefinitely.

**What holds a physical address** across a translation: the fetch
window (safe: emptied by any PTE write), `XFC$VMR`/`XFC$VMW` (returns a
physical address to VAX code; nothing uses it today), section page I/O
(`readPage`/`writePage`, safe), and the PCB (S0, never paged).

## How VMS pages (what to be faithful to)

**Invalid PTE forms** (figure 14-3; bit 26 is TYP0, bit 22 TYP1; bits
31, 30–27, 24–23 are valid, protection, owner in every form):

| TYP0 | TYP1 | Rest | Meaning |
|---|---|---|---|
| 0 | 0 | PFN = 0 | demand-zero page |
| 0 | 0 | PFN ≠ 0 | transition: the page is in that frame, on the free or modified list (or I/O in progress) |
| 0 | 1 | GPTX | global page: see the global page table entry |
| 1 | 0 | PSTX | image or section file page: see the process section table entry |
| 1 | 1 | page file VBN | the page is in the page file (VBN 0: a block will be allocated but hasn't been) |

govax's `vm.PTE.SectionIndex` takes TYP0 alone; it must require TYP1
clear once the page file form exists.

**The PFN database** (14.2): per frame, its state (active, on the free
list, on the modified list, read or write in progress, release pending),
the PTE that maps it, its backing store address (BAK), a reference count,
a share count for global pages, and the saved modify bit.

**Lists.** The *free page list* holds clean frames: their contents are
already on backing store, or never need to be. The *modified page list*
holds frames whose contents must be written before the frame can be
reused. A page removed from a working set keeps its frame. The PTE goes
to the transition form, and the frame goes to the tail of the free list
or the modified list, by the saved modify bit. A fault on a transition
page is a **soft fault**: the frame is taken off its list and the PTE
made valid again, with no I/O. A frame taken from the head of the free
list for a new use breaks the tie: the old PTE is set back to its
backing store form (PSTX, page file VBN, GPTX). Deleted pages go to the
*head* of the free list, to be used first.

**Working sets** (14.1.2, 15.4, table 16-1). Each valid process page is
in the process's working set list. On a fault that needs a new entry,
the pager takes a free entry if the working set is below its limit and
the growth rule allows: always below WSQUOTA, up to WSEXTENT while the
free list has more than GROWLIM pages (and the list itself grows past
WSQUOTA only while there are more than BORROWLIM). Otherwise it replaces
the oldest entry (FIFO from WSNEXT), skipping up to TBSKIPWSL entries
still valid in the TB, and skipping locked entries. WSLIMIT starts at
WSDEFAULT for each image; `$ADJWSL` moves it, and automatic adjustment
(PFRATL/PFRATH, WSINC/WSDEC, AWSTIME) can too. `$PURGWS` removes a range
of pages; `$LKWSET` locks pages into the working set; `$LCKPAG` locks
them in memory.

**Backing store.** A page from an image or section file that isn't copy
on reference goes back to that file if it's writable and modified. A
copy-on-reference page and a demand-zero page are "effectively
modified" from their first fault (saved modify bit set). Their backing
store becomes the page file, and their first removal sends them to the
modified list. The **modified page writer** wakes when the modified list
exceeds MPW_HILIMIT and writes until it falls to MPW_LOLIMIT. It clusters
up to MPW_WRTCLUSTER pages, allocates page file blocks in runs, and moves
written pages to the free list, where they can still be faulted back
softly. A page file that's 65% or 90% full gets SYSTEM-W-PAGEFRAG or
PAGECRIT on the console.

**Page file quota** (16.2.1.1). Each demand-zero page and each
copy-on-reference page is charged to the job's PGFLQUOTA *when the
virtual page is created*. That's why `$CRETVA` and `$EXPREG` can fail
with SS$_EXQUOTA. Deletion gives the charge back.

**Global pages** (15.3). The process PTE holds a GPTX until the process
faults the page. If the global page is valid in another process's
working set, the fault is soft: the share count goes up and the PTE
points at the same frame. When the last working set lets go, the frame
goes to the free or modified list with the *global* page table entry in
transition.

**Clustering** (15.5.1). A hard fault reads neighboring pages with the
same backing store too, up to the section's PFC (or PFCDEFAULT).

**Wait states** (15.6). PFW while a read is in progress, FPG when there
is no free frame, COLPG when the page is already being read for someone
else.

**Image activation and reset** (21.1, 21.3; 16.2). The image activator
doesn't read the image. It makes a process section table entry for each
private ISD and fills the PTEs with its PSTX (writable sections copy on
reference), DZRO sections with demand-zero PTEs, and global sections
with GPTXs. The pages come in through faults. Image reset deletes all of
P0 and part of P1.

**Swapping** (chapter 17) removes whole processes from memory. That's out
of scope; see "Future work".

## Your request, in VMS's terms

- *"A free list of pages de-assigned from an address space, which could
  be re-assigned with content unchanged"*: VMS's free page list and
  modified page list, together. Pages on either keep their contents and
  their transition PTEs, and a fault takes them back.
- *"If the free list gets too large, pages are moved to a pagefile"*:
  in VMS it's the **modified** list that triggers writing (above
  MPW_HILIMIT). A clean page on the free list needs no write: its
  contents are already in a file, or it was never written. After the
  write a page moves to the free list, still faultable, and its PTE
  learns its page file block only when its frame is reused. The plan
  follows VMS (Decision 7); see open question 2.
- *"When a TNV fault occurs, look where the page is and reassign it"*:
  the PTE's form says where it is (table above). For a transition page
  the PFN database says which list it's on.
- *"Accounting per process"*: Decision 12.
- **The settings.** Three of the names match VMS's SYSGEN parameters:
  `PQL_DWSQUOTA`, `PQL_DWSEXTENT`, `PQL_DWSDEFAULT`. The fourth should be
  **`pql_dpgflquota`**, not `pql_mpgflquota`. In VMS the D parameters are
  the defaults `$CREPRC` gives a process, and the M parameters are the
  minimums a quota list can't go below (`quotas.go` already has both). It
  limits the *page file pages* a process's job may use (demand-zero and
  copy-on-reference pages), not the pages it may have. It's pooled: one
  count per job, which is how govax keeps it already.

# Part B — decisions (proposed, for review)

**Decision 1 — paging is a mode, chosen at VMINIT.** With
`vax.sysgen.paging.enable` false (the default) govax behaves exactly as
it does today: the same allocator, the same pager, the same counts. Every
existing test and oracle keeps passing unchanged. With it true, VMINIT
installs the new frame manager and pager. A VMINIT qualifier
`/[NO]PAGING`, defaulting to the setting, lets a procedure or test
choose. `vax.init` passes the setting like the `vax.memory.*` keys.
The mode can't change without a new VMINIT, because VMINIT is when
physical memory is wiped and the tables are built.

**Decision 2 — the pager's bookkeeping is a leaf package,
`internal/mmg`** (VMS's facility prefix for memory management). It's
like `internal/sched`, `internal/lck`, and `internal/vmheap`: plain Go, no
VAX dependency, tested on its own with a `Check` of its invariants after
every operation. It holds:

- the PFN database (one entry per frame: state, owner as an opaque
  handle and virtual address, backing store, reference and share counts,
  the saved modify bit);
- the free and modified lists;
- each process's working set list and its limits;
- the page file's block map;
- the policies (growth, replacement, reclaim, writer thresholds).

It reaches PTEs and backing store through small interfaces that
`internal/corevms` implements: reading or writing a PTE in a given
address space, reading or writing a frame, and the page I/O. corevms
remains the glue: `System.PageIn` (the `vm.Pager`), the section tables,
the services.

**Decision 3 — what lives in VAX memory.** The page tables, in every PTE
form above (transition and page file PTEs included), and the pages
themselves: a page on the free or modified list is a real frame of VAX
memory, still holding its contents. Everything that describes them lives
in Go: the PFN database, the list links, working set lists, the process
section table, the global page table, and the page file map. This is how
I read *"only the free-list of pages and the virtual address space maps
must live in VAX memory"*; see open question 1.

**Decision 4 — VMS's PTE forms, bit for bit**, as in the table above.
`vm` gains `TransitionPTE`/`PFN` on an invalid PTE, `PageFilePTE`/
`PageFileBlock`, and `SectionIndex` requires TYP1 clear. With paging
off, none of the new forms is ever written, so the fallback "any other
invalid page is demand-zero" stays correct.

**Decision 5 — faults are resolved synchronously, inside the
translation, as they are now.** Disk I/O in govax completes at once, so a
hard fault reads the page from the page file or image file in Go and the
instruction carries on. There's no PFW, no COLPG (no read is ever in
progress when another fault comes), and FPG is avoided by reclaiming
(Decision 7). This keeps paging deterministic in quantum-clock mode.
Faults are counted as soft or hard either way. The alternative was a
real TNV exception that backs out the instruction, with PFW until a
simulated read completes. It would let other processes run during page
reads, but it costs a fault path through the CPU and the scheduler and
gains nothing visible to programs (open question 4).

**Decision 6 — working set replacement is VMS's modified FIFO.**

- **Growth:** an entry is free to use below WSLIMIT. The limit grows
  past WSQUOTA toward WSEXTENT while the free list has more than
  BORROWLIM pages, and a new page may be added past WSQUOTA while it has
  more than GROWLIM.
- **Replacement:** the oldest entry goes first. Up to TBSKIPWSL entries
  still valid in the TB are skipped (govax's TB answers that), and
  entries locked by `$LKWSET` always are.
- **Instruction safety:** pages translated during the current
  instruction are never chosen. An instruction touches several pages; the
  CPU re-translates each access, so evicting one mid-instruction would
  only cost a refault. But a Go routine that translated two pages before
  copying, or a working set smaller than an instruction's pages, must
  not livelock or read a stale frame. `vm` keeps the set of frames
  translated since the last instruction boundary, and MINWSCNT
  (default 20) bounds WSLIMIT from below.
- **Image exit** resets WSLIMIT to WSDEFAULT.
- **Automatic adjustment** (PFRATL/PFRATH) is left out at first: a
  later subtask, if a probe shows programs depend on it.

**Decision 7 — two lists, VMS's rules, and a synchronous writer and
reclaimer.**

- **Removal:** a page leaving a working set goes to the tail of the free
  list (clean) or the modified list (dirty, or "effectively modified").
  A deleted page's frame goes to the head of the free list.
- **Allocation:** a new frame comes from the head of the free list,
  breaking its old tie. Demand-zero frames are zeroed *when allocated*;
  today they're zeroed when freed, which would destroy the free list's
  contents.
- **The modified page writer** runs when an operation leaves the modified
  list above MPW_HILIMIT, and writes down to MPW_LOLIMIT in clusters of up
  to MPW_WRTCLUSTER: to the page file, or to the section file for a
  writable non-CRF section page.
- **The reclaimer:** when the free list falls below FREELIM, it first
  runs the writer, then trims working sets above WSQUOTA (the processes
  waiting longest first), toward FREEGOAL. This is the part of the
  swapper's job govax needs.
- **Both run inside the call that crossed the threshold**, in Go: their
  I/O completes at once, so there's no need for a separate process.
- **Without a page file** (none configured, or before Phase 64), dirty
  pages that belong in the page file stay on the modified list. If it
  fills memory, a fault fails as it does today when memory runs out.

**Decision 8 — every frame's backing store is one of:** none
(demand-zero, never written), the page file (a block, or "one will be
allocated"), a process section (image file or `$CRMPSC` file: the PSTE
and the page's block), or a global section page (GPTX). A
copy-on-reference page takes the page file as backing store at its first
fault, as on VMS.

**Decision 9 — the page file is govax's own format**: block n holds one
512-byte page, and the allocation map is in Go, rebuilt empty at each
start (a page file's contents never outlive the session).

- **Where:** `vax.sysgen.paging.file` names it, as a volume file
  (`DUA0:[WORK]PAGEFILE.SYS`) or a host file (`"./pagefile.sys"`), with
  `vax.sysgen.paging.size` blocks. It's created contiguous if it's
  missing, and used at its own size if it exists.
- **When it's opened:** at first need, not at VMINIT. The default volume
  is mounted after `vax.init` runs, and a page file that can't be opened
  gets one console message, after which paging goes on without one
  (Decision 7).
- **I/O:** blocks go straight through the file's block I/O
  (`ods2`'s `File.ReadBlock`/`WriteBlock` for a volume file, `ReadAt`/
  `WriteAt` for a host one), never through RMS's record path. That is
  VMS's pager doing I/O through its window on the page file. It also
  avoids re-entering RMS when a fault happens in the middle of an RMS
  operation that is copying into a user buffer. That reentrancy needs an
  audit in Phase 64: does any ods2 call hold state while govax writes VAX
  memory?
- **Holding the volume:** DISMOUNT refuses a volume that holds the open
  page file.
- **Messages:** at 65% and 90% full, PAGEFRAG and PAGECRIT, once each.
- **A full page file** stops the writer, so dirty pages stay on the
  modified list.

**Decision 10 — PGFLQUOTA is charged per page that has, or will have,
page file backing**, and given back when the page is deleted. VMS charges
at creation. Until Phase 67 gives govax created-space semantics, every
P0/P1 page "exists" from the start, so the charge is made at the page's
first fault (demand-zero or CRF). `$CRETVA`, `$EXPREG`, and `$CRMPSC` of a
CRF section check that the pages they create, plus what's charged, fit,
and return SS$_EXQUOTA if they don't, as VMS's do. A first-touch charge
never fails a fault: it may exceed the quota, and JPI$_PAGFILCNT shows
it. Phase 67 moves the charge to creation and drops the first-touch rule
(one function, `chargePageFile`).

**Decision 11 — the settings** are `vax.sysgen.*` keys (each in
`cmd/govax/main.go`'s `validConfigs` and in `HELP CONFIG KEYS`), read into
a `Sysgen` struct on the `System`. `quotaRules`' defaults and process 1's
nominal values come from it, so **each process gets its own copy at
creation** (`applyQuotas`, as today), ready for a later per-process
SET WORKING_SET or `$CREPRC` quota list to change.

| Key | VMS name | Default (unconfirmed unless noted) | Meaning |
|---|---|---|---|
| `vax.sysgen.paging.enable` | — (govax) | false | paging on (Decision 1) |
| `vax.sysgen.paging.file` | — (govax) | none | page file: volume or host path |
| `vax.sysgen.paging.size` | (SYSGEN CREATE /SIZE) | 32768 blocks | page file size if govax creates it |
| `vax.sysgen.pql_dwsdefault` | PQL_DWSDEFAULT | 294 (VMS 7.1's) | a new process's WSDEFAULT |
| `vax.sysgen.pql_dwsquota` | PQL_DWSQUOTA | 588 (VMS 7.1's) | its WSQUOTA |
| `vax.sysgen.pql_dwsextent` | PQL_DWSEXTENT | 16400 (VMS 7.1's) | its WSEXTENT |
| `vax.sysgen.pql_dpgflquota` | PQL_DPGFLQUOTA | 16400 (VMS 7.1's) | its job's PGFLQUOTA |
| `vax.sysgen.minwscnt` | MINWSCNT | 20 | least WSLIMIT |
| `vax.sysgen.freelim` | FREELIM | 32 | reclaim below this |
| `vax.sysgen.freegoal` | FREEGOAL | 200 | reclaim up to this |
| `vax.sysgen.growlim` | GROWLIM | 63 | free pages needed to grow past WSQUOTA |
| `vax.sysgen.borrowlim` | BORROWLIM | 300 | free pages needed to extend the list past WSQUOTA |
| `vax.sysgen.mpw_hilimit` | MPW_HILIMIT | 500 | writer starts above this |
| `vax.sysgen.mpw_lolimit` | MPW_LOLIMIT | 120 | writer stops at this |
| `vax.sysgen.mpw_wrtcluster` | MPW_WRTCLUSTER | 96 | pages per write |
| `vax.sysgen.pfcdefault` | PFCDEFAULT | 32 | read cluster |
| `vax.sysgen.tbskipwsl` | TBSKIPWSL | 8 (the book's) | TB-valid entries skipped |

The minimums (PQL_Mxxx) stay `quotaRules`' table values. The defaults
marked unconfirmed should come from SYSGEN SHOW on the simh VAX (a
subtask asks for that run). Only the first four rows were in your
request; the rest are the parameters VMS's pager reads. Each is optional
and keeps its default when unset.

**Decision 12 — accounting.**

- **Per process:** faults (all, as JPI$_PAGEFLTS counts them, which
  includes soft ones once paging is on), hard faults (page reads),
  pages read and written, working set size and peak, and process and
  global page counts.
- **Per job:** page file pages charged.
- **System-wide:** frames free, modified, and in use; the page file's
  blocks used; the writer's writes; soft and hard faults.
- **Where they show:** `$GETJPI` and F$GETJPI gain PPGCNT, GPGCNT,
  WSPEAK, VIRTPEAK, PAGFILCNT, FREP0VA, FREP1VA, and WSSIZE becomes the
  real working set size. SHOW SYSTEM's and SHOW PROCESS's page fault
  fields use the counts. SHOW MEMORY gains the lists and the page file.
  `$GETSYI` gains the memory items a probe shows programs use.
- **With paging off,** counting stays as it is.

**Decision 13 — system space stays non-paged, and so do page tables.**
S0 (the kernel, stacks, PCBs, page tables) never faults, as VMS's
nonpaged pool and resident executive don't. Process page tables aren't
paged either. That second level of faults is something VMS has, but
nothing a program sees depends on it. The P1 vector's frames are
system frames: valid in every process, in no working set, never on a
list.

**Decision 14 — sections go through the pager.**

- **Private file section pages:** enter the working set, leave it to the
  lists, and come back in a soft or hard fault from their PSTX. The
  writer writes modified writable non-CRF pages to the section file;
  `$UPDSEC` and `$DELTVA` write modified pages wherever they are (valid
  or on the modified list).
- **Global pages:** get global page table entries with VMS's states
  (valid in n working sets with a share count, in transition on a list,
  or out). The section's `Frames` stop being permanent.
- **Page-file global sections:** become global demand-zero pages backed
  by the page file instead of frames allocated at creation.
- `releasePTE` stays the single exit point and learns the transition
  form.

**Decision 15 — image activation maps the image instead of copying it**
(your question: yes, with paging on). Each process-private ISD becomes a
process section table entry for the image file, flagged as an image
section. Its PTEs hold the PSTX with the section's protection already
set. DZRO ISDs get demand-zero PTEs, and the header blocks are mapped the
same way. Pages come in when the program (or the fixup pass) touches
them, so image activation's faults are counted as on VMS, only touched
pages use memory, and a read-only page leaves the working set without a
write.

- **Writing back:** image sections are never written back to the image
  file. A page modified after its fault (a CRF section, a fixup cell)
  takes the page file as backing store. That's VMS's rule for CRF and
  read-only sections. Writable non-CRF private image sections, which the
  linker doesn't produce, are treated as CRF.
- **The file stays open** while the image runs. For a volume image it's
  an `rms.ACPFile` held as a section file is (`holdFile`); for a host
  image, an `os.File`. It's let go at image rundown.
- **Protection:** `setImageProtection` sets protection in every PTE form,
  not only valid PTEs.
- **Image rundown deletes P0**, as VMS's image reset does: the pages,
  their frames or page file blocks, and their sections. The working set
  goes back to WSDEFAULT.
- **Unchanged:** `ReadImage` still reads the header and debug symbols in
  Go; only the pages come from the file. `$CREPRC`'s activation
  (`ActivateCreatedImage`) and shareable images take the same path.
- **With paging off,** activation copies as it does today.

**Decision 16 — page tables and the address space under paging** (your
two notes). Paging doesn't *need* new page table management: Phases 62–66
work on today's full-size, resident tables. But with paging on, VMS's
address space model becomes worth having, and it's its own phase (67):

- **When the tables are made.** VMINIT still builds process 1's tables,
  and `BuildAddressSpace` each new process's, at process creation. Each
  table's *virtual* size is fixed then: VMINIT's `/P0` and `/P1` for
  process 1, and the same sizes (or a `vax.sysgen.virtualpagecnt` key)
  for the others. Only the S0 pages of a table that cover *created*
  pages get frames, and growing the region backs more table pages. With
  the defaults that returns most of each process's 192 table frames to
  the pager.
- **Created space.** P0LR and P1LR describe the pages that exist, as on
  VMS. `$CRETVA`, `$EXPREG`, and image activation create pages, and
  `$DELTVA`, `$CNTREG`, and image reset delete them. A touch beyond
  them is a length violation (ACCVIO). P1 below the stack grows by
  itself (EXE$EXPANDSTK, 16.2.1.3). PGFLQUOTA is charged at creation
  (Decision 10).
- **S0 under paging.** VMINIT backs only what it lays out itself. The S0
  pool's pages get frames when `AllocateS0` hands them out, so unused
  pool goes to the pager. Today's eager identity map of every S0 page
  holds about half of physical memory.
- **Process 1's console.** ASM and DEPOSIT into P0 beyond its end
  expand P0, as a convenience the console keeps.

This is the riskiest phase (PCB copies of the length registers, any code
assuming an S0 page's physical address is its virtual address minus
0x80000000, tests that use P0 freely), so it goes last and can be
deferred without blocking the rest (open question 6).

**Decision 17 — testing.**

- **Unit tests:** `internal/mmg` with `Check` after every operation,
  and `vm` and corevms tests in both modes.
- **The full suite, paging off:** passes unchanged at every commit.
- **The full suite, paging on:** a second run with a small physical
  memory and small working sets, so pages really leave. The oracle
  tests' VMS output must still match: paging must be invisible to
  programs except in the counts.
- **VMS probes:** each phase has one on the simh VAX (as `testdata/lib5x`
  did): fault counts for controlled access patterns after `$PURGWS` with
  a known WSLIMIT; the working set services' statuses and `$GETJPI`
  items; SS$_EXQUOTA from `$EXPREG`; global section share counts;
  activation faults for a small image (LIB$SHOW_TIMER).

# Part C — the phases

| Phase | Subject | Builds on |
|---|---|---|
| **62** | The plan (this doc), settings, `internal/mmg`'s PFN database and lists, the PTE forms, and the frame manager wired in, with nothing evicted yet | — |
| **63** | Working sets, soft faults, the free and modified lists, the working set services, the reclaimer, and accounting | 62 |
| **64** | The page file, the modified page writer, hard faults from the page file, and PGFLQUOTA | 63 |
| **65** | Sections under the pager: private file sections, the global page table's states, page-file global sections, `$UPDSEC`, and read clustering | 64 |
| **66** | Image activation by mapping, and image reset deleting P0 | 65 |
| **67** | Created address space, page tables backed on demand, the S0 pool backed on demand, and PGFLQUOTA at creation | 66 (deferrable) |

Each phase gets its own doc when it starts (`PHASE-6n - ....md`), with
its survey, subtasks, and progress log; the subtasks below are the
starting list. Each subtask is one commit with its tests, and the work
stops for review after each.

## Phase 62 — groundwork (this phase)

1. **The survey and plan** (this doc): done when the author has
   reviewed it.
2. **Settings:** the `Sysgen` struct and the `vax.sysgen.*` keys
   (`validConfigs`, HELP CONFIG KEYS); `quotaRules`' defaults and process
   1's nominal values read from it; VMINIT `/[NO]PAGING`; `vax.init`
   passing the setting.
3. **`internal/mmg`'s PFN database and lists:** frame states, owners,
   backing store, counts, the free list's head and tail rules, the
   modified list, and `Check`. Unit tests only.
4. **`vm`:** the transition and page file PTE forms (`SectionIndex`
   fixed); a `FrameAllocator` seam on `Memory`, which uses `pageMap` with
   paging off and `mmg` with it on; demand-zero frames zeroed when
   allocated in paging mode; the set of frames translated since the last
   instruction boundary.
5. **corevms wiring:** with paging on, every frame claim and release
   goes through `mmg`, with its owner recorded. The callers are
   `DemandZero`, `readPage`, `createSection`, `releaseFrame`, and
   `releasePTE`'s free. Nothing is evicted yet, and the suite passes in
   both modes.
6. **Showing it:** SHOW MEMORY's frame counts by state in paging mode,
   and SHOW PAGE (`LookupPTE`'s report) naming each PTE form.
7. **Close-out:** CLAUDE.md, HELP, and PLAN.md's index.

## Phase 63 — working sets and soft faults

1. **Survey:** the working set services from the manuals (`$ADJWSL`,
   `$PURGWS`, `$LKWSET`/`$ULWSET`, `$LCKPAG`/`$ULKPAG`, statuses such as
   SS$_LKWSETFUL), and the DCL commands (SHOW/SET WORKING_SET).
2. **Working set lists:** a page enters at its fault, and FIFO
   replacement follows Decision 6. Removal writes the transition PTE and
   puts the frame on a list; a fault on a transition page is soft.
3. **Growth and limits:** WSQUOTA, GROWLIM, BORROWLIM, WSEXTENT,
   MINWSCNT; `$ADJWSL` changes the real limit; image exit resets it to
   WSDEFAULT.
4. **Every exit path handles transition pages:** `releasePTE`,
   `TeardownAddressSpace`, `$DELTVA`, `$CRETVA` over pages, and
   `unmapSections`. A deleted page's frame goes to the head of the free
   list.
5. **The working set services:** `$PURGWS`; real `$LKWSET`/`$ULWSET`
   (locked entries) and `$LCKPAG`/`$ULKPAG` (frames never reclaimed).
6. **Private section pages through the working set:** clean pages go to
   the free list and their PTE returns to the PSTX when the frame is
   reused. Dirty write-back pages go to the modified list and the writer
   writes them to the section file (the page file arrives in Phase 64).
7. **The reclaimer:** FREELIM and FREEGOAL; working sets above WSQUOTA
   are trimmed.
8. **Accounting:** soft and hard counts; `$GETJPI` PPGCNT, GPGCNT,
   WSSIZE, WSPEAK; SHOW SYSTEM and SHOW PROCESS; DCL SHOW WORKING_SET.
9. **The VMS probe** (`testdata/pg63`; SYSGEN SHOW of the paging
   parameters too). govax is then changed to its answers.
10. **Close-out.**

## Phase 64 — the page file

1. **Survey:** the page file in the SYSAdmin manuals, SHOW MEMORY/FILES,
   and an audit of ods2's reentrancy (Decision 9).
2. **The page file:** open or create it (volume or host), the block map,
   and its block I/O.
3. **The modified page writer:** thresholds and clustering. A page that
   belongs in the page file gets its block, and its PTE takes the page
   file form when its frame is reused.
4. **Hard faults from the page file:** a block is freed when its page is
   deleted or modified again (its old block no longer holds its
   contents).
5. **PGFLQUOTA:** charge and refund (Decision 10); SS$_EXQUOTA from
   `$CRETVA`, `$EXPREG`, and `$CRMPSC`; JPI$_PAGFILCNT; PAGEFRAG and
   PAGECRIT; the full page file's behavior.
6. **Rundown and deletion** give back blocks and charges; DISMOUNT
   refuses the page file's volume.
7. **SHOW MEMORY's page file report.**
8. **The probe** (`testdata/pg64`).
9. **Close-out.**

## Phase 65 — sections under the pager

1. **Survey:** global page transitions (15.3) against today's
   `GlobalSection`.
2. **Private file sections, completely:** CRF pages take the page file
   as backing store, and `$UPDSEC` and `$DELTVA` reach pages on the
   modified list.
3. **The global page table's states:** share counts, soft faults on a
   global page valid elsewhere or in transition, global pages on the
   lists, and GPGCNT.
4. **Page-file global sections become demand-zero global pages** backed
   by the page file; a permanent section's pages leave memory when it's
   unmapped.
5. **Global copy-on-reference.**
6. **Read clustering:** PFCDEFAULT, `$CRMPSC`'s pfc argument, and the
   ISD's cluster. This is done only if the probes' fault counts show VMS
   clustering (it will, for image and section files), since clustering
   changes JPI$_PAGEFLTS.
7. **The probe** (extending `testdata/mp`'s section probes), and the
   close-out.

## Phase 66 — image activation by mapping

1. **Survey:** chapter 21.1's activator; the ISD flags (CRF, WRT, DZRO,
   GBL, FIXUPVEC, PROTECT) as govax's images use them.
2. **The image page source:** a volume image as a held `ACPFile`, a
   host image as an `os.File`, kept by the ICB and let go at rundown.
3. **Mapping the ISDs** (Decision 15): process section table entries,
   PTEs with protection, DZRO pages, the header. The fixup pass faults
   the pages it writes; `setImageProtection` covers every PTE form.
4. **Shareable images, the same way.**
5. **Image reset:** delete P0 (and the part of P1 VMS deletes), let the
   files go, reset the working set.
6. **`$CREPRC`'s activation through the same path.**
7. **The full suite with paging on:** the oracle tests' images run
   mapped, and the probe compares activation faults with VMS's.
8. **Close-out.**

## Phase 67 — the address space and page tables under paging (deferrable)

1. **Survey:** region lengths in the PCB and every place that reads
   P0LR or P1LR; any code assuming S0's physical address is its virtual
   address minus 0x80000000; tests that use P0 freely.
2. **Created space:** the length registers track it; `$CRETVA`,
   `$EXPREG`, `$DELTVA`, and `$CNTREG` grow and shrink it; automatic user
   stack expansion; FREP0VA and FREP1VA.
3. **Page tables backed on demand** within a virtual size fixed at
   process creation (VMINIT's `/P0` and `/P1`, or
   `vax.sysgen.virtualpagecnt`).
4. **The S0 pool backed on demand**, and VMINIT backing only its own
   layout in paging mode.
5. **PGFLQUOTA at creation** (Decision 10's switch).
6. **The console's P0 conveniences** for process 1 (ASM, DEPOSIT).
7. **The probe and close-out.**

## Future work (not planned)

- **Swapping:** outswapping whole processes; this plan's reclaimer only
  trims working sets.
- **Installed shareable images as global sections** shared between
  processes.
- **Per-process working set settings after creation:** SET
  WORKING_SET /QUOTA /EXTENT, and quotas from an authorization file.
- **Automatic working set adjustment** (PFRATL/PFRATH, WSINC/WSDEC).
- **Paged page tables, more than one page file, and a SYSGEN command.**

## Open questions (for the review)

1. **The free list in VAX memory.** Decision 3 reads your note as: the
   free (and modified) pages are real frames of VAX memory that keep
   their contents, and the list's links are Go. Did you mean the list
   structure itself should be in VAX memory (for example a PFN database
   in S0 that VAX code could inspect)? **Answer** The list structure
   itself does not need to reside in VAX memory. `govax` will not support
   user programs that attempt to directly access internal VAX/VMS data
   structures.
2. **Two lists or one?** The plan follows VMS: clean pages on a free
   list, dirty ones on a modified list that the writer drains to the page
   file when it grows past MPW_HILIMIT (Decision 7). Your description had
   a single free list spilling to the page file. Is VMS's model what you
   want? **Answer** Yes, follow the VMS model.
3. **Settings.** Is the rename to `pql_dpgflquota` OK? Do you want the
   extra pager parameters in the table as keys, or kept as constants
   until a probe gives VMS's values? **Answer** Yes, renaming the
   parameters to match VMS `sysgen` names is good. You can extend the
   config parameters as needed; that's better than constants. The goal
   is that somewhat familiar with `SYSGEN` on VMS will recognize the
   meaning/use of the config keys with corresponding `vax.sysgen.` names.
4. **Wait states.** Decision 5 resolves faults synchronously, with no
   PFW, COLPG, or FPG states. Is that acceptable, or should a hard fault
   make the process wait (letting others run) as VMS does? **Answer** it
   is fine to do all paging operations synchronously; the host system is
   so much faster than a real VAX (20x in most cases) that there is no
   meaningful benefit to asynchronous operations, and lots of added
   complexity that doesn't benefit the user-written programs' perception
   of how things are operating.
5. **Host images.** With mapping, a host image's pages are read from
   the host file while it runs. If something rewrites that file in place
   meanwhile (LINK to the same host path), the running image would see
   it. Should host images keep reading from the file (VMS-like), or page
   from the Go copy `ReadImage` already made? **Answer** While I do not
   anticipate there is every a case where external writes to an image
   or section file would happen, the VMS-like behavior is correct.
6. **Phase 67:** in the program, or deferred until the rest is done?
   **Answer** You can defer to the end.
7. **The package name:** `internal/mmg`, or something plainer
   (`internal/paging`)? **Answer** I prefer "internal/paging" for ease
   of reading by future maintainers of `govax`.

## Progress log

- **2026-10-15:** Survey written (Part A), decisions proposed (Part B),
  and the phases laid out (Part C). For review before any
  implementation. The author's notes during the survey: the page tables'
  management may change when paging is on (when they're created, and
  whether by settings or VMINIT qualifiers), and Decision 16 and Phase 67
  answer that.
