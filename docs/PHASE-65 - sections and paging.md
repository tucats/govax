# Phase 65 — sections under the pager

**Status:** done (2026-10-18). Subtasks 1 to 8: the survey (Part A,
its questions answered by the author); private copy-on-reference pages
(page file backing, charged at mapping); the global page table's states
and share counts (`corevms/globalpages.go`); page-file global sections
as global demand-zero pages and the GBLSECTIONS, GBLPAGES, and GBLPAGFIL
limits; global copy-on-reference checked under the new states; read
clustering (`corevms/cluster.go`, `vax.sysgen.pfcdefault`); the VMS
probe (`testdata/pg65`, run on VMS 7.1: `$UPDSEC` by UPDFLG and deleted
sections pending were changed to its answers); the close-out. Needs Phase 64 (the page file and the
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
- **Phase 64 (done 2026-10-18):** page file backing (`corevms/pagefile.go`
  over `paging.PageFileMap` and `rms.PageFileStore`), the writer's
  clusters to the page file and section files (`corevms/mpw.go`; woken
  at MPW_HILIMIT, it writes down to MPW_HILIMIT less MPW_WRTCLUSTER,
  `mpwGoal`, as VMS 7.1 did), the page file PTE's hard fault (one page
  at a time: read clustering from the page file is this phase's, with
  the sections'), and PGFLQUOTA (`corevms/pgflquota.go`). The writer
  writes no global pages yet.

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

# Part A — the survey (subtask 1, 2026-10-18)

## Sources

- ***VAX/VMS Internals and Data Structures*** (the OCR text, beside the
  PDF): 14.2.6 and 14.2.7 (REFCNT and SHRCNT), 14.3 (the global section
  descriptor, the global section table entry, global page table entries
  and their forms, process PTEs for global pages; figures 14-14 to
  14-19), 15.2.1.3 and 15.2.3 (copy-on-reference pages, private and
  global), 15.3 (page faults for global pages: read-only, read/write,
  copy-on-reference, page-file backing store; figures 15-6 to 15-8),
  15.5.1 (page reads and clustering, Table 15-1), 15.5.3 (`$UPDSEC`),
  16.2.2.1 (what deletion gives back), and 16.3 (`$CRMPSC`, `$MGBLSC`,
  `$DGBLSC`, `$UPDSEC`).
- **VMS 5.0 System Services Reference Manual** (extracted): `$CRMPSC`'s
  arguments table (which arguments each kind of section uses), its pfc
  argument, its description (PGFLQUOTA for copy-on-reference pages,
  GBLPAGFIL for page-file pages), its statuses (SS$_EXGBLPAGFIL,
  SS$_EXQUOTA, SS$_GPTFULL, SS$_GSDFULL); `$GETSYI`'s FREE_GBLPAGES,
  FREE_GBLSECTS, and CONTIG_GBLPAGES.
- **HP OpenVMS System Management Utilities Reference Manual**, M–Z,
  7.3-2 (`VMS SYSAdmin II.pdf`, extracted): GBLPAGES, GBLPAGFIL,
  GBLSECTIONS, and PFCDEFAULT in appendix C; SYSGEN SHOW's rule for
  abbreviations on a VAX (6–27: `SHOW GBL` shows only GBLSECTIONS, and
  GBLPAGFIL needs `SHOW GBLPAGF`).
- **Earlier VMS runs:** `testdata/pg64/vax/pg64.log`'s SYSGEN SHOW of
  GBLPAGFIL and PFCDEFAULT, and `testdata/pg63`'s `$GETSYI` of
  PFCDEFAULT, on the VMS 7.1 VAX.
- **govax:** `corevms/gblsec.go`, `filesec.go`, `frames.go`,
  `workingset.go`, `mpw.go`, `pgflquota.go`, `accounting.go`,
  `getsyi.go`; `internal/paging`'s `database.go`; `vmsimage`'s ISD.

## Global pages on VMS (14.3, 15.3)

**The structures.** A global section has three: a global section
descriptor (GSD: name, protection, ident, flags; used only to create,
map, and delete), a global section table entry (GSTE, in the system
header, the twin of a process section table entry: the file's window,
base VBN, the section's page fault cluster, flags, and the count of
process PTEs that map the section), and one **global page table entry**
per page. The global page table sits, as far as memory management's
code is concerned, at the virtual end of the system page table: a
global page table index (GPTX) is a pseudo system page number. The
hardware never sees it.

**A GPTE's forms** (14.3.3): **valid** (the page is in at least one
working set, the PFN in the GPTE), **transition** (its frame is on the
free or modified list, or being read or written; the PFN STATE array
says which), **demand zero** (a page-file section's page never
touched), and **out**: a global section table index (GSTX) for a page
in a section file, or, for a page-file section, a page file block. The
modify bit in a GPTE means nothing: the hardware can't reach it.

**A process PTE for a global page** holds the GPTX until the process
faults the page, and only then the PFN, valid; when the page leaves the
process's working set it gets the GPTX back, never a transition form
(15.3.1, steps 6 and 7; 15.3.2). The GPTX is recomputed from the PFN
entry's PTE pointer, which for a global page points at the GPTE.

**The PFN entry of a global frame:** its PTE pointer is the GPTE's
address, not any process PTE's; BAK is the GSTX (or the page file
backing store address for a page-file page); **SHRCNT** counts the
process PTEs valid on the frame. Its first share adds one to REFCNT and
its last takes it away (14.2.7). A frame with a share count is on no
list.

**The read-only example** (15.3.1, figure 15-6): processes A and B map
the section (each PTE a GPTX; nothing else changes). B faults first:
the pager follows the GPTX to the GPTE, finds a GSTX, allocates a frame
(REFCNT 2, SHRCNT 1 while the read is in progress, BAK the GSTX) and
reads it; the GPTE is in transition and B's PTE still a GPTX until the
read completes, when both become valid. A then faults: the GPTE is
valid, so A's PTE is a copy of it and SHRCNT becomes 2 (a soft fault,
MONITOR's "global valid"). B's working set lets go: SHRCNT 1, B's PTE
gets its GPTX back. A's lets go: SHRCNT 0, REFCNT 0, the frame goes to
the free list (or the modified list by its saved modify bit) with the
GPTE in transition. When the frame is reused, the tie is broken through
the GPTE: BAK goes back into it, and no process PTE changes.

**Read/write global pages** (15.3.2) go through the same states as a
private page that isn't copy on reference, but in the GPTE. A process's
modify bit is gathered into the frame's saved modify bit as its PTE
leaves (the GPTE has none).

