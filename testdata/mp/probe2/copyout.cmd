! COPYOUT.CMD - copies the results of probe 1 off mp-probe2.dsk into
! testdata/mp/probe2/vax/ (README.md). Run from the repository root after
! @PROBE2/OUTPUT=PROBE2.LOG:
!
!     govax console < testdata/mp/probe2/copyout.cmd
!
MOUNT DUA1 "testdata/disks/mp-probe2.dsk"
COPY DUA1:[000000]PROBE2.LOG "testdata/mp/probe2/vax/probe2.log"/HOST/QUIET
DISMOUNT DUA1
