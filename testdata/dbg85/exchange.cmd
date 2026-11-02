! EXCHANGE.CMD - builds the exchange volume for Phase 85's debugger probe
! (README.md) with govax. Run from the repository root:
!
!     govax console < testdata/dbg85/exchange.cmd
!
! It makes testdata/disks/dbg85-exchange.dsk (RD51 size, label DBG85, gitignored),
! on DUA2, since a startup procedure may mount DUA1, with the sources, the
! compiler's MACRO files for DBG85.FOR, and govax's own /DEBUG images. On VMS, mount
! it, make it the default directory, and run @DBG85/OUTPUT=DBG85.LOG.
!
INITIALIZE/CONTAINER "testdata/disks/dbg85-exchange.dsk" /DEVICE=RD51 DBG85
MOUNT/WRITE DUA2 "testdata/disks/dbg85-exchange.dsk"
COPY "testdata/dbg85/dbg85.for"/HOST DUA2:[000000]DBG85.FOR
COPY "testdata/dbg85/dbg85m.mar"/HOST DUA2:[000000]DBG85M.MAR
COPY "testdata/dbg85/dbg85n.mar"/HOST DUA2:[000000]DBG85N.MAR
COPY "testdata/dbg85/dbg85s.mar"/HOST DUA2:[000000]DBG85S.MAR
COPY "testdata/dbg85/dbg85s.opt"/HOST DUA2:[000000]DBG85S.OPT
COPY "testdata/dbg85/dbg85c.mar"/HOST DUA2:[000000]DBG85C.MAR
COPY "testdata/dbg85/dbg85c.opt"/HOST DUA2:[000000]DBG85C.OPT
COPY "testdata/dbg85/dbg85.dbg"/HOST DUA2:[000000]DBG85.DBG
COPY "testdata/dbg85/dbg85m.dbg"/HOST DUA2:[000000]DBG85M.DBG
COPY "testdata/dbg85/dbg85n.dbg"/HOST DUA2:[000000]DBG85N.DBG
COPY "testdata/dbg85/dbg85c.dbg"/HOST DUA2:[000000]DBG85C.DBG
COPY "testdata/dbg85/dbg85.com"/HOST DUA2:[000000]DBG85.COM
FORTRAN/NOOPTIMIZE DUA2:[000000]DBG85.FOR
FORTRAN/OPTIMIZE/MACRO=DUA2:[000000]DBG85O.MAR DUA2:[000000]DBG85.FOR
MACRO/DEBUG/OBJECT=DUA2:[000000]GVDBG85.OBJ DUA2:[000000]DBG85.MAR
LINK/DEBUG/NOSYSLIB/MAP=DUA2:[000000]GVDBG85.MAP/EXECUTABLE=DUA2:[000000]GVDBG85.EXE DUA2:[000000]GVDBG85.OBJ
MACRO/DEBUG/OBJECT=DUA2:[000000]GVDBG85M.OBJ DUA2:[000000]DBG85M.MAR
LINK/DEBUG/NOSYSLIB/MAP=DUA2:[000000]GVDBG85M.MAP/EXECUTABLE=DUA2:[000000]GVDBG85M.EXE DUA2:[000000]GVDBG85M.OBJ
DIRECTORY DUA2:[000000]
DISMOUNT DUA2
