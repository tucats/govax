! EXCHANGE.CMD - builds the exchange volume for Phase 67's probe
! (testdata/pg67/README.md). Run from the repository root:
!
!     govax console < testdata/pg67/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/pg67-exchange.dsk" /DEVICE=RD51 PG67
MOUNT/WRITE DUA1 "testdata/disks/pg67-exchange.dsk"
COPY "testdata/pg67/pg67.mar"/HOST DUA1:[000000]PG67.MAR
COPY "testdata/pg67/pg67.com"/HOST DUA1:[000000]PG67.COM
COPY "testdata/pg67/pg67s.opt"/HOST DUA1:[000000]PG67S.OPT
DIRECTORY DUA1:[000000]
DISMOUNT DUA1
