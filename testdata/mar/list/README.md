# Listing, traceback, and debugger probe (Phase 29)

These are the probe for `docs/PHASE-29.md`'s subtask 1. The Phase 27 and
28 fixtures already give 21 real listings and objects. These sources cover
what those don't: the listing directives, the binary field's harder cases,
the symbol table's rules, errors in a listing, the cross reference,
`/DEBUG` objects, and VMS's traceback output. Like the earlier fixtures,
real VAX MACRO assembles them on the user's simh VAX running VMS 7.3, and
the results are what govax is checked against.

| File | What it covers |
| --- | --- |
| `lctl.mar` | One body (macro definitions and calls, nested calls, repeat blocks, conditionals) listed under each `.SHOW` and `.NOSHOW` option, each section with its own `.SBTTL`. Then `.NLIST`/`.LIST` levels, counted options, a long `.SBTTL`, `.PAGE`, and lines longer than 132 columns. It's written by `gen.go`; don't edit it by hand. |
| `binary.mar` | The binary field: long data, `.QUAD`/`.OCTA`, floating, `.PACKED`, `.ADDRESS`, `.MASK`, gaps, every operand mode, indexed operands, external and relocatable fields, long instructions, branches, and a case table |
| `symtab.mar` | The symbol table: assigned, global, relocatable, external, weak, and undefined symbols, a local label, symbols a macro defines but the program doesn't use (with `.ENABLE`/`.DISABLE SUPPRESSION`), and the psect synopsis's attributes and a 31-character name |
| `notitle.mar` | No `.TITLE` or `.IDENT` (MACRO's default heading), and a `.SBTTL` first |
| `errors.mar` | One statement for each kind of error govax reports, `.ERROR`, `.WARN`, `.PRINT`, and an error inside a macro expansion |
| `errend.mar` | Errors found only at the end: an open conditional, an open macro definition, and no `.END` |
| `xref.mar` | `/CROSS_REFERENCE`: symbols, a macro, registers, and `.NOCROSS`/`.CROSS`, with and without a symbol list |
| `trace.mar` | What traceback and debugger records describe: routines, a JSB subroutine, labels, assigned symbols, three psects |
| `dbgsrc.mar` | `.ENABLE DEBUG`, then `.DISABLE TRACEBACK` and `.DISABLE DEBUG` partway through |
| `failmain.mar`, `failsub.mar` | A two-module program that takes an access violation two calls deep |
| `failsig.mar` | A program that signals an error (and goes on), then stops with a fatal condition |

`list.com` does the VMS side (`@LIST/OUTPUT=LIST.LOG`):

- **Listings.** It assembles every source with `MACRO/LIST`. It
  assembles `LCTL` again with `/SHOW=` and `/NOSHOW=`, `BINARY` with
  `/NOOBJECT`, and `XREF` and `SYMTAB` with `/CROSS_REFERENCE` (`=ALL`
  too). It records `$STATUS` and any objects after the error sources.
- **Objects.** It assembles `TRACE` with each choice of `/DEBUG`
  (`ALL`, `TRACEBACK`, `SYMBOLS`, `NONE`, or none at all), `/NODEBUG`,
  `/ENABLE=DEBUG`, and `/DISABLE=TRACEBACK`, and assembles the failing
  programs with `/DEBUG` too. It analyzes every object.
- **Images.** It links `TRACE`, its `/DEBUG` object, and the failing
  programs in three ways: with traceback, `/NOTRACEBACK`, and
  `/DEBUG`. Every link writes a map, and every image gets
  `ANALYZE/IMAGE`. It runs every image except the `/DEBUG` links, which
  would start the debugger, so the log keeps what VMS's traceback
  printed.
- It ends with a `DIRECTORY/SIZE=ALL/DATE` of the volume.

No source calls a system macro (`failsig.mar` writes out its condition
values). So no listing here can show STARLET macro text, and the
clean-room hook doesn't need to hold them back (docs/PHASE-32.md).

## The exchange volume

`exchange.cmd` builds it with govax, run from the repository root:

    go run testdata/mar/list/gen.go          # if gen.go changed
    govax console < testdata/mar/list/exchange.cmd

It makes `testdata/disks/list-exchange.dsk` (RD51 size, label LISTXCHG,
gitignored) and copies the sources and `list.com` onto it. The user
attaches it to simh, mounts it on VMS, sets it as the default directory,
and runs `@LIST/OUTPUT=LIST.LOG`. With simh paused (or the disk
detached), govax mounts it read-only and copies the results into `vax/`:
text files as text, objects in the host variable-length record layout,
and images as raw blocks (`COPY/BINARY`), with names lowercased.

## What came back (`vax/`)

From the user's run of `@LIST/OUTPUT=LIST.LOG` on 2-OCT-2026.
`copyout.cmd` (`govax console < testdata/mar/list/copyout.cmd`) copied
all 96 results off the volume: listings (`.LIS`), object and image
analyses (`.ANL`, `.ANI`), maps, `LIST.LOG`, the objects, and the images.
Every object decodes with `internal/obj`.

What stood out on a first look (the subtasks that use each one look
closer):

- **Real MACRO crashed twice.**
  - On `ERREND` it stopped with `%MACRO-F-INSVIRMEM` and left an empty
    `ERREND.LIS` and no object. So there's no reference for errors
    found only at the end of a source, and govax keeps its own
    messages for them.
  - On `DBGSRC` it took an access violation after line 12, the
    `.PSECT CODE` that follows `.ENABLE DEBUG`. It left a listing cut
    off there and an object that `ANALYZE/OBJECT` rejects (no end of
    module record, a longword left on the stack). Which statement is
    to blame needs a smaller follow-up source.
- **An assembly with errors still writes an object.** `ERRORS.OBJ`
  exists, and `$STATUS` is `%X10000002`, an error. govax writes no
  object when there are errors (Phase 27's choice).
- **Errors in the listing.** Each message follows its line, with a `!`
  under the column where the error was found. The listing ends with a
  summary and the line numbers that had messages. That summary is
  "There were 22 errors, 2 warnings and 0 information messages, on
  lines:", followed by `line (file)` pairs, five to a row.
- **Table of contents.** `.SBTTL` makes MACRO write a "Table of
  contents" page, numbered page 0, before page 1. Each entry gives the
  file number, the line number, and the subtitle.
- **Which records each choice writes.** Traceback records are TBT;
  debugger records are DBG.

  | Choice | TBT | DBG |
  | --- | --- | --- |
  | no qualifier, `/DEBUG=TRACEBACK` | 3 | 0 |
  | `/DEBUG`, `/DEBUG=ALL`, `/ENABLE=DEBUG` | 3 | 2 |
  | `/DEBUG=SYMBOLS` | 0 | 1 |
  | `/DEBUG=NONE`, `/NODEBUG`, `/DISABLE=TRACEBACK` | 0 | 0 |

  So `/NODEBUG` turns traceback off too.
- **Traceback output.** After the condition's message comes
  `%TRACE-F-TRACEBACK, symbolic stack dump follows` (or `-E-`, for an
  error that lets the program go on). Then a table with the columns
  module name, routine name, line, rel PC, and abs PC, innermost call
  first. A MACRO module leaves the line column empty, even when it was
  assembled with `/DEBUG` (`FAILDBG`).
  - Without traceback, an unhandled access violation gets the
    "Improperly handled condition" register dump instead (`FAILNOTB`).
  - Without traceback, a signaled condition just gets its message
    (`FSIGNOTB`).

## What govax said before the probe ran

Every source but the two error sources assembles under govax. On
`errors.mar`, govax reports 20 errors, plus `.WARN`'s warning and
`.PRINT`'s message. It doesn't yet report:

- `.LONG NOWHERE` under `.DISABLE GLOBAL` (undefined);
- `MOVL (R0)+[R0], R1` (the index register is the base register);
- `CLRL PC`.

On `errend.mar` it reports the open macro definition (`NOENDM`), but not
the open conditional or the missing `.END`. The probe will show which of
these real MACRO reports, and how. Subtask 8 settles the differences.

Subtask 8 settled them: real MACRO reports `(R0)+[R0]` (ILLINDXREG) and
`CLRL PC` (ILLREGHERE), and govax now does too. `.DISABLE GLOBAL`'s
`NOWHERE` is an external reference to real MACRO, not an error. For
`errend.mar` there's no reference, so govax keeps its own messages, and
it now also reports the open conditional (docs/PHASE-29.md, subtask 8's
log).

