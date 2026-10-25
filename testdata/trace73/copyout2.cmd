! COPYOUT2.CMD - copies round 2 of Phase 73's probe off
! trace73-exchange.dsk into testdata/trace73/vax (README.md). Run from
! the repository root after @TRACE73B/OUTPUT=TRACE73B.LOG on the VAX:
!
!     govax console < testdata/trace73/copyout2.cmd
!
MOUNT DUA2 "testdata/disks/trace73-exchange.dsk"
COPY DUA2:[000000]TRACE73B.LOG "testdata/trace73/vax/trace73b.log"/HOST/QUIET
COPY DUA2:[000000]TRACE73B.MAP "testdata/trace73/vax/trace73b.map"/HOST/QUIET
COPY/BINARY DUA2:[000000]TRACE73B.EXE "testdata/trace73/vax/trace73b.exe"/HOST/QUIET
COPY DUA2:[000000]TRACE73B.OBJ "testdata/trace73/vax/trace73b.obj"/HOST/QUIET
DISMOUNT DUA2
