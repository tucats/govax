! COPYOUT.CMD - copies the results of the Phase 45 definition probes off
! mp-defs.dsk into testdata/mp/defs/vax/ (README.md). Run from the
! repository root after @DEFS/OUTPUT=DEFS.LOG:
!
!     govax console < testdata/mp/defs/copyout.cmd
!
MOUNT DUA1 "testdata/disks/mp-defs.dsk"
COPY DUA1:[000000]DEF_ACC.OBJ "testdata/mp/defs/vax/def_acc.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]DEF_ACC.ANL "testdata/mp/defs/vax/def_acc.anl"/HOST/QUIET
COPY DUA1:[000000]DEF_MSG.OBJ "testdata/mp/defs/vax/def_msg.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]DEF_MSG.ANL "testdata/mp/defs/vax/def_msg.anl"/HOST/QUIET
COPY DUA1:[000000]DEF_PQL.OBJ "testdata/mp/defs/vax/def_pql.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]DEF_PQL.ANL "testdata/mp/defs/vax/def_pql.anl"/HOST/QUIET
COPY DUA1:[000000]DEF_PRC.OBJ "testdata/mp/defs/vax/def_prc.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]DEF_PRC.ANL "testdata/mp/defs/vax/def_prc.anl"/HOST/QUIET
COPY DUA1:[000000]DEFS.LOG "testdata/mp/defs/vax/defs.log"/HOST/QUIET
DISMOUNT DUA1
