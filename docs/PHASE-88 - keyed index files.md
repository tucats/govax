# Phase 88 — keyed index files

**Status:** planned 2026-11-03. Subtask 1, the survey, done the same day, and
reviewed and accepted by the author the same day ("The review", in Part B).
Subtask 2, the probe of RMS, done the same day, after three runs ("What round 1 settled",
in Part A). Subtask 3, `internal/isam`'s reader, done the same day ("What the reader settled", in Part A). Subtask 4, its writing without splits, done the same day ("What the writer settled", in Part A). Subtask 5, its growth, done the same day ("What the splits settled", in Part A). Subtask 6, its update and delete, done the same day ("What update and delete settled", in Part A). Subtask 7, RMS's `$CREATE`, `$OPEN`, `$DISPLAY`, and `$EXTEND`, done the same day ("What RMS's `$CREATE` settled", in Part A). Subtask 8, RMS's record operations, done the same day ("What the record services settled", in Part A). Subtask 9, DCL, done 2026-11-04 ("What DCL's round settled", in Part A). Subtask 10a, round 3 of the probe (the utilities), done 2026-11-04 ("What round 3 settled", in Part A). Subtask 10b, the FDL reader and CREATE/FDL, done 2026-11-04 ("What CREATE/FDL settled", in Part A). Subtask 10c, CONVERT, done 2026-11-04 ("What CONVERT settled", in Part A). Subtask 10d, ANALYZE/RMS_FILE, done 2026-11-04 ("What ANALYZE/RMS_FILE settled", in Part A). Needs nothing beyond Phases 1–87. It picks up what Phase 81 left (its Decision
13) and Phase 74's review deferred to a later program.

## Where things stand

Part A is the survey, Part B the accepted decisions and the author's answers, and
Part C the subtasks. `internal/isam` reads every Prolog 3 file VMS made, by every key, and makes files by `Create`, `Put`, and `PutLoad`,
splits and all: the probe's in-order loads, three index levels, root splits, duplicates past a bucket, and SIDRs past a bucket come
out as VMS's in every structure, and `Check` holds any file to the rules. `Update`, `Delete`, and `DropPointer` change and take out
records, the probe's twelve "changing" cases coming out as VMS's at every dump, ten of them to the byte. RMS makes and opens
indexed files through it (`rms/indexed.go`, `indexedxab.go`, `extend.go`): every `$CREATE` of the probe gives VMS's status, STV,
and FAB but where govax's Prolog 3 alone refuses what VMS made as Prolog 1 or 2, and `$OPEN` and `$DISPLAY` of VMS's own 77
Prolog 3 files report every key and area as VMS's did. The record services read and change them (`rms/indexedrecords.go`): `$GET`,
`$FIND`, `$PUT`, `$UPDATE`, `$DELETE`, `$REWIND`, by key, in sequence on any key, and by RFA, with ROP's options and record locks,
and `TestIdx88Oracle` holds 146 of the probe's 153 cases to VMS's run whole, the other seven listed with their reasons.
DCL reads and writes them (subtask 9): READ's `/KEY`, `/INDEX`, `/MATCH`, `/DELETE`, and `/NOLOCK`, WRITE, and WRITE/UPDATE through
the record services (`rms/dclkeyed.go`), TYPE, DUMP/RECORD, and DIRECTORY/FULL's prolog and keys, held to VMS's run of round 4 line for
line by `TestIdx88DOracle`. `testdata/idx88` holds VMS's runs, files, and definitions. ods2 needed no change. **Subtask 10,** the
utilities, has begun: round 3 of the probe (`idx88c.com`, CREATE/FDL, CONVERT, and ANALYZE/RMS_FILE) ran on VMS 7.1, its log and
the files it made in `vax/` ("What round 3 settled"). `internal/fdl` reads FDL files and CREATE/FDL makes files from them through
RMS's `$CREATE` (`rms/fdlcreate.go`, `dclcmd/createfdl.go`), `TestIdx88CCreateOracle` holding sections C2 to C5 to VMS's log and 42
files to VMS's byte for byte, the Prolog 1 files govax refuses left out. CONVERT reads and writes through RMS's services, and fast-loads through `isam`'s
`Load` (`rms/convert.go`, `dclcmd/convert.go`), `TestIdx88CConvertOracle` holding sections C6 to C10 to VMS's log and 60 files to VMS's, and `TestFastLoadAsVMS` 17 fast loads. ANALYZE/RMS_FILE writes all four reports (`isam/inspect.go`'s walk, `anl/rmsfile.go`'s layout, `dclcmd/analyzerms.go`), `TestIdx88ReportsOracle` holding 309 of round 1's reports to VMS's line for line, `TestIdx88CAnalyzeOracle` every such command of round 3, statuses and damaged files too, and `TestStatisticsAsVMS` the figures of 155 keys. **Next: subtask 11,** FORTRAN: the compiler.

## Goal

RMS's third file organization, on volume files, and FORTRAN's statements for it:

- **RMS:** `$CREATE` and `$OPEN` of an indexed file (FAB$C_IDX), with XABKEY,
  XABALL, and XABSUM; `$CONNECT`, `$GET`, `$FIND`, `$PUT`, `$UPDATE`, `$DELETE`,
  `$REWIND`, `$FREE`, `$RELEASE`, and `$DISPLAY`, by key (exact, generic,
  approximate), in sequence on any key, and by RFA; primary and alternate keys
  of every data type, ascending and descending, segmented, with duplicates,
  changeable values, and null values; compression; areas; record locks.
- **FORTRAN:** OPEN with ORGANIZATION='INDEXED', ACCESS='KEYED', and KEY=; the
  keyed READ (KEY=, KEYEQ=, KEYNXT=, KEYNXTNE=, KEYGT=, KEYGE=, KEYLT=, KEYLE=,
  KEYID=); WRITE, REWRITE, DELETE, and UNLOCK on an indexed unit; a sequential
  READ after a keyed one; INQUIRE's answers. FTNRTL does this through RMS, as
  VMS's FORRTL does.
- **The files are VMS's.** VMS's RMS reads and changes what govax makes, and
  govax's what VMS makes. VMS-made files and VMS's runs set the layout and the
  rules, as they did for relative files (Phase 81's Decision 8).
- **`ods2`**, the sibling project, may change as needed (the author, 2026-11-03).
  Part A's "ods2" section shows little need to.

# Part A — the survey (2026-11-03)

How sure each rule is, marked as in earlier phases:

- **[manual]** is from a manual below.
- **[bytes]** is read from a VMS-made file. It is one reading of real output, not
  VMS's definition, and it stands until the probe confirms or corrects it.
- **[open]** is unknown. The probe answers it (subtask 2).

## Sources

- **The *Guide to OpenVMS File Applications*** (7.3 text,
  `vms-file-applications-7.3.txt`): 2.2.3 (the organization, keys, key
  characteristics), 3.5 (prologs, index structures, records, areas, compression,
  fill factors), 7.3.5, 8.4.3 and 8.4.4 (processing, key matches, RFA), Table 8-3
  (a stream's context), 9.2.8, 10.1.6 (ANALYZE/RMS_FILE's walk of a file, which
  gives names and worked examples of a bucket, a data record, and an SIDR), Table
  2-2 (record overhead).
- **The *RMS Reference Manual*** (`OVMS_731_RMS.pdf`, 444 pages; now extracted with
  `pdftotext` when needed, not kept): chapter 14 (XABKEY), 9 (XABALL), 18 (XABSUM),
  4.5 (BKS), 7.8 to 7.10 and 7.19 (KBF, KRF, KSZ, ROP), `$CREATE`'s "Creating
  Indexed Files", `$CONNECT`, `$GET`, `$FIND`, `$PUT`'s "Inserting Records",
  `$UPDATE`, `$DELETE`, and each service's list of condition values.
- **The 1988 *VAX FORTRAN Language Reference Manual*** (the LRM): 7.1.1.7 and
  7.1.1.8 (the key specifiers), 7.2.3 (indexed READ), 7.3.3 (indexed WRITE), 7.4
  (REWRITE), 9.1 (OPEN; 9.1.15 KEY), 9.7 (DELETE), 9.8 (UNLOCK). HP's 2005 manual is
  the secondary source, as Phase 81's answer 4 has it; on keys it adds nothing here.
- **The *Record Management Utilities Reference Manual*** (OpenVMS AXP 1.5 and VAX 6.0, 1993,
  AA-PV6QA-TK; text `vms-rmu.txt`, an OCR, beside its PDF; the author, 2026-11-04): ANALYZE/RMS_FILE
  (chapter 1), CONVERT (2), CONVERT/RECLAIM (3), the File Definition Language with every section and
  attribute (4), CREATE/FDL (5), and EDIT/FDL with FDL's validity rules (6.1.1). A DIGITAL manual, so
  usable as the others are. The DCL Dictionary only points to it for these three commands.
- **The *Utility Routines Manual*** (`OVMS_731_UTILITY_ROUTINES.pdf`): CONV$ (chapter 6, with CONVERT's
  condition values and its statistics array) and FDL$ (chapter 11).
- **The DCL Dictionary:** READ's `/KEY`, `/INDEX`, `/MATCH`, `/DELETE`, `/NOLOCK`;
  WRITE's `/UPDATE`; F$FILE_ATTRIBUTES's KNOWN, NOK, NOA, PVN.
- **VMS's own files, read only.** The local volumes hold real indexed files, and
  this survey read their blocks through govax's DUMP on volumes mounted read-only:

  | File | Keys | Notable |
  | ---- | ---- | ------- |
  | `testdata/backup69`'s INDEXED.IDX | 1 string | made by VMS 7.1's CONVERT from an FDL file we wrote, so it's ours to quote |
  | RIGHTSLIST.DAT | BN4, 2 strings | named keys; one index still unmade; SIDRs without duplicates |
  | SYSUAF.DAT (the author's permission, 2026-11-03; a development VAX, nothing secret) | string at position 4, BN4, 2 BN8 | 3 areas; SIDRs with duplicates |
  | VMSKITBLD.IDX, DECW$KITBLD.IDX | 2 strings | a one-level index over 84 and 95 data buckets, full compression |
  | MSGHLP$LIBRARY.MSGHLP$DATA | string, IN4 | 8,729 blocks, 42,492 records, 3 areas, 63-block buckets |
  | LMF$LURT.DAT | BN2 | |
  | VMS$VMS.DAT, VMS$IMAGE_VERSION.DAT, VMS$LAYERED.DAT | strings | **real RRVs and deleted records** |

  All are Prolog 3. Their contents aren't quoted here and nothing was copied;
  licensed files stay off the repository as always. LMF$LICENSE.LDB wasn't opened.

**The clean-room line.** No VMS source, and nothing of RMS's internals from memory:
Part A's layout comes only from the Guide's examples and those bytes. VMS's own names
for prolog structures (BKT$, IRC$, KEY$, AREA$) aren't in `vmsdef`, and only the
handful the Guide prints are used. Question 5 asks about the rest.

## What's there today

**RMS (`internal/rms`).**

| Need | Today | Where |
| ---- | ----- | ---- |
| `$CREATE` with FAB$C_IDX | **no:** RMS$_ORG | `create.go:301` |
| `$OPEN` of an indexed file through DCL's OPEN, TYPE, DUMP/RECORD | **no:** RMS$_DEV, or "not a sequential file" (tried on INDEXED.IDX) | ods2's reader, `dclfile.go` |
| XABKEY | a type code and a length of 0: accepted in a chain, then ignored | `xab.go` |
| XABSUM | filled with NOA 0, NOK 0, PVN 0 | `xab.go:279` |
| XABALL | one area's ALQ and DEQ at `$CREATE` | `xab.go:434` |
| `$FIND`, `$GET`, `$PUT`, `$UPDATE`, `$DELETE` routes | a branch to relative's routines (`handle.relativeRecords()`), the seam indexed files join | `get.go`, `put.go`, `recordops.go` |
| A stream's current and next records | yes, sequential and relative | `stream.go` |
| Record locks by RFA | yes | `recordlock.go`, Phase 47 |
| ROP's KGE, KGT, NXT, REV, LIM, LOA, CDK, FDL | not read | |
| `$EXTEND`, `$SPACE` | **no service at all** | `corevms/rms.go`'s table |
| RAB$B_KRF, RAB$B_KSZ | KSZ for relative numbers only | `relative.go:475` |
| SET DEBUG RMS's trace of KRF, KSZ, ROP's key bits, the XABKEY chain | names the types only | `trace.go` |
| FAB$C_IDX, XAB$C_*, XAB$M_*, RAB$V_KGE and the rest, RMS$_DUP, CHG, SEQ, OK_DUP, OK_IDX, OK_LIM, RRV, PLG, IRC | **all in `vmsdef`** | |
| `$XABKEY`, `$XABSUM` macros | yes; `testdata/mar/rms`'s `init_xabkey_all.mar` already holds `$XABKEY` to real MACRO's object | `starlet.mar` |

**DCL and the utilities.** DIRECTORY/FULL already shows an indexed file's attributes;
BACKUP saves and restores one (block for block, `testdata/backup69`); COPY copies one
block for block. F$FILE_ATTRIBUTES's NOK, NOA, and PVN answer 0, KNOWN false. READ has
no `/KEY`, `/INDEX`, or `/MATCH`, and WRITE no `/UPDATE`. There's no CONVERT, CREATE/FDL,
EDIT/FDL, or ANALYZE/RMS_FILE.

**ods2.** `RecordFormat.Organization` names the organization nibble, including
`OrganizationIndexed`. `File.ReadBlock`, `WriteBlock`, `SetEndOfFile`, `WriteAttributes`,
and `volume.Extend` are what `rms/relative.go` already builds on, and an indexed file
needs the same. `RecAttr` holds the bucket size. Expected ods2 changes are small: at most
allocation by area, so that areas can be contiguous, and whatever subtask 6 finds.

**The compiler** (Phases 76 and 81).

- `ast.KeyList` reads OPEN's KEY=; `sema/fileio.go` takes it (`keysKeyword`) and
  doesn't use it. KEY= with another organization is already refused.
- `sema/io.go`'s `keySpecs` compiles KEY=, KEYEQ=, KEYGE=, KEYGT=, and KEYID=, and
  reports the rest NOTIMPL. **Its comment is wrong:** it says KEYLE=, KEYLT=, KEYNXT=,
  and KEYNXTNE= aren't the 1988 manual's. They are, in 7.1.1.7.
- `ir/lowerio.go` already makes `FTN$READ_KF`/`_KU` calls: key descriptor, match, key
  number. WRITE, REWRITE, DELETE, and UNLOCK compile as for relative files.

**FTNRTL.**

- `ftn_open.mar` answers FOR$_NOTIMP for ORG_INDEXED, asked for or found (two places).
- `ftn_statement.mar`'s `FTN$READ_KF`/`_KU` exist and connect the unit, then OPEDEFREQ or
  NOTIMP (`50$`).
- `ftn_inquire.mar` already answers INDEXED and KEYED.
- `ftn_record.mar`'s DELETE and UNLOCK assume relative; `ftn_position.mar` refuses keyed
  access.
- GSMATCH is 1,17.

**FORRTL's interface, as Phase 81's trials left it.** FOR$OPEN's code 23 is KEY: "an
address, followed only with ORGANIZATION='INDEXED'", INCOPECLO otherwise; indexed with no
KEY is INVKEYSPE. ACCESS value 4 is KEYED ("as DIRECT, without UIF"). `FOR$READ_KF`,
`_KU`, and `_KO` exist, and their arguments are unknown. A USEROPEN routine sees the FAB
and its XAB chain, so **the XABKEYs FORRTL builds for each KEY= can be logged over govax,
before RMS has indexed files**, which is subtask 2's first trial.

## The file on disk

These are the survey's main result. A **bucket** is RMS's unit of I/O: 1 to 63 blocks,
its size set per area. Offsets below are in hex.

**Evidence, computed by scratch scripts over the 12 files with contents.** A walk of each
primary key's data buckets, using the record layout below, landed exactly on each bucket's
free-space offset in 8 of the files, over **46,155 records**. It failed only at RRVs, in the
2 files that have them, so that form is still [open]. The other 2 files hold one deleted
record and no data. All 370 data buckets on those chains have matching first and last bytes
and end their chain as the table says, and every key and area descriptor block's sum is right.

### The prolog [bytes]

Virtual block 1 starts with key 0's descriptor. Later keys' descriptors chain on from it.
The file-level fields at 66 on share their places with a relative file's prologue (Phase
81), though only the version and the sum are the same fields:

| Offset in VBN 1 | Size | Holds |
| ---- | ---- | ---- |
| 66 | byte | VBN of the first area descriptor |
| 67 | byte | number of areas |
| 74 | word | prolog version: 3 |
| 1FE | word | sum of the block's other 255 words (the same rule in each key descriptor block and each area descriptor block) |

### A key descriptor [bytes]

The XABKEY's fields, kept per key, descriptors 66 hex bytes apart within a block:

| Offset | Size | Holds |
| ---- | ---- | ---- |
| 00 | long | VBN of the next key's descriptor, 0 for the last |
| 04 | word | its offset in that block |
| 06, 07, 08 | bytes | area of the index levels, of the lowest index level, of the data level (IAN, LAN, DAN) |
| 09 | byte | level of the root (0: no index yet) |
| 0A, 0B | bytes | index and data bucket sizes, in blocks (IBS, DBS) |
| 0C | long | root bucket's VBN (RVB). With no index yet, it holds the areas instead (bytes: DAN, LAN, IAN) [open] |
| 10 | byte | flags, below |
| 11 | byte | data type: XAB$C_'s values (seen 0 STG, 2 BN2, 3 IN4, 4 BN4, 7 BN8) |
| 12 | byte | number of segments (NSG) |
| 13 | byte | null character (NUL) |
| 14 | byte | total key size (TKS) |
| 15 | byte | key of reference (REF) |
| 16 | word | minimum record length to hold this key (MRL) |
| 18, 1A | words | index and data fill sizes (IFL, DFL) |
| 1C | 8 words | segment positions |
| 2C | 8 bytes | segment sizes |
| 34 | 32 bytes | key name (KNM), blank or zero filled |
| 54 | long | first data bucket's VBN (DVB), 0 with no index yet |
| 58 | 8 bytes | each segment's data type |

**The flags.** Bits 0, 1, 2 are XAB$V_DUP, CHG, NUL. Bits 3, 6, 7 sit where XAB$V_IDX_NCMPR,
KEY_NCMPR, and DAT_NCMPR do, **but read the other way round**: set means compressed, as
VMSKITBLD's flags (C9), its compressed records, and RIGHTSLIST's numeric primary key (81)
show. Bit 4 means the index hasn't been made yet (RIGHTSLIST's key 1, flags 5F, root 0).

