# Phase 50 — DCL command procedures

**Status:** in progress (started 2026-10-09). The first round
(subtasks 1 to 7) is done (2026-10-09). It gives the console DCL's
command levels: `@file` with
parameters and `/OUTPUT`, a local symbol table per level, VMS's default
error action, and EXIT that ends a procedure. Later rounds (subtasks 8
onward) make the console more fully a DCL command interpreter. Subtasks
8 and 9, symbol substitution and expressions, are done and checked
against VMS 7.3 (2026-10-09), and so is subtask 10, `$STATUS`, EXIT's
status, ON, and SET [NO]ON (2026-10-09). Subtasks 11 to 13, labels and
GOTO, IF/THEN/ELSE/ENDIF, GOSUB/RETURN, and CALL/SUBROUTINE, are done
(2026-10-09, not yet probed); the other lexical functions are next.

The console should act like a VMS DCL command processor. Until now it
had INCLUDE (and `@` as its alias), which runs a file of console
commands as though each line had been typed: no parameters, no scope for
local symbols, errors reported and passed over. That was eVAX's version
of the idea, and this phase replaces it with DCL's. INCLUDE is removed;
`@` is the way to run a command procedure.

## Goal

- **`@filespec[/OUTPUT=filespec] [p1 [p2 ... p8]]`**, as the OpenVMS
  User's Manual (7.3) describes it, with the file on the host or on a
  mounted ODS-2 volume (`rms.Session.Locate`'s rules).
- **Command levels.** The terminal is command level 0, and each `@`
  pushes a level onto an input stack, with its own source of lines and
  its own local symbol table. The level is popped when its input ends,
  or on EXIT, and its local symbols are discarded. This makes `=` and
  `:=` (local) different from `==` and `:==` (global) in a way that
  matters.
- **An input stack designed for what comes next.** A level's source
  can be repositioned: rewound, searched for a label, and returned to a
  remembered place. Labels, GOTO, GOSUB/RETURN, block IF, and CALL all
  need that, though none of them is part of this round.

## What earlier phases leave in place

- `internal/console/dclsym.go` (Phase 34): `dclSymbolTable`, a global
  table and a stack of local tables (`levels`), searched from the
  current level outward, then the global table (the User's Manual's
  12.10.3 search order). Only level 0 ever existed.
- `Console.Include` (`misc.go`): reads a file through `c.Paths` (the
  configured search path, then bootdata's embedded files) and dispatches
  each line, skipping blank lines and those starting with `;` or `!`.
  `cmd/govax` runs `vax.init` with it, and the INCLUDE verb (`@` was an
  alias) and the debugger's `@` call it too.
- INCLUDE/COMMAND_LINE: `vax.init`'s last line, which runs the one-shot
  command left on govax's command line (`IncludeCommandLine`).
- `Dispatcher.Dispatch` routes a line to the debugger while a session is
  active, so a command file can mix the two grammars (`TestCommandFile*`).

## The rules (the User's Manual, 7.3)

What OpenVMS User's Manual chapters 12 to 14 say, with section numbers:

- **Default file type** `.COM` for `@` (13.1.1).
- **Command lines start with `$`** (13.1.3). Lines without one are
  data for the image that is running. DCL skips data lines no image
  reads, with %DCL-W-SKPDAT (13.8's example). A continuation line, after
  a line ending in `-`, has no `$` (13.1.2).
- **Labels** are `$ LABEL:` and go into the level's local symbol table
  (13.2). For duplicate labels, GOTO picks the nearest one that DCL has
  already processed (13.2.2).
- **`!` starts a comment**, except inside quotes (13.3).
- **Parameters** (14.2): up to eight, in P1 to P8, separated by blanks
  or tabs. Unquoted text is uppercased. Quoted text keeps its blanks and
  case, and `""` passes an empty parameter. P1 to P8 are local symbols at
  the new level, and those not given are `""` (12.10.1, and the `SHOW
  SYMBOL/LOCAL/ALL` example in 12.14). A nested procedure's P1 to P8 are
  its own (14.4).
- **`/OUTPUT=filespec`** (13.6.4): the procedure's output goes to the
  file instead of the terminal, and error messages go to both. It must
  follow the file name with no space between them. Otherwise DCL takes
  it for a parameter.
- **Command levels** (13.7): the terminal is level 0, and a procedure run
  interactively is level 1. The end of the file, or EXIT, returns to the
  level above. `EXIT status` passes a status up. STOP returns to level 0.
  Levels nest at most 32 deep (the glossary, "command level").
- **Errors** (13.8): by default DCL runs EXIT when a command returns an
  error or severe error. The procedure exits to the level above with
  `$STATUS`, its high-order digit set so the message isn't shown again.
  Success, informational, and warning statuses continue. ON, SET [NO]ON,
  and Ctrl/Y actions override this (13.9 to 13.13).
- **Symbol tables** (12.10): one local table per command level,
  discarded when the level ends. The search runs from the current level
  outward, then the global table. SET SYMBOL can hide outer levels'
  symbols (12.11.1).

## Design

### The input stack

`internal/console/procedure.go` holds the stack: `Console.levels`, a
slice of `*commandLevel`, innermost last. Level 0, the terminal, has no
entry, because `cmd/govax`'s prompt loop reads it. A procedure level
has:

- `source`, a `*procedureSource`: the file's records, read whole when
  the procedure starts, and a cursor (`next`). `Read` returns the next
  line: a command (its `$` removed, continuation lines joined, `$!`
  comments skipped) or a data line. `Position` and `Seek` save and
  restore the cursor, and `Rewind` goes back to the start. Labels, GOTO,
  GOSUB/RETURN, and block IF can all be built from these. Reading the
  file whole makes a backward GOTO a seek. DCL itself reads ahead and
  rewinds; the observable behavior is the same for files on disk.
- `exiting`: set by EXIT, so the level's loop stops after the command
  that is running now.
- `output`, the `/OUTPUT` file, if any, and `restoreOut`, the
  `Console.Out` to put back when the level ends.
- `sysCommand`: where error messages also go when output is
  redirected. This is the terminal, the `Console.Out` of level 0.

The local symbol tables stay in `dclSymbolTable.levels`, and the input
stack pushes and pops one there with each level (`push`, `pop`). The
symbol table never pushes a level by itself, so each procedure level
has exactly one symbol level, above level 0's.

The procedure runs synchronously: the `@` handler pushes a level, reads
and dispatches its commands until the end of the file, EXIT, or an
error, and pops the level. Any caller can use `@`: the prompt, another
procedure, `TIME`, a symbol's value, the debugger's `@`, and
`cmd/govax`'s `vax.init`. Each of them gets the same behavior. The stack
is explicit rather than kept in Go's call stack, for two reasons. EXIT,
SHOW SYMBOL, and (later) GOTO, `F$ENVIRONMENT("DEPTH")`, and ON need to
reach the current level. And the loop that reads a level asks the level
for its next command, so a later GOTO only needs to move that level's
cursor.

### Parsing `@`

`@` is read before the grammar, as DCL itself treats it: a leading `@`
is a command by itself (`readCommandVerb`). Its rule that `/OUTPUT`
must touch the file name, and that anything later is a parameter,
doesn't fit the grammar's qualifiers. `parseProcedureCommand` reads
the file name (quoted or not, up to a blank or `/`), then any
qualifiers that follow it directly (`/OUTPUT=file`, abbreviated to
`/O` or more), then the parameters by DCL's rules (14.2). `console.dcl`
keeps no `@` verb.

### Command lines and data lines

The author asked for VMS's rule: a command line starts with `$` as the
record's first character, and `$!` starts a comment. A record without
`$` is a data line (13.1.3). In VMS, the image the procedure runs reads
data lines as its SYS$INPUT. In govax, two things read the console's
input. One is the debugger, during a session started by `$ DEBUG` or
`$ RUN/DEBUG`. The other is the interactive assembler, after a bare
`$ ASM`. While either one is reading, data lines are dispatched to it,
which is how a VMS procedure gives the debugger its commands.
`vax.init` does that: `$ debug`, then the data lines
`go exe$initialize` and `exit`. When nothing is reading, DCL skips the
data lines, with one `%DCL-W-SKPDAT` for each run of them (13.8's
example). A command line reached while the debugger or assembler is
still reading ends its input, as a VMS image reading SYS$INPUT gets end
of file at the next `$` line. The debugger EXITs, and the assembler
takes it as `.END`, each as it does for Ctrl/Z.

The debugger's own `@` reads the VMS debugger's format instead: every
line is a command, with no `$` (`RunDebuggerProcedure`). If the file
ends the session, its remaining lines go to the console, as before.
The debugger keeps INCLUDE as another name for its `@` (the author's
choice, 2026-10-09): a debugger command file is a different thing from
a DCL procedure, and the name shows it.

Data lines for an image a procedure runs (LIB$GET_INPUT, an RMS read of
SYS$INPUT) are still a later subtask (16). Such an image reads the
terminal for now.

### The one-shot command

INCLUDE/COMMAND_LINE goes with INCLUDE. Its replacement follows VMS's
own shape. `vax.init` plays the part of a login command procedure: it
runs first, at level 1. When it ends, the command interpreter reads
level 0's input. That input is normally the terminal. With a one-shot
command it is the command's text, and govax ends when that input runs
out, as a process ends when its input does. `cmd/govax` therefore runs
the text left on its command line (`Console.RunCommandLine`) after
`vax.init` returns, and the procedure doesn't have to name it.

The author suggested another way: put the text in a logical name and
have `vax.init` run it with something like `@SYS$COMMANDARGS`. That
keeps the choice of when it runs in `vax.init`. It isn't needed, though,
since nothing in `vax.init` follows the command, and it would make every
`vax.init` carry a line DCL has no counterpart for. A `vax.init` on a
user's search path that still has the old line gets
CLI_UNRECOGNIZED on it. The default action then ends `vax.init` there,
at its last line, so the effect is the same as before.

### Symbol substitution (subtask 8)

`internal/console/dclsubst.go` follows the User's Manual's three phases
of command processing (12.12, 12.13), with the details VMS 7.3 showed in
`testdata/dcl50`:

1. `DispatchConsole` first replaces each symbol after an apostrophe
   (`substituteApostrophes`), before anything else reads the line:
   `'NAME'` outside quotes (blanks may follow the apostrophe, and the
   closing one may be left out), its value scanned again (iterative),
   and `''NAME'` inside quotes, not scanned again. A lexical function
   call can stand in for the name. An undefined symbol becomes nothing,
   and a symbol whose value names itself is EXPSYN.
2. `dispatchCommand` then checks that the command starts with a letter
   (NOCOMD), looks up the first word as a symbol (an alias or a foreign
   command; an alias's value isn't looked up again), and replaces each
   `&NAME` (after a blank or a special character, outside quotes) once.
   DCL does this after it uppercases the line, and in VMS's DEFINE the
   value was one token with its case and blanks, so for the grammar's
   commands the value goes in quoted. In an expression (`=`) it goes in
   as it is, after the text is uppercased. A `:=` assignment, `@`'s
   parameters, and a foreign command's text get no `&` substitution.
3. Expressions replace their symbols as they're evaluated.

An alias's value, and IF's THEN command, don't come back through phase
1, so their apostrophes aren't substituted a second time (12.13.4's EXEC
example). Data lines and debugger commands are never substituted. An
unknown or ambiguous verb is DCL's IVVERB or ABVERB, with the verb as
the segment (`verbError`).

### Expressions (subtask 9)

`internal/console/dclexpr.go` evaluates DCL's expressions (12.5 to
12.9): integers (with `%X`, `%O`, `%D`), quoted strings, symbols, and
lexical function calls; the operators in 12.8.5's precedence; integer
and string values with 12.8.6's typing and 12.9's conversions; longword
arithmetic that wraps. `=` and `==` use it, and IF will (subtask 12).
What the manual doesn't say came from VMS (DEVIATIONS.md's "[Phase 50]
Substitution and expression rules, settled by a probe"): division by
zero, unclosed quotes, string tokens, optional closing dots, `.NOT.`
after an operator, a stray `)`.

The messages are DCL's own CLI$_ texts and severities (all warnings),
shown as `%DCL-`, as VMS shows its command interpreter's (the CLI
facility's display name is now DCL for every message). Some have a
segment line, ` \TEXT\`: a `VMSError` made with `vmserrors.NewSegment`.

`dcllexical.go` is the lexical function call syntax and a table of
functions (`lexicalFunctions`), each with how many arguments it takes. A
name may be any prefix that names one of VMS's lexical functions
(`lexicalNames`; a call of one govax doesn't have yet is
CLI_LEXNOTIMPL). Subtask 9 gives it F$INTEGER, F$LENGTH, and F$STRING;
subtask 14 adds the rest.

### Statuses and ON (subtask 10)

`internal/console/dclstatus.go` keeps `$STATUS` and `$SEVERITY` (13.14)
as reserved global symbols, strings as VMS shows them
(`$STATUS == "%X00030001"`), set after each command line
`DispatchConsole` reads (`statusOf`). A command reports success with nil
and failure with an error, so `conditionValue` turns the error into a
VMS condition value: VMS's value for the message of the same name
(CLI_IVVERB is CLI$_IVVERB, %X00038090), with govax's severity, or
govax's own code where VMS has none, with bit 28 (STS$M_INHIB_MSG) for a
message already shown. Success is CLI$_NORMAL. SHOW SYMBOL, IF, CONTINUE,
and EXIT leave `$STATUS` alone when they succeed (13.15, and the probe
for SHOW SYMBOL); RUN gives the image's exit status (and shows its
message if it failed), and @ the procedure's.

How a procedure ends decides @'s status (`procedureExit`), as VMS 7.3
showed: at the end of the file, `$STATUS` as it is, not shown again; at
EXIT, `$STATUS` with bit 28 set; at `EXIT n`, n, its message shown at
the level above unless n is odd or has bit 28; and by the error action,
the failing status with bit 28 (13.8). The procedure's failure is the
command's own error when it has one (`statusErr`), so its message keeps
govax's text and arguments.

Each level has an `onAction`: ON's severity and command (taken once,
then the default is back, 13.9.1), SET NOON's off switch (13.10), and ON
CONTROL_Y's command (13.12). The procedure loop (`procedureLine`) looks
at each command's status after showing its message: by default an error
or severe error ends the procedure; ON's severity or worse runs ON's
command at the level, which is handled the same way. CONTROL_Y's command
is kept for subtask 18, which delivers Ctrl/Y to procedures; until then
Ctrl/Y still ends govax. What was chosen without VMS is in DEVIATIONS.md
("[Phase 50] Statuses and ON").

### Labels, IF blocks, GOSUB, and CALL (subtasks 11 to 13)

The User's Manual's rules (13.2, 14.16, 14.17) and what was chosen
without them (DEVIATIONS.md's "[Phase 50] Labels, IF blocks, GOSUB, and
CALL"). Each command level, and the terminal, has a `flowState`
(`dcllabel.go`): its labels, its open IF blocks, the lines being passed
over, and GOSUB's return places. The three files share it, which is why
the subtasks were done together.

- **Labels and GOTO** (`dcllabel.go`). `DispatchConsole` takes a label
  off the front of a line (`takeLabel`) and records where its line
  starts (`procedureSource.lineStart`) and which IF blocks are open
  there. GOTO uses the place recorded last, if those blocks are still
  open, and otherwise searches forward (`searchLabel`), following the
  THEN, ELSE, ENDIF, SUBROUTINE, and ENDSUBROUTINE lines it passes, so a
  label inside a separate block or subroutine isn't a target; the labels
  it passes are recorded. A missing label leaves the cursor at the end
  of the file, so the procedure ends after USGOTO unless an ON action
  moves it. Going to a label closes the blocks it is outside of.
- **IF** (`dclif.go`). `IF expr THEN command` runs the command when the
  expression's value is odd. `IF expr` alone opens a block, whose next
  command must be THEN (`awaitThen`). THEN and ELSE start a branch; for
  the branch that doesn't run, `skipLine` passes over the lines, before
  any substitution, counting the blocks inside by their THEN and ENDIF
  lines. This works line by line, so blocks work at the terminal as in
  a procedure. The console's old IF, on the machine's expressions, is
  gone, and so is vax.init's `IF DEFINED(...)` line (the author's
  choice: it isn't DCL).
- **GOSUB and RETURN** (`dclcall.go`). GOSUB finds its label as GOTO
  does and pushes the place after itself and the open blocks; RETURN
  [status] goes back and puts the blocks back. At most 16 deep.
- **CALL, SUBROUTINE, ENDSUBROUTINE** (`dclcall.go`). CALL finds
  `label: SUBROUTINE` as GOTO finds a label, then its ENDSUBROUTINE
  (`subroutineEnd`), and runs the lines between at a new level:
  `runLevel`, @'s own loop, with a `procedureSource` sharing the file's
  records whose `end` is the ENDSUBROUTINE. Its parameters and /OUTPUT
  are read as @'s are (`parseProcedureCommand`), and its status is @'s
  (`finishProcedure`). Running line by line, SUBROUTINE passes over the
  subroutine.

## Subtasks

1. **Survey**: the User's Manual's rules for `@`, parameters, command
   levels, symbol scope, and errors (above, "The rules").
2. **The input stack** (`procedure.go`): `commandLevel`,
   `procedureSource` (`$` lines, data lines, continuation, comments,
   cursor),
   symbol level push and pop, the 32-level limit, and
   `Console.RunProcedure`, the one loop every caller uses.
3. **`@`**: parsing, finding the file (host or volume, `.COM` by
   default, the search path for host files), P1 to P8, and
   CLI_MAXPARM for a ninth parameter.
4. **`/OUTPUT`**: the redirected output (host files written as they
   go, volume files written when the level ends), messages to the
   terminal too, `.LIS` by default, and NL: to discard.
5. **Errors and EXIT**: the default error action, EXIT ending the
   current procedure (at level 0 it still ends govax), and QUIT as its
   own verb, which always ends govax.
6. **INCLUDE removed**: the verb, INCLUDE/COMMAND_LINE (now
   `RunCommandLine`, called by `cmd/govax`), and `vax.init`'s last line.
   The debugger's `@` runs on the same stack.
7. **Docs and help**: HELP `@` (console.help), CLAUDE.md, PLAN.md,
   dclsym.go's header.

Later rounds of this phase, in a likely order:

8. **Symbol substitution**: apostrophes (`'SYM'`, `''SYM'` in quotes)
   and `&SYM`, so `@NAME 'P1'` works.
9. **Expressions**: DCL's full expression syntax for `=` and IF (the
   operators `.EQS.`, `.EQ.`, `.AND.`, ..., integers and strings,
   lexical calls), replacing `symbolExpression`'s two forms.
