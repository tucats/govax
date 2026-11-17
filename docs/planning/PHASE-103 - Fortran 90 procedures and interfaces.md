# Phase 103 — Fortran 90: procedures and interfaces

**Status:** planned 2026-11-17 with Phase 99 (`PHASE-99 - Fortran 90.md`,
Part C), for review. Needs Phase 102.

## Goal

Fortran 90's procedures (the standard's chapter 12):

- **internal procedures**: a main program's or a subprogram's CONTAINS part,
  each internal procedure a scope whose host is its container, seeing the
  host's names by **host association** (12.1.2.2.1), and reaching a host's
  frame by Decision 11's hidden argument when the host's storage is there;
- **interface blocks** (`INTERFACE … END INTERFACE`) describing external and
  dummy procedures; an **explicit interface** (an internal procedure's, an
  interface block's, and, in Phase 106, a module procedure's) checked at every
  call: the number, types, kinds, and ranks of the arguments;
- **keyword arguments** (`CALL SOLVE(A, TOL=1.0E-6)`) and **OPTIONAL** dummies
  with **PRESENT**; an absent argument is VMS's null argument (Decision 13);
- **INTENT(IN/OUT/INOUT)**: an INTENT(IN) dummy not changed, an INTENT(OUT)
  one not read before it's set where the compiler can tell (`/WARNINGS`);
- **generic interfaces** (`INTERFACE SWAP` with several specific procedures),
  resolved by type, kind, and rank at compile time, with the standard's
  ambiguity rules checked at the block (14.1.2.3);
- **defined operators** (`INTERFACE OPERATOR(.CROSS.)`, `OPERATOR(+)` on new
  types) and **defined assignment** (`INTERFACE ASSIGNMENT(=)`);
- a FUNCTION whose result is an array or has a run-time length, when its
  interface is explicit, waits for Phase 104 (its result is passed through a
  descriptor).

## What earlier phases leave in place

- RECURSIVE and RESULT (Phase 89); ENTRY; alternate returns; procedure dummies
  (Phase 79); `%VAL`, `%REF`, `%DESCR`, `%LOC`; the argument list's
  conventions (Table 6-1; `sema/arguments.go`).
- The scope chain (Phase 100), which internal procedures and interface bodies
  give its first hosts.
- `checkReferences`: a file's units checked against each other (FUNCTYPE,
  ARGCOUNT), which explicit interfaces make exact.

## Design notes

- **Nesting in the AST.** `ast.Unit` gains `Contains []*Unit`; the parser's
  unit forming (`Parse`) learns CONTAINS and that an internal procedure's END
  must name its kind (`END SUBROUTINE`).
- **Where an internal procedure's code goes:** after its host's in the same
  `.MAR` module, its entry a local name (`host.$Pn`, Decision 10), so it can't be
  called from outside, as the standard says.
- **Host association's code.** A host variable in `$LOCAL` is named directly; one
  in the host's frame is `n(Rx)` with Rx loaded from the hidden argument. The
  optimizer must know that a call of an internal procedure may change any host
  variable it can see (`opt/effects.go`'s call rule widens).
- **Generic resolution** is one function over candidate specifics, explained
  with the standard's own example; a reference matching none or two is an error
  naming them.
- **Defined operators** are parsed as any `.name.` operator, with the standard's
  precedence (a unary defined operator highest, a binary one lowest), and DEC's
  record field reading tried first (`PHASE-99`'s table).

## Open questions

1. Should a call through an implicit interface to a routine in the same file
   be checked against its definition as an explicit one would be? Today's
   ARGCOUNT says yes for counts; the plan extends it to kinds and ranks as
   warnings.

## Subtasks (provisional)

1. **Survey** (the standard's chapter 12 and 14.1; the Handbook's chapter 12),
   for review.
2. **CONTAINS and internal procedures** with static hosts.
3. **Hosts in the frame**: the hidden argument; recursive hosts; `/RECURSIVE`
   everywhere.
4. **Interface blocks and explicit interfaces**, checked.
5. **Keyword arguments, OPTIONAL, PRESENT, INTENT.**
6. **Generic interfaces.**
7. **Defined operators and assignment.**
8. **Programs end to end**, every mode, gfortran's witness.
9. **Close-out**: FORTRAN-INTERNALS.md's section 28 (nested scopes, host
   association, and resolving a call), MANUAL.md.

## Progress log

- **2026-11-17:** Planned with Phase 99.
