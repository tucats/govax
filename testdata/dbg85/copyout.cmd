! COPYOUT.CMD - copies the results of Phase 85's debugger probe off
! dbg85-exchange.dsk into testdata/dbg85/vax/ (README.md). Run from the
! repository root after @DBG85/OUTPUT=DBG85.LOG, with simh paused or the disk
! detached:
!
!     govax console < testdata/dbg85/copyout.cmd
!
MOUNT DUA2 "testdata/disks/dbg85-exchange.dsk"
COPY DUA2:[000000]DBG85.LOG "testdata/dbg85/vax/dbg85.log"/HOST/QUIET
COPY DUA2:[000000]DBG85.MAR "testdata/dbg85/vax/dbg85.mar"/HOST/QUIET
COPY DUA2:[000000]DBG85O.MAR "testdata/dbg85/vax/dbg85o.mar"/HOST/QUIET
COPY DUA2:[000000]DBG85M.DLG "testdata/dbg85/vax/dbg85m.dlg"/HOST/QUIET
COPY DUA2:[000000]GVDBG85M.DLG "testdata/dbg85/vax/gvdbg85m.dlg"/HOST/QUIET
COPY DUA2:[000000]DBG85.DLG "testdata/dbg85/vax/dbg85.dlg"/HOST/QUIET
COPY DUA2:[000000]GVDBG85.DLG "testdata/dbg85/vax/gvdbg85.dlg"/HOST/QUIET
COPY DUA2:[000000]DBG85C.DLG "testdata/dbg85/vax/dbg85c.dlg"/HOST/QUIET
COPY DUA2:[000000]DBG85.LIS "testdata/dbg85/vax/dbg85.lis"/HOST/QUIET
COPY DUA2:[000000]DBG85M.LIS "testdata/dbg85/vax/dbg85m.lis"/HOST/QUIET
COPY DUA2:[000000]DBG85N.LIS "testdata/dbg85/vax/dbg85n.lis"/HOST/QUIET
COPY DUA2:[000000]DBG85O.LIS "testdata/dbg85/vax/dbg85o.lis"/HOST/QUIET
COPY DUA2:[000000]DBG85.MAP "testdata/dbg85/vax/dbg85.map"/HOST/QUIET
COPY DUA2:[000000]DBG85M.MAP "testdata/dbg85/vax/dbg85m.map"/HOST/QUIET
COPY DUA2:[000000]DBG85C.MAP "testdata/dbg85/vax/dbg85c.map"/HOST/QUIET
COPY DUA2:[000000]DBG85.OBJ "testdata/dbg85/vax/dbg85.obj"/HOST/BINARY/QUIET
COPY DUA2:[000000]DBG85M.OBJ "testdata/dbg85/vax/dbg85m.obj"/HOST/BINARY/QUIET
COPY DUA2:[000000]DBG85.EXE "testdata/dbg85/vax/dbg85.exe"/HOST/BINARY/QUIET
COPY DUA2:[000000]DBG85M.EXE "testdata/dbg85/vax/dbg85m.exe"/HOST/BINARY/QUIET
COPY DUA2:[000000]DBG85N.DLG "testdata/dbg85/vax/dbg85n.dlg"/HOST/QUIET
DISMOUNT DUA2
