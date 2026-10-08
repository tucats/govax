# Phase 48 — Multiprocessing, part 6: LIB$SPAWN and the milestone

**Status:** in progress (started 2026-10-08); decisions taken
2026-10-06 (see PHASE-43.md, Part A). Needs Phases 43–47.

The program this phase belongs to is described in
[PHASE-43 - processes](PHASE-43%20-%20processes.md), Part A. Read that first.

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

Reordered and expanded on 2026-10-08, when work started: the CLI comes
first, since `LIB$SPAWN` can't be tested without something for the
subprocess to run, and the definitions a MACRO program needs to call
`LIB$SPAWN` (`$CLIDEF`, `$LIBDEF`) are a subtask of their own.

1. **The subprocess CLI's machinery and commands** (`internal/console/
   subcli.go`, `corevms/cliprocess.go`): a process that runs a CLI
   instead of an image, running one image after another in the same
   process; the engine's image call (`cpu.ServiceCall.Image`); command
   input from SYS$INPUT (terminal, mailbox, NL:, file); RUN, MCR,
   foreign commands and aliases, symbol assignments, DELETE/SYMBOL,
   EXIT/LOGOUT, unknown verbs; `$CREPRC` of LOGINOUT. Tests.
2. **`LIB$SPAWN`** in `internal/librtl`, on `$CREPRC`'s process
   creation: the arguments, flags, process name, SYS$INPUT/SYS$OUTPUT,
   symbol and logical-name copying, completion status, event flag, AST,
   waiting and `CLI$M_NOWAIT`, `LIB$_NOCLI`. Tests in Go and from MACRO.
3. **`$CLIDEF` and `$LIBDEF`** for govax's own macro library: a
   definition probe for VMS (as Phase 45's `testdata/mp/defs`), and the
   macros generated from its output. Until the probe has run, a program
   uses the names as external symbols, which LINK resolves from
   STARLET.OLB's values (`vmsdef.LibrarySymbols`), as on VMS.
4. **Console SPAWN** (optional).
5. **The milestone programs** and their README.
6. **The acceptance tests** as above.
7. **Probe** (optional) and the masked comparison; with it, the
   `LIB$SPAWN` behaviors only VMS settles (the default process name,
   the completion status, which logical names are copied).
8. **Scheduler on by default**: run the whole suite with the flag on;
   fix what differs; flip the default (Decision 5); `HELP CONFIG KEYS`.
9. **Documentation**: `CLAUDE.md` (the new packages and the process
   model), `PLAN.md` (the program's summary, as earlier multi-phase
   efforts have), `MODE-STACKS.md`, `PERFORMANCE.md` (context-switch
   cost), `DEVIATIONS.md` (every unconfirmed rule from Phases 43–48 in
   one place), HELP for every new command.
10. **Close-out** of Phase 48 and of the program.

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
- 2026-10-08: Started. Subtask 1 (the subprocess CLI). Design: the
  CLI runs in its process as a small procedure in a pool page of the
  process's own (the "CLI stub": `MOVL #64,R0; XFC #XFC$SHIM; PUSHL R0;
  CALLS #1,SYS$EXIT; RET`), whose shim, `EXE$CLI_COMMAND`, is the CLI
  in Go (`Console.cliCommand`). It reads a command and carries it out;
  a command that runs an image has it activated in the process's P0 (as
  `$CREPRC`'s startup activates one) and returns a `corevms.CallRequest`
  with the new `Image` flag, so the engine calls the image's IMAGE$INIT
  driver on a frame whose saved PC and FP are the console's sentinel
  (`cpu.ServiceCall.Image`; shims may now return a call, as services
  could). The image's `$EXIT` or return then ends just the image
  (`ErrConsoleCallReturned`), and `StepMachine` hands the process back
  to the CLI (`endCLIImage`): $STATUS is R0, DCL's message is shown for
  a failure (unless STS$M_INHIB_MSG), the image is run down (user-mode
  process logical names, `ImageRundown`), and FP, AP, and PC (the stub's
  MOVL) are put back, so the shim runs again for the next command. With
  no more commands the shim returns $STATUS and the stub's `$EXIT` logs
  the process out: its deletion, with that final status. A command line
  not there yet (the terminal's, a mailbox's) and output to a full
  mailbox wait through the usual `ErrWait` retry. The CLI's input is
  `corevms.Environment.OpenCommandInput`: SYS$INPUT undefined in the
  process table, or a terminal, is the shared terminal (prompted with
  "$ "); a mailbox or NL: is read a message at a time; anything else is
  a file, read whole. `$CREPRC` of LOGINOUT (any directory, any
  version) now starts the CLI reading SYS$INPUT (`System.Interpreter`,
  the console's `cliHost`), replacing Phase 45's SS$_UNSUPPORTED.
  Commands: `$` prefixes and `!` comments, symbol assignments and
  DELETE/SYMBOL (the console's code, now methods on a symbol table so
  each CLI has its own), foreign commands and aliases, RUN (qualifiers
  ignored; quotes keep a host path whole), MCR, EXIT [status], LOGOUT,
  and `%DCL-W-IVVERB` (with the verb between backslashes on the next line, as DCL shows
  it) for anything else; RUN of a missing image is `%DCL-W-ACTIMAGE`
  and the status's message. Unconfirmed against VMS: the verb
  abbreviations (`R` for RUN, `MC`, `LO`, `EXI`); EXIT ending a
  subprocess's CLI (an interactive DCL may ignore EXIT at command level
  0); that the CLI goes on after a failing command when SYS$INPUT is a
  file or mailbox; ACTIMAGE's second line. `Environment.HasCLI` (set
  for process 1 and CLI processes) is for `LIB$SPAWN`'s `LIB$_NOCLI`.
  Bug found and fixed: image rundown kept the heap's block lists
  (LIB$GET_VM, malloc), though the next image is loaded at the bottom
  of P0 again, over them, so a block left on the free list could be
  handed out inside the next image (process 1's RUNs had the same bug).
  `ImageRundown` now forgets the heap (`releaseHeap`), as VMS's deletes
  the P0 pages it was in. Tests: `cpu`'s `TestXfcShimImageCall`;
  `console/subcli_test.go` (a command file with RUN, a comment, an
  unknown verb, a foreign command, a failing image, an alias, LOGOUT
  and nothing after it; the terminal, prompted, with EXIT's status; a
  missing SYS$INPUT; RUN of a missing image).
- 2026-10-08: Subtask 2 (`LIB$SPAWN`). `librtl/spawn.go` reads the 13
  arguments (LIB$_WRONUMARG for more; SS$_ACCVIO for an argument it
  can't read, or an output longword it can't write, before anything is
  made) and `corevms/spawn.go`'s `Environment.Spawn` does the rest:
  SS$_UNSUPPORTED with the scheduler off, LIB$_NOCLI from a process
  with no CLI (a `$CREPRC` child running an image; `HasCLI`), LIB$_INVARG
  for a flag bit `$CLIDEF` doesn't define for it (above CLI$M_SUBSYSTEM),
  the event flag checked as `$SETEF` checks one. The subprocess is made
  by `CreateProcess`, at the parent's base priority and with its current
  privileges, named as asked or by default the user name and `_n`
  (SYSTEM_1, SYSTEM_2, ...: the smallest n free in the group, the user
  name shortened to fit 15 characters), with its SYS$INPUT and
  SYS$OUTPUT the files asked for or the parent's process-table
  translations (SYS$ERROR follows an output-file, or is the parent's),
  and its startup's `CLI` set to run the command (or SYS$INPUT) with the
  prompt. Unless CLI$M_NOLOGNAM, the parent's user- and supervisor-mode
  process logical names are copied, but not CONFINE ones (DCL's SPAWN
  help); unless CLI$M_NOCLISYM, its DCL symbols (the console's for
  process 1, a CLI's own for a CLI process; `CommandInterpreter.
  InheritSymbols`). The event flag is cleared at once. Completion is
  done directly in Go (the design's choice between that and a
  termination mailbox): `DeleteProcess` calls `completeSpawn` after the
  termination message, which writes the final status to
  completion-status-address through the parent's address space, sets
  the event flag (`postFlag`), queues the AST in the mode LIB$SPAWN was
  called from, and reports the event to the parent. Without
  CLI$M_NOWAIT the caller waits (LEF) until the subprocess is deleted
  (`AwaitSpawn`, the shim's retry finding `spawnWait`). LIB$SPAWN is
  LIBRTL offset 0x518, shim code 43. That made 43 shim stubs, one more
  than VMINIT's 512-byte stub page holds; rather than make the page
  bigger, which would move the SCB and everything VMINIT places after
  it, the stubs that don't fit go to an overflow page from the S0 pool
  (`shimOverflow`), so no address moves. Accepted and ignored:
  CLI$M_NOTIFY, NOCONTROL, NOKEYPAD, TRUSTED, AUTHPRIV, SUBSYSTEM, the
  cli and table-name arguments; CTRL/Y and CTRL/C while the parent
  waits do what they do to any run. Unconfirmed (for subtask 7's
  probe): the default name, the event flag's clearing, the modes and
  which names are copied, the order of the completion's effects. HELP
  CONFIG KEYS' "Processes" now describes LIB$SPAWN and the CLI. Tests
  (`console/spawn_test.go`): a MACRO parent spawning `ECHO from the
  child` (the console's ECHO foreign command, inherited) and waiting,
  then checking the completion status; another with CLI$M_NOWAIT, an
  event flag, and an AST (the subprocess, at its parent's priority,
  preempts it at once, so its line comes first, then the AST, then the
  parent's lines); `Spawn`'s rules in Go (names, priority, owner, SYS$
  names, which logical names are copied, NOLOGNAM, LIB$_INVARG,
  LIB$_NOCLI); and SS$_UNSUPPORTED with the scheduler off.
  `TestEnsureShims_overflowPage` replaces the two tests of the page
  limit.
