# Phase 69 — BACKUP save sets

**Status:** done (2026-10-20). Planned 2026-10-20; the plan reviewed and
accepted by the author the same day (Part B). Subtask 1, the survey, is
done: Part A holds what the first save set
(`testdata/backup69/vax/pi.bck`) and the public save set readers show of
the format. Subtask 2, the VMS probe (`testdata/backup69`), is done:
"The probe's answers" in Part A settles the format, the CRCs, the XOR
blocks, and VMS's messages. Subtasks 3 (COPY to a host name), 4 (`internal/backup`, the
reader), 5 (opening a save set), 6 (the command), 7 (BACKUP/LIST), 8
(restoring a file), 9 (BACKUP's restore), 10 (BACKUP/COMPARE), and 11
(saving), 12 (the oracle, with the second VAX round), and 13 (the
close-out) are done.

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
8. **Restoring a file** — done (2026-10-20) (`internal/rms`): a new file on a volume with
   the saved record attributes, characteristics, owner, protection,
   dates, revision count, version limit, ACL, allocation, and data up to
   its end of file. Directories made as needed. Host output by COPY's
   rule. ods2 gains what's missing (a release tagged and pinned as
   `docs/PHASE-22.md` describes).
9. **BACKUP's restore** — done (2026-10-20): the output-directory rules (7.3.3), `/SELECT`,
   `/EXCLUDE`, the existing-file rules and `/NEW_VERSION`, `/OVERLAY`,
   `/REPLACE`, `/BY_OWNER`, and `/LOG`. Checked against the probe's
   restores, file for file and header for header.
10. **BACKUP/COMPARE** — done (2026-10-20): a save set's files against files on a volume or
    the host, with VMS's messages for a difference and a missing file.
11. **Saving** — done (2026-10-20): `internal/backup`'s writer (summary, file records, data
    records split as VMS splits them, null fill, XOR blocks, header
    checksum, data CRC) and the save command with its output save-set
    qualifiers and input file-selection qualifiers.
12. **The oracle** — done (2026-10-20): `TestBackup69Oracle` runs the probe's procedure on
    govax and matches VMS's log and listings, the restored files, and
    their headers, with a listed set of differences; govax's save sets
    of the probe's tree against VMS's. Then a second VAX round in which
    VMS lists, restores, and compares govax's save sets, replayed by
    `TestBackup69Round2`.
