# Phase 29's debugger-record fixtures

Real VAX MACRO's output with debugger (DBG) records, on VMS 7.3 (simh),
for docs/PHASE-29.md's subtask 12: FORTH, and a probe of sources
written to settle what FORTH leaves open.

## FORTH

Real VAX MACRO's and LINK's output for `testdata/mar/forth.mar` built with
debugger records.
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
| `forth.lis` | the listing: `../vax/forth.lis` but for its headings and the symbol table's `D` flags |
| `forth.dia` | `/DIAGNOSTICS`' file (no messages) |
| `forth.exe`, `forth.map` | the `LINK/DEBUG` image and its map |
| `forth.anl`, `forth.ani` | ANALYZE/OBJECT's and ANALYZE/IMAGE's reports |

The simh VAX's clock had drifted from real time, so the dates in these
files are VMS's own, not the host's. The listing's second heading line
gives the source file's revision date (06:15:15), which the object's
source-file DST record holds too.

## The probe (dst2)

Each source is small and settles one question; its comments say which.

| Source | What it settles |
|---|---|
| `dstsym.mar` | the symbol record each data directive gives a label (every `.BLKx`, lists, strings, floating), and assigned, global, relocatable, suppressed, external, local, and absolute symbols |
| `dstln1.mar` | line tables: lines before the first instruction, label-only and comment lines, data and padding in code, gaps over 127 and 255 bytes, a run of over 255 lines without code |
| `dstln2.mar` | two code psects and switching between them, code before any `.ENTRY`, code in a NOEXE psect, a routine not ending in RET |
| `dstln3.mar` | user macros of data before the code and of code in it, conditionals, and a repeat block |
| `dstln4.mar`, `dstln5.mar`, `dstln6.mar` | whether system macros (`$FABDEF`; `$FAB` before the code; `$OPEN` and `$CLOSE` in it) move the line numbers, as something before FORTH's code moves them by 512 |
| `dstdbg.mar`, `dstdis.mar` | `.ENABLE DEBUG` and `.DISABLE DEBUG` partway through |

`DST.COM` also makes `DSTVAR.MAR`, DSTLN1 with variable-length records,
for the source-file record's format fields. The system macros are called
with the listing's defaults, which show no expansion.

To run it:

1. `govax console < testdata/mar/dst/exchange.cmd` makes
   `testdata/disks/dst2-exchange.dsk` (label DST2XCHG) with the sources
   and `DST.COM`.
2. On VMS, mount it, make it the default directory, and
   `@DST/OUTPUT=DST.LOG`.
3. `govax console < testdata/mar/dst/copyout.cmd` copies each object,
   listing, and analysis, `DSTVAR.MAR`, and `DST.LOG` into `vax/`.
