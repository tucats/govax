# Phase 77 — MTHRTL: VMS's mathematics library, emulated

**Status:** planned (2026-10-29). Needs Phase 74 (the plan) only: this
phase is independent of the compiler and could be done at any time
before Phase 78. Part of the FORTRAN program laid out in `PHASE-74 -
FORTRAN plan.md` (Decision 14).

## Goal

Add VMS's mathematics run-time library, MTHRTL.EXE, to govax's RTL
support, as LIBRTL is: Go routines a program reaches through stubs, for
**any** program (MACRO, govax's FORTRAN, or an image VMS's compilers
made), not part of FTNRTL. This was the author's direction
(2026-10-29). It covers:

- the **MTH$ routines** in the VMS 5.0 *MTH$* manual, each in the
  floating formats it has (F, D, G, H): square root, exponential and
  logarithms, the trigonometric functions in radians and degrees and
  their inverses, the hyperbolic functions, ATAN2, the complex
  functions (CABS, CEXP, CLOG, CSQRT, CSIN, CCOS, and the D and G
  forms), the random number generator (MTH$RANDOM), and the
  integer and bit functions FORTRAN names as MTH$ routines;
- the **OTS$ routines** the compiler needs that VMS keeps with them:
  the powers (OTS$POWxx for every base and exponent type pair) and
  complex division (OTS$DIVC and its D and G forms);
- **their conditions** as the manual gives them (MTH$_LOGZERNEG,
  MTH$_SQUROONEG, MTH$_UNDEXP, MTH$_FLOOVEMAT, MTH$_INVARGMAT, ...),
  signaled with the manual's arguments;
- **JSB entry points** (`MTH$SIN_R4` and the like) where the manual
  documents them, through Phase 54's JSB shim mechanism.

## What earlier phases leave in place

- **`internal/librtl`** (Phases 34, 53–59) is the model: a routine
  table, `XFC$SHIM` codes, stubs placed by `EnsureShims`, JSB entries
  (`JSBRoutines`), missing-routine stubs, and descriptor helpers.
- **`machine.runtimeLibraries`** already lists MTHRTL, so programs
  linked against it activate, with every routine resolving to a
  NOTIMPL stub today.
- **`vmsdef`** has the MTH$ condition values and 15 MTH message texts,
  and `vmsdef/gen -image` captures a shareable image's global symbol
  table.
- **`internal/vaxfloat`** unpacks and packs every format, and rounds
  exactly.

## Design notes

- **A package of its own**, `internal/mthrtl`, registered beside
  `internal/librtl`. Its routines take their arguments by the manual's
  mechanisms and return F and integer results in R0 and D and G results
  in R0:R1. Per the calling standard, H and D/G complex results come
  back through a first argument.
- **Accuracy:** F and G values fit exactly in a float64, so those forms
  compute with Go's `math` and round once with `vaxfloat`. D (56-bit
  fraction) and H (113-bit) are computed with `math/big` at enough
  precision (argument reduction and series), then rounded. Results
  should be within one ulp of the true value. The tests compare each
  form against a high-precision reference.
- **The vector** (the author's answer to Phase 74's review question 2):
  MTHRTL's global symbol table is captured from VMS 7.1's MTHRTL.EXE on
  the local system disk (`testdata/disks/rq0-ra92.dsk`) with
  `vmsdef/gen -image`, so each routine's offset is VMS's and images
  VMS's compilers made call the right routine. LINK then gets MTHRTL's
  symbols from govax's tables, as it does LIBRTL's.
- **Which library holds the OTS$ powers** on VMS 7.1 (MTHRTL or
  LIBRTL) is settled by the captured tables; the routine goes where VMS
  put it.

## Open questions

- **The exact set:** the 5.0 manual's routines, or 7.3's too
  (`OVMS_73_RTL_MTH.PDF`)? Start with 5.0, add 7.3's when a probe or
  program asks.
- **MTH$RANDOM's sequence:** the manual documents the generator (a
  linear congruential one), so VMS's sequence can be matched exactly;
  FORTRAN's RAN uses it.
- **A VMS probe:** a MACRO program calling each routine with edge
  arguments, run on VMS 7.1, would confirm conditions and the
  last-bit results. Optional, since exact last bits aren't a goal.

## Subtasks (provisional)

1. **Survey and capture:** the MTH$ and OTS$ manuals' routine lists,
   arguments, entry points, and conditions; MTHRTL.EXE's global symbol
   table captured into `vmsdef` (the image itself is licensed and stays
   local).
2. **The package and its registration**, with the stubs, LINK's symbols,
   and the missing-routine stubs.
3. **Square root, exponentials, logarithms** in all four formats.
4. **Trigonometric and inverse functions**, radians and degrees.
5. **Hyperbolic functions, ATAN2, and the remaining real functions.**
6. **Complex functions.**
7. **OTS$ powers and complex division.** Phase 76's folding
   (`sema/constant.go`'s `power`, `complexArith`) squares repeatedly,
   rounding each product, from the reciprocal for a negative exponent, and
   rounds a complex quotient's parts once; unconfirmed. These routines and
   it should agree, whichever changes.
8. **JSB entries, MTH$RANDOM, and the conditions.**
9. **Tests from MACRO** programs, and the optional VMS probe.
10. **Close-out:** CLAUDE.md, MANUAL.md's RTL section, HELP.

## Progress log

- **2026-10-29:** Planned with Phase 74. The author directed that the
  MTH$ functions be general RTL support, usable outside FORTRAN, and
  approved capturing VMS 7.1's MTHRTL vector.
