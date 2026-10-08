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
| `def_acc.mar`, `def_msg.mar`, `def_pql.mar`, `def_prc.mar` | `$ACCDEF`, `$MSGDEF`, `$PQLDEF`, `$PRCDEF`, each with `GLOBAL` (Phase 45) |
| `def_sec.mar`, `def_lck.mar`, `def_lki.mar`, `def_psl.mar`, `def_dc.mar` | `$SECDEF`, `$LCKDEF`, `$LKIDEF`, `$PSLDEF`, `$DCDEF` (Phase 46: global sections, the lock services Phase 47 needs, access modes, device classes; not yet run) |
| `defs.com` | Assembles Phase 46's with `/NOLIST` and analyzes their objects |
| `exchange.cmd` | The govax console script that builds the exchange volume (Phase 46's) |
| `copyout.cmd` | The govax console script that copies the results into `vax/` |
| `vax/` | The VMS 7.3 run's objects, analyses, and log (`defs45.log`, 2026-10-07); Phase 46's log will be `defs46.log` |
| `decode.go` | Turns the objects into `defined.txt` |
| `defined.txt` | Every name and value the four macros define |

## The VAX run

Phase 46's probes run with the other end-of-Phase-46 runs, on one volume
(`../run46/README.md`). The steps below are for this directory alone.

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
    go run ./internal/vmsdef/gen -values testdata/mp/defs/defined.txt

The first writes `defined.txt` from the objects; the second merges
it into `internal/vmsdef`'s `Symbols`. The `MSG$_` values agree with
STARLET.OLB's (`TestSymbols_matchLibrarySymbols`), and the `ACC$` offsets
with the manual's, except that VMS 7.3 has `ACC$L_JOBID` at offset 12,
which VMS 5.0's manual lists as unused.

`decode.go` reads every `vax/def_*.obj`, so once Phase 46's objects are
back the same two commands add their names too. `$SECDEF`'s values are
already in `vmsdef.LibrarySymbols` (STARLET.OLB), which the probe will
confirm; `$LCKDEF`, `$LKIDEF`, and `$PSLDEF` are new to govax.

`internal/bootdata/mkdefs` can build `$PRCDEF`, `$PQLDEF`, and `$ACCDEF`
for govax's own macro library from `defined.txt`, when a MACRO
program needs them (docs/PHASE-45.md, subtask 13).
