# Phase 48 — Multiprocessing, part 6: LIB$SPAWN and the milestone

**Status:** done (2026-10-08), and with it the multiprocessing program;
decisions taken 2026-10-06 (see PHASE-43.md, Part A). Needs Phases
43–47. What the program left for later is Phase 49's; what only VMS can
settle is `testdata/mp/final`'s, the next VAX run.

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
9. **Documentation** *(done)*: `CLAUDE.md` (the new packages and the process
   model), `PLAN.md` (the program's summary, as earlier multi-phase
   efforts have), `MODE-STACKS.md`, `PERFORMANCE.md` (context-switch
   cost), `DEVIATIONS.md` (every unconfirmed rule from Phases 43–48 in
   one place), HELP for every new command.
10. **Close-out** of Phase 48 and of the program *(done: the progress
    log's last entries)*.

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
- 2026-10-08: Subtask 3, first part. On "no MLB for LIB": VMS has no
  macro library of LIB$ calls either; a MACRO program calls LIB$SPAWN
  with an argument list and `CALLS`, as for any RTL routine. What it
  does take from VMS's macro library is two definition macros: `$CLIDEF`
  (the CLI$M_ flags) and `$LIBDEF` (the LIB$_ statuses), and govax's
  library has neither. govax knows their values (STARLET.OLB's, in
  `vmsdef.LibrarySymbols`), but not which names each macro defines, which
  only real MACRO's output can say (clean room). So
  `testdata/mp/defs/def_cli.mar` and `def_lib.mar` are new definition
  probes, `$CLIDEF GLOBAL` and `$LIBDEF GLOBAL`, with `defs.com`,
  `exchange.cmd`, and `copyout.cmd` now for them; they'll run with the
  phase's other VMS work (subtask 7's `run48`). Once `defined.txt` has
  them, `mkdefs` generates the two macros as it does the others. Until
  then a program doesn't call the macros and uses the names as external
  symbols, which MACRO leaves to LINK and LINK resolves from STARLET.OLB's
  values, as on VMS.
- 2026-10-08: Subtasks 5 and 6 (the milestone programs and the
  acceptance tests). The programs are `testdata/mp/msparent.mar` and
  `mschild.mar` (not `parent.mar`/`child.mar`: Phase 45's
  `child.mar` is `TestCreChild`'s), described in `testdata/mp/README.md`.
  The parent's command line is `CREPRC image` or `SPAWN image`; it makes
  `SHARED.DAT` (FAC=PUT, SHR=GET|PUT, RAB$V_EOF) and `PARENT.DAT`, two
  temporary mailboxes (`MS_TO_CHILD`, `MS_TO_PARENT`, in the job table),
  and starts the child: by `$CREPRC` with a termination mailbox, or by
  `LIB$SPAWN` of `RUN image` with CLI$M_NOWAIT (an external symbol LINK
  resolves from STARLET.OLB), event flag 10, and a completion status.
  Eight rounds: the parent writes `MSG n` (finished when the child has
  read it), appends `PARENT n` to both its files while the child appends
  `CHILD n` to its two, and reads the child's `ACK n`. Then `DONE`, the
  child's end (its status, 3, from the termination message or
  LIB$SPAWN's completion status), and `SHARED.DAT` read back and
  counted. A failing service is reported (`Parent: failed with status
  ...`). Every line is printed while the other process waits, so the
  output is the same however they're scheduled. Tests:
  `console/milestone_test.go`'s `TestMilestone` assembles and links both
  with govax's MACRO and LINK onto a fresh volume and runs each way at
  quanta 7, 500, and 20,000, checking the output line by line, PARENT.DAT
  and CHILD.DAT whole, SHARED.DAT's 16 records (each process's in order;
  the writers change 14 or 15 times), and, after a dismount and mount,
  ods2's volume analysis (`VerifyVolume`): all six runs passed the first
  time they ran, once the programs assembled. `TestMilestone_schedulerOff`
  is the negative test (SS$_UNSUPPORTED from both, reported by the
  parent, no process made). `cmd/govax`'s `TestRun_milestone` builds the
  programs with the `macro` and `link` subcommands onto the configured
  default volume (`vax.default.volume.file`) and runs both ways with the
  `run` subcommand under `--instruction-limit`, then checks the volume.
  Learned on the way: LINK puts a bare `/EXECUTABLE` name beside its
  object, which was a host file, so the test names the volume.
