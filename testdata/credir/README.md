# The Phase 34 oracle: CREATE/DIRECTORY

These fixtures are `docs/PHASE-34.md`'s subtask 1. govax's
`CREATE/DIRECTORY`, and ods2's directory creation under it, are checked
against what VMS 7.3 writes for the same commands.

| File | What it holds |
| ---- | ------------- |
| `credir.com` | The CREATE/DIRECTORY cases: defaults, an existing directory, `/OWNER_UIC` (a UIC and `PARENT`), `/VERSION_LIMIT`, `/PROTECTION` (full, partial, long names), `/ALLOCATION`, several levels at once, a list, relative directories, a logical name, names at the length and depth limits, and errors. Then `DIRECTORY` listings of the result. |
| `exchange.cmd` | The govax console script that builds the exchange volume |

## The VAX run

1. Build the exchange volume, from the repository root:

       govax console < testdata/credir/exchange.cmd

   This makes `testdata/disks/credir-exchange.dsk` (RD51 size, label
   CREDIR, gitignored).
2. Attach it to the simh VAX, mount it, set its `[000000]` as the default
   directory, and, logged in as SYSTEM (UIC `[1,4]`), run:

       @CREDIR/OUTPUT=CREDIR.LOG

   Some of its commands fail on purpose. It runs once: run it again only
   on a freshly built volume.
3. Dismount the volume and copy the container back as
   `vax/credir-vax.dsk.gz` (gzipped). No copy of the volume from before
   the run is needed: the test builds its own.

The run's container is `vax/credir-vax.dsk.gz` (2026-10-01, VMS 7.3 on
simh, device DUA1, as SYSTEM). It holds the directories VMS made, `[ALLOC]`'s
`FIRST.DAT`, and `CREDIR.LOG`. `TestCreateDirectoryOracle`
(`internal/console/credir_oracle_test.go`) replays `credir.com`'s
commands under govax on a freshly built exchange volume (a COPY from a
host file stands in for `CREATE [ALLOC]FIRST.DAT`), and compares each
directory's file header with VMS's, and each CREATE/DIRECTORY's messages
with the log's. File IDs, LBNs, and dates aren't compared, since they
depend on the order of allocation and the time of the run; the one
masked field is listed, with its reason, in the test.