13. **The close-out** — done (2026-10-20): `docs/MANUAL.md` (BACKUP, with examples run on
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

### 2026-10-20 — Subtask 8: restoring a file

- **ods2** (committed locally, not tagged or pushed; Decision 2's
  `docs/PHASE-22.md` step is the author's): `ondisk.FileHeader.ACL`
  reads the header's access control entries (up to a zero length
  byte), and `ondisk.ACLArea` pads a list to whole words. Every header
  rewrite (`existingAreas`: `UpdateHeader`, `Extend`, `Close`,
  `SetVersionLimit`) used to drop the ACL; it now keeps it.
  `volume.SetACL` sets or removes it. `rms.NewReaderFrom` reads records
  from any `BlockSource`, `rms.BytesSource` being a file's data in
  memory, so a saved file's records can be read with no volume.
- **`rms.Session.RestoreFile`** (`internal/rms/restore.go`) makes one
  `backup.File` at a `FileLocation`. On a volume: one name with no
  wildcards; the next version without one, and an existing version an
  `*OutputExistsError` with the full name (RMS$_FEX). The directories
  that don't exist are made first, with what a directory inherits from
  its parent, and given revision count 1, as VMS's BACKUP gave every
  directory it made (`r1.lst`'s [R1.B69], [R1.B69.SUB1]); they are
  returned top down for CREDIR. The file gets the saved FAT whole (end
  of file and longest record included), its characteristics (less
  marked-for-delete and directory), protection, record protection,
  access level, journaling and recovery-unit bytes, revision count, four
  dates, version limit (VERLIMIT, 32767 kept as 0, ods2's "none"), the
  saved allocation or more if the data needs it, the data, and the ACL.
  A contiguous file needs one free run as long as its allocation
  (`ErrNoContiguousSpace`, which wraps `ErrACPDeviceFull`; VMS's message
  unconfirmed). A header-only (NOBACKUP) file gets its allocation and
  end of file and no data, reading as zeros. A file that can't be made
  whole is deleted. A saved directory file (`SUB1.DIR`) is made as a
  directory with its saved default version limit (its FAT's, 5), owner,
  protection, and allocation, new dates, and revision 1; one that exists
  is left alone (`Created` false). `RestoreOptions.Owner` overrides the
  saved owner, for subtask 9's `/BY_OWNER`.
- **The high-water mark** is the restore's own (the block after the last
  written, 1 when none was), not the saved HIGHWATER: the blocks between
  the end of file and it weren't saved. What VMS records is unconfirmed.
- **To the host**, by COPY's rule (Decision 9): an object module's
  records (by the name's type) in the host's object layout, which is
  ODS-2's, so PI.OBJ comes out as VMS's bytes up to its end of file; a
  text file's records as lines (`restoredAsText`: a sequential format
  with implied, print, or FORTRAN carriage control, govax's line, since
  VMS has no host); anything else its bytes up to its end of file. The
  host file's time is the saved revision time. A saved directory is a
  host directory. An existing host file is an `*OutputExistsError`.
- **DIRECTORY/FULL** (`internal/rms/directory.go`), so a restored tree
  can be checked against VMS's: "Access Cntrl List:" shows the ACL
  (`aclText`: identifier ACEs as `(IDENTIFIER=[SYSTEM],ACCESS=READ+WRITE)`,
  later ones beneath the first, the identifier named as the owner is;
  the access words past READ and WRITE in an order that's unconfirmed,
  and any other kind of ACE in hexadecimal, govax's text); a directory's
  record attributes read "No carriage control, Non-spanned" (VMS's
  `tree.lst`; govax wrote "Non-spanned"); and an indexed file's
  attributes line has "Maximum bucket size: 2, " (a relative file's
  taken to be the same, unconfirmed). VMS's "Indexed, Prolog: 3, Using
  1 key" needs the file's prolog, which DIRECTORY doesn't read: a listed
  difference.
- **Tests** (`internal/rms/restore_test.go`): `TestRestoreTreeOracle`
  restores all 25 of B69.BCK's files under [R1] on a volume with VMS's
  3-block clusters, checks the four directories made, and matches
  DIRECTORY/FULL of [R1...] with VMS's `r1.lst` line for line, file IDs
  and the directories' dates masked, but for the prolog line (owner
  [1,4] given, as VMS's restore gave OWNED.TXT). `TestRestoreData` (data
  across save set blocks, a VFC file, an empty file, PI.EXE from
  `pi.bck` byte for byte against VMS's `pi.exe`; B69.BCK's PI.EXE and
  PI.OBJ were made again by the probe, so they aren't the fixture's),
  `TestRestoreOwnerKept`, `TestRestoreVersions`, `TestRestoreHeaderOnly`,
  `TestRestoreContiguousNoRoom`, `TestRestoreDirectoryExists`,
  `TestRestoreReadOnly`, `TestRestoreToHost` (lines for the text, VFC,
  and stream files alike, PI.EXE and PI.OBJ byte for byte, a directory,
  an existing file), `TestACLText`. ods2: `TestACLRoundTrip`,
  `TestACLNone`, `TestACLOddLength`, `TestACLTruncated`, `TestSetACL`
  (kept through `UpdateHeader` and `Extend`, removed by nil),
  `TestReaderFromBytes`, `TestBytesSourceReadBlock`.
- **For subtask 9:** VMS's default restore gave OWNED.TXT [SYSTEM], not
  its saved [200,201] (`/BY_OWNER`'s default); the R1 restore's log has
  no CREDIR for [R1], which didn't exist (the probe never made it), only
  for [R1.B69], made with it for the first file, while [R3...]'s shows
  [R3]: VMS seems to log one CREDIR per path it makes, naming the
  deepest level, which `RestoreResult.Directories` (every level) lets
  the command do; and a restore to
  an existing version needs the existing-file rules on top of
  `OutputExistsError`.

### 2026-10-20 — Subtask 9: BACKUP's restore

- **The command** (`internal/dclcmd/backuprestore.go`,
  `Context.restoreSaveSet`): the input save set's files that `/SELECT`
  and `/EXCLUDE` choose, in save set order, each made by
  `rms.Session.RestoreFile` at the name the output specifier gives it.
  The output specifier is parsed from its expanded name
  (`Session.ExpandName`): omitted or `[]` the default directory, `[*...]`
  the saved directory, `[dir]` everything into dir (a saved directory
  file too: `[B69]SUB1.DIR` went to `[R4]SUB1.DIR`), `[dir...]` dir with
  the carried part below it. A name or type given replaces the saved
  one (`[R9]NEW.TXT`); one left out or wild keeps it, and a restored file
  keeps its saved version. A host output names a directory, `"out/..."`
  being `[dir...]`'s form (govax's), its files named as COPY names them
  (`NAME.TYPE;1`) and written by COPY's rule. `/LIST` with a restore
  lists nothing yet.
- **Selection** (`backupselect.go`): `/SELECT` and `/EXCLUDE` are matched
  as text against the saved names, by the manual's rules (an omitted
  name, type, or version is `*`; `;` and `;-n` every version) and what
  VMS V7.1 did: a specification with no directory is `[*...]`, so
  `/SELECT=*.TXT` and `/EXCLUDE=*.DAT` reach every directory, and a
  device is ignored. Under `[dir...]` a file carries its whole directory
  without `/SELECT` (`[R1.B69]`) and with it the components the first
  matching selection's ellipses matched (`/SELECT=[B69.SUB1...]`:
  `[R3]`, `[R3.SUB2]`; `/SELECT=*.TXT`: `[R5]`, `[R5.SUB1]`, the `*`
  having matched B69). An ellipsis inside a directory (`[A...B]`) carries
  what it matched (unconfirmed).
- **`/BY_OWNER`** (output): DEFAULT, the default, is the process's UIC
  (`RTL.Process.UIC`, SYSTEM's without one); ORIGINAL, the default with
  no option, the saved owner; PARENT the directory's
  (`RestoreOptions.ParentOwner`); a UIC. A bad UIC is SS$_IVIDENT (VMS's
  message unconfirmed). The directories a restore makes take their
  parents' owners, whatever `/BY_OWNER` says (unconfirmed).
- **Existing files** (`rms.RestoreOptions.Existing`): with no qualifier,
  the version there is OPENOUT with `-RMS-E-FEX`; `/NEW_VERSION` makes
  the next version when the saved version or a higher one is there, the
  saved version otherwise (the manual's entry); `/OVERLAY` writes over
  the version there, keeping its file ID and blocks, extended if the
  saved file needs more, its own ACL dropped and its high-water mark the
  restore's; `/REPLACE` deletes it and makes the file anew (a new file
  ID, as `r7.lst` shows; VMS makes the new one before deleting the old,
  govax the other way round). A file some process has open can be
  neither overlaid nor replaced (SS$_ACCONFLICT, VMS's message
  unconfirmed). Without a qualifier, govax refuses only the same
  version; the manual says an equal or lower one is refused too
  (unconfirmed, not followed: RMS makes a lower version that isn't
  there).
- **Messages and status**, as VMS's log shows them: with `/LOG`, one
  CREDIR per file naming the deepest directory made for it (VMS made
  [R1] and [R1.B69] and named only [R1.B69]), a saved directory file
  made is a CREDIR of the directory (one that's there says nothing,
  unconfirmed), and CREATED for each file (a NOBACKUP file's too, not
  HEADCOPIED). VBNMISSING comes before its file's CREATED, with or
  without `/LOG`. A file that can't be made is OPENOUT with the reason
  on the next line, and the restore goes on. After the files, XORERRS
  for rebuilt blocks, then NOFILES when no file was made (even when one
  was selected and refused). `$STATUS` is the first of the most severe
  messages, error over warning (OPENOUT's %X10A38012 though NOFILES
  followed; VBNMISSING's %X10A3822A though files followed), else
  %X10000001.
- **Found and fixed on the way:**
  - ods2's `filespec.Parse` took `[]` as the master file directory. VMS's
    `[]` is the default directory (a restore to `[]` went to `[R8]`, the
    default), as it now is in ods2 (committed locally there, untagged,
    beside subtask 8's change: `docs/PHASE-22.md`'s step is the
    author's). `DIRECTORY []` listed `[000000]` before.
  - DIRECTORY took one specification, though the user's manual shows
    `file-spec[,...]`: a comma list is now each element in turn with one
    grand total, as VMS's log shows (`eachListed`; console.dcl's SPEC is a
    list). VMS's sticky defaults between elements aren't done.
  - DIRECTORY/OWNER's owner column is 31 wide, as VMS's
    `DIRECTORY/OWNER/PROTECTION` of [R10A...] shows; govax's was 20.
- **Tests:** `TestBackupRestoreOracle` (`internal/console`) replays the
  log from Part 4 on: every restore (Parts 4 to 6, the damaged save sets
  included), the CREATE/DIRECTORY and SET DEFAULT they need, the
  DIRECTORY reports of what they made (a made directory's time masked),
  and each `SHOW SYMBOL $STATUS`, 52 commands, all matching VMS's
  output line for line, on a volume with VMS's 3-block clusters
  (`mountBackupSets` now makes one); the `[B69]` tree the `[*...]`
  restores find is restored first. Then DIRECTORY/FULL of `[R7]` matches
  `r7.lst`, file IDs masked. `TestBackupRestoreHost` (both host forms,
  PI.EXE byte for byte, no output without `/LOG`),
  `TestBackupRestoreBadOwner`; `TestBackupSelection` and
  `TestBackupTargetLocation` (`internal/dclcmd`); `TestRestoreExisting`
  and `TestRestoreParentOwner` (`internal/rms`); ods2's
  `TestParseEmptyBracketsIsDefault`. `TestBackupMessages` checks a
  restore's OPENIN where it had NOTYET.

### 2026-10-20 — Subtask 10: BACKUP/COMPARE

- **`rms.Session.CompareFile`** (`internal/rms/compare.go`) holds one
  `backup.File` up against the file a `FileLocation` names. On a volume
  the file is found as DUMP finds one (`OpenDump`: one file, the saved
  version), and each block the save set holds is compared whole with the
  file's block of the same number, as the manual says BACKUP "processes
  files by blocks"; a block past the file's allocation differs, and
  blocks the save set lacks aren't compared. A header-only (NOBACKUP)
  file has nothing to compare and matches, as VMS found NOBACK.TXT; a
  saved directory file matches if it's there (its blocks hold file IDs a
  restore changes; unconfirmed, the probe compared none). The header's
  attributes aren't compared (unconfirmed: the probe changed none). On
  the host the file is compared with what a restore would write there by
  COPY's rule, in 512-byte pieces numbered as blocks (govax's). A missing
  file is a `*DCLFileError` (RMS$_FNF, or DNF).
- **The command** (`internal/dclcmd/backupcompare.go`,
  `Context.compareSaveSet`): the files `/SELECT` and `/EXCLUDE` choose,
  each compared with the file the output specifier names for it by a
  restore's rules (`restoreTarget`, `location`: `[*...]`, `[dir]`,
  `[dir...]`, a name or type given, the saved version), on the restore's
  progress type (`backupRestore`, its counter now `files`, its ending
  `finish`, shared). VERIFYERR for each block that differs, with or
  without `/LOG`, then COMPARED with `/LOG` (even after VERIFYERR, as
  VMS showed); OPENIN at severity E with RMS's reason for a missing file,
  and the compare goes on; NOFILES when none was compared; `$STATUS` by
  the restore's rule (VERIFYERR's %X10A38272, OPENIN's %X10A3800A). A
  file the save set has only part of is VBNMISSING before its COMPARED
  (unconfirmed for a compare). NOTYET is now only a save's.
- **Tests:** `TestBackupCompareOracle` (`internal/console`) replays Part
  5 of VMS's log (B69TXT against the [B69] tree, TEXT against a restored
  copy, the copy with a line appended, the copy deleted, and each
  `$STATUS`), matching VMS line for line; `TestBackupCompareSelections`
  (`/SELECT`, a file shorter than the saved one, no COMPARED without
  `/LOG`); `TestBackupCompareHost` (a clean host restore, a changed
  byte's piece, a missing host file); `TestBackupMessages` checks a
  compare's OPENIN of a missing save set where it had NOTYET.

### 2026-10-20 — Subtask 11: saving

- **The layout** (`internal/backup/writer.go`, `Writer`), read off VMS's
  save sets with a scratch dump and confirmed by rewriting all 16 of
  them (`pi.bck` and the probe's 15) byte for byte from what `Read` makes
  of them (`TestRewriteVAXSaveSets`):
  - A record's body is padded with zeros to a multiple of 16 (the
    summary's too: no terminator of its own, just the padding); a file
    record's attributes are in the order Part A lists, DIR_STATUS (one
    byte, %X0F) for a directory file and ACLSEGMENT for an ACL last.
  - Data records hold whole blocks and never cross a block's end; a
    record that doesn't fit starts the next block, where a file's data
    goes on in a new record (an attribute record may end a block with
    its data in the next: B69K8's MULTI.TXT;3). govax always leaves room
    for the null record's header, which no save set contradicts or
    settles (unconfirmed).
  - A block's header names the file whose attribute record was written
    last before the block began, its FID and directory ID with RVN 1
    (the file record's FID has RVN 1 too, its BACKLINK 0): LIST.BCK's
    block 2, which starts with A.TXT's record, names A.TXT.
  - **An input list's elements each start a new block** (LIST.BCK:
    TEXT.TXT in block 1, A.TXT at the start of block 2), `EndBlock`.
  - The buffer count is 256K over the block size (8 for 32256, 32 for
    8192; the rule between them govax's).
- **The save set's messages:** VMS's log has every NOBACKUP notice
  before the first COPIED, the whole tree fitting in its 8 buffers, so
  `Writer.Saved` reports a file only when the block holding its last
  record leaves the buffers (the oldest written when all are full; how
  VMS's buffers cycle is unconfirmed).
- **RMS's side** (`internal/rms/save.go`): `SaveSearch` walks an input
  specifier as VMS did: a directory's entries in order, every version
  when none is given (the manual's 7.3.3), and in a `[dir...]` search
  each subdirectory where its .DIR entry is, saved first as a `Walked`
  file whatever the selection says (B69NONE, B69BYO), then its files.
  A search without "..." that finds nothing is RMS$_FNF, a missing
  directory RMS$_DNF. Each file's attributes come from its header (the
  FAT copied raw), its directory entry, and its directory: VERLIMIT and
  DIR_VERLIM are the entry's version limit, which VMS wrote alike for
  every file (VERLIM.TXT 3, [B69.SUB1]'s files 5), or the one govax
  keeps in a file's FAT when it has one; the FAT's version word (a
  directory's default limit) is 0 for a file, as VMS's are. A relative
  or indexed file is saved whole: VMS saved all 9 blocks of
  INDEXED.IDX, 8/9 by DIRECTORY (whether VMS saves to its end-of-file
  block or its allocation, the same there, is unconfirmed). `Read`
  accesses a file denying writers: a file open for writing is OPENIN
  with -SYSTEM-W-ACCONFLICT, or with /IGNORE=INTERLOCK saved after a
  W-ACCONFLICT notice (the manual's text; unconfirmed). `CreateSaveSet`
  makes the save set's file before anything is read (OPENOUT first, as
  VMS opens its output first), and `Finish` gives it its blocks: fixed
  records of the block size, no carriage control, extend 0, owned by the
  process or the output's /BY_OWNER UIC (sets.lst).
- **The command** (`internal/dclcmd/backupsave.go`): the summary records
  the command as DCL hands it over (B69COM's /COMMENT lost its quotes;
  the rest uppercased and blanks squeezed, unconfirmed), the process's
  user and UIC, the time, OpenVMS VAX and govax's version, the SID
  (`corevms.System.SystemID`), BACKUP V6.2, and the output's full device
  name ("_GOVAX$DUA1:", "_GOVAX$HOST:" for a host save set); no node
  name, as VMS without DECnet wrote none. /BLOCK_SIZE is 2048 to 65535,
  rounded up to 512 (the manual's 10000 is 10240), /GROUP_SIZE 0 to 100
  (BLOCKSIZE and GROUPSIZE, govax's messages). /EXCLUDE never excludes a
  directory file (the manual), /BY_OWNER takes `*` for the group or
  member, and /SINCE and /BEFORE test the date /MODIFIED, /CREATED,
  /EXPIRED, or /BACKUP names, read by DCL's time parser
  (`Interpreter.ParseTime`, given to the commands as
  `Context.ParseTime`); /BACKUP's /BEFORE takes a file never backed up.
  An input list element with no device or directory takes the one
  before it (DCL's sticky defaults; unconfirmed for BACKUP). An element
  that finds nothing is SEARCHFAIL with RMS's reason (VMS's message as
  it's known, unconfirmed), and a save whose searches went well but saved
  nothing ends NOFILES, naming the first input (unconfirmed). NOTYET is
  gone.
- **Tests:** `TestBackupSaveOracle` (`internal/console`) restores VMS's
  [B69] tree from B69.BCK and B69IGN.BCK, replays Part 2's 15 saves
  (TODAY written as 20-OCT-2026, the test's clock having a day of its
  own), matches VMS's messages and `$STATUS` line for line, and holds
  each save set govax writes up against VMS's byte for byte, once what
  is the run's own is taken from govax's (the summary's date, CPU ID,
  device, and system version; the file IDs, backlinks, and high-water
  marks a restore makes anew; a directory's dates, revision, and
  contents). `TestBackupSaveMessages` (SEARCHFAIL, NOFILES, OPENOUT, a
  host save set with /CRC restored and compared), `TestRecordedCommand`,
  `TestStickyDefaults`, `TestSaveFilter`, `TestSaveSetName`
  (`internal/dclcmd`), `TestWriterSaved`, `TestWriterBlockSize`.
- **For subtask 12:** a restore sets a file's high-water mark to the
  block after its data, where VMS's tree had other values (EXPIRE.TXT,
  allocated 3 with 1 used, had 4: SET FILE seems to move it); the save
  oracle masks it. govax keeps a file's version limit in its FAT where
  VMS keeps it in the directory entry; the save writes VMS's form, and
  the restore still writes the FAT's.

### 2026-10-20 — Subtask 12: the oracle, and round 2 prepared

- **`TestBackup69Oracle`** (`internal/console/backup69oracle_test.go`)
  runs BACKUP69.COM on govax, on an RD54 with VMS's 3-block clusters as
  DUA1 holding SSCOPY.MAR, and holds its log and reports to VMS's. govax
  has no CONVERT, SET FILE, or SET SECURITY, so Part 1's tree is VMS's
  own, restored from B69.BCK and B69IGN.BCK (`/BY_OWNER=ORIGINAL`, every
  saved attribute), and govax's copy of the procedure has CREATE/DIRECTORY
  [SETS] in place of the lines that make it; the logs are compared without
  them. TODAY is written 20-OCT-2026 in the procedure and in VMS's log, as
  `TestBackupSaveOracle` writes it. Everything else runs as VMS ran it:
  the 15 saves, every listing, the restores, the compares, the errors,
  and SSCOPY (MACRO, LINK, RUN) making the truncated and damaged save
  sets from govax's own, which govax then lists and restores.
- **The masks** (`backup69Mask`) take out only what is the run's own:
  the times of the files the run made (a directory, save set, report, or
  log, on its entry's lines in DIRECTORY and BACKUP/LIST reports), a
  summary's date, the system that wrote a save set (version, CPU ID,
  device), and file IDs. The restored files' dates are the saved files',
  so they're compared.
- **What matches:** the log, 2900 lines after Part 1's are taken out, but
  for 5 listed differences (`backup69Differences`); TREE.LST, SETS.LST,
  R1.LST, R7.LST, TEXT.LIS, and B69FULL.LIS line for line but for 6; and
  Part 2's 15 save sets byte for byte against VMS's in `[VAX]`, once the
  run's own values are taken from govax's (`compareSaveSet`, now given
  the two files' names, shared with `TestBackupSaveOracle`). The listed
  differences: F$GETSYI's line; DIRECTORY/FULL of INDEXED.IDX, where VMS
  shows `Indexed, Prolog: 3, Using 1 key` and govax `Indexed` (govax
  doesn't read an indexed file's prolog; no definition of its layout is
  in govax, so it's left for a later phase rather than read off one
  file); and the revision count of the tree's [B69.SUB1] and
  [B69.SUB1.SUB2], 0 as VMS's CREATE/DIRECTORY made them, 1 as the
  test's restore makes them, as VMS's BACKUP does.
- **Round 2** (`testdata/backup69/round2`, its README), for the author:
  `exchange.cmd` builds `testdata/disks/backup69-round2.dsk` with round
  1's tree restored by govax and Part 2's save sets made by govax (the
  same commands, `/SINCE` and `/BEFORE` naming 20-OCT-2026), plus
  HOST.BCK, a save set govax wrote to the host, copied on as fixed
  512-byte records. `run2.com` has VMS look at the tree's headers
  (TREE2.LST), list every save set (and `/FULL`), restore each with
  `/LOG` (B69CRC.BCK and HOST.BCK with `/CRC` on the input), compare
  B69.BCK with the tree and with VMS's restore of it, damage B69.BCK and
  B69CRC.BCK in block 2's data and B69.BCK in block 2's header with
  SSCOPY and restore them (govax's XOR blocks), and ANALYZE/DISK_STRUCTURE
  the volume. `copyout.cmd` brings the results back into `vax/round2/`,
  govax's save sets into `vax/round2/govax/`. govax ran RUN2.COM on a
  copy of the volume first: every command but ANALYZE/DISK_STRUCTURE,
  which govax lacks, ran, with XORERRS for the CRC and header damage and
  nothing for the data damage without a CRC, as round 1's VMS run showed.
  `TestBackup69Round2` follows VMS's run.

### 2026-10-20 — Subtask 12: the second round

- **VMS's run** (`testdata/backup69/vax/round2`; `round2/README.md` has
  the findings): VMS V7.1 listed, restored, and compared every save set
  govax made, `/CRC` on the input included, and recovered SSCOPY's damage
  to them through govax's XOR blocks as it had its own. ANALYZE/
  DISK_STRUCTURE found the volume sound, but for BADHIGHWATER on the
  twelve header-only NOBACK.TXT files its own restores made (unconfirmed
  whether it does that for its own save sets).
- **Fixed from its answers:**
  - A listing leaves out `CPU ID register:` when the CPU ID is 0, as VMS
    did for govax's (`writeSaveSetSummary`). The round 1 oracle takes the
    line out of both runs.
  - A compare passes over a saved directory file without a word, as VMS
    did (`backupRestore.compare`); subtask 10 had it match.
  - A restored directory file counts as a file restored, so a save set of
    directory files alone (B69NONE.BCK) ends without NOFILES, as VMS's
    did (an existing one counts too, unconfirmed).
  - VMS reads a file's version limit from its directory record (its
    DIRECTORY/FULL of a VERLIM.TXT govax restored said "No version
    limit"). The restore now sets the name's record to the saved limit
    too (`volumeRestore.setEntryLimit`, over ods2's new
    `Directory.SetNameVersionLimit`; `TestRestoreEntryVersionLimit`),
    keeping the header's, which govax's DIRECTORY and ods2's version
    enforcement read. The ods2 change is in its v0.1.24, which
    `go.mod` pins.
- **Left as it is:** VMS refuses a save set stored as 512-byte records
  (NOTSAVESET, %X10A38104), which govax reads (Decision 3).
- **`TestBackup69Round2`** (`internal/console/backup69round2_test.go`)
  makes the exchange volume again by running `exchange.cmd`'s lines
  (paths put in the test's directories), holds Part 2's 15 save sets to
  the ones VMS read, byte for byte but for the run's own values, runs
  RUN2.COM, and holds the log (1971 lines but for F$GETSYI's), TREE2.LST,
  SETS2.LST, VB69.LST, and B69FULL2.LIS to VMS's, masked as round 1's.
  HOST.BCK's listing and restore, the DIRECTORY that follows (compared up
  to [VHOST...]), and ANALYZE/DISK_STRUCTURE are cut out and checked on
  their own. TREE2.LST's VERLIM.TXT lines are a listed difference: VMS
  read the tree before the restore fix. SSCOPY's copies, VMS's and
  govax's, each differ from their source in the one byte changed.

### 2026-10-20 — Subtask 13: the close-out

- **`docs/MANUAL.md`**: a BACKUP entry in section 6.1 (its four forms,
  where qualifiers stand, saving to a volume and to the host, a listing,
  restoring by the output-directory rules, an existing file and
  `/NEW_VERSION`, a restore to a host directory, a compare that finds a
  missing file and a changed block, and damaged save sets). Every
  example was run on govax, on a fresh volume under a scratch profile.
  Section 1 lists BACKUP among what govax does, section 5's "Moving
  files" points to it for whole trees, 13.1 names `internal/backup`, and
  Appendix C has BACKUP's limits and the 512-byte-record difference.
- **`HELP BACKUP`** (subtask 6's, kept up through subtask 12) was read
  against the command as it stands and needed no change.
- **`CLAUDE.md`**: the phase in the overview, `testdata/backup69` among
  the reference material, BACKUP among `internal/dclcmd`'s commands.
  **`docs/PLAN.md`**: the phase's row marked done.
  **`docs/DEVIATIONS.md`**: "[Phase 69] BACKUP save sets", what the probe
  changed, where govax follows VMS 7.1 over the manual (`/NOCRC`), where
  it differs (512-byte records, an indexed file's prolog, "_To:"), and
  the unconfirmed choices.
