# Phase 36 — The FORTH fixture: a MACRO-32 program, and what it took to run

**Status:** done (2026-10-04). Validated against VMS 7.3: govax's object,
listing, image, and map for `forth.mar` match real MACRO's and real
LINK's.

## Goal

Give govax a larger, richer MACRO-32 program than the fixture ladder's
small modules: one that assembles with MACRO, links with LINK, and runs
with RUN, for future work on ANALYZE/OBJECT, ANALYZE/IMAGE, and the
debugger. The program is `testdata/mar/forth.mar`, a port of
`testdata/asm/forth.asm` (Vforth, Andy Valencia's 1984 subroutine-threaded
FORTH, which eVAX ran under its console assembler).

The author directed that bugs found in `forth.asm` be fixed in the port,
that I/O be replaced with VMS facilities, and that missing or wrong
system-service and RTL support found along the way is in scope. They also
asked for a FORTH command line (LIB$GET_FOREIGN and foreign commands), the
removal of the console's vestigial FORTH verb, and the use of their VMS
7.3 run of the program as validated results.

## The program (`testdata/mar/forth.mar`)

`testdata/asm/forth.asm` is unchanged. The port:

- **Psects:** code (`FORTH_CODE`, NOWRT EXE), constant text and tables
  (`FORTH_TEXT`), variables and buffers (`FORTH_DATA`), the built-in
  dictionary's headers (`FORTH_WORDS`, NOWRT), and the dictionary new words
  are compiled into (`FORTH_DICT`, WRT EXE PAGE). `$FAB`'s strings go in
  `$RMSNAM`, as real STARLET's do.
