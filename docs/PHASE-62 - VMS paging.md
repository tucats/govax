# Phase 62 — VMS paging: the plan, the decisions, and the groundwork

**Status:** planned (2026-10-15). Part A (the survey) and Part B (the
decisions) were reviewed and accepted by the author on 2026-10-15 (the
answers are under "The review"). Part C splits the work into Phases 62
to 67, each with a planning doc of its own. Subtasks 1 to 4 are done
(the plan, the settings, `internal/paging`'s PFN database, and `vm`'s
PTE forms and frame allocator). Needs Phases 43–49 (processes, the scheduler,
sections) and 52.

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
  follows VMS (Decision 7), as the author confirmed.
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

# Part B — decisions (accepted 2026-10-15)

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
`internal/paging`** (the author's choice of name over VMS's facility
prefix, MMG, for future maintainers' sake). It's
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
section table, the global page table, and the page file map. The
author confirmed this reading: govax doesn't support programs that read
VMS's internal data structures directly, so none of them needs a VAX
layout.

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
gains nothing visible to programs. The author agreed: the host is about
20 times faster than a real VAX, so asynchronous paging would add
complexity without changing how programs see the system.

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
| | | | *Each key below is added to `validConfigs` and HELP in the subtask that first reads it; subtask 2 added `paging.enable`, the four PQL keys, and `minwscnt`.* |
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

**Process 1 and future logins.** On VMS a process that logs in gets
its quotas from its account's SYSUAF record, not the PQL parameters.
govax has one account, SYSTEM, the console's process, and no
authorization file yet. The author plans one, and plans process 1 to
become one of many detached jobs (2026-10-15). So a logging-in process's
quotas come from an `Account` record (today always `SystemAccount()`),
with any configured PQL_Dxxx parameter laid over it
(`Sysgen.LoginQuotas`). A future SYSUAF supplies other records through
the same function. With no key set, process 1 keeps the SYSTEM
account's values (WSDEFAULT 512, WSQUOTA 1024, WSEXTENT 16400, its job's
PGFLQUOTA 40960). A process `$CREPRC` makes gets the PQL defaults.

The minimums (PQL_Mxxx) stay `quotaRules`' table values. The defaults
marked unconfirmed should come from SYSGEN SHOW on the simh VAX (a
subtask asks for that run). Only the first four rows were in the
author's request; the rest are the parameters VMS's pager reads. Each
is optional and keeps its default when unset. The author's rule for
adding more (2026-10-15): a setting the pager reads should be a key, not
a constant, named `vax.sysgen.` plus the VMS SYSGEN parameter's name in
lowercase, so that someone who knows SYSGEN recognizes it. Only
settings VMS has no parameter for (`paging.enable`, `.file`, `.size`)
are govax's own. Later phases add their keys by the same rule:
`vax.sysgen.gblpages`, `gblsections`, and `gblpagfil` (Phase 65, today's
`DefaultGlobalPageLimit` and `DefaultGlobalSectionLimit` constants
among them), and `vax.sysgen.virtualpagecnt` (Phase 67).

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
  image, an `os.File`. It's let go at image rundown. Pages are read
  from the file, not from the Go copy `ReadImage` makes, for a host
  image too: the author chose VMS's behavior.
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
0x80000000, tests that use P0 freely), so it goes last: the author
deferred it to the end of the program.

**Decision 17 — testing.**

- **Unit tests:** `internal/paging` with `Check` after every operation,
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
| **62** | The plan (this doc), settings, `internal/paging`'s PFN database and lists, the PTE forms, and the frame manager wired in, with nothing evicted yet | — |
| **63** | Working sets, soft faults, the free and modified lists, the working set services, the reclaimer, and accounting | 62 |
| **64** | The page file, the modified page writer, hard faults from the page file, and PGFLQUOTA | 63 |
| **65** | Sections under the pager: private file sections, the global page table's states, page-file global sections, `$UPDSEC`, and read clustering | 64 |
| **66** | Image activation by mapping, and image reset deleting P0 | 65 |
| **67** | Created address space, page tables backed on demand, the S0 pool backed on demand, and PGFLQUOTA at creation | 66 (deferred to the end) |

Each later phase has a planning doc of its own: its goal, what earlier
phases leave in place, its open questions, and its provisional subtasks
(the survey that starts the phase may change them):

- [PHASE-63 - working sets](PHASE-63%20-%20working%20sets.md)
- [PHASE-64 - page file](PHASE-64%20-%20page%20file.md)
- [PHASE-65 - sections and paging](PHASE-65%20-%20sections%20and%20paging.md)
- [PHASE-66 - mapped image activation](PHASE-66%20-%20mapped%20image%20activation.md)
- [PHASE-67 - address space under paging](PHASE-67%20-%20address%20space%20under%20paging.md)

