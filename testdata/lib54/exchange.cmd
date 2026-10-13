! EXCHANGE.CMD - builds the Phase 54 probe's exchange volume with
! govax. Written by testdata/lib54/gen.go. Run from the repository root:
!
!     govax console < testdata/lib54/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/lib54-exchange.dsk" /DEVICE=RD51 LIB54
MOUNT/WRITE DUA1 "testdata/disks/lib54-exchange.dsk"
COPY "testdata/lib54/lib54.mar"/HOST DUA1:[000000]LIB54.MAR
COPY "testdata/lib54/lib54.com"/HOST DUA1:[000000]LIB54.COM
DIRECTORY DUA1:[000000]
DISMOUNT DUA1
