! EXCHANGE.CMD - builds the Phase 58 probe's exchange volume with
! govax. Written by testdata/lib58/gen.go. Run from the repository root:
!
!     govax console < testdata/lib58/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/lib58-exchange.dsk" /DEVICE=RD51 LIB58
MOUNT/WRITE DUA1 "testdata/disks/lib58-exchange.dsk"
COPY "testdata/lib58/lib58.mar"/HOST DUA1:[000000]LIB58.MAR
COPY "testdata/lib58/lib58.com"/HOST DUA1:[000000]LIB58.COM
DIRECTORY DUA1:[000000]
DISMOUNT DUA1
