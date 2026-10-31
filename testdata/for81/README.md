# The Phase 81 probe: FORRTL's file statements, and RMS's files

These fixtures are `docs/PHASE-81 - FORTRAN files.md`'s subtask 2. The simh VAX has
no FORTRAN compiler, so VMS's own FORRTL.EXE is called from MACRO, and its behavior
becomes FTNRTL's requirement (only what it does is used, never its code). An RMS
program in MACRO does the same for relative and fixed-length files, whose bytes
become RMS's requirement.

`gen.go` writes everything here but this README and the output files:

    go run testdata/for81/gen.go

| File | What it holds |
| ---- | ------------- |
| `for81.mar` | FORRTL: `FOR$OPEN`, `FOR$CLOSE`, `FOR$INQUIRE`, USEROPEN, segments, REWIND, BACKSPACE, ENDFILE, direct access, FIND, DEFINE FILE, relative files, DELETE, REWRITE, UNLOCK |
| `for81r.mar` | RMS: relative files' layout and operations, keyed `$PUT` and `$GET` on FIX sequential files, odd records, FOP's bits, `$PARSE` with no NAM |
| `for81.com`, `for81b.com` | Build and run both |
| `exchange.cmd`, `copyout.cmd` | The govax console scripts for the exchange volume |
| `vax/for81-vax.txt`, `vax/for81r-vax.txt`, `vax/for81.log` | VMS's run |

## How the programs work

A case is a routine of its own, and each statement in it another, under a handler
that prints each condition, continues from one below severe, and unwinds that
statement from a severe one. So a failure costs one statement, and the case goes on.
After every case the driver closes units 10 to 19.

    #n  title
    =WHAT r=x          a statement or service, and its R0
    =c cond n=x a=x x  a condition: the signal array's size and first two arguments
    =u                 the handler unwound
     fab ... / rab ... the unit's FAB and RAB, through FOR$RAB
     q34 r=x |text|    INQUIRE's answer to one question, by its code
     stat name: ...    a closed file's attributes, by $OPEN with XABFHC
     rec n: ...        its records, the first 32 bytes of each in hex
     vbn n             its blocks in hex, by block I/O, all-zero lines left out
     uo ...            what a USEROPEN routine was given: FAB, RAB, names, XABs

Written bytes come from a buffer holding each offset's low byte, so a record shows
which bytes it holds. A read's buffer starts as 55s.

`FOR$OPEN`'s keywords are in the phase doc, found by calling it over govax.

## The VAX run

Allow several minutes: MACRO has 25,000 lines to read. The output has no times in
it to mask but `DIRECTORY`'s. The PRINT and SUBMIT cases really ask for those: each
file holds a line or two (`$ EXIT` in the CLOSE cases), so a queue that's running
prints or runs something harmless. If nothing is set up they fail, which is one of
the things to see, and `SHOW QUEUE/ALL` in the log shows which it was.

If a case hangs, Ctrl/Y and tell me which `#n` `FOR81-VAX.TXT` or `FOR81R-VAX.TXT`
reached. The likeliest is the last case of `for81.mar`, a record locked by another
unit, which is last for that reason.

1. Build the exchange volume, from the repository root:

       govax console < testdata/for81/exchange.cmd

   This makes `testdata/disks/for81-exchange.dsk` (RD54 size, label FOR81,
   gitignored); hundreds of small files need more index room than an RD51 has. It
   needs `SET RQ2 RD54` at the `sim>` prompt before the attach.
2. Attach it to the simh VAX as a spare drive, DUA2 for example, log in as SYSTEM,
   and:

       $ MOUNT DUA2: FOR81
       $ SET DEFAULT DUA2:[000000]
       $ @FOR81/OUTPUT=FOR81.LOG
       $ DISMOUNT DUA2:

3. Copy the results back, from the repository root:

       govax console < testdata/for81/copyout.cmd

No licensed file goes on the volume, so it needs no erasing.

## Round 1's results

| File | What it holds |
| ---- | ------------- |
| `vax/for81-vax.txt` | Cases #0 to #579 of `FOR81`. #580 ran away (the phase doc), so it is cut there |
| `vax/for81r-vax.txt` | `FOR81R`, whole |
| `vax/for81.log` | `FOR81.COM`'s log |
| `vax/for81-blocks.txt` | VMS-made files' blocks, by govax's DUMP/BLOCK on the exchange volume, all-zero lines left out (the programs' own dumps failed with RMS$_UPI) |

## Round 2

`for81c.mar` (72 cases on FORRTL), `for81d.mar` (28 on RMS), and `for81x.mar` (units left
open at image exit) ask what round 1 left open; the phase doc lists it. On its own volume:

    govax console < testdata/for81/exchangec.cmd

makes `testdata/disks/for81c-exchange.dsk` (RD54 size, label FOR81C). At `sim>`, `SET RQ2 RD54`
before the attach. On the VAX:

    $ MOUNT DUA2: FOR81C
    $ SET DEFAULT DUA2:[000000]
    $ @FOR81C/OUTPUT=FOR81C.LOG
    $ DISMOUNT DUA2:

then `govax console < testdata/for81/copyoutc.cmd`. A hang is likeliest in `FOR81C`'s last case, a
record held by another unit: Ctrl/Y, and say so. The volume's own files are still wanted after
that, so please don't erase it. `SYS$BATCH` has 12 of round 1's jobs pending, each `$ EXIT` or an
empty file: `DELETE/ENTRY` or leave them, as the queue is stopped.
