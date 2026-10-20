# Phase 69 — BACKUP save sets

**Status:** planned 2026-10-20. Subtask 1, the survey, is done: Part A
holds what the first save set (`testdata/backup69/vax/pi.bck`) shows of
the format. The plan waits for the author's review. Part B's questions
are open.

## Goal

govax's DCL gets a `BACKUP` command that reads VMS save sets, with VMS
BACKUP's syntax:

1. **List** a save set: `BACKUP/LIST saveset/SAVE_SET`, with VMS's
   layouts for the summary and file lines, `/BRIEF` (the default) and
   `/FULL`, and `/LIST=file`.
2. **Restore** files from a save set: `BACKUP saveset/SAVE_SET
   output-spec`, onto a mounted volume (and, by question 3's answer, to
   the host). Each file keeps its saved attributes: record format, dates,
   protection, owner, characteristics, allocation, and end of file. The
   qualifiers people use are in scope: `/SELECT`, `/EXCLUDE`, `/LOG`,
   `/NEW_VERSION`, `/OVERLAY`, `/REPLACE`, `/BY_OWNER`, and the
   output-directory rules (`[*...]`, `[dir...]`, `[dir]`, omitted).

A save set may be a file on a mounted volume, which is how VMS keeps
one: fixed-length records the size of the save set's block. It may also
be a host file holding the same bytes, as a binary FTP or a copy out of
a container leaves it.

Saving files into a new save set is a candidate for later in the phase
(question 1). It would let real VMS read govax's save sets in a second
round, as Phase 68's second round ran govax's images. Image and
physical backups, tapes, journals, incremental backups, and multivolume
save sets are out of scope (question 5).

As in other phases, the target is real VMS's output. BACKUP/LIST's
listings and `/LOG`'s messages should match VMS's line for line, and a
restored file should match its original block for block and attribute
for attribute.

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

Not used: VMS's sources and definition files (the clean room). Several
public, independently reverse-engineered save set readers exist (such as
the `vmsbackup` program). Whether the phase may consult them is
question 6. Until it's answered, everything below comes from the
fixture alone, and every name in the tables is this survey's own.

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
  revision count, characteristics, highwater, allocation) is to be
  checked in subtask 6.
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
  must come from the manual's examples, the probe, or VMS's message
  definitions through `internal/vmsdef/gen` (question 4).

### The save set as the fixture shows it

`pi.bck` is 126 blocks: two save set blocks of 32256 bytes, the
`Block size` the summary names. The volume file's records are fixed
length, the same 32256 bytes, so each record is one save set block.
Block 1 holds the summary and the three files. Block 2 is block 1's
redundancy (XOR) block: the group size is 10, the group has one data
block, and the XOR of one block is that block. Its records repeat block
1's byte for byte, and only its header differs (below).

Integers are little-endian. "Confirmed" means the fixture or VMS's
listing settles a field. "Guess" means it fits this one save set and
needs the probe.

#### The block header (256 bytes)

Each block starts with a 256-byte header. Records follow it, up to the
block's end.

