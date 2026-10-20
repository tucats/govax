# The Phase 69 probe, round 2: VMS reads govax's save sets

Subtask 12 of `docs/PHASE-69 - backup save sets.md`. Round 1
(`testdata/backup69`) had VMS make the probe's tree and save sets, and
`TestBackup69Oracle` holds govax's run of the same `BACKUP69.COM` to
VMS's log. This round goes the other way: govax makes the save sets, and
VMS lists, restores, and compares them. It asks:

- Does VMS's BACKUP/LIST read each of govax's save sets with govax's
  summary (`Written on: _GOVAX$DUA1:`, CPU ID 0, V7.3) and round 1's
  file lines, `/FULL` included?
- Does VMS restore every file of them, the headers as round 1's restores
  had them (`VB69.LST` against round 1's `r1.lst`)? Are govax's data
  CRCs right (B69CRC.BCK and HOST.BCK restored and compared with `/CRC`
  on the input)?
- Does VMS's compare find govax's B69.BCK the same as the tree it was
  made from, and as VMS's own restore of it? The directory files there
  hold the file IDs of each restore's own files.
- Do govax's XOR blocks recover a damaged block? SSCOPY complements a
  data byte of block 2 in B69.BCK (no CRC, so nothing should notice, as
  round 1's B69NOCRCD) and in B69CRC.BCK (the CRC finds it: XORERRS),
  and a byte of block 2's header in B69HDRD.BCK (XORERRS always).
- Does VMS read a save set govax wrote to the host and that came onto
  the volume as fixed 512-byte records, as a binary transfer leaves one
  (HOST.BCK)? govax reads one anyway (Part B's Decision 3); VMS's
  message for it, if it has one, is noted as a difference.
- Does ANALYZE/DISK_STRUCTURE find govax's restores and saves left the
  volume sound?

| File | What it is |
| ---- | ---------- |
| `exchange.cmd` | The govax console script that makes the exchange volume: round 1's tree restored from VMS's save sets, then round 1's Part 2 saves made by govax (with `/SINCE` and `/BEFORE` naming round 1's day, 20-OCT-2026, where Part 2 said TODAY), a save set written to the host and copied on as HOST.BCK, RUN2.COM, and SSCOPY.MAR |
| `run2.com` | The procedure VMS runs |
| `copyout.cmd` | The govax console script that copies the results into `../vax/round2/` and govax's save sets, as VMS read them, into `../vax/round2/govax/` |

govax runs RUN2.COM too (all but ANALYZE/DISK_STRUCTURE, which it lacks);
that dry run was checked before the volume went to the VAX.

## Running it

1. From the repository root, build the exchange volume:

       govax console < testdata/backup69/round2/exchange.cmd

   This makes `testdata/disks/backup69-round2.dsk` (gitignored), an RD54
   labeled BACKUP69 with 3-block clusters, and
   `testdata/disks/backup69-host.bck`, the host save set.
2. On the VAX, mount it and run the procedure once, with its `[000000]`
   as the default directory, as SYSTEM (the restores' `/BY_OWNER=ORIGINAL`
   and ANALYZE need its privileges):

       $ MOUNT DUA1: BACKUP69
       $ SET DEFAULT DUA1:[000000]
       $ @RUN2/OUTPUT=RUN2.LOG

3. Back on the host, copy the results into `testdata/backup69/vax/round2/`:

       govax console < testdata/backup69/round2/copyout.cmd

Nothing on the volume is DIGITAL's: the tree is round 1's own files
(PI.EXE linked from `testdata/mar/pi.mar`), and the save sets are
govax's.

## The results (2026-10-20)

VMS V7.1 ran it; everything is in `../vax/round2/`, govax's save sets
as VMS read them in `govax/`, and SSCOPY's damaged copies beside them.
`TestBackup69Round2` (`internal/console/backup69round2_test.go`) makes
the exchange volume again from `exchange.cmd`, holds its save sets to the
ones VMS read (byte for byte but for the run's own values), runs RUN2.COM
on govax, and matches VMS's log and reports.

- VMS listed every one of govax's save sets as govax lists them, and
  restored and compared all of them, `/CRC` on the input included: govax's
  data CRCs and XOR blocks are right. SSCOPY's damage was recovered as
  round 1's was (XORERRS for B69CRCD and B69HDRD, nothing for B69D, whose
  data has no CRC), and every file came back.
- **A CPU ID of 0 isn't listed:** VMS left out `CPU ID register:` for
  govax's save sets. govax's listing now does too.
- **A compare passes over directory files:** VMS said nothing at all of
  SUB1.DIR and SUB2.DIR, no COMPARED and no VERIFYERR. govax now does the
  same.
- **A restore of directory files alone isn't NOFILES:** B69NONE.BCK
  holds only the two directory files; VMS made them (two CREDIRs) and
  ended with %X10000001. govax counted none and said NOFILES; it now
  counts a restored directory file.
- **A save set as 512-byte records is NOTSAVESET:** VMS refused HOST.BCK
  (`%BACKUP-F-NOTSAVESET, ... is not a BACKUP save set`, %X10A38104),
  listing and restoring. govax reads it (Decision 3), a listed
  difference.
- **VMS reads a file's version limit from its directory record:** its
  DIRECTORY/FULL of govax's restored tree (`tree2.lst`) said "No version
  limit" for VERLIM.TXT, whose limit govax's restore had put only in the
  header. govax's restore now sets the record too (ods2's
  `Directory.SetNameVersionLimit`).
- ANALYZE/DISK_STRUCTURE found the volume sound. Its only notices were
  QUOTA.SYS missing and BADHIGHWATER ("inconsistent highwater mark and
  EFBLK") for the twelve header-only NOBACK.TXT files VMS's own restores
  made; govax's NOBACK.TXT, restored with its data, wasn't named. Whether
  VMS's restores of its own save sets do the same is unconfirmed (round 1
  ran no ANALYZE).
