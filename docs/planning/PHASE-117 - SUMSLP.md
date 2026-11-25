# Phase 117 — SUMSLP (EDIT/SUM)

**Status:** planned 2026-11-25 (subtask 1, the survey and this plan);
the plan reviewed and accepted by the author the same day, with the
answers recorded in "The review". This document is the development plan; as
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
if VMS 7.1 refuses it, it is govax's own and DEVIATIONS.md says so (the
review's answer 1). MACRO's `/UPDATE`, which runs the same editor on a
source before assembling it, is part of the phase (answer 2).

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

**The SUMSLP Utility Manual** (found in subtask 2): the *OpenVMS SUMSLP
Utility Manual*, AA-PS6EA-TE (May 1993, for OpenVMS VAX 6.0 and AXP 1.5),
is on the documentation CD VAXDOCMAR012 (`CD Images/AG-QL93Z-RE.iso.zip`),
as the Bookreader book `[OPSYS.VMSOS721.ARCHIVE]DY4YAA44.BKB`. A copy of
the book and its text, pulled out with govax's MOUNT and COPY, are beside
the other manuals: `~/Documents/Technical Doc/VMS/sumslp-utility-manual-
AA-PS6EA-TE.bkb` and `.txt` (the text is the book's words in order, its
layout lost; the examples' listings are readable). It is DIGITAL's manual,
usable under the clean-room rules. What it says:

- **Format**: `EDIT/SUM input-file`. Command qualifiers `/[NO]LISTING
  [=file]` (default /NOLISTING; the input's name, type `.LIS`),
  `/[NO]OUTPUT[=file]` (default: the input's name and type, a new
  version), `/[NO]HEADER` (default /NOHEADER: the output VFC, "the line
  insert number and audit trail information in the record header block").
  `/UPDATE[=(file[,...])]` is a file qualifier only: type `.UPD` by
  default, the first file's other fields from the input's, each later
  file's from the one before (`/UPDATE=(UPD3A,UPD3B.ENH,UPD3C)` reads
  `UPD3C.ENH`); a bare `/UPDATE` is the input's name, type `.UPD`; no
  `/UPDATE` applies nothing (a way to get a numbered listing). A command
  procedure may give `/UPDATE=SYS$INPUT` and the update after the
  command. No privileges. The input may have at most 32,767 lines; the
  manual warns against large scripts from DIFFERENCES/SLP for that reason.
- **Operators**, as a line's first character: `-` an edit command; `\`
  suppresses the audit trail; `%` enables it again; `/` (in column 1)
  ends the update file; `<` makes the next operator an ordinary character
  (`</` is a line starting with a slash).
- **The edit command**: `-locator1,locator2,/audittrail/;comment`, every
  field positional, commas kept for omitted fields. A locator is a number
  from 1 to 32,767 or `.`, the current line. One locator: the current line
  moves there, and the text lines that follow are inserted after it. Two:
  lines locator1 to locator2 are deleted, and the text lines that follow
  inserted in their place (a replace). `-5,,/;**08-JUN**/` is an insert
  with an audit trail; `-,,/newtrail/` changes the audit trail for every
  later line. `;comment` is ignored. SUMSLP edits sequentially: the line
  pointer never moves back.
- **Several update files**: commands are applied in ascending order of
  locator1; edits that don't conflict are all applied; a conflict goes to
  the file of higher precedence, the last listed highest. All inserts at
  one line are kept, the highest precedence's first; a delete or replace
  takes away the lines it names and any lower-precedence inserts or
  replacements at them.
- **The output** has no audit trail or line numbers. **The listing** has
  each line's number, an inserted line numbered `.1`, `.2`, ... from each
  series' start, the audit trail (by default `; **NEW**`, the text cut in
  the book), and on the first unchanged line after a deletion a marker
  `-n`, n the lines deleted or replaced. Its examples show the layout's
  order but not its columns.
- **Messages** are in the *System Messages and Recovery Procedures
  Reference Manual*; the 7.3 edition here has no SUM entries (it lists the
  facility only), so the probe collects them.

What the manual doesn't settle is A.3. The VMS source archive stays off
limits (clean room).

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

1. **The command**: the defaults the manual gives, confirmed; `/UPDATE`
   after the verb and `/LISTING` after the file (the manual says each is
   one kind only); `/NOOUTPUT`; wildcards; a missing input, a missing
   update file (MACRO's entry says informational for MACRO); whether
   `EDIT/SLP` is accepted; `/UPDATE=SYS$INPUT` with and without a
   terminator; the online help's own words.
2. **The edit commands' edges**: a bare `-` (DIFFERENCES/SLP's insert at
   the start) and `-0`; the number forms (blanks, zeros, signs, past
   32,767, past the input's end); the comment with and without the
   commas; the terminator missing or followed by more; `\` and `%` alone
   on a line or before text; `<` before a character that is no operator;
   how DIFFERENCES/SLP writes a revision record that starts with an
   operator (none of Phase 116's 62 scripts had one).
3. **Order in one file**: commands out of order, overlapping or repeated
   (the manual: the pointer never moves back).
4. **Merging**: the precedence rule's cases one by one, in both orders,
   and whether a conflict is reported.
5. **The audit trail**: the default text exactly, its column in the
   listing, records that reach it, tabs, a long trail, `\` and `%`; what
   `/HEADER`'s record headers hold.
6. **The listing**: its layout to the byte: headings, the numbers'
   columns, `.n`, the `-n` marker (also at the end of the file), numbers
   past 999 and 9999, an empty input.
7. **Messages and statuses**: facility, numbers, texts, severities, and
   `$STATUS` for success and every error; whether F$MESSAGE knows them.
8. **Files**: the output's record format and attributes for VAR, VFC,
   STREAM_LF, and FIX inputs; records with every byte; 32,767 and 32,768
   lines.
9. **The round trip**: VMS's DIFFERENCES/SLP script applied by VMS's
   SUMSLP gives the revision, and where it doesn't.
10. **MACRO/UPDATE**: the source as updated, the listing's audit trail,
    `.UPD`'s default, a missing update file's message, a list and a
    concatenation, `/NOUPDATE`, `/UPDATE` after the verb.

## Part B — decisions (proposed)

1. **VMS 7.1 is the reference**, as for every probe since Phase 55.
2. **The engine is a package of its own, `internal/sumslp`** (the
   review's answer 3): a reader (`Parse`: a script's text into edits,
   with each error's line), `Merge` (several scripts' edits into one
   list, with their conflicts), and `Apply` (an input's records and the
   merged list into an output's records and an audit of what changed).
   A leaf package, plain Go, knowing nothing of RMS or DCL, so its tests
   are fast. `diff.SLP` stays the writer; `sumslp`'s tests import `diff`
   for the round trip, the package itself doesn't.
3. **The command is `internal/dclcmd/sumslp.go`**: names, files, output,
   listing, status. The grammar: EDIT's syntax switched by `/SUM` (and
   `/SLP`) to an `edit_sum` syntax with its own parameter and qualifiers:
   `/LISTING`, `/OUTPUT`, and `/HEADER` the verb's, `/UPDATE` the file's
   (the manual), so its messages are DCL's.
4. **The rules are fitted to VMS's output**, never written from VMS's
   code (clean room): each rule written in `doc.go` with the cases that
   showed it; what no case settles is a choice marked unconfirmed and
   listed in DEVIATIONS.md.
5. **The round trip is a test**: `diff.SLP` then `sumslp.Apply` gives the
   revision for every pair `TestProbeListings` reads and for seeded
   random pairs, in milliseconds, and the scripts VMS wrote (Phase 116's
   62 and this probe's 136) applied to their masters give their
   revisions.
6. **Messages** go into `vmsdef` as SUMSLP's facility (its number from
   the probe's `$STATUS` values), as DIF's did.
7. **MACRO/UPDATE is in this phase** (the review's answer 2): a
   positional qualifier on each source, the update files merged by
   `sumslp` and the updated text handed to the assembler (`internal/asm`
   never sees the files), the listing's audit trail as VMS's, a missing
   update file an informational message and the assembly going on.

## Part C — subtasks

Each is one commit, with tests, stopping for review.

1. **The survey**: Part A, this plan.
2. **The probe** (written 2026-11-25; the VAX run waits for the author):
   the documentation CD searched for the SUMSLP manual (found, A.1);
   `testdata/sum117`, as in Part D, written, rehearsed on govax, and its
   exchange volume built; run on the VAX by the author (a short round 2
   may follow for what round 1's HELP shows that it didn't ask); A.3
   answered (as A.4) and Part B adjusted.
3. **The command's frame**: the grammar (EDIT/SUM and EDIT/SLP beside
   plain EDIT), HELP EDIT's `/SUM` topic in VMS's shape, the messages and
   statuses in `vmsdef`, the names (input, updates, output, listing),
   the errors; held to the probe's COMMAND section.
4. **The engine, one update file**: `internal/sumslp`'s `Parse` and
   `Apply`, every edit
   command and its errors; held to the BASICS and ORDER sections, and the
   round trip over Phase 116's pairs and VMS's 62 scripts.
5. **Merging**: several update files, their order and conflicts; held
   to MERGE.
6. **Audit trail and listing**: held to AUDIT and LISTING.
7. **Files and the round trip on VMS**: record formats, every byte, long
   records; held to FORMATS and ROUNDTRIP.
8. **MACRO/UPDATE**: the grammar's positional qualifier, `.UPD`'s
   default, the source updated through `sumslp` before assembly, the
   listing's audit trail, a missing update file's message; held to the
   MACRO section (listings and objects).
9. **The close-out**: MANUAL.md (section 6's file commands, beside
   DIFFERENCES, with a worked DIFFERENCES/SLP and EDIT/SUM example run on
   govax, and MACRO's chapter for `/UPDATE`), HELP, DEVIATIONS.md (what
   stays unconfirmed), PLAN.md, CLAUDE.md.

## Part D — the probe (subtask 2)

`testdata/sum117/` (its README has the run's steps), on an exchange
volume as Phase 116's was (`exchange.cmd`, `copyout.cmd`, an RD53 in
`testdata/disks`, attached to the simh VAX). `gen.go` writes every file
but the README, and `cases.txt`, the whole probe (every file's records
and every case's commands), is the one record govax's oracle tests read;
`in/`, the host copies, is made from it and gitignored.

`sum117.com` runs with SET VERIFY, a section per A.3 question, 346 cases.
A case is DCL lines before, steps, and DCL lines after; after each step
the log has `STATUS` with `$STATUS` (written as the string it is, Phase
116's lesson) and `F$MESSAGE`'s text. EDIT/SUM's steps put the output in
`[SUM117.OUT]ID.OUT` and the listing in `ID.LIS`, so their bytes come back
unchanged; the default-name cases run in `[SUM117.SCR]` on copies, list
it, and keep what they made as `ID.Kn`. No command line is longer than
200 characters.

- **HELP** (6): `HELP/NOPROMPT EDIT /SUM...`, `SUMSLP...`, `EDIT`,
  `MACRO /UPDATE...`, and `HELP/MESSAGE/FACILITY=` SUM and SUMSLP.
- **COMMAND** (31): `.UPD`'s default, a bare `/UPDATE`, none, three files
  with defaults carried (`C04C` read as `.ENH`), a directory carried from
  `[-.IN2]`, qualifiers in the wrong place, the default output and listing
  names, `/NOOUTPUT`, `/OUTPUT=.NEW`, the input named `;1`, missing and
  wildcarded files, two inputs, a directory file, `EDIT/SLP`,
  `/READ_ONLY` and `/TECO`, `/UPDATE=SYS$INPUT` with and without `/`, the
  output written over the input's name.
- **BASICS** (41): every edit on `S10.SRC` (ten lines), the operators,
  the period, comments, the terminator, records starting with each
  operator, blank, tab, empty, and 300-byte records.
- **SYNTAX** (24) and **ORDER** (12): A.3's 2 and 3.
- **MERGE** (34): 18 shapes of two or three files meeting, most in both
  orders, each file with its own audit trail in one.
- **AUDIT** (18): the trail's text, column, and changes on records up to
  140 bytes and with tabs; deletions' markers; `/HEADER` with `DUMP/RECORD`
  of the output.
- **LISTING** (11): no update, an empty file, 1,200 and 12,000 lines, a
  long insert, 32,767 and 32,768 lines.
- **FORMATS** (17): VFC, STREAM_LF, and fixed inputs (one holding every
  byte) and update files, control and 8-bit bytes, form feeds, trailing
  blanks; DIRECTORY/FULL and DUMP/RECORD of each output.
- **ROUNDTRIP** (136): `DIFFERENCES/SLP`, `EDIT/SUM` of the script, and
  `DIFFERENCES` of the result with the revision; 16 by hand (empty files,
  identical files, revision records starting with each operator or like
  an edit command) and 120 generated from a fixed seed.
- **MACRO** (16): `SUMA.MAR`, `SUMB.MAR`, and `SUMP.MAR` (a macro
  definition, no `.END`) with update files: A.3's tenth question, the
  listings and objects kept.

`sum117t.com` is the terminal, by hand: the output and listing to TT:, the
prompt for a missing parameter, `/UPDATE=SYS$COMMAND` typed (with and
without `/`), an error's messages. `copyout.cmd` brings back the log and
every output. The oracle tests (subtask 3 on) read `cases.txt`, run each
case on govax, and compare the outputs byte for byte and the log's
messages and `STATUS` lines.

## The review (2026-11-25)

The author's answers to the plan's four questions:

1. **`EDIT/SLP`**: yes, govax's own synonym if VMS 7.1 refuses it.
2. **MACRO/UPDATE**: in this phase (subtask 8, Decision 7).
3. **The package**: a new one, `internal/sumslp` (Decision 2).
4. **The number**: 117 is right.

## Progress log

- **2026-11-25 — subtask 1, the survey.** The DCL Dictionary's EDIT/SUM
  and MACRO/UPDATE entries read; Phase 116's SLP writer and its 62 VMS
  scripts found as test material; govax's EDIT grammar and file pieces
  found; the plan and the probe's design written.
- **2026-11-25 — the review.** The author answered the four questions
  ("The review"): EDIT/SLP as govax's own if need be, MACRO/UPDATE in the
  phase (subtask 8), the engine in `internal/sumslp`, number 117.
  Decisions 2 and 7 and Part C adjusted, the probe's MACRO section
  widened.
- **2026-11-25 — subtask 2, the probe written.** The documentation CDs
  among `CD Images` searched with govax (each ISO an ODS-2 volume,
  `MOUNT/NOWRITE`): the VMS 7.1 kit (VAXVMS071) has SUMSLP.EXE and
  SUMSHR.EXE, and VAXDOCMAR012 the *OpenVMS SUMSLP Utility Manual* as a
  Bookreader book, whose text is now A.1; A.3 narrowed to what the manual
  leaves open. `testdata/sum117` written (`gen.go`, 346 cases), its
  exchange volume built, and `sum117.com` rehearsed on govax on a copy of
  it: every step but EDIT/SUM, MACRO/UPDATE, and HELP/MESSAGE (none of
  which govax has yet) ran clean. The rehearsal found that govax's COPY
  doesn't take a comma list of inputs (`COPY A,B,C DIR:`, which VMS
  does: `%COPY-E-SEARCHFAIL` on the whole list as one name) and that
  DIRECTORY's `/DATE` takes no value (VMS's `/DATE=CREATED`); the probe
  uses neither, and both are for a later fix. The VAX run waits for the
  author (README.md's steps).
