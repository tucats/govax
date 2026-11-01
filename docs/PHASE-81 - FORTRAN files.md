# Phase 81 — FORTRAN files

**Status:** in progress: subtask 1, the survey, done 2026-10-31 and
accepted by the author the same day (Part A's answers); subtask 2, the probe,
done the same day (three VMS runs); subtask 3, RMS and ods2's groundwork, done the
same day; subtask 4, OPEN and CLOSE, the same day; subtask 5, INQUIRE, the same day; subtask 6,
REWIND, BACKSPACE, and ENDFILE, the same day; subtask 7, unformatted sequential I/O, the same day.
Planned 2026-10-29. Needs Phase 80 (formatting and the
error model). Part of the FORTRAN program laid out in `PHASE-74 -
FORTRAN plan.md` (Decisions 12, 13, and 21).

## Goal

FORTRAN's file statements and every access method this program
supports, on volume files (the author's answer 3: host files are
brought in and out by COPY, and RMS's full function is a container's):

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

**The author's answers (2026-10-31):**

1. They will run the probe on the simh VAX, in as many rounds as it takes:
   subtask 2, as recommended.
2. Find `FOR$OPEN`'s arguments by trial, **(a)**, with as many runs at it as
   it takes; the half-day bound is dropped.
3. **(a):** host files are imported and exported (COPY), and RMS's full
   function is for files in a container. FORTRAN's files are volume files.
   This narrows the goal's "volume files and host files".
4. Yes, the 2005 HP manual is a secondary source. govax emulates no one
   VMS compiler, so the feature sets may be mixed a little: what the 2005
   manual adds may be taken where it fits, a probe case where FORRTL can
   show it.

So Decisions 1 to 14 stand as proposed, Decision 1 with FORRTL first, the
1988 LRM next, and the 2005 manual after them.

## FOR$OPEN's arguments, found by calling them (2026-10-31)

Answer 2's trial, over govax with `SET DEBUG RMS`; no disassembly was involved. The
argument list is a list of keywords processed in order, the last of a code winning.
Each keyword is a longword, and some are followed by a value argument:

| Bits | Holds |
| ---- | ----- |
| 0-7 | the keyword's code |
| 8-15 | the mechanism |
| 16-31 | the value, for the immediate mechanisms |

| Mechanism | The value |
| --------- | --------- |
| 0 | 1: the keyword alone |
| 1, 2 | the high word, signed |
| 3 | a word, at the next argument's address (an output's width: a word) |
| 4 | the next argument itself: a value, or an address to store to or a descriptor's |
| 5 | a longword, at the next argument's address |
| 7 | a descriptor, at the next argument's address |
| 8 | seen to store a word, otherwise not understood; 9 looks like it |
| 10 | a byte, at the next argument's address |
| 6, over 10 | FOR$_INVARGFOR |

FOR$OPEN and FOR$CLOSE share these codes; 29 and up are INVARGFOR to them, and 27
and 28 to FOR$CLOSE alone.

