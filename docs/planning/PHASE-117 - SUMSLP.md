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

### A.4 The probe's answers (round 1, 2026-11-25)

Round 1 ran whole on VMS 7.1 (`testdata/sum117/vax/sum117.log`, 627 files
in `vax/out/`, the terminal in `vax/terminal.log`). Case IDs are
`cases.txt`'s.

1. **The command.**
   - EDIT/SUM's online help is the EDIT topic's one line, "Invoke the
     SUMSLP batch-oriented editor to update a single input file with
     multiple files of edit commands (see /SUM)", and a `/SUM` subtopic
     with no text ("Sorry, no documentation on EDIT /SUM...", H01);
     there is no SUMSLP topic and no MACRO /UPDATE one (H02, H04).
     HELP/MESSAGE/FACILITY=SUM (or SUMSLP) lists EDITSCLSH "edits
     clash", EDOUTSEQ "edits out of sequence", and PRMEOF "premature
     end-of-file" (H05, H06).
   - `.UPD` is the update file's type (C01); a bare `/UPDATE` is the
     input's name and `.UPD` (C02, C14); none copies the input and lists
     it (C03). Each later update file takes the device and directory of
     the one before (C16: `C16B` read from `[SUM117.IN2]`) but **not**
     its type, against the manual's example 3: C04's `C04C` was read as
     `C04C.UPD`, not found, `%SUM-I-OPENIN` and `-RMS-E-FNF`, and the
     other two applied.
   - `/UPDATE` after the verb is `%DCL-W-IVQLOC` (C05); `/LISTING` and
     `/OUTPUT` work after the file too (C06, C07).
   - No `/OUTPUT` is the input's name, a new version (C08, C10, C13: even
     for `S10.SRC;1`); a bare `/LISTING` is `S10.LIS` (C09);
     `/OUTPUT=.NEW` and `/LISTING=.LST` take the input's name (C12);
     `/NOOUTPUT` makes no output (C11); `/NOLISTING` after `/LISTING`
     makes none (C24).
   - A missing input: `%SUM-I-OPENIN, error opening
     DUA1:[SUM117.IN]NOSUCH.SRC; as input` / `-RMS-E-FNF`, status
     `%X10018292` (C17). A missing update file: the same `-I-` message,
     the others applied, success (C18, C19). A wildcard in either:
     `%SUM-E-OPENIN` / `-RMS-F-WLD`, `%X10018744` (C20, C21). Two inputs:
     `%DCL-W-NOLIST` (C27). Qualifiers EDIT/SUM lacks: `%DCL-W-IVQUAL`
     (C25, C26).
   - **EDIT/SLP** fails (C23): `%DCL-W-IVQUAL` naming `\UPDATE\`, not
     `\SLP\`, so 7.1's EDIT takes `/SLP` as something else; round 2's
     SLPVERB asks what.
   - **Statuses are RMS's**, with bit 28 (inhibit message) set: success
     `%X10010001` (RMS-S-NORMAL), whatever warnings the run had (B17's
     PRMEOF, G12's EDITSCLSH, Y03's SLPSYNERR); a failure is its RMS
     status (FNF, WLD, RAC). SUM's own message numbers never reach
     `$STATUS`, so the probe has none (F$MESSAGE knows only RMS's).
   - **Messages**: the OPENIN and READERR failures go to SYS$OUTPUT (and
     the log). The editing warnings (SLPSYNERR, EDITSCLSH, EDOUTSEQ,
     PRMEOF) go **into the listing** when there is one, else to
     SYS$OUTPUT (the terminal's Y08). Each is the message line, then a
     line per edit it is about: the command line as written, four
     blanks, the update file's full name (`-5,5    DUA1:[...]G03B.UPD;1`;
     PRMEOF's command is empty: `    DUA1:[...]B17.UPD;1`, or
     `SYS$INPUT:.;`).
   - `/UPDATE=SYS$INPUT` in a procedure works (C29), and without `/` is
     PRMEOF at the next `$` line (C30). Under SET VERIFY the log shows
     the data lines read twice, the command lines doubled the second
     time: SUMSLP reads each update file twice. At a terminal
     (`/UPDATE=SYS$COMMAND`) the first reading ends at `/` (or Ctrl/Z,
     shown ` Exit `) and the second fails: `%SUM-E-READERR, error reading
     SYS$COMMAND:.;` / `-RMS-F-RAC`, `%X10018644`; the author's session
     ended at the first `/`.
   - Missing parameter: `%DCL-W-INSFPRM` (no prompt).
2. **The edit commands.**
   - A bare `-`, `-0`, `-.` at the start, and `-0,5` all insert before
     line 1 (B08, B09, B33, Y24); `-n` inserts after line n, `-m,n`
     replaces m to n, nothing following deletes them.
   - Blanks and tabs inside a command are ignored (`- 5,5`, `-5 , 5`,
     `-\t5,5`, `-5   `: Y01, Y04, Y15, Y23), as are leading zeros
     (Y02); `-5 6` is a syntax error (Y11). A trailing comma, `-5,`, is
     an insert (B38). `-5,5,;Y16` is a replace (the third field not an
     audit trail, ignored, Y16); `-5,5,//` an empty trail (Y18).
   - `;` starts a comment wherever it is (B34, B35, B37); after the trail
     too (B36).
   - **Syntax errors** (SLPSYNERR): a sign (Y03, Y19), letters (Y08,
     Y09), `%X5` (Y20), an unterminated trail (Y10), `-5,/;Y17/` (Y17),
     extra commas (Y07), a reversed range (O04), a record starting with
     `-` (B20). The bad command is **numbered as an inserted line but not
     written** (the next line is `.2`), and the text after it is
     inserted where the edit before left off (Y21) or before line 1.
   - **The terminator**: `/` in column 1 ends the file, whatever follows
     on its line (B39); a line ` /` is text (B40); after `/` nothing is
     read (B18). No `/` is PRMEOF, the edits applied (B17); an empty
     update file applies nothing, with no message (B19).
   - **Operators** (B21 to B28, B41): `<` makes the next character
     ordinary (`<-`, `</`, `<<`, `<\`, `<%`), and is dropped before any
     other (`<B24` is `B24`); a line of only `<` is an empty record.
     `\` and `%` lines turn the audit trail off and on and **their text
     is dropped** (`\B25 ...` and `%B26 ...` vanish).
   - **Record length**: an update record of 300 bytes is `%SUM-E-READERR,
     error reading ...B30.UPD;1` / `-RMS-W-RTB, 300 byte record too
     large for user's buffer`, nothing applied, success (B30); round 2's
     LENGTHS finds the limit, and a source record's (140 is fine).
   - **DIFFERENCES/SLP** writes a revision record that starts with `-`,
     `/`, `<`, `\`, or `%` after a `<` (R09 to R14), which govax's
     `diff.SLP` didn't: fixed (commit ae4b4e3d, `TestSum117Scripts`).
3. **Order in one file** (O01 to O12) and **4. merging** (G01 to G18,
   C22) fit one rule, with one exception. The edits of every update
   file, the first listed lowest, a file's later lines above its
   earlier ones, are sorted by locator1, the higher first among equals,
   and applied in one pass over the input:
   - a replace or delete whose first line has already been written is
     dropped (O02, O06, O09, O12, G03, G04, G06, G07, G17R, G18);
   - an insert after a line already passed goes at the current place
     (O05, O08, G05: an insert after 5 inside a delete of 4 to 6 follows
     it);
   - EDOUTSEQ is reported for a command whose locator1 is not above the
     one before it in its file (O01, O03, O06 to O10, O12, B32); it is
     listed just before that edit is applied;
   - EDITSCLSH is reported for edits whose ranges meet (an insert after
     n being n's range): two inserts at one line (O03, G02, G13, C22),
     an insert inside a range (O05, G05), overlapping ranges (O02, G07,
     G18), but not adjacent ones (O10, O11, G10). It is listed when the
     pass reaches the first line of the first of them, before that line,
     naming them in the sorted order.
   - Inserts at one line come out highest first (O03, G02, G13); a
     replace and an insert at one line both survive if the replace is
     higher (O07, G04R, G17), the insert only if it is (O06, G04, G17R).
   - **The exception, G14**: a replace of 5, a delete of 4 to 6, and an
     insert after 5 in three files, in both orders, write only the
     delete: the insert is lost, where G05 kept it. Round 2's CLASH2
     asks why.
   - The manual's "a delete takes away lower-precedence inserts" (3.3)
     isn't what G05 shows.
5. **The audit trail** (U01 to U10, G12, B36):
   - The default trail is **empty**: nothing is marked (U01, C01).
   - A trail given on a command (`/;U02 AUDIT/`) or by `-,,/text/`
     marks every line inserted after it **in that update file**, until
     changed; `-,,/;U03 NEW/` after an insert marked that earlier insert
     too (U03: the trail seems to be the file's at its end, or applied
     when the edits are sorted; round 2 doesn't ask, so unconfirmed).
     Each file has its own (G12).
   - It shows only in the listing (and `/HEADER`'s record headers), cut
     to 16 characters in the listing (U06), any text (U07, U08). A `\`
     line suppresses it for the lines after, `%` brings it back (U09).
     Deleted lines have none (U10).
6. **The listing** (all `.LIS`, L01 to L08):
   - A variable-length file, CR carriage control. Pages of 60 lines: two
     heading lines, a blank line, and 57 lines. Heading line 1 is a form
     feed, 60 blanks, the listing's date and time (`25-NOV-2026
     14:37:46`), 4 blanks, `VAX-11 SUMSLP V01.00`, 16 blanks, `Page`,
     and the page number right-aligned in 5; line 2 is 60 blanks, the
     input's date (creation or revision: round 2's DATES asks), 4 blanks,
     and the input's full name, blank-padded (to 45 characters in C01;
     round 2's P04 shows a long one).
   - Each line: a 16-column field (the audit trail, or the `-n`
     deletion marker), the line number right-aligned in 7 columns
     (`.1`, `.2`, ... for inserted lines, counted from each run's start;
     an insert next to a replace is one run, L04), a blank, and the
     record as it is (tabs kept, never cut: 140 bytes in U04).
   - The marker `-n` (n the lines deleted or replaced) is on the first
     line after them that is written; a deletion at the input's end
     leaves none (B12, B13, U05). Runs of deleted lines separated only by
     written lines each get their own (B15).
   - Messages are listing lines (do they count toward the 57? round 2's
     P01).
   - An empty input's listing is the heading alone (L02).
7. **Files** (F01 to F09, U11 to U14, L09, L10):
   - The output is always variable-length, CR carriage control, whatever
     the input's format (VFC, Stream_LF, fixed). Records come out byte
     for byte (control and 8-bit bytes, form feeds, tabs, empty records,
     trailing blanks).
   - `/HEADER`: VFC with a 12-byte header, CR carriage control: a word
     with the input's line number (0 for an inserted line), a word with
     the insert number (`.n`'s n, 0 for an input line), and 8 bytes of
     the audit trail, zero-filled.
     *(Subtask 6, from the VAX's own files: for an input line the second
     word is the count its `-n` marker shows, 0 for none; U11's line 6 has
     1.)*
   - No limit at 32,767 lines: 32,768 lines edited as well (L10).
   - (The probe's own inputs were Stream_LF, govax's COPY/HOST's
     default, not variable as `cases.txt` says: F05S.)
8. **The round trip**: all 136 pairs give the revision back exactly
   (compared on the host; the procedure's own DIFFERENCES took its second
   file's directory from the first and failed, a slip in the probe).
9. **Past the end** (Y06, Y12 to Y14): an insert after a line past the
   end is lost, with no message; a replace or delete reaching past the end
   reads the input **again from its start**, numbering on (Y13: the ten
   lines, the text, then lines 11 to 20, the input once more). A locator
   past 32,767 is EDOUTSEQ and went after line 1 (Y05's 99999). Round 2's
   NUMBERS and PASTEND look for the rules.
10. **MACRO/UPDATE** (M01 to M16): `/UPDATE` is a file qualifier
    (IVQLOC after the verb, M11); a missing update file is
    `%SUM-I-OPENIN` and the assembly goes on (M07, M16); `/NOUPDATE`
    after `/UPDATE` applies none (M08). The listing is MACRO's own, with
    SUMSLP's numbering in the line-number column (`.1`, `.2`) and its
    16-column field (audit trail, `-n`) over the left of the object code
    columns (M10); SUMSLP's warnings are lines in it (M13). The listing
    header and assembly messages otherwise as MACRO's. The objects and
    listings are kept.

What round 1 left is round 2 (`sum117b.com`, 76 cases): CLASH2 (G14's
exception), NUMBERS, PASTEND, LENGTHS (update records of 132 to 513
bytes, source records of 255 to 32,767), PAGES (messages and a long
insert at a page's end, a long name in the heading), SLPVERB (what
EDIT/SLP is), and DATES (which date the heading shows).

### A.5 Round 2's answers (2026-11-25)

Round 2 ran whole (`vax/sum117b.log`, its outputs in `vax/out/` beside
round 1's). Messages that go to the terminal rather than the log
(`%DCL-W-ACTIMAGE`) showed at the console too.

1. **EDIT/SLP is a different utility on VMS 7.1** (SLPVERB): `EDIT/SLP
   S10.SRC` runs an image BACKTRANS, not installed on the VAX
   (`%DCL-W-ACTIMAGE, error activating image BACKTRANS` /
   `-CLI-E-IMAGEFNF, image file not found
   SIMVAX$DUA0:[SYS0.SYSCOMMON.][SYSEXE]BACKTRANS.EXE;`, status
   `%X100388B2`, V02, V03); its syntax has `/OUTPUT` but not `/UPDATE`
   (round 1's C23) and no `/SUM` (V04); HELP EDIT has no `/SLP` (V01).
   `EDIT/SU` is EDIT/SUM (V05); `EDIT/S` is `%DCL-W-ABKEYW` (V06).
2. **G14's lost insert** (CLASH2): while a delete or replace of m to n
   is open, the text of inserts after lines in m to n is held, and
   written after the range's own text when the range ends (K03, K05,
   K07, G05); a second range starting inside the open one is dropped
   (A.4.3), and **dropping it discards the held inserts**, whatever their
   precedence or place in the sort (KACB, KBAC, KBCA, KCAB, K04, K06,
   K09, K09R, K10), but not the open range's own text (G06). K01's and
   K08's delete with a replace inside write only the delete.
3. **Locators past 32,767** (NUMBERS): a locator is read as a 16-bit
   word, its value modulo 65,536, signed: 65,536 is 0 (N04), 65,537 is 1
   (N05), 65,541 and 131,077 are 5 (N06, N08), and 32,768 to 65,535 are
   negative (N01 to N03, N07, Y05's 99,999). Six digits are fine (N13).
   A negative locator is out of sequence (EDOUTSEQ even as the file's
   first command); a negative insert goes after line 1 (N01, N09, N14),
   a negative range deletes nothing and inserts before line 1 (N10), and
   a range whose second locator is below its first is a syntax error
   (N12's 5,32768, O04).
4. **Past the end** (PASTEND): an insert after a line beyond the input's
   last is lost, silently (E08, E09's second, E11's second, E12, E15). If
   the input ends while a range is open, the range ends there and its
   text is written, and the input is read **again from its start**,
   numbered on (E01 to E07, E13, Y13, Y14); the lines read again are
   never deleted by the range that ran out, but a later edit reaches them
   by their new numbers (E10's 5,5 replaced the second reading's line 2).
   The input is read again once: the second end ends the run (E01). The
   deleted lines' count of a range that ran out is shown only by the next
   range to end normally (E10's `-3` on line 6, for 2, 3, and 5; none in
   E05). An empty input with `-1,1` writes the text (E14).
5. **Record lengths** (LENGTHS): records of up to 255 bytes, in an update
   file or the input (W255, X00255); 256 or more is `%SUM-E-READERR,
   error reading DUA1:[SUM117.IN]X00256.SRC;1` / `-RMS-W-RTB, 256 byte
   record too large for user's buffer`. An update file's is passed over,
   none of its edits applied, the status success (W256 to W513); at an
   input's the run **stops there**: the output and listing have what came
   before, and the status is RTB with the inhibit bit, `%X100181A8`
   (X00256 to X32767; *corrected in subtask 7*, which read the log again:
   this item said success for both). (RMS gave 16,385 and 32,768
   bytes for X16384's and X32767's records: their Stream_LF records as
   VMS counted them.) A listing line is cut at 255 bytes (W255's 279-byte
   line).
6. **Pages** (PAGES): message lines count toward a page's 57 (P01's
   clash message ended page 1); inserted runs and deletions cross pages
   with nothing more (P02, P03). Heading line 1 is 130 bytes: a form feed,
   60 blanks, the run's date and time, 4 blanks, `VAX-11 SUMSLP V01.00`,
   16 blanks, `Page`, the number right-aligned in 5. Line 2 is 129 bytes:
   60 blanks, the input's date and time, 4 blanks, the input's full name
   blank-padded or cut to 45 characters (P04).
