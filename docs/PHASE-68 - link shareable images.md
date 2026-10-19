# Phase 68 — LINK makes shareable images, and programs use them

**Status:** planned (2026-10-19); the plan reviewed and accepted by the
author the same day. Part A is the survey. Part B holds the decisions
and the author's answers to the plan's questions. Part C is the
subtasks. Subtask 1, the VMS probe (`testdata/link68`), is written and
waits for the author's run on the simh VAX.

## Goal

The full VMS cycle for shareable images, all inside govax:

1. MACRO assembles the routines of a library, with a transfer vector
   (`.TRANSFER`, `.MASK`, `JMP L^routine+2`).
2. `LINK/SHAREABLE` links them into a shareable image. An options file
   names the image's universal symbols (`UNIVERSAL=`), its match control
   (`GSMATCH=`), and its layout (`CLUSTER=`, `COLLECT=`,
   `PSECT_ATTR=`, ...).
3. MACRO assembles a program that calls those routines and reads their
   data through `G^`. LINK links it against the shareable image, named
   in an options file (`MYSHR/SHAREABLE`).
4. RUN finds the shareable image the way VMS's image activator does
   (a logical name, else `SYS$SHARE:name.EXE`), checks its ident against
   the one the program was linked with, maps it, does its fixups and the
   program's, and runs the program.

As in Phase 30, the target is real VMS's output. govax's
`LINK/SHAREABLE` should write the images and maps real LINK writes for
the probe's sources, byte for byte apart from times, names, and the
linker ID. Real VMS should run govax's shareable images and the programs
linked against them, and govax should run VMS's.

Work on MACRO is in scope for this phase (the author, 2026-10-19).
`.TRANSFER` is missing now (see "Gaps"), and the probe may find more.

## Part A — the survey (2026-10-19)

### Sources

- *VMS 5.0 Linker Utility Manual* (`AA-LA62A-TE`; OCR text in
  `~/Documents/Technical Doc/VMS/linker utility manual.txt`). The parts
  this phase uses:
  - 1.9 and chapter 4: shareable images, transfer vectors, the
    upward-compatibility rules, `UNIVERSAL=`, `GSMATCH=`, and the
    examples (4-3, 4-4);
  - 3.3: every option, with its defaults (`BASE=` is ^X200 for an
    executable and 0 for a shareable image);
  - 6.2.2 and 6.3: clusters, the two allocation passes, Table 6-1's
    image section order for shareable images (the significant attributes
    add `PIC` and `SHR`), demand-zero compression (`DZRO_MIN=`,
    `ISD_MAX=`), and when the fixup section goes into a shareable image;
  - 7.4: `TIR$C_OPR_REDEF`, the command `.TRANSFER` produces. It defines
    a universal symbol at the location counter, in the GST only.
- *VSI OpenVMS Linker Utility Manual* (HTML and text, same folder). It is
  usable as DIGITAL's manuals are. It describes Alpha and I64 too, so
  only its VAX parts apply; `SYMBOL_VECTOR=` isn't VAX's.
- *VAX MACRO and Instruction Set Reference Manual* (VSI's text, same
  folder): `.TRANSFER`, `.MASK`, and `.PSECT`'s `GBL`/`SHR`/`PIC`/`VEC`.
- *VAX/VMS Internals and Data Structures*, chapter 21 (21.1.1.4: how the
  activator finds and maps shareable images; the image control blocks).
