# Phase 56 — LIB$ date and time

**Status:** planned (2026-10-12). Needs Phases 53 and 54. Part of the
LIB$ program laid out in `PHASE-53 - RTL LIB support.md` (Part A).

## Goal

The 5.0 manual's chapter 3 and the date/time routines of its reference
section, in two parts:

- **Arithmetic and conversion**: adding, subtracting, and scaling
  times (the VMS 64-bit internal format: 100 ns units since
  17-Nov-1858, deltas negative), converting between internal times and
  operations' values (CVT_TO/FROM_INTERNAL_TIME and their F-floating forms,
  with the manual's operation codes: LIB$K_DELTA_SECONDS,
  LIB$K_DAY_OF_YEAR, ...), the seven-word vector (CVT_VECTIM), and the
  simple routines DATE_TIME, DAY, and DAY_OF_WEEK.
- **Formatting** (the manual's section 3.3): FORMAT_DATE_TIME and
  CONVERT_DATE_STRING with a context (INIT_DATE_TIME_CONTEXT,
  FREE_DATE_TIME_CONTEXT). Formats come from the logical names
  LIB$DT_FORMAT and LIB$DT_INPUT_FORMAT and the language's
  LIB$DATE_FORMAT_nnn and LIB$TIME_FORMAT_nnn, or are set at run time by
  mnemonic. Also GET_DATE_FORMAT and GET_MAXIMUM_DATE_LENGTH, and
  SYS$LANGUAGE. VMS defines these names in a startup procedure
  (LIB$DT_STARTUP), so govax will need a small definition set of its own,
  for English and the predefined formats.

## Routines

| Group | Routines (5.0 manual page) |
| --- | --- |
| Arithmetic | ADD_TIMES (5), SUB_TIMES (397), MULT_DELTA_TIME (297), MULTF_DELTA_TIME (298) |
| Conversion | CVT_TO_INTERNAL_TIME (72), CVTF_TO_INTERNAL_TIME (74), CVT_FROM_INTERNAL_TIME (67), CVTF_FROM_INTERNAL_TIME (70), CVT_VECTIM (78) |
| Simple | DATE_TIME (80), DAY (82), DAY_OF_WEEK (84) |
| Formatting | INIT_DATE_TIME_CONTEXT (231), FREE_DATE_TIME_CONTEXT (172), FORMAT_DATE_TIME (169), CONVERT_DATE_STRING (27), GET_DATE_FORMAT (204), GET_MAXIMUM_DATE_LENGTH (216) |

## What earlier phases leave in place

- `vmsdef/time.go`: the internal time format and its conversions;
  `$GETTIM`, `$NUMTIM`, `$ASCTIM`, `$BINTIM` (`corevms`); the quantum and
  host clocks (`vax.hardware.clock`).
- DCL's F$CVTIME and F$TIME (`dclshell/dcllextime.go`), whose parsing of
  absolute and delta times the conversion routines can share.
- `vaxfloat` for the F-floating forms.
- Logical names (`internal/lnm`) for the format names.

## Open questions

- **The formatting logical names**: VMS's startup procedure defines them
  (in LNM$DT_FORMAT_TABLE). govax needs its own definitions, made from the
  manual's tables of mnemonics and predefined formats (clean room). Do
  they belong in `vax.init`, or are they built in?
- **Languages**: English only, or the manual's other predefined
  languages? Proposed: English, the table built so others can be added.
- **CONVERT_DATE_STRING's defaults** for missing fields (the manual's
  defaults argument and flags) and its error statuses are subtle; a probe
  is worth it.

## Subtasks (provisional)

1. **Survey**: chapter 3 and each routine's entry; the operation codes;
   the format mnemonics.
2. **Arithmetic**: ADD_TIMES, SUB_TIMES, MULT_DELTA_TIME, MULTF_DELTA_TIME.
3. **Conversion**: CVT(F)_TO/FROM_INTERNAL_TIME, CVT_VECTIM.
4. **Simple**: DATE_TIME, DAY, DAY_OF_WEEK.
5. **Format context and logical names**: INIT/FREE_DATE_TIME_CONTEXT, the
   names' definitions.
6. **Output**: FORMAT_DATE_TIME, GET_DATE_FORMAT, GET_MAXIMUM_DATE_LENGTH.
7. **Input**: CONVERT_DATE_STRING.
8. **The program** (`testdata/lib56/`), its console test, and the VMS
   probe.
9. **Close-out**.

## Progress log

- **2026-10-12** — Planned (Phase 53, Part A).
