# The Phase 64 probe: the page file

These fixtures are `docs/PHASE-64 - page file.md`'s subtask 9: the VMS
run that settles what the phase's survey and progress log left
unconfirmed (the doc's "The probe's cases" lists the questions). One
MACRO program reads the process's page file quota and the page file's
use with `$GETJPIW` and `$GETSYIW` around each case; a second,
`PG64V`, is the LIB$GET_VM case; and a command procedure asks SYSGEN,
SHOW MEMORY/FILES, DIRECTORY/FULL of the page file, and F$GETSYI and
F$GETJPI between the runs, and makes the subprocesses.

| File | What it holds |
| ---- | ------------- |
| `pg64.mar` | The program (below), by hand |
| `pg64v.mar` | LIB$GET_VM's charges, and its status past the quota |
| `pg64.com` | The procedure: SYSGEN's parameters, SHOW MEMORY/FILES and /FILES/FULL, SHOW MEMORY, DIRECTORY/FULL of `SYS$SYSTEM:PAGEFILE.SYS`, SHOW PROCESS/QUOTA, the programs' runs, and `pf` lines (F$GETSYI's PAGEFILE_FREE, PAGEFILE_PAGE, SWAPFILE_FREE, SWAPFILE_PAGE; F$GETJPI's PGFLQUOTA, PAGFILCNT, PAGFILLOC, DIRIO, PAGEFLTS) around each |
| `pg64l.com` | A spawned subprocess's part: part T, then LOGOUT/FULL |
| `exchange.cmd` | The govax console script that builds the exchange volume |
| `copyout.cmd` | The govax console script that copies the results back |
| `pg64x.com`, `pg64m.com` | Round 2: the writer's stopping point (below) |
| `exchange2.cmd`, `copyout2.cmd` | Round 2's scripts: the new files onto the existing volume, and its results back to `vax/round2/` |
| `vax/` | VMS's output, after the run |

## How the program works

It uses only system services and RMS (LIBRTL's writable pages would be
charged to the quota on VMS and not on govax), and prints with `$FAOL`
and `$PUT` to SYS$OUTPUT. A snapshot (`SNAP`) is one `$GETJPIW` of
PAGEFLTS, PAGFILCNT, PGFLQUOTA, PAGFILLOC, DIRIO, PPGCNT, WSSIZE, and
CURPRIV, and one `$GETSYIW` of PAGEFILE_FREE and PAGEFILE_PAGE. A case's
charge is the difference of two PAGFILCNTs (negative for a refund), and
the page file's use the difference of two PAGEFILE_FREEs. Counts that
depend on the account's quota or the VAX's configuration appear only as
differences: from PAGFILCNT (`pages=pagfilcnt-3`), or from the pages a
case made (`flts=pages+3`). Statuses are in hex.

The logical name PG64_PART picks the part:

| Part | Lines | What they ask |
| ---- | ----- | ------------- |
| Q | Q1 | PGFLQUOTA, PAGFILCNT, and PAGFILLOC at the first instruction (what activation and DCL charged); whether EXQUOTA is held |
| | Q2 | `$EXPREG` of 100 P0 pages: charged at creation? at the first touch? `$DELTVA` of each half: refunded? |
| | Q3 | Pages made and deleted untouched |
| | Q4 | `$CRETVA` of 100 pages at P0's end; touched; `$CRETVA` over them (charged again? are they zero?); deleted |
| | Q5 | `$EXPREG` of 100 P1 pages, touched, deleted |
| | Q10 | The quota filled with untouched pages (P1, then P0, 1000 at a time) until PAGFILCNT is 200, or the address space ends first |
| | Q11 | A request for PAGFILCNT + 10 pages: `$EXPREG` of P0, `$CRETVA`, `$EXPREG` of P1. The status (EXQUOTA? EXPGFLQUOTA?), the pages made (partly? none?), retadr, and the refund after `$DELTVA` |
| | Q12 | The same `$EXPREG`, its pages touched: any further charge? |
| | Q13 | `$EXPREG` of exactly PAGFILCNT pages, then one more |
| C | C1, C2 | Part Q again in a detached process (`$CREPRC`, PRCNAM PG64DET) with a PGFLQUOTA of 2048 (raised to PQL_MPGFLQUOTA if that's more), with this process's privileges and then without EXQUOTA; its lines are `PG64Q2.OUT` and `PG64Q3.OUT`. The image's and output files' names are PG64_IMAGE, PG64_OUT2, and PG64_OUT3, with the device's full name, since a detached process sees only the system's logical names |
| W | W1 | MPW_HILIMIT, MPW_LOLIMIT, MPW_WRTCLUSTER; pages = MPW_HILIMIT + 1000 (less if the quota is short), at a working set limit of the floor plus 80 |
| | W2 | `$EXPREG` of them: charged, page file used (nothing yet?) |
| | W3 | The first sweep writes each page: faults, DIRIO (does page file I/O count?), charges (none at writing?), and the page file's use after a 3-second wait: did the writer write, and how much? |
| | W4 | The second sweep reads them back (`bad` counts pages without their contents): faults, DIRIO, the page file's use (blocks freed by reading back?) |
| | W5 | The third writes them again (blocks freed and taken again?), then a read |
| | W6 | `$DELTVA`: the refund, and are the blocks back at once, or after a wait? |
| S | S1-S3 | A subprocess (PG64SUB, spawned waiting) and this process: is PAGFILCNT the job's, the same in both, moving when this process charges? |
| T | T1 | 300 pages touched in a spawned subprocess, before its LOGOUT/FULL ("Peak page file size") |

`PG64V`: LIB$GET_VM of 100 pages, then 400 (what the heap charges as it
grows), then of PAGFILCNT + 10 pages (the status: LIB$_INSVIRMEM?).

The `pf` lines in the log show what each image's exit gave back
(PAGFILCNT back to where it was? the page file's blocks free again?), and
SHOW MEMORY/FILES before and after the writer shows VMS's VAX layout.

## The VAX run

1. Build the exchange volume, from the repository root:

       govax console < testdata/pg64/exchange.cmd

   This makes `testdata/disks/pg64-exchange.dsk` (RD51 size, label PG64,
   gitignored).
2. Attach it to the simh VAX, log in as SYSTEM, and mount it **/SYSTEM**
   (part C's detached process is a job of its own, and can't open files
   on a volume mounted for SYSTEM's job alone):

       MOUNT/SYSTEM DUA1: PG64
       SET DEFAULT DUA1:[000000]
       @PG64/OUTPUT=PG64.LOG

   It takes a few minutes: part W dirties a few thousand pages three
   times, each detached run may take up to two minutes, and part S waits
   for its subprocess.
3. Copy the results back, from the repository root:

       govax console < testdata/pg64/copyout.cmd

   The procedure's log, the programs' outputs, and the subprocess's log
   go to `vax/`. MACRO's and LINK's messages (`pg64_build.log`) may quote
   lines of STARLET macro expansions: the clean-room hook refuses them
   until the author has audited them.

## govax's dry run (2026-10-17)

Before the VAX run, govax ran it all on an RD54 volume, paging on, with
a 20000-block volume page file and MPW_HILIMIT 1500 (the dry run's own
test, not committed). Every part ran. What it shows, for comparison with
VMS's:

- **What govax's DCL lacks:** F$GETSYI's HW_NAME, SYSGEN, SHOW
  MEMORY/FILES, the SYS$SYSTEM directory, SHOW PROCESS/QUOTA, RUN/DETACHED
  (so part C makes the detached process with `$CREPRC`), SET
  PROCESS/PRIVILEGE (so part C drops EXQUOTA through `$CREPRC`'s
  privileges), and a subprocess's LOGOUT/FULL.
- **The address space before the quota:** in SYSTEM's own process
  (PGFLQUOTA 40960), govax's P0 (16384 pages) and P1 run out before the
  quota: Q10 stops at SS$_VASFULL and Q11 to Q13 meet VASFULL, not the
  quota. In the detached process (PGFLQUOTA 16400, govax's
  PQL_MPGFLQUOTA), the fill reaches 200 pages and the requests past it
  return SS$_EXQUOTA with exactly the quota's pages made, with EXQUOTA
  and without (govax doesn't let EXQUOTA exceed the page file quota).
- **`$DELTVA` doesn't contract a region:** govax's `$EXPREG` keeps a
  high-water mark that `$DELTVA` at the region's end doesn't move back,
  so pages deleted at P0's end don't give their addresses back to the
  next `$EXPREG` (on VMS they do). An early ordering of part Q, with the
  requests past the quota before the fill, filled P0 this way; part Q now
  fills first. Phase 67 (the address space) is where this belongs.
- **P1 pages outlive their image:** after part Q, PAGFILCNT stays 2000
  pages lower, the P1 pages the fill made: govax's image rundown deletes
  P0 (Phase 63) but not P1 pages an image created. The `pf` lines show
  whether VMS gives them back.
- **The writer:** W3 wrote MPW_HILIMIT + 1 pages, W4's read-back freed
  only a few blocks and found every page's contents, W5's rewrite
  freed and rewrote blocks, W6's `$DELTVA` gave back every block at
  once; DIRIO doesn't count page file I/O; nothing is charged by writing.
- **A host-file SYS$OUTPUT for an image** (`DEFINE/USER_MODE SYS$OUTPUT
  X.OUT` with the default directory on the host) writes nowhere: the
  first dry run, on a host directory, lost the programs' output, so the
  dry run moved to a volume, where VMS's run is too. Not fixed here.
- **Found and fixed:** a device's full name with its node's prefix
  (`_GOVAX$DUA1:`, F$GETDVI's FULLDEVNAM, which part C's names use)
  named no mounted volume in RMS, so `$CREPRC` couldn't open the image;
  `rms.normalizeDeviceName` now drops a node's or allocation class's
  prefix.

## The VAX run (2026-10-17)

OpenVMS V7.1 on simh's VAX 8600 (`vax/`: the procedure's log, the
programs' outputs, the subprocess's log; MACRO and LINK wrote no
messages). The first attempt had the volume mounted without /SYSTEM, so
part C's detached process couldn't open its files; the second, kept
here, ran everything. The logs were audited by the author.
`TestPG64Oracle` (`internal/console/pg64_test.go`) runs the parts on
govax and matches VMS's lines but for `expectedPG64Differences`; the
phase doc's progress log lists what VMS answered and what govax
changed. The modified page writer's stopping point is still open: VMS
wrote far fewer pages than govax's writer does (W3 to W6).

## Round 2: where the writer stops

Round 1's part W showed VMS's writer leaving its modified list far above
MPW_LOLIMIT, but a program can't see the list's size. Round 2 runs part
X, part W's case with the time of day on each line (the first sweep in
chunks of 500 pages with the page file's use after each, a 24-second
rest sampled every 3 seconds, the read-back sweep in chunks, the
deletion), while a monitor subprocess (`pg64m.com`) writes the time and
SHOW MEMORY/PHYSICAL_PAGES once a second, whose last column is the
modified list's size. The two logs together show the list's size when
the writer starts and stops. `pg64x.com` also asks SYSGEN for the
writer's other parameters (MPW_WAITLIMIT, MPW_LOWAITLIMIT, MPW_THRESH,
MPW_IOLIMIT, MPW_PRIO, FREELIM, FREEGOAL).

1. With the volume dismounted on the VAX, add round 2's files to it,
   from the repository root:

       govax console < testdata/pg64/exchange2.cmd

2. On the VAX, as SYSTEM:

       MOUNT/SYSTEM DUA1: PG64
       SET DEFAULT DUA1:[000000]
       @PG64X/OUTPUT=PG64X.LOG

   It takes about two minutes.
3. Copy the results back, from the repository root:

       govax console < testdata/pg64/copyout2.cmd

govax's dry run of part X: the first sweep's chunks show nothing written
until the list passes MPW_HILIMIT (between 5500 and 6000 pages), then
5520 pages written at once, the list down to MPW_LOLIMIT; nothing more
during the rest or the read-back.
