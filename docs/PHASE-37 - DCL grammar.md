# Phase 37 — The fixed console commands move onto the DCL grammar

**Status:** done (2026-10-04). Every console command is parsed by the DCL
grammar; `fixedCommands` is gone.

## Goal

Every console command is parsed by the DCL grammar (`internal/console/dcl`,
`internal/bootdata/files/console.dcl`), with parameters and qualifiers
declared there and handlers reading a `dcl.Result`. Until this phase about
twenty verbs were "fixed" commands: `fixedCommands` in
`internal/console/dispatch.go` matched a verb's first four characters
(console_dispatch_table's convention in the C source) ahead of the grammar
and handed the rest of the line to a handler that picked its own
qualifiers and parameters apart by hand.

The author directed that the DCL engine be fixed or extended wherever these
commands need something it lacks, and agreed (2026-10-04) that names whose
case matters, such as host file paths, must be quoted, as they already must
be for COPY, MACRO, and LINK.

## The fixed commands

| Verb (fixed spellings) | Form | Handler |
|---|---|---|
| ZERO | `ZERO` | `Console.Zero` |
| EXAMINE (`EXAM`, `EX`, `DUMP`) | `EXAMINE[/size] [addr [end]]` or a register | `Console.Examine` |
| DEPOSIT (`DEP`, `D`) | `DEPOSIT[/size] target[=]value` | `Console.Deposit` |
| STEP (`ST`, `S`) | `STEP[/mode] [addr]` | `Console.Step` |
| EXECUTE (`EXEC`, `GO`, `G`) | `EXECUTE [addr]` | `Console.Execute` |
| RUN (`R`) | `RUN[/qual] file[/HOST]` | `Console.Run` |
| SAVE, LOAD | `SAVE/ROM file`, `LOAD/NVRAM[/NOERROR] [file]` | `rom.go` |
| TIME | `TIME [command]` | `Console.Time` |
| PRINT (`PRIN`, `ECHO`) | `PRINT [item[,item...]]` | `Console.Print` |
| HELP (`?`) | `HELP [topic...]` | `Console.Help` |
| INCLUDE (`INCL`, `INC`, `@`) | `INCLUDE file`, `INCLUDE/COMMAND_LINE` | `Console.Include` |
| SET | about twenty sub-forms, plus `SET [/qual] name=value` | `set.go` |
| ASM (`ASSE`) | `ASM [file]` | `Console.Assemble`/`AssembleBegin` |
| DISASSEMBLE (`DISA`, `DIS`) | `DISASSEMBLE [start [end]]` | `Console.Disassemble` |
| CALL | `CALL[/STEP] entry[(arg,...)]` | `Console.Call` |
| IF | `IF expr [THEN] command` | dispatches the command |
| BOOT, ROM | not implemented | `CLI_NEEDDEP` |

Defects in the brute-force parsing, found while surveying:

- `DEPOSIT` spelled out never reached a handler: its first four
  characters, `DEPO`, aren't in the table (only `DEP` and `D` are), and the
  grammar had no DEPOSIT verb. The same holds for any spelling between the
  table's entries and the full word (`EXA`, `EXECU` reached `EXEC`, but
  `PRI`, `INCLU`... vary).
- At most one leading qualifier was understood (RUN, CALL, STEP, EXAMINE),
  and a second one was read as part of the file name or expression.
- `LOAD/ROM/NOERROR` worked only because `LoadROM` compared its whole file
  name argument with `/NOERROR`.
- `INCLUDE/COMMAND_LINE` likewise compared its file name with
  `/command_line`.

## Design

### What the grammar does and what the handler does

The grammar owns the verb, its abbreviations, every qualifier, and how the
line divides into parameters. Address and value expressions stay the
console expression evaluator's business (`expr.go`): the grammar finds an
expression's extent, and the handler evaluates it. A few parameters are
small languages of their own and stay `$rest_of_line` for their handlers:
`IF`'s conditioned command, `TIME`'s timed command, `HELP`'s topic words,
and `SET PSL`'s and `SET PTE`'s `field=value` lists.

### DCL engine additions (subtask 1)

