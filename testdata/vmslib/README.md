# Local-only VMS library files

This directory holds library files copied from a real VAX/VMS 7.3 system, for
govax's LINK to read (docs/PHASE-30.md, subtask 3):

- `imagelib.olb`: `SYS$LIBRARY:IMAGELIB.OLB`, the shareable image library,
  holding each shareable image's global symbol table.
- `starlet.olb`: `SYS$LIBRARY:STARLET.OLB`, the system object library.
- `librtl.exe`: `SYS$SHARE:LIBRTL.EXE`, the run-time library image.

They are licensed VMS files, so nothing here but this README is committed
(see `.gitignore`). They're copied as raw blocks (`COPY/BINARY`). Tests that
use them skip when they aren't present.
