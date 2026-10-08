# CLAUDE.md

This file provides guidance to Claude Code when working in this repository.

## Project overview

`govax` is a from-scratch Go rewrite (not a cross-compile) of `eVAX`, a C emulator for
a DEC VAX/VMS system. The goal is a careful evaluation of the C implementation followed
by rewriting it in idiomatic Go, with a comprehensive unit test suite the C version
never had.

- `docs/PLAN.md` — high-level plan, locked-in architecture decisions, and the phase
  index.
- `docs/PHASE-00.md` … `PHASE-48.md` — one doc per phase: goal, C-source file
  mapping, deliverables, open questions, and a dated progress log (all
  done through 47; 40 follows 38 directly: there is no Phase 39). Phases
  43–48 are the multiprocessing program (subprocesses, a scheduler,
  interprocess mailboxes and shared memory, RMS file sharing and the lock
  manager; 48 planned);
  `PHASE-43.md`'s Part A describes the whole program. Read the relevant phase doc
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

- `internal/vax` — core machine state: registers, PSL, condition codes (Phase 01).
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
  blocking notices. Operations return `Event`s for any owner, and
  `lck.Deliver` hands each to its lock's `Data` if that's a `Notifier`
  (how `$ENQ` completes in the owner's process and an RMS record-lock
  wait is woken). No deadlock detection. A leaf package; owners are PIDs.
- `internal/console` — interactive monitor + DCL grammar interpreter (Phase 08).
  Every console command is parsed by the DCL grammar
  (`internal/bootdata/files/console.dcl`); Phase 37 moved the last
  hand-parsed "fixed" commands (EXAMINE, SET, STEP, RUN, ...) onto it, with
  handlers in `commands.go` and `setcommand.go`. An address or value is an
  `$expression` parameter, whose extent the grammar finds and whose value
  the handler gets from the expression evaluator (`expr.go`). Unquoted
  text is uppercased, so a case-sensitive host file name must be quoted.
