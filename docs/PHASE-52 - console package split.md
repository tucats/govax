# Phase 52 — Splitting internal/console

**Status:** in progress (planned and approved 2026-10-12; the author's
answers to the review questions are at the end). Comes before Phase
51's remaining subtasks, which will then be taught the new structure.

## Where things stand

`internal/console` holds 81 source files (26,400 lines) and 138 test
files (33,100 lines). Two of the pieces named in the request are
already separate packages:

- **The DCL grammar parser** is `internal/console/dcl` (2,500 lines). It
  is a separate package, but it lives under `console` although the
  debugger uses it too.
- **The debugger** is `internal/debugger` (15,000 lines, Phase 42). It
  reaches the machine through `console/export.go` and about 30 other
  `console.` names (`Console`, `RunOptions`, `Activation`,
  `UnhandledException`, the symbol kinds, the stack and operand
  constants, `Help`, `Dispatcher`). The helpers it uses are still in
  `console`: `dbgcalls.go`, `dbgnames.go`, `dbgoperands.go`,
  `dbgtrace.go`, `source.go`, `exam.go`, `disasm.go`, and `chf.go`.

The real problem is not the number of files. It is that **one struct,
`Console` (`machine.go`), has about 75 fields from four different
subjects**, and the methods of all 81 files hang off it:

| Subject | Fields (examples) | Files |
|---|---|---|
| The machine | `Engine`, `CPU`, `Mem`, `Symbols`, `RTL`, `Regions`, `shimBase`, `s0Free`, `asmSession`, `proc1Images`, `otherImages`, `Trace`, `Radix` | init_vax, vminit, shim, services, rom, image, images, shared, run, execute, trace, asm, expr, symbols, set, show, exam, disasm, dbg*, chf, source, export |
| Operator session | `Devices`, `Logicals`, `Mounts`, `ContainerSession`, `Paths`, `HostLibrary`, `In`, `Out`, `ScreenSize` | the utility commands below |
| The DCL interpreter | `dclSymbols`, `levels`, `terminalFlow`, `status*`, `endStatus*`, `Verify`, `echo`, `control*`, `stopping`, `dclFiles`, `searchStreams`, `subCLI`, `clis`, `lending` | dcl*.go, procedure, subcli, spawncmd (22 files, 9,800 lines) |
| Front end | `Dispatcher`, `Debugger`, `quit`, `assemblerMode`, run limits | dispatch, commands, setcommand, debugger, misc, prompt |

A Go type's methods must all be in one package. So a file can't move to
another package while its functions are methods on `Console`. **Splitting
the struct is the main job. Moving files is the easy part that follows.**

### How tightly the groups are coupled (measured)

