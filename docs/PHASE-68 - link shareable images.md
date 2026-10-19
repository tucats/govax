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

### The probe's answers (subtask 1, 2026-10-19)

VMS V7.1 on the simh VAX (an 8600) ran `LINK68.COM`. Its LINK is
V11-39 and its MACRO V5.4-3. Everything is in `testdata/link68/vax/`:
`link68.log`, the listings, objects, maps, `.ANI`s, and images, plus
`shr1.stb`, `myshr.olb`, and `myshr.lis`. Every image ANALYZE/IMAGE
checked has 0 errors. Unconfirmed points are marked; a later round
can settle them.

**MACRO.** `.TRANSFER sym` is `TIR$C_OPR_REDEF` with the symbol's
name, in the TIR stream at the location counter, and nothing else.
`.MASK sym,expr` is `STA_UB expr`, `STA_EPM sym`, `OPR_IOR`, then
`STO_W` (`xfr1.anl`).

**A shareable image's header** (`shr1.ani`; `shr1a.exe`'s bytes):

- `IHD$B_IMGTYPE` is 2 (`IHD$K_LIM`).
- GSMATCH is in the existing fields. `IHD$L_IDENT` (+0x24) is the
  major ident in its top byte and the minor in its low 24 bits, and the
  match control is the top byte of `IHD$L_LNKFLAGS` (+0x20: 0 ALWAYS,
  1 EQUAL, 2 LEQUAL).