Each subtask is one commit with its tests, and the work stops for
review after each.

## Phase 62 — groundwork subtasks (this phase)

1. **The survey and plan** (this doc, and the planning docs of Phases
   63 to 67): done (2026-10-15).
2. **Settings** (done 2026-10-15): the `Sysgen` struct and the
   `vax.sysgen.*` keys (`validConfigs`, HELP CONFIG KEYS); `$CREPRC`'s
   defaults and process 1's login quotas read from it; VMINIT
   `/[NO]PAGING`, defaulting to the setting (so `vax.init` needs no
   change).
3. **`internal/paging`'s PFN database and lists** (done 2026-10-15):
   frame states, owners, backing store, counts, the free list's head and
   tail rules, the modified list, and `Check`. Unit tests only.
4. **`vm`** (done 2026-10-15): the transition and page file PTE forms (`SectionIndex`
   fixed); a `FrameAllocator` seam on `Memory`, which uses `pageMap` with
   paging off and `paging` with it on; demand-zero frames zeroed when
   allocated in paging mode; the set of frames translated since the last
   instruction boundary.
5. **corevms wiring:** with paging on, every frame claim and release
   goes through `paging`, with its owner recorded. The callers are
   `DemandZero`, `readPage`, `createSection`, `releaseFrame`, and
   `releasePTE`'s free. Nothing is evicted yet, and the suite passes in
   both modes.
6. **Showing it:** SHOW MEMORY's frame counts by state in paging mode,
   and SHOW PAGE (`LookupPTE`'s report) naming each PTE form.
7. **Close-out:** CLAUDE.md's package descriptions (`internal/paging`,
   the settings), HELP, and PLAN.md's entry.

## Future work (not planned)

- **Swapping:** outswapping whole processes; this plan's reclaimer only
  trims working sets.
- **Installed shareable images as global sections** shared between
  processes.
- **Per-process working set settings after creation:** SET
  WORKING_SET /QUOTA /EXTENT, and quotas from an authorization file.
- **Automatic working set adjustment** (PFRATL/PFRATH, WSINC/WSDEC).
- **Paged page tables, more than one page file, and a SYSGEN command.**

## The review (2026-10-15)

The questions the plan asked, with the author's answers; the
decisions above have been changed to follow them.


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
- **2026-10-15:** The author reviewed the plan and answered its
  questions ("The review"). The decisions were changed to follow the
  answers: the package is `internal/paging` (Decision 2); the lists'
  links stay in Go (Decision 3); VMS's two lists (Decision 7);
  synchronous paging (Decision 5); settings named for SYSGEN's
  parameters, with more keys added as needed (Decision 11); host images
  read from their file (Decision 15); Phase 67 deferred to the end
  (Decision 16). Phases 63 to 67 each got a planning doc, and PLAN.md's
  index lists them. Subtask 1 is done.
- **2026-10-15, subtask 2 (settings):** `corevms/sysgen.go`: `Sysgen`
  (`Paging`, the configured PQL defaults by quota name, `MinWSCount`) on
  the `System` (`SetSysgen`), with `DefaultSysgen`. `resolveQuotas`
  takes a quota's default from it (`pqlDefault`). `Account`,
  `SystemAccount`, and `LoginQuotas` give a logging-in process (process 1
  today, `NewEnvironment`) its working set sizes and its job's PGFLQUOTA,
  laid over by the configuration, and MINWSCNT. `$CREPRC` sets the
  child's MINWSCNT from it too. The author's note that process 1 will
  become one of many detached jobs with a SYSUAF behind them shaped
  this: no special case for process 1, only an account record.
  `machine/sysgen.go` reads `vax.sysgen.paging.enable`, `minwscnt`,
  `pql_dwsdefault`, `pql_dwsquota`, `pql_dwsextent`, and `pql_dpgflquota`
  (`ParseSysgenCount`: decimal or %X; anything not a positive integer
  keeps the default, and cmd/govax warns at startup).
  `Machine.VMInitPaging` takes the mode, `VMInit` takes it from the
  setting, and INIT and ZERO clear it. The mode reaches each new System
  through `newRTL`. VMINIT's `/[NO]PAGING` qualifier is in `console.dcl`
  (id 1818). HELP VMINIT and CONFIG KEYS describe them; HELP says paging
  is being added in stages and changes nothing yet. Tests:
  `corevms/sysgen_test.go`, `machine/sysgen_test.go`,
  `console/vmpaging_test.go`. The full suite passes.