- **The utility commands are loosely coupled.** TYPE, DIRECTORY, COPY,
  DELETE, PURGE, RENAME, CREATE, SET/SHOW DEFAULT, MOUNT/DISMOUNT,
  INITIALIZE/CONTAINER, and the default volume use only
  `ContainerSession`, `Mounts`, `Devices`, `Logicals`, and `Printf`.
  MACRO, LINK, LIBRARY, and ANALYZE add only `Paths` and `HostLibrary`.
  `logical.go` and `createfile.go` are the exceptions: they reach into
  process 1 (`RTL`, image rundown, DCL's open files).
- **The DCL interpreter uses about 45 members it doesn't own.** Most are
  the process (`RTL` 33 uses, `Engine` 20, `CPU` 10), output and session
  state, and a handful of machine operations (run or stop an image,
  step, `ensureShims`, `activateCreatedImage`), plus `InDebugger` and
  `InAssemblerMode`. That is a manageable interface of about 25 methods.
- **The machine side uses the DCL side in only a few places:**
  XFC$CONSOLE_CMD (`services.go`), `$STATUS` after RUN (`run.go`'s
  `setCommandStatus`), Ctrl/Y (`controlYTaken`), and the subprocess hooks
  in `newRTL`. Each can be a hook (a function field) that the front end
  sets, so the machine package needn't import the shell.
- **Subprocess DCL is where this split helps most.** `subcli.go`'s
  `dclState` copies about 35 fields in and out of `Console`
  (`swapDCLState`) each time a subprocess's interpreter runs. With the
  DCL state in its own struct, each process has its own instance, and
  the swap shrinks to choosing which one is current (plus the process's
  `RTL`, `Logicals`, session, and `Out`).

### Tests

99 of the 138 test files (24,700 lines) are `package console` (internal);
39 are `console_test`. Most internal tests run commands through a
`Dispatcher` and only happen to be internal. About 36 files use
unexported state (`ensureShims`, `shimBase`, `imageLoad`, `dclSymbols`,
`storeLong`, `asmSession`, ...). The oracle tests (the Probe50 rounds,
FORTH, `TestInsn35Oracle`, `TestRMS3Oracle`, the credir and LIB$
oracles) need the whole system assembled, so they stay with the front
end as integration tests.

## Verdict

**Practical, if done in stages.** The utility commands and the shim and
XFC bridge can move almost at once. The machine/DCL split is real work,
but the coupling is narrow enough to describe with two interfaces, and
it makes the subprocess CLI simpler. Nothing here changes behavior. Each
stage ends with `go build ./...`, `go vet ./...`, golangci-lint, and the
whole test suite passing, and with the same set of test names as before
(compared from `go test -json` output), so no test gets lost in a move.

## Target layout

Layered from the bottom up. Each package imports only the ones below it.

| Package | Contents | Approx. lines |
|---|---|---|
| `internal/dcl` | The grammar engine: today's `internal/console/dcl`, moved, with no other change. The console and the debugger both use it. | 2,500 |
| `internal/help` | `help.go`'s `Help` (help-file topics). The console and the debugger both use it. | 160 |
| `internal/corevms` (additions) | The shim table and stub encoding; the process side of `cpu.SystemServices` (see "The shims and corevms"). | +250 |
| `internal/machine` | The emulated system as the operator sees it: INIT/ZERO, VMINIT and its S0 layout, ROM/NVRAM, shim placement, the image loader and the per-process image state, RUN/execute/trace, the console symbol table, the expression evaluator, ASM, EXAMINE/DEPOSIT support, disassembly, the debug-symbol helpers, CHF frames, and today's `export.go` (which becomes this package's public API). The debugger imports this package instead of `console`. | ~10,000 |
| `internal/dclcmd` | The utility commands, each as a function over a small `Context` (Out, the session, mounts, devices, logical names, paths, the pager): files, volumes, devices, logical-name commands, MACRO, LINK, LIBRARY, ANALYZE, the library search (`syslib.go`, `filenames.go`, `linksource.go`), and paging (`page.go`). It could be split again later (`dclcmd/files`, `dclcmd/devtools`) if it grows. | ~5,300 |
| `internal/dclshell` | The DCL interpreter: a `Shell` per process holding command levels and procedures, symbols, substitution, expressions, labels/IF/GOSUB/CALL, statuses and ON, verification, SYS$INPUT and DECK, DCL's file I/O, STOP and Ctrl/Y, WAIT, CPU lending, the subprocess CLI, SPAWN. It reaches the machine through a `Host` interface that the front end implements. The lexical functions (2,200 lines, already table driven) could become `dclshell/lexical` if they don't need much of `Shell`'s internals. | ~9,800 |
| `internal/console` | The front end: `Console` assembles a `machine.Machine`, a session, the shell, and the debugger hook; `Dispatcher` and the grammar bindings; routing between shell, debugger, and assembler mode; the XFC console I/O; run limits and the one-shot command line. Also the integration and oracle tests, and `consoletest`. | ~2,000 |

`show.go` (1,700 lines, 41 methods) and `set.go` are split by subject:
registers, memory, TB, SCB, stack, clock, and faults go to `machine`;
DEFAULT, LOGICAL, and DEVICE go to `dclcmd`; SYMBOL goes to whichever of
the two symbol tables it shows.

The names are suggestions (see the questions at the end). `dclshell`
avoids `cli`, which `cmd/govax` already uses for gopackages' `app-cli/cli`.

## The shims and corevms

Part of the shim support belongs in corevms, and part does not:

- **Move to corevms:** the shim *table* (`shim.go`'s `baseShims`: name,
  library, offset, code). These rows describe routines corevms
  implements, and `internal/librtl` already keeps its own rows next to
  its routines (`librtl.Routines`). In the table-driven style, each
  registration in `corevms.ShimTable` would carry its name, library, and
  offset, and the full list would come from the registry, with no
  separate list to keep in step. The 12-byte stub encoding
  (`ShimStub(code) []byte`) would move too.
