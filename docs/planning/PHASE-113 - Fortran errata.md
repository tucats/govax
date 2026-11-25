# Phase 113 — Fortran errata

**Status:** planned 2026-11-22. Phase 111's close-out planned Phase 112 as the
Fortran errata, Fortran 95's additions among them; subtask 1, the survey (Part A),
was done the same day, and at its review the author split the phase
("The author's answers", below): **Fortran 95 is Phase 112**
(`PHASE-112 - Fortran 95.md`), and this doc, renamed from
`PHASE-112 - Fortran errata.md` with its survey, is **Phase 113, the errata**:
the rest of Part A's list. `CONVERT=` is **Phase 114**
(`PHASE-114 - FORTRAN CONVERT.md`). Needs Phase 112, so its probe round checks
both. Part A was written while this was Phase 112, so its **proposed: 112** marks
mean this errata phase or Phase 112, as Part B sorts them.

## Goal

A complete, sorted **to-do list** of every gap left behind, and as much of it
done as the author chooses:

- **VAX FORTRAN** (Phases 74 to 89): the language, the compiler, FTNRTL, and
  the run-time libraries a FORTRAN program calls (MTHRTL, LIBRTL's STR$ and
  LIB$, the services);
- **Fortran 90** (Phases 99 to 111): every NOTIMPL, every "measured, left for
  later", every unconfirmed choice;
- **Fortran 95's additions** were group D of the list; they are Phase 112's
  now, and A.4 below only records them;
- **the tools beside them**: MACRO, LINK, LIBRARY, ANALYZE, the debugger,
  and what the VMS rounds found in VMS's tools or ods2's volumes.

The list may run past what one phase does. Each item ends in one of these
places: **done** here, with a test; **its own phase** (planned and named);
**a probe** (a question only a VMS run answers, for the next simh round);
**measured and left**; **not planned** (by decision, with the reason); or
**already resolved**, which the survey records where the item was written.

**Resolved items are corrected where they were written** (the author's
direction, 2026-11-22): an old phase doc's "Where things stand", a
DEVIATIONS.md entry, or MANUAL.md's lists that still names something a later
phase did gets a note saying it was resolved and in which phase
("*(Resolved in Phase 88.)*"). A DEVIATIONS.md entry whose every bullet is
resolved moves to `DEVIATIONS-RESOLVED.md`. The close-out began this for
Phases 81 to 86 and DEVIATIONS.md's Phase 27 and 28 entries; the survey
finishes it for every phase doc from 27 on.

## What earlier phases leave in place

- Each phase's "Where things stand" (Phases 78 to 111; Phase 86's is the
  VAX FORTRAN program's, Phase 111's the Fortran 90 program's).
- DEVIATIONS.md, by phase; MANUAL.md's 14.7 ("What isn't there", "Choices")
  and 14.13 ("What Fortran 90 isn't here").
- The compiler's NOTIMPL, F95ONLY, and UNRECSTMT reports, findable in the
  code (`sema/notimpl.go`, `report("NOTIMPL"`), and FTNRTL's NOTIMP.
- The oracles' lists of known differences (`ftn86VMSDifferences`,
  `f90rObjectDifferences`, `tir111Differences`, `expectedDifferences`,
  `for88Differences`, ...), each item with its reason.

# Part A — the survey (subtask 1, 2026-11-22)

The close-out's seed list, checked against the code and made whole from the sources above.
Swept: the follow-up sections of every phase doc from 27 on ("Where things stand", "Future
work", "Future features", "Known differences", "Not planned", "Choices of ours"; the
planning-time "Open questions" sections were answered when their phases began and hold no
to-do items), DEVIATIONS.md, MANUAL.md's 14.7 and 14.13, the compiler's NOTIMPL, F95ONLY, and
UNRECSTMT sites (`grep '"NOTIMPL"\|notImplForm\|"F95ONLY"'` under `internal/fortran`: 53),
FTNRTL's NOTIMP (one, CONVERT=), the system service vector (179 of its 310 entries without a
routine), and the oracles' difference lists. A few items were checked by compiling small
programs (A.5, C.5, D).

Each item ends with its **place**, as the Goal names them. For items this phase could do, the
place is a *proposal* for the author's review (**proposed: 112**); nothing is implemented yet.

- **112** — proposed for this phase, with a test.
- **own phase** — a program of its own, planned and named (or already planned).
- **probe** — a question only a VMS run answers, for the next simh round (A.12).
- **measured** — measured, left.
- **recorded** — an unconfirmed choice no VMS can settle (there is no VAX FORTRAN or Fortran
  90); it stays recorded where it was made.
- **not planned** — by decision, with the reason.
- **resolved** — done by a later phase, and now marked so where it was written (A.13).

## A.1 VAX FORTRAN: the language and the compiler

1. `CONVERT=` other than NATIVE (FTNRTL's only NOTIMP, `ftn_open.mar`; Phases 81, 86). VAX
   FORTRAN's conversions are between VAX formats and IEEE, IBM, CRAY, and big-endian
   integers, on unformatted records. **Own phase if wanted** (question 5): it is a library of
   conversions, unformatted I/O's every path, and no probe could check it (VMS's FORRTL could:
   **probe** first). *(Phase 114, the author's answer 5.)*
2. A running program's files on the host (Phase 81's answer 3; MANUAL.md 14.7). RMS reaches
   only volumes; COPY brings a host file in. **Not planned**: govax's RMS is a volume's, and
   Phase 97's `vax.host.sysprv` is the host's one door.
