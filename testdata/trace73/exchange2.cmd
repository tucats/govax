! EXCHANGE2.CMD - adds round 2 of Phase 73's probe to the exchange
! volume (README.md). Run from the repository root:
!
!     govax console < testdata/trace73/exchange2.cmd
!
MOUNT/WRITE DUA2 "testdata/disks/trace73-exchange.dsk"
COPY "testdata/trace73/trace73b.mar"/HOST DUA2:[000000]TRACE73B.MAR
COPY "testdata/trace73/trace73b.com"/HOST DUA2:[000000]TRACE73B.COM
DIRECTORY DUA2:[000000]TRACE73B.*
DISMOUNT DUA2
