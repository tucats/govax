! EXCHANGE.CMD - builds the Phase 55 probe's exchange volume with
! govax. Written by testdata/lib55/gen.go. Run from the repository root:
!
!     govax console < testdata/lib55/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/lib55-exchange.dsk" /DEVICE=RD51 LIB55
MOUNT/WRITE DUA1 "testdata/disks/lib55-exchange.dsk"
COPY "testdata/lib55/lib55.mar"/HOST DUA1:[000000]LIB55.MAR
COPY "testdata/lib55/lib55.com"/HOST DUA1:[000000]LIB55.COM
DIRECTORY DUA1:[000000]
DISMOUNT DUA1
