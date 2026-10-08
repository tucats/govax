! COPYOUT.CMD - copies the results of the system service macro probes off
! mp-macros.dsk into testdata/mp/macros/vax/ (README.md). Written by gen.go.
! Run from the repository root:
!
!     govax console < testdata/mp/macros/copyout.cmd
!
MOUNT DUA1 "testdata/disks/mp-macros.dsk"
COPY DUA1:[000000]R5_MISC.OBJ "testdata/mp/macros/vax/r5_misc.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]R5_MISC.ANL "testdata/mp/macros/vax/r5_misc.anl"/HOST/QUIET
COPY DUA1:[000000]R6_CRMPSC.OBJ "testdata/mp/macros/vax/r6_crmpsc.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]R6_CRMPSC.ANL "testdata/mp/macros/vax/r6_crmpsc.anl"/HOST/QUIET
COPY DUA1:[000000]R6_MGBLSC.OBJ "testdata/mp/macros/vax/r6_mgblsc.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]R6_MGBLSC.ANL "testdata/mp/macros/vax/r6_mgblsc.anl"/HOST/QUIET
COPY DUA1:[000000]R6_DGBLSC.OBJ "testdata/mp/macros/vax/r6_dgblsc.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]R6_DGBLSC.ANL "testdata/mp/macros/vax/r6_dgblsc.anl"/HOST/QUIET
COPY DUA1:[000000]R6_ENQ.OBJ "testdata/mp/macros/vax/r6_enq.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]R6_ENQ.ANL "testdata/mp/macros/vax/r6_enq.anl"/HOST/QUIET
COPY DUA1:[000000]R6_ENQW.OBJ "testdata/mp/macros/vax/r6_enqw.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]R6_ENQW.ANL "testdata/mp/macros/vax/r6_enqw.anl"/HOST/QUIET
COPY DUA1:[000000]R6_DEQ.OBJ "testdata/mp/macros/vax/r6_deq.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]R6_DEQ.ANL "testdata/mp/macros/vax/r6_deq.anl"/HOST/QUIET
COPY DUA1:[000000]R6_GETLKI.OBJ "testdata/mp/macros/vax/r6_getlki.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]R6_GETLKI.ANL "testdata/mp/macros/vax/r6_getlki.anl"/HOST/QUIET
COPY DUA1:[000000]R6_GETLKIW.OBJ "testdata/mp/macros/vax/r6_getlkiw.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]R6_GETLKIW.ANL "testdata/mp/macros/vax/r6_getlkiw.anl"/HOST/QUIET
COPY DUA1:[000000]R7_LOCK.OBJ "testdata/mp/macros/vax/r7_lock.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]R7_LOCK.ANL "testdata/mp/macros/vax/r7_lock.anl"/HOST/QUIET
COPY DUA1:[000000]MACROS5.LOG "testdata/mp/macros/vax/macros5.log"/HOST/QUIET
COPY DUA1:[000000]MACROS6.LOG "testdata/mp/macros/vax/macros6.log"/HOST/QUIET
DISMOUNT DUA1
