# Phase 69 — BACKUP save sets

**Status:** in progress. Planned 2026-10-20; the plan reviewed and
accepted by the author the same day (Part B). Subtask 1, the survey, is
done: Part A holds what the first save set
(`testdata/backup69/vax/pi.bck`) and the public save set readers show of
the format. Subtask 2, the VMS probe (`testdata/backup69`), is done:
"The probe's answers" in Part A settles the format, the CRCs, the XOR
blocks, and VMS's messages. Subtasks 3 (COPY to a host name), 4 (`internal/backup`, the
reader), 5 (opening a save set), 6 (the command), and 7 (BACKUP/LIST)
are done.

## Goal

govax's DCL gets a `BACKUP` command for VMS save sets, with VMS BACKUP's
syntax:

1. **List** a save set: `BACKUP/LIST saveset/SAVE_SET`, with VMS's
   layouts for the summary and file lines, `/BRIEF` (the default) and
   `/FULL`, and `/LIST=file`.
2. **Restore** files from a save set: `BACKUP saveset/SAVE_SET
   output-spec`, onto a mounted volume or to the host. Each file keeps
   its saved attributes: record format, dates, protection, owner,
   characteristics, allocation, and end of file. The qualifiers people
   use are in scope: `/SELECT`, `/EXCLUDE`, `/LOG`, `/NEW_VERSION`,
   `/OVERLAY`, `/REPLACE`, `/BY_OWNER`, and the output-directory rules
   (`[*...]`, `[dir...]`, `[dir]`, omitted).
3. **Compare** a save set with files: `BACKUP/COMPARE saveset/SAVE_SET
   file-spec`.
4. **Save** files into a new save set: `BACKUP file-spec
   saveset/SAVE_SET`, with `/BLOCK_SIZE`, `/GROUP_SIZE`, `/CRC`,
   `/COMMENT`, `/BY_OWNER`, and the input file-selection qualifiers
   (`/SINCE`, `/BEFORE`, `/BY_OWNER`, `/EXCLUDE`, ...). Real VMS reads
   govax's save sets in a second round, as Phase 68's second round ran
   govax's images.

A save set may be a file on a mounted volume, which is how VMS keeps
one: fixed-length records the size of the save set's block. It may also
be a host file holding the same bytes, as a binary FTP, COPY/BINARY, or
a copy out of a container leaves it.

Out of scope: image, physical, and incremental operations, `/RECORD`,
`/JOURNAL`, `/VERIFY`, `/DELETE`, tapes (and SIMH `.tap` images, a later
phase), multivolume save sets, `/CONFIRM`, ODS-5 names, and
`/INTERCHANGE`.

As in other phases, the target is real VMS's output. Listings and
`/LOG`'s messages should match VMS's line for line, and a restored file
should match its original block for block and attribute for attribute.

## Part A — the survey (2026-10-20)

### Sources

- **The first save set** (the author, 2026-10-20). On the simh VAX
  (OpenVMS VAX V7.1, BACKUP V6.2) the author ran
  `BACKUP [.PI]*.* PI.BCK/SAVE` in `DUA1:[XFER]`, saving `PI.EXE`,
  `PI.MAR`, and `PI.OBJ` from `[XFER.PI]`, and sent the container
  (`testdata/disks/xfer.dsk`, local-only), VMS's `DIRECTORY/FULL` of the
  save set, and VMS's `BACKUP/LIST` of it. The save set and the three
  files, copied off the volume block for block, are in
  `testdata/backup69/vax/`; its README has VMS's listing and the files'
  attributes. Real VMS output, so usable under the clean room.
