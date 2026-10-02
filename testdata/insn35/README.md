# The Phase 35 oracle: G, H, octaword, and packed decimal instructions

These fixtures are `docs/PHASE-35.md`'s subtask 3. Each instruction this
phase adds, and the existing F_floating and D_floating instructions whose
results the new floating core changes, is written from the *VAX
Architecture Reference Manual*. Then it's checked against what VMS 7.3
does with these probe programs.

Everything here except `vax/` is written by `gen.go`:

    go run testdata/insn35/gen.go

| Files | What they hold |
| ----- | -------------- |
| `p35fd.mar` | F_floating and D_floating: moves, compares, and arithmetic with rounding ties, D's low fraction bits, reserved operands, overflow, underflow (with and without PSL<FU>), divide by zero, the integer conversions, ACBF/ACBD, CVTFD, CVTDF, EMODF/EMODD, and POLYF/POLYD |
| `p35g.mar` | G_floating: every instruction, CVTGF and CVTFG, literals and immediates, register pairs, EMODG, and POLYG |
| `p35h.mar` | H_floating: every instruction, the conversions to and from F, D, and G, literals and immediates, register quads, EMODH, and POLYH |
| `p35o.mar` | Octawords: CLRO, MOVO, MOVAO, and PUSHAO in each addressing mode, immediates, condition codes, and an octaword from R12 |
| `p35p.mar` | Packed decimal: MOVP, CMPP3/4, CVTLP, CVTPL, ADDP4/6, SUBP4/6, MULP, DIVP, ASHP, CVTPS, CVTSP, CVTPT, CVTTP, and EDITPC, with bad digits and signs, lengths 0, 31, and 32, overflow (with and without PSL<DV>), and divide by zero |
| `insn35.com` | Assembles, links, and runs the probes (all, or those named in P1) |
| `exchange.cmd` | The govax console script that builds the exchange volume |

## How a probe works

Each case is one instruction, run in a routine of its own under a
condition handler. Before the instruction the routine fills DST (where
write and modify operands go) with `^XAA`, copies in any modify operand's
starting value, loads R0-R11 with `^X5A5A5Ann` (nn the register number)
or the case's own values, and sets N, Z, V, and C, so a condition code the
instruction leaves alone reads as 1. Right after the instruction it saves
the PSL and R0-R11.

Floating operands are written as bytes, so every value is exact: reserved
operands, values with all the low fraction bits set, rounding ties. A few
cases use literal and immediate operands (`#0.5`, `#1.1`), so they also
check MACRO's encoding and the CPU's expansion of each format's short
literals.

The handler records the signal array. After a trap (overflow or divide by
zero in integer or decimal arithmetic, floating underflow, and the
traps of older processors), it continues, so the instruction's results
are still recorded. After a fault (reserved operand, reserved addressing
mode, or a floating fault), it unwinds, and the case records only the
signal. A second condition in one case unwinds too, so a run on an
emulator that reports a trap as a fault still finishes.

Each probe writes `name.DMP`: one 260-byte variable-length record per
case, with the case's number and name, flags (completed, branch taken,
signalled), the PSL, R0-R11, up to 8 longwords of the signal array, and
DST's 128 bytes. `gen.go`'s comment gives the layout.

## The VAX run

1. Build the exchange volume, from the repository root:

       govax console < testdata/insn35/exchange.cmd

   This makes `testdata/disks/insn35-exchange.dsk` (RD51 size, label
   INSN35, gitignored).
2. Attach it to the simh VAX, mount it, set its `[000000]` as the default
   directory, and run:

       @INSN35/OUTPUT=INSN35.LOG

   To run only some probes, name them: `@INSN35/OUTPUT=INSN35.LOG
   P35G,P35H`. Running a probe again writes a new version of its .DMP.
3. Skim `INSN35.LOG` for MACRO or LINK messages, then dismount the volume
   and copy the container back, gzipped, as `vax/insn35-vax.dsk.gz`.

The log records the VMS version and the hardware name
(`F$GETSYI("HW_NAME")`). The author's system reports OpenVMS V7.1 on a
VAX 8600, simh's model of a VAX with the full instruction set, so G,
H, and packed decimal run in the simulated processor rather than in VMS's
instruction emulator (which a MicroVAX would use for H and packed
decimal). Either implements the architected results, so either is the
reference; if a difference turns up in those families, this is worth
remembering.

The run's container is `vax/insn35-vax.dsk.gz` (2026-10-02, OpenVMS
V7.1 on simh's VAX 8600, node SIMVAX, device DUA1). It holds the probes,
their objects and images, the five .DMP files (all 567 cases), and
`INSN35.LOG`; the author audited it. `TestInsn35VAXRun` checks it has a
record for every case and that the probes it ran are the ones here.

An earlier run (2026-10-02) stopped at MACRO: each probe grouped an
expression with parentheses (`#<4*(1+1+12+9)>`), which VAX MACRO rejects
with `%MACRO-E-ILLEXPR` (it groups only with angle brackets). govax's
MACRO had accepted it; it now reports the same error.

## govax's run

`TestInsn35ProbesRun` (`internal/console/insn35_probe_test.go`) builds
each probe with govax's own MACRO and LINK, runs it on a fresh volume, and
checks that it wrote a well-formed record for every case.
`TestInsn35Oracle` compares every record with VMS's: flags, PSL, R0-R11,
the signal array, and the destination bytes. All 567 cases match, but for
the fields `insn35Masks` lists with their reasons (DIVP's UNPREDICTABLE
quotient for an invalid digit, and PSL<FPD> in EDITPC's aborts, which
govax doesn't model).

Some registers the instructions leave hold addresses (packed decimal and
POLY leave operand addresses in R1, R3, and R5), and a signal array holds
the PC. They match because govax's MACRO and LINK lay each probe's image
out exactly as VMS's do.

## The assembler fixture

`asm/` holds a second, smaller VMS run: `asm35.mar`, assembled by VAX
MACRO so govax's MACRO can be compared with its object byte for byte. It
covers what govax's assembler had to decide in Phase 35 without VAX
MACRO's output to go by: `.F_FLOATING` through `.H_FLOATING`, `.QUAD`
and `.OCTA` of negative values and symbols, `.PACKED` (signed, unsigned,
`-0`, 31 digits, and its length symbol), quadword and octaword immediates
of negative expressions, floating literals and immediates in each format,
integer literals on floating instructions, and floating operands given
as symbols (which govax doesn't assemble yet).

1. Build its exchange volume, from the repository root:

       govax console < testdata/insn35/asm/exchange.cmd

   This makes `testdata/disks/asm35-exchange.dsk` (label ASM35).
2. On VMS, mount it, set its `[000000]` as the default directory, and
   run `@ASM35/OUTPUT=ASM35.LOG`.
3. Copy the container back, gzipped, as `asm/vax/asm35-vax.dsk.gz`.
