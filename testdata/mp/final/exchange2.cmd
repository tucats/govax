! EXCHANGE2.CMD - builds the exchange volume for the second part of the
! multiprocessing program's last VAX run (README.md, "The second run").
! Run from the repository root:
!
!     govax console < testdata/mp/final/exchange2.cmd
!
INITIALIZE/CONTAINER "testdata/disks/mp-final2.dsk" /DEVICE=RD53 MPFINAL2
MOUNT/WRITE DUA1 "testdata/disks/mp-final2.dsk"
COPY "testdata/mp/probe5/p5args.mar"/HOST DUA1:[000000]P5ARGS.MAR
COPY "testdata/mp/probe5/p5args.com"/HOST DUA1:[000000]P5ARGS.COM
DIRECTORY DUA1:[000000]
DISMOUNT DUA1
