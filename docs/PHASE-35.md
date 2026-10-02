# Phase 35 — The rest of the instruction set: G, H, octaword, and packed decimal

**Status:** done (2026-10-02), but for one check: VAX MACRO's run of the
assembler fixture (`testdata/insn35/asm`), which settles a few encodings
govax's assembler chose without VAX MACRO's output (see subtask 16's
log entry). The author accepted the plan and every proposed answer under
"Decisions".

## Goal

govax's CPU implements every remaining VAX instruction that works on a
quadword, octaword, floating, or decimal data type:

- **G_floating** (the `FD`-prefixed 40–56 group, plus CVTGF and CVTFG);
- **H_floating** and the **octaword** moves (the `FD`-prefixed 60–7F group,
  plus the conversions between H and F, D, and G);
- the **F_floating and D_floating** instructions the earlier phases left
  out: EMODF/EMODD, POLYF/POLYD, CVTFD, and CVTDF;
- **packed decimal**: the arithmetic, compare, move, shift, and conversion
  instructions, and EDITPC.

Every one is written from DIGITAL's *VAX Architecture Reference Manual*
(`vax_instr_set.pdf`), since the C reference has none of them, and checked
against VMS 7.3 by an oracle program (see "Method").

The author also directed (2026-10-02) that **bugs and missing capabilities
in existing code that these instructions depend on are in scope**: the
instruction table, operand decoding, the floating-point core (including
D_floating's lost precision), the assembler, and the disassembler. Those are
listed under "Gaps in existing code" and have their own subtasks.

## Inventory

Taken from the live table (`Table.Implemented`) on 2026-10-02: 284 entries,
59 without a handler. Another 30 architected opcodes aren't in the table at
all, because `internal/cpu/gen` generates it from the C reference's
`instruction_table.h`, which never listed them. Every unimplemented entry
also has **no operand metadata** (count 0, scales 0), so the decoder,
assembler, and disassembler can't handle them even before a handler exists.

### In scope

| Family | Opcodes | In table, no handler | Not in table |
| --- | --- | --- | --- |
| F/D leftovers | EMODF 54, POLYF 55, CVTFD 56, EMODD 74, POLYD 75, CVTDF 76 | all 6 | — |
| G_floating | ADDG2/3, SUBG2/3, MULG2/3, DIVG2/3 (FD40–47); CVTGB/GW/GL/RGL (FD48–4B); CVTBG/WG/LG (FD4C–4E); ACBG FD4F; MOVG FD50; CMPG FD51; MNEGG FD52; TSTG FD53; EMODG FD54; POLYG FD55; CVTGH FD56; CVTGF FD33; CVTFG FD99 | 23 | CVTGF, CVTFG |
| H_floating | ADDH2/3, SUBH2/3, MULH2/3, DIVH2/3 (FD60–67); CVTHB/HW/HL/RHL (FD68–6B); CVTBH/WH/LH (FD6C–6E); ACBH FD6F; MOVH FD70; CMPH FD71; MNEGH FD72; TSTH FD73; EMODH FD74; POLYH FD75; CVTHG FD76; CVTFH FD98; CVTHF FDF6; CVTDH FD32; CVTHD FDF7 | CVTDH, CVTFH, CVTHF (and CVTGH above) | the other 24 |
| Octaword | CLRO FD7C, MOVO FD7D, MOVAO FD7E, PUSHAO FD7F | — | all 4 |
| Packed decimal | CVTPS 08, CVTSP 09, ADDP4 20, ADDP6 21, SUBP4 22, SUBP6 23, CVTPT 24, MULP 25, CVTTP 26, DIVP 27, MOVP 34, CMPP3 35, CVTPL 36, CMPP4 37, EDITPC 38, ASHP F8, CVTLP F9 | all 17 | — |

That is 49 handlers for entries already in the table and 30 for new ones:
79 instructions in all. (The first count, on 2026-10-02, said 52, 29, and
81; subtask 1's generator check corrected it.)

The quadword *integer* instructions (ASHQ, EMUL, EDIV, CLRQ, MOVQ, MOVAQ,
PUSHAQ) are already implemented. The assembler's spellings that share an
opcode (CLRG/CLRH, MOVAG/MOVAH, PUSHAG/PUSHAH, MOVAD, PUSHAD) are aliases,
not table entries; see subtask 1.

### Not in scope (see Decisions)

- **LDPCTX/SVPCTX** (06, 07): unimplemented, but they're process-context
  instructions, not data-type ones (Decision 1).
- **Reserved opcodes** (57, 59, 5A, 5B, 77, FE, FF and the `FD`-page gaps):
  they correctly raise the reserved-instruction fault.
- **Vector instructions** (the VAX vector extension's `FD`-page opcodes).
  They stay reserved-instruction faults.

## Gaps in existing code

Each of these was found while surveying for this plan, and each blocks one
or more of the families above.

1. **Table metadata.** The generator reads only the C header, so the 59
   unimplemented entries have no operands, and 30 opcodes are missing.
   Re-running `go generate` would lose any hand edit to
   `instructions_table.go`.
2. **One literal type per instruction.** `Instruction.Type` is just
   `ShortLiteralInt` or `ShortLiteralFloat`, and floating operands are
   told apart only by size (`fpuLoad(raw, size)`). D_floating and
   G_floating are both 8 bytes, so a G immediate would be decoded as D.
   Conversions mix formats in one instruction (CVTFG reads F and writes G;
   CVTPL reads packed and writes a longword), so the type has to be per
   operand.
3. **Operands wider than 8 bytes.** `Operand.Value` is a `uint64`;
   `loadSized` and `storeValue` panic on any size but 1, 2, 4 (and 8);
   `decodeImmediate` reads at most 8 bytes; register mode for an octaword
   must span Rn through Rn+3. Indexed mode already scales by the operand
   size (`operand.go:367`), so `[Rx]` on an octaword should just need a
   test.
4. **D_floating loses precision.** `fpu.go` converts every float to a Go
   `float64`, whose 52-bit fraction can't hold D_floating's 55 bits: the
   low 3 bits are always stored as zero (PHASE-05.md's design notes). So
   even MOVD of an arbitrary D value can change it, and D arithmetic is
   rounded to the wrong precision. `float64` also rounds ties to even,
   while VAX rounding is half away from zero, which affects F, D, and G
   results. H_floating's 113-bit fraction can't use `float64` at all.
5. **The assembler** has no `.G_FLOATING`, `.H_FLOATING`, `.OCTA`, or
   `.PACKED` directives (`.BLKG`/`.BLKH`/`.BLKO` exist), encodes float
   immediates by size only (`cpu.EncodeFloat(scale, f)`), has no octaword
   immediates (DEVIATIONS.md's Phase 28 quadword-immediate entry says
   they'd "come with G/H floating, a later phase"), and lacks the aliases
   CLRG, CLRH, MOVAD, MOVAG, MOVAH, PUSHAD, PUSHAG, and PUSHAH.
6. **The disassembler** decodes float literals and immediates by size
   (`cpu.DecodeFloat(bits, size)`), so it would show a G or H operand as D
   or fail.

## Design

### Instruction table

The generator keeps the C header as its base, and gains a Go-side
supplement (`internal/cpu/gen/operands.go`, as built in subtask 1)
holding, from the manual, every instruction's operands in the manual's
notation (`rl,rl,wl`). Blank header entries are filled from it, the
entries the header lacks are added, and an entry that disagrees stops the
generator unless it's listed, with its reason, as a correction, so the two
can't drift silently. The generator stays the only way the table changes.

Each operand gets a **data type** alongside its access and size: byte,
word, longword, quadword, octaword, F, D, G, or H, the manual's letter.
(A decimal or character string is a length operand and an address
operand, `rw,ab`; the decoder doesn't need to know what the bytes at the
address are, so strings get no data types of their own.) The
short-literal rules then follow the operand's own type: an integer
literal for integers, and the format's own expansion for F, D, G, or H
(each format places the literal's 3 exponent and 3 fraction bits
differently, though the value is the same). `Instruction.Type` stays as a
derived convenience for current callers, or is retired if nothing still
needs it.

### Operands up to an octaword

`Operand` grows to carry 128 bits (for example, `Value` plus a `Hi`
word, or a `[2]uint64`; settled in subtask 2 by what reads best at the
call sites). `Load`/`Store` and the register, memory, immediate, and
autoincrement/autodecrement paths handle 16 bytes, and register mode uses
four consecutive registers, as quadwords use two. The manual leaves an
octaword in R12 or above UNPREDICTABLE; govax will raise the
reserved-addressing-mode fault, recorded in `DEVIATIONS.md` as a choice.

### Floating point: one exact core for all four formats

A new floating core (proposed: `internal/vaxfloat`, a leaf package with no
CPU dependency) holds an **unpacked** value: sign, unbiased exponent, and a
fraction wide enough for H_floating's 113 bits plus guard bits. It has:

- `Unpack(format, bits)` and `Pack(format, value)` for F, D, G, and H,
  with each format's word-swapped layout, bias, and reserved-operand
  encoding (sign 1, exponent 0);
- add, subtract, multiply, and divide, each **rounded once** to the
  destination format's precision, half away from zero, as the
  architecture defines;
- the exact operations EMOD and POLY need (an extended-precision product,
  and integer/fraction separation);
- overflow and underflow reported to the caller, which turns them into the
  same faults `fpu.go` raises now (`faultFltOvf`, `faultFltUnd` under
  PSL<FU>), and conversions to integers with the existing overflow bounds.

F and D move onto this core too (subtask 5), which fixes gap 4: MOVD
becomes exact, and D arithmetic rounds at 56 bits. Whether the core is
built on `math/big.Float` (simplest; it supports the rounding mode and any
precision) or on hand-written 128-bit arithmetic is Decision 2.

`loadFloat`/`storeFloat` take the operand's format instead of inferring it
from size, and still hand callers a value of the core's type rather than a
`float64`. `EncodeFloat`/`DecodeFloat` (used by `internal/asm`) take a
format too, and keep a `float64` convenience form for the assembler's
literal parsing, plus an exact path for H.

### Packed decimal

A packed decimal core (proposed: `internal/cpu/decimal.go` with a pure
helper type) reads a string given its length (0–31 digits) and address,
validates it (a bad digit or sign nibble is a reserved-operand fault,
unless the manual allows it for that instruction), and holds the value as
a sign and up to 31 digits, wide enough for MULP's and DIVP's
intermediates. It writes results with the preferred sign codes (C for
plus, D for minus) and the manual's rules for the unused high nibble.

Each instruction then follows the manual's definition, including:

- **condition codes**, and the registers it leaves (R0–R3, or R0–R5),
  which programs and VMS's own code read after the instruction;
- **decimal overflow**: V set, and a decimal-overflow trap (arithmetic
  trap code 6) when PSL<DV> is set; **divide by zero** in DIVP: the
  "floating or decimal divide by zero" trap (code 4);
- **reserved operands**: a length over 31, or (for EDITPC) a bad pattern
  operator.

EDITPC runs its pattern operators (the `EO$` table the assembler already
knows, `internal/asm/builtins.go`) over the source, with the fill, sign,
and significance state the manual defines. CVTPS/CVTSP (leading separate
numeric) and CVTPT/CVTTP (trailing numeric, with a translation table) get
their own subtask.

**Faults part way through.** On a real VAX these long instructions can
take a memory fault part way and resume with PSL<FPD> set. govax doesn't
model FPD for the character-string instructions. Subtask 10 checks what
MOVC3/MOVC5 do on a fault part way, and the decimal instructions follow
the same approach; anything that differs from the architecture goes in
`DEVIATIONS.md`.

### Assembler and disassembler

- Directives `.G_FLOATING`, `.H_FLOATING`, `.OCTA`, and
  `.PACKED string[,symbol]` (MACRO-32's form: it stores the packed
  string and can define the symbol as its digit count).
- Immediates of each float format, octaword immediates, and
  short-literal float encoding by the operand's format (`FindShortFloat`
  becomes format-aware).
- The aliases from gap 5.
- The disassembler shows G and H literals and immediates in the right
  format and octaword immediates in full.

Real VAX MACRO's output wins where it disagrees with eVAX (as in earlier
phases): each new directive gets a fixture checked against an object
assembled on VMS.

## Method: the manual, then the oracle

Each instruction is written from the *VAX Architecture Reference Manual*'s
definition, and unit-tested with values worked out by hand from it
(including the manual's own EDITPC and POLY examples).

Then, as in Phases 33 and 34, an **oracle** checks govax against VMS 7.3:

- `testdata/insn35/` holds a MACRO-32 probe program (`probe.mar`), a
  command procedure to assemble, link, and run it on VMS, and a README.
  For each instruction, the probe runs a table of operand cases and
  writes, per case: the inputs, the result bytes, the PSL condition codes,
  the registers the instruction leaves (R0–R5), and, if it faulted or
  trapped, the signal array (caught by a condition handler that records it
  and continues). Each family is a separate section, so a run can cover
  just one.
- A govax test (`TestInsn35Oracle`) runs the same image, built by govax's
  own MACRO and LINK, and compares every case with VMS's output byte for
  byte. Differences are fixed, or recorded in `DEVIATIONS.md` with a
  reason.
- The author's VMS 7.3 system (simh's MicroVAX 3900) does H_floating and
  packed decimal in VMS's instruction emulator rather than in hardware.
  That emulator implements the architected results, so it's still the
  reference; the README notes it in case a difference shows up.

The probe can be assembled on VMS, so its run doesn't wait for govax's
assembler work. govax running it does.

## Subtasks

Each is independently testable and ends with `go build`, `go vet`,
`go test`, and golangci-lint clean, a commit, and `build -i` when it
changes behavior. Each adds to this doc's progress log.

1. **Instruction table.** The generator's supplement (operands for the 49
   blank entries and the 30 new ones, per-operand data types), the
   regenerated table, and the assembler aliases. A test checks every
   in-scope opcode's operand list against a table transcribed from the
   manual, and that the assembler and disassembler round-trip each new
   opcode with register operands. No handlers yet: SHOW INSTRUCTIONS
   still lists them as unimplemented.
2. **Octaword operands, and the octaword moves.** 128-bit `Operand`, the
   register/memory/immediate/autoincrement/indexed paths, and octaword
   immediates in the assembler (`.OCTA` too). Handlers: CLRO, MOVO, MOVAO,
   PUSHAO. Tests for each addressing mode, including Rn..Rn+3, `(Rn)+`
   advancing by 16, and the R12+ reserved-addressing fault.
3. **The oracle probe** (`testdata/insn35/`): the program, the command
   procedure, and the README, with sections for F/D, G, H, octaword, and
   packed decimal. The author runs it on VMS while subtasks 4–13 go
   ahead; its output is checked in as the reference.
4. **The floating core** (`internal/vaxfloat`): unpack/pack for F, D, G,
   and H, the four operations with correct rounding, overflow and
   underflow reporting, and integer conversion. Tests: the manual's
   example bit patterns, reserved operands, the largest and smallest
   values of each format, rounding ties, and round trips. No CPU change
   yet.
5. **F and D on the core.** `loadFloat`/`storeFloat` and the immediate and
   short-literal paths go format-aware; every existing F/D handler moves
   to the core; `EncodeFloat`/`DecodeFloat` take a format. Adds CVTFD and
   CVTDF. Fixes D_floating's lost bits and the tie rounding, with tests
   that would have failed before (MOVD of a value with the low 3 fraction
   bits set; a tie in ADDF). Existing tests keep passing, or are corrected
   where they encoded the old imprecision (each such change noted in the
   log).
6. **G_floating.** Every G instruction except EMODG and POLYG, plus
   CVTGF and CVTFG; `.G_FLOATING`, G immediates and literals in the
   assembler and disassembler. Tests per instruction, condition codes,
   and faults.
7. **H_floating.** Every H instruction except EMODH and POLYH, and the
   conversions to and from F, D, and G; `.H_FLOATING`, H immediates and
   literals. Tests as for G.
8. **EMOD** (F, D, G, H): the extended multiplier (a byte for F/D, a word
   for G/H), the integer part (with V on overflow) and fraction part.
9. **POLY** (F, D, G, H): the manual's evaluation order and rounding, the
   R0–R5 results, the degree limit of 31 (reserved operand), and the
   manual's examples.
10. **Packed decimal core, and the simple instructions.** The core (read,
    validate, write); MOVP, CMPP3, CMPP4, CVTLP, CVTPL; `.PACKED` in the
    assembler. Settle the part-way-fault approach (see the design note).
11. **Packed decimal arithmetic.** ADDP4, ADDP6, SUBP4, SUBP6, MULP, DIVP,
    ASHP (with its rounding digit), decimal overflow under PSL<DV>, and
    DIVP's divide-by-zero trap.
12. **Numeric string conversions.** CVTPS, CVTSP, CVTPT, CVTTP, including
    the trailing-numeric translation tables and the overpunched signs.
13. **EDITPC.** Every pattern operator, the significance and sign state,
    and the reserved-operand cases; the manual's worked examples as
    tests.
14. **Reconcile with the oracle.** `TestInsn35Oracle` runs the probe
    under govax and compares with VMS's output. Each difference is fixed,
    or masked with the reason recorded and a `DEVIATIONS.md` entry.
15. **Clean-up of the old "missing" references.** Now that the
    instructions exist, find and update every place that says they don't:
    - `DEVIATIONS.md`: the Phase 28 quadword-immediate entry's "Octawords
      would extend the same way, though no implemented instruction has
      one yet (they come with G/H floating, a later phase)"; the Phase 05
      entries about D_floating's precision and its unimplemented opcodes;
      any other entry this phase resolves (marked resolved, with the
      subtask that resolved it, rather than deleted, as the log's other
      resolved entries are);
    - earlier phase docs: PHASE-05.md (its sub-phase 9 close-out lists
      EMODF/POLYF/CVTFD and the D counterparts as remaining on
      `unimplementedHandler`; its design notes on D_floating's 3 lost
      bits), and any others a search turns up (a "now implemented in
      Phase 35" note, not a rewrite of history);
    - code comments: `internal/cpu/fpu.go`'s F/D-only description,
      `internal/cpu/gen`'s note that the table is the C header alone,
      `internal/asm/operand.go`'s quadword/octaword immediate comments,
      `internal/asm/pseudo.go`'s `.F_FLOAT`/`.D_FLOAT`-only description,
      and any other comment a search for the instruction names and for
      "unimplemented", "G_float", "H_float", "octaword", and "packed"
      turns up;
    - a test that every opcode in the table except LDPCTX/SVPCTX (unless
      Decision 1 changes) and the reserved ones has a handler, so SHOW
      INSTRUCTIONS `/UNIMPLEMENTED` lists only those.
16. **Close-out.** This doc's status, PLAN.md's index, CLAUDE.md's
    package notes (`internal/vaxfloat`, and `internal/cpu`'s phases), and
    HELP if any console output changed.

## Decisions

The author accepted each proposal below on 2026-10-02.

1. **LDPCTX and SVPCTX.** They're the only other unimplemented
   non-reserved instructions, but they load and save a process's hardware
   context from a PCB, and govax runs processes through its RTL rather
   than a VMS scheduler, so nothing reaches them today. Decided: out of
   scope, left for a phase that needs them; Phase 07's and Phase 21's
   notes about them stay as they are.
2. **How the floating core does its arithmetic.** `math/big.Float`
   supports each format's exact precision and half-away-from-zero
   rounding (`big.ToNearestAway`), so it's the shortest path to correct
   results, at the cost of allocation on every floating instruction.
   Hand-written 128-bit arithmetic is faster but more code to get right.
   Decided: `big.Float` behind the core's own API, with a benchmark
   in subtask 4; F and D get a hand-written fast path later only if a
   real program shows the cost.
3. **Changing existing F/D results.** Fixing gap 4 changes the low bits of
   some D results and some F/D tie roundings that existing tests and the
   C reference agree on. Decided: fix them, as the author's direction
   on existing bugs says, with a `DEVIATIONS.md` entry for each changed
   behavior and the oracle to confirm the new values.
4. **Octawords in R12 and above.** Decided: a reserved
   addressing-mode fault (the manual says UNPREDICTABLE), checked against
   VMS by the probe if a case can be written safely.

## Out of scope

- LDPCTX/SVPCTX (Decision 1), vector instructions, and reserved opcodes.
- Modelling PSL<FPD> for interrupted long instructions, beyond matching
  what the character-string instructions already do.
- The console's EXAMINE/DEPOSIT showing G, H, or packed data in their own
  formats; a follow-on if wanted.
- Run-time library routines for these types (OTS$, the LIB$ packed
  conversions); a later `internal/librtl` addition.

## Progress log

- 2026-10-02: Plan written, after listing the live instruction table (59
  entries without a handler, 29 architected opcodes absent) and surveying
  the decoder, operand access, the F/D floating code, and the assembler
  and disassembler. The author directed that bugs and missing
  capabilities in existing code these instructions depend on are in
  scope.
- 2026-10-02: The author accepted the plan and each proposed decision.
  Subtasks proceed, with a pause for review after subtask 1.
- 2026-10-02: Subtask 1 done: the instruction table.
  - **The manual as a second source.** `internal/cpu/gen/operands.go`
    gives every instruction's operands in the manual's notation, and
    `gen/manual.go` applies it: it fills the 49 blank entries, adds the 30
    missing ones (`missingInstructions`; the table now has 314 entries),
    and stops the generator on any row that disagrees with the manual
    unless `manualCorrections` lists it with a reason. Each entry's
    short-literal `Type` is now derived from its operands (floating if it
    reads a floating operand), and checked against the header's.
  - **`DataType`** (`internal/cpu/datatype.go`): each `Instruction` has a
    `DataType [6]DataType` column (byte through octaword, F, D, G, H).
    Nothing reads it yet; subtask 5 makes operand loads and short literals
    use it.
  - **Existing rows the check found wrong** (fixed, `DEVIATIONS.md`):
    CVTWL/CVTWB/CVTBL/CVTBW/CVTLB/CVTLW wrote through a modify operand;
    BISW3 had a longword destination; PROBER/PROBEW/INSQUE/REMQUE/CALLG/
    CALLS sized address operands as longwords (wrong indexed-mode scaling);
    ACBF's literals were integers, so `ACBF #10.0,#1.0,...` looped wrongly
    (`TestEmulAcbFloatShortLiterals`).
  - **A decoder panic fixed in passing**: an `I^#` immediate of 16 bytes
    panicked in `loadSized`. It's reachable now that H and octaword
    instructions decode; the decoder now reads and steps over it, and
    subtask 2 keeps its value.
  - **Assembler aliases**: MOVAD, PUSHAD, CLRG, MOVAG, PUSHAG, CLRH,
    MOVAH, PUSHAH (`internal/asm/opcode.go`).
  - **Tests**: `TestInstructionTableMatchesManual` (a separate
    transcription of each Phase 35 instruction and each corrected row),
    `TestInstructionTableConsistent` (Scale, DataType, and Type agree on
    every entry), `TestNewInstructionsDecodeAsReserved` (each new
    instruction decodes with immediate operands and raises the
    reserved-instruction fault until its handler exists),
    `TestRoundTripEveryInstruction` (every instruction disassembles and
    reassembles to the same bytes), `TestFloatingAliases`.
  - The table's `go:generate` directive stays disabled, as it has been
    since `7fbf5c7`; regenerate with `go run ./gen -in
    ../../reference/eVAX/eVAX/Headers/instruction_table.h -out
    instructions_table.go` from `internal/cpu`.
- 2026-10-02: The author reviewed subtask 1 and asked for subtask 2.
- 2026-10-02: Subtask 2 done: octaword operands and the octaword moves.
  - **`Octaword`** (`internal/cpu/octaword.go`): a 128-bit value as two
    `uint64` halves, with `Operand.LoadOctaword`/`StoreOctaword` for
    registers (Rn..Rn+3), memory, and immediates. `Operand` gains `High`,
    the top 64 bits of a 16-byte immediate. `Load`/`Store` stay 64-bit:
    on a 16-byte operand `Load` returns the low half and `Store`
    zero-extends (which is all CLRO needs). A memory store checks both
    ends are writable before writing either half, so a fault leaves the
    destination unchanged.
  - **Decision 4**: an octaword register operand past R11 is a reserved
    addressing-mode fault at decode (`DEVIATIONS.md`).
  - **Handlers**: CLRO (the shared CLR handler), MOVO (its own, since
    `emulMove` carries 64 bits), MOVAO and PUSHAO (the shared MOVA/PUSHA
    handlers; only decode cares about the size).
  - **Assembler**: integer literals are 128 bits wide (`internal/asm/
    wide.go`), so octaword immediates keep every bit, and a value of
    2^64 or more is never taken for a short literal. New `.OCTA`
    directive (both dialects; HELP updated). The disassembler shows
    quadword and octaword immediates in full; it used to drop a
    quadword's high longword (`DEVIATIONS.md`).
  - **Tests**: `internal/cpu/octaword_test.go` (register, immediate,
    short literal, `(Rn)+`/`-(Rn)`, indexed scaling by 16, condition
    codes, the R12 limit, CLRO, MOVAO/PUSHAO, and a store across an
    invalid page leaving memory unchanged); `internal/asm/octaword_test.go`
    (`.OCTA`, MOVO immediates, wide disassembly round trips).

- 2026-10-02: Subtask 3 done: the oracle probe (`testdata/insn35/`).
  - **`gen.go`** writes five probes, one per family (`p35fd.mar`,
    `p35g.mar`, `p35h.mar`, `p35o.mar`, `p35p.mar`: 146, 113, 126, 28,
    and 154 cases), `insn35.com`, and `exchange.cmd`. Each case is one
    instruction in its own routine under a condition handler, with known
    registers, DST filled with `^XAA`, and N, Z, V, C preset to 1; it
    records the PSL, R0-R11, DST, and any signal array. Traps continue
    (so results are recorded); faults unwind. Floating operands are
    written as bytes by `gen.go`'s own encoder (`math/big`, rounded half
    away from zero), independent of the floating core to come.
  - **The VMS run** is the author's: see the README. Its container goes
    in `testdata/insn35/vax/` for subtask 14.
  - **govax runs every probe to the end** (`TestInsn35ProbesRun`, a
    smoke test subtask 14 turns into the comparison). Doing so found:
    - MOVA and PUSHA (so MOVAO and PUSHAO) never set the condition codes;
      the manual sets N and Z from the address and clears V. Fixed
      (`DEVIATIONS.md`).
    - DIVF/DIVD by zero signal floating overflow instead of floating
      divide by zero, and the floating-to-integer conversions fault on
      every overflow, ignoring PSL<IV>, where the manual sets V and traps
      only under IV (`DEVIATIONS.md`; subtask 5, and the decimal traps in
      subtasks 10-11, which need a general arithmetic-trap path). A
      handler that continued looped forever, so the probe's handler now
      unwinds a case that signals twice.
  - **The microkernel tests use is bootdata's.** The tests' helpers
    assembled `testdata/asm/kernel.asm`, an old copy of the microkernel
    without the arithmetic and translation-not-valid SCB vectors, so an
    arithmetic exception in a test image stopped the run ("exception
    vector is zero") where govax itself would have signalled it. At the
    author's direction the copy is gone: every test assembles
    `internal/bootdata/files/kernel.asm` (`kernelPath` in
    `internal/console`, `kernelDir` in `internal/asm`), and the golden
    snapshots were regenerated. `signals.asm` and `unwind.asm` declared
    LIB$ shims the old kernel lacked; each is now skipped when the kernel
    has defined it.
- 2026-10-02: The author's first VMS run of the probe stopped at MACRO:
  every probe's SETUP used `#<4*(1+1+12+9)>`, and VAX MACRO, which groups
  expressions only with angle brackets, rejected it (`%MACRO-E-ILLEXPR`).
  govax's MACRO dialect had taken the parentheses as grouping; it now
  reports VAX_ILLEXPR as VMS does (`TestMACRODialectParenthesisIsIllegalExpression`;
  the console dialect still groups with parentheses, as eVAX did), and
  the probe uses `#<4*<1+1+12+9>>`. The run's log also shows the
  author's system is OpenVMS V7.1 on simh's VAX 8600, not VMS 7.3 on a
  MicroVAX 3900 as "Method" assumed: G, H, and packed decimal run in the
  simulated processor, not VMS's instruction emulator. The README says
  so.
- 2026-10-02: The author's second VMS run completed (OpenVMS V7.1, simh
  VAX 8600), and audited the volume; it's `testdata/insn35/vax/
  insn35-vax.dsk.gz`. All five probes assembled, linked, and ran, 567
  records in all. `TestInsn35VAXRun` checks the run is complete and was
  of the current probes. A first look agrees with the manual where govax
  was already known to differ: MOVAO/PUSHAO set N, Z, and V (the MOVA
  fix above); MOVO from R12 is a reserved addressing-mode fault (Decision
  4); DIVF by zero is SS$_FLTDIV_F; CVTFL overflow sets V and completes,
  trapping (SS$_INTOVF, continued) only under IV; ties round away from
  zero (ADDF2, ADDD2, CVTLF). For subtask 14: VMS's PSL after the
  instruction has the previous mode user (`^X03C0nnnn`, govax `^X0300nnnn`),
  and the signal arrays' PCs differ by a few bytes, so govax's image
  layout isn't VMS's yet.
- 2026-10-02: Subtask 4 done: the floating core, `internal/vaxfloat`.
  - **`Format`** (F, D, G, H), **`Bits`** (an operand's bytes as the CPU
    loads them, `Lo`/`Hi`), and **`Value`**, an exact number held as a
    `big.Float` (Decision 2). `Unpack`/`Pack` handle each format's
    word-swapped layout, bias, zero (exponent 0, sign clear, whatever the
    fraction), and reserved operands (`ErrReserved`).
  - **Arithmetic** (`Add`, `Sub`, `Mul`, `Div`) rounds once, to the
    destination's precision, half away from zero (`big.ToNearestAway`),
    then checks the exponent: `ErrOverflow`, or `ErrUnderflow` with a
    zero result for the CPU to store when PSL<FU> is clear;
    `ErrDivideByZero`. `Round` does the same for conversions.
  - **For EMOD and POLY**: `MulExact`, `AddExact`, `Value.Split`
    (integer and fraction parts); **integers**: `Value.Int` (truncated,
    or rounded half away from zero, exact at any size), `FromInt`,
    `FromBigInt`; **literals**: `ShortLiteral`/`FindShortLiteral` (the
    value is the same in every format; `Pack` gives each format's bits),
    and `Parse` for the assembler's decimal literals.
  - **Checked against VMS**: `TestVMSVectors` runs 198 of the probes'
    floating cases (arithmetic, moves, negation, the conversions between
    formats and to and from integers) through the core and compares with
    the bits VMS stored or the condition it signalled; every one
    matches, including the ties, D's low bits, G and H division, and
    underflow with and without FU. Hand-worked tests cover the layouts,
    reserved operands and dirty zeros, each format's limits, ties, short
    literals, and integer conversion.
  - **Benchmark** (Decision 2): unpack two operands, operate, and pack
    takes 200-280 ns and 15-18 allocations on the author's machine (F
    add 206 ns, H multiply 282 ns). That's acceptable; no F/D fast path
    unless a real program shows the cost.
- 2026-10-02: Subtask 5 done: F and D on the floating core.
  - **Decode**: `decodeOperand` takes each operand's `DataType` instead of
    the instruction's short-literal type. A floating short literal is
    expanded to its value's bits in the operand's own format (so the same
    literal differs in F, G, and H), and a floating immediate stays as its
    raw bits; neither is converted to IEEE bits any more. EMODF's
    extension byte and POLYF's degree word, as literals, are integers.
  - **`fpu.go`** is now the CPU's side of `internal/vaxfloat`:
    `Engine.loadFloat`/`storeFloat` by operand and format (16-byte H
    operands through the octaword paths), `floatException` (reserved
    operand; overflow, divide by zero, and, under PSL<FU>, underflow
    faults; underflow with FU clear stores zero), and `arithmeticTrap`, a
    trap whose saved PC is the next instruction's.
  - **Handlers**, each format-generic so subtasks 6-7 only register G and
    H opcodes: ADD/SUB/MUL/DIV (`floatArithmetic`), MOV, MNEG (now clears
    C), CMP, TST, ACB, and CVT to and from integers (overflow: low-order
    bits, V, and an integer-overflow trap only under IV) and between
    formats: **CVTFD and CVTDF are new**.
  - **Assembler**: `cpu.EncodeFloat`/`DecodeFloat` take a
    `vaxfloat.Format`; the assembler still picks F or D by size, and
    still parses decimal literals through `float64` (so `MOVD #1.1` is
    3 bits off VMS's encoding), until subtask 6.
  - **Against VMS**: run under govax, the F/D probe matches VMS's records
    exactly (results, condition codes, conditions) on every case but
    EMOD/POLY (subtasks 8-9) and `MOVD #1.1`.
  - **Tests changed**: the old CVT overflow tests expected a fault on
    every overflow; they now check the manual's (and VMS's) behavior. The
    tests' float64 helpers (`fpuStore`/`fpuLoad`) moved to
    `fpu_helpers_test.go`, on the core. `shortdouble.go`'s table is gone
    (`vaxfloat.ShortLiteral`).
- 2026-10-02: Subtask 6 done: G_floating.
  - **CPU**: the G instructions (ADDG2/3 through DIVG2/3, CVTGB/GW/GL,
    CVTRGL, CVTBG/WG/LG, ACBG, MOVG, CMPG, MNEGG, TSTG, CVTGF, CVTFG)
    are the format-generic handlers registered for their FD opcodes
    (`regFD`). Under govax the G probe matches VMS on every case but
    EMODG and POLYG.
  - **Assembler**: literals and immediates take each operand's own data
    type (`Instruction.DataType`), so `CVTLG #3` and EMODG's extension
    word are integers; floating literals are parsed exactly
    (`vaxfloat.Parse`) and rounded once to the operand's format, which
    also fixes `MOVD #1.1` (it now matches VAX MACRO's encoding); new
    `.G_FLOATING` and `.H_FLOATING` directives (HELP updated). The
    disassembler shows a floating immediate in its own format, with the
    fewest digits that reassemble to the same bytes (`Value.Decimal`).
  - **Wide immediates**: the probe's `MOVO #-1` showed VAX MACRO
    zero-extends a 32-bit value into an octaword immediate, where govax
    sign-extended (Phase 28's rule for quadwords). Quadword and octaword
    immediates now zero-extend (`DEVIATIONS.md`); with that, the
    octaword probe matches VMS on all 28 cases.
  - `cpu.EncodeFloat`, `DecodeFloat`, `ShortFloat`, and `FindShortFloat`
    are gone: the assembler uses `internal/vaxfloat` directly.
    `Instruction.Type` is now read only by tests and the generator;
    subtask 15 retires it.
  - **For a later VMS run** (batched with subtask 10's `.PACKED`): the
    new directives (`.G_FLOATING`, `.H_FLOATING`, `.OCTA`), `MOVQ #-1`,
    `.QUAD`/`.OCTA` of a negative expression, and a floating operand
    given as a symbol (`MOVG #PI,R6`, which MACRO stores unconverted and
    govax doesn't support yet), each checked against VAX MACRO's object.
- 2026-10-02: Subtask 7 done: H_floating.
  - **CPU**: the H instructions (ADDH2/3 through DIVH2/3, CVTHB/HW/HL,
    CVTRHL, CVTBH/WH/LH, ACBH, MOVH, CMPH, MNEGH, TSTH) and the
    conversions CVTFH, CVTHF, CVTDH, CVTHD, CVTGH, and CVTHG are the
    format-generic handlers, registered for their FD opcodes. An H
    operand in registers spans four, through the octaword paths of
    subtask 2. Under govax the H probe matches VMS on every case but
    EMODH and POLYH (95 of 126).
  - **Assembler**: nothing more was needed: subtask 6's per-operand
    formats, exact literals, `.H_FLOATING`, and the disassembler already
    covered H (`TestFloatingImmediates`, `TestFloatingDirectives`,
    `TestDisassembleFloatingImmediates`).
  - **Tests**: `TestHFloating` (a tie in ADDH, CVTGH widening exactly
    into four registers, CVTHG overflow).
- 2026-10-02: Subtask 8 done: EMODF, EMODD, EMODG, EMODH.
  - **`vaxfloat.EMOD`** follows the manual: the extension operand's bits
    (a byte for F and D; the high 11 of the word for G, the high 15 for
    H) extend the multiplier's fraction; the exact product is truncated,
    before normalization, to a 32-bit (F), 64-bit (D, G), or 128-bit (H)
    fraction; the result splits into an exact integer part and a fraction
    part rounded to the format (`ErrUnderflow`, with both zero, if the
    fraction underflows).
  - **The CPU** (`internal/cpu/emod.go`) stores the integer's low 32 bits
    (V on overflow, an integer overflow trap under PSL<IV>) and the
    fraction; N and Z from the fraction, C cleared.
  - **Against VMS**: all 68 EMOD cases of the F/D, G, and H probes match
    VMS's run under govax (the extension's edge bits, truncation, integer
    overflow with and without IV, underflow with and without FU, and
    reserved operands). `TestEMOD`, `TestEMODExtension` (whose expected
    fraction, 255 * 2^-31, was worked out exactly from the manual's
    definition, truncation included).
- 2026-10-02: Subtask 9 done: POLYF, POLYD, POLYG, POLYH.
  - **The arithmetic** comes from the POLY "Operation" in the 1987
    *VAX Architecture Reference Manual* (EY-3459E-DP; the instruction-set
    manual leaves it out): each Horner step keeps the 31 (F), 63 (D, G),
    or 127 (H) most significant bits of the unnormalized product, adds
    the next coefficient and truncates the unnormalized sum the same way,
    then rounds; overflow and underflow are checked after the whole step,
    and underflow with FU clear zeroes the partial result and goes on.
    `vaxfloat.POLYStep` is one step, reading "truncate the unnormalized
    sum" as truncating relative to the larger operand's exponent; VMS's
    results agree.
  - **The CPU** (`internal/cpu/poly.go`) checks the degree (over 31 is a
    reserved operand), walks the table, and sets the registers the
    manual lists (the result, zeros, and the address past the table in
    R3, or R5 for POLYH); POLYH also leaves its argument in the 16 bytes
    below SP, as the manual's Operation does. A fault leaves the
    registers unchanged (no PSL<FPD> resumption).
  - **Against VMS**: all 56 POLY cases match, so every floating and
    octaword case of the probes (413 in all) now matches VMS's run under
    govax: result bytes, R0-R11, condition codes, and conditions.
    `TestPOLYStep`, `TestPOLYFManualExample` (the manual's example).
- 2026-10-02: Subtask 10 done: the packed decimal core, and MOVP, CMPP3,
  CMPP4, CVTLP, CVTPL; `.PACKED`.
  - **What the manual leaves UNPREDICTABLE**, VMS's run settles: an
    invalid digit nibble (^XA-^XF) isn't checked. MOVP copies it as it
    is; the arithmetic uses its value (CVTPL of "4C6" is 526; CMPP3 of
    "12B" with "123" is greater). An even length's unused high nibble is
    written as zero. Any sign nibble other than ^XB or ^XD is plus.
  - **Results** (`internal/cpu/decimal.go`): the preferred signs, ^XC and
    ^XD; a zero result is plus, but an overflowed negative result keeps
    ^XD over zero digits (VMS: ADDP4 of -123 and -877 into 3 digits is
    "000D", N clear, Z and V set); N only for a negative result with a
    nonzero digit. Decimal overflow stores the low-order digits, sets V,
    and under PSL<DV> traps (type 6, SS$_DECOVF). A length over 31 is a
    reserved operand before anything changes.
  - **A fault part way** (the plan's open question): like MOVC3 and the
    other character instructions, the decimal instructions don't model
    PSL<FPD>. Each reads all its sources, works out the result, writes
    the destination, and only then sets the registers, so after a memory
    fault the restarted instruction redoes the same work (CVTPL sets its
    registers before storing, as the manual requires, so its destination
    may be R0-R3).
  - **Against VMS**: all 47 MOVP, CMPP3, CMPP4, CVTLP, and CVTPL cases of
    the packed probe match, registers included. `TestMOVP`, `TestCMPP`,
    `TestCVTLP`, `TestCVTPL`, `TestDecimalLengthOver31`.
  - **`.PACKED decimal-string[,symbol]`** (both dialects; HELP updated),
    from the MACRO manual, with its examples as `TestPacked`. Whether an
    unsigned string gets ^XC and `-0` keeps ^XD is for the batched VMS
    fixture run (subtask 6's note).
- 2026-10-02: Subtask 11 done: ADDP4, ADDP6, SUBP4, SUBP6, MULP, DIVP,
  ASHP (`internal/cpu/decimalmath.go`).
  - Each computes the exact result from its sources' values and stores it
    with the core's rules (low-order digits and V on overflow, the DV
    trap); the registers are the manual's (R1, R3, R5 the operand
    addresses). DIVP truncates toward zero, and a zero divisor is the
    divide-by-zero trap (type 4) with nothing changed, as VMS did. ASHP
    rounds a right shift by adding the round operand's low nibble to the
    most significant discarded digit.
  - **Against VMS**: every case of these instructions matches but one,
    DIVP with an invalid divisor digit, whose result the manual makes
    UNPREDICTABLE (`DEVIATIONS.md`; masked in subtask 14).
    `TestDecimalArithmetic`, `TestDIVPByZero`, `TestASHP`.
- 2026-10-02: Subtask 12 done: CVTPS, CVTSP, CVTPT, CVTTP
  (`internal/cpu/decimalconvert.go`).
  - From the manual: CVTPS writes a sign byte by the source's value (-0
    gives "+") and the digits in ASCII; CVTSP takes "+", "-", or a space,
    and digits "0"-"9", any other byte a reserved operand; CVTPT
    translates the source's last byte (last digit and sign, even for -0)
    through the program's 256-byte table into the destination's last
    byte; CVTTP translates the source's last byte back into the last
    digit and sign, a non-digit in any other byte or an invalid
    translation being a reserved operand. The conversions to numeric
    strings set N and Z from the source's value, those to packed strings
    from the result; overflow keeps the low-order digits, sets V, and
    traps under DV.
  - **Against VMS**: all 28 cases of the packed probe match.
    `TestCVTPSAndCVTSP`, `TestCVTPTAndCVTTP`.
- 2026-10-02: Subtask 13 done: EDITPC (`internal/cpu/editpc.go`).
  - Every pattern operator, from the instruction-set manual and the 1987
    architecture manual's Operation (which neither has a worked EDITPC
    example; the tests use cases VMS confirmed): the fill and sign
    registers, significance, Z, V from EO$ADJUST_INPUT's discarded
    digits, EO$BLANK_ZERO and EO$REPLACE_SIGN reaching back into the
    output, N cleared at the end for -0, and the manual's registers.
  - A source length over 31, a reserved pattern operator, a pattern that
    reads a digit the source doesn't have, and digits left over at
    EO$END are reserved operands. Digits are read one at a time, as the
    1987 manual's READ does, so the output before the missing digit is
    stored, and VMS's run shows the destination keeps what was produced
    before any of these aborts; govax stores it too. The registers are
    left unchanged (no PSL<FPD>, so not the manual's part-way state).
  - **Against VMS**: all 22 EDITPC cases match. With that, every case of
    the packed probe matches VMS's run under govax but the one
    UNPREDICTABLE DIVP case (subtask 11). `TestEDITPC`.
- 2026-10-02: Subtask 14 done: `TestInsn35Oracle`.
  - It runs each probe under govax and compares every record with VMS's
    run: flags, the whole PSL, R0-R11, the whole signal array (its PC
    too: govax's MACRO and LINK lay the probes out exactly as VMS's
    did), and the destination bytes. All 567 cases match but for the
    masked fields, each with its reason in the test and in
    `DEVIATIONS.md`: DIVP's UNPREDICTABLE quotient for an invalid digit
    (subtask 11), and PSL<FPD> in the signal arrays of EDITPC's three
    aborts (govax doesn't model FPD).
  - **One more fix it needed**: every PSL differed in its previous-mode
    field. govax ran images in user mode with a previous mode of kernel,
    which REI would never allow; VMS has user. A drop to a less
    privileged mode now sets the previous mode to the new one
    (`setModeStack`; `DEVIATIONS.md`).
  - The test skips a probe whose source differs from the one VMS ran, so
    a change to `gen.go` asks for a new VMS run rather than failing.
- 2026-10-02: Subtask 15 done: clean-up of the old "missing" references.
  - **`TestEveryInstructionImplemented`**: every instruction in the table
    has a handler but LDPCTX and SVPCTX (Decision 1) and the reserved and
    prefix placeholder entries, so SHOW INSTRUCTIONS/UNIMPLEMENTED lists
    only those. Subtask 1's `TestNewInstructionsDecodeAsReserved` became
    `TestNewInstructionsDecode` (every Phase 35 instruction still decodes
    to its full length with 16-byte immediates; none is reserved now).
  - **`Instruction.Type` is retired**, with the `ShortLiteralType` type:
    a short literal's kind comes from its operand's `DataType` (subtask
    5). The generator still checks the C header's OP_TYPE against the
    manual but no longer emits it; the table was regenerated (only the
    `Type` lines changed), and its header and the generator's comment now
    give the `go run ./gen` command, since the `go:generate` directive is
    disabled.
  - **Notes, not rewrites**: PHASE-05.md (D's three lost bits; EMODF,
    POLYF, CVTFD and the D counterparts), PHASE-28.md (G/H, octawords,
    packed decimal "for a later phase"), DEVIATIONS.md's Phase 28
    quadword-immediate entry, and `dispatch.go`'s comment on the
    unimplemented handler. The other comments the plan named were
    rewritten as their subtasks went: `fpu.go`, `pseudo.go`'s
    `pseudoFloat`, and `operand.go`'s wide-immediate comments.
- 2026-10-02: Subtask 16 done: close-out. PLAN.md's index, CLAUDE.md's
  package notes (`internal/vaxfloat`, and `internal/cpu`'s Phase 35 work),
  and this doc's status. HELP gained `.G_FLOATING`, `.H_FLOATING`,
  `.OCTA`, and `.PACKED` along the way (subtasks 2, 6, and 10).
  - **Outcome**: all 79 instructions are implemented, and every one of
    the probes' 567 cases matches VMS 7.1's run under govax but two
    masked fields (an UNPREDICTABLE DIVP result, and PSL<FPD> on EDITPC's
    aborts). Fixes to existing code along the way: D_floating's lost
    bits and tie rounding, MNEG's C, MOVA/PUSHA's condition codes,
    floating divide by zero, floating-to-integer overflow and the IV
    trap, the previous mode images run with, MACRO's parenthesized
    expressions, wide immediates' zero extension, and the tests' stale
    copy of the kernel.
  - **Still open**: the assembler fixture (`testdata/insn35/asm`), for
    the author's next VMS session. Its object will settle whether govax's
    `.QUAD`/`.OCTA` of a negative value, `MOVQ #-1`, `.PACKED`'s unsigned
    and `-0` signs, and the floating directives match VAX MACRO's, and
    how MACRO stores a floating operand given as a symbol (`MOVG #PI`),
    which govax doesn't assemble yet. A test comparing the objects comes
    with that run.
