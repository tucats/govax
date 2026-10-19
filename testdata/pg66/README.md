# The Phase 66 probe: image activation

These fixtures are `docs/PHASE-66 - mapped image activation.md`'s
subtask 7: the VMS run that settles what the phase's survey and progress
log left unconfirmed (the doc's "The probe's cases" lists the
questions). One MACRO program, linked twice, reads the process's paging
counts with `$GETJPIW` around each case; a command procedure asks
SYSGEN for the parameters, takes the counts before and after each RUN,
and runs part D in a subprocess.

| File | What it holds |
| ---- | ------------- |
| `pg66.mar` | The program (below), by hand |
| `pg66c.opt` | PG66C.EXE's options: BIG in a cluster of its own, cluster factor 8 |
| `pg66.com` | The procedure: SYSGEN's parameters, SHOW WORKING_SET, the build, the runs, and each part A run's counts across RUN |
| `exchange.cmd` | The govax console script that builds the exchange volume |
| `copyout.cmd` | The govax console script that copies the results back |
| `vax/` | VMS's output, after the run |

## The image

| Psect | Pages | What it is |
| ----- | ----- | ---------- |
| `CODE` | about 7 | The program: read-only, executable |
| `BIG` | 100 | Read-only data, page n's first longword n+1 |
| `DATA` | 3 | Initialized writable data: a copy-on-reference section |
| `ZERO` | 20 | Uninitialized writable data: a demand-zero section |
| `$RMSNAM` | 1 | The `$FAB` macros' file names (writable) |
| fixup | 1 | LINK's, for the G^ calls of LIB$INIT_TIMER and LIB$STAT_TIMER |

PG66.EXE is linked /NOTRACEBACK, so START is the image's first
instruction, with every section's cluster factor the default
(PFCDEFAULT). PG66C.EXE is the same with BIG in a cluster of its own,
`CLUSTER=BIGCL,,8`, so BIG's image section has a cluster factor of 8.
govax's LINK has no CLUSTER= option, so govax runs VMS's images (`vax/`)
once they're back; until then its dry run links PG66.EXE itself.

## How the program works

It uses only system services and RMS, but for LIB$INIT_TIMER and
LIB$STAT_TIMER, and prints with `$FAOL` and `$PUT` to SYS$OUTPUT. A
snapshot (`SNAP`) is one `$GETJPIW` of PAGEFLTS, PAGFILCNT, GPGCNT,
PPGCNT, DIRIO, and WSSIZE; a `REPORT` line is the changes between two:

    id what: flts= charged= gpg= ppg= dirio=

(`charged` is PAGFILCNT gone down by). Statuses are in hex. A page's
contents are checked as it's read (`bad=` counts the pages that don't
hold what they should). The logical name PG66_PART picks the part:

