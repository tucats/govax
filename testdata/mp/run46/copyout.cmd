! COPYOUT.CMD - copies the results of RUN46.COM off mp-run46.dsk, each
! into its own directory's vax/ (testdata/mp/run46/README.md). Run from
! the repository root:
!
!     govax console < testdata/mp/run46/copyout.cmd
!
MOUNT DUA1 "testdata/disks/mp-run46.dsk"
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
COPY DUA1:[000000]MACROS5.LOG "testdata/mp/macros/vax/macros5.log"/HOST/QUIET
COPY DUA1:[000000]MACROS6.LOG "testdata/mp/macros/vax/macros6.log"/HOST/QUIET
COPY DUA1:[000000]DEF_SEC.OBJ "testdata/mp/defs/vax/def_sec.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]DEF_SEC.ANL "testdata/mp/defs/vax/def_sec.anl"/HOST/QUIET
COPY DUA1:[000000]DEF_LCK.OBJ "testdata/mp/defs/vax/def_lck.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]DEF_LCK.ANL "testdata/mp/defs/vax/def_lck.anl"/HOST/QUIET
COPY DUA1:[000000]DEF_LKI.OBJ "testdata/mp/defs/vax/def_lki.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]DEF_LKI.ANL "testdata/mp/defs/vax/def_lki.anl"/HOST/QUIET
COPY DUA1:[000000]DEF_PSL.OBJ "testdata/mp/defs/vax/def_psl.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]DEF_PSL.ANL "testdata/mp/defs/vax/def_psl.anl"/HOST/QUIET
COPY DUA1:[000000]DEF_DC.OBJ "testdata/mp/defs/vax/def_dc.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]DEF_DC.ANL "testdata/mp/defs/vax/def_dc.anl"/HOST/QUIET
COPY DUA1:[000000]DEFS.LOG "testdata/mp/defs/vax/defs46.log"/HOST/QUIET
COPY DUA1:[000000]PROBE3.LOG "testdata/mp/probe3/vax/probe3.log"/HOST/QUIET
COPY DUA1:[000000]PINGPONG.LOG "testdata/mp/vax/pingpong.log"/HOST/QUIET
DISMOUNT DUA1
