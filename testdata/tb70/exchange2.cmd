! EXCHANGE2.CMD - adds round 2 of Phase 70's probe to the exchange
! volume (README.md). Run from the repository root:
!
!     govax console < testdata/tb70/exchange2.cmd
!
MOUNT/WRITE DUA1 "testdata/disks/tb70-exchange.dsk"
COPY "testdata/tb70/tb70b.mar"/HOST DUA1:[000000]TB70B.MAR
COPY "testdata/tb70/tb70b.com"/HOST DUA1:[000000]TB70B.COM
DIRECTORY DUA1:[000000]TB70B*.*
DISMOUNT DUA1
