# Phase 42 — The debugger: its own package, grammar, and prompt

**Status:** in progress. Planned and reviewed 2026-10-05 (the author
took every recommended decision). Subtasks 1 to 11 are done (see the
progress log).

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
4. ~~**STEP/OVER isn't tied to a call frame.**~~ Not a bug: the probe
   (`step.dlg`) shows VMS's STEP/OVER of FACT's recursive CALLS stopping
   at the first return to BACK, four frames deep, as govax's does. govax
   keeps that behavior.
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
step), `/AFTER` count, `/TEMPORARY`, `WHEN`, and `DO`. A step
breakpoint belongs to the STEP that set it (bugs 2 and 3). Fault breakpoints
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
   an address with a user breakpoint. Bug 6: the instruction-break
   message names the location. Tests: each bug's scenario on
   `dbgdis.exe` fails before the fix and passes after.
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
   `SHOW STEP`. `/BRANCH` and `/CALL` step to the next instruction of
   that class. Tests: `dbgdis.dlg`'s instruction and line steps, and
   `step.dlg`.

   **STEP/RETURN copies VMS's** (the author's choice at subtask 2's
   review), as `step.dlg` shows it:
   - It stops *at* the RET that ends the call frame current when the
     command was given (the frame FP pointed to), before that RET runs,
     still in the routine: `stepped on return from X to Y: RET`, where
     X is the location the STEP/RETURN was given at. govax's stops
     after the RET, in the caller.
   - It is bound to the frame, not to the code. From a JSB subroutine,
     which has no frame of its own, it waits for the RET of the CALLS
     frame the subroutine runs in, and passes the subroutine's RSB.
     In a recursive routine, a deeper call's RET doesn't stop it, since
     that RET ends another frame.
   - It stays pending until that RET is reached. A break, an exception
     break, or other STEPs in between don't cancel it: in `step.dlg` it
     fired three STEPs after an unhandled-exception break, at START's
     RET. This replaces subtask 2's removal of STEP/RETURN's breakpoint
     when its run stops (`endStep`); `TestStepReturnEndsAtBreakpoint`
     changes to expect the pending return to fire later.
   - Rules `step.dlg` doesn't settle, chosen and logged as unconfirmed:
     a second STEP/RETURN replaces a pending one; the pending return
     fires the same way during a GO as during a STEP; it is dropped when
     the frame goes away without its RET running (an unwind, or the
     image's exit).
   - STEP/OVER interrupted by a break keeps subtask 2's behavior (it
     ends). VMS's wasn't probed; this is logged as unconfirmed, for a
     later simh round.
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

### 2026-10-05 — Subtask 1: the probe's results

The author ran `@DBGCMD/OUTPUT=DBGCMD.LOG` on VMS 7.3; `copyout.cmd`
brought back `testdata/dbgcmd/vax/` (the README lists the files). Every
session ran to its end. govax's object for DBGCMD matches VMS's but for
header text and the source file's name. What the sessions show, for the
subtasks that use them:

**Breakpoints** (`break.dlg`, `brkcls.dlg`; subtask 6)

