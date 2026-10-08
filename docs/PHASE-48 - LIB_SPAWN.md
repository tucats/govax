# Phase 48 — Multiprocessing, part 6: LIB$SPAWN and the milestone

**Status:** planned (2026-10-06); decisions taken 2026-10-06 (see
PHASE-43.md, Part A). Not started. Needs Phases 43–47.

The program this phase belongs to is described in
[PHASE-43.md](PHASE-43.md), Part A. Read that first.

## Goal

Finish the program:

- **`LIB$SPAWN`**: a subprocess running a command, with the parent's
  symbols and logical names, waiting or not, reporting its completion
  status, event flag, and AST, as the LIB$ Reference Manual describes.
- **The milestone**: a MACRO-32 parent and child that exchange messages
  through mailboxes and both read and write files on an ODS-2 volume,
  without corrupting it, created both by `$CREPRC` and by `LIB$SPAWN`,
  checked by acceptance tests under several quanta.
- **The scheduler on by default** (Decision 5), and the documentation.

## What earlier phases leave in place

- Phases 43–47: processes, scheduling, `$CREPRC`, process deletion and
  termination mailboxes, jobs and logical-name tables, interprocess
  mailboxes, CEFs, global sections, RMS on mailboxes and `NL:`, the
  shared terminal, the lock manager, file sharing, the shared FCB.
- `internal/librtl` (Phase 34): LIB$ routines in Go, reached through
  shim stubs, each with its transfer-vector offset checked against
  LIBRTL's GST (`vmsdef.ImageSymbols`). `LIB$SPAWN`'s offset is there.
- DCL symbols and foreign commands (`internal/console/dclsym.go`, Phase
  36); `LIB$GET_FOREIGN` reading `Environment.CommandLine`.

## Design

### `LIB$SPAWN`

```text
LIB$SPAWN [command-string] [,input-file] [,output-file] [,flags]
          [,process-name] [,process-id] [,completion-status-address]
          [,byte-integer-event-flag-num] [,AST-address]
          [,varying-AST-argument] [,prompt-string] [,cli] [,table-name]
```

