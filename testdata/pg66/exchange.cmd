! EXCHANGE.CMD - builds the exchange volume for Phase 66's probe
! (testdata/pg66/README.md). Run from the repository root:
!
!     govax console < testdata/pg66/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/pg66-exchange.dsk" /DEVICE=RD51 PG66
MOUNT/WRITE DUA1 "testdata/disks/pg66-exchange.dsk"
COPY "testdata/pg66/pg66.mar"/HOST DUA1:[000000]PG66.MAR
COPY "testdata/pg66/pg66.com"/HOST DUA1:[000000]PG66.COM
COPY "testdata/pg66/pg66c.opt"/HOST DUA1:[000000]PG66C.OPT
DIRECTORY DUA1:[000000]
DISMOUNT DUA1
