# Phase 81 — FORTRAN files

**Status:** in progress: subtask 1, the survey, done 2026-10-31, for the author's review. Planned 2026-10-29. Needs Phase 80 (formatting and the
error model). Part of the FORTRAN program laid out in `PHASE-74 -
FORTRAN plan.md` (Decisions 12, 13, and 21).

## Goal

FORTRAN's file statements and every access method this program
supports, on volume files and host files:

- **OPEN** with every keyword in chapter 9 (Table 9-1): FILE, NAME,
  DEFAULTFILE, STATUS, DISPOSE, ACCESS, FORM, ORGANIZATION, RECORDTYPE,
  RECL, RECORDSIZE, CARRIAGECONTROL, BLANK, BLOCKSIZE, BUFFERCOUNT,
  EXTENDSIZE, INITIALSIZE, MAXREC, ASSOCIATEVARIABLE, READONLY, SHARED,
  NOSPANBLOCKS, TYPE, USEROPEN (a program's routine called to do the
  `$OPEN` or `$CREATE`), ERR, IOSTAT;
- **CLOSE** (STATUS and DISPOSE: KEEP, DELETE, PRINT, SUBMIT), and the
  units closed at image exit (an exit handler);
- **INQUIRE** by unit and by file, every specifier in 9.3;
- **REWIND, BACKSPACE, ENDFILE**;
- **unformatted sequential** I/O (VAX FORTRAN's segmented records for
  RECORDTYPE='SEGMENTED');
- **direct access:** READ and WRITE with REC= on fixed-length sequential
  files, ASSOCIATEVARIABLE, MAXREC; DEFINE FILE and FIND (appendix A);
- **relative files:** ORGANIZATION='RELATIVE', direct and sequential
  access, DELETE of a record;
- **default names:** unit n is file FORnnn.DAT unless a logical name
  FORnnn is defined, and the terminal statements' names (FOR$READ,
  FOR$ACCEPT, FOR$PRINT, FOR$TYPE): the User Manual's conventions,
  marked unconfirmed;
- **the RMS (and `ods2`) additions** these need.

Indexed files (ORGANIZATION='INDEXED', keyed READ, REWRITE, DELETE,
UNLOCK on them) are not in this phase: the author deferred them to a
later program (Phase 74's review, answer 4). Until then, OPEN with
ORGANIZATION='INDEXED' fails with a FOR$_ error (the survey picks which;
FOR$_INCFILORG or FOR$_NOTIMP), and the keyed statements compile but
report the same at run time. REWRITE and UNLOCK on relative files are
in this phase.

## What earlier phases leave in place

- **Phase 80:** FTNRTL's formatting and error model.
- **RMS:** sequential files on volumes and the host, FAB/RAB/NAM/XAB
  handling, FORTRAN carriage control, file sharing and record locks
  (Phase 47), `$UPDATE` and `$TRUNCATE` (Phase 49), keyed reads of
  fixed-length sequential files (Phase 72).
- **`ods2`**, which this phase may change (Decision 21: change the
  sibling checkout, tag, then `GOWORK=off go get`).

## Design notes

- **FTNRTL's unit table** gets, for each open unit, a FAB, RAB, NAM, and
  record buffer from LIB$GET_VM, and the unit's attributes; INQUIRE
  answers from it and from the NAM.
- **RMS additions expected:** keyed `$PUT` and `$UPDATE` on fixed-length
  sequential files (direct-access WRITE), which may need the file
  extended through the record written; the relative organization
  (prologue, cells with their deleted and present flags, `$GET`, `$PUT`,
  `$UPDATE`, `$DELETE`, `$FIND` by record number and sequentially),
  which needs ods2 to create files with FAB$C_REL's attributes. The
  survey settles which of these need ods2 changes.
- **Segmented records:** VAX FORTRAN's unformatted sequential files hold
  a logical record as variable-length RMS records, each beginning with a
  control word (first, last, middle, or only segment); the manual's
  RECORDTYPE section describes them.

## Open questions

- **Relative files' prologue and cell layout:** from the *File
  Applications* guide and the RMS manual; checking against a VMS-made
  relative file (an exchange volume) would confirm the layout.
- **DISPOSE='PRINT' and 'SUBMIT':** govax has no print or batch queues;
  the proposal is to keep the file and report that it wasn't queued.

# Part A — the survey (2026-10-31)

Section numbers are the 1988 *VAX FORTRAN Language Reference Manual*'s
(the LRM). As in Phases 78 to 80, what a rule means for a pass is in
brackets. **[trial]** marks what VMS 7.1's FORRTL.EXE did when called
from MACRO over govax (Phase 80's ruling: its behavior may be studied,
its code neither copied nor disassembled). Those results are FORRTL's
logic over govax's RMS, so they stand until a VMS run confirms them.

## Sources

- **The LRM:** chapter 9 (OPEN, CLOSE, INQUIRE, REWIND, BACKSPACE,
  ENDFILE, DELETE, UNLOCK), chapter 7's unformatted, direct access, and
  REWRITE statements (7.2.1.4, 7.2.2, 7.3.1.4, 7.3.2, 7.4), the record
  specifier (7.1.1.6), and appendix A.2 (DEFINE FILE) and A.3 (FIND).
  The manual sends the reader to the *VAX FORTRAN User Manual* for
  default names, file sharing, USEROPEN's interface, and IOSTAT's
  numbers. That manual still isn't here.
- **The *Guide to OpenVMS File Applications*** (7.3 text): relative
  files in outline (2.2.2, 3.4): a prologue, then fixed-length cells,
  each a deleted-record byte and the record, 1 byte longer than a
  fixed-length record or 3 longer than a variable-length one's maximum,
  in buckets of up to 63 blocks. No byte layout of the prologue or the
  control byte's bits. **It also says sequential files pad an odd-length
  record to an even byte** (3.2, "RMS adds an empty byte"). That holds
  for VAR records in ods2. For FIX records govax and ods2 lay records end
  to end (`rms/keyed.go` already marks this unconfirmed).
- **The RMS reference** (`OVMS_731_RMS.pdf`, no text extraction yet):
  `$PUT`, `$UPDATE`, `$DELETE`, and `$FIND` by RRN, FAB$L_MRN, and
  FAB$L_FOP's DLT, TMD, TMP, SCF, SPL.
- **FORRTL.EXE** on the simh VAX's system disk
  (`testdata/disks/rq0-ra92.dsk`, local only). Its symbol names include
  the statement entries this phase needs: `FOR$OPEN`, `FOR$CLOSE`,
  `FOR$INQUIRE`, `FOR$REWIND`, `FOR$BACKSPACE`, `FOR$ENDFILE`,
  `FOR$DEF_FILE_W`, `FOR$FIND`, `FOR$DELETE`, `FOR$UNLOCK`, `FOR$READ_SU`,
  `FOR$READ_DF/DU/DO`, `FOR$READ_KF/KU/KO`, `FOR$REWRITE_SF/SU/SO`, and
  **`FOR$RAB`**, which returns a unit's RAB. With FOR$RAB a probe can
  print every FAB, RAB, and XAB field FORRTL sets for a unit, without
  looking inside FORRTL. `SET DEBUG RMS` (`rms/trace.go`) shows each RMS
  call FORRTL makes over govax, which is how the trials below were read.
- **On the shelf, not yet used:** HP's 2005 *HP Fortran for OpenVMS
  Language Reference Manual* (`VMS-FORTRAN-LANGUAGE.txt`), the
  descendant of the 1988 LRM. It repeats chapter 9's rules (FORnnn.DAT,
  the record types) but not the User Manual's details. Question 4 asks
  whether it may be used.

