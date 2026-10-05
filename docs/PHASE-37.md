# Phase 37 — The fixed console commands move onto the DCL grammar

**Status:** in progress (started 2026-10-04).

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

(Filled in as subtasks land.)

## Progress log

- 2026-10-04: Survey and plan.
- 2026-10-04: Subtask 1, DCL engine: `$expression`
  (`internal/console/dcl/expression.go`'s `readExpression`; lists of
  expressions too, for PRINT), `/separator=`, `/assignment=`, keyword
  `/nonegatable`, and `@` as a verb. `TestReadExpression` and
  `TestExpressionGrammar` cover each.