- **2026-10-15, subtask 3 (the PFN database):** `internal/paging`, a
  leaf package (`doc.go` describes the model, from the book's 14.2 and
  15.2). `Database` (`New(frames)`: every frame free, in PFN order, so
  the lowest go first as today) keeps an `Entry` per frame: `State`
  (`Free`, `Modified`, `Active`), `Type` (`ProcessPage`, `GlobalPage`,
  `SystemPage`), the tie to a virtual page (`Tied`, `Owner`: an opaque
  handle and a virtual address), `Backing` (none, page file block or
  "to be allocated", section PSTX and block, GPTX), `RefCount`,
  `ShareCount`, and `SavedModify`. The lists are doubly linked through
  Go arrays (`list.go`, VMS's FLINK/BLINK). Operations: `Reserve` (a
  system frame, VMINIT's), `Allocate` (the free list's head; a tied
  frame's old tie is broken first through the caller's `TieBreaker`),
  `Release` (to the tail of the free or modified list by the saved
  modify bit, ORed with the PTE's), `Hold`, `Share`/`Unshare` (the first
  share is one reference), `Reclaim` (a soft fault, from anywhere on
  either list, keeping the bit), `Delete` (untied, to the free list's
  head), `SetBacking`, `SetModified`, `FirstModified` and `Cleaned` (the
  writer's: to the free list's tail, still tied). Left out, as Decision
  5 allows: the read/write in progress and release pending states, the
  collided bit, and the bad page list; the WSLX is Phase 63's. Two rules
  of govax's own: `Release` refuses an untied frame (`Delete` gives a
  reserved one back), and a frame on the modified list must be tied.
  `Check` verifies the links both ways, each frame in one place, the
  counts, each state's fields, and that no two tied frames share an
  owner. Tests (`database_test.go`): each operation and order rule,
  `Check` finding 24 kinds of damage, and 20,000 random operations with
  `Check` after each (which found the untied-frame rule). Nothing uses
  the package yet.
- **2026-10-15, subtask 4 (`vm`):** `pager.go`: the page file form
  (`PageFilePTE`, `PageFileBlock`, `MaxPageFileBlock`; TYP0 and TYP1
  set, block 0 a block yet to be allocated), the transition form
  (`TransitionPTE`, `Transition`: both clear, a frame number other than
  0), and `IsDemandZero` (both clear, frame 0). `SectionIndex` now
  requires TYP1 clear. Nothing writes the new forms yet; with paging off
  any other invalid page is still demand-zero. `frames.go`: a
  `FrameAllocator` on `Memory`, `bitmapFrames` (the old `pageMap`, its
  rules unchanged: lowest frame first, a freed frame cleared) or
  `pagingFrames` (the PFN database; frame 0 reserved when it's made).
  `SetPaging` chooses, and VMINIT calls it on the memory it makes, with
  its paging mode; `Memory.Paging` returns the database (nil with paging
  off). `AllocateFrame` takes a `FrameClaim` (the zero value an untied
  system frame; `Tied` with a type, owner, and backing store for
  `paging.Allocate`); with paging on a frame is cleared when it's handed
  out unless the claim says the caller fills it (`Fill`), and `FreePage`
  leaves its contents, the frame at the free list's head
  (`Database.Delete`). `AllocatePage`, `ReservePage` (a second
  reservation harmless, as before), `FreePage`, and `MappedPages` keep
  their meanings in both modes, so subtask 5's callers change only to
  record owners. `internal/paging` gained `AllocateSystem` (the free
  list's head for an untied system frame, as Reserve gives a chosen
  one). The translated-frames set: with paging on, `touched[pfn]` holds
  the instruction generation in which the frame was last translated (a
  page table walk, a TB hit, or `TranslateIn`), and
  `BeginInstruction`, which `cpu.Engine.Step` calls first thing (inlined;
  one comparison with paging off), moves the generation on.
  `TranslatedThisInstruction(pfn)` also counts the STC's frame and the
  fetch window's, so the STC's hit path stays as it was. Tests:
  `vm/frames_test.go` (both modes, clearing rules, the database's
  `Check`, the set through walks, TB hits, the STC, the fetch window,
  `TranslateIn`, and the generation's wrap), `vm/pager_test.go`'s
  forms, `paging`'s `TestAllocateSystem`, and
  `console/pagingframes_test.go` (vax.init with paging on, then an
  image touching demand-zero pages and `$EXPREG`/`$DELTVA`, the
  database sound after). The full suite passes.