- 2026-10-08: Subtask 4 (the console's SPAWN), included since it costs
  little on the LIB$SPAWN path and makes the CLI usable at the prompt.
  `SPAWN [/NOWAIT] [/INPUT=] [/OUTPUT=] [/PROCESS=] [/PROMPT=]
  [/NOSYMBOLS] [/NOLOGICAL_NAMES] [command]` (`console.dcl` verb 1690;
  `console/spawncmd.go`) writes a procedure that calls LIB$SPAWN, its
  argument list, flags, and strings in two S0 pool pages of the
  system's (allocated once per machine; not CONSOLE$SCRATCH, where an
  image stopped mid-run may still return), and calls it in process 1 as
  CALL does, through the debugger when one is installed. So the
  subprocess is made exactly as a program's LIB$SPAWN makes one, and
  process 1 waits in LIB$SPAWN while the machine runs. Process 1 may run
  in user mode, and only kernel mode may write the pool pages, so the
  console passes no process-id or completion-status address and finds
  the new process by comparing the process table before and after.
  Messages as DCL's: `%DCL-S-SPAWNED, process NAME spawned` for
  /NOWAIT, `%DCL-S-RETURNED, control returned to process SYSTEM` after
  a wait (unconfirmed wording). A /NOWAIT subprocess runs only while the
  machine runs (Decision 4), which HELP SPAWN says. Tests:
  `TestSpawnCommand` (a foreign command, /NOWAIT and its subprocess
  running at the next run, /INPUT of a command file), `cmd/govax`'s
  `TestRun_spawnAtThePrompt` (typed at the prompt, the debugger
  installed), `TestGrammarSplit`; the evax grammar's verb count is 49.
