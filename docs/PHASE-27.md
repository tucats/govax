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

**Status: subtask 1 done; subtask 2 next.**

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

The real-VAX fixtures (ladder steps 4, 5, and 7) confirm this. If they show
MACRO choosing sizes this rule doesn't predict, the decision is reopened
here.

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
them probably lives in `internal/rms` too, since only it can parse an ODS-2
spec. Subtask 9 decides exactly where it goes.

**How the records are stored.** On VMS, a `.OBJ` file is an RMS
variable-length record (`RFM=VAR`) file, and the record boundaries are part
of the format.

- **On an ODS-2 volume**, the object is a real `RFM=VAR` file, written
  through `ods2`'s `rms.Writer`. Phase 22's `SYS$PUT` already writes VAR
  records through it, so no new `ods2` support is expected. The exact file
  attributes (the `RAT` flags and the maximum record size) are copied from a
  real VAX `.OBJ` once one is available.
- **On the host**, there are no record boundaries, so the file uses ODS-2's
  own on-disk VAR layout: for each record, a 2-byte little-endian length,
  then the data, padded to an even length. That's the same bytes a raw
  copy of the file's blocks would have. Subtask 9 must confirm the layout
  survives a COPY in both directions:
  - a real `.OBJ` copied from a container to the host (COPY `/BINARY`, if
    that copies blocks as they are) must be readable by `internal/obj`;
  - a host `.OBJ` copied into a container must come back as a real VAR
    file.

  If COPY can't do either of those today, the fix is in scope, in govax's
  COPY or in `ods2` itself.

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
  `.SUBTITLE`/`.SBTTL` (accepted, and ignored until listings exist)
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
  `.BLKx`, `.ASCII*`, `.ALIGN`, the floats, `.MASK`), which now allow
  relocatable operands
- `.END [transfer]`, which sets the EOM transfer address

**Later sub-phases.** Each gets its own subtask entry when it starts:

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
2. **`internal/obj`: record model, writer, reader, dumper, and checker**,
   with unit tests on hand-built modules. There's no assembler involvement
   yet.
3. **First real-VAX fixtures** (needs the user). Fixtures 1 to 3 from the
   ladder, assembled on the VAX. Decode them with the subtask 2 reader and
   record what we learn here: the MHD field values real MACRO uses, the LNM
   text, the psect attributes it writes for defaults, how it splits records,
   and its TIR idioms. Fix the reader wherever real objects show it's wrong.
4. **Refactor `internal/asm` for a shared core** (no behavior change).
   Introduce the dialect setting and per-directive dialect flags, and the
   section model with absolute sections for the console dialect. Move
   eVAX-only directives behind the console dialect. Pass criterion: every
   existing `internal/asm` and `internal/console` test passes, and every
   `testdata/asm` fixture assembles to the same image and symbols as
   before.
5. **Relocatable values.** `exprVal`, `fixup`, and `symbol` carry psect
   and external terms. Unresolved relocatable and external references become
   relocation records in MACRO dialect. Expression-rule tests (absolute vs.
   relocatable vs. complex).
6. **MACRO-dialect directives:** the first-milestone list above.
7. **Operand encoding for relocatable and external operands.** Implement
   the manual's displacement-size rule: the smallest size for a target
   already defined in the same psect, otherwise the `.DEFAULT DISPLACEMENT`
   size. Add `.DEFAULT` and `G^` general mode (`STO_PICR`/`STO_PIDR`).
   Check against fixtures 4, 5, and 7. If they disagree with the rule,
   reopen [Pass structure](#pass-structure).
8. **Object emitter.** Turn sections, symbols, and relocations into
   `internal/obj` records: PSC and SYM/EPM GSD subrecords, TIR (STORE
   IMMEDIATE runs, `CTL_SETRB`, relocation stack programs), and EOM with the
   severity and transfer address.
9. **Shared host/ODS-2 file access.** The helper that classifies a file
   name (see
   [File specifications](#file-specifications-host-files-and-ods-2-volumes)),
   the `internal/rms` record API for reading and creating record files on a
   mounted volume, and the host VAR layout reader and writer. Confirm that
   `.OBJ` files survive COPY both ways, and fix COPY or `ods2` if they
   don't. Unit tests cover each classification rule, including Windows drive
   letters, logical names, `/HOST`, and the `SET DEFAULT` rule for bare
   names.
10. **Command surface.** The console `MACRO` verb with the source's `/HOST`
    and `/[NO]OBJECT[=file]`, the `govax macro` subcommand, a repeatable
    `--mount DEVICE=container` CLI option that mounts volumes before a
    one-shot command runs, default object naming with case copied from the
    source's extension, `.INCLUDE` across host and ODS-2, and writing only
    on success. Update the help text in `vax.help`.
11. **Fixture ladder 4 to 9** (needs the user). Compare each fixture with the
    VAX's output and add each real object to the reader corpus. Log the
    differences.
12. **Clean-up and docs.** Update `PLAN.md`, `DEVIATIONS.md`, and
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
