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
(2026-10-09, not yet probed), and so is subtask 14, the lexical
functions (2026-10-09, not yet probed). Subtask 15, SET VERIFY, SET
PREFIX, and F$VERIFY's echo, is done and checked against VMS 7.3
(2026-10-11), and so is subtask 16, SYS$INPUT from a procedure, with
DECK and EOD (2026-10-11, checked against VMS 7.3), and so is subtask
17, DCL's file I/O: OPEN, CLOSE, READ, WRITE, and INQUIRE (2026-10-11,
checked against VMS 7.3 in round 4 of the probe), and so is subtask
18, STOP and Ctrl/Y (2026-10-11, not yet probed), and so is subtask 19,
which, at the author's request, gives a subprocess the console's whole
DCL rather than `@` alone (2026-10-11). Subtask 20, the VMS probe of
what is still unconfirmed, is next.

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
SYS$INPUT) came with subtask 16 (below, "SYS$INPUT from a procedure"),
and with them VMS's rule for data nothing reads: skipped after an image,
read as a command after one of DCL's own commands.

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
subtask 14 adds the rest (below, "Lexical functions").

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
is subtask 18's (below, "STOP and Ctrl/Y"). What was chosen without VMS is in DEVIATIONS.md
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

### Lexical functions (subtask 14)

The User's Manual's chapter 15 describes the lexical functions by
example; the DCL Dictionary, their reference, isn't among the project's
manuals, so what the examples don't show was chosen (DEVIATIONS.md's
"[Phase 50] Lexical functions: choices without the DCL Dictionary").

- **One table.** `lexicalFunctions` (`dcllexical.go`) has an entry per
  function: its argument counts, the argument that names a symbol
  rather than giving a value (F$TYPE's, F$PID's context: read as a
  name, CLI_IVSYMB otherwise), whether it asks the console about the
  system (a subprocess's CLI, with no console, gets CLI_LEXNOTIMPL for
  those), and the Go function. Adding a function is adding an entry.
- **Keywords are tables too.** Each function's keywords (F$ENVIRONMENT's
  items, F$GETJPI's, F$GETSYI's, F$GETDVI's, F$TRNLNM's, F$PARSE's
  fields and parse types, F$EDIT's edits, F$CVTIME's formats and fields)
  are a map from the keyword to what it returns, read by one helper
  (`lexicalKeyword`): spelled in full, in any case, and CLI_IVKEYW with
  the keyword as its segment otherwise.
- **The same answers as the services.** F$GETJPI, F$GETSYI, and F$GETDVI
  read `$GETJPI`'s, `$GETSYI`'s, and `$GETDVI`'s items by name
  (`corevms/lexical.go`: `JPIItem`, `SYIItem`, `DVIItem`); F$FAO is
  `$FAO`'s formatter with DCL's values in place of addresses
  (`FormatFAOValues`); F$SETPRV is `$SETPRV`'s rule
  (`SetProcessPrivileges`); F$PARSE and F$SEARCH are RMS's name
  processing (`rms/lexical.go`: `Session.Parse`, `Session.Search`), with
  host files searched on the host by govax's host-or-volume rule.
- **Files by subject**: strings and data types (`dcllexstring.go`:
  F$CVSI, F$CVUI, F$EDIT, F$ELEMENT, F$EXTRACT, F$FAO, F$INTEGER,
  F$LENGTH, F$LOCATE, F$STRING, F$TYPE), the process and its command
  environment (`dcllexprocess.go`: F$DIRECTORY, F$ENVIRONMENT,
  F$GETJPI, F$GETSYI, F$MODE, F$PID, F$PRIVILEGE, F$PROCESS, F$SETPRV,
  F$USER, F$VERIFY), files and devices (`dcllexfile.go`: F$GETDVI,
  F$PARSE, F$SEARCH), logical names and messages (`dcllexname.go`:
  F$LOGICAL, F$MESSAGE, F$TRNLNM), and time (`dcllextime.go`: F$CVTIME,
  F$TIME, with DCL's own time syntax: `dd-mmm-yyyy:hh:mm`, TODAY,
  `dddd-hh:mm`, combinations).
- **Not yet**: F$CONTEXT, F$CSID, F$DEVICE, F$FILE_ATTRIBUTES,
  F$GETQUI, F$IDENTIFIER, and F$LICENSE stay CLI_LEXNOTIMPL. F$VERIFY
  keeps procedure and image verification (`Console.Verify`,
  `verifyImage`); echoing lines is subtask 15's (below), and so is
  F$VERIFY's being evaluated in a `$!` comment, as DCL does.

