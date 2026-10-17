! EXCHANGE.CMD - builds the exchange volume for Phase 64's probe
! (testdata/pg64/README.md). Run from the repository root:
!
!     govax console < testdata/pg64/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/pg64-exchange.dsk" /DEVICE=RD51 PG64
MOUNT/WRITE DUA1 "testdata/disks/pg64-exchange.dsk"
COPY "testdata/pg64/pg64.mar"/HOST DUA1:[000000]PG64.MAR
COPY "testdata/pg64/pg64v.mar"/HOST DUA1:[000000]PG64V.MAR
COPY "testdata/pg64/pg64.com"/HOST DUA1:[000000]PG64.COM
COPY "testdata/pg64/pg64l.com"/HOST DUA1:[000000]PG64L.COM
DIRECTORY DUA1:[000000]
DISMOUNT DUA1