- Real output: `ANALYZE/IMAGE` of the real LIBRTL.EXE (govax's own
  ANALYZE, which matches VMS's) and the Phase 30 fixtures. The clean
  room still bars the linker's source listings, so whatever the manuals
  don't settle comes from the probe (subtask 1).

### What earlier phases leave in place

**LINK (Phase 30; `internal/link`, `internal/dclcmd/link.go`).** It
links executables only:

- one user cluster;
- psects grouped by `WRT`/`EXE`/`VEC` (`pass2.go`'s `sectionKey`, the
  executable half of Table 6-1), from ^X200;
- the fixup section with G^ cells and `.ADDRESS` lists, a
  change-protection list, and a shareable image list
  (`fixup.go`, `image.go`);
- the global section ISD for each shareable image it uses
  (`isdTypeSharedPIC`, `LIBRTL_001`, the ident and match control from
  the image's header);
- a DST from TBT records, and with `/DEBUG` the debug module table and
  a GST (`debug.go`'s `globalSymbolTable`, an object module of absolute
  symbols, for the debugger).

It reads shareable images as **input**:

- `ReadShareableImage` (`libsource.go`) turns a real image's GST into a
  `TableSource`: relocatable symbols become image plus offset, with the
  page count, ident, and match control.
- An options file's `file/SHAREABLE` reads one (`dclcmd/link.go`'s
  `linkInput`, type EXE). IMAGELIB.OLB's images are found through
  `SYS$SHARE` or `vax.library` (`syslib.go`, `linksource.go`).

The options file (`options.go`) knows only `STACK=`, `IDENTIFICATION=`,
`NAME=`, and `SYMBOL=`, plus the input qualifiers `/LIBRARY`,
`/INCLUDE`, `/SELECTIVE_SEARCH`, and `/SHAREABLE` (not `=COPY`). The
console's LINK has `/EXECUTABLE`, `/[NO]TRACEBACK`, `/DEBUG`,
`/[NO]SYSLIB`, `/MAP`, `/BRIEF`, and the positional input qualifiers.

**MACRO (Phases 27 to 29; `internal/asm`).**

- `.PSECT` takes every attribute, `PIC`, `SHR`, `VEC`, and `GBL`
  included (`psect.go`).
- `.MASK symbol[,expr]` emits the linker's mask command
  (`macrodir.go`'s `pseudoMaskMACRO`).
- The object library writes every TIR command, and `internal/obj`
  decodes `OPR_REDEF` (`tir.go`).
- **There is no `.TRANSFER` directive**, and the linker's pass 2 has no
  case for `OPR_REDEF`.

**RUN (Phases 13, 20, 66; `internal/machine/image.go`, `imagemap.go`).**

- The main image's fixup section names its shareable images.
  `finishLoad` loads each one, by either loader, at P0's high-water
  mark, recursively: a shareable image's own fixup section loads *its*
  images.
- G^ cells and `.ADDRESS` longwords resolve to a loaded image's base
  plus the offset (`resolveFixupTarget`). Otherwise they resolve to the
  shim `SHIM$name_offset`, otherwise to `LIB$_UNRESOLVED`.
- A shareable image the loader can't find is silently skipped, and the
  shims stand in for it.
- **Finding** a shareable image is `findImage`: a host file `name.EXE`
  in the current directory, or after `SharePrefix` (SET
  SHARE_PREFIX). No logical names, no `SYS$SHARE`, no volume files. The
  main image, by contrast, is found by RMS's rules on volumes
  (`readImage`).
- The global section ISD's ident and match control are ignored: there
  is no GSMATCH check.
- `/INIT` runs each shareable image's first transfer address before the
  main image (`run.go`, `SHARE$name_INITIALIZE`).
- `TestMappedSharedImage` and `TestCopiedSharedImage`
  (`internal/console/imageshare_test.go`) use a shareable image
  **made by hand** from an executable. Its comment says a real one
  "needs a probe image or a LINK that makes them".

**ANALYZE/IMAGE (Phase 40)** already reports shareable images. Its
report of LIBRTL.EXE reads: type `IHD$K_LIM`; a first transfer address
of ^XCD4 (what is it? the probe should say); the GST at VBN 266, 20
records; ISD 1 is 264 pages of `ISD$K_SHRPIC` at VPN 0, VBN 2, with no
flags; ISD 2 is 15 pages of DZRO+WRT `ISD$K_PRVPIC` at ^X21000; there is
no fixup section and no stack ISD.

### Gaps

| Area | What's missing |
| ---- | -------------- |
| MACRO | `.TRANSFER` (an `OPR_REDEF` at the location counter); whatever else the probe's objects show |
| Options | `UNIVERSAL=`, `GSMATCH=`, `CLUSTER=`, `COLLECT=`, `PSECT_ATTR=`, `BASE=`, `DZRO_MIN=`, `ISD_MAX=`, `PROTECT=`, `IOSEGMENT=`, `UNSUPPORTED=`; `SYS$INPUT/OPTIONS` from a procedure's data lines (the manual's examples all use it) |
| LINK layout | Clusters (the cluster list, based clusters, the two allocation passes); demand-zero compression by `DZRO_MIN`/`ISD_MAX` |
| LINK output | `/SHAREABLE[=file]`: the `IHD$K_LIM` header, base 0, Table 6-1's shareable order, section types, no stack, GSMATCH in the header, the universal-symbol GST, `OPR_REDEF`, the fixup section's conditions and self-relative `.ADDRESS` entries; `/SYMBOL_TABLE` (.STB); the map's universal symbols and `/FULL` |
| LINK input | govax's own shareable images as input (should already work once they carry a GST; to be proved); a shareable image that uses another |
| Finding | Logical names and `SYS$SHARE:` on volumes, at RUN; one rule shared by RUN, LINK, and (Phase 61) `LIB$FIND_IMAGE_SYMBOL` |
| Activation | The GSMATCH check (`SS$_SHRIDMISMAT`, already in `vmsdef.Messages`); VMS's message for a missing image (`-CLI-E-IMGNAME`, ...) in place of silently skipping it; RUN of a shareable image itself |
| Libraries | Shareable image libraries (`LIBRARY/CREATE/SHARE`) and user default libraries (`LNK$LIBRARY`, `/USERLIBRARY`): see question 2 |

## Part B — decisions and questions

### Decisions (accepted by the author, 2026-10-19)

1. **The probe comes first.** The clean room bars the linker's
   listings, so the layouts the manual leaves open come from real LINK's
   output. That covers the GST's records, `OPR_REDEF`'s two values, the
   fixup section of a shareable image, section types, cluster order,
   maps, and messages. Subtask 1 writes the probe, and the author runs
   it on the simh VAX. Subtasks 2 and 3 don't depend on its answers, so
   they can go ahead while it waits.
2. **Fidelity is Phase 30's:** byte for byte on the probe's images and
   maps, the link time, image name, and linker ID aside. Real VMS runs
   govax's images (a second VAX round, subtask 13).
3. **One locator for shareable images**, in `internal/machine` (exported
   for LINK and later LIB$FIND_IMAGE_SYMBOL). It tries, in order:
   - **(a)** RMS's parse of the image name with the default
     `SYS$SHARE:.EXE`, as the activator does (the Linker manual, 4.4).
     A logical name of the image's name, in any table RMS searches,
     leads to the file. Otherwise `SYS$SHARE` does, on a mounted
     volume.
   - **(b)** Then the host: `SharePrefix`, then the `vax.library`
     directory, then the current directory (today's rule, widened).
   - **(c)** **Then govax's shims**, for an image govax emulates
     (LIBRTL, LIBRTL2, SYS$PUBLIC_VECTORS, ...).

   An emulated image goes to (c) before (a) and (b), unless a logical
   name of the image's own name is defined (question 1's answer).
4. **A shareable image that isn't found, and has no shims, stops
   activation** with VMS's messages (from the probe), instead of being
   skipped. The program would only fail later at its first call, as
   `LIB$_UNRESOLVED`.
5. **GSMATCH is enforced at activation.** The global section ISD's
   ident and match control are compared with the image's header (ALWAYS,
   EQUAL, LEQUAL, NEVER), and a mismatch is `SS$_SHRIDMISMAT`. The shims
   always match.
