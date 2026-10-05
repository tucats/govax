# Phase 38 — ANALYZE/OBJECT

**Status:** in progress (started 2026-10-04).

## Goal

A console `ANALYZE/OBJECT` command whose output matches VMS 7.3's
`ANALYZE/OBJECT` (ANALYZ V07-04) line for line: every record of an object
module described and annotated (record numbers and sizes, header text, each
GSD subrecord's fields and flag bits, each TIR command with its code and the
linker's stack depth, STORE IMMEDIATE data as a hex and ASCII dump, the end
of module record), the errors ANALYZE finds, a summary of record types, and
VMS's page layout.

`internal/obj` already reads, checks, and dumps object modules (Phase 27),
but `obj.Dump` is a one-line-per-item debugging format of govax's own, and
no console command shows a module. This phase adds ANALYZE as the user sees
it on VMS.

The author asked (2026-10-04) that the grammar allow for an `ANALYZE/IMAGE`
command later. This phase builds only `/OBJECT`, but the verb, the paged
output, and the package are laid out so `/IMAGE` slots in beside it.

## Reference output

Fifty-four `.anl` files in `testdata/` have the `.obj` they describe beside
them, all from VMS 7.3's `ANALYZE/OBJECT/OUTPUT=NAME.ANL NAME.OBJ`:

| Directory | Pairs | What the modules exercise |
|---|---|---|
| `testdata/mar/vax/` | 12 | Phase 27's ladder: psects, globals, relocation, branches, entry masks, expressions |
| `testdata/mar/macros/vax/` | 17 | Phase 28's macro fixtures (system macros, a library's modules), and VMS's analysis of govax's own objects for them (`gv_*`) |
| `testdata/mar/list/vax/` | 23 | Phase 29's listing fixtures: TBT and DBG records, `.ENABLE DEBUG`, a module with errors (`dbgsrc`, no EOM) |
| `testdata/mar/round/vax/` | 2 | Phase 29's round trip: modules without `.TITLE` (`notitle2`, `notitle3`) |

The 23 other `.anl` files (`testdata/mar/vax/govax/gv_*.anl` and
`testdata/mar/round/vax/gv*.anl`, without objects beside them) are
VMS's analysis of objects govax wrote at the time; they serve as extra
samples of the format but aren't oracle pairs. `testdata/link/vax/*.ani`
are `ANALYZE/IMAGE` output, for a later phase.

### What the output looks like

```
<FF>
Analyze Object File                          29-SEP-2026 13:13:51.97   Page 1
DUA2:[000000]HELLO.OBJ;1
ANALYZ V07-04

This is an OpenVMS VAX object file

1.  MODULE HEADER (OBJ$C_HDR_MHD), 50 bytes

	structure level: 0
	...

10.  TEXT INFORMATION/RELOCATION (OBJ$C_TIR), 48 bytes

	1)  TIR$C_STA_PB (4, %X'04')                            stack depth: 1
		psect: 1
		value: 0 (%X'00000000')
	...
	9)  Store Immediate, 13 bytes:
		  7  6  5  4  3  2  1  0          01234567
		------------------------          --------
		 77 20 2C 6F 6C 6C 65 48|  0000  |Hello, w|
		          21 64 6C 72 6F|  0008  |orld!   |
...
SUMMARY STATISTICS:

Record Type	Count	Total Bytes

OBJ$C_HDR	    4	   111
...
Totals		   18	   368


The analysis uncovered NO errors.


ANALYZE/OBJECT/OUTPUT=HELLO.ANL HELLO.OBJ<blanks to column 80>
```

