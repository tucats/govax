! EXCHANGE.CMD - builds Phase 70's probe exchange volume with govax
! (README.md). Run from the repository root:
!
!     govax console < testdata/tb70/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/tb70-exchange.dsk" /DEVICE=RD51 TB70
MOUNT/WRITE DUA1 "testdata/disks/tb70-exchange.dsk"
COPY "testdata/tb70/tb70.mar"/HOST DUA1:[000000]TB70.MAR
COPY "testdata/tb70/tb70long.mar"/HOST DUA1:[000000]TB70LONG.MAR
COPY "testdata/tb70/tb70shr.mar"/HOST DUA1:[000000]TB70SHR.MAR
COPY "testdata/tb70/tb70.com"/HOST DUA1:[000000]TB70.COM
DIRECTORY DUA1:[000000]
DISMOUNT DUA1
