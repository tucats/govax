# Phase 107 — Fortran 90: pointers

**Status:** planned 2026-11-17 with Phase 99 (`PHASE-99 - Fortran 90.md`,
Part C), for review. Needs Phase 106.

## Goal

Fortran 90's pointers (the standard's 5.1.2.7, 6.3, 7.5.2), by Decision 12:

- **POINTER** and **TARGET**, as attributes and statements; scalar, character,
  array, and derived-type pointers;
- **pointer assignment** (`P => T`, `P => A(1:N:2)`, `P => Q`), with its
  constraints (a target must be a TARGET or a pointer; rank and type agree);
- **NULLIFY**, **ASSOCIATED(P [, T])**, and ALLOCATE and DEALLOCATE of pointers
  (Phase 104's routines, the memory owned by whoever deallocates it);
- **pointer components** of derived types, including a type pointing to itself
  (linked lists, trees), in modules;
- **pointer dummies** and **pointer function results**, needing an explicit
  interface;
- **using a pointer** where its target would go: in expressions, as an actual
  argument to an ordinary dummy (the target is passed), in I/O;
- the **optimizer's aliasing rules**: a POINTER aliases a TARGET or another
  POINTER of its type and kind, nothing else; a call may change any TARGET it can
  reach.

## What earlier phases leave in place

- NCA descriptors, ALLOCATE and DEALLOCATE (Phase 104), derived types (Phase
  105), modules (Phase 106).
- `opt/effects.go`'s `Storage` and the places analysis (`opt/places.go`), which
  learn the new rule rather than a new pass.

## Design notes

- **Representation** (Decision 12): a scalar pointer is an address, 0 when
  disassociated; a character pointer a class S descriptor; an array pointer an
  NCA descriptor. Pointer assignment copies the address or builds the
  descriptor; it never copies data.
- **Undefined status.** Pointers start disassociated (static storage is zero;
  frame pointers cleared at entry): Fortran 95's behavior, a choice, in MANUAL.md.
- **A dangling pointer** (to a deallocated target, or a returned routine's local)
  is undefined behavior the standard doesn't require catching; `/CHECK=POINTERS`
  (Phase 110) may catch a disassociated pointer's use.
- **Teaching:** FORTRAN-INTERNALS.md walks a linked list's code, and explains
  why the TARGET attribute exists: it is what lets the optimizer keep every other
  variable in a register across a pointer store.

## Open questions

1. Should DEALLOCATE of a pointer to memory not obtained by ALLOCATE be caught?
   Proposed: FTNRTL's heap blocks carry a mark it checks, an error if absent.

## Subtasks (provisional)

1. **Survey** (the standard's 5.1.2.7 and 5.1.2.8, 6.3.1.2, 6.3.3.2, 7.5.2,
   12.4.1.1; the Handbook's chapters 5 to 7), for review.
2. **Scalar and character pointers**, `=>`, NULLIFY, ASSOCIATED.
3. **Array pointers** and pointers to sections.
4. **ALLOCATE and DEALLOCATE of pointers.**
5. **Pointer components**, recursive types.
6. **Pointer dummies and results.**
7. **The optimizer's aliasing rules**, with programs that would break if they
   were wrong, both settings.
8. **Programs end to end** (a linked list and a binary tree in modules, a sparse
   matrix), every mode, gfortran's witness.
9. **Close-out**: FORTRAN-INTERNALS.md's section 32 (pointers and aliasing),
   MANUAL.md.

## Progress log

- **2026-11-17:** Planned with Phase 99.
