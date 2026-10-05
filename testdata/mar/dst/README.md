# Phase 29's debugger-record fixtures

Real VAX MACRO's and LINK's output for `testdata/mar/forth.mar` built with
debugger records, on VMS 7.3 (simh), for docs/PHASE-29.md's subtask 12.
The source is the same file `testdata/mar/vax/forth.*` were built from
without `/DEBUG`.

The author ran, on the exchange volume `testdata/disks/dst1-exchange.dsk`
(local only):

    $ MACRO/DEBUG FORTH/DIAG/LIST
    $ LINK/DEBUG/MAP FORTH
    $ ANAL/OBJ FORTH.OBJ/OUT=FORTH.ANL
    $ ANAL/IMAGE FORTH.EXE/OUT=FORTH.ANI

and `vax/` holds the results:

| File | What it is |
|---|---|
| `forth.obj` | the object: traceback (TBT) and debugger (DBG) records |
| `forth.lis` | the listing, the same as `../vax/forth.lis` but for its headings |
| `forth.dia` | `/DIAGNOSTICS`' file (no messages) |
| `forth.exe`, `forth.map` | the `LINK/DEBUG` image and its map |
| `forth.anl`, `forth.ani` | ANALYZE/OBJECT's and ANALYZE/IMAGE's reports |

The simh VAX's clock had drifted from real time, so the dates in these
files are VMS's own, not the host's. The listing's second heading line
gives the source file's revision date (06:15:15), which the object's
source-file DST record holds too.