- **`$expression` value type.** A console expression may contain blanks
  (`X + 4`), parentheses (`F(1,2)`), quoted strings, `/` as division, and
  `=` as a comparison, none of which a plain DCL token allows. The
  `$expression` scanner reads one expression: it continues across blanks
  when an operator joins the two sides, keeps parentheses and quotes
  balanced, and stops at a top-level `,` (a list's next element), at the
  parameter's separator (below), or at a `/` that follows a blank and is
  followed by a letter (a qualifier: `EXAMINE 100 /BYTE`). A `/` anywhere
  else is division. The value keeps its quotes, so the evaluator still sees
  a string literal as one.
- **Parameter `/separator=c`.** The parameter's value ends at an unquoted,
  top-level `c`, and one `c` (with blanks around it) is skipped before the
  next parameter: DEPOSIT's `D X=5` and SET's `SET PC = 200`.
- **Verb or syntax `/assignment=syntax`.** When the first positional token
  is a name followed by `=`, parsing continues in the named syntax:
  `SET R=5` is an assignment, though `R` abbreviates `RADIX`, as in
  console_set.c, which looks for `name=` before any keyword.
- **Keyword `/nonegatable`.** dclrtl.c's per-keyword DCL_NONEGATE flag,
  which `matchKeyword` didn't port: SET's `NOTRACE`, `NOVM`, and
  `NOVERBOSE` are negated keywords, but `NORADIX` must be an error.
- **`@` is a verb by itself.** `@FILE`, as DCL reads it.

### Verb spellings

The old four-character spellings become DCL verbs or aliases (`verb
ex/alias=examine`), so exact spellings keep working and every unambiguous
abbreviation now works too. An exact match wins over a prefix match, so `S`
is STEP and `D` is DEPOSIT, as before.

### Quoting

DCL uppercases everything outside quotes, and a `/` outside quotes starts
a qualifier. A host file name with lowercase letters or a `/` must be
quoted: `ASM "kernel.asm"`, `RUN "/tmp/prog.exe"/HOST`. `vax.init` and
`cmd/govax` already quote, or are changed to.

## Subtasks

1. DCL engine: `$expression`, `/separator=`, `/assignment=`,
   `/nonegatable` keywords, `@` as a verb. Unit tests in
   `internal/console/dcl`.
2. ZERO, BOOT, ROM, TIME, PRINT/ECHO, HELP/`?`, IF.
3. STEP, EXECUTE/GO/G, CALL, RUN/R.
4. EXAMINE/EX/DUMP, DEPOSIT/DEP/D, DISASSEMBLE/DIS.
5. ASM/ASSEMBLE, INCLUDE/INC/`@`, SAVE, LOAD (`/NOERROR` becomes a real
   qualifier); `cmd/govax` quotes the file names it passes.
6. SET, every sub-form.
7. `fixedCommands` retired; `Dispatcher`'s documentation, `vax.help`,
   `CLAUDE.md`, and `PLAN.md` updated.

Each subtask is committed when its tests pass.

## Decisions and unconfirmed rules

- **PRINT's items are a DCL list,** separated by commas. console_print.c
  (and the old Go loop) also took items separated only by blanks
  (`PRINT "A" 1`); that form is now an extra parameter. A quoted string by
  itself is printed as text; inside a larger expression it is the
  evaluator's string literal, as before.
- **IF's expression is an `$expression`,** so it ends where a blank isn't
  joined by an operator: `IF X = 1 THEN ...` and `IF DEFINED("X") SET ...`
  read as before. Text left after evaluating it is an error, where it
  used to be dispatched as the command.
- **Qualifiers combine and go anywhere** DCL allows: `RUN/NOINIT/STEP`,
  `STEP 200 /OVER`. console_run, console_call, and console_step read one
  leading qualifier.
- **CALL's argument list may follow a blank** (`CALL F (1,2)`): the list
  is a second `$expression` parameter, since a blank before `(` ends the
  routine's expression. The list itself is still read by the handler,
  with the evaluator, as console_call reads it.
- **RUN's file is required by the grammar** (`/prompt=`): a bare RUN is
  CLI_MISSINGPARAMETER, not CLI_NOFILE.
- **DEPOSIT's target ends at `=`** (`/separator="="`), so a comparison
  in a DEPOSIT target needs parentheses; the value may compare freely.
  Every DEPOSIT spelling now works, `DEPOSIT` itself included.
- **`/NOERROR` and `/COMMAND_LINE` are qualifiers.** `LoadROM` and
  `LoadNVRAM` take a `noError` flag instead of comparing their file name
  with `/NOERROR`; with it, the file may be omitted (DEFAULT.ROM,
  DEFAULT.NVRAM) and an unopenable file loads nothing, as before. LOAD's
  file may now come before the qualifier, too. `INCLUDE/COMMAND_LINE` is
  a syntax of its own (`Console.IncludeCommandLine`), split out of
  `Include`. console_load.c's `/SILENT` synonym for `/NOERROR`, and
  console_include's `/[NO]VERIFY`, `/LIST`, and `/ASM`, were never in the
  Go port and aren't added.
- **`govax asm` quotes its file names,** as `govax run`, `macro`, and
  `link` do (`doCmd`).
- **SET is a keyword parameter whose keywords redirect** into one syntax
  per form (`set_types`, `set_radix`, ...), as SHOW's are. An assignment
  is recognized first (`/assignment=set_symbol`), as console_set.c does:
  `SET R=5` assigns R though `R` abbreviates RADIX, and `SET RADIX = 10`
  assigns a symbol named RADIX. SET's keywords now abbreviate (`SET BR`,
  `SET UIQ`), where the old switch wanted the exact words it listed.
  `/PERMANENT`, `/ENTRY`, and `/LABEL` (and C's `/PRM`, `/LBL`) may come
  before or after the assignment.
- **SET's NO forms are negated keywords:** `NOTRACE`, `NODISASSEMBLE`
  (`DISASSEMBLER`, which `vax.help` documents, was added beside
  `DISASSEMBLY` so the old `NODISASSEMBLE` spelling still matches), `NOVM`,
  `NOMAPEN`, `NOVERBOSE`. Every other SET keyword is `/nonegatable`.
- **SET PSL's clauses are a DCL list of `$expression`s**, each split at its
  first `=` by the handler, so `SET PSL IPL = 1F, N=1` works. SET PTE's
  changes stay `$rest_of_line`, after an `$expression` address: the
  optional `TO address` between them is the handler's to recognize, since
  DCL has no optional positional keyword.
- **SET DEBUG's flags are a DCL list:** commas, not blanks, separate them.
- **SET QUANTUM, UIQUANTUM, FAULT, and HISTORY take `$integer`**, so a bad
  count is CLI_BADINTEGER (was CLI_BADNUMBER).
- **Missing parameters are the grammar's CLI_MISSINGPARAMETER.** The
  former fixed commands' own statuses for them (CLI_NEEDSETARG,
  CLI_NEEDRADIX, CLI_NEEDBREAKADDR, CLI_NEEDBREAKOPCODE, CLI_NEEDSTEPMODE,
  CLI_NEEDMODE, CLI_NEEDDEPOSIT, CLI_NEEDENTRY, and the unrecognized-SET-
  qualifier CLI_BADQUALPREFIX) are no longer raised; they stay defined in
  `internal/vmserrors`. CLI_NEEDROMNVRAM and CLI_NEEDFILENAME are still
  SAVE's and LOAD's, whose handlers name the qualifier they want.
