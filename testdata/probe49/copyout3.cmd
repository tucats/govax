! COPYOUT3.CMD - copies the probe's third round's logs off probe49c.dsk
! into the repository (README.md). Run from the repository root after
! @PROBE6C:
!
!     govax console < testdata/probe49/copyout3.cmd
!
MOUNT DUA1 "testdata/disks/probe49c.dsk"
COPY DUA1:[000000]PROBE6C.LOG "testdata/probe49/vax/probe6c.log"/HOST/QUIET
COPY DUA1:[000000]P6C_BUILD.LOG "testdata/probe49/vax/p6c_build.log"/HOST/QUIET
DISMOUNT DUA1