### An area descriptor [bytes, names from the Guide's Example 10-5]

64 bytes, starting in the block byte 66 of VBN 1 names:

| Offset | Size | Holds |
| ---- | ---- | ---- |
| 02 | byte | area number |
| 03 | byte | bucket size |
| 06, 07 | bytes | alignment; flags (bit 5 CBT, bit 7 CTG; the Guide's bit list; seen 20) [open which is which] |
| 0C | long | current extent's first VBN |
| 10 | long | its blocks |
| 14 | long | blocks used |
| 18 | long | next free VBN |
| 32 | word | total allocation [open] |

The Guide's "Default Extend Quantity" is somewhere in here too [open].

### A bucket [bytes, names from the Guide's Example 10-6]

14 bytes of header, then records up to the free-space offset:

| Offset | Size | Holds |
| ---- | ---- | ---- |
| 00 | byte | check character; **the bucket's last byte is the same value** (seen 0, 0E, 12, 14, 20; how it changes is [open]) |
| 01 | byte | the Guide calls it "Area Number", but RIGHTSLIST's key 2 buckets hold 2 in a file of one area, so it looks like the key of reference [open] |
| 02 | word | VBN sample: the low word of the bucket's VBN |
| 04 | word | free-space offset |
| 06 | word | next record ID (the Guide's "Free Record ID Range") |
| 08 | long | next bucket's VBN at this level; the chain's last points back to its first (a ring), so one bucket points to itself |
| 0C | byte | level: 0 data, then up to the root |
| 0D | byte | flags: bit 0 last in chain, bit 1 root (the Guide's BKT$V_LASTBKT, ROOTBKT) |

### A data record, Prolog 3 [bytes, checked by the walk]

| Part | Size | Holds |
| ---- | ---- | ---- |
| control | byte | low 2 bits: pointer size less 2; bit 2 deleted, bit 3 RRV, bit 4 no pointer (the Guide's IRC$V_DELETED, RRV, NOPTRSZ) |
| ID | word | the record's ID in this bucket |
| RRV ID | word | the ID its RFA names |
| RRV VBN | (control's low bits)+2 bytes | the bucket its RFA names |
| size | word | the bytes that follow. On fixed-length records with no compression it should be absent (Table 2-2's 2 bytes' difference) [open] |
| key | | the primary key, **always first, wherever it is in the record** (SYSUAF's is at position 4) |
| data | | the rest of the record, without the key |

So a record's **RFA is (RRV VBN, RRV ID)** (the Guide's 8.4.4 and Example 10-6): it stays
the same when a split moves the record, the old place keeping an RRV that forwards it. An
RRV's own form is [open]; the walk's guess, control, ID, pointer, is wrong.

**Compression:**

- **A compressed key** is a byte of length, a byte of front count (characters shared with
  the bucket's previous key), and the bytes that differ. The key's last byte stands for
  the rest when it repeats: a 7-byte key `BBASE  ` is `06 00 BBASE⎵`. This is the Guide's
  front and rear compression, and it applies to string keys of 6 bytes or more.
- **Compressed data** is fragments: a word's length, that many bytes, and a byte of how
  many more times the fragment's last byte repeats. The Guide says runs under 5 aren't
  compressed.

### An index bucket [bytes]

Keys and pointers are apart. The keys run up from offset 0E, each compressed as above
when the index is, and fixed-size otherwise. **The VBN pointers run down from the bucket's
end, last key's pointer nearest the end.** The bucket's last 4 bytes are a word (the offset
below the pointers: free space ends there), a byte of 0, and the check character. Pointers
here were 2 bytes each; what makes them wider is [open]. The last key in each level is the
high key: FF bytes for a binary key, a one-byte FF rear-compressed for a string.

### An alternate index's data level: SIDRs [bytes]

The Guide's "secondary index data records". Each is a word's size, the key, and then an
array of pointers, one per record with that key value:

    0B 00 | 04 00 01 00 | 82 03 00 04 00 00 00
    size    key (BN4)     control, ID, VBN

`82` is a pointer's control byte (bit 7 and pointer size 4); pointer size 2 appears as
`80`. Other control bits (the Guide's IRC$V_KEYDELETE, bit 5) mark pointers to deleted
records [open]. The Guide's Example 10-7 shows a record ID on the SIDR as well, which
these Prolog 3 files don't have [open].

### Unused bytes aren't defined

VMSKITBLD's second key descriptor block has leftover text past its descriptor, and the sum
still comes out right. So **byte-for-byte comparison can't be the whole test**; unused bytes
must be ignored (Question 3).

## What round 1 settled (2026-11-03)

VMS 7.1 ran `testdata/idx88`'s 154 cases to their end with no condition, and ANALYZE/RMS_FILE/CHECK found 0 errors
in every file but E85 (below). `vax/idx88-vax.txt` is the run; `vax/idx88.bck` the 140 files' survivors; `vax/anl` VMS's
report, check, and FDL of each; `defined.txt` the 246 names of `$BKTDEF`, `$IRCDEF`, `$KEYDEF`, `$AREADEF`, and
`$PLGDEF` (values only). Where this section and "The file on disk" differ, this one wins.

**The names confirm Part A's offsets,** and from here on the code may use them: `KEY$` for the descriptor (its `KEY$C_BLN` is 60
hex, with `KEY$W_IDXFILL`, `KEY$W_DATFILL`, `KEY$W_MINRECSZ`, `KEY$L_LDVBN`, `KEY$B_TYPE0`), `AREA$` (`AREA$L_CVBN`, `CNBLK`,
`USED`, `NXTVBN`, `AREA$L_NXT` at 1C and `NXBLK` at 20 for an extent to come, `AREA$W_DEQ` at 24, `AREA$L_TOTAL_ALLOC` at 32,
`AREA$W_CHECK` at 3E), `PLG$` (`PLG$B_AVBN`, `AMAX`, `PLG$W_DVBN` at 68, `PLG$L_MRN` at 6C, `PLG$L_EOF` at 70, `PLG$W_VER_NO`
at 74, `PLG$W_GBC` at 76, and collating's `PLG$W_COLVBN`), `BKT$`, and `IRC$`.

**Answers to Part A's [open]s:**

| Question | Answer |
| -------- | ------ |
| A bucket's byte 1 | The key of reference (`BKT$B_INDEXNO`; ANALYZE says "Key of Reference"), overlaid by `BKT$B_AREANO`: the Guide's example was only ever key 0 |
| The check character | A count of the bucket's writes: 1 after the first, 4 after 3 `$PUT`s and a `$DELETE`. Bytes 0 and last alike |
| An index's pointer size | `BKT$M_PTR_SZ`, bits 3 and 4 of the bucket's flags, less 2; 2 bytes in every case so far |
| An RRV | Control `0A` (bit 3, pointer size 4), ID word, RRV ID word, pointer: 9 bytes, `IRC$C_RRVOVHSZ3`. So a moved record is `02 01 00 02 00 04 00 00 00...` in its new bucket, with RRV (4,2), and `0A 02 00 01 00 08 00 00 00` in the old, "ID 2 is ID 1 in VBN 8". That's where Part A's walk went wrong |
| A deleted record with no RRV to keep | Gone: the bucket's records close up, and its RFA is RMS$_DEL. The next `$PUT`, even of the same key, gets a new ID: 3,4 after 3,2. IDs aren't reused |
| Bytes past the free offset | Left as they were, stale records and all, like VMSKITBLD's leftovers: not to be compared |
| Fixed-length records | No size word with data compression off (`02 ID RRVID VBN key data`); with it on, the word and fragments are there |
| Control `80`, `82` on SIDR pointers | `IRC$M_FIRST_KEY`, bit 7, with the pointer size in the low bits. `IRC$M_NOPTRSZ` and `NODUPCNT` are one bit, 4 |
| An index not made yet | `KEY$M_INITIDX` (10 hex) until its first record; the root field holds the areas meanwhile, as Part A read |
| FLG's compression bits | Inverted on disk, as Part A read: `KEY$M_KEY_COMPR`, `IDX_COMPR`, `REC_COMPR` |
| ROP REV | **In VMS 7.1,** by Tables 7-3 and 7-4, and the three combinations that are RMS$_ROP are too. Decision 9's doubt is gone |
| Default bucket size | 2 for a small record; MRS 5000 is 10. VMS picks more than one record's worth, the formula for subtask 7 to fit from the shape cases |
| Allocation | `$CREATE` returns ALQ 3 with none asked for, and `$OPEN` later 9 with one key, 15 with two: 3 at a time by bucket size 2; ALQ 50 is 51 (a multiple of 3, the cluster here). Replayed in subtask 4 |
| DFL under half the bucket | Taken as half, as the manual says (200 and 300 show 512 of 1024) |
| An in-order load | A full bucket is left alone and a new one takes the next record: no records move, and the ring's next pointer goes to the new one |

**`$CREATE`'s statuses, one bad field at a time:** no XABKEY or key 1 alone RMS$_NPK (STV the XAB's address for the second); keys 0 and 2, two key 0s, XABALLs 0 and 2 or AID 1 alone RMS$_IMX (STV the XAB); CHG or NUL on the primary, or FLG all ones,
RMS$_FLG; a primary past MRS or a FIX shorter than the key RMS$_POS; size 0, BN4 of 2, PAC of 17, or 256 in all RMS$_SIZ; DTP 9 or 200 RMS$_DTP; DAN, IAN, LAN 3 with no XABALL RMS$_DAN, IAN, LAN; DFL, IFL past the bucket
RMS$_DFL, IFL; a bad KNM address RMS$_KNM, **which leaves a file with an empty prolog** (E85, the one `/CHECK` error); BN2 of two segments RMS$_SEG; BKS 64 RMS$_BKS, XABALL BKZ 64 RMS$_BKZ; BKS 1 and MRS 600, FIX MRS 0, MRS 32234 RMS$_MRS; VFC, STM, UDF RMS$_RFM; PROLOG 9
RMS$_PLV. Taken: an alternate before the primary in the chain, MRS 32225 (a lower prolog), CIF over a file. PROLOG=1, PROLOG=2, and overlapping primary segments make Prolog 1 and 2 files (pvn 1, 2), which answer 1 makes an error of ours.

**The record services:** `$PUT` is RMS$_SEQ out of order by RAC SEQ, RMS$_DUP, or the success RMS$_OK_DUP where a key's duplicate was taken; `$UPDATE` RMS$_CHG (the primary, an alternate with no CHG, or a record too short for one with no CHG, and one with it if it's short of another),
RMS$_RSZ (too short for the primary), RMS$_CUR (no current record, and after a failed `$GET`), RMS$_DUP, RMS$_OK_DUP; `$GET` RMS$_RNF, RMS$_KSZ, RMS$_KRF, RMS$_KBF, RMS$_ROP, RMS$_RTB (USZ short), RMS$_DEL (by a deleted record's RFA), RMS$_RLK, RMS$_OK_RRL, RMS$_OK_LIM, RMS$_OK_DUP (CDK);
`$TRUNCATE` RMS$_FAC with FAC lacking TRN; `$DELETE` RMS$_CUR. Each case's exact order is the run's, for `TestIdx88Oracle`.

**Still to be fitted from the run, not asked of VMS again:** where a split falls when a record lands mid-bucket; continuation buckets for a long duplicate chain; SIDR pointers across buckets; how index compression treats a split; and an area's extension. The block dumps after each step hold all of these, and
subtasks 3 to 6 replay them. A case the dumps don't answer can go in a later round.

**The later rounds move to their subtasks,** since each is best written with its code at hand: FORRTL's (round 2) before subtask 11, the utilities' (round 3) before 10, and DCL's (round 4) before 9.

## What the reader settled (2026-11-03)

Subtask 3's tests read the probe's 103 Prolog 3 files and the 27 indexed files on the local system disk (178,472 records, no error). Where this section and the ones above differ, this one wins.

- **An index bucket's pointers:** the *first* key's is nearest the bucket's end, not the last's as "An index bucket" above has it. ANALYZE's walks (`idx88w2.txt`, 2 entries) and every search through an index say so.
- **The last entry of a level's last bucket** is passed to, never compared: it's FF bytes, which isn't the highest value of a signed or descending key.
- **A generic REV,** which the probe's list asked: REV+KGE of `P00001` is `P0000100`, the *first* record answering, not the last. REV+KGT of it is `P0000099`, the last before them. The manual's "a duplicate set found by REV gives only its first record" isn't tried by any case on a whole key's duplicates, so `findBackward` has that rule from the manual alone.
- **A SIDR's pointers:** the value's very first has control `80` and the rest `00`, with the pointer size in the low bits. A value past one bucket repeats its key in the next, whose first pointer is `00` (`G114`): `IRC$M_FIRST_KEY` marks the value's start, not the SIDR's.
- **Packed keys** order by value, both ways (`F122`'s key 5, `S22`, `S30`). All the probe's are positive, so the sign rule is the VAX Architecture's, unconfirmed on a key.
- **The size word's rule is partly unconfirmed.** A fixed file with nothing compressed has none, and one with both compressed has one (`S13`, `G109`; `S12`). Fixed with only one of key and data compressed isn't in the probe; `fixedStorage` gives it a size word.
- **`IRC$M_NOPTRSZ`** on a data record or a SIDR pointer appears in no file here. The decoder takes it to mean no VBN follows, which is unconfirmed.
- **ROP FDL's pointers** still aren't seen: `U134` (case 144) and every system file have no pointer to a deleted record, though `$DELETE` with FDL should leave them, and that case's reads by alternate key said RMS$_RNF. An entry whose `Record` is `ErrDeleted` is therefore possible only in theory so far. Subtask 6 settles it with `U134`'s dumps, and RMS must skip such an entry.

## What the writer settled (2026-11-03)

Subtask 4 replays the probe's `$PUT`s on `Create`'s files. The 61 Prolog 3 "key shapes" files (cases 1 to 65, every key type, flag, and area layout) and each step of the ten growth cases up to its first split come out
**the same as VMS's to the byte**, unused bytes too. So answer 3's decoded comparison is met and more, for now; a later subtask may fall back on `Dump` where a stale byte's origin isn't worth following. Where this section and the ones above differ, this one wins.

- **Data compression scans a longword at a time,** from the start of the fragment being built: a run is seen only if a whole longword at that stride is one byte, and then it's measured both ways. That's why a run of 6 is compressed or not by where it falls (one starting a byte past a stride holds no whole longword), which looked like no rule at all until `fragoff` was tabulated. The Guide's "runs under 5" holds: 5 is the shortest, which only the local system disk's files show (`SYSUAF.DAT` and others fail at 6; the probe's fail at 4). `CheckCompression` recompresses every key and record of a file, and every file here passes.
- **A run past 256 bytes is unconfirmed.** A byte's count allows 255 repeats, and `compressData` starts a new fragment there, but no file here has so long a run (setting the limit to 200 or to none changes nothing). The VMS round can try one.
- **A level's last index entry is the key's highest value by its type,** not FF bytes: `7F` as the top byte of a signed key, `9999999C` for packed, and for a descending key the lowest (zeros, a top byte of `80` when signed, `9999999B` packed). Compressed like any key. "An index bucket" above has only the ascending unsigned form.
- **A new key's bucket order:** the level 0 bucket, then the index bucket over it. A level 1 area apart from the index area makes the root a level 2 bucket over a level 1 bucket (`S63`). Index buckets start with a check character of 0, level 0 buckets get 1 at their first record.
- **Allocation:** area 0 starts at VBN 1, its first extent holding the prolog; the others follow in order, each rounded by the volume. What's left of an extent too small for a bucket stays unused, and the next extent is a new one even when contiguous. `Extend` asks for the larger of the bucket and DEQ; a 3-block cluster hides which VMS asks for, so that's unconfirmed.
- **Order within a bucket:** a record with an equal key goes after those there. A SIDR's new pointer goes last in its array. The next key is stored again against the new one, **before** the new record goes in for a data bucket and **after** it for a SIDR, which VMS's stale bytes show (`S6`, `S39`).
- **A failed `$PUT`:** a primary duplicate is refused with nothing written and no ID spent. An alternate's is found after the record went in: it comes out again, the bucket counting two writes and an ID spent, the old bytes left past the free offset (`S41`). Taking out earlier alternates' pointers has no VMS case, since none has two alternates failing at the second: `TestBackOutOfAlternates` holds only that the file reads as before.
- **The prolog:** a named key's name is blank filled, an unnamed one's zeros. Keys 1 on start at VBN 2, `66` hex apart. VMS's files show 3 in a block; 5 is as many as fit, the next block past that being unconfirmed.
- **Pointers:** a data record's is always 4 bytes, an index's and a SIDR's 2 here. Wider ones for VBNs past 65,535 are `pointerBytes`'s guess, unconfirmed. So is an unmade key's root field's byte order, DAN, LAN, IAN, when the areas differ: no dump catches one.
- **Left to later subtasks:** `Spec`'s flags are the descriptor's, so an XABKEY's mapping onto them (which keys compress: strings of 6 bytes or more, data only on the primary) and the default bucket size are subtask 7's, as are RMS's checks of a `Spec`. `TestShapesAsVMS` reads its `Spec` from VMS's own descriptors.

## What the splits settled (2026-11-03)

Subtask 5 replays the growth cases (104 to 119) through every split, and the bulk loads to their end. Where this section and
the ones above differ, this one wins.

- **A new bucket goes to the right.** The old bucket keeps its VBN, so a key's first data bucket never changes, and the new
  one takes the old one's place in the ring and its last flag.
- **An append doesn't split** when the bucket's last record is also the newest it was given: the record gets a bucket to
  itself and nothing moves, so a load in key order leaves full buckets (G104; G113 for a duplicate key). When the last record
  isn't the newest, a record past it splits the bucket like any other (G108's 16 after 8, 9, 12, 15).
- **Where a split falls:** where the two sides' bytes differ least, the new record counted, and the RRVs the split itself
  leaves on the left counted, those already there not; a tie goes to the place further right. G118's scattered load is what
  tells these apart; the Guide only says "half".
- **IDs and RFAs:** records staying keep their IDs. The new record, if it stays, takes the bucket's next ID; if it moves, the
  old bucket spends none on it. The new bucket's records are numbered from 1 in key order, and the new record's RFA is its
  place there (G106).
- **RRVs:** a record leaving the bucket its RFA names leaves an RRV, after those already there. A record that moved before
  and moves again has its RRV rewritten where it is; each such bucket is written once per split, which its check character
  shows (G106, G118). RRVs stay in their bucket for good, so a bucket can end up mostly RRVs (G105's VBN 3).
- **The index:** the level above gets an entry for the left side's new high key, keeping the pointer to the old bucket, and the
  entry that pointed there now points to the new bucket.
- **Continuations:** a run of one key going past a bucket goes on in the next with no index entry (G113). A SIDR alone in a
  full bucket goes on in a new bucket with its key repeated, its first pointer `00` (G114). A SIDR that's the last of several
  moves whole, its new pointer with it (G115).
- **An index split** leaves on the left as many old entries as take no more than half the bucket's room, and the new entry too
  if it falls among them: 24 and 26 of 50 (G115), and a compressed bucket cut by bytes (G116). An index bucket is full when
  its keys would reach its pointers, not only pass them (G116's VBN 539). A root that splits gets a new root over the two,
  allocated after the new bucket, check character 1 (G115's VBN 59).
- **ROP LOA** (`PutLoad`): a record past a bucket's last stops at the key's data fill size, and an index bucket at its index
  fill size; later `Put`s fill past them (G119).
- **What matches.** Each step of G104 to G113 is VMS's by `Dump`, and byte for byte until its first split; past it VMS's
  stale bytes beyond a free offset aren't followed (answer 3). G114, G115, G116, and G119 are VMS's by `Dump`. G118 (500
  scattered records, no compression) has VMS's data buckets, records, IDs, RRVs, and check characters in VMS's order, and its
  index entries and values; a few of its SIDR buckets split elsewhere, which moves VBNs after them. **G117** (the same,
  compressed) differs: one write more in VBN 4's check character, then split places from its 30th data bucket on. Neither is
  explained, so neither file is held to VMS's structure, only to `Check` and the same values by each key. The VMS round, or
  a probe case of its own, can look again.
- **Unconfirmed, no case here reaching them:** a record that no split leaves room for gets a bucket of its own between
  the two halves (`alone`; records of one size never need it); a SIDR filling a bucket with SIDRs after it splits the bucket
  first; index pointers wider than 2 bytes; extension past an area's extent is `allocate`'s, unchanged.

## What update and delete settled (2026-11-03)

Subtask 6 replays the probe's "changing" cases (138 to 149, files U128 to U139) on files made by the same `$PUT`s, each
`$UPDATE` and `$DELETE` with VMS's status, and holds the file to every dump the log has of it: all twelve by `Dump`, and the
ten with no split byte for byte, stale bytes past the free offsets too (`TestChangesAsVMS`). Where this section and the ones
above differ, this one wins.

- **An update is in place.** The record keeps its offset, ID, RFA, and control byte; its bytes after the key are rewritten,
  and the rest of the bucket moves up or down to suit (U128: 100 bytes, then 250, then 60, in the same bucket, which counts a
  write for each). Alternates whose values didn't change aren't touched: their buckets' check characters stay.
- **An update too long for its bucket splits it** like a `$PUT`, the record in its place among the others and keeping its RFA
  (U137: 150 bytes grown to 390 beside one other record; it went right with ID 1, the RRV in the bucket its RFA names pointed
  at it there, and the level above got an entry for the left bucket's new high key).
- **A changed alternate:** the new pointer goes in first, last among the new value's, and then the old one comes out
  (U130's stale bytes are the new SIDR's, moved down over the old SIDR's place).
- **The statuses, all before anything is written:** a different primary key, an alternate without CHG changed, or a record
  too short for an alternate without CHG are RMS$_CHG (`ErrKeyChanged`), even when another key had CHG (U131); too short for
  the primary key is RMS$_RSZ (`ErrRecordSize`); a new value a key without DUP already has is RMS$_DUP; one a key with DUP
  has is RMS$_OK_DUP. A null value given a real one with CHG enters the index (U135).
- **A deleted record's bytes close up,** and if keys are compressed the next record's key is stored again against the one
  before, **after** the record comes out (U136's stale bytes, for a SIDR; a data record's order is taken to be the same,
  unconfirmed). The bucket's next ID doesn't go back, so the RFA's ID is missing: RMS$_DEL.
- **The last record left in a bucket stays as a stub:** its control byte with `IRC$M_DELETED`, ID, RFA, a size word, and its
  key, no data (U137's only record, and each of U139's buckets, emptied one record at a time). Whether a bucket keeps its last
  record *by key* or only its last one left, no case tells apart: `lastLeft` takes "only", unconfirmed. A fixed file with no
  size word can't hold a shorter record, and no case deletes one there: its stub keeps the whole record, marked. A `$PUT` of a
  stub's key takes its place in the same write, with a new ID (U139).
- **A record a split moved,** deleted, takes its RRV with it: the RRV in the bucket its RFA names closes up (U137).
- **A SIDR's pointer comes out,** and the SIDR with it if it was the only one. A value's first pointer passing its
  `IRC$M_FIRST_KEY` to the next, and a SIDR bucket left empty, are unconfirmed: no case takes out a first pointer with others
  after it, or empties a SIDR bucket.
- **ROP FDL,** answering "What the reader settled": VMS 7.1 left the pointers, and the probe's later `$GET`s by each alternate,
  finding them, took them out and said RMS$_RNF, as the Guide's 8.2.4 says RMS must. That's why no file showed one.
  `Delete(rfa, true)` leaves them; `DropPointer` is RMS's taking one out, which subtask 8 calls when a search finds one;
  `Check` allows a pointer to a record whose ID was given and is gone; and a value held only by such pointers isn't a
  duplicate (`valueHeld`), the Guide's "find that the record no longer exists".
- **Found by the randomized test** (`TestChangesAgainstAModel`, 3,000 mixed puts, updates, and deletes in eleven shapes), a
  fault of subtask 4's, not VMS's: RRVs can sit between records, a record put past a bucket's last going after them, so the
  record whose key is stored again after a removal is the next one *with a key*, not the next entry.
- **ods2** needed nothing.

## What RMS's `$CREATE` settled (2026-11-03)

Subtask 7 runs the probe on govax (`TestIdx88CreateOracle`, on a volume of VMS's 3-block clusters) and holds every `$CREATE` of
cases 1 to 103 to VMS's, with the FAB the probe printed after it, and each refusal case whole; and it restores VMS's files from
the save set and opens each as the probe's `OPN` did (`TestIndexedOpenAsVMS`, in `internal/rms`): all 77 Prolog 3 files' FAB,
XABSUM, XABKEYs, and XABALLs are VMS's. Case 2's file, made with no records, is VMS's to the byte. Where this section and the
ones above differ, this one wins.

- **The order of the checks** is read off what the FAB holds after each refusal. A key's faults (RMS$_NPK, IMX, DTP, FLG, SEG,
  SIZ, POS) and PLV, BKS, BKZ, RFM, and FIX's MRS 0 leave FAB$B_BKS and FAB$L_ALQ as they were. The XABALLs write ALQ as each
  is read (E100: 4, then RMS$_IMX at AID 2), and then the whole with area 0 holding at least the prolog (E79 to E81: 2). The
  keys' areas are checked next (DAN, IAN, LAN), then BKS is written, then the record's size against the primary key's data
  bucket (RMS$_MRS) and each fill size (DFL, IFL). KNM is read only as the prolog is written, after the file exists, which is
  why E85 is a file with an empty prolog; govax's `$OPEN` of such a file is RMS$_PLG.
- **STV:** the key of reference for DTP, FLG, SEG, SIZ, POS, DAN, IAN, LAN, DFL, IFL, KNM, and KSI ("for key = !UL" in each
  message), the area for BKZ. RMS$_IMX's is an XAB's address: for two XABKEYs of one key, the second in chain order (E70); for
  a gap, **the key before it** (E69: key 0's, for keys 0 and 2); for a XABALL out of order, **the primary key's XABKEY** (E99, E100),
  which no rule explains. RMS$_NPK's is the first XABKEY, 0 with none. A success's STV is 0 in govax where VMS's was A0, the
  channel; the oracle masks it, as earlier phases' did.
- **Flags:** an XABKEY's DUP, CHG, and NUL go to the descriptor as they are. A string key of 6 bytes or more has its key and
  index compressed unless the XABKEY says not; no numeric or packed key, and no shorter string, is (S3, S4, S16 to S30). Only
  the primary key's records are compressed (an alternate's FLG always shows DAT_NCMPR, S35). `$DISPLAY`'s FLG is the
  descriptor's with the three compression bits inverted, KEY$M_INITIDX (10 hex) as it is (S2, S47).
- **The default bucket** is the smallest that holds one record of MRS bytes, and at least 2 blocks (S54's 10 for MRS 5000; 2
  for every smaller record). A Prolog 3 bucket of b blocks holds a record of b×512−32 bytes (S55's 32224 in 63 blocks); 9 more
  fit VMS's Prolog 1 (E92), which govax refuses with RMS$_PLV, and past that it's RMS$_MRS (E91, E93). That FIX records have
  the same limit is unconfirmed.
- **The header:** an indexed file's bucket size and default extension are area 0's (S62's 3; S64's DEQ 4, S66's 2), its record
  size MRS for FIX and 0 for VAR, and **its end of file 0** until RMS extends it past its first allocation (E71, E84, S2, S56,
  S64, S65), when it's the block after the allocation; `$CLOSE` leaves it so. `$READ` reads every allocated block whatever
  the end of file (S2's dump). A XABALL's CTG or CBT makes the file contiguous, which `$OPEN`'s FOP shows and `$CREATE`'s
  doesn't (S64, S65).
- **govax's choices, by answer 1:** PROLOG=1 or 2, a primary key whose segments overlap, and a record only a lower prolog would
  hold are RMS$_PLV where VMS quietly made Prolog 1 or 2 (S34, S59 to S61, E92); `$OPEN` of a Prolog 1 or 2 file is RMS$_PLV.
  Collating keys are RMS$_DTP (Decision 7).
- **Unconfirmed, no case reaching them:** an alternate key past MRS is RMS$_POS as the primary is; an alternate's overlapping
  segments are taken; RMS$_KSI when an index bucket can't hold two keys of 4 bytes' overhead each; no XABALL's area takes AOP
  from FOP's CTG or CBT; a REF past the file's keys at `$OPEN` or `$DISPLAY` is RMS$_REF, an AID past its areas RMS$_AID, each
  with the number as STV.
- **`$EXTEND`** (the RMS manual's description): FAB$L_ALQ, or each XABALL's ALQ for its area of an indexed file, which becomes
  the area's current extent if it has none, or else the extent to come (AREA$L_NXT, NXBLK), used when the current one is
  (`isam.File.ExtendArea`); an ALQ of 0 is RMS$_ALQ, no room RMS$_FUL, a file not opened to write RMS$_FAC. ALQ's and STV's
  outputs, and the end of file afterwards, are unconfirmed.
- **Found on the way:** a `$CREATE` or `$OPEN` failing after the file was accessed released its IFI but not the access, so the
  file stayed held (RMS$_FLK at the next `$OPEN`); `abandon` closes it.

## What the record services settled (2026-11-03)

Subtask 8 runs the whole probe on govax and holds every case to VMS's run (`TestIdx88Oracle`): 146 of the 153 cases are VMS's
line for line, statuses, RABs, records, walks, and blocks, a block dump that isn't VMS's byte for byte being the same file by
`isam.Dump`. The seven left are the five Prolog 1 and 2 refusals (answer 1) and G117 and G118 ("What the splits settled").
Where this section and the ones above differ, this one wins.

- **The key of reference.** A stream follows one key in sequence: `$CONNECT` and `$REWIND` take it from RAB$B_KRF, a keyed `$GET`
  or `$FIND` makes it the key searched, and an RFA access the primary key. A sequential `$GET` ignores RAB$B_KRF (case 125: a keyed
  `$GET` by key 3, then sequential `$GET`s with KRF 0 went on by key 3). `$CONNECT` with ROP EOF and KRF 1 was taken (case 137),
  though the manual says RMS$_ROP.
- **The stream's place** is the record it reached last, by key and RFA: a successful `$GET` or `$FIND` of any kind moves it there,
  and a sequential `$GET` right after a `$FIND` reads the found record. A failure leaves it (case 131: a keyed `$GET` past the
  end, RMS$_RNF, then the record after the last one read). After a `$DELETE` the stream goes on from the record after the deleted
  one by the stream's key (case 146); govax works that out before the record goes, since isam can find only what follows a gone
  record's *key*, which would pass over its other duplicates (`indexedStream.queued`). That a keyed `$FIND` moves the place as a
  `$GET` does is unconfirmed: no case tells it from leaving it.
- **What a service leaves in the RAB.** A failed `$GET` or `$FIND` leaves RFA 0 and a `$GET` RSZ 0 (RNF, RLK, EOF, KSZ, KRF,
  KBF, ROP, DEL); RMS$_RTB leaves the RFA, RSZ the part stored, and STV the record's size (case 130). A failed `$PUT`, `$UPDATE`,
  or `$DELETE` leaves both as they were (cases 133, 139 to 141, 153). A successful `$DELETE` leaves RFA 0. A successful `$UPDATE`
  that split its bucket leaves **STV 1** (case 147), why isn't known; `isam.PutResult.Split` reports it.
- **Keyed access** (cases 120 to 129): ROP's KGE, KGT, and REV as the manual's Table 7-4, RMS$_ROP for REV alone and for KGE
  with KGT. A string key's KSZ may be 1 to its size, a generic search; 0 or more is RMS$_KSZ. Any other key's must be 0 (the whole
  key) or its size, packed decimal too. KRF past the keys is RMS$_KRF before KSZ is looked at; a KBF RMS can't read RMS$_KBF.
- **ROP LIM** compares the record's key, by KSZ bytes, with KBF's, and a different one is the success RMS$_OK_LIM with the record
  returned and the stream past it (case 130: two in a row). **ROP CDK** is RMS$_OK_DUP when the *next* record by the key has the
  same value: of three duplicates, the first two (case 130).
- **`$PUT` in sequence** is refused RMS$_SEQ for a primary key below the stream's last sequential `$PUT`, by the key's own order
  (case 136, descending), and an equal one goes on to RMS$_DUP (case 133). After `$CONNECT` with ROP EOF the last is the file's
  highest key (case 137). That `$REWIND` and `$CONNECT` forget it, and that a `$GET` doesn't change it, are unconfirmed.
- **`$PUT` with UIF** over a record that's there is an update of it, its RFA in the RAB, RMS$_RLK if another stream holds it
  (cases 142, 153). **`$TRUNCATE`** without TRN access is RMS$_FAC (case 132); with it, RMS$_IOP, unconfirmed.
- **Locks** are by RFA as on the other organizations (Phase 47), and VMS's answers to another stream's lock are the same:
  RMS$_RLK for a plain `$GET`, NLK, REA, NXR, and TMO without WAT, RMS$_OK_RRL with RRL (case 151); ULK, `$RELEASE`, and
  `$FREE` as before (case 152).
- **A pointer a fast delete left** is taken out when a search reaches it, and the search goes on (`findLive`, `nextLive`), so case
  144's later `$GET`s by each alternate were RMS$_RNF and its walks one record short, as VMS's.
- **The header.** Once RMS extends an indexed file its end of file is the block after the allocation (VMS's DIRECTORY/FULL: "Size:
  9/9"), and the longest record stays 0 ("longest 0 bytes").
- **`$FAO`'s `!AF`** kept the DEC Multinational graphic characters (BA to FE in the records) and showed periods for 00 to 1F, 80
  to 9F, and FF, where govax's showed a period for every byte past 7E; `faoPrintable` is VMS's now, 7F and A0 unconfirmed.
- **Not done:** `$SPACE`, which govax has for no organization (it's block I/O, not a record service); the subtask list's mention
  of it was the survey's guess.

## What DCL's round settled (2026-11-04)

Round 4 (`testdata/idx88`'s `idx88d.com`, sections D1 to D11) ran on VMS 7.1, `vax/idx88d.log`, and `TestIdx88DOracle` holds
govax's run to it line for line, times masked. Where this section and the ones above differ, this one wins.

- **WRITE** is a `$PUT` by key: records came in any order (D1). Its `$STATUS` is RMS's success status, RMS$_OK_DUP (`%X00018011`)
  for a record whose alternate key another has, not always RMS$_NORMAL; WRITE/UPDATE's likewise (D6, D9).
- **OPEN/APPEND** is `$CONNECT` with ROP EOF, after which the *first* `$PUT` must be past the file's highest primary key, by any RAC
  (RMS$_SEQ), and later ones may come in any order: on an empty file, M001 then A001 were both taken (D10). `indexedStream.fromEOF`.
- **`/MATCH=G`** is CLI$_ABKEYW ("ambiguous qualifier or keyword"), `/MATCH=LT` CLI$_IVKEYW, both with the keyword as segment. A
  qualifier with no value it needs is CLI$_VALREQ, the qualifier as segment (`/MATCH`, `/INDEX`): the grammar engine (`dcl/parse.go`)
  now says so for every qualifier, where it used govax's own NEEDQUALIFIERVALUE.
- **`/KEY`** on a binary key takes a string of the key's size as its bytes, and another length is RMS$_KSZ; an empty or long key
  RMS$_KSZ; a key of reference past the file's RMS$_KRF, and `/INDEX` stays the failed one (D3, D4). `/KEY` on a sequential file is
  RMS$_RAC, and `/INDEX` alone changes nothing there (D5).
- **READ/DELETE** puts the record in the symbol even when its `$DELETE` fails (RMS$_FAC on a file opened to read, RMS$_IOP on a
  sequential file), and when it works leaves `$STATUS` RMS$_NORMAL, the `$DELETE`'s, not CLI$_NORMAL (D5, D7).
- **Locks:** a READ locks its record on a file shared for writing, another stream's READ of it is RMS$_RLK, `/NOLOCK` reads it anyway
  (ROP NLK and RRL), and a WRITE/UPDATE of a record read that way is RMS$_RNL. **`$UPDATE` gives up the record's lock** (the other
  stream read it at once, D8), as the File Applications guide's 7.2.1 has it, and so does a `$PUT`; govax's indexed services kept
  them, and now don't, unless ROP ULK.
- **TYPE** of an indexed file types it by the primary key; TYPE of the logical name of one DCL has open is `%TYPE-W-OPENIN, error
  opening F:.; as input` with RMS$_ORG, and READ goes on where it was (D11).
- **DUMP/RECORD**'s "End of file block" of an indexed file is its allocation, 27, where the header says block 28, byte 0, as
  VMS's own indexed headers do (round 1's save set); VMS's DUMP of a relative file so ended showed the header's 4 of 3
  (`testdata/for81`). Whether the organization or `/RECORD` makes the difference is unconfirmed; `lastUsedBlock` goes by the
  organization.
- **Revisions:** each OPEN of an existing file to write gives it a new revision at its close (D1.IDX's 5), as `$CLOSE` does; a
  file OPEN makes keeps 1 (`FileHandle.reviseAtClose`).
- **DIRECTORY/FULL** says "Indexed, Prolog: 3, Using 4 keys", and `F$FILE_ATTRIBUTES`'s ORG, NOK, NOA, PVN, and the rest are VMS's.

## What round 3 settled (2026-11-04)

Round 3 (`testdata/idx88`'s `idx88c.com`) ran on VMS 7.1 to its end: `vax/idx88c.log` (21,342 lines) and `vax/idx88c.bck`, the
191 files it made, which govax's BACKUP reads and restores. The manual has the attributes and qualifiers; this is what it leaves
out. Subtasks 10b to 10d hold govax to the log, and read the save set's files for what the log only summarizes.

**FDL's parse** (C4, and C2's and C3's faults):

- **A statement is a line or a `;`-separated part of one**, numbered from 1 through the file, and every message names the
  statement: `KEY; SEG0_LENGTH 8` on line 3, after `FILE; ORGANIZATION indexed`, is statements 3 and 4. Messages with a
  segment end their first line with a blank, and the segment's line is the text upcased between backslashes.
- **A statement that isn't a secondary of the current primary is read as a primary**, so an unknown secondary (`ORGANISATION`),
  a secondary before any primary, and every secondary after a failed primary are each `%FDL-E-UNPRIKW, unrecognized primary keyword
  in statement n`. `KEY` with no number is `%FDL-W-VALPRI, value required on primary`; `KEY 300`, and keys or areas out of
  order (KEY 1 then KEY 0; KEY 0 then KEY 2), are `%FDL-E-OUTORDER, key or area primary defined out of order`. A primary
  whose secondaries all failed adds `%FDL-W-NULLPRI, null primary block`.
- **Warnings make the file anyway**, the value left at its default: an unknown keyword value or switch (`squential`, `maybe`,
  `BLOCK_SPAN 1`, `variabel`) is `%FDL-W-UNQUAKW, unrecognized qualifier keyword`, an ambiguous one (`CONT`) `%FDL-W-ABKW`, a
  secondary twice `%FDL-W-MULSEC, multiple secondary definition` (the last value kept: C46's two ALLOCATIONs made 9 blocks), a primary
  twice (its secondaries going on into the first: C47) or a value missing `%FDL-W-NOQUAL, no qualifier`, a bad date `%FDL-W-INVDATIM, invalid date/time`; then
  `%FDL-W-WARNING, parsed with warnings`, and `$STATUS` is `%X00010001` (from CONVERT, `%X00B48058`, and CONVERT then made
  no file). An error (`-E-`) ends with `%FDL-E-FDLERROR, error parsing FDL file`, `%X00B48072`, and no file; a bad protection
  is `%FDL-E-SYNTAX, syntax error`, its segment the value with its leading blank.
- **Not refused:** primaries out of the manual's order (RECORD before FILE), a string with no closing quotation mark, T, F,
  true, abbreviations, comments, and tabs.
- **CREATE/FDL's own:** `%FDL-F-OPENFDL, error opening DUA1:[R3]NOSUCH.FDL;` with RMS's reason (`%X10B4808C`; the default type
  `.FDL`), `%FDL-F-CREATE, error creating DUA1:[R3]C44.DAT;` with RMS's (`%X10B4806C`: MRS for a relative bucket of 64, SIZ, FLG,
  POS, IAN, DFL, NPK, each "for key = 0" from the STV), and `/LOG`'s `%FDL-I-CREATED, DUA1:[R3]C23.DAT;1 created`. A file not
  named takes FILE NAME, then is `DUA1:[R3].;1`; a name without a type takes DEFAULT_NAME's (`C2BOTHER.XYZ`). `/FDL` with no
  value is DCL's VALREQ.
- **Prolog levels:** PROLOG 2, PROLOG 1, and an overlapping primary key with no PROLOG all make a file ANALYZE calls Prolog 1.
  govax refuses them (answer 1); the status is subtask 10b's choice.
- **Round trip:** CREATE/FDL from ANALYZE/RMS_FILE/FDL's file of S35.IDX made a file whose FDL matches S35's to the ANALYSIS
  sections.
- **The end of file:** a file CREATE/FDL makes, and one CONVERT loads with `/NOFAST_LOAD`, has an EFBLK of 0 in its header (C31,
  C72), and VMS's DIRECTORY/SIZE and BACKUP/LIST count it at its high block (C31's 3, C72's 246), where govax's `backup.Attributes.Used`
  counts 0: 15 of the save set's files. A fast load's has HIBLK+1 (C75, 105 and 106). A sequential file with an EFBLK of 1 and no
  data counts 0 on both.

**CONVERT** (C6 to C10):

- **Success is `%X00000001`**, exceptions and all. An exception is an informational message, one per record, to SYS$OUTPUT:
  `%CONVERT-I-DUP, duplicate key encountered`, `-I-RSK, record shorter than primary key`, `-I-RTL, record longer than maximum
  record length`, `-I-RTS, record too short for fixed record format file`, `-I-SEQ` (each record out of order under `/NOSORT`:
  299 of a reversed 300), `-I-VFC`; `/PAD` on a file that isn't fixed is `%CONVERT-I-PAD, PAD option ignored, output record format
  not fixed`, once. `/EXIT` ends at the first with `%CONVERT-E-FATALEXC, fatal exception encountered` (`%X00B2808A`), the file
  left with the records before it.
- **Sorting is stable:** of `AAA two` and `AAA a duplicate`, the first is kept. `/EXCEPTIONS_FILE` with no value names the output's
  file with type `.EXC`; its records are the input's.
- **`/STATISTICS`** is six lines, `CONVERT Statistics` then the counts beside the I/O and times (a tab layout, in the log).
- **Refusals:** `%CONV-F-OPENIN` (`%X10B2109C`), `%CONV-F-OPENOUT` (a wildcard output: RMS's FNM; `/NOCREATE` of a missing file;
  `/PROLOG=4`, RMS's PLV; `%X10B210A4`), `%CONV-E-NOWILD` for a wildcard input, `%CONVERT-E-INP_FILES` past 10 inputs,
  `%CONVERT-E-NOKEY` for `/KEY=9`, `%CONV-E-CONFQUAL` for `/APPEND/MERGE`, `%CONVERT-E-NOTSEQ` for `/APPEND` to an indexed file,
  and `%CONV-F-WRITEERR` with RMS's NEF for `/NOCREATE` or `/MERGE` into a sequential file. `/WORK_FILES=11` is SORT's
  `%SORT-F-BAD_MERGE` after the output file is made. **Not refused:** `/KEY=1` on a sequential input, `/PAD=%X300`.
  `/STATISTICS=FULL` and `/IDENTICAL` are DCL's NOVALU and IVQUAL.
- **Inputs, outputs:** `/KEY=n` reads in that key's order; with no FDL the output has the input's attributes (areas too), its type
  the input's.

**ANALYZE/RMS_FILE** (C11, C12):

- **The status:** a report to SYS$OUTPUT, or `/NOOUTPUT`, ends with `%ANLRMS-I-ERRORS, <DUA1:[R3]T.TXT;1> 0 errors` and is
  `%X103B8103`; one to a file, and `/SUMMARY` anywhere (which writes no ERRORS line), is `%X103B8009`; with errors the
  status is an error's (`%X103B877A` and up). With no `/OUTPUT`, `/CHECK` and `/STATISTICS` go to SYS$OUTPUT and `/FDL` to `name.FDL`.
- **`/CHECK` on damage:** each finding is `***  VBN n:  text`, and some end the walk with `Unrecoverable error encountered in
  structure of file.`, the report going on to its end with the descriptors it hadn't shown left out. The texts seen: "Bucket check
  byte is out of phase" (either end, both reported at the bucket's first VBN), "Prolog block checksum is invalid" (the
  descriptor shown with the changed value), "Invalid key of reference in bucket header", "Invalid level number in bucket header",
  "BKT$V_ROOTBKT flag in index bucket header is not correct", "Key and/or data bytes do not fit in primary data record", "Record id
  at offset %X'00CE' is duplicate or too large", "SIDR pointer extends past end of SIDR record", "Key spills over into free key
  space of bucket", "Reserved flag bit 1 is set" (once per record), "The bucket chains for key #0 contain a loop", "VBN free space
  offset is invalid", "Invalid bucket address sample in bucket header", "Invalid first free byte offset in bucket header", and
  "Next-bucket pointer out of range of file". A key's data type changed is only the block's sum; the next-bucket pointer of the
  only bucket in its chain isn't checked at all (X07: no errors).
- **The four reports** (`Check RMS File Integrity`, `RMS File Statistics`, `RMS File Summary`, and FDL) are in the log for
  sequential, relative, and indexed files of every format.

**HELP/MESSAGE** knows the facilities CONVERT, CONV, FDL, and ANALYZE (the last one message); CREATEFDL and ANLRMS are
`%MSGHLP-F-NOSUCHFAC`. Their text is in `testdata/disks/idx88c-help`, local only.

## What CREATE/FDL settled (2026-11-04)

Subtask 10b, read from round 3's log and save set. Where this section and "What round 3 settled" differ, this one wins.

- **FDLERROR is DCL's to show.** `%X00B48072` has no STS$M_INHIB_MSG, while CREATE's and OPENFDL's (`%X10B4806C`, `%X10B4808C`) do. VMS's
  facility field is bits 16 to 27: FDL is 180.
- **NULLPRI** comes at the next primary recognized after a refused one (C3D's KEY 0), not at the end of the file (C3E, C4D, and C4E have none).
- **Abbreviations:** a keyword's exact match wins (`STREAM`, beside `STREAM_CR`), `SEG0_LENGTH`'s shown form is `SEG0`, and `1` isn't a switch's value.
  `CONTROL_FIELD_SIZE` is 7.1's name, where the manual has `CONTROL_FIELD`.
- **`FILE NAME` is used only with no file named.** C2BOTHER took DEFAULT_NAME's `.XYZ`, not NAME's `.DAT`. FOP MXV is on by default: a NAME
  of `S35.IDX;1` made `;2`. `$CREATE` had no MXV, and has now.
- **Fills are percentages of the area's bucket,** in bytes (C37's 80 percent of 4 blocks is 1638), and with no bucket size given there's nothing
  to take a percentage of: K50's 50 is a whole bucket in VMS's C73.
- **A key's name** is blank-padded (the manual says null), and `NAME ""` is 32 blanks where no NAME is 32 zeros (`KeySpec.Named`).
- **Found in `$CREATE`, each as VMS's:** FAB$C_UDF on a sequential file (C28); no name and no type is the file `.;1` (C2.C, where govax said
  RMS$_FNM, unconfirmed since Phase 33); FOP CTG and CBT mark a sequential or relative file's header when it has blocks (C29, C2A, not C2G); an indexed
  file's contiguity is any area's (C38's area 2); a file of more than one area ends at HIBLK+1, one area at 0 (C38, C52; C31).
- **Found elsewhere:** an end of file of 0 counts the high block (`FileAttributes.Used`, `backup.Attributes.Used`); DIRECTORY's full brief line is 79 characters;
  DIRECTORY/FULL's `In n areas`, and a relative file's `maximum record number` and `Bucket size` (`FileAttributes.MaxRecord`). BACKUP passed over any file with no
  type, whose entry has no dot, and couldn't restore `.;1` (`dottedName`, `restoreToVolume`).

**The status for Prologs 1 and 2 (answer 1's open choice):** `%FDL-F-CREATE` with `-RMS-F-PLV`, `%X10B4806C`, where VMS makes a Prolog 1 file. These are C3A, C3B, and C3C,
`expectedFDLDifferences`.

**Unconfirmed, govax's choices:** a blank or comment line takes no statement number (no fault followed one); a number that isn't one is SYNTAX; an ambiguous primary is ABKW; a
protection's unnamed categories have no access; an OWNER identifier the rights database lacks is ignored; FILE's other run-time switches (SUPERSEDE, CREATE_IF, TEMPORARY, DELETE_ON_CLOSE, and the
others) go to FOP as MAXIMIZE_VERSION does, though only that one was seen; AREA without BUCKET_SIZE takes FILE's; CBT with no allocation sets nothing, as CTG doesn't; a CTG file not in one run is left
unmarked rather than refused; FAB$C_UDF on a relative or indexed file is RMS$_RFM; how a name the command gives only in part combines with FILE NAME.

**Left:** `CREATE/FDL=SYS$INPUT`, which the manual shows for CONVERT, is 10c's to settle. A directory entry for a file with no type has no dot on govax's volumes (ods2's and `specFileName`'s
convention, older than this phase), and the `.;1` file's name is empty; whether VMS finds such entries on a volume it mounts hasn't been tried, and subtask 15 is the place. `DIRECTORY/TOTAL`, which
C0 and C13 use, is still an unrecognized qualifier.

## What CONVERT settled (2026-11-04)

Subtask 10c, read from round 3's log and save set. Where this section and those above differ, this one wins.

**A fast load** (`isam/load.go`; `TestFastLoadAsVMS` makes 17 of VMS's files the same by `Dump` and prolog bytes):

- A key's first data bucket and first index bucket are allocated together, before anything's written: data 5, root 7, then data 9 (C71).
  C78D, stopped after one record, grew by both and has an area and key that know nothing of it, so the prolog is written only as each key ends. C10B has key 0 whole and key 1 empty.
- A full data bucket's highest key goes up first, which can add a level (a root, then a new bucket: G116's root 98, level 1's 99), and then the next data bucket is allocated.
- Every bucket's check character is 0. SIDR pointers have 4-byte VBNs (control 82 and 02), where a `$PUT`'s are the shortest that fit. A SIDR isn't divided if it fits a bucket.
- A loaded index bucket keeps room for one key whole: G116's stopped at 504 of 512 bytes, refusing 507 (a key of 8; other sizes unconfirmed).
- A file of one area ends at the area's next block; of several, where `$CREATE` left it. Stopped before key 0's end, it ends where it did (C78D's 0).
- **Revisions:** one for each alternate loaded, so a file of 8 keys is 8 and of 1 key is 1; `/NOFAST_LOAD`'s is 2 (`$CREATE`, `$OPEN`); a sequential or relative file's 1; `/NOCREATE` adds one.

**The command** (`dclcmd/convert.go`, held by `TestIdx88CConvertOracle`):

- VMS corrected the manual three times: an alternate's `DUPLICATES` default is NO (K2's key 1, C68), which changes CREATE/FDL too; a VFC record with no control of its own in a print file has `01 8D`, not null (C83D); `/APPEND` to an indexed file is NOTSEQ, not ignored.
- With more than one input, equal keys sort by the whole record (C79's "AAA a duplicate" before "AAA two"); with one, they keep their order (C78). A record short of the key sorts zero-padded (`CC` before `CCC`). Exceptions come out in sorted order, since they're found at the load.
- CONVERT reads nothing else of a key already in order: `/WORK_FILES=11` copying S35.IDX to a file with the same primary key loaded key 0 and failed at key 1 (C10B), so the check waits for the first sort needed.
- A file of undefined records holds its bytes one after another, written by block (C84E): `$PUT` frames no such record.
- **`DUMP/BLOCKS=(START:1,COUNT:1)`** is VMS's spelling, which govax's `/BLOCK` and `=` weren't (`/BLOCK` still abbreviates it, and `=` is still taken), and DUMP/RECORD says "1 (0001) byte".
- **DCL:** `%DCL-W-IVQUAL` and `%DCL-W-NOVALU`, with their segments and VMS's statuses, replace govax's own messages for a qualifier a command lacks and a value on one that takes none (`verbError`).

**Differences left, listed in the oracle:** the Prolog 1 files (C93A, C93B, so C93C), `%CONV-F-OPENOUT` with `-RMS-F-PLV`, as CREATE/FDL's. C12A.SEQ, the records VMS could still read past a damaged free offset (X05), where govax's reader stops
at the bucket: not worth emulating. `/NOFAST_LOAD`'s roots of keys 0 and 1 have check characters 3B and D3 where govax has 1B and 03, keys 2 to 7 and every data bucket the same (C72, C74, C75), and C91's data bucket 3 where govax has 2: no rule fits so far, and a check character says nothing of what a bucket holds. COPY's `%COPY-S-COPIED` without `/LOG` is govax's own, and the oracle ignores it. `/STATISTICS` shows VMS's layout with 0 for I/O and page faults, its CPU time the time taken.

**Unconfirmed, govax's choices:** a load's fill smaller than the bucket follows ROP LOA's (K50's had no bucket size to be a percentage of, so nothing tested one); a primary key's duplicates in a load, a value with more pointers than a bucket holds, and a no-duplicates alternate's second value (DUP); `/APPEND` making a missing file (the manual's words); `READERR`'s status, never reached, since a read failure ends the reading quietly; an output's owner and protection with no FDL being the creator's and the volume's; `/NOCREATE` on an indexed file that has records going by `$PUT`; a host file as output, which RMS refuses.

**Left:** `/FDL=SYS$INPUT`, the manual's examples 7 and 8, and CREATE/FDL's too. `A122.FDL` and the others were VMS's in the oracle until 10d, and are govax's now.

## What ANALYZE/RMS_FILE settled (2026-11-04)

Subtask 10d, read from rounds 1's and 3's reports. Where this section and those above differ, this one wins.

**The output and status** (`dclcmd/analyzerms.go`):

- "What round 3 settled" had this wrong for a report to a file. Any report with `/OUTPUT` or `/NOOUTPUT` shows `%ANLRMS-I-ERRORS` and is `%X103B8103`; to a file the message goes to the terminal and not in the file (216 such lines in `IDX88.LOG`), and to SYS$OUTPUT it's in the report. A report with no `/OUTPUT`, and `/SUMMARY` and `/FDL` however they're sent, shows none and is `%X103B8009`.
- **The status is the first finding's**, but for "Reserved flag bit n is set", which counts as an error and sets none (X11's is its loop's). The reports chosen by more than one qualifier go FDL, then STATISTICS, then SUMMARY, then CHECK.
- `/FDL` of a list writes only its first file (C11L). A missing file is `%ANALYZE-E-OPENIN`, `%X10B1109A`.

**The walk** (`isam/inspect.go`'s `Inspector`):

- **The prolog's sums come first,** each block of the key chain and the areas once, before FIXED PROLOG. A bad sum doesn't stop anything: a damaged descriptor is shown as it is, and used.
- **A key is walked from its root down, a level at a time, and then its level 0 from the descriptor's first data bucket,** not from the index (X14 reports VBN 11, the descriptor's changed value, and nothing of the root).
- **A bucket's header** is checked in this order: check byte, key of reference, address sample, free offset, level, the root flag (or at level 0, each reserved bit), then, if it isn't the last, the next pointer. A key of reference, a sample, a free offset, or a next pointer wrong ends the walk, once the rest are reported. A level wrong doesn't: a data bucket claiming level 1 is taken for an index bucket, its end's free word is checked ("VBN free space offset is invalid", X12), and the key ends there, with the next key still walked.
- **A chain's loop:** a level may have as many buckets as the file could hold, allocation over bucket size. X11's bucket pointing to itself was read 7 times in 15 blocks of buckets of 2, its reserved bit reported each time, and then the loop at the 8th. (Round 3's note took those 7 for one per record.) The last bucket's next pointer isn't looked at (X07).
- Records: an ID at or past the bucket's next ID, or seen before in the bucket, is reported and goes on (X06). A record that doesn't parse, a SIDR's pointer past its record, or an index key past the free offset ends the walk (X05, X09, X10).
- `/SUMMARY` walks nothing. `/FDL` walks after the KEY and ANALYSIS_OF_AREA sections, with an ANALYSIS_OF_KEY after each key's walk, so a finding comes just before the section it would have had; and it reads the areas where the report tests the prolog, so E85's is "Attempt to read block with invalid VBN 0" there and "Information for locating area descriptors" in the report.

**The figures** (`KeyStats`, held to 155 keys by `TestStatisticsAsVMS`):

- A bucket's fill counts everything in use: a data bucket's free offset and its check byte, an index bucket's free offset, its pointers, and its 4 end bytes. Percentages are truncated.
- Index compression counts the pointers on both sides: (key + pointer) against (stored entry + pointer). Key compression compares the key's size to the 2 count bytes and what's stored. Record compression compares the bytes after the key, expanded and stored. All go negative when compression costs more than it saves.
- **Mean length:** a primary key's is a record uncompressed, an alternate's a SIDR's bytes as stored (size word, compressed key, and pointers, the bytes between each bucket's header and its free offset), both rounded: 21 bytes over 2 SIDRs is 11.
- **Duplicates:** FDL's DUPLICATES_PER_SIDR is the extra pointers over the SIDRs, rounded (3 over 2 is 2), and the statistics report's Mean Duplicates the same truncated (1).
- Overall space efficiency is the records' uncompressed bytes as a share of the file's allocation.
- FDL's fills are `(bytes+1)*100/bucket`, truncated and at most 100: the only form giving C37's 80 from 1638 of 2048 and G119's 58 from 300 of 512.

**The report's layout** (`anl/rmsfile.go`):

- A page is a form feed line, the heading, the file's name, two blanks, and 54 lines; a section's heading goes to a new page with fewer than 3 lines left for it.
- "Longest Record" is shown for a sequential file and for a file of any organization with fixed records (G109, C2H), not for another (C65, whose header has 200). "Null Character" follows "Key Segments" when NULKEYS is set; a primary key has no CHGKEYS or NULKEYS lines, and an alternate no REC_COMPR.
- An area's AOP gives FDL's AREA BEST_TRY_CONTIGUOUS (XAB$M_CBT, S64) and CONTIGUOUS (XAB$M_CTG, S65).

**Differences left:** Prolog 1 and 2 files stop with "Information for locating area descriptors" after their fixed prolog (answer 1), so E92, S34, S59, S60, and S61 and round 3's C3A, C3B, C3C, C93A, C93B, and C93C aren't compared. L138, U126, and U127 have reports but aren't in the save set. [000000]R3.DIR differs, ods2 extending a directory by less than VMS (9 blocks to 15).

**Unconfirmed, govax's choices:** the status of a /CHECK or /STATISTICS report to a file, `%X103B8103` by its message (round 1 showed the message and no status, and round 3 no such command); the status of a finding no report had first (key of reference, sample, free offset, root flag, next pointer, VBN free space, the area descriptors' and read failures': the invalid level's, `inspectUnseen`); a report's Reliability Options words (read-check, write-check), and Performance Options and Journaling always `none`; a key's order and its index entries' pointers aren't checked, no report having a message for them; Count of Reclaimed Blocks and RECLAIMED_SPACE always 0, as govax never reclaims; the layout of statistics for an alternate key followed by another key, no file showing one; `/OUTPUT` with no value as no `/OUTPUT`; `/FDL`'s default in the default directory, not the input's (the two were the same in the log).

**Left:** `/INTERACTIVE` and `/RU_JOURNAL` (answer 2's scope).

## The rules

### Keys [manual]

- **At least one key.** Primary is key 0; up to 254 alternates. 1 to 8 segments for a
  string; numeric keys are contiguous.
- **Types:** STG, IN2, BN2, IN4, BN4, IN8, BN8, PAC, COL, each ascending or descending
  (D prefixed, 20 added). Records with the same key keep the order they were written,
  whichever the sort order.
- **Flags:** a primary key can't change, and CHG on it is an error. DUP and CHG are free on
  alternates. **`$XABKEY`'s defaults differ from an XAB not made by the macro:** the macro
  gives alternates DUP and CHG. NUL is alternates only; a null string key's value is
  XAB$B_NUL, a numeric's 0, and such a record isn't in that index.
- **A short record:** one too short for an alternate key isn't in that index; every record
  needs the whole primary key.
- **Limits:** Prolog 3, 32,224-byte records; fixed and variable formats only, no VFC;
  records don't cross buckets. BKS 0 is a size RMS picks (the manual says both "one record" and "two"; [open]).
- **Prolog level:** XAB$B_PROLOG, or Prolog 3 unless the keys forbid it (overlapping primary
  segments, or too long a record), when RMS quietly makes Prolog 2. Asking for a level the
  keys can't have is an error.

### Finding a record [manual]

**By key** (RAC KEY): KBF, KSZ, KRF, and ROP. For a string key, KSZ under the key's size is
generic: only those leading bytes compare. For a numeric key KSZ must be 0 or the key's size.
ROP's EQNXT (KGE) is "equal or next" and NXT (KGT) "next", by the key's own sort order; REV
reverses either, for a key search only; Table 7-4 has the combinations, and three are
RMS$_ROP. Table 7-3 is a worked set, and an oracle case as it stands. A duplicate set found
by REV gives only its first record.

**In sequence** (RAC SEQ): in the current key of reference's order. `$CONNECT`, `$REWIND`, or
a keyed `$GET`/`$FIND` sets that key; an RFA access sets the primary key. ROP LIM stops at a
different key with RMS$_OK_LIM. ROP CDK makes `$GET` report RMS$_OK_DUP if another record has
that key.

**By RFA:** any record, by its stable address, until the file is reorganized.

### Writing [manual]

- **`$PUT`:** RAC KEY in any order; RAC SEQ only in sort order of primary key, else RMS$_SEQ.
  RMS$_DUP where a key forbids duplicates. UIF on a duplicate primary key makes it an update
  (RMS$_FAC without update access). ROP LOA fills buckets to DFL and IFL, otherwise full.
- **Splits:** a full bucket gives half its records to a new one, leaving RRVs behind (3.5.1.5).
  An index update that fails after the record went in is the success RMS$_OK_IDX.
- **`$UPDATE`:** the length may change, the primary key may not (RMS$_CHG), an alternate only
  with CHG, and a new value that duplicates where forbidden is RMS$_DUP.
- **`$DELETE`:** the current record, with ROP FDL (fast delete) leaving alternates' pointers
  behind. Space isn't reclaimed in place, and empty buckets wait for CONVERT/RECLAIM (3.5.1.1).
- **`$CONNECT`** with ROP EOF: primary key only, else RMS$_ROP; a first `$PUT` out of order is
  RMS$_SEQ.
- **Stream context:** the Guide's Table 8-3, as for the other organizations.
- **`$CREATE`'s own statuses** include RMS$_AID, BKZ, DAN, DFL, DTP, FLG, IAN, IFL, IMX, KNM,
  KSI, LAN, NPK, POS, REF, SIZ, SEG, RFM. Which field earns which is the probe's.

### FORTRAN [manual; FORRTL over it]

- **OPEN (9.1.15).** `KEY=(e1:e2[:dt[:dr]],...)`: bytes numbered from 1, 1 to 255 long, dt
  INTEGER (2 or 4 bytes) or CHARACTER, dr ASCENDING or DESCENDING. Mandatory on creating a
  file; on an existing one optional and "must be identical". Position in the list is the key
  of reference.
- **The specifiers (7.1.1.7).** KEY and KEYEQ are equal. KEYNXT is equal-or-next and KEYNXTNE
  next, by the key's direction, so both work either way. KEYGT and KEYGE are ascending only,
  KEYLT and KEYLE descending only; the other direction is a run-time error (FOR$_INVMATKEY,
  94, by its text). Note that KEYLE on a descending key is RMS's plain KGE: nothing here uses
  ROP REV.
- **A value.** Integer against integer keys, compared signed. Character is generic when shorter
  than the key, an error when longer. BYTE arrays of Hollerith data count as character.
- **KEYID** defaults to the unit's last, then 0.
- **READ** (7.2.3): a sequential READ after a keyed one goes on in that key's order. No END= on
  keyed reads.
- **WRITE** (7.3.3): always a new record. Unfilled fixed records are padded with blanks or
  zeros.
- **REWRITE** (7.4): the current record; a changed primary key "usually" errors (INCKEYCHG, 50,
  by its text).
- **DELETE** (9.7): `DELETE (u)` is the current record.
- **UNLOCK** (9.8) frees the unit's locked record.
- **Defaults.** RECORDTYPE FIXED, RECL required for FIXED (Table 9-3), FORM by ACCESS as Phase
  81 has it.
- **Conditions.** ATTACCNON 36 (no record), INVKEYSPE 49, INCKEYCHG 50, SPERECLOC 52,
  NO_CURREC 53, INVMATKEY 94. Which failure gets which is the probe's. Also FORRTL's
  sequential-against-keyed WRITE, which RAC it uses for each statement, and what it does at
  BACKSPACE, REWIND, and ENDFILE on an indexed unit.

## What has to change

- **A new package, `internal/isam`** (Decision 2): prolog, descriptors, buckets, records,
  compression, key comparison by type, search, insert, split, update, delete, `Check`, and a
  readable `Dump`, over a block interface. No VAX memory, FABs, or RABs in it.
- **RMS:** `create.go`, `open.go`, `xab.go` (XABKEY, XABSUM, multiple XABALLs), `get.go`,
  `put.go`, `recordops.go`, `stream.go`, `connect.go`, and `trace.go`; a new `rms/indexed.go`,
  as `relative.go` is for relative files; `$EXTEND` and `$SPACE`; `rms/fileattr.go` for
  F$FILE_ATTRIBUTES.
- **DCL:** READ's `/KEY`, `/INDEX`, `/MATCH`, `/DELETE`, `/NOLOCK`; WRITE's `/UPDATE`; TYPE and
  DUMP/RECORD on an indexed file; the utilities Question 2 chooses.
- **Compiler:** `sema`'s OPEN KEY= list checked and passed on; `keySpecs`'s four other
  specifiers (and its comment); the unit's direction known at run time only, so the check is
  FTNRTL's.
- **FTNRTL:** an XABKEY per key in `ftn_open.mar`; keyed READ; WRITE on a keyed unit; REWRITE,
  DELETE, UNLOCK; the conditions. Vector entries, and GSMATCH up.
- **ods2:** what subtask 6 finds, tagged and pinned by CLAUDE.md's procedure.
- **Docs:** MANUAL.md's sections 14.7 and chapter 14's files, RMS's limits (line 6783), HELP,
  FORTRAN-INTERNALS.md.

# Part B — decisions and questions

## Decisions

Accepted by the author, 2026-11-03 ("The review", below).

1. **VMS sets the requirement, in this order:** VMS's runs and files, then the manuals, then a
   reasonable choice logged as unconfirmed. Where the Guide and the bytes disagree (a bucket's
   byte 1, so far), the bytes win and the doc says so.
2. **The format is a leaf package, `internal/isam`, in plain Go,** like `backup`, `paging`, and
   `lck`. Each operation takes a key or a record and a small block interface
   (`ReadBlock`, `WriteBlock`, `Extend`). `rms/indexed.go` maps FAB, RAB, and XAB onto it.
   **Why:** it can be tested alone with a map of blocks, against VMS's files, and against a
   `Check` that walks the whole tree after each operation in a randomized test (as `vmheap` and
   `paging` do). See Question 4.
3. **No state across services but a stream's position.** Every RMS service already runs whole
   between instructions, so processes can't see a half-made split, and **bucket locks aren't
   needed**; record locks stay by RFA (Phase 47). Each service re-reads what it needs (key
   descriptor, root) through ods2. A position is (key of reference, RFA, the key's value), so a
   split by another stream can't lose it. VMS's buckets in memory, multibuffer counts, global
   buffers, and DFW have no effect: counted and answered, as BKS and MBF are now.
4. **Compression is in from the start.** VMS's default for string keys of 6 or more bytes is on,
   and every real file uses it, so govax must read it, and to make what VMS would, write it.
5. **Multiple areas are in.** Real files have them (SYSUAF's 3), and EDIT/FDL's default is 2 per
   index. XABALL's AID, BKZ, ALQ, DEQ, and AOP are kept. ALN and placement within the volume are
   accepted and not honored, as now.
6. **Volume files only,** as Phase 81's answer 3. A host file comes in through COPY or CONVERT.
7. **Left out, each answering as VMS would for a file it can't use:** collating keys (COL, DCOL:
   they need the National Character Set utility; RMS$_DTP), RU journaling, statistics monitoring,
   and DECnet access. Prologs 1 and 2 are out too (answer 1).
8. **The FORTRAN specifiers are the LRM's:** all of 7.1.1.7, correcting `keySpecs`. FTNRTL's
   interface stays our own, named as FORRTL names its entries, with FORRTL's keyword codes where
   Phase 81 already adopted them.
9. **ROP REV as VMS 7.1 has it.** VMS 7.1's RMS has it, by Tables 7-3 and 7-4 (round 1).
10. **Tests follow the existing pattern.** `TestIdx88Oracle` holds RMS's statuses and records to
    VMS's run; `isam`'s tests read VMS's files, or skip on a clean clone (the files are local) but
    for INDEXED.IDX and the probe's own, which are made from our data and can be committed; a
    round 2 has VMS run ANALYZE/RMS_FILE/CHECK and its RMS on govax's files; `testdata/fortran`'s
    end-to-end programs run both `/OPTIMIZE` ways.

## Questions for the author

1. **Which prolog levels?** VMS writes Prolog 3 by default, and every file on the system disk is
   Prolog 3. Prolog 1 and 2 are a different record layout (no compression, other overhead), used
   for RMS-11 compatibility and for overlapping primary segments.
   - (a) **Prolog 3 only.** Asking for a lower level, or a key only Prolog 2 can hold, or opening
     such a file, is an RMS error. VMS has no such refusal to copy, so the status is our
     choice, made in subtask 7 and logged. **Recommended:**
     nothing here uses them, and it's a clean later addition, since `isam` would be built around
     one record layout with room for another.
   - (b) Prolog 3, and also read 1 and 2.
   - (c) All three, both ways.
2. **Which utilities?** Without any, indexed files are made only by a program, and nothing checks
   one but our tests.
   - (a) None. RMS, DCL's READ and WRITE, and FORTRAN only.
   - (b) **CREATE/FDL, CONVERT, and ANALYZE/RMS_FILE** (`/CHECK`, `/FDL`, the report), and an FDL
     reader they share. **Recommended:** these are how a VMS user makes, loads, and checks an
     indexed file, `/CHECK` becomes a test of ours as VMS's is, and the probe's logs give their
     formats. `CONVERT` also ends the "govax has no CONVERT" gap Phase 69 worked around.
   - (c) Those, plus EDIT/FDL, CONVERT/RECLAIM, and ANALYZE/RMS_FILE/INTERACTIVE.
3. **How alike must the files be?**
   - (a) **Same structure, and the same decisions where VMS's are deterministic:** VMS's
     ANALYZE/RMS_FILE/CHECK passes our files, VMS's RMS reads and updates them, ours do the same
     for VMS's, and for each sequence the probe runs, a decoded comparison finds the same records,
     IDs, RRVs, levels, pointers, split points, and allocations, ignoring unused bytes. A
     difference that remains is listed, as `expected…Differences` lists do. **Recommended:** the
     leftover bytes above rule out strict identity anyway.
   - (b) Only that each side can use the other's files, with govax free to split and allocate its
     own way. Quicker, and less faithful.
4. **Where does the format live?** Your note allows ods2 changes.
   - (a) **`internal/isam` in govax,** with ods2 changing only as it must. **Recommended:** it's
     where relative files already are, and it keeps RMS's rules in one project.
   - (b) ods2's own `rms` package, so ods2's tools can read indexed files too. Better for ods2 as
     a library, at the cost of a tag and pin for each change while the format is still moving. It
     could also move there at the phase's end, once it's settled.
5. **Internal definitions.** The Guide prints only a few names. VMS's `$BKTDEF`, `$IRCDEF`, `$KEYDEF`,
   `$AREADEF`, and `$PLGDEF` would name every field and settle many of the offsets above. They
   would come from a probe assembling them `GLOBAL` and `decode.go` turning the object into values,
   as `testdata/for83`'s `def_opc.mar` did, from SYS$LIBRARY:LIB.MLB; or by your running
   `vmsdef/gen` over the licensed definition files, if they're there.
   - (a) **Yes, values only, through a probe,** never the macro text. **Recommended**, since you
     offered to capture symbols and it costs one round. The rule is yours, though: this library
     isn't one the clean-room notes name.
   - (b) No: ANALYZE/RMS_FILE's reports and the bytes only. Workable, with a few more rounds.

## The review (2026-11-03)

The author read the plan and accepted the recommendation on all five questions:

1. **(a):** Prolog 3 only.
2. **(b):** CREATE/FDL, CONVERT, and ANALYZE/RMS_FILE (`/CHECK`, `/FDL`, the report), with a shared
   FDL reader. EDIT/FDL, CONVERT/RECLAIM, and interactive ANALYZE stay out.
3. **(a):** the same structure, and VMS's choices where they're deterministic, held by a decoded
   comparison that ignores unused bytes; VMS's `/CHECK` passes govax's files.
4. **(a):** `internal/isam` in govax, ods2 changing only as it must. It could move later.
5. **(a):** values only, through a probe: `$BKTDEF`, `$IRCDEF`, `$KEYDEF`, `$AREADEF`, and `$PLGDEF`
   assembled `GLOBAL`, no macro text and no listing. This extends the clean-room notes to these
   definitions' values.

On Decision 7, the author first leaned towards including collating keys, then, after reading about them,
said to omit them. RU journaling, statistics monitoring, and network access are out as proposed.

# Part C — the subtasks

Each is one commit, complete and tested, then a stop for review. Provisional; the probe may change
them.

1. **Survey** (this doc). (done 2026-11-03)
2. **The probe,** `testdata/idx88`, on the simh VAX; described below. (done 2026-11-03, RMS's round; the others move
   to the subtasks that need them)
3. **`isam`: the format, reading.** Prolog, key and area descriptors, buckets, records, RRVs, SIDRs,
   decompression, key comparison for each type and direction; search, sequence, and `Dump`; every
   VMS-made file read whole, and each record found by each key. (done 2026-11-03)
4. **`isam`: writing, without splits.** `Create`, areas, first records, indexes made at first need,
   compression, IDs, check characters, the prolog's sums; files as the probe's first cases. (done 2026-11-03)
5. **`isam`: growth.** Splits, RRVs, index levels and root splits, duplicates, continuation, SIDR
   arrays, extension; `Check`, and a randomized test against a Go map. (done 2026-11-03)
6. **`isam`: update and delete.** Length changes, alternates moved, deleted records and pointers, RRV
   chains; ods2's additions if any. (done 2026-11-03; ods2 needed none)
7. **RMS: `$CREATE`, `$OPEN`, `$DISPLAY`.** XABKEY, XABSUM, XABALLs, every status; CIF;
   F$FILE_ATTRIBUTES; `$EXTEND`; SET DEBUG RMS. (done 2026-11-03)
8. **RMS: record operations.** `$CONNECT`, `$GET`, `$FIND`, `$PUT`, `$UPDATE`, `$DELETE`, `$REWIND`,
   `$FREE`, `$RELEASE`, `$SPACE`; ROP's options; contexts; locks; `TestIdx88Oracle`. (done 2026-11-03; `$SPACE` left out, as
   "What the record services settled" says)
9. **DCL.** READ's and WRITE's qualifiers, TYPE, DUMP/RECORD, DIRECTORY's remaining fields; each held
   to a probe log. (done 2026-11-04)
10. **Utilities,** as Question 2 chooses, in four parts: (a) round 3 of the probe, `idx88c.com`
    (done 2026-11-04); (b) the FDL reader and CREATE/FDL (done 2026-11-04); (c) CONVERT (done 2026-11-04); (d)
    ANALYZE/RMS_FILE (`/CHECK`, `/FDL`, `/STATISTICS`, `/SUMMARY`), each held to round 3's log (done 2026-11-04).
11. **FORTRAN: the compiler.** KEY= checked and passed; all the specifiers; DELETE's and REWRITE's
    forms on indexed units; messages.
12. **FTNRTL: OPEN and CLOSE.** The XABKEY chain, indexed defaults and refusals, INQUIRE.
13. **FTNRTL: the record statements.** Keyed READ, sequential READ after it, WRITE, REWRITE, DELETE,
    UNLOCK, positioning's refusals, each condition; the oracle on FORRTL's run.
14. **End to end.** `testdata/fortran` programs (an inventory by part number and by name, duplicates,
    both key directions, integer and character keys, shared units); RMS programs in MACRO; both
    paging modes.
15. **The VMS round.** VMS's ANALYZE/RMS_FILE/CHECK and RMS on govax's files, and on FTNRTL's. Probably
    two passes.
16. **Close-out.** MANUAL.md (chapter 14's table of what isn't there, RMS's limits, the new commands),
    HELP, FORTRAN-INTERNALS.md, CLAUDE.md, PLAN.md, and "Where things stand".

## The probe (subtask 2)

The author runs it (2026-11-03: "as always, happy to run probes"). It follows `testdata/for81`'s shape:
`gen.go` writes the programs, procedures, and `exchange.cmd`/`copyout.cmd`, on DUA5 (the author's startup
has XFER on DUA2), each case under a handler, every status printed, every RAB and XAB field shown. Files of our
own data come back in a save set, which `internal/backup` can read, so they can be committed with their
attributes. `TestIdx88Probe` rehearses the program on govax.

**Over govax first (no VAX needed):** FORRTL's FOR$OPEN with KEY= and a USEROPEN routine that logs the XAB
chain, to learn code 23's argument and what XABKEYs FORRTL builds. `FOR$READ_KF`'s arguments can't be seen
this way, since it must reach RMS; they go on the VAX. **Tried 2026-11-03, and not finished:** with mechanism 4
the value is the address of a block, and FORRTL reads a word at +2 first (mechanisms 5 and 10 fault at that
longword's value plus 2, which shows it). That word is a count: 0 is taken and makes no key, a multiple of 3 is
INVKEYSPE, and anything else INVARGFOR. Three items per key, then. Longwords, words, bytes, mechanism-coded
longwords, and pointers, across about 400 shapes, are all still INVKEYSPE or INVARGFOR, with one more clue: a
third longword item not below the second is INVARGFOR. No shape has put an XABKEY in the chain. This doesn't hold
anything up: FTNRTL's interface is its own (Decision 8), and round 2 can make files by RMS and open them
without KEY=, so only which XABKEY flags KEY= asks for still depends on it.

**Round 1, on RMS (`idx88.mar`):**

- **Keys:** a file per kind: each data type, ascending and descending; segmented, overlapping too;
  DUP, CHG, NUL; compression off for each of key, index, and data; multiple areas; names; fixed and
  variable records; an alternate at a position past some records' ends. Each read back through
  `$OPEN` and `$DISPLAY` into XABKEY, XABSUM, and XABALL.
- **`$CREATE`'s refusals:** each bad field alone, with STS and STV.
- **Growth:** records in order, reversed, and scattered, sized to force splits at known points; the
  file dumped after chosen steps, so that a split point, RRV, ID, and check character can be read off.
  Duplicates to continuation. Enough records for 3 levels, on a small bucket.
- **Finding:** every match in Tables 7-3 and 7-4 on each direction, generic lengths, KSZ errors, REV,
  LIM, CDK, RFA across a split, key of reference changes, `$REWIND`.
- **Changing:** `$UPDATE` longer, shorter, changing each kind of alternate; RMS$_CHG and RMS$_DUP; UIF;
  `$DELETE` with and without FDL, then finding by each key; `$PUT` into deleted space.
- **Sequence:** Table 8-3's cases; EOF; RMS$_SEQ.
- **Locks and sharing:** two streams; `$FREE`, `$RELEASE`, NLK, RRL, ULK; a second process's lock.

For each file: blocks by `$READ` (all-zero lines left out), **ANALYZE/RMS_FILE's report and `/CHECK`
and `/FDL`**, and DIRECTORY/FULL, so that VMS names what the bytes hold.

**Round 2, on FORRTL (`for88.mar`):** FOR$OPEN with KEY= of each form; `FOR$READ_KF`/`_KU` with each
match, through FOR$RAB to show RAC, KRF, KSZ, and ROP; WRITE, REWRITE, DELETE, UNLOCK; sequential after
keyed; each condition; positioning on an indexed unit.

**Round 3 (answer 2), `idx88c.com`:** CREATE/FDL and CONVERT's messages and qualifiers; ANALYZE's
output on files of each shape. Written 2026-11-04, after the *Record Management Utilities* manual came,
so it asks what the manual leaves to behavior: FDL's messages, one fault a file; the files CREATE/FDL
makes from each section and attribute, and from ANALYZE's own FDL files; CONVERT between every pair of
organizations, its loads (fast or not, fills, sorted or not), exceptions and the exceptions file,
`/STATISTICS`' layout, the record format qualifiers, and its refusals; ANALYZE/RMS_FILE on sequential,
relative, and indexed files of each format, its four reports, and `/CHECK` on 14 copies of S35.IDX with
one byte changed each (`idx88p.mar`: a check character at either end, a block's sum, a root VBN, a free
offset, a record ID, a chain pointer, an area's next VBN, a SIDR pointer's control byte, an index key, a
bucket's flags and level, a key's data type and first data VBN). Round 1's files are its inputs, restored
by govax into `[IN]` while it makes the exchange volume. HELP/MESSAGE's text for the facilities comes
back too, to `testdata/disks` only, being DIGITAL's. The README has its sections and how to run it.

**Round 4, DCL's side:** READ's and WRITE's qualifiers, F$FILE_ATTRIBUTES's KNOWN, NOK, NOA,
PVN; TYPE, DUMP/RECORD of an indexed file, as a procedure with SET VERIFY.

**What the rounds must settle** (Part A's [open]s, in one list): an RRV's form, and a deleted record's;
where a split falls; how IDs are given out and reused; how the check character changes; a bucket's byte
1; fixed records' size word; when pointers widen; an unmade index's root field; continuation and a
duplicate primary key's chain; SIDR flags, including KEYDELETE and ULK; each `$CREATE` status; allocation
and extension of areas, default bucket size, and the end of file; what a generic REV does; whether 7.1
has REV at all.

# Progress log

- **2026-11-03: subtask 1, the survey.** The author asked for this phase (RMS's keyed file type and
  FORTRAN's support for it; ods2 changes are fair game) and offered probe runs. Read the Guide, the RMS
  manual, the LRM, and the DCL Dictionary on indexed files, and the code they touch. Found the local
  system disk's real indexed files (17 names, 12 with contents read through govax's DUMP on a volume
  mounted read-only; the author allowed SYSUAF.DAT and it was read too, LMF$LICENSE.LDB wasn't), and
  decoded enough to write the layout above: a walk over 46,155 records of 8 files agrees, and RRVs are
  what's left. Found `keySpecs`'s comment wrong about the 1988 manual. Scratch scripts only, not kept;
  no code, volume, or local file changed. Part B has 10 proposed decisions and 5 questions.
- **2026-11-03: the review.** The author accepted the survey and the recommendation on all five
  questions, and omitted collating keys ("The review", in Part B).
- **2026-11-03: subtask 2, the probe, round 1 written.** `testdata/idx88`: `gen.go` writes `idx88.mar`, 152 cases
  making 138 files (key shapes, areas, `$CREATE`'s refusals, growth a step at a time, finding by Tables 7-3 and 7-4,
  generic and numeric keys, sequence, `$UPDATE`, `$DELETE`, RFAs across splits, null keys, locks); `idx88.com`, with
  ANALYZE/RMS_FILE on every file, three scripted interactive walks, and a save set; and the five `def_*.mar`, values
  only by answer 5, MACRO's output kept out of the main log with SYS$ERROR NL:. `TestIdx88Probe` assembles, links,
  and runs it on govax to its end with no condition, the first case exercising every helper on a sequential file.
  The rehearsal caught two assembly errors that would have cost a round (`$XABKEY` itself refuses CHG on a primary
  key, so that case now pokes the field at run time; a branch out of range). FOR$OPEN's KEY= was tried over govax and
  isn't found yet (the section above). `sema/io.go`'s comment is still wrong; subtask 11 changes it with the code.
- **2026-11-03, a mistake of ours in the rehearsal.** The author's startup mounts `testdata/disks/xfer.dsk` as DUA2,
  so the first rehearsal's `MOUNT DUA2` failed and its files landed on XFER. Its 16 files, all dated that minute, were
  deleted by name, which leaves XFER's directory as it was, though the container's bytes aren't identical (header
  slots, sequence numbers). The probe now uses DUA5.
- **2026-11-03: round 1's first run.** VMS's MACRO refused four of `idx88.mar`'s `$XABKEY`s (`IDX88M.LOG`, audited by the
  author), so nothing ran past LINK: IN4 and BN4 sizes, segments on a type but STG and COL, PAC's 16 bytes, and 255 in
  all. Those cases now poke the fields at run time, and govax's `$XABKEY` makes the same four checks, so the rehearsal
  would have caught them. The five `def_*.obj` came back whole. Whether other sizes (BN2, IN8) have such checks isn't known.
- **2026-11-03: round 1's second run, and a third to come.** VMS ran all 152 cases with no condition, and `/CHECK` found 0
  errors in every file but E85, whose `$CREATE` was refused (RMS$_KNM) and left a file with no prolog. The definitions
  came back, `defined.txt`: 246 names for the buckets, records, keys, areas, and prolog. But `p.put` wrote RAC SEQ, so a record
  out of order was RMS$_SEQ (247 of them), and the shape, growth, and split cases that wrote out of order made little.
  `put` now writes by key, a pair of cases sets a load by RAC SEQ beside the same by RAC KEY, and the interactive walks use the paths VMS
  listed. Seen so far: ROP REV is in VMS 7.1, by Tables 7-3 and 7-4; FLG's compression bits are stored inverted, as Part A read them.
- **2026-11-03: round 1's third run, and subtask 2 done.** All 154 cases, 428 `$PUT`s with RMS$_SUC and only the intended
  RMS$_SEQ. "What round 1 settled", in Part A, has the answers: RRVs, deleted records, the check character, fixed records, `$CREATE`'s statuses, and
  the names. `vax/` is committed: the run, the save set, VMS's reports. `IDX88M.LOG` doesn't exist, MACRO having said nothing.
- **2026-11-03: subtask 3, `isam`'s reader.** `internal/isam`: `prolog.go` (`Open`, `Key`, `Area`, the block sums; Prologs 1 and 2 an
  `ErrPrologLevel`, a file with none `ErrBadProlog`), `key.go` (each type's order both ways, `Value`, `IsNull`, `Indexes`, `assemble`),
  `bucket.go` (buckets, index entries, data records and RRVs, SIDRs, both compressions), `search.go` (`Find` with RMS's five
  answers, `First`, `Following`, `Walk`, `Record`), and `dump.go` (`Dump`, which later subtasks compare files with, showing nothing past
  a free offset). The reader read everything at its first run, so the survey and round 1 had the layout right but for the two
  corrections in "What the reader settled". The tests hold it to VMS: every prolog field to ANALYZE's report on the same file, each bucket and
  record in the interactive walks, every record rebuilt from `MKREC`'s number, the log's walks, RFAs (across splits), and searches (Tables
  7-3 and 7-4 both ways, generic, numeric, exact, sequence), each alternate's entries against the records that belong in it, and `Find`
  against the long way round at every level. `TestRealIndexedFiles` (`internal/rms`, local) reads the system disk's 27 files, LMF$LICENSE.LDB
  left out as in the survey. Nothing outside the package calls it yet.
- **2026-11-03: subtask 4, `isam`'s writing, without splits.** `write.go` (`Create`, a `Spec` of `KeySpec`s and `AreaSpec`s; `Put`; `allocate`; `makeIndex`; each insertion and its backing out; `WritableBlocks`,
  which `Open` finds by asking its `Blocks`) and `compress.go` (`compressKey`, `compressData`, `recordRest`, `CheckCompression`). `write_test.go` holds it to VMS: `TestShapesAsVMS` makes each shape file by the log's `$PUT`s with each status and RFA, and compares by `Dump`, then
  byte for byte; `TestGrowthStepsAsVMS` does the growth cases after every `$PUT`, reopening each time, up to the first split, against the blocks in the log; `TestCompressionIsVMSs` recompresses every file, and `TestRealIndexedFiles` the 27 on the system disk. Changing `shortestRun`, or the stride,
  made these fail, so they do hold those rules; the longest run's limit they don't (above). "What the writer settled" has the findings. `ErrBucketFull` is the seam subtask 5 replaces.
- **2026-11-03: subtask 5, `isam`'s growth.** `split.go` (data, SIDR, and index splits, RRVs, continuations, root splits, ROP LOA)
  and `check.go` (`Check`: rings, order, index entries over their buckets, RFAs and RRVs, every alternate's pointers both ways);
  `ErrBucketFull` is gone. `write_test.go`'s `TestGrowthStepsAsVMS` now follows every step, `TestBulkLoadsAsVMS` the five bulk
  cases, `TestScatteredDataAsVMS` G118's data buckets, `TestLoadAsVMS` case 119; `growth_test.go` has `Check` on every VMS
  file and on damaged ones, and `TestGrowthAgainstAModel`, 30 runs of 1,500 records of mixed sizes against a model, which reach
  what the probe doesn't. The rules were fitted by replaying the probe's dumps with each candidate rule and keeping the one that
  matched; "What the splits settled" has them, and what's still open (G117 and part of G118).
- **2026-11-03: subtask 6, `isam`'s update and delete.** `change.go`: `Update` (in place, or `splitForUpdate`, the split's
  code with the updated record as an old item that keeps its RFA), `Delete` (closing up, a bucket's last record a stub, a moved
  record's RRV taken out, each alternate's pointer), `DropPointer`, `ErrKeyChanged`; `split.go`'s `leftBytes`, `moveRRVs`, and
  `alone` now take each leaving item's destination, and a stub leaves no RRV; `insertRecord` puts a record in its key's stub's
  place; `Check` allows a fast delete's pointers. `change_test.go`'s `TestChangesAsVMS` replays cases 138 to 149 against every
  dump in the log; `TestChangesAgainstAModel` runs mixed puts, updates, and deletes against the growth test's model (now with
  each key's order of equal values), `Check` every 25 operations; `TestChangeRefuses` the refusals. Two orders VMS uses were
  read off stale bytes (U130, U136); the randomized test found subtask 4's RRV-between-records fault. "What update and delete
  settled" has the findings.
- **2026-11-03: subtask 7, RMS's `$CREATE`, `$OPEN`, `$DISPLAY`, and `$EXTEND`.** `rms/indexed.go`: `checkIndexedCreate` (the
  XABKEYs and XABALLs read and checked in VMS's order, an `isam.Spec` made, FAB$L_ALQ and FAB$B_BKS written as VMS's are),
  `initIndexed` (the key names, then `isam.Create` through `indexedBlocks`, a volume file as `isam.WritableBlocks`, and the
  header's fields), `isIndexed`, `openIndexed`, and the record services' interim RMS$_ORG (`indexedPending`); `indexedxab.go`
  (`fillIndexedXABs`: XABSUM, XABKEY by REF, XABALL by AID, for `$OPEN`, `$DISPLAY`, and `$CREATE`'s XABSUM); `extend.go`
  (`SysExtend`, registered in corevms). `isam`: `PrologBlocks`, `ExtendArea`, `SizedBlocks`, and `allocate` taking the extent
  to come. F$FILE_ATTRIBUTES's NOK, NOA, and PVN (`FileAttributes.Keys`, `Areas`, `Prolog`); SET DEBUG RMS shows a XABKEY's
  REF, DTP, FLG, and segments, a XABALL's AID and BKZ, and a RAB's KRF, KSZ, and key on an indexed file. Tests:
  `TestIdx88CreateOracle` (`internal/console`; `runIdx88` now runs the probe on an RD54 of 3-block clusters, so `TestIdx88Probe`
  does too), `TestIndexedOpenAsVMS` (VMS's files restored and opened), and `indexed_test.go`'s cases the probe lacks, and
  `TestExtendArea`. "What RMS's `$CREATE` settled" has the findings.
- **2026-11-03: subtask 8, RMS's record operations.** `rms/indexedrecords.go`: `indexedStream` (the key of reference, the place
  as an `isam.Entry`, the record queued after a deletion, the current record, the last sequential `$PUT`'s key), `connectIndexed`
  (from `armForFAC`), `indexedLocate` (`$GET` and `$FIND`: sequence, key with `keyMatch` and `searchValue`, RFA; locks through
  `lockFound`; LIM and CDK by `keyStatus`), `indexedPut` (SEQ, UIF by `putOver`, LOA), `indexedUpdate`, `indexedDelete`
  (FDL), `indexedRewind`; `findLive` and `nextLive` take out a fast delete's pointers as a search meets them. The interim
  `indexedPending` is gone, `$TRUNCATE` is RMS$_IOP after its FAC check, and RMS$_OK_RRL is an indexed file's as a relative
  one's. `isam.PutResult.Split`; `indexedBlocks.Extend` sets the end of file; `$FAO`'s `!AF` keeps the DEC Multinational
  graphics (`corevms/fao.go`'s `faoPrintable`). Tests: `TestIdx88Oracle` (`internal/console/idx88oracle_test.go`, every case
  whole, dumps that differ compared by `isam.Dump`; a mutation ignoring ROP LOA fails it), `TestIdx88CreateOracle` without its
  two subtask 8 exceptions, and `indexedrecords_test.go` for what the probe lacks (FAC refusals, `$FIND` then `$UPDATE` and
  `$DELETE` with the sequence going on among duplicates, RFA access, the end of file, `$REWIND`'s key). "What the record
  services settled" has the findings.
- **2026-11-04: subtask 9, DCL, written; round 4 waits for its run.** Round 4 of the probe (`testdata/idx88`'s `idx88d.mar`, which
  makes three empty indexed files, and `idx88d.com`, sections D1 to D11: WRITE in any order and its refusals, READ in sequence,
  `/KEY` exact and generic, `/MATCH`, `/INDEX` and its persistence, a descending and a binary key, `/DELETE`, WRITE/UPDATE,
  two streams and `/NOLOCK`, a fixed-length file, OPEN/APPEND, TYPE, DUMP/RECORD, DIRECTORY/FULL, F$FILE_ATTRIBUTES;
  `exchanged.cmd`, `copyoutd.cmd`). The code: `rms/dclkeyed.go` gives a DCL file that needs one a RAB in a small memory of its
  own (MAPEN off) and calls `SysGet`, `SysPut`, `SysUpdate`, and `SysDelete` with it, as VMS's DCL calls RMS, so subtask 8's
  services, statuses, stream context, and locks are DCL's with nothing written twice; `DCLFile.Read` takes a `DCLRead`, the
  DCL side's `readQualifiers` and `keyedStream`. An OPEN's file is locked in the process's lock database (`DCLAccess.Locks`).
  TYPE and DUMP/RECORD walk the primary key (`writeIndexedRecords`, `DumpFile.indexedRecords`, an RFA's offset word the record's
  ID); DIRECTORY/FULL shows "Indexed, Prolog: 3, Using n keys" (`organizationText`), as VMS's did, so `TestBackup69Oracle` and
  `restore_test.go` lost their entries for it. HELP READ and WRITE and MANUAL.md's file I/O section describe them.
  `TestIdx88DProbe` rehearses round 4 on govax and holds it to the DCL Dictionary meanwhile. The choices the run is to settle,
  each unconfirmed: WRITE is a `$PUT` by key (RAC KEY), so any order, after OPEN/APPEND too; `/MATCH` takes an abbreviation
  only if it's one keyword's (`G` is CLI$_IVKEYW); `/NOLOCK` is ROP NLK with RRL, after the manual's "a record ... locked by other
  accessors"; `/INDEX` stays the file's even when its READ fails; READ/DELETE is a `$GET` then a `$DELETE`, and a failed delete
  leaves the symbol as it was; an indexed stream opened to write has FAB$V_DEL; `/KEY` on a sequential file is whatever `$GET` by
  key says (RMS$_RAC); TYPE's order and DUMP/RECORD's RFAs. Found on the way: a process-permanent file armed its stream again when
  it had no Reader or Writer, which for an indexed stream dropped DEL and its key of reference (`ppf.go`); and an endless READ loop
  in a probe that goes round on any error under SET NOON, so round 4's loops leave on `/ERROR` too.
- **2026-11-04: round 4's run, and subtask 9 done.** VMS ran `idx88d.com` (`vax/idx88d.log`, 1,087 lines), and govax's log differed in
  ten places, now VMS's ("What DCL's round settled"): WRITE's and WRITE/UPDATE's RMS$_OK_DUP; OPEN/APPEND's first `$PUT` checked
  against the file's last key; CLI$_ABKEYW, and CLI$_VALREQ from the grammar engine for every qualifier; READ/DELETE's symbol and
  status; `$UPDATE` and `$PUT` giving up record locks (`indexedrecords.go`); a VFC record's fixed control left out of a READ through
  the RAB; TYPE of an open indexed file's logical name; DUMP's end of file for an indexed file; revisions at CLOSE (`FileHandle.reviseAtClose`, `ppf.go`'s
  `closeRevision`). `TestIdx88DOracle` holds the whole log, times masked; `TestIdx88DProbe` is the rehearsal still. Round 1's
  oracles are unchanged.
- **2026-11-04: subtask 10a, round 3 written; it waits for its run.** The author added the *Record Management Utilities
  Reference Manual* (VAX 6.0), which documents FDL's every attribute, CONVERT's qualifiers and exceptions, CREATE/FDL, and
  ANALYZE/RMS_FILE's qualifiers; before it, only online help on the VAX had them. So round 3 (`testdata/idx88`'s
  `idx88c.com`, sections C0 to C13, and `idx88p.mar`, a byte patcher for damaged copies; `exchangec.cmd`, `copyoutc.cmd`) asks
  only what the manual leaves to behavior ("The probe", round 3). `TestIdx88CProbe` (`internal/console/idx88util_test.go`)
  rehearses it on govax: IDX88P assembles, links, and changes each byte meant, and the procedure reaches its end. The
  rehearsal caught two faults that would have cost a round: IDX88P's early `BLBC`s couldn't reach past the service
  macros (now `ONERR`, a branch round a `BRW`), and each CALL level begins with ON ERROR THEN EXIT, so a failure in a
  subroutine hid what followed (each now says SET NOON). Two of the probe's own plans were wrong and were changed before
  the run: F122's keys 1 and 4 both give the primary key's order, so the reversed input is key 2's (-n); and a CREATE/FDL
  with no file named, from ANALYZE's FDL of `[IN]S35.IDX`, would make a new version of the file C12 copies, so it's last.
  Not done: DCL's bit-field assignment (`R[0,32] = n`), which govax lacks, so the round makes no binary data in DCL and
  takes round 1's files for that.
- **2026-11-04: round 3's run, and subtask 10a done.** VMS 7.1 ran `idx88c.com` to its end, and the author audited `IDX88CM.LOG`
  and brought back `vax/idx88c.log`, `vax/idx88c.bck` (191 files, read and restored by govax's BACKUP), `vax/idx88cbck.txt`, and
  HELP/MESSAGE's text (local only). "What round 3 settled" has the findings: FDL's statement numbering and messages, which faults are
  warnings that still make the file, CREATE/FDL's names and refusals, Prologs 1 and 2, CONVERT's exceptions, statuses, and
  refusals, ANALYZE's statuses and the texts of 20 kinds of damage. Found for 10b: VMS counts a file whose EFBLK is 0 at its high
  block, which govax's `backup.Attributes.Used` (and so BACKUP/LIST) doesn't. No code changed.
- **2026-11-04: subtask 10b, the FDL reader and CREATE/FDL.** `internal/fdl` is a leaf package: `table.go` has every primary and secondary of the manual's chapter 4, `parse.go` splits statements and applies
  round 3's rules (`doc.go`), `definition.go` is the result, `messages.go` the texts. `rms/fdlcreate.go` lays a FAB, NAM, and XABs out in a memory of its own, as `dclkeyed.go` does a RAB, and calls `SysCreate` and
  `SysClose`, so every refusal is RMS's own. `dclcmd/createfdl.go` is the command, and `console.dcl`'s `create_fdl` its syntax; so that `/FDL=file` can take a value and still make the file parameter optional, a typed qualifier
  with `/syntax=` now does both (`dcl.Qualifier.hasValue`). `TestIdx88CCreateOracle` holds every CREATE/FDL of C2 to C4 to VMS's log, times and file IDs masked and ANALYZE's reports left out, and 39 files' headers and blocks to VMS's save set, and 3 more
  (C5) from VMS's own A35, A62, and A122.FDL, restored since ANALYZE is 10d's, with C13's `;2` and K50's fills; `TestCreateFDLForms` is what the log lacks; `fdl/parse_test.go` the parser alone. "What CREATE/FDL settled" has the findings.
- **2026-11-04: subtask 10c, CONVERT.** `isam/load.go` is the fast load (`StartLoad`, `Add`, `FinishPrimary`, `FinishAlternate`; `File.holdProlog`), fitted to VMS's files by `TestFastLoadAsVMS`, which reads each file's records and builds it again. `rms/convert.go` reads an input whole through a FAB and RAB of its own (`ReadConvertInput`), makes a file with its attributes when there's no FDL (`CreateLike`, shown fills in bytes), and writes through `$PUT` or the load (`ConvertOutput`). `dclcmd/convert.go` is the command: exceptions, sorting, padding, statistics. `fdlcreate.go` split so CONVERT shares its `$CREATE`. `TestIdx88CConvertOracle` holds C6 to C10 command by command and 60 of the files; "What CONVERT settled" has the findings.
- **2026-11-04: subtask 10d, ANALYZE/RMS_FILE.** `isam/inspect.go` is an `Inspector`, a lenient reading of the prolog and a walk of each key that reports each `Finding` as it comes and counts `KeyStats`; `anl/rmsfile.go` is the four reports (anl now imports rms); `rms/analyze.go`'s `AnalyzeSearch` finds the files; `dclcmd/analyzerms.go` is the command. `TestStatisticsAsVMS` fitted the figures to round 1's FDL files, `TestIdx88ReportsOracle` holds the report, /CHECK, and /FDL of each file to VMS's, and `TestIdx88CAnalyzeOracle` every ANALYZE command of round 3 with its status. `runIdx88C` no longer restores VMS's FDL files: C5 to C7 use govax's own, and CREATE's oracle restores VMS's for itself. "What ANALYZE/RMS_FILE settled" has the findings.
