# Phase 40 — ANALYZE/IMAGE

**Status:** done (2026-10-05). All 29 fixture analyses match VMS 7.3's
byte for byte, page layout included, but for the time. Debug table
contents are future expansion.

## Goal

A console `ANALYZE/IMAGE` command whose output matches VMS 7.3's
`ANALYZE/IMAGE` (ANALYZ V07-04) line for line: the image header (fixed
header, activation, symbol table and debug pointers, identification,
patch information, each image section descriptor with its flag bits), the
image activator fixup section (its fixed part, the shareable image list,
G^ and .ADDRESS reference fixups, protection change fixups), the errors
ANALYZE finds, and VMS's page layout.

Phase 38 built `ANALYZE/OBJECT` and laid the verb, the `Pager`, and
`internal/anl` out so `/IMAGE` could slot in beside it (`TitleImage` is
already defined). This phase adds that analyzer, its grammar, and its
console and `govax analyze` entry points.

## Reference output

Twenty-nine `.ani` files in `testdata/` have the `.exe` they describe
beside them, all from VMS 7.3's `ANALYZE/IMAGE/OUTPUT=NAME.ANI NAME.EXE`:

| Directory | Pairs | What the images exercise |
|---|---|---|
| `testdata/link/vax/` | 11 | Phase 30's LINK fixtures: no transfer address (`exprs`, `globals`), three or four ISDs, LIBRTL references (`addr`: a G^ and a .ADDRESS fixup), `STACK=30` (`prog`) |
| `testdata/mar/list/vax/` | 10 | Phase 29's traceback images: a DST (VBN and block count), `LINK/DEBUG` (`trlnkdbg`, `trdbglnk`: LNKDEBUG set, a GST and a DMT), several G^ references to one image |
| `testdata/mar/round/vax/` | 8 | Phase 29's round trip (`rl*`, `cells*`): up to four G^ references on a line |

Twenty-one more (`testdata/link/vax/govax/gv_*.ani`,
`testdata/mar/round/vax/gv*.ani`) are VMS's analyses of images govax's
LINK wrote at the time, without the images beside them: extra samples of
the format, not oracle pairs. Since govax's LINK now writes the fixtures'
images byte for byte, an image govax links from the fixtures' objects
analyzes to the same report but for the link time.

All 29 are executable images (`IHD$K_EXE`) referring at most to LIBRTL.
None is a shareable or system image, has patches, more than one header
block, an image with ISDs of other section types, or an error.

### What the output looks like

```
<FF>
Analyze Image                                30-SEP-2026 06:01:55.94   Page 1
DUA1:[000000]PROG.EXE;1
ANALYZ V07-04

This is an OpenVMS VAX image file

IMAGE HEADER

	Fixed Header Information

		image format major id: 02, minor id: 05
		header block count: 1
		image type: executable (IHD$K_EXE)
		I/O channel count: default
		I/O page count: default
		linker flags:
			(0)  IHD$V_LNKDEBUG   0
			...
			(8)  IHD$V_UPCALLS    0

	Image Activation Information

		first transfer address:  %X'7FFEDF68'
		second transfer address: %X'00000400'
		third transfer address:  %X'00000000'

	Global Symbol Table & Debug Symbol Table Information

		debug symbol table VBN:  5, block count: 1
		global symbol table VBN: 0, record count: 0
		debug module/psect table VBN: 0, byte count: 0

	Image Identification Information

		image name: "PROG"
		image file identification: "V9"
		link date/time: 30-SEP-2026 06:01:55.90
		linker identification: "V11-39"

	Patch Information

		There are no patches at this time.

	Image Section Descriptors (ISD)

		1)  image section descriptor (16 bytes)
			page count: 1
			base virtual address: %X'00000200' (P0 space)
			page fault cluster size: default
			ISD flags:
				(0)  ISD$V_GBL        0
				...
				(18) ISD$V_PROTECT    0
			section type: ISD$K_NORMAL
			base VBN: 2
		...
		5)  image section descriptor (31 bytes)
			...
			section type: ISD$K_SHRPIC
			base VBN: 0
			global section major id: %X'01', minor id: %X'00000E'
			match control: ISD$K_MATLEQ
			global section name: "LIBRTL_001"
<FF> ... (new page)
IMAGE ACTIVATOR FIXUP SECTION


	Fixed Information

		Flags:
			(0)  IAF$V_SHR        0
		shareable image count: 2
		extra image count: 0

	Shareable Image List

		0)  this image
		1)  "LIBRTL"

	G^ Reference Fixups

		1 reference to image 1:
			  00000478

	.ADDRESS Reference Fixups (relative to %X'00000200')

		1 reference to image 1:
			  00000000

	Protection Change Fixups (relative to %X'00000200')

		address: %X'00000400', page count: 1
		protection: PRT$C_UREW



The analysis uncovered NO errors.


ANALYZE/IMAGE/OUTPUT=ADDR.ANI ADDR.EXE<blanks to column 80>
```

