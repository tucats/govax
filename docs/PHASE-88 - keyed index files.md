# Phase 88 — keyed index files

**Status:** planned 2026-11-03. Subtask 1, the survey, done the same day, and
reviewed and accepted by the author the same day ("The review", in Part B).
Subtask 2, the probe of RMS, done the same day, after three runs ("What round 1 settled",
in Part A). Needs nothing beyond Phases 1–87. It picks up what Phase 81 left (its Decision
13) and Phase 74's review deferred to a later program.

## Where things stand

Part A is the survey, Part B the accepted decisions and the author's answers, and
Part C the subtasks. No RMS code is implemented yet. `testdata/idx88` holds VMS's run, files, and
definitions. **Next: subtask 3,** `internal/isam`'s reader.

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
   VMS-made file read whole, and each record found by each key.
4. **`isam`: writing, without splits.** `Create`, areas, first records, indexes made at first need,
   compression, IDs, check characters, the prolog's sums; files as the probe's first cases.
5. **`isam`: growth.** Splits, RRVs, index levels and root splits, duplicates, continuation, SIDR
   arrays, extension; `Check`, and a randomized test against a Go map.
6. **`isam`: update and delete.** Length changes, alternates moved, deleted records and pointers, RRV
   chains; ods2's additions if any.
7. **RMS: `$CREATE`, `$OPEN`, `$DISPLAY`.** XABKEY, XABSUM, XABALLs, every status; CIF;
   F$FILE_ATTRIBUTES; `$EXTEND`; SET DEBUG RMS.
8. **RMS: record operations.** `$CONNECT`, `$GET`, `$FIND`, `$PUT`, `$UPDATE`, `$DELETE`, `$REWIND`,
   `$FREE`, `$RELEASE`, `$SPACE`; ROP's options; contexts; locks; `TestIdx88Oracle`.
9. **DCL.** READ's and WRITE's qualifiers, TYPE, DUMP/RECORD, DIRECTORY's remaining fields; each held
   to a probe log.
10. **Utilities,** as Question 2 chooses: an FDL reader, CREATE/FDL, CONVERT, ANALYZE/RMS_FILE, each
    possibly its own subtask.
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

**Round 3 (answer 2):** CREATE/FDL and CONVERT's messages and qualifiers; ANALYZE's
output on files of each shape.

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