## Where the compiler stands

**The parser is done** (Phase 76): `ast.AuxIOStmt` is OPEN, CLOSE,
INQUIRE, REWIND, BACKSPACE, ENDFILE, UNLOCK, DELETE, and FIND (unit,
record, the other specifiers in order); `ast.DefineFileStmt` is DEFINE
FILE; `ast.TransferStmt` has REWRITE, the record (`u'r` and REC=), KEY=
and KEYID= specifiers, and `ast.KeyList` OPEN's KEY=.

**Semantic analysis reports NOTIMPL** for: every `AuxIOStmt` and DEFINE
FILE (`handled`, `sema/notimpl.go`); REWRITE (`handled`); a READ or WRITE
with a record ("direct access I/O") or with neither format nor namelist
("unformatted I/O", `transferStatement`, `sema/io.go`); and any
control-list specifier but ERR=, END=, and IOSTAT= (`ioSpecs`: KEY=,
KEYEQ=, KEYGE=, KEYGT=, KEYID=). `ir` and `codegen` have nothing for
these yet.

## FTNRTL today

- **Units** (`ftn_units.mar`): two static streams, OUTPUT (units -1,
  -2, 6) and INPUT (-3, -4, 5), each a block with a FAB, RAB, NAM, and
  resultant-name buffer, connected at first use by `$CREATE` of
  SYS$OUTPUT or SYS$INPUT. `FTN$$UNIT_FIND` answers NOTIMP for 0 to 119
  but 5 and 6, and INVLOGUNI outside -4 to 119.
- **Records:** a statement's record is in its context (`CTX_RECORD`, 133
  bytes for output, `INPUT_MAX` 199 for input), reached through
  `CTX_RECP`, which internal files already point elsewhere. A unit's
  RECL can reach 32766, so the record must become the unit's buffer,
  `CTX_RECP` the seam.