| Part | Lines | What they ask |
| ---- | ----- | ------------- |
| A | A1 | The counts at the first instruction (its first action is a snapshot) against the ones PG66.COM took before RUN (PG66_FLTS, PG66_CNT, PG66_PPG): activation's faults, and its charges (the writable and demand-zero pages, not the read-only ones?) |
| | A2 | LIB$INIT_TIMER, the first call through the fixup section; then the program's start |
| | A3 | PROBEW from user mode of a demand-zero page nothing has touched: does PROBE fault the page in? (docs/DEVIATIONS.md: govax's does) |
| | A4 | CODE and DATA locked in the working set |
| | A5, A6 | BIG read in order, a longword a page: faults (one per cluster: PFCDEFAULT for PG66.EXE, 8 for PG66C.EXE), PPGCNT, DIRIO; then again |
| | A7 | ZERO read (demand-zero faults; zero?), then written |
| | A8 | LIB$STAT_TIMER's fault count against `$GETJPI`'s over the same span |
| | A9 | The image's own file (PG66_IMAGE) opened while the image runs: to read, others reading; to read, no sharing; to write (RMS$_FLK?) |
| | A10 | The protection of every page from `^X200` to the image's end, a letter a page (W writable from user mode, R readable only, - neither), last so the probes can't change the counts above: the fixup page UREW? |
| | A11 | The whole image's faults and charges |
| W | W1-W3 | BIG read three times at a small working set (the floor plus 10): the second and third passes' faults soft (the free list) or hard (the image file) |
| D | D1-D3 | Run from PG66D.EXE (a COPY of PG66.EXE) in the subprocess PG66DEL: makes PG66D.RDY, waits ten seconds while PG66.COM deletes PG66D.EXE, then reads BIG, which it hasn't touched (does it still read?), and makes PG66D.END |

PG66.COM runs part A from PG66.EXE twice in a row (R1, R2: the counts
across RUN, and that the charges all come back), then from PG66C.EXE
(R3), then W and D. After each part A run it writes the counts' changes
across the RUN: `id image around RUN: status= flts= charged= ppg=`.
Between the procedure's `F$GETJPI` and the RUN, DCL's own work (the
DEFINEs) is in the counts too.

## The VAX run

1. Build the exchange volume, from the repository root:

       govax console < testdata/pg66/exchange.cmd

   This makes `testdata/disks/pg66-exchange.dsk` (RD51 size, label PG66,
   gitignored).
2. Attach it to the simh VAX, log in as SYSTEM, and:

       MOUNT DUA1: PG66
       SET DEFAULT DUA1:[000000]
       @PG66/OUTPUT=PG66.LOG

   Part D waits for its subprocess (at most a minute for each marker
   file); the whole run takes well under a minute.
3. Copy the results back, from the repository root:

       govax console < testdata/pg66/copyout.cmd

   The procedure's log, the runs' outputs, MACRO's and the two LINKs'
   messages and maps, the object, and both images go to `vax/`. MACRO's
   messages (`pg66_build.log`) may quote STARLET macro expansions: the
   clean-room hook refuses them until the author has audited them.

## govax's dry run (2026-10-18)

`TestPG66DryRun` (`internal/console/pg66_test.go`) builds PG66.EXE with
govax's MACRO and LINK/NOTRACEBACK on an RD54 volume (paging on, a
20000-block page file on the volume, PFCDEFAULT 16) and runs parts A
(twice), W, and D as PG66.COM does. What govax answers, for comparison
with VMS's:

- **A1:** 3 faults by the first instruction; 24 pages charged in the
  first run (DATA, ZERO, `$RMSNAM`), 25 in the second, where the first
  run's exit gave back one page more than its activation took (R1's
  `charged=-1`): a page charged before the first RUN, not chased. A11's
  whole image charges nothing more.
- **A3:** PROBEW faults the page in (1 fault): the deviation the case
  asks about.
- **A5:** BIG in 7 faults (clusters of 16), PPGCNT 100, DIRIO 0 (an
  image page's read isn't direct I/O); A6 none.
- **A7:** 19 demand-zero faults (A3 took the 20th), every page zero.
- **A8:** LIB$STAT_TIMER counts the faults `$GETJPI` does.
- **A9:** the file opens to be read with others reading (RMS$_NORMAL);
  without sharing, or to be written, RMS$_FLK: the activator holds it
  read-shared (Phase 66, question 2).
- **A10:** BIG 100 R, DATA and ZERO 23 W, CODE R, `$RMSNAM` W, the
  fixup page R (UREW).
- **R1, R2:** the charges come back at the image's end.
- **W:** at the floor plus 10, BIG's first pass in 75 faults (clusters
  cut short by the working set's free entries), the second and third 100
  soft faults each, DIRIO 0.
- **D:** DELETE of the running image's file succeeds and the name is
  gone at once; the image then reads all of BIG (7 faults, every page
  right) from the file it still holds.
- **PG66.COM on govax:** it runs through but for what govax lacks
  (F$GETSYI's HW_NAME, SYSGEN, LINK's /FULL and CLUSTER=, so no
  PG66C.EXE); govax's MACRO and LINK, which are images on VMS, leave
  the DEFINE/USER_MODE names before them in place (docs/DEVIATIONS.md),
  so the next DEFINE says `%DCL-I-SUPERSEDE`.
- **Found and fixed:** COPY from one volume file to another reframed it
  as Stream_LF text, unless /BINARY or an object, so the copied
  PG66D.EXE (fixed 512-byte records) had its bytes moved and didn't run
  (a reserved opcode at its first instruction). It now copies the file
  as it is, as VMS's COPY keeps its organization and record format
  (`TestSessionCopy_volumeToVolumeKeepsFormat`).