| Offset | Size | Block 1 | Block 2 | Meaning |
| --- | --- | --- | --- | --- |
| 0 | word | 256 | 256 | The header's size (confirmed: the records start there) |
| 2 | word | %X0400 | %X0400 | The operating system: the same value as the summary's attribute 7 (guess: 1024 is VAX/VMS) |
| 4 | word | 1 | 1 | Unknown (guess: a subsystem or format code) |
| 6 | word | 1 | 2 | Unknown. 1 in the data block, 2 in the XOR block (guess: the block's kind, or a second count) |
| 8 | long | 1 | 2 | The block's number in the save set (guess) |
| 12–31 | 20 | 0 | 0 | Unused here |
| 32 | word | %X0101 | %X0101 | A structure level (guess: 1.1, as the records' bodies start) |
| 34 | word | 1 | 1 | The volume number (guess: for multivolume save sets) |
| 36 | long | 0 | 0 | The data CRC (guess, from the author's note; zero here, see below) |
| 40 | long | 32256 | 32256 | The block size (confirmed) |
| 44 | long | 1 | 1 | Flags (guess: bit 0 says the block has no CRC) |
| 48 | 32 | `PI.BCK` | `PI.BCK` | The save set's name, a counted string (confirmed) |
| 80–253 | | 0 | 0 | Unused here (room for a file ID, a directory ID, a name, record attributes, a size, all zero in a disk save set? unknown) |
| 254 | word | %XBE02 | %X1B5D | The header's checksum (confirmed, below) |

**The header checksum** is the CRC-16 of the whole 256-byte header,
taking its last word as zero. The polynomial is %XA001 (CRC-16,
reflected, the VAX CRC instruction's "CRC-16" table), the initial value
0, and nothing is XORed at the end. It matches both blocks. They differ
in two fields, so a chance match is very unlikely. Found by searching
every 16-bit polynomial against the two headers' difference, then every
initial value.

**The data CRC:** none was stored. No AUTODIN-II variant over the block
(initial 0 or all ones, complemented or not, over the whole block, its
records, or the block with the CRC field zeroed) comes out zero. No
other header field is nonzero. Yet the command didn't say `/NOCRC`, and
the author's note says `/CRC` is the default. Either BACKUP V6.2 didn't
compute one for this disk save set, which would make flag bit 0 the
"no CRC" bit, or it's stored somewhere the fixture can't show. The probe
writes the same files with `/CRC` and `/NOCRC` to settle it.

#### Records

A record starts with a 16-byte record header:

| Offset | Size | Meaning |
| --- | --- | --- |
| 0 | word | The size of the record's body, not counting this header (confirmed) |
| 2 | word | The record's type (confirmed for the four below) |
| 4 | long | Flags (zero in every record here) |
| 8 | long | An address: the first virtual block of a data record, 0 otherwise (confirmed) |
| 12 | long | Zero in every record here |

Types seen:

| Type | Meaning | In the fixture |
| --- | --- | --- |
| 0 | Null: fills the rest of the block. Its size runs exactly to the block's end | One per block, after the last file's data |
| 1 | The save set summary | First in each block |
| 3 | A file's attributes | Before each file's data |
| 4 | A file's data (virtual blocks) | After each file's attributes: the file's used blocks, starting at its address (VBN 1). Byte for byte the file's blocks |

The summary repeats in every block. Each block can be read alone, so a
damaged block costs only its own files. A file's data records hold its
used blocks only, up to its end of file (PI.OBJ: 5 of 6 allocated).
Here every file fits one record in one block. How a file larger than a
block (more than 63 blocks of data) or a record that would cross a
block's end is split is for the probe.

#### Attribute lists

The summary's and a file record's bodies start with a word, %X0101
(guess: a structure level), followed by attributes. Each attribute is a
word giving the value's size, a word giving its type, and the value.
The list ends at a zero size and type (the summary's) or at the
record's end (a file's).

**The summary's attributes**, in the order the listing shows them:

| Type | Size | Value here | Listing line |
| --- | --- | --- | --- |
| 1 | 6 | `PI.BCK` | Save set |
| 2 | 27 | `BACKUP [.PI]*.* PI.BCK/SAVE` | Command |
| 3 | — | absent | (guess: `/COMMENT`'s text, a `Comment:` line) |
| 4 | 12 | `SYSTEM` blank-padded to 12 | Written by |
| 5 | 4 | [1,4] as member word, group word | UIC |
| 6 | 8 | 20-OCT-2026 10:45:22.88 | Date |
| 7 | 2 | %X0400 | Operating system (with attribute 8: `OpenVMS VAX version V7.1`) |
| 8 | 4 | `V7.1` | the version in the Operating system line |
| 9 | — | absent | (guess: a node name, for a save set written over DECnet) |
| 10 | 4 | %X040704D2 | CPU ID register |
| 11 | 13 | `_SIMVAX$DUA1:` | Written on |
| 12 | 4 | `V6.2` | BACKUP version |
| 13 | 4 | 32256 | Block size |
| 14 | 2 | 10 | Group size |
| 15 | 2 | 8 | Buffer count |

**A file's attributes**, all present for each of the three files and in
this order:

| Type | Size | PI.EXE | PI.MAR | PI.OBJ | Meaning |
| --- | --- | --- | --- | --- | --- |
| %X2A | var. | `[XFER.PI]PI.EXE;1` | ... | ... | The file's name as listed (confirmed) |
| %X2B | 2 | %X0201 | %X0201 | %X0201 | Structure level: ODS-2 (guess) |
| %X2C | 6 | (19,1,0) | (18,1,0) | (16,2,0) | The file ID (confirmed) |
| %X2D | 6 | (17,1,0) | (17,1,0) | (17,1,0) | The directory's file ID: the back link (confirmed, [XFER.PI]) |
| %X2E | 4 | 6 | 24 | 6 | The allocation in blocks (confirmed) |
| %X2F | 4 | [1,4] | [1,4] | [1,4] | The owner (confirmed) |
| %X30 | 2 | %XFA00 | %XFA00 | %XFA00 | The protection: (S:RWED,O:RWED,G:RE,W) (confirmed) |
| %X31 | 2 | 0 | 0 | 0 | Unknown (guess: record protection) |
| %X32 | 1 | 0 | 0 | 0 | Unknown |
| %X33 | 4 | %X20 | 0 | 0 | The file characteristics: %X20 contiguous best try, which PI.EXE has (confirmed) |
| %X34 | 32 | | | | The record attributes, the file header's 32-byte FAT area: format, attributes, record size, highest and end-of-file blocks, first free byte, maximum record, default extend (confirmed: PI.OBJ's extend 20, PI.MAR's longest record 79) |
| %X35 | 2 | 1 | 2 | 1 | The revision count (confirmed) |
| %X36 | 8 | | | | The creation time (confirmed) |
| %X37 | 8 | | | | The revision time (confirmed) |
| %X38 | 8 | 0 | 0 | 0 | The expiration time (guess) |
| %X39 | 8 | 0 | 0 | 0 | The backup time (guess) |
| %X47 | 4 | [1,4] | [1,4] | [1,4] | A UIC (guess: the directory's owner, or the file's creator) |
| %X48 | 2 | %XBA88 | %XBA88 | %XBA88 | Unknown, the same for all three (guess: a protection, perhaps the directory's or the volume's default) |
| %X4A | 2 | 32767 | 32767 | 32767 | A version limit (guess: the directory's) |
| %X4B | 2 | 32767 | 32767 | 32767 | A version limit (guess: the file's; DIRECTORY/FULL says "No version limit") |
| %X4F | 4 | 7 | 25 | 7 | The allocation plus one (guess: the highwater mark) |
| %X50 | 1 | 0 | 0 | 0 | Unknown (guess: journaling flags) |
| %X57 | 1 | 0 | 0 | 0 | Unknown |

The type numbers have gaps: %X3A–%X46, %X49, %X4C–%X4E, %X51–%X56.
Those are probably attributes these files lack: an ACL, journaling, a
placement, an RMS attribute area, a directory file's own data. The
probe's files with ACLs, version limits, expiration dates, and other
record formats are meant to show them.

The listing's size column is the used blocks, which the FAT's
end-of-file block and first free byte give: 6, 24, 5. Its date is the
creation time. "Total of 3 files, 35 blocks" is the used blocks' sum.

### What the fixture doesn't settle (for the probe)

1. A file larger than a block, and data split across blocks or groups.
2. Redundancy groups of more than one block (group size 10 with 11 or
   more blocks, `/GROUP_SIZE=3`, `/GROUP_SIZE=0`), and the header words at
   offsets 4 to 8 across them. What VMS says when it recovers a damaged
   block from its group, and when it can't.
3. The data CRC (`/CRC`, `/NOCRC`) and the flags.
4. Other block sizes (`/BLOCK_SIZE=8192`, the smallest allowed), and
   whether the volume file's record size always equals the block size.
5. Directory files saved by a `[dir...]` save, and how a restore makes
   directories from them.
6. Files with an ACL, an expiration date, a version limit, several
   versions, no blocks at all, and other record formats (fixed, VFC,
   stream, indexed by FDL): the attribute types missing above.
7. The summary's absent attributes (`/COMMENT`, and others as found).
8. VMS's `/LIST/FULL` layout, `/LIST=file`, and the listing of a damaged
   save set.
9. Restores: every output-directory form, `/SELECT`, `/EXCLUDE`, the
   version qualifiers, `/BY_OWNER`, `/LOG`'s messages, an existing file
   without a version qualifier, a missing `/SAVE_SET`, a missing or
   non-save-set input, and the restored files' headers (DIRECTORY/FULL).
10. Whether VMS's DIRECTORY/FULL shows "Contiguous best try" for PI.EXE.
    govax's doesn't (side finding 2).

### Side findings (2026-10-20)

1. **COPY's destination ignores govax's host-name rule 3.** `COPY
   DUA1:[XFER]PI.BCK "./pi.bck"` wrote the volume file
   `DUA0:[WORK]./pi.bck;1` (a name with a `/` in it). The user's manual,
   section 5, says a name with `/` is a host file. Only `/HOST` reached
   the host. `dclcmd.Context.Copy` hands both names to
   `rms.Session.Copy` without `Session.Locate`'s rules. Not fixed here:
   a small change to COPY, to be made in this phase or on its own as the
   author prefers. Host COPY also turns records into lines (the save set
   came out as 64514 bytes), so it can't move a save set to the host
   whole. The survey read the volume through ods2 directly.
2. **DIRECTORY/FULL may be missing "Contiguous best try"** on the File
   attributes line for a file with FCH$V_CONTIGB (PI.EXE). Unconfirmed
   until the probe's DIRECTORY/FULL of PI.EXE.

## Part B — decisions and questions

### Proposed decisions

1. **The probe comes first, then the reader.** As in Phases 53 to 68,
   real VMS's output settles what the fixture can't. Subtask 2 writes
   the probe (sources, a command procedure, and the files it saves), and
   the author runs it on the simh VAX. The reader (subtask 3) can start
   on `pi.bck` while it waits.
2. **A leaf package, `internal/backup`,** holds the format: reading
   blocks (the header and its checksum, the data CRC when present, XOR
   recovery), records, the summary, and file attributes, decoded into Go
   types (`Summary`, `File` with an `rms`-independent attribute struct)
   from any `io.ReaderAt`. It knows nothing of volumes or DCL, like
   `internal/obj` and `internal/lbr`. A writer joins it if question 1's
   answer is yes.
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
5. **Messages are VMS's**, as `%BACKUP-` lines, from the probe's log
   (question 4 for their condition values).
6. **Fidelity:** listings and `/LOG` messages line for line, with times
   masked only where they're the run's own. Restored files block for
   block, with DIRECTORY/FULL matching VMS's restored copies. govax
   keeps what VMS keeps (the dates, revision count, owner, protection),
   and the probe shows what VMS resets.

### Questions for the author

1. **Saving too?** Should the phase also write save sets
   (`BACKUP files saveset/SAVE_SET`, with `/BLOCK_SIZE`, `/GROUP_SIZE`,
   `/CRC`, `/COMMENT`, and the input file-selection qualifiers `/SINCE`,
   `/BEFORE`, `/BY_OWNER`, `/EXCLUDE`)? Proposed: yes, after list and
   restore work (subtasks 9 and 10), so VMS can read govax's save sets
   in a second round. That's the strongest check of the format.
2. **Host save sets.** Proposed: a save set may be a host file
   (`saveset.bck/HOST/SAVE_SET`, or a name govax's rules make a host
   file), read as raw bytes in blocks. SIMH tape images (`.tap`, how many
   VMS kits are distributed) would make a later phase.
3. **Restoring to the host.** When the output names a host directory,
   what should a host file get? Proposed: COPY's rule for a volume file
   going to the host (records become lines for text files, raw bytes
   otherwise), so a restored `PI.MAR` is readable on the host. The other
   choice is raw blocks always.
4. **BACKUP's message texts and codes.** `vmsdef.Messages` has none.
   Should the author add the BACKUP facility's messages with
   `internal/vmsdef/gen -msg` from the licensed definitions (which this
   phase's tools can't touch, by the clean-room hook), or should the
   texts come from the probe's log, with the codes unconfirmed?
5. **Out of scope, as proposed:** image, physical, and incremental
   operations, `/COMPARE`, `/RECORD`, `/JOURNAL`, `/VERIFY`, `/DELETE`,
   tapes, multivolume save sets, `/CONFIRM`, ODS-5 names, `/INTERCHANGE`.
   Should `/COMPARE saveset/SAVE_SET files` be in? It reuses the reader
   and is small.
6. **Public reverse-engineered descriptions.** Readers such as
   `vmsbackup` (public C programs, not DIGITAL's) describe the block and
   record layouts and name the fields. May the phase consult them, like
   a manual? Or should the format come from real VMS output only, with
   the fixture and probe as the whole source? Either works. They would
   mainly speed up naming the fields Part A marks "guess", and the probe
   would still confirm each one.
7. **Side finding 1 (COPY to a `/`-named host file):** fix it in this
   phase (a small extra subtask), or separately?

### The probe's cases (subtask 2, proposed; `testdata/backup69`)

The author runs a command procedure on the simh VAX, on an exchange
volume that holds the probe's files. Its log, its listings, the save
sets, and the restored trees come back on the volume.

- **Files to save**, made by the procedure in a tree `[B69...]`
  with two levels of subdirectories:
  - small text files of each record format (variable with CR, VFC,
    stream LF, fixed 80), an indexed file (CREATE/FDL plus a few
    records), and an empty file;
  - a file of more than 63 blocks and one of more than 630 (so a file
    spans blocks and groups);
  - a file with an ACL (SET SECURITY/ACL), one with an expiration date,
    one with a version limit, several versions of one file, a file owned
    by another UIC, and a file marked NOBACKUP (SET FILE/NOBACKUP).
- **Save sets**:
  - `[B69...]` with the defaults;
  - `/BLOCK_SIZE=8192`;
  - `/GROUP_SIZE=0` and `/GROUP_SIZE=3`;
  - `/CRC` and `/NOCRC` (the same files);
  - `/COMMENT="..."`;
  - one input with `/EXCLUDE`;
  - `/BY_OWNER` on the save set.
- **Listings:** BACKUP/LIST, /LIST/FULL, /LIST/BRIEF, and /LIST=file of
  each save set. DIRECTORY/FULL of each save set file (its record
  attributes), and of `[XFER.PI]PI.EXE` (side finding 2).
- **Restores** into fresh directories, each followed by DIRECTORY/FULL
  of what it made:
  - to `[*...]`, `[R1...]`, `[R2]`, and the default directory;
  - with `/SELECT` (a file, a wildcard, a directory tree) and
    `/EXCLUDE`;
  - over existing files: none of `/NEW_VERSION`, `/OVERLAY`, `/REPLACE`,
    then each of them;
  - `/BY_OWNER=ORIGINAL`, `DEFAULT`, `PARENT`, and a UIC;
  - all with `/LOG`.
- **Errors:** a save set without `/SAVE_SET`, a missing save set, a
  file that isn't a save set, a save set truncated by a block, and a
  copy with one block's bytes changed: in a group with its XOR block
  (VMS should recover it) and with `/GROUP_SIZE=0` (VMS can't). The
  damaged copies are made on govax's side and put on the exchange
  volume, so the VAX only reads them.

## Part C — subtasks

1. **The survey** — done (2026-10-20). Part A: the fixture's format, the
   sources, what earlier phases provide, the side findings, the plan.
2. **The VMS probe** (`testdata/backup69`): the files to save, the
   command procedure (Part B's cases), the damaged copies, and the
   exchange volume; then the author's run. Part A's tables are updated
   from its answers ("guess" becomes confirmed or corrected), and its
   README lists what each output shows.
3. **`internal/backup`, reading:** blocks (header, checksum, data CRC,
   flags), records, the summary, file attributes into Go types, a file's
   data in VBN order, and XOR recovery of a damaged block from its group.
   Tested on `pi.bck` (the files match `pi.exe`, `pi.mar`, `pi.obj`),
   then on the probe's save sets, including the damaged ones.
4. **Opening a save set:** a volume file through RMS (block I/O, record
   attributes checked) or a host file, as an `io.ReaderAt` of blocks.
   The `/SAVE_SET` rule and its errors.
5. **The command:** `BACKUP` in `console.dcl` (input and output
   specifiers, command qualifiers, and position-dependent qualifiers),
   `HELP BACKUP` in `console.help`, and BACKUP's messages in `vmserrors`
   (question 4).
6. **BACKUP/LIST:** the summary block and file lines in VMS's layout,
   `/BRIEF`, `/FULL` (sharing DIRECTORY/FULL's writer where the layouts
   agree), `/LIST=file`, the totals, and "End of save set". Checked
   against the probe's listings.
7. **Restoring a file** (`internal/rms`): a new file on a volume with
   the saved record attributes, characteristics, owner, protection,
   dates, revision count, version limit, allocation, and data up to its
   end of file. Directories made as needed. Host output by question 3.
   ods2 gains what's missing (a release tagged and pinned as
   `docs/PHASE-22.md` describes).
8. **BACKUP's restore:** the output-directory rules (7.3.3), `/SELECT`,
   `/EXCLUDE`, the existing-file rules and `/NEW_VERSION`, `/OVERLAY`,
   `/REPLACE`, `/BY_OWNER`, and `/LOG`. Checked against the probe's
   restores, file for file and header for header.
9. **(If question 1 is yes) Writing save sets:** `internal/backup`'s
   writer (summary, file records, data records split as VMS splits them,
   null fill, XOR blocks, header checksum, data CRC) and the save
   command with its qualifiers. govax's save sets of the probe's files
   should match VMS's, apart from times, IDs, and the CPU and node
   fields.
10. **The oracle:** `TestBackup69Oracle` runs the probe's procedure on
    govax and matches VMS's log and listings, the restored files, and
    their headers, with a listed set of differences. With subtask 9,
    a second VAX round in which VMS lists and restores govax's save sets,
    replayed by `TestBackup69Round2`.
11. **The close-out:** `docs/MANUAL.md` (BACKUP, with examples run on
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