Observations from the survey (all from the fixtures and the images'
bytes; the clean-room rule leaves ANALYZE's output and real LINK's images
as the only sources):

- **Page header and trailer** are Phase 38's, with the title `Analyze
  Image`. Pages hold 55 report lines, as ANALYZE/OBJECT's do. The fixup
  section always starts a new page.
- **Indentation** is tabs: one for a section's parts, two for their
  fields and items, three for an item's fields and flags, four for an
  ISD's flag bits.
- **Flag bits** are listed by number and name with their value, padded as
  ANALYZE/OBJECT's are (`(10) ISD$V_FIXUPVEC   0`); only the named bits
  are shown (ISD bits 4–6 are the match control, 12–16 and 19–23 are
  unnamed, and the top byte is the section type). `IHD$L_LNKFLAGS`'s top
  byte (match control, 1 in every fixture) isn't shown.
- **Header fields** read from the bytes (offsets confirmed against the
  fixture images, and already used by `internal/link/image.go`): the
  fixed header's offsets to the other blocks at +0/+2/+4/+6, the patch
  block's at +8 (0: "There are no patches at this time."), ASCII major and
  minor ids, block count, image type, I/O counts at +0x1C/+0x1E (0 shows
  as `default`), link flags at +0x20. Activation: three transfer
  addresses. Symbol table and debug: DST VBN +0, GST VBN +4, DST blocks
  +8 (word), GST records +10 (word), DMT VBN +12, DMT bytes +16.
  Identification: counted name (40 bytes), counted id (16), link time
  (quadword, shown as VMS shows times, the day blank-padded), counted
  linker id (16).
- **ISDs**: size word, page count word, a longword holding the VPN (low 23
  bits) and the page fault cluster (top byte; 0 shows as `default`), the
  flags (section type in the top byte), and, unless demand zero, the base
  VBN. A global section's ISD adds its ident (`major id: %X'01', minor
  id: %X'00000E'`: the top byte, then 24 bits), match control
  (`ISD$K_MATLEQ`, from flag bits 4–6), and counted name. A base address
  is `(P0 space)` or `(P1 space)` (bit 30).
- **The fixup section** is found through `IHD$L_IAFVA` and the ISD with
  `ISD$V_FIXUPVEC` that maps it, read from that ISD's base VBN. Its fixed
  part gives the G^ list (+0x0C), .ADDRESS list (+0x10),
  change-protection list (+0x14), and shareable image list (+0x18)
  offsets, and the shareable image count (+0x1C). A list part is shown
  only when its offset isn't 0 (no G^ or .ADDRESS lists in `prog`).
- **Reference fixups**: per shareable image, `n reference(s) to image
  k:` then the values on lines of their own, each `  XXXXXXXX`, up to four
  per line in the fixtures. A G^ list shows each cell's contents (the
  target's offset in the shareable image); a .ADDRESS list shows each
  longword's address relative to the image base.
- **Protection change fixups**: one `address:`/`protection:` pair per
  entry, the protection named from `PRT$C_` (in `vmsdef.Symbols`).
- **The relative base** (`relative to %X'00000200'`) is the base of the
  image's first P0 section for every fixture (unconfirmed for a based
  image).
- The debug tables' **contents** (DST, DMT, and the GST of an executable
  linked /DEBUG) are not shown by ANALYZE/IMAGE: the fixtures give only
  their VBNs and sizes, even with LNKDEBUG set.

## Design

### Package `internal/anl`

Beside the object analyzer, sharing `Line`, `report`, and `Pager`:

- `image.go` — `ReadImage(data []byte) (*Image, error)`, a decoder of its
  own (not the console's loader, which works through emulated memory, nor
  `internal/link`, which only writes): the header blocks, ISDs (across
  header blocks; a size of `0xFFFF` continues in the next block), and the
  fixup section. Decoding problems are collected for the report, not
  returned, unless the file can't be an image at all.
- `imagehdr.go` — `AnalyzeImage(img, ImageOptions) []Line`: the image
  header report. Table-driven like the object analyzer (feedback: tables,
  not switches): the IHD and ISD flag bits, image types, section types,
  match controls, each a table of names.
- `imagefix.go` — the fixup section report.
- Image-header names (`IHD$V_`, `ISD$V_`, `ISD$K_`, `IAF$V_`, and the
  image type names) aren't in `vmsdef.Symbols`; they're taken from
  ANALYZE's output and kept in `anl`'s tables. `PRT$C_` names come from
  `vmsdef.Symbols`.

`ImageOptions` carries the selection (`/HEADER`, `/FIXUP_SECTION`).

### DCL grammar

```
    syntax analyze_image
        parameter files/type=$string/list/prompt="File"
        qualifier host/parameter=files
        qualifier output/type=$string/default=""
        qualifier header
        qualifier fixup_section

    verb analyze
        qualifier object/syntax=analyze_object
        qualifier image/syntax=analyze_image
