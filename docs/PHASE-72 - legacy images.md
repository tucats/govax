# Phase 72 — legacy images

**Status:** in progress. Planned 2026-10-24; the author answered Part
B's questions the same day. Subtask 1 (reading a V1 image's header,
and RUN/DEBUG of one) is done; subtask 2's probe (`testdata/legacy72`)
is written and waiting for its VAX run.

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

### Side findings

- **DIRECTORY with a list of specs** (`DIRECTORY/FULL
  DUA2:[000000]*.EXE,*.DAT`) lists only the first spec's files.
- **govax's debugger ignores `DBG$INPUT`**: RUN/DEBUG in a procedure
  that defines it shows the INITIAL message and ends the session at
  once, where VMS's debugger reads its commands from the file.
- **DUNGEON_V552.EXE** (02.05, the same game linked on VMS 5.5-2
  against FORRTL) activates and runs on govax, then stops with
  `Message number 0018813C` (FOR facility, number 39, severe, likely
  FOR$_ERRDURREA) and a traceback through FORRTL. That's a separate
  problem, for later.

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
3. **The old vector as forwarding stubs** in S0 at 0x80000000. Each
   entry at offset n jumps to the P1 vector's entry at 0x7FFEDE00+n
   (keeping the caller's frame, so the service sees its arguments as
   usual). VMINIT's S0 layout moves up past the vector (subtask 3).
4. **A V1 global section ISD named `NAME_nnn`** maps the shareable
   image `NAME` (`SYS$SHARE:NAME.EXE` by the Phase 68 locator). The
   cluster-for-cluster reading proposed first doesn't fit VMSRTL.EXE
   (above); how VMS places it is the probe's question, and this
   decision waits for its answer.

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
5. **The close-out**: the manual (`docs/MANUAL.md`), CLAUDE.md,
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

