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
   prompt without `-` continuation (Phase 86). **Proposed: 112**, small, both.
2. COPY's `%COPY-S-COPIED` without `/LOG` (Phases 83, 87, 91; `dclcmd/copy.go` still prints it
   by default); TYPE ending a line after a record with no carriage control
   (Phase 83); STR$MATCH_WILD with `%` (Phase 83). **Proposed: 112** for COPY (VMS says
   nothing; every log that shows it would change: BUILDFTN.COM's and the tests'); TYPE
   **probe**; STR$MATCH_WILD **probe**.
3. DIRECTORY/TOTAL: **resolved** in Phase 91 (`rms/directory.go`'s `Total`; marked in Phase
   88's doc). ods2's null type (`rms.ExplicitNullType`, Phase 88): **not planned** (it works;
   moving it into ods2 is tidiness).
4. *(New.)* DEVIATIONS-RESOLVED.md holds copies of 29 entries DEVIATIONS.md still has open,
   from Phase 32's to Phase 71's (`[Phase 35] An octaword in R12 or above`, the Phase 42 to 51
   choices, `[Phase 71] Process-permanent files`, the template `[Phase NN]`), their statuses
   still open or deferred: copied over in bulk at some time, not resolved. **Proposed: 112**:
   take the copies whose text is the same out of DEVIATIONS-RESOLVED.md, and compare the
   others first. (A 30th, the console's `SET name=value`, really was resolved, in Phase 50:
   this survey took it out of DEVIATIONS.md and marked the copy.)
5. *(New, not Fortran's, recorded for completeness.)* Phase 87's work volume lock and Windows
   locking, SAVE/ROM to a volume, `vax.library` after the volume's library; Phase 91's default
   name's search list; Phase 94's RUN/DETACH and INSTALL's other subcommands; Phase 95's
   remote logins and AUTHORIZE's unkept fields; Phase 96's `--mount` of a RAM disk; Phase
   97's EDIT and SET SHARE_PREFIX; Phase 98's print queues and LIB$GETQUI; Phase 90's grammar
   leftovers (TEST, SHOW EXPAND, SHOW MAPS, SET UIQUANTUM, CANCEL MEMORY with no handlers).
   **Not this phase**: each stays in its phase's "Where things stand". The grammar leftovers
   could go here as a cleanup if the author likes (question 10).

## A.12 The probe round, if there is one

Gathered from the **probe** places above, for one simh session after this phase's
implementation (so the round also checks what this phase does): the index file extension
header (A.9.1); `ftn86.com` again (A.9.5); RECURAST with a longer timer (A.9.3); STR$'s
arithmetic (`testdata/str113`, written in subtask 7) and MATCH_WILD with `%` (A.2.1, A.11.2); a FORRTL round on Phase 80's and 81's
choices and Fortran 95's zero widths on output, which FORRTL may not take (A.10, D.7); MACRO's
`.LONG R1`, `<A;B>`, `.ERROR`/`.WARN`, `A+B` numbering, and an object with errors (A.6); a
debugger round (A.8.6); TYPE after a record with no carriage control (A.11.2); `CONVERT=`'s
answers, if A.1.1 goes ahead. *(Since the refresh, A.15.3: Phase 112's programs awaiting VMS,
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
   11, along with any others that subtask's own inventory finds.

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
  specifiers' `later`, and `io.go`'s default case have no entries left that reach them.

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
10. The SHARE$LIBRTL fault (A.9.2), after subtask 6.
11. Housekeeping (A.11.1, 2, 4, 5; A.15.2; A.16's dead hooks).
12. The probe round (A.12, A.15.3): written, rehearsed on govax, run on VMS 7.1.
13. Its fixes, the ods2 header among them.
14. The close-out.

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
