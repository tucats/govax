! COPYOUT.CMD - copies probe 4's log back into testdata/mp/probe4/vax/
! (README.md). Run from the repository root after @PROBE4/OUTPUT=PROBE4.LOG:
!
!     govax console < testdata/mp/probe4/copyout.cmd
!
MOUNT DUA1 "testdata/disks/mp-probe4.dsk"
COPY DUA1:[000000]PROBE4.LOG "testdata/mp/probe4/vax/probe4.log"/HOST/QUIET
DISMOUNT DUA1