### Verification (subtask 15)

`dclverify.go`. Procedure verification (`Console.Verify`) shows each
command line of a procedure; image verification (`verifyImage`) the
data lines given to what reads the procedure's input. The rules come from
the DCL Dictionary (SET VERIFY, SET PREFIX, F$VERIFY) and the User's
Manual (13.5.5), checked by testdata/dcl50's round 2 (`probe50v.com`,
`TestProbe50VOracle`; DEVIATIONS.md's "[Phase 50] Verification").

- **When a line is shown.** The procedure loop (`runLevel`) makes the
  line it is about to run the pending echo (`Console.echo`,
  `newPendingEcho`: its records, the line they make, and where they were
  joined). `DispatchConsole` shows it after the first phase's
  substitutions (`echoLine`), or drops it: a line whose scan fails, an IF
  branch's lines that don't run (but for the ELSE or ENDIF that ends the
  passing over), the THEN of a false block IF (`thenOfFalseIf`). The
  comment and blank lines `procedureSource.Read` passes over are shown
  before the line (`comments`). CALL shows the SUBROUTINE line; a CALL
  level's end shows its ENDSUBROUTINE.
- **Continued commands, a record at a time.** VMS substitutes and shows
  each record. govax scans the joined line once (a lexical function's
  side effects happen once), with `joinMark` where records were joined
  (`markJoins`), then splits the scanned line back into records
  (`scannedRecords`): each record is shown as it is but for its part of
  the line.
- **F$VERIFY in comments** (`verifyInComment`): performed, and replaced
  by its value in what is shown; called from `substituteApostrophes` at
  a command's `!`, and from `comments` for a comment line.
- **SET VERIFY** takes `[NO]PROCEDURE` and `[NO]IMAGE`, abbreviated;
  SET NOVERIFY with keywords is CLI$_CONFLICT, its segment the word as
  typed (the handler reads it from `Dispatcher.line`). **SET PREFIX**'s
  control string is checked (`validPrefix`: only !%T and !%D take an
  argument, 0, the current time) and formatted by `$FAO`'s code for each
  line shown.
- **SET name=value removed.** The console's old SET form that assigned a
  machine symbol or a register (with /PERMANENT, /ENTRY, /LABEL) is gone:
  DCL's `=`, `==`, and `:=` assign symbols, and the debugger's DEPOSIT
  sets general registers. (Nothing now sets a privileged register or the
  whole PSL from a command line; the debugger's SET PSL sets its
  fields.)

### SYS$INPUT from a procedure (subtask 16)

