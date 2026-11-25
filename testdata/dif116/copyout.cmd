! COPYOUT.CMD - copies Phase 116's probe results off dif116-exchange.dsk
! into testdata/dif116/vax (README.md). Written by testdata/dif116/gen.go.
! Run from the repository root after the VAX runs:
!
!     mkdir -p testdata/dif116/vax/out
!     go run ./cmd/govax --profile tbprobe --system-file none console < testdata/dif116/copyout.cmd
!
MOUNT DUA5 "testdata/disks/dif116-exchange.dsk"
DIRECTORY/SIZE DUA5:[DIF116],[DIF116.OUT]
COPY DUA5:[DIF116]DIF116.LOG "testdata/dif116/vax/dif116.log"/HOST/QUIET
COPY DUA5:[DIF116.OUT]*.DIF "testdata/dif116/vax/out/"/HOST/QUIET
DISMOUNT DUA5
