# The Phase 33 oracle: RMS name blocks and XABs at run time

These fixtures are `docs/PHASE-33.md`'s subtask 1. govax's `$PARSE`,
`$SEARCH`, `$DISPLAY`, and the NAM and XAB handling in `$OPEN`, `$CREATE`,
and `$CLOSE` are written from the RMS Reference Manual, then checked
against what VMS 7.3's RMS does with these probe programs.

Everything here except `vax/` is written by `gen.go`:

    go run testdata/mar/rms3/gen.go

| Files | What they hold |
| ----- | -------------- |
| `parse.mar` | `$PARSE` cases: defaults, related files, wildcards, logical names and search lists, NOP options, and errors |
| `search.mar` | `$PARSE`, then `$SEARCH` until it fails, for wildcard and plain specifications |
| `open.mar` | `$OPEN` with a NAM, then `$CLOSE` |
| `xab.mar` | `$OPEN` and `$DISPLAY` with a chain of XABDAT, XABRDT, XABFHC, XABPRO, XABALL, and XABSUM, and bad chains |
| `namfid.mar` | `$OPEN` by name block (FAB$V_NAM): by file ID, and by directory ID and name |
| `create.mar` | `$CREATE` with a NAM and XABs, versions, CIF, and `$CLOSE` with XABRDT and XABPRO |
| `build.com` | Builds the test tree: `[TEST]`, `[TEST.SUB]`, `[TEST.EMPTY]`, `[OUT]`, and `[CRE]` |
| `run.com` | Assembles, links, and runs the probes |
| `exchange.cmd` | The govax console script that builds the exchange volume |

Each probe writes `[OUT]name.DMP`: variable-length records, each a tag
(`STAT`, `FAB_`, `NAM_`, `ESA_`, `RSA_`, or an XAB's), the case number,
the service just called, and the bytes. `gen.go`'s comment describes the
layout.

## The VAX run

1. Build the exchange volume, from the repository root:

       govax console < testdata/mar/rms3/exchange.cmd

   This makes `testdata/disks/rms3-exchange.dsk` (RD51 size, label
   RMSXCHG3, gitignored).
2. Attach it to the simh VAX, mount it, set its `[000000]` as the default
   directory, and run:

       @BUILD/OUTPUT=BUILD.LOG
       @RUN/OUTPUT=RUN.LOG

   `BUILD.COM` runs once. `RUN.COM` empties `[OUT]` and `[CRE]` first, so
   it can be run again.
3. Dismount the volume and copy the container back.

The run's container is `vax/rms3-vax.dsk.gz` (gzipped; 2026-10-01, VMS 7.3
on simh, node SIMVAX, device DUA1). It holds the tree, the dumps, both
logs, and the probes VMS built; the author audited it. `TestRMS3Oracle`
(`internal/console/rms3oracle_test.go`) runs each probe under govax on a
copy of it, so the file IDs, directory IDs, and dates are the ones VMS
saw, and compares the dumps byte for byte. The fields that differ for
reasons outside RMS's definition are masked there, each with its reason.

Nothing here makes a listing of a macro expansion: the probes are
assembled with `/NOLIST`. The logs hold DCL's commands, `DIRECTORY`'s
output, and MACRO's and LINK's messages; skim them before handing the
container back.
