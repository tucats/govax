! EXCHANGE.CMD - builds the exchange volume for Phase 65's probe
! (testdata/pg65/README.md). Run from the repository root:
!
!     govax console < testdata/pg65/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/pg65-exchange.dsk" /DEVICE=RD51 PG65
MOUNT/WRITE DUA1 "testdata/disks/pg65-exchange.dsk"
COPY "testdata/pg65/pg65.mar"/HOST DUA1:[000000]PG65.MAR
COPY "testdata/pg65/pg65.com"/HOST DUA1:[000000]PG65.COM
DIRECTORY DUA1:[000000]
DISMOUNT DUA1
