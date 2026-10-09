! COPYOUT.CMD - copies Phase 49's probe results off probe49.dsk into
! the repository (README.md). Run from the repository root after
! @PROBE6:
!
!     govax console < testdata/probe49/copyout.cmd
!
MOUNT DUA1 "testdata/disks/probe49.dsk"
COPY DUA1:[000000]PROBE6.LOG "testdata/probe49/vax/probe6.log"/HOST/QUIET
COPY DUA1:[000000]P6LINK.LOG "testdata/probe49/vax/p6link.log"/HOST/QUIET
COPY DUA1:[000000]P6BUILD.LOG "testdata/probe49/vax/p6build.log"/HOST/QUIET
DISMOUNT DUA1
