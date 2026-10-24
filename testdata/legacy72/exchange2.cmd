! EXCHANGE2.CMD - adds round 2's files to Phase 72's probe exchange
! volume (README.md). Run from the repository root, with the volume
! detached from simh:
!
!     govax console < testdata/legacy72/exchange2.cmd
!
MOUNT/WRITE DUA2 "testdata/disks/legacy72-exchange.dsk"
COPY "testdata/legacy72/legacy72b.com"/HOST DUA2:[000000]LEGACY72B.COM
COPY "testdata/legacy72/dun72b.dbg"/HOST DUA2:[000000]DUN72B.DBG
DIRECTORY/SIZE/DATE DUA2:[000000]
DISMOUNT DUA2
