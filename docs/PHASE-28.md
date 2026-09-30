# Phase 28: The MACRO-32 macro facility

## Goal

Add MACRO-32's macro facility to the MACRO dialect of `internal/asm`, so that
ordinary VMS MACRO programs assemble with govax's `MACRO` command. Almost
every real program calls system macros (`$EXIT_S`, `$QIOW_S`, `$FAB`, `$RAB`,
and so on) from `SYS$LIBRARY:STARLET.MLB`. Without the macro facility, govax
can only assemble programs that call `SYS$...` entry points directly, as the
Phase 27 fixtures do.

Split out of Phase 27's "later sub-phases" (docs/PHASE-27.md, subtask 12).

**Status: planned, not started.**

## Scope

- Macro definitions: `.MACRO`/`.ENDM`, with positional and keyword
  arguments, default values, created local labels (`?label`), string
  concatenation (`'`), and `.NARG`/`.NCHR`.
- Repeat blocks: `.IRP`, `.IRPC`, `.REPEAT`/`.REPT`, and `.MEXIT`.
- Macro libraries: `.MCALL`, `.LIBRARY`, and MACRO's automatic search of
  `STARLET.MLB` for an undefined macro name.
- The console dialect is unchanged. Whether it should also get macros is an
  open question.

## What Phase 27 leaves in place

- `internal/asm` assembles in one pass, a statement at a time
  (`assembleLines`). Macro expansion fits in front of that: an expanded
  macro's lines go through the same statement path, the way `.INCLUDE`'s
  do, with `includeLines`-style nesting for error locations.
- In the MACRO dialect, assembly reports every error
  (`asm.Errors`). Expansion errors should name the macro call's line.
- `.INCLUDE` resolution across host and ODS-2 files
  (`rms.Session.LocateRelated`) is the model for finding a `.LIBRARY`
  file.

## References

- *VAX MACRO and Instruction Set Reference Manual* (OpenVMS VAX 7.3),
  chapter 4 (macro arguments and string operators) and the macro
  directives in chapter 6.
- `vmssrc_archive/v73/lbr/lis/` and `librar/lis/`: the librarian, whose
  listings and `lbr.sdl` describe the `.MLB` file format.
- `vmssrc_archive/v73/starlet/lis/*.mar`: some STARLET macro sources. The
  system-service macros (`$EXIT_S` and so on) aren't in the archive as
  source; the real `STARLET.MLB` from the user's VAX has them.

## Open questions

- **Where the system macros come from.** Read the real `STARLET.MLB`
  (copied from the VAX, and gitignored as licensed material), which needs
  a reader for the librarian's file format, or generate the macros govax
  needs? The first is faithful; the second avoids a new file format.
- **Should the console dialect get macros too?**
- **Fixtures.** Real MACRO's objects and listings for a set of macro
  fixtures, from the user's VAX as in Phase 27.

## Subtasks

To be written when the phase starts.

## Progress Log

### 2026-09-30 — Planned

- Created from Phase 27's "later sub-phases" when that phase closed.
