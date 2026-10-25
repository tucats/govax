! EXCHANGE.CMD - builds Phase 73's probe exchange volume with govax
! (README.md). Run from the repository root:
!
!     govax console < testdata/trace73/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/trace73-exchange.dsk" /DEVICE=RD51 TRACE73
MOUNT/WRITE DUA2 "testdata/disks/trace73-exchange.dsk"
COPY "testdata/trace73/trace73.mar"/HOST DUA2:[000000]TRACE73.MAR
COPY "testdata/trace73/trace73.com"/HOST DUA2:[000000]TRACE73.COM
COPY "testdata/trace73/trstep.dbg"/HOST DUA2:[000000]TRSTEP.DBG
COPY "testdata/trace73/trgo.dbg"/HOST DUA2:[000000]TRGO.DBG
COPY "testdata/trace73/trexc.dbg"/HOST DUA2:[000000]TREXC.DBG
COPY "testdata/trace73/trhand.dbg"/HOST DUA2:[000000]TRHAND.DBG
COPY "testdata/trace73/trcall.dbg"/HOST DUA2:[000000]TRCALL.DBG
COPY "testdata/trace73/trsvc.dbg"/HOST DUA2:[000000]TRSVC.DBG
COPY "testdata/trace73/trdep.dbg"/HOST DUA2:[000000]TRDEP.DBG
COPY "testdata/trace73/trdepgo.dbg"/HOST DUA2:[000000]TRDEPGO.DBG
COPY "testdata/trace73/trunh.dbg"/HOST DUA2:[000000]TRUNH.DBG
COPY "testdata/trace73/trpriv.dbg"/HOST DUA2:[000000]TRPRIV.DBG
DIRECTORY DUA2:[000000]
DISMOUNT DUA2
