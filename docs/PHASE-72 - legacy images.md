# Phase 72 — legacy images

**Status:** in progress. Planned 2026-10-24; the author answered Part
B's questions the same day. Subtasks 1 (reading a V1 image's header,
and RUN/DEBUG of one), 2 (the probe, `testdata/legacy72`, both rounds
run on VMS 7.1; Part A, "The probe's answers"), 3 (the old system
service vector), 4 (global section ISDs, VMSRTL mapped), and 5
(FORRTL's I/O: both games play at the terminal), and 6 (OTS$'s
floating conversions, moves, and dynamic strings, and FORRTL's floating
edit descriptors, checked against VMS 7.1 by `testdata/ots72`) are
done.

## Goal

govax runs an image linked by VMS V1's linker as VMS V7 does. The
image that started it is DUNGEON.EXE (`DUA0:[WORK.DUNGEON]` on the
author's work volume), a FORTRAN program linked on 7-DEC-1978. It runs
on the author's simh VAX. On govax it failed at once:

    %SYSTEM-F-OPCDEC, opcode reserved to Digital fault at PC=8000016A, PSL=03C00000

      Improperly handled condition, image exit forced.

RUN/DEBUG didn't help either: the image ran without the debugger, so
there was no way to start it under the debugger at its transfer
address.

1. **The header.** Read format 02.01's header blocks, so RUN,
   ANALYZE/IMAGE, and the debugger see the image's real transfer
   addresses, debug symbol table, and identification.
2. **The image startup.** Start such an image the way its linker meant:
   its first transfer address is a stub that jumps to SYS$IMGSTA's
   entry in the old system service vector.
3. **The old system service vector.** Give the image VMS V1's vector in
   S0, at 0x80000000, which is how it calls every system service.
4. **Global section ISDs.** Map the shareable image its global section
   ISDs name (VMSRTL_001 to VMSRTL_003) at the addresses they give.

Out of scope: formats 02.02 to 02.04 (nothing on hand shows them; they
are read as 02.05), and DUNGEON_V552.EXE's own failure (below).

## Where things stand (2026-10-24, for picking the phase up again)

- **Done**: subtasks 1 to 5 (see the progress log). RUN DUNGEON and
  RUN DUNGEON_V552 play at the terminal as VMS 7.1 does: a pty run of
  `look`, `score`, and `quit` matches `dungeon-term.log` line for line.
  Subtask 6: the OTS$ routines, and FORRTL's F, E, D, and G edit
  descriptors through `OTS$$CVT_x_T_R8`, matching VMS 7.1's run of
  `testdata/ots72`. **Next**: subtask 7, the debugger on FORTRAN images.
  What follows in this item is subtask 4's plan, kept for the record.
- **Subtask 4** (done), Decision 4. When an image's ISD has ISD$V_GBL
  and a name `NAME_nnn` and the image is a V1 image (`ICB`/`vmsimage`'s
  `Legacy`), activate `NAME` (here VMSRTL: `SYS$SHARE:VMSRTL.EXE` by
  `rms.ImageLocator`/`machine.ImageLocator`, Phase 68) whole at the
  first such ISD's address (^X1A600 for DUNGEON), and pass over the
  image's other ISDs naming it (VMSRTL_002, _003). VMSRTL.EXE is VMS
  7.1's PIC jacket (Part A, "VMSRTL.EXE"), with its own fixup section
  and shareable image list (BASRTL, COBRTL, CMA$TIS_SHR, FORRTL,
  LIBRTL, MTHRTL), so once it's placed, Phase 68's activation (fixups,
  the images it uses, activated from its list's end: answer 2) should
  do the rest. Both loaders need it: the copying one
  (`machine/image.go`, paging off) and the mapping one
  (`machine/imagemap.go`, paging on); `machine/activate.go` places
  shareable images (`imageBase`). Where the current loaders skip a GBL
  ISD of a main image with no fixup section is the place to start.
  VMS's placement to match is answer 1 and 2's SHOW IMAGE.
- **Then**: subtask 7 (the debugger on FORTRAN images: decimal radix,
  SHOW MODULE's language column, EXAMINE's next location after
  `EXAMINE/INSTRUCTION .PC`; answer 6), and an end-to-end check against
  `testdata/legacy72/vax/dungeon-term.log` (the eight moves; the game
  reads and writes FORTRAN unit 5 at the terminal). Side findings to
  file or fix: DIRECTORY with a list of specs, the debugger's
  DBG$INPUT, the FOR facility's message texts.
- **The author's setup**: DUNGEON.EXE, DUNGEON_V552.EXE, DINDX.DAT, and
  DTEXT.DAT are in `DUA0:[WORK.DUNGEON]` on the work volume
  (`~/work.dsk`, the default profile). The FORTRAN RTLs and VMSRTL.EXE
  are in `DUA1:[XFER]` on `testdata/disks/xfer.dsk`, which the work
  volume's `SYS$STARTUP.COM` mounts (with SYS$SHARE, FORRTL, FORRTL2,
  and MTHRTL logical names; `startup.com` at the repository root, not
  committed, is a copy). So run govax from the repository root, and
  use DUA2 for exchange volumes. Copy an image to the host with
  COPY/BINARY.

## Part A — the survey (2026-10-24)

### Sources

- **The image itself** (DUNGEON.EXE, 210 blocks, fixed 512-byte
  records). Every layout below was read off it. No DIGITAL manual on
  hand describes format 02.01.
- *VAX/VMS Internals and Data Structures* (usable in full), 21.1.3.2
  and 21.1.3.3: SYS$IMGSTA is the image's first transfer address when
  it's linked with traceback, and it calls the next one.
- `docs/DEBUG-RECORDS.md`, 4.5: language code 1 is FORTRAN.
- **The author's simh VAX** runs DUNGEON.EXE, so VMS V7 still supports
  both the old vector and VMSRTL's global section ISDs.

### The image

The fixed header is 0x28 bytes, not 02.05's 0x30. The fields it shares
with 02.05 sit at the same offsets; IHD$L_SYSVER and IHD$L_IAFVA aren't
there (the activation block starts where they would be).

| Offset | Value | Field |
| --- | --- | --- |
| 0x00 | 0x007C | the ISDs' offset |
| 0x02 | 0x0028 | the activation block's offset |
| 0x04 | 0x0038 | the symbol table block's offset |
| 0x06 | 0x0044 | the identification block's offset |
| 0x08 | 0 | no patch block |
| 0x0C | "0201" | major and minor id |
| 0x10, 0x11 | 1, 1 | one header block; IHD$K_EXE |
| 0x20 | 0x02000000 | link flags? Bit 25 isn't a 02.05 flag (unconfirmed) |
| 0x24 | 0 | ident |

- **Activation block** (0x28, 0x10 bytes): transfer addresses
  0x0001A433 and 0x0000E1B0, then the zero that ends them. There's no
  shareable image initialization list.
- **Symbol table block** (0x38, 0x0C bytes): DST at VBN 193, 18 blocks;
  no GST. There's no debug module table and no 32-bit sizes. Before
  this phase govax read 0x1C bytes here, took the image name's text as
  a debug module table's VBN (1314210823), and failed to read the DST.
- **Identification block** (0x44, 56 bytes): the name in a 16-byte
  counted field ("DUNGEON"; 02.05's is 40 bytes), the file ident in 16
  ("01"), the link time (7-DEC-1978 14:30:35.88), the linker's ident
  in 16 ("01.20").
- **ISDs** (0x7C): the same layout as 02.05's. There are nine private
  ones: code, data, demand-zero, and the user stack (type 0xFD, 20
  pages at 0x7FFFD800). One page at 0x1A400 holds the startup stub.
  Then three global section ISDs (size 0x24, ident 0x010003E8, match
  control 2):

  | Name | Pages | Address | Type, flags |
  | --- | --- | --- | --- |
  | VMSRTL_001 | 4 | 0x1A600 | 3, GBL LASTCLU |
  | VMSRTL_002 | 49 | 0x1AE00 | 3, GBL LASTCLU |
  | VMSRTL_003 | 2 | 0x21000 | 4, GBL CRF WRT LASTCLU |

  There's no fixup section and no shareable image list. The image
  calls VMSRTL at the absolute addresses its linker mapped it at.
- **The DST** uses the record types 02.05's do (module begin and end
  0xBC/0xBD, routine begin and end 0xBE/0xBF, line numbers 0xB9) with
  language 1, FORTRAN. It has no psect records, so a module is placed
  only by its routines.
- **The startup stub** at 0x1A433: `.WORD 0` then `JMP @#8000016A`,
  that is, past the entry mask of 0x80000168. In 02.05, SYS$IMGSTA is
  at 0x7FFEDF68 in the P1 vector, whose base is 0x7FFEDE00: offset
  0x168, the same as 0x80000168's from 0x80000000. So V1's system
  service vector was in S0 at 0x80000000, and V4's P1 vector kept its
  layout (inferred from this one entry; unconfirmed for the rest).

### What govax has

- `vmsimage.ReadImage` assumed 02.05's layout. It rejected the
  activation block (its offset, 0x28, was "inside the fixed header"),
  read the fixed header's last two longwords from the activation block
  (an IAFVA of 0xE1B0, so ANALYZE/IMAGE reported a garbage fixup
  section), and misread the symbol table and identification blocks.
  ANALYZE/IMAGE called the format an error.
- The loaders read the transfer addresses straight from the header, so
  RUN got 0x1A433 right. `mainTransferAddress` took it as the main
  routine (it skips only P1 and S0 addresses), and the driver called
  the stub, which jumped into VMINIT's S0 layout at 0x8000016A: the
  OPCDEC.
- With no debug symbol table, `runsUnderDebugger` runs the image
  without the debugger whatever RUN asks, so RUN/DEBUG did nothing.
- S0 from 0x80000000 is VMINIT's (SCB, stacks, scratch, string pool,
  shim stubs). Nothing is at the old vector's addresses.
