# Phase 89 — FORTRAN: recursive and reentrant routines

**Status:** in progress. Planned 2026-11-04, with subtask 1, the survey (Part A
below), which the author reviewed the same day, taking every recommendation ("The
author's answers"); subtask 2, the syntax, done 2026-11-04; subtask 3, the storage rule, done 2026-11-04; subtask 4, the compiler's cells in the frame, done 2026-11-04. Subtask 5, frame addresses in data, done 2026-11-04. Subtask 6, variable format expressions, done 2026-11-04. Needs Phases 74 to 88 (the FORTRAN program, and the
frame area Phase 82 gave AUTOMATIC).
Follows the pattern of the FORTRAN program laid out in `PHASE-74 - FORTRAN plan.md`;
its "Future work" listed "RECURSIVE routines (VAX FORTRAN V5 had none)", and
Phases 82, 83, and 86 each left "routines really reentrant" for a later phase.

## Goal

A FORTRAN routine that can be active more than once at the same time, by
recursion (direct or through other routines) or by an AST or condition handler
that calls it while the main line is in it, each activation with storage of its
own, as the later DEC dialect has it (the *HP Fortran for OpenVMS Language
Reference Manual*, V8.0, 2005: its 5.3, 5.17, 8.5.1.1, 8.5.2.1, 8.11, and 13.3):

- **RECURSIVE** on a FUNCTION or SUBROUTINE statement, in either order with a
  type (`RECURSIVE INTEGER FUNCTION F(N)`, `INTEGER RECURSIVE FUNCTION F(N)`);
- **RESULT(name)** on FUNCTION (and ENTRY), so a function's name in its body is a
  call and the result is another variable;
- **`/[NO]RECURSIVE`** on the FORTRAN command, and on the OPTIONS statement, for
  every unit of a file or of one unit;
- **the storage rule:** a recursive unit's local variables are in the stack frame
  unless SAVE'd, given initial values, in COMMON, or STATIC; the compiler's own
  cells (temporaries, DO counts, descriptors, ENTRY's argument cells, ...) are in
  the frame too, so nothing an activation uses is shared but what the program
  says is shared;
- **the run-time side:** FTNRTL's statements nesting through recursive I/O, its
  calls of variable format expressions reaching the right activation, and the
  debugger telling the activations apart (SET SCOPE).

Fidelity is to the language rules, as in every FORTRAN phase (Phase 74, Decision
1). There is no DEC compiler to compare against (Phase 74's answer 5); the HP
manual is the source, read for the DEC-dialect rules only, as Phase 82 read it.

## What earlier phases leave in place

- **Phase 79:** the RECURSIVE message (a unit calling itself, section 6.2), ENTRY
  through argument cells (`$An`) and a selector (`$E`), adjustable arrays through
  descriptor cells (`$Bn`), hidden results, alternate returns.
- **Phase 80:** FTNRTL's statement contexts on a stack (nested I/O, RECIO_OPE on
  the same unit); variable format expressions as a routine FTNRTL calls with the
  caller's AP.
- **Phase 82:** AUTOMATIC: the frame area (`Unit.FrameSize`, `SUBL2 #n,SP` at each
  entry point, FP-relative cells, `Cell.Frame`), the `$FP` cell for a variable
  format expression that names an AUTOMATIC variable, the descriptor of a frame
  variable given its address at run time.
- **Phase 83:** FTNRTL holding ASTs off while its stack and free list change, and
  popping the contexts of statements left by a signal.
- **Phase 84:** the optimizer's `Storage` (what a call may change), whose comment
  rests on section 6.2's "no routine runs twice at once".
- **Phase 85:** the debugger's FORTRAN names, arguments and AUTOMATIC variables
  found in "the innermost active frame of a routine in its unit"; OPTIONS's
  qualifiers (`source/options.go`).

# Part A — the survey (2026-11-04)

"The 1988 manual" is the *VAX FORTRAN Language Reference Manual* (OCR text
`~/Documents/Technical Doc/VMS/vax-fortran-lrm-1988.txt`); "the HP manual" is
`VMS-FORTRAN-LANGUAGE.txt` in the same folder. As in earlier surveys, what a rule
means for a pass is in brackets, and a rule the manuals leave open is our choice,
marked unconfirmed, for MANUAL.md's 14.7.

## The language

**The 1988 manual forbids it.** Section 6.2: "A subprogram can refer to other
subprograms, but it cannot refer to itself, either directly or indirectly." It
has no RECURSIVE and no AUTOMATIC. govax reports a direct self-reference
(RECURSIVE, `sema/expr.go`, `exec.go`, `stmtfunc.go`); an indirect one through
another file is never seen.

**The HP manual permits it, by request.** 8.5.1.1: "Recursion is permitted if the
keyword RECURSIVE is specified in a FUNCTION or SUBROUTINE statement, or if
RECURSIVE is specified as a compiler option or in an OPTIONS statement." RECURSIVE
is required if the subprogram invokes itself, invokes one of its own ENTRY names,
or an ENTRY of it invokes itself, another of its ENTRYs, or the subprogram. 8.5.2
puts the keyword before or after the type (`type [keyword]`, `keyword [type]`).
[lex: a FUNCTION or SUBROUTINE statement may begin with RECURSIVE, before or
after a type and its length; the classifier's typed-FUNCTION test (`lex.go`'s
`FunctionType`, only at a unit's start) grows a `Recursive` flag. Fixed form
squeezes the blanks: `RECURSIVEFUNCTIONF(N)` and `INTEGERRECURSIVEFUNCTIONF(N)`
must still classify, and `RECURSIVE=1` stays an assignment.]

**RESULT (8.5.2.1).** "If you use the RESULT keyword in a FUNCTION statement, you
can specify a local variable name for the function result. In this case, all
references to the function name are recursive calls, and the function name must
not appear in specification statements." The RESULT name differs from the
function's. The manual's example:

```
      RECURSIVE FUNCTION FACTORIAL(P) RESULT(L)
      INTEGER P, L
      IF (P .EQ. 1) THEN
        L = 1
      ELSE
        L = P * FACTORIAL(P - 1)
      END IF
      END
```

[sema: the result name becomes the `ResultCell`'s symbol, typed by its own
declarations (the function's type comes from it); the function's name is then a
procedure in its own body. 8.11.1 allows RESULT on ENTRY too, with the same
association rules for result variables as without (Phase 79's groups).]

**Without RESULT** the HP manual says only that an array-valued directly recursive
function needs it. VAX FORTRAN functions are scalar, so the question is whether
`F(N-1)` in F's own body is a call or a use of the result. Fortran 90's rule
(12.5.2.2) is that, the result being scalar, a reference with an argument list is
a call; a CHARACTER function's `F(1:3)` is a substring of its result, told apart
by the colon. Question 2. [sema: in a recursive unit without RESULT, `Apply` of
the unit's own name with arguments is a `FuncRef`; with no list, or a substring,
it is the result.]

**ENTRY (8.11).** "An ENTRY statement can reference itself if the function or
subroutine subprogram was defined as RECURSIVE." [sema: in a recursive unit a
call of any of the unit's entry names is a call; in a non-recursive one it stays
RECURSIVE's error, extended to calls of the unit's other ENTRY names, which
govax doesn't check today. Dummies "do not retain their association from one
reference of a subprogram to another" already: the `$An` cells are written by
each entry point.]

**Storage (5.3).** "By default, the compiler allocates local variables of
non-recursive subprograms ... in the static storage area. ... To change the
default for variables, specify them as AUTOMATIC or specify RECURSIVE." And:
"Variables that are data-initialized, and variables in COMMON and SAVE statements
are always static. This is regardless of whether a compiler option specifies
recursion." STATIC names a variable static explicitly. [sema: in a recursive unit
a local variable or array is `Automatic` unless it is SAVE'd (a SAVE with no list
saves every local, 5.17), has a DATA or type-statement initial value (or is a
record whose structure has one), is in COMMON, is named STATIC, or is EQUIVALENCEd
to one of those (the group is one storage sequence, so it goes where its static
member goes: ours, unconfirmed; the HP manual is silent). A dummy, a function
result, a PARAMETER, and a statement function's dummy aren't variables of the
frame. The main program and BLOCK DATA are never recursive: `/RECURSIVE` leaves
them alone (ours; a main program can't be called).]

**A contradiction in the HP manual.** 5.17 (SAVE) lists, among "variables saved
by default", "Local variables of recursive subprograms", against 5.3's plain
statement that RECURSIVE changes the default to the stack. Saving them would make
RECURSIVE useless, so the reading here is 5.3's. Question 3.

**OPTIONS (13.3)** lists `/[NO]RECURSIVE` among its qualifiers. [`source/options.go`:
`OptionsQualifier`'s table gains RECURSIVE; it applies to the unit OPTIONS
precedes, overriding the command, as `/[NO]I4` does.]

**Fortran 2018 changed the default:** procedures are recursive unless declared
NON_RECURSIVE. Nothing in our DEC sources has it. Question 1 is which default
govax takes.

**`/STANDARD`:** RECURSIVE, RESULT, and OPTIONS `/RECURSIVE` aren't FORTRAN-77
(`sema/standard.go`'s tables get the forms).

## Where the compiler keeps things today

Every cell is made by `ir`'s `newCell` (`lower.go`), at the next longword of
`$LOCAL`, except AUTOMATIC variables (`Cell.Frame`, from `Symbol.Automatic`), and
the optimizer's cells, which `opt/cse.go` and `opt/registers.go` make themselves
at `r.LocalSize`. Each kind, and what a second activation does to it:

| Cell | What | Shared by two activations today | In a recursive unit |
|---|---|---|---|
| variables, arrays | the unit's locals | yes (static) | frame, by the rule above |
| `ResultCell` | the function's value | yes | frame |
| `$Tn` `TempCell` | a statement's temporaries | yes | frame |
| `$Tn` `TextCell` | a concatenation's value | yes | frame (large ones are already dynamic strings) |
| `$Cn` `$Sn` `LimitCell` | DO counts, steps, limits | yes | frame |
| `$Dn` `DescriptorCell` | descriptors whose address is set at run time | yes; their constant fields are initial values in `$LOCAL` | frame, every field written at run time |
| `$Bn` | an adjustable array's descriptor | yes | frame |
| `$An` `ArgCell`, `$E` `SelectorCell` | ENTRY's dummies' addresses, the entry used | yes | frame |
| `$FP` `FrameBaseCell` | the FP a variable format expression reaches AUTOMATIC variables by | yes: wrong under any reentry | gone (below) |
| `$KLn` `KeyListCell` | OPEN's KEY= list, filled by the statement | yes | frame |
| `$FCn` `FormatCell` | FTNRTL's parsed FORMAT, filled once | yes, harmlessly: the parsed form is the same for every activation | stays static |
| `$Xn` (codegen) | spill cells | yes | frame |
| `$Un`, `$Hn` (opt) | CSE and hoisted temporaries | yes | frame |
| `$PDATA` data | constants, texts, control blocks, FORMAT blocks | read-only | unchanged |
| `$PDATA` descriptors and `$Nname` blocks | an item's descriptor; a NAMELIST's descriptors | a static variable's address, fixed at link time | of a frame variable: built at run time |

[ir: one constructor decides where a cell goes, by `Routine.Reentrant`; the
optimizer's two constructors go through it. Frame cells keep their `Offset`
in the frame area; `FrameSize` grows instead of `LocalSize`. A frame descriptor
can't have initial values: the code that stores its address stores its whole
first quadword too (a `MOVQ` from a `$PDATA` copy, then the address), Question 6's
alternative being a block of stores at each entry point.]

**NAMELIST** of an AUTOMATIC variable is refused today (NMLAUTOMATIC,
`sema/io.go`), since its block in `$PDATA` holds each item's descriptor. In a
recursive unit nearly every NAMELIST would hit it. [ir: a group with a frame
member gets a block in the frame, filled at each statement that uses it (the
group's text and counts copied from `$PDATA`, the descriptors' addresses
stored); NMLAUTOMATIC goes away for AUTOMATIC variables too.]

**Variable format expressions** (`ir/lowervfe.go`) are a routine per FORMAT that
FTNRTL calls with the expression's number and the I/O routine's AP; its
temporaries are static, and an AUTOMATIC variable is reached through `$FP`,
which each entry point writes. Under recursion `$FP` names whichever activation
entered last. FTNRTL's statement context already records the FP of the routine
that began the statement (`ftn_statement.mar`, for popping dead contexts).
[FTNRTL: RESOLVE passes that FP as a third argument; an old image's VFE routine
reads only two, so nothing breaks (GSMATCH's minor id raised). ir: the VFE
routine reaches frame cells through 12(AP) loaded into a register, its own
temporaries in its own frame; `$FP` and `FrameBaseCell` go for every unit, which
also makes AUTOMATIC with VFEs correct under AST reentry.]

**Hidden results and dynamic strings.** A CHARACTER, REAL\*16, or COMPLEX\*16
function's caller passes a temporary (or its descriptor); a concatenation too long
for a `TextCell` is a class D string from LIB$SGET1_DD whose descriptor is a cell,
freed at the statement's end. Both are cells in the table above and move with it.

**Entry sequences.** Each entry point reserves the frame (`SUBL2 #n,SP`). A big
frame (an array of 100,000 REAL\*8s) is one `SUBL2`, and the stack's growth comes
from the access below it: with paging on, `corevms/userstack.go`'s `expandStack`
creates every page from the access down to P1's low end, so a frame of any size
grows the stack by the first touch. With paging off the user stack is VMINIT's
fixed P1 region; how large, and what a frame past it does (ACCVIO, and the
condition's report with no user stack, Phase 67's `noStackToReport`), is subtask
9's to measure. A runaway recursion's end is Phase 70's (`%X1000000C`, about
290,000 levels of a small routine in the default stack).

## The optimizer

`opt/effects.go`'s comment says why a local nobody took survives a call: "A local
that isn't taken could only be changed by this same routine running again, and
section 6.2 ... rules that out." In a recursive unit:

- **frame locals** still survive a call: each activation has its own, and a call
  that recurses changes the new activation's, never this one's. `ChangedByCall`'s
  `FrameRegion` rule holds as it is.
- **static locals** (SAVE'd, DATA, STATIC) of a recursive unit may be changed by
  any call, since any call may lead back into the unit. [opt: `ChangedByCall` of a
  `LocalRegion` place is true in a recursive routine; the comment's argument is
  rewritten for both kinds of unit.]
- **registers** R6 to R11 are saved by each entry mask and restored by RET, so the
  allocator needs nothing; the dead-code pass's compaction of `$LOCAL`
  (`deadcode.go`) gets a frame counterpart, or leaves the frame alone.
- A non-recursive unit is unchanged: section 6.2 still holds for it, and an AST
  that reenters it is still VOLATILE's business.

## FTNRTL

FTNRTL is already reentrant in the sense that matters: all its writable state is
process-wide by design (`$FTN_DATA`: the unit table and the statement stack), each
statement has a context of its own, and ASTs are held off while the stack changes
(Phase 83). What recursion adds:

- **Recursive I/O** (a function in an I/O list that does I/O, and itself calls
  the function): another unit or an internal file nests, the same unit is
  RECIO_OPE (the 1988 manual's 7.1.2), as now. To be tested at depth.
- **Popping dead contexts** compares each context's FP with the caller's. A
  recursive activation's frame is below its caller's, so a nested statement in a
  deeper activation is kept, and one in a shallower activation that a signal
  unwound is popped. The known blind spot, "a routine called again from the same
  place has the very frame it had", doesn't get worse: a recursive call is from a
  deeper frame. To be tested with a handler that unwinds several levels.
- **The format cache** (`$FCn`): two activations may race to parse the same
  FORMAT if an AST comes between the test and the store; the loser's parsed form
  leaks. [FTNRTL: store with the cell checked again under `$SETAST` off, freeing
  the extra copy.]

## The debugger

`debugger/fortran.go`'s `fortranFrameOf` finds an argument or AUTOMATIC variable
in the innermost active frame of its unit, so in a recursion EXAMINE always shows
the deepest activation. The VMS debugger's answer is SET SCOPE (`/CALL n`,
`/CURRENT`) and CANCEL SCOPE; govax has SHOW SCOPE but not SET SCOPE
(`debugger/program.go`'s comment says so). [debugger: SET SCOPE/CALL n picks the
frame names are looked up in; SHOW SCOPE marks it; a STEP or GO resets it, as on
VMS (to be confirmed by a probe of VMS's debugger on a recursive MACRO program,
since VMS has no FORTRAN to debug).] `/DEBUG`'s table (`dbgtable`) already has a
`Frame` place kind; frame cells of the compiler's own aren't in it, as static ones
aren't.

## Testing without a DEC compiler

- **Hand-worked programs** in `testdata/fortran`, both optimizer modes, as every
  FORTRAN phase has had.
- **gfortran** (16.2 on the development machine), as Phase 86 used it: the
  programs written in the subset both accept (RECURSIVE, RESULT, plain
  FORTRAN-77 otherwise) also run under gfortran, and a script compares the
  outputs. It is a check on the hand-worked outputs, never the reference.
- **The corpus under `/RECURSIVE`:** like `optimizerModes`, a mode that compiles
  every corpus program `/RECURSIVE`. A conforming FORTRAN-77 program doesn't
  depend on an unsaved local keeping its value between calls (17.3 of the
  standard; the 1988 manual's 4.12's SAVE), so its output must not change; a
  program that does change is listed with the reason (and may be a fault in the
  program). The same for DUNGEON, locally: a game played the same `/RECURSIVE` is
  the strongest test the frame code can get.
- **ASTs:** a routine active in the main line and entered again by a timer AST,
  its locals and temporaries intact afterwards (as `asts.for` and `optasts.for`
  do for static storage).
- **The VMS round:** as `testdata/ftn86` did, VMS 7.1's MACRO and LINK build the new
  programs from govax's `.MAR` files and FTNRTL from its sources, and run them; it
  checks the frame code on a real VAX, not the language. It needs a run on the
  author's VAX.

## Decisions (proposed)

1. **Recursion is by request**, as the HP manual has it: RECURSIVE, `/RECURSIVE`,
   or OPTIONS `/RECURSIVE`. The default stays static (Phase 74's Decision 10), so
   every existing program compiles to the same code. (Question 1.)
2. **A recursive unit is reentrant whole:** its locals by 5.3's rule, and every
   cell of the compiler's in the frame, so it is also safe to enter from an AST
   or handler while active. A non-recursive unit's cells stay static, AUTOMATIC
   or not (Question 5).
3. **RESULT** on FUNCTION and ENTRY, HP's rules; without it, a recursive
   function's own name with an argument list is a call (Question 2).
4. **The storage rule** is 5.3's, with an EQUIVALENCE group going static if any
   member must; the main program and BLOCK DATA are never recursive.
5. **A frame descriptor is written whole at its use**, not at entry (cheaper for
   a routine that only sometimes does I/O; Question 6).
6. **VFEs get the I/O routine's FP from FTNRTL** (a third argument), and `$FP`
   goes for every unit.
7. **NAMELIST groups with frame members** are built in the frame by each statement
   that uses them; NMLAUTOMATIC is withdrawn.
8. **The optimizer's call rule** changes only for static locals of a recursive unit.
9. **Frame variables aren't cleared** at entry: an unsaved local is undefined at
   each call (the standard's rule, and what a stack gives). `/CHECK` gains nothing
   for it in this phase.
10. **An address FTNRTL keeps past the statement** (ASSOCIATEVARIABLE=) naming a
    frame variable is a warning (new, ASSOCFRAME): the unit may return while the
    file is open. Ours, unconfirmed.

## Questions for the author

1. **The default.** Keep FORTRAN-77's and the HP manual's static default with
   RECURSIVE by request (recommended: every program and golden file unchanged,
   DUNGEON unaffected), or take Fortran 2018's recursive default with
   NON_RECURSIVE? A middle way: the static default, and also accept NON_RECURSIVE
   to exempt one unit from `/RECURSIVE`.
2. **A recursive function without RESULT:** is `F(N-1)` in F's own body a call
   (Fortran 90's rule for a scalar result; recommended), or must a function use
   RESULT to recurse directly (stricter, and error-prone for old code)?
3. **The HP manual's contradiction** (5.17 "saved by default: local variables of
   recursive subprograms" against 5.3): read as 5.3 (recommended), noting it in
   MANUAL.md's 14.7?
4. **HP Fortran's `/REENTRANCY=(ASYNC|NONE|THREADED)`** qualifier, which I recall
   from its User Manual (not among our sources, so unconfirmed): leave it out
   (recommended; FTNRTL is AST-safe always, and govax has no threads), or accept
   and ignore it?
5. **AUTOMATIC in a non-recursive unit:** keep its compiler cells static as now
   (recommended; the HP manual doesn't make AUTOMATIC permit recursion), or put
   them in the frame whenever a unit has a frame, so AUTOMATIC alone makes a unit
   AST-reentrant (Phase 82's original design note)?
6. **Frame descriptors:** written whole at each use (recommended), or initialized
   once at each entry point?
7. **SET SCOPE in the debugger** in this phase (recommended, with a small VMS probe
   of the debugger on a recursive MACRO program, `testdata/dbg89`), or later?
8. **The VMS round** (subtask 12) needs a run on your VAX: include it, or leave the
   frame code checked by govax and gfortran only?

## The author's answers (2026-11-04)

Every recommendation is taken:

1. **The default stays static**, recursion by request (RECURSIVE, `/RECURSIVE`,
   OPTIONS `/RECURSIVE`). NON_RECURSIVE, the "middle way", isn't taken: OPTIONS
   `/NORECURSIVE` exempts a unit from `/RECURSIVE`.
2. **A recursive function's own name with an argument list is a call**, without
   RESULT.
3. **5.3 wins** over 5.17's "saved by default"; MANUAL.md's 14.7 is to say so.
4. **No `/REENTRANCY`.**
5. **AUTOMATIC alone keeps the compiler's cells static.**
6. **Frame descriptors are written whole at each use.**
7. **SET SCOPE is this phase's**, with `testdata/dbg89`.
8. **The VMS round is in**, and wanted: VMS's MACRO and LINK build and run the
   `.MAR` files the compiler writes for recursive programs (subtask 12).

## Subtasks (provisional, revised by the survey)

One phase is enough: the work is a storage class threaded through passes that
already exist, and FTNRTL needs little. If subtask 8 or 10 grows past a subtask,
it moves to a Phase 90.

1. **Survey** (done 2026-11-04): Part A, for the author's review.
2. **The syntax** (done 2026-11-04): RECURSIVE in both orders, RESULT on FUNCTION and ENTRY, the
   command's `/[NO]RECURSIVE` (`console.dcl`, `dclcmd/fortran.go`, `govax fortran
   --recursive`), OPTIONS `/[NO]RECURSIVE`; sema's RECURSIVE message lifted in a
   recursive unit and extended to ENTRY names in others; a function's name as a
   call; `/STANDARD`; the listing's and cross reference's notes. Lex, parse, and
   sema goldens. A recursive unit is NOTIMPL past sema until subtask 4.
3. **The storage rule** (done 2026-11-04) (sema): which locals are automatic in a recursive unit
   (SAVE, initial values, COMMON, STATIC, EQUIVALENCE groups), the storage map's
   frame column, messages; goldens.
4. **The compiler's cells in the frame** (done 2026-11-04) (ir, codegen, opt): one constructor by
   `Routine.Reentrant`, every kind in the table, spill cells, the optimizer's
   cells, frame alignment (OCTA records under `/ALIGN`), `ir.Check`; frame
   descriptors written whole (Decision 5). Recursive programs without I/O run
   end to end: factorial, Ackermann, mutual recursion, recursive ENTRY.
5. **Frame addresses in data** (done 2026-11-04)**:** I/O items, arguments, hidden results, dynamic
   strings, internal files, ENCODE/DECODE, KEY= lists, `%DESCR`, adjustable
   arrays; NAMELIST blocks built in the frame (Decision 7, NMLAUTOMATIC withdrawn);
   ASSOCFRAME (Decision 10).
6. **Variable format expressions** (done 2026-11-04) in a reentrant unit: FTNRTL passes the FP
   (GSMATCH's minor id raised), the VFE routine's own frame, `$FP` gone; a VFE
   test in recursion and under an AST.
7. **The optimizer under recursion:** `ChangedByCall` for a recursive unit's
   static locals, `effects.go`'s argument rewritten, dead code and the frame;
   `optrecur.for`, both modes; every corpus program run `/RECURSIVE` too
   (`recursiveModes`), with any exception explained.
8. **FTNRTL:** recursive I/O at depth, dead-context popping across unwound
   recursive levels, the format cache under ASTs; fixes as found.
9. **The stack:** big frames in both paging modes, paging off's limit measured
   and documented, runaway recursion's report, LINK's STACK= in MANUAL.md.
10. **The debugger:** SET SCOPE /CALL n and /CURRENT, CANCEL SCOPE, SHOW SCOPE's
    mark, EXAMINE of each activation's locals and arguments; the probe
    `testdata/dbg89` on VMS 7.1 (Question 7).
11. **End to end:** `testdata/fortran`'s recursive programs (a tree of records
    walked recursively, quicksort, a recursive descent expression evaluator with
    CHARACTER functions and concatenation, recursion across files in `modules/`,
    a routine entered by an AST while active, NAMELIST and VFEs in recursion), in
    both optimizer and paging modes; the gfortran comparison for the portable
    ones; DUNGEON `/RECURSIVE`, locally.
12. **The VMS round:** `testdata/ftn86`'s next round, the new programs built and
    run by VMS's MACRO and LINK (Question 8).
13. **Close-out:** MANUAL.md (14.2's statements, 14.3's qualifier, 14.7's
    differences and unconfirmed choices, a section on recursion and ASTs),
    FORTRAN-INTERNALS.md's new section (storage classes, frames, and what
    reentrancy costs a compiler), HELP FORTRAN, PLAN.md, CLAUDE.md, "Where things
    stand".

## Not planned

- Threads (DECthreads): govax has no PTHREAD$RTL; FTNRTL's process-wide state
  would need locks.
- Fortran 90's other procedure features (PURE, ELEMENTAL, modules, explicit
  interfaces, INTENT, allocatable arrays): a dialect of their own.

## Progress log

- **2026-11-04:** The plan and subtask 1, the survey (Part A), written for the
  author's review. The survey found the HP manual's rules (RECURSIVE, RESULT,
  OPTIONS `/RECURSIVE`, the storage rule and a contradiction in it), every static
  cell the compiler makes and what each needs, that `$FP` is wrong under any
  reentry today, that NAMELIST would refuse nearly every recursive unit's
  variables, that the optimizer's call rule rests on section 6.2 and needs one
  change, that FTNRTL is already reentrant but for a leak in the format cache,
  and that the debugger always shows the innermost activation. Eight questions
  wait for answers.
- **2026-11-04:** The author's answers: every recommendation taken, and the VMS round
  wanted. Subtask 2, the syntax.
  - **Lexer** (`lex.go`): `Statement.Recursive`. At a unit's start, text beginning
    RECURSIVE is classified again from after the keyword, and taken if that is a
    FUNCTION or SUBROUTINE statement not already recursive; `typedFunction` takes
    RECURSIVE between the type's length and FUNCTION. So `RECURSIVE INTEGER*2
    FUNCTION`, `INTEGER RECURSIVE FUNCTION`, and `RECURSIVE SUBROUTINE` are headers,
    `RECURSIVE = 1` is still an assignment (the = test comes first), and RECURSIVE
    twice, away from a unit's start, or before PROGRAM is UNRECSTMT
    (`TestRecursiveRefused`). The keyword is no token; `Dump` shows "(recursive)".
  - **Parser** (`parse/decl.go`): `FunctionStmt.Recursive` and `Result`,
    `SubroutineStmt.Recursive`, `EntryStmt.Result`; `result` reads `RESULT (name)`
    only after a parenthesized list, since squeezed `FUNCTION F RESULT(R)` is a name
    FRESULT (both of the HP manual's syntax diagrams write the parentheses with
    RESULT); a SUBROUTINE's RESULT is RESULTSUB.
  - **Sema:** `Unit.Recursive` (the header's RECURSIVE, or the unit's `/RECURSIVE`;
    never a main program or BLOCK DATA); `Options.Recursive`, OPTIONS
    `/[NO]RECURSIVE` (`source/options.go`'s table, `prologue`). In a recursive unit a
    CALL of the subroutine's own name or an entry point's is a `Call`, and a
    function's own name or an entry point's with an argument list a `FuncRef`
    (`selfReference`), its type the value's; both are among the unit's `calls`, so
    `checkReferences` now checks a recursive unit's references to itself (ARGCOUNT)
    as another unit's. An entry point's name may be used before its ENTRY there
    (`usedBeforeEntry`; its dummies still may not). RESULT (`resultVariable`): the
    function's or entry point's name becomes a `UnitName` whose `ResultVar` is the
    variable, which is `Unit.Result` or `Entry.Sym` and gets the header's type;
    RESULTSAME, RESULTNAME (a routine's name, another result, a dummy), RESULTDECL (a
    type statement naming the function), RESULTSUB (an ENTRY in a subroutine). `Entry.Name`
    is the entry point's name, which `Entry.Sym.Name` no longer is with RESULT
    (lowering's `.ENTRY` uses it). A statement function still can't use itself, and
    RECURSIVE's text now says the subprogram isn't RECURSIVE. Without RECURSIVE every
    old message stands; `RESULT` in a function that isn't recursive makes the name's
    references RECURSIVE's error. The golden is `sema/testdata/recursive.for`; the
    dump's unit line says "recursive".
  - **`/STANDARD`:** "a RECURSIVE subprogram" and "RESULT", and RESULT's name is
    checked as a name (`TestRecursiveStandard`).
  - **The command:** `/[NO]RECURSIVE` (`console.dcl`, id 1391; `dclcmd`'s
    `FortranOptions.Recursive`; `fortran.Options.Recursive`), `govax fortran
    --recursive`, HELP FORTRAN, MANUAL.md's 6.5 and 14.3 (a paragraph on recursion).
    `TestFortranRecursiveQualifier` runs it through DCL.
  - **Until subtask 4** `fortran.Compile` reports each recursive unit NOTIMPL ("a
    RECURSIVE subprogram") at its header after analysis, so nothing is lowered with
    static storage that a second activation would share (`TestRecursiveUnits`).
  - **Left for later:** the listing and cross reference show nothing new yet; the
    storage map's frame column is subtask 3's. HELP and MANUAL.md say recursive units
    are NOTIMPL, which subtask 4 takes out.
- **2026-11-04:** Subtask 3, the storage rule (`sema/storage.go`'s `recursiveStorage`,
  run before `layout`).
  - **The rule:** in a recursive unit every variable and array of its own, a name
    nothing classified among them, is `Symbol.Automatic` unless it is SAVE'd, by name
    or by a SAVE with no list (`analyzer.saveAll`, new: such a SAVE was a no-op
    before, every local being static anyway), has an initial value (DATA, a type
    statement, or a record whose structure gives one, `Structure.image`), is in a
    common block, or is named STATIC. A name made automatic so is placed by `layout`
    as an AUTOMATIC one is, in the frame area (`Unit.FrameSize`); the storage map's
    `FP-n` column and the debug table's `Frame` places follow from that.
  - **An EQUIVALENCE group** is static if any member must be, else in the frame
    whole (ours, unconfirmed). A group with an AUTOMATIC statement's name keeps
    Phase 82's handling: the conflicts are AUTOSAVE, AUTOCOMMON, AUTOEQUIVALENT, and
    AUTOINIT, as in any unit; a name made automatic by recursion is never an error,
    since whatever would conflict makes it static instead.
  - **The function's value** is in the frame too (the function's name, RESULT's
    variable, and the one place `placeResults` gives the names of a function with
    ENTRY statements), unless it goes through a hidden argument. The survey's note
    that "a function result" isn't a variable of the frame was about sema's
    classes; its table already put `ResultCell` in the frame, and since sema lays the
    value out, this is where it moves.
  - **No new messages.** NMLAUTOMATIC doesn't fire for names automatic by
    recursion (the rule runs after `checkNamelists`): a NAMELIST in a recursive
    unit waits for subtask 5's blocks in the frame, behind subtask 2's NOTIMPL.
  - **The dump** says "the frame area is n bytes" when a unit has one
    (`automatic.sema` gains the line). The golden is `sema/testdata/recurstore.for`:
    every case above, a CHARACTER function's hidden result, a SAVE with no list, and
    the same names in a unit that isn't recursive, all static; `recursive.sema`'s
    results moved to the frame.
  - **Left for later:** frame alignment past a longword (a NATURAL record of a
    recursive unit is on a longword, as an AUTOMATIC one is) is subtask 4's; the
    listing's storage map can't show a recursive unit until subtask 4 lifts the
    NOTIMPL.
- **2026-11-04:** Subtask 4, the compiler's cells in the frame.
  - **One constructor** (`ir.Routine.NewCell`): `Routine.Reentrant` is `Unit.Recursive`,
    and in such a routine every cell of the compiler's goes in the frame area, after
    sema's frame variables, which grows by it; lowering's `newCell` and the optimizer's
    `newTemporary` ($U, $H) go through it. A FORMAT's cache ($FCn) stays in `$LOCAL`, as
    the survey's table has it, and so would the VFEs' `$FP`. `ir.Check` refuses a
    reentrant routine's compiler's cell in `$LOCAL`, and now checks the frame's cells for
    overlaps as it did `$LOCAL`'s (`checkArea`).
  - **Frame descriptors are written whole at each use** (Decision 5): `descriptorCell`
    stores `Descriptor.Fixed()`, every longword the compiler knows (length, type, class;
    a class D one's null pointer; a class A one's flags, size, multipliers, and bounds),
    before the statement stores the addresses, as `MOVL #n` (the code generator's
    comments name each field); an adjustable array's `$B` cell is filled the same way at
    each entry point (`computeArrayBounds`). Constants rather than a `MOVQ` from a copy in
    `$PDATA`, as the survey had it: no datum, and only the longwords that aren't
    addresses. `ir.ArrayFlags` replaces the code generator's own flag names.
  - **The code generator:** a reentrant routine's spill cells ($Xn) are in the frame
    (`frameSize`, which every displacement from FP is now counted from, since a spill is
    made while the code is), ENTRY's argument cells and selector are reached as
    `$A1(FP)` (`cellPlace`), the entry point's `SUBL2` says "this activation's own
    storage", and the storage map's heading says RECURSIVE (`TestRecursiveUnits` reads
    it, now that recursive units compile).
  - **The optimizer:** `closeUp` closes up the frame as it does `$LOCAL`, for cells no
    statement uses and cells made registers (`closeUpBoth`). And the call rule moved here
    from subtask 7, since `/OPTIMIZE` is the default and a recursive program came out
    wrong without it: in a reentrant routine a call may change any static local
    (`ChangedByCall`), so `V = 7; CALL S(...); N = V` no longer forwards the 7 when the
    call recurses and sets V. The rest of subtask 7 (`effects.go`'s argument rewritten,
    `optrecur.for`, the corpus `/RECURSIVE`) stays there.
  - **Two bugs found on the way, fixed:** a recursive CHARACTER (or REAL\*16, COMPLEX\*16)
    function's call of itself went through its hidden argument as if that were a dummy
    procedure (`callee`: the function's own value's symbol is a call by name); and, in any
    unit, the register allocator could give one of a function's ENTRY names a register by
    liveness though its names share one place, so `G = .TRUE.` after `ENTRY G` was lost
    when the return read F's (`findTemporaries` now skips a cell that shares storage,
    `TestSharedResultsStayInMemory`). The second is Phase 84's, and no corpus program had
    a function whose entry points store their own names.
  - **Still NOTIMPL in a recursive unit** (`fortran.notYetReentrant`, `TestNotYetReentrant`):
    a NAMELIST group with a frame variable (subtask 5's blocks in the frame) and a FORMAT with
    variable format expressions (subtask 6). Everything else compiles, I/O among it: an I/O
    item, argument, or hidden result in the frame already had a descriptor cell, now in the
    frame and written whole.
  - **Frame alignment:** unchanged, a longword at most, as for AUTOMATIC (Phase 82): CALLS
    leaves SP on a longword, so a frame variable's boundary in the area is its boundary in
    memory only up to that, and more would need the frame reached through a register each
    entry point aligns. A NATURAL record of a recursive unit under `/ALIGN` is on a longword
    (subtask 3); the VAX doesn't fault on unaligned data. Ours, unconfirmed.
  - **Tests:** `testdata/fortran/recursion.for` (factorial by name, Ackermann by RESULT,
    mutual recursion through ENTRY, quicksort of an adjustable array, CHARACTER, REAL\*16,
    and COMPLEX\*16 values through hidden arguments, Fibonacci's local outliving a call,
    permutations printed from the deepest activation, alternate returns, a statement
    function, SAVE'd and COMMON storage shared, 10,000 activations, a 1,600-byte array in each
    frame), both optimizer modes and both paging modes; its output was worked by hand, and
    gfortran 16 prints the same given RESULT for each direct recursion, but `-0.0` for an
    imaginary part (IEEE's negative zero; the VAX has none). gfortran refuses a function's
    name as a call without RESULT ("refers to the result variable"; its 12.5.2.1), against the
    survey's reading of Fortran 90; the author's answer 2 stands for govax. The goldens
    `ir/testdata/recursive.for` and `codegen/testdata/recursive.for` (every kind of cell,
    the descriptors' fields, a dynamic string, ENTRY), `TestSpillInFrame`,
    `opt/recursive_test.go` (the frame closed up, the call rule), `TestFortranRecursiveQualifier`
    (no message now). `testdata/ftn86`'s procedure is regenerated with RECURSION, and
    `TestFTN86Oracle` passes over it as `ftn86AwaitingVMS` until subtask 12's round.
  - **Left for later:** the debug table's `Through` places and adjustable arrays' descriptors
    name a reentrant routine's frame cells by their symbols, which are displacements, so the
    debugger reads them as addresses (subtask 10). MANUAL.md's paragraph on recursion and HELP
    FORTRAN now say what is NOTIMPL; the full write-up is subtask 13's.
- **2026-11-04:** Subtask 5, frame addresses in data.
  - **NAMELIST** (`ir/lowerio.go`'s `namelist`, `namelistCell`; Decision 7): a group with an
    `Automatic` member has its block in a `NamelistCell` (`$NLname`, one per group), in FTNRTL's
    layout, which each statement using the group fills in whole: the name's address, the count, and
    each entity's name and descriptor, the descriptors written by `item` as for any I/O item. The
    names stay in `$PDATA`. A group of static variables is still a datum in `$PDATA`, so nothing
    changes for a unit without frame variables. In a reentrant routine the cell is in the frame, by
    `newCell`; beside AUTOMATIC alone it is in `$LOCAL`, as the author's answer 5 has the compiler's
    cells.
  - **NMLAUTOMATIC is withdrawn** (`sema/io.go`, `diag/messages.go`), for AUTOMATIC in any unit, and
    `notYetReentrant` no longer reports such a group. Only variable format expressions are left in
    it (subtask 6).
  - **ASSOCFRAME** (Decision 10; ours, unconfirmed): a warning for an associated variable, OPEN's
    `ASSOCIATEVARIABLE=` or DEFINE FILE's, that is `Automatic`. `associatedVariable` keeps each in
    `analyzer.associated`, since statements are analyzed before `recursiveStorage` decides what is in
    the frame, and `checkAssociated` reports after it. A warning, since a routine that closes the file
    before returning is correct. A SAVE'd one has no warning.
  - **The rest of the subtask's list needed no change in the compiler:** I/O items, arguments, hidden
    results, dynamic strings, internal files, ENCODE and DECODE, KEY= lists, `%DESCR`, and adjustable
    arrays already go through descriptor cells and blocks that subtask 4 put in the frame. This subtask
    is what tests them, each with its I/O after the recursive call returns, so shared storage would
    print the innermost activation's values.
  - **Tests:** `testdata/fortran/recurio.for`, worked by hand before it ran, both optimizer modes and
    both paging modes; the goldens `ir/testdata/framenml.for` and `codegen/testdata/framenml.for`;
    `TestErrors`'s cases for AUTOMATIC in a group and ASSOCFRAME; `TestNotYetReentrant`. The program's
    first version wrote unit 6 from a function called in a WRITE to unit 6, and FTNRTL rightly stopped
    it with RECIO_OPE (the 1988 manual's 7.1.2), so the value is computed first.
    `testdata/ftn86`'s procedure is regenerated with RECURIO, in `ftn86AwaitingVMS` until subtask 12's round.
  - **Left for later:** recursive I/O at depth and unwinding are subtask 8's.
- **2026-11-04:** Subtask 6, variable format expressions.
  - **FTNRTL** (`ftn_format.mar`'s `EXPRESSION`) calls F(n, ap, fp), `CTX_FP` being the FP of the routine that
    began the statement; GSMATCH 1,19, since a new image's expressions read the third argument.
  - **The base is a register, R11, not what the survey expected.** Decision 6 stands, but the survey's "its own
    temporaries in its own frame" isn't done. An expression's code is written like any other block's, but for the
    register a frame cell's displacement is from (`routineGen.frameBase`: FP, or R11 in `ir.ExpressionBlocks`' blocks),
    and its routine's start loads it (`MOVL 12(AP),R11`). So an expression's temporaries, spill cells, and descriptors
    stay in the frame of the activation it computes for, where lowering and the code generator already put them, with no
    second frame to lay out and nothing for the optimizer's cells to choose between. That's safe because an activation is
    in one statement at a time, and an expression's call that comes back into the unit is another activation, with
    another frame. `needsFrameBase` says when a routine has one: it has expressions, and a frame or may get one by a spill.
  - **`$FP` is gone for every unit** (`FrameBaseCell`, `Routine.FrameBase`, `frameBaseMem`, and `opt/places.go`'s case for
    it), as Decision 6 has it, so AUTOMATIC with expressions is right under an AST too. The IR now names a frame cell
    in an expression as it does anywhere, so the optimizer sees the one place rather than the whole frame.
  - **Keeping R11 free:** `noteCellRegisters` marks it as a cell register, which keeps expressions off it, and as used,
    which saves it in the routine's one entry mask, and tells the allocator through `ExpressionRegisters`. A routine with
    expressions and a frame has one register cell fewer and saves one more register at each call. One without a frame is
    unchanged (`TestFrameBaseKept`), so `vfe.for`'s code is as it was. `ir.Successors` and `ir.ExpressionBlocks` moved from
    `opt`, since the code generator needs them too.
  - **`notYetReentrant` is gone:** a recursive unit has no NOTIMPL left of its own (`TestVFEsReentrant`).
  - **Tests:** `testdata/fortran/recurvfe.for`, its output worked by hand before it ran, passed in both optimizer modes and
    both paging modes. Its expressions reach a frame variable, a dummy through ENTRY's cell, an adjustable array's bound,
    a temporary, a descriptor, and a spill cell through R11, as its MACRO file shows, and LOOPER's cells stop at R10. A
    statement entered again by an AST in the middle of its expressions is run in a RECURSIVE unit and in an AUTOMATIC one.
    Two checks that the program can fail: FTNRTL changed to pass its own FP fails `recurvfe` and `automatic`; and the AUTOMATIC case on
    the build before this subtask printed `[     7 7 7    ]`, reading the AST's W through `$FP`, where this one prints
    `[     7 7     7]`. The goldens `ir/testdata/recurvfe.for` and `codegen/testdata/recurvfe.for`; `automatic`'s changed as
    above. gfortran has no variable format expressions, so this program has no comparison there. `testdata/ftn86`'s
    procedure is regenerated with RECURVFE, in `ftn86AwaitingVMS` until subtask 12's round.
  - **Left for later:** the format cache's race under ASTs is subtask 8's.
