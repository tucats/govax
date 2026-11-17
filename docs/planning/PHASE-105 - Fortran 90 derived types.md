# Phase 105 — Fortran 90: derived types

**Status:** planned 2026-11-17 with Phase 99 (`PHASE-99 - Fortran 90.md`,
Part C), for review. Needs Phase 104.

## Goal

Fortran 90's derived types (the standard's 4.4 and 6.1.2), on Phase 82's
STRUCTURE machinery (Decision 9):

- **definitions**: `TYPE POINT … END TYPE POINT`, components of every
  intrinsic type and kind, arrays, character, and other derived types;
  SEQUENCE; PRIVATE components (with Phase 106);
- **objects**: `TYPE(POINT) :: P, PATH(100)`; `%` and DEC's `.` both select a
  component, including through arrays (`PATH%X`, a section, and
  `PATH(I)%X`);
- **structure constructors** (`P = POINT(1.0, 2.0)`), in expressions,
  initialization, and PARAMETERs;
- **intrinsic assignment** and **equality** only by defined operators (Phase
  103's), as the standard has it;
- derived types as **dummies**, **function results** (through a hidden argument
  past a quadword, as COMPLEX*16 is), in **COMMON** and **EQUIVALENCE** (SEQUENCE
  types only), and in **DATA**;
- **I/O** of a derived-type object, a component at a time in order (formatted,
  list-directed, unformatted), and in **NAMELIST**.

## What earlier phases leave in place

- `sema.Structure` and `Field`, unions and maps, `/ALIGN` (`sema/align.go`), the
  IR's `fieldMem`, record moves (`moveRecord`), record arguments compared by form
  (`Structure.form`, RECORDARG); `testdata/fortran/itemlist.for`.
- Scopes (a type's definition is a scope for its components' names), explicit
  interfaces, arrays as values.

## Design notes

- **One model.** A derived type is a `Structure` with a name and the `Derived`
  mark; STRUCTURE's unions and maps aren't allowed in one, and a TYPE's SEQUENCE
  and alignment rules are checked where they differ. A RECORD and a TYPE object
  of the same layout are still different types for argument checking (the
  standard's type equality, 4.4.2: same definition, or SEQUENCE types with the
  same name and components).
- **Layout** (Phase 99's question 6): SEQUENCE packed by `/ALIGN`'s rules for
  STRUCTURE, other types naturally aligned; the storage map shows every
  component's offset.
- **A section of components** (`PATH%X`) is a section with a stride of the
  type's size: an NCA descriptor (Phase 104), not a copy.
- **Teaching:** FORTRAN-INTERNALS.md compares STRUCTURE and TYPE, and shows why a
  component section is just another stride.

## Open questions

1. Should a derived type and a STRUCTURE of the same name and fields be
   interchangeable as arguments? Proposed: no (the standard's rule), with a
   clear message.

## Subtasks (provisional)

1. **Survey** (the standard's 4.4, 6.1.2, 7.5.1.5, 9.4.2; the Handbook's chapter
   4; the HP manual's derived types and records side by side), for review.
2. **Definitions and objects**; `%`; the storage map.
3. **Components of arrays** and arrays of components (sections by stride).
4. **Structure constructors**, initialization, PARAMETERs.
5. **Assignment, arguments, results**, type equality.
6. **COMMON, EQUIVALENCE, DATA** with SEQUENCE types.
7. **I/O and NAMELIST** of derived types (FTNRTL takes the components as items;
   the compiler expands them).
8. **Programs end to end**, every mode, gfortran's witness.
9. **Close-out**: FORTRAN-INTERNALS.md's section 30, MANUAL.md.

## Progress log

- **2026-11-17:** Planned with Phase 99.
