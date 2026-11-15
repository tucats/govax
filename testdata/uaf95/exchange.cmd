! EXCHANGE.CMD - builds Phase 95's probe exchange volume with govax.
! Written by testdata/uaf95/gen.go. Run from the repository root:
!
!     go run ./cmd/govax --profile tbprobe --system-file none console < testdata/uaf95/exchange.cmd
!
INITIALIZE/CONTAINER "testdata/disks/uaf95-exchange.dsk" /DEVICE=RD53 UAF95
MOUNT/WRITE DUA5 "testdata/disks/uaf95-exchange.dsk"
CREATE/DIRECTORY DUA5:[UAF95LOG]/PROTECTION=(S:RWE,O:RWE,G:RWE,W:RWE)
CREATE/DIRECTORY DUA5:[UAF95SCR]/PROTECTION=(S:RWE,O:RWE,G:,W:)
CREATE/DIRECTORY DUA5:[UAF95]
COPY "testdata/uaf95/uaf95.com"/HOST DUA5:[000000]UAF95.COM
COPY "testdata/uaf95/uaf95acct.com"/HOST DUA5:[000000]UAF95ACCT.COM
COPY "testdata/uaf95/uaf95drop.com"/HOST DUA5:[000000]UAF95DROP.COM
COPY "testdata/uaf95/uaf95i.com"/HOST DUA5:[000000]UAF95I.COM
COPY "testdata/uaf95/uaf95s.com"/HOST DUA5:[000000]UAF95S.COM
COPY "testdata/uaf95/login.com"/HOST DUA5:[UAF95]LOGIN.COM
COPY "testdata/uaf95/uaf95x.com"/HOST DUA5:[UAF95]UAF95X.COM
DIRECTORY/OWNER/PROTECTION DUA5:[000000...]
DISMOUNT DUA5
