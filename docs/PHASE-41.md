# Phase 41 — Symbolic disassembly from an image's debug symbol table

**Status:** planned (2026-10-05). Reviewed; the open questions are
decided (see Decisions). Not started.

## Goal

When an image is loaded and resident, the console's disassembler (the
`DISASSEMBLE` command, the instruction trace, and `STEP`) shows
instructions the way the VMS debugger's `EXAMINE/INSTRUCTION` does: an
operand that refers to an address the image's debug symbol table (DST)
names shows that name (`MOD\NAME`, or `NAME+offset`) instead of a raw
hexadecimal address, and each instruction can be placed by module,
routine, and listing line.

This phase is groundwork for a later govax **debugger mode**: an image
activated with its debugger, as VMS's image activator maps DEBUG ahead
of an image linked `/DEBUG`. So the pieces are built for that use, not
only for disassembly:

- **A disassembler other packages can use.** It moves out of
  `internal/asm` into its own package, decodes operands into structured
  values, and leaves turning addresses into names to a caller-supplied
  symbolizer.
- **One symbol model.** Assembler symbols, the console's symbol table,
  and the DST's symbols are looked up through the same interface, by name
  and by address.
- **The debugger's symbol table.** A reader for an image's DST, debug
  module table (DMT), and global symbol table (GST) builds what the
  debugger works from: modules, routines, labels and data symbols in
  their scopes, psects, and the line-number table, with lookups both ways
  (address to `MOD\ROUTINE\%LINE n`, and path name to address).

A loaded, resident image is assumed throughout. Disassembling an image
file that isn't loaded is out of scope.

## What earlier phases leave in place

- **The disassembler** (`internal/asm/disasm.go`, Phase 11).
  `Disassemble(r ByteReader, pc)` decodes one instruction with
  `internal/cpu`'s instruction table and returns a `Decoded`: the
  mnemonic, each operand as *text*, a `Values` slot per operand (whose
  meaning varies by addressing mode), and the length. Its doc comment
  says symbolic formatting was left out deliberately, for a caller to add
  by post-processing the text. It depends on `internal/cpu` and
  `internal/vaxfloat`, and on two `internal/asm` helpers (`regNames`,
  `floatFormat`).
- **The console's use of it** (`internal/console/disasm.go`).
  `decodeInstruction` recognizes a `.ENTRY` register-save mask by
  looking the PC up in `Console.Symbols` (`EntryAt`), and replaces a
  `CALLS`/`CALLG` target `@#addr` with an entry symbol's name. Both are
  linear scans of the console table. `traceStep` (`trace.go`) uses the
  same decoder. Only the console's `ASM` command puts entry symbols in the
  table (from `asm.Assembler.Symbols()`, a `map[string]SymbolInfo`), so
  for a real VMS image `RUN` loads, no routine's mask is recognized: the
  mask word is decoded as an instruction.
- **Image loading** (`internal/console/image.go`, `run.go`).
  `imageLoad` reads the whole image file (`readImage`), maps its ISDs at
  `ICB.Base` (0 for the main image, so link-time addresses are run-time
  addresses), and keeps an `ICB` per image in `Console.ICBList`, until
  the next `RUN` (`resetICBList`). `RUN/STEP` leaves the image loaded and
  stopped at its first instruction. Nothing reads the image's debug
  tables.
- **DST records in objects** (`internal/obj/dst.go`, `dbg.go`,
  `dbglines.go`; Phase 29). Decoding and encoding the DST records TBT and
  DBG records carry, as TIR commands. The record types real MACRO writes
  are named there: module begin and end, routine begin, psect (TBT); the
  symbol records (labels, typed data, literals, string and array
  descriptors), source correlation, and line numbers (DBG). These read
  *object* records, where the linker hasn't yet stored the addresses;
  an image's DST is the plain byte stream with the addresses in place.
- **DST, DMT, and GST in images** (`internal/link/debug.go`, Phase 29).
  LINK writes all three as real LINK does, byte for byte on TRDBGLNK,
  TRLNKDBG, and FORTH (with the GST padded, Decision 7 there).
- **Image headers** (`internal/anl/image.go`, Phase 40). `ReadImage`
  decodes the header blocks, including the IHS: the DST's VBN and block
  count, the GST's VBN and record count, the DMT's VBN and byte count.