- SHOW BREAK lines: `breakpoint at routine DBGCMD\FACT` (a routine
  name), `breakpoint at DBGCMD\FACT\BACK` (a label),
  `breakpoint at DBGCMD\FACT\%LINE 62` (an address with no label, named
  by its line), with ` [temporary]` appended, and indented
  `   /after: 3`, `   when (.WATCHL EQL 2)`, `   do (EXAMINE R0; SHOW
  CALLS)` lines below. None: `%DEBUG-I-NOBREAKS, no breakpoints are
  set` (also CANCEL BREAK of one that isn't set).
- The order SHOW BREAK lists them in isn't the order they were set
  (`LOOP, ODD` then `LAST` lists ODD, LOOP, LAST; adding BUMP, CATCH,
  and HANDLR lists LAST first). Unconfirmed: govax will choose an order
  and log it, unless a rule shows itself.
- Breaks: `break at routine DBGCMD\FACT`, `break at DBGCMD\FACT\BACK`,
  then the source line. `/AFTER:3` breaks the third time and every time
  after.
- Instruction classes: `breakpoint on calls:` followed by the opcodes
  eight to a line in nine-column fields (BSBB BSBW CALLG CALLS JSB RET
  RSB); `breakpoint on branches:` likewise (43 opcodes, including
  CASEx, JMP, ACBx, AOBxxx, SOBxxx, BBxx, BLBx); `breakpoint on lines`;
  `breakpoint on instruction(s): ` and the opcodes given;
  `breakpoint on instructions` (all). Breaks: `break on calls at X`,
  `break on branches at X`, `break on lines at X`,
  `break on instruction(s) at X` (given opcodes), `break on instruction
  at X` (all).
- `/RETURN FACT`: `breakpoint on return from routine DBGCMD\FACT`;
  `break on return from routine DBGCMD\FACT at DBGCMD\FACT\%LINE 58`,
  at each RET of each recursion.
- **WHEN and DO.** In a MACRO language expression a data label's value
  is its contents: `WHEN (.WATCHL EQL 2)` read address 2,
  `%DEBUG-E-NOACCESSR, no read access to address 00000002`, and the
  break was taken anyway (an error in WHEN breaks). DO's commands are
  echoed as split at the `;`: `EXAMINE R0;` then `  SHOW CALLS`.
- SHOW CALLS shows a handler's frame with `----- above condition handler
  called with exception 00000870:`, the condition's message, and
  `----- end of exception message`.

**Exceptions** (`except.dlg`, every session; subtasks 5 and 6)

- An unhandled condition: its message, then `break on unhandled
  exception preceding DBGCMD\START\%LINE 48` and the source line:
  *preceding*, since the PC is after the CALLS of LIB$SIGNAL. It stops
  even for a warning, so every session that reached it stopped there.
- `SET BREAK/EXCEPTION` (`breakpoint on exception`): `break on exception
  preceding X` for a handled condition and for an unhandled one, and
  then `break on unhandled exception` too for the unhandled one. STEP
  from an exception break goes into the handler: `stepped to routine
  DBGCMD\HANDLR`.
- The image's end: `%DEBUG-I-EXITSTATUS, is '%SYSTEM-S-NORMAL, normal
  successful completion'`. After it, GO and STEP give
  `%DEBUG-E-BADSTARTPC, cannot start from PC 00000000`, SHOW CALLS
  `%DEBUG-E-NOCALLS, no active call frames`, and the image's data can
  still be examined.

**Stepping** (`step.dlg`; subtask 7)

- SHOW STEP: `step type: source, nosilent, by line,` then
  `           over routine calls` (or `into`, `by instruction`,
  `nosource`).
- `stepped to X` (by line) or `stepped to X: INSTRUCTION` (by
  instruction), then the source line; `stepped to routine DBGCMD\FACT`
  into a routine. `STEP 2` takes a count. `/SILENT` prints nothing;
  `/NOSOURCE` only the `stepped to` line.
- **STEP/OVER isn't frame-bound**: over FACT's recursive CALLS it
  stopped at the first return to BACK, four frames deep (R0 = 1). So
  bug 4 isn't a bug; the plan is corrected.
- **STEP/RETURN stops at the RET**: `stepped on return from
  DBGCMD\FACT\BACK to DBGCMD\FACT\%LINE 62: RET`, still in the routine.
  From a JSB subroutine it didn't stop at the RSB; it ran to the
  unhandled exception, and the step's return event fired later, at
  START's RET (`stepped on return from DBGCMD\FACT\BUMP to
  DBGCMD\START\LAST`). VMS kept that pending step across the exception
  break. govax will copy this (the author, at subtask 2's review; see
  subtask 7).
- `STEP/INTO/OVER` is accepted (the last wins); so is
  `EXAMINE/BYTE/WORD`.

**Watchpoints** (`watch.dlg`; subtask 13)

- SHOW WATCH: `watchpoint of DBGCMD\WATCHL`, `watchpoint of
  DBGCMD\BUFFER[0:15]` (an array), `[temporary]`; none:
  `%DEBUG-I-NOWATCHES, no watchpoints are set`.
- A report: `watch of DBGCMD\WATCHL at DBGCMD\START\%LINE 40`, the
  source line, `   old value: 00000000`, `   new value: 00000002`,
  then `break at` the next instruction and its source line. Values are
  shown at the datum's size (`00` for a byte).
- MOVC3 into the watched array gives one report per element changed,
  from [15] down to [0], each with its own `break at`, for one GO.
- DEPOSIT to a watched location doesn't trigger it; a temporary
  watchpoint is gone after it triggers.

**Tracepoints** (`trace.dlg`; subtask 13)

- SHOW TRACE: `tracepoint on instructions`, `tracepoint at routine X`,
  `tracepoint at X`, `tracepoint on lines`, `tracepoint on branches:`
  and the opcodes; none: `%DEBUG-I-NOTRACES, no tracepoints are set, no
  opcode tracing`.
- Reports: `trace on instruction at X`, `trace at routine X`,
  `trace on lines at X`, `trace on branches at X`, each followed by the
  source line, which is shown only once when two reports (or a trace
  and a break) fall on one PC.

**Examining** (`exam.dlg`; subtasks 9 to 11)

- Registers: `DBGCMD\FACT\%R0:        00000006`, named in the current
  routine's scope. `EXAMINE PSL` (and `/PSL`) prints a field table:
  `        CMP TP FPD IS CURMOD PRVMOD IPL DV FU IV T N Z V C` and the
  values beneath.
- A location with no symbol is shown by address: `.SP` and `@SP` give
  `7FED5314:       00000000`; `.AP+4` works.
- `EXAMINE` with no address shows the next location (after WATCHL,
  WATCHB); `.` the current; `^` the previous (here `DBGCMD\WATCHL+3`,
  a longword: going back by the current type's size from a byte).
- Data are typed by the DST: SOURCE (`.ASCII`) is a string
  (`'0123456789ABCDEF'`), BUFFER (`.BLKB 16`) a byte array shown one
  element a line (`    [0]:        00`), WATCHB a byte (`00`), MSG
  (`.ASCID`) its string. `EXAMINE/ASCII:16 SOURCE` shows
  `'0123456789ABCDEF......'`, 22 characters: VMS's own oddity, to
  match or log. A numeric range is shown by symbol and type
  (`EXAMINE 200:20C`: WATCHL, WATCHB, BUFFER[0], BUFFER[4]).
- Radix forms: decimal unpadded (`4`), octal 11 digits
  (`00000000004`), binary in two groups of 16 bits.
- **EVALUATE**: a data label is its contents (`EVALUATE WATCHL` is 0),
  a register its value, `.R2` the contents at R2's value; MOD, EQL,
  NEQ, infix `@` (shift), NOT, AND work; a hex result starting with a
  letter gets a leading 0 (`0FFFFFFFF`); `EVALUATE/DECIMAL 100` is 256
  (input hexadecimal). DEPOSIT's value is read in the input radix
  (`DEPOSIT R3 = 99` gives `00000099`).
- SHOW STACK: per frame, `stack frame n (address)`, the handler, SPA,
  S, mask, PSW, saved AP, FP, PC (symbolic), saved registers, and the
  argument list; the last frame is the debugger's own
  (`SHARE$DEBUG+0AD`). `SHOW STACK 1` shows one frame.
- SET MODE NOSYMBOLIC numbers instruction addresses (`00000485:
  MULL2 R2,R0`) but EXAMINE of data still names it. NOLINE names a
  location `DBGCMD\FACT+1A`. SHOW MODE: `modes: symbolic, line,
  d_float, noscreen, scroll, nokeypad, dynamic, interrupt, no separate
  window` and the two radix lines; OPERANDS adds `brief operands`.
  CANCEL MODE and CANCEL RADIX restore the defaults.
- SHOW SCOPE lists the call levels: ` *  0 [ = DBGCMD\FACT ], `,
  `    1 [ = DBGCMD\FACT 1 ], `, `    2 [ = DBGCMD\START ]`.

**CALL** (`call.dlg`; subtasks 4 and 6)

- `value returned is 00000018`; R0 and R2 are as before the call.
- A break inside a called routine stops there; SHOW CALLS marks the
  boundary with `----- above routine called from DEBUG CALL command`,
  and the GO that finishes the call prints `value returned is`.
- `CALL LIB$PUT_OUTPUT` gives NOSYMBOL: the debugger doesn't know
  LIBRTL's symbols until it is told to (`SET IMAGE`).

**Errors** (`errors.dlg`; subtasks 3 and 6)

- Unknown verbs, keywords, and qualifiers, DCL commands (`DIRECTORY`,
  `SHOW DEFAULT`, `SET DEFAULT`), an address with `/CALL`, and
  `SET RADIX 7` all give `%DEBUG-E-SYNTAX, command syntax error at or
  near 'X'`, naming the first word it couldn't take. So subtask 3's
  message for a console command at `DBG>` is this one, with govax's
  hint after it.
- Unknown symbols: `%DEBUG-E-NOSYMBOL, symbol 'NOSUCH' is not in the
  symbol table`. A missing value: `%DEBUG-W-NEEDMORE, unexpected end of
  command line`. A bad address: `%DEBUG-E-NOACCESSR, no read access to
  address 00000000`.
- Under `SET OUTPUT VERIFY`, a command that fails while being parsed or
  while its names are looked up isn't echoed; its message is all the
  log shows. Subtask 16's oracle needs this rule.

### 2026-10-05 — Subtask 2: the run-control bugs, fixed in place

Fixed in `internal/console`, before anything moves to the debugger
package, each with a regression test in `runcontrol_test.go` on
DBGDIS. Each test failed on the old code (checked by stashing the
fixes) and passes now.

- **Bug 1, RUN and CALL ignored breakpoints.** `Console.Call` had its
  own run loop with no breakpoint check, and RUN goes through it. It
  now runs through `runLoop`, as GO does. The first instruction is
  checked too (`skipFirstCheck` false): it's the called routine's, not
  where the console was stopped, so a breakpoint there fires at once.
  `TestRunStopsAtBreakpoint` (RUN stops at SUB2 twice, then GO
  finishes the image) and `TestCallStopsAtBreakpoint` (a break on the
  called routine's first instruction). The console's `/entry=` commands
  (ABOUT, SHOW VERSION) call kernel routines through `Call` too, and so
  now stop at a breakpoint set in them.
- **Bugs 2 and 3, a STEP's one-shot breakpoint outlived it.**
  `setStepBreakpoint` returns the breakpoint, and STEP/OVER and
  STEP/RETURN remove it when their run stops for any reason (`endStep`,
  deferred). That covers a run stopped elsewhere first (bug 2) and a
  return to an address with a user breakpoint, which `breakpointAt`
  finds first (bug 3); the stop is reported as the user's breakpoint.
  `TestStepOverEndsAtBreakpoint`, `TestStepReturnEndsAtBreakpoint`,
  `TestStepOverReturnsToBreakpoint`. VMS's debugger kept a STEP/RETURN
  pending across an exception break (subtask 1's results). At review,
  the author chose to copy that: subtask 7 makes STEP/RETURN wait for
  its frame's RET across other stops, replacing `endStep` for it.
  `endStep`'s comment says so.
- **Bug 4** was not a bug (subtask 1's results); nothing changed.
- **Bug 6, instruction breaks gave a bare address.** The message names
  the location as `Break at` does: `Instruction break at
  DBGDIS\START\%LINE 87` in a debug image, the eight-digit address
  elsewhere (so the existing tests are unchanged).
  `TestInstructionBreakNamesLocation`.

Bugs 5 (grammar entries with no handler) and 7 (registers in
expressions) belong to the subtasks that rework those commands (14 and
9). No other bug turned up in this code. `go build`, `go vet`, `go test
./...`, and golangci-lint are clean.

### 2026-10-05 — Subtask 2 reviewed

The author chose to copy VMS's STEP/RETURN rather than end it when its
run is interrupted. Subtask 7 now says how: it stops at the RET that ends
the frame current when it was given, and stays pending across other
stops until that RET runs. Until then, subtask 2's `endStep` removes
STEP/RETURN's breakpoint as it does STEP/OVER's; its comment points at
subtask 7.

### 2026-10-05 — Subtask 3: the package, the grammar, and the mode switch

`internal/debugger` exists, with a `Debugger` (session state, and the
`console.Debugger` interface it implements) and a `Dispatcher` over its
own grammar. Nothing of the debugger's old work has moved yet: `GO`,
`CALL`, `STEP`, `EXAMINE`, and the rest are still the console's
(subtask 4 on). What this subtask makes is the frame they move into.

- **Grammar and help.** `internal/bootdata/files/debug.dcl` has `EXIT`,
  `QUIT`, `HELP` (and `?`), and `@`; `debug.help` documents them in
  `vax.help`'s format. Verb ids start at 1, since the grammar is parsed
  apart from `console.dcl`'s. `EXIT` and `QUIT` both end the session
  (they differ on VMS only in running exit handlers, which govax has
  none of yet; logged here as unconfirmed for later).
- **Console side** (`internal/console/debugger.go`). `Console.Debugger`
  is typed by a small interface (`Start`, `Active`, `Dispatch`);
  `Activation` and `ActivateAttach` name the one way in so far. The new
  console verb `DEBUG` (`console.dcl`, id 6000) calls
  `Console.StartDebugger`. With no debugger installed it says
  `%DEBUG-E-NOTAVAILABLE`.
- **Routing.** `Dispatcher.Dispatch` sends a line to the debugger when a
  session is active, after the assembler-mode check (`ASM>` still wins).
  It's in `Dispatch`, not only the front end, so a command file or a
  script on stdin changes grammar at the right line. The rest of the old
  `Dispatch` is `DispatchConsole`, which `XFC$CONSOLE_CMD`
  (`Console.ConsoleCommand`) calls, so a VAX program asking for a console
  command gets the console even with a session active.
- **A console command at `DBG>`.** The debugger's parse failures that are
  a bad word come out as the VMS debugger words them, `%DEBUG-E-SYNTAX,
  command syntax error at or near 'X'`, naming the word as typed (the
  grammar reports `/NOFOO` as `FOO`; `qualifierAsTyped` puts the `NO`
  back, as `errors.dlg` shows). When the word is a console verb, a govax
  hint follows (`%DEBUG-I-CONSOLECOMMAND, ... EXIT returns to the
  console`). Both lines are printed by the debugger, and the error is
  returned with its message inhibited, so the loop doesn't print the
  first again. Other parse errors (a missing parameter) pass through
  unchanged until the subtasks that add commands choose their messages.
  New: `vmserrors/codes_dbg.go`, and the facility prints as `DEBUG` (it
  was `DBG`, used by nothing).
- **Front end.** `cmd/govax` parses `debug.dcl` and `debug.help` (both
  required, as the console's are), calls `debugger.Install`, and shows
  `DBG> ` while `Console.InDebugger()`.
- **`internal/console/consoletest`**: a test-support package with a
  runnable console, `RepoPath`/`BootFile`/`KernelPath`/`DebugImagePath`,
  both grammars, and the help files. Subtask 4's moved tests will use it.
  The console's own in-package tests keep their private helpers, since
  the package can't import `consoletest` without a cycle.
- **`dcl.Grammar.HasVerb`** says whether a word names a verb, for the
  hint above. `TestLoadEvaxGrammar_verbCount` is 57 with `DEBUG`.

Tests (`internal/debugger/debugger_test.go`, `cmd/govax/debug_test.go`):
DEBUG enters and EXIT/QUIT leave without quitting the console; the
console's EXIT still quits; the prompt follows the mode; a console verb
at `DBG>` is refused with the hint and doesn't run, while an unknown word
and an unknown qualifier get the plain syntax error; `HELP` reads
`debug.help` at `DBG>` and `vax.help` at `VAX>`; a command file changes
grammar at the right lines, in both directions, and `@file` at `DBG>`
does the same; `XFC$CONSOLE_CMD` reaches the console grammar during a
session; DEBUG with no debugger installed; and the whole thing through
`cmd/govax`'s `run`. `go build`, `go vet`, `go test ./...`, and
golangci-lint are clean.

### 2026-10-05 — Subtask 4: run control moves to the debugger

The run loop, the breakpoint lists, the step modes, and the stop messages
now live in `internal/debugger` (`runcontrol.go`, `step.go`,
`instbreak.go`, `faultbreak.go`); `internal/console` keeps the VMS side.

- **Who owns what.** `Debugger` holds `Breakpoints`,
  `InstructionBreakpoints`, and `StepMode`; fault breakpoints are still
  armed on `cpu.Engine` (they trip inside `Engine.Step`) but are set,
  cleared, and shown through the debugger. `Console.Execute`, `Call`, and
  `Step` are now thin: with a debugger installed they hand the run to
  `Debugger.Start` with an `Activation` (`ActivateGo`, `ActivateCall`,
  `ActivateStep`); a bare `Console` (no debugger) runs GO and CALL to the
  end with no breakpoints (`runPlain`), and refuses STEP and CALL/STEP.
  `RUN`'s IMAGE$INIT driver and the CHF's condition handlers still call
  `Console.Call`, so they go through the debugger too (bug 1 stays fixed).
- **What the console exports** (`export.go`, one file as planned):
  `RequireInit`, `LocationText`, `TraceStep`, `ExceptionName`,
  `ReportStop` (every non-debugger way a run ends: HALT, limits, a CALL's
  return, a CHF exception), `EvalWhole`, and `ParseCall` (CALL's routine
  and argument list, shared by both grammars).
- **The console's breakpoint commands** (`SET/CLEAR/SHOW BREAKPOINT`,
  `SET STEP`, `SHOW STEP_MODE`) stay in the console grammar for now, but
  reach the debugger's state through the `console.Eventpoints` interface
  (embedded in `console.Debugger`); with no debugger they say
  `%DEBUG-E-NOTAVAILABLE`. Subtasks 6 and 7 give the debugger its own
  `SET BREAK` and `SET STEP`, and this interface goes away.
- **Session lifetime (Decision 2).** `Debugger.Start` classifies each run
  as ended (HALT, CALL return, a govax limit, a CHF-reported exception) or
  stopped (breakpoint, completed STEP, a fault breakpoint, Ctrl-C). A stop
  opens the session (`DBG>`); an end opens none, so `go exe$initialize` in
  `vax.init` and any GO that halts return to `VAX>`. A session already
  open stays open however the run ends. A run started inside another (a
  condition handler) doesn't decide this; only the outermost does. A
  `STEP` at the console always stops, so it opens a session.
- **`debug.dcl`** gained `GO` (`EXECUTE`, `G`), `CALL`, and `STEP` (`ST`,
  `S`), same qualifiers as the console's; `debug.help` has their topics.
- **Known limit until subtask 6 and later:** at `DBG>` only the commands
  above, `EXIT`/`QUIT`, `HELP`, and `@` exist. `SET BREAK`, `EXAMINE`,
  `SHOW CALLS` and the rest still live in the console grammar, so after a
  stop one types `EXIT` to reach them, and `GO` from the console continues
  as before. (The moved tests send those through
  `Dispatcher.DispatchConsole`.)
- **Tests.** `step_test`, `instbreak_test`, `faultbreak_test`,
  `dbgstep_test`, `runcontrol_test`, the break/step parts of
  `execute_test`/`dispatch_test`/`set_test`/`misc_test` moved to
  `internal/debugger` (package `debugger_test`, helpers in
  `helpers_test.go`), changed only where they used console-private names.
  New `session_test.go`: GO that halts returns to `VAX>`; GO that breaks
  gives `DBG>` and the debugger's own GO continues it; STEP at `DBG>` and
  at the console; `CALL/STEP`; `CALL` with arguments at `DBG>`; Ctrl-C
  through `Engine.Attention`; breakpoint commands without a debugger.
  `internal/console`'s tests that only use `RUN/STEP` to land in an image
  use `stepdebugger_test.go`, a stand-in that takes one step (the real
  debugger imports the console, so the tests can't use it).
- `go build`, `go vet`, `go test ./...`, and golangci-lint are clean.

### 2026-10-05 — Subtask 5: RUN under the debugger

`RUN` now starts the debugger on an image the way VMS does
(`internal/debugger/image.go`, `internal/console/run.go`).

- **Qualifiers.** `/DEBUG` is a negatable qualifier; `/STEP` and `/BREAK`
  are synonyms for it (`RunOptions.Debug`: `DebugDefault`, `DebugOn`,
  `DebugOff`, replacing `Step`). `govax run` gets `--debug` and
  `--no-debug`.
- **When the debugger starts.** `/DEBUG`: if the image has a debug symbol
  table (traceback alone is enough, as `dbgtrc` showed). `/NODEBUG`:
  never. Neither: if the image header has `IHD$V_LNKDEBUG` (new
  `ICB.LinkDebug`) and a debugger is installed. An image with no table
  (`/NOTRACEBACK`) runs without the debugger whatever was asked, silently,
  as VMS ran DBGNOTB. `/DEBUG` with no debugger installed is
  `%DEBUG-E-NOTAVAILABLE`.
- **Start-up.** A new `console.ActivateImage` runs RUN's IMAGE$INIT driver
  (so shareable images' initialization routines run first) until the PC
  reaches the main routine's entry plus two (its entry mask is not code), a
  silent `Breakpoint.Quiet` of the debugger's own. It prints a banner and
  `%DEBUG-I-INITIAL, Language: MACRO, Module: DBGDIS` and opens the
  session. VMS's banner names its own version, so govax prints
  `govax VAX DEBUG`; the probe logs start after it, so no oracle compares
  it. Only language code 0 (MACRO) is known; other codes print `UNKNOWN`
  (unconfirmed).
- **Image exit.** When the driver's call returns (the outermost run only;
  a condition handler's call returns the same way) the debugger prints
  `%DEBUG-I-EXITSTATUS, is '<status message>'` from R0 and keeps the
  session open. GO and STEP then say `%DEBUG-E-BADSTARTPC, cannot start
  from PC 00000000` until the next RUN (PC is shown as 0, as the probe
  shows). A status with no message of its own is shown as `$GETMSG` would,
  FAO directives and all (`!XB`), which is what VMS prints too.
- **Unhandled conditions.** `corevms`' catch-all handler now writes its
  message once and, if `Environment.OnUnhandled` (the console's
  `OnUnhandled`, which the debugger sets) wants the condition, makes the
  `SYS$SRCHANDLER` service wait (`ErrWait`): the program is paused with
  its XFC unexecuted. The run loop sees the pending condition after that
  instruction and prints `break on unhandled exception at LOC` (a hardware
  exception: the faulting instruction) or `... preceding LOC` (LIB$SIGNAL
  or LIB$STOP: the return address). GO runs the XFC again, which skips the
  already-shown message and does the catch-all's action: exit for a severe
  condition, continue for a warning (`except.dlg`: ENDOFFILE breaks, then
  GO reaches the exit). Only while an image runs under the debugger; a
  plain GO or RUN/NODEBUG behaves as before. The source line VMS shows
  below the break is subtask 8's.
- **Unconfirmed.** The exit status of an image that ended in a severe
  condition is shown by VMS's debugger in a way `faillnk.dlg` doesn't show;
  govax prints the condition's message line. STEP/GO at an unhandled
  break all just run the pending XFC.
- **Tests.** `image_test.go`: RUN/DEBUG stops at START with no breakpoint
  left; defaults follow the link flag, `/NODEBUG`, and `/NOTRACEBACK`;
  exit status and BADSTARTPC; the access violation break (`faillnk`); the
  signalled warning's `preceding` break and its continue (the VMS-built
  `dbgcmd.exe`); no break without the debugger. `TestStatusText`,
  `TestRunCommandDebugQualifiers`, and RUN's option parsing. The tests
  that RUN/STEP-ed an image to land in it now land at its main routine
  (the stand-in debugger in `internal/console` does the same).
- `go build`, `go vet`, `go test ./...` clean; golangci-lint reports
  nothing in the changed files (existing findings elsewhere remain).

### 2026-10-05 — Subtask 6: breakpoints as VMS's

`SET BREAK`, `SHOW BREAK`, and `CANCEL BREAK` are now debugger commands
(`internal/debugger/breakcmd.go`, the `set`/`show`/`cancel` verbs in
`debug.dcl`), with the VMS messages from `break.dlg` and `brkcls.dlg`.
The console's own `SET BREAKPOINT`, `SHOW BREAKPOINTS`, and `CLEAR
BREAKPOINT` still work at `VAX>` (subtask 14 removes them); they and
the debugger's commands share the one list.

- **One list, more kinds.** `Breakpoint.Kind` is now `BreakAddress`,
  `BreakCall`, `BreakBranch`, `BreakLine`, `BreakInstruction` (opcodes),
  `BreakAnyInstruction`, `BreakReturn` (a routine's RETs), or
  `BreakException`; each has `/AFTER`, `/TEMPORARY`, `WHEN`, and `DO`
  (`eventpoint.go`). `runLoop` asks `breakpointHit`, which counts every
  breakpoint reached, applies `/AFTER` then `WHEN`, removes the one-shot
  ones that stop the run, and prints the first stop's message. A STEP's
  one-shot breakpoint now coexists with a user breakpoint at the same
  address and is removed when it's reached (bug 3's other half).
  Fault breakpoints stay on `cpu.Engine`, and the console's
  opcode-flag `InstructionBreakpoints` map stays for the old command;
  `CANCEL BREAK/ALL` clears all three.
- **Messages.** `break at X`, `break at routine DBGCMD\FACT` (a routine's
  name breaks just past its entry mask, found by
  `Console.RoutineEntry`), `break on calls|branches|lines at X`,
  `break on instruction(s) at X`, `break on instruction at X`, `break on
  return from routine R at X`, `break on exception preceding X`, and the
  condition's message line before an exception break. `SHOW BREAK`
  words each as `brkcls.dlg` does, with opcodes eight to a line (the
  branch list's 43 names are the probe's). Nothing set (or cancelled) is
  `%DEBUG-I-NOBREAKS`. The old stop text "Break at" is now "break at".
  govax's `/FAULT=code` is kept for `SET`, and `CANCEL BREAK/FAULT[=code]`.
- **`/EXCEPTION`** needs to know of every signal, handled or not, so
  `corevms.Environment` has `OnSignal` (called from `startDispatch`) and the
  console `Console.OnSignal`; the debugger holds the signal for the run
  loop, which stops at the next instruction boundary. The program is
  paused with the dispatch set up, and GO sends it into the handlers.
  The stop is after the dispatch is set up, where VMS's is before the
  signal is delivered, so `SHOW CALLS` there may differ from VMS's
  (untested; the SHOW CALLS rewrite is a later subtask's).
- **WHEN and DO.** `condition.go` is a stopgap for subtask 9's
  evaluator: comparisons (`EQL NEQ LSS LEQ GTR GEQ`), `AND`, `OR`, `NOT`,
  parentheses, and `.expr` for the longword at an address. A condition
  that can't be evaluated shows its error and breaks (as in `break.dlg`).
  **Difference from VMS, unconfirmed:** the probe's `WHEN (.WATCHL EQL 2)`
  failed with `NOACCESSR` at address 2, since in a MACRO-language
  expression the debugger takes a data label's value to be its contents;
  govax's evaluator takes a label's value to be its address, so `.WATCHL`
  is WATCHL's contents and the condition works. Subtask 10 (data typing)
  decides whether to follow VMS; `TestDebuggerSessionOracle` will list it
  as an expected difference if not. DO's commands run when the debugger is
  back at its prompt (`Start` calls `runDo`) and are not echoed (VMS echoes
  them only under SET OUTPUT VERIFY). EXAMINE and SHOW CALLS aren't debugger
  commands yet, so a DO using them reports `%DEBUG-E-SYNTAX` until the
  later subtasks.
- **SHOW BREAK's order** is the order set. The probe's isn't (a list's
  order reversed; `LAST` before the later ones), and no rule showed itself:
  unconfirmed.
- **Source lines** after each break are subtask 8's.
- **DCL parser.** A qualifier's value may follow `:` as well as `=`
  (`/AFTER:3`; `dcl.readBareToken`, `TestQualifierValueColon`). A
  qualifier with a `/default` is present in every result, so the
  grammar's `DISALLOW` can't say "at most one of /CALL, /INSTRUCTION, ..."
  once `/INSTRUCTION` takes an optional list; the commands check that
  themselves (`class`), using `Result.Defaulted`.
- **Tests.** `breakcmd_test.go` runs the probe's DBGCMD image: the
  routine, `/AFTER`, unlabeled-address, list, temporary, `WHEN`, `DO`,
  unreadable-`WHEN`, `/CALL`, `/RETURN`, `/BRANCH`, `/LINE`,
  `/INSTRUCTION`, and `/EXCEPTION` sequences of `break.dlg`, `brkcls.dlg`,
  and `except.dlg` (the messages and stops, not the SHOW CALLS and
  EXAMINE lines), the `WHEN` operators on a program of NOPs, and the
  commands' errors. The existing run-control tests changed only for the
  new text ("break at", "no breakpoints are set", `breakpoint on fault`).
- `go build`, `go vet`, `go test ./...` clean; golangci-lint reports
  nothing in the changed files.

### 2026-10-05 — Subtask 7: STEP as VMS's

`STEP`, `SET STEP`, and `SHOW STEP` are the debugger's commands now
(`internal/debugger/step.go`, the `step`/`set`/`show` verbs in `debug.dcl`),
with the output of `step.dlg` and `dbgdis.dlg`. The console's own `STEP`,
`SET STEP`, and `SHOW STEP_MODE` still work at `VAX>` (subtask 14 removes
them) and reach the same code.

- **Three choices, not one mode.** The unit (`/LINE`, the default, or
  `/INSTRUCTION`), what to do about calls (`/OVER`, the default, `/INTO`
  or `/IN`, `/RETURN`), and the report (`/[NO]SILENT`, `/[NO]SOURCE`);
  `/BRANCH` and `/CALL` run to the next instruction of the class. SET STEP
  takes the same keywords, several at once (`SET STEP INSTRUCTION,INTO`),
  and SHOW STEP prints the two-line form. **The defaults are VMS's now**
  (line, over), where `StepMode` used to default to INTO and
  `/INSTRUCTION` was a synonym for INTO. `STEP n` (the debugger's
  parameter is a count; the console's `STEP address` is unchanged) reports
  only where the last step ends. Of `/INTO`, `/OVER`, `/RETURN` the last
  typed wins (`lastCallMode` reads the order from the command line, since
  the grammar result doesn't keep it).
- **Reports.** `stepped to LOC` (by line), `stepped to LOC: INSTR` (by
  instruction, with `Console.InstructionText`: the mnemonic padded to 8),
  `stepped to routine R` (the PC is just past a routine's entry mask), and
  `stepped on return from X to Y: RET`. A stop that isn't the step's (a
  break, an unhandled exception, the program ending) says what it always
  did and ends the whole command. **STEP no longer shows the trace line
  for each instruction** (VMS doesn't); `SET TRACE` still does, and
  Decision 3's `SET MODE REGISTERS` is to bring back the register lines.
- **Line stepping** executes until the PC is at the start of a line in
  the line table (`Console.LineStart`). Where the PC has no line table
  (the kernel, console-assembled code) a step is by instruction. A line step
  that began in code with lines and leaves it (into a library routine, or
  off START's RET into the image driver) carries on until it is back in
  some or the program ends, so stepping off the last RET reaches
  `%DEBUG-I-EXITSTATUS`. **Unconfirmed:** a RET or RSB ends a line step
  at once, in the middle of the caller's line.
- **STEP/OVER** is as before: a call-like instruction (`BSBx`, `JSB`,
  `CALLG`, `CALLS`, `CHMx`) runs silently to the instruction after it, not
  bound to the frame (bug 4 wasn't a bug). Interrupted by a break it ends,
  as before (unconfirmed for VMS).
- **STEP/RETURN copies VMS's** (`pendingReturn`): bound to FP when the
  command was given, it fires when a RET is about to run with FP equal to
  that frame (`returnDue`, checked before every instruction by `runLoop` and
  `stepOne`), so a deeper call's RET and a JSB subroutine's RSB pass. It
  stays pending across breaks, exception breaks, and other commands, and
  a plain STEP that begins at the RET reports it without executing the
  RET (`step.dlg`'s STEP after `stepped to ... LAST: RET`). `endStep` no
  longer touches it. **Unconfirmed rules:** a second STEP/RETURN replaces
  a pending one; GO fires it as STEP does; it is dropped when FP moves
  above the frame (an unwind) and when the image exits or a new one runs.
  A frame whose saved PC can't be read, or no frame at all, is still
  `%CLI-F-NOFRAMES`/the memory error.
- **Step breakpoints.** A STEP's own breakpoints (the OVER target, and
  the class breakpoints `/BRANCH` and `/CALL` use) are `Quiet` and
  `Step`, are removed when they fire, and set `Breakpoint.fired`, which is
  how the STEP knows its breakpoint, not a user's, stopped the run. Where a
  user breakpoint stops the run first, only its message is shown. "Stepped
  to" as a breakpoint message is gone.
- **Exception breaks name the instruction when steps are by
  instruction** (`exceptionLocation`): `step.dlg`'s `break on unhandled
  exception preceding DBGCMD\START\%LINE 48: PUSHAQ   L^00000230` was in
  that mode, and every other log's break was in the default mode and has no
  instruction. Applied to exception breaks only; unconfirmed for the rest.
- **Not done here:** the source line after a step or break and
  `/[NO]SOURCE`'s effect (subtask 8), and `/EXCEPTION` for STEP (not in the
  plan). `SHOW CALLS` and `EXAMINE`, which `step.dlg` runs between the
  steps, aren't debugger commands yet, so `TestStep*` in `stepcmd_test.go`
  check the steps' own lines of the log.
- **Tests.** `stepcmd_test.go` replays `step.dlg` on the probe's DBGCMD
  image (defaults and units, a count, into a routine, STEP/OVER of the
  recursive call, STEP/RETURN and its stop at the frame's RET, BRANCH and
  CALL, the pending return across the unhandled-exception break, /SILENT,
  SET/SHOW STEP, last-mode-wins, a break inside a stepped-over call).
  `step_test.go` adds STEP/RETURN at the RET and its frame binding. Older
  tests changed only where the output did ("stepped to", no trace line, the
  defaults, `STEP 1` at `DBG>`).
- `go build`, `go vet`, `go test ./...` clean; golangci-lint reports nothing
  in `internal/debugger` or `internal/console`.

### 2026-10-05 — Subtask 8: source lines

After a break or a step the debugger shows the source line at the PC, as
VMS does, and `SET SOURCE`, `SHOW SOURCE`, and `CANCEL SOURCE` say where to
find the file.

- **Finding the line** (`internal/console/source.go`, `Console.SourceLine`):
  `dbgsym`'s `LineAt` gives the module and listing line, `Module.SourceOf`
  the file and record, and the file is read with `ReadRecordFile`. The
  file name in the DST is the build machine's (`DUA1:[000000]DBGCMD.MAR;1`
  in the probe's images), so the search is: the name as recorded; each
  `SET SOURCE` directory with the file's name and type (as recorded, then
  lower case, for a host file system with case); then the default
  directory. Files are cached by recorded name until the list changes. A
  file that can't be found, or a record it doesn't have, shows nothing:
  the location line stands alone.
- **Layout** (`internal/debugger/source.go`, `formatSource`), from the
  logs: the listing line number right-aligned in six columns, `: `, then
  the record with tabs expanded to every eighth column of the text; 72
  columns of text per line, with the overflow on `     -: ` lines
  (`dbgdis.dlg`'s line 110). **Unconfirmed:** a record over 144 columns
  wraps again the same way; no probe line is that long.
- **Where it shows.** After `break at ...` and the other break messages
  (`stopMessage`), `break on [unhandled] exception ...` (at the exception's
  PC), and every STEP report: `stepped to`, `stepped to routine`, class
  steps, and `stepped on return`. `/NOSOURCE` and `SET STEP NOSOURCE` turn
  it off for steps (STEP/RETURN keeps the choice made when it was given,
  as it does `/SILENT`). Breaks always show it, as VMS's SET STEP NOSOURCE
  didn't affect them in `step.dlg`; `SET BREAK/[NO]SOURCE` is still
  refused. govax's own `Instruction break at` shows none.
- **SET SOURCE dir[,dir...]** takes host paths and VMS directory
  specifications. Unquoted text is made upper case and `/` starts a
  qualifier, so a host path is quoted. `SHOW SOURCE` prints
  `source directory search list for all modules:` and the directories
  indented four; with none, `%DEBUG-I-NOSOURCEDIR, no source directory
  search list is in effect`. **Both wordings are govax's** (no probe
  showed them; unconfirmed). Per-module lists, `/LATEST` and `/EXACT`
  aren't done.
- **Tests.** `source_test.go` replays the source lines of `step.dlg`,
  `break.dlg`, and `except.dlg` on the probe image with `SET SOURCE`
  (steps, `/NOSOURCE`, `SET STEP NOSOURCE`, breaks at a routine, an
  exception break, STEP/RETURN), the not-found case, and SET/SHOW/CANCEL
  SOURCE; `source_format_test.go` checks the layout against `dbgdis.dlg`'s
  wrapped line. Not yet replayed: `dbgdis.dlg`, `fail.dlg`, and the
  probe's remaining sessions need commands (`SHOW CALLS`, `EXAMINE`) that
  come in later subtasks; the session oracle (subtask 16) covers them.
- `go build`, `go vet`, `go test ./...` clean; golangci-lint reports nothing
  in the packages touched.

### 2026-10-05 — Subtask 9: expressions and EXAMINE/INSTRUCTION

Bug 7 is fixed, and the debugger has `EXAMINE/INSTRUCTION`.

- **Evaluator** (`internal/console/expr.go`). A register name (`R0` to
  `R11`, `AP`, `FP`, `SP`, `PC`, `PSL`, any case, and `%R0`) is its
  contents, so `R1+4` and `SP-8` work wherever an address does. A `.` or
  `@` before an operand is "the contents of": for a register the same
  value (`.PC` is `PC`; `EXAMINE .SP` then shows memory at SP, as
  `exam.dlg` does), otherwise the longword at that address, read through
  `Evaluator.Load` (kernel-mode translation, as EXAMINE reads). An
  unreadable address is `%DEBUG-E-NOACCESSR`. A lone `.` is still the
  current location. A console symbol of a register's name beats the
  register. `condition.go`'s stopgap `.expr` handling is gone: WHEN
  operands go through the evaluator, so `WHEN (R1 GTR 2)` works too.
  `.` before a digit is now "contents of" that number (it was an error).
- **The debugger's radix** (`SET RADIX [/INPUT|/OUTPUT] radix`, `CANCEL
  RADIX`; Decision 8). Every expression the debugger evaluates
  (`Debugger.evalWhole`) reads unprefixed numbers in its input radix, and
  the output radix decides the offsets in names (`GLIMIT+589`). `SHOW RADIX`
  and the rest of the radix commands are subtask 11's.
- **`SET MODE [NO]SYMBOLIC`, `[NO]OPERANDS[=FULL|BRIEF]`**, the display
  modes `EXAMINE/INSTRUCTION` follows. `SYMBOLIC` is seeded from
  `vax.disassemble.symbolic` (`console.SymbolicDefault`). The other
  modes and govax's access mode are subtask 11's; any other word is
  `%DEBUG-E-SYNTAX`.
- **`EXAMINE/INSTRUCTION`** (`internal/debugger/examine.go`): a list of
  locations, each `a` or `a:b`, split outside parentheses and quotes, one
  instruction layout per `Console.DisassembleWith` (which got `Radix` and
  `Operands` options). A range's start typed as `%LINE n` is named by the
  line. `/CONSTANTS` and `/SHAREABLE` are govax's. With no location it
  shows the instruction at the current location (the deposit address, for
  now; subtask 10 gives the debugger its own). The data forms of EXAMINE
  (`EXAMINE R0`, ...) answer `%DEBUG-E-NOTAVAILABLE` until subtask 10. The
  console's `DISASSEMBLE` is untouched until subtask 14 removes it.
- **`/OPERANDS`** (`internal/console/dbgoperands.go`): after the
  instruction, a line per register or memory operand, as `dbgdis.dlg`,
  `dbgtrc.dlg`, and `exam.dlg` show: five spaces, the operand's text in
  ten columns, then `R0 contains 00000003` or `NAME (address A) contains V`
  (the address alone when no symbol names it); an operand longer than
  ten columns takes a line, and its description follows indented to
  column 16. The machine is as it is before the instruction runs (an
  autoincrement operand's address is the register's value now). Operand
  addresses are computed from the registers without running anything.
  `/OPERANDS` alone implies `/INSTRUCTION`. **Unconfirmed, govax's
  choices:** `=FULL` is the same as brief (the probe's two are identical);
  literals, immediates, branch targets, and inline data get no line; a
  quadword register operand shows the register pair as 16 digits; a
  value wider than 8 bytes shows 4; an unreadable location shows
  `<inaccessible>`.
- **Tests.** `TestExamineInstructionOracle` is Phase 41's
  `TestDebuggerOracle` moved here and driven through the debugger:
  every symbolic range in the seven sessions (514 lines, the session's and
  relinked images), with `SET RADIX` for the decimal offsets.
  `examine_test.go` replays `dbgdis.dlg`'s and `dbgtrc.dlg`'s
  `EXAMINE/OPERANDS` lines, ranges and lists, `SET MODE`/`SET RADIX`,
  and registers in expressions; `expr_test.go` has the evaluator's cases.
  The `exam.dlg` register lines (`EXAMINE R0` and the like) wait for
  subtask 10.
- `go build`, `go vet`, `go test ./...`, and golangci-lint on the packages
  touched are clean.

### 2026-10-05 — Subtask 10: data EXAMINE, DEPOSIT, EVALUATE, SYMBOLIZE

- **EXAMINE of data** (`internal/debugger/data.go`). Each location is a
  register (`DBGCMD\FACT\%R2:`, named in the routine the PC is in), a
  location, a range `a:b`, or, with nothing, the next location after the
  last one shown (`.` that one again, `^` the one before it, by the size
  of the last item). A location is typed by the debug symbols where a
  label is exactly at the address (`dbgsym.Program.DatumAt`: a longword by
  its DST type, a byte, a string by its descriptor length or, for
  `.ASCID`, through the descriptor in memory, an array one element a line
  under `NAME[lo:hi]`), else by the array it is an element of, else a
  longword. `/BYTE`, `/WORD`, `/LONGWORD`, `/QUADWORD`, `/ASCII[:n]`,
  `/HEXADECIMAL`, `/DECIMAL`, `/OCTAL`, `/BINARY`, `/PSL` (the field
  table, `pslTable`), and govax's `/PTE`; `/SYMBOLIC` is accepted and
  ignored. A location's name is the data symbol, an array element
  (`BUFFER[4]`), or the nearest data symbol before it and the offset
  (`WATCHL+3`), padded to the next multiple of 8 columns as the log's tab
  is (`tabPad`). The output radix (`SET RADIX/OUTPUT`) is the default.
- **Probe findings.** `/ASCII:16` is 22 characters because the count is
  read in the *input radix* (hex 16 is 22): not a VMS oddity, so nothing
  to log. A range steps by a longword (or the size typed), however big
  each item is (`EXAMINE 200:20C` shows WATCHL, WATCHB, BUFFER[0], and
  BUFFER[4]). The nearest-data rule isn't bounded by program section
  (`SYMBOLIZE NOLAB2-4` is `DBGDIS\TEXT+1A`, though NOLAB2's psect
  starts after TEXT's).
- **EVALUATE[/ADDRESS][/radix]**: the expression evaluator has a *Value*
  mode (`Evaluator.Value`, `Console.EvalWholeMode`): a data label is its
  contents sized by its type (`EVALUATE WATCHL`), `.R2` dereferences the
  register (EXAMINE's `.SP` is the location SP holds, as before), and
  `NAME[n]` is an array element, in both modes (`DebugData`,
  `imageNames.DataSize/Element`). Operators added for it: `MOD`, `@`
  (shift, as MACRO's), `NOT`, `AND`/`OR`/`XOR`, and `EQL`/`NEQ`/`LSS`/
  `LEQ`/`GTR`/`GEQ`. Hex output with a leading letter gets a `0`.
- **DEPOSIT[/type] location = value**: a number in the input radix sized
  by `/type`, else the label's type, else a longword; `/ASCII[:n]` stores
  a quoted string (cut or blank-padded to n); a register takes a longword.
- **SYMBOLIZE address** (`dbgsym.Program.SymbolizeNames`): the module
  names (the symbol, or the routine or nearest data symbol plus an offset;
  then the line, in the module's scope for a CALLS routine's entry mask),
  then `(global)` and the GST's name for an image that has one.
- **Unconfirmed, govax's choices:** `EXAMINE` with nothing and `^` before
  any EXAMINE start from the deposit address; a register named outside a
  routine is a bare `%R2`; a float or octaword label shows as raw hex of
  its size; `/ASCII` of an array shows its element count of characters;
  `EVALUATE` of `WATCHL+1` is the *address* WATCHL plus 1 (only a bare
  label, or one with a subscript, is contents); SYMBOLIZE of an address
  in no image prints only its heading; subscripts are decimal;
  `EXAMINE/PTE` of a register.
- **Left for later:** `NOSYMBOL` messages for an undefined name (the
  probe's `%DEBUG-E-NOSYMBOL`; govax says `CLI-E-UNDEFSYM`), and
  `EXAMINE/INSTRUCTION`'s numeric layout under `SET MODE NOSYMBOLIC`
  (`00000485:       MULL2    R2,R0`, tab-padded, where the console's own
  layout is still used): subtask 11's `SET MODE` and the final cleanup.
- **Tests.** `TestExamineDataOracle` replays `exam.dbg` and compares every
  EXAMINE and EVALUATE the log shows (those that depend on the stack
  addresses, the user-mode PSL, or `/INSTRUCTION` are left out);
  `TestSymbolizeOracle` replays the SYMBOLIZE and `EVALUATE/ADDRESS` lines
  of `dbgdis.dlg` and `dbgtrc.dlg`; `data_test.go` has the PSL table,
  EVALUATE's radix and operators and errors, DEPOSIT, radix qualifiers,
  text, and error cases.
- `go build`, `go vet`, `go test ./...`, and golangci-lint on the packages
  touched are clean. (`TestCtrlCReturnsToPrompt` failed once under the
  whole suite's load and passed on every rerun; it is timing-sensitive.)

### 2026-10-05 — Subtask 11: CPU and kernel state commands move

The debugger has the SHOW, SET, and CANCEL keywords the tables assign it.
The console's own copies stay until subtask 14 removes them, so both
grammars answer for now.

- **SHOW** (`internal/debugger/machine.go`, `debug.dcl`): REGISTERS/REG,
  every register and privileged-register keyword (`SHOW R0`, `SHOW IPL`),
  PSL, CPU_STATUS, CLOCK, BASE, MEMORY/VM, MAPS, TB, REGIONS, PAGE/PTE, SCB,
  SHIM, EXCEPTIONS/FAULTS, and the stack dumps `SHOW KSP|ESP|SSP|USP|ISP`.
  Each calls the console's `Show*` method, so the output is unchanged.
  Numbers they take (a count, an address) are read in the debugger's
  input radix. `SHOW SP` is now the current stack's dump (Decision 6),
  not the SP register's value; `SHOW R14` still shows it.
- **SHOW CALLS [n]** is the console's VMS-format table (Phase 41), with
  `%DEBUG-E-NOCALLS, no active call frames` (new `DBG_NOCALLS`, from
  `errors.dlg`) once the image has exited or there is no frame.
  **Still missing:** the `----- above routine called from DEBUG CALL
  command` and `above condition handler called with exception` lines
  (`call.dlg`, `except.dlg`); they need the frame walk to know a frame's
  caller kind, which comes with subtask 12's program knowledge.
- **SHOW STACK [n]** (`stack.go`) follows Decision 6: VMS's per-frame
  description (`stack frame 0 (addr)`, handler, SPA, S, mask, PSW, saved
  AP/FP/PC/registers, argument list), laid out from `exam.dlg`. The
  frame's saved PC is named with the debugger's location text.
  **Unconfirmed:** the layout of an argument list with more than one
  argument (the continuation lines are indented to the value column);
  `PSW:` in a radix other than hexadecimal; a zero-argument list; a saved
  PC that is govax's console sentinel prints `<console>` (VMS's last
  frame is its own debugger's `SHARE$DEBUG+0AD`); a frame whose AP isn't
  a plausible argument list shows none. The walk stops where the chain
  doesn't climb.
- **SET MODE / CANCEL MODE / SHOW MODE** (`modes.go`, replacing subtask
  9's `setMode`). One keyword table, each word a unique abbreviation
  (`S` is `%CLI-E-AMBIGUOUS`): `[NO]SYMBOLIC`, `[NO]LINE`, `D_FLOAT`,
  `G_FLOAT`, `[NO]OPERANDS[=FULL|BRIEF]`, `[NO]SCROLL`, `[NO]DYNAMIC`,
  `[NO]INTERRUPT`, and govax's access modes `KERNEL`, `EXECUTIVE`,
  `SUPERVISOR`, `USER`, `ISP`. The VMS screen-mode words (`[NO]SCREEN`,
  `[NO]KEYPAD`, `[NO]SEPARATE`) are known, for the ambiguity rule, but
  refused with `%DEBUG-E-SYNTAX`. `SHOW MODE` prints the modes line and
  the two radix lines exactly as `exam.dlg` does, then govax's
  `access mode: KERNEL`. `CANCEL MODE` restores the start-up modes
  (`SYMBOLIC` per `vax.disassemble.symbolic`); it leaves the radixes and
  the access mode.
  - **Decision (Decision 4's one collision):** `INTERRUPT` is VMS's
    display mode, so govax's switch to the interrupt stack, which used
    that word in the console, is `SET MODE ISP` here.
  - **Not acted on yet:** `NOLINE` (VMS then names `FACT+1A` in place of
    `%LINE n` and steps by instruction), `G_FLOAT`, `NOSCROLL`,
    `NODYNAMIC`, and `NOINTERRUPT` are recorded and shown, but change no
    output. No govax output shows floating values or scrolls. Left for
    the final cleanup alongside `SET MODE NOSYMBOLIC`'s numeric layout.
- **SHOW RADIX / SET RADIX / CANCEL RADIX**: `SHOW RADIX` prints VMS's
  `input radix : hexadecimal` and `output radix: hexadecimal`.
- **SET** PSL, PTE/PAGE, FAULT or HISTORY, VM/MAPEN (and `NOVM`), BASE,
  with their values read in the debugger's input radix. `SET PTE addr TO
  addr field=value[,...]` splits the address at the first blank outside
  parentheses, so `SET PTE (a + 1) TO ...` needs the parentheses.
- **CANCEL** INTERRUPT [/ALL] [n], TB, MEMORY/STATISTICS, and `CLEAR` as a
  synonym for the verb (Decision 5). `CANCEL MEMORY` without
  `/STATISTICS` is `%DEBUG-E-SYNTAX` (the console's `CLEAR MEMORY`, which
  zeroes memory, stays console-only).
- **Not moved in this subtask:** SHOW TRACE, WATCH (subtask 13), SHOW
  IMAGE, MODULE, SYMBOL (subtask 12). Register assignment
  (`SET R0=5`) is `DEPOSIT R0 = 5`, which subtask 10 has.
- **Tests** (`machine_test.go`): `TestMachineStateOracle` replays
  `exam.dbg` and compares every `SHOW MODE`, `SHOW RADIX`, `SHOW CALLS`,
  and `SHOW STACK` the log has (the stack addresses masked, the
  last frame, VMS's own debugger, left out); `TestShowStackLayout` checks
  the blank lines the log reader can't see; plus SET/CANCEL MODE and the
  abbreviation rule, the access modes, the moved SHOW/SET/CANCEL commands
  (with decimal input radix), and NOCALLS after the image exits.
- `go build`, `go vet`, `go test ./...`, and golangci-lint on the packages
  touched are clean.
