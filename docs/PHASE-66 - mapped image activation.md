# Phase 66 — image activation by mapping, and image reset

**Status:** planned (2026-10-15). Needs Phase 65 (sections under the
pager, read clustering). Part of the paging program laid out in
`PHASE-62 - VMS paging.md` (Decision 15 is this phase's).

## Goal

With paging on, RUN, `$CREPRC`, and shareable image loading **map** an
image instead of copying it, as VMS's image activator does (21.1):

- each process-private ISD becomes a process section table entry for
  the image file, flagged as an image section. Its PTEs hold the PSTX,
  and its pages come in when the program, or the fixup pass, touches
  them;
- a DZRO ISD's pages get demand-zero PTEs;
- an image section is never written back to the image file. A page
  modified after its fault takes the page file as backing store: VMS's
  rule for copy-on-reference and read-only sections. Writable non-CRF
  private image sections, which the linker doesn't produce, are treated
  as CRF;
- the image file stays open while the image runs, read at fault time.
  That's an `rms.ACPFile` for a volume image and an `os.File` for a host
  image; the author chose VMS's behavior for host images too;
- **image reset** at rundown deletes P0, as VMS's does (16.2, 21.3).
  That covers the pages, their frames and page file blocks, and the
  image's sections; the files are let go and the working set returns to
  WSDEFAULT.

With paging off, activation copies as it does today. (Rundown deleting
P0 is already done, in both modes: Phase 63's probe found govax kept an
image's pages after its exit where VMS's PPGCNT fell, and
`corevms.Environment.DeleteP0` came in then; see "What earlier phases
leave in place".)

## What earlier phases leave in place

- **Phase 63 (2026-10-16):** image rundown deletes P0 in both modes
  (`Environment.DeleteP0`: every P0 page back to `ProcessPTE`'s form, its
  frame released through `releasePTE`), but for an image the debugger
  holds, whose pages stay until the next activation (`KeepP0`,
  `ResetICBList`). The whole suite passed with it; the tests that found
  their P0 scratch data gone after a rundown were changed.
- **Phase 64 (2026-10-18):** the image's part of P1 is deleted at
  rundown too, as Phase 64's probe found VMS's PAGFILCNT back where it
  was after each image (`vaspace.go`'s `deleteImageP1`, called by
  `DeleteP0`: the pages from P1's low end up to the user stack's
  reserve, freed and their PGFLQUOTA refunded, and the low end reset).

- **`machine/image.go`:** `ReadImage` (the whole file in Go, still used
  for the header, the ISD list, the fixups' tables, and debug symbols),
  `imageLoad`, `imageFixup`, `setImageProtection`, `ResetICBList`, and
  `ActivateCreatedImage`.
- **Phase 65 (done 2026-10-18):** private sections with PSTX pages; a
  copy-on-reference section's first fault claims the frame with page
  file backing, effectively modified (`processSection.copyOnRef`,
  `copyOnRefBacking`), which is the rule an image section's modified
  pages need; CRF pages charged at mapping (`mapChargedPage`), as an
  image's writable pages are already charged at load. Read clustering
  (`corevms/cluster.go`'s `readCluster`) takes a private section's pfc
  from `processSection.pfc` (0 for PFCDEFAULT, `vax.sysgen.pfcdefault`;
  at most 127): an image section gets its ISD's cluster factor there.
  Its scan reads neighbors through `resolveFault`, so an image file's
  read (an `os.File` for a host image) needs only the section's page
  reader. Image rundown (`DeleteP0`) also releases the global sections
  left pending (`ReleasePendingSections`).
- **Phase 49:** `holdFile`/`dropFile`, which keep a section's file
  accessed after its channel goes.

## Design notes

- **The page source** is an interface the process section entry holds:
  `ReadPage(vbn, buf)`. It's implemented by `rms.ACPFile` (volume) and a
  host file, opened when the image is read and kept by the ICB until
  rundown. `ReadImage`'s Go copy is no longer where pages come from.
- **The header is not written to P0 in paging mode** (the author,
  2026-10-15, after first leaving it for later). A closer look shows it isn't kept in P0 at all.
  `imageLoad` writes the header blocks at the image's base plus 0x200
  only to read the transfer offset, ident offset, and ISDs back with
  `loadWord`. It doesn't move `RegionSize` for them. The image's own
  pages (an executable's first ISD is at virtual page 1, address 0x200;
  a shareable image's sections start at its base) are then written over
  the same addresses. So the header is a scratch copy, and no program or
  RTL code can depend on finding it there. That matches VMS, where the
  activator reads the header into a buffer of its own.

  Under mapping, writing it there would cost something. Writing first
  would fault in demand-zero pages, counted in PAGEFLTS and charged to
  PGFLQUOTA, that the mapping replaces at once. Writing after the mapping
  would dirty image pages and give them page file backing. The
  rule for subtask 3: in paging mode, parse the header from
  `ReadImage`'s Go copy and don't write it to P0. The only visible
  difference is in pages inside the header's range that no ISD covers,
  which read as zeros rather than header bytes. Subtask 1 checks that no
  image govax runs relies on that. With paging off, the header goes to
  P0 as today.
- **Protection and fixups:** the pages are mapped UW, the fixup pass
  writes them through faults, and then `setImageProtection` gives each
  section its protection *in every PTE form*. Today it skips pages
  that aren't valid, which would be most of a mapped image.
- **What image reset deletes in P1:** VMS deletes the part of P1 the
  image created. In govax that's what `$EXPREG` region 1 and
  `SEC$M_EXPREG` added below the user stack's reserve (`expandP1`),
  which `deleteImageP1` already deletes (Phase 64). The survey checks
  nothing else in P1 belongs to an image.
- **Process 1's P0 after rundown:** the console's own uses of P0
  (ASM and DEPOSIT into process 1's P0 before a RUN) are lost at the
  next image reset, as on VMS. The survey looks for tests that rely on
  P0 surviving a run.
- **Fault counts at activation won't match VMS exactly:** VMS's
  shareable images (LIBRTL and the rest) are installed and shared, and
  its activator works in P1 buffers, while govax's RTL is shims. The
  probe measures the difference, and it's listed rather than chased.

## Open questions

- **Host shareable images:** load mapped and private per process, as
  today's copies are. Installing them as shared global sections is future
  work.

## Subtasks (provisional)

1. **Survey:** chapter 21.1's activator and 21.3's image reset; the ISD
   flags govax's images use (CRF, WRT, DZRO, GBL, FIXUPVEC, PROTECT,
   the cluster factor); the header question; tests that rely on P0 or P1
   surviving a run.
2. **The page source:** a volume image as a held `ACPFile`, a host image
   as a host file, kept by the ICB.
3. **Mapping the ISDs:** image section entries, PSTX and DZRO PTEs, the
   header blocks, and the ISD's cluster factor; fixups through faults;
   `setImageProtection` over every PTE form.
4. **Shareable images** the same way.
5. **Image reset:** let the files go (deleting P0 and the working
   set's reset are done, Phase 63, and the image's part of P1, Phase
   64); `ResetICBList`'s reload-over-old-pages
   workaround is not needed in paging mode.
6. **`$CREPRC`'s activation** (`ActivateCreatedImage`) through the same
   path.
7. **The full suite with paging on:** every oracle test's images run
   mapped, with a small memory and small working sets. Then the VMS
   probe: PAGEFLTS around the run of a small image (F$GETJPI before and
   after RUN, and LIB$INIT_TIMER/SHOW_TIMER inside).
8. **Close-out.**

## Progress log

- **2026-10-15:** Planned with Phase 62's review.
- **2026-10-15:** The author: leave the header mapped for now and decide
  later. The design notes record that it's a scratch copy overwritten by
  the image's own pages, and recommend not writing it in paging mode.
- **2026-10-15:** The author agreed: the header isn't written to P0 in
  paging mode.