- **Macros:** `DEFWORD`/`DEFPRIM`/`ENDPRIM` build each dictionary header
  (chained through the redefined symbol `LASTWORD`) and compute a
  primitive's in-line length (`LABEL'_E-LABEL`) instead of hand-counting
  it. Names with MACRO-special characters are passed as `<;>` or `^%>r%`.
- **I/O is RMS.** The terminal is `SYS$INPUT` (`$OPEN`, `$GET` with
  `ROP=PMT`) and `SYS$OUTPUT` (`$CREATE`, `$PUT`). A *unit block* (FAB,
  RAB, line length, line buffer) is one open file; `input` and `output`
  open `.FTH` and `.LIS` files from templates. Output is gathered a line at
  a time and written as one record; a line still unfinished when the
  terminal is read becomes part of the prompt. The original used
  Unix-style `CHMK` calls (read/write/open/close) and DEC C string routines.
- **Command text:** at start-up it calls LIB$GET_FOREIGN; text there is
  interpreted with `" halt"` appended, as the original's string-argument
  entry did, and an error then ends the run with SS$_ABORT instead of
  falling back to the terminal. Without text it prints a banner and is
  interactive. Word lookup ignores case, since DCL uppercases a command.
- **BSD-isms** (`jbr`, `jeql`, `.jneq`, `jnequ`, ...) became
  `BRB`/`BRW`/`BSBW` and branches around `BRW`; addresses are taken with
  `MOVAB`/`MOVAL`; `.set`, `.space`, `.asciz "..."`, and C-style character
  literals became MACRO-32's forms.
- **The operand stack** is an area of its own (`OPSTK`, 256 longwords,
  with a guard), with underflow and overflow checks. The original put it
  80 bytes below the return stack, where nested calls and RMS calls
  overwrote it.

### Bugs fixed in the port

| Original | Fix |
|---|---|
| `(` compared R0 with the byte at address 0 (`cmpb r0,0`) | `TSTB` |
| `and` was logical (`bitl`), not bitwise as its comment says | `MCOML`/`BICL2` |
| `c@` sign-extended | `MOVZBL` |
| `mod` ignored a negative dividend's sign (`clrl r3`) | `EMUL #1` sign-extends |
| `fill` with a count of 0 wrote a byte | test first; `MOVC5` |
| `.` had 14 bytes for digits (base 2 needs 33) and no trailing blank | 34 bytes; prints a blank |
| `12x` accepted as 12; digits above F misread in bases over 16 | trailing junk is `not found`; letters map through Z |
| `;` with an unmatched control structure didn't abort | aborts |
| `."` across input lines lost its string pointer (prompt clobbered R1) | `GETLIN` saves R1–R5 |
| words over 80 characters, `."` strings over 132, more than 7 nested files overran buffers | bounded |
| `+loop` redefined `XLoop1`/`XLoop2` | `XPLOOP1`/`XPLOOP2` |

Behaviors kept on purpose: `(` comments end at a line's end; `.` on an
empty stack prints before the underflow check catches it; `>r`, `r>`, `i`,
`j`, `k`, and `leave` only make sense compiled in line.

## Fixtures and tests

| Fixture / test | What it checks |
|---|---|
| `testdata/mar/forth.mar` | the program |
| `testdata/mar/vax/forth.{obj,lis,map,exe}` | real MACRO V5.4-3 (`/LIST/CROSS_REFERENCE`) and LINK V11-39 (`/MAP`) on VMS 7.3 |
| `internal/asm` ladder tests (`usesStarlet`) | object record for record (`TestFixtureLadderObjects`), declarations, text and relocations, source pages (`TestListingLines`), whole listing (`TestFixtureListings`) |
| `internal/link` `TestLinkForthMatchesRealLINK` | image byte for byte, from govax's object and real MACRO's |
| `internal/link` `TestMapMatchesRealLINK/forth` | map line for line |
| `internal/console` `TestForth_interpreter` | arithmetic, stack words, every control structure, variables, constants, strings, floats, bases, errors |
| `internal/console` `TestForth_files` | nested `input` files resuming, `output` to a file, exit at end of input |
| `internal/console` `TestForth_foreignCommand` | `FO*RTH :== $...`, command text, SS$_ABORT on an error |

The VMS run came on `testdata/disks/forth-exchange.dsk` (local only, like
every container). Its `FORTH.MAR;2` is the committed source. Text files
were copied out with `COPY .../HOST`; the image needs `/HOST/BINARY`
(a plain host copy adds a line feed after each 512-byte block).

**Clean-room audit of `vax/forth.lis`:** checked mechanically, printing
only counts: all 1733 source lines carry their line numbers and no
unnumbered line has source text, so the listing shows no macro expansion
(MACRO's default `/SHOW`). Its closing pages name some of VMS STARLET's
internal symbols and helper macros; those are names, and the tests mask
them rather than copying them anywhere.

## govax changes

### RTL and system services

- **LIB$PUT_OUTPUT** is `internal/librtl/output.go` (XFC code 40, offset
  ^X478): one descriptor, one line on SYS$OUTPUT. kernel.asm's old version
  (a character at a time through the console transmit interrupt) is now
  its private `EXE$PUT_OUTPUT`/`EXE$PUT_ONE`, for the kernel's own
  messages; kernel.asm's `.shim` row for code 40 still defines
  `LIB$PUT_OUTPUT` for eVAX-dialect `ASM` programs. `foo.asm` now runs to
  completion.
- **LIB$GET_FOREIGN** is `internal/librtl/foreign.go` (code 41, ^X878);
  see "Command text: LIB$GET_FOREIGN and DCL symbols" below. Shim stubs:
  41 of 42 used (`shimPageBytes`).
- **Terminal SYS$GET** (`internal/rms/terminal.go`): a RAB connected to the
  terminal used to get RMS$_PRV. It now writes the RAB$L_PBF/RAB$B_PSZ
  prompt when RAB$V_PMT is set, reads to a CR or LF (a host CRLF is one),
  at most RAB$W_USZ bytes (the rest is the next record), and returns
  RMS$_EOF at Ctrl/Z or end of input. `rms.Context.ConsoleIn` is the
  console reader the terminal driver shares.
- **The RTL's console streams** are read through `c.In`/`c.Out` at each
  use (`consoleInput`/`consoleOutput`, `internal/console/machine.go`), so
  a program's I/O follows a later change of them, as the console device's
  already did.

### Console and DCL

- **The FORTH verb is gone** from `console.dcl`, with kernel.asm's
  `exe$forth_dcl` (whose calls to a microkernel FORTH were commented out)
  and the commented-out forth include and `exe$forth_init` call.
- **DCL symbols and foreign commands**, and **`govax run IMAGE
  text...`**: see the next section.

## Command text: LIB$GET_FOREIGN and DCL symbols

An image's *command text* is what followed the command's verb, which the
image reads with LIB$GET_FOREIGN. On VMS an image gets one when it's run
as a *foreign command*: a DCL symbol whose value is `$` and an image's
file. RUN takes no parameters, so an image RUN runs has none. govax
models both, and adds a host-side way in.

### Where the text lives

`corevms.Environment.CommandLine` (a string) holds the current image's
command text. The console sets it in `Console.Run` from
`RunOptions.CommandLine` before every image starts, so each RUN replaces
the last one's text (an empty string for a plain RUN). Three paths set
`RunOptions.CommandLine`:

| How the image is started | Text | Treatment |
|---|---|---|
| `RUN file` | none | — |
| a foreign command at the `VAX>` prompt (`FORTH 2 3 + .`) | the rest of the line | DCL's (below) |
| `govax run file text...` from the host shell | the arguments after the file, joined by blanks | none: the shell already parsed it, so case and quotes are as the shell left them |

The host path: `cmd/govax/grammar.go`'s `runCmd` puts the extra
arguments in `console.RunCommandLine` and runs `RUN file` as the one-shot
command; `Console.Include("/command_line")` moves the text to the console
(`runCommandLine`), and `cmdRun` hands it to that RUN only.

### LIB$GET_FOREIGN (`internal/librtl/foreign.go`)

    LIB$GET_FOREIGN resultant-string [,prompt-string] [,resultant-length] [,flags]

Written from the RTL Library manual:

- It returns `Environment.CommandLine` in *resultant-string*.
- If the text is empty, or bit 0 of the longword at *flags* is set, and a
  *prompt-string* is given, it prompts on SYS$INPUT instead
  (`Environment.ReadInputLine`: the prompt, then one line without its
  terminator) and returns what was typed. At the end of the input it
  returns RMS$_EOF.
- If *flags* is given, it's set to 1 afterward, so a program that calls it
  in a loop prompts from the second call on.
- *resultant-string* may be fixed length (a MACRO program's static
  descriptor: copied, blank-padded, LIB$_INPSTRTRU if cut short) or
  dynamic (DSC$K_CLASS_D: its old storage freed, new storage of the
  text's length allocated from the RTL heap). *resultant-length*, a word,
  gets the number of characters stored.
- No *resultant-string* is LIB$_INVARG; an unreadable descriptor,
  SS$_ACCVIO.

`foreign_test.go` covers each case. `forth.mar` calls it with a static
descriptor over its input buffer and no prompt.

### DCL symbols (`internal/console/dclsym.go`)

The console keeps a table of DCL symbols: `Console.dclSymbols`, a map
from each symbol's full, uppercase name to a `dclSymbol` (name, the
shortest abbreviation's length, value). It's a different table from the
console's *VAX symbols* (labels and values from ASM, kernel.asm, and
LINK's images, which ASM, CALL, EXAMINE, SHOW SYMBOL, and CLEAR SYMBOL
use), as DCL's symbols are separate from an image's. It lasts for the
console session; INIT, VMINIT, and ZERO don't clear it.

`Dispatcher.Dispatch` gives every command line to `dclSymbolLine` before
any verb is looked up (after the interactive-ASM check), because DCL
looks for a symbol before a verb. `dclSymbolLine` handles, in order:

1. **An assignment** (`splitAssignment`): a name, then `:=`, `:==`, `=`,
   or `==`.
2. **`DELETE/SYMBOL`** (`isDeleteSymbol`): DELETE, abbreviated to at
   least DEL, followed by /SYMBOL, at least /SY. The DELETE verb in the
   grammar (files) is never reached for it.
3. **A command whose first word is a symbol** (`readCommandVerb`'s word,
   looked up with abbreviations): substitution, below.

Anything else goes on to the fixed commands and the grammar as before.

### Console commands that use DCL symbols

**Assignment** defines or redefines a symbol:

    name :=  string        name =  expression
    name :== string        name == expression

- A name starts with a letter, `$`, or `_`, and has letters, digits, `$`,
  and `_`. One `*` may mark the shortest abbreviation that still means the
  symbol: after `FO*RTH :== $FORTH`, FO, FOR, FORT, and FORTH all do.
  Lookup tries the exact name, then the abbreviations, in name order.
- `:=`/`:==` assign a string: the rest of the line, with DCL's treatment
  (uppercase and one blank for each run of blanks outside quotes, `!`
  starts a comment), quotes removed, and `""` inside quotes one quote
  (`dclText(text, false)`).
- `=`/`==` assign an expression's value: the console evaluates a quoted
  string (`""` for a quote) or a decimal integer (its decimal string);
  anything else is CLI_EXPSYN, an unclosed string CLI_UNTERMSTR.
- DCL's local (`:=`, `=`) and global (`:==`, `==`) symbols share one
  table, since the console has no command procedures for a local symbol
  to be local to.

**Using a symbol as a command** (the first word of a line):

- A value starting with `$` is a **foreign command**: the rest of the
  value names the image (quotes trimmed), found as RUN finds an image
  (DCL would default it to SYS$SYSTEM:, which govax doesn't define), and
  the rest of the command line, after DCL's treatment with quotes kept
  (`dclText(text, true)`), is its command text. The image runs as RUN
  runs it (`DefaultRunInits` applies).
- Any other value is the start of a command: the line is dispatched again
  with the value in place of the word, so a symbol abbreviates or renames
  a command (`SD :== SET DEFAULT`, then `SD DUA0:[X]`). An alias can name
  another; one that leads back to itself stops at 16 substitutions with
  CLI_SYMDEPTH.

**`DELETE/SYMBOL [/GLOBAL | /LOCAL] name`** removes a symbol (any allowed
abbreviation names it); /GLOBAL and /LOCAL are accepted and change
nothing. An undefined name is CLI_UNDEFSYM.

**`SHOW SYMBOL/DCL [name]`** shows DCL symbols as DCL's SHOW SYMBOL does
(`ShowDCLSymbols`; the grammar's `show_sym_dcl` syntax, id 157, a
qualifier of SHOW SYMBOL): with no name every one, in name order; with a
wildcard name (`*`, `%`, `lnm.Match`) the ones that match; otherwise the
one the name or an allowed abbreviation means (CLI_UNDEFSYM if none).
Each is shown as

      FO*RTH == "$DUA0:[TOOLS]FORTH.EXE"
      N = 42   Hex = 0000002A  Octal = 00000000052

`==` for a symbol assigned with `:==` or `==` (global, to DCL), `=` for
`:=` or `=` (local), the `*` where the abbreviation point is, a string in
quotes with any quote in it doubled, and an integer (`=`/`==` of a
number) in decimal, hexadecimal, and octal. For this, each `dclSymbol`
also records `global` and `integer`. Without /DCL, SHOW SYMBOL shows the
console's VAX symbols as before. With no symbols defined it says "No DCL
symbols are defined".

**HELP SYMBOL, HELP FOREIGN, HELP DELETE /SYMBOL, and HELP SHOW SYMBOL
/DCL** describe them
(`vax.help`). `Console.DCLSymbol(name)` returns a symbol's value for Go
callers and tests.

What the console does **not** have: apostrophe substitution
(`'SYMBOL'` inside a command), symbols in expressions other commands
evaluate, SHOW SYMBOL's /GLOBAL and /LOCAL (the table is one), and
DELETE/SYMBOL/ALL.

### Command text's treatment

`dclText(text, keepQuotes)` is DCL's handling of a command's parameters:
outside quotes, letters are uppercased, each run of blanks and tabs
becomes one blank, leading and trailing blanks are dropped, and a `!`
ends the text; quoted text is kept as typed. For a foreign command the
quotes are kept (`FORTH ." Hi"` gives `." Hi"`); for a string assignment
they're removed. Unconfirmed against VMS: that LIB$GET_FOREIGN's text
keeps the quotes and their contents' case.

### Tests

`dclsym_test.go`: `dclText`'s rules, `splitAssignment` against commands
that look like assignments (`SET PC=200`), `assignSymbol`'s forms,
abbreviations, and errors, and through the dispatcher an alias, a
self-referencing alias, and DELETE/SYMBOL; `TestShowDCLSymbols`, SHOW
SYMBOL/DCL's forms and format. `forth_test.go`'s
`TestForth_foreignCommand` defines `FO*RTH :== $...` and runs forth with
command text through `Dispatch`.

## govax changes, continued

### Assembler (MACRO-32 fidelity)

Found by writing the program, or by comparing with the VMS run:

1. **`;` inside a macro argument.** `<;>` and `^%...;...%` were cut at the
   `;` as a comment. `preprocessCase` now keeps a `;` inside angle brackets
   or a `^x...x` argument (`delimitedArgumentAt`), which must start an
   argument (after a blank or comma), so `B^^X0C(AP)` is untouched.
2. **A forward-referenced expression stays a linker expression.**
   `.WORD INLMAX-HALT_LEN`, with `HALT_LEN = .-HALT_TEXT` later, was folded
   to 1017. Real MACRO wrote `STA_UW 1024, STA_UB 7, OPR_SUB, STO_W`: it
   decides in pass 1, so a field that used an undefined symbol goes to the
   linker with the value substituted but not folded (`completeFixup`, for
   address/data fixups).
3. **Symbol table columns.** Real MACRO lays the table out down a page's 57
   lines, then down the next column: two columns of 58-character entries
   for 31-wide names; a short last page stays one column
   (`symbolTableColumns`). govax wrote one column, which the smaller
   fixtures couldn't tell apart.
4. **Cross reference `#-` marks.** A symbol in any instruction operand that
   is read, written, modified, or branched to is marked; one in an
   address-access operand (`MOVAB X`, MOVC3's addresses) isn't
   (`assembleOperand`). govax marked only literals and registers.
5. **Cross reference width** is the symbol table's (31 if any symbol, listed
   or not, is longer than 15), not the listed names'.
6. **A library macro's definition line** is the line that loaded it (its
   first call), also listed as a reference (`loadLibraryMacro`). PHASE-29's
   rule (none) had no real evidence; it's superseded.

### govax's STARLET (clean room, from real MACRO's output)

- **Option bits from `$V_` positions.** VMS's `$FAB`/`$RAB` refer to
  `FAB$V_GET`, `FAB$V_CR`, `RAB$V_PMT`, ... (the listing's symbol table),
  not the `$M_` masks. `$$RMSBIT` now ORs `1@PFX'X`, and every caller
  passes a `$V_` prefix; the bytes are unchanged (the RMS oracles pass).
- **`$xxxDEF` under `.NOCROSS`.** Real MACRO's cross reference gives a
  `$FABDEF` symbol no definition line and leaves out the unused ones;
  `internal/bootdata/mkdefs` now wraps the definitions in
  `.NOCROSS`/`.CROSS`.

### Tests

- Ladder tests take `usesStarlet` sources; `ladderAssemble` gives them
  govax's STARLET. The Text test's real-object summary keeps the last of
  two stores to one field (`$FAB`'s FNA/DNA, stored 0 then the string),
  matching `Relocations()`.
- `withoutMacroInternals` (`listpage_test.go`) is the one allowed
  difference for listings of programs that call system macros, replacing
  Phase 28's `systemMacroDifferences`: `$$` symbols and macros, VMS's
  helper macros (only macros the source names are compared), library
  macro sizes and counts, and `FAB$V_FILE_MODE` (below).

## Decisions and unconfirmed rules

- **FAB$V_FILE_MODE.** VMS's `$FAB` refers to it (building FAB$B_ACMODES
  from a file mode too), but the RMS manual gives `$FAB` no such argument,
  so govax's `$FAB` doesn't; the listing test masks the name.
- **Unconfirmed** (no real output yet): SHOW SYMBOL/DCL's integer line
  spacing (written from memory of DCL's, not a captured run); a 15-wide
  symbol table takes three
  columns; a field operand's base (INSV, EXTV) is unmarked in the cross
  reference; LIB$GET_FOREIGN sets flags to 1 whether or not it prompted,
  and returns a prompted line as typed; a foreign command's text keeps its
  quotes and their contents' case.
- **One DCL symbol table** for local and global symbols (no command
  procedures to be local to). Not done: apostrophe substitution, DCL's SHOW
  SYMBOL (the console's is for VAX symbols), SYS$SYSTEM as a foreign
  image's default directory.

## Open items

- RUN doesn't print an image's failing exit status, as DCL does
  (`%SYSTEM-F-ABORT`); R0 has it.
- The shim page has room for one more stub (41 of 42).
- `COPY .../HOST` of a fixed-512 file with no carriage control renders it
  as text; images need `/BINARY`.
- A VMS probe for the unconfirmed rules above, and ANALYZE/OBJECT and
  ANALYZE/IMAGE output for `forth`, would be the next validation.

## Progress log

- 2026-10-04: `forth.mar` written; terminal `$GET`, `LIB$PUT_OUTPUT` in
  librtl, dynamic RTL console streams, `;` in macro arguments;
  `TestForth_interpreter`/`_files` (881d5da).
- 2026-10-04: FORTH verb removed (7dff869).
- 2026-10-04: LIB$GET_FOREIGN, DCL symbols and foreign commands,
  `govax run IMAGE text...`, forth reads its command text (fc5da65).
- 2026-10-04: The author's VMS 7.3 run (`forth-exchange.dsk`) added as
  `testdata/mar/vax/forth.*`; `forth` joined the ladder and the LINK
  comparisons. Fixed: forward-referenced expressions, symbol table
  columns, cross reference marks, width, and library macro definitions;
  STARLET's `$V_` option bits and `.NOCROSS` definitions. Object, listing,
  image, and map all match VMS (2614b23).
- 2026-10-04: SHOW SYMBOL/DCL, at the author's request.
