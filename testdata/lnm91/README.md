# The Phase 91 probe: concealed and rooted logical names

These fixtures are `docs/PHASE-91 - concealed logical names.md`'s
subtask 2. They ask VMS what RMS and DCL do with a concealed logical name
and with a rooted-device logical name (`DEV:[ROOT.]`), where the manuals
stop or disagree.

| File | What it holds |
| ---- | ------------- |
| `lnm91.mar` | The program, by hand: takes one case per data line and reports what RMS did |
| `gen.go` | Writes the next three files, `go run testdata/lnm91/gen.go` from the repository root |
| `lnm91.com` | The whole probe: makes a tree, defines the names, runs the cases, then DCL's commands |
| `exchange.cmd` | The govax console script that builds the exchange volume |
| `copyout.cmd` | The govax console script that copies the results into `vax/` |
| `lnm91b.com`, `exchangeb.cmd`, `copyoutb.cmd` | Round 2's, also written by `gen.go` |
| `vax/lnm91.log` | VMS's log |
| `vax/lnm91.exe` | VMS's image, for a later oracle to run on govax |

## The program's cases

A data line is `OF NAME|DEFAULT|RELATED`. `O` is the operation: `P`
`$PARSE`, `S` `$PARSE` and `$SEARCH` until it stops, `O` `$OPEN`, `C`
`$CREATE`, `E` `$ERASE`, `R` `$RENAME` from NAME to the second field, `D`
`$ASSIGN` and `$GETDVIW`, `F` `$OPEN` and `LIB$FID_TO_NAME`, and `J` this
image's `JPI$_IMAGNAME`. `F` is `-`, or `N` for `NAM$V_NOCONCEAL`, or `S`
for `NAM$V_SYNCHK`.

Each case prints its own line after `==`, then the status and STV, the
expanded and resultant strings with their lengths, FNB (and its
`CNCL_DEV`, `ROOT_DIR`, and `DIR_LVLS`), each component's length, DVI,
DID, and FID.

The cases are data lines, which DCL doesn't substitute, so they name the
device through the logical name `LDEV` instead of a symbol.

## The tree

Everything is made on the exchange volume. `BASE` is
`[LNM91.ROOT1.]`. Top-level `[SUB]`, `[JONES]`, and `[OTHER]` are decoys,
each with an `A.DAT`, so a name that loses its root finds a different
file or directory rather than simply failing. `[D1.D2. ... .D8]` is for
the depth cases.

## The VAX run

Only process logical names are defined, and they are deassigned at the
end. The VAX's system tables and system disk aren't changed. The
procedure doesn't wait for input.

1. From the repository root, on the host:

       go run ./cmd/govax --profile tbprobe --system-file none console < testdata/lnm91/exchange.cmd

   This makes `testdata/disks/lnm91-exchange.dsk` (RD51, label LNM91,
   gitignored). If that file is left from an earlier run, delete it first.

2. Attach it to the simh VAX on a free disk unit, for example at the
   `sim>` prompt:

       attach rq1 testdata/disks/lnm91-exchange.dsk

   (use the full path if simh wasn't started in the repository root).

3. On the VAX, using that unit, from an account that can write the volume
   (SYSTEM is simplest):

       $ MOUNT DUA1: LNM91
       $ SET DEFAULT DUA1:[000000]
       $ @LNM91/OUTPUT=LNM91.LOG

   It takes a few minutes and prints nothing at the terminal. The last
   line of the log is `LNM91 FINISHED`, which `TYPE LNM91.LOG` will show.

4. If MACRO or LINK failed, the procedure stops early: `LNM91.MLG` and
   `LNM91.LLG` on the volume say why.

5. Dismount it and detach it from simh:

       $ SET DEFAULT SYS$LOGIN
       $ DISMOUNT DUA1:

       sim> detach rq1

6. Back on the host, from the repository root:

       mkdir -p testdata/lnm91/vax
       go run ./cmd/govax --profile tbprobe --system-file none console < testdata/lnm91/copyout.cmd

## Round 2

`lnm91b.com` runs the same program on what round 1 left open: plain and
concealed names whose equivalence has a plain directory, `SYS$DISK` with a
directory, a tree 15 levels deep through a 7-level root, and angle
brackets. Its steps are round 1's with these names:

1. `go run ./cmd/govax --profile tbprobe --system-file none console < testdata/lnm91/exchangeb.cmd`
   (makes `testdata/disks/lnm91b-exchange.dsk`, label LNM91B; delete an
   old one first).
2. `sim> attach rq1 testdata/disks/lnm91b-exchange.dsk`
3. On the VAX:

       $ MOUNT DUA1: LNM91B
       $ SET DEFAULT DUA1:[000000]
       $ @LNM91B/OUTPUT=LNM91B.LOG

   The log's last line is `LNM91B FINISHED`.
4. `$ SET DEFAULT SYS$LOGIN`, `$ DISMOUNT DUA1:`, `sim> detach rq1`
5. `go run ./cmd/govax --profile tbprobe --system-file none console < testdata/lnm91/copyoutb.cmd`
   (writes `vax/lnm91b.log`).

## What the run is expected to show

Errors in the log are part of the probe: many cases are there to see what
VMS refuses. A few DCL lines may also fail for reasons of the VAX's own
(a directory's delete protection, for instance), which `SET NOON` lets
the procedure go on from.

## On govax

`TestLnm91ParseOracle` (`internal/console/lnm91oracle_test.go`) replays the
`$PARSE` cases of both logs with `vax/lnm91.exe`, on a volume built by the
log's own commands, and matches VMS's report line for line.
