! COPYOUT.CMD - copies Phase 50's probe logs off dcl50.dsk into the
! repository (README.md). Run from the repository root after the round's
! @PROBE50F/OUTPUT=PROBE50F.LOG and @PROBE50Q (round 4; round 1's was
! @PROBE50/OUTPUT=PROBE50.LOG, round 2's @PROBE50V/OUTPUT=PROBE50V.LOG,
! and round 3's @PROBE50I/OUTPUT=PROBE50I.LOG, copied to vax/probe50.log,
! vax/probe50v.log, and vax/probe50i.log the same way):
!
!     govax console < testdata/dcl50/copyout.cmd
!
! PROBE50P.LOG is the log PROBE50Q's Q11 makes. PROBE50Q's own session is
! the terminal's, which is kept by hand as vax/probe50q.log.
!
MOUNT DUA1 "testdata/disks/dcl50.dsk"
COPY DUA1:[000000]PROBE50F.LOG "testdata/dcl50/vax/probe50f.log"/HOST/QUIET
COPY DUA1:[000000]PROBE50P.LOG "testdata/dcl50/vax/probe50p.log"/HOST/QUIET
DISMOUNT DUA1
