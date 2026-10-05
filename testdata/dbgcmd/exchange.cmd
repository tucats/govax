! EXCHANGE.CMD - builds the exchange volume for Phase 42's debugger
! probe (docs/PHASE-42.md, subtask 1) with govax. Run from the
! repository root:
!
!     govax console < testdata/dbgcmd/exchange.cmd
!
! It makes testdata/disks/dbgcmd-exchange.dsk (RD51 size, label DBGCMDX,
! gitignored) and copies the source, the debugger command files, and
! DBGCMD.COM onto it. On VMS, mount it, make it the default directory,
! and run @DBGCMD/OUTPUT=DBGCMD.LOG.
!
INITIALIZE/CONTAINER "testdata/disks/dbgcmd-exchange.dsk" /DEVICE=RD51 DBGCMDX
MOUNT/WRITE DUA1 "testdata/disks/dbgcmd-exchange.dsk"
COPY "testdata/dbgcmd/dbgcmd.mar"/HOST DUA1:[000000]DBGCMD.MAR
COPY "testdata/dbgcmd/dbgcmd.com"/HOST DUA1:[000000]DBGCMD.COM
COPY "testdata/dbgcmd/break.dbg"/HOST DUA1:[000000]BREAK.DBG
COPY "testdata/dbgcmd/brkcls.dbg"/HOST DUA1:[000000]BRKCLS.DBG
COPY "testdata/dbgcmd/step.dbg"/HOST DUA1:[000000]STEP.DBG
COPY "testdata/dbgcmd/watch.dbg"/HOST DUA1:[000000]WATCH.DBG
COPY "testdata/dbgcmd/trace.dbg"/HOST DUA1:[000000]TRACE.DBG
COPY "testdata/dbgcmd/exam.dbg"/HOST DUA1:[000000]EXAM.DBG
COPY "testdata/dbgcmd/call.dbg"/HOST DUA1:[000000]CALL.DBG
COPY "testdata/dbgcmd/except.dbg"/HOST DUA1:[000000]EXCEPT.DBG
COPY "testdata/dbgcmd/errors.dbg"/HOST DUA1:[000000]ERRORS.DBG
DIRECTORY DUA1:[000000]
DISMOUNT DUA1
