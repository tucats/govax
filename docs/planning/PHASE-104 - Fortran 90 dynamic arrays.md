# Phase 104 — Fortran 90: dynamic arrays

**Status:** planned 2026-11-17 with Phase 99 (`PHASE-99 - Fortran 90.md`,
Part C), for review. Needs Phase 103.

## Goal

Arrays whose shape, or whose place, is known only at run time:

- **NCA descriptors** (Decision 7, the Calling Standard's 5.7) built, passed,
  and read: the IR's descriptor gains class NCA, with strides and bounds, and
  `Mem.Ptr` reaches an element through A0 and strides;
- **assumed-shape dummies** (`REAL A(:, :)`): passed an NCA descriptor by every
  caller that has an explicit interface (a whole array, a section with any
  strides, another assumed-shape array); `LBOUND` of a dummy is 1 unless
  declared (`A(0:, :)`);
- **sections to old dummies**: a section or assumed-shape array passed to an
  explicit-shape or assumed-size dummy is copied to a contiguous temporary and
  back when not known contiguous (the standard's 12.4.1.1);
- **automatic arrays** (`REAL W(SIZE(A))`, `CHARACTER(LEN=N) :: S`) in the frame
  (Decision 8);
- **ALLOCATABLE, ALLOCATE, DEALLOCATE, ALLOCATED**, with STAT= and the errors
  (allocating an allocated array, deallocating an unallocated one); a local
  deallocated at return (Decision 8; question 4 of Phase 99);
- **array-valued functions** and functions of run-time character length, their
  result through a hidden descriptor argument, as CHARACTER results are today;
- the **transformational intrinsics** on run-time shapes: MATMUL, TRANSPOSE,
  RESHAPE, SPREAD, PACK, UNPACK, CSHIFT, EOSHIFT, MERGE, and the reductions of
  Phase 102 over descriptors;
- the scalarizer's **temporaries of run-time size**, in the frame or, past the
  limit, on the heap (Decision 8).

## What earlier phases leave in place

- Class A descriptors and adjustable arrays (`ir/lowerbounds.go`); the frame and
  `ir.Routine.NewCell` (Phase 89); class D strings and `lowerer.after`.
- LIB$GET_VM and LIB$FREE_VM; FTNRTL's error model (`FTN$$IO_SIGNAL`, FOR's
  messages).
- The scalarizer and explicit interfaces (Phases 102, 103).

## Design notes

- **Teaching descriptors.** FORTRAN-INTERNALS.md draws the NCA descriptor of
  `A(2:10:2, J)` beside the array's memory and shows how A0 and the strides give
  each element's address with one multiply-add per dimension; then why an
  assumed-shape dummy needs the caller's interface (the callee can't know how it
  was called).
- **ALLOCATE** is an FTNRTL call (`FTN$ALLOCATE(desc, element size, bounds...)`,
  Decision 15) that fills the descriptor and takes the memory from LIB$GET_VM, so
  its error handling (STAT= or a signal) is in one place; DEALLOCATE gives it back
  and clears POINTER.
- **Strides are bytes**, so a section of a REAL*8 array with stride 2 has stride
  16; a contiguous array has strides equal to the element size times the lower
  dimensions' extents.
- **The copy-in/out rule** uses a "known contiguous" flag from sema: a whole
  explicit-shape array, a section with unit stride in the first dimension and
  whole later dimensions, never an assumed-shape dummy (unless `/ASSUME=CONTIGUOUS`
  is wanted later).
- **FTNRTL's array movers** take NCA descriptors and an element size, so one
  routine serves every type.

## Open questions

1. The heap limit for temporaries (64 KB proposed): fixed, or a qualifier?
2. An allocatable array in COMMON is not allowed (the standard), but one in
   EQUIVALENCE neither; confirm no DEC extension wants either.

## Subtasks (provisional)

1. **Survey** (the standard's 5.1.2.4, 6.3, 12.4.1.1, 13.10.13 to 13.10.18; the
   Handbook's chapters 5, 6, 12), the NCA layout checked in MACRO, for review.
2. **NCA descriptors in the IR** and an element through them.
3. **Assumed-shape dummies**, with every kind of actual.
4. **Copy-in and copy-out** for old dummies.
5. **Automatic arrays** and run-time-sized temporaries.
6. **ALLOCATABLE and ALLOCATE/DEALLOCATE** (`ftn_alloc.mar`), STAT=, the new
   FOR conditions, deallocation at return.
7. **Array-valued functions** and run-time character lengths.
8. **The transformational intrinsics** (`ftn_array.mar`, or in line where Phase
   102's are), each against the standard's examples.
9. **Programs end to end** (a linear-equation solver with assumed-shape
   arrays, a sieve with ALLOCATE), every mode, gfortran's witness.
10. **Close-out**: FORTRAN-INTERNALS.md's section 29 (descriptors and dynamic
    storage), MANUAL.md.

## Progress log

- **2026-11-17:** Planned with Phase 99.