**Global copy-on-reference pages** (15.3.3, figure 15-7): the GPTE keeps
its GSTX for good (with the CRF bit). The fault reads the page into a
frame that is the process's own: its PTE becomes a transition PTE and
then valid, the PFN entry points at the process PTE, BAK is the page
file with no block, and the saved modify bit is set. All ties with the
section are broken; from then on it's a private copy-on-reference page
(figure 15-4).

**Global page-file pages** (15.3.4, figure 15-8): each GPTE starts as
demand zero. The first fault anywhere allocates a frame (the PFN entry
points at the GPTE, BAK the page file with no block), stores its PFN in
the GPTE, makes it valid, and makes the process PTE valid too; a later
fault by another process copies the valid GPTE. From then on the
transitions are a page file page's (figure 15-5), in the GPTE: written
by the writer to a page file block, faulted back from it.

**Private copy-on-reference pages** (15.2.1.3, figure 15-4): the
section's PTEs hold the PSTX with the CRF bit. The first fault reads the
page from the file, **sets the saved modify bit** (so it goes to the
modified list whenever it leaves the working set, modified or not), and
makes the page file its backing store, block not yet allocated: "all
ties to the original image file are broken". From then on it's a page
file page.

## Lifetimes, writing back, and deletion

- **`$DELTVA`** (16.2.2.1) gives back the frame (valid or in
  transition), an allocated page file block, the working set entry, and
  the PGFLQUOTA of every page with page file backing store; a modified
  page of a section file is written back first. A private section's
  reference count (in its PSTE) drops, and the PSTE goes at zero. For a
  global page, the count of PTEs in the GSTE drops.
