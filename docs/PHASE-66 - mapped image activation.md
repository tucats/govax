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

With paging off, activation copies as it does today, and rundown leaves
P0 alone.

## What earlier phases leave in place

- **`machine/image.go`:** `ReadImage` (the whole file in Go, still used
  for the header, the ISD list, the fixups' tables, and debug symbols),
  `imageLoad`, `imageFixup`, `setImageProtection`, `ResetICBList`, and
  `ActivateCreatedImage`.
- **Phase 65:** private sections with PSTX pages, CRF to the page file,
  and read clustering with a per-section PFC (an ISD's own cluster
  factor, when the linker gave one).
- **Phase 49:** `holdFile`/`dropFile`, which keep a section's file
  accessed after its channel goes.

## Design notes

- **The page source** is an interface the process section entry holds:
  `ReadPage(vbn, buf)`. It's implemented by `rms.ACPFile` (volume) and a
  host file, opened when the image is read and kept by the ICB until
  rundown. `ReadImage`'s Go copy is no longer where pages come from.
- **The header:** govax puts the image header in P0 at the image's base
  (VMS reads it into a buffer of its own, not P0), and `imageLoad` reads
  the transfer and ident offsets from there. Mapping the header blocks
  as image section pages keeps that working. The alternative, reading
  them from the Go copy and not mapping them at all, would change P0's
  layout and every address after it. Subtask 1 decides.
- **Protection and fixups:** the pages are mapped UW, the fixup pass
  writes them through faults, and then `setImageProtection` gives each
  section its protection *in every PTE form*. Today it skips pages
  that aren't valid, which would be most of a mapped image.
- **What image reset deletes in P1:** VMS deletes the part of P1 the
  image created. In govax that's what `$EXPREG` region 1 and
  `SEC$M_EXPREG` added below the user stack's reserve (`expandP1`).
  The survey checks nothing else in P1 belongs to an image.
- **Process 1's P0 after rundown:** the console's own uses of P0
  (ASM and DEPOSIT into process 1's P0 before a RUN) are lost at the
  next image reset, as on VMS. The survey looks for tests that rely on
  P0 surviving a run.
- **Fault counts at activation won't match VMS exactly:** VMS's
  shareable images (LIBRTL and the rest) are installed and shared, and
  its activator works in P1 buffers, while govax's RTL is shims. The
  probe measures the difference, and it's listed rather than chased.

## Open questions

- **The header in P0:** map it (keeping today's layout), or stop putting
  it there?
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
5. **Image reset:** delete P0 and the image's part of P1, let the files
   go, and reset the working set; `ResetICBList`'s reload-over-old-pages
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
