# Phase 85 — FORTRAN compiler usability: listings, checks, warnings

**Status:** in progress. Subtask 1, the survey, was reviewed by the author on
2026-11-02; their answers are in Part A, "The review". Planned 2026-10-29. Needs
Phase 80 (I/O, for the run-time checks' messages); best done after Phase 83.
Part of the FORTRAN program laid out in `PHASE-74 - FORTRAN plan.md`
(Decisions 18 and 19).

## Goal

The qualifiers that make the compiler pleasant to use, in VAX FORTRAN's
spirit but our own layouts:

- **`/LIST[=file]`:** a listing of the source with line numbers and
  include levels, the messages under their lines, and the storage map
  (each variable, its type, psect, and offset, and the `.MAR` symbol);
  `/SHOW=(INCLUDE, MAP, ...)` choosing its parts; CDEC$ TITLE and
  SUBTITLE in its headings;
- **`/CROSS_REFERENCE`:** where each name and label is defined and used;
- **`/CHECK=(BOUNDS, OVERFLOW, UNDERFLOW, ALL, NONE)`:** subscript and
  substring checks (the INDEX instruction, which traps on a range
  error, or compare and signal), integer overflow (the entry mask's IV
  bit), floating underflow reporting;
- **`/WARNINGS=(DECLARATIONS, GENERAL, NONE)`:** undeclared names
  (IMPLICIT NONE without the statement), unused variables, labels never
  used;
- **`/STANDARD=(SYNTAX, SOURCE_FORM)`:** flagging DEC extensions for
  FORTRAN-77 conformance;
- **`/[NO]DEBUG`:** the `.MAR` file gets `.ENABLE DEBUG`, so MACRO
  writes debugger records without being asked, and every variable a
  labeled symbol the debugger can EXAMINE;
- **`/[NO]OBJECT[=file]`** (approved in Phase 74's review, answer 3):
  FORTRAN runs MACRO on the `.MAR` file it wrote, keeping the `.MAR`
  file, so `FORTRAN/OBJECT; LINK; RUN` works as on VMS. Off by default:
  FORTRAN writes only the `.MAR` file unless asked;
- **`/F77` and `/NOF77`**, if kept (appendix A.6);
- **the debugger on FORTRAN programs** (added 2026-11-02 at the author's
  request): the debugger can't read the names the compiler writes
  (`EXAMINE SQUARES.I`), which Phase 74's Decision 10 chose those names to allow.
  A review of what debugging a compiled program is like, and fixes.

## What earlier phases leave in place

- `/[NO]I4` and `/[NO]G_FLOATING`, on the command and in OPTIONS (Phase 78);
  OPTIONS's `/CHECK` and `/EXTEND_SOURCE` are this phase's.
- `diag`'s messages with positions (Phase 74), the storage map comment
  (Phases 75–76), MACRO's `/DEBUG` and the debugger's EXAMINE by symbol.

## Open questions

- **The listing's layout:** ours, modeled loosely on VAX FORTRAN's
  (which the User Manual shows); page headings like MACRO's.
- **Bounds checks on assumed-size arrays:** only the lower bound and
  the known dimensions can be checked; the manual's rule.

## Subtasks

The list as the survey proposed it and the author's review settled it:

1. **Survey** (done 2026-11-02): Part A below.
2. **The debugger reads dotted names** (done 2026-11-02), with a VMS probe of the
   rule, run by the author on the simh VAX (Part A, "What the probe settled").
3. **`/LIST`, `/SHOW`, TITLE and SUBTITLE** (done 2026-11-02).
4. **`/CROSS_REFERENCE`** (done 2026-11-02).
5. **`/CHECK`,** with OPTIONS's `/CHECK` and the optimizer's rules for it (done 2026-11-02).
6. **OPTIONS's `/EXTEND_SOURCE`,** which reaches back into the first pass (done 2026-11-02).
7. **`/WARNINGS`,** with `UNUSED` a keyword of its own, off by default (answer 3).
8. **`/STANDARD`,** with the list of extensions read from the manual's pages.
9. **`/DEBUG` and `/OBJECT`.**
10. **The rest of the debugger review,** level B (answer 1): D2 to D9, statement
    labels, bare names in a routine, the FORTRAN line at a stop, and subscripts and
    types. Likely to be split once it starts.
11. **`/[NO]F77`** (answer 5).
12. **Close-out:** HELP FORTRAN complete; MANUAL.md; FORTRAN-INTERNALS.md.

# Part A — the survey (2026-11-02)

## Summary

- **The manuals to hand don't describe the command.** The DCL Dictionary has no
  FORTRAN entry, only examples that use it, and the LRM sends every qualifier
  question to the *VAX FORTRAN User Manual*, which we don't have. What's documented is
  OPTIONS's forms, a few asides, and later HP Fortran's language manual. So
  layouts, and most keywords and defaults, are ours (the phase's goal says so), each
  marked below by where it comes from. There's no VAX FORTRAN to probe.
- **The debugger is the largest finding.** The reported problem is real: every
  name the compiler writes is refused. It's also the first of nine problems. Dummy
  arguments are recorded as small constants, no command takes a FORTRAN line number,
  and one location is named after an unrelated common block.
- **`/CHECK` is the riskiest part.** In a test run, a subscript of 7 in a
  3-element array had already been folded by the optimizer into a plain
  `MOVL T.A+24,T.J`. Checks have to be something the optimizer can see and must
  keep, and Phase 84's rule that integer arithmetic moves freely holds only with
  overflow off.
- **`/CHECK`'s default is the one decision with real consequences** (question 2).
  Nothing to hand states it, and one piece of evidence suggests that VAX
  FORTRAN's default isn't what govax assumed.

## What the manuals say

*LRM* is the 1988 VAX FORTRAN manual; *HP* the later HP Fortran language manual,
usable on the terms of Phase 82's survey; *plan* this phase's goal; *memory* what I
recall of VAX FORTRAN's command, which no document to hand confirms and which is
only a starting point.

| Qualifier | Source | Forms | Default | What the source says |
|---|---|---|---|---|
| `/LIST[=file]` | LRM 10.3.3, plan | `/[NO]LIST` | `/NOLIST` (plan: interactive; memory: on in batch) | TITLE and SUBTITLE need it |
| `/SHOW` | plan, memory | `INCLUDE`, `MAP`, `ALL`, `NONE` | `MAP`, no `INCLUDE` (memory) | Nothing in the manuals. `DICTIONARY`, `PREPROCESSOR`, `SINGLE` are memory only and have nothing here to control |
| INCLUDE's `/LIST` | LRM 1.4.2 | in the string | `/NOLIST` | "A number precedes each incorporated statement": the depth. Already in `Line.Listed` |
| `/CROSS_REFERENCE` | plan | `/[NO]` | off | Nothing in the manuals |
| `/CHECK` | LRM 1.4.3, D; HP 13.3 | `ALL`, `NONE`, `[NO]BOUNDS`, `[NO]OVERFLOW`, `[NO]UNDERFLOW`; `/NOCHECK` | **not stated** | LRM's example: `OPTIONS /CHECK` is "full checking". Same forms on the command |
| `/WARNINGS` | LRM 4.8's note, plan | `[NO]DECLARATIONS`, `[NO]GENERAL`, `NONE` | general on, declarations off (memory) | LRM: `=DECLARATIONS` warns of names used but not typed, as IMPLICIT NONE would |
| `/STANDARD` | LRM 4.14 note, plan | `SYNTAX`, `SOURCE_FORM` | off | LRM: leaving out a needed SAVE isn't flagged, since the compiler can't tell. Memory adds `SEMANTIC`, `MIA`, out of scope |
| `/[NO]DEBUG` | Dictionary's examples, plan | memory: `ALL`, `NONE`, `[NO]SYMBOLS`, `[NO]TRACEBACK` | traceback only (memory) | The Dictionary's examples use `FORTRAN/DEBUG/NOOPTIMIZE` |
| `/[NO]OBJECT[=file]` | LRM 10.3.1, Phase 74 | | off (Phase 74's answer 3; VAX FORTRAN's was on) | LRM: CDEC$ IDENT has no effect with `/NOOBJECT` |
| `/[NO]F77` | LRM 1.4.3, 5.3.1, 9.1, A.6; HP | | `/F77` | Below |
| `/[NO]EXTEND_SOURCE` in OPTIONS | LRM 1.4.3 | | the command's | For one unit |

`/MACHINE_CODE` (memory) isn't in the plan: the `.MAR` file is the machine code,
each statement above its instructions. **Decision: left out.**

**`/NOF77`** does four things by the manuals: a DO whose count is zero or negative runs
once (5.3.1); OPEN's BLANK= defaults to `'ZERO'` (9.1) and STATUS= to `'NEW'` (9.1);
and EXTERNAL takes the old form, `EXTERNAL *v` (A.6). OPTIONS takes it for one unit.

## What's in place, tried on this build

- The FORTRAN command refuses all of the above as unrecognized, and OPTIONS's
  `/CHECK`, `/EXTEND_SOURCE`, `/NOF77` are NOTIMPL (`sema/analyze.go`'s `prologue`).
- CDEC$ TITLE and SUBTITLE are checked and kept (`sema/directive.go`).
- `source.Program.Lines` has every line with its file, number, depth, and `Listed`,
  even after an error, and each `diag.Message` its position, which a listing needs.
  But `Result` carries only the MACRO file, and only without errors.
- The storage map is `codegen`'s `writeMap`, comments in the `.MAR` file written after
  lowering; under `/OPTIMIZE` it says which register holds what. It takes the IR, so a
  compilation with errors has none.
- MACRO's and ANALYZE's pagers are their own (`asm`'s unexported `listPager`, `anl.Pager`,
  each with rules reconstructed from VMS's output); neither fits.
- The CPU's INDEX stores its result, then traps with SS$_SUBRNG out of range
  (`cpu/misc.go`); its message is in `vmsdef`. CALLS clears FU and sets IV from the
  entry mask (`cpu/call.go`), so overflow is a mask bit but underflow needs a `BISPSW`
  at each entry. No generated code has yet run with any of these.
- FOR's message table has ADJARRDIM ("adjustable array dimension error") and no
  substring message.
- A constant subscript out of bounds compiles without a word (`A(7)` in a `A(3)`), though a
  constant substring bound out of range is an error.

## `/CHECK`

**The trap.** In a program with `INTEGER A(3)`, `I = 7`, `J = A(I)`, today's
`/OPTIMIZE` writes `MOVL T.A+24,T.J`: forwarding replaced the subscript before anything
could check it. It runs and gets garbage, which `/CHECK=BOUNDS` is meant to stop.

Proposed:

- **Decision 1: an IR operation, not a pass.** A new `Op` that yields its operand
  or faults, written `INDEX s,low,high,#1,#0,Rn`, which leaves `s` in `Rn` (`(0+s)*1`).
  Each subscript is wrapped where it's lowered, so the rest of an element's address is
  unchanged, and expression registers and spill just work. No compare and signal: the
  plan offered both, and one instruction gives VMS's own condition.
- **Decision 2: the optimizer may remove a check it can prove,** folding when the
  value is known in range, and keeps it otherwise, a known-bad one included. A check counts as
  something that faults under Phase 84's Decision 8: hoisted only from a block
  that runs on every trip, and never removed with a dead store.
- **Decision 3: substrings use INDEX too.** `1 <= e1 <= e2 <= len` is `INDEX e2,#1,len`
  then `INDEX e1,#1,e2`, also SS$_SUBRNG, since FOR has no message for it (ours).
- **Decision 4: assumed-size.** The manual gives no rule (the phase's open question
  assumed one): 6.1.1.2 says only that the size isn't known. Every bound known is checked,
  which for the last dimension is the lower alone (ours). A dummy's last dimension of `(1)`,
  FORTRAN-66's way to write `(*)`, is treated as `(*)` (ours, or such programs fail
  `/CHECK`).
- **Decision 5: an adjustable array's bounds are read when checked,** from the descriptor
  cell Phase 79 fills. An upper below the lower signals FOR$_ADJARRDIM at the entry point,
  under BOUNDS (ours; the manuals don't give its rule).
- **Decision 6: overflow is the mask's IV bit,** on each `.ENTRY` of a unit compiled
  with it. Then integer arithmetic can fault, so `ir.Routine` carries the unit's
  checks, and `opt` treats integer operations as it does division. `opt/forward.go` leaves an
  overflowing constant to fault at run time. Address arithmetic runs under IV too; a
  wild subscript may then trap as INTOVF rather than run wild, which is fine.
- **Decision 7: underflow is `BISPSW #^X40` at each entry point** of a unit compiled
  with it, so an underflow is SS$_FLTUND_F, severe, to the program's handlers and
  traceback. "Reporting" in the plan may mean something gentler on VAX FORTRAN; the
  manuals don't say, and LIB$FLT_UNDER is there for a program that wants otherwise.
- **Decision 8: the checks are per unit:** `sema.Options`, read in `prologue`, as `/I4` is.
- **Decision 9: a constant subscript out of bounds is a warning,** GENERAL's, in any
  compilation, but not for a dummy's `(1)` or `(*)` dimension (ours).

## OPTIONS's `/EXTEND_SOURCE`

Lines become statements in the first pass, but OPTIONS is read in the fourth, so the
statement field's end is settled before anything knows of it. **Decision 10:** `source`
recognizes a line beginning `OPTIONS` before a unit, and `END`, enough to switch the field
for that unit's lines. Both are simple there. It reads only this one qualifier.

## The listing

**Decision 11: a package of its own,** `internal/fortran/listing`, plain Go like the other
passes, given the source's lines, the messages, and each unit's map and cross reference,
with its own small pager. `Result` gets a `Listing`. A listing is written even when a
compilation has errors, since that's when it's wanted: then with messages but no map.
`.LIS`, beside the source like the `.MAR`, by `outputLocation`.

**Decision 12: the map moves to data.** `codegen` exposes what `writeMap` writes, and both
the comment and the listing use it. Same words in both.

Layout (ours, in MACRO's manner; 132 columns, 66 lines; question 6):

```
SQUARES                                  2-NOV-2026 14:05:11  govax FORTRAN V1.0-115        Page 1
TAXES                                    DUA0:[WORK]SQUARES.FOR;1

    1        PROGRAM SQUARES
    2        INTEGER I, N
    3        INCLUDE 'LIMITS.FOR/LIST'
  1 1        PARAMETER (MAX = 10)
    4        DO 20 I = 1, MAX
    5        N = I * I + K
                       ^
%FORT-W-..., ...
    6  20    CONTINUE
    7        END


Storage map for SQUARES, the main program
  I     INTEGER     $LOCAL+0   SQUARES.I
  N     INTEGER     $LOCAL+4   SQUARES.N

Cross reference for SQUARES                 (D defined, A assigned, a blank is used)
  I     INTEGER   2D  4A  5
  K     INTEGER   5
  N     INTEGER   2D  5A
  MAX   constant  1D  4
  20    label     4   6D
```

TITLE and SUBTITLE take effect on the next page, the last on a page winning, and a directive
without a string changes nothing (10.3.3). With neither, the title is the unit's name.

**Decision 13: the cross reference is recorded by `sema`,** which knows what a name is,
rather than by a second walk. Each `Symbol` and label collects positions and kinds.
`/CROSS_REFERENCE` without `/LIST` is an informational message and nothing else (ours).

## `/WARNINGS` and `/STANDARD`

- `DECLARATIONS` is the check IMPLICIT NONE already makes, reported as a warning.
- `GENERAL` (`NOGENERAL`, `NONE`) holds back `diag`'s warnings and informational messages,
  as a filter in one place; `$STATUS` follows what's shown. Errors stay.
- `ALIGNMENT` joins the keywords, the command's form of CDEC$ OPTIONS's `/WARN` (Phase 82 left
  this here).
- Unused names and labels: question 3.
- **Decision 14: `/STANDARD`'s messages are informational,** `%FORT-I-EXTENSION`, so a
  flagged program still compiles with a good status. `/STANDARD` alone is both keywords (ours).
- **Decision 15: `/STANDARD`'s checks are one walk, in `sema/standard.go`,** over the tree
  with a table of what's flagged, and for `SOURCE_FORM` over the lines (tabs, lowercase, `!`
  comments, D lines, past column 72, more than 19 continuations). The other passes stay as they are.
- **The list of extensions can be read from the LRM itself.** "Extensions to the
  FORTRAN-77 standard appear in blue print." The OCR text drops that, and the scan is
  one bit per pixel, but on the one page rendered (2-1) the blue came out as visibly lighter
  text: "(REAL\*4)" against "REAL". So a script could pick out the lighter runs across the 464
  pages for reading against the OCR. HP's appendix E helps little, being relative to
  Fortran 95. That's subtask 8's start; one page isn't proof it works throughout.

## `/DEBUG` and `/OBJECT`

- **`/DEBUG`** writes `.ENABLE DEBUG` in the `.MAR`, so plain `MACRO` gives debugger records.
  Keywords as MACRO's `debugFunctions`, and `/NODEBUG` keeps traceback as today unless
  `/DEBUG=NOTRACEBACK`.
- **Decision 16: `/DEBUG` doesn't turn the optimizer off.** The Dictionary's examples say
  `/DEBUG/NOOPTIMIZE` outright, so a programmer says so too. Under both, an
  informational message says variables may be in registers.
- **`/OBJECT`** calls `Context.Macro` on the `.MAR` just written, with `/DEBUG` carried over,
  once per compilation. MACRO's own messages appear as they do when it's run by hand.
- **Decision 17: `/OBJECT/NOMACRO` is refused,** since MACRO reads a file and the plan
  keeps the `.MAR` (ours). A compilation with errors assembles nothing.

## The debugger

Tried on two small programs, `MACRO/DEBUG`, `LINK/DEBUG`, `RUN`, both ways. **What works:**
breaking on a routine, STEP, SHOW CALLS, SHOW REGISTERS, EXAMINE/INSTRUCTION, GO to the end,
EXAMINE by number with the right name shown (`EXAMINE/FLOAT 400` is `D\D.X:  0.0000000`).

| | What happens | Where it starts |
|---|---|---|
| **D1** | `EXAMINE T.I`, `EVALUATE/ADDRESS T.K`, `EXAMINE T\T.I`: `%DCL-E-EXTRAPARAMETER, ... near ".I"`. Yet the debugger itself prints `L^T\T.K` and `D\SUB.I`, so it can't read its own output | `machine/expr.go`'s `isSymbolChar` lacks `.`, as does the path loop; the message comes from `dcl/parse.go`, and which step splits the name is subtask 2's to trace |
| **D2** | A dummy argument is `SUB.A = 4`, so SHOW SYMBOL has `SUB.A` as "constant: 4", and once D1 is fixed `EXAMINE SUB.A` would look at address 4, not the argument | The offset is what the code needs (`@SUB.A(AP)`); the debugger can't tell that from a constant. Under ENTRY it's different again (`$A1`) |
| **D3** | SHOW SYMBOL calls `D.X` "address", `D.V`, `D.DD`, `D.H`, `D.NAME` "descriptor address", and COMMON's `D.IC` "label". What EXAMINE does with each can't be tried until D1 | MACRO's types, from the directive after the label. MACRO follows real MACRO, so the compiler's output is what can change |
| **D4** | `EXAMINE X`, `I`, `V(1)`: NOSYMBOL. Only `unit.name` exists, and no subscripts | Phase 74's Decision 3: no FORTRAN debugger records |
| **D5** | `%LINE` and every stop are the `.MAR` file's lines. A stop shows `MOVAL @SUB.A(AP),SUB.$B1+4`, not the FORTRAN line a comment above it holds; STEP goes by instruction lines; no command takes a FORTRAN line | Same |
| **D6** | At `SUB+2`, `EXAMINE .PC` is named `D\BLK.$BASE+26C`, a common block at ^X450 in another psect, while EXAMINE/INSTRUCTION has it right | Probably `debugger/data.go`'s nearest-label rule. A bug whatever else is decided |
| **D7** | EXAMINE/SOURCE is a syntax error; `SHOW MODULE D` an extra parameter; `EXAMINE D\$LOCAL` NOSYMBOL (naming `D$LOCAL`) though SHOW SYMBOL lists it | General gaps, met here |
| **D8** | SHOW MODULE lists FTNRTL's 19 `FTN_` modules beside the program, "total MACRO modules: 20" | Not explained yet; to check against VMS for a shareable image |
| **D9** | Under `/OPTIMIZE` a loop's variable is in a register and its home is stale. SHOW SYMBOL also lists `SUB.$1`, `DSC$K_`, and the like | Inherent, but the storage map knows; Decision 16 warns |

**D1's rule.** A `.` inside a name after its start joins the name. `.X` and `.PC` stay "contents
of", and `.` alone the current location, so `X+.Y` is unchanged. Since `T.I` is an error today, nothing
that works now changes meaning. **A probe** (`testdata/dbg85`): MACRO labels with periods under
VMS 7.1's debugger, EXAMINE, EVALUATE, `T\T.I`, `T.A+4`, `.T.I`, SHOW SYMBOL, and the same for D2's
and D3's symbols. It needs only MACRO, so the simh VAX can run it; I'd write it for you to run, as
before. D6 to D8 can go in it too.

## Tests

No oracle for the compiler's new output, there being no VAX FORTRAN: by hand, from the rules.
Listings and cross references as goldens; `testdata/fortran` programs for each check tripped and
not tripped, both optimizer settings; `TestFortranPrograms`' silence stays required of the corpus
under the defaults; DUNGEON stays quiet under the defaults (and has its own build's `/NOCHECK`).
The debugger: oracle tests against the probe where it speaks, a new session on a compiled program
in `internal/debugger/fortran_test.go`.

## The review (2026-11-02)

The author accepted the survey's decisions and the subtask list above, and answered:

1. **Level B:** symbol names, plus subscripts and types. The debugger work stays in this
   phase. `EXAMINE V(1)` shows the element, and a REAL shows as REAL. The language stays MACRO,
   and Phase 74's Decision 3 stands, so there are no FORTRAN debugger records. How the debugger
   learns a variable's type and bounds is still open, and subtask 10 starts by settling it: what
   the compiler can write that MACRO and LINK carry into the image, or what the debugger can read
   beside it as it already reads the `.MAR` file for source lines.
2. **No checks by default,** as recommended. MANUAL.md's 14.7 lists VAX FORTRAN's default as a
   possible difference.
3. **`UNUSED` is its own keyword,** off by default and included in `ALL`.
4. **The probe:** yes. I write it, and the author runs it on the simh VAX.
5. **`/[NO]F77` stays,** as the last subtask before the close-out.
6. **The listing's layout stands as drawn:** the two-line heading, and maps and cross references
   after the whole source.

## What the probe settled (2026-11-02)

`testdata/dbg85`, VMS 7.1's run in its `vax/`. Language MACRO:

- **A period is an ordinary character of a name but for the first:** `A.B`, `E.`, `F..G`,
  `H.1`, `$.K`, `_.L`, and `R.TN\LOC.AL` as a path. So is the case: `a.b` finds `A.B`.
- **D1's proposed rule held.** `.A.B` is the contents of `A.B`, and `..A` the contents of the
  contents. `C.D`, whose parts exist and which doesn't, is NOSYMBOL for the whole name, as is
  `DBG85M.A`. `.P.Q`, though MACRO accepts that label and SHOW SYMBOL lists it, is NOSYMBOL `P.Q`:
  a leading period is always the operator.
- **`A .B` is `%DEBUG-E-MISOPEMIS`** at the `.`, as `@SUB85.N(AP)` is at the `(`.

Language FORTRAN, which `SET LANGUAGE` reaches, is different, and our images aren't in it:

- `A.B` with `A` data is `%DEBUG-W-NOTRECORD`, "cannot select component B"; `GLOB.AL` is NOSYMBOL
  `GLOB`; and `DBG85.X`, with `DBG85` a routine, is NOSYMBOL `DBG85\X`, so there `R.X` is a way
  to write a path. The radix goes to decimal.

For subtask 10, and changing what the survey assumed:

- **Level B has more help than expected.** VMS gets from `.BLKF 3`'s label an array, `DBG85.V[0:2]`,
  and `EXAMINE/FLOAT DBG85.V[1]` works, but everything is typed by MACRO's directive: `DBG85.X`
  shows as a longword, and `M(2,3)` is one dimension (`TOOMANSUB` for two subscripts).
- **D2:** `EXAMINE SUB85.A` on VMS too is an access violation at address 4. `.(.AP+SUB85.N)` is the
  argument. So a debugger that knows dummies does more than VMS's did with this MACRO.
- **D3:** a common block's `DBG85.IC` is a label, so VMS decodes it as an instruction (`SCANC`).
  That's what VMS does with the compiler's current output, not something to copy for level B.
- **D6 is fixed:** anywhere in a routine `EXAMINE` shows an instruction. That's why `.PC` looked wrong.
- **D7 is partly no gap:** `EXAMINE $LOCAL` is NOSYMBOL on VMS, though SHOW SYMBOL * lists it. It
  needs EXAMINE/SOURCE, TYPE, SHOW MODULE's name, and SHOW SYMBOL without addresses, with globals.
- **D8:** VMS's SHOW MODULE lists only the current image's modules, `SHOW MODULE/SHARE` adds a
  `SHARE$` line per image, and `SET IMAGE` reaches the rest; govax's list of FTNRTL's is wrong.
- **Real MACRO assembled `/OPTIMIZE`'s output** (`DBG85O.MAR`) without a message, a small start on
  what Phase 84 left to Phase 86. VMS's debugger read govax's images but for the MACRO bugs in the log,
  and could say no more about source lines since the volume was DUA2 here and DUA1 there.

## Questions, as put to the author

1. **How far should the debugger go, and in which phase?** You asked for this as
   part of the work; I've made it subtasks here rather than a new phase, since Phase 86 is the
   milestone, and it can move out if you'd rather. As to how far, there are three levels:
   - **A, MACRO-level, made usable (recommended):** D1, D3, D6, D7, D8; D2 by a debugger rule for
     a dummy's symbol; a statement label per FORTRAN line under `/DEBUG`
     (`SUB.S25`, so `SET BREAK SUB.S25`); a stop showing the FORTRAN comment above its line;
     and in a routine, bare `X` meaning `routine.X`. Language stays MACRO.
   - **B, A plus subscripts and types:** `EXAMINE V(1)`, shown as REAL, which means teaching the
     debugger arrays from something the compiler writes.
   - **C, real FORTRAN debugging:** the compiler writing debugger records so the language is FORTRAN.
     This reverses Phase 74's Decision 3, and is a phase of its own.

2. **What should `/CHECK`'s default be?** Nothing to hand states it, and govax has
   assumed none since Phase 75 (Phase 86's notes agree). But DUNGEON's own build instructions compile
   with `/NOCHECK`, which is odd if nothing were on, and I recall VAX FORTRAN's default as
   `(NOBOUNDS,OVERFLOW,NOUNDERFLOW)`, with programs that hash by multiplying needing
   `/CHECK=NOOVERFLOW`. **I recommend staying with none:** my memory isn't a source, and overflow on by
   default would take away integer code motion from every default compilation and could stop
   programs that run today. MANUAL.md's 14.7 would list it as a possible difference. If you know
   it was on, say so and I'll match it; the switch itself is small.

3. **Unused names and labels: on by default?** The plan lists them under `/WARNINGS`. Under GENERAL,
   which is on by default, they'd end the corpus's silence, which `TestFortranPrograms` requires, and
   would likely flag many real programs. **I recommend a keyword of their own, `UNUSED`, off by default** and included in `ALL`.

4. **May I write the debugger probe for you to run on the simh VAX?** Without it D1's rule stands
   as ours, unconfirmed. **Recommended,** since it's cheap and the debugger has been held to VMS
   until now.

5. **Keep `/[NO]F77`?** The plan says "if kept", and Phase 74's Decision 1 "late, optional". It's four small
   behaviors, but the one-trip DO reaches `ir`'s `countedLoop` and `opt`'s DO passes. **I recommend
   keeping it,** as the last subtask before the close-out, since OPTIONS `/NOF77` is in the manual and
   old programs are the reason to want it. Dropping it saves about a subtask.

6. **Is the listing's layout above acceptable?** Mostly a matter of taste: it's ours either way. In
   particular the two-line heading, and maps and cross references per unit, after the whole source,
   rather than after each unit's lines. Say if you'd rather have them after each unit.

## Progress log

- **2026-10-29:** Planned with Phase 74.
- **2026-11-02:** Subtask 1, the survey (Part A), written and reviewed; the author chose
  level B for the debugger and took the other recommendations ("The review"). The author asked
  during it for a review of debugging FORTRAN programs, after
  finding the debugger refuses the compiler's dotted names; reproduced, and made
  subtasks 2 and 10. No code changed.
  Subtask 2's probe, `testdata/dbg85`, written and run through on govax to find its own
  mistakes, then run by the author on VMS 7.1 ("What the probe settled").
- **2026-11-02:** Subtask 2. **The evaluator** (`machine/expr.go`): `isNameChar`, a period inside a
  name, and `FieldDots`, `Machine.FieldDots`, `component` for language FORTRAN. `..A` now works.
  `EvalWholeMode` says MISOPEMIS (new, with NOTRECORD) for what's left starting `.` or `(`. **SET
  LANGUAGE** (`debugger/program.go`, `useLanguage`; ours, unconfirmed, that languages but FORTRAN
  take MACRO's rule). **Paths** (`dbgsym.Lookup`): one may start at a routine, and a module's name is
  seen from its routines. **EXAMINE** anywhere in a routine is an instruction (D6). **`!XL`** is in
  capitals, as VMS's is; it showed in `NOACCESSR ... 0000000C`. **MACRO**, four bugs VMS's objects
  showed: a leading `_` was still the console dialect's scope (`_.L` became `__1_.L`); a name past 31
  characters was kept whole, now cut with `ILLSYMLEN`; and a source of 128 to 255 lines wrote the byte
  form of DEFLINES, and a `.TITLE` with no text no TTL record. Tests: `TestDebugRecords` holds DBG85M's
  and the compiler's DBG85's objects to VMS's byte for byte; `TestMacroSymbolNames`;
  `TestDottedNamesOnCompiledProgram`, on a program compiled, assembled, and linked by govax; and
  `TestDebuggerSessionOracle` replays DBG85M's and DBG85's sessions, all the name cases matching, with
  what remains listed for subtask 10 (mostly SHOW SYMBOL's form, EXAMINE/SOURCE, and D3).
- **2026-11-02:** Subtask 3, by Decisions 11 and 12. **`internal/fortran/listing`** (`Make`, from an
  `Input`) is the pager and the layout as the review accepted it, its number column as wide as the
  longest number shown (so one digit, as drawn, for a short program), tabs expanded to the source's stops,
  and control characters made blanks. `Result.Listing` is made even when the source pass stops the
  compilation (the title then the file's name, `listinginput.go`). **The map:** `codegen.Generate`
  returns each routine's `StorageMap`, its lines of text, which the MACRO file has as comments, so the words
  are the same. **Messages:** a message goes under the line it's about, even an included line nobody
  asked for; since a file included twice has each line twice, `source.Line.Serial` numbers the readings
  and `diag.Position` carries it. One about no line comes after the source, whole. **`/SHOW`**
  (`Show.With`): the survey's keywords, cut short while unambiguous; `/SHOW=INCLUDE` overrides an INCLUDE's
  `/NOLIST`. **The command:** `/LIST[=file]`, `/SHOW`, `--list`, `--list-file`, `--show`; `writeCompilerFile`
  is `writeMACRO` for either file, and FORSAMEFILE names the qualifier to use. Ours, each unconfirmed, the
  VAX FORTRAN User Manual not being to hand: each unit and each file of a `+` list starts a page, so a page has one
  unit's name; a unit starts past the last unit's last statement, so its opening comments are with it; a page that
  starts during the maps has that map's unit's name; `/SHOW` without `/LIST` is `%FORT-I-NOLISTING`, as
  Decision 13 has for `/CROSS_REFERENCE`; a source line past 132 columns isn't cut. By 10.3.3's words a TITLE on
  page 1 heads page 2, even on the program's first line, so a one-page program never shows it.
  Tests: `listing_test.go`, `TestListing*` in `compile_test.go`, `TestFortran_list`, `TestFortran_listVolume`.
- **2026-11-02:** Subtask 4, by Decision 13 and the review's answer 6. **`sema/crossref.go`:** `lookup` notes a use,
  `declare` (lookup for a specification statement's name) a definition, and `noteAssigned` an assignment, in the callers
  of `place` and `target`, since `ioUnit` uses `place` for reads too; `noteOther` is what has no `Symbol` (common blocks,
  namelists, structures, intrinsics, system routines), and `noteLabel` labels. Every mention is recorded and
  `Unit.CrossReference` settles them when asked: source order, one per place at its strongest kind, then one per line and
  kind. **`listing`:** `Input.CrossRefs`, `crossLines`, and `units`, which merges maps and cross references by unit, since either
  may be missing (no maps after errors or with `/SHOW=NOMAP`). **The command:** `/[NO]CROSS_REFERENCE`, `--cross-reference`,
  `%FORT-I-NOLISTING` without `/LIST`. **A change to subtask 3's `Line.Serial`:** lines are now numbered as they join the
  program (`reader.add`), not as their file is read, so an included line's serial falls between those of the lines around
  its INCLUDE; it still tells two readings apart, and is now the source's order. `FieldRef.NamePos` is where the record's
  name is, its `Pos` being the last field's. Ours, each unconfirmed, no manual to hand giving VAX FORTRAN's: `DATA` counts as
  A, every specification statement (SAVE and EQUIVALENCE too) as D, and NAMELIST's items and an adjustable array's bound names
  as used; rows grouped as the drawing has them (variables, constants, routines, others, labels), alphabetically within a
  group; `/SHOW` doesn't touch a cross reference; shown after errors, for the units analysis reached, with none if the source pass
  stopped the compilation. **Left out:** a statement function's dummies and a DATA implied-DO's variable (in no table of the
  unit), the unit's own name, a record's fields, and a text library's names the program doesn't use. **A change to the accepted drawing,** by the
  author's decision at hand-over: a mention on an included line has its include depth, as the source lines do (`listingCrossRefs`),
  so the drawing's `MAX constant 1D 4` is `MAX constant 1:1D 4`, since a plain 1 can't be told from the main file's line 1. Two
  files at one depth still share a prefix, as they do among the source lines. **Known limit:** `REAL SQRT` and a reference to it show as a REAL
  variable's row. Tests: `TestCrossReference*` in `listing_test.go` and `compile_test.go`, `TestNoCrossReference`, `TestFortran_list`,
  `TestFortranCommand`.
- **2026-11-02:** Subtask 5, by Decisions 1 to 9. **The check** is the IR's `InRange` (three operands, not an `Op`), written
  `INDEX x,low,high,#1,#0,Rn`; `ir/lowerchecks.go` wraps each subscript where it's lowered (`mem`, `fieldMem`, `adjustableIndex`) and each
  substring's bounds (`checkedSubstring`), `checkedSubscript` moving the bounds by the subscript's constant so that `TABLE(I+1)` keeps its address's form.
  It stays inside the expression rather than going ahead of the statement, so a branch's `.AND.` keeps it from running. A substring is
  two `INDEX`es at most: `S(I:)`, `S(k:J)`, and `S(I:I)` need one, and `S(I:I)` keeps its constant length. **The optimizer:** `Forward` folds a check its facts
  prove and keeps a failing one; `mayFault` says yes to one, as `unsafeToMove` always did; `CommonSubexpressions` and `LoopInvariants` take one as a candidate
  (`exprKey`, `costOf`), so that it moves only with a sure use. **Overflow:** `ir.Routine.Checks` (Decision 8, read in `prologue`); `ir.MayOverflow`, which `mayFault(e, overflow)` and `unsafeToMove(e, overflow)` ask;
  `ir.FoldFor`, which leaves an overflowing or underflowing constant operation to trap. `^M<...,IV>` on every entry point and a variable format expression's routine. sema already refuses a
  constant expression that overflows, so lowering's `arith` is unchanged. **Found on the way:** ASHL sets V, so `ISHFT` and `IBSET`/`IBCLR` would trap, though they aren't
  arithmetic: under overflow they're the new `Rot`, ROTL, which doesn't (a shift then masked). `findStepped` is off under overflow, since a stepped value runs ahead of the program's and past its last trip. `Forward` now looks inside a computed address's index (`Addr`, a packed field's), which only a second run did, as `TestPassesOnCorpus` showed on `checks.for`.
  **Underflow:** `BISPSW #^X40` after each `.ENTRY`, `writeUnderflow`. **ADJARRDIM** is FTNRTL's `FTN$ADJARRDIM` (`ftn_control.mar`, vector 656, GSMATCH 1,17), a `LIB$STOP`, called from a block `checkArrayBounds` makes at each
  entry point. **Decision 9:** `SUBSCRIPTRANGE` (`checkConstantSubscripts`), for elements, records, and fields. **The command:** `/[NO]CHECK[=(keyword,...)]`, `--check`, `CLI_FORCHECK`, OPTIONS's `/[NO]CHECK` (`BADCHECK`); `sema.Checks.With`, keywords cut short as `/SHOW`'s. Ours, each unconfirmed:
  `/CHECK=UNDERFLOW` is severe (Decision 7), and a zero-extent `A(0)` is ADJARRDIM, as Decision 5's words have it (`adjust.for`'s `SIZES(V,0,4)` does this, and is fine unchecked); a `DO` variable stepping past the integer limit after its last trip overflows (`optloop.for`'s LARGEST).
  **Tried once, not kept:** the whole corpus under `/CHECK=ALL`, both ways: no panic and no false `SUBRNG`. Nine programs differ, each for a reason read from its source: word arithmetic that wraps on purpose (`lengths`, `mixed`, `mixedg`, `optreg`, `powers`), `optloop`'s limit, `handlers`' own `INTOVF`, `adjust`'s zero extent, and `directives`' moved PC. DUNGEON under `/CHECK=BOUNDS`, both ways: no `SUBRNG`, and the same game.
  Tests: `TestFortranCheck` (28 programs, each with the check and without, both optimizer settings, both loaders), `TestFortranCheckQualifier`, `testdata/fortran/checks.for` (nothing to find, in the corpus), and `TestImage`.
- **2026-11-02:** Subtask 6, by Decision 10. **`source/options.go`:** `followUnit`, called with each statement, tracks the reader's
  `form` (`command`'s, as OPTIONS statements before the unit change it, until its END) and `inUnit`, by the parser's and lexer's rules: an OPTIONS
  statement, a directive, or a statement of only a label opens no unit, and END is `isEnd`, section 5.5's initial line. `readFile` now gets only a line's kind ahead of time (which doesn't
  depend on the field's end), and `statement` and `directive` split each line under the form they find. `sema`'s `prologue` passes over the qualifier. The form follows the statements, so an included file's lines are their unit's,
  and OPTIONS or END may be in an included file, or a unit run on into the next file of `A+B`. **Found on the way:** section 1.4.3 says OPTIONS's qualifiers take the command's abbreviations, and `prologue` took only whole names, so `OptionsQualifier`
  (one table, used here and by `prologue`) gives a name cut short its whole form when it stands for only one of them, for every qualifier; and an OPTIONS statement inside a unit said NOTIMPL,
  which it never will be: it's now OPTIONSFIRST. Ours, each unconfirmed, no manual saying: the statement's own lines are read under the form before it, so a qualifier takes effect only after them; several OPTIONS statements before a unit each apply, the last winning;
  a unit a header interrupts with no END (NOEND, so the compilation fails anyway) keeps the form to the next END, the one place this pass and the parser differ. Tests: `TestOptionsExtendSource`, `TestOptionsExtendSourceFiles`, and `TestOptionsQualifier` in `source_test.go`;
  `TestOptionsExtendSource` in `compile_test.go`; `sema_test.go`'s OPTIONS cases; `testdata/fortran/extsource.for`, in the corpus both ways.