- 2026-10-08: Subtask 7, the VMS side prepared (the run itself is the
  author's). `testdata/mp/run48/` puts every Phase 48 question on one
  exchange volume run by `@RUN48`: the `$CLIDEF`/`$LIBDEF` definition
  probes (subtask 3), the milestone both ways (`milestone.com`, each
  run's files typed), and probe 4 (`testdata/mp/probe4/`), eleven steps
  asking what govax guesses: the completion status of a spawned RUN,
  EXIT 7, an unknown verb, and RUN of a missing image (and DCL's
  messages for the last two); which process logical names (user,
  supervisor, executive, CONFINE) and DCL symbols (with and without
  CLI$M_NOCLISYM) a subprocess gets; LIB$SPAWN's status for an undefined
  flag; with CLI$M_NOWAIT, whether the event flag is cleared, the default
  name, and the completion; a `$CREPRC` of LOGINOUT reading a command
  file (its final status); LIB$SPAWN from a process with no CLI; and
  DCL's own SPAWN messages. `TestProbe4` runs the same program under
  govax and logs its report and the spawned processes' logs, for the
  comparison; govax answers every step. The definitions' MACRO log is
  on the clean-room hook's unaudited list until the author has checked
  it, as Phase 46's was.
  Found by probe 4 under govax and fixed: a process whose SYS$OUTPUT is
  a file (LIB$SPAWN's output-file, SPAWN/OUTPUT, `$CREPRC`'s output)
  wrote its lines on the terminal, since `PutOutput` knew only devices.
  Now a SYS$OUTPUT naming a file (no device, or a disk) gets each line as
  a record of that file (`corevms/outfile.go`): created at the first line,
  or, for a CLI process, when it starts, as DCL makes its log at login
  even if nothing is written; written through whole at each line
  (`RewriteRecordFile`, keeping the version), so the creator reading it
  after the process ends sees everything. `TestSpawn_outputFile`.
- 2026-10-08: Subtask 8 (the scheduler on by default, Decision 5). With
  the milestone passing, the whole suite was run with
  `vax.process.scheduler` defaulting to true: everything passed as it
  was, the MACRO, LINK, ANALYZE, debugger, RMS, and instruction-set
  oracles included, since with one process the scheduler changes
  nothing a test sees (Phase 44's design). So the default is flipped:
  `corevms.DefaultProcessSettings` has the scheduler on, and the console
  turns it off only when the key is set to false (`settings.Exists`). The
  three tests that asserted the old default now say so (`TestCreprc_
  unsupported` turns the scheduler off itself; `TestNewSystemProcess
  Settings`, `TestProcessSettings`), and `TestProcessSettings_
  schedulerDefault` checks both ways. HELP CONFIG KEYS and HELP SPAWN
  give the new default.
- 2026-10-08: The author's VMS run (`testdata/mp/run48`) is back.
  Subtask 3 finished: `decode.go` read `def_cli.obj` and `def_lib.obj`
  into `defined.txt` (314 names new to `vmsdef.Symbols`, none differing
  from STARLET.OLB's values), and `mkdefs` generated `$CLIDEF` and
  `$LIBDEF` for govax's macro library. `internal/asm`'s new
  `TestDefinitionProbeObjects` assembles all eleven definition probes of
  `testdata/mp/defs` (Phases 45, 46, and 48) with govax's library and
  checks each object against real MACRO's, record for record. The
  definitions' MACRO log, `defs48.log`, waits for the author's audit
  (the clean-room hook) and isn't read or committed.
- 2026-10-08: Subtask 7, the VMS run's answers. The milestone ran on
  VMS 7.3 both ways with the output and files govax's test expects
  (`testdata/mp/vax/milestone.log`; the `$CREPRC` child's two lines went
  to the terminal), and `TestMilestone_vmsLog` now holds the
  expectations to that log. Probe 4 (`testdata/mp/probe4/vax/
  probe4.log`) agreed with govax on steps 1–3 and 8–11 (statuses, the
  default name SYSTEM_1, the event flag cleared and set, the final
  status of a LOGINOUT job, LIB$_NOCLI) and on which logical names
  (user and supervisor, not executive or CONFINE) and symbols a
  subprocess gets. Changed to match it: RUN of a missing image is
  `-CLI-E-IMAGEFNF, image file not found DEV:[DIR]NAME.EXE;` (the name
  as RMS expands it, `rms.Session.ExpandName`) with the status
  CLI$_IMAGEFNF and STS$M_INHIB_MSG; the CLI has SHOW SYMBOL (DCL's
  line, or `%DCL-W-UNDSYM`) and SHOW LOGICAL (the console's display,
  now `logicalDisplay` over a process's own tables, status SS$_NORMAL
  with STS$M_INHIB_MSG), and SHOW LOGICAL's table headers, the
  console's too, stand between blank lines as VMS's do; LOGINOUT's CLI
  (`CLIStartup.Login`) reading a file or mailbox echoes each line (DCL's
  verify; the ping-pong log of Phase 46 shows it too) and ends with
  LOGOUT's report (`corevms/logout.go`, VMS's layout, govax's counts
  zero as in the termination message; an interactive job's one line is
  unconfirmed); the CLI writes what it has queued before it runs a
  command or calls an image, so an echo comes first; and the console's
  SPAWN prints `%DCL-S-SPAWNED` and, when it waits, `%DCL-S-ATTACHED`
  when the subprocess has been made (`Environment.SpawnNotice`), before
  its output. `TestProbe4` now compares govax's report with VMS's line
  for line and each spawned process's log with VMS's (the volume, the
  logout time, and LOGOUT's counts masked); both match. Also: the
  `$GETJPIW` process trace was the one `DEBUG(PROCESS)` line written as
  `DEBUG:`, which the tests' filter missed; and Phase 45's unused
  `ssSuspended` is gone. DEVIATIONS.md's Phases 43–48 entry keeps only
  what no run has settled.
- 2026-10-08: ods2 v0.1.18 counts each volume's logical I/O operations
  (`Device.Operations`, `Volume.Operations`: block reads and writes).
  `rms.MountTable.Operations` returns a mounted device's, and SHOW
  DEVICE/FULL's "Operations completed" and `$GETDVI`'s `DVI$_OPCNT`
  report it for a disk with a volume mounted (the device record's own
  count otherwise). Tests: `TestShowDevices_operationsCompleted`,
  `TestGetdvi_operationCount`; HELP SHOW DEVICE says what it counts.
- 2026-10-08: The program's close-out began: the author chose to move
  Phase 47's deferred features and Phase 46's file-backed sections to a
  new Phase 49, to add the debugger's SET PROCESS now, and to prepare
  one more VMS probe round for what's still unconfirmed. First, Phase
  45's carry-forward item 6: SHOW DEVICE/FULL of a terminal or a
  mailbox in VMS 7.1's layouts (Phase 45's subtask 14 note), by
  `showRecordDeviceFull` (`console/device.go`), which NLA0: now shares:
  "Terminal" or "Device" first, the device type within its class
  ("local memory mailbox", "null device", the terminal types), the
  characteristics from DEVCHAR (CCL is "carriage control"), the
  sentence wrapped a word at a time within 80 columns (VMS's three
  samples all fit that rule; NLA0:'s phrase rule gave the same
  result), the owner UIC [1,4] as `[SYSTEM]` (`uicText`; other UICs in
  octal), and the protection as VMS writes it (`protectionText`: a
  mailbox's from its promsk; VMS 7.1's `S:RWPL,O:RWPL,G,W` for a
  terminal). `vax.init` gives TTA0: a terminal's DEVCHAR (0C040007:
  REC, CCL, TRM, AVL, IDV, ODV; unconfirmed) and SYSTEM's UIC. Every
  `$QIO` now counts in its device's operations completed when it
  completes (`completeIO`), as VMS's UCB$L_OPCNT. Test:
  `TestShowDeviceFull_terminalAndMailbox`.
- 2026-10-08: Close-out, Phase 46's carry-forward items 3 and 5.
  `EXE$INPUT` and `DECC$GETS` read the shared terminal through its
  queue (`readSharedConsoleLine`: `awaitTerminal` for a line ending in
  a newline, which is what they read), so with the scheduler on a read
  with no whole line typed waits in LEF and the shim runs again, rather
  than the machine stopping in the host's read (`TestTerminal_
  inputShims`). `TestExecute_stopsOnAttention` raced its one CTRL/C
  against `Execute`'s `BeginRun`, which clears a CTRL/C typed before a
  run; its goroutine now presses CTRL/C every millisecond until
  `Execute` returns (200 runs, and 20 under the race detector, pass).
- 2026-10-08: From the author's VMS system: SHOW DEVICE takes the start
  of a device name (`SHOW DEVICE DU` shows every DU device), and with no
  device matching says `%SYSTEM-W-NOSUCHDEV, no such device available`.
  The console's SHOW DEVICE now does both (it matched one whole name and
  printed nothing for none, as the C source did). Test:
  `TestShowDevices_prefix`; HELP SHOW DEVICE says so.
- 2026-10-08: Close-out, Phase 44's future work: the debugger's SET
  PROCESS (`debugger/process.go`, `debug.dcl` ids 316–319). `SET
  PROCESS [/VISIBLE] [pid|name]` makes a process the visible one: the
  CPU moves to it (`Console.ShowProcessContext`, through
  `System.SwitchCPU`, as when a run stops in it), so EXAMINE, DEPOSIT,
  and SHOW REGISTERS see its context; with no name it's process 1
  again. A pid is hexadecimal, a name is looked up in process 1's group,
  and a missing process is `%SYSTEM-W-NONEXPR`. Running is unchanged
  (Decision 11): STEP and breakpoints stay with process 1, and GO, STEP,
  and CALL hand the CPU back to it first. `SHOW PROCESS` (the debugger's
  own; the console keeps its VMS-style one) lists every process's PID,
  name, state (`System.ProcessState`), and PC (`Console.ProcessPC`: the
  CPU's, or the one saved in the process's PCB), the visible one marked
  "*" (govax's layout, unconfirmed). Tests: `TestSetProcess` (a
  subprocess hibernating in an image; by name and by PID, back, and
  NONEXPR), `TestGrammarSplit`; HELP SET PROCESS and SHOW PROCESS.
- 2026-10-08: The next VAX run, prepared (`testdata/mp/final`): round 7
  of the macro probes (`r7_lock`, written by `gen.go`: candidate
  keywords for `$ENQ`'s twelfth and thirteenth arguments and
  `$GETLKI`'s seventh) and probe 5 (`testdata/mp/probe5`: a
  subprocess's priority, working set, and AST limit at creation; PID
  reuse; what a subprocess inherits; `$ENQ` NOQUEUE and value blocks
  after an EX holder's deletion; `$OPEN` of a mailbox and FAB$W_MRS;
  `$ERASE` of a file open for writing; 41 services' argument-count
  minimums; and, from DCL, verb abbreviations, statuses, the copied
  names' modes, device characteristics, and SHOW DEVICE). `TestProbe5`
  runs probe 5 under govax; it found `$EXPREG` failing in Go when called
  with fewer than four arguments (fixed: `optArg`). The clean-room hook
  lists round 7's MACRO log as unaudited.
- 2026-10-08: Subtask 9, the documentation. `CLAUDE.md` (the phase
  range, `testdata/mp`, LIB$SPAWN and the subprocess CLI, corevms's
  Phase 48 files, the debugger's SET PROCESS); `PLAN.md` (Phase 48's
  row, Phase 49's, and the program's summary); `PERFORMANCE.md` ("Check:
  context switches": `BenchmarkContextSwitch` puts a switch at about
  360 ns, 0.02 ns an instruction at the default quantum; most of it is
  `System.Schedule`, chiefly `budget`'s scan of every process's timers);
  `DEVIATIONS.md` (every unconfirmed rule of Phases 43–48 in one entry,
  with what `testdata/mp/final` asks); `MODE-STACKS.md` already had
  Phase 43's other processes' stacks, and Phase 48 changed none. Phase
  49's plan (`PHASE-49 - record updates and locks.md`) takes over the
  carry-forward sections' features, which now point to it or to
  `testdata/mp/final`.
- 2026-10-08: Close-out (subtask 10). Phases 43–48 are done: every
  carry-forward item of Phases 44–47 is done, in Phase 49's plan, or a
  question for `testdata/mp/final`. Waiting on the author: the audit of
  `defs48.log` (and, after the next run, of round 7's log; both on the
  clean-room hook's list) and the `testdata/mp/final` run.
