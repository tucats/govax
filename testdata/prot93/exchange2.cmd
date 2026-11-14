! EXCHANGE2.CMD - adds round 2 of Phase 93's probe to round 1's volume.
! Written by testdata/prot93/gen.go. Run from the repository root, after
! keeping a copy of round 1's volume (README.md's "Round 2"):
!
!     go run ./cmd/govax --profile tbprobe --system-file none console < testdata/prot93/exchange2.cmd
!
MOUNT/WRITE DUA5 "testdata/disks/prot93-exchange.dsk"
COPY "testdata/prot93/prot93r.com"/HOST DUA5:[000000]PROT93R.COM
COPY "testdata/prot93/prot93v.com"/HOST DUA5:[000000]PROT93V.COM
COPY "testdata/prot93/prot93y.com"/HOST DUA5:[000000]PROT93Y.COM
COPY "testdata/prot93/prot93f.dat"/HOST DUA5:[000000]PROT93F.DAT
COPY "testdata/prot93/prot93g.dat"/HOST DUA5:[000000]PROT93G.DAT
DIRECTORY DUA5:[000000]PROT93*.*
DISMOUNT DUA5
