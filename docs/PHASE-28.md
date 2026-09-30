# Phase 28: The MACRO-32 macro facility

## Goal

Add MACRO-32's macro facility to the MACRO dialect of `internal/asm`, so that
ordinary VMS MACRO programs assemble with govax's `MACRO` command. Almost
every real program calls system macros (`$EXIT_S`, `$QIOW_S`, `$FAB`, `$RAB`,
and so on) from `SYS$LIBRARY:STARLET.MLB`. Without the macro facility, govax
can only assemble programs that call `SYS$...` entry points directly, as the
Phase 27 fixtures do.

Split out of Phase 27's "later sub-phases" (docs/PHASE-27.md, subtask 12).

**Status: in progress (subtasks 1-8 done; next, subtask 9).**

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
  the host directory the `vax.link.library` setting names (to become
  `vax.library`, shared with MACRO; subtask 9).

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

### Where the system macros come from (decided)

Macros always come from `.MLB` macro libraries, as on VMS. `STARLET.MLB` is
looked for as LINK looks for its libraries: `SYS$LIBRARY:STARLET.MLB` on a
mounted volume, then `STARLET.MLB` in the host library directory (the
`vax.library` setting). When neither exists (a fresh clone has no licensed
STARLET.MLB), govax uses its own `STARLET.MLB`, kept in the bootdata file
system (`internal/bootdata/files`).

govax's STARLET.MLB is made from whole cloth (decided 2026-09-30, revising
the earlier plan of a `STARLET.MAR` read as text). Its source is a
govax-written `STARLET.MAR` of macro definitions, starting with `$EXIT_S`
and growing over time. The definitions are written from the *System
Services* and *RMS* reference manuals' argument lists, not copied from
STARLET, and checked against the real ones by expanding both. The source
is made into the library by govax's own librarian (the `internal/lbr`
writer and the `LIBRARY` command), so there is only one kind of macro
library to search, and the tool that builds govax's library is the one
users build theirs with.

The `.MLB` is generated from the source (`go generate`) and committed next
to it in bootdata, and a test rebuilds it and checks that the two match,
so the source and the library can't drift apart.

### A librarian (decided 2026-09-30)

`internal/lbr` gains a writer: create a library, insert, replace, and
delete modules, and write the file, with its B-tree indexes. It writes the
V3 format real LIBRARIAN writes (without DCX data reduction, which is
optional in the format), for macro libraries (one module per macro, the
index keyed by macro name) and object libraries (a module per object
module, with a second index of the global symbols each defines). A console
`LIBRARY` command, in the style of VMS's, drives it, so users can make
their own `.MLB` and `.OLB` files, for MACRO's `/LIBRARY=` and for LINK.
Real VMS's `LIBRARY/LIST` and MACRO/LINK reading govax's libraries are
part of the fixtures (subtask 10).

### Macro libraries in `internal/asm`

`internal/asm` gets a small `MacroLibrary` interface (look up a macro
name, get its definition's lines), implemented over an `*lbr.Library`.
`internal/lbr` is a leaf package, so `asm` importing it is fine. The
console decides which files to open and hands the assembler its libraries
plus a resolver for `.LIBRARY` names, as it already hands it an `.INCLUDE`
resolver.

The search order is MACRO's: the `.LIBRARY` libraries in the reverse of
the order they were named, then the `/LIBRARY=` files (also reversed),
then `STARLET.MLB`. A name is looked up in the libraries only when it
isn't a directive, a macro already defined, or an opcode, except that
`.MCALL` looks it up regardless (which is how a library macro can replace
an opcode).

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

1. **Done.** **Macro definitions and calls.** `.MACRO`/`.ENDM` definitions
   (nested definitions, a labeled `.ENDM`, a continued `.MACRO` line), the
   macro table (redefinition, `.MDELETE`), call recognition ahead of
   opcodes, actual-argument parsing (positional, keyword, defaults, `<...>`
   and `^x...x` delimiters, nesting), substitution with `'`
   concatenation, the source stack with `.MEXIT`, and error locations that
   name the call. Symbols may contain `.`. Both dialects.
2. **Done.** **Argument features and attribute directives:** created local
   labels (`?L1`), `\symbol` values, `.NARG`, `.NCHR`, `.NTYPE`, and the
   string operators `%LENGTH`, `%LOCATE`, and `%EXTRACT`.
3. **Done.** **Repeat blocks:** `.REPEAT`/`.REPT`, `.IRP`, `.IRPC`, `.ENDR`, and
   `.MEXIT` inside a repeat block.
4. **Done.** **Message and listing directives:** `.ERROR`, `.WARN`, and `.PRINT` in
   MACRO's form (message from the comment; `.ERROR` is an assembly error,
   `.WARN` a warning, `.PRINT` an informational message), and the
   listing-control directives (`.LIST`, `.NLIST`, `.SHOW`, `.NOSHOW`,
   `.CROSS`, `.NOCROSS`, `.PAGE`) accepted and ignored.
