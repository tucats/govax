# Phase 64 — the page file and the modified page writer

**Status:** in progress. Subtask 1, the survey (Part A below), done
2026-10-16, and its questions answered by the author the same day
("Questions for the author"). Subtask 2, the page file, done
2026-10-16. Subtask 3, the modified page writer, done 2026-10-16. Subtask 4,
blocks freed, done 2026-10-16. Planned 2026-10-15. Needs Phase 63 (working sets, the
lists, soft faults). Part of the paging program laid out in
`PHASE-62 - VMS paging.md` (Decisions 7, 8, 9, and 10 are this phase's).

## Goal

Give the modified list somewhere to go, so that paging really frees
memory:

- **the page file**, `PAGEFILE.SYS`: `vax.sysgen.paging.file` names a
  volume file (`DUA0:[WORK]PAGEFILE.SYS`) or a host file
  (`"./pagefile.sys"`), `vax.sysgen.paging.size` blocks, created if it's
  missing, opened at first need (Decision 9);
- **the modified page writer**: when the modified list grows past
  MPW_HILIMIT it writes pages down to MPW_LOLIMIT, in clusters of up to
  MPW_WRTCLUSTER, to the page file (demand-zero and CRF pages) or to the
  section file (writable non-CRF section pages, Phase 63's writer). A
  written page moves to the free list, still faultable. Its PTE takes
  the page file form (TYP0 and TYP1 set, the block number) when its
  frame is reused;
- **hard faults from the page file**: a page whose PTE holds a page file
  block is read back into a new frame and enters the working set;
- **PGFLQUOTA** charged and given back by Decision 10's rules (as the
  author's answer to the survey's question 1 changed them), with
  SS$_EXQUOTA from `$CRETVA`, `$EXPREG`, and `$CRMPSC`;
- the page file's blocks given back when pages are deleted, at image
  rundown, and at process deletion; and its use reported.

## What earlier phases leave in place

- **Phase 63:** the modified list, the transition PTE, the reclaimer
  (which can now free frames by draining the modified list first), and
  the section-file writer.
- **Phase 62:** the page file PTE form, `Sysgen`, PGFLQUOTA's per-job
  copy (`Job.Pooled.PGFLQUOTA`). (Phase 63 added MPW_HILIMIT and
  MPW_LOLIMIT; MPW_WRTCLUSTER and the page file keys are this phase's,
  the survey's finding 1.)
- **Phase 47:** ods2's `Volume.Access` and RMS's per-file openers, which
  the page file uses to hold its file open. `ods2`'s
  `File.ReadBlock`/`WriteBlock` do its block I/O.
- **Phase 51:** DISMOUNT's checks, which learn to refuse the page file's
  volume.

## Design notes

- **Layout:** block n is one page, and the allocation map is a bitmap
  in `internal/paging`, empty at each start. Nothing about the file's
  contents outlives the session, so a host file is simply reused at its
  size.
- **Creation:** a host file is created and extended to the size. A
  volume file is created contiguous, if ods2 can (survey), through RMS's
  create path, then held open with exclusive access for the session.
- **Block I/O bypasses RMS's record layer** (Decision 9), so a fault in
  the middle of an RMS operation never re-enters RMS. Subtask 1 audits
  ods2 for any state held across a call that could be disturbed by a
  page-file write from a fault in the middle of that call.
- **Allocation:** in runs for the writer's clusters (the book's 15.5.2.4
  rule, shrinking the run size when space is fragmented, is optional).
  At 65% and 90% use, SYSTEM-W-PAGEFRAG and PAGECRIT on the console,
  once each.
- **A block is freed** when its page is deleted, or when the page is
  faulted back and modified again: its block no longer holds its
  contents (15.2.1.1, step 6).
- **The quota:** charged at a page's first fault while every page
  exists from the start (until Phase 67); checked by the services that
  create pages. JPI$_PAGFILCNT reports what's left of the job's quota
  (the probe confirms the item's meaning).
- **Read clustering** from the page file is Phase 65's subtask with the
  sections', so clustering is built once, for every backing store.
- **Exhaustion** (the author, 2026-10-15): when the page file is full and
  no frame can be freed, govax writes a console message and the fault
  fails, as it does today when memory runs out. No process waits for
  space (VMS's RWMPE and RWMPB states aren't modeled).

## Open questions

The survey (Part A) answered the first and third, and the author the
second (question 3 under "Questions for the author").

- **Contiguous creation on a volume:** *answered.* No new ods2 release:
  its `Extend` takes one free run of the whole size when the volume has
  one, which is SYSGEN CREATE's default, contiguous-best-try. govax
  needs an `internal/rms` function to create the file (Part A, "The ods2
  audit").
- **SHOW MEMORY:** *answered* (the author, 2026-10-16). govax's own
  layout gains a page file section, and `/FILES` is added in VMS's VAX
  layout once the probe has captured it (the Dictionary's examples are
  Alpha's).
- **`$GETSYI` items** for the page file: *answered.* SYI$_PAGEFILE_FREE
  and SYI$_PAGEFILE_PAGE are in the VMS 5.0 manual, so 7.x has them
  (finding 14); the probe shows their values.

# Part A — the survey (subtask 1, 2026-10-16)

## Sources

- **HP OpenVMS System Management Utilities Reference Manual**, A–L and
  M–Z, 7.3-2 (`VMS SYSAdmin I.pdf`, `VMS SYSAdmin II.pdf`, extracted
  with `pdftotext -layout` to scratch files): SYSGEN's CREATE, INSTALL,
  and DEINSTALL (6–13 to 6–17), and the parameters in appendix C
  (MPW_*, PAGFILCNT, PQL_DPGFLQUOTA, PQL_MPGFLQUOTA, GBLPAGFIL,
  PFCDEFAULT). Neither volume covers sizing a page file; that's the
  System Manager's Manual, which isn't in the folder. AUTOGEN's sizing
  doesn't matter to govax, whose size is a setting.
- **DCL Dictionary**, 7.3-1, N–Z (the PDF, with its layout): SHOW MEMORY
  (DCLII–355 to 372), its /FILES and /FULL qualifiers, and examples 1
  and 11. Both examples are from Alpha systems (8KB pages, page file
  indexes counting down from 254), so the VAX layout is the probe's.
- **OpenVMS 7.3 System Messages** (`OVMS_73_SYS_MES.PDF`, extracted):
  PAGEFRAG and PAGECRIT.
- ***VAX/VMS Internals and Data Structures***: 14.5 (the page and swap
  file database, figure 14-22), 15.2.1.1 step 6 (a page file block
  freed when its page is modified again), 15.2.2 (demand-zero pages'
  backing store), 15.2.4 (pages faulted from the page file), 15.5.2 (the
  modified page writer: clustering, backing store addresses, page file
  allocation, the PAGEFRAG and PAGECRIT messages, figure 15-9), 16.2.1.1
  (the page file quota), and 16.2.2.1 (what deletion gives back).
- **VMS 5.0 System Services Reference Manual** (extracted): `$CRETVA`,
  `$EXPREG`, `$CRMPSC`, `$MGBLSC`, `$SETPRT` (their PGFLQUOTA rules and
  SS$_EXQUOTA), `$GETJPI`'s PAGFILCNT, PAGFILLOC, and PGFLQUOTA, and
  `$GETSYI`'s PAGEFILE_FREE and PAGEFILE_PAGE. VSI's text agrees where
  it covers the same services.
- **Earlier VMS runs:** `testdata/pg63/vax/pg63.log`, whose SYSGEN SHOW
  lines give each parameter's current value, SYSGEN default, minimum,
  and maximum on the VMS 7.1 VAX.
- **The ods2 module** (`../ods2`, v0.1.23, which govax's `go.mod` pins):
  `volume/file.go`, `writefile.go`, `writeheader.go`, `bitmap.go`,
  `access.go`, `volume.go`, `diskimage/plain.go`; and govax's
  `internal/rms/acp.go`, which wraps them.

## The page file on VMS

**Creating and installing.** SYSGEN's `CREATE file-spec /SIZE=n
[/[NO]CONTIGUOUS]` makes the file. The default is contiguous-best-try:
contiguous if the disk can, otherwise as many blocks as it can find,
with a warning if it got fewer than asked for. The file isn't used
until SYSGEN `INSTALL file-spec /PAGEFILE` (CMKRNL; the volume must be
mounted /SYSTEM). At boot SYSINIT opens `SYS$SYSTEM:PAGEFILE.SYS`
itself. `DEINSTALL` (7.x) marks a page file in use "deinstall pending";
it goes when nothing uses it, and SHOW MEMORY/FILES shows the state.
PAGFILCNT (VAX) is the most page files a system may install.

**Its database** (book 14.5, figure 14-22): a vector of page file
control blocks (PFL), one per installed page or swap file. Each PFL
holds the file's window, its base VBN, the page fault cluster for reads
(PFL$B_PFC), the bitmap's address and size (one bit per block, set
meaning free), where the next scan starts, the current allocation size
(PFL$L_ALLOCSIZ), and two counts: pages that may still be *allocated*
(blocks) and pages that may still be *reserved*. The second is the
committed use that SHOW MEMORY calls reservable. SYSINIT opens the
file, stores its window, and allocates the bitmap with every bit set:
the file's old contents never matter. A new process is assigned the
page file with the most free blocks, for life (PCB/PHD$B_PAGFIL;
JPI$_PAGFILLOC's fourth byte). A swap file holds whole working sets;
with no swap file, swapping uses the page file.

**PAGEFRAG and PAGECRIT.** The book (15.5.2.4) gives the V4 text,
garbled by the OCR: `SYSTEM-W-PAGEFRAG, Page file 65% full, system
continuing` and `SYSTEM-W-PAGECRIT, Page file 90% full, system trying to
continue`, issued on the console by the special-case allocator (below)
when more than 65% or 90% of the file is in use. The 7.3 messages
manual has different words: `PAGEFRAG, page file filling up; please
create more space` and `PAGECRIT, page file nearly full; system trying
to continue`. Neither is in `vmsdef.Messages`, which came from the
system services' message file. govax uses 7.3's words with the SYSTEM
facility and W severity, unconfirmed: a probe would have to fill the
VAX's page file.

## The modified page writer and allocation (book 15.5.2)

**A run** takes the modified list's first frame, then scans the page
table around its PTE, first toward lower addresses, then higher, for
more transition PTEs whose frames are on the modified list with the same
backing store. Lower first, because a program is likelier to touch
higher addresses next. The scan stops at an invalid page table page, a
PTE that isn't in transition, a frame not on the modified list, or the
cluster's size; for a section, a different section index; for the page
file, a different page file. Each write's pages are marked write in
progress, and when the write completes they move to the free list, and
the writer goes on while the list is above MFYLOLIM (MPW_LOLIMIT).

**Clusters for the page file** (15.5.2.3, figure 15-9). The writer
first allocates up to MPW_WRTCLUSTER contiguous blocks. The cluster
written there is a series of miniclusters, each a run of virtually
contiguous pages; when one ends short of the size, the writer searches
the modified list for another page bound for the same page file and
builds the next minicluster around it, possibly in another process. A
page bound for a section file is passed over in a page file run. A
section's clusters are virtually contiguous pages of one section.

**Allocation** (15.5.2.4). The writer looks for a run of the current
allocation size (PFL$L_ALLOCSIZ, normally MPW_WRTCLUSTER), scanning
from the file's start. With no such run, the size drops by 16 and the
scan starts again. When freeing makes a large enough run, the size goes
back up by 8, to at most MPW_WRTCLUSTER. At 16 or less, a special-case
routine takes the first free run of 1 to 16 blocks, and that routine
issues PAGEFRAG and PAGECRIT.

**A full page file** puts a process that wants space into a wait (the
RWMPB and RWMPE states); `$UPDSEC` too (15.5.3). MPW_WAITLIMIT stops a
process that makes modified pages faster than they can be written
(MPWBUSY until MPW_LOWAITLIMIT). govax has none of these waits
(Decision 5, and the author's exhaustion rule above).

## A page's life with the page file

- **Demand-zero page** (15.2.2): at its first fault, its backing store
  is the page file, block not yet allocated (BAK: page file index, VBN
  0), and its saved modify bit is set. A copy-on-reference page's
  private copy is the same (15.2.1.3).
- **Leaving the working set** dirty: to the modified list. **If the page
  already has a block** (written once, faulted back, modified again),
  the block is freed and the BAK's VBN cleared, since the block no
  longer holds the page's contents (15.2.1.1, step 6). A clean page
  goes to the free list and keeps its block.
- **Written:** the page gets its block in BAK and moves to the free
  list, still in transition; a touch is still a soft fault.
- **Its frame reused:** the PTE takes the page file form with the
  block's VBN.
- **A fault on that PTE** (15.2.4) reads the block into a new frame
  (clustered by the PFL's PFC; Phase 65 here). The page is clean, with
  its block as backing store: it goes to the free list if it leaves
  unmodified, and is never written again unless modified.
- **Deleted** (16.2.2.1): the frame (valid or in transition), the block
  if one is allocated, the working set entry, and the page file quota
  for every page with page file backing store, block allocated or not,
  are all given back. A page file page's contents are never written at
  deletion.

## PGFLQUOTA

- **What's charged** (16.2.1.1): each demand-zero page and each
  copy-on-reference section page, at creation, against the job's quota
  (JIB$L_PGFLCNT counts down; the quota is pooled). Deletion gives the
  charge back (16.2.2.1).
- **The services:** `$CRETVA` and `$EXPREG` return SS$_EXQUOTA "the
  process has exceeded its paging file quota". Pages are created one at
  a time, so both can succeed in part, and retadr gives the pages made
  (the book's 16.2.1; the 5.0 manual's text). `$CRMPSC` returns
  SS$_EXQUOTA when the quota runs out "while creating copy-on-reference
  or page-file-backing-store pages", `$MGBLSC` needs the quota for a CRF
  section, and `$SETPRT` charges it when a read-only private section page
  becomes read/write (SS$_EXQUOTA too). VMS 7.3's definitions also have
  SS$_EXPGFLQUOTA ("exceeded pagefile quota", %X2A2C), which the 5.0
  manual's services don't name: the probe shows which 7.1 returns.
- **Items:** JPI$_PAGFILCNT is "the remaining paging file quota",
  JPI$_PGFLQUOTA the quota, JPI$_PAGFILLOC the process's page file index
  in its fourth byte. LOGOUT/FULL's "Peak page file size" and the
  termination message's ACC$L_PGFLPEAK are the process's peak.
- **The parameters:** PQL_DPGFLQUOTA is a `$CREPRC` process's default
  (HP: not below PQL_DWSEXTENT), PQL_MPGFLQUOTA the minimum (not below
  PQL_MWSEXTENT).

## Reporting on VMS

- **SHOW MEMORY** shows "Paging file usage (in blocks)" among its
  sections; **/FILES** shows only the page and swap files, and **/FULL**
  with /FILES adds each file's details, including "Page File Number",
  the index SYSGEN DEINSTALL/INDEX takes. The Alpha example's /FILES:

  ```
  Swap File Usage (8KB pages):              Index      Free     Size
  DISK$ALPHASYS:[SYS48.SYSEXE]SWAPFILE.SYS      1         904      904
    Total size of all swap files:                                 1952
  Paging File Usage (8KB pages):           Index       Free     Size
  DISK$PAGE:[SYS48.SYSEXE]PAGEFILE.SYS;1     253        16888    16888
  DISK$ALPHASYS:[SYS48.SYSEXE]PAGEFILE.SYS   254        16888    16888
    Total size of all paging files:                              33776
    Total committed paging file usage:                            1964
  ```

  Page file indexes count down from 254, swap files up from 1; a long
  name goes on a line of its own (example 1). A VAX's columns differ
  (pages, not 8KB pages, and a reservable count, the PFL's second
  count), so the probe captures VMS 7.1's.
- **`$GETSYI`:** SYI$_PAGEFILE_FREE (free pages in the installed page
  files) and SYI$_PAGEFILE_PAGE (their size), both in the 5.0 manual, so
  VMS 7.x has them (the open question's answer); SYI$_SWAPFILE_FREE and
  SWAPFILE_PAGE beside them; and the parameters SYI$_MPW_WRTCLUSTER,
  MPW_WAITLIMIT, MPW_THRESH, PAGFILCNT (the parameter, not usage),
  PQL_MPGFLQUOTA. All are in `vmsdef.Symbols`.
- **MONITOR PAGE and IO** count pages and write I/Os to the page file
  (Page Write Rate, Page Write I/O Rate): the system-wide counts
  Decision 12 asks for; govax shows them as SHOW MEMORY lines.

## The parameters

From the pg63 VAX's SYSGEN SHOW (current, SYSGEN default, minimum,
maximum) and appendix C:

| Parameter | VAX current | Default | Range | govax today |
|---|---|---|---|---|
| MPW_HILIMIT | 5634 | 500 | 0– | key, 500 |
| MPW_LOLIMIT | 120 | **32** | 0– | key, **120** |
| MPW_WRTCLUSTER | 120 | 120 | 16–120, multiples of 8 | none (Decision 11's table said 96) |
| PFCDEFAULT | 16 | 32 | 0–127 | `$GETSYI` constant 32 (Phase 65 reads it) |
| MPW_WAITLIMIT, MPW_LOWAITLIMIT, MPW_THRESH | not asked | | | none (no waits, no swapper) |
| PAGFILCNT, SWPFILCNT | not asked | | | none (one page file) |
| PQL_DPGFLQUOTA, PQL_MPGFLQUOTA | not asked | | | key and `quotaRules`, 16400 each |

## What govax has today

- **No page file and no page file keys.** `Sysgen` (`corevms/sysgen.go`)
  has MPW_HILIMIT and MPW_LOLIMIT but not MPW_WRTCLUSTER, PFCDEFAULT,
  `paging.file`, or `paging.size`. (The doc's "What earlier phases leave
  in place" said Phase 62 left them; it didn't. Decision 11's rule
  stands: each key comes with the subtask that first reads it.)
- **Backing store** (`frames.go`): a demand-zero page's frame is claimed
  with **no backing** (`resolveFault`'s `faultClaim(f, paging.Backing{})`)
  and made "effectively modified"; a copy-on-reference copy, and a
  page-file global section's page, have `PageFile` backing with block 0.
  VMS gives the demand-zero page page file backing too (15.2.2). Either
  way the writer must treat a modified page with no backing as bound
  for the page file.
- **The writer** (`mpw.go`) writes only private section pages, one page
  per write, head first. Every other page stays on the modified list and
  counts in `mpwStuck`; the reclaimer's `reclaimStuck` is the same.
- **A page file PTE isn't faulted.** `resolveFault` handles transition,
  global, and section PTEs; a page file PTE (TYP0 and TYP1) falls through
  to **demand-zero**, so a page whose frame was reused after a page file
  write would come back as zeros. No code makes the form yet
  (`BreakTie`'s `backingPTE` can, but only from a free-list frame, and
  page-file-bound pages never reach the free list), so it's latent, and
  subtask 4's first job.
- **Deletion paths** (`releasePTE`, `TeardownAddressSpace`, `DeleteP0`
  through `releasePTE`) know valid and transition frames, global and
  section PTEs; none knows a page file PTE or a frame's page file block.
  `heldFrame` returns the frame and modify bit, not its backing.
- **`$EXPREG`** (`core.go`) creates a P0 range all at once (or
  SS$_VASFULL), with no partial success; P1 goes through
  `expandP1Pages`. **`$CRETVA`** (`vaspace.go`) creates page by page and
  reports partial success through retadr already. The heap's page pool
  grows through `expandP0` (`memory.go`), not `$EXPREG`. None checks a
  quota.
- **PGFLQUOTA** is a recorded value (`Job.Pooled.PGFLQUOTA`, `quotaRules`
  16400, the SYSTEM account's 40960). Nothing counts what's charged:
  there's no JIB$L_PGFLCNT, no JPI$_PAGFILCNT or PAGFILLOC (F$GETJPI
  lacks them too), no per-process peak (LOGOUT's "Peak page file size"
  and ACC$L_PGFLPEAK are 0, `logout.go`, `termmsg.go`).
- **`$GETSYI`** has none of the page file items. F$GETSYI's keyword list
  (`dclshell/dcllexprocess.go`, `syiKeywords`) would need them added too.
- **SHOW MEMORY** is the machine's command (`machine/show.go`,
  `console.dcl`'s `show_memory`, no qualifiers), govax's own layout:
  physical memory, the regions, and in paging mode the frames by state
  and the fault counts.
- **DISMOUNT** refuses a volume with files open (`rms.ErrFilesOpen`,
  shown as SS$_NOMOUNT by `dclcmd/mount.go`), so a page file held open
  through `rms.ACPAccessWith` keeps its volume mounted with no new code.
  `DismountAll` at session end forces it.
- **Block I/O is in place:** `rms.ACPFile.ReadPage` and `WritePage` read
  and write one block of any allocated VBN without touching the end of
  file, as the section pager uses them; `ACPAccessWith` with NoRead and
  NoWrite is an exclusive access.
- **No Go path creates an empty, allocated volume file.** `$CREATE`
  (`rms/create.go`) needs a FAB in VAX memory, `CreateRecordFile` writes
  records, and neither honors FAB$V_CTG or CBT (`xab.go` only reports a
  header's contiguity bits).

## The ods2 audit

**Reentrancy (Decision 9's question): safe, with one rule.** A page file
read or write is `rms.ACPFile.ReadPage`/`WritePage`, which call ods2's
`File.ReadBlock`/`WriteBlock` on the page file's own `*volume.File`. Those
touch only:

- that File's in-memory header: `WriteBlock` moves its high-water mark
  and `maxWrittenVBN`, `ReadBlock` reads them (a block past the mark
  reads as zeros, and the pager writes a block before it reads it);
- the container, through `ReadAt`/`WriteAt` at an absolute offset
  (`diskimage/plain.go`), so there's no file position for an
  interrupted operation to lose;
- the device's operation count, an atomic counter.

ods2 has no locks, no block cache, and no shared buffers (`readExtents`
reads into a fresh slice). So a page file write from a fault in the
middle of an RMS operation, whose own ods2 state is another File's
header, the bitmap, or a directory, can't disturb it. The rule:
**`WriteBlock` past the file's allocation extends the file** (`Extend`,
which changes the storage bitmap and headers). That would be reentry
into exactly the state an interrupted `$PUT` may be changing. The page
file must never write past its allocation: `ACPFile.WritePage` already
refuses a VBN past `Blocks()` (ErrACPEndOfFile), and the block map
hands out nothing past the file's size. govax's own RMS state (the IFI
table, streams, record locks) isn't touched by the page file at all.
A host page file is an `os.File` used with `ReadAt`/`WriteAt` only, which
has the same property.

**Contiguous allocation: no new ods2 release needed.** ods2's `Extend`
allocates through `allocateExtents`, which takes the first free run of
the whole request when the volume has one (`min(remaining,
LargestFreeRun())`), and only otherwise the longest runs it can find. So
one `Extend` of the page file's size is SYSGEN CREATE's default,
contiguous-best-try. `Bitmap.LargestFreeRun` and `FreeClusters` are
exported, so govax can tell beforehand whether the result will be
contiguous, and record FCH$V_CONTIGB (as SYSGEN's default makes it) in the
header. The missing piece is govax's, not ods2's: an `internal/rms`
function that creates a file in a directory with a given allocation,
sets its characteristics, and returns its ACP access, built on
`volume.CreateFileVersion`, `volume.Extend`, and the header write ods2's
`File.WriteAttributes` does. rms is still the only package that imports
ods2.

## Findings that change the plan

1. **Page file keys and MPW_WRTCLUSTER come in subtask 2 and 3**, not
   from Phase 62: `vax.sysgen.paging.file` and `paging.size` (subtask 2),
   `vax.sysgen.mpw_wrtcluster` (subtask 3; default 120, VMS's VAX
   default and maximum, 16 to 120 in multiples of 8, not the 96 Decision
   11's table guessed). PFCDEFAULT stays Phase 65's.
2. **MPW_LOLIMIT's SYSGEN default is 32**, not 120 (the VAX's 120 was
   AUTOGEN's). Phase 63 took SYSGEN's defaults for BORROWLIM and
   PFCDEFAULT; subtask 3 changes `DefaultMPWLoLimit` to 32 for the same
   reason (the author's answer to question 5).
3. **A dirty page with no backing store is page file bound.** The
   writer treats `NoBacking` and `PageFile` frames alike. The claim for
   a demand-zero page could instead take `PageFile` backing, block 0, as
   VMS's BAK does. Subtask 3 does that, so that `BreakTie` never sees a
   written page with no backing.
4. **The page file PTE needs a fault path before anything writes one**
   (subtask 4, but its test comes first): today it would read as
   demand-zero. Subtask 3's writer can't be committed while a reused
   frame would lose a page, so subtasks 3 and 4 land with the fault
   path in place from subtask 3 on (the read itself is a few lines;
   freeing blocks on deletion and re-modification stays subtask 4's).
5. **Freeing a block on re-modification** happens where a frame goes to
   the modified list (`evictPage`, and `Unshare`'s callers once global
   pages page, Phase 65): if its backing has a block, the block is freed
   and the backing's block set to 0 (15.2.1.1 step 6). `paging.Database`
   needs nothing new (`SetBacking`).
6. **Deletion's paths learn two things** (subtask 4/6): a page file PTE
   frees its block, and a held frame's page file block is freed with the
   frame. `heldFrame` (or a sibling) returns the frame's backing. Quota
   refunds go through the same place.
7. **Page file allocation** is a bitmap in `internal/paging`
   (`PageFileMap`: Allocate a run, Free, counts, `Check`), with the
   book's allocation-size rule (try the current size, drop by 16 on
   failure, regrow by 8 on a large free, first fit at 16 or less). It's
   cheap and decides only block placement. PAGEFRAG and PAGECRIT are
   shown when use first passes 65% and 90%, once each per session, as
   the plan says (the book issues them only from the small-run
   allocator, which with govax's sizes amounts to the same). Blocks are
   numbered from 1, so block n is VBN n and a PTE's 0 still means
   unallocated. The PTE field limits a page file to 4,194,303 blocks.
8. **The writer clusters as the book does** (subtask 3): from the list's
   first frame, scan the process's page table down then up for
   transition pages on the modified list with the same kind of backing,
   to MPW_WRTCLUSTER or the run allocated; for the page file, continue
   with the next list frame bound for it. A cluster is one write in the
   system's counts (write I/Os and pages written, Decision 12). With
   synchronous I/O the writer may end a little below MPW_LOLIMIT, by up
   to one cluster.
9. **The quota's count.** The job gets JIB$L_PGFLCNT (pages left;
   JPI$_PAGFILCNT), each process its own charged pages and peak
   (LOGOUT's "Peak page file size", ACC$L_PGFLPEAK). A subprocess's
   charges come from the job's pool and go back at its deletion.
10. **Where the charge is made** (question 1; the author agreed). Decision 10 charges at a
    page's first fault because every P0 and P1 page exists from the
    start. But the services that create pages know which ones they
    create: `$CRETVA`, `$EXPREG` (both regions), the heap's `expandP0`.
    Charging there, and at first touch only for a page no service
    created (an image's pages under today's copying activation, ASM's
    and DEPOSIT's, the user stack), with a per-process set of charged
    pages so nothing is charged twice, would make PAGFILCNT move as
    VMS's does in the probe's `$EXPREG` and `$DELTVA` cases now, not in
    Phase 67. Phase 67 then only drops the first-touch rule.
11. **`$EXPREG` of P0 succeeds in part**, page by page, when the quota
    runs out (the book's 16.2.1), as `$CRETVA` already does; today it's
    all or nothing. The probe confirms retadr's range.
12. **Image pages under copying activation** (question 2; the author
    chose to charge only writable and DZRO ISDs' pages). Until Phase
    66, every image page is a demand-zero page the loader wrote, so its
    contents exist only in memory and its backing must be the page file,
    read-only code included. On VMS only the writable (CRF) and DZRO
    sections are charged. govax's PAGFILCNT in an image would be lower
    than VMS's by the image's read-only pages if every page were charged,
    so the loader charges only the pages of writable and DZRO ISDs (the
    ones VMS charges); the read-only ones are backed by the page file
    but uncharged until Phase 66 maps them.
13. **SHOW MEMORY** (question 3; the author agreed): keep govax's layout with a page file
    section (its name, size, free and charged blocks, PAGEFRAG/PAGECRIT
    state, the writer's counts), and add `/FILES` in VMS 7.1's VAX
    layout once the probe has captured it.
14. **`$GETSYI` and F$GETSYI** gain PAGEFILE_FREE, PAGEFILE_PAGE (0 with
    no page file open), SWAPFILE_FREE and SWAPFILE_PAGE (0: govax has
    no swap file; unconfirmed, VMS may count the page file a swap file
    falls back to), MPW_WRTCLUSTER, and PQL_MPGFLQUOTA; `$GETJPI` and
    F$GETJPI gain PAGFILCNT and PAGFILLOC (index by the probe's answer).
15. **Creating the volume file** needs a new `internal/rms` function
    (the ods2 audit). Record attributes, owner, and protection are
    unconfirmed until the probe's DIRECTORY/FULL of the VAX's
    `SYS$SYSTEM:PAGEFILE.SYS`. A host file is created and truncated to
    its size, and isn't registered with RMS's host sharing
    (`System.HostOpeners`): nothing else in govax opens it.
16. **VMINIT, INIT, and ZERO** make a new System, so the old one's page
    file must be closed (its access ended) first; otherwise its volume
    couldn't be dismounted until the session ends.
17. **I/O counts.** Page file I/O counts in the disk's operation count
    (ods2 counts every block), as VMS's driver counts paging I/O; it
    doesn't count in any process's DIRIO (unconfirmed; the probe's DIRIO
    before and after the page file run shows it).

## Questions for the author

Answered by the author on 2026-10-16: each recommendation accepted. The
findings and subtasks above and below follow the answers.

1. **Charge at creation where govax knows it** (finding 10), first
   touch only for pages no service created? *Recommended: yes.* It
   changes Decision 10's rule now instead of in Phase 67, and makes the
   probe's PAGFILCNT cases comparable. **Answer:** yes.
2. **Image pages until Phase 66** (finding 12): charge every page the
   loader writes (simple, but PAGFILCNT lower than VMS's), or only the
   pages of writable and DZRO ISDs (closer to VMS; the loader marks
   them)? *Recommended: only writable and DZRO ISDs.* Either way every
   image page is backed by the page file until Phase 66. **Answer:**
   only writable and DZRO ISDs.
3. **SHOW MEMORY** (finding 13): govax's layout plus a page file
   section, with VMS's `/FILES` layout added from the probe?
   *Recommended: yes.* **Answer:** yes.
4. **A probe case that changes the VAX's configuration:** SYSGEN CREATE
   and INSTALL a small second page file on DUA1, to see SHOW MEMORY/FILES
   with two files, the reservable count, and DISMOUNT's refusal. VMS can
   only DEINSTALL it to "deinstall pending", so DUA1 couldn't be
   dismounted until the VAX reboots. *Recommended: leave it out* unless
   you're willing to reboot the VAX after the run. **Answer:** left
   out; the probe doesn't change the VAX's configuration.
5. **MPW_LOLIMIT's default** (finding 2): SYSGEN's 32, or keep 120?
   *Recommended: 32.* **Answer:** 32.

## The probe's cases (subtask 8)

`testdata/pg64`: a program, a command procedure, and `exchange.cmd`, as
pg63's. Values that vary from run to run print as differences.

- **SYSGEN SHOW** of PAGFILCNT, SWPFILCNT, MPW_WRTCLUSTER,
  MPW_WAITLIMIT, MPW_LOWAITLIMIT, MPW_THRESH, PQL_DPGFLQUOTA,
  PQL_MPGFLQUOTA, GBLPAGFIL, VIRTUALPAGECNT. SHOW MEMORY/FILES and
  /FILES/FULL (the VAX layout), before and after the program; DIRECTORY/
  FULL of `SYS$SYSTEM:PAGEFILE.SYS`; F$GETSYI of PAGEFILE_FREE,
  PAGEFILE_PAGE, SWAPFILE_FREE, SWAPFILE_PAGE.
- **The quota:** PGFLQUOTA, PAGFILCNT, and PAGFILLOC at the image's
  first instruction (what activation charged); PAGFILCNT after `$EXPREG`
  of n P0 pages, after touching them, after `$DELTVA` of half; the same
  for `$CRETVA` and for P1 `$EXPREG`; an `$EXPREG` of PAGFILCNT + 10
  pages (the status, EXQUOTA or EXPGFLQUOTA, and retadr's range: partial
  or none); the same for `$CRETVA`; with and without the EXQUOTA
  privilege (`SET PROCESS/PRIVILEGE`), in case it lets the quota be
  exceeded; LIB$GET_VM of more than the quota allows (its status). In a
  SPAWNed subprocess, PAGFILCNT before and after the parent's `$EXPREG`
  (the pool shared).
- **The writer and the page file:** at a small working set limit, dirty
  more pages than MPW_HILIMIT (the VAX's 5634; the program asks
  `$GETSYI` and sizes itself to it), read SYI$_PAGEFILE_FREE before and
  after, and PAGEFLTS and DIRIO for a second sweep. PAGEFLTS can't tell
  soft faults from hard on VMS 7.1 (no per-process item), so the case
  shows what the counts are, and the page file's use shows that the
  writer wrote; PAGFILCNT shouldn't change at writing (quota charged at
  creation, not use).
- **Deletion:** PAGEFILE_FREE after `$DELTVA` of written pages (blocks
  back at once?), and after image exit.
- **LOGOUT/FULL** of a subprocess that used the page file: "Peak page
  file size".

## Subtasks (provisional)

1. **Survey** (done 2026-10-16): Part A. The page file in the SYSGEN
   chapter of the utilities manuals (CREATE, INSTALL; sizing isn't in
   them, and govax's is a setting); SHOW MEMORY/FILES; the book's 14.5,
   15.2, 15.5.2, and 16.2; the services' quota rules; the ods2
   reentrancy audit (safe, as long as the page file never extends) and
   contiguous allocation (ods2's `Extend` already is
   contiguous-best-try).
2. **The page file** (done 2026-10-16): the `vax.sysgen.paging.file` and `paging.size`
   keys; open or create it (host, and volume through a new
   `internal/rms` function, contiguous-best-try; finding 15) at first
   need, held with exclusive access, closed when its System goes
   (finding 16); the block map in `internal/paging` with the book's
   allocation-size rule (finding 7); block I/O that never extends the
   file; PAGEFRAG and PAGECRIT. Unit tests on a host file and on a test
   volume.
3. **The modified page writer** (done 2026-10-16): `vax.sysgen.mpw_wrtcluster`, and
   MPW_LOLIMIT's default (findings 1, 2); demand-zero pages claimed with
   page file backing (finding 3); clustering (finding 8); blocks
   assigned at write time, pages to the free list after the write, the
   page file PTE form when the frame is reused, and the fault that reads
   it back (finding 4), so no page is lost between commits.
4. **Blocks freed** (done 2026-10-16) on re-modification (finding 5) and on deletion
   (finding 6: `$DELTVA`, `$CRETVA` over pages, image rundown, process
   teardown); hard faults counted as page reads.
5. **PGFLQUOTA:** the job's count and each process's charge and peak
   (finding 9), charged at creation by `$CRETVA`, `$EXPREG`, and the
   heap's expansion, by the image loader for writable and DZRO ISDs'
   pages, and at first touch only for pages nothing created, with a
   per-process set of charged pages (findings 10 and 12, the author's
   answers); SS$_EXQUOTA (or EXPGFLQUOTA, by the probe) from `$CRETVA` and
   `$EXPREG` with partial success (finding 11) and from the heap's
   expansion; refunds at deletion; JPI$_PAGFILCNT and PAGFILLOC
   (F$GETJPI too); LOGOUT's peak and ACC$L_PGFLPEAK; exhaustion's console
   message and failed fault, and the stuck counts (`mpwStuck`,
   `reclaimStuck`) for a full page file.
6. **The rest of the system:** DISMOUNT's refusal of the page file's
   volume tested (it holds already, the survey found); the reclaimer's
   writer with the page file; a subprocess's charges back to the job at
   its deletion.
7. **Reporting:** SHOW MEMORY's page file section (question 3) and the
   writer's counts; `$GETSYI` and F$GETSYI's items (finding 14).
8. **The VMS probe** (`testdata/pg64`), Part A's case list; govax is then
   changed to its answers (SHOW MEMORY/FILES's VAX layout among them).
9. **Close-out.**

## Progress log

- **2026-10-15:** Planned with Phase 62's review.
- **2026-10-15:** The author settled exhaustion: a console message and
  a failed fault, with no process waiting.
- **2026-10-16:** Subtask 1, the survey (Part A): SYSGEN's CREATE,
  INSTALL, and DEINSTALL and the paging parameters from the utilities
  manuals; SHOW MEMORY/FILES from the DCL Dictionary (Alpha examples
  only); the book's page file database, a page's life with the page
  file, the writer's clustering and allocation, the quota; the services'
  quota statuses and the `$GETJPI`/`$GETSYI` items. Answered: ods2 is
  safe to call from a fault as long as the page file never extends past
  its allocation, and its `Extend` is already contiguous-best-try, so no
  new ods2 release is needed; SYI$_PAGEFILE_FREE and PAGEFILE_PAGE exist
  (5.0 manual). Found: Phase 62 left none of the page file keys or
  MPW_WRTCLUSTER (the doc said it had); MPW_LOLIMIT's SYSGEN default is
  32; a page file PTE would fault as demand-zero today; demand-zero
  frames have no backing store; `$EXPREG` of P0 is all or nothing; no
  Go path creates an allocated volume file; DISMOUNT already refuses a
  volume with the page file open. The subtasks are adjusted, the probe's
  cases listed, and five questions are for the author ("Questions for
  the author").
- **2026-10-16:** The author reviewed the survey and accepted each
  recommendation: PGFLQUOTA is charged at creation where a service or
  the image loader creates the pages, and at first touch only for pages
  nothing created (a change to Phase 62's Decision 10, noted there and
  in Phase 67's doc); image pages are charged only for writable and DZRO
  ISDs until Phase 66; SHOW MEMORY keeps govax's layout with a page file
  section and adds VMS's `/FILES` layout after the probe; the probe
  doesn't install a second page file on the VAX; MPW_LOLIMIT's default
  becomes 32. Subtask 2 is next.
- **2026-10-16:** Subtask 2, the page file. `internal/paging/pagefile.go`
  is the allocation map (`PageFileMap`): a bitmap, blocks numbered from
  1, `Allocate(want)` by the book's rule (a run of the allocation size,
  first fit from the file's start; no such run drops the size by 16; at
  16 or less, the first free run however short; never more than asked
  for), `Free` (the size regrows by 8, to MPW_WRTCLUSTER, when the freed
  blocks' run is long enough), and `Check`; PAGEFRAG and PAGECRIT come
  back from `Allocate` as a `Warning` the first time use passes 65% and
  90%. `internal/rms/pagefile.go`'s `Session.OpenPageFile` opens or
  creates the file: a quoted name, or one `Locate` calls a host path, is
  a host file (created sparse by `Truncate`); anything else a volume
  file, created through `MountTable.ACPCreate` with its blocks (refused,
  leaving nothing behind, when the volume hasn't room), marked
  contiguous-best-try (and contiguous when it is) with fixed 512-byte
  records and its end of file at its allocation (unconfirmed until the
  probe), then accessed with Write, NoRead, NoWrite. Either is used at
  its own size if it exists. Its `ReadBlock`/`WriteBlock` refuse block
  0 and blocks past its size, so it never grows. `corevms/pagefile.go`
  is the System's: `OpenPageFile` opens it at first need, only with
  paging on and a name configured, in process 1's RMS session; a
  failure is one console message (`%GOVAX-W-NOPAGEFILE`, with the
  reason) and no page file after it. `AllocatePageFile` writes PAGEFRAG
  and PAGECRIT on the console (7.3's texts, SYSTEM facility,
  unconfirmed); `FreePageFile`; `ClosePageFile`, which the machine calls
  before it replaces the System (INIT, VMINIT, ZERO; finding 16) and the
  console at the session's end, before the volumes are dismounted.
  `Sysgen` gains `PageFileName`, `PageFileSize` (default 32768 blocks,
  `DefaultPageFileSize`), and `MPWWriteCluster` (default 120; its key is
  subtask 3's). The keys are in `machine/sysgen.go`, `validConfigs`, and
  HELP CONFIG KEYS. Tests: the map's rules (`internal/paging`), host and
  volume files with exclusive access and DISMOUNT's refusal
  (`internal/rms`), the System's lazy open, failure message, warnings,
  and close (`internal/corevms`), and the keys and VMINIT's close
  (`internal/machine`). For subtask 3: opening may create the file,
  which changes the volume's bitmap and directory, so the writer must
  call `OpenPageFile` at an instruction boundary (the reclaimer's), not
  from inside a fault; afterward the block I/O is safe anywhere.
- **2026-10-16:** Subtask 3, the modified page writer. The
  `vax.sysgen.mpw_wrtcluster` key (`machine.ParseWriteCluster`: a
  multiple of 8 from 16 to 120, else the default, 120, with cmd/govax's
  warning), and MPW_LOLIMIT's default is 32 (finding 2). A demand-zero
  page's frame is claimed with page file backing, block 0
  (`corevms/frames.go`'s `demandZeroBacking`; finding 3). `mpw.go`'s
  writer now writes in clusters (finding 8): from the list's first
  frame, `miniCluster` takes the neighbors in its process's page table,
  down then up, that are in transition with frames on the modified list
  bound for the same place, to MPW_WRTCLUSTER; a section's cluster is
  that run, written to the section's blocks; a page file cluster first
  allocates a run of blocks (no more than there are page-file-bound
  frames, to MPW_WRTCLUSTER), fills it with miniclusters around that
  frame and the next ones on the list bound for the page file, writes
  each page to its block, sets the block as the frame's backing store,
  and moves the frame to the free list (`Cleaned`); unfilled blocks go
  back. A cluster is one write: `PageCounts.WriteIOs` beside `Written`.
  When the page file is full or missing, the frames bound for it count
  as stuck, as before; the console message for exhaustion is subtask
  5's. The page file is opened at an instruction boundary: the writer,
  woken inside a fault with the file configured but unopened, asks for
  the boundary, where `openWantedPageFile` opens it (when the modified
  list is over MPW_HILIMIT, or over MPW_LOLIMIT for the reclaimer's
  writing), forgets the stuck counts, and runs the writer. A reused
  frame's page takes the page file PTE form with its block (`BreakTie`,
  unchanged), and `resolveFault` reads a page file PTE with a block
  back into a new, clean frame backed by that block
  (`pageInFromPageFile`, a hard fault, counted as a read); block 0 is
  demand-zero (finding 4). Brought forward from subtask 4, because the
  writer relies on it: a frame that goes to the modified list with a
  page file block (read back, then modified again) gives the block up
  at once (`forgetPageFileBlock`, finding 5), and the writer allocates
  a new one; the writer also frees an old block it finds. Deletion
  still doesn't free a page's block (subtask 4). Tests:
  `internal/console`'s `TestPageFileWriter` (an 80-page array at a
  30-page working set, thresholds 10 and 2, clusters of 16, a host page
  file: pages written in clusters to consecutive blocks holding their
  contents, page file PTEs after the frames are reused, every page read
  back by hard faults, re-modified pages' blocks replaced with no
  growth) and `TestPageFileWriterNoFile` (no page file: pages stay on
  the modified list, as before); `internal/machine`'s
  `TestSysgenWriteCluster` and the new MPW_LOLIMIT default.
- **2026-10-16:** Subtask 4, blocks freed. Re-modification was subtask
  3's (`forgetPageFileBlock`). Deletion (finding 6):
  `corevms/workingset.go`'s `releasePageFileBlock` gives back the block a
  page being deleted holds, in its PTE (the page file form) or as the
  backing store of the frame it still has, valid or in transition, tied
  to the page (`heldFrame`, `tiedEntry`): a page written and soft-faulted
  back, or read back from the page file, and not modified since. Its
  contents are never written. `releasePTE` calls it, so `$DELTVA`,
  `$CRETVA` over existing pages, `$CRMPSC` over pages, and image
  rundown (`DeleteP0`) all give blocks back, and `TeardownAddressSpace`
  calls it for each P0 and P1 PTE at process deletion. Hard faults on
  the page file count in `PageCounts.PageFileReads` (among
  `Faults.Read`, which counts section reads too), shown by SHOW MEMORY
  as a "Page file" line under Read. The quota's refunds come with the
  quota, subtask 5. Tests: `TestPageFileWriter` now checks the page
  file reads and that image rundown leaves no block in use;
  `internal/console`'s `TestDeleteGivesPageFileBlocksBack` (blocks in
  page file PTEs and in valid frames read back, then `$DELTVA` of the
  array; `checkPageFileHolders` checks that the blocks in use are
  exactly those process 1's pages hold, one page each) and
  `TestDeleteProcessGivesPageFileBlocksBack` (a second process's pages
  written and waiting on the free list, then the process removed).
