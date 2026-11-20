# Phase 109 — Fortran 90: I/O

**Status:** done 2026-11-20. "Where things stand", at the end, is what the phase
left. Planned 2026-11-17 with Phase 99 (`PHASE-99 - Fortran 90.md`,
Part C). Needs Phase 105. Subtask 1, the survey (Part A), done 2026-11-19 and
accepted by the author the same day ("The author's answers"); its A.4 revises
the subtasks below. Subtask 2, the F90 flag and the new edits, done 2026-11-19;
subtask 3, COMPLEX(16) I/O, done 2026-11-20; subtask 4, OPEN and INQUIRE's new
specifiers, done 2026-11-20; subtask 5, nonadvancing I/O, done 2026-11-20; subtask 6,
list-directed and namelist under the flag, done 2026-11-20 in two parts: 6a, the forms and
DELIM=, and 6b, sections and components in namelist names; subtask 7, items by descriptor, done
2026-11-20; subtask 8, programs end to end, done 2026-11-20; subtask 9, the close-out, done 2026-11-20.

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
   IOLENGTH= (finding 7), and ACTION='WRITE''s condition (done 2026-11-20).
5. **Nonadvancing I/O**: ADVANCE= (constant and run-time, `FTN$ADVANCE`), SIZE= through
   FTN$IO_END's argument, EOR= and -2, the record kept in the unit and the left tab limit,
   the terminal's prompt, PAD= on input (findings 2 to 5), the new conditions (finding 8)
   (done 2026-11-20).