5. **Done.** **Macro libraries in `internal/asm`:** the `MacroLibrary` interface
   over `*lbr.Library`, `.MCALL`, `.LIBRARY` (through a resolver, default
   type `.MLB`), and the automatic search for an undefined opcode.
   Acceptance: every one of STARLET.MLB's 1529 macros loads, and a set of
   calls (`$EXIT_S`, `$QIOW_S`, `$FAB`, `$RAB`, `$SSDEF`, `$IODEF`) expand
   and assemble.
6. **Done.** **The `internal/lbr` writer:** create a library, insert, replace, and
   delete modules, and write the V3 format with its B-tree indexes: macro
   libraries (one module per `.MACRO`, keyed by name) and object libraries
   (module-name index, plus the global symbols from each object's GSD).
   Round-trip tests through `lbr.Open`; the real STARLET.MLB's modules
   rewritten into a new library must read back unchanged.
7. **Done.** **The `LIBRARY` command,** in VMS's style: `/CREATE`, `/INSERT`,
   `/REPLACE`, `/DELETE=`, `/EXTRACT=` with `/OUTPUT=`, `/LIST`, and
   `/MACRO` or `/OBJECT` (the default, as on VMS) choosing the type and the
   default file type (`.MLB` or `.OLB`). Host files and volume files, by
   the rules MACRO and LINK use. The `cmd/govax library` subcommand
   follows.
8. **Done.** **govax's own STARLET.MLB** in bootdata: a govax-written
   `STARLET.MAR` (starting with `$EXIT_S`) and the `.MLB` generated from
   it, with a test that they match.
9. **The MACRO command:** STARLET.MLB found as above (volume, then
   `vax.library`, then bootdata), `.LIBRARY` names resolved relative to the
   source like `.INCLUDE`, and command-line libraries with a govax-style
   `/LIBRARY=(file[,...])`. The host library setting `vax.link.library`
   becomes `vax.library`, shared by MACRO and LINK (as VMS's `SYS$LIBRARY`
   is), still reading the old name when the new one isn't set. The
   `cmd/govax macro` subcommand follows.
10. **Fixtures** (needs the user). New `testdata/mar` fixtures: one of
    user-defined macros (every argument form, created labels, repeat
    blocks, string operators, `.NARG`/`.NCHR`/`.NTYPE`) and programs that
    call system macros (`$QIOW_S` "hello", and an RMS `$FAB`/`$RAB` file
    copy). The user assembles them with real MACRO; govax's objects must
    match, and the programs must link and run under govax's `LINK`/`RUN`.
    Also govax-made `.MLB` and `.OLB` libraries, checked with real
    `LIBRARY/LIST` and used by real MACRO and LINK.
11. **Clean-up and docs:** `PLAN.md`, `CLAUDE.md`, `DEVIATIONS.md`, the
    MACRO and LIBRARY help topics, and this document's closing entry.

## Open questions

All settled (2026-09-30):

1. **A built-in fallback:** yes, as a govax-made STARLET.MLB in bootdata,
   built from a govax-written source starting with `$EXIT_S` and added to
   over time (subtask 8). That needs a librarian: an `internal/lbr` writer
   and a `LIBRARY` command (subtasks 6 and 7), which users can use for
   their own `.MLB` and `.OLB` libraries too.
2. **Command-line libraries:** a govax-style `/LIBRARY=(file,...)`
   qualifier, not VMS's `PROG+LIB/LIBRARY`.
3. **The host library setting:** generalized to `vax.library`, shared by
   MACRO and LINK.
4. **Should the console dialect get macros too?** The definition side yes
   (it's free), library search only if it's free.

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

### 2026-09-30 — User decisions

- The built-in fallback STARLET.MAR is wanted, starting with `$EXIT_S`.
- Command-line libraries take a govax-style `/LIBRARY=` qualifier.
- `vax.link.library` becomes `vax.library`, shared by MACRO and LINK.
- Subtasks 6 and 9 and the open questions are updated to match.

### 2026-09-30 — Revised: the fallback is a govax-made STARLET.MLB

- The user revised the fallback: rather than reading a `STARLET.MAR` as
  text, govax keeps its own `STARLET.MLB` in bootdata, so macros always
  come from a macro library. Building it needs a librarian, so an
  `internal/lbr` writer and a VMS-style `LIBRARY` command join the phase;
  users get them for their own `.MLB` and `.OLB` libraries too.
- The subtasks are renumbered: 6 is the `lbr` writer, 7 the `LIBRARY`
  command, 8 govax's STARLET.MLB, 9 the MACRO command, 10 the fixtures,
  11 the docs. The design sections above are rewritten to match.

### 2026-09-30 — Subtask 2: argument features and attribute directives

- **Created local labels.** A blank `?NAME` formal argument gets the next
  created label, `30000$` onward (`Assembler.createdLabel`, reset by each
  `Assemble`). A given one is used as written.
- **`\expression`** passes the expression's value, in decimal
  (`scanActual`); it must be absolute and already defined.
- **`.NARG`** (positional arguments of the innermost call, null ones
  counted, keyword ones not), **`.NCHR`** (a string's length, read like a
  macro argument), and **`.NTYPE`** (`macroattr.go`).
- **`.NTYPE` doesn't assemble anything.** Operand assembly writes bytes
  as it parses, so `operandType` classifies the operand's syntax itself,
  choosing the short-literal/immediate form and the displacement size by
  the same rules operand assembly uses (a value known now, else the
  default size). The manual gives the PC-based forms their own modes (0
  literal, 1 immediate, 2 absolute, 3 general); govax returns those in
  bits 4-7 with register 15 (`^X1F` and so on). The register field of the
  literal (0), and relative mode's chosen size, are the parts to confirm
  against real MACRO in the fixtures (subtask 10). An indexed operand
  gives the base's mode in the high byte and `^X4x` in the low byte.
- **String operators** are evaluated as each line of an expansion is
  reached (`stringOperators`, called by `runSource` for a macro's frame),
  not when arguments are substituted: the manual's `RESERVE` sets a
  symbol with `%LOCATE` on one line and uses it in `%EXTRACT` on the
  next. Lines a conditional leaves out aren't evaluated, and neither is a
  comment. `%LOCATE`'s start and `%EXTRACT`'s position and length take a
  decimal number or a defined absolute symbol.
- New status code `VAX_BADOPERATOR` (wrong argument count).
- Tests (`macroattr_test.go`) are the manual's `POSITIVE`, `TESTDEF`,
  `CNT_ARG`, `CHAR`, `PUSHADR`, `BIT_NAME`, and `RESERVE`, a table of
  `.NTYPE` values for every operand form, and the operators' error cases.
  The manual says `.NCHR` of `<14, 75.39 4>` is 12, but the string as
  printed has 11 characters; the test uses 11.
- A scratch check defined all of the real STARLET.MLB's macros and called
  `$EXIT_S` and `$QIOW_S`. Both expanded to what real MACRO generates
  (`$QIOW_S`'s `$PUSHADR` chose `PUSHAQ (R3)+` for `IOSB=(R3)+` through
  `.NTYPE`, and `PUSHAB` for a label).

### 2026-09-30 — Subtask 3: repeat blocks

- **`.REPEAT`/`.REPT`, `.IRP`, `.IRPC`** (`repeat.go`). The directive
  starts collecting the block's range, as `.MACRO` starts a body: a
  `definition` now holds either a macro or a `repeatBlock`, and
  `collectDefinition` counts nested blocks the same way. At the matching
  `.ENDR` (or `.ENDM`, as in STARLET.MLB's `$$POS`, now outside a macro
  too) each repetition is assembled as a `sourceRepeat` frame, with the
  formal argument substituted by `substitute`, apostrophes and all.
  `.REPEAT`'s count must be absolute and defined, and zero or less
  assembles nothing. `.IRP`'s list is read by `parseActuals` (commas make
  null arguments, `<...>` and `^x...x` delimit, `\symbol` passes a value);
  an empty list assembles nothing. `.IRPC` repeats once per character.
- **`.MEXIT`** ends the innermost expansion, macro or repetition; in a
  repetition it ends that repetition and the ones after it (the manual's
  `.MEXIT` notes 1 and 2).
- **Error locations** name the block's directive line and the
  repetition: `line 2: in repetition 1 of .IRP, line 2: ...`
  (`ExpansionError` gained `Block` and `Repetition`). String operators are
  evaluated in a repetition's lines as in a macro's. The nesting limit
  counts repeat blocks along with macro expansions.
- New status codes `VAX_NOENDR` and `VAX_NOTINREPEAT` (a stray `.ENDR`);
  `VAX_NOTINMACRO`'s text now says "macro expansion or repeat block".
- Both dialects, and at the console's interactive prompt.
- Tests (`repeat_test.go`) are the manual's `COPIES`, `CALL_SUB`, and
  `HASH_SYM`, plus argument forms, concatenation, nesting (and a block that
  defines macros), `.MEXIT`, `.ENDM` endings, errors, and error locations
  in both dialects.
- A scratch check against the real STARLET.MLB: `$$R_VBFSET FAB,<GET,PUT,DEL>`
  (the `.IRP` behind `$FAB`'s and `$RAB`'s bit-set arguments) gives 7, and an
  unknown bit reaches its `.ERROR` branch. Found for later subtasks:
  `$DEFINI`/`$DEFEND` need `.NOCROSS`/`.CROSS` (subtask 4), and
  `$$R_TABINIT`'s `.IIF NE .&3, ...` (an alignment check on a relocatable
  `.`) is rejected as `VAX_RELEXPR`; real MACRO evidently accepts it, so
  that has to be settled for subtask 5's `$FAB`/`$RAB` acceptance.

### 2026-09-30 — Subtask 4: message and listing directives

- **`.ERROR`, `.WARN`, `.PRINT`** (`message.go`), in MACRO's form
  `[expression] ;comment`. The message is the expression's value in
  decimal (left out when zero, as the manual says) and then the comment,
  trimmed of blanks and of the closing `;` a macro library's comments end
  with. `.ERROR` is an assembly error (`VAX_GENERR`, "Generated ERROR:"),
  `.WARN` a warning (`VAX_GENWRN`, "Generated WARNING:"), and `.PRINT` an
  informational message, kept apart from the warnings in
  `Assembler.Messages()` so it doesn't make the object's severity a
  warning. The MACRO command displays each as it is, with no prefix (see
  the next entry).
- **The comment reaches the directive** through preprocessing:
  `preprocessComment` is `preprocessLine` returning the comment too, and
  `statement` keeps it (a continued statement's is its last line's) in
  `a.comment`. Arguments are already substituted into it, since
  substitution happens on the raw lines, so `.ERROR ; ... : ORG;` names the
  argument. `.IIF cond, .ERROR ;comment` works as in STARLET.MLB.
- **Listing control** (`.LIST`, `.NLIST`, `.SHOW`, `.NOSHOW`, `.CROSS`,
  `.NOCROSS`, `.PAGE`) is accepted, arguments and all, and ignored.
- Both dialects get `.ERROR`, `.WARN`, and the listing directives; the
  console's `.PRINT` stays eVAX's (`byDialect`).
- Tests (`message_test.go`) are the manual's `.ERROR`, `.WARN`, and
  `.PRINT` examples, a macro whose argument is substituted into the
  message, skipped branches, continuation, errors, the listing directives,
  and the console dialect.
- A scratch check against the real STARLET.MLB: `$SSDEF`, `$IODEF`, and
  `$FABDEF` twice (the second expanding to nothing through `$DEFEND`) now
  assemble, and `$FAB FAC=<GET,BOGUS>` reports `$$R_VBFSET`'s
  "Generated ERROR: UNDEFINED BIT VALUE CODE: BOGUS".

### 2026-09-30 — Real MACRO's behavior decides (user decision)

- The user settled subtask 3's open question for every case like it:
  where real VMS MACRO and the manual (or govax) differ, govax does what
  real MACRO does.
- **Conditionals on relocatable values.** The manual says an `.IF`/`.IIF`
  expression must be absolute, but `$$R_TABINIT`, in every `$FAB` and
  `$RAB`, checks alignment with
  `.IIF NE .&3, .print ;%MACRO-I-GENINFO, Generated INFO: RMS BLOCK NOT
  LONGWORD ALIGNED;`. Real MACRO assembles every `$FAB`, and prints this
  only for a misaligned block, so it evidently tests a relocatable value
  by its offset in its psect. `condition` now does the same (an undefined
  or external symbol is still an error).
- **`.PRINT` has no prefix of its own.** That same comment carries its own
  `%MACRO-I-GENINFO` prefix, so real MACRO must display `.PRINT`'s message
  bare. `Messages()` now returns plain strings, and `VAX_GENPRINT` and
  `CLI_ASMMESSAGE` are gone.
- The fixtures (subtask 10) should include a misaligned `$FAB` to confirm
  both.
- A scratch check against the real STARLET.MLB: `$FAB FNM=<X.DAT>,
  FAC=<GET,PUT>` then `$RAB FAB=FAB1` assembles to 148 bytes (80 + 68),
  and a `$FAB` after a `.BYTE` gets the alignment message.

### 2026-09-30 — Subtask 5: macro libraries in `internal/asm`

- **`MacroLibrary`** (`maclib.go`): look up a macro by name and get its
  module's lines. `NewMacroLibrary` adapts an `*lbr.Library` (a macro
  library only). Tests use a map-backed one, so the mechanics are tested
  without the licensed STARLET.MLB.
