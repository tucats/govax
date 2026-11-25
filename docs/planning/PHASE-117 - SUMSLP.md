# Phase 117 — SUMSLP (EDIT/SUM)

**Status:** planned 2026-11-25 (subtask 1, the survey and this plan),
awaiting the author's review. This document is the development plan; as
the work proceeds it becomes the implementation record (the progress
log, and at the end "Where things stand").

## Goal

govax gets VMS's SUMSLP batch editor as `EDIT/SUM`: one input file
updated by one or more files of SLP edit commands, the updates merged
into one list and applied in a single pass, the result written as a new
file, with SUMSLP's qualifiers, audit trail, listing, messages, and exit
statuses, so that the file govax writes is the file VMS 7.1 writes for
the same input and updates.

`EDIT/SLP` is accepted as a synonym, the name older VMS versions used;
if VMS 7.1 refuses it, it is govax's own and DEVIATIONS.md says so.

Phase 116's `DIFFERENCES/SLP` writes the scripts SUMSLP reads. The two
together close a loop the tests can use: for every pair of files,
`DIFFERENCES/SLP A B` then `EDIT/SUM A /UPDATE=A.DIF` must give B again.

## Part A — the survey (2026-11-25)

### A.1 What VMS has

The *OpenVMS DCL Dictionary* (7.3-1), DCLI–218, says only this:

    EDIT/SUM input-file

    Invokes the SUMSLP utility, a batch-oriented editor, to update a
    single input file with multiple files of edit commands. For more
    information about the SUMSLP utility, refer to the OpenVMS SUMSLP
    Utility Manual (available on the Documentation CD-ROM) or online
    help.

MACRO's `/UPDATE` entry (DCLI, MACRO) adds what SUMSLP does for the
assembler:

- `/UPDATE[=(update-filespec[,...])]` is a positional qualifier on an
  input file. The update file defaults to the source's name with type
  `.UPD`; wildcards are refused.
- A missing update file is an informational message, and the assembly
  goes on.
- Several update files are *merged into a single list of updates before
  being applied*.
- The source and update files aren't changed; with `/LIST`, the listing
  carries an audit trail of the changes.

Phase 116's probe showed the script DIFFERENCES writes (A.4 there, and
`internal/diff/slp.go`):

    -    5,    5          the master's records 5 to 5 replaced by ...
    data line 5, revised  ... the records that follow
    -   15                insert after the master's record 15
    inserted one
    inserted two
    -   26,   27          records 26 to 27 deleted (nothing follows)
    /                     the end of the script

Numbers are five columns wide, more when they need it; an insertion
before the first record is a bare `-`; identical files give only `/`.
Every line number refers to the *original* input file, never to the file
as edited so far: that is what lets several update files be merged.

What the references available here don't give: the SUMSLP Utility
Manual itself (it is on the documentation CD, which may be among
`~/Documents/Technical Doc/VMS/CD Images/`'s ISOs; subtask 2 looks), and
VMS's online help for EDIT/SUM, which the probe captures first (Part D,
section HELP). The VMS source archive is off limits (clean room).

### A.2 What govax has today

- **EDIT** is govax's own (`internal/dclcmd/edit.go`): a host editor on
  a scratch copy, with one parameter and `/HOST`. EDIT/SUM must be a
  second syntax of the same verb, chosen by its qualifier, as ANALYZE's
  kinds are (`console.dcl`: a qualifier with `/syntax=`); a plain EDIT
  stays what it is.
- **Reading and writing record files**: `rms.Session.ReadRecordFile`
  and `CreateRecordFile` (host or volume, through Phase 93's checked
  layer), `LocateOutput` for the output's name, `dir.NextVersion` for a
  next higher version. DIFFERENCES' command (`dclcmd/differences.go`)
  has the pattern for inputs, an output file, and a DIF status.
- **The scripts**: `internal/diff`'s `SLP` writes them; nothing reads
  them.
- **Messages and statuses**: `vmsdef`'s facility tables, as Phase 116's
  `difmessages.go`.
- **Test material**: `testdata/dif116` has 62 SLP scripts VMS 7.1's
  DIFFERENCES wrote (`vax/out/*.DIF`) with their master and revision
  files in `cases.txt`: a round trip on VMS's own scripts at no cost.
- **MACRO** has no `/UPDATE` (`internal/asm`, `dclcmd/macro.go`).

### A.3 What the references leave open

The probe (Part D) is to answer each:

