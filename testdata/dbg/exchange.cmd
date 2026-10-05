! EXCHANGE.CMD - builds the exchange volume for Phase 41's debugger
! probe (docs/PHASE-41.md, subtask 1) with govax. Run from the
! repository root:
!
!     govax console < testdata/dbg/exchange.cmd
!
! It makes testdata/disks/dbg-exchange.dsk (RD51 size, label DBGXCHG,
! gitignored), copies the sources, the debugger command files, and
! DBG.COM onto it, and builds govax's own /DEBUG images of DBGDIS and
! TRACE there (GVDBGDIS.EXE, GVTRACE.EXE) for VMS's debugger to read.
! On VMS, mount it, make it the default directory, and run
! @DBG/OUTPUT=DBG.LOG.
!
INITIALIZE/CONTAINER "testdata/disks/dbg-exchange.dsk" /DEVICE=RD51 DBGXCHG
MOUNT/WRITE DUA1 "testdata/disks/dbg-exchange.dsk"
COPY "testdata/dbg/dbgdis.mar"/HOST DUA1:[000000]DBGDIS.MAR
COPY "testdata/dbg/dbgsub.mar"/HOST DUA1:[000000]DBGSUB.MAR
COPY "testdata/mar/list/trace.mar"/HOST DUA1:[000000]TRACE.MAR
COPY "testdata/mar/list/failmain.mar"/HOST DUA1:[000000]FAILMAIN.MAR
COPY "testdata/mar/list/failsub.mar"/HOST DUA1:[000000]FAILSUB.MAR
COPY "testdata/mar/forth.mar"/HOST DUA1:[000000]FORTH.MAR
COPY "testdata/dbg/dbgdis.dbg"/HOST DUA1:[000000]DBGDIS.DBG
COPY "testdata/dbg/trace.dbg"/HOST DUA1:[000000]TRACE.DBG
COPY "testdata/dbg/notb.dbg"/HOST DUA1:[000000]NOTB.DBG
COPY "testdata/dbg/fail.dbg"/HOST DUA1:[000000]FAIL.DBG
COPY "testdata/dbg/forth.dbg"/HOST DUA1:[000000]FORTH.DBG
COPY "testdata/dbg/dbg.com"/HOST DUA1:[000000]DBG.COM
MACRO/DEBUG/OBJECT=DUA1:[000000]GVDBGDIS.OBJ DUA1:[000000]DBGDIS.MAR
MACRO/DEBUG/OBJECT=DUA1:[000000]GVDBGSUB.OBJ DUA1:[000000]DBGSUB.MAR
LINK/DEBUG/MAP=DUA1:[000000]GVDBGDIS.MAP/EXECUTABLE=DUA1:[000000]GVDBGDIS.EXE DUA1:[000000]GVDBGDIS.OBJ,DUA1:[000000]GVDBGSUB.OBJ
MACRO/DEBUG/OBJECT=DUA1:[000000]GVTRACE.OBJ DUA1:[000000]TRACE.MAR
LINK/DEBUG/MAP=DUA1:[000000]GVTRACE.MAP/EXECUTABLE=DUA1:[000000]GVTRACE.EXE DUA1:[000000]GVTRACE.OBJ
DIRECTORY DUA1:[000000]
DISMOUNT DUA1
