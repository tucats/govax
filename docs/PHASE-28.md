# Phase 28: The MACRO-32 macro facility

## Goal

Add MACRO-32's macro facility to the MACRO dialect of `internal/asm`, so that
ordinary VMS MACRO programs assemble with govax's `MACRO` command. Almost
every real program calls system macros (`$EXIT_S`, `$QIOW_S`, `$FAB`, `$RAB`,
and so on) from `SYS$LIBRARY:STARLET.MLB`. Without the macro facility, govax
can only assemble programs that call `SYS$...` entry points directly, as the
Phase 27 fixtures do.

Split out of Phase 27's "later sub-phases" (docs/PHASE-27.md, subtask 12).

**Status: in progress (subtask 1 done; next, subtask 2).**

## Scope

- Macro definitions: `.MACRO`/`.ENDM`, with positional and keyword
  arguments, default values, created local labels (`?label`), string
  concatenation (`'`), and `.NARG`/`.NCHR`/`.NTYPE`.
- The string operators `%LENGTH`, `%LOCATE`, and `%EXTRACT`, and `\symbol`
  value passing.
- Repeat blocks: `.IRP`, `.IRPC`, `.REPEAT`/`.REPT`, and `.MEXIT`.
- Macro libraries: `.MCALL`, `.LIBRARY`, `.MDELETE`, and MACRO's automatic
  search of `STARLET.MLB` for an undefined opcode.
- The message directives the system macros use (`.ERROR`, `.WARN`,
  `.PRINT`), and the listing-control directives they use, accepted and
  ignored.
- The console dialect gets the definition side (`.MACRO`, repeat blocks)
  because the core is shared and it costs nothing extra; it doesn't get
  library search unless that falls out for free (the user's call,
  2026-09-30: nice to have, not required).

## What Phase 27 leaves in place

- `internal/asm` assembles in one pass, a statement at a time
  (`assembleLines`). Macro expansion fits in front of that: an expanded
  macro's lines go through the same statement path, the way `.INCLUDE`'s
  do, with `includeLines`-style nesting for error locations.
- In the MACRO dialect, assembly reports every error
  (`asm.Errors`). Expansion errors should name the macro call's line.
- `.INCLUDE` resolution across host and ODS-2 files
  (`rms.Session.LocateRelated`) is the model for finding a `.LIBRARY`
  file.
- `internal/lbr` (Phase 30) already reads librarian files, including
  data-reduced (DCX) ones. It reads the real `STARLET.MLB` as-is: a macro
  library's module is the macro's source text, one record per line.
- LINK's `readLinkFile` (`internal/console/linksource.go`) already finds a
  VMS library file through its logical name on a mounted volume, then in
  the host directory the `vax.link.library` setting names.

## What the real STARLET.MLB shows

`SYS$LIBRARY:STARLET.MLB` from the VMS 7.3 system disk
(`testdata/disks/rq0-ra92.dsk`, `[VMS$COMMON.SYSLIB]`) was copied as raw
blocks to `testdata/vmslib/starlet.mlb` (gitignored, licensed). It's a
data-reduced macro library of 1529 modules, 46,207 lines in all. Each
module is one macro's source: `.MACRO name args` ... `.ENDM name`.