- **`docs/DEBUG-RECORDS.md`.** The clean-room description of the DST
  format: the record stream and scope nesting (§3), the record header
  and type codes (§4), scope records (§5), data symbols (§7), PSECT,
  label, label-or-literal, and entry records (§12), the line-number
  program (§13), source correlation (§14), continuation records (§16),
  fixups (§18), and how to read a DST (§23.1).

## The fixtures

Images with debug data, all already in `testdata/` with their listings
and maps beside them:

| Image | Built with | Debug data |
|---|---|---|
| `mar/list/vax/trlnkdbg.exe`, `trdbglnk.exe` | `MACRO/DEBUG`, `LINK/DEBUG` | DST (TBT and DBG records), DMT, GST |
| `mar/dst/vax/forth.exe` | `MACRO/DEBUG`, `LINK/DEBUG` | the same, five psects, many symbols and lines |
| `mar/list/vax/trace.exe`, `trdbgtrc.exe`, `failmain.exe`, `failsig.exe`, `faildbg.exe` | traceback links | DST with module, routine, and psect records only (or more: to survey) |
| `mar/list/vax/trnotb.exe`, `failnotb.exe`, `fsignotb.exe` | `LINK/NOTRACEBACK` | none |

The listings give each line's address (the oracle for the line-number
table) and the maps give each symbol's (the oracle for the symbol
records). What these don't give is how the VMS debugger *shows* an
instruction: that needs a probe (subtask 1).

govax's own `MACRO/DEBUG` and `LINK/DEBUG` produce the same images, so a
program assembled and linked inside govax gets symbolic disassembly too.

## Design

### Packages

```
internal/disasm   (new)  decode an instruction into structured operands;
                         format them, asking a Symbolizer for names.
                         imports cpu, vaxfloat. Not asm.
internal/symtab   (new)  Symbol, Table: by-name and by-address indexes,
                         nearest-preceding lookup. A leaf package.
internal/dbgsym   (new)  an image's DST/DMT/GST read into the debugger's
                         symbol table: modules, scopes, symbols, lines.
                         imports obj (GST records), symtab.
internal/asm             assembler; its symbols exported as symtab.
internal/console         loads dbgsym per image; chains symbol sources
                         into the disassembler's Symbolizer.
```

`internal/disasm` must not import `internal/asm`: the assembler, the
console, ANALYZE, and a future debugger all sit above it. `internal/asm`'s
round-trip tests import `disasm` (a test-only dependency, no cycle).

### Structured operands

Today an operand is decoded straight to text. The new `Decoded` keeps
each operand's parts:

```go
type Operand struct {
    Mode     Mode          // literal, register, deferred, autoinc, ...,
                           // displacement, PC-relative, absolute,
                           // immediate, branch
    Register int           // Rn, or -1
    Index    int           // the index register, or -1
    Disp     int32         // displacement, sign-extended
    Literal  uint32        // short literal or immediate (low longword)
    Target   uint32        // the address the operand refers to, when
    HasTarget bool         //   it is known without running (branch,
                           //   PC-relative, absolute, @#, deferred forms)
    Access   cpu.AccessKind
    Type     cpu.DataType
    Size     int
}
```

Formatting is separate: `Format(dec, Options)` renders the operands, and
`Options` carries a `Symbolizer`, the radix, and the style. With no
symbolizer, the output is today's, byte for byte, so the assembler's
round trip and every existing console test are unchanged.

### Symbolizer

```go
type Symbolizer interface {
    // Symbolize names addr: a symbol at it, or the nearest preceding
    // one within its scope plus an offset. ok is false for none.
    Symbolize(addr uint32) (name string, ok bool)
    // EntryAt reports a routine's entry (a register-save mask) at addr.
    EntryAt(addr uint32) (name string, mask bool)
}
```

The console builds a chain: the DST of the image the address falls in,
then shareable image names (subtask 10), then its own symbol table.
Which operands are symbolized, and how a name is written (path name,
offset, radix), are what the probe settles (subtask 9).

### The debugger's symbol table (`internal/dbgsym`)

Read as `docs/DEBUG-RECORDS.md` §23.1 says, from the image bytes
`readImage` already holds:

- **Finding the tables**: the IHS (via the shared header reader,
  Decision 4), with `IHD$V_IHSLONG`'s 32-bit sizes.
- **The DST**: records joined with their continuation records, a stack of
  open scopes (module, routine, block; MACRO's routines have no routine
  end, §5.3, so a MACRO routine runs to the next routine or the module's
  end), unknown types skipped, fixups applied.
