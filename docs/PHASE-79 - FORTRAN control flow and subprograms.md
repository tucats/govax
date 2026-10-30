# Phase 79 — FORTRAN control flow and subprograms

**Status:** planned (2026-10-29). Needs Phase 78 (expressions in every
type). Part of the FORTRAN program laid out in `PHASE-74 - FORTRAN
plan.md` (Decisions 8, 10, and 11).

## Goal

Generate code for all of the manual's chapters 5 and 6:

- **control statements:** unconditional, computed, and assigned GO TO;
  ASSIGN (of statement and FORMAT labels); arithmetic, logical, and
  block IF; DO with every form of control (INTEGER and REAL variables,
  negative and computed increments, the trip count rule), nested DO
  loops sharing a terminal statement, transfers out of and back into a
  loop's range (5.3.1.4, extended range); DO WHILE; END DO and DO
  without a label; CONTINUE; STOP and PAUSE with a code or message;
  RETURN with alternate returns; END;
- **subprograms:** SUBROUTINE and FUNCTION of every result type
  (CHARACTER and COMPLEX through the hidden argument), statement
  functions (expanded inline), ENTRY in subroutines and functions,
  alternate return arguments (`*label`), adjustable arrays with bounds
  from arguments or COMMON, assumed-size arrays, passed-length
  CHARACTER arguments, CHARACTER and Hollerith constants as actual
  arguments, procedure names as arguments (EXTERNAL and INTRINSIC),
  null arguments, and %VAL, %REF, %DESCR, and %LOC (Table 6-1);
- **the calling standard** (Decision 11) as the rest of VMS sees it, so
  a FORTRAN routine can be a callback (Phase 83 uses this).

## What earlier phases leave in place

- **Phase 75:** CALL, simple functions, logical and block IF, labeled DO.
- **Phase 76:** label tables and the statement order rules.
- **Phase 78:** every type's code.

## Design notes

- **ENTRY:** each entry point is a `.ENTRY` that copies its arguments'
  addresses into the routine's static argument cells, then branches to
  the shared code; dummy arguments are reached through those cells, so
  one body serves every entry. The doc explains why FORTRAN can do this
  (static storage, no recursion).
- **Assigned GO TO** stores a label's address; with a label list the
  generated code checks it (CASEL over the list).
- **Alternate returns:** RETURN n sets R0 = n; the caller's CASEL
  dispatches on it, falling through for 0 or out of range.
- **Adjustable arrays:** their bounds and multipliers are computed on
  entry, from the arguments' values, into static cells (the manual: a
  later change to the bound variables doesn't change the array).
  Phase 76's subtask 5 declares and checks them: `sema/arrays.go` keeps
  each bound that isn't constant as its syntax tree (`Dim.LowerExpr`,
  `UpperExpr`) and reports NOTIMPL at it, in `checkArrays`.
- **PAUSE** writes its message to SYS$OUTPUT and reads a line from
  SYS$INPUT (VAX FORTRAN's PAUSE at a terminal waits for CONTINUE; in a
  batch-like run it is a no-op). The choice is recorded.
- **STOP n** ends the image with a status: the manual's rules, with the
  displayed text through FTNRTL.

## Open questions

- **PAUSE's behavior** at govax's terminal: the User Manual's
  description isn't in the folder; a prompt and a line read is the
  proposal.
- **A REAL DO variable's** iteration count when the increment is
  inexact: the manual's formula, computed in the loop variable's type.

## Subtasks (provisional)

1. **Survey:** chapters 5 and 6, the calling standard's argument list,
   Table 6-1.
2. **GO TO in every form, ASSIGN, arithmetic IF.**
3. **DO in every form**, DO WHILE, END DO, extended range.
4. **STOP, PAUSE, RETURN, END.**
5. **Function results of every type**, CHARACTER and COMPLEX functions.
6. **ENTRY.**
7. **Alternate returns.**
8. **Adjustable and assumed-size arrays, passed-length characters.**
9. **Procedure arguments, null arguments, %VAL/%REF/%DESCR/%LOC.**
10. **Statement functions.**
11. **End to end**, then **close-out** (FORTRAN-INTERNALS.md's chapter
    on calls and control flow).

## Progress log

- **2026-10-29:** Planned with Phase 74.
