# Phase 10: RTL simulators

## Goal

Emulate the VMS runtime-library/system-service calling convention on top of the CPU:
argument-list marshaling, RMS file I/O, the CLI, and the LIB$ math/string/time/print/
memory utility routines. This is what lets real VMS `.exe` fixtures (`testdata/exe/`)
run.

## Scope / C source mapping

- `reference/eVAX/eVAX/Source/RTL/p1_vector.c`, `shim.c` — dispatch `SYS$`/`LIB$`
  system-service calls by reading an argument-list vector out of emulated VAX memory.
  This is the foundational piece the rest of this phase builds on.
- `reference/eVAX/eVAX/Source/RTL/service.c` — individual `SYS$` services.
- `reference/eVAX/eVAX/Source/RTL/cli.c` — command-line-interface emulation.
- `reference/eVAX/eVAX/Source/RTL/rms.c` + `structure_mapping.c` — RMS file I/O,
  mapping FAB/RAB struct fields (`reference/eVAX/eVAX/Headers/fab.h`/`rab.h`) onto VAX
  memory via a declarative offset table (`structure_mapping.c`'s `map()`, `STROFF`
  macro in `memmap.h`). `AUDIT.md` fixed FAB/RAB native-pointer fields to store VAX
  addresses (not host pointers) — the Go port should use a VAX-address type
  consistently here from the start rather than repeat that class of bug.
- `reference/eVAX/eVAX/Source/RTL/librtl_file.c`, `librtl_input.c`, `librtl_math.c`,
  `librtl_memory.c`, `librtl_print.c`, `librtl_strings.c`, `librtl_time.c`,
  `librtl_utils.c` — individual LIB$ routines.

## Deliverables

- `internal/rtl` package implementing the calling convention and the library routines
  above, depending on `internal/io` (Phase 09) for device/logical-name resolution.
- Tests: unit tests per LIB$ routine where feasible, plus RMS round-trip tests (create/
  write/read/close a file through the emulated FAB/RAB layer).
- This phase is the last blocker before `testdata/exe/put.exe`, `putc.exe`, `cli.exe`,
  `sieve.exe`, `simple.exe` (the fixtures `AUDIT.md` calls "working") can run
  end-to-end — a good milestone check before moving to Phase 11.

## Open questions / notes

- The string-symbol native-pointer-truncation bug class `AUDIT.md` fixed (N1: adding a
  proper `svalue` field instead of casting through `LONGWORD`) is a general pattern to
  watch for anywhere this phase needs to carry a host-side string/pointer alongside a
  VAX-side numeric value — Go's type system makes the two impossible to conflate by
  construction, but keep the underlying scenario in mind when structuring RTL argument
  types.

## Progress Log

_Not started._
