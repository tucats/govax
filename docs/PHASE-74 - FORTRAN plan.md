# Phase 74 — FORTRAN: the plan, the decisions, and the groundwork

**Status:** in progress. Part A (the survey), Part B (the decisions),
and Part C (the phases) were reviewed and accepted by the author on
2026-10-29 (the answers are under "The review" at the end). Subtasks 1
(the plan) and 2 (the compiler's skeleton and the FORTRAN command) are
done; subtasks 3 to 5 are next. Phases 75 to 86 each have a
skeletal planning doc of their own. Needs nothing new: MACRO-32 (Phases
27–29), LINK and shareable images (Phases 30 and 68), the RTL machinery
(Phases 34 and 53–59), and traceback (Phase 70) are all in place.

govax can assemble, link, and run MACRO-32 programs, and it runs images
that VMS's compilers made. It has no compiler of its own for a
high-level language. This program adds one: a **FORTRAN compiler**,
built into govax as the MACRO assembler is, and invoked by a `FORTRAN`
DCL command. It reads a FORTRAN source file from the host or from a
Files-11 volume and writes a **MACRO-32 source file**. MACRO assembles
that file, and LINK links it against **FTNRTL.EXE**, a shareable image
of FORTRAN run-time routines that we write in MACRO-32 ourselves. RUN
then runs the program:

```
$ FORTRAN SQUARES          ! SQUARES.FOR -> SQUARES.MAR
$ MACRO SQUARES            ! SQUARES.MAR -> SQUARES.OBJ
$ LINK SQUARES             ! SQUARES.OBJ + FTNRTL.EXE, LIBRTL, MTHRTL -> SQUARES.EXE
$ RUN SQUARES
```

**Fidelity here means something different from earlier phases.** The
goal is to compile and run FORTRAN programs correctly. Matching DIGITAL's
VAX FORTRAN compiler's object code, psect contents, optimizations,
listings, or messages is **not** a goal. The VMS documentation sets the
target in three ways:

- the **language** to accept (VAX FORTRAN's syntax and meaning);
- the **run-time behavior** a program can observe (values, I/O formats,
  IOSTAT numbers, condition values);
- how a FORTRAN program **calls VMS**: system services, LIB$ and other
  RTL routines, argument-passing mechanisms, and condition handlers.

A second goal is **teaching**. The compiler's Go code should be readable
by someone who has never written a compiler, or never met the VAX: each
pass explains what it does and why, and the MACRO it writes explains
itself. The generated `.MAR` file carries the FORTRAN source as comments,
line by line, so that someone debugging a program sees both.

## Sources

- ***VAX FORTRAN Language Reference Manual*** (AA-D034E-TE, June 1988,
  VAX FORTRAN V5.0 on VMS 5.0): the dialect this program targets. The
  scan's OCR text was saved beside its PDF on 2026-10-29 as
  `~/Documents/Technical Doc/VMS/vax-fortran-lrm-1988.txt` (searchable,
  with OCR errors: `~` for `*` in `REAL*8`, `I F` for `IF`, and so on).
  The chapters this program relies on most: 1 (source forms), 2 (data
  types, constants, arrays, records), 3 to 6 (statements and
  subprograms; Table 6-1, the argument-passing defaults and %VAL, %REF,
  %DESCR), 7 to 9 (I/O statements, formatting, OPEN/CLOSE/INQUIRE; Table
  7-1, the list-directed output formats), 10 (CDEC$ directives; Table
  10-1, common block psect attributes), and appendices A, C, and D (old
  forms, data representation, the intrinsic function table D-3).
- ***HP Fortran for OpenVMS Language Reference Manual*** (2005,
  `VMS-FORTRAN-LANGUAGE.txt`): Fortran 95 on Alpha and Itanium. It is
  useful only where it describes a VAX FORTRAN feature more clearly. It
  is not the target dialect (Decision 1).
- **The VAX FORTRAN User Manual is not in the folder.** It is where VMS
  documents the compiler's qualifiers, the run-time error list, and the
  default logical names for units (FOR$READ, FOR$PRINT, FORnnn). Where
  this plan relies on those, it says so and marks the choice
  unconfirmed.
- **The calling standard**: *OpenVMS Calling Standard* (V7.3,
  `OVMS_73_CALL_STAND.PDF`), its VAX chapter: CALLS/CALLG frames, the
  argument list, descriptors, function values in R0/R1 or through a
  hidden first argument, and register preservation.
- **The RTL manuals**: VMS 5.0 *MTH$* (`AA-LA72A`) and *OTS$*
  (`AA-LA73A`), with the 7.3 versions (`OVMS_73_RTL_MTH.PDF`); the LIB$
  manuals already used by Phases 53–61; the 5.0 *System Services
  Reference Manual* and VSI's text.
- ***VAX MACRO and Instruction Set Reference Manual*** (VSI's text):
  what the generated MACRO may use.
- **RMS**: the *RMS Reference Manual* and the *Guide to OpenVMS File
  Applications*, for FTNRTL's file I/O.

The clean-room rules (CLAUDE.md) apply as before. The compiler and FTNRTL
are written from the manuals above. VMS's FORRTL.EXE is not studied as a
model: FTNRTL is our own design. The system definitions a FORTRAN
program INCLUDEs come from govax's own `vmsdef` tables (Decision 17),
never from VMS's FORSYSDEF.TLB.

# Part A — the survey

## What a compiler does, briefly

For readers new to compilers, the work splits into passes. Each pass
reads the previous one's output and hands on something simpler:

```
 SQUARES.FOR
     |
     v
 source forms ---- fixed, tab, and extended lines; continuations;
     |             comments; D lines; INCLUDE         -> statements
     v
 lexer ----------- FORTRAN's blanks don't matter and its keywords
     |             aren't reserved: classify each statement first,
     |             then split it into tokens           -> tokens
     v
 parser ---------- grammar rules, operator precedence  -> syntax tree (AST)
     v
 semantic analysis  names and types, implicit typing, constants,
     |             arrays, COMMON/EQUIVALENCE storage, intrinsics,
     |             label and DO rules, errors           -> typed program
     v
 lowering -------- arrays to address arithmetic, DO to trip counts,
     |             I/O statements to run-time calls     -> IR (basic blocks)
     v
 optimizer ------- optional: folding, common subexpressions,
     |             loop invariants, register allocation -> better IR
     v
 code generator -- VAX instructions and addressing modes,
     |             registers, the calling standard     -> MACRO-32 text
     v
 SQUARES.MAR  --MACRO-->  SQUARES.OBJ  --LINK (FTNRTL, LIBRTL, MTHRTL)-->  SQUARES.EXE
```

The first four passes are the **front end**: they know FORTRAN but not
the VAX. The last two are the **back end**: they know the VAX but not
FORTRAN's syntax. The IR (intermediate representation) between them is
the contract.

## What govax already has

- **MACRO-32** (`internal/asm`, Phases 27–29): real MACRO's language,
  the macro facility, psects, listings, traceback records by default,
  and debugger records with `/DEBUG`. Its object files match VMS's. The
  compiler's output only has to be correct MACRO.
- **LINK** (`internal/link`, Phases 30 and 68): images, shareable images
  (`/SHAREABLE`, transfer vectors through `.TRANSFER`, GSMATCH,
  universal symbols), options files, clusters, overlaid psects (all
  contributions to an `OVR` psect start at its base: COMMON blocks).
- **Finding shareable images** (`rms.ImageLocator`,
  `machine/shareable.go`): a logical name of the image's name, then
  `SYS$SHARE:name.EXE`, then the host library directory (`vax.library`).
  `runtimeLibraries` makes govax stand in for VMS's run-time libraries,
  FORRTL and MTHRTL among them: a routine no shim provides resolves to a
  stub that signals NOTIMPL.
- **LIBRTL in Go** (`internal/librtl`): LIB$SIGNAL, LIB$STOP,
  LIB$ESTABLISH, LIB$REVERT, LIB$MATCH_COND, LIB$GET_VM/FREE_VM,
  LIB$INDEX, LIB$DATE_TIME, the timers, LIB$GET_INPUT/PUT_OUTPUT,
  LIB$SPAWN, and **OTS$'s conversions** from Phase 72: OTS$CVT_T_F/D/G/H
  (text to floating), OTS$CVT_L_TI/TO/TZ/TL and back (integers),
  OTS$CNVOUT, and `OTS$$CVT_x_T_R8` (floating to text, which VMS's FORRTL
  uses for its F, E, G, and D edit descriptors). FTNRTL can call all of
  these.
- **The VAX's arithmetic in Go** (`internal/vaxfloat`): F, D, G, and H
  formats, with exact values and the CPU's rounding. The compiler's
  constant folding can use it, so a folded constant is bit for bit what
  the CPU would compute.
- **VMS's definitions** (`internal/vmsdef`): every symbolic constant
  (`$SSDEF`, `$IODEF`, `$JPIDEF`, ...), including the **FOR$_ condition
  values** (`FOR$_ENDDURREA` is `%X1880C4`; its message number, 24, is
  the IOSTAT value VAX FORTRAN documents). There are 15 MTH and 10 OTS
  message texts, but **no FOR message texts**.
- **The librarian** (`internal/lbr`): reads and writes text libraries
  (`.TLB`, `TypeText`), which `INCLUDE '(module)'` reads.
- **RMS** (`internal/rms`): sequential files on volumes and the host,
  FORTRAN carriage control at the terminal (`ftncc.go`, Phase 72), and
  keyed *reads* of fixed-length sequential files by record number
  (`keyed.go`, FORTRAN's direct-access READ).
- **Traceback** (Phase 70): an unhandled error in a program linked with
  traceback gets its symbolic stack dump. For our programs the names and
  line numbers are those of the `.MAR` file, which is why the `.MAR`
  file carries the FORTRAN source.
- **The command plumbing**: `dclcmd.Context` commands find a file on the
  host or a volume (`rms.Session.Locate`, `ReadRecordFile`,
  `CreateRecordFile`), and `console.dcl` binds verbs and qualifiers. The
  MACRO command (`internal/dclcmd/macro.go`) and `govax macro` are the
  model for FORTRAN and `govax fortran`.

## What govax lacks

- **MTHRTL**: no MTH$ routine exists (SIN, SQRT, LOG, ...). Neither do
  the OTS$ routines VMS keeps there or beside it: OTS$POWxx
  (exponentiation) and complex division. FORTRAN's intrinsic functions
  and `**` need them. This is a VMS library the program adds
  (Decision 14).
- **FOR message texts**: needed for `%FOR-F-ENDDURREA, end-of-file
  during read`. Our own texts (Decision 15).
- **RMS**: no relative or indexed organization, and no keyed *write*
  ($PUT or $UPDATE by record number) on fixed-length sequential files.
  FORTRAN's direct-access WRITE needs the latter. ORGANIZATION='RELATIVE'
  and 'INDEXED' need the others. The author confirmed (2026-10-29) that
  changes to the `ods2` library, a peer project in `go.work`, are fair
  game if RMS needs them.
- **STR$**: most STR$ entries are stubs that signal NOTIMPL. The
  compiler will do character operations inline (MOVC3, MOVC5, CMPC5),
  so it doesn't need them, but a FORTRAN program calling STR$ routines
  would. They are filled in where a program needs them.
- **Phases 60 and 61** (LIB$ trees and TPARSE; LIB$ conditions and
  images) are still planned. Phase 61's LIB$FIXUP_FLT, LIB$FLT_UNDER,
  and LIB$SIG_TO_RET are things FORTRAN programs use; Phase 83 takes
  them if they're still missing.
- **A system definitions library for FORTRAN**: VMS's
  `SYS$LIBRARY:FORSYSDEF.TLB` (`INCLUDE '($SSDEF)'`) is licensed. govax
  generates its own (Decision 17).

## The language: VAX FORTRAN V5.0

The 1988 manual describes **ANSI FORTRAN-77** plus DEC's extensions.
Grouped by how much work each is:

- **FORTRAN-77 core:** program units (PROGRAM, SUBROUTINE, FUNCTION,
  BLOCK DATA, ENTRY, statement functions); types INTEGER, REAL, DOUBLE
  PRECISION, COMPLEX, LOGICAL, CHARACTER; IMPLICIT, DIMENSION, COMMON,
  EQUIVALENCE, DATA, PARAMETER, SAVE, EXTERNAL, INTRINSIC; arithmetic,
  character, relational, and logical expressions; assignment and
  ASSIGN; GO TO (unconditional, computed, assigned), arithmetic,
  logical, and block IF, DO, CONTINUE, STOP, PAUSE, RETURN (with
  alternate returns), END; READ, WRITE, PRINT, OPEN, CLOSE, INQUIRE,
  REWIND, BACKSPACE, ENDFILE, FORMAT; formatted, list-directed, and
  unformatted I/O; sequential and direct access; internal files.
- **DEC's extensions:** tab-format and 132-column lines, `!` comments,
  D lines; INCLUDE; names of 31 characters with `$` and `_`; `*n` type
  lengths (BYTE, INTEGER*2, LOGICAL*1, REAL*16, COMPLEX*16); DO WHILE,
  END DO, DO without a label; IMPLICIT NONE; octal, hexadecimal, and
  Hollerith constants; `.XOR.`; %VAL, %REF, %DESCR, %LOC; null
  arguments; STRUCTURE, RECORD, UNION, MAP; NAMELIST; VOLATILE;
  AUTOMATIC and STATIC; ACCEPT, TYPE, REWRITE, DELETE, UNLOCK; keyed
  READ (indexed files); ENCODE, DECODE, DEFINE FILE, FIND (appendix A);
  OPTIONS; CDEC$ PSECT, IDENT, TITLE, SUBTITLE.
- **Left out** (Decision 1): CPAR$ parallel directives, DICTIONARY (the
  CDD), and anything from Fortran 90 (free source form, modules, array
  expressions, pointers, allocatable arrays, derived types in the
  Fortran 90 sense).

# Part B — decisions (accepted 2026-10-29)

**Decision 1 — the dialect is VAX FORTRAN V5.0**, as the 1988 manual
describes it: FORTRAN-77 with DEC's extensions (the list above). The
`/F77` interpretation is the default. `/NOF77` (appendix A.6: one-trip
DO loops and the old EXTERNAL rule) is a late, optional subtask. Fortran
90 is out of scope. A feature the compiler doesn't support yet is a
compile-time error (`%FORT-E-NOTIMPL`), never silently wrong code
(Decision 19).

**Decision 2 — what "correct" means.** Faithful to:

- the language rules in the manual (types, conversions, evaluation, DO
  iteration counts, storage association);
- the run-time behavior the manual documents: list-directed output
  (Table 7-1), the edit descriptors (chapter 8), the OPEN defaults
  (Table 9-1), carriage control, and the FOR$_ condition values and
  IOSTAT numbers;
- the VAX calling standard (Decision 11), so FORTRAN calls VMS and VMS
  calls FORTRAN (handlers, ASTs, LIB$ callbacks).

Not faithful to, and free to differ: the object code and its
optimization, psect contents beyond what the manual names, the
compiler's messages and listing layout, and FTNRTL's internals and entry
points. Where the manual is silent, we make a choice and record it in
the phase's doc. The MANUAL.md chapter collects them as "Differences
from VAX FORTRAN". `docs/DEVIATIONS.md` stays for ISA and VMS fidelity
issues.

**Decision 3 — the compiler writes MACRO-32 source, and only that.**
The user runs MACRO and LINK. The `.MAR` file must assemble with govax's
MACRO with no errors and no warnings; that is an invariant every test
checks. It should also assemble with real VMS MACRO, since it is plain
MACRO-32 (an optional VMS round, Phase 86). There is no object file
writer and no debugger records of the FORTRAN source. Traceback and
`MACRO/DEBUG` work at the `.MAR` file's level, and the comments map it
back to the FORTRAN. One source file makes one `.MAR` file and one
object module, however many program units it holds, since a MACRO
source is one module. Each routine is a `.ENTRY` in it, and the module
is named for the first program unit.

**Decision 4 — the compiler is pure Go, in `internal/fortran` and its
subpackages**, one per pass, each with a `doc.go` that teaches it:

| Package | Pass |
|---|---|
| `internal/fortran` | the driver: `Compile(Options, Files) (Result, error)`, the pass order, the outputs |
| `internal/fortran/diag` | messages: severity, `%FORT-s-IDENT, text`, the source line and a caret |
| `internal/fortran/source` | source forms: lines into statements with positions; INCLUDE |
| `internal/fortran/lex` | statement classification and tokens |
| `internal/fortran/ast` | the syntax tree's node types |
| `internal/fortran/parse` | the parser |
| `internal/fortran/sema` | symbols, types, constants, storage layout, checks |
| `internal/fortran/ir` | the intermediate representation and the lowering into it |
| `internal/fortran/opt` | the optimizer's passes (Phase 84) |
| `internal/fortran/codegen` | instruction selection, registers, the MACRO writer |

The compiler imports `vaxfloat` (constant arithmetic), `vmsdef`
(definitions for INCLUDE), and `lbr` (text libraries). It imports
nothing of the machine, the console, or RMS. It reads files through a
small interface the FORTRAN command supplies, so it is tested on
in-memory sources. `internal/dclcmd/fortran.go` is the command.

**Decision 5 — lexing classifies a statement before tokenizing it.** In
fixed form, FORTRAN ignores blanks, and its keywords aren't reserved:
`DO 10 I = 1.5` assigns 1.5 to the variable `DO10I`, while
`DO 10 I = 1,5` is a loop. `IF(X) = 3` is an assignment to an array
element named IF. So the lexer first strips blanks outside character
constants. It then decides the statement's kind by looking for an `=`
at parenthesis level 0 and a `,` after it (the classic test), and only
then splits the text into tokens, knowing where keywords may be. This
is the one place FORTRAN differs most from the languages most compiler
texts use, and its doc comment explains it with these examples.

**Decision 6 — a hand-written recursive-descent parser** with
precedence climbing for expressions (FORTRAN's precedence: `**`, which
is right-associative; `*` and `/`; unary and binary `+` and `-`; `//`;
relational operators; `.NOT.`; `.AND.`; `.OR.`; `.EQV.`, `.NEQV.`, and
`.XOR.`). Each statement kind is a function. The AST keeps every
statement's source position, and its FORMAT statements as text: FTNRTL
interprets formats (Decision 12), and the compiler checks their syntax
so errors appear at compile time.

**Decision 7 — semantic analysis builds one symbol table per program
unit** and does what FORTRAN leaves to declarations: implicit typing
(I–N integer, the rest real, or IMPLICIT's rules); PARAMETER constants
and constant expressions, folded with `vaxfloat`; arrays and their
bounds (constant, adjustable, assumed-size); COMMON and EQUIVALENCE
storage, solved as offsets from each block's base (an EQUIVALENCE that
extends a COMMON block forward lengthens it, and one that extends it
backward is an error); DATA; SAVE; statement functions; and the generic
intrinsics resolved to specific ones by argument type (Table D-3). It
also checks labels (DO termination, jumps into blocks, FORMAT labels)
and argument consistency between units in the same file.

**Decision 8 — the IR is typed expression trees in basic blocks.**
Lowering makes every implicit thing explicit:

- array subscripts become address arithmetic (column-major order, lower
  bounds);
- DO loops become a trip count computed on entry (the manual's
  5.3.1.1: `MAX(INT((m2-m1+m3)/m3), 0)`), so zero-trip loops and REAL
  loop variables are right;
- character assignment and comparison become moves with blank fill;
- I/O statements become FTNRTL call sequences (Decision 12);
- logical IFs become conditional branches.

The code generator walks the trees and picks VAX instructions and
addressing modes: three-operand arithmetic (`ADDL3`), indexed mode
(`base[Rx]`) for array elements, displacement-deferred mode for dummy
arguments (`@4(AP)`), `CLRL`, `INCL`, `MOVAL`, and so on. Registers come
from a Sethi–Ullman count of each tree's needs. Branches are the
generator's job too: MACRO doesn't lengthen a branch whose target is out
of range, so the generator emits a long form (`BRW`, `JMP`, or an
inverted branch around one) whenever the distance isn't known to be
short.

**Decision 9 — optimization is separate, optional, and later** (Phase
84). Without it, the generator's code is straightforward and correct.
With `/OPTIMIZE`, IR passes do constant propagation, dead code removal,
common subexpressions within a block, loop-invariant code motion,
strength reduction of subscripts, and allocation of R6–R11 to loop
variables and frequently used addresses; a peephole pass uses VAX loop
instructions (`AOBLEQ`, `SOBGTR`, `ACBL`). Every test program runs both
ways and must print the same thing. `/NOOPTIMIZE` stays the default
until that holds for the whole corpus, then `/OPTIMIZE` becomes it, as
in VAX FORTRAN (the review's answer 7).

**Decision 10 — storage is static, as VAX FORTRAN's is by default.** A
program unit's variables and the compiler's temporaries live in a
writable psect, not on the stack. That follows FORTRAN-77 (a program
unit isn't reentrant) and makes DATA trivial. AUTOMATIC (Phase 82)
puts a variable in the stack frame. The psects:

| Psect | Holds | Attributes |
|---|---|---|
| `$CODE` | instructions | PIC, SHR, EXE, NOWRT |
| `$PDATA` | constants, descriptors, formats | PIC, SHR, NOEXE, NOWRT |
| `$LOCAL` | variables, temporaries | PIC, NOSHR, NOEXE, WRT |
| `name` (each COMMON block) | the block | Table 10-1: OVR, GBL, SHR, WRT, LONG |
| `$BLANK` | blank COMMON | as a COMMON block |

**Names** in the `.MAR` file: a routine's entry is its FORTRAN name, a
global symbol; its storage is local symbols named `unit.name`
(`SQUARES.I`). MACRO allows a period in a local symbol, and FORTRAN
names can't contain one, so the names never collide, and `MACRO/DEBUG`
users can `EXAMINE SQUARES.I`. Labels are `unit.Lnnn` (FORTRAN label
nnn) and `unit.$n` (the compiler's own). Names that would pass MACRO's
31 characters get a numbered form, listed in the storage map comment.

**Decision 11 — the VAX calling standard.** Every routine is a
`.ENTRY` with a register mask, called by CALLS or CALLG:

- arguments are passed by reference, except CHARACTER (by descriptor,
  class S) and the mechanisms %VAL, %REF, and %DESCR select (Table 6-1).
  Null arguments (`CALL X(A,,B)`) pass 0. %LOC is an address;
- a function's value comes back in R0 (INTEGER, LOGICAL, REAL*4), R0:R1
  (REAL*8, COMPLEX*8), or through a hidden first argument (CHARACTER, as
  a descriptor; COMPLEX*16 and REAL*16, by reference);
- a subroutine with alternate returns returns the index in R0, and the
  caller dispatches with CASEL;
- R0 and R1 are scratch. A routine saves the R2–R11 it uses in its entry
  mask. The character instructions destroy R0–R5, which the register
  allocator knows;
- a main program is a `.ENTRY` named for its PROGRAM statement (or
  `MAIN`), the transfer address in `.END`, returning SS$_NORMAL in R0.

This is what makes a FORTRAN program able to call any system service or
RTL routine, and makes FORTRAN routines usable as condition handlers,
AST routines, and callbacks.

**Decision 12 — the run-time interface is a call per I/O step, with
descriptors.** An I/O statement becomes:

1. a *begin* call naming the statement's kind (PRINT, list-directed
   READ, formatted WRITE, ...), with a control block in `$PDATA` holding
   the unit, the format, REC=, and which of ERR=, END=, and IOSTAT= the
   statement has (and IOSTAT's address);
2. one *item* call per I/O list element: `FTN$IO_ITEM(desc)` with a VMS
   descriptor whose DTYPE and LENGTH say what the item is. A whole array
   is one call with a class A descriptor. Since storage is static, most
   descriptors are constants in `$PDATA`; a dummy argument's descriptor
   is filled in at run time. Implied-DO lists are ordinary loops around
   item calls;
3. an *end* call, which writes or finishes the record and returns the
   statement's status in R0.

When the statement has ERR=, END=, or IOSTAT=, FTNRTL stores IOSTAT,
lets later item calls do nothing after an error, and returns the status;
the compiled code branches. Without them, FTNRTL signals the FOR$_
condition itself (LIB$SIGNAL or LIB$STOP by severity), so the program
ends with `%FOR-F-...` and a traceback, as on VMS. Formats are
interpreted at run time from their text (run-time formats in character
variables need that anyway). Each FORMAT's control block has a longword
where FTNRTL caches the parsed form.

**Decision 13 — FTNRTL is MACRO-32 we write, built by govax's own
MACRO and LINK, and always available.**

- **Where:** a new package `internal/ftnrtl`. It holds the MACRO
  sources (`mar/*.mar`: one module per subject, such as units, formats,
  list-directed I/O, conversions, errors, and the system subroutines),
  the options file, and `ftnrtl.exe`. `go generate ./internal/ftnrtl`
  builds the image with `internal/asm` and `internal/link`, the same code
  as the MACRO and LINK commands. The image is committed and embedded
  (`go:embed`), like `bootdata`'s STARLET.MLB. A test checks that a fresh
  build matches the committed image, with the link time pinned.
- **Found:** the image locator's last place to look, after a logical
  name, `SYS$SHARE:`, and the host library directory, is the embedded
  image. So RUN finds it, and LINK resolves `FTN$` routines from it
  without an options file: govax's own symbol source names FTNRTL as it
  names LIBRTL. Someone changing FTNRTL can build their own with MACRO
  and `LINK/SHAREABLE` and `DEFINE FTNRTL` to it; the locator already
  prefers the logical name.
- **A transfer vector** (`.TRANSFER` and `.MASK`) keeps each entry's
  offset fixed as FTNRTL grows, and GSMATCH=LEQUAL lets a program linked
  against an older FTNRTL run on a newer one.
- **Names:** entries are `FTN$...`, our own facility prefix, distinct
  from VMS FORRTL's `FOR$`. Condition values are VAX FORTRAN's `FOR$_`
  values (Decision 15).
- **What it uses:** RMS services directly ($OPEN, $CREATE, $CONNECT,
  $GET, $PUT, ...), LIBRTL (LIB$GET_VM, LIB$SIGNAL, LIB$STOP,
  LIB$DATE_TIME, OTS$'s conversions; floating output through
  `OTS$$CVT_x_T_R8`, govax's own Go routine since Phase 72), and system
  services ($DCLEXH for an exit handler that closes the units, $EXIT).
  Its writable data (the unit table) is a NOSHR psect, so each process
  gets its own copy.
- **Its comments** follow the same teaching standard as the Go code.

**Decision 14 — MTHRTL is a VMS library, emulated in Go like LIBRTL.**
The author's direction (2026-10-29): the MTH$ functions extend govax's
RTL support, usable by any program (MACRO, our FORTRAN, or an image
VMS's compilers made), and are not embedded in FTNRTL or tied to the
compiler. A new package `internal/mthrtl`, following CLAUDE.md's "each
further *RTL.EXE emulated gets a package like it", registers MTH$ routines in
all four floating formats (MTH$SQRT, MTH$DSQRT, MTH$GSQRT, MTH$HSQRT,
...), the complex ones (MTH$CABS, MTH$CEXP, ...), and the OTS$ powers
(OTS$POWII, OTS$POWRR, OTS$POWDJ, ...) and complex division, with the
manual's condition signals (MTH$_LOGZERNEG, MTH$_SQUROONEG, ...: the
texts exist). Accuracy: F and G are computed in float64, which holds
them exactly, and rounded once to the format. D (56 bits) and H (113
bits) need more than float64, and are computed with `math/big` series
to within an ulp. MTHRTL's vector is VMS's, captured from VMS 7.1's
MTHRTL.EXE on the local system disk with `vmsdef/gen -image` as
LIBRTL's was, so VMS-compiled images that call MTHRTL also work (the
review's answer 2).

**Decision 15 — FOR messages are govax's own text, under VAX FORTRAN's
condition values.** `vmsdef` has the FOR$_ values; a hand-written
`internal/vmsdef` table adds the FOR facility's messages (our wording,
FAO arguments for the unit and file name), so `$GETMSG` and `$PUTMSG`,
and therefore traceback, print `%FOR-F-INPCONERR, input conversion
error`. IOSTAT values are the message numbers (`FOR$_ENDDURREA` gives
24), so programs that test IOSTAT against VAX FORTRAN's documented
numbers work, and ERRSNS returns real condition values.

**Decision 16 — floating point.** REAL*8 is D_floating unless
`/G_FLOATING`; REAL*16 is H_floating. Constants are converted at
compile time with `vaxfloat` and written in the `.MAR` file in hex
(`.LONG ^X...`), with the source's spelling in a comment. MACRO isn't
asked to convert them again, so the bits are certainly the compiler's.
Folded expressions round as the CPU does.

**Decision 17 — INCLUDE and the system definitions.** `INCLUDE 'file'`
reads a file (default type `.FOR`) from the host or a volume.
`INCLUDE '(module)'` reads a module of a text library: those named with
`/LIBRARY`, then `FORT$LIBRARY`, then `SYS$LIBRARY:FORSYSDEF.TLB` if
there is one, and last govax's own modules, generated in Go from
`vmsdef` (`$SSDEF`, `$IODEF`, `$JPIDEF`, `$SYIDEF`, `$DVIDEF`,
`$RMSDEF`, `$LIBDEF`, `$FORDEF`, `$DSCDEF`, `$STSDEF`, `$LNMDEF`,
`$SECDEF`, `$PSLDEF`, `$PRVDEF`, and `$SYSSRVNAM`, which declares each
service as an INTEGER*4 function), as PARAMETER statements. LIBRARY
gains `/TEXT` if a phase needs it to build libraries.

**Decision 18 — the FORTRAN command.** `FORTRAN file[,...]` compiles
each file to a `.MAR` named for the source, written where MACRO's
`/OBJECT` writes its file; `file+file` compiles the concatenation. Each
qualifier arrives with the phase that needs it:

| Qualifier | Phase |
|---|---|
| `/MACRO[=file]`, `/NOMACRO` (check only), `/[NO]EXTEND_SOURCE`, `/[NO]D_LINES`, `/CONTINUATIONS=n` | 74 |
| `/[NO]G_FLOATING`, `/[NO]I4` | 78 |
| `/LIBRARY` | 83 |
| `/[NO]OPTIMIZE` | 84 |
| `/[NO]LIST`, `/SHOW`, `/CHECK`, `/WARNINGS`, `/STANDARD`, `/[NO]DEBUG`, `/[NO]OBJECT` (runs MACRO; off by default) | 85 |

`$STATUS` is the worst message's severity, and no `.MAR` file is
written when there are errors, as MACRO writes no object. `govax
fortran` is the one-shot form, as `govax macro` is. `HELP FORTRAN`
documents the command.

**Decision 19 — diagnostics.** A message names the file, line, and
column, quotes the line, and points at the column:

```
%FORT-E-UNDEFLBL, statement label 20 is never defined
  at line 14 of DUA0:[WORK]SQUARES.FOR;1
          GO TO 20
          ------^
```

The identifiers are ours. The compiler goes on after an error where it
can, so one run reports many. A feature not yet implemented is
`%FORT-E-NOTIMPL`, naming it.

**Decision 20 — testing.**

- **Each pass is unit-tested** on its own: source forms, tokens, AST
  dumps (golden files), symbol tables and storage maps, IR dumps, and
  code-generation snippets.
- **A corpus of programs**, `testdata/fortran/`, each `name.for` with
  its expected `name.out` (and `name.in` for input), compiled, assembled,
  linked, and run on govax by a Go test (`TestFortranPrograms` in
  `internal/console`). Every phase adds programs for what it built, and
  every program stays. Every generated `.MAR` must assemble without a
  message.
- **Both paging modes** for a subset, and **both optimizer settings**
  for all, from Phase 84.
- **FTNRTL's and MTHRTL's routines** are also tested directly, from
  MACRO test programs, as LIBRTL's are.
- **Optionally, VMS:** VMS's MACRO can assemble our `.MAR` files and
  FTNRTL, to run our code on real VMS (Phase 86). There is no VAX
  FORTRAN compiler on the simh VAX (the review's answer 5), so our
  run-time output isn't compared with VAX FORTRAN's.

**Decision 21 — what the program finds missing elsewhere, it fixes.**
A bug found in MACRO, LINK, RMS, the CPU, or the services is fixed in
the subtask that found it, with a test, and logged in that phase's
progress log (and DEVIATIONS.md if it is a fidelity issue). A missing
system service, LIB$ routine, or RMS feature that a FORTRAN program or
FTNRTL needs is added. That includes the `ods2` library: a change there
is made in the sibling checkout, tagged and pushed, and pinned with
`GOWORK=off go get github.com/tucats/ods2@vX.Y.Z` before govax is
pushed (CLAUDE.md's procedure).

**Decision 22 — the code teaches.** Beyond the usual commenting
standard:

- each compiler package's `doc.go` is a short tutorial on its pass,
  with a worked example from FORTRAN to its output;
- comments explain FORTRAN's peculiarities where the code handles them
  (blank-insensitive lexing, storage association, DO trip counts, the
  Hollerith past) and the VAX's where the generator uses them
  (addressing modes, CALLS frames, why MOVC5 destroys R0–R5);
- `docs/FORTRAN-INTERNALS.md` grows phase by phase into a guided tour
  of the compiler and FTNRTL for someone who wants to write a compiler
  for the VAX under VMS. MANUAL.md gains a user's chapter.

## A sketch of the generated MACRO

For this program:

```fortran
      PROGRAM SQUARES
      INTEGER I
      DO 10 I = 1, 5
         PRINT *, I, I*I
10    CONTINUE
      END
```

the `.MAR` file would look roughly like this (the details are settled
in Phase 75):

```
        .TITLE  SQUARES
        .IDENT  /01/
;
; Compiled by govax FORTRAN V1.0-nnn on 29-OCT-2026 14:05:11.20
; from DUA0:[WORK]SQUARES.FOR;1, /NOOPTIMIZE
;
; Storage map for SQUARES
;   I        INTEGER*4   $LOCAL+0    SQUARES.I
;
        .PSECT  $LOCAL,PIC,CON,REL,LCL,NOSHR,NOEXE,RD,WRT,LONG
SQUARES.I:      .BLKL   1               ; I
SQUARES.$T1:    .BLKL   1               ; temporary for I*I
SQUARES.$C1:    .BLKL   1               ; DO 10's trip count

        .PSECT  $PDATA,PIC,CON,REL,LCL,SHR,NOEXE,RD,NOWRT,LONG
SQUARES.$D1:    .WORD   4               ; descriptor of I: 4 bytes,
                .BYTE   DSC$K_DTYPE_L   ; a longword integer,
                .BYTE   DSC$K_CLASS_S   ; scalar,
                .ADDRESS SQUARES.I      ; at I
SQUARES.$D2:    ...                     ; descriptor of the temporary

        .PSECT  $CODE,PIC,CON,REL,LCL,SHR,EXE,RD,NOWRT,LONG
;      1        PROGRAM SQUARES
        .ENTRY  SQUARES,^M<>            ; the main program
;      2        INTEGER I
;      3        DO 10 I = 1, 5
        MOVL    #1,SQUARES.I            ; I = 1
        MOVL    #5,SQUARES.$C1          ; trip count: MAX((5-1+1)/1, 0)
        BLEQ    SQUARES.L10$END         ; no trips
SQUARES.$1:                             ; top of DO 10
;      4           PRINT *, I, I*I
        CALLS   #0,G^FTN$PRINT_LIST     ; begin a list-directed PRINT
        PUSHAQ  SQUARES.$D1             ; item 1: I
        CALLS   #1,G^FTN$IO_ITEM
        MULL3   SQUARES.I,SQUARES.I,SQUARES.$T1 ; item 2: I*I
        PUSHAQ  SQUARES.$D2
        CALLS   #1,G^FTN$IO_ITEM
        CALLS   #0,G^FTN$IO_END         ; write the record
;      5  10    CONTINUE
SQUARES.L10:
        INCL    SQUARES.I               ; I = I + 1
        SOBGTR  SQUARES.$C1,SQUARES.$1  ; another trip?
SQUARES.L10$END:
;      6        END
        MOVL    #SS$_NORMAL,R0          ; a main program's status
        RET
        .END    SQUARES
```

# Part C — the phases

| Phase | Subject | Builds on |
|---|---|---|
| **74** | The plan (this doc); the FORTRAN command; source forms and diagnostics; FTNRTL's build and its place in RUN and LINK | — |
| **75** | First light: every pass for a small subset, list-directed PRINT, and the end-to-end test harness | 74 |
| **76** | The whole front end: every statement parsed; declarations and storage (types, IMPLICIT, PARAMETER, arrays, COMMON, EQUIVALENCE, DATA, SAVE, BLOCK DATA) | 75 |
| **77** | MTHRTL: the MTH$ routines, the OTS$ powers, and complex arithmetic, emulated in Go | 74 (independent of 75–76) |
| **78** | Expressions and data types: every type and length, conversions, CHARACTER and substrings, COMPLEX, the intrinsic functions | 76, 77 |
| **79** | Control flow and subprograms: every GO TO, IF, and DO form; functions of every type, ENTRY, statement functions, alternate returns, adjustable arrays, passed-length characters, procedure arguments, %VAL/%REF/%DESCR/%LOC | 78 |
| **80** | Formatted and list-directed I/O: FORMAT and every edit descriptor, run-time formats, list-directed input, internal files, NAMELIST, ENCODE/DECODE, ACCEPT/TYPE, the error model (ERR, END, IOSTAT, FOR messages, ERRSNS) | 79 |
| **81** | Files: OPEN, CLOSE, INQUIRE, REWIND, BACKSPACE, ENDFILE, unformatted and direct access, relative files, DEFINE FILE and FIND; the RMS (and ods2) additions they need | 80 |
| **82** | DEC's data extensions: STRUCTURE, RECORD, UNION, MAP; typeless and Hollerith constants; VOLATILE, AUTOMATIC, STATIC; OPTIONS; CDEC$ directives | 79 |
| **83** | The VMS interface: INCLUDE of text library modules and govax's system definitions; calling system services and RTL routines; condition handlers, ASTs, and exit handlers in FORTRAN; the LIB$ routines this needs | 81, 82 |
| **84** | The optimizer, `/OPTIMIZE` | 83 |
| **85** | Compiler usability: listings, cross references, run-time checks, warnings, standards checking, `/DEBUG` | 80 |
| **86** | The milestone and close-out: a substantial program compiled and run, the manual's chapter, the internals tour, and an optional VMS round | all |

Indexed files (ORGANIZATION='INDEXED', keyed READ, REWRITE, DELETE,
UNLOCK on them) are not in the table: the review deferred them to a
later program (answer 4), since RMS's indexed organization is a large
piece of work on its own.

Each later phase has a planning doc of its own: its goal, what earlier
phases leave in place, its design notes, its open questions, and its
provisional subtasks. The survey that starts each phase may change them.

- [PHASE-75 - FORTRAN first light](PHASE-75%20-%20FORTRAN%20first%20light.md)
- [PHASE-76 - FORTRAN front end](PHASE-76%20-%20FORTRAN%20front%20end.md)
- [PHASE-77 - MTHRTL](PHASE-77%20-%20MTHRTL.md)
- [PHASE-78 - FORTRAN expressions and types](PHASE-78%20-%20FORTRAN%20expressions%20and%20types.md)
- [PHASE-79 - FORTRAN control flow and subprograms](PHASE-79%20-%20FORTRAN%20control%20flow%20and%20subprograms.md)
- [PHASE-80 - FORTRAN formatted IO](PHASE-80%20-%20FORTRAN%20formatted%20IO.md)
- [PHASE-81 - FORTRAN files](PHASE-81%20-%20FORTRAN%20files.md)
- [PHASE-82 - FORTRAN DEC extensions](PHASE-82%20-%20FORTRAN%20DEC%20extensions.md)
- [PHASE-83 - FORTRAN and VMS](PHASE-83%20-%20FORTRAN%20and%20VMS.md)
- [PHASE-84 - FORTRAN optimizer](PHASE-84%20-%20FORTRAN%20optimizer.md)
- [PHASE-85 - FORTRAN compiler usability](PHASE-85%20-%20FORTRAN%20compiler%20usability.md)
- [PHASE-86 - FORTRAN milestone](PHASE-86%20-%20FORTRAN%20milestone.md)

Each subtask is one commit with its tests, and the work stops for
review after each.

## Phase 74 — groundwork subtasks (this phase)

1. **The survey and plan** (this doc, the skeletal docs of Phases 75 to
   86, PLAN.md's rows, CLAUDE.md's overview, the 1988 manual's text):
   written 2026-10-29, for review.
2. **The compiler's skeleton and the FORTRAN command** (done
   2026-10-29): the driver
   (`internal/fortran`: `Options`, `Compile`, `Result`), `diag`, the
   FORTRAN verb in `console.dcl` with Decision 18's first qualifiers,
   `dclcmd.Context.Fortran` reading the source from the host or a volume
   and writing the `.MAR` file, `govax fortran`, `HELP FORTRAN`. The
   compiler at this stage writes a header, each source line as a comment,
   and `.END`: an empty module MACRO assembles cleanly. Tests on host and
   volume files.
3. **Source forms** (`internal/fortran/source`): fixed-format lines
   (label field, continuation column, statement field, sequence field),
   tab-format lines, `/EXTEND_SOURCE`, comment lines (`C`, `*`, `!`),
   D lines, `!` comments outside character constants, continuation
   limits (`/CONTINUATIONS=n`, 0 to 99, default 19: the User Manual's
   numbers as remembered, unconfirmed; DUNGEON's VMS build uses
   `/CONT=40`), labels, and INCLUDE of files (nested, with a depth limit),
   each statement with its file, line, and column for messages and for
   the `.MAR` comments.
4. **FTNRTL's pipeline** (`internal/ftnrtl`): the package, its first
   module (a version routine, and FTN$STOP), the transfer vector, the
   generator, the embedded image; the locator's fallback for RUN and
   LINK; LINK's govax source naming FTNRTL. A MACRO program calls FTNRTL,
   links with no options file, and runs, in both paging modes, and the
   committed image matches a fresh build. MACRO and LINK fixes as found.
5. **Close-out:** CLAUDE.md's package descriptions (`internal/fortran`,
   `internal/ftnrtl`), HELP, MANUAL.md's new chapter's skeleton,
   `docs/FORTRAN-INTERNALS.md` begun, PLAN.md.

## Future work (not planned)

- An object file writer, so FORTRAN needn't go through MACRO.
- Debugger records of the FORTRAN source (EXAMINE by FORTRAN name, STEP
  by FORTRAN line, SET LANGUAGE FORTRAN).
- Fortran 90 and later features.
- A FORTRAN-77 compatible FORRTL, so VMS-compiled FORTRAN images run
  without VMS's FORRTL.EXE.
- RECURSIVE routines (VAX FORTRAN V5 had none).
- RMS indexed files, and FORTRAN's keyed I/O on them (deferred by the
  review).

## The review (2026-10-29)

The questions the plan asked, with the author's answers; the decisions
and the later phases' docs have been changed to follow them.

1. **MTHRTL in Go or in MACRO?** **Answer** (given during planning): the
   MTH$ functions extend govax's RTL support, usable inside and outside
   FORTRAN, not embedded in FTNRTL. Decision 14 (Go, `internal/mthrtl`).
2. **MTHRTL's vector:** capture VMS 7.1's MTHRTL.EXE global symbol table
   from the local system disk (`testdata/disks/rq0-ra92.dsk`) with
   `vmsdef/gen -image`, as LIBRTL's was, or give govax's MTHRTL its own
   layout? **Answer** Yes, capture VMS's (Decision 14, Phase 77).
3. **`/OBJECT`:** should FORTRAN be able to run MACRO for the user,
   keeping the `.MAR` file? **Answer** Yes: an opt-in qualifier in
   Phase 85; by default FORTRAN writes only the `.MAR` file (Decision 18).
4. **Indexed files:** in this program, or a later RMS program?
   **Answer** Defer to later. Phase 81 keeps relative files and direct
   access on sequential files; OPEN with ORGANIZATION='INDEXED' fails
   with a FOR$_ error until a later program adds the organization.
5. **A VAX FORTRAN compiler on the simh VAX?** **Answer** There isn't
   one. Run-time choices the manual leaves open stand as ours, marked
   unconfirmed in each phase's doc and collected in the manual's
   "Differences from VAX FORTRAN". VMS's MACRO and LINK can still check
   our generated code on a real VAX (Phase 86's optional round).
6. **A milestone program?** **Answer** None in mind yet; the author
   likes DUNGEON, and will look for its FORTRAN source from a freely
   available place and check that its license allows our use. Until
   then, Phase 86 plans for DUNGEON with classic programs as the
   fallback. *Later the same day* the author placed DUNGEON V3.2A's
   source, cleared as freely available, in
   `~/Documents/Technical Doc/VMS/dungeon-3.2A/`; Phase 86's doc has
   what it needs.
7. **`/OPTIMIZE` as the default** once the optimizer passes the whole
   corpus both ways? **Answer** Agreed (Decision 9, Phase 84).
8. **Names:** `FTN$` for FTNRTL's entries, `%FORT-` for the compiler's
   messages, `internal/fortran`, `internal/ftnrtl`, `internal/mthrtl`.
   **Answer** Agreed.

## Progress log

- **2026-10-29:** Survey written (Part A), decisions proposed (Part B),
  and the phases laid out (Part C), with skeletal docs for Phases 75 to
  86. The 1988 *VAX FORTRAN Language Reference Manual*'s OCR text saved
  beside its PDF. During the survey the author added that changes to the
  `ods2` library are fair game when RMS needs them (Decision 21), and
  that the MTH$ functions must be general RTL support usable outside
  FORTRAN too, not part of FTNRTL (Decision 14; question 1 answered).
  For review before any implementation.
- **2026-10-29:** The author reviewed the plan and answered its
  questions (under "The review"): MTHRTL's vector captured from VMS 7.1;
  `/OBJECT` as an opt-in qualifier; indexed files deferred to a later
  program; no VAX FORTRAN compiler is available for output checks;
  DUNGEON's source is the hoped-for milestone, pending a freely usable
  copy; `/OPTIMIZE` becomes the default once the corpus passes both
  ways; the names accepted. The decisions and Phases 77, 81, 85, and 86
  were updated to match. Subtask 1 is done; subtask 2 is next.
- **2026-10-29:** Subtask 2 done: the compiler's skeleton and the
  FORTRAN command.
  - `internal/fortran` is the driver: `Options` (the qualifiers, the
    version, the compile time, the command line), `File` (the source's
    specification, name, and lines), `Result` (the MACRO file's lines,
    nil after an error, and the messages with their counts), and
    `Compile`, which so far writes the header, each source line as a
    comment, and `.END`. The module is named for the source file
    (`ModuleName`: capitals, `_` for a character MACRO can't take, `F_`
    before a leading digit, 31 characters) until the compiler can name it
    for its first program unit.
  - `internal/fortran/diag` is Decision 19's messages: `Severity` (I, W,
    E, F, with their VMS status codes), `Position`, `Message` (printed
    with the line quoted and a pointer that copies the line's tabs, so it
    lines up under tab-format lines), `List` (`Report` by identifier,
    `Worst`, `Failed`), and the table of messages (`messages.go`), which
    holds only NOTIMPL so far; later phases add theirs.
  - `internal/fortran/codegen` starts with `Writer`, the MACRO file's
    layout: statements in MACRO's columns (opcode at 8, operands at 16,
    comments at 40), a label of seven characters or more on its own
    line, comment lines split at blanks, and `SourceLine`, whose
    8-character prefix (`;`, the number in five columns, two blanks) is
    one tab stop, so a tab-format source line keeps its layout. Every
    line is kept within MACRO's 132 characters (the VAX MACRO manual's
    limit): a long source line continues on lines prefixed `;    -  `,
    and a statement's comment that won't fit goes on the line before.
    Control characters become `?`.
  - The command is `internal/dclcmd/fortran.go` (`FortranOptions`,
    `Context.Fortran`), bound in `internal/console/dispatch.go`, with
    the verb in `console.dcl` (ids 1370 to 1375): a list of sources,
    `/HOST`, `/[NO]MACRO[=file]`, `/[NO]EXTEND_SOURCE`, `/[NO]D_LINES`.
    It follows MACRO's file rules (the MACRO file beside the source, in
    the source's case, a VMS name without a type given MAR). New
    statuses in `vmserrors`: CLI_FORERRORS (no MACRO file after errors),
    CLI_FORWARNINGS ($STATUS after warnings, not shown), and
    CLI_FORSAMEFILE, which refuses a host source whose default MACRO
    file would be itself (a `.MAR` source), a choice of ours. With
    several sources each failure is shown as it happens, the rest are
    still compiled, and `$STATUS` is the last failure's.
  - `govax fortran SOURCE... [--macro FILE | --no-macro]
    [--extend-source] [--d-lines]`; `HELP FORTRAN` (keys `FORT` and
    `FOR`) and the HELP index; MANUAL.md's command reference (6.5) and
    the subcommand table.
  - Left for later: Decision 18's `file+file` concatenation (with the
    `+LIB/LIBRARY` form, Phase 83), and `/EXTEND_SOURCE` and `/D_LINES`
    having an effect (subtask 3; they are accepted and passed to the
    compiler now).
  - Tests: `diag`'s and `codegen`'s units, `TestCompileSkeleton` (the
    whole file for a small program), the console's `TestFortran_*` (host
    and volume sources, names, versions, `/MACRO=`, `/NOMACRO`, the
    failures, and DCL's dispatch with a list and `$STATUS`), each MACRO
    file assembled by MACRO without a message; `cmd/govax`'s
    `TestFortranCommand` and `TestFortranThenMacro`; the grammar tests'
    verb count and the grammar split.
- **2026-10-29:** The milestone's source arrived: DUNGEON V3.2A
  (1-Oct-94), in `~/Documents/Technical Doc/VMS/dungeon-3.2A/`. It is
  written for VAX FORTRAN on OpenVMS (its `dungeon.doc` gives the VMS
  build: `FORT/NOCHECK/CONT=40`), so subtask 3 adds `/CONTINUATIONS=n`.
  Phase 86's doc lists what else it needs.
