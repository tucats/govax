# Phase 66 — image activation by mapping, and image reset

**Status:** done (2026-10-19). Subtasks 1 to 8: the survey (Part A,
its questions answered by the author); the page source; mapping the
ISDs (image sections, demand-zero pages, protection over every PTE
form, the change-protection list); shareable images; image reset;
`$CREPRC`'s and a subprocess's activation; the suite run with paging
forced on, and the probe, checked against VMS 7.1 (`testdata/pg66`);
the close-out. Needs Phase 65 (sections under the
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
  work. **Settled** (subtask 4): mapped privately after the main image;
  installing them shared stays future work.

# Part A — the survey (subtask 1, 2026-10-18)

## Sources

- ***VAX/VMS Internals and Data Structures*** (the OCR text, beside the
  PDF): 21.1 (image initiation: "no special code to read images into
  memory"), 21.1.1 (the activator's arguments, among them `hdrbuf`, the
  512-byte buffer the header is read into), 21.1.1.3 (activating an
  image with no global sections, steps 1 to 11; figures 21-1 to 21-4:
  the header, the ISD and its flags, the PSTE and PSTX PTEs of a private
  section, the demand-zero PTEs of a DZRO section), 21.1.1.4 (shareable
  images: `$MGBLSC` when installed, else a private `$CRMPSC`), 21.1.2
  (`$IMGFIX`), 21.3 and 21.3.1 (rundown, step 7: MMG$IMGRESET; figure
  21-11, the part of P1 deleted at image exit), 15.2.1 (faults on image
  pages, not copy-on-reference and copy-on-reference; figure 15-3),
  15.5.1.3 (the cluster factor of a process section: the ISD's, or
  PFCDEFAULT), and appendix B.1.14 (the ISD's fields).
- **govax:** `machine/image.go` (`imageLoad`, `readISD`, `loadSection`,
  `imageFixup`, `setImageProtection`, `ResetICBList`), `machine/run.go`
  (`Run`, `ActivateImage`, `ActivateCreatedImage`, `ImageRundown`, the
  IMAGE$INIT driver), `machine/export.go` (`WriteBytes`),
  `corevms/filesec.go` (process sections, `holdFile`/`dropFile`,
  `releasePTE`, `resolveFault`), `corevms/vaspace.go` (`DeleteP0`,
  `deleteImageP1`), `corevms/process.go` (`ImageRundown`),
  `rms/acp.go` (`ACPFile`), `vmsimage/image.go`, `link/image.go`.
- **The images govax runs:** every `.EXE` under `testdata` that the
  tests use (68 images: `testdata/exe`, `dbg`, `dbgcmd`, `link`, and
  `mar`'s, real LINK's and govax's), decoded with `vmsimage.ReadImage`
  by a throwaway program (not kept).

## The activator on VMS (21.1.1.3)

1. The image file is **opened as a process-permanent file**, and its
   header read into the activator's own buffer (`hdrbuf`), never into
   the image's address space.
2. Each ISD becomes a call of a memory management service:
   - a **private section** (not GBL, not DZRO) is a `$CRMPSC` of the
     image file: one PSTE for the section (figure 21-3: the file's WCB,
     the base VBN, the VPN, the page count, the pfc), and a PSTX PTE per
     page, protection UR or UW. The pages come in at their first fault,
     "from the appropriate blocks in the image file";
   - a **demand-zero section** is a `$CRETVA`: demand-zero PTEs, UW;
   - the **user stack** ISD (type 253) is put off until step 7;
   - a **global section** ISD calls the activator again for the
     shareable image, which maps each of that image's own ISDs with
     `$MGBLSC` if it's installed, or a private `$CRMPSC` if it isn't.
3. The image I/O segment (IMGIOCNT pages) and the user stack
   (`$EXPREG`, 20 pages by default, `STACK=`) are created in P1.
4. Privileges and the SYS.STB check; then `$IMGFIX` resolves G^ cells
   and `.ADDRESS` longwords (21.1.2), which are pointers *in* the image,
   written in the image's access mode.

So nothing is read at activation but the header: every page of the
image, code and data, the fixup section included, comes in by a page
fault, `$IMGFIX`'s writes faulting in the pages they write.

**Image pages' faults (15.2.1).** A PSTX PTE with PTE<16> (CRF) clear
is a page whose backing store is the image file: read in, it stays
backed by the file, a clean page leaves to the free list, and a fault
after its frame is reused reads the file again. "Writeable pages that
are not copy on reference are written back to the image file", but such
sections "are not usual products of the linker". With CRF set, the
first fault's frame gets the saved modify bit and the page file as
backing store, "all ties to the original image file are broken"
(Phase 65's `copyOnRefBacking` already does this for `$CRMPSC`'s CRF
sections).

**Image reset (21.3.1, step 7, MMG$IMGRESET):** all of P0 is deleted,
"freeing the image file and any other file that is mapped", frames
released and page file blocks deallocated; the non-permanent part of P1
(the user stack, the image I/O segment, and P1 expansions below them)
is deleted; the working set returns to its default; privileges are
reset; released global sections are deleted. Step 8 then deassigns the
channels the deletion freed, the image file's among them.

## The ISDs govax's images have

Every one of the 68 images is an executable (IHD type 1, no shareable
image among them: govax's LINK writes only executables, `link/image.go`)
with **one header block**. Their ISDs have seven forms:

| Flags      | Meaning                                | ISDs | Activation today      |
|------------|----------------------------------------|-----:|-----------------------|
| `00000080` | read-only private (LASTCLU)            |   80 | copied, uncharged, UR |
| `0000008A` | writable, CRF (LASTCLU)                |   58 | copied, charged, UW   |
| `0000040A` | the fixup section: writable, CRF       |   68 | copied, charged, UW   |
| `0000008C` | demand zero, writable (LASTCLU)        |   16 | zeroed, charged, UW   |
| `FD00008C` | the user stack (type 253), DZRO        |   68 | skipped               |
| `03000021` | global, type 3 (shareable PIC), LEQUAL |   44 | skipped (shims)       |
| `0400002B` | global, type 4 (private PIC), WRT, CRF |    3 | skipped (shims)       |

- **No writable non-CRF section**, so the design notes' "treated as
  CRF" rule has nothing to apply to yet; **no PROTECT, VECTOR, or
  BASED** ISD; the global ISDs are all shareable image references,
  which govax resolves to its shims (or a host shareable image, below)
  and doesn't map.
- **The cluster factor is 0 in every ISD**, so every image section
  reads clusters of PFCDEFAULT. govax's LINK has no `CLUSTER=` option
  (its ISDs' pfc is always 0); only an image linked on VMS with one
  would test the ISD's own factor.
- **No section runs past the file's allocation** (VBN + pages - 1 never
  exceeds the file's blocks), but `ACPFile.ReadPage` fails past it where
  today's copy zero-fills (finding 7).
- **Fixups write only CRF pages.** Every G^ cell is in the fixup section
  (CRF), and the fixtures' one `.ADDRESS` longword is in a writable CRF
  section. So in these images the fixup pass never modifies a read-only
  section's page.
- **The change-protection list.** Every fixup section has one entry:
  the fixup section's own page (an address relative to the image base;
  so in all 68), code 13, PRT$C_UREW. VMS's activator makes the fixup page user-read,
  executive-write after `$IMGFIX`; govax reads `OffsetChgprot`
  (`readIAF`) but never applies the list, and the page stays UW
  (`imageProtection` of a WRT section). Finding 5.

## The header question

- **Every image has one header block, and the first ISD of each is at
  VPN 1**, so the header's scratch copy at ^X200 is always overwritten by
  the image's own first page: no image govax runs has a page in the
  header's range that no ISD covers.
- **Nothing reads the header from memory after `imageLoad`:** the ICB's
  users (`export.go`, `shared.go`, `show.go`, the debugger's
  `program.go`) use `Base`, `End`, and the ISD list; the debug symbols
  come from `ReadImage`'s Go copy; the IMAGE$INIT driver is in S0
  (`driverAddress`: CONSOLE$SCRATCH for process 1, a pool page for
  others), not P0.
- So the rule the design notes give stands: with paging on, parse the
  header from the Go copy and don't write it to P0. The only image kind
  it could matter for is a shareable image, whose header copy goes at
  base + ^X200, its own page 1; govax runs none from its tests.
- **`readISD` is the C port's, and has two gaps** that parsing from the
  Go copy closes: it reads the VPN as a word (the ISD's is 23 bits, the
  pfc in its top byte), and it doesn't know the 0xFFFF size that
  continues the list in the next header block (`vmsimage.sections`
  does). Neither shows with one header block and P0 VPNs under 64K.
  Finding 3.

## What govax has today

- **The copy.** `imageLoad` reads the whole file in Go (`ReadImage`:
  `rms.Session.ReadRawFile` for a volume image, `os.ReadFile` through
  `Paths` for a host one), writes the header blocks at base + ^X200,
  reads the transfer and ident offsets and the ISDs back from P0, then
  for each private ISD writes every page (`loadSection`, charged only
  for WRT sections) and zeroes each DZRO page. Each write is the page's
  first touch: with paging on, a demand-zero fault counted in PAGEFLTS,
  claimed with page file backing, and a full working set evicts pages
  of the image being loaded. Nothing keeps the file: a volume image is
  read without an ods2 access, so while it runs it can be written,
  deleted, or its volume dismounted.
- **Fixups** (`imageFixup`) read the IAF and the lists through
  `ImageProcess.loadLong`/`storeLong`, which go through `vm.LoadIn` and
  `StoreIn` with kernel-mode protection. `TranslateIn` calls the pager
  on an invalid PTE (`resolvePTE`), so a mapped image's fixups will
  fault the pages in, in the image's own process, whichever is current
  (`PageFault.Space`; `processForSpace`).
- **Protection** (`setImageProtection`) is set after the fixups, but
  only on pages that are valid or in transition (`touched`); with
  mapping most pages hold a PSTX or are demand-zero, and would keep
  their mapping protection.
- **Process sections** (Phase 49, 65) are what an image section needs,
  almost: `processSection` has the file and VBN, the first address,
  which pages are still mapped, `writeBack`, `copyOnRef`, and `pfc`;
  faults read through `sys.readPage(ps.file, ...)`, clusters through
  `resolveFault`; `releasePTE` unmaps a page, and the section goes with
  its last page, dropping its file (`maybeDropProcessSection`,
  `dropFile`). But `file` is a `*rms.ACPFile`, as `holdFile`/`dropFile`'s
  map key is, so a host image needs a page source behind an interface
  (subtask 2). Only `writePage` (for `writeBack` sections, `$UPDSEC`,
  and the writer) needs a file that can be written; an image section
  never does.
- **Rundown.** `Environment.ImageRundown` deletes P0 (`DeleteP0`, every
  page not in `ProcessPTE`'s form released through `releasePTE`, which
  for a PSTX page unmaps it from its section), the image's P1 pages
  (`deleteImageP1`), and resets the working set. So a mapped image's
  sections, and their files, go at rundown with no new code: a section
  drops its file when the last of its pages, touched or not, is
  released. The debugger's `KeepP0` keeps them, files and all, until
  the next activation's `ResetICBList` calls `DeleteP0`.
- **What else is in P1 for an image:** nothing govax makes per image
  but what `deleteImageP1` already deletes (`$EXPREG` region 1 and
  `SEC$M_EXPREG` expansions). govax has no image I/O segment, and the
  user stack ISD is ignored: the user stack is the process's fixed
  reserve below `UserStackTop` (`userStackPages`), not created at
  activation or deleted at rundown, so its pages and their contents
  survive from one image to the next. That is Phase 67's (its design
  notes: the initial stack created and charged, the rest of the reserve
  room to grow), not this phase's.
- **The debugger's DEPOSIT** (`Machine.WriteBytes`) writes a read-only
  page (an image's code) by translating for read and storing into the
  frame, leaving the PTE's modify bit clear. Under mapping that page is
  clean with the image file as its backing store: once it leaves the
  working set and its frame is reused, the next fault reads the file
  and the deposit is gone. (Breakpoints aren't affected: they're PC
  checks in the run loop, not BPTs written into the code.) Finding 6.

## Tests that rely on P0 or P1 surviving a run

- **P0 before a RUN** is already lost in both modes: `Run` calls
  `ResetICBList`, which calls `DeleteP0` (Phase 63), before activating.
  A test that ASMs or DEPOSITs into process 1's P0 and then RUNs would
  already fail; the suite passes, so none does.
- **P0 after a RUN** is lost at rundown in both modes since Phase 63,
  but for a debugger session (`KeepP0`); the tests that found their P0
  scratch data gone were changed then.
- **P1 after a RUN:** since Phase 64 the image's expansions go;
  the user stack's reserve stays (above). No test reads P1 below the
  stack's top after a run.
- **What mapping changes** is only with paging on, and only in counts
  and frame states, not contents: about 20 test files run images with
  paging on (`pg63_test.go`, `pg64_test.go`, `pg65_test.go`, the
  `workingset*`, `pagefile*`, `pgflquota`, `clusterpaging`,
  `globalpaging`, `globalcrf`, `pagingframes`, `setworkingset` tests,
  and `cmd/govax/stats_test.go`). Those that compare counts across an
  image's activation (PAGEFLTS, PAGFILCNT, frames tied to the process,
  `--stats`' figures) will move: activation no longer takes a fault per
  image page or claims a frame for pages the program never touches.
  Most compare differences inside a running image, which don't move;
  subtask 7 runs them all and changes what's expected where the new
  count is VMS's (the probe decides).

## Findings that change the plan

1. **The page source** (subtask 2): `processSection.file` becomes an
   interface (`ReadPage(vbn, buf) error`, and an identity for the
   held-file map), with writing only through a separate interface that
   `writeBack` sections' files have (`rms.ACPFile`). A volume image is an
   `rms.ACPFile` from `MountTable.ACPAccessWith` (read access; the
   sharing is question 2), a host image an `os.File` wrapped to read
   blocks. `ReadImage` still reads the file in Go for the header, the
   ISDs, and the debug symbols (the fixup lists are read through the
   faulted pages).
2. **No new rundown code** (subtask 5 shrinks to tests): `DeleteP0`
   releasing every PSTX page drops each image section and its file; the
   ICB need hold the source only so that a section's file can be
   dropped by the section, not the ICB. `ResetICBList`'s
   `setImageProtection(icb, false)` is unneeded with paging on (the
   pages are deleted right after), harmless with it off.
3. **The ISDs come from `vmsimage.ReadImage`** in paging mode (subtask
   3): its decoding has the 23-bit VPN, the pfc, and the continuation
   blocks, which `readISD` lacks. The header isn't written to P0.
   Each private ISD maps through `mapProcessSection` (UW for every
   section until the fixups are done; `copyOnRef` for WRT, which every
   writable section the linker makes is anyway; `pfc` from the ISD),
   CRF pages charged at mapping (`mapChargedPage`, Phase 65), DZRO pages
   demand-zero and charged at creation, as `$CRETVA`'s, read-only pages
   uncharged (Phase 64, finding 12). RegionSize[0] and `icb.End` move by
   the ISDs' extent as `loadSection` moves them now.
4. **`setImageProtection` over every PTE form** (subtask 3): a PSTX,
   demand-zero, transition, or page file PTE takes the section's
   protection as a valid one does; pages of a GBL or user-stack ISD are
   still skipped.
5. **The change-protection list** (subtask 3, question 1): after the
   fixups, each entry's pages take its PRT$C_ code (the fixup page
   UREW), as VMS's `$IMGFIX` sets them.
6. **A modified non-CRF image page** takes the page file as backing
   store (the design notes' rule; subtask 3): no image the tests run
   writes one from the program (they're UR), but the debugger's DEPOSIT
   does, and `WriteBytes` must set the PTE's modify bit when it writes
   past the protection, so the change survives the page leaving the
   working set (question 3).
7. **Pages past the file's allocation** read as zeros for an image
   section (question 4), as the copy fills them today, rather than
   failing the fault as `ACPFile.ReadPage` does for `$CRMPSC`.
8. **Shareable images** (subtask 4): only a host shareable image found
   through `SharePrefix` is ever loaded (`findImage`); the tests load
   none but `put.exe` as a "secondary" executable
   (`TestImageLoad_alreadyLoadedIsNoOp`). Subtask 4 maps a shareable
   image's ISDs at its base exactly as subtask 3's, privately, and
   needs a test image of type 2; govax's LINK can't make one, so the
   test builds one by hand or uses an executable as today's test does.
9. **The user stack** (type 253 ISD, `STACK=`) stays the process's
   reserve, not created per image: Phase 67's.

## Questions for the author

Answered by the author on 2026-10-18: each recommendation accepted. The
findings and subtasks follow the answers.

1. **The change-protection list** (finding 5): apply it after the
   fixups, making each fixup page UREW as VMS does? It's activation's
   protection, not paging, so *recommended: yes, in both modes* (with
   paging off nothing else changes; a program writing its own G^ cells
   from user mode would then get an ACCVIO, as on VMS). The probe checks
   the page's protection with PROBEW. **Answer:** yes, in both modes.
2. **Sharing the held image file:** VMS opens it as a process-permanent
   file; the Internals book doesn't say with what sharing.
   *Recommended:* read access, others allowed to read but not to write
   (ods2's `NoWrite`), so a `$CREATE`/OPEN of the same file for writing
   gets RMS$_FLK (and DISMOUNT of its volume is refused, its file open)
   while the image runs; unconfirmed until the probe tries it. A host
   image is held open, its writers not stopped. **Answer:** yes, read
   access sharing reads only; the probe confirms.
3. **DEPOSIT into a read-only page** (finding 6): set the PTE's modify
   bit when the debugger writes past a page's protection, so that the
   page keeps the change (backed by the page file from then on)?
   *Recommended: yes,* with paging on only (with it off a page never
   leaves memory, so nothing changes). **Answer:** yes, paging on only.
4. **Past the file's allocation** (finding 7): zeros for an image
   section's page there? *Recommended: yes,* as the copy does; no image
   the tests run has one. **Answer:** yes.

## The probe's cases (subtask 7)

`testdata/pg66`: a small image with read-only code, a writable CRF data
section, a DZRO section, and a fixup section (G^ calls of LIBRTL),
linked on VMS (with a second link using `CLUSTER=` with a pfc), and a
procedure. Counts print as differences.

- **PAGEFLTS around RUN:** F$GETJPI's PAGEFLTS (and PPGCNT, PAGFILCNT)
  before and after RUN of the image, run twice in a row; LIB$INIT_TIMER
  at the image's first instruction and LIB$SHOW_TIMER's faults inside
  it; the same with a 100-page read-only section the image reads in
  order, with the default pfc and the `CLUSTER=` one.
- **Charges at activation:** PAGFILCNT at the first instruction against
  before RUN (the writable and DZRO pages charged, the read-only ones
  not), and after the image exits (all back).
- **Protection after activation:** PROBER/PROBEW in user mode of a code
  page, a data page, a DZRO page, and the fixup page (question 1).
- **The image file while it runs:** `$OPEN` of the image's own file
  (JPI$_IMAGNAME) for read with SHR=GET, for read without sharing, and
  for write (question 2); in a subprocess, DELETE of the file while the
  image waits, then the image touching a page it hasn't (does it still
  read?).
- **At a small working set** (`$ADJWSL` to near the floor), touching
  every code page twice (the second pass's faults: soft from the free
  list, or hard from the image file once the frames are reused).

## Subtasks (provisional; adjusted by the survey)

1. **Survey** (done 2026-10-18): Part A.
2. **The page source** (finding 1): `processSection`'s file behind an
   interface, `holdFile`/`dropFile` on it; a volume image as a held
   `ACPFile` (question 2's sharing), a host image as a host file.
3. **Mapping the ISDs** (findings 3 to 7): from `vmsimage.ReadImage`,
   with paging on; image section entries, PSTX and DZRO PTEs, no header
   in P0, the ISD's cluster factor; fixups through faults;
   `setImageProtection` over every PTE form; the change-protection list
   (question 1); a modified non-CRF page to the page file, DEPOSIT's
   modify bit (question 3); zeros past the allocation (question 4).
4. **Shareable images** the same way (finding 8).
5. **Image reset** (finding 2): the tests that the files go at rundown
   (and with the debugger's `KeepP0`, at the next activation), and that
   `ResetICBList` needs nothing more in paging mode.
6. **`$CREPRC`'s activation** (`ActivateCreatedImage`) and a
   subprocess's RUN through the same path (both already reach
   `ImageProcess.ActivateImage`; the tests).
7. **The full suite with paging on** (done 2026-10-19): every oracle test's images run
   mapped, with a small memory and small working sets; the paging-on
   tests' activation counts changed where they move. Then the VMS
   probe (`testdata/pg66`, the cases above), and govax changed to its
   answers.
8. **Close-out** (done 2026-10-19).

## Progress log

- **2026-10-15:** Planned with Phase 62's review.
- **2026-10-15:** The author: leave the header mapped for now and decide
  later. The design notes record that it's a scratch copy overwritten by
  the image's own pages, and recommend not writing it in paging mode.
- **2026-10-15:** The author agreed: the header isn't written to P0 in
  paging mode.
- **2026-10-18:** Subtask 1, the survey (Part A): the Internals book's
  activator (the header read into a buffer of its own, each private ISD
  a `$CRMPSC` of the image file with PSTX PTEs, DZRO ISDs `$CRETVA`,
  every page brought in by faults, `$IMGFIX`'s included), image pages'
  faults (non-CRF pages backed by the image file, CRF pages by the page
  file from their first fault), and image reset (P0 and the
  non-permanent P1 deleted, the image file let go, the working set
  reset). The 68 images the tests run: all executables with one header
  block; read-only, writable CRF, fixup (CRF), DZRO, user stack, and
  global ISDs; no writable non-CRF, PROTECT, or BASED section; every
  pfc 0; fixups write only CRF pages; each fixup section's
  change-protection entry (UREW) is never applied. The header's copy in
  P0 is always overwritten by page 1 and nothing reads it. Found:
  `readISD` reads the VPN as a word and doesn't follow the ISD list
  into a second header block; a volume image is read without an access,
  so nothing holds it while it runs; rundown needs no new code (the
  sections, and their files, go with P0's pages); the debugger's
  DEPOSIT into a read-only page leaves it clean, so under mapping the
  deposit would be lost when the page's frame is reused; the user stack
  stays the process's reserve (Phase 67). No test relies on P0 or P1
  surviving a run; the paging-on tests' activation counts will move.
  The subtasks are adjusted, the probe's cases listed, and four
  questions are for the author.
- **2026-10-18:** The author accepted the survey's findings, the
  revised subtasks, and each recommendation: the change-protection list
  applied after the fixups in both modes (the fixup page UREW); the held
  image file read with others allowed to read but not write (the probe
  confirms); the debugger's DEPOSIT past a page's protection sets its
  modify bit, with paging on only; an image section's pages past the
  file's allocation read as zeros.
- **2026-10-18:** Subtask 2, the page source. `corevms/pagesource.go`:
  a section's file is a `PageSource` (`ReadPage(vbn, buf)`,
  `Deaccess`), which `processSection.file`, `holdFile`/`dropFile`/
  `orphanFile` (`System.sectionFiles` is keyed by it), `readPage`, and
  `mapProcessSection` take; writing back goes through `pageWriter`
  (`WritePage`), which `writePage` asks the source for (an error for a
  read-only one), so only an accessed volume file can be written.
  `GlobalSection.File` stays an `*rms.ACPFile`. A host file is a
  `HostPageSource` (an `io.ReaderAt` and its size: blocks of 512 bytes,
  the last filled out with zeros, nothing past it; `Deaccess` closes
  it). `System.ReleaseSource` is the activator letting go of the file
  once its sections are mapped: left accessed while a section uses it
  (`orphanFile`), else let go at once. `rms/imagefile.go`'s
  `Session.AccessImage` finds a volume image as `ReadRawFile` does and
  accesses it for reading with others allowed only to read
  (`ACPAccessMode{NoWrite: true}`, question 2), returning the
  `ACPFile` and the file's bytes; while it's held an RMS writer gets
  RMS$_FLK, an IO$_ACCESS for writing ACCONFLICT, and DISMOUNT is
  refused. `machine/imagesource.go`'s `Machine.OpenImage` is
  `ReadImage` returning the held source with the bytes (a volume image
  through `AccessImage`, a host image found by `findImage` as a
  `HostPageSource`); subtask 3's loader uses it with paging on. Tests:
  `TestAccessImage` (rms), `TestHostPageSource`, `TestReleaseSource`,
  `TestSourcePages` (corevms), `TestOpenImage_volume`/`_host`
  (console). Nothing calls `OpenImage` yet, so no behavior changes.
- **2026-10-18:** Subtask 3, mapping the ISDs. With paging on and
  mapping enabled, the main image is mapped (`machine/imagemap.go`'s
  `imageMap`, chosen by `mapsImages`; a shareable image is still copied,
  subtask 4's). `OpenImage` holds the file; the header is decoded by
  `vmsimage.ReadImage` from the Go copy and never written to P0 (the
  transfer addresses from the activation offset, the ISDs with their
  23-bit VPN and pfc: `ISD.VPN` is now a `uint32`, `ISD.PFC` new); P0's
  high-water mark and the ICB's end move by each ISD's extent
  (`extend`). Each private ISD is `corevms.Environment.MapImageSection`
  (`corevms/imagesec.go`): a process section of the file flagged
  `image`, UW, user-owned PSTX PTEs written through the process's own
  page tables (`mapImagePage`, so the process needn't be current),
  copy-on-reference and charged at mapping for a writable ISD (the "treat
  as CRF" rule), the ISD's pfc as the section's. Each DZRO ISD is
  `CreateImagePages`: demand-zero PTEs, charged at creation. The
  activator lets its hold on the file go (`ReleaseSource`) once the
  sections are mapped. The loader's shared tail is `finishLoad` (debug
  symbols, the protection list, the shareable image list and its loads,
  `readSharedImages`), with `newICB` and `setTransfer` shared with the
  copying path, which is otherwise unchanged. The fixups fault in the
  pages they read and write. `setImageProtection` now sets every PTE
  that exists, whatever its form (`protectPages`, `pageExists`), and,
  after the sections', applies the fixup section's change-protection
  list (`ICB.Protections`, from `vmsimage`'s decoding; addresses
  relative to `imageLow`), in both modes (question 1): the fixup page is
  UREW. An image section's block past the file's allocation reads as
  zeros (`processSection.source`, `zeroPastEnd`; question 4). A modified
  page of a private section that doesn't write back (an image's
  read-only page, written by DEPOSIT) takes the page file as backing
  store when it goes to the modified list (`evictPage`'s
  `leaveReadOnlySection`), so the writer writes it there and its PTE
  takes the page file form when its frame is reused; `Machine.WriteBytes`
  sets the modify bit of a page it writes past its protection, with
  paging on (`markModified`; question 3). Tests: `TestMappedImage`
  (section PTEs, protections, the fixup page UREW, the sections counted,
  the writable and DZRO pages charged and nothing else, the volume held
  while the image runs, the sections and file gone after),
  `TestMappedImageOutput` (the G^ call works), `TestMappedImageDeposit`,
  `TestMappedImageCluster` (an image section's fault reads a
  PFCDEFAULT cluster), `TestZeroPastEnd` (corevms). Changed:
  `TestLink_helloPageProtection` expects the fixup page UREW;
  `TestModifiedPageWriter`'s program now stops after its add loop
  (checkpoint 5) and re-reads 14 pages down from the highest one on the
  modified list, as the old fixed range passed by one page's luck (the
  writer's last run decides which pages are still on the list, and the
  lazily faulted code moved it); `TestDeleteTransitionPages` records the
  `$DELTVA` range's frames, valid or in transition, just before the
  deletion, as more of the array is still valid at checkpoint 1 now.
- **2026-10-18:** Subtask 4, shareable images. With paging on, a
  shareable image the main image names is mapped as the main image is
  (`mapsImages` no longer asks which image it is): `OpenImage` finds it
  on the host (`findImage`, the shareable image prefix) and holds it as
  a `HostPageSource`, and `imageMap` maps its sections privately at P0's
  high-water mark after the main image, from its own VPN 0, as the
  book's 21.1.1.4 maps a shareable image that isn't installed. With
  paging off it's copied as before. Tests (`imageshare_test.go`):
  govax's LINK makes only executables, so the test makes a stand-in by
  hand from a govax-linked image of one read-only data page (its
  sections one page lower, so the first is at VPN 0; its transfer
  addresses cleared, since an executable's first, SYS$IMGSTA's, would be
  taken for the shareable image's initialization routine; its type
  IHD$K_LIM), saved as CRFSHR.EXE under the shareable image prefix. The
  main image's G^ reference to CRF$INSRTKEY (CRFSHR's offset 0, from
  `vmsdef.ImageSymbols`) gets the stand-in's base in its fixup cell.
  `TestMappedSharedImage`: the stand-in's page is a process section of
  its file, UR, in its section form until the main image reads it, its
  contents read right, its section gone at the image's end;
  `TestCopiedSharedImage`: the same with paging off. **Not yet checked:**
  a real shareable image (no user stack ISD, its own fixup section and
  G^ references, a transfer vector, more than one section); that needs
  an image linked /SHAREABLE on VMS (a probe image brought back as test
  data) or a LINK that makes shareable images.
- **2026-10-18:** Subtask 5, image reset. Finding 2 held for an image
  that ends by itself and for one abandoned (stopped and run down by the
  next RUN's `EndAbandonedImage`), but not under the debugger: image
  rundown unmapped every section page (`unmapSections`) before asking
  `KeepP0`, so an image ending under the debugger lost its sections, and
  with mapping its own code and data, which then read as demand-zero
  pages, while the session still showed the image. `unmapSections` is
  now `DeleteP0`'s first step, not rundown's, so `KeepP0` keeps the
  sections, and the file they hold, with the rest of P0, and the next
  activation's `ResetICBList` (`DeleteP0`) lets them go; process
  deletion still reaches it through `ImageRundown`. `ResetICBList` needs
  nothing more with paging on. Tests (`imagereset_test.go`, each ending
  with `checkImageReset`: no process sections, every P0 PTE in its fresh
  form, no frame tied to a P0 page of process 1, the working set at
  WSDEFAULT with no P0 page, the PFN database and working sets
  consistent, and the image's volume dismountable): `TestMappedImageReset`
  (an image that exits; its page file quota all back),
  `TestMappedImageResetAbandoned` (an image stopped with its array's
  pages valid, in transition, and on the modified list),
  `TestMappedImageResetDebugger` (RUN/DEBUG: the sections and the hold
  on the file kept after the image's end, DISMOUNT refused; then a
  second RUN of the same image from its file, with only its own
  sections, both runs' output, and nothing left after).
- **2026-10-18:** Subtask 6, `$CREPRC`'s activation and a subprocess's
  RUN. Both already reached the mapping path
  (`ActivateCreatedImage`, then `ImageProcess.ActivateImage` and
  `imageMap`, which maps through the process's own page tables and
  `OpenImage`'s current session), so no code changed; the tests
  (`imagecreated_test.go`) check it. `TestMappedCreatedImage`: a
  `$CREPRC` of the mapped-image program from the volume; at its first
  checkpoint the image's sections are the child's (process 1 has none),
  its never-run code still in its section form, every section's
  protection and the fixup page's UREW in the child's page tables, and
  the volume can't be dismounted; it runs to its end through a small
  working set (its G^ call says hello), and once the process is deleted
  and the CPU has left it (`awaitGone`: a deleted process's memory goes
  then) no frame is tied to its P0, the working sets are consistent, and
  the volume dismounts. `TestMappedSpawnedImage`: SPAWN/NOWAIT of a
  procedure that RUNs the image twice; each run is mapped into the
  subprocess, and each image's rundown takes its sections before the
  next RUN (the ICB list still the run's own), then the subprocess goes
  with nothing left. (Process 1's DCL leaves its CPU at no instruction
  of its own after lending it, so the test puts process 1 back in its
  spin loop, below the subprocess's priority: its 10-million-instruction
  quantum would otherwise keep the CPU, and expire no timers.)
  `TestMilestone_paging`: the multiprocessing milestone with paging on,
  by `$CREPRC` and by LIB$SPAWN, both images mapped from the volume
  (checked by hand: sections mapped in both processes), with
  `TestMilestone`'s output and files, and the volume dismounted and
  verified after.
- **2026-10-18:** At the author's request, the debugger's SHOW
  IMAGE/FULL shows how each image's sections are laid out in memory
  (govax's own; VMS's debugger has nothing like it): the image's file and
  whether it was mapped or copied, then per ISD its kind, addresses,
  pages, protection now, and backing store (the image file from its VBN
  and its process section's index, then the page file for a writable
  one; the page file for demand-zero pages; "none" for a copied image),
  its pages by PTE form (valid, in transition on the free or modified
  list, not yet read, in the page file, demand zero), and its pages in
  memory by their frames' backing store. `corevms/pageinfo.go`
  (`PageInfoAt`, `SectionAt`: a read-only look at a page's PTE, frame,
  and section), `machine/imagelayout.go` (`ImageSections`; `ICB.Mapped`,
  set by `imageMap`), the debugger's `showSections`. Tests:
  `TestShowImageFull_mapped` (at the mapped image's two checkpoints) and
  `TestShowImageFull_copied` (paging off).
- **2026-10-18:** Subtask 7, part 1: the whole suite with paging forced
  on, and the probe written (`testdata/pg66`).
  - **The forced runs** (an experiment, not committed; the patch is
    small: environment variables that turn paging on whatever the
    setting, set PQL_DWSDEFAULT and PQL_DWSQUOTA to n and PQL_DWSEXTENT
    to 2n, and take free frames after VMINIT until a given number is
    left, with the PFN database's and every working set's `Check` run
    after every fault). Every image the suite runs was mapped. At
    20-page sets with 180 free frames, image pages were read again from
    their files 318 times (frames reused) and pages read back from the
    page file 7085 times; at 30-page sets with 200 and 400 free frames,
    195 and 58 image rereads.
  - **Found and fixed:** a working set list grew past its limit while it
    had available entries: when the growth checks pass over the
    available entries (past WSQUOTA, the free list at GROWLIM) and every
    page is protected (as `$LKWSET`'s pages are while it locks them),
    `WorkingSet.Add` gave the page a new entry rather than an available
    one, breaking the list's rule that a list over its limit has every
    entry in use (`Check` found it in `TestPG63Oracle`'s `$LKWSET`, at
    30-page sets). The page now takes the first available entry after
    next, and the list grows only with none
    (`availableAfterNext`, shared with `AddAvailable`;
    `TestWorkingSetNowhereElse`). After it, no check failed in any
    configuration.
  - **What else failed, and why:** only tests of the forced settings
    themselves: the paging-off tests and those of the defaults; tests
    that expect a 30-page set's counts or a limit of 30 (`$ADJWSL`,
    SET WORKING_SET, the replacement and reclaimer tests,
    `TestProcessServices_assembledProgram`'s WSDEFAULT+10); the lock
    tests at the floor (SS$_LKWSETFUL with a 20-page set); counts and
    clusters of the writer tests and `TestShowImageFull_mapped`;
    `TestLib55Oracle`'s LIB$SHOW_TIMER fault count (two digits at a
    20-page set); `TestPageFileWriter`'s last check (the user stack's
    top page, written to the page file at a 20-page set, keeps its
    block, as it should: it outlives the image); `TestPG64WriterRound2`
    (VMS's timing with the VAX's memory); and at 200 free frames
    `TestPG64Oracle`'s detached process, whose page tables (a 16384-page
    P0) take the memory: Phase 67's subject. The LOGOUT tests' peak and
    `TestLkwsetFull` fail with paging forced on at any size, as Phase
    64 found. No output oracle differed but for counts.
  - **The probe** (`testdata/pg66`, README.md): one program linked
    twice (PG66.EXE, and PG66C.EXE with BIG in a `CLUSTER=` of its own,
    pfc 8), in three parts: A (activation's faults and charges at the
    first instruction against the procedure's counts before RUN,
    PROBEW of an untouched page, BIG read in order, ZERO, LIB$STAT_TIMER
    against `$GETJPI`, the image's file opened three ways while it runs,
    and a protection map of every page), W (BIG at a small working set),
    and D (the image's file, a copy, deleted while it runs in a
    subprocess). `TestPG66DryRun` runs it on govax with govax's own link
    of PG66.EXE (govax's LINK has no CLUSTER=), checking what doesn't
    depend on VMS's answers; the README lists govax's answers.
  - **Found by the dry run:** PROBER/PROBEW page in the page they probe
    (docs/DEVIATIONS.md): the protection map brought the whole image in.
    Left for the probe's case A3 to confirm on VMS before PROBE is
    changed. And COPY from one volume file to another reframed it as
    Stream_LF text unless /BINARY, so a copied image didn't run: it now
    copies the file as it is, as VMS's COPY keeps its record format
    (`internal/rms/copy.go`; `TestSessionCopy_volumeToVolumeKeepsFormat`).
    The suite passed unchanged with it.
  - **Next:** the VAX run (the README's steps), then govax changed to
    its answers and `TestPG66Oracle` in place of the dry run.
- **2026-10-19:** Subtask 7, part 2: the probe's VMS run
  (`testdata/pg66/vax`; README.md's "The VAX run"), OpenVMS V7.1 on
  simh's VAX 8600. `TestPG66Oracle` (in place of the dry run) runs VMS's
  own two images on govax as PG66.COM did and matches VMS's lines but for
  `expectedPG66Differences`.
  - **Confirmed:** the image's layout and every page's protection after
    activation, the fixup page UREW (question 1); the running image's
    file readable by others who share reads, RMS$_FLK otherwise (question
    2); demand-zero and image section faults, BIG's clusters by the
    section's cluster factor (the default, PFCDEFAULT, or LINK's
    CLUSTER=); image page reads aren't direct I/O; every image's charges
    back at its exit; the image file deleted while it runs, its name gone
    at once and its pages still read (part D, line for line).
  - **Changed:** PROBER/PROBEW no longer page in what they probe
    (`vm.Memory.ProbeAccess`: the PTE's protection field, the valid bit
    ignored; `TestProbeAccessDoesNotPageIn`; docs/DEVIATIONS.md): VMS's
    PROBEW of an untouched page faulted nothing. RMS$_FLK's STV is
    SS$_ACCONFLICT (`stvFor`).
  - **Differences that stay:** activation's faults and charges (VMS
    charged 62 pages at the first instruction, govax 24: LIBRTL's
    writable pages and P1's image pieces); LIBRTL's and RMS's own faults
    and I/O; a successful `$OPEN`'s STV; and the effects of VMS's working
    set list holding DCL's pages and its permanent part, which govax's
    doesn't have (Phase 63): BIG's pages replacing each other as they came
    in, and part W's first pass clustered by replacement on VMS (14
    faults) where govax's clusters take only free entries, the book's
    15.5.1.1 rule (75). Phase 65's K2 found VMS not clustering at a
    small working set at all, so the rule stays the book's.
  - **Noted, not changed:** govax's LINK puts the two LIBRTL fixup
    cells in the other order (a fifth data point for `orderCells`, which
    no rule fits); govax's DELETE, an image on VMS, leaves CLI$_NORMAL
    in `$STATUS` where VMS's leaves %X10000001 (docs/DEVIATIONS.md, with
    the user-mode names an image's exit deletes). The first RUN's
    `charged=-1` in govax's dry run was the test console's own spin loop
    in P0, which the first activation deletes.
- **2026-10-19:** JPI$_IMAGNAME, which the probe had to do without (its
  image's file came to it as the logical name PG66_IMAGE). The image
  activator records where it found the main image (`ICB.Spec`, from
  `readImage`/`openImage`, either mode) and `ActivateImage` gives it to
  the process (`corevms.Environment.SetImageName`, `Process.ImageName`);
  `ImageRundown` clears it, so at DCL's prompt it's empty, as on VMS. A
  volume file's name is its full specification with the *physical*
  device, the node's name in front, where RMS's resultant name would
  show a concealed device's logical name (`rms.FileLocation.Physical`):
  the author's VMS 7.3 on simh gave
  `SIMVAX$DUA0:[SYS0.SYSCOMMON.][SYSEXE]REMACP.EXE;1` for a process
  running REMACP. A host image's name is its absolute host path. $GETJPI,
  LIB$GETJPI, and F$GETJPI all have it (`TestImageName`,
  `TestRun_imageOnVolume`, `TestLexicalProcess`). Unconfirmed: an image
  found through a rooted logical name; govax writes the directory the
  file was found in as RMS's match gives it, not checked against VMS's
  `[SYS0.SYSCOMMON.][SYSEXE]` form.
- **2026-10-19:** Subtask 8, the close-out. CLAUDE.md describes the
  phase (the paging program's paragraph; corevms's page sources and
  image sections, and the machine's `imageMap` and `OpenImage`); PLAN.md's
  row is done; HELP's VMINIT/PAGING says that RUN maps an image; Phase
  67's doc lists what this phase leaves it. The open question is
  settled above.
  **Carried forward:** activation's faults and charges, which differ
  from VMS's because VMS's LIBRTL and the rest are installed images
  with writable pages of their own, and its activator works in P1
  (govax's RTL is shims; `expectedPG66Differences`); clustering by
  replacement at a small working set, which VMS 7.1 did in part W
  where govax keeps the book's rule of free entries only (Phase 65's
  K2 saw VMS not cluster at all there), and the working set list's
  permanent part (Phase 63); host shareable images installed as shared
  global sections; the order of LINK's fixup cells (`orderCells`);
  DELETE's `$STATUS` (docs/DEVIATIONS.md); JPI$_IMAGNAME for an image
  found through a rooted logical name; the user stack ISD and the
  page tables at small memories (Phase 67). With paging off the header
  is still written to P0 as a scratch copy, as before this phase.
