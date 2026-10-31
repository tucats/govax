! EXCHANGE.CMD - builds the Phase 77 probe's exchange volume with
! govax. Written by testdata/mth77/gen.go. Run from the repository root:
!
!     govax console < testdata/mth77/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/mth77-exchange.dsk" /DEVICE=RD51 MTH77
MOUNT/WRITE DUA2 "testdata/disks/mth77-exchange.dsk"
COPY "testdata/mth77/mth77.mar"/HOST DUA2:[000000]MTH77.MAR
COPY "testdata/mth77/mth77.com"/HOST DUA2:[000000]MTH77.COM
DIRECTORY DUA2:[000000]
DISMOUNT DUA2
