# Phase 82 — FORTRAN: DEC's data extensions

**Status:** planned (2026-10-29). Needs Phase 79 (subprograms). Part of
the FORTRAN program laid out in `PHASE-74 - FORTRAN plan.md`
(Decisions 1 and 10).

## Goal

DEC's extensions to FORTRAN-77's data and declarations, which VMS
programs use heavily (above all to build item lists for system
services):

- **STRUCTURE, RECORD, UNION, MAP** (4.13, 4.15, 2.2.5): structure
  declarations, substructures, unions of maps, `%FILL`, field
  references (`REC.FIELD.SUB(3)`), aggregate assignment (3.4), records
  as arguments, records in COMMON and EQUIVALENCE, arrays of records,
  and records in I/O lists (unformatted);
- **typeless constants:** octal and hexadecimal (`'777'O`, `'FF'X`,
  `"777`), Hollerith (`5HHELLO`) and their typing by context (2.2.1.4,
  2.2.1.7, appendix A.5), RADIX-50 constants (appendix B.3);
- **VOLATILE** (4.16): every reference goes to memory;
- **AUTOMATIC and STATIC**: a variable in the stack frame, or static
  explicitly;
- **OPTIONS** (1.4.3) and the **CDEC$ directives** (chapter 10): PSECT
  (COMMON block attributes, Table 10-1), IDENT (`.IDENT`), TITLE and
  SUBTITLE (listings, Phase 85).

## What earlier phases leave in place

- **Phase 76:** STRUCTURE blocks parsed; COMMON and EQUIVALENCE layout.
- **Phase 78:** every type's code, character moves (aggregate assignment
  is a byte move).

## Design notes

- **Structures are laid out** as the manual's 2.2.5.1 says (fields in
  order, no padding unless `%FILL`); a union is as long as its longest
  map. Field references lower to a base address plus a constant offset,
  plus subscripts.
- **AUTOMATIC variables** live in a frame area the entry sequence
  reserves (`SUBL2 #n,SP`), reached through FP. A routine with them is
  still not recursive in VAX FORTRAN V5's sense, but it becomes
  reentrant for AST use.
- **VOLATILE** turns off the optimizer's (Phase 84) caching of the
  variable; before Phase 84 every reference is to memory anyway, so the
  statement is recorded and checked.

## Open questions

- **RADIX-50 constants:** rare in VMS programs; include for
  completeness, or leave as NOTIMPL until a program needs them?
- **Alignment of records** under `/ALIGN` (a later VAX FORTRAN's
  qualifier): out, as V5.0 has none.

## Subtasks (provisional)

1. **Survey:** the manual's sections above, a table of every form.
2. **STRUCTURE and its layout**, UNION, MAP, `%FILL`.
3. **RECORD and field references**, aggregate assignment, records as
   arguments and in COMMON.
4. **Typeless and Hollerith constants**, RADIX-50 if kept.
5. **VOLATILE, AUTOMATIC, STATIC.**
6. **OPTIONS and CDEC$ directives.**
7. **End to end:** an item list for `$GETJPIW` built from a structure
   (run in Phase 83 once services are reachable; here, its layout
   checked byte by byte), then **close-out**.

## Progress log

- **2026-10-29:** Planned with Phase 74.