- ***HP OpenVMS System Management Utilities Reference Manual: A–L***
  (AA–PV5PH–TK, September 2003, OpenVMS 7.3-2), chapter 7, the Backup
  utility: the command format, Table 7–1 (input and output by operation),
  the qualifier types and positions (7.3.2), the wildcard and
  output-directory rules (7.3.3), Table 7–2 and every qualifier's entry
  (7.5), the examples (7.6), and Appendix G (valid qualifier
  combinations). On disk as `~/Documents/Technical Doc/VMS/VMS SYSAdmin
  I.pdf`. The file name doesn't say so, but that's what it is. An HP
  manual, usable like DIGITAL's. It says nothing about the save set's
  layout ("Save sets are written in a format that only BACKUP can
  interpret"). A `pdftotext -layout` extraction made for the survey is in
  the session's scratchpad. A copy beside the PDF, like the others'
  `.txt` files, would make it searchable later.
- **The DCL Dictionary** (`OVMS_731_dcl_dictionary-a-m.txt`) gives only
  BACKUP's format and refers to the manual above.
- **The author's note on the data CRC** (2026-10-20): with `/CRC`, the
  default, BACKUP computes the AUTODIN-II 32-bit CRC (the VAX CRC
  instruction's polynomial) over each data block, stores it in the block
  header, and checks it on a restore with `/CRC`.
- **Public save set readers** (allowed by the author, 2026-10-20; Part
  B, question 6): independently reverse-engineered programs, not
  DIGITAL's. They were read for the format's layouts and field names
  only, and no code is copied from them (some are GPL):
  - `vmsbackup` (github.com/FreddieAkeroyd/vmsbackup, from
    vms2linux.de): `vmsconstants.h` names the operating system,
    subsystem, and application codes and every attribute code (1 to
    101), and `vmsbackup.c` lays out the block header (`bbhdef`), the
    record header (`brhdef`), and the record types (0 to 11);
  - a second `vmsbackup` (github.com/DaveShepperd/vmsbackup), the same
    layouts (it leaves the header checksum as a TODO);
  - `VMSBackupPython` (github.com/TonyBUK/VMSBackupPython), which names
    the block flag `NOCRC` and the record flags.

  None of them checks the header checksum or the data CRC, or recovers
  a block from its XOR group. Those come from the fixture and the probe.

Not used: VMS's sources and definition files (the clean room).

### What earlier phases leave in place

- **RMS and ods2** read and write volume files. `rms.FileAttributes`
  (`internal/rms/fileattr.go`, Phase 51) holds nearly everything a save
  set's file record carries: record attributes, characteristics, owner,
  protection, revision count, and the four dates. DIRECTORY/FULL's
  writer (`internal/rms/directory.go`) can likely lay out
  `BACKUP/LIST/FULL`, which the manual says is "similar to ...
  DIRECTORY/FULL". ods2's `volume.CreateFileVersion`, `WriteBlock`,
  `CloseWithFinalByte`, and `UpdateHeader` can make a file and set its
  header fields. Whether they set every field a restore needs (dates,
  revision count, characteristics, highwater, allocation, ACL) is to be
  checked in subtask 8.
- **Directories:** `rms.Session.CreateDirectory` (Phase 34) makes the
  directories a restore to `[*...]` or `[dir...]` needs.
- **Name processing and wildcards:** `rms` parses names and
  `filespec.Glob` matches them. `/SELECT` and `/EXCLUDE` match names
  inside the save set, so they need a matcher over strings, not a
  directory walk. `filespec`'s wildcard matcher is unexported today.
- **The grammar** has qualifiers that belong to one parameter (COPY's
  `SOURCE/HOST` and `DESTINATION/HOST`), which BACKUP's
  position-dependent qualifiers need.
- **Times:** `vmstime` converts VMS quadword times. The fixture's six
  times decode to the listing's and DIRECTORY/FULL's exactly.
- **CRCs:** `cpu.CRC` (Phase 53) is the VAX CRC instruction, usable for
  both checks below with the right table and initial value.
- **Messages:** `vmsdef.Messages` has no `BACKUP$_` messages. The texts
  come from the probe's log, their codes unconfirmed (question 4).
- **COPY/BINARY** copies a volume file to the host byte for byte. It
  brought `pi.bck` out identical to the ods2 extraction, so the probe's
  copy-out uses it.

### The save set as the fixture shows it

`pi.bck` is 126 blocks: two save set blocks of 32256 bytes, the
`Block size` the summary names. The volume file's records are fixed
length, the same 32256 bytes, so each record is one save set block.
Block 1 holds the summary and the three files. Block 2 is block 1's
redundancy (XOR) block: the group size is 10, the group has one data
block, and the XOR of one block is that block. Its records repeat block
1's byte for byte, and only its header differs (below).

Integers are little-endian. In the tables, "confirmed" means the
fixture or VMS's listing settles a field. "Readers" means the public
readers name it and the fixture agrees, which the probe should still
confirm. "Guess" means it fits this save set and needs the probe.

#### The block header (256 bytes)

Each block starts with a 256-byte header. Records follow it, up to the
block's end. The names are the readers'.

| Offset | Size | Block 1 | Block 2 | Field | Meaning |
| --- | --- | --- | --- | --- | --- |
| 0 | word | 256 | 256 | size | The header's size (confirmed: the records start there) |
| 2 | word | %X0400 | %X0400 | opsys | The operating system: %X0400 VAX, %X0800 Alpha, %X1000 I64 (readers); the summary's attribute 7 too |
| 4 | word | 1 | 1 | subsys | The subsystem: 1, BACKUP (readers) |
| 6 | word | 1 | 2 | applic | The application: 1 a data block, 2 an XOR block (readers; fits the fixture) |
| 8 | long | 1 | 2 | number | The block's sequence number in the save set (readers) |
| 12–31 | 20 | 0 | 0 | | Spare |
| 32 | word | %X0101 | %X0101 | struclev | The block's structure level (readers) |
| 34 | word | 1 | 1 | volnum | The volume number, for multivolume save sets (readers) |
| 36 | long | 0 | 0 | crc | The data CRC (readers; zero here, see below) |
| 40 | long | 32256 | 32256 | blocksize | The block size (confirmed) |
| 44 | long | 1 | 1 | flags | Flags. Bit 0 is NOCRC: no data CRC (readers) |
| 48 | 32 | `PI.BCK` | `PI.BCK` | ssname | The save set's name, a counted string (confirmed) |
| 80 | 6 | 0 | 0 | fid | The current file's ID (readers; zero in a disk save set?) |
| 86 | 6 | 0 | 0 | did | The current file's directory ID (readers) |
| 92 | 128 | 0 | 0 | filename | The current file's name (readers) |
| 220 | 1 | 0 | 0 | rtype | The current file's record type (readers) |
| 221 | 1 | 0 | 0 | rattrib | Its record attributes (readers) |
| 222 | word | 0 | 0 | rsize | Its record size (readers) |
| 224 | 1 | 0 | 0 | bktsize | Its bucket size (readers) |
| 225 | 1 | 0 | 0 | vfcsize | Its VFC area's size (readers) |
| 226 | word | 0 | 0 | maxrec | Its maximum record size (readers) |
| 228 | long | 0 | 0 | filesize | Its allocation (readers) |
| 232–253 | 22 | 0 | 0 | | Spare |
| 254 | word | %XBE02 | %X1B5D | checksum | The header's checksum (confirmed, below) |

**The header checksum** is the CRC-16 of the whole 256-byte header,
taking its last word and the CRC longword (offset 36) as zero (the
second part from the probe, answer 2). The polynomial is %XA001 (CRC-16,
reflected, the VAX CRC instruction's "CRC-16" table), the initial value
0, and nothing is XORed at the end. It matches both blocks. They differ
in two fields, so a chance match is very unlikely. Found by searching
every 16-bit polynomial against the two headers' difference, then every
initial value. The readers don't check it.

**The data CRC:** none was stored, and the header says so: flag bit 0
is NOCRC. The command didn't say `/NOCRC`, though, and the author's note
says `/CRC` is the default. The probe found that this VAX's BACKUP
writes a CRC only when `/CRC` is given, and found the CRC's rule ("The
probe's answers", 1).

In `pi.bck` the CRC longword is zero, so the survey couldn't tell that
the checksum leaves it out.

#### Records

A record starts with a 16-byte record header:

| Offset | Size | Field | Meaning |
| --- | --- | --- | --- |
| 0 | word | rsize | The size of the record's body, not counting this header (confirmed) |
| 2 | word | rtype | The record's type (confirmed for 0, 1, 3, 4; the rest are the readers') |
| 4 | long | flags | Flags (readers): BADDATA, DIRECTORY, NONSEQUENTIAL, BLOCKERRS, ALIAS_ENTRY, HEADONLY, taken to be bits 0 to 5 in that order (guess). Zero in every record here |
| 8 | long | address | The first virtual block of a data record (confirmed), or an LBN; 0 otherwise |
| 12 | word | blockflags | A per-block error mask (readers). Zero here |
| 14 | word | | Reserved. Zero here |

Record types (the readers' list, with what the fixture has):

| Type | Meaning | In the fixture |
| --- | --- | --- |
| 0 | Null: fills the rest of the block. Its size runs exactly to the block's end | One per block, after the last file's data |
| 1 | The save set summary | First in each block |
| 2 | A volume summary (image backups) | |
| 3 | A file's attributes | Before each file's data |
| 4 | A file's data (virtual blocks) | After each file's attributes: the file's used blocks, starting at its address (VBN 1). Byte for byte the file's blocks |
| 5 | Physical volume attributes | |
| 6 | Volume blocks (LBNs) | |
| 7 | A file identification | |
| 8 | A file extension record | |
| 9 | 576-byte LBNs | |
| 10 | RSTS directory attributes | |
| 11 | A file alias entry | |

A file save uses types 0, 1, 3, and 4 (the probe saw no others). Only
block 1 holds the summary (answer 4; the survey's guess that it repeats
in every block came from `pi.bck`'s XOR block, a copy of block 1). A
file's data records hold its used blocks only, up to its end of file
(PI.OBJ: 5 of 6 allocated). A file larger than the room left in a block
goes on in the next block in a new data record (answer 5).

#### Attribute lists

The summary's and a file record's bodies start with a word, %X0101 (a
structure level, guess), followed by attributes. Each attribute is a
word giving the value's size, a word giving its type, and the value.
The list ends at a zero size and type (the summary's) or at the
record's end (a file's).

**The summary's attributes**, in the order the listing shows them (the
names are the readers'):

| Type | Name | Size | Value here | Listing line |
| --- | --- | --- | --- | --- |
| 1 | SSNAME | 6 | `PI.BCK` | Save set |
| 2 | COMMAND | 27 | `BACKUP [.PI]*.* PI.BCK/SAVE` | Command |
| 3 | COMMENT | — | absent | (`/COMMENT`'s text; the probe's B69COM) |
| 4 | USERNAME | 12 | `SYSTEM` blank-padded to 12 | Written by |
| 5 | USERUIC | 4 | [1,4] as member word, group word | UIC |
| 6 | DATE | 8 | 20-OCT-2026 10:45:22.88 | Date |
| 7 | OPSYS | 2 | %X0400 | Operating system (with attribute 8: `OpenVMS VAX version V7.1`) |
| 8 | SYSVER | 4 | `V7.1` | the version in the Operating system line |
| 9 | NODENAME | — | absent | (a node name) |
| 10 | SIR | 4 | %X040704D2 | CPU ID register |
| 11 | DRIVEID | 13 | `_SIMVAX$DUA1:` | Written on |
| 12 | BACKVER | 4 | `V6.2` | BACKUP version |
| 13 | BLOCKSIZE | 4 | 32256 | Block size |
| 14 | XORSIZE | 2 | 10 | Group size |
| 15 | BUFFERS | 2 | 8 | Buffer count |
| 16–19 | VOLSETNAM, NVOLS, BACKSIZE, BACKFILES | — | absent | (a volume set's name and size, the save set's size and file count) |

Types 20 to 41 are a volume summary's (image backups), out of scope.

**A file's attributes**, all present for each of the three files (names
are the readers'; "confirmed" means the fixture's values settle the
meaning). The table is in type order. The record's order is the same
for every file in every save set the probe made: 42, 43, 44, 46, 47,
52, 45, 48, 49, 50, 51, 75, 80, 87, 79, 53, 54, 55, 56, 57, 71, 72, 74,
then 73 for a directory and 78 for a file with an ACL:

| Type | Name | Size | PI.EXE | PI.MAR | PI.OBJ | Meaning |
| --- | --- | --- | --- | --- | --- | --- |
| 42 (%X2A) | FILENAME | var. | `[XFER.PI]PI.EXE;1` | ... | ... | The file's name as listed (confirmed) |
| 43 (%X2B) | STRUCLEV | 2 | %X0201 | %X0201 | %X0201 | The volume's structure level: ODS-2 |
| 44 (%X2C) | FID | 6 | (19,1,1) | (18,1,1) | (16,2,1) | The file ID, RVN 1, which DIRECTORY shows as 0 (confirmed) |
| 45 (%X2D) | BACKLINK | 6 | (17,1,0) | (17,1,0) | (17,1,0) | The directory's file ID (confirmed, [XFER.PI]) |
| 46 (%X2E) | FILESIZE | 4 | 6 | 24 | 6 | The allocation in blocks (confirmed) |
| 47 (%X2F) | UIC | 4 | [1,4] | [1,4] | [1,4] | The owner (confirmed) |
| 48 (%X30) | FPRO | 2 | %XFA00 | %XFA00 | %XFA00 | The protection: (S:RWED,O:RWED,G:RE,W) (confirmed) |
| 49 (%X31) | RPRO | 2 | 0 | 0 | 0 | The record protection |
| 50 (%X32) | ACLEVEL | 1 | 0 | 0 | 0 | The file's access level |
| 51 (%X33) | UCHAR | 4 | %X20 | 0 | 0 | The file characteristics: %X20 contiguous best try, which PI.EXE has (confirmed) |
| 52 (%X34) | RECATTR | 32 | | | | The record attributes, the file header's 32-byte FAT area: format, attributes, record size, highest and end-of-file blocks, first free byte, maximum record, default extend (confirmed: PI.OBJ's extend 20, PI.MAR's longest record 79) |
| 53 (%X35) | REVISION | 2 | 1 | 2 | 1 | The revision count (confirmed) |
| 54 (%X36) | CREDATE | 8 | | | | The creation time (confirmed) |
| 55 (%X37) | REVDATE | 8 | | | | The revision time (confirmed) |
| 56 (%X38) | EXPDATE | 8 | 0 | 0 | 0 | The expiration time |
| 57 (%X39) | BAKDATE | 8 | 0 | 0 | 0 | The backup time |
| 71 (%X47) | DIR_UIC | 4 | [1,4] | [1,4] | [1,4] | The directory's owner |
| 72 (%X48) | DIR_FPRO | 2 | %XBA88 | %XBA88 | %XBA88 | The directory's protection: (S:RWE,O:RWE,G:RE,W:E) |
| 74 (%X4A) | DIR_VERLIM | 2 | 32767 | 32767 | 32767 | The directory's version limit |
| 75 (%X4B) | VERLIMIT | 2 | 32767 | 32767 | 32767 | The file's version limit (DIRECTORY/FULL: "No version limit") |
| 79 (%X4F) | HIGHWATER | 4 | 7 | 25 | 7 | The highwater mark: the allocation plus one here |
| 80 (%X50) | JNL_FLAGS | 1 | 0 | 0 | 0 | Journaling flags |
| 87 (%X57) | RU_ACTIVE | 1 | 0 | 0 | 0 | The recoverable facility of active recovery units |

The readers name the types these files lack. Among them are 70
PLACEMENT, 73 DIR_STATUS, 76 and 77 the retention times, 78 ACLSEGMENT
(the probe's ACL.TXT), 81 the encryption key, 82 LBNSIZE, and 88
BACKREV, the backup revision count. Types 93 to 101 are ODS-5's and
later versions'.

The listing's size column is the used blocks, which the FAT's
end-of-file block and first free byte give: 6, 24, 5. Its date is the
creation time. "Total of 3 files, 35 blocks" is the used blocks' sum.

### What the fixture didn't settle (the probe's questions; answered below)

1. A file larger than a block, and data split across blocks or groups.
2. Redundancy groups of more than one block (group size 10 with 11 or
   more blocks, `/GROUP_SIZE=3`, `/GROUP_SIZE=0`): what the XOR block
   covers (its header? every data block of the group, padded?). What VMS
   says when it recovers a damaged block from its group, and when it
   can't.
3. The data CRC (`/CRC`, `/NOCRC`): what it covers, its initial value,
   and when BACKUP stores one.
4. Other block sizes (`/BLOCK_SIZE=8192`), and whether the volume file's
   record size always equals the block size.
5. Directory files saved by a `[dir...]` save (record flag DIRECTORY?),
   and how a restore makes directories from them.
6. Files with an ACL, an expiration date, a version limit, several
   versions, no blocks at all, NOBACKUP (HEADONLY?), and other record
   formats (fixed, VFC, stream, indexed): the attribute types missing
   above.
7. The summary's absent attributes (COMMENT, and others as found).
8. VMS's `/LIST/FULL` layout, `/LIST=file`, and the listing of a damaged
   save set.
9. Restores: every output-directory form, `/SELECT`, `/EXCLUDE`, the
   version qualifiers, `/BY_OWNER`, `/LOG`'s messages, an existing file
   without a version qualifier, a missing `/SAVE_SET`, a missing or
   non-save-set input, and the restored files' headers (DIRECTORY/FULL).
10. `/COMPARE`'s messages for a match, a difference, and a missing file.
11. Saving: `/LOG`'s messages, a save that selects no files, input lists.
12. Whether VMS's DIRECTORY/FULL shows "Contiguous best try" for an
    image (the probe's [B69]PI.EXE). govax's doesn't (side finding 2).

### The probe's answers (subtask 2, 2026-10-20)

VMS V7.1 (BACKUP V6.2) on the simh VAX ran `BACKUP69.COM` once, cleanly:
every command in Part 1 worked, and every case gave an answer.
`testdata/backup69/vax/` holds the log, the DIRECTORY/FULL reports, the
listings, and the 20 save sets, 118 blocks in all. The answers below
were checked on all of them with scratch scripts. The exchange volume
also holds VMS's restored trees; answer 13 used them.

1. **The data CRC is CRC-32**, the AUTODIN-II polynomial reflected
   (%XEDB88320), initial value all ones, the result complemented. It is
   the common CRC-32 (zlib's), taken over the whole block, header and
   all, with the CRC longword (offset 36) and the checksum word (254)
   taken as zero. Found by solving for the initial value over candidate
   regions. It holds for every block of B69CRC, its XOR block too.
   **This VAX's BACKUP writes a CRC only with `/CRC`.** The other 19
   save sets, the defaults included, have flag bit 0 (NOCRC) and a zero
   CRC. The command recorded in the summary shows no `/NOCRC`.
2. **The header checksum** is the CRC-16 of Part A, over the header
   with both the checksum word and the CRC longword zeroed. It holds
   for all 118 blocks but the one SSCOPY damaged in its header (B69HDRD
   block 2), and the CRC-32 for all of B69CRC's but the one damaged in
   B69CRCD.
3. **XOR blocks.** A group's XOR block follows its last data block.
   Its body (bytes 256 to the end) is the XOR of the group's bodies.
   Its header is the XOR of the group's headers, with its own size,
   opsys, subsystem, application (2), number, flags, CRC, and checksum
   written over. So in a group with an even number of blocks the
   struclev, block size, and save set name come out zero, and in one
   with an odd number they read normally. The last group may be short
   (B69K8: groups of 10, 10, and 5). `/GROUP_SIZE=0` writes no XOR
   blocks. Block numbers count XOR blocks too (B69G3: data 1–3, XOR 4,
   data 5–7, XOR 8).
4. **The summary is in block 1 only.** A later data block starts with
   whatever record comes next, and its header's "current file" fields
   (offsets 80 to 253) name the file being saved: its FID (with RVN 1,
   as the file record's FID has too, which DIRECTORY shows as 0;
   corrected in subtask 4), directory ID, name as
   a counted string, record type, record attributes, record size,
   maximum record, and allocation. They are zero in block 1.
5. **Data records never cross a block's end.** A file's data goes on in
   the next block as a new VBN record whose address is the next VBN
   (CONTIG.DAT: VBNs 1–60 in block 1, 61 on in block 2), with no new
   attribute record. A data record holds whole blocks. The null record
   fills what's left.
6. **Directories are saved as files** in a `[dir...]` save. They are
   listed in their place (`[B69]SUB1.DIR;1`, 1 block), and their
   attribute and data records have record flag 2 (DIRECTORY). Their
   attributes add DIR_STATUS (73) = %X0F. A save that selects no files
   (`/BEFORE=TODAY`) still saves the directories it walked, and writes a
   save set (B69NONE).
7. **NOBACKUP files** are saved as an attribute record with no data
   records, and listed with their size (1). The save says
   `%BACKUP-I-NOBACKUP, DUA1:[B69]NOBACK.TXT;1 data not copied, file
   marked NOBACKUP`, and the restore says `%BACKUP-S-HEADCOPIED`.
   `/IGNORE=NOBACKUP` saves the data.
8. **An ACL** is attribute 78 (ACLSEGMENT): the ACEs packed, each
   identifier ACE 12 bytes (size, type 1, flags, access mask, and the
   UIC as member word and group word).
9. **The summary:** `/COMMENT` adds attribute 3 after the command.
   `/BY_OWNER` on the output save set adds nothing to the summary and
   sets the save set file's owner. `/BLOCK_SIZE=8192` makes the buffer
   count 32 (attribute 15).
10. **The save set file:** fixed-length records of the block size
    (8192 for B69K8), sized to the blocks written (B69NONE: 2 blocks,
    126 disk blocks).
11. **Messages and statuses** (from the log; codes as `$STATUS`
    showed them, with STS$M_INHIB_MSG, bit 28, set; facility 163,
    %XA3, BACKUP, which this answer first gave as 266 by reading bit 28
    as part of it, corrected in subtask 6):
    - `%BACKUP-S-CREATED, created file`, `%BACKUP-S-CREDIR, created
      directory dir`, `%BACKUP-S-COPIED, copied file` (a save with
      `/LOG`), `%BACKUP-S-COMPARED, compared file`,
      `%BACKUP-S-HEADCOPIED`, `%BACKUP-I-NOBACKUP`;
    - `%BACKUP-F-LISINPSAV, /LIST requires save set as input`
      (%X10A3808C);
    - `%BACKUP-F-OPENIN, error opening file as input` with
      `-RMS-E-FNF` (%X10A3800C);
    - `%BACKUP-F-NOTSAVESET, file is not a BACKUP save set`
      (%X10A38104);
    - `%BACKUP-W-NOFILES, no files selected from saveset` (%X10A38400);
    - `%BACKUP-E-OPENOUT, error opening file as output` with
      `-RMS-E-FEX, file already exists, not superseded`, for an
      existing file without a version qualifier (then NOFILES);
    - `%BACKUP-E-VBNMISSING, file has missing blocks 11 through 74`
      (%X10A3822A), for the truncated save set;
    - `%BACKUP-I-XORERRS, 1 error recovered by redundancy group in
      saveset`, for the damaged header;
    - `%BACKUP-E-VERIFYERR, verification error for block 1 of file`
      (%X10A38272), a compare with a difference;
    - `%BACKUP-E-OPENIN` with `-RMS-E-FNF` and NOFILES (%X10A3800A), a
      compare with the file missing.
12. **Version qualifiers:** `/NEW_VERSION` makes `;2`, and `/OVERLAY`
    and `/REPLACE` write `;1` again (both `%BACKUP-S-CREATED`). `[*...]`
    restores to the saved directory. The `/BY_OWNER` keywords, the
    output-directory forms, `/SELECT`, and `/EXCLUDE` all ran. Their
    results are in the log and in `r1.lst` and `r7.lst`.
13. **Damaged data is restored without complaint, CRC or not.** VMS
    listed and restored B69CRCD, B69G0D, and B69NOCRCD with no message
    and success, and the restored CONTIG.DAT on the exchange volume has
    the changed byte in all three, B69CRCD's included, although its CRC
    shows the damage. The damaged header (B69HDRD) was found and
    recovered from its XOR group. The second round (`round2.log`)
    explains it: **VMS 7.1 checks the data CRC only when `/CRC` is given
    on the input save set.** No BACKUP symbol is defined. With `/CRC`,
    the restore of B69CRCD ends `%BACKUP-I-XORERRS, 1 error recovered by
    redundancy group in DUA1:[SETS]B69CRCD.BCK;1` (after the last file's
    CREATED line), and the restored CONTIG.DAT on the exchange volume
    matches the original. So on this VAX `/NOCRC` is the default both
    for writing and for reading, where the 7.3-2 manual says `/CRC`
    (question 8).
14. **DIRECTORY/FULL** of [B69]PI.EXE ends its File attributes line with
    ", Contiguous best try", which govax's leaves out (side finding 2,
    confirmed).

### Side findings (2026-10-20)

1. **COPY's destination ignores govax's host-name rule 3.** `COPY
   DUA1:[XFER]PI.BCK "./pi.bck"` wrote the volume file
   `DUA0:[WORK]./pi.bck;1` (a name with a `/` in it). The user's manual,
   section 5, says a name with `/` is a host file. Only `/HOST` reached
   the host. `dclcmd.Context.Copy` hands both names to
   `rms.Session.Copy` without `Session.Locate`'s rules. Subtask 3 fixes
   it (the author). Host COPY turns records into lines unless
   `/BINARY` is given; with it, a save set comes out byte for byte.
2. **DIRECTORY/FULL is missing "Contiguous best try"** on the File
   attributes line for a file with FCH$V_CONTIGB (PI.EXE). Confirmed by
   the probe's `tree.lst` (answer 14). A small fix, made with subtask 3.

## Part B — decisions and questions

### Decisions (accepted by the author, 2026-10-20)

1. **The probe comes first, then the reader.** As in Phases 53 to 68,
   real VMS's output settles what the fixture can't. Subtask 2 writes
   the probe, and the author runs it on the simh VAX. The reader
   (subtask 4) can start on `pi.bck` while it waits.
2. **A leaf package, `internal/backup`,** holds the format: reading
   blocks (the header and its checksum, the data CRC when present, XOR
   recovery), records, the summary, and file attributes, decoded into Go
   types (`Summary`, `File` with an `rms`-independent attribute struct)
   from any `io.ReaderAt`, and writing them (subtask 11). It knows
   nothing of volumes or DCL, like `internal/obj` and `internal/lbr`.
3. **The block size comes from the first block's header**, not the file's
   record attributes, so a save set whose attributes were lost (a host
   file, or a volume file FTP'd as fixed 512) still reads. A volume file
   whose record size disagrees is read anyway. VMS's own message for it,
   if the probe shows one, is noted as a difference.
4. **Restoring is RMS's work** (`internal/rms`, the only package that
   may import ods2): a new `Session` method makes a file on a volume
   with a save set's attributes and data. `internal/dclcmd/backup.go` is
   the command: it parses the specifiers, selects files, maps names to
   output names, and prints. That follows COPY's split.
5. **Messages are VMS's**, as `%BACKUP-` lines, with the texts from the
   probe's log and the codes unconfirmed (question 4).
6. **Fidelity:** listings and messages line for line, with times masked
   only where they're the run's own. Restored files block for block,
   with DIRECTORY/FULL matching VMS's restored copies. govax keeps what
   VMS keeps (the dates, revision count, owner, protection), and the
   probe shows what VMS resets. govax's save sets of the probe's files
   should match VMS's but for times, file IDs, and the system's own
   fields (CPU ID, device, node).
7. **Saving is in** (question 1), after list and restore work.
8. **A save set may be a host file** (question 2), read as raw bytes in
   blocks: `saveset.bck/HOST/SAVE_SET`, or a name govax's rules make a
   host file.
9. **Restoring to the host follows COPY's rule** (question 3): a text
   file's records become lines, other files are raw bytes.
10. **`/COMPARE` is in** (question 5). The rest of question 5's list stays
    out (Goal).
11. **The public readers may be consulted** (question 6), for layouts and
    names, with no code copied. The probe confirms what they say.
12. **Side finding 1 is fixed in this phase** (question 7), as a small
    subtask.

### Questions for the author (answered 2026-10-20)

The author took each proposal and added `/COMPARE`.

1. **Saving too?** Should the phase also write save sets? *Yes, after
   list and restore work, so VMS can read govax's save sets in a second
   round.*
2. **Host save sets?** May a save set be a host file, read as raw bytes
   in blocks? *Yes. SIMH tape images would be a later phase.*
3. **Restoring to the host:** COPY's rule (text files as lines, others
   raw), or raw blocks always? *COPY's rule.*
4. **BACKUP's message texts and codes:** add VMS's message definitions
   with `internal/vmsdef/gen`, or take the texts from the probe's log?
   *The probe's log.*
5. **Out of scope** (image, physical, incremental, `/RECORD`,
   `/JOURNAL`, `/VERIFY`, `/DELETE`, tapes, multivolume, `/CONFIRM`,
   ODS-5, `/INTERCHANGE`); should `/COMPARE` be in? *`/COMPARE` is in.*
6. **Public reverse-engineered descriptions** such as `vmsbackup`: may
   the phase consult them? *Yes.*
7. **Side finding 1** (COPY to a `/`-named host file): fix it here or
   separately? *Here, as a small subtask.*

### The probe's question (answered by the second round, 2026-10-20)

8. **Does govax check the CRC on input?** VMS restored B69CRCD's damaged
   block silently, though the CRC shows the damage and the manual says
   `/CRC` is the input default (answer 13). *The second round settled
   it: VMS 7.1 checks the CRC only when the input save set has `/CRC`,
   and then recovers the block from its group (XORERRS).* govax does as
   VMS 7.1 did: `/NOCRC` is the default for writing and for reading.
   With `/CRC` on the input, a block whose CRC fails is recovered from
   its group, as a block whose header checksum fails always is. The
   header checksum is checked whatever `/CRC` says, as VMS found
   B69HDRD's damage without it.

### The probe's cases (subtask 2; `testdata/backup69`)

`testdata/backup69/README.md` lists the files and each part's cases in
full. In brief: one procedure, `BACKUP69.COM`, run once on a fresh RD54
exchange volume, does six parts.

1. **The tree:** it makes `[B69...]`: text files of each record format,
   an indexed file, an empty file, an image, files larger than a save
   set block, and files with an ACL, an expiration date, a version limit,
   several versions, another owner, NOBACKUP, a protection, and
   contiguous allocation. `TREE.LST` is their DIRECTORY/FULL.
2. **Saves:** the tree with the defaults and with each output save-set
   qualifier (`/BLOCK_SIZE`, `/GROUP_SIZE` 0 and 3, `/CRC`, `/NOCRC`,
   `/COMMENT`, `/BY_OWNER`), the input file-selection qualifiers
   (`/EXCLUDE`, `/BY_OWNER`, `/SINCE`, `/BEFORE` selecting nothing),
   `/IGNORE=NOBACKUP`, an input list, and `/LOG`.
3. **Listings:** every save set listed, plus `/FULL`, `/BRIEF`,
   `/LIST=file`, and listings with `/SELECT` and `/EXCLUDE`.
4. **Restores**, all with `/LOG`: every output-directory form, `/SELECT`,
   `/EXCLUDE`, a new name, the `/BY_OWNER` keywords, an existing file
   with and without each version qualifier, and the save sets of every
   format. DIRECTORY/FULL of what they made.
5. **Compares:** a match, a changed file, and a missing file.
6. **Errors:** a missing `/SAVE_SET`, a missing save set, a file that
   isn't one, and a `/SELECT` matching nothing. SSCOPY (`sscopy.mar`, a
   small MACRO program run on the VAX, tested on govax first) makes a
   truncated save set and four damaged ones from VMS's own: a data byte
   changed with `/CRC`, with no XOR group, and with `/NOCRC`, and a
   header byte changed. Each is listed and restored.

`copyout.cmd` brings back the log, the listings, the DIRECTORY/FULL
reports, and every save set byte for byte (COPY/BINARY).

## Part C — subtasks

1. **The survey** — done (2026-10-20). Part A: the fixture's format, the
   public readers' names, the sources, what earlier phases provide, the
   side findings, the plan.
2. **The VMS probe** — done (2026-10-20). `testdata/backup69`: the
   procedure, SSCOPY, the exchange volume, the author's run, and Part
   A's "The probe's answers".
3. **COPY to a host name** — done (2026-10-20). Side finding 1: COPY's
   source and destination follow `Session.Locate`'s rules, as the
   user's manual says, so `"./pi.bck"` is a host file. Side finding 2:
   DIRECTORY/FULL's characteristic words, among them "Contiguous best
   try".
4. **`internal/backup`, reading** — done (2026-10-20): blocks (header, checksum, data CRC,
   flags), records, the summary, file attributes into Go types, a file's
   data in VBN order, and XOR recovery of a damaged block from its group.
   Tested on `pi.bck` (the files match `pi.exe`, `pi.mar`, `pi.obj`),
   then on the probe's save sets, including the damaged ones.
5. **Opening a save set** — done (2026-10-20): a volume file through RMS (block I/O, record
   attributes checked) or a host file, as an `io.ReaderAt` of blocks.
   The `/SAVE_SET` rule and its errors.
6. **The command** — done (2026-10-20): `BACKUP` in `console.dcl`
   (input and output specifiers, command qualifiers, and
   position-dependent qualifiers), `HELP BACKUP` in `console.help`, and
   BACKUP's messages (from the probe's log) in `vmsdef`, where VMS's
   condition values live, rather than `vmserrors`.
7. **BACKUP/LIST** — done (2026-10-20): the summary block and file lines in VMS's layout,
   `/BRIEF`, `/FULL` (sharing DIRECTORY/FULL's writer where the layouts
   agree), `/LIST=file`, `/SELECT` and `/EXCLUDE`, the totals, and "End
   of save set". Checked against the probe's listings.
8. **Restoring a file** (`internal/rms`): a new file on a volume with
   the saved record attributes, characteristics, owner, protection,
   dates, revision count, version limit, ACL, allocation, and data up to
   its end of file. Directories made as needed. Host output by COPY's
   rule. ods2 gains what's missing (a release tagged and pinned as
   `docs/PHASE-22.md` describes).
9. **BACKUP's restore:** the output-directory rules (7.3.3), `/SELECT`,
   `/EXCLUDE`, the existing-file rules and `/NEW_VERSION`, `/OVERLAY`,
   `/REPLACE`, `/BY_OWNER`, and `/LOG`. Checked against the probe's
   restores, file for file and header for header.
10. **BACKUP/COMPARE:** a save set's files against files on a volume or
    the host, with VMS's messages for a difference and a missing file.
11. **Saving:** `internal/backup`'s writer (summary, file records, data
    records split as VMS splits them, null fill, XOR blocks, header
    checksum, data CRC) and the save command with its output save-set
    qualifiers and input file-selection qualifiers.
