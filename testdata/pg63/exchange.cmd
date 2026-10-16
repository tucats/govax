! EXCHANGE.CMD - builds the exchange volume for Phase 63's probe
! (testdata/pg63/README.md). Run from the repository root:
!
!     govax console < testdata/pg63/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/pg63-exchange.dsk" /DEVICE=RD51 PG63
MOUNT/WRITE DUA1 "testdata/disks/pg63-exchange.dsk"
COPY "testdata/pg63/pg63.mar"/HOST DUA1:[000000]PG63.MAR
COPY "testdata/pg63/pg63.com"/HOST DUA1:[000000]PG63.COM
COPY "testdata/pg63/pg63s.com"/HOST DUA1:[000000]PG63S.COM
COPY "testdata/mp/macros/r8_purgws.mar"/HOST DUA1:[000000]R8_PURGWS.MAR
COPY "testdata/mp/macros/macros8.com"/HOST DUA1:[000000]MACROS8.COM
DIRECTORY DUA1:[000000]
DISMOUNT DUA1
