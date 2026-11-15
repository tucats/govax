! EXCHANGE.CMD - builds Phase 94's probe exchange volume with govax.
! Written by testdata/priv94/gen.go. Run from the repository root:
!
!     go run ./cmd/govax --profile tbprobe --system-file none console < testdata/priv94/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/priv94-exchange.dsk" /DEVICE=RD53 PRIV94
MOUNT/WRITE DUA5 "testdata/disks/priv94-exchange.dsk"
CREATE/DIRECTORY DUA5:[PRIV94LOG]/PROTECTION=(S:RWE,O:RWE,G:RWE,W:RWE)
COPY "testdata/priv94/priv94.mar"/HOST DUA5:[000000]PRIV94.MAR
COPY "testdata/priv94/priv94h.mar"/HOST DUA5:[000000]PRIV94H.MAR
COPY "testdata/priv94/priv94i.mar"/HOST DUA5:[000000]PRIV94I.MAR
COPY "testdata/priv94/priv94.com"/HOST DUA5:[000000]PRIV94.COM
COPY "testdata/priv94/priv94p.com"/HOST DUA5:[000000]PRIV94P.COM
COPY "testdata/priv94/priv94d.com"/HOST DUA5:[000000]PRIV94D.COM
COPY "testdata/priv94/priv94u.com"/HOST DUA5:[000000]PRIV94U.COM
COPY "testdata/priv94/priv94z.com"/HOST DUA5:[000000]PRIV94Z.COM
DIRECTORY/OWNER/PROTECTION DUA5:[000000]
DISMOUNT DUA5
