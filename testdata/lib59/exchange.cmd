! EXCHANGE.CMD - builds the Phase 59 probe's exchange volume with
! govax. Written by testdata/lib59/gen.go. Run from the repository root:
!
!     govax console < testdata/lib59/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/lib59-exchange.dsk" /DEVICE=RD51 LIB59
MOUNT/WRITE DUA1 "testdata/disks/lib59-exchange.dsk"
COPY "testdata/lib59/lib59.mar"/HOST DUA1:[000000]LIB59.MAR
COPY "testdata/lib59/lib59.com"/HOST DUA1:[000000]LIB59.COM
DIRECTORY DUA1:[000000]
DISMOUNT DUA1
