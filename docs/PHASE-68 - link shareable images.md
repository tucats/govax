# Phase 68 — LINK makes shareable images, and programs use them

**Status:** planned (2026-10-19); the plan reviewed and accepted by the
author the same day. Part A is the survey. Part B holds the decisions
and the author's answers to the plan's questions. Part C is the
subtasks. Subtask 1, the VMS probe (`testdata/link68`), is done, and so
are subtask 2 (MACRO's `.TRANSFER`), subtask 3 (options files),
subtask 4 (clusters for executables), subtask 5 (`LINK/SHAREABLE`,
the image), subtask 6 (the global symbol table), subtask 7 (fixups
and bases), subtask 8 (maps and ANALYZE/IMAGE), and subtask 9 (linking
against govax's shareable images).

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
the routine's own entry mask (`SHR1_ADD`'s 0, not the vector's
`^M<R4,R5>`; subtask 6). Inside the image, references go to the routine:
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
2. **Done (2026-10-19).** **MACRO: `.TRANSFER`.** The directive, its `OPR_REDEF` in the
   object, its listing line, and errors. It is matched against real
   MACRO's `XFR1.OBJ` and listing once the probe is back; until then it
   is written from the manual's 7.4. Anything else the probe's objects
   show MACRO lacks. In the linker, pass 2 takes `OPR_REDEF`: an
   executable link ignores it, as V11-39 did with E2.
3. **Done (2026-10-19).** **Options files, all of VAX LINK's options.** The parser and its
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
4. **Done (2026-10-19).** **Clusters, for executables.** The cluster list: the default cluster,
   `CLUSTER=` (with base and file names), `COLLECT=`, and shareable
   image clusters in their place. Also the two allocation passes (6.3.4),
   `PSECT_ATTR=`, demand-zero compression by `DZRO_MIN`/`ISD_MAX`
   (splitting a section's uninitialized run off as its own ISD, as
   EXE1's was), `BASE=`, and `IOSEGMENT=`. With `BASE=`, shareable
   images are based at link time in the gaps (EXE1's LIBRTL_001/002,
   BASED, no G^ cells). The map's cluster lines. Matched against
   `EXE1`, and the Phase 30 fixtures must stay byte for byte.
5. **Done (2026-10-19).** **`LINK/SHAREABLE`, the image.** The console's `/SHAREABLE[=file]`
   and `govax link --shareable`. The `IHD$K_LIM` header at base 0,
   Table 6-1's shareable order (PIC, SHR), each ISD's section type
   (SHRPIC, PRVPIC, SHRFXD, PRVFXD), no stack ISD, GSMATCH in the
   header (`IHD$L_IDENT` and the link flags' top byte; the default
   from the link time), LNKNOTFR, and the first transfer address
   (`SYS$IMGSTA` with traceback). No demand-zero compression, and
   `ISD_MAX=`'s OPTIGNSHR. `PROTECT=`'s `ISD$V_PROTECT`. A shareable
   image with no outside references and no `.ADDRESS`, matched against
   the probe's sections and header.
6. **Done (2026-10-19).** **`LINK/SHAREABLE`, the GST.** Universal symbols from `UNIVERSAL=`
   and `OPR_REDEF`: the transfer vector's address outside the image,
   and the routine's inside it. Also entry masks and the GST's records
   as real LINK writes them (Part A: MHD, LNM, `.$$ABS$$.`, one GSD of
   EPM/SYM entries with UNI and REL, EOM; the order to be worked out
   from the probe's images), so that `ReadShareableImage` reads
   govax's images as it reads LIBRTL's. `/SYMBOL_TABLE[=file]` writes
   the same table as an `.STB` file.
7. **Done (2026-10-19).** **`LINK/SHAREABLE`, fixups and bases.** A shareable image's fixup
   section (`PRVFXD`, `IAF$V_SHR`), under the manual's two conditions:
   the `.ADDRESS` list relative to the image itself (SHL entry 0), and
   G^ cells for the images it uses (all three done in subtask 5, which
   SHR1 needed; what's left is checking them against SHR2's and SHR6's
   and anything they show). A `.ADDRESS` in a shared read-only
   section makes it CRF+WRT, with a `PRT$C_UR` change-protection entry
   (SHR6). Also NOPIC psects (`SHRFXD`/`PRVFXD`; PICIMG stays set
   unless based), and based images (`BASE=`, no fixup section,
   absolute GST symbols). Matched against SHR1, SHR2, SHR3, SHR3P,
   and SHR6.
8. **Done (2026-10-19).** **Maps and ANALYZE/IMAGE for shareable images.** The map:
   universal symbols (`U`), the transfer vector's two values (`RU` by
   name, `R-` by value), `/FULL`'s sections (the Image Section
   Synopsis with clusters, types, global section names, and
   match/idents), and the synopsis's `Image type: PIC, SHAREABLE.
   Global Section Match=...`. ANALYZE/IMAGE: the header's GSMATCH
   lines, the GST section, and the change-protection address printed
   twice for the based images (MAIN3, EXE1), all matched against the
   probe's `.ANI`s.
9. **Done (2026-10-19).** **Linking against govax's shareable images.** An options file's
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
    dependencies, based shareable images at their addresses (and an
    executable's link-time-placed images, EXE1's LIBRTL: its shims at
    the placed addresses, subtask 4), and where
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


### 2026-10-19 — Subtask 2: MACRO's `.TRANSFER`

`.TRANSFER symbol` is a MACRO-only directive (`macrodir.go`'s
`pseudoTransfer`). It stores nothing and makes the symbol global, and
the object gets a `TIR$C_OPR_REDEF` with the symbol's name at the
location counter (`output.go`'s `evTransfer`). Its listing line shows
only the location counter. govax's objects and listings of all eleven
of the probe's sources now match real MACRO's (`internal/asm`'s
`TestLink68Objects` and `TestLink68Listings`). Getting there turned up
three things:

- **`.MASK sym,expr` stacks the expression first**, then `STA_EPM`,
  then `OPR_IOR`. govax had the two the other way round, and two unit
  tests (`TestMaskMACRO`, `TestObjectMask`) held that guess; they now
  hold XFR1's order.
- **A symbol `.TRANSFER` names is referred to by name.** XFR1's
  `BRW SHR1_LOCAL+2` reaches its own `.ENTRY`, defined later in another
  psect, through `STA_GBL SHR1_LOCAL`, not the psect and offset. govax
  takes the rule to be the `.TRANSFER` (`SymTransfer`): the linker may
  redefine such a symbol, so every reference to it, forward or back,
  goes by name. **Unconfirmed**: the other reading is that MACRO refers
  to any global of its own by name when the reference is forward and in
  another psect. No real object settles it. Every cross-psect reference
  to a module's own global in `testdata` is a backward one, and those go
  by psect and offset. A reference read before the `.TRANSFER`, to a
  symbol already defined, keeps its psect and offset in govax. A later
  VAX round can settle both with a forward reference to a global of
  another psect, without `.TRANSFER`.
- **A branch the linker finishes is listed as a displacement.** The
  listing shows that BRW's field as `FFDD'`, the placeholder (2) less
  the end of the field (^X25). An operand's displacement shows the
  placeholder itself (`JMP L^SHR1_BASE+2` is `00000002'`), as govax
  already did. No earlier listing had a relocated branch.

`.TRANSFER` in an absolute psect is ABSDATA, as data there is. The
linker must know the location, but real MACRO's rule there is
unconfirmed. `.TRANSFER` with no symbol is EXTRATEXT, and of a local
label NOTGLOBAL, as `.GLOBAL`'s are; the console dialect's ASM refuses
it (MACROONLY).

In the linker, pass 2 takes `OPR_REDEF` and does nothing with it in an
executable link (`pass2.go`). That is enough for the probe's E2 (XFR1
and SHR1 linked as an executable) to match real LINK's image byte for
byte, from real MACRO's objects and from govax's
(`TestLinkTransferVectorExecutable`), with `%LINK-W-USRTFR` as VMS
gave it. Subtask 6 makes the redefinition a shareable image's
universal value.

