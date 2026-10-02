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
