# The Phase 77 probe: MTHRTL

These fixtures are `docs/PHASE-77 - MTHRTL.md`'s subtask 10. One MACRO program calls
MTHRTL's routines, case by case, and prints what each returned and signaled. Run on VMS,
it settles the questions the phase doc's "Open questions" lists.
`TestMTH77Oracle` (`internal/console/mth77_test.go`) compares govax's output with VMS's
(`vax/mth77-vax.txt`), and `TestMTH77Runs` checks that the program runs to its end on
govax, whatever VMS says.

`gen.go` writes everything here but this README and the output files:

    go run testdata/mth77/gen.go

| File | What it holds |
| ---- | ------------- |
| `mth77.mar` | The program, 1,950 lines of output: `MTH$RANDOM`; each family's conditions and limits; Table 1-1's; the complex functions; the `OTS$` powers and complex arithmetic; the JSB entries the manuals give, and those they don't; a sweep of 1,072 calls with arguments from a fixed sequence |
| `mth77.com` | Assembles, links, and runs the program |
| `exchange.cmd` | The govax console script that builds the exchange volume |
| `copyout.cmd` | The govax console script that copies the results back |
| `vax/mth77-vax.txt` | The program's output on VMS (`MTH77.OUT`) |
| `vax/mth77.log` | `MTH77.COM`'s log on VMS |

## How the program works

The table holds each case's routine and title, and how many registers and result
longwords to show. The driver presets the result area, `RES`, to `^X55555555`, calls the
case, and prints a line:

    title R=registers M=result-longwords c=condition n=size a=argument pc=ret mr=R0 R1 unw

`R=` and `M=` appear only if the call came back. `c=` and what follows appear only if a
condition was signaled, and show the first signal array and the mechanism array's R0 and
R1 as the handler found them. An argument equal to the call's return address is `a=ret`,
and the PC is `pc=ret` or `pc=oth`. A title ends `FU` if the caller had floating underflow
enabled, `UW` if the handler was told to unwind whatever the condition, and `CORR` if it
stored 1.5 in the mechanism array's R0 and R1 before continuing.

**The handler doesn't continue from just anything.** Returning `SS$_CONTINUE` from a fault
runs the faulting instruction again, so on VMS it would loop forever. It continues only from
the library's own conditions (facility MTH) and the arithmetic traps, whose PC is past the
instruction. It unwinds from everything else (`unw`), and from any second signal in a case
(`h=` then counts them), so no case can hang that way.

`RANDOM(...)`'s seed comes back in `M=`. The JSB entries no manual gives are shown with every
register, R0 to R11, with markers (`A1A1A1A1`, ...) in those not holding an argument, since
their registers are the question. `MTH$$REDUCE_`, internal to VMS's library, isn't called.

A title starting `~` is a line of the sweep, whose results the oracle compares to within a
count of last bits. The letter after it is the format.

## The VAX run

1. Build the exchange volume, from the repository root:

       govax console < testdata/mth77/exchange.cmd

   The scripts use DUA2 on govax, since a startup procedure may mount DUA1. This makes `testdata/disks/mth77-exchange.dsk` (RD51 size, label MTH77, gitignored).
2. Attach it to the simh VAX as a spare drive, DUA2 for example (the startup procedure mounts
   DUA1), log in as SYSTEM, and:

       $ MOUNT DUA2: MTH77
       $ SET DEFAULT DUA2:[000000]
       $ @MTH77/OUTPUT=MTH77.LOG
       $ DISMOUNT DUA2:

3. Copy the results back, from the repository root:

       govax console < testdata/mth77/copyout.cmd

No licensed file goes on the volume, so it needs no erasing. `MTH77.MLG` is MACRO's log, and
isn't copied.
