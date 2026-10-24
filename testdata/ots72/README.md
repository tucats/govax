# The Phase 72 probe: LIBRTL's OTS$ routines

These fixtures are `docs/PHASE-72 - legacy images.md`'s subtask 6. Two
MACRO programs:

- **`ots72.mar`** calls the OTS$ routines govax added in subtask 6, case
  by case, and prints what each returned, signaled, and stored: the
  floating conversions (`OTS$CNVOUT`, `_G`, `_H`; `OTS$CVT_T_D`, `_F`,
  `_G`, `_H`), the moves (`OTS$MOVE3`, `OTS$MOVE5`, and their JSB
  entries), and the dynamic strings (`OTS$SCOPY_DXDX`, `OTS$SCOPY_R_DX`,
  `OTS$SGET1_DD`, `OTS$SFREE1_DD`, `OTS$SFREEN_DD`, and theirs). The
  cases cover the manual's examples, each flag, and every choice the
  phase doc marked unconfirmed. `TestOTS72Oracle`
  (`internal/console/ots72_test.go`) holds govax's output to VMS's,
  `vax/ots72-vax.txt`, as `TestLib54Oracle` does Phase 54's.
- **`ots72t.mar`** is a study, run on VMS only: how VMS's FORRTL calls
  LIBRTL's undocumented `OTS$$CVT_D_T_R8`, `_F_`, `_G_`, and `_H_`,
  which its `F`, `E`, `D`, and `G` edit descriptors use. No manual
  describes them, so the program watches FORRTL call them (below);
  govax's versions (`internal/librtl/otscvtfor.go`) were written from
  what it caught, and `TestOTSCvtForDigits` checks them against it.

`gen.go` writes everything here but the output files:

    go run testdata/ots72/gen.go

| File | What it holds |
| ---- | ------------- |
| `ots72.mar` | The probe: 148 cases |
| `ots72t.mar` | The study: a self-test, then 29 FORRTL calls (`FOR$CVT_x_Ty`) |
| `ots72.com` | Assembles, links, and runs both on VMS (`OTS72T` linked with `SYS$SHARE:FORRTL`) |
| `exchange.cmd` | The govax console script that builds the exchange volume |
| `copyout.cmd` | The govax console script that copies the results back into `vax/` |

## How `ots72.mar` works

As Phase 54's probe (`testdata/lib54`): each case runs in a routine of
its own, whose condition handler records the condition it's given (and
an access violation's reason mask and address) and unwinds, so the
output tells a status a routine *returned* from a condition it
*signaled*. A case prints

    name R0=xxxxxxxx [signal condition mask address]

then a line for each value it shows: longwords in hex (a floating
value's bits, as they're stored), or a string's length and text. An
output string starts as `~` characters, so the text shows what a
routine didn't write.

A JSB case sets R1 to R6 to `5A5A5A51` to `5A5A5A56`, loads its
arguments into R0, R1, ..., and shows R0 to R6 after. A register that
holds a program address after the call is shown relative to a label:
its argument's, or the string it should point past. `OTS$MOVE3_R5`'s
case puts two different destinations in R2 and R3, since nothing says
which register it takes.

## How `ots72t.mar` works

FORRTL's conversions are called with `FOR$CVT_x_Ty`'s arguments (the
2.0 Run-Time Library manual's: value, string, digits in the fraction,
scale factor, digits in the integer, digits in the exponent, flags).
Run under govax, those routines were seen to JSB to `OTS$$CVT_x_T_R8`
with R0 the value's address and R1 their own FP, the routine's results
going into their frame; what the frame holds is the question.

Each case sets PSL<T> before its `CALLS`, so the trace trap follows
each instruction. The case's handler, `TRACER`, takes each `SS$_TBIT`;
when the next instruction is the `OTS$$CVT_x_T_R8` entry, it resumes at
`CAPTURE` instead, and stops the tracing. `CAPTURE` records R0 to R11,
AP, FP, and SP, 16 bytes at R0, and the 192 bytes from R1-176 to R1+16
(the caller's frame), replaces the JSB's return address with `RETCAP`,
and jumps to the routine; `RETCAP` records the registers and the frame
again and returns to FORRTL. The case then prints the string FORRTL
wrote, the tracer's state (2 if both points were caught), and the two
records, the frame a line of four longwords each, named by the first
one's offset from R1.

Case 0 tests the tracer on the program's own JSB routine: it should show
tracer state 2, R2 `12345678` at the RSB, and `87654321` in the frame's
last longword below R1 at the RSB.

govax has no trace trap, so on govax every case shows tracer state 0,
and the FORRTL calls signal `SS$_UNSUPPORTED` (govax lacks the routine
being studied).

## The VAX run

1. Build the exchange volume, from the repository root:

       govax console < testdata/ots72/exchange.cmd

   This makes `testdata/disks/ots72-exchange.dsk` (RD51 size, label
   OTS72, gitignored), on DUA2.
2. Attach it to the simh VAX, mount it, set its `[000000]` as the default
   directory, and run:

       @OTS72/OUTPUT=OTS72.LOG

   `OTS72.OUT` and `OTS72T.OUT` get the programs' output, `OTS72.MLG`
   and `OTS72T.MLG` MACRO's messages (none are expected), and
   `OTS72.LOG` the procedure's own.
3. Copy the results back:

       govax console < testdata/ots72/copyout.cmd

   which puts them in `vax/` as `ots72-vax.txt`, `ots72t-vax.txt`, and
   `ots72.log`.

## The run

2026-10-24, OpenVMS V7.1 on simh's VAX 8600. MACRO wrote no messages,
both programs exited with SS$_NORMAL, and the tracer caught every call.
VMS differed from govax's choices in about a dozen lines of
`ots72-vax.txt`; govax now does what VMS did (the phase doc's "The
probe's answers (`testdata/ots72`)" lists them), but for what
`TestOTS72Oracle` expects to differ: H's extension bits, which VMS
computes to 128 bits only, and the registers the dynamic-string JSB
entries leave (VMS's heap internals). Run on govax with VMS's FORRTL,
`ots72t.mar`'s FORRTL conversions give VMS's text but for the 40-digit
H case's last digits, where VMS's digits drift from the true value's.
