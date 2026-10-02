# Phase 35 — The rest of the instruction set: G, H, octaword, and packed decimal

**Status:** in progress (2026-10-02). The author accepted the plan and
every proposed answer under "Decisions".

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
