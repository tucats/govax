# Phase 100 — Fortran 90: declarations and kinds

**Status:** done (2026-11-17). Planned 2026-11-17 with Phase 99
(`PHASE-99 - Fortran 90.md`, Part C). Subtask 1, the survey (Part A, below,
with a revised subtask list and questions for the author), was accepted by
the author ("The author's answers"); subtasks 2 to 9 (the scope chain, `::`
declarations and attributes, kind selectors and kinds on constants,
INTEGER(1), the kind and inquiry functions, COMPLEX(16), programs end to
end, and the close-out) are done (2026-11-17). "Where things stand", at the
end, is what the phase left. Needs Phase 99.

## Goal

Fortran 90's way of declaring things, with every type's **kind**:

- type declarations with `::`, an attribute list, and initialization:
  `REAL(KIND=8), DIMENSION(10), SAVE :: X = 0.0_8`,
  `INTEGER, PARAMETER :: N = 100`, `CHARACTER(LEN=*), PARAMETER :: NAME = 'X'`;
- the type selectors: `INTEGER(2)`, `REAL(KIND=8)`, `LOGICAL(1)`,
  `COMPLEX(KIND(0.0D0))`, `CHARACTER(LEN=10)`, `CHARACTER(10)`,
  `CHARACTER(*)`, `CHARACTER(LEN=N, KIND=1)`; DEC's `*n` lengths go on working
  beside them;
- kinds on constants (`1.5_8`, `7_2`, `.TRUE._1`, `1_'text'`) and kinds
  named by PARAMETERs (`INTEGER, PARAMETER :: DP = KIND(1.0D0)`; `1.0_DP`);
- the kind functions, folded at compile time: KIND, SELECTED_INT_KIND,
  SELECTED_REAL_KIND; and the numeric model's inquiries: HUGE, TINY, EPSILON,
  DIGITS, PRECISION, RANGE, RADIX, MAXEXPONENT, MINEXPONENT, BIT_SIZE (the
  standard's 13.7.1, with the VAX formats' parameters);
- the attributes, as attributes and as statements: PARAMETER, DIMENSION,
  SAVE, EXTERNAL, INTRINSIC, and DEC's VOLATILE, AUTOMATIC, STATIC; INTENT,
  OPTIONAL, PUBLIC, PRIVATE, POINTER, TARGET, ALLOCATABLE are parsed and
  checked for consistency here and compiled by their phases (103, 106, 107,
  104);
- IMPLICIT with kinds (`IMPLICIT REAL(8) (A-H, O-Z)`) and the parser's way of
  telling `REAL(8) (A-H)` from a letter list;
- sema's **scope chain** (Decision 4): every unit's table becomes a scope with
  a host pointer, the host nil until Phase 103; `lookup` is one function.

## What earlier phases leave in place

