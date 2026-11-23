# Phase 114 — FORTRAN CONVERT=

**Status:** planned 2026-11-22, by the author's answer 5 at the review of the errata survey
(`PHASE-113 - Fortran errata.md`, A.1.1 and "The author's answers"). A skeletal doc:
subtask 1 is the survey, reviewed before any implementation. Needs Phases 81 and 113.

## Goal

OPEN's `CONVERT=` other than `'NATIVE'`, today FOR's NOTIMP when the OPEN is done
(`internal/ftnrtl/mar/ftn_open.mar`; MANUAL.md's 14.7): FTNRTL converting unformatted
records' numbers between the VAX's formats and the others VAX FORTRAN names, on reading and
writing, in sequential, direct, and keyed I/O; the compiler's checks of the keyword, and
INQUIRE's answer.

## Sources

The VAX FORTRAN Language Reference Manual (1988) and the HP Fortran manual for the
keywords; VMS 7.1's FORRTL.EXE, run as a black box (Phase 80's survey's rule), for what
each conversion writes, through a probe like `testdata/for81`'s.

## Open questions (for subtask 1)

1. Which conversions V5.0's FORRTL has (the HP manual lists more than VAX FORTRAN had), and
   which this phase does.
2. IEEE's values the VAX can't hold (infinities, NaNs, denormals, numbers past F's range):
   what FORRTL does with each, on reading and on writing.
3. Whether `vaxfloat` holds the conversions, as for the CPU's, or FTNRTL's MACRO does them.

## Subtasks (provisional)

1. **The survey**, with the probe on VMS 7.1's FORRTL.
2. Onward: as its review decides.

## Progress log

- **2026-11-22:** Planned at the review of Phase 113's survey (then Phase 112's), the author's
  answer 5: `CONVERT=` a phase of its own.
