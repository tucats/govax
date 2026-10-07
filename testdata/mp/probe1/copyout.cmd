! COPYOUT.CMD - copies the results of probe 1 off mp-probe1.dsk into
! testdata/mp/probe1/vax/ (README.md). Run from the repository root after
! @PROBE1/OUTPUT=PROBE1.LOG:
!
!     govax console < testdata/mp/probe1/copyout.cmd
!
MOUNT DUA1 "testdata/disks/mp-probe1.dsk"
COPY DUA1:[000000]PROBE1.LOG "testdata/mp/probe1/vax/probe1.log"/HOST/QUIET
DISMOUNT DUA1
