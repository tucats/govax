! COPYOUT2.CMD - copies Phase 116's round 2 results off dif116-exchange.dsk
! into testdata/dif116/vax (README.md). Written by testdata/dif116/gen.go.
! Run from the repository root after the VAX runs round 2:
!
!     go run ./cmd/govax --profile tbprobe --system-file none console < testdata/dif116/copyout2.cmd
!     for f in testdata/dif116/vax/out/*\;1; do mv "$f" "${f%;1}"; done
!
MOUNT DUA5 "testdata/disks/dif116-exchange.dsk"
DIRECTORY/SIZE DUA5:[DIF116],[DIF116.OUT],[DIF116.SCR]
COPY DUA5:[DIF116]DIF116B.LOG "testdata/dif116/vax/dif116b.log"/HOST/QUIET
COPY DUA5:[DIF116.OUT]*.DIF "testdata/dif116/vax/out/"/HOST/QUIET
DISMOUNT DUA5
