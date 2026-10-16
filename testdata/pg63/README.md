# The Phase 63 probe: working sets

These fixtures are `docs/PHASE-63 - working sets.md`'s subtask 10: the
VMS run that settles what the phase's survey and progress log left
unconfirmed (the doc's "The probe's cases" lists the questions). One
MACRO program reads the process's working set counts with `$GETJPIW`
around each case; a command procedure asks DCL's SET and SHOW
WORKING_SET, SYSGEN, and a subprocess; and the procedure also runs round
8 of the system service macro probes (`$PURGWS`, `testdata/mp/macros`).

| File | What it holds |
| ---- | ------------- |
| `pg63.mar` | The program (below), by hand |
| `pg63.com` | The procedure: SHOW WORKING_SET, SHOW MEMORY/PHYSICAL_PAGES, SYSGEN's paging parameters, SET WORKING_SET's cases, the subprocess, the program's two runs, and the macro round |
| `pg63s.com` | The subprocess's part: its own values, and a /QUOTA beyond its authorized one |
| `pg63x.com` | One SET WORKING_SET near or below the floor, in a subprocess spawned for it alone (round 2) |
| `exchange.cmd` | The govax console script that builds the exchange volume (with `../mp/macros/r8_purgws.mar` and `macros8.com`) |
| `copyout.cmd` | The govax console script that copies the results back |
| `vax/` | VMS's output, after the run |

## How the program works

It uses only system services and RMS, so no shareable image's pages are
in VMS's working set and not in govax's, and prints with `$FAOL` and
`$PUT` to SYS$OUTPUT. A snapshot (`SNAP`) is one `$GETJPIW` of
PAGEFLTS, PPGCNT, GPGCNT, WSSIZE, WSPEAK, DFWSCNT, WSQUOTA, WSEXTENT,
WSAUTH, and WSAUTHEXT; a case's faults are the difference of two. The
array is 700 pages from `$EXPREG`, a sweep (`SWEEP first,count`) writes
a longword into each of a run of its pages, and `SETLIM n` makes the
limit n with `$ADJWSL`. Since round 2, the limits are the **floor**
(what `$ADJWSL` -100000 leaves, B4: MINWSCNT on govax, and on VMS
MINWSCNT and the list's permanent part, about 94 pages) plus a margin,
`SETREL k`, so that VMS's patterns have the dynamic room govax's have. Statuses are in hex; addresses appear only as
differences from the address asked for, or as -1.

The logical name PG63_PART picks the part. **Part A** (`PG63A.OUT`):

| Lines | What they ask |
| ----- | ------------- |
| A1, A2 | The counts at the image's first instruction (the image startup purge, finding 11), and the quotas |
| A3 | `$GETSYIW` of MINWSCNT, WSMAX, GROWLIM, BORROWLIM, FREELIM, FREEGOAL, MPW_HILIMIT, MPW_LOLIMIT, PFCDEFAULT, TBSKIPWSL, one at a time |
| B1-B5 | `$ADJWSL`: 0, `pagcnt` omitted, +100000 (WSEXTENT? WSMAX?), -100000 (MINWSCNT? a higher floor?) and PPGCNT straight after, `wsetlm` in the code (ACCVIO; is the limit changed anyway?) |
| B6 | A shrink from floor+180 to floor+40 with 150 pages in use: PPGCNT straight after |
| B7 | A grow from floor+40 to floor+180, then two sweeps of 150: is the room used at once (no faults on the second)? |
| C | At floor+80, after a purge of everything, N pages (30, 60, 75, 90, 120, 300) swept three times: the faults of each sweep, PPGCNT, GPGCNT, WSPEAK |
| D1 | `$PURGWS` of all of P0 and P1: PPGCNT just after (the residue) |
| D2-D4 | One page; a reversed range (are its pages purged?); a range with a page locked in the working set (does it stay?) |
| D5-D7 | A range in S0 and one from P1 into S0 (SS$_NOPRIV, VSI's list?); `inadr` 0 |
| E1-E6 | `$LKWSET`/`$ULWSET` statuses: lock, lock again, a partly locked range, unlock all locked, none locked, mixed; `retadr` |
| E7 | At floor+80 and floor+130, after a purge, pages locked one at a time until a lock fails: how many, and the status (SS$_LKWSETFUL's threshold, finding 6) |
| E9 | One `$LKWSET` of 200 pages at floor+80: the status and `retadr` |
| E10, E11 | A writable page-file global section page: `$LKWSET` (SS$_NOPRIV?), `$LCKPAG`, `$ULKPAG`; the same section mapped read-only: `$LKWSET` |
| E12-E16 | A system page, a missing page (`retadr` -1?), `retadr` in the code, `acmode` kernel, `inadr` 0 |
| F1-F8 | `$LCKPAG`/`$ULKPAG` with PSWAPM, then without (`$SETPRV`): does `$ULKPAG` need it? which comes first, the privilege or `inadr`? Does `$LCKPAG` lock in the working set too (`$ULWSET` after it)? |
| G1 | What part A leaves: a limit of floor+230, page 0 locked in the working set and page 1 in memory, WSPEAK and PAGEFLTS at its end |

**Part B** (`PG63B.OUT`), the second image: R1 is its first snapshot
(is WSSIZE back at DFWSCNT? is WSPEAK the process's or the image's? how
many faults did activation take, against the DCL line before the RUN?),
R2 unlocks part A's pages (gone with its P0), R3 its end.

`pg63.com`'s `ws` lines (F$GETJPI's DFWSCNT, WSSIZE, quotas, PPGCNT,
GPGCNT, WSPEAK, PAGEFLTS) follow every SET WORKING_SET and each run, so
the log shows what DCL's own commands and image exit change (does WSSIZE
follow SET WORKING_SET/LIMIT at DCL level? is PPGCNT back to DCL's own
pages after an image exits?).

## The VAX run

1. Build the exchange volume, from the repository root:

       go run testdata/mp/macros/gen.go
       govax console < testdata/pg63/exchange.cmd

   This makes `testdata/disks/pg63-exchange.dsk` (RD51 size, label PG63,
   gitignored).
2. Attach it to the simh VAX, log in as SYSTEM, mount it, set its
   `[000000]` as the default directory, and run:

       @PG63/OUTPUT=PG63.LOG

   It takes a few minutes: the patterns fault a few thousand times. If
   the process is ever left unable to run images (round 1's trouble),
   logging it out should be enough; the other processes aren't affected.
3. Copy the results back, from the repository root:

       govax console < testdata/pg63/copyout.cmd

   The procedure's log, the program's two outputs, and the subprocess's
   log go to `vax/`; round 8's object, analysis, and log to
   `../mp/macros/vax/`. MACRO's own messages (the build log and round
   8's log) may quote lines of STARLET macro expansions: the clean-room
   hook refuses them until the author has audited them.

## Round 1 (2026-10-16)

OpenVMS V7.1 on simh's VAX 8600 (`vax/round1/`). The procedure got as
far as SET WORKING_SET/QUOTA=5, which set the quota to 94 (with
`%SET-I-NOMSG, Message number 007781C3`); from then on no image would
run in the process (LIB-F-INSVIRMEM, and SYSTEM-F-INSFWSL activating
LIBRTL), so MACRO, LINK, and the program never ran, and the VAX was
rebooted. What it settled before then: SHOW WORKING_SET's and NEWLIMS's
layouts, the bounds of /LIMIT above the quota and the authorized values,
a /LIMIT of 5 giving 94 (the floor) with WSSIZE following at DCL level,
no message within bounds without /LOG, and a subprocess's extent being
its creator's plus 4 too. Round 2 keeps the process able to run images:
the cases near the floor run in subprocesses of their own (PG63X.COM),
and the program's limits are relative to the floor.

## govax's dry run

Before the VAX run, govax ran it all (2026-10-16, paging on). What it
already shows, for comparison with VMS's: govax's DCL has no SYSGEN,
SHOW MEMORY/PHYSICAL_PAGES, or F$GETSYI's HW_NAME; and govax's image
rundown doesn't delete P0, so after part A exits DCL still sees its
array's pages in the working set (PPGCNT 210), and an image run after
another of the same layout takes few faults or none: its pages are still
there.
