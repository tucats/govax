! COPYOUT.CMD - copies the results of the definition probes off mp-defs.dsk
! into testdata/mp/defs/vax/ (README.md). Run from the repository root
! after @DEFS/OUTPUT=DEFS.LOG:
!
!     govax console < testdata/mp/defs/copyout.cmd
!
MOUNT DUA1 "testdata/disks/mp-defs.dsk"
COPY DUA1:[000000]DEF_CLI.OBJ "testdata/mp/defs/vax/def_cli.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]DEF_CLI.ANL "testdata/mp/defs/vax/def_cli.anl"/HOST/QUIET
COPY DUA1:[000000]DEF_LIB.OBJ "testdata/mp/defs/vax/def_lib.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]DEF_LIB.ANL "testdata/mp/defs/vax/def_lib.anl"/HOST/QUIET
COPY DUA1:[000000]DEFS.LOG "testdata/mp/defs/vax/defs48.log"/HOST/QUIET
DISMOUNT DUA1