1. **The command**: EDIT/SUM's qualifiers and their defaults (from the
   VAX's HELP): how the update files are named (`/UPDATE=` on the
   input, as MACRO's, or otherwise), `.UPD`'s default, the output's name
   (`/OUTPUT`; by default the input's name, next higher version?), the
   listing (`/LISTING`, its default name and type), any header or
   audit-trail qualifiers; wildcards; a missing input or update file;
   whether `EDIT/SLP` is accepted.
2. **The edit commands**: `-n`, `-n,m`, a bare `-`; what else a command
   line may carry (an audit trail text, a text locator, comments); the
   forms the numbers may take (leading blanks, zero, signs); `/` as the
   end, what follows it, and a file with no `/`; how an inserted record
   that itself starts with `-` or `/` is written (none of Phase 116's 62
   scripts had one: does DIFFERENCES/SLP escape it, and how does SUMSLP
   read it?).
3. **Order and range**: commands out of order, overlapping or repeated
   ranges in one file, a number past the input's end, `-0`, deleting the
   last record, inserting after it, an empty input, an empty update
   file.
4. **Merging several files**: disjoint edits; two inserts at the same
   place (which goes first: the list's order?); a replace in one and an
   insert or replace of the same records in another (a conflict: an
   error, a warning, or one wins?); the order of the `/UPDATE` list.
5. **The audit trail**: where the changes are marked in the output and
   the listing (columns, the text, its length, a record that reaches the
   column, tabs), and what deleted records leave.
6. **The listing**: its layout to the byte, if SUMSLP writes one.
7. **Messages and statuses**: facility, numbers, texts, severities, and
   `$STATUS` for success, every syntax error, and every conflict;
   whether F$MESSAGE knows them.
8. **Files**: the output's record format and attributes (DIRECTORY/FULL)
   for VAR, VFC, STREAM_LF, and FIX inputs; records with every byte;
   long records.
9. **The round trip**: VMS's own DIFFERENCES/SLP scripts applied by
   VMS's SUMSLP to their masters give the revisions (on Phase 116's
   pairs and new generated ones), and where they don't.
10. **MACRO/UPDATE** (only if the review keeps it in scope, question 2):
    the source as updated, the listing's audit trail, the `.UPD` default,
    a missing update file's message.

## Part B — decisions (proposed)

1. **VMS 7.1 is the reference**, as for every probe since Phase 55.
2. **The engine is in `internal/diff`** beside the writer: a reader
   (`ParseSLP`: a script's text into edits, with each error's line) and
   `Apply` (an input's records and the merged edits of several scripts
   into an output's records and an audit of what changed). Plain Go,
   knowing nothing of RMS or DCL, so its tests are fast. The package's
   name stays `diff` (it is the SLP format's home); if the author prefers
   a package of its own, `internal/sumslp`, the reader moves there and
   `diff.SLP` stays the writer.
3. **The command is `internal/dclcmd/sumslp.go`**: names, files, output,
   listing, status. The grammar: EDIT's syntax switched by `/SUM` (and
   `/SLP`) to an `edit_sum` syntax with its own parameter and qualifiers,
   the update files a positional qualifier if VMS's are, so its messages
   are DCL's.
4. **The rules are fitted to VMS's output**, never written from VMS's
   code (clean room): each rule written in `doc.go` with the cases that
   showed it; what no case settles is a choice marked unconfirmed and
   listed in DEVIATIONS.md.
5. **The round trip is a test**: `diff.SLP` then `Apply` gives the
   revision for every pair `TestProbeListings` reads and for seeded
   random pairs, in milliseconds, and the 62 VMS scripts applied to
   their masters give their revisions.
6. **Messages** go into `vmsdef` as SUMSLP's facility (its number from
   the probe's `$STATUS` values), as DIF's did.
7. **MACRO/UPDATE** is out of this phase unless the review says
   otherwise (question 2); the probe asks about it anyway, so the
   answers are on hand.

## Part C — subtasks

Each is one commit, with tests, stopping for review.

1. **The survey**: Part A, this plan.
2. **The probe**: the documentation CD searched for the SUMSLP manual;
   `testdata/sum117`, as in Part D, written and its exchange volume
   built; run on the VAX by the author (round 1 starts with HELP, so a
   short round 2 may follow for what HELP shows that round 1 didn't ask);
   A.3 answered (as A.4) and Part B adjusted.
3. **The command's frame**: the grammar (EDIT/SUM and EDIT/SLP beside
   plain EDIT), HELP EDIT's `/SUM` topic in VMS's shape, the messages and
   statuses in `vmsdef`, the names (input, updates, output, listing),
   the errors; held to the probe's COMMAND section.
4. **The engine, one update file**: `ParseSLP` and `Apply`, every edit
   command and its errors; held to the BASICS and ORDER sections, and the
   round trip over Phase 116's pairs and VMS's 62 scripts.
5. **Merging**: several update files, their order and conflicts; held
   to MERGE.
6. **Audit trail and listing**: held to AUDIT and LISTING.
7. **Files and the round trip on VMS**: record formats, every byte, long
   records; held to FORMATS and ROUNDTRIP.
8. **The close-out**: MANUAL.md (section 6's file commands, beside
   DIFFERENCES, with a worked DIFFERENCES/SLP and EDIT/SUM example run on
   govax), HELP, DEVIATIONS.md (what stays unconfirmed), PLAN.md,
   CLAUDE.md.

(If the review puts MACRO/UPDATE in scope, it is a subtask between 7
and 8.)

## Part D — the probe (subtask 2)

`testdata/sum117/`, on an exchange volume as Phase 116's was
(`exchange.cmd`, `copyout.cmd`, an RD53 in `testdata/disks`, attached to
the simh VAX). `gen.go` writes every file but the README, and
`cases.txt`, the whole probe (every file's records and every case), is
the one record govax's oracle tests read; `in/`, the host copies, is
made from it and gitignored.

`sum117.com` runs with SET VERIFY, a section per A.3 question. Each
case runs EDIT/SUM with `/OUTPUT=[SUM117.OUT]Cnnn.OUT` (and the listing,
if any, to `[SUM117.OUT]Cnnn.LIS`), then writes `$STATUS` and
`F$MESSAGE($STATUS)` to the log under a `#### CASE id: title` line; error
cases run at the log so their messages show. Output goes to files, so the
bytes come back unchanged. After each case that should succeed,
`DIFFERENCES` of the output against the expected file goes to the log
too, a quick reading of the result.

- **HELP**: `HELP EDIT /SUM` and every subtopic, `HELP SUMSLP`, written
  to the log first (the online help is the manual we lack).
- **COMMAND**: each qualifier the help names, with and without values;
  the update file's default type; the output's and listing's default
  names (run in a scratch directory, then DIRECTORY); a missing input; a
  missing update file; a wildcard; `EDIT/SLP`; no update file at all.
- **BASICS**: replace one record, many, all; insert after the first,
  middle, last record; insert before the first (bare `-`, and `-0`);
  delete one, many, the last, all; several commands in one file; an empty
  update (only `/`); a script with no `/`; text after `/`; inserted
  records that start with `-`, `/`, a blank, or a tab; empty inserted
  records; a long record.
- **SYNTAX**: number forms (blanks, zero, a sign, a huge number), a
  missing number after the comma, extra commas, letters, an audit or
  locator text if HELP shows one; each its message.
- **ORDER**: commands out of order, overlapping and repeated ranges in
  one file, a number past the input's end, a range reversed (`-9,5`).
- **MERGE**: two and three update files: disjoint; inserts at the same
  place (each order of the list); replace against replace, insert, and
  delete of the same records; a range in one inside another's; each
  conflict's message and status, and what the output holds.
- **AUDIT**: the audit trail in the output and listing for replaced,
  inserted, and deleted records; records that reach its column; tabs.
- **LISTING**: its layout for one update and several.
- **FORMATS**: VAR, VFC, STREAM_LF, and FIX inputs made by
  `exchange.cmd`'s COPY `/RECORD_FORMAT` (Phase 86), records holding
  every byte; DIRECTORY/FULL of the outputs.
- **ROUNDTRIP**: Phase 116's 62 scripts (VMS's own) applied to their
  masters, and 200 seeded pairs: `DIFFERENCES/SLP`, then `EDIT/SUM`, then
  `DIFFERENCES` of the result against the revision, its status logged.
- **MACRO**: two `.MAR` files with `.UPD` files, `MACRO/UPDATE/LIST`,
  the listing kept; a missing `.UPD`.

`copyout.cmd` brings back the log, every output and listing, and the
DIRECTORY/FULL. The oracle tests (subtask 3 on) read the case list from
`cases.txt`, run each case on govax, and compare the output files byte
for byte and the log's messages and `$STATUS` lines.

## Questions for the review

1. **The synonym**: `EDIT/SLP` as govax's own if VMS 7.1 refuses it
   (the plan), or only if 7.1 accepts it?
2. **MACRO/UPDATE**: in this phase (a subtask before the close-out), a
   later phase, or not at all? The probe asks about it either way.
3. **The package**: the reader in `internal/diff` beside the writer
   (Decision 2), or `internal/sumslp` of its own?
4. **The number**: 117 is the next free one (114 and 115 are planned and
   not started).

## Progress log

- **2026-11-25 — subtask 1, the survey.** The DCL Dictionary's EDIT/SUM
  and MACRO/UPDATE entries read; Phase 116's SLP writer and its 62 VMS
  scripts found as test material; govax's EDIT grammar and file pieces
  found; the plan and the probe's design written.
