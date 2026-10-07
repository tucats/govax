# Phase 45's definition probes

`docs/PHASE-45.md`, subtask 1: the values of the `$CREPRC` definitions
govax didn't have. The System Services Reference Manual names the
`$CREPRC` status flags (`PRC$M_*`) and quota codes (`PQL$_*`) but gives no
values, and govax has no clean-room source for them. So these probes ask
real VAX MACRO: each `def_*.mar` calls one `$xxxDEF` macro with `GLOBAL`,
so the object's global symbol directory lists every name the macro
defines, with its value. No listing is made, and nothing is read from
VMS's macro library.

| File | What it holds |
| ---- | ------------- |
| `def_acc.mar`, `def_msg.mar`, `def_pql.mar`, `def_prc.mar` | `$ACCDEF`, `$MSGDEF`, `$PQLDEF`, `$PRCDEF`, each with `GLOBAL` |
| `defs.com` | Assembles each with `/NOLIST` and analyzes its object |
| `exchange.cmd` | The govax console script that builds the exchange volume |
| `copyout.cmd` | The govax console script that copies the results into `vax/` |
| `vax/` | The VMS 7.3 run's objects, analyses, and log (2026-10-07) |
| `decode.go` | Turns the objects into `phase45-defined.txt` |
| `phase45-defined.txt` | Every name and value the four macros define |

## The VAX run

1. Build the exchange volume, from the repository root:

       govax console < testdata/mp/defs/exchange.cmd

   This makes `testdata/disks/mp-defs.dsk` (RD53 size, label MPDEFS,
   gitignored).
2. Attach it to the simh VAX, mount it, set it as the default directory,
   and run:

       @DEFS/OUTPUT=DEFS.LOG

3. Copy the results back into `vax/`:

       govax console < testdata/mp/defs/copyout.cmd

## Into govax's tables

    go run testdata/mp/defs/decode.go
    go run ./internal/vmsdef/gen -values testdata/mp/defs/phase45-defined.txt

The first writes `phase45-defined.txt` from the objects; the second merges
it into `internal/vmsdef`'s `Symbols`. The `MSG$_` values agree with
STARLET.OLB's (`TestSymbols_matchLibrarySymbols`), and the `ACC$` offsets
with the manual's, except that VMS 7.3 has `ACC$L_JOBID` at offset 12,
which VMS 5.0's manual lists as unused.

`internal/bootdata/mkdefs` can build `$PRCDEF`, `$PQLDEF`, and `$ACCDEF`
for govax's own macro library from `phase45-defined.txt`, when a MACRO
program needs them (docs/PHASE-45.md, subtask 13).
