! COPYOUT2.CMD - copies the probe's second round's logs off probe49b.dsk
! into the repository (README.md). Run from the repository root after
! @PROBE6B:
!
!     govax console < testdata/probe49/copyout2.cmd
!
MOUNT DUA1 "testdata/disks/probe49b.dsk"
COPY DUA1:[000000]PROBE6B.LOG "testdata/probe49/vax/probe6b.log"/HOST/QUIET
COPY DUA1:[000000]P6B_BUILD.LOG "testdata/probe49/vax/p6b_build.log"/HOST/QUIET
DISMOUNT DUA1