- **Handlers live in `internal/console/commands.go`** (`bindConsoleCommands`),
  called from `bindGrammar`, and SET's in `setcommand.go`
  (`bindSetCommands`).

## Open items

- `vax.help` is the C project's help file and still documents C features
  the port never had (EXAMINE/INSTRUCTION, EXAMINE's float and string
  subtypes, SAVE/TEXT, LOAD/ROM's /BASE and /SIZE, INIT/NVRAM). Only its
  INCLUDE example changed here (quoting). A pass to match it to govax's
  commands is its own task.
- The expression evaluator has no register names (expr.go's doc
  comment), so `PRINT R0` and `DEPOSIT 2000 = R0` don't work; EXAMINE and
  DEPOSIT special-case a register by itself, as console_exam.c does.
- `respath` matches names exactly, so `@vax.init` (uppercased to
  VAX.INIT) misses the embedded copy; `@"vax.init"` finds it. A
  case-insensitive fallback for the embedded files would make the
  unquoted form work too.

## Progress log

- 2026-10-04: Survey and plan.
- 2026-10-04: Subtask 1, DCL engine: `$expression`
  (`internal/console/dcl/expression.go`'s `readExpression`; lists of
  expressions too, for PRINT), `/separator=`, `/assignment=`, keyword
  `/nonegatable`, and `@` as a verb. `TestReadExpression` and
  `TestExpressionGrammar` cover each.
- 2026-10-04: Subtask 2: ZERO, BOOT, ROM, TIME, PRINT/ECHO, HELP/`?`, and
  IF are grammar verbs; `Console.Print` takes the item list.
  `commands_test.go` covers them through `Dispatch`.
- 2026-10-04: Subtask 3: STEP/ST/S, EXECUTE/GO/G, CALL, and RUN/R. The old
  unbound `call` verb in `console.dcl` is replaced. `govax run` quotes the
  image's name. `TestRunOptions_defaultAndOverride` replaces the
  `parseRunQualifier` test.
- 2026-10-04: Subtask 4: EXAMINE/EX/DUMP, DEPOSIT/D, DISASSEMBLE/DIS.
  Sizes are qualifiers anywhere on the line, at most one of them
  (`disallow any2`). `TestCommands_depositExamine`.
- 2026-10-04: Subtask 5: ASM/ASSEMBLE, INCLUDE/`@`, SAVE, LOAD. Tests
  quote the host paths they pass. `TestCommands_saveLoad`,
  `TestCommands_include`. Only SET is left in `fixedCommands`.
- 2026-10-04: Subtask 6: SET, every form (`setcommand.go`,
  `TestCommands_set`). `fixedCommands` is now empty. Also fixed: the DCL
  package's grammar-file tests (`TestLoadEvaxGrammar_verbCount`, and the
  parameter-qualifier walk, which now expects RUN's /HOST) had failed
  since subtasks 2 and 3, which ran only the console package's tests.
- 2026-10-04: Subtask 7: `fixedCommands`, `fixedHandler`, and Dispatch's
  four-character lookup removed; `Dispatcher`'s documentation rewritten;
  tests named for the fixed table renamed. `vax.help`'s INCLUDE example
  quotes its file; `CLAUDE.md` and `PLAN.md` updated. Checked end to end
  through `govax console` (boot, DEPOSIT/EXAMINE, PRINT, SET symbol, IF,
  SET STEP, DISASSEMBLE, HELP, TIME).
