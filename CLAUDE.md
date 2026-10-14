# CLAUDE.md

This file provides guidance to Claude Code when working in this repository.

## Project overview

`govax` is a from-scratch Go rewrite (not a cross-compile) of `eVAX`, a C emulator for
a DEC VAX/VMS system. The goal is a careful evaluation of the C implementation followed
by rewriting it in idiomatic Go, with a comprehensive unit test suite the C version
never had.

- `docs/PLAN.md` — high-level plan, locked-in architecture decisions, and the phase
  index.
- `docs/PHASE-00.md` … `PHASE-52.md` — one doc per phase: goal, C-source file
  mapping, deliverables, open questions, and a dated progress log (all
  done through 52, but for Phase 42's Part B, the debugger's further
  features, in progress; 40 follows 38 directly: there is no Phase
  39). Phases 43–48 are the multiprocessing program (subprocesses, a
  scheduler, interprocess mailboxes and shared memory, RMS file sharing
  and the lock manager, LIB$SPAWN; done 2026-10-08; PLAN.md summarizes
  it), and Phase 49 the work it left for later (done 2026-10-09);
  `PHASE-43.md`'s Part A describes the whole program. Phase 50 (done
  2026-10-12) made the console a DCL command interpreter: command
  procedures and command levels, symbol substitution, expressions,
  `$STATUS`/ON, labels and GOTO, IF blocks, GOSUB, CALL, the lexical
  functions, SET VERIFY, SYS$INPUT from a procedure (DECK/EOD), DCL's
  file I/O (OPEN, CLOSE, READ, WRITE, INQUIRE), STOP and Ctrl/Y, the
  console's whole DCL in subprocesses, and the other processes running
  between DCL commands and during WAIT. Phase 51 (done 2026-10-12) is
  what it left for later: round 6 of the DCL probe, SPAWN from a
  subprocess, Ctrl/Y in a subprocess's procedures, the other processes
  running while the prompt waits for a line, and the known differences
  from VMS (missing files' messages, F$CONTEXT, F$DEVICE,
  F$FILE_ATTRIBUTES, F$IDENTIFIER, STOP/IMAGE and /[NO]EXIT, DELETE's and
  DIRECTORY's layouts, DUMP). Phase 52 (done 2026-10-12, paused
  Phase 51) split `internal/console` into `internal/machine`,
  `internal/dclshell`, `internal/dclcmd`, `internal/dcl`, and
  `internal/help`, leaving `internal/console` the front end; its doc
  has the table of where each old `internal/console` file went.
  Phases 53–61 (planned 2026-10-12) are the LIB$ program: `PHASE-53`'s
  Part A sorts every LIBRTL routine into them (from the VMS 5.0 LIB$
  manual, `~/Documents/Technical Doc/VMS/rtl-library-lib-manual.txt`,
  an OCR with errors, beside its PDF), Part B is the groundwork they
  share, and Part C its own group, the instruction routines (done
  2026-10-12, checked against VMS by `testdata/lib53`). Phase 54 (done
  2026-10-12) is strings, descriptors, and conversion, with JSB entry
  points, the `LIB$AB_` tables, and LIBRTL2's CVT_DX_DX, checked
  against VMS by `testdata/lib54` (2026-10-13). Phase 55 (done
  2026-10-13) is the service jackets, the item routines, event flag and
  LUN pools, timers, LIB$WAIT and its neighbors, and the 7.x
  protection-text routines, checked against VMS 7.1 by `testdata/lib55`.
  Phase 56 (done 2026-10-13) is date and time: arithmetic, conversion,
  DATE_TIME/DAY/DAY_OF_WEEK, and the formatting routines (contexts,
  FORMAT_DATE_TIME, CONVERT_DATE_STRING) with LNM$DT_FORMAT_TABLE and
  the system's LIB$DT_FORMAT/LIB$DT_INPUT_FORMAT built in, checked
  against VMS 7.1 by `testdata/lib56`.
  Phase 57 (done 2026-10-13) is the CLI routines: `corevms.CLI`
  (`Environment.CLI`, nil without a CLI) is a process's DCL as the LIB$
  routines reach it, `dclshell/libcli.go` its implementation on each
  process's own Shell; the symbol routines are done (subtask 2), and
  SET_LOGICAL and DELETE_LOGICAL (subtask 3, `$CRELNM`'s and
  `$DELLNM`'s code at supervisor mode, `librtl/logical.go`), and
  GET_COMMAND (subtask 4, GET_INPUT's code reading SYS$COMMAND,
  `librtl/input.go`), and GET_COMMON and PUT_COMMON (subtask 5,
  `librtl/common.go`, the area kept in Go as
  `corevms.Environment.Common`, not at SYS$GL_COMMON), and DO_COMMAND
  and RUN_PROGRAM (subtask 6, `librtl/chain.go`: `CLI.Chain`, then a
  tail call of `$EXIT`, `Environment.ExitFromShim`; DCL runs the text
  after the line that ran the image, `dclshell/dclchain.go`), and
  DISABLE_CTRL and ENABLE_CTRL (subtask 7, `CLI.ControlKeys`: SET
  CONTROL's settings are each process's `Shell.control`, and
  `ControlYEnabled` answers for the process the terminal is attached
  to, from an atomic copy `noteControlKeys` keeps), and PAUSE (subtask 8,
  `CLI.Pause`, `dclshell/dclpause.go`: the Ctrl/Y level at any command
  level, read by `readControlYLevel`; in a subprocess, a tail call of
  the CLI stub's Ctrl/Y routine), and ATTACH (subtask 9,
  `corevms/attach.go`: the job's terminal holder, `Job.terminal`, which
  `Attached` returns; `AttachTerminal`/`AwaitAttach`; LIB$ATTACH and
  DCL's ATTACH, `dclshell/attachcmd.go`), and the probe (subtask 10,
  `testdata/lib57`, checked against VMS 7.1; subtask 11 changed govax
  to its answers).
  Phase 58 (in progress) is callbacks and the file routines: subtask 1
  is the survey, subtask 2 the callback mechanism
  (`corevms/callback.go`: a routine registered with
  `ShimTable.RegisterCallback` runs on a goroutine of its own, and
  `Environment.Callback` calls a VAX procedure and returns its R0;
  the XFC its RET lands on finds the routine by the stub's FP and SP),
  subtask 3 FIND_FILE and FIND_FILE_END (`librtl/findfile.go`: a heap
  block holding a FAB and NAMs, run through `$PARSE`/`$SEARCH`),
  subtask 4 FILE_SCAN and FILE_SCAN_END (`librtl/filescan.go`, the
  first routine in `librtl`'s `callsBack` set, which `Register`
  installs with `RegisterCallback`).
  Read the relevant phase doc
  before starting work on that subsystem, and extend its progress log as you go.
- `docs/DEVIATIONS.md` — running log of suspected ISA/behavior fidelity issues found in
  the C source during porting (see "Bug-fixing policy" below).
- `docs/DEBUG-RECORDS.md` — a clean-room description of the debug symbol
  table (DST) format the author provided (Phase 29): record types,
  data symbols and descriptors, the line-number program, and source
  correlation. Usable freely as a reference, though it may have errors;
  check it against real MACRO's output.
- `docs/PERFORMANCE.md` — performance studies: the profiling method
  (`govax --cpu-profile FILE`, the `perf` configuration profile), and per
  workload the observations, ranked recommendations, and an implementation
  log of the fixes made.
- `docs/MODE-STACKS.md` — where VMINIT puts each access mode's stack, their sizes and
  page protections, and what was deliberately left unchanged.

## Reference material

- `reference/eVAX/` — a full, read-only copy of the C source tree (imported via
  `git archive` from the upstream `tucats/evax` repo). This is the primary behavioral
  reference during conversion — read the corresponding C file before/while porting a
  subsystem. Do not edit it; if upstream changes, re-import.
- `reference/AUDIT.md` — the C project's own closed 32-vs-64-bit portability audit
  (root-cause `LONGWORD` typedef bug and its downstream fixes). Explains *why* certain
  things in the C source look the way they do; the fixes it documents are already
  present in `reference/eVAX/`.
- `reference/CLAUDE.md` — the C project's own CLAUDE.md, useful background on its
  architecture and gotchas (e.g. CR-only line endings in some files).
- `testdata/{asm,exe,rom,dcl}/` — fixtures pulled from the C repo's root
  (`.asm` sources, real VMS `.exe` binaries, `xdefault.rom`, `evax.dcl`/
  `vax.init`), organized by type for use in Go tests. `vax.help` used to have
  a second copy here too; it was consolidated down to the single, live copy
  at `internal/bootdata/files/vax.help` (Phase 22) since, unlike `evax.dcl`,
  it had no ongoing reason to track a separate upstream-import lineage.
- `testdata/mar/forth.mar` — a FORTH interpreter in MACRO-32 (RMS I/O,
  five psects, macro-built dictionary; Phase 36), run by `internal/console`'s
  `TestForth` tests; its VMS 7.3 object, listing, image, and map are in
  `testdata/mar/vax/`, and govax's match them.
- `testdata/mp/` — the multiprocessing program's programs and VMS runs
  (README.md): the milestone (`msparent.mar`, `mschild.mar`), probes 1 to
  5, the definition probes (`defs`), the system service macro probes
  (`macros`, rounds 1 to 7), and each VAX run's exchange scripts
  (`run46`, `run48`, `final`); `vax/` directories hold VMS's output.
- `testdata/dcl50/` — Phase 50's VMS probe of DCL (README.md): symbol
  substitution, expressions, `@`, and statuses, as a command procedure
  run with SET VERIFY; `vax/probe50.log` is VMS 7.3's log, which
  `TestProbe50Oracle` replays. Round 2 (`probe50v.com`,
  `vax/probe50v.log`, `TestProbe50VOracle`) is verification, and round
  3 (`probe50i.com`, `vax/probe50i.log`, `TestProbe50IOracle`) SYS$INPUT,
  DECK, and EOD. Round 4 (`probe50f.com`, `vax/probe50f.log`,
  `TestProbe50FOracle`; the interactive `probe50q.com`,
  `vax/probe50q.log`, `TestProbe50QOracle`) is DCL's file I/O. Round 5
  (`probe50t.com`, `vax/probe50t.log`, `TestProbe50TOracle`; the
  interactive `probe50y.com`, `vax/probe50y.log`) is statuses, flow, the
  lexical functions, subprocesses, STOP, and Ctrl/Y. Round 6
  (`probe50a.com`, with `probe50b.com` and `probe50c.com`,
  `vax/probe50a.log`, `TestProbe50AOracle`; the lines typed at the
  terminal, `vax/probe50a-term.log`, `TestProbe50ATerminal`; Phase 51's
  subtask 1) is what round 5 left open. Since Phase 51 the round 4 and 6
  oracles compare DELETE's, DIRECTORY's, and DUMP's output too, on a
  volume with VMS's 3-block clusters (`mountClusteredContainer`), file
  IDs and times masked (`maskVolumeDetails`).
- `testdata/lib53/` — Phase 53's probe of LIBRTL's instruction routines
  (README.md): `gen.go` writes `lib53.mar` (132 cases, each under a
  handler that tells a signaled condition from a returned status), and
  `vax/lib53-vax.txt` is VMS V7.1's output, which `TestLib53Oracle`
  matches line for line.
- `testdata/lib54/` — Phase 54's probe of LIBRTL's string and conversion
  routines (README.md): `gen.go` writes `lib54.mar` (143 cases; JSB
  cases show R1 to R6, the translation tables are dumped), and
  `vax/lib54-vax.txt` is VMS V7.1's output, which `TestLib54Oracle`
  matches line for line but for `expectedLib54Differences` (the
  dynamic-string JSB entries' heap registers).
- `testdata/lib55/` — Phase 55's probe of LIBRTL's jackets, item
  routines, allocators, timers, and protection text (README.md):
  `gen.go` writes `lib55.mar` (216 cases), `lib55.com` (which defines
  `LIB55_DISK`), and `exchange.cmd`; values that vary from run to run
  print on `  ~` lines, whose names alone are compared, and SHOW_TIMER's
  line is also shown with its digits made 9s. `vax/lib55-vax.txt` is
  VMS V7.1's output, which `TestLib55Oracle` (a twin of the exchange
  volume on DUA1, NLA0:, SYS$LANGUAGE ENGLISH) matches line for line
  but for `expectedLib55Differences`.
- `testdata/lib56/` — Phase 56's probe of LIBRTL's date and time
  routines (README.md): `gen.go` writes `lib56.mar` (426 cases; times
  shown as `$ASCTIM` writes them, the formatting names defined by the
  program a case at a time), `lib56.com` (which runs LIB$DT_STARTUP
  when the predefined formats are missing, its own lines not echoed:
  they are DIGITAL's), and `exchange.cmd`. `vax/lib56-vax.txt` is VMS
  V7.1's second run (`lib56-run1.*` the first, which hung), which
  `TestLib56Oracle` matches line for line but for `expectedLib56Cases`
  (CONVERT_DATE_STRING's SS$_ROPRAND on that VAX; docs/DEVIATIONS.md).
- `testdata/lib57/` — Phase 57's probe of LIBRTL's CLI routines
  (README.md): `gen.go` writes `lib57.mar` (181 cases in 42 groups, a
  group per run, named by a foreign command's parameter), `lib57b.mar`
  (the image LIB$RUN_PROGRAM runs), `lib57.com` (runs each group and
  shows DCL's state after it, sections S1 to S9), its small procedures,
  `lib57i.com` (the interactive part, the README's table), and
  `def_libcli.mar` (`$LIBCLIDEF GLOBAL`; VMS 7.1 has no such macro).
  `TestLib57Oracle` runs `@LIB57` on govax and matches VMS V7.1's log,
  `vax/lib57.log`, section by section (`lib57Compared` masks the job and
  group tables' SHOW LOGICAL/FULL headers, `lib57RunHistory` takes out
  the traces of an earlier run); `vax/lib57-term.log` is the interactive
  session.
- `testdata/probe49/` — Phase 49's VMS probe in three rounds (`probe6`,
  `probe6b`, `probe6c`; README.md has each round's answers), run by
  `TestProbe6*`; `testdata/rms49/` and `testdata/sec49/` are the phase's
  record-operation and file-section programs.
- `testdata/mar/` — Phase 27's MACRO-32 fixtures, with real VAX MACRO's objects,
  listings, and analyses in `vax/` (see its README for the simh round trip).
  `testdata/disks/` holds local-only ODS-2 containers (gitignored).
- `~/Documents/Technical Doc/VMS/vax_instr_set.pdf` — the VAX architecture/
  instruction-set reference manual (a copy may also be at
  `reference/vax_instr_set.pdf`, local-only).
- **Clean-room barrier (Phase 32).** govax's own system macros
  (`internal/bootdata/files/starlet.mar`) are written from DIGITAL's manuals
  and from real MACRO's *output*, never from VMS's STARLET macro library's text.
  `.claude/hooks/cleanroom.sh`, a PreToolUse hook, refuses tool calls that
  name that library, `testdata/vmslib`, `reference/vms`, or unaudited
  real-MACRO listings. It matches on text, so a command or search that merely
  mentions the library's file name is refused too. Use the Edit/Write tools
  for docs that name it, and `git commit -F` for such commit messages. Don't
  work around the hook; its test cases are in `cleanroom_test.sh`.
  Since 2026-10-04 the project is fully clean room: the VMS source archive
  (`~/Documents/Technical Doc/VMS/vmssrc_archive/`, VMS's own source
  listings) is off-limits too. Earlier phase docs that cite it record
  history, not a method to follow. When neither a manual nor real VMS
  output settles a rule, make a reasonable choice and log it as
  unconfirmed in the phase doc. One exception (2026-10-06, `PHASE-43.md`'s
  Decision 6): DIGITAL's book *VAX/VMS Internals and Data Structures* is
  usable in full (descriptions, layouts, example code and listings), cited
  where used. The *VMS Internals I ... Listings* course book is not.
  A second (2026-10-13, `PHASE-55`'s survey, "Sources"): VMS Software,
  Inc.'s public *VSI OpenVMS RTL Library (LIB$) Manual*
  (`~/Documents/Technical Doc/VMS/VSI OpenVMS RTL Library (LIB$)
  Manual.html`) is usable like DIGITAL's manuals; it describes current
  OpenVMS, so what it adds to a routine VMS 7.3 had is a probe case.
  So are VSI's public system services pages (2026-10-13), extracted as
  `VSI-system-services-1.txt` and `-2.txt` in the same folder: the
  second place to look when the 5.0 System Services manual falls short,
  on the same terms. Entries are headed by their descriptive names ("Get
  Message — ..."), so search for those or the service's arguments.
- `reference/vms/` — local-only (gitignored, Phase 31): licensed VMS 7.3
  definition files (SDL sources, C headers, BLISS and message listings) that
  `internal/vmsdef`'s tables were first generated from. Nothing in the build
  or tests reads them; use them with `internal/vmsdef/gen` to add
  definitions. Never commit DIGITAL/HP/VSI-copyright material (see also
  `testdata/vmslib/` and `testdata/disks/`).

## Build & test

- `go build ./...`
- `go vet ./...`
- `go test ./...` (as tests are added per phase)

## Architecture

Module `github.com/tucats/govax`. State lives in an instantiated struct passed
explicitly / receiver-bound — no package-level singleton mirroring the C source's
global `struct VAX vax`. Current package layout (see `docs/PLAN.md` for rationale, and
expect adjustment as phases land):

- `internal/registers` — core machine state: registers, PSL, condition codes (Phase 01;
  `internal/vax` until 2026-10-12).
- `internal/vm` — virtual memory: address translation, load/store primitives (Phase 02).
  The decoder reads the instruction stream through `fetch.go`'s
  instruction-fetch window (`FetchByte`/`TryFetchByte`/..., one page
  translated once; docs/PERFORMANCE.md, Study 1, R4), data through the
  Load/Store methods; anything that empties the STC empties the window.
  `space.go` (Phase 43): a `vm.AddressSpace` (P0BR/P0LR/P1BR/P1LR) names
  one process's page tables, and `TranslateIn`/`LoadIn`/`StoreIn`/
  `LookupPTEIn`/`StorePTEIn` reach a process that isn't current through
  it (kernel-mode protection, no TB). `InvalidateProcessTB` empties only
  P0/P1 TB slots, as LDPCTX does. The length checks follow the
  architecture: P0LR and SLR are page counts, P1LR the lowest P1 page.
  `pager.go` (Phase 49): the invalid PTE forms (TYP0, a process section
  table index; TYP1, a global page table index; `SectionPTE`,
  `GlobalPTE`, `ValidPTE`) and the `Pager` hook (`SetPager`): every
  fault on an invalid page goes to it (`PageIn`, given a `PageFault`
  with the address space), or is demand-zero without one
  (`DemandZero`). The seam a working set manager and page file will use.
- `internal/cpu` — instruction decode/execute engine and instruction-set emulation
  (Phases 03-07). Phase 35 finished the set and Phase 43 added LDPCTX and
  SVPCTX: every instruction has a handler (`TestEveryInstructionImplemented`).
  `context.go` has the 96-byte hardware PCB (`PCB`, `ReadPCB`/`WritePCB`
  by physical address), the two instructions, and `Engine.SaveContext`/
  `LoadContext`, the same code the Go scheduler uses (Phase 44:
  `SaveMemoryContext` writes the live P0/P1 registers into the outgoing
  PCB first). `schedule.go` is the scheduling hook (Phase 44): a
  `Scheduler` installed with `SetScheduler`, called by `Step` (after
  interrupt delivery, before AST delivery) when the budget it returned is
  spent or after `RequestReschedule`; `Preemptible` (IPL < 3, not on the
  interrupt stack, a mode `PreemptModes` allows); `FreezeScheduling`
  (STEP, nested runs); `SwitchIfDue` (run loops switch before looking at
  the next instruction). `idle.go`: `IdleUntil` (the quantum clock
  jumps, the host clock sleeps) and `InstructionsUntil`. Each operand has a
  `DataType` in the generated table, which decides how a short literal or
  floating operand is read. F, D, G, and H instructions run on
  `internal/vaxfloat` (`fpu.go` is the CPU's side), packed decimal on
  `decimal.go` (`decimalmath.go`, `decimalconvert.go`, `editpc.go`), and
  octawords through `Operand.LoadOctaword`/`StoreOctaword`. An arithmetic
  *trap* (integer or decimal overflow, DIVP by zero) is
  `Engine.arithmeticTrap`, whose saved PC is the next instruction's. The
  Phase 35 oracle (`testdata/insn35`, `TestInsn35Oracle` in
  `internal/console`) compares 567 instruction cases with VMS's run.
- `internal/vaxfloat` — the VAX floating formats (Phase 35): `Unpack`/`Pack` for
  F, D, G, and H, and exact `Value`s (`math/big`) whose arithmetic rounds once,
  half away from zero, to the destination format, reporting overflow,
  underflow, reserved operands, and divide by zero; plus EMOD and POLY's
  arithmetic, short literals, and decimal parsing for the assembler. A leaf
  package: no CPU dependency.
- `internal/sched` — VMS's scheduling rules as plain Go (Phase 44), from
  *VAX/VMS Internals and Data Structures*, chapter 10: `State` (the
  `SCH$C_*` codes) and MWAIT `Resource`s, priority-boost `Class`es (Table
  10-3) and the boost rule, one FIFO queue per priority, `Reschedule`
  (choose the highest, demoting a boosted normal process by one),
  `Charge` (quantum; no quantum end for real-time), `Wait`/`Ready` (and
  the higher-or-equal preemption test), `SetBasePriority`, `Choose`. A
  leaf package; processes are opaque `Handle`s (PIDs).
- `internal/lck` — the lock manager (Phase 47), from the System Services
  manual ($ENQ/$DEQ) and the Internals book's chapter 13: a `Manager` of
  resources (name, UIC group or 0, access mode, parent) with granted,
  conversion, and waiting queues (FIFO), the six modes' compatibility
  table, NOQUEUE, conversions, sublocks, CANCEL, DEQALL, value blocks,
  blocking notices; `info.go` (Phase 49) answers `$GETLKI`'s questions
  and defines which lock blocks which (`Blocks`); `deadlock.go` is
  the book's deadlock detection (a search from a request that has
  waited DEADLOCK_WAIT; the victim gets SS$_DEADLOCK). Operations return `Event`s for any owner, and
  `lck.Deliver` hands each to its lock's `Data` if that's a `Notifier`
  (how `$ENQ` completes in the owner's process and an RMS record-lock
  wait is woken). A leaf package; owners are PIDs.
- `internal/console` — the front end (Phase 08; since Phase 52 only
  that): a `Console` embeds a `*machine.Machine` and a
  `*dclshell.Interpreter`, and its `Dispatcher` routes each line to the
  interactive assembler, the debugger, or DCL, letting the other
  processes run at command level 0. Every command is parsed by the
  grammar (`internal/bootdata/files/console.dcl`); DCL binds its verbs,
  and this package the machine's (`commands.go`, `setcommand.go`,
  `dispatch.go`) and dclcmd's (`logicalcmds.go`, `dispatch.go`). An
  address or value is an `$expression` parameter, whose extent the
  grammar finds and whose value the handler gets from the machine's
  expression evaluator. Unquoted text is uppercased, so a case-sensitive
  host file name must be quoted. The one-shot command line and run
  limits are `misc.go`. Most end-to-end and oracle tests are here;
  `internal/console/consoletest` is test support for packages that drive
  a console (the debugger's tests).
- `internal/machine` — the emulated machine as the operator sees it
  (Phase 52; earlier in `internal/console`): `Machine` holds the engine,
  CPU, and memory, INIT/ZERO, VMINIT's layout, ROM/NVRAM, the shim stubs
  (`shim.go`, over `corevms.ShimLink`), the image loader and each
  process's image state (`image.go`, `images.go`, `ImageState`), RUN and
  its IMAGE$INIT driver, the run loop and trace, the console symbol table
  and expression evaluator (`symbols.go`, `expr.go`), ASM's session,
  EXAMINE's sizes, disassembly, the debug-symbol helpers, CHF frames,
  SET and SHOW of the machine's state, and `export.go`, the debugger's
  surface. It is the engine's `cpu.SystemServices` (and `ASTSource`,
  `ExceptionDispatcher`, `AttentionHandler`, asserted in `services.go`),
  passing services, shims, ASTs, and exceptions to the running process
  (`corevms.EngineHooks`); what it needs of the DCL running a command
  (output, default directory, `$STATUS`, Ctrl/Y, the subprocess CLI,
  process 1's terminal, XFC$CONSOLE_CMD, quit) is `Machine.Hooks`, which
  `dclshell.NewInterpreter` sets. `NewMachine` makes one with hooks of
  its own, so it works, and is tested, alone (`helpers_test.go`:
  `newTestMachine`, `newBootableMachine`). Imports only lower packages.
- `internal/dclcmd` — the utility commands (Phase 52; earlier in
  `internal/console`): DIRECTORY, TYPE, COPY, DELETE, PURGE, RENAME,
  DUMP (`dump.go`, Phase 51),
  CREATE, SET/SHOW DEFAULT, MOUNT/DISMOUNT, INITIALIZE/CONTAINER, the
  default volume, DEFINE/ASSIGN/DEASSIGN and SHOW LOGICAL/TRANSLATION,
  DEFINE/DEVICE and SHOW DEVICES, SHOW SYSTEM/PROCESS, MACRO, LINK
  (`linksource.go`, `GovaxSymbols`), LIBRARY, ANALYZE, the VMS library
  search (`syslib.go`), and paging (`page.go`). Each is a method of
  `Context`, which the console builds for each command from the DCL
  running it (`Console.cmd`; tests and cmd/govax use
  `Console.Commands`); what a command asks of DCL is its function fields
  (`Running`, `SetStatus`, `ReadReply`, `TypeDCL`). Knows nothing of
  the console, DCL, or the machine.
- `internal/dcl` — the grammar engine (Phase 08; `internal/console/dcl`
  until Phase 52): reads a grammar file (`console.dcl`, `debug.dcl`),
  parses a command line into a `Result`, and dispatches it to the
  handler bound to its verb or syntax. The console and the debugger
  each have a grammar. `internal/help` reads the help files (`help.File`:
  `Parse`, `Load`, `Show`).
- `internal/dclshell` — DCL, the command language interpreter (Phases 48,
  50, 51; its own package since Phase 52). An `Interpreter` is every
  process's DCL: each process that runs DCL has a `Shell` (its process,
  logical names, default directory, output, symbols, command levels,
  `$STATUS`, open files, Ctrl/Y and STOP state), and the current one is
  embedded, so its fields read as the interpreter's; process 1's is
  `home`, a subprocess CLI's its own (`subcli.go`, made current by
  `useShell`). `NewInterpreter` (`dclhooks.go`) makes process 1's Shell
  and installs the machine's `Hooks`. The interpreter holds the console's
  grammar (`Grammar`) and binds DCL's own verbs in it (`BindDCL`);
  `DispatchConsole` (`dcldispatch.go`) runs a line through DCL's phases,
  and lines a procedure gives the debugger or the interactive assembler
  go through the console's routing (`SetRoute`). `inspect.go` is what
  the console's tests see of its state. Its tests run on a harness of
  their own (`helpers_test.go`: the console's grammar with only DCL's
  verbs bound).
  **Command procedures** (Phase 50, `procedure.go`):
  `@file[/OUTPUT=file] [p1 ... p8]` is read before the grammar (the
  console has no INCLUDE) and runs the file at a new command level on
  `Shell.levels`, the input stack, with its own local symbol table
  (`dclSymbolTable.push`/`pop`) holding P1 to P8. Its lines follow VMS:
  `$` starts a command, `$!` a comment, `-` continues; a line without
  `$` is data, given to the debugger or interactive assembler when one
  is reading, else skipped (SKPDAT), and a `$` line ends that reader's
  input. An error or severe status ends the procedure (DCL's default
  ON ERROR THEN EXIT), EXIT ends it (at level 0, govax), QUIT always
  ends govax. **Statuses** (`dclstatus.go`): `$STATUS`/`$SEVERITY` are
  global string symbols set around each line `DispatchConsole` reads
  (`statusOf`); a handler's error becomes VMS's condition value for the
  same message name (`ConditionValue`, CLI_IVVERB is CLI$_IVVERB), and a
  handler can name its status (`SetCommandStatus`: @, RUN's image exit
  status) or keep it (`keepStatus`: SHOW SYMBOL, IF, CONTINUE). Each
  level's `onAction` is ON's and SET [NO]ON's; `procedureLine` shows a
  command's message, then takes the action; a procedure ends with a
  `procedureExit` (end of file, EXIT, `EXIT n`, or the error action). A level's `procedureSource` keeps the whole file, a
  cursor (`Position`/`Seek`/`Rewind`), and an `end` (a CALL level's is
  its ENDSUBROUTINE). **Flow control** (`dcllabel.go`, `dclif.go`,
  `dclcall.go`): each level's `flowState` (the terminal's is
  `Shell.terminalFlow`) holds its labels, open IF blocks, the lines
  being passed over (`blockSkip`), and GOSUB's returns.
  `DispatchConsole` passes over a branch's lines unread (`skipLine`),
  records a label (`takeLabel`), and checks for a block's THEN
  (`awaitThen`) before substitution. GOTO, GOSUB, and CALL find a label
  where it was last processed or by a forward search that follows the
  blocks it passes (`searchLabel`); a missing one leaves the procedure
  at its end of file. CALL runs `label: SUBROUTINE`'s lines at a new
  level (`runLevel`, as @ does); running line by line passes over a
  subroutine.
  `vax.init` is a DCL procedure, run like a login command procedure
  (`RunHostProcedure`); the one-shot command on govax's command line
  is then level 0's input (`RunCommandLine`). The debugger's `@` reads
  the VMS debugger's format, every line a command
  (`RunDebuggerProcedure`).
  **Symbol substitution and expressions** (Phase 50): `DispatchConsole`
  replaces `'SYMBOL'` (and `''SYMBOL'` in quotes) first
  (`dclsubst.go`), then `dispatchCommand` does the rest: NOCOMD, the
  verb as a symbol (an alias's value isn't looked up again), `&SYMBOL`
  (its value one quoted token, so its case and blanks are kept; none in
  `:=` or `@`), the grammar, and IVVERB for an unknown verb. `=`
  evaluates DCL's expressions (`dclexpr.go`); lexical functions are a
  table (`dcllexical.go`'s `lexicalFunctions`, one entry per function;
  each function's keywords are a table too, read by `lexicalKeyword`),
  implemented by subject in `dcllexstring.go`, `dcllexprocess.go`
  (F$CONTEXT's selections for F$PID in `dcllexcontext.go`: a
  PROCESS_CONTEXT symbol, `dclSymbol.context`, naming a context the
  Shell keeps), `dcllexfile.go`, `dcllexname.go`, and `dcllextime.go`;
  F$CSID, F$GETQUI, and F$LICENSE are LEXNOTIMPL by choice; F$CONFIG,
  govax's own (not VMS's), reads the configuration (`dcllexconfig.go`). F$GETJPI,
  F$GETSYI, F$GETDVI, F$DEVICE (`DeviceScan`), F$IDENTIFIER, F$FAO, and
  F$SETPRV use the services' own code (`corevms/lexical.go`), F$PARSE
  and F$SEARCH RMS's (`rms/lexical.go`), F$FILE_ATTRIBUTES the file
  header (`rms/fileattr.go`, `Session.FileAttributes`). The messages are DCL's CLI$_ texts from
  `vmsdef.Messages`, shown as `%DCL-` (the CLI facility's display name),
  some with a segment line (`vmserrors.NewSegment`). `testdata/dcl50` is
  the VMS probe, and `TestProbe50Oracle` replays it against VMS 7.3's
  log.
  **Verification** (`dclverify.go`): SET [NO]VERIFY and SET [NO]PREFIX;
  the procedure loop makes each line a pending echo (`Shell.echo`),
  which `DispatchConsole` shows after substitution (a continued command
  a record at a time, through `joinMark`) or drops; F$VERIFY in a
  comment is performed and shown as its value (round 2 of the probe,
  `TestProbe50VOracle`). The console's SET has no `name=value` form:
  DCL's assignments replaced it.
  **SYS$INPUT** (`dclinput.go`, subtask 16): each procedure line runs
  with process 1's input redirected (`corevms.Environment.RedirectInput`)
  to a `procedureInput`, which pulls the level's next data record
  (`procedureSource.readData`) as an image reads, so LIB$GET_INPUT,
  RMS's `$GET`, and terminal `$QIO` reads get the data lines up to the
  next `$` line; TYPE SYS$INPUT reads the same way. DECK/EOD (a deck in
  `procedureSource`). Data nothing reads is SKPDAT after an image and a
  command after one of DCL's own commands (`dclOwnCommands`, by the
  grammar's syntax name; `Shell.imageRan`).
  **DCL's file I/O** (`dclfile.go`, subtask 17): OPEN opens a file
  through `rms.Session.OpenDCLFile` (`internal/rms/dclfile.go`: a
  volume file by `$OPEN`'s and `$CREATE`'s code, among the file's RMS
  openers, with a `FileHandle`'s record stream; a host file as lines)
  and defines its logical name (process table, supervisor mode);
  `Shell.dclFiles` keeps it open past its procedure, until CLOSE or
  `EndSession`. READ/WRITE need no OPEN for SYS$INPUT, SYS$OUTPUT,
  SYS$ERROR, and SYS$COMMAND (`processPermanentFile`); READ SYS$INPUT
  takes the procedure's next record, SYS$COMMAND and INQUIRE read the
  terminal (`readTerminal`). The logical name is the way to the file
  (`openFile`; TYPE of it reads the same stream). /ERROR and
  /END_OF_FILE branch with `branchStatus`, which stops `procedureLine`'s
  ON action and leaves the procedure's own status (`endStatus`, EXIT's)
  CLI$_NORMAL.
  **STOP and Ctrl/Y** (`dclcontroly.go`, subtask 18): STOP sets
  `Shell.stopping`, which ends every level; an attention key no AST
  takes is DCL's in a procedure (`HandleAttention` during an image,
  `takeControlY` after a DCL command), then forgotten
  (`Engine.ClearAttention`) so cmd/govax doesn't end govax for it: ON
  CONTROL_Y's command, a nested level's end, or the Ctrl/Y level
  (`controlYLevel`: CONTINUE, EXIT, STOP; an image command,
  `leaveControlY`, ends the procedure and runs at level 0,
  `afterStop`). SET [NO]CONTROL (`ControlYEnabled`, asked by
  cmd/govax's key handling). The keys go to the process the terminal
  is attached to (`corevms.Environment.Attached`, down the chain of
  waiting SPAWNs; Phase 51): a subprocess's DCL takes them
  (`subprocessAttention`), interrupting its image with an AST to its
  CLI stub (`imageInterrupted`, subcli.go's "Ctrl/Y").
  **Between commands** (`dcllend.go`, subtask 21): after each DCL
  command (`commandBoundary`: `runLevel`, and the subprocess's level-0
  loop) process 1 lends the CPU (`lendCPU`, corevms's `lend.go`:
  `LendCPU` yields, `sched.Scheduler.Yield`; the machine runs the others
  until the scheduler chooses process 1, stopped before it runs an
  instruction; `ReclaimCPU`), and a subprocess's DCL yields from its
  shim (`yieldCommand`, `Environment.Yield`); so SPAWN/NOWAIT procedures
  interleave with process 1's. At process 1's level 0 (`Dispatch`)
  `LendWhileIdle` waits until the others are idle, at most
  `promptLendLimit`; then, while cmd/govax reads the next line at a
  terminal on a goroutine of its own (`readCommandLine`),
  `LendWhileReading` lends it until the line comes (Phase 51, subtask
  4), the host waiting `promptPoll` at a time when every other process
  waits (corevms's `HostIdle`: nobody's CPU time; in quantum-clock mode
  the clock moves on by the host's time, no further than the next
  timer). cmd/govax's `promptOutput` sends their output through
  readline's writer in whole lines, which redraws the line being typed,
  and `attentionStdin.lineRead` keeps programs' terminal reads off the
  keys meanwhile. Input from a pipe or a test stays synchronous. WAIT
  lends the CPU (a subprocess's waits in the scheduler, `yieldTimer`).
  Not inside a machine run (`consoleCommands`, frozen scheduling), the
  debugger, or ASM.
- `internal/debugger` — the machine debugger (Phase 42), modeled on the VMS
  debugger: a `Debugger` session and its `Dispatcher` over `debug.dcl`
  (`internal/bootdata/files`), with `debug.help` and the `DBG> ` prompt.
  **The console and the debugger are two front ends to one machine, each
  with its own grammar:** `console.dcl` has the VMS command line (RUN,
  MACRO, LINK, MOUNT, DIRECTORY, DEFINE, SET DEFAULT, SAVE/LOAD, ...) and `debug.dcl` the machine's commands (EXAMINE, DEPOSIT,
  EVALUATE, STEP, SET/SHOW/CANCEL BREAK, TRACE, WATCH, SHOW REGISTERS,
  CALLS, IMAGE, SYMBOL, SET MODE/RADIX, ...); `TestGrammarSplit` says which
  command is in which. The debugger works on a `*machine.Machine`, which
  knows it only through the `machine.Debugger` interface
  (`internal/machine/debugger.go`); what it needs of the console (its
  routing for `@file`, its verbs, debugger procedures) is the
  `debugger.Console` interface, which `*console.Console` implements, so
  its own code doesn't import `internal/console` (Phase 52). `cmd/govax`
  installs it, and the console works without one. A session
  GO, CALL, and ASM are debugger commands (live code): the console has
  no GO or CALL, and accepts ASM only until the microkernel is in place
  (`Machine.KernelPlaced`, set when ASM defines EXE$INITIALIZE), which is
  how `vax.init` loads it before `$ DEBUG` and the data lines
  `GO EXE$INITIALIZE` / `EXIT`.
  Tests that need them install the debugger and enter its mode with
  `consoletest.InstallDebugger` and `consoletest.DebugMode`. A session
  starts at the console's `DEBUG`, when a `GO`/`CALL` stops, and for `RUN`
  of an image linked `/DEBUG` (or `RUN/DEBUG`), stopped at the main
  routine's first instruction; a run that ends by itself returns to
  `VAX>`. While a session is active `Dispatcher.Dispatch` routes lines to
  the debugger (`Interpreter.DispatchConsole` is DCL's, which
  `XFC$CONSOLE_CMD` uses); `EXIT` returns. Run control (`runcontrol.go`,
  `step.go`, eventpoints), EXAMINE/DEPOSIT (`data.go`, `examine.go`), the
  display modes (`modes.go`), source lines (`source.go`), and RUN under the
  debugger (`image.go`) are here, and the debugger reaches the machine
  through `internal/machine/export.go`. Its output is the VMS debugger's, checked
  against the VMS 7.3 logs of `testdata/dbg` and `testdata/dbgcmd` by
  oracle tests; `TestDebuggerSessionOracle` replays every session and
  keeps the list of what still differs (`expectedDifferences`).
  `SET MODE` takes the VMS display modes and govax's access modes;
  the console and the debugger each have their own radix. `SET PROCESS
  [pid|name]` (`process.go`) moves the CPU to another process so the
  debugger shows its context (STEP and breakpoints stay with process 1),
  and the debugger's `SHOW PROCESS` lists the processes. **Symbols are
  split the same way:** the console's SHOW SYMBOL and DELETE/SYMBOL are
  DCL's (`internal/dclshell/dclsym.go`'s `dclSymbolTable`: a global table
  and a local table per command level), and the machine's symbol table
  (`Machine.Symbols`, ASM's symbols and the predefined ones) is the
  debugger's SHOW SYMBOL (`/ALL`, `/SYSTEM`) and CANCEL/CLEAR SYMBOL.
- `internal/io` — device abstraction (Phase 09).
- `internal/vmsdef` — VMS's own definitions, shared by the assembler, RTL, RMS,
  and LINK: `Symbols` (every symbolic constant, one table; `symbols.go` says
  what each prefix holds), `Messages`, the FAB/RAB layouts, the P1 vector, and
  the symbols LINK would read from VMS files (`SharedImages`/`ImageSymbols`
  from LIBRTL.EXE, `LibrarySymbols` from STARLET.OLB's definition modules).
  The `*_generated.go` tables are committed data, and building doesn't
  regenerate them: `go run ./internal/vmsdef/gen` merges new definitions
  (`-h`/`-sdl`/`-bliss`/`-msg`/`-image`/`-olb FILE`) into them, add-only
  unless `-replace`; `-drop NAME` removes a stale name, `-n` is a dry run
  (Phase 31).
- `internal/lnm` — VMS logical-name database: directories, tables, access modes,
  search lists, `$TRNLNM`-style lookup and RMS file-spec translation (Phase 25).
  A leaf package shared by the console, `internal/rms`, and `internal/corevms`.
- `internal/corevms` — VMS RTL/system-service simulation (Phase 10). Since
  Phase 43, what every process shares is a `System` (`system.go`: the
  machine, service and shim registries, devices, mounts, mailboxes, common
  event flags, OPCOM, the clock, the process table (`proctable.go`, real
  PIDs), the S0 page pool (`s0pool.go`), and `ProcessSettings`), and each
  process is an `Environment` that embeds it, with its `Space`
  (`addrspace.go`: `BuildAddressSpace`, the shared P1 vector, teardown)
  and `Stacks` (`stacks.go`: privileged stacks and the PCB page). Process
  1 is the console's, on VMINIT's tables and stacks; the machine keeps
  each process's image state (`internal/machine/images.go`). Phase 44's
  scheduler: the `System` owns a `sched.Scheduler` kept in step with the
  table and is the engine's hook (`schedule.go`: `Schedule`, `switchTo`,
  `SwitchCPU`; installed by the machine's `newRTL` with
  `InstallScheduler` when `vax.process.scheduler` is on). Phase 45:
  `$CREPRC` (`creprc.go`, quotas in `quotas.go`), process startup in the
  new process's context (`startup.go`), and deletion (`delete.go`:
  `DeleteProcess`'s rundown; the memory of the process the CPU is in is
  freed by `switchTo` once the CPU leaves it; `$DELPRC` of another
  process only marks it, `deletePending`, and `switchTo` deletes it when
  it gets the CPU; an owner's deletion marks its subprocesses and waits,
  MWAIT, until they've gone), and the termination
  message to the creator's mailbox (`termmsg.go`). An I/O request
  completes for the process that made it (`ioRequest.owner`, its IOSB
  through that process's address space). A waiting
  service says what it waits for (`waits.go`: `waitOn`, a state and a Go
  test), the process waits in the scheduler, and each scheduling call
  expires every process's timers and tests the waiters (`pollEvents`);
  with nothing to run, `idle.go` moves time to the next timer. Phase 46:
  an event is reported when it happens (`reportEvent`, SCH$RSE: an I/O
  completion with its request's boost class, a timer, a common event
  flag set through `postFlag`, mailbox room), making the waiter
  computable at once; UIC protection (`uicprot.go`) on mailboxes and
  global sections; global sections (`gblsec.go`: `$CRMPSC`, `$MGBLSC`,
  `$DGBLSC`, page-file sections whose frames the section owns,
  `System.releaseFrame`); mailbox and NL: record streams for RMS
  (`recdevice.go`; `LIB$PUT_OUTPUT` writes to a mailbox SYS$OUTPUT); and
  the shared terminal (`terminal.go`: one buffer, reads in FIFO order,
  a read with no whole line waits in LEF, with the scheduler on and a
  `TerminalSource` input; since Phase 49 a `$QIO` read stays pending
  instead, completed by `serviceTerminal` at each scheduling call). CPU time
  and SHOW SYSTEM/SHOW PROCESS's reports are in `showsys.go`. Phase 47:
  `System.Locks` is the one lock database (`internal/lck`); `$ENQ`,
  `$ENQW`, `$DEQ` are `enq.go` (each request's `enqRequest` is its
  lock's notifier: LKSB, event flag, completion and blocking ASTs in the
  owner's process); `$GETLKI(W)` is `getlki.go` (Phase 49); image rundown dequeues user-mode locks, process
  deletion all (`locks.go`); the RMS context carries the database, the
  PID, and a record-lock wait (`awaitLock`, `lockWaker`). The
  machine's engine hooks reach the current process
  (`Machine.RunningProcess`, `corevms.EngineHooks`); `Machine.ProcessOne`
  is process 1, and only process 1's image ending ends a console run
  (`Machine.StepMachine`). A run that stops in another process leaves the
  CPU there until the next run or the debugger's EXIT
  (`Machine.ReturnToProcessOne`). Phase 48: `LIB$SPAWN`'s process side
  (`spawn.go`: the subprocess, its SYS$ names, copied logical names and
  symbols, the completion in Go at deletion; `Environment.SpawnNotice`);
  processes that run a CLI rather than an image (`cliprocess.go`:
  `CommandInterpreter`, the console's; `$CREPRC` of LOGINOUT;
  `OpenCommandInput`; LOGOUT's report, `logout.go`); a SYS$OUTPUT that
  names a file (`outfile.go`). The scheduler is on by default
  (`vax.process.scheduler`). Every completed `$QIO` counts in its
  device's operations (UCB$L_OPCNT; a mounted disk's are ods2's).
  Phase 49: file-backed sections (`filesec.go`): `$CRMPSC` with a
  channel, private (the process section table, `processSection`) or
  global (the global page table, `GlobalSections.pageTable`), pages read
  in by the system's pager (`System.PageIn`, installed on `vm.Memory`)
  when first touched; `releasePTE` is the one place a page leaving a PTE
  is accounted for (a private modified page written back, a global one
  marked `Dirty`, written back when the section goes); `$UPDSEC(W)`; a
  section keeps its file accessed after its channel goes
  (`sectionFiles`); P1 expansion (`expandP1`, below the user stack's
  reserve, `userStackPages`) for `SEC$M_EXPREG` and `$EXPREG` region 1.
  A section ends at the file's end of file. Per-process I/O counts
  (`iocount.go`: JPI$_BUFIO/DIRIO, counted as each `$QIO` completes; a
  disk's and NL:'s are direct I/O); EXE$OPEN's host files are shared by
  RMS's rule (`System.HostOpeners`, `rms/hostshare.go`).
- `internal/librtl` — LIBRTL.EXE's routines (Phase 34): the LIB$ and STR$
  shims a program reaches through `SHIM$LIBRTL_<offset>` stubs. `Routines`
  lists each with its XFC$SHIM code (its transfer-vector offset is
  `vmsdef.ImageSymbols`'s, `Routine.Offset`); code 0 is a routine in VAX
  code in `kernel.asm` (LIB$CALLG; Phase 53, Decision 1). The stubs fill
  VMINIT's page, then as many S0 pages as they need (`EnsureShims`).
  The console registers them into each RTL environment and builds the
  stubs from the same table. Strings go through `descriptor.go`
  (Phase 53): `readString` takes every input class (Z, S, D, SD, A,
  NCA, VS), `writeString` writes by the output's class (fixed, VS, D),
  and `stringFailure` maps their errors to LIB$_INVSTRDES and the rest.
  An argument a routine can't reach is a signaled SS$_ACCVIO
  (`fault.go`'s `memoryFault`, through `Environment.SignalFailure`), and
  every LIBRTL CALL entry govax lacks has a stub (`missing.go`,
  `Missing`, codes from 1000) that signals SS$_UNSUPPORTED and
  GOVAX-E-NOTIMPL (`vmsdef.GovaxNotImplemented`) with its name. The process
  machinery they use (condition dispatch, the heap, memory) stays in
  `internal/corevms`, reached through `export.go`. Each further *RTL.EXE emulated
  gets a package like it. New routines are written from DIGITAL's manuals
  (clean room), and checked on VMS where a probe can: LIB$CREATE_DIR
  (`createdir.go`) against `testdata/credir/libcrd.mar`'s VMS 7.3 run
  (`TestLibCreateDirOracle`). The instruction routines (Phase 53: bit
  fields, ADDX/SUBX/EMUL/EDIV/ADAWI, EMOD and POLY, the interlocked queues
  and BBSSI/BBCCI, LOCC to MOVTUC, CRC; `bitfield.go`, `math.go`,
  `float.go`, `queue.go`, `chars.go`, `crc.go`) do their work with the
  CPU's own code, exported from `internal/cpu` as functions of addresses
  (`cpu.MemoryField`, `cpu.InsertInterlocked`, `cpu.LocateCharacter`,
  `cpu.MoveCharacters`, `cpu.CRC`, ...); `testdata/lib53`'s probe
  (`TestLib53Oracle`) matches VMS's run of all 132 cases. XFC$SHIM code
  64 is DCL's `EXE$CLI_COMMAND`, so no LIBRTL routine may use it
  (`TestCLIShimCodeFree`). Phase 54: the table spans LIBRTL2.EXE too
  (each name's image is `vmsdef.ImageSymbols`'s; LIB$CVT_DX_DX,
  `cvtdxdx.go`). **JSB entries** (`JSBRoutines`, a
  `corevms.JSBShimFunc` on R0–R6) have stubs that push their code for
  `XFC #XFC$JSBSHIM` (selector 5, `corevms.JSBShimStub`), and a
  condition one signals is raised in its caller's frame; **data
  entries** (`Tables`, `tables.go`: LIB$AB_ASC_EBC, LIB$AB_EBC_ASC,
  LIB$AB_UPCASE) are copied after the stubs (`ShimLink.Data`). Every
  LIBRTL and LIBRTL2 entry resolves (missing JSB stubs, `MissingJSB`,
  codes from 2000; LIBRTL2's `LIB$AB_` tables excepted). Dynamic
  strings (`dynstr.go`, LIB$SGET1_DD and the SFREE routines) keep a
  heap block while it holds the text (`Environment.BlockSize`), and
  `writeString`'s class D output shares the rule. Beside them `copy.go`
  (SCOPY, ANALYZE_SDESC), `strfuncs.go` (LEN, CHAR, ICHAR, INDEX),
  `translate.go`, `cvttb.go`, `lookupkey.go` (and govax's
  `$LIB_KEY_TABLE` macro), `locale.go`. Phase 55: the service jackets
  (`jackets.go`, through `corevms.Environment.CallService` and a scratch
  buffer in the routine's frame), the item routines (`items.go`, over
  `corevms`'s `ProcessItem`/`DeviceItem`/`SystemItem` and the item text
  forms in `corevms/itemtext.go`, shared with the lexical functions in
  two `Style`s), the event flag and LUN pools (`resources.go`, highest
  first) and the timers (`timers.go`; per-image data in
  `Environment.ImageData`; SHOW_TIMER's action routine is a tail call,
  `CallRequest.Tail`), `misc.go` (AST_IN_PROG, ASN_WTH_MBX, WAIT,
  GET_USERS_LANGUAGE and the `vax.language` key), and `protection.go`
  (FORMAT/PARSE_SOGW_PROT, PARSE_ACCESS_CODE, GET_ACCNAM's FILE
  table). Phase 56: the time arithmetic (`times.go`), conversions
  (`cvttime.go`, the LIB$K_ operations as a table; CVT_VECTIM over
  `corevms.VectorTime`), `days.go`, and the formatting routines: the
  mnemonics and parser (`dtformat.go`), the context (`dtcontext.go`, a
  tagged heap block with Go state per image; what INIT didn't give comes
  from the logical names), output (`dtoutput.go`) and input
  (`dtinput.go`, a format-driven matcher); the predefined formats and
  the system's default names are `internal/lnm/dtformat.go`'s.
  LIB$PUT_OUTPUT is here too (`output.go`); kernel.asm's
  old interrupt-driven one is now its private `EXE$PUT_OUTPUT`.
  LIB$GET_FOREIGN (`foreign.go`) returns `Environment.CommandLine`: a
  foreign command's text (DCL symbols, `internal/dclshell/dclsym.go`), or
  what follows the image on `govax run IMAGE text...`.
  LIB$GET_INPUT (`input.go`) reads a line from the terminal with the
  terminal's rules (`Environment.ReadInputLine`). LIB$SPAWN (`spawn.go`,
  Phase 48) reads its 13 arguments and calls `Environment.Spawn`. The
  subprocess runs the console's own DCL (`internal/dclshell/subcli.go`,
  Phase 50's subtask 19): its interpreter is a goroutine that takes
  turns with the engine (strict handoff through its stub's shim,
  `EXE$CLI_COMMAND`: resume, then a `cliYield` to wait, give the CPU up
  after a command, call an image, or log out), with the process's own
  `Shell` (its RTL, Logicals, ContainerSession, Out, symbols, levels,
  $STATUS, ...) the interpreter's current one while it runs
  (`useShell`; `Machine.ProcessOne` is process 1 throughout, `subCLI`
  the subprocess's CLI). RUN there is
  `subprocessRun`; `processOneCommands` (DEBUG, ASM, VMINIT, ...)
  are VAX_NOTSUBPROC. DCL's SPAWN (`internal/dclshell/spawncmd.go`) calls
  LIB$SPAWN in process 1; in a subprocess (Phase 51) it calls corevms's
  `Spawn` from Go and waits with `AwaitSpawn` through a `cliYield`
  (`spawnInSubprocess`).
- `internal/disasm` — the disassembler (Phase 11; moved out of `internal/asm` in
  Phase 41 so other packages can use it): `Disassemble` decodes one instruction
  with `internal/cpu`'s table into a `Decoded` of structured `Operand`s
  (`operand.go`: mode, registers, displacement, value, and the `Target`
  address where it's known). `String` (`format.go`) renders the text
  `internal/asm` reassembles; `Format(Options)` (`symbolic.go`) renders in a
  `Style`, `StyleDebugger` being the VMS debugger's `EXAMINE/INSTRUCTION`
  text, with names from a caller's `Symbolizer` (and, optionally,
  `ConstantNamer` and `CellNamer`). It knows no symbols itself. It imports
  `cpu` and `vaxfloat`, never `asm`. `cpu.RegisterName` and
  `cpu.DataType.FloatFormat` are shared by the CPU, assembler, and disassembler.
- `internal/symtab` — a symbol table searched both ways (Phase 41): `Table`
  by name (ignoring case) and by address (`At`, and `Nearest` for
  `NAME+offset`), each `Symbol` with `Flags` (entry, label, data, literal,
  psect, module, global, ...) and a `Scope`. The assembler's `Symbols()`, the
  console's `SymbolTable`, and `internal/dbgsym` all keep their symbols in
  it. A leaf package.
- `internal/vmsimage` — decodes a VMS image file's header blocks, ISDs, and
  fixup section (`ReadImage`; moved out of `internal/anl` in Phase 41, and not
  named `image` so as not to shadow Go's). Exports the `IHD`/`ISD`/`IAF`/`SHL`
  layouts; `DSTBlockCount`/`GSTRecordCount` read the IHS's 32-bit sizes.
- `internal/dbgsym` — an image's debug symbol table (Phase 41): `Read` turns
  the DST (`read.go`, `records.go`), the line-number program (`lines.go`),
  source correlation (`source.go`), the debug module table (`dmt.go`), and the
  GST (`gst.go`) into a `Program` of modules, routines, labels, data,
  psects, and lines, relocated by the image's load base. Lookups go both
  ways: `Lookup` of a path (`MOD\ROUTINE\LABEL`), `AddressOfLine`, and
  `Symbolize`/`LineAt`/`RoutineAt`/`ModuleAt` of an address, by the
  debugger's rules (`symbolize.go`; `Names` adapts it to
  `disasm.Symbolizer`). Its rules come from the probe in `testdata/dbg`
  (VMS 7.3 debugger sessions, `vax/*.dlg`, and their images). The console
  keeps one per loaded image (`ICB.Debug`) for `DISASSEMBLE` (symbolic by
  default), the trace, `STEP`, `SHOW CALLS`, and path names and `%LINE n`
  in expressions (`dbgnames.go`, `dbgtrace.go`, `dbgcalls.go`);
  `TestDebuggerOracle` matches `DISASSEMBLE` with every symbolic
  `EXAMINE/INSTRUCTION` in the sessions. Groundwork for a debugger.
- `internal/asm` — assembler (Phase 11). Two dialects share one core
  (Phase 27): the console's `ASM` (absolute, into emulated memory, eVAX
  directives) and MACRO-32 (`SetDialect(DialectMACRO)`: psects, relocation
  trees, every error reported, and `Object()` for a `.OBJ` module). Phase 28's
  macro facility is in both: macro definitions and calls (`macros.go`,
  `macroargs.go`), repeat blocks (`repeat.go`), and a stack of sources
  (`source.go`); the MACRO dialect also searches macro libraries
  (`MacroLibrary`, `maclib.go`) handed in by the console's MACRO command.
  `overwrite.go` keeps a field stored twice (as `$FAB` does) as real MACRO
  writes it. Phase 29 adds MACRO listings: `SetListing` records a
  `listLine` per source line (`listing.go`), and `Listing` lays out the
  source pages (`listpage.go`), the listing controls (`listctl.go`),
  messages, the cross reference (`xref.go`, `SetCrossReference`), and the
  closing pages (`listclose.go`), as real MACRO lays them out. `Object()`
  writes traceback (TBT) records by default (`traceback`; `SetFunctions`
  is `/ENABLE=`/`/DISABLE=`, and the console maps `/DEBUG` onto it).
  With `.ENABLE DEBUG` (`/DEBUG`), it writes debugger (DBG) records too
  (`debug.go`, Phase 29's subtask 12): the line-number table, whose DBG
  records go out among the TIR records as real MACRO's do, and, with
  traceback, a symbol record for each symbol, typed by the data
  directive after a label. `TestDebugRecords` matches 13 real `/DEBUG`
  objects whole; FORTH's (`TestDebugRecordsForth`) differs only in the
  `$$` symbols VMS's and govax's STARLETs define.
- `internal/obj` — the VAX object language (Phase 27): reads, writes, dumps, and
  checks `.OBJ` object modules, keeping every record so a real VAX object
  round-trips byte for byte; `Builder` packs a module's psects, symbols, and TIR
  commands into records. Codes and layouts come from `vmsdef.Symbols`
  (the VAX object language names of VMS 7.3's `objfmt.sdl`). Host files hold records in ODS-2's
  on-disk variable-length layout (`ReadRecords`/`WriteRecords`).
  `dst.go` (Phase 29) decodes and encodes the debug symbol table (DST)
  records TBT and DBG records carry, and builds real MACRO's four
  traceback records; their layouts come from real objects (clean room),
  checked against `docs/DEBUG-RECORDS.md`. `dbg.go` builds the debugger
  records' (symbols with descriptors, source correlation, line-number
  commands), and `dbglines.go`'s `LineTable` packs a line-number table
  into DBG records as real MACRO does; `Builder.Insert` places them among
  the TIR records, and `Builder.Debug` adds the symbol records.
  and `obj.Dump` shows them. `Builder.Traceback` adds TBT records.
- `internal/rms` — RMS (`SYS$CREATE`/`CONNECT`/`OPEN`/`CLOSE`/`GET`/`PUT`/`RENAME`,
  and Phase 33's `PARSE`/`SEARCH`/`DISPLAY` with NAM blocks and XABs) file
  I/O backed by the sibling Go module `github.com/tucats/ods2`'s real ODS-2
  volume/file implementation, plus the `MOUNT`/`DISMOUNT`-facing `MountTable`
  (Phase 22). Also decides whether a typed file name means a host file or a
  volume file (`Session.Locate`, `location.go`) and reads/creates record files
  on either side (`ReadRecordFile`/`CreateRecordFile`, and `RewriteRecordFile`,
  which keeps a volume file's version; `recordfile.go`; Phase
  27). The sole place in this project allowed to import `ods2`; owns its
  own IFI (open-file) table separately from `internal/corevms`'s state, since it
  tracks real `ods2` handles Phase 10's RTL layer never needed. `go.mod`
  pins a tagged `ods2` release, so a plain clone builds; a local, gitignored
  `go.work` (`use .` / `use ../ods2`) overrides it with the sibling checkout
  for co-development (see `docs/PHASE-22.md`'s "Dependency: `go.work`, not a
  `replace` directive"). Because the workspace hides a stale pin, when govax
  starts using new `ods2` API, tag and push `ods2`, then
  `GOWORK=off go get github.com/tucats/ods2@vX.Y.Z` and check
  `GOWORK=off go build ./...` before pushing govax. Name processing
  (`name.go`: default names, related files, logical names, search lists)
  is shared by every service, and `$SEARCH` (`search.go`) keeps VMS's two
  kinds of wildcard context; Phase 33's runtime oracle (`testdata/mar/rms3`,
  `TestRMS3Oracle`) checks them against VMS 7.3 byte for byte.
  `Session.CreateDirectory` (`createdir.go`, Phase 34) is the console's
  `CREATE/DIRECTORY` (`internal/dclcmd/create.go`): ods2 lays the directory
  files out (`volume.CreateDirectory`, `filespec.CreateDirectoryPath`), and
  `TestCreateDirectoryOracle` (`testdata/credir`) checks headers and
  messages against VMS 7.3's run. A RAB connected to the terminal reads lines from the
  console's input, prompting with RAB$L_PBF when RAB$V_PMT is set
  (`terminal.go`; with the shared terminal, through
  `Context.AwaitTerminal`, Phase 46). A name whose device is a mailbox or
  NL: opens a `RecordDevice` stream from `Context.Devices` (`recdevice.go`,
  Phase 46; `internal/corevms` is the opener). Phase 47: files shared
  between processes. Every `$OPEN`/`$CREATE` and IO$_ACCESS goes through
  ods2's `Volume.Access` (`sharing.go`: the File Applications guide's
  per-operation FAC/SHR rule over a per-file list of RMS openers, and
  ods2's coarser write/no-read/no-write counts underneath; RMS$_FLK,
  SS$_ACCONFLICT), and while a
  file is open every opener shares ods2's one `*File` (the FCB: header,
  extents, end of file); a file deleted while open goes at its last close.
  RAB$V_EOF appends (RMS$_NEF otherwise); a stream on a file others may
  use writes each record through at the current end of file. Record
  locks (`recordlock.go`, by RFA, on `Context.Locks`), `$FLUSH`,
  `$ERASE`, `$FREE`, `$RELEASE`; `$SEARCH` contexts reopen directories
  each call; `MountTable.VerifyVolume` (`verify.go`) checks a volume;
  DISMOUNT refuses a volume with files open (`DismountAll` forces).
  Phase 49: each stream's current and next records (`stream.go`, the
  File Applications guide's Table 8-3), shared by `$GET` and `$FIND`
  (`locate`; sequential or by RFA), and `recordops.go`'s `$FIND`,
  `$UPDATE` (in place, same length), `$TRUNCATE`, `$DELETE` (RMS$_IOP
  on sequential files), `$REWIND`; `$PUT` with RAB$V_TPT; one stream may
  both read and write. User file open (`ufo.go`, FAB$V_UFO): `$OPEN`/
  `$CREATE` hand the file, as an `ACPFile`, to a channel
  (`Context.AssignFileChannel`), its number in FAB$L_STV; `ACPFile`'s
  `ReadPage`/`WritePage` are a section's page I/O. A write-shared UFO
  needs FAB$V_UPI (RMS$_SHR), a UFO `$CREATE` ends the file at block
  ALQ, and a UFO of a device that holds no files is a plain channel to
  it. A stream file's `$UPDATE` may change a record's length (written
  over the bytes there, as VMS does). RMS's share of the I/O counts is
  a model fitted to VMS's (`iocount.go`, `Context.CountIO`).
  Phase 51: `Session.Directory` lays out DIRECTORY as VMS 7.3 does
  (brief columns, /SIZE, /DATE, /FULL, totals; every version when none is
  given), `Session.NotFound` is a missing file's expanded name and
  RMS$_FNF/DNF for DCL's messages, `fileattr.go` copies a header into a
  `FileAttributes` (F$FILE_ATTRIBUTES, DIRECTORY), `dump.go` reads blocks
  and records with RFAs for DUMP, and `$PUT` and DCL's WRITE record the
  longest record (FAT$W_RSIZE, `noteLongest`). ods2's writer ends a
  block's variable-length records with VMS's FFFF word.
- `internal/link` — the VAX linker (Phase 30): builds a VMS executable image from
  `internal/obj` modules, laid out as real LINK lays images out (byte for byte on
  the fixtures). The console's `LINK` command (`internal/dclcmd/link.go`) drives it.
  Undefined symbols come from symbol sources (`source.go`, `libsource.go`):
  IMAGELIB.OLB plus shareable images' GSTs, STARLET.OLB, then govax's own tables
  (`internal/dclcmd/linksource.go`). With traceback (the default), pass 2
  also runs each module's TBT records into the image's debug symbol table,
  which follows the image's other blocks and the IHS block points at
  (Phase 29); DBG records are skipped. `LINK/DEBUG` (`Options.Debug`,
  Phase 29's subtasks 16 to 20) runs the DBG records into the DST with the
  TBT records, sets IHD$V_LNKDEBUG, and adds the debug module table and
  global symbol table after the DST (`debug.go`). The three real `/DEBUG`
  images (TRDBGLNK, TRLNKDBG, FORTH) match byte for byte, except that
  govax pads the GST to a whole block where real LINK ends the file
  mid-block (Decision 7). Fixup cells for a shareable image
  are in symbol-name order (`orderCells`).
- `internal/lbr` — the librarian file format (Phase 30): reads `.OLB`/`.MLB`/etc.
  libraries, their B-tree indexes and module records, including DCX data-reduced
  libraries (`dcx.go`) such as STARLET.OLB. Writes them too (Phase 28): `Builder`
  (`Create`/`Edit`, `Insert`/`Replace`/`Delete`, `Bytes`) lays a library out as
  VMS's librarian does, and `MacroModules`/`ObjectModules` (`input.go`) apply
  LIBRARIAN's rules for turning macro source and object files into modules;
  `List` is LIBRARY/LIST's listing, in LIBRARIAN's own formats. The console's
  `LIBRARY` command (`internal/dclcmd/library.go`) drives it. Imports only
  `internal/obj` and `internal/vmsdef`.
- `internal/anl` — VMS's ANALYZE utility (Phase 38): `AnalyzeObject` turns an
  object file's records into report `Line`s exactly as VMS 7.3's ANALYZE/OBJECT
  words them (`object.go`, `gsd.go`, `tir.go`, `dump.go`, errors in
  `check.go`), and `Pager` (`page.go`) lays them out on ANALYZE's pages, by a
  page-break rule reconstructed from real output (`Line.Keep`/`Spill`;
  docs/PHASE-38.md). `TestObjectPages` matches all 54 `.anl`/`.obj` pairs in
  `testdata/mar` byte for byte but for the time; `testdata/kinds.txt` shows
  the layouts no fixture settles. ANALYZE/IMAGE (Phase 40) is beside it:
  `vmsimage.ReadImage` decodes an image's header blocks, ISDs, and fixup
  section, and `AnalyzeImage` (`imagehdr.go`, `imagefix.go`) reports them on
  the same `Pager` (`TitleImage`). `TestImagePages` matches all 29
  `.ani`/`.exe` pairs (`testdata/link/vax`, `testdata/mar/list/vax`,
  `testdata/mar/round/vax`) byte for byte but for the time;
  `testdata/imagekinds.txt` shows the unconfirmed layouts. Debug table
  contents aren't shown (future expansion). The console's
  `ANALYZE/OBJECT` and `ANALYZE/IMAGE` (`internal/dclcmd/analyze.go`,
  sharing `analyzeFiles`) find files (and object libraries' modules) and
  write the report; in `console.dcl` each kind of analysis is a qualifier
  switching to its own syntax.
- `cmd/govax` — `main.go` (CLI entry point) plus `grammar.go` (the `tucats/gopackages`
  `app-cli/cli` option/subcommand grammar — `stats`/`path`/`instruction-limit`/
  `time-limit` options, repeatable `mount`/`mount-write DEVICE=container`,
  `console`/`asm`/`run`/`macro`/`link`/`library`/`analyze` subcommands;
  `analyze --image` is ANALYZE/IMAGE). A one-shot subcommand that fails
  makes govax exit nonzero (124, as timeout(1) does, when `--instruction-limit`
  or `--time-limit` stopped it), and volumes still mounted are dismounted (flushed)
  when a session ends. The control keys are VMS's (`attention.go`,
  `terminal_unix.go`; `HELP KEYS`): Ctrl-C interrupts, Ctrl-Y ends govax,
  Ctrl-Z is end of file. Briefly moved to the repo
  root (2026-09-17); moved back into `cmd/govax` as the more standard layout
  (`go build ./...`/`go run ./cmd/govax`).
- `tucats/gopackages` also brings config-settings support (`app-cli/settings`), read at
  `Engine` construction (`internal/cpu/engine.go`'s `NewEngine`). Settings implemented
  so far: `vax.hardware.clock` (bool) — when true, the system time is the
  host's, and `Engine.Step` looks at the host clock every 1,024 instructions, only while
  the interval clock runs or an interrupt is queued (`internal/cpu/clock.go`); when
  false/unset, `tickQuantum` counts instructions into emulated milliseconds
  (deterministic). TODR is computed when read, in both modes. The microkernel no
  longer starts the interval clock, and writes the console through XFCs
  (`XFC$CONSOLE_PUT`), not TXDB and its interrupt (docs/PERFORMANCE.md, Study 1 R1). `vax.quantum` (int) — default
  quantum-tick interval instead of the hard-coded `defaultQuantum` (20); only takes
  effect if `> 0`. `vax.disassemble.symbolic` (bool) — DISASSEMBLE's default for
  `/SYMBOLIC` (the VMS debugger's layout and names, Phase 41), and for SHOW
  CALLS'; the trace and STEP show debug images' locations symbolically when
  it's on; true when unset.
  `vax.default.volume.file`/`.label`/`.device`/`.type`/`.directory` — a
  container `cmd/govax` mounts at startup (creating it, sized by the type,
  with the directory in it, if it doesn't exist) and does a SET DEFAULT
  to (`dclcmd.Context.MountDefaultVolume`, `internal/dclcmd/defvolume.go`;
  defaults WORK, DUA0:, RD54, [WORK]; messages in `vmserrors/codes_mount.go`).
  Unset file: no default volume, names go to the host. Every key read must
  be in `cmd/govax/main.go`'s `validConfigs`, and documented in vax.help's
  `HELP CONFIG KEYS`.
  `vax.library` (string) — the host directory LINK and MACRO look
  in for IMAGELIB.OLB, STARLET.OLB, shareable images, and STARLET.MLB when
  `SYS$LIBRARY`/`SYS$SHARE` don't lead to them (read by the console, not `NewEngine`;
  `internal/dclcmd/syslib.go`); the older `vax.link.library` is still read when
  it isn't set. With no STARLET.MLB found, MACRO uses govax's own from bootdata:
  `files/starlet.mar` (system service and RMS macros, hand-written clean-room;
  the process and other service macros, in `_S`, argument-list, and `_G`
  forms, are checked against real MACRO's objects by `TestServiceMacroObjects`,
  from the probes in `testdata/mp/macros`, Phase 45; round 6 added the global
  section and lock service macros, Phase 46; `$UPDSEC`/`$UPDSECW`,
  Phase 49, unchecked)
  plus `files/starletdef.mar` (the `$xxxDEF` macros, generated by
  `internal/bootdata/mkdefs` from `testdata/mar/rms/defined.txt` and
  `testdata/mp/defs/defined.txt`), built into
  `starlet.mlb` by `go generate ./internal/bootdata` (Phase 32). Each RMS macro
  is checked byte for byte against real MACRO's objects for the oracle probes
  in `testdata/mar/rms` (`internal/asm/oracle_test.go`).

## Bug-fixing policy while porting

The C source's fidelity to the VAX ISA/hardware definition is good but not perfect.
When you hit a bug or suspicious behavior while converting a piece of C to Go:

- **Clear, obvious logic errors not tied to ISA semantics** (off-by-one, copy-paste
  mistakes, dead code, a condition that plainly contradicts its own comment) — just fix
  them in the Go code as you go. No need to ask or log these.
- **Suspected ISA/hardware-definition fidelity issues** — cases where the emulated
  behavior doesn't match the VAX spec, or doesn't match what the C code's own comments
  claim it does — default to **documenting and deferring**: record the finding in
  `docs/DEVIATIONS.md`, and have the Go port replicate the C source's current
  (possibly imperfect) behavior for now, to be revisited in a future debugging round
  (see Phase 12). Only fix immediately if the correct fix is clear-cut and fits
  naturally within the current change's scope.
- **When it's not obvious which of the above applies**, ask the user rather than
  deciding unilaterally — this is a judgment call by design.

This mirrors how `reference/eVAX/AUDIT.md` was produced on the C side: findings get
catalogued with enough detail to act on later, not silently patched over or ignored.
