! COPYOUT-R3B.CMD - copies round 3b's logs off sec93-exchange.dsk into
! testdata/prot93/vax/round3b. Written by testdata/prot93/gen3.go. Run from
! the repository root after the VAX runs:
!
!     go run ./cmd/govax --profile tbprobe --system-file none console < testdata/prot93/copyout-r3b.cmd
!
MOUNT DUA5 "testdata/disks/sec93-exchange.dsk"
COPY DUA5:[PROT93LOG]SEC93B.LOG "testdata/prot93/vax/round3b/sec93b.log"/HOST/QUIET
COPY DUA5:[PROT93LOG]SEC93Y.LOG "testdata/prot93/vax/round3b/sec93y.log"/HOST/QUIET
DISMOUNT DUA5
