# Phase 27: MACRO-32 object modules (`.MAR` → `.OBJ`)

## Goal

Produce a valid VAX/VMS object module (`.OBJ`) from MACRO-32 source (`.MAR`),
using a new command:

```text
MACRO source[/HOST] [/[NO]OBJECT[=object]]
```

If `/OBJECT` doesn't name a file, the object file is the source file name
with its extension replaced by `.OBJ`. The request asked for `/OUTPUT=`; the
user then chose VMS's own `/[NO]OBJECT` (2026-09-29). The object must be one a real VAX/VMS linker
accepts: `LINK` on a real VAX must build a working image from it, and
`ANALYZE/OBJECT` must report no errors.

The assembler that does this is the existing `internal/asm` package, not a
second assembler. The console's `ASM` command (the "mini assembler") and the
new `MACRO` command share one core: one expression evaluator, one instruction
encoder, one symbol table, and one directive table. Where the current
structure doesn't allow that, this phase refactors it first (subtask 4).

This document is the plan to start with. Like the other phase documents, it
becomes the record of the implementation: each subtask adds a
[progress log](#progress-log) entry, and the open questions get their answers
recorded here.

**Status: complete (2026-09-30).** All twelve subtasks are done, and so are the `ods2` interop fixes (including a VMS-faithful `Initialize`), confirmed on VMS. The later sub-phases below moved to Phases 28 (macro facility), 29 (listings, traceback, and debugger records), and 30 (a govax `LINK`).

## Why this phase looks different

eVAX has no object-module output. Its assembler writes directly into emulated
memory, and so does govax's port of it (Phase 11). So there's no
`reference/eVAX` source to port here. The correctness references are real VMS
materials, plus object files from the user's real VAX.

## Can we build a valid object file? Yes

The project's document archive has everything needed. Reverse engineering
isn't needed to learn the format. The real-VAX objects are still valuable,
though: they show which of the format's legal choices real MACRO-32 makes
(see [Validation strategy](#validation-strategy)).

| Source | What it gives us |
| --- | --- |
| *VMS 5.0 Linker Utility Manual*, Chapter 7 "VAX Object Language" (`~/Documents/Technical Doc/VMS/AA-LA62A-TE_VMS_5.0_Linker_Utility_Manual_198804.pdf`) | The full specification: every record type, every GSD subrecord, all 85 TIR commands plus STORE IMMEDIATE, EOM/EOMW, debugger and traceback records. It is written for "programmers writing compilers or assemblers". `pdftotext -layout` extracts it cleanly. **This is the primary reference.** |
| *VAX-11 Linker Reference Manual*, v2.0, Appendix C "VAX-11 Object Language" (`AA-D019B-TE_…_198003.pdf`) | An older version of the same specification. Useful for cross-checking, and as a simpler baseline (fewer GSD subrecord types). |
| `vmssrc_archive/v73/starlet_b64/lis/objfmt.sdl` | The real SDL definitions: `$OBJRECDEF` (record types, `GSD$C_*`/`TIR$C_*` codes, `OBJ$C_MAXRECSIZ` = 2048, `OBJ$C_SYMSIZ` = 31, `OBJ$C_STRLVL` = 0), `$MHDEF`, `$EOMDEF`, `$GPSDEF` (psect flags), `$GSYDEF`, `$EPMDEF`, `$SRFDEF`, and the other subrecord layouts. It is the same kind of input `internal/vmsdef/gen` already generates constants from. The file also has the Alpha `EOBJ`/`EGSD`/`ETIR` definitions, which we ignore. |
| `vmssrc_archive/v73/analyz/lis/obj*.lis` (`objdrive`, `objgsd`, `objtir`, `objmisc`, …) | Source listings of `ANALYZE/OBJECT`, the VMS object validator. They show exactly what VMS checks, which is useful for building our own dumper and checker. |
| `vmssrc_archive/v73/linker/lis/lnkobjps1_v.lis` (and `lnkobjps2`) | Source listings of the VAX linker's object-reading passes. Use them when the manual is ambiguous. |
| `OVMS_83_LINKER.pdf`, around line 7740 of its text | An annotated `ANALYZE/OBJECT` dump example, showing what the analyzer's output looks like. |

For the MACRO-32 language itself, the user added (2026-09-29) the *VAX MACRO
and Instruction Set Reference Manual* for OpenVMS VAX 7.3, in two printings:

| Source | What it gives us |
| --- | --- |
| Compaq printing, AA-PS6GD-TE, April 2001 (`~/Documents/Technical Doc/VMS/138206246-VAX-MACRO-and-Instruction-Set-Reference-Manual.pdf`) | **The language reference.** Chapter 3 covers expressions and the absolute/relocatable/external rules. Chapter 5 covers addressing modes, including how MACRO picks displacement sizes. Chapter 6 covers every directive (`.PSECT` and its defaults, `.ENABLE`/`.DISABLE`, `.DEFAULT`, `.TITLE`, the macro facility, and more). `pdftotext -layout` extracts it cleanly. Section numbers in this document refer to this printing. |
| VSI reissue of the same manual (`vsi-openvms-vax-macro-and-instruction-set-reference-manual.pdf`) | The same content, re-typeset. Use it to cross-check any passage the Compaq text extracts badly. |

The manuals don't describe the DCL `MACRO` command's qualifiers
(`/OBJECT`, `/LIST`, `/DEBUG`); those are in the DCL dictionary. Where the
manual and a real VAX disagree, the real VAX wins, following the existing
"real MACRO-32 behavior wins over eVAX" rule for `internal/asm`.

### What the MACRO manual settles

- **Displacement sizes (§5.2.1, §5.2.2, `.DEFAULT` in ch. 6).** For relative
  and relative-deferred operands with no `B^`/`W^`/`L^`, MACRO uses the
  smallest displacement only when the target's value is *known*: already
  defined, and in the same psect. Otherwise (a forward reference, another
  psect, or an external) it uses the default displacement, which is a
  **longword** unless `.DEFAULT DISPLACEMENT,BYTE|WORD|LONG` changes it.
  This is a rule a single pass can follow, so MACRO's operand sizes can be
  matched without a sizing pass (see [Pass structure](#pass-structure)).
- **General mode (`G^`, §5.2.5)** is always 5 bytes. The linker turns it
  into relative mode for a relocatable address or absolute mode for an
  absolute one. That is what the object language's `STO_PICR` and
  `STO_PIDR` store commands are for.
- **Default psects (`.PSECT`, ch. 6, note 2 and Table 6-7).** There are two:
  - `. ABS .`: NOPIC, USR, CON, ABS, LCL, NOSHR, NOEXE, NORD, NOWRT, NOVEC,
    BYTE. Symbol definitions that come before any code, data, or `.PSECT`
    go here.
  - `. BLANK .`: NOPIC, USR, CON, REL, LCL, NOSHR, EXE, RD, WRT, NOVEC,
    BYTE. Code and data that come before the first named `.PSECT` go here,
    and so does a `.PSECT` with no name.

  A named `.PSECT` defaults to CON, EXE, LCL, NOPIC, NOSHR, RD, REL, WRT,
  NOVEC. Continuing a psect may repeat its attributes, but must not change
  them. There can be at most 254 user-defined psects, so the word-psect
  GSD forms are never needed.
- **Module name (`.TITLE`).** The name is the first 1 to 31 non-blank
  characters. Without a `.TITLE`, the module is named `.MAIN.`. If there are
  several, the last one wins.
- **`.ENABLE`/`.DISABLE` defaults (Table 6-3).** GLOBAL is on: undefined
  symbols are external. **TRACEBACK is on**: psect names and lengths, module
  names, and routine names go into the object for the debugger. So real
  MACRO objects include traceback records by default, which affects fixture
  comparison (see [Validation strategy](#validation-strategy)). ABSOLUTE,
  DEBUG, LOCAL_BLOCK, SUPPRESSION, and TRUNCATION are off.
- **Expression restrictions (§3.5).** The operands of `.ALIGN`, `.BLKx`,
  `.IF`/`.IIF`, `.REPEAT`, `.OPDEF`, `.ENTRY`, data repetition factors, and
  direct assignment (`=`) may only use symbols already defined in the
  module, never external or forward ones. Most must be absolute; a direct
  assignment may be relocatable. This is stricter than today's assembler,
  which accepts forward references in some of these places. The MACRO
  dialect enforces the manual's rule, and the console dialect keeps its
  current behavior.

## Object format summary

This is enough to design against. Chapter 7 has the details.

- An object module is a sequence of **variable-length records**, at most
  `OBJ$C_MAXRECSIZ` (2048) bytes each. The first byte of each record is its
  type (`OBJ$C_HDR`=0, `GSD`=1, `TIR`=2, `EOM`=3, `DBG`=4, `TBT`=5, `LNK`=6,
  `EOMW`=7).
- **Record order:** header records (main header `MHD` first, then language
  processor `LNM`, and optionally source-file `SRC`, title `TTL`, and others),
  then GSD records, then TIR records (and DBG/TBT records if present), then
  exactly one EOM record.
- **MHD** carries the structure level, the maximum record size, the module
  name (from `.TITLE`), the module version (from `.IDENT`), and the creation
  and patch dates.
- **GSD** records hold packed subrecords. For a MACRO module the important
  ones are:
  - `GSD$C_PSC`: a psect definition, with alignment, the `PIC`/`USR`/`CON`/
    `REL`/`LCL`/`SHR`/`EXE`/`RD`/`WRT`/`VEC` flags, allocation size, and
    name. The psect index is the order of definition.
  - `GSD$C_SYM`: a global symbol definition (psect index plus value, flags
    `DEF`/`REL`/`WEAK`/`UNI`) or reference (`SRF` form, name only).
  - `GSD$C_EPM`: an entry point, which is a definition plus a register
    save mask. `.ENTRY` produces this for a global name.
  - The word-psect variants (`SYMW`, `EPMW`, `PROW`) are only needed past
    255 psects, so we don't plan to use them.
- **TIR** records hold commands for the linker's stack machine:
  - `STA_*` commands push a value: a literal, a psect base plus an offset
    (`STA_PB`/`PW`/`PL`), or a global symbol's value (`STA_GBL`).
  - `STO_*` commands pop a value and store it: `STO_B`/`W`/`L`, the displaced
    (PC-relative) stores `STO_BD`/`WD`/`LD`, `STO_PICR` and `STO_PIDR` for
    position-independent operand references, and the repeated stores.
  - `OPR_*` commands do arithmetic on the stack (add, subtract, multiply,
    divide, logical operations, shifts, and more), which is how expressions
    the assembler can't finish (like `EXT+4` or `A-B` across psects) reach
    the linker.
  - `CTL_SETRB` sets the location counter (usually to a psect base plus an
    offset).
  - **STORE IMMEDIATE** is any negative command byte n: store the next |n|
    bytes (1 to 128) as they are. Most code and data goes out this way.

  The stack must be empty when the EOM record is reached.
- **EOM** carries a severity code (the assembly's worst error level), and
  optionally the transfer address (psect index plus offset) from
  `.END label`.

## What exists today

`internal/asm` (Phases 11 and 24, and the recent MACRO-32 alignment work)
already has most of what MACRO-32 needs: the operand parser and instruction
encoder, MACRO-32 expressions (decimal default radix, equal operator
priority, `^X`/`^O`/`^B`/`^A`/`^M`, strings), local labels (`n$`), `.ENTRY`,
the data and storage directives, and `.IF` conditional assembly.

Three things about its structure matter for this phase:

1. **Output is absolute.** Every byte goes into a sparse, address-keyed
   `image` at an absolute VAX address, starting from a location counter
   (`deposit`, default `0x200`). There are no program sections. A symbol's
   value is a plain `uint32`. `.REGION` switches between two absolute
   location counters (P0 and S0), which is the closest thing to a psect today.
2. **One pass, with fixups.** A forward reference queues a `fixup`, which is
   a location plus an expression. That expression is already a **linear
   combination** of symbols (`addend + Σ coeff·symbol`, see `fixup` and
   `fixupTerm` in `symbol.go`), and its comment already notes that "MACRO-32's
   object records carry such expressions to the linker". This is the right
   base for relocation: a relocatable value is the same kind of linear
   combination, with terms that are psect bases or external symbols instead of
   not-yet-defined local symbols. But because it's one pass, an operand's
   size is decided the first time it's seen. A forward reference can't later
   shrink its displacement.
3. **The directive set mixes MACRO-32 and eVAX console features.**
   `.SHIM`, `.SCB`, `.VECTOR`, `.REGION`, `.P1VECTOR`, `.MICROKERNEL`,
   `.CONSOLE`, `.BASE`, `.SET`, `.CLEAR`, `.SYM`, `.PSL`, `.VERSION`,
   `.RMSDEF`, `.FAB`, `.RAB`, and the `J`*cc* aliases only make sense when
   assembling straight into emulated memory. A real MACRO-32 object can't
   express most of them.

## Design decisions (proposed)

These are proposals. The ones that need the user's input are repeated under
[Open questions](#open-questions).

### One core, two dialects

Add a **dialect** to `Assembler`: `DialectConsole` (today's behavior, and the
default, so `ASM` doesn't change) and `DialectMACRO`. Each entry in the
directive table records which dialects allow it, following the
table-driven-dispatch convention. In MACRO dialect, eVAX-only directives are
errors ("not a MACRO-32 directive"). In console dialect, the MACRO-only
directives (`.PSECT`, `.GLOBL`, `.EXTERNAL`, `.TITLE`, `.IDENT`, …) are
accepted where they have a sensible meaning in absolute mode. `.PSECT`, for
example, can just switch location counters, and `.TITLE` can be recorded.
This keeps the two languages converging.

### Program sections become the core's location model

Replace the single absolute location counter with a set of **sections**, each
with a name, attributes, and its own location counter. A value's location is
then (section, offset), not an absolute address.

- **Console dialect** stays an absolute-address special case: sections have
  fixed base addresses (P0 at `origin`, S0 at `s0Origin`), so any
  (section, offset) turns into an absolute address right away, and output
  still goes to the sparse `image`. `.REGION` becomes "switch to the other
  absolute section", with the same observable behavior.
- **MACRO dialect** sections are relocatable. Their bases are unknown until
  link time, so their contents are kept as byte buffers plus a list of
  **relocations** (location, width, kind, and the unresolved linear
  expression).

The existing `image` stays as the console dialect's output store. The section
buffers are the MACRO dialect's.

### Relocatable values

Extend `exprVal` (and `fixup` and `symbol`) so a value is
`constant + Σ coeff·base(psect) + Σ coeff·external`. The manual's three
kinds of expression (§3.5) fall out of that form:

- **absolute**: no psect or external terms. That includes label − label in
  the same psect, since the psect base cancels out.
- **relocatable**: exactly one psect base with coefficient 1, and no
  externals.
- **external**: any external term.

An expression that is relocatable or external is emitted as TIR stack
arithmetic unless it matches one of the simple store forms (psect base +
offset, or a global + offset).

Values that are still unresolved at the end of assembly become relocations
instead of errors, as long as the symbol is external. That's the case when
it's declared with `.EXTERNAL`, or it's undefined and `.ENABLE GLOBAL` is on,
which is MACRO's default.

### Pass structure

**Decision: one pass with fixups, in both dialects.** The MACRO manual
settles this (see [What the MACRO manual settles](#what-the-macro-manual-settles)).
MACRO only picks the smallest displacement for a target that is already
defined in the same psect. For anything else it uses the default
displacement (longword, or whatever `.DEFAULT DISPLACEMENT` set). Both cases
are decided when the operand is read, which is exactly what a single pass
does. So the current fixup design can match MACRO's operand sizes, and no
sizing pass is needed.

The only changes are in the MACRO dialect:

- A known target in another psect counts as unknown, so it gets the default
  displacement.
- `.DEFAULT DISPLACEMENT` is supported.

The real-VAX fixtures (ladder steps 4, 5, and 7) confirm this (subtask 7):
govax's operands match real MACRO's byte for byte in all nine fixtures.

### A new `internal/obj` package

This is a leaf package for the VAX object language itself, with no dependency
on `internal/asm`:

- record and subrecord types, with constants generated from `objfmt.sdl`
  through `internal/vmsdef/gen` (new `$OBJRECDEF`, `$MHDEF`, `$EOMDEF`,
  `$GPSDEF`, `$GSYDEF`, `$EPMDEF`, `$SRFDEF` modules; the Alpha `E*`
  modules are skipped);
- a **writer**: a module builder (header, psects, symbols, text and
  relocation, EOM) that splits TIR output into records under the 2048-byte
  limit and gathers STORE IMMEDIATE runs;
- a **reader and dumper**: it parses any VAX object module and prints it
  in a format modeled on `ANALYZE/OBJECT`. This is how we decode the
  user's real-VAX objects, how tests check our output, and later how a
  govax `ANALYZE/OBJECT` or `LINK` would read objects;
- a **checker**: structural validation modeled on the `analyz/lis/obj*.lis`
  checks (record order, subrecord lengths, psect indexes in range, TIR stack
  balanced at EOM, and so on).

`internal/asm` gets a small emitter that turns its sections, symbols, and
relocations into `obj` builder calls. The reader and writer are symmetric, so
round-trip tests are cheap.

### Command surface

```text
MACRO source[/HOST] [/[NO]OBJECT[=object]]
```

- **Console DCL verb** `MACRO`, added to `internal/bootdata/files/console.dcl`
  in its govax-native block. The one required parameter is the source file,
  which can carry a parameter-scoped `/HOST` qualifier, the same one COPY
  uses (Phase 23). The object qualifier is VMS's own `/[NO]OBJECT[=file]`
  (decided 2026-09-29). `/OBJECT` with no value, or no qualifier at all,
  means the default name. `/NOOBJECT` assembles and reports errors but
  writes nothing. `/LIST` is kept for later.
- **`govax` CLI subcommand** `macro`, passed through to the console command
  the same way `asm` and `run` are (`doCmd` in `cmd/govax/grammar.go`). It
  needs to pass the source file and an optional object name through.
- **Default object name:** the source's own location (the same host
  directory, or the same device and directory), with the extension replaced
  by `OBJ`, **in the same case as the source's extension**: `hello.mar` gives
  `hello.obj` and `HELLO.MAR` gives `HELLO.OBJ` (decided 2026-09-29). The case
  is copied letter by letter, so `Hello.Mar` gives `Hello.Obj`. With no
  extension, `OBJ` is lowercase if the name has no uppercase letters, and
  uppercase otherwise. On an ODS-2 volume names are uppercase anyway. The
  version number on an ODS-2 source is dropped, so the object gets the next
  version of its own name.
- **Errors:** every error is reported, and then **no object file is
  written** (decided 2026-09-29). The object is built in memory and only
  written once assembly succeeds, so a failed assembly never leaves a
  partial file or replaces a good one.

### File specifications: host files and ODS-2 volumes

`MACRO`, and the future `LINK`, must work equally well with host files and
with files on a mounted ODS-2 volume, and let the two be mixed: a host
source to an ODS-2 object, or the reverse. The rules for deciding which kind
of file a name means live in one shared helper, so every command that
accepts file names agrees:

1. **An explicit `/HOST`** on the parameter means a host path, whatever the
   name looks like. This is COPY's parameter-scoped qualifier. It handles
   the rare host path that happens to look like VMS syntax.
2. **A name that is obviously VMS syntax** means a file on a mounted volume.
   That's a name with a directory in brackets (`[DIR.SUB]` or `<DIR>`), or a
   `name:` prefix where `name` is at least two characters long, followed by
   no `/` or `\`. The two-character minimum keeps a Windows drive letter
   (`C:\x`) on the host side. A logical name prefix (`SRC:HELLO.MAR`) goes
   through Phase 25's `TranslateFileSpec` and must end on a mounted device.
   If it doesn't, that's an error, not a silent fall back to the host.
3. **A name that is obviously a host path**, one containing `/` or `\`,
   means a host file.
4. **Anything else** (a bare `HELLO.MAR`):
   - for the **source**, a file in the default directory of a mounted volume
     if a `SET DEFAULT` onto one is in effect, and a host file otherwise
     (decided 2026-09-29). This keeps MACRO consistent with COPY, TYPE, and
     DIRECTORY whenever an ODS-2 default is set, and keeps
     `govax macro hello.mar` working with nothing mounted;
   - for the **object** named in `/OBJECT=`, the same side as the source,
     so `MACRO DUA0:[X]HELLO.MAR/OBJECT=OTHER.OBJ` writes
     `DUA0:[X]OTHER.OBJ`. To send an ODS-2 source's object to the host,
     give a host path (`/OBJECT=./hello.obj`).

`.INCLUDE` names are resolved the same way, relative to the file being
assembled, whichever side it's on.

ODS-2 access goes through `internal/rms`, the only package allowed to import
`ods2`. It reuses Phase 23's `rms.Session`, which holds the default directory
and `resolveVolume`, and adds a small host-side record API: open a file and
read its records, or create a file with given record attributes and write
records. Host access uses `os` directly. The shared helper that picks between
them lives in `internal/rms` too (`location.go` and `recordfile.go`), since
only it can parse an ODS-2 spec (subtask 9).

**How the records are stored.** On VMS, a `.OBJ` file is an RMS
variable-length record (`RFM=VAR`) file, and the record boundaries are part
of the format.

- **On an ODS-2 volume**, the object is a real `RFM=VAR` file, written
  through `ods2`'s `rms.Writer`, with real MACRO's attributes: no `RAT`
  flags, a maximum record size of 0, a default extension of 20 blocks,
  and the longest record in `RSIZE` (subtask 9). No `ods2` change was
  needed.
- **On the host**, there are no record boundaries, so the file uses ODS-2's
  own on-disk VAR layout: for each record, a 2-byte little-endian length,
  then the data, padded to an even length. That's the same bytes a raw
  copy of the file's blocks would have. Subtask 9 confirmed the layout
  survives a COPY in both directions, after one COPY change: a `.OBJ` is
  now always copied as records, so a host `.OBJ` comes back into a
  container as a real VAR file (see the subtask 9 log).

**Changes to `ods2`** are in scope for this phase whenever they're needed
(the user decided this 2026-09-29), for example if its record-format support
turns out to be missing something `.OBJ` files need. `ods2` is a separate
repository with its own conventions (`ods2/CLAUDE.md`):

- detailed comments for readers new to VMS;
- tests in the same commit as the code;
- one commit per task;
- **no AI attribution lines in commit messages**;
- no pushing.

govax's `go.work` picks up local `ods2` changes right away.

## Language scope

**First milestone.** This is enough to link and run a realistic hand-written
program:

- `.TITLE` (the module name, `.MAIN.` by default), `.IDENT`, and
  `.SUBTITLE`/`.SBTTL` (accepted, and ignored until listings exist;
  done in Phase 29)
- `.PSECT name[,attributes…]`, `.SAVE_PSECT`/`.RESTORE_PSECT`, the default
  psects `. ABS .` and `. BLANK .` with the manual's attributes, the
  named-psect attribute defaults, the attribute-consistency check on
  continuation, and psect alignment
- `.ENTRY name, mask` (a global EPM definition)
- `label::` and `sym==value` (global definitions), `.GLOBAL`/`.GLOBL`,
  `.EXTERNAL`/`.EXTRN`, `.WEAK`
- `.ENABLE`/`.DISABLE`: `GLOBAL` (on by default) and `ABSOLUTE` are
  implemented. `TRACEBACK` and `DEBUG` are recorded for when traceback and
  debugger records exist. `LOCAL_BLOCK` uses the existing local-label
  blocks. The rest are accepted and ignored, with a warning.
- `.DEFAULT DISPLACEMENT,BYTE|WORD|LONG` (longword by default), which the
  displacement-size rule needs
- the existing data and storage directives (`.BYTE` through `.QUAD`,
  `.BLKx`, `.ASCII*`), which now allow relocatable operands, plus
  MACRO-32's own forms of the directives whose eVAX versions differ
  (see the subtask 4 log): `.ALIGN BYTE|WORD|LONG|QUAD|PAGE|n` (n is a
  power of two), `.F_FLOATING`/`.FLOAT` and `.D_FLOATING`/`.DOUBLE`, and
  `.MASK symbol[,expression]`
- `.END [transfer]`, which sets the EOM transfer address

**Later sub-phases.** These became Phases 28, 29, and 30 when this phase
closed (see [PLAN.md](PLAN.md)):

- **The macro facility:** `.MACRO`/`.ENDM`, arguments and defaults,
  `.NARG`, `.IRP`/`.IRPC`/`.REPT`, `.MEXIT`, `.MCALL`, and `.LIBRARY`.
  Almost every real VMS program calls system macros (`$EXIT_S`, `$QIOW_S`,
  `$FAB`, and so on) from `SYS$LIBRARY:STARLET.MLB`, so this is what makes
  ordinary MACRO programs assemble. It is large enough to be its own phase.
  Until then, test fixtures use `CALLS`/`CALLG` to `SYS$…` entry points
  directly, as the existing govax fixtures do.
- **Listing file** (`/LIST`).
- **Debugger and traceback records** (`DBG`, `TBT`), for `/DEBUG` and
  traceback. `.ENABLE TRACEBACK` is on by default, so real MACRO objects
  have TBT records and the first milestone's objects won't. The linker
  doesn't need them to build a working image; they're only used for the
  traceback shown when a program fails.
- A govax **`LINK`**, so a govax-built `.OBJ` can `RUN` inside govax without
  a real VAX. This is probably its own phase, and `internal/obj`'s reader is
  its foundation.

## Validation strategy

We check our output in three ways, from cheapest to most authoritative:

1. **Self-consistency (unit tests).** Round-trip `obj` write and read
   tests, checker tests, and assembler tests that check the relocation list
   and the dumped TIR program for small sources.
2. **Comparison with real MACRO output.** The user assembles each `.MAR`
   fixture on the real VAX and returns:
   - the `.OBJ`, sent in a way that keeps record boundaries (see the open
     questions);
   - the `.LIS` listing (`MACRO/LIST`). It shows the bytes MACRO generated,
     the psect synopsis, and the symbol table, which makes it the easiest
     thing to diff against;
   - optionally, `ANALYZE/OBJECT/OUTPUT=x.ANL` output and a `LINK/MAP` map.

   Real MACRO objects include traceback (TBT) records by default (see
   [What the MACRO manual settles](#what-the-macro-manual-settles)). Our
   reader parses DBG and TBT records, and the first milestone's comparisons
   leave them out. That way the fixtures don't need special assembly
   options, and the same files serve the later traceback sub-phase.

   We decode the real `.OBJ` with our dumper and compare it with ours at
   three levels: the same GSD content (psects, attributes, sizes, symbols),
   the same bytes after relocation, and, where practical, the same TIR
   command choices. Differences go in this document. Where they're behavior
   differences rather than encoding choices, they go in `DEVIATIONS.md`.
3. **Readability both ways** (the acceptance criterion, decided
   2026-09-29). `internal/obj` reads, dumps, and checks every govax object
   and every real VAX object in the fixture set without errors. That's what
   the future govax `LINK` needs: to read its own objects and real ones.
   Nobody is expected to take a govax object to a real VAX's `LINK`, so that
   isn't required. But the format shouldn't differ from real MACRO's without
   a reason. When it's cheap, a govax object is also checked with the real
   `ANALYZE/OBJECT` and `LINK` as extra evidence.

**Fixture ladder** (`testdata/mar/`, with the real-VAX outputs stored next to
them under `testdata/mar/vax/`). Each step adds one feature:

1. An empty module: `.TITLE`, `.IDENT`, `.END`. This checks MHD, LNM, and
   EOM.
2. One data psect: `.PSECT DATA`, `.LONG 1,2,3`, `.ASCII`. This checks PSC
   and STORE IMMEDIATE.
3. `.ENTRY MAIN,^M<>` with `RET`, and `.END MAIN`. This checks EPM, the
   transfer address, and a runnable image.
4. Two psects that refer to each other (`MOVAL DATA_ITEM,R0` from code, and
   `.ADDRESS` of a code label in data). This checks relocation.
5. External calls: `CALLS #0,G^LIB$…` or `SYS$EXIT`, including the default
   mode MACRO chooses when no `G^`/`L^` is given. This checks STA_GBL and
   PICR/PIDR.
6. Globals: `::`, `==`, `.GLOBL`, `.WEAK`.
7. Branches, forward and backward, and `.ALIGN`/`.BLKB` gaps. This checks
   `CTL_SETRB` and repeated stores.
8. Complex expressions: `EXT+4`, `A-B` across psects, and `.LONG EXT1-EXT2`.
   This checks OPR commands.
9. A "real" program: a hello-world that uses `SYS$QIOW` or `LIB$PUT_OUTPUT`.
   Real MACRO's object for it goes into the reader corpus for the future
   `LINK`. Linking govax's object on the VAX is optional.

## Subtasks

1. **Done.** **Object-language constants.** Teach `internal/vmsdef/gen` the VAX
   object modules in `objfmt.sdl` and generate them. Tests check the
   generated values against Chapter 7's tables.
2. **Done.** **`internal/obj`: record model, writer, reader, dumper, and checker**,
   with unit tests on hand-built modules. There's no assembler involvement
   yet.
3. **Done.** **First real-VAX fixtures** (needs the user). Fixtures 1 to 3 from the
   ladder, assembled on the VAX. Decode them with the subtask 2 reader and
   record what we learn here: the MHD field values real MACRO uses, the LNM
   text, the psect attributes it writes for defaults, how it splits records,
   and its TIR idioms. Fix the reader wherever real objects show it's wrong.
4. **Done.** **Refactor `internal/asm` for a shared core** (no behavior change).
   Introduce the dialect setting and per-directive dialect flags, and the
   section model with absolute sections for the console dialect. Move
   eVAX-only directives behind the console dialect. Pass criterion: every
   existing `internal/asm` and `internal/console` test passes, and every
   `testdata/asm` fixture assembles to the same image and symbols as
   before.
5. **Done.** **Relocatable values.** `exprVal`, `fixup`, and `symbol` carry psect
   and external terms. Unresolved relocatable and external references become
   relocation records in MACRO dialect. Expression-rule tests (absolute vs.
   relocatable vs. complex).
6. **Done.** **MACRO-dialect directives:** the first-milestone list above.
7. **Done.** **Operand encoding for relocatable and external operands.** Implement
   the manual's displacement-size rule: the smallest size for a target
   already defined in the same psect, otherwise the `.DEFAULT DISPLACEMENT`
   size. Add `.DEFAULT` and `G^` general mode (`STO_PICR`/`STO_PIDR`).
   Check against fixtures 4, 5, and 7. If they disagree with the rule,
   reopen [Pass structure](#pass-structure).
8. **Done.** **Object emitter.** Turn sections, symbols, and relocations into
   `internal/obj` records: PSC and SYM/EPM GSD subrecords, TIR (STORE
   IMMEDIATE runs, `CTL_SETRB`, relocation stack programs), and EOM with the
   severity and transfer address.
9. **Done.** **Shared host/ODS-2 file access.** The helper that classifies a file
   name (see
   [File specifications](#file-specifications-host-files-and-ods-2-volumes)),
   the `internal/rms` record API for reading and creating record files on a
   mounted volume, and the host VAR layout reader and writer. Confirm that
   `.OBJ` files survive COPY both ways, and fix COPY or `ods2` if they
   don't. Unit tests cover each classification rule, including Windows drive
   letters, logical names, `/HOST`, and the `SET DEFAULT` rule for bare
   names.
10. **Done.** **Command surface.** The console `MACRO` verb with the source's `/HOST`
    and `/[NO]OBJECT[=file]`, the `govax macro` subcommand, a repeatable
    `--mount DEVICE=container` CLI option that mounts volumes before a
    one-shot command runs, default object naming with case copied from the
    source's extension, `.INCLUDE` across host and ODS-2, and writing only
    on success. Update the help text in `vax.help`.
11. **Done.** **Fixture ladder 4 to 9** (needs the user). Compare each fixture with the
    VAX's output and add each real object to the reader corpus. Log the
    differences.
12. **Done.** **Clean-up and docs.** Update `PLAN.md`, `DEVIATIONS.md`, and
    `CLAUDE.md` (for the new `internal/obj` package). Mark the phase
    complete, and move the macro facility, listings, and `LINK` into new
    phase entries.

Subtasks 1 and 2 don't depend on anything else and can start now. Subtask 3
can run in parallel as soon as the user has fixtures, and should come before
subtask 7, since real output decides the encoding questions.

## Open questions

None are open. All were answered on 2026-09-29, and the design sections
above record the answers:

- **Qualifier name:** VMS's `/[NO]OBJECT[=file]`, not `/OUTPUT`.
- **Fidelity:** `internal/obj` (and the future `LINK`) must read both govax
  objects and real VAX objects. Linking govax objects with a real VAX
  `LINK` isn't required, but the format shouldn't differ from real MACRO's
  without a reason.
- **Default object name:** the source's own location, with an `OBJ`
  extension in the same case as the source's extension.
- **Errors:** no object file is written.
- **LNM record:** it names govax's own MACRO, `govax MACRO V<build>`.
- **ODS-2 output:** in scope, including any `ods2` changes it needs.
- **Host vs. ODS-2:** the source takes COPY's parameter-scoped `/HOST`;
  obvious VMS syntax means a volume. A bare name means the mounted volume
  when a `SET DEFAULT` onto one is in effect, and the host otherwise.
- **One-shot CLI:** a repeatable `--mount DEVICE=container` option.
- **Where the real objects come from:** the user's simh VAX 8600 running
  VMS 7.3. Its disk is a container that `ods2` can read. The user pauses
  simh before govax reads it, and govax mounts it **read-only**. Each
  fixture's `.OBJ` (with its file attributes) and `.LIS` listing are copied
  into `testdata/mar/vax/`. The `.MAR` sources go to the VAX on a small
  separate exchange container that govax creates.

## Progress Log

### 2026-09-29 — Planning

- The user asked for a plan for `.MAR` → `.OBJ` object-module output, and
  asked that the existing `ASM` functionality be reused (refactoring if
  needed) rather than a second assembler written.
- Searched the document archive for object-format material. It's complete:
  VMS 5.0 Linker manual chapter 7, the VAX-11 v2.0 Linker manual appendix C,
  `objfmt.sdl` for constants, and the ANALYZE/OBJECT and linker source
  listings for behavior. Reverse engineering the format isn't needed. The
  user's real VAX will instead be used to learn which encoding choices
  MACRO makes and to confirm linker acceptance.
- Confirmed from the manual: STORE IMMEDIATE is a negative TIR command byte
  (1 to 128 bytes); records are at most 2048 bytes; the TIR stack must be
  empty at EOM.
- Noted the gap: there's no VAX MACRO language manual in the archive. Use
  the VSI online manual plus real-VAX output. (The user filled this gap the
  same day; see the next entry.)
- Reviewed `internal/asm`. It is one-pass and absolute, and its forward
  fixups are already linear symbol expressions, which is the natural base
  for relocations. The eVAX console directives need to move behind a
  dialect setting.

### 2026-09-29 — VAX MACRO manual added

- The user added the *VAX MACRO and Instruction Set Reference Manual*
  (OpenVMS VAX 7.3), in the Compaq AA-PS6GD-TE and VSI printings. It's now
  listed under [References](#can-we-build-a-valid-object-file-yes).
- The manual settles several things the plan had left open. They're recorded
  in [What the MACRO manual settles](#what-the-macro-manual-settles):
  - Displacement sizes: the smallest size only for a target already
    defined in the same psect; otherwise the `.DEFAULT DISPLACEMENT` size,
    which is a longword by default.
  - The `. ABS .` and `. BLANK .` psects and the attribute defaults.
  - `.MAIN.` as the module name when there's no `.TITLE`.
  - The `.ENABLE` defaults (GLOBAL and TRACEBACK are on).
  - Which directives can't use forward or external symbols.
- **Pass structure decided:** one pass with fixups. The displacement rule is
  decided when the operand is read, so a sizing pass isn't needed to match
  MACRO. The fixtures will confirm it.
- The fixture plan now handles TBT records: real objects will have them, so
  the reader parses them and the first milestone's comparisons leave them
  out.
- Still open: the manual doesn't cover the DCL `MACRO` qualifiers
  (`/OBJECT` vs. `/OUTPUT`), so open question 1 stands.

### 2026-09-29 — User decisions; host and ODS-2 file access

- The user answered every open question (see
  [Open questions](#open-questions)): `/[NO]OBJECT`, no object on errors,
  the object's extension case follows the source's, the LNM record names
  govax, and "reads both govax and real objects" replaces real-VAX `LINK`
  as the acceptance test.
- The user asked for MACRO, and later LINK, to work equally with host files
  and files on mounted ODS-2 volumes, with `ods2` changes in scope. Added
  [File specifications](#file-specifications-host-files-and-ods-2-volumes).
  It covers the classification rules (COPY's `/HOST`, obvious VMS syntax,
  obvious host paths, and bare names following `SET DEFAULT`), the object
  following the source's side, an `internal/rms` record API built on
  Phase 23's `rms.Session`, the host VAR layout, and COPY round trips.
  Added subtask 9 for this, and renumbered the rest to 12.
- Checked: Phase 22 already writes VAR records through `ods2`'s
  `rms.Writer`, so `.OBJ` output itself shouldn't need `ods2` changes.
  `ods2` commits must not carry AI attribution lines (`ods2/CLAUDE.md`).
- A repeatable `--mount DEVICE=container` CLI option mounts volumes for
  one-shot commands (subtask 10).
- Real objects will come from the user's simh VAX 8600 running VMS 7.3.
  govax reads its disk container read-only while simh is paused.
- (These edits were written on 2026-09-29, but a sandbox failure in the
  editor held them up until after a restart.)

### 2026-09-30 — Subtask 1: object-language constants

- Copied `objfmt.sdl` from the VMS 7.3 source archive to
  `reference/vms/objfmt.sdl`. `internal/vmsdef/gen` now takes it as
  `-objfmt`, parses only the VAX modules (everything before
  `module $EOBJRECDEF;`, where the Alpha definitions start), and emits
  `vmsdef.OBJConstants`: 635 names covering record types, header and GSD
  subrecord types, TIR commands, psect and symbol flags, and every record's
  field offsets.
- The SDL parser (`gen/sdl.go`) only understood bitfield aggregates, so it
  now models real record layouts: a stack of nested structures and unions
  tracking byte offsets and bit runs; byte, word, longword, quadword, and
  character fields with `length`, `dimension`, `prefix`, and `tag`; typed
  aggregates such as `FLAGS union word unsigned`; `constant X equals .`;
  `origin FIELD`; and `ifsymbol` blocks. Output for every existing module is
  byte-identical, checked by regenerating and diffing
  `constants_generated.go` before adding the new map.
- Three old "rejects unsupported" test cases (a byte field, a typed nested
  aggregate, a bitfield directly in a union) are now supported forms and
  were replaced by positive layout and `origin` tests, plus new rejections
  (an unknown field type, `length` on a word, a missing origin field, `.`
  outside an aggregate).
- `TestOBJConstants_values` checks the generated values against chapter 7's
  tables and record diagrams.
- Faithful oddity, noted in the map's doc comment: `$OBJRECDEF`'s SDA-only
  `SDADEFS` aggregate puts its flag bitfields directly in a union, so SDL
  places every `OBJ$V_PSC_*`/`OBJ$V_SYM_*` at bit 0. Use the `GPS$` and
  `GSY$` flags, which are laid out correctly.

### 2026-09-30 — Subtask 2: `internal/obj`

- New package `internal/obj`, with every code and layout taken from
  `vmsdef.OBJConstants`:
  - **Record model** (`module.go`). `Module` keeps every record in order:
    `MainHeader`, `TextHeader`, `GSD` with its subrecords, `TIR` (also used
    for DBG and TBT, which carry the same commands), `EOM` (and EOMW), `LNK`,
    and `Unknown`. That makes `Encode(Decode(records))` reproduce a real
    object byte for byte. All 19 GSD subrecord types are decoded, since a
    subrecord has no length field and the reader must know every layout to
    step past one. The symbol forms share one `Symbol` type driven by a
    per-type layout table: plain, word-psect, vectored, version-mask, and
    module-local, each as a symbol, entry point, or procedure.
  - **TIR commands** (`tir.go`). All 65 commands (codes 0-19, 20-42,
    50-66, 80-84) plus STORE IMMEDIATE, with each command's operand format
    and stack effect.
  - **Host record layout** (`varfile.go`). `ReadRecords`/`WriteRecords`
    use ODS-2's on-disk variable-length layout: a 2-byte length, the data,
    and a pad byte to an even offset. The reader honors the 0xFFFF
    end-of-block marker, so a raw block copy of a VMS file also reads.
  - **Checker** (`check.go`). Record order, record sizes, MHD fields, name
    lengths, psect indexes (in symbols, TIR commands, and the transfer
    address), the linker's stack (no underflow, at most 25 longwords, empty
    at the end), TIR globals that the GSD doesn't declare, and reserved
    severities and flags.
  - **Dumper** (`dump.go`). One line per record, subrecord, and command,
    in the spirit of ANALYZE/OBJECT.
  - **Builder** (`builder.go`). Takes psects, symbols, commands
    (`SetLocation`, `Store`, which merges bytes into STORE IMMEDIATE runs of
    at most 128, and `Emit`), and a transfer address, and packs them into
    records under a limit without splitting a subrecord or command.
- Details confirmed from ANALYZE/OBJECT's source
  (`analyz/lis/objgsd.lis`): a procedure has one formal argument
  descriptor per its *maximum* argument count, and an IDC ident is always a
  counted string (4 bytes when binary).
- Decisions to confirm against real objects in subtask 3:
  - the MHD patch-time field is written as 17 zero bytes, taking the
    manual's "padded with 17 zeros" literally;
  - the builder's default record limit is `OBJ$C_MAXRECSIZ` (2048);
  - the checker's rule that every `STA_GBL` name must also be in the GSD.
- `TestRealObjects` will read every real VAX object placed in
  `testdata/mar/vax/`. It checks the byte-for-byte round trip and a clean
  `Check`, and skips while there are none.
- Test coverage is 87% (the rest is mostly error paths).

### 2026-09-30 — Subtask 3 prepared: fixtures and exchange container

- Wrote the whole fixture ladder at once (`testdata/mar/*.mar`, steps 1 to
  9), so one simh session covers everything. Also wrote
  `testdata/mar/assemble.com`, which does the VMS side: `MACRO/LIST` and
  `ANALYZE/OBJECT` for each fixture, `LINK/MAP` and `RUN` for `entry` and
  `hello`, and `DIRECTORY/FULL` of the objects for their record attributes.
  `testdata/mar/README.md` describes the round trip.
- Built the exchange container with govax's own commands, which also
  exercises the host-to-ODS-2 path: `INITIALIZE/CONTAINER`, `MOUNT /WRITE`,
  and `COPY .../HOST` for each file. The result is
  `testdata/disks/mar-exchange.dsk` (4000 blocks, volume label MARXCHG,
  gitignored like the other containers).
- Found along the way, outside this phase:
  - `COPY` doesn't expand a wildcard in a host source (`*.mar/HOST`), so
    each file was copied by name;
  - `COPY .../HOST` creates STREAM_LF files, which VMS RMS and MACRO read
    fine;
  - `DIRECTORY DKA1:[000000]` heads its listing "Directory DKA1:[]", where
    VMS shows "[000000]".
- Waiting on the user to attach the container to simh, run `@ASSEMBLE`,
  and pause simh so govax can copy the results into `testdata/mar/vax/`.

### 2026-09-30 — `ods2` interop bugs found while exchanging fixtures

- VMS 7.3 refused to mount the govax-built exchange container
  (`%MOUNT-F-NOHOMEBLK`). Phase 22's interop tests only ever had govax
  read disks VMS made; this was the first time VMS read a volume `ods2`
  initialized. MOUNT's home block test (`CHECK_HOMEBLK2`, in
  `vmssrc_archive/v73/mount96/lis/chkhm2.lis`) requires `HOMELBN` to equal
  the LBN the block was read from, and these to be nonzero: `ALTIDXLBN`,
  `CLUSTER`, `HOMEVBN`, `ALHOMEVBN`, `ALTIDXVBN`, `IBMAPVBN`, `IBMAPLBN`,
  `MAXFILES`, `IBMAPSIZE`, and `RESFILES`. It also requires both checksums
  to be correct.
- `ods2`'s `volume.Initialize` fails five of those checks: `ALTIDXLBN`,
  `HOMEVBN`, `ALHOMEVBN`, and `ALTIDXVBN` are 0, and
  `ondisk.EncodeHomeBlock` never computes `CHECKSUM1`. Behind the zero VBNs
  is a nonstandard `INDEXF.SYS` layout. VMS `INIT`'s `INIT_INDEX`
  (`init/lis/inindx.lis`) lays it out as a boot block cluster, two home
  block clusters (every block a home block copy, with the secondary home
  block at a geometry-derived LBN), a backup index file header cluster,
  then the index bitmap (`(MAXFILES+4095)/4096` blocks), then the headers.
  That gives `HOMEVBN` 2, `ALHOMEVBN` 3c (for the usual placement),
  `ALTIDXVBN` 3c+1, and `IBMAPVBN` 4c+1. VMS 7.3 also has 10 reserved
  files (file 10 is `SECURITY.SYS`); `ods2` makes 9. Rewriting `Initialize`
  to follow `INIT_INDEX` is its own task, tracked below as `ods2` work.
- Workaround, and a reference: the user ran `INITIALIZE DUA1: MARXCHG` on
  simh with RQ1 set to an RD51 (21,600 blocks). An RD54-sized device
  didn't match a small container, and simh couldn't autosize it. The
  pristine volume is kept as `testdata/disks/vms-init-rd51.dsk`
  (gitignored) as the byte-level reference for the `Initialize` rewrite.
  simh 4 appends a 512-byte metadata footer, which makes the file 21,601
  blocks.
- Copying the fixtures onto that volume found a second `ods2` bug: VMS
  `INIT` preallocates only 16 header slots in `INDEXF.SYS`, and `ods2`
  never extended the index file, so the 7th new file failed ("virtual block
  23 is beyond the end of the file"). Fixed in `ods2` (`39bfb8e`):
  `CreateHeader` and new extension segments grow `INDEXF.SYS` (by at least
  16 blocks, zeroed, with its end of file and high-water mark moved past
  them) when their slot lies beyond its mapped blocks. There's also a new
  `InitializeOptions.Headers` (`INITIALIZE/HEADERS`). The exchange volume
  now holds all nine fixtures and `ASSEMBLE.COM`, and its `INDEXF.SYS` grew
  from 22 to 38 blocks.
- **`ods2` work still to do** (in scope for this phase, per the user):
  rewrite `volume.Initialize` to match VMS `INIT_INDEX`. That means the
  home block fields and `CHECKSUM1`, home block copies, secondary home
  block and backup index header, the `INDEXF.SYS` layout and preallocation,
  and the tenth reserved file. Check it structure by structure against
  `vms-init-rd51.dsk`, and confirm VMS mounts the result.

### 2026-09-30 — Third `ods2` bug: HEADERFULL; exchange volume rebuilt

- The user's first `@ASSEMBLE` on the exchange volume produced EMPTY,
  DATA, ENTRY, and RELOC (ANALYZE/OBJECT: 0 errors each). Every file
  created after that failed with `SYSTEM-W-HEADERFULL`. By then the 32
  header slots of the grown `INDEXF.SYS` were used up (10 reserved, 10
  fixtures, 12 new VMS files), and VMS couldn't extend the index file.
- Cause: `ods2`'s `ondisk.EncodeFileHeader` put the ACL area right after
  the retrieval pointers in use. VMS counts a header's free map room as
  `ACOFFSET - MPOFFSET - MAP_INUSE`, so every header `ods2` wrote looked
  full, and VMS could not extend any file `ods2` had written. That
  included `INDEXF.SYS`, whose header `ods2` had re-encoded when it grew
  the file.
- Fixed in `ods2` (`f1e6758`): the ACL goes at the end of the header, and
  with no ACL both `AclOffset` and `EndOffset` are 255, as VMS writes them.
  Nothing else in `ods2` relied on the old packing: map reads use
  `MapWordsInUse`, and `existingAreas` rebuilds the map from the decoded
  retrieval pointers.
- Rebuilt `mar-exchange-vms.dsk` from the pristine `vms-init-rd51.dsk`
  with all ten files. Its `INDEXF.SYS` header now has 131 free map words.
- The `ods2` `Initialize` rewrite (VMS-mountable volumes) is still to do.

### 2026-09-30 — ANALYZE/DISK_STRUCTURE findings; three more `ods2` fixes

- The HEADERFULL fix wasn't enough: the second `@ASSEMBLE` failed on the
  first new file. VMS created each file's header, then failed to enter it
  in `000000.DIR`. The user ran `ANALYZE/DISK_STRUCTURE`, `DUMP/HEADER`,
  and a single `CREATE` on a freshly rebuilt volume. ANALYZE reported
  three kinds of problem, each traced to an `ods2` bug and fixed:
  - **`BADDIR` / `BAD_DIRTYPE`**, which rejected the whole MFD and made
    every file "not found in a directory". ANALYZE's rule
    (`verify/lis/verify_dir.lis`) rejects a directory record whose version
    limit is 0 or has its high bit set. `ods2` dropped version limits on
    decode and wrote 0 on encode, so any directory it rewrote became
    invalid; VMS's own entries lost their limit of 1. Fixed (`d616929`):
    `DirEntry.VersionLimit` survives decode and encode, a name with none
    gets 32767 (`NoVersionLimit`), a new name takes its directory's
    default, and `Initialize` gives reserved files 1.
  - **`FUTCREDAT` / `FUTREVDAT`** (dates in the future). `vmstime` treated
    tick counts as UTC, but VMS keeps local wall-clock time. Fixed
    (`5ebb6d7`): conversions and `ParseVMSTime` use `vmstime.Location`,
    which is the host's local zone by default. The tests pin UTC, and a new
    test checks the zone behavior; the suite passes under
    `TZ=America/New_York` too.
  - **`ALTIHDBAD`**. The copy of `INDEXF.SYS`'s header at `ALTIDXLBN`
    (identical to the primary on a VMS volume) went stale when `ods2`
    rewrote the primary. Fixed (`84a3805`): writing `INDEXF.SYS`'s header
    writes the copy too.
- Rebuilt `mar-exchange-vms.dsk` from `vms-init-rd51.dsk` and checked it.
  The MFD's version limits are 1 and 32767, the alternate index header
  matches the primary, and file dates match the host's local clock.
- Lesson recorded for the `Initialize` rewrite: run the result through
  `ANALYZE/DISK_STRUCTURE` on VMS. It names exactly which structure is
  wrong, which was far quicker than inferring from failures.

### 2026-09-30 — Subtask 3: real VAX objects in hand

- With the `ods2` fixes, the user's `@ASSEMBLE` ran cleanly:
  `ANALYZE/DISK_STRUCTURE` reported nothing but the missing `QUOTA.SYS`,
  all nine objects passed `ANALYZE/OBJECT` with 0 errors, and `HELLO`
  linked and ran ("Hello, world!").
- Copied the results out with govax, with the volume mounted read-only.
  The objects were copied with `COPY/BINARY`, which gives their raw
  on-disk bytes, the variable-length layout `obj.ReadRecords` reads. The
  listings (`.lis`), analyses (`.anl`), and link maps (`.map`, for `entry`
  and `hello`) were copied as text. All are in `testdata/mar/vax/`.
  (`OBJECTS.DIR` didn't come across; the record attributes it would show
  can be read from the container directly.)
- `TestRealObjects` passes for all nine: each decodes, re-encodes byte for
  byte, and passes `Check`. That needed one checker fix. Real MACRO
  **interleaves GSD and TIR records**, so the "no GSD after text" rule was
  wrong and is gone. The rule that every `STA_GBL` name is in the GSD holds
  for all nine.
- What real MACRO (`VAX MACRO V5.4-3`) writes. These are the conventions
  subtasks 7 and 8 should follow, since the format shouldn't differ
  without a reason:
  - **Headers:**
    - MHD maximum record size **512** (so `obj.DefaultRecordLimit` is now
      512), and actual records are small, at most 50 bytes here;
    - patch time is **17 spaces**, not zeros (the builder now writes
      spaces);
    - LNM `VAX MACRO V5.4-3`;
    - a **SRC** header holding the command line (`MACRO/LIST ENTRY`);
    - a TTL header with the `.TITLE` comment, cut to 40 characters.
  - **Traceback records** (TBT) come right after the headers and just
    before the EOM, as TRACEBACK's default implies.
  - **Record order:** records are emitted as assembly proceeds. External
    references come first, as one GSD of `SYM` references, each flagged
    `REL`. Then psect 0, `. ABS .` (always defined, even when empty), then
    each psect's `PSC` just before its first text, and each entry point's
    `EPM` next to its code. `. BLANK .` isn't defined when nothing uses it.
  - **Setting the location:** `STA_PB psect offset` + `CTL_SETRB`. Psect
    offsets use the shortest stack command (`STA_PB`, falling back to
    `STA_PL`).
  - **Entry mask:** `STA_UB mask`, then the `EPM` GSD record, then
    `STO_W`.
  - **Data and code bytes:** STORE IMMEDIATE, in runs broken wherever a
    relocation intervenes.
  - **Gaps** (`.ALIGN`, `.BLKB`): `CTL_AUGRB n`, never stored zeros.
  - **`.ADDRESS label`:** `STA_PB` + `STO_PIDR`.
  - **Relative operands, per the manual's displacement rule:**
    - a backward reference in the same psect is finished by MACRO in the
      smallest form (`AF FD`);
    - a forward reference, a label in another psect, or an external gets
      the default: the mode byte as immediate data (`EF`), then
      `STA_PB`/`STA_GBL` + `STO_LD`;
    - after `.DEFAULT DISPLACEMENT,WORD`, it's `CF` + `STO_WD`.

    Even a same-psect forward reference is left for the linker to
    finish.
  - **`G^` (general mode):** `STA_GBL` + `STO_PICR`, and the linker writes
    the mode byte.
  - **Expressions:** `EXT+4` is `STA_GBL`, `STA_UB 4`, `OPR_ADD`.
    Differences, products, and masks are all TIR arithmetic, including
    `B-A` across psects and even `<C-A>*2` within one psect. MACRO leaves
    them all to the linker.
  - **The EOM** has a transfer address only when `.END` names one.
- The fixtures are a regression suite for the reader and checker now, and
  the reference for the assembler's object output later.

### 2026-09-30 — `ods2` `Initialize` rewritten to match VMS INITIALIZE

- The user asked to fix `ods2` volume initialization now ("we'll need this
  again soon") and made more reference volumes on simh with a plain
  `INITIALIZE`: an RX33 (`rq1-rx33.dsk`, 2400 blocks, a "small" disk) and
  an RD54 (`rq3-rd54.dsk`, 311200 blocks, cluster factor 3). These join
  the RD51. All three were mounted once on VMS, which leaves a lock name
  and mount time in the storage control block.
- The algorithm comes from VMS 7.3 INIT's own source (`init/lis/`):
  - `inidsk.lis`: the defaults. The cluster factor is 1 up to 50000
    blocks, and otherwise max(3, whatever keeps `BITMAP.SYS` within 255
    blocks). Maximum files is `MAXBLOCK/((c+1)*2)`. There are 16 headers
    and room for 16 MFD entries. The index file goes at `MAXBLOCK/2`, or
    at 0 on a disk of 4096 blocks or fewer.
  - `iniall.lis`: the allocation table, where each structure goes in the
    first free position, rounded to clusters. The secondary home block is
    the first free LBN on the sequence 1, 1+delta, and so on.
  - `get_delta.lis`: delta is `HM2$C_GEOM_INDEPEND_DELTA`, 1033, or 1 when
    that exceeds a tenth of the volume.
  - `inindx.lis`: the home block copies and every reserved header.
- One more piece of knowledge came from the references: the first
  longword of `SECURITY.SYS`'s security profile is the XOR of the whole
  longwords after it. The kernel routine that builds it isn't in the
  archive.
- `ods2` commits (no attribution, per its rules):
  - `ccd3e7b`: headers laid out as VMS lays them out. The IDENT area is at
    word 40 with the map at 100. `RECPROT` is decoded and encoded. Names
    are recorded as `NAME.TYP;VER`, padded with spaces
    (`ondisk.IdentName`, `NewFileHeader.Version`).
  - `8429d65`: `Initialize` rewritten to follow INIT. It computes
    `CHECKSUM1`, writes home block copies, and adds a tenth reserved file,
    `SECURITY.SYS` (`ondisk.SecurityFileFid`, `ReservedFileCount` 10;
    older volumes' own `ReservedFiles` still governs). It records disk
    geometry (known DEC disk sizes, or `InitializeOptions.Geometry`), puts
    a partial last cluster in `BADBLK.SYS`, and exposes INIT's qualifiers
    as options.
- govax `52bb2b8` updates the attribute tests for the new names.
  `01bd971` adds `TestVMSInitializeFidelity`, an opt-in test that
  initializes a scratch volume the size and label of each reference and
  compares block by block. Only timestamps, the boot block (which INIT
  fills with a PDP-11 "not a system disk" stub that `ods2` leaves zeroed),
  and checksums over them are masked. **All three references match.** A
  deliberately wrong label makes the test fail, confirming it can.
- Built `testdata/disks/ods2-init-rd51.dsk` with govax's own
  `INITIALIZE/CONTAINER` and two files copied on, for the user to mount on
  VMS and check with `ANALYZE/DISK_STRUCTURE`.
- **Confirmed on VMS** (2026-09-30). simh recognized `ods2-init-rd51.dsk`
  as an ODS-2 volume (label ODS2INIT, 21600 sectors), and VMS 7.3 mounted
  it. `ANALYZE/DISK_STRUCTURE` reported nothing but the missing
  `QUOTA.SYS`, and `DIRECTORY` listed the ten reserved files and the two
  files govax copied. VMS then assembled, linked, and ran `HELLO` on it
  ("Hello, world!"). So an `ods2`-initialized volume, and files `ods2`
  wrote to it, are fully usable by VMS. The `ods2` interop work in this
  phase is complete.
- Paused before subtask 4 (the assembler refactor) at the user's request,
  so they can push.

### 2026-09-29 — Subtask 4: a shared assembler core

- **Proof of no behavior change first.** `TestGoldenFixtures`
  (`internal/asm/golden_test.go`, committed before any refactoring)
  snapshots every `testdata/asm` fixture's assembly into
  `internal/asm/testdata/golden/`: every byte written, every symbol with
  its value and flags, the final P0 and S0 locations, the `.END` entry,
  and `.PRINT` output. Each fixture is assembled on its own and again
  after `kernel.asm` in the same `Assembler`, the way a console session
  that booted the microkernel assembles it. All 58 fixtures assemble
  cleanly both ways. The snapshots are unchanged after the refactor, and
  `go test -run TestGoldenFixtures -update` rewrites them after a
  deliberate change.
- **Sections** (`section.go`). The single `deposit` counter and its
  `.REGION` bookkeeping (`origin`, `p0Deposit`, `s0Deposit`, `s0Origin`,
  `regionIsS0`) are now two absolute sections, P0 and S0, each a base
  address plus a location offset, and `cur`, the one output goes to.
  `.REGION` just switches `cur`. Everything that emitted bytes now goes
  through `pc`/`setPC`/`advance` and `emitByte`/`emitBytes`/`emitWord`/
  `emitLongword`/`emitScaled`, so subtask 5 can change what a location
  is in one place. The public API (`Origin`, `Deposit`, `S0Origin`,
  `S0End`, `Bytes`, and the setters) is unchanged. One edge moved:
  `SetOrigin` after a `.REGION S0` used to move the S0 counter to the P0
  address; now it only resets P0. Nothing calls it that way.
- **Dialects and the directive table** (`directive.go`). `Dialect` has
  `DialectConsole`, the default, and `DialectMACRO`, set with
  `SetDialect`. `pseudoNames` and the `dispatchPseudo` switch are
  replaced by one `directives` table, where each entry has its dialects
  and its function. In the MACRO dialect, a console-only directive is
  the new `VAX_NOTMACRO` ("!S is not a MACRO-32 directive"), and a
  directive needs its leading ".", so bare `JEQL` or `BYTE` reaches the
  instruction table as it would in MACRO-32. The console dialect accepts
  everything it did before.
- **Which directives are shared.** A directive is in both dialects only
  if MACRO-32 (Table 6-1 of the manual) has it with the same syntax and
  meaning: `.BYTE`, `.WORD`, `.LONG`, `.QUAD`, `.ASCII`/`Z`/`C`/`D`, the
  `.BLKx` forms, `.END`, `.ENTRY`, and the `.IF` family and `.IIF`. So is
  `.INCLUDE`. MACRO-32 has no `.INCLUDE`, but subtask 10 resolves it
  across host and ODS-2 files for the MACRO command. Besides the
  eVAX-only directives this plan already listed, three more turned out
  to be console-only, because MACRO-32 has the name with a different
  meaning:
  - `.ALIGN n`: MACRO-32 aligns to 2^n bytes, or takes a keyword.
  - `.MASK`: MACRO-32's reserves a transfer vector's mask word for a
    symbol.
  - `.PRINT`: MACRO-32's prints its comment.

  eVAX's `.F_FLOAT`/`.D_FLOAT` aren't MACRO-32 names, and neither are
  `.SPACE`, `.CASE`, `.SCOPE`, `.DATA`, or `.TEXT`. So the
  first-milestone list above now names MACRO-32's own forms, which
  subtask 6 adds.
- Tests: `dialect_test.go` checks that every console-only table entry is
  `VAX_NOTMACRO` in the MACRO dialect, that the shared directives
  assemble there, that the dot is required, and that `.REGION`/`.BASE`
  still work with the new sections. `go test ./...` passes.

### 2026-09-29 — Subtask 5: relocatable values

- **A tree, not a linear combination.** The plan proposed extending the
  forward-reference form (`constant + Σ coeff·symbol`) with psect and
  external terms. Real MACRO's objects (subtask 3) rule that out:
  - `EXT2&^XFF` is a mask, which no linear form can hold.
  - MACRO keeps each expression's shape as written. `EXT1*2` is
    `STA_GBL`, `STA_UB 2`, `OPR_MUL`, and `ITEM+4` is `STA_PB 1,0`,
    `STA_UB 4`, `OPR_ADD`, not `STA_PB 1,4`.

  So a value the assembler can't finish is now an expression tree
  (`rexpr`, in the new `internal/asm/reloc.go`). Its leaves are
  constants, a psect base plus an offset, and symbols not yet defined,
  and its operators are the source's. `exprVal` is a constant or such a
  tree, and a `fixup` holds a tree and its section. The `addend`/`terms`
  form and `pendingTerm`/`fixupTerm` are gone.
- **What folds.** Operations on constants are done at once. The one
  other simplification is the manual's §3.5 rule: the difference of two
  labels already defined in the same psect is absolute. A label defined
  later becomes its psect base when it's defined, and nothing else
  folds. So `<C-A>*2`, with C a forward reference, stays a
  subtraction and a multiplication, as real MACRO wrote it. A bare
  `SYM-SYM` is still 0.
- **When a fixup completes.** When its last symbol is defined, its tree
  is resolved:
  - A constant is stored exactly as before.
  - So is a displacement to a location in the fixup's own psect, whatever
    the psect's base.
  - Anything else becomes a **relocation** (psect, offset, kind, tree),
    and its field is left zero.

  A value that uses only a psect base (a label already defined in a
  relocatable psect) waits on no symbol. Its fixup completes at the end
  of its statement (`flushReady`), because operand parsers adjust the
  fixup they just queued. At the end of a MACRO-dialect assembly
  (`finish`), every symbol still undefined is marked `SymExternal`
  (GLOBAL is on by default), and the fixups waiting on it become
  relocations, sorted by psect and offset. A local label can't be
  external, so an undefined one is still `VAX_UNDEFSYM`.
- **Sections.** Each section now has an index (its psect number), a
  `relocatable` flag, its own image (the console's P0 and S0 share the
  one absolute image), and `hi`, the highest location reached (the
  psect's allocation). `SetDialect(DialectMACRO)` replaces P0 and S0 with
  `. ABS .` (absolute) and `. BLANK .` (relocatable, where assembly
  starts). A label in a relocatable psect is defined as (psect, offset)
  (`symbol.sect`), and `.` there is the psect base plus the location.
  In the MACRO dialect, `Bytes` returns the current psect's contents.
- **A minimal `.PSECT name`** (MACRO dialect only). The tests need more
  than one psect before subtask 6. It switches to the psect, creating it
  the first time, ends the local label block, and ignores anything after
  the name. Subtask 6 adds the attributes, the rules for the default
  psects, and `.SAVE_PSECT`/`.RESTORE_PSECT`. In the console dialect,
  `.PSECT` is the new `VAX_MACROONLY` ("!S is only valid in MACRO-32
  source").
- **Where a value must be absolute.** `exprNoForward` (`.BLKx`, `.ALIGN`,
  `.IF`, and so on) now requires an absolute value, and a relocatable one
  is the new `VAX_RELEXPR`. A direct assignment (`X = A+4`) may be
  relocatable if it's a label plus or minus a constant (the manual's
  rule). The symbol then gets that psect and offset. `. =` must stay in
  its own psect. `.END MAIN` records a relocatable transfer address
  (`entrySect`) for the EOM record. The console's `__ENTRY` symbol isn't
  defined in the MACRO dialect.
- **Operators.** The MACRO dialect gives any operator on an unfinished
  value to the linker: `/`, `@`, `&`, `!`, `\`, unary minus (`NEG`), and
  `^C` (`COM`). The console dialect keeps `VAX_FWDOPERATOR` for anything
  but adding, subtracting, or multiplying a forward reference by a
  constant. One corner of it changed: a forward reference cancelled
  only in `SYM-SYM`, where the old linear form also cancelled
  `B+1-B`. That now waits for B. The stored bytes are the same, and no
  fixture depends on it (the golden snapshots are unchanged).
- **`.ASCID`** in the MACRO dialect relocates its address field as
  `.+4` with a new `fixAddress` kind, which is `STO_PIDR`. That's what
  real MACRO wrote for hello.mar, and `.ADDRESS` will use it too.
- **Left for subtask 7.** Every value that uses a psect base is
  currently deferred like a forward reference, so a relative operand to
  one gets a longword displacement:
  - For a label in another psect, or an external, that's already what
    MACRO does.
  - For a label already defined in the same psect, MACRO uses the
    smallest displacement.
  - For a forward reference in the same psect, MACRO leaves the
    displacement to the linker (`STO_LD`), where govax finishes it
    itself. Branches to the same psect are finished by both.
- **Tests** (`reloc_test.go`). `TestRelocationsMatchRealMACRO` assembles
  `testdata/mar/exprs.mar` (without `.TITLE`/`.IDENT`, which subtask 6
  adds). Each of its eight relocations has the shape of the TIR program
  real MACRO wrote for that line, the externals are EXT1 and EXT2, and
  DATA's allocation is 23, as in the real PSC record. Other tests cover
  the absolute rules, relocatable operands and data, branches within a
  psect, the `.ASCID` pointer, the transfer address, operators, the
  places a value must be absolute, and `.PSECT`. The console golden
  snapshots are unchanged, and `go test ./...` passes.

### 2026-09-29 — Subtask 6: MACRO-dialect directives

- **The first-milestone directives** are in the directive table:
  - `.TITLE`, `.IDENT`, and `.SUBTITLE`/`.SBTTL`, which is ignored until
    listings exist (done in Phase 29). `preprocessLine` keeps the case of `.TITLE`'s comment
    and of `.IDENT`'s string. The module name is uppercased and cut to 31
    characters, and the comment is cut to 40. Without a `.TITLE`, the
    module is `.MAIN.`.
  - `.PSECT` with its attributes and alignment (new `psect.go`). A
    psect's attributes are kept as the object language's `GPS$M_` bits,
    ready for the PSC record. A continuation may repeat attributes but
    not change them. An `ABS` psect defines offsets: its labels are
    absolute, `.BLKx` moves its location, and code or data in it is the
    new `VAX_ABSDATA`. There's a limit of 254 named psects.
  - `.SAVE_PSECT [LOCAL_BLOCK]`/`.RESTORE_PSECT` (and `.SAVE`/`.RESTORE`),
    with a 31-entry stack. A saved local label block isn't checked when a
    `.PSECT` ends it, since `.RESTORE_PSECT` returns to it.
  - Global symbols. `::`, `==`, and `.ENTRY` define globals, and
    `.GLOBAL`/`.GLOBL`, `.EXTERNAL`/`.EXTRN`, and `.WEAK` declare them.
    The manual gives `.GLOBAL` and `.EXTERNAL` the same meaning for a
    symbol the module doesn't define, so they share one implementation.
    A declared symbol that's never defined is external. A local label
    can't be global (the new `VAX_NOTGLOBAL`).
  - `.ENABLE`/`.DISABLE` (and `.ENABL`/`.DSABL`), in long and short
    forms. `GLOBAL` is implemented: with it disabled, an undefined symbol
    not declared external is `VAX_UNDEFSYM`. `LOCAL_BLOCK` holds a local
    label block open across labels and `.PSECT`s, as §3.4 describes.
    `ABSOLUTE`, `DEBUG`, `SUPPRESSION`, and `TRACEBACK` are recorded.
    `TRUNCATION` and `VECTOR` aren't supported, so enabling one is the
    new warning `VAX_IGNORED`. Warnings are collected (`Warnings()`),
    each naming its line, and assembly goes on.
  - `.DEFAULT DISPLACEMENT,BYTE|WORD|LONG` is recorded (longword by
    default).
  - MACRO-32's own `.ALIGN` (a keyword, or a power of two from 0 to 9,
    with an optional fill; more than the psect's alignment is
    `VAX_ALIGNPSECT`) and `.MASK symbol[,expression]`. `.MASK` is a word
    relocation holding a new `rMask` leaf, the entry point's mask for the
    linker to supply (`STA_EPM`), ORed with the expression. `.ALIGN` and
    `.MASK` now have one table entry each that picks the console or
    MACRO form (`byDialect`), so the console dialect's forms don't
    change.
  - `.ADDRESS` (a `fixAddress` longword, `STO_PIDR`), and the
    floating-point names `.F_FLOATING`/`.FLOAT` and `.D_FLOATING`/
    `.DOUBLE`.
  - `.ENTRY` in the MACRO dialect is global, and its mask is any absolute
    expression that doesn't use R0, R1, AP, or FP (the new
    `VAX_ENTRYMASK`). The mask is kept with the symbol for the EPM
    record.
- **The default psects, as real MACRO names and numbers them.** Its
  objects and listings name the absolute psect `.  ABS  .`, with two
  blanks on each side of ABS, although the manual prints `. ABS .`.
  Listings in the VMS source archive show `. BLANK .` with one blank on
  each side, so both names are nine characters. Assembly now starts in
  `.  ABS  .`, and `. BLANK .` is defined only when a label, code, or
  data comes before any `.PSECT` (`useBlankPsect`). So an unused
  `. BLANK .` doesn't take a psect number, which matches the fixtures:
  DATA is psect 1 in `data.obj`. One choice here isn't confirmed by a
  fixture: the manual puts "symbol definitions" before any code in
  `. ABS .`, and govax reads that as direct assignments only. A label
  there names the location that follows it, so it moves to `. BLANK .`.
- **Declared but undefined symbols.** A symbol `.GLOBAL` names before
  defining it exists in the table without a value. The new `SymUndefined`
  flag and `symbol.defined()` replace the "no pending fixups" tests that
  used to mean defined. One console quirk is kept, but only in the
  console dialect: where forward references aren't allowed, a symbol
  still waiting on its definition reads as its placeholder value. In the
  MACRO dialect it's `VAX_UNDEFSYM`.
- **Checked against real MACRO.** `TestFixtureLadderDeclarations`
  assembles each `testdata/mar` fixture and compares it with the real
  object's MHD and GSD. It checks the module name and version, each
  psect's index, attributes, alignment, and allocation, and each global
  symbol: defined or referred to, weak, psect, value, and entry mask.
  Six of the nine match exactly. The other three wait on subtask 7:
  `extern` and `hello` use `G^`, and `branch`'s CODE allocation is 49
  where MACRO's is 44, because of the displacement-size rule. They're
  skipped by name (`awaitingOperandEncoding`), and subtask 7 removes them
  from that list. `TestRelocationsMatchRealMACRO` now assembles
  `exprs.mar` as it is, `.TITLE` and `.IDENT` included.
- **Left for subtask 7:** `.DEFAULT DISPLACEMENT` and `.ENABLE ABSOLUTE`
  are recorded but don't change operands yet, and there's no `G^`.
- **Two test fixes, outside this phase.** The recent lint-hygiene commit
  (`a8b9eee`) changed two tests' `var x []T` to `make([]T, n)` where
  `make([]T, 0, n)` was meant, so each started with empty elements.
  `TestSubrecords_roundTrip` (`internal/obj`) panicked on nil subrecords,
  and `TestGetjpi_privileges` (`internal/corevms`) failed. Each is fixed in
  its own commit. `golangci-lint` still reports `dispFixup` as unused, as
  it did before this subtask. Subtask 7's displacement work is the
  likely place to use it or remove it.
- Tests: `macrodir_test.go` covers each directive and its errors, and
  `go test ./...` passes.

### 2026-09-29 — Subtask 7: operand encoding

- **The displacement-size rule.** Relative and relative deferred operands
  with no `B^`/`W^`/`L^` now follow the manual (§5.2.1, §5.2.2). A target
  already defined in the same psect gets the smallest displacement, and
  govax finishes it. Any other target gets the `.DEFAULT DISPLACEMENT`
  size, and the linker finishes it: a forward reference (even one in the
  same psect), a label in another psect, an external, or an absolute
  address, whose distance depends on where the psect goes. The console
  dialect keeps its own sizes: the smallest size for any known address,
  and a longword for a forward reference.
- **Displacement mode.** `value(Rn)` with an unknown value (relocatable,
  external, or defined later) gets a word in the MACRO dialect (§5.1.6),
  and a longword in the console dialect, as before.
- **One path for every displacement operand.** Bare operands, `B^`/`W^`/
  `L^`, and `.ENABLE ABSOLUTE` all go through `displacementOperand`,
  which decides the size before anything is stored. So the old trick of
  queueing a longword fixup and then changing it through `lastFixup` is
  gone from these paths (the `#` literal path still uses it).
- **Operand displacements aren't branches.** `fixDispB/W/L` had no users.
  They're now an operand's PC-relative displacement, measured from the
  end of the field like a branch's, which fixed `dispFixup`'s
  "unused" lint report. The two kinds differ in one way. A branch to a
  label defined later in its own psect is finished by govax, and so is
  one by real MACRO (`BRB 20$` in `branch.obj`). An operand's
  displacement to such a label is left to the linker (`STO_LD`), as real
  MACRO leaves `MOVAB HERE,R1`. A branch or displacement from a
  relocatable psect to an absolute address is left to the linker too.
- **`G^`, general mode** (§5.2.5). In the MACRO dialect, a relocatable or
  external address takes five zero bytes, starting at the mode byte,
  and a new `fixPICR` relocation (`STO_PICR`), which the linker writes as
  relative or absolute mode. An address already known to be absolute is
  assembled as absolute mode (`9F` and the address), which is what the
  linker would make of it. A forward reference later defined as absolute
  gets the same. No fixture shows which of these real MACRO emits for a
  known absolute address, so this is a choice. In the console dialect,
  where every address is absolute, `G^` is absolute mode. `@G^` is
  `VAX_BADMODE`.
- **`.ENABLE ABSOLUTE`** makes relative operands absolute mode (`9F`), with
  a longword address (`STO_L`, as `@#` uses). Relative deferred operands
  are unchanged, since there's no absolute deferred mode. No fixture uses
  it, so the choice of `STO_L` over `STO_PIDR` isn't confirmed.
- **Checked against real MACRO.** The new `TestFixtureLadderText` replays
  each real object's TIR records the way a linker would, without choosing
  psect bases. STORE IMMEDIATE and `CTL_AUGRB` fill and move through each
  psect, and each other store records its stack program in
  `Relocations()`'s form (a stored constant, like the `.ENTRY` mask's
  `STA_UB` + `STO_W`, counts as data). **All nine fixtures match
  exactly:** every psect's bytes, and every relocation's location, kind,
  and stack program. That includes `branch`'s `AF FD` and `CF` +
  `STO_WD`, `extern`'s `L^` and `G^` forms, and `hello`. The skip list in
  `TestFixtureLadderDeclarations` is gone, so all nine match there too.
- Tests: `encoding_test.go` covers the cases no fixture does: deferred
  and explicit-size relative operands, `.DEFAULT DISPLACEMENT,BYTE`,
  displacement mode with unknown values, absolute targets, `G^` with
  absolute, forward, and indexed addresses, `.ENABLE ABSOLUTE`, and the
  console dialect's sizes. The console golden snapshots are unchanged,
  and `go test ./...` passes.

### 2026-09-29 — Subtask 8: the object emitter

- **Records in source order.** Real MACRO writes its object as its second
  pass reads the source, so the order of its GSD and TIR records follows
  the source, not the psects. A MACRO-dialect assembly now keeps an
  output log (`outEvent`, in the new `output.go`), and the emitter
  replays it. The log records:
  - each psect switch, including the start in `.  ABS  .`;
  - `. =`;
  - each run of stored bytes;
  - each gap (`.BLKx`, or `.ALIGN` with no fill);
  - `.ENTRY`;
  - the two constants real MACRO stores through the linker's stack
    rather than as data.

  A run of stored bytes names only its psect, offset, and length.
  Emitting reads its bytes from the finished psect, and its relocations
  from the relocation list, so fixups finished after the bytes were
  stored are included.
- **`Assembler.Object(ObjectOptions)`** (new `object.go`) builds the
  module:
  - **Headers:** MHD, then LNM (`govax MACRO` unless the options name
    another; subtask 10 adds the build number), then SRC (the command
    line, when given), then TTL (`.TITLE`'s comment).
  - **Global symbols:** one GSD of every global symbol but the entry
    points, sorted by name, as real MACRO writes it before anything
    else. Every reference is flagged REL, as real MACRO's are.
  - **Psects:** each psect's PSC, then `STA_Px` + `CTL_SETRB`, at its
    first `.PSECT`. `.  ABS  .` is always defined, even in an empty
    module. Going back to a psect starts a new TIR record and sets the
    location again.
  - **Data:** STORE IMMEDIATE runs, each broken where a relocation's
    field starts. A relocation becomes its tree's stack program in
    postfix order, then its store command: `STO_B/W/L`, `STO_BD/WD/LD`
    (displacements and branches), `STO_PIDR`, or `STO_PICR`. `.MASK`'s
    entry point mask is `STA_EPM`.
  - **Gaps:** each is one `CTL_AUGRB`, so `.ALIGN LONG` then `.BLKB 5` is
    two, as in `branch.obj`.
  - **Entry points:** `.ENTRY` is `STA_UB mask`, then an EPM GSD record,
    then `STO_W` in a TIR record of its own.
  - **`.ASCID`:** the descriptor's first longword is `STA_LW` (length
    still 0) + `STO_L`. After the text, the length is stored back with
    `CTL_AUGRB -n`, `STA_UB len`, `STO_W`, `CTL_AUGRB`.
  - **The EOM record:** severity SUCCESS, or WARNING when there were
    warnings, and the transfer address only when `.END` names one.
- **Operand order.** For an operand whose value the linker finishes,
  real MACRO stacks the value, then stores the addressing mode byte,
  then stores the value (`STA_GBL`, STORE IMMEDIATE `EF`, `STO_LD`).
  Fixups that follow a mode byte are marked (`fixup.mode`, and
  `relocation.mode`), and the emitter stores them in that order.
- **Stack forms.** A constant uses the shortest form, as real MACRO's do:
  `STA_UB`, then `STA_UW`, then `STA_LW`. A psect offset uses `STA_PB`,
  `STA_PW`, or `STA_PL`; the byte and word forms sign-extend, so
  `STA_PB` reaches only 0x7F. `.` is always `STA_PL`, as in
  `hello.obj`'s `.ASCID`, so `rexpr` marks a base that came from `.`.
- **`obj.Builder`** now keeps content in the order it's added. A run of
  `AddPsect`/`AddSymbol` calls fills GSD records, and a run of
  `Emit`/`Store`/`SetLocation` calls fills TIR records. The new `Break`
  ends a record early, and `Source` adds a SRC header. Its existing
  callers add everything to the GSD before the TIR, so their output
  doesn't change.
- **Checked against real MACRO.** `TestFixtureLadderObjects` builds each
  fixture's object, with real MACRO's creation time, LNM, and SRC text.
  It encodes and decodes the object, runs `Check`, and compares its dump
  with the real object's dump, traceback records left out. **All nine
  fixtures match record for record:** the same headers, the same GSD
  and TIR records in the same order, the same subrecords and commands,
  and the same EOM.
- **Not confirmed by a fixture.** These follow the confirmed cases, and
  more fixtures would settle them:
  - The value-first order is applied to every operand with a mode byte.
    Only relative mode is confirmed; displacement, immediate, and
    absolute mode aren't.
  - The signed constant forms (`STA_SB`/`STA_SW`) are used for negative
    values.
  - Going back to a psect starts a new record.
  - `. =` is `CTL_SETRB`.
  - An absolute psect's PSC allocates 0 bytes, as the manual says.
  - The manual says MACRO leaves out global symbols defined in an
    absolute psect that no relocatable psect refers to. govax writes
    them all, since leaving out a global definition could break a link,
    and no fixture shows the behavior.
- **Not written yet:** traceback records (the later traceback
  sub-phase). *Done in Phase 29 (subtask 11): the objects match real
  MACRO's with their traceback records.*
- Tests: `object_test.go` covers the headers, the global symbol GSD,
  weak symbols, going back to a psect, `. =`, the psect offset forms,
  gaps, `.MASK`, each operand mode's order, the constant forms, long
  data (STORE IMMEDIATE runs of 128 bytes, records of 512), a warning's
  severity, absolute psects, and the console dialect's refusal.
  `go test ./...` passes.

### 2026-09-30 — Subtask 9: shared host and ODS-2 file access

- **Classifying a name** (`internal/rms/location.go`). `Session.Locate`
  applies the four rules of
  [File specifications](#file-specifications-host-files-and-ods-2-volumes)
  and returns a `FileLocation` (host or volume, and the name). Two
  details the plan left open:
  - A name with `/` or `\` is a host path even when it also looks like
    VMS syntax (`DUA0:/x`), since no VMS specification has either.
  - A one-letter prefix (`C:x.mar`, with no slash) is a Windows drive,
    so it's a host path, not a bare name.

  VMS syntax must end on a mounted device, through logical names and
  search lists; otherwise it's a `*NotMountedError`, and a directory with
  no device and no default device is an error too. A bare name goes to
  the volume only while `DefaultOnVolume` is true: `SYS$DISK` is defined
  and one of its elements is mounted. `Session.LocateRelated` is the
  object's form of rule 4: a bare name takes the source's side, and on a
  volume its device and directory, so `DUA0:[X]HELLO.MAR` with
  `OTHER.OBJ` gives `DUA0:[X]OTHER.OBJ`, and a host source's directory is
  used on the host.
- **Records on either side** (`internal/rms/recordfile.go`).
  `ReadRecordFile` and `CreateRecordFile` take a `FileLocation` and a
  `RecordKind`, which decides the host layout:
  - `TextRecords`: host lines (a CR before the LF is dropped); a new
    volume file is VAR with carriage-return carriage control, as VMS
    makes text files.
  - `VariableRecords`: the ODS-2 VAR layout on the host
    (`obj.ReadRecords`/`WriteRecords`, which `internal/rms` now imports
    instead of duplicating); a new volume file has real MACRO's `.OBJ`
    attributes, read from `mar-exchange-vms.dsk`: `RFM=VAR`, no `RAT`,
    `MRS` 0, `DEQ` 20, and `RSIZE` set to the longest record, as RMS
    does.

  A volume name must match one file, the highest version when none is
  given (`*AmbiguousError`, whose message no longer names TYPE unless
  TYPE is the caller). The location returned is the file actually read or
  created, with its version, which is what `LocateRelated` and the SRC
  header want. A read-only volume and wildcards in a new name are
  refused. A host file is written to a temporary file and then renamed,
  so a failure never leaves a partial object or replaces a good one. An
  `.OBJ` that an older `COPY/BINARY` left as an Undefined-format file
  still reads, from its raw bytes.
- **COPY round trips.** Real objects already came out with
  `COPY/BINARY` in subtask 3. The other direction didn't work: a host
  `.OBJ` became Undefined-format with `/BINARY` and Stream_LF without
  it, and a plain COPY between volumes also made it Stream_LF. Now a
  table of file types (`hostRecordTypes`, only `OBJ` so far) makes COPY
  copy those files as records, with or without `/BINARY`: host to
  volume reads the VAR layout and writes a real VAR file (a host file
  that isn't in that layout is an error); volume to host writes the VAR
  layout; volume to volume copies the blocks and the attributes.
- **A COPY bug fixed on the way.** `COPY/BINARY` between volumes made
  every copy Undefined-format, losing the source's record format. It
  now keeps the source's record attributes. `vax.help`'s COPY entry
  describes both changes.
- No `ods2` change was needed: its `rms.Writer` writes VAR records, and
  `obj.ReadRecords` handles the 0xFFFF end-of-block marker that `ods2`'s
  reader doesn't (real MACRO objects don't use it, since their `RAT`
  lacks the no-span bit).
- Tests: `location_test.go` covers every classification rule (Windows
  paths and drive letters, logical names onto mounted and unmounted
  devices, `/HOST`, and `SET DEFAULT` for bare names, including a search
  list `SYS$DISK`) and `LocateRelated`. `recordfile_test.go` covers both
  kinds on both sides, versions, attributes, errors, and the host write's
  all-or-nothing rule. `copyobj_test.go` copies real VAX objects both
  ways, with and without `/BINARY`, between volumes, and, when
  `mar-exchange-vms.dsk` is present, reads all nine objects in place on
  the real VAX volume and copies them out with a plain COPY, matching the
  subtask 3 fixtures record for record. `go test ./...` passes.

### 2026-09-30 — Subtask 10: the MACRO command

- **The console verb** (`console.dcl`, `internal/console/macro.go`):
  `MACRO source[/HOST] [/OBJECT[=object] | /NOOBJECT]`. SOURCE takes
  COPY's parameter-scoped `/HOST`. `/OBJECT` is a `$string` qualifier
  whose default is empty, so `/OBJECT` alone, like no qualifier, means
  the default name. `Console.Macro`:
  - finds the source with `rms.Session.Locate` and reads it with
    `ReadRecordFile` (subtask 9). A source with no file type gets `.MAR`,
    as on VMS (on the host, only when no file of that exact name exists;
    the extension is lowercase unless the name has an uppercase letter);
  - resolves `.INCLUDE` names with `LocateRelated` against the source
    actually read, so a bare name is found beside it, on either side;
  - assembles in the MACRO dialect, reports each warning
    (`CLI_ASMWARNING`) and each error (`CLI_ASSEMBLING`, naming its
    line), and after errors returns `CLI_ASMERRORS` without writing
    anything;
  - builds the object in memory (`Assembler.Object`, `obj.Encode`), with
    LNM `govax MACRO V<build>` (the new `console.BuildVersion`, set by
    `cmd/govax`) and SRC the command line as typed, as real MACRO
    records its own;
  - writes it with `CreateRecordFile`, which on the host writes a
    temporary file and renames it, so an existing object is replaced
    only by a complete one.
- **Object names.** The default is the source's name and place with the
  type OBJ, its case copied letter by letter from the source's type
  (`hello.mar`, `HELLO.MAR`, `Hello.Mar` give `hello.obj`, `HELLO.OBJ`,
  `Hello.Obj`; with no type, lowercase unless the name has an uppercase
  letter). On a volume the version is dropped, so each assembly makes
  the next version. `/OBJECT=` takes subtask 9's rules: a bare name goes
  beside the source, a host path or VMS specification where it says, and
  one that names only a directory (an existing host directory, or
  `DUA0:[OBJ]`) gets the default name in it, as VMS fills an output
  specification's missing fields from the input's.
- **Every error is reported.** The assembler used to stop at its first
  error. In the MACRO dialect it now goes on to the next statement,
  collecting each error, and `Assemble` returns them all as the new
  `asm.Errors` (a single error still comes back alone, so no existing
  caller or test changed). Errors found at the end, like an undefined
  local label, come last. An error or warning in an included file is
  wrapped in an `*Error` for each `.INCLUDE` that led to it
  (`includeLines`), the same nesting a failed `.INCLUDE` statement
  already had. The console dialect still stops at the first error.
- **The CLI** (`cmd/govax/grammar.go`): `govax macro <source>` with
  `--object <file>` or `--no-object`, passed to the console as a MACRO
  command with each file name quoted. Quoting keeps a host path's case
  (DCL uppercases anything unquoted) and stops its `/` from being read
  as a qualifier. `--mount DEVICE=container` (read-only, like MOUNT) and
  `--mount-write DEVICE=container` can each be repeated. They mount
  before `vax.init` runs, and a mount that fails ends govax with its
  error.
- **Two fixes to one-shot commands, found along the way:**
  - **A failed one-shot command dropped into the REPL.** `vax.init`'s
    comment says a command-line command exits the emulator when it's
    done, but on failure `Include` returned the error, `vax.init`
    printed it as `vax.init: ...`, and the prompt came up with exit
    status 0. Now the command always ends the session. `run` returns
    its failure (`Console.CommandLineErr`), and `govax` prints it and
    exits 1. This applies to `asm` and `run` too.
  - **Volumes left mounted at exit lost data.** Nothing dismounted a
    volume when a session ended, so the allocation bitmaps `ods2` caches
    never reached the container. The next session to create a file found
    the index-file bitmap inconsistent with the headers. A one-shot
    `--mount-write` MACRO hit this every time, and an interactive
    session that exited without DISMOUNT had the same problem. `run` now
    calls the new `MountTable.DismountAll` as the session ends.
    `TestRun_mountWrite` fails without it.
- **Help.** A new `MACRO` topic in `vax.help` covers the syntax, the
  file-name rules, object naming, error behavior, and the CLI form.
  MACRO is also in the topic list.
- **Checked by hand** with a govax-initialized container: host source to
  a volume object, a volume source (no type given) to its default object
  through `govax --mount-write`, then to a host object, and a later
  session listing all of them and creating more files without error.
- Tests:
  - `internal/asm/errors_test.go`: every error reported, a single error
    alone, errors and warnings inside nested includes, and the console
    dialect stopping at the first error.
  - `internal/console/macro_test.go`: case copying; default names for
    each case and with no type; headers; errors writing nothing and
    leaving the old object; warnings; `/NOOBJECT`; `/OBJECT` as a bare
    name, a path, a directory, and a missing directory; missing sources;
    volume sources with next versions and a directory-only `/OBJECT`;
    host source to a volume object and a read-only volume; `.INCLUDE`
    across host and volume; every `testdata/mar` fixture; and the verb
    through the DCL grammar.
  - `cmd/govax/macro_test.go`: the command the subcommand builds, a
    one-shot MACRO, a failed one ending `run` with its error, a
    `--mount-write` MACRO run twice on one volume, and a `--mount`
    failure.
  - `rms`: `DismountAll`.
  - Two `dcl` grammar tests now expect the MACRO verb.

  `go test ./...` passes.

### 2026-09-30 — Subtask 11 prepared: a second round of fixtures

- **What was left.** Subtask 3 already brought back real objects for all
  nine ladder fixtures, and subtasks 7 and 8 match govax's output to them
  record for record. So subtask 11's own list was already done. What
  remained was two things:
  - the choices those subtasks logged as not confirmed by a fixture;
  - checking govax's own objects with the real `ANALYZE/OBJECT`, `LINK`,
    and `RUN`, which the plan called extra evidence "when it's cheap".
    It is cheap now: govax writes objects straight onto a volume VMS
    mounts.
- **Three new fixtures**, each aimed at unconfirmed choices:
  - `modes.mar` (10): displacement (with no size, and `L^`),
    displacement deferred, immediate, absolute, and relative indexed
    operands whose values the linker finishes; and negative constants in
    linker expressions (`STA_SB`/`STA_SW`/`STA_LW`).
  - `psects.mar` (11): data before any `.PSECT`; an `ABS` psect of
    offsets with global symbols, one used nowhere; going back to a
    psect; `. = . + 8` and `. = ONE + 64`; `.SAVE_PSECT` and
    `.RESTORE_PSECT`. It links and runs, returning 1.
  - `general.mar` (12): `G^` to an address known to be absolute, to a
    relocatable one, to one defined later as absolute, and indexed to an
    external; relative mode to an absolute address; `.ENABLE ABSOLUTE`;
    and a forward reference after `.DEFAULT DISPLACEMENT, BYTE`.

  govax assembles all three, and its objects pass `Check`. The ladder
  tests skip a fixture whose real object isn't in `testdata/mar/vax/`
  yet.
- **`assemble.com`** now takes all twelve. It checks every `GV_NAME.OBJ`
  (govax's object) with `ANALYZE/OBJECT`, and links and runs `ENTRY`,
  `HELLO`, and `PSECTS` from both real MACRO's objects and govax's,
  showing `$STATUS`. It runs with `SET VERIFY` into a log, and writes
  `OBJECTS.LST` rather than `OBJECTS.DIR`, which COPY skipped as a
  directory last time.
- **The exchange volume** `testdata/disks/mar-exchange2.dsk` (label
  MARXCHG2, RD51 size, gitignored) was built entirely with govax:
  `INITIALIZE/CONTAINER/DEVICE=RD51`, `COPY .../HOST` of the sources
  and the script, then `MACRO .../HOST/OBJECT=DUA1:[000000]GV_NAME.OBJ`
  for each fixture. That exercises subtask 10's ODS-2 object output for
  real.

### 2026-09-30 — Subtask 11: real VMS accepts govax's objects

- **The VAX run.** The user attached `mar-exchange2.dsk` to simh,
  mounted it on VMS 7.3, and ran `@ASSEMBLE/OUTPUT=ASSEMBLE.LOG`.
  `ANALYZE/DISK_STRUCTURE` of the govax-built volume reported only the
  usual missing `QUOTA.SYS`.
  - **Every govax object passed.** `ANALYZE/OBJECT` reported 0 errors
    for all twelve of govax's objects (written straight onto the volume
    by govax's MACRO), as for all twelve of real MACRO's.
  - **They link and run.** `GV_ENTRY`, `GV_HELLO`, and `GV_PSECTS`
    linked with no messages and ran: `GV_HELLO` printed "Hello, world!",
    and each exited with `$STATUS` `%X00000001`, the same as the real
    objects' images.

  That was with the encoding before the fixes below, so VMS accepts
  both forms. The run's log, the new fixtures' objects, listings, and
  analyses, and the `ANALYZE/OBJECT` output and maps for govax's objects
  (`vax/govax/`) are in `testdata/mar/vax/`. The nine older fixtures
  were assembled again and match the corpus apart from their dates.
- **What the new fixtures settled.** These choices were confirmed:
  - an operand's stack program comes before its mode byte for
    displacement, displacement deferred, immediate (`#TABLE` is
    `8F` + `STO_L`), and absolute (`9F` + `STO_L`) modes;
  - `L^` displacement is `STO_L`;
  - `.ENABLE ABSOLUTE` is `9F` + `STO_L`, not `STO_PIDR`;
  - relative mode to an absolute address is `STA_LW` + `STO_LD`;
  - `.DEFAULT DISPLACEMENT, BYTE` gives a forward reference `AF` +
    `STO_BD`;
  - a label before any `.PSECT` goes in `. BLANK .`;
  - MACRO writes every global defined in an absolute psect, even one
    used nowhere.

  Six were wrong, and govax now follows real MACRO:
  1. **Displacement mode** with a value the linker finishes stores a
     signed word: `STO_SW`, not `STO_W`. There are new fixup kinds
     `fixSignedB`/`fixSignedW` for MACRO-dialect displacement fields,
     so a byte one is `STO_SB` by analogy.
  2. **Indexed operands** stack the value before the index byte too:
     `TABLE[R2]` is `STA_PB`, then `42 EF`, then `STO_LD`, and
     `G^EXTV[R1]` is `STA_GBL`, then `41`, then `STO_PICR`. A fixup's
     `mode` flag became `prefix`, the count of operand-specifier bytes
     before its field. The operand parser adds one for an index byte,
     and `@#`'s mode byte now counts too.
  3. **`G^` is always left to the linker**, even for an address
     already known to be absolute: `G^IOBASE` is `STA_LW ^X20000000` +
     `STO_PICR`, and `G^LATER`, defined later as `^X300`, is
     `STA_UW ^X300` + `STO_PICR`.
  4. **Real MACRO folds nothing in a linker expression.**
     `EXTVAL+<-4>` is `STA_GBL`, `STA_UB 4`, `OPR_NEG`, `OPR_ADD`, and
     `EXTVAL*<-300>` is `STA_UW ^X12C`, `OPR_NEG`, `OPR_MUL`. In the
     MACRO dialect a constant computed by an operation now keeps its
     shape (`exprVal.shape`), which goes into the tree when it joins
     such a value. `rexpr.resolved` now folds only a whole tree that
     turns out constant, and `simpleRelocatable` accepts constant
     subtrees. Only unary minus is confirmed; binary operations on
     constants are treated the same way, since MACRO evidently doesn't
     fold within these expressions.
  5. **`. =` is `CTL_AUGRB`** by the distance moved (`. = . + 8` is
     `CTL_AUGRB 8`, `. = ONE + 64` is `CTL_AUGRB ^X2C`), not
     `STA_Px` + `CTL_SETRB`. Going back to a psect sets the location in
     the same record; it doesn't start a new one.
  6. **Absolute psects.**
     - An absolute psect gets its PSC record (allocation 0) and no TIR
       commands at all.
     - A global label in one carries the psect's index, not 0, and no
       `REL` flag.
     - Each psect definition is a GSD record of its own. The first
       ladder never showed this, since TIR always came between two
       definitions.
     - `.  ABS  .`'s location is set at the start (`STA_PB 0` +
       `CTL_SETRB`) except when code or data before any `.PSECT`
       moves assembly into `. BLANK .` first. The emitter holds that
       command back until it sees what comes next (`flushAbsStart`,
       and `outEvent.implicit`).
- **All twelve fixtures now match real MACRO** in
  `TestFixtureLadderDeclarations`, `TestFixtureLadderText`, and
  `TestFixtureLadderObjects`. The text replay knows `STO_SB`/`STO_SW`,
  treats a constant stored by a displaced or position-independent
  store as a relocation (it isn't stored as it is), and skips
  absolute psects.
- Unit tests that pinned the old guesses now expect real MACRO's
  forms: `TestDisplacementModeUnknown`, `TestGeneralMode`,
  `TestObjectOperands`, `TestObjectPsectReentry`, `TestObjectLocation`
  (now also a backward `. =`), and `TestObjectAbsolutePsect`. The console
  golden snapshots are unchanged, and `go test ./...` passes.
- Nothing here is a behavior difference: each was a choice of object
  encoding, and VMS linked the old forms to the same effect. So nothing
  goes in `DEVIATIONS.md`.

### 2026-09-30 — Subtask 12: phase complete

- **Docs.**
  - `PLAN.md`: rows for Phases 28-30, and a summary of this phase.
  - `CLAUDE.md`: `internal/asm`'s two dialects, `testdata/mar/` and
    `testdata/disks/`, and the phase-doc range. It already described
    `internal/obj`, `internal/rms`'s file-name and record API, and the CLI.
  - `README.md`: the `MACRO` command in the current status, and the next
    steps.
- **`DEVIATIONS.md`** has a Phase 27 section with three entries:
  - the missing macro facility, listings, and traceback/debugger records;
  - smaller simplifications and govax extensions: `TRUNCATION`/`VECTOR`,
    the command's qualifiers, error reporting, `.INCLUDE`, the header
    texts, and the host `.OBJ` layout;
  - the encoding choices no fixture has confirmed: `STO_SB`, binary
    constant operations left unfolded, signed constant forms, a backward
    `. =`, and the start of `.  ABS  .` in cases not covered.

  None of them is a behavior difference: VMS links each form to the
  same image.
- **The later sub-phases** are now planned phases, each with its own
  document, scope, references, and open questions:
  - `PHASE-28.md`: the macro facility. Its main question is whether to
    read the real `STARLET.MLB`, which needs a reader for the
    librarian's file format, since the system-service macros aren't in
    the source archive.
  - `PHASE-29.md`: listings, traceback, and debugger records. Real
    MACRO's `.lis` files and full objects are already in
    `testdata/mar/vax/`.
  - `PHASE-30.md`: a govax `LINK`, built on `internal/obj`, the
    `internal/rms` file access, and Phase 13's image activation.
- `go test ./...` passes.