Observations from the survey (all from the fixtures; the clean-room rule
leaves ANALYZE's output as the only source):

- **Page header.** A form feed on a line of its own, then the title
  (`Analyze Object File`, padded to column 45), the date and time to the
  hundredth (day of month blank-padded: ` 2-OCT-2026`), three blanks, and
  `Page N`; then the input's full file specification, `ANALYZ V07-04`, and
  a blank line. The time is read as each page starts (the last page of
  `hello.anl` is a hundredth later than the rest). `ANALYZE/IMAGE`'s header
  is the same with the title `Analyze Image`.
- **Pagination** is by line count, but not a fixed one: full pages hold 56
  to 62 lines. A record's heading never starts a page below line 56; other
  breaks fall almost anywhere, even between two flag lines, but a hex dump
  doesn't start where its first rows won't fit. The rule, reconstructed in
  subtask 3, is under "Page layout" below.
- **Indentation** is tabs: one for a record's items, two for their fields,
  three for flag bits.
- **TIR commands** are numbered from 1 within each record:
  `N)  TIR$C_name (code, %X'hh')`, padded to 56 characters, then
  `stack depth: d` when the command changes the linker's stack depth
  (CTL_AUGRB and OPR_NEG don't show it). Fields follow on their own lines:
  `psect: n`, `value: n (%X'hhhhhhhh')` (signed decimal),
  `unsigned value: ...` (STA_UB, STA_UW), `symbol: "NAME"`.
- **STORE IMMEDIATE** is `N)  Store Immediate, n byte(s):` and a dump of
  eight bytes per row, highest address on the left, the row's offset, and
  the bytes as characters. A byte shows as itself from `%X'20'` to
  `%X'7E'` and from `%X'A1'` to `%X'FE'` (DEC Multinational), else as `.`.
  The output file is 8-bit (ISO Latin-1 bytes), not UTF-8.
- **GSD subrecords** spell out their type (`Program Section Definition
  (GSD$C_PSC)`), each field, and every flag bit by number and name with its
  value (`(3)  GPS$V_REL        1`). A psect definition is tagged
  `<-- psect n` with its index. An entry mask is a register list
  (`<R2,R3>`).
- **Errors** are `***  ` lines where ANALYZE finds them (`dbgsrc.anl`: `End
  of module record is missing from previous module.` and `The stack still
  contains 1 longword.`), and the count closes the analysis (`The analysis
  uncovered 2 errors.`, or `NO errors.`).
- **The trailer** is the command line, after DCL's symbol substitution,
  padded to 80 columns.

## Design

### Package `internal/anl`

A new package, `internal/anl` (ANALYZE), holds the analyzers; the console
only finds files and hands bytes in. It imports `internal/obj` and
`internal/vmsdef`, nothing from the console or RMS.

- `page.go` — `Pager`: the paginated output, shared by every analyzer.
  It takes the title, the file specification, a clock (so tests are
  deterministic), and writes lines through the page-break rule. A writer
  can ask that the next n lines stay together (a hex dump's first rows, a
  record heading), which is how the reconstructed rule is expressed.
  `ANALYZE/IMAGE` will reuse it with its own title.
- `object.go` — `Object(w, records, Options)`: decodes and describes each
  record, accumulates the summary, and reports errors. Record and
  subrecord descriptions are table-driven (feedback: dispatch through
  tables, not switches): one entry per record type, per header type, per
  GSD subrecord type, and per TIR command, giving its title and how its
  fields print. The TIR table extends `internal/obj`'s command table
  (`obj.Op`, `StackEffect`) rather than repeating it.
- `dump.go` — the STORE IMMEDIATE hex/ASCII dump.
- `check.go` — ANALYZE's own error checks and messages, run as the
  records are described (an error is printed where it's found, so it must
  be found then, not by `obj.Check` afterwards). Where `obj.Check` already
  knows a rule, `anl` reuses it; the message text is ANALYZE's.

`Options` carries the record-type selection (`/MHD`, `/GSD`, `/TIR`,
`/TBT`, `/DBG`, `/LNK`, `/EOM`), the file specification for the page
header, and the command line for the trailer.

A file may hold more than one module (an object file made by
concatenating objects): each module ends at its EOM, and a header after
records without one gets `End of module record is missing from previous
module.`, as `dbgsrc.anl` shows at the end of the file.

### DCL grammar

```
    syntax analyze_object
        parameter files/type=$string/list/prompt="File"
        qualifier host/parameter=files
        qualifier output/type=$string
        qualifier dbg
        qualifier eom
        qualifier gsd
        qualifier lnk
        qualifier mhd
        qualifier tbt
        qualifier tir
        qualifier include/type=$string/list

    verb analyze
        qualifier object/syntax=analyze_object
        ! qualifier image/syntax=analyze_image   (a later phase)
```

A qualifier of the object syntax may come before `/OBJECT`
(`ANALYZE/GSD/OBJECT`), as real DCL allows: when the verb doesn't know a
qualifier, the parser looks ahead on the line for one of the verb's
syntax-switching qualifiers and switches early (`syntaxLater` in
`internal/console/dcl/parse.go`; a pure fallback, so lines that parsed
before parse as they did).

`ANALYZE` alone answers that a qualifier is needed (VMS's own default
isn't settled from the evidence here); `/OBJECT` selects the object
syntax, and `/IMAGE` will select its own syntax, with its own
qualifiers, when it's built. `/HOST` follows the convention of MACRO,
LINK, and LIBRARY.

### Files

Input names follow `rms.Session.Locate`, with default type `.OBJ`; each
file of the list is analyzed in turn. `/OUTPUT=` defaults to the console
(SYS$OUTPUT); a named output file defaults to the input's name with type
`.ANL`, beside the input, and is a text file (`rms.TextRecords`): one
record per line, the form feed a record of its own. The page header's
file specification is the file actually read: a volume file's full
specification with its version (`DUA2:[000000]HELLO.OBJ;1`), or the host
path.

`/INCLUDE=(module,...)` analyzes modules of an object library (`.OLB`)
through `internal/lbr`; with no list, every module. There's no fixture for
a library's analysis, so its layout (where the module names appear,
whether each module's pages restart) is a reasonable choice, logged as
unconfirmed.

### `govax analyze`

A one-shot `analyze` subcommand in `cmd/govax`, beside `macro`, `link`,
and `library`: `govax analyze --object FILE [--output FILE]`, running the
console command.

## Subtasks

1. **Plan** (this document), committed.
2. **`internal/anl` content, unpaginated.** The record, subrecord, and TIR
   command tables; the hex dump; the summary and closing lines. Tested
   against the 54 fixtures with page headers removed and dates masked:
   every content line must match.
3. **Pagination.** `Pager` and the page-break rule, reconstructed from the
   fixtures' 400-odd page breaks; the trailer. The fixture test now
   compares whole files, masking only the date and time.
4. **Errors.** ANALYZE's checks, in place, with `dbgsrc.anl`'s two
   messages exactly, and the rest of `obj.Check`'s rules in ANALYZE's
   style (unconfirmed text). Unit tests for each, on hand-built modules.
5. **Record and command types the fixtures lack.** Every GSD subrecord
   type (SYM/EPM/PRO in their word, vectored, version-mask, and local
   forms; IDC, ENV, SPSC), every TIR command, the LNK record, EOMW, and
   the other header types (CPR, MTC, GTX), in the fixtures' style. Each
   round-trips through `obj.Builder` in a test. Logged as unconfirmed.
6. **Console command.** The grammar (`console.dcl`), the handler
   (`internal/console/analyze.go`), files on the host or a volume,
   `/OUTPUT`, the record-type qualifiers, multiple files. Tests through
   `Dispatch`, including a fixture analyzed off a volume, whose header
   names the volume file.
7. **Object libraries.** `/INCLUDE`, through `internal/lbr`.
8. **`govax analyze`**, help text (`vax.help`), `CLAUDE.md`, `PLAN.md`.

## Page layout

Reconstructed from the fixtures' page breaks (subtask 3), and matching
all 54 byte for byte:

- A page is a 5-line header (form feed line, title, file, version, blank)
  and room for **55 lines** of the report.
- Before each line is written, ANALYZE checks that the lines it needs are
  left on the page, and starts a new page when they aren't. Most lines
  need 1. Headings need room for what follows them:

  | Line | Needs |
  |---|---|
  | record heading (`N.  ...`) | 5 |
  | GSD subrecord heading (`k)  Program Section ...`) | 3 (2 or 3 fit the fixtures) |
  | TIR command heading (`k)  TIR$C_...`) | 3, with or without fields |
  | `attribute flags:`, `symbol flags:` | 3 |
  | `Store Immediate, n bytes:` | 2 |
  | hex dump heading (`7  6  5 ...`) | 4, however many rows follow |

- The two blank lines that close each record are written without the
  check, so a page can run to 57 lines; the blank line between two items
  of a record is checked like any other line, so it can start a page.
- The summary always starts a new page.

The method: with the report's content known to match, each kind of line
gives two bounds, the furthest down a page it's ever written and the
fullest page it was ever pushed off of; where they don't overlap the
need is fixed. Only blank lines conflicted, which separated the closing
blanks (never pushed) from the separators (pushed at 55).

## Decisions and unconfirmed rules

- The output is matched to the fixtures byte for byte except for the date
  and time; anything the fixtures don't show is a reasonable choice in the
  same style, recorded here when made.
- The closing command line is the line as typed, trimmed (VMS shows it
  after DCL's processing; the fixtures' commands were already uppercase).
- Several files on one command, or `/OUTPUT` with several inputs: each
  report is complete (its own pages from 1, its own summary and closing
  line), one after another (unconfirmed).
- `ANALYZE` with no qualifier asks for one rather than assuming `/OBJECT`
  (unconfirmed what VMS does).
- **ANALYZE's names differ from VMS 7.3's objfmt.sdl in two places.**
  TIR command 22 is `TIR$C_STO_LW` to ANALYZE and `TIR$C_STO_L` in the
  SDL (`commandNames` in `internal/anl/tir.go`). ANALYZE labels psect
  flag bit 10 `GPS$V_COM` and bit 11 `GPS$V_NOMOD`; the SDL has
  `GPS$V_NOMOD` = 10 and `GPS$V_COM` = 11. No fixture sets either bit,
  so whether ANALYZE's labels or the bits it reads are out of step is
  unconfirmed; govax shows ANALYZE's labels over the bits in their
  positions (`psectFlagBits`).
- **Text headers** are shown 65 characters to a line, each piece quoted on
  its own line, with unprintable characters as periods (the hex dump's
  rule). Read off `lctlnosh.anl`, `gv_uselibm.anl`, and `notitle.anl`.
- **Stack depth** is shown only for a command that changes it, and the
  depth carries from record to record (TIR, DBG, and TBT alike) until the
  module's end.
- **Unconfirmed layouts** (no fixture shows them; same style as the rest):
  the MHD's `last patch date/time:` line (its label is the one `creation
  date/time` is padded to match), EOM transfer flags, severities other than
  `successful`, a psect's shareable-image base, symbol environments,
  vectors, version masks, procedures' formal arguments, IDC and ENV
  subrecords, the GSD titles other than PSC, SYM, and EPM, and the fields
  of TIR commands other than the stack commands the fixtures show and
  CTL_AUGRB. Data types are named from `DSC$K_DTYPE_` (only `Z` is seen).

## Errors

An error is a `***  ` line right after what it's about (or, for a module
that ends badly, before the next module or the summary), and the count
closes the report. Real ANALYZE's output shows two messages; the rest are
govax's, in their style (unconfirmed), for the object language's rules
that `obj.Check` also applies (`internal/anl/check.go`):

| Message | When |
|---|---|
| `End of module record is missing from previous module.` | a module ends (another MHD, or the file's end) without EOM — **real** |
| `The stack still contains n longword(s).` | a module ends with a nonempty stack — **real** |
| `The module header record is missing.` | a module's first record isn't an MHD |
| `The record is longer than the maximum record size, n bytes.` | a record exceeds its MHD's maximum (the MHD included) |
| `GSD subrecord k is malformed: ...` / `Command k is malformed: ...` | decoding stops there; the items before it are shown |
| `The record is malformed: ...` | a header, EOM, or LNK record doesn't decode |
| `Record type n is undefined.` | an unknown record type |
| `The stack underflowed.` / `The stack is deeper than 25 longwords.` | a TIR command's effect on the linker's stack |
| `Psect n is undefined.` | a psect index (TIR, symbol, EOM transfer) the module doesn't define; psects defined later in the module count |
| `Severity n is undefined.` | an EOM severity above 3 |
| `The psect/symbol/module/environment name must be 1 to 31 characters.` | a name's length |
| `Psect alignment n is larger than a page.` | an alignment above 9 |

## Open items

- `ANALYZE/IMAGE` (a later phase; `testdata/link/vax/*.ani` are its
  reference output).
- `/INTERACTIVE` (ANALYZE's record-by-record mode) isn't planned.

## Progress log

- 2026-10-04: Survey and plan. 54 `.anl`/`.obj` pairs found; line formats,
  the hex dump's character rule, the error lines, and the trailer read off
  them; pagination found to need its own reconstruction.
- 2026-10-04: Subtask 2: `internal/anl` (`line.go`, `object.go`,
  `gsd.go`, `tir.go`, `dump.go`). `TestObjectContent` matches all 54
  fixtures line for line once page headers and the closing command line
  are removed. Found on the way: ANALYZE's `STO_LW` and COM/NOMOD labels,
  the 65-character text-header lines, no blank line after the closing
  errors. `obj.Symbol` gains `HasEntryMask` and `IsLocal`, so `anl`
  doesn't repeat `obj`'s layout table.
- 2026-10-04: Subtask 3: `Pager` (`internal/anl/page.go`) and the
  page-break rule ("Page layout"): `Line.Keep` and `Line.Spill` carry
  it. `TestObjectPages` compares each of the 54 fixtures whole, masking
  only the page headers' times: all match byte for byte. The trailer is
  the command padded to 80 columns and a line end (the survey's "8 blanks
  without a line end" was a misreading of an octal dump).
- 2026-10-04: Subtask 4: errors ("Errors"). GSD and TIR records are
  decoded item by item (`obj.DecodeSubrecord`, `obj.DecodeCommand`,
  exported for this), so a malformed record is shown up to its bad item.
  Each module's psects are counted when it starts, as `obj.Check` does.
  `TestObjectErrors` and `TestObjectMalformedCommand`; the 54 fixtures
  still match.
- 2026-10-04: Subtask 5: every record, header, GSD subrecord, and TIR
  command type. `TestEveryCommandShown` decodes each command code and
  checks it's named and its operands shown; `TestEveryKind` analyzes a
  module with one of everything (stack balanced, no errors) against
  `internal/anl/testdata/kinds.txt`, a golden report that collects the
  unconfirmed layouts in one place for review (`go test -update` rewrites
  it). IDC flags are shown field by field (ident match, error severity,
  binary ident); ENV flags as named bits. On a stack underflow the stack
  is taken as empty, so each underflow is reported once.
- 2026-10-05: Subtask 6: the console command. `console.dcl`'s `analyze`
  verb and `analyze_object` syntax; `Console.AnalyzeObject`
  (`internal/console/analyze.go`): host or volume files (default type
  OBJ), the report to the console (Latin-1 converted to UTF-8 there) or
  with `/OUTPUT` to a text file (default NAME.ANL beside the first input;
  each input's report paged on its own, one after another), the
  record-type qualifiers, `CLI_ANALYZE` and `CLI_ANALYZEERRORS` (a
  warning, after every report is written). The DCL engine switches to a
  syntax early for a qualifier written before the one that selects it.
  The page header names a host file by its absolute path. Tests:
  `TestAnalyze_hostFile` (VMS's HELLO.ANL, times, file, and command
  masked), `_console`, `_volume` (header `DUA0:[000000]HELLO.OBJ;1`),
  `_errors`; `TestParse_syntaxQualifierLater`. An early run of the tests,
  with `/OUTPUT`'s empty default mistaken for `/OUTPUT`, overwrote two
  fixtures beside their objects; they were restored from git before
  anything was committed.