6. **Shareable images stay process-private**, mapped (paging on) or
   copied (off), as the book's 21.1.1.4 does for an image that isn't
   installed. INSTALL, known images, and shared or writable global
   sections across processes are a later phase.
7. **Out of scope:** `/SYSTEM` images, `/P0IMAGE`, `/HEADER`,
   `/CONTIGUOUS`, `/CROSS_REFERENCE`, `/SHAREABLE=COPY`,
   `SYMBOL_VECTOR=` (not VAX), and LIB$FIND_IMAGE_SYMBOL (Phase 61,
   which will use this phase's locator and GST reader). The debugger's
   `SET IMAGE` for a shareable image linked `/DEBUG` is also out, unless
   it turns out to be small; the loader already reads each image's DST.
8. **MACRO's changes are this phase's** (the author). They are checked
   byte for byte against real MACRO's objects for the probe's sources.

### Questions for the author (answered 2026-10-19)

The author took each proposal.


1. **Emulated images: shims or files first?** Should an image govax has
   shims for (LIBRTL, ...) resolve to the shims even when a real
   `LIBRTL.EXE` can be found through `SYS$SHARE` or `vax.library`? Real
   VMS code there can't run on govax's microkernel, but govax's LINK
   reads those files' GSTs.

   *Answer:* shims win unless a logical name of that image's name is
   defined. Defining one is an explicit opt-in, as `DEFINE LIBRTL
   DUA1:[X]LIBRTL.EXE` is.
2. **Shareable image libraries:** should `LIBRARY/CREATE/SHARE`,
   linking through a user shareable image library, and `LNK$LIBRARY`/
   `/USERLIBRARY` be in this phase (subtask 12), or a later one?
   *Answer:* in, as the last subtask before the close-out, so it can be
   cut without disturbing the rest. The probe covers it.
3. **The probe's VAX:** VMS 7.1 on simh, as for Phases 55 to 67, with
   its LINK (`V11-xx`) as the reference? The Phase 30 fixtures came from
   7.3's LINK V11-39. Does it matter whether both are run?
   *Answer:* VMS 7.1 on simh, its LINK the reference. The probe logs
   the version, so a difference from V11-39 shows.
4. **The doc's name** uses VMS's spelling, "shareable". Fine?
   *Answer:* yes.

### The probe's cases (subtask 1; `testdata/link68`)

One command procedure, `LINK68.COM`, run with `/OUTPUT=LINK68.LOG`, as
`testdata/link/link.com` was. It runs MACRO/LIST on every source, then
the links below with `/MAP/FULL` (and some `/BRIEF`), then ANALYZE/IMAGE
of every image and ANALYZE/OBJECT of the `.TRANSFER` object, then the
runs. The images and objects come back too: the GST is read from the
image file, since ANALYZE/IMAGE doesn't show it.

| Image | What it asks |
| ----- | ------------ |
| `XFR1.OBJ` | Real MACRO's object for `.TRANSFER` with `.MASK` (CALL entry) and with `JMP`+`.BLKB 2` (JSB entry); `.TRANSFER` of a symbol defined later, and of an external |
| `SHR1.EXE` | The model library: a transfer vector module first (`CLUSTER=`), CALL and JSB routines, a universal data longword (`UNIVERSAL=`), a writable private psect, a read-only shared psect, a `.ADDRESS` of its own label (a self-relative fixup), a G^ call to `LIB$PUT_OUTPUT` (its own fixup section and SHL), `GSMATCH=LEQUAL,1,1000`. The questions: the header, ISD order and types, the GST's records, `OPR_REDEF`'s internal and external values, and the fixup section |
| `SHR1A.EXE` | `SHR1` with no transfer vectors, only `UNIVERSAL=`; no GSMATCH (the time-based default) |
| `SHR2.EXE` | A shareable image that uses SHR1 (`SHR1/SHAREABLE`): a shareable image's SHL and G^ cells, and whether a program linked to SHR2 lists SHR1 |
| `SHR3.EXE` | `BASE=%X10000` with a `NOPIC` psect: a based image, `ISD$V_BASED`, PICIMG |
| `SHR4.EXE` | `GSMATCH=ALWAYS,0,0`, `PSECT_ATTR=` changing a psect to NOSHR, `COLLECT=`, and `DZRO_MIN=`/`ISD_MAX=` |
| `MAIN1.EXE` | Calls SHR1's routines (CALLS and JSB through G^), reads its universal data through G^, and holds `.ADDRESS` of a SHR1 routine. It prints each address it reaches, less the image base it finds (the probe shows where the activator put SHR1) |
| `MAIN2.EXE` | Uses SHR2, which uses SHR1 |
| `EXE1.EXE` | An executable with `CLUSTER=`, `COLLECT=`, `PSECT_ATTR=`, `BASE=`, `IOSEGMENT=`, and options from `SYS$INPUT` |
| (errors) | LINK: `UNIVERSAL=` of an undefined symbol, `.TRANSFER` in an executable link, `/SHAREABLE` on the command line, `GSMATCH=` with bad values, a based image that overlaps another. The messages and whether an image is made |

The runs (each followed by `SHOW SYMBOL $STATUS`):

- MAIN1 with `DEFINE SHR1 [...]SHR1.EXE`, then with the logical name
  deassigned and the image copied to a directory `SYS$SHARE` names (if
  the probe may write there; otherwise a process-level `SYS$SHARE`
  search list), then with neither (the missing image's messages);
- MAIN1 after SHR1 is relinked with minor 1001 (it runs), with minor
  999, and with major 2 (`SHRIDMISMAT`?);
- MAIN1 against SHR1A relinked without GSMATCH;
- MAIN2;
- `RUN SHR1` (a shareable image run directly).

## Part C — subtasks

Each subtask is one tested commit, as in earlier phases. Subtasks 2, 3,
and 10 don't need the probe's answers. The rest are checked against
them.

1. **The probe** (`testdata/link68`; written 2026-10-19, waiting for
   the VAX run): the sources, options files,
   `link68.com`, `exchange.cmd` and `copyout.cmd` (as
   `testdata/pg67`'s), and a README with the cases. The author runs it
   on the VAX, and the output goes in `vax/`. Then a pass over the
   results records the answers in this doc's Part A: the GST's layout,
   `OPR_REDEF`, the section types and order, the shareable image's
   fixup section, the maps, and the messages. The pass also adjusts the
   later subtasks.
2. **MACRO: `.TRANSFER`.** The directive, its `OPR_REDEF` in the
   object, its listing line, and errors. It is matched against real
   MACRO's `XFR1.OBJ` and listing once the probe is back; until then it
   is written from the manual's 7.4. Anything else the probe's objects
   show MACRO lacks. In the linker, pass 2 takes `OPR_REDEF` (an
   executable link ignores or refuses it, as the probe says).
3. **Options files, all of VAX LINK's options.** The parser and its
   checks for `UNIVERSAL=`, `GSMATCH=`, `CLUSTER=`, `COLLECT=`,
   `PSECT_ATTR=`, `BASE=`, `DZRO_MIN=`, `ISD_MAX=`, `PROTECT=`,
   `IOSEGMENT=`, and `UNSUPPORTED=`, with their abbreviations and
   values. Also `SYS$INPUT/OPTIONS`, reading a procedure's data lines
   (DCL's SYS$INPUT, Phase 50) and the terminal at level 0. Until later
   subtasks act on an option, `Link` refuses it with a clear message.
   Unit tests only.
4. **Clusters, for executables.** The cluster list: the default cluster,
   `CLUSTER=` (with base and file names), `COLLECT=`, and shareable
   image clusters in their place. Also the two allocation passes (6.3.4),
   `PSECT_ATTR=`, demand-zero compression by `DZRO_MIN`/`ISD_MAX`, and
   `BASE=`. The map's cluster lines. Matched against `EXE1`, and the
   Phase 30 fixtures must stay byte for byte.
5. **`LINK/SHAREABLE`, the image.** The console's `/SHAREABLE[=file]`
   and `govax link --shareable`. The `IHD$K_LIM` header at base 0,
   Table 6-1's shareable order (PIC, SHR), each ISD's section type
   (SHRPIC, PRVPIC, SHRFXD, PRVFXD), no stack ISD, GSMATCH in the
   header (the default from the link time), and the transfer addresses
   a shareable image has. A shareable image with no outside references
   and no `.ADDRESS`, matched against the probe's sections and header.
6. **`LINK/SHAREABLE`, the GST.** Universal symbols from `UNIVERSAL=`
   and `OPR_REDEF`: the transfer vector's address outside the image,
   and the routine's inside it. Also entry masks and the GST's records
   as real LINK writes them, so that `ReadShareableImage` reads
   govax's images as it reads LIBRTL's. `/SYMBOL_TABLE[=file]` writes
   the same table as an `.STB` file.
7. **`LINK/SHAREABLE`, fixups and bases.** A shareable image's fixup
   section, under the manual's two conditions: the `.ADDRESS` list
   relative to the image itself (SHL entry 0), and G^ cells for the
   images it uses. Also a non-PIC image (the PICIMG flag), and based
   images (`BASE=`, `ISD$V_BASED`). Matched against SHR1, SHR2, and
   SHR3.
8. **Maps for shareable links.** Universal symbols (`U`), the
   transfer vector's two values, `/FULL`'s sections as the probe shows
   them, and the synopsis of a link that uses govax's own shareable
   images.
9. **Linking against govax's shareable images.** An options file's
   `MYSHR/SHAREABLE` of a govax-made image. The global section ISD and
   SHL entry from its header (its name, `_001`, ident, match), the
   order of several shareable images, and a shareable image that uses
   another. MAIN1 and MAIN2, byte for byte.
10. **Finding a shareable image.** Decision 3's locator, used by both
    loaders (copy and map), by LINK for IMAGELIB's images (folding
    `readLibraryFile`'s rule into it), and exported for Phase 61. SHOW
    IMAGES shows each image's file. The missing image's messages
    (Decision 4). Console tests with a logical name, `SYS$SHARE` on a
    volume, and the host.
11. **Activating govax's shareable images.** The GSMATCH check, a
    shareable image's own fixups (self-relative `.ADDRESS`, its G^
    cells), dependencies of dependencies, and where each image lands
    (as the probe shows). RUN of a shareable image itself. All of this
    in both paging modes. `imageshare_test.go`'s hand-made image gives
    way to one LINK/SHAREABLE makes.
12. **Shareable image libraries** (if question 2 says so).
    `LIBRARY/CREATE/SHARE` and `/INSERT` of a shareable image (its GST
    as the library's module), and `LIBRARY/LIST` of one. LINK searches
    a user's shareable image library (`MYLIB/LIBRARY`), and its default
    libraries (`LNK$LIBRARY`, `LNK$LIBRARY_1`, ...; `/USERLIBRARY`).
13. **The milestone and the oracle.** `TestLink68Oracle` runs
    `LINK68.COM` on govax: MACRO, LINK/SHAREABLE, LINK, ANALYZE, and
    RUN. It matches VMS's log, maps, and `.ANI`s (times masked) and the
    images byte for byte, but for a listed set of differences. A second
    VAX round runs govax's images on VMS (needs the author).
14. **Close-out.** HELP LINK and HELP RUN (`console.help`: the new
    qualifiers and options, and how a shareable image is found),
    `CLAUDE.md`, `PLAN.md`, and `DEVIATIONS.md`. The Phase 30 doc gets a
    pointer here.

## Progress log

### 2026-10-19 — Planned

Surveyed the linker, MACRO, the image loaders, and the Linker manual
(Part A). Proposed the decisions and subtasks for the author's review.

### 2026-10-19 — Plan accepted; subtask 1's probe written

The author accepted the proposals (Part B's answers). The probe is in
`testdata/link68` (its README lists the files and steps). It has eleven
sources: XFR1 (the transfer vector), SHR1, SHR2, SHR3, SHR6, MAIN1,
MAIN2, MAIN3, MAIN6, EXE1, and EXE1B. There are 21 links that make
images, 8 GSMATCH relinks of SHR1, 12 runs, the shareable image library
cases, and 9 error cases. All but XFR1 assemble with govax's MACRO.
`exchange.cmd` builds `testdata/disks/link68-exchange.dsk`, and
`copyout.cmd` brings back the log, listings, objects, maps, `.ANI`s,
images, `SHR1.STB`, and `MYSHR.OLB`. The VAX run is the author's.

A dry run of `LINK68.COM` under govax's own DCL found no mistakes in
the procedure. It did list govax's gaps, which later subtasks will
close:

- MACRO has no `.TRANSFER` (subtask 2).
- LINK has no `/SHAREABLE`, `/FULL`, `/SYMBOL_TABLE`, or
  `/USERLIBRARY` (subtasks 5, 6, 8, and 12).
- LINK doesn't read `SYS$INPUT/OPTIONS`: it looks for a file
  `SYS$INPUT.OPT` (subtask 3).
- When a LINK fails, the options lines after it run as DCL commands.
  govax's DCL treats data after one of its own commands that way
  (Phase 50's `dclOwnCommands`); VMS, where LINK is an image, skips
  them (SKPDAT). Subtask 3 should make the data LINK's, whether or not
  the link goes ahead.
- `LIBRARY` has no `/SHARE` (subtask 12), and `DIRECTORY/OUTPUT` is
  missing.
- `SYS$SHARE` isn't defined in govax (subtask 10).
- `F$GETSYI("HW_NAME")` isn't an item govax's F$GETSYI knows.

