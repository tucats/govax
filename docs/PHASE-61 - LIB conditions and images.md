# Phase 61 — LIB$ conditions, faults, and image routines

**Status:** planned (2026-10-12). Needs Phases 53 and 58. Part of the
LIB$ program laid out in `PHASE-53 - RTL LIB support.md` (Part A).

## Goal

The rest of the 5.0 manual's chapter 4 (condition handling) and its
chapter 7 (image initialization and termination), with the routine that
activates a shareable image at run time:

- **Handlers the RTL supplies**: SIG_TO_STOP (a handler that makes a
  signal a stop). FIXUP_FLT (a handler that replaces a floating reserved
  operand and continues). DECODE_FAULT (a handler's aid that decodes the
  faulting instruction's operands and calls the program's action
  routine). SIM_TRAP (turns floating-point faults into the traps older
  processors gave).
- **Enabling and disabling** overflow and underflow detection for the
  caller's frame: INT_OVER, DEC_OVER, FLT_UNDER. Each sets the PSW bits
  the caller's frame restores, as the manual's section 4.4.2 describes.
- **GET_OPCODE** (7.x): the original opcode at a breakpoint address.
  Only if its interface is settled.
- **FIND_IMAGE_SYMBOL**: activate a shareable image by name (if it isn't
  already) and return a universal symbol's value. This needs govax's
  image loader to activate a shareable image while a program runs, not
  only at RUN, and to merge it into the process's image list.
- **LIB$INITIALIZE** (chapter 7): govax runs dependencies' LIB$INITIALIZE
  entries already (Phase 13, RUN/INIT, SET DEBUG LIBINIT). This phase
  compares that with the manual's description (the initialization
  argument list, the LIB$INITIALIZE psect of routine addresses,
  co-routine calls) and closes the gaps.
- **Review of the existing condition routines**: SIGNAL, STOP, ESTABLISH,
  REVERT, SIG_TO_RET, and MATCH_COND came from eVAX (Phases 26 and 34).
  Each is checked against the manual's entry, and DEVIATIONS.md's
  entries on them (for example LIB$SIGNAL's mechanism-array R0) are
  settled.

## Routines

| Group | Routines (5.0 manual page) |
| --- | --- |
| Handlers | SIG_TO_STOP (372), FIXUP_FLT (165), DECODE_FAULT (86, LIBRTL2), SIM_TRAP (374, LIBRTL2) |
| Detection | INT_OVER (255), DEC_OVER (104), FLT_UNDER (167) |
| Images | FIND_IMAGE_SYMBOL (160), LIB$INITIALIZE (chapter 7), GET_OPCODE (7.x) |
| Review | SIGNAL (365), STOP (394), ESTABLISH (140), REVERT (331), SIG_TO_RET (369), MATCH_COND (272) |

## What earlier phases leave in place

- The condition dispatcher (`corevms/condition.go`, `signal.go`,
  `unwind.go`) and the console's fallback (`machine/chf.go`).
- The image loader (`machine/image.go`, `images.go`): ICBs, G^ and
  .ADDRESS fixups, RUN/INIT's LIB$INITIALIZE calls (`machine/run.go`).
- `internal/disasm`, which DECODE_FAULT can use to find operands.
- `vaxfloat` for FIXUP_FLT's replacement values.

## Open questions

- **FIND_IMAGE_SYMBOL** needs real shareable images govax can load.
  Which does govax have besides its own emulated LIBRTL and DECC$SHR?
  A program's own shareable image (LINK/SHAREABLE) is the useful case.
  Does govax's LINK make one?
- **DECODE_FAULT** is long in the manual (LIB-86 to LIB-103) and works on
  the instruction stream and registers at a fault. It may need the
  engine's exception details beyond what the signal array carries.
- **DECODE_FAULT and SIM_TRAP** need LIBRTL2.EXE's offsets (Phase 53,
  subtask 13).

## Subtasks (provisional)

1. **Survey**: chapter 4's routines, chapter 7, FIND_IMAGE_SYMBOL.
2. **Review of the existing condition routines.**
3. **SIG_TO_STOP, INT_OVER, DEC_OVER, FLT_UNDER.**
4. **FIXUP_FLT.**
5. **DECODE_FAULT, SIM_TRAP** (if their offsets are known).
6. **LIB$INITIALIZE** against chapter 7.
7. **FIND_IMAGE_SYMBOL**: run-time activation of a shareable image.
8. **The program** (`testdata/lib61/`), its console test, and the VMS
   probe.
9. **Close-out**: the LIB$ program's summary in PLAN.md.

## Progress log

- **2026-10-12** — Planned (Phase 53, Part A).
