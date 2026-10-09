! COPYOUT.CMD - copies Phase 50's probe log off dcl50.dsk into the
! repository (README.md). Run from the repository root after
! @PROBE50/OUTPUT=PROBE50.LOG:
!
!     govax console < testdata/dcl50/copyout.cmd
!
MOUNT DUA1 "testdata/disks/dcl50.dsk"
COPY DUA1:[000000]PROBE50.LOG "testdata/dcl50/vax/probe50.log"/HOST/QUIET
DISMOUNT DUA1