`internal/console/dclinput.go`. In VMS, an image a procedure runs reads
the procedure as its SYS$INPUT: the records after its command, up to the
next `$` record, its end of file (User's Manual, 13.1.3, 14.1). DECK and
EOD come from the DCL Dictionary. What VMS 7.3 showed is testdata/dcl50's
section V6; what was chosen is in DEVIATIONS.md ("[Phase 50] SYS$INPUT
from a procedure").

- **The image pulls its lines.** govax runs an image to its end inside
  the command that runs it, so the procedure can't push lines to it as
  it does to the debugger. Each line of a procedure is dispatched with
  process 1's input redirected (`withProcedureInput`;
  `corevms.Environment.RedirectInput`) to a `procedureInput`, a reader
  that takes the level's next data record (`procedureSource.readData`)
  each time the image wants more, through a buffer of its own. Every way
  an image reads its input goes through it: a terminal `$QIO` read,
  RMS's `$GET`, LIB$GET_INPUT, the input shims. What the image doesn't
  read stays in the procedure.
- **TYPE SYS$INPUT** (`typeInput`) reads the same way, a line at a time
  (`Environment.ReadLine`): the data lines in a procedure, the terminal
  up to Ctrl/Z at level 0. A logical name that translates to SYS$INPUT,
  through as many names as RMS would follow, is the input too
  (`isInputStream`); a program's RMS open of one already reached it,
  through the translation to the terminal.
- **Image verification** shows each record as it's read (`showData`),
  before the image uses it, so TYPE SYS$INPUT's lines come out twice,
  shown and typed, as on VMS.
- **DECK and EOD.** A `$ DECK[/DOLLARS[=string]]` record read by an
  image starts a deck (`procedureSource.deck`): every record is data up
  to its end of file indicator, `$ EOD` or a record starting with the
  string, which is used up. An EOD record outside a deck is the image's
  end of file too, and is used up, so a second read gets the data after
  it. DECK and indicator records aren't shown by verification. DECK read
  by DCL as a command (a verb in `console.dcl`) is CLI$_IVVALU and
  starts no deck; EOD read by DCL does nothing. A deck an image started
  and left unfinished goes on as data lines (`procedureSource.Read`).
- **Data nothing reads.** After an image, DCL skips the data up to the
  next `$` record, a lone `$` or a comment included, with one SKPDAT;
  after one of its own commands, it reads the next record as a command,
  `$` or not (V6: `%DCL-W-IVVERB \DATA\` after SET VERIFY; round 3
  checked 28 commands). `dispatchParsed` records
  whether the command it ran was an image (`Console.imageRan`), by the
  grammar's syntax name and a table of DCL's own commands
  (`dclOwnCommands`); a foreign command is an image, and an assignment,
  @, and CALL are DCL's.

### DCL's file I/O (subtask 17)

`internal/console/dclfile.go`, and `internal/rms/dclfile.go` for the
files. The rules are the DCL Dictionary's (OPEN, CLOSE, READ, WRITE,
INQUIRE); what VMS 7.3 showed in testdata/dcl50's round 4, and what was
chosen without it, is in DEVIATIONS.md ("[Phase 50] DCL's file I/O").

- **The files are RMS's.** `rms.Session.OpenDCLFile` opens a volume file
  by `$OPEN`'s and `$CREATE`'s own code, called from Go: name processing
  (the default type .DAT, logical names, search lists), the file's RMS
  openers and the file system's access (sharing.go), so DCL's files
  conflict with a program's opens of the same file as they would on VMS
  (RMS$_FLK), and DISMOUNT sees them open. The record stream is a
  `FileHandle`'s (stream.go): READ a sequential `$GET`, WRITE a `$PUT`
  (RMS$_NEF away from the end), WRITE/UPDATE an `$UPDATE`; an open that
  writes may read and update too. Without /SHARE the sharing is RMS's
  default: a writer keeps everyone out (VMS 7.3's F9). A file
  OPEN/WRITE makes is VFC, two bytes of fixed control (01 8D), RAT=PRN.
  A host file is lines of text, with the same rules.
- **Process-permanent.** `Console.dclFiles` holds the open files by
  logical name, which OPEN defines in the process table, supervisor
  mode, CONFINE, equated to the node's physical device name
  (`_GOVAX$DUA0`, TERMINAL). The name is the way to the file
  (`openFile`): once it is deassigned the file is out of reach
  (`dclOrphans`), and TYPE of it types the file from READ's place
  (`typeOpenFile`). A file outlives the procedure that opened it;
  `EndSession` closes what is left. SYS$INPUT, SYS$OUTPUT,
  SYS$ERROR, and SYS$COMMAND need no OPEN, by name or through logical
  names (`processPermanentFile`, which TYPE's `isInputStream` now uses):
  READ SYS$INPUT takes a procedure's next record, whatever it is
  (`procedureSource.readRecord`; round 3's I6), and READ SYS$COMMAND
  (and the others, and SYS$INPUT at the terminal) reads the terminal
  (`readTerminal`, through `readReply`, so a procedure's data doesn't
  stand in for it), prompting with /PROMPT's string or "Data:" and a
  tab. WRITE SYS$ERROR and SYS$COMMAND go to the terminal.
- **Statuses.** Success is %X00000001 for OPEN and INQUIRE, CLI$_NORMAL
  for READ, RMS$_NORMAL for WRITE and CLOSE. A failure is RMS's status:
  OPEN's shown as @'s was (`%DCL-E-OPENIN, error opening NAME as input`,
  OPENOUT for a new file, then RMS's message), with bit 28 set in
  `$STATUS`; READ's and WRITE's RMS's message alone, without bit 28.
  /ERROR and /END_OF_FILE go to their label instead (`Console.branch`):
  Goto, then `branchStatus`, which sets `$STATUS` and marks the status
  handled, so `procedureLine` takes no ON action (the manual: the
  qualifier "overrides any ON condition action"), and leaves the
  procedure's own status CLI$_NORMAL (`endStatus`, what EXIT with no
  status and the end of file use: VMS 7.3's F7). A name OPEN didn't
  define is CLI$_UNDFIL.
- **INQUIRE** prompts at the terminal (`terminalOut`: the terminal even
  under /OUTPUT) with its prompt, or the symbol's name, and ": "; the
  reply is substituted (`'X'`) and read as a `:=` value is (`dclText`),
  and shown on the procedure's output with verification on.
- **Fixed in passing**: `rms.writeRecords` expanded every VFC file's
  fixed control as print control; it is carriage control only with
  RAT=PRN, so a RAT=CR file is a line per record (and a PRN record with
  OPEN's 01 8D is a line too). TYPE is one of a file's RMS openers
  (RMS$_FLK for a file kept from readers). `readReply` takes Ctrl/Z as
  the end of the input. SHOW LOGICAL/FULL shows a name's own attributes
  (`[super,confine]`), and DEASSIGN leaves SS$_NORMAL.

### STOP and Ctrl/Y (subtask 18)

`internal/console/dclcontroly.go`. The rules are the User's Manual's
(13.7.1, 13.8.1, 13.11 to 13.13) and the DCL Dictionary's (STOP,
CONTINUE, SET CONTROL); what was chosen is in DEVIATIONS.md ("[Phase
50] STOP and Ctrl/Y").

- **STOP** with no process sets `Console.stopping`: each level's loop
  ends after its line (`procedureLine`), its @ with SS$_ABORT (bit 28
  set), and the levels outside it end the same way, taking no ON
  action; `runLevel` turns it off when the last level has gone. At the
  terminal, STOP runs down an image Ctrl/C or Ctrl/Y left stopped. With
  a process name or /IDENTIFICATION it is Phase 45's `StopProcess`.
- **Whose key it is.** An attention key typed while an image runs is
  offered first to the process's Ctrl/C or Ctrl/Y AST, as before;
  otherwise, in a DCL procedure, it is DCL's (`HandleAttention`, now in
  this file, on the engine's Step loop). One typed while DCL runs a
  command of its own stays pending in the engine, and the procedure
  loop takes it after the command (`takeControlY`). Either way the key
  is then forgotten (`Engine.ClearAttention`), so cmd/govax doesn't end
  govax for it. Ctrl/C counts as Ctrl/Y in a procedure.
- **The action** (`controlYAction`): ON CONTROL_Y's command, run as
  ON's others are (the image stopped and run down first); or, below
  level 1, the level's end with SS$_CONTROLY; or, at level 1, the
  Ctrl/Y level (`controlYLevel`): lines read from the terminal at the
  console's prompt, dispatched in the procedure's context. During an
  image, the Ctrl/Y level is read inside `HandleAttention`, with the
  image paused, so CONTINUE lets it go on; EXIT, STOP, and a command
  that would run an image (`leaveControlY`, by `dclOwnCommands`) stop
  it. Such a command runs at level 0 once the procedures are gone
  (`Console.afterStop`, run by `finishProcedure`). The image's
  `%VAX-I-ATTENTION` isn't shown.
- **SET [NO]CONTROL[=(T,Y)]**: Ctrl/Y's switch is atomic, since
  cmd/govax's key handling asks it (`ControlYEnabled`); with it off,
  Ctrl/Y only echoes, and an attention key no AST takes is ignored.
  F$ENVIRONMENT's CONTROL and NOCONTROL report the settings.

### The subprocess's DCL (subtask 19)

`internal/console/subcli.go`. The author asked for more than `@` in the
small CLI Phase 48 gave subprocesses: "a fully-functional version of the
DCL command processor", so that `SPAWN DIRECTORY` runs DIRECTORY and
logs out, and a spawned procedure can use every command the console has.
So the small CLI is gone, and a subprocess runs the console's own DCL.

- **Two goroutines taking turns.** The console's DCL is synchronous Go:
  a procedure is a loop (`runLevel`) that runs each line to its end. A
  subprocess's images, though, must run on the engine, which is also
  running the other processes. So each subprocess's interpreter runs on
  a goroutine of its own (`runInterpreter`), which takes turns with the
  engine through the CLI stub's shim: the shim hands it the CPU
  (`subprocessCLI.run`), and it hands it back (`handBack`) with a
  `cliYield`: wait (its next command line isn't there yet; the shim
  returns ErrWait, and the process waits in the scheduler), call an
  image (RUN, MCR, a foreign command: `subprocessRun`; the shim returns
  the `CallRequest`, and `endCLIImage` puts the CLI back when the image
  ends, with its status for the command), or log out. Only one of the
  two runs at a time, so nothing is shared at once; the race detector
  agrees.
- **The DCL state is the process's.** While the interpreter runs, the
  console's per-process DCL fields are the subprocess's
  (`swapDCLState`, exchanging them with a `dclState`): its environment
  (RTL), logical names, RMS session (its default directory), output
  (`cliOutput`, its SYS$OUTPUT a line at a time), symbols, command
  levels, $STATUS, verification, OPEN's files, and the rest. Process 1
  is `Console.processOne` throughout (its images, `InheritSymbols`), and
  the terminal is always process 1's Out (`consoleOutput`). A
  procedure's data lines reach the subprocess's image as process 1's do
  (`procedureInput`, now with the subprocess's verification).
- **Process 1's commands.** Those that work the machine itself or
  process 1's own session (`processOneCommands`: DEBUG, ASM, INITIALIZE/
  VAX, VMINIT, ZERO, SAVE, LOAD, BOOT, ROM, CLEAR, SET QUANTUM, SPAWN,
  ...) and the console's CALL and GO are `%VAX-E-NOTSUBPROC` in a
  subprocess. SPAWN is among them until a subprocess's DCL can wait for
  its own subprocess.
- **Logging out.** LOGOUT (now a console verb; QUIT in process 1),
  EXIT at command level 0 (its status is $STATUS, the final status), or
  the end of the input. A deleted process's interpreter, waiting for its
  turn, is unwound (`abort`, a panic its goroutine recovers) with its
  state in place, so its levels and files are its own to leave.
- **What changed for the old CLI's users.** An alias's value is no
  longer looked up as a symbol again (VMS 7.3 doesn't, round 1 of the
  probe); SHOW LOGICAL's status is SS$_NORMAL or SHOW$_NOTRAN with bit
  28, in process 1 too (VMS's SHOW image, probes 4 and 5); a job that
  isn't interactive has SET VERIFY on, so its procedures' lines show too.
  MCR is a console verb now.
- **Parallel only while the machine runs.** A SPAWN/NOWAIT subprocess
  runs while the engine does (Phase 43's Decision 4): during process 1's
  RUN, GO, a waiting SPAWN, and the like, not while process 1's DCL runs
  its own commands or sits at the prompt. Running the other processes
  between process 1's commands, and at the prompt, is left for later.

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
    debugger and the interactive assembler, came with subtask 2.) Image
    verification of those lines comes with it: VMS 7.3 shows each line
    TYPE SYS$INPUT reads, and read a data line after SET VERIFY (not an
    image) as a command, `%DCL-W-IVVERB \DATA\`, not SKPDAT
    (testdata/dcl50's round 2, section V6).
17. **INQUIRE, READ, WRITE, OPEN, CLOSE** (DCL's file I/O; done, see
    "DCL's file I/O"). Round 3 of
    the probe (DEVIATIONS.md's "[Phase 50] SYS$INPUT from a procedure")
    showed READ SYS$INPUT taking the next record whatever it is (a `$`
    line or EOD too), and WRITE leaving `$STATUS` at RMS$_NORMAL; WAIT
    and SET SYMBOL are DCL's own commands (`dclOwnCommands`). Its I6 and
    I9 sections wait for READ and WRITE (`TestProbe50IOracle`).
18. **STOP** returning to level 0, and Ctrl/Y within procedures (done,
    see "STOP and Ctrl/Y").
19. **The subprocess CLI** (`subcli.go`) gets `@`, on the same stack
    (done, and more: the console's whole DCL; see "The subprocess's
    DCL").
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
- For a later round, on labels, IF, GOSUB, and CALL (DEVIATIONS.md's
  "[Phase 50] Labels, IF blocks, GOSUB, and CALL"): whether a GOTO's
  forward search records the labels it passes; `IF expr THEN` with
  nothing after it; a command other than THEN after a block's IF; THEN,
  ELSE, ENDIF, GOSUB, and RETURN's effect on `$STATUS`; RETURN n's
  message and the ON action; a missing CALL target; labels, GOTO, block
  IF, GOSUB, and CALL at the terminal; abbreviations of ENDIF and
  ENDSUBROUTINE.
- For a later round, on the lexical functions: everything DEVIATIONS.md's
  "[Phase 50] Lexical functions: choices without the DCL Dictionary"
  lists, chiefly F$CVTIME's formats and defaults, F$SEARCH without
  wildcards, keyword abbreviations, and the range errors.
- For a later round, on statuses (DEVIATIONS.md's "[Phase 50] Statuses and
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
- 2026-10-09: Subtask 14, the lexical functions (see "Lexical
  functions"). The author asked that they stay table-driven, so each
  function is an entry in `lexicalFunctions` and each function's
  keywords a table of their own. New files `dcllexstring.go`,
  `dcllexprocess.go`, `dcllexfile.go`, `dcllexname.go`,
  `dcllextime.go`; `corevms/lexical.go` (items by name, privilege names,
  `FormatFAOValues`, `FormatTime`; `$SETPRV`'s change is now
  `setPrivileges`, shared with F$SETPRV; the FAO formatter gains its DCL
  mode) and `rms/lexical.go` (`Parse`, `Search`, `DefaultDirectory`).
  `dclstatus.go`'s `messageText` is the message file's text, which
  F$MESSAGE returns. New messages, DCL's CLI$_ texts: CLI_IVKEYW,
  CLI_IVSYMB, CLI_INVRANGE, CLI_IVATIME, CLI_IVDTIME, CLI_IVVALU. HELP
  LEXICAL lists the functions. Tests: `dcllexical_test.go` (chapter 15's
  examples, every function and its errors, F$PARSE and F$SEARCH on the
  host and on a volume, a fixed clock for the time functions, the
  subprocess CLI's subset).
- 2026-10-11: Subtask 15, verification (see "Verification"). SET
  [NO]VERIFY, SET [NO]PREFIX, F$ENVIRONMENT("VERIFY_PREFIX"), the echo of
  a procedure's lines, and F$VERIFY in comments. A VMS probe, round 2
  of testdata/dcl50 (`probe50v.com`, `probe50w.com`, `vax/probe50v.log`),
  changed govax: continued commands shown a record at a time, the THEN
  of a false IF hidden, CALL's SUBROUTINE line shown, a comment's
  F$VERIFY replaced by its value (closing apostrophe optional), SET
  VERIFY's keywords abbreviated, SET NOVERIFY=keyword CLI$_CONFLICT, SET
  PREFIX's CLI$_INVPREFIX, and F$VERIFY(,n) changing nothing; new
  messages CLI_CONFLICT and CLI_INVPREFIX. At the author's request, SET
  name=value (the console's old machine-symbol and register
  assignment, and its qualifiers) is removed: DCL's assignments replace
  it. Tests: `dclverify_test.go`, `TestProbe50Verify` (round 1's log)
  and `TestProbe50VOracle` (round 2's, all but V6, which waits for
  subtask 16).
- 2026-10-11: Subtask 16, SYS$INPUT from a procedure (see "SYS$INPUT
  from a procedure"). New file `dclinput.go`: `procedureInput`, the
  reader an image's input is redirected to while a procedure's line runs
  (`corevms.Environment.RedirectInput`, and `ReadLine` for the console's
  own reads); `procedureSource.readData`; DECK and EOD (new verbs in
  `console.dcl`, and a deck in `procedureSource.Read`); TYPE SYS$INPUT;
  and `dclOwnCommands`, which decides whether a data line nothing reads
  is skipped (after an image) or read as a command (after one of DCL's
  own). HELP: DECK/EOD, and TYPE SYS$INPUT, @, and SET VERIFY updated.
  Tests: `dclinput_test.go` (TYPE SYS$INPUT, EOD, DECK and /DOLLARS,
  DECK as a command, data after DCL's commands and after an image, image
  verification, nested procedures, the terminal, `parseDeck`),
  `procinput_test.go` (an image reading with LIB$GET_INPUT: all its data,
  then one line of three, the rest SKPDAT); `TestProbe50VOracle` now
  compares every section of round 2, V6 and V7's TYPE SYS$INPUT
  included, and all match.
- 2026-10-11: At the author's suggestion, TYPE follows logical names to
  SYS$INPUT, as RMS translates a file name iteratively (`isInputStream`;
  `TestIsInputStream`). Round 3 of the VMS probe, `testdata/dcl50`'s
  `probe50i.com` (with `probe50j.com` and `probe50k.com`), asks about
  each of subtask 16's unconfirmed choices, and READ SYS$INPUT for
  subtask 17; the author is to run it.
- 2026-10-11: Round 3 ran on the simh VAX (the author ran it;
  `testdata/dcl50/vax/probe50i.log`), and `TestProbe50IOracle` replays
  it: sections I2, I3, I4, I5, I7, and I8 line for line (WRITE, which
  govax doesn't have yet, emulated from its IVVERB), and in I1 what
  became of each of 28 data lines. VMS agreed with govax on which
  commands are images, a procedure's first record, DECK/DOLLARS's
  string, TYPE through logical names, `$` after blanks, and verification.
  Changes: DECK read by DCL is CLI$_IVVALU (it had started a deck); a
  lone `$` (and, unconfirmed, a comment) ends the skipping of an image's
  data; a line starting with `%` is IVVERB, not NOCOMD. READ's and
  WRITE's answers are kept for subtask 17.
- 2026-10-11: At the author's request, CREATE makes a file from
  SYS$INPUT (`createfile.go`): the terminal up to Ctrl/Z, or a
  procedure's data lines. CREATE's bare form in `console.dcl` takes a
  FILE parameter (with COPY's parameter-scoped /HOST) and /LOG
  (CREATE-I-CREATED with the version; CREATE-E-OPENOUT for a file that
  can't be written). A volume file is RFM=VAR, RAT=CR
  (`rms.TextRecords`); a host file is text whose lines end as the
  host's do (`rms.hostLineEnd`: CR LF on Windows, LF elsewhere, now for
  every host text file `CreateRecordFile` writes). Where the file goes
  is decided before input is read. CREATE counts as an image for data
  nothing reads, as VMS's CREATE is one. Unconfirmed against VMS: the
  OPENOUT wording, and that no other qualifier is needed. HELP CREATE
  describes the command and names /DIRECTORY and /NAME_TABLE as its
  other forms. Tests: `createfile_test.go`.
- 2026-10-11: Subtask 17, DCL's file I/O (see "DCL's file I/O"). New
  files `internal/console/dclfile.go` (OPEN, CLOSE, READ, WRITE, INQUIRE;
  new verbs in `console.dcl`) and `internal/rms/dclfile.go`
  (`Session.OpenDCLFile` and `DCLFile`: volume files through `$OPEN`'s
  and `$CREATE`'s code and the stream context, host files as lines).
  `dclstatus.go` gains `branchStatus`, and `procedureLine` takes no ON
  action for a status a branch handled. `isInputStream` is now
  `processPermanentFile`'s SYS$INPUT case. New message CLI_UNDFIL. The
  grammar's /SHARE is present only when typed (`Defaulted`: a qualifier
  with a /default is otherwise present). TYPE of a VFC file with RAT=CR
  no longer treats its fixed control as print control (`rms.writeRecords`,
  `TestSession_typeVFCCarriageReturn`). HELP: OPEN, CLOSE, READ, WRITE,
  INQUIRE. Tests: `dclfile_test.go` (host and volume round trips,
  versions, /APPEND, /READ/WRITE and /UPDATE, every failure's message and
  status, the branches and ON, the process-permanent files, INQUIRE,
  files outliving their procedure, a name opened twice, sharing, data
  after OPEN, `splitDCLList`), `rms/dclfile_test.go` (the new file's
  attributes and fixed control, the failures' statuses, host files);
  `TestProbe50IOracle` now compares I6 (READ SYS$INPUT) and I9 too, and
  every section but I1 matches line for line, WRITE no longer emulated.
  Round 4 of the probe (`testdata/dcl50`'s `probe50f.com` and
  `probe50g.com`, and the interactive `probe50q.com` and `probe50p.com`)
  asks about each choice; the author is to run it.
- 2026-10-11: Round 4 of the probe ran on the simh VAX (the author ran
  it: `testdata/dcl50/vax/probe50f.log`, `probe50q.log`, and
  `probe50p.log`), and govax now does what VMS did. `TestProbe50FOracle`
  replays `probe50f.com` with /OUTPUT and matches every section (but for
  DELETE's, DIRECTORY's, and DUMP's layouts and SHOW LOGICAL's extra
  blank before a process-permanent name), and `TestProbe50QOracle`
  replays the interactive `probe50q.com` with the author's replies.
  Changes: OPEN and INQUIRE leave %X00000001, READ CLI$_NORMAL,
  CLOSE/NOLOG of a name never opened CLI$_NORMAL, DEASSIGN SS$_NORMAL;
  READ's and WRITE's failures have no bit 28 and OPEN's always do;
  OPENOUT only for a new file, with SS$_NOSUCHFILE under RMS$_DNF;
  CONFLICT names /WRITE; every open that writes may read and update, and
  without /SHARE the sharing is RMS's default (a writer keeps everyone
  out); a new file is RAT=PRN with 01 8D control (unconfirmed bytes);
  the logical name is CONFINE, equated to `_NODE$DEVICE` (TERMINAL), and
  the only way to the file (deassigned, the file is out of reach); TYPE
  of it types the file from READ's place; `SYS$OUTPUT:` is UNDFIL;
  SYS$ERROR and SYS$COMMAND are the terminal; a WRITE item over 255
  characters is CLI_TKNOVF (new message), and a segment line is cut at
  255 characters; READ's prompt is "Data:" and a tab; INQUIRE's reply is
  shown with verification; after a branch the procedure's own status is
  CLI$_NORMAL (`endStatus`). Fixed on the way: TYPE took no part in a
  file's sharing (now RMS$_FLK), and the terminal's Ctrl/Z reached
  INQUIRE and READ as a character. Still open: the fixed control bytes,
  and SHOW LOGICAL's extra blank.
- 2026-10-11: Subtask 18, STOP and Ctrl/Y (see "STOP and Ctrl/Y"). New
  file `dclcontroly.go`; `HandleAttention` moved there from
  `services.go`; `Engine.ClearAttention`; each `commandLevel` keeps its
  `dispatch`. STOP with no process ends every procedure (and, at the
  terminal, runs down a stopped image; it was NOPRIV). Ctrl/Y in a
  procedure: ON CONTROL_Y's command, a nested procedure's end, or the
  Ctrl/Y level with CONTINUE, EXIT, STOP, and image commands. SET
  [NO]CONTROL[=(T,Y)] (new `console.dcl` syntax `set_control`, one of
  DCL's own commands), and cmd/govax's Ctrl/Y does nothing but echo
  while it's off. HELP: STOP, CONTINUE, ON, SET CONTROL, KEYS. Tests:
  `dclcontroly_test.go` (STOP and ON ERROR THEN STOP nested, ON
  CONTROL_Y's GOTO and continuing, a nested procedure's end, the Ctrl/Y
  level's five ways out, SET CONTROL), `controlyimage_test.go` (an image
  interrupted mid-run: STOP, ON CONTROL_Y, CONTINUE). The choices are in
  DEVIATIONS.md ("[Phase 50] STOP and Ctrl/Y").
- 2026-10-11: Subtask 19, the subprocess's DCL (see "The subprocess's
  DCL"). The author asked for the console's whole DCL in a subprocess,
  not `@` alone. `subcli.go` is rewritten: the interpreter is a goroutine
  taking turns with the engine, with the console's per-process DCL state
  swapped in (`dclState`, `swapDCLState`); `Console.processOne` and
  `subCLI`; `Run` in a subprocess is `subprocessRun`; new verbs LOGOUT
  and MCR; new message VAX_NOTSUBPROC; `NewDispatcher` makes the first
  dispatcher the console's; EXIT n at level 0 sets $STATUS; SHOW
  LOGICAL's status is VMS's. The old CLI's own parser and displays are
  gone. HELP: SPAWN, LOGOUT, MCR. Tests: `subdcl_test.go` (SPAWN of a
  DCL command; a spawned procedure with IF, a GOTO loop, an image reading
  its data lines, lexicals, OPEN/READ, inherited symbols, and process
  1's DCL untouched; DEBUG refused; STOP of a subprocess mid-procedure),
  and the Phase 48 tests unchanged but for an alias that relied on
  being looked up twice. The author asked whether a procedure's SPAWN of
  another procedure works, and with /NOWAIT whether they run in
  parallel: yes, and in turns while the machine runs (above).
- 2026-10-11: At the author's request, ABOUT and XTEST are removed: old
  tests of running microkernel code as console commands, whose
  purpose HELP ABOUT serves now. With them goes the mechanism: the
  grammar's `/entry=` (`dcl.Entry.EntryPoint`, `Result.EntryPoint`),
  the dispatcher's CALL of it, XFC$DCL (`cpu`'s `emulXfcDCL` and
  `SystemServices`' four DCL methods, the console's stubs, the
  assembler's builtin name), and kernel.asm's DCL$ routines, EXE$ABOUT,
  and EXE$XTEST. SHOW VERSION, which called EXE$ABOUT, prints the same
  two lines from Go (`ShowVersion`). The assembler's golden fixtures are
  regenerated: code assembled after kernel.asm moved.

