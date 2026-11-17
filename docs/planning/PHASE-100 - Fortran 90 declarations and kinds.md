# Phase 100 — Fortran 90: declarations and kinds

**Status:** planned 2026-11-17 with Phase 99 (`PHASE-99 - Fortran 90.md`,
Part C). Subtask 1, the survey (Part A, below, with a revised subtask list
and questions for the author), written 2026-11-17, for review. Needs Phase 99.

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

1. **Survey** (this Part A), for review.
2. **The scope chain** in sema (finding 13): a refactoring, no change in what
   is compiled; every old test passes and `TestNoF90Unchanged` holds.
3. **`::` declarations and attributes:** the parser's entity lists and
   attribute specs, the attribute statements in both forms, sema's attribute
   table (finding 11), `= initialization` through `data.go` with scalars
   broadcast (finding 10), implied SAVE, PARAMETER with `::`, `CHARACTER(*)`
   PARAMETERs.
4. **Kind selectors and kinds on constants:** every type spec of R502 and
   R506, `Type.KindNumber`, `ast.KindConst` compiled, kinds named by
   PARAMETERs, IMPLICIT with kinds, FUNCTION with a kind selector, and the
   messages for a kind the processor lacks.
5. **INTEGER(1)** (finding 1): sema's type and rules, the IR's and codegen's
   byte arithmetic and conversions, intrinsics through INTEGER(2), I/O, the
   debugger table's type; `INTEGER*1`.
6. **The kind and inquiry functions**, folded (A.1's table, both
   `/G_FLOATING` settings, `/NOI4`), and **KIND= on the conversions**
   (finding 7), with the intrinsic table's keyword and optional columns
   (finding 8).
7. **COMPLEX(16)**, as far as question 1's answer takes it.
8. **Programs end to end** (`testdata/fortran90`: declarations of every form, a
   program portable to gfortran using `SELECTED_REAL_KIND` and `KIND=`).
9. **Close-out**: FORTRAN-INTERNALS.md's section 25 (scopes and kinds),
   MANUAL.md's 14.13, HELP FORTRAN.

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
