# Phase 42 — The debugger: its own package, grammar, and prompt

**Status:** in progress. Planned and reviewed 2026-10-05 (the author
took every recommended decision). Subtask 1's probe is ready for VMS.

## Goal

The govax console has two jobs, as eVAX's did. It is a VMS command line
(SET DEFAULT, MOUNT, MACRO, LINK, RUN, ...), and it is a machine
debugger (EXAMINE, DEPOSIT, STEP, SET BREAK, SHOW REGISTERS, ...). Both
share one prompt and one grammar (`console.dcl`). This phase separates
them:

- **A debugger package** (`internal/debugger`) with its own DCL grammar
  (`debug.dcl`), help file, and `DBG>` prompt, modeled on the VMS
  debugger. Where it implements a VMS debugger command, the syntax and
  output match the VMS debugger's.
- **Debugger mode** starts with:
  - the console's `GO` and `CALL` commands, which run code by address;
  - `RUN` of an image linked `/DEBUG`, or `RUN/DEBUG` of any image.
- **No VMS command line in the debugger.** Commands that emulate DCL
  (DIRECTORY, MACRO, SET DEFAULT, ...) aren't available in the
  debugger. `EXIT` leaves the debugger and returns to the console.
- **SET and SHOW split.** Commands that show or change CPU or kernel
  state (SHOW CPU, SHOW REGISTERS, SHOW PTE, SHOW SCB, SET PSL, SET
  MODE, ...) become debugger commands. Commands that set defaults,
  environment flags, symbols, and logical names stay console commands.
- **`DISASSEMBLE` becomes `EXAMINE/INSTRUCTION`** in the debugger.

This phase doesn't implement every VMS debugger command. Each command
it does implement (breakpoints, stepping, examining, call frames)
behaves as the VMS debugger's does, so a VMS user's habits carry over,
and so a session can be checked against a real VMS debugger session.

**Bugs found in the current debugger-related commands are in scope.**
Each is fixed in this phase, with a regression test, and logged in the
progress log. The bugs known at planning time are listed below; more
will turn up as the code moves.

## What earlier phases leave in place

- **One grammar, one dispatcher** (`internal/bootdata/files/console.dcl`,
  `internal/console/dispatch.go`, `commands.go`, `setcommand.go`).
  Phase 37 moved every command onto the grammar. `Dispatcher.Dispatch`
  handles assembler mode, DCL symbols, and grammar dispatch. `SHOW`,
  `SET`, and `CLEAR` each have one keyword type that mixes console and
  machine-debugging keywords.
- **Run control in `Console`** (`execute.go`, `call.go`, `step.go`,
  `trace.go`, `instbreak.go`, `faultbreak.go`). `runLoop` drives
  `Engine.Step` and checks address and instruction breakpoints between
  steps. Fault breakpoints live on `cpu.Engine`. `reportStopReason`
  turns a stop (HALT, Ctrl-C, limits, console-call return, CHF fault)
  into a message.
- **Phase 41's symbolic pieces**: `internal/dbgsym` (an image's DST,
  DMT, and GST, with lookups both ways), `internal/disasm`'s debugger
  style, and the console's use of them (`dbgnames.go`, `dbgtrace.go`,
  `dbgcalls.go`). `DISASSEMBLE` already prints the VMS debugger's
  `EXAMINE/INSTRUCTION` lines. STEP, the trace, and `SHOW CALLS` name
  locations as the debugger does. Phase 41 left the debugger itself
  (`RUN/DEBUG`, `DBG>`, breakpoints by routine, unhandled-exception
  breaks, source lines) to this phase.
- **The probe** (`testdata/dbg/`): nine VMS 7.3 debugger sessions
  (`vax/*.dlg`) over images built `/DEBUG`, with traceback only, and
  `/NOTRACEBACK`. They show EXAMINE, EVALUATE, SYMBOLIZE, STEP by
  instruction and by line, SET BREAK, GO, SHOW CALLS, source display,
  the unhandled-exception break, and the image-exit message. Phase 41
  used only their `EXAMINE/INSTRUCTION` lines.
- **The REPL** (`cmd/govax/main.go`): readline with a `VAX> ` prompt,
  or `ASM> ` in assembler mode. Ctrl-C (`attention.go`) stops a running
  program through `cpu.Engine.Attention`.
- **The boot script** (`internal/bootdata/files/vax.init`) uses
  commands this phase moves: `go exe$initialize` (which ends in a HALT),
  `set PC=200`, `set quantum 1`, and `set debug ...`.

## Known bugs (to fix in this phase)

Found while planning. Items 1 and 2 were confirmed with a throwaway test
against `dbgdis.exe`.

1. **`RUN` and `CALL` ignore breakpoints.** `Console.Call`'s
   run-to-completion loop (`call.go`) never checks breakpoints, and
   `RUN` without `/STEP` goes through it. `SET BREAK 55A` followed by
   `RUN DBGDIS.EXE` runs to `DBGDIS: done`. Breakpoints fire only after
   `RUN/STEP` and then `GO`.
2. **STEP/OVER and STEP/RETURN leak their one-shot breakpoint.** If the
   run stops somewhere else first (a user breakpoint, a fault
   breakpoint, Ctrl-C, a HALT), the temporary step breakpoint stays in
   `Console.Breakpoints`. A later `GO` then stops there with a stray
   "Stepped to". (Confirmed: after `STEP/OVER` stopped at a breakpoint
   in SUB2, a temporary step breakpoint at 4FD was still set.)