### 2026-10-19 — Subtask 3: options files

`internal/link/options.go` reads every VAX LINK option: `BASE=`,
`CLUSTER=` (name, base, pfc, files; the files go among the input files
with their cluster), `COLLECT=`, `DZRO_MIN=`, `GSMATCH=`,
`IDENTIFICATION=`, `IOSEGMENT=` (`[NO]P0BUFS`), `ISD_MAX=`, `NAME=`,
`PROTECT=`, `PSECT_ATTR=` (also spelled `PSECT_ATTRIBUTE=`, VSI's name:
the attribute keywords and an alignment by keyword or power of 2),
`STACK=`, `SYMBOL=`, `UNIVERSAL=`, and `UNSUPPORTED=`, each abbreviated
to any unique prefix. `OptionsParser` reads a file a line at a time, as
VMS's LINK does: the log shows `%LINK-I-OPTIGNSHR` between the line it
was for and the next. A line in error is an `OptionError`, which words
OPTSYNERR and OPTVALERR as the probe's log does, the part in error
between apostrophes, with LINK's statuses (%X106483B4 and %X106483BC,
the inhibit bit set). `TestOptionErrors` holds the probe's four
messages word for word. A later options file's values replace an
earlier one's, and its lists add to them (`OptionsFile.Merge`), so the
last `GSMATCH=` wins, as E6 showed.

In the LINK command (`internal/dclcmd/link.go`):

- **`SYS$INPUT/OPTIONS`** (or a logical name for SYS$INPUT) reads the
  options from SYS$INPUT, a line at a time through the process's input:
  in a procedure, the data lines after the command; at the terminal,
  lines typed up to Ctrl/Z. Its name in a message is `SYS$INPUT:.;`, as
  in the probe's E4.
- **With image verification on, an options file's lines are shown as
  LINK reads them**, as VMS's log shows SHR1.OPT's and GSM.OPT's (DCL
  already shows the SYS$INPUT data lines an image reads).
