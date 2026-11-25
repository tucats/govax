! COPYOUT.CMD - copies Phase 117's probe results off sum117-exchange.dsk
! into testdata/sum117/vax (README.md). Written by testdata/sum117/gen.go.
! Run from the repository root after the VAX runs:
!
!     mkdir -p testdata/sum117/vax/out
!     go run ./cmd/govax --profile tbprobe --system-file none console < testdata/sum117/copyout.cmd
!
MOUNT DUA5 "testdata/disks/sum117-exchange.dsk"
DIRECTORY/SIZE DUA5:[SUM117],[SUM117.OUT],[SUM117.SCR]
COPY DUA5:[SUM117]SUM117.LOG "testdata/sum117/vax/sum117.log"/HOST/QUIET
COPY DUA5:[SUM117.OUT]*.* "testdata/sum117/vax/out/"/HOST/QUIET
DISMOUNT DUA5
