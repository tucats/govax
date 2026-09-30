# Phase 30: A govax LINK

## Goal

Add a `LINK` command that builds a VMS executable image (`.EXE`) from object
modules, so that a program assembled by govax's `MACRO` command can `RUN`
inside govax without a real VAX. It must link govax's own objects and real
VAX objects alike.

Split out of Phase 27's "later sub-phases" (docs/PHASE-27.md, subtask 12).

**Status: in progress (started 2026-09-30).** The user chose to do this phase
before Phases 28 and 29, so that simple programs get a full MACRO, LINK, and RUN
cycle first (see [Phase order](#phase-order)).

## What Phase 27 leaves in place

- **Reading objects.** `internal/obj` reads, decodes, and checks every
  record of the VAX object language. That includes all GSD subrecord types
  and all TIR commands, each with its stack effect. It reads both govax's
  objects and real ones: the twelve real objects in `testdata/mar/vax/`
  are its regression corpus.
- **Finding files.** `rms.Session.Locate`/`LocateRelated` and
  `ReadRecordFile`/`CreateRecordFile` find and read input objects, and
  write outputs, on the host or a mounted volume, with the same rules as
  `MACRO`.
- **Running images.** govax's `RUN` (Phase 13) activates real VMS images:
  it maps image sections, resolves sharable-image references, and applies
  fixups. So the image LINK writes has a consumer in govax already.
- **References to compare with.** Real `LINK/MAP` maps for `entry`,
  `hello`, and `psects`, from both real MACRO's objects and govax's, are
  in `testdata/mar/vax/` and `testdata/mar/vax/govax/`. The images real
  LINK built from them are on the (gitignored) exchange volume
  `testdata/disks/mar-exchange2.dsk`.

## Decisions

### Phase order

This phase comes before Phases 28 (macro facility) and 29 (listings and
traceback records), decided 2026-09-30. Neither is a prerequisite:

- **Phase 28:** LINK consumes object modules, and macros only change what
  source can be written. Programs that call `SYS$...` and `LIB$...` entry
  points directly, as the Phase 27 fixtures do, are enough to exercise
  LINK.
- **Phase 29:** real MACRO's objects carry traceback (TBT) records, and
  real LINK builds a debug symbol table (DST) from them by default. This
  phase's LINK reads TBT and DBG records and skips them. Its images carry
  no DST, which is what real LINK writes for govax's objects, which have
  no TBT records (`GV_PSECTS.EXE`). The traceback transfer address is
  still written: it's `SYS$IMGSTA` in the P1 vector, which govax has at
  the same address as VMS.
- **The librarian reader** this phase needs for `.OLB` object libraries
  reads the same file format as `.MLB` macro libraries, so Phase 28 gets
  it for free.

### Where external symbols come from

The user hasn't decided how far govax should rely on its own shims rather
than real VMS run-time library images. Anyone else using govax needs one
of them: real VAX files to import, or very complete shims. So LINK has to
work with either, and the images it writes must not depend on which one
it used.

That holds because of how shareable-image references work. A linked
image refers to a routine in a shareable image by the image's name and
the routine's offset in it (its transfer vector entry, such as
`LIBRTL` + `^X478` for `LIB$PUT_OUTPUT`). govax's `RUN` already resolves
such a reference two ways (`resolveFixupTarget` in
`internal/console/image.go`):

- against the real image, if a file of that name is loaded;
- otherwise against the shim registered as `SHIM$LIBRTL_00000478`.

The shims are keyed by the real offsets, so the same image runs either
way.

So LINK resolves symbols through a list of **symbol sources**, an
interface with interchangeable implementations. Each maps a global
symbol name to a definition: an absolute value (system services, in the
P1 vector), or a shareable image name plus offset, or an object module
to add to the link.

1. **govax's own tables**, which need no VAX files: the shim table from
   `kernel.asm`'s `.SHIM` entries (name, image, offset) and the P1
   vector table (`vmsdef.P1VectorTable`). They cover exactly the
   routines govax can run.
2. **A shareable image's global symbol table (GST).** This is ordinary
   object-language records, which `internal/obj` already reads. It can
   come from:
   - a real image (`LIBRTL.EXE` carries its GST, at `IHS$L_GSTVBN`);
   - the matching module in `IMAGELIB.OLB`;
   - a GST file govax ships in `internal/bootdata`.

   A GST holds only names and offsets, no code, so extracting one from
   a real image and shipping it would let a link go ahead with no VAX
   files present, and the result would run against the shims. Whether
   to ship such files is the user's call; the design allows it.
3. **Object libraries** (`.OLB`, such as `STARLET.OLB`), through the
   librarian reader: a module that defines a still-undefined symbol is
   pulled into the link, as real LINK does.

Sources are searched in order, as real LINK searches its libraries. The
default list is still to be decided. Real VMS searches `IMAGELIB.OLB`,
then `STARLET.OLB`; govax's tables are a fallback, or the only source
when there are no VAX files.

## What real LINK writes (from the fixture images)

Real LINK V11-39 built `ENTRY`, `HELLO`, and `PSECTS` from both real
MACRO's objects and govax's (Phase 27 subtask 11). Those images, plus
`ihddef.sdl` (`vmssrc_archive/v73/pcsi/lis/`, now copied to
`reference/vms/ihddef.sdl`) and eVAX's `imgdef.h`, give the format. The
remaining sub-block layouts (`$IHADEF`, `$ISDDEF`, `$IAFDEF`, `$SHLDEF`,
`$ICPDEF`) aren't in the source archive as text, so the fields below are
read from the images. `ANALYZE/IMAGE` output from the VAX would confirm
them.

- **Image header** (block 1, `IHD$B_HDRBLKCNT` = 1):
  - **`IHD`** (0x30 bytes). `IHD$W_SIZE` 0xB0 is where the ISDs start,
    after the sub-blocks. The sub-block offsets are `ACTIVOFF` 0x30,
    `SYMDBGOFF` 0x44, and `IMGIDOFF` 0x60. The IDs are `"02"`/`"05"`,
    and the type is 1 (executable). `PRIVREQS` is all ones.
    `LNKFLAGS` is 0x010000A8 in every image: `PICIMG`, `DBGDMT`, and
    `IHSLONG` (bits 3, 5, and 7, as `ANALYZE/IMAGE` names them), and
    `MATCHCTL` 1 in the top byte. `IDENT` is bytes 2 to 5 of the link
    time, and `IAFVA` is the fixup section's address.
  - **`IHA`** (0x14 bytes): four transfer addresses and `INISHR`. With
    traceback these are `SYS$IMGSTA` (0x7FFEDF68), then the user
    transfer address.
  - **`IHS`** (0x1C bytes): where the DST and GST are. All zero without
    a DST.
  - **`IHI`** (0x50 bytes): the image name (counted, in 40 bytes), the
    image ID (counted, 16 bytes), the link time, and the linker ID
    (counted, 16 bytes, `V11-39`).
  - **ISDs.** A private section's ISD is 16 bytes: size, page count,
    VPN (low 21 bits of a longword, PFC in its top byte), flags, and
    VBN. A demand-zero ISD is 12 bytes, with no VBN. A zero word ends
    the list, and the rest of the block is 0xFF.
- **Image sections** follow the manual (§6.3.4):
  - psects are grouped by their `WRT`/`EXE`/`VEC` attributes, in
    Table 6-1's order, alphabetically within a section;
  - each section is page-aligned, from 0x200;
  - ISD flags are `LASTCLU` for the one user cluster, plus `WRT`+`CRF`
    for a writable section;
  - a writable section with nothing stored in it becomes demand-zero
    (`DZRO`+`WRT`), as `PSECTS`'s 4-byte `. BLANK .` did.
- **The fixup section** always follows the last user section (§6.3.6.2).
  Its ISD flags are `FIXUPVEC`+`WRT`+`CRF`. Its page holds the `IAF`:
  - two links;
  - 0x40;
  - offsets of the G^ fixup list, the .ADDRESS fixup list, the
    change-protection list (`ICP`), and the shareable image list (`SHL`);
  - the SHL count.

  Then come the G^ list (`{count, SHL index, count cells}` groups, ended
  by a zero count), the ICP list, and the SHL. The SHL has 0x40-byte
  entries, the first being the image itself, and names at +0x18.

  The one ICP entry is the fixup section itself: 1 page, whose
  protection becomes `PRT$C_UREW` (0x0D, user read and executive
  write) once the fixups are done. Its address is relative to the
  image's base, 0x200 (`ANALYZE/IMAGE`: "relative to %X'00000200'").
  The IAF's shareable image count includes the image itself, and its
  "extra image count" is 0.
- **A shareable image reference** (from `ANALYZE/IMAGE` of `HELLO.EXE`)
  adds a 31-byte global section ISD after the user stack's. For
  `LIBRTL` it has:
  - the flags `GBL` and type `ISD$K_SHRPIC`, VPN 0 and VBN 0;
  - the shareable image's page count (264);
  - its global section ident (major 1, minor 0x0E) and match control
    (`ISD$K_MATLEQ`);
  - the section name `LIBRTL_001`.

  So a symbol source must supply, for each shareable image, its page
  count, ident, and match control as well as its symbols' offsets.
  govax's `RUN` skips global section ISDs, but real VMS needs them.
- **The user stack** is the last ISD: 20 pages of demand-zero, type 253,
  VPN 0x3FFFEC.
- **`GV_PSECTS.EXE`** (govax's object) is byte for byte `PSECTS.EXE`
  (real MACRO's) without the DST block. So this phase's first target is
  to write `GV_PSECTS.EXE` exactly, apart from the link time and names.

## Scope

- The linker's two passes over the objects:
  - collect psects (concatenated or overlaid by attribute, and aligned),
    global symbols, and entry points;
  - run each TIR program on the linker's stack machine to lay out and
    relocate the image.
- Resolving references against sharable images (`SYS$PUBLIC_VECTORS`,
  `LIBRTL`, and so on). govax's shim table (Phase 13/20) knows those
  entry points.
- The image header and image sections that govax's `RUN` (and ideally
  real VMS) accept.
- `/MAP` for a link map; `/EXECUTABLE[=file]`.

## References

- *VMS 5.0 Linker Utility Manual* (`AA-LA62A-TE`): the linking process,
  image layout, the object language (chapter 7), and maps.
- `vmssrc_archive/v73/linker/lis/`: the VAX linker's source listings.
  - `lnkobjps1_v.lis` and `lnkobjps2.lis`: its two object passes.
  - `lnkimgout.lis`: image output.
  - `lnkmaprtn.lis`: maps.
- `reference/eVAX`'s `console_run.c` and Phase 13's docs: the image format
  from the loader's side.

## Open questions

- **Image fidelity.** Must a govax-linked image run on real VMS, or only
  in govax? The Phase 27 answer for objects was "readable both ways";
  the equivalent here would be "govax runs real images, and real VMS
  runs govax's".
- **Sharable images.** Where do sharable-image symbol tables come from:
  real `.EXE` files copied from the VAX, or govax's shim table?
- **Scope of LINK's qualifiers** and options files.

## Subtasks

1. **Done.** **One self-contained object.** A new `internal/link` package:
   - pass 1: psects, contributions, and global symbols;
   - allocation into image sections;
   - pass 2: running each TIR program on the linker's stack machine;
   - demand-zero sections, the fixup section, and the image header.

   A console `LINK` command (`/EXECUTABLE`, `/[NO]TRACEBACK`), and a
   `govax link` subcommand. Pass criterion: linking `psects.obj` writes
   `GV_PSECTS.EXE`'s bytes (the link time, image name, and linker ID
   aside), and govax `RUN`s the image with status 1.
2. **Done.** **External references through govax's own tables:** the symbol-source
   interface, the shim and P1 vector sources, G^ fixups into the fixup
   section, and the SHL and global-section ISDs, so `hello` links and
   runs in govax. Compare with `GV_HELLO.EXE`.
3. **The librarian reader,** and GST and object-library sources
   (`IMAGELIB.OLB`, `STARLET.OLB`, a real image's GST), tested against
   files from the VAX when present.
4. **More than one object,** `CON`/`OVR` psects across modules, `/MAP`,
   options files, and whatever the fixtures show next.
5. **Real VMS checks** (needs the user): `ANALYZE/IMAGE` of the real and
   govax images, `/NOTRACEBACK` links, and running govax-linked images
   on the VAX.

## Progress Log

### 2026-09-30 — Planned

- Created from Phase 27's "later sub-phases" when that phase closed.

### 2026-09-30 — Started; format read from real images

- The user chose to do this phase before Phases 28 and 29, and asked
  that LINK work with govax's shims or with real run-time library files
  alike. Both are recorded under [Decisions](#decisions).
- Decoded the image format from `PSECTS.EXE`, `GV_PSECTS.EXE`, and
  `HELLO.EXE` (copied out of `mar-exchange2.dsk`), `ihddef.sdl`, and the
  Linker manual's chapter 6. See
  [What real LINK writes](#what-real-link-writes-from-the-fixture-images).

### 2026-09-30 — Subtask 1: MACRO, LINK, and RUN for self-contained programs

- **`internal/link`** (new) builds an executable image from
  `internal/obj` modules:
  - `link.go`: pass 1, collecting psects, contributions, global symbols,
    and the transfer address;
  - `pass2.go`: allocation, the TIR stack machine, and demand-zero
    compression;
  - `image.go`: the header, ISDs, and fixup section.
- **How it follows the manual.**
  - Psects are grouped by `WRT`/`EXE`/`VEC` in Table 6-1's order,
    alphabetically within a section, with each section page-aligned
    from 0x200.
  - A concatenated psect's contributions follow one another, each at
    its own alignment; an overlaid psect's all start at its base.
  - A writable section with nothing (nonzero) stored in it is
    demand-zero, and so is any run of at least 5 such pages in one
    (`DZRO_MIN`).
- **Symbols and the transfer address.**
  - A weak definition yields to a strong one, and a second strong
    definition is an error. So is a second strong transfer address.
  - The image ID is the ident of the module with the transfer address,
    or else the first module's.
  - Real MACRO writes its global symbols before its psect definitions,
    so a symbol's psect is resolved once its module has been read.
  - Traceback and debugger records are skipped.
  - Every symbol must be defined in the objects for now: the symbol
    sources are subtask 2.
- **Matches real LINK byte for byte.** `TestLinkMatchesRealLINK` links
  `psects` and `entry` from govax's objects and from real MACRO's. Each
  image equals `GV_PSECTS.EXE`/`GV_ENTRY.EXE` exactly, given real LINK's
  image name, link time, and linker ID. That covers the header, the
  ISDs, the data, the demand-zero `. BLANK .`, and the fixup section.
  The real images are now fixtures: `testdata/mar/vax/*.exe` and
  `govax/gv_*.exe`.
- **The command.** `LINK object[,...][/HOST] [/EXECUTABLE[=image] |
  /NOEXECUTABLE] [/[NO]TRACEBACK]` (`internal/console/link.go`), and
  `govax link <object>... [--executable f | --no-executable]
  [--no-traceback]`.
  - Objects and the image follow MACRO's host and ODS-2 file rules. An
    object's default type is `.OBJ`. The image's default name is the
    first object's, with `EXE` in the case of its type.
  - The header's image name is the image file's name, and its linker ID
    is `govax V<build>`.
  - MACRO's naming helpers moved to `internal/console/filenames.go` and
    take the file type as a parameter.
  - The new `CLI_LINKING` status reports link failures.
  - `vax.help` has a LINK topic.
- **Images on volumes and the host.** `rms` has a third record kind,
  `ImageBlocks`: fixed 512-byte records, with real LINK's attributes on
  a volume (`RFM=FIX`, 512-byte records), and the raw blocks on the
  host.
- **Host file permissions.** A host file `CreateRecordFile` writes had
  mode 0600, from `os.CreateTemp`. It's now created beside the target
  with `O_EXCL` and 0644 (less the umask), or the existing file's mode,
  then renamed into place. This affects MACRO's objects too.
- **The cycle works.** `govax macro psects.mar`, `govax link psects`,
  and `RUN psects.exe` run the program, and R0 is 1.
  `TestLink_runsRelocatedCode` links two modules into a program that
  calls across them and loads its status through a relocated data
  reference. It returns 42 under RUN. `TestRun_macroLinkRunOneShot`
  runs all three commands as one-shot commands.
- Tests:
  - `internal/link`: the real-image comparison, psect layout, demand-zero
    splitting, symbol and transfer rules, and concatenated and overlaid
    contributions.
  - `internal/console/link_test.go`: default names and types, several
    objects, header names, `/NOEXECUTABLE`, `/NOTRACEBACK`, errors,
    volume images, and LINK through DCL.
  - `rms`: `ImageBlocks`, and the new file mode.
  - `cmd/govax`: `linkCommand`, and the one-shot cycle.
  - Two `dcl` grammar tests expect the LINK verb.

  `go test ./...` passes.

### 2026-09-30 — ANALYZE/IMAGE of HELLO.EXE

- The user ran `ANALYZE/IMAGE` on `HELLO.EXE` (from
  `mar-exchange2.dsk`). It names `LNKFLAGS`' bits, and shows that the
  change-protection entry covers the fixup section, relative to the
  image base, with protection UREW. It also shows the global section
  ISD a shareable image reference needs. See
  [What real LINK writes](#what-real-link-writes-from-the-fixture-images).
- `internal/link/image.go` now names the flags, and computes the entry's
  address from the image base rather than as "the page below the fixup
  section". The bytes are the same, and the real-image comparison still
  passes.

### 2026-09-30 — LINK/NOTRACEBACK confirmed

- The user linked `PSECTS` with `LINK/NOTRACEBACK` on the VAX
  (`PSECTS.EXE;2` on `mar-exchange2.dsk`, now
  `testdata/mar/vax/psects-notraceback.exe`). Its transfer addresses
  are the user transfer address and then zeros, with no `SYS$IMGSTA`.
  It has no debug symbol table, and the same link flags.
- govax's `/NOTRACEBACK` already wrote exactly that:
  `TestLinkNoTracebackMatchesRealLINK` links the fixture from govax's
  and real MACRO's objects, and both equal the real image byte for
  byte.

### 2026-09-30 — Subtask 2: hello links and runs

- **Symbol sources** (`internal/link/source.go`). `SymbolSource` has two
  questions:
  - `Lookup(name)`: an absolute value, or a shareable image plus the
    offset in it;
  - `Image(name)`: the image's page count, global section ident, and
    match control, which the image's global section ISD records.

  `TableSource` answers both from maps. `Options.Sources` are searched
  in order for each symbol the modules refer to but don't define, and a
  symbol none of them defines is still an error.
- **govax's own source** (`internal/console/linksource.go`), built from:
  - `vmsdef.P1VectorTable`: system services, absolute;
  - the console's `shimTable`: each routine's image and real offset, so
    `LIB$PUT_OUTPUT` is `LIBRTL` + `^X478`;
  - `sharedImages`: only `LIBRTL`'s facts are known (264 pages, ident
    1/0x0E, `MATLEQ`, from the user's `ANALYZE/IMAGE`). The others are
    recorded by name, with a match control that accepts any ident.

  The console's LINK uses it. A GST or object library source goes ahead
  of it in subtask 3.
- **Shareable image references.**
  - A general mode operand whose value is in a shareable image becomes
    longword relative deferred mode (`FF`), through a cell in the fixup
    section that holds the target's offset. The image activator adds
    the image's base to it (the Linker manual, §6.3.6.2).
  - Each target gets one cell, however many operands reach it. The
    fixup section lists the shareable images in the order they were
    first referred to: one G^ fixup list per image, the change-protection
    entry, and a shareable image list naming each after the image
    itself.
  - Each image gets a 31-byte-and-up global section ISD after the user
    stack's: `GBL`, the match control in bits 4-6, type `SHRPIC`, the
    page count, the ident, and the section name `<image>_001`.
  - The displacements are stored after pass 2, once the fixup section is
    laid out (`patchGRefs`). A fixup section bigger than a page is
    allowed for.
  - Any other reference to a shareable image address (a store, a
    displacement, arithmetic beyond adding or subtracting a constant) is
    an error that asks for general mode. `.ADDRESS` fixups aren't
    written yet.
- **Matches real LINK byte for byte.** `TestLinkSharedImageMatchesRealLINK`
  links `hello` from govax's and real MACRO's objects, and both images
  equal `GV_HELLO.EXE`. That covers the `@L^` operand, the fixup
  section's G^ list and shareable image list, and `LIBRTL`'s global
  section ISD.
- **It runs.** `govax macro hello.mar`, `govax link hello`, and
  `govax run hello.exe` print "Hello, world!": RUN finds no
  `LIBRTL.EXE`, so it connects the cell to the shim for
  `SHIM$LIBRTL_00000478`, which is govax's own `LIB$PUT_OUTPUT`.
- Tests:
  - `internal/link`: the real-image comparison; several targets across
    two images (one cell per target, list order, counts, displacements,
    and global section ISDs); and the general mode error.
  - `internal/console`: hello's output under RUN; a G^ system service
    call (absolute mode at `SYS$EXIT`'s P1 address); and the undefined
    symbol error, now with a name nothing defines.

  The LINK help topic describes the symbol lookup, and `go test ./...`
  passes.

### 2026-09-30 — Where subtask 3 starts

- The user copied `IMAGELIB.OLB`, `STARLET.OLB`, and `LIBRTL.EXE` from
  VMS 7.3 onto `mar-exchange2.dsk` (`[LIB]`). They're now in
  `testdata/vmslib/` (`imagelib.olb`, `starlet.olb`, `librtl.exe`, raw
  blocks, gitignored). Tests that use them must skip when they're
  absent.
- They were deleted from the container, and all its free blocks
  zeroed, so no licensed copy travels with it.
- **What subtask 3 needs:**
  1. A reader for the librarian's file format (`.OLB`, and later
     `.MLB` for Phase 28). The references are
     `vmssrc_archive/v73/lbr/lis/` (`lbr.sdl`, `index.lis`, `data.lis`,
     `openclose.lis`) and `librar/lis/`. The file starts with a header
     naming "Librarian T09-20". It belongs in a new leaf package
     (probably `internal/lbr`), with no dependency on `ods2`.
  2. A GST source: a shareable image's global symbol table is
     object-language records (`internal/obj` reads them). It comes
     from `IMAGELIB.OLB`'s module for the image, or from the image
     itself (`IHS$L_GSTVBN`/`GSTRECS` in `librtl.exe`'s header). It
     answers `Lookup` with image plus offset, and `Image` with pages,
     ident, and match control. Check its answers against
     `sharedImages` in `internal/console/linksource.go` (`LIBRTL`: 264
     pages, ident 1/0x0E, `MATLEQ`) and against the shim table's
     offsets.
  3. An object library source: a `STARLET.OLB` module that defines a
     still-undefined symbol is added to the link (pass 1 on it, then
     look again, as real LINK does).
  4. Decide the default source order (real LINK searches `IMAGELIB`,
     then `STARLET`; govax's tables are the fallback), and how the
     console finds the library files (a setting, or a logical name
     like `SYS$LIBRARY`).