- **Move to corevms:** the process half of `services.go`: `running`,
  `SystemService`, `NextAST`, `DispatchException`, `Shim`,
  `translateHalt`, and the process-switching helpers (`ReturnToProcessOne`,
  `ShowProcessContext`, `ProcessPC`). They only delegate to the running
  `Environment`. They live in the console because corevms once avoided
  importing `internal/cpu`, and corevms has imported it since Phase 44.
  The comment that gives that reason is out of date.
- **Keep with the machine:** placing the stubs (VMINIT's reserved page,
  the overflow page) and defining the `SHIM$lib_offset` symbols, which
  belong to the operator's symbol table, not to VMS. XFC$CONSOLE_WRITE,
  XFC$CONSOLE_READ, XFC$CONSOLE_CMD, and XFC$QUIT stay in the front end,
  as a small type that embeds corevms's half to implement
  `cpu.SystemServices`.
- **Not yet:** VMINIT itself (building the page tables and the S0 layout)
  could also be called system bootstrap and so belong in corevms. But it
  defines console symbols (`CONSOLE$SCRATCH`, the string pool) and
  resets the assembler session, so it goes to `machine` for now. Moving
  it on to corevms can be decided later.

## Stages

Each stage is one or more commits. Each commit builds and passes the
whole test suite. I commit at the end of each stage and run `build -i`,
and you push.

**Stage 0 — Groundwork.** Decide how this fits with Phase 51. Record the
baseline: the list of test names (`go test -json ./... `), and the lint
state. Run `deadcode` to find and remove code nothing reaches, so dead
code isn't moved. Install the tools (below).

**Stage 1 — The easy moves.** `internal/console/dcl` becomes
`internal/dcl`, which only rewrites the import path in about 15 files.
`help.go` becomes `internal/help`. Leaf files that don't use `Console`
move to the packages they'll end up in once those exist: `dclexpr.go`'s
evaluator core, `filenames.go`, `page.go`. Low risk. It is also a trial
of the tools.

**Stage 2 — Shims and the services bridge into corevms**, as described
above. Small and self-contained. It sets the pattern of hooks in place
of imports.

**Stage 3 — Split the `Console` struct, inside the package.** This is
the main stage. Add `Machine`, `Session`, and `Shell` structs to
`internal/console`, and move the fields into them. `Console` embeds
pointers to the three, so `c.Field` keeps compiling through promotion
and call sites can change gradually. Then move methods, file by file,
from `*Console` to the struct that owns their state. Where a method
needs another component, that need goes on the component's interface
(`Shell`'s `Host`, the machine's hooks). The package stays put, so
everything compiles at every step. The result is the exact list of what
crosses each boundary. Subprocess DCL's `dclState` becomes "the current
`*Shell`" here.

**Stage 4 — Move the utility commands to `internal/dclcmd`.** Each
command becomes a function over a `dclcmd.Context`. `dclcmd` exports
its bindings as a table (syntax name → handler), and the front end
binds them into the grammar, as `librtl` exports its routines. Their
unit tests move with them.

**Stage 5 — Move the machine to `internal/machine`.** This includes the
debugger's helpers (dbg*, source, exam, disasm, chf). The debugger
switches its import from `console` to `machine`, and `export.go`
becomes `machine`'s ordinary API. The machine's unit tests move with it,
and tests that poke at unexported state either move into `machine` or
get a small exported test hook.

**Stage 6 — Move the shell to `internal/dclshell`.** The front end's
type implements `Host`. The shell's unit tests (expressions,
substitution, lexicals, symbols) move with it. The procedure oracles
stay in `console`.

**Stage 7 — Finish.** `console` is now only the front end. Make
integration tests `console_test` where only the exported API is used.
Update CLAUDE.md's architecture section and PLAN.md's package list, and
add an old-path → new-path table here. Earlier phase docs are history,
so they keep their old file names; the table is the key to them.

Later, and separately: whether the debugger-only helpers (SHOW CALLS,
source lines, EXAMINE/INSTRUCTION operands) should move from `machine`
into `internal/debugger`, and whether VMINIT moves to corevms.

## Risks

- **Import cycles.** The machine runs XFC$CONSOLE_CMD into the shell,
  and the shell runs images on the machine. The fix is the layering
  above: the lower package defines a hook or interface and the front end
  connects the two. Stage 3 finds every such crossing before any package
  boundary exists.
