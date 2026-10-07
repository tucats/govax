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
| `decode.go` | Turns the objects into `defined.txt` |
| `phase45-expected.txt` | The values govax uses until the run: the `ACC$` offsets from the manual, `MSG$_DELPROC` from STARLET.OLB, and the `PRC$` and `PQL$` values unconfirmed |

## The VAX run

1. Build the exchange volume, from the repository root:

       govax console < testdata/mp/defs/exchange.cmd

   This makes `testdata/disks/mp-defs.dsk` (RD53 size, label MPDEFS,
   gitignored).
2. Attach it to the simh VAX, mount it, set it as the default directory,
   and run:

       @DEFS/OUTPUT=DEFS.LOG

3. Copy the results back into `vax/` (names as VMS writes them, as for
   `testdata/mar/rms/vax`): every `.OBJ`, in the host variable-length
   record layout, every `.ANL`, and `DEFS.LOG`.

## After the run

    go run testdata/mp/defs/decode.go
    go run ./internal/vmsdef/gen -replace -values testdata/mp/defs/defined.txt

The first writes `defined.txt` from the objects; the second merges it into
`internal/vmsdef`'s `Symbols`, correcting any expected value the run
contradicts (`-drop NAME` removes an expected name VMS doesn't define).
Then `phase45-expected.txt`'s unconfirmed groups can go, and
`internal/bootdata/mkdefs` can build `$PRCDEF`, `$PQLDEF`, and `$ACCDEF`
for govax's own STARLET.MLB from `defined.txt`.
