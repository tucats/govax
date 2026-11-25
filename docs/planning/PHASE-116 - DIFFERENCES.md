# Phase 116 — DIFFERENCES

**Status:** planned 2026-11-25 (subtask 1, the survey and this plan);
the plan reviewed and accepted by the author the same day, with the
answers recorded in "The review". This document is the development plan; as
the work goes on it becomes the implementation record: the progress log,
and at the end "Where things stand".

## Goal

govax gets VMS's DIFFERENCES command: two record files compared record
by record, and the records that don't match listed in VMS's forms, with
VMS's matching rule, qualifiers, messages, and exit statuses, so that a
listing govax prints is the listing VMS 7.1 prints for the same files.

The author's session on the VAX, the starting point:

    $ diff foo.txt foo.tst
    ************
    File SYS$SYSROOT:[SYSMGR]FOO.TXT;10
        3   line 3
    ******
    File SYS$SYSROOT:[SYSMGR]FOO.TST;1
        3   extra cat
        4   extra dog
        5   line 3
    ************

    Number of difference sections found: 1
    Number of difference records found: 2

    DIFFERENCES /IGNORE=()/MERGED=1-
        SYS$SYSROOT:[SYSMGR]FOO.TXT;10-
        SYS$SYSROOT:[SYSMGR]FOO.TST;1

