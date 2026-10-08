! COPYOUT.CMD - copies probe 3's log off mp-probe3.dsk into vax/ (README.md).
! Run from the repository root after @PROBE3/OUTPUT=PROBE3.LOG:
!
!     govax console < testdata/mp/probe3/copyout.cmd
!
MOUNT DUA1 "testdata/disks/mp-probe3.dsk"
COPY DUA1:[000000]PROBE3.LOG "testdata/mp/probe3/vax/probe3.log"/HOST/QUIET
DISMOUNT DUA1