- Global section ISDs: the loaders map a shareable image found through
  the fixup section's shareable image list (Phase 68). An image without
  one gets nothing at its global sections' addresses.

### VMSRTL.EXE (2026-10-24)

The author put VMS 7.1's `VMSRTL.EXE` in `DUA1:[XFER]`. It isn't V1's
VMSRTL: it is a 02.05 PIC shareable image ("VMS V6-001", linked
24-NOV-1996 by linker V11-38, GSMATCH 1,0xC1C MATLEQ, 31 blocks), with
a 16-page shareable PIC section at 0 (the transfer vector, by its size),
a 13-page fixup section at ^X2000, and global section ISDs for the
images it forwards to: BASRTL, COBRTL, CMA$TIS_SHR, FORRTL, LIBRTL,
and MTHRTL. Its GST has no symbols. So it is a jacket, and DUNGEON's
three ISDs (4, 49, and 2 pages) can't each be one of its sections.

What DUNGEON uses of it, found by scanning the image's code for
PC-relative operands in VMSRTL's ranges: only transfer vector entries
from ^X1A600 to ^X1A830 (offsets 0 to ^X230, all in VMSRTL_001's first
page), and perhaps one deferred operand at ^X1AA33. Nothing in the
VMSRTL_002 or _003 ranges. From the old system vector it uses only
SYS$IMGSTA (^X80000168, the stub) and SYS$NUMTIM (^X800001B8).

A likely reading (unconfirmed until the probe): VMS maps VMSRTL.EXE at
the first VMSRTL ISD's address, so the old vector offsets land on the
new image's vector, and ignores the other two. The probe's SHOW IMAGE
and EXAMINEs settle it.

### The probe's answers (round 1, 2026-10-24)

VMS V7.1 on a VAX 8600 (`testdata/legacy72/vax/`):

1. **VMSRTL is mapped whole at the first VMSRTL ISD's address.** The
   debugger's SHOW IMAGE shows `VMSRTL 0001A600 0001DFFF`: all 29
   pages, its vector and its fixup section, at VMSRTL_001's address.
   VMSRTL_002's and _003's ranges aren't VMSRTL's: MTHRTL starts at
   ^X1E000, just past it. So VMS maps the image a V1 global section
   ISD names once, at the first such ISD, and ignores the others.