- LINK stops at the first line in error, so the data lines after it are
  DCL's to skip, with SKPDAT, as after any image (LINK isn't one of
  `dclOwnCommands`). The dry run's lines run as commands came from
  LINK's qualifiers the grammar didn't have yet: a command the grammar
  refuses runs no image, so DCL reads the next line as a command (what
  VMS does there is unconfirmed).
- An option later subtasks act on (`BASE=`, `CLUSTER=`, `COLLECT=`,
  `DZRO_MIN=`, `IOSEGMENT=`, `ISD_MAX=`, `PSECT_ATTR=`, `UNSUPPORTED=`)
  is refused: "the option BASE= isn't supported yet"
  (`OptionsFile.Unimplemented`). `UNIVERSAL=` and `GSMATCH=` are
  accepted and ignored in an executable link, as VMS's LINK accepted
  `UNIVERSAL=` there (E8; `GSMATCH=` there is unconfirmed). `PROTECT=`
  only marks the clusters after it, so it needs no refusal of its own.

`TestLinkOptionsInput` runs the cases through DCL with SET VERIFY: a
SYS$INPUT error, a file's error, a good link, and a refused option.

Unconfirmed, govax's choices where the probe is silent:

- The checks beyond the probe's: a value that isn't a number or a
  keyword, a missing or too-long name, a value too many or too few, and
  an input file with an unknown qualifier are OPTSYNERR with that value
  quoted; a number out of range is OPTVALERR. The ranges: the GSMATCH
  minor ID 0 to %XFFFFFF (its header field), STACK= 1 to 2097152, a
  cluster's pfc 0 to 127, an alignment 0 to 9, DZRO_MIN= and ISD_MAX=
  at least 1.
- The line shown in a message is the statement with its comment
  removed and its continuation lines joined (the probe's were all one
  line).
- `PROTECT=` holds to the end of its options file, not into the next.
- `IOSEGMENT=`'s default is P0BUFS (the 5.0 manual; VSI's says
  NOP0BUFS).
- What `UNSUPPORTED=` does: neither manual describes it. It takes a
  number.
- `/SHAREABLE=COPY` stays a plain refusal, not a syntax error: it is
  VMS's, not govax's.

`OPTIGNSHR` (`ISD_MAX=` in a shareable image) waits for `/SHAREABLE`,
subtask 5.

### 2026-10-19 — Subtask 4: clusters, for executables

The linker has clusters now (`internal/link/cluster.go`), and acts on
every layout option: `BASE=`, `CLUSTER=`, `COLLECT=`, `PSECT_ATTR=`,
`DZRO_MIN=`, `ISD_MAX=`, `IOSEGMENT=`, and `PROTECT=`. Only
`UNSUPPORTED=` is still refused. `TestLinkClustersExecutable` links
EXE1, from real MACRO's objects and from govax's, and its image is real
LINK's byte for byte. `TestMapClustersExecutable` holds the lines of
EXE1's map that a default map shares with VMS's `/FULL` one: the psect
synopsis and the image synopsis's layout lines. The Phase 30 fixtures
stay byte for byte. `link.Options` takes the options, and the console's
LINK passes them.

What EXE1's bytes and the other maps settled:

- **The cluster list** is the CLUSTER= options' clusters and the options
  files' shareable images, in order, then the default cluster, then the
  shareable images found in libraries. Pass 1 reads the files cluster by
  cluster, so EXE1B's module comes before EXE1's (the object module
  synopsis). The maps fit this, with one more rule that waits for
  subtask 9: a shareable image's own images come right after it (MAIN1:
  SHR1, LIBRTL, DEFAULT_CLUSTER; MAIN3: SHR3, DEFAULT_CLUSTER, LIBRTL).
- **A local psect belongs to its cluster.** EXE1's "Number of program
  sections: 11" counts two `. ABS .` psects, EXE1's and EXE1B's. A
  global psect is looked for in every cluster up to the current one.
  COLLECT= makes a psect global (EXE1_RO shows GBL in the map), as the
  manual's 6.3.2 says.
- **Allocation** is the manual's two passes. Based clusters come first,
  in list order (SECOND at ^X40000, the default cluster at BASE='s
  ^X10000). Then the rest go at the lowest free addresses that hold
  them.
