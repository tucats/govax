! COPYOUT.CMD - copies Phase 50's probe log off dcl50.dsk into the
! repository (README.md). Run from the repository root after the round's
! @PROBE50I/OUTPUT=PROBE50I.LOG (round 3; round 1's was
! @PROBE50/OUTPUT=PROBE50.LOG and round 2's @PROBE50V/OUTPUT=PROBE50V.LOG,
! copied to vax/probe50.log and vax/probe50v.log the same way):
!
!     govax console < testdata/dcl50/copyout.cmd
!
MOUNT DUA1 "testdata/disks/dcl50.dsk"
COPY DUA1:[000000]PROBE50I.LOG "testdata/dcl50/vax/probe50i.log"/HOST/QUIET
DISMOUNT DUA1