What the macros use, by count of lines (so the facility has to cover all
of it, not just the directives the manual's chapter 4 lists):

| Directive | Uses | Directive | Uses |
| --- | --- | --- | --- |
| `.MACRO`/`.ENDM` | 2393/2411 | `.IRP`/`.ENDR` | 237/227 |
| `.IF`/`.IFF`/`.ENDC` | 1256/885/1256 | `.MEXIT` | 250 |
| `.IIF` | 344 | `.ERROR`/`.WARN` | 53/15 |
| `.NTYPE` | 18 | `.NCHR` | 10 |
| `.IRPC` | 8 | `.MDELETE` | 6 |
| `.SAVE`/`.RESTORE` | 9/9 | `.NARG` | 1 |
| `.PRINT` | 1 | `.LIST`/`.NLIST`/`.CROSS`/`.NOCROSS` | 3/3/1/1 |
| `%LOCATE`/`%LENGTH` | 2/2 | `%EXTRACT` | 0 |

Things the macros rely on that the plan had not listed:

- **Macros defined by macros.** `$GBLINI` defines `$DEF`, `$EQU`, and
  `$VIELD1` when it's called, and `$DEFEND` redefines `$xxxDEF` to an
  empty macro so that a second `$FABDEF` expands to nothing. A `.MACRO`
  inside an expansion is an ordinary definition.
- **Arguments are substituted everywhere**, inside `.ASCII /.../` strings
  and comments included: `.ERROR ; UNDEFINED VALUE FOR FIELD : ORG;`
  names its argument in the comment, and the comment is the message.
- **Apostrophes.** `FAB$C_'ORG` (concatenation), and `PREFIX''SYM` in a
  macro that defines a macro. The outer expansion drops the apostrophe
  next to its own argument `PREFIX` and leaves the other one for the inner
  macro's `SYM`.
- **Symbols containing `.`**: `BIT...`, `SIZ...`, `$$.TAB`, `$$.TMP`.
  `internal/asm` doesn't yet allow a `.` in a symbol name.
- **`.NTYPE`** (in `$PUSHADR`, which every `_S` service macro uses):
  the addressing mode of an operand, as a number.
- **`.SAVE LOCAL_BLOCK`** and `.PSECT $ABS$,ABS` in `$DEFINI`, which
  every `$xxxDEF` symbol-definition macro calls. These already work.

VMS has no `STARLET.MAR`: the system macros exist only in the library. A
macro source file is made into a library with `LIBRARY/CREATE/MACRO`, and
MACRO's `/LIBRARY` qualifier and `.LIBRARY` both take only `.MLB` files.

## Design decisions

### Where the system macros come from (proposed; the user's question)

The user proposed: use `STARLET.MLB` if it's found, else a `STARLET.MAR`
holding the macro definitions. That's how this phase will work, with one
difference from VMS to note: a `.MAR` library is a govax extension (VMS
has neither the file nor a way to search one). It costs almost nothing,
because a macro library in `internal/asm` is just "give me the text of
macro NAME". An `.MLB` answers from its index, and a `.MAR` answers from
the definitions it holds.

The lookup is LINK's: `SYS$LIBRARY:STARLET.MLB` on a mounted volume, then
`STARLET.MLB` in the host library directory (`vax.link.library`), then
`STARLET.MAR` in the same two places. A missing library isn't an error
until a macro is needed and nobody defines it (then it's MACRO's own
"unknown opcode" error, with a note that no system library was found).

**A suggestion on top of that (open question 1):** a fresh clone has no
STARLET.MLB, since it's licensed. govax could embed a small `STARLET.MAR`
of its own as the last fallback, with govax-written definitions of the
common macros (`$EXIT_S`, `$QIOW_S`, `$ASSIGN_S`, `$DASSGN_S`, `$SSDEF`,
`$IODEF`, and perhaps `$FAB`/`$RAB`/`$OPEN`/`$GET`/`$PUT`). They would be
written from the *System Services* and *RMS* reference manuals' argument
lists, not copied from STARLET, and checked against the real ones by
expanding both. That's subtask 9, and it's optional.

### Macro libraries in `internal/asm`

`internal/asm` gets a small `MacroLibrary` interface (look up a macro
name, get its definition's lines), with two implementations: one over an
`*lbr.Library` (a `.MLB`) and one over source text holding `.MACRO`
definitions (a `.MAR`). `internal/lbr` is a leaf package, so `asm`
importing it is fine. The console decides which files to open and hands
the assembler libraries plus a resolver for `.LIBRARY` names, as it
already hands it an `.INCLUDE` resolver.

The search order is MACRO's: the `.LIBRARY` libraries in the reverse of
the order they were named, then the command line's `/LIBRARY` files (also
reversed), then `STARLET.MLB`. A name is looked up in the libraries only
when it isn't a directive, a macro already defined, or an opcode, except
that `.MCALL` looks it up regardless (which is how a library macro can
replace an opcode).

### Expansion: substitute text, then assemble it as ordinary lines

A macro definition is kept as its raw source lines, comments and case
included, because substitution happens everywhere (strings and comments
too). A call parses its actual arguments, binds them to the formal ones,
substitutes into the raw lines, and feeds the result through the same
statement path as source lines (`statement` → `assembleStatement`), so
preprocessing (uppercasing, comment stripping, continuation lines) runs
on the expanded text exactly as it would on text typed in the source.

The line loop becomes a stack of *sources*: the main file, an included
file, a macro expansion, or a repeat block's expansion. Each knows what to
put in an error's location. A `.MACRO` seen in any source starts a
definition that swallows the following lines of that source until its
matching `.ENDM` (counting nested `.MACRO`s); while a definition is being
collected nothing in it is assembled, not even conditionals. `.MEXIT`
ends the innermost expansion, and closes any conditional blocks opened
inside it. Repeat blocks collect their lines to `.ENDR` the same way.

### Where the manual and STARLET disagree with govax today

- Macros take precedence over opcodes (the manual's `.MACRO` note 1).
  Directives are still tried first, since every directive name starts
  with `.` in the MACRO dialect.
- `.ERROR`, `.WARN`, and `.PRINT` take their message from the comment. The
  console dialect's `.PRINT "text"` stays as it is (`byDialect`).
- Created local labels count up from `30000$`, as the manual says.

## References

- *VAX MACRO and Instruction Set Reference Manual* (OpenVMS VAX 7.3),
  chapter 4 (macro arguments and string operators) and the macro
  directives in chapter 6 (`~/Documents/Technical Doc/VMS/`).
- `testdata/vmslib/starlet.mlb`: the real library, read by tests that skip
  without it.
- `vmssrc_archive/v73/lbr/lis/` and `librar/lis/`: the librarian, whose
  listings and `lbr.sdl` describe the `.MLB` file format (already used by
  `internal/lbr`).

## Subtasks

1. **Macro definitions and calls.** `.MACRO`/`.ENDM` definitions (nested
   definitions, a labeled `.ENDM`, a continued `.MACRO` line), the macro
   table (redefinition, `.MDELETE`), call recognition ahead of opcodes,
   actual-argument parsing (positional, keyword, defaults, `<...>` and
   `^x...x` delimiters, nesting), substitution with `'` concatenation, the
   source stack with `.MEXIT`, and error locations that name the call.
   Symbols may contain `.`. Both dialects.
2. **Argument features and attribute directives:** created local labels
   (`?L1`), `\symbol` values, `.NARG`, `.NCHR`, `.NTYPE`, and the string
   operators `%LENGTH`, `%LOCATE`, and `%EXTRACT`.
3. **Repeat blocks:** `.REPEAT`/`.REPT`, `.IRP`, `.IRPC`, `.ENDR`, and
   `.MEXIT` inside a repeat block.
4. **Message and listing directives:** `.ERROR`, `.WARN`, and `.PRINT` in
   MACRO's form (message from the comment; `.ERROR` is an assembly error,
   `.WARN` a warning, `.PRINT` an informational message), and the
   listing-control directives (`.LIST`, `.NLIST`, `.SHOW`, `.NOSHOW`,
   `.CROSS`, `.NOCROSS`, `.PAGE`) accepted and ignored.
5. **Macro libraries in `internal/asm`:** the `MacroLibrary` interface,
   the `.MLB` and `.MAR` implementations, `.MCALL`, `.LIBRARY` (through a
   resolver, default type `.MLB`), and the automatic search for an
   undefined opcode. Acceptance: every one of STARLET.MLB's 1529 macros
   loads, and a set of calls (`$EXIT_S`, `$QIOW_S`, `$FAB`, `$RAB`,
   `$SSDEF`, `$IODEF`) expand and assemble.
6. **The MACRO command:** find STARLET (`.MLB`, then `.MAR`, as above),
   resolve `.LIBRARY` names relative to the source like `.INCLUDE`, and
   take command-line libraries in VMS's form, `MACRO PROG+MYLIB/LIBRARY`
   (open question 2). The `cmd/govax macro` subcommand follows.
7. **Fixtures** (needs the user). New `testdata/mar` fixtures: one of
   user-defined macros (every argument form, created labels, repeat
   blocks, string operators, `.NARG`/`.NCHR`/`.NTYPE`) and programs that
   call system macros (`$QIOW_S` "hello", and an RMS `$FAB`/`$RAB` file
   copy). The user assembles them with real MACRO; govax's objects must
   match, and the programs must link and run under govax's `LINK`/`RUN`.
8. **Clean-up and docs:** `PLAN.md`, `CLAUDE.md`, `DEVIATIONS.md`, the
   MACRO help topic, and this document's closing entry.
9. **(Optional) govax's own STARLET.MAR** for clones without the licensed
   library (open question 1).

## Open questions

1. **A built-in fallback STARLET.MAR** (subtask 9): worth doing, and if so,
   which macros? Without it, a MACRO program that calls a system macro
   needs the user's own STARLET.MLB.
2. **Command-line libraries.** VMS writes them as `MACRO PROG+LIB/LIBRARY`.
   Is that form wanted, or a govax-style `/LIBRARY=(file,...)` qualifier,
   or both? (govax's DCL grammar takes the source as one string, so the
   `+` form would be split by the MACRO command itself.)
3. **The host library setting.** LINK's host directory setting is
   `vax.link.library`. MACRO looking there too is simplest; the name then
   reads oddly. Keep it, or add a general `vax.library` that both use?
4. **Should the console dialect get macros too?** Settled: the definition
   side yes (it's free), library search only if it's free.

## Progress Log

### 2026-09-30 — Planned

- Created from Phase 27's "later sub-phases" when that phase closed.

### 2026-09-30 — Subtasks written; STARLET.MLB surveyed

- Copied `STARLET.MLB` off the system disk (`COPY/BINARY` from
  `DUA0:[VMS$COMMON.SYSLIB]`) into `testdata/vmslib/`. `internal/lbr`
  reads it unchanged, and every module's text was dumped to survey what
  the system macros use (see "What the real STARLET.MLB shows").
- Wrote the design and subtasks above.

### 2026-09-30 — Subtask 1: macro definitions and calls

- **The source stack** (`internal/asm/source.go`). The line loop
  (`runSource`) now runs one *source* at a time on a stack of
  `sourceFrame`s: the program, an `.INCLUDE` file, or a macro's
  expansion. `.INCLUDE`'s old `includeLines`/`includeDepth` bookkeeping
  is gone. `located` puts every frame's location on an error, so an
  error in a nested expansion reads
  `line 9: in expansion of macro OUTER, line 2: in expansion of macro
  INNER, line 1: ...` (`ExpansionError`). The console dialect, which
  stops at the first error, builds the same text by having each frame
  wrap its own location on the way out.
- **Definitions** (`macros.go`). `.MACRO` reads the name and formal
  arguments (`parseFormals`: `NAME`, `NAME=default`, `?NAME`), then
  `collectDefinition` takes the source's raw lines (comments and case
  kept) until the matching `.ENDM`. Nested blocks are counted:
  `.MACRO`, `.IRP`, `.IRPC`, `.REPEAT`, and `.REPT` open one, `.ENDM` or
  `.ENDR` closes one. STARLET.MLB ends some `.IRP` blocks with `.ENDM`
  (`$$POS`, for one), which VAX MACRO evidently accepts from MACRO-11. A
  label on the final `.ENDM` line stays in the body. `.ENDM NAME` must
  name its macro. A definition that its source doesn't finish is
  `VAX_NOENDM`.
- **Calls.** A statement whose first word is a macro's name is a call,
  tried after directives and before instructions. `parseActuals` reads
  positional and keyword arguments (keyword only when the name is a
  formal argument, else `LOCATION=12` is one positional argument, as the
  manual's `RESERVE` example needs), with `<...>` (nesting) and `^x...x`
  delimiters. `bind` applies the manual's rules (later wins, defaults for
  blanks, too many is `VAX_TOOMNYARGS`), and `substitute` replaces whole
  names case-blind everywhere in the line, dropping a concatenating
  apostrophe on either side. `.MEXIT` ends the innermost expansion and
  closes the conditional blocks it opened; `.MDELETE` deletes macros.
  The expansion depth limit is 1000.
- **Symbols may contain `.`** (`isSymbolChar`), and may start with one
  (`.LEN`) when a non-digit symbol character follows (`isSymbolStart`).
  A directive's name isn't an assignment target, so `.ASCII =abc=` is
  still the directive.
- Both dialects get macros; the console's work without the `.`, and at
  the interactive prompt (`AssembleLine` collects definition lines).
- New status codes: `VAX_NOENDM`, `VAX_ENDMNAME`, `VAX_NOTINDEF`,
  `VAX_NOTINMACRO`, `VAX_MACRONAME`, `VAX_MACRODEPTH`, `VAX_TOOMNYARGS`,
  `VAX_BADFORMAL`.
- Tests (`macros_test.go`) follow the manual's chapter 4 examples
  (`STORE`, `REPEAT`, `CNTRPT`/`CNTRPT2`, `CONCAT`, `POSITIVE`, `USERDEF`,
  `P0L0`), plus a macro that defines a macro with `PREFIX''SYM`, error
  locations in both dialects, and dotted symbols. A scratch check
  collected all 1529 of STARLET.MLB's definitions with no errors (about
  30 ms); subtask 5 makes that a permanent test.
- Noted, not changed: a forward label difference such as
  `.BYTE LAB2-LAB1-1` becomes a linker relocation (Phase 27's one-pass
  design), where real MACRO computes it. The object is still correct; the
  subtask 7 fixtures will show whether real MACRO's object differs enough
  to matter.
- Known gap for later subtasks: `preprocessLine` treats `'` as a quote,
  so a call argument such as `<P'S AND Q'S>` isn't uppercased between
  the apostrophes, and a `;` inside `<...>` still starts a comment.
