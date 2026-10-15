# Phase 65 — sections under the pager

**Status:** planned (2026-10-15). Needs Phase 64 (the page file and the
writer). Part of the paging program laid out in
`PHASE-62 - VMS paging.md` (Decision 14 is this phase's).

## Goal

Bring every kind of section into the working set and list machinery, so
that section pages leave memory and come back the way VMS's do:

- **Private file sections** (`$CRMPSC` with a channel): their pages
  already go through the working set (Phase 63). This phase finishes
  them. A copy-on-reference page takes the page file as backing store at
  its first fault (charged to PGFLQUOTA). `$UPDSEC` and `$DELTVA` write
  back modified pages wherever they are, valid or on the modified list
  (16.2.2.1).
- **Global sections** get VMS's global page states (15.3). Today a
  global page's frame belongs to its section for good (`Frames`). Here a
  global page is one of:
  - **valid** in one or more working sets, with a *share count*;
  - **in transition** on the free or modified list, once the last
    working set lets it go;
  - **out**, in its backing store: the section file, the page file, or
    demand-zero.

  A process faulting a global page valid elsewhere, or in transition,
  takes the frame with a soft fault. A process PTE that leaves its
  working set goes back to the GPTX form.
- **Page-file global sections** (`SEC$M_PAGFIL`) stop allocating every
  frame at creation. Their pages start as global demand-zero pages and
  are backed by the page file once modified, as VMS's are. A permanent
  section's pages can leave memory while nothing maps them.
- **Global copy-on-reference**: the first fault copies the page into a
  private page backed by the page file.
- **Read clustering** (15.5.1) for every backing store: a hard fault
  also reads the neighboring pages that have the same backing store, up
  to the section's PFC (the `$CRMPSC` pfc argument, an ISD's cluster in
  Phase 66) or PFCDEFAULT, while the working set has room.

## What earlier phases leave in place

- **Phases 46 and 49:** `GlobalSection` (`Frames`, `Dirty`, `Refs`),
  the global page table in Go (`GlobalSections.pageTable`, GPTX PTEs),
  `pageInGlobal`, `releaseFrame`, `maybeDeleteSection`,
  `writeBackSection`, `$UPDSEC(W)`, `sectionFiles` holding files open.
- **Phase 63:** working sets with GPGCNT, global pages kept resident
  until now, the section-file writer.
- **Phase 64:** page file backing, the writer, PGFLQUOTA.

## Design notes

- **The global page table entry** (in Go) gains a state and a share
  count, beside the section and page number it has now. `Refs` stays the
  count of process PTEs that map the section (its lifetime), while the
  share count is how many of them are valid on the page right now
  (where the frame lives). The two are separate on VMS too.
- **Modified bits** are gathered from each process PTE as it leaves its
  working set into the frame's saved modify bit, as `Dirty` gathers them
  now. A transition global page with the bit set is on the modified list.
  The writer writes it to the section file or the page file.
- **Global page-file sections count against GBLPAGFIL**, VMS's
  system-wide limit on page-file global pages, not against a process's
  PGFLQUOTA. GBLPAGES and GBLSECTIONS, today constants
  (`DefaultGlobalPageLimit`, `DefaultGlobalSectionLimit`), become keys by
  the author's rule for settings: `vax.sysgen.gblpages`,
  `vax.sysgen.gblsections`, `vax.sysgen.gblpagfil`.
- **Clustering changes JPI$_PAGEFLTS:** neighbors read with a fault
  don't fault when touched. The probes' counts decide the cluster rules,
  and clustering can be switched off by a PFCDEFAULT of 1.

## Open questions

- **How visible is clustering?** VMS's fault counts for sequential
  access to a section with a known PFC should show the cluster size
  directly. If a probe shows a rule beyond the book's (for example the
  backward scan's effect), follow the probe.
- **Permanent sections with no mappers** keep their contents in the page
  file on VMS. Confirm with a probe that a permanent page-file section
  survives with its contents while unmapped (govax keeps them today by
  keeping the frames).

## Subtasks (provisional)

1. **Survey:** chapter 15.3 and 16.3 against today's `GlobalSection` and
   `processSection`; the `$CRMPSC` pfc argument; GBLPAGFIL in the
   SYSAdmin manuals.
2. **Private file sections finished:** CRF pages backed by the page
   file and charged; `$UPDSEC` and `$DELTVA` over pages on the modified
   list.
3. **The global page table's states and share counts:** soft faults on
   global pages valid elsewhere or in transition; global pages leaving
   working sets; the GPTX form back in the process PTE; global pages on
   the lists and written by the writer.
4. **Page-file global sections as global demand-zero pages;** the
   GBLPAGES, GBLSECTIONS, and GBLPAGFIL keys.
5. **Global copy-on-reference.**
6. **Read clustering** for section files, image files (used in Phase
   66), and the page file.
7. **The VMS probe**, extending `testdata/mp`'s section probes: GPGCNT
   and share behavior across two processes, fault counts for sequential
   access with and without a pfc, `$UPDSEC` after pages leave the
   working set.
8. **Close-out.**

## Progress log

- **2026-10-15:** Planned with Phase 62's review.
