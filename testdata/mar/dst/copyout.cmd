! COPYOUT.CMD - copies the results of Phase 29's second debugger-record
! probe off dst2-exchange.dsk into testdata/mar/dst/vax/ (docs/PHASE-29.md,
! subtask 12). Run from the repository root after @DST/OUTPUT=DST.LOG:
!
!     govax console < testdata/mar/dst/copyout.cmd
!
MOUNT DUA1 "testdata/disks/dst2-exchange.dsk"
COPY DUA1:[000000]DSTSYM.OBJ "testdata/mar/dst/vax/dstsym.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]DSTSYM.LIS "testdata/mar/dst/vax/dstsym.lis"/HOST/QUIET
COPY DUA1:[000000]DSTSYM.ANL "testdata/mar/dst/vax/dstsym.anl"/HOST/QUIET
COPY DUA1:[000000]DSTLN1.OBJ "testdata/mar/dst/vax/dstln1.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]DSTLN1.LIS "testdata/mar/dst/vax/dstln1.lis"/HOST/QUIET
COPY DUA1:[000000]DSTLN1.ANL "testdata/mar/dst/vax/dstln1.anl"/HOST/QUIET
COPY DUA1:[000000]DSTLN2.OBJ "testdata/mar/dst/vax/dstln2.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]DSTLN2.LIS "testdata/mar/dst/vax/dstln2.lis"/HOST/QUIET
COPY DUA1:[000000]DSTLN2.ANL "testdata/mar/dst/vax/dstln2.anl"/HOST/QUIET
COPY DUA1:[000000]DSTLN3.OBJ "testdata/mar/dst/vax/dstln3.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]DSTLN3.LIS "testdata/mar/dst/vax/dstln3.lis"/HOST/QUIET
COPY DUA1:[000000]DSTLN3.ANL "testdata/mar/dst/vax/dstln3.anl"/HOST/QUIET
COPY DUA1:[000000]DSTLN4.OBJ "testdata/mar/dst/vax/dstln4.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]DSTLN4.LIS "testdata/mar/dst/vax/dstln4.lis"/HOST/QUIET
COPY DUA1:[000000]DSTLN4.ANL "testdata/mar/dst/vax/dstln4.anl"/HOST/QUIET
COPY DUA1:[000000]DSTLN5.OBJ "testdata/mar/dst/vax/dstln5.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]DSTLN5.LIS "testdata/mar/dst/vax/dstln5.lis"/HOST/QUIET
COPY DUA1:[000000]DSTLN5.ANL "testdata/mar/dst/vax/dstln5.anl"/HOST/QUIET
COPY DUA1:[000000]DSTLN6.OBJ "testdata/mar/dst/vax/dstln6.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]DSTLN6.LIS "testdata/mar/dst/vax/dstln6.lis"/HOST/QUIET
COPY DUA1:[000000]DSTLN6.ANL "testdata/mar/dst/vax/dstln6.anl"/HOST/QUIET
COPY DUA1:[000000]DSTDIS.OBJ "testdata/mar/dst/vax/dstdis.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]DSTDIS.LIS "testdata/mar/dst/vax/dstdis.lis"/HOST/QUIET
COPY DUA1:[000000]DSTDIS.ANL "testdata/mar/dst/vax/dstdis.anl"/HOST/QUIET
COPY DUA1:[000000]DSTVAR.OBJ "testdata/mar/dst/vax/dstvar.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]DSTVAR.LIS "testdata/mar/dst/vax/dstvar.lis"/HOST/QUIET
COPY DUA1:[000000]DSTVAR.ANL "testdata/mar/dst/vax/dstvar.anl"/HOST/QUIET
COPY DUA1:[000000]DSTDBG.OBJ "testdata/mar/dst/vax/dstdbg.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]DSTDBG.LIS "testdata/mar/dst/vax/dstdbg.lis"/HOST/QUIET
COPY DUA1:[000000]DSTDBG.ANL "testdata/mar/dst/vax/dstdbg.anl"/HOST/QUIET
COPY DUA1:[000000]DSTVAR.MAR "testdata/mar/dst/vax/dstvar.mar"/HOST/QUIET
COPY DUA1:[000000]DST.LOG "testdata/mar/dst/vax/dst.log"/HOST/QUIET
DISMOUNT DUA1
