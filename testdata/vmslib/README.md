# Local-only VMS library files

This directory holds library files copied from a real VAX/VMS 7.3 system, for
govax's LINK (docs/PHASE-30.md, subtask 3) and MACRO (docs/PHASE-28.md) to
read:

- `imagelib.olb`: `SYS$LIBRARY:IMAGELIB.OLB`, the shareable image library,
  holding each shareable image's global symbol table.
- `starlet.olb`: `SYS$LIBRARY:STARLET.OLB`, the system object library.
- `librtl.exe`: `SYS$SHARE:LIBRTL.EXE`, the run-time library image.
- `starlet.mlb`: `SYS$LIBRARY:STARLET.MLB`, the system macro library
  (`$EXIT_S`, `$FAB`, and so on), from `[VMS$COMMON.SYSLIB]` on the VMS 7.3
  system disk.

They are licensed VMS files, so nothing here but this README is committed
(see `.gitignore`). They're copied as raw blocks (`COPY/BINARY`). Tests that
use them skip when they aren't present.