From the LIB$ manual: it creates a subprocess of the caller (as
`$CREPRC` does, through the same Go path), running the CLI; copies the
caller's DCL symbols (unless `CLI$M_NOCLISYM`) and process logical names
(unless `CLI$M_NOLOGNAM`); gives it `input-file`/`output-file` as
SYS$INPUT/SYS$OUTPUT (default: the caller's); names it (default: the
caller's name with `_n` appended, as the manual describes); and, unless
`CLI$M_NOWAIT`, waits until the subprocess ends. On its end the
subprocess's final status goes to `completion-status-address`, the event
flag is set, and the AST is queued. The caller's wait is an ordinary
Phase 44 wait (the parent is not computable meanwhile), and Ctrl-C/Ctrl-Y
handling while waiting follows the manual (`CLI$M_NOCONTROL` and the
others recorded). Errors as the manual lists them.

govax implements the completion through a termination mailbox of its own
(as VMS's does) or directly in Go on the subprocess's deletion; subtask 1
picks the clearer.

### The subprocess CLI (Decision 8)

The subprocess runs a small Go command interpreter, not a VAX image: the
"image" of a CLI process. On its first dispatch (Phase 45's process
startup), it executes `command-string`, or reads commands from SYS$INPUT
until end of file or `LOGOUT` when there's none. It understands:

- `RUN file` (with the console's RUN qualifiers that make sense);
- foreign commands: a DCL symbol whose value is `$image`, with the rest
  of the line as `LIB$GET_FOREIGN`'s text (Phase 36's rules);
- `MCR image [text]`, if wanted;
- `EXIT`/`LOGOUT`;
- anything else: the message VMS's DCL gives for an unknown verb
  (`%DCL-W-IVVERB`), status returned to the parent.

Each command activates an image in the subprocess (Phase 45's activator
through the subprocess's address space) and waits for it to exit, then
runs image rundown, as DCL does; the subprocess ends after the last
command, with the last image's status. The subprocess's image symbols
and image list are its own (Phase 43).

This is where `$CREPRC`'s `SYS$SYSTEM:LOGINOUT.EXE` (Phase 45) also
lands: a `$CREPRC` of LOGINOUT gets the same CLI, reading SYS$INPUT.

### The console's SPAWN (optional)

A `SPAWN [command]` console command (`/NOWAIT`, `/INPUT`, `/OUTPUT`,
`/PROCESS`) on the same path, run from process 1. Optional because, by
Decision 4, a `/NOWAIT` subprocess only runs while the console runs
something; included if it helps testing.

### The milestone programs

`testdata/mp/` (with a README, as `testdata/mar` has):

- `parent.mar`: mounts nothing itself (the test mounts a fresh container
  and sets the default directory); creates a mailbox for each direction
  and a termination mailbox; starts `child.exe` (by `$CREPRC`, or by
  `LIB$SPAWN` with `CLI$M_NOWAIT` and the command `RUN CHILD`, chosen by
  a foreign-command argument); then, in a loop, writes a numbered message,
  reads the child's reply, and checks it; appends a record per round to
  `SHARED.DAT` (opened with `FAB$V_SHRPUT!FAB$V_SHRGET`) and to its own
  `PARENT.DAT`; at the end sends a "done" message, reads the termination
  message (or the spawn's completion status), checks the child's status,
  reads back `SHARED.DAT` and counts each process's records, and prints a
  summary line.
- `child.mar`: assigns channels to the mailboxes by their logical names,
  replies to each message, appends to `SHARED.DAT` and `CHILD.DAT`, exits
  with a known success status after "done".
- Assembled and linked by govax's MACRO and LINK in the test, so the test
  needs no prebuilt images.

### The acceptance tests

In `internal/console` (consoletest), flag on:

- for each creation path ($CREPRC, LIB$SPAWN) and each of several quanta
  (very small, so switches land everywhere; medium; the default):
  run the parent; check its output line by line; check `SHARED.DAT` has
  every record of both processes, in an order each process's own records
  keep; check `PARENT.DAT` and `CHILD.DAT`; dismount and run ods2's volume
  analysis (no lost, free-but-used, or multiply allocated blocks;
  consistent headers, directories, and index file);
- the same through `govax run` on the command line (the CLI subcommand
  path, as Phase 36's tests do), with `--instruction-limit` as a guard;
- a negative test: with the flag off, `$CREPRC` and `LIB$SPAWN` report
  the unsupported status and the parent says so.

### Optional probe (Decision 7)

The same `parent.mar`/`child.mar` on VMS 7.3, with the output and the
files captured in `testdata/mp/vax/`. PIDs and times differ; the
messages, statuses, and file contents should match, and the test compares
them with PIDs and times masked.

## Subtasks

1. **`LIB$SPAWN`'s arguments and creation** in `internal/librtl`, on
   Phase 45's process creation; symbol and logical-name copying;
   completion status, event flag, AST; waiting. Tests in Go.
2. **The subprocess CLI**: RUN, foreign commands, MCR (if wanted),
   EXIT/LOGOUT, unknown verbs; command string or SYS$INPUT; LOGINOUT via
   `$CREPRC`. Tests.
3. **Console SPAWN** (optional).
4. **The milestone programs** and their README.
5. **The acceptance tests** as above.
6. **Probe** (optional) and the masked comparison.
7. **Scheduler on by default**: run the whole suite with the flag on;
   fix what differs; flip the default (Decision 5); `HELP CONFIG KEYS`.
8. **Documentation**: `CLAUDE.md` (the new packages and the process
   model), `PLAN.md` (the program's summary, as earlier multi-phase
   efforts have), `MODE-STACKS.md`, `PERFORMANCE.md` (context-switch
   cost), `DEVIATIONS.md` (every unconfirmed rule from Phases 43–48 in
   one place), HELP for every new command.
9. **Close-out** of Phase 48 and of the program.

## Open questions

- How the parent waits for a `NOWAIT` spawn's end in the milestone:
  termination mailbox via `$CREPRC`, completion event flag via
  `LIB$SPAWN`; both are tested.
- Whether to include DCL's `@file` command procedures in the subprocess
  CLI later (out of scope here).

## Progress log

- 2026-10-06: Planned with Phase 43.
- 2026-10-06: The author took every recommended decision in
  PHASE-43.md, Part A.
