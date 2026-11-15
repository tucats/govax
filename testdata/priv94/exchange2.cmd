! EXCHANGE2.CMD - adds round 2 of Phase 94's probe to round 1's exchange
! volume. Written by testdata/priv94/gen.go. Run from the repository root:
!
!     go run ./cmd/govax --profile tbprobe --system-file none console < testdata/priv94/exchange2.cmd
!
MOUNT/WRITE DUA5 "testdata/disks/priv94-exchange.dsk"
COPY "testdata/priv94/priv94b.mar"/HOST DUA5:[000000]PRIV94B.MAR
COPY "testdata/priv94/priv94r.com"/HOST DUA5:[000000]PRIV94R.COM
COPY "testdata/priv94/priv94v.com"/HOST DUA5:[000000]PRIV94V.COM
COPY "testdata/priv94/priv94e.com"/HOST DUA5:[000000]PRIV94E.COM
COPY "testdata/priv94/priv94y.com"/HOST DUA5:[000000]PRIV94Y.COM
DIRECTORY/OWNER/PROTECTION DUA5:[000000]PRIV94*.*
DISMOUNT DUA5