- **What it keeps**: per module, its name, language, psects (from PSECT
  records and the DMT), routines (name, entry, extent, JSB or CALL
  linkage), labels, data symbols (address, data type, descriptor),
  literals (from label-or-literal and value records), the source file,
  and the line-number table (a row per line: address, length, line).
- **The GST**: the debugger's fallback when no DST names an address
  (`docs/DEBUG-RECORDS.md` §1.3), read with `internal/obj`'s record
  reader.
- **Lookups**: `ModuleAt(addr)`, `RoutineAt(addr)`, `LineAt(addr)`
  (line, offset within it), `Symbolize(addr)`, `Lookup(path)` for
  `MOD\NAME`, `MOD\ROUTINE\%LINE n`, and a bare name in a current scope.
  These are the queries a debugger's `EXAMINE`, `SET BREAK`, `STEP`,
  `SHOW CALLS`, and `SET MODULE` need; this phase loads every module
  eagerly, where VMS's debugger loads them on demand (`SET MODULE`).

Addresses are relocated by `ICB.Base`, so a shareable image's DST (none
of the fixtures) works as the main image's does.

## Subtasks

Each is independently testable and ends with `go build`, `go vet`,
`go test`, and golangci-lint clean, a commit, and `build -i` when it
changes behavior. Each adds to this doc's progress log.

1. **The probe** (`testdata/dbg/`): a source, a link, a debugger command
   file, and a README, carried to VMS on an exchange container as
   earlier phases' were. The author runs it on simh while subtasks 2 to 8
   go ahead; its logs are checked in under `vax/` once audited. It
   covers:
   - a MACRO program (`dbgdis.mar`) with two modules, built
     `MACRO/DEBUG` and `LINK/DEBUG`, whose instructions use every
     addressing mode against: labels of code and of data, `.ENTRY`
     routines (CALLS/CALLG, JSB/BSB), constants (`=`), psect bases with
     no label, addresses past a label (`LABEL+6`), a label in the other
     module, a global, a LIBRTL routine (`G^LIB$PUT_OUTPUT`), branch
     targets, a `CASEx` table, and register displacements off `AP`/`FP`;
   - the same program linked without `/DEBUG` (traceback only) and
     `/NOTRACEBACK`, run with `RUN/DEBUG`, to see what the debugger shows
     from TBT records or the GST alone;
   - debugger commands, logged with `SET LOG`/`SET OUTPUT LOG`:
     `EXAMINE/INSTRUCTION` over ranges given by address, by routine, and
     by `%LINE`; `SET MODE SYMBOLIC` and `NOSYMBOLIC`; `SET RADIX`;
     `SYMBOLIZE`; `EVALUATE/ADDRESS`; `SHOW SYMBOL/ADDRESS`;
     `SHOW MODULE`; `SET STEP INSTRUCTION` with a few `STEP`s (the
     "stepped to" lines); `SHOW CALLS`; `EXAMINE/SOURCE`;
   - TRLNKDBG, TRDBGLNK, and FORTH from the fixtures, a few routines
     each;
   - a govax `LINK/DEBUG` image of TRACE, run under VMS's debugger: the
     check Phase 29's Decision 7 left for the next simh round.
2. **`internal/disasm`.** Move the disassembler out of `internal/asm`,
   unchanged in behavior: `Disassemble`, `Decoded`, `ByteReader`,
   `SliceReader`, `FormatMask`, and the formatters. The register names
   become `disasm.RegisterName` (or move to `internal/cpu`), and
   `floatFormat` becomes a `cpu.DataType` method both packages call.
   `internal/asm`'s round-trip tests and the console move to the new
   package. No output changes; every existing test passes untouched.
3. **Structured operands.** `Decoded.Operands` becomes `[]Operand`
   (above), with `Format` producing today's text when there's no
   symbolizer. `Values` goes away; the console's `CALLS`/`CALLG` special
   case uses `Target`. Tests: every addressing mode's fields, and the
   round trip over the fixtures still reassembles byte for byte.
4. **`internal/symtab` and the assembler's and console's symbols.** A
   `Symbol` (name, value, kind: label, entry, routine, data, literal,
   psect, module; scope; size and data type when known) and a `Table`
   with a name index and a sorted address index (exact and
   nearest-preceding lookup). `asm.Assembler.Symbols()` returns a
   `symtab.Table` instead of `map[string]SymbolInfo`. The console's
   `SymbolTable` keeps its attributes (permanent, label, system) but is
   built on `symtab` (Decision 3), so `EntryAt` and `FindByValue` stop
   being linear scans. Both satisfy `disasm.Symbolizer`.
