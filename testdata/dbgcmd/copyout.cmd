! COPYOUT.CMD - copies the results of Phase 42's debugger probe off
! dbgcmd-exchange.dsk into testdata/dbgcmd/vax/ (docs/PHASE-42.md,
! subtask 1). Run from the repository root after
! @DBGCMD/OUTPUT=DBGCMD.LOG, with simh paused or the disk detached:
!
!     mkdir -p testdata/dbgcmd/vax
!     govax console < testdata/dbgcmd/copyout.cmd
!
MOUNT DUA1 "testdata/disks/dbgcmd-exchange.dsk"
COPY DUA1:[000000]DBGCMD.LOG "testdata/dbgcmd/vax/dbgcmd.log"/HOST/QUIET
COPY DUA1:[000000]DBGCMD.LIS "testdata/dbgcmd/vax/dbgcmd.lis"/HOST/QUIET
COPY DUA1:[000000]DBGCMD.OBJ "testdata/dbgcmd/vax/dbgcmd.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]DBGCMD.EXE "testdata/dbgcmd/vax/dbgcmd.exe"/HOST/BINARY/QUIET
COPY DUA1:[000000]DBGCMD.MAP "testdata/dbgcmd/vax/dbgcmd.map"/HOST/QUIET
COPY DUA1:[000000]DBGCMD.ANI "testdata/dbgcmd/vax/dbgcmd.ani"/HOST/QUIET
COPY DUA1:[000000]BREAK.DLG "testdata/dbgcmd/vax/break.dlg"/HOST/QUIET
COPY DUA1:[000000]BRKCLS.DLG "testdata/dbgcmd/vax/brkcls.dlg"/HOST/QUIET
COPY DUA1:[000000]STEP.DLG "testdata/dbgcmd/vax/step.dlg"/HOST/QUIET
COPY DUA1:[000000]WATCH.DLG "testdata/dbgcmd/vax/watch.dlg"/HOST/QUIET
COPY DUA1:[000000]TRACE.DLG "testdata/dbgcmd/vax/trace.dlg"/HOST/QUIET
COPY DUA1:[000000]EXAM.DLG "testdata/dbgcmd/vax/exam.dlg"/HOST/QUIET
COPY DUA1:[000000]CALL.DLG "testdata/dbgcmd/vax/call.dlg"/HOST/QUIET
COPY DUA1:[000000]EXCEPT.DLG "testdata/dbgcmd/vax/except.dlg"/HOST/QUIET
COPY DUA1:[000000]ERRORS.DLG "testdata/dbgcmd/vax/errors.dlg"/HOST/QUIET
DISMOUNT DUA1
