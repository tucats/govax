# Phase 110 — Fortran 90: the compiler's usability

**Status:** planned 2026-11-17 with Phase 99 (`PHASE-99 - Fortran 90.md`,
Part C). Needs Phases 107 to 109. Subtask 1, the survey (Part A), done
2026-11-20 and accepted by the author the same day ("The author's answers"); its
A.4 revises the subtasks below.

## Goal

What Phase 85 gave FORTRAN-77, extended to Fortran 90:

- **`/STANDARD=F90`**: flag what isn't Fortran 90 (DEC's extensions, govax's), and
  `/STANDARD=OBSOLESCENT` (or a keyword the survey chooses) the standard's B.2
  obsolescent features: arithmetic IF, real and double precision DO variables,
  shared DO termination, alternate return, PAUSE, ASSIGN and assigned GO TO and
  FORMAT, H editing; free form's blank rules reported, not only enforced;
- **`/WARNINGS`**: INTENT(OUT) read before set, an unused module entity, a USE'd
  name hidden, a generic call that needed a conversion;
- **listings** (`/LIST`, `/SHOW`) of free-form source, with each scope's storage
  map (module psects, frame variables, descriptors), and **cross references** by
  scope, with names from modules marked;
- **`/CHECK`**: BOUNDS on sections and through descriptors, shape conformance at
  run time, POINTERS (use of a disassociated pointer), and an unallocated
  array's use;
- **`/DEBUG` and the debugger**: the `/DEBUG` table (`dbgtable`) learns
  descriptor-based arrays, allocatables and pointers (their status), derived
  types and `%`, module variables (`M%X` or `M.X`: the survey decides), and an
  internal procedure's host names; `EXAMINE A(2:5)` of an assumed-shape array;
- **`/OPTIMIZE`** measured on the Fortran 90 corpus: whether the scalarizer's
  loops need more (loop fusion across statements and across a WHERE construct's
  assignments, the run-time overlap test of Phase 102's question 1, and the
  other cases Phase 102's "Where things stand" lists).

## What earlier phases leave in place

- Phase 85's listing package, cross reference, checks, warnings, `/STANDARD`'s
  walker (`sema/standard.go`, with tables of what FORTRAN-77 has), and the
  debugger's FORTRAN support (`debugger/fortran.go`); Phase 89's SET SCOPE.

## Design notes

- `/STANDARD`'s tables gain a column: FORTRAN-77, Fortran 90, extension. A
  construct's row says in which standard it is, so `/STANDARD=F77` and `=F90` are
  one walk.
- The debugger's language stays FORTRAN; `%` is a component selector beside `.`,
  as in VMS's debugger for Fortran 90.

## Open questions

1. `/STANDARD`'s keywords: `F90`, `F77`, `SYNTAX`, `SEMANTIC`, `SOURCE_FORM`,
   `OBSOLESCENT`? The survey proposes a set modeled on the HP compiler's.

## Subtasks (provisional)

1. **Survey** (done 2026-11-20; Part A, accepted).
2. **`/STANDARD=F90` and the obsolescent features.**
3. **`/WARNINGS`** for the new constructs.
4. **Listings and cross references** with scopes and modules.
5. **`/CHECK`** on shapes, descriptors, pointers, allocation.
6. **`/DEBUG` and the debugger** on Fortran 90's data.
7. **The optimizer measured** on the corpus; what's worth doing now done.
8. **Close-out**: FORTRAN-INTERNALS.md, MANUAL.md, HELP FORTRAN.

Revised by the survey: A.4 is the list to follow.

# Part A — the survey (subtask 1)

Read for it: the standard's 1.4 (what a conforming processor must detect and
report), 1.6.2 and B.2 (the obsolescent features), section 3 (source form), and
10.1.1's last paragraph (a format that isn't a FORMAT statement need not be
checked); the HP Fortran manual's Appendix A (deleted and obsolescent features)
and 13.3 (OPTIONS); Phase 85's survey and decisions; and the "Where things
stand" of Phases 99 to 109, each of which names something for this phase. And
what govax has: `sema/standard.go` and `source/standard.go` (`/STANDARD`),
`sema/warnings.go` and `diag/warnings.go` (`/WARNINGS`), `listing` and
`sema/crossref.go`, `sema/checks.go` and `ir/lowerchecks.go` (`/CHECK`),
`dbgtable`, `codegen/debugtable.go`, and `debugger/fortran.go`.

Probes, all on 2026-11-20 and all scratch work, not kept:

- a module and a main program that use most of Fortran 90 (a derived type, a
  defined operator, PRIVATE and PUBLIC, allocatable, pointer, and target arrays,
  WHERE, SELECT CASE, a named DO, EXIT and CYCLE, a constructor, kinds, an
  internal procedure with an assumed-shape dummy, ADVANCE=), compiled
  `/STANDARD=ALL`, then `/LIST/CROSS_REFERENCE/SHOW=ALL/WARNINGS=ALL`;
- the obsolescent features, and small programs for each warning in the goal;
- nine programs compiled `/CHECK=ALL` and run until they fail: a section past
  its bounds, an assumed-shape element past its bounds, an unallocated array, a
  deallocated array, a nullified scalar and array pointer, shapes that don't
  conform, SIZE of an unallocated array;
- two debugger sessions (`/DEBUG/NOOPTIMIZE`, `MACRO/DEBUG`, `LINK/DEBUG`) on a
  program with a module's variables, a derived type, an allocatable array, a
  target and two pointers, and an internal procedure with an assumed-shape dummy;
- five array kernels compiled `/OPTIMIZE`, their inner loops read from the MACRO
  file; and `TestFortran90Programs -v`'s gains for the whole corpus.

There's no VMS probe, there being no VAX Fortran 90. gfortran `-std=f95` is a
witness for `/STANDARD=F90` (finding 3).

## A.1 What the standard asks

Only `/STANDARD` answers to the standard. The rest of the goal (warnings,
listings, checks, the debugger, the optimizer) is the processor's own business,
and the HP *User Manual*, which would give DEC's qualifiers, isn't to hand. As in
Phase 85, the keywords below that come from memory of HP Fortran's command are
marked so, and are unconfirmed.

- **What must be reported** (1.4). A conforming processor can detect and report,
  in a program unit: (2) a deleted or obsolescent form, as far as the numbered
  syntax rules and their constraints show it; (3) a form or relationship the
  syntax rules and constraints don't allow; (4) a kind the processor doesn't
  have; (5) source form or characters section 3 doesn't allow; (6) names used
  against chapter 14's scope rules; (7) intrinsic procedures not in chapter 13.
  (4) and (6) are errors in govax already. (2), (3), (5), and (7) are this
  phase's. A format that isn't a FORMAT statement (a character constant or a
  variable) needn't be checked (10.1.1).
- **The obsolescent features** (B.2), all of them FORTRAN-77's: (1) the
  arithmetic IF; (2) REAL and DOUBLE PRECISION DO variables and loop control
  expressions; (3) shared DO termination, and a DO ending on a statement other
  than END DO or CONTINUE; (4) a branch to END IF from outside its block; (5) the
  alternate return; (6) PAUSE; (7) ASSIGN and the assigned GO TO; (8) an assigned
  FORMAT specifier; (9) the H edit descriptor. Fortran 90 deletes nothing. The HP
  manual's A.3 gives the same nine.
- **Source form** (3.1 to 3.3): the character set (no tab, no `$`); names of
  up to 31 letters, digits, and underscores; free form's lines of up to 132
  characters and 39 continuations, and its blank rules; fixed form's columns and
  19 continuations, `!` comments and `;` allowed in both forms. INCLUDE is
  Fortran 90's (3.4). D lines and tab format aren't.

## A.2 What govax has