- **Loading a library macro** runs its module's lines as a source of their
  own (`sourceLibrary`), so the `.MACRO` line (continued, as `$FAB`'s is)
  and `.ENDM` take the same path as a definition in the program. It
  happens in the middle of the statement that needs the macro, so that
  statement's comment and continuation state are saved around it. An
  error in a module reads `in library definition of macro NAME, line n:`.
- **Search order** is MACRO's: `.LIBRARY`'s libraries, the last named
  first, then the caller's (`SetMacroLibraries`, searched in the order
  given; subtask 9 passes `/LIBRARY=`'s files, reversed, then
  STARLET.MLB).
- **Automatic search** (`assembleLibraryCall`): a statement's first word
  that isn't a directive, a defined macro, or an opcode is looked up, and
  if a library has it, the macro is defined and called. Later calls use
  the definition without searching again.
- **`.MCALL`** loads each named macro whether or not it's defined, so it
  can replace a source definition or an opcode (the manual's `.MCALL
  INSQUE`). A name no library has is `VAX_UNDEFMACRO`.
- **`.LIBRARY /file-spec/`** hands the name, as written (preprocessing
  keeps its case, as for `.IDENT`), to the resolver
  (`SetLibraryResolver`), which applies the `.MLB` default type. No
  resolver is `VAX_NOLIBRESOLVER`; its failure is `VAX_LIBRARY`.
  `readDelimited` now reads both `.IDENT`'s string and this one.
- Both dialects search libraries when they're given any; the console
  isn't given any yet (the "only if free" of the scope).
- **Found on the way:** `.BLKA`, `.BLKG`, `.BLKH`, `.BLKO`, and `.BLKQ`
  were missing (a program reserving an IOSB with `.BLKQ` failed), and
  `.BLKx` with no count now reserves one unit, as the manual says.
- **Acceptance** (tests that skip without `testdata/vmslib/starlet.mlb`):
  all 1529 of STARLET.MLB's macros load through `.MCALL`, each defining
  the macro its module is named for (about 30 ms). A program calling
  `$SSDEF`, `$IODEF`, `$FAB`, `$RAB`, `$QIOW_S`, and `$EXIT_S` through the
  automatic search assembles into an object with no warnings. Its code is
  what real MACRO generates for these calls: `$QIOW_S`'s argument list
  pushed in reverse and `CALLS #12,G^SYS$QIOW`, then `PUSHL #1` and
  `CALLS #1,G^SYS$EXIT`. The FNM string lands in `$RMSNAM`.

### 2026-09-30 — Subtask 6: the `internal/lbr` writer

- **`Builder`** (`write.go`): `Create(type)` starts an empty library with
  LIBRARY/CREATE's defaults for the type (key size, module header user
  data size, index case options, from `librar/lis/database.lis`), and
  `Edit(*Library)` loads an existing one, its modules in data order. Then
  `Insert`, `Replace` (the new data goes last, as LIBRARIAN writes it at the
  end), and `Delete`, with index 2's symbols kept per module: a symbol
  another module defines, a duplicate module, an over-long key, or a
  record over 2048 bytes is an error, and a failed replacement leaves the
  old module. Callers set the creation, update, and insertion times
  (`vmsdef.Time` converts a `time.Time`), so output is reproducible (subtask 8's
  generated STARLET.MLB needs that).