3. **A step breakpoint at an address that already has a user breakpoint
   is never removed.** `breakpointAt` returns the user breakpoint
   first, so the temporary one is never hit. `runLoop`'s comment
   accepts this, but it's the same leak as item 2.
4. **STEP/OVER isn't tied to a call frame.** Its return breakpoint
   fires at the address however it's reached. A recursive or re-entered
   routine stops in the wrong frame. The VMS debugger's STEP/OVER
   completes in the frame it started from. (Found by reading the code.
   Subtask 2 confirms it with a test.)
5. **Grammar entries with no handler.** `CLEAR ERROR`, `CLEAR PROFILES`,
   `SHOW ASSEMBLER_FLAGS`, `SHOW COMMAND_ARGS`, `SHOW ERROR`,
   `SHOW SYMBOL/TEMPORARY`, `SHOW SYMBOL/UNRESOLVED`, and
   `SHOW WATCHPOINTS` parse, then fail with "no handler bound". Each is
   either implemented or removed in this phase.
6. **Instruction breakpoints report a bare address**
   ("Instruction break at %08X"), even in an image whose locations
   are otherwise shown symbolically.
7. **Expressions can't use registers.** The evaluator doesn't know
   register names (`expr.go`'s doc comment), so `EXAMINE R1+4` and
   `SET BREAK @SP` don't work. Only a bare register name works, as a
   special case in EXAMINE and DEPOSIT. The VMS debugger accepts
   registers anywhere in an address expression.

## How the VMS debugger behaves (what this phase matches)

From the *VMS 5.5 Debugger Manual* (`~/Documents/Technical
Doc/VMS/AA-LA59D-TE_VMS_5.5_Debugger_Manual_199111.pdf`, the command
dictionary, part III) and the Phase 41 probe's sessions:

- **Starting.** `RUN` of an image linked `/DEBUG` starts the debugger;
  `RUN/NODEBUG` doesn't, and `RUN/DEBUG` starts it for any image with a
  DST. A `/NOTRACEBACK` image runs without it, silently (probe:
  `dbg.log`). The debugger prints its banner and
  `%DEBUG-I-INITIAL, Language: MACRO, Module: DBGDIS`, then prompts
  with the PC at the main routine's first instruction, after the entry
  mask (`EXAMINE/INSTRUCTION .PC` shows `DBGDIS\START\%LINE 42`).
- **Ending.** `EXIT` (or Ctrl/Z) ends the session, running exit
  handlers. `QUIT` ends it without running them. When the program
  exits, the debugger prints `%DEBUG-I-EXITSTATUS, is '%SYSTEM-S-NORMAL,
  normal successful completion'` and keeps prompting.
- **Ctrl/C** interrupts the running program and returns to `DBG>`.
  Ctrl/Y followed by the DCL command `DEBUG` starts the debugger on an
  image that is running without it.
- **Messages.** `break at DBGDIS\LOCALR\JSBRTN`; `break at routine
  DBGSUB\SUB2` for a break set on a routine name, which takes effect
  after the entry mask; `stepped to DBGDIS\START\%LINE 43: MOVL ...`
  (step by instruction) or `stepped to DBGDIS\START\%LINE 48` (step by
  line). Each is followed by the source line, unless that is turned off
  with `SET STEP NOSOURCE`. An unhandled condition prints its message,
  then `break on unhandled exception at FAILSUB\SUB2\%LINE 12`. All
  lowercase, unlike govax's current `Break at` and `Stepped to`.
- **Commands this phase covers** (manual §2.4): `GO`, `STEP`,
  `SET/SHOW STEP`, `SET/SHOW/CANCEL BREAK`, `SET/SHOW/CANCEL TRACE`,
  `SET/SHOW/CANCEL WATCH`, `SHOW CALLS`, `SHOW STACK`, `CALL`,
  `EXAMINE`, `DEPOSIT`, `EVALUATE[/ADDRESS]`, `SYMBOLIZE`,
  `SET/SHOW/CANCEL RADIX`, `SET/SHOW/CANCEL MODE`, `SHOW IMAGE`,
  `SHOW MODULE`, `SHOW SYMBOL`, `EXIT`, `QUIT`, `HELP`, `@file`.
- **SET BREAK qualifiers** this phase implements: `/AFTER:n`,
  `/TEMPORARY`, `/INSTRUCTION[=(opcode,...)]`, `/CALL`, `/BRANCH`,
  `/RETURN`, `/LINE`, `/EXCEPTION`, plus `WHEN (expr)` and
  `DO (cmd;...)`. Others (`/ACTIVATING`, `/EVENT`, `/VECTOR_INSTRUCTION`,
  `/[NO]SHARE`, `/[NO]SYSTEM`, `/SILENT`, `/[NO]SOURCE`, `/MODIFY`) are
  refused, or left to Future features.
- **STEP**: units `/INSTRUCTION` and `/LINE` (the default where there
  is line information); `/INTO` and `/OVER` (the default); `/RETURN`;
  `/[NO]SOURCE`; `/SILENT`.

Where the manual and the probe disagree, the probe (VMS 7.3) wins. Where
neither settles a format, govax chooses one and logs it as unconfirmed
here, for the probe in subtask 1 or a later simh round.

## Design

### Packages and dependency direction

```
cmd/govax            wires console and debugger together; the REPL picks
                     the prompt (VAX>, ASM>, DBG>) from the active mode.
internal/debugger    (new) the debugger: session state (breakpoints,
                     watchpoints, tracepoints, step defaults, radix,
                     modes, current location), run control (the run
                     loop, STEP), its Dispatcher over debug.dcl, and its
                     command handlers. Imports console, cpu, disasm,
                     dbgsym, symtab.
internal/console     the VMS command line, image activation, CHF, the
                     machine's lifetime (INIT, VMINIT, BOOT). Knows the
                     debugger only through a small interface.
```

`internal/debugger` imports `internal/console`, not the other way round
(Decision 1). `Console` gets a field typed by a small interface:

```go
// Debugger is what the console asks of the debugger (internal/debugger
// implements it; cmd/govax installs it).
type Debugger interface {
    // Start runs code under the debugger: GO, CALL, or an image's main
    // routine (RUN). It returns when the run ends, or when the debugger
    // has stopped it and is waiting for DBG> commands.
    Start(a Activation) error
    // Active reports a debugger session in progress (the DBG> prompt).
    Active() bool
    // Dispatch runs one debugger command line.
    Dispatch(line string) error
}
```

`Activation` says what is starting (GO from an address, CALL of a
routine with arguments, RUN of an image with or without `/DEBUG`), and
whether to stop before the first instruction.

The console keeps what is VMS or the machine, not the debugger: image
activation and rundown, the IMAGE$INIT driver, `Engine.CallEntry`, the
CHF and `handleConsoleFault`, and the machine's lifetime. It exports
what the debugger needs (the engine, memory, symbol tables, loaded
images and their `dbgsym.Program`s, the expression evaluator, and the
classification of a stop). Like `internal/coreos`'s `export.go`, these
go in one file, so the surface stays visible.

Run control moves to the debugger: `runLoop`, the breakpoint lists,
STEP, the trace, instruction and fault breakpoints, and the stop
messages. The console's `GO`, `CALL`, and `RUN` hand off to
`Debugger.Start`. Without a debugger installed (a bare `Console` in a
console test), they run to completion, as `Call` does now but with no
breakpoints.

### Dispatch and modes

The front end routes each line by mode:

1. Interactive assembler mode (`ASM`) is unchanged and takes priority.
2. With a debugger session active, the line goes to
   `Debugger.Dispatch` (the `debug.dcl` grammar).
3. Otherwise it goes to the console grammar.

Routing by mode lets a command file mix the two: `@file` or a script
fed to stdin can `GO`, and the lines after a breakpoint are debugger
commands until `EXIT`. The VMS debugger likewise reads `DBG$INPUT` and
switches back to DCL on exit. `XFC$CONSOLE_CMD` (a VAX program running
a console command) always uses the console grammar.

A console command typed at `DBG>` gets the VMS debugger's message for an
unknown verb, plus a hint (`EXIT to return to the console`). HELP at
`DBG>` reads `debug.help`.

### The session's lifetime (Decision 2)

- **GO or CALL at the console** starts a session and runs. If the run
  finishes without stopping (a HALT for GO, the routine's return for
  CALL), the session ends quietly and the console prompt returns, as
  now. `vax.init`'s `go exe$initialize` keeps working. If it stops (a
  breakpoint, STEP, a watchpoint, a fault breakpoint, an unhandled
  exception, Ctrl-C), the `DBG>` prompt appears. `GO/STEP` and
  `CALL/STEP` stop before the first instruction.
