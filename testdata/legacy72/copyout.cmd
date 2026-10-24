! COPYOUT.CMD - copies Phase 72's probe results off
! legacy72-exchange.dsk into testdata/legacy72/vax (README.md). Run
! from the repository root after @LEGACY72/OUTPUT=LEGACY72.LOG on the
! VAX:
!
!     govax console < testdata/legacy72/copyout.cmd
!
! VMSRTL.EXE and the game's images aren't copied: they are DIGITAL's,
! or the game's, not the probe's.
MOUNT DUA2 "testdata/disks/legacy72-exchange.dsk"
COPY DUA2:[000000]LEGACY72.LOG "testdata/legacy72/vax/legacy72.log"/HOST/QUIET
COPY DUA2:[000000]DUN72.DLG "testdata/legacy72/vax/dun72.dlg"/HOST/QUIET
COPY DUA2:[000000]DUNGEON.ANI "testdata/legacy72/vax/dungeon.ani"/HOST/QUIET
COPY DUA2:[000000]VMSRTL.ANI "testdata/legacy72/vax/vmsrtl.ani"/HOST/QUIET
COPY DUA2:[000000]GLOBAL.LIS "testdata/legacy72/vax/global.lis"/HOST/QUIET
DISMOUNT DUA2
