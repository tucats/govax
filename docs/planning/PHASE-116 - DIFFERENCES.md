# Phase 116 — DIFFERENCES

**Status:** planned 2026-11-25 (subtask 1, the survey and this plan),
awaiting the author's review. This document is the development plan; as
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

## Part B — decisions (proposed)

1. **VMS 7.1 is the reference**, as for every probe since Phase 55: what
   7.3-1's dictionary adds that 7.1 refuses is left out (the probe's
   A.3.10). If 7.1 has `/PAGE`, govax takes `/PAGE` and `/PAGE=SCROLL`
   and `CLEAR_SCREEN` through `page.go`, and refuses `SAVE` (screen
   navigation) with a message, a deviation logged. *Question 1.*
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
   own (`vmsdef/difmessages.go`), the numbers from the probe's
   `$STATUS` values and the texts from its log, as JBC's were in Phase 98.
6. **`/SLP` is written** though govax has no SLP editor: it is only an
   output form, and a script made on govax should work on VMS.
   *Question 2.*
7. **The grammar**: a verb `DIFFERENCES` (abbreviable to `DIF`, as VMS's)
   with two `$file` parameters, `/CHANGE_BAR` on the verb and on each
   parameter.

## Part C — subtasks

Each is one commit, with tests, stopping for review.

1. **The survey**: Part A, this plan.
2. **The probe**: `testdata/dif116`, as in Part D, written and its
   exchange volume built; run on the VAX by the author; A.3 answered
   (as A.4) and Part B adjusted.
3. **The command's frame**: the grammar, HELP DIFFERENCES in VMS's
   shape, the messages and statuses in `vmsdef`, names (the second's
   defaults, the next lower version), the errors, identical files, the
   output file; held to the probe's NAMES section.
4. **The engine and the merged form**: `internal/diff`'s matching,
   `/MATCH`, `/WINDOW`, `/MAXIMUM_DIFFERENCES`, the merged listing, the
   summary, and the closing echo; held to every generated pair.
5. **The other forms**: `/SEPARATED`, `/PARALLEL`, `/CHANGE_BAR`,
   `/NONUMBER`, `/WIDTH`, several forms together.
6. **Radix and SLP**: `/MODE=HEX` and `OCTAL`, `/SLP`.
7. **Ignoring**: `/IGNORE`'s keywords, EDITED/EXACT/PRETTY,
   `/COMMENT_DELIMITER` and the defaults by file type.
8. **The terminal and the rest**: the terminal's width, `/PAGE` if
   Decision 1 keeps it, records that aren't text.
9. **The close-out**: MANUAL.md (section 6's file commands), HELP,
   VAX-VMS-INTERNALS.md if a mechanism changed, DEVIATIONS.md (what
   stays unconfirmed), PLAN.md, CLAUDE.md.

## Part D — the probe (subtask 2)

`testdata/dif116/`, on an exchange volume as the earlier probes were
(`exchange.cmd`, `copyout.cmd`, an RD53 in `testdata/disks`, attached to
the simh VAX). `gen.go` writes every file but the README:

- **The pairs**: `[DIF116.IN]` holds each case's files, made by
  `gen.go` (A.3's hand-written cases, and about 300 generated pairs from
  a fixed seed: short files of a few distinct lines, so repeats and
  near-matches are common, with insertions, deletions, changes, and
  moves).
- **`dif116.com`**: SET VERIFY, then a section per A.3 question; each
  case runs DIFFERENCES with `/OUTPUT=[DIF116.OUT]Cnnn.DIF` and writes
  `$STATUS` and `F$MESSAGE($STATUS)` to the log; the error cases run at
  the log itself, so their messages show. `/OUTPUT` keeps the bytes: tabs
  stay tabs when the files come back.
- **`dif116t.com`**: the terminal cases, run by hand at an 80- and a
  132-column terminal, whose session log the author keeps (A.3.10).
- **The other record formats**: made on the VAX by CONVERT/FDL from
  `gen.go`'s FDL files, so their attributes are VMS's own.

`copyout.cmd` brings back the log, every `.DIF` file, and a
DIRECTORY/FULL of the outputs. The oracle test (subtask 4 on) reads the
case list from `gen.go`'s output, runs each case on govax, and compares
the `.DIF` files byte for byte and the log's `$STATUS` lines.

## Questions for the author

1. If VMS 7.1 has `/PAGE`, is `/PAGE=SAVE` (screen navigation with the
   keypad, search, highlighting) worth doing, or left out as a deviation?
2. `/SLP`: write its output form (proposed), or refuse it since govax has
   no SLP editor?
3. Is 116 the right number, with Phases 114 and 115 planned and not yet
   done?

## Progress log

- **2026-11-25 — subtask 1, the survey.** The DCL Dictionary's entry
  read; govax's pieces found; the plan and the probe's design written.