- **RUN** of an image linked `/DEBUG` (`IHD$V_LNKDEBUG`), or `RUN/DEBUG`
  of an image with a DST, starts the session stopped at the main
  routine's first instruction, as VMS does. `RUN/NODEBUG` runs without
  stopping. Breakpoints set earlier still apply (bug 1). `RUN/STEP`
  remains a govax synonym for `/DEBUG`.
- **Once `DBG>` has appeared, the session lasts until `EXIT` or `QUIT`.**
  If the program ends meanwhile, the debugger reports it
  (`%DEBUG-I-EXITSTATUS` for an image; a HALT or the CALL's return for
  the others) and keeps prompting, as VMS's does. `EXIT` runs the image
  down (user-mode logical names, as an image exit) and returns to the
  console. The machine's state is left as it is.
- **The console's `DEBUG` command** (new) starts a session on the
  machine as it stands, with nothing running. It is the counterpart of
  VMS's Ctrl/Y followed by `DEBUG`. It is how to EXAMINE memory or SHOW
  REGISTERS after boot, now that those are debugger commands.

### The command split

Each current command, where it goes, and its VMS debugger counterpart.
"Console" commands keep their current syntax. "Debugger" commands take
the VMS syntax where VMS has the command, and keep govax's otherwise
(marked *govax*).

**Verbs other than SET, SHOW, and CLEAR**

| Today | Goes to | In the debugger |
|---|---|---|
| EXAMINE, EX, DUMP | debugger | `EXAMINE` (VMS form: `a[:b][,...]`, type qualifiers; `/PTE` *govax*) |
| DEPOSIT, D | debugger | `DEPOSIT a = v` |
| DISASSEMBLE, DIS | debugger | `EXAMINE/INSTRUCTION` (`/CONSTANTS`, `/SHAREABLE` *govax*) (Decision 7) |
| STEP, S, ST | debugger | `STEP` |
| EXECUTE, GO, G | both | console: starts a session (above); debugger: `GO [address]` |
| CALL | both | console: starts a session; debugger: `CALL routine [(args)]` |
| RUN, R | console | (starts a session with `/DEBUG` or a `/DEBUG` image) |
| EXIT, QUIT | both | console: leave govax; debugger: end the session |
| HELP, ? | both | each reads its own help file |
| INCLUDE, @ | both | console: `INCLUDE`; debugger: `@file` of debugger commands |
| PRINT, ECHO, IF, TIME | console | (`IF`/`FOR`/`WHILE`: Future features) |
| ASM, SAVE, LOAD, ZERO, BOOT, ROM, INITIALIZE, VMINIT | console | |
| ABOUT, TEST | console | |
| DCL-emulation verbs (DIRECTORY, MACRO, LINK, ...) | console | |
| (new) DEBUG | console | starts a session with nothing running |

