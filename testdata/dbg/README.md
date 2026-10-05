# Phase 41's debugger probe

How the VMS debugger (VMS 7.3, on simh) shows instructions, symbols,
addresses, and call chains for images with debug data, for govax's
symbolic disassembler to be checked against (docs/PHASE-41.md, subtask
1). Clean room: what govax does comes from these logs and DIGITAL's
manuals, never VMS's source.

## The sources

| File | What it is |
|---|---|
| `dbgdis.mar`, `dbgsub.mar` | A two-module program whose instructions refer, in every addressing mode, to what a DST names: labels of code and data, routines (CALLS, CALLG, JSB, BSBW), constants, a global constant, a psect base with no label, addresses past a label, the other module's data, a LIBRTL routine, branches, and a case table. It runs to the end without faulting (govax runs it too). |
| `../mar/list/trace.mar` | Phase 29's TRACE, built four ways |
| `../mar/list/failmain.mar`, `failsub.mar` | Phase 29's program with an access violation two calls deep |
| `../mar/forth.mar` | FORTH: a large module, system macros' code, labels a macro defines |

## The debugger command files

`DBG.COM` runs each image under the debugger with one of these, logging
the session (commands and output) to `IMAGE.DLG`:

| File | Images | What it asks |
|---|---|---|
| `dbgdis.dbg` | DBGDIS (`/DEBUG`), GVDBGDIS (govax's), DBGTRC (traceback link) | `SHOW MODULE`/`IMAGE`/`MODE`/`SYMBOL`; `EXAMINE/INSTRUCTION` by routine, range, `%LINE`, symbolic and `NOSYMBOLIC`, hex and decimal; `EXAMINE` of data; `EVALUATE`, `EVALUATE/ADDRESS`, `SYMBOLIZE`; `STEP` by instruction and line; `EXAMINE/OPERANDS`; breaks in JSB subroutines and three and four calls deep, with `SHOW CALLS` |
| `trace.dbg` | TRDBGLNK, TRLNKDBG, TRDBGTRC, GVTRACE (govax's) | the same questions, smaller, for each kind of image: DBG records and `LINK/DEBUG`, TBT records only with `LINK/DEBUG`, DBG records with a traceback link |
| `notb.dbg` | DBGNOTB, TRNOTB (`/NOTRACEBACK`) | what the debugger does with no DST, if it starts at all |
| `fail.dbg` | FAILLNK (`/DEBUG` both) | the access violation and `SHOW CALLS` there (Decision 6) |
| `forth.dbg` | FORTH | instructions in system macros' code, JSB routines, macro-defined labels; not run, since FORTH reads the terminal |

GVDBGDIS and GVTRACE are govax's `MACRO/DEBUG` and `LINK/DEBUG` output,
built onto the volume by `exchange.cmd`. Their sessions should match
DBGDIS's and TRDBGLNK's but for dates; they're also Phase 29's
Decision 7 check (whether VMS minds a GST padded to a whole block).

## Running it

1. Build the volume, from the repository root (it makes
   `testdata/disks/dbg-exchange.dsk`, label DBGXCHG, gitignored):

       go build -o govax ./cmd/govax
       ./govax console < testdata/dbg/exchange.cmd

2. On VMS, with the disk attached to simh:

       $ MOUNT DUA1: DBGXCHG            ! or the unit you attached it as
       $ SET DEFAULT DUA1:[000000]
       $ @DBG/OUTPUT=DBG.LOG

   It takes a few minutes, and prints nothing to the terminal but the
   programs' own output.

3. Pause simh (or detach the disk), then copy the results back:

       mkdir -p testdata/dbg/vax
       ./govax console < testdata/dbg/copyout.cmd

   `vax/` gets `dbg.log`; each image's `.exe`, `.map`, `.ani`, and
   session log `.dlg`; and DBGDIS's and DBGSUB's listings and objects.

If a session's `.dlg` is missing or empty, the debugger didn't take its
commands from `DBG$INPUT`. Then run that one by hand: `RUN/DEBUG IMAGE`,
and at `DBG>` type `@RUN.DBG` after making `RUN.DBG` as `DBG.COM`'s
`DEBUG` subroutine does (`SET LOG`, `SET OUTPUT LOG,VERIFY`,
`@probe.DBG`, `EXIT`).

## What came back (`vax/`)

Not yet run.
