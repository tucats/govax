! COPYOUT.CMD - copies the results of Phase 41's debugger probe off
! dbg-exchange.dsk into testdata/dbg/vax/ (docs/PHASE-41.md, subtask 1).
! Run from the repository root after @DBG/OUTPUT=DBG.LOG, with simh
! paused or the disk detached:
!
!     govax console < testdata/dbg/copyout.cmd
!
MOUNT DUA1 "testdata/disks/dbg-exchange.dsk"
COPY DUA1:[000000]DBG.LOG "testdata/dbg/vax/dbg.log"/HOST/QUIET
COPY DUA1:[000000]DBGDIS.LIS "testdata/dbg/vax/dbgdis.lis"/HOST/QUIET
COPY DUA1:[000000]DBGSUB.LIS "testdata/dbg/vax/dbgsub.lis"/HOST/QUIET
COPY DUA1:[000000]DBGDIS.OBJ "testdata/dbg/vax/dbgdis.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]DBGSUB.OBJ "testdata/dbg/vax/dbgsub.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]DBGDIS.EXE "testdata/dbg/vax/dbgdis.exe"/HOST/BINARY/QUIET
COPY DUA1:[000000]DBGDIS.MAP "testdata/dbg/vax/dbgdis.map"/HOST/QUIET
COPY DUA1:[000000]DBGDIS.ANI "testdata/dbg/vax/dbgdis.ani"/HOST/QUIET
COPY DUA1:[000000]DBGDIS.DLG "testdata/dbg/vax/dbgdis.dlg"/HOST/QUIET
COPY DUA1:[000000]DBGTRC.EXE "testdata/dbg/vax/dbgtrc.exe"/HOST/BINARY/QUIET
COPY DUA1:[000000]DBGTRC.MAP "testdata/dbg/vax/dbgtrc.map"/HOST/QUIET
COPY DUA1:[000000]DBGTRC.ANI "testdata/dbg/vax/dbgtrc.ani"/HOST/QUIET
COPY DUA1:[000000]DBGTRC.DLG "testdata/dbg/vax/dbgtrc.dlg"/HOST/QUIET
COPY DUA1:[000000]DBGNOTB.EXE "testdata/dbg/vax/dbgnotb.exe"/HOST/BINARY/QUIET
COPY DUA1:[000000]DBGNOTB.MAP "testdata/dbg/vax/dbgnotb.map"/HOST/QUIET
COPY DUA1:[000000]DBGNOTB.ANI "testdata/dbg/vax/dbgnotb.ani"/HOST/QUIET
COPY DUA1:[000000]DBGNOTB.DLG "testdata/dbg/vax/dbgnotb.dlg"/HOST/QUIET
COPY DUA1:[000000]TRDBGLNK.EXE "testdata/dbg/vax/trdbglnk.exe"/HOST/BINARY/QUIET
COPY DUA1:[000000]TRDBGLNK.MAP "testdata/dbg/vax/trdbglnk.map"/HOST/QUIET
COPY DUA1:[000000]TRDBGLNK.ANI "testdata/dbg/vax/trdbglnk.ani"/HOST/QUIET
COPY DUA1:[000000]TRDBGLNK.DLG "testdata/dbg/vax/trdbglnk.dlg"/HOST/QUIET
COPY DUA1:[000000]TRLNKDBG.EXE "testdata/dbg/vax/trlnkdbg.exe"/HOST/BINARY/QUIET
COPY DUA1:[000000]TRLNKDBG.MAP "testdata/dbg/vax/trlnkdbg.map"/HOST/QUIET
COPY DUA1:[000000]TRLNKDBG.ANI "testdata/dbg/vax/trlnkdbg.ani"/HOST/QUIET
COPY DUA1:[000000]TRLNKDBG.DLG "testdata/dbg/vax/trlnkdbg.dlg"/HOST/QUIET
COPY DUA1:[000000]TRDBGTRC.EXE "testdata/dbg/vax/trdbgtrc.exe"/HOST/BINARY/QUIET
COPY DUA1:[000000]TRDBGTRC.MAP "testdata/dbg/vax/trdbgtrc.map"/HOST/QUIET
COPY DUA1:[000000]TRDBGTRC.ANI "testdata/dbg/vax/trdbgtrc.ani"/HOST/QUIET
COPY DUA1:[000000]TRDBGTRC.DLG "testdata/dbg/vax/trdbgtrc.dlg"/HOST/QUIET
COPY DUA1:[000000]TRNOTB.EXE "testdata/dbg/vax/trnotb.exe"/HOST/BINARY/QUIET
COPY DUA1:[000000]TRNOTB.MAP "testdata/dbg/vax/trnotb.map"/HOST/QUIET
COPY DUA1:[000000]TRNOTB.ANI "testdata/dbg/vax/trnotb.ani"/HOST/QUIET
COPY DUA1:[000000]TRNOTB.DLG "testdata/dbg/vax/trnotb.dlg"/HOST/QUIET
COPY DUA1:[000000]FAILLNK.EXE "testdata/dbg/vax/faillnk.exe"/HOST/BINARY/QUIET
COPY DUA1:[000000]FAILLNK.MAP "testdata/dbg/vax/faillnk.map"/HOST/QUIET
COPY DUA1:[000000]FAILLNK.ANI "testdata/dbg/vax/faillnk.ani"/HOST/QUIET
COPY DUA1:[000000]FAILLNK.DLG "testdata/dbg/vax/faillnk.dlg"/HOST/QUIET
COPY DUA1:[000000]FORTH.EXE "testdata/dbg/vax/forth.exe"/HOST/BINARY/QUIET
COPY DUA1:[000000]FORTH.MAP "testdata/dbg/vax/forth.map"/HOST/QUIET
COPY DUA1:[000000]FORTH.ANI "testdata/dbg/vax/forth.ani"/HOST/QUIET
COPY DUA1:[000000]FORTH.DLG "testdata/dbg/vax/forth.dlg"/HOST/QUIET
COPY DUA1:[000000]GVDBGDIS.EXE "testdata/dbg/vax/gvdbgdis.exe"/HOST/BINARY/QUIET
COPY DUA1:[000000]GVDBGDIS.MAP "testdata/dbg/vax/gvdbgdis.map"/HOST/QUIET
COPY DUA1:[000000]GVDBGDIS.ANI "testdata/dbg/vax/gvdbgdis.ani"/HOST/QUIET
COPY DUA1:[000000]GVDBGDIS.DLG "testdata/dbg/vax/gvdbgdis.dlg"/HOST/QUIET
COPY DUA1:[000000]GVTRACE.EXE "testdata/dbg/vax/gvtrace.exe"/HOST/BINARY/QUIET
COPY DUA1:[000000]GVTRACE.MAP "testdata/dbg/vax/gvtrace.map"/HOST/QUIET
COPY DUA1:[000000]GVTRACE.ANI "testdata/dbg/vax/gvtrace.ani"/HOST/QUIET
COPY DUA1:[000000]GVTRACE.DLG "testdata/dbg/vax/gvtrace.dlg"/HOST/QUIET
DISMOUNT DUA1