- **`Bytes` lays the file out as the librarian would** by creating the
  library and inserting the modules in order (`lbr/lis/openclose.lis`'s
  `prealloc_index`, `getput.lis`'s `write_record`, `index.lis`,
  `subs.lis`): the header, then the preallocated index blocks (the
  default's 128 modules, plus 512 globals for an object library, at
  `500 / (keysize + 6)` keys a block, or more if the index needs them),
  with the unused ones on the free list through each block's first
  longword; then one chain of data blocks holding every module's header
  record, records, and end-of-text record. Index blocks are full B-tree
  blocks built bottom-up, each upper entry naming its child and the
  child's highest key, and each block's parent VBN set. A data block's
  `DATA$B_RECS` counts the records with any part in it, following
  `write_record` exactly, down to a record ending at a block's end
  (the next block is started, with no records yet) and a length word
  filling a block. The header's counts (`IDXBLKS`, `IDXCNT`, `MODCNT`,
  `MODHDRS`, `IDXOVH`, `NEXTRFA`, `NEXTVBN`, `FREIDXBLK`, `FREEIDX`,
  `HIPREAL`, `HIPRUSD`) are set to match. No DCX data reduction and no
  update history records; `LHD$W_MAXLUHREC` is 20, LIBRARIAN's default.
- **The reader** now reads an index with no keys (VBN 0 in its
  descriptor, as in a new library) and the header's history limit.
