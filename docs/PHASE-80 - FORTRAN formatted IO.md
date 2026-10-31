# Phase 80 — FORTRAN formatted and list-directed I/O

**Status:** planned (2026-10-29). Needs Phase 79 (control flow, for
implied DO loops and ERR/END branches). Part of the FORTRAN program laid
out in `PHASE-74 - FORTRAN plan.md` (Decisions 12, 13, and 15).

## Goal

Everything about turning values into text and text into values, on the
terminal and in internal files:

- **FORMAT** and every edit descriptor in chapter 8: I, F, E, D, G, L,
  A, H, character constants, X, T, TL, TR, `/`, `:`, `$`, Q, P scale
  factors, BN and BZ, S, SP, SS, O and Z, repeat counts and nested
  groups, reversion, variable format expressions (`<n>`), and default
  field widths (Table 8-4);
- **run-time formats** in character variables, arrays, and expressions;
- **list-directed** input and output in full (7.2.1.2; Table 7-1's output
  formats are Phase 78's): value separators, repeat counts (`3*5`), null values, slash termination,
  complex and character input;
- **NAMELIST** input and output (4.10, 7.2.1.3, 7.3.1.3);
- **internal files** (READ and WRITE to CHARACTER variables and arrays),
  and ENCODE and DECODE (appendix A.1);
- **the terminal statements:** READ with a format and no unit, ACCEPT,
  TYPE, PRINT, and units 5 and 6 (to SYS$INPUT and SYS$OUTPUT);
- **carriage control** for formatted output (8.4, Table 8-5);
- **the error model:** ERR=, END=, IOSTAT=, the FOR$_ conditions
  signaled when there are none, the FOR message texts (Decision 15),
  and ERRSNS (D.4.3).

## What earlier phases leave in place

- **Phase 75:** FTNRTL's list-directed output, the I/O call protocol,
  the unit table, SYS$OUTPUT.
- **Phase 78:** list-directed output of every type (its Part A's
  corrections, accepted 2026-10-30).
- **LIBRTL:** OTS$CVT_T_F/D/G/H (text to floating), OTS$CVT_TI_L,
  TO_L, TZ_L, TL_L (text to integer), OTS$CVT_L_Tx, and
  `OTS$$CVT_x_T_R8` (floating to digits with FORTRAN's rounding).
- **RMS's FORTRAN carriage control** (`rms/ftncc.go`, Phase 72).

## Design notes

- **The format interpreter** is MACRO in FTNRTL: it parses a format's
  text into a compact list of operations once (cached in the FORMAT's
  control block, Decision 12) and walks it as items arrive. The
  compiler checks a FORMAT statement's syntax at compile time, so a
  malformed FORMAT is a compile-time error; a run-time format is parsed
  when used.
- **Item calls drive the format:** each `FTN$IO_ITEM` advances the
  format to the next data edit descriptor, doing the positional and
  literal ones on the way; `FTN$IO_END` finishes it (to a colon or the
  end).
- **Variable format expressions** (`<n>`) need the program's values at
  run time: the compiler emits a small routine per format that FTNRTL
  calls to evaluate them, the one place FTNRTL calls back into compiled
  code.
- **Errors:** FTNRTL records the first error in the statement's context
  and makes later item calls no-ops; `FTN$IO_END` returns it (with ERR,
  END, or IOSTAT) or signals it. With END= in an input statement with an
  implied DO, the compiled loop tests the status after each item.
- **FOR messages** go in a hand-written `internal/vmsdef` table, in our
  words, with FAO arguments for the unit and file.

## Open questions

- **The exact text of the FOR messages** and their FAO arguments: ours,
  since the User Manual isn't in the folder. A VMS probe of FORRTL's
  messages (a MACRO program calling `$GETMSG` for each FOR$_ value on
  VMS) would give VMS's texts, if the author would like them matched.
- **List-directed output's line length** and where records break.
- **G editing** (Table 8-2) and E's exponent forms at large exponents:
  checked against the manual's examples.

## Subtasks (provisional)

1. **Survey:** chapters 7 and 8, appendix A.1, D.4.3; the edit
   descriptors' rules as a table with the manual's examples as test
   cases.
2. **FORMAT's compile-time check** and its control block.
3. **The format interpreter:** parsing and the positional descriptors.
4. **Integer, logical, and character editing.**
5. **Floating editing:** F, E, D, G, P, and complex.
6. **List-directed input.**
7. **Internal files, ENCODE, DECODE.**
8. **NAMELIST.**
9. **The error model:** ERR/END/IOSTAT, FOR messages, signaling, ERRSNS.
10. **Terminal statements**, units 5 and 6, carriage control.
11. **End to end**, then **close-out** (FORTRAN-INTERNALS.md's chapter
    on FTNRTL).

## Progress log

- **2026-10-29:** Planned with Phase 74.