- **The context's layout is restated in each module** (`ftn_statement`,
  `ftn_format`, `ftn_listin`, ...), each with the offsets it uses. A
  phase adding fields to every module's copy may want one included
  definition first (a macro library, or a `.REQUIRE`-like shared file, if
  govax's MACRO and LINK build allow). A housekeeping choice, not a rule.
- **No exit handler.** RMS's rundown closes the files at image exit
  (`rms.FileTable.Rundown`), but a disposition (DELETE, a scratch file)
  needs FTNRTL's own handler.
- Vector: 49 entries to `FTN$ERRSNS_W` at 384, GSMATCH 1,9.

## RMS and ods2 today

| Need | Today | Where |
|---|---|---|
| `$OPEN`/`$CREATE` of a volume file, NAM, XABs, CIF, UFO, CTG, CBT, ALQ, DEQ | yes | `rms/open.go`, `create.go` |
| a host file through `$OPEN`/`$CREATE` | **no**: a program's RMS reaches volume files, record devices, the terminal, and process-permanent files only; a name on no mounted volume is DNR | `createOnVolume` |
| RFM FIX, VAR, VFC, STM, STMLF, STMCR on create | yes (UDF refused) | `create.go` |
| ORG other than sequential | **no**: RMS$_ORG | `create.go`, `fab.go` |
| `$GET`, `$FIND` by RRN on FIX sequential | yes (Phase 72) | `keyed.go`, `stream.go` |
| `$PUT` by RRN, with or without UIF | **no**: RMS$_RAC (the trial below hits it) | `put.go` |
| `$UPDATE` (same length), `$TRUNCATE`, `$REWIND`, `$DELETE` (RMS$_IOP on sequential) | yes (Phase 49) | `recordops.go` |
| `$PUT` with TPT, RAB$V_EOF (APPEND) | yes | `put.go`, `connect.go` |
| FOP DLT (delete at close), TMD, TMP, SCF, SPL, MXV | **no**: not read | `close.go`, `create.go` |
| FAB$L_MRN, FAB$B_BKS for relative files | **no** | |
| record locks, sharing (SHARED) | yes (Phase 47) | `sharing.go`, `recordlock.go` |
| odd-length FIX records padded to a word | **no**: end to end | ods2 `rms/reader.go`, `writer.go`; `keyed.go` |
| SET DEBUG RMS showing FOP, ALQ, DEQ, BKS, MRN | **no**: FAC, SHR, ORG, RFM, RAT, MRS, IFI, and the RAB's ROP | `rms/trace.go` |

**ods2:** `RecAttr.Format` is FAT$B_RTYPE's whole byte, the organization
in its high nibble. govax splits it (`fileattr.go`), but ods2's own
reader and writer take any byte as a record format, so a relative file
read as records is misread. `Volume.CreateFile` takes a `RecAttr`, and
`File.WriteBlock` and `SetEndOfFile` exist, so govax's RMS can lay out a
relative file over block I/O with small ods2 changes: name the
organization, refuse record access to a non-sequential file, pad odd FIX
records.

**Found in passing:** ods2's record reader fails on VMS's own FORRTL.EXE
("fixed-length record: unexpected EOF"), so govax's COPY of it to the
host stops early. The file is FIX 512 with its end of file at block 80,
byte 330, mid-record. RUN maps it by blocks and doesn't notice. What VMS's
`$GET` returns for such a last record is a probe case. The fix belongs
with the FIX padding work.

## The rules

### OPEN (9.1)

Keywords in any order; UNIT= optional if first; character values may have
trailing blanks, never leading or embedded ones; numeric expressions are
converted to integer. [sema: each keyword at most once, each value's
type; a constant character value checked against the keyword's list at
compile time, a run-time one by FTNRTL, FOR$_KEYVALERR (45). Which pairs
conflict (ACCESS='DIRECT' without RECL, SEGMENTED with formatted, ...)
is listed below; whether VAX FORTRAN's compiler or FORRTL catches each is
unknown, so ours are compile-time where the values are constants.]

How each keyword reaches RMS (proposed; [trial] where seen):

| Keyword | Values | RMS | Notes |
|---|---|---|---|
| UNIT | e | | 0 to 99 by 7.1.1.2; FORRTL takes 0 to 119 (Phase 80), and -4 to -1 are its default units |
| FILE, NAME | c | FNA | numeric: ASCII ending in a NUL (9.1.11) |
| DEFAULTFILE | c1 | DNA | default `FORnnn.DAT` (9.1.7) |
| STATUS, TYPE | OLD, NEW, SCRATCH, UNKNOWN | `$OPEN`; `$CREATE`; `$CREATE` FOP TMD?; `$CREATE` FOP CIF? | default UNKNOWN (NEW under /NOF77) [the implicit open is a `$CREATE`, trial] |
| DISPOSE, DISP | KEEP, SAVE, DELETE, PRINT, PRINT/DELETE, SUBMIT, SUBMIT/DELETE | `$CLOSE` FOP DLT, SPL, SCF | "a read-only file cannot be deleted; a scratch file cannot be saved, printed, or submitted" |
| ACCESS | SEQUENTIAL, DIRECT, KEYED, APPEND | RAC SEQ; RAC KEY and ROP UIF [trial]; —; ROP EOF | KEYED is the indexed program's |
| FORM | FORMATTED, UNFORMATTED | | default by ACCESS: sequential formatted, direct and keyed unformatted |
| CARRIAGECONTROL | FORTRAN, LIST, NONE | RAT FTN [trial], CR, 0 | default FORTRAN formatted, NONE unformatted [trial: RAT 0] |
| RECORDTYPE | FIXED, VARIABLE, SEGMENTED, STREAM, STREAM_CR, STREAM_LF | RFM FIX, VAR, VAR [trial], STM, STMCR, STMLF | defaults: relative and direct FIXED, formatted sequential VARIABLE, unformatted sequential SEGMENTED |
| RECL, RECORDSIZE | e | MRS | bytes formatted, longwords unformatted; limits Table 9-2 (32766/8191 sequential, 16380/4095 relative); defaults Table 9-3 (FIXED none, SEGMENTED 2048, others 133); mandatory for FIXED and relative |
| ORGANIZATION | SEQUENTIAL, RELATIVE, INDEXED | ORG | an existing file's own if omitted; must match if given (INCFILORG, 51) |
| BLOCKSIZE | e | MBC (sequential), BKS (relative) | rounded up to blocks |
| BUFFERCOUNT | e | MBF | 1 to 127, 0 the default; no effect on govax |
| INITIALSIZE, EXTENDSIZE | e | ALQ with FOP CBT, DEQ | |
| MAXREC | e | MRN (relative); FTNRTL's own check otherwise? | |
| ASSOCIATEVARIABLE | asv | | FTNRTL stores the next record number after each direct operation; not a dummy argument [sema] |
| NOSPANBLOCKS | | RAT BLK | |
| READONLY | | FAC GET, SHR GET | lets others read; a DELETE disposition is refused |
| SHARED | | SHR PUT, GET, DEL, UPD | records locked as RMS locks them |
| BLANK | NULL, ZERO | | default NULL after OPEN, ZERO with none (Phase 80's Decision 11) |
| USEROPEN | p | | declared EXTERNAL [sema]; the routine does the `$OPEN`/`$CREATE` and `$CONNECT` |
| KEY | (e1:e2[:dt[:dr]],...) | | indexed only: Decision 13 |
| ERR, IOSTAT | | | the OPEN alone, as for other statements |

**A unit already connected (9.1.27):** to another file, the OPEN first
closes it; to the same file, or with no FILE=, only BLANK may change, and
the position stays. [FTNRTL compares the new FNA's expanded name with the
open file's.]

### Default names [trial]

Without OPEN, the first statement on unit n does a `$CREATE` with FNA
`FORnnn` and DNA `FORnnn.DAT` (units 0, 1, 2, 3, 99, 119 seen). So a
logical name FORnnn leads to any file, and with none the file is
FORnnn.DAT in the default directory. Units 5 and 6 and the default units
are different. FORRTL itself looks for the logical name: if it's defined,
FNA is that name, and if not, FNA is SYS$INPUT: or SYS$OUTPUT: and DNA the
unit's default file:

| Unit | FNA without the logical name | DNA | Logical name |
|---|---|---|---|
| 5 | `SYS$INPUT:` | `FOR005.DAT` | FOR005 |
| 6 | `SYS$OUTPUT:` | `FOR006.DAT` | FOR006 (seen: defined, FNA became `FOR006`) |
| -1 (PRINT) | `SYS$OUTPUT:` | `FORPRINT.DAT` | FOR$PRINT, by the manual's name |
| -2 (TYPE) | `SYS$OUTPUT:` | `FORTYPE.DAT` | FOR$TYPE |
| -3 (ACCEPT), -4 (READ) | `SYS$INPUT:` presumably | `FORACCEPT.DAT`, `FORREAD.DAT` presumably | FOR$ACCEPT, FOR$READ |

**Each unit has its own FAB**, even -1, -2, and 6 on one SYS$OUTPUT.
FTNRTL's Phase 80 streams share one RAB so that a `$` prompt stays open
across them (Decision 3). The implicit open uses FAC=GET,PUT,DEL,UPD,TRN
and ROP TPT, RAH, WBH, LOC [trial].

### CLOSE (9.2)

`CLOSE ([UNIT=]u[,STATUS|DISPOSE|DISP=p][,ERR=s][,IOSTAT=ios])`. CLOSE's
disposition overrides OPEN's, but a scratch file can't be kept and a
read-only one can't be deleted. Default: DELETE for a scratch file, KEEP
otherwise. [CLOSE of a unit not open does nothing: unconfirmed.]
**Image exit closes every unit** [trial: FORRTL's `$CLOSE`s ran after
the program's RET, through an exit handler] with its disposition.

### INQUIRE (9.3)

`INQUIRE (FILE=fi[,DEFAULTFILE=dfi],flist)` or `INQUIRE ([UNIT=]u,flist)`.
Specifiers: ACCESS, BLANK, CARRIAGECONTROL, DIRECT, EXIST, FORM, FORMATTED,
KEYED, NAME, NAMED, NEXTREC, NUMBER, OPENED, ORGANIZATION, RECL,
RECORDTYPE, SEQUENTIAL, UNFORMATTED, and ERR, IOSTAT. Values are words
padded with blanks (SEQUENTIAL, DIRECT, KEYED, UNKNOWN; YES, NO,
UNKNOWN; ...), logical (EXIST, NAMED, OPENED), or integer (NEXTREC,
NUMBER, RECL). [sema: each a place of the right type, character for the
words; FILE= or UNIT=, not both? The manual allows DEFAULTFILE with
either.] RECL is in longwords for a file opened unformatted, otherwise
bytes; for a closed file, MRS, or the longest record (LRL) when MRS is
0, zero for no file. NAME is a full name valid for FILE= (the resultant
string). NEXTREC is direct access only, the last record plus 1. [By
file: `$PARSE` and `$SEARCH` for EXIST and NAME, and an `$OPEN` for the
attributes (XABFHC for LRL)? Which calls FORRTL makes for each is a probe
case; so is what EXIST says of a unit (any 0 to 119? -4 to -1?).]

### REWIND, BACKSPACE, ENDFILE (9.4–9.6)

Each `stmt ([UNIT=]u[,ERR=s][,IOSTAT=ios])` or `stmt u`. REWIND: a file
open for sequential or append access. BACKSPACE: "rewinding to the start
of the file and then performing n-1 successive reads", so FTNRTL keeps a
unit's record count, and BACKSPACE on direct, keyed, or append access is
an error (BACERR, 23?). ENDFILE: on a unit not open, the default file is
opened for unformatted output; only formatted sequential and segmented
files; the end-file record is a 1-byte record holding ^Z (1A), which a
READ meets as an end of file. [trial: REWIND then ENDFILE on unit 1 is a
`$REWIND`, then a `$PUT` of `1A`, with ROP TPT, so the file ends there.]

### Unformatted sequential I/O (7.2.1.4, 7.3.1.4)

READ reads exactly one logical record, discards what the list leaves,
and an item past the record's end is an error (INPSTAREQ, 67?). With no
list it skips a record. WRITE writes exactly one record; with no list, a
null record.

**Segmented records [trial]:** each RMS record (VAR, RAT none) begins
with a 2-byte control word. A one-segment record written by
`WRITE (1) 42` was `03 00 2A 00 00 00`: control 3, then the longword. So
the control word's bit 0 must mean the first segment and bit 1 the last
(0 a middle one). Where FORRTL splits a long record (RECL, 2048 by Table
9-3, in bytes or longwords?), and SEGRECFOR (35) for a bad control word,
are probe cases. A file of VAR records without segments is read
unformatted only with RECORDTYPE='VARIABLE' given (9.1.23). [FTN$IO_ITEM
copies an item's bytes into the record (a CHARACTER item its text, a
whole array all of it) on output, and out of it on input.]

### Direct access (7.2.2, 7.3.2, 7.1.1.6)

`READ (u,REC=r[,fmt]...)`, `u'r`, and WRITE. r from 1 to the file's
records, converted to integer. Formatted READ: fields past the record
read as blanks. Formatted WRITE: the record padded with blanks; past the
record, an error (OUTSTAOVE?). Unformatted: padded with zeros; too long
an error. After each, the associated variable is the next record's
number.

**On a sequential file [trial]:** DEFINE FILE's file was a sequential FIX
file, and the first `WRITE (3'5)` was a `$PUT` with RAC KEY, KBF 5, and ROP
UIF (update if the record exists), which govax's RMS refuses with
RMS$_RAC. So FORRTL writes a direct record anywhere with a keyed `$PUT`.
**What VMS's RMS does with a record past the end of file** (records 1 to
4 made with zeros? an error?) and with UIF clear on an existing record,
and what a direct READ of a record past the end gives (ATTACCNON, 36, or
RECNUMOUT, 25) are probe cases.

### Relative files, REWRITE, DELETE, UNLOCK (9.1.19, 7.4, 9.7, 9.8)

ORGANIZATION='RELATIVE': RECL mandatory, FIXED by default, direct and
sequential access (a sequential READ passes over empty cells, Guide
Figure 2-2). **REWRITE** updates the current record (the one the last
READ on the unit got), formatted or unformatted, filled with blanks or
zeros; any other I/O statement on the unit between them loses it
(NO_CURREC, 53?). **DELETE** `(u,REC=r)` or `(u'r)` deletes a relative
file's record and sets the associated variable to the next record; the
current-record form `DELETE (u)` is for indexed files. **UNLOCK** frees a
record a READ locked (on a shared file); with none locked it does
nothing. [RMS: `$DELETE`, `$UPDATE`, `$FREE`, `$FIND` by RRN on a
relative file; RECORD locking already exists.]

### DEFINE FILE and FIND (A.2, A.3)

`DEFINE FILE u(m,n,U,asv)[,...]`: m records of n 16-bit words,
unformatted, asv the associated variable (not a dummy argument). The
manual says it describes a relative file. The file is opened by the first
direct statement: a WRITE creates it, a READ or FIND opens an existing
one. **[trial] FORRTL's DEFINE FILE made a sequential FIX file**
(FNA FOR003, ORG 0, RFM FIX, MRS 16 for n=8), so the manual's "relative"
may be wrong for VMS 7.1's FORRTL. The VMS run settles it (Decision 1).
`FIND (u'r)` or `FIND ([UNIT=]u,REC=r)` positions a direct access file
and sets its associated variable to r, without moving data; it may open
the file. (A.3 says it "must refer to a relative organization file",
the same question.)

### Indexed files

Not in this phase (Phase 74's review). The forms that are indexed only:
ORGANIZATION='INDEXED', ACCESS='KEYED', KEY=, the keyed READ (KEY=,
KEYEQ=, KEYGE=, KEYGT=, KEYID=), a WRITE or REWRITE or `DELETE (u)` on an
indexed file, and INQUIRE's KEYED=YES. See Decision 13.

### IOSTAT and the FOR numbers

The new statements' errors, by VMS's FOR message list (`vmsdef`'s
`forMessages`, number = the code's bits 3 to 14): REWERR 20, DUPFILSPE
21, BACERR 23, RECNUMOUT 25, OPEDEFREQ 26, TOOMANREC 27, CLOERR 28,
FILNOTFOU 29, OPEFAI 30, MIXFILACC 31, ENDFILERR 33, UNIALROPE 34,
SEGRECFOR 35, ATTACCNON 36, INCRECLEN 37, NO_SUCDEV 42, FILNAMSPE 43,
INCRECTYP 44, KEYVALERR 45, INCOPECLO 46, WRIREAFIL 47, INCFILORG 51,
SPERECLOC 52, NO_CURREC 53, REWRITERR 54, DELERR 55, UNLERR 56, FINERR
57; and two warnings, IGNORED 3 ("ignored requested disposition") and
IGNNOTDEL 4 ("..., file not deleted"). Which condition each failure gets
(an OPEN of a missing file: FILNOTFOU, or OPEFAI with RMS$_FNF?) is a
probe case per statement.

## What FORRTL shows (three trials, 2026-10-31)

Scratch programs (not kept) linked against FORRTL.EXE, run with
`SET DEBUG RMS` on a scratch volume as DUA2. Every result is [trial].

1. **Implicit opens.** `FOR$WRITE_SU` on unit 1 and `FOR$WRITE_SO` on
   unit 2: each a `$PARSE` (no NAM, so RMS$_NAM, which FORRTL ignores),
   then `$CREATE` FNA `FOR00n`, DNA `FOR00n.DAT`, FAC 1F, RFM VAR, RAT 0
   (unformatted) or FTN (formatted), MRS 0; `$CONNECT` ROP TPT, RAH, WBH,
   LOC. The segment control word above. `FOR$REWIND` then `FOR$ENDFILE`:
   `$REWIND`, `$PUT` of `1A`. At image exit, a `$CLOSE` for each unit.
2. **DEFINE FILE.** `FOR$DEF_FILE_W(3, 10, 8, asv)` (unit, m, n by
   value, asv's address, found by trial) and `FOR$WRITE_DU(3, 5)` (unit,
   record by value): `$CREATE` ORG SEQ, RFM FIX, MRS 16, then the keyed
   `$PUT` with UIF and RMS$_RAC from govax, so `%FOR-F-ERRDURWRI` with
   `-RMS-F-RAC`. (`FOR$CLOSE(4)` was never reached.)
3. **Default units.** The names in the table above, for units 0, 6, 99,
   119, -1, -2, 5. Writing unit 5 when SYS$INPUT is a command procedure
   is RMS$_FAC, ERRDURWRI, as on VMS (Phase 80).

## What has to change

- **Compiler.** `sema`: OPEN, CLOSE, INQUIRE, the positioning
  statements, DELETE, UNLOCK, FIND, DEFINE FILE, REWRITE, unformatted and
  direct READ and WRITE; the keyword checks and new messages (a keyword
  given twice, a value not in the list, a missing RECL, ASSOCIATEVARIABLE
  a dummy, USEROPEN not EXTERNAL, ...). `ir`: a call per statement
  (Decision 4), the record number by value, specifiers' outputs as
  addresses or descriptors. `codegen`: nothing new expected.
- **FTNRTL.** `ftn_units.mar` becomes the unit table (Decision 2); new
  modules for OPEN and CLOSE (`ftn_open.mar`), INQUIRE
  (`ftn_inquire.mar`), the positioning and record statements
  (`ftn_position.mar`), and unformatted records (`ftn_unformatted.mar`);
  the exit handler; the context's record moved to the unit; new vector
  entries, GSMATCH raised.
- **RMS.** Keyed `$PUT` (and UIF) on FIX sequential files; FOP DLT, TMD,
  TMP, SCF/SPL (accepted, nothing queued), MXV if FORRTL uses it; MRN,
  BKS; the relative organization (create, open, `$GET`, `$PUT`, `$FIND`,
  `$UPDATE`, `$DELETE` by RRN and sequentially, `$REWIND`, locks);
  `$DISPLAY` and XABFHC for relative files; SET DEBUG RMS showing FOP,
  ALQ, DEQ, BKS, and MRN, for the trials and the probe.
- **ods2.** The organization nibble named (`RecAttr`), record access to a
  non-sequential file refused, odd FIX records padded to a word, and a
  last FIX record cut short by the end of file read as VMS reads it.
  Tagged and pinned by CLAUDE.md's procedure.

## Decisions

Proposed, for the author's review.

1. **FORRTL settles what the LRM leaves open,** as in Phase 80, and VMS's
   run of a probe is the requirement. Where the manual and FORRTL
   disagree (DEFINE FILE's organization, so far), FORRTL wins and the doc
   says so.
2. **The unit table:** a longword per unit from -4 to 119, the address of
   the unit's block, from LIB$GET_VM at the unit's first use: FAB, RAB,
   NAM and its buffer, XABFHC, the record buffer (RECL, at least 133 for
   formatted, 199 for input), and the unit's attributes (access, form,
   blank, carriage control, organization, record type, RECL in bytes,
   MAXREC, the associated variable's address, disposition, read-only,
   scratch, the record count for BACKSPACE, the next record for
   NEXTREC, the current record for REWRITE). The table and its blocks
   are the process's own (NOSHR).
3. **Each unit its own FAB, as FORRTL has it:** units 5, 6, and -1 to -4
   become ordinary units with the default names above, replacing Phase
   80's two shared streams. A `$` prompt must still stay open from one
   unit to another on the same terminal. If it doesn't, the fix goes in
   RMS's process-permanent stream, where VMS must keep it too, not back
   in FTNRTL. Phase 80's terminal tests hold this.
4. **One call per statement, keywords as pairs.** OPEN is
   `FTN$OPEN(control block, unit, code, value, code, value, ...)`: a
   keyword's code by value, then its value (a number by value, a
   character value by descriptor, ASSOCIATEVARIABLE's and USEROPEN's by
   address). CLOSE and INQUIRE are the same (INQUIRE's values are where
   to store, by descriptor or address). REWIND, BACKSPACE, ENDFILE,
   UNLOCK are `FTN$REWIND(block, unit)` and the like; DELETE and FIND add
   the record by value; DEFINE FILE is `FTN$DEF_FILE(unit, m, n, asv)`
   per file. Each returns the status in R0 as `FTN$IO_END` does (0, -1,
   or the FOR number) and signals when no specifier takes it, by the
   same code. The control block is Phase 80's (size, the ERR=/IOSTAT=
   flags, IOSTAT's place). New begin calls for the transfer statements,
   named as FORRTL names its own: `FTN$READ_SU`, `FTN$WRITE_SU`,
   `FTN$READ_DF`, `FTN$READ_DU`, `FTN$WRITE_DF`, `FTN$WRITE_DU` (the
   record by value), `FTN$REWRITE_SF`, `FTN$REWRITE_SU`. Our own
   interface, Decision 12's spirit, FORRTL's only in its names.
5. **Default names as FORRTL's** (the table above): FTNRTL translates the
   unit's logical name itself (`$TRNLNM`, the process's tables) and uses
   SYS$INPUT:/SYS$OUTPUT: when it isn't defined.
6. **Segmented records as FORRTL's**, the control word's bits as the
   trial shows them, the segment size whatever the probe shows.
7. **Direct access on a sequential file is a keyed `$PUT` with UIF,** as
   FORRTL's, and RMS learns it. What RMS does past the end of file is the
   probe's.
8. **Relative files in govax's RMS over ods2's block I/O,** their layout
   (prologue, cells, the control byte's bits, buckets, MRN, the end of
   file) taken from VMS-made files, not from guesses: the probe's RMS
   program makes them on VMS and copies them back, and the bytes are the
   specification.
9. **Odd-length FIX records are padded to a word,** if a VMS-made file
   confirms the guide: ods2's reader and writer and `rms/keyed.go`'s
   offsets change together.
10. **DISPOSE='PRINT' and 'SUBMIT'** keep the file and signal
    FOR$_IGNORED (3, a warning), and 'PRINT/DELETE' and 'SUBMIT/DELETE'
    keep it and signal FOR$_IGNNOTDEL (4): VMS's own conditions for a
    disposition not carried out. This replaces the design note's "report
    that it wasn't queued" with the message VMS has for it. Unconfirmed:
    when VMS gives them.
11. **SCRATCH** is a `$CREATE` with FOP TMD if the probe (FOR$RAB after
    an OPEN, or the trace) shows FORRTL uses it, else FTNRTL deletes the
    file at CLOSE and at exit. A scratch file's name is the probe's.
12. **The exit handler** is declared at the first unit's connection and
    closes every open unit with its disposition.
13. **Indexed files fail with FOR$_NOTIMP (2)** at OPEN (ORGANIZATION=
    'INDEXED', ACCESS='KEYED', KEY=, or an existing indexed file), and
    the keyed statements compile into begin calls (`FTN$READ_KF`,
    `FTN$READ_KU`, `FTN$DELETE` without a record on an indexed unit) that
    answer the same. Not INCFILORG: that is VMS's message for an
    organization that doesn't match the file, which would mislead.
14. **INQUIRE answers from the unit's block,** and for a file not open
    from RMS (`$PARSE`, `$SEARCH`, and an `$OPEN` with XABFHC, closed
    again) as the probe shows FORRTL doing.

## Questions for the author

1. **The VMS run.** A probe, `testdata/for81`, in two parts for the simh
   VAX: (a) an RMS program in MACRO making relative files and FIX
   sequential files by `$PUT` by RRN (past the end, UIF on and off, odd
   record sizes, deletes, updates), printing every status, with DUMP/BLOCK
   of the files and the files copied back on the exchange volume; (b) a
   FORRTL program through the entries we can call (implicit opens,
   DEFINE FILE, direct and unformatted READ and WRITE, segments, REWIND,
   BACKSPACE, ENDFILE, FIND, DELETE, the default names), printing each
   unit's FAB, RAB, and XAB fields through FOR$RAB. It would be subtask
   2, before any code. **Recommended.**
2. **FOR$OPEN's arguments.** OPEN, CLOSE, and INQUIRE are where most of
   the open questions are, and only `FOR$OPEN` can answer them. Its
   argument list (keywords encoded somehow) is in no manual on hand. (a)
   Find it by trial, as the namelist block was found: guard blocks whose
   access violations show which longwords FORRTL follows, with a limit on
   the effort (say, half a day). If found, the probe covers every keyword,
   USEROPEN's arguments, and INQUIRE. If not, those rest on the manual and
   are marked unconfirmed. (b) Don't try. **Recommended: (a).**
3. **Host files.** The phase's goal says volume files and host files, but
   RMS's services reach no host file today. A host file for RMS would
   need its record format kept somewhere (a host file has no FAT), so
   FIX, segmented, and relative files on the host would be govax's
   invention. (a) FORTRAN's files are volume files only; a name on no
   mounted volume fails as RMS fails it (DNR, so FOR$_OPEFAI or
   NO_SUCDEV), and a host file is brought in by COPY. (b) Also let RMS
   open a host file as STREAM_LF text, formatted sequential only. (c)
   Every organization on the host, in ODS-2's own byte layout, attributes
   chosen at open. **Recommended: (a)** for this phase, (b) as a later
   item if wanted. DUNGEON's data files would be put on a volume for
   Phase 86 (its direct access file is a FIX file of RECL longwords, which
   a host file can't describe).
4. **The 2005 HP Fortran LRM.** May it be used as DIGITAL's and VSI's
   manuals are, the later manual's additions treated as probe cases
   since it describes a later Fortran? **Recommended: yes,** as a
   secondary source.

## Subtasks (provisional, revised by the survey)

1. **Survey** (this Part A).
2. **The probe**, `testdata/for81`: the RMS program and the FORRTL
   program (with FOR$OPEN if question 2 finds it), and VMS's runs.
3. **RMS and ods2 groundwork:** odd FIX records, a FIX file's short last
   record, the organization nibble, FOP DLT/TMD/TMP/SCF/SPL, MRN and
   BKS, the keyed `$PUT` on FIX sequential files, and SET DEBUG RMS's new
   fields.
4. **OPEN and CLOSE** on sequential files, the unit table (Phase 80's
   streams folded in), default names, the exit handler.
5. **INQUIRE.**
6. **REWIND, BACKSPACE, ENDFILE.**
7. **Unformatted sequential I/O** and segmented records.
8. **Direct access**, DEFINE FILE, FIND.
9. **RMS relative files** (and ods2 as needed), held to VMS's files.
10. **FORTRAN on relative files:** DELETE, REWRITE, UNLOCK; indexed as
    NOTIMP.
11. **USEROPEN, SHARED, READONLY**, and the remaining keywords.
12. **End to end**, then the **close-out** (MANUAL.md's FORTRAN files
    section, FORTRAN-INTERNALS.md).

## Progress log

- **2026-10-29:** Planned with Phase 74. The author confirmed that ods2
  changes are fair game, and deferred indexed files to a later program.
- **2026-10-31, subtask 1, the survey.** Read chapter 9, chapter 7's
  unformatted, direct, and REWRITE statements, and A.2–A.3; the File
  Applications guide on relative files and record padding; what the
  compiler, FTNRTL, RMS, and ods2 have. Three trials of VMS's FORRTL
  over govax, traced with SET DEBUG RMS on a scratch volume as DUA2, show
  the default names (FORnnn, with SYS$INPUT:/SYS$OUTPUT: for units 5, 6,
  and -1 to -4 unless their logical names are defined), the segment
  control word, ENDFILE's ^Z record, the exit handler's closes, DEFINE
  FILE making a sequential FIX file, and direct WRITE as a keyed `$PUT`
  with UIF, which govax's RMS refuses. Found in passing: ods2's record
  reader stops on VMS's FORRTL.EXE, a FIX file whose end of file is
  mid-record. Part A has the rules, 14 decisions, 4 questions, and the
  revised subtasks. No code changed; nothing in the repository or the
  local volumes was written.