| Code | Keyword | Values and what was seen |
| ---- | ------- | ------------------------ |
| 0, 25, 28 | not known | accepted, no effect seen by any value or mechanism, on sequential, direct, or relative units; FTNRTL needs none of them |
| 17 | ASSOCIATEVARIABLE | where to store, a word by mechanism 3 or 4, a longword by 5; untouched at OPEN, then the next record after each direct statement and FIND |
| 1 | UNIT | -1 and 120 are INVLOGUNI; missing, unit 0 |
| 2 | DISPOSE | 1 KEEP, 2 DELETE (FOP DLT), 3 PRINT (SPL), 4 SUBMIT (SCF), 5 PRINT/DELETE (SPL, DLT), 6 SUBMIT/DELETE (SCF, DLT), at `$CLOSE` |
| 3 | ERR= | not 0: a condition returns in R0 and isn't signaled |
| 4 | ACCESS | 1 DIRECT (FIX, unformatted, RAC KEY, ROP 00010612), 2 SEQUENTIAL, 3 APPEND (INCOPECLO with 'NEW'), 4 KEYED (as DIRECT, without UIF, on a new file), by comparing the values with the texts |
| 5 | FORM | 1 FORMATTED (RAT FTN), 2 UNFORMATTED (RAT 0) |
| 6 | RECL | MRS, times 4 if unformatted |
| 7 | CARRIAGECONTROL | 1 FORTRAN, 2 LIST (RAT CR), 3 NONE (RAT 0) |
| 8 | READONLY | bit 0: FAC 0, and INCOPECLO with STATUS 'NEW' |
| 9 | BUFFERCOUNT | RAB's MBF; an address-sized value is KEYVALERR |
| 10 | INITIALSIZE | FAB's ALQ |
| 11 | EXTENDSIZE | FAB's DEQ |
| 12 | NOSPANBLOCKS | RAT BLK |
| 13 | SHARED | SHR 0F |
| 14 | FILE, NAME | a descriptor; the unit's default name stays the DNA |
| 15 | STATUS, TYPE | 1 OLD (`$OPEN`), 2 NEW (`$CREATE`), 3 SCRATCH (`$CREATE`, FOP TMD), 4 UNKNOWN (`$CREATE`, FOP CIF); none is as NEW, so the compiler passes the default: 'UNKNOWN', or 'NEW' under `/NOF77` (the LRM's 9.1.25) |
| 16 | MAXREC | FAB's MRN |
| 18 | BLOCKSIZE | in bytes: RAB's MBC, rounded up to blocks |
| 19 | ORGANIZATION | 1 SEQUENTIAL, 2 RELATIVE (ORG 10, FIX), 3 INDEXED (ORG 20) |
| 20 | RECORDTYPE | 1 FIXED, 2 VARIABLE, 3 SEGMENTED (INCOPECLO if formatted), 4 STREAM, 5 STREAM_CR, 6 STREAM_LF |
| 21 | USEROPEN | the routine's address |
| 22 | IOSTAT | where to store: 0 on success, the FOR number on failure, by the mechanism's width; it stops the signal as ERR= does. Found after round 1 (over govax) |
| 23 | KEY | an address, followed only with ORGANIZATION='INDEXED'; INCOPECLO otherwise |
| 24 | BLANK | 1 ZERO, 2 NULL (INQUIRE's answers, #500 and #501); none is ZERO, so the compiler must pass the LRM's NULL |
| 26 | DEFAULTFILE | a descriptor; replaces the unit's default name |
| 27 | CONVERT | 0 NATIVE, 1 LITTLE_ENDIAN, 2 BIG_ENDIAN, 3 CRAY, 4 IBM, 5 VAXD, 6 VAXG, 7 FDX, 8 FGX, 9 NATIVE again; a text of those, else KEYVALERR; over 9 INVARGFOR |

FOR$OPEN also returns the condition value (INCRECLEN's 0018812C, FOR number 37, what
IOSTAT stores). On success OPEN, CLOSE, and INQUIRE return 1, and REWIND, BACKSPACE, ENDFILE,
UNLOCK, and FOR$IO_END 0.

FOR$INQUIRE takes the same input codes (1 UNIT, 3 ERR=, 14 FILE, 26 DEFAULTFILE), and
its questions are these. A number or logical goes where mechanism 3, 5, or 10 says;
a text's place is a descriptor's address by mechanism 4.

| Code | Question | Code | Question |
| ---- | -------- | ---- | -------- |
| 29 | not known: stores nothing, unit or file | 39 | FORMATTED |
| 30 | EXIST (true for an unopened unit) | 40 | UNFORMATTED |
| 31 | OPENED | 41 | RECL, in bytes for a formatted unit |
| 32 | NUMBER | 42 | NEXTREC |
| 33 | NAMED | 43 | BLANK |
| 34 | NAME | 44 | ORGANIZATION |
| 35 | ACCESS | 45 | RECORDTYPE |
| 36 | SEQUENTIAL | 46 | KEYED |
| 37 | DIRECT | 47 | CARRIAGECONTROL |
| 38 | FORM | 48 | CONVERT: on an unformatted unit its name, else UNKNOWN |

The other entries take the unit by value: `FOR$REWIND(u)`, `FOR$BACKSPACE(u)`,
`FOR$ENDFILE(u)`, `FOR$UNLOCK(u)`, `FOR$DELETE(u)`, and `FOR$FIND(u, rec)`,
`FOR$DELETE_D(u, rec)`, `FOR$WRITE_DU(u, rec)`, `FOR$READ_DU(u, rec)`. A USEROPEN
routine gets three arguments, by reference: the FAB, the RAB, and the unit. The FAB
holds the names and an XAB chain of an XABFHC and an XABSUM, with a NAM; since that is
what FORRTL gives `$CREATE`, the probe logs it, which on VMS is as good as the trace.

What this changes:

- **Decision 10 is withdrawn** by round 1: see below.
- **Decision 11:** SCRATCH is FOP TMD, as VMS's run confirms.
- **Decision 4:** FTNRTL's interface stays our own, but keywords as a code and a value
  are FORRTL's shape too, and its codes can be ours, which keeps the probe's tables
  directly usable.

Found in RMS and ods2 along the way, for subtask 3:

- `$CREATE` ignored FAB$W_DEQ, and wrote the file's 0 back over it, so EXTENDSIZE
  looked lost. It is now kept in the header (`rms/create.go`), and the console's tests
  pass with it. `SET DEBUG RMS` now also shows FOP, the sizes, MRN, NAM, the XAB chain's
  types, and the RAB's MBC and MBF.
- RMS has no `$READ`, `$WRITE`, or `$DISCONNECT`: a program calling one gets a reserved
  operand fault. The probe's block dumps need `$READ` to run on govax.
- `FAB$V_TMP` leaves a directory entry on govax, which it shouldn't (the probe's
  `TMPF.DAT`), and `$CREATE` returns no allocation in FAB$L_ALQ.
- **`ods2` refuses a name whose versions overflow its directory record** (about 60
  versions: "directory entries need 516 bytes, more than fit in one 512-byte block"),
  where VMS goes on in another record. A program that opens STATUS='NEW' in a loop would
  meet it.

## Round 1 on VMS (2026-10-31)

VMS 7.1 ran `FOR81R` whole and `FOR81` to case #580 of 583. That case was the probe's
mistake: `FOR$IO_END` with no statement active faults, and the handler's unwind faulted
again until the stack ran out. `vax/for81-vax.txt` is cut at that case (the image wrote
1.18 million lines of the same two). The block dumps failed too (`FB3` asked to share
writes with block I/O, RMS$_UPI), but the files are on the exchange volume, so
`vax/for81-blocks.txt` is their blocks through govax's DUMP, all-zero lines left out.

**Relative files [VMS's bytes].**

- **The prologue, VBN 1:** the longword at 68 is the first data VBN (2), at 6C MRN
  (7FFFFFFF for 0), at 70 the end of file VBN, the word at 74 the version (1), and the word
  at 1FE the sum of the block's other words.
- **A cell:** a control byte (08, a record is there; a cell never used is all 00), then
  for FIX the record, for VAR a word's length and MRS bytes, for VFC a word's length (with
  FSZ in it), FSZ bytes, and MRS bytes. A cell's unused bytes are 0.
- **Buckets:** cells don't cross a bucket, and a record's place is by whole buckets from
  VBN 2 (cells per bucket, 512 times BKS over the cell's size). BKS 0 is chosen as the
  smallest that holds a cell (MRS 10 is 1, 512 is 2).
- **`$CREATE`:** ALQ comes back 3 (a cluster), and the end of file is the block after the
  last, FFB 0, always. A FIX or VAR MRS of 0, or over the limit, is RMS$_MRS; RFM STM RMS$_RFM.
- **Statuses:** a missing record's `$GET` RNF (by key) or skipped (in sequence), then EOF;
  `$PUT` over one REX, with UIF an update; past MRN RMS$_MRN; key 0 KEY; a wrong size RSZ;
  `$UPDATE` or `$DELETE` with no current record CUR; `$TRUNCATE` FAC; `$FREE` with nothing
  held RNL.
- **Sequential `$PUT`** goes to the next record after the last one reached, REX if it's there.
- **RFA:** the record number, then 0.
- `$OPEN` writes MRN only for a relative file, and DEQ only if the file's isn't 0.

**Sequential files [VMS's run].**

- **FIX records pad to a word,** as the guide says: MRS 5 at offsets 0, 6, 12, MRS 513 at
  VBN 2, offset 2. Decision 9 stands.
- **Keyed `$PUT`** (Decision 7): RAC KEY on a FIX file. An existing record without UIF is NEF,
  with UIF replaced; past the end the file grows, the gap zeros, which then read as records.
  Key 0 KEY, a wrong size RSZ. `$GET` past the end EOF. On VAR RMS$_RAC.
- `$DELETE` IOP, `$TRUNCATE` of FAC-less FAC, as govax's. `$CREATE`'s ALQ comes back rounded to
  the cluster if asked, else 0. FOP CIF returns RMS$_CREATED, 00010619, if it made the file,
  and a found file's attributes in the FAB.
- FOP TMP and TMD leave no entry, and NAM's result name has no version (`P37.DAT;`). DLT
  goes at `$CLOSE`, from either call.
- **`$PARSE` with no NAM is RMS$_NORMAL,** or a bad name's own status (FORRTL's first call
  depends on it); govax says RMS$_NAM.
- **SPL and SCF with no queue running:** `$CLOSE` is NORMAL and the file stays, **even with DLT**.
  SCF queued a job (`SHOW QUEUE`: SYS$BATCH is stopped), SPL nothing, with no SYS$PRINT here.

**OPEN [VMS's run].**

- **Defaults:** a formatted unit's record is 133 (USZ, INQUIRE's RECL 85 hex). An unformatted
  segmented one's USZ is 2044, RECL 1FE; with RECL=n, 4n+2. VARIABLE 4n, FIXED 4n with MRS 4n,
  formatted FIXED MRS n. MRS is 0 for VAR at OPEN, whatever RECL says.
- **Limits:** a formatted RECL over 65535 or negative is INCRECLEN, FIXED 0 too, FIXED past
  32767 OPEFAI (RMS's MRS). Unformatted over 16383 INCRECLEN. Direct, as FIXED by its units.
  BUFFERCOUNT 0 to 127, else KEYVALERR; BLOCKSIZE 0 to 65024 or so (MBC, blocks up), else
  KEYVALERR; with a relative file also BLS, BKS. EXTENDSIZE to 65535, then KEYVALERR, -1 as 1.
  INITIALSIZE -1 as 1, and any non-0 sets FOP CBT (00200000).
- **Combinations:** direct needs RECL (INCRECLEN) and FIXED (INCOPECLO for VARIABLE,
  SEGMENTED); SEGMENTED is unformatted only; relative needs RECL, is FIXED by default, and
  may be VARIABLE, not STREAM; indexed with no KEY is INVKEYSPE; READONLY is INCOPECLO with NEW
  or SCRATCH, and with UNKNOWN opens only (FILNOTFOU if missing); SCRATCH with any DISPOSE but
  DELETE is INCOPECLO, and a FILE name is taken; READONLY with DELETE is INCOPECLO at OPEN or
  CLOSE. CC by FORM (FORTRAN/NONE), an explicit one kept even for unformatted, and an
  unformatted STREAM gets CR. KEY without indexed is INCOPECLO.
- **Texts:** any case, trailing blanks, not leading; unknown KEYVALERR.
- **Existing files (STATUS 'OLD'):** the file's RFM, RAT, and MRS win; FORM is taken from the
  keyword, default formatted, for a file with no carriage control too. RECL other than the
  file's in the same units, INCRECLEN (even FIX 20 unformatted, MRS 80, given 20 and no FORM:
  it is a formatted question); RECORDTYPE different, INCRECTYP; SEGMENTED formatted INCOPECLO;
  another organization INCFILORG.
- **NEW over a file:** a new version. **UNKNOWN:** CIF. **No STATUS:** as NEW. **Two opens of
  a file unshared:** OPEFAI. **SHARED:** SHR 0F, both fine. **READONLY:** FAC 0, SHR 0.
- **A connected unit:** the same file, or no name, changes **BLANK alone** and nothing else
  (not FORM, RECL, DISPOSE, STATUS), silently. Another file closes the first by its
  disposition, and if the new one fails, the unit is left closed.
- **USEROPEN:** three arguments by reference, FAB, RAB, unit, called with the names set, a
  NAM, and XABs FHC then SUM; `$CREATE` or `$OPEN` and `$CONNECT` are the routine's. Its status
  fails the OPEN. Modes 2 and 3 of the probe show what follows.

**CLOSE:** DISPOSE overrides OPEN's either way. Numbers and texts alike. INCOPECLO for SCRATCH
keep or print, and READONLY delete; a bad value KEYVALERR or INVARGFOR, **the unit left open**.
A unit not open, or twice: success, no call of RMS. An invalid one INVLOGUNI. **PRINT and
SUBMIT say nothing:** the file stays, status 1, so **Decision 10 goes**: FTNRTL sets FOP, and
RMS does as VMS's with no queue, keeping the file. FOR$_IGNORED and IGNNOTDEL weren't seen.

**INQUIRE:** always status 1, for a bad name or unit too (EXIST false). An unopened unit: EXIST
true, NUMBER its number, text UNKNOWN, NAME blank, numbers 0; invalid: EXIST false, NUMBER 0. By
file: found closed, EXIST and NAMED true, full NAME, NUMBER 0, its ORG and RT and CC and RECL,
SEQUENTIAL YES, DIRECT YES if FIX, ACCESS and FORM and BLANK UNKNOWN. Missing: EXIST false,
NAMED true, the parsed name. A wildcard or bad name: NAME as given, all false. An open file by
name answers as its unit. DIRECT is YES only for FIX or relative; KEYED NO. FORMATTED and
UNFORMATTED YES always when known. BLANK UNKNOWN when unformatted. NEXTREC 1 on opening direct.
Unit 5's name is `SYS$INPUT:.;`, unit 6's `SYS$OUTPUT:.;`; SCRATCH's `DUA2:[000000]FOR010.DAT;`,
NAMED false. RECL in longwords when unformatted, in bytes when formatted.

**Default names:** as the survey's trial, with USEROPEN: FNA FORnnn, DNA FORnnn.DAT for 0 to 119;
unit 5, 6 SYS$INPUT:, SYS$OUTPUT:. Units -3 and -4, written to, ERRDURWRI. NAMED.DAT by a FOR011
logical name.

**Unformatted records [VMS's bytes].** A control word first, 03 only, 01 first, 00 middle, 02
last, for SEGMENTED: segments of USZ with its word, so 2042 bytes of data by default, a zero
record just 0300. VARIABLE and FIXED no word, and over the size OUTSTAOVE; a short FIXED record
is padded (`vax/for81-blocks.txt`'s DIR1.DAT). Reads: fewer items is fine, more than the record INPSTAREQ, past the end
ENDDURREA. A segmented read of unsegmented data SEGRECFOR. STREAM keeps text with its CR
ending. Formatted against unformatted MIXFILACC. BACKSPACE steps a whole logical record.

**REWIND, BACKSPACE, ENDFILE:** REWIND and BACKSPACE of an unopened unit succeed without opening
it; ENDFILE opens it. Direct unit: REWERR, BACERR, ENDFILERR. Readonly: ENDFILE ENDFILERR. A write
after a read cuts the file there. ENDFILE writes `1A` records, read as ENDDURREA, and reading
goes on past them. BACKSPACE at the start does nothing; BACKSPACE of unit 6 works.

**Direct access:** OPEDEFREQ on a sequential unit, even FIX. Rec 0 is RECNUMOUT; rec -1 is
ERRDURREA reading and ERRDURWRI writing (RMS's); missing past the end ATTACCNON; a gap reads as zeros;
past MAXREC RECNUMOUT. Formatted pads with blanks, OUTSTAOVE over. A sequential statement
on a direct unit MIXFILACC. Direct on a VAR file INCRECTYP. Readonly write WRIREAFIL. FIND
sets NEXTREC, past the end ATTACCNON. NEXTREC is the record after the last. DEFINE FILE:
`FOR$DEF_FILE_W(u, m, n, asv)` (word), `FOR$DEF_FILE` (longword); FIX, MRS 2n, MRN m, RECL
in INQUIRE n/2; no file until the first I/O; its variable updates after each; on an open unit
UNIALROPE; twice DUPFILSPE; OPEN after one DUPFILSPE; m 0 or -1, n 0: accepted; n too big
(40000) INCRECTYP at define.

**Relative files through FORRTL:** direct as the RMS rules: a write over a record replaces it;
DELETE_D then a read ATTACCNON. REWRITE after a read, any VAR length, DELETE of the current. A
formatted file read as unformatted is MIXFILACC (the mistake that spoiled #572; round 2 repeats it).
Files' keywords: other organization INCFILORG, RECL INCRECLEN, type INCRECTYP.

**DELETE, REWRITE, UNLOCK:** on a sequential file DELERR, REWRITE after a read fine by a FIX or
same-length VAR file, else REWRITERR; a formatted short rewrite pads. UNLOCK fine. On a unit
not open: DELERR, UNLERR, and NO_CURREC.

**What needs round 2** (`for81c.mar`, 72 cases, and `for81d.mar`, 28): the true
ASSOCIATEVARIABLE code, by direct writes on each of codes 0, 17, 25, 28; IOSTAT by each width and
on CLOSE and INQUIRE; APPEND, which every "append" case above missed by using the value for
KEYED; relative files read in sequence, with the right FORM; a deleted cell left deleted, whose
control byte no file above shows; how a relative file grows (ALQ, DEQ, BKS, EOF) and BKS chosen by
MRS; record locks; CONVERT's values; the keyed statements' conditions; and what the exit handler
does with each disposition (`for81x.mar`).

## Round 2 on VMS (2026-10-31)

`FOR81D` (RMS) and `FOR81X` (image exit) ran whole. `FOR81C` ran away at its case #5: IOSTAT
by mechanism 10 follows the place's contents as an address, and unwinding that fault out of
FOR$OPEN leaves every later OPEN faulting in FORRTL's own frames, the same runaway as round
1's. `vax/for81c-vax.txt` is cut there; it shows IOSTAT by a word and by a longword, 37 on
INCRECLEN and 0 on success, and no signal with it. The program, without that case or the bad
KEY address, and with a guard that ends the image if the handler goes round 200 times, runs
again on its own volume (`exchangef.cmd`, `@FOR81F`), for `vax/for81f-vax.txt`.

**Relative files [VMS's bytes, `vax/for81c-blocks.txt`].**

- **A deleted cell's control byte is 0C** (the 04 bit added to 08), its record, and a VAR
  cell's length, left in place. `$GET` by key then RNF, in sequence passes over it, and
  `$PUT` into it needs no UIF, as into an empty cell. RSZ past MRS on `$UPDATE`, RSZ.
- **BKS 0 is chosen** as the fewest blocks that hold a cell: FIX MRS 511 (a cell of 512) is 1,
  512 is 2, 1023 is 2, 1024 is 3; VAR 509 (a cell of 512) is 1, 510 is 2; MRS 16000 is 32; 32255
  is 63. A cell past 63 blocks, or BKS too small or past 63, is RMS$_BKS, with an ALQ of 1 left
  in the FAB and the FAB's IFI 0, so the calls after it get IFI and ISI.
- **The file's allocation** is what the highest bucket written needs, rounded up to the
  cluster (3 here), never less than ALQ, rounded up too: `$CREATE`'s ALQ 1 or 2 comes back 3,
  20 comes back 21. A record 500 with MRS 100 ends at VBN 101, allocated 102; DEQ didn't
  add to it (record 1000: VBN 201, allocated 201). With BKS 63, one bucket to VBN 64, allocated 66.
- **The end of file** is always the block after the allocation, FFB 0, in the header and
  in the prologue's longword at 70: 202 for 201.
- **Locks, two streams with SHR on:** a `$GET` of a held record RLK, with NLK or NXR too; RRL
  reads it, RMS$_OK_RRL. `$RELEASE` frees it. No waits happened, with WAT not asked for.
- `$OPEN` gives back ORG, RFM, RAT, MRS, MRN, ALQ, DEQ, BKS.

**Image exit:** FORRTL closes every unit by its disposition: KEEP stayed, DELETE and SCRATCH
went, PRINT stayed (nothing queued), an implicit unit stayed (`FOR015.DAT`), and each file
holds its records. Decision 12 stands.

## Round 2's FORRTL program on VMS (2026-10-31)

`FOR81C` ran whole, on a volume of its own (`vax/for81f-vax.txt`). With it subtask 2 is done.

- **IOSTAT:** a word by mechanism 3, a longword by 5. It stores 0 on success and the FOR number
  on failure (FILNOTFOU 29, NO_SUCDEV 42, FILNAMSPE 43, INVLOGUNI 32), and no signal follows.
  CLOSE takes it too. INQUIRE always stores 0, for a bad unit or name as well.
- **ASSOCIATEVARIABLE is code 17** (the table above): after a write of record 3 it holds 4, a
  read of 1 holds 2, FIND of 2 holds 2. On a relative unit too.
- **APPEND (value 3):** STATUS 'OLD' or 'UNKNOWN' only; no STATUS, NEW, SCRATCH, or READONLY is
  INCOPECLO, and a missing file with OLD FILNOTFOU. ROP gains EOF (00010702) and FOP has no
  NEF (00000020). ACCESS answers SEQUENTIAL, so INQUIRE doesn't show APPEND. Writes go to the
  end, on VAR, FIX, relative, and unformatted files. BACKSPACE straight after opening an
  existing file and writing is BACERR; on a file UNKNOWN just created it works. REWIND then a
  read works, and a write there cuts the file, as on any sequential unit.
- **ACCESS=KEYED** on a sequential file is INCFILORG, and ORGANIZATION='INDEXED' on one too.
- **Relative files in sequence:** a read passes over what isn't there, and then ENDDURREA. A
  write goes after the last record reached, and is ERRDURWRI's RMS REX if there's one there.
  DELETE takes the current record (status 0); a second DELERR. REWRITE needs a current record,
  NO_CURREC at the end. **REWIND is REWERR, BACKSPACE BACERR, and ENDFILE ENDFILERR on a
  relative unit**, though RMS's own `$REWIND` works, so these are FORRTL's rule.
- **Direct on a relative file:** DELETE_D of a missing record ATTACCNON, of 0 or past MAXREC
  RECNUMOUT; FIND of a deleted record ATTACCNON, and then DELETE DELERR; REWRITE with no
  current record NO_CURREC.
- **The keyed statements:** OPEDEFREQ on a sequential unit. On a unit not open they do READ's
  implicit open, FILNOTFOU if there's no file.
- **Record locks:** a read of a record another unit holds is FOR$_SPERECLOC, at once, not a
  wait. FORRTL holds a read record until the unit's next operation or UNLOCK.
- **CONVERT:** the names in the table. The bytes of text are unchanged by any of them, which is
  all the probe wrote. FORM='FORMATTED' answers UNKNOWN.
- **Records:** a formatted write past RECL is OUTSTAOVE and writes nothing. A read with a RECL
  smaller than the record takes what it asks for, with no error.

**Where the Decisions stand.** 1 to 3, 5, 6, 11, 12, and 14 stand, settled by the runs above. 4
stands, with FORRTL's own codes (the tables above), so a keyword is one table from the compiler
to FTNRTL. 7, 8, and 9 are specified by the runs. **10 is withdrawn:** DISPOSE='PRINT' and
'SUBMIT' set FOP, and RMS keeps the file and returns success, as VMS's does when nothing is
queued; DELETE with them keeps it too. 13 stands for ORGANIZATION='INDEXED' and an existing
indexed file; ACCESS='KEYED' on anything else is VMS's INCFILORG or INCOPECLO.

**A question for the author: CONVERT.** It isn't in the 1988 LRM, and it is in the 2005 manual
and in VMS 7.1's FORRTL. (a) Take the keyword and answer INQUIRE, with NATIVE working and any
other name FOR$_NOTIMP when the OPEN is done. (b) All of it, converting unformatted data for
each format: real work (FDX, FGX, IBM, CRAY floating), and a phase of its own. (c) Leave it
out, as in the 1988 manual. **Recommended: (a).**

**The author's answer (2026-10-31): (a).** OPEN takes the keyword and INQUIRE answers it, NATIVE
works, and any other name is FOR$_NOTIMP when the OPEN is done. Subtask 11 does it.

## Subtasks (provisional, revised by the survey)

1. **Survey** (this Part A).
2. **The probe**, `testdata/for81`: done, the rounds above.
3. **RMS and ods2 groundwork:** odd FIX records, a FIX file's short last
   record, the organization nibble, FOP DLT/TMD/TMP/SCF/SPL, MRN and
   BKS, the keyed `$PUT` on FIX sequential files, `$PARSE` with no NAM,
   CIF's status, `$READ`, `$WRITE`, `$DISCONNECT`, ALQ returned, a name's
   versions past one directory record, and `TestFor81RMSOracle` holding
   `for81r.mar`'s FIX and FOP cases to VMS's run.
4. **OPEN and CLOSE** on sequential files, the unit table (Phase 80's
   streams folded in), default names, the exit handler.
5. **INQUIRE.**
6. **REWIND, BACKSPACE, ENDFILE.**
7. **Unformatted sequential I/O** and segmented records.
8. **Direct access**, DEFINE FILE, FIND.
9. **RMS relative files** (and ods2 as needed), held to VMS's files.
10. **FORTRAN on relative files:** DELETE, REWRITE, UNLOCK; indexed as
    NOTIMP.
11. **USEROPEN, SHARED, READONLY**, CONVERT (NATIVE only, the author's answer), and the
    remaining keywords.
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
- **2026-10-31, the review.** The author accepted the survey and answered
  its four questions (after Part A's questions): the probe in as many
  rounds as needed, `FOR$OPEN`'s arguments found by trial without a time
  bound, volume files only (host files by COPY), and the 2005 HP manual as
  a secondary source.
- **2026-10-31, subtask 2, the probe, round 1 written.** `FOR$OPEN`, `FOR$CLOSE`,
  and `FOR$INQUIRE`'s keywords found by calling them over govax (the section above).
  `testdata/for81`: `for81.mar`, 583 cases on FORRTL, and `for81r.mar`, 60 on RMS,
  from `gen.go`, each statement in its own frame so that a severe condition costs only
  it. `FOR81.COM` runs whole on govax over VMS's FORRTL.EXE, one output file, though
  many answers there are govax's gaps until VMS's run says what they should be.
  Waiting for that run.
- **2026-10-31, round 1's run, and round 2 written.** VMS 7.1's answers are the section
  "Round 1 on VMS". They correct the keyword table (BLANK's and ACCESS's values, code 22 is
  IOSTAT, code 27 and question 48 CONVERT) and withdraw Decision 10. `vax/` holds the run, and
  `vax/for81-blocks.txt` the files' blocks, read from the exchange volume. Round 2,
  `for81c.mar`, `for81d.mar`, and `for81x.mar`, runs whole on govax, waiting for VMS's run.
- **2026-10-31, round 2's run.** The section "Round 2 on VMS": deleted cells, BKS, how a
  relative file grows, locks, image exit. `FOR81C` ran away by the probe's mistake again, so
  it is to run once more, with a guard (`for81f.com`).
- **2026-10-31, subtask 2 done.** `FOR81C`'s rerun: ASSOCIATEVARIABLE is code 17, IOSTAT's forms,
  APPEND, relative files in sequence, the keyed statements, locks, and CONVERT (the section
  "Round 2's FORRTL program on VMS"). One question to the author, on CONVERT. Also from these
  runs: `$CREATE` keeps FAB$W_DEQ, and `SET DEBUG RMS` shows more of a FAB and RAB.
- **2026-10-31, subtask 3, RMS and ods2's groundwork.** `TestFor81RMSOracle`
  (`internal/console/for81rms_test.go`) holds `for81r.mar`'s cases on sequential files and
  FOP's bits, #0, #1, and #25 to #59, to VMS's run line for line; #2 to #24, relative files,
  wait for subtask 9. It masks only the channel VMS leaves in FAB$L_STV at `$CREATE` (govax
  opens none) and the stat lines' MRN, which that FAB gets from a relative file. Unit tests
  are `internal/rms/for81_test.go` and ods2's own.
  - **ods2** (committed and tagged v0.1.25 locally; not pushed or pinned, so govax builds only
    with `go.work` until it is): `RecordFormat.Organization` and `.Record` name the byte's
    nibbles, and `Reader` and `Writer` refuse a file that isn't sequential (`ErrNotSequential`).
    FIX records pad to a word (`FixedRecordSpace`); with `AttrNoSpan` a FIX or VAR record that
    won't fit goes to the next block, and a VAR one longer than a block is refused
    (`ErrSpansBlock`); `FixedRecordOffset` is a record's place by number. A name's versions go
    on in the next directory block past one's worth.
  - **Keyed `$PUT`** (`rms/keyedput.go`), with UIF, on FIX sequential files, gaps as zeros.
    VMS's "keyed then sequential" case shows a stream's place doesn't move: on a stream at the
    end of the file, a keyed `$PUT` that grows it leaves the next sequential `$PUT` going where
    the end was, and the file then ends after that record (`streamContext.heldEnd`). A direct
    WRITE never mixes the two, so FORTRAN doesn't meet this.
  - **FOP** (`rms/fop.go`): TMP and TMD leave no directory entry and a result name ending in `;`;
    TMD and DLT delete at the last deaccess; SPL and SCF keep the file, even with DLT, as VMS
    did with no queue running. MXV, DFW, NEF, OFP, and CTG on an empty file are accepted, and
    a `$CREATE`'s CTG stays in the FAB.
  - **Block I/O** (`rms/blockio.go`): `$READ`, `$WRITE`, `$DISCONNECT`, RMS$_UPI for a block
    `$OPEN` or `$CREATE` that lets others write without UPI.
  - **Smaller answers from the run:** `$PARSE` with no NAM is the name's status alone, and a
    directory after a name (`A[B`) RMS$_SYN; a failed `$GET` leaves RSZ 0, and RMS$_EOF or
    RMS$_KEY no RFA; `$UPDATE` of the wrong size on a FIX file is RMS$_RSZ before RMS$_CUR; FIX
    with MRS 0 is RMS$_MRS; `$OPEN` writes DEQ only if the file's isn't 0; a FIX file's XAB$W_LRL
    is its size; a `$PUT` refused leaves the longest record alone.
  - **Left for subtask 9:** FAB$L_MRN and `$CREATE`'s FAB$B_BKS mean something only on relative
    files, so they go with them. `$OPEN` already gives BKS back and leaves MRN alone on a
    sequential file, as VMS's does.
  - **Unconfirmed, govax's choices.** A last FIX record the end of file cuts short (FORRTL.EXE's
    own) reads as what the file has of it: the case meant to show it, #38, never made such a file,
    so a later probe round should. A sequential `$GET` after a keyed `$PUT` that moved the end is
    RMS$_EOF. RMS$_IOP for a record operation on a block stream and a block operation on a record
    stream (`vmsdef` has no RMS$_BIO or RMS$_BRO, which may not be VMS 7.1's names); a `$READ` stops
    at the end of the block holding the end of file; `$WRITE` zeros the rest of its last block,
    and `$READ` and `$WRITE` are tested only by unit tests, since VMS refused the probe's dumps
    with RMS$_UPI. RMS$_UPI is applied to FAB$V_BRO too.
- **2026-10-31, CONVERT.** The author answered the question left by subtask 2: (a), the keyword
  taken and INQUIRE answering it, NATIVE working, and any other name FOR$_NOTIMP at OPEN.
- **2026-10-31, subtask 4, OPEN and CLOSE.** `sema/fileio.go` (`FileStmt`, `FileKey`; a table of
  keywords and their codes; KEYTWICE, KEYNUMBER, KEYTEXT, KEYCHOICE, OPENCONFLICT, NEEDRECL) and
  `ir/lowerfile.go`; FTNRTL's unit table (`ftn_units.mar`, rewritten: Phase 80's two streams are
  gone, Decision 3), `ftn_open.mar` (FTN$OPEN, FTN$CLOSE; offsets 392 and 400, GSMATCH 1,10), the
  exit handler, the default names, and each statement's record in its unit's buffer (`CTX_RECP`,
  `CTX_UNITMAX`, `CTX_UNITBLANK`). `TestFor81OpenOracle` holds 170 of the probe's OPEN cases
  (texts, RECL's limits, combinations) to VMS's statuses, with the values in variables so that
  FTNRTL decides them; `testdata/fortran/files.for` is the rest, each IOSTAT VMS's;
  `TestFortranFiles` is image exit, `TestFortranOpenSignals` the signals. `SET DEBUG RMS` shows
  the FABs VMS's run has, but ROP LOC (below).
  - **OPEN takes more than this subtask's list,** because the rules are one chain: READONLY,
    SHARED, the sizes, BLOCKSIZE, BUFFERCOUNT, MAXREC, NOSPANBLOCKS, and every combination rule
    work now, on sequential files. **What is left:** ASSOCIATEVARIABLE= (subtask 8) and USEROPEN=
    (11) draw NOTIMPL at compile time; CONVERT= (11) isn't parsed yet; ORGANIZATION='RELATIVE'
    and a relative file found are FOR$_NOTIMP until subtask 10; so is indexed, Decision 13, with
    KEY= taken but not read. Direct and unformatted units open, and a formatted sequential
    statement on one is MIXFILACC; their statements are subtasks 7 and 8.
  - **Found by the oracle:** `$CREATE` of a FIX file with MRS past 32767 is RMS$_MRS (#205),
    now in `rms/create.go`.
  - **Each unit its own FAB has a visible result, as on VMS:** with `DEFINE/USER_MODE SYS$OUTPUT
    file`, PRINT and unit 6 each `$CREATE` a version of the file (FOR81.COM's comment says VMS's
    do). So `TestFor80Oracle` now defines FOR006, and `for80.for`'s markers are `WRITE (6,*)`, not
    PRINT. A program with a `$` prompt on one unit and the reply on another still works; RMS needed
    no change.
  - **Unconfirmed, govax's choices.** An implicit READ's `$OPEN` uses FAC 1F as the write does,
    with no retry read-only for a protected file; ROP LOC isn't set, FTNRTL having its own buffer;
    a read is limited to 199 bytes only on SYS$INPUT:, and elsewhere to the buffer, which holds
    the file's longest record as of the open, past which is INPRECTOO; list-directed output's
    record is 81 bytes or a shorter RECL, a longer RECL not widening it; the same file on a
    connected unit is by expanded names without versions (`FTN$$UNIT_SAME`); RMS$_DNF is OPEFAI
    for OLD as VMS showed for NEW, and which FOR number a failed name's other statuses get (FNM,
    DIR, WLD, TYP, VER: FILNAMSPE) beyond `A[B`'s; a failed OPEN's message names what RMS expanded;
    `STATUS=''`, which faulted in VMS's FOR$OPEN, is KEYVALERR, and a FORTRAN program can't write it
    as a constant; the compile-time rules (OPENCONFLICT, NEEDRECL) are FORRTL's run-time ones, as
    the survey proposed, since no VAX FORTRAN compiler is here to ask.
- **2026-10-31, subtask 5, INQUIRE.** `FTN$INQUIRE` (`ftn_inquire.mar`, offset 408, GSMATCH
  1,11), `sema/fileio.go`'s `inquireKeywords` and `fileAnswer`, `ir`'s `fileAnswer`.
  `TestFor81InquireOracle` holds 25 of the probe's cases (#474 to #509) to VMS's answers, every
  specifier; `testdata/fortran/inquire.for` is the rest, by these rules, and its output was
  read against them by hand. New messages: INQUIREBOTH, INQUIRENONE, KEYPLACE.
  - **It takes no context and can't fail,** as VMS's never reported one, so R0 is 0 and IOSTAT=
    stores it. Its unit is keyword 1, since it may ask by file. Decision 4 otherwise stands.
  - **What a fourth trial added, FORRTL over govax (`SET DEBUG RMS`; not kept).** A unit is not
    connected by INQUIRE: units 5 and 6 say OPENED false before their first use, or after a
    CLOSE, so the probe's #476 and #477 had connected them earlier. Units -1 to -5 are as an
    invalid unit, even once connected. By a name, FORRTL does `$PARSE` with default name `.DAT`
    (not the unit's FORnnn.DAT), `$SEARCH`, `$OPEN` with FAC GET, SHR 4F, a NAM and an XABFHC,
    `$CLOSE`, then a `$PARSE` with FAB$L_DNA 0, DNS not 0, and the NAM's strings cleared, ending
    the search. A closed file's RECL is MRS, or the longest record with none. `[NOSUCH]A.DAT` is
    `$PARSE`'s DNF, so NAMED false, as written.
  - **RMS:** that last `$PARSE` faulted over govax. A file name or default name it can't read is
    now RMS$_FNA or RMS$_DNA (`nameInputs.BadString`), and `$PARSE` leaves the NAM's answers.
  - **The unit's block** gained UB_SEGMENTED, an unformatted unit's with no other RECORDTYPE, and
    UB_NEXTREC, 1 on a direct unit (subtask 8 moves it). An existing VAR file opened unformatted
    answers SEGMENTED as a new one does (unconfirmed for an existing one); any other keeps its
    own type, and RECL is its record size in longwords.
  - **Left:** CONVERT= (11), which the parser doesn't take in INQUIRE yet; relative files' answers
    (#491, #492) with subtask 10; #507's NEXTREC moving, 8; #508's unit with no OPEN, 7.
  - **Open.** FORRTL's by-name NAME comes back blank over govax, where VMS's run shows the full
    name (#479), so something in RMS's NAM differs from VMS's. FTNRTL isn't affected, and this
    wasn't found; a later subtask running FORRTL may need it.
  - **Unconfirmed, govax's choices.** INQUIRE by both a unit and a file is a compile-time error,
    where FOR$INQUIRE answers for the file (#485; the LRM has only the two forms). An open file is
    found by its whole resultant name, so another version of it answers as closed. A file that
    exists but can't be opened answers EXIST and NAME only. Blanks after a name are dropped, as
    FORTRAN's character values need, where FORRTL over govax took `for013.dat   ` as a bad name,
    which may be govax's `$PARSE`. A closed VFC file's RECORDTYPE is UNKNOWN, an indexed file's
    KEYED YES and DIRECT NO, and DEFAULTFILE= alone with a name that won't parse leaves NAME blank.
- **2026-10-31, subtask 6, REWIND, BACKSPACE, and ENDFILE.** `ftn_position.mar` (`FTN$REWIND`,
  `FTN$BACKSPACE`, `FTN$ENDFILE`, offsets 416 to 432, GSMATCH 1,12), each `(control block, unit)`,
  begun by `FTN$$STMT_FILE` as OPEN is; sema's `fileStatement` and `ir`'s take the three as
  statements with only ERR= and IOSTAT=. `TestFor81PositionOracle` holds the probe's cases on
  formatted units (#543 to #545, #547, #549, #550; round 2's #21, #22, #26, #27) to VMS's
  statuses, reads, and records; `testdata/fortran/positions.for` is the rest, read against these
  rules by hand; `TestFor81PositionNotOpen` is #548; `TestFortranPositionSignals` and
  `TestFortranPositionProcedure` the messages. #546, unformatted, waits for subtask 7.
  - **A fifth trial, FORRTL over govax (`SET DEBUG RMS`; not kept),** showed how: REWIND is a
    `$REWIND`, its RMS$_IOP on a terminal or a procedure ignored (the place doesn't move, so a
    procedure's next line is read). BACKSPACE is the manual's: a `$REWIND`, then a `$GET` for each
    record before the one to go back to, so FTNRTL's record count (`UB_RECORDS`, which
    INVTEXREC's record number already used) is now the records before the unit's place: REWIND
    clears it, BACKSPACE takes one off, and a read at the end of the file adds nothing (two reads
    past the end, then BACKSPACE, reads the last record again). It refuses a device that isn't
    file-oriented (DEV$V_FOD: SYS$OUTPUT, SYS$INPUT, TT:, NL:), and a file opened APPEND that was
    there, until a REWIND (`UB_APPENDED`), as round 2's #21 shows. ENDFILE is a `$PUT`
    of `1A`, which RMS refuses on a READONLY unit (FAC, so ENDFILERR with -RMS-F-FAC); FORRTL itself
    refuses it on FIX records and on unformatted files that aren't segmented (VARIABLE, STREAM),
    and allows it on formatted STREAM, STREAM_CR, and STREAM_LF and on SYS$OUTPUT. A unit
    ENDFILE connects is `$CREATE`d with RAT 0, and INQUIRE answers FORM and BLANK UNKNOWN until its
    next statement gives it a form (VMS's #548); FTNRTL's `FTN$$UNIT_OPEN` takes 2 for this.
  - **A read of `1A` alone is ENDDURREA,** with no RMS status in the signal or ERRSNS, where an
    RMS end of file has -RMS-E-EOF (the trial's messages).
  - **BACKSPACE's second condition.** FORRTL's BACERR carries one more condition with no
    arguments: 0018884C for direct access or APPEND, 00188844 for a device that isn't
    file-oriented. VMS 7.1's `$GETMSG` has no text for them over govax (`-NONAME-F-NOMSG`), and no
    VMS run printed these messages, so their names are unknown; FTNRTL signals the same values
    (`CTX_REASON`, shown the same way). A refusal after an RMS failure (`$REWIND`'s IOP on the
    terminal) also carries RMS's status, as the trial showed.
  - **The signal's list of conditions RMS reports** is now a table in `ftn_statement.mar`
    (`RMS_REPORTED`), with REWERR, BACERR, and ENDFILERR in it.
  - **Unconfirmed, govax's choices.** That a relative unit's REWERR, BACERR, and ENDFILERR (round 2)
    come from the same test as a direct unit's, waiting for relative files in subtask 10; that
    BACKSPACE passes over a record too long for the buffer (RMS$_RTB) rather than failing; and the
    second condition's values for a relative unit.
- **2026-10-31, subtask 7, unformatted sequential I/O.** `FTN$READ_SU` and `FTN$WRITE_SU`
  (offsets 440 and 448, GSMATCH 1,13), begun as the other sequential statements are, and
  `ftn_unformatted.mar` (`FTN$$UNF_START`, `_ITEM`, `_END`): a WRITE's items' bytes into the unit's
  buffer, a READ's out of it, a segment written only when it is full and another byte comes, so a
  record that fills one exactly is an only segment. The compiler's side is `sema.UnformattedForm`
  (`unformatted`; UNFUNIT for an internal file or `*`) and `U` in `beginRoutine`.
  `TestFor81UnformattedOracle` holds the probe's cases #522 to #542 and #546 to VMS's statuses, the
  bytes each read returned, and every record's length and first 32 bytes, all matching;
  `TestFor81InquireOracle` now has #508, the unit with no OPEN; `testdata/fortran/unformatted.for`
  is the rest (every type, arrays, implied-DO lists, BACKSPACE over two segments, ENDFILE's record),
  read against these rules by hand; `TestFortranUnformattedSignals` is the messages.
  - **FTN_UNITS:** `FTN$$UNIT_FIND` and `FTN$$UNIT_OPEN` take 4 for an unformatted statement, which
    connects a unit with no OPEN as VMS's does (#508: unformatted, segmented, RAT 0, RECL 510);
    `FTN$$UNIT_RECORD` takes the statement's form, MIXFILACC for the other one; `FTN$$UNIT_READY`
    clears UB_SEGMENTED for a file that isn't VAR (an existing FIXED file opened unformatted).
  - **The unit's record count** is of logical records: a segment that doesn't end its record is
    taken off, and BACKSPACE's `$GET`s read a record's segments to its last (#540).
  - **#534, a STREAM file,** reads back without the record's first byte (a 0) on govax too, so it
    is in the oracle.
  - **Unconfirmed, govax's choices.** What is SEGRECFOR beyond VMS's one case (a first word of
    0100): any word over 3, a segment with no room for its word, a first segment where a record
    goes on, and a later one where one starts; a file that ends inside a segmented record is
    ENDDURREA. A unit ENDFILE connected, whose first statement is unformatted, becomes what a unit
    with no OPEN would (segmented, 2044 bytes), so its buffer is made that large to begin with.
    UNFUNIT is a compile-time error, since no VAX FORTRAN compiler is here to ask.