6. **List-directed and namelist under the flag** (finding 6), in two parts:
   - 6a. List-directed input's quoted and undelimited text, DELIM= on list-directed and
     namelist output, namelist's `&…/` forms and quoted values (done 2026-11-20).
   - 6b. Sections with strides and components in namelist input's names (`X(3:4)`,
     `K(2:4:2)`, `PTS(2)%X`), and arrays of derived types in NAMELIST groups (sema's
     NOTIMPL, Phase 105's leftover) (done 2026-11-20).
7. **Items by descriptor** (FTN$IO_ITEM's class NCA loop) and sections as internal files
   and buffers (finding 11) (done 2026-11-20).
8. **Programs end to end**, every mode, gfortran's witness (records compared byte for byte
   where the standard fixes them), with derived types in the new forms (finding 12) (done
   2026-11-20).
9. **Close-out**: FORTRAN-INTERNALS.md (how a record stays open across statements; the F90
   flag), MANUAL.md's 14.13 and the FOR numbers 306 up, HELP FORTRAN (done 2026-11-20).

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
- **2026-11-20:** Subtask 4, OPEN and INQUIRE's new specifiers. The parser takes ACTION=,
  POSITION=, DELIM=, and PAD= in OPEN, those and READ=, WRITE=, READWRITE=, and IOLENGTH= in
  INQUIRE, and an output list after INQUIRE's control list (`ast.AuxIOStmt.Items`). sema's
  table has finding 7's codes (OPEN's 49 to 52, INQUIRE's 53 to 59, STATUS='REPLACE' 5),
  every new keyword accepted under either `/F90` setting (no valid old program changes; the
  `/STANDARD` walk calls each an extension, from its table). `checkOpen` refuses, for constant
  values, READONLY with another ACTION=, REPLACE with READONLY or ACCESS='APPEND', ACTION='READ'
  with NEW, SCRATCH, REPLACE, or ACCESS='APPEND' (but not DISPOSE='DELETE': the HP manual's
  12.6.24 makes that READONLY's difference), POSITION= with direct or keyed access,
  POSITION='REWIND' with ACCESS='APPEND', and DELIM= or PAD= on an unformatted file (by FORM=,
  or by direct or keyed access with no FORM=); REPLACE without FILE= is the new NEEDFILE (the
  standard's 9.3.4; VAX's FORnnn default isn't offered for it). FTNRTL's FTN_OPEN does the same
  at run time (INCOPECLO): CODES grew to 53, the choices' table to 52; READONLY is ACTION='READ'
  inside it. The unit block gains `UB_ACTION`, `UB_DELIM`, `UB_PAD`, and `UB_POSITION` (UB_BLN
  620+44), set by FTN$$UNIT_INIT to READWRITE, NONE, YES, ASIS. ACTION='READ' clears FAC (GET),
  ACTION='WRITE' gives FAC PUT, DEL, TRN; FTN$$UNIT_RECORD's WRIREAFIL is now any unit of
  ACTION READ (and FTN_RECORD's DELETE of an indexed one), and a read of an ACTION WRITE unit is
  the new REAWRIFIL, FOR number 306 (`vmsdef/formessages.go`, `FOR$_REAWRIFIL` ^X188994 in
  FTN_UNITS). POSITION='APPEND' is ACCESS='APPEND''s RAB$V_EOF and NEF cleared, an existing
  file's UB_APPENDED set (BACKSPACE has no count); 'REWIND' and 'ASIS' need nothing, a new OPEN
  putting the file at its start. STATUS='REPLACE' is a `$ERASE` of the FAB (RMS$_FNF passed
  over), then the `$CREATE` NEW's; the new file takes the next version left, so the erased one's
  number when it was the highest (unconfirmed, as finding 7 says; RMS's supersede would need the
  version named). A re-OPEN of the same file changes DELIM= and PAD= beside BLANK=. FTN_INQUIRE's
  table runs to 59: with no connection ACTION, POSITION, and DELIM are UNDEFINED, READ, WRITE,
  and READWRITE UNKNOWN, PAD YES (Fortran 90's 9.6.1.21 and the HP manual; gfortran answers
  Fortran 2003's UNDEFINED); DELIM is UNDEFINED for an unformatted unit, POSITION for a direct
  or keyed one; POSITION is the OPEN's, or REWIND after a REWIND statement (FTN_POSITION sets
  it), where the standard leaves a moved file's answer to the processor. No new entry, so
  GSMATCH stays 1,30. IOLENGTH= is the compiler's, as finding 7 planned: `sema.IOLength` (the
  IOLENGTHFORM message for a form mixed with others), lowered by `ir/lowerfile.go`'s `ioLength`
  to a temporary that each item adds its bytes to, then `(n+3)/4` into the variable: a scalar's
  type size, or its CHARACTER length when only the program knows it (`chars`); an array's
  elements times their size, its element count `lowerarray.go`'s new `elementCount` (the
  extents' product, no loop) when not known, an element at a time only for an array of a
  run-time CHARACTER length; an implied DO a counted loop; a derived type's object its bytes, as
  unformatted I/O writes it. Nothing else in the list is evaluated; the optimizer folds a list of
  constant sizes. `testdata/fortran90/openings.f90` runs every new specifier, by constants and by
  values, the two conditions by IOSTAT=, a re-OPEN, and IOLENGTH= of each kind of item, then as a
  direct file's RECL=, in every mode; gfortran agrees but for IOLENGTH='s bytes (finding 13), PAD=
  with no connection, and POSITION= after a write (compare.py's KNOWN). sema's TestErrors has the
  new checks. MANUAL.md's 14.13 has a paragraph on OPEN and INQUIRE. Subtask 5 (nonadvancing
  I/O) is next; it and subtask 6 give DELIM= and PAD= their effect.
- **2026-11-20:** Subtask 5, nonadvancing I/O. The parser takes ADVANCE=, SIZE=, and EOR= in a
  READ and ADVANCE= in a WRITE (`writeSpecs`), under either `/F90` setting as subtask 4's
  keywords are (`/STANDARD` calls them extensions, from its table). sema (`io.go`'s
  `advanceSpec`, `checkAdvance`): ADVANCE= a CHARACTER value, a constant YES or NO (any case,
  trailing blanks off) checked now (KEYCHOICE, KEYTEXT) and its NO `IO.Nonadvancing`, any other
  value `IO.Advance`; only a formatted sequential statement with a format on an external unit
  (ADVANCEFORM: not list-directed, namelist, internal, ENCODE or DECODE, REC=, keyed, or
  REWRITE); SIZE= an INTEGER place (BADSIZE), `IO.Size`; EOR= a label, `IO.EOR`; SIZE= and EOR=
  only with ADVANCE= (ADVANCENEEDED). The IR: control block bits 5, 6, and 7,
  `ControlNonadvancing`, `ControlEOR`, `ControlBadAdvance`; a run-time ADVANCE= is a call of
  FTNRTL's new `FTN$ADVANCE` (0 YES, 1 NO, 2 neither) and a `Switch` to one of three begin calls,
  each with its control block (`advanceBegin`; the statement's end work held, `holdingAfter`);
  SIZE= a longword FTN$IO_END is given the address of, then stored in the variable with a
  conversion (with the copies back); `ioFinish` takes EOR= and whether the READ may be
  nonadvancing, and for -2 branches to EOR=, or with IOSTAT= alone past END= and ERR='s Sign
  (`ir/testdata/f90advance`). FTNRTL: FTN$ADVANCE at 1096, GSMATCH 1,31. The unit block keeps a
  record left open (`UB_OPENREC` 1 output or 2 input, `UB_DOLLAR`, `UB_RECLEN`, `UB_RECPOS`; UB_BLN
  620+56) in the unit's buffer, where FTN_FORMAT built or read it. The begin call
  (FTN_STATEMENT's 70$) leaves it for a formatted sequential statement of its direction (not
  direct, REWRITE, or keyed) and ends it for any other (`FTN$$UNIT_FLUSH`: an output record
  written, `$`'s rewrite if a `$` marked it, an input one passed); a READ first calls
  `FTN$$UNIT_PROMPT`, which at a terminal writes each terminal unit's open output record as a
  prompt (the author's answer 4); a nonadvancing statement on a keyed unit is NONADVUNIT. CLOSE
  (and so the exit handler) and REWIND, BACKSPACE, and ENDFILE (FTN_POSITION's START) end it too.
  FTN$$FMT_START takes it up, reading no record, its position the left tab limit (`CTX_LEFT`,
  which T adds to and TL stops at; 0 again at each new record), and FTN$$FMT_END leaves a
  nonadvancing statement's record in the unit if it had no error. Input: FTN_EDIT's FIELD and A
  count the characters taken (`CTX_COUNTED`, SIZE='s) and give a field the record's end cut short
  (no comma) to `FTN$$FMT_SHORT`: in a nonadvancing READ that sets `CTX_EOR`, and after the item
  FTN$$FMT_ITEM fails the statement with EORDURREA, which FTN$IO_END returns as -2 to EOR= or
  IOSTAT= and otherwise signals; an advancing F90 statement on a PAD='NO' unit is RECTOOSHO; and
  where the standard pads (PAD YES, nonadvancing or F90) and BZ makes blanks count, FIELD pads the
  field into the context's own CTX_RECORD (a unit's record is in the unit's buffer), at most 256
  characters (finding 5; the author's answer 3). VAX FORTRAN's statements are unchanged: the VMS
  oracles (`TestFor80Oracle`, `TestFor80InputOracle`, `TestNML80Oracle`) and `TestNoF90Unchanged`
  pass. New conditions, `vmsdef/formessages.go`: EORDURREA 307, BADADVANCE 308, NONADVUNIT 309,
  RECTOOSHO 310. The new context fields are past CTX_RECP+88 (CTX_SIZE CTX_RECP+100), cleared by
  PUSH_CONTEXT. `testdata/fortran90/advance.f90` (with `advance.in`) runs in every mode;
  gfortran agrees but for four of govax's choices, in compare.py's KNOWN: IOSTAT='s value for a bad
  ADVANCE=, the prompt (gfortran continues the record), a list-directed statement after a
  nonadvancing one (gfortran continues it; ours ends it, finding 4), and PAD='NO''s short record
  (gfortran's end of record, Fortran 2003's rule; ours an error, with SIZE= counting what A took).
  Unconfirmed choices: a field longer than 256 characters is padded only to 256; a record left open
  at a unit that isn't a terminal stays open across a terminal READ; units 6 and `*` are distinct,
  so a record open on one isn't continued by the other. MANUAL.md's 14.13 has "Nonadvancing I/O".
  Subtask 6 (list-directed and namelist under the flag, DELIM=) is next.
- **2026-11-20:** Subtask 6a, list-directed and namelist forms and DELIM=. Subtask 6 is split:
  6a the forms (this entry), 6b sections and components in namelist names, which need arrays of
  derived types in groups, still sema's NOTIMPL. No compiler change: the control block's F90
  flag (subtask 2) already reached every list-directed and namelist statement, and FTNRTL reads
  it from `CTX_FLAGS`. **List-directed input** (`ftn_listin.mar`): under the flag a quotation
  mark opens a character constant as an apostrophe does (READ_TEXT, `FTN$$LI_TEXT`, takes its
  delimiter from the record, so namelist shares it), and a V_WORD is undelimited text to a
  CHARACTER item of a list-directed READ (`CTX_KIND` K_READ_SL; namelist's values must be
  delimited, 10.9.1.3); without the flag both are VMS's LISIO_SYN still. **List-directed output**
  (`ftn_listout.mar`'s `PUT_DELIMITED`): a unit whose `UB_DELIM` is APOSTROPHE or QUOTE has its
  text delimited, doubled delimiters, a blank before; one that doesn't fit what's left of the
  record starts the next, one longer than a record goes on in the next record's first column (the
  standard's 10.8.2 rule, so a READ \* takes it back whole; at a terminal the carriage control
  eats that character, unconfirmed choice), a doubled delimiter never split; an internal file never
  delimits. This holds with or without the flag (finding 6: DELIM= honored if the program gave
  it). **The unit's DELIM=** now starts at 0, not given (`ftn_units.mar`), which INQUIRE answers
  NONE as before, so that a VAX FORTRAN statement can tell DEC's default from an explicit 'NONE'.
  **Namelist** (`ftn_namelist.mar`): under the flag input reads from column 1 (`CTX_LI_COLUMN`
  0, no carriage control), finds the group at a record's first byte that isn't a blank or a tab
  (`&GRP` in column 1, as gfortran writes it), ends at a slash wherever a $ or an & would end it
  (`IS_END`: between assignments, between values, after `r*`, ending a number), and takes `"…"`
  values; output is ` &GRP` and ` /` (`TEXT_FORM`, `DELIMCH`, `CONTBLANK`), text undelimited
  unless DELIM= is APOSTROPHE or QUOTE (the author's answer 2), commas kept between an array's
  undelimited elements as the HP manual's example has them (unconfirmed: the standard says
  adjacent undelimited values aren't separated), a delimited value continued in the next record's
  first column, a doubled delimiter never split, and a comma at a full record's end starting the
  next after a blank. Without the flag, VMS's output, but DELIM='QUOTE' writes quotation marks
  and DELIM='NONE' none. The VMS oracles (`TestFor80Oracle`, `TestFor80InputOracle`,
  `TestNML80Oracle`, `TestNML80Messages`, `TestNML80Prompt`) and `TestNoF90Unchanged` are
  unchanged. `testdata/fortran90/listnml.f90` (with `listnml.in`) runs in every mode; gfortran
  agrees on every value, its differences the records' length and namelist output's layout and
  quotation marks (compare.py's KNOWN). `cplx16io.out` and `typenml.out` now show `&GRP` ... `/`
  and `typenml`'s text undelimited. MANUAL.md's 14.13 has "List-directed and namelist I/O".
  FTNRTL's GSMATCH stays 1,31 (no new entry). Subtask 6b (sections and components in names) is
  next.
- **2026-11-20:** Subtask 6b, sections and components in namelist names, and arrays of derived
  types in NAMELIST groups; subtask 6 is done. **The compiler** (`sema/derivedio.go`): an array of
  a derived type, or a component that is one, is no longer NOTIMPL. Its entities are as a scalar
  object's (a header, then each component's, named by designator), but each is every element's
  across the arrays it goes through: `NamelistEntity`'s new `Sym`, `Offset` (from the object's
  start, every subscript at its lower bound), `Type`, and `Ranks` (`NamelistDim`s, the fastest
  first: the component's own, then each enclosing array's, innermost first; `ranksOf`,
  `namelistRanked`, `namelistArray`). An entity with no ranked part keeps its old `Ref`, so a
  scalar object's block is unchanged. Lowering (`ir/lowerio.go`'s `namelistDescriptor`) makes a
  ranked component a class NCA descriptor with constant strides (`ir.Descriptor.Strides`, written
  by codegen's `writeStrided`, `Fixed` giving them to a frame cell, `describe` its A0 by
  `NCAFirst`), and a ranked header a class Z descriptor whose pointer is its shape
  (`Descriptor.Shape`), a class NCA descriptor of dtype Z with no address that FTNRTL reads the
  rank, bounds, and strides from; a scalar object's header still points at nothing. FTNRTL's
  frame limits are sema's: 15 dimensions in all and 8 objects deep (`namelistMaxRank`,
  `namelistMaxDepth`; past them NOTIMPL). **FTNRTL** (`ftn_namelist.mar`): `DESIGNATOR` replaces
  FIND_ENTITY and REFERENCE. A name is parts joined by `%`, each looked up as a prefix (the header
  `PTS`, then `PTS%X`) before its subscripts are read; a part's own dimensions are those its
  entity has past the prefix before it (`RANK_OF`, `DIM_INFO`). Subscripts fix an element's
  bytes (`OFFSET`), a section (`SUBSCRIPTS` reads `lo:hi:stride` under the F90 flag only, so VMS's
  syntax errors and INVREFVARs stand) and a part without subscripts walk theirs; the walk is up to
  15 dimensions, the slowest first, and `ADDRESS` takes an element number apart into bytes. A name
  of one element, every part's subscripts given and none a section, keeps VMS's rule, it and the
  rest of its last part's array, which the oracles hold; `VS%V(1)` is a section. A name of an
  object fills level by level (`NEXT_PART`, a stack of 8 `LEVELS`): a level's elements (the walk's
  at the top, a nested array's next to each other below it), and in each its parts in turn, a
  part that is a header a level of its own, so `PTS = ...` and `LS(1) = 'QRS', ...` go element by
  element, each element's components in order. Output (`ENTITY_VALUES`, `ELEMENT_AT`) writes a
  class NCA entity's elements in array element order, `PTS%X = 3*0.0000000E+00,` on its line;
  `LS%ENDS%X` with LS(2) and ENDS(2) is LS(1)'s two, then LS(2)'s. Unconfirmed choices (ours): a
  name with two ranked parts (`LS%ENDS%X`, which the standard's designators forbid and output
  writes) is taken in input too, the rightmost part's dimensions fastest; an unknown name's message
  shows the text from its start to the end of the word at the failure (`PTS(2)%Z`); a section is
  checked against the bounds only at its first and last elements, and an empty one isn't checked.
  `testdata/fortran90/nmlsect.f90` (with `nmlsect.in`) runs in every mode, a recursive routine's
  frame array among them (`namelistCell`'s descriptors written each statement). gfortran isn't a
  witness for it: it refuses `PTS%Y = 3*0.5` ("Repeat count too large", its own limit with a
  component of an array of a derived type) and VMS's element rule; the first two lines agree.
  `TestDerivedIO` now expects the arrays to compile and NOTIMPL past FTNRTL's limit; the VMS
  oracles (`TestNML80Oracle`, `TestNML80Messages`, `TestNML80Prompt`, `TestFor80Oracle`) and
  `typenml`, `listnml` are unchanged. MANUAL.md's 14.13 has the names and the arrays. FTNRTL's
  GSMATCH stays 1,31 (no new entry). Subtask 7 (items by descriptor) is next.
- **2026-11-20:** Subtask 7, items by descriptor. **FTNRTL** (`ftn_statement.mar`): FTN$IO_ITEM
  sends a class NCA item to a routine of its own, `ITEM_NCA`, which walks the descriptor (its
  POINTER the first element; each dimension's count its extent, the first dimension fastest, each
  step its stride in bytes, a finished dimension stepped back by stride times extent) and gives each
  element to the statement kind's module (`PASS_ITEM`) by a class S descriptor in its frame,
  stopping at the statement's first failure; an empty dimension is no elements. So FTN_FORMAT,
  FTN_LISTOUT, FTN_LISTIN, and FTN_UNFORMATTED are unchanged, and see what the element calls gave
  them. The walk is a routine apart so that FTN$IO_ITEM's frame (`^M<R2>`) stays as it was for every
  other item: a first try that saved R3 to R7 there changed what an old program's undefined value
  read from the stack (`optforward.for`'s KEEP under `/RECURSIVE`, which `TestFTN86Oracle` holds to
  VMS's 0). No new entry: GSMATCH stays 1,31. **The compiler** (`ir/lowerio.go`'s `ncaItem`): an item
  that is a whole assumed-shape array or array pointer (`Symbol.Strided`, the dummy's or pointer's own
  descriptor, `wholeNCA`), or a section, a ranked component (but a scalar record's whole array
  component, still class A), or a substring of a section or array, with no vector subscript
  (`sema.Described`), of an intrinsic type, and not a PARAMETER's, is one FTN$IO_ITEM call with a class
  NCA descriptor (`sectionNCA`, `arrayNCA`); a contiguous whole array keeps its class A descriptor, and
  a derived type's object (a component at a time), a record, a vector subscript, an expression, and a
  constructor keep the element path. The descriptor is built at its item's call, so `READ *, N,
  A(1:N)` still uses the N just read. **Sections as internal files and buffers**: sema's
  `sectionPlace` NOTIMPL is gone; a section (or its substring, or a ranked component) as an internal
  file or ENCODE's or DECODE's buffer goes through Phase 104's `sequence` (`ioUnit`): described where
  it is if `sema.Contiguous`, tested when the program runs if it is an assumed-shape array's, else a
  temporary, copied back after a WRITE or ENCODE. One with a vector subscript is the new VECTORFILE
  (the standard's 9.2.2.1). The IR golden `f90io` now shows sections as one call;
  `testdata/nof90.sum` was rewritten for `testdata/fortran`'s `recarrays.for` and `recdyn.for`, whose
  ranked components in I/O lists are now one call each (no VAX FORTRAN 5.0 program has a section, so
  the promise of Decision 16 stands). `testdata/fortran90/ioitems.f90` (with `ioitems.in`; strided
  sections both ways in two dimensions, an array pointer, an assumed-shape dummy, formatted,
  list-directed, unformatted, and nonadvancing, an empty section, an end of file part way through a
  section, a ranked component, a substring of a section, sections as internal files written and read)
  runs in every mode, and gfortran prints the same 16 lines; `ioitemsvms.f90` (ENCODE and DECODE on a
  strided section, and an assumed-shape array's section as an internal file) is govax's only. MANUAL.md's
  14.13 says how I/O lists take sections and that a section may be an internal file. Subtask 8
  (programs end to end) is next.
- **2026-11-20:** Subtask 8, programs end to end. Three programs in `testdata/fortran90`, each
  worked by hand from the standard, run in every mode (`/NOOPTIMIZE`, `/OPTIMIZE`, both
  `/RECURSIVE`) with paging off and on, and once more under `/CHECK=BOUNDS`, and each printing the
  same on gfortran (`compare.py`'s `PROGRAMS90`, no known differences). `journal.f90`: an array of a
  derived type written to a file by one statement whose format reverts for each element (B, ES, L,
  and G of a CHARACTER item); each record read back whole by a nonadvancing READ whose SIZE= at
  EOR= gives its length, so the records are compared byte for byte where the standard fixes them
  (trailing blanks and all); the file read back by the same format into a second array and
  compared; a direct access file whose RECL= is INQUIRE's IOLENGTH= (the value itself never
  printed, so the longwords and gfortran's bytes don't show), written last to first and read in
  another order; NAMELIST output with DELIM='APOSTROPHE' read back, then a group of changes named
  by sections and components (`BOOK(2:3)%CLEARED`, `BOOK(5)%MEMO` with a doubled apostrophe); a
  report whose lines are built by nonadvancing statements with EN; components of every element
  and of every other as list items. `wordcount.f90` (with `wordcount.in`): the terminal read a
  character at a time by nonadvancing READs, each line's end by EOR= and the input's by END=, an
  empty line and leading and repeated blanks among the lines; words in an array of a derived type,
  sorted by swapping whole objects; a histogram whose bars are drawn a character at a time and
  ended by `WRITE (*, '()')`; the words packed into lines of 40 columns. `gradebook.f90`: a type
  with an array component through files made with STATUS='REPLACE', ACTION='WRITE', and
  POSITION='APPEND' (asked by INQUIRE), read with ACTION='READ', a short record padded; a test's
  column as one ranked component across the students in formatted and unformatted lists; names
  written to an internal file that is a section with a stride. Nothing in govax needed changing:
  the three differences on the way were the programs'. gfortran refuses a repeat count on a
  component's section (`BOOK(2:3)%CLEARED = 2*T`, "Repeat count too large", as `nmlsect`'s notes
  found), which the standard allows, so `journal` writes `T, T`; `wordcount` first used MAXLOC's
  DIM=, which govax rightly refused (BADARG): it is Fortran 95's, and Fortran 90's MAXLOC has only
  ARRAY and MASK (13.13.64), so it takes the subscript from a vector of one; and `gradebook`'s
  hand-worked output had names flush left where A10 output of a shorter constant pads on the left
  (10.5.4), on which both compilers agreed, so the program now writes the names by A and T11.
  Twelve programs earlier subtasks and phases had left off `fortran90PagingPrograms` (`advance`,
  `cplx16io`, `listnml`, `ptralias`, `ptrequiv`, `results`, `structs`, `transfer`, `transfervms`,
  `typearrays`, `types`, `typevalues`), though the README says every program runs with paging on;
  they are on it now and pass. The README has rows for the three programs, and for `nmlsect`,
  `ioitems`, and `ioitemsvms`, which had none. Subtask 9, the close-out, is next.
- **2026-11-20:** Subtask 9, the close-out; the phase is done. FORTRAN-INTERNALS.md's section 34,
  "Fortran 90: I/O", follows a program through the new statements: the F90 flag (why per statement,
  what it changes, why the new specifiers need none), the new edits as old code with a parameter,
  the record kept in the unit across statements (`UB_OPENREC` and its three deciders, the left tab
  limit, the prompt shared with `$`), the end of record as a condition (SIZE= through FTN$IO_END's
  argument, FOR numbers 306 to 310), a run-time ADVANCE= by `FTN$ADVANCE` and three begin calls, a
  section as one item (`ITEM_NCA`, and why it is a routine apart), OPEN, INQUIRE, and IOLENGTH=,
  list-directed and namelist I/O, and what the programs found; its listings are govax's own
  output. The stale forward references in sections 27, 32, and 33 and the introduction are
  updated. MANUAL.md: 14.13 gains "Edit descriptors" (B, EN, ES, G of every type, `r/`, quotation
  marks in run-time formats, BLANK='NULL' under `/F90`), which subtask 2 had left out; 14.6's
  FTNRTL table has the control block's bits 4 to 7, `FTN$ADVANCE`, FTN$IO_ITEM's class NCA and
  HC, and FTN$IO_END's SIZE= argument and -2; "Not there yet" says what of I/O isn't (FTNRTL's
  NAMELIST limits; Fortran 95's zero widths, which FTNRTL takes for a field too narrow, OUTCONERR,
  found by running the manual's example). HELP FORTRAN's Fortran_90 topic lost four stale passages
  (COMPLEX(16) I/O, sections element by element, arrays of types in NAMELIST, no ES or EN) and has
  a paragraph on Fortran 90's I/O; `helplib.hlb` rebuilt by `go generate`. PLAN.md's row and
  summary.

## Where things stand

Phase 109 is done: every Fortran 90 I/O statement, specifier, and edit descriptor compiles and
runs, VAX FORTRAN's statements unchanged (the VMS oracles and `TestNoF90Unchanged`). What it left,
and where each goes:

- **Phase 110:** `/STANDARD=F90`'s judgment of the new specifiers and edits (today each is called
  an extension to FORTRAN-77 only); the debugger's EXAMINE of a unit's open record, and of the new
  forms' values.
- **Phase 112 (optional):** Fortran 95's I/O, zero widths (`I0`, `F0.d`), which FTNRTL today takes
  for a field too narrow for any value (OUTCONERR).
- **Not planned:** Fortran 2003's I/O (DECIMAL=, ROUND=, stream access, a short record's end on a
  PAD='NO' unit as an end of record, which gfortran gives); a NAMELIST object past FTNRTL's limits,
  8 levels of arrays and derived types or 15 dimensions in all (NOTIMPL, named).
- **Unconfirmed choices (ours, logged above):** INQUIRE's BLANK= answers 'ZERO' for a unit no OPEN
  described, having no F90 flag; STATUS='REPLACE''s new file takes the erased version's number
  when it was the highest; a COMPLEX(16) list-directed field longer than a record is a record of
  its own; a field padded under BZ is padded to 256 characters at most; a record left open on a
  unit that isn't a terminal stays open across a terminal READ; units 6 and `*` are distinct, so a
  record open on one isn't continued by the other; commas kept between an array's undelimited
  namelist values; a delimited value continued in a record's first column loses that character to
  the carriage control at a terminal; a namelist name with two ranked parts (`LS%ENDS%X`) taken in
  input, the rightmost part fastest; a section in a namelist name checked against its bounds only
  at its first and last elements.
