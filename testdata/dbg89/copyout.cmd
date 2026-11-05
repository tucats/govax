! COPYOUT.CMD - copies the results of Phase 89's debugger probe off
! dbg89-exchange.dsk into testdata/dbg89/vax/ (README.md). Run from the
! repository root after @DBG89/OUTPUT=DBG89.LOG, with simh paused or the disk
! detached:
!
!     govax console < testdata/dbg89/copyout.cmd
!
MOUNT DUA2 "testdata/disks/dbg89-exchange.dsk"
COPY DUA2:[000000]DBG89.LOG "testdata/dbg89/vax/dbg89.log"/HOST/QUIET
COPY DUA2:[000000]DBG89F.MAR "testdata/dbg89/vax/dbg89f.mar"/HOST/QUIET
COPY DUA2:[000000]DBG89.DLG "testdata/dbg89/vax/dbg89.dlg"/HOST/QUIET
COPY DUA2:[000000]DBG89S.DLG "testdata/dbg89/vax/dbg89s.dlg"/HOST/QUIET
COPY DUA2:[000000]DBG89F.DLG "testdata/dbg89/vax/dbg89f.dlg"/HOST/QUIET
COPY DUA2:[000000]GVDBG89.DLG "testdata/dbg89/vax/gvdbg89.dlg"/HOST/QUIET
COPY DUA2:[000000]DBG89.LIS "testdata/dbg89/vax/dbg89.lis"/HOST/QUIET
COPY DUA2:[000000]DBG89B.LIS "testdata/dbg89/vax/dbg89b.lis"/HOST/QUIET
COPY DUA2:[000000]DBG89F.LIS "testdata/dbg89/vax/dbg89f.lis"/HOST/QUIET
COPY DUA2:[000000]DBG89.MAP "testdata/dbg89/vax/dbg89.map"/HOST/QUIET
COPY DUA2:[000000]DBG89F.MAP "testdata/dbg89/vax/dbg89f.map"/HOST/QUIET
COPY DUA2:[000000]DBG89.OBJ "testdata/dbg89/vax/dbg89.obj"/HOST/BINARY/QUIET
COPY DUA2:[000000]DBG89B.OBJ "testdata/dbg89/vax/dbg89b.obj"/HOST/BINARY/QUIET
COPY DUA2:[000000]DBG89F.OBJ "testdata/dbg89/vax/dbg89f.obj"/HOST/BINARY/QUIET
COPY DUA2:[000000]DBG89.EXE "testdata/dbg89/vax/dbg89.exe"/HOST/BINARY/QUIET
COPY DUA2:[000000]DBG89F.EXE "testdata/dbg89/vax/dbg89f.exe"/HOST/BINARY/QUIET
DISMOUNT DUA2