- Without `GSMATCH=`, the ident is bits 16–47 of the link time (EQUAL).
  Executables already get this (Phase 30's "IDENT is bytes 2 to 5 of
  the link time").
- `IHD$V_LNKNOTFR` is set: a shareable image has no user transfer
  address. The first transfer address is `SYS$IMGSTA` with traceback
  and 0 without (SHR1A, `/NOTRACEBACK`).
- `PICIMG` is set unless the image is based (SHR3); a nonbased image
  with NOPIC psects still has it (SHR3P).
- VMS's ANALYZE/IMAGE shows the GSMATCH under the image type (`global
  section major id: %X'01', minor id: %X'0003E8'`, `match control:
  ISD$K_MATLEQ`), and, after the ISDs, a section `SHAREABLE IMAGE -
  GLOBAL SYMBOL TABLE` analyzing the GST's records, as ANALYZE/OBJECT
  would. govax's ANALYZE/IMAGE has neither.

**Sections** (`shr1.ani`, `shr1.map`):

- Base 0 and no user stack ISD. Each cluster starts on a page:
  SHR1_VECTORS (XFR1's two psects, in one section) at 0, then the
  default cluster in Table 6-1's shareable order.
- Section types: SHR+PIC is `SHRPIC`, NOSHR+PIC `PRVPIC`, SHR+NOPIC
  `SHRFXD`, NOSHR+NOPIC `PRVFXD`. Writable private sections get CRF+WRT,
  as in executables.
- **The fixup section is `PRVFXD`** (CRF, WRT, FIXUPVEC), after the last
  user cluster, and its IAF has `IAF$V_SHR` set. It appears when the
  image uses another shareable image or has a `.ADDRESS` (SHR3P has
  only `.ADDRESS` fixups; SHR3, based, has no fixup section at all).
- **`.ADDRESS` fixups are relative to image 0, the image itself** ("3
  references to image 0", relative to `%X'00000000'`); `.ASCID`'s
  pointers count too. The change-protection entry is the fixup page,
  `PRT$C_UREW`, as in executables.
- **A `.ADDRESS` in a shared, read-only psect** (SHR6) makes that
  section CRF+WRT (still `SHRPIC`), and a second change-protection
  entry sets its page to `PRT$C_UR` after the fixups. At run time the
  fixup was right (`MAIN6: SHR6_GET() 0`).
- **No demand-zero compression in these shareable images.** SHR1's 8
  pages that are never stored in are written into the file, in a
  9-page `PRVPIC` section, even with `DZRO_MIN=1` (SHR4). `ISD_MAX=` is
  ignored with `%LINK-I-OPTIGNSHR, option ignored for shareable image`.
  LIBRTL has a DZRO section, so the rule is unconfirmed: these images
  have none.
- `PSECT_ATTR=SHR1_RO,NOSHR` made a read-only `PRVPIC` section (no CRF).
  `COLLECT=FIRST,SHR1_RO` moved it into cluster FIRST, after XFR1's
  `SHRPIC` section, by Table 6-1's order (SHR4).
- `PROTECT=YES` around a cluster sets `ISD$V_PROTECT` (bit 18) on its
  sections (SHR5).
- A based shareable image (SHR3, `BASE=%X300000`) has its sections at
  their addresses and no fixup section.

**The GST** (`shr1.ani`, 5 records):

- a main header (the image's name and ident, the link time; maximum
  record size 512);
- an LNM, `Linker V11-39`;
- a GSD defining psect 0, `.$$ABS$$.` (PIC, LIB, RD);
- one GSD of the universal symbols: `EPM` for an entry point (with its
  mask) and `SYM` otherwise, each flagged DEF, UNI, REL, psect 0, with
  its offset in the image;
- an EOM.

The symbols' order was SHR1_LOCAL, TABLE, SHOW, ADD, BASE, JSB,
ORIGIN, COUNT; the rule is unconfirmed. In a based image the symbols
are absolute (no REL). A `.TRANSFER`ed symbol's GST value is its
transfer vector entry (`SHR1_ADD` is 8), and it is an `EPM` carrying
the vector's mask. Inside the image, references go to the routine:
the map's "Symbols By Name" shows `SHR1_ADD 00000407-RU`, and "By
Value" shows `R-SHR1_ADD` without the U. `shr1.stb`
(`/SYMBOL_TABLE`) is the same table as an object module file (262
bytes).

**A program linked against a shareable image** (`main1.ani`):

- The global section ISD's page count is the shareable image's first
  ISD's (SHR1: 1; LIBRTL: 264). Its name is `NAME_001`, with the
  image's ident and match control.
- The SHL lists the images in the order of the link's input:
  SHR1 (`/SHAREABLE` in an options file) before LIBRTL (IMAGELIB).
- G^ cells hold offsets in the image: the transfer vector entries for
  routines, data offsets for data.
- A based shareable image (MAIN3 against SHR3) gets a global ISD per
  section (`SHR3_001` `SHRFXD`, `SHR3_002` `PRVFXD`), each with BASED
  and LASTCLU, at its own address. No G^ cells: references resolve at
  link time.
- `PICIMG` is clear in MAIN3 and EXE1, and set in MAIN1 and MAIN2.
- In MAIN3 and EXE1, VMS's ANALYZE/IMAGE prints the change-protection
  entry's address line twice; in MAIN1 and MAIN2, once. govax's must
  match.

**EXE1** (`exe1.ani`, `exe1.map`):

- `BASE=%X10000` puts the default cluster there, and `CLUSTER=SECOND,
  %X40000` the other; `COLLECT=` moved EXE1_RO into SECOND.
- LINK also **based LIBRTL** at link time, in the gap after the
  default cluster: two global ISDs, `LIBRTL_001` at ^X10E00 and
  `LIBRTL_002` at ^X31E00, with BASED. There are no G^ fixups, and the
  SHL still lists LIBRTL.
- `DZRO_MIN=2` split a section: EXE1_ZERO's 4 uninitialized pages
  became a DZRO ISD, and SAY_DATA's page stayed a separate CRF ISD.
  Phase 30 makes only a whole section DZRO.
- `PSECT_ATTR=EXE1_DATA,NOWRT` worked; `IOSEGMENT=40` is the header's
  I/O page count.
- The fixup section follows the highest user cluster (^X40600), and
  LASTCLU marks cluster SECOND's sections.

**Activation** (`link68.log`):

- MAIN1 at ^X400 (code), LIBRTL mapped at ^X800, SHR1 at ^X23600 (its
  origin ^X23A00): **LIBRTL, last in the SHL, was mapped first**, right
  after the main image. MAIN3P (SHR3P, which doesn't use LIBRTL) puts
  SHR3P after LIBRTL too. So the order is either the SHL from its end,
  or installed images first; unconfirmed, and a later round can
  settle it with two uninstalled images.
- Each image is laid out contiguously: LIBRTL's two ISDs take ^X22E00
  bytes.
- Finding the image (R1–R4):
  - A full logical name works.
  - A logical name with no device (`[000000]SHR1`) takes `SYS$SHARE`'s
    device and root: `%DCL-W-ACTIMAGE, error activating image SHR1`,
    `-CLI-E-IMAGEFNF, image file not found
    SIMVAX$DUA0:[SYS0.SYSCOMMON.][000000]SHR1.EXE;`. The default spec
    is `SYS$SHARE:.EXE`.
  - No logical name gives the same message with
    `...[SYSLIB]SHR1.EXE;`. `$STATUS` is %X100388B2.
  - A process `SYS$SHARE` search list finds SHR1 in its first
    directory.
- GSMATCH, for MAIN1 (linked with LEQUAL 1,1000):
  - LEQUAL 1,1001 runs; LEQUAL 1,999 and LEQUAL 2,1000 fail;
  - **EQUAL 1,1000 fails**, as do EQUAL 1,1001 and ALWAYS 3,3;
  - MAIN1N (EQUAL 1,1001) runs against EQUAL 1,1001 and fails against
    ALWAYS 3,3;
  - MAIN4 (ALWAYS 0,0) runs against ALWAYS 7,7;
  - MAIN1A fails once SHR1A is relinked (the time-based ident).

  The rule that fits every case: **the match controls must agree, and
  then the linked-with control decides**. ALWAYS matches any ident,
  EQUAL needs both idents equal, and LEQUAL needs the same major and a
  minor at least the linked one. ALWAYS in the program against another
  control in the image wasn't tried. A mismatch is `%DCL-W-ACTIMAGE`,
  `-CLI-E-IMGNAME, image file DUA1:[000000]SHR1.EXE;3`,
  `-SYSTEM-F-SHRIDMISMAT, ident mismatch with shareable image`, with
  `$STATUS` %X100020BC.
- `RUN SHR1.EXE` is `%DCL-E-NOTFR, no transfer address`, `$STATUS`
  %X0003889A.
- Every program's results were right: G^ calls and data, a JSB through
  G^, `.ADDRESS` of a routine (the vector) and of data, self-relative
  `.ADDRESS`, a shareable image using another (MAIN2: 42, 1, 0), based
  and nonbased NOPIC images (MAIN3: ^X300200; MAIN3P: ^X23800, so the
  activator relocated SHR3P's `.ADDRESS`), and the library links.

**Options and errors:**

- `GSMATCH=NEVER` is a syntax error in V11-39 (`%LINK-F-OPTSYNERR,
  syntax error in options file ...GSM.OPT;7`, `-LINK-E-OPTLIN, options
  line in error`, then the line with the bad token in quotes:
  `GSMATCH='NEVER',1,1000`). The keywords are ALWAYS, EQUAL, and
  LEQUAL.
- A major ident past 255 is `%LINK-F-OPTVALERR, GSMATCH option value
  300. is out of range (0. to 255.)`. A misspelled option is
  OPTSYNERR with the keyword quoted. `GSMATCH=` twice: the last wins
  (E6's header), with no message.
- `UNIVERSAL=` of an undefined symbol: `%LINK-W-NUDFSYMS`/`UDFSYM`, and
  the image is still made (E1).
- `UNIVERSAL=` in an executable is accepted silently (E8).
- `.TRANSFER` in an executable link: no message about it; LINK only
  warns `%LINK-W-USRTFR, image ... has no user transfer address` and
  makes the image, with LNKNOTFR set (E2).
- A shareable image named as an object (E3): `RECTYP`, `SEQNCE`, `NOEOM`,
  `ERRORS`, `NOIMGFIL`, then the undefined symbols and `ILLRECTYP`s,
  `$STATUS` %X106482B2.
- An executable based on top of a based shareable image (E5):
  `%LINK-W-BASSHRBEL` and `%LINK-E-CONFMEM`, no image.
- The statuses: %X10000001 for a clean link, %X10648268 with warnings,
  %X106483B4 for OPTSYNERR, and %X106483BC for OPTVALERR.
- `/NOUSERLIBRARY` (E9) leaves SHR1's symbols undefined (`USEUNDEF` for
  each reference) and still makes an image.

**Libraries:** `LIBRARY/CREATE/SHARE` makes a "SHAREABLE IMAGE SYMBOL
TABLE library" (`myshr.lis`). Each module is listed with `Ident
01,0003E8` and its symbol count. Linking `MYSHR/LIBRARY` and through
`LNK$LIBRARY` both found SHR1, and the programs ran.

**What govax already does with VMS's images:** `link.ReadShareableImage`
reads SHR1, SHR1A, SHR2, and SHR4 correctly: the vector offsets, the
ident, the match control, and the first section's page count. It reads
SHR3 (based) as absolute symbols, but with a page count of 0, and it
refuses `shr1.stb`, which is an object module, not an image.

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

1. **Done (2026-10-19).** **The probe** (`testdata/link68`; Part A's
   "The probe's answers"): the sources, options files,
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
   show MACRO lacks. In the linker, pass 2 takes `OPR_REDEF`: an
   executable link ignores it, as V11-39 did with E2.
3. **Options files, all of VAX LINK's options.** The parser and its
   checks for `UNIVERSAL=`, `GSMATCH=`, `CLUSTER=`, `COLLECT=`,
   `PSECT_ATTR=`, `BASE=`, `DZRO_MIN=`, `ISD_MAX=`, `PROTECT=`,
   `IOSEGMENT=`, and `UNSUPPORTED=`, with their abbreviations and
   values. Also `SYS$INPUT/OPTIONS`, reading a procedure's data lines
   (DCL's SYS$INPUT, Phase 50) and the terminal at level 0. Until later
   subtasks act on an option, `Link` refuses it with a clear message.
   The probe's messages: OPTSYNERR with the bad token quoted (NEVER
   isn't a GSMATCH keyword), OPTVALERR, and the last GSMATCH= wins.
   When a LINK fails, its `SYS$INPUT` data lines are skipped
   (SKPDAT), not run as commands. Unit tests only.
4. **Clusters, for executables.** The cluster list: the default cluster,
   `CLUSTER=` (with base and file names), `COLLECT=`, and shareable
   image clusters in their place. Also the two allocation passes (6.3.4),
   `PSECT_ATTR=`, demand-zero compression by `DZRO_MIN`/`ISD_MAX`
   (splitting a section's uninitialized run off as its own ISD, as
   EXE1's was), `BASE=`, and `IOSEGMENT=`. With `BASE=`, shareable
   images are based at link time in the gaps (EXE1's LIBRTL_001/002,
   BASED, no G^ cells). The map's cluster lines. Matched against
   `EXE1`, and the Phase 30 fixtures must stay byte for byte.
5. **`LINK/SHAREABLE`, the image.** The console's `/SHAREABLE[=file]`
   and `govax link --shareable`. The `IHD$K_LIM` header at base 0,
   Table 6-1's shareable order (PIC, SHR), each ISD's section type
   (SHRPIC, PRVPIC, SHRFXD, PRVFXD), no stack ISD, GSMATCH in the
   header (`IHD$L_IDENT` and the link flags' top byte; the default
   from the link time), LNKNOTFR, and the first transfer address
   (`SYS$IMGSTA` with traceback). No demand-zero compression, and
   `ISD_MAX=`'s OPTIGNSHR. `PROTECT=`'s `ISD$V_PROTECT`. A shareable
   image with no outside references and no `.ADDRESS`, matched against
   the probe's sections and header.
6. **`LINK/SHAREABLE`, the GST.** Universal symbols from `UNIVERSAL=`
   and `OPR_REDEF`: the transfer vector's address outside the image,
   and the routine's inside it. Also entry masks and the GST's records
   as real LINK writes them (Part A: MHD, LNM, `.$$ABS$$.`, one GSD of
   EPM/SYM entries with UNI and REL, EOM; the order to be worked out
   from the probe's images), so that `ReadShareableImage` reads
   govax's images as it reads LIBRTL's. `/SYMBOL_TABLE[=file]` writes
   the same table as an `.STB` file.
7. **`LINK/SHAREABLE`, fixups and bases.** A shareable image's fixup
   section (`PRVFXD`, `IAF$V_SHR`), under the manual's two conditions:
   the `.ADDRESS` list relative to the image itself (SHL entry 0), and
   G^ cells for the images it uses. A `.ADDRESS` in a shared read-only
   section makes it CRF+WRT, with a `PRT$C_UR` change-protection entry
   (SHR6). Also NOPIC psects (`SHRFXD`/`PRVFXD`; PICIMG stays set
   unless based), and based images (`BASE=`, no fixup section,
   absolute GST symbols). Matched against SHR1, SHR2, SHR3, SHR3P,
   and SHR6.
8. **Maps and ANALYZE/IMAGE for shareable images.** The map:
   universal symbols (`U`), the transfer vector's two values (`RU` by
   name, `R-` by value), `/FULL`'s sections (the Image Section
   Synopsis with clusters, types, global section names, and
   match/idents), and the synopsis's `Image type: PIC, SHAREABLE.
   Global Section Match=...`. ANALYZE/IMAGE: the header's GSMATCH
   lines, the GST section, and the change-protection address printed
   twice for the based images (MAIN3, EXE1), all matched against the
   probe's `.ANI`s.
9. **Linking against govax's shareable images.** An options file's
   `MYSHR/SHAREABLE` of a govax-made image. The global section ISD and
   SHL entry from its header (its name, `_001`, ident, match, and the
   first ISD's page count), the order of several shareable images (the
   link's input order, IMAGELIB's last), and a shareable image that
   uses another. A based shareable image: a global ISD per section,
   BASED and LASTCLU, no G^ cells (`ReadShareableImage` must give its
   sections, not a page count of 0). MAIN1, MAIN2, and MAIN3, byte for
   byte.
10. **Finding a shareable image.** Decision 3's locator, used by both
    loaders (copy and map), by LINK for IMAGELIB's images (folding
    `readLibraryFile`'s rule into it), and exported for Phase 61. SHOW
    IMAGES shows each image's file. The default spec is
    `SYS$SHARE:.EXE`, merged with a logical name's translation (R2).
    The missing image's messages (Decision 4): ACTIMAGE, then IMAGEFNF
    with the spec RMS tried, `$STATUS` %X100388B2. Console tests with a
    logical name, a partial one, `SYS$SHARE` on a volume and as a
    search list, and the host.
11. **Activating govax's shareable images.** The GSMATCH check (Part
    A's rule: the match controls agree, then the linked-with control
    decides; ACTIMAGE, IMGNAME, SHRIDMISMAT, %X100020BC), a shareable
    image's own fixups (self-relative `.ADDRESS`, its G^ cells, a
    read-only page fixed up then made UR), dependencies of
    dependencies, based shareable images at their addresses, and where
    each image lands (the SHL from its end, as the probe suggests,
    unless a later round says otherwise). `RUN` of a shareable image:
    `%DCL-E-NOTFR`. All of this in both paging modes. `imageshare_test.go`'s hand-made image gives
    way to one LINK/SHAREABLE makes.
12. **Shareable image libraries.**
    `LIBRARY/CREATE/SHARE` and `/INSERT` of a shareable image (its GST
    as the library's module), and `LIBRARY/LIST` of one. LINK searches
    a user's shareable image library (`MYLIB/LIBRARY`), and its default
    libraries (`LNK$LIBRARY`, `LNK$LIBRARY_1`, ...; `/USERLIBRARY`),
    matched against `myshr.olb`, `myshr.lis`, and MAIN1L/MAIN1U.
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

### 2026-10-19 — Subtask 1: the probe's VMS run

The author ran `LINK68.COM` on VMS V7.1 (LINK V11-39) and copied the
results into `testdata/link68/vax/`. Part A's "The probe's answers"
records what they show, and the subtasks they touch now carry those
answers. Several things the plan didn't foresee:

- The fixup section of a shareable image is `PRVFXD` with `IAF$V_SHR`.
- A `.ADDRESS` in a shared read-only psect makes it CRF+WRT, with a UR
  change-protection entry.
- These shareable images got no demand-zero compression, even with
  `DZRO_MIN=1`.
- With `BASE=`, an executable bases LIBRTL at link time.
- `DZRO_MIN` splits an uninitialized run off a section.
- GSMATCH needs the match controls to agree.
- LIBRTL was mapped before SHR1.
- VMS's ANALYZE/IMAGE shows a shareable image's GSMATCH and GST.
- `GSMATCH=NEVER` isn't accepted.

Two points are left unconfirmed: the order in which the activator maps
images, and the GST's symbol order. A second round can take them up
when subtask 11 needs them.