7. **Dates** (DATES): line 2's date is the input's **creation** date, not
   its revision date (D01: created 14:52:06, revised 14:52:09, the
   heading 14:52:06). (D01's update file was looked for in `[SUM117.SCR]`,
   the input's directory, a slip in the case; the date question didn't
   need it.)

What stays unconfirmed: U03's audit trail (a `-,,/text/` marking an
insert made before it); how a range that runs out of input interacts
with several update files; a second reading's own end in the middle of a
range.

## Part B — decisions (proposed; adjusted by A.4)

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
   `/UPDATE` the file's alone (IVQLOC on the verb), `/LISTING`,
   `/OUTPUT`, and `/HEADER` either's (A.4.1), so its messages are DCL's.
   EDIT/SLP is govax's own synonym (on VMS 7.1 it runs another utility,
   BACKTRANS, which isn't there, A.5.1; the review's answer 1), in
   DEVIATIONS.md; `EDIT/S` stays ambiguous, as VMS's.
4. **The rules are fitted to VMS's output**, never written from VMS's
   code (clean room): each rule written in `doc.go` with the cases that
   showed it; what no case settles is a choice marked unconfirmed and
   listed in DEVIATIONS.md.
5. **The round trip is a test**: `diff.SLP` then `sumslp.Apply` gives the
   revision for every pair `TestProbeListings` reads and for seeded
   random pairs, in milliseconds, and the scripts VMS wrote (Phase 116's
   62 and this probe's 136) applied to their masters give their
   revisions.
6. **Messages** go into `vmsdef` as SUMSLP's facility, SUM, with the
   texts the probe showed; `$STATUS` is RMS's status with the inhibit bit
   (A.4.1), so the SUM messages' numbers never show and are govax's
   choice, unconfirmed.
8. **What VMS does with odd input is followed** where the probe makes a
   rule of it (A.4, A.5: the held inserts a dropped range discards, a
   range past the end reading the input again, locators modulo 65,536, a
   syntax error's numbered but unwritten line, the 255-byte records);
   what stays unclear is govax's choice, unconfirmed, in DEVIATIONS.md. SUMSLP's reading each
   update file twice is not copied: govax reads a terminal's update
   once (a deviation that only makes `/UPDATE=SYS$COMMAND` work).
7. **MACRO/UPDATE is in this phase** (the review's answer 2): a
   positional qualifier on each source, the update files merged by
   `sumslp` and the updated text handed to the assembler (`internal/asm`
   never sees the files), the listing's audit trail as VMS's, a missing
   update file an informational message and the assembly going on.

## Part C — subtasks

Each is one commit, with tests, stopping for review.

1. **The survey**: Part A, this plan.
2. **The probe** (done 2026-11-25: two rounds on VMS 7.1, their answers
   A.4 and A.5):
   the documentation CD searched for the SUMSLP manual (found, A.1);
   `testdata/sum117`, as in Part D, written, rehearsed on govax, and its
   exchange volume built; run on the VAX by the author (a short round 2
   may follow for what round 1's HELP shows that it didn't ask); A.3
   answered (as A.4) and Part B adjusted.
3. **The command's frame** (done 2026-11-25): the grammar (EDIT/SUM and EDIT/SLP beside
   plain EDIT), HELP EDIT's `/SUM` topic in VMS's shape, the messages and
   statuses in `vmsdef`, the names (input, updates, output, listing),
   the errors; held to the probe's COMMAND section.
4. **The engine, one update file** (done 2026-11-25): `internal/sumslp`'s
   `Parse` and `Apply`, every edit
   command and its errors; held to the BASICS and ORDER sections, and the
   round trip over Phase 116's pairs and VMS's 62 scripts. (The one pass
   that fits one file's cases fits several files' too: MERGE and CLASH2,
   the trail, and the listing's pages already match, below.)
5. **Merging** (done 2026-11-25): several update files, their order and
   conflicts; held to MERGE. (Held since subtask 4, engine and command; what
   was left was the unconfirmed cases: a range that runs out with several
   files.)
6. **Audit trail and listing** (done 2026-11-25): held to AUDIT and
   LISTING. (The engine's part held since subtask 4; left was `/HEADER`'s
   VFC output, U11 to U14.)
7. **Files and the round trip on VMS** (done 2026-11-25): record formats,
   every byte, long records; held to FORMATS and ROUNDTRIP.
8. **MACRO/UPDATE** (done 2026-11-25): the grammar's positional qualifier, `.UPD`'s
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
- **2026-11-25 — subtask 2, round 1 on VMS.** The author ran
  `sum117.com` and `sum117t.com` on VMS 7.1; the results came back with
  `copyout.cmd` (`vax/sum117.log`, `vax/terminal.log`, 627 files in
  `vax/out/`). Their answers are A.4. The round trip held for all 136
  pairs (compared on the host: the procedure's closing DIFFERENCES looked
  for the revision in `[SUM117.OUT]`, a slip). They found a Phase 116 bug:
  DIFFERENCES/SLP writes a `<` before a record that starts with an SLP
  operator, and govax's `diff.SLP` didn't; fixed (commit ae4b4e3d), and
  `internal/diff`'s `TestSum117Scripts` holds `diff.SLP` to all 136 of
  VMS's scripts. Round 2, `sum117b.com` (76 cases; `gen.go` writes it
  after round 1's cases, which don't change: `cases.txt` marks it with a
  `round 2` line), asks what A.4 left open; `exchange2.cmd` put its files
  on the exchange volume, and it was rehearsed on govax on a copy.
- **2026-11-25 — subtask 2 done: round 2 on VMS.** The author ran
  `sum117b.com`; `copyout2.cmd` brought back `vax/sum117b.log` and the
  outputs. Its answers are A.5: EDIT/SLP is VMS 7.1's BACKTRANS, not
  installed; the held inserts a dropped range discards (G14); locators
  modulo 65,536, signed; the input read again when a range runs past its
  end; 255-byte records; message lines on the page; the heading's widths
  and the input's creation date. Decisions 3 and 8 adjusted. Subtask 3
  is next.
- **2026-11-25 — subtask 3 done: the command's frame.** `console.dcl`'s
  EDIT has `/SUM` and `/SLP` switching to a syntax of their own,
  `edit_sum`: the input a `/single` `$file` (two files are VMS's NOLIST,
  C27), `/UPDATE` the input's alone (IVQLOC after the verb, C05), and
  `/LISTING`, `/OUTPUT`, `/HEADER` anywhere (C06, C07); a qualifier it
  lacks is IVQUAL (C25, C26). `internal/dclcmd/sumslp.go` is the command:
  the names (the first update file the input's device, directory, and
  name with `.UPD`, each later one the device and directory of the one
  before, C04, C16, C28; `/UPDATE=SYS$INPUT` the procedure's data lines to
  the `/`, read once, Decision 8; the output beside the input, a new
  version; the listing only with `/LISTING`, the input's name and `.LIS`),
  `%SUM-I-OPENIN` for a file that can't be opened (the input's ending the
  command with RMS's status, an update file's passed over), `%SUM-E-OPENIN`
  and WLD for a wildcard, `$STATUS` RMS's with the inhibit bit, and a
  missing file DCL's INSFPRM. DIFFERENCES' OPENIN analysis is shared
  (`Context.openFailureOf`). The SUM messages' texts are
  `vmsdef/summessages.go`, with no numbers (none ever shows).
  `internal/sumslp` is begun: `Apply` (no edits yet: the output is the
  input, the listing numbers its lines) and `Listing`, the page layout,
  held byte for byte to C03's, L01's, and L02's listings (an empty input's
  listing has no records: a page's heading goes out with its first line,
  where A.4.6 guessed the heading alone). HELP EDIT has `/SUM` in VMS's
  shape and `/SLP`. DCL's ABKEYW for an ambiguous qualifier (EDIT/S, V06)
  replaces govax's own AMBIGUOUS message, as IVQUAL replaced UNRECOGNIZED.
  `internal/console`'s `sum117_test.go` makes the probe's volume and runs
  its cases as SUM117.COM does; `TestSum117CommandOracle` holds the
  COMMAND section's logs, statuses, and files made to VMS's, and the
  files' records where no edit applies (C03, C18, C24's absent listing);
  the others' records wait for subtask 4 (`sum117Later`). Not compared:
  C23 (EDIT/SLP, govax's own), C28's records (a directory file's), C29's
  and C30's logs (VMS reads SYS$INPUT twice). Found on the way, for a
  later fix: govax's DELETE leaves `$STATUS` `%X00030001`, VMS's
  `%X10000001` (C31, masked by `sum117Known`). Unconfirmed: the output's
  directory (the input's) and the listing's (the default), every case
  having run in the input's directory; a day before the 10th in the
  heading shown with a blank. `/HEADER` is accepted and does nothing until
  subtask 6. Subtask 4 is next.
- **2026-11-25 — subtask 4 done: the engine.** `internal/sumslp`'s
  `parse.go` reads an update file into `Edit`s (each command's locators,
  modulo 65,536 and signed; "."; the trail; a bad command SLPSYNERR, a
  numbered but unwritten line of the edit before it; PRMEOF; EDOUTSEQ; an
  insert with no text no edit), and `sumslp.go`'s `Apply` is the one pass:
  every file's edits sorted by locator1, the higher precedence first among
  equals; an insert written after its line, at the current place once the
  pass is past it, lost past the end; a range's lines deleted as it opens
  and its text, then the inserts it held, written when it closes; a range
  starting in the open one dropped, discarding every insert that one holds
  (G14R showed it is all of them, even those sorted after the dropped
  range); a range the input ends in closing there and the input read again,
  numbered on, once; EDITSCLSH a message per group of edits that meet,
  before the group's first line; the "-n" marks and ".n" runs; the trail
  the file's last. `doc.go` has each rule and the cases that showed it, and
  the choices no case settled ("." after a replace is its locator2; a range
  starting two past the end is lost; EDOUTSEQ of a dropped or held edit
  listed when the pass reaches it; PRMEOF unnumbered; a range open at the
  second end closes there), for DEVIATIONS.md at the close-out.
  `probe_test.go` replays the probe's single EDIT/SUM cases on the engine,
  output and listing byte for byte: BASICS, SYNTAX, ORDER, NUMBERS, and
  PASTEND (`TestProbeOneFile`), and, as the same pass already fits them,
  MERGE and CLASH2 (`TestProbeMerged`) and AUDIT, LISTING, and PAGES
  (`TestProbeTrailsAndPages`), all of them VMS's. `roundtrip_test.go`
  applies every script VMS's DIFFERENCES/SLP wrote in both probes to its
  master (`TestVMSScriptsRoundTrip`, R14's records holding line feeds left
  to the next test) and `diff.SLP`'s scripts for the same pairs and 3,000
  seeded random ones (`TestGovaxScriptsRoundTrip`): each gives its revision,
  with no message. The command: an update record past 255 bytes is
  `%SUM-E-READERR` with RMS's RTB and the file's edits skipped (B30), and
  with no listing the warnings go to SYS$OUTPUT. `internal/console`'s
  `TestSum117CommandOracle` now compares every COMMAND case's records
  (`sum117Later` is empty), and `TestSum117EditsOracle` and
  `TestSum117MergeOracle` hold the five one-file sections and MERGE and
  CLASH2, run by EDIT/SUM, to VMS's logs and files. Found on the way: the
  tests' reading of cases.txt's `=lines` rule took a one-word prefix
  (L03's and L04's "listing record"), fixed in both readers. Subtask 5
  is next; with its sections already held, it is small.
- **2026-11-25 — subtask 5 done: merging.** The MERGE and CLASH2 sections
  were held by subtask 4's pass (`TestProbeMerged`, and through EDIT/SUM
  `TestSum117MergeOracle`). Added: `merge_test.go`'s `TestSplitScriptsMerge`
  deals the edits of each DIFFERENCES/SLP script (both probes' pairs and
  2,000 random ones) among two or three update files at random; merged, they
  give the revision with no message, so edits that don't meet merge into one
  file's result whatever files they are in. `TestMergeChoices` pins govax's
  choices where no case had one, a range that runs out with another file's
  edits after it: an insert or a range of the other file reaches the second
  reading by its numbers, a range starting inside the one that ran out is
  dropped, and a range open at the second end closes there (`doc.go`; for
  DEVIATIONS.md at the close-out). The random pairs' maker is shared,
  `randomPair`. No engine change was needed. Subtask 6 is next.
- **2026-11-25 — subtask 6 done: the audit trail and the listing.** The
  engine's trails, numbers, markers, and pages were held by subtask 4's
  pass; what was left was `/HEADER`. The probe's host copies of U11 to U14
  had lost their fixed control (copyout.cmd's COPY to the host writes text),
  so the records were read from the exchange volume's blocks with govax's
  DUMP/BLOCK and kept as `testdata/sum117/vax/headers.txt`. They showed what
  A.4.7 had missed: an input line's second word is the deletion count its
  `-n` shows. `sumslp.Result` now has a `Head` per output record and
  `Headed()`, the records with their fixed control (`TestHeadedAsVMS`
  holds all four files to VMS's byte for byte), and EDIT/SUM/HEADER writes
  them through RMS's new `Session.CreateVFCFile` (VFC, the given fixed
  control, CR carriage control, the longest record counting data alone, no
  maximum, as VMS's DIRECTORY/FULL showed; a host file gets the data as
  lines; `createVolumeRecordsAllocated` now takes the attributes, not a
  kind). `internal/console`'s `TestSum117AuditOracle` holds the AUDIT,
  LISTING, and PAGES sections, run by EDIT/SUM, to VMS's logs and files,
  U11D's DUMP/RECORD and DIRECTORY/FULL included. The oracles now mask file
  IDs (the volume's own) and skip a procedure line's `-` continuations
  (P04's), and DUMP's success status joins DELETE's among the known DCL
  differences (`dclSuccessStatus`). Subtask 7 is next.
- **2026-11-25 — subtask 7 done: files and the round trip.**
  `internal/console`'s `TestSum117FilesOracle` holds the FORMATS, LENGTHS,
  DATES, and ROUNDTRIP sections, run by EDIT/SUM, to VMS's logs and files:
  all 136 round trips (DIFFERENCES/SLP, EDIT/SUM of its script, DIFFERENCES
  of the result) and every update record's length held as they stood. What
  was missing was an input record past 255 bytes (X00256 to X32767): the
  pass now stops where it reaches one (`sumslp`'s `applier.read`,
  `Result.Long`), the output and listing holding what came before, and the
  command reports `%SUM-E-READERR` with RMS's RTB, which is `$STATUS` with the
  inhibit bit (A.5.5 had said success; corrected). `sumslp`'s
  `TestProbeFormatsAndLengths` holds the same sections on the engine (F03's
  host copy, a record holding a line feed, is the console oracle's, by
  DUMP/RECORD); `doc.go` has a "Records" section. Found on the way and fixed:
  VMS 7.1's DUMP shows the DEC Multinational graphic characters, `%XA1` to
  `%XFE`, as themselves in its text column, where govax showed dots
  (`dclcmd/dump.go`'s `dumpPrintable`; F03D); `testdata/for81`'s
  `vax/for81-blocks.txt` and `for81c-blocks.txt`, made by govax's DUMP on
  the exchange volume, had their text columns recomputed from their hex by
  the new rule. For a later look: D01's input,
  a Stream_LF file with no carriage control copied by COPY and appended to by
  OPEN/APPEND, has carriage return carriage control on VMS and none on govax
  (one of the two commands; masked by `sum117Known`). The oracles' file ID
  mask now takes the blanks after an ID, whose number its digits decide.
  Before the subtask, origin's EDIT/READONLY commits were merged (the help
  library regenerated from the merged `.HLP` files); EDIT/SUM still refuses
  `/READ_ONLY` (C25). Subtask 8, MACRO/UPDATE, is next.
- **2026-11-25 — subtask 8 done: MACRO/UPDATE.** MACRO's grammar has
  `/UPDATE[=(file,...)]` as a positional qualifier on each source, and the
  handler makes one after the verb VMS's IVQLOC (M11); `/LIST` and
  `/OBJECT` are positional too, as VMS's are, so `MACRO A/LIST=A1,B/LIST=B1`
  makes two listings (M04). `dclcmd/macroupdate.go` runs `sumslp` on each
  file with `/UPDATE` before the assembler reads it, the update files named
  by EDIT/SUM's `sumUpdates` (the source's name and `.UPD`, a missing one
  `%SUM-I-OPENIN` and the assembly going on, M07, M16), SUMSLP's warnings to
  the terminal, and turns SUMSLP's listing lines into an `asm.Update`
  (`sumUpdate`). `internal/asm/update.go` is the listing's side
  (`SetUpdate`; `listLine.update`): SUMSLP's numbers in the line number
  column, its 16-column field (trail or `-n`) over the binary field's first
  columns on the line's first listing line and the binary field 20 columns
  wide on all of them (`layoutRows` now takes a width), a SUMSLP warning
  listed before its line as two empty lines, its lines, and the location
  line, and counted as a warning on the input's last line, and an inserted
  line in the summary's list as `6.1` (`0.2` before the first). Held by
  `internal/console`'s `TestSum117MacroOracle`: every MACRO case's log (but
  M06's and M13's, whose assembly errors govax shows in its own form, not
  VMS's listing line), files, listing source pages, summary and lines
  read, and objects record for record (M14's `/DEBUG` line table too), all
  VMS's; and by `asm`'s `TestListingUpdated`. Found and fixed on the way:
  (1) a positional qualifier's value in a `/joinable` list now ends at a
  `+`, as a bare element does (`dcl`'s `splitJoinedValue`, M05's
  `SUMP/UPDATE=M05+SUMA`; `TestPositionalJoinedValue`); (2) NOTENUFOPR's
  `!` is past an immediate operand's `#` (M06's `MOVL #1`, beside
  errors.lis's `MOVL R0`); (3) the audit trail is the edit's, not the
  file's last: M10's two inserts show `;M10 FIRST` and `;M10 SECOND`, and an
  edit before any command gave a trail takes the file's first (U03), so
  `sumslp.Edit.Trail` replaces the file's one trail, for EDIT/SUM too
  (`TestTrailPerEdit`; every EDIT/SUM case still VMS's). Unconfirmed,
  for DEVIATIONS.md at the close-out: a source none of whose update files
  opened is assembled with a plain listing (M07's lines fit either way); a
  macro expansion under an updated line keeps the full binary field; a
  warning after a file's last line is listed after it; an input record past
  255 bytes is READERR and the lines before it are assembled. HELP MACRO
  has `/UPDATE` (VMS 7.1's has none, H04). Subtask 9, the close-out, is
  next.
