! EXCHANGE2.CMD - adds Phase 116's round 2 procedure to dif116-exchange.dsk.
! Written by testdata/dif116/gen.go. Run from the repository root:
!
!     go run ./cmd/govax --profile tbprobe --system-file none console < testdata/dif116/exchange2.cmd
!
MOUNT/WRITE DUA5 "testdata/disks/dif116-exchange.dsk"
COPY "testdata/dif116/dif116b.com"/HOST DUA5:[DIF116]DIF116B.COM
DIRECTORY/SIZE DUA5:[DIF116]
DISMOUNT DUA5
