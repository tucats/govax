# Phase 58 — LIB$ callbacks into the program, and file routines

**Status:** planned (2026-10-12). Needs Phases 53 and 54. Part of the
LIB$ program laid out in `PHASE-53 - RTL LIB support.md` (Part A).

## Goal

Two things that come together here because the file routines are the
first to need the first:

- **Calling back into the program.** Several LIB$ routines call
  routines the program supplies: FILE_SCAN's success and error routines,
  DELETE_FILE's and RENAME_FILE's success, error, and confirm routines,
  and later the tree routines' compare and action routines (Phase 60),
  TPARSE's action routines (Phase 60), and a user zone's allocation
  routines (Phase 59). A LIB$ routine written in Go must be able to call
  a VAX routine, get its R0, and go on, perhaps many times in one call.
  The pattern exists for services: a service returns a
  `corevms.CallRequest`, the engine calls the routine with the service's
  XFC as the return address, and the service runs again to continue
  (`$CMKRNL`, `cmode.go`; exit handlers, `exit.go`; the condition
  dispatcher). This phase generalizes it for shims. A shim keeps its
  state across the calls it makes in a per-call record, found again by
  frame and stack pointer, as `cmodeCall` is. The engine allows a shim
  to return a CallRequest. Nesting works too: a callback that calls the
  same LIB$ routine. The routine's Go code should read as a loop, not
  a hand-written state machine, so the design should look at whether a
  goroutine handoff (as `dclshell/subcli.go` does for the subprocess CLI)
  or an explicit continuation is simpler to keep correct.
- **Files.** FIND_FILE and FIND_FILE_END: wildcard search with a
  context, over RMS's `$PARSE`/`$SEARCH` (Phase 33). FILE_SCAN and
  FILE_SCAN_END: the same through a FAB and NAM the program supplies,
  calling its routines. DELETE_FILE and RENAME_FILE, with wildcards,
  related and default names, and the callbacks. FID_TO_NAME: a file ID to
  a file name, through the directory back-links, which ods2 can read.
  TRIM_FILESPEC: fitting a file name to a width.

## Routines

| Group | Routines (5.0 manual page) |
| --- | --- |
| Search | FIND_FILE (155), FIND_FILE_END (159), FILE_SCAN (151), FILE_SCAN_END (153) |
| Delete and rename | DELETE_FILE (106), RENAME_FILE (319) |
| Names | FID_TO_NAME (149), TRIM_FILESPEC (461) |

## What earlier phases leave in place

- `CallRequest` and its users (above); `corevms.Environment.Shim`
  (`environment.go`), which today passes a shim's error through without
  looking for a CallRequest.
- RMS: `$PARSE`, `$SEARCH` (with VMS's two kinds of wildcard context),
  `$ERASE`, `$RENAME`, NAM blocks, related names (`internal/rms`,
  Phases 24, 33, 47). DCL's DELETE and RENAME (`internal/dclcmd`) for
  how govax already words their messages.
- `LIB$CREATE_DIR` (Phase 34), the existing LIB$ file routine, for its
  conventions.

## Open questions

- **The callback mechanism's shape** (above): continuation versus
  handoff. Decided in subtask 2, with a test that calls back from a
  callback.
- **A condition in a callback**: the program's handlers and a `$UNWIND`
  past the LIB$ routine's frame must leave the shim's saved state
  cleaned up, as the condition dispatcher's own stack is.
- **FIND_FILE's flags** (multiple-file context, NODEFAULTS) and the
  context's lifetime; and FID_TO_NAME on a file with no back-link.

## Subtasks (provisional)

1. **Survey.**
2. **Callbacks from a shim**: design, the engine's side, nesting,
   unwinding; a test routine that calls back.
3. **FIND_FILE, FIND_FILE_END.**
4. **FILE_SCAN, FILE_SCAN_END.**
5. **DELETE_FILE.**
6. **RENAME_FILE.**
7. **FID_TO_NAME, TRIM_FILESPEC.**
8. **The program** (`testdata/lib58/`, on a volume), its console test,
   and the VMS probe.
9. **Close-out**: Phase 53's Part B gets the callback convention.

## Progress log

- **2026-10-12** — Planned (Phase 53, Part A).
