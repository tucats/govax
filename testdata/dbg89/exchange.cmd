! EXCHANGE.CMD - builds the exchange volume for Phase 89's debugger probe
! (README.md) with govax. Run from the repository root:
!
!     govax console < testdata/dbg89/exchange.cmd
!
! It makes testdata/disks/dbg89-exchange.dsk (RD51 size, label DBG89, gitignored),
! on DUA2, since a startup procedure may mount DUA1, with the sources, the
! compiler's MACRO file for DBG89F.FOR, and govax's own /DEBUG image of DBG89.
! On VMS, mount it, make it the default directory, and run @DBG89/OUTPUT=DBG89.LOG.
!
INITIALIZE/CONTAINER "testdata/disks/dbg89-exchange.dsk" /DEVICE=RD51 DBG89
MOUNT/WRITE DUA2 "testdata/disks/dbg89-exchange.dsk"
COPY "testdata/dbg89/dbg89.mar"/HOST DUA2:[000000]DBG89.MAR
COPY "testdata/dbg89/dbg89b.mar"/HOST DUA2:[000000]DBG89B.MAR
COPY "testdata/dbg89/dbg89f.for"/HOST DUA2:[000000]DBG89F.FOR
COPY "testdata/dbg89/dbg89.dbg"/HOST DUA2:[000000]DBG89.DBG
COPY "testdata/dbg89/dbg89s.dbg"/HOST DUA2:[000000]DBG89S.DBG
COPY "testdata/dbg89/dbg89f.dbg"/HOST DUA2:[000000]DBG89F.DBG
COPY "testdata/dbg89/dbg89.com"/HOST DUA2:[000000]DBG89.COM
FORTRAN/NOOPTIMIZE DUA2:[000000]DBG89F.FOR
MACRO/DEBUG/OBJECT=DUA2:[000000]GVDBG89.OBJ DUA2:[000000]DBG89.MAR
MACRO/DEBUG/OBJECT=DUA2:[000000]GVDBG89B.OBJ DUA2:[000000]DBG89B.MAR
LINK/DEBUG/NOSYSLIB/MAP=DUA2:[000000]GVDBG89.MAP/EXECUTABLE=DUA2:[000000]GVDBG89.EXE DUA2:[000000]GVDBG89.OBJ,DUA2:[000000]GVDBG89B.OBJ
DIRECTORY DUA2:[000000]
DISMOUNT DUA2
