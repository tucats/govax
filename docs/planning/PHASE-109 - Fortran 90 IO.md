# Phase 109 — Fortran 90: I/O

**Status:** planned 2026-11-17 with Phase 99 (`PHASE-99 - Fortran 90.md`,
Part C). Needs Phase 105. Subtask 1, the survey (Part A), done 2026-11-19 and
accepted by the author the same day ("The author's answers"); its A.4 revises
the subtasks below. Subtask 2, the F90 flag and the new edits, done 2026-11-19;
subtask 3, COMPLEX(16) I/O, done 2026-11-20; subtask 4 is next.

## Goal

What Fortran 90 adds to I/O (the standard's chapters 9 and 10), in the compiler
and FTNRTL:

- **nonadvancing I/O**: `ADVANCE='NO'` on formatted sequential READ and WRITE,
  `SIZE=` and `EOR=` on READ, a prompt without a new line at the terminal;
- **OPEN's new specifiers**: ACTION= (READ, WRITE, READWRITE), POSITION=
  (ASIS, REWIND, APPEND), PAD=, DELIM=, STATUS='REPLACE'; and the same as
  INQUIRE's answers, with INQUIRE's IOLENGTH= form;
- **edit descriptors**: B (binary), EN (engineering), ES (scientific), G for
  INTEGER, LOGICAL, and CHARACTER as well as REAL, `Bw.m`, `Ow.m`, `Zw.m`, and
  a zero width where the standard allows none (it doesn't in 90: checked);
- **list-directed and namelist output's delimiters** (DELIM=) and Fortran 90's
  **NAMELIST form** (`&GROUP X=1 /`) beside DEC's `$GROUP … $END`, on input
  and output;
- **I/O items by descriptor**: an array section, an assumed-shape array, an
  allocatable array, a pointer's target, as one item with an NCA descriptor
  rather than an element at a time (Phase 102's expansion kept for what can't);
  and a section as an internal file or ENCODE's buffer, NOTIMPL in Phase 102;
- **derived types** in lists, a component at a time (Phase 105), checked again
  here with the new edits and nonadvancing records.
- **character constants between quotation marks in a run-time format held in
  a variable**: Phase 99's subtask 6 lets a FORMAT statement have them under
  `/F90`, giving FTNRTL the format with apostrophes, and Phase 101's subtask 5
  does the same for a format written as a character constant or a PARAMETER
  (`WRITE (*, '(1X, "X =", I3)') X`; sema's `constantFormat`). A variable's
  text is known only at run time, and FTNRTL's interpreter still refuses a `"`
  (SYNERRFOR, as VMS's FORRTL does), not knowing whether the program was
  compiled `/F90`: it needs FTNRTL to accept the mark (a new entry, or a flag in
  the statement's control block).

## What earlier phases leave in place

- FTNRTL's statement contexts, format interpreter, list-directed and namelist
  I/O, units, RMS's sequential, relative, and indexed files (Phases 80, 81, 88).
- `format`'s checker (`internal/fortran/format`), which learns the new edits.

## Design notes

- **Nonadvancing records**: the unit keeps its record open between statements
  (`UB_` flags in `ftn_units.mar`); the record is written when an advancing
  statement ends it, the unit is closed, or the image ends (the `$` prompt's
  rundown, Phase 80, shows the way).
- **POSITION='APPEND'** is RMS's RAB$V_EOF; ACTION= sets FAC; STATUS='REPLACE'
  deletes and creates.
- **FOR's condition values** for the new errors (EOR, a nonadvancing statement
  on an unsuitable unit) take new message numbers in govax's wording (Decision
  15).
- **Teaching:** FORTRAN-INTERNALS.md's FTNRTL section gains how a record is kept
  open across statements.
- **From Phase 100's survey** (2026-11-17): I/O of COMPLEX(16), which Phase
  100 declares and computes with, is NOTIMPL until this phase (FTNRTL's
  editing of H complex values).

## Open questions

1. VAX FORTRAN's `$` edit descriptor and Fortran 90's ADVANCE='NO' are close
   relatives; should they share FTNRTL's mechanism? Proposed: yes.
2. Namelist output's form under `/F90`: Fortran 90's `&GROUP … /`, DEC's
   `$GROUP … $END` otherwise? Proposed: yes, by `/F90`, since output changes.

## Subtasks (provisional)

Revised by the survey: A.4 is the list to follow.

1. **Survey** (the standard's 9.2.1.3, 9.3.4, 9.6, 10.2, 10.5, 10.8, 10.9; the
   Handbook's chapters 9 and 10) (done 2026-11-19; Part A, accepted).
2. **The new edit descriptors** in `format` and FTNRTL.
3. **Nonadvancing I/O**, SIZE=, EOR=.
4. **OPEN and INQUIRE's new specifiers**, IOLENGTH=.
5. **NAMELIST's Fortran 90 form and DELIM=.**
6. **Items by NCA descriptor** (`FTN$IO_ITEM` learns class 10).
7. **Programs end to end**, every mode, gfortran's witness (records compared
   byte for byte where the standard fixes them).
8. **Close-out**: FORTRAN-INTERNALS.md, MANUAL.md.

# Part A — the survey (subtask 1)

Read for it: the standard's 9.2.1.3 (file position, advancing and nonadvancing),
9.3.4 (OPEN), 9.4.1 (the control list, ADVANCE=, SIZE=, EOR=), 9.4.3 (the
conditions), 9.4.4.4 (formatted and unformatted transfer, padding), 9.6
(INQUIRE), 10.2 (the edit descriptors' grammar, R1005 to R1017), 10.5 (data
editing: 10.5.1.1's B, O, Z with m; 10.5.1.2.3 and .4's EN and ES; 10.5.4's G of
every type), 10.6.1.1 (the left tab limit), 10.8 (list-directed), and 10.9
(namelist); the *Fortran 90 Handbook*'s 9.2.5 (nonadvancing transfer); and the HP
Fortran manual (`VMS-FORTRAN-LANGUAGE.txt`), the 2005 secondary source Phase 81's
answer 4 allows, for what DEC's own Fortran 90 on VMS chose where the standard
leaves a choice (12.5 and 12.6's INQUIRE and OPEN specifiers, 10.5.1.3's namelist
output). And what govax has: `parse/io.go`'s control lists, `sema/io.go` and
`fileio.go`, `ir/lowerio.go` and `lowerfile.go`, `internal/fortran/format`, and
FTNRTL's `ftn_statement.mar`, `ftn_format.mar`, `ftn_edit.mar`, `ftn_float.mar`,
`ftn_listin.mar`, `ftn_listout.mar`, `ftn_namelist.mar`, `ftn_units.mar`, and
`ftn_open.mar`; Phase 80's and 81's VMS runs (`testdata/for80`, `testdata/for81`)
for what VMS's FORRTL refuses.

Nine probe programs, one per area (nonadvancing input and output, the edits on
output and input and at run time, OPEN and INQUIRE, re-OPEN, list-directed input,
namelist, I/O items), were given to govax on 2026-11-19 (FORTRAN, MACRO, LINK, and
RUN, on a scratch volume where files are made) and to gfortran 16.2, `-std=f95`, the
witness. They were scratch work; subtask 7's corpus programs replace them. There is
no VMS probe: no VAX Fortran 90.

## A.1 What the standard asks

- **Nonadvancing I/O** (9.2.1.3.1, 9.4.1.7 to 9.4.1.9, 9.4.3). `ADVANCE=` is a
  scalar default character *expression* (not only a constant), YES or NO, any case,
  trailing blanks ignored; allowed only in a formatted sequential statement with an
  explicit format (not `*`, not namelist) on an external unit (not an internal file,
  not REC=). `SIZE=` (an INTEGER variable) and `EOR=` (a label) only in a READ, and
  only with ADVANCE=; SIZE= and EOR= take effect only when ADVANCE is NO. A
  nonadvancing statement leaves the file within the record: the next statement on
  the unit, of either kind, continues it (9.2.1.3.2: on output "if there is a current
  record, the file position is not changed"). Input past the record's end is the
  **end-of-record condition**: with PAD YES the item is padded with blanks, the
  statement ends, the file goes past the record, IOSTAT= gets a processor-dependent
  negative value (not END='s), SIZE= the count of characters the data edit
  descriptors transferred (padding not counted), and EOR= is taken; with neither EOR=
  nor IOSTAT= the program ends. The **left tab limit** (10.6.1.1) is where the
  statement began in the record, and T and TL are relative to it: a nonadvancing
  `(1X, A)` of `abc` and then `(T2, A)` of `X` puts X in column 6 (gfortran agrees).
- **OPEN** (9.3.4). POSITION= (ASIS, REWIND, APPEND; default ASIS; sequential
  only), ACTION= (READ, WRITE, READWRITE; default processor dependent), DELIM=
  (APOSTROPHE, QUOTE, NONE; default NONE; formatted only), PAD= (YES, NO; default
  YES; formatted only), and STATUS='REPLACE' (FILE= required; an existing file is
  deleted and a new one made). An OPEN of a unit already connected to the same file
  may change BLANK=, DELIM=, and PAD= only. CLOSE gains nothing.
- **INQUIRE** (9.6). Answers POSITION=, ACTION=, READ=, WRITE=, READWRITE=, DELIM=,
  PAD= (each UNDEFINED or UNKNOWN where it doesn't apply), and the **IOLENGTH=** form,
  `INQUIRE (IOLENGTH=n) list`, the length an unformatted record of the list would
  have, in RECL='s units.
- **Edit descriptors** (10.2.1, 10.5). New: `Bw[.m]` (binary), `ENw.d[Ee]`
  (engineering: an exponent that is a multiple of 3, a significand from 1 to 1000),
  `ESw.d[Ee]` (scientific: a significand from 1 to 10; the scale factor ignored on
  output), and `[r]/`, a repeated slash. G (10.5.4) edits every intrinsic type: an
  INTEGER as Iw, a LOGICAL as Lw, a CHARACTER as Aw, d and e ignored. Iw.m, Ow.m,
  Zw.m (and Bw.m) with m 0 and a zero value write blanks. `Gw.d` needs its d in the
  standard (gfortran refuses `G6`); w, e, and n must be positive, so `I0` and `F0.d`
  aren't Fortran 90 (they are Fortran 95's).
- **List-directed** (10.8). Input takes character constants between quotation
  marks as well as apostrophes, and **undelimited** character values (ended by a
  blank, comma, slash, or the record's end; not starting with a quote or `r*`).
  Output writes character values undelimited unless the unit's DELIM= says otherwise;
  delimited values are then separated by value separators, and the delimiter inside
  doubled.
- **Namelist** (10.9). Input starts at `&GROUP`, ends at `/`; names may carry
  sections with strides (`X(3:4) = 2*1.5`, `K(2:4:2) = 7, 8`) and component
  designators (`PTS(2)%X`); character values between quotes or apostrophes. Output is
  `&GROUP`, each `NAME=` and its values, then `/`; character values undelimited
  unless DELIM= says otherwise.
- **I/O items** (9.4.2): a whole array, a section, an allocatable array, a pointer's
  target: its elements in array element order. A section of a CHARACTER array as an
  internal file is a file of its elements' records (9.2.2.1).

## A.2 What govax has

What each probe found:

| Area | govax today | VMS's FORRTL (Phases 80, 81) |
| ---- | ----------- | ---------------------------- |
| ADVANCE=, SIZE=, EOR= | BADSPECIFIER (parse) | — |
| ACTION=, POSITION=, DELIM=, PAD= in OPEN; their INQUIRE answers; IOLENGTH= | BADSPECIFIER | 29 and up are INVARGFOR to FOR$OPEN; INQUIRE knows 29 to 48 |
| STATUS='REPLACE' | KEYCHOICE | — |
| B, EN, ES in a FORMAT statement | FMTSYNTAX | — |
| B, EN, ES, `3/` in a run-time format | SYNERRFOR (FOR 62) | `3/` is SYNERRFOR (#375) |
| `"…"` in a run-time format held in a variable | SYNERRFOR | SYNERRFOR |
| G of a LOGICAL or CHARACTER | OUTCONERR (63), asterisks | G13.6 of an INTEGER or text is FORVARMIS (#169, #170, #271, #280) |
| Ow.m, Zw.m; Iw.0 of 0 | work; blanks | the same |
| `I0`, `F0.2` | OUTCONERR at run time; `format` accepts I0 | VMS's choice, kept |
| `"dq"` or `plain` to a CHARACTER item, list-directed | LISIO_SYN (59) | LISIO_SYN (#806) |
| Namelist input `&G … /`, `X(3:4)=` | INVREFVAR at the section | — |
| Namelist output | ` $GRP`, `'HELLO'`, ` $END` | the same |
| Whole arrays, sections, allocatable, assumed-shape, pointer targets as items | work, an element at a time (class A for a whole contiguous array) | — |
| A section as an internal file or buffer | NOTIMPL (`sema.sectionPlace`) | — |
| COMPLEX(16) items | NOTIMPL (`complex32IO`) | — |
| WRITE on a READONLY unit | WRIREAFIL (47) | the same |
| Re-OPEN of a connected unit, same file, BLANK='ZERO' | works, the new BLANK in effect | — |

And where each change lands:

- **The parser** (`parse/io.go`): `transferSpecs` and `errorSpecs` are the transfer
  statements' keywords; `openSpecs` and `inquireSpecs` OPEN's and INQUIRE's. INQUIRE's
  control list has no IOLENGTH= form and no item list.
- **sema** (`fileio.go`): a table of keywords with FORRTL's codes (OPEN 1 to 27,
  INQUIRE's answers 29 to 48) and choices (`statusChoices` 1 to 4), which FTNRTL reads
  in the same codes. `io.go`'s `ioSpecs` reads ERR=, END=, IOSTAT=; the control
  block's flags word has bits 0 to 3 (ERR=, END=, IOSTAT=, a run-time format) and its
  longword at +4 is "not used yet"; its size word (8) lets it grow. `constantFormat`
  rewrites a constant format's `"…"` under `/F90` (Phase 101) but checks nothing else:
  a bad constant format is FTNRTL's SYNERRFOR at run time, as in VAX FORTRAN.
- **The IR** (`lowerio.go`): END=, ERR=, IOSTAT= by a `Sign` on FTN$IO_END's status
  (0, -1 at an end of file, the FOR number); `ioUnit` already sends an assumed-shape
  array as an internal file through Phase 104's `sequence` (its elements in a
  temporary, copied back after a WRITE to it).
- **`format`**: one `Code` per descriptor, the data descriptors first (I to A), with
  `Options.Quotes` and `Options.RunTime`; `TestVMSSyntax` holds it to VMS's syntax
  cases.
- **FTNRTL**: a statement's record is built in its context (`CTX_RECORD`, 256 bytes,
  `CTX_POS`, `CTX_RECLEN`) and written at `/`, reversion, and the end; input reads a
  record at the start (`FTN$$FMT_START`). The units (`ftn_units.mar`) keep a block per
  unit (`UB_` fields, a record buffer at `UB_RECP`), and units -4 to -1, 5, and 6 two
  streams on the terminal. The `$` descriptor marks the record (`CTX_DOLLAR`), whose
  carriage control becomes `$` (a prompt) when it is written, and a `$` prompt left at
  image rundown is ended by RMS (`rms.FileTable.Rundown`). The format's operations
  (`C_I` to `C_END`) follow `format`'s codes. FTN$IO_ITEM takes class S and class A
  descriptors and hands each to the statement kind's module. GSMATCH is 1,30; the
  last vector entry FTN$RANDOM_SEED_W at 1088. FOR numbers 300 to 305 are govax's
  (Phases 102 to 108).

## A.3 Findings, and what they change

1. **Fortran 90 and VMS's FORRTL read the same text differently at run time, so
   FTNRTL must be told which a statement wants.** Five differences are already held by
   VMS's runs: `"…"` in a run-time format (SYNERRFOR), `r/` (SYNERRFOR, #375), G of an
   INTEGER, LOGICAL, or CHARACTER (FORVARMIS, #169), `"ABC"` and undelimited text in
   list-directed input (LISIO_SYN, #806), and namelist's `$GROUP … $END`. The oracles
   (`TestFor80Oracle`, `TestFor80InputOracle`, `TestNML80Oracle`) hold FTNRTL to VMS's
   answers, and `TestNoF90Unchanged` holds `/NOF90` programs to theirs. **Proposed:** a
   control block flag, bit 4, **F90**, set for every data transfer statement of a unit
   compiled `/F90` (`Unit.F90`), under which FTNRTL takes Fortran 90's reading: B, EN,
   ES, `r/`, and `"…"` in run-time formats; G of every type; list-directed input's
   quoted and undelimited text; namelist's Fortran 90 forms on input and output (finding 6);
   and PAD's padding (finding 5). Without it, nothing changes. The skeleton's last goal
   (the run-time format's `"`) becomes this flag, not a new entry. The FORMAT
   statement's check follows `/F90` the same way (`format.Options.F90`, beside
   `Quotes`): B, EN, ES, and `r/` are accepted only then, so `TestVMSSyntax` stands.
   `Gw` without d and `I0` stay accepted, as VMS's extensions.
2. **ADVANCE= is an expression.** A constant (the usual case) is a flag in the control
   block (bit 5, **nonadvancing**), checked when compiling (KEYCHOICE for any other
   text). A value known only at run time: the compiler calls a small new FTNRTL routine
   (`FTN$ADVANCE(desc)`: 0 for YES, 1 for NO, and anything else a new condition,
   finding 8) and branches to one of two begin calls with two control blocks; the items and
   the end are shared. So FTNRTL's begin call always knows, before it reads a record,
   whether the statement continues one. The compile-time checks (9.4.1's constraints) are
   new sema messages: ADVANCE= with `*`, a namelist, an internal file, or REC=; SIZE= or
   EOR= without ADVANCE=; END=, EOR=, or SIZE= in a WRITE.
3. **SIZE= and EOR=.** SIZE='s variable is given to FTN$IO_END as an optional argument
   (`CALLS #1,G^FTN$IO_END` with its address; `CALLS #0` as now), so no new entry and no
   change to old calls: VAX's argument count says which. EOR= is control block bit 6;
   FTN$IO_END returns **-2** at an end of record (gfortran's value, and IOSTAT='s), -1
   at an end of file as now, and the IR's `Sign` becomes a `Switch` where EOR= is given.
   An end of record with neither EOR= nor IOSTAT= is signaled: a new condition (finding 8).
4. **The open record.** A nonadvancing statement's record is kept in the unit
   (`UB_RECP`'s buffer, with new `UB_` fields: its length, the position, and a flag
   saying it is open, for input and output alike); the next statement on the unit starts
   from it (its left tab limit that position), and doesn't read a record. It is written
   (output) or passed (input) when an advancing statement ends it, and by REWIND,
   BACKSPACE, ENDFILE, CLOSE, and the image's end (the exit handler that closes units).
   A list-directed or unformatted statement on a unit with a record open is the
   standard's undefined case: proposed, the record is ended first (gfortran continues a
   list-directed one; unconfirmed choice, ours). **The terminal:** a READ from the
   terminal while an output unit on it has a record open writes that record first as a
   prompt, with the `$` carriage control the `$` descriptor uses, and the rest of the
   record starts a new one. This is the skeleton's open question 1: the `$` descriptor
   and ADVANCE='NO' share the prompt's writing, not the record keeping (`$` still ends
   its record at the statement's end, a file getting a `$`-marked record, as VMS's).
   The record's limit stays the unit's (133 bytes, or RECL), OUTSTAOVE past it.
5. **PAD=.** FTNRTL today ends a numeric input field at the record's end, so under BZ
   `123` read by I5 is 123 (VAX FORTRAN's rule, which the HP manual describes as
   FORTRAN-77's); Fortran 90's PAD='YES' pads it to `123  `, 12300 under BZ. A and other
   fields already read blanks past the end. **Proposed:** a unit's PAD is YES unless
   OPEN says NO; with the F90 flag, YES pads by the standard; without it, today's rule;
   PAD='NO' (only `/F90` programs can say it) is an error for an advancing READ that
   needs more than the record holds (a new condition) and the end-of-record condition
   for a nonadvancing one.
6. **Namelist under the F90 flag.** Input: `&GROUP` (and DEC's `$GROUP`) starts the
   group, `/` ends it as `$` or `&` does today, names take sections with strides and
   array elements' components (`PTS(2)%X`, Phase 105's leftover), values take `"…"`.
   Output: `&GROUP`, `/`, and character values undelimited unless the unit's DELIM= says
   otherwise — the standard's rule and the HP manual's 10.5.1.3 (its example writes
   `NAME = ' ', ' HEISENBERG'` only with DELIM='APOSTROPHE'). gfortran writes quotation
   marks by default; the standard and DEC agree against it. The skeleton's open question
   2 asked the same and proposed `/F90`; this is the flag's way to it. Without the flag,
   DEC's output, with DELIM= honored if the program gave it.
7. **OPEN and INQUIRE's codes.** FORRTL's codes stop at 27 (OPEN) and 48 (INQUIRE), and
   it refuses 29 up; the new ones are govax's, past them: OPEN's POSITION= 49 (ASIS 1,
   REWIND 2, APPEND 3), ACTION= 50 (READ 1, WRITE 2, READWRITE 3), DELIM= 51 (APOSTROPHE
   1, QUOTE 2, NONE 3), PAD= 52 (YES 1, NO 2), STATUS='REPLACE' 5; INQUIRE's POSITION=
   53, ACTION= 54, READ= 55, WRITE= 56, READWRITE= 57, DELIM= 58, PAD= 59. Their
   meanings, from the HP manual where the standard leaves a choice: ACTION= defaults to
   READWRITE; READ is READONLY's FAC and sharing (a WRITE then WRIREAFIL, as now); WRITE
   gives FAC PUT, DEL, and TRN, a READ then a new condition (finding 8); POSITION='APPEND' is
   DEC's ACCESS='APPEND' (RAB$V_EOF) with ACCESS SEQUENTIAL, 'REWIND' the file's start,
   'ASIS' as now; a POSITION= with ACCESS='DIRECT' INCOPECLO; STATUS='REPLACE' deletes the
   file's highest version and makes a new one with that name (unconfirmed: whether the new
   file takes the deleted version's number, as RMS's supersede would, or the next; older
   versions are left). DELIM= and PAD= are kept in the unit and may change by a re-OPEN
   of the same file, as BLANK= already may. INQUIRE answers by the HP manual's 12.5
   (UNDEFINED when not connected, UNKNOWN where it can't tell). **IOLENGTH=** is the
   compiler's: the list's bytes in longwords, rounded up (RECL='s unit for unformatted
   files, Phase 81), a constant when every item's size is known, else computed (an
   allocatable or assumed-length item); no FTNRTL call.
8. **New conditions** (Decision 15), FOR numbers from 306 in govax's wording:
   end-of-record during a READ (EOR=, IOSTAT -2), a READ on a unit opened ACTION='WRITE',
   an input record too short under PAD='NO', a bad ADVANCE= value, and a nonadvancing
   statement on a unit that can't take one (direct or keyed access, unformatted).
9. **The edits in FTNRTL.** B is FTN_EDIT's integer editing in radix 2 (`FTN$$EDIT_INT`
   has O and Z already, and m); EN and ES are FTN_FLOAT's `LAYOUT` with the exponent
   chosen differently (a multiple of 3, or one digit before the point) and the scale
   factor ignored; input as F. G of an INTEGER, LOGICAL, or CHARACTER item (by the item's
   descriptor's type, under the flag) is I, L, or A with w alone. `r/` is a repeat count
   on `C_SLASH`. New operation codes go after `C_Q` for the data descriptors, so
   `format`'s and FTNRTL's orders stay one: B, EN, and ES as data codes, both lists
   renumbered together (`format`'s `Code.Data` is "at most A"; FTNRTL's test "at most
   C_Q"). EN and ES of COMPLEX items as two reals, as E's are.
10. **COMPLEX(16) items.** FTN_FLOAT edits H values already; a complex H item is two of
    them (DSC$K_DTYPE_HC, 30), in formatted, list-directed (FTN_LISTOUT's and FTN_LISTIN's
    complex forms), namelist, and unformatted statements (its 32 bytes). sema's
    `complex32IO` NOTIMPL goes.
11. **Items by descriptor.** Every item form works today, an element at a time, through
    the scalarizer (Phase 102), so a descriptor is an economy (one call for n), not a
    missing feature. **Proposed:** FTN$IO_ITEM learns class NCA (10) in one place: it walks
    the descriptor's strides and gives each element to the statement kind's module as a
    class S item, so FTN_FORMAT, FTN_LISTOUT, FTN_LISTIN, and FTN_UNFORMATTED don't change.
    The compiler passes a descriptor for a whole allocatable, assumed-shape, or pointer
    array and a section of a variable (Phase 104's `buildNCA`, `sectionNCA`), of an
    intrinsic type; derived types (a component at a time, Phase 105), vector subscripts,
    expressions, and implied-DO keep the element path. Input of `N, A(1:N)` still works:
    each item's descriptor is built at its own call. A run of contiguous elements in one
    unformatted record is unaffected (the bytes are the same). **A section as an internal
    file or ENCODE's or DECODE's buffer** goes through Phase 104's `sequence`, as an
    assumed-shape array already does (a contiguous section described in place, any other
    copied to a temporary and, for a WRITE, back), rather than teaching FTNRTL strided
    records.
12. **Derived types** in lists already go a component at a time (Phase 105); with the new
    edits and open records nothing changes but the cases to test: subtask 7's programs.
13. **gfortran's differences to expect** in the witness: namelist output's quotation marks
    (finding 6), its column-1 text (normalized by `compare.py` as before), IOLENGTH= in bytes
    (32 where govax's is 8), and IOSTAT='s nonzero values for errors.

## A.4 The subtasks, revised

1. **Survey** (this Part A) (done 2026-11-19; accepted by the author the same day).
2. **The F90 flag and the new edits**: control block bit 4 set by the compiler for `/F90`
   units; `format.Options.F90` (B, EN, ES, `r/`); FTNRTL's B, EN, ES, `r/`, G of every type,
   and `"…"` in run-time formats under the flag (finding 1, 9); FTNRTL's operation codes and
   `format`'s renumbered together (done 2026-11-19).
3. **COMPLEX(16) I/O** in every kind of statement (finding 10) (done 2026-11-20).
4. **OPEN and INQUIRE's new specifiers**, STATUS='REPLACE', re-OPEN's DELIM= and PAD=,
   IOLENGTH= (finding 7), and ACTION='WRITE''s condition.
5. **Nonadvancing I/O**: ADVANCE= (constant and run-time, `FTN$ADVANCE`), SIZE= through
   FTN$IO_END's argument, EOR= and -2, the record kept in the unit and the left tab limit,
   the terminal's prompt, PAD= on input (findings 2 to 5), the new conditions (finding 8).
6. **List-directed and namelist under the flag**: list-directed input's quoted and
   undelimited text, DELIM= on list-directed and namelist output, namelist's `&…/` forms,
   sections and components in names (finding 6).
7. **Items by descriptor** (FTN$IO_ITEM's class NCA loop) and sections as internal files
   and buffers (finding 11).
8. **Programs end to end**, every mode, gfortran's witness (records compared byte for byte
   where the standard fixes them), with derived types in the new forms (finding 12).
9. **Close-out**: FORTRAN-INTERNALS.md (how a record stays open across statements; the F90
   flag), MANUAL.md's 14.13 and the FOR numbers 306 up, HELP FORTRAN.

The edits come before nonadvancing I/O (the skeleton had OPEN first) because the flag they
introduce is what nonadvancing input's PAD= and the namelist subtask use.

## A.5 Questions for the author

1. **The F90 flag** (finding 1): every place Fortran 90 and VMS's FORRTL read the same text
   differently is chosen per statement by the compiling unit's `/F90`, through a control
   block bit, so `/NOF90` programs and the VMS oracles are untouched. The alternatives:
   FTNRTL always takes Fortran 90's reading (simpler, but VMS's answers in five oracle cases
   change, and an old program's errors become values), or new begin entries for Fortran 90
   statements (twice the vector for one bit). **Recommendation:** the flag.
2. **Namelist output under `/F90`** (finding 6): `&GROUP … /` with character values
   undelimited unless DELIM= is given, as the standard and the HP manual say (such output
   can't be read back by a NAMELIST READ, as the HP manual warns), or delimited with
   apostrophes by default, as gfortran does with quotes? **Recommendation:** the standard's.
3. **PAD's padding** (finding 5): standard padding only for `/F90` statements (so `123` by I5
   under BZ is 12300 there and 123 in an old program), or for every statement once a unit
   says PAD='YES'? **Recommendation:** by the flag; PAD='NO' as the standard says.
4. **A record left open across a terminal READ** (finding 4): written as a prompt, the rest
   a new record (what DEC's terminals and the `$` descriptor do), or kept whole until the
   output statement that ends it (the standard's strict reading, which would show the prompt
   only after the answer)? **Recommendation:** the prompt.
5. **IOSTAT='s values**: -1 at an end of file (as now), -2 at an end of record (gfortran's)?
   **Recommendation:** yes.
6. **Items by descriptor** (finding 11): keep subtask 7 as an economy (FTN$IO_ITEM's loop,
   the element path kept for what has no descriptor), or drop it, every item form already
   working? **Recommendation:** keep it, small and in one place; sections as internal files
   through Phase 104's copies.

## The author's answers (2026-11-19)

The author read the survey, accepted the plan, and took every recommendation:

1. **The F90 flag** (finding 1): control block bit 4, set for every data transfer
   statement of a `/F90` unit, chooses Fortran 90's reading in FTNRTL; without it
   nothing changes, and `format.Options.F90` does the same for FORMAT statements.
2. **Namelist output under `/F90`**: `&GROUP … /`, character values undelimited unless
   DELIM= is given (the standard's and the HP manual's rule).
3. **PAD's padding** by the flag: standard padding for `/F90` statements, today's rule
   otherwise; PAD='NO' as the standard says.
4. **A record open across a terminal READ** is written first as a prompt (`$` carriage
   control), the rest of it a new record.
5. **IOSTAT=**: -1 at an end of file, -2 at an end of record.
6. **Items by descriptor** stay (A.4's subtask 7): FTN$IO_ITEM's class NCA loop, the
   element path kept for what has no descriptor; sections as internal files and buffers
   through Phase 104's `sequence`.

A.4's nine subtasks are the phase's list; subtask 2 is next.

## Progress log

- **2026-11-17:** Planned with Phase 99.
- **2026-11-19:** Subtask 1, the survey (Part A), for review: the standard's chapters 9
  and 10 and the Handbook's 9.2.5 read with the HP manual's choices; nine probe programs
  on govax and gfortran 16.2 (`-std=f95`), each area's present answer in A.2's table; VMS's
  own refusals found in Phase 80's run (`3/` #375, G of an INTEGER #169, `"ABC"` in
  list-directed input #806), which make a per-statement F90 flag necessary (finding 1);
  thirteen findings, the subtasks revised into nine (A.4), six questions (A.5).
- **2026-11-19:** The author accepted the plan and every recommendation ("The author's
  answers"). Subtask 1 is done; subtask 2 (the F90 flag and the new edits) is next.
- **2026-11-19:** Subtask 2, the F90 flag and the new edits. The compiler sets the control
  block's bit 4, `ir.ControlF90` (`F90` in IR dumps and the MACRO file's comment), for
  every data transfer statement of a `/F90` unit (`ir/lowerio.go`); the `ir` and `codegen`
  goldens of `/F90` units changed by that bit alone. `format.Options.F90` (implying
  `Quotes`) reads `Bw[.m]`, `ENw.d[Ee]`, and `ESw.d[Ee]`, each needing its width (EN and ES
  their d), and `r/` (`Item.Repeat` on a `Slash`, a comma needed before it, none after);
  the codes are `B`, `EN`, `ES` after `A`, `Code.Data` "at most ES". FORMAT statements,
  constant formats, and the `/STANDARD` walk (B, EN, ES, and a repeated slash are
  extensions to FORTRAN-77) pass `F90`. FTNRTL: `ftn_format.mar`'s codes renumbered with
  `C_B`, `C_EN`, `C_ES` after `C_Q` (10 to 12), so FTN_EDIT's and FTN_FLOAT's are unchanged;
  PARSE takes a fifth argument, the flag (`EXTZV` of `CTX_FLAGS`'s bit), under which it
  reads B, EN, ES, `r/`, and `"…"` (QUOTED's mark in R4); the walk's `/` ends `r` records;
  EDIT's `GENERAL` makes G of an INTEGER (B, W, L), LOGICAL (BU, WU, LU, so BYTE too), or
  CHARACTER item a copy of the operation as I, L, or A with w alone. `ftn_edit.mar`'s B is
  O and Z's code with a digit of 1 bit, both ways. `ftn_float.mar`'s ES is E with the scale
  factor 1; EN (`OUT_EN`) rounds to d+3 digits for the exponent, then to k+d, and a carry
  there adds a digit before the point, the digits past the rounding zeros (`L_LIMIT`, which
  SHOWN now honors); zero is `0.ddd` with exponent 0 (gfortran's). No new entry, so GSMATCH
  stays 1,30. `testdata/fortran90/edits.f90`, its output worked by hand and the same on
  gfortran, runs in every mode; the VMS oracles (`TestFor80Oracle`, `TestFor80InputOracle`,
  `TestNML80Oracle`) and `TestNoF90Unchanged` are unchanged, the flag being clear for them.
  Found on the way: an internal file's and an unopened unit's BLANK= was VAX FORTRAN's
  'ZERO', where Fortran 90's default is 'NULL' (the standard's 9.3.4.9, 9.2.2.2); so
  `'1.5E3   '` read by ES8.1 was `1.5E3000` and INPCONERR in a `/F90` program, where
  gfortran reads 1500. **The author's answer (2026-11-19):** under `/F90`, the standard's
  default, as gfortran. A unit's `UB_BLANK` has a third value, 3, "ZERO by default" (no
  OPEN has said; a new unit block's), and an internal file's `CTX_UNITBLANK` is 3 too;
  FTN$$UNIT_RECORD passes it on, and FTN$$FMT_START makes it ZERO for a statement
  without the F90 flag and NULL with it. An OPEN already gave BLANK='NULL' by default
  (sema's `openDefaults`, 'ZERO' under `/NOF77`). INQUIRE, having no flag, still answers
  'ZERO' for 3 (VAX's answer; unconfirmed whether a Fortran 90 program should see
  'NULL'). `edits.f90` reads `1.5E3` unjustified, and `12 3` by I5 as 123 and by
  `(BZ, I5)` as 12030, the same on gfortran. Subtask 3 (COMPLEX(16) I/O) is next.
- **2026-11-20:** Subtask 3, COMPLEX(16) I/O. sema's two NOTIMPLs (`complex32IO`, an item and
  a NAMELIST entity) are gone; the compiler already gave such an item a descriptor of
  DSC$K_DTYPE_HC (30), so the work is FTNRTL's, with no new entry (GSMATCH stays 1,30).
  `ftn_format.mar`'s FTN$$FMT_ITEM splits an HC item into two H parts, each taking an edit
  descriptor, as it does the other complex types. `ftn_listout.mar` writes one in
  `1PG42.33E4` per part (COMPLEX\*16's widths one narrower than its real's, as the table's
  others are), between ` (`, `,`, and `)`; its text, up to 86 bytes, needed CTEXT widened
  from 52 to 88, and a field longer than a record of 81 is written whole as a record of its
  own (PUT_FIELD's rule already; VMS's FORTRAN had no COMPLEX\*32, so govax's choice,
  unconfirmed, which a READ takes back whole), but split after its comma in an internal file
  (the existing rule). `ftn_listin.mar`'s STORE takes HC (CONVERT's H, its VALUE grown to
  32 bytes, a number's imaginary part of 0 cleared at 16 bytes); `ftn_namelist.mar` takes a
  complex value for an HC entity, and its output frame's FIELD grew from 100 bytes to 120
  for the 86-byte field. Unformatted I/O needed nothing: the item is its 32 bytes.
  `testdata/fortran90/cplx16io.f90` runs every kind of statement in every mode; its output
  worked by hand (F form's 33 significant digits, E form's 34 after 1P, the namelist's value
  on the record after its name when it won't fit beside it). gfortran agrees but for
  list-directed and namelist output's layouts, the processor's: `compare.py`'s KNOWN now
  lets a line stand for a list of lines (none, for one gfortran doesn't print). MANUAL.md's
  14.13 says what COMPLEX(16) I/O does, and its list of what isn't there lost the stale ES,
  EN, and run-time `"…"` sentences subtask 2 had left. Subtask 4 (OPEN and INQUIRE's new
  specifiers) is next.
