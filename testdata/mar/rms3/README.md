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
3. Dismount the volume and copy the container back as
   `testdata/disks/rms3-vax.dsk`. The tree, the dumps, and both logs are
   on it; govax's test runs the probes against a copy of it, so the file
   IDs, directory IDs, and dates are the ones VMS saw.

Nothing here makes a listing of a macro expansion: the probes are
assembled with `/NOLIST`. The logs hold DCL's commands, `DIRECTORY`'s
output, and MACRO's and LINK's messages; skim them before handing the
container back.