FOO.TXT is `line 1` to `line 4`; FOO.TST has `extra cat` and `extra dog`
between lines 2 and 3. Already this shows things the manual doesn't
say: an insertion's section in the master lists no unmatched record,
only the one matched record `/MERGED=1` puts after it; the revision's
lists its two inserted records and that same matched record; the count
of difference records is 2 (the larger side's unmatched records, not
both sides'); the line number is right-aligned in five columns and the
text starts at column 9 (a tab, or three spaces: the probe must read the
bytes); the closing echo of the command shows the defaults it used, and
breaks after each file with a `-`.

## Part A — the survey (2026-11-25)

### A.1 What VMS has

The reference is the *OpenVMS DCL Dictionary* (7.3-1), DCLI–170 to 179
(`~/Documents/Technical Doc/VMS/OVMS_731_dcl_dictionary-a-m.txt`, line
8072 on). Its examples' spacing was lost in the text; the PDF beside it
keeps the layout and is to be read for them.

**Format.** `DIFFERENCES input1-filespec [input2-filespec]`. No
wildcards in either. The second's missing fields come from the first; a
missing second file is the next lower version of the first.

**The comparison.** Record by record. After a difference, the files are
in step again only when `/MATCH=n` (default 3) records in a row are
identical in both. `/WINDOW=n` bounds the search for a match (default:
to the ends of both files); the manual calls the window "the minimum
size of a differences section that will cause DIFFERENCES to lose
synchronization". `/MAXIMUM_DIFFERENCES=n` stops after n unmatched
records, counted as the sum over sections of each section's larger
side; it then gives at most one listing form and a warning.

**What is ignored.** `/IGNORE=(keyword,...)`: BLANK_LINES, CASE,
COMMENTS, FORM_FEEDS, HEADER[=n] (default 2), SPACING,
TRAILING_SPACES; and how ignored text appears: EDITED (the default:
left out), EXACT (kept), PRETTY (formatted: tabs expanded, `<CR>`,
`<LF>`, `<VT>`, `<FF>`, other nonprinting characters a period).
COMMENTS, FORM_FEEDS, HEADER, and SPACING are applied before
TRAILING_SPACES, then BLANK_LINES. `/COMMENT_DELIMITER[=(c,...)]` (up
to 32 characters, or the keywords COLON, COMMA, EXCLAMATION, FORM_FEED,
LEFT, RIGHT, SEMI_COLON, SLASH, SPACE, TAB) implies
`/IGNORE=COMMENTS`; without a character, the file type chooses (a table
by type; .COB and .FOR's only in column 1). The manual's table lost its
columns in the OCR; the PDF has it.

**The listing forms.** Any of these together, but `/SLP` alone:

- `/MERGED[=n]` (the default form, n = 1): each section's master
  records, then `******`, then its revision records, each followed by n
  matched records; n ≤ `/MATCH`.
- `/SEPARATED[=(MASTER,REVISION)]`: each file's unmatched records alone.
- `/PARALLEL[=n]`: side by side (default n = 0); always formatted
  (PRETTY); ASCII only.
- `/MODE=(ASCII,HEXADECIMAL,OCTAL)`: each radix a listing of its own;
  hexadecimal and octal dump each record with a `RECORD NUMBER`
  header, `***CHANGE***` marking a changed one.
- `/CHANGE_BAR[=(c,[NO]NUMBER)]`: a positional qualifier: on the verb,
  the revision's (the second) listing; on a file, that file's. Default
  `!`. The whole file is listed, changed records marked.
- `/[NO]NUMBER` (default NUMBER), `/WIDTH=n` (default 132; at a
  terminal, the terminal's width).
- `/SLP`: an edit script for the SLP editor that makes the master the
  revision; its output file defaults to the master's name, type .DIF.
- `/OUTPUT[=file]`: default SYS$OUTPUT; with no file, the first input's
  name, type .DIF.

**Statuses.** SUCCESS: identical. INFORMATIONAL: different. WARNING:
`/MAXIMUM_DIFFERENCES` reached. ERROR: no virtual memory.

**7.3-1's paging.** `/PAGE[=CLEAR_SCREEN|SCROLL|SAVE[=n]]`, `/SEARCH`,
`/EXACT`, `/HIGHLIGHT`, `/WRAP` are the newer screen utilities' shared
qualifiers. They may not be in VMS 7.1; the probe asks.

### A.2 What govax has today

No DIFFERENCES: no verb in `console.dcl`, no help topic, no `DIF`
messages in `vmsdef`. What it would stand on:

- **Reading the inputs**: `rms.Session.ReadRecordFile` (host or volume
  file, by `Locate`'s rules, through the checked access layer of Phase
  93), as TYPE does (`dclcmd/type.go`, whose `typeFailure` gives VMS's
  messages for a file it can't open). The next lower version needs a
  search by version (`$SEARCH` on `name.type;*`, or `rms`'s directory
  walk), as PURGE's code does.
- **Names shown**: `resolvedSpec.shownFile` (Phase 91), so a file under
  a concealed root shows as `SYS$SYSROOT:[SYSMGR]FOO.TXT;10`, as the
  author's log has it.
- **The output file**: `Session.LocateOutput` and `CreateRecordFile`, as
  DIRECTORY/OUTPUT (`dclcmd/directory.go`'s `DirectoryTo`) and CONVERT
  do; default type by `TemporaryDefaults`.
- **The terminal's width**: SET TERMINAL/WIDTH (`dclcmd/terminal.go`)
  keeps it; DIFFERENCES needs to know whether SYS$OUTPUT is the
  terminal.
- **A positional qualifier**: `internal/dcl`'s parameter-scoped
  qualifiers (COPY's `/HOST`, Phase 23) are what `/CHANGE_BAR` on a file
  needs; the verb's own `/CHANGE_BAR` beside them is a qualifier of the
  same name in both places, which the grammar must allow.
- **Paging**: TYPE/PAGE's `dclcmd/page.go`.
- **Statuses**: `SetStatus` with a DIF condition value; `$STATUS` and
  `F$MESSAGE` need the messages in `vmsdef`.

### A.3 What the references leave open

The probe (Part D) is to answer each:

1. **The layout, to the byte**: the rows of asterisks; blank lines; the
   line number's width and what follows it (tab or spaces); `File` lines;
   a record wider than `/WIDTH` (cut, or wrapped, and where); the
   summary lines and the closing echo (which qualifiers it shows, in what
   order and form, where it breaks a long line, the `-` and the indent).
2. **The matching rule**: what is a section when records repeat, when a
   block moves, when a change and an insertion are next to each other,
   at the start and end of a file, when one file is empty or both are;
   `/MATCH=1`, `/MATCH` larger than what's left; `/WINDOW`'s exact
   effect. The manual's one sentence won't fit every case: a few hundred
   generated pairs (seeded, so govax's tests can make them again) let
   the algorithm be fitted to VMS's answers rather than guessed.
3. **Counting**: "difference records" per section (the larger side? the
   master's?), and `/MAXIMUM_DIFFERENCES`'s stop.
4. **Identical files**: what is printed, and `$STATUS`.
5. **Each form's layout**: `/SEPARATED` (each keyword), `/PARALLEL[=n]`
   at widths 80 and 132 and with long records, `/MODE=HEX` and `OCTAL`
   (the dump's columns), `/CHANGE_BAR` in each placement with and
   without NUMBER, `/NONUMBER`, `/SLP`'s script, and several forms in
   one command (their order).
6. **Ignoring**: each `/IGNORE` keyword alone and in the manual's
   combination, EDITED/EXACT/PRETTY, HEADER's form-feed rule, comment
   delimiters given and by each file type, and what the echo shows.
7. **Names and errors**: one parameter (the next lower version; none
   lower), a missing file, a wildcard, a directory default, a bad
   qualifier value, `/MERGED` greater than `/MATCH`, `/SLP` with another
   form; the messages' facility, texts, and `$STATUS` values (and
   whether `F$MESSAGE` knows them).
8. **The output file**: its name with and without a value, its record
   format and attributes (DIRECTORY/FULL), its width when not a terminal.
9. **Records that aren't text**: FIX, VFC, STREAM_LF, and binary bytes
   (tabs, control characters, 8-bit) in each form.
10. **The terminal**: SYS$OUTPUT at a terminal of width 80 and 132,
    `/WIDTH` ignored there; and whether VMS 7.1 has `/PAGE` and its
    companions.

### A.4 The probe's answers (2026-11-25)

Two rounds on VMS 7.1 (`testdata/dif116/vax`: `dif116.log` round 1 to
case N59, `dif116b.log` round 2 from N60, `terminal.log`, 796 listings
in `out/`). What they settle, by A.3's question; the details each
subtask needs are in the listings, which its oracle holds govax to.

1. **Layout.** Spaces, never tabs. A section is a row of 12 asterisks,
   `File` and the master's full name with version, its records, 6
   asterisks, the revision's, 12 asterisks; sections follow one another
   with no blank line. A record's line is its number right-aligned in
   five columns (wider when it needs: `100005`), three blanks, the text,
   cut (never wrapped) at `/WIDTH`. Then a blank line, `Number of
   difference sections found: n`, `Number of difference records found:
   n`, a blank line, and the echo: `DIFFERENCES ` and the qualifiers in a
   fixed order (IGNORE always, as `/IGNORE=()` when empty; then
   COMMENT_DELIMITERS, WIDTH, MATCH, MAXIMUM_DIFFERENCES, MERGED, MODE,
   OUTPUT with the listing's full name, PARALLEL, SEPARATED, WINDOW,
   NONUMBER), each written only if given or a default the form uses
   (`/MERGED=1`); a line broken with `-` before the width, continued
   with four blanks; then each file's full name on its own line, with its
   change bar (`/CHANGE_BAR=("#")`), `-` after all but the last.
   `/IGNORE`'s keywords come in a fixed order (SPACING, TRAILING_SPACES,
   HEADER=n, FORM_FEEDS, CASE, EXACT, COMMENTS, BLANK_LINES), and a
   comment delimiter is shown as its keyword (`EXCLAMATION`, `SLASH`) or
   quoted (`";"`, `"C"`).
2. **Matching.** A section takes in matched records between two
   differences until `/MATCH` of them run together (A16, A17: one
   section; A18: two). The listing's records are the section's, then
   `/MERGED`'s n matched records after it; an insertion's master side
   (or a deletion's revision side) lists only those. Which records a
   section claims when records repeat (A23, the generated pairs) is not
   the longest common subsequence's choice: subtask 4 fitted the rule to
   the generated runs (`internal/diff`'s `doc.go`, "The search": a
   square search, the larger side minimized, the master's place first;
   /WINDOW a square of that depth, moved on a record at a time).
3. **Counting.** A section's count is its larger side's records, the
   matched ones inside it included (A16: 3); the total is their sum.
   `/MAXIMUM_DIFFERENCES` stops with `%DIF-W-MAXDIF, maximum number of
   difference records encountered - DIF aborted` and status `%X006C8018`;
   `/MAXIMUM_DIFFERENCES=0` is no limit.
4. **Identical files** (and two empty ones): no section, the two counts
   of 0, and the echo; status `%X006C8009`. Different files:
   `%X006C8013`.
5. **The forms.** `/SEPARATED` lists one file's section records under one
   section header. `/PARALLEL` is a rule of width less one, the two
   names, and per section a rule with each side's first record number, a
   `|` between columns; no line numbers. `/MODE=HEX` and `OCTAL` list
   each record as `RECORD NUMBER n (hex) LENGTH n (hex or octal)` and a
   DUMP-style line per 16 bytes. `/CHANGE_BAR` lists the whole file, a
   changed record marked after its number (a master's bar also marks the
   matched record that follows an insertion), one listing per file with
   a bar. `/SLP` is `-    5,    5` (the master's records replaced),
   `-   15` (insert after 15), the new records, and `/` at the end.
6. **Ignoring.** Each file type's default comment characters are the
   manual's table (CBL, CMD: `!` and `;`; COB: `*` and `/`; FOR: `!`, C,
   D, c, d; MAC, MAR: `;`; others of the table `!`; any other type
   none). How each keyword and EDITED, EXACT, PRETTY change a listing is
   subtask 7's, from cases I01 to I42.
7. **Names and errors.** The prompts are `_File 1: ` and `_File 2: `;
   in a procedure a missing parameter is `%DCL-W-INSFPRM`. One file is
   its next lower version, `%DIF-F-NOLOWVER, no lower version of file
   name found` (`%X106C8064`) when there's none. A missing file is
   `%DIFF-F-OPENIN, error opening name as input` with RMS's reason
   (FNF, DNF, DEV, NOSUCHFILE); a wildcard in the first is
   `%DIFF-F-NOWILD`, in the second an OPENIN; no type opens `name.`.
   `/CHANGE_BAR` is only a parameter's (`%DCL-W-IVQLOC` on the verb);
   `/SLP` with `/MERGED`, `/PARALLEL`, `/SEPARATED`, or `/MODE` is
   `%DCL-W-CONFLICT`; `/MERGED` or `/PARALLEL` above `/MATCH` is a
   warning (`%DIF-W-MRGGTMTCH`, `%DIF-W-PARGTMTCH`) and goes on;
   `/WINDOW=0` is `%DIFF-F-BADVALUE, '0' is an invalid keyword value`; a
   comment delimiter of two characters `%DIF-F-CMDSYNTAX`, 33 of them
   `%DIF-F-TOOMNYDLM`. The facility is 108 (`%X6C`); messages of its own
   show as `DIF`, the shared ones (OPENIN, NOWILD, BADVALUE) as `DIFF`;
   F$MESSAGE knows none of them.
8. **The output file.** Variable records, carriage-return carriage
   control. `/OUTPUT` with no file, a directory, or a name with no type,
   and `/SLP` without `/OUTPUT`, make `name.DIF` in the default
   directory (the first input's name where none is given).
9. **Record formats.** VFC, STREAM_LF, and fixed files compare by their
   records' text (R01 to R07); the listings of every byte are R10 to R15.
10. **The terminal.** At a terminal the listing is the terminal's width,
    `/WIDTH` ignored (`terminal.log`). VMS 7.1 takes `/PAGE`; `/SEARCH`,
    `/WRAP`, `/HIGHLIGHT`, and `/EXACT` without `/PAGE=SAVE` are
    `%DCL-W-CONFLICT` (`\NOPAGE\`).

## Part B — decisions (proposed)

1. **VMS 7.1 is the reference**, as for every probe since Phase 55: what
   7.3-1's dictionary adds that 7.1 refuses is left out. `/PAGE` and its
   companions (`/SEARCH`, `/EXACT`, `/HIGHLIGHT`, `/WRAP`) are left out
   whatever 7.1 has, by the author's choice (the review's answer 1); the
   probe still asks whether 7.1 has them, for DEVIATIONS.md.
2. **The engine is a leaf package**, `internal/diff`: records in (bytes,
   with their numbers), options in, a `Result` of sections out, and the
   listing forms as functions from a `Result` to lines. Plain Go, knowing
   nothing of RMS or DCL, so its tests are fast and run every probe pair.
   `internal/dclcmd/differences.go` is the command: names, files,
   output, status.
3. **The algorithm is fitted to VMS's output**, never written from VMS's
   code (clean room). Its first form is the manual's: walk both files;
   at a mismatch, look for the nearest place where `/MATCH` records agree
   (within `/WINDOW` if given), the section being what lies before it.
   "Nearest" is the question the generated pairs settle; each rule the
   fit needs is written in `doc.go` with the cases that showed it.
4. **Inputs through RMS's checked layer**, host or volume, as TYPE's;
   the output through `LocateOutput`, its record format what the probe
   shows (VAR with CR carriage control expected).
5. **Messages and statuses** go into `vmsdef` as a facility of their
   own, 108 (`vmsdef/difmessages.go`), the numbers from the probe's
   `$STATUS` values and the texts from its log, as JBC's were in Phase 98:
   its own messages shown as `DIF`, the shared ones as `DIFF` (A.4.7).
   VMS's F$MESSAGE knows none of them; govax's will, a harmless
   deviation, unless the author prefers VMS's NOMSG.
6. **`/SLP` is written** though govax has no SLP editor: it is only an
   output form, and a script made on govax should work on VMS (the
   review's answer 2).
7. **The grammar**: a verb `DIFFERENCES` (abbreviable to `DIF`, as VMS's)
   with two `$file` parameters prompting `_File 1:` and `_File 2:`,
   `/CHANGE_BAR` on each parameter only (A.4.7), and `/SLP`'s conflicts
   as the grammar's DISALLOW, so the messages are DCL's, as VMS's are.
   The paging qualifiers are IVQUAL on govax (Decision 1), a deviation.

## Part C — subtasks

Each is one commit, with tests, stopping for review.

1. **The survey**: Part A, this plan.
2. **The probe** (done 2026-11-25, two rounds on the VAX): `testdata/dif116`, as in Part D, written and its exchange
   volume built; run on the VAX by the author; A.3 answered (as A.4) and
   Part B adjusted.
3. **The command's frame** (done 2026-11-25): the grammar, HELP DIFFERENCES in VMS's
   shape, the messages and statuses in `vmsdef`, names (the second's
   defaults, the next lower version), the errors, identical files, the
   output file; held to the probe's NAMES section.
4. **The engine and the merged form** (done 2026-11-25): `internal/diff`'s matching,
   `/MATCH`, `/WINDOW`, `/MAXIMUM_DIFFERENCES`, the merged listing, the
   summary, and the closing echo; held to every generated pair.
5. **The other forms** (done 2026-11-25): `/SEPARATED`, `/PARALLEL`, `/CHANGE_BAR`,
   `/NONUMBER`, `/WIDTH`, several forms together.
6. **Radix and SLP** (done 2026-11-25): `/MODE=HEX` and `OCTAL`, `/SLP`.
7. **Ignoring**: `/IGNORE`'s keywords, EDITED/EXACT/PRETTY,
   `/COMMENT_DELIMITER` and the defaults by file type.
8. **The terminal and the rest**: the terminal's width, records that
   aren't text.
9. **The close-out**: MANUAL.md (section 6's file commands), HELP,
   VAX-VMS-INTERNALS.md if a mechanism changed, DEVIATIONS.md (what
   stays unconfirmed), PLAN.md, CLAUDE.md.

## Part D — the probe (subtask 2)

`testdata/dif116/`, on an exchange volume as the earlier probes were
(`exchange.cmd`, `copyout.cmd`, an RD53 in `testdata/disks`, attached to
the simh VAX). `gen.go` writes every file but the README:

- **The pairs**: `[DIF116.IN]` holds each case's files, made by
  `gen.go` (A.3's hand-written cases, and 270 generated pairs from a
  fixed seed: 240 short files of four or twelve distinct lines, so
  repeats and near-matches are common, and 30 longer ones, with
  insertions, deletions, changes, moves, and duplicates). `cases.txt` is
  the whole probe, every file's records and every case, the one record
  govax's oracle tests read; `in/`, the host copies, is made from it and
  gitignored.
- **`dif116.com`**: SET VERIFY, then a section per A.3 question; each
  case runs DIFFERENCES with `/OUTPUT=[DIF116.OUT]Cnnn.DIF` and writes
  `$STATUS` and `F$MESSAGE($STATUS)` to the log; the error cases run at
  the log itself, so their messages show. `/OUTPUT` keeps the bytes: tabs
  stay tabs when the files come back.
- **`dif116t.com`**: the terminal cases, run by hand at an 80- and a
  132-column terminal, whose session log the author keeps (A.3.10).
- **The other record formats**: VFC, STREAM_LF, and fixed files made
  by `exchange.cmd`'s COPY `/RECORD_FORMAT` (Phase 86), so govax's tests
  can make them the same way; the fixed ones copied as binary, so a
  record may hold CR and LF (a text copy drops a CR before a LF).

`copyout.cmd` brings back the log, every `.DIF` file, and a
DIRECTORY/FULL of the outputs. The oracle test (subtask 4 on) reads the
case list from `gen.go`'s output, runs each case on govax, and compares
the `.DIF` files byte for byte and the log's `$STATUS` lines.

## The review (2026-11-25)

The author's answers to the plan's three questions:

1. **`/PAGE`**: not needed at this time; left out with its companions
   (Decision 1).
2. **`/SLP`**: yes, written, to match VMS's behavior (Decision 6).
3. **The number**: 116 is right.

## Progress log

- **2026-11-25 — subtask 1, the survey.** The DCL Dictionary's entry
  read; govax's pieces found; the plan and the probe's design written.
  The author accepted it the same day ("The review").
- **2026-11-25 — subtask 2, the probe, written.** `testdata/dif116`:
  `gen.go` writes `cases.txt` (659 input files, 859 cases in nine
  sections: LAYOUT, MATCHING, FORMS, IGNORING, LIMITS, NAMES, OUTPUT,
  FORMATS, GENERATED), `dif116.com`, `dif116t.com` (the terminal and the
  paging qualifiers, by hand; DIFFERENCES with no parameters moved here,
  since its prompts would stop the logged run), `exchange.cmd`, and
  `copyout.cmd`. The exchange volume built on govax, its records checked
  with DUMP/RECORD, and `dif116.com` rehearsed to its end (every
  DIFFERENCES an unknown verb for now). The README has the VAX steps.
  Noticed on the way: govax has no SHOW TIME (the procedure writes
  `F$TIME()` instead), not this phase's to add.
- **2026-11-25 — subtask 2, round 1 on the VAX.** The author ran
  `dif116.com` and `dif116t.com` on VMS 7.1; `copyout.cmd` brought back
  `vax/dif116.log`, `vax/out/` (207 listings), and the author pasted
  `vax/terminal.log`. The procedure stopped at case N60, a 285-byte line
  DCL couldn't read (`%RMS-W-RTB`): 260 of 859 cases ran. Its status
  lines print `F$FAO("!XL")` of the string `$STATUS`, the string's
  address on VMS; on govax the same line printed the value, a small
  deviation to log at the close-out. First answers, to be written up as
  A.4 after round 2: listings use spaces, not tabs (`"    3   line 3"`);
  the closing echo includes `/OUTPUT=` with the listing's full name;
  identical files print counts of 0 and the echo, status `%X006C8009`
  (DIFF-S), different ones `%X006C8013` (DIFF-I); `F$MESSAGE` doesn't
  know the DIFF facility's messages; VMS 7.1 takes `/PAGE` and
  `/PAGE=SCROLL`, and refuses `/SEARCH`, `/WRAP`, `/HIGHLIGHT`, and
  `/EXACT` alone (`%DCL-W-CONFLICT`); DIFFERENCES with no parameters in a
  procedure is `%DCL-W-INSFPRM`. Round 2 (`dif116b.com`, `exchange2.cmd`,
  `copyout2.cmd`; long lines broken after a blank too) runs the cases from
  N60 on, and the prompts at the `$` prompt by hand.
- **2026-11-25 — subtask 2 done: round 2 and A.4.** The author ran round
  2 and the prompts by hand; `copyout2.cmd` brought back `dif116b.log`
  (599 cases to `DIF116B FINISHED`) and the rest of the listings, 796 in
  all. Twelve file cases made none, every one a `/CHANGE_BAR` on the
  verb, which VMS refuses (IVQLOC). A.4 is the answers; Decisions 5 and 7
  are adjusted to them.
- **2026-11-25 — subtask 3 done: the command's frame.** DIFFERENCES runs:
  `internal/dclcmd/differences.go` (the command), `internal/console/differences.go`
  (its binding), `internal/diff` (the engine), and HELP DIFFERENCES
  (`bootdata/files/help/differences.hlp`, with a Restrictions topic for what
  the later subtasks add). `TestDif116NamesOracle` (`internal/console/dif116_test.go`)
  makes the probe's volume from `cases.txt` as `exchange.cmd` does, runs the
  NAMES and OUTPUT cases as DIF116.COM runs them, and holds every case's
  messages, status, and listing to VMS's: all but N29 (two directory files,
  whose records are the volume's own) and O20 (DIRECTORY/FULL's file IDs and
  times), with N36, N62, and N64's listings left to subtasks 5 and 6
  (`dif116Later`). The rest of `dif116_test.go` is for the later subtasks'
  oracles: `holdDif116(t, sections...)`.
  - **The engine came early.** NAMES' listings need the comparison, the
    merged form, the counts, and the echo, so `internal/diff` has them now:
    `Compare` is the manual's rule (the place that leaves the fewest records
    in the section, both files' together; at least one record must agree,
    whatever /MATCH is, so /MATCH=0 and -1 act as 1, as N30 and N31 show),
    which subtask 4 fits to the generated pairs. `Echo` is VMS's line
    breaking, worked out from N37, N38, N60, N61, and N64 and held by
    `internal/diff`'s tests: pieces ("/IGNORE=(SPACING," is one, a list
    breaking between elements) go on a line while they fit in one less than
    the width, else a "-" and a new line of four blanks; each file starts a
    line; and under that every line is cut into pieces of width-1 and a "-"
    (so /WIDTH=12's "DIFFERENCES-" / " /IGNORE=()-"). The counts are broken
    the same way, records and "File" lines cut at the width.
  - **Names.** The inputs go through `$OPEN` with a NAM, as CONVERT's do
    (`rms.Session.ReadInputDefaulted`: CONVERT's reader with a default name),
    so the names and errors are RMS's. The second file's default is the
    first's resultant name without its version (N04: V01.TXT;1 then V03.TXT
    is ;3). One file is compared with its highest lower version, found by
    `Session.Search` of `;*`. OPENIN's secondary status is shown only under
    DNF (N20 has none under FNF). A wildcard in the first is NOWILD before
    anything is opened, in the second RMS's WLD.
  - **Qualifiers.** The numbers are read by the command (`$any` in the
    grammar), so a bad one is VMS's `%DCL-W-NUMBER` with its segment, and in
    32 bits compared signed (MRGGTMTCH and PARGTMTCH as N30, N31, N34, N36;
    /MERGED=-1 lists no records after). /WIDTH below 12 is 12; /WINDOW=0 is
    BADVALUE. /SLP's CONFLICT is the command's check, not a grammar
    DISALLOW as Decision 7 had it: the grammar's DISALLOW gives
    BADQUALIFIERCOMBO, and the message is VMS's either way.
  - **Messages and statuses** (Decision 5, revised): `vmsdef/difmessages.go`
    has the statuses and the texts of DIF's own messages, which
    `LookupMessage` doesn't find, so govax's F$MESSAGE answers NOMSG for them
    as VMS's did; the shared ones (OPENIN, NOWILD, and BADVALUE, new among
    `sharedMessages`) it knows, as VMS's did. Facility 108's name, DIFF, is
    in `otherFacilities`, and NOMSG now names such a facility
    (`vmsdef.FacilityName`) and shows the whole value, STS$M_INHIB_MSG
    included (VMS's "Message number 106C8064"). NOWILD's and BADVALUE's
    values are reckoned, unconfirmed: round 1 printed no real status.
  - **DCL, beyond DIFFERENCES.** `internal/dcl`: a qualifier only a
    parameter takes, given after the verb, is `%DCL-W-IVQLOC` (N49), and a
    parameter's `/optional` beside its `/prompt=` is asked for only after
    an earlier one was (the terminal log's `_File 1:` then `_File 2:`; an
    empty reply leaves it out). `dclshell`'s `verbError`: an extra
    parameter is VMS's `%DCL-W-MAXPARM` with its first word as the segment
    (N27's `\A03\`), for every command.
  - **A user-mode SYS$OUTPUT** naming a file gets the listing (O09), as the
    image's own SYS$OUTPUT: DIFFERENCES translates it itself
    (`differences.userOutput`). govax's other utilities don't yet; not this
    phase's.
  - **Echo forms not yet seen settled**, to be checked by the subtasks whose
    sections show them: /MODE's order (HEXADECIMAL, OCTAL, ASCII, from N64's
    one case), /SEPARATED's and /CHANGE_BAR's NUMBER forms, and which
    characters LEFT and RIGHT name ("[" and "]" assumed).
- **2026-11-25 — subtask 4 done: the engine and the merged form.** The
  matching rule fitted to VMS's listings (`internal/diff/doc.go`'s "The
  search", `compare.go`'s `resync` and `square`), and it fits all 541 merged
  listings of the LAYOUT, MATCHING, LIMITS, and GENERATED sections.
  - **The rule.** Subtask 3's (the fewest records in the section, both sides
    together, the master's furthest place first) already fitted every pair at
    the defaults, but four of the 40 `/MATCH=1` pairs (M1G017, M1G018,
    M1G023, M1G033) showed VMS minimizing the *larger* side: the search is a
    square growing a record at a time from the first records that differ, at
    depth d trying (i+d, j+t) then (i+t, j+d) for t from 0 to d. Searching
    the revision's place first fails 11 cases.
  - **/WINDOW=w** stops the square at depth w; with no place found the
    search starts again a record further on in both files, at depth 0. So
    M03's eight changed records end where they would have at every window,
    while a moved block is swallowed (H04 at /WINDOW=2). Moving on by w
    records, the first reading, fitted all but five (H08_W2, H14_W5, H15_W2,
    H22_W5, H26_W5): in H08_W2, for one, a jump of two records passes over
    the place VMS's last section ends, a record before both files' ends.
  - **/MAXIMUM_DIFFERENCES=n** stops at the section that brings the count to
    n or more (L01_9: exactly 9); that section is cut to the records the
    count had left, of each file from the section's start, whether or not
    they are the section's (L01_2's master "entry 10" after an insertion),
    with no matched records after it, and the echo says `/MERGED=0`. The
    listing is written, then `%DIF-W-MAXDIF` (after it at the terminal too,
    L04), status `%X006C8018`.
  - **Tests.** `internal/diff`'s `TestProbeListings` (`probe_test.go`)
    reads `cases.txt` and holds the 541 listings, up to the echo, in a few
    milliseconds; `internal/console`'s `TestDif116MergedOracle` runs those
    sections on govax and holds every case's messages, status, and whole
    listing, but for the later forms' (PGnnn and EGnnn, subtask 5; SGnnn and
    L03, subtask 6; L02, `dif116IsLater`). HELP DIFFERENCES describes the
    rule, /WINDOW, /MAXIMUM_DIFFERENCES, and MAXDIF's status.
- **2026-11-25 — subtask 5 done: the other forms.** `internal/diff/forms.go`:
  `Separated`, `Parallel` (with `Pretty`, the printable form), `ChangeBars`
  and `Marks`, and `List`, which writes a command's forms in VMS's order;
  `Merged` takes a `Style` (`After`, `NoNumber`, `Closing`). The command
  builds the `diff.Forms` from its qualifiers (`differences.forms`).
  `TestDif116FormsOracle` holds the FORMS section, and `TestDif116MergedOracle`
  now the generated pairs run `/PARALLEL` (PGnnn) and `/SEPARATED` (EGnnn) and
  N36, N62, and L02, all to VMS's listings; `internal/diff`'s
  `TestProbeListings` holds the PG and EG listings too (591 in all). Only
  `/MODE`'s and `/SLP`'s listings (and N64's ignoring) are left as later.
  - **Parallel.** Every line is one less than the width, two halves of
    (width-1)/2 with the `|` at the second's first column; each side's text
    cut two short of it, the right's two after it, padded with blanks (F10,
    F13, F14). A section's rule has `" n "` (each side's first record, an
    empty side's place) from the middle of each half, none under
    `/NONUMBER` (F16); its rows pair the two sides, then `/PARALLEL`'s n
    records after both. The text is always printable: tabs expanded, LF,
    VT, FF, and CR as `<LF>` and the rest, other controls, DEL, %X80 to %X9F,
    and %XFF a period (R12, R11; %XA0 to %XFE kept).
  - **Change bars.** The whole file, `%5d %c ` before each record (`%c `
    under NONUMBER, N64), a changed record's bar its character; where a
    section's side in this file is empty, the record after the place is
    marked (F40's master 16, F41's revision 28). Both files' listings, the
    master's first (F42). A file's `/CHANGE_BAR` alone lists only change
    bars: no merged form, no `/MERGED` in the echo (F40). On identical
    files, nothing (assumed, as for `/PARALLEL`'s F45: F44 put the
    qualifier on the verb, IVQLOC). A file's NUMBER or NONUMBER over
    `/[NO]NUMBER` is unconfirmed (F26 and F27 were on the verb too).
  - **Several forms** (F31 to F33, N64, F21, F22): parallel, then per radix
    the merged, separated, and change bar listings, a blank line between
    each; a merged listing not first gets a closing section with no records
    (`Style.Closing`). With `/MAXIMUM_DIFFERENCES` stopping, only the first
    (L02), the two separated listings counting as one (unconfirmed).
  - **The echo.** `/SEPARATED` of both files, by default or named, is bare
    (F09); `/OUTPUT=` and the name are two pieces, so a narrow listing breaks
    between them (F14, F23); `/MODE`'s order is OCTAL, HEXADECIMAL, ASCII
    (F22, R13; subtask 3's note had it wrong), and `/PARALLEL` adds ASCII
    (F34); a file's `/CHANGE_BAR` with no value is bare (F40).
  - **A race fixed.** `cpu.Table.ByName` built its index lazily with no
    lock, and two consoles assembling at once (the parallel oracle tests)
    crashed on it; it is built under a `sync.Once` now.
- **2026-11-25 — subtask 6 done: radix and SLP.** `internal/diff/radix.go`
  (`Radix`, the dump) and `slp.go` (`SLP`); `Style.Radix` and
  `Forms.Radixes`; the command passes `/MODE`'s radixes and writes `/SLP`'s
  script in place of the listing, counts, and echo. `TestDif116FormsOracle`
  and `TestDif116MergedOracle` now hold every `/MODE` and `/SLP` listing
  (F19 to F23, F33, F35, F36, F48, F49, L03, SAnn, SGnnn); `internal/diff`'s
  `TestProbeListings` reads `/MODE`, `/SLP`, `/WIDTH`, and `/MERGED` beside
  another form, and holds 696 listings (FORMS', IGNORING's I30 and I33, and
  FORMATS' R10, R12 to R14 among them). Only N64's (its `/IGNORE`) is left.
  - **The dump.** Each record a blank line, `RECORD NUMBER n (n) LENGTH n
    (n) ` (the numbers in parentheses eight digits in the radix, the line
    ending in a blank), a blank line, and DUMP-style lines: longwords from
    the highest, each a blank and 8 hexadecimal or 11 octal digits, one the
    record only partly fills showing only its bytes' digits (octal: 3, 6, 8
    for 1 to 3 bytes) right-aligned, one it doesn't reach blank; a blank,
    four characters a longword (`Printable`: a period for what `Pretty`
    makes a period, and for every control; the record's end filled with
    periods); a blank and the offset, six digits. A line holds the most
    longwords that fit in one less than the width, an even number (8
    hexadecimal at 132, 4 at 80, 6 octal at 132); other widths, and an octal
    offset past 0, are unconfirmed. An empty record is its heading alone.
  - **Change bars in a radix** (N64, held by its messages only until
    subtask 7): a marked record's heading gets ` ***CHANGE***`, the heading
    cut at the dump line's length (`***CHANGE**` at /WIDTH=80); the bar's
    character isn't shown.
  - **Order.** Parallel, then per radix ASCII, hexadecimal, octal (F22,
    F33: not the echo's order) the merged, separated, and change bar
    listings. The closing empty section follows a merged listing only after
    a listing of the records as text, the parallel or the ASCII merged one:
    R13's (/MODE=(HEX,OCTAL)) octal listing after the hexadecimal has none,
    where F22's, after ASCII's, has. Subtask 5's rule (every merged listing
    not first) was wrong. With `/PARALLEL` the other forms are listed only in
    the radixes given, not the ASCII `/PARALLEL` adds to the echo
    (unconfirmed: no case).
  - **SLP.** Per section `-%5d,%5d` (the master's first and last records) or,
    for an insertion, `-%5d` (the master's record it follows; `-` alone
    before the first, SA06), the revision's records uncut; `/` at the end,
    alone for identical files (SG137). Under `/SLP`, `/MAXIMUM_DIFFERENCES`
    counts both files' records of a section (`Options.BothSides`): L03 at 3
    stopped at an insertion of two after a section of one, its script
    replacing one record with one, where the merged L01_3 listed two of
    each. Fitted to that one case, unconfirmed.
