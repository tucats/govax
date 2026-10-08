! COPYOUT.CMD - copies the results of the definition probes off mp-defs.dsk
! into testdata/mp/defs/vax/ (README.md). Run from the repository root
! after @DEFS/OUTPUT=DEFS.LOG:
!
!     govax console < testdata/mp/defs/copyout.cmd
!
MOUNT DUA1 "testdata/disks/mp-defs.dsk"
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
DISMOUNT DUA1
