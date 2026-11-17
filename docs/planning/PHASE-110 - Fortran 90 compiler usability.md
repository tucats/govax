# Phase 110 — Fortran 90: the compiler's usability

**Status:** planned 2026-11-17 with Phase 99 (`PHASE-99 - Fortran 90.md`,
Part C), for review. Needs Phases 107 to 109.

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
  loops need more (loop fusion across statements, the run-time overlap test of
  Phase 102's question 1).

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

1. **Survey**, for review.
2. **`/STANDARD=F90` and the obsolescent features.**
3. **`/WARNINGS`** for the new constructs.
4. **Listings and cross references** with scopes and modules.
5. **`/CHECK`** on shapes, descriptors, pointers, allocation.
6. **`/DEBUG` and the debugger** on Fortran 90's data.
7. **The optimizer measured** on the corpus; what's worth doing now done.
8. **Close-out**: FORTRAN-INTERNALS.md, MANUAL.md, HELP FORTRAN.

## Progress log

- **2026-11-17:** Planned with Phase 99.
