# Phase 64 — the page file and the modified page writer

**Status:** planned (2026-10-15). Needs Phase 63 (working sets, the
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
- **PGFLQUOTA** charged and given back by Decision 10's rules, with
  SS$_EXQUOTA from `$CRETVA`, `$EXPREG`, and `$CRMPSC`;
- the page file's blocks given back when pages are deleted, at image
  rundown, and at process deletion; and its use reported.

## What earlier phases leave in place

- **Phase 63:** the modified list, the transition PTE, the reclaimer
  (which can now free frames by draining the modified list first), and
  the section-file writer.
- **Phase 62:** the page file PTE form, `Sysgen` (MPW_HILIMIT,
  MPW_LOLIMIT, MPW_WRTCLUSTER, PFCDEFAULT, the page file keys),
  PGFLQUOTA's per-job copy (`Job.Pooled.PGFLQUOTA`).
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

- **Contiguous creation on a volume:** does ods2 offer contiguous
  allocation, or does govax need a new ods2 release (tag, then
  `GOWORK=off go get`, per CLAUDE.md)?
- **SHOW MEMORY:** keep govax's own layout with a page file section
  added, or add VMS's `SHOW MEMORY/FILES` layout (DCL Dictionary)
  beside it?
- **`$GETSYI` items** for the page file (SYI$_PAGEFILE_FREE,
  SYI$_PAGEFILE_PAGE): add them if the probe shows VMS 7.x has them.

## Subtasks (provisional)

1. **Survey:** the page file in the SYSAdmin manuals (SYSGEN CREATE,
   sizing); SHOW MEMORY/FILES; the ods2 reentrancy audit and contiguous
   allocation.
2. **The page file:** open or create it (host and volume), the block
   map in `internal/paging`, block I/O, and the PAGEFRAG and PAGECRIT
   messages. Unit tests on a host file and on a test volume.
3. **The modified page writer:** thresholds, clustering, page file
   backing assigned at write time, pages to the free list after the
   write, and the page file PTE form when the frame is reused.
4. **Hard faults from the page file**, and blocks freed on delete and on
   re-modification.
5. **PGFLQUOTA:** charge and refund, SS$_EXQUOTA from the creating
   services, JPI$_PAGFILCNT; exhaustion's console message and failed
   fault.
6. **Rundown and deletion** give blocks and charges back; DISMOUNT
   refuses the page file's volume; the reclaimer runs the writer first.
7. **Reporting:** SHOW MEMORY's page file usage; the system-wide writer
   counts.
8. **The VMS probe** (`testdata/pg64`): SS$_EXQUOTA boundaries for
   `$EXPREG` and `$CRETVA` against PGFLQUOTA; PAGFILCNT before and after;
   fault counts when pages come back from the page file (forced by a
   small limit and enough touched pages).
9. **Close-out.**

## Progress log

- **2026-10-15:** Planned with Phase 62's review.
- **2026-10-15:** The author settled exhaustion: a console message and
  a failed fault, with no process waiting.
