# Phase 108 — Fortran 90: the intrinsic procedures

**Status:** planned 2026-11-17 with Phase 99 (`PHASE-99 - Fortran 90.md`,
Part C), for review. Needs Phase 104 (may move earlier or later).

## Goal

The rest of Fortran 90's intrinsic procedures (the standard's chapter 13, the
Handbook's Appendix A), each a row in sema's table (Decision 14), each usable
on arrays where elemental:

- **character**: ACHAR, IACHAR, ADJUSTL, ADJUSTR, TRIM, LEN_TRIM, REPEAT, SCAN,
  VERIFY, INDEX with BACK=, LGE/LGT/LLE/LLT (already), and character results
  whose length is known only at run time (TRIM, REPEAT, ADJUSTL of a passed
  length), through class D strings as concatenation's are (Phase 78);
- **numeric**: CEILING, FLOOR, MODULO; the kind= arguments of AINT, ANINT, INT,
  NINT, REAL, CMPLX, LOGICAL, CHAR;
- **the model**: EXPONENT, FRACTION, NEAREST, RRSPACING, SPACING, SCALE,
  SET_EXPONENT, on each VAX format's bits (in line where short, MTHRTL or FTNRTL
  where not);
- **bits**: MVBITS (a subroutine), BTEST, IAND, IBITS, ISHFT, ISHFTC, NOT, and
  the rest already from MIL-STD-1753, now with Fortran 90's argument names;
- **TRANSFER**, with its three forms;
- **subroutines**: DATE_AND_TIME, SYSTEM_CLOCK, RANDOM_NUMBER, RANDOM_SEED
  (`MTH$RANDOM`'s generator or a new one: the survey decides), MVBITS;
- **keyword arguments** for every intrinsic (the table's dummies' names).

## What earlier phases leave in place

- The intrinsic table and its in-line, MTHRTL, and FTNRTL forms
  (`sema/intrinsics.go`, `ir/lowerintrinsic.go`, `ftn_intrinsics.mar`), the
  system subroutines (`ftn_system.mar`, Phase 83: DATE, TIME, RAN, SECNDS).
- `vaxfloat` for the model functions' bits; MTHRTL's engine.

## Design notes

- **Which form for each** is a column in the table, chosen by cost: a few
  instructions in line; a loop or a heap-using result in FTNRTL; a numeric
  function MTHRTL would naturally hold in MTHRTL (but MTHRTL is VMS's vector, so
  new routines there would be govax's own names: the survey decides whether that
  is acceptable or whether FTNRTL takes them).
- **DATE_AND_TIME's zone** is the host's offset, as LIB$'s time routines see it.
- **RANDOM_NUMBER** must give the same sequence for the same seed on every host
  (as `MTH$RANDOM` does).

## Open questions

1. New numeric routines in MTHRTL under govax's own names, or in FTNRTL? Proposed:
   FTNRTL (MTHRTL's vector stays VMS's).
2. RANDOM_SEED's seed size: one longword (MTH$RANDOM's) or more?

## Subtasks (provisional)

1. **Survey** (each intrinsic's form, cost, and test), for review.
2. **Character intrinsics** and run-time-length results.
3. **Numeric and kind= arguments.**
4. **The model functions** in every format.
5. **Bits and TRANSFER.**
6. **The subroutines**: DATE_AND_TIME, SYSTEM_CLOCK, RANDOM_NUMBER,
   RANDOM_SEED, MVBITS.
7. **Programs end to end**, every mode, gfortran's witness (but random numbers
   and times).
8. **Close-out**: FORTRAN-INTERNALS.md (section 27's table extended), MANUAL.md's
   Appendix B.

## Progress log

- **2026-11-17:** Planned with Phase 99.