5. **Reading the DST** (`internal/dbgsym`). Move `anl.ReadImage` into
   `internal/image` (Decision 4), with `internal/anl` and its tests
   unchanged in behavior. Find the DST through the IHS; read the record stream with continuations and the scope stack;
   keep modules, routines, psects, labels, data symbols, literals, and
   entry records; skip what it doesn't know; apply fixup records. Tests
   on every fixture image with a DST: each symbol's address is the one
   its map gives, each routine is a `.ENTRY` in its listing, and every
   record in the fixtures is a known type (so nothing is skipped
   silently). The TBT-only and `/NOTRACEBACK` images give what they hold
   and nothing.
6. **Lines and source files.** The line-number program of
   `docs/DEBUG-RECORDS.md` §13 (`DST$K_SET_STMTNUM`'s operand checked
   against real output, §23.3) and the source correlation records of §14
   (the source file's name). `LineAt` and `AddressOfLine`. Tests: every
   line with code in TRLNKDBG's, TRDBGLNK's, and FORTH's listings maps
   to the address the listing shows, and back.
7. **The DMT and the GST.** Module psect ranges from the DMT (checked
   against the DST's PSECT records), and the GST's symbols as the
   fallback table, read with `internal/obj`. Tests against the three
   `/DEBUG` images, and that an image without them (traceback links)
   still works from the DST alone.
8. **Loading it with the image.** `imageLoad` reads each image's debug
   tables from the bytes it already has, relocated by `ICB.Base`, and
   keeps them on the `ICB` (dropped with `resetICBList`). `SHOW IMAGES`
   says which images have debug data. The disassembler's entry-mask check
   asks the DST's routines too, so the mask at a real image's `.ENTRY` is
   shown as one: a fix even for traceback-only images. Tests: `RUN/STEP`
   of TRLNKDBG, then `DISASSEMBLE` of a routine shows its mask.
9. **Symbolic operands.** The rules from the probe's logs, in a
   `disasm.Symbolizer` backed by `dbgsym`: which operand modes are
   symbolized (branch, PC-relative and deferred, absolute, and whether
   immediates or displacements ever are), how a name is written (path
   name, the module prefix left off in the current module, `+offset`
   and its radix, `%LINE`), what's shown when nothing names the address,
   and what the GST alone gives. The constant-name option (Decision 5)
   is off by default. Rules no probe line settles are chosen
   and logged as unconfirmed here. Tests: unit tests per rule on built
   images.
10. **Shareable image references.** A `G^` reference goes through a
    fixup cell or a `SHIM$LIBRTL_<offset>` stub to a LIBRTL routine.
    Name it as the debugger does (per the probe), from
    `internal/librtl`'s `Routines` table (offset to `LIB$` name) and
    `vmsdef.ImageSymbols`, so `CALLS #1,@#...` shows the routine's name.
11. **The `DISASSEMBLE` command.** `/[NO]SYMBOLIC`, on by default with a
    console setting for the default (Decision 2), and the debugger's
    line layout under `/SYMBOLIC` (Decision 1). A start or end may be a path name or a line
    (`DISASSEMBLE FORTH\NEXT`, `DISASSEMBLE %LINE 120`), which means the
    expression evaluator (`expr.go`) resolves names through `dbgsym`
    after the console table, and `$expression` accepts `\` in a name.
    `console.dcl`, `vax.help`.
12. **Trace and `STEP`.** The instruction trace and `STEP`'s display use
    the same formatter and options, so stepping through a `/DEBUG` image
    shows where each instruction is (`MOD\ROUTINE\%LINE n`). `SHOW CALLS`
    names each frame's module, routine, and line where the debug data
    covers its PC, as VMS's traceback does (Decision 6). Tests: a
    `RUN/STEP` of FAILMAIN (an access violation in a nested call) into
    the fault, then `SHOW CALLS`.
13. **The oracle.** `TestDebuggerOracle` (in `internal/console`) runs the
    probe's images (real LINK's, and govax's links of the same objects)
    and compares govax's `DISASSEMBLE/SYMBOLIC` over the probe's ranges
    with the debugger's `EXAMINE/INSTRUCTION` lines from the logs.
14. **Close-out.** `CLAUDE.md` (the three new packages, the
    disassembler's move), `PLAN.md`'s index, `HELP`, and code comments
    that still say the disassembler lives in `internal/asm` or that
    symbolic output is left to the caller.

## Decisions

The author decided each of these on 2026-10-05, taking the plan's
recommendations for 1 to 5.

1. **Line layout.** Under `/SYMBOLIC`, the debugger's layout, as the
   probe shows it (a location such as `MOD\ROUTINE\%LINE n` or
   `NAME+offset`, then the instruction); under `/NOSYMBOLIC`, the
   console's current `ADDRESS: MNEMONIC operands`. A future debugger's
   `EXAMINE/INSTRUCTION` and this command print the same.
2. **The default.** Symbolic by default, as `SET MODE SYMBOLIC` is the
   debugger's default: `/NOSYMBOLIC` turns it off for one command, and a
   console setting changes the default.
3. **The symbol refactor.** The console's `SymbolTable` is rebuilt on
   `symtab` (subtask 4), not left behind an adapter: a debugger needs
   address lookups the linear scans can't give, and the console's table
   is the debugger's last fallback.
4. **A shared image-header reader.** `anl.ReadImage` moves into a leaf
   `internal/image` that `anl`, the console, and `dbgsym` share (subtask
   5), so `dbgsym` doesn't import ANALYZE.
5. **Constants.** govax matches the debugger by default: if the probe
   shows only addresses symbolized, so are govax's. An option goes
   further: an immediate or short literal is shown as a constant's name
   when exactly one literal in the current module has that value
   (subtask 9).
6. **`SHOW CALLS`** names each frame's module, routine, and line in this
   phase, wherever the loaded images' debug data covers the frame's PC
   (subtask 12). A frame outside any image with debug data shows as it
   does now. A traceback printed by `RUN` stays with the debugger phase.

Standing rules this phase follows:

- **Clean room.** The debugger's behavior comes from DIGITAL's manuals
  (the *VMS Debugger Manual*: `EXAMINE/INSTRUCTION`, `SET MODE`, path
  names, `%LINE`, symbolization) and real VMS output (the probe's logs),
  never from VMS's source. A rule neither settles is chosen and logged
  here as unconfirmed.
- **The DST format** comes from `docs/DEBUG-RECORDS.md`, checked against
  the fixture images. Where they disagree, the images win, and the doc
  gets a note.

## Out of scope

- **The debugger itself**: `RUN/DEBUG`, the image activator mapping
  DEBUG, the `DBG>` prompt, breakpoints and watchpoints, `SET MODULE`'s
  on-demand loading, and examining data by type. This phase builds the
  symbol table and display they'll use.
- **Disassembling an image file that isn't loaded** (a resident image is
  assumed).
- **DST contents in `ANALYZE/IMAGE`** (Phase 40's future expansion),
  though `dbgsym` would make it straightforward.
- **Languages other than MACRO.** The reader skips record types MACRO
  doesn't write (type specifications, records, enumerations, Ada and C++
  records) rather than interpreting them.
- **Source display beside instructions** (`EXAMINE/SOURCE`): the source
  file's name is read (subtask 6), but showing its lines is left for the
  debugger phase.

## References

- `docs/DEBUG-RECORDS.md`: the DST, DMT, and line-number formats.
- *VMS Debugger Manual* (`~/Documents/Technical Doc/VMS/`): machine-code
  debugging, `EXAMINE/INSTRUCTION`, `SET MODE [NO]SYMBOLIC`, path names
  and `%LINE`, `SYMBOLIZE`.
- *VMS 5.0 Linker Utility Manual*, chapter 7: the image's debug symbol
  table.
- `docs/PHASE-11.md` (the disassembler), `docs/PHASE-29.md` (DST
  records, LINK/DEBUG), `docs/PHASE-40.md` (image headers).
- **Not** the VMS source archive (off-limits since 2026-10-04).

## Progress Log

### 2026-10-05 — Planned

This plan written for review. Two pieces of the author's guidance shaped
it: the disassembler moves out of `internal/asm` for other packages' use,
which may change how assembler symbols are stored (subtasks 2 to 4); and
the phase is groundwork for a debugger mode with the image loaded and
resident (the design of `internal/dbgsym`, and the out-of-scope list).

### 2026-10-05 — Reviewed

The author took the recommendations on open questions 1 to 5 and asked
for `SHOW CALLS` with module, routine, and line in this phase where the
data is available (6). They're Decisions 1 to 6; the subtasks cite them.

### 2026-10-05 — Subtask 1: the probe, ready for VMS

- **`testdata/dbg/`**: `dbgdis.mar` and `dbgsub.mar` (the two-module
  program the plan describes), five debugger command files (`*.dbg`),
  `dbg.com`, `exchange.cmd`, `copyout.cmd`, and a README saying how to
  run it.
- **The program runs.** govax assembles, links (`/DEBUG`), and runs
  DBGDIS to the end; the first draft faulted on `B^LIMIT(R1)` after
  `MOVQ` had set R1, fixed by reloading R1.
- **The images.** `dbg.com` builds DBGDIS three ways (`/DEBUG`,
  traceback, `/NOTRACEBACK`), TRACE four ways (as Phase 29's probe did),
  FAILMAIN and FAILSUB linked `/DEBUG` (FAILLNK, new: Phase 29 linked
  FAILDBG with traceback only), and FORTH `/DEBUG`, with maps and
  `ANALYZE/IMAGE`, then runs each under the debugger with
  `DBG$INPUT` pointing at a generated command file that logs the
  session to `IMAGE.DLG`.
- **govax's images on VMS.** `exchange.cmd` builds GVDBGDIS and GVTRACE
  with govax's `MACRO/DEBUG` and `LINK/DEBUG` onto the volume; their
  sessions use the same commands as DBGDIS's and TRDBGLNK's. This is
  also Phase 29's Decision 7 check.
- **Unconfirmed until it runs**: that the debugger reads `DBG$INPUT` when
  started by `RUN/DEBUG` from a command procedure (the README gives the
  by-hand fallback), and the exact syntax of a few commands
  (`EXAMINE/OPERANDS=FULL`, `SHOW SYMBOL/TYPE`). A command VMS rejects
  just logs an error; the rest of the session goes on.
- Waiting on the author's simh run.

### 2026-10-05 — Subtask 1: the probe's results

The author ran the probe; `testdata/dbg/vax/` holds the results and its
README says what came back. Nine debugger sessions were logged; the two
`/NOTRACEBACK` images ran without the debugger, which `RUN/DEBUG`
silently skips for an image with no DST.

**govax's images.** VMS's debugger reads govax's `LINK/DEBUG` images as
its own: GVTRACE's session is TRDBGLNK's, which closes Phase 29's
Decision 7 (the padded GST is harmless). GVDBGDIS's differed only from
a MACRO bug, fixed here: `MOVAB START+2,R0` got a longword displacement
where real MACRO gives a word, because `knownTarget` took only a bare
label, not a label plus a constant, as a known same-psect target. Both
probe objects now match real MACRO's whole (`TestDebugRecords`'s
`dbgdis` and `dbgsub`).

**What the debugger shows**, the rules subtasks 6, 9, and 12 follow
(DBGDIS's session but where another is named):

- **Instruction lines** (`EXAMINE/INSTRUCTION`): a location, a colon,
  spaces to the next multiple of 8 columns (a full 8 when the colon
  ends on one), then the mnemonic left-justified in 8 columns, a space,
  and the operands. Spaces, never tabs. A line with no operands ends
  in the mnemonic's padding (`RET     `).
- **The location** is the most specific name for the address:
  - a routine's entry: `DBGDIS\START:` and `entry mask ^M<R2,R3,R4>`
    for the mask word (`^M<R2,...,R11,IV,DV>` for SUB2);
  - a label: `DBGDIS\START\LOOP:`, a label being in the scope of the
    routine before it (`DBGDIS\LOCALR\JSBRTN`, though it's a JSB routine
    after LOCALR's RET);
  - else a line's first instruction: `DBGDIS\START\%LINE 42:`, and a
    later one in the same line `FORTH\%LINE 332+6:` (FORTH's `$OPEN`
    expands to two instructions on one line);
  - else, with no line table (a traceback link), the routine plus a hex
    offset: `DBGDIS\START+2:`, `+0C` (a leading 0 when the first digit
    is a letter), `TRACE+10:`;
  - a routine named as its module drops one: `FORTH\%LINE 332:`,
    `FAILMAIN:`, `FAILMAIN+0A:`;
  - `SET MODE NOSYMBOLIC`: `00000400:`, 8 hex digits.
- **Operands.**
  - Short literals `S^#0A`, hex, two digits; immediates `I^#000003E8`,
    as many digits as the operand's size (`I^#9F16` for a word);
    floating literals `S^#1.500000`. Constants are never shown by name
    (`MOVL #LIMIT,R2` is `S^#0A`): Decision 5's default.
  - PC-relative and absolute operands always carry their width prefix
    (`L^`, `W^`, `B^`, `@L^`, `@#`) and are named: `L^DBGDIS\COUNT`,
    `@#DBGDIS\COUNT`, with the module always given, even in the
    current module.
  - A typed array's element: `L^DBGDIS\TABLE[2]` (TABLE+8), `BYTES[3]`,
    and `L^DBGDIS\TABLE[0][R4]` for an indexed operand.
  - Data with a string descriptor (`.ASCII`, `.ASCID`) isn't used for
    an operand: TEXT+10 and MSG fall to the global symbols, by nearest
    value at or below the address, constants included: `L^GLIMIT+24D`,
    `PUSHAQ L^GLIMIT+22F`, and the G^ cell `@L^SUB2+0F0`. `SYMBOLIZE`
    still names `DBGDIS\TEXT+10`. In FORTH, an address past a `$FAB`
    (`TTIN+UB_RAB`) is just `L^00003408`.
  - A system service's G^ reference: `@#SYS$OPEN`.
  - An address that's a line's first instruction but has no label:
    `W^DBGDIS\START\%LINE 42`.
  - Branch destinations have no prefix: a label (`DBGDIS\START\LOOP`,
    `FORTH\ABORT`) or a line (`DBGDIS\START\%LINE 82`); BSBW likewise.
    JSB to a label keeps `L^`.
  - Register displacements are numbers: `B^0A(R1)`, `B^04(AP)`, even
    when a constant has the value.
  - Without DBG records, operands are numbers: `L^00000200`, but a
    routine named in the DST still names a CALLS target
    (`L^TRACE\FIRST`), and globals from the GST name others
    (`L^LEVEL+3FE`).
  - `SET RADIX DECIMAL` makes the offsets decimal (`GLIMIT+589`).
- **Case tables**: after `CASEL`, one line per entry, 16 spaces and the
  destination (`DBGDIS\START\%LINE 100`).
- **Out-of-code bytes**: `FAILMAIN+0A:    HALT`.
- **STEP**: `stepped to DBGDIS\START\%LINE 43: MOVL     I^#000003E8,R3`
  (one space after the colon), then the source line (`    43:` and the
  text, wrapped at 80 columns with `     -:` continuation lines). By
  line: `stepped to DBGDIS\START\%LINE 48` alone.
- **Breakpoints**: `break at DBGDIS\LOCALR\JSBRTN`, `break at routine
  DBGSUB\SUB2`; an access violation: the `%SYSTEM-F-ACCVIO` message,
  then `break on unhandled exception at FAILSUB\SUB2\%LINE 12`.
- **SHOW CALLS** (Decision 6): a heading
  `module name     routine name      line                rel PC           abs PC`,
  then a line per frame with `*` before the module name, the line
  number right-justified, the PC relative to the routine and absolute.
  A JSB subroutine has no frame: at JSBRTN the one line is LOCALR's.
  Without a line table the line column is blank.
- **SYMBOLIZE** gives the DST's names (`DBGDIS\START+2` and
  `DBGDIS\START\%LINE 42`) and then the GST's (`(global)`, `START+2`).
  A psect base with no label symbolizes as the label before it
  (`DBGDIS\TEXT+1A` for NOLAB2-4), though psects are listed as
  symbols (`label DBGDIS\NOLABEL`, with its size).
- **SHOW SYMBOL/ADDRESS** lists routines with their sizes (START's is
  0x132: from its entry to LOCALR's), data symbols with addresses,
  constants, labels with their routine, and the psects. A data symbol
  with a descriptor shows a debugger-internal "descriptor address".
- The debugger's language for these modules is MACRO, with hex input
  and output radix by default.

Still unconfirmed (not covered by the probe): a byte displacement to a
named address (`B^` relative), a symbol in a shareable image other than
system services, and the layout of a location longer than 24 columns.

### 2026-10-05 — Subtask 2: `internal/disasm`

- **The move.** `internal/asm/disasm.go` is now `internal/disasm/disasm.go`
  (`git mv`, so its history follows), package `disasm`, with a package
  doc (`doc.go`). Its API is unchanged: `Disassemble`, `Decoded`,
  `ByteReader`, `SliceReader`, `FormatMask`. It imports `internal/cpu`,
  `internal/vaxfloat`, and `internal/vmserrors`, not `internal/asm`;
  `*asm.Assembler` still satisfies `disasm.ByteReader` through its
  `ByteAt`.
- **Shared helpers in `internal/cpu`.** The register names are
  `cpu.RegisterName` (`registers.go`), used by the disassembler and the
  assembler's index-base message. There were three copies of "data type
  to floating format": the disassembler's, the assembler's (both
  falling back to F), and the CPU's (which panics on a non-floating
  type, a table error). They're now `cpu.DataType.FloatFormat`, falling
  back to F; the CPU's `floatFormat` keeps its panic as a wrapper.
