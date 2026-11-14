! EXCHANGE-R3B.CMD - builds round 3b of Phase 93's probe on a new volume.
! Written by testdata/prot93/gen3.go. Run from the repository root:
!
!     go run ./cmd/govax --profile tbprobe --system-file none console < testdata/prot93/exchange-r3b.cmd
!
INITIALIZE/CONTAINER "testdata/disks/sec93-exchange.dsk" /DEVICE=RD53 SEC93
MOUNT/WRITE DUA5 "testdata/disks/sec93-exchange.dsk"
CREATE/DIRECTORY DUA5:[PROT93LOG]/PROTECTION=(S:RWE,O:RWE,G:RWE,W:RWE)
CREATE/DIRECTORY DUA5:[SEC93]/PROTECTION=(S:RWE,O:RWE,G:RWE,W:RWE)
COPY "testdata/prot93/sec93.mar"/HOST DUA5:[000000]SEC93.MAR
COPY "testdata/prot93/sec93b.com"/HOST DUA5:[000000]SEC93B.COM
COPY "testdata/prot93/sec93y.com"/HOST DUA5:[000000]SEC93Y.COM
COPY "testdata/prot93/sec93t.dat"/HOST DUA5:[PROT93LOG]T.DAT
DIRECTORY/OWNER/PROTECTION DUA5:[000000],[PROT93LOG]
DISMOUNT DUA5
