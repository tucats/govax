! COPYOUT2.CMD - copies Phase 117's round 2 results off sum117-exchange.dsk
! into testdata/sum117/vax (README.md). Written by testdata/sum117/gen.go.
! Run from the repository root after the VAX runs round 2:
!
!     go run ./cmd/govax --profile tbprobe --system-file none console < testdata/sum117/copyout2.cmd
!     for f in testdata/sum117/vax/out/*\;*; do mv "$f" "${f%;*}"; done
!
MOUNT DUA5 "testdata/disks/sum117-exchange.dsk"
DIRECTORY/SIZE DUA5:[SUM117],[SUM117.SCR]
COPY DUA5:[SUM117]SUM117B.LOG "testdata/sum117/vax/sum117b.log"/HOST/QUIET
COPY DUA5:[SUM117.OUT]*.* "testdata/sum117/vax/out/"/HOST/QUIET
DISMOUNT DUA5