- **Tests.** The three pure decoding tests moved to
  `internal/disasm/disasm_test.go`. The round-trip tests, which
  reassemble, stay in `internal/asm` and call `disasm.Disassemble`, as do
  the fixture, floating, octaword, and opcode-table tests. No test's
  expectations changed.
- **The console** (`internal/console/disasm.go`) uses `disasm`. Output
  is unchanged.
- `CLAUDE.md` lists `internal/disasm`; `internal/asm`'s package doc
  says where the disassembler went.

### 2026-10-05 — Subtask 3: structured operands

- **`disasm.Operand`** (`operand.go`): the mode (`ModeLiteral` ...
  `ModeBranch`, and `ModeInline` for XFC's and BUGL's/BUGW's inline
  data), `Deferred`, `Register` and `Index` (-1 for none), `Width`,
  `Displacement` (sign-extended), `Value` and `Bytes` (a literal's or
  immediate's value, all its bytes for wide and floating ones), `Target`
  and `HasTarget` (branch, relative, and absolute operands; a deferred
  relative operand's is the pointer's address, and an indexed operand's
  the base's), and the table's `Access`, `Type`, and `Size`.
- **`Symbol`**: a name the caller sets for `Target`, which `String`
  shows in place of the number with the mode's prefix kept (`@#START`,
  `L^START`, `BRB START`). Subtask 9's symbolizer fills it; for now the
  console's CALLS/CALLG case does, replacing its string surgery.
- **`Decoded`** has `[]Operand`; `Values` is gone. `EntryMask` decodes a
  routine's mask word (`IsMask`, `Mask`, `Name`), replacing the console's
  hand-built `.ENTRY` Decoded.
- **Decoding and rendering are apart**: `disasm.go` decodes, and
  `format.go` renders the text internal/asm reassembles, byte for byte
  as before (every existing test, the fixture round trips included,
  passes unchanged).
- **Tests**: `TestOperandFields` (every mode's fields, Targets worked
  out by hand), `TestOperandSymbol`, `TestEntryMask`.

### 2026-10-05 — Subtask 4: `internal/symtab`

- **`internal/symtab`**, a leaf package: `Symbol` (name, value, `Flags`,
  `Scope`, `Size`) and `Table` (by name, ignoring case, and by value
  through a sorted index rebuilt on the first lookup after a change).
  `At(v, match)` finds a symbol at an address and `Nearest(v, match)`
  the nearest at or below it with the offset (`NAME+offset`); `match`
  filters (entry points only, no literals, ...), and of several at one
  value the first by name wins.
- **Flags, not a kind.** A console symbol can be both an entry point and
  a label (`SET/ENTRY/LABEL`), so attributes are flags: `Entry`,
  `Label`, `Data`, `Literal`, `Psect`, `Module`, `Global`, `System`,
  `Permanent`, `Builtin`. The DST's kinds of symbol (subtask 5) map onto
  the first six.
- **The assembler.** `asm.Assembler.Symbols()` returns a `*symtab.Table`
  instead of `map[string]SymbolInfo`, carrying the label, entry,
  permanent, system, and global flags over (`symbolFlags`).
- **The console** (Decision 3). `SymbolTable` keeps its methods but
  stores a `symtab.Table`, and `console.Symbol` is `symtab.Symbol`: the
  old `Kind`, `IsEntry`, `Permanent`, `IsLabel`, and `Predefined` fields
  are the `System`, `Entry`, `Permanent`, `Label`, and `Builtin` flags.
  `EntryAt` and `FindByValue` use the address index instead of scanning
  every symbol. `Table()` hands the table to the disassembler (subtask
  9). The ASM command's merge is unchanged in effect: it still sets only
  the entry flag and decides "system" by a `$` in the name, so SHOW
  SYMBOL's output is unchanged.
- **The `disasm.Symbolizer` interface** waits for subtask 9, where its
  shape is settled by the probe's rules; `symtab.Table`'s `At` and
  `Nearest` are what it will use.
