! EXCHANGE3.CMD - adds PROT93S.COM to round 1's volume, after round 2's
! accounts were refused PROT93V.COM (README.md's "Round 2"). Written by
! testdata/prot93/gen.go. Run from the repository root:
!
!     go run ./cmd/govax --profile tbprobe --system-file none console < testdata/prot93/exchange3.cmd
!
MOUNT/WRITE DUA5 "testdata/disks/prot93-exchange.dsk"
COPY "testdata/prot93/prot93s.com"/HOST DUA5:[000000]PROT93S.COM
DIRECTORY/OWNER/PROTECTION DUA5:[000000]PROT93*.*
DISMOUNT DUA5
