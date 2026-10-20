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