```

### Files

As ANALYZE/OBJECT: `rms.Session.Locate`, default type `.EXE`; `/OUTPUT`
defaults to the console, and a named or empty `/OUTPUT` to NAME.ANI beside
the first input. Each input gets a complete report.

### `govax analyze`

`govax analyze --image [--header] [--fixup-section] FILE...`, beside
Phase 38's object options.

## Subtasks

1. **Plan** (this document), committed.
2. **Image decoding.** `ReadImage`: header blocks, ISDs, the fixup
   section. Unit tests on the fixture images (fields against the values
   the `.ani` files show) and on hand-built images.
3. **Header and fixup content, unpaginated.** `AnalyzeImage`, tested
   against the 29 fixtures with page headers removed: every content line
   must match.
4. **Pagination.** Each line kind's `Keep`, reconstructed from the
   fixtures' page breaks as Phase 38 did; the fixup section's new page.
   The fixture test then compares whole files, masking only the time.
5. **Errors.** ANALYZE's checks in place (a header block too short, a
   block offset or ISD outside the header, an ISD list without its end, a
   fixup section that no ISD maps or whose lists overrun it, a shareable
   image index out of range), in ANALYZE's style (unconfirmed text),
   with the count closing the report. Unit tests on damaged images.
6. **Image kinds the fixtures lack.** Shareable (`IHD$K_LIM`) and system
   images, other section types (`ISD$K_SHRFXD`, `PRVFXD`, `PRVPIC`, and
   the rest), the other match controls, a page fault cluster, I/O counts,
   based images, patch information, more than one header block, more
   transfer addresses, `IHD$V_INISHR`. A golden report collects these
   unconfirmed layouts in one place (as `kinds.txt` does for objects).
   Logged as unconfirmed.
7. **Console command.** The `analyze_image` syntax, the handler beside
   `AnalyzeObject` in `internal/console/analyze.go`, host and volume
   files, `/OUTPUT`, `/HEADER`, `/FIXUP_SECTION`. Tests through `Dispatch`,
   including an image govax LINKs from fixture objects, whose analysis
   must match VMS's but for the times.
8. **`govax analyze --image`**, help text (`vax.help`), `CLAUDE.md`,
   `PLAN.md`.

### Future expansion

- **Debug data.** govax's MACRO and LINK don't write debugger (DBG)
  records or a DMT yet (Phase 29's subtask 12), and ANALYZE/IMAGE shows
  only where the debug tables are. Showing their contents (the DST's
  records, as `obj.Dump` decodes them; the DMT; the GST of an image
  linked /DEBUG) is left for later, as an option of govax's own if real
  ANALYZE never shows them.
- **A shareable image's GST.** A shareable image's global symbol table is
  object-language GSD records; if ANALYZE/IMAGE shows them, it would be in
  ANALYZE/OBJECT's style, and `anl`'s object analyzer can describe them.
  No fixture shows it, and govax's LINK doesn't write shareable images yet.
- `/INTERACTIVE` isn't planned.

## Page layout

Reconstructed from the fixtures' page breaks (subtask 4), and matching all
29 byte for byte. The geometry is ANALYZE/OBJECT's (docs/PHASE-38.md): a
5-line page header and 55 lines of report. The fixup section starts a new
page. Each heading needs room for what follows it (`Line.Keep`):

| Line | Needs | Evidence |
|---|---|---|
| `ISD flags:` | 3 | written at row 53, and pushed off a page holding 53: exactly 3 |
| `N)  image section descriptor` | 3 | written as low as row 50, never pushed: 1 to 6 fit (unconfirmed) |
| a part's title (`\tImage Activation ...`) | 3 | written as low as row 46, never pushed: 1 to 10 fit (unconfirmed) |
| `linker flags:`, `Flags:` | 3 | always near a page's top: any fits (unconfirmed) |
| `n references to image k:` | 3 | always near a page's top: any fits (unconfirmed) |
| anything else | 1 | |

## Errors

No fixture shows an ANALYZE/IMAGE error, so every message is govax's, in
ANALYZE/OBJECT's style (unconfirmed): a `***  ` line, and the count in
the closing line. A check on a field follows that field's line;
something the decoder couldn't read (`Image.Problems`) is shown at the end
of the part it's about: the header blocks (after the patch information),
the ISDs (after the last one read), or the fixup section (at its end, or,
when it can't be found, where it would start).

| Message | When |
|---|---|
| `The header block count, n, is more than the file holds.` | IHD$B_HDRBLKCNT past the file's end |
| `The xxx block's offset, n, is outside the header.` | an activation, symbol table, identification, or patch block not wholly in the first header block |
| `Image format mm.nn is not the VAX image format, 02.05.` | the major and minor ids |
| `Image type n is undefined.` | not executable or shareable |
| `The image section descriptors' offset, n, is inside the fixed header.` / `... don't end within the header.` | the ISD list's start or end |
| `Image section descriptor k's size, n, is invalid.` | shorter than a demand-zero ISD, or past the header |
| `Section type n is undefined.` / `Match control n is undefined.` | an ISD's type or a global section's match control |
| `The section's blocks, v to w, are not in the file's n blocks of image sections.` | a private section's VBN in the header or its pages past the file's end |
| `No image section holds the fixup section at %X'a'.` / `The fixup section at %X'a' is outside the file.` | IHD$L_IAFVA |
| `The xxx list runs past the end of the fixup section.` | a G^, .ADDRESS, protection change, or shareable image list |
| `Image k is not a shareable image in the shareable image list.` | a reference list's image index |
| `Protection code n is undefined.` | a protection change's code |

