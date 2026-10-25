! COPYOUT.CMD - copies Phase 73's probe results off
! trace73-exchange.dsk into testdata/trace73/vax (README.md). Run from
! the repository root after @TRACE73/OUTPUT=TRACE73.LOG on the VAX:
!
!     govax console < testdata/trace73/copyout.cmd
!
! The images come back byte for byte (/BINARY), so govax's oracle can
! run VMS's own images.
MOUNT DUA2 "testdata/disks/trace73-exchange.dsk"
COPY DUA2:[000000]TRACE73.LOG "testdata/trace73/vax/trace73.log"/HOST/QUIET
COPY DUA2:[000000]TRACE73.MAP "testdata/trace73/vax/trace73.map"/HOST/QUIET
COPY/BINARY DUA2:[000000]TRACE73.EXE "testdata/trace73/vax/trace73.exe"/HOST/QUIET
COPY DUA2:[000000]TRACE73NT.MAP "testdata/trace73/vax/trace73nt.map"/HOST/QUIET
COPY/BINARY DUA2:[000000]TRACE73NT.EXE "testdata/trace73/vax/trace73nt.exe"/HOST/QUIET
COPY DUA2:[000000]TRACE73D.MAP "testdata/trace73/vax/trace73d.map"/HOST/QUIET
COPY/BINARY DUA2:[000000]TRACE73D.EXE "testdata/trace73/vax/trace73d.exe"/HOST/QUIET
COPY DUA2:[000000]TRACE73.OBJ "testdata/trace73/vax/trace73.obj"/HOST/QUIET
COPY DUA2:[000000]TRACE73D.OBJ "testdata/trace73/vax/trace73d.obj"/HOST/QUIET
COPY DUA2:[000000]TRSTEP.DLG "testdata/trace73/vax/trstep.dlg"/HOST/QUIET
COPY DUA2:[000000]TRGO.DLG "testdata/trace73/vax/trgo.dlg"/HOST/QUIET
COPY DUA2:[000000]TREXC.DLG "testdata/trace73/vax/trexc.dlg"/HOST/QUIET
COPY DUA2:[000000]TRHAND.DLG "testdata/trace73/vax/trhand.dlg"/HOST/QUIET
COPY DUA2:[000000]TRCALL.DLG "testdata/trace73/vax/trcall.dlg"/HOST/QUIET
COPY DUA2:[000000]TRSVC.DLG "testdata/trace73/vax/trsvc.dlg"/HOST/QUIET
COPY DUA2:[000000]TRDEP.DLG "testdata/trace73/vax/trdep.dlg"/HOST/QUIET
COPY DUA2:[000000]TRDEPGO.DLG "testdata/trace73/vax/trdepgo.dlg"/HOST/QUIET
COPY DUA2:[000000]TRUNH.DLG "testdata/trace73/vax/trunh.dlg"/HOST/QUIET
COPY DUA2:[000000]TRPRIV.DLG "testdata/trace73/vax/trpriv.dlg"/HOST/QUIET
DISMOUNT DUA2