12. **The oracle:** `TestBackup69Oracle` runs the probe's procedure on
    govax and matches VMS's log and listings, the restored files, and
    their headers, with a listed set of differences; govax's save sets
    of the probe's tree against VMS's. Then a second VAX round in which
    VMS lists, restores, and compares govax's save sets, replayed by
    `TestBackup69Round2`.
13. **The close-out:** `docs/MANUAL.md` (BACKUP, with examples run on
    govax), `HELP BACKUP`, `CLAUDE.md`, `docs/PLAN.md`, and any
    `docs/DEVIATIONS.md` entries.

## Progress log

### 2026-10-20 — Planned; subtask 1, the survey

The author asked for BACKUP save sets (list and restore, VMS syntax)
and provided `xfer.dsk` with `[XFER]PI.BCK`, a save set of `[XFER.PI]`'s
three files, plus VMS's DIRECTORY/FULL and BACKUP/LIST of it.

- Copied the save set and the three files off the container block for
  block, with a small ods2 program in the scratchpad: govax's COPY to
  the host converts records to lines, and `"./name"` didn't reach the
  host (side finding 1). Removed the stray files those attempts left in
  `DUA0:[WORK]`. Added them as `testdata/backup69/vax/` with a README.
- Decoded the save set with a scratch walker (Part A): the block header,
  the four record types, the summary's and file records' attribute
  lists. The data records match the original files byte for byte. The
  second block is the first's XOR block. The six times match VMS's
  listing.
