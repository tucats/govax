! EXCHANGE.CMD - builds the exchange volume for Phase 29's second
! debugger-record probe (docs/PHASE-29.md, subtask 12) with govax. Run
! from the repository root:
!
!     govax console < testdata/mar/dst/exchange.cmd
!
! It makes testdata/disks/dst2-exchange.dsk (RD51 size, label DST2XCHG,
! gitignored) and copies the sources and DST.COM onto it. On VMS, mount
! it, make it the default directory, and run @DST/OUTPUT=DST.LOG.
!
INITIALIZE/CONTAINER "testdata/disks/dst2-exchange.dsk" /DEVICE=RD51 DST2XCHG
MOUNT/WRITE DUA1 "testdata/disks/dst2-exchange.dsk"
COPY "testdata/mar/dst/dstsym.mar"/HOST DUA1:[000000]DSTSYM.MAR
COPY "testdata/mar/dst/dstln1.mar"/HOST DUA1:[000000]DSTLN1.MAR
COPY "testdata/mar/dst/dstln2.mar"/HOST DUA1:[000000]DSTLN2.MAR
COPY "testdata/mar/dst/dstln3.mar"/HOST DUA1:[000000]DSTLN3.MAR
COPY "testdata/mar/dst/dstln4.mar"/HOST DUA1:[000000]DSTLN4.MAR
COPY "testdata/mar/dst/dstln5.mar"/HOST DUA1:[000000]DSTLN5.MAR
COPY "testdata/mar/dst/dstln6.mar"/HOST DUA1:[000000]DSTLN6.MAR
COPY "testdata/mar/dst/dstdbg.mar"/HOST DUA1:[000000]DSTDBG.MAR
COPY "testdata/mar/dst/dstdis.mar"/HOST DUA1:[000000]DSTDIS.MAR
COPY "testdata/mar/dst/dst.com"/HOST DUA1:[000000]DST.COM
DIRECTORY DUA1:[000000]
DISMOUNT DUA1