- **The subprocess CLI's goroutine handoff** (`cliYield`, `swapDCLState`)
  is the most delicate code involved. Stage 3 changes how its state is
  held without changing the handoff. The Phase 48/50 subprocess tests
  and the probe oracles are the safety net.
- **Phase 51 conflicts.** Its remaining subtasks (Ctrl/Y in a
  subprocess's procedures, the prompt reading on a goroutine) touch
  `subcli.go`, `dclcontroly.go`, and `dcllend.go`, which are exactly the
  files Stage 3 rewrites.
- **Blame and history.** Move files whole where possible (`git mv`, with
  the content mostly unchanged in the same commit), so `git log
  --follow` and blame still work.
- **Doc churn.** CLAUDE.md names many `internal/console/*.go` files, and
  Stage 7 updates them all at once.

## Tools

Installed already and useful here: `gopls` (`gopls rename` for safe
renames across the module), `goimports`, `golangci-lint`, `staticcheck`,
`go-callvis`.

Worth installing:

- **`rsc.io/rf`** (`go install rsc.io/rf@latest`): Russ Cox's refactoring
  tool, used for the Go compiler's own restructuring. Its `mv` moves
  declarations (functions, and types with all their methods) to another
  package and fixes every reference. It can't put one type's methods in
  two packages, since Go forbids that, which is why Stage 3 comes first.
  It is labeled experimental, so I'd try it in Stage 1 and fall back to
  scripted `git mv` plus `gopls` if it misbehaves.
- **`golang.org/x/tools/cmd/deadcode`** (`go install
  golang.org/x/tools/cmd/deadcode@latest`): reports functions that
  nothing can reach from `main` or the tests, for Stage 0.
- **Optional: `github.com/loov/goda`**: package dependency graphs and
  "why does A import B", handy for checking the layering after each
  stage. `go list` can do the same with more typing.

`gomvpkg` isn't worth installing. It predates modules, and Stage 1's
package move is a plain import-path rewrite.

## Questions for review

1. **Order with Phase 51.** Should this come before Phase 51's remaining
   subtasks or after them? Before is cleaner: Ctrl/Y in a subprocess is
   easier with a `Shell` per process. After avoids pausing work in
   progress. **ANSWER** Lets to the refactoring now, even though it will
   entail teaching previous incomplete phases the new structure.
2. **Package names:** `internal/dcl`, `internal/help`,
   `internal/machine`, `internal/dclcmd`, `internal/dclshell`. Any you'd
   rather call something else? **ANSWER** These names are fine.
3. **Phase number.** Is this Phase 52 (with an entry in PLAN.md's
   index), or something outside the phase sequence? **ANSWER** Let's
   do this as PHASE-52 and reflect in PLAN.md.
4. **Tests.** Is it acceptable to add a few small exported test hooks
   where a moved test pokes at unexported state, and to convert
   integration tests to `console_test`? **ANSWER** Yes.
5. **One `dclcmd` package or two** (files/volumes/devices/logical names
   vs. MACRO/LINK/LIBRARY/ANALYZE)? **ANSWER** One dclcmd package is ok.

## Progress log

- **2026-10-12 — Stage 0.** Baseline: 4,610 tests (4,595 pass, 15
  skip) and 3 golangci-lint findings, none in `internal/console`. Each
  stage's gate is `go build`, `go vet`, the whole suite with `-count=1`
  (its multiset of test names and results compared with the
  baseline's, so that no test is lost in a move), and golangci-lint
  compared with the baseline. `deadcode -test` finds nothing to prune in
  `internal/console`: its only unreachable-from-main functions
  (`imageLoad`, `loadLong`, `LoadHelpFile`, ...) are test helpers.
  `rf` refuses a move whose result doesn't compile and then changes
  nothing, which makes it safe to try; it can't move a declaration
  that a test in the old package reaches unexported.
- **2026-10-12 — Stage 1.** `internal/console/dcl` is `internal/dcl`
  (import paths and comments only; its tests find `bootdata` one
  directory up less). `help.go` is `internal/help`: `help.File`
  (`Parse`, `Load`, and `Show`, which writes a topic to an
  `io.Writer`), with `Console.Help` a two-line wrapper the HELP command
  and the tests call; `helpKey`'s test moved with it. The leaf files the
  plan named for this stage (`filenames.go`, `page.go`, `dclexpr.go`)
  move with the packages they belong to, in Stages 4 and 6, rather than
  ahead of them.
- **2026-10-12 — Stage 2.** The shim rows turned out to be *link*
  data, not registration data: `DECC$OPEN` at DECC$SHR+4E8 is shim 4,
  which corevms registers as `EXE$OPEN`. So rather than adding names and
  offsets to `ShimTable.Register`, corevms has `ShimLink` and
  `CRTLLinks` (`shimlink.go`), beside the registrations, and
  `librtl.Links()` gives LIBRTL's rows from `librtl.Routines`. The stub
  is `corevms.ShimStub(code)` (`ShimStubSize`). What stays in the
  console is the microkernel's code-0 rows (`kernelLinks`, which name
  kernel.asm's labels), placing the stubs, and the `SHIM$` symbols.
  The process half of `cpu.SystemServices` is `corevms.EngineHooks`
  (`enginehooks.go`): `Running`, `SystemService`, `Shim`, `NextAST`,
  `DispatchException`, and `EngineError` (was `translateHalt`; its test
  is now corevms's `TestEngineError`). `Console` embeds it, so it still
  satisfies `cpu.SystemServices`, `ASTSource`, and
  `ExceptionDispatcher`. The process-switching helpers
  (`ReturnToProcessOne`, `ShowProcessContext`, `ProcessPC`,
  `ProcessNote`) use the engine and the debugger's view of process 1,
  so they stay with the machine and move in Stage 5.
- **2026-10-12 — Stage 3, step 1: `Shell`.** The per-process DCL state
  that `subcli.go`'s `dclState` listed (the process, its logical names,
  session, and output; symbols; command levels; verification; $STATUS
  and the end status; Ctrl/Y and STOP; IF blocks at the terminal; DCL's
  open files; F$SEARCH's streams; the last image's status) is now a
  `Shell` struct (`shell.go`). `Console` embeds a `*Shell`, the DCL
  whose commands run now, so every `c.RTL`, `c.Out`, `c.levels` still
  compiles. `Console.home` is process 1's, and each subprocess CLI has
  its own `shell`. `swapDCLState`'s 35 field swaps became `useShell`,
  one pointer swap that returns its restore. `consoleOutput` (the
  terminal) is `c.home.Out`, and `procedureInput` keeps the `Shell`
  its procedure belongs to.
  **Bug fixed:** while a subprocess's DCL ran, its swapped-out
  `cli.state` held *process 1's* state, so a SPAWN from a subprocess
  (Phase 51, subtask 2) gave the new process a copy of process 1's
  symbols instead of its parent's (`InheritSymbols` runs synchronously
  in `env.Spawn`, in the parent's turn). With a `Shell` per process
  each CLI's state is always its own. `TestSpawnDCL_nestedInherits`
  (failed before, passes now).
- **2026-10-12 — Stage 3, step 2: `Machine`.** The machine's state is a
  `Machine` struct (`vaxmachine.go`), embedded in `Console` beside the
  `Shell`. The methods of the machine's files (INIT/ZERO, VMINIT, ROM,
  shims, images, RUN, execute and trace, ASM, EXAMINE support,
  disassembly, the debug-symbol helpers, CHF, source lines, the
  debugger's export surface, SET, and most of SHOW) now have `*Machine`
  receivers, still named `c` until the package move. Changing the
  receivers first and reading the compiler's errors gave the exact
  boundary: 111 errors. Most were `c.RTL` meaning process 1, and the
  machine now says `processOne`. What the machine needs from the DCL
  running a command is `Machine.Hooks`, a struct of functions the
  console fills in: `Output`, `Session`, `ProcessOneOnly`,
  `CommandStatus` (an image's status as `$STATUS`), `ControlYTaken`,
  `CLIImageEnded`, `NewProcessOne`, and `ProcessDeleted`. `newRTL`
  splits in two: the System's part is the machine's, and process 1's
  terminal streams and the CLI's registration are the console's
  `newProcessOne`. `EngineHooks`, `Debugger`, the engine's `Services`
  (the console), and process 1's image-run fields (`imageActive`,
  `imageStatus`, `imageEnded`, `runHost`, which only process 1 sets)
  moved to `Machine`. So did process 1's logical-name database
  (`processOneLogicals`, which outlives the machine), because image
  rundown deletes its user-mode names whether or not there is a
  machine (`TestImageRundown`). `Console.Run` sends a subprocess's RUN
  to `subprocessRun` and process 1's to `Machine.Run`.
  `Console.runDownAbandonedImage` does nothing in a subprocess's DCL,
  where `imageActive` was always false before. What stays on `Console`:
  the XFC console selectors (`ConsoleWrite`, `ConsoleReadByte`,
  `ConsoleCommand`, `RequestQuit`), `StopProcess` and `ShowRegions`
  (which act on the DCL's own process), and everything DCL. Two
  readings of `c.RTL` that meant process 1 but would have found a
  subprocess's process during its DCL now say process 1:
  `imageProcess.process()` for process 1's images, and
  `EngineHooks.Home`. The second matters only when the System has no
  current process.
- **2026-10-12 — Stage 3, step 3: `Interpreter`.** `Console` embeds an
  `Interpreter` (`interpreter.go`): every process's DCL, holding process
  1's `Shell` (`home`), the subprocess CLIs, SET CONTROL, CPU lending,
  SPAWN's page, the terminal input (`In`), and the current `Shell`,
  embedded. The DCL files' methods (procedures, symbols, expressions,
  flow, statuses, verification, SYS$INPUT, file I/O, Ctrl/Y, WAIT,
  lending, the lexical functions, the subprocess CLI, SPAWN) have
  `*Interpreter` receivers. Changing them gave 85 errors, nearly all the
  machine itself (the engine for lending and attention keys, the CPU
  for the CLI's image calls, LIB$SPAWN's call). Since `dclshell` will
  sit above `machine`, the interpreter holds it directly
  (`Interpreter.machine`) rather than through an interface, so the plan's
  `Host` interface shrinks to one hook: `dispatchConsole`, the
  `Dispatcher` that runs a subprocess CLI's command lines.
  `InDebugger` moved to `Machine`, which owns the debugger.
  `Running`/`Quit`, the DCL prompt, and `readReply` moved to
  `Interpreter`. Each layer has its own `Printf`: the machine's goes
  through `Hooks.Output`, and the interpreter's and the console's go
  to the DCL's `Out`. The machine's abandoned-image rundown is
  `endAbandonedImage`; only the interpreter's wrapper is called
  `runDownAbandonedImage`, so nothing hides behind a promoted name of
  the same spelling. The command handlers in the DCL files that are
  `*Dispatcher` methods (GOTO, IF, CALL, OPEN, ...) and
  `DispatchConsole` itself stay on `Dispatcher` until Stage 6.
- **2026-10-12 — Stage 4: `internal/dclcmd`.** The utility commands
  first became methods of an in-package context, built for each command
  from the DCL running it (`Console.cmd`; tests and cmd/govax use
  `Console.Commands`). What they needed of DCL turned out to be four
  hooks on it: `Running`, `SetStatus` (DEASSIGN's and SHOW LOGICAL's
  `$STATUS`), `ReadReply` (paging), and `TypeDCL` (TYPE of SYS$INPUT,
  or of a file DCL's OPEN opened). The shim rows LINK takes as symbols
  come in as `ShimLinks`. The helpers that dclcmd, the DCL interpreter,
  and the machine all use went below all three: RMS's naming rules
  are `rms.FileLocation.WithDefaultType`, `rms.MatchCase`,
  `rms.SplitVersion`, and `rms.NameType` (`TestMatchCase` moved with
  them), and the failure carrying a VMS condition value (`statusError`)
  is `vmserrors.ConditionError`. `rf` then exported, in place, what the
  console and DCL use of the commands (`Context`, `ConditionLine`,
  `FileFailure`, the settings' names, `GovaxSymbols`, ...). It did 13
  renames in one script, tests included, in two seconds. Then the 23
  files and `context.go` moved with `git mv`, which keeps their history,
  and the compiler's "undefined" errors placed the `dclcmd.`
  qualifiers. The ANALYZE binding's loop over the record-type table
  became `dclcmd.AnalyzeRecordTypes`. dclcmd imports only lower
  packages (anl, asm, bootdata, corevms, io, lbr, link, lnm, obj,
  respath, rms, vmsdef, vmserrors, vmsimage). The commands' tests stay
  in `internal/console`, where they run the commands through a console;
  they reach the few unexported names they used through exports
  (`PagePrompt`, `LibrarySetting`, `TerminalSize`, `LinkSources`,
  `HostLibraryDir`, `JoinLines`).
