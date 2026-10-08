! COPYOUT2.CMD - copies the second run's log off mp-final2.dsk (README.md).
! Run from the repository root after @P5ARGS/OUTPUT=P5ARGS.LOG:
!
!     govax console < testdata/mp/final/copyout2.cmd
!
MOUNT DUA1 "testdata/disks/mp-final2.dsk"
COPY DUA1:[000000]P5ARGS.LOG "testdata/mp/probe5/vax/p5args.log"/HOST/QUIET
DISMOUNT DUA1