10. **`$STATUS` and `$SEVERITY`**, `EXIT status`, and the
    high-order-digit rule. Then **ON** (WARNING, ERROR, SEVERE_ERROR,
    CONTROL_Y) and **SET [NO]ON**, kept per level.
11. **Labels and GOTO**: labels as the level's local symbols, the
    duplicate-label rule (13.2.2), a forward search and rewind with
    `procedureSource.Seek`, and the warning and exit for a missing
    label.
12. **IF**: DCL's `IF expr THEN command`, and block
    `IF/THEN/ELSE/ENDIF`. This replaces the console's present IF, which
    evaluates the machine's expressions.
13. **GOSUB/RETURN** (a stack of return positions in the level) and
    **CALL/SUBROUTINE/ENDSUBROUTINE** (a new level in the same file).
14. **Lexical functions**: `F$ENVIRONMENT` (DEPTH, PROCEDURE, ...),
    `F$LENGTH`, `F$EXTRACT`, `F$LOCATE`, `F$ELEMENT`, `F$EDIT`,
    `F$INTEGER`, `F$STRING`, `F$TYPE`, `F$SEARCH`, `F$PARSE`,
    `F$TRNLNM`, `F$MESSAGE`, `F$TIME`, `F$VERIFY`, ...
15. **SET VERIFY**: echo each command (and data line) as it is run.
16. **SYS$INPUT from a procedure**: data lines for the image a
    procedure runs, and DECK/EOD. (SKPDAT, and data lines for the
    debugger and the interactive assembler, came with subtask 2.)