**SHOW**

| Keyword | Goes to | In the debugger |
|---|---|---|
| DEFAULT, LOGICAL, TRANSLATION, DEVICES, VERSION, XTEST | console | |
| SYMBOLS (console table), DCL symbols | console | debugger `SHOW SYMBOL` is VMS's (debug symbols, then the console table) |
| DEBUG, QUANTUM, INSTRUCTIONS, SHARE_PREFIX, EXPAND | console | |
| ROM, NVRAM, STRING_POOL | console | |
| COMMAND_ARGS, ERROR, ASSEMBLER_FLAGS | console | implemented or removed (bug 5) |
| REGISTERS, REG, R0…PC, privileged registers | debugger | *govax* `SHOW REGISTERS`, `SHOW <register>` |
| PSL, CPU_STATUS, CLOCK | debugger | *govax* |
| BASE | debugger | the current location (`.`); *govax* `SHOW BASE` |
| MEMORY, VM (with /STATISTICS etc.), MAPS, TB, REGIONS | debugger | *govax* |
| PAGE, PTE, SCB, SHIM | debugger | *govax* |
| STACK, ISP, KSP, ESP, SSP, USP | debugger | `SHOW STACK` is VMS's (Decision 6); the per-stack dumps stay *govax* |
| CALL_FRAMES, CALLS | debugger | `SHOW CALLS [n]` (VMS); frame detail is `SHOW STACK` |
| EXCEPTIONS, FAULTS | debugger | *govax* |
| BREAKPOINTS | debugger | `SHOW BREAK` |
| WATCHPOINTS | debugger | `SHOW WATCH` (bug 5) |
| TRACE, DISASSEMBLY | debugger | `SHOW TRACE` |
| STEP_MODE | debugger | `SHOW STEP` |
| MODE | debugger | `SHOW MODE`: VMS's modes, then the access mode (Decision 4) |
| RADIX | both | each has its own radix (Decision 8); the debugger's prints VMS's two lines |
| IMAGES | debugger | `SHOW IMAGE` (VMS's table; `/FULL` *govax*) |
| (new) | debugger | `SHOW MODULE`, `SHOW SCOPE` (current scope only), `SHOW LANGUAGE` |

**SET**

| Keyword | Goes to | In the debugger |
|---|---|---|
| DEFAULT, DEBUG, QUANTUM, UIQUANTUM, VERBOSE, VERIFY | console | |
| `name=value` for a console symbol | console | |
| `name=value` for a register or privileged register | debugger | `DEPOSIT R0 = 5`, `DEPOSIT PC = 200` |
| RADIX | both | `SET RADIX [/INPUT\|/OUTPUT] DECIMAL\|HEXADECIMAL\|OCTAL\|BINARY` (also 8, 10, 16) |
| BREAKPOINT (/TEMPORARY, /INSTRUCTION, /FAULT) | debugger | `SET BREAK` (VMS qualifiers; `/FAULT=name` *govax*) |
| STEP | debugger | `SET STEP` (VMS keywords; govax's IN and RETURN accepted) |
| TRACE, DISASSEMBLY, DISASSEMBLER | debugger | `SET TRACE/INSTRUCTION` (a tracepoint on each instruction: govax's trace) |
| MODE (access mode) | debugger | `SET MODE` takes VMS's keywords and the access modes (Decision 4) |
| PSL, PTE, VM, MAPEN, BASE, FAULT HISTORY | debugger | *govax* |

**CLEAR** becomes the debugger's `CANCEL` where it acts on debugger
state (Decision 5):

| Keyword | Goes to | In the debugger |
|---|---|---|
| BREAKPOINT (/ALL, /FAULT, /INSTRUCTION) | debugger | `CANCEL BREAK [/ALL\|/INSTRUCTION\|/EXCEPTION…]` |
| INTERRUPT, TB | debugger | *govax* `CANCEL INTERRUPT`, `CANCEL TB` |
| MEMORY/STATISTICS | debugger | *govax* |
| SYMBOL, STRINGS, MEMORY | console | |
| ERROR, PROFILES | removed unless a meaning turns up (bug 5) | |

`vax.init` changes with the split. `set quantum 1` and `set debug ...`
stay console commands. `set PC=200` is dropped (the author, at review):
nothing needs it. A loaded program starts at its own transfer address,
and the console's assembler already starts at X^200 on its first run
(a new session's origin is the current location, which VMINIT sets to
200).

### Output: VMS's by default

The debugger's messages and layouts are VMS's (above). govax's extra
per-instruction detail (the stack prefix, the register changes after
each traced instruction) is kept as an option the user turns on
(Decision 3), not the default.

### Expressions

The debugger reuses the console's `Evaluator` with what the VMS
debugger's address expressions need:

- register names (`R0`…`R11`, `AP`, `FP`, `SP`, `PC`, `PSL`, and `%R0`
  forms) as values (bug 7);
- `.` before an operand for its contents (`.PC`, `.R1+4`), and a lone
  `.` for the current location;
- ranges (`A:B`) and lists (`A,B`) in EXAMINE;
- path names and `%LINE n` (Phase 41 already has them);
- the debugger's own input radix.

### Breakpoints, tracepoints, watchpoints

A single eventpoint list replaces `Console.Breakpoints`, the
instruction-breakpoint map, and the step breakpoints. Each entry has a
kind (address, instruction class, exception, govax fault, internal
step), `/AFTER` count, `/TEMPORARY`, `WHEN`, `DO`, and, for a step
breakpoint, the frame it belongs to (bugs 2 to 4). Fault breakpoints
still trip inside `Engine.Step` (`cpu/faultbreak.go`), but they are set
and cancelled through the same list. A tracepoint is a breakpoint that
reports and continues. A watchpoint compares its location after each
instruction and stops when the value changes. VMS's static and
nonstatic watchpoints both reduce to that here. The VMS report shows
the old and new values and the instruction that changed them.

## The fixtures

- **The Phase 41 probe** (`testdata/dbg/vax/*.dlg`): the start-up
  state, STEP by instruction and line, SET BREAK on routines, labels,
  and JSB subroutines, `GO`, `SHOW CALLS`, the unhandled-exception break
  (`faillnk.dlg`), the image-exit message, EXAMINE of data and
  registers, EVALUATE, SYMBOLIZE, SHOW IMAGE, SHOW MODULE, SHOW SYMBOL.
- **A new probe** (subtask 1, `testdata/dbgcmd/`) for what the Phase 41
  probe doesn't show: SHOW BREAK, SHOW TRACE, SHOW WATCH, and SHOW STEP
  output; SET BREAK with `/AFTER`, `/TEMPORARY`, `/INSTRUCTION`,
  `/CALL`, `/BRANCH`, `/RETURN`, `/LINE`, `/EXCEPTION`, `WHEN`, and
  `DO`; tracepoint and watchpoint reports; STEP/OVER across a recursive
  call; STEP/RETURN; `CANCEL BREAK` messages; DEPOSIT to memory and
  registers; EXAMINE type qualifiers (`/BYTE`, `/WORD`, `/ASCII:n`,
  `/DECIMAL`, `/PSL`, `/OPERANDS`); `SHOW STACK`; `SET MODE` and
  `SHOW MODE`; `CALL` from the debugger; Ctrl/C isn't scriptable, so it
  is noted rather than probed; error messages for a bad symbol, a bad
  qualifier, and a command typed out of place.

## Subtasks

Each is independently testable and ends with `go build`, `go vet`,
`go test`, and golangci-lint clean, a commit, and `build -i` when it
changes behavior. Each adds to this doc's progress log, including any
bug found and fixed on the way.

1. **The probe** (`testdata/dbgcmd/`): a small MACRO program (a
   recursive routine, a data area to watch, JSB and CALLS routines, a
   branch-heavy loop, a signalled condition with a handler, and an
   unhandled one), a `.DBG` command file per topic above, a `.COM` to
   build and run them, `exchange.cmd`/`copyout.cmd`, and a README, as
   Phase 41's probe. The author runs it on simh while subtasks 2 to 5
   go ahead; its logs are checked in under `vax/` once audited.
2. **Fix the run-control bugs in place**, before anything moves, so the
   move starts from a correct baseline. Bug 1: `Call`'s loop uses the
   shared run loop, so RUN and CALL honor breakpoints. Bugs 2 and 3:
   step breakpoints are removed whenever the run stops, and can share
   an address with a user breakpoint. Bug 4: STEP/OVER's breakpoint
   records the frame (FP) and fires only there. Bug 6: the
   instruction-break message names the location. Tests: each bug's
   scenario on `dbgdis.exe` (and a recursive fixture for bug 4) fails
   before the fix and passes after.
3. **The package, the grammar, and the mode switch.**
   `internal/debugger` with a `Debugger` (session state) and its
   `Dispatcher`; `internal/bootdata/files/debug.dcl` with `EXIT`,
   `QUIT`, `HELP`, and `@`; `debug.help` started; `Console.Debugger`
   and the front end's routing; the console's `DEBUG` verb;
   `cmd/govax` loading the second grammar and showing `DBG> `. A test
   support package (`internal/console/consoletest`) exports the helpers
   the debugger's tests need: a runnable console, the kernel's and the
   fixtures' paths, the grammars. Tests: DEBUG enters and EXIT leaves;
   the prompt follows the mode; a console verb at `DBG>` is refused with
   the hint; a command file switches grammars at the right line;
   `XFC$CONSOLE_CMD` still reaches the console grammar.
4. **Run control moves to the debugger**: the run loop, the eventpoint
   list (address, instruction, and fault breakpoints, and step
   breakpoints), `GO`, `CALL`, and `STEP` with govax's current modes,
   and the stop messages. The console's `GO`/`CALL` start sessions by
   the Decision 2 rules; Ctrl-C returns to `DBG>`. `vax.init` still
   boots. The tests that drive these move from `internal/console` to
   `internal/debugger` (`execute_test`, `step_test`, `instbreak_test`,
   `faultbreak_test`, `dbgstep_test`, ...), changed only where they
   entered the debugger implicitly. Tests: the moved tests; GO that
   halts returns to `VAX>`; GO that hits a breakpoint gives `DBG>`;
   `CALL/STEP`; Ctrl-C through `Engine.Attention`.
5. **RUN under the debugger.** `RUN /[NO]DEBUG` (with `/STEP` kept as a
   synonym), `IHD$V_LNKDEBUG` from the image header, the start-up
   messages, stopping at the main routine's first instruction after its
   entry mask (not inside IMAGE$INIT's driver), `%DEBUG-I-EXITSTATUS`
   at image exit, and the unhandled-exception break (`faillnk.dlg`).
   A `/NOTRACEBACK` image runs without the debugger. The `govax run`
   subcommand gets `--debug`. Tests: the start of `dbgdis.dlg` and
   `faillnk.dlg`, and `RUN` of `dbgnotb.exe`.
6. **Breakpoints as VMS's.** `SET BREAK` (address lists; a routine name
   breaks after its entry mask with `break at routine`; `/AFTER:n`,
   `/TEMPORARY`, `/INSTRUCTION[=(…)]`, `/CALL`, `/BRANCH`, `/RETURN`,
   `/LINE`, `/EXCEPTION`, `/FAULT=` *govax*, `WHEN`, `DO`),
   `SHOW BREAK`, `CANCEL BREAK [/ALL]`, and the VMS messages. Tests:
   `dbgdis.dlg`'s break sequence (JSBRTN twice, SUB2 twice, LAST, with
   SHOW CALLS at each), and the new probe's breakpoint sessions.
7. **STEP as VMS's.** `STEP` and `SET STEP` with `/INSTRUCTION`,
   `/LINE`, `/INTO`, `/OVER`, `/RETURN`, `/SILENT`, and `/[NO]SOURCE`
   (accepted now, and acted on in subtask 8); line stepping from
   `dbgsym`'s line table; govax's `IN` and `RETURN` spellings;
   `SHOW STEP`. Tests: `dbgdis.dlg`'s instruction and line steps, the
   new probe's STEP/OVER across recursion, and STEP/RETURN.
8. **Source lines.** Locate a module's source file from its DST source
   correlation (`dbgsym.SourceFile`): on a mounted volume, as a host
   file, or along a `SET SOURCE` directory list (`SHOW SOURCE`,
   `CANCEL SOURCE`). Show the line after a step or break, wrapped at
   VMS's margin as the probe shows (`     -: e`). `SET STEP NOSOURCE`
   turns it off. Tests: every source line in `dbgdis.dlg`, `fail.dlg`,
   and the new probe; a missing source file shows the location alone.
9. **Expressions and `EXAMINE/INSTRUCTION`.** The evaluator additions
   above (bug 7), and `EXAMINE/INSTRUCTION` taking over `DISASSEMBLE`'s
   code, with ranges, lists, `.PC`, path names, `%LINE`, and
   `/OPERANDS[=FULL]`. `SET MODE [NO]SYMBOLIC` replaces the
   `vax.disassemble.symbolic` setting as the default (the setting seeds
   it). Tests: Phase 41's `TestDebuggerOracle` re-pointed at
   `EXAMINE/INSTRUCTION`, and `dbgdis.dlg`'s `EXAMINE/OPERANDS` lines.
10. **EXAMINE, DEPOSIT, EVALUATE, SYMBOLIZE for data.** EXAMINE of a
    location in VMS's layout (`DBGDIS\COUNT:   00000000`), typed by the
    DST where it names a datum (arrays as `TABLE[0:3]`, strings by
    descriptor), with `/BYTE`, `/WORD`, `/LONGWORD`, `/QUADWORD`,
    `/ASCII:n`, `/HEXADECIMAL`, `/DECIMAL`, `/OCTAL`, `/BINARY`, `/PSL`,
    and `/PTE` *govax*; registers (`FAILSUB\SUB2\%R2:`); DEPOSIT;
    `EVALUATE`, `EVALUATE/ADDRESS`, and `SYMBOLIZE`. Tests: every
    EXAMINE, EVALUATE, and SYMBOLIZE line in the probe sessions.
11. **CPU and kernel state commands move.** The debugger takes the SHOW,
    SET, and CANCEL keywords the tables above assign it, with govax's
    output where VMS has no command. `SET MODE` takes both keyword sets
    (Decision 4). `SET RADIX`, `SHOW RADIX`, and `CANCEL RADIX` follow
    VMS. `SHOW STACK` follows Decision 6. `SET PSL`, `SET PTE`, and the
    rest of govax's commands keep their syntax. Tests: the moved
    `show_test`/`set_test` cases, and the probe's `SHOW MODE` and
    `SHOW RADIX` lines.
12. **What the debugger knows about the program.** `SHOW IMAGE`,
    `SHOW MODULE`, `SHOW SYMBOL [/ADDRESS|/TYPE] pattern [IN module]`,
    `SHOW SCOPE`, `SHOW LANGUAGE`, and `SET MODULE` (accepted; govax
    loads every module eagerly, so it changes only `SHOW MODULE`'s
    column). Formats from the probe. Tests: `dbgdis.dlg`'s start-up
    block and its SHOW SYMBOL listings.
13. **Tracepoints and watchpoints.** `SET/SHOW/CANCEL TRACE`, with
    `SET TRACE/INSTRUCTION` replacing govax's `SET TRACE`, and
    `SET/SHOW/CANCEL WATCH` (bug 5's `SHOW WATCHPOINTS`). Reports in
    VMS's format from the new probe. Tests: the probe's tracepoint and
    watchpoint sessions; a watchpoint changed by a MOVC3 stops after
    that instruction.
14. **The console after the split.** Remove the moved verbs and
    keywords from `console.dcl`; resolve bug 5's remaining entries;
    drop `vax.init`'s `set PC=200` (see above), checking that a bare
    `ASM` after boot still starts at X^200; and update
    `internal/console`'s package doc. Check that every remaining console
    test passes without a debugger installed. Tests: a test lists every
    verb and keyword in each grammar, so a command that lands in the
    wrong one, or in neither, fails.
15. **Help.** `debug.help` covers every debugger command and qualifier.
    `vax.help` loses the moved commands and gains `DEBUG`, and its
    `RUN`, `GO`, and `CALL` entries describe starting the debugger.
    Tests: each grammar's verbs have a help topic (as Phase 37 did for
    the console).
16. **The session oracle.** `TestDebuggerSessionOracle` replays each
    probe command file (`testdata/dbg/*.dbg`, `testdata/dbgcmd/*.dbg`)
    through govax's debugger over the same images, and compares the
    output with the `.dlg` log line for line. It keeps a short,
    commented list of expected differences: VMS's memory counts and
    images that only VMS has (DEBUG, DBGSSISHR, DBGTBKMSG). The harness
    goes in at subtask 6 with the commands then supported, and each
    later subtask removes lines from the expected differences.
17. **Close-out.** `CLAUDE.md` (the new package, the two grammars, the
    modes), `PLAN.md`'s index, `DEVIATIONS.md` for anything left
    deliberately different from VMS, and the rules this doc logged as
    unconfirmed.

## Decisions

The author took each recommendation at review (2026-10-05).

1. **Dependency direction.** *Decided:* `internal/debugger` imports
   `internal/console`; the console sees the debugger through the
   `Debugger` interface that `cmd/govax` installs. The alternative,
   moving the machine (engine, memory, symbols, images) into a third
   package that both import, is cleaner but is a much larger move with
   no behavior gained. It can come later if the console's exported
   surface grows awkward.
2. **When `DBG>` appears for the console's GO and CALL.** *Decided:*
   only when the run stops; a run that finishes returns to `VAX>`, and
   once `DBG>` has appeared it stays until `EXIT`. Keeps `vax.init` and
   one-shot `GO`s as they are. The alternative is that GO and CALL
   always leave the user at `DBG>`.
3. **govax's extra trace detail.** *Decided:* VMS's output by
   default. `SET MODE [NO]REGISTERS` (*govax*, off by default) adds the
   register-change lines and the stack prefix to step, trace, and break
   reports.
4. **SET MODE's two meanings.** VMS's `SET MODE` sets display modes
   (`[NO]SYMBOLIC`, `[NO]LINE`, `[NO]OPERANDS`, `[NO]G_FLOAT`, ...);
   govax's sets the access mode. *Decided:* one `SET MODE` accepts
   both keyword sets, which don't collide; an ambiguous abbreviation
   such as `S` is refused, as DCL refuses any. `SHOW MODE` prints VMS's
   modes line, then `access mode: KERNEL`. The alternative is a separate
   `SET ACCESS_MODE`.
5. **CLEAR or CANCEL.** *Decided:* `CANCEL` is the debugger's verb,
   as VMS's, and `CLEAR` is accepted as a govax synonym for it.
6. **SHOW STACK.** VMS's `SHOW STACK` describes each call frame
   (govax's `SHOW CALL_FRAMES`); govax's `SHOW STACK` dumps the current
   stack's longwords. *Decided:* VMS's meaning. The longword dump
   stays as `SHOW KSP`/`ESP`/`SSP`/`USP`/`ISP` and as `SHOW SP` for the
   current stack.
7. **DISASSEMBLE.** *Decided:* gone from the console, and not kept
   as a debugger synonym. `EXAMINE/INSTRUCTION` is the command, and
   gains DISASSEMBLE's `/CONSTANTS` and `/SHAREABLE` as *govax*
   qualifiers.
8. **Radix.** *Decided:* the console and the debugger each have
   their own: the console's for its expressions (PRINT, IF, INIT), and
   the debugger's (hexadecimal, as VMS's MACRO default) for debugger
   input and output.
9. **A probe round.** *Decided:* yes (subtask 1). The manual gives
   syntax, but the Phase 41 probe showed that real output decides
   details the manual doesn't (case, column widths, line wrapping).

## Out of scope

- **Screen mode and the DECwindows interface** (`SET MODE SCREEN`,
  displays, windows, keypad).
- **Multiprocess and tasking debugging**, vector instructions, and event
  facilities.
- **Languages other than MACRO**, as in Phase 41.
- **Debugging shareable images' code by their own DSTs** (`SET IMAGE`):
  govax's shareable images are its own LIBRTL stubs.
- **Changing the CPU's fault-breakpoint mechanism** beyond listing it
  with the other eventpoints.

## Future features

Debugger commands govax would benefit from, roughly in order of value,
for a later phase:

- **EXAMINE's other types**: `/ASCIC`, `/ASCID`, `/ASCIW`, `/ASCIZ`,
  `/PACKED:n`, `/F_FLOAT`, `/D_FLOAT`, `/G_FLOAT`, `/H_FLOAT`,
  `/OCTAWORD`, `/DATE_TIME`, `/CONDITION_VALUE`, `/PSW`, `/TYPE=`,
  `/SOURCE`, and `SET TYPE`/`SHOW TYPE`/`CANCEL TYPE` for untyped
  locations. `vaxfloat` and `decimal.go` already have the conversions.
- **`DEPOSIT/INSTRUCTION`**: assemble one instruction in place with
  `internal/asm`.
- **Built-in symbols**: `%CURLOC`, `%NEXTLOC`, `%PREVLOC`, `^` (previous
  location), `\` (last value), Return to examine the next location, and
  `%ADDR`/`%NAME` forms.
- **`DEFINE`** (`/ADDRESS`, `/COMMAND`, `/VALUE`), `DELETE`, and
  `SHOW DEFINE`: debugger symbols and command abbreviations.
- **Control structures**: `IF`, `FOR`, `WHILE`, `REPEAT`, `EXITLOOP`,
  and `DO` clauses on tracepoints and watchpoints.
- **Logging**: `SET LOG`, `SET OUTPUT [NO]LOG/[NO]VERIFY/[NO]TERMINAL`,
  so a govax session can be logged like the probe's.
- **Scope**: `SET SCOPE`/`CANCEL SCOPE` (by path and call level),
  `SET MODULE`/`CANCEL MODULE` with on-demand loading, and
  `SHOW SYMBOL/DEFINED`.
- **Source commands**: `TYPE`, `SEARCH`, `SET/SHOW MARGINS`,
  `SET/SHOW MAX_SOURCE_FILES`, `EXAMINE/SOURCE`.
- **Exceptions**: stepping into a handler from an exception break,
  `GO` resignaling, and `SHOW EXIT_HANDLERS`.
- **`CALL` argument forms**: `%VAL`, `%REF`, `%DESCR`.
- **An initialization file** (`DBG$INIT`) and `SET PROMPT`.
- **`SPAWN`**: run one console command from `DBG>` without ending the
  session.
- **`SET WATCH` details**: `/AFTER`, `/TEMPORARY`, `/INTO`, `/OVER`,
  watching registers, and aggregates.

## References

- *VMS 5.5 Debugger Manual* (AA-LA59D-TE, November 1991,
  `~/Documents/Technical Doc/VMS/`): §2.4 (the command summary),
  chapters 3 and 4 (execution control, examining and depositing), and
  part III (the command dictionary). Also the *VAX/VMS 2.0 Symbolic
  Debugger Reference* (AA-D026B-TE) there.
- The Phase 41 probe: `testdata/dbg/README.md` and `vax/*.dlg`.
- `docs/PHASE-41.md` (the symbol table and display this phase builds
  on), `docs/PHASE-37.md` (the console grammar), `docs/PHASE-18.md`
  (STEP), `docs/PHASE-16.md` (fault breakpoints), `docs/PHASE-13.md`
  (RUN and CALL).
- **Clean room**: the debugger's behavior comes from DIGITAL's manuals
  and real VMS output only, never from VMS's source. A rule neither
  settles is chosen and logged here as unconfirmed.

## Progress Log

### 2026-10-05 — Planned

This plan written for review. Before writing it, two of the known bugs
(RUN and CALL ignoring breakpoints, and the STEP/OVER breakpoint leak)
were confirmed with a throwaway test, which was not kept. During
planning the author added three points: the VMS 5.5 Debugger Manual is
the reference for command syntax; bugs and gaps in the existing
breakpoint and step code are in scope; and the plan lists future
debugger features.

### 2026-10-05 — Reviewed

The author took every recommended decision (1 to 9). `vax.init`'s
`set PC=200` is dropped rather than replaced: a loaded program starts at
its transfer address, and the assembler's first run starts at X^200.
Subtask 14 removes the line, and its tests check the assembler's start.

### 2026-10-05 — Subtask 1: the probe, ready for VMS

`testdata/dbgcmd/`, laid out as Phase 41's probe was:

- **`dbgcmd.mar`**: one module with a recursive routine (`FACT`, called
  as FACT(5), returning to `BACK` at each depth), a three-pass loop of
  branches, data written by MOVL, ADDL2, INCL, MOVB, and MOVC3, a JSB
  subroutine, and `SS$_ENDOFFILE` (a warning) signalled twice: once
  with a frame handler established, and once with none. govax
  assembles, links, and runs it (`DBGCMD: done`, after
  `%SYSTEM-W-ENDOFFILE` for the unhandled signal); its listing confirms
  the addresses the command files assume (`BACK+3` is the RET; DATA at
  200, CODE at 400).
- **Nine debugger command files**, one per topic: `break`, `brkcls`
  (instruction-class breaks), `step`, `watch`, `trace`, `exam`, `call`,
  `except`, `errors`. The README's table says what each asks.
- **`dbgcmd.com`** builds the image `/DEBUG`, with its listing, map, and
  ANALYZE/IMAGE, and runs it under the debugger once per file, each
  session logged to its own `.dlg`.
- **`exchange.cmd`** builds `testdata/disks/dbgcmd-exchange.dsk` (label
  DBGCMDX): checked by running it under govax; the volume is built and
  ready. **`copyout.cmd`** brings the results back to `vax/`.

No govax image goes on the volume this time. Phase 41 showed that VMS
reads govax's `LINK/DEBUG` images, and subtask 16's oracle runs VMS's
own image under govax.

Ctrl/C can't be scripted, so it isn't probed; the manual's description
(it interrupts the program and returns to `DBG>`) stands for it.

From the *VMS 5.5 Debugger Manual*, notes for later subtasks:

- **MACRO's language expressions** (appendix E.8) use BLISS-style
  operators: `EQL`, `NEQ`, `GTR`, `GEQ`, `LSS`, `LEQ` and their
  unsigned `U` forms, `MOD`, `NOT`, `AND`, `OR`, `XOR`, `EQV`, infix
  `@` for a left shift, and prefix `.` and `@` for contents. Also `[ ]`
  subscripts, `<p,s,e>` bit fields, and a label followed by a storage
  directive is an array. Subtasks 6 (`WHEN`), 9, and 10 use these; the
  probe's `exam.dbg` checks a few.
- **`CALL`** passes arguments `%ADDR` by default (`%VAL`, `%REF`,
  `%DESCR` otherwise), and saves and restores the general registers
  around the call.
- **SHOW STACK** takes a count (`SHOW STACK [n]`), as SHOW CALLS does.