| Area | What the probes found |
| ---- | --------------------- |
| `/STANDARD` | One walk judges a unit by FORTRAN-77 (`Conformance`: `SYNTAX`, `SOURCE_FORM`; `%FORT-I-EXTENSION, extension to FORTRAN-77: …`). On the Fortran 90 probe it flagged `%` as "a reference to a record's field", RESULT, IMPLICIT NONE, unlabeled DO and END DO, the new intrinsics (KIND, SIZE, SUM, LBOUND), and ADVANCE=, and **nothing else**: not MODULE, USE, CONTAINS, INTERFACE, a derived type's definition, `::`, any attribute, ALLOCATE, `=>`, NULLIFY, WHERE, SELECT CASE, EXIT, CYCLE, a construct name, `(/ … /)`, sections, keyword arguments, defined operators, `1.0_DP`, or free form itself (`source/standard.go`'s `LineMarks` passes over every free-form line). The walker's comments mark the Fortran 90 cases it only descends through, "which /STANDARD judges from Phase 110". |
| Obsolescent features | Not flagged in either form. They're FORTRAN-77's, so the FORTRAN-77 reading is right not to flag them. **A bug:** `ASSIGN 10 TO LAB` in free form is `%FORT-E-BLANKINNAME` at TO (and in fixed form under `/F90` it compiles): the lexer's blank check sees `TOLAB` as one token. |
| `/WARNINGS` | Keywords `DECLARATIONS`, `GENERAL`, `UNUSED`, `ALIGNMENT`, `ARGUMENT_CHECKING` (Phase 103). INTENT(OUT) read before set is **already there**, INTENTOUTREF (Phase 103, statement order), but its message points at the SUBROUTINE statement rather than at the use. `UNUSED` reports an unused dummy and, in principle, a module's private names, but `INTEGER, PRIVATE :: A` and `PRIVATE :: B`, both never used, **weren't reported**. Nothing warns of a variable set and never read, or of an internal procedure's local hiding a host's name. A generic reference needing a conversion **can't arise**: the choice is exact (GENERICNONE is an error), and a name extending an intrinsic falls back to the intrinsic with no conversion. |
| Listing, map | Free-form lines listed as they are; each scope its own page, map, and cross reference, headed `SHAPES.DIST`, `PROBE.SHOW` (Phases 103, 106); a module's map in its psect (`$MOD$SHAPES+8`); "Type POINT, 8 bytes" with its components; a use-associated name `from SHAPES`; `REAL, POINTER (:)`; register lines under `/OPTIMIZE`. Missing: ALLOCATABLE (`REAL*8 (:)  $LOCAL+0`, which is its descriptor, not its elements), TARGET, INTENT, OPTIONAL, PUBLIC and PRIVATE. And **a leak:** `$W  LOGICAL (?)  $LOCAL+184  PROBE.$R1`, a WHERE mask's temporary, listed among the program's names. |
| Cross reference | Names, kinds, `from SHAPES`, constructs (`OUTER construct`), derived types, modules. Missing: generic names, `OPERATOR(.PLUS.)` and `ASSIGNMENT(=)` (in the module and where they're used), a module procedure in its module's cross reference (`DIST` isn't listed; `ADDP` only as the operator's `12`). A host's variable that an internal procedure sets (`LINE  76A`) is in the host's cross reference, but nothing in the internal procedure's says that it reaches LINE. |
| `/CHECK` | `BOUNDS`, `OVERFLOW`, `UNDERFLOW`. **BOUNDS already covers Fortran 90's forms:** a section past its bounds and an assumed-shape element through its descriptor trap SUBRNG, and shapes that conform only at run time fail with FOR's NOTCONFOR (Phase 102), BADARRDESC (Phase 104). **What it misses:** a nullified scalar pointer's use is ACCVIO at address 0; a nullified array pointer's element, and an unallocated array's element, are SUBRNG only because their descriptors' bounds happen to be 1:0; after DEALLOCATE, `SUM(A)` ran without a word and printed 1.000000 from the freed block; and `SIZE(A)` of an array never allocated printed 1. |
| `/DEBUG`, debugger | See D1 to D8 under finding 9. What works: explicit and allocatable arrays (`DBG.A(2,3): 4.000000`), assumed-shape dummies through their descriptors (`V(1)` to `V(4)`), integers, FORTRAN lines at stops, STEP by line, SET SCOPE's frames. |
| `/OPTIMIZE` | On by default (Phase 84). In the corpus it cuts the array programs' instructions by 30 to 72% (`rtemps` −72.2%, `seqargs` −65.7%, `seqvms` −64.6%, `sieve` −52.2%, `transfer` −51.9%, `solver` −36.8%, `sorter` −35.7%), and the rest, which are mostly printing, by 0 to 5%. The programs are too short, and FTNRTL's share too large, to say what to do next; the kernels in finding 10 do. |

## A.3 Findings, and what they change

1. **`/STANDARD` reads Fortran 90 by FORTRAN-77's tables, and misses most of
   it.** So even the FORTRAN-77 check is wrong today for a Fortran 90 program:
   every new statement and form is an extension to FORTRAN-77 and should say so.
   **Proposed:** each of the walker's tables gets the skeleton's column. A row
   says whether its construct is FORTRAN-77's, Fortran 90's, both, or neither
   (DEC's), and the standard chosen picks which rows are reported. One walk still
   does it all. The rows to add, by table:
   - *statements*: every Fortran 90 kind the lexer has (`Module` to `WhereBlock`,
     and `TypeDef` beside DEC's `Type`, `Record`, `Structure`);
   - *declarations*: `::`, each attribute, kind selectors and `KIND=`/`LEN=`,
     `CHARACTER(LEN=…)`, initial values with `=`, and DEC's `REAL*8` length forms
     (FORTRAN-77 has only `CHARACTER*n`; Fortran 90 too, so `REAL*8` is an
     extension to both);
   - *expressions*: `%` beside DEC's `.` field (the walk already tells them apart,
     `ast.FieldRef.Percent`), constructors of both kinds, sections, vector
     subscripts, keyword arguments, defined operators, `_kind` on constants, the
     new relational spellings (`==`, `/=`, …), `"…"` constants;
   - *specifiers*: OPEN's, INQUIRE's, and the transfer statements' new keywords
     and values (Phase 109) in a column of their own;
   - *edit descriptors*: B, EN, ES, `r/`, `Bw.m` are Fortran 90's; Q and `$`
     remain extensions to both;
   - *intrinsics*: chapter 13's names, generic and specific, as a second list;
     DEC's (`IISHFTC`, `SIZEOF`, `JNINT`, `ZEXT`, …) are extensions to both;
   - *names*: 31 characters and `_` are Fortran 90's, `$` neither's.

   The message names the standard: `%FORT-I-EXTENSION, extension to Fortran 90:
   the STRUCTURE statement`. DEC's things that Fortran 90 adopted (DO WHILE, END
   DO, IMPLICIT NONE, INCLUDE, NAMELIST, `!` comments, long names, RECURSIVE) aren't
   flagged under F90.
2. **The obsolescent features need meaning as well as form.** Five can be seen
   in the tree: the arithmetic IF, PAUSE, ASSIGN, the assigned GO TO, an alternate
   return (`*` in a dummy list, `*label` in a CALL, `RETURN i`), and H in a FORMAT
   statement. Four need what sema knows: a DO variable's type, a DO's ending (shared,
   or not END DO or CONTINUE: `endDo`), a jump to END IF from outside (`labels.go`),
   and an integer variable as a format (`PRINT LAB`). **Proposed:** the walk reports
   the first five, and sema reports the other four where it decides them, to the
   same `extensions` list (as `noteIntrinsic` already does for intrinsics, Phase 85's
   one departure from "one walk"). A new message, `%FORT-I-OBSOLESCENT, obsolescent in
   Fortran 90: the arithmetic IF statement`, informational as EXTENSION is (Phase 85's
   Decision 14). Under the FORTRAN-77 reading there's nothing to flag. Fixing the
   ASSIGN bug (A.2) belongs here.
3. **Source form under Fortran 90.** Under F90, `SOURCE_FORM` flags only what
   section 3 forbids: a tab, a D line, a `$` in a name, a line past 132
   characters (including comment lines, which Phase 99 left unreported), and more
   than 39 continuations in free form or 19 in fixed form. Lowercase, `!`, `;`,
   and free form itself aren't flagged. Under FORTRAN-77, free form is one finding
   per file ("free source form"), not one per line. The goal's "free form's blank
   rules reported, not only enforced" is already met: free form's blank rules are
   errors (BLANKINNAME, NEEDBLANK, Phase 99), and fixed form has none. A token split
   by `&` without the second `&` stays unreported (Phase 99's choice: the source
   pass can't see it). **The witness:** every program in `compare.py`'s
   `PROGRAMS90` builds with gfortran `-std=f95`, so under `/STANDARD=F90` each
   should report nothing but its obsolescent features and the few things Fortran 95
   deleted. That's a test, and a check on the tables.
4. **What else `/STANDARD=F90` must judge by meaning** is short. A REAL argument
   to CEILING, FLOOR, or MODULO in an initialization expression (7.1.6.1; Phase
   108) and a nonstandard intrinsic in a constant expression (Phase 100's folding
   allows any) are noted by sema the same way. Everything else that the standard's
   constraints forbid is already an error. `SEMANTIC` stays out, as in Phase 85.
5. **`/WARNINGS`.** Of the goal's four, one is already there, one is a bug, one
   can't arise, and one is new:
   - *INTENT(OUT) read before set* is INTENTOUTREF (Phase 103). It should point at
     the use, not at the SUBROUTINE statement.
   - *An unused module entity*: `neverUsed` already means to report a module's
     private names (`!s.private` lets them through), but neither form of PRIVATE was
     reported. To find out why in subtask 4; the module's mentions may not be settled
     before `reportUnused` runs. A public entity is for its users, unknowable when
     compiling.
   - *A USE'd name hidden* can't happen in a scope that USEs it: redeclaring a
     use-associated name is an error. It can happen in an internal procedure, whose
     own declaration hides a host's name, use-associated or not. That's legal and
     often deliberate (a local loop index I), so it's a new warning, HOSTHIDDEN,
     under a keyword (question 3).
   - *A generic call that needed a conversion* can't arise (A.2): dropped.
   - New, beside UNUSED: a variable set and never read (`W = 2`, nothing reading W),
     UNREAD (question 4). It uses the cross reference's mentions, as UNUSED does,
     so it's in statement order, not flow, and not for a dummy, a COMMON or module
     variable, an EQUIVALENCE, or a VOLATILE name.
6. **Listings and cross references:** Phases 103 and 106 already did most of
   this. What's left:
   - (a) generic names, `OPERATOR(op)`, and `ASSIGNMENT(=)` as entries, with
     "generic", "operator", "assignment", in the module that defines them and in
     each unit using them;
   - (b) a module procedure in its module's cross reference;
   - (c) attributes in both the map and the cross reference (ALLOCATABLE, TARGET,
     INTENT(IN), OPTIONAL, PRIVATE); and in the map, an allocatable or automatic
     array's descriptor named as such (`descriptor at $LOCAL+0`);
   - (d) no compiler temporaries (`$W`) in the map, as for every other `$` cell;
   - (e) in an internal procedure's cross reference, the host names it reaches,
     marked `from PROBE` as a USE's are marked `from SHAPES`.

   The rest (each contained unit on a page of its own, the two-line heading) stands.
7. **`/CHECK`: BOUNDS is done; POINTERS is what's missing.** **Proposed:**
   `[NO]POINTERS` (included in `ALL`), checking at each statement that uses it
   that a scalar pointer's cell, or an array pointer's or allocatable array's
   descriptor POINTER, isn't 0. A pointer never assigned is the same as a nullified
   one in govax, since pointers are cleared at entry and in static storage (Phase
   107). The check counts as something that faults, so the optimizer may hoist it
   from a loop but never drop it (Phase 85's Decision 2). Failures are two new FOR
   conditions in govax's wording (Decision 15): 311, a pointer not associated, and
   312, an array not allocated. They come from FTNRTL routines (`FTN$NOTASSOC`,
   `FTN$NOTALLOC`; GSMATCH 1,32), as NOTCONFOR does. A nullified array pointer, or an
   unallocated array, then gets the right message rather than SUBRNG's. Subtask 6
   also finds why SUM after DEALLOCATE read the freed block (the descriptor, or an
   address the optimizer kept). The `/CHECK` the HP compiler has for pointers isn't
   in the manual to hand: the name is the goal's.
8. **One correction to the skeleton's design notes:** the debugger's language for
   a compiled program is MACRO (Phase 85's answer 1: the module is MACRO's, and
   `%DEBUG-I-INITIAL` says so). SET LANGUAGE FORTRAN is what turns on `.` as a field
   selector (`Machine.FieldDots`). `%` joins `.` there, and in the FORTRAN path
   (`fortranItem`), whatever the language.
9. **The debugger on Fortran 90's data.** What the sessions showed:

   | | What happens | Where it starts |
   | --- | --- | --- |
   | **D1** | `EXAMINE PT` of a derived type is two hex longwords; `PT%X` and `PT.X` are NOSYMBOL | The table's REC type has no layout (Phase 85 left records as longwords) |
   | **D2** | `EXAMINE A(1:2,1)` and `EXAMINE T(2:3)` are `%DCL-E-EXTRAPARAMETER … near ":2"`, for every array, not only Fortran 90's | The grammar's location parameter ends at `:` |
   | **D3** | In an internal procedure, the host's `PT`, `A`, `I`, `T` are NOSYMBOL. `DBG.PT` works. After `SET SCOPE/CURRENT 1` they're still NOSYMBOL | `findFortranVar` looks only in the innermost FORTRAN frame's unit, and a bare name ignores the scope for anything but frame variables |
   | **D4** | `EXAMINE P`, `EXAMINE S` (pointers) are NOSYMBOL | `codegen/debugtable.go` leaves pointers out (Phase 107's finding 12) |
   | **D5** | `EXAMINE STORE.COUNT` (a module's variable) shows `00000007` as a MACRO label's longword, not the table's INTEGER; bare `COUNT` from a user's internal procedure is NOSYMBOL | A module has no E line, so its unit isn't found by the dotted path, and the label answers instead; D3 for the rest |
   | **D6** | `SET BREAK DBG.INNER` stops before the entry code copies V's descriptor, so `EXAMINE V` says `%DEBUG-W-NOTALLOC` until a STEP | A routine break is at its first instruction |
   | **D7** | `SHOW CALLS` gives the main program's module as STORE | The MACRO module is named for the file's first unit, here the module. It stays: the module is the file |
   | **D8** | `SET BREAK %LINE 20` is the MACRO file's line 20; FORTRAN's line 20 is `DBG.$LINE20` | Phase 85's level B; noted, not proposed |

   **Proposed:**
   - the table's new lines: `;DBG$ S name size` and a `;DBG$ C …` line per
     component (offset, type, size, bounds, and a nested type's name), a V line's
     type `REC:name`, so EXAMINE shows an object component by component and
     `PT%X`, `PTS(2)%Y`, and DEC's `R.F` find a component;
   - pointers: a new place kind for a scalar pointer (the cell holds the target's
     address), and an array pointer Described as an allocatable is, with
     `%DEBUG-W-NOTASSOC` when the address is 0;
   - a module's units in the table, found by `M.X` (question 6);
   - a bare name looked for in the unit's hosts in turn (the unit's name less its
     last part), and in the frame of the scope SET SCOPE chose, with a frame host's
     variables through the FP an internal procedure is given (Phase 103);
   - sections: the location parameter takes `:` inside parentheses, and `A(2:5)`,
     `A(1:2,1)`, and a section with a stride show each element, as `wholeArray`
     already does for a whole array;
   - a break at a FORTRAN routine (one with an E line) stops after its entry code,
     at its first statement's line (question 7).
10. **The optimizer, measured on kernels.** Instructions per element in the
    inner loop, under `/OPTIMIZE`:

    | Kernel | Loop | Per element |
    | --- | --- | --- |
    | `A = B`, explicit shape | `MOVF (R9)[R6],(R8)[R6]`; `AOBLEQ` | 2 |
    | `A = B + 1.0` then `C = A * 2.0` | two loops, A stored and loaded again | 2 + 2 (fused: 3) |
    | WHERE (A > 0) B = SQRT(A) ELSEWHERE B = 0 | two loops, the mask tested in each | 6 + 4 (fused: about 7) |
    | `SUM(A)` | `ADDF2 (R9)[R6],R7`, the sum in R7 | 2 |
    | `V = V + W`, assumed shape | `MULL3 R6,S1,R0`; `ADDL2 A1,R0`; the same for W; `ADDF2`; `AOBLEQ`, the strides and bases read from memory each time | 6 |

    So explicit-shape loops are as short as they can be, and a reduction keeps its
    sum in a register (Phase 102's "stays in memory" no longer holds here). Two things
    stand out:
    - **Assumed-shape and strided loops cost three times as much,** and assumed
      shape is how a Fortran 90 program passes arrays. Strength reduction (an address
      per operand stepping by its stride, the strides in registers) makes it 4. A
      second copy of the loop for when every stride is the element's size makes it 2.
      Phase 104's "second stride multiply" is part of this.
    - **Fusion** of adjacent array assignments, and of a WHERE construct's arms, over
      the same shape where `ArrayOrder` says neither meets the other, saves a loop's
      overhead and the second mask test: about 25% of such loops.

    The run-time overlap test (Phase 102's open question 1, Phase 107's answer 6), and
    Phase 104's list (`A = F(X)` into A directly, SPREAD and CSHIFT where used, the
    copies for sections to old dummies), want a kernel each before anything is done.
    A whole-array copy as MOVC3 would show as fewer instructions without being
    faster, so the instruction count can't judge it: left out unless host time is
    measured. Kernels go in `testdata/fortran90` as `opt*.f90` programs, so
    `TestFortran90Programs -v` logs their gains as the old corpus's `opt*.for` do.
11. **Noticed, not Fortran 90's, not proposed:** a FORTRAN program's traceback has
    no line numbers (MACRO's traceback records carry none), where VAX FORTRAN's
    had the FORTRAN line. For a later phase, if wanted.

## A.4 The subtasks, revised

1. **Survey** (this Part A) (done 2026-11-20; accepted by the author the same day).
2. **`/STANDARD=F90`**: the tables' column and the Fortran 90 rows (finding 1),
   so that the FORTRAN-77 reading also flags every new form; the keywords
   (question 1); source form under F90 (finding 3); the ASSIGN bug; the witness
   test over `PROGRAMS90`. (done 2026-11-20)
3. **The obsolescent features** (finding 2) and the judgments by meaning (finding 4).
4. **`/WARNINGS`** (finding 5): INTENTOUTREF's position, the module's private
   names, HOSTHIDDEN, UNREAD.
5. **Listings and cross references** (finding 6, (a) to (e)).
6. **`/CHECK=POINTERS`** (finding 7), FOR numbers 311 and 312, FTNRTL's two routines.
7. **`/DEBUG` and the debugger** (finding 9), in two parts:
   - 7a. Derived types and `%`; sections in EXAMINE.
   - 7b. Pointers, module variables (`M.X`), host names and the scope, the break after
     the entry code.
8. **The optimizer** (finding 10): the kernels, strength reduction for strided loops,
   fusion; the overlap test and Phase 104's list as the kernels say.
9. **Close-out**: FORTRAN-INTERNALS.md, MANUAL.md (14.3's qualifiers, 14.7, 14.10,
   14.13), HELP FORTRAN.

## A.5 Questions for the author

1. **`/STANDARD`'s keywords** (the skeleton's open question 1). Proposed: `F77`
   and `F90` say which standard to judge by; `[NO]SYNTAX` and `[NO]SOURCE_FORM` say
   what's looked at, as now; `[NO]OBSOLESCENT` flags B.2's features (it means
   nothing under F77); `ALL` and `NONE`. HP Fortran's, from memory, were `F90`,
   `F95`, `SYNTAX`, `SEMANTIC`, `SOURCE_FORM`, `MIA`, `NONE`. `SEMANTIC` and `MIA`
   stay out as in Phase 85. **Recommendation:** these.
2. **The standard when none is named.** `/STANDARD` alone, or `/STANDARD=SYNTAX`:
   by the unit's `/F90` setting (F90 for free form and `/F90`, FORTRAN-77 for `/NOF90`,
   with OBSOLESCENT on under F90), or always F90? **Recommendation:** by `/F90`, so an
   old program's check is what it was.
3. **HOSTHIDDEN's keyword.** Under `DECLARATIONS` (it's about where a name is
   declared, and off by default), `GENERAL` (on by default, but noisy for loop
   indexes), or a keyword of its own? **Recommendation:** `DECLARATIONS`.
4. **UNREAD under `UNUSED`** (a variable set and never read), or not at all?
   **Recommendation:** under `UNUSED`.
5. **`/CHECK=POINTERS` for allocatable arrays too**, or a keyword of their own?
   Both test the same field of a descriptor. **Recommendation:** one keyword,
   `POINTERS`, covering both.
6. **A module's variable in the debugger:** `M.X`, as every compiled name already
   is (`DBG.A`, `SUB.X`), or `M%X`, which collides with `%` as a component
   selector? **Recommendation:** `M.X`.
7. **A break at a FORTRAN routine:** after its entry code, at its first statement,
   where its dummies can be read (what a FORTRAN programmer expects), or at its first
   instruction, as now and as for MACRO? **Recommendation:** after the entry code, for
   a unit with an E line only.
8. **The optimizer's scope:** strength reduction and fusion in this phase, and the
   overlap test and Phase 104's list only if their kernels show a gain worth it?
   **Recommendation:** yes.

## The author's answers (2026-11-20)

The author read the survey, accepted the plan, and took every recommendation:

1. **`/STANDARD`'s keywords:** `F77` and `F90` (which standard), `[NO]SYNTAX`,
   `[NO]SOURCE_FORM`, `[NO]OBSOLESCENT` (meaningless under F77), `ALL`, and `NONE`;
   `SEMANTIC` and `MIA` stay out.
2. **The standard when none is named** follows the unit's `/F90` setting: F90 (with
   OBSOLESCENT on) for free form and `/F90`, FORTRAN-77 for `/NOF90`.
3. **HOSTHIDDEN** is under `/WARNINGS=DECLARATIONS`.
4. **UNREAD** (a variable set and never read) is under `/WARNINGS=UNUSED`.
5. **`/CHECK=POINTERS`** covers allocatable arrays too: one keyword.
6. **A module's variable in the debugger** is `M.X`.
7. **A break at a FORTRAN routine** (a unit with an E line) stops after its entry
   code, at its first statement; MACRO routines are unchanged.
8. **The optimizer:** strength reduction for strided loops and fusion in this phase;
   the run-time overlap test and Phase 104's list only if their kernels show a gain.

A.4's nine subtasks are the phase's list; subtask 2 is next.

## Progress log

- **2026-11-17:** Planned with Phase 99.
- **2026-11-20:** Subtask 1, the survey: Part A, for review. Probes on scratch
  programs (the Fortran 90 probe under `/STANDARD`, `/LIST`, `/WARNINGS`; nine
  `/CHECK=ALL` runs; two debugger sessions; five kernels; the corpus's gains). It
  found that `/STANDARD` misses most of Fortran 90 under either reading, that
  `/CHECK=BOUNDS` already covers sections, descriptors, and conformance, that much of
  the listing work was done by Phases 103 and 106, and that assumed-shape loops cost
  three times what explicit-shape ones do. Two bugs: ASSIGN in free form
  (BLANKINNAME), and a module's private names never reported by `/WARNINGS=UNUSED`.
  Also one mispositioned message (INTENTOUTREF). A.4 revises the subtasks, and A.5
  has eight questions.
- **2026-11-20:** The author accepted the survey and every recommendation in A.5
  ("The author's answers"). Subtask 2, `/STANDARD=F90`, is next.
- **2026-11-20:** Subtask 2, `/STANDARD=F90`. `sema.Conformance` gains `Standard`
  (`StandardOfUnit`, `StandardF77`, `StandardF90`) and `Obsolescent`/`ObsolescentGiven`;
  the keywords are `F77`, `F90`, `[NO]OBSOLESCENT` beside the old ones, and a list that
  says nothing of what's looked at (none, or only `F77`, `F90`, and `NO` forms) starts
  from `ALL`, so `/STANDARD=F90` looks at everything and `/STANDARD=NOSYNTAX` is now the
  source form alone (before, nothing). `judging(f90)` resolves a unit's standard from
  its `/F90` setting (the author's answer 2); OBSOLESCENT follows SYNTAX under Fortran 90
  unless given, and its findings are subtask 3's. Every table has a column,
  `source.Standards` (`InF77`, `InF90`): a `row` is a message's text and the standards
  that have the thing, and `extensions.add` flags it when the standard judged by isn't
  among them, so one walk serves both readings. `statementRows` has every Fortran 90
  statement; `newIntrinsics` is chapter 13's names beyond FORTRAN-77's (sema's own
  `f90Intrinsics` is only the names VAX FORTRAN lacked, which leaves out IAND and the
  other bit functions Fortran 90 adopted); `f90Specifiers` and `f90Values` are Phase
  109's keywords and words. The walker now descends into Fortran 90's statements
  (ALLOCATE, `=>`, NULLIFY, CASE, WHERE, interface bodies, a derived type's components)
  and notes `::`, attributes, selectors, `= value`, deferred and assumed shape, kind
  parameters, `==` and its kin (`ast.Binary.Symbolic`, set by the parser), `"…"`
  constants (`ast.CharConst.Quoted`, read from the source text at the token), `%`,
  keyword arguments, constructors, defined operators, construct names, named ENDs, a DO
  without loop control, and Fortran 90's B, O, and Z constants (in DATA only; elsewhere,
  as DEC allows, an extension to both). Sections and vector subscripts are noted by
  `sectionList`, which alone tells a section from a substring. The message is
  `extension to %s: %s`, naming the standard. Source form: each `source.Mark` carries its
  standards and the `/F90` setting in force (`Line.F90`, set as a line is added); under
  Fortran 90 a tab anywhere, a free-form comment line over 132 characters, and more than
  39 free-form continuations are flagged, and small letters, `!`, and INCLUDE aren't;
  under FORTRAN-77 a free-form file is one mark, "free source form", at its first line.
  The ASSIGN bug: the lexer now splits ASSIGN's TO off as a keyword token after the
  label's digits (`assignTo`), with free form's blank rules on it, and the parser takes
  it (the old `TOI` path kept for a statement without digits). Tests:
  `TestStandardF90Flagged`, `TestStandardF90Quiet`, `TestStandardF77SeesF90`,
  `TestStandardF90SourceForm`, `TestStandardWhichStandard`, and the witness,
  `TestStandardF90Witness` (`standard90_test.go`: compare.py's `PROGRAMS90` read from
  the script, each program's other files compiled first with their module texts kept),
  which finds nothing under `/STANDARD=F90`; the same programs under `F77` give about
  9,000 findings, every Fortran 90 form among them. HELP FORTRAN's and MANUAL.md's
  `/STANDARD` say so. Subtask 3, the obsolescent features, is next.