17. **INQUIRE, READ, WRITE, OPEN, CLOSE** (DCL's file I/O).
18. **STOP** returning to level 0, and Ctrl/Y within procedures.
19. **The subprocess CLI** (`subcli.go`) gets `@`, on the same stack.
20. **A VMS probe**: run a set of procedures on VMS 7.3 for the details
    the manual leaves open (see "Open questions"). Round 1,
    `testdata/dcl50`, came with subtasks 8 and 9.

## Open questions

- `/OUTPUT=` with no name part: VMS 7.3 made a file named `.LOG` (no
  name) in the default directory; govax refuses it.
- Whether qualifiers after a blank but before the first parameter are
  qualifiers or parameters. The manual says parameters (13.6.4), and so
  does govax.
- The rest of what `testdata/dcl50`'s round 1 left open (its README):
  a negative number divided by zero, `''NAME` without its closing
  apostrophe, a foreign command's `&`.
- For a round 2, on labels, IF, GOSUB, and CALL (DEVIATIONS.md's
  "[Phase 50] Labels, IF blocks, GOSUB, and CALL"): whether a GOTO's
  forward search records the labels it passes; `IF expr THEN` with
  nothing after it; a command other than THEN after a block's IF; THEN,
  ELSE, ENDIF, GOSUB, and RETURN's effect on `$STATUS`; RETURN n's
  message and the ON action; a missing CALL target; labels, GOTO, block
  IF, GOSUB, and CALL at the terminal; abbreviations of ENDIF and
  ENDSUBROUTINE.
- For a round 2, on statuses (DEVIATIONS.md's "[Phase 50] Statuses and
  ON"): EXIT with no status after a success (is bit 28 set?); `$STATUS`
  at an ON ERROR THEN GOTO label; whether ON's command is substituted
  when ON is read or when it runs (`ON ERROR THEN WRITE SYS$OUTPUT
  "''$STATUS'"`); ON CONTROL_Y and SET NOON together; SKPDAT and ON
  WARNING; `ON FOO` (IVKEYW?) and `EXIT 3 4`; where `EXIT n`'s message
  goes with `/OUTPUT`.

(Answered by `testdata/dcl50`: a ninth parameter is CLI$_DEFOVF, the
32nd level CLI$_STKOVF, both warnings; the expression and substitution
details are in DEVIATIONS.md.)

## Progress log

- 2026-10-09: Planned. The author asked for `@` with parameters and a
  log file, local symbols scoped to the input stream, and a design that
  allows labels and IF later. Decisions: `/OUTPUT` redirects as VMS's
  does (the author first named it /LOG, then asked for VMS's behavior).
  The default error action is VMS's now, not after ON exists. EXIT ends
  a procedure, and QUIT always ends govax. INCLUDE is removed, since
  the console should act like DCL. INCLUDE/COMMAND_LINE becomes
  `cmd/govax` reading the one-shot command as level 0's input after
  `vax.init`, not a logical name (see "The one-shot command"). Subtask
  1, the survey, is above.
- 2026-10-09: Subtasks 2 to 7, in one change. `internal/console/
  procedure.go` holds the input stack (`Console.levels`, one
  `commandLevel` per procedure), `procedureSource`, `@`'s parser, and
  `runProcedure`, the one loop every caller uses. `dclSymbolTable` gains
  `push`/`pop`. `@` is read before the grammar (`DispatchConsole`). The
  console's INCLUDE verb, INCLUDE/COMMAND_LINE, and `Console.Include`
  are gone: `RunProcedure`, `RunHostProcedure` (vax.init, from the host
  search path even with SET DEFAULT on a volume), and
  `RunDebuggerProcedure` replace them, and `RunCommandLine` runs the
  one-shot command after vax.init. QUIT is its own verb, and EXIT is
  `Console.Exit`. During the work the author asked for VMS's `$` rule
  (above, "Command lines and data lines"), so `vax.init` is now a DCL
  procedure (`$` lines, `$!` comments; govax's `;` comments are gone),
  and so are the tests' procedures. New messages: CLI_MAXPARM,
  CLI_MAXDEPTH, CLI_SKPDAT. HELP has an `@` topic, and EXIT and QUIT are
  described separately. Tests: `procedure_test.go` (the parser, the
  source, parameters, symbol scope, EXIT and QUIT, the error action,
  SKPDAT, /OUTPUT on the host, on a volume, and to NL:, host names, the
  depth limit), and `TestCommandFileSwitchesGrammars` and
  `TestCommandLineEndsDebuggerInput` in the debugger. The rules chosen
  without the manual are in DEVIATIONS.md ("[Phase 50] Command procedure
  rules chosen without a manual or probe"). Removed the console's unused
  `optionalAddress`, left over from GO and CALL's move to the debugger.
- 2026-10-09: Subtasks 8 and 9. Symbol substitution
  (`internal/console/dclsubst.go`: apostrophes in phase 1, `&` in phase
  2, see "Symbol substitution") and DCL's expressions
  (`dclexpr.go`, with lexical function calls and a function table in
  `dcllexical.go`), replacing `symbolExpression`'s quoted string or
  decimal integer. `DispatchConsole` substitutes, then
  `dispatchCommand` does the rest; aliases and IF's THEN re-enter there.
  The grammar gains `ParseUpcased` (and exports `UpcaseOutsideQuotes`)
  so an `&` value keeps its case. New messages, in DCL's words from
  `vmsdef.Messages`: CLI_UNDSYM, CLI_IVOPER, CLI_IVFNAM, CLI_ABFNAM,
  CLI_ARGREQ, CLI_NOPAREN, CLI_IVCHAR; CLI_EXPSYN takes DCL's text and
  becomes a warning. SHOW SYMBOL no longer doubles a quote in a value
  (the manual's 12.6.1 example). HELP: SYMBOL EXPRESSIONS, SYMBOL
  SUBSTITUTION, LEXICAL. Tests: `dclexpr_test.go` (the manual's examples
  from 12.4 to 12.9, every message), `dclsubst_test.go` (12.12 and
  12.13's examples, `&`, and through the dispatcher: parameters, the
  EXEC alias, DEFINE's case). The author offered to run probes on the
  simh VAX: `testdata/dcl50` is round 1 (substitution, expressions, the
  `@` open questions, and statuses for subtask 10). The rules chosen
  without it are in DEVIATIONS.md.
- 2026-10-09: The probe's round 1 ran on the simh VAX (the author ran it;
  `testdata/dcl50/vax/probe50.log`), and govax now does what VMS did:
  `TestProbe50Oracle` replays sections A and B against the log, and every
  case matches. Changes: the CLI facility's messages show as `%DCL-` (the
  author's choice: VMS's standard); `VMSError.Segment` and `NewSegment`
  for the ` \TEXT\` line, only on the messages VMS gives one; new
  CLI_SYMDEL, CLI_NOCOMD, CLI_DEFOVF, CLI_IVVERB, CLI_ABVERB,
  CLI_LEXNOTIMPL; CLI_MAXDEPTH became DCL's CLI_STKOVF, a warning, at 31
  levels above the terminal; CLI_SYMDEPTH is gone. The expression and
  substitution answers are listed in DEVIATIONS.md, and the statuses
  (section D) are saved for subtask 10. SHOW SYMBOL and DELETE/SYMBOL of
  an undefined symbol are CLI_UNDSYM. The grammar's `ParseUpcased` from
  the first pass is gone again (an `&` value now goes in quoted). At the
  author's request, `?` is no longer an alias of HELP, in the console or
  the debugger.
- 2026-10-09: Subtask 10. `internal/console/dclstatus.go`: `$STATUS`
  and `$SEVERITY` (global string symbols, set by `statusOf` around each
  line `DispatchConsole` reads), `conditionValue` (govax's statuses as
  VMS's condition values, by message name; `vmserrors.MessageNames` is
  now filled by `DefineMessage`), `statusError` for a status given as a
  VMS value (an image's, EXIT's), ON (`onAction`, per level; ON
  CONTROL_Y's command kept for subtask 18), SET [NO]ON, CONTINUE, and
  EXIT's status. `procedure.go`'s loop now runs each line through
  `procedureLine` (message, then the level's action) and ends with a
  `procedureExit` (see "Statuses and ON"). RUN reports its image's exit
  status (`imageCompletion`): a failure shows its message and fails RUN,
  so `govax run IMAGE` of an image that fails exits nonzero. New
  messages: CLI_NOTHEN, CLI_INSFPRM. The system facility's message 0 is
  now SS$_NORMAL's only (`vmsdef.LookupMessage`; NONAME-x-NOMSG for EXIT
  0, 2, and 4, as VMS showed). HELP: ON, CONTINUE, SET ON, SYMBOL
  STATUS, and EXIT's status. Tests: `dclstatus_test.go`;
  `TestProbe50Oracle` now compares every case's `$STATUS` with VMS's
  (all 109 match); `TestProbe50Statuses` replays section D (all but TYPE
  and @ of a missing file, whose messages are govax's). The console's
  `EX` is EXIT's abbreviation, so `EX 200` now parses there as `EXIT
  200` (TestGrammarSplit).
- 2026-10-09: Subtasks 11 to 13, together, since labels, IF blocks, and
  GOSUB share each level's `flowState` (see "Labels, IF blocks, GOSUB,
  and CALL"). New files `dcllabel.go`, `dclif.go`, `dclcall.go`;
  `procedureSource` gains `end`, `lineStart`, and `lineAt` (a read that
  doesn't move the cursor, for scans); `runProcedure` is split into
  `runLevel` (shared with CALL) and `finishProcedure`. New verbs in
  `console.dcl`: THEN, ELSE, ENDIF, GOTO, GOSUB, RETURN, CALL (now both
  the console's, DCL's, and the debugger's, TestGrammarSplit),
  SUBROUTINE, ENDSUBROUTINE; IF takes the rest of the line. New
  messages, in DCL's words: CLI_NOLBLS, CLI_USGOTO, CLI_MSNGENDS,
  CLI_INVIFNEST, CLI_INVGOSUB, CLI_GOSUBMAX, CLI_USGOSUB, CLI_BADRET,
  CLI_USCALL, CLI_INVCALL. vax.init's `IF DEFINED("CONSOLE$ARG_FILE")
  THEN SET NOVERBOSE` is gone (the author's choice: it isn't DCL; the
  symbol was never defined, so it never ran; quieting a one-shot
  command is left for later). HELP: IF/THEN/ELSE/ENDIF, GOTO (and
  labels), GOSUB/RETURN, CALL/SUBROUTINE/ENDSUBROUTINE. Tests:
  `dclflow_test.go` (the manual's examples: loops, 13.2.2's duplicate
  labels, the GOTO into a block, GOSUB.COM, the BAR subroutine; the
  label search; blocks nested, at the terminal, and with `$STATUS`;
  every message), and `TestDispatch_if` for DCL's truth rule.