3. Object modules from the compiler itself (Phase 74's Future work; `/OBJECT` runs MACRO).
   **Not planned** (Phase 99 and 111 already say so): the MACRO file is the design.
4. `/STANDARD=SEMANTIC`, a FUNCTION with no parentheses and `&s` (the tree doesn't keep them),
   a bare `X` (Phase 85). **Proposed: 112** for the two forms the tree can be made to keep;
   SEMANTIC (what a program means) **not planned**: the manuals at hand don't say what VAX
   FORTRAN flagged, so every rule would be ours.
5. A typed name that's an intrinsic or system function (`REAL SQRT`) still gets a `$LOCAL`
   cell and a storage map line (Phase 86). Checked 2026-11-22: still so (`SQ.SQRT: .BLKL 1`).
   **Proposed: 112**, small.
6. Frame alignment a longword at most, whatever `/ALIGN` asks; `/CHECK` for a frame variable
   read before it's set (Phase 89, items 3 and 4). **Not planned** for alignment (it needs a
   register each entry point aligns, for no program that needs it); the uninitialized check
   **not planned** (Decision 9 of Phase 89 still stands; VAX FORTRAN had none).
7. An AUTOMATIC record of a natural structure only on a longword (a consequence of 6); an
   aligned block's layout differing between separately compiled files with no message
   (Phase 82). **Not planned**: neither LINK nor the compiler sees both layouts.
8. DICTIONARY, CDEC$ PACK, CPAR$ directives, threads and `/REENTRANCY`, `NON_RECURSIVE`
   (Phases 82, 86, 89). **Not planned**, as recorded.
9. *(New.)* Algebraic identities left in the code: `Y*1.0`, `0.0*Z`, and a widening then
   narrowing (`SNGL(DBLE(Z))` is CVTFD then CVTDF) compile as written, `/OPTIMIZE` or not
   (checked 2026-11-22). Phase 78 left them to Phase 84, which didn't take them up. Removing
   them changes which reserved operands fault, so a rule is needed. **Measured** (E): a
   program doesn't write them, and a constant folded into them is rare.
10. *(New.)* A REAL constant that a short literal holds (`1.0`, `0.5`, `2.0`) is a `$PDATA`
    longword (`ONE.$KF1`), where `S^#1.0` would do and save a memory reference. **Proposed:
    112**, small (codegen's operand forms; MACRO already takes floating short literals).
11. *(New.)* A statement the lexer doesn't know (PURE, ELEMENTAL, FORALL today) inside a
    module or before CONTAINS cascades into a dozen messages (AFTERCONTAINS, ENDKIND,
    TWOMAINS) as the parser loses the unit's structure. D's statements remove the case that
    shows it most; **proposed: 112** as a recovery for any unrecognized statement that starts
    with a known prefix word.

## A.2 FTNRTL and the run-time libraries

1. STR$'s ADD, DIVIDE, MUL, RECIP, ROUND, and STR$ELEMENT_R8 NOTIMPL (Phase 83's answer 2;
   `librtl/str.go`). Still NOTIMPL. Decimal string arithmetic, documented in the manual,
   checkable on VMS. **Proposed: 112** (question 7), with a **probe** for VMS's answers, or
   left. *(Done in subtask 7, its probe `testdata/str113` for subtask 12's round.)*
2. LIB$DECODE_FAULT, SIM_TRAP, FIND_IMAGE_SYMBOL, GET_OPCODE, and LIB$INITIALIZE (Phase 83's
   answer 4; Phase 92 also notes no LIB$INITIALIZE), and Phase 60's balanced trees
   (INSERT_TREE, LOOKUP_TREE, TRAVERSE_TREE). **Own phase**: Phases 60 and 61, still planned
   (question 4). No FORTRAN program in the corpus needs them.
3. Service vector entries without a routine: **179 of 310** today (Phase 83 counted 191;
   Phases 93 to 98 added some). Most are journaling, DECdtm, vector processor, POSIX,
   licensing, and spares, which govax won't have. Ones a FORTRAN program might call:
   `$GETCHN` (`ftn86VMSDifferences`), `$GETDEV`, `$ASCUTC`/`$BINUTC`/`$NUMUTC`/`$GETUTC`/
   `$TIMCON`, `$SETDDIR`, `$DEVICE_SCAN`, `$PROCESS_SCAN`, `$GETLUI`, `$SETPRA`, `$BRDCST`,
   `$SNDERR`, `$SETSTK`, `$SETSSF`, `$HASH_PASSWORD`, `$GRANTID`/`$REVOKID`, `$SETIME` (out
   by Phase 94's review), RMS's `$SPACE` (Phase 88's item 8). **Proposed: 112** for `$GETCHN`,
   `$GETDEV`, and the UTC five (small, documented, a FORTRAN program's usual calls); the rest
   **as needed** (Phase 83's Decision 6). *(`$GETCHN`, `$GETDEV`, and the UTC five done in
   subtask 8, its probe `testdata/svc113` for subtask 12's round.)*
4. The last bits of OTS$ negative powers and MTHRTL's complex functions against VMS's (Phase
   86; `ftn86VMSDifferences`'s POWERS, POWBITS, MATHFNS). **Measured**: six orders of
   operations tried, none fits; a probe with more cases could find VMS's method (**probe**,
   low priority).
5. FTNRTL's per-statement paths, about 170 instructions a record; `astrace` parsing formats
   80,000 times (Phase 84, subtask 11). **Proposed: 112** to look at the 80,000 parses (a
   FORMAT's cache that misses is a bug, not a speed); the paths themselves **measured**.
   *(Done in subtask 8: a CHARACTER constant format was a run-time format, parsed at every
   execution; it is now compiled as a FORMAT is.)*
6. The same frames after an unwind: RECIO_OPE on a unit a handler unwound a statement from,
   when the routine is called again from the same place (Phases 83, 89; DEVIATIONS.md).
   **Not planned**: nothing in the frames tells the activations apart; recorded as the known
   difference (`TestFor83GoneOnFTNRTL`, `TestRTL89SameFrames`).
7. Heap blocks after an unwind: an allocatable local's or a function value's block stays
   allocated until the next use (Phase 104). **Recorded**; a fix would need an unwind
   handler in every unit with such locals. Not planned unless the author wants it.
8. A NAMELIST object past 8 levels or 15 dimensions (Phase 109). **Not planned**: FTNRTL's
   limits, NOTIMPL by name.
9. *(New.)* LIBRTL's OTS$ packed decimal conversions (`OTS$CVTDP_R9` and its kin) and
   `OTS$$CVT_MUL`, which Phase 72 left, are still missing stubs. **As needed**: FTNRTL doesn't
   call them; a VMS-compiled image would.
10. *(New.)* MTHRTL's routines end before a handler's continue, where VMS's go on (DEVIATIONS.md,
    Phase 77). **Recorded**.

## A.3 Fortran 90

1. **NOTIMPL, named** (MANUAL.md's "What Fortran 90 isn't here"; each checked in the code
   2026-11-22). Proposed places, cheapest first:
   - CSHIFT, EOSHIFT, SPREAD with a `DIM=` not constant; RESHAPE's `ORDER=` not constant
     (`sema/transform.go`). The reductions already take a run-time DIM= (Phase 104, subtask
     8), so the machinery is there. **Proposed: 112.** *(Done in subtask 2.)*
   - A `BACK=` that is an array (`sema/charfuncs.go`): an elemental argument. **Proposed: 112.**
     *(Done in subtask 2.)*
   - COMPLEX(16) to SUM, PRODUCT, and their kin (`complex32Intrinsic`, three sites).
     **Proposed: 112.** *(Done in subtask 2.)*
   - A component or function value of CHARACTER length zero (`decl.go`'s "a CHARACTER length
     of zero here"). **Proposed: 112.** *(Done in subtask 2.)*
   - A statement function of a derived type (`decl.go`). **Proposed: 112**, small. *(Done in
     subtask 2.)*
   - TRANSFER to a CHARACTER MOLD of zero or run-time length, or of such an array
     (`sema/transfer.go`). **Proposed: 112.** *(Done in subtask 2.)*
   - An assumed-shape array to `%LOC`, `%REF`, `%DESCR`, or a system subroutine
     (`arguments.go`): `%LOC` and `%REF` of a contiguous one could take its first element.
     **Not planned** beyond a better message; the rule is ours.
   - A CHARACTER array expression of run-time length not made of names, as an actual argument
     or array constructor value (`shaped.go`, `sequence.go`, `constructor.go`). **Not
     planned** (Phase 104's "ours").
   - A name an internal procedure's declarations use that its host uses only in executable
     statements (`internal.go`). **Not planned**: Phase 103's four-step host analysis would
     need a fifth pass for it; declaring the name in the host is the cure.
   - A CHARACTER pointer of run-time length but a `LEN=*` dummy (`pointer.go`). **Not
     planned** (Phase 107).
2. **Modules** (Phase 106): ENTRY in a module procedure; an external procedure a specific of
   two generics; a type known by two local names; a hidden entity whose name the module has
   for something else; a COMMON block a module and a module it uses both declare; a structure
   with initial values, an unnamed nested structure, a constant whose bits are no number, in a
   module's text (`modwrite.go`). **Not planned**, ours, but for structures with initial values
   in a module's text, which default initialization (D.3) brings to every module with a
   derived type: **proposed: 112** with D.3. *(A derived type's default values in a module's
   text resolved in Phase 112, subtask 5; a STRUCTURE with initial values in subtask 2 here.)*
3. govax's definition modules (`$SSDEF` and the rest) as module files to USE (Phase 106,
   "later, if wanted"). **Proposed: 112**, small (`sysdef` already writes them as text;
   `USE $SSDEF` is not a legal name, so the spelling is question 8).
4. **Noticed, not changed**: NOTRECORD's wording for `F(A)%N`; `Y(D(1)%N)` in a bound
   BADBOUND (Phase 107); EXPONENT and FRACTION faulting on a reserved operand in their MOVF;
   TRANSFER to a derived type never a constant (Phase 108). **Proposed: 112**, each small.
   *(Done in subtask 2: FUNCPART for the first.)*
5. *(New.)* An explicit-shape array of no elements, `REAL A(0)` or `B(5:1)`, is BOUNDORDER
   under `/F90` too (checked 2026-11-22; MANUAL.md's table lists it). Fortran 90's 5.1.2.4.1
   makes its extent zero; VAX FORTRAN calls it an error. **Proposed: 112**, under `/F90`
   only (question 8), so nothing old changes (`TestNoF90Unchanged`). *(Resolved in Phase
   112, subtask 3.)*
6. *(New.)* A module's defined operators in a fixed-form `/NOF90` unit (MANUAL.md's table):
   VAX FORTRAN reads `A.B.C` as a field. **Not planned**, by design.
7. Not planned, ours (Phase 104): a constructor of a substring's values. **Not planned.**
8. Not Fortran 90 or 95: INTEGER(8); Fortran 2003's `[ ]`, ALLOCATABLE components and dummies,
   procedure pointers, bounds remapping, `ISO_C_BINDING`, stream access, DECIMAL=, ROUND=;
   DEC's later extensions. **Not planned** (Phases 99, 107, 109, 111).

## A.4 Fortran 95's additions (the author's go)

*Moved to Phase 112 at the review (`PHASE-112 - Fortran 95.md`, whose Part A starts from
this list), with A.1.11, A.3.2's structures with initial values in a module's text, and
A.3.5. Kept here as the survey found it.*

Checked 2026-11-22 by compiling one program with all of them: none is in, each as listed. The
HP Fortran manual (`VMS-FORTRAN-LANGUAGE.txt`) describes every one (FORALL 69 times, ELEMENTAL
193, CPU_TIME 8, and its deleted-feature notes); the Fortran 90 texts at hand are no help.

1. FORALL, the statement and the construct, with masks and nested FORALL and WHERE inside:
   UNRECSTMT today. On Phase 102's scalarizer and overlap rules (a FORALL is an array
   assignment whose index set is named). **Proposed: 112.** *(The statement resolved in
   Phase 112: the statement in subtask 8, the construct in subtask 9.)*
2. PURE and ELEMENTAL procedures, module and internal ones among them: UNRECSTMT. PURE's
   constraints checked; an ELEMENTAL reference with array arguments scalarized as an
   intrinsic elemental is (`ir`'s `elementalCall`); a PURE procedure allowed in FORALL and
   specification expressions. **Proposed: 112.** *(Resolved in Phase 112: PURE, its
   constraints, and specification functions in subtask 6; ELEMENTAL in subtask 7; FORALL's
   use of PURE procedures in subtask 8, the statement's.)*
3. `NULL()` and `=> NULL()` initialization; default initialization of components (NOTIMPL,
   `derived.go`): a variable of such a type is initialized at its storage's start (static:
   `$LOCAL`'s bytes; frame, automatic, allocated: at entry or ALLOCATE, as pointer components
   are cleared today, Phase 107's `clearRecords`); INTENT(OUT) dummies of such a type
   initialized at entry. **Proposed: 112**, with A.3.2's module text. *(Resolved in Phase 112:
   `NULL()` and `=> NULL()` in subtask 4, default initialization and the module text in subtask
   5.)*
4. CPU_TIME (a subroutine; `$GETJPI`'s CPUTIM in FTNRTL). **Proposed: 112.** *(Resolved in
   Phase 112, subtask 11.)*
5. A WHERE inside a WHERE, and ELSEWHERE with a mask (F95ONLY, `sema/where.go`, three sites).
   **Proposed: 112.** *(Resolved in Phase 112, subtask 10.)*
6. MAXLOC's and MINLOC's `DIM=` (KEYWORDNAME today), CEILING's and FLOOR's `KIND=`
   (KEYWORDNAME today), and Fortran 95's other intrinsic changes: SIGN of a negative zero (no
   such thing on the VAX: nothing to do), `KIND=` on others the HP manual lists, and NULL's
   MOLD=. **Proposed: 112.** *(Resolved in Phase 112: MAXLOC's and MINLOC's `DIM=` and CEILING's
   and FLOOR's `KIND=` in subtask 11, NULL's MOLD= in subtask 4.)*
7. Zero widths in output, `I0`, `B0`, `O0`, `Z0`, `F0.d` (OUTCONERR today, FTNRTL's
   `ftn_edit.mar` and `ftn_float.mar`), under the F90 flag only. **Proposed: 112.** *(Resolved
   in Phase 112, subtask 12, with namelist input's comments.)*
8. `/STANDARD=F95` beside `F77` and `F90`: F95's additions not flagged under it, the deleted
   features (a REAL DO variable, branching to END IF from outside, PAUSE, ASSIGN and assigned
   GO TO, the H edit descriptor) flagged as deleted, Fortran 95's obsolescent list. Whether
   `/F90` units are judged by F95 by default is question 9. **Proposed: 112.** *(Resolved in
   Phase 112, subtask 13, with no F95 keyword: `/STANDARD=F90` judges by Fortran 90 with Fortran
   95's additions, and flags Fortran 95's deleted features as DELETED and its obsolescent ones.)*
9. *(New.)* Automatic deallocation of allocatable arrays at a procedure's end and every
   pointer starting disassociated are already Fortran 95's in govax (Phase 99's Decision 8,
   Phase 107's answer 2): **resolved**, nothing to do.

## A.5 The optimizer, measured and left

1. Global CSE and PRE, autoincrement, a rotated test copied to its way in, unread
   `Call.Result`s, read-only data no statement uses (Phase 84). **Measured.**
2. A loop's copy after the DO's own store; a copy into a register before the loop sets it; a
   `$H` in memory for lack of a register (Phase 84). **Measured.**
3. A run-time test of two sections' overlap choosing a loop's direction; `A = F(X)` into A;
   SPREAD and CSHIFT as elements; copies of sections to old dummies (one for INTENT(OUT) too);
   a whole-array copy as MOVC3; fusion of arrays of run-time extent (Phases 102, 104, 110).
   **Measured.**
4. `caseForm`'s thresholds, unmeasured (Phase 101), fcalc's dispatch not hot enough; what
   internal procedures cost the optimizer (Phase 103). **Measured** (nothing showed).
5. Real MACRO assembling `/OPTIMIZE`'s output. **Resolved** in Phase 86 (marked).
6. A.1.9's identities. **Measured.**

## A.6 MACRO

1. `PROG+LIB/LIBRARY`, the positional macro library (Phases 27, 86; still only
   `/LIBRARY=(...)`, `dclcmd/macro.go`). **Proposed: 112**, small (MACRO's list already reads
   `A+B`, `dclcmd.MacroList`).
2. No object for a module with errors, where VMS's MACRO writes one with severity 2
   (DEVIATIONS.md, Phase 111). **Proposed: 112**: the object is well defined (VMS's TIRR
   and TIRX, `testdata/tir111`), and LINK's "compilation errors" message comes with it.
3. A register name in a data expression (`.LONG R1`), unsettled (Phase 111). **Probe.**
4. VMS MACRO's occasional byte of a long immediate run through the stack
   (`tir111Differences`). **Not planned**: no rule found after a probe, and the image is the
   same.
5. `.ENABLE TRUNCATION` and `VECTOR` ignored; messages in govax's form; only the first
   undefined symbol reported (Phase 27). **Proposed: 112** for every undefined symbol (VMS's
   MACRO lists each; small); the rest **not planned**, by design.
6. Object encoding choices no fixture confirms (Phase 27); a `;` inside `<...>` in a macro
   call, and `.ERROR`/`.WARN`'s text (Phase 28). **Probe.**
7. `MACRO A+B`'s numbering of files and `/DEBUG`'s source file, a `.END` before the last file
   (Phase 86). **Probe.**

## A.7 LINK, LIBRARY, and ANALYZE

1. LINK doesn't translate a logical name for an options file (`LINK X/OPTIONS`; Phase 86).
   **Proposed: 112**, small.
2. `testdata/link82`'s C22, C23, C26, C27, and C29 differ from VMS (Phase 82; DEVIATIONS.md's
   VEC entry for C29). **Not planned** until a program needs it (as recorded).
3. The GST padded to a whole block where real LINK ends mid-block (Phase 29's Decision 7); the
   image's symbol tables past the DST not compared in `TestFTN86Objects` (Phase 86). **Not
   planned** for the padding (Decision 7 stands); **proposed: 112** for the comparison, a
   test only.
4. LINK's `STACK=` doing nothing with paging off, and a stack past P1's top 1,024 pages meeting
   a P1 expansion (Phase 89). **Not planned**: paging off's fixed stack is by design; paging
   on does both.
5. LIBRARY's `/COMPRESS`, `/CROSS_REFERENCE`, `/BEFORE`, `/SINCE`, `/ONLY` (Phases 83, 90);
   real DCX reduction (Phase 90); a new library allocated only the blocks it uses (Phase 28,
   Phase 90's "Allocation when the volume is full"); WD4's block counts and W13's binary keys
   (Phase 90). **Not planned** for this phase (not Fortran's); the LIBRARIAN rules are
   **probe** items (Phase 90's list).
6. ANALYZE/IMAGE showing the debug tables' contents (Phase 40's Future expansion). **Not
   planned.**

## A.8 The debugger

1. Real FORTRAN debugger records, SET LANGUAGE FORTRAN (Phase 85's review, level C; Phases 74,
   99, 110). **Own phase if wanted** (question 6): it is the DST's language records in MACRO
   or an object writer, and the debugger reading them. *(Not planned, the author's answer 6.)*
2. `SET BREAK %LINE n`, SHOW CALLS's line column, TYPE, and EXAMINE/SOURCE counting the MACRO
   file's lines (Phases 110, 111). **Proposed: 112**: the `/DEBUG` table already maps lines
   both ways (`UNIT.$LINEn`, `showFortranLine`). *(Resolved in subtask 9.)*
3. A component after subscripts in an expression (`EVALUATE P(2)%X + 1`; EXAMINE takes it;
   Phase 110). **Proposed: 112.** *(Resolved in subtask 9.)*
4. Phase 42's known differences (DEVIATIONS.md items 1 to 10; `expectedDifferences`): (9),
   the ACCVIO text, is **resolved** (Phase 55 gave `$GETMSG`'s table VMS 7.1's text; marked).
   (3), CALL's `%VAL`/`%REF`/`%DESCR` and "value returned is", is with the future features
   below. The rest **not planned** or **probe**, as their entry says.
5. Phase 42's future features still not done (its Part B, in progress): DEPOSIT/INSTRUCTION;
   DEFINE, DELETE, SHOW DEFINE; IF, FOR, WHILE, REPEAT and DO clauses; SEARCH, SET/SHOW
   MARGINS and MAX_SOURCE_FILES; SHOW EXIT_HANDLERS and stepping into a handler from an
   exception break; CALL's argument forms; DBG$INIT and SET PROMPT; SPAWN; SET MODULE.
   EXAMINE's types, the built-ins, SET LOG and SET OUTPUT, SET SCOPE, TYPE and EXAMINE/SOURCE,
   and WATCH's `/AFTER` and `/TEMPORARY` are **resolved** (marked). **Not this phase**: they
   are Phase 42's Part B, not Fortran's.
6. `testdata/dbg85`'s differences (`SHOW SYMBOL *`'s VMS images, `@(AP+N)`), `dbg85c.dlg` not
   replayed (Phase 85); Phase 89's unconfirmed scope details (item 5); Phase 42's unconfirmed
   rules (DEVIATIONS.md). **Probe.**

## A.9 The VMS rounds' open items

1. VMS 7.1's XQP and ANALYZE/DISK_STRUCTURE failing on an index file extension header ods2 made
   (Phase 111; DEVIATIONS.md). **Probe** (let VMS grow an index file past its header and
   compare), then the fix in ods2 (**proposed: 112** once the probe says which field).
   `tools/odscheck`, the seed of an ANALYZE/DISK_STRUCTURE/REPAIR phase: **own phase**.
2. Real LIBRTL on govax faulting at `SHARE$LIBRTL+0CE1` (Phase 86). **Proposed: 112** to find
   out why (it may be a CPU or activation bug), if the author wants it; no probe needed.
   *(Resolved in subtask 10: govax called LIBRTL's transfer address, LIB$EMODD's.)*
3. RECURAST's WAITED (Phase 89; a longer timer settles it) and SECNDS within a second (Phase
   86). **Probe** for RECURAST; SECNDS is timing, **not planned**.
4. `testdata/idx88`'s split points, check characters, DATA_FILL, SIDRs, contiguity, `$PUT`
   after a keyed `$GET` (Phase 88's items 1 to 6). **Probe** (more cases), low priority.
5. *(New.)* `ftn86AwaitingVMS`: RECARRAYS and RECDYN (Phase 105), and FTNRTL's FTN_ALLOC
   module (`ftn86NewModules`, Phase 104), are newer than `testdata/ftn86`'s last VMS run.
   **Probe**: the next round runs `ftn86.com` again (f90r's round covered them in Fortran 90
   form only).
6. *(New.)* `recursiveDifferences`' ALIGNR (a frame record's 16-byte alignment on VMS's stack)
   and OPTFLOWR (ENTRY AGAIN reading an unsaved local). **Recorded**: both are what an
   undefined value happens to be.

## A.10 Unconfirmed choices

Each phase's own list (Phases 75, 80, 81, 83, 84, 85, 88, 89, and 100 to 110), the ones a user
sees in MANUAL.md's 14.7 "Choices". Sorted:

- **A VMS run could settle** (FORRTL, LINK, MACRO, LIBRARIAN, the debugger): Phase 80's
  terminal record limits, unit 6 read at a terminal, units -1 to -4 by statement, ERRSNS
  after a specifier took the error, an item's error PC; Phase 81's current record after
  REWIND/BACKSPACE/ENDFILE/UNLOCK, DELETE by number in odd cases, a keyed record -1's
  status, an implicit READ's FAC, INQUIRE by unit and file, a last FIX record cut short, a
  relative file's growth, FORRTL's by-name NAME; Phase 88's longword on a word key; Phase
  83's `$FORIOSDEF` names; Phase 90's LIBRARIAN rules; Phase 89's debugger items; Phase 42's
  rules. **Probe**: one FORRTL round (as `testdata/for81`'s) and one debugger round.
- **No VMS can settle** (the compiler's own: there is no VAX FORTRAN or Fortran 90): Phase 75's
  and 82's representation choices, 83's INCLUDE order and BADUSE, 84's aliasing and loop
  rules, 85's layouts and `/CHECK` default, 100 to 110's messages, FOR numbers 301 to 312,
  module file form, NAMELIST forms, and the rest of each list. **Recorded**, where they are.

## A.11 Small things seen on the way

1. A full volume `%DCL-E-BADFILESPEC` where VMS says the device is full; the Ctrl/Y level's
   prompt without `-` continuation (Phase 86). **Proposed: 112**, small, both. *(Done in subtask 11.)*
2. COPY's `%COPY-S-COPIED` without `/LOG` (Phases 83, 87, 91; `dclcmd/copy.go` still prints it
   by default); TYPE ending a line after a record with no carriage control
   (Phase 83); STR$MATCH_WILD with `%` (Phase 83). **Proposed: 112** for COPY (VMS says
   nothing; every log that shows it would change: BUILDFTN.COM's and the tests'); TYPE
   **probe**; STR$MATCH_WILD **probe**. *(COPY done in subtask 11.)*
3. DIRECTORY/TOTAL: **resolved** in Phase 91 (`rms/directory.go`'s `Total`; marked in Phase
   88's doc). ods2's null type (`rms.ExplicitNullType`, Phase 88): **not planned** (it works;
   moving it into ods2 is tidiness).
4. *(New.)* DEVIATIONS-RESOLVED.md holds copies of 29 entries DEVIATIONS.md still has open,
   from Phase 32's to Phase 71's (`[Phase 35] An octaword in R12 or above`, the Phase 42 to 51
   choices, `[Phase 71] Process-permanent files`, the template `[Phase NN]`), their statuses
   still open or deferred: copied over in bulk at some time, not resolved. **Proposed: 112**:
   take the copies whose text is the same out of DEVIATIONS-RESOLVED.md, and compare the
   others first. (A 30th, the console's `SET name=value`, really was resolved, in Phase 50:
   this survey took it out of DEVIATIONS.md and marked the copy.) *(Done in subtask 11.)*
5. *(New, not Fortran's, recorded for completeness.)* Phase 87's work volume lock and Windows
   locking, SAVE/ROM to a volume, `vax.library` after the volume's library; Phase 91's default
   name's search list; Phase 94's RUN/DETACH and INSTALL's other subcommands; Phase 95's
   remote logins and AUTHORIZE's unkept fields; Phase 96's `--mount` of a RAM disk; Phase
   97's EDIT and SET SHARE_PREFIX; Phase 98's print queues and LIB$GETQUI; Phase 90's grammar
   leftovers (TEST, SHOW EXPAND, SHOW MAPS, SET UIQUANTUM, CANCEL MEMORY with no handlers).
   **Not this phase**: each stays in its phase's "Where things stand". The grammar leftovers
   could go here as a cleanup if the author likes (question 10). *(The grammar's leftovers done
   in subtask 11: all but SHOW EXPAND were already gone.)*

## A.12 The probe round, if there is one

Gathered from the **probe** places above, for one simh session after this phase's
implementation (so the round also checks what this phase does): the index file extension
header (A.9.1); `ftn86.com` again (A.9.5); RECURAST with a longer timer (A.9.3); STR$'s
arithmetic (`testdata/str113`, written in subtask 7) and MATCH_WILD with `%` (A.2.1, A.11.2); a FORRTL round on Phase 80's and 81's
choices and Fortran 95's zero widths on output, which FORRTL may not take (A.10, D.7); MACRO's
`.LONG R1`, `<A;B>`, `.ERROR`/`.WARN`, `A+B` numbering, and an object with errors (A.6); a
debugger round (A.8.6); TYPE after a record with no carriage control (A.11.2); `CONVERT=`'s
answers, if A.1.1 goes ahead; from subtask 11, a full volume's messages (COPY's WRITEERR and
an output utility's, `-RMS-F-FUL` and `-SYSTEM-W-DEVICEFULL`, and whether COPY leaves the
partial file), DELETE-E-DELVER's `$STATUS`, a bad specification's message in PURGE and in COPY's
output, INITIALIZE and START with no qualifier, and PURGE/KEEP=0. *(Since the refresh, A.15.3: Phase 112's programs awaiting VMS,
`f90rAwaitingVMS` and `ftn86`'s ZEROVAX, and FORRTL's zero widths.)*

## A.13 Resolved items, marked where they were written

This subtask marked these, each "*(Resolved in Phase N.)*" or a like note where the item was
written: Phase 40's Future expansion (shareable images, Phase 68); Phase 42's Future features
(EXAMINE's types and the built-ins, its own subtasks 18 and 20; WATCH and TRACE's `/AFTER` and
`/TEMPORARY`, subtask 13; logging, Phase 73; SET SCOPE, Phase 89; TYPE and EXAMINE/SOURCE,
Phase 85); Phase 62's Future work (SET WORKING_SET's quota and extent, Phase 63; quotas from an
authorization file, Phase 95); Phase 72's left items (FOR's messages, Phase 80; DBG$INPUT and
the trace trap, Phase 73; keyed `$PUT`, Phase 81); Phase 78's list; Phase 80's, 81's, and 82's
items for named later phases; Phase 87's rooted directories and SYS$SYSDEVICE (Phase 91);
Phase 88's DIRECTORY/TOTAL (Phase 91); Phase 93's and 94's identities (Phase 95); Phase 95's
batch jobs (Phase 98); Phase 99's to 110's items for named later phases; DEVIATIONS.md's Phase
42 ACCVIO text (Phase 55) and SHOW SCOPE's numbering (Phase 89); and its "[Phase 42] The
console's `SET name=value`" entry, whose one point Phase 50 resolved, taken out of
DEVIATIONS.md (DEVIATIONS-RESOLVED.md already had a copy, now marked resolved).

## A.14 What the survey proposed

For the author's review; question 1 decided it ("The author's answers" below, and Part B,
which replaces this list). As the survey wrote it: In the order the work would go:

1. **Fortran 95** (A.4, all of it), with A.3.2's module text for default initialization, A.1.11's
   recovery, and A.3.5's zero-sized arrays under `/F90`. The bulk of the phase.
2. **Fortran 90's cheap NOTIMPLs** (A.3.1's six marked 112) and the noticed items (A.3.4).
3. **The compiler's small things**: A.1.4's two forms, A.1.5, A.1.10.
4. **The tools**: MACRO's positional library, an object with errors, every undefined symbol
   (A.6.1, 2, 5); LINK's options logical name (A.7.1); `TestFTN86Objects`' symbol tables
   (A.7.3); the debugger's FORTRAN lines and components after subscripts (A.8.2, 3).
5. **The libraries**: `$GETCHN`, `$GETDEV`, the UTC services (A.2.3); the 80,000 parses
   (A.2.5); STR$'s arithmetic if wanted (A.2.1).
6. **Small things**: A.11.1, COPY's message (A.11.2), the duplicated deviations (A.11.4); the
   SHARE$LIBRTL fault (A.9.2) if wanted.
7. **The probe round** (A.12), then its fixes (the ods2 header among them), and the close-out.

Left to **own phases**: `CONVERT=` (A.1.1, if wanted), real debugger records (A.8.1, if
wanted), Phases 60 and 61 (A.2.2), ANALYZE/DISK_STRUCTURE/REPAIR (A.9.1), Phase 42's Part B
(A.8.5).

## A.15 The survey refreshed after Phase 112 (2026-11-23)

Part A was written before Phase 112 was done. Before this phase starts, each item Part B
takes was checked again against the code as Phase 112 left it, and Phase 112's "Where things
stand" was added to the list.

**Still open, each checked 2026-11-23.** Nothing in Part B was done by Phase 112:

- A.3.1: all six of this phase's NOTIMPLs are still reported (`sema/transform.go`'s DIM= and
  ORDER=, `charfuncs.go`'s array BACK=, `complex32Intrinsic` at its four sites, `decl.go`'s
  CHARACTER length zero and statement function of a derived type, `transfer.go`'s MOLD). The
  compiler now has 48 `notImplForm` calls; Phase 112's FORALL added 8 (`forallorder.go`).
- A.3.4: all four, by compiling small programs: `K = F(2)%N` is NOTRECORD ("a procedure with
  an interface body"), `REAL Y(DD(1)%N)` is BADBOUND, TRANSFER to a derived type in a PARAMETER
  is NOTCONST, and `lowermodel.go` still loads EXPONENT's and FRACTION's argument with MOVF.
- A.1.5: `REAL SQRT` still gets `T1.SQRT: .BLKL 1` and a storage map line. A.1.10:
  `SQRT(X)*2.0 + 0.5` still reads `T1.$KF1` and `T1.$KF2` from `$PDATA`.
- A.3.3: `USE STARLET_SSDEF` is MODNOTFOUND.
- A.6.2: `dclcmd/macro.go` still writes no object after an error. A.2.1: STR$'s arithmetic
  still NOTIMPL (`librtl/str.go`). A.2.3: of `$GETCHN`, `$GETDEV`, and the UTC five, only
  `$GETCHN` is mentioned, in `missingservice.go`'s signal; none has a routine.
- A.11.2: COPY prints `%COPY-S-COPIED` unless `/NOLOG`. A.11.4: 30 entries' titles are in
  both DEVIATIONS.md and DEVIATIONS-RESOLVED.md (the 29, and the `SET name=value` copy marked
  resolved). A.11.5: SHOW EXPAND is still in the grammar (`show_expand`, no handler).
- No commit since the survey touched MACRO, LINK, the debugger, LIBRTL, COPY, or the services,
  so A.6.1, A.6.5, A.7.1, A.7.3, A.8.2, A.8.3, A.9.2, and A.11.1 stand as written.

**Found by the refresh:**

1. **LIB$INITIALIZE already has a mechanism, and it isn't VMS's.** RUN's driver
   (`machine/run.go`'s `BuildImageInitDriver`, RUN `/[NO]INIT`, SET DEBUG LIBINIT, on by
   default) calls each shareable dependency's `SHARE$name_INITIALIZE` symbol, if one is
   defined, with three arguments (0, 0, and a flag that is 100 for LIBRTL alone, "apparently").
   That is the C emulator's convention, carried over. VMS's is the main image's own: a program
   puts the addresses of its routines in a `LIB$INITIALIZE` psect, and LINK pulls in STARLET's
   LIB$INITIALIZE module, which calls each. A.2.2's work in subtask 6 therefore begins by deciding
   what becomes of the old mechanism. It may also bear on A.9.2: real LIBRTL faulting at
   `SHARE$LIBRTL+0CE1` may be its initialization routine called with the guessed arguments.
   Subtask 10 looks at that first.
2. **SHOW ASSEMBLER_FLAGS** is in the grammar with no handler as well (`dispatch_test.go`'s
   `TestDispatch_unboundShowSubformErrors` expects the error). It joins A.11.5's list for subtask
   11, along with any others that subtask's own inventory finds. *(Done in subtask 11: it was
   already gone; the inventory found SHOW EXPAND, an unreachable SHOW INSTRUCTIONS syntax, and
   INITIALIZE and START alone.)*

**What Phase 112 passed on** (its "Where things stand"):

3. **The VMS round of Phase 112's programs**: `f90rAwaitingVMS` (ZEROSIZE, NULLS, DEFAULTS,
   PURES, ELEMENTALS, FORALLS, FORALLC, WHERES, INTRINS, ZEROWID, ZEROWIDVMS, M_COLONY) and
   `ftn86`'s ZEROVAX. FORRTL's answer for a zero width goes with them, if anything wants it.
   **Probe**: added to A.12 (subtask 12).
4. **A STRUCTURE with initial values in a module's text** (A.3.2's remaining part;
   `modwrite.go` still refuses it). Phase 112 names it as Phase 113's, but Part B doesn't list
   it. **Proposed: 113**, in subtask 2 with the Fortran 90 items: a derived type's default values
   are written already, so the module text has a form for them to follow.
5. **FORALL's remaining NOTIMPLs**: a statement through temporaries that is a pointer
   assignment, a defined assignment, a WHERE, or an assignment to an array or to a CHARACTER
   target of run-time length; a statement through temporaries, or a mask its statements change,
   in a FORALL whose bounds use an outer index; a target whose subscripts read its own storage
   (`V(V(I)) = 0`); a nested FORALL's bound that its own statements may change; a FORALL whose
   pointer's storage isn't known; an elemental defined assignment of a CHARACTER array of
   run-time length overlapping its target. **Not planned** (the refresh's proposal): each is an
   unusual program and named by its message, and Phase 112 judged them past its scope. Question
   A below.
6. **PURE's and ELEMENTAL's unchecked rules**: storage association with a global (an
   EQUIVALENCE partner), a pointer component's target through a global or an INTENT(IN) dummy, a
   constructor's pointer component given a global, an internal procedure of a PURE one being PURE,
   and ELEMENTAL's 12.7.1 rule for dummies in specification expressions; and
   `ASSOCIATED(P, NULL())` being NULLPLACE. **Proposed: 113** for the PURE checks and 12.7.1 (each
   a rule the standard states, small, in `sema/pure.go` and `elemental.go`), in subtask 2;
   NULLPLACE **recorded**. Question B below.
7. Phase 112's unconfirmed choices (an empty dimension shown as (1:0), recursive specification
   functions, DATA of a default-initialized object, CPU_TIME's -1, `I0` of zero, `F0.d`'s sign, a
   nested FORALL's kept bounds) are **recorded** in its doc. `I0` and `F0.d` go to the probe
   round with item 3, if FORRTL takes zero widths.

**Questions for the author:**

- **A.** Phase 112's FORALL NOTIMPLs (item 5): leave them named and not planned, or take any
  into this phase?
- **B.** PURE's and ELEMENTAL's unchecked rules (item 6): into subtask 2 as proposed, or
  left?
- **C.** The old `SHARE$name_INITIALIZE` call (item 1): replace it with VMS's
  `LIB$INITIALIZE` psect (the refresh's proposal, with RUN `/[NO]INIT` kept for the new
  mechanism), or keep both?

**The author's answers (2026-11-23):**

- **A. Taken in.** Phase 112's FORALL NOTIMPLs (item 5) are this phase's, to wrap up as much
  as can be: a subtask of their own (3, below). Each is either compiled or, where the standard
  makes the program nonconforming, refused with a message that says so rather than NOTIMPL.
- **B. As proposed.** PURE's storage association, pointer components' targets, constructors'
  pointer components, an internal procedure of a PURE one, and ELEMENTAL's 12.7.1 go into
  subtask 2; `ASSOCIATED(P, NULL())`'s NULLPLACE stays recorded.
- **C. Replaced.** The `SHARE$name_INITIALIZE` call was the C emulator's way around a real
  DECCRTL's startup, and isn't needed. It goes, and SET DEBUG LIBINIT with it. VMS's
  `LIB$INITIALIZE` psect takes its place (subtask 6), and RUN `/[NO]INIT` controls the new
  mechanism, on by default, so `/NOINIT` remains for the rare case, such as debugging a new
  run-time library. One thing subtask 6's survey must settle (a probe case if no manual does): VMS runs
  the initialization routines a *shareable* image lists in its own `LIB$INITIALIZE` psect too,
  as DECC$SHR's are, or only the main image's. If shareables' are run, the new mechanism does
  what the old hack was for, the VMS way.

## A.16 The other NOTIMPLs, weighed (2026-11-23)

After answer A the author asked what the rest of the compiler's NOTIMPLs would weigh, wanting the
list cleared but the phases kept alike in size. Every remaining site was read (2026-11-23), beyond
those A.3.1 already named:

- A section, or a whole assumed-shape array, to a system subroutine or a built-in
  (`arguments.go`): small to medium.
- CHARACTER array expressions of run-time length not made of names (three sites), and function
  values of run-time length where still refused (`charlength.go`, `decl.go`'s selector): medium, one
  or two subtasks.
- A name an internal procedure's declarations use that its host uses only in executable
  statements (`internal.go`): medium.
- A CHARACTER pointer of run-time length (`pointer.go`): small to medium.
- The module file's gaps (`modwrite.go`'s `fail`, ENTRY in a module procedure): medium to large,
  about two subtasks.
- DATA naming an array section (`data.go`, `datatypes.go`): small; the survey missed it.
- A NAMELIST object past FTNRTL's 8 levels or 15 dimensions (`derivedio.go`): small.
- **Dead hooks**: the attribute table's `attrNotYet`, the kind table's `pending`, the file
  specifiers' `later`, and `io.go`'s default case have no entries left that reach them. *(Done in subtask 11.)*

Together, seven to nine subtasks, which would take this phase past twenty. **The author's choice:
Phase 115** (`PHASE-115 - Fortran 90 NOTIMPLs.md`) takes them all but the dead hooks, which this
phase's housekeeping removes (subtask 11).

## Open questions

Note: any ANSWER that says "this phase" likely means the new PHASE 113 given the answer
to the first question, which splits _this_ phase into A.14's list (and probably should
be renamed to summarize it's work as "Fortran 95 extensions"), and moves everything
else to the _next_ PHASE 113, which really is the "Errata" phase.

1. **Scope**: A.14's list as this phase, or a part of it (Fortran 95 alone first, the rest a
   Phase 113)? ANSWER: Let's separate the Fortran 95 as the first phase, and the rest moved
   to a (new) PHASE 113. That will keep this document from growing unwieldy as we work though
   what will doubtless be a _lot_ of subtasks.
2. **Sources for Fortran 95**: the HP Fortran manual describes all of group D (FORALL, PURE,
   ELEMENTAL, NULL, default initialization, CPU_TIME, the deleted features), and gfortran
   `-std=f95` is the witness, as for Fortran 90. Enough, or should a Fortran 95 text be found? 
   (The survey's view: enough; where the HP manual adds DEC's extensions, the standard's
   form is what gfortran `-std=f95` takes.)
   ANSWER: I am okay with using the HP Fortran manual with gfortran as verification
   and/or backup. 
3. **A simh round** (A.12): yes, once, after implementation? ANSWER: Yes, let's do a
   probe round.
4. **Phases 60 and 61**: left as planned phases (the survey's view: no FORTRAN program needs
   them), or LIB$INITIALIZE folded in here, since Phase 92 found a main image's
   initialization routines never run? ANSWER: for completness let's fold in LIB$INITIALIZE.
5. **`CONVERT=`**: a phase of its own, or not planned? ANSWER: A phase of it's own.
6. **Real FORTRAN debugger records** (level C): a phase of its own, or not planned now that
   the `/DEBUG` table covers names, types, sections, pointers, and lines? ANSWER: let's 
   plan on not doing it; we don't have a good source for actual FORTRAN debug records and
   risk deviating from code that could run on a real VMS system if we add our own debug
   record types.
7. **STR$'s arithmetic** (A.2.1): in this phase, with a probe, or left? ANSWER: let do
   it in this phase.
8. **Spellings and modes**: zero-sized explicit-shape arrays under `/F90` only (A.3.5)? And
   the definition modules to USE (A.3.3): `USE SSDEF`, `USE STARLET_SSDEF`, or another name,
   since `$` can't start a module's name? ANSWER: yes on zero-sized arrays. As for definition
   modlues, lets use "STARLET_" as a prefix to make it unlikely to have a collision with
   anything a user program would want/use for another purpose.
9. **`/STANDARD=F95`** (A.4.8): does a `/F90` unit's default judgment become F95 (Fortran 95
   is a superset but for the deleted features), or stay F90 with F95 asked for?
   ANSWER: let's let /F90 expand to encompass the F95 set we're implementing.
10. **Housekeeping outside Fortran**: the grammar leftovers and the duplicated deviations (A.11.4,
    A.11.5) in this phase, or left? ANSWER: lets do them in this phase.

## The author's answers (2026-11-22)

Written into the questions above; what each decides:

1. **Split.** Fortran 95 is Phase 112 (A.14's item 1: A.4, with A.1.11, A.3.2's
   structures with initial values in a module's text, and A.3.5), so this doc doesn't grow
   past use; everything else proposed is this phase, Phase 113. The author left the split's
   lines to the survey's judgment.
2. **Sources for Fortran 95**: the HP Fortran manual, gfortran `-std=f95` the witness and
   backup (Phase 112's).
3. **A probe round**: yes, once, here, after Phases 112 and 113's implementation (A.12),
   then its fixes.
4. **LIB$INITIALIZE** is folded in here (Phase 61 keeps the rest of its list, and Phase 60
   its trees).
5. **`CONVERT=`** is a phase of its own: Phase 114 (`PHASE-114 - FORTRAN CONVERT.md`).
6. **Real FORTRAN debugger records: not planned.** No source describes VAX FORTRAN's own
   records, and records of govax's invention would make objects VMS's debugger may not
   read. The `/DEBUG` table stays the way (A.8.1).
7. **STR$'s arithmetic** is done here, with probe cases (A.2.1).
8. **Zero-sized explicit-shape arrays** under `/F90` only (Phase 112). **Definition modules
   to USE** are `STARLET_` and the module's name without its `$`: `USE STARLET_SSDEF`
   (A.3.3, here).
9. **`/STANDARD`**: no F95 keyword. `/F90` takes in the Fortran 95 that Phase 112 adds, and
   `/STANDARD=F90` judges by that widened language (Phase 112).
10. **Housekeeping**: the duplicated deviations and the grammar's verbs with no handlers are
    done here (A.11.4, A.11.5).

# Part B — what Phase 113 does

Part A's places, as the answers settle them. Everything marked **proposed: 112** in Part A
but for A.4, A.1.11, A.3.2's module text, and A.3.5 is this phase's; A.1.1 is Phase 114's;
A.8.1 is not planned.

- **Fortran 90**: A.3.1's six NOTIMPLs marked 112 (DIM= and ORDER= not constant, an array
  BACK=, COMPLEX(16) to SUM and its kin, CHARACTER length zero in a component or function
  value, a statement function of a derived type, TRANSFER to a CHARACTER MOLD of run-time
  length); A.3.3, `USE STARLET_SSDEF` and the rest; A.3.4's noticed items. Since the
  refresh (A.15): a STRUCTURE with initial values in a module's text, PURE's and ELEMENTAL's
  unchecked rules, and Phase 112's FORALL NOTIMPLs (answers A and B).
- **The compiler**: A.1.4's two forms for `/STANDARD`, A.1.5 (a typed intrinsic's cell),
  A.1.10 (floating short literals).
- **The libraries**: STR$'s arithmetic (A.2.1); LIB$INITIALIZE (A.2.2): LINK building the
  image's initialization list and the activator calling it, as VMS does, with probe cases,
  in place of RUN's `SHARE$name_INITIALIZE` call (answer C);
  `$GETCHN`, `$GETDEV`, and the UTC services (A.2.3); `astrace`'s 80,000 format parses
  (A.2.5).
- **The tools**: MACRO's positional library, an object with errors, every undefined symbol
  (A.6.1, 2, 5); LINK's options file by logical name (A.7.1); `TestFTN86Objects`' symbol
  tables (A.7.3); the debugger's FORTRAN lines and a component after subscripts (A.8.2, 3).
- **The SHARE$LIBRTL fault** (A.9.2): found and fixed if it is govax's, else recorded.
- **Housekeeping**: A.11.1 (a full volume's message, Ctrl/Y's continuation), COPY's message
  (A.11.2), the 29 duplicated deviations (A.11.4), the grammar's verbs with no handlers
  (A.11.5).
- **The probe round** (A.12, with Phase 112's questions a VMS run can answer, if any) and
  its fixes, the ods2 index file extension header among them (A.9.1).

## Subtasks (provisional)

1. **The survey** (done 2026-11-22): Part A; the author's answers the same day. Refreshed
   after Phase 112 (done 2026-11-23): A.15.
2. Fortran 90's NOTIMPLs and noticed items (A.3.1, A.3.4), a STRUCTURE with initial values in
   a module's text (A.15.4), and PURE's and ELEMENTAL's unchecked rules (A.15.6, answer B)
   (done 2026-11-23).
3. FORALL's remaining NOTIMPLs (A.15.5, answer A) (done 2026-11-23).
4. The compiler's small things and `STARLET_` modules (A.1.4, A.1.5, A.1.10, A.3.3) (done 2026-11-23).
5. MACRO (A.6.1, 2, 5) (done 2026-11-23).
6. LINK and LIB$INITIALIZE (A.7.1, A.7.3, A.2.2; answer C: the old call removed) (done 2026-11-24).
7. STR$'s arithmetic (A.2.1) (done 2026-11-24).
8. Services and FTNRTL's format parses (A.2.3, A.2.5) (done 2026-11-24).
9. The debugger (A.8.2, 3) (done 2026-11-24).
10. The SHARE$LIBRTL fault (A.9.2), after subtask 6 (done 2026-11-24).
11. Housekeeping (A.11.1, 2, 4, 5; A.15.2; A.16's dead hooks) (done 2026-11-24).
12. The probe round (A.12, A.15.3): written, rehearsed on govax, run on VMS 7.1. In four parts
    (2026-11-24), so each is a subtask's size:
    - 12a. The round's own volume, `testdata/err113`: STR113 (with STR$MATCH_WILD's `%`), SVC113,
      LIB$INITIALIZE, MACRO's questions (A.6.3, 6, 7), DCL's messages from subtask 11, TYPE
      without carriage control, a full volume, and the index file past its header (A.9.1);
      RECURAST's longer timer (A.9.3) (done 2026-11-24).
    - 12b. The FORRTL round (A.10's FORRTL items, Fortran 95's zero widths on output) (done 2026-11-24).
    - 12c. The debugger round (A.8.6) (done 2026-11-24).
    - 12d. The session on VMS 7.1: `testdata/err113`, `testdata/ftn86` (round 6), and
      `testdata/f90r` (round 3), with 12b's and 12c's; the results copied back (done 2026-11-24).
13. Its fixes, the ods2 header among them. In parts (2026-11-24), each a subtask's size:
    - 13a. STR$'s arithmetic to VMS's run (`testdata/str113`, `str113Compared`) (done 2026-11-24).
    - 13b. The services to VMS's run (`testdata/svc113`, `svc113Compared`) (done 2026-11-24).
    - 13c. ERR113's sections: LIB$INITIALIZE's arguments, MACRO's register names and
      `.WARN`/`.ERROR`/`.PRINT`, DCL's messages, TYPE without carriage control, a full volume;
      `TestErr113Procedure` held to `vax/err113.log` (done 2026-11-24).
    - 13d. The index file's growth (A.9.1): ods2 extending INDEXF.SYS as VMS's XQP does (done 2026-11-24).
    - 13e. FTNRTL to the FORRTL round (`testdata/for81`'s round 3 and the terminal's limits) (done 2026-11-24).
    - 13e2. Round 3b, what 13e left open (`testdata/for81`'s FOR81K and FOR81W): written and
      rehearsed, run on VMS 7.1, and govax and FTNRTL made its answers (2026-11-24); FOR81W's
      terminal lines to be run again.
    - 13f. The debugger to the debugger round (`testdata/dbg113`'s sessions) (done 2026-11-25).
    - 13g. The FORTRAN rounds: `TestFTN86Oracle` on `testdata/ftn86`'s round 6 and
      `TestF90ROracle` on `testdata/f90r`'s round 3, their awaiting lists emptied (done 2026-11-25).
14. The close-out. In three parts (2026-11-25), since the author offered another VAX run:
    - 14a. Round 2 of the probe round (`testdata/err113`'s README, "Round 2"): what 12 and 13 left
      unconfirmed, written and rehearsed, with FOR81W's terminal lines again and FTN86's round 7
      and F90R's round 4 (done 2026-11-25).
    - 14b. Round 2's answers, after the author's run (2026-11-25), in parts: 14b1, DCL's file
      parameters, MACRO's registers in data, FOR81M, and FOR81W (done 2026-11-25); 14b2, LINK's
      NONPICREF and the DST's blocks (done 2026-11-25); 14b3, RMS's extension size and HEADERFULL (ods2).
    - 14c. The documents: MANUAL.md, FORTRAN-INTERNALS.md, HELP, PLAN.md, "Where things stand".

Each subtask may begin with a survey of its own, as earlier phases' did.

## Progress log

- **2026-11-22:** Planned at Phase 111's close-out: first as Fortran 95's
  additions, the author's go; then, by the author's direction, as the errata
  of both Fortran programs and their tools, with Part A's seed list gathered
  from the phase docs, DEVIATIONS.md, and MANUAL.md. Resolved items found
  while gathering it marked in their own docs: Phases 81, 82, 83, 84, 85, and
  86 (indexed files, Phase 88; reentrant routines, SET SCOPE, and which call a
  dummy means, Phase 89; LIBRARY in place, Phase 90; `$QUIDEF` and
  `$SJCDEF`, Phase 98; a record's fields in the debugger, Phase 110; `/CHECK`
  through the optimizer, Phase 85; real MACRO on `/OPTIMIZE`'s output, Phase 86), and DEVIATIONS.md's Phase 27 (`MACRO A+B`,
  Phase 86) and Phase 28 (LIBRARY, Phase 90) entries. Subtask 1, the survey,
  is next.
- **2026-11-22, subtask 1 (done):** the survey. Every phase doc's follow-up sections from 27 on,
  DEVIATIONS.md, MANUAL.md's 14.7 and 14.13, the compiler's 53 NOTIMPL and F95ONLY sites,
  FTNRTL's one NOTIMP, the service vector (179 of 310 without a routine, counted by a
  throwaway test over `vmsdef.P1VectorTable` and `registerServices`), and the oracles'
  difference lists; a few items checked by compiling small programs (a typed intrinsic's
  cell, zero-sized arrays, Fortran 95's forms, `*1.0` and `SNGL(DBLE(...))`). Part A is the
  seed list made whole: eleven items new (A.1.9 to 11, A.2.9 and 10, A.3.5 and 6, A.4.9,
  A.9.5 and 6, A.11.4), each item given its place, A.12 the probe round's questions, A.14
  the proposal, and ten open questions. Resolved items marked where they were written
  (A.13), and DEVIATIONS.md's `SET name=value` entry taken out (its copy in DEVIATIONS-RESOLVED.md
  marked); 29 other entries found copied into DEVIATIONS-RESOLVED.md though open (A.11.4).
  Nothing implemented: the review of A.14 and the questions comes first.
- **2026-11-22, the review:** the author's answers in "Open questions", summarized in "The
  author's answers": the phase split, Fortran 95 becoming Phase 112 (`PHASE-112 - Fortran
  95.md`, new) and this doc, renamed from `PHASE-112 - Fortran errata.md`, Phase 113;
  `CONVERT=` Phase 114 (`PHASE-114 - FORTRAN CONVERT.md`, new); real debugger records not
  planned; LIB$INITIALIZE, STR$'s arithmetic, `STARLET_` modules, and the housekeeping
  here. Part B is the phase's scope and its subtasks. References to "Phase 112's A.n" in
  Phases 78 to 107's docs now say Phase 113's.
- **2026-11-23, subtask 1 refreshed (done):** Phase 112 is done, so before work begins every
  item Part B takes was checked again against the code (A.15): all still open, the compiler's
  by compiling small programs. Phase 112's "Where things stand" added: its programs for the
  probe round, a STRUCTURE's initial values in a module's text (proposed for subtask 2), its
  FORALL NOTIMPLs (proposed not planned), and PURE's and ELEMENTAL's unchecked rules (proposed
  for subtask 2). Two findings: RUN's driver already calls a dependency's
  `SHARE$name_INITIALIZE`, the C emulator's convention and not VMS's `LIB$INITIALIZE` psect,
  which subtask 5 must settle and which may explain A.9.2's fault; and SHOW ASSEMBLER_FLAGS, a
  second grammar entry with no handler. Three questions for the author (A.15's A to C).
  Subtask 2 is next, after the answers.
- **2026-11-23, the refresh's review:** the author's answers (A.15): Phase 112's FORALL
  NOTIMPLs taken in, as a new subtask 3; PURE's and ELEMENTAL's rules into subtask 2; RUN's
  `SHARE$name_INITIALIZE` call, the C emulator's DECCRTL workaround, replaced by VMS's
  `LIB$INITIALIZE` psect under RUN `/[NO]INIT` (subtask 6). The subtasks after 2 renumbered.
  Subtask 2 is next.
- **2026-11-23, A.16:** the compiler's other remaining NOTIMPLs weighed at the author's request:
  seven to nine subtasks together. By the author's choice they are Phase 115
  (`PHASE-115 - Fortran 90 NOTIMPLs.md`, new), but for the dead hooks, which subtask 11 removes.
  Subtask 2 is next.
- **2026-11-23, subtask 2 (done):** Fortran 90's NOTIMPLs and noticed items, a STRUCTURE's
  initial values in a module's text, and PURE's and ELEMENTAL's rules.
  - *A DIM= not constant* of SPREAD, CSHIFT, and EOSHIFT (`sema/transform.go`'s `dimArg`; the
    `ArrayFunc`'s `DimExpr`, `Dim` 0, but 1 when the rank allows nothing else) is lowered as a
    reduction's is: a branch on DIM's value to each constant DIM's code (`ir/lowertransform.go`'s
    `fillByDim`), SPREAD's extents chosen when the program runs (`spreadExtents`), DIM out of
    range FOR's NOTCONFOR under `/CHECK=BOUNDS` (`keepDim`); a SHIFT or BOUNDARY array then has
    its rank checked, not its extents. *RESHAPE's ORDER= not constant* is `ArrayFunc.OrderExpr`
    (`orderValues`, its size SHAPE's if known): the walk's extents and each result subscript's loop
    are chosen from ORDER's values when the program runs (`reshapeInto`, `orderValues`).
  - *An array BACK=* is an elemental argument like the others (the refusal was all there was).
  - *COMPLEX(16) to SUM, PRODUCT, DOT_PRODUCT, and MATMUL*: the reductions' code was already
    general; the NOTIMPLs went, and with them `complex32Intrinsic`. The intrinsic reference's site
    caught only specific names of other COMPLEX types (CSIN, DREAL), now BADARG as any other
    wrong type: every generic that takes a COMPLEX has COMPLEX(16)'s row.
  - *CHARACTER of length zero*: a FUNCTION statement's (`functionType`, kept as a type
    statement's, `runtimeLength`), IMPLICIT's (`implicitRules.zero`, given each name `newName`
    makes; a type statement for the name takes it away), and a derived type's component
    (`Field.Empty`: CHARACTER of length 0, no storage). A reference to such a component is an
    empty substring, (1:0), of its place taken as CHARACTER\*1 (`record.go`'s `emptyComponent`,
    `derivedio.go`'s `fieldOf` for I/O), so nothing is ever read or written there and every use of
    a value takes it; a substring of it is `emptyComponentSubstring`'s; the module text writes
    `CHARACTER(LEN=0)`, the map `CHARACTER*0`. A pointer component of length zero stays NOTIMPL.
    Found on the way and fixed: a passed-length dummy's `C(1:0)` was SUBLOW under `/F90`, not
    empty (`passedSubstring`).
  - *A statement function of a derived type*: its dummies take the type's structure
    (`stmtFuncDummy`), its value and actuals are aggregates (`aggregateValue`), and a reference
    is built or moved into a temporary of the type's size (`ir/lowerstmtfunc.go`'s
    `stmtFuncRecord`; an actual not a variable of the type copied, `actualPlace`).
  - *TRANSFER to a CHARACTER MOLD of zero or run-time length*: the scalar keeps MOLD as its
    second argument and is a dynamic string of MOLD's length (`transferText`); the array keeps
    MOLD as `Pad`, its elements' length read from it (`moldLength`: a name, a section, a
    substring, or a constructor's first value; another expression NOTIMPL, Phase 115's), its
    temporary of run-time length. With no SIZE, a MOLD of elements of length zero is BADARG
    when the compiler can see it (`knownEmpty`) and no elements when it can't (ours,
    unconfirmed). `valueLength` takes a substring (`substringLength`).
  - *A.3.4*: `F(2)%N` is the new **FUNCPART** ("F(...) is a function's value, which has no field
    N to select: assign the value to a variable first"; `functionName`); a component of an
    element, `Y(D(1)%N)`, is a bound (`boundComponent` takes `ast.Apply`, its subscripts bound
    expressions); TRANSFER to a derived type folds (`foldable`; `bytesConst`, now a function the
    module writer shares, makes a record's constant with `Parts` from its bytes,
    `componentFromBytes`), but not a type with a pointer component or one of length zero; and
    EXPONENT and FRACTION move a variable, element, or field into their temporary as an integer
    of its size (`lowermodel.go`'s `modelPlace`, TRANSFER's `storeSource`), so a reserved operand
    no longer faults there (another expression's value is still moved as REAL).
  - *A STRUCTURE with initial values in a module's text*: each field's values after its name,
    VAX FORTRAN's `/v1, v2/`, each the constant its bytes hold (`modwrite.go`'s
    `fieldInitials`); a value whose bytes don't give it back (a reserved operand's) still fails.
  - *PURE* (with the HP manual's 8.5.1.2): a global, an INTENT(IN) dummy, or any dummy of a PURE
    function (`restrictedWhy`) is not given a value through a pointer component, nor a PURE
    function's pointer dummy through itself (`pureThroughPointer`); is not a pointer's target,
    in a pointer assignment (INTENT(IN) dummies too now) or as a structure constructor's pointer
    component; and is not assigned whole to a variable of a type with a pointer component
    (`pureAggregate`). An internal procedure of a PURE one must be PURE (`pureInternals`). An
    EQUIVALENCE partner of a variable in a common block was already in the block, a global:
    tested, nothing to add. NULLPLACE stays as it was (answer B). *ELEMENTAL's 12.7.1*: a dummy
    in a local's bound or CHARACTER length other than as an inquiry's argument is the new
    **ELEMBOUND** (`elementalBounds`, over `adjustableArray.inquired`).
  - Tests: `sema`'s `TestTransform`, `TestComplex16Reductions` (was `TestComplex16NotImpl`),
    `TestPure`'s two new cases, `TestElementalProcedures`' 12.7.1 case, `TestBoundFromComponent`,
    `TestPhase108ZeroLengths`, `TestPhase108Transfer`, `TestDerivedArguments`; `fortran`'s
    `TestErrataModuleText` (`errata_test.go`); and `testdata/fortran90`'s `rundims.f90`,
    `errata.f90`, and `zerolen.f90` (each the same on gfortran, `compare.py`), `modelraw.f90`
    (VMS's own), and `modules/stamps` (the STRUCTURE and the type with a component of length zero
    through a module file), the new programs added to `testdata/f90r`'s procedure (`gen.go`) and to
    `f90rAwaitingVMS` for subtask 12's round. MANUAL.md's 14.13 (length zero, PURE's and ELEMENTAL's rules, the
    statement function, and "What Fortran 90 isn't here" six rows shorter) and
    FORTRAN-INTERNALS.md's section 33 follow; the items are marked resolved in Phases 104 to 108,
    111, and 112. Subtask 3 is next.
- **2026-11-23, subtask 3 (done):** FORALL's remaining NOTIMPLs, all but one compiled.
  - *Recorded stores* (`ir/lowerrecord.go`; sema's `Forall.Recorded`, the dump's "[stores
    recorded]"): a step that can't go through temporaries of the index set's shape is lowered three
    times over its own walk, each store to the program's storage intercepted (`recordsStore`, in
    `assignScalar`, `moveRecord`, `call`, `pointerAssign`, and `nullify`): sized, recorded in a buffer
    of the statement's (each entry the target's address, its length, the value's length, the value),
    and played back by a counted loop (`playRecords`). It takes a target whose subscripts read its
    own storage (`V(V(I)) = 0`), a target reached through a pointer, an array target, a CHARACTER
    target of run-time length, a triangle of tuples, pointer assignments (scalar, CHARACTER, and
    array pointer components; the storage's bytes), defined assignments (the subroutine called at
    playback, `slotSymbol`), and a WHERE, a pass per assignment (`sema.WhereAssignments`,
    `WhereOnly`). The one form left NOTIMPL is a WHERE of more than one assignment whose assignments
    change what its masks read (`whereRecordable`): each pass tests the masks again. `forallStepOrder`
    keeps the old temporaries for a scalar assignment over a rectangle (`forallTemporaries`).
  - *Listed tuples* (`Forall.List`, "[tuples listed]"; `ir`'s `forallList` and `walkList`): a nested
    FORALL's bound its statements change, or a mask they change where the index set isn't a
    rectangle or is inside a listed one, lists the tuples as the FORALL is reached, masks tested, in
    an INTEGER temporary; the statements under it walk the list.
  - *An elemental defined assignment of CHARACTER values of run-time length overlapping its target*
    (`operators.go`): `ArrayOrder.Recorded`, its calls recorded by `maskedCall`.
  - *Found and fixed*: `forallVaries` asked `readSymbols`, which doesn't look inside an operation,
    so `FORALL (J = 1:I-1)` was a rectangle and its bounds were computed before I had a value (now
    `readsAny`); a mask's test in a FORALL's loops (`forallMasked`'s `stmtCond`) gave back every
    run-time temporary of the statement at the first tuple; an array pointer component's
    references all dump as `V()%P`, so `V(I+1)%P => V(I)%P` looked like the same element
    (`referenceKey` adds the component's symbol), and a pointer assignment's target that is a
    pointer now counts as reading its home's storage. And subtask 2's left-over: `errata.f90` and
    `zerolen.f90` draw Fortran 95's obsolescent notices, now in `TestStandardF90Witness`'s list.
  - Tests: `sema`'s `TestForallRecorded` (and `TestForallErrors`'/`TestForallConstructErrors`'
    old NOTIMPLs gone, the one left); `testdata/fortran90/forallrec.f90` (every case, in all eight
    modes; gfortran the same but for the pointer and defined assignments, which it does tuple by
    tuple, `compare.py`'s KNOWN), added to `testdata/f90r`'s procedure and `f90rAwaitingVMS`.
    MANUAL.md's 14.13 and its table, FORTRAN-INTERNALS.md's section 37 ("Recording the stores"),
    and Phase 112's list follow. Subtask 4 is next.
- **2026-11-23, subtask 4 (done):** the compiler's small things and the `STARLET_` modules.
  - *A.1.4, `/STANDARD`'s short spellings*: `FUNCTION F` with no parentheses (the tree keeps it: a
    nil `Params`, `()` an empty list), `&s` for an alternate return's `*s` (`ast.AltReturn.Ampersand`),
    and a bare `X` for `1X` (`format.Item.Bare`) are each an extension to both standards
    (`sema/standard.go`). SEMANTIC stays not planned. `TestStandardSyntaxFlagged`'s three cases,
    `TestStandardSpellingsQuiet`.
  - *A.1.5, a typed intrinsic's cell*: a name typed but otherwise unclassified that the program
    references as an intrinsic or system function (`REAL SQRT`, then `SQRT(X)`) is marked
    (`Symbol.calledIntrinsic`), and layout classes it Intrinsic: no `$LOCAL` cell, no map line.
  - *A.1.10, floating short literals*: a REAL constant a short literal holds (64 values, 0.5 to 120
    with a three-bit fraction) is `S^#v` in an operand the instruction reads as floating
    (`codegen`'s `floatingOperand`: ADD, SUB, MUL, DIV, MOV, MNEG, CMP, TST, ACB of F, D, G, H, and a
    conversion's floating source). `constant` names it with a placeholder unless a datum holds it
    already, and `emit` makes the placeholder the literal, or the `$PDATA` datum where an instruction
    reads it otherwise (`PUSHL` of `%VAL(1.5)`, MOVQ of a D argument by value), so `$KFn` are only the
    data written. `Code.Bound` counts `S^#` as one byte. The literal is exact in every format, so the
    bits are still the compiler's (Decision 16). Real MACRO hasn't yet assembled `S^#` in a D, G, or
    H instruction; subtask 12's round will, rebuilding the corpus. The change reaches nearly every old
    program's code, so `testdata/nof90.sum` is rewritten (`TestNoF90Unchanged`, as it asks for an
    intended change), as are `codegen`'s goldens and fcalc's image (`go generate
    ./internal/fcalc`). `TestFTN86Oracle`'s OPTFORWARDR now differs from VMS's log in `KEPT`, the
    value KEEP(.FALSE.) never assigned under `/RECURSIVE`, which the smaller `$PDATA` moved
    (`ftn86VMSDifferences`, beside OPTFLOWR).
  - *A.3.3, the definition modules to USE*: `USE STARLET_SSDEF` and the rest, every one of
    `sysdef`'s 50 (`dclcmd/fortranstarlet.go`): the module search's last place, after every
    directory, compiles the definition module wrapped in MODULE and END MODULE, once a session for
    each REAL*8 format, its text the compiler's own; no object, so nothing to LINK. On the way a gap
    in modules: a module's external procedure with only an EXTERNAL statement or attribute
    (`INTEGER, EXTERNAL :: F`, `$SYSSRVNAM`'s every service) was neither in its text nor reached by
    a USE (`modwrite.go`'s `externals`; `useEntity` and `symbolCopy`). `dclcmd`'s
    `TestStarletModules`, `TestUseStarlet`; `fortran`'s `TestModuleExternals`.
  - MANUAL.md's 14.13 (definition modules to USE, external procedures from modules, `/STANDARD`'s
    spellings, a row gone from "What Fortran 90 isn't here") and FORTRAN-INTERNALS.md (short
    literals, sections 9 and 11's listings) follow; the items are marked resolved in Phases 85, 86,
    and 106. Subtask 5 is next.
- **2026-11-23, subtask 5 (done):** MACRO.
  - *A.6.1, the positional `/LIBRARY`*: MACRO's `/LIBRARY` is a positional qualifier with an
    optional value (`console.dcl`): after a list element with no value the element is a macro
    library (`dclcmd.MacroSource.Library`), VMS's `PROG+LIB/LIBRARY`, joining `/LIBRARY`'s for
    its assembly in the order written (searched the last first, as before); with a value, after an
    element or after the verb, govax's older form names the libraries. A list of libraries alone is
    CLI-W-INSFPRM. Two changes to the grammar engine (`internal/dcl`) made it work: a positional
    qualifier after the verb is the whole command's, as DCL has it (`Entry.positionalQualifiers`),
    and in a joinable list a `+` ends a positional qualifier's name (`LIB/LIBRARY+PROG`).
    `TestDispatch_macroLibraryQualifier`'s new cases.
  - *A.6.2, an object for a module with errors*: govax's MACRO writes the object after errors,
    as VMS's does, its end record's severity 2 (`asm.Object`, `obj.SeverityError`); ASMERRORS
    no longer says "no object file written". To make the object whole, `finish` goes on past a
    fixup that fails (each one's error kept, a repeated value's once per statement, in statement
    order), and a directive may report an error and go on (`Assembler.continuingError`): `.ASCIZ`,
    `.ASCIC`, and `.ASCID` with nothing after them are DIRSYNX and assembled as the empty
    string, as VMS's are, so TIRX's object is VMS's record for record
    (`TestTIR111ErrorObjects`). TIRR's isn't: VMS took `R1` in `.LONG R1[2]` for a register and
    stored a longword of 0, govax an external symbol (A.6.3, the probe round's). LINK refuses a
    module whose severity is 2 or more as VMS 7.1's did in f90r's round 1 (`link`'s
    `compilationErrors`: ERRORS with the module and file, NOIMGFIL after the first, ERRORS for each
    later one; `TestLinkCompilationErrors`, on VMS's own TIRX and TIRR objects).
    `cmd/govax`'s `TestRun_failedOneShotEndsWithItsError` now expects the object.
  - *A.6.5, every undefined symbol*: with `.DISABLE GLOBAL`, each undeclared symbol is its own
    UNDEFSYM, sorted, where govax gave only the first. `.ENABLE TRUNCATION` and `VECTOR`, and
    the messages' form, stay as designed.
  - DEVIATIONS.md's entry for the object moves to DEVIATIONS-RESOLVED.md, and its Phase 27
    entry's positional `/LIBRARY` and first undefined symbol are marked resolved; MANUAL.md's
    MACRO table and examples, HELP MACRO, and Phases 86 and 111 follow. Subtask 6 is next.
- **2026-11-24, subtask 6 (done):** LINK and LIB$INITIALIZE.
  - *A.7.1, an options file by logical name*: since Phase 95 `Locate` translates a bare name that
    is a logical name, so LINK's first element worked; after a host file, `LocateRelated` took the
    name for a file beside that one before asking whether it was a logical name. It asks first now
    (`rms/location.go`). `TestLinkOptionsByLogicalName`.
  - *A.7.3, the image's symbol tables*: `TestFTN86Objects` compares FTNRTL's debug symbol table
    block for block and its global symbol table record for record with VMS's
    (`compareImageSymbolTables`; the MHD record without its two times, the LNM record, the linker's
    name, left out, and govax's padding past the last record, Phase 29's Decision 7). It found the
    DST fixup records split as VMS's LINK doesn't: VMS's first record has the whole list's count, the
    name, and 58 offsets, and each after it only offsets, 63 at most (176 as 58, 63, 55), where govax
    repeated the header in each (`link/shareable.go`'s `withDSTFixups`, which had called the split
    unconfirmed). FTNRTL's committed image is rebuilt (`go generate ./internal/ftnrtl`).
  - *A.2.2, LIB$INITIALIZE, and answer C*: the VMS 5.0 LIB$ manual's chapter 7 ("Image
    Initialization and Termination", which Phase 61's plan names) describes the mechanism whole, so
    it is followed as written: a program puts routine addresses in the program section
    LIB$INITIALIZE and refers to LIB$INITIALIZE; LINK puts that routine's address in the image's
    initialization list between SYS$IMGSTA's (or the debugger's) and the main program's
    (`link/libinit.go`'s `initializeTransfer`, when the section has a contribution of some length
    and the symbol is defined; the Internals book's Figure 21-8), a shareable image's too. The
    routine is govax's own module, `bootdata/files/libinit.mar`, written from chapter 7 (7.2's
    argument list and coding sequence, 7.4's call of each routine with `(init-coroutine,
    cli-coroutine [, image-info])`, 7.5's coroutine call, which runs the rest and the main program
    and returns its status), assembled once and offered to LINK after STARLET.OLB
    (`dclcmd/libinit.go`'s `libinitSource`); its empty sections LIB$INITIALIZD and LIB$INITIALIZE$
    bracket the list, LINK sorting sections by name within an image section (the Linker manual).
    RUN's driver (`machine/run.go`) calls each image's LIB$INITIALIZE, the shareable images' first,
    with `(start, 0, 0)`, `start` its entry in a copy of the image's list placed after the driver's
    code, the list itself pushed by CALLS since the routine steps `start`; the main image's
    LIB$INITIALIZE calls the main program, which the driver then doesn't. The main routine is now
    an image's last P0 transfer address (`mainTransferAddress`, `p0Transfers`, `initTransfers`). The
    old call of each shareable image's `SHARE$name_INITIALIZE` with guessed arguments is gone, as is
    SET DEBUG LIBINIT (`registers.DebugLibinit`, `vax.debug.libinit`); RUN `/[NO]INIT`, on by
    default, controls the new calls; a shareable image's transfer symbols are all
    `SHARE$name_TRANSFER_n`. Unconfirmed, for the probe round (subtask 12): whether VMS calls a
    shareable image's LIB$INITIALIZE (govax does), and what a CLI coroutine and image information
    VMS passes (govax passes 0 for each, and a routine calling the CLI coroutine would fault).
    `TestLibInitialize` (the order, a coroutine call, `/NOTRACEBACK`, `/NOINIT`),
    `TestLibInitializeShareable`, `machine`'s `TestMainTransferLegacy`.
  - MANUAL.md's RUN, HELP RUN /INIT, HELP SET DEBUG and CONFIGURATION, VAX-VMS-INTERNALS.md's driver
    paragraph, and Phases 61 and 83 follow. Subtask 7 is next.
- **2026-11-24, subtask 7 (done):** STR$'s arithmetic.
  - *The routines* (`librtl/strmath.go`), from the VMS 5.0 and OpenVMS 7.3 STR$ manuals: STR$ADD,
    STR$MUL, STR$DIVIDE, STR$RECIP, and STR$ROUND, each number a sign, an exponent, and a digit
    string, exact in `math/big` (a `decimal`). ADD aligns on the smaller exponent, MUL adds them,
    DIVIDE's quotient has total-digits places (its exponent -total-digits), rounded half away
    from zero when the indicator's bit 0 is set, ROUND keeps places significant digits, a carry
    past them dropping a zero into the exponent (the manual's 999.9998 to 3 is `100` times 10).
    Every manual example gives its printed answer (`TestStrMathManualExamples`). The digits go to
    the destination as every STR$ routine writes (STR$_TRU when cut), then the exponent and sign.
  - *Unconfirmed choices*, each a case of the probe: a result has no leading zeros and keeps its
    trailing ones; zero is `0`, exponent 0, sign 0; an empty digit string is zero; anything but
    digits, a sign other than 0 or 1, places below 1, or an exponent past a longword is
    LIB$_INVARG; a result past 65,535 digits STR$_STRTOOLON; too few arguments STR$_WRONUMARG;
    a zero divisor STR$_DIVBY_ZER. STR$RECIP's "precision limit specified by the second decimal
    string" is read as B's exponent alone: 1 / A truncated at that place, its trailing zeros
    moved into the exponent, which is what makes the manual's `1 -3 1` from 1 / -1000 at .0002's
    place (`trimZeros`).
  - *STR$ELEMENT_R8*, in the vector and in neither manual: R0 the destination, R1 the element
    number by value, R2 the delimiter, R3 the source, as STR$LEFT_R8 takes its position
    (`strElementR8`, STR$ELEMENT's code, `element`); a guess the probe tests both ways.
  - *The probe*, `testdata/str113` (`gen.go`, str83's framework, 106 cases; README), for subtask
    12's round. `TestStr113Probe` runs it on govax and holds the manuals' examples until VMS's
    output is in `vax/str113-vax.txt`, then the whole of it. `testdata/fortran/strmath.for` calls
    the routines from FORTRAN (in `TestNoF90Unchanged`'s baseline, and in `testdata/ftn86`,
    regenerated with STRMATH in `ftn86AwaitingVMS`).
  - MANUAL.md's FORTRAN strings paragraph (with an example) and Appendix B follow; Phase 83's
    items are marked resolved. Subtask 8 is next.
- **2026-11-24, subtask 8 (done):** services and FTNRTL's format parses.
  - *The format parses* (A.2.5). `astrace.for`'s formats are CHARACTER constants, `'(I6)'`,
    which sema made run-time formats: FTNRTL parsed each at every execution and freed it at the
    statement's end, some 4,000 parses for its 2,000 passes and the ASTs' statements. A
    FORMAT statement's block keeps its parse in a cell; now a constant that reads as a format is
    a `sema.Format` of its own, Label 0 (`compiledFormat`), whose block ir writes as a FORMAT's
    (`formatBlock`: `$FK1`, its cell `$FKC1`, one per text in a routine), so it is parsed once, as
    VAX FORTRAN's compiled constant formats were. One that doesn't read as a format stays a
    run-time format, FTNRTL's to refuse when it runs. Every compiled program with a constant
    format changes (`TestNoF90Unchanged`'s baseline, the golden dumps, fcalc's image rebuilt).
    `directives.for` prints a COMMON block's address, which moved 4 bytes: its `.out` follows,
    and DIRECTIVES is in `ftn86VMSDifferences` until the next VMS round. `allocvms.f90` counted a
    PRINT's first parse of its format among the bytes a recursion kept: the call moved out of the
    PRINT, its output VMS's as before.
  - *The UTC services* (A.2.3; `corevms/utc.go`), from VSI's manual: `$GETUTC`, `$ASCUTC`,
    `$BINUTC`, `$NUMUTC`, `$TIMCON`. The 128-bit form is no manual's we may use: govax's is the DCE
    time service's (ticks since 15-Oct-1582 UTC, a 48-bit inaccuracy, all ones infinite, a 12-bit
    TDF in minutes under a version of 1), unconfirmed. The local zone is
    SYS$TIMEZONE_DIFFERENTIAL's, rounded to minutes (`localTDF`); every time govax makes has an
    infinite inaccuracy. Unconfirmed choices, each a probe case: `$NUMUTC`'s 13 words (the local
    time's seven, the inaccuracy's five, `FFFF` each when infinite, the TDF last; VSI's text lost
    the figure); a version other than 1 or a time before 17-Nov-1858 SS$_IVTIME; `$TIMCON`'s other
    flags SS$_BADPARAM and a missing address SS$_ACCVIO. `$ASCUTC` and `$BINUTC` run in the
    caller's mode (the manual's words) and don't count their arguments.
  - *`$GETCHN` and `$GETDEV`* (`corevms/getchn.go`), VMS V1's device information services,
    which the 5.0 manual lists only as obsolete: the device by channel or name as `$GETDVI`
    finds it, then the device information buffer, primary and secondary (govax's devices have
    no associated mailbox, so both the device's). Its layout is govax's reconstruction (the
    file's comment: DEVCHAR to DEVDEPND2 in 44 bytes, reserved to 116, then the controller's
    name and the volume's label as counted strings at DEVNAMOFF and VOLNAMOFF), unconfirmed;
    a short buffer SS$_BUFFEROVF. No `$DIBDEF` in `vmsdef` or the macro library until the round
    gives its values (`testdata/svc113`'s `def_dib.mar`). `testdata/fortran/services.for`'s
    `SYS$GETCHN(,,,,)` now fails with SS$_IVCHAN, unsignaled, as VMS's run showed: SERVICES is
    out of `ftn86VMSDifferences`; the tests of a missing service name `SYS$GETLUI`.
  - The seven services' `_S`, list, and `_G` macros in `starlet.mar` (unchecked against real
    MACRO until a macro round runs them); FORTRAN's `$SYSSRVNAM` already had them from the
    vector. `corevms`'s `TestUTCServices`, `TestUTCOctaword`, `TestLocalTDF`,
    `TestGetchnGetdev`.
  - *The probe*, `testdata/svc113` (`gen.go`, str113's framework, 57 cases; README), for subtask
    12's round: octawords made for the VAX's TDF of -300 and shown whole, `$NUMUTC`'s words, the
    device buffers of NLA0:, the terminal, and the disk dumped whole. `TestSvc113Probe` runs it on
    govax and holds what the manuals settle until `vax/svc113-vax.txt` exists, then the whole.
  - MANUAL.md's formats paragraph, its missing-service example, the VMS round's paragraph, and
    Appendix A; FORTRAN-INTERNALS.md's format interpreter. Subtask 9 is next.
- **2026-11-24, subtask 9 (done):** the debugger.
  - *A.8.2, FORTRAN's lines.* A program compiled `/DEBUG` is assembled from the compiler's MACRO
    file, so its line table numbers that file's lines. When the machine loads an image with a debug
    symbol table it reads each module's source, and a module whose source is the compiler's (the
    traceback marker, `fortranMade`) gets a second numbering (`machine/traceline.go`'s
    `numberFortranLines`, over `dbgsym/renumber.go`'s `Module.Renumber`): each row the FORTRAN line
    whose comment comes last at or before it, rows of one line that follow each other in memory
    made one, code before any comment in no line, an included file's line its own file's number
    (as a traceback shows it). `Module.Listing` keeps the object's table, which `SourceLine` and
    `SourceAt` now search (`ListingAt`), so a stop still shows the MACRO instruction's line under the
    FORTRAN one, numbered as the MACRO file's. Everything else asks `Lines` and so counts FORTRAN
    lines: `%LINE n` and `SET BREAK %LINE n`, `SET BREAK/LINE`, SHOW CALLS's line column, a location's
    name (`FV\SUB\%LINE 38`), TYPE with no line after a stop. TYPE and EXAMINE/SOURCE show the
    FORTRAN source (`debugger/fortranlines.go`, `Machine.FortranLines`): the file the MACRO header's
    `; from` line names when there is one, found as any source file is, else the MACRO file's
    comments, which have every line of the first file. The numbering is redone when the files may
    have changed (`ForgetSources`, SET SOURCE). A traceback, which already mapped MACRO lines to
    FORTRAN's (Phase 110's 7c), gives the same lines. Unconfirmed, ours: VAX FORTRAN's debugger
    numbers a listing's lines, an included file's among them; govax's are the source file's.
  - *A.8.3, a component after subscripts.* The evaluator offers a name followed by any run of
    subscripts in parentheses and `%` or `.` components (`machine/expr.go`'s `referenceEnd`,
    `closingParen`) to the debugger's FORTRAN names whole, then without the parentheses as before;
    `EVALUATE S%P(2)%X + 1` and `EVALUATE M(2,3) + 1` work. `fortranName` no longer takes a dotted
    static name with subscripts for a MACRO label, whose first longword the evaluator would have
    read. Arithmetic stays on longwords, as for a MACRO label's data: a REAL's is its bits.
  - `debugger`'s `TestFortranLineNumbers` and `TestFortranLinesWithoutSource`, and cases in
    `TestFortranRecords` and `TestFortranVariables`; MANUAL.md's 14.10, FORTRAN-INTERNALS.md's
    section 35, HELP's FORTRAN_Programs and TYPE (the debug help library rebuilt); Phases 110 and
    111's items marked resolved. Subtask 10 is next.
- **2026-11-24, subtask 10 (done):** the SHARE$LIBRTL fault (A.9.2). It was govax's.
  - *The cause.* VMS 7.1's LIBRTL.EXE (on the local system disk, `DEFINE LIBRTL` to it) has a
    first transfer address, ^XCD4, which is LIB$EMODD's vector entry: a module's `.END`, kept by
    VMS's LINK, which nothing calls. govax called it at activation, before subtask 6 as
    `SHARE$LIBRTL_INITIALIZE`, since subtask 6 as a shareable image's initialization routine,
    with `(start, 0, 0)`, and LIB$EMODD read its arguments' operands: +0CE1 was the first's, +0D3F
    the third's. The rule is the header's: DECC$SHR.EXE, VMS's one shareable image with
    initialization routines, sets IHD$V_INISHR and puts its LIB$INITIALIZE's address in
    IHA$L_INISHR (its first transfer address too); LIBRTL's flag is clear. So a shareable image's
    initialization is IHA$L_INISHR's, when the flag says so, and its transfer addresses are never
    called (`machine/run.go`'s `initTransfers`, `ICB.InitShare`). LINK/SHAREABLE now sets both for
    an image with a LIB$INITIALIZE contribution (`link/image.go`), and keeps a module's transfer
    address, with LNKNOTFR clear, as LIBRTL's and DECC$SHR's headers show, which Phase 68 had left
    unconfirmed and passed over (`noteTransfer`; the image's ident stays as it was, unconfirmed). RUN
    of a shareable image is NOTFR whatever its transfer addresses (`mainTransferAddress`;
    unconfirmed for one that has one). `TestShareableTransferNotCalled`, and
    `TestLibInitializeShareable` now checks the header. Answer C's question, whether VMS runs a
    shareable image's routines, is answered yes by DECC$SHR's header; how it calls them (the
    argument list) stays the round's.
  - *The next stop, SYS$SRCHANDLER.* With LIBRTL's own code running, VMS's LIB$SIGNAL removes its
    frame, lays out the arrays and argument list below SP, and jumps to SYS$SRCHANDLER (the
    Internals book's 4.2.2.2), which govax knew only as the place its own dispatches return to.
    It now takes such a condition up as an exception's (`corevms/condition.go`'s `adoptDispatch`):
    the layout govax's own, but for a longword VMS's LIB$SIGNAL leaves between the mechanism and
    signal arrays (2 in the case seen; no source gives its meaning), and a mechanism array whose
    depth VMS starts at -3. `TestSrchandlerFromProgram`.
  - *What VMS's LIBRTL now runs on govax*: a MACRO program's LIB$PUT_OUTPUT, and
    `testdata/fortran`'s TEXT, its output its `.out`'s to the byte. STRMATH stops with FTNRTL's
    OUTSTAOVE: VMS's STR$ADD, STR$MUL, and STR$ROUND write a result's digits into a CHARACTER*20
    destination right-justified with leading zeros (`00000000000009999998`), where govax's
    (subtask 7) writes them left-justified with blanks, so SHOW's INDEX finds no blank. Not changed
    here: `testdata/str113`'s round settles it on the VAX (subtask 12), and whether VMS's LIBRTL
    may serve as a black box on govax, as FORRTL does (Phase 80's exception), is the author's to
    say.
  - MANUAL.md's RUN, HELP RUN (the help library rebuilt), and VAX-VMS-INTERNALS.md's driver and
    10.7 follow; in passing, HELP FORTRAN's three paragraphs with a line inserted unwrapped (the
    Fortran 95 paragraph, the arrays', /STANDARD's) are wrapped again; Phase 68's unconfirmed transfer address and Phase 86's fault are marked resolved.
    Subtask 11 is next.
- **2026-11-24, subtask 11 (done):** housekeeping.
  - *A full volume* (A.11.1). ods2 has no error value for an allocation that finds too few
    clusters, so `rms.DeviceFull` knows it by ods2's words (`ods2NoSpace`, held by
    `TestDeviceFullFromODS2`) or by `ErrACPDeviceFull`, and COPY's output is a
    `rms.DeviceFullError` naming it with the version made (`fullOutputName`). COPY says
    `%COPY-E-WRITEERR, error writing DUA2:[000000]BIG.TXT;1`, then `-RMS-F-FUL` and
    `-SYSTEM-W-DEVICEFULL`, `$STATUS` RMS$_FUL; an output utility's message (CREATE's OPENOUT,
    MACRO's OBJWRITE, through `objectFailureAs`) and `FileFailure`'s catch-all take the same two
    lines, where each was `%DCL-E-BADFILESPEC` with the Go error's chain. Unconfirmed, for the
    round: the STV line, each utility's own message, and whether VMS's COPY leaves the blocks it
    wrote (govax does). `TestConsoleCopy_deviceFull`. A proper ods2 error value would need an ods2
    release; the text match is the one place that would change.
  - *The Ctrl/Y level's prompt* (A.11.1) takes `-` continuation as level 0's does
    (`readControlYLevel`, `JoinContinued`; the prompt `_$ `; Ctrl/Z at a continuation's prompt
    drops what's held). `TestControlY_level`'s two new cases.
  - *COPY's message* (A.11.2): nothing without `/LOG`, as VMS's (`copyLogs`); `/VERBOSE`, govax's
    own, shows it too, `/QUIET` never. The tests that filtered the line still do; MANUAL.md's
    examples, README.md, and HELP COPY follow.
  - *The duplicated deviations* (A.11.4): the 29 entries DEVIATIONS-RESOLVED.md held copies of,
    none fully resolved, came out of it, with a stray copy of the entry template; the four whose
    texts differed were older copies of what DEVIATIONS.md has.
  - *Commands with no handler* (A.11.5, A.15.2): `dcl.Grammar.Unhandled` lists every command a
    grammar parses that only NOHANDLER can answer (a syntax with no handler; a verb with none,
    alone, or by a keyword or qualifier with no syntax of its own), and
    `TestNoCommandWithoutHandler` (console and debugger) holds both grammars to none. TEST, SHOW
    MAPS, SET UIQUANTUM, CANCEL MEMORY, and SHOW ASSEMBLER_FLAGS were already gone; it found SHOW
    EXPAND (removed), a console SHOW INSTRUCTIONS syntax no keyword reached (removed; it is the
    debugger's), and INITIALIZE and START with no qualifier, now DCL's INSFPRM as SET, SHOW, and
    INSTALL alone are (unconfirmed: VMS's INITIALIZE alone asks for a device).
  - *The dead hooks* (A.16): the attribute table's `attrNotYet`, the kind table's `pending` and
    `kindProblem.notImpl`, the file keywords' `later`, gone; `ioSpecs`' default, which no
    specifier reaches, is a panic naming the specifier.
  - *The author's report, DELETE's version message*: `DELETE FOO.TXT` said
    `%DCL-E-NEEDVERSION, ...` and the Go error's chain, where VMS 7.3's says
    `%DELETE-E-DELVER, explicit version number or wild card required` (the author's session). It
    says that now (SHR$_DELVER in DELETE's facility, raised to E as shown; `CLI_NEEDVERSION`
    unused). The same pattern, a govax message with the Go chain where VMS's utility gives its
    message over RMS's status, was in a bad specification's PURGE (`PURGE-W-SEARCHFAIL` over
    RMS$_SYN, as DELETE's), COPY's output (`COPY-E-OPENOUT`), and an output utility's
    (`objectFailureAs`: CREATE's OPENOUT, MACRO's OBJWRITE); each now shows RMS's status. A sweep of
    the file commands' common mistakes (missing files, directories, versions, bad names) found no
    other.
  - *PURGE's `/KEEP`* (the author's direction): PURGE's qualifier is VMS's `/KEEP=n`, where it
    was ods2's `/LIMIT=n` (now IVQUAL); `/KEEP=0` is DCL's `%DCL-W-IVVALU` with `\0\` as its
    segment, where it was govax's BADLIMIT (unconfirmed: the round). HELP PURGE and MANUAL.md
    follow.
  - MANUAL.md's COPY, HELP COPY (the library rebuilt); Phases 83, 86, 87, and 91 marked.
    Subtask 12, the probe round, is next.
- **2026-11-24, subtask 12a (done):** the round's own volume. Subtask 12 is split in four (the
  subtask list): this volume, the FORRTL round, the debugger round, and the session on VMS.
  - *`testdata/err113`* (README; the files by hand, no `gen.go`): `exchange.cmd` makes the volume
    (ERR113, RD54) and an RD51 scratch disk VMS initializes itself (`/HEADERS=16/EXTENSION=1`);
    `ERR113.COM` runs a section per question: STR113 and SVC113 (their outputs files of their own,
    copied back into `testdata/str113/vax` and `testdata/svc113/vax`), INI113 and INI113NT
    (`ini113.mar` and the shareable `ini113s.mar`: each initialization routine and the main program
    show their argument lists, INITB calls the init-coroutine and shows the status it returns),
    MAC113A to D (a register name as data; `;` in `<...>`; `.WARN`, `.ERROR`, and `.PRINT`;
    `MACRO/DEBUG A+B`; an `.END` before the last file; listings and objects copied back), DCL
    (DELVER, PURGE and COPY with a bad name, PURGE/KEEP=0, INITIALIZE and START alone), NOCC (TYPE
    of a file made by CREATE/FDL with no carriage control, and the same at the terminal by hand),
    FULL (COPY until a copy doesn't fit, what's left of it, DCL's OPEN/WRITE and WRITE, CREATE,
    and MACRO's object on the full volume), and INDEX (3,000 one-block files on the scratch
    volume, so its index file grows in pieces between their blocks: an extension header, or
    HEADERFULL? DUMP/HEADER and ANALYZE/DISK_STRUCTURE show it; the disk comes back whole).
    `TestErr113Procedure` rehearses it on govax (`internal/console/err113_test.go`).
  - *STR$MATCH_WILD's `%`*: `testdata/str113`'s `ascid` writes a `%` as `<37>`, so the label
    prefix spares it, and twelve MATCH_WILD cases follow the arithmetic (118 cases).
  - *RECURAST* (A.9.3): its timer is 200 ms, not 10, so VMS's AST shouldn't end before the wait
    begins; its output is unchanged on govax (`nof90.sum`'s three lines follow), and
    `ftn86VMSDifferences`' RECURAST says the round runs it.
  - *What the rehearsal found in govax*, fixed: govax's LIB$INITIALIZE module had its code in
    `$CODE`, so a program with a `$CODE` of other attributes got LINK-W-MULPSC; it is
    `_LIB$CODE` now. RMS's `$CREATE` and DCL's OPEN on a volume with no room for the file said
    RMS$_DEV; they say RMS$_FUL with SS$_DEVICEFULL now, as COPY does (`rms.DeviceFull`; the
    STV unconfirmed with the rest of subtask 11's full volume).
  - *Found and left*, for subtask 13 or later: govax's built-in utilities (TYPE, DIRECTORY, DUMP,
    ...) write to DCL's SYS$OUTPUT whatever `DEFINE/USER SYS$OUTPUT` says, where VMS's are images
    that use it (DEVIATIONS.md); and a shareable image's `.LONG` of one of its own addresses keeps
    the address as linked, base 0 (INI113S's first version, `.LONG SINIT`, called address 0),
    with no message from govax's LINK, where `.ADDRESS` is relocated, as the round's programs now
    use (what VMS's LINK says of the `.LONG` unconfirmed). govax calls a main program with LIB$INITIALIZE's
    three arguments when an image has the section (`MAIN: 3 arguments`); VMS's count is the
    round's answer.
  - Subtask 12b, the FORRTL round, is next.
- **2026-11-24, subtask 12b (done):** the FORRTL round, round 3 of `testdata/for81` (its
  README's "Round 3"), on the round's volume in `[FOR81G]` (`err113`'s `exchange.cmd`,
  `copyout.cmd`, and README's session).
  - *What it asks* (`gen.go`'s `round3`, `for81g.mar`, 64 cases): the current record after
    REWIND, BACKSPACE, ENDFILE, and UNLOCK (DELETE on relative units, REWRITE on sequential FIX
    ones, since REWIND of a relative unit is REWERR); DELETE by number on a unit not open and on a
    sequential-access relative unit, and the associated variable after it (code 17: round 1's
    `kAssoc` was 22, IOSTAT, which is how IOSTAT was found; `kAssocVar` now); record -1 and 0
    through FORRTL, and RMS's keyed `$GET` of a relative and a FIX file (`KEYGET`); an implicit
    READ's and WRITE's FAB; a last FIX record the end of file cuts short (`MKSHORT`: one block
    `$WRITE` to a FIX 24 file, 21 records and 8 bytes), read formatted, unformatted, and direct; a
    relative file's growth from one record far out; thirteen files of segment words; ERRSNS after
    ERR= and IOSTAT= took an error; the PC and the user PC of errors found at the begin call, an
    item's call, and FOR$IO_END (`HNDPC`); I0, Iw.m, B0, O0, Z0, F0.d, and E, G, L, A, D with w 0;
    a longword, a word, and a byte value on a signed word key; units -1 to -4 with FOR$PRINT,
    FOR$TYPE, FOR$ACCEPT, and FOR$READ defined. `for81t.mar` is the terminal's part, by hand (a
    line of 250 characters, unit 6 and ACCEPT read there, 140 and 200 written). `$FORIOSDEF`'s
    names (A.10) can't be asked of FORRTL; FORRTL's by-name NAME over govax is govax's own bug,
    not a VMS question.
  - *The rehearsal*, `TestFor81GProcedure` (`internal/console/for81g_test.go`), runs it on VMS's
    FORRTL.EXE over govax. FORRTL's own logic runs there as on the VAX, so what doesn't reach RMS
    is already answered (the VAX confirms it); subtask 13 compares FTNRTL with these:
    - Units: -1 opens FOR$PRINT, -2 FOR$TYPE, -3 FOR$ACCEPT, -4 FOR$READ.
    - The user PC: an output conversion error (OUTCONERR) and FORVARMIS report FOR$IO_END's
      return, as FTNRTL does; INPCONERR reports the failing item's call's return, where FTNRTL
      reports FOR$IO_END's; an end of file and FMTSYN the begin call's.
    - ERRSNS: an error ERR= or IOSTAT= took on OPEN or CLOSE is saved, and ERRSNS clears it (a
      second call is all zeros); INQUIRE of unit 120 with ERR= succeeds.
    - Segment words: only bit 1 (last) ends a record: a first bit again, a last segment first,
      and a middle one first are all joined into one record; a word over 3, a record shorter
      than 2 bytes, and an empty record are SEGRECFOR (35); a file ending inside a record is
      ENDDURREA (24).
    - Zero widths: FORRTL takes w 0 in I, O, Z, F, E, G, L, D as a field of no columns,
      OUTCONERR (63) for every value but I0.0 of 0, which is empty without error; A0 writes nothing
      without error; B is FMTSYN. So VMS's FORRTL has no Fortran 95 zero widths; Phase 112's
      `I0` of zero and F0.d's sign stay govax's choices.
    - Keys: a longword on a word key is taken if its value fits (5, 70, -3) and KEYVALERR (49)
      if not (70005, 65541: not truncated); a byte value is KEYVALERR, with RMS's status.
    - The associated variable (preset to -1 before each statement): DELETE of record 3 sets 4, a
      second DELETE of 3 (DELERR) leaves it, READ of 1 sets 2, then DELETE of 1 sets 3 (not 2:
      FTNRTL's r+1 holds only for the first), and FIND of 4 sets 4. The rule isn't plain yet;
      subtask 13 may want a case or two more.
    - Record -1 and 0: FORRTL's RECNUMOUT (25) on a relative unit, before RMS; on a sequential FIX
      direct unit RMS is asked (ERRDURREA and ERRDURWRI, govax's RMS$_KEY).
  - *For the VAX* (RMS's side): the current record after each operation; RMS's keyed `$GET`
    statuses; the short FIX record (govax gives the 8 bytes, blank-padded formatted, INPSTAREQ
    unformatted and direct); the relative files' allocations (govax: 3, 3, 21, 93, 51, 603, and 9
    blocks); an implicit READ's FAC (govax: 1F); the terminal's limits.
  - Subtask 12c, the debugger round, is next.
- **2026-11-24, subtask 12c (done):** the debugger round, `testdata/dbg113` (its README), on
  the round's volume in `[DBG113]` (`err113`'s `exchange.cmd`, `copyout.cmd`, and README's
  session).
  - *What it asks* (A.8.6): one program, `dbg113.mar`, a stage per question, and eleven
    sessions. DBG113S: an outer scope's PSL (IV set by REC's mask, DV by LEAF's, N live), so
    whether VMS shows the live PSL, the frame's saved PSW, or neither; `%NEXT_SCOPE_ENTRY` at
    the last levels; EXAMINE with no location after PC, PSL, SP, and R11; TYPE with none
    shown; and dbg89's lost DEPOSITs again in `%HEX`, to an outer scope's R2 (in a frame), R5
    (in none), R6, R0, SP, PSL, and PC, then where each value is after the RETs. DBG113A:
    dbg85's `@(AP+N)`, which read memory at FFFF0038, as if AP's address were FFFF0030 (Rn's
    FFFF0000+4n?): registers in EVALUATE, EVALUATE/ADDRESS, `@(AP+4)`, `R2:R4`, and EXAMINE
    and DEPOSIT of FFFF0008 to FFFF0044. DBG113I: G and H short literals; EXAMINE/INSTRUCTION
    of a register past its bytes; an entry mask's next location; EXAMINE/OPERANDS brief and
    FULL of every operand kind. DBG113T: CANCEL of nothing, SHOW BREAK's and SHOW TRACE's
    order, tracepoints by address, line, and branch at one PC, `/AFTER` and WHEN on
    tracepoints and watchpoints. DBG113R, N, and O: STEP/RETURN and STEP/OVER interrupted by
    a break, then GO (and a second STEP/RETURN). DBG113L: a line step from a RET into the
    middle of the caller's line (a macro's two CALLS). DBG113X: exception breaks' locations
    under SET STEP LINE and INSTRUCTION, SHOW LANGUAGE with the PC in no module, and the
    program's normal end (its exit handler's line in the log). DBG113M: SHOW SYMBOL with
    `/ADDRESS` and `/TYPE`, SHOW LANGUAGE and SET LANGUAGE UNKNOWN, SET, SHOW, and CANCEL
    SOURCE with `/MODULE`, NOSUCHMODULE, a 184-column source record, and EXIT at a break.
    DBG113Q: QUIT at a break. DBG113S runs again on GVDBG113, govax's image, DUA5 (where
    govax's MACRO named the source) a logical name for the round's device. Not asked: Phase
    42's multi-dimension bounds (MACRO describes no array), and dbg85's `SHOW SYMBOL *` and
    `dbg85c.dlg` (VMS's own images).
  - *The rehearsal*, `TestDbg113Procedure` (`internal/console/dbg113_test.go`), runs
    DBG113.COM on govax and logs each session (each compared with VMS's once `vax/` is
    here). It found two mistakes in the sessions, fixed: `LOOPTOP+2` was the middle of an
    instruction (LOOPADD is a label now), and govax read `%HEX FFFF0030` as a symbol (a
    leading 0 now). What govax does now, for subtask 13 to compare:
    - TYPE with none shown is CLI MISSINGPARAMETER; scope 5 (below START) isn't there
      (SYMNOTACT), so `%NEXT_SCOPE_ENTRY` stops at 4; an outer scope's PSL has the frame's
      PSW with its condition codes clear; after PC, EXAMINE goes to PSL, and stays there.
    - The DEPOSITs go where the value is kept (the frame, or the live register) and come
      back after the RETs; R0 is SYMNOTACT, SP and PSL OPTNOTALLOW, PC taken.
    - A register's name in an address expression is its value: FFFF00xx is no read access.
    - EXAMINE with no location after EXAMINE/INSTRUCTION R5 went to LOOP's line 126, an
      address that seems to come from nowhere: probably a govax bug, whatever VMS says.
    - OPERANDS=FULL is brief; an immediate or branch operand gets no line.
    - CANCEL TRACE and WATCH of nothing say NOTRACES and NOWATCHES; the interrupted
      STEP/RETURN's stop comes at the GO, a second STEP/RETURN replaces the first, and an
      interrupted STEP/OVER's end is dropped; a line step from SUBC's RET stops at
      `%LINE 63+7`.
    - SET and CANCEL SOURCE have no `/MODULE` (SYNTAX); EXIT and QUIT at a break run no
      exit handler, and QUIT isn't logged.
  - Subtask 12d, the session on VMS 7.1, is next.
- **2026-11-24, subtask 12d (done):** the session on VMS 7.1, the author's, every part run and
  its results copied back (`testdata/err113`'s README, "What came back").
  - *ERR113* took three passes. The first stopped in STR113: STR$RECIP with a precision limit
    whose digits are `0` (`RECIP 6 0e-4`) never returned (minutes of CPU time, no I/O). That is
    VMS's answer; the case is out of `testdata/str113` (117 cases), and ERR113.COM takes P2, a
    section to start at. The second ran every section with the scratch disk unattached, so FULL
    and INDEX were run again by a third (`"" FULL`); `vax/err113.log` is the second with the
    third's two sections, `vax/err113-rerun.log` the third as it came.
  - *Where things are*: `testdata/err113/vax` (the log, INI113's images, MAC113's listings and
    objects), `testdata/str113/vax/str113-vax.txt`, `testdata/svc113/vax` (and `def_dib.obj`),
    `testdata/for81/vax`'s `for81g-vax.txt`, `for81g.log`, and `for81t.log` (the terminal, by
    hand), `testdata/dbg113/vax` (eleven sessions, GVDBG113's, the listing, map, object, and
    image), `testdata/ftn86/vax/round6` (DUNGEON's log `testdata/disks/ftn86d-round6.log`, the
    same as round 5's), `testdata/f90r/vax/round3`; the scratch disk stays in `testdata/disks`
    as the VAX left it.
  - *TestStr113Probe and TestSvc113Probe* hold govax to the manuals' examples until subtask 13
    sets `str113Compared` and `svc113Compared`; meanwhile they log how many lines differ from
    VMS's run (93 and 62). VMS agrees with every manual example.
  - *A first look*, for subtask 13 to work through:
    - STR$: VMS moves a result's trailing zeros into the exponent and drops its leading zeros
      (10e1 + 0e1 is `1` with exponent 2, 007 + 3 `1` with exponent 1, where govax gives `10`
      with exponent 1 and 0). `DEFINE/USER SYS$OUTPUT STR113.TXT` of an existing file put the second
      run's output after the first run's in that file, version 1, not in a new version (taken
      off `str113-vax.txt`); whether govax does so is a question for 13.
    - LIB$INITIALIZE: the shareable image's routine runs first, with 1 argument; the main
      image's routines and the main program get 6 (govax's main: 3); the init-coroutine runs the
      main program and returns its status (3), the image's `$STATUS`; the same `/NOTRACEBACK`.
    - MACRO: a register name in a data directive is an error (DIRSYNX; DATALSTSYN in a list;
      ADRLSTSYNX for `.ADDRESS`), status E. `.WARN` and `.ERROR` are GENWRN and GENERR with the
      expression's value before the text; `.PRINT` the value and the text, no message.
    - DCL: DELETE without a version is DELETE-E-DELVER (`%X1093120A`); a bad directory to PURGE
      and as COPY's output DCL-W-DIRECT with the segment; PURGE/KEEP=0 PURGE-F-BADVALUE;
      INITIALIZE and START alone DCL-W-INSFPRM.
    - NOCC: in the log `ONETWOAFTER`, one line; at the terminal `ONE` began a line, `TWO`
      followed on it, and `AFTER` began a line of its own.
    - FULL: five copies of BIG.DAT (4,064 blocks) fit, the sixth is COPY-E-OPENOUT with
      RMS-F-FUL (`%X106710A2`) and COPY-W-NOTCOPIED, nothing left of it
      (NOFILES); DCL's OPEN/WRITE opens, its WRITEs go on until one is RMS-F-FUL (`%X00018544`),
      and CLOSE succeeds; CREATE makes the file and then CREATE-E-WRITEERR with RMS-F-FUL;
      MACRO's object MACRO-E-OPENOUT with RMS-F-FUL.
    - INDEX: all 3,000 files made; INDEXF.SYS grew to 3,374 blocks in eleven extents (807, 686,
      583, ... blocks, not ones), all in its one header, no extension header.
    - FOR81T: a terminal READ through FORRTL takes 132 characters, then FOR error 39 (ERRDURREA)
      with RMS$_TNS; the rest of the line went to the next read.
    - The debugger: VMS's debugger on GVDBG113, govax's image, wrote DBG113S's session to the
      byte.
  - Subtask 13, the fixes, is next.
- **2026-11-24, subtask 13a (done):** STR$'s arithmetic made VMS 7.1's. Subtask 13 is split in
  six (the subtask list): STR$, the services, ERR113's sections, the index file, FTNRTL, and the
  debugger.
  - *What VMS's run showed* (`internal/librtl/strmath.go`'s comments have each rule):
    only a sign's bit 0 counts (2 positive, 3 and -1 negative); digits are read up to the first
    character that isn't one (" 12" is 0, "12A" 12, "1.2" 1), never refused; ADD's, MUL's,
    and RECIP's results lose leading zeros and move trailing zeros into the exponent; ADD's
    and RECIP's zero is sign and exponent 0, MUL's keeps the sign and the exponents' sum (-3 times
    0 is sign 1); exponents wrap as longwords with no condition; a fixed destination takes a
    shorter result right-justified with zeros (`006`); a destination of no class is LIB$_INVARG,
    nothing written; DIVIDE with too few arguments is LIB$_INVARG (the others STR$_WRONUMARG).
  - *DIVIDE*: the quotient is truncated to places+indicator digits after the point, then, for
    an indicator of exactly 1, rounded half up to places; its exponent is always -places (so 2
    and 3 keep more digits, -1 one fewer); a zero dividend gives places+indicator zeros with sign
    0 (`00`; the one case), a quotient that truncates to zero keeps its sign; a zero divisor
    writes the sign and exponent before STR$_DIVBY_ZER.
  - *RECIP*: the precision limit B gives the places p: from -B's exponent, one more while 10^-p
    is not below B, so digits of 1 give one more place and any others none (1e-4: 5 places;
    9e-4, 25e-4: 4). Digits of 0 loop for ever on VMS; govax gives LIB$_INVARG (DEVIATIONS.md).
  - *ROUND*: the digits as written, leading zeros and all (0099 to 1 place is `0` times 10^3);
    a number no longer than the places returned as it is; places 0 a plain zero; -1 a zero too,
    where VMS's writes 65,535 bytes from beyond the digits (`expectedStr113Differences`, with
    STR$ELEMENT_R8's registers, which are VMS's code's scratch values).
  - `TestStr113Probe` now holds govax to `vax/str113-vax.txt` (`str113Compared`), the unit
    tests in `internal/librtl/strmath_test.go` to the same answers; MANUAL.md's "Strings" says
    what the routines do, and `testdata/str113`'s README.
  - *STRMATH* (`testdata/fortran/strmath.for`) assumed blank padding: its SHOW printed
    `C(1:INDEX(C, ' ') - 1)`. Round 6 ran it on VMS, and the PRINT stopped with OUTSTAOVE, as
    govax's does now; SHOW prints the whole field, `strmath.out` the zero-padded digits
    (`nof90.sum` rewritten for it; `ftn86AwaitingVMS` keeps STRMATH until 13g).
  - VMS's `DEFINE/USER SYS$OUTPUT` of an existing file appending (12d's note) is not STR$'s;
    it goes with ERR113's sections, 13c.
  - Subtask 13b, the services, is next.
- **2026-11-24, subtask 13b (done):** the services made VMS 7.1's (`testdata/svc113`'s README
  lists what the run settled).
  - *The device information buffer* (`internal/corevms/getchn.go`): `$DIBDEF`, decoded from the
    round's `def_dib.obj`, put the names in DIB$T_DEVNAME's 76 bytes at 36 and DIB$L_MAXBLOCK at
    112, where govax had MAXBLOCK at 36, a DEVDEPND2 at 40, and the names after the 116 bytes.
    VMS writes only the fixed fields, the names (the label blank-padded to 12), and MAXBLOCK; the
    returned length is the buffer's, at most 116. Fewer than five arguments is SS$_INSFARG.
  - *`$ASSIGN`'s owner* (`devices.go`'s `newChannel`): VMS's NLA0: kept PID 0 and [1,1] after the
    program's `$ASSIGN`; govax made every assigner a device's owner (eVAX's rule). Now only a
    device without DEV$M_SHR becomes the assigner's, VMS's implicit allocation; a mailbox stays
    its creator's.
  - *The UTC services* (`utc.go`): `$TIMCON` takes any flag but 0 as 1 (govax's BADPARAM gone)
    and signals an access violation at an address it can't reach, its write bit for the store,
    as VMS's did for an address of 0; `$ASCUTC` into a buffer that holds the date and its blank
    succeeds; `$BINUTC` of a delta time writes the infinite inaccuracy before SS$_IVTIME;
    `$NUMUTC` of an inaccuracy of 0 gives days 17.
  - `TestSvc113Probe` now holds govax to VMS's output (`svc113Compared`), DIB$L_OPCNT masked
    (`svc113Masked`), the TT and SYS$DISK cases explained differences (the test console has
    neither device); `TestUTCServices` and `TestGetchnGetdev` follow the new rules.
  - Subtask 13c, ERR113's sections, is next.
- **2026-11-24, subtask 13c (done):** ERR113's sections made VMS 7.1's. `TestErr113Procedure`
  now holds govax's log to `vax/err113.log` section by section, but for `err113NotCompared`
  (MAC113A and B, whose messages are govax's own form, and INDEX, 13d's) and
  `err113Differences` (each with why), the arguments' addresses masked (`err113Masked`);
  `TestErr113Macro` holds the MAC113 listings and the A+B objects to VMS's.
  - *LIB$INITIALIZE*: RUN's driver calls a main image's LIB$INITIALIZE with the six arguments
    VMS gives a main program (start; the CLI coroutine, image header, and file information, 0 in
    govax; the image's link flags, `ICB.LinkFlags`; the CLI flags, 0), and a shareable image's
    with start alone (`machine/run.go`'s `initCall`). `libinit.mar` passes each routine a copy of
    its own list, its first argument CONTINUE (`ARGS`), so INITA and INITB get 6, SINIT 1, and
    the main program its six with start stepped to it, as VMS's did. `TestLibInitialize` checks
    for six. A program with no initialization routines is still called with none.
  - *MACRO*: a register name as a data item is DIRSYNX as a directive's first item (a zero
    stored, the rest left), DATALSTSYN as a later one (a zero in its place), ADRLSTSYNX then
    DIRSYNX in `.ADDRESS` (nothing stored) (`asm`'s `registerAhead`, `registerItem`; the
    `!` under the item, or a later item's comma; `VAX_DATALSTSYN`, `VAX_ADRLSTSYNX`); two
    messages with no bytes between each have the location after them (`listNotedLine`).
    VMS's listing marks MAC113A's DATALSTSYN zero relocatable and puts `.ADDRESS`'s `!`s
    under columns 7 and 12, which govax doesn't follow (`mac113Allowed`); `4+R1` stays a
    symbol (unconfirmed). DEVIATIONS.md's Phase 111 entry moves to DEVIATIONS-RESOLVED.md.
    `.PRINT`'s line lists its value; the terminal's `.PRINT` lines, warnings, and errors are
    shown in their lines' order (`dclcmd`'s `showReports`, `Assembler.MessageLines`).
    `MACRO A+B` numbers each file's lines from 1 in the listing and heads a page with the file
    it starts in (`Assembler.SetFileLines`, `ListingOptions.Files`), and declares each file
    to the debugger where its code starts, its line numbers going on from the last file's
    (`obj.DSTNextSourceFileRecord`, `LineTable.Source`, `ObjectOptions.SourceFiles`), so
    MAC113C's object is VMS's but for the files' dates.
  - *DCL*: DELVER's status has STS$M_INHIB_MSG (`%X1093120A`); a `$FILE` parameter (the grammar's
    `TypeFile`, PURGE's and COPY's) with a directory bracket left open is CLI$_DIRECT with the
    value as its segment, before the command runs (`dcl`'s `directoryClosed`, `badParameter`);
    PURGE's facility is 148 (`purgeFacility`), and PURGE/KEEP=0 is PURGE-F-BADVALUE
    (`%X10941114`). Other commands' file parameters are still `$string` (VMS's are surely
    `$FILE` too; unconfirmed one by one).
  - *NOCC*: TYPE of a file whose records have no carriage control writes them one after
    another and ends the line after the last (`rms`'s `writeUncontrolledRecords`), as VMS's
    TYPE showed them at the terminal. VMS's log has `ONETWOAFTER`, govax's COPY to the host
    running the VFC file's records with no control into the next (the records are
    `00 00`-controlled; `err113Differences`).
  - *FULL*: COPY gives a volume output its input's blocks as it makes it, so on a full volume
    the copy fails at its open, leaving nothing: COPY-E-OPENOUT, `-RMS-F-FUL`, then
    COPY-W-NOTCOPIED for the input, status `%X106710A2` (`rms.DeviceFullError.Open`,
    `copyOpenOut`); a copy that works leaves `$STATUS` SS$_NORMAL. DCL's WRITE on a full
    volume is RMS$_FUL (it was RSZ). A record file that fills as it's written is left, a
    `DeviceFullError` naming it: CREATE says CREATE-E-WRITEERR (`createWriteErr`,
    `%X109110D2`), and MACRO deletes the object and says MACRO-E-OPENOUT (`macroOpenOut`,
    `%X100310A2`). VMS showed RMS$_FUL with no STV line in each, so the utilities' messages
    no longer add SS$_DEVICEFULL. On govax the volume REST.DAT filled has no block for
    MORE.TXT's header, its index file growing by a cluster at least where VMS's grew one
    block (INDEXF.SYS's 1-block extent at LBN 20479), so CREATE still fails at its open:
    13d's.
  - *`$DIBDEF`*: `testdata/svc113/decode.go` writes `defined.txt` from VMS's `def_dib.obj`;
    its names are in `vmsdef`, the macro library (`mkdefs`), and FORTRAN's modules
    (`$DIBDEF`), so SVC113's DEF_DIB assembles on govax.
  - *A user-mode SYS$OUTPUT file*: an image's records go at the end of an existing file, as the
    LIB$ manual's LIB$PUT_OUTPUT opens one (`corevms/outfile.go`), which is what VMS 7.1 did with
    STR113.TXT (13a's note); only a file that isn't there is made.
  - Tests that held govax's earlier choices now hold VMS's: `TestTIR111Objects` and
    `TestTIR111ErrorObjects` compare TIRR's object whole (VMS's record for record),
    `TestConsoleCopy_deviceFull` (no STV; CREATE's WRITEERR), `TestConsolePurge_badLimit`,
    `TestTB70OutputSameFile` (both runs in one version); `TestFTNRTLListOutput`'s program had
    data labeled R1 to R14, which VMS's MACRO refuses too, now X1 to X14.
  - MANUAL.md's TYPE, COPY, PURGE, the MACRO command, LIB$INITIALIZE, and SYS$OUTPUT files;
    HELP COPY and PURGE.
  - Subtask 13d, the index file's growth, is next.
- **2026-11-24, subtask 13d (done):** the index file grows as VMS 7.1's does (ods2's
  `volume/writeheader.go`, `indexFileGrowth`).
  - *What VMS's run showed* (ERR113's INDEX section: an RD51, MAXFILES 5,400, `/HEADERS=16`,
    3,000 one-block files): INDEXF.SYS grew by 807, 686, 583, 496, 421, and 358 blocks, each one
    extent, and each 15% of the header slots MAXFILES still allowed (5,383, 4,576, 3,890, 3,307,
    2,811, and 2,390 left), the fraction dropped; 3,374 blocks in eleven extents, all in its
    one header. In FULL, with one block free, it grew by that block.
  - *ods2*: a new header's slot past the index file's end grows it by 15% of the slots left, at
    least what the slot needs, in one piece as long as the volume's longest free run, else what
    the slot needs (in pieces if it must); it was 16 blocks at a time, a retrieval pointer each,
    which is how Phase 111's volume needed the extension header VMS couldn't read. Where VMS puts
    each piece (the 807 wasn't the first free run that fit) isn't followed: ods2 takes the first.
    A one-block growth on a header with room for one pointer wrapped round (`blocks-first`); fixed.
    `TestIndexFileGrowsAsVMS` makes VMS's 3,000 files and gets VMS's six extents;
    `TestIndexFileGrowthOnAFullVolume` the one free block; `TestIndexFileGrowsIntoExtensionHeaders`
    now takes every other cluster first, so its growths are still one block each and still fill
    the header.
  - *What VMS does when the header does fill* (an extension header, or HEADERFULL) is still
    unconfirmed; ods2 still writes its own extension header there, now only on a volume too
    fragmented to give the pieces whole (DEVIATIONS.md's Phase 111 entry, updated).
  - `TestErr113Procedure` checks INDEX's 3,000 files and INDEXF.SYS's size, 3,373 blocks: VMS's
    less the block its index file grew by in FULL (govax has no DUMP/HEADER or
    ANALYZE/DISK_STRUCTURE for the rest). FULL's MORE.TXT difference stays with a new reason:
    VMS's REST.DAT left one block free (why is unconfirmed), and govax's fills the volume, so
    there is no block for the header.
  - `TestLib58Oracle`'s "FTN unused header" differences are gone: file number 200's slot is
    inside govax's index file now, as it was inside VMS's 829 blocks (SS$_FILESTRUCT); Phase 58's
    log marked.
  - Subtask 13e, FTNRTL to the FORRTL round, is next.
- **2026-11-24, subtask 13e (done):** FTNRTL made VMS 7.1's FORRTL in round 3 of
  `testdata/for81`. `TestFor81GOnFTNRTL` (`internal/console/for81g_test.go`) runs FOR81G on
  FTNRTL, its FOR$ calls passed on by `testdata/for88`'s `for88f.mar` and the new
  `testdata/for81/for81gf.mar` (FOR$IO_L_V, IO_L_R, IO_F_R, ERRSNS; for88f.mar now passes
  IOSTAT=, code 22, as FTNRTL's control block flag), and holds all 64 cases to
  `vax/for81g-vax.txt` but for `for81GFTNRTLDifferences`; `TestFor81GProcedure` now holds FORRTL
  over govax to it too (`for81GDifferences`); both run in `[FOR81G]`, as on the VAX, with the
  condition's own PC masked (`for81GMasked`: it is wherever FORRTL or FTNRTL was loaded).
  - *RMS and ods2* (seen under both libraries): a FIX file's last record cut short by the end of
    file is read whole, the bytes past the end as the blocks hold them (ods2's
    `blockStream.readPastEnd`, the Reader's `nextFixed`; ods2's change is in the sibling
    checkout, to be tagged and pinned, CLAUDE.md's rule); `$REWIND` leaves no current record
    (`recordops.go`); a relative file's key of a negative longword is RMS$_KEY, as 0 is
    (`relativeKey`); a terminal `$GET` whose buffer fills before the line ends is RMS$_TNS with the
    record, the rest of the line the next `$GET`'s (`rms/terminal.go`, `getTerminal`).
  - *FTNRTL*: FTN$IO_END returns the FOR number after a signal a handler went on from
    (OUTCONERR's 63, where it returned 0), and an end of file or SYNERRFOR found before the end
    reports the user PC of the call that found it (CTX_ITEMPC; an OPEFAI at the begin call keeps
    FOR$IO_END's, as ftn86's DIALOG shows VMS's); a REWIND or BACKSPACE refused (REWERR, BACERR)
    keeps the current record, a BACKSPACE's is the last record it read again, and one a READ
    reached that they take away is `CURRENT_GONE`, a REWRITE's NO_CURREC then carrying RMS$_CUR
    (NO_CURREC is in `RMS_REPORTED`, each refusal setting the unit's status); UNLOCK on a direct
    unit makes DELETE (u) DELERR with the `$FREE`'s RMS$_RNL (`CURRENT_UNLOCKED`; on a
    sequential-access unit it went on, #9); DELETE (u'r) on a sequential-access unit is
    OPEDEFREQ, and a record number below 1 on a relative unit is RECNUMOUT before RMS (also
    `FTN$$UNIT_DIRECT`'s); a segment's control word says only where a record ends (bit 1), a
    first bit out of place no longer SEGRECFOR; a longword on a word key the key can't hold is
    INVKEYSPE (49) with no RMS status (an unsigned word's limit govax's choice); after a `$GET`
    RAB$W_USZ is OPEN's size again (133, #18's implicit READ); and at a terminal a READ's `$GET`
    asks for OPEN's size, so a line past 133 characters is ERRDURREA with RMS$_TNS and its rest
    the next READ's, as FOR81T showed (`TestFortranTerminalLongLine`).
  - *The terminal's limit and fcalc* (the author's choice, VMS's 133 everywhere): fcalc reads
    199-character lines from files still, but at a terminal a longer line than 132 is LINELONG and
    its rest the next line. `testdata/fcalc`'s `basics.out` and `errors.out` are the terminal's
    transcripts now, and `basics.file.out` and `errors.file.out` (the old ones) what a file gives,
    which `TestF90ROracle` (`f90rFileOut`) and `compare.py` hold VMS's and gfortran's runs to;
    `TestFortranFormattedInput` and `TestFortranListInput` type lines VMS's way; MANUAL.md's 14.7
    and FTNRTL notes, fcalc's comments.
  - *Left, unconfirmed* (for a round 3b on the VAX): the associated variable after DELETE (u'r)
    (#14: VMS set 3 after a READ of 1 and DELETE of 1; FTNRTL's r+1 gives 2; the larger of
    FORRTL's next record and r, plus 1, or the next record plus r, both fit); VMS's `0` written
    before an O0 or Z0 field of 0 (#58, under FORRTL over govax too: VMS's OTS$ conversion into a
    field of no columns, it seems); a REWRITE after UNLOCK on a direct unit; an unsigned word key's
    limit; the user PC of an error found at an item's call other than INPCONERR; a terminal line of
    exactly 133 characters (whether its return is an empty next line); a REWRITE after a
    BACKSPACE to the first record.
  - Subtask 13f, the debugger to the debugger round, is next.
- **2026-11-24, subtask 13e2 (written and rehearsed):** round 3b of `testdata/for81`, by the
  author's offer of another VAX run, asks what 13e left open (its README's "Round 3b").
  `gen.go`'s `round3b` writes FOR81K (28 cases): A, the associated variable and INQUIRE's NEXTREC
  after DELETE by number in orders where 13e's two candidate rules differ; B, OTS$CVT_L_TO, _TZ,
  and _TI called directly into fields of 0 and 1 columns; C, REWRITE and DELETE after a direct
  READ, with and without UNLOCK; D, an unsigned word key (UW.IDX, made by RMS, DTP=BN2) and a
  signed word key's limits; E, the user PC of ENDDURREA at a `/`, OUTSTAOVE, INPSTAREQ, an
  unformatted ENDDURREA, and FILNOTFOU; F, REWRITE after BACKSPACE on records told apart. `term3b`
  writes FOR81W, lines of exactly 133 and 132 characters at the terminal, by hand.
  `exchangek.cmd` makes `testdata/disks/for81k-exchange.dsk`, `copyoutk.cmd` brings back
  `vax/for81k-vax.txt` and `vax/for81k.log`; FOR81W's text goes in `vax/for81w.log`.
  - *The rehearsal* (`internal/console/for81k_test.go`): `TestFor81KProcedure` on VMS's FORRTL
    over govax, `TestFor81KOnFTNRTL` on FTNRTL (`for81gf.mar` now has FOR$READ_DO and
    FOR$WRITE_DO). What FORRTL already shows over govax, for the VAX to confirm: DELETE (u'r)
    sets the associated variable and NEXTREC to r+1, but to r+2 when r is the record just read
    (READ 4, DELETE 4: 6; round 3's READ 1, DELETE 1: 3), which may come from govax's RMS; an
    unsigned word key takes a longword from 0 to 65,535 (65,536 and -1 INVKEYSPE) and a word as
    it stands, a signed one -32,768 to 32,767; an unformatted READ's end of file is found at its
    first item's call, and FILNOTFOU at the begin call reports the begin call's user PC (FTNRTL:
    FOR$IO_END's for both); a REWRITE after BACKSPACE rewrites the record before the one gone
    back to; govax's OTS$CVT_L_TO into no columns writes nothing and returns OTS$_OUTCONERR.
- **2026-11-24, subtask 13e2 (VMS's answers):** the author ran round 3b on VMS 7.1 (V7.1, a VAX
  8600 on simh); `vax/for81k-vax.txt` and `for81k.log` came back by `copyoutk.cmd`, and FOR81W's
  terminal text is `vax/for81w.log`. `TestFor81KProcedure` (FORRTL over govax) now holds all 28
  cases to VMS's run, and `TestFor81KOnFTNRTL` all but #19 (`for81KFTNRTLDifferences`); round 3's
  `for81GDifferences` and `for81GFTNRTLDifferences` are empty.
  - *An RMS bug* (C and F): a REWRITE of a sequential FIX record under FORRTL or FTNRTL was lost.
    `$UPDATE` wrote the record, then `$CLOSE` flushed ods2's Writer, which held the file's partly
    filled last block for appends, over it. ods2's `Writer.Reload` (in the sibling checkout, to be
    tagged with 13e's change) reads the block again; `rms`'s `FileHandle.writeInPlace` flushes the
    Writer, writes, and reloads it, for `$UPDATE`, DCL's WRITE/UPDATE, a keyed `$PUT`, and `$WRITE`.
  - *OTS$* (B): VMS's OTS$CVT_L_TO and _TZ given 0 and a string of no characters write the 0 in
    the byte before it and return OTS$_OUTCONERR (L_TI writes nothing), which is round 3's
    `O0 0 0|`; govax's do so now (`librtl/otscvt.go`'s `zeroBeforeEmpty`,
    `TestOTSZeroIntoNoColumns`), and so does FTNRTL's O0 and Z0 of 0 outside Fortran 90
    (`ftn_edit.mar`'s OUT_OZ).
  - *The associated variable* (A): FORRTL over govax gave VMS's values throughout: DELETE (u'r)
    makes the next record r+1, or r+2 when it was r+1 already (READ 4, DELETE 4: 6); FTNRTL's
    `ftn_record.mar` follows (after FIND r, whose next record is r, unconfirmed).
  - *The current record* (C, F): after a REWRITE on a direct unit DELETE (u) is DELERR with no RMS
    status, and FORRTL reports RMS's status for such a refusal only when RMS's last operation failed
    (`CURRENT_UNLOCKED`, set by FTN$IO_END for a direct REWRITE); a REWRITE after UNLOCK goes on; a
    REWRITE after BACKSPACE with nothing read is NO_CURREC with RMS$_CUR (`TAKE_CURRENT` always).
  - *Keys* (D): an unsigned word key takes a longword from 0 to 65,535, a signed one -32,768 to
    32,767, and a word as it stands, as 13e had them.
  - *The user PC* (E): it is the call during which FORRTL found the error. An unformatted READ's
    record is read at the first item's call (`ftn_unformatted.mar` marks `PC_NEXT`); after an item
    FORRTL goes on through the format to the next data descriptor, so a `/`'s end of file is the
    item before's (`ftn_format.mar` marks `PC_PREVIOUS` for a failure on the way to an item's first
    descriptor); OUTSTAOVE and INPSTAREQ are their item's, FILNOTFOU the begin call's. FTN$IO_END
    reports every failure found before the end at its call (`CTX_ITEMPC`, adjusted by
    `CTX_PCMOVE`, a byte of the context), but OPEFAI, which keeps FOR$IO_END's (ftn86's DIALOG).
    #19 is left: FTNRTL signals at FOR$IO_END, after the second item's call, where VMS's FORRTL
    signals during the first; the user PC is VMS's.
  - *FOR81W*: the lines typed were 121 and 119 characters, under the 133 asked for, so whether a
    line of exactly 133 leaves its Return to the next READ is still open; the README gives lines
    to paste for another run.
  - *Found and left*: `TestBatchLaterSectionsOracle` fails about one run in six, before this work
    too (a SYNCHRONIZE/ENTRY=63 whose job is sometimes still there): a timing race in the test.
- **2026-11-25, subtask 13f (done):** the debugger made VMS 7.1's in `testdata/dbg113`'s round
  (its README's "What came back" lists every answer). `TestDebuggerSessionOracle` now replays the
  eleven sessions and GVDBG113S on VMS's image, and `TestDbg113Procedure` holds DBG113.COM's logs
  on govax to VMS's whole (an LCS of lines, stack addresses masked, `dbg113Differences`).
  - *Scopes* (`machine/scope.go`, `debugger/scope.go`): the scope walk goes on past a frame whose
    PC is in no routine (`walkFrames`' `through`), such a level named by its number (`1\%PC`, and
    `0\%R2` at a break in no routine); an outer SP is NOALTERSP, an outer PSL's DEPOSIT goes to the
    frame's saved PSW (`SetSavedPSW`); after the PSL, EXAMINE is NOSUCC.
  - *Locations and values* (`machine/expr.go`'s `RegisterAddresses`, `EvalLocation`,
    `EvalDepositValue`; `debugger/data.go`'s `registerLocation`, `examineRegisterAddress`): a
    register's name in a location is its own place, FFFF0000 + 4n (VMS's numbers, which its
    SYMBOLIZE names), so `@(AP+N)` reads memory at FFFF0034 and testdata/dbg85's long-standing
    `@(AP+N)` difference is gone; DEPOSIT's value and a WHEN's operands are values, a code label
    its instruction's bytes (`DebugLabels.CodeLabel`); `break.dlg`'s WHEN differences are gone, and
    `TestBreakWhenAndDo` writes `COUNT EQL 1`.
  - *Instructions* (`machine/dbgoperands.go`'s `explainOperand`, `disasm.go`): NOTCURPC before
    operands away from the PC, FULL's explanations, `(no read access)`, CASE's 20-displacement
    limit; a register's instruction and the location after it (`registerInstructionAt`).
  - *Eventpoints and steps*: address tracepoints after the classes', a tracepoint at a
    watchpoint's stop, "routine" names (`routineLocation`), `/after` hidden once reached,
    `pendingReturns` (a list by frame), an interrupted STEP/OVER's end kept (`Breakpoint.stepEnd`;
    `TestStepOverEndsAtBreakpoint` now expects VMS's stop), STEP/RETURN's report by its unit.
  - *The rest*: TYPE's NONXTLIN; SHOW LANGUAGE the session's; SET/CANCEL SOURCE/MODULE and VMS's
    SHOW SOURCE (`machine/source.go`'s per-module lists); NOSUCHMODU; EXIT runs exit handlers
    (`runExitHandlers`, a silent `$EXIT`), QUIT doesn't, and in an `@` file of an image session
    either ends the file; the log's echo by VMS's rule (`refusedAsRead`, `notEchoed`, found from
    every probe's logs) and a message's tab-led second line without "!".
  - DEVIATIONS.md's Phase 42 entry is all resolved but multi-dimension bounds; Phase 89's and
    Phase 85's unconfirmed debugger items are marked; HELP's EXIT, QUIT, SOURCE, and Expressions,
    and MANUAL.md's debugger tables.
  - Left: the activator's registers, govax's larger P0 (OPCDEC at 005001D0 where VMS had an
    access violation), and the condition dispatcher at an exception break.
  - Subtask 13g, the FORTRAN rounds, is next.
- **2026-11-25, subtask 13g (done):** the FORTRAN rounds made the oracles' logs. `testdata/ftn86`'s
  round 6 is now `vax/` (round 5 moved to `vax/round5/`) and `testdata/f90r`'s round 3 is now `vax/`
  (round 2 to `vax/round2/`); both READMEs say what came back, and gen.go's copy-outs name the next
  rounds (`round7`, `vax/round4`).
  - *The awaiting lists are empty.* FTN86's 362 sections (RECARRAYS, RECDYN, ZEROVAX, and STRMATH
    now among them, and FTN_ALLOC's MACRO in FTNRTL's) and F90R's 482 (Phase 112's and this phase's
    programs) are govax's but for `ftn86VMSDifferences`: STRMATH joined it, round 6 having run it as
    it was before subtask 13a (its OUTSTAOVE is govax's too; the program now prints the whole field,
    for the next round), and DIRECTIVES left it, round 6 having built the program as govax's
    compiler makes it now. `ftn86NewModules` is empty.
  - *SET VERIFY.* Round 3 of F90R ran with verification on, as every FTN86 round has (round 2 of
    F90R hadn't): `ftn86Lines` now also takes out a procedure's `$!` comments as DCL shows them,
    and `f90rVerifiedData` the data lines DCL showed in ONESHOT as fcalc read them from the
    procedure. govax shows the same lines under SET VERIFY (run by hand on ONESHOT's form: the
    `$!` lines, then each data line before fcalc's own echo of it).
  - *A shareable image's debug symbol table* (`internal/link/shareable.go`'s `withDSTFixups`):
    `TestFTN86Objects`, on round 6's FTNRTL (its sources copied from F90R's round 3, the same
    session's, and checked against the round 6 volume's), found VMS's DST 14 blocks to govax's 13,
    the records the same, VMS's last block all zeros. The fixup records start on a block of their
    own; VMS's LINK reserves as many blocks as both parts' record bytes fill and one more, which is
    what every VMS image of the probes has (FTNRTL's rounds, whose fixup records ran to 718, 1,004,
    and 1,022 bytes, and every smaller shareable image), and govax's gave only the blocks the
    padded layout used, a block less once the fixup records run past a block. govax now reserves
    VMS's; whether VMS rounds the sum up and adds one or rounds it down and adds two, which differ
    only at a multiple of 512, is unconfirmed (govax rounds up). The built-in FTNRTL.EXE was
    remade (`go generate ./internal/ftnrtl`).
  - `ftn86.com`'s copy-out now takes the FTNRTL sources VMS built (`vax/roundN/mar/`), which round 6's
    didn't.
  - Subtask 14, the close-out, is next.
- **2026-11-25, subtask 14a (done):** round 2 of the probe round, written and rehearsed. The author
  offered another VAX run, so the close-out is in three parts (the subtask list): this round, its
  answers, and the documents.
  - *What it asks* (`testdata/err113`'s README, "Round 2"; `ERR113B.COM` on a volume of its own,
    `exchangeb.cmd`, with round 1's scratch disk): LNK113L, a shareable image's `.LONG` of its own
    address (12a's found and left); LNK113D, sixteen shareable images whose DST records total 504
    to 519 bytes on govax (`testdata/err113/gen.go`, a routine's name a character longer each time),
    for 13g's rounding, the test to read the totals from VMS's own images; MAC113F, `4+R1`, `2*R3`,
    and `<R5>` in data (13c); DCL2, a directory left open in the file parameters of eighteen other
    commands (13c); FULL2, round 1's FULL with the free blocks counted, for the block VMS's REST.DAT
    left (13c, 13d); HDRFULL, the index file past its header on a volume whose free space is all
    single blocks (8-block files, every other deleted, 7-block files in the holes, then 1-block
    files), A.9.1's question 13d left open. Its `[FOR81M]` is round 3c of `testdata/for81`
    (`gen.go`'s `round3c`, `avCase`), and FOR81W again, which now prints each long line just
    before its READ for pasting (`prog.say`, `longText`: a descriptor over `.ASCII` pieces, past a
    MACRO line). FTN86's round 7 and F90R's round 4 run again beside it: round 6 predates 13e's
    FTNRTL, and STRMATH is to be run as it is now.
  - *FOR81M on FORRTL over govax* already answered part of 13e2's question, FORRTL's own logic:
    DELETE (u'r) gives r+2 only when the unit's last direct statement was a READ of r; after FIND r
    a DELETE of r-1 leaves r (FTNRTL gave r+1), and after a WRITE of r, a FIND, or a READ of another
    record, r+1. FTNRTL's `ftn_record.mar` follows: a unit byte, `UB_LASTREAD` (UB_ASV+45), set by
    a direct READ's `DIRECT_DONE` and cleared by `SET_NEXT`; `TestFor81MOnFTNRTL` holds FTNRTL to
    FORRTL's run until VMS's is back. The built-in FTNRTL.EXE is remade.
  - *COPY/CONTIGUOUS*: HDRFULL needs a 7-block file to stay in one 8-block hole, and VMS 7.1's COPY
    has the qualifier (the DCL Dictionary), so govax's has it now (`rms.CopyOptions.Contiguous`,
    `createBinaryFile`: no free run long enough is RMS$_FUL at the open, no file left; the output
    marked FCH$V_CONTIG); `TestConsoleCopy_contiguous`, HELP COPY.
  - *The rehearsal*, `TestErr113BProcedure` (`internal/console/err113b_test.go`), runs ERR113B.COM
    with P3 `KEEP` (govax has no INITIALIZE of a device: a scratch volume made as `/HEADERS=16`
    makes it, its files deleted between sections). It changed the procedure three times: VMS 7.1's
    DCL can't WRITE a 512-byte string (the templates are 128-byte FIX records, their allocations by
    FDL), a directory must be contiguous so each phase's is preallocated (`CREATE/DIRECTORY
    /ALLOCATION`), and the templates are made on the scratch volume, whose cluster is a block (the
    round's volume's is 3, which would have made the 7-block files fill the holes).
  - *Found in govax*, for 14b: COPY on a volume whose directory can't grow, and a file on a volume
    with no free header slot, are DCL-E-BADFILESPEC with ods2's words (`FindFreeSlot: no free header
    slot found`), not VMS's conditions; govax's PRINT took `DCL[X.TXT`'s `DCL` for a symbol
    (UNDEFSYM). `TestBatchLaterSectionsOracle` passed 40 runs alone; its flake shows only under the
    whole suite's load.
- **2026-11-25, subtask 14b1 (done):** round 2 ran on VMS 7.1 (`testdata/err113`'s README, "What
  came back from round 2"; FTN86's round 7 and F90R's round 4 didn't run, for a later round).
  - *DCL's file parameters* (DCL2): every command VMS was asked about refused a directory left
    open as CLI$_DIRECT before running, so CREATE (and /FDL), CONVERT, DIRECTORY, DELETE, DUMP,
    BACKUP, TYPE, MACRO, LINK, ANALYZE, LIBRARY, RENAME, SUBMIT, OPEN, RUN, and SET DEFAULT take
    the grammar's `$FILE`; the segment stops at a version's `;`; `@` checks its own
    (`dcl.DirectoryClosed`, the segment left out as VMS's). govax's PRINT is the console's, so its
    case differs (`err113BDifferences`); `TestErr113BProcedure` holds DCL2 to VMS's log.
  - *MACRO's registers in data* (MAC113F): an operator before a register ends the item
    (`exprState.stopAtRegister`; `4+R1` stores 4, `2*R3` 2), and the register is the next item,
    round 1's rule for a later one (DATALSTSYN and a zero; `.ADDRESS` ADRLSTSYNX alone, nothing
    stored), the `!` under the operator (`operatorBefore`); `<R5>` first is DIRSYNX, the `!` under
    the R (`registerGroupAhead`). `TestErr113Macro` holds the listing to VMS's but for the zeros'
    relocatable marks, as MAC113A's.
  - *FOR81M*: VMS's FORRTL gave the nine cases 14a's FTNRTL gives; `TestFor81MOnFTNRTL` and
    `TestFor81MProcedure` hold both to `vax/for81m-vax.txt`.
  - *FOR81W*: a READ of exactly 133 characters at the terminal ended when the buffer filled, with
    no Return, ERRDURREA with RMS$_TNS, as govax's `terminalRecord` does; a Return typed then is the
    next read's empty line, which is what govax's whole lines give. `TestFortranTerminalLongLine`
    has the case.
  - *For 14b2 and 14b3*: LNK113L (NONPICREF, BASDUERRS); LNK113D (VMS's DST blocks: the traceback
    records' blocks, then `ceil((fixup bytes + X) / 512)` for an X from 21 to 294, every VMS image
    in testdata agreeing; a sweep of fixup sizes for round 3 to settle X); FULL2 (the File
    Applications guide's sequential extension, the smaller of twice the multiblock count and 256,
    at least the volume's: 2 blocks here, all or nothing, which explains both rounds' last free
    block); HDRFULL (HEADERFULL when the index file's map area fills, no extension header).
- **2026-11-25, subtask 14b2 (done):** LINK made VMS 7.1's in round 2's LNK113L and LNK113D.
  - *NONPICREF*: in a shareable image, a relocatable address stored by other than `.ADDRESS` (or a
    displacement) is LINK-W-NONPICREF, naming the psect, offset, module, and file (pass 2's
    `nonPICReference`, `machine.where`), after one LINK-I-BASDUERRS, both the module's messages. The
    image is then based where it was linked (`baseImage`, `linker.nonPIC`): VMS's LNK113L kept its
    sections' addresses and the map's `-R`s, unmarked ISD$V_BASED, but cleared PICIMG, made its
    universal symbols absolute, and had no fixups of its own addresses (`.ADDRESS` or DST), no
    relocatable reference synopsis (`module.ownRelocations` taken back), and still a fixup section
    holding its own protection change (`hasFixupSection`). Pass 2's bytes are the same either way.
    `TestErr113BLink` links VMS's object and holds the image byte for byte and the map line for line
    to VMS's; MANUAL.md's LINK section.
  - *The DST's blocks* (`withDSTFixups`): the traceback records' blocks, then
    `ceil((fixup bytes + dstFixupSlack) / 512)`, the slack 255, unconfirmed in 21 to 294, which
    every VMS image in testdata fits; 13g's rule (both parts' bytes together, rounded up, plus a
    block) gave LNK113D08 to 15 a block too many. `TestErr113BDSTBlocks` links the sweep; FTNRTL's and
    fcalc's images are unchanged. Round 3 is to sweep fixup records from about 730 to 1,010 bytes
    to settle the slack.