- `internal/debugger` — the machine debugger (Phase 42), modeled on the VMS
  debugger: a `Debugger` session and its `Dispatcher` over `debug.dcl`
  (`internal/bootdata/files`), with `debug.help` and the `DBG> ` prompt.
  **The console and the debugger are two front ends to one machine, each
  with its own grammar:** `console.dcl` has the VMS command line (RUN,
  MACRO, LINK, MOUNT, DIRECTORY, DEFINE, SET DEFAULT, SAVE/LOAD, GO and
  CALL, ...) and `debug.dcl` the machine's commands (EXAMINE, DEPOSIT,
  EVALUATE, STEP, SET/SHOW/CANCEL BREAK, TRACE, WATCH, SHOW REGISTERS,
  CALLS, IMAGE, SYMBOL, SET MODE/RADIX, ...); `TestGrammarSplit` says which
  command is in which. The debugger imports `internal/console`, which knows
  it only through the `console.Debugger` interface (`debugger.go`);
  `cmd/govax` installs it, and the console works without one. A session
  starts at the console's `DEBUG`, when a `GO`/`CALL` stops, and for `RUN`
  of an image linked `/DEBUG` (or `RUN/DEBUG`), stopped at the main
  routine's first instruction; a run that ends by itself returns to
  `VAX>`. While a session is active `Dispatcher.Dispatch` routes lines to
  the debugger (`DispatchConsole` is the console's own, for
  `XFC$CONSOLE_CMD`); `EXIT` returns. Run control (`runcontrol.go`,
  `step.go`, eventpoints), EXAMINE/DEPOSIT (`data.go`, `examine.go`), the
  display modes (`modes.go`), source lines (`source.go`), and RUN under the
  debugger (`image.go`) are here, and the debugger reaches the machine
  through `console/export.go`. Its output is the VMS debugger's, checked
  against the VMS 7.3 logs of `testdata/dbg` and `testdata/dbgcmd` by
  oracle tests; `TestDebuggerSessionOracle` replays every session and
  keeps the list of what still differs (`expectedDifferences`).
  `SET MODE` takes the VMS display modes and govax's access modes;
  the console and the debugger each have their own radix.
  `internal/console/consoletest` is its test support.
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
  1 is the console's, on VMINIT's tables and stacks; the console keeps
  each process's image state (`internal/console/images.go`). Phase 44's
  scheduler: the `System` owns a `sched.Scheduler` kept in step with the
  table and is the engine's hook (`schedule.go`: `Schedule`, `switchTo`,
  `SwitchCPU`; installed by the console's `newRTL` with
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
  `TerminalSource` input). CPU time
  and SHOW SYSTEM/SHOW PROCESS's reports are in `showsys.go`. Phase 47:
  `System.Locks` is the one lock database (`internal/lck`); `$ENQ`,
  `$ENQW`, `$DEQ` are `enq.go` (each request's `enqRequest` is its
  lock's notifier: LKSB, event flag, completion and blocking ASTs in the
  owner's process); image rundown dequeues user-mode locks, process
  deletion all (`locks.go`); the RMS context carries the database, the
  PID, and a record-lock wait (`awaitLock`, `lockWaker`). The
  console's engine hooks reach the current process (`Console.running`);
  `Console.RTL` stays process 1, and only process 1's image ending ends
  a console run (`Console.StepMachine`). A run that stops in another
  process leaves the CPU there until the next run or the debugger's EXIT
  (`Console.ReturnToProcessOne`).
- `internal/librtl` — LIBRTL.EXE's routines (Phase 34): the LIB$ and STR$
  shims a program reaches through `SHIM$LIBRTL_<offset>` stubs. `Routines`
  lists each with its transfer-vector offset (checked against
  `vmsdef.ImageSymbols`) and XFC$SHIM code; the console registers them into
  each RTL environment and builds the stubs from the same table. The process
  machinery they use (condition dispatch, the heap, memory) stays in
  `internal/corevms`, reached through `export.go`. Each further *RTL.EXE emulated
  gets a package like it. New routines are written from DIGITAL's manuals
  (clean room), and checked on VMS where a probe can: LIB$CREATE_DIR
  (`createdir.go`) against `testdata/credir/libcrd.mar`'s VMS 7.3 run
  (`TestLibCreateDirOracle`). LIB$PUT_OUTPUT is here too (`output.go`); kernel.asm's
  old interrupt-driven one is now its private `EXE$PUT_OUTPUT`.
  LIB$GET_FOREIGN (`foreign.go`) returns `Environment.CommandLine`: a
  foreign command's text (DCL symbols, `internal/console/dclsym.go`), or
  what follows the image on `govax run IMAGE text...`.
  LIB$GET_INPUT (`input.go`) reads a line from the terminal with the
  terminal's rules (`Environment.ReadInputLine`).
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
  `CREATE/DIRECTORY` (`internal/console/create.go`): ods2 lays the directory
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
- `internal/link` — the VAX linker (Phase 30): builds a VMS executable image from
  `internal/obj` modules, laid out as real LINK lays images out (byte for byte on
  the fixtures). The console's `LINK` command (`internal/console/link.go`) drives it.
  Undefined symbols come from symbol sources (`source.go`, `libsource.go`):
  IMAGELIB.OLB plus shareable images' GSTs, STARLET.OLB, then govax's own tables
  (`internal/console/linksource.go`). With traceback (the default), pass 2
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
  `LIBRARY` command (`internal/console/library.go`) drives it. Imports only
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
  `ANALYZE/OBJECT` and `ANALYZE/IMAGE` (`internal/console/analyze.go`,
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
  to (`Console.MountDefaultVolume`, `internal/console/defvolume.go`;
  defaults WORK, DUA0:, RD54, [WORK]; messages in `vmserrors/codes_mount.go`).
  Unset file: no default volume, names go to the host. Every key read must
  be in `cmd/govax/main.go`'s `validConfigs`, and documented in vax.help's
  `HELP CONFIG KEYS`.
  `vax.library` (string) — the host directory LINK and MACRO look
  in for IMAGELIB.OLB, STARLET.OLB, shareable images, and STARLET.MLB when
  `SYS$LIBRARY`/`SYS$SHARE` don't lead to them (read by the console, not `NewEngine`;
  `internal/console/syslib.go`); the older `vax.link.library` is still read when
  it isn't set. With no STARLET.MLB found, MACRO uses govax's own from bootdata:
  `files/starlet.mar` (system service and RMS macros, hand-written clean-room;
  the process and other service macros, in `_S`, argument-list, and `_G`
  forms, are checked against real MACRO's objects by `TestServiceMacroObjects`,
  from the probes in `testdata/mp/macros`, Phase 45; round 6 added the global
  section and lock service macros, Phase 46)
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
