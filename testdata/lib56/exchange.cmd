! EXCHANGE.CMD - builds the Phase 56 probe's exchange volume with
! govax. Written by testdata/lib56/gen.go. Run from the repository root:
!
!     govax console < testdata/lib56/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/lib56-exchange.dsk" /DEVICE=RD51 LIB56
MOUNT/WRITE DUA1 "testdata/disks/lib56-exchange.dsk"
COPY "testdata/lib56/lib56.mar"/HOST DUA1:[000000]LIB56.MAR
COPY "testdata/lib56/lib56.com"/HOST DUA1:[000000]LIB56.COM
DIRECTORY DUA1:[000000]
DISMOUNT DUA1
