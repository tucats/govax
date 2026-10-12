# Phase 57 — LIB$ access to the command language interpreter

**Status:** planned (2026-10-12). Needs Phases 53–55, and Phases 48,
50, and 51 (DCL in every process). Part of the LIB$ program laid out in
`PHASE-53 - RTL LIB support.md` (Part A).

## Goal

The routines of the 5.0 manual's section 2.2: a program working with
the DCL that ran it. Symbols and supervisor-mode logical names; the
command line (GET_COMMAND, with GET_FOREIGN already done); chaining,
where DO_COMMAND and RUN_PROGRAM end the image and have DCL run a command
or another image; the CLI's common area (GET_COMMON, PUT_COMMON), which
keeps text from one image to the next; control-key interception
(DISABLE_CTRL, ENABLE_CTRL); and moving the terminal between processes
(ATTACH) or back to DCL for a moment (PAUSE).

Every one of these runs in a process whose DCL is govax's own: process
1's, or a subprocess's CLI (`internal/dclshell/subcli.go`). With no CLI
(a `$CREPRC` image), they return LIB$_NOCLI as the manual says. This
phase is mostly about the boundary between `internal/librtl` and
`internal/dclshell`. On VMS these routines go through the CLI callback
($CLI); govax needs an interface `corevms` can call that each process's
DCL implements, as `Machine.Hooks` does for the machine.

## Routines

| Group | Routines (5.0 manual page) |
| --- | --- |
| Symbols | GET_SYMBOL (219), SET_SYMBOL (343), DELETE_SYMBOL (116) |
| Logical names | SET_LOGICAL (340), DELETE_LOGICAL (114) |
| Command line | GET_COMMAND (199) |
| Chaining | DO_COMMAND (124), RUN_PROGRAM (332) |
| Common area | GET_COMMON (202), PUT_COMMON (309) |
| Control keys | DISABLE_CTRL (122), ENABLE_CTRL (138) |
| Terminal | ATTACH (17), PAUSE (299) |

## What earlier phases leave in place

- DCL's symbol tables (`dclshell/dclsym.go`: global and per-level local),
  and how a subprocess gets its parent's symbols (LIB$SPAWN, Phase 48).
- Logical names, supervisor mode, the process table (`internal/lnm`).
- LIB$GET_FOREIGN's `Environment.CommandLine`, LIB$GET_INPUT's terminal
  reads, and SYS$COMMAND (`dclshell/dclinput.go`).
- `$CLI` (`corevms/cli.go`), SET [NO]CONTROL and Ctrl/Y and Ctrl/C
  handling (`dclshell/dclcontroly.go`, Phase 50, subtask 18).
- SPAWN, ATTACH-style terminal handoff (`corevms.Environment.Attached`,
  Phase 51).

## Open questions

- **The CLI interface**: one Go interface for symbols, logicals (or
  `lnm` directly), the common area, chaining, and control keys, set by
  `dclshell` on each process's Environment. Shape decided in the survey.
- **Chaining**: the image exits (its exit handlers run), then DCL runs
  the command as if typed, in the procedure level that ran the image.
  DCL's `$STATUS` and verify echo are what the probe should check.
- **ATTACH and PAUSE** at the console: PAUSE returns to DCL with the
  image suspended (DCL's CONTINUE resumes it, as after Ctrl/Y). ATTACH
  moves the terminal to another process of the job. How each looks
  from govax's console is to be settled.
- **The common area**: the manual describes a fixed-size area (252
  bytes, to be checked in the survey) kept by the CLI between images.
  govax's P1 layout has no place for it yet, and it could equally be kept
  in the process's `Shell`. Decided in the survey.

## Subtasks (provisional)

1. **Survey**, and the CLI interface's design.
2. **Symbols**: GET_SYMBOL, SET_SYMBOL, DELETE_SYMBOL (local and global
   tables; LIB$_NOSUCHSYM, LIB$_NOCLI).
3. **Logical names**: SET_LOGICAL, DELETE_LOGICAL.
4. **GET_COMMAND.**
5. **The common area**: GET_COMMON, PUT_COMMON.
6. **Chaining**: DO_COMMAND, RUN_PROGRAM.
7. **Control keys**: DISABLE_CTRL, ENABLE_CTRL.
8. **ATTACH and PAUSE.**
9. **The program** (`testdata/lib57/`, a command procedure running it),
   its console test, and the VMS probe.
10. **Close-out**.

## Progress log

- **2026-10-12** — Planned (Phase 53, Part A).
