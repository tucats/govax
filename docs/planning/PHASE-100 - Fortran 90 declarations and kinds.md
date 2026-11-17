# Phase 100 — Fortran 90: declarations and kinds

**Status:** planned 2026-11-17 with Phase 99 (`PHASE-99 - Fortran 90.md`,
Part C), for review. Needs Phase 99.

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

## Subtasks (provisional)

1. **Survey** (the standard's chapter 5 and 4.3, the Handbook's chapters 4 and
   5), with any change to this plan, for review.
2. **The scope chain** in sema, a refactoring with no change in what is
   compiled: every old test passes and `TestNoF90Unchanged` holds.
3. **`::` declarations and attributes** (the parser's entity lists, sema's
   attribute table, initialization through `data.go`).
4. **Kind selectors and kinds on constants**; IMPLICIT with kinds.
5. **The kind and numeric inquiry functions**, folded; held to the standard's
   model with each VAX format's parameters, both `/G_FLOATING` settings.
6. **Programs end to end** (`testdata/fortran90`: declarations of every form,
   a program portable to gfortran using `SELECTED_REAL_KIND`).
7. **Close-out**: FORTRAN-INTERNALS.md's section 25 (scopes and kinds),
   MANUAL.md's 14.13.

## Progress log

- **2026-11-17:** Planned with Phase 99.