- Found the header checksum: CRC-16, polynomial %XA001, initial 0, over
  the header with the checksum word zeroed. Found no data CRC in the
  save set, although the command didn't say `/NOCRC` (Part A; the probe
  asks).
- Found the BACKUP chapter of the *System Management Utilities
  Reference Manual* (the file named `VMS SYSAdmin I.pdf`) for the
  command's syntax and qualifiers.
- Wrote Parts B and C. Paused for the author's review.

### 2026-10-20 — Plan accepted; subtask 2's probe written

The author accepted every proposal, added `/COMPARE`, allowed the public
save set readers, and asked for side finding 1's fix in this phase.
Parts B and C now say so. Saving, `/COMPARE`, and the COPY fix are
subtasks 11, 10, and 3.

- **The public readers** (Part A's sources) name every field of the
  block and record headers, the record types, and the attribute codes.
  Each agrees with the fixture: offset 6 is the application, 1 for a
  data block and 2 for an XOR block; flag bit 0 is NOCRC, so the
  fixture's save set says it has no CRC; and the file attributes Part A
  had marked unknown are the record protection, access level, the
  directory's owner, protection, and version limit, the file's version
  limit, highwater mark, journaling flags, and recovery-unit facility.
  None checks the header checksum or the CRC, or recovers from an XOR
  block, so the checksum found in subtask 1 is new.
- **The probe** (`testdata/backup69`): `backup69.com`, `sscopy.mar`,
  `exchange.cmd`, `copyout.cmd`, and the README's tables. SSCOPY copies
  a save set record by record, cut short or with one byte complemented,
  so the damaged save sets come from VMS's own in the same run. It was
  assembled, linked, and run on govax against `pi.bck`: the changed byte
  was 32256 + 5000, complemented, and the cut copy was block 1 alone,
  with VMS's record size. The whole procedure was dry-run on govax.
  govax lacks BACKUP, CONVERT, SET FILE, SET SECURITY, SET PROTECTION,
  and COPY/CONTIGUOUS, so those lines failed as expected, and the rest
  (the loops, MACRO and LINK of PI and SSCOPY, SSCOPY's data lines) ran
  as written. MEDIUM.DAT came out 74 blocks, BIG.DAT 169.
- COPY/BINARY copies a save set off a volume byte for byte (checked on
  `pi.bck`), so `copyout.cmd` uses it.
- Built `testdata/disks/backup69-exchange.dsk` (RD54, BACKUP69) for the
  author's run.

### 2026-10-20 — Subtask 2: the probe's VMS run

The author ran `@BACKUP69` on the simh VAX (V7.1). `copyout.cmd` brought
back the log, `tree.lst`, `sets.lst`, `r1.lst`, `r7.lst`, `text.lis`,
`b69full.lis`, and all 20 save sets (B69NONE too: VMS writes a save set
when no files are selected). Every command of Part 1 worked, and every
case gave an answer.

- Found the data CRC: standard CRC-32 over the block with the CRC and
  checksum fields zeroed, by solving for the initial value over
  candidate regions. It's written only with `/CRC`. Corrected the
  checksum's rule: it leaves out the CRC longword too. Both rules hold
  for every block of every save set but the two SSCOPY damaged.
- XOR blocks are the XOR of their group, header and body, with the
  header's own fields written over (answer 3). The summary is in block 1
  only, and later blocks' headers name the file being saved (answer 4),
  correcting the survey's guess. Data records stay within a block
  (answer 5).
- Directories, NOBACKUP files, ACLs, `/COMMENT`, and the save set file
  itself (answers 6 to 10). VMS's messages and statuses for every case
  (answer 11).
- Checked VMS's restored trees on the exchange volume: the damaged data
  byte reached CONTIG.DAT in all three data-damage cases, the `/CRC` one
  included, with no message. Question 8 asks how govax should treat a
  bad CRC, and proposes a short second VAX round.
- VMS's DIRECTORY/FULL shows "Contiguous best try" for PI.EXE, which
  confirms side finding 2. It goes with subtask 3.

### 2026-10-20 — Subtask 3: COPY to a host name, and DIRECTORY/FULL's characteristics

- **COPY** (`internal/dclcmd/copy.go`): `Context.Copy` now decides each
  name's side with `rms.Session.Locate`, as every other command does
  (`copyFiles`), and hands the results to `Session.Copy` as its explicit
  host flags. `/HOST` still forces a host name, a VMS specification is a
  volume file, a name with `/` or `\` is a host file, and a bare name
  goes with the default directory: on its volume after a SET DEFAULT
  onto one, the host otherwise. Before, a name without `/HOST` was
  always a volume file, so `COPY ... "./pi.bck"` made a volume file of
  that name. `Session.Copy` is unchanged. `TestDispatch_copyHostPathWithoutHost`
  (`internal/console/copy_test.go`) copies to and from a quoted host
  path without `/HOST` and checks that no volume file was named after
  it. It fails without the change. The user's manual's "Moving files"
  section says so, with an example run on govax.
- **DIRECTORY/FULL** (`internal/rms/directory.go`): the File attributes
  line's characteristic words are a table (`characteristicWords`) in bit
  order. It now has VMS 7.1's "Contiguous best try", "Contiguous", and
  "Directory file" (`tree.lst`) beside the page file's "Backups
  disabled" and "MoveFile disabled". A directory's version limit reads
  "Default version limit: n", 32767 when it has none
  (`versionLimitText`), as VMS shows a directory. The order of
  characteristics no example shows together (Backups disabled with
  Contiguous, say) is taken to be by bit, unconfirmed.
  `TestCharacteristicsText` and `TestVersionLimitText`.
- Checked end to end on `xfer.dsk`: `COPY/BINARY DUA1:[XFER]PI.BCK
  "./pi2.bck"` writes the host file, byte for byte, and leaves the
  default volume alone. DIRECTORY/FULL of PI.EXE and PI.DIR prints VMS's
  lines.

### 2026-10-20 — Subtask 2: the second round (question 8)

The author ran the two commands question 8 asked for (`round2.log`).
No BACKUP symbol is defined. With `/CRC` on the input save set, the
restore of B69CRCD reports `%BACKUP-I-XORERRS` after the last file, and
the restored CONTIG.DAT on the exchange volume (`[X.B69]`) matches the
original, the damaged block rebuilt from its group. So VMS 7.1 checks
the CRC only when told, and recovers a block whose CRC fails as it does
a block whose header checksum fails. Question 8's answer, and answer 13,
say so. govax follows VMS 7.1, defaulting to `/NOCRC` both ways.

### 2026-10-20 — Subtask 4: `internal/backup`, the reader

A leaf package (standard library only), `internal/backup`:

- `block.go`: the block header (`ParseHeader`, `BlockHeader`, its
  current-file fields as `CurrentFile`), `Plausible` (size 256,
  subsystem 1, a block size from 2048 to 65535), the NOCRC flag.
- `crc.go`: `HeaderChecksum` (CRC-16, %XA001, table-driven) and
  `DataCRC` (`hash/crc32`'s IEEE CRC-32), each with the CRC longword and
  checksum word taken as zero.
- `record.go`: `Records` splits a block's records (packed with no
  padding, a null record or the block's end ends them), the record
  types, and the record flags.
- `attributes.go`: attribute lists, `Summary` (every summary attribute)
  and `Attributes` (every file attribute the probe's save sets carry,
  the FAT area kept whole with accessors for its fields, `Used`, ACL
  segments packed), each with the raw `List` for later use (the writer).
- `reader.go`: `Read(io.ReaderAt, size, Options)` reads every block,
  repairs what it can, and decodes the records into `SaveSet.Files`.
  A block is damaged when its header checksum fails, or with
  `Options.CheckCRC` (`/CRC` on the input) its data CRC. `repair`
  rebuilds a damaged data block as the XOR of its group's other blocks
  and its redundancy block, writing back the header fields the
  redundancy block wrote over (size, opsys, subsystem, application,
  number, flags) and recomputing the checks. The group's place comes from
  the summary's group size, or, if block 1 is the damaged one, from the
  first intact redundancy block. A block that can't be rebuilt (no
  group, or two damaged in one) is `Lost`; the files whose data it held
  miss those blocks, and the files whose records it held are gone (after
  a lost block, data records are kept only if the block header's current
  file is the file being read). `Recovered` lists the rebuilt blocks'
  numbers, so the listing can place XORERRS as VMS does: after the files
  of the group (B69HDRD's listing prints it after the last file, before
  the total). `File.Data` gives the used blocks in VBN order, zeros where
  missing; `File.Missing` the VBN ranges VBNMISSING names;
  `File.HeaderOnly` a NOBACKUP file saved without its data (whose record
  has no HEADONLY flag: no record flag but DIRECTORY was seen).
  `ErrNotSaveSet` for a first block that isn't a BACKUP block or holds
  no summary.
- **Tests** (`backup_test.go`): `pi.bck`'s summary, attributes, and data
  against VMS's listing and the original files; every block of all 21
  save sets passes its checks but the two SSCOPY damaged
  (`TestChecks`); every probe save set's files, sizes, and total match
  VMS's plain BACKUP/LIST in the log (`TestVAXListings`); large files
  split across blocks read the same from every block and group size
  (`TestSplitData`); directories, NOBACKUP, the ACL, an empty file, an
  owner (`TestFileKinds`); the comment, buffer count, and group sizes;
  the current-file header fields; B69HDRD and (with `/CRC`) B69CRCD
  repaired byte for byte into B69 and B69CRC, B69CRCD without `/CRC`
  and B69G0D and B69NOCRCD read with the damage (`TestRecovery`); a
  lost block in B69G0 (`TestLostBlock`); B69CUT's MEDIUM.DAT missing
  blocks 11 through 74, as VMS's VBNMISSING said (`TestTruncated`);
  NOTSAVESET for a text file, an image, nothing, and zeros.
- **Corrections to Part A:** the file record's FID carries RVN 1, like
  the block header's (answer 4 said 0: DIRECTORY shows it as 0). The
  probe's UICs are octal: OWNED.TXT's [200,201] is %X00800081.
- **For subtask 7:** a save set with `/GROUP_SIZE=0` lists no "Group
  size" line (B69G0's listing). B69CUT's listing shows every file the
  two blocks hold, MEDIUM.DAT with its full 74 blocks, and no message.


### 2026-10-20 — Subtask 5: opening a save set

- **`rms.Session.OpenSaveSet`** (`internal/rms/saveset.go`): a
  `SaveSetFile`, an `io.ReaderAt` of the save set's bytes. A volume file
  is accessed for reading, sharing reads only, while it's open (govax's
  choice; VMS's sharing for a save set read is unconfirmed), and read a
  virtual block at a time up to its end of file; a host file is read as
  it is. `Name` is the file found with its version
  ("DUA0:[000000]PI.BCK;1", NOTSAVESET's name). `Format` and
  `RecordSize` are the volume file's record attributes, and
  `RecordsMatch(blockSize)` says whether they're VMS's (fixed-length
  records of the block size); a host file matches. A missing file is
  `NotFound`'s `*DCLFileError` (RMS$_FNF or RMS$_DNF with the expanded
  name, "DUA0:[000000]NONE.BCK;", as VMS's OPENIN shows it), a wildcard
  RMS$_WLD (a save set on a disk is one file; VMS's message unconfirmed),
  a missing host file RMS$_FNF with its path, and a file another opener
  writes RMS$_FLK.
- **The `/SAVE_SET` rule** (`internal/dclcmd/backupset.go`):
  `BackupOperationFor` takes the two `BackupSpecifier`s (text, `/HOST`,
  `/SAVE_SET`) and `/LIST` and `/COMPARE`, and gives Table 7-1's
  operation: a save set as input is a restore, or a compare with
  `/COMPARE`; a save set as output is a save; `/LIST` with no output
  specifier lists the input, which must be a save set
  (`ErrListNeedsSaveSet`, LISINPSAV); `/LIST` with an output specifier
  is the specifiers' operation, listed too. Both save sets
  (`ErrBothSaveSets`, which the manual forbids), neither
  (`ErrNoSaveSet`, a file copy, out of scope), and `/COMPARE` with the
  save set as output (`ErrCompareToSaveSet`, not in Table 7-1) are
  errors whose VMS messages are unconfirmed.
- **`Context.ReadSaveSet`** opens the input save set by `Locate`'s rules
  and reads it with `backup.Read` (`/CRC` as `Options.CheckCRC`), then
  closes it. A `SaveSetInput` has the found `Name`, the `Expanded` name
  as given ("DUA1:[SETS]B69.BCK;", the name VMS's NOFILES shows), the
  `Set`, and `RecordsMatch` (a mismatch is read anyway, Decision 3).
  Failures are a `*SaveSetOpenError` (OPENIN: the name and RMS's error)
  and a `*NotSaveSetError` (NOTSAVESET: the file's full name, as VMS
  named `DUA1:[B69]TEXT.TXT;2`). Subtask 6 words them as `%BACKUP-`
  messages.
- **Tests:** `TestOpenSaveSetVolume` (PI.BCK copied onto a volume
  with COPY/BINARY reads byte for byte at offsets inside and across
  blocks and past the end; undefined records don't match, fixed 32256
  records do), `TestOpenSaveSetSharing`, `TestOpenSaveSetErrors`,
  `TestOpenSaveSetHost`; `TestReadSaveSet` (a volume and a host save
  set, their names, `/CRC` finding B69CRCD's damage only when given),
  `TestReadSaveSetErrors` (OPENIN with expanded names, NOTSAVESET for a
  volume text file and a host image), `TestBackupOperationFor`.

### 2026-10-20 — Subtask 6: the command

- **The grammar** (`console.dcl`'s BACKUP): INPUT (a list, prompt
  "From") and OUTPUT (no prompt: `/LIST` alone needs none) each carry
  their own qualifiers, so a qualifier's place says what it is about,
  as the manual's 7.2 describes: on INPUT `/SAVE_SET`, `/CRC`,
  `/SELECT`, `/EXCLUDE`, `/SINCE`, `/BEFORE`, `/BY_OWNER[=uic]`, the
  date kinds `/BACKUP`, `/CREATED`, `/EXPIRED`, `/MODIFIED`, and govax's
  `/HOST`; on OUTPUT `/SAVE_SET`, `/CRC`, `/BLOCK_SIZE`, `/GROUP_SIZE`,
  `/COMMENT`, `/BY_OWNER[=option]`, `/NEW_VERSION`, `/OVERLAY`,
  `/REPLACE`, `/HOST`; on the verb `/LIST[=file]`, `/FULL`, `/BRIEF`
  (not both), `/LOG`, `/COMPARE`, `/IGNORE=(NOBACKUP,INTERLOCK)`. Every
  probe command parses.
- **The DCL engine** (`internal/dcl`, in scope by the author's word):
  a parameter-scoped qualifier with a `/list` value kept only its first
  element; it now keeps them all (`ParamList`), for `/SELECT=(A,B)`. A
  parameter-scoped qualifier filled from its `/default=` can now be told
  from one given (`ParamDefaulted`, as `Defaulted` is at the verb's
  level), so `/BY_OWNER` absent isn't `/BY_OWNER` alone.
  `TestBackupShapedCommands` (`internal/dcl/backup_test.go`).
- **The command** (`internal/console/backup.go`'s `backupCommand`,
  `internal/dclcmd/backup.go`): the parse becomes a `BackupCommand`
  (the command line too, for a save's summary), with the input's date
  kind (`BackupDateKind`, `/MODIFIED` the default, as the manual's
  `/SINCE` says) and the output's existing-file choice
  (`BackupExisting`). Two of either is BADQUALIFIERCOMBO, checked in Go
  since a grammar DISALLOW knows only the verb's qualifiers (VMS's
  message unconfirmed). `Context.Backup` finds the operation
  (`BackupOperationFor`), reads the input save set for a list, restore,
  or compare, and then, until subtasks 7 to 11, ends with govax's
  `%BACKUP-F-NOTYET`. No output without `/LIST` is DCL's INSFPRM (at a
  terminal VMS asks for "_To:"; govax doesn't yet).
- **The messages** (`internal/vmsdef/backup.go`): BACKUP's, keyed by
  condition value as the system message file's are, so `LookupMessage`
  (and `F$MESSAGE`, `ConditionLine`) know them. OPENIN, OPENOUT,
  LISINPSAV, NOTSAVESET, VBNMISSING, VERIFYERR, and NOFILES have the
  codes `$STATUS` showed; CREATED, CREDIR, COPIED, COMPARED, HEADCOPIED,
  NOBACKUP, and XORERRS VMS's texts with govax's message numbers (they
  never reached `$STATUS`), counted down from %X1FFF; BOTHSAVSET,
  NOSAVESET, CMPINPSAV (the `/SAVE_SET` rule's other errors), and NOTYET
  are govax's, text and code. XORERRS's plural ("2 errors") is govax's
  guess. `FormatMessage` writes a message with its `!AS`, `!UL`, and
  `!%S` parameters. A failure is a `ConditionError` whose status is the
  code with STS$M_INHIB_MSG, so `$STATUS` reads as VMS's did
  (%X10A3808C for LISINPSAV); `dclcmd.BackupLine` is a message as a line
  of output, for `/LOG`.
- **Why `vmsdef`, not `vmserrors`:** `vmserrors`' codes are govax's own
  layout (facility at bit 20), which `dclshell.VMSCondition` maps to
  VMS's through `vmsdef.Symbols`, which has no `BACKUP$_` names. The
  messages' VMS codes belong beside the other messages VMS showed
  (`observedMessages`).
- **`HELP BACKUP`**, with a paragraph saying which operations are still
  to come, and BACKUP in `HELP`'s list.
- **Tests:** `TestBackupMessages` (vmsdef: the codes and every message
  as the log shows it); `TestBackupCommandParse` (the probe's commands
  into `BackupCommand`s, the conflicts); `TestBackupMessages` in
  `internal/console` (LISINPSAV, OPENIN with `-RMS-E-FNF`, NOTSAVESET
  for a volume and a host file, govax's three, NOTYET per operation,
  each with its `$STATUS`; INSFPRM); `TestBackupMessageShown` (in a
  procedure, the message shown once and the procedure ended with its
  status). The verb count and the parameter-qualifier walk in
  `internal/dcl`'s tests know BACKUP.

### 2026-10-20 — Subtask 7: BACKUP/LIST

- **The listing** (`internal/dclcmd/backuplist.go`, `listSaveSet`):
  "Listing of save set(s)" and a blank, written before the save set is
  opened (VMS shows OPENIN and NOTSAVESET after it); the summary, each
  label padded to 19 columns; a line per file (`/BRIEF`, the default:
  the name in 52 columns, the used blocks in 9, the creation time to the
  minute) or a block per file (`/FULL`); a blank; "Total of n files, m
  blocks" (the used blocks, `plural`'s "1 file, 1 block"); and "End of
  save set" with a CR LF of its own, as `$FAO`'s `!/` writes it (the log
  and the `.LIS` files show a CR and then an empty line). `$STATUS` is
  %X10000001, as VMS's was. `/LIST=file` writes the lines as a text
  file, type LIS by default (DIRECTORY/OUTPUT's writer), and BACKUP's
  messages among them still go to SYS$OUTPUT (unconfirmed: the probe's
  `/LIST=file` commands had none).
- **`/FULL` isn't DIRECTORY/FULL's layout**, so it shares nothing with
  `rms`'s writer: three columns (22, 24, the rest), the name on a line of
  its own when longer than 21 characters (MEDIUM.DAT's 22; 21 is
  unconfirmed), the owner as six-digit octal halves, the file ID with
  its RVN ("(36,1,1)"), "Allocation = 3, Extend = 0" (", maximum bucket
  size = 2" for an indexed file; for a relative one unconfirmed),
  "Global Buffer Count = 0" with ", Contiguous" or ", Contiguous-best-try"
  (nothing for NOBACKUP or a directory, and no version limit), the record
  format with ", maximum n bytes" when the file has a limit, and the
  record attributes "Carriage return", "Print file format", "Non-spanned",
  "None". The words for Stream, Stream_CR, Undefined, Fortran carriage
  control, combined attributes, and other characteristics are govax's
  guesses (unconfirmed). `backup.Attributes.GlobalBuffers` is new
  (FAT$W_GBC).
- **What VMS V7.1 showed:** the summary leaves out `/COMMENT` (B69COM's
  listings), a listing ignores `/SELECT` and `/EXCLUDE` (the probe's two
  such listings show all 25 files; `HELP BACKUP` says so), B69CUT lists
  its 18 files with MEDIUM.DAT at 74 blocks and no message, and B69HDRD's
  XORERRS comes after the last file, before the blank and the total,
  naming the save set with its version. govax reports XORERRS once with
  the count of rebuilt blocks; where VMS puts it for damage in several
  groups is unconfirmed, as is what a listing says of a lost block
  (govax: nothing).
- **Tests:** `TestBackupListOracle` (`internal/console/backuplist_test.go`)
  puts the 20 save sets on a fresh DUA1 in [SETS], runs each of the
  log's BACKUP/LIST commands but the two with `/LIST=file` and the one
  naming the [B69] tree (28 listings, errors included), and matches VMS's
  output line for line, with `$STATUS`; `TestBackupListFile` matches
  TEXT.LIS and B69FULL.LIS byte for byte as copied to the host, and a
  default LIS type; `TestBackupListHost` lists `pi.bck` as a host file
  (the README's listing); `TestBackupListingFields` (dclcmd) covers the
  file ID's high byte, protection, the UIC, and the record attributes the
  probe doesn't reach. `TestBackupMessages` no longer expects NOTYET for
  a listing.
