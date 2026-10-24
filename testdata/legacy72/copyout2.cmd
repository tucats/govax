! COPYOUT2.CMD - copies Phase 72's round 2 results off
! legacy72-exchange.dsk into testdata/legacy72/vax (README.md). Run
! from the repository root after @LEGACY72B/OUTPUT=LEGACY72B.LOG on the
! VAX:
!
!     govax console < testdata/legacy72/copyout2.cmd
!
MOUNT DUA2 "testdata/disks/legacy72-exchange.dsk"
COPY DUA2:[000000]LEGACY72B.LOG "testdata/legacy72/vax/legacy72b.log"/HOST/QUIET
COPY DUA2:[000000]DUN72B.DLG "testdata/legacy72/vax/dun72b.dlg"/HOST/QUIET
COPY DUA2:[000000]GLOBAL.LIS "testdata/legacy72/vax/global.lis"/HOST/QUIET
DISMOUNT DUA2