One fault can lead to another: an ISD list that can't be read leaves the
fixup section without a section to hold it.

## Decisions and unconfirmed rules

- The output is matched to the fixtures byte for byte except for the page
  headers' times; anything they don't show is a reasonable choice in the
  same style, recorded here when made.
- **Unconfirmed layouts** (no fixture shows them; collected in
  `internal/anl/testdata/imagekinds.txt` for review): a shareable image's
  type (`shareable (IHD$K_LIM)`); nonzero I/O counts and page fault
  clusters shown as numbers; `(system space)` for an S0 address; the
  section types other than NORMAL, SHRPIC, and USRSTACK; the match
  controls other than MATLEQ; a patched image's block, shown as a hex dump
  (`patch block, n bytes:`) because nothing shows its fields; the
  shareable image initialization list's address (`IHA` +0x10) when
  `IHD$V_INISHR` is set; IAF flags at +0x24 and the extra image count at
  +0x20; four references per line; a blank line between protection
  changes.
- **The `/HEADER` and `/FIXUP_SECTION` selection**: with neither, both
  parts; `/HEADER` alone, the header; `/FIXUP_SECTION` (with or without
  `/HEADER`), both, since the fixup section's addresses only make sense
  beside the ISDs (unconfirmed).

## Progress log

- 2026-10-05: Survey and plan. 29 `.ani`/`.exe` pairs found; the header,
  ISD, and fixup-section formats read off them and the images' bytes; the
  page geometry is ANALYZE/OBJECT's (55 lines), with the fixup section on
  a new page. Debug data marked future expansion at the author's request.