2. **The images VMSRTL uses follow it**, in the reverse of its ISD
   list's order: MTHRTL ^X1E000, LIBRTL ^X48400, FORRTL ^X6B200,
   CMA$TIS_SHR ^X74200, COBRTL ^X76000, BASRTL ^X0AA000 (its list is
   BASRTL, COBRTL, CMA$TIS_SHR, FORRTL, LIBRTL, MTHRTL). That's Phase
   68's rule for an image's shareable image list. The debugger's own
   images come last.
3. **The game runs on VMS**: DUNGEON gets through INIT into GAME, as
   DUNGEON_V552 does. In a procedure both stop at their first message
   with `%FOR-F-ERRDURWRI` on unit 5, which is SYS$INPUT there. govax's
   DUNGEON_V552 stops with the same condition (its `Message number
   0018813C`), so that side finding was the procedure's input, not a
   govax bug. govax lacks the FOR facility's message texts. (Corrected
   2026-10-24, subtask 5: `0018813C` is FOR-F-ERRDURREA, govax's RMS
   lacking keyed reads of DTEXT.DAT; VMS's ERRDURWRI in a procedure is
   the input entry's GET-only access, which govax now matches, its
   `00188134`.)
4. **ANALYZE/IMAGE of a 02.01 image** (`dungeon.ani`): govax's layout,
   but with no "debug module/psect table" line. The image name shows as
   `"DUN"` and four NULs, flagged `Symbol contains invalid
   character(s).` (the report's one error), and the linker ident as
   `"01."` and two NULs. VMS apparently converts the V1 header and keeps
   only the first longword of those counted strings. The file ident
   ("01") and the link time are right.
5. **INSTALL**: VMSRTL is installed `Open Hdr Shar Lnkbl` with one
   global section. `INSTALL LIST` has no `/OUTPUT` (round 2).
6. **The debugger on a V1 FORTRAN image** (`dun72.dlg`):
   `%DEBUG-I-INITIAL, Language: FORTRAN, Module: DUNGEO`; SHOW LANGUAGE
   `language: FORTRAN`; the radix is decimal for input and output
   (`DUNGEO+2: MOVAL L^11720,R11`); SHOW CALLS and the first STEP
   (`stepped to DUNGEO\%LINE 218`, line 218, rel PC 9) as govax's.
   SHOW MODULE has a language column (FORTRAN, and MACRO for
   FOR$IDATE, MTH$MOD, OTS$LINKAGE, and RND), headed `module name
   symbols language size`, and ends `total modules: 93.`. EXAMINE with
   no address goes on from the last instruction examined. The second
   STEP stopped on the unit 5 write, with a SHOW CALLS whose FORRTL
   frames are named `SHARE$FORRTL`.
7. Every `%X` address was refused (`MISOPEMIS`): the S0 vector and
   VMSRTL's transfer vector are round 2's.

### The probe's answers (round 2, 2026-10-24)

`legacy72b.log`, `dun72b.dlg`, `global.lis`, and `dungeon-term.log`:

8. **The old S0 vector is still there on VMS 7.1, with the P1
   vector's layout.** Its entries are the system's own
   stubs: offset 0 (SYS$QIOW's in P1) is `.WORD ^XFFC`, `CHMK #^X32`,
   `BLBC R0, ...`; ^X1B8 (SYS$NUMTIM) `.WORD ^XFC`, `CHME #^XB`, `RET`;
   ^X168 (SYS$IMGSTA) `.WORD 0`, `JMP @#81929BC6` (into the executive);
   ^X800 `.WORD 0`, `JMP @#80003C50`; ^X400 and ^X918 change-mode
   stubs. ^X920 holds no entry (^X898F) and ^XA00 is zero. So the old
   vector and the P1 vector have the same entries at the same offsets,
   which confirms Part A's inference. (The probe took ^X918 for the P1
   vector's last entry; it's SYS$SYNCH_INT's at ^XB10. So the old vector
   ends somewhere past ^X918, before the P1 vector does: where is
   unconfirmed.)
   (The P1 entries examined showed `JMP @#SHARE$DBGSSISHR+...`: the
   debugger intercepts services there, so they say nothing of the
   undebugged P1 vector.)
9. **VMSRTL's transfer vector** is an ordinary 02.05 shareable image's:
   8-byte entries, `.WORD ^XFFC` and `JMP @L^cell(PC)` through a cell in
   its fixup section (offset 0 through ^X28D0, ^X178 through ^X298C,
   ...). The entry at ^X230 has no mask (`JMP @L^...`, a JSB entry), as
   does ^X430. So once VMSRTL is mapped at ^X1A600, Phase 68's
   activation, with its fixups, makes the vector work.
10. **The old VMSRTL_002 and _003 ranges** are other images': ^X1AE00
    is SHARE$VMSRTL+800, ^X21000 is SHARE$MTHRTL+3000. Answer 1 holds.
11. **INSTALL's global sections**: VMSRTL is installed with one section,
    `INS$81B09BA0_001`, ident ^X01000C1C, 16 pages (its transfer
    vector section only: the fixup section is private). The installed
    images' sections are named `INS$address_001`, not `VMSRTL_001`, so
    VMS finds a V1 ISD's image by the name before `_nnn`, not by a
    global section of that name.
12. **The games at a terminal** (`dungeon-term.log`): DUNGEON and
    DUNGEON_V552 both play the probe's eight moves with the same
    output, from `Welcome to Dungeon.` to `Do you wish to leave the
    game?`. That's the phase's end-to-end target.

### OTS$ in LIBRTL (2026-10-24, subtask 6)

VMS 7.1's LIBRTL has 54 OTS$ entries (`vmsdef.ImageSymbols`; the
complex, power, and other math routines the OTS$ manual lists are
MTHRTL's). By what describes them:

| Entries | Described by | govax |
| --- | --- | --- |
| CVT_L_TI, _TU, _TB, _TO, _TZ, _TL and their inverses | the 5.0 OTS$ manual | subtask 5 |
| CNVOUT, _G, _H; CVT_T_D, _F, _G, _H | the manual | subtask 6 (`otsfloat.go`) |
| MOVE3, MOVE5, and their `_R5` JSB entries | the manual (the JSB entries' outputs only) | subtask 6 (`otsstring.go`) |
| SCOPY_DXDX, SCOPY_R_DX, SGET1_DD, SFREE1_DD, SFREEN_DD, and their JSB entries | the manual | subtask 6 (`otsstring.go`) |
| DIV_PK_LONG, DIV_PK_SHORT (and the alias DIV_PKSHORT) | the manual | not yet: no caller on hand |
| CVTxP_R9, CVTPx_R9, CVTRxP_R9 (packed to and from floating, x D, F, G, H) | nothing | not yet: no caller on hand |
| $$CVT_D_T_R8, $$CVT_F_T_R8, $$CVT_G_T_R8, $$CVT_H_T_R8 | nothing | the study below, then subtask 6's second part |
| $$CVT_MUL, $$RET_A_CVT_TAB_R1 | nothing | not yet: FORRTL's conversions didn't call them under govax |

**FORRTL's floating output.** VMS 7.1's FORRTL exports the 2.0 Run-Time
Library manual's `FOR$CVT_x_Ty` (x D, F, G, H; y D, E, F, G: value,
string, digits in the fraction, scale factor, digits in the integer,
digits in the exponent, flags; FOR$CNV_OUT_y are aliases), and those
JSB to `OTS$$CVT_x_T_R8`. Called from a MACRO program under govax,
where the JSB reached govax's NOTIMPL stub, the stub saw:

- R0 the value's address, and R1 always the calling routine's FP. R2
  varied with the format and arguments (^X30 for F format whatever the
  digits; ^X17 for E with 4 digits, ^X1C for E with 9 and scale 1, ^X2D
  for G with 4): its meaning is open.
- No register FORRTL reads after the RSB: setting R0 to R11 to other
  values on return changed nothing FORRTL wrote.
- FORRTL formats from its own frame: ASCII digits in a buffer at the
  frame's bottom (SP+4, FP-^X74 for F format), whose address is a
  longword at FP-^X14; a longword at FP-^X20 is the offset of the first
  digit in it, which the routine is to write (it was 0 before the call).
  FORRTL rounds the digits itself (G with 4 digits of "12345" wrote
  0.1235). Where the decimal exponent and sign go wasn't found by
  setting fields: so the study, `testdata/ots72/ots72t.mar`, watches
  VMS's own routine through the trace trap instead (its README).

These observations, made by running VMS's FORRTL under govax and
watching the call, are black-box: nothing was disassembled (the author
agreed the method is within the clean-room rules, 2026-10-24).

### The probe's answers (`testdata/ots72`, 2026-10-24)

VMS V7.1 on the VAX 8600 ran both programs (`vax/`). `ots72.mar`
matched govax's choices but for these, which govax now follows:

- **CNVOUT**: a reserved operand is SS$_ROPRAND *signaled*; an H value
  with a four-digit exponent is written, the E and one digit of the
  fraction given up for it (1.0E1000 with 4 digits: `0.100+1001`);
  with digits-in-fraction omitted, asterisks and OTS$_OUTCONERR (VMS
  reads past the argument list).
- **CVT_T_z**: the extension bits aren't stored on an error; an input
  descriptor of an unknown class (3) is read by its length and pointer;
  VMS's H extension bits differ from the true value's in the last bits
  (the manual's 128-bit caveat): an expected difference.
- **The moves**: MOVE3_R5 takes its destination in R2 (R0, R1, R2, the
  CALL entry's order, as MOVE5_R5's R0 to R4).
- **The copies' JSB entries** leave R0 in R6 too.
- **SFREE1_DD and SFREEN_DD check the class**, which the manual says
  they don't: a class 0 descriptor is OTS$_INVSTRDES, signaled; storage
  that isn't a dynamic string's is STR$_ERRFREDYN, signaled.
- Everything else the files marked unconfirmed was as govax chose.

`ots72t.mar`'s tracer caught every call (its self-test too), and the
records give `OTS$$CVT_x_T_R8`'s contract, which `librtl/otscvtfor.go`'s
opening comment states in full: R0 the value, R1 FORRTL's FP, R2 the
digit buffer's size; FP-9 the descriptor's kind (0 E or D, 1 G, 2 F),
FP-16 E's significant digits, FP-20 the buffer, FP-36 F's digits after
the point; out, a guard 0 and the exact decimal digits in ASCII,
rounded in place (the digits after the rounding position left as they
were; a carry into the guard moves the first digit back), FP-32 the
first digit's offset, FP-28 the decimal exponent, FP-24 the sign. G is
not rounded: FORRTL rounds it. VMS's routine produces its digits in
groups of nine (it works in longwords of 10^9), fewer than govax's
exact ones, but FORRTL prints none past those it asked for. Its H
digits drift past H's 34 significant digits (a 40-digit E of 3.14159
ends `...117173`, the true value `...116973`), which govax doesn't copy.

### Side findings

- **govax has no trace trap**: PSL<T> is kept but never traps
  (SS$_TBIT), so a program can't trace itself as the study does, and
  the study runs on VMS only.
- **DIRECTORY with a list of specs** (`DIRECTORY/FULL
  DUA2:[000000]*.EXE,*.DAT`) lists only the first spec's files.
- **govax's debugger ignores `DBG$INPUT`**: RUN/DEBUG in a procedure
  that defines it shows the INITIAL message and ends the session at
  once, where VMS's debugger reads its commands from the file.
- **DUNGEON_V552.EXE** (02.05, the same game linked on VMS 5.5-2
  against FORRTL) activates and runs on govax, then stops with
  `Message number 0018813C`. The probe showed it's FOR-F-ERRDURWRI, as
  on VMS when unit 5 can't be written (answer 3); govax lacks the FOR
  facility's message texts. (Corrected 2026-10-24: `0018813C` is
  ERRDURREA; see subtask 5's entry. Both games now play.)
- **govax's debugger on a FORTRAN image** (answer 6): it keeps
  hexadecimal where VMS's uses decimal, SHOW MODULE has no language
  column and other sizes, and EXAMINE with no address after
  `EXAMINE/INSTRUCTION .PC` shows ^X200, not the next instruction.

## Part B — decisions and questions

### Decisions (proposed 2026-10-24; the author's answers below)

1. **Header layouts as a table** in `vmsimage`, by minor id: the fixed
   header's length, and each block's length and fields. 02.01 is read
   off DUNGEON.EXE; any other minor id is read as 02.05.
2. **The startup stub is recognized by its code**, read from the image
   file at load time (`ICB.LegacyStartup`): an empty entry mask and
   `JMP @#80000168+2`. Then the driver installs the traceback handler,
   as for a first transfer address of 0x7FFEDF68, and calls the second
   transfer address. That's what SYS$IMGSTA does, and it doesn't need
   the old vector yet.
3. **The old vector as forwarding stubs** in S0 at 0x80000000, one
   for each P1 vector entry, at the same offset (answer 8). A
   procedure entry's stub is the P1 entry's own entry mask and
   `JMP @#P1-entry+2`, so a CALL saves what the P1 entry's call would
   (VMS's own SYS$IMGSTA entry is this form, answer 8); a JMP entry's
   (SYS$SRCHANDLER) is `JMP @#P1-entry`; the data cells get none.
   S0 is an identity map of physical memory (VA ^X80000000+x is PA x),
   and VMINIT's layout starts at physical 0, so it moves up past the
   vector's pages (subtask 3). The author accepted moving S0's layout
   and adjusting the tests (Question 2).
4. **A V1 global section ISD named `NAME_nnn`** maps the shareable
   image `NAME` (`SYS$SHARE:NAME.EXE` by the Phase 68 locator), whole,
   at the first such ISD's address, and the image's other ISDs naming
   `NAME` are passed over (the probe's answer 1). The images `NAME`
   uses are activated after it by Phase 68's rules (answer 2).
5. **ANALYZE/IMAGE as VMS's** for a 02.01 image (answer 4), its quirks
   included, in the report alone: `vmsimage` keeps the true strings.
6. **The debugger's language rules** (answer 6), a subtask of their
   own: decimal for FORTRAN, SHOW MODULE's language column, EXAMINE's
   next location.

### Questions for the author (answered 2026-10-24)

1. Can a VMSRTL.EXE from the simh VAX go on DUA1? Yes: it's in
   `DUA1:[XFER]`.
2. Moving VMINIT's S0 layout (Decision 3) changes S0 addresses that
   tests compare (CONSOLE$SCRATCH, the driver's return address
   8002D415 in dumps, ...). Is that acceptable? Yes: move S0's layout
   as needed, and adjust the tests.
3. A probe on the simh VAX? Yes (subtask 2, `testdata/legacy72`).

## Part C — subtasks

1. **The V1 header** (Decision 1), the startup stub (Decision 2), and
   RUN/DEBUG of a V1 image.
2. **The probe** (Question 3, `testdata/legacy72`): ANALYZE/IMAGE of
   DUNGEON.EXE and VMSRTL.EXE, INSTALL's view of VMSRTL, a debugger
   session (SHOW IMAGE, the old vector's entries, VMSRTL's transfer
   vector, STEP), and the two games' output for a few moves.
3. **The old system service vector** (Decision 3).
4. **Global section ISDs** (Decision 4), with VMSRTL.EXE.
5. **FORRTL's I/O** (added 2026-10-24, when the games stopped at their
   first message; the author put the RMS and debugging tools they
   needed in scope): random access by relative record number, the
   terminal's input entry writable, FORTRAN carriage control at the
   terminal, OTS$'s integer conversions, and SET DEBUG RMS.
6. **OTS$'s floating-point conversions** (added 2026-10-24 at the
   author's request): at least OTS$CNVOUT (D, G, H), OTS$CVT_T_D,
   _F, _G, and _H, and the `OTS$$CVT_x_T_R8` routines FORRTL's F, E,
   D, and G edit descriptors use, as standard FORTRAN and C programs
   will need them; with the rest of OTS$ surveyed (MOVE3/MOVE5, the
   SCOPY/SGET/SFREE family, the packed and power routines) and the
   ones as cheap done too. Written from the 5.0 OTS$ manual, on
   `internal/vaxfloat`, and checked by a probe on VMS 7.1. In two parts
   (the internal routines have no manual): first the documented
   routines and the probe, `testdata/ots72`; then, after VMS's run,
   govax changed to its answers, the oracle test, and the internal
   routines written from the study's (both done).
7. **The debugger on a FORTRAN image** (Decision 6).
8. **The close-out**: the manual (`docs/MANUAL.md`), CLAUDE.md,
   PLAN.md.

## Progress log

### 2026-10-24 — Planned; subtask 1, the V1 header

The survey above. Then:

- `vmsimage` has a `layout` per format (`layouts`, by minor id;
  `layoutOf`), which sets the fixed header's length, and the
  activation, symbol table, and identification blocks' lengths and
  fields. `Image.Legacy` says an image is 02.01. A V1 image has no
  SYSVER, IAFVA, initialization list, debug module table, or 32-bit
  sizes. `Image.At` reads the bytes at an address from the file,
  through the private section holding them.
- ANALYZE/IMAGE accepts 02.01. Its report has 02.05's layout, with
  the DMT line showing zeros (VMS's report is unconfirmed: Question 3).
  DUNGEON.EXE now analyzes with 0 errors.
- `machine`: `legacyStartupStub` recognizes the stub (`legacyImgsta`,
  the old vector's SYS$IMGSTA, from the P1 entry's offset), setting
  `ICB.LegacyStartup` at load. `mainTransferAddress` then takes the
  second transfer address, and the driver puts EXE$TRACEBACK in its
  frame.
- `dbgsym.Program.ModuleAt` places a module with no psect records by
  its routines. The debugger names language 1 FORTRAN
  (`languageName`, unconfirmed by a probe).
- Result: RUN/DEBUG DUNGEON stops at `DUNGEO\%LINE 218`'s first
  instruction (0xE1B2) with `%DEBUG-I-INITIAL, Language: FORTRAN,
  Module: DUNGEO`, and SHOW CALLS and STEP are symbolic. RUN gets into
  the program, then takes an access violation at 0x1A778, in VMSRTL's
  unmapped range, from INIT\%LINE 232, with a symbolic traceback.
- Tests: `vmsimage`'s `TestLegacyHeader` and `TestAt` (on DUNGEON.EXE's
  header bytes alone, not the program), `machine`'s
  `TestLegacyStartupStub` and `TestMainTransferLegacy`, `dbgsym`'s
  `TestModuleAtByRoutines`.

### 2026-10-24 — Subtask 2: the probe, written

The author answered Part B's questions, and put VMSRTL.EXE on DUA1.
Its analysis (Part A, "VMSRTL.EXE") showed it's a jacket, so Decision 4
now waits for the probe.

`testdata/legacy72` (README.md): `legacy72.com` analyzes both images,
lists VMSRTL with INSTALL and its global sections, runs DUNGEON under
the debugger with `dun72.dbg`'s commands, then plays a few moves in
DUNGEON and DUNGEON_V552. `exchange.cmd` builds the exchange volume on
DUA2 (the startup procedure may mount DUA1), copying the game from the
work volume; `copyout.cmd` brings back the log, the debugger's log,
both analyses, and the global section listing. A dry run on govax ran
the procedure through (two side findings: DIRECTORY's list, the
debugger's DBG$INPUT).

### 2026-10-24 — Subtask 2: round 1's answers; round 2 written

The author ran round 1 on VMS 7.1 (Part A, "The probe's answers").
Decision 4 is settled by it, and Decisions 5 and 6 are new. Subtask 5
(the debugger on a FORTRAN image) is new.

- ANALYZE/IMAGE matches VMS's report of DUNGEON.EXE (Decision 5): no
  DMT line for a V1 image; the image name and linker ident cut to their
  first longword (`legacyIdentField`); an image name with a control
  character gets `Symbol contains invalid character(s).` (`printable`;
  which printable characters VMS refuses is unconfirmed).
  `TestLegacyImagePages` matches `vax/dungeon.ani` page for page, from
  the header block alone (`testdata/legacy72/dungeon-hdr.bin`, no code)
  and zeros to the file's 210 blocks.
- Round 2 (`legacy72b.com`, `dun72b.dbg`, `exchange2.cmd`,
  `copyout2.cmd`): INSTALL's global sections through a user-mode
  SYS$OUTPUT, the EXAMINEs with `SET RADIX HEXADECIMAL` (and the old
  vector's extent: entries at ^X400, ^X800, ^X918, ^X920, ^XA00), and
  the games played at a terminal, the screen saved.

### 2026-10-24 — Subtask 2: round 2's answers

The author ran round 2 and played both games at a terminal (Part A,
answers 8 to 12). The old S0 vector has the P1 vector's layout and
extent, as Decision 3 assumed, and its SYS$IMGSTA entry is a mask and
a JMP, the form Decision 3's stubs take. VMSRTL's vector needs only
Phase 68's activation once VMSRTL is at ^X1A600. `dungeon-term.log`
is the phase's end-to-end target. The probe is done.

### 2026-10-24 — Subtask 3: the old system service vector

- `machine/legacyvector.go`: VMS V1's vector, a stub at each P1
  vector entry's offset from ^X80000000 (`legacyStub`): the entry's
  empty mask (.P1VECTOR's) and `JMP @#` its XFC, two bytes in, whose
  dispatch goes by the P1 address; for SYS$SRCHANDLER, reached by JMP,
  the jump alone; none for the two data cells. That's every P1 entry,
  to ^XB18 (`legacyVectorBytes`, six pages): VMS's old vector reaches at
  least ^X918, and a superset is harmless (answer 8's correction).
  `legacyVectorBase`, `p1VectorBase`, and `legacyImgsta` moved here
  from `run.go`.
- VMINIT: S0 still maps frame n at page n, so the vector is physical
  0 to ^XBFF, written by `writeLegacyVector` before MAPEN goes on, and
  the system page table follows it: SBR is ^XC00, where it was 0, and
  the rest of the layout moves up by ^XC00 (CONSOLE$SCRATCH ^X8002E000
  at the default sizes, where it was ^X8002D400). The places that took
  S0 page n's PTE to be at physical n*4 (`setS0Protection`, the string
  pool's protection) add SBR; `internal/vm` already did.
  `layoutPages` and the paging-off S0TOOSMALL check count the vector's
  pages.
- Tests: `TestLegacyVectorStubs` (every stub in memory after VMINIT),
  `TestLegacyStubForms`, and the console's `TestLegacyVectorService`
  (an assembled program calls $NUMTIM through ^X800001B8, as DUNGEON
  does, and through the P1 vector, and gets the same date). Adjusted:
  `TestVMInit_pageTablesPacked` (the tables after the vector; SBR),
  `TestVaxInitPagingSizes` (S0BR 00000C00), and `sysgen_test.go`'s two
  page file tests, whose 96-page S0 no longer held the layout and 32
  pages more (104 now).
- The manual's VMINIT paragraph names the vector, and its SHOW MEMORY
  and SHOW PAGE examples have the new addresses (P0BR 80008C00, a PTE
  at 8001CBFC), rerun on govax; `docs/MODE-STACKS.md` says the tables
  follow the vector.
- DUNGEON.EXE: unchanged, as expected. RUN still stops at ^X1A778, in
  VMSRTL's range, until subtask 4 maps it.

### 2026-10-24 — Subtask 4: global section ISDs

- `machine/legacyshare.go`: `finishLoad` of a V1 image
  (`vmsimage.Image.Legacy`) calls `activateLegacyImages`. Each image its
  global section ISDs name (`legacyImageNames`: the name before a final
  `_nnn`, in the order of their first ISDs) is loaded by `loadImage`,
  both loaders alike, with its first ISD as the linked section, marked
  based, so `imageBase` puts its first section at that ISD's address
  (VMSRTL at ^X1A600). The other ISDs naming it are passed over. Its ident
  is checked as any shareable image's (DUNGEON's 0x010003E8 LEQUAL
  against VMSRTL's 0x01000C1C). VMSRTL's own fixup section then
  activates its images from its list's end, by Phase 68's code.
  An image the shims stand for gets nothing (none on hand).
- **Run-time library routines govax lacks** (the author's choice, asked
  when VMSRTL's activation stopped on `SHIM$BASRTL_00000088`): VMSRTL has
  a G^ cell for every routine of BASRTL, COBRTL, and CMA$TIS_SHR, which
  govax stands for (`runtimeLibraries`) with no shims. Phase 68 made
  such a cell `LIB$_UNRESOLVED` at activation. Now `resolveFixupTarget`
  resolves it to a stub of its own (`machine/unresolved.go`,
  `unresolvedStub`: a CALL stub in S0, its code from 10000 registered
  with the System's shims, one per library and offset, made again for
  each new System). If called, it signals SS$_UNSUPPORTED and
  `GOVAX-E-NOTIMPL, routine at BASRTL+^X518 is not implemented in govax`
  (`librtl.NotImplemented`), which names what a new shim would need. A
  library whose file a logical name of its name leads to is still
  activated from that file (FORRTL, MTHRTL here). A JSB entry reached
  through such a stub runs its entry mask as a HALT: nothing says which
  entries are JSB ones. An image of the user's with no shims still
  stops activation.
- **Result**: `RUN DUNGEON` activates DUNGEON, VMSRTL at ^X1A600 to
  ^X1DFFF, and MTHRTL at ^X1E000, as on VMS (answer 1 and 2's SHOW
  IMAGE). FORRTL follows MTHRTL at ^X48400 because govax's LIBRTL is
  shims and takes no pages (VMS's LIBRTL is at ^X48400 and its FORRTL at
  ^X6B200). The game runs INIT into GAME, then stops at its first
  message with FOR-F-ERRDURWRI (`Message number 0018813C`) in FORRTL,
  at the terminal too (tried through a pty). **DUNGEON_V552 stops the
  same way at the same place**, so this is FORRTL's unit 5 I/O on govax
  and not the V1 activation. That's the next thing to find. It blocks the
  end-to-end check against `dungeon-term.log`.
- Tests: `TestLegacyImageNames`, `TestUnresolvedRuntimeStub`. The
  activation itself needs DUNGEON.EXE and VMSRTL.EXE, which can't be
  committed. It was checked by hand, as above, and with the debugger
  (`CALL ^X800323B0`, a BASRTL stub, gives the NOTIMPL message).
- The manual's section on VAX programs says when a library's own image
  is used, shows the new message, and mentions V1 images.

### 2026-10-24 — Subtask 5: FORRTL's I/O

Both games stopped in FORRTL at their first message. SET DEBUG RMS,
which had no consumer, was given one first, and it found three things
in turn; a fourth was a missing routine.

- **SET DEBUG RMS** (`rms/trace.go`, called from `corevms/rms.go`'s
  service wrappers): after each RMS service, one `DEBUG(RMS):` line with
  the FAB (file and default names, FAC, SHR, ORG, RFM, RAT, MRS, IFI,
  STV) or the RAB (ISI, the stream's kind and access: file, terminal,
  record device, or indirect; RAC, ROP, the record number for
  RAB$C_KEY, RSZ and the record's first 60 bytes), and the status by
  name (`StatusName`, `RMS$_FAC/F (00018514)`). In console.help.
- **The first failure was a read, not a write**: FOR-F-ERRDURREA
  (`0018813C`, FORTRAN's error 39; the "ERRDURWRI" in subtask 4's entry
  was wrong), from `$GET` of DTEXT.DAT with RAB$C_KEY, RMS$_RAC. DTEXT
  is a sequential file of 80-byte fixed-length records, read by record
  number (FORTRAN's direct-access READ), which govax's RMS lacked.
  `rms/keyed.go`: RAB$C_KEY on a sequential file of fixed-length
  records, the longword at RAB$L_KBF the record number (from 1),
  RAB$B_KSZ 0 or 4 (else RMS$_KSZ), from the RMS Reference Manual and
  the File Applications guide (8.4.1.2); a sequential `$GET` then reads
  the next record. Unconfirmed: RMS$_KEY for record 0, RMS$_EOF past
  the end, RMS$_RAC for another format, and odd record sizes laid end
  to end (as govax's sequential reads lay them). Keyed `$PUT` (with
  RAB$V_UIF) is not done: nothing on hand uses it.
- **Then FOR-F-ERRDURWRI** (`00188134`): FORRTL's unit 5 is a `$CREATE`
  of `SYS$INPUT:` (default `FOR005.DAT`, RAT FTN), an indirect access to
  the terminal's input entry, which `corevms/ppfstart.go` opened with
  GET alone (marked unconfirmed in Phase 71), so the `$PUT` was
  RMS$_FAC. VMS 7.1 runs the game at the terminal, so its terminal
  input entry takes PUT: `openTerminalEntry` now gives a terminal entry
  GET and PUT. A procedure's or file's input entry stays GET only, as
  Phase 71's probe showed (I9).
- **FORTRAN carriage control at the terminal** (`rms/ftncc.go`): the
  game's records start with FORTRAN carriage control (`$>` its prompt,
  `$Your score is` and `+   0 [...]` its score line), which govax wrote
  as text. An indirect accessor whose open asked for FTN (FAB$V_PPF_RAT
  in the ISI) now has the control character turned into the terminal's
  line model: `' '` a line, `'0'` a blank line first, `'1'` a form feed
  first, `'+'` continuing an open line, `'$'` leaving the line open,
  NUL neither. The open line is the process's (`FileTable.lineOpen`),
  so SYS$OUTPUT's records (LIB$PUT_OUTPUT, `$PUTMSG`) end it, and a
  terminal read ends it (the echoed reply). With piped input nothing is
  echoed, so a reply's output follows the prompt on its line. A
  terminal opened directly (not through a process-permanent file) is
  written as before: its RAT isn't kept (unconfirmed).
- **OTS$CVT_L_TI** was a NOTIMPL stub, as were all 54 OTS$ entries in
  LIBRTL (the LIB$ program, Phases 53 to 61, covered LIB$ only).
  `librtl/otscvt.go` has the integer conversions from the VMS 5.0
  OTS$ manual: CVT_L_TI, _TU, _TB, _TO, _TZ, _TL and CVT_TI_L, _TU_L,
  _TB_L, _TO_L, _TZ_L, _TL_L (codes 185 to 196), with the file's
  unconfirmed choices listed in its opening comment. The manual's
  CVT_TO_L example gives 1017346 octal as 274150; it is 270054. The
  floating, packed, and string routines are still stubs: subtask 6.
- **Result**: both games play. Under a pty, `look`, `score`, `quit`, `y`
  match `testdata/legacy72/vax/dungeon-term.log` line for line,
  `Your score is   0 [total of 585 points], in    2 moves.` included.
- Tests: `TestSysGetByRelativeRecordNumber`, `TestSysGetByKeyStatuses`,
  `TestTraceShowsKeyedGet`, `TestFTNTerminalText`,
  `TestTerminalTextJoinsPromptAndReply` (rms), `TestOTSIntegerToText`,
  `TestOTSTextToInteger` (librtl); `TestOpenTerminalFiles` now wants the
  terminal's input entry GET and PUT, and `TestSysGet_invalidRAC` uses an
  access mode RMS doesn't define. The games themselves can't be
  committed, so the end-to-end run was by hand.

### 2026-10-24 — Subtask 6, part 1: OTS$'s documented routines, and the probe

The survey (Part A, "OTS$ in LIBRTL"), then:

- `librtl/otsfloat.go`: OTS$CNVOUT, _G, _H (FORTRAN's E format:
  `[-]0.dddE+nn`, a three-digit exponent without the E, right-justified;
  OTS$_OUTCONERR's asterisks; SS$_ROPRAND for a reserved operand, the
  string unchanged) and OTS$CVT_T_D, _F, _G, _H (the manual's syntax:
  leading blanks, a sign, digits and a period, an exponent by letter E,
  D, or Q or, unless flags bit 5, by its sign; digits-in-fraction; the
  scale factor; flags 0 to 6; extension bits; OTS$_INPCONERR with +0.0
  stored). Values are exact rationals, rounded once half away from
  zero (`vaxfloat.Round`) or truncated (`truncateToFormat`, which also
  gives the extension bits). The file's opening comment lists the
  unconfirmed choices. Codes 197 to 203.
- `librtl/otsstring.go`: OTS$MOVE3 and OTS$MOVE5 with longword lengths
  (MOVC3's and MOVC5's moves and R0), their `_R5` JSB entries (input
  registers guessed: MOVE3_R5 as MOVC3 holds them, R0, R1, R3; MOVE5_R5
  R0 to R4); OTS$SCOPY_DXDX, SCOPY_R_DX, SGET1_DD, SFREE1_DD, SFREEN_DD
  on LIB$'s helpers, scalars by value, the copies returning MOVC5's R0
  (bytes not copied), errors signaled as OTS$_INVSTRDES, INSVIRMEM, and
  FATINTERR (for SFREE1_DD of storage that isn't a dynamic string's,
  where LIB$ returns STR$_ERRFREDYN; unconfirmed); and their JSB
  entries. Codes 204 to 217.
- The probe, `testdata/ots72` (README.md; `gen.go` writes it):
  `ots72.mar`'s 148 cases of those routines, every unconfirmed choice
  among them; and `ots72t.mar`, the study of FORRTL's calls to
  `OTS$$CVT_x_T_R8`: 29 `FOR$CVT_x_Ty` calls, each traced with PSL<T>
  until the JSB, then the registers and the caller's frame recorded at
  the JSB and the RSB, after a self-test of the tracer on the program's
  own JSB routine. Both ran through on govax (the study catches nothing
  there: no trace trap, Part A's side finding).
- Tests: `TestOTSCnvout`, `TestOTSTextToFloat` (the manual's CVT_T_F
  example among them), `TestOTSTextToFloatTruncated`,
  `TestExponentText`, `TestOTSMove`, `TestOTSCopy`, `TestOTSDynamic`.
- The manual's FORTRAN paragraph says the OTS$ floating conversions a
  program calls itself work, and FORRTL's floating edit descriptors
  don't yet.

### 2026-10-24 — Subtask 6, part 2: VMS's answers, and FORRTL's floating output

The author ran `testdata/ots72` on VMS 7.1 (Part A, "The probe's
answers (`testdata/ots72`)"). Then:

- `otsfloat.go` and `otsstring.go` changed to VMS's answers (CNVOUT's
  signaled ROPRAND, four-digit exponent, and omitted digits; CVT_T_z's
  extension bits and any input class; MOVE3_R5's R2; the copies' R6;
  SFREE's class check and STR$_ERRFREDYN), their opening comments
  rewritten to say what VMS confirmed.
- `librtl/otscvtfor.go`: OTS$$CVT_D_T_R8, _F_, _G_, and _H_ (JSB
  codes 218 to 221), from the study's contract. A reserved operand is
  SS$_ROPRAND, signaled (unconfirmed).
- `TestOTS72Oracle` (`internal/console/ots72_test.go`) matches
  `vax/ots72-vax.txt` line for line but for `expectedOTS72Differences`
  (H's extension bits; the dynamic-string JSB entries' heap registers).
  `TestOTSCvtForDigits` (librtl) runs the R8 routines in frames laid
  out as FORRTL's, against VMS's records. `ots72t.mar` itself needs
  VMS's FORRTL.EXE, which can't be committed: run by hand with the
  author's DUA1 FORRTL, its 29 FORRTL conversions give VMS's text but
  for the 40-digit H case's last digits (above).
- The manual's FORTRAN paragraph now says the floating edit descriptors
  work.