- **Input rules** (`input.go`), from `librar/lis/inputmac.lis` and
  `inputobj.lis`:
  - `MacroModules`: a module is an outermost `.MACRO` line through its
    matching `.ENDM` (nested `.MACRO`s counted, an unnamed `.ENDM` inside a
    repeat block ending the block, as STARLET.MLB's `$$POS` needs); lines
    outside macros are skipped; the name is upper-cased unless the index is
    case-sensitive. The line scan is `scan_line`'s, quirks included (a
    label is skipped; `.ENDM;X` names `X`). `/SQUEEZE` (LIBRARIAN's
    default) leaves the `.MACRO` line and `.ERROR`/`.WARN`/`.PRINT`/`.IIF`
    lines alone, keeps empty lines as empty records, and otherwise cuts
    each line at its *last* semicolon and drops trailing blanks, dropping a
    line that ends up empty. So `.ASCII /a;b/` with no comment becomes
    `.ASCII /a`, as on VMS; govax's STARLET.MAR (subtask 8) must keep `;`
    out of its strings. Mismatched `.ENDM` names and `.ENDR`s are
    warnings; no `.MACRO` at all, an unfinished macro, or an over-long name
    ends the file, keeping the modules before it.
  - `ObjectModules`: one module per main header through end of module
    (EOM or EOMW); the key is the module name; index 2 gets the non-weak
    symbol definitions and every entry point and procedure, never
    module-local ones. The header's user data is the `MHD$B_OBJSTAT` byte
    (`OBJTIR` if the module has TIR records, `SELSRC` for
    `/SELECTIVE_SEARCH`) and the counted ident, zero-filled to 33 bytes, as
    in the real STARLET.OLB. Checked against it: each module's reference
    count is one more than its symbols. govax doesn't insist on an LNM
    header record, which LIBRARIAN's sequence check does.