- 2026-10-05: Subtask 2: `anl.ReadImage` (`internal/anl/image.go`): the
  fixed header and its blocks, ISDs (continued across header blocks at a
  size of `0xFFFF`), and the fixup section, found as the private section
  holding `IHD$L_IAFVA`. What can't be decoded becomes `Image.Problems`
  (shown as errors in subtask 5). `TestReadImageFixtures` decodes all 29
  fixture images cleanly; `TestReadImageAddr` checks every field of
  ADDR.EXE against `addr.ani`; `TestReadImageContinuedISDs` a two-block
  header. The IAF's flags (+0x24) and extra image count (+0x20) are
  placed by guess: both are 0 in every fixture (unconfirmed).
- 2026-10-05: Subtask 3: `anl.AnalyzeImage` (`imagehdr.go`, the header
  and ISDs; `imagefix.go`, the fixup section), table-driven: the IHD,
  ISD, and IAF flag bits, image types, section types, and match
  controls are tables; protections are named from `vmsdef.Symbols`'s
  `PRT$C_`. `TestImageContent` matches all 29 fixtures line for line once
  page headers and the closing command line are removed (a deliberately
  broken label fails the 14 with G^ lists, so the comparison bites).
  Choices the fixtures don't settle: up to four references per line, a
  blank line between protection changes, `(system space)` for an S0
  address, a shareable image's type text, and a patched image's one-line
  note (subtask 6 fills in the rest).
- 2026-10-05: Subtask 4: pagination ("Page layout"). `TestImagePages`
  lays each fixture's report out with `Pager` and compares whole files,
  masking only the page headers' times: all 29 match byte for byte. Of
  the page-break rules only `ISD flags:` needing 3 lines is pinned by the
  fixtures; the other headings' 3 is a choice inside their bounds.
- 2026-10-05: Subtask 5: errors ("Errors"). Decode problems carry the
  part of the image they're about (`Problem.Part`) and are shown at its
  end; the analyzer's own checks follow the field they're about.
  `TestImageErrors` damages ADDR.EXE eight ways and checks each message,
  its place, and the closing count.
- 2026-10-05: Subtask 6: kinds the fixtures lack. A patched image's
  patch block is a hex dump; with `IHD$V_INISHR` the activation block's
  initialization list address is shown. `TestEveryImageKind` builds a
  shareable image with two header blocks (ISDs continued into the
  second), a patch block, every section type and match control, an S0
  section, a page fault cluster, I/O counts, two shareable images'
  G^ lists (one wrapping), a .ADDRESS list, and two protection changes,
  and checks it analyzes without errors to
  `internal/anl/testdata/imagekinds.txt` (`go test -update` rewrites it).
- 2026-10-05: Subtask 7: the console command. `console.dcl`'s
  `analyze_image` syntax (`/HOST`, `/OUTPUT`, `/HEADER`,
  `/FIXUP_SECTION`) beside `/OBJECT`; `Console.AnalyzeImage`
  (`internal/console/analyze.go`), sharing a new `analyzeFiles` loop with
  `AnalyzeObject` (locating, paging, `/OUTPUT`, `CLI_ANALYZEERRORS`):
  default input type EXE, output NAME.ANI. A bare `ANALYZE` now asks for
  `/OBJECT or /IMAGE`. Tests (`analyzeimage_test.go`): VMS's ADDR.ANI
  reproduced from a host file (`/OUTPUT`'s default name, written beside a
  copy in a temp directory so no fixture can be overwritten) and from a
  volume file (header `DUA0:[000000]ADDR.EXE;1`); the qualifiers'
  selection, before or after `/IMAGE`; errors (a damaged image, a short
  file, a missing one); and EXTERN.EXE linked by govax's own LINK from
  the fixture objects, whose analysis matches VMS's analysis of real
  LINK's image but for the times and the linker's identification
  (`"govax Vn"`, by design).
- 2026-10-05: Subtask 8: `govax analyze --image [--header]
  [--fixup-section]` (`cmd/govax/analyze.go`; the object-only options
  with `--image`, or `--header`/`--fixup-section` without it, are
  refused), `TestAnalyzeCommand` and `TestRun_analyzeImageOneShot`;
  ANALYZE/IMAGE in `HELP ANALYZE` (`vax.help`); `CLAUDE.md` and
  `PLAN.md`. Checked by hand with `govax analyze --image` on CELLS2.EXE.
  Phase done.
