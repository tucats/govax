# The Phase 83 probe: FORRTL's system subroutines, conditions, and LIBRARY/TEXT

These fixtures are `docs/PHASE-83 - FORTRAN and VMS.md`'s subtask 2. The simh VAX has no
FORTRAN compiler, so VMS's own FORRTL.EXE and LIBRTL.EXE are called from MACRO, and what they
do becomes FTNRTL's and LIBRTL's requirement (only what they do is used, never their code).
`testdata/str83` is the same volume's other program, the STR$ routines.

`gen.go` writes everything here but this README and the output files:

    go run testdata/for83/gen.go

| File | What it holds |
| ---- | ------------- |
| `for83.mar` | Part A, 79 cases: `FOR$DATE`, `FOR$TIME` and their `_T_DS` forms, `FOR$IDATE`, `FOR$JDATE`, `FOR$SECNDS`, `FOR$IMVBITS`, `FOR$JMVBITS`, `MTH$RANDOM`, with short buffers, missing arguments, and bits out of range |
| `for83b.mar` | Part B, 64 runs on FORRTL: nine errors under a handler that continues, unwinds, or calls LIB$SIG_TO_RET; a condition between a statement's items; I/O from a handler and from an AST during a statement, on the same unit, another, an internal file, unit 6, and a CLOSE; every way of ending an image with a unit open; exit handlers declared before and after an OPEN |
| `for83c.mar` | Part C, 109 runs on LIBRTL: LIB$SIGNAL, LIB$STOP, LIB$ESTABLISH, LIB$REVERT, LIB$MATCH_COND, LIB$SIG_TO_RET, LIB$SIG_TO_STOP, LIB$FIXUP_FLT with a reserved operand in each place and format, LIB$INT_OVER, LIB$DEC_OVER, LIB$FLT_UNDER |
| `lnk83.mar` | References only: LINK's log says which definitions the system's libraries don't define globally, which decides what `EXTERNAL NAME` can find |
| `def_opc.mar` | `$OPCDEF GLOBAL`: its values from the object, no listing, as `testdata/mp/defs` |
| `for83.com` | Builds and runs them all, then part D: LIBRARY/TEXT, each library kept at each step for its bytes |
| `for83r.com` | Runs a program's cases, one image each, then types and deletes what a case wrote |
| `exchange.cmd`, `copyout.cmd` | The govax console scripts for the exchange volume |

## How the programs work

Part A is laid out as `testdata/lib54`'s: a case is a routine under a handler that records a
condition and unwinds, so a status returned shows apart from one signaled.

    name R0=x [signal cond mask address]
      label: longwords or text

SECNDS's value depends on the time, so a case shows `(value - (now - x)) / 100`, rounded: 0, or
864 if it added a day. A `DATE` or `TIME` buffer shows with its 4 bytes of guard, `UUUU`.

Parts B and C run one case per image, by the number on a foreign command, since a case may end
its image. A handler chooses what to do by the case, stops the program after 12 conditions rather
than looping, and prints

    #n title
     =c n=x cond=x arg=x x pc=+x psl=x   the signal array; pc is from the routine under test
     =m n=x frame=+x depth=n r0=x r1=x   the mechanism array; frame is from that routine's FP
     =u                                  the handler's call for an unwind
     =what r0=x [r1=x]                   a place the case reached
     =value x, =read r0=x |text|         what a statement got
    $STATUS == "%X..."                   the image's end
     file NAME:                          what the case wrote

Some lines hold what varies between systems (a PC in FORRTL, a bad address); the oracle tests
mask those.

## The VAX run

Allow several minutes: three programs of several thousand lines to assemble, then 173 images.

If a case hangs, Ctrl/Y and tell me which `#n` the last `*-VAX.TXT` reached. Part C's FLT_UNDER
cases, where a trap may repeat, are the likeliest, and the handler should stop those itself.

1. Build the exchange volume, from the repository root:

       go run testdata/for83/gen.go
       go run testdata/str83/gen.go
       govax console < testdata/for83/exchange.cmd

   This makes `testdata/disks/for83-exchange.dsk` (RD54 size, label FOR83, gitignored). It
   needs `SET RQ2 RD54` at the `sim>` prompt before the attach.
2. Attach it to the simh VAX as a spare drive, DUA2 for example, log in as SYSTEM, and:

       $ MOUNT DUA2: FOR83
       $ SET DEFAULT DUA2:[000000]
       $ @FOR83/OUTPUT=FOR83.LOG
       $ DISMOUNT DUA2:

3. Copy the results back, from the repository root:

       govax console < testdata/for83/copyout.cmd

No licensed file goes on the volume, so it needs no erasing.