- `internal/lbr` now imports `internal/obj` (for `ObjectModules`); it is
  otherwise still a leaf.
- Tests (`write_test.go`): a layout checker that re-derives what `Open`
  doesn't check (each index block's parent, the free list, the data
  chain, and every block's record count, by reading every record in
  order); new libraries of each type against `prealloc_index`'s numbers;
  macro source splitting and squeezing; `scan_line` cases; 400 macro
  modules whose record lengths hit every word offset of a block; a
  three-level index of 6000 long keys (more blocks than are
  preallocated); an object library from `obj.Builder` modules;
  insert/replace/delete rules; and a rewritten library rewriting to the
  same bytes.
- **Acceptance** (skips without `testdata/vmslib`): the real STARLET.MLB
  (data-reduced), STARLET.OLB (data-reduced), and IMAGELIB.OLB, loaded by
  `Edit` and written by `Bytes`, read back with the same header settings,
  the same keys in both indexes naming the same modules, and every
  module's header and records unchanged; rewriting the result gives the
  same bytes.

### 2026-09-30 — Subtask 7: the `LIBRARY` command

- **`LIBRARY library [input,...]`** (`internal/console/library.go`, the
  `library` verb in `console.dcl`), in LIBRARIAN's style: `/CREATE`,
  `/INSERT`, `/REPLACE`, `/DELETE=(...)`, `/EXTRACT=(...)` with `/OUTPUT=`,
  `/LIST[=file]` with `/FULL`, `/NAMES`, and `/WIDTH=`, `/MACRO` or
  `/OBJECT` (the default), `/[NO]SQUEEZE`, `/SELECTIVE_SEARCH`, and `/LOG`.
  The library and the inputs each take COPY's parameter-scoped `/HOST`.
  - With input files and no operation named, the inputs replace modules
    of the same names, LIBRARIAN's default `/REPLACE`; with `/INSERT` or
    `/CREATE`, a module already in the library is a warning and is left
    alone. Macro sources (`.MAR`) go into a macro library, object files
    (`.OBJ`) into an object library, through subtask 6's `MacroModules`
    and `ObjectModules`.
  - `/DELETE=` runs first, then the inserts; `/EXTRACT=` and `/LIST` see
    the library as changed. Module names in `/DELETE=` and `/EXTRACT=` may
    hold `*` and `%` (`lbr.Library.Match`, `lbr.Builder.Match`), upper-cased
    unless the index compares case as it is (an object library's does).
    A name matching nothing is a warning.
  - An existing library keeps its own type; `/MACRO` on an object library
    (or the reverse) is an error. `/MACRO` and `/OBJECT` together, and
    `/INSERT` with `/REPLACE`, are refused by the grammar.
  - Files are found as MACRO and LINK find theirs (`rms.Session.Locate`):
    each input beside the one before it, default type `MAR` or `OBJ`;
    `/OUTPUT=` and a `/LIST=` file beside the library, default types `OBJ`
    or `MAR` (by the library's type) and `LIS`. The library's default
    type is `OLB`, or `MLB` with `/MACRO`.
  - **One difference from LIBRARIAN, on purpose:** LIBRARIAN updates a
    library in place. govax builds the whole library in memory and writes
    it only when every step has succeeded, so a failure (an unfinished
    macro, a missing input, a symbol another module defines) leaves the
    library as it was. (As first written, a changed volume library got a
    new version; see the next entry.)
  - New status codes: `CLI_LIBRARY` (error), `CLI_LIBWARNING` (warning),
    and `/LOG`'s `CLI_LIBINSERTED`, `CLI_LIBREPLACED`, and
    `CLI_LIBDELETED`, worded as LIBRARIAN's `INSERTED`, `REPLACED`, and
    `DELETED` messages. `/LOG` prints once the library is written, so the
    messages can name the version written.
- **The listing** (`lbr.Library.List`, `internal/lbr/list.go`) is
  LIBRARIAN's `listlib.lis`, format string for format string: the
  "Directory of ... library ... on ..." line and the six header lines
  (creation and revision dates, format level, module count, key length,
  other entries, preallocated and used index blocks, deleted blocks, and
  history records), "Library is in DCX data reduced format" when it is,
  then a line per module. `/FULL` adds an object module's ident,
  insertion time, and symbol count (the unpadded form when the name or
  ident is over 15 characters, "Selectively searched" under a selective
  module, and a shareable image library module's GSMATCH as `!2XL,!6XL`),
  or a macro's insertion time. `/NAMES` prefixes "Module " and lists each
  object module's global symbols in columns a key and two blanks wide,
  as many as fit in the line width (80 on the console, 132 in a file, or
  `/WIDTH=`), then a blank line. Dates are `$ASCTIM`'s cut to 20
  characters, and numbers overflowing their FAO field become asterisks.
  Real `LIBRARY/LIST` output isn't in the fixtures yet; subtask 10
  compares the two.
- `internal/lbr` gained the header counts the listing reports
  (`HistoryRecords`, `IndexEntries`, `IndexBlocks`, `Preallocated`,
  `DeletedBlocks`), `KeySize`, `DataReduced`, and `Header` (a module's
  header without its records). Subtask 6's `lbr.VMSTime` is gone:
  `vmsdef.Time` already converts with VMS's wall-clock convention, which
  `VMSTime` got wrong (it ignored the local zone offset).
- **`govax library LIB [INPUT...]`** (`cmd/govax`): `--create`,
  `--insert`, `--replace`, `--delete` and `--extract` (comma-separated,
  repeatable), `--output`, `--list`, `--list-file`, `--full`, `--names`,
  `--macro`, `--object`, `--no-squeeze`, `--selective-search`, `--log`,
  run as a one-shot LIBRARY command with every name quoted.
- Tests: `internal/lbr/list_test.go` (wildcards, case rules, and a full
  `/NAMES` listing line for line); `internal/console/library_test.go` (a
  host macro library through create, insert, replace, `/NOSQUEEZE`,
  extract, delete, and list, then used by the assembler through
  `asm.NewMacroLibrary`; an object library listed with `/FULL/NAMES` and
  searched by LINK for a subroutine, the program run to check the
  subroutine's result; a volume library getting a new version and a
  listing file; failures leaving the library unchanged; and the DCL
  grammar); `cmd/govax/library_test.go` (the command line, and a
  one-shot create and list).

### 2026-09-30 — A changed library keeps its version (user decision)

- The user wants govax's libraries usable on a real VAX, where a library
  that changes version with every update would surprise VMS (LIBRARIAN
  updates in place, so the version never changes). A changed library now
  keeps its name and version; `/CREATE` still makes a new version, as
  VMS's does.
- **`rms.Session.RewriteRecordFile`** (`recordfile.go`) replaces an
  existing file's contents at its own version. A host file is replaced
  through a temporary file and a rename, as before. ods2 can't rewrite a
  file's blocks in place to a new length, so a volume file takes four
  steps, each leaving a complete copy of the old or the new contents on
  the volume: write the new contents to `GOVAX$REWRITE.TMP` beside it
  (a failure here changes nothing), delete the old version, write the
  contents again at that version, and delete the temporary file. If the
  second write fails, the error names the temporary file, which holds
  the new library.
  - The safe copy goes under another name, not as version N+1, because
    of version limits: with a limit of one, writing N+1 purges N (fine),
    but then re-creating N while N+1 exists makes N the oldest, and the
    limit purges it. Through another name, the file's version count is
    the same afterwards as before.
  - The file keeps its version limit. With no other version left to
    inherit it from, the new file would otherwise take its directory's
    default, so the limit is read before the delete and set again after.
  - Unlike an in-place update, the file gets a new file ID and creation
    date.
- Tests: `TestRewriteRecordFile` (a shorter file rewritten at version 1
  with version 2 untouched, no temporary file left, a version limit of
  one kept and not purging the file, and a location without a version
  refused); `TestLibrary_volume` now checks that a change keeps version
  1, and `/CREATE` makes version 2.

### 2026-09-30 — Subtask 8: govax's own STARLET.MLB

- **`internal/bootdata/files/starlet.mar`** holds govax's system macros:
  `$EXIT_S`, and the services a "hello" program needs, `$ASSIGN_S`,
  `$DASSGN_S`, `$QIO_S`, and `$QIOW_S`. They're written from the *System
  Services Reference Manual*'s argument lists (each service's list is in a
  comment above its macro), with the same keyword names and defaults as
  VMS's so that calls written for VMS work unchanged. Four helpers push
  arguments: `$PUSHADR` (by reference, or zero for an omitted one),
  `$PUSHVALS` (two by value), and `$PUSHVALADR`/`$PUSHADRVAL` (one each).
  Each pair helper clears a quadword (`CLRQ -(SP)`) when both of its
  arguments are omitted, as VMS's macros do.
- **`$PUSHADR` and `.NTYPE`.** An address is pushed with `PUSHAB` unless the
  addressing mode makes it depend on the operand's size: a literal,
  autoincrement, autodecrement, or an index. A literal is recognized by
  its `#` (but not absolute mode's `@#`) rather than by `.NTYPE`. The first
  version used `.NTYPE` for literals too, and caught absolute and general
  mode, which govax's `.NTYPE` numbers `^X2F` and `^X3F` (the manual's PC
  modes 2 and 3). Checking the text works whichever numbering real
  MACRO uses (still to be confirmed, subtask 10).
- **`BuildStarlet`** (`internal/bootdata/starlet.go`) builds the library
  as `LIBRARY/CREATE/MACRO` does, through `lbr.Create`, `MacroModules`
  (squeezed), and `Insert`, with a fixed creation and insertion time
  (30-SEP-2026 00:00, UTC) so the output changes only when the source
  does. A librarian warning is an error. `go generate ./internal/bootdata`
  runs `internal/bootdata/mkstarlet` to write `files/starlet.mlb` (9
  modules, 15 blocks), which is committed. `bootdata.StarletLibrary` and
  `StarletSource` name the two files for subtask 9.
- Tests: `internal/bootdata/starlet_test.go` checks that the committed
  library is what the source builds (the drift check), what its modules
  hold, and that a source with errors is refused.
  `internal/asm/govaxstarlet_test.go` assembles calls of every macro,
  with arguments omitted, zero, by value, and by reference in every
  addressing mode `$PUSHADR` distinguishes. `TestGovaxStarletAssembles`
  runs without the licensed library. `TestGovaxStarletMatchesReal` (which
  skips without `testdata/vmslib/starlet.mlb`) checks that each call
  gives the same object module, record for record, from govax's library
  as from VMS's.
- Found on the way: the assembler couldn't assemble a quadword immediate
  (`PUSHAQ I^#5`, which `$ASSIGN_S DEVNAM=I^#5` expands to from either
  library), and it assembled `PUSHAW S^#6` as a short literal in an address
  operand, a reserved addressing mode fault on a VAX. Both are fixed in the
  next entry.


### 2026-09-30 — Core defects found in subtask 8, fixed (user direction)

- The user ruled that defects found in core code along the way are in
  scope and get fixed, not just logged. Each fix below has an entry in
  `docs/DEVIATIONS.md`.
- **Quadword immediates** (`internal/asm/operand.go`): eight bytes, a
  single number read at full width, any other expression sign-extended,
  and a forward reference's high longword zero, all as `.QUAD` does.
- **Addressing modes checked against operand access** (the architecture
  manual's tables 8-5 and 8-6, `modeAllowed`): `#n` in an address or
  field operand is immediate mode (`PUSHAL #5` no longer chooses a short
  literal), and a literal anywhere but a read operand, an immediate in a
  modified or written one, a register as an address, and an indexed
  literal, immediate, or register are errors (`VAX_MODEACCESS`).
  `CALLG`'s register argument list stays allowed, the eVAX idiom
  `kernel.asm` uses. `TestRoundTripFixtures` now starts after each
  fixture's `.ENTRY` mask, which it used to decode as an instruction
  (`movq.asm`'s `F0 00` is an `INSV` with a literal base), and the
  immediate round-trip case is `PUSHL`, since `CLRL` can't write one.
- **Bit-field bases are field operands** (`OP_VA`): the table generator
  (`internal/cpu/gen`, run by hand since its `go:generate` line is
  disabled) now marks the base of the 15 bit-field instructions as
  such. `SHOW INSTRUCTIONS` names them `field`.
- **The CPU's immediate decode** (`decodeImmediate`): an 8-byte immediate
  panicked the emulator, a float immediate loaded as 0, and an address
  operand in immediate mode had no address (`PUSHAL I^#5` pushed 0). All
  three were the port's own regressions from `decode_operand.c`.
- **`fpuLoad`'s zero rule:** -0 is a reserved operand, and a zero exponent
  with a sign of 0 is zero whatever the fraction. It had these backwards,
  as `fpu_load` does.
- **`CALL`'s trailing text:** the console's `CALL` ignored anything after
  the address or argument list (`CALL X(1) junk` called X). Now it's
  `CLI_EXTRAPARAMETER`. Found by the linter.
- Tests: `internal/asm/immediate_test.go`, `internal/cpu/immediate_test.go`,
  the fpu and bit-field tests, and a `CALL` case in `dispatch_test.go`.
  `$ASSIGN_S DEVNAM=I^#5,CHAN=#6` joins the STARLET comparison.
- Still open, not part of this: other packages have older lint findings
  (an unused `strPut` in `internal/rtl/utils.go`, an ineffective assignment
  in `internal/rtl/core.go`, a gosimple hint in `cmd/govax/grammar.go`).
  G/H floating, octawords, and packed decimal belong to a later phase.