- `sema.Type{Kind, Size}`: the size is the kind number (Decision 1), so a kind is
  already representable; `sema/types.go`, `implicit.go`, `parameter.go`,
  `constant.go` (constant expressions in every type), `attributes.go`, `data.go`
  (initial values, which `::`'s initialization reuses).
- `vaxfloat`'s formats, for the model's parameters (F: 24 bits, exponent -127 to
  127; D: 56 bits; G: 53 bits, -1023 to 1023; H: 113 bits, -16383 to 16383).

## Design notes

- **Kinds are byte sizes** (the HP compiler's numbering): INTEGER 1, 2, 4;
  LOGICAL 1, 2, 4; REAL 4, 8, 16; COMPLEX 4, 8, 16 (the kind of its parts);
  CHARACTER 1. REAL(8) is D or G by `/G_FLOATING`, as REAL*8 is.
  SELECTED_REAL_KIND(p, r) returns the smallest kind with precision p and
  range r; with `/G_FLOATING` off, D's range (10^38) means a request for range
  above 38 needs kind 16 (H), not 8: noted in MANUAL.md.
- An initialized entity has SAVE (the standard's 5.1), which matters only under
  `/RECURSIVE` (Phase 89's storage rule already keeps initialized locals static).
- A `::` declaration's attribute list is checked against the standard's 5.1
  constraints in one table (which attributes may go together), not scattered
  conditions.
- `/NOI4` still makes the default INTEGER kind 2; `KIND(0)` follows it.

## Open questions

1. `REAL*8` and `REAL(8)` are the same type; does `DOUBLE PRECISION` stay
   `REAL(8)` under `/G_FLOATING` (yes, as today)?
2. What does `SELECTED_REAL_KIND` return when only H fits but the program asks
   for a range D can't hold and G could (`/NOG_FLOATING`)? The plan: 16, the
   only kind that fits under the unit's formats.

# Part A — the survey (subtask 1)

Read for it: the standard's 4.3 (intrinsic types, kinds, literal constants),
5.1 and 5.2 (R501 to R549: type declarations, attributes, attribute
statements), 5.3 (IMPLICIT), 7.1.4.2 (the type and kind of a mixed
operation), 7.1.6.1 (initialization expressions), 13.7.1 (the numeric
models), and 13.13's entries for the kind and inquiry functions; the HP
manual's kind numbers (its 3.x and 5.x); and what sema, the parser, the IR,
and FTNRTL do today.

## A.1 What the standard asks

- **R501 to R509.** `type-spec [[, attr-spec]... ::] entity-decl-list`. The
  `::` is required when an attribute or an `= initialization-expr` is
  written, optional otherwise. An entity is `name [(array-spec)] [*char-length]
  [= initialization-expr]`. The type specs are INTEGER, REAL, COMPLEX, LOGICAL
  with an optional `([KIND=] expr)`, DOUBLE PRECISION with none, CHARACTER with
  R506's four orders of `LEN=` and `KIND=` or a `*char-length` (the comma after
  `CHARACTER*10,` only without `::`), and `TYPE(name)` (Phase 105). A kind must
  be a nonnegative scalar integer initialization expression naming a kind the
  processor has.
- **5.1's constraints** are a table in all but name: which attributes may go
  together (POINTER not with TARGET, INTENT, EXTERNAL, INTRINSIC; TARGET not
  with POINTER, EXTERNAL, INTRINSIC, PARAMETER; ALLOCATABLE not with POINTER;
  EXTERNAL not with INTRINSIC), which entities may have each (PARAMETER not on
  dummies, pointers, allocatables, functions, COMMON objects; INTENT and
  OPTIONAL only on dummies; SAVE not on COMMON objects, dummies, procedures,
  results, automatic objects; PUBLIC and PRIVATE only in a module), and which
  may be initialized (not dummies, results, COMMON objects outside BLOCK DATA,
  blank COMMON, allocatables, pointers, externals, intrinsics, automatic
  objects). No attribute twice for one entity in a scoping unit, in one
  statement or across several.
- **Initialization** is intrinsic assignment of the expression's value: a
  scalar initializes every element of an array (7.5.1.4); it implies SAVE but
  in a named COMMON block.
- **Initialization expressions** (7.1.6.1) are constants, constructors,
  elemental intrinsics of integer or character type on such arguments,
  SELECTED_INT_KIND, SELECTED_REAL_KIND, REPEAT, RESHAPE, TRANSFER, TRIM, and
  the inquiry functions (KIND, LEN, BIT_SIZE, the numeric ones, the array ones
  but ALLOCATED) on a variable whose inquired-about properties are constant. An
  inquiry's argument need not have a value: `HUGE(X)` of any REAL X is a
  constant.
- **5.2's attribute statements** (DIMENSION, SAVE, EXTERNAL, INTRINSIC,
  PARAMETER, and the new ALLOCATABLE, INTENT, OPTIONAL, POINTER, TARGET,
  PUBLIC, PRIVATE) give the same attributes, under the same constraints;
  DIMENSION, SAVE, EXTERNAL, INTRINSIC, ALLOCATABLE, POINTER, TARGET, PUBLIC,
  PRIVATE, INTENT, and OPTIONAL take an optional `::`.
- **IMPLICIT** (5.3) takes any type spec, so `IMPLICIT REAL(8) (A-H)` and
  `IMPLICIT CHARACTER(LEN=10) (C)`; the parser's `kindBeforeLetters` already
  tells the two parentheses apart.
- **Kinds on constants** (4.3): `digits_kind` for integers, a real's kind only
  with an `E` exponent or none (`1.0D0_8` is an error), logicals'
  `.TRUE._kind`, and a character constant's kind *before* it (`1_'text'`). A
  kind written as a name must be a named integer constant.
- **Every real kind has a complex kind** (4.3.1.3): "Each approximation method
  used to represent data entities of type real must be available for both the
  real and imaginary parts" of a complex. A COMPLEX's kind is its parts' kind.
- **Mixed operations** (7.1.4.2): two integers of different kinds give the
  kind with the larger range, two reals the one with the greater precision, an
  integer and a real the real's kind; a complex and a real the higher
  precision of the two.
- **The numeric models** (13.7.1): an integer has r = 2 and q digits; a real
  b = 2, p digits, and exponents emin to emax, the
  fraction 0.f with f1 nonzero. Each VAX format is such a model
  exactly (the hidden bit makes the fraction 0.1f, and the exponent field's
  bias is 128 in F and D, 1024 in G, 16384 in H), so the inquiry functions
  have exact answers:

  | Kind | DIGITS | MIN/MAXEXPONENT | HUGE | TINY | EPSILON | PRECISION | RANGE |
  |---|---|---|---|---|---|---|---|
  | INTEGER(1) | 7 | | 127 | | | | 2 |
  | INTEGER(2) | 15 | | 32767 | | | | 4 |
  | INTEGER(4) | 31 | | 2147483647 | | | | 9 |
  | REAL(4), F | 24 | -127, 127 | 1.70141173E38 | 2.93873588E-39 | 1.19209290E-7 | 6 | 38 |
  | REAL(8), D | 56 | -127, 127 | 1.70141183E38 | 2.93873588E-39 | 2.77555756E-17 | 16 | 38 |
  | REAL(8), G | 53 | -1023, 1023 | 8.98846567E307 | 5.56268465E-309 | 2.22044605E-16 | 15 | 307 |
  | REAL(16), H | 113 | -16383, 16383 | about 5.9E4931 | about 8.4E-4933 | 1.92592994E-34 | 33 | 4931 |

  RADIX is 2 for all, BIT_SIZE 8, 16, and 32. HUGE is (1 - 2**(-p)) ×
  2**emax, TINY 2**(emin-1), EPSILON
  2**(1-p), PRECISION INT((p-1) × LOG10(2)), RANGE INT(MIN(LOG10(HUGE),
  -LOG10(TINY))). Each is to be computed with `vaxfloat`, not typed in: the
  table is the test's expectation.
- **SELECTED_REAL_KIND(P, R)** (13.13.93) is the kind of least precision
  meeting both, the smallest such kind if several, else -1 (precision), -2
  (range), or -3 (both). With the table above, under `/NOG_FLOATING` P ≤ 6 and
  R ≤ 38 give 4, P ≤ 16 and R ≤ 38 give 8, P ≤ 33 and R ≤ 4931 give 16;
  under `/G_FLOATING` the middle row is P ≤ 15 and R ≤ 307. SELECTED_INT_KIND(R)
  is 1, 2, or 4 for R up to 2, 4, or 9, else -1. Its arguments are optional and
  may be given by keyword (`SELECTED_REAL_KIND(R=100)`).

## A.2 What govax has

- **Types:** `sema.Type{Kind, Size}` (`types.go`) has every VAX FORTRAN type:
  INTEGER 2 and 4, LOGICAL 1, 2, 4, REAL 4, 8, 16, COMPLEX 8 and 16 (bytes, the
  whole value), CHARACTER 1 to 65535 or 0 for `(*)`. `lengths` and `fixedTypes`
  (`decl.go`) are the allowed lengths; `typeOf` and `characterType` turn a
  `*n` into a Type. `rank`, `higher`, and `operationType` are 2.3.1.2's mixed
  rules, with logicals "treated as integers" and LOGICAL\*1 as INTEGER\*2.
- **The symbol table:** one per unit, `Unit.byName` and `Unit.Symbols`;
  `analyzer.lookup` enters a name with its implicit type, `declare` is lookup
  plus a cross reference note. 33 reads of `a.unit.byName[...]` that must not
  enter a name are spread over 19 files; statement functions swap their
  dummies into and out of `byName` (`stmtfunc.go`); `Unit.Lookup` is the
  exported read the IR and the debugger table use. COMMON blocks, namelists,
  and structures have tables of their own (the standard's other classes of
  local entities).
- **Declarations:** `declarations` (`decl.go`) walks the specification
  statements; `typeStatement` takes DEC's `/clist/` initial values into
  `data.go`'s `Initial`s; `parameter.go` gives PARAMETER names their values,
  converted to the name's type; `attributes.go` and `storage.go` hold SAVE,
  EXTERNAL, INTRINSIC, VOLATILE, AUTOMATIC, and STATIC, each statement's
  checks written where it is read.
- **Constant expressions** (`constant.go`, `constfuncs.go`): every type, and
  MIN, MAX, ABS, MOD, DIM, ICHAR, NINT, DPROD, CMPLX, CONJG, AIMAG, the bit
  functions, the lexical comparisons, and CHAR, folded with `vaxfloat`.
- **The parser** makes a `PendingStmt` (NOTIMPL) of every Fortran 90 form this
  phase compiles: `::` in a type or attribute statement (`f90Declaration`,
  `parseDimension`, `parseAttr`), a kind or length selector after a type
  keyword, in IMPLICIT, and before FUNCTION; and the lexer's ALLOCATABLE,
  INTENT, OPTIONAL, POINTER, PUBLIC, PRIVATE, TARGET statements. A kind on a
  constant is `ast.KindConst`, which sema reports NOTIMPL.
- **Below sema:** the IR has a `Byte` integer type (BISB3's B) and codegen
  writes its instructions by suffix; FTNRTL's I editing and list-directed
  output already take `DSC$K_DTYPE_B`. None of them has a 32-byte complex.

## A.3 Findings, and what they change

1. **INTEGER(1) is a new type.** VAX FORTRAN has no INTEGER\*1: BYTE is
   LOGICAL\*1, and its arithmetic is done as INTEGER\*2 (Table 2-1; the HP
   manual makes BYTE INTEGER(1), but nothing old may change, so BYTE stays
   LOGICAL\*1). Phase 99's Decision 1 gives INTEGER kinds 1, 2, and 4, so
   `{Integer, 1}` joins `lengths`, `rank` (below INTEGER\*2), the constant
   folder (its range), the IR (`Byte` arithmetic: the VAX has ADDB, SUBB, MULB,
   DIVB, CMPB, CVTBL, CVTLB), conversions, and I/O (DTYPE_B, which FTNRTL
   reads). An intrinsic of an INTEGER(1) argument is done in INTEGER(2) and
   converted back, the result INTEGER(1) as 13.13 says (the generic table has
   no byte rows; ours). `INTEGER*1` in old form comes with it, as the HP manual
   allows. A subtask of its own (below).
2. **A COMPLEX's kind is not its size.** By the HP numbering and 4.3.1.3,
   COMPLEX(4) is COMPLEX\*8 and COMPLEX(8) is COMPLEX\*16: the kind is the
   parts' size. Decision 1's "the size is the kind number" holds for every
   type but COMPLEX, whose kind is `Size/2`; `Type.KindNumber()` says so in one
   place, and DOUBLE COMPLEX is COMPLEX(8).
3. **COMPLEX(16), H complex, is required by the standard** (4.3.1.3) and is
   new: VAX FORTRAN has no COMPLEX\*32, MTHRTL and OTS$ have no H complex
   routines, and FTNRTL no H complex editing. Question 1 asks how far to go.
4. **Zero-length character entities.** Fortran 90 allows `LEN=0` and a negative
   length means 0 (5.1.1.5); VAX FORTRAN's lengths are 1 to 65535, and the IR,
   descriptors, and FTNRTL assume at least 1. Zero lengths belong with Phase
   108's character results of run-time length; until then `LEN=0` is NOTIMPL,
   "a CHARACTER length of zero" (question 3).
5. **A length or bound that isn't constant** in a non-dummy's declaration is an
   automatic object (Phase 104, Decision 8). A dummy's nonconstant LEN
   (`CHARACTER(LEN=N) :: S`, N a dummy) is not FORTRAN-77's and needs the
   same entry-time computation; both stay NOTIMPL here, named, for Phase 104.
6. **The new intrinsic names under `/NOF90`.** KIND, SELECTED_INT_KIND,
   SELECTED_REAL_KIND, HUGE, TINY, EPSILON, DIGITS, PRECISION, RANGE, RADIX,
   MAXEXPONENT, MINEXPONENT, and BIT_SIZE are not in Table D-3, so by Decision 2
   they are intrinsic only under `/F90` or by an INTRINSIC statement. A
   fixed-form program can still write `REAL(KIND=8)` and `1.0_8` (new syntax,
   always accepted), but `KIND(1.0D0)` in it is a call of its own KIND unless
   declared INTRINSIC, and then not a constant: KINDNOTCONST says why.
7. **KIND= on the conversion functions.** INT, REAL, CMPLX, NINT, AINT, ANINT,
   and the new LOGICAL take an optional KIND= (13.13). They are how a program
   uses kinds once it has them (`REAL(N, KIND=DP)`), and they are small rows in
   the intrinsic table; this survey proposes moving them here from Phase 108.
8. **Keyword arguments to intrinsics start here.** `SELECTED_REAL_KIND(R=40)`
   and `INT(X, KIND=2)` need Decision 14's new columns (the dummies' names,
   which are optional). Subtask 6 adds the columns and fills them for this
   phase's functions; Phase 108 fills the rest. Keyword arguments to a
   program's own procedures stay Phase 103's.
9. **`SELECTED_REAL_KIND(15, 307)`**, the common idiom for "double", gives 16
   (H) under `/NOG_FLOATING`, since D's range is 38; under `/G_FLOATING` it
   gives 8. A portable program runs either way, slower in H. This is the plan's
   open question 2 in practice: the answer stands (16, the only kind that
   fits), and MANUAL.md recommends `/G_FLOATING` for such programs.
10. **Array initialization:** a scalar `= value` initializes every element
    (`REAL :: A(100) = 0.0`), through `data.go` as a repeat count. An array
    constructor as the value, and array-valued PARAMETERs, are Phase 102's
    (NOTIMPL, named).
11. **The attributes' fate in this phase.** The constraints are checked for
    all of them now, in one table (`sema/attrtable.go`): a row per attribute
    with what it may not join, what entities may have it, and whether it is
    compiled. Proposed:

    | Attribute | In this phase |
    |---|---|
    | PARAMETER, DIMENSION, SAVE, EXTERNAL, INTRINSIC, VOLATILE, AUTOMATIC, STATIC | compiled, as their old statements are |
    | INTENT | recorded and compiled as nothing (a dummy is passed as before); Phase 103 checks what a unit does with it |
    | OPTIONAL | recorded; a dummy that is always passed works today, and an absent argument needs Phase 103's explicit interfaces and PRESENT |
    | TARGET | recorded; it means something only beside pointers (Phase 107) |
    | POINTER, ALLOCATABLE | NOTIMPL by name (Phases 107 and 104) |
    | PUBLIC, PRIVATE | an error outside a module (R510's constraint); modules are NOTIMPL until Phase 106 |

12. **`/clist/` and `=`.** DEC's `/clist/` initial values stay legal without
    `::`; with `::` only `=` is (HP's rule, and R504's). `X /1.0/` after a
    `::` is a syntax error saying so.
13. **The scope chain** (Decision 4) as the refactoring: a `scope` type
    (`sema/scope.go`) holds `byName`, `Symbols`, the implicit rules, and `host`
    (nil until Phase 103); `scope.find` is the read that doesn't enter a name
    (the 33 reads), `analyzer.lookup` the one that does, searching the chain;
    `Unit` keeps `Lookup` for its callers. Statement functions' dummies become a
    scope of their own, the statement function's, whose host is the unit, in
    place of the swap: the standard's own description of them (12.5.4) and the
    first use of the chain. The other classes (COMMON blocks, namelists,
    structures) stay in the unit: they aren't host-associated. No output
    changes, which `TestNoF90Unchanged` and the golden dumps check.
14. **Nothing changes under `/STANDARD`** this phase; Phase 110 judges `::`,
    kinds, and the attributes.

## A.4 The subtasks, revised

The provisional list gains INTEGER(1) and the conversions' KIND= (findings 1
and 7), and COMPLEX(16) by question 1's answer.

1. **Survey** (this Part A) (done 2026-11-17; accepted by the author the
   same day).
2. **The scope chain** in sema (finding 13): a refactoring, no change in what
   is compiled; every old test passes and `TestNoF90Unchanged` holds (done
   2026-11-17).
3. **`::` declarations and attributes:** the parser's entity lists and
   attribute specs, the attribute statements in both forms, sema's attribute
   table (finding 11), `= initialization` through `data.go` with scalars
   broadcast (finding 10), implied SAVE, PARAMETER with `::`, `CHARACTER(*)`
   PARAMETERs (done 2026-11-17).
4. **Kind selectors and kinds on constants:** every type spec of R502 and
   R506, `Type.KindNumber`, `ast.KindConst` compiled, kinds named by
   PARAMETERs, IMPLICIT with kinds, FUNCTION with a kind selector, and the
   messages for a kind the processor lacks (done 2026-11-17).
5. **INTEGER(1)** (finding 1): sema's type and rules, the IR's and codegen's
   byte arithmetic and conversions, intrinsics through INTEGER(2), I/O, the
   debugger table's type; `INTEGER*1` (done 2026-11-17).
6. **The kind and inquiry functions**, folded (A.1's table, both
   `/G_FLOATING` settings, `/NOI4`), and **KIND= on the conversions**
   (finding 7), with the intrinsic table's keyword and optional columns
   (finding 8) (done 2026-11-17).
7. **COMPLEX(16)**, as far as question 1's answer takes it (done
   2026-11-17).
8. **Programs end to end** (`testdata/fortran90`: declarations of every form, a
   program portable to gfortran using `SELECTED_REAL_KIND` and `KIND=`) (done
   2026-11-17).
9. **Close-out**: FORTRAN-INTERNALS.md's section 25 (scopes and kinds),
   MANUAL.md's 14.13, HELP FORTRAN (done 2026-11-17).

## A.5 Questions for the author

1. **COMPLEX(16).** The standard requires a complex kind for every real kind,
   so H complex should exist; VAX FORTRAN, MTHRTL, and FTNRTL have none. (a)
   **Declarations, moves, `+`, `-`, `*`, comparisons, and conversions in line
   (the parts are H values, which the VAX has instructions for), `/` by a new
   FTNRTL routine, I/O and the complex intrinsics (ABS, SQRT, EXP, ...) NOTIMPL
   until Phases 108 and 109 take them**; (b) all of that here, I/O and
   intrinsics too (new FTNRTL and MTHRTL-style routines); or (c) refuse
   `COMPLEX(16)` as a kind this processor lacks, a documented deviation.
   **Recommendation:** (a): it keeps the standard's promise for arithmetic at
   modest cost, and the rest lands with the phases that build those routines.
2. **INTEGER(1)** as a full type in this phase (finding 1), with `INTEGER*1`
   too, and BYTE left LOGICAL\*1? **Recommendation:** yes.
3. **Zero-length CHARACTER** (finding 4): NOTIMPL until Phase 108?
   **Recommendation:** yes; it touches descriptors, the IR's moves, and FTNRTL,
   and Phase 108 builds character values of run-time length anyway.
4. **KIND= on INT, REAL, CMPLX, NINT, AINT, ANINT, LOGICAL** moved into this
   phase from Phase 108 (finding 7)? **Recommendation:** yes.
5. **INTENT, OPTIONAL, and TARGET recorded and compiled as nothing** until
   their phases (finding 11), rather than NOTIMPL? A program written for
   Fortran 90 nearly always says INTENT. **Recommendation:** yes.
6. **The plan's open questions:** DOUBLE PRECISION stays REAL(8), D or G by
   `/G_FLOATING` (open question 1), and SELECTED_REAL_KIND gives 16 when only H
   fits the unit's formats (open question 2, finding 9)? **Recommendation:**
   both as planned.

## The author's answers (2026-11-17)

The author accepted every recommendation:

1. **COMPLEX(16)** as option (a): declarations, moves, `+`, `-`, `*`,
   comparisons, and conversions in line, `/` by a new FTNRTL routine; its I/O
   and the complex intrinsics NOTIMPL until Phases 108 and 109.
2. **INTEGER(1)** is a full type in this phase, with `INTEGER*1`; BYTE stays
   LOGICAL\*1.
3. **Zero-length CHARACTER** is NOTIMPL until Phase 108.
4. **KIND= on INT, REAL, CMPLX, NINT, AINT, ANINT, and LOGICAL** moves here
   from Phase 108 (subtask 6).
5. **INTENT, OPTIONAL, and TARGET** are recorded and compiled as nothing until
   their phases; POINTER and ALLOCATABLE are NOTIMPL.
6. **The plan's open questions:** DOUBLE PRECISION stays REAL(8), D or G by
   `/G_FLOATING`; SELECTED_REAL_KIND gives 16 when only H fits the unit's
   formats.

## Progress log

- **2026-11-17:** Planned with Phase 99.
- **2026-11-17, subtask 1 (written, for review):** the survey, Part A: the
  standard's rules for declarations, attributes, initialization, kinds, and
  the numeric models, with each VAX format's model parameters computed (A.1's
  table); what sema, the parser, the IR, and FTNRTL have (A.2); fourteen
  findings (A.3), among them that INTEGER(1) and COMPLEX(16) are new types,
  that a COMPLEX's kind is half its size, and that the scope chain can make a
  statement function a scope; the subtasks revised (A.4) and six questions
  (A.5).
