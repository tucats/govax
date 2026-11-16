! COPYOUT10.CMD - copies round 10's results off mp-macros.dsk into
! testdata/mp/macros/vax/ (README.md, "Round 10"). Written by gen.go.
! Run from the repository root:
!
!     go run ./cmd/govax --profile tbprobe --system-file none console < testdata/mp/macros/copyout10.cmd
!
MOUNT DUA1 "testdata/disks/mp-macros.dsk"
COPY DUA1:[000000]R10_SNDJBC.OBJ "testdata/mp/macros/vax/r10_sndjbc.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]R10_SNDJBC.ANL "testdata/mp/macros/vax/r10_sndjbc.anl"/HOST/QUIET
COPY DUA1:[000000]R10_SNDJBCW.OBJ "testdata/mp/macros/vax/r10_sndjbcw.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]R10_SNDJBCW.ANL "testdata/mp/macros/vax/r10_sndjbcw.anl"/HOST/QUIET
COPY DUA1:[000000]R10_GETQUI.OBJ "testdata/mp/macros/vax/r10_getqui.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]R10_GETQUI.ANL "testdata/mp/macros/vax/r10_getqui.anl"/HOST/QUIET
COPY DUA1:[000000]R10_GETQUIW.OBJ "testdata/mp/macros/vax/r10_getquiw.obj"/HOST/BINARY/QUIET
COPY DUA1:[000000]R10_GETQUIW.ANL "testdata/mp/macros/vax/r10_getquiw.anl"/HOST/QUIET
COPY DUA1:[000000]MACROS10.LOG "testdata/mp/macros/vax/macros10.log"/HOST/QUIET
DISMOUNT DUA1
