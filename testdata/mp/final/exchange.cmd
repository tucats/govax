! EXCHANGE.CMD - builds the exchange volume for the multiprocessing
! program's last VAX run (testdata/mp/final/README.md). Run from the
! repository root:
!
!     govax console < testdata/mp/final/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/mp-final.dsk" /DEVICE=RD53 MPFINAL
MOUNT/WRITE DUA1 "testdata/disks/mp-final.dsk"
COPY "testdata/mp/macros/r7_lock.mar"/HOST DUA1:[000000]R7_LOCK.MAR
COPY "testdata/mp/macros/macros7.com"/HOST DUA1:[000000]MACROS7.COM
COPY "testdata/mp/probe5/probe5.mar"/HOST DUA1:[000000]PROBE5.MAR
COPY "testdata/mp/probe5/p5sleep.mar"/HOST DUA1:[000000]P5SLEEP.MAR
COPY "testdata/mp/probe5/p5lock.mar"/HOST DUA1:[000000]P5LOCK.MAR
COPY "testdata/mp/probe5/p5info.mar"/HOST DUA1:[000000]P5INFO.MAR
COPY "testdata/mp/probe5/probe5.com"/HOST DUA1:[000000]PROBE5.COM
COPY "testdata/mp/final/final.com"/HOST DUA1:[000000]FINAL.COM
DIRECTORY DUA1:[000000]
DISMOUNT DUA1