- **2026-11-17:** The author reviewed the survey and accepted every
  recommendation ("The author's answers"). Subtask 1 is done; subtask 2, the
  scope chain, is next.
- **2026-11-17, subtask 2 (done):** the scope chain. `sema/scope.go`'s
  `scope` holds a scoping unit's names (`byName`), the list it keeps them
  in (a unit's is `Unit.Symbols`), its implicit rules, and its `host`;
  `find` looks a name up the chain and enters nothing (the 33 reads of
  `a.unit.byName` became `a.scope.find`), `enter` puts a new name in the
  nearest scope that keeps a list, `define` makes a name a scope's own, and
  `rules` is the nearest implicit rules (the analyzer's `implicit` field is
  gone). `Unit.byName` became `Unit.scope`, `Unit.Lookup` its `find`; the
  analyzer's `scope` is the one being read. A statement function's
  definition is now a scope of its own (`newInnerScope`, host the unit's),
  replacing the swap of its dummies into and out of the unit's table; each
  dummy is defined as it is read, so a name given twice finds the first, as
  the swap did. COMMON blocks, namelists, and structures stay the unit's.
  Nothing compiled changed: the `internal/fortran` packages (golden dumps,
  `TestNoF90Unchanged`), `dclcmd`, and the console's FORTRAN, Fortran 90, and
  DUNGEON tests pass; `TestScopeChain` checks the chain itself. sema's
  `doc.go` and FORTRAN-INTERNALS.md's statement function section say so.
- **2026-11-17, subtask 3 (done):** `::` declarations and attributes.
  The parser: a type statement with a `::` (`TypeStmt.Colons`) reads its
  attribute list (`attrSpec`: a keyword from `parse/f90.go`'s `attributes`,
  DIMENSION's bounds, INTENT's spec, IN OUT in free form; BADATTR) and each
  entity's `= value` (`entityValue`, `Declarator.Value`); DEC's `/clist/`
  after `::` is SLASHINIT, and an array constructor as the value NOTIMPL by
  name. DIMENSION and every attribute statement take an optional `::`
  (`DimensionStmt.Colons`, `AttrStmt.Colons`); INTENT, OPTIONAL, and TARGET
  statements are `AttrStmt`s (`AttrStmt.Intent`; TARGET's items may have
  bounds, as AUTOMATIC's); ALLOCATABLE, POINTER, PUBLIC, and PRIVATE stay
  PendingStmts. A bound with a colon and no upper bound, `(:)` or `(lb:)`, is
  read (`Bound.Colon`) and is NOTIMPL in sema, "an assumed-shape or
  deferred-shape array". A kind or length selector is still the
  PendingStmt it was (subtask 4). Sema: `sema/attrtable.go` is finding 11's
  table, a row per attribute with the attributes it excludes and its fate
  (compiled, recorded, NOTIMPL, module only); `attrSpecs` reads a list once
  for the statement (ATTRTWICE, ATTRPAIR, NOTIMPL for POINTER and
  ALLOCATABLE, NOTINMODULE for PUBLIC and PRIVATE), and `giveAttributes` gives
  each entity each attribute as its old statement would (DIMENSION through
  `dimension`, EXTERNAL and INTRINSIC through `external` and `intrinsic`,
  SAVE, VOLATILE, AUTOMATIC, and STATIC as one-name statements read with the
  unit's, `attributeLater`), so every old check applies (SAVE on a dummy is
  SAVEDUMMY), after checking it against the attributes the name has already
  (`attrsOf`, ATTRCONFLICT). INTENT, OPTIONAL, and TARGET are recorded
  (`Symbol.Intent`, `Optional`, `Target`; ATTRDUMMY, ATTRAGAIN). Only a
  Fortran 90 form is checked against the table, so two old statements keep
  their old messages; an old statement records its attribute only on a
  name already known (`attributeItems`), since a symbol made earlier would
  change the order of `$LOCAL`. PARAMETER with `::` is `defineParameter`
  (split from `parameter`) after the type, PARAMNOVALUE without `=`, and an
  array that is a PARAMETER NOTIMPL (Phase 102). An `= value` is a `dataSet`
  with `broadcast` (the one value every element's) and `assign` (INITTYPE:
  a character value only for CHARACTER, a number only for the rest; DEC's
  typeless and Hollerith constants as DATA takes them), and gives SAVE
  (`impliedSaves`, after AUTOMATIC and STATIC are read, not in a common
  block); a structure's field may have an `= value` too, but no attributes
  (FIELDATTR). A PUBLIC or PRIVATE statement outside a module is NOTINMODULE.
  The dump shows the recorded attributes and SAVE from a Fortran 90 form.
  Tests: `parse`'s `TestF90Declarations`, sema's golden `f90decl.for` and 33
  `TestErrors` cases, and `testdata/fortran90/declare.f90` (the same on
  gfortran, `compare.py`), run in every mode. MANUAL.md's 14.13 and HELP
  FORTRAN say what is compiled now; the full write-up is subtask 9's.
- **2026-11-17, subtask 4 (done):** kind selectors and kinds on constants.
  The parser: `typeSelector` (`parse/f90.go`) reads a type's selector into
  its `ast.TypeSpec` (`Selector`, `Kind`, `KindPos`; a CHARACTER selector's
  length into `Len`, as `*(...)`'s is) in a type statement, before FUNCTION,
  and in IMPLICIT (where `kindBeforeLetters` still tells it from the
  letters): `([KIND=] k)` for every type, and R506's forms for CHARACTER,
  `(len)`, `(LEN=len)`, `(len, [KIND=]k)`, `(LEN=len, KIND=k)`, `(KIND=k,
  LEN=len)`, `(KIND=k)`, each len an expression or `*`; a kind unnamed only
  after a length unnamed, and a selector and a `*n` not both (SYNTAX). The
  PendingStmt "a kind or length selector" is gone. Sema: `sema/kinds.go` has
  `Type.KindNumber` (a COMPLEX's kind half its size, CHARACTER's 1), the
  table of each keyword's kinds (`typeKinds`; INTEGER(1) and COMPLEX(16)
  NOTIMPL by name until subtasks 5 and 7), and `kindType`, which `typeOf`
  (through `selectedType` and `characterKind`) and the evaluator share; a
  selector's kind is a constant integer expression, 0 or more
  (KINDNOTCONST), one its type has (NOSUCHKIND; NOKIND on DOUBLE PRECISION,
  DOUBLE COMPLEX, and BYTE). A length after an entity's name replaces the
  statement's kind or length (`REAL(8) :: X*4`), as it replaces a `*n`. A
  CHARACTER selector's length that isn't constant (a dummy's N, finding 5)
  and one of zero or less (finding 4) are NOTIMPL by name;
  `characterType`'s `selector` says when, so an old `*(N)` keeps BADCHARLEN.
  `ast.KindConst` is the evaluator's `kindConst`: the kind digits or an
  INTEGER constant's name (KINDNAME); an integer in its kind's range
  (INTRANGE), a real read at its kind's precision by `realAs` (split from
  `real`), so `1.1_8` is `1.1D0`; KINDEXPONENT after a D or Q exponent;
  logicals converted; CHARACTER's only kind 1; KINDCONST on any other
  constant. A complex constant's parts may have kinds. Nothing changes
  under `/STANDARD` (finding 14). Tests: `parse`'s `TestF90TypeSelectors`,
  sema's golden `f90kinds.for` and 24 `TestErrors` cases, the Phase 99 tests
  that expected NOTIMPL for these forms, and `testdata/fortran90/kinds.f90`,
  run in every mode and the same on gfortran (`compare.py`). MANUAL.md's
  14.13 has a table of kinds, and HELP FORTRAN says what is compiled.
- **2026-11-17, subtask 5 (done):** INTEGER(1). Sema: `{Integer, 1}` is a
  type (`lengths` takes `INTEGER*1`, `typeKinds` no longer pends kind 1);
  `rank` puts it between LOGICAL and INTEGER*2, so two bytes give a byte and
  a byte beside a wider integer or a real takes that type (7.1.4.2);
  `intRange` is -128 to 127. An integer constant without a kind stays a
  longword beside a byte (`I + 5` is INTEGER): `mixedType`'s word rule for
  INTEGER\*2 is VAX FORTRAN's and isn't extended to bytes, which is the
  standard's reading. Two logicals are still "treated as integers" of at
  least INTEGER\*2, and BYTE is still LOGICAL\*1, so nothing old changes. The
  intrinsic table has no byte rows: `selectForm` takes INTEGER\*1 arguments
  to the INTEGER\*2 rows, and `byteResult` converts a generic function's
  result back to INTEGER\*1 when every INTEGER\*2 row of it gives an
  INTEGER\*2 (ABS, MOD, MAX, IAND, ISHFT, ...), but not INT (the default
  integer by footnote 10) or a specific name (IIABS's own INTEGER\*2). A
  power of bytes is done in INTEGER\*2 (OTS$POWII, or multiplied out) and
  converted back (`power`). A constant assigned to a byte may be -128 to 255,
  as LOGICAL\*1's may (`assigned`, VAX FORTRAN's rule for short integers;
  unchanged). Below sema nothing was missing: the IR's `Byte` type and
  codegen's suffixes already wrote ADDB3, MULB3, DIVB3, CMPB, MNEGB, CVTBW,
  CVTWB, CVTBL, CVTBF; the descriptor type is DSC$K_DTYPE_B (`ir.DTypeB`,
  `dtypeOf`); the debugger reads a one-byte integer already. FTNRTL's
  formatted and list-directed input took DTYPE_B; list-directed output (and
  so namelist output) refused it with BADPARAM, and now writes it as I5, as
  it writes a BYTE (`ftn_listout.mar`; no new entry, so GSMATCH stays). Tests:
  the golden `f90bytes.for` in sema, ir, and codegen, and
  `testdata/fortran90/bytes.f90`, run in every mode and the same on gfortran
  (`compare.py`). MANUAL.md's 14.13 and HELP FORTRAN say so.
- **2026-11-17, subtask 6 (done):** the kind and inquiry functions, and
  KIND= on the conversions. The parser: an argument in a reference's list
  may be `name=expr` (`ast.KeywordArg`; `parse/expr.go`'s `subscripts`, by
  `atKeyword`, so `==` is still a comparison). The table (Decision 14): a row
  may have two more columns, its class (E, I, T) and its dummies' names, a
  `?` after an optional one (`form.class`, `form.dummies`); INT, NINT, AINT,
  ANINT, REAL, and CMPLX's generic rows name theirs, with KIND. A second
  table, `fortran90Table`, has LOGICAL and the thirteen kind and inquiry
  functions (how `INQ`, `SIK`, `SRK`); its new names are `f90Intrinsics`,
  intrinsic under `/F90` or by an INTRINSIC statement (`intrinsicNamed`), and
  not passable (NOTPASSABLE). `sema/intrinsickeys.go`'s `intrinsicSlots` puts
  a reference's arguments in its dummies' order by the standard's 12.4.1
  (KEYWORDNAME, KEYWORDTWICE, KEYWORDORDER, MISSINGARG; a keyword to a
  function whose row names no dummies, or by a specific name, is NOTIMPL until
  Phase 108, and to a program's own procedure until Phase 103) and takes KIND
  out before the row is chosen; `resultKind` makes the result that kind of
  its type (a conversion converts to it at once, NINT and AINT convert their
  result; `selectForm` prefers a row already of that type, so `NINT(X,
  KIND=2)` is ININT). The constant evaluator does the same for NINT and CMPLX
  (`withKind`), and INT joins its functions (`constInt`: an integer's
  value, a real's truncated), since Fortran 90's initialization expressions
  have INT. `sema/inquiry.go`: the inquiries by name, each with the kinds of
  argument it takes and its answer, from `vaxfloat`'s new model (`model.go`:
  `Format.Digits`, `MinExponent`, `MaxExponent`, `Huge`, `Tiny`, `Epsilon`,
  held to Part A's table); an argument is analyzed only for its type (the
  evaluator's `inquiryArgType` reads a name without entering it, as declared
  or by IMPLICIT), and an INTEGER or LOGICAL constant without a kind is of
  the default kind, so `KIND(0)` is 2 under `/NOI4`. SELECTED_INT_KIND and
  SELECTED_REAL_KIND fold constants; of a value they are NOTIMPL. Part A's
  table held exactly but for H's HUGE and TINY, which it gave as "about": they
  are 5.94865748E4931 and 8.40525786E-4933. Nothing changes under `/NOF90`
  for a program that compiled before (`TestNoF90Unchanged`); `CMPLX(X, Y, X)`
  is now KINDNOTCONST rather than INTRINARGS. Tests: `vaxfloat`'s
  `TestModel`, `parse`'s `TestKeywordArguments`, sema's `TestInquiries` (every
  answer in each format setting and `/NOI4`), `TestKindArguments`,
  `TestInquiryErrors`, `TestF90NamesUnderNoF90`, and `TestKindsInDeclarations`,
  and `testdata/fortran90/inquire.f90`, run in every mode and on gfortran
  (`compare.py`, the seven lines of the real model in its KNOWN). MANUAL.md's
  14.13 has the model's table and finding 9's advice, and HELP FORTRAN says
  what is compiled.
- **2026-11-17, subtask 7 (done):** COMPLEX(16), by the author's answer 1
  (a). Sema: `{Complex, 32}`, COMPLEX*32 (`sema/kinds.go`'s `complex32`), is
  a type: `typeKinds` no longer pends kind 16, `lengths` takes `COMPLEX*32`
  (the HP manual's old form, as `INTEGER*1`), and `rank` puts it above
  COMPLEX*16. A complex constant is COMPLEX*32 when a part has the kind 16,
  `(1.0_16, 0.0)` (`evaluator.complex`'s `hKind`); since only a kind on a
  constant says so, a form VAX FORTRAN hasn't, `(1.0Q0, 0.0)` is still
  COMPLEX*16. Mixed operations keep VAX FORTRAN's rule, so a REAL(16) beside a
  COMPLEX(4) or COMPLEX(8) gives COMPLEX*16 where the standard's 7.1.4.2 would
  give COMPLEX(16) (a known difference, MANUAL.md's 14.13; `CMPLX(X, KIND=16)`
  makes one). A power of one, an intrinsic function no row of which takes one,
  and its I/O (an item, an implied-DO's, a namelist's) are NOTIMPL by name
  (`complex32Power`, `complex32Intrinsic`, `complex32IO`). The conversions and
  parts the IR writes in line have rows of their own in `fortran90Table`:
  INT, REAL (a REAL(16), the real part, by the standard's 13.13.84, where
  Table D-3's REAL of a COMPLEX*16 is REAL*4), DBLE, QEXT, CMPLX, DCMPLX,
  CONJG, and AIMAG, made generic for it (its COMPLEX*8 row the specific
  AIMAG's; nothing old changed). The folder does a product an instruction at a
  time, as F's (`vaxfloat.ComplexMulSteps` for H too), and a quotient as
  FTN$DIVCH does (`vaxfloat.ComplexDivSteps`, new: the denominator `c*c +
  d*d`, then each numerator, then the divisions, each step rounded).
  The IR: `Hexa`, 32 bytes, the whole value, which no instruction moves:
  lowering moves it as two octawords (`moveHexa`; a constant's halves as raw
  Octa constants, `assignHexa`; `value` for an argument copied), and `Check`
  refuses a Load or an Assign of one. A Hexa constant's imaginary part is
  `Const.ImBits` and `ImHigh`. Parts are H places as ever (`parts`), the
  product in line (no OTS$ routine), the quotient `hexaQuotient`, a call of
  FTN$DIVCH with a hidden result and both operands by reference;
  `DSC$K_DTYPE_HC` for a descriptor. Codegen: a Hexa place is only addressed,
  MOVAO or PUSHAO with its index doubled to octawords (`index`, `addressOp`).
  FTNRTL: `FTN$DIVCH` (`ftn_intrinsics.mar`, at vector offset 672; GSMATCH
  1,20), everything read before RESULT is written, so it may be an operand.
  The debugger table's type is `CH16` (`dbgtable.ComplxH`), which the debugger
  shows as a pair of H values. Tests: `vaxfloat`'s `TestComplexDivSteps`,
  sema's `TestComplex16` and `TestComplex16NotImpl`, the golden
  `f90cplx16.for` in sema, ir, and codegen (elements and dummy arrays with
  indexes, each address checked by hand), and `testdata/fortran90/cplx16.f90`,
  run in every mode and the same on gfortran (`compare.py`), its one third to
  33 digits. MANUAL.md's 14.13 and HELP FORTRAN say what is compiled.
- **2026-11-17, subtask 8 (done):** programs end to end. Two programs in
  `testdata/fortran90`, each worked by hand (the numbers by exact rational
  arithmetic) and run by `TestFortran90Programs` in every mode, paging off and
  on. `precise.f90` is portable: each unit asks for its precision by
  SELECTED_REAL_KIND(P=12) and SELECTED_INT_KIND, every constant has its kind,
  every conversion says KIND=; IMPLICIT REAL(SELECTED_REAL_KIND(P=12)), a kind
  named by KIND of a dummy, a RESULT variable, a function passed as an
  argument; Newton's square root, Simpson's rule for pi, a loan's payments, a
  quadratic's complex roots. It prints the same on gfortran with no known
  difference (`compare.py`'s PROGRAMS90): kind 8 is D_floating here and IEEE
  double there, and nothing it prints tells them apart. `forms.f90` is every
  declaration form the phase compiles in one program, Fortran 90's beside VAX
  FORTRAN's (attributes both ways, `*n` after a name overriding a selector,
  IMPLICIT with kinds, `/clist/`, DATA, and `=`, COMMON and EQUIVALENCE of
  names declared with `::`, STATIC and AUTOMATIC, INTEGER\*1, BYTE, DOUBLE
  COMPLEX, COMPLEX(16)); DEC's forms keep it from gfortran. Both compiled and
  ran as written but for two forms of later phases, which the programs no
  longer use: a DO with no control and EXIT (Phase 101) and an assignment to
  a whole array (Phase 102). No compiler change was needed.
- **2026-11-17, subtask 9 (done):** the close-out. FORTRAN-INTERNALS.md's
  section 25, "Fortran 90: scopes, declarations, and kinds" (the scope chain,
  the attribute table, kinds as byte counts, INTEGER(1) and COMPLEX(16), the
  inquiry functions and keyword arguments, what the programs found), its
  introduction, and section 21's map. MANUAL.md: 14.13's "Writing for any
  processor" (its example run on govax), and the chapter's overview, table,
  and 14.7's row naming declarations and kinds; HELP FORTRAN's IMPLICIT with
  kinds (the help library regenerated). PLAN.md's table row and Fortran 90
  section. The phase is done; Phase 101, control constructs, is next.

## Where things stand

What Phase 100 left, each for the phase named:

- **POINTER and ALLOCATABLE** are NOTIMPL (Phases 107 and 104); **INTENT,
  OPTIONAL, and TARGET** are recorded on the `Symbol` and change nothing:
  INTENT's checks and PRESENT are Phase 103's, TARGET's meaning Phase 107's.
  **PUBLIC and PRIVATE** are NOTINMODULE until modules (Phase 106).
- **Arrays:** array constructors as initial values, array PARAMETERs, and
  assumed- or deferred-shape bounds (`A(:)`) are NOTIMPL (Phases 102 and 104).
- **CHARACTER** of length zero, or of a length that isn't constant in a
  non-dummy's declaration, is NOTIMPL (Phase 108, answer 3).
- **COMPLEX(16)'s** I/O (Phase 109), powers, and intrinsic functions other
  than the conversions and parts (Phase 108). A REAL(16) beside a COMPLEX(4)
  or COMPLEX(8) gives COMPLEX\*16, VAX FORTRAN's rule, where the standard's
  7.1.4.2 gives COMPLEX(16) (MANUAL.md's 14.13).
- **SELECTED_INT_KIND and SELECTED_REAL_KIND of a value** that isn't constant
  are NOTIMPL (Phase 108).
- **Keyword arguments** to intrinsic functions whose rows don't name their
  dummies (Phase 108), and to a program's own procedures (Phase 103).
- **The scope chain** has hosts only for statement functions; internal and
  module procedures give it its use (Phase 103).
- **`/STANDARD`** judges nothing new (finding 14; Phase 110).
