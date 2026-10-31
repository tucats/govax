# Phase 85 — FORTRAN compiler usability: listings, checks, warnings

**Status:** planned (2026-10-29). Needs Phase 80 (I/O, for the
run-time checks' messages); best done after Phase 83. Part of the
FORTRAN program laid out in `PHASE-74 - FORTRAN plan.md` (Decisions 18
and 19).

## Goal

The qualifiers that make the compiler pleasant to use, in VAX FORTRAN's
spirit but our own layouts:

- **`/LIST[=file]`:** a listing of the source with line numbers and
  include levels, the messages under their lines, and the storage map
  (each variable, its type, psect, and offset, and the `.MAR` symbol);
  `/SHOW=(INCLUDE, MAP, ...)` choosing its parts; CDEC$ TITLE and
  SUBTITLE in its headings;
- **`/CROSS_REFERENCE`:** where each name and label is defined and used;
- **`/CHECK=(BOUNDS, OVERFLOW, UNDERFLOW, ALL, NONE)`:** subscript and
  substring checks (the INDEX instruction, which traps on a range
  error, or compare and signal), integer overflow (the entry mask's IV
  bit), floating underflow reporting;
- **`/WARNINGS=(DECLARATIONS, GENERAL, NONE)`:** undeclared names
  (IMPLICIT NONE without the statement), unused variables, labels never
  used;
- **`/STANDARD=(SYNTAX, SOURCE_FORM)`:** flagging DEC extensions for
  FORTRAN-77 conformance;
- **`/[NO]DEBUG`:** the `.MAR` file gets `.ENABLE DEBUG`, so MACRO
  writes debugger records without being asked, and every variable a
  labeled symbol the debugger can EXAMINE;
- **`/[NO]OBJECT[=file]`** (approved in Phase 74's review, answer 3):
  FORTRAN runs MACRO on the `.MAR` file it wrote, keeping the `.MAR`
  file, so `FORTRAN/OBJECT; LINK; RUN` works as on VMS. Off by default:
  FORTRAN writes only the `.MAR` file unless asked;
- **`/F77` and `/NOF77`**, if kept (appendix A.6).

## What earlier phases leave in place

- `/[NO]I4` and `/[NO]G_FLOATING`, on the command and in OPTIONS (Phase 78);
  OPTIONS's `/CHECK` and `/EXTEND_SOURCE` are this phase's.
- `diag`'s messages with positions (Phase 74), the storage map comment
  (Phases 75–76), MACRO's `/DEBUG` and the debugger's EXAMINE by symbol.

## Open questions

- **The listing's layout:** ours, modeled loosely on VAX FORTRAN's
  (which the User Manual shows); page headings like MACRO's.
- **Bounds checks on assumed-size arrays:** only the lower bound and
  the known dimensions can be checked; the manual's rule.

## Subtasks (provisional)

1. **Survey:** the qualifiers as the DCL Dictionary's FORTRAN entry (if
   any) and the manual describe them.
2. **`/LIST`, `/SHOW`, TITLE and SUBTITLE.**
3. **`/CROSS_REFERENCE`.**
4. **`/CHECK`.**
5. **`/WARNINGS` and `/STANDARD`.**
6. **`/DEBUG` and `/OBJECT`.**
7. **Close-out:** HELP FORTRAN complete; MANUAL.md.

## Progress log

- **2026-10-29:** Planned with Phase 74.
