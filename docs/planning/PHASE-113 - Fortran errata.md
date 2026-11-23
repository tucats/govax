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
   left.
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
   **as needed** (Phase 83's Decision 6).
4. The last bits of OTS$ negative powers and MTHRTL's complex functions against VMS's (Phase
   86; `ftn86VMSDifferences`'s POWERS, POWBITS, MATHFNS). **Measured**: six orders of
   operations tried, none fits; a probe with more cases could find VMS's method (**probe**,
   low priority).
5. FTNRTL's per-statement paths, about 170 instructions a record; `astrace` parsing formats
   80,000 times (Phase 84, subtask 11). **Proposed: 112** to look at the 80,000 parses (a
   FORMAT's cache that misses is a bug, not a speed); the paths themselves **measured**.
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
     8), so the machinery is there. **Proposed: 112.**
   - A `BACK=` that is an array (`sema/charfuncs.go`): an elemental argument. **Proposed: 112.**
   - COMPLEX(16) to SUM, PRODUCT, and their kin (`complex32Intrinsic`, three sites).
     **Proposed: 112.**
   - A component or function value of CHARACTER length zero (`decl.go`'s "a CHARACTER length
     of zero here"). **Proposed: 112.**
   - A statement function of a derived type (`decl.go`). **Proposed: 112**, small.
   - TRANSFER to a CHARACTER MOLD of zero or run-time length, or of such an array
     (`sema/transfer.go`). **Proposed: 112.**
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
   text resolved in Phase 112, subtask 5; a STRUCTURE with initial values still isn't written.)*
3. govax's definition modules (`$SSDEF` and the rest) as module files to USE (Phase 106,
   "later, if wanted"). **Proposed: 112**, small (`sysdef` already writes them as text;
   `USE $SSDEF` is not a legal name, so the spelling is question 8).
4. **Noticed, not changed**: NOTRECORD's wording for `F(A)%N`; `Y(D(1)%N)` in a bound
   BADBOUND (Phase 107); EXPONENT and FRACTION faulting on a reserved operand in their MOVF;
   TRANSFER to a derived type never a constant (Phase 108). **Proposed: 112**, each small.
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
   Phase 112's subtask 8; the construct is its subtask 9's.)*
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
4. CPU_TIME (a subroutine; `$GETJPI`'s CPUTIM in FTNRTL). **Proposed: 112.**
5. A WHERE inside a WHERE, and ELSEWHERE with a mask (F95ONLY, `sema/where.go`, three sites).
   **Proposed: 112.**
6. MAXLOC's and MINLOC's `DIM=` (KEYWORDNAME today), CEILING's and FLOOR's `KIND=`
   (KEYWORDNAME today), and Fortran 95's other intrinsic changes: SIGN of a negative zero (no
   such thing on the VAX: nothing to do), `KIND=` on others the HP manual lists, and NULL's
   MOLD=. **Proposed: 112.**
7. Zero widths in output, `I0`, `B0`, `O0`, `Z0`, `F0.d` (OUTCONERR today, FTNRTL's
   `ftn_edit.mar` and `ftn_float.mar`), under the F90 flag only. **Proposed: 112.**
8. `/STANDARD=F95` beside `F77` and `F90`: F95's additions not flagged under it, the deleted
   features (a REAL DO variable, branching to END IF from outside, PAUSE, ASSIGN and assigned
   GO TO, the H edit descriptor) flagged as deleted, Fortran 95's obsolescent list. Whether
   `/F90` units are judged by F95 by default is question 9. **Proposed: 112.**
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
   both ways (`UNIT.$LINEn`, `showFortranLine`).
3. A component after subscripts in an expression (`EVALUATE P(2)%X + 1`; EXAMINE takes it;
   Phase 110). **Proposed: 112.**
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
arithmetic and MATCH_WILD with `%` (A.2.1, A.11.2); a FORRTL round on Phase 80's and 81's
choices and Fortran 95's zero widths on output, which FORRTL may not take (A.10, D.7); MACRO's
`.LONG R1`, `<A;B>`, `.ERROR`/`.WARN`, `A+B` numbering, and an object with errors (A.6); a
debugger round (A.8.6); TYPE after a record with no carriage control (A.11.2); `CONVERT=`'s
answers, if A.1.1 goes ahead.

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
  length); A.3.3, `USE STARLET_SSDEF` and the rest; A.3.4's noticed items.
- **The compiler**: A.1.4's two forms for `/STANDARD`, A.1.5 (a typed intrinsic's cell),
  A.1.10 (floating short literals).
- **The libraries**: STR$'s arithmetic (A.2.1); LIB$INITIALIZE (A.2.2): LINK building the
  image's initialization list and the activator calling it, as VMS does, with probe cases;
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

1. **The survey** (done 2026-11-22): Part A; the author's answers the same day.
2. Fortran 90's NOTIMPLs and noticed items (A.3.1, A.3.4).
3. The compiler's small things and `STARLET_` modules (A.1.4, A.1.5, A.1.10, A.3.3).
4. MACRO (A.6.1, 2, 5).
5. LINK and LIB$INITIALIZE (A.7.1, A.7.3, A.2.2).
6. STR$'s arithmetic (A.2.1).
7. Services and FTNRTL's format parses (A.2.3, A.2.5).
8. The debugger (A.8.2, 3).
9. The SHARE$LIBRTL fault (A.9.2).
10. Housekeeping (A.11.1, 2, 4, 5).
11. The probe round (A.12): written, rehearsed on govax, run on VMS 7.1.
12. Its fixes, the ods2 header among them.
13. The close-out.

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
