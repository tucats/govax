# Phase 63 — working sets, soft faults, and the page lists

**Status:** planned (2026-10-15). Needs Phase 62 (settings, the
`internal/paging` PFN database and lists, the PTE forms, frame
allocation through `internal/paging`). Part of the paging program laid
out in `PHASE-62 - VMS paging.md` (Parts A and B: the survey and the
accepted decisions, which this doc doesn't repeat).

## Goal

With paging on, each process gets a **working set**, and pages start
leaving memory as they do on VMS:

- every valid P0/P1 page the pager brings in enters the process's
  working set list;
- when the list is full (WSLIMIT, with VMS's growth rules toward
  WSEXTENT), the oldest page leaves (Decision 6's modified FIFO);
- a page that leaves keeps its frame: its PTE takes the *transition*
  form, and the frame goes to the free list (clean) or the modified list
  (dirty, or "effectively modified": every demand-zero and
  copy-on-reference page);
- a fault on a transition page is a **soft fault**: the frame comes off
  its list, and no I/O happens;
- the working set services stop being bookkeeping: `$ADJWSL` changes the
  real limit, `$PURGWS` empties a range, `$LKWSET` locks pages in, and
  `$LCKPAG` locks them in memory;
- DCL's **SET WORKING_SET** (/LIMIT, /QUOTA, /EXTENT, /[NO]ADJUST) changes
  the current process's own values within what it was authorized at
  creation, the first per-process setting after creation (the author,
  2026-10-15), and SHOW WORKING_SET reports them;
- faults are counted per process as soft or hard, and the working set's
  sizes are reported.

There's no page file yet (Phase 64), so a dirty page that belongs in
the page file stays on the modified list. The one writer in this phase
writes modified pages of writable `$CRMPSC` file sections back to their
own files. Eviction in this phase therefore mostly moves pages between
the working set and the lists. Physical memory is freed only by clean
pages being reused and by deletion, as before.

## What earlier phases leave in place

- **Phase 62:** the `Sysgen` settings (PQL defaults, MINWSCNT, GROWLIM,
  BORROWLIM, FREELIM, FREEGOAL, TBSKIPWSL), copied into each process at
  creation; the PFN database with owners and backing store; the free
  list's head and tail rules; the transition PTE form; the set of frames
  translated since the last instruction boundary.
- **Phase 49:** `releasePTE`, the single exit point for a page leaving
  a PTE; private file sections (`processSection`); `writePage`.
- **Phase 26:** `$ADJWSL` (`process.go`, the `WSLimit` field), the
  page lock maps (`pageprot.go`), `$DELTVA`/`$CRETVA`.
- **Phases 43–44:** each process's `Environment` and address space, and
  the scheduler. A fault on a page of a process that isn't current
  (`TranslateIn`) still belongs to that process
  (`processForSpace`).

## Design notes

- **The working set list** lives in `internal/paging`, one per process:
  a circular list of entries (virtual address, global or private,
  locked), WSNEXT, the limit and the quotas, and PPGCNT and GPGCNT.
  Process handles are PIDs, so the package knows nothing of corevms.
- **Global section pages stay resident in this phase.** A page-file
  section's frames are valid in mapping PTEs from the start, and a file
  section's frames stay once read. They count in GPGCNT but are never
  chosen for replacement until Phase 65 gives the global page table its
  states. Private file section pages do go through the working set.
- **Replacement skips:** entries still valid in the TB (current process
  only: the TB holds no other process's entries), up to TBSKIPWSL;
  locked entries; and frames translated during the current instruction.
- **Image exit** shrinks the working set to WSDEFAULT, removing entries
  oldest first.
- **The reclaimer** (Decision 7) is built here but can only trim to the
  lists. It frees frames only when trimmed pages are clean, until Phase
  64's writer drains the modified list to the page file.
- **Soft faults count.** JPI$_PAGEFLTS counts every fault in paging
  mode, soft and hard. With paging off, the count is today's.

## Open questions

- **JPI$_WSSIZE:** govax returns WSLimit; is VMS's value the limit or
  the pages in use? SHOW WORKING_SET and the probe settle it.
- **Page table faults:** VMS's page tables are pageable and their faults
  count too. govax's page tables don't fault (Decision 13). The probe
  shows how much of VMS's counts that accounts for; differences are
  expected and listed, not chased.
- **SET WORKING_SET's limits:** the DCL Dictionary bounds /QUOTA and
  /EXTENT by the authorized values (WSAUTH, WSAUTHEXT) and by PQL_MWSQUOTA
  and PQL_MWSEXTENT. govax's authorized values are those the process got
  at creation. /ADJUST has nothing to switch until automatic adjustment
  exists, so it's recorded and shown (the survey checks how VMS reports
  it).
- **Statuses:** `$LKWSET`'s SS$_LKWSETFUL threshold, and `$PURGWS` on a
  range with locked pages. The manuals first, then the probe.

## Subtasks (provisional)

1. **Survey:** `$ADJWSL`, `$PURGWS`, `$LKWSET`/`$ULWSET`,
   `$LCKPAG`/`$ULKPAG` from the 5.0 manual and VSI's text (arguments,
   statuses, privilege); SHOW WORKING_SET and SET WORKING_SET in the DCL
   Dictionary; the book's 14.1.2 and 15.4 details the design needs.
2. **Working set lists** in `internal/paging`, with `Check`: add,
   replace (Decision 6), remove, lock. Unit tests.
3. **corevms:** a resolved fault adds the page to the working set,
   replacement removes one (transition PTE, frame to a list), and a soft
   fault takes a transition page back.
4. **Limits:** WSQUOTA, GROWLIM, BORROWLIM, WSEXTENT, and MINWSCNT;
   `$ADJWSL` on the real list; the reset to WSDEFAULT at image exit.
5. **Every exit path handles transition pages:** `releasePTE`,
   `TeardownAddressSpace`, `$DELTVA`, `$CRETVA` over existing pages,
   and `unmapSections`. Deleted pages' frames go to the head of the free
   list.
6. **The working set services:** `$PURGWS`; real `$LKWSET`/`$ULWSET`
   and `$LCKPAG`/`$ULKPAG`.
7. **Private file section pages through the working set:** a clean
   page's PTE returns to its PSTX when its frame is reused, and a dirty
   write-back page is written to its file from the modified list. The
   writer's thresholds are MPW_HILIMIT and MPW_LOLIMIT, as Phase 64's will
   be.
8. **The reclaimer** (FREELIM, FREEGOAL), and accounting: soft and hard
   counts; `$GETJPI` PPGCNT, GPGCNT, WSSIZE, WSPEAK; SHOW SYSTEM's and
   SHOW PROCESS's page faults.
9. **DCL SET WORKING_SET and SHOW WORKING_SET** (the console's grammar
   and `internal/dclshell`), in process 1 and in subprocesses, through
   the same code as `$ADJWSL`.
10. **The VMS probe** (`testdata/pg63`): a program that sets a limit
   with `$ADJWSL`, purges with `$PURGWS`, touches pages in known
   patterns, and reports PAGEFLTS deltas, PPGCNT, GPGCNT, and WSSIZE;
   also the services' statuses, and SYSGEN SHOW of the paging parameters
   (Decision 11's table). govax is then changed to its answers.
11. **Close-out:** CLAUDE.md, HELP, PLAN.md.

## Progress log

- **2026-10-15:** Planned with Phase 62's review.
- **2026-10-15:** The author put SET WORKING_SET in this phase
  (subtask 9).