- **`$DGBLSC`** (16.3.3) marks the section delete pending and clears
  its permanent bit; the section goes when the GSTE's count reaches zero,
  and its writable pages are written to their backing store first (the
  section file; a page-file section's contents just go).
- **`$UPDSEC`** (15.5.3, 16.3.4): for a private section only modified
  pages are written (the PTE's modify bit, or the PFN's for a transition
  page). For a **global** section VMS can't know which pages are
  modified (no back pointers from a frame to the process PTEs), so
  **every page in the range is written**, modified or not, unless
  the flags' low bit says the caller is the only writer; then the
  caller's PTEs choose. Writes cluster up to MPW_WRTCLUSTER, in the
  direction the range is given.
- **Permanent sections:** a permanent page-file section's pages,
  unmapped, are global pages with no share count: on the lists, then in
  the page file (the open question's case).

## Read clustering (15.5.1)

**The scan.** On a hard fault the pager looks at the PTEs at higher
addresses first, for pages in the same backing store, until the cluster
is full or it meets: a PTE that isn't the same kind (below); an invalid
page table page; no more working set list entries; no free frame. **Only
if the forward scan added no page** does it scan toward lower addresses,
by the same rules. Every page read is added to the working set.

**What matches** (15.5.1.2): for a PSTX, exactly the same PSTX (every
page of a private section holds the same one, so the cluster runs over
the section's pages still out); for a page file PTE, successively
increasing (or decreasing) blocks; for a GPTX, successive GPTXs whose
GPTEs hold the same GSTX. Only hard faults cluster: not demand zero,
not soft faults.

**The cluster's size** (15.5.1.3, Table 15-1):

| Page | Cluster |
|---|---|
| private section (image file or `$CRMPSC`) | the PSTE's PFC, or PFCDEFAULT (through PHD$B_DFPFC) when 0 |
| global section | the GSTE's PFC, or PFCDEFAULT |
| global copy-on-reference | the same |
| page file | the page file's PFL$B_PFC, normally 0, so PFCDEFAULT |
| process page table pages | PAGTBLPFC (govax: no paged page tables, Decision 13) |
| global page table pages | 1 |

A section's PFC comes from LINK's CLUSTER option (an ISD's PFC byte,
Phase 66) or from `$CRMPSC`'s pfc argument. The 5.0 manual: pfc is "how
many pages are to be brought into memory when a page fault occurs for a
single page", **not used for page-file sections** (nor PFN-mapped or
shared-memory ones), and `$MGBLSC` has no pfc argument: a global
section's PFC is its creator's. PFCDEFAULT (appendix C): "the number of
image pages read from disk per I/O operation", from an image file or the
page file, "less than PFCDEFAULT, depending on the size of image
sections and the pattern of page references", at most 127.

**Unconfirmed by the book:** whether the faulting page counts in the
cluster's size (the probe's sequential fault counts show it); what "no
more working set list entries" means when the list is full but could
replace (govax's reading: the cluster stops rather than evicting); how a
page-file global section's pages match once their GPTEs hold page file
blocks rather than a GSTX; a pfc above 127, and what PFCDEFAULT 0 does
(the range allows it).

## The parameters

| Parameter | VAX current | SYSGEN default | Range | Meaning | govax today |
|---|---|---|---|---|---|
| GBLSECTIONS | not asked | 40 (7.3-2's example) | 20– | global section descriptors | constant 128 |
| GBLPAGES | not asked | not given | | global page table entries: each section a page per page plus 2, rounded up to even | constant 4096, a page per page |
| GBLPAGFIL | 4096 | 1024 | 128–4194176 | page-file global section pages, systemwide | none |
| PFCDEFAULT | 16 | 32 | 0–127 | the default read cluster | `$GETSYI` constant 32 |
| PAGTBLPFC | not asked | 2 (the book) | | page table pages' cluster | none (no paged page tables) |

`$GETSYI` has items for each (SYI$_GBLSECTIONS, GBLPAGES, GBLPAGFIL,
PFCDEFAULT, PAGTBLPFC), and for what's left: SYI$_FREE_GBLPAGES (free
global page table entries), FREE_GBLSECTS, and CONTIG_GBLPAGES (the
largest run of free entries). All are in `vmsdef.Symbols`; govax answers
only PFCDEFAULT. FREE_GBLPAGES before and after a `$CRMPSC` shows the
"plus 2, rounded up to even" rule on the VAX.

## What govax has today

- **Page-file global sections** (`createSection`) allocate every frame
  at creation, zeroed, claimed as global pages backed by the page file
  (`globalClaim`), and a mapping process's PTEs are **valid at once**,
  pointing at them (`mapSection`). They have no global page table
  entries, and `byFrame` maps their frames back to the section. So
  JPI$_GPGCNT counts a page from its mapping, not its first touch
  (Phase 63's finding 9), and nothing faults.
- **File global sections** (`createFileSection`) have a global page
  table in Go (`GlobalSections.pageTable`, an entry per page: the
  section and page number), and mapping PTEs hold the GPTX. The first
  fault anywhere reads the page into a frame the section keeps
  (`Frames[i]`) until it's deleted; later faults are global valid
  (`pageInGlobal`). The entry has no state: a page is in (`Frames[i]`)
  or out.
- **A global frame never leaves memory.** It's claimed `GlobalPage`
  with one reference and stays active; `paging.Database`'s `Share` and
  `Unshare` exist but nothing calls them. Global pages enter working
  sets (counted in GPGCNT) but `wsProtected` keeps them from being
  chosen, `evictPage` refuses any frame that isn't a process page's, and
  `BreakTie` ignores global frames. A process PTE leaving (`releasePTE`)
  drops the section's `Refs`, and with paging off or on its modify bit
  goes into `Dirty[i]`, which `writeBackSection` (at deletion) and
  `$UPDSEC` clear.
- **Private copy-on-reference sections** (`createPrivateSection` with
  SEC$M_CRF) are read with the **section file as backing store**
  (`sectionBacking`) and no saved modify bit. With paging on, a clean
  page leaves to the free list and is read from the file again (VMS's
  goes to the modified list and the page file); a modified one goes to
  the modified list, which nothing writes (`writableSectionPage` wants
  a section that writes back), so it waits there, counted stuck, until
  a soft fault or its deletion. It's **never charged** to PGFLQUOTA.
- **Global copy-on-reference** pages are read into a private frame
  backed by the page file and effectively modified, as VMS's, but
  **charged at first touch** (`chargeFirstTouch`), not at mapping.
- **`$UPDSEC` and `$DELTVA` over private pages on the modified list
  are done** (Phase 63's subtasks 5 and 7: `modifiedTransitionPage`,
  `heldFrame`, `releasePTE`); the plan's subtask 2 listed them.
  `$UPDSEC` of a global section writes every resident page in the range
  unless the flags' low bit is set, as the book says, and a page another
  process brought in that this one hasn't touched (its PTE still a
  GPTX) too.
- **The writer** writes private section pages and page-file-bound
  process pages; it knows no global frame (`pageFileBound` and
  `writableSectionPage` want process pages).
- **No limits beyond two constants:** GBLSECTIONS (128) and GBLPAGES
  (4096, a page per page, file and page-file sections alike); no
  GBLPAGFIL, no SS$_EXGBLPAGFIL; no `vax.sysgen.gblpages`,
  `gblsections`, `gblpagfil`, or `pfcdefault` key (Phase 62's table
  planned `pfcdefault`; nothing added it).
- **No clustering:** every hard fault reads one page. `$CRMPSC`'s pfc
  (argument 12) isn't read, and the image ISD's PFC (`vmsimage.ISD.PFC`)
  is decoded but unused.
- **Counts:** `FaultCounts.Read` counts hard faults; nothing counts
  pages read apart from faults (MONITOR PAGE's page reads against page
  read I/Os), which clustering makes different.

## Findings that change the plan

1. **Subtask 2 shrinks** to private copy-on-reference pages: the first
   fault claims the frame with the page file as backing store (no
   block) and sets the saved modify bit, as `pageInGlobal`'s CRF path
   already does; from then on the writer and the page file PTE's fault
   handle it, and a modified page no longer waits forever. `$UPDSEC` and
   `$DELTVA` over the modified list were Phase 63's.
2. **Copy-on-reference pages are charged at mapping** (16.2.1.1, the
   5.0 manual's SS$_EXQUOTA "while creating copy-on-reference ...
   pages"): `$CRMPSC` of a private CRF section, and `$CRMPSC`/`$MGBLSC`
   of a global CRF one, charge every page mapped, refusing past the quota
   (whether in part or whole is the probe's question; `$CRETVA` maps in
   part). The global CRF fault's first-touch charge then finds the page
   already charged. Unconfirmed: whether a read-only CRF mapping is
   charged too (the book's rule says every CRF page).
3. **The global page table entry gets a state** (subtask 3): valid
   (frame, share count), transition (frame on a list), out (the section
   file's block, a page file block, or demand zero). `pageTable` covers
   every section, page-file ones included (subtask 4); `Frames` and
   `byFrame` become what the entries say rather than permanent.
4. **Share counts are the database's.** A process PTE made valid on a
   global frame is a `Share`; leaving the working set, or being
   released, an `Unshare` with the PTE's modify bit (gathered into the
   saved modify bit, replacing `Dirty` with paging on); the frame goes to
   a list when the last share goes. The frame's allocation reference
   must become the first share's (the database's `Release` refuses a
   shared frame's last reference). `evictPage` learns global pages
   (the process PTE gets its GPTX back, not a transition form),
   `wsProtected` stops protecting them, `softFault` finds a GPTE in
   transition through the GPTX, and `BreakTie` breaks a global frame's
   tie through the global page table (no process PTE changes).
5. **The writer learns global frames** (subtask 3): a writable file
   section's modified page to its file, clustered over successive GPTXs
   of one section; a page-file section's to the page file, the block
   then kept in the GPTE when the frame is reused. Deleting a section
   writes back its pages on the modified list as well as its valid ones,
   and deletes their frames (`Database.Delete` of a listed frame).
6. **Page-file global sections become demand-zero GPTEs** (subtask 4):
   no frames at creation; the mapping PTEs hold the GPTX; the first fault
   anywhere is demand zero (the frame claimed `GlobalPage`, page file
   backing, effectively modified), later ones global valid. GPGCNT then
   counts a page from its first touch, closing Phase 63's finding 9.
7. **GBLPAGFIL is checked at creation** for page-file sections, the
   whole section at once (SS$_EXGBLPAGFIL, "no part of the section was
   mapped"), and **not PGFLQUOTA** (the 5.0 manual names PGFLQUOTA only
   for CRF pages; the probe confirms PAGFILCNT doesn't move). GBLPAGES
   counts entries by the manual's rule (pages plus 2, rounded up to
   even), which SYI$_FREE_GBLPAGES shows. Three keys:
   `vax.sysgen.gblsections`, `gblpages`, `gblpagfil` (question 2 for
   their defaults), and `$GETSYI`'s SYI$_GBLSECTIONS, GBLPAGES,
   GBLPAGFIL, FREE_GBLPAGES, FREE_GBLSECTS, CONTIG_GBLPAGES (F$GETSYI
   too).
8. **Read clustering** (subtask 6) is the book's: forward first, then
   backward only if forward added nothing; the matching rules above;
   stopping at a full working set list (without evicting) or an empty
   free list. The size: `$CRMPSC`'s pfc for its section (kept in the
   process section entry or the global section; 0 meaning PFCDEFAULT),
   PFCDEFAULT for the page file and page-file sections, an ISD's PFC in
   Phase 66. A new key, `vax.sysgen.pfcdefault` (default 32, SYSGEN's;
   0–127), replaces `getsyi.go`'s constant. Pages read with a fault don't
   fault later, so PAGEFLTS drops; `PageCounts` gains pages read beside
   the read faults (Decision 12's "pages read").
9. **`$UPDSEC` of a global page out of the caller's working set**
   follows the GPTE (subtask 3): valid elsewhere or on the modified list,
   it's written (the latter moved to the free list); out, nothing to
   write. Of a page-file section: unconfirmed; govax writes nothing
   (SS$_NOTMODIFIED) until the probe says otherwise.
10. **Process deletion and image rundown** release global pages through
    `releasePTE` already; with share counts, a valid global PTE's
    release is an `Unshare` (finding 4), so no new path is needed.

## Questions for the author

Answered by the author on 2026-10-18: each recommendation accepted. The
findings and subtasks follow the answers.

1. **Paging off.** Decision 1 says that with paging off govax behaves as
   before. Should the new global page states (finding 6: page-file
   sections faulting on first touch, frames allocated lazily) and read
   clustering (finding 8) apply only with paging on? *Recommended: yes,*
   both only with paging on: with it off, page-file sections keep their
   frames from creation and valid PTEs, and every read is one page, so
   no paging-off count changes. The limits (GBLPAGFIL, GBLPAGES'
   counting rule, the new keys and `$GETSYI` items) apply in both modes,
   since they aren't paging. **Answer:** yes, paging on only.
2. **The limits' defaults.** VMS's SYSGEN defaults are lower than
   govax's constants (GBLSECTIONS 40 against 128; GBLPAGFIL 1024, where
   the VAX had 4096; GBLPAGES' isn't in the manual, and the probe's
   SYSGEN SHOW gives it). Phase 64 took SYSGEN's defaults for
   MPW_LOLIMIT. *Recommended:* keep govax's present values as defaults
   (GBLSECTIONS 128, GBLPAGES 4096; GBLPAGES raised if the new counting
   rule's two extra entries a section make an existing test fail) and
   take the VAX's 4096 for GBLPAGFIL, so that no existing program meets
   a limit it didn't before; the keys lower them. PFCDEFAULT takes
   SYSGEN's 32, as `$GETSYI` reports now. **Answer:** yes.
3. **CRF charges** (finding 2): charge copy-on-reference pages at
   mapping, as VMS does? *Recommended: yes.* **Answer:** yes.

## The probe's cases (subtask 7)

`testdata/pg65`: a program (parts picked by a logical name, as pg64's),
a procedure, `exchange.cmd`. A second process for the sharing cases
(SPAWN, as pg64's part S). Counts print as differences.

- **SYSGEN SHOW** GBLSECTIONS, GBLPAGES, GBLPAGFIL (`SHOW GBLPAGF`),
  PFCDEFAULT, PAGTBLPFC; `$GETSYI` of FREE_GBLPAGES, FREE_GBLSECTS,
  CONTIG_GBLPAGES before and after a 1-, 2-, and 3-page section (the
  plus-2, even rule).
- **Page-file global sections:** PAGFILCNT and GPGCNT before and after
  creating and mapping (no PGFLQUOTA charge? GPGCNT 0 until touched?);
  touching every page (faults, GPGCNT); a second process mapping and
  touching (global valid faults, GPGCNT); a section larger than
  GBLPAGFIL (SS$_EXGBLPAGFIL, retadr); a permanent section unmapped
  with its contents written, at a small working set while another image
  dirties memory, then mapped again (contents kept; faults).
- **Global pages leaving working sets:** at a small working set limit,
  touch a file section's pages and others, `$PURGWS` the section's
  range, touch again (soft faults, from the list or global valid);
  two processes each purging while the other holds.
- **Copy-on-reference:** PAGFILCNT after mapping a private CRF section
  and a global one (charged at mapping; in part past the quota?), after
  touching; a read-only CRF mapping; a temporary global CRF section
  after its only mapper has touched every page (`$MGBLSC` of it by name:
  does the section live on, or did the faults take its references?).
- **`$UPDSEC`** of a global section with the flags' low bit clear and
  set, after the caller's pages left the working set (which pages are
  written: IOSB, retadr); of a page-file section.
- **Clustering:** sequential reads over a 200-page file section with
  pfc 0, 1, 4, 16, 64, 127, 200 (PAGEFLTS, and DIRIO, which shouldn't
  move); backward reads (the backward scan); a private section and a
  global one; pages written to the page file and read back sequentially
  (PFCDEFAULT from the page file); a working set limit smaller than the
  cluster (where the cluster stops).

## Subtasks (provisional; adjusted by the survey)

1. **Survey** (done 2026-10-18): Part A.
2. **Private copy-on-reference pages** (done 2026-10-18; finding 1): the page file as
   backing store and the saved modify bit at the first fault; their
   PGFLQUOTA charged at mapping, with the global CRF mapping's (finding
   2, question 3).
3. **The global page table's states and share counts** (done
   2026-10-18; findings 3, 4, 5, 9, 10): soft faults on global pages valid elsewhere or in
   transition; global pages leaving working sets with the GPTX back in
   the process PTE; global frames on the lists, written by the writer,
   their ties broken through the GPTE; sections deleted with pages on
   the lists; `$UPDSEC` through the GPTE.
4. **Page-file global sections as global demand-zero pages** (done
   2026-10-18; finding 6), with paging on; the GBLSECTIONS, GBLPAGES, and GBLPAGFIL keys,
   GBLPAGES' counting rule, SS$_EXGBLPAGFIL, and the `$GETSYI` items
   (finding 7).
5. **Global copy-on-reference** checked against the new states (done
   2026-10-18; no code change: its fault already makes a private page,
   and subtasks 2 to 4 left it one; the tests).
6. **Read clustering** (done 2026-10-18; finding 8): `vax.sysgen.pfcdefault`, the pfc
   argument kept per section, the scan for section files, global
   sections, and the page file (image files in Phase 66), pages read
   counted.
7. **The VMS probe** (`testdata/pg65`, done 2026-10-18), the cases
   above; govax is then changed to its answers.
8. **Close-out** (done 2026-10-18).

## Progress log

- **2026-10-15:** Planned with Phase 62's review.
- **2026-10-18:** Subtask 1, the survey (Part A): the Internals book's
  global page structures (GSD, GSTE, GPTEs and their forms, process
  PTEs for global pages, SHRCNT), the four kinds of global page fault
  (read-only, read/write, copy-on-reference, page-file), private
  copy-on-reference pages, deletion, `$DGBLSC`, `$UPDSEC` (every page of
  a global section unless the caller is the only writer), and read
  clustering (forward first, backward only if forward found nothing;
  the matching rules; the cluster sizes of Table 15-1); `$CRMPSC`'s pfc
  (not for page-file sections) and GBLPAGFIL from the 5.0 manual;
  GBLPAGES (a page per page plus 2, rounded up to even), GBLPAGFIL,
  GBLSECTIONS, and PFCDEFAULT from the utilities manual. Found: the
  plan's `$UPDSEC` and `$DELTVA` work was done in Phase 63; private
  copy-on-reference pages keep the section file as backing store, are
  never effectively modified or charged, and a modified one waits on the
  modified list for good; global CRF pages are charged at first touch,
  not mapping; `paging.Database`'s share counts are unused; page-file
  sections are charged to nothing (VMS: GBLPAGFIL); no `pfcdefault` key
  exists. The subtasks are adjusted, the probe's cases listed, and three
  questions are for the author.
- **2026-10-18:** The author accepted each of the survey's
  recommendations: the new global page states and read clustering only
  with paging on (the limits in both modes); GBLSECTIONS 128, GBLPAGES
  4096, GBLPAGFIL 4096, PFCDEFAULT 32 by default; copy-on-reference
  pages charged at mapping.
- **2026-10-18:** Subtask 2, private copy-on-reference pages
  (`corevms/filesec.go`, `gblsec.go`; tests in
  `internal/console/crfsection_test.go`):
  - **The first fault** of a private section made with SEC$M_CRF
    (`processSection.copyOnRef`, new) claims the frame with the page
    file as backing store, no block (`copyOnRefBacking`), and sets its
    saved modify bit (`effectivelyModified`), as the global CRF fault
    already did: the page goes to the modified list whenever it leaves
    the working set, the writer writes it to the page file
    (`pageFileBound` takes it), and a reused frame leaves its PTE in the
    page file form, whose fault reads it back. The section's file is
    never written. With paging off nothing changes: there's no PFN
    database to record the backing store in.
  - **Charged at mapping** (the author's answer to question 3):
    `mapChargedPage` is `mapPage` for a copy-on-reference section's
    page, private (`mapProcessSection`) or global (`mapSectionPage`, so
    `$CRMPSC` and `$MGBLSC`): with no page of quota left, SS$_EXQUOTA,
    the pages before it mapped and charged, as `$CRETVA` does
    (unconfirmed: the probe shows whether VMS's `$CRMPSC` maps in part or
    not at all). The fault's first-touch charge then finds the page
    charged.
  - **Rundown and deletion of a page in the page file form:**
    `releasePTE` gave back its block and charge but returned before the
    section lost the page, so a CRF section whose copies were in the page
    file was never dropped and kept its file accessed. It now unmaps the
    page from its section whether or not the page holds a frame.
    `Environment.ProcessSections` (new) counts a process's private
    sections, for the test.
  - **The tests:** `TestCopyOnRefSectionPaging` maps a 64-page writable
    CRF section of a file opened read-only, with a 30-page working set
    and a host page file: every page is charged at mapping; after each
    page is modified, every frame is backed by the page file, at most
    MPW_HILIMIT wait on the modified list and the rest hold their new
    values in page file blocks; with their frames reused, their PTEs
    are page file blocks, and every page reads back right; after
    rundown, nothing is charged, the page file's blocks match their
    holders, the section is gone, and the file was never written.
    `TestCopyOnRefSectionPagingOff` runs the same with paging off.
    `TestCopyOnRefSectionQuota` leaves 20 pages of quota: the private
    mapping and then a global CRF mapping each stop at 20 pages with
    SS$_EXQUOTA, and the global page's fault makes an effectively
    modified private copy without charging again. Each fails without the
    change it checks (the backing store, the charge, and the unmapping).
- **2026-10-18:** Subtask 3, the global page table's states and share
  counts, with paging on (`corevms/globalpages.go`, new; `filesec.go`,
  `gblsec.go`, `workingset.go`, `mpw.go`; `paging.Database.ShareFirst`;
  tests in `internal/console/globalpaging_test.go`):
  - **States.** A global file section's page is valid (its frame active,
    one share per process PTE valid on it), in transition (its frame on
    the free or modified list, tied to the global page), or out (its
    `Frames` slot 0, its contents in the file). The PFN database's state
    says which of the first two; the entry needs nothing more in Go.
    `Refs` stays the count of PTEs mapping the section.
  - **Faults** come through the GPTX (`pageInSharedGlobal`): valid
    elsewhere is a global valid fault and a `Share`; in transition a soft
    fault off its list (counted as a free or modified list fault), the
    frame reclaimed and given its first share; out a hard read. A frame's
    allocation (or reclaim) reference becomes its first share
    (`ShareFirst`, new), so the frame goes to a list with its last share.
  - **Leaving a working set** (`evictPage`, `evictGlobalPage`): the
    process PTE gets its GPTX back and its share goes (`Unshare`, the
    PTE's modify bit gathered into the saved modify bit); `wsProtected`
    now protects only page-file sections' pages (subtask 4), and so does
    `$PURGWS`. Releasing a valid PTE (`releaseFrame`: `$DELTVA`, rundown,
    process deletion) is an `Unshare` too; `Dirty` is paging off's alone.
  - **Reuse** breaks the tie through the global page table
    (`BreakTie` → `loseGlobalFrame`): the slot goes to 0, no process PTE
    changes.
  - **The writer** writes a writable section's global frames on the
    modified list to their blocks, clustered over the section's
    neighboring pages on the list, lower first, then higher, up to
    MPW_WRTCLUSTER (`writeGlobalCluster`; the order is govax's,
    unconfirmed, as `miniCluster`'s).
  - **Deletion** writes back the pages left on the modified list
    (`globalPageModified`, the saved modify bit) and deletes every frame
    the section's pages hold.
  - **`$UPDSEC`** of a page the caller's PTE doesn't hold valid follows
    the GPTE (`updateGlobalPage`): valid elsewhere, written and its saved
    modify bit cleared; on the modified list, written and moved to the
    free list; on the free list or out, not written (finding 9; the
    free-list case is govax's reading of "every page", unconfirmed). A
    valid page written clears its frame's saved modify bit too.
  - **Paging off** is unchanged: a page keeps its frame until the section
    goes, `Dirty` gathers the modify bits.
  - **The tests:** `TestGlobalSectionPaging` maps one writable global
    section of PAGES.DAT twice in process 1 (two PTEs per page), with a
    30-page working set: page 0 through both ranges is a hard fault, then
    a global valid fault, two shares; each page modified through one
    range leaves the working set, its PTE a GPTX, the writer writing the
    pages in clusters to the file; reading through the other range (from
    the top down) takes frames off the lists and shares those still
    valid, with no hard fault; reused frames leave their pages out, read
    back from the file; `$UPDSEC` writes the pages on the modified list
    and those valid only through the other range (its first run, pages 0
    to 8, in retadr), leaving none on the list; rundown deletes the
    section, writing back what waited on the modified list. After each
    step every PTE is valid or a GPTX, each frame's share count matches
    the valid PTEs, and the database checks. `TestGlobalSectionPagingOff`
    runs the same touches with paging off. `TestShareFirst` is the
    database's. Each fails without the change it checks (the share, the
    tie broken, the cluster, `$UPDSEC`'s GPTE, deletion's write-back).
- **2026-10-18:** Subtask 4, page-file global sections as global
  demand-zero pages, and the global section limits (`corevms/gblsec.go`,
  `globalpages.go`, `filesec.go`, `mpw.go`, `sysgen.go`, `getsyi.go`;
  `machine/sysgen.go`; tests in `internal/console/pfglobal_test.go`):
  - **No frames at creation** (with paging on): `createSection` gives a
    page-file section global page table entries (`GlobalSection.paged`,
    `hasPageTable`), and a mapping process's PTEs hold the GPTX, as a
    file section's. The first fault anywhere is a demand-zero fault
    (`pageInGlobalOut`): a frame of zeros claimed `GlobalPage`, backed by
    the page file with no block, effectively modified; later faults are
    global valid or soft, as subtask 3's. GPGCNT now counts a page from
    its first touch, closing Phase 63's finding 9. With paging off,
    nothing changes (the author's answer to question 1).
  - **The page file:** a page-file section's frame on the modified list
    is page-file bound (`pageFileGlobal`), so the writer writes it with
    the process pages, clustered with its section's neighbors on the
    list (`globalMiniCluster`; a global frame's owner is a section's
    serial number, not a PID, so `miniCluster` no longer looks it up as
    a process). A reused frame's block goes to the section's `blocks`
    slot for the page (`loseGlobalFrame`, now found through `byFrame`
    for both kinds of section), and the next fault reads it back
    (`readPageFileBlock`, shared with the page file PTE's fault), the
    block then the frame's backing store and the page clean. A page
    modified again gives its block up as its share goes
    (`dropStaleBlock`). Deletion gives back every block, out or in a
    frame's backing store. A permanent section's pages leave memory
    while nothing maps it, and come back with their contents.
  - **Not charged to PGFLQUOTA** at mapping or at the fault (finding 7).
  - **The limits, in both paging modes:** GBLSECTIONS, GBLPAGES, and
    GBLPAGFIL are `vax.sysgen.gblsections`, `gblpages`, `gblpagfil`
    (counts; defaults 128, 4096, 4096 by the author's answer to question
    2; HELP CONFIG KEYS), the System's `Sections.SectionLimit`,
    `PageLimit`, `PageFileLimit` from `SetSysgen`. A section uses its
    pages plus 2, rounded up to even, of GBLPAGES (`globalPageEntries`),
    and a page-file section its pages of GBLPAGFIL, whole at creation
    (SS$_EXGBLPAGFIL, retadr -1). The checks' order (GBLSECTIONS,
    GBLPAGES, GBLPAGFIL) is govax's, unconfirmed.
  - **`$GETSYI` (and F$GETSYI):** GBLSECTIONS, GBLPAGES, GBLPAGFIL,
    FREE_GBLSECTS, FREE_GBLPAGES, and CONTIG_GBLPAGES (FREE_GBLPAGES'
    value: govax's global page table has no fixed size to fragment;
    unconfirmed). The last three item codes are among STARLET.OLB's
    definitions only (`vmsdef.LibrarySymbols`), which `syiCodeOf` reads.
  - **The tests:** `TestPageFileGlobalSection` runs a program that
    creates a permanent 64-page section, maps it twice, deletes both
    ranges, maps it again, and deletes it, with a 30-page working set and
    a host page file: no frames and no charge at mapping; page 0 a
    demand-zero fault then a global valid fault, two shares; every page
    written, leaving to the modified list and the page file in clusters;
    frames reused, pages out in their blocks and read back by hard
    faults; pages rewritten giving up their blocks; the unmapped section
    keeping its contents; and at deletion every frame and block given
    back. After each step every PTE is valid or a GPTX, shares match the
    valid PTEs, and the page file's blocks are exactly those the pages
    hold (`checkPageFileHolders` now counts the sections' too). It fails
    without the deletion's block release (either holder), the block kept
    at reuse, or the block given up when modified. `TestCrmpsc_limits`
    checks the counting rule, SS$_EXGBLPAGFIL, and the `$GETSYI` counts;
    `TestPagingGlobalSectionFrames` and `TestWorkingSetGlobalPages` now
    expect frames from the first touch, and the pages to leave working
    sets and come back.
- **2026-10-18:** Subtask 5, global copy-on-reference under the new
  states (tests in `internal/console/globalcrf_test.go`; no code
  change). The survey's reading of 15.3.3 holds against subtasks 2 to 4:
  the global page table entry of a CRF section keeps its section file
  form for good (`pageInGlobal` takes the CRF path before
  `pageInSharedGlobal`, so the section's `Frames` stay 0 and no frame is
  ever tied to it); each PTE's first fault reads the file into a frame
  of the process's own (`faultClaim`, a `ProcessPage` tied to the PTE's
  page), backed by the page file and effectively modified, and the
  section loses that PTE's reference ("all ties ... are broken"; that
  the GSTE's count drops is govax's reading, unconfirmed, now a probe
  case). From then on every path treats it as subtask 2's private
  copy: `faultIsGlobal` keeps it out of GPGCNT, so `evictPage` gives
  it the transition form rather than a GPTX and `wsProtected` doesn't
  hold it; the writer writes it to the page file (`pageFileBound`);
  `BreakTie` gives its PTE the page file form; `releasePTE` finds no
  section for it (no `byFrame` entry) and frees the frame and block;
  and `$UPDSEC` passes over it (neither a section's frame nor a GPTX),
  so a range of copies is SS$_NOTMODIFIED.
  - **The tests:** `TestGlobalCopyOnRefPaging` maps a writable global
    CRF section of PAGES.DAT, opened read-only, twice in process 1, with
    a 30-page working set and a host page file: at mapping every PTE is
    a GPTX and charged, the section 128 references and no frames; page
    0 through both ranges is two hard faults (no global valid one) on
    two frames, range A's write not reaching range B's copy; after every
    page is modified through both ranges no PTE is a GPTX, every frame is
    process 1's own, backed by the page file and effectively modified
    (or holding its block), no two PTEs share one, GPGCNT is 0, nothing
    more is charged, and the temporary section has gone with its last
    reference; frames reused leave copies in the page file form, each
    reading back its own range's value; `$UPDSEC` over range A is
    SS$_NOTMODIFIED; rundown gives back every charge and block; PAGES.DAT
    is never written. `TestGlobalCopyOnRefPagingOff` runs the touches
    with paging off. It fails if the copy counts as a global page
    (`faultIsGlobal`), if the fault keeps the section's reference, or
    if the copy isn't effectively modified.
- **2026-10-18:** Subtask 6, read clustering, with paging on
  (`corevms/cluster.go`, new; `filesec.go`, `gblsec.go`, `sysgen.go`,
  `getsyi.go`, `accounting.go`; `paging.WorkingSet.AddAvailable`;
  `machine/sysgen.go`, `show.go`; cmd/govax; tests in
  `internal/console/clusterpaging_test.go`):
  - **PFCDEFAULT** is `vax.sysgen.pfcdefault` (0 to 127, default 32,
    `ParsePFCDefault`; cmd/govax warns about a value out of range; HELP
    CONFIG KEYS), `Sysgen.PFCDefault`, which `$GETSYI`'s SYI$_PFCDEFAULT
    now reports in place of the constant.
  - **The pfc argument** of `$CRMPSC` (its twelfth) is kept in a private
    section's entry (`processSection.pfc`) and a global file section
    (`GlobalSection.PFC`); a page-file section has none (the manual), and
    `$MGBLSC` maps with its creator's.
  - **The scan** (`readCluster`, called by `PageIn` after a hard fault):
    up from the faulting page first, then down only if up read nothing,
    each neighbor read as a fault on it would be (`resolveFault`), given
    its valid PTE, and added to the working set. A neighbor matches by
    the faulting PTE's form: the same process section table index; the
    next page file block (the one before, going down); the next global
    page table index (the one before) of the same section, the page out
    (a file section's page with no frame, every copy-on-reference page,
    a page-file section's page the writer wrote). The scan stops at a
    page that doesn't match, a PTE that can't be read, the region's end,
    an empty free list, or a working set with no available entry: it
    takes none from another page (`AddAvailable`, which fills the first
    available entry after next, past the quota only above GROWLIM, as
    the list's growth does). Neighbors aren't faults: JPI$_PAGEFLTS
    counts the one.
  - **The size** is the section's pfc or, when 0, PFCDEFAULT; the page
    file's and a page-file section's PFCDEFAULT. Unconfirmed, for the
    probe: the size counts the faulting page; 0 and 1 read it alone; a
    pfc above 127 is 127; a page-file section's written pages match
    whatever their blocks.
  - **Counts:** `PageCounts.PagesRead` (new) counts every page hard
    faults read, the faulting pages and their clusters, beside
    `Faults.Read`, the reads; `PageFileReads` counts the page file's
    pages among them. SHOW MEMORY's paging section and `--stats` show
    it. With paging off nothing clusters (the author's answer to
    question 1), and `PagesRead` equals the read faults.
  - **The tests:** `TestReadClustering` reads a 64-page section of
    PAGES.DAT in order with a working set big enough for it, private
    with pfc 0 (PFCDEFAULT 16, 32, 1, and 0), 1, 4, 5, 64, and 200, up
    and, with 16 and 5, down; global with 16, and with PFCDEFAULT 8 down;
    and global copy-on-reference with 8: one read fault per cluster
    (13 for 5: the last cluster short), no other fault, 64 pages read,
    every page valid, in the working set, and right, a global page's
    frame with one share and a copy the process's own, effectively
    modified. `TestReadClusteringWorkingSet` reads it with a 30-page
    working set and pfc 64: the first cluster fills the set to its limit
    and takes no page out, later faults read less, every page is right.
    `TestReadClusteringPageFile` writes 64 demand-zero pages with a
    30-page working set, reuses the written pages' frames, grows the
    limit with `$ADJWSL`, and reads them back in order: one read fault
    for each run of successive blocks (up to PFCDEFAULT, 16).
    `TestWorkingSetAddAvailable` is the list's, `TestSysgenPFCDefault`
    the key's. The backward cases fail without the scan down.
    `TestGlobalCopyOnRefPaging` and `TestReclaimer` now run with
    PFCDEFAULT 1, so that each page is still its own fault there.
- **2026-10-18:** Subtask 7, the probe, written (`testdata/pg65`,
  README.md; the VAX run is next). `pg65.mar` is one program in parts
  (PG65_PART), each case read with `$GETJPIW` (PAGEFLTS, PAGFILCNT,
  GPGCNT, PPGCNT, DIRIO, WSSIZE) and `$GETSYIW` (PAGEFILE_FREE,
  FREE_GBLPAGES, FREE_GBLSECTS, CONTIG_GBLPAGES, the last three by code),
  printed as changes; the image's own pages are locked in the working set
  first, so the faults are the cases'. The parts follow "The probe's
  cases": M makes PG65.DAT (200 blocks, block n holding n); G the limits
  and GBLPAGES' counting (page-file sections of 1 to 4 pages, a 3-page
  file section, one of GBLPAGFIL + 1 pages); F a permanent page-file
  section (charges at mapping, first faults, a second mapping), D the
  writer run with its pages on the modified list, R the section mapped
  again, `$DGBLSC` while mapped, and unmapped; P a global file section's
  pages purged from a small working set and back; C copy-on-reference
  (private writable and read-only, and a temporary global section
  `$MGBLSC`'d by name after its only mapper touched every page); U
  `$UPDSECW` with UPDFLG 0 and 1, after a purge and with the pages valid,
  of a page-file section and a private section, with a private mapping
  of the same blocks counting what reached the file; K clustering (pfc
  0, 1, 4, 16, 64, 127, 200 up; 16 and 0 down; global; CRF; a small
  working set); H, in a spawned subprocess, and S here, stepping through
  common event flags: another process's valid pages, purged, and after
  the holder purged its own. Not probed: a CRF mapping past PGFLQUOTA,
  and clustering from the page file (no frame reuse on that VAX).
  - **govax's dry run** (README.md lists its answers):
    `TestPG65DryRun` runs every part, the holder in a subprocess, and
    checks that every page reads back right, GBLPAGES' rule, and the
    holder's steps. G4 meets SS$_GPTFULL before EXGBLPAGFIL with govax's
    equal defaults (the checks' order is still a probe question). govax's
    DCL lacks F$GETSYI's HW_NAME, SYSGEN, and SHOW MEMORY/PHYSICAL_PAGES,
    which the procedure asks for (it goes on past them).
  - **Fixed on the way:** F$GETSYI didn't take the global section items
    or PFCDEFAULT that subtasks 4 and 6 gave `$GETSYI`
    (`dclshell/dcllexprocess.go`'s `syiKeywords`; HELP LEXICAL lists
    them); `TestGetSYIGlobalSectionItems` compares each with `$GETSYI`'s.
- **2026-10-18:** The probe's run on VMS V7.1 (`testdata/pg65/vax`;
  README.md has every answer), and govax changed to its answers.
  `TestPG65Oracle` (replacing the dry run) runs every part with the
  VAX's SYSGEN values (GBLSECTIONS 520, GBLPAGES 50000, GBLPAGFIL 4096,
  PFCDEFAULT 16) and matches VMS's lines but for
  `expectedPG65Differences`.
  - **Confirmed:** GBLPAGES' counting; SS$_EXGBLPAGFIL with retadr -1
    (the checks' order still unprobed: GBLPAGES was far from full); no
    charge for page-file sections, GPGCNT from the first touch; a
    permanent section's contents kept; NOSUCHSEC at once after
    `$DGBLSC`; copy-on-reference pages charged at mapping, read-only
    ones too (question 3's answer), and refunded at deletion; the
    temporary global CRF section gone by name once its only mapper
    touched every page (subtask 5's reading); read clustering as subtask
    6 has it (the faulting page counted, pfc 1 alone, above 127 is 127,
    backward only when forward read nothing); the second process's
    global valid faults and soft faults after purges.
  - **`$UPDSEC` of a global section** (`corevms/globalpages.go`'s
    `updateGlobalPage`, `filesec.go`): UPDFLG 0 writes every page with a
    frame, valid anywhere or on either list, modified or not (the book's
    16.3.4; U3 rewrote 64 clean pages); UPDFLG 1 writes the modified
    ones, the caller's valid PTEs' and, out of its working set, those on
    the modified list or valid with the saved modify bit (U5). Pages out
    are never written (unprobed). `TestGlobalSectionPaging`'s
    checkpoint 2 now expects the run up to the first page out.
  - **Deleted sections wait** (`corevms/gblsec.go`): a page-file or
    copy-on-reference section with nothing mapping or keeping it leaves
    the name table at once but goes on `GlobalSections.pending`
    (`PendingSections`), still counted in GBLSECTIONS, GBLPAGES, and
    GBLPAGFIL and holding its frames and page file blocks, until
    `ReleasePendingSections` (`releaseSection`, the old teardown): at the
    next global section's creation and at image rundown (`DeleteP0`). A
    file section goes at once, as G3's did. Why VMS waits, and when else
    it releases them, is unknown (govax's choice, unconfirmed).
    `TestCrmpsc_lifetime`, `TestPagingGlobalSectionFrames`, and
    `TestPageFileGlobalSection` check the section pending, then
    released; `checkPageFileHolders` counts pending sections' blocks.
  - **PAGTBLPFC** is 2 (SYSGEN's default, the VAX's) in `$GETSYI` and
    F$GETSYI.
  - **Differences that stay:** VMS's pageable process page tables (a few
    more faults and PPGCNT at each P0 expansion; Phase 67), the working
    set's floor (parts P and K2: at the floor plus 20 VMS's list had no
    free entry for a cluster), part D's writer, G1's sections in use.
  - The `.out` files are force-added past `.gitignore`, as pg64's.
- **2026-10-18:** Subtask 8, the close-out. CLAUDE.md describes the
  phase (the paging program's paragraph; `internal/paging`'s
  `ShareFirst` and `AddAvailable`; corevms's copy-on-reference pages,
  global page states, page-file sections, limits, `$UPDSEC`, pending
  sections, and read clustering; the `testdata/pg65` entry); PLAN.md's
  row is done; HELP's VMINIT/PAGING describes section pages and
  clustering, and HELP LEXICAL F$GETSYI's global section items; Phase
  66's doc lists what this phase leaves it (the CRF rule an image
  section's modified pages need, `processSection.pfc` for an ISD's
  cluster factor, the scan through `resolveFault`, pending sections
  released at rundown), and Phase 67's the page table differences the
  probe found. The open questions above are answered: clustering is
  visible in the fault counts and follows the book (the probe's K
  lines), and a permanent page-file section keeps its contents while
  unmapped and through the writer (F and R).
  **Carried forward:** the order of the GBLSECTIONS, GBLPAGES, and
  GBLPAGFIL checks (GBLPAGES was never near full on the VAX); why VMS
  holds a deleted page-file or CRF section's entries and blocks, and
  when it releases them besides the next creation (govax adds image
  rundown); `$UPDSEC` of pages out of memory (not written) and UPDFLG 1
  for pages another process modified (written if on the modified list);
  a CRF mapping past PGFLQUOTA (govax maps the pages the quota allows,
  then SS$_EXQUOTA); read clustering from the page file and over a
  page-file section's written pages, and a PFCDEFAULT of 0, all
  unprobed (that VAX never reused frames); an image file's clusters
  (Phase 66); VMS's pageable process page tables and the DIRIO of 2
  beside them (Phase 67, Decision 13); the working set floor behind
  parts P and K2 (pg63's finding: govax's list has no permanent part).
