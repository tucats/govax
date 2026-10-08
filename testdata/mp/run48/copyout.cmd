! COPYOUT.CMD - copies the results of every VAX run of Phase 48 off
! mp-run48.dsk into the repository (README.md). Run from the repository
! root after @RUN48:
!
!     govax console < testdata/mp/run48/copyout.cmd
!
MOUNT DUA1 "testdata/disks/mp-run48.dsk"
COPY DUA1:[000000]DEF_CLI.OBJ "testdata/mp/defs/vax/def_cli.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]DEF_CLI.ANL "testdata/mp/defs/vax/def_cli.anl"/HOST/QUIET
COPY DUA1:[000000]DEF_LIB.OBJ "testdata/mp/defs/vax/def_lib.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]DEF_LIB.ANL "testdata/mp/defs/vax/def_lib.anl"/HOST/QUIET
COPY DUA1:[000000]DEFS.LOG "testdata/mp/defs/vax/defs48.log"/HOST/QUIET
COPY DUA1:[000000]MILESTONE.LOG "testdata/mp/vax/milestone.log"/HOST/QUIET
COPY DUA1:[000000]PROBE4.LOG "testdata/mp/probe4/vax/probe4.log"/HOST/QUIET
DISMOUNT DUA1
