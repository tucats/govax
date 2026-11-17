# Phase 101 — Fortran 90: control constructs

**Status:** planned 2026-11-17 with Phase 99 (`PHASE-99 - Fortran 90.md`,
Part C), for review. Needs Phase 100.

## Goal

Fortran 90's control constructs (the standard's 8.1):

- **SELECT CASE**: `SELECT CASE (expr)`, `CASE (1, 3:5, 10:)`, `CASE ('A':'Z')`,
  `CASE (.TRUE.)`, `CASE DEFAULT`, `END SELECT`; INTEGER, CHARACTER, and LOGICAL
  selectors; overlapping cases an error at compile time (8.1.3.2);
- **DO with no loop control** (`DO … END DO`), ended by EXIT;
- **EXIT** and **CYCLE**, with or without a construct name;
- **construct names** on IF, DO, and SELECT CASE (`OUTER: DO`, `END DO OUTER`,
  `EXIT OUTER`, `ELSE IF (…) THEN OUTER`, `CASE (1) OUTER`), checked as the
  standard's constraints say (a name ends where it began, EXIT and CYCLE name
  only a DO that contains them).

## What earlier phases leave in place

- `sema/stmt.go`'s stack of open constructs and `labels.go`'s jump checks; the
  IR's `Switch` (CASEL, Phase 79) and `Jump`; DO WHILE and labelless DO.
- The optimizer's loop analysis (natural loops), which sees EXIT and CYCLE as
  ordinary jumps.

## Design notes

- **SELECT CASE's code.** An INTEGER selector with dense constant cases becomes
  `Switch` (CASEL), ranges filling their slots; a sparse one, or a range wider
  than a limit, becomes a chain of comparisons ordered as the cases are, or a
  binary search over sorted cases if there are many: one function decides,
  with its threshold explained. CHARACTER cases compare as CHARACTER relations
  do (the shorter blank-padded), LOGICAL ones are a Branch.
- EXIT jumps past the construct's END DO; CYCLE to the loop's increment (for a
  counted DO) or its test (DO WHILE, no control). Both leave a DO's iteration
  count right, since the count is in a cell, not recomputed.
- Construct names are scoped to the unit: a construct name may not also be a
  variable's (the standard's 14.1.2).

## Open questions

1. Should `/WARNINGS` flag a SELECT CASE with no DEFAULT that can miss? (The
   standard allows it: nothing happens.) Proposed: no.

## Subtasks (provisional)

1. **Survey** (the standard's 8.1, the Handbook's chapter 8), for review.
2. **Construct names, DO with no control, EXIT, CYCLE.**
3. **SELECT CASE**: parsing, sema's checks (types, overlaps, one DEFAULT), the
   lowering's three forms, each tested.
4. **Programs end to end** (`testdata/fortran90`), both optimizer settings,
   gfortran's witness; the optimizer's tests for EXIT out of nested loops.
5. **Close-out**: FORTRAN-INTERNALS.md's section 26, MANUAL.md.

## Progress log

- **2026-11-17:** Planned with Phase 99.
