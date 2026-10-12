! EXCHANGE.CMD - builds the Phase 53 probe's exchange volume with
! govax. Written by testdata/lib53/gen.go. Run from the repository root:
!
!     govax console < testdata/lib53/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/lib53-exchange.dsk" /DEVICE=RD51 LIB53
MOUNT/WRITE DUA1 "testdata/disks/lib53-exchange.dsk"
COPY "testdata/lib53/lib53.mar"/HOST DUA1:[000000]LIB53.MAR
COPY "testdata/lib53/lib53.com"/HOST DUA1:[000000]LIB53.COM
DIRECTORY DUA1:[000000]
DISMOUNT DUA1
