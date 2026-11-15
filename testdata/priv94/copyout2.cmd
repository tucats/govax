! COPYOUT2.CMD - copies round 2's results off priv94-exchange.dsk into
! testdata/priv94/vax/round2 (README.md). Written by testdata/priv94/gen.go.
! Run from the repository root after the VAX runs:
!
!     go run ./cmd/govax --profile tbprobe --system-file none console < testdata/priv94/copyout2.cmd
!
MOUNT DUA5 "testdata/disks/priv94-exchange.dsk"
COPY DUA5:[PRIV94LOG]PRIV94R.LOG "testdata/priv94/vax/round2/priv94r.log"/HOST/QUIET
COPY DUA5:[PRIV94LOG]PRIV94V.LOG "testdata/priv94/vax/round2/priv94v.log"/HOST/QUIET
COPY DUA5:[PRIV94LOG]PRIV94Y.LOG "testdata/priv94/vax/round2/priv94y.log"/HOST/QUIET
COPY/BINARY DUA5:[000000]PRIV94B.EXE "testdata/priv94/vax/round2/priv94b.exe"/HOST/QUIET
DISMOUNT DUA5
