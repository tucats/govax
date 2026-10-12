# Phase 54 — LIB$ strings, descriptors, and text conversion

**Status:** planned (2026-10-12). Needs Phase 53. Part of the LIB$
program laid out in `PHASE-53 - RTL LIB support.md` (Part A), which also
has the sources and the conventions (Part B) this phase follows.

## Goal

The LIB$ routines that copy, allocate, and examine strings by
descriptor; character and numeric text conversion; and the locale
routines that read a logical name. With them, the dynamic string becomes
a real RTL object. Phase 53's output helper allocates one with
LIB$SGET1_DD's own code, so every routine that returns a string, in this
phase and later ones, shares one allocator.

This phase also brings two new kinds of transfer-vector entry:

- **JSB entry points.** `LIB$SCOPY_DXDX6`, `LIB$SCOPY_R_DX6`,
  `LIB$SGET1_DD_R6`, `LIB$SFREE1_DD6`, `LIB$SFREEN_DD6`, and
  `LIB$ANALYZE_SDESC_R2` take their arguments in registers (the manual:
  "with R0 containing the first argument ...") and return with RSB. A
  CALLS stub's entry mask, reached by JSB, would run as a HALT
  (`machine/shim.go`'s note on `kernelLinks`). They need a stub without
  an entry mask that keeps the registers the routine doesn't return in,
  ending in RSB, and a way for the Go function to read its arguments
  from registers rather than an argument list.
- **Data.** `LIB$AB_ASC_EBC`, `LIB$AB_EBC_ASC`, and `LIB$AB_UPCASE`
  (offsets 0x000, 0x100, and 0x200) are 256-byte translation tables, not
  routines. Their `SHIM$LIBRTL_<offset>` symbols must name the tables
  themselves, laid down in system space with the stubs.

## Routines

| Group | Routines (5.0 manual page) |
| --- | --- |
| Dynamic strings | SGET1_DD (350), SFREE1_DD (347), SFREEN_DD (348), and their JSB entries SGET1_DD_R6, SFREE1_DD6, SFREEN_DD6 |
| Copying | SCOPY_DXDX (336), SCOPY_R_DX (338), and JSB entries SCOPY_DXDX6, SCOPY_R_DX6 |
| Descriptors | ANALYZE_SDESC (10), ANALYZE_SDESC_R2 (JSB) |
| Characters and search | LEN (257), CHAR (25), ICHAR (227), INDEX (229) |
| Translation | TRA_ASC_EBC (453), TRA_EBC_ASC (457); tables AB_ASC_EBC, AB_EBC_ASC, AB_UPCASE |
| Numeric text | CVT_DTB, CVT_HTB, CVT_OTB (76, "LIB$CVT_xTB") |
| Keywords | LOOKUP_KEY (261) |
| Locale | DIGIT_SEP (120), RADIX_POINT (313), CURRENCY (59), LP_LINES (267): SYS$DIGIT_SEP, SYS$RADIX_POINT, SYS$CURRENCY, SYS$LP_LINES, with the manual's defaults |
| LIBRTL2 | CVT_DX_DX (61): general data-type conversion between descriptors; needs LIBRTL2.EXE's offset (Phase 53, subtask 13) |

## What earlier phases leave in place

- Phase 53's `descriptor.go`: input of every class, S/VS/D output,
  LIB$_STRTRU, LIB$_INVSTRDES.
- The heap: `corevms.Environment.AllocateVM`/`FreeVM`, shared with
  DECC$MALLOC and LIB$GET_VM. Phase 59 gives it zones; this phase
  allocates dynamic strings as one zone's blocks if 59's design is
  already known, and from the default heap otherwise.
- The STR$ facility: only STR$UPCASE is emulated. The STR$ manual is a
  separate book (`OVMS_73_RTL_STR.PDF`, 7.3; `AA-LA75A-TE`, 5.0), and its
  routines are out of this program's scope. They'd share this phase's
  dynamic-string code when they come.

## Open questions

- **JSB stubs**: how the shim learns its arguments (the XFC handler reads
  R0–R6 directly, flagged in the routine's table row) and which registers
  the routine must preserve (the manual's "R0 to R6 ... are used"). Decide
  in the first subtask, and record the JSB linkage in Phase 53's Part B.
- **The EBCDIC tables**: which ASCII–EBCDIC mapping the manual's tables
  are (the manual has the tables in TRA_ASC_EBC's entry). Transcribe them
  from the PDF, and check against a VMS dump of `LIB$AB_ASC_EBC` if the
  probe runs.
- **CVT_DX_DX** is large (every data type to every other). It might go
  to its own phase, or be done only for the integer and text types.

## Subtasks (provisional)

1. **Survey**: each routine's format, arguments, and condition values;
   the JSB entries' register conventions.
2. **JSB stubs and data entries** in the stub area and the table.
3. **Dynamic strings**: SGET1_DD, SFREE1_DD, SFREEN_DD (and JSB forms);
   Phase 53's D-class output moved onto them.
4. **Copying and descriptors**: SCOPY_DXDX, SCOPY_R_DX, ANALYZE_SDESC (and
   JSB forms).
5. **Characters**: LEN, CHAR, ICHAR, INDEX.
6. **Translation**: the three tables, TRA_ASC_EBC, TRA_EBC_ASC.
7. **Numeric text**: CVT_DTB, CVT_HTB, CVT_OTB.
8. **LOOKUP_KEY**.
9. **Locale**: DIGIT_SEP, RADIX_POINT, CURRENCY, LP_LINES.
10. **CVT_DX_DX** (if its offset is known, and in the scope decided).
11. **The program** (`testdata/lib54/`) and its console test; VMS probe
    with the author if wanted.
12. **Close-out**.

## Progress log

- **2026-10-12** — Planned (Phase 53, Part A).
