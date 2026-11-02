# Phase 85's debugger probe

How the VMS debugger (VMS 7.1, on simh) treats names with periods, and what it makes
of the MACRO the FORTRAN compiler writes, for govax's debugger to be checked against
(`docs/PHASE-85 - FORTRAN compiler usability.md`, subtask 2). There's no VAX FORTRAN
on the VAX, but none is needed: VMS's MACRO and LINK build the compiler's `.MAR` file.

VMS's session logs leave out a command that fails, so `*.dbg` names each case in a
comment (`!>`) just before it.

## The files

| File | What it is |
|---|---|
| `dbg85m.mar`, `dbg85m.dbg` | Names with a period in each place, a name whose parts exist and which doesn't (`C.D`), expressions around them (`A+.B`, `.A.B`, `A.B:C`), paths, DEPOSIT, breaks, a watch, and the same under `SET LANGUAGE FORTRAN` |
| `dbg85.for`, `dbg85.dbg` | A FORTRAN program with no I/O statement, so no run-time library. The session asks about each kind of variable (survey's D3), dummy arguments (D2), `EXAMINE .PC` (D6), EXAMINE/SOURCE, `$LOCAL`, SHOW MODULE's parameter (D7), subscripts, and bare names (D4) |
| `dbg85s.mar`, `dbg85s.opt`, `dbg85c.mar`, `dbg85c.opt`, `dbg85c.dbg` | A program and a shareable image: what SHOW MODULE says of its modules (D8) |
| `dbg85n.mar`, `dbg85n.dbg` | A name starting with a period, alone since it may not assemble |
| `dbg85.com` | Builds everything and runs each session, logging to `IMAGE.DLG` |
| `exchange.cmd`, `copyout.cmd` | govax's scripts: the volume, and the results back |

`exchange.cmd` compiles `DBG85.FOR` onto the volume twice: `DBG85.MAR` (`/NOOPTIMIZE`), which
the sessions use, and `DBG85O.MAR` (`/OPTIMIZE`), which VMS's MACRO only assembles. It also
builds govax's own images, GVDBG85 and GVDBG85M, for VMS's debugger to read.

## Running it

1. Build the volume, from the repository root (it makes
   `testdata/disks/dbg85-exchange.dsk`, label DBG85, gitignored):

       go build -o govax ./cmd/govax
       ./govax console < testdata/dbg85/exchange.cmd

2. On VMS, with the disk attached to simh (no privileges needed):

       $ MOUNT DUA1: DBG85            ! or the unit you attached it as
       $ SET DEFAULT DUA1:[000000]
       $ @DBG85/OUTPUT=DBG85.LOG

3. Pause simh or detach the disk, then copy the results back:

       mkdir -p testdata/dbg85/vax
       ./govax console < testdata/dbg85/copyout.cmd

If `DBG85N` didn't assemble, `copyout.cmd` reports that `DBG85N.DLG` isn't there, which is fine.

The same procedure ran on govax (a scratch copy, 2026-11-02) from end to end, to find mistakes in
the probe.

## What came back (`vax/`)

From the author's run on VMS 7.1, 2-NOV-2026, by `copyout.cmd`. MACRO's one warning is the probe's
own: `AB.CDEF...01234` is 32 characters, which is a useful accident (ILLSYMLEN). The phase doc's
"What the probe settled" has the answers. `TestDebuggerSessionOracle` replays `dbg85m.dlg` and
`dbg85.dlg`, and `TestDebugRecords` holds `dbg85m.obj` and `dbg85.obj` to govax's MACRO.
`gvdbg85*.dlg` are VMS's debugger on govax's images (made before the MACRO fixes the run led
to), whose source lines it couldn't read, the volume having been DUA2 when they were built.
