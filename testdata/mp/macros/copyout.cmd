! COPYOUT.CMD - copies the results of the system service macro probes off
! mp-macros.dsk into testdata/mp/macros/vax/ (README.md). Written by gen.go.
! Run from the repository root:
!
!     govax console < testdata/mp/macros/copyout.cmd
!
MOUNT DUA1 "testdata/disks/mp-macros.dsk"
COPY DUA1:[000000]R5_MISC.OBJ "testdata/mp/macros/vax/r5_misc.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]R5_MISC.ANL "testdata/mp/macros/vax/r5_misc.anl"/HOST/QUIET
COPY DUA1:[000000]MACROS.LOG "testdata/mp/macros/vax/macros5.log"/HOST/QUIET
DISMOUNT DUA1