- **With BASE=, LIBRTL is placed at link time,** in that second pass:
  ^X10E00, the first gap that holds its 279 pages. Each of its sections
  gets a global ISD with ISD$V_BASED (LIBRTL_001 SHRPIC, LIBRTL_002
  PRVPIC with DZRO+WRT). Its references become absolute (`9F` operands,
  `LIB$PUT_OUTPUT` at ^X11278), with no G^ list. The shareable image
  list still names LIBRTL, and IAF+0x0C (the G^ lists' offset) is 0.
  MAIN3 shows that a based shareable image alone doesn't trigger this:
  its LIBRTL stays activator-placed. Whether a based CLUSTER= without
  BASE= triggers it is **unconfirmed**; govax doesn't place it then.
- **The ISDs are in address order,** user sections, placed shareable
  sections, the fixup section, and the stack together. The
  activator-placed global ISDs come last (MAIN3: the stack before
  LIBRTL_001). The file's blocks follow that order: the default
  cluster's at VBNs 2–4, then SECOND's, then the fixup section.
- **LASTCLU** marks the sections of the cluster at the highest address
  (EXE1: SECOND; MAIN3: SHR3's global ISDs; otherwise the default
  cluster). The stack has it when the default cluster does, and the
  fixup section never has it.
- **The fixup section** follows the user cluster at the highest
  address. Its change-protection address is relative to the image's
  first section (^X10000 in EXE1), as ANALYZE and govax's loader read
  it.
- **PICIMG** is clear when any cluster has a base (EXE1, MAIN3).
  **IOSEGMENT=** is IHD$W_IMGIOCNT (+0x1E), and NOP0BUFS sets
  IHD$V_NOP0BUFS (unconfirmed: the probe had only P0BUFS).
- **DZRO_MIN=** sets the split threshold that runs already used: EXE1's
  four untouched pages became a DZRO ISD at 2. **ISD_MAX=** stops the
  splits once the image's sections would pass it. Its default is 96.
  The manual calls both of those numbers approximate, so govax's exact
  counts are **unconfirmed**. A split costs one section at a section's
  start or end, and two in its middle.
- **CONFMEM:** a based cluster whose pages another already has is VMS's
  `%LINK-E-CONFMEM, conflicting virtual memory requirement` / `at
  %X00300000 for 2. pages in cluster DEFAULT_CLUSTER`, then
  `%LINK-E-NOIMGFIL, image file not created`, `$STATUS` %X106482B2, as
  in E5 (`link.FatalError`).
- The SHL lists an image an options file named before those from
  IMAGELIB (`imageOrder`, the probe's MAIN1). Those from IMAGELIB keep
  the order of their first reference.

A shareable image is placed using its sections (`SharedImage.Layout`).
`ReadShareableImage` now reads them from the image's ISDs, and notes a
based image (PICIMG clear). govax's own tables take LIBRTL's from
`vmsdef.SharedImageSections`. That table is kept by hand from EXE1's
`.ANI`, real VMS output; `TestReadShareableImage` and
`TestCapturedLIBRTLMatchesImage` check it against LIBRTL.EXE where the
file is present. An image with no known sections can't be placed at link
time, and the link says so. A based shareable image (SHR3) is refused
for now; subtask 9 maps it.

Other choices, all **unconfirmed**:

- A COLLECT= of a cluster nothing else makes adds it after the default
  cluster.
- A CLUSTER= option's shareable image gets a cluster of its own after
  the named one.
- A library module goes in the default cluster.
- A shareable image an options file names but nothing uses is left out,
  as before. MAIN3's based SHR3, whose symbols are all absolute,
  suggests VMS keeps it; that's subtask 9's.

Left for later subtasks:

- The map's Image Section Synopsis, which only `/FULL` has (subtask 8).
- **RUN of an image that placed LIBRTL at link time** (subtask 11).
  govax's loader puts LIBRTL's shims at P0's high-water mark, not at the
  placed addresses, so EXE1's absolute calls would fault there. Images
  without BASE= are unaffected, and `TestLink_clusters` runs one laid out
  by BASE= and CLUSTER=.

### 2026-10-19 — Subtask 5: `LINK/SHAREABLE`, the image

`link.Options.Shareable` links a shareable image, and
`link.Options.GSMatch` is its GSMATCH= (`internal/link/shareable.go`
says what differs from an executable). The console's LINK has
`/SHAREABLE[=file]`, and `govax link --shareable`. The probe's SHR1,
SHR1A, SHR4, and SHR5 link byte for byte as real LINK's, from real
MACRO's objects and from govax's, but for the global symbol table, which
is subtask 6's (`TestLinkShareableImages`). `TestLinkShareable` runs
SHR4's link through DCL with SYS$INPUT options and SET VERIFY, and
matches VMS's log and image.

What the images settled:

- **The header:** `IHD$K_LIM`, LNKNOTFR, PICIMG, `IHD$L_IDENT` and the
  link flags' top byte from GSMATCH=, or the link time and EQUAL without
  it (SHR1A). The first transfer address is `SYS$IMGSTA` with traceback
  and 0 without. A module's transfer address in a shareable link is
  passed over (**unconfirmed**: no probe image had one).
- **Sections:** base 0, no user stack ISD, and the shareable half of
  Table 6-1: the section key adds NOSHR and PIC below VEC, so the order
  is SHRFXD, PRVFXD, SHRPIC, PRVPIC, each NOWRT/NOEXE, WRT, EXE, WRT+EXE
  (VSI's manual, Table 7.3; SHR1 and SHR4). Only a private writable
  section is CRF. **No section gets LASTCLU** in a shareable image (none
  of the probe's had it), and no demand-zero run is split off; a whole
  writable section nothing is stored in still becomes DZRO, as LIBRTL's
  is (**unconfirmed** for these images, which have none). The VECTOR bit
  Table 7.3 gives VEC sections isn't set: no probe image had one.
- **The fixup section** (pulled forward from subtask 7, since every
  probe image but the based SHR3 has one): it's there only when the image
  refers to another shareable image or has a `.ADDRESS`, typed PRVFXD,
  and its IAF has `IAF$V_SHR`. **IAF$V_SHR is in the word at IAF+0x0A**,
  after the fixed part's size, a word at +8, not at +0x24 as
  `internal/vmsimage` had guessed; `vmsimage.IAFFlags` is corrected, so
  ANALYZE/IMAGE reads it right now. A `.ADDRESS` of the image's own data
  goes in a `.ADDRESS` list for image 0, before the other images' lists
  (the order is **unconfirmed**: SHR1 has only image 0's), relative to 0.
- **The DST has fixup records** (`docs/DEBUG-RECORDS.md`'s 18.1, which
  the probe confirms): after the last module's records, from the next
  block, a `DST$K_FIXUP` record whose group is a longword count, the
  counted name `DEFAULT_CLUSTER`, and the DST offset of each longword
  that holds an image address (every relocatable longword pass 2 stores
  in the DST). The offsets are in the order of the clusters the
  addresses are in, then the DST's: SHR4's SHR1_RO, collected into
  FIRST, came before the default cluster's routines. Why the group is
  named DEFAULT_CLUSTER, when some of its addresses are in another
  cluster, and how a list too long for one record is split, are
  **unconfirmed**.
- **Options:** `ISD_MAX=` in a shareable link is `%LINK-I-OPTIGNSHR,
  option ignored for shareable image`, shown after its line
  (`OptionsParser.Shareable`, `Notices`), and `DZRO_MIN=` is taken
  silently. Neither changes the image.
- **LINK's status** is VMS's now, for executables too: %X10000001 for a
  clean link, %X10648268 for one with warnings (USRTFR, NUDFSYMS,
  USEUNDEF), the inhibit bit set. A shareable image has no USRTFR, in
  the log or the map.

Refused until subtask 7, with a message: a based shareable image (BASE=,
or a based CLUSTER=), a position-dependent (NOPIC) psect, and a
`.ADDRESS` in a read-only section (SHR6). The map still describes the
image as an executable's (subtask 8). `/EXECUTABLE=` and `/SHAREABLE=`
naming the image both is refused (VMS's message there is
**unconfirmed**).

### 2026-10-19 — Subtask 6: the global symbol table

A shareable image's global symbol table lists its universal symbols
(`internal/link/universal.go`). `link.Options.Universals` is
`UNIVERSAL=`'s list, which the console's LINK now passes. Pass 2's
`OPR_REDEF` makes a symbol universal, with the `.TRANSFER`'s location
as its value. The table is written by the code that writes a `/DEBUG`
image's (`globalSymbolTable`), after the DST and its fixup records. The
probe's SHR1, SHR1A, SHR4, SHR5, and E1 now link byte for byte as real
LINK's, GST included (`TestLinkShareableImages`), padded to a whole
block as Phase 29's Decision 7 has it. `ReadShareableImage` reads
govax's images as it reads VMS's (`TestReadGovaxShareableImage`).

What the probe's tables settled:

- **The entries.** Each is DEF and UNI, and REL when its psect's cluster
  has no base of its own (SHR3's, based, have no REL; govax can't link
  SHR3 until subtask 7). An entry point is an `EPM` with the routine's
  own mask. A `.TRANSFER`'d symbol's value is its vector entry, but its
  mask is still the routine's (`SHR1_ADD`'s 0, not the vector's
  `^M<R4,R5>`). A `.TRANSFER`'d JSB routine is a `SYM` (`SHR1_JSB`).
- **A universal symbol nothing defines** (E1's `UNIVERSAL=NOSUCH`) is
  one of the link's undefined symbols: `NUDFSYMS` and `UDFSYM`. It isn't
  listed, so E1's table has no symbol GSD record at all. **Its end of
  module record's severity is 1**, the link's warning (`03 01`). govax
  sets it for any link that warned, executables linked `/DEBUG` too
  (**unconfirmed** there).
- **The order** (**unconfirmed**; `universalSymbols`): absolute
  symbols first, then the psects in address order. Within a psect come
  the entry points, then the other symbols, each in descending order of
  value, which in these sources is the reverse of the modules'
  definitions. A `.TRANSFER`'d symbol is placed by its routine, not by
  its vector. That fits all seven of the probe's tables (the tests
  link four; SHR2's, SHR3's, and SHR6's are checked by hand until
  subtasks 7 and 9 can link them), and Phase 29's
  TRACE too (`LEVEL`, then DATA's `GLOBDATA`, then `$CODE`'s `SECOND`,
  `FIRST`, `TRACE`). FORTH's `SYS$` names in name order don't fit it, so
  executables keep Phase 29's rule. The sources can't tell descending
  value from these two alternatives: entry points in reverse definition
  order with the others in definition order, or a hash of the names
  (no simple one fits). Equal values of one kind, and several modules'
  symbols in one psect, are ordered by the reverse of their definitions.
- **An executable** still takes `UNIVERSAL=` and `.TRANSFER` without
  effect (`TestUniversalInExecutable`: E2 with `UNIVERSAL=` added is
  still real LINK's image), and gets no table unless linked `/DEBUG`.

**`/SYMBOL_TABLE[=file]`** (and `govax link --symbol-table FILE`)
writes the image's table as an object module file, by default the first
input file's name with the type STB. It holds the table record for
record: govax's SHR1.STB is real LINK's byte for byte
(`TestLinkSymbolTableFile`), and `TestLinkShareable` checks it through
DCL. For an executable, the file is the table the image would have
linked `/DEBUG`. What real LINK writes there is **unconfirmed**.

The map's universal symbols (`U`, and the vector's two values) are still
subtask 8's.

### 2026-10-19 — Subtask 7: fixups and bases

`LINK/SHAREABLE` refuses nothing now. The probe's SHR2, SHR3, SHR3P,
and SHR6 link byte for byte as real LINK's, from real MACRO's objects
and from govax's, global symbol tables included
(`TestLinkShareableImages`, which now holds all nine of the probe's
shareable images). `TestReadGovaxShareableImage` reads all seven
govax-made images as it reads VMS's. `TestLinkShareableUsingAnother`
links SHR2 through DCL against VMS's SHR1.EXE, named in SYS$INPUT
options.

What the images settled:

- **A `.ADDRESS` in a read-only section** (SHR6). The section becomes
  CRF+WRT and keeps its type (SHRPIC) and its place in Table 6-1's
  order. The change-protection list gets an entry after the fixup
  section's: the section's address and pages, `PRT$C_UR`
  (`shareable.go`'s `referSelf`, `section.fixedUp`,
  `linker.readOnlyFixups`). An entry covers the whole section; whether
  real LINK gives a multi-page section one entry or one per page is
  **unconfirmed**.
- **NOPIC psects** need nothing new. They take the types SHRFXD and
  PRVFXD (subtask 5's `sectionType`), and PICIMG stays set unless the
  image is based (SHR3P).
- **A based image** (SHR3) has its sections at their addresses and
  absolute universal symbols (subtask 6's rule). It records no
  image-relative `.ADDRESS` or DST fixups (`basedAt`), so it has no
  fixup section. Whether a `.ADDRESS` from a based cluster to an
  unbased one (only possible with a based CLUSTER=) needs a fixup is
  **unconfirmed**. govax goes by the target's cluster.
- **A shareable image's own images** (SHR2). SHR2 uses SHR1, and its
  shareable image list and global section ISDs also name LIBRTL, which
  SHR1 uses and SHR2 doesn't refer to. `ReadShareableImage` now reads an
  image's shareable image list (`SharedImage.Uses`), and
  `addDependencies` gives each image a cluster right after its own,
  unless it has one already. That is also MAIN1's and MAIN1L's cluster
  order (SHR1, LIBRTL, then the default cluster or after it), which
  subtask 9 will check. An image that is in the list only as another's
  dependency gets a list entry with no cells.
- **The image ident** of an image without a transfer address is the
  first module's, and a shareable image an options file names counts as
  a module, read first. So SHR2's ident is SHR1's global symbol table's
  module version, "V1.0", not its own "V2.0" (`firstImageIdent`,
  `SharedImage.Ident`).
- **The order of G^ cells**, open since Phase 29, has a rule that fits
  every fixture with two cells or more (`orderCells`):
  - When an options file names the image, the cells are in descending
    name order: MAIN1, MAIN1A, MAIN1N, MAIN2, MAIN3P, MAIN4, E8, and
    SHR2.
  - When the image was found in a library, the cells are in descending
    order of the symbols' places in the image's own global symbol
    table. MAIN1L and MAIN1U list SHR1's cells in the reverse of SHR1's
    table. LIBRTL's table gives FAILSIG's, FSIGNOTB's, PI's, PG66's,
    CELLS's, CELLS2's, and DBGCMD's orders.

  `Definition.Order` carries a symbol's place, read from the image's
  table. FAILSIG, FSIGNOTB, and PI, known differences since Phase 29,
  now match real LINK's images when linked with VMS's own libraries
  (`TestLinkProbeImagesMatchRealLINK`, `TestLinkPiMatchesRealLINK`).
  govax's own LIBRTL table (`vmsdef.ImageSymbols`) keeps the places
  too (`ImageSymbol.Order`). The author regenerated it from LIBRTL.EXE
  (`go run ./internal/vmsdef/gen -image .../librtl.exe`; the generator
  fills in places without `-replace`), so the RLFSIG and RLFSIGNT round
  images, linked with govax's tables, match real LINK's as well, and
  `TestCapturedLIBRTLMatchesImage` checks every place. LIBRTL2's and
  CRFSHR's symbols have their places too, merged from the images again
  (copied back from the VAX for it); CRFSHR's absolute `CRF$K_`
  constants need none. Why real LINK's two orders
  differ is **unconfirmed**. It may come from how it enters an options
  file's image's symbols (all at once) and a library's (as each is
  looked up).

Left for later subtasks:

- Linking against a based shareable image (`placeClusters` still
  refuses one, subtask 9). `ReadShareableImage` gives SHR3 a page count
  of 0, since it has no SHRPIC section.
- The map's account of these images (subtask 8).
- Activating them: the second change-protection entry, the
  self-relative `.ADDRESS` list, and the dependencies (subtask 11).

### 2026-10-19 — Subtask 8: maps and ANALYZE/IMAGE

LINK writes `/FULL` maps now (`MapOptions.Full`, the console's
`LINK/FULL`, `govax link --full`; with `/BRIEF` it is DCL's CONFLICT,
since the Linker manual calls the two incompatible, though VMS's own
message is **unconfirmed**). The full map's sections are
`internal/link/mapfull.go`. All eight of the probe's shareable images'
maps match real LINK's line for line up to the run statistics, when
linked with VMS's own libraries (`TestMapShareableImagesFull`), and so
do EXE1's, MAIN3P's, and MAIN6's (`TestMapExecutablesFull`). With
govax's own tables, the lines differ only where the tables lack
LIBRTL's ident and file, STARLET's SYS$P1_VECTOR, and the counts that
follow (`TestMapShareableImagesGovaxTables`, which runs without VMS's
files).

What the maps settled:

- **A full map puts each section on a page of its own.** The object
  module synopsis lists every module and each shareable image used,
  cluster by cluster: an image an options file names, and those it
  uses, in its place in the cluster list; an image a library supplied
  after the library modules (`mapModules`). An image is a module of 0
  bytes with its global symbol table's ident, creation time, and
  creator (`SharedImage.File`, `Created`, `Creator`). An image the link
  uses only through another (SHR2's LIBRTL) is read all the same, as
  its map lists it (`ImageLibrarySource.Image` opens one the library
  lists).
- **The module relocatable reference synopsis** follows on the first
  page, when any module has something to list: the module's `.ADDRESS`
  longwords that hold offsets in shareable images, and in a shareable
  image those that hold its own addresses and its traceback records'
  relocated longwords (`module.relocations`). SHR1's 10 are 3
  `.ADDRESS`/`.ASCID` longwords and 7 traceback ones; SHR1A,
  `/NOTRACEBACK`, has 3. The Linker manual's "number of .ADDRESS
  directives" leaves out the traceback longwords. How more than three
  entries fill the lines is **unconfirmed** (govax fills across).
- **The image section synopsis**: each cluster's sections, after a
  blank line, its name on the first (the fixup section with the cluster
  at the highest address, the user stack with the default cluster),
  with the type, pages, address (`P-` protected, `-R` relocatable: a
  shareable image's unbased sections and those of the images used),
  VBN, PFC, and protection and paging (`READ ONLY`, `COPY ON REF`,
  `DEMAND ZERO`, `FIXUP VECTORS`; a read-only section the activator
  fixes up is `NON-SHAREABLE ADDRESS DATA` in its own map, SHR6, but
  `COPY ON REF` in MAIN6's). A shareable image lists each of its own
  sections as global section `NAME_00n` with its match and idents
  (`SharedSection.Fixups` marks its fixup section). A cluster name
  longer than 15 characters goes on a line of its own (**unconfirmed**).
- **The program section synopsis** in a full map has a blank line
  before each psect, and the library modules' contributions too.
- **Symbols by name** adds what libraries and shareable images define
  and the image refers to: a symbol in a shareable image at its cell in
  the fixup section, `-RX`, or at its address, unflagged, if the link
  placed the image (EXE1's LIB$PUT_OUTPUT). Universal symbols are `U`.
  The key follows the symbols by value.
- **Symbols by value**: each value and its symbols, by name, each its
  flags and a dash right-aligned in four columns and its name as wide
  as the map's longest. A `.TRANSFER`'d symbol is `R-` without the U,
  its value there its routine's. How many fit on a line is
  **unconfirmed** (no fixture has more than two at a value), and so is
  whether undefined symbols are listed (govax lists them, `*`).
- **The image synopsis**: a shareable image has no stack (0 pages);
  `Image type:  PIC, SHAREABLE. Global Section Match=..., Ident,
  Major=n, Minor=n` (`NOPIC` if based; `gsMatch`, which the header now
  shares); the address fixups count the image's own `.ADDRESS`
  longwords too; a full map's estimated length counts the psects and
  symbols twice (`7 + modules/4 + 2*(psects + symbols/16)`, which fits
  every probe map).
- Two counts follow rules fitted to the eight images, both
  **unconfirmed**: the image binary block count leaves out the pages
  of the sections that hold the image's own `.ADDRESS` longwords
  (SHR1's 13 blocks are 4, SHR6's 3 are 2, SHR2's 3 stay 3), and the
  global symbol count leaves out the defined symbols `UNIVERSAL=`
  names, though not those a `.TRANSFER` makes universal (SHR1A's seven
  leave 305, LIBRTL's). STARLET.OLB counts among the files only when a
  module came from it (SHR1A's 3 files).

ANALYZE/IMAGE (`internal/anl`) now matches every `.ANI` of the probe
that has its image (`testdata/link68/vax` joins `TestImagePages`):

- A shareable image's header shows its global section ident and match
  control under the image type.
- A shareable image's global symbol table follows the ISDs on a page
  of its own, `SHAREABLE IMAGE - GLOBAL SYMBOL TABLE`, its records
  described as ANALYZE/OBJECT describes them (`vmsimage.Image.GST`,
  read by the header's record count, as govax pads the table's block).
  A module header's patch time is shown only when it differs from the
  creation time: real LINK writes the table's patch time as its
  creation time, which ANALYZE leaves out (**unconfirmed** otherwise).
- `.ADDRESS` fixups for image 0, the image itself, are a shareable
  image's, not an error.
- A based image without a fixup section shows the section's title and
  "There is no fixup section in this image." (SHR3).
- The blank line between two ISDs is written even on a full page
  (EXE1's page 3 has 61 lines).
- The change-protection address printed twice for MAIN3 and EXE1
  needed nothing new.

Left for subtask 9: MAIN1's, MAIN2's, and MAIN4's maps, where real
LINK put LIBRTL right after the shareable image that uses it, though the
program refers to LIBRTL itself (govax's cluster for it comes from
IMAGELIB, at the end), and MAIN3's (a based shareable image, which
`placeClusters` still refuses).

### 2026-10-19 — Subtask 9: linking against govax's shareable images

Every program of the probe that uses a shareable image links byte for
byte as real LINK's: MAIN1, MAIN1A, MAIN1N, MAIN2, MAIN3, MAIN3P,
MAIN4, and MAIN6, against VMS's shareable images and against the ones
govax's LINK makes, from real MACRO's objects and from govax's
(`TestLinkProgramsAgainstShareableImages`, `programs_test.go`). Their
`/FULL` maps match real LINK's line for line up to the run statistics
(`TestMapExecutablesFull`, which now holds all eight with EXE1).
`TestLinkAgainstGovaxShareableImages` runs the cycle through DCL on a
volume, all govax's: MACRO, LINK/SHAREABLE of SHR1 and of SHR2 against
it, then MAIN2 against SHR2, with SHR1 found through its logical name;
MAIN2's header and shareable image list are VMS's but for its name,
time, ident, and linker ID.

What the images settled:

- **A global section ISD is the image's first section's**: its pages
  (SHR3P's first section is SHRFXD, so `ReadShareableImage` no longer
  looks only for an SHRPIC one), and its type and attributes too, with
  GBL and the match control added. MAIN3P's ISD for SHR3P is
  `ISD$K_SHRFXD`, and MAIN6's for SHR6 is CRF+WRT, since SHR6's first
  section is one the activator fixes up (`globalSectionISD`). An image
  whose sections aren't known is taken to start with an SHRPIC section,
  as LIBRTL does.
- **The images a shareable image uses come right after it** in the
  cluster list, even when the program refers to them itself: MAIN1,
  which calls LIB$PUT_OUTPUT, has SHR1, LIBRTL, then the default
  cluster, and MAIN2 SHR2, SHR1, LIBRTL. `addImageClusters` now adds
  the options files' images' dependencies before the clusters of the
  images found in libraries (the maps' module and image section
  synopses showed it; the images themselves were already right).
- **An image used only through another is read from its file.** MAIN2's
  ISD for SHR1 has SHR1's ident, match control, and first section's
  pages, and its map lists `DUA1:[000000]SHR1.EXE;8`: real LINK opened
  SHR1.EXE through the logical name the probe had defined. A shareable
  image list holds only names, so the file is the only place these
  come from. `link.ImageFileSource` is a source that describes images
  by opening their files, after every other source, and defines no
  symbols. DCL's LINK opens each by RMS's parse of the image's name with
  the default `SYS$SHARE:.EXE`, then as IMAGELIB's images are found
  (`dclcmd.usedImages`). Subtask 10's locator will take its place.
- **A based shareable image** (MAIN3's SHR3) is placed in the first
  allocation pass, each section where it was linked
  (`placeBasedImage`), and gets a global section ISD per section, BASED,
  with LASTCLU when it is the cluster at the highest address. Then the
  default cluster's sections and the user stack have no LASTCLU (MAIN3's
  don't; MAIN1's do). PICIMG is clear. Its symbols, absolute in its
  table, are now read as the image plus their addresses, its base being
  0, so the link resolves them as it does those of an image it placed
  (EXE1's LIBRTL): no cells, the image counted as used, and the map
  showing them at their addresses, unflagged, with the global symbol
  count right (310).
- **E5**, an executable based on top of SHR3: `%LINK-W-BASSHRBEL`, then
  `CONFMEM` for the default cluster, and no image, with real LINK's
  status (`TestLinkBasedOverBasedImage`). BASSHRBEL is given when the
  image's base is at or below the executable's BASE=; whether it is
  given for one below the base that doesn't overlap is **unconfirmed**.
  The FatalError of a CONFMEM now carries the link's earlier messages.
- MAIN1N was linked while SHR1 was EQUAL,1,1001 (the probe's `NEVER`
  relink failed), and the tests link it against that SHR1.

